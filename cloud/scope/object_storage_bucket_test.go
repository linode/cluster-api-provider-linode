package scope

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/mock"
)

func Test_validateObjectStorageBucketScopeParams(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		params ObjectStorageBucketScopeParams
		expectedErr error
	}{
		// TODO: Add test cases.
		{
			name: "Success - Valid ObjectStorageBucketScopeParams",
			params: ObjectStorageBucketScopeParams{
				Bucket: &infrav1alpha1.LinodeObjectStorageBucket{},
				Logger: &logr.Logger{},
			},
			expectedErr: nil,
		},
		{
			name: "Failure - Invalid ObjectStorageBucketScopeParams. Logger is nil",
			params: ObjectStorageBucketScopeParams{
				Bucket: &infrav1alpha1.LinodeObjectStorageBucket{},
				Logger: nil,
			},
			expectedErr: fmt.Errorf("logger is required when creating an ObjectStorageBucketScope"),
		},

		{
			name: "Failure - Invalid ObjectStorageBucketScopeParams. Bucket is nil",
			params: ObjectStorageBucketScopeParams{
				Bucket: nil,
				Logger: &logr.Logger{},
			},
			expectedErr: fmt.Errorf("object storage bucket is required when creating an ObjectStorageBucketScope"),
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			err := validateObjectStorageBucketScopeParams(testcase.params)
			if err != nil {
				assert.EqualError(t, err, testcase.expectedErr.Error())
			}
		})
	}
}

func TestNewObjectStorageBucketScope(t *testing.T) {
	t.Parallel()
	type args struct {
		apiKey string
		params ObjectStorageBucketScopeParams
	}
	tests := []struct {
		name    string
		args    args
		want    *ObjectStorageBucketScope
		wantErr bool
		expectedErr error
		patchFunc   func(obj client.Object, crClient client.Client) (*patch.Helper, error)
		getFunc     func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error
	}{
		{
			name: "Success - Pass in valid args and get a valid MachineScope",
			args: args{
				apiKey: "test-key",
				params: ObjectStorageBucketScopeParams{
					Bucket: &infrav1alpha1.LinodeObjectStorageBucket{},
					Logger: &logr.Logger{},
				},
			},
			want:        &ObjectStorageBucketScope{},
			expectedErr: nil,
			patchFunc: func(obj client.Object, crClient client.Client) (*patch.Helper, error) {
				return &patch.Helper{}, nil
			},
			getFunc: func(ctx context.Context, key types.NamespacedName, obj *corev1.Secret, opts ...client.GetOption) error {
				return nil
			},
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockK8sClient := mock.NewMockk8sClient(ctrl)

			if testcase.args.params.Bucket != nil && testcase.args.params.Bucket.Spec.CredentialsRef != nil {
				mockK8sClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(testcase.getFunc).Times(1)
			}

			testcase.args.params.Client = mockK8sClient

			got, err := NewObjectStorageBucketScope(context.Background(), testcase.args.apiKey, testcase.args.params)

			if testcase.expectedErr != nil {
				assert.EqualError(t, err, testcase.expectedErr.Error())
			} else {
				assert.NotEmpty(t, got)
			}
		})
	}
}

func TestObjectStorageBucketScopeMethods(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		Bucket  *infrav1alpha1.LinodeObjectStorageBucket
		wantErr bool
	}{
		{
			name: "Success - finalizer should be added to the Linode Machine object",
			Bucket: &infrav1alpha1.LinodeObjectStorageBucket{},
			wantErr: false,
		},
		{
			name: "Failure - finalizer should not be added to the Bucket object. Function returns nil since it was already present",
			Bucket: &infrav1alpha1.LinodeObjectStorageBucket{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{infrav1alpha1.GroupVersion.String()},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockK8sClient := mock.NewMockk8sClient(ctrl)
			mockPatchHelper := mock.NewMockPatchHelper(ctrl)

			objScope := &ObjectStorageBucketScope{
				client:            mockK8sClient,
				Bucket:            testcase.Bucket,
				Logger:            logr.Logger{},
				LinodeClient:      &linodego.Client{},
				BucketPatchHelper: mockPatchHelper,
			}

			if testcase.Bucket.Finalizers == nil {
				mockPatchHelper.EXPECT().Patch(gomock.Any(), gomock.Any()).Return(nil)
			}

			
			if err := objScope.AddFinalizer(context.Background()); (err != nil) != testcase.wantErr {
				t.Errorf("ObjectStorageBucketScope.AddFinalizer() error = %v, wantErr %v", err, testcase.wantErr)
			}

			if objScope.Bucket.Finalizers[0] != infrav1alpha1.GroupVersion.String() {
				t.Errorf("Not able to add finalizer: %s", infrav1alpha1.GroupVersion.String())
			}

		})
	}
}

func TestObjectStorageBucketScope_ApplyAccessKeySecret(t *testing.T) {
	t.Parallel()
	type fields struct {
		client            client.Client
		Bucket            *infrav1alpha1.LinodeObjectStorageBucket
		Logger            logr.Logger
		LinodeClient      *linodego.Client
		BucketPatchHelper *patch.Helper
	}
	type args struct {
		ctx        context.Context
		keys       [NumAccessKeys]linodego.ObjectStorageKey
		secretName string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &ObjectStorageBucketScope{
				client:            tt.fields.client,
				Bucket:            tt.fields.Bucket,
				Logger:            tt.fields.Logger,
				LinodeClient:      tt.fields.LinodeClient,
				BucketPatchHelper: tt.fields.BucketPatchHelper,
			}
			if err := s.ApplyAccessKeySecret(tt.args.ctx, tt.args.keys, tt.args.secretName); (err != nil) != tt.wantErr {
				t.Errorf("ObjectStorageBucketScope.ApplyAccessKeySecret() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestObjectStorageBucketScope_GetAccessKeySecret(t *testing.T) {
	t.Parallel()
	type fields struct {
		client            client.Client
		Bucket            *infrav1alpha1.LinodeObjectStorageBucket
		Logger            logr.Logger
		LinodeClient      *linodego.Client
		BucketPatchHelper *patch.Helper
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *corev1.Secret
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &ObjectStorageBucketScope{
				client:            tt.fields.client,
				Bucket:            tt.fields.Bucket,
				Logger:            tt.fields.Logger,
				LinodeClient:      tt.fields.LinodeClient,
				BucketPatchHelper: tt.fields.BucketPatchHelper,
			}
			got, err := s.GetAccessKeySecret(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("ObjectStorageBucketScope.GetAccessKeySecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ObjectStorageBucketScope.GetAccessKeySecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestObjectStorageBucketScope_GetAccessKeysFromSecret(t *testing.T) {
	t.Parallel()
	type fields struct {
		client            client.Client
		Bucket            *infrav1alpha1.LinodeObjectStorageBucket
		Logger            logr.Logger
		LinodeClient      *linodego.Client
		BucketPatchHelper *patch.Helper
	}
	type args struct {
		ctx    context.Context
		secret *corev1.Secret
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    [NumAccessKeys]int
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &ObjectStorageBucketScope{
				client:            tt.fields.client,
				Bucket:            tt.fields.Bucket,
				Logger:            tt.fields.Logger,
				LinodeClient:      tt.fields.LinodeClient,
				BucketPatchHelper: tt.fields.BucketPatchHelper,
			}
			got, err := s.GetAccessKeysFromSecret(tt.args.ctx, tt.args.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("ObjectStorageBucketScope.GetAccessKeysFromSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ObjectStorageBucketScope.GetAccessKeysFromSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestObjectStorageBucketScope_ShouldRotateKeys(t *testing.T) {
	t.Parallel()
	type fields struct {
		client            client.Client
		Bucket            *infrav1alpha1.LinodeObjectStorageBucket
		Logger            logr.Logger
		LinodeClient      *linodego.Client
		BucketPatchHelper *patch.Helper
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &ObjectStorageBucketScope{
				client:            tt.fields.client,
				Bucket:            tt.fields.Bucket,
				Logger:            tt.fields.Logger,
				LinodeClient:      tt.fields.LinodeClient,
				BucketPatchHelper: tt.fields.BucketPatchHelper,
			}
			if got := s.ShouldRotateKeys(); got != tt.want {
				t.Errorf("ObjectStorageBucketScope.ShouldRotateKeys() = %v, want %v", got, tt.want)
			}
		})
	}
}
