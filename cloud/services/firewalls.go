package services

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
)

const (
	maxFirewallRuleLabelLen = 32
	maxIPsPerFirewallRule   = 255
	maxRulesPerFirewall     = 25
)

var (
	errTooManyIPs         = errors.New("too many IPs in this ACL, will exceed rules per firewall limit")
	errDuplicateFirewalls = errors.New("duplicate firewalls found")
)

// HandleFirewall takes the CAPL firewall representation and uses it to either create or update the Cloud Firewall
// via the given linode client
func HandleFirewall(
	ctx context.Context,
	firewall *infrav1alpha1.LinodeFirewall,
	linodeClient *linodego.Client,
	logger logr.Logger,
) (linodeFW *linodego.Firewall, err error) {
	clusterUID := firewall.Spec.ClusterUID
	linodeFWs, err := fetchFirewalls(ctx, firewall.Name, *linodeClient)
	if err != nil {
		logger.Info("Failed to list Firewalls", "error", err.Error())

		return nil, err
	}

	// firewall conflict
	if len(linodeFWs) > 1 {
		logger.Info("Multiple firewalls found", "error", errDuplicateFirewalls.Error())

		return nil, errDuplicateFirewalls
	}

	// build out the firewall rules for create or update
	fwConfig, err := processACL(firewall, []string{clusterUID})
	if err != nil {
		logger.Info("Failed to process ACL", "error", err.Error())

		return nil, err
	}

	if len(linodeFWs) == 0 {
		logger.Info(fmt.Sprintf("Creating firewall %s", firewall.Name))
		linodeFW, err = linodeClient.CreateFirewall(ctx, *fwConfig)
		if err != nil {
			logger.Info("Failed to create firewall", "error", err.Error())

			return nil, err
		}
		if linodeFW == nil {
			err = errors.New("nil firewall")
			logger.Error(err, "Created firewall is nil")

			return nil, err
		}
	} else {
		logger.Info(fmt.Sprintf("Updating firewall %s", firewall.Name))

		linodeFW = &linodeFWs[0]
		if err = updateFirewall(ctx, linodeClient, linodeFW, clusterUID, fwConfig); err != nil {
			logger.Info("Failed to update firewall", "error", err.Error())

			return nil, err
		}
	}

	// Need to make sure the firewall is appropriately enabled or disabled after
	// create or update and the tags are properly set
	var status linodego.FirewallStatus
	if firewall.Spec.Enabled {
		status = linodego.FirewallEnabled
	} else {
		status = linodego.FirewallDisabled
	}
	if _, err = linodeClient.UpdateFirewall(
		ctx,
		linodeFW.ID,
		linodego.FirewallUpdateOptions{
			Status: status,
			Tags:   util.Pointer([]string{clusterUID}),
		},
	); err != nil {
		logger.Info("Failed to update Linode Firewall status and tags", "error", err.Error())

		return nil, err
	}

	return linodeFW, nil
}

func updateFirewall(
	ctx context.Context,
	linodeClient *linodego.Client,
	linodeFW *linodego.Firewall,
	clusterUID string,
	fwConfig *linodego.FirewallCreateOptions,
) error {
	if !slices.Contains(linodeFW.Tags, clusterUID) {
		err := fmt.Errorf(
			"firewall %s is not associated with cluster UID %s. Owner cluster is %s",
			linodeFW.Label,
			clusterUID,
			linodeFW.Tags[0],
		)

		return err
	}

	if _, err := linodeClient.UpdateFirewallRules(ctx, linodeFW.ID, fwConfig.Rules); err != nil {
		return err
	}

	return nil
}

// fetch Firewalls returns all Linode firewalls with a label matching the CAPL Firewall name
func fetchFirewalls(
	ctx context.Context,
	name string,
	linodeClient linodego.Client,
) (firewalls []linodego.Firewall, err error) {
	var linodeFWs []linodego.Firewall
	if linodeFWs, err = linodeClient.ListFirewalls(
		ctx,
		linodego.NewListOptions(
			1,
			util.CreateLinodeAPIFilter(name, []string{}),
		),
	); err != nil {
		return nil, err
	}

	return linodeFWs, nil
}

// chunkIPs takes a list of strings representing IPs and breaks them up into
// one or more lists capped at the maxIPsPerFirewallRule for length
func chunkIPs(ips []string) [][]string {
	chunks := [][]string{}
	ipCount := len(ips)

	// If the number of IPs is less than or equal to maxIPsPerFirewall,
	// return a single chunk containing all IPs.
	if ipCount <= maxIPsPerFirewallRule {
		return [][]string{ips}
	}

	// Otherwise, break the IPs into chunks with maxIPsPerFirewall IPs per chunk.
	chunkCount := 0
	for ipCount > maxIPsPerFirewallRule {
		start := chunkCount * maxIPsPerFirewallRule
		end := (chunkCount + 1) * maxIPsPerFirewallRule
		chunks = append(chunks, ips[start:end])
		chunkCount++
		ipCount -= maxIPsPerFirewallRule
	}

	// Append the remaining IPs as a chunk.
	chunks = append(chunks, ips[chunkCount*maxIPsPerFirewallRule:])

	return chunks
}

// processACL uses the CAPL LinodeFirewall representation to build out the inbound
// and outbound rules for a linode Cloud Firewall and returns the configuration
// for creating or updating the Firewall
//
//nolint:gocyclo,cyclop // As simple as possible.
func processACL(firewall *infrav1alpha1.LinodeFirewall, tags []string) (
	*linodego.FirewallCreateOptions,
	error,
) {
	createOpts := &linodego.FirewallCreateOptions{
		Label: firewall.Name,
		Tags:  tags,
	}

	// process inbound rules
	for _, rule := range firewall.Spec.InboundRules {
		ruleIPv4s := []string{}
		ruleIPv6s := []string{}

		if rule.Addresses.IPv4 != nil {
			ruleIPv4s = append(ruleIPv4s, *rule.Addresses.IPv4...)
		}

		if rule.Addresses.IPv6 != nil {
			ruleIPv6s = append(ruleIPv6s, *rule.Addresses.IPv6...)
		}

		ruleLabel := fmt.Sprintf("%s-%s", rule.Action, rule.Label)
		if len(ruleLabel) > maxFirewallRuleLabelLen {
			ruleLabel = ruleLabel[0:maxFirewallRuleLabelLen]
		}

		// Process IPv4
		// chunk IPs to be in 255 chunks or fewer
		ipv4chunks := chunkIPs(ruleIPv4s)
		for i, chunk := range ipv4chunks {
			v4chunk := chunk
			createOpts.Rules.Inbound = append(createOpts.Rules.Inbound, linodego.FirewallRule{
				Action:      rule.Action,
				Label:       ruleLabel,
				Description: fmt.Sprintf("Rule %d, Created by CAPL: %s", i, rule.Label),
				Protocol:    linodego.TCP,
				Ports:       rule.Ports,
				Addresses:   linodego.NetworkAddresses{IPv4: &v4chunk},
			})
		}

		// Process IPv6
		// chunk IPs to be in 255 chunks or fewer
		ipv6chunks := chunkIPs(ruleIPv6s)
		for i, chunk := range ipv6chunks {
			v6chunk := chunk
			createOpts.Rules.Inbound = append(createOpts.Rules.Inbound, linodego.FirewallRule{
				Action:      rule.Action,
				Label:       ruleLabel,
				Description: fmt.Sprintf("Rule %d, Created by CAPL: %s", i, rule.Label),
				Protocol:    linodego.TCP,
				Ports:       rule.Ports,
				Addresses:   linodego.NetworkAddresses{IPv6: &v6chunk},
			})
		}
	}
	if firewall.Spec.InboundPolicy == "ACCEPT" {
		// if an allow list is present, we drop everything else.
		createOpts.Rules.InboundPolicy = "DROP"
	} else {
		// if a deny list is present, we accept everything else.
		createOpts.Rules.InboundPolicy = "ACCEPT"
	}

	// process outbound rules
	for _, rule := range firewall.Spec.OutboundRules {
		ruleIPv4s := []string{}
		ruleIPv6s := []string{}

		if rule.Addresses.IPv4 != nil {
			ruleIPv4s = append(ruleIPv4s, *rule.Addresses.IPv4...)
		}

		if rule.Addresses.IPv6 != nil {
			ruleIPv6s = append(ruleIPv6s, *rule.Addresses.IPv6...)
		}

		ruleLabel := fmt.Sprintf("%s-%s", firewall.Spec.OutboundPolicy, rule.Label)
		if len(ruleLabel) > maxFirewallRuleLabelLen {
			ruleLabel = ruleLabel[0:maxFirewallRuleLabelLen]
		}

		// Process IPv4
		// chunk IPs to be in 255 chunks or fewer
		ipv4chunks := chunkIPs(ruleIPv4s)
		for i, chunk := range ipv4chunks {
			v4chunk := chunk
			createOpts.Rules.Outbound = append(createOpts.Rules.Outbound, linodego.FirewallRule{
				Action:      rule.Action,
				Label:       ruleLabel,
				Description: fmt.Sprintf("Rule %d, Created by CAPL: %s", i, rule.Label),
				Protocol:    linodego.TCP,
				Ports:       rule.Ports,
				Addresses:   linodego.NetworkAddresses{IPv4: &v4chunk},
			})
		}

		// Process IPv6
		// chunk IPs to be in 255 chunks or fewer
		ipv6chunks := chunkIPs(ruleIPv6s)
		for i, chunk := range ipv6chunks {
			v6chunk := chunk
			createOpts.Rules.Outbound = append(createOpts.Rules.Outbound, linodego.FirewallRule{
				Action:      rule.Action,
				Label:       ruleLabel,
				Description: fmt.Sprintf("Rule %d, Created by CAPL: %s", i, rule.Label),
				Protocol:    linodego.TCP,
				Ports:       rule.Ports,
				Addresses:   linodego.NetworkAddresses{IPv6: &v6chunk},
			})
		}
	}
	if firewall.Spec.OutboundPolicy == "ACCEPT" {
		// if an allow list is present, we drop everything else.
		createOpts.Rules.OutboundPolicy = "DROP"
	} else {
		// if a deny list is present, we accept everything else.
		createOpts.Rules.OutboundPolicy = "ACCEPT"
	}

	// need to check if we ended up needing to make too many rules
	// with IP chunking
	if len(createOpts.Rules.Inbound)+len(createOpts.Rules.Outbound) > maxRulesPerFirewall {
		return nil, errTooManyIPs
	}

	return createOpts, nil
}

// AddNodeToApiServerFW adds a Node's IPs to the given Cloud Firewall's inbound rules
func AddNodeToApiServerFW(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
	firewall *infrav1alpha1.LinodeFirewall,
) error {
	if firewall.Spec.FirewallID == nil {
		err := errors.New("no firewall ID")
		logger.Error(err, "no ID is set for the firewall")

		return err
	}

	ipv4s, ipv6s, err := getInstanceIPs(ctx, machineScope.LinodeClient, machineScope.LinodeMachine.Spec.InstanceID)
	if err != nil {
		logger.Error(err, "Failed get instance IP addresses")

		return err
	}

	// get the rules and append a new rule for this Node to access the api server
	newRule := infrav1alpha1.FirewallRule{
		Action:      "ACCEPT",
		Label:       "api-server",
		Description: "Rule created by CAPL",
		Ports:       fmt.Sprint(machineScope.LinodeCluster.Spec.ControlPlaneEndpoint.Port),
		Protocol:    linodego.TCP,
		Addresses: &infrav1alpha1.NetworkAddresses{
			IPv4: util.Pointer(ipv4s),
			IPv6: util.Pointer(ipv6s),
		},
	}
	// update the inbound rules
	firewall.Spec.InboundRules = append(firewall.Spec.InboundRules, newRule)

	// reprocess the firewall to make sure we won't exceed the IP and rule limit
	clusterUID := firewall.Spec.ClusterUID
	fwConfig, err := processACL(firewall, []string{clusterUID})
	if err != nil {
		logger.Info("Failed to process ACL", "error", err.Error())

		return err
	}

	// finally, update the firewall
	if _, err := machineScope.LinodeClient.UpdateFirewallRules(ctx, *firewall.Spec.FirewallID, fwConfig.Rules); err != nil {
		logger.Info("Failed to update firewall", "error", err.Error())

		return err
	}

	return nil
}

// DeleteNodeFromApiServerFW removes Node from the given Cloud Firewall's inbound rules
func DeleteNodeFromApiServerFW(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
	firewall *infrav1alpha1.LinodeFirewall,
) error {
	if firewall.Spec.FirewallID == nil {
		logger.Info("Firewall already deleted, no Firewall address to remove")

		return nil
	}

	if machineScope.LinodeMachine.Spec.InstanceID == nil {
		return errors.New("no InstanceID")
	}

	ipv4s, ipv6s, err := getInstanceIPs(ctx, machineScope.LinodeClient, machineScope.LinodeMachine.Spec.InstanceID)
	if err != nil {
		logger.Error(err, "Failed get instance IP addresses")

		return err
	}

	for _, rule := range firewall.Spec.InboundRules {
		rule.Addresses.IPv4 = util.Pointer(setDiff(*rule.Addresses.IPv4, ipv4s))
		rule.Addresses.IPv6 = util.Pointer(setDiff(*rule.Addresses.IPv6, ipv6s))
	}

	// reprocess the firewall
	clusterUID := firewall.Spec.ClusterUID
	fwConfig, err := processACL(firewall, []string{clusterUID})
	if err != nil {
		logger.Info("Failed to process ACL", "error", err.Error())

		return err
	}

	// finally, update the firewall
	if _, err := machineScope.LinodeClient.UpdateFirewallRules(ctx, *firewall.Spec.FirewallID, fwConfig.Rules); err != nil {
		logger.Info("Failed to update firewall", "error", err.Error())

		return err
	}

	return nil
}

func getInstanceIPs(ctx context.Context, client *linodego.Client, instanceID *int) (ipv4s, ipv6s []string, err error) {
	addresses, err := client.GetInstanceIPAddresses(ctx, *instanceID)
	if err != nil {
		return ipv4s, ipv6s, err
	}

	// get all the ipv4 addresses for the node
	for _, addr := range addresses.IPv4.Private {
		ipv4s = append(ipv4s, addr.Address)
	}
	for _, addr := range addresses.IPv4.Public {
		ipv4s = append(ipv4s, addr.Address)
	}

	// get all the ipv6 addresses for the node
	ipv6s = []string{addresses.IPv6.SLAAC.Address, addresses.IPv6.LinkLocal.Address}

	return ipv4s, ipv6s, nil
}

// setDiff: A - B
func setDiff(a, b []string) (diff []string) {
	m := make(map[string]bool)
	for _, item := range b {
		m[item] = true
	}
	for _, item := range a {
		if _, ok := m[item]; !ok {
			diff = append(diff, item)
		}
	}

	return diff
}
