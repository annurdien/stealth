package solver

import (
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// TurnstileCheckboxOffset is the approximate position of the Cloudflare
// Turnstile checkbox within its iframe, from the iframe's top-left corner.
// The cross-origin iframe prevents DOM access, so we click by coordinates
// relative to the outer iframe element's bounding box.
const (
	TurnstileCheckboxOffsetX = 22.0
	TurnstileCheckboxOffsetY = 22.0
)

// SolveTurnstile attempts to locate the Cloudflare Turnstile iframe and
// click the checkbox by calculating its absolute page coordinates.
//
// Strategy:
//  1. Locate the iframe element from the parent page (cross-origin-safe)
//  2. Get the iframe's bounding box via CDP Shape (works for outer element)
//  3. Click at iframe.TopLeft + checkboxOffset to hit the checkbox
//
// Returns nil if no Turnstile is found — not an error, just means no captcha
// is present on this page.
func SolveTurnstile(page *rod.Page) error {
	el, err := page.Timeout(2 * time.Second).Element(
		`iframe[src*="challenges.cloudflare.com"], #turnstile-wrapper iframe`,
	)
	if err != nil {
		return nil
	}

	if err := el.WaitStable(500 * time.Millisecond); err != nil {
		return fmt.Errorf("turnstile iframe not stable: %w", err)
	}

	shape, err := el.Shape()
	if err != nil {
		return fmt.Errorf("failed to get turnstile iframe shape: %w", err)
	}

	box := shape.Box()

	clickX := box.X + TurnstileCheckboxOffsetX
	clickY := box.Y + TurnstileCheckboxOffsetY

	mouse := page.Mouse
	targetPoint := proto.Point{X: clickX, Y: clickY}
	if err := mouse.MoveLinear(targetPoint, 10); err != nil {
		return fmt.Errorf("mouse move to turnstile failed: %w", err)
	}

	time.Sleep(100 * time.Millisecond)

	if err := mouse.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("mouse click on turnstile failed: %w", err)
	}

	time.Sleep(2 * time.Second)

	return nil
}
