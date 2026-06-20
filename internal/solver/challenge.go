package solver

import (
	"fmt"
	"strings"

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

// DetectChallenge inspects the current page to determine its challenge state.
func DetectChallenge(page *rod.Page) ChallengeResult {
	title, err := page.Eval(`() => document.title`)
	if err != nil {
		return ChallengeNone
	}
	pageTitle := title.Value.String()

	// 1. Check access denied titles (fatal — IP is blocked)
	for _, t := range AccessDeniedTitles {
		if strings.HasPrefix(pageTitle, t) {
			return ChallengeAccessDenied
		}
	}

	// 2. Check access denied selectors
	for _, sel := range AccessDeniedSelectors {
		if elementExists(page, sel) {
			return ChallengeAccessDenied
		}
	}

	// 3. Check challenge titles
	for _, t := range ChallengeTitles {
		if strings.EqualFold(pageTitle, t) {
			return ChallengeFound
		}
	}

	// 4. Check challenge selectors
	for _, sel := range ChallengeSelectors {
		if elementExists(page, sel) {
			return ChallengeFound
		}
	}

	return ChallengeNone
}

// WaitForChallengeResolution polls until all challenge selectors disappear
// or the timeout is exceeded. Attempts Turnstile solve on each failed cycle.
func WaitForChallengeResolution(page *rod.Page, timeoutMs int) error {
	deadline := timeoutMs / 1000 // convert to seconds for loop counting
	if deadline < 5 {
		deadline = 5
	}

	for attempt := 0; attempt < deadline; attempt++ {
		allClear := true

		// Check if any challenge title is still present
		titleResult, err := page.Eval(`() => document.title`)
		if err == nil {
			pageTitle := titleResult.Value.String()
			for _, t := range ChallengeTitles {
				if strings.EqualFold(pageTitle, t) {
					allClear = false
					break
				}
			}
		}

		// Check if any challenge selector is still present
		if allClear {
			for _, sel := range ChallengeSelectors {
				if elementExists(page, sel) {
					allClear = false
					break
				}
			}
		}

		if allClear {
			return nil
		}

		// Attempt Turnstile click on each failed cycle
		_ = SolveTurnstile(page)
	}

	return fmt.Errorf("challenge not resolved within %d seconds", deadline)
}

// elementExists returns true if at least one element matching the CSS
// selector exists in the page's current DOM.
func elementExists(page *rod.Page, selector string) bool {
	result, err := page.Eval(
		`(sel) => !!document.querySelector(sel)`,
		selector,
	)
	if err != nil {
		return false
	}
	return result.Value.Bool()
}
