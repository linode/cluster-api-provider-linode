package controller

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/clients"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
)

const (
	maxFirewallRuleLabelLen = 32
	maxIPsPerFirewallRule   = 255
	maxRulesPerFirewall     = 25
)

const (
	ruleTypeInbound  = "inbound"
	ruleTypeOutbound = "outbound"
)

var (
	errTooManyIPs = errors.New("too many IPs in this ACL, will exceed rules per firewall limit")
)

func findObjectsForObject(logger logr.Logger, tracedClient client.Client) handler.MapFunc {
	logger = logger.WithName("LinodeFirewallReconciler").WithName("findObjectsForObject")
	return func(ctx context.Context, obj client.Object) []ctrl.Request {
		ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultMappingTimeout)
		defer cancel()

		// Get all Firewalls because we can't filter on arbitrary fields in the spec
		firewalls := &infrav1alpha2.LinodeFirewallList{}
		if err := tracedClient.List(ctx, firewalls, &client.ListOptions{}); err != nil {
			switch {
			case apierrors.IsNotFound(err) || firewalls == nil:
				logger.Info("LinodeFirewall(s) not found for %s")

				return nil
			case err != nil:
				logger.Error(err, "Failed to get LinodeFirewalls")

				return nil
			}
		}

		return buildRequests(firewalls.Items, obj)
	}
}

// Constructs a unique list of requests for updating LinodeFirewalls that either reference the
// AddressSet / FirewallRule
func buildRequests(firewalls []infrav1alpha2.LinodeFirewall, obj client.Object) []reconcile.Request {
	requestSet := make(map[reconcile.Request]struct{})
	for _, firewall := range firewalls {
		for _, inboundRule := range firewall.Spec.InboundRules {
			requestSet = buildRequestsHelper(requestSet, firewall, inboundRule.AddressSetRefs, obj)
		}
		for _, outboundRule := range firewall.Spec.OutboundRules {
			requestSet = buildRequestsHelper(requestSet, firewall, outboundRule.AddressSetRefs, obj)
		}
		requestSet = buildRequestsHelper(requestSet, firewall, firewall.Spec.InboundRuleRefs, obj)
		requestSet = buildRequestsHelper(requestSet, firewall, firewall.Spec.OutboundRuleRefs, obj)
	}

	return slices.Collect(maps.Keys(requestSet))
}

func buildRequestsHelper(requestSet map[reconcile.Request]struct{}, firewall infrav1alpha2.LinodeFirewall, objRefs []*corev1.ObjectReference, obj client.Object) map[reconcile.Request]struct{} {
	for _, objRef := range objRefs {
		if objRef.Namespace == "" {
			objRef.Namespace = firewall.Namespace
		}
		if objRef.Name == obj.GetName() && objRef.Namespace == obj.GetNamespace() {
			requestSet[reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      firewall.GetName(),
					Namespace: firewall.GetNamespace(),
				},
			}] = struct{}{}
		}
	}

	return requestSet
}

// reconcileFirewall takes the CAPL firewall representation and uses it to either create or update the Cloud Firewall
// via the given linode client
func reconcileFirewall(
	ctx context.Context,
	k8sClient clients.K8sClient,
	fwScope *scope.FirewallScope,
	logger logr.Logger,
) error {
	// build out the firewall rules for create or update
	if fwScope.LinodeFirewall.Namespace == "" {
		fwScope.LinodeFirewall.Namespace = "default"
	}
	fwConfig, err := processACL(ctx, k8sClient, logger, fwScope.LinodeFirewall)
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
		return [][]string{}
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

// transformToCIDR converts a single IP address to CIDR notation if needed
// e.g., "192.168.1.1" becomes "192.168.1.1/32"
func transformToCIDR(ip string) string {
	// If already contains /, assume it's already in CIDR notation
	if strings.Contains(ip, "/") {
		return ip
	}

	// Try parsing as IPv4
	if parsed := net.ParseIP(ip); parsed != nil {
		if parsed.To4() != nil {
			return ip + "/32"
		}
		// For IPv6
		return ip + "/128"
	}

	// If not a valid IP, return as-is (will be validated later)
	return ip
}

func filterDuplicates(ipv4s, ipv6s []string) (filteredIPv4s, filteredIPv6s []string) {
	// Declare "sets". Empty structs occupy 0 memory
	ipv4Set := make(map[string]struct{})
	ipv6Set := make(map[string]struct{})
	for _, ip := range ipv4s {
		ipv4Set[ip] = struct{}{}
	}
	for _, ip := range ipv6s {
		ipv6Set[ip] = struct{}{}
	}
	return slices.Collect(maps.Keys(ipv4Set)), slices.Collect(maps.Keys(ipv6Set))
}

// processRule handles a single inbound/outbound rule
func processRule(ctx context.Context, k8sClient clients.K8sClient, firewall *infrav1alpha2.LinodeFirewall, log logr.Logger, rule infrav1alpha2.FirewallRuleSpec, ruleType string, createOpts *linodego.FirewallCreateOptions) error {
	ruleIPv4s := make([]string, 0)
	ruleIPv6s := make([]string, 0)
	if rule.Addresses != nil {
		ipv4s, ipv6s := processAddresses(rule.Addresses)
		ruleIPv4s = append(ruleIPv4s, ipv4s...)
		ruleIPv6s = append(ruleIPv6s, ipv6s...)
	}
	if rule.AddressSetRefs != nil {
		ipv4s, ipv6s, err := processAddressSetRefs(ctx, k8sClient, firewall, rule.AddressSetRefs, log)
		if err != nil {
			return err
		}
		ruleIPv4s = append(ruleIPv4s, ipv4s...)
		ruleIPv6s = append(ruleIPv6s, ipv6s...)
	}
	ruleIPv4s, ruleIPv6s = filterDuplicates(ruleIPv4s, ruleIPv6s)

	ruleLabel := formatRuleLabel(rule.Action, rule.Label)

	switch ruleType {
	case ruleTypeInbound:
		processIPRules(ruleIPv4s, rule, ruleLabel, linodego.IPTypeIPv4, &createOpts.Rules.Inbound)
		processIPRules(ruleIPv6s, rule, ruleLabel, linodego.IPTypeIPv6, &createOpts.Rules.Inbound)
	case ruleTypeOutbound:
		processIPRules(ruleIPv4s, rule, ruleLabel, linodego.IPTypeIPv4, &createOpts.Rules.Outbound)
		processIPRules(ruleIPv6s, rule, ruleLabel, linodego.IPTypeIPv6, &createOpts.Rules.Outbound)
	}

	return nil
}

// processAddresses extracts and transforms IPv4 and IPv6 addresses
func processAddresses(addresses *infrav1alpha2.NetworkAddresses) (ipv4s, ipv6s []string) {
	// Initialize empty slices for consistent return type
	ipv4s = make([]string, 0)
	ipv6s = make([]string, 0)
	// Early return if addresses is nil
	if addresses == nil {
		return ipv4s, ipv6s
	}
	// Declare "sets". Empty structs occupy 0 memory
	ipv4Set := make(map[string]struct{})
	ipv6Set := make(map[string]struct{})
	// Process IPv4 addresses
	if addresses.IPv4 != nil {
		for _, ip := range *addresses.IPv4 {
			ipv4Set[transformToCIDR(ip)] = struct{}{}
		}
	}

	// Process IPv6 addresses
	if addresses.IPv6 != nil {
		for _, ip := range *addresses.IPv6 {
			ipv6Set[transformToCIDR(ip)] = struct{}{}
		}
	}

	return slices.Collect(maps.Keys(ipv4Set)), slices.Collect(maps.Keys(ipv6Set))
}

// processAddressSetRefs extracts and transforms IPv4 and IPv6 addresses from the reference AddressSet(s)
func processAddressSetRefs(ctx context.Context, k8sClient clients.K8sClient, firewall *infrav1alpha2.LinodeFirewall, addressSetRefs []*corev1.ObjectReference, log logr.Logger) (ipv4s, ipv6s []string, err error) {
	// Initialize empty slices for consistent return type
	ipv4s = make([]string, 0)
	ipv6s = make([]string, 0)
	// Declare "sets". Empty structs occupy 0 memory
	ipv4Set := make(map[string]struct{})
	ipv6Set := make(map[string]struct{})

	for _, addrSetRef := range addressSetRefs {
		addrSet := &infrav1alpha2.AddressSet{}
		if addrSetRef.Namespace == "" {
			addrSetRef.Namespace = firewall.Namespace
		}
		if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: addrSetRef.Namespace, Name: addrSetRef.Name}, addrSet); err != nil {
			log.Error(err, "failed to get AddressSet", "namespace", addrSetRef.Namespace, "name", addrSetRef.Name)

			return ipv4s, ipv6s, err
		}

		// Process IPv4 addresses
		if addrSet.Spec.IPv4 != nil {
			for _, ip := range *addrSet.Spec.IPv4 {
				ipv4Set[transformToCIDR(ip)] = struct{}{}
			}
		}
		// Process IPv6 addresses
		if addrSet.Spec.IPv6 != nil {
			for _, ip := range *addrSet.Spec.IPv6 {
				ipv6Set[transformToCIDR(ip)] = struct{}{}
			}
		}
	}

	return slices.Collect(maps.Keys(ipv4Set)), slices.Collect(maps.Keys(ipv6Set)), nil
}

// formatRuleLabel creates and formats the rule label
func formatRuleLabel(prefix, label string) string {
	ruleLabel := fmt.Sprintf("%s-%s", prefix, label)
	if len(ruleLabel) > maxFirewallRuleLabelLen {
		return ruleLabel[0:maxFirewallRuleLabelLen]
	}
	return ruleLabel
}

// processIPRules processes IP rules and adds them to the rules slice
func processIPRules(ips []string, rule infrav1alpha2.FirewallRuleSpec, ruleLabel string, ipType linodego.InstanceIPType, rules *[]linodego.FirewallRule) {
	// Initialize rules if nil
	if *rules == nil {
		*rules = make([]linodego.FirewallRule, 0)
	}

	// If no IPs, return early
	if len(ips) == 0 {
		return
	}

	ipchunks := chunkIPs(ips)
	//nolint:exhaustive // This function only handles explicit IPv4 and IPv6 types; other types like IPv6 Pool/Range are not relevant here.
	switch ipType {
	case linodego.IPTypeIPv4:
		for i, chunk := range ipchunks {
			*rules = append(*rules, linodego.FirewallRule{
				Action:      rule.Action,
				Label:       ruleLabel,
				Description: fmt.Sprintf("Rule %d, Created by CAPL: %s", i, rule.Label),
				Protocol:    rule.Protocol,
				Ports:       rule.Ports,
				Addresses:   linodego.NetworkAddresses{IPv4: &chunk},
			})
		}
	case linodego.IPTypeIPv6:
		for i, chunk := range ipchunks {
			*rules = append(*rules, linodego.FirewallRule{
				Action:      rule.Action,
				Label:       ruleLabel,
				Description: fmt.Sprintf("Rule %d, Created by CAPL: %s", i, rule.Label),
				Protocol:    rule.Protocol,
				Ports:       rule.Ports,
				Addresses:   linodego.NetworkAddresses{IPv6: &chunk},
			})
		}
	}
}

func processFirewallRule(ctx context.Context, k8sClient clients.K8sClient, firewall *infrav1alpha2.LinodeFirewall, log logr.Logger, ruleRef *corev1.ObjectReference, ruleType string, createOpts *linodego.FirewallCreateOptions) error {
	rule := &infrav1alpha2.FirewallRule{}
	if ruleRef.Namespace == "" {
		ruleRef.Namespace = firewall.Namespace
	}
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: ruleRef.Namespace, Name: ruleRef.Name}, rule); err != nil {
		log.Error(err, "failed to get FirewallRule", "namespace", ruleRef.Namespace, "name", ruleRef.Name)

		return err
	}
	if err := processRule(ctx, k8sClient, firewall, log, rule.Spec, ruleType, createOpts); err != nil {
		return err
	}

	return nil
}

// processACL uses the CAPL LinodeFirewall representation to build out the inbound
// and outbound rules for a linode Cloud Firewall
func processACL(ctx context.Context, k8sClient clients.K8sClient, log logr.Logger, firewall *infrav1alpha2.LinodeFirewall) (*linodego.FirewallCreateOptions, error) {
	createOpts := &linodego.FirewallCreateOptions{
		Label: firewall.Name,
	}

	// Process inbound rules
	for _, rule := range firewall.Spec.InboundRules {
		if err := processRule(ctx, k8sClient, firewall, log, rule, ruleTypeInbound, createOpts); err != nil {
			return nil, err
		}
	}
	for _, ruleRef := range firewall.Spec.InboundRuleRefs {
		if err := processFirewallRule(ctx, k8sClient, firewall, log, ruleRef, ruleTypeInbound, createOpts); err != nil {
			return nil, err
		}
	}

	// Set inbound policy
	if firewall.Spec.InboundPolicy == "" {
		createOpts.Rules.InboundPolicy = "ACCEPT"
	} else {
		createOpts.Rules.InboundPolicy = firewall.Spec.InboundPolicy
	}

	// Process outbound rules
	for _, rule := range firewall.Spec.OutboundRules {
		if err := processRule(ctx, k8sClient, firewall, log, rule, ruleTypeOutbound, createOpts); err != nil {
			return nil, err
		}
	}
	for _, ruleRef := range firewall.Spec.OutboundRuleRefs {
		if err := processFirewallRule(ctx, k8sClient, firewall, log, ruleRef, ruleTypeOutbound, createOpts); err != nil {
			return nil, err
		}
	}

	// Set outbound policy
	if firewall.Spec.OutboundPolicy == "" {
		createOpts.Rules.OutboundPolicy = "ACCEPT"
	} else {
		createOpts.Rules.OutboundPolicy = firewall.Spec.OutboundPolicy
	}

	// Check rule count
	if len(createOpts.Rules.Inbound)+len(createOpts.Rules.Outbound) > maxRulesPerFirewall {
		return nil, errTooManyIPs
	}

	return createOpts, nil
}
