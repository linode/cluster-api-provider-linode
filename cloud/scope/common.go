package scope

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/bxcodec/gotcha"
	"github.com/bxcodec/gotcha/cache"
	"github.com/bxcodec/httpcache"
	"github.com/bxcodec/httpcache/cache/inmem"
	"github.com/linode/linodego"
	"golang.org/x/oauth2"
)

const (
	// DefaultLinodeAPITimeout is the default timeout for Linode API calls.
	DefaultLinodeAPITimeout = 10 * time.Second
	// DefaultLinodeAPIMaxIdleConns is the default max idle connections for Linode API calls.
	DefaultLinodeAPIMaxIdleConns = 100
	// DefaultLinodeAPICacheControlOverride is the default cache control override for Linode API calls.
	DefaultLinodeAPICacheControlOverride = "max-age=60"
	// DefaultLinodeAPICacheExpiration is the default cache expiration for Linode API calls.
	DefaultLinodeAPICacheExpiration = 5 * time.Second
	// DefaultLinodeAPICacheMaxSizeItem is the default cache max size item for Linode API calls.
	DefaultLinodeAPICacheMaxSizeItem = 500
	// DefaultLinodeAPICacheMaxMemory is the default cache max memory for Linode API calls.
	DefaultLinodeAPICacheMaxMemory = cache.MB
)

var (
	initClient   sync.Once
	linodeClient linodego.Client
)

func createLinodeClient(apiKey string) *linodego.Client {
	initClient.Do(func() {
		//nolint:forcetypeassert // Always OK.
		baseTrans := http.DefaultTransport.(*http.Transport).Clone()
		baseTrans.MaxIdleConns = DefaultLinodeAPIMaxIdleConns
		// Otherwise we use just a few connections.
		baseTrans.MaxConnsPerHost = baseTrans.MaxIdleConns
		baseTrans.MaxIdleConnsPerHost = baseTrans.MaxIdleConns

		oauthTrans := &oauth2.Transport{
			Source: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: apiKey}),
			Base:   baseTrans,
		}

		oauth2Client := &http.Client{
			Timeout:   DefaultLinodeAPITimeout,
			Transport: oauthTrans,
		}

		cacheStore := gotcha.New(&cache.Option{
			AlgorithmType: cache.LRUAlgorithm,
			ExpiryTime:    DefaultLinodeAPICacheExpiration,
			MaxSizeItem:   DefaultLinodeAPICacheMaxSizeItem,
			MaxMemory:     DefaultLinodeAPICacheMaxMemory,
		})
		cachedTrans := httpcache.NewCacheHandlerRoundtrip(&maxAgeFixRoundTripper{
			oauthTrans,
		}, false, inmem.NewCache(cacheStore))

		oauth2Client.Transport = &reqURIFixRoundTripper{
			plain:  oauthTrans,
			cached: cachedTrans,
		}

		linodeClient = linodego.NewClient(oauth2Client)
	})

	return &linodeClient
}

type maxAgeFixRoundTripper struct {
	http.RoundTripper
}

func (r *maxAgeFixRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.RoundTripper.RoundTrip(req)
	if err == nil && req.Method == http.MethodGet {
		// Workaround, because Linode API sends max-age=0 all the time.
		resp.Header.Set("Cache-Control", DefaultLinodeAPICacheControlOverride)
	}

	return resp, err
}

type reqURIFixRoundTripper struct {
	plain  http.RoundTripper
	cached http.RoundTripper
}

func (r *reqURIFixRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet {
		return r.plain.RoundTrip(req)
	}

	// Workaround because Linodego doesn't set it.
	req.RequestURI = req.URL.Path

	if filter := req.Header.Get("X-Filter"); filter != "" {
		req.RequestURI = fmt.Sprintf("%s?filter=%s", req.RequestURI, filter)
	}

	return r.cached.RoundTrip(req)
}
