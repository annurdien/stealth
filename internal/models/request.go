package models

// V1Request is the input payload for POST /v1/request.
type V1Request struct {
	// URL is the target URL to navigate to and/or fetch from. Required.
	URL string `json:"url"`

	// Method is the HTTP method for the fetch injection step.
	// Allowed: "GET", "POST". Defaults to "GET" if empty.
	Method string `json:"method"`

	// MaxTimeout is the maximum execution time in milliseconds.
	// Defaults to 60000 (60s).
	MaxTimeout int `json:"maxTimeout"`

	// Session is an optional session ID. If provided, the request
	// reuses an existing browser session. If the session doesn't exist,
	// a new one is created with this ID.
	Session string `json:"session,omitempty"`

	// Headers are custom HTTP headers passed to the fetch() call.
	// Only applied in Phase 2 fetch injection, NOT during initial navigation.
	Headers map[string]string `json:"headers,omitempty"`

	// PostData is the request body for POST requests.
	// Ignored when Method is "GET".
	PostData string `json:"postData,omitempty"`

	// Proxy configures an HTTP/SOCKS5 proxy for the browser instance.
	// Only applied when creating a new session/browser.
	Proxy *ProxyConfig `json:"proxy,omitempty"`

	// UserAgent overrides the browser's User-Agent string.
	UserAgent string `json:"userAgent,omitempty"`

	// Cookies are injected into the browser context before navigation.
	Cookies []Cookie `json:"cookies,omitempty"`

	// ReturnOnlyCookies skips Phase 2 fetch and returns only clearance cookies.
	ReturnOnlyCookies bool `json:"returnOnlyCookies,omitempty"`

	// ReturnScreenshot includes a base64-encoded PNG screenshot of the page.
	ReturnScreenshot bool `json:"returnScreenshot,omitempty"`

	// DisableMedia blocks images, CSS, and fonts during navigation.
	// Significantly speeds up challenge solving.
	DisableMedia bool `json:"disableMedia,omitempty"`

	// WaitAfterMs is an additional delay in milliseconds after challenge
	// clearance, before executing the fetch.
	WaitAfterMs int `json:"waitAfterMs,omitempty"`
}

// EffectiveMethod returns the resolved HTTP method, defaulting to GET.
func (r *V1Request) EffectiveMethod() string {
	if r.Method == "" {
		return "GET"
	}
	return r.Method
}

// EffectiveTimeout returns the timeout duration in milliseconds, defaulting to 60000.
func (r *V1Request) EffectiveTimeout() int {
	if r.MaxTimeout <= 0 {
		return 60000
	}
	return r.MaxTimeout
}

// ProxyConfig defines proxy connection settings.
// Authentication is handled via CDP Fetch domain interception.
type ProxyConfig struct {
	// URL is the proxy URL. Format: "http://ip:port" or "socks5://ip:port"
	URL string `json:"url"`

	// Username for proxy authentication. Optional.
	Username string `json:"username,omitempty"`

	// Password for proxy authentication. Optional.
	Password string `json:"password,omitempty"`
}

// Cookie represents a browser cookie for injection or extraction.
type Cookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path,omitempty"`
	Expires  float64 `json:"expires,omitempty"`
	HTTPOnly bool    `json:"httpOnly,omitempty"`
	Secure   bool    `json:"secure,omitempty"`
	SameSite string  `json:"sameSite,omitempty"` // "Strict", "Lax", "None"
}

// SessionCreateRequest is the input for POST /v1/sessions.
type SessionCreateRequest struct {
	// Session is an optional user-defined session ID.
	// If empty, a UUID is generated.
	Session string `json:"session,omitempty"`

	// TTL is the session time-to-live in seconds based on last use.
	// 0 means no auto-expiry (manual destroy only).
	TTL int `json:"ttl,omitempty"`

	// Proxy configures the proxy for this session's browser.
	Proxy *ProxyConfig `json:"proxy,omitempty"`
}
