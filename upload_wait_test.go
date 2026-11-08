package smallpict

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testUploadURL = "https://storage.test/uploads/job_1?X-Amz-Signature=abc"

// fakeFlow plays the API and the storage for one Optimize call:
// POST /v1/optimize, PUT to the upload URL, then GET /v1/optimize/status.
type fakeFlow struct {
	t         *testing.T
	queued    map[string]interface{}   // POST answer; nil means a queued job_1
	putStatus int                      // 0 means 200
	statuses  []map[string]interface{} // poll answers in order, the last one repeats; nil means completed
	options   map[string]interface{}   // options sent with the POST
	filenames []string                 // filename of each POST, in order
	post      *http.Request
	put       *http.Request
	putBody   []byte
	polls     int
	requests  int
}

func (f *fakeFlow) roundTrip(r *http.Request) *http.Response {
	f.requests++
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/optimize":
		f.post = r
		var body struct {
			Filename string                 `json:"filename"`
			Options  map[string]interface{} `json:"options"`
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		f.options = body.Options
		f.filenames = append(f.filenames, body.Filename)
		q := f.queued
		if q == nil {
			q = map[string]interface{}{"job_id": "job_1", "status": "queued", "upload_url": testUploadURL, "object_key": "in/job_1", "bucket": "uploads"}
		}
		return jsonResponse(http.StatusOK, q)
	case r.Method == http.MethodPut && r.URL.String() == testUploadURL:
		f.put = r
		if r.Body != nil {
			f.putBody, _ = io.ReadAll(r.Body)
		}
		status := f.putStatus
		if status == 0 {
			status = http.StatusOK
		}
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("<Error>AccessDenied</Error>"))}
	case r.Method == http.MethodGet && r.URL.Path == "/v1/optimize/status":
		statuses := f.statuses
		if statuses == nil {
			statuses = []map[string]interface{}{{"job_id": "job_1", "status": "completed", "url": "https://cdn.test/o.webp", "format": "webp", "bytes_saved": 1}}
		}
		i := f.polls
		if i >= len(statuses) {
			i = len(statuses) - 1
		}
		f.polls++
		return jsonResponse(http.StatusOK, statuses[i])
	}
	f.t.Errorf("unexpected request %s %s", r.Method, r.URL)
	return jsonResponse(http.StatusInternalServerError, map[string]interface{}{})
}

func (f *fakeFlow) client(opts ...Option) *Client {
	f.t.Helper()
	all := append([]Option{
		WithAPIKey("sp_sdk_test"),
		WithSecretKey("sec_test_secret_123"),
		WithHTTPClient(newMockHTTPClient(f.roundTrip)),
		WithPollInterval(time.Millisecond),
	}, opts...)
	c, err := NewClient(all...)
	if err != nil {
		f.t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestOptimizeUploadsAndWaits(t *testing.T) {
	quality := 62
	f := &fakeFlow{t: t, statuses: []map[string]interface{}{
		{"job_id": "job_1", "status": "processing"},
		{"job_id": "job_1", "status": "completed", "url": "https://cdn.test/o.avif", "format": "avif",
			"bytes_saved": 4, "mode_applied": "auto", "quality_applied": quality, "folder": "shop"},
	}}
	img := []byte("0123456789")

	res, err := f.client().OptimizeBytes(context.Background(), img, &OptimizeOptions{MIMEType: "image/png", Format: FormatWebP})
	if err != nil {
		t.Fatalf("Optimize: %v", err)
	}

	if f.put == nil {
		t.Fatal("no PUT to upload_url")
	}
	if got := f.put.Header.Get("Content-Type"); got != "image/png" {
		t.Errorf("PUT Content-Type = %q, want image/png", got)
	}
	if f.put.ContentLength != int64(len(img)) {
		t.Errorf("PUT Content-Length = %d, want %d", f.put.ContentLength, len(img))
	}
	for _, h := range []string{"X-API-Key", "X-Timestamp", "X-Signature", "Authorization", "Idempotency-Key"} {
		if v := f.put.Header.Get(h); v != "" {
			t.Errorf("PUT carries %s: %q", h, v)
		}
	}
	if !bytes.Equal(f.putBody, img) {
		t.Errorf("PUT body = %q, want %q", f.putBody, img)
	}
	if f.polls != 2 {
		t.Errorf("polls = %d, want 2", f.polls)
	}

	want := OptimizeResult{
		JobID: "job_1", Status: "completed", URL: "https://cdn.test/o.avif", Format: "avif",
		OriginalSize: 10, CompressedSize: 6, BytesSaved: 4, SavingsPercentage: 40,
		ModeApplied: "auto", Folder: "shop",
	}
	if res.QualityApplied == nil || *res.QualityApplied != quality {
		t.Errorf("QualityApplied = %v, want %d", res.QualityApplied, quality)
	}
	res.QualityApplied = nil
	if res.JobID != want.JobID || res.Status != want.Status || res.URL != want.URL || res.Format != want.Format ||
		res.OriginalSize != want.OriginalSize || res.CompressedSize != want.CompressedSize || res.BytesSaved != want.BytesSaved ||
		res.SavingsPercentage != want.SavingsPercentage || res.ModeApplied != want.ModeApplied || res.Folder != want.Folder {
		t.Errorf("result = %+v, want %+v", *res, want)
	}
}

func TestOptimizeJobFailed(t *testing.T) {
	f := &fakeFlow{t: t, statuses: []map[string]interface{}{
		{"job_id": "job_1", "status": "failed", "error": map[string]interface{}{"code": "upload_missing", "message": "client upload never arrived"}},
	}}
	_, err := f.client().OptimizeBytes(context.Background(), []byte("img"), nil)

	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("expected *Error, got %v", err)
	}
	if e.Code != "job_failed" || e.Message != "client upload never arrived" || e.Details["job_id"] != "job_1" {
		t.Errorf("got code %q message %q details %v", e.Code, e.Message, e.Details)
	}
}

func TestOptimizeWaitTimeout(t *testing.T) {
	f := &fakeFlow{t: t, statuses: []map[string]interface{}{{"job_id": "job_1", "status": "processing"}}}
	_, err := f.client(WithWaitTimeout(5*time.Millisecond)).OptimizeBytes(context.Background(), []byte("img"), nil)

	if !IsTimeout(err) {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if !strings.Contains(err.Error(), "job_1") || !strings.Contains(err.Error(), "GetJobStatus") {
		t.Errorf("message must name the job and GetJobStatus: %v", err)
	}
	if f.polls == 0 {
		t.Error("expected at least one poll")
	}
}

func TestOptimizeContextCancelStopsWait(t *testing.T) {
	f := &fakeFlow{t: t, statuses: []map[string]interface{}{{"job_id": "job_1", "status": "processing"}}}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := f.client(WithWaitTimeout(time.Minute)).OptimizeBytes(ctx, []byte("img"), nil)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline error, got %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Error("wait did not stop on context cancellation")
	}
}

func TestOptimizeUploadRefused(t *testing.T) {
	f := &fakeFlow{t: t, putStatus: http.StatusForbidden}
	_, err := f.client().OptimizeBytes(context.Background(), []byte("img"), nil)

	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("expected *Error, got %v", err)
	}
	if e.Code != "upload_failed" || e.StatusCode != http.StatusForbidden || !strings.Contains(e.Message, "AccessDenied") {
		t.Errorf("got code %q status %d message %q", e.Code, e.StatusCode, e.Message)
	}
	if f.polls != 0 {
		t.Errorf("polls = %d after a refused upload, want 0", f.polls)
	}
}

func TestOptimizeFileRejectsURL(t *testing.T) {
	for _, src := range []string{"https://example.com/a.jpg", "http://example.com/a.jpg", "HTTPS://example.com/a.jpg"} {
		f := &fakeFlow{t: t}
		_, err := f.client().OptimizeFile(context.Background(), src, nil)
		if !IsValidationError(err) || !strings.Contains(err.Error(), "does not fetch URLs") {
			t.Errorf("%s: expected validation error about URLs, got %v", src, err)
		}
		if f.requests != 0 {
			t.Errorf("%s: %d requests sent, want 0", src, f.requests)
		}
	}
}

// A caller that reuses one options value for many files must not have the first file's
// name sent for every later one.
func TestOptimizeFileKeepsCallerOptions(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.png", "b.png"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("img"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	f := &fakeFlow{t: t}
	c := f.client()
	opts := &OptimizeOptions{Format: FormatWebP}
	for _, name := range []string{"a.png", "b.png"} {
		if _, err := c.OptimizeFile(context.Background(), filepath.Join(dir, name), opts); err != nil {
			t.Fatalf("OptimizeFile %s: %v", name, err)
		}
	}
	if got := strings.Join(f.filenames, ","); got != "a.png,b.png" {
		t.Fatalf("filenames sent = %q, want a.png,b.png", got)
	}
	if opts.Filename != "" {
		t.Fatalf("caller's options were changed: Filename = %q", opts.Filename)
	}
}
