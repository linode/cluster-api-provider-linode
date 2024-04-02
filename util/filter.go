package util

import (
	"encoding/json"
	"maps"
	"strconv"
	"strings"
)

// Filter holds the fields used for filtering results from the Linode API.
//
// The fields within Filter are prioritized so that only the most-specific
// field is present when Filter is marshaled to JSON.
type Filter struct {
	ID                *int              // Filter on the resource's ID (most specific).
	Label             string            // Filter on the resource's label.
	Tags              []string          // Filter resources by their tags (least specific).
	AdditionalFilters map[string]string // Filter resources by additional parameters
}

// MarshalJSON returns a JSON-encoded representation of a [Filter].
// The resulting encoded value will have exactly 1 (one) field present.
// See [Filter] for details on the value precedence.
func (f Filter) MarshalJSON() ([]byte, error) {
	filter := make(map[string]string, len(f.AdditionalFilters)+1)
	switch {
	case f.ID != nil:
		filter["id"] = strconv.Itoa(*f.ID)
	case f.Label != "":
		filter["label"] = f.Label
	case len(f.Tags) != 0:
		filter["tags"] = strings.Join(f.Tags, ",")
	}

	maps.Copy(filter, f.AdditionalFilters)
	return json.Marshal(filter)
}

// String returns the string representation of the encoded value from
// [Filter.MarshalJSON].
func (f Filter) String() (string, error) {
	p, err := f.MarshalJSON()
	if err != nil {
		return "", err
	}

	return string(p), nil
}
