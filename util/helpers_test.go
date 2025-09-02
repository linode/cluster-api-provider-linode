package util

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/linode/linodego"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/mock"
)

func TestIgnoreLinodeAPIError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		err          error
		code         []int
		shouldFilter bool
	}{{
		name:         "Not Linode API error",
		err:          errors.New("foo"),
		code:         []int{0},
		shouldFilter: false,
	}, {
		name: "Ignore not found Linode API error",
		err: linodego.Error{
			Response: nil,
			Code:     400,
			Message:  "not found",
		},
		code:         []int{400},
		shouldFilter: true,
	}, {
		name: "Don't ignore not found Linode API error",
		err: linodego.Error{
			Response: nil,
			Code:     400,
			Message:  "not found",
		},
		code:         []int{500},
		shouldFilter: false,
	}, {
		name: "Don't ignore with 2+ API errors",
		err: linodego.Error{
			Response: nil,
			Code:     400,
			Message:  "not found",
		},
		code:         []int{500, 418},
		shouldFilter: false,
	}, {
		name: "Ignore with 2+ API errors",
		err: linodego.Error{
			Response: nil,
			Code:     418,
			Message:  "not found",
		},
		code:         []int{500, 418},
		shouldFilter: true,
	}}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			err := IgnoreLinodeAPIError(testcase.err, testcase.code...)
			if testcase.shouldFilter && err != nil {
				t.Error("expected err but got nil")
			}
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{{
		name: "unexpected EOF",
		err:  io.ErrUnexpectedEOF,
		want: true,
	}, {
		name: "not found Linode API error",
		err: &linodego.Error{
			Response: nil,
			Code:     http.StatusNotFound,
			Message:  "not found",
		},
		want: false,
	}, {
		name: "Rate limiting Linode API error",
		err: &linodego.Error{
			Response: nil,
			Code:     http.StatusTooManyRequests,
			Message:  "rate limited",
		},
		want: true,
	}}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			if testcase.want != IsRetryableError(testcase.err) {
				t.Errorf("wanted %v, got %v", testcase.want, IsRetryableError(testcase.err))
			}
		})
	}
}

func TestGetInstanceID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		providerID *string
		wantErr    bool
		wantID     int
	}{{
		name:       "nil",
		providerID: nil,
		wantErr:    true,
		wantID:     -1,
	}, {
		name:       "invalid provider ID",
		providerID: Pointer("linode://foobar"),
		wantErr:    true,
		wantID:     -1,
	}, {
		name:       "valid",
		providerID: Pointer("linode://12345"),
		wantErr:    false,
		wantID:     12345,
	}}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			gotID, err := GetInstanceID(testcase.providerID)
			if testcase.wantErr && err == nil {
				t.Errorf("wanted %v, got %v", testcase.wantErr, err)
			}
			if gotID != testcase.wantID {
				t.Errorf("wanted %v, got %v", testcase.wantID, gotID)
			}
		})
	}
}

func TestIsLinodePrivateIP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		// Valid IPs in the Linode private range (192.168.128.0/17)
		{
			name:     "valid IP at start of range",
			ip:       "192.168.128.0",
			expected: true,
		},
		{
			name:     "valid IP in middle of range",
			ip:       "192.168.200.123",
			expected: true,
		},
		{
			name:     "valid IP at end of range",
			ip:       "192.168.255.255",
			expected: true,
		},
		{
			name:     "valid IP at boundary of range",
			ip:       "192.168.255.254",
			expected: true,
		},

		// Valid IPs outside the Linode private range
		{
			name:     "valid IP below range",
			ip:       "192.168.127.255",
			expected: false,
		},
		{
			name:     "valid IP above range",
			ip:       "192.169.0.0",
			expected: false,
		},
		{
			name:     "private IP from different range (10.0.0.0/8)",
			ip:       "10.0.0.1",
			expected: false,
		},
		{
			name:     "public IP",
			ip:       "203.0.113.1",
			expected: false,
		},
		{
			name:     "localhost IP",
			ip:       "127.0.0.1",
			expected: false,
		},

		// Invalid IP formats
		{
			name:     "empty string",
			ip:       "",
			expected: false,
		},
		{
			name:     "invalid format",
			ip:       "not-an-ip",
			expected: false,
		},
		{
			name:     "incomplete IP",
			ip:       "192.168",
			expected: false,
		},
		{
			name:     "IPv6 address",
			ip:       "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			expected: false,
		},
		{
			name:     "IP with invalid segments",
			ip:       "192.168.256.1",
			expected: false,
		},
		{
			name:     "IP with extra segments",
			ip:       "192.168.1.1.5",
			expected: false,
		},
	}

	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			result := IsLinodePrivateIP(testcase.ip)
			if result != testcase.expected {
				t.Errorf("IsLinodePrivateIP(%q) = %v, want %v", testcase.ip, result, testcase.expected)
			}
		})
	}
}

// TestSetOwnerReferenceToLinodeCluster has a high cognitive complexity
// due to the comprehensive nature of its test cases, covering various scenarios
// for setting owner references. This level of detail is acceptable and beneficial
// for a test function ensuring robust functionality.
//
//nolint:gocognit,cyclop // This is a valid test function with high cognitive and cyclomatic complexity
func TestSetOwnerReferenceToLinodeCluster(t *testing.T) {
	t.Parallel()

	baseTestScheme := runtime.NewScheme()
	if err := clusterv1.AddToScheme(baseTestScheme); err != nil {
		t.Fatalf("Failed to add clusterv1 to scheme: %v", err)
	}
	if err := infrav1alpha2.AddToScheme(baseTestScheme); err != nil {
		t.Fatalf("Failed to add infrav1alpha2 to scheme: %v", err)
	}
	if err := corev1.AddToScheme(baseTestScheme); err != nil {
		t.Fatalf("Failed to add corev1 to scheme: %v", err)
	}

	linodeClusterGVK := infrav1alpha2.GroupVersion.WithKind("LinodeCluster")

	validLinodeCluster := &infrav1alpha2.LinodeCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-linodecluster",
			Namespace: "test-namespace",
			UID:       types.UID("uid-linodecluster"),
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       linodeClusterGVK.Kind,
			APIVersion: linodeClusterGVK.GroupVersion().String(),
		},
	}

	validCluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-namespace",
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				Name: validLinodeCluster.Name,
				Kind: linodeClusterGVK.Kind,
			},
		},
	}

	baseObjToOwn := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "test-namespace",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
	}

	tests := []struct {
		name          string
		cluster       *clusterv1.Cluster
		obj           client.Object
		mockK8sClient func(m *mock.MockK8sClient)
		scheme        *runtime.Scheme
		wantErr       bool
		expectedError string
		validateObj   func(t *testing.T, obj client.Object)
	}{
		{
			name:    "success",
			cluster: validCluster,
			obj:     baseObjToOwn.DeepCopy(),
			mockK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: validLinodeCluster.Name, Namespace: validLinodeCluster.Namespace}, gomock.AssignableToTypeOf(&infrav1alpha2.LinodeCluster{}), gomock.Any()).DoAndReturn(
					func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						lc, ok := obj.(*infrav1alpha2.LinodeCluster)
						if !ok {
							return errors.New("object is not of type *infrav1alpha2.LinodeCluster")
						}
						*lc = *validLinodeCluster
						return nil
					}).Times(1)
				mockK8sClient.EXPECT().Update(gomock.Any(), gomock.AssignableToTypeOf(baseObjToOwn.DeepCopy()), gomock.Any()).Return(nil).Times(1)
			},
			scheme:  baseTestScheme,
			wantErr: false,
			validateObj: func(t *testing.T, obj client.Object) {
				t.Helper()
				if len(obj.GetOwnerReferences()) != 1 {
					t.Fatalf("Expected 1 owner reference, got %d", len(obj.GetOwnerReferences()))
				}
				ownerRef := obj.GetOwnerReferences()[0]
				if ownerRef.APIVersion != validLinodeCluster.APIVersion ||
					ownerRef.Kind != validLinodeCluster.Kind ||
					ownerRef.Name != validLinodeCluster.Name ||
					ownerRef.UID != validLinodeCluster.UID {
					t.Errorf("OwnerReference mismatch: got GVK=%s/%s Name=%s UID=%s, want GVK=%s/%s Name=%s UID=%s",
						ownerRef.APIVersion, ownerRef.Kind, ownerRef.Name, ownerRef.UID,
						validLinodeCluster.APIVersion, validLinodeCluster.Kind, validLinodeCluster.Name, validLinodeCluster.UID)
				}
				if ownerRef.Controller == nil || !*ownerRef.Controller {
					t.Errorf("Expected owner reference to be a controller, but it was not")
				}
				if ownerRef.BlockOwnerDeletion == nil || !*ownerRef.BlockOwnerDeletion {
					t.Errorf("Expected owner reference to block owner deletion, but it did not")
				}
			},
		},
		{
			name:          "cluster is nil - can happen when deleting",
			cluster:       nil,
			obj:           baseObjToOwn.DeepCopy(),
			scheme:        baseTestScheme,
			wantErr:       false,
			expectedError: "",
		},
		{
			name: "cluster infrastructureRef is empty - can happen when deleting",
			cluster: func() *clusterv1.Cluster {
				c := validCluster.DeepCopy()
				c.Spec.InfrastructureRef = clusterv1.ContractVersionedObjectReference{}
				return c
			}(),
			obj:           baseObjToOwn.DeepCopy(),
			scheme:        baseTestScheme,
			wantErr:       false,
			expectedError: "",
		},
		{
			name:    "k8s client Get returns NotFound error",
			cluster: validCluster,
			obj:     baseObjToOwn.DeepCopy(),
			mockK8sClient: func(m *mock.MockK8sClient) {
				m.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: validLinodeCluster.Name, Namespace: validLinodeCluster.Namespace}, gomock.AssignableToTypeOf(&infrav1alpha2.LinodeCluster{}), gomock.Any()).Return(
					apierrors.NewNotFound(infrav1alpha2.GroupVersion.WithResource("linodeclusters").GroupResource(), "test-linodecluster"),
				).Times(1)
			},
			scheme:        baseTestScheme,
			wantErr:       false,
			expectedError: "",
			validateObj: func(t *testing.T, obj client.Object) {
				t.Helper()
				if len(obj.GetOwnerReferences()) != 0 {
					t.Fatalf("Expected 0 owner references when LinodeCluster is not found, got %d", len(obj.GetOwnerReferences()))
				}
			},
		},
		{
			name:    "k8s client Get returns non-NotFound error",
			cluster: validCluster,
			obj:     baseObjToOwn.DeepCopy(),
			mockK8sClient: func(m *mock.MockK8sClient) {
				m.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: validLinodeCluster.Name, Namespace: validLinodeCluster.Namespace}, gomock.AssignableToTypeOf(&infrav1alpha2.LinodeCluster{}), gomock.Any()).Return(
					errors.New("internal server error"),
				).Times(1)
			},
			scheme:        baseTestScheme,
			wantErr:       true,
			expectedError: "internal server error",
		},
		{
			name:    "k8s client Update returns error",
			cluster: validCluster,
			obj:     baseObjToOwn.DeepCopy(),
			mockK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: validLinodeCluster.Name, Namespace: validLinodeCluster.Namespace}, gomock.AssignableToTypeOf(&infrav1alpha2.LinodeCluster{}), gomock.Any()).DoAndReturn(
					func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						lc, ok := obj.(*infrav1alpha2.LinodeCluster)
						if !ok {
							return errors.New("object is not of type *infrav1alpha2.LinodeCluster")
						}
						*lc = *validLinodeCluster
						return nil
					}).Times(1)
				mockK8sClient.EXPECT().Update(gomock.Any(), gomock.AssignableToTypeOf(baseObjToOwn.DeepCopy()), gomock.Any()).Return(errors.New("update failed")).Times(1)
			},
			scheme:        baseTestScheme,
			wantErr:       true,
			expectedError: "update failed",
		},
		{
			name:    "SetControllerReference fails because owner type not in scheme",
			cluster: validCluster,
			obj:     baseObjToOwn.DeepCopy(),
			mockK8sClient: func(mockK8sClient *mock.MockK8sClient) {
				mockK8sClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Name: validLinodeCluster.Name, Namespace: validLinodeCluster.Namespace}, gomock.AssignableToTypeOf(&infrav1alpha2.LinodeCluster{}), gomock.Any()).DoAndReturn(
					func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						lc, ok := obj.(*infrav1alpha2.LinodeCluster)
						if !ok {
							return errors.New("object is not of type *infrav1alpha2.LinodeCluster")
						}
						*lc = *validLinodeCluster
						return nil
					}).Times(1)
			},
			scheme: func() *runtime.Scheme {
				s := runtime.NewScheme()
				corev1.AddToScheme(s)
				return s
			}(),
			wantErr:       true,
			expectedError: "no kind is registered for the type v1alpha2.LinodeCluster",
		},
	}

	for _, tt := range tests {
		tc := tt // Capture range variable for parallel sub-tests
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockK8sClient := mock.NewMockK8sClient(mockCtrl)

			tc.obj.SetOwnerReferences(nil)

			if tc.mockK8sClient != nil {
				tc.mockK8sClient(mockK8sClient)
			}

			err := SetOwnerReferenceToLinodeCluster(t.Context(), mockK8sClient, tc.cluster, tc.obj, tc.scheme)

			if (err != nil) != tc.wantErr {
				t.Errorf("SetOwnerReferenceToLinodeCluster() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if tc.wantErr && err != nil {
				if !strings.Contains(err.Error(), tc.expectedError) {
					t.Errorf("SetOwnerReferenceToLinodeCluster() error = %q, want error containing %q", err.Error(), tc.expectedError)
				}
			}

			if !tc.wantErr && tc.validateObj != nil {
				tc.validateObj(t, tc.obj)
			}
		})
	}
}
