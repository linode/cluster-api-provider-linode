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

func TestIsLinodePrivateIP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		// Valid IPs in the Linode private range (192.168.128.0/17)
		{
			name:     "valid IP at start of range",
			ip:       "192.168.128.0",
			expected: true,
		},
		{
			name:     "valid IP in middle of range",
			ip:       "192.168.200.123",
			expected: true,
		},
		{
			name:     "valid IP at end of range",
			ip:       "192.168.255.255",
			expected: true,
		},
		{
			name:     "valid IP at boundary of range",
			ip:       "192.168.255.254",
			expected: true,
		},

		// Valid IPs outside the Linode private range
		{
			name:     "valid IP below range",
			ip:       "192.168.127.255",
			expected: false,
		},
		{
			name:     "valid IP above range",
			ip:       "192.169.0.0",
			expected: false,
		},
		{
			name:     "private IP from different range (10.0.0.0/8)",
			ip:       "10.0.0.1",
			expected: false,
		},
		{
			name:     "public IP",
			ip:       "203.0.113.1",
			expected: false,
		},
		{
			name:     "localhost IP",
			ip:       "127.0.0.1",
			expected: false,
		},

		// Invalid IP formats
		{
			name:     "empty string",
			ip:       "",
			expected: false,
		},
		{
			name:     "invalid format",
			ip:       "not-an-ip",
			expected: false,
		},
		{
			name:     "incomplete IP",
			ip:       "192.168",
			expected: false,
		},
		{
			name:     "IPv6 address",
			ip:       "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			expected: false,
		},
		{
			name:     "IP with invalid segments",
			ip:       "192.168.256.1",
			expected: false,
		},
		{
			name:     "IP with extra segments",
			ip:       "192.168.1.1.5",
			expected: false,
		},
	}

	for _, tt := range tests {
		testcase := tt
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			result := IsLinodePrivateIP(testcase.ip)
			if result != testcase.expected {
				t.Errorf("IsLinodePrivateIP(%q) = %v, want %v", testcase.ip, result, testcase.expected)
			}
		})
	}
}
