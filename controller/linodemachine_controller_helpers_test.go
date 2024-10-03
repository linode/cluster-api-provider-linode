package controller

import (
	"bytes"
	"context"
	b64 "encoding/base64"
	"encoding/gob"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/cluster-api-provider-linode/util"
)

func TestLinodeMachineSpecToCreateInstanceConfig(t *testing.T) {
	t.Parallel()

	subnetID := 1

	machineSpec := infrav1alpha2.LinodeMachineSpec{
		Region:          "region",
		Type:            "type",
		Group:           "group",
		RootPass:        "rootPass",
		AuthorizedKeys:  []string{"key"},
		AuthorizedUsers: []string{"user"},
		BackupID:        1,
		Image:           "image",
		Interfaces: []infrav1alpha2.InstanceConfigInterfaceCreateOptions{
			{
				IPAMAddress: "address",
				Label:       "label",
				Purpose:     linodego.InterfacePurposePublic,
				Primary:     true,
				SubnetID:    &subnetID,
				IPv4: &infrav1alpha2.VPCIPv4{
					VPC:     "vpc",
					NAT1To1: "nat11",
				},
				IPRanges: []string{"ip"},
			},
		},
		BackupsEnabled: true,
		PrivateIP:      util.Pointer(true),
		Tags:           []string{"tag"},
	}

	createConfig := linodeMachineSpecToInstanceCreateConfig(machineSpec)
	assert.NotNil(t, createConfig, "Failed to convert LinodeMachineSpec to InstanceCreateOptions")

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(createConfig)
	require.NoError(t, err, "Failed to encode InstanceCreateOptions")

	var actualMachineSpec infrav1alpha2.LinodeMachineSpec
	dec := gob.NewDecoder(&buf)
	err = dec.Decode(&actualMachineSpec)
	require.NoError(t, err, "Failed to decode LinodeMachineSpec")

	assert.Equal(t, machineSpec, actualMachineSpec)
}

func TestSetUserData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		machineScope  *scope.MachineScope
		createConfig  *linodego.InstanceCreateOptions
		wantConfig    *linodego.InstanceCreateOptions
		expectedError error
		expects       func(client *mock.MockLinodeClient, kClient *mock.MockK8sClient)
	}{
		{
			name: "Success - SetUserData metadata",
			machineScope: &scope.MachineScope{Machine: &v1beta1.Machine{
				Spec: v1beta1.MachineSpec{
					ClusterName: "",
					Bootstrap: v1beta1.Bootstrap{
						DataSecretName: ptr.To("test-data"),
					},
					InfrastructureRef: corev1.ObjectReference{},
				},
			}, LinodeMachine: &infrav1alpha2.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec:   infrav1alpha2.LinodeMachineSpec{Region: "us-ord", Image: "linode/ubuntu22.04"},
				Status: infrav1alpha2.LinodeMachineStatus{CloudinitMetadataSupport: true},
			}},
			createConfig: &linodego.InstanceCreateOptions{},
			wantConfig: &linodego.InstanceCreateOptions{Metadata: &linodego.InstanceMetadataOptions{
				UserData: b64.StdEncoding.EncodeToString([]byte("test-data")),
			}},
			expects: func(mockClient *mock.MockLinodeClient, kMock *mock.MockK8sClient) {
				kMock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					cred := corev1.Secret{
						Data: map[string][]byte{
							"value": []byte("test-data"),
						},
					}
					*obj = cred
					return nil
				})
			},
		},
		{
			name: "Success - SetUserData StackScript",
			machineScope: &scope.MachineScope{Machine: &v1beta1.Machine{
				Spec: v1beta1.MachineSpec{
					ClusterName: "",
					Bootstrap: v1beta1.Bootstrap{
						DataSecretName: ptr.To("test-data"),
					},
					InfrastructureRef: corev1.ObjectReference{},
				},
			}, LinodeMachine: &infrav1alpha2.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec:   infrav1alpha2.LinodeMachineSpec{Region: "us-east", Image: "linode/ubuntu22.04", Type: "g6-standard-1"},
				Status: infrav1alpha2.LinodeMachineStatus{CloudinitMetadataSupport: false},
			}},
			createConfig: &linodego.InstanceCreateOptions{},
			wantConfig: &linodego.InstanceCreateOptions{StackScriptID: 1234, StackScriptData: map[string]string{
				"instancedata": b64.StdEncoding.EncodeToString([]byte("label: test-cluster\nregion: us-east\ntype: g6-standard-1")),
				"userdata":     b64.StdEncoding.EncodeToString([]byte("test-data")),
			}},
			expects: func(mockClient *mock.MockLinodeClient, kMock *mock.MockK8sClient) {
				kMock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					cred := corev1.Secret{
						Data: map[string][]byte{
							"value": []byte("test-data"),
						},
					}
					*obj = cred
					return nil
				})
				mockClient.EXPECT().ListStackscripts(gomock.Any(), &linodego.ListOptions{Filter: "{\"label\":\"CAPL-dev\"}"}).Return([]linodego.Stackscript{{
					Label: "CAPI Test 1",
					ID:    1234,
				}}, nil)
			},
		},
		{
			name: "Error - SetUserData large bootstrap data",
			machineScope: &scope.MachineScope{Machine: &v1beta1.Machine{
				Spec: v1beta1.MachineSpec{
					ClusterName: "",
					Bootstrap: v1beta1.Bootstrap{
						DataSecretName: ptr.To("test-data"),
					},
					InfrastructureRef: corev1.ObjectReference{},
				},
			}, LinodeMachine: &infrav1alpha2.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec:   infrav1alpha2.LinodeMachineSpec{Region: "us-ord", Image: "linode/ubuntu22.04"},
				Status: infrav1alpha2.LinodeMachineStatus{},
			}},
			createConfig: &linodego.InstanceCreateOptions{},
			wantConfig:   &linodego.InstanceCreateOptions{},
			expects: func(mockClient *mock.MockLinodeClient, kMock *mock.MockK8sClient) {
				kMock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					cred := corev1.Secret{
						Data: map[string][]byte{
							"value": make([]byte, maxBootstrapDataBytes+1),
						},
					}
					*obj = cred
					return nil
				})
			},
			expectedError: fmt.Errorf("bootstrap data too large"),
		},
		{
			name: "Error - SetUserData get bootstrap data",
			machineScope: &scope.MachineScope{Machine: &v1beta1.Machine{
				Spec: v1beta1.MachineSpec{
					ClusterName: "",
					Bootstrap: v1beta1.Bootstrap{
						ConfigRef:      nil,
						DataSecretName: nil,
					},
					InfrastructureRef: corev1.ObjectReference{},
				},
			}, LinodeMachine: &infrav1alpha2.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec:   infrav1alpha2.LinodeMachineSpec{},
				Status: infrav1alpha2.LinodeMachineStatus{},
			}},
			createConfig: &linodego.InstanceCreateOptions{},
			wantConfig:   &linodego.InstanceCreateOptions{},
			expects: func(c *mock.MockLinodeClient, k *mock.MockK8sClient) {
			},
			expectedError: fmt.Errorf("bootstrap data secret is nil for LinodeMachine default/test-cluster"),
		},
		{
			name: "Error - SetUserData failed to get stackscripts",
			machineScope: &scope.MachineScope{Machine: &v1beta1.Machine{
				Spec: v1beta1.MachineSpec{
					ClusterName: "",
					Bootstrap: v1beta1.Bootstrap{
						DataSecretName: ptr.To("test-data"),
					},
					InfrastructureRef: corev1.ObjectReference{},
				},
			}, LinodeMachine: &infrav1alpha2.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec:   infrav1alpha2.LinodeMachineSpec{Region: "us-east", Image: "linode/ubuntu22.04", Type: "g6-standard-1"},
				Status: infrav1alpha2.LinodeMachineStatus{CloudinitMetadataSupport: false},
			}},
			createConfig: &linodego.InstanceCreateOptions{},
			wantConfig: &linodego.InstanceCreateOptions{StackScriptID: 1234, StackScriptData: map[string]string{
				"instancedata": b64.StdEncoding.EncodeToString([]byte("label: test-cluster\nregion: us-east\ntype: g6-standard-1")),
				"userdata":     b64.StdEncoding.EncodeToString([]byte("test-data")),
			}},
			expects: func(mockClient *mock.MockLinodeClient, kMock *mock.MockK8sClient) {
				kMock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					cred := corev1.Secret{
						Data: map[string][]byte{
							"value": []byte("test-data"),
						},
					}
					*obj = cred
					return nil
				})
				mockClient.EXPECT().ListStackscripts(gomock.Any(), &linodego.ListOptions{Filter: "{\"label\":\"CAPL-dev\"}"}).Return(nil, fmt.Errorf("failed to get stackscripts"))
			},
			expectedError: fmt.Errorf("ensure stackscript: failed to get stackscript with label CAPL-dev: failed to get stackscripts"),
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockLinodeClient(ctrl)
			mockK8sClient := mock.NewMockK8sClient(ctrl)
			testcase.machineScope.LinodeClient = mockClient
			testcase.machineScope.Client = mockK8sClient
			testcase.expects(mockClient, mockK8sClient)
			logger := logr.Logger{}

			err := setUserData(context.Background(), testcase.machineScope, testcase.createConfig, logger)
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			} else {
				assert.Equal(t, testcase.wantConfig.Metadata, testcase.createConfig.Metadata)
				assert.Equal(t, testcase.wantConfig.StackScriptID, testcase.createConfig.StackScriptID)
				assert.Equal(t, testcase.wantConfig.StackScriptData, testcase.createConfig.StackScriptData)
			}
		})
	}
}

func TestCreateInstanceConfigDeviceMap(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		instanceDisks   map[string]*infrav1alpha2.InstanceDisk
		expectedDiskMap linodego.InstanceConfigDeviceMap
		expectedError   error
	}{
		{
			name: "Success - single disk gets added to config",
			instanceDisks: map[string]*infrav1alpha2.InstanceDisk{"sdb": {
				DiskID: 101,
				Size:   resource.MustParse("10Gi"),
				Label:  "disk1",
			}},
			expectedDiskMap: linodego.InstanceConfigDeviceMap{SDA: &linodego.InstanceConfigDevice{DiskID: 100},
				SDB: &linodego.InstanceConfigDevice{DiskID: 101},
			},
		},
		{
			name: "Success - multiple disks gets added to config",
			instanceDisks: map[string]*infrav1alpha2.InstanceDisk{"sdb": {
				DiskID: 101,
				Size:   resource.MustParse("10Gi"),
				Label:  "disk1",
			}, "sdc": {
				DiskID: 102,
				Size:   resource.MustParse("10Gi"),
				Label:  "disk1",
			}, "sdd": {
				DiskID: 103,
				Size:   resource.MustParse("10Gi"),
				Label:  "disk1",
			}, "sde": {
				DiskID: 104,
				Size:   resource.MustParse("10Gi"),
				Label:  "disk1",
			}, "sdf": {
				DiskID: 105,
				Size:   resource.MustParse("10Gi"),
				Label:  "disk1",
			}, "sdg": {
				DiskID: 106,
				Size:   resource.MustParse("10Gi"),
				Label:  "disk1",
			}, "sdh": {
				DiskID: 107,
				Size:   resource.MustParse("10Gi"),
				Label:  "disk1",
			}},
			expectedDiskMap: linodego.InstanceConfigDeviceMap{
				SDA: &linodego.InstanceConfigDevice{DiskID: 100},
				SDB: &linodego.InstanceConfigDevice{DiskID: 101},
				SDC: &linodego.InstanceConfigDevice{DiskID: 102},
				SDD: &linodego.InstanceConfigDevice{DiskID: 103},
				SDE: &linodego.InstanceConfigDevice{DiskID: 104},
				SDF: &linodego.InstanceConfigDevice{DiskID: 105},
				SDG: &linodego.InstanceConfigDevice{DiskID: 106},
				SDH: &linodego.InstanceConfigDevice{DiskID: 107},
			},
		},
		{
			name: "Error - single disk with invalid name",
			instanceDisks: map[string]*infrav1alpha2.InstanceDisk{"sdx": {
				DiskID: 101,
				Size:   resource.MustParse("10Gi"),
				Label:  "disk1",
			}},
			expectedError: fmt.Errorf("unknown device name: \"sdx\""),
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			actualConfig := linodego.InstanceConfig{
				ID:    0,
				Label: "root disk",
				Devices: &linodego.InstanceConfigDeviceMap{
					SDA: &linodego.InstanceConfigDevice{DiskID: 100},
				},
			}
			err := createInstanceConfigDeviceMap(testcase.instanceDisks, actualConfig.Devices)
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			} else {
				assert.Equal(t, actualConfig.Devices.SDA, testcase.expectedDiskMap.SDA)
				assert.Equal(t, actualConfig.Devices.SDB, testcase.expectedDiskMap.SDB)
				assert.Equal(t, actualConfig.Devices.SDC, testcase.expectedDiskMap.SDC)
				assert.Equal(t, actualConfig.Devices.SDD, testcase.expectedDiskMap.SDD)
				assert.Equal(t, actualConfig.Devices.SDE, testcase.expectedDiskMap.SDE)
				assert.Equal(t, actualConfig.Devices.SDF, testcase.expectedDiskMap.SDF)
				assert.Equal(t, actualConfig.Devices.SDG, testcase.expectedDiskMap.SDG)
			}
		})
	}
}
