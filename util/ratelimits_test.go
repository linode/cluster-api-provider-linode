/*
Copyright 2023 Akamai Technologies, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/linode/cluster-api-provider-linode/util/reconciler"
)

func TestGetPostReqCounter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		tokenHash string
		want      *PostRequestCounter
	}{
		{
			name:      "provide hash which exists in map",
			tokenHash: "abcdef",
			want: &PostRequestCounter{
				ReqRemaining: 5,
				RefreshTime:  3,
			},
		},
		{
			name:      "provide hash which doesn't exist",
			tokenHash: "uvwxyz",
			want: &PostRequestCounter{
				ReqRemaining: reconciler.DefaultPOSTRequestLimit,
				RefreshTime:  0,
			},
		},
	}
	for _, tt := range tests {
		postRequestCounters["abcdef"] = &PostRequestCounter{
			ReqRemaining: reconciler.SecondaryPOSTRequestLimit,
			RefreshTime:  3,
		}
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := GetPostReqCounter(tt.tokenHash); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetPostReqCounter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPostRequestCounter_IsPOSTLimitReached(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		fields *PostRequestCounter
		want   bool
	}{
		{
			name: "not reached rate limits",
			fields: &PostRequestCounter{
				ReqRemaining: 7,
				RefreshTime:  int(time.Now().Unix()),
			},
			want: false,
		},
		{
			name: "reached token rate limit",
			fields: &PostRequestCounter{
				ReqRemaining: reconciler.SecondaryPOSTRequestLimit,
				RefreshTime:  int(time.Now().Unix() + 100),
			},
			want: true,
		},
		{
			name: "reached account rate limits",
			fields: &PostRequestCounter{
				ReqRemaining: 0,
				RefreshTime:  int(time.Now().Unix() + 100),
			},
			want: true,
		},
		{
			name: "refresh time smaller than current time",
			fields: &PostRequestCounter{
				ReqRemaining: reconciler.SecondaryPOSTRequestLimit,
				RefreshTime:  int(time.Now().Unix() - 100),
			},
			want: false,
		},
		{
			name: "refresh time smaller than current time",
			fields: &PostRequestCounter{
				ReqRemaining: 0,
				RefreshTime:  int(time.Now().Unix() - 100),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &PostRequestCounter{
				ReqRemaining: tt.fields.ReqRemaining,
				RefreshTime:  tt.fields.RefreshTime,
			}
			if got := c.IsPOSTLimitReached(); got != tt.want {
				t.Errorf("PostRequestCounter.IsPOSTLimitReached() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPostRequestCounter_ApiResponseRatelimitCounter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		fields  *PostRequestCounter
		args    *resty.Response
		wantErr bool
	}{
		{
			name: "not a POST call",
			fields: &PostRequestCounter{
				ReqRemaining: 6,
				RefreshTime:  int(time.Now().Unix()),
			},
			args: &resty.Response{
				Request: &resty.Request{
					Method: http.MethodGet,
				},
			},
			wantErr: false,
		},
		{
			name: "endpoint different than /linode/instances",
			fields: &PostRequestCounter{
				ReqRemaining: 6,
				RefreshTime:  int(time.Now().Unix()),
			},
			args: &resty.Response{
				Request: &resty.Request{
					Method: http.MethodPost,
					URL:    "/v4/vpc/ips",
				},
			},
			wantErr: false,
		},
		{
			name: "no headers in response",
			fields: &PostRequestCounter{
				ReqRemaining: 6,
				RefreshTime:  int(time.Now().Unix()),
			},
			args: &resty.Response{
				Request: &resty.Request{
					Method: http.MethodPost,
					URL:    "/v4/linode/instances",
				},
			},
			wantErr: true,
		},
		{
			name: "missing one value in response header",
			fields: &PostRequestCounter{
				ReqRemaining: 6,
				RefreshTime:  int(time.Now().Unix()),
			},
			args: &resty.Response{
				Request: &resty.Request{
					Method: http.MethodPost,
					URL:    "/v4/linode/instances",
				},
				RawResponse: &http.Response{
					Header: http.Header{"X-Ratelimit-Remaining": []string{"10"}},
				},
			},
			wantErr: true,
		},
		{
			name: "correct headers in response",
			fields: &PostRequestCounter{
				ReqRemaining: 6,
				RefreshTime:  int(time.Now().Unix()),
			},
			args: &resty.Response{
				Request: &resty.Request{
					Method: http.MethodPost,
					URL:    "/v4/linode/instances",
				},
				RawResponse: &http.Response{
					Header: http.Header{"X-Ratelimit-Remaining": []string{"10"}, "X-Ratelimit-Reset": []string{"10"}},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &PostRequestCounter{
				ReqRemaining: tt.fields.ReqRemaining,
				RefreshTime:  tt.fields.RefreshTime,
			}
			if err := c.ApiResponseRatelimitCounter(tt.args); (err != nil) != tt.wantErr {
				t.Errorf("PostRequestCounter.ApiResponseRatelimitCounter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
