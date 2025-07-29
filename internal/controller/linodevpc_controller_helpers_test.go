package controller

import (
	"reflect"
	"testing"

	"github.com/linode/linodego"
	"k8s.io/utils/ptr"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
)

func Test_linodeVPCSpecToVPCCreateConfig(t *testing.T) {
	t.Parallel()
	type args struct {
		vpcSpec infrav1alpha2.LinodeVPCSpec
	}
	tests := []struct {
		name string
		args args
		want *linodego.VPCCreateOptions
	}{
		{
			name: "no ipv6 ranges",
			args: args{
				vpcSpec: infrav1alpha2.LinodeVPCSpec{
					Description: "description",
					Region:      "region",
					Subnets: []infrav1alpha2.VPCSubnetCreateOptions{
						{
							Label: "subnet",
							IPv4:  "ipv4",
						},
					},
				},
			},
			want: &linodego.VPCCreateOptions{
				Description: "description",
				Region:      "region",
				Subnets: []linodego.VPCSubnetCreateOptions{
					{
						Label: "subnet",
						IPv4:  "ipv4",
						IPv6:  []linodego.VPCSubnetCreateOptionsIPv6{},
					},
				},
				IPv6: []linodego.VPCCreateOptionsIPv6{},
			},
		},
		{
			name: "ipv6 ranges without allocation_class",
			args: args{
				vpcSpec: infrav1alpha2.LinodeVPCSpec{
					Description: "description",
					Region:      "region",
					IPv6Range: []infrav1alpha2.VPCCreateOptionsIPv6{
						{
							Range: ptr.To("2001:db8::/52"),
						},
					},
					Subnets: []infrav1alpha2.VPCSubnetCreateOptions{
						{
							Label: "subnet",
							IPv4:  "ipv4",
							IPv6Range: []infrav1alpha2.VPCSubnetCreateOptionsIPv6{
								{
									Range: ptr.To("2001:db8:1::/56"),
								},
							},
						},
					},
				},
			},
			want: &linodego.VPCCreateOptions{
				Description: "description",
				Region:      "region",
				Subnets: []linodego.VPCSubnetCreateOptions{
					{
						Label: "subnet",
						IPv4:  "ipv4",
						IPv6: []linodego.VPCSubnetCreateOptionsIPv6{
							{
								Range: ptr.To("2001:db8:1::/56"),
							},
						},
					},
				},
				IPv6: []linodego.VPCCreateOptionsIPv6{
					{
						Range: ptr.To("2001:db8::/52"),
					},
				},
			},
		},
		{
			name: "ipv6 ranges with allocation_class",
			args: args{
				vpcSpec: infrav1alpha2.LinodeVPCSpec{
					Description: "description",
					Region:      "region",
					IPv6Range: []infrav1alpha2.VPCCreateOptionsIPv6{
						{
							Range:           ptr.To("2001:db8::/52"),
							AllocationClass: ptr.To("myclass"),
						},
					},
					Subnets: []infrav1alpha2.VPCSubnetCreateOptions{
						{
							Label: "subnet",
							IPv4:  "ipv4",
							IPv6Range: []infrav1alpha2.VPCSubnetCreateOptionsIPv6{
								{
									Range: ptr.To("2001:db8:1::/56"),
								},
							},
						},
					},
				},
			},
			want: &linodego.VPCCreateOptions{
				Description: "description",
				Region:      "region",
				Subnets: []linodego.VPCSubnetCreateOptions{
					{
						Label: "subnet",
						IPv4:  "ipv4",
						IPv6: []linodego.VPCSubnetCreateOptionsIPv6{
							{
								Range: ptr.To("2001:db8:1::/56"),
							},
						},
					},
				},
				IPv6: []linodego.VPCCreateOptionsIPv6{
					{
						Range:           ptr.To("2001:db8::/52"),
						AllocationClass: ptr.To("myclass"),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := linodeVPCSpecToVPCCreateConfig(tt.args.vpcSpec); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("linodeVPCSpecToVPCCreateConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
