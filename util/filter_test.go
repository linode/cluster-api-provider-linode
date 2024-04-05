package util

import "testing"

func TestString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		filter    Filter
		expectErr bool
	}{{
		name: "success ID",
		filter: Filter{
			ID:    Pointer(123),
			Label: "",
			Tags:  nil,
		},
		expectErr: false,
	}, {
		name: "success label",
		filter: Filter{
			ID:    nil,
			Label: "test",
			Tags:  nil,
		},
		expectErr: false,
	}, {
		name: "success tags",
		filter: Filter{
			ID:    nil,
			Label: "",
			Tags:  []string{"testtag"},
		},
		expectErr: false,
	}, {
		name: "success additional info",
		filter: Filter{
			ID:                nil,
			Label:             "",
			Tags:              []string{"testtag"},
			AdditionalFilters: map[string]string{"mine": "true"},
		},
		expectErr: false,
	}, {
		name: "failure unmarshal",
		filter: Filter{
			ID:    nil,
			Label: "",
			Tags:  []string{},
		},
		expectErr: true,
	}}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			_, err := testcase.filter.String()
			if testcase.expectErr && err != nil {
				t.Error("expected err but got nil")
			}
		})
	}
}
