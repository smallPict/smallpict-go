package smallpict

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestOptimizeFolder(t *testing.T) {
	cases := []struct {
		name       string
		folder     string
		wantWire   string // "" means the folder key is absent
		respFolder interface{}
		wantResult string
	}{
		{name: "unset is omitted"},
		{name: "sent and surfaced", folder: "Products/Shoes", wantWire: "Products/Shoes", respFolder: "products/shoes", wantResult: "products/shoes"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status := map[string]interface{}{"job_id": "job_1", "status": "completed", "url": "https://cdn.test/f.webp", "format": "webp", "bytes_saved": 1}
			if tc.respFolder != nil {
				status["folder"] = tc.respFolder
			}
			f := &fakeFlow{t: t, statuses: []map[string]interface{}{status}}

			res, err := f.client().Optimize(context.Background(), bytes.NewBufferString("img"), &OptimizeOptions{Folder: tc.folder})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			sent := f.options
			got, present := sent["folder"]
			if tc.wantWire == "" && present {
				t.Errorf("folder must be omitted, got %v", got)
			}
			if tc.wantWire != "" && got != tc.wantWire {
				t.Errorf("options.folder = %v, want %q", got, tc.wantWire)
			}
			if res.Folder != tc.wantResult {
				t.Errorf("result Folder = %q, want %q", res.Folder, tc.wantResult)
			}
		})
	}
}

func TestJobStatusFolder(t *testing.T) {
	mock := newMockHTTPClient(func(r *http.Request) *http.Response {
		return jsonResponse(http.StatusOK, map[string]interface{}{"job_id": "j1", "status": "succeeded", "folder": "blog/2026"})
	})
	client, _ := NewClient(WithSecretKey("sk_test_secret"), WithAPIKey("sp_sdk_test"), WithHTTPClient(mock))

	res, err := client.GetJobStatus(context.Background(), "j1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Folder != "blog/2026" {
		t.Errorf("Folder = %q, want blog/2026", res.Folder)
	}
}

func TestPurgeFolderBody(t *testing.T) {
	var body map[string]interface{}
	mock := newMockHTTPClient(func(r *http.Request) *http.Response {
		if r.URL.Path != "/v1/purge" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		return jsonResponse(http.StatusAccepted, map[string]interface{}{"message": "Purge job enqueued successfully"})
	})
	client, _ := NewClient(WithSecretKey("sk_test_secret"), WithAPIKey("sp_sdk_test"), WithHTTPClient(mock))

	if _, err := client.PurgeFolder(context.Background(), "products/shoes"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]interface{}{"purge_type": "folder", "folder": "products/shoes"}
	if len(body) != len(want) {
		t.Fatalf("body = %v, want exactly %v", body, want)
	}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("body[%q] = %v, want %v", k, body[k], v)
		}
	}
}

func TestPurgeURLBodyHasNoFolder(t *testing.T) {
	var body map[string]interface{}
	mock := newMockHTTPClient(func(r *http.Request) *http.Response {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		return jsonResponse(http.StatusAccepted, map[string]interface{}{"message": "ok"})
	})
	client, _ := NewClient(WithSecretKey("sk_test_secret"), WithAPIKey("sp_sdk_test"), WithHTTPClient(mock))

	if _, err := client.PurgeCDN(context.Background(), []string{"https://cdn.smallpict.app/c/j.avif"}, PurgeTypeURL); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := body["folder"]; ok {
		t.Errorf("folder must not be sent for url purges: %v", body)
	}
}

func TestFolderClientSideRejects(t *testing.T) {
	folders := []string{"", "a//b", "/a", "a/", "../x", "a/../b", "a/b/c/d"}

	for _, folder := range folders {
		t.Run("purge "+folder, func(t *testing.T) {
			calls := 0
			mock := newMockHTTPClient(func(r *http.Request) *http.Response {
				calls++
				return jsonResponse(http.StatusAccepted, map[string]interface{}{})
			})
			client, _ := NewClient(WithSecretKey("sk_test_secret"), WithAPIKey("sp_sdk_test"), WithHTTPClient(mock))

			_, err := client.PurgeFolder(context.Background(), folder)
			if !IsValidationError(err) {
				t.Fatalf("expected validation error, got %v", err)
			}
			if calls != 0 {
				t.Fatalf("expected no HTTP call, got %d", calls)
			}
		})
		if folder == "" {
			continue // empty Folder means unset for Optimize
		}
		t.Run("optimize "+folder, func(t *testing.T) {
			calls := 0
			mock := newMockHTTPClient(func(r *http.Request) *http.Response {
				calls++
				return jsonResponse(http.StatusOK, map[string]interface{}{})
			})
			client, _ := NewClient(WithSecretKey("sk_test_secret"), WithAPIKey("sp_sdk_test"), WithHTTPClient(mock))

			_, err := client.Optimize(context.Background(), bytes.NewBufferString("img"), &OptimizeOptions{Folder: folder})
			if !IsValidationError(err) {
				t.Fatalf("expected validation error, got %v", err)
			}
			if calls != 0 {
				t.Fatalf("expected no HTTP call, got %d", calls)
			}
		})
	}
}

func TestPurgeCDNFolderTypeNeedsFolder(t *testing.T) {
	calls := 0
	mock := newMockHTTPClient(func(r *http.Request) *http.Response {
		calls++
		return jsonResponse(http.StatusAccepted, map[string]interface{}{})
	})
	client, _ := NewClient(WithSecretKey("sk_test_secret"), WithAPIKey("sp_sdk_test"), WithHTTPClient(mock))

	_, err := client.PurgeCDN(context.Background(), nil, PurgeTypeFolder)
	if !IsValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected no HTTP call, got %d", calls)
	}
}

func TestFolderServerErrors(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		code      string
		message   string
		check     func(error) bool
		wantCalls int
	}{
		{name: "400 invalid_folder", status: http.StatusBadRequest, code: "invalid_folder", message: "folder segment too long", check: IsValidationError, wantCalls: 1},
		{name: "429 purge_rate_limited is not retried", status: http.StatusTooManyRequests, code: "purge_rate_limited", message: "Too many folder purges this hour", check: IsRateLimit, wantCalls: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			mock := newMockHTTPClient(func(r *http.Request) *http.Response {
				calls++
				return jsonResponse(tc.status, map[string]interface{}{
					"success": false,
					"error":   map[string]interface{}{"code": tc.code, "message": tc.message},
				})
			})
			client, _ := NewClient(WithSecretKey("sk_test_secret"), WithAPIKey("sp_sdk_test"), WithHTTPClient(mock), WithMaxRetries(3))

			_, err := client.PurgeFolder(context.Background(), "products")
			if !tc.check(err) {
				t.Fatalf("wrong error class: %v", err)
			}
			e, _ := err.(*Error)
			if e == nil || e.Code != tc.code || e.Message != tc.message {
				t.Errorf("error = %+v, want code %q message %q", e, tc.code, tc.message)
			}
			if calls != tc.wantCalls {
				t.Errorf("HTTP calls = %d, want %d", calls, tc.wantCalls)
			}
		})
	}
}

func TestOtherRateLimitStillRetried(t *testing.T) {
	calls := 0
	mock := newMockHTTPClient(func(r *http.Request) *http.Response {
		calls++
		if calls == 1 {
			return jsonResponse(http.StatusTooManyRequests, map[string]interface{}{
				"error": map[string]interface{}{"code": "RATE_LIMIT_EXCEEDED", "message": "slow down"},
			})
		}
		return jsonResponse(http.StatusAccepted, map[string]interface{}{"message": "ok"})
	})
	client, _ := NewClient(WithSecretKey("sk_test_secret"), WithAPIKey("sp_sdk_test"), WithHTTPClient(mock), WithMaxRetries(1))

	if _, err := client.PurgeFolder(context.Background(), "products"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Errorf("HTTP calls = %d, want 2 (one retry)", calls)
	}
}
