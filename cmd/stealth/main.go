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
	solver.Version = version

	host := envOr("HOST", "0.0.0.0")
	port := envOr("PORT", "8191")

	log.Printf("Stealth %s starting on %s:%s", version, host, port)

	sm := session.NewManager()

	app := api.NewServer(sm, version)

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

	sig := <-quit
	log.Printf("Received signal %s — shutting down...", sig)

	if err := app.Shutdown(); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}

	sm.Stop()

	log.Println("Shutdown complete.")
}

// envOr returns the value of the environment variable key,
// or the fallback value if the variable is not set or empty.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
