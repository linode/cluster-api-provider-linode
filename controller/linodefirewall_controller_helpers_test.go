package controller

import (
	"reflect"
	"testing"

	"github.com/linode/linodego"

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
			got, err := processACL(tt.firewall)
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
				if !reflect.DeepEqual(got.Rules.Inbound[i], tt.want.Rules.Inbound[i]) {
					t.Errorf("processACL() Inbound rule %d = %+v, want %+v",
						i, got.Rules.Inbound[i], tt.want.Rules.Inbound[i])
				}
			}
		})
	}
}
