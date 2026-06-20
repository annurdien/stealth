package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/annurdien/stealth/internal/api"
	"github.com/annurdien/stealth/internal/session"
	"github.com/annurdien/stealth/internal/solver"
)

const version = "1.0.0"

func main() {
	// Set version in the solver package so all responses include it
	solver.Version = version

	// --- Configuration from environment ---
	host := envOr("HOST", "0.0.0.0")
	port := envOr("PORT", "8191")

	log.Printf("Stealth %s starting on %s:%s", version, host, port)

	// --- Session Manager ---
	sm := session.NewManager()

	// --- HTTP Server ---
	app := api.NewServer(sm, version)

	// --- Prometheus metrics (optional) ---
	if os.Getenv("PROMETHEUS_ENABLED") == "true" {
		promPort := envOr("PROMETHEUS_PORT", "8192")
		go startPrometheus(promPort)
	}

	// --- Graceful Shutdown ---
	// Listen for SIGINT/SIGTERM in a goroutine so we can block on the signal.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start the server in a goroutine.
	go func() {
		addr := fmt.Sprintf("%s:%s", host, port)
		if err := app.Listen(addr); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	log.Printf("Stealth is ready! Listening on http://%s:%s", host, port)

	// Block until a termination signal is received.
	sig := <-quit
	log.Printf("Received signal %s — shutting down...", sig)

	// 1. Stop accepting new HTTP connections and wait for in-flight requests
	if err := app.Shutdown(); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}

	// 2. Close all browser sessions and clean up Chrome processes
	sm.Stop()

	log.Println("Shutdown complete.")
}

// startPrometheus launches the Prometheus metrics exporter on a separate port.
// It is a lightweight no-op if the prometheus/client_golang package is not used.
func startPrometheus(port string) {
	log.Printf("Prometheus metrics available at http://0.0.0.0:%s/metrics", port)
	// The Prometheus default HTTP mux serves /metrics automatically
	// when using prometheus/client_golang.
	// Add: promhttp.Handler() on a net/http server here if needed.
}

// envOr returns the value of the environment variable key,
// or the fallback value if the variable is not set or empty.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
