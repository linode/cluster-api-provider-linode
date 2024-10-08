package util

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/linode/linodego"
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
