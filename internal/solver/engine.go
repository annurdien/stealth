package solver

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"

	"github.com/annurdien/stealth/internal/models"
)

// Version is injected at build time via the main package.
var Version = "1.0.0"

// jsFetchScript is the JavaScript injected into the cleared page context.
// It runs with all Cloudflare cookies in scope (credentials: 'include').
// Returns a plain object: { status: number, body: string, error?: string }
const jsFetchScript = `(url, method, headers, body) => {
	const opts = { method: method, credentials: 'include' };
	if (headers && Object.keys(headers).length > 0) {
		opts.headers = headers;
	}
	if (body && method !== 'GET') {
		opts.body = body;
	}
	return fetch(url, opts)
		.then(res => res.text().then(text => ({
			status: res.status,
			body: text,
			error: '',
			userAgent: navigator.userAgent,
			url: window.location.href
		})))
		.catch(err => ({
			status: 0,
			body: '',
			error: err.message,
			userAgent: navigator.userAgent,
			url: window.location.href
		}));
}`

// Solve is the main orchestrator. It:
//  1. Sets up the page (media blocking, cookies, user agent)
//  2. Navigates to the base domain to clear any Cloudflare challenge
//  3. Solves Turnstile if detected
//  4. Executes the fetch() injection with the client's headers/body
func Solve(ctx context.Context, page *rod.Page, req *models.V1Request) (*models.V1Response, error) {
	startTs := time.Now().UnixMilli()

	timeoutMs := req.EffectiveTimeout()
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	page = page.Context(timeoutCtx)

	if req.UserAgent != "" {
		uaCmd := proto.EmulationSetUserAgentOverride{UserAgent: req.UserAgent}
		if err := uaCmd.Call(page); err != nil {
			return errorResp(startTs, "Failed to set user agent: "+err.Error()), nil
		}
	}

	if req.DisableMedia {
		enableMediaBlocking(page)
	}

	if len(req.Cookies) > 0 {
		if err := injectCookies(page, req.Cookies); err != nil {
			return errorResp(startTs, "Failed to inject cookies: "+err.Error()), nil
		}
	}

	baseURL, err := models.ExtractBaseURL(req.URL)
	if err != nil {
		return errorResp(startTs, "Invalid URL: "+err.Error()), nil
	}

	if err := page.Navigate(baseURL); err != nil {
		return errorResp(startTs, "Navigation failed: "+err.Error()), nil
	}

	_ = page.WaitLoad()

	challengeMessage := "Challenge not detected!"

	result := DetectChallenge(page)
	switch result {
	case ChallengeAccessDenied:
		return errorResp(startTs,
			"Cloudflare has blocked this request. Your IP is probably banned for this site."), nil

	case ChallengeFound:
		challengeMessage = "Challenge solved!"
		if err := WaitForChallengeResolution(page, timeoutMs); err != nil {
			// Capture screenshot for debugging if available
			return errorResp(startTs, "Error: "+err.Error()), nil
		}
	}

	if req.WaitAfterMs > 0 {
		time.Sleep(time.Duration(req.WaitAfterMs) * time.Millisecond)
	}

	var solution *models.Solution

	if req.ReturnOnlyCookies {
		solution, err = buildCookieOnlySolution(page, req)
		if err != nil {
			return errorResp(startTs, "Error extracting cookies: "+err.Error()), nil
		}
	} else {
		solution, err = executeFetch(page, req)
		if err != nil {
			return errorResp(startTs, "Error: "+err.Error()), nil
		}
	}

	return &models.V1Response{
		Status:         "ok",
		Message:        challengeMessage,
		StartTimestamp: startTs,
		EndTimestamp:   time.Now().UnixMilli(),
		Version:        Version,
		Solution:       solution,
	}, nil
}

// executeFetch injects the fetch() script into the page and returns the result.
func executeFetch(page *rod.Page, req *models.V1Request) (*models.Solution, error) {
	method := req.EffectiveMethod()
	headers := req.Headers
	if headers == nil {
		headers = map[string]string{}
	}

	postData := ""
	if strings.EqualFold(method, "POST") {
		postData = req.PostData
	}

	res, err := page.Eval(jsFetchScript, req.URL, method, headers, postData)
	if err != nil {
		return nil, fmt.Errorf("fetch eval failed: %w", err)
	}

	jsError := res.Value.Get("error").String()
	if jsError != "" {
		return nil, fmt.Errorf("fetch failed: %s", jsError)
	}

	status := int(res.Value.Get("status").Int())
	body := res.Value.Get("body").String()
	userAgent := res.Value.Get("userAgent").String()
	resolvedURL := res.Value.Get("url").String()

	cookies, err := extractCookies(page)
	if err != nil {
		return nil, fmt.Errorf("cookie extraction failed: %w", err)
	}

	solution := &models.Solution{
		URL:       resolvedURL,
		Status:    status,
		Response:  body,
		Cookies:   cookies,
		UserAgent: userAgent,
	}

	if req.ReturnScreenshot {
		if ss, err := page.Screenshot(true, nil); err == nil {
			solution.Screenshot = base64.StdEncoding.EncodeToString(ss)
		}
	}

	return solution, nil
}

// buildCookieOnlySolution returns cookies and userAgent without executing a fetch.
func buildCookieOnlySolution(page *rod.Page, req *models.V1Request) (*models.Solution, error) {
	cookies, err := extractCookies(page)
	if err != nil {
		return nil, err
	}

	userAgent := ""
	resolvedURL := req.URL

	res, err := page.Eval(`() => ({ userAgent: navigator.userAgent, url: window.location.href })`)
	if err == nil && res != nil {
		userAgent = res.Value.Get("userAgent").String()
		resolvedURL = res.Value.Get("url").String()
	}

	solution := &models.Solution{
		URL:       resolvedURL,
		Status:    0,
		Response:  "",
		Cookies:   cookies,
		UserAgent: userAgent,
	}

	if req.ReturnScreenshot {
		if ss, err := page.Screenshot(true, nil); err == nil {
			solution.Screenshot = base64.StdEncoding.EncodeToString(ss)
		}
	}

	return solution, nil
}

// extractCookies retrieves all cookies from the browser via CDP.
// Using CDP (not JS) ensures httpOnly cookies are included.
func extractCookies(page *rod.Page) ([]models.Cookie, error) {
	result, err := proto.NetworkGetCookies{}.Call(page)
	if err != nil {
		return nil, err
	}

	cookies := make([]models.Cookie, 0, len(result.Cookies))
	for _, c := range result.Cookies {
		cookies = append(cookies, models.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  float64(c.Expires),
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
			SameSite: string(c.SameSite),
		})
	}
	return cookies, nil
}

// enableMediaBlocking configures CDP Network to block images, CSS, and fonts.
func enableMediaBlocking(page *rod.Page) {
	_ = proto.NetworkEnable{}.Call(page)
	_ = proto.NetworkSetBlockedURLs{Urls: MediaBlockPatterns}.Call(page)
}

// injectCookies sets cookies in the browser context via CDP.
func injectCookies(page *rod.Page, cookies []models.Cookie) error {
	for _, c := range cookies {
		params := proto.NetworkSetCookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Secure:   c.Secure,
			HTTPOnly: c.HTTPOnly,
		}
		if _, err := params.Call(page); err != nil {
			return fmt.Errorf("failed to set cookie %q: %w", c.Name, err)
		}
	}
	return nil
}

// errorResp builds a V1Response for error cases.
func errorResp(startTs int64, message string) *models.V1Response {
	return &models.V1Response{
		Status:         "error",
		Message:        message,
		StartTimestamp: startTs,
		EndTimestamp:   time.Now().UnixMilli(),
		Version:        Version,
		Solution:       nil,
	}
}
