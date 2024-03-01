package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/linode/cluster-api-provider-linode/util"
	"net/http"
	"slices"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
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

func HandleFirewall(
	ctx context.Context,
	firewallScope *scope.FirewallScope,
	logger logr.Logger,
) (linodeFW *linodego.Firewall, err error) {
	clusterUID := string(firewallScope.LinodeCluster.UID)
	tags := []string{string(firewallScope.LinodeCluster.UID)}
	fwName := firewallScope.LinodeFirewall.Name

	linodeFWs, err := fetchFirewalls(ctx, firewallScope)
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
	fwConfig, err := processACL(firewallScope.LinodeFirewall, tags)
	if err != nil {
		logger.Info("Failed to process ACL", "error", err.Error())

		return nil, err
	}

	if len(linodeFWs) == 0 {
		logger.Info(fmt.Sprintf("Creating firewall %s", fwName))

		if linodeFW, err = firewallScope.LinodeClient.CreateFirewall(ctx, *fwConfig); err != nil {
			logger.Info("Failed to create Linode Firewall", "error", err.Error())
			// Already exists is not an error
			apiErr := linodego.Error{}
			if errors.As(err, &apiErr) && apiErr.Code != http.StatusFound {
				return nil, err
			}

			if linodeFW != nil {
				logger.Info(fmt.Sprintf("Linode Firewall %s already exists", fwName))
			}
		}

	} else {
		logger.Info(fmt.Sprintf("Updating firewall %s", fwName))

		linodeFW = &linodeFWs[0]
		if !slices.Contains(linodeFW.Tags, clusterUID) {
			err := errors.New("firewall conflict")
			logger.Error(err, fmt.Sprintf(
				"Firewall %s is not associated with cluster UID %s. Owner cluster is %s",
				fwName,
				clusterUID,
				linodeFW.Tags[0],
			))

			return nil, err
		}

		if _, err := firewallScope.LinodeClient.UpdateFirewallRules(ctx, linodeFW.ID, fwConfig.Rules); err != nil {
			logger.Info("Failed to update Linode Firewall", "error", err.Error())

			return nil, err
		}
	}

	// Need to make sure the firewall is appropriately enabled or disabled after
	// create or update and the tags are properly set
	var status linodego.FirewallStatus
	if firewallScope.LinodeFirewall.Spec.Enabled {
		status = linodego.FirewallEnabled
	} else {
		status = linodego.FirewallDisabled
	}
	if _, err = firewallScope.LinodeClient.UpdateFirewall(
		ctx,
		linodeFW.ID,
		linodego.FirewallUpdateOptions{
			Status: status,
			Tags:   util.Pointer(tags),
		},
	); err != nil {
		logger.Info("Failed to update Linode Firewall status and tags", "error", err.Error())

		return nil, err
	}

	return linodeFW, nil
}

// fetch Firewalls returns all Linode firewalls with a label matching the CAPL Firewall name
func fetchFirewalls(ctx context.Context, firewallScope *scope.FirewallScope) (firewalls []linodego.Firewall, err error) {
	var linodeFWs []linodego.Firewall
	filter := map[string]string{
		"label": firewallScope.LinodeFirewall.Name,
	}

	rawFilter, err := json.Marshal(filter)
	if err != nil {
		return nil, err
	}
	if linodeFWs, err = firewallScope.LinodeClient.ListFirewalls(ctx, linodego.NewListOptions(1, string(rawFilter))); err != nil {
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

// processACL builds out a Linode firewall configuration for a given CAPL Firewall object which can then
// be used to create or update a Linode firewall
func processACL(firewall *infrav1alpha1.LinodeFirewall, tags []string) (*linodego.FirewallCreateOptions, error) {
	createOpts := &linodego.FirewallCreateOptions{
		Label: firewall.Name,
		Tags:  tags,
	}

	// process inbound rules
	for _, rule := range firewall.Spec.InboundRules {
		var ruleIPv4s []string
		var ruleIPv6s []string

		if rule.Addresses.IPv4 != nil {
			ruleIPv4s = append(ruleIPv4s, *rule.Addresses.IPv4...)
		}

		if rule.Addresses.IPv6 != nil {
			ruleIPv6s = append(ruleIPv6s, *rule.Addresses.IPv6...)
		}

		ruleLabel := fmt.Sprintf("%s-%s", firewall.Spec.InboundPolicy, rule.Label)
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
		var ruleIPv4s []string
		var ruleIPv6s []string

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
