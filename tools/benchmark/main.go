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

	"github.com/elazarl/goproxy"
	"github.com/elazarl/goproxy/ext/auth"
)

func main() {
	// 1. Start a proxy with NO auth on 8080
	proxyNoAuth := goproxy.NewProxyHttpServer()
	proxyNoAuth.Verbose = false
	go func() {
		log.Println("Starting No-Auth Proxy on :8080")
		http.ListenAndServe(":8080", proxyNoAuth)
	}()

	// 2. Start a proxy WITH auth on 8081 (user:pass)
	proxyWithAuth := goproxy.NewProxyHttpServer()
	proxyWithAuth.Verbose = false
	auth.ProxyBasic(proxyWithAuth, "my_realm", func(user, passwd string) bool {
		return user == "admin" && passwd == "secret123"
	})
	go func() {
		log.Println("Starting Auth Proxy on :8081 (admin:secret123)")
		http.ListenAndServe(":8081", proxyWithAuth)
	}()

	time.Sleep(1 * time.Second)

	targetURLs := []string{
		"https://api.ipify.org?format=json",
		"https://nowsecure.nl/",
	}

	scenarios := []struct {
		Name  string
		Proxy map[string]interface{}
	}{
		{
			Name:  "1. Direct (No Proxy)",
			Proxy: nil,
		},
		{
			Name: "2. Proxy (No Auth)",
			Proxy: map[string]interface{}{
				"url": "http://host.docker.internal:8080",
			},
		},
		{
			Name: "3. Proxy (With Auth)",
			Proxy: map[string]interface{}{
				"url":      "http://host.docker.internal:8081",
				"username": "admin",
				"password": "secret123",
			},
		},
	}

	client := &http.Client{Timeout: 30 * time.Second}
	stealthAPI := "http://127.0.0.1:8191/v2/request"

	for _, target := range targetURLs {
		fmt.Println("========================================")
		fmt.Println("Target:", target)
		fmt.Println("========================================")

		for _, sc := range scenarios {
			fmt.Printf("Scenario: %s\n", sc.Name)

			payload := map[string]interface{}{
				"url":          target,
				"disableMedia": true,
			}
			if sc.Proxy != nil {
				payload["proxy"] = sc.Proxy
			}

			bodyBytes, _ := json.Marshal(payload)

			// Run 3 iterations to warm up session/CDP and get average
			var totalDur time.Duration
			iterations := 3
			var lastIP string
			var lastStatus string

			for i := 0; i < iterations; i++ {
				req, _ := http.NewRequest("POST", stealthAPI, bytes.NewBuffer(bodyBytes))
				req.Header.Set("Content-Type", "application/json")

				start := time.Now()
				resp, err := client.Do(req)
				dur := time.Since(start)

				if err != nil {
					fmt.Printf("  [Iter %d] Error: %v\n", i+1, err)
					continue
				}

				respBody, _ := io.ReadAll(resp.Body)
				resp.Body.Close()

				totalDur += dur

				var res map[string]interface{}
				json.Unmarshal(respBody, &res)

				status := fmt.Sprintf("%v", res["status"])
				if status == "ok" {
					sol := res["solution"].(map[string]interface{})
					resData := fmt.Sprintf("%v", sol["response"])

					// Grab a snippet of the response or check for clearance cookie
					cookies := sol["cookies"].([]interface{})
					hasClearance := false
					for _, c := range cookies {
						cmap := c.(map[string]interface{})
						if cmap["name"] == "cf_clearance" {
							hasClearance = true
						}
					}

					if hasClearance {
						lastIP = fmt.Sprintf("HTML Length: %d, cf_clearance: true", len(resData))
					} else {
						lastIP = fmt.Sprintf("HTML Length: %d, cf_clearance: false", len(resData))
					}
					lastStatus = "ok"
				} else {
					lastStatus = fmt.Sprintf("error: %v", res["message"])
				}
				fmt.Printf("  [Iter %d] %v - %s\n", i+1, dur.Round(time.Millisecond), lastStatus)
			}

			avgDur := totalDur / time.Duration(iterations)
			fmt.Printf("  -> Avg Latency: %v\n", avgDur.Round(time.Millisecond))
			fmt.Printf("  -> IP Returned: %s\n\n", lastIP)
		}
	}

	os.Exit(0)
}
