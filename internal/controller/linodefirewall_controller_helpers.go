package controller

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/http"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	"github.com/linode/linodego/v2"
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
	errTooManyIPs  = errors.New("too many IPs in this ACL, will exceed rules per firewall limit")
	errNilFirewall = errors.New("nil error and nil firewall")
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
		linodeFW, err = createFirewall(ctx, fwScope, fwConfig, logger)
	default:
		linodeFW, err = updateFirewall(ctx, fwScope, fwConfig, logger)
	}
	if err != nil {
		return err
	}

	// Need to make sure the firewall is appropriately enabled or disabled after
	// create or update and the tags are properly set
	status := linodego.FirewallEnabled
	if !fwScope.LinodeFirewall.Spec.Enabled {
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

func createFirewall(ctx context.Context, fwScope *scope.FirewallScope, fwConfig *linodego.FirewallRules, logger logr.Logger) (*linodego.Firewall, error) {
	logger.Info(fmt.Sprintf("Creating firewall %s", fwScope.LinodeFirewall.Name))
	opts := linodego.FirewallCreateOptions{
		Label: fwScope.LinodeFirewall.Name,
		Rules: linodego.FirewallRulesCreateOptions{
			Inbound:        fwConfig.Inbound,
			InboundPolicy:  fwConfig.InboundPolicy,
			Outbound:       fwConfig.Outbound,
			OutboundPolicy: fwConfig.OutboundPolicy,
		},
	}
	linodeFW, err := fwScope.LinodeClient.CreateFirewall(ctx, opts)
	// Handle the edge case where API did create the firewall eventually after timing out on the client side
	if linodego.ErrHasStatus(err, http.StatusBadRequest) && strings.Contains(err.Error(), "Label must be unique") {
		logger.Error(err, "Failed to create firewall, received [400 BadRequest] response.")

		// check if instance already exists
		listFilter := util.Filter{Label: fwScope.LinodeFirewall.Name}
		filter, errFilter := listFilter.String()
		if errFilter != nil {
			logger.Error(errFilter, "Failed to create filter to list firewall")
			return nil, errFilter
		}
		firewalls, listErr := fwScope.LinodeClient.ListFirewalls(ctx, linodego.NewListOptions(1, filter))
		if listErr != nil {
			return nil, listErr
		}
		if len(firewalls) > 0 {
			linodeFW = &firewalls[0]
		}
	} else if err != nil {
		logger.Info("Failed to create firewall", "error", err.Error())

		return nil, err
	}
	if linodeFW == nil {
		return nil, errNilFirewall
	}
	fwScope.LinodeFirewall.Spec.FirewallID = util.Pointer(linodeFW.ID)

	return linodeFW, nil
}

func updateFirewall(ctx context.Context, fwScope *scope.FirewallScope, fwConfig *linodego.FirewallRules, logger logr.Logger) (*linodego.Firewall, error) {
	logger.Info(fmt.Sprintf("Updating firewall %s", fwScope.LinodeFirewall.Name))
	linodeFW, err := fwScope.LinodeClient.GetFirewall(ctx, *fwScope.LinodeFirewall.Spec.FirewallID)
	if err != nil {
		logger.Info("Failed to get firewall", "error", err.Error())

		return nil, err
	}
	if linodeFW == nil {
		return nil, errNilFirewall
	}
	opts := linodego.FirewallRulesUpdateOptions{
		Inbound:        fwConfig.Inbound,
		InboundPolicy:  fwConfig.InboundPolicy,
		Outbound:       fwConfig.Outbound,
		OutboundPolicy: fwConfig.OutboundPolicy,
	}
	if _, err = fwScope.LinodeClient.UpdateFirewallRules(ctx, linodeFW.ID, opts); err != nil {
		logger.Info("Failed to update firewall rules", "error", err.Error())

		return nil, err
	}
	return linodeFW, nil
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
func processRule(
	ctx context.Context,
	k8sClient clients.K8sClient,
	firewall *infrav1alpha2.LinodeFirewall,
	log logr.Logger,
	fwRuleSpec infrav1alpha2.FirewallRuleSpec,
	ruleType string,
	rules *linodego.FirewallRules,
) (*linodego.FirewallRules, error) {
	ruleIPv4s := make([]string, 0)
	ruleIPv6s := make([]string, 0)
	if fwRuleSpec.Addresses != nil {
		ipv4s, ipv6s := processAddresses(fwRuleSpec.Addresses)
		ruleIPv4s = append(ruleIPv4s, ipv4s...)
		ruleIPv6s = append(ruleIPv6s, ipv6s...)
	}
	if fwRuleSpec.AddressSetRefs != nil {
		ipv4s, ipv6s, err := processAddressSetRefs(ctx, k8sClient, firewall, fwRuleSpec.AddressSetRefs, log)
		if err != nil {
			return nil, err
		}
		ruleIPv4s = append(ruleIPv4s, ipv4s...)
		ruleIPv6s = append(ruleIPv6s, ipv6s...)
	}
	ruleIPv4s, ruleIPv6s = filterDuplicates(ruleIPv4s, ruleIPv6s)

	return processIPRules(ruleIPv4s, ruleIPv6s, fwRuleSpec, rules, ruleType), nil
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
func processIPRules(ipv4s, ipv6s []string, fwRuleSpec infrav1alpha2.FirewallRuleSpec, rules *linodego.FirewallRules, ruleType string) *linodego.FirewallRules {
	// Initialize rules if nil
	if rules == nil {
		rules = &linodego.FirewallRules{}
	}

	// If no IPs, return early
	if len(ipv4s) == 0 && len(ipv6s) == 0 {
		return rules
	}

	ipv4chunks := chunkIPs(ipv4s)
	ipv6chunks := chunkIPs(ipv6s)
	ruleLabel := formatRuleLabel(fwRuleSpec.Action, fwRuleSpec.Label)

	switch ruleType {
	case ruleTypeInbound:
		for i, chunk := range ipv4chunks {
			rules.Inbound = append(rules.Inbound, linodego.FirewallRuleInbound{
				Action:      fwRuleSpec.Action,
				Label:       ruleLabel,
				Description: fmt.Sprintf("Rule %d, Created by CAPL: %s", i, fwRuleSpec.Label),
				Protocol:    fwRuleSpec.Protocol,
				Ports:       fwRuleSpec.Ports,
				Addresses:   linodego.NetworkAddresses{IPv4: chunk},
			})
		}
		for i, chunk := range ipv6chunks {
			rules.Inbound = append(rules.Inbound, linodego.FirewallRuleInbound{
				Action:      fwRuleSpec.Action,
				Label:       ruleLabel,
				Description: fmt.Sprintf("Rule %d, Created by CAPL: %s", i, fwRuleSpec.Label),
				Protocol:    fwRuleSpec.Protocol,
				Ports:       fwRuleSpec.Ports,
				Addresses:   linodego.NetworkAddresses{IPv6: chunk},
			})
		}
	case ruleTypeOutbound:
		for i, chunk := range ipv4chunks {
			rules.Outbound = append(rules.Outbound, linodego.FirewallRuleOutbound{
				Action:      fwRuleSpec.Action,
				Label:       ruleLabel,
				Description: fmt.Sprintf("Rule %d, Created by CAPL: %s", i, fwRuleSpec.Label),
				Protocol:    fwRuleSpec.Protocol,
				Ports:       fwRuleSpec.Ports,
				Addresses:   linodego.NetworkAddresses{IPv4: chunk},
			})
		}
		for i, chunk := range ipv6chunks {
			rules.Outbound = append(rules.Outbound, linodego.FirewallRuleOutbound{
				Action:      fwRuleSpec.Action,
				Label:       ruleLabel,
				Description: fmt.Sprintf("Rule %d, Created by CAPL: %s", i, fwRuleSpec.Label),
				Protocol:    fwRuleSpec.Protocol,
				Ports:       fwRuleSpec.Ports,
				Addresses:   linodego.NetworkAddresses{IPv6: chunk},
			})
		}
	}

	return rules
}

func processFirewallRule(
	ctx context.Context,
	k8sClient clients.K8sClient,
	firewall *infrav1alpha2.LinodeFirewall,
	log logr.Logger,
	ruleRef *corev1.ObjectReference,
	ruleType string,
	rules *linodego.FirewallRules,
) (*linodego.FirewallRules, error) {
	rule := &infrav1alpha2.FirewallRule{}
	if ruleRef.Namespace == "" {
		ruleRef.Namespace = firewall.Namespace
	}
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: ruleRef.Namespace, Name: ruleRef.Name}, rule); err != nil {
		log.Error(err, "failed to get FirewallRule", "namespace", ruleRef.Namespace, "name", ruleRef.Name)

		return nil, err
	}
	return processRule(ctx, k8sClient, firewall, log, rule.Spec, ruleType, rules)
}

// processACL uses the CAPL LinodeFirewall representation to build out the inbound
// and outbound rules for a linode Cloud Firewall
func processACL(ctx context.Context, k8sClient clients.K8sClient, log logr.Logger, firewall *infrav1alpha2.LinodeFirewall) (*linodego.FirewallRules, error) {
	rules := &linodego.FirewallRules{}
	var err error

	// Process inbound rules
	for _, rule := range firewall.Spec.InboundRules {
		rules, err = processRule(ctx, k8sClient, firewall, log, rule, ruleTypeInbound, rules)
		if err != nil {
			return nil, err
		}
	}
	for _, ruleRef := range firewall.Spec.InboundRuleRefs {
		rules, err = processFirewallRule(ctx, k8sClient, firewall, log, ruleRef, ruleTypeInbound, rules)
		if err != nil {
			return nil, err
		}
	}

	// Set inbound policy
	if firewall.Spec.InboundPolicy == "" {
		rules.InboundPolicy = "ACCEPT"
	} else {
		rules.InboundPolicy = firewall.Spec.InboundPolicy
	}

	// Process outbound rules
	for _, rule := range firewall.Spec.OutboundRules {
		rules, err = processRule(ctx, k8sClient, firewall, log, rule, ruleTypeOutbound, rules)
		if err != nil {
			return nil, err
		}
	}
	for _, ruleRef := range firewall.Spec.OutboundRuleRefs {
		rules, err = processFirewallRule(ctx, k8sClient, firewall, log, ruleRef, ruleTypeOutbound, rules)
		if err != nil {
			return nil, err
		}
	}

	// Set outbound policy
	if firewall.Spec.OutboundPolicy == "" {
		rules.OutboundPolicy = "ACCEPT"
	} else {
		rules.OutboundPolicy = firewall.Spec.OutboundPolicy
	}

	// Check rule count
	if len(rules.Inbound)+len(rules.Outbound) > maxRulesPerFirewall {
		return nil, errTooManyIPs
	}

	return rules, nil
}
