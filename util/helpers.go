package util

import (
	"github.com/linode/linodego"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Pointer returns the pointer of any type
func Pointer[T any](t T) *T {
	return &t
}

func NewPatchHelper(obj client.Object, crClient client.Client) (*patch.Helper, error) {
	return patch.NewHelper(obj, crClient)
}

// IgnoreLinodeAPIError returns the error except matches to status code
func IgnoreLinodeAPIError(err error, code int) error {
	apiErr := linodego.Error{Code: code}
	if apiErr.Is(err) {
		err = nil
	}

	return err
}
