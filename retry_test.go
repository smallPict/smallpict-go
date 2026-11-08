package smallpict

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func unavailable(retryAfter string) roundTripFunc {
	return func(r *http.Request) *http.Response {
		h := http.Header{"Content-Type": []string{"application/json"}}
		if retryAfter != "" {
			h.Set("Retry-After", retryAfter)
		}
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: h,
			Body: io.NopCloser(strings.NewReader(`{"error":{"code":"INTERNAL_ERROR","message":"down"}}`))}
	}
}

// A cancelled context must end a request that is waiting between retries, not after the wait.
func TestRetryBackoffStopsWhenContextEnds(t *testing.T) {
	c, err := NewClient(WithSecretKey("sk_test_secret"), WithAPIKey("sp_sdk_test"), WithMaxRetries(3),
		WithHTTPClient(newMockHTTPClient(unavailable(""))))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = c.GetQuota(ctx)
	if elapsed := time.Since(start); elapsed > 150*time.Millisecond {
		t.Fatalf("returned after %s; the 250 ms backoff ignored the cancelled context", elapsed)
	}
	if !IsTimeout(err) {
		t.Fatalf("want a TIMEOUT_ERROR, got %v", err)
	}
}

// One Retry-After: 3600 must not hang the caller for an hour.
func TestRetryAfterIsCapped(t *testing.T) {
	prev := maxRetryAfter
	maxRetryAfter = 20 * time.Millisecond
	defer func() { maxRetryAfter = prev }()
	c, err := NewClient(WithSecretKey("sk_test_secret"), WithAPIKey("sp_sdk_test"), WithMaxRetries(1),
		WithHTTPClient(newMockHTTPClient(unavailable("3600"))))
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { _, err := c.GetQuota(context.Background()); done <- err }()
	select {
	case err := <-done:
		if !IsServer(err) {
			t.Fatalf("want the 503 after the capped wait, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Retry-After: 3600 was not capped")
	}
}
