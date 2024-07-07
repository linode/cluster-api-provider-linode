package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"time"

	"github.com/linode/linodego"
	kutil "sigs.k8s.io/cluster-api/util"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	rutil "github.com/linode/cluster-api-provider-linode/util/reconciler"
)

const numOfPublicIPs = 2

type IPAndDNSRecordType struct {
	IP            string
	DNSRecordType linodego.DomainRecordType
}

// AddIPToDNS creates the A and TXT record for the machine
func AddIPToDNS(ctx context.Context, mscope *scope.MachineScope) error {
	dnsTTLSec := rutil.DefaultDNSTTLSec
	if mscope.LinodeCluster.Spec.Network.DNSTTLSec != 0 {
		dnsTTLSec = mscope.LinodeCluster.Spec.Network.DNSTTLSec
	}

	// Check if instance is a control plane node
	if !kutil.IsControlPlaneMachine(mscope.Machine) {
		return nil
	}

	// Get the public IP that was assigned
	publicIPs, err := getMachinePublicIPs(mscope)
	if err != nil {
		return err
	}

	// Get domainID from domain name
	domainID, err := GetDomainID(ctx, mscope)
	if err != nil {
		return err
	}
	domainHostname := mscope.LinodeCluster.ObjectMeta.Name + "-" + mscope.LinodeCluster.Spec.Network.DNSUniqueIdentifier

	// Create/Update the A and TXT record for this IP and name combo
	for _, publicIP := range publicIPs {
		if err := CreateUpdateDomainRecord(ctx, mscope, domainHostname, publicIP.IP, dnsTTLSec, domainID, publicIP.DNSRecordType); err != nil {
			return err
		}
		if err := CreateUpdateDomainRecord(ctx, mscope, domainHostname, publicIP.IP, dnsTTLSec, domainID, "TXT"); err != nil {
			return err
		}
	}

	return nil
}

// DeleteIPFromDNS deletes the A and TXT record for the machine
func DeleteIPFromDNS(ctx context.Context, mscope *scope.MachineScope) error {
	dnsTTLSec := rutil.DefaultDNSTTLSec
	if mscope.LinodeCluster.Spec.Network.DNSTTLSec != 0 {
		dnsTTLSec = mscope.LinodeCluster.Spec.Network.DNSTTLSec
	}

	// Check if instance is a control plane node
	if !kutil.IsControlPlaneMachine(mscope.Machine) {
		return nil
	}

	// Get the public IP that was assigned
	publicIPs, err := getMachinePublicIPs(mscope)
	if err != nil {
		return err
	}

	// Get domainID from domain name
	domainID, err := GetDomainID(ctx, mscope)
	if err != nil {
		return err
	}
	domainHostname := mscope.LinodeCluster.ObjectMeta.Name + "-" + mscope.LinodeCluster.Spec.Network.DNSUniqueIdentifier

	// Delete A record
	for _, publicIP := range publicIPs {
		if err := DeleteDomainRecord(ctx, mscope, domainHostname, publicIP.IP, dnsTTLSec, domainID, publicIP.DNSRecordType); err != nil {
			return err
		}
		// Delete TXT record
		if err := DeleteDomainRecord(ctx, mscope, domainHostname, publicIP.IP, dnsTTLSec, domainID, "TXT"); err != nil {
			return err
		}
	}

	// Wait for TTL to expire
	time.Sleep(time.Duration(dnsTTLSec) * time.Second)

	return nil
}

// getMachinePublicIPs gets the machines public IP
func getMachinePublicIPs(mscope *scope.MachineScope) ([]IPAndDNSRecordType, error) {
	if mscope.LinodeMachine.Status.Addresses == nil {
		return nil, fmt.Errorf("no addresses available on the LinodeMachine resource")
	}
	addresses := make([]IPAndDNSRecordType, numOfPublicIPs)
	for count, IPs := range mscope.LinodeMachine.Status.Addresses {
		if IPs.Type == "ExternalIP" {
			addr, err := netip.ParseAddr(IPs.Address)
			if err != nil {
				return nil, fmt.Errorf("not a valid IP %w", err)
			}
			if addr.Is4() {
				addresses[count].DNSRecordType = "A"
				addresses[count].IP = IPs.Address
			} else {
				addresses[count].DNSRecordType = "AAAA"
				addresses[count].IP = IPs.Address
			}
		}
	}

	return addresses, nil
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

func CreateUpdateDomainRecord(ctx context.Context, mscope *scope.MachineScope, hostname, target string, ttl, domainID int, recordType linodego.DomainRecordType) error {
	// Check if domain record exists for this IP and name combo
	filter, err := json.Marshal(map[string]interface{}{"name": hostname, "target": target, "type": recordType})
	if err != nil {
		return err
	}

	domainRecords, err := mscope.LinodeDomainsClient.ListDomainRecords(ctx, domainID, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		return err
	}

	// If record doesnt exist, create it
	if len(domainRecords) == 0 {
		if err := CreateDomainRecord(ctx, mscope, hostname, target, ttl, domainID, recordType); err != nil {
			return err
		}
		return nil
	}

	// If record exists, update it
	if len(domainRecords) != 0 && recordType != "TXT" {
		isOwner, err := IsDomainRecordOwner(ctx, mscope, hostname, target, domainID)
		if err != nil {
			return err
		}
		if !isOwner {
			return fmt.Errorf("the domain record is not owned by this entity. wont update")
		}
	}
	if err := UpdateDomainRecord(ctx, mscope, hostname, target, ttl, domainID, domainRecords[0].ID, recordType); err != nil {
		return err
	}
	return nil
}

func DeleteDomainRecord(ctx context.Context, mscope *scope.MachineScope, hostname, target string, ttl, domainID int, recordType linodego.DomainRecordType) error {
	// Check if domain record exists for this IP and name combo
	filter, err := json.Marshal(map[string]interface{}{"name": hostname, "target": target, "type": recordType})
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
	if recordType != "TXT" {
		isOwner, err := IsDomainRecordOwner(ctx, mscope, hostname, target, domainID)
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

func CreateDomainRecord(ctx context.Context, mscope *scope.MachineScope, hostname, target string, ttl, domainID int, recordType linodego.DomainRecordType) error {
	recordReq := linodego.DomainRecordCreateOptions{
		Type:   recordType,
		Name:   hostname,
		Target: target,
		TTLSec: ttl,
	}

	if _, err := mscope.LinodeDomainsClient.CreateDomainRecord(ctx, domainID, recordReq); err != nil {
		return err
	}
	return nil
}

func UpdateDomainRecord(ctx context.Context, mscope *scope.MachineScope, hostname, target string, ttl, domainID, domainRecordID int, recordType linodego.DomainRecordType) error {
	recordReq := linodego.DomainRecordUpdateOptions{
		Type:   recordType,
		Name:   hostname,
		Target: target,
		TTLSec: ttl,
	}

	if _, err := mscope.LinodeDomainsClient.UpdateDomainRecord(ctx, domainID, domainRecordID, recordReq); err != nil {
		return err
	}
	return nil
}

func IsDomainRecordOwner(ctx context.Context, mscope *scope.MachineScope, hostname, target string, domainID int) (bool, error) {
	// Check if domain record exists
	filter, err := json.Marshal(map[string]interface{}{"name": hostname, "target": target, "type": "TXT"})
	if err != nil {
		return false, err
	}

	domainRecords, err := mscope.LinodeDomainsClient.ListDomainRecords(ctx, domainID, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		return false, err
	}

	// If record exists, update it
	if len(domainRecords) == 0 {
		return false, fmt.Errorf("no txt record %s found with value %s for machine %s", hostname, target, mscope.LinodeMachine.Name)
	}

	return true, nil
}
