package controller

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	b64 "encoding/base64"
	"fmt"
	"slices"
	"testing"

	awssigner "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/go-logr/logr/testr"
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
	}

	createConfig := linodeMachineSpecToInstanceCreateConfig(machineSpec, []string{"tag"})
	assert.NotNil(t, createConfig, "Failed to convert LinodeMachineSpec to InstanceCreateOptions")
}

func TestSetUserData(t *testing.T) {
	t.Parallel()

	userData := []byte("test-data")
	if gzipCompressionFlag {
		var userDataBuff bytes.Buffer
		gz := gzip.NewWriter(&userDataBuff)
		_, err = gz.Write([]byte("test-data"))
		err = gz.Close()
		require.NoError(t, err, "Failed to compress bootstrap data")
		userData = userDataBuff.Bytes()
	}

	tests := []struct {
		name          string
		machineScope  *scope.MachineScope
		createConfig  *linodego.InstanceCreateOptions
		wantConfig    *linodego.InstanceCreateOptions
		expectedError error
		expects       func(client *mock.MockLinodeClient, kClient *mock.MockK8sClient, s3Client *mock.MockS3Client, s3PresignedClient *mock.MockS3PresignClient)
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
				Spec: infrav1alpha2.LinodeMachineSpec{Region: "us-ord", Image: "linode/ubuntu22.04"},
			}},
			createConfig: &linodego.InstanceCreateOptions{},
			wantConfig: &linodego.InstanceCreateOptions{Metadata: &linodego.InstanceMetadataOptions{
				UserData: b64.StdEncoding.EncodeToString(userData),
			}},
			expects: func(mockClient *mock.MockLinodeClient, kMock *mock.MockK8sClient, s3Client *mock.MockS3Client, s3PresignedClient *mock.MockS3PresignClient) {
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
			name: "Success - SetUserData metadata and cluster object store (large bootstrap data)",
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
				Spec: infrav1alpha2.LinodeMachineSpec{Region: "us-ord", Image: "linode/ubuntu22.04"},
			}, LinodeCluster: &infrav1alpha2.LinodeCluster{
				Spec: infrav1alpha2.LinodeClusterSpec{
					ObjectStore: &infrav1alpha2.ObjectStore{CredentialsRef: corev1.SecretReference{Name: "fake"}},
				},
			}},
			createConfig: &linodego.InstanceCreateOptions{},
			wantConfig: &linodego.InstanceCreateOptions{Metadata: &linodego.InstanceMetadataOptions{
				UserData: b64.StdEncoding.EncodeToString([]byte(`#include
https://object.bucket.example.com
`)),
			}},
			expects: func(mockClient *mock.MockLinodeClient, kMock *mock.MockK8sClient, s3Mock *mock.MockS3Client, s3PresignedMock *mock.MockS3PresignClient) {
				kMock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					largeData := make([]byte, maxBootstrapDataBytesCloudInit*10)
					_, rerr := rand.Read(largeData)
					require.NoError(t, rerr, "Failed to create bootstrap data")
					cred := corev1.Secret{
						Data: map[string][]byte{
							"value": largeData,
						},
					}
					*obj = cred
					return nil
				})
				kMock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					cred := corev1.Secret{
						Data: map[string][]byte{
							"bucket":          []byte("fake"),
							"bucket_endpoint": []byte("fake.example.com"),
							"endpoint":        []byte("example.com"),
							"access":          []byte("fake"),
							"secret":          []byte("fake"),
						},
					}
					*obj = cred
					return nil
				})
				s3Mock.EXPECT().PutObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(&s3.PutObjectOutput{}, nil)
				s3PresignedMock.EXPECT().PresignGetObject(gomock.Any(), gomock.Any()).Return(&awssigner.PresignedHTTPRequest{URL: "https://object.bucket.example.com"}, nil)
			},
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
			expects: func(c *mock.MockLinodeClient, k *mock.MockK8sClient, s3Client *mock.MockS3Client, s3PresignedClient *mock.MockS3PresignClient) {
			},
			expectedError: fmt.Errorf("bootstrap data secret is nil for LinodeMachine default/test-cluster"),
		},
		{
			name: "Error - SetUserData failed to upload to Cluster Object Store",
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
				Spec: infrav1alpha2.LinodeMachineSpec{Region: "us-ord", Image: "linode/ubuntu22.04"},
			}, LinodeCluster: &infrav1alpha2.LinodeCluster{
				Spec: infrav1alpha2.LinodeClusterSpec{
					ObjectStore: &infrav1alpha2.ObjectStore{CredentialsRef: corev1.SecretReference{Name: "fake"}},
				},
			}},
			createConfig: &linodego.InstanceCreateOptions{},
			wantConfig:   &linodego.InstanceCreateOptions{},
			expects: func(mockClient *mock.MockLinodeClient, kMock *mock.MockK8sClient, s3Mock *mock.MockS3Client, s3PresignedMock *mock.MockS3PresignClient) {
				kMock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					largeData := make([]byte, maxBootstrapDataBytesCloudInit*10)
					_, rerr := rand.Read(largeData)
					require.NoError(t, rerr, "Failed to create bootstrap data")
					cred := corev1.Secret{
						Data: map[string][]byte{
							"value": largeData,
						},
					}
					*obj = cred
					return nil
				})
				kMock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
					cred := corev1.Secret{
						Data: map[string][]byte{
							"bucket":          []byte("fake"),
							"bucket_endpoint": []byte("fake.example.com"),
							"endpoint":        []byte("example.com"),
							"access":          []byte("fake"),
							"secret":          []byte("fake"),
						},
					}
					*obj = cred
					return nil
				})
				s3Mock.EXPECT().PutObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &s3types.NoSuchBucket{})
			},
			expectedError: fmt.Errorf("put object"),
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
			mockS3Client := mock.NewMockS3Client(ctrl)
			mockS3PresignClient := mock.NewMockS3PresignClient(ctrl)
			testcase.machineScope.LinodeClient = mockClient
			testcase.machineScope.Client = mockK8sClient
			testcase.machineScope.S3Client = mockS3Client
			testcase.machineScope.S3PresignClient = mockS3PresignClient
			testcase.expects(mockClient, mockK8sClient, mockS3Client, mockS3PresignClient)
			logger := testr.New(t)

			err := setUserData(t.Context(), testcase.machineScope, testcase.createConfig, gzipCompressionFlag, logger)
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			} else {
				assert.Equal(t, testcase.wantConfig.Metadata, testcase.createConfig.Metadata)
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

// validateInterfaceExpectations is a helper function to check VPC interface expectations in tests
func validateInterfaceExpectations(
	t *testing.T,
	err error,
	iface *linodego.InstanceConfigInterfaceCreateOptions,
	expectErr bool,
	expectErrMsg string,
	expectInterface bool,
	expectSubnetID int,
	interfaces interface{},
) {
	t.Helper()

	if expectErr {
		require.Error(t, err)
		require.Contains(t, err.Error(), expectErrMsg)
		require.Nil(t, iface)
		return
	}

	require.NoError(t, err)
	if expectInterface {
		require.NotNil(t, iface)
		require.Equal(t, linodego.InterfacePurposeVPC, iface.Purpose)
		require.True(t, iface.Primary)
		require.NotNil(t, iface.SubnetID)
		require.Equal(t, expectSubnetID, *iface.SubnetID)
		require.NotNil(t, iface.IPv4)
		require.NotNil(t, iface.IPv4.NAT1To1)
		require.Equal(t, "any", *iface.IPv4.NAT1To1)
	} else {
		require.Nil(t, iface)
	}
}

func TestGetVPCInterfaceConfigFromDirectID(t *testing.T) {
	t.Parallel()

	// Setup test cases
	testCases := []struct {
		name            string
		vpcID           int
		interfaces      []linodego.InstanceConfigInterfaceCreateOptions
		subnetName      string
		mockSetup       func(mockLinodeClient *mock.MockLinodeClient)
		expectErr       bool
		expectErrMsg    string
		expectInterface bool
		expectSubnetID  int
	}{
		{
			name:       "Success - Valid VPC with subnets, no subnet name",
			vpcID:      123,
			interfaces: []linodego.InstanceConfigInterfaceCreateOptions{},
			mockSetup: func(mockLinodeClient *mock.MockLinodeClient) {
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), 123).Return(&linodego.VPC{
					ID: 123,
					Subnets: []linodego.VPCSubnet{
						{
							ID:    456,
							Label: "subnet-1",
						},
						{
							ID:    789,
							Label: "subnet-2",
						},
					},
				}, nil)
			},
			expectErr:       false,
			expectInterface: true,
			expectSubnetID:  456, // First subnet ID
		},
		{
			name:       "Success - Valid VPC with subnets, specific subnet name",
			vpcID:      123,
			interfaces: []linodego.InstanceConfigInterfaceCreateOptions{},
			subnetName: "subnet-2",
			mockSetup: func(mockLinodeClient *mock.MockLinodeClient) {
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), 123).Return(&linodego.VPC{
					ID: 123,
					Subnets: []linodego.VPCSubnet{
						{
							ID:    456,
							Label: "subnet-1",
						},
						{
							ID:    789,
							Label: "subnet-2",
						},
					},
				}, nil)
			},
			expectErr:       false,
			expectInterface: true,
			expectSubnetID:  789, // Matching subnet ID
		},
		{
			name:  "Success - VPC interface already exists",
			vpcID: 123,
			interfaces: []linodego.InstanceConfigInterfaceCreateOptions{
				{
					Purpose: linodego.InterfacePurposeVPC,
				},
			},
			mockSetup: func(mockLinodeClient *mock.MockLinodeClient) {
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), 123).Return(&linodego.VPC{
					ID: 123,
					Subnets: []linodego.VPCSubnet{
						{
							ID:    456,
							Label: "subnet-1",
						},
					},
				}, nil)
			},
			expectErr:       false,
			expectInterface: false,
			expectSubnetID:  456,
		},
		{
			name:       "Error - VPC does not exist",
			vpcID:      999,
			interfaces: []linodego.InstanceConfigInterfaceCreateOptions{},
			mockSetup: func(mockLinodeClient *mock.MockLinodeClient) {
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), 999).Return(nil, fmt.Errorf("VPC not found"))
			},
			expectErr:       true,
			expectErrMsg:    "VPC not found",
			expectInterface: false,
		},
		{
			name:       "Error - VPC has no subnets",
			vpcID:      123,
			interfaces: []linodego.InstanceConfigInterfaceCreateOptions{},
			mockSetup: func(mockLinodeClient *mock.MockLinodeClient) {
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), 123).Return(&linodego.VPC{
					ID:      123,
					Subnets: []linodego.VPCSubnet{},
				}, nil)
			},
			expectErr:       true,
			expectErrMsg:    "no subnets found in VPC",
			expectInterface: false,
		},
		{
			name:       "Error - Subnet name not found",
			vpcID:      123,
			interfaces: []linodego.InstanceConfigInterfaceCreateOptions{},
			subnetName: "non-existent",
			mockSetup: func(mockLinodeClient *mock.MockLinodeClient) {
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), 123).Return(&linodego.VPC{
					ID: 123,
					Subnets: []linodego.VPCSubnet{
						{
							ID:    456,
							Label: "subnet-1",
						},
					},
				}, nil)
			},
			expectErr:       true,
			expectErrMsg:    "subnet with label non-existent not found in VPC",
			expectInterface: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Setup mock controller and client
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLinodeClient := mock.NewMockLinodeClient(ctrl)
			mockK8sClient := mock.NewMockK8sClient(ctrl)

			tc.mockSetup(mockLinodeClient)

			// Create test context
			ctx := t.Context()
			logger := testr.New(t)

			// Create machine scope
			machineScope := &scope.MachineScope{
				LinodeClient: mockLinodeClient,
				Client:       mockK8sClient,
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					Spec: infrav1alpha2.LinodeMachineSpec{},
				},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							SubnetName: tc.subnetName,
						},
					},
				},
			}

			// Call the function being tested
			iface, err := getVPCInterfaceConfigFromDirectID(ctx, machineScope, tc.interfaces, logger, tc.vpcID)

			// Check expectations
			validateInterfaceExpectations(t, err, iface, tc.expectErr, tc.expectErrMsg, tc.expectInterface, tc.expectSubnetID, tc.interfaces)

			// Additional check for interface updates
			if !tc.expectErr && !tc.expectInterface && len(tc.interfaces) > 0 && tc.interfaces[0].Purpose == linodego.InterfacePurposeVPC {
				require.NotNil(t, tc.interfaces[0].SubnetID)
				require.Equal(t, tc.expectSubnetID, *tc.interfaces[0].SubnetID)
			}
		})
	}
}

func TestAddVPCInterfaceFromDirectID(t *testing.T) {
	t.Parallel()

	// Setup test cases
	testCases := []struct {
		name          string
		vpcID         int
		createConfig  *linodego.InstanceCreateOptions
		mockSetup     func(mockLinodeClient *mock.MockLinodeClient)
		expectErr     bool
		expectErrMsg  string
		expectNoIface bool
	}{
		{
			name:  "Success - Interface added correctly",
			vpcID: 123,
			createConfig: &linodego.InstanceCreateOptions{
				Interfaces: []linodego.InstanceConfigInterfaceCreateOptions{},
			},
			mockSetup: func(mockLinodeClient *mock.MockLinodeClient) {
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), 123).Return(&linodego.VPC{
					ID: 123,
					Subnets: []linodego.VPCSubnet{
						{
							ID:    456,
							Label: "subnet-1",
						},
					},
				}, nil)
			},
			expectErr: false,
		},
		{
			name:  "Error - getVPCInterfaceConfigFromDirectID returns error",
			vpcID: 999,
			createConfig: &linodego.InstanceCreateOptions{
				Interfaces: []linodego.InstanceConfigInterfaceCreateOptions{},
			},
			mockSetup: func(mockLinodeClient *mock.MockLinodeClient) {
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), 999).Return(nil, fmt.Errorf("VPC not found"))
			},
			expectErr:    true,
			expectErrMsg: "VPC not found",
		},
		{
			name:  "Success - Interface already exists",
			vpcID: 123,
			createConfig: &linodego.InstanceCreateOptions{
				Interfaces: []linodego.InstanceConfigInterfaceCreateOptions{
					{
						Purpose: linodego.InterfacePurposeVPC,
					},
				},
			},
			mockSetup: func(mockLinodeClient *mock.MockLinodeClient) {
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), 123).Return(&linodego.VPC{
					ID: 123,
					Subnets: []linodego.VPCSubnet{
						{
							ID:    456,
							Label: "subnet-1",
						},
					},
				}, nil)
			},
			expectErr:     false,
			expectNoIface: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Setup mock controller and client
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLinodeClient := mock.NewMockLinodeClient(ctrl)
			mockK8sClient := mock.NewMockK8sClient(ctrl)

			tc.mockSetup(mockLinodeClient)

			// Create test context
			ctx := t.Context()
			logger := testr.New(t)

			// Create machine scope
			machineScope := &scope.MachineScope{
				LinodeClient: mockLinodeClient,
				Client:       mockK8sClient,
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					Spec: infrav1alpha2.LinodeMachineSpec{},
				},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{},
				},
			}

			// Store original interface count
			originalCount := len(tc.createConfig.Interfaces)

			// Call the function being tested
			err := addVPCInterfaceFromDirectID(ctx, machineScope, tc.createConfig, logger, tc.vpcID)

			// Check expectations
			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErrMsg)
			} else {
				require.NoError(t, err)
				if tc.expectNoIface {
					// If interface already existed, count should remain the same
					require.Len(t, tc.createConfig.Interfaces, originalCount)
				} else {
					// If interface was added, count should increase
					require.Len(t, tc.createConfig.Interfaces, originalCount+1)
					require.Equal(t, linodego.InterfacePurposeVPC, tc.createConfig.Interfaces[0].Purpose)
					require.True(t, tc.createConfig.Interfaces[0].Primary)
				}
			}
		})
	}
}

func TestConfigureVPCInterface(t *testing.T) {
	t.Parallel()

	vpcRef := &corev1.ObjectReference{
		Name:      "test-vpc",
		Namespace: "default",
	}

	subnetID := 456

	// Setup test cases
	testCases := []struct {
		name            string
		machineVPCID    *int
		clusterVPCID    *int
		vpcRef          *corev1.ObjectReference
		createConfig    *linodego.InstanceCreateOptions
		mockSetup       func(mockLinodeClient *mock.MockLinodeClient, mockK8sClient *mock.MockK8sClient)
		expectErr       bool
		expectErrMsg    string
		expectInterface bool
	}{
		{
			name:         "Success - VPCID on machine",
			machineVPCID: ptr.To(123),
			createConfig: &linodego.InstanceCreateOptions{
				Interfaces: []linodego.InstanceConfigInterfaceCreateOptions{},
			},
			mockSetup: func(mockLinodeClient *mock.MockLinodeClient, mockK8sClient *mock.MockK8sClient) {
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), 123).Return(&linodego.VPC{
					ID: 123,
					Subnets: []linodego.VPCSubnet{
						{
							ID:    456,
							Label: "subnet-1",
						},
					},
				}, nil)
			},
			expectErr:       false,
			expectInterface: true,
		},
		{
			name:         "Success - VPCID on cluster",
			clusterVPCID: ptr.To(123),
			createConfig: &linodego.InstanceCreateOptions{
				Interfaces: []linodego.InstanceConfigInterfaceCreateOptions{},
			},
			mockSetup: func(mockLinodeClient *mock.MockLinodeClient, mockK8sClient *mock.MockK8sClient) {
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), 123).Return(&linodego.VPC{
					ID: 123,
					Subnets: []linodego.VPCSubnet{
						{
							ID:    456,
							Label: "subnet-1",
						},
					},
				}, nil)
			},
			expectErr:       false,
			expectInterface: true,
		},
		{
			name:   "Success - VPC reference",
			vpcRef: vpcRef,
			createConfig: &linodego.InstanceCreateOptions{
				Interfaces: []linodego.InstanceConfigInterfaceCreateOptions{},
			},
			mockSetup: func(mockLinodeClient *mock.MockLinodeClient, mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Get(gomock.Any(), client.ObjectKey{
					Name:      "test-vpc",
					Namespace: "default",
				}, gomock.Any()).DoAndReturn(func(_ context.Context, _ client.ObjectKey, vpc *infrav1alpha2.LinodeVPC, _ ...client.GetOption) error {
					vpc.Status.Ready = true
					vpc.Spec.VPCID = ptr.To(123)
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{
						{
							SubnetID: subnetID,
							Label:    "subnet-1",
						},
					}
					return nil
				})
			},
			expectErr:       false,
			expectInterface: true,
		},
		{
			name: "Success - No VPC configuration",
			createConfig: &linodego.InstanceCreateOptions{
				Interfaces: []linodego.InstanceConfigInterfaceCreateOptions{},
			},
			mockSetup: func(mockLinodeClient *mock.MockLinodeClient, mockK8sClient *mock.MockK8sClient) {
				// No expectations needed
			},
			expectErr:       false,
			expectInterface: false,
		},
		{
			name:         "Error - VPCID on machine, VPC not found",
			machineVPCID: ptr.To(999),
			createConfig: &linodego.InstanceCreateOptions{
				Interfaces: []linodego.InstanceConfigInterfaceCreateOptions{},
			},
			mockSetup: func(mockLinodeClient *mock.MockLinodeClient, mockK8sClient *mock.MockK8sClient) {
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), 999).Return(nil, fmt.Errorf("VPC not found"))
			},
			expectErr:    true,
			expectErrMsg: "VPC not found",
		},
		{
			name:         "Error - VPCID on cluster, VPC not found",
			clusterVPCID: ptr.To(999),
			createConfig: &linodego.InstanceCreateOptions{
				Interfaces: []linodego.InstanceConfigInterfaceCreateOptions{},
			},
			mockSetup: func(mockLinodeClient *mock.MockLinodeClient, mockK8sClient *mock.MockK8sClient) {
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), 999).Return(nil, fmt.Errorf("VPC not found"))
			},
			expectErr:    true,
			expectErrMsg: "VPC not found",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Setup mock controller and clients
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLinodeClient := mock.NewMockLinodeClient(ctrl)
			mockK8sClient := mock.NewMockK8sClient(ctrl)

			tc.mockSetup(mockLinodeClient, mockK8sClient)

			// Create test context
			ctx := t.Context()
			logger := testr.New(t)

			// Create machine scope
			machineScope := &scope.MachineScope{
				LinodeClient: mockLinodeClient,
				Client:       mockK8sClient,
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					Spec: infrav1alpha2.LinodeMachineSpec{
						VPCID:  tc.machineVPCID,
						VPCRef: tc.vpcRef,
					},
				},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						VPCID:  tc.clusterVPCID,
						VPCRef: tc.vpcRef,
					},
				},
			}

			// Store original interface count
			originalCount := len(tc.createConfig.Interfaces)

			// Call the function being tested
			err := configureVPCInterface(ctx, machineScope, tc.createConfig, logger)

			// Check expectations
			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErrMsg)
			} else {
				require.NoError(t, err)
				if tc.expectInterface {
					// If interface was added, count should increase
					require.Len(t, tc.createConfig.Interfaces, originalCount+1)
					require.Equal(t, linodego.InterfacePurposeVPC, tc.createConfig.Interfaces[0].Purpose)
				} else {
					// If no interface was added, count should remain the same
					require.Len(t, tc.createConfig.Interfaces, originalCount)
				}
			}
		})
	}
}

func TestGetVPCInterfaceConfig(t *testing.T) {
	t.Parallel()

	// Setup test cases
	testCases := []struct {
		name            string
		vpcRef          *corev1.ObjectReference
		interfaces      []linodego.InstanceConfigInterfaceCreateOptions
		subnetName      string
		mockSetup       func(mockK8sClient *mock.MockK8sClient)
		expectErr       bool
		expectErrMsg    string
		expectInterface bool
		expectSubnetID  int
	}{
		{
			name: "Success - Finding VPC with default namespace",
			vpcRef: &corev1.ObjectReference{
				Name: "test-vpc",
			},
			interfaces: []linodego.InstanceConfigInterfaceCreateOptions{},
			mockSetup: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Get(gomock.Any(), client.ObjectKey{
					Name:      "test-vpc",
					Namespace: "default", // Default namespace
				}, gomock.Any()).DoAndReturn(func(_ context.Context, _ client.ObjectKey, vpc *infrav1alpha2.LinodeVPC, _ ...client.GetOption) error {
					vpc.Status.Ready = true
					vpc.Spec.VPCID = ptr.To(123)
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{
						{
							SubnetID: 456,
							Label:    "subnet-1",
						},
					}
					return nil
				})
			},
			expectErr:       false,
			expectInterface: true,
			expectSubnetID:  456, // First subnet ID
		},
		{
			name: "Success - Finding VPC with specific namespace",
			vpcRef: &corev1.ObjectReference{
				Name:      "test-vpc",
				Namespace: "custom-namespace",
			},
			interfaces: []linodego.InstanceConfigInterfaceCreateOptions{},
			mockSetup: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Get(gomock.Any(), client.ObjectKey{
					Name:      "test-vpc",
					Namespace: "custom-namespace",
				}, gomock.Any()).DoAndReturn(func(_ context.Context, _ client.ObjectKey, vpc *infrav1alpha2.LinodeVPC, _ ...client.GetOption) error {
					vpc.Status.Ready = true
					vpc.Spec.VPCID = ptr.To(123)
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{
						{
							SubnetID: 456,
							Label:    "subnet-1",
						},
					}
					return nil
				})
			},
			expectErr:       false,
			expectInterface: true,
			expectSubnetID:  456,
		},
		{
			name: "Success - With subnet name specified and found",
			vpcRef: &corev1.ObjectReference{
				Name: "test-vpc",
			},
			subnetName: "subnet-2",
			interfaces: []linodego.InstanceConfigInterfaceCreateOptions{},
			mockSetup: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Get(gomock.Any(), client.ObjectKey{
					Name:      "test-vpc",
					Namespace: "default",
				}, gomock.Any()).DoAndReturn(func(_ context.Context, _ client.ObjectKey, vpc *infrav1alpha2.LinodeVPC, _ ...client.GetOption) error {
					vpc.Status.Ready = true
					vpc.Spec.VPCID = ptr.To(123)
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{
						{
							SubnetID: 456,
							Label:    "subnet-1",
						},
						{
							SubnetID: 789,
							Label:    "subnet-2",
						},
					}
					return nil
				})
			},
			expectErr:       false,
			expectInterface: true,
			expectSubnetID:  789,
		},
		{
			name: "Success - VPC interface already exists",
			vpcRef: &corev1.ObjectReference{
				Name: "test-vpc",
			},
			interfaces: []linodego.InstanceConfigInterfaceCreateOptions{
				{
					Purpose: linodego.InterfacePurposeVPC,
				},
			},
			mockSetup: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Get(gomock.Any(), client.ObjectKey{
					Name:      "test-vpc",
					Namespace: "default",
				}, gomock.Any()).DoAndReturn(func(_ context.Context, _ client.ObjectKey, vpc *infrav1alpha2.LinodeVPC, _ ...client.GetOption) error {
					vpc.Status.Ready = true
					vpc.Spec.VPCID = ptr.To(123)
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{
						{
							SubnetID: 456,
							Label:    "subnet-1",
						},
					}
					return nil
				})
			},
			expectErr:       false,
			expectInterface: false,
			expectSubnetID:  456,
		},
		{
			name: "Error - Failed to fetch LinodeVPC",
			vpcRef: &corev1.ObjectReference{
				Name: "nonexistent-vpc",
			},
			interfaces: []linodego.InstanceConfigInterfaceCreateOptions{},
			mockSetup: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Get(gomock.Any(), client.ObjectKey{
					Name:      "nonexistent-vpc",
					Namespace: "default",
				}, gomock.Any()).Return(fmt.Errorf("vpc not found"))
			},
			expectErr:       true,
			expectErrMsg:    "vpc not found",
			expectInterface: false,
		},
		{
			name: "Error - VPC is not ready",
			vpcRef: &corev1.ObjectReference{
				Name: "test-vpc",
			},
			interfaces: []linodego.InstanceConfigInterfaceCreateOptions{},
			mockSetup: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Get(gomock.Any(), client.ObjectKey{
					Name:      "test-vpc",
					Namespace: "default",
				}, gomock.Any()).DoAndReturn(func(_ context.Context, _ client.ObjectKey, vpc *infrav1alpha2.LinodeVPC, _ ...client.GetOption) error {
					vpc.Status.Ready = false
					vpc.Spec.VPCID = ptr.To(123)
					return nil
				})
			},
			expectErr:       true,
			expectErrMsg:    "vpc is not available",
			expectInterface: false,
		},
		{
			name: "Error - VPC has no subnets",
			vpcRef: &corev1.ObjectReference{
				Name: "test-vpc",
			},
			interfaces: []linodego.InstanceConfigInterfaceCreateOptions{},
			mockSetup: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Get(gomock.Any(), client.ObjectKey{
					Name:      "test-vpc",
					Namespace: "default",
				}, gomock.Any()).DoAndReturn(func(_ context.Context, _ client.ObjectKey, vpc *infrav1alpha2.LinodeVPC, _ ...client.GetOption) error {
					vpc.Status.Ready = true
					vpc.Spec.VPCID = ptr.To(123)
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{}
					return nil
				})
			},
			expectErr:       true,
			expectErrMsg:    "failed to find subnet",
			expectInterface: false,
		},
		{
			name: "Error - Subnet name not found",
			vpcRef: &corev1.ObjectReference{
				Name: "test-vpc",
			},
			subnetName: "nonexistent-subnet",
			interfaces: []linodego.InstanceConfigInterfaceCreateOptions{},
			mockSetup: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Get(gomock.Any(), client.ObjectKey{
					Name:      "test-vpc",
					Namespace: "default",
				}, gomock.Any()).DoAndReturn(func(_ context.Context, _ client.ObjectKey, vpc *infrav1alpha2.LinodeVPC, _ ...client.GetOption) error {
					vpc.Status.Ready = true
					vpc.Spec.VPCID = ptr.To(123)
					vpc.Spec.Subnets = []infrav1alpha2.VPCSubnetCreateOptions{
						{
							SubnetID: 456,
							Label:    "subnet-1",
						},
					}
					return nil
				})
			},
			expectErr:       true,
			expectErrMsg:    "failed to find subnet as subnet id set is 0",
			expectInterface: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Setup mock controller and client
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockK8sClient := mock.NewMockK8sClient(ctrl)
			mockLinodeClient := mock.NewMockLinodeClient(ctrl)

			tc.mockSetup(mockK8sClient)

			// Create test context
			ctx := t.Context()
			logger := testr.New(t)

			// Create machine scope
			machineScope := &scope.MachineScope{
				LinodeClient: mockLinodeClient,
				Client:       mockK8sClient,
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
					},
					Spec: infrav1alpha2.LinodeMachineSpec{},
				},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							SubnetName: tc.subnetName,
						},
					},
				},
			}

			// Call the function being tested
			iface, err := getVPCInterfaceConfig(ctx, machineScope, tc.interfaces, logger, tc.vpcRef)

			// Check expectations
			validateInterfaceExpectations(t, err, iface, tc.expectErr, tc.expectErrMsg, tc.expectInterface, tc.expectSubnetID, tc.interfaces)

			// Additional check for interface updates
			if !tc.expectErr && !tc.expectInterface && len(tc.interfaces) > 0 && tc.interfaces[0].Purpose == linodego.InterfacePurposeVPC {
				require.NotNil(t, tc.interfaces[0].SubnetID)
				require.Equal(t, tc.expectSubnetID, *tc.interfaces[0].SubnetID)
			}
		})
	}
}

func TestConvertStrArrayToSet(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		inpStr string
		expErr string
		expSet map[string]struct{}
	}{
		{
			name:   "Empty string",
			inpStr: "[]",
			expSet: map[string]struct{}{},
		},
		{
			name:   "Invalid string",
			inpStr: "test-string",
			expErr: "invalid character 'e' in literal true (expecting 'r')",
		},
		{
			name:   "valid string",
			inpStr: "[\"tag1\", \"tag2\", \"tag2\"]",
			expSet: map[string]struct{}{
				"tag1": {},
				"tag2": {},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out, err := convertStrArrayToSet(tc.inpStr)

			if tc.expErr != "" {
				require.ErrorContains(t, err, tc.expErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expSet, out)
			}
		})
	}
}

func TestGetTags(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name             string
		clusterName      string
		currInstanceTags []string
		machine          *infrav1alpha2.LinodeMachine
		expMachine       *infrav1alpha2.LinodeMachine
		expInstanceTags  []string
	}{
		{
			name:            "Success - No Tags annotation",
			machine:         &infrav1alpha2.LinodeMachine{},
			expInstanceTags: []string{},
			expMachine: &infrav1alpha2.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						machineTagsAnnotation:            "[]",
						machineLastAppliedTagsAnnotation: "[]",
					},
				},
			},
		},
		{
			name:            "Success - add capl-auto-gen tags",
			machine:         &infrav1alpha2.LinodeMachine{},
			clusterName:     "test-cluster",
			expInstanceTags: []string{"test-cluster"},
			expMachine: &infrav1alpha2.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						machineTagsAnnotation:            "[]",
						machineLastAppliedTagsAnnotation: "[]",
					},
				},
			},
		},
		{
			name:             "Success - add tags through annotation",
			clusterName:      "test-cluster",
			currInstanceTags: []string{"instance-manually-added-tag"},
			machine: &infrav1alpha2.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						machineTagsAnnotation: "[\"tag1\", \"tag2\"]",
					},
				},
			},
			expMachine: &infrav1alpha2.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						machineTagsAnnotation:            "[\"tag1\", \"tag2\"]",
						machineLastAppliedTagsAnnotation: "[\"tag1\", \"tag2\"]",
					},
				},
			},
			expInstanceTags: []string{"test-cluster", "tag1", "tag2", "instance-manually-added-tag"},
		},
		{
			name:             "Success - add tags with previous tags",
			clusterName:      "test-cluster",
			currInstanceTags: []string{"instance-manually-added-tag", "tag1", "tag2"},
			machine: &infrav1alpha2.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						machineLastAppliedTagsAnnotation: "[\"tag1\", \"tag2\"]",
						machineTagsAnnotation:            "[\"tag1\",\"tag2\",\"tag3\", \"tag4\"]",
					},
				},
			},
			expMachine: &infrav1alpha2.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						machineTagsAnnotation:            "[\"tag1\",\"tag2\",\"tag3\", \"tag4\"]",
						machineLastAppliedTagsAnnotation: "[\"tag1\",\"tag2\",\"tag3\", \"tag4\"]",
					},
				},
			},
			expInstanceTags: []string{"test-cluster", "tag1", "tag2", "instance-manually-added-tag", "tag3", "tag4"},
		},
		{
			name:             "Success - remove tags",
			clusterName:      "test-cluster",
			currInstanceTags: []string{"instance-manually-added-tag", "tag1", "tag2", "tag3", "tag4"},
			machine: &infrav1alpha2.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						machineTagsAnnotation:            "[\"tag3\", \"tag4\"]",
						machineLastAppliedTagsAnnotation: "[\"tag1\",\"tag2\",\"tag3\", \"tag4\"]",
					},
				},
			},
			expMachine: &infrav1alpha2.LinodeMachine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						machineTagsAnnotation:            "[\"tag3\", \"tag4\"]",
						machineLastAppliedTagsAnnotation: "[\"tag3\", \"tag4\"]",
					},
				},
			},
			expInstanceTags: []string{"test-cluster", "tag3", "tag4", "instance-manually-added-tag"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Setup mock controller and clients
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Call the function being tested.
			// ignoring error check since internal function calls throw errors.
			// internal functions are tested on their corresponding unit-test.
			out, _ := getTags(&scope.MachineScope{
				LinodeMachine: tc.machine,
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: tc.clusterName,
					},
				},
			}, tc.currInstanceTags)

			// Check expectations
			slices.Sort(tc.expInstanceTags)
			slices.Sort(out)

			require.Equal(t, tc.expInstanceTags, out)
			require.Equal(t, tc.expMachine.Annotations, tc.machine.Annotations)
		})
	}
}
