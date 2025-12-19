package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/dns"
	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	"golang.org/x/exp/slices"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	kutil "sigs.k8s.io/cluster-api/util"

	"github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/clients"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	rutil "github.com/linode/cluster-api-provider-linode/util/reconciler"
)

type DNSEntries struct {
	options []DNSOptions
	mux     sync.RWMutex
}

type DNSOptions struct {
	Hostname      string
	Target        string
	DNSRecordType linodego.DomainRecordType
	DNSTTLSec     int
}

// EnsureDNSEntries ensures the domainrecord on Linode Cloud Manager is created, updated, or deleted based on operation passed
func EnsureDNSEntries(ctx context.Context, cscope *scope.ClusterScope, operation string) error {
	// Get the public IP that was assigned
	var dnss DNSEntries
	dnsEntries, err := dnss.getDNSEntriesToEnsure(ctx, cscope)
	if err != nil {
		return err
	}

	if len(dnsEntries) == 0 {
		return nil
	}

	if cscope.LinodeCluster.Spec.Network.DNSProvider == "akamai" {
		if err := deleteStaleAkamaiEntries(ctx, cscope); err != nil {
			return err
		}
		for _, dnsEntry := range dnsEntries {
			if err := EnsureAkamaiDNSEntries(ctx, cscope, operation, dnsEntry); err != nil {
				return err
			}
		}
	} else {
		if err := EnsureLinodeDNSEntries(ctx, cscope, operation, dnsEntries); err != nil {
			return err
		}
	}

	return nil
}

func getMachineIPs(cscope *scope.ClusterScope) (ipv4IPs, ipv6IPs []string, err error) {
	for _, eachMachine := range cscope.LinodeMachines.Items {
		if !eachMachine.Status.Ready {
			continue
		}
		for _, IPs := range eachMachine.Status.Addresses {
			if IPs.Type != v1beta1.MachineExternalIP {
				continue
			}
			addr, err := netip.ParseAddr(IPs.Address)
			if err != nil {
				return nil, nil, fmt.Errorf("not a valid IP %w", err)
			}
			if addr.Is4() {
				ipv4IPs = append(ipv4IPs, IPs.Address)
			} else {
				ipv6IPs = append(ipv6IPs, IPs.Address)
			}
		}
	}
	return ipv4IPs, ipv6IPs, nil
}

func resetAkamaiRecord(ctx context.Context, cscope *scope.ClusterScope, recordBody *dns.RecordBody, machineIPList []string, rootDomain string) error {
	freshEntries := make([]string, 0)
	for _, ip := range recordBody.Target {
		ip = strings.Replace(ip, ":0:0:", "::", 8) //nolint:mnd // 8 for 8 octet
		if slices.Contains(machineIPList, ip) {
			freshEntries = append(freshEntries, ip)
		}
	}
	if len(freshEntries) == 0 {
		return cscope.AkamaiDomainsClient.DeleteRecord(ctx, recordBody, rootDomain)
	} else {
		recordBody.Target = freshEntries
		return cscope.AkamaiDomainsClient.UpdateRecord(ctx, recordBody, rootDomain)
	}
}

func deleteStaleAkamaiEntries(ctx context.Context, cscope *scope.ClusterScope) error {
	ipv4IPs, ipv6IPs, err := getMachineIPs(cscope)
	if err != nil {
		return err
	}

	rootDomain := cscope.LinodeCluster.Spec.Network.DNSRootDomain
	fqdn := getSubDomain(cscope) + "." + rootDomain

	// A record
	aRecordBody, err := cscope.AkamaiDomainsClient.GetRecord(ctx, rootDomain, fqdn, "A")
	if err != nil {
		if !strings.Contains(err.Error(), "Not Found") {
			return err
		}
	}
	if aRecordBody != nil {
		if err := resetAkamaiRecord(ctx, cscope, aRecordBody, ipv4IPs, rootDomain); err != nil {
			return err
		}
	}

	// AAAA record
	aaaaRecordBody, err := cscope.AkamaiDomainsClient.GetRecord(ctx, rootDomain, fqdn, "AAAA")
	if err != nil {
		if !strings.Contains(err.Error(), "Not Found") {
			return err
		}
	}
	if aaaaRecordBody != nil {
		if err := resetAkamaiRecord(ctx, cscope, aaaaRecordBody, ipv6IPs, rootDomain); err != nil {
			return err
		}
	}

	return nil
}

func deleteStaleLinodeEntries(ctx context.Context, cscope *scope.ClusterScope, domainRecords []linodego.DomainRecord, domainID int) error {
	ipv4IPs, ipv6IPs, err := getMachineIPs(cscope)
	if err != nil {
		return err
	}

	if len(domainRecords) > 0 {
		for _, record := range domainRecords {
			if record.Type == linodego.RecordTypeTXT {
				continue
			}
			if !slices.Contains(ipv4IPs, record.Target) && !slices.Contains(ipv6IPs, record.Target) {
				if err := cscope.LinodeDomainsClient.DeleteDomainRecord(ctx, domainID, record.ID); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// EnsureLinodeDNSEntries ensures the domainrecord on Linode Cloud Manager is created, updated, or deleted based on operation passed
func EnsureLinodeDNSEntries(ctx context.Context, cscope *scope.ClusterScope, operation string, dnsEntries []DNSOptions) error {
	// Get domainID from domain name
	domainID, err := GetDomainID(ctx, cscope)
	if err != nil {
		return err
	}

	filter, err := json.Marshal(map[string]interface{}{"name": getSubDomain(cscope)})
	if err != nil {
		return err
	}

	listOptions := linodego.NewListOptions(0, string(filter))
	listOptions.PageSize = 500 // set a high page size to avoid multiple requests

	domainRecords, err := cscope.LinodeDomainsClient.ListDomainRecords(ctx, domainID, listOptions)
	if err != nil {
		return err
	}

	if err := deleteStaleLinodeEntries(ctx, cscope, domainRecords, domainID); err != nil {
		return err
	}

	for _, dnsEntry := range dnsEntries {
		if operation == "delete" {
			if err := DeleteDomainRecord(ctx, cscope, domainID, dnsEntry); err != nil {
				return err
			}
		} else {
			if err := CreateDomainRecord(ctx, cscope, domainID, dnsEntry); err != nil {
				return err
			}
		}
	}
	return nil
}

// EnsureAkamaiDNSEntries ensures the domainrecord on Akamai EDGE DNS is created, updated, or deleted based on operation passed
func EnsureAkamaiDNSEntries(ctx context.Context, cscope *scope.ClusterScope, operation string, dnsEntry DNSOptions) error {
	linodeCluster := cscope.LinodeCluster
	linodeClusterNetworkSpec := linodeCluster.Spec.Network
	rootDomain := linodeClusterNetworkSpec.DNSRootDomain
	akaDNSClient := cscope.AkamaiDomainsClient
	fqdn := getSubDomain(cscope) + "." + rootDomain

	// Get the record for the root domain and fqdn
	recordBody, err := akaDNSClient.GetRecord(ctx, rootDomain, fqdn, string(dnsEntry.DNSRecordType))

	if err != nil {
		if !strings.Contains(err.Error(), "Not Found") {
			return err
		}
		// Record was not found - if operation is not "create", nothing to do
		if operation != "create" {
			return nil
		}
		// Create record
		return createAkamaiEntry(ctx, akaDNSClient, dnsEntry, fqdn, rootDomain)
	}
	if recordBody == nil {
		return fmt.Errorf("akamai dns returned empty dns record")
	}

	// if operation is delete and we got the record, delete it
	if operation == "delete" {
		return deleteAkamaiEntry(ctx, cscope, recordBody, dnsEntry)
	}
	// if operation is create and we got the record, update it
	// Check if the target already exists in the target list
	for _, target := range recordBody.Target {
		if recordBody.RecordType == "TXT" {
			if strings.Contains(target, dnsEntry.Target) {
				return nil
			}
		} else {
			if slices.Equal(net.ParseIP(target), net.ParseIP(dnsEntry.Target)) {
				return nil
			}
		}
	}
	// Target doesn't exist so lets append it to the existing list and update it
	recordBody.Target = append(recordBody.Target, dnsEntry.Target)
	return akaDNSClient.UpdateRecord(ctx, recordBody, rootDomain)
}

func createAkamaiEntry(ctx context.Context, client clients.AkamClient, dnsEntry DNSOptions, fqdn, rootDomain string) error {
	return client.CreateRecord(
		ctx,
		&dns.RecordBody{
			Name:       fqdn,
			RecordType: string(dnsEntry.DNSRecordType),
			TTL:        dnsEntry.DNSTTLSec,
			Target:     []string{dnsEntry.Target},
		},
		rootDomain,
	)
}

func deleteAkamaiEntry(ctx context.Context, cscope *scope.ClusterScope, recordBody *dns.RecordBody, dnsEntry DNSOptions) error {
	linodeCluster := cscope.LinodeCluster
	linodeClusterNetworkSpec := linodeCluster.Spec.Network
	rootDomain := linodeClusterNetworkSpec.DNSRootDomain
	// If record is A/AAAA type, verify ownership
	if dnsEntry.DNSRecordType != linodego.RecordTypeTXT {
		isOwner, err := IsAkamaiDomainRecordOwner(ctx, cscope)
		if err != nil {
			return err
		}
		if !isOwner {
			return fmt.Errorf("the domain record is not owned by this entity. wont delete")
		}
	}
	switch {
	case len(recordBody.Target) > 1:
		recordBody.Target = removeElement(
			recordBody.Target,
			// Linode DNS API formats the IPv6 IPs using :: for :0:0: while the address from the LinodeMachine status keeps it as is
			// So we need to match that
			strings.Replace(dnsEntry.Target, "::", ":0:0:", 8), //nolint:mnd // 8 for 8 octest
		)
		return cscope.AkamaiDomainsClient.UpdateRecord(ctx, recordBody, rootDomain)
	default:
		return cscope.AkamaiDomainsClient.DeleteRecord(ctx, recordBody, rootDomain)
	}
}

func removeElement(stringList []string, elemToRemove string) []string {
	for index, element := range stringList {
		if element == elemToRemove {
			stringList = slices.Delete(stringList, index, index+1)
			continue
		}
	}
	return stringList
}

func isCapiMachineReady(capiMachine *v1beta1.Machine) bool {
	if capiMachine.Status.V1Beta2 == nil {
		return false
	}
	for _, condition := range capiMachine.Status.V1Beta2.Conditions {
		if condition.Type == v1beta1.ReadyV1Beta2Condition && condition.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

func processLinodeMachine(ctx context.Context, cscope *scope.ClusterScope, machine v1alpha2.LinodeMachine, dnsTTLSec int, subdomain string, firstMachine bool) ([]DNSOptions, error) {
	// Look up the corresponding CAPI machine, see if it is marked for deletion
	capiMachine, err := kutil.GetOwnerMachine(ctx, cscope.Client, machine.ObjectMeta)
	if err != nil {
		return nil, fmt.Errorf("failed to get CAPI machine for LinodeMachine %s: %w", machine.Name, err)
	}

	if capiMachine == nil || capiMachine.DeletionTimestamp != nil {
		// If the CAPI machine is deleted, we don't need to create DNS entries for it.
		return nil, nil
	}

	if !firstMachine && !isCapiMachineReady(capiMachine) {
		// always process the first linodeMachine, and add its IP to the DNS entries.
		// For other linodeMachine, only process them if the CAPI machine is ready
		logger := logr.FromContextOrDiscard(ctx)
		logger.Info("skipping DNS entry creation for LinodeMachine as the CAPI machine is not ready", "LinodeMachine", machine.Name)
		return nil, nil
	}

	options := []DNSOptions{}
	for _, IPs := range machine.Status.Addresses {
		recordType := linodego.RecordTypeA
		if IPs.Type != v1beta1.MachineExternalIP {
			continue
		}
		addr, err := netip.ParseAddr(IPs.Address)
		if err != nil {
			return nil, fmt.Errorf("not a valid IP %w", err)
		}
		if !addr.Is4() {
			recordType = linodego.RecordTypeAAAA
		}
		options = append(options, DNSOptions{subdomain, IPs.Address, recordType, dnsTTLSec})
	}
	return options, nil
}

// getDNSEntriesToEnsure return DNS entries to create/delete
func (d *DNSEntries) getDNSEntriesToEnsure(ctx context.Context, cscope *scope.ClusterScope) ([]DNSOptions, error) {
	d.mux.Lock()
	defer d.mux.Unlock()
	dnsTTLSec := rutil.DefaultDNSTTLSec
	if cscope.LinodeCluster.Spec.Network.DNSTTLSec != 0 {
		dnsTTLSec = cscope.LinodeCluster.Spec.Network.DNSTTLSec
	}

	subDomain := getSubDomain(cscope)
	firstMachine := true
	for _, eachMachine := range cscope.LinodeMachines.Items {
		options, err := processLinodeMachine(ctx, cscope, eachMachine, dnsTTLSec, subDomain, firstMachine)
		firstMachine = false
		if err != nil {
			return nil, fmt.Errorf("failed to process LinodeMachine %s: %w", eachMachine.Name, err)
		}
		d.options = append(d.options, options...)
	}
	d.options = append(d.options, DNSOptions{subDomain, cscope.LinodeCluster.Name, linodego.RecordTypeTXT, dnsTTLSec})

	return d.options, nil
}

// GetDomainID gets the domains linode id
func GetDomainID(ctx context.Context, cscope *scope.ClusterScope) (int, error) {
	rootDomain := cscope.LinodeCluster.Spec.Network.DNSRootDomain
	filter, err := json.Marshal(map[string]string{"domain": rootDomain})
	if err != nil {
		return 0, err
	}
	domains, err := cscope.LinodeDomainsClient.ListDomains(ctx, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		return 0, err
	}
	if len(domains) != 1 || domains[0].Domain != rootDomain {
		return 0, fmt.Errorf("domain %s not found in list of domains owned by this account", rootDomain)
	}

	return domains[0].ID, nil
}

func CreateDomainRecord(ctx context.Context, cscope *scope.ClusterScope, domainID int, dnsEntry DNSOptions) error {
	// Check if domain record exists for this IP and name combo
	filter, err := json.Marshal(map[string]interface{}{"name": dnsEntry.Hostname, "target": dnsEntry.Target, "type": dnsEntry.DNSRecordType})
	if err != nil {
		return err
	}

	domainRecords, err := cscope.LinodeDomainsClient.ListDomainRecords(ctx, domainID, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		return err
	}

	// If record doesnt exist, create it
	if len(domainRecords) == 0 {
		if _, err := cscope.LinodeDomainsClient.CreateDomainRecord(
			ctx,
			domainID,
			linodego.DomainRecordCreateOptions{
				Type:   dnsEntry.DNSRecordType,
				Name:   dnsEntry.Hostname,
				Target: dnsEntry.Target,
				TTLSec: dnsEntry.DNSTTLSec,
			},
		); err != nil {
			return err
		}
	}
	return nil
}

func DeleteDomainRecord(ctx context.Context, cscope *scope.ClusterScope, domainID int, dnsEntry DNSOptions) error {
	// Check if domain record exists for this IP and name combo
	filter, err := json.Marshal(map[string]interface{}{"name": dnsEntry.Hostname, "target": dnsEntry.Target, "type": dnsEntry.DNSRecordType})
	if err != nil {
		return err
	}

	domainRecords, err := cscope.LinodeDomainsClient.ListDomainRecords(ctx, domainID, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		return err
	}

	// Nothing to do if records dont exist
	if len(domainRecords) == 0 {
		return nil
	}

	// If record is A/AAAA type, verify ownership
	if dnsEntry.DNSRecordType != linodego.RecordTypeTXT {
		isOwner, err := IsLinodeDomainRecordOwner(ctx, cscope, dnsEntry.Hostname, domainID)
		if err != nil {
			return err
		}
		if !isOwner {
			return fmt.Errorf("the domain record is not owned by this entity. wont delete")
		}
	}

	// Delete record
	return cscope.LinodeDomainsClient.DeleteDomainRecord(ctx, domainID, domainRecords[0].ID)
}

func IsLinodeDomainRecordOwner(ctx context.Context, cscope *scope.ClusterScope, hostname string, domainID int) (bool, error) {
	// Check if domain record exists
	filter, err := json.Marshal(map[string]interface{}{"name": hostname, "type": linodego.RecordTypeTXT})
	if err != nil {
		return false, err
	}

	domainRecords, err := cscope.LinodeDomainsClient.ListDomainRecords(ctx, domainID, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		return false, err
	}

	// If record exists, update it
	if len(domainRecords) == 0 {
		return false, fmt.Errorf("no txt record %s found", hostname)
	}

	return true, nil
}

func IsAkamaiDomainRecordOwner(ctx context.Context, cscope *scope.ClusterScope) (bool, error) {
	linodeCluster := cscope.LinodeCluster
	linodeClusterNetworkSpec := linodeCluster.Spec.Network
	rootDomain := linodeClusterNetworkSpec.DNSRootDomain
	akaDNSClient := cscope.AkamaiDomainsClient
	fqdn := getSubDomain(cscope) + "." + rootDomain
	recordBody, err := akaDNSClient.GetRecord(ctx, rootDomain, fqdn, string(linodego.RecordTypeTXT))
	if err != nil || recordBody == nil {
		return false, fmt.Errorf("no txt record %s found", fqdn)
	}

	return true, nil
}

func getSubDomain(cscope *scope.ClusterScope) (subDomain string) {
	if cscope.LinodeCluster.Spec.Network.DNSSubDomainOverride != "" {
		subDomain = cscope.LinodeCluster.Spec.Network.DNSSubDomainOverride
	} else {
		uniqueID := ""
		if cscope.LinodeCluster.Spec.Network.DNSUniqueIdentifier != "" {
			uniqueID = "-" + cscope.LinodeCluster.Spec.Network.DNSUniqueIdentifier
		}
		subDomain = cscope.LinodeCluster.Name + uniqueID
	}
	return subDomain
}
