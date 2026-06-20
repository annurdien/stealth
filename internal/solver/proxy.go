package solver

import (
	"fmt"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// EnableProxyAuth sets up CDP Fetch domain interception to handle
// HTTP 407 Proxy Authentication Required challenges.
//
// Chrome ignores credentials embedded in proxy URLs (http://user:pass@host:port).
// Chrome extensions (which FlareSolverr uses) don't load in headless mode.
// This approach uses CDP's Fetch domain to intercept auth challenges directly
// at the protocol level — cleanest solution for headless Go/Rod.
func EnableProxyAuth(page *rod.Page, username, password string) error {
	// Enable the Fetch domain and configure it to pause on auth challenges.
	fetchEnable := proto.FetchEnable{HandleAuthRequests: true}
	if err := fetchEnable.Call(page); err != nil {
		return fmt.Errorf("failed to enable CDP Fetch domain: %w", err)
	}

	// Listen for auth-required and generic paused events concurrently.
	go func() {
		page.EachEvent(
			// Handler for proxy auth challenges: provide credentials.
			func(e *proto.FetchAuthRequired) (stop bool) {
				continueAuth := proto.FetchContinueWithAuth{
					RequestID: e.RequestID,
					AuthChallengeResponse: &proto.FetchAuthChallengeResponse{
						Response: proto.FetchAuthChallengeResponseResponseProvideCredentials,
						Username: username,
						Password: password,
					},
				}
				_ = continueAuth.Call(page)
				return false
			},
			// Handler for all other paused requests: continue normally.
			func(e *proto.FetchRequestPaused) (stop bool) {
				continueReq := proto.FetchContinueRequest{RequestID: e.RequestID}
				_ = continueReq.Call(page)
				return false
			},
		)()
	}()

	return nil
}

// DisableProxyAuth disables the CDP Fetch domain interception.
func DisableProxyAuth(page *rod.Page) error {
	disable := proto.FetchDisable{}
	return disable.Call(page)
}

