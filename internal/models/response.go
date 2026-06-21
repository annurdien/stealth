package models

// V1Response is the unified response format for all /v1/ endpoints.
// Errors use status="error" with solution=nil.
type V1Response struct {
	// Status is "ok" or "error".
	Status string `json:"status"`

	// Message is a human-readable description.
	Message string `json:"message"`

	// StartTimestamp is Unix epoch milliseconds when processing began.
	StartTimestamp int64 `json:"startTimestamp"`

	// EndTimestamp is Unix epoch milliseconds when processing finished.
	EndTimestamp int64 `json:"endTimestamp"`

	// Version is the Stealth version string.
	Version string `json:"version"`

	// Solution contains the result data. Nil on errors.
	Solution *Solution `json:"solution"`
}

// Solution holds the resolved data from a successful request.
type Solution struct {
	// URL is the final URL after any redirects.
	URL string `json:"url"`

	// Status is the HTTP status code from the fetch() response.
	// 0 when ReturnOnlyCookies is true.
	Status int `json:"status"`

	// Response is the raw response body from fetch().
	// Empty when ReturnOnlyCookies is true.
	Response string `json:"response"`

	// Cookies are all cookies in the browser context after solving.
	Cookies []Cookie `json:"cookies"`

	// UserAgent is the User-Agent string used by the browser.
	UserAgent string `json:"userAgent"`

	// Screenshot is a base64-encoded PNG. Only set when ReturnScreenshot is true.
	Screenshot string `json:"screenshot,omitempty"`
}

// SessionResponse is returned by session management endpoints.
type SessionResponse struct {
	Status   string   `json:"status"`
	Message  string   `json:"message"`
	Session  string   `json:"session,omitempty"`  // For create
	Sessions []string `json:"sessions,omitempty"` // For list
}

// IndexResponse is returned by GET /.
type IndexResponse struct {
	Msg       string `json:"msg"`
	Version   string `json:"version"`
	UserAgent string `json:"userAgent"`
}

// HealthResponse is returned by GET /health.
type HealthResponse struct {
	Status string `json:"status"`
}
