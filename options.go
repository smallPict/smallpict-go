package smallpict

import (
	"net/http"
	"strings"
	"time"
)

// Option is a functional option for configuring a SmallPict Client.
type Option func(*Client)

// WithAPIKey sets the customer API key (e.g. sp_sdk_..., sp_test_...).
func WithAPIKey(apiKey string) Option {
	return func(c *Client) {
		c.apiKey = apiKey
	}
}

// WithSecretKey sets the customer secret key used for HMAC-SHA256 request signing.
func WithSecretKey(secretKey string) Option {
	return func(c *Client) {
		c.secretKey = secretKey
	}
}

// WithBaseURL sets the SmallPict API base endpoint (default: https://api.smallpict.app).
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithTimeout sets the HTTP client request timeout (default: 30s).
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.timeout = timeout
		if c.httpClient != nil {
			c.httpClient.Timeout = timeout
		}
	}
}

// WithMaxRetries sets the maximum number of retry attempts on 429 and 5xx errors (default: 3).
func WithMaxRetries(retries int) Option {
	return func(c *Client) {
		c.maxRetries = retries
	}
}

// WithFallbackMode sets fallback behavior on quota exhaustion (FallbackThrow or FallbackPassthrough).
func WithFallbackMode(mode string) Option {
	return func(c *Client) {
		c.fallbackMode = mode
	}
}

// WithHTTPClient provides a custom standard library *http.Client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithPollInterval sets how often Optimize polls the job status while it waits (default: 1s).
func WithPollInterval(interval time.Duration) Option {
	return func(c *Client) {
		c.pollInterval = interval
	}
}

// WithWaitTimeout sets how long Optimize waits for the job to finish (default: 120s).
// After it, Optimize returns a TIMEOUT_ERROR; the job keeps running on the server.
func WithWaitTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.waitTimeout = timeout
	}
}
