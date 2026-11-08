package smallpict

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"
)

// Fixtures from smallpict-sdk-shared/capacity.json (quota_responses).
const capacityQuotaFixtures = `{
  "wordpress_with_capacity": {
    "plan": "pro",
    "bytes_used": 1048576,
    "capacity": {
      "period_end": "2026-11-30T00:00:00Z",
      "storage": { "limit_bytes": 75161927680, "used_bytes": 7516192768, "base_bytes": 53687091200, "addon_blocks": 2 },
      "cdn": { "limit_bytes": 53687091200, "used_bytes": 44023414784, "status": "warn_80" },
      "booster": {
        "active": true,
        "wp_quota_bytes": 2147483648,
        "api_transforms": null,
        "cdn_bytes": 107374182400,
        "next_expiry": { "kind": "cdn_bytes", "units": 42949672960, "expires_at": "2027-01-05T00:00:00Z" }
      },
      "lapse": null,
      "manage_url": "https://smallpict.app/dashboard/billing"
    }
  },
  "api_blocked": {
    "plan": "velocity",
    "bytes_used": 0,
    "capacity": {
      "period_end": null,
      "storage": { "limit_bytes": 107374182400, "used_bytes": 1073741824, "base_bytes": 107374182400, "addon_blocks": 0 },
      "cdn": null,
      "booster": { "active": false, "wp_quota_bytes": null, "api_transforms": 5000, "cdn_bytes": null, "next_expiry": null },
      "lapse": { "stage": "blocked", "delete_after": "2027-01-28T00:00:00Z" },
      "manage_url": "https://smallpict.app/dashboard/billing"
    }
  },
  "sandbox": {
    "plan": "sandbox",
    "bytes_used": 0,
    "capacity": {
      "period_end": null,
      "storage": null,
      "cdn": { "limit_bytes": 1073741824, "used_bytes": 0, "status": "ok" },
      "booster": { "active": false, "wp_quota_bytes": null, "api_transforms": 0, "cdn_bytes": 0, "next_expiry": null },
      "lapse": null,
      "manage_url": "https://smallpict.app/dashboard/billing"
    }
  },
  "capacity_unreadable": { "plan": "pro", "bytes_used": 0, "capacity": null },
  "older_server_without_capacity": { "plan": "pro", "bytes_used": 0 }
}`

const billingURL = "https://smallpict.app/dashboard/billing"

func quotaFromFixture(t *testing.T, name string) *QuotaResponse {
	t.Helper()
	var fixtures map[string]json.RawMessage
	if err := json.Unmarshal([]byte(capacityQuotaFixtures), &fixtures); err != nil {
		t.Fatal(err)
	}
	body := fixtures[name]
	mock := newMockHTTPClient(func(r *http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body))}
	})
	client, _ := NewClient(WithSecretKey("sk_test_secret"), WithAPIKey("sp_sdk_test"), WithHTTPClient(mock))
	q, err := client.GetQuota(context.Background())
	if err != nil {
		t.Fatalf("GetQuota: %v", err)
	}
	return q
}

func i64(v int64) *int64 { return &v }

func ts(t *testing.T, s string) *time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return &v
}

func TestGetQuotaCapacity(t *testing.T) {
	t.Run("wordpress_with_capacity", func(t *testing.T) {
		q := quotaFromFixture(t, "wordpress_with_capacity")
		if q.Plan != "pro" || q.BytesUsed != 1048576 {
			t.Fatalf("existing fields changed: %+v", q)
		}
		want := &Capacity{
			PeriodEnd: ts(t, "2026-11-30T00:00:00Z"),
			Storage:   &CapacityStorage{LimitBytes: 75161927680, UsedBytes: 7516192768, BaseBytes: 53687091200, AddonBlocks: 2},
			CDN:       &CapacityCDN{LimitBytes: 53687091200, UsedBytes: 44023414784, Status: "warn_80"},
			Booster: CapacityBooster{
				Active:       true,
				WPQuotaBytes: i64(2147483648),
				CDNBytes:     i64(107374182400),
				NextExpiry:   &CapacityNextExpiry{Kind: "cdn_bytes", Units: 42949672960, ExpiresAt: *ts(t, "2027-01-05T00:00:00Z")},
			},
			ManageURL: billingURL,
		}
		if !reflect.DeepEqual(q.Capacity, want) {
			t.Fatalf("capacity = %+v, want %+v", q.Capacity, want)
		}
		if q.Capacity.Booster.APITransforms != nil || q.Capacity.Lapse != nil {
			t.Fatal("null members must stay nil")
		}
	})

	t.Run("api_blocked", func(t *testing.T) {
		c := quotaFromFixture(t, "api_blocked").Capacity
		if c == nil || c.PeriodEnd != nil || c.CDN != nil || c.Booster.WPQuotaBytes != nil || c.Booster.CDNBytes != nil || c.Booster.NextExpiry != nil {
			t.Fatalf("null members must stay nil: %+v", c)
		}
		if c.Booster.Active || c.Booster.APITransforms == nil || *c.Booster.APITransforms != 5000 {
			t.Fatalf("booster = %+v", c.Booster)
		}
		if c.Storage == nil || c.Storage.AddonBlocks != 0 || c.Storage.LimitBytes != 107374182400 {
			t.Fatalf("storage = %+v", c.Storage)
		}
		want := &CapacityLapse{Stage: "blocked", DeleteAfter: ts(t, "2027-01-28T00:00:00Z")}
		if !reflect.DeepEqual(c.Lapse, want) {
			t.Fatalf("lapse = %+v", c.Lapse)
		}
	})

	t.Run("sandbox", func(t *testing.T) {
		c := quotaFromFixture(t, "sandbox").Capacity
		if c == nil || c.Storage != nil || c.Lapse != nil || c.Booster.WPQuotaBytes != nil {
			t.Fatalf("null members must stay nil: %+v", c)
		}
		// Zeros are present values, not nil.
		if c.Booster.APITransforms == nil || *c.Booster.APITransforms != 0 || c.Booster.CDNBytes == nil || *c.Booster.CDNBytes != 0 {
			t.Fatalf("sandbox zeros = %+v", c.Booster)
		}
		if !reflect.DeepEqual(c.CDN, &CapacityCDN{LimitBytes: 1073741824, Status: "ok"}) {
			t.Fatalf("cdn = %+v", c.CDN)
		}
	})

	for _, name := range []string{"capacity_unreadable", "older_server_without_capacity"} {
		t.Run(name, func(t *testing.T) {
			q := quotaFromFixture(t, name)
			if q.Capacity != nil || q.Plan != "pro" {
				t.Fatalf("quota = %+v", q)
			}
		})
	}
}

// Cases from smallpict-sdk-shared/capacity.json (error_cases).
func TestCapacityErrors(t *testing.T) {
	cases := []struct {
		status       int
		code         string
		details      map[string]interface{}
		upgradeURL   string
		allowedModes []string
	}{
		{402, "QUOTA_EXCEEDED", map[string]interface{}{"upgrade_url": billingURL}, billingURL, nil},
		{402, "storage_quota_exceeded", map[string]interface{}{"limit_bytes": 10, "used_bytes": 10, "remaining_bytes": 0, "upgrade_url": billingURL}, billingURL, nil},
		{402, "account_read_only", map[string]interface{}{"delete_after": "2027-01-28T00:00:00Z", "upgrade_url": billingURL}, billingURL, nil},
		{402, "QUOTA_EXCEEDED", map[string]interface{}{"payload_bytes": 60000000}, "", nil},
		{403, "mode_not_in_plan", map[string]interface{}{"mode": "extreme", "allowed_modes": []string{"lossy", "lossless"}}, "", []string{"lossy", "lossless"}},
		{409, "booster_period_limit_exceeded", map[string]interface{}{"remaining_steps": 0, "upgrade_url": billingURL}, billingURL, nil},
		// Not in the shared fixture: a non-string upgrade_url is not exposed.
		{402, "QUOTA_EXCEEDED", map[string]interface{}{"upgrade_url": 42}, "", nil},
	}

	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			mock := newMockHTTPClient(func(r *http.Request) *http.Response {
				return jsonResponse(tc.status, map[string]interface{}{
					"error": map[string]interface{}{"code": tc.code, "message": "capacity", "details": tc.details},
				})
			})
			client, _ := NewClient(WithSecretKey("sk_test_secret"), WithAPIKey("sp_sdk_test"), WithHTTPClient(mock))
			_, err := client.GetQuota(context.Background())

			var e *Error
			if !errors.As(err, &e) {
				t.Fatalf("expected *Error, got %v", err)
			}
			if e.Code != tc.code || e.StatusCode != tc.status {
				t.Fatalf("code/status = %s/%d", e.Code, e.StatusCode)
			}
			switch tc.status {
			case 402:
				if !IsQuotaExceeded(err) || IsPermissionDenied(err) {
					t.Fatal("402 must be QuotaExceeded only")
				}
			case 403:
				if !IsPermissionDenied(err) || IsQuotaExceeded(err) {
					t.Fatal("403 must be PermissionDenied only")
				}
			case 409:
				if IsQuotaExceeded(err) || IsPermissionDenied(err) || IsValidationError(err) {
					t.Fatal("409 must stay the base error")
				}
			}
			if got := e.UpgradeURL(); got != tc.upgradeURL {
				t.Fatalf("UpgradeURL = %q, want %q", got, tc.upgradeURL)
			}
			if got := e.AllowedModes(); !reflect.DeepEqual(got, tc.allowedModes) {
				t.Fatalf("AllowedModes = %v, want %v", got, tc.allowedModes)
			}
		})
	}
}
