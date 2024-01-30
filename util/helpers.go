package util

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/linode/linodego"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Pointer returns the pointer of any type
func Pointer[T any](t T) *T {
	return &t
}

// RenderObjectLabel renders a 63 charater long unique label
func RenderObjectLabel(i types.UID) string {
	return fmt.Sprintf("CLi-%s", strings.ReplaceAll(string(i), "-", ""))
}

// CreateLinodeAPIFilter converts variables to API filter string
func CreateLinodeAPIFilter(label string, tags []string) string {
	filter := map[string]string{}

	if label != "" {
		filter["label"] = label
	}

	if len(tags) != 0 {
		filter["tags"] = strings.Join(tags, ",")
	}

	rawFilter, err := json.Marshal(filter)
	if err != nil {
		// This should never happen
		panic(err.Error() + " Oh, snap... Earth has over, we can't parse map[string]string to JSON! I'm going to die ...")
	}

	return string(rawFilter)
}

// IgnoreLinodeAPIError returns the error except matches to status code
func IgnoreLinodeAPIError(err error, code int) error {
	apiErr := linodego.Error{Code: code}
	if apiErr.Is(err) {
		err = nil
	}

	return err
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
