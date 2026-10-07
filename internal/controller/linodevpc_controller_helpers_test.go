package controller

import (
	"context"
	"reflect"
	"testing"

	"github.com/linode/linodego/v2"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
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
			name: "no ipv6 or ipv4 ranges",
			args: args{
				vpcSpec: infrav1alpha2.LinodeVPCSpec{
					Description: "description",
					Region:      "region",
					Subnets: []infrav1alpha2.VPCSubnetCreateOptions{
						{
							Label: "subnet",
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
						IPv6:  []linodego.VPCSubnetCreateOptionsIPv6{},
					},
				},
				IPv6: []linodego.VPCCreateOptionsIPv6{},
			},
		},
		{
			name: "BYO ipv4 range",
			args: args{
				vpcSpec: infrav1alpha2.LinodeVPCSpec{
					Description: "description",
					Region:      "region",
					IPv4Range:   []string{"10.0.0.0/8"},
					IPv6Range: []infrav1alpha2.VPCCreateOptionsIPv6{
						{
							Range: new("2001:db8::/52"),
						},
					},
					Subnets: []infrav1alpha2.VPCSubnetCreateOptions{
						{
							Label: "subnet",
							IPv6Range: []infrav1alpha2.VPCSubnetCreateOptionsIPv6{
								{
									Range: new("2001:db8:1::/56"),
								},
							},
							IPv4: "10.1.2.0/24",
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
						IPv6: []linodego.VPCSubnetCreateOptionsIPv6{
							{
								Range: new("2001:db8:1::/56"),
							},
						},
						IPv4: "10.1.2.0/24",
					},
				},
				IPv6: []linodego.VPCCreateOptionsIPv6{
					{
						Range: new("2001:db8::/52"),
					},
				},
				IPv4: []linodego.VPCCreateOptionsIPv4{
					{
						Range: new("10.0.0.0/8"),
					},
				},
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
							Range: new("2001:db8::/52"),
						},
					},
					Subnets: []infrav1alpha2.VPCSubnetCreateOptions{
						{
							Label: "subnet",
							IPv6Range: []infrav1alpha2.VPCSubnetCreateOptionsIPv6{
								{
									Range: new("2001:db8:1::/56"),
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
						IPv6: []linodego.VPCSubnetCreateOptionsIPv6{
							{
								Range: new("2001:db8:1::/56"),
							},
						},
					},
				},
				IPv6: []linodego.VPCCreateOptionsIPv6{
					{
						Range: new("2001:db8::/52"),
					},
				},
			},
		},
		{
			name: "ipv6 ranges with AllocationClassLegacy only",
			args: args{
				vpcSpec: infrav1alpha2.LinodeVPCSpec{
					Description: "description",
					Region:      "region",
					IPv6Range: []infrav1alpha2.VPCCreateOptionsIPv6{
						{
							Range:                 new("2001:db8::/52"),
							AllocationClassLegacy: new("myclass_legacy"),
						},
					},
					Subnets: []infrav1alpha2.VPCSubnetCreateOptions{
						{
							Label: "subnet",
							IPv6Range: []infrav1alpha2.VPCSubnetCreateOptionsIPv6{
								{
									Range: new("2001:db8:1::/56"),
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
						IPv6: []linodego.VPCSubnetCreateOptionsIPv6{
							{
								Range: new("2001:db8:1::/56"),
							},
						},
					},
				},
				IPv6: []linodego.VPCCreateOptionsIPv6{
					{
						Range:           new("2001:db8::/52"),
						AllocationClass: new("myclass_legacy"),
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
							Range:           new("2001:db8::/52"),
							AllocationClass: new("myclass"),
						},
					},
					Subnets: []infrav1alpha2.VPCSubnetCreateOptions{
						{
							Label: "subnet",
							IPv6Range: []infrav1alpha2.VPCSubnetCreateOptionsIPv6{
								{
									Range: new("2001:db8:1::/56"),
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
						IPv6: []linodego.VPCSubnetCreateOptionsIPv6{
							{
								Range: new("2001:db8:1::/56"),
							},
						},
					},
				},
				IPv6: []linodego.VPCCreateOptionsIPv6{
					{
						Range:           new("2001:db8::/52"),
						AllocationClass: new("myclass"),
					},
				},
			},
		},
		{
			name: "ipv6 ranges with AllocationClass and AllocationClassLegacy",
			args: args{
				vpcSpec: infrav1alpha2.LinodeVPCSpec{
					Description: "description",
					Region:      "region",
					IPv6Range: []infrav1alpha2.VPCCreateOptionsIPv6{
						{
							Range:                 new("2001:db8::/52"),
							AllocationClass:       new("myclass"),
							AllocationClassLegacy: new("myclass_legacy"),
						},
					},
					Subnets: []infrav1alpha2.VPCSubnetCreateOptions{
						{
							Label: "subnet",
							IPv6Range: []infrav1alpha2.VPCSubnetCreateOptionsIPv6{
								{
									Range: new("2001:db8:1::/56"),
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
						IPv6: []linodego.VPCSubnetCreateOptionsIPv6{
							{
								Range: new("2001:db8:1::/56"),
							},
						},
					},
				},
				IPv6: []linodego.VPCCreateOptionsIPv6{
					{
						Range:           new("2001:db8::/52"),
						AllocationClass: new("myclass"),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := linodeVPCSpecToVPCCreateConfig(tt.args.vpcSpec); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("linodeVPCSpecToVPCCreateConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReconcileVPC_VPCTypeMismatch(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinodeClient := mock.NewMockLinodeClient(ctrl)
	logger := zap.New()

	rdmaType := linodego.VPCTypeRDMA
	vpcScope := &scope.VPCScope{
		LinodeClient: mockLinodeClient,
		LinodeVPC: &infrav1alpha2.LinodeVPC{
			ObjectMeta: metav1.ObjectMeta{Name: "test-vpc", Namespace: "default"},
			Spec: infrav1alpha2.LinodeVPCSpec{
				Region:  "us-ord",
				VPCType: rdmaType,
			},
		},
	}

	// ListVPCs returns a regular VPC even though spec requires rdma.
	mockLinodeClient.EXPECT().ListVPCs(gomock.Any(), gomock.Any()).Return([]linodego.VPC{
		{ID: 1, Label: "test-vpc", VPCType: linodego.VPCTypeRegular},
	}, nil)

	err := reconcileVPC(context.Background(), vpcScope, logger)
	if err == nil {
		t.Fatal("expected error due to VPC type mismatch, got nil")
	}
	expected := `existing VPC "test-vpc" has type "regular" but vpcType "rdma" is required`
	if err.Error() != expected {
		t.Errorf("unexpected error message:\ngot:  %q\nwant: %q", err.Error(), expected)
	}
}

func TestReconcileVPC_MatchingRDMATypeAdopted(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinodeClient := mock.NewMockLinodeClient(ctrl)
	logger := zap.New()

	rdmaType := linodego.VPCTypeRDMA
	vpcID := 1
	vpcScope := &scope.VPCScope{
		LinodeClient: mockLinodeClient,
		LinodeVPC: &infrav1alpha2.LinodeVPC{
			ObjectMeta: metav1.ObjectMeta{Name: "rdma-vpc", Namespace: "default"},
			Spec: infrav1alpha2.LinodeVPCSpec{
				Region:  "us-ord",
				VPCType: rdmaType,
			},
		},
	}

	// ListVPCs returns an RDMA VPC matching the spec type — adoption should proceed.
	mockLinodeClient.EXPECT().ListVPCs(gomock.Any(), gomock.Any()).Return([]linodego.VPC{
		{ID: vpcID, Label: "rdma-vpc", VPCType: linodego.VPCTypeRDMA},
	}, nil)

	err := reconcileVPC(context.Background(), vpcScope, logger)
	if err != nil {
		t.Fatalf("expected no error when VPC type matches, got: %v", err)
	}
	if vpcScope.LinodeVPC.Spec.VPCID == nil || *vpcScope.LinodeVPC.Spec.VPCID != vpcID {
		t.Errorf("expected VPCID to be set to %d after adoption", vpcID)
	}
}
