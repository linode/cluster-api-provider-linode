package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/dns"
	"github.com/linode/linodego"
	"golang.org/x/exp/slices"
	"sigs.k8s.io/cluster-api/api/v1beta1"

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
	dnsEntries, err := dnss.getDNSEntriesToEnsure(cscope)
	if err != nil {
		return err
	}

	if len(dnsEntries) == 0 {
		return errors.New("dnsEntries are empty")
	}

	if cscope.LinodeCluster.Spec.Network.DNSProvider == "akamai" {
		for _, dnsEntry := range dnsEntries {
			if err := EnsureAkamaiDNSEntries(ctx, cscope, operation, dnsEntry); err != nil {
				return err
			}
		}
	} else {
		for _, dnsEntry := range dnsEntries {
			if err := EnsureLinodeDNSEntries(ctx, cscope, operation, dnsEntry); err != nil {
				return err
			}
		}
	}

	return nil
}

// EnsureLinodeDNSEntries ensures the domainrecord on Linode Cloud Manager is created, updated, or deleted based on operation passed
func EnsureLinodeDNSEntries(ctx context.Context, cscope *scope.ClusterScope, operation string, dnsEntry DNSOptions) error {
	// Get domainID from domain name
	domainID, err := GetDomainID(ctx, cscope)
	if err != nil {
		return err
	}

	if operation == "delete" {
		return DeleteDomainRecord(ctx, cscope, domainID, dnsEntry)
	}
	return CreateDomainRecord(ctx, cscope, domainID, dnsEntry)
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
		return deleteAkamaiEntry(ctx, akaDNSClient, recordBody, dnsEntry, rootDomain)
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

func deleteAkamaiEntry(ctx context.Context, client clients.AkamClient, recordBody *dns.RecordBody, dnsEntry DNSOptions, rootDomain string) error {
	switch {
	case len(recordBody.Target) > 1:
		recordBody.Target = removeElement(
			recordBody.Target,
			// Linode DNS API formats the IPv6 IPs using :: for :0:0: while the address from the LinodeMachine status keeps it as is
			// So we need to match that
			strings.Replace(dnsEntry.Target, "::", ":0:0:", 8), //nolint:mnd // 8 for 8 octest
		)
		return client.UpdateRecord(ctx, recordBody, rootDomain)
	default:
		return client.DeleteRecord(ctx, recordBody, rootDomain)
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

// getDNSEntriesToEnsure return DNS entries to create/delete
func (d *DNSEntries) getDNSEntriesToEnsure(cscope *scope.ClusterScope) ([]DNSOptions, error) {
	d.mux.Lock()
	defer d.mux.Unlock()
	dnsTTLSec := rutil.DefaultDNSTTLSec
	if cscope.LinodeCluster.Spec.Network.DNSTTLSec != 0 {
		dnsTTLSec = cscope.LinodeCluster.Spec.Network.DNSTTLSec
	}

	subDomain := getSubDomain(cscope)

	for _, eachMachine := range cscope.LinodeMachines.Items {
		for _, IPs := range eachMachine.Status.Addresses {
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
			d.options = append(d.options, DNSOptions{subDomain, IPs.Address, recordType, dnsTTLSec})
		}
		if len(d.options) == 0 {
			continue
		}
		d.options = append(d.options, DNSOptions{subDomain, eachMachine.Name, linodego.RecordTypeTXT, dnsTTLSec})
	}

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
		isOwner, err := IsDomainRecordOwner(ctx, cscope, dnsEntry.Hostname, domainID)
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

func IsDomainRecordOwner(ctx context.Context, cscope *scope.ClusterScope, hostname string, domainID int) (bool, error) {
	// Check if domain record exists
	filter, err := json.Marshal(map[string]interface{}{"name": hostname, "target": cscope.LinodeCluster.Name, "type": linodego.RecordTypeTXT})
	if err != nil {
		return false, err
	}

	domainRecords, err := cscope.LinodeDomainsClient.ListDomainRecords(ctx, domainID, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		return false, err
	}

	// If record exists, update it
	if len(domainRecords) == 0 {
		return false, fmt.Errorf("no txt record %s found with value %s for machine %s", hostname, cscope.LinodeCluster.Name, cscope.LinodeCluster.Name)
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
