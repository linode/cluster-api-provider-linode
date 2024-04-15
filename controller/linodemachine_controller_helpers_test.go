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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/mock"
	"github.com/linode/cluster-api-provider-linode/util"
)

func TestLinodeMachineSpecToCreateInstanceConfig(t *testing.T) {
	t.Parallel()

	subnetID := 1

	machineSpec := infrav1alpha1.LinodeMachineSpec{
		Region:          "region",
		Type:            "type",
		Group:           "group",
		RootPass:        "rootPass",
		AuthorizedKeys:  []string{"key"},
		AuthorizedUsers: []string{"user"},
		BackupID:        1,
		Image:           "image",
		Interfaces: []infrav1alpha1.InstanceConfigInterfaceCreateOptions{
			{
				IPAMAddress: "address",
				Label:       "label",
				Purpose:     linodego.InterfacePurposePublic,
				Primary:     true,
				SubnetID:    &subnetID,
				IPv4: &infrav1alpha1.VPCIPv4{
					VPC:     "vpc",
					NAT1To1: "nat11",
				},
				IPRanges: []string{"ip"},
			},
		},
		BackupsEnabled: true,
		PrivateIP:      util.Pointer(true),
		Tags:           []string{"tag"},
		FirewallID:     1,
	}

	createConfig := linodeMachineSpecToInstanceCreateConfig(machineSpec)
	assert.NotNil(t, createConfig, "Failed to convert LinodeMachineSpec to InstanceCreateOptions")

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(createConfig)
	require.NoError(t, err, "Failed to encode InstanceCreateOptions")

	var actualMachineSpec infrav1alpha1.LinodeMachineSpec
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
		expects       func(client *mock.MockLinodeMachineClient, kClient *mock.MockK8sClient)
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
			}, LinodeMachine: &infrav1alpha1.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec:   infrav1alpha1.LinodeMachineSpec{Region: "us-ord", Image: "linode/ubuntu22.04"},
				Status: infrav1alpha1.LinodeMachineStatus{},
			}},
			createConfig: &linodego.InstanceCreateOptions{},
			wantConfig: &linodego.InstanceCreateOptions{Metadata: &linodego.InstanceMetadataOptions{
				UserData: b64.StdEncoding.EncodeToString([]byte("test-data")),
			}},
			expects: func(mockClient *mock.MockLinodeMachineClient, kMock *mock.MockK8sClient) {
				kMock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					cred := corev1.Secret{
						Data: map[string][]byte{
							"value": []byte("test-data"),
						},
					}
					*obj = cred
					return nil
				})
				mockClient.EXPECT().GetRegion(gomock.Any(), "us-ord").Return(&linodego.Region{
					Capabilities: []string{"Metadata"},
				}, nil)
				mockClient.EXPECT().GetImage(gomock.Any(), "linode/ubuntu22.04").Return(&linodego.Image{
					Capabilities: []string{"cloud-init"},
				}, nil)
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
			}, LinodeMachine: &infrav1alpha1.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec:   infrav1alpha1.LinodeMachineSpec{Region: "us-east", Image: "linode/ubuntu22.04", Type: "g6-standard-1"},
				Status: infrav1alpha1.LinodeMachineStatus{},
			}},
			createConfig: &linodego.InstanceCreateOptions{},
			wantConfig: &linodego.InstanceCreateOptions{StackScriptID: 1234, StackScriptData: map[string]string{
				"instancedata": b64.StdEncoding.EncodeToString([]byte("label: test-cluster\nregion: us-east\ntype: g6-standard-1")),
				"userdata":     b64.StdEncoding.EncodeToString([]byte("test-data")),
			}},
			expects: func(mockClient *mock.MockLinodeMachineClient, kMock *mock.MockK8sClient) {
				kMock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					cred := corev1.Secret{
						Data: map[string][]byte{
							"value": []byte("test-data"),
						},
					}
					*obj = cred
					return nil
				})
				mockClient.EXPECT().GetRegion(gomock.Any(), "us-east").Return(&linodego.Region{
					Capabilities: []string{"Metadata"},
				}, nil)
				mockClient.EXPECT().GetImage(gomock.Any(), "linode/ubuntu22.04").Return(&linodego.Image{}, nil)
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
			}, LinodeMachine: &infrav1alpha1.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec:   infrav1alpha1.LinodeMachineSpec{Region: "us-ord", Image: "linode/ubuntu22.04"},
				Status: infrav1alpha1.LinodeMachineStatus{},
			}},
			createConfig: &linodego.InstanceCreateOptions{},
			wantConfig:   &linodego.InstanceCreateOptions{},
			expects: func(mockClient *mock.MockLinodeMachineClient, kMock *mock.MockK8sClient) {
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
			}, LinodeMachine: &infrav1alpha1.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec:   infrav1alpha1.LinodeMachineSpec{},
				Status: infrav1alpha1.LinodeMachineStatus{},
			}},
			createConfig: &linodego.InstanceCreateOptions{},
			wantConfig:   &linodego.InstanceCreateOptions{},
			expects: func(c *mock.MockLinodeMachineClient, k *mock.MockK8sClient) {
			},
			expectedError: fmt.Errorf("bootstrap data secret is nil for LinodeMachine default/test-cluster"),
		},
		{
			name: "Error - SetUserData failed to get regions",
			machineScope: &scope.MachineScope{Machine: &v1beta1.Machine{
				Spec: v1beta1.MachineSpec{
					ClusterName: "",
					Bootstrap: v1beta1.Bootstrap{
						DataSecretName: ptr.To("test-data"),
					},
					InfrastructureRef: corev1.ObjectReference{},
				},
			}, LinodeMachine: &infrav1alpha1.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec:   infrav1alpha1.LinodeMachineSpec{Region: "us-ord", Image: "linode/ubuntu22.04"},
				Status: infrav1alpha1.LinodeMachineStatus{},
			}},
			createConfig: &linodego.InstanceCreateOptions{},
			wantConfig:   &linodego.InstanceCreateOptions{},
			expects: func(mockClient *mock.MockLinodeMachineClient, kMock *mock.MockK8sClient) {
				kMock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					cred := corev1.Secret{
						Data: map[string][]byte{
							"value": []byte("hello"),
						},
					}
					*obj = cred
					return nil
				})
				mockClient.EXPECT().GetRegion(gomock.Any(), "us-ord").Return(nil, fmt.Errorf("cannot find region"))
			},
			expectedError: fmt.Errorf("cannot find region"),
		},
		{
			name: "Error - SetUserData failed to get images",
			machineScope: &scope.MachineScope{Machine: &v1beta1.Machine{
				Spec: v1beta1.MachineSpec{
					ClusterName: "",
					Bootstrap: v1beta1.Bootstrap{
						DataSecretName: ptr.To("test-data"),
					},
					InfrastructureRef: corev1.ObjectReference{},
				},
			}, LinodeMachine: &infrav1alpha1.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec:   infrav1alpha1.LinodeMachineSpec{Region: "us-ord", Image: "linode/ubuntu22.04"},
				Status: infrav1alpha1.LinodeMachineStatus{},
			}},
			createConfig: &linodego.InstanceCreateOptions{},
			wantConfig:   &linodego.InstanceCreateOptions{},
			expects: func(mockClient *mock.MockLinodeMachineClient, kMock *mock.MockK8sClient) {
				kMock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					cred := corev1.Secret{
						Data: map[string][]byte{
							"value": []byte("hello"),
						},
					}
					*obj = cred
					return nil
				})
				mockClient.EXPECT().GetRegion(gomock.Any(), "us-ord").Return(&linodego.Region{
					Capabilities: []string{"Metadata"},
				}, nil)
				mockClient.EXPECT().GetImage(gomock.Any(), "linode/ubuntu22.04").Return(nil, fmt.Errorf("cannot find image"))
			},
			expectedError: fmt.Errorf("cannot find image"),
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
			}, LinodeMachine: &infrav1alpha1.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec:   infrav1alpha1.LinodeMachineSpec{Region: "us-east", Image: "linode/ubuntu22.04", Type: "g6-standard-1"},
				Status: infrav1alpha1.LinodeMachineStatus{},
			}},
			createConfig: &linodego.InstanceCreateOptions{},
			wantConfig: &linodego.InstanceCreateOptions{StackScriptID: 1234, StackScriptData: map[string]string{
				"instancedata": b64.StdEncoding.EncodeToString([]byte("label: test-cluster\nregion: us-east\ntype: g6-standard-1")),
				"userdata":     b64.StdEncoding.EncodeToString([]byte("test-data")),
			}},
			expects: func(mockClient *mock.MockLinodeMachineClient, kMock *mock.MockK8sClient) {
				kMock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					cred := corev1.Secret{
						Data: map[string][]byte{
							"value": []byte("test-data"),
						},
					}
					*obj = cred
					return nil
				})
				mockClient.EXPECT().GetRegion(gomock.Any(), "us-east").Return(&linodego.Region{
					Capabilities: []string{"Metadata"},
				}, nil)
				mockClient.EXPECT().GetImage(gomock.Any(), "linode/ubuntu22.04").Return(&linodego.Image{}, nil)
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

			mockClient := mock.NewMockLinodeMachineClient(ctrl)
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
