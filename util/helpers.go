package util

import (
	"encoding/json"
	"strings"

	"github.com/linode/linodego"
)

// Pointer returns the pointer of any type
func Pointer[T any](t T) *T {
	return &t
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
