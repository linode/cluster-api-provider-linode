package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/linode/linodego"
	kutil "sigs.k8s.io/cluster-api/util"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	rutil "github.com/linode/cluster-api-provider-linode/util/reconciler"
)

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
	publicIP, err := GetMachinePublicIP(ctx, mscope)
	if err != nil {
		return fmt.Errorf("failed to get public IP of machine: %w", err)
	}

	// Get domainID from domain name
	domainID, err := GetDomainID(ctx, mscope)
	if err != nil {
		return fmt.Errorf("failed to get domain ID: %w", err)
	}
	domainHostname := mscope.LinodeCluster.ObjectMeta.Name + "-" + mscope.LinodeCluster.Spec.Network.DNSUniqueIdentifier

	// Create/Update the A record for this IP and name combo
	if err := CreateUpdateDomainRecord(ctx, mscope, domainHostname, publicIP, dnsTTLSec, domainID, "A"); err != nil {
		return fmt.Errorf("failed to create/update A domain record: %w", err)
	}

	// Create/Update the TXT record for this IP and name combo
	target := "owner:" + mscope.LinodeCluster.Name + ",ip:" + publicIP
	if err := CreateUpdateDomainRecord(ctx, mscope, domainHostname, target, dnsTTLSec, domainID, "TXT"); err != nil {
		return fmt.Errorf("failed to create/update TXT domain record: %w", err)
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
	publicIP, err := GetMachinePublicIP(ctx, mscope)
	if err != nil {
		return fmt.Errorf("failed to get public IP of machine: %w", err)
	}

	// Get domainID from domain name
	domainID, err := GetDomainID(ctx, mscope)
	if err != nil {
		return fmt.Errorf("failed to get domain ID: %w", err)
	}
	domainHostname := mscope.LinodeCluster.ObjectMeta.Name + "-" + mscope.LinodeCluster.Spec.Network.DNSUniqueIdentifier

	// Delete A record
	if err := DeleteDomainRecord(ctx, mscope, domainHostname, publicIP, dnsTTLSec, domainID, "A"); err != nil {
		return fmt.Errorf("failed to delete A domain record: %w", err)
	}

	// Delete TXT record
	target := "owner:" + mscope.LinodeCluster.Name + ",ip:" + publicIP
	if err := DeleteDomainRecord(ctx, mscope, domainHostname, target, dnsTTLSec, domainID, "TXT"); err != nil {
		return fmt.Errorf("failed to delete A domain record: %w", err)
	}

	// Wait for TTL to expire
	time.Sleep(time.Duration(dnsTTLSec) * time.Second)

	return nil
}

// GetMachinePublicIP gets the machines public IP
func GetMachinePublicIP(ctx context.Context, mscope *scope.MachineScope) (string, error) {
	// Verify instance id is not nil
	if mscope.LinodeMachine.Spec.InstanceID == nil {
		err := errors.New("instance ID is nil. cant get machine's public ip")
		return "", err
	}

	// Get the public IP that was assigned
	addresses, err := mscope.LinodeClient.GetInstanceIPAddresses(ctx, *mscope.LinodeMachine.Spec.InstanceID)
	if err != nil {
		return "", fmt.Errorf("failed to get ip address of the instance: %w", err)
	}
	if len(addresses.IPv4.Public) == 0 {
		err := errors.New("no public IP address")
		return "", err
	}

	return addresses.IPv4.Public[0].Address, nil
}

// GetDomainID gets the domains linode id
func GetDomainID(ctx context.Context, mscope *scope.MachineScope) (int, error) {
	rootDomain := mscope.LinodeCluster.Spec.Network.DNSRootDomain
	filter, err := json.Marshal(map[string]string{"domain": rootDomain})
	if err != nil {
		return 0, fmt.Errorf("failed to marshal domain filter: %w", err)
	}
	domains, err := mscope.LinodeDomainsClient.ListDomains(ctx, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		return 0, fmt.Errorf("failed to list domains: %w", err)
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
		return fmt.Errorf("failed to marshal domain filter: %w", err)
	}

	domainRecords, err := mscope.LinodeDomainsClient.ListDomainRecords(ctx, domainID, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		return fmt.Errorf("unable to get current DNS record from API: %w", err)
	}

	// If record doesnt exist, create it else update it
	if len(domainRecords) == 0 {
		if err := CreateDomainRecord(ctx, mscope, hostname, target, ttl, domainID, recordType); err != nil {
			return fmt.Errorf("failed to create domain record: %w", err)
		}
	} else {
		if err := UpdateDomainRecord(ctx, mscope, hostname, target, ttl, domainID, domainRecords[0].ID, recordType); err != nil {
			return fmt.Errorf("failed to update domain record: %w", err)
		}
	}
	return nil
}

func DeleteDomainRecord(ctx context.Context, mscope *scope.MachineScope, hostname, target string, ttl, domainID int, recordType linodego.DomainRecordType) error {
	// Check if domain record exists for this IP and name combo
	filter, err := json.Marshal(map[string]interface{}{"name": hostname, "target": target, "type": recordType})
	if err != nil {
		return fmt.Errorf("failed to marshal domain filter: %w", err)
	}

	domainRecords, err := mscope.LinodeDomainsClient.ListDomainRecords(ctx, domainID, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		return fmt.Errorf("unable to get current DNS record from API: %w", err)
	}

	// If domain record exists, delete it
	if len(domainRecords) != 0 {
		err := mscope.LinodeDomainsClient.DeleteDomainRecord(ctx, domainID, domainRecords[0].ID)
		if err != nil {
			return fmt.Errorf("failed to delete domain record: %w", err)
		}
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
		return fmt.Errorf("failed to create domain record of type A: %w", err)
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
		return fmt.Errorf("failed to create domain record of type A: %w", err)
	}
	return nil
}
