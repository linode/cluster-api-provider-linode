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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

var (
	errTooManyIPs = errors.New("too many IPs in this ACL, will exceed rules per firewall limit")
)

func findObjectsForAddressSet(logger logr.Logger, tracedClient client.Client) handler.MapFunc {
	logger = logger.WithName("LinodeFirewallReconciler").WithName("findObjectsForAddressSet")
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
// AddressSet in the Inbound and/or OutboundRules
func buildRequests(firewalls []infrav1alpha2.LinodeFirewall, obj client.Object) []reconcile.Request {
	requestSet := make(map[reconcile.Request]struct{})
	for _, firewall := range firewalls {
		for _, inboundRule := range firewall.Spec.InboundRules {
			requestSet = buildRequestsHelper(requestSet, firewall, inboundRule.AddressSetRefs, obj)
		}
		for _, outboundRule := range firewall.Spec.OutboundRules {
			requestSet = buildRequestsHelper(requestSet, firewall, outboundRule.AddressSetRefs, obj)
		}
	}

	return slices.Collect(maps.Keys(requestSet))
}

func buildRequestsHelper(requestSet map[reconcile.Request]struct{}, firewall infrav1alpha2.LinodeFirewall, addrSetRefs []*corev1.ObjectReference, obj client.Object) map[reconcile.Request]struct{} {
	for _, addrSetRef := range addrSetRefs {
		if addrSetRef.Name == obj.GetName() && addrSetRef.Namespace == obj.GetNamespace() {
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

// processInboundRule handles a single inbound rule
func processInboundRule(ctx context.Context, k8sClient clients.K8sClient, log logr.Logger, rule infrav1alpha2.FirewallRule, lfw *infrav1alpha2.LinodeFirewall, createOpts *linodego.FirewallCreateOptions) error {
	var ruleIPv4s []string
	var ruleIPv6s []string
	var err error
	if rule.Addresses != nil {
		ruleIPv4s, ruleIPv6s = processAddresses(rule.Addresses)
	}
	if rule.AddressSetRefs != nil {
		ruleIPv4s, ruleIPv6s, err = processAddressSetRefs(ctx, k8sClient, lfw, rule.AddressSetRefs, log)
		if err != nil {
			return err
		}
	}
	ruleLabel := formatRuleLabel(rule.Action, rule.Label)

	// Process IPv4
	processIPv4Rules(ruleIPv4s, rule, ruleLabel, &createOpts.Rules.Inbound)

	// Process IPv6
	processIPv6Rules(ruleIPv6s, rule, ruleLabel, &createOpts.Rules.Inbound)

	return nil
}

// processOutboundRule handles a single outbound rule
func processOutboundRule(ctx context.Context, k8sClient clients.K8sClient, log logr.Logger, rule infrav1alpha2.FirewallRule, lfw *infrav1alpha2.LinodeFirewall, createOpts *linodego.FirewallCreateOptions) error {
	var ruleIPv4s []string
	var ruleIPv6s []string
	var err error
	if rule.Addresses != nil {
		ruleIPv4s, ruleIPv6s = processAddresses(rule.Addresses)
	}
	if rule.AddressSetRefs != nil {
		ruleIPv4s, ruleIPv6s, err = processAddressSetRefs(ctx, k8sClient, lfw, rule.AddressSetRefs, log)
		if err != nil {
			return err
		}
	}
	ruleLabel := formatRuleLabel(lfw.Spec.OutboundPolicy, rule.Label)

	// Process IPv4
	processIPv4Rules(ruleIPv4s, rule, ruleLabel, &createOpts.Rules.Outbound)

	// Process IPv6
	processIPv6Rules(ruleIPv6s, rule, ruleLabel, &createOpts.Rules.Outbound)

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

// get finalizer for Firewall that references the AddressSet
func getFinalizer(lfw *infrav1alpha2.LinodeFirewall) string {
	return fmt.Sprintf("lfw.%s.%s", lfw.Namespace, lfw.Name)
}

// processAddressSetRefs extracts and transforms IPv4 and IPv6 addresses from the reference AddressSet(s)
func processAddressSetRefs(ctx context.Context, k8sClient clients.K8sClient, lfw *infrav1alpha2.LinodeFirewall, addressSetRefs []*corev1.ObjectReference, log logr.Logger) (ipv4s, ipv6s []string, err error) {
	// Initialize empty slices for consistent return type
	ipv4s = make([]string, 0)
	ipv6s = make([]string, 0)
	// Declare "sets". Empty structs occupy 0 memory
	ipv4Set := make(map[string]struct{})
	ipv6Set := make(map[string]struct{})

	for _, addrSetRef := range addressSetRefs {
		addrSet := &infrav1alpha2.AddressSet{}
		if addrSetRef.Namespace == "" {
			addrSetRef.Namespace = lfw.Namespace
		}
		if k8sClient != nil {
			if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: addrSetRef.Namespace, Name: addrSetRef.Name}, addrSet); err != nil {
				log.Error(err, "failed to fetch referenced AddressSet", "namespace", addrSetRef.Namespace, "name", addrSetRef.Name)
				return ipv4s, ipv6s, err
			}
			finalizer := getFinalizer(lfw)
			if !controllerutil.ContainsFinalizer(addrSet, finalizer) {
				controllerutil.AddFinalizer(addrSet, finalizer)
				if err := k8sClient.Update(ctx, addrSet); err != nil {
					return ipv4s, ipv6s, err
				}
			}
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

// processIPv4Rules processes IPv4 rules and adds them to the rules slice
func processIPv4Rules(ips []string, rule infrav1alpha2.FirewallRule, ruleLabel string, rules *[]linodego.FirewallRule) {
	// Initialize rules if nil
	if *rules == nil {
		*rules = make([]linodego.FirewallRule, 0)
	}

	// If no IPs, return early
	if len(ips) == 0 {
		return
	}

	ipv4chunks := chunkIPs(ips)
	for i, chunk := range ipv4chunks {
		v4chunk := chunk
		*rules = append(*rules, linodego.FirewallRule{
			Action:      rule.Action,
			Label:       ruleLabel,
			Description: fmt.Sprintf("Rule %d, Created by CAPL: %s", i, rule.Label),
			Protocol:    rule.Protocol,
			Ports:       rule.Ports,
			Addresses:   linodego.NetworkAddresses{IPv4: &v4chunk},
		})
	}
}

// processIPv6Rules processes IPv6 rules and adds them to the rules slice
func processIPv6Rules(ips []string, rule infrav1alpha2.FirewallRule, ruleLabel string, rules *[]linodego.FirewallRule) {
	// Initialize rules if nil
	if *rules == nil {
		*rules = make([]linodego.FirewallRule, 0)
	}

	// If no IPs, return early
	if len(ips) == 0 {
		return
	}

	ipv6chunks := chunkIPs(ips)
	for i, chunk := range ipv6chunks {
		v6chunk := chunk
		*rules = append(*rules, linodego.FirewallRule{
			Action:      rule.Action,
			Label:       ruleLabel,
			Description: fmt.Sprintf("Rule %d, Created by CAPL: %s", i, rule.Label),
			Protocol:    rule.Protocol,
			Ports:       rule.Ports,
			Addresses:   linodego.NetworkAddresses{IPv6: &v6chunk},
		})
	}
}

// processACL uses the CAPL LinodeFirewall representation to build out the inbound
// and outbound rules for a linode Cloud Firewall
func processACL(ctx context.Context, k8sClient clients.K8sClient, log logr.Logger, firewall *infrav1alpha2.LinodeFirewall) (*linodego.FirewallCreateOptions, error) {
	createOpts := &linodego.FirewallCreateOptions{
		Label: firewall.Name,
	}

	// Process inbound rules
	for _, rule := range firewall.Spec.InboundRules {
		if err := processInboundRule(ctx, k8sClient, log, rule, firewall, createOpts); err != nil {
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
		if err := processOutboundRule(ctx, k8sClient, log, rule, firewall, createOpts); err != nil {
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
