package util

import (
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/linode/linodego"
)

// Pointer returns the pointer of any type
func Pointer[T any](t T) *T {
	return &t
}

// IgnoreLinodeAPIError returns the error except matches to status code
func IgnoreLinodeAPIError(err error, code int) error {
	apiErr := linodego.Error{Code: code}
	if apiErr.Is(err) {
		err = nil
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

// IsTransientError determines if the error is transient, meaning a controller that
// encounters this error should requeue reconciliation to try again later
func IsTransientError(err error) bool {
	if linodego.ErrHasStatus(
		err,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusGatewayTimeout,
		http.StatusServiceUnavailable) {
		return true
	}

	if errors.Is(err, http.ErrHandlerTimeout) || errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	return false
}
