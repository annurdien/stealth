package solver

import (
	"fmt"
	"os"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	rodstealth "github.com/go-rod/stealth"

	"github.com/annurdien/stealth/internal/models"
)

// systemChromiumPaths lists candidate locations for a system-installed Chromium.
// Rod will use the first one it finds, or fall back to downloading if none exist.
var systemChromiumPaths = []string{
	"/usr/bin/chromium",         // Debian/Ubuntu apt package
	"/usr/bin/chromium-browser", // Some Debian variants
	"/usr/bin/google-chrome",    // Google Chrome
	"/usr/bin/google-chrome-stable",
}

// findChromiumBin returns the path to the Chromium binary to use.
// Priority: CHROME_BIN env var → system paths → empty (Rod downloads).
func findChromiumBin() string {
	// Allow explicit override via environment
	if bin := os.Getenv("CHROME_BIN"); bin != "" {
		return bin
	}
	// Walk known system paths
	for _, path := range systemChromiumPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return "" // Rod will attempt to download
}

// LaunchBrowser creates a new headless Chrome instance with stealth flags.
// If a proxy is provided, it configures the proxy-server flag.
func LaunchBrowser(proxy *models.ProxyConfig) (*rod.Browser, error) {
	l := launcher.New().
		// Prevent Chrome from detecting automation via common flags
		Set("disable-blink-features", "AutomationControlled").
		Set("no-sandbox").
		Set("disable-setuid-sandbox").
		Set("disable-dev-shm-usage").
		Set("no-zygote").
		Set("window-size", "1920,1080").
		Set("ignore-certificate-errors").
		Set("ignore-ssl-errors").
		Set("disable-features", "LocalNetworkAccessChecks").
		Set("disable-search-engine-choice-screen")

	headless := os.Getenv("HEADLESS") != "false"
	l = l.Headless(headless)

	// Point Rod at the system Chromium binary so it doesn't try to download one.
	if bin := findChromiumBin(); bin != "" {
		l = l.Bin(bin)
	}

	if proxy != nil && proxy.URL != "" {
		l = l.Set("proxy-server", proxy.URL)
	}

	controlURL, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to browser: %w", err)
	}

	return browser, nil
}

// CreateStealthPage creates a new browser page with go-rod/stealth patches applied.
// These patches mask navigator.webdriver, chrome.runtime, permissions, etc.
func CreateStealthPage(browser *rod.Browser) (*rod.Page, error) {
	page, err := rodstealth.Page(browser)
	if err != nil {
		return nil, fmt.Errorf("failed to create stealth page: %w", err)
	}
	return page, nil
}

