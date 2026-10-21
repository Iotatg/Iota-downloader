// Package httpx provides the shared HTTP helpers of the bot: browser like
// headers, a reusable client and small download helpers.
package httpx

import (
	"math/rand"
	"net/http"
	"time"
)

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Safari/605.1.15",
	"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Mobile/15E148 Safari/604.1",
}

var acceptLanguages = []string{"en-US,en;q=0.9", "en-GB,en;q=0.9", "de-DE,de;q=0.9,en;q=0.8"}

// RandomUserAgent returns a random desktop or mobile browser user agent.
func RandomUserAgent() string {
	return userAgents[rand.Intn(len(userAgents))]
}

// BrowserHeaders returns a header set that looks like a real browser.
func BrowserHeaders() map[string]string {
	return map[string]string{
		"User-Agent":      RandomUserAgent(),
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		"Accept-Language": acceptLanguages[rand.Intn(len(acceptLanguages))],
		"Accept-Encoding": "gzip, deflate, br",
		"Sec-Fetch-Dest":  "document",
		"Sec-Fetch-Mode":  "navigate",
		"Sec-Fetch-Site":  "none",
		"Sec-Fetch-User":  "?1",
	}
}

// MergeHeaders merges the custom headers on top of the browser headers.
func MergeHeaders(custom map[string]string) map[string]string {
	merged := BrowserHeaders()
	for key, value := range custom {
		merged[key] = value
	}
	return merged
}

// NewClient builds an HTTP client with sane timeouts.
func NewClient(timeout time.Duration, maxConnections int) *http.Client {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	if maxConnections <= 0 {
		maxConnections = 64
	}
	transport := &http.Transport{
		MaxIdleConns:        maxConnections,
		MaxIdleConnsPerHost: maxConnections / 2,
		MaxConnsPerHost:     maxConnections,
		IdleConnTimeout:     90 * time.Second,
	}
	return &http.Client{Transport: transport, Timeout: timeout}
}
