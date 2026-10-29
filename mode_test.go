package smallpict

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func jsonResponse(status int, body interface{}) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(b)),
	}
}

// Mapping cases from smallpict-sdk-shared/compression-modes.json.
func TestOptimizeModeMapping(t *testing.T) {
	cases := []struct {
		name      string
		opts      OptimizeOptions
		wireMode  string // "" means the mode key is absent
		wantError bool
	}{
		{name: "no mode, no flag"},
		{name: "mode only", opts: OptimizeOptions{Mode: ModeExtreme}, wireMode: "extreme"},
		{name: "deprecated flag only", opts: OptimizeOptions{Lossless: true}, wireMode: "lossless"},
		{name: "flag false is ignored", opts: OptimizeOptions{Lossless: false}},
		{name: "flag agrees with mode", opts: OptimizeOptions{Mode: ModeLossless, Lossless: true}, wireMode: "lossless"},
		{name: "flag contradicts mode", opts: OptimizeOptions{Mode: ModeLossy, Lossless: true}, wantError: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			var sent map[string]interface{}
			mock := newMockHTTPClient(func(r *http.Request) *http.Response {
				calls++
				var body struct {
					Options map[string]interface{} `json:"options"`
				}
				b, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(b, &body)
				sent = body.Options
				return jsonResponse(http.StatusOK, map[string]interface{}{"job_id": "job_mode", "status": "queued"})
			})
			client, _ := NewClient(WithAPIKey("sp_sdk_test"), WithHTTPClient(mock))

			opts := tc.opts
			_, err := client.Optimize(context.Background(), bytes.NewBufferString("img"), &opts)

			if tc.wantError {
				if !IsValidationError(err) {
					t.Fatalf("expected validation error, got %v", err)
				}
				if calls != 0 {
					t.Fatalf("expected no HTTP call, got %d", calls)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if _, ok := sent["lossless"]; ok {
				t.Error("lossless key must not be sent")
			}
			got, present := sent["mode"]
			if tc.wireMode == "" {
				if present {
					t.Errorf("expected no mode key, got %v", got)
				}
			} else if got != tc.wireMode {
				t.Errorf("expected mode %q, got %v", tc.wireMode, got)
			}
		})
	}
}

func TestGetJobStatusModeApplied(t *testing.T) {
	mock := newMockHTTPClient(func(r *http.Request) *http.Response {
		return jsonResponse(http.StatusOK, map[string]interface{}{
			"job_id": "job_1", "status": "completed", "mode_applied": "extreme", "quality_applied": 68,
		})
	})
	client, _ := NewClient(WithAPIKey("sp_sdk_test"), WithHTTPClient(mock))
	res, err := client.GetJobStatus(context.Background(), "job_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ModeApplied != "extreme" {
		t.Errorf("unexpected ModeApplied: %q", res.ModeApplied)
	}
	if res.QualityApplied == nil || *res.QualityApplied != 68 {
		t.Errorf("unexpected QualityApplied: %v", res.QualityApplied)
	}

	// Old jobs carry neither field.
	mockOld := newMockHTTPClient(func(r *http.Request) *http.Response {
		return jsonResponse(http.StatusOK, map[string]interface{}{"job_id": "job_old", "status": "completed"})
	})
	client, _ = NewClient(WithAPIKey("sp_sdk_test"), WithHTTPClient(mockOld))
	res, err = client.GetJobStatus(context.Background(), "job_old")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ModeApplied != "" || res.QualityApplied != nil {
		t.Errorf("expected empty mode/quality for old job, got %q %v", res.ModeApplied, res.QualityApplied)
	}
}

func TestOptimizeModeNotInPlan(t *testing.T) {
	mock := newMockHTTPClient(func(r *http.Request) *http.Response {
		return jsonResponse(http.StatusForbidden, map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "mode_not_in_plan",
				"message": "Mode lossless is not available on your plan",
				"details": map[string]interface{}{
					"mode":          "lossless",
					"allowed_modes": []string{"lossy", "extreme"},
				},
			},
		})
	})
	client, _ := NewClient(WithAPIKey("sp_sdk_test"), WithHTTPClient(mock))
	_, err := client.Optimize(context.Background(), bytes.NewBufferString("img"), &OptimizeOptions{Mode: ModeLossless})
	if !IsPermissionDenied(err) {
		t.Fatalf("expected permission denied, got %v", err)
	}
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if apiErr.Code != "mode_not_in_plan" || apiErr.Message != "Mode lossless is not available on your plan" {
		t.Errorf("unexpected code/message: %q %q", apiErr.Code, apiErr.Message)
	}
	allowed, ok := apiErr.Details["allowed_modes"].([]interface{})
	if !ok || len(allowed) != 2 || allowed[0] != "lossy" || allowed[1] != "extreme" {
		t.Errorf("unexpected allowed_modes: %v", apiErr.Details["allowed_modes"])
	}
	if apiErr.Details["mode"] != "lossless" {
		t.Errorf("unexpected details.mode: %v", apiErr.Details["mode"])
	}
}
