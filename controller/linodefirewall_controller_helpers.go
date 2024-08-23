package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/clients"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
)

const (
	maxFirewallRuleLabelLen = 32
	maxIPsPerFirewallRule   = 255
	maxRulesPerFirewall     = 25
)

var (
	errTooManyIPs = errors.New("too many IPs in this ACL, will exceed rules per firewall limit")
)

// reconcileFirewall takes the CAPL firewall representation and uses it to either create or update the Cloud Firewall
// via the given linode client
func reconcileFirewall(
	ctx context.Context,
	fwScope *scope.FirewallScope,
	logger logr.Logger,
) error {
	// build out the firewall rules for create or update
	fwConfig, err := processACL(fwScope.LinodeFirewall)
	if err != nil {
		logger.Info("Failed to process ACL", "error", err.Error())

		return err
	}
	var linodeFW *linodego.Firewall

	switch fwScope.LinodeFirewall.Spec.FirewallID {
	case nil:
		logger.Info(fmt.Sprintf("Creating firewall %s", fwScope.LinodeFirewall.Name))
		linodeFW, err = fwScope.LinodeClient.CreateFirewall(ctx, *fwConfig)
		if err != nil {
			logger.Info("Failed to create firewall", "error", err.Error())

			return err
		}
		if linodeFW == nil {
			err = errors.New("nil firewall")
			logger.Error(err, "Created firewall is nil")

			return err
		}
		fwScope.LinodeFirewall.Spec.FirewallID = util.Pointer(linodeFW.ID)
	default:
		logger.Info(fmt.Sprintf("Updating firewall %s", fwScope.LinodeFirewall.Name))
		linodeFW, err = fwScope.LinodeClient.GetFirewall(ctx, *fwScope.LinodeFirewall.Spec.FirewallID)
		if err != nil {
			logger.Info("Failed to get firewall", "error", err.Error())

			return err
		}
		if err = updateFirewall(ctx, fwScope.LinodeClient, linodeFW, fwConfig); err != nil {
			logger.Info("Failed to update firewall", "error", err.Error())

			return err
		}
	}

	// Need to make sure the firewall is appropriately enabled or disabled after
	// create or update and the tags are properly set
	var status linodego.FirewallStatus
	if fwScope.LinodeFirewall.Spec.Enabled {
		status = linodego.FirewallEnabled
	} else {
		status = linodego.FirewallDisabled
	}
	if _, err = fwScope.LinodeClient.UpdateFirewall(
		ctx,
		linodeFW.ID,
		linodego.FirewallUpdateOptions{
			Status: status,
		},
	); err != nil {
		logger.Info("Failed to update Linode Firewall status and tags", "error", err.Error())

		return err
	}

	return nil
}

func updateFirewall(
	ctx context.Context,
	linodeClient clients.LinodeClient,
	linodeFW *linodego.Firewall,
	fwConfig *linodego.FirewallCreateOptions,
) error {
	if _, err := linodeClient.UpdateFirewallRules(ctx, linodeFW.ID, fwConfig.Rules); err != nil {
		return err
	}

	return nil
}

// chunkIPs takes a list of strings representing IPs and breaks them up into
// one or more lists capped at the maxIPsPerFirewallRule for length
func chunkIPs(ips []string) [][]string {
	ipCount := len(ips)
	if ipCount == 0 {
		return nil
	}

	// If the number of IPs is less than or equal to maxIPsPerFirewall,
	// return a single chunk containing all IPs.
	if ipCount <= maxIPsPerFirewallRule {
		return [][]string{ips}
	}

	// Otherwise, break the IPs into chunks with maxIPsPerFirewall IPs per chunk.
	chunkCount := 0
	chunks := [][]string{}
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
func processACL(firewall *infrav1alpha2.LinodeFirewall) (
	*linodego.FirewallCreateOptions,
	error,
) {
	createOpts := &linodego.FirewallCreateOptions{
		Label: firewall.Name,
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
	if firewall.Spec.InboundPolicy == "" {
		createOpts.Rules.InboundPolicy = "ACCEPT"
	} else {
		createOpts.Rules.InboundPolicy = firewall.Spec.InboundPolicy
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

	if firewall.Spec.OutboundPolicy == "" {
		createOpts.Rules.OutboundPolicy = "ACCEPT"
	} else {
		createOpts.Rules.OutboundPolicy = firewall.Spec.OutboundPolicy
	}

	// need to check if we ended up needing to make too many rules
	// with IP chunking
	if len(createOpts.Rules.Inbound)+len(createOpts.Rules.Outbound) > maxRulesPerFirewall {
		return nil, errTooManyIPs
	}

	return createOpts, nil
}
