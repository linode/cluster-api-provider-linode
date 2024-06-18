package services

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	kutil "sigs.k8s.io/cluster-api/util"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
)

// AddIPToDNS creates domain record for machine public ip
func AddIPToDNS(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
) error {
	// Check if instance is a control plane node
	if !kutil.IsControlPlaneMachine(machineScope.Machine) {
		return nil
	}

	// Get the public IP that was assigned
	publicIP, err := GetMachinePublicIP(ctx, logger, machineScope)
	if err != nil {
		logger.Error(err, "Failed to get public IP of machine")
		return err
	}

	// Get domainID from domain name
	domainID, err := GetDomainID(ctx, logger, machineScope)
	if err != nil {
		logger.Error(err, "Failed to get domain ID")
		return err
	}

	// Check if record exists for this IP and name combo
	domainHostname := machineScope.LinodeCluster.ObjectMeta.Name + "-" + machineScope.LinodeCluster.Spec.Network.DNSUniqueIdentifier
	filter, err := json.Marshal(map[string]interface{}{"name": domainHostname, "target": publicIP})
	if err != nil {
		logger.Error(err, "Failed to marshal domain filter")
		return err
	}

	domainRecords, err := machineScope.LinodeDomainsClient.ListDomainRecords(ctx, domainID, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		logger.Error(err, "unable to get current DNS record from API")
		return err
	}

	// If record doesnt exist, create it else update it
	if domainRecords == nil {
		recordReq := linodego.DomainRecordCreateOptions{
			Type:   "A",
			Name:   domainHostname,
			Target: publicIP,
		}

		_, err := machineScope.LinodeDomainsClient.CreateDomainRecord(ctx, domainID, recordReq)
		if err != nil {
			logger.Error(err, "Failed to create domain record")
			return err
		}
	} else {
		recordReq := linodego.DomainRecordUpdateOptions{
			Type:   "A",
			Name:   domainHostname,
			Target: publicIP,
		}

		_, err := machineScope.LinodeDomainsClient.UpdateDomainRecord(ctx, domainID, domainRecords[0].ID, recordReq)
		if err != nil {
			logger.Error(err, "Failed to update domain record")
			return err
		}
	}

	return nil
}

// DeleteNodeFromNB removes a backend Node from the Node Balancer configuration
func DeleteIPFromDNS(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
) error {
	// Check if instance is a control plane node
	if !kutil.IsControlPlaneMachine(machineScope.Machine) {
		return nil
	}

	// Get the public IP that was assigned
	publicIP, err := GetMachinePublicIP(ctx, logger, machineScope)
	if err != nil {
		logger.Error(err, "Failed to get public IP of machine")
		return err
	}

	// Get domainID from domain name
	domainID, err := GetDomainID(ctx, logger, machineScope)
	if err != nil {
		logger.Error(err, "Failed to get domain ID")
		return err
	}

	// Check if record exists for this IP and name combo
	domainHostname := machineScope.LinodeCluster.ObjectMeta.Name + "-" + machineScope.LinodeCluster.Spec.Network.DNSUniqueIdentifier
	filter, err := json.Marshal(map[string]interface{}{"name": domainHostname, "target": publicIP})
	if err != nil {
		logger.Error(err, "Failed to marshal domain filter")
		return err
	}

	domainRecords, err := machineScope.LinodeDomainsClient.ListDomainRecords(ctx, domainID, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		logger.Error(err, "unable to get current DNS record from API")
		return err
	}

	// If domain record exists, delete it
	if domainRecords != nil {
		err := machineScope.LinodeDomainsClient.DeleteDomainRecord(ctx, domainID, domainRecords[0].ID)
		if err != nil {
			logger.Error(err, "Failed to delete domain record")
			return err
		}
	}

	return nil
}

// GetMachinePublicIP gets the machines public IP
func GetMachinePublicIP(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
) (string, error) {
	// Get the public IP that was assigned
	addresses, err := machineScope.LinodeClient.GetInstanceIPAddresses(ctx, *machineScope.LinodeMachine.Spec.InstanceID)
	if err != nil {
		logger.Error(err, "Failed get instance IP addresses")

		return "", err
	}
	if len(addresses.IPv4.Public) == 0 {
		err := errors.New("no public IP address")
		logger.Error(err, "no public IPV4 addresses set for LinodeInstance")

		return "", err
	}

	return addresses.IPv4.Public[0].Address, nil
}

// GetDomainID gets the domains linode id
func GetDomainID(
	ctx context.Context,
	logger logr.Logger,
	machineScope *scope.MachineScope,
) (int, error) {
	// Get domainID from domain name
	rootDomain := machineScope.LinodeCluster.Spec.Network.DNSRootDomain
	filter, err := json.Marshal(map[string]interface{}{"domain": rootDomain})
	if err != nil {
		logger.Error(err, "Failed to marshal domain filter")
		return 0, err
	}

	domains, err := machineScope.LinodeDomainsClient.ListDomains(ctx, linodego.NewListOptions(0, string(filter)))

	if err != nil {
		logger.Error(err, "Failed to list matching domains")
		return 0, err
	}
	if len(domains) != 1 || domains[0].Domain != rootDomain {
		logger.Error(err, "Failed to retrieve Linode Domain")
		return 0, err
	}

	return domains[0].ID, nil
}
