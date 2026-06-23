package solver

import (
	"fmt"
	"time"

	"github.com/go-rod/rod"
)

// ChallengeResult classifies the state of the page.
type ChallengeResult int

const (
	// ChallengeNone means the page loaded normally — no challenge.
	ChallengeNone ChallengeResult = iota
	// ChallengeFound means an active, solvable challenge was detected.
	ChallengeFound
	// ChallengeAccessDenied means the IP is blocked — abort immediately.
	ChallengeAccessDenied
)

// AccessDeniedTitles triggers an immediate error — the IP is blocked.
var AccessDeniedTitles = []string{
	"Access denied",
	"Attention Required! | Cloudflare",
}

// AccessDeniedSelectors — same purpose, DOM-based detection.
var AccessDeniedSelectors = []string{
	"div.cf-error-title span.cf-code-label span",
	"#cf-error-details div.cf-error-overview h1",
}

// ChallengeTitles indicates an active challenge page.
var ChallengeTitles = []string{
	"Just a moment...",
	"DDoS-Guard",
}

// ChallengeSelectors indicates challenge elements present in the DOM.
// Ported from FlareSolverr's battle-tested list.
var ChallengeSelectors = []string{
	"#cf-challenge-running",
	".ray_id",
	".attack-box",
	"#cf-please-wait",
	"#challenge-spinner",
	"#trk_jschal_js",
	"#turnstile-wrapper",
	".lds-ring",
	// Custom CloudFlare for EbookParadijs, Film-Paleis, etc.
	"td.info #js_info",
	// Fairlane / pararius.com
	"div.vc div.text-box h2",
}

// MediaBlockPatterns used when DisableMedia is true.
var MediaBlockPatterns = []string{
	"*.png", "*.jpg", "*.jpeg", "*.gif", "*.webp", "*.svg", "*.ico",
	"*.bmp", "*.tiff", "*.avif", "*.heic", "*.apng",
	"*.PNG", "*.JPG", "*.JPEG", "*.GIF", "*.WEBP", "*.SVG", "*.ICO",
	"*.css", "*.CSS",
	"*.woff", "*.woff2", "*.ttf", "*.otf", "*.eot",
	"*.WOFF", "*.WOFF2", "*.TTF", "*.OTF", "*.EOT",
}

const detectChallengeScript = `() => {
	const title = document.title || '';
	const accessDeniedTitles = ["Access denied", "Attention Required! | Cloudflare"];
	for (const t of accessDeniedTitles) {
		if (title.startsWith(t)) return 2;
	}
	const accessDeniedSelectors = ["div.cf-error-title span.cf-code-label span", "#cf-error-details div.cf-error-overview h1"];
	for (const sel of accessDeniedSelectors) {
		if (document.querySelector(sel)) return 2;
	}
	const challengeTitles = ["Just a moment...", "DDoS-Guard"];
	for (const t of challengeTitles) {
		if (title.toLowerCase() === t.toLowerCase()) return 1;
	}
	const challengeSelectors = [
		"#cf-challenge-running", ".ray_id", ".attack-box", "#cf-please-wait",
		"#challenge-spinner", "#trk_jschal_js", "#turnstile-wrapper", ".lds-ring",
		"td.info #js_info", "div.vc div.text-box h2"
	];
	for (const sel of challengeSelectors) {
		if (document.querySelector(sel)) return 1;
	}
	return 0;
}`

// DetectChallenge inspects the current page to determine its challenge state
// using a single batched JavaScript evaluation to minimize CDP overhead.
func DetectChallenge(page *rod.Page) ChallengeResult {
	res, err := page.Eval(detectChallengeScript)
	if err != nil {
		return ChallengeNone
	}

	switch res.Value.Int() {
	case 1:
		return ChallengeFound
	case 2:
		return ChallengeAccessDenied
	default:
		return ChallengeNone
	}
}

// WaitForChallengeResolution polls until all challenge selectors disappear
// or the timeout is exceeded. Attempts Turnstile solve on each failed cycle.
func WaitForChallengeResolution(page *rod.Page, timeoutMs int) error {
	deadline := timeoutMs / 1000 // convert to seconds for loop counting
	if deadline < 5 {
		deadline = 5
	}

	for attempt := 0; attempt < deadline; attempt++ {
		if DetectChallenge(page) == ChallengeNone {
			return nil
		}

		start := time.Now()
		_ = SolveTurnstile(page)

		// If SolveTurnstile returns immediately (because it didn't find the iframe),
		// we should sleep to avoid a tight busy-wait loop that burns CPU.
		if time.Since(start) < 200*time.Millisecond {
			time.Sleep(1 * time.Second)
		}
	}

	return fmt.Errorf("challenge not resolved within %d seconds", deadline)
}
