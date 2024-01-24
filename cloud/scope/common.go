package scope

import (
	"net/http"

	"github.com/linode/linodego"
	"golang.org/x/oauth2"
)

func createLinodeClient(apiKey string) *linodego.Client {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: apiKey})

	oauth2Client := &http.Client{
		Transport: &oauth2.Transport{
			Source: tokenSource,
		},
	}
	linodeClient := linodego.NewClient(oauth2Client)

	return &linodeClient
}
