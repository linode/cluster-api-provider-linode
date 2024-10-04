package util

import (
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/linode/linodego"
)

func TestIgnoreLinodeAPIError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		err          error
		code         []int
		shouldFilter bool
	}{{
		name:         "Not Linode API error",
		err:          errors.New("foo"),
		code:         []int{0},
		shouldFilter: false,
	}, {
		name: "Ignore not found Linode API error",
		err: linodego.Error{
			Response: nil,
			Code:     400,
			Message:  "not found",
		},
		code:         []int{400},
		shouldFilter: true,
	}, {
		name: "Don't ignore not found Linode API error",
		err: linodego.Error{
			Response: nil,
			Code:     400,
			Message:  "not found",
		},
		code:         []int{500},
		shouldFilter: false,
	}, {
		name: "Don't ignore with 2+ API errors",
		err: linodego.Error{
			Response: nil,
			Code:     400,
			Message:  "not found",
		},
		code:         []int{500, 418},
		shouldFilter: false,
	}, {
		name: "Ignore with 2+ API errors",
		err: linodego.Error{
			Response: nil,
			Code:     418,
			Message:  "not found",
		},
		code:         []int{500, 418},
		shouldFilter: true,
	}}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			err := IgnoreLinodeAPIError(testcase.err, testcase.code...)
			if testcase.shouldFilter && err != nil {
				t.Error("expected err but got nil")
			}
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{{
		name: "unexpected EOF",
		err:  io.ErrUnexpectedEOF,
		want: true,
	}, {
		name: "not found Linode API error",
		err: &linodego.Error{
			Response: nil,
			Code:     http.StatusNotFound,
			Message:  "not found",
		},
		want: false,
	}, {
		name: "Rate limiting Linode API error",
		err: &linodego.Error{
			Response: nil,
			Code:     http.StatusTooManyRequests,
			Message:  "rate limited",
		},
		want: true,
	}}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			if testcase.want != IsRetryableError(testcase.err) {
				t.Errorf("wanted %v, got %v", testcase.want, IsRetryableError(testcase.err))
			}
		})
	}
}

func TestGetInstanceID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		providerID *string
		wantErr    bool
		wantID     int
	}{{
		name:       "nil",
		providerID: nil,
		wantErr:    true,
		wantID:     -1,
	}, {
		name:       "invalid provider ID",
		providerID: Pointer("linode://foobar"),
		wantErr:    true,
		wantID:     -1,
	}, {
		name:       "valid",
		providerID: Pointer("linode://12345"),
		wantErr:    false,
		wantID:     12345,
	}}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			gotID, err := GetInstanceID(testcase.providerID)
			if testcase.wantErr && err == nil {
				t.Errorf("wanted %v, got %v", testcase.wantErr, err)
			}
			if gotID != testcase.wantID {
				t.Errorf("wanted %v, got %v", testcase.wantID, gotID)
			}
		})
	}
}
