package util

import (
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
