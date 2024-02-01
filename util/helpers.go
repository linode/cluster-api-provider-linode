package util

import (
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Pointer returns the pointer of any type
func Pointer[T any](t T) *T {
	return &t
}

// IgnoreKubeNotFound returns the error even if aggregated except not found
func IgnoreKubeNotFound(err error) error {
	//nolint:errorlint // This is specific non wrapped error.
	errs, ok := err.(kerrors.Aggregate)
	if !ok {
		return client.IgnoreNotFound(err)
	}

	for _, e := range errs.Errors() {
		if client.IgnoreNotFound(e) != nil {
			return err
		}
	}

	return nil
}
