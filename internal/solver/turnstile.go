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
	// Search for Turnstile iframes using multiple known selectors
	el, err := page.Timeout(2 * time.Second).Element(
		`iframe[src*="challenges.cloudflare.com"], #turnstile-wrapper iframe`,
	)
	if err != nil {
		// No Turnstile found — not an error
		return nil
	}

	// Wait for the iframe to be visible and geometrically stable
	if err := el.WaitStable(500 * time.Millisecond); err != nil {
		return fmt.Errorf("turnstile iframe not stable: %w", err)
	}

	// Get the iframe's bounding box from the parent page.
	// This works because we're reading the outer element, not cross-origin content.
	shape, err := el.Shape()
	if err != nil {
		return fmt.Errorf("failed to get turnstile iframe shape: %w", err)
	}

	// shape.Box() gives us the iframe's top-left coordinates and dimensions.
	box := shape.Box()

	// Calculate the absolute page coordinates for the checkbox.
	clickX := box.X + TurnstileCheckboxOffsetX
	clickY := box.Y + TurnstileCheckboxOffsetY

	// Move in 10 linear steps to simulate a human-like glide to the target.
	mouse := page.Mouse
	targetPoint := proto.Point{X: clickX, Y: clickY}
	if err := mouse.MoveLinear(targetPoint, 10); err != nil {
		return fmt.Errorf("mouse move to turnstile failed: %w", err)
	}

	// Brief pause before clicking, mimicking human reaction time.
	time.Sleep(100 * time.Millisecond)

	// Perform the left-click.
	if err := mouse.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("mouse click on turnstile failed: %w", err)
	}

	// Wait for Cloudflare to process the interaction.
	time.Sleep(2 * time.Second)

	return nil
}
