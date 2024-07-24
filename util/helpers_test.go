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
		code         int
		shouldFilter bool
	}{{
		name:         "Not Linode API error",
		err:          errors.New("foo"),
		code:         0,
		shouldFilter: false,
	}, {
		name: "Ignore not found Linode API error",
		err: linodego.Error{
			Response: nil,
			Code:     400,
			Message:  "not found",
		},
		code:         400,
		shouldFilter: true,
	}, {
		name: "Don't ignore not found Linode API error",
		err: linodego.Error{
			Response: nil,
			Code:     400,
			Message:  "not found",
		},
		code:         500,
		shouldFilter: false,
	}}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			err := IgnoreLinodeAPIError(testcase.err, testcase.code)
			if testcase.shouldFilter && err != nil {
				t.Error("expected err but got nil")
			}
		})
	}
}

func TestIsTransientError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		err         error
		shouldRetry bool
	}{{
		name:        "unexpected EOF",
		err:         io.ErrUnexpectedEOF,
		shouldRetry: true,
	}, {
		name: "not found Linode API error",
		err: &linodego.Error{
			Response: nil,
			Code:     http.StatusNotFound,
			Message:  "not found",
		},
		shouldRetry: false,
	}, {
		name: "Rate limiting Linode API error",
		err: &linodego.Error{
			Response: nil,
			Code:     http.StatusTooManyRequests,
			Message:  "rate limited",
		},
		shouldRetry: true,
	}}
	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			if testcase.shouldRetry != IsTransientError(testcase.err) {
				t.Errorf("wanted %v, got %v", testcase.shouldRetry, IsTransientError(testcase.err))
			}
		})
	}
}
