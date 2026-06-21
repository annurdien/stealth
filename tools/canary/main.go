package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

var targets = []string{
	"https://nowsecure.nl/",
	"https://www.cloudflare.com/",
	"https://api.ipify.org?format=json",
	"https://chatgpt.com/",
}

func main() {
	fmt.Println("========================================")
	fmt.Println("       Stealth E2E Canary Test          ")
	fmt.Println("========================================")

	stealthAPI := "http://127.0.0.1:8191/v1/request"
	client := &http.Client{Timeout: 90 * time.Second}

	successCount := 0
	totalTargets := len(targets)

	for _, target := range targets {
		fmt.Printf("Testing: %s\n", target)

		payload := map[string]interface{}{
			"url":          target,
			"disableMedia": true,
			"maxTimeout":   60000,
		}

		bodyBytes, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", stealthAPI, bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		start := time.Now()
		resp, err := client.Do(req)
		dur := time.Since(start).Round(time.Millisecond)

		if err != nil {
			fmt.Printf("  ❌ FAILED: HTTP Error: %v\n\n", err)
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var res map[string]interface{}
		if err := json.Unmarshal(respBody, &res); err != nil {
			fmt.Printf("  ❌ FAILED: Invalid JSON Response: %v\n\n", err)
			continue
		}

		status := fmt.Sprintf("%v", res["status"])
		if status == "ok" {
			fmt.Printf("  ✅ SUCCESS in %v\n\n", dur)
			successCount++
		} else {
			fmt.Printf("  ❌ FAILED in %v: %v\n\n", dur, res["message"])
		}
	}

	fmt.Println("========================================")
	fmt.Printf("Results: %d/%d passed\n", successCount, totalTargets)

	// We require a 75% success rate to pass the canary test, as some external
	// sites may block data-center IPs regardless of Turnstile clearance.
	successRate := float64(successCount) / float64(totalTargets)
	if successRate < 0.75 {
		log.Fatalf("🚨 CANARY FAILED! Success rate %.2f%% is below 75%% threshold.", successRate*100)
	}

	fmt.Println("🎉 Canary test passed successfully!")
	os.Exit(0)
}
