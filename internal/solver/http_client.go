package solver

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/imroc/req/v3"

	"github.com/annurdien/stealth/internal/models"
)

var ErrChallengeDetected = errors.New("cloudflare challenge detected")

// ExecuteNativeRequest attempts standard HTTP request execution using Go's net/http client
func ExecuteNativeRequest(ctx context.Context, v1Req *models.V1Request, cachedCookies []models.Cookie, userAgent string) (*models.Solution, error) {
	client := req.C()
	client.ImpersonateChrome()
	client.SetTimeout(time.Duration(v1Req.EffectiveTimeout()) * time.Millisecond)

	if v1Req.Proxy != nil && v1Req.Proxy.URL != "" {
		proxyURL := v1Req.Proxy.URL
		if v1Req.Proxy.Username != "" {
			// Parse proxy URL to inject credentials
			if strings.HasPrefix(proxyURL, "http://") {
				proxyURL = "http://" + v1Req.Proxy.Username + ":" + v1Req.Proxy.Password + "@" + strings.TrimPrefix(proxyURL, "http://")
			} else if strings.HasPrefix(proxyURL, "https://") {
				proxyURL = "https://" + v1Req.Proxy.Username + ":" + v1Req.Proxy.Password + "@" + strings.TrimPrefix(proxyURL, "https://")
			} else if strings.HasPrefix(proxyURL, "socks5://") {
				proxyURL = "socks5://" + v1Req.Proxy.Username + ":" + v1Req.Proxy.Password + "@" + strings.TrimPrefix(proxyURL, "socks5://")
			}
		}
		client.SetProxyURL(proxyURL)
	}

	r := client.R().SetContext(ctx)

	// Set Headers
	for k, v := range v1Req.Headers {
		r.SetHeader(k, v)
	}

	ua := userAgent
	if ua == "" {
		ua = v1Req.UserAgent
	}
	if ua == "" {
		ua = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36"
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
	defer resp.Body.Close()

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
