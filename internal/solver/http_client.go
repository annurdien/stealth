package solver

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/imroc/req/v3"

	"github.com/annurdien/stealth/internal/models"
)

var ErrChallengeDetected = errors.New("cloudflare challenge detected")

var (
	// clientPool pools req/v3 clients by proxy key to avoid TLS handshake overhead on every request
	clientPool sync.Map
)

// getPooledClient returns a configured req.Client, reusing an existing one if the proxy matches
func getPooledClient(proxy *models.ProxyConfig) *req.Client {
	proxyKey := ""
	if proxy != nil {
		proxyKey = proxy.HashKey()
	}

	if pooled, ok := clientPool.Load(proxyKey); ok {
		return pooled.(*req.Client)
	}

	client := req.C()
	client.ImpersonateChrome()

	if proxy != nil && proxy.URL != "" {
		proxyURL := proxy.URL
		if proxy.Username != "" {
			if strings.HasPrefix(proxyURL, "http://") {
				proxyURL = "http://" + proxy.Username + ":" + proxy.Password + "@" + strings.TrimPrefix(proxyURL, "http://")
			} else if strings.HasPrefix(proxyURL, "https://") {
				proxyURL = "https://" + proxy.Username + ":" + proxy.Password + "@" + strings.TrimPrefix(proxyURL, "https://")
			} else if strings.HasPrefix(proxyURL, "socks5://") {
				proxyURL = "socks5://" + proxy.Username + ":" + proxy.Password + "@" + strings.TrimPrefix(proxyURL, "socks5://")
			}
		}
		client.SetProxyURL(proxyURL)
	}

	clientPool.Store(proxyKey, client)
	return client
}

// ExecuteNativeRequest attempts standard HTTP request execution using Go's net/http client
func ExecuteNativeRequest(ctx context.Context, v1Req *models.V1Request, cachedCookies []models.Cookie, userAgent string) (*models.Solution, error) {
	client := getPooledClient(v1Req.Proxy)

	// Timeout applies to the specific request only
	r := client.R().SetContext(ctx)

	// req/v3 SetTimeout on client applies to all future requests,
	// so for request-specific timeouts we should ideally use Context timeout,
	// but the context passed here already has the timeout applied via engine.go/server.go
	// However, we'll still set client.SetTimeout just in case, though it affects the pool.
	// A better approach in req/v3 is setting it on the request `r` if supported, but Context works perfectly.

	// Set Headers
	for k, v := range v1Req.Headers {
		r.SetHeader(k, v)
	}

	ua := userAgent
	if ua == "" {
		ua = v1Req.UserAgent
	}
	if ua == "" {
		ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	}
	r.SetHeader("User-Agent", ua)

	// Inject Cookies
	for _, cookie := range cachedCookies {
		r.SetCookies(&http.Cookie{
			Name:  cookie.Name,
			Value: cookie.Value,
		})
	}
	for _, cookie := range v1Req.Cookies {
		r.SetCookies(&http.Cookie{
			Name:  cookie.Name,
			Value: cookie.Value,
		})
	}

	method := strings.ToUpper(v1Req.EffectiveMethod())
	if method == "POST" {
		r.SetBodyString(v1Req.PostData)
	}

	resp, err := r.Send(method, v1Req.URL)
	if err != nil {
		return nil, err
	}

	bodyStr := resp.String()

	// Check for Cloudflare challenge
	if IsCloudflareChallenge(resp.Response, bodyStr) {
		return nil, ErrChallengeDetected
	}

	var outCookies []models.Cookie
	for _, cookie := range resp.Cookies() {
		outCookies = append(outCookies, models.Cookie{
			Name:   cookie.Name,
			Value:  cookie.Value,
			Domain: cookie.Domain,
			Path:   cookie.Path,
		})
	}

	return &models.Solution{
		URL:       v1Req.URL,
		Status:    resp.StatusCode,
		Response:  bodyStr,
		Cookies:   outCookies,
		UserAgent: ua,
	}, nil
}

// IsCloudflareChallenge checks headers and payload bodies for security validation flags
func IsCloudflareChallenge(resp *http.Response, body string) bool {
	serverHeader := strings.ToLower(resp.Header.Get("Server"))
	if strings.Contains(serverHeader, "cloudflare") || strings.Contains(serverHeader, "ddos-guard") {
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == 429 {
			return true
		}
	}

	bodyLower := strings.ToLower(body)
	signatures := []string{
		"challenges.cloudflare.com",
		"just a moment...",
		"cf-challenge-running",
		"__cf_chl_rt_sig",
		"id=\"turnstile-wrapper\"",
		"ddos-guard",
	}
	for _, sig := range signatures {
		if strings.Contains(bodyLower, sig) {
			return true
		}
	}
	return false
}
