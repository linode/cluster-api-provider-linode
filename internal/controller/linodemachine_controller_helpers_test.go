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
	"sigs.k8s.io/cluster-api/api/core/v1beta2"
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
		LinodeInterfaces: []infrav1alpha2.LinodeInterfaceCreateOptions{{
			FirewallID: ptr.To(123),
			DefaultRoute: &infrav1alpha2.InterfaceDefaultRoute{
				IPv4: ptr.To(true),
				IPv6: ptr.To(true),
			},
			Public: &infrav1alpha2.PublicInterfaceCreateOptions{
				IPv4: &infrav1alpha2.PublicInterfaceIPv4CreateOptions{Addresses: []infrav1alpha2.PublicInterfaceIPv4AddressCreateOptions{{
					Address: "1.2.3.4",
					Primary: nil,
				}}},
				IPv6: &infrav1alpha2.PublicInterfaceIPv6CreateOptions{Ranges: []infrav1alpha2.PublicInterfaceIPv6RangeCreateOptions{{
					Range: "1234:5678:90ab:cdef:1234:5678:90ab:cdef/64",
				}}},
			},
		}, {
			FirewallID: ptr.To(123),
			DefaultRoute: &infrav1alpha2.InterfaceDefaultRoute{
				IPv4: ptr.To(true),
				IPv6: ptr.To(true),
			},
			VPC: &infrav1alpha2.VPCInterfaceCreateOptions{
				IPv4: &infrav1alpha2.VPCInterfaceIPv4CreateOptions{Addresses: []infrav1alpha2.VPCInterfaceIPv4AddressCreateOptions{{
					Address:        "1.2.3.4",
					Primary:        nil,
					NAT1To1Address: ptr.To("true"),
				}},
					Ranges: []infrav1alpha2.VPCInterfaceIPv4RangeCreateOptions{{
						Range: "1.2.3.4/32",
					}}},
				IPv6: &infrav1alpha2.VPCInterfaceIPv6CreateOptions{
					SLAAC: []infrav1alpha2.VPCInterfaceIPv6SLAACCreateOptions{{Range: "1234:5678:90ab:cdef:1234:5678:90ab:cdef/64"}},
					Ranges: []infrav1alpha2.VPCInterfaceIPv6RangeCreateOptions{{
						Range: "1234:5678:90ab:cdef:1234:5678:90ab:cdef/64",
					}},
					IsPublic: ptr.To(false),
				},
			},
		}, {
			FirewallID: ptr.To(123),
			DefaultRoute: &infrav1alpha2.InterfaceDefaultRoute{
				IPv4: ptr.To(true),
				IPv6: ptr.To(true),
			},
			VLAN: &infrav1alpha2.VLANInterface{
				VLANLabel:   "test-label",
				IPAMAddress: nil,
			},
		}},
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
			machineScope: &scope.MachineScope{Machine: &v1beta2.Machine{
				Spec: v1beta2.MachineSpec{
					ClusterName: "",
					Bootstrap: v1beta2.Bootstrap{
						DataSecretName: ptr.To("test-data"),
					},
					InfrastructureRef: v1beta2.ContractVersionedObjectReference{},
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
			machineScope: &scope.MachineScope{Machine: &v1beta2.Machine{
				Spec: v1beta2.MachineSpec{
					ClusterName: "",
					Bootstrap: v1beta2.Bootstrap{
						DataSecretName: ptr.To("test-data"),
					},
					InfrastructureRef: v1beta2.ContractVersionedObjectReference{},
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
			machineScope: &scope.MachineScope{Machine: &v1beta2.Machine{
				Spec: v1beta2.MachineSpec{
					ClusterName: "",
					Bootstrap: v1beta2.Bootstrap{
						DataSecretName: nil,
					},
					InfrastructureRef: v1beta2.ContractVersionedObjectReference{},
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
			machineScope: &scope.MachineScope{Machine: &v1beta2.Machine{
				Spec: v1beta2.MachineSpec{
					ClusterName: "",
					Bootstrap: v1beta2.Bootstrap{
						DataSecretName: ptr.To("test-data"),
					},
					InfrastructureRef: v1beta2.ContractVersionedObjectReference{},
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
		instanceDisks   *infrav1alpha2.InstanceDisks
		expectedDiskMap linodego.InstanceConfigDeviceMap
		expectedError   error
	}{
		{
			name: "Success - single disk gets added to config",
			instanceDisks: &infrav1alpha2.InstanceDisks{SDB: &infrav1alpha2.InstanceDisk{
				DiskID: 101,
				Size:   resource.MustParse("10Gi"),
				Label:  "disk1",
			}},
			expectedDiskMap: linodego.InstanceConfigDeviceMap{SDA: &linodego.InstanceConfigDevice{DiskID: 100},
				SDB: &linodego.InstanceConfigDevice{DiskID: 101},
			},
		},
		{
			name:            "Success - no disks",
			instanceDisks:   nil,
			expectedDiskMap: linodego.InstanceConfigDeviceMap{SDA: &linodego.InstanceConfigDevice{DiskID: 100}},
		},
		{
			name: "Success - multiple disks gets added to config",
			instanceDisks: &infrav1alpha2.InstanceDisks{SDB: &infrav1alpha2.InstanceDisk{
				DiskID: 101,
				Size:   resource.MustParse("10Gi"),
				Label:  "disk1",
			}, SDC: &infrav1alpha2.InstanceDisk{
				DiskID: 102,
				Size:   resource.MustParse("10Gi"),
				Label:  "disk1",
			}, SDD: &infrav1alpha2.InstanceDisk{
				DiskID: 103,
				Size:   resource.MustParse("10Gi"),
				Label:  "disk1",
			}, SDE: &infrav1alpha2.InstanceDisk{
				DiskID: 104,
				Size:   resource.MustParse("10Gi"),
				Label:  "disk1",
			}, SDF: &infrav1alpha2.InstanceDisk{
				DiskID: 105,
				Size:   resource.MustParse("10Gi"),
				Label:  "disk1",
			}, SDG: &infrav1alpha2.InstanceDisk{
				DiskID: 106,
				Size:   resource.MustParse("10Gi"),
				Label:  "disk1",
			}, SDH: &infrav1alpha2.InstanceDisk{
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
			createInstanceConfigDeviceMap(testcase.instanceDisks, actualConfig.Devices)
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
func disksEqual(t *testing.T, expected, actual *infrav1alpha2.InstanceDisks) {
	t.Helper()
	if expected == nil || actual == nil {
		return
	}
	if expected.SDB != nil {
		assert.Equal(t, expected.SDB.DiskID, actual.SDB.DiskID)
	}
	if expected.SDC != nil {
		assert.Equal(t, expected.SDC.DiskID, actual.SDC.DiskID)
	}
	if expected.SDD != nil {
		assert.Equal(t, expected.SDD.DiskID, actual.SDD.DiskID)
	}
	if expected.SDE != nil {
		assert.Equal(t, expected.SDE.DiskID, actual.SDE.DiskID)
	}
	if expected.SDF != nil {
		assert.Equal(t, expected.SDF.DiskID, actual.SDF.DiskID)
	}
	if expected.SDG != nil {
		assert.Equal(t, expected.SDG.DiskID, actual.SDG.DiskID)
	}
	if expected.SDH != nil {
		assert.Equal(t, expected.SDH.DiskID, actual.SDH.DiskID)
	}
}
func TestCreateDisks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                  string
		instanceDisks         *infrav1alpha2.InstanceDisks
		instanceDiskIDs       map[string]int
		expectedInstanceDisks *infrav1alpha2.InstanceDisks
		expectedError         string
	}{
		{
			name: "Success - single disk created",
			instanceDisks: &infrav1alpha2.InstanceDisks{SDB: &infrav1alpha2.InstanceDisk{
				Size:       resource.MustParse("10Gi"),
				Filesystem: "raw",
			}},
			instanceDiskIDs: map[string]int{"sdb": 101},
			expectedInstanceDisks: &infrav1alpha2.InstanceDisks{SDB: &infrav1alpha2.InstanceDisk{
				DiskID: 101,
				Size:   resource.MustParse("10Gi"),
			}},
		},
		{
			name:                  "Success - no disks",
			expectedInstanceDisks: &infrav1alpha2.InstanceDisks{},
		},
		{
			name: "Success - single existing disk used",
			instanceDisks: &infrav1alpha2.InstanceDisks{SDB: &infrav1alpha2.InstanceDisk{
				Size:       resource.MustParse("10Gi"),
				Filesystem: "raw",
				DiskID:     101,
			}},
			expectedInstanceDisks: &infrav1alpha2.InstanceDisks{SDB: &infrav1alpha2.InstanceDisk{
				DiskID: 101,
				Size:   resource.MustParse("10Gi"),
			}},
		},
		{
			name: "Failure - sdb",
			instanceDisks: &infrav1alpha2.InstanceDisks{SDB: &infrav1alpha2.InstanceDisk{
				Size:       resource.MustParse("10Gi"),
				Filesystem: "raw",
			}},
			instanceDiskIDs: map[string]int{"sdb": 101},
			expectedError:   "failed to create disk: sdb",
		},
		{
			name: "Failure - sdc",
			instanceDisks: &infrav1alpha2.InstanceDisks{SDC: &infrav1alpha2.InstanceDisk{
				Size:       resource.MustParse("10Gi"),
				Filesystem: "raw",
			}},
			instanceDiskIDs: map[string]int{"sdc": 101},
			expectedError:   "failed to create disk: sdc",
		},
		{
			name: "Failure - sdd",
			instanceDisks: &infrav1alpha2.InstanceDisks{SDD: &infrav1alpha2.InstanceDisk{
				Size:       resource.MustParse("10Gi"),
				Filesystem: "raw",
			}},
			instanceDiskIDs: map[string]int{"sdd": 101},
			expectedError:   "failed to create disk: sdd",
		},
		{
			name: "Failure - sde",
			instanceDisks: &infrav1alpha2.InstanceDisks{SDE: &infrav1alpha2.InstanceDisk{
				Size:       resource.MustParse("10Gi"),
				Filesystem: "raw",
			}},
			instanceDiskIDs: map[string]int{"sde": 101},
			expectedError:   "failed to create disk: sde",
		},
		{
			name: "Failure - sdf",
			instanceDisks: &infrav1alpha2.InstanceDisks{SDF: &infrav1alpha2.InstanceDisk{
				Size:       resource.MustParse("10Gi"),
				Filesystem: "raw",
			}},
			instanceDiskIDs: map[string]int{"sdf": 101},
			expectedError:   "failed to create disk: sdf",
		},
		{
			name: "Failure - sdg",
			instanceDisks: &infrav1alpha2.InstanceDisks{SDG: &infrav1alpha2.InstanceDisk{
				Size:       resource.MustParse("10Gi"),
				Filesystem: "raw",
			}},
			instanceDiskIDs: map[string]int{"sdg": 101},
			expectedError:   "failed to create disk: sdg",
		},
		{
			name: "Failure - sdh",
			instanceDisks: &infrav1alpha2.InstanceDisks{SDH: &infrav1alpha2.InstanceDisk{
				Size:       resource.MustParse("10Gi"),
				Filesystem: "raw",
			}},
			instanceDiskIDs: map[string]int{"sdh": 101},
			expectedError:   "failed to create disk: sdh",
		},
	}
	for _, tc := range tests {
		testcase := tc
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			// Create test context
			ctx := t.Context()
			logger := testr.New(t)
			ctrl := gomock.NewController(t)
			mockLinodeClient := mock.NewMockLinodeClient(ctrl)
			mockK8sClient := mock.NewMockK8sClient(ctrl)
			machineScope := &scope.MachineScope{
				LinodeClient:  mockLinodeClient,
				Client:        mockK8sClient,
				LinodeMachine: &infrav1alpha2.LinodeMachine{Spec: infrav1alpha2.LinodeMachineSpec{DataDisks: testcase.instanceDisks, InstanceID: ptr.To(123)}},
				LinodeCluster: &infrav1alpha2.LinodeCluster{Spec: infrav1alpha2.LinodeClusterSpec{}},
			}
			for device, id := range testcase.instanceDiskIDs {
				if testcase.expectedError == "" {
					mockLinodeClient.EXPECT().CreateInstanceDisk(ctx, 123, linodego.InstanceDiskCreateOptions{
						Label:      device,
						Size:       10738,
						Filesystem: "raw",
					}).Return(&linodego.InstanceDisk{
						ID: id,
					}, nil)
				} else {
					mockLinodeClient.EXPECT().CreateInstanceDisk(ctx, 123, gomock.Any()).Return(nil, fmt.Errorf("failed to create disk: %s", device))
				}
			}
			if testcase.instanceDisks != nil && testcase.expectedError == "" {
				mockLinodeClient.EXPECT().ListInstanceConfigs(ctx, 123, gomock.Any()).Return([]linodego.InstanceConfig{{
					ID:      123,
					Devices: &linodego.InstanceConfigDeviceMap{},
				}}, nil)
				mockLinodeClient.EXPECT().UpdateInstanceConfig(ctx, 123, 123, gomock.Any()).Return(&linodego.InstanceConfig{}, nil)
			}
			err := createDisks(ctx, logger, machineScope, 123)
			if testcase.expectedError != "" {
				assert.ErrorContainsf(t, err, testcase.expectedError, "expected an error containing %s", testcase.expectedError)
			} else {
				disksEqual(t, testcase.instanceDisks, testcase.expectedInstanceDisks)
				assert.NoError(t, err)
			}
		})
	}
}

// validateInterfaceExpectations is a helper function to check VPC interface expectations in tests
func validateInterfaceExpectations(
	t *testing.T,
	err error,
	iface *linodego.InstanceConfigInterfaceCreateOptions,
	linodeIface *linodego.LinodeInterfaceCreateOptions,
	expectErr bool,
	expectErrMsg string,
	expectInterface bool,
	expectLinodeInterface bool,
	expectSubnetID int,
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
		if iface.IPv6 != nil && iface.IPv6.SLAAC != nil {
			require.Equal(t, defaultNodeIPv6CIDRRange, iface.IPv6.SLAAC[0].Range)
		} else if iface.IPv6 != nil && iface.IPv6.Ranges != nil {
			require.Equal(t, defaultNodeIPv6CIDRRange, *iface.IPv6.Ranges[0].Range)
		}
		require.True(t, iface.Primary)
		require.NotNil(t, iface.SubnetID)
		require.Equal(t, expectSubnetID, *iface.SubnetID)
		require.NotNil(t, iface.IPv4)
		require.NotNil(t, iface.IPv4.NAT1To1)
		require.Equal(t, "any", *iface.IPv4.NAT1To1)
	} else {
		require.Nil(t, iface)
	}
	if expectLinodeInterface {
		require.NotNil(t, linodeIface)
		require.NotNil(t, linodeIface.VPC)
		if linodeIface.VPC.IPv6 != nil && linodeIface.VPC.IPv6.SLAAC != nil {
			slaac := *linodeIface.VPC.IPv6.SLAAC
			require.Equal(t, defaultNodeIPv6CIDRRange, slaac[0].Range)
		} else if linodeIface.VPC.IPv6 != nil && linodeIface.VPC.IPv6.Ranges != nil {
			ranges := *linodeIface.VPC.IPv6.Ranges
			require.Equal(t, defaultNodeIPv6CIDRRange, ranges[0].Range)
		}
		require.NotNil(t, linodeIface.VPC.SubnetID)
		require.Equal(t, expectSubnetID, linodeIface.VPC.SubnetID)
		require.NotNil(t, linodeIface.VPC.IPv4)
		require.NotNil(t, linodeIface.VPC.IPv4.Addresses)
		addresses := *linodeIface.VPC.IPv4.Addresses
		require.NotNil(t, addresses[0].NAT1To1Address)
		require.Equal(t, "auto", *addresses[0].NAT1To1Address)
	} else {
		require.Nil(t, linodeIface)
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
			name:       "Success - Valid VPC with subnets and ipv6 ranges, specific subnet name",
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
							IPv6: []linodego.VPCIPv6Range{
								{
									Range: "2001:0db8::/56",
								},
							},
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
							IPv6: []linodego.VPCIPv6Range{
								{
									Range: "2001:0db8::/56",
								},
							},
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
			validateInterfaceExpectations(t, err, iface, nil, tc.expectErr, tc.expectErrMsg, tc.expectInterface, false, tc.expectSubnetID)

			// Additional check for interface updates
			if !tc.expectErr && !tc.expectInterface && len(tc.interfaces) > 0 && tc.interfaces[0].Purpose == linodego.InterfacePurposeVPC {
				require.NotNil(t, tc.interfaces[0].SubnetID)
				require.Equal(t, tc.expectSubnetID, *tc.interfaces[0].SubnetID)
			}
		})
	}
}

func TestGetVPCLinodeInterfaceConfigFromDirectID(t *testing.T) {
	t.Parallel()

	// Setup test cases
	testCases := []struct {
		name                  string
		vpcID                 int
		linodeInterfaces      []linodego.LinodeInterfaceCreateOptions
		subnetName            string
		mockSetup             func(mockLinodeClient *mock.MockLinodeClient)
		expectErr             bool
		expectErrMsg          string
		expectLinodeInterface bool
		expectSubnetID        int
	}{
		{
			name:             "Success - Valid VPC with subnets, no subnet name",
			vpcID:            123,
			linodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
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
			expectErr:             false,
			expectLinodeInterface: true,
			expectSubnetID:        456, // First subnet ID
		},
		{
			name:             "Success - Valid VPC with subnets, specific subnet name",
			vpcID:            123,
			linodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
			subnetName:       "subnet-2",
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
			expectErr:             false,
			expectLinodeInterface: true,
			expectSubnetID:        789, // Matching subnet ID
		},
		{
			name:             "Success - Valid VPC with subnets and ipv6 ranges, specific subnet name",
			vpcID:            123,
			linodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
			subnetName:       "subnet-2",
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
							IPv6: []linodego.VPCIPv6Range{
								{
									Range: "2001:0db8::/56",
								},
							},
						},
					},
				}, nil)
			},
			expectErr:             false,
			expectLinodeInterface: true,
			expectSubnetID:        789, // Matching subnet ID
		},
		{
			name:             "Success - VPC interface already exists",
			vpcID:            123,
			linodeInterfaces: []linodego.LinodeInterfaceCreateOptions{{VPC: &linodego.VPCInterfaceCreateOptions{}}},
			mockSetup: func(mockLinodeClient *mock.MockLinodeClient) {
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), 123).Return(&linodego.VPC{
					ID: 123,
					Subnets: []linodego.VPCSubnet{
						{
							ID:    456,
							Label: "subnet-1",
							IPv6: []linodego.VPCIPv6Range{
								{
									Range: "2001:0db8::/56",
								},
							},
						},
					},
				}, nil)
			},
			expectErr:             false,
			expectLinodeInterface: false,
			expectSubnetID:        456,
		},
		{
			name:             "Error - VPC does not exist",
			vpcID:            999,
			linodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
			mockSetup: func(mockLinodeClient *mock.MockLinodeClient) {
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), 999).Return(nil, fmt.Errorf("VPC not found"))
			},
			expectErr:             true,
			expectErrMsg:          "VPC not found",
			expectLinodeInterface: false,
		},
		{
			name:             "Error - VPC has no subnets",
			vpcID:            123,
			linodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
			mockSetup: func(mockLinodeClient *mock.MockLinodeClient) {
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), 123).Return(&linodego.VPC{
					ID:      123,
					Subnets: []linodego.VPCSubnet{},
				}, nil)
			},
			expectErr:             true,
			expectErrMsg:          "no subnets found in VPC",
			expectLinodeInterface: false,
		},
		{
			name:             "Error - Subnet name not found",
			vpcID:            123,
			linodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
			subnetName:       "non-existent",
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
			expectErr:             true,
			expectErrMsg:          "subnet with label non-existent not found in VPC",
			expectLinodeInterface: false,
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
			linodeIface, err := getVPCLinodeInterfaceConfigFromDirectID(ctx, machineScope, tc.linodeInterfaces, logger, tc.vpcID)

			// Check expectations
			validateInterfaceExpectations(t, err, nil, linodeIface, tc.expectErr, tc.expectErrMsg, false, tc.expectLinodeInterface, tc.expectSubnetID)

			// Additional check for interface updates
			if !tc.expectErr && !tc.expectLinodeInterface && len(tc.linodeInterfaces) > 0 && tc.linodeInterfaces[0].VPC != nil {
				require.NotNil(t, tc.linodeInterfaces[0].VPC.SubnetID)
				require.Equal(t, tc.expectSubnetID, tc.linodeInterfaces[0].VPC.SubnetID)
			}
		})
	}
}

func TestAddVPCInterfaceFromDirectID(t *testing.T) {
	t.Parallel()

	// Setup test cases
	testCases := []struct {
		name                string
		vpcID               int
		createConfig        *linodego.InstanceCreateOptions
		mockSetup           func(mockLinodeClient *mock.MockLinodeClient)
		expectErr           bool
		expectErrMsg        string
		expectNoIface       bool
		expectNoLinodeIface bool
	}{
		{
			name:  "Success - Interface added correctly with new network interfaces",
			vpcID: 123,
			createConfig: &linodego.InstanceCreateOptions{
				LinodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
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
			expectErr:           false,
			expectNoLinodeIface: true,
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
			name:  "Error - getVPCInterfaceConfigFromDirectID returns error with new network interfaces",
			vpcID: 999,
			createConfig: &linodego.InstanceCreateOptions{
				LinodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
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
			expectErr:           false,
			expectNoIface:       true,
			expectNoLinodeIface: true,
		},
		{
			name:  "Success - Interface already exists with new network interfaces",
			vpcID: 123,
			createConfig: &linodego.InstanceCreateOptions{
				LinodeInterfaces: []linodego.LinodeInterfaceCreateOptions{
					{
						VPC: &linodego.VPCInterfaceCreateOptions{},
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
			expectErr:           false,
			expectNoIface:       true,
			expectNoLinodeIface: true,
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
				LinodeCluster: &infrav1alpha2.LinodeCluster{},
			}

			// Store original interface count
			originalIfaceCount := len(tc.createConfig.Interfaces)
			originalLinodeIfaceCount := len(tc.createConfig.LinodeInterfaces)

			// Call the function being tested
			err := addVPCInterfaceFromDirectID(ctx, machineScope, tc.createConfig, logger, tc.vpcID)

			// Check expectations
			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErrMsg)
			} else {
				require.NoError(t, err)
				switch {
				case tc.expectNoLinodeIface:
					// If Linode interface already existed, count should remain the same
					require.Len(t, tc.createConfig.LinodeInterfaces, originalLinodeIfaceCount)
				default:
					// If Linode interface was added, count should increase
					require.Len(t, tc.createConfig.LinodeInterfaces, originalLinodeIfaceCount+1)
					require.NotNil(t, tc.createConfig.LinodeInterfaces[0].VPC)
				}
				switch {
				case tc.expectNoIface:
					// If interface already existed, count should remain the same
					require.Len(t, tc.createConfig.Interfaces, originalIfaceCount)
				default:
					// If interface was added, count should increase
					require.Len(t, tc.createConfig.Interfaces, originalIfaceCount+1)
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
		name                  string
		machineVPCID          *int
		clusterVPCID          *int
		vpcRef                *corev1.ObjectReference
		createConfig          *linodego.InstanceCreateOptions
		mockSetup             func(mockLinodeClient *mock.MockLinodeClient, mockK8sClient *mock.MockK8sClient)
		expectErr             bool
		expectErrMsg          string
		expectInterface       bool
		expectLinodeInterface bool
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
							IPv6: []linodego.VPCIPv6Range{
								{
									Range: "2001:0db8::/56",
								},
							},
						},
					},
				}, nil)
			},
			expectErr:       false,
			expectInterface: true,
		},
		{
			name:         "Success - VPCID on machine with new network interfaces",
			machineVPCID: ptr.To(123),
			createConfig: &linodego.InstanceCreateOptions{
				LinodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
			},
			mockSetup: func(mockLinodeClient *mock.MockLinodeClient, mockK8sClient *mock.MockK8sClient) {
				mockLinodeClient.EXPECT().GetVPC(gomock.Any(), 123).Return(&linodego.VPC{
					ID: 123,
					Subnets: []linodego.VPCSubnet{
						{
							ID:    456,
							Label: "subnet-1",
							IPv6: []linodego.VPCIPv6Range{
								{
									Range: "2001:0db8::/56",
								},
							},
						},
					},
				}, nil)
			},
			expectErr:             false,
			expectLinodeInterface: true,
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
			name:         "Success - VPCID on cluster with new network interfaces",
			clusterVPCID: ptr.To(123),
			createConfig: &linodego.InstanceCreateOptions{
				LinodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
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
			expectErr:             false,
			expectLinodeInterface: true,
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
			name:   "Success - VPC reference with new network interfaces",
			vpcRef: vpcRef,
			createConfig: &linodego.InstanceCreateOptions{
				LinodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
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
			expectErr:             false,
			expectLinodeInterface: true,
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
			name: "Success - No VPC configuration with new network interfaces",
			createConfig: &linodego.InstanceCreateOptions{
				LinodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
			},
			mockSetup: func(mockLinodeClient *mock.MockLinodeClient, mockK8sClient *mock.MockK8sClient) {
				// No expectations needed
			},
			expectErr:             false,
			expectLinodeInterface: false,
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
			name:         "Error - VPCID on machine, VPC not found with new network interfaces",
			machineVPCID: ptr.To(999),
			createConfig: &linodego.InstanceCreateOptions{
				LinodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
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
		{
			name:         "Error - VPCID on cluster, VPC not found with new network interfaces",
			clusterVPCID: ptr.To(999),
			createConfig: &linodego.InstanceCreateOptions{
				LinodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
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
			originalIfaceCount := len(tc.createConfig.Interfaces)
			originalLinodeIfaceCount := len(tc.createConfig.LinodeInterfaces)

			// Call the function being tested
			err := configureVPCInterface(ctx, machineScope, tc.createConfig, logger)

			// Check expectations
			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErrMsg)
			} else {
				require.NoError(t, err)
				switch {
				case tc.expectLinodeInterface:
					// If Linode interface was added, count should increase
					require.Len(t, tc.createConfig.LinodeInterfaces, originalLinodeIfaceCount+1)
					require.NotNil(t, tc.createConfig.LinodeInterfaces[0].VPC)
				default:
					// If no Linode interface was added, count should remain the same
					require.Len(t, tc.createConfig.LinodeInterfaces, originalLinodeIfaceCount)
				}
				switch {
				case tc.expectInterface:
					// If interface was added, count should increase
					require.Len(t, tc.createConfig.Interfaces, originalIfaceCount+1)
					require.Equal(t, linodego.InterfacePurposeVPC, tc.createConfig.Interfaces[0].Purpose)
					require.True(t, tc.createConfig.Interfaces[0].Primary)
				default:
					// If no interface was added, count should remain the same
					require.Len(t, tc.createConfig.Interfaces, originalIfaceCount)
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
							IPv6: []linodego.VPCIPv6Range{
								{
									Range: "2001:0db8::/56",
								},
							},
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
							IPv6: []linodego.VPCIPv6Range{
								{
									Range: "2001:0db8::/56",
								},
							},
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
					Spec: infrav1alpha2.LinodeMachineSpec{
						IPv6Options: &infrav1alpha2.IPv6CreateOptions{
							EnableSLAAC:  ptr.To(true),
							IsPublicIPv6: ptr.To(true),
						},
					},
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
			validateInterfaceExpectations(t, err, iface, nil, tc.expectErr, tc.expectErrMsg, tc.expectInterface, false, tc.expectSubnetID)

			// Additional check for interface updates
			if !tc.expectErr && !tc.expectInterface && len(tc.interfaces) > 0 && tc.interfaces[0].Purpose == linodego.InterfacePurposeVPC {
				require.NotNil(t, tc.interfaces[0].SubnetID)
				require.Equal(t, tc.expectSubnetID, *tc.interfaces[0].SubnetID)
			}
		})
	}
}

func TestGetVPCLinodeInterfaceConfig(t *testing.T) {
	t.Parallel()

	// Setup test cases
	testCases := []struct {
		name                  string
		vpcRef                *corev1.ObjectReference
		linodeInterfaces      []linodego.LinodeInterfaceCreateOptions
		subnetName            string
		mockSetup             func(mockK8sClient *mock.MockK8sClient)
		expectErr             bool
		expectErrMsg          string
		expectLinodeInterface bool
		expectSubnetID        int
	}{
		{
			name: "Success - Finding VPC with default namespace",
			vpcRef: &corev1.ObjectReference{
				Name: "test-vpc",
			},
			linodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
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
							IPv6: []linodego.VPCIPv6Range{
								{
									Range: "2001:0db8::/56",
								},
							},
						},
					}
					return nil
				})
			},
			expectErr:             false,
			expectLinodeInterface: true,
			expectSubnetID:        456, // First subnet ID
		},
		{
			name: "Success - Finding VPC with specific namespace",
			vpcRef: &corev1.ObjectReference{
				Name:      "test-vpc",
				Namespace: "custom-namespace",
			},
			linodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
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
			expectErr:             false,
			expectLinodeInterface: true,
			expectSubnetID:        456,
		},
		{
			name: "Success - With subnet name specified and found",
			vpcRef: &corev1.ObjectReference{
				Name: "test-vpc",
			},
			subnetName:       "subnet-2",
			linodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
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
			expectErr:             false,
			expectLinodeInterface: true,
			expectSubnetID:        789,
		},
		{
			name: "Success - VPC interface already exists",
			vpcRef: &corev1.ObjectReference{
				Name: "test-vpc",
			},
			linodeInterfaces: []linodego.LinodeInterfaceCreateOptions{
				{
					VPC: &linodego.VPCInterfaceCreateOptions{},
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
							IPv6: []linodego.VPCIPv6Range{
								{
									Range: "2001:0db8::/56",
								},
							},
						},
					}
					return nil
				})
			},
			expectErr:             false,
			expectLinodeInterface: false,
			expectSubnetID:        456,
		},
		{
			name: "Error - Failed to fetch LinodeVPC",
			vpcRef: &corev1.ObjectReference{
				Name: "nonexistent-vpc",
			},
			linodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
			mockSetup: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Get(gomock.Any(), client.ObjectKey{
					Name:      "nonexistent-vpc",
					Namespace: "default",
				}, gomock.Any()).Return(fmt.Errorf("vpc not found"))
			},
			expectErr:             true,
			expectErrMsg:          "vpc not found",
			expectLinodeInterface: false,
		},
		{
			name: "Error - VPC is not ready",
			vpcRef: &corev1.ObjectReference{
				Name: "test-vpc",
			},
			linodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
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
			expectErr:             true,
			expectErrMsg:          "vpc is not available",
			expectLinodeInterface: false,
		},
		{
			name: "Error - VPC has no subnets",
			vpcRef: &corev1.ObjectReference{
				Name: "test-vpc",
			},
			linodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
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
			expectErr:             true,
			expectErrMsg:          "failed to find subnet",
			expectLinodeInterface: false,
		},
		{
			name: "Error - Subnet name not found",
			vpcRef: &corev1.ObjectReference{
				Name: "test-vpc",
			},
			subnetName:       "nonexistent-subnet",
			linodeInterfaces: []linodego.LinodeInterfaceCreateOptions{},
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
			expectErr:             true,
			expectErrMsg:          "failed to find subnet as subnet id set is 0",
			expectLinodeInterface: false,
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
					Spec: infrav1alpha2.LinodeMachineSpec{
						IPv6Options: &infrav1alpha2.IPv6CreateOptions{
							EnableSLAAC:  ptr.To(true),
							IsPublicIPv6: ptr.To(true),
						},
					},
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
			linodeIface, err := getVPCLinodeInterfaceConfig(ctx, machineScope, tc.linodeInterfaces, logger, tc.vpcRef)

			// Check expectations
			validateInterfaceExpectations(t, err, nil, linodeIface, tc.expectErr, tc.expectErrMsg, false, tc.expectLinodeInterface, tc.expectSubnetID)

			// Additional check for interface updates
			if !tc.expectErr && !tc.expectLinodeInterface && len(tc.linodeInterfaces) > 0 && tc.linodeInterfaces[0].VPC != nil {
				require.NotNil(t, tc.linodeInterfaces[0].VPC.SubnetID)
				require.Equal(t, tc.expectSubnetID, tc.linodeInterfaces[0].VPC.SubnetID)
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
			name:            "Success - No Tags",
			machine:         &infrav1alpha2.LinodeMachine{},
			expInstanceTags: []string{},
			expMachine: &infrav1alpha2.LinodeMachine{
				Spec:   infrav1alpha2.LinodeMachineSpec{},
				Status: infrav1alpha2.LinodeMachineStatus{},
			},
		},
		{
			name:            "Success - add capl-auto-gen tags",
			machine:         &infrav1alpha2.LinodeMachine{},
			clusterName:     "test-cluster",
			expInstanceTags: []string{"test-cluster"},
			expMachine: &infrav1alpha2.LinodeMachine{
				Spec:   infrav1alpha2.LinodeMachineSpec{},
				Status: infrav1alpha2.LinodeMachineStatus{},
			},
		},
		{
			name:             "Success - add tags with no previous tags",
			clusterName:      "test-cluster",
			currInstanceTags: []string{"instance-manually-added-tag"},
			machine: &infrav1alpha2.LinodeMachine{
				Spec: infrav1alpha2.LinodeMachineSpec{
					Tags: []string{"tag1", "tag2"},
				},
			},
			expMachine: &infrav1alpha2.LinodeMachine{
				Spec: infrav1alpha2.LinodeMachineSpec{
					Tags: []string{"tag1", "tag2"},
				},
				Status: infrav1alpha2.LinodeMachineStatus{
					Tags: []string{"tag1", "tag2"},
				},
			},
			expInstanceTags: []string{"test-cluster", "tag1", "tag2", "instance-manually-added-tag"},
		},
		{
			name:             "Success - add tags with previous tags",
			clusterName:      "test-cluster",
			currInstanceTags: []string{"instance-manually-added-tag", "tag1", "tag2"},
			machine: &infrav1alpha2.LinodeMachine{
				Spec: infrav1alpha2.LinodeMachineSpec{
					Tags: []string{"tag1", "tag2", "tag3", "tag4"},
				},
				Status: infrav1alpha2.LinodeMachineStatus{
					Tags: []string{"tag1", "tag2"},
				},
			},
			expMachine: &infrav1alpha2.LinodeMachine{
				Spec: infrav1alpha2.LinodeMachineSpec{
					Tags: []string{"tag1", "tag2", "tag3", "tag4"},
				},
				Status: infrav1alpha2.LinodeMachineStatus{
					Tags: []string{"tag1", "tag2", "tag3", "tag4"},
				},
			},
			expInstanceTags: []string{"test-cluster", "tag1", "tag2", "instance-manually-added-tag", "tag3", "tag4"},
		},
		{
			name:             "Success - remove tags",
			clusterName:      "test-cluster",
			currInstanceTags: []string{"instance-manually-added-tag", "tag1", "tag2", "tag3", "tag4"},
			machine: &infrav1alpha2.LinodeMachine{
				Spec: infrav1alpha2.LinodeMachineSpec{
					Tags: []string{"tag3", "tag4"},
				},
				Status: infrav1alpha2.LinodeMachineStatus{
					Tags: []string{"tag1", "tag2", "tag3", "tag4"},
				},
			},
			expMachine: &infrav1alpha2.LinodeMachine{
				Spec: infrav1alpha2.LinodeMachineSpec{
					Tags: []string{"tag3", "tag4"},
				},
				Status: infrav1alpha2.LinodeMachineStatus{
					Tags: []string{"tag3", "tag4"},
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
			out := getTags(&scope.MachineScope{
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
			require.Equal(t, tc.expMachine.Status.Tags, tc.machine.Spec.Tags)
		})
	}
}

func TestBuildInstanceAddrs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		machineScope  *scope.MachineScope
		instanceID    int
		expectedAddrs []v1beta2.MachineAddress
		expectedError error
		expects       func(client *mock.MockLinodeClient)
	}{
		{
			name: "Success - basic public IPv4 and IPv6 SLAAC",
			machineScope: &scope.MachineScope{
				LinodeMachine: &infrav1alpha2.LinodeMachine{},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{},
					},
				},
			},
			instanceID: 123,
			expectedAddrs: []v1beta2.MachineAddress{
				{Address: "172.0.0.2", Type: v1beta2.MachineExternalIP},
				{Address: "fd00::", Type: v1beta2.MachineExternalIP},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), 123).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{{Address: "172.0.0.2"}},
					},
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{Address: "fd00::"},
					},
				}, nil)
			},
		},
		{
			name: "Success - with private IPv4",
			machineScope: &scope.MachineScope{
				LinodeMachine: &infrav1alpha2.LinodeMachine{},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{},
					},
				},
			},
			instanceID: 123,
			expectedAddrs: []v1beta2.MachineAddress{
				{Address: "172.0.0.2", Type: v1beta2.MachineExternalIP},
				{Address: "fd00::", Type: v1beta2.MachineExternalIP},
				{Address: "192.168.0.2", Type: v1beta2.MachineInternalIP},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), 123).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Public:  []*linodego.InstanceIP{{Address: "172.0.0.2"}},
						Private: []*linodego.InstanceIP{{Address: "192.168.0.2"}},
					},
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{Address: "fd00::"},
					},
				}, nil)
			},
		},
		{
			name: "Success - with VPC IPv4",
			machineScope: &scope.MachineScope{
				LinodeMachine: &infrav1alpha2.LinodeMachine{},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{},
					},
				},
			},
			instanceID: 123,
			expectedAddrs: []v1beta2.MachineAddress{
				{Address: "172.0.0.2", Type: v1beta2.MachineExternalIP},
				{Address: "10.0.0.5", Type: v1beta2.MachineInternalIP},
				{Address: "fd00::", Type: v1beta2.MachineExternalIP},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				vpcAddress := "10.0.0.5"
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), 123).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{{Address: "172.0.0.2"}},
						VPC:    []*linodego.VPCIP{{Address: &vpcAddress}},
					},
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{Address: "fd00::"},
					},
				}, nil)
			},
		},
		{
			name: "Success - with VPC IPv6 public (skips SLAAC)",
			machineScope: &scope.MachineScope{
				LinodeMachine: &infrav1alpha2.LinodeMachine{},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{},
					},
				},
			},
			instanceID: 123,
			expectedAddrs: []v1beta2.MachineAddress{
				{Address: "172.0.0.2", Type: v1beta2.MachineExternalIP},
				{Address: "2001:db8::1", Type: v1beta2.MachineExternalIP},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				isPublic := true
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), 123).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{{Address: "172.0.0.2"}},
					},
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{Address: "fd00::"},
						VPC: []linodego.VPCIP{
							{
								IPv6IsPublic:  &isPublic,
								IPv6Addresses: []linodego.VPCIPIPv6Address{{SLAACAddress: "2001:db8::1"}},
							},
						},
					},
				}, nil)
			},
		},
		{
			name: "Success - with VPC IPv6 private (includes SLAAC)",
			machineScope: &scope.MachineScope{
				LinodeMachine: &infrav1alpha2.LinodeMachine{},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{},
					},
				},
			},
			instanceID: 123,
			expectedAddrs: []v1beta2.MachineAddress{
				{Address: "172.0.0.2", Type: v1beta2.MachineExternalIP},
				{Address: "fd00:1::1", Type: v1beta2.MachineInternalIP},
				{Address: "fd00::", Type: v1beta2.MachineExternalIP},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				isPublic := false
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), 123).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{{Address: "172.0.0.2"}},
					},
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{Address: "fd00::"},
						VPC: []linodego.VPCIP{
							{
								IPv6IsPublic:  &isPublic,
								IPv6Addresses: []linodego.VPCIPIPv6Address{{SLAACAddress: "fd00:1::1"}},
							},
						},
					},
				}, nil)
			},
		},
		{
			name: "Error - GetInstanceIPAddresses fails",
			machineScope: &scope.MachineScope{
				LinodeMachine: &infrav1alpha2.LinodeMachine{},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{},
					},
				},
			},
			instanceID:    123,
			expectedError: fmt.Errorf("get instance ips"),
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), 123).Return(nil, fmt.Errorf("API error"))
			},
		},
		{
			name: "Error - no public IPv4 addresses",
			machineScope: &scope.MachineScope{
				LinodeMachine: &infrav1alpha2.LinodeMachine{},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{},
					},
				},
			},
			instanceID:    123,
			expectedError: errNoPublicIPv4Addrs,
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), 123).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{},
					},
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{Address: "fd00::"},
					},
				}, nil)
			},
		},
		{
			name: "Error - no IPv6 address",
			machineScope: &scope.MachineScope{
				LinodeMachine: &infrav1alpha2.LinodeMachine{},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{},
					},
				},
			},
			instanceID:    123,
			expectedError: errNoPublicIPv6Addrs,
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), 123).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{{Address: "172.0.0.2"}},
					},
					IPv6: nil,
				}, nil)
			},
		},
		{
			name: "Error - no IPv6 SLAAC address",
			machineScope: &scope.MachineScope{
				LinodeMachine: &infrav1alpha2.LinodeMachine{},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{},
					},
				},
			},
			instanceID:    123,
			expectedError: errNoPublicIPv6SLAACAddrs,
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), 123).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{{Address: "172.0.0.2"}},
					},
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: nil,
					},
				}, nil)
			},
		},
		{
			name: "Success - with VLAN using LinodeInterfaces",
			machineScope: &scope.MachineScope{
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					Spec: infrav1alpha2.LinodeMachineSpec{
						LinodeInterfaces: []infrav1alpha2.LinodeInterfaceCreateOptions{
							{VLAN: &infrav1alpha2.VLANInterface{VLANLabel: "test-vlan"}},
						},
					},
				},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							UseVlan: true,
						},
					},
				},
			},
			instanceID: 123,
			expectedAddrs: []v1beta2.MachineAddress{
				{Address: "172.0.0.2", Type: v1beta2.MachineExternalIP},
				{Address: "fd00::", Type: v1beta2.MachineExternalIP},
				{Address: "10.0.0.1", Type: v1beta2.MachineInternalIP},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), 123).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{{Address: "172.0.0.2"}},
					},
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{Address: "fd00::"},
					},
				}, nil)
				vlanIPAM := "10.0.0.1/11"
				mockClient.EXPECT().ListInterfaces(gomock.Any(), 123, gomock.Any()).Return([]linodego.LinodeInterface{
					{VLAN: &linodego.VLANInterface{IPAMAddress: &vlanIPAM}},
				}, nil)
			},
		},
		{
			name: "Success - with VLAN using legacy Interfaces",
			machineScope: &scope.MachineScope{
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					Spec: infrav1alpha2.LinodeMachineSpec{},
				},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							UseVlan: true,
						},
					},
				},
			},
			instanceID: 123,
			expectedAddrs: []v1beta2.MachineAddress{
				{Address: "172.0.0.2", Type: v1beta2.MachineExternalIP},
				{Address: "fd00::", Type: v1beta2.MachineExternalIP},
				{Address: "10.0.0.2", Type: v1beta2.MachineInternalIP},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), 123).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{{Address: "172.0.0.2"}},
					},
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{Address: "fd00::"},
					},
				}, nil)
				mockClient.EXPECT().ListInstanceConfigs(gomock.Any(), 123, gomock.Any()).Return([]linodego.InstanceConfig{
					{
						Interfaces: []linodego.InstanceConfigInterface{
							{Purpose: linodego.InterfacePurposeVLAN, IPAMAddress: "10.0.0.2/11"},
						},
					},
				}, nil)
			},
		},
		{
			name: "Error - VLAN ListInterfaces fails",
			machineScope: &scope.MachineScope{
				LinodeMachine: &infrav1alpha2.LinodeMachine{
					Spec: infrav1alpha2.LinodeMachineSpec{
						LinodeInterfaces: []infrav1alpha2.LinodeInterfaceCreateOptions{
							{VLAN: &infrav1alpha2.VLANInterface{VLANLabel: "test-vlan"}},
						},
					},
				},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{
							UseVlan: true,
						},
					},
				},
			},
			instanceID:    123,
			expectedError: fmt.Errorf("handle vlan ips"),
			expects: func(mockClient *mock.MockLinodeClient) {
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), 123).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{{Address: "172.0.0.2"}},
					},
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{Address: "fd00::"},
					},
				}, nil)
				mockClient.EXPECT().ListInterfaces(gomock.Any(), 123, gomock.Any()).Return(nil, fmt.Errorf("list interfaces error"))
			},
		},
		{
			name: "Success - complete scenario with all IP types",
			machineScope: &scope.MachineScope{
				LinodeMachine: &infrav1alpha2.LinodeMachine{},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{},
					},
				},
			},
			instanceID: 123,
			expectedAddrs: []v1beta2.MachineAddress{
				{Address: "172.0.0.2", Type: v1beta2.MachineExternalIP},
				{Address: "10.0.0.5", Type: v1beta2.MachineInternalIP},
				{Address: "10.0.0.6", Type: v1beta2.MachineInternalIP},
				{Address: "fd00::", Type: v1beta2.MachineExternalIP},
				{Address: "192.168.0.2", Type: v1beta2.MachineInternalIP},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				vpcAddr1 := "10.0.0.5"
				vpcAddr2 := "10.0.0.6"
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), 123).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Public:  []*linodego.InstanceIP{{Address: "172.0.0.2"}},
						Private: []*linodego.InstanceIP{{Address: "192.168.0.2"}},
						VPC: []*linodego.VPCIP{
							{Address: &vpcAddr1},
							{Address: &vpcAddr2},
						},
					},
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{Address: "fd00::"},
					},
				}, nil)
			},
		},
		{
			name: "Success - VPC IPv4 with empty address is skipped",
			machineScope: &scope.MachineScope{
				LinodeMachine: &infrav1alpha2.LinodeMachine{},
				LinodeCluster: &infrav1alpha2.LinodeCluster{
					Spec: infrav1alpha2.LinodeClusterSpec{
						Network: infrav1alpha2.NetworkSpec{},
					},
				},
			},
			instanceID: 123,
			expectedAddrs: []v1beta2.MachineAddress{
				{Address: "172.0.0.2", Type: v1beta2.MachineExternalIP},
				{Address: "10.0.0.5", Type: v1beta2.MachineInternalIP},
				{Address: "fd00::", Type: v1beta2.MachineExternalIP},
			},
			expects: func(mockClient *mock.MockLinodeClient) {
				vpcAddr1 := "10.0.0.5"
				emptyAddr := ""
				mockClient.EXPECT().GetInstanceIPAddresses(gomock.Any(), 123).Return(&linodego.InstanceIPAddressResponse{
					IPv4: &linodego.InstanceIPv4Response{
						Public: []*linodego.InstanceIP{{Address: "172.0.0.2"}},
						VPC: []*linodego.VPCIP{
							{Address: &vpcAddr1},
							{Address: &emptyAddr},
							{Address: nil},
						},
					},
					IPv6: &linodego.InstanceIPv6Response{
						SLAAC: &linodego.InstanceIP{Address: "fd00::"},
					},
				}, nil)
			},
		},
	}

	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockLinodeClient(ctrl)
			testcase.machineScope.LinodeClient = mockClient
			testcase.expects(mockClient)

			addrs, err := buildInstanceAddrs(t.Context(), testcase.machineScope, testcase.instanceID)
			if testcase.expectedError != nil {
				assert.ErrorContains(t, err, testcase.expectedError.Error())
			} else {
				require.NoError(t, err, "expected no error but got one")
				assert.Equal(t, testcase.expectedAddrs, addrs)
			}
		})
	}
}
