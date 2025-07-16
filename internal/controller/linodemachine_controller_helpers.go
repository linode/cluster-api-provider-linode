/*
Copyright 2023 Akamai Technologies, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"bytes"
	"compress/gzip"
	"context"
	b64 "encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"text/template"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/linode/linodego"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	kutil "sigs.k8s.io/cluster-api/util"
	conditions "sigs.k8s.io/cluster-api/util/conditions/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/cloud/services"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"

	_ "embed"
)

const (
	maxBootstrapDataBytesCloudInit = 16384
	vlanIPFormat                   = "%s/11"
)

var (
	//go:embed cloud-init.tmpl
	cloudConfigTemplate string

	errNoPublicIPv4Addrs      = errors.New("no public ipv4 addresses set")
	errNoPublicIPv6Addrs      = errors.New("no public IPv6 address set")
	errNoPublicIPv6SLAACAddrs = errors.New("no public SLAAC address set")
)

func retryIfTransient(err error, logger logr.Logger) (ctrl.Result, error) {
	if util.IsRetryableError(err) {
		if linodego.ErrHasStatus(err, http.StatusTooManyRequests) {
			return ctrl.Result{RequeueAfter: reconciler.DefaultLinodeTooManyRequestsErrorRetryDelay}, nil
		}
		return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerRetryDelay}, nil
	}
	logger.Error(err, "unknown Linode API error")
	return ctrl.Result{RequeueAfter: reconciler.DefaultMachineControllerRetryDelay}, nil
}

func fillCreateConfig(createConfig *linodego.InstanceCreateOptions, machineScope *scope.MachineScope) {
	if machineScope.LinodeMachine.Spec.PrivateIP != nil {
		createConfig.PrivateIP = *machineScope.LinodeMachine.Spec.PrivateIP
	} else {
		createConfig.PrivateIP = true
	}

	if createConfig.Tags == nil {
		createConfig.Tags = []string{}
	}

	if createConfig.Label == "" {
		createConfig.Label = machineScope.LinodeMachine.Name
	}

	if createConfig.Image == "" {
		createConfig.Image = reconciler.DefaultMachineControllerLinodeImage
	}
	if createConfig.RootPass == "" {
		createConfig.RootPass = uuid.NewString()
	}
}

func newCreateConfig(ctx context.Context, machineScope *scope.MachineScope, gzipCompressionEnabled bool, logger logr.Logger) (*linodego.InstanceCreateOptions, error) {
	var err error

	createConfig := linodeMachineSpecToInstanceCreateConfig(machineScope, getTags(machineScope, []string{}))
	if createConfig == nil {
		err = errors.New("failed to convert machine spec to create instance config")
		logger.Error(err, "Panic! Struct of LinodeMachineSpec is different than InstanceCreateOptions")
		return nil, err
	}

	createConfig.Booted = util.Pointer(false)
	if err := setUserData(ctx, machineScope, createConfig, gzipCompressionEnabled, logger); err != nil {
		return nil, err
	}

	fillCreateConfig(createConfig, machineScope)

	// Configure VPC interface if needed
	if err := configureVPCInterface(ctx, machineScope, createConfig, logger); err != nil {
		return nil, err
	}

	// Configure VLAN interface if needed
	if machineScope.LinodeCluster.Spec.Network.UseVlan {
		if err := configureVlanInterface(ctx, machineScope, createConfig, logger); err != nil {
			return nil, err
		}
	}

	// Configure placement group if needed
	if machineScope.LinodeMachine.Spec.PlacementGroupRef != nil {
		if err := configurePlacementGroup(ctx, machineScope, createConfig, logger); err != nil {
			return nil, err
		}
	}

	// Configure firewall if needed
	if machineScope.LinodeMachine.Spec.FirewallRef != nil || machineScope.LinodeMachine.Spec.FirewallID != 0 {
		if err := configureFirewall(ctx, machineScope, createConfig, logger); err != nil {
			return nil, err
		}
	}

	return createConfig, nil
}

// configureVPCInterface handles all VPC configuration scenarios and adds the appropriate interface
func configureVPCInterface(ctx context.Context, machineScope *scope.MachineScope, createConfig *linodego.InstanceCreateOptions, logger logr.Logger) error {
	// First check if a direct VPCID is specified on the machine then the cluster
	if machineScope.LinodeMachine.Spec.VPCID != nil {
		return addVPCInterfaceFromDirectID(ctx, machineScope, createConfig, logger, *machineScope.LinodeMachine.Spec.VPCID)
	} else if machineScope.LinodeCluster.Spec.VPCID != nil {
		return addVPCInterfaceFromDirectID(ctx, machineScope, createConfig, logger, *machineScope.LinodeCluster.Spec.VPCID)
	}

	// Finally check for VPC reference
	if vpcRef := getVPCRefFromScope(machineScope); vpcRef != nil {
		return addVPCInterfaceFromReference(ctx, machineScope, createConfig, logger, vpcRef)
	}

	// No VPC configuration found, nothing to do
	return nil
}

// addVPCInterfaceFromDirectID handles adding a VPC interface from a direct ID
func addVPCInterfaceFromDirectID(ctx context.Context, machineScope *scope.MachineScope, createConfig *linodego.InstanceCreateOptions, logger logr.Logger, vpcID int) error {
	iface, err := getVPCInterfaceConfigFromDirectID(ctx, machineScope, createConfig.Interfaces, logger, vpcID)
	if err != nil {
		logger.Error(err, "Failed to get VPC interface config from direct ID")
		return err
	}

	if iface != nil {
		// add VPC interface as first interface
		createConfig.Interfaces = slices.Insert(createConfig.Interfaces, 0, *iface)
	}

	return nil
}

// addVPCInterfaceFromReference handles adding a VPC interface from a reference
func addVPCInterfaceFromReference(ctx context.Context, machineScope *scope.MachineScope, createConfig *linodego.InstanceCreateOptions, logger logr.Logger, vpcRef *corev1.ObjectReference) error {
	iface, err := getVPCInterfaceConfig(ctx, machineScope, createConfig.Interfaces, logger, vpcRef)
	if err != nil {
		logger.Error(err, "Failed to get VPC interface config")
		return err
	}

	if iface != nil {
		// add VPC interface as first interface
		createConfig.Interfaces = slices.Insert(createConfig.Interfaces, 0, *iface)
	}

	return nil
}

func buildInstanceAddrs(ctx context.Context, machineScope *scope.MachineScope, instanceID int) ([]clusterv1.MachineAddress, error) {
	addresses, err := machineScope.LinodeClient.GetInstanceIPAddresses(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("get instance ips: %w", err)
	}

	ips := []clusterv1.MachineAddress{}
	// check if a node has public ipv4 ip and store it
	if len(addresses.IPv4.Public) == 0 {
		return nil, errNoPublicIPv4Addrs
	}
	ips = append(ips, clusterv1.MachineAddress{
		Address: addresses.IPv4.Public[0].Address,
		Type:    clusterv1.MachineExternalIP,
	})

	// check if a node has public ipv6 ip and store it
	if addresses.IPv6 == nil {
		return nil, errNoPublicIPv6Addrs
	}
	if addresses.IPv6.SLAAC == nil {
		return nil, errNoPublicIPv6SLAACAddrs
	}
	ips = append(ips, clusterv1.MachineAddress{
		Address: addresses.IPv6.SLAAC.Address,
		Type:    clusterv1.MachineExternalIP,
	})

	// check if a node has vpc specific ip and store it
	for _, vpcIP := range addresses.IPv4.VPC {
		if vpcIP.Address != nil && *vpcIP.Address != "" {
			ips = append(ips, clusterv1.MachineAddress{
				Address: *vpcIP.Address,
				Type:    clusterv1.MachineInternalIP,
			})
		}
	}

	if machineScope.LinodeCluster.Spec.Network.UseVlan {
		// get the default instance config
		configs, err := machineScope.LinodeClient.ListInstanceConfigs(ctx, instanceID, &linodego.ListOptions{})
		if err != nil || len(configs) == 0 {
			return nil, fmt.Errorf("list instance configs: %w", err)
		}

		// Iterate over interfaces in config and find VLAN specific ips
		for _, iface := range configs[0].Interfaces {
			if iface.Purpose == linodego.InterfacePurposeVLAN {
				// vlan addresses have a /11 appended to them - we should strip it out.
				ips = append(ips, clusterv1.MachineAddress{
					Address: netip.MustParsePrefix(iface.IPAMAddress).Addr().String(),
					Type:    clusterv1.MachineInternalIP,
				})
			}
		}
	}

	// if a node has private ip, store it as well
	// NOTE: We specifically store VPC ips first so that they are used first during
	//       bootstrap when we set `registrationMethod: internal-only-ips`
	if len(addresses.IPv4.Private) != 0 {
		ips = append(ips, clusterv1.MachineAddress{
			Address: addresses.IPv4.Private[0].Address,
			Type:    clusterv1.MachineInternalIP,
		})
	}

	return ips, nil
}

func linodeClusterToLinodeMachines(logger logr.Logger, tracedClient client.Client) handler.MapFunc {
	logger = logger.WithName("LinodeMachineReconciler").WithName("linodeClusterToLinodeMachines")

	return func(ctx context.Context, o client.Object) []ctrl.Request {
		ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultMappingTimeout)
		defer cancel()

		linodeCluster, ok := o.(*infrav1alpha2.LinodeCluster)
		if !ok {
			logger.Info("Failed to cast object to Cluster")

			return nil
		}

		if !linodeCluster.DeletionTimestamp.IsZero() {
			logger.Info("Cluster has a deletion timestamp, skipping mapping")

			return nil
		}

		cluster, err := kutil.GetOwnerCluster(ctx, tracedClient, linodeCluster.ObjectMeta)
		switch {
		case apierrors.IsNotFound(err) || cluster == nil:
			logger.Info("Cluster for LinodeCluster not found, skipping mapping")

			return nil
		case err != nil:
			logger.Error(err, "Failed to get owning cluster, skipping mapping")

			return nil
		}

		request, err := requestsForCluster(ctx, tracedClient, cluster.Namespace, cluster.Name)
		if err != nil {
			logger.Error(err, "Failed to create request for cluster")

			return nil
		}

		return request
	}
}

func requestsForCluster(ctx context.Context, tracedClient client.Client, namespace, name string) ([]ctrl.Request, error) {
	labels := map[string]string{clusterv1.ClusterNameLabel: name}

	machineList := clusterv1.MachineList{}
	if err := tracedClient.List(ctx, &machineList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	result := make([]ctrl.Request, 0, len(machineList.Items))
	for _, item := range machineList.Items {
		if item.Spec.InfrastructureRef.GroupVersionKind().Kind != "LinodeMachine" || item.Spec.InfrastructureRef.Name == "" {
			continue
		}

		infraNs := item.Spec.InfrastructureRef.Namespace
		if infraNs == "" {
			infraNs = item.Namespace
		}

		result = append(result, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Namespace: infraNs,
				Name:      item.Spec.InfrastructureRef.Name,
			},
		})
	}

	return result, nil
}

func getPlacementGroupID(ctx context.Context, machineScope *scope.MachineScope, logger logr.Logger) (int, error) {
	name := machineScope.LinodeMachine.Spec.PlacementGroupRef.Name
	namespace := machineScope.LinodeMachine.Spec.PlacementGroupRef.Namespace
	if namespace == "" {
		namespace = machineScope.LinodeMachine.Namespace
	}

	logger = logger.WithValues("placementGroupName", name, "placementGroupNamespace", namespace)

	linodePlacementGroup := infrav1alpha2.LinodePlacementGroup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	objectKey := client.ObjectKeyFromObject(&linodePlacementGroup)
	err := machineScope.Client.Get(ctx, objectKey, &linodePlacementGroup)
	if err != nil {
		logger.Error(err, "Failed to fetch LinodePlacementGroup")
		return -1, err
	} else if !linodePlacementGroup.Status.Ready || linodePlacementGroup.Spec.PGID == nil {
		logger.Info("LinodePlacementGroup is not ready")
		return -1, errors.New("placement group is not ready")
	}

	return *linodePlacementGroup.Spec.PGID, nil
}

func getFirewallID(ctx context.Context, machineScope *scope.MachineScope, logger logr.Logger) (int, error) {
	name := machineScope.LinodeMachine.Spec.FirewallRef.Name
	namespace := machineScope.LinodeMachine.Spec.FirewallRef.Namespace
	if namespace == "" {
		namespace = machineScope.LinodeMachine.Namespace
	}

	logger = logger.WithValues("firewallName", name, "firewallNamespace", namespace)

	linodeFirewall := &infrav1alpha2.LinodeFirewall{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	objectKey := client.ObjectKeyFromObject(linodeFirewall)
	err := machineScope.Client.Get(ctx, objectKey, linodeFirewall)
	if err != nil {
		logger.Error(err, "Failed to fetch LinodeFirewall")
		return -1, err
	}
	if linodeFirewall.Spec.FirewallID == nil {
		err = errors.New("nil firewallID")
		logger.Error(err, "Failed to fetch LinodeFirewall")
		return -1, err
	}

	return *linodeFirewall.Spec.FirewallID, nil
}

func getVlanInterfaceConfig(ctx context.Context, machineScope *scope.MachineScope, interfaces []linodego.InstanceConfigInterfaceCreateOptions, logger logr.Logger) (*linodego.InstanceConfigInterfaceCreateOptions, error) {
	logger = logger.WithValues("vlanName", machineScope.Cluster.Name)

	// Try to obtain a IP for the machine using its name
	ip, err := util.GetNextVlanIP(ctx, machineScope.Cluster.Name, machineScope.Cluster.Namespace, machineScope.Client)
	if err != nil {
		return nil, fmt.Errorf("getting vlanIP: %w", err)
	}

	logger.Info("obtained IP for machine", "name", machineScope.LinodeMachine.Name, "ip", ip)

	for i, netInterface := range interfaces {
		if netInterface.Purpose == linodego.InterfacePurposeVLAN {
			interfaces[i].IPAMAddress = fmt.Sprintf(vlanIPFormat, ip)
			return nil, nil //nolint:nilnil // it is important we don't return an interface if a VLAN interface already exists
		}
	}

	return &linodego.InstanceConfigInterfaceCreateOptions{
		Purpose:     linodego.InterfacePurposeVLAN,
		Label:       machineScope.Cluster.Name,
		IPAMAddress: fmt.Sprintf(vlanIPFormat, ip),
	}, nil
}

// getVPCInterfaceConfig returns the interface configuration for a VPC based on the provided VPC reference
func getVPCInterfaceConfig(ctx context.Context, machineScope *scope.MachineScope, interfaces []linodego.InstanceConfigInterfaceCreateOptions, logger logr.Logger, vpcRef *corev1.ObjectReference) (*linodego.InstanceConfigInterfaceCreateOptions, error) {
	// Get namespace from VPC ref or default to machine namespace
	namespace := vpcRef.Namespace
	if namespace == "" {
		namespace = machineScope.LinodeMachine.Namespace
	}

	name := vpcRef.Name

	logger = logger.WithValues("vpcName", name, "vpcNamespace", namespace)

	linodeVPC := infrav1alpha2.LinodeVPC{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	if err := machineScope.Client.Get(ctx, client.ObjectKeyFromObject(&linodeVPC), &linodeVPC); err != nil {
		logger.Error(err, "Failed to fetch LinodeVPC")

		return nil, err
	} else if !linodeVPC.Status.Ready || linodeVPC.Spec.VPCID == nil {
		logger.Info("LinodeVPC is not available")

		return nil, errors.New("vpc is not available")
	}

	if len(linodeVPC.Spec.Subnets) == 0 {
		logger.Error(nil, "Failed to find subnet")

		return nil, errors.New("failed to find subnet")
	}

	var subnetID int

	subnetName := machineScope.LinodeCluster.Spec.Network.SubnetName // name of subnet to use

	if subnetName != "" {
		for _, subnet := range linodeVPC.Spec.Subnets {
			if subnet.Label == subnetName {
				subnetID = subnet.SubnetID
			}
		}

		if subnetID == 0 {
			logger.Info("Failed to fetch subnet ID for specified subnet name")
		}
	} else {
		subnetID = linodeVPC.Spec.Subnets[0].SubnetID // get first subnet if nothing specified
	}

	if subnetID == 0 {
		return nil, errors.New("failed to find subnet as subnet id set is 0")
	}

	for i, netInterface := range interfaces {
		if netInterface.Purpose == linodego.InterfacePurposeVPC {
			interfaces[i].SubnetID = &subnetID
			return nil, nil //nolint:nilnil // it is important we don't return an interface if a VPC interface already exists
		}
	}

	return &linodego.InstanceConfigInterfaceCreateOptions{
		Purpose:  linodego.InterfacePurposeVPC,
		Primary:  true,
		SubnetID: &subnetID,
		IPv4: &linodego.VPCIPv4{
			NAT1To1: ptr.To("any"),
		},
	}, nil
}

// getVPCInterfaceConfigFromDirectID returns the interface configuration for a VPC based on a direct VPC ID
func getVPCInterfaceConfigFromDirectID(ctx context.Context, machineScope *scope.MachineScope, interfaces []linodego.InstanceConfigInterfaceCreateOptions, logger logr.Logger, vpcID int) (*linodego.InstanceConfigInterfaceCreateOptions, error) {
	vpc, err := machineScope.LinodeClient.GetVPC(ctx, vpcID)
	if err != nil {
		logger.Error(err, "Failed to fetch VPC from Linode API", "vpcID", vpcID)
		return nil, err
	}

	if len(vpc.Subnets) == 0 {
		logger.Error(nil, "Failed to find subnet in VPC")
		return nil, errors.New("no subnets found in VPC")
	}

	var subnetID int
	var subnetName string

	// Safety check for when LinodeCluster is nil (e.g., when using direct VPCID without cluster)
	if machineScope.LinodeCluster != nil && machineScope.LinodeCluster.Spec.Network.SubnetName != "" {
		subnetName = machineScope.LinodeCluster.Spec.Network.SubnetName
	}

	// If subnet name specified, find matching subnet; otherwise use first subnet
	if subnetName != "" {
		for _, subnet := range vpc.Subnets {
			if subnet.Label == subnetName {
				subnetID = subnet.ID
				break
			}
		}
		if subnetID == 0 {
			return nil, fmt.Errorf("subnet with label %s not found in VPC", subnetName)
		}
	} else {
		subnetID = vpc.Subnets[0].ID
	}

	// Check if a VPC interface already exists
	for i, netInterface := range interfaces {
		if netInterface.Purpose == linodego.InterfacePurposeVPC {
			interfaces[i].SubnetID = &subnetID
			return nil, nil //nolint:nilnil // it is important we don't return an interface if a VPC interface already exists
		}
	}

	// Create a new VPC interface
	return &linodego.InstanceConfigInterfaceCreateOptions{
		Purpose:  linodego.InterfacePurposeVPC,
		Primary:  true,
		SubnetID: &subnetID,
		IPv4: &linodego.VPCIPv4{
			NAT1To1: ptr.To("any"),
		},
	}, nil
}

func linodeMachineSpecToInstanceCreateConfig(machineScope *scope.MachineScope, machineTags []string) *linodego.InstanceCreateOptions {
	machineSpec := machineScope.LinodeMachine.Spec
	interfaces := make([]linodego.InstanceConfigInterfaceCreateOptions, len(machineSpec.Interfaces))
	for idx, iface := range machineSpec.Interfaces {
		interfaces[idx] = linodego.InstanceConfigInterfaceCreateOptions{
			IPAMAddress: iface.IPAMAddress,
			Label:       iface.Label,
			Purpose:     iface.Purpose,
			Primary:     iface.Primary,
			SubnetID:    iface.SubnetID,
			IPRanges:    iface.IPRanges,
		}
	}
	privateIP := false
	if machineSpec.PrivateIP != nil {
		privateIP = *machineSpec.PrivateIP
	}
	return &linodego.InstanceCreateOptions{
		Label:           getDesiredLinodeInstanceLabel(machineScope),
		Region:          machineSpec.Region,
		Type:            machineSpec.Type,
		AuthorizedKeys:  machineSpec.AuthorizedKeys,
		AuthorizedUsers: machineSpec.AuthorizedUsers,
		RootPass:        machineSpec.RootPass,
		Image:           machineSpec.Image,
		Interfaces:      interfaces,
		PrivateIP:       privateIP,
		Tags:            machineTags,
		FirewallID:      machineSpec.FirewallID,
		DiskEncryption:  linodego.InstanceDiskEncryption(machineSpec.DiskEncryption),
	}
}

func compressUserData(bootstrapData []byte) ([]byte, error) {
	var userDataBuff bytes.Buffer
	var err error
	gz := gzip.NewWriter(&userDataBuff)
	defer func(gz *gzip.Writer) {
		err = gz.Close()
	}(gz)
	if _, err := gz.Write(bootstrapData); err != nil {
		return nil, err
	}
	err = gz.Close()
	return userDataBuff.Bytes(), err
}

func setUserData(ctx context.Context, machineScope *scope.MachineScope, createConfig *linodego.InstanceCreateOptions, gzipCompressionEnabled bool, logger logr.Logger) error {
	bootstrapData, err := resolveBootstrapData(ctx, machineScope, gzipCompressionEnabled, logger)
	if err != nil {
		return err
	}

	createConfig.Metadata = &linodego.InstanceMetadataOptions{
		UserData: b64.StdEncoding.EncodeToString(bootstrapData),
	}

	return nil
}

func resolveBootstrapData(ctx context.Context, machineScope *scope.MachineScope, gzipCompressionEnabled bool, logger logr.Logger) ([]byte, error) {
	bootstrapdata, err := machineScope.GetBootstrapData(ctx)
	if err != nil {
		return nil, err
	}

	var (
		size       = len(bootstrapdata)
		compressed []byte
		limit      int
	)

	// Determine limits for delivery service
	limit = maxBootstrapDataBytesCloudInit

	// Determine the delivery mechanism for the bootstrap data based on limits. This informs the formatting of the
	// bootstrap data.
	switch {
	// Best case: Deliver data directly.
	case size < limit:
		return bootstrapdata, nil
	// Compromise case (Metadata): Use compression.
	case gzipCompressionEnabled:
		if compressed, err = compressUserData(bootstrapdata); err != nil {
			// Break and use the Cluster Object Store workaround on compression failure.
			logger.Info(fmt.Sprintf("Failed to compress bootstrap data: %v. Using Cluster Object Store instead.", err))
			break
		}

		size = len(compressed)
		if len(compressed) < limit {
			return compressed, nil
		}
	}

	// Worst case: Upload to Cluster Object Store.
	logger.Info("decoded bootstrap data exceeds size limit", "limit", limit, "size", size)

	if machineScope.LinodeCluster.Spec.ObjectStore == nil {
		return nil, errors.New("must enable cluster object store feature to bootstrap linodemachine")
	}

	logger.Info("Uploading bootstrap data the Cluster Object Store")

	// Upload the original bootstrap data.
	url, err := services.CreateObject(ctx, machineScope, bootstrapdata)
	if err != nil {
		return nil, fmt.Errorf("upload bootstrap data: %w", err)
	}

	// Format a "pointer" cloud-config.
	tmpl, err := template.New(string(machineScope.LinodeMachine.UID)).Parse(cloudConfigTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse cloud-config template: %w", err)
	}
	var config bytes.Buffer
	if err := tmpl.Execute(&config, []string{url}); err != nil {
		return nil, fmt.Errorf("execute cloud-config template: %w", err)
	}

	return config.Bytes(), err
}

// This *may* need to revisit w.r.t. rate-limits for shared(?) buckets 🤷‍♀️
func deleteBootstrapData(ctx context.Context, machineScope *scope.MachineScope) error {
	if machineScope.LinodeCluster.Spec.ObjectStore != nil {
		return services.DeleteObject(ctx, machineScope)
	}
	return nil
}

func createInstanceConfigDeviceMap(instanceDisks map[string]*infrav1alpha2.InstanceDisk, instanceConfig *linodego.InstanceConfigDeviceMap) error {
	for deviceName, disk := range instanceDisks {
		dev := linodego.InstanceConfigDevice{
			DiskID: disk.DiskID,
		}
		switch deviceName {
		case "sdb":
			instanceConfig.SDB = &dev
		case "sdc":
			instanceConfig.SDC = &dev
		case "sdd":
			instanceConfig.SDD = &dev
		case "sde":
			instanceConfig.SDE = &dev
		case "sdf":
			instanceConfig.SDF = &dev
		case "sdg":
			instanceConfig.SDG = &dev
		case "sdh":
			instanceConfig.SDH = &dev
		default:
			return fmt.Errorf("unknown device name: %q", deviceName)
		}
	}

	return nil
}

func configureDisks(ctx context.Context, logger logr.Logger, machineScope *scope.MachineScope, linodeInstanceID int) error {
	if machineScope.LinodeMachine.Spec.DataDisks == nil && machineScope.LinodeMachine.Spec.OSDisk == nil {
		return nil
	}

	if err := resizeRootDisk(ctx, logger, machineScope, linodeInstanceID); err != nil {
		return err
	}
	if !reconciler.ConditionTrue(machineScope.LinodeMachine, ConditionPreflightAdditionalDisksCreated) {
		if err := createDisks(ctx, logger, machineScope, linodeInstanceID); err != nil {
			return err
		}
	}
	return nil
}

func createDisks(ctx context.Context, logger logr.Logger, machineScope *scope.MachineScope, linodeInstanceID int) error {
	for deviceName, disk := range machineScope.LinodeMachine.Spec.DataDisks {
		if disk.DiskID != 0 {
			continue
		}
		label := disk.Label
		if label == "" {
			label = deviceName
		}
		// create the disk
		diskFilesystem := defaultDiskFilesystem
		if disk.Filesystem != "" {
			diskFilesystem = disk.Filesystem
		}
		linodeDisk, err := machineScope.LinodeClient.CreateInstanceDisk(
			ctx,
			linodeInstanceID,
			linodego.InstanceDiskCreateOptions{
				Label:      label,
				Size:       int(disk.Size.ScaledValue(resource.Mega)),
				Filesystem: diskFilesystem,
			},
		)
		if err != nil {
			if !linodego.ErrHasStatus(err, linodeBusyCode) {
				logger.Error(err, "Failed to create disk", "DiskLabel", label)
			}

			conditions.Set(machineScope.LinodeMachine, metav1.Condition{
				Type:    ConditionPreflightAdditionalDisksCreated,
				Status:  metav1.ConditionFalse,
				Reason:  util.CreateError,
				Message: err.Error(),
			})
			return err
		}
		disk.DiskID = linodeDisk.ID
		machineScope.LinodeMachine.Spec.DataDisks[deviceName] = disk
	}
	err := updateInstanceConfigProfile(ctx, logger, machineScope, linodeInstanceID)
	if err != nil {
		return err
	}
	conditions.Set(machineScope.LinodeMachine, metav1.Condition{
		Type:   ConditionPreflightAdditionalDisksCreated,
		Status: metav1.ConditionTrue,
		Reason: "AdditionalDisksCreated",
	})
	return nil
}

func resizeRootDisk(ctx context.Context, logger logr.Logger, machineScope *scope.MachineScope, linodeInstanceID int) error {
	if reconciler.ConditionTrue(machineScope.LinodeMachine, ConditionPreflightRootDiskResized) {
		return nil
	}

	instanceConfig, err := getDefaultInstanceConfig(ctx, machineScope, linodeInstanceID)
	if err != nil {
		logger.Error(err, "Failed to get default instance configuration")

		conditions.Set(machineScope.LinodeMachine, metav1.Condition{
			Type:    ConditionPreflightRootDiskResized,
			Status:  metav1.ConditionFalse,
			Reason:  util.CreateError,
			Message: err.Error(),
		})
		return err
	}

	if instanceConfig.Devices.SDA == nil {
		conditions.Set(machineScope.LinodeMachine, metav1.Condition{
			Type:    ConditionPreflightRootDiskResized,
			Status:  metav1.ConditionFalse,
			Reason:  util.CreateError,
			Message: "root disk not yet ready",
		})

		return errors.New("root disk not yet ready")
	}

	rootDiskID := instanceConfig.Devices.SDA.DiskID

	// carve out space for the etcd disk
	if !reconciler.ConditionTrue(machineScope.LinodeMachine, ConditionPreflightRootDiskResizing) {
		rootDisk, err := machineScope.LinodeClient.GetInstanceDisk(ctx, linodeInstanceID, rootDiskID)
		if err != nil {
			logger.Error(err, "Failed to get root disk for instance")

			conditions.Set(machineScope.LinodeMachine, metav1.Condition{
				Type:    ConditionPreflightRootDiskResizing,
				Status:  metav1.ConditionFalse,
				Reason:  util.CreateError,
				Message: err.Error(),
			})

			return err
		}
		// dynamically calculate root disk size unless an explicit OS disk is being set
		additionalDiskSize := 0
		for _, disk := range machineScope.LinodeMachine.Spec.DataDisks {
			additionalDiskSize += int(disk.Size.ScaledValue(resource.Mega))
		}
		diskSize := rootDisk.Size - additionalDiskSize
		if machineScope.LinodeMachine.Spec.OSDisk != nil {
			diskSize = int(machineScope.LinodeMachine.Spec.OSDisk.Size.ScaledValue(resource.Mega))
		}

		if err := machineScope.LinodeClient.ResizeInstanceDisk(ctx, linodeInstanceID, rootDiskID, diskSize); err != nil {
			conditions.Set(machineScope.LinodeMachine, metav1.Condition{
				Type:    ConditionPreflightRootDiskResizing,
				Status:  metav1.ConditionFalse,
				Reason:  util.CreateError,
				Message: err.Error(),
			})
			return err
		}
		conditions.Set(machineScope.LinodeMachine, metav1.Condition{
			Type:   ConditionPreflightRootDiskResizing,
			Status: metav1.ConditionTrue,
			Reason: "RootDiskResizing",
		})
	}

	conditions.Delete(machineScope.LinodeMachine, ConditionPreflightRootDiskResizing)
	conditions.Set(machineScope.LinodeMachine, metav1.Condition{
		Type:   ConditionPreflightRootDiskResized,
		Status: metav1.ConditionTrue,
		Reason: "RootDiskResized",
	})

	return nil
}

func updateInstanceConfigProfile(ctx context.Context, logger logr.Logger, machineScope *scope.MachineScope, linodeInstanceID int) error {
	// get the default instance config
	configs, err := machineScope.LinodeClient.ListInstanceConfigs(ctx, linodeInstanceID, &linodego.ListOptions{})
	if err != nil || len(configs) == 0 {
		logger.Error(err, "Failed to list instance configs")

		return err
	}
	instanceConfig := configs[0]

	if machineScope.LinodeMachine.Spec.DataDisks != nil {
		if err := createInstanceConfigDeviceMap(machineScope.LinodeMachine.Spec.DataDisks, instanceConfig.Devices); err != nil {
			return err
		}
	}
	if _, err := machineScope.LinodeClient.UpdateInstanceConfig(ctx, linodeInstanceID, instanceConfig.ID, linodego.InstanceConfigUpdateOptions{Devices: instanceConfig.Devices}); err != nil {
		return err
	}

	return nil
}

func getDefaultInstanceConfig(ctx context.Context, machineScope *scope.MachineScope, linodeInstanceID int) (linodego.InstanceConfig, error) {
	configs, err := machineScope.LinodeClient.ListInstanceConfigs(ctx, linodeInstanceID, &linodego.ListOptions{})
	if err != nil || len(configs) == 0 {
		return linodego.InstanceConfig{}, fmt.Errorf("failing to list instance configurations: %w", err)
	}

	return configs[0], nil
}

// createInstance provisions linode instance after checking if the request will be within the rate-limits
// Note:
//  1. this method represents the critical section. It takes a lock before checking for the rate limits and releases it after making request to linode API or when returning from function
//  2. returned time duration here is not always used.
//     a) In case of an error, we calculate for how long to requeue in method which checks if its a transient error or not.
//     b) If POST limit is reached, only then the returned time duration is used to retry after that time has elapsed.
func createInstance(ctx context.Context, logger logr.Logger, machineScope *scope.MachineScope, createOpts *linodego.InstanceCreateOptions) (*linodego.Instance, time.Duration, error) {
	ctr := util.GetPostReqCounter(machineScope.TokenHash)
	ctr.Mu.Lock()
	defer ctr.Mu.Unlock()

	if ctr.IsPOSTLimitReached() {
		logger.Info(fmt.Sprintf("Cannot make more POST requests as rate-limit is reached. Waiting and retrying after %v seconds", ctr.RetryAfter()))
		return nil, ctr.RetryAfter(), util.ErrRateLimit
	}

	machineScope.LinodeClient.OnAfterResponse(ctr.ApiResponseRatelimitCounter)
	inst, err := machineScope.LinodeClient.CreateInstance(ctx, *createOpts)

	// if instance already exists, we get 400 response. get respective instance and return
	if linodego.ErrHasStatus(err, http.StatusBadRequest) && strings.Contains(err.Error(), "Label must be unique") {
		logger.Error(err, "Failed to create instance, received [400 BadRequest] response.")

		// check if instance already exists
		listFilter := util.Filter{Label: createOpts.Label}
		filter, errFilter := listFilter.String()
		if errFilter != nil {
			logger.Error(err, "Failed to create filter to list instance")
			return nil, ctr.RetryAfter(), err
		}
		instances, listErr := machineScope.LinodeClient.ListInstances(ctx, linodego.NewListOptions(1, filter))
		if listErr != nil {
			return nil, ctr.RetryAfter(), listErr
		}
		if len(instances) > 0 {
			return &instances[0], ctr.RetryAfter(), nil
		}
	}

	return inst, ctr.RetryAfter(), err
}

// getVPCRefFromScope returns the appropriate VPC reference based on priority:
// 1. Machine-level VPC reference
// 2. Cluster-level VPC reference
func getVPCRefFromScope(machineScope *scope.MachineScope) *corev1.ObjectReference {
	if machineScope.LinodeMachine.Spec.VPCRef != nil {
		return machineScope.LinodeMachine.Spec.VPCRef
	}
	return machineScope.LinodeCluster.Spec.VPCRef
}

// configureVlanInterface adds a VLAN interface to the configuration
func configureVlanInterface(ctx context.Context, machineScope *scope.MachineScope, createConfig *linodego.InstanceCreateOptions, logger logr.Logger) error {
	iface, err := getVlanInterfaceConfig(ctx, machineScope, createConfig.Interfaces, logger)
	if err != nil {
		logger.Error(err, "Failed to get VLAN interface config")
		return err
	}

	if iface != nil {
		// add VLAN interface as first interface
		createConfig.Interfaces = slices.Insert(createConfig.Interfaces, 0, *iface)
	}

	return nil
}

// configurePlacementGroup adds placement group configuration
func configurePlacementGroup(ctx context.Context, machineScope *scope.MachineScope, createConfig *linodego.InstanceCreateOptions, logger logr.Logger) error {
	pgID, err := getPlacementGroupID(ctx, machineScope, logger)
	if err != nil {
		logger.Error(err, "Failed to get Placement Group config")
		return err
	}

	createConfig.PlacementGroup = &linodego.InstanceCreatePlacementGroupOptions{
		ID: pgID,
	}

	return nil
}

// configureFirewall adds firewall configuration
func configureFirewall(ctx context.Context, machineScope *scope.MachineScope, createConfig *linodego.InstanceCreateOptions, logger logr.Logger) error {
	// First check if a direct FirewallID is specified
	if machineScope.LinodeMachine.Spec.FirewallID != 0 {
		// Direct FirewallID is provided, use it
		logger.Info("Using direct FirewallID", "firewallID", machineScope.LinodeMachine.Spec.FirewallID)
		createConfig.FirewallID = machineScope.LinodeMachine.Spec.FirewallID
		return nil
	}

	// If no direct FirewallID, use FirewallRef
	fwID, err := getFirewallID(ctx, machineScope, logger)
	if err != nil {
		logger.Error(err, "Failed to get Firewall config from reference")
		return err
	}

	createConfig.FirewallID = fwID
	return nil
}

func constructSet(arrs ...[]string) map[string]struct{} {
	strSet := make(map[string]struct{})
	for _, arr := range arrs {
		for _, elem := range arr {
			strSet[elem] = struct{}{}
		}
	}
	return strSet
}

// get tags on the linodemachine
func getTags(machineScope *scope.MachineScope, instanceTags []string) []string {
	machineTagSet := constructSet(instanceTags, machineScope.LinodeMachine.Spec.Tags, util.GetAutoGenTags(machineScope.LinodeCluster))
	desiredMachineTags := constructSet(machineScope.LinodeMachine.Spec.Tags)
	for _, tag := range machineScope.LinodeMachine.Status.Tags {
		if _, ok := desiredMachineTags[tag]; !ok {
			delete(machineTagSet, tag)
		}
	}

	outTags := []string{}
	for tag := range machineTagSet {
		outTags = append(outTags, tag)
	}

	machineScope.LinodeMachine.Status.Tags = slices.Clone(machineScope.LinodeMachine.Spec.Tags)
	return outTags
}

func areSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aSet := constructSet(a)
	bSet := constructSet(b)
	if len(aSet) != len(bSet) {
		return false
	}
	for key := range aSet {
		if _, ok := bSet[key]; !ok {
			return false
		}
	}
	return true
}

func getDesiredLinodeInstanceLabel(machineScope *scope.MachineScope) string {
	// If no label prefix is specified, use the machine name as the label
	if machineScope.LinodeMachine.Spec.LabelPrefix == "" {
		return machineScope.LinodeMachine.Name
	}

	// if machine is created by a deployment / control-plane, it's name will be prefixed with the label of linode.
	machineOwners := machineScope.Machine.GetOwnerReferences()

	// get the longest prefix match from machine owner names.
	longestPrefix := ""
	for _, owner := range machineOwners {
		if strings.HasPrefix(machineScope.LinodeMachine.Name, owner.Name) && len(owner.Name) > len(longestPrefix) {
			longestPrefix = owner.Name
		}
	}

	// If no owner name matches the prefix, use the machine name as the label
	if longestPrefix == "" {
		// If no owner name matches the prefix, use the label prefix
		return machineScope.LinodeMachine.Spec.LabelPrefix + "-" + machineScope.LinodeMachine.Name
	} else {
		return strings.Replace(machineScope.LinodeMachine.Name, longestPrefix, machineScope.LinodeMachine.Spec.LabelPrefix, 1)
	}
}
