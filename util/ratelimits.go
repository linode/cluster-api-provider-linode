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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/linode/cluster-api-provider-linode/util/reconciler"
)

// PostRequestCounter keeps track of rate limits for POST to /linode/instances
type PostRequestCounter struct {
	Mu           sync.RWMutex
	ReqRemaining int
	RefreshTime  time.Time
}

var (
	// mu is global lock to coordinate access to shared resource postRequestCounters
	mu sync.RWMutex
	// postRequestCounters stores token hash and pointer to its equivalent PostRequestCounter
	postRequestCounters = make(map[string]*PostRequestCounter, 0)
)

// ApiResponseRatelimitCounter updates ReqRemaining and RefreshTime when a POST call is made to /linode/instances
func (c *PostRequestCounter) ApiResponseRatelimitCounter(resp *resty.Response) error {
	if resp.Request.Method != http.MethodPost || !strings.HasSuffix(resp.Request.URL, "/linode/instances") {
		return nil
	}

	var err error
	c.ReqRemaining, err = strconv.Atoi(resp.Header().Get("X-Ratelimit-Remaining"))
	if err != nil {
		return err
	}

	epochTime, err := strconv.ParseInt(resp.Header().Get("X-Ratelimit-Reset"), 10, 64)
	if err != nil {
		return err
	}
	c.RefreshTime = time.Unix(epochTime, 0)
	return nil
}

// IsPOSTLimitReached checks whether POST limits have been reached.
func (c *PostRequestCounter) IsPOSTLimitReached() bool {
	return time.Now().Before(c.RefreshTime) && c.ReqRemaining == 0
}

// RetryAfter returns how long to wait in seconds for rate-limit to reset
func (c *PostRequestCounter) RetryAfter() time.Duration {
	currTime := time.Now()
	if currTime.After(c.RefreshTime) {
		return 0
	}
	return c.RefreshTime.Sub(currTime) + (1 * time.Second)
}

// GetPostReqCounter returns pointer to PostRequestCounter for a given token hash
func GetPostReqCounter(tokenHash string) *PostRequestCounter {
	mu.Lock()
	defer mu.Unlock()

	ctr, exists := postRequestCounters[tokenHash]
	if !exists {
		ctr = &PostRequestCounter{
			ReqRemaining: reconciler.DefaultPOSTRequestLimit,
			RefreshTime:  time.Time{},
		}
		postRequestCounters[tokenHash] = ctr
	}
	return ctr
}
