package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"

	"github.com/linode/linodego"
	kutil "sigs.k8s.io/cluster-api/util"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	rutil "github.com/linode/cluster-api-provider-linode/util/reconciler"
)

const numOfDNSEntries = 3

type DNSOptions struct {
	Hostname      string
	Target        string
	DNSRecordType linodego.DomainRecordType
	DNSTTLSec     int
}

// EnsureDNSEntries ensures the domainrecord is created, updated, or deleted based on operation passed
func EnsureDNSEntries(ctx context.Context, mscope *scope.MachineScope, operation string) error {
	// Check if instance is a control plane node
	if !kutil.IsControlPlaneMachine(mscope.Machine) {
		return nil
	}

	// Get the public IP that was assigned
	dnsEntries, err := getDNSEntriesToEnsure(mscope)
	if err != nil {
		return err
	}

	// Get domainID from domain name
	domainID, err := GetDomainID(ctx, mscope)
	if err != nil {
		return err
	}

	for _, dnsEntry := range dnsEntries {
		if operation == "delete" {
			if err := DeleteDomainRecord(ctx, mscope, domainID, dnsEntry); err != nil {
				return err
			}
			continue
		}
		if err := CreateUpdateDomainRecord(ctx, mscope, domainID, dnsEntry); err != nil {
			return err
		}
	}

	return nil
}

// getDNSEntriesToEnsure gets the machines public IP
func getDNSEntriesToEnsure(mscope *scope.MachineScope) ([]DNSOptions, error) {
	dnsTTLSec := rutil.DefaultDNSTTLSec
	if mscope.LinodeCluster.Spec.Network.DNSTTLSec != 0 {
		dnsTTLSec = mscope.LinodeCluster.Spec.Network.DNSTTLSec
	}

	if mscope.LinodeMachine.Status.Addresses == nil {
		return nil, fmt.Errorf("no addresses available on the LinodeMachine resource")
	}
	domainHostname := mscope.LinodeCluster.ObjectMeta.Name + "-" + mscope.LinodeCluster.Spec.Network.DNSUniqueIdentifier

	entries := make([]DNSOptions, 0, numOfDNSEntries)
	recordType := "A"

	for _, IPs := range mscope.LinodeMachine.Status.Addresses {
		if IPs.Type != "ExternalIP" {
			continue
		}
		addr, err := netip.ParseAddr(IPs.Address)
		if err != nil {
			return nil, fmt.Errorf("not a valid IP %w", err)
		}
		if !addr.Is4() {
			recordType = "AAAA"
		}
		entries = append(entries, DNSOptions{domainHostname, IPs.Address, linodego.DomainRecordType(recordType), dnsTTLSec})
	}
	entries = append(entries, DNSOptions{domainHostname, mscope.LinodeMachine.Name, "TXT", dnsTTLSec})

	return entries, nil
}

// GetDomainID gets the domains linode id
func GetDomainID(ctx context.Context, mscope *scope.MachineScope) (int, error) {
	rootDomain := mscope.LinodeCluster.Spec.Network.DNSRootDomain
	filter, err := json.Marshal(map[string]string{"domain": rootDomain})
	if err != nil {
		return 0, err
	}
	domains, err := mscope.LinodeDomainsClient.ListDomains(ctx, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		return 0, err
	}
	if len(domains) != 1 || domains[0].Domain != rootDomain {
		return 0, fmt.Errorf("domain %s not found in list of domains owned by this account", rootDomain)
	}

	return domains[0].ID, nil
}

func CreateUpdateDomainRecord(ctx context.Context, mscope *scope.MachineScope, domainID int, dnsEntry DNSOptions) error {
	// Check if domain record exists for this IP and name combo
	filter, err := json.Marshal(map[string]interface{}{"name": dnsEntry.Hostname, "target": dnsEntry.Target, "type": dnsEntry.DNSRecordType})
	if err != nil {
		return err
	}

	domainRecords, err := mscope.LinodeDomainsClient.ListDomainRecords(ctx, domainID, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		return err
	}

	// If record doesnt exist, create it
	if len(domainRecords) == 0 {
		if err := CreateDomainRecord(ctx, mscope, domainID, dnsEntry); err != nil {
			return err
		}
		return nil
	}

	// If record exists, update it
	if len(domainRecords) != 0 && dnsEntry.DNSRecordType != "TXT" {
		isOwner, err := IsDomainRecordOwner(ctx, mscope, dnsEntry.Hostname, domainID)
		if err != nil {
			return err
		}
		if !isOwner {
			return fmt.Errorf("the domain record is not owned by this entity. wont update")
		}
	}
	if err := UpdateDomainRecord(ctx, mscope, domainID, domainRecords[0].ID, dnsEntry); err != nil {
		return err
	}
	return nil
}

func DeleteDomainRecord(ctx context.Context, mscope *scope.MachineScope, domainID int, dnsEntry DNSOptions) error {
	// Check if domain record exists for this IP and name combo
	filter, err := json.Marshal(map[string]interface{}{"name": dnsEntry.Hostname, "target": dnsEntry.Target, "type": dnsEntry.DNSRecordType})
	if err != nil {
		return err
	}

	domainRecords, err := mscope.LinodeDomainsClient.ListDomainRecords(ctx, domainID, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		return err
	}

	// Nothing to do if records dont exist
	if len(domainRecords) == 0 {
		return nil
	}

	// If record is A type, verify ownership
	if dnsEntry.DNSRecordType != "TXT" {
		isOwner, err := IsDomainRecordOwner(ctx, mscope, dnsEntry.Hostname, domainID)
		if err != nil {
			return err
		}
		if !isOwner {
			return fmt.Errorf("the domain record is not owned by this entity. wont delete")
		}
	}

	// Delete record
	if deleteErr := mscope.LinodeDomainsClient.DeleteDomainRecord(ctx, domainID, domainRecords[0].ID); deleteErr != nil {
		return deleteErr
	}
	return nil
}

func CreateDomainRecord(ctx context.Context, mscope *scope.MachineScope, domainID int, dnsEntries DNSOptions) error {
	recordReq := linodego.DomainRecordCreateOptions{
		Type:   dnsEntries.DNSRecordType,
		Name:   dnsEntries.Hostname,
		Target: dnsEntries.Target,
		TTLSec: dnsEntries.DNSTTLSec,
	}

	if _, err := mscope.LinodeDomainsClient.CreateDomainRecord(ctx, domainID, recordReq); err != nil {
		return err
	}
	return nil
}

func UpdateDomainRecord(ctx context.Context, mscope *scope.MachineScope, domainID, domainRecordID int, dnsEntries DNSOptions) error {
	recordReq := linodego.DomainRecordUpdateOptions{
		Type:   dnsEntries.DNSRecordType,
		Name:   dnsEntries.Hostname,
		Target: dnsEntries.Target,
		TTLSec: dnsEntries.DNSTTLSec,
	}

	if _, err := mscope.LinodeDomainsClient.UpdateDomainRecord(ctx, domainID, domainRecordID, recordReq); err != nil {
		return err
	}
	return nil
}

func IsDomainRecordOwner(ctx context.Context, mscope *scope.MachineScope, hostname string, domainID int) (bool, error) {
	// Check if domain record exists
	filter, err := json.Marshal(map[string]interface{}{"name": hostname, "target": mscope.LinodeMachine.Name, "type": "TXT"})
	if err != nil {
		return false, err
	}

	domainRecords, err := mscope.LinodeDomainsClient.ListDomainRecords(ctx, domainID, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		return false, err
	}

	// If record exists, update it
	if len(domainRecords) == 0 {
		return false, fmt.Errorf("no txt record %s found with value %s for machine %s", hostname, mscope.LinodeMachine.Name, mscope.LinodeMachine.Name)
	}

	return true, nil
}
