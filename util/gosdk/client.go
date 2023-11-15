package gosdk

import (
	"net/http"

	"github.com/linode/linodego"
	"golang.org/x/oauth2"
)

// Client is wrapped Linode client to implement project specific helpers.
type Client struct {
	linodego.Client
}

// NewClient constructs a Linode client.
func NewClient(apiKey string) *Client {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: apiKey})

	oauth2Client := &http.Client{
		Transport: &oauth2.Transport{
			Source: tokenSource,
		},
	}

	return &Client{
		linodego.NewClient(oauth2Client),
	}
}
