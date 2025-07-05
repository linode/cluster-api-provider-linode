package util

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/linode/linodego"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
)

// Pointer returns the pointer of any type
func Pointer[T any](t T) *T {
	return &t
}

// IgnoreLinodeAPIError returns the error except matches to status code
func IgnoreLinodeAPIError(err error, codes ...int) error {
	for _, code := range codes {
		apiErr := linodego.Error{Code: code}
		if apiErr.Is(err) {
			return nil
		}
	}

	return err
}

// UnwrapError safely unwraps an error until it can't be unwrapped.
func UnwrapError(err error) error {
	var wrappedErr interface{ Unwrap() error }
	for errors.As(err, &wrappedErr) {
		err = errors.Unwrap(err)
	}

	return err
}

// IsRetryableError determines if the error is retryable, meaning a controller that
// encounters this error should requeue reconciliation to try again later
func IsRetryableError(err error) bool {
	return linodego.ErrHasStatus(
		err,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusGatewayTimeout,
		http.StatusServiceUnavailable,
		linodego.ErrorFromError) || errors.Is(err, http.ErrHandlerTimeout) || errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(err, io.ErrUnexpectedEOF)
}

// GetInstanceID determines the instance ID from the ProviderID
func GetInstanceID(providerID *string) (int, error) {
	if providerID == nil {
		err := errors.New("nil ProviderID")
		return -1, err
	}
	instanceID, err := strconv.Atoi(strings.TrimPrefix(*providerID, "linode://"))
	if err != nil {
		return -1, err
	}
	return instanceID, nil
}

// GetAutoGenTags returns tags to be added to linods when a cluster is provisioned using CAPL
func GetAutoGenTags(cluster *infrav1alpha2.LinodeCluster) []string {
	if cluster == nil || cluster.Name == "" {
		return []string{}
	}
	return []string{cluster.Name}
}

// IsLinodePrivateIP checks if an IP address belongs to the Linode private IP range (192.168.128.0/17)
func IsLinodePrivateIP(ipAddress string) bool {
	// Parse the IP address
	ip := net.ParseIP(ipAddress)
	if ip == nil {
		return false
	}

	// Define the Linode private IP CIDR (192.168.128.0/17)
	_, linodePrivateNet, err := net.ParseCIDR("192.168.128.0/17")
	if err != nil {
		// This should never happen with a hardcoded valid CIDR
		return false
	}

	// Check if the IP is contained in the Linode private network
	return linodePrivateNet.Contains(ip)
}

// SetOwnerReferenceToLinodeCluster fetches the LinodeCluster and sets it as the owner reference of a given object.
func SetOwnerReferenceToLinodeCluster(ctx context.Context, k8sclient client.Client, cluster *clusterv1.Cluster, obj client.Object, scheme *runtime.Scheme) error {
	logger := log.Log.WithName("SetOwnerReferenceToLinodeCluster")

	if cluster == nil || cluster.Spec.InfrastructureRef == nil {
		logger.Info("the Cluster or InfrastructureRef is nil, cannot fetch LinodeCluster")
		return nil
	}

	var linodeCluster infrav1alpha2.LinodeCluster
	key := types.NamespacedName{
		Namespace: cluster.Spec.InfrastructureRef.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := k8sclient.Get(ctx, key, &linodeCluster); err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to fetch LinodeCluster")
			return err
		}
		logger.Info("LinodeCluster not found, skipping owner reference setting")
		return nil
	}

	if err := controllerutil.SetControllerReference(&linodeCluster, obj, scheme); err != nil {
		logger.Error(err, "Failed to set owner reference to LinodeCluster")
		return err
	}

	if err := k8sclient.Update(ctx, obj); err != nil {
		logger.Error(err, "Failed to update object")
		return err
	}

	return nil
}
