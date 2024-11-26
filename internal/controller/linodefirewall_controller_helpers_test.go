package controller

import (
	"context"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
)

func TestTransformToCIDR(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "IPv4 single address",
			input:    "192.168.1.1",
			expected: "192.168.1.1/32",
		},
		{
			name:     "IPv4 CIDR notation",
			input:    "192.168.1.0/24",
			expected: "192.168.1.0/24",
		},
		{
			name:     "IPv6 single address",
			input:    "2001:db8::1",
			expected: "2001:db8::1/128",
		},
		{
			name:     "IPv6 CIDR notation",
			input:    "2001:db8::/32",
			expected: "2001:db8::/32",
		},
		{
			name:     "Invalid IP",
			input:    "invalid-ip",
			expected: "invalid-ip",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := transformToCIDR(tt.input)
			if result != tt.expected {
				t.Errorf("transformToCIDR(%s) = %s; want %s",
					tt.input, result, tt.expected)
			}
		})
	}
}

func TestProcessACL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		firewall *infrav1alpha2.LinodeFirewall
		want     *linodego.FirewallCreateOptions
		wantErr  bool
	}{
		{
			name: "Single IP addresses are converted to CIDR",
			firewall: &infrav1alpha2.LinodeFirewall{
				Spec: infrav1alpha2.LinodeFirewallSpec{
					InboundRules: []infrav1alpha2.FirewallRule{
						{
							Action:   "ACCEPT",
							Label:    "test-rule",
							Protocol: "TCP",
							Ports:    "80",
							Addresses: &infrav1alpha2.NetworkAddresses{
								IPv4: &[]string{"192.168.1.1"},
								IPv6: &[]string{"2001:db8::1"},
							},
						},
					},
				},
			},
			want: &linodego.FirewallCreateOptions{
				Rules: linodego.FirewallRuleSet{
					Inbound: []linodego.FirewallRule{
						{
							Action:      "ACCEPT",
							Label:       "ACCEPT-test-rule",
							Description: "Rule 0, Created by CAPL: test-rule",
							Protocol:    "TCP",
							Ports:       "80",
							Addresses: linodego.NetworkAddresses{
								IPv4: &[]string{"192.168.1.1/32"},
							},
						},
						{
							Action:      "ACCEPT",
							Label:       "ACCEPT-test-rule",
							Description: "Rule 0, Created by CAPL: test-rule",
							Protocol:    "TCP",
							Ports:       "80",
							Addresses: linodego.NetworkAddresses{
								IPv6: &[]string{"2001:db8::1/128"},
							},
						},
					},
					InboundPolicy: "ACCEPT",
				},
			},
			wantErr: false,
		},
		{
			name: "Mixed single IPs and CIDR notation",
			firewall: &infrav1alpha2.LinodeFirewall{
				Spec: infrav1alpha2.LinodeFirewallSpec{
					InboundRules: []infrav1alpha2.FirewallRule{
						{
							Action:   "ACCEPT",
							Label:    "test-rule",
							Protocol: "TCP",
							Ports:    "80",
							Addresses: &infrav1alpha2.NetworkAddresses{
								IPv4: &[]string{
									"192.168.1.1",
									"10.0.0.0/8",
								},
							},
						},
					},
				},
			},
			want: &linodego.FirewallCreateOptions{
				Rules: linodego.FirewallRuleSet{
					Inbound: []linodego.FirewallRule{
						{
							Action:      "ACCEPT",
							Label:       "ACCEPT-test-rule",
							Description: "Rule 0, Created by CAPL: test-rule",
							Protocol:    "TCP",
							Ports:       "80",
							Addresses: linodego.NetworkAddresses{
								IPv4: &[]string{
									"192.168.1.1/32",
									"10.0.0.0/8",
								},
							},
						},
					},
					InboundPolicy: "ACCEPT",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logger := logr.Logger{}

			got, err := processACL(context.Background(), k8sClient, logger, tt.firewall)
			if (err != nil) != tt.wantErr {
				t.Errorf("processACL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Compare the structures field by field for better error messages
			if !reflect.DeepEqual(got.Rules.InboundPolicy, tt.want.Rules.InboundPolicy) {
				t.Errorf("processACL() InboundPolicy = %v, want %v",
					got.Rules.InboundPolicy, tt.want.Rules.InboundPolicy)
			}

			if len(got.Rules.Inbound) != len(tt.want.Rules.Inbound) {
				t.Errorf("processACL() number of Inbound rules = %d, want %d",
					len(got.Rules.Inbound), len(tt.want.Rules.Inbound))
				return
			}

			for i := range got.Rules.Inbound {
				if (tt.want.Rules.Inbound[i].Addresses.IPv4 != nil && !assert.ElementsMatch(t, *got.Rules.Inbound[i].Addresses.IPv4, *tt.want.Rules.Inbound[i].Addresses.IPv4)) ||
					(tt.want.Rules.Inbound[i].Addresses.IPv6 != nil && !assert.ElementsMatch(t, *got.Rules.Inbound[i].Addresses.IPv6, *tt.want.Rules.Inbound[i].Addresses.IPv6)) ||
					!cmp.Equal(got.Rules.Inbound[i], tt.want.Rules.Inbound[i], cmpopts.IgnoreFields(linodego.NetworkAddresses{}, "IPv4", "IPv6")) {
					t.Errorf("processACL() Inbound rule %d = %+v, want %+v",
						i, got.Rules.Inbound[i], tt.want.Rules.Inbound[i])
				}
			}
		})
	}
}

func TestProcessAddresses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		addresses *infrav1alpha2.NetworkAddresses
		wantIPv4  []string
		wantIPv6  []string
	}{
		{
			name: "Both IPv4 and IPv6 addresses",
			addresses: &infrav1alpha2.NetworkAddresses{
				IPv4: &[]string{"192.168.1.1", "10.0.0.0/8"},
				IPv6: &[]string{"2001:db8::1", "2001:db8::/32"},
			},
			wantIPv4: []string{"192.168.1.1/32", "10.0.0.0/8"},
			wantIPv6: []string{"2001:db8::1/128", "2001:db8::/32"},
		},
		{
			name: "Only IPv4 addresses",
			addresses: &infrav1alpha2.NetworkAddresses{
				IPv4: &[]string{"192.168.1.1", "172.16.0.0/12"},
			},
			wantIPv4: []string{"192.168.1.1/32", "172.16.0.0/12"},
			wantIPv6: []string{},
		},
		{
			name: "Only IPv6 addresses",
			addresses: &infrav1alpha2.NetworkAddresses{
				IPv6: &[]string{"2001:db8::1", "2001:db8::/32"},
			},
			wantIPv4: []string{},
			wantIPv6: []string{"2001:db8::1/128", "2001:db8::/32"},
		},
		{
			name:      "Empty addresses",
			addresses: &infrav1alpha2.NetworkAddresses{},
			wantIPv4:  []string{},
			wantIPv6:  []string{},
		},
		{
			name:      "Nil addresses",
			addresses: nil,
			wantIPv4:  []string{},
			wantIPv6:  []string{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotIPv4, gotIPv6 := processAddresses(tt.addresses)
			if !assert.ElementsMatch(t, gotIPv4, tt.wantIPv4) {
				t.Errorf("processAddresses() IPv4 = %v, want %v", gotIPv4, tt.wantIPv4)
			}
			if !assert.ElementsMatch(t, gotIPv6, tt.wantIPv6) {
				t.Errorf("processAddresses() IPv6 = %v, want %v", gotIPv6, tt.wantIPv6)
			}
		})
	}
}

func TestFormatRuleLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prefix string
		label  string
		want   string
	}{
		{
			name:   "Short label",
			prefix: "ACCEPT",
			label:  "test-rule",
			want:   "ACCEPT-test-rule",
		},
		{
			name:   "Label exactly max length",
			prefix: "ACCEPT",
			label:  "test-rule-exactly-32-chars-long",
			want:   "ACCEPT-test-rule-exactly-32-char",
		},
		{
			name:   "Label exceeds max length",
			prefix: "ACCEPT",
			label:  "test-rule-that-is-way-too-long-and-should-be-truncated",
			want:   "ACCEPT-test-rule-that-is-way-too",
		},
		{
			name:   "Empty prefix",
			prefix: "",
			label:  "test-rule",
			want:   "-test-rule",
		},
		{
			name:   "Empty label",
			prefix: "ACCEPT",
			label:  "",
			want:   "ACCEPT-",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatRuleLabel(tt.prefix, tt.label)
			if got != tt.want {
				t.Errorf("formatRuleLabel() = %v, want %v", got, tt.want)
			}
			if len(got) > maxFirewallRuleLabelLen {
				t.Errorf("formatRuleLabel() length = %d, want <= %d", len(got), maxFirewallRuleLabelLen)
			}
		})
	}
}

func TestProcessIPv4Rules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		ips       []string
		rule      infrav1alpha2.FirewallRule
		ruleLabel string
		want      []linodego.FirewallRule
	}{
		{
			name: "Single IPv4 address",
			ips:  []string{"192.168.1.1/32"},
			rule: infrav1alpha2.FirewallRule{
				Action:   "ACCEPT",
				Protocol: "TCP",
				Ports:    "80",
				Label:    "test-rule",
			},
			ruleLabel: "ACCEPT-test",
			want: []linodego.FirewallRule{
				{
					Action:      "ACCEPT",
					Label:       "ACCEPT-test",
					Description: "Rule 0, Created by CAPL: test-rule",
					Protocol:    "TCP",
					Ports:       "80",
					Addresses:   linodego.NetworkAddresses{IPv4: &[]string{"192.168.1.1/32"}},
				},
			},
		},
		{
			name: "Multiple IPv4 addresses within limit",
			ips:  []string{"192.168.1.1/32", "10.0.0.0/8"},
			rule: infrav1alpha2.FirewallRule{
				Action:   "DROP",
				Protocol: "UDP",
				Ports:    "53",
				Label:    "test-rule",
			},
			ruleLabel: "DROP-test",
			want: []linodego.FirewallRule{
				{
					Action:      "DROP",
					Label:       "DROP-test",
					Description: "Rule 0, Created by CAPL: test-rule",
					Protocol:    "UDP",
					Ports:       "53",
					Addresses:   linodego.NetworkAddresses{IPv4: &[]string{"192.168.1.1/32", "10.0.0.0/8"}},
				},
			},
		},
		{
			name: "Empty IP list",
			ips:  []string{},
			rule: infrav1alpha2.FirewallRule{
				Action:   "ACCEPT",
				Protocol: "TCP",
				Ports:    "80",
				Label:    "test-rule",
			},
			ruleLabel: "ACCEPT-test",
			want:      []linodego.FirewallRule{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got []linodego.FirewallRule
			processIPv4Rules(tt.ips, tt.rule, tt.ruleLabel, &got)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("processIPv4Rules() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcessIPv6Rules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		ips       []string
		rule      infrav1alpha2.FirewallRule
		ruleLabel string
		want      []linodego.FirewallRule
	}{
		{
			name: "Single IPv6 address",
			ips:  []string{"2001:db8::1/128"},
			rule: infrav1alpha2.FirewallRule{
				Action:   "ACCEPT",
				Protocol: "TCP",
				Ports:    "80",
				Label:    "test-rule",
			},
			ruleLabel: "ACCEPT-test",
			want: []linodego.FirewallRule{
				{
					Action:      "ACCEPT",
					Label:       "ACCEPT-test",
					Description: "Rule 0, Created by CAPL: test-rule",
					Protocol:    "TCP",
					Ports:       "80",
					Addresses:   linodego.NetworkAddresses{IPv6: &[]string{"2001:db8::1/128"}},
				},
			},
		},
		{
			name: "Multiple IPv6 addresses within limit",
			ips:  []string{"2001:db8::1/128", "2001:db8::/32"},
			rule: infrav1alpha2.FirewallRule{
				Action:   "DROP",
				Protocol: "UDP",
				Ports:    "53",
				Label:    "test-rule",
			},
			ruleLabel: "DROP-test",
			want: []linodego.FirewallRule{
				{
					Action:      "DROP",
					Label:       "DROP-test",
					Description: "Rule 0, Created by CAPL: test-rule",
					Protocol:    "UDP",
					Ports:       "53",
					Addresses:   linodego.NetworkAddresses{IPv6: &[]string{"2001:db8::1/128", "2001:db8::/32"}},
				},
			},
		},
		{
			name: "Empty IP list",
			ips:  []string{},
			rule: infrav1alpha2.FirewallRule{
				Action:   "ACCEPT",
				Protocol: "TCP",
				Ports:    "80",
				Label:    "test-rule",
			},
			ruleLabel: "ACCEPT-test",
			want:      []linodego.FirewallRule{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got []linodego.FirewallRule
			processIPv6Rules(tt.ips, tt.rule, tt.ruleLabel, &got)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("processIPv6Rules() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcessInboundRule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		firewall   *infrav1alpha2.LinodeFirewall
		createOpts *linodego.FirewallCreateOptions
		want       *linodego.FirewallCreateOptions
	}{
		{
			name: "Process inbound rule with IPv4 and IPv6",
			firewall: &infrav1alpha2.LinodeFirewall{
				Spec: infrav1alpha2.LinodeFirewallSpec{
					InboundRules: []infrav1alpha2.FirewallRule{{
						Action:   "ACCEPT",
						Label:    "test-rule",
						Protocol: "TCP",
						Ports:    "80",
						Addresses: &infrav1alpha2.NetworkAddresses{
							IPv4: &[]string{"192.168.1.1"},
							IPv6: &[]string{"2001:db8::1"},
						},
					}},
				},
			},
			createOpts: &linodego.FirewallCreateOptions{
				Rules: linodego.FirewallRuleSet{},
			},
			want: &linodego.FirewallCreateOptions{
				Rules: linodego.FirewallRuleSet{
					Inbound: []linodego.FirewallRule{
						{
							Action:      "ACCEPT",
							Label:       "ACCEPT-test-rule",
							Description: "Rule 0, Created by CAPL: test-rule",
							Protocol:    "TCP",
							Ports:       "80",
							Addresses:   linodego.NetworkAddresses{IPv4: &[]string{"192.168.1.1/32"}},
						},
						{
							Action:      "ACCEPT",
							Label:       "ACCEPT-test-rule",
							Description: "Rule 0, Created by CAPL: test-rule",
							Protocol:    "TCP",
							Ports:       "80",
							Addresses:   linodego.NetworkAddresses{IPv6: &[]string{"2001:db8::1/128"}},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logger := logr.Logger{}
			for _, rule := range tt.firewall.Spec.InboundRules {
				err := processInboundRule(context.Background(), k8sClient, logger, rule, tt.firewall, tt.createOpts)
				require.NoError(t, err)
				if !reflect.DeepEqual(tt.createOpts, tt.want) {
					t.Errorf("processInboundRule() = %v, want %v", tt.createOpts, tt.want)
				}
			}
		})
	}
}

func TestProcessOutboundRule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		firewall   *infrav1alpha2.LinodeFirewall
		createOpts *linodego.FirewallCreateOptions
		want       *linodego.FirewallCreateOptions
	}{
		{
			name: "Process outbound rule with IPv4 and IPv6",
			firewall: &infrav1alpha2.LinodeFirewall{
				Spec: infrav1alpha2.LinodeFirewallSpec{
					OutboundRules: []infrav1alpha2.FirewallRule{{
						Action:   "ACCEPT",
						Label:    "test-rule",
						Protocol: "TCP",
						Ports:    "80",
						Addresses: &infrav1alpha2.NetworkAddresses{
							IPv4: &[]string{"192.168.1.1"},
							IPv6: &[]string{"2001:db8::1"},
						},
					}},
					OutboundPolicy: "DROP",
				},
			},
			createOpts: &linodego.FirewallCreateOptions{
				Rules: linodego.FirewallRuleSet{},
			},
			want: &linodego.FirewallCreateOptions{
				Rules: linodego.FirewallRuleSet{
					Outbound: []linodego.FirewallRule{
						{
							Action:      "ACCEPT",
							Label:       "DROP-test-rule",
							Description: "Rule 0, Created by CAPL: test-rule",
							Protocol:    "TCP",
							Ports:       "80",
							Addresses:   linodego.NetworkAddresses{IPv4: &[]string{"192.168.1.1/32"}},
						},
						{
							Action:      "ACCEPT",
							Label:       "DROP-test-rule",
							Description: "Rule 0, Created by CAPL: test-rule",
							Protocol:    "TCP",
							Ports:       "80",
							Addresses:   linodego.NetworkAddresses{IPv6: &[]string{"2001:db8::1/128"}},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logger := logr.Logger{}
			for _, rule := range tt.firewall.Spec.OutboundRules {
				err := processOutboundRule(context.Background(), k8sClient, logger, rule, tt.firewall, tt.createOpts)
				require.NoError(t, err)
				if !reflect.DeepEqual(tt.createOpts, tt.want) {
					t.Errorf("processOutboundRule() = %v, want %v", tt.createOpts, tt.want)
				}
			}
		})
	}
}
