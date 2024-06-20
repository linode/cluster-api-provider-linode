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
)

var dnsTTLSec = 30

// AddIPToDNS creates domain record for machine public ip
func AddIPToDNS(ctx context.Context, mscope *scope.MachineScope) error {
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

	// Check if record exists for this IP and name combo
	domainHostname := mscope.LinodeCluster.ObjectMeta.Name + "-" + mscope.LinodeCluster.Spec.Network.DNSUniqueIdentifier
	filter, err := json.Marshal(map[string]interface{}{"name": domainHostname, "target": publicIP})
	if err != nil {
		return fmt.Errorf("failed to marshal domain filter: %w", err)
	}

	domainRecords, err := mscope.LinodeDomainsClient.ListDomainRecords(ctx, domainID, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		return fmt.Errorf("unable to get current DNS record from API: %w", err)
	}

	if mscope.LinodeCluster.Spec.Network.DNSTTLSec != 0 {
		dnsTTLSec = mscope.LinodeCluster.Spec.Network.DNSTTLSec
	}

	// If record doesnt exist, create it else update it
	if domainRecords == nil {
		recordReq := linodego.DomainRecordCreateOptions{
			Type:   "A",
			Name:   domainHostname,
			Target: publicIP,
			TTLSec: dnsTTLSec,
		}

		if _, err := mscope.LinodeDomainsClient.CreateDomainRecord(ctx, domainID, recordReq); err != nil {
			return fmt.Errorf("failed to create domain record: %w", err)
		}
		return nil
	}
	recordReq := linodego.DomainRecordUpdateOptions{
		Type:   "A",
		Name:   domainHostname,
		Target: publicIP,
		TTLSec: dnsTTLSec,
	}
	if _, err := mscope.LinodeDomainsClient.UpdateDomainRecord(
		ctx,
		domainID,
		domainRecords[0].ID,
		recordReq,
	); err != nil {
		return fmt.Errorf("failed to update domain record: %w", err)
	}
	return nil
}

// DeleteNodeFromNB removes a backend Node from the Node Balancer configuration
func DeleteIPFromDNS(ctx context.Context, mscope *scope.MachineScope) error {
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

	// Check if record exists for this IP and name combo
	domainHostname := mscope.LinodeCluster.ObjectMeta.Name + "-" + mscope.LinodeCluster.Spec.Network.DNSUniqueIdentifier
	filter, err := json.Marshal(map[string]interface{}{"name": domainHostname, "target": publicIP})
	if err != nil {
		return fmt.Errorf("failed to marshal domain filter: %w", err)
	}

	domainRecords, err := mscope.LinodeDomainsClient.ListDomainRecords(ctx, domainID, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		return fmt.Errorf("unable to get current DNS record from API: %w", err)
	}

	// If domain record exists, delete it
	if domainRecords != nil {
		err := mscope.LinodeDomainsClient.DeleteDomainRecord(ctx, domainID, domainRecords[0].ID)
		if err != nil {
			return fmt.Errorf("failed to delete domain record: %w", err)
		}
	}

	if mscope.LinodeCluster.Spec.Network.DNSTTLSec != 0 {
		dnsTTLSec = mscope.LinodeCluster.Spec.Network.DNSTTLSec
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
	// Get domainID from domain name
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
