package common

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/clients"
)

type controllerTestScope struct {
	calls                            []string
	addFinalizerErr                  error
	addCredentialsRefFinalizerErr    error
	removeCredentialsRefFinalizerErr error
	closeErr                         error
	setCredentialRefTokenErr         error
}

func (s *controllerTestScope) AddFinalizer(context.Context) error {
	s.calls = append(s.calls, "add-finalizer")
	return s.addFinalizerErr
}

func (s *controllerTestScope) RemoveFinalizer() {
	s.calls = append(s.calls, "remove-finalizer")
}

func (s *controllerTestScope) AddCredentialsRefFinalizer(context.Context) error {
	s.calls = append(s.calls, "add-credentials-finalizer")
	return s.addCredentialsRefFinalizerErr
}

func (s *controllerTestScope) RemoveCredentialsRefFinalizer(context.Context) error {
	s.calls = append(s.calls, "remove-credentials-finalizer")
	return s.removeCredentialsRefFinalizerErr
}

func (s *controllerTestScope) Close(context.Context) error {
	s.calls = append(s.calls, "close")
	return s.closeErr
}

func (s *controllerTestScope) SetCredentialRefTokenForLinodeClients(context.Context) error {
	s.calls = append(s.calls, "set-credential-token")
	return s.setCredentialRefTokenErr
}

type controllerTestReconciler struct {
	object           *infrav1alpha2.LinodeVPC
	scope            *controllerTestScope
	name             string
	requeueAfter     time.Duration
	timeout          time.Duration
	newScopeErr      error
	applyResult      ctrl.Result
	applyErr         error
	deleteResult     ctrl.Result
	deleteErr        error
	newScopeCluster  *clusterv1.Cluster
	failureErrors    []error
	addWatchesCalled bool
}

func (r *controllerTestReconciler) Name() string                     { return r.name }
func (r *controllerTestReconciler) Object() *infrav1alpha2.LinodeVPC { return r.object }
func (r *controllerTestReconciler) RequeueAfter() time.Duration      { return r.requeueAfter }
func (r *controllerTestReconciler) Timeout() time.Duration           { return r.timeout }
func (r *controllerTestReconciler) AddWatches(*builder.Builder, clients.K8sClient) {
	r.addWatchesCalled = true
}
func (r *controllerTestReconciler) NewScope(_ context.Context, _ clients.K8sClient, _ *infrav1alpha2.LinodeVPC, cluster *clusterv1.Cluster) (*controllerTestScope, error) {
	r.newScopeCluster = cluster
	return r.scope, r.newScopeErr
}
func (r *controllerTestReconciler) Apply(context.Context, *controllerTestScope, logr.Logger) (ctrl.Result, error) {
	r.scope.calls = append(r.scope.calls, "apply")
	return r.applyResult, r.applyErr
}
func (r *controllerTestReconciler) Delete(context.Context, *controllerTestScope, logr.Logger) (ctrl.Result, error) {
	r.scope.calls = append(r.scope.calls, "delete")
	return r.deleteResult, r.deleteErr
}
func (r *controllerTestReconciler) HandleFailureCondition(_ *controllerTestScope, err error) {
	r.failureErrors = append(r.failureErrors, err)
}

func newControllerTest(t *testing.T, object *infrav1alpha2.LinodeVPC, scope *controllerTestScope) (*controllerTestReconciler, *Controller[*controllerTestScope, *infrav1alpha2.LinodeVPC]) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, clusterv1.AddToScheme(scheme))
	require.NoError(t, infrav1alpha2.AddToScheme(scheme))
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(object).WithObjects(object).Build()

	rec := &controllerTestReconciler{
		object:       object,
		scope:        scope,
		name:         "test-controller",
		requeueAfter: time.Second,
		timeout:      time.Minute,
	}
	controller := NewControllerFor[*controllerTestScope, *infrav1alpha2.LinodeVPC](rec).
		WithClientForTest(k8sClient).
		WithLoggerForTest(logr.Discard())
	return rec, controller
}

func newControllerTestObject() *infrav1alpha2.LinodeVPC {
	return &infrav1alpha2.LinodeVPC{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}}
}

func newDeletingControllerTestObject(deletionTime time.Time) *infrav1alpha2.LinodeVPC {
	object := newControllerTestObject()
	object.DeletionTimestamp = &metav1.Time{Time: deletionTime}
	object.Finalizers = []string{"test-finalizer"}
	return object
}

func TestControllerReconcileApply(t *testing.T) {
	t.Parallel()

	t.Run("success marks ready and closes scope", func(t *testing.T) {
		t.Parallel()
		scope := &controllerTestScope{}
		rec, controller := newControllerTest(t, newControllerTestObject(), scope)

		result, err := controller.Reconcile(context.Background(), rec.object)

		require.NoError(t, err)
		require.Equal(t, ctrl.Result{}, result)
		require.Equal(t, metav1.ConditionTrue, rec.object.GetCondition(string(clusterv1.ReadyCondition)).Status)
		require.Equal(t, "Reconciled", rec.object.GetCondition(string(clusterv1.ReadyCondition)).Reason)
		require.Equal(t, []string{"set-credential-token", "add-credentials-finalizer", "add-finalizer", "apply", "close"}, scope.calls)
		require.Empty(t, rec.failureErrors)
	})

	t.Run("apply requeue leaves ready unknown", func(t *testing.T) {
		t.Parallel()
		scope := &controllerTestScope{}
		rec, controller := newControllerTest(t, newControllerTestObject(), scope)
		rec.applyResult = ctrl.Result{RequeueAfter: 2 * time.Second}

		result, err := controller.Reconcile(context.Background(), rec.object)

		require.NoError(t, err)
		require.Equal(t, 2*time.Second, result.RequeueAfter)
		require.Equal(t, metav1.ConditionUnknown, rec.object.GetCondition(string(clusterv1.ReadyCondition)).Status)
		require.Empty(t, rec.failureErrors)
	})

	t.Run("transient apply error requeues", func(t *testing.T) {
		t.Parallel()
		scope := &controllerTestScope{}
		rec, controller := newControllerTest(t, newControllerTestObject(), scope)
		rec.applyErr = errors.New("temporary failure")

		result, err := controller.Reconcile(context.Background(), rec.object)

		require.NoError(t, err)
		require.GreaterOrEqual(t, result.RequeueAfter, rec.requeueAfter)
		require.Empty(t, rec.failureErrors)
	})

	t.Run("stale apply failure is returned and reported", func(t *testing.T) {
		t.Parallel()
		scope := &controllerTestScope{}
		rec, controller := newControllerTest(t, newControllerTestObject(), scope)
		rec.timeout = -time.Second
		rec.applyErr = errors.New("permanent failure")

		result, err := controller.Reconcile(context.Background(), rec.object)

		require.ErrorIs(t, err, rec.applyErr)
		require.Equal(t, ctrl.Result{}, result)
		require.Equal(t, []error{rec.applyErr}, rec.failureErrors)
	})

	t.Run("finalizer failures stop apply and are reported", func(t *testing.T) {
		for _, tc := range []struct {
			name      string
			configure func(*controllerTestScope, error)
			wantCalls []string
		}{
			{
				name: "credentials finalizer",
				configure: func(scope *controllerTestScope, err error) {
					scope.addCredentialsRefFinalizerErr = err
				},
				wantCalls: []string{"set-credential-token", "add-credentials-finalizer", "close"},
			},
			{
				name: "resource finalizer",
				configure: func(scope *controllerTestScope, err error) {
					scope.addFinalizerErr = err
				},
				wantCalls: []string{"set-credential-token", "add-credentials-finalizer", "add-finalizer", "close"},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				scope := &controllerTestScope{}
				failure := errors.New("finalizer failure")
				tc.configure(scope, failure)
				rec, controller := newControllerTest(t, newControllerTestObject(), scope)

				_, err := controller.Reconcile(context.Background(), rec.object)

				require.ErrorIs(t, err, failure)
				require.Equal(t, tc.wantCalls, scope.calls)
				require.Equal(t, []error{failure}, rec.failureErrors)
			})
		}
	})
}

func TestControllerReconcileEarlyFailuresAndPause(t *testing.T) {
	t.Parallel()

	t.Run("scope creation failure skips close", func(t *testing.T) {
		t.Parallel()
		scope := &controllerTestScope{}
		rec, controller := newControllerTest(t, newControllerTestObject(), scope)
		rec.newScopeErr = errors.New("scope failure")

		_, err := controller.Reconcile(context.Background(), rec.object)

		require.ErrorIs(t, err, rec.newScopeErr)
		require.Empty(t, scope.calls)
		require.Empty(t, rec.failureErrors)
	})

	t.Run("credential token failure skips deferred close", func(t *testing.T) {
		t.Parallel()
		scope := &controllerTestScope{setCredentialRefTokenErr: errors.New("credential failure")}
		rec, controller := newControllerTest(t, newControllerTestObject(), scope)

		_, err := controller.Reconcile(context.Background(), rec.object)

		require.ErrorIs(t, err, scope.setCredentialRefTokenErr)
		require.Equal(t, []string{"set-credential-token"}, scope.calls)
		require.Empty(t, rec.failureErrors)
	})

	t.Run("paused object skips scope creation", func(t *testing.T) {
		t.Parallel()
		object := newControllerTestObject()
		object.Annotations = map[string]string{clusterv1.PausedAnnotation: ""}
		scope := &controllerTestScope{}
		rec, controller := newControllerTest(t, object, scope)

		result, err := controller.Reconcile(context.Background(), object)

		require.NoError(t, err)
		require.Equal(t, ctrl.Result{}, result)
		require.Empty(t, scope.calls)
		require.Nil(t, rec.newScopeCluster)
	})

	t.Run("cluster metadata passes cluster to scope", func(t *testing.T) {
		t.Parallel()
		object := newControllerTestObject()
		object.Labels = map[string]string{clusterv1.ClusterNameLabel: "test-cluster"}
		scope := &controllerTestScope{}
		rec, controller := newControllerTest(t, object, scope)
		cluster := &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: object.Namespace}}
		scheme := runtime.NewScheme()
		require.NoError(t, clusterv1.AddToScheme(scheme))
		require.NoError(t, infrav1alpha2.AddToScheme(scheme))
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(object).WithObjects(cluster, object).Build()
		controller.WithClientForTest(k8sClient)

		_, err := controller.Reconcile(context.Background(), object)

		require.NoError(t, err)
		require.Equal(t, cluster, rec.newScopeCluster)
	})
}

func TestControllerReconcileDelete(t *testing.T) {
	t.Parallel()

	t.Run("success removes credentials and resource finalizers", func(t *testing.T) {
		t.Parallel()
		object := newDeletingControllerTestObject(time.Now())
		scope := &controllerTestScope{}
		_, controller := newControllerTest(t, object, scope)

		result, err := controller.Reconcile(context.Background(), object)

		require.NoError(t, err)
		require.Equal(t, ctrl.Result{}, result)
		require.Equal(t, []string{"set-credential-token", "delete", "remove-credentials-finalizer", "remove-finalizer", "close"}, scope.calls)
	})

	t.Run("delete requeue preserves finalizers", func(t *testing.T) {
		t.Parallel()
		object := newDeletingControllerTestObject(time.Now())
		scope := &controllerTestScope{}
		rec, controller := newControllerTest(t, object, scope)
		rec.deleteResult = ctrl.Result{RequeueAfter: 3 * time.Second}

		result, err := controller.Reconcile(context.Background(), object)

		require.NoError(t, err)
		require.Equal(t, 3*time.Second, result.RequeueAfter)
		require.Equal(t, []string{"set-credential-token", "delete", "close"}, scope.calls)
	})

	t.Run("delete failure is reported", func(t *testing.T) {
		t.Parallel()
		object := newDeletingControllerTestObject(time.Now())
		scope := &controllerTestScope{}
		rec, controller := newControllerTest(t, object, scope)
		rec.deleteErr = errors.New("delete failure")

		_, err := controller.Reconcile(context.Background(), object)

		require.ErrorIs(t, err, rec.deleteErr)
		require.Equal(t, []error{rec.deleteErr}, rec.failureErrors)
		require.Equal(t, []string{"set-credential-token", "delete", "close"}, scope.calls)
	})

	t.Run("credentials finalizer failure requeues before timeout", func(t *testing.T) {
		t.Parallel()
		object := newDeletingControllerTestObject(time.Now())
		scope := &controllerTestScope{removeCredentialsRefFinalizerErr: errors.New("remove credentials finalizer")}
		rec, controller := newControllerTest(t, object, scope)

		result, err := controller.Reconcile(context.Background(), object)

		require.NoError(t, err)
		require.GreaterOrEqual(t, result.RequeueAfter, rec.requeueAfter)
		require.Equal(t, []string{"set-credential-token", "delete", "remove-credentials-finalizer", "close"}, scope.calls)
		require.Empty(t, rec.failureErrors)
	})

	t.Run("credentials finalizer failure after timeout is returned", func(t *testing.T) {
		t.Parallel()
		object := newDeletingControllerTestObject(time.Now().Add(-time.Hour))
		scope := &controllerTestScope{removeCredentialsRefFinalizerErr: errors.New("remove credentials finalizer")}
		rec, controller := newControllerTest(t, object, scope)

		_, err := controller.Reconcile(context.Background(), object)

		require.ErrorIs(t, err, scope.removeCredentialsRefFinalizerErr)
		require.Equal(t, []error{scope.removeCredentialsRefFinalizerErr}, rec.failureErrors)
		require.NotContains(t, scope.calls, "remove-finalizer")
	})
}

func TestControllerReconcileCloseErrors(t *testing.T) {
	t.Parallel()

	t.Run("close error is joined with reconcile error", func(t *testing.T) {
		t.Parallel()
		scope := &controllerTestScope{closeErr: errors.New("patch failure")}
		rec, controller := newControllerTest(t, newControllerTestObject(), scope)
		rec.timeout = -time.Second
		rec.applyErr = errors.New("apply failure")

		_, err := controller.Reconcile(context.Background(), rec.object)

		require.ErrorIs(t, err, rec.applyErr)
		require.ErrorIs(t, err, scope.closeErr)
		require.Equal(t, []error{rec.applyErr}, rec.failureErrors)
	})

	t.Run("not found close error is ignored", func(t *testing.T) {
		t.Parallel()
		scope := &controllerTestScope{closeErr: apierrors.NewNotFound(corev1.Resource("linodevpcs"), "test")}
		rec, controller := newControllerTest(t, newControllerTestObject(), scope)

		_, err := controller.Reconcile(context.Background(), rec.object)

		require.NoError(t, err)
	})
}
