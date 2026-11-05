# SmallPict Go SDK

Official idiomatic Go SDK for the [SmallPict Image Optimization API](https://smallpict.app) — high-performance next-gen image transcoding (AVIF, WebP), smart compression, Edge CDN delivery, and cache purging.

[![Go Reference](https://pkg.go.dev/badge/github.com/smallpict/smallpict-go.svg)](https://pkg.go.dev/github.com/smallpict/smallpict-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/smallpict/smallpict-go)](https://goreportcard.com/report/github.com/smallpict/smallpict-go)

---

## ⚡ Features

- **🚀 100% Standard Library:** Zero external third-party HTTP dependencies.
- **⏱️ Full `context.Context` Propagation:** Seamless timeout and cancellation control on all network calls.
- **🌊 Any Byte Source:** `Optimize` takes an `io.Reader`, `OptimizeBytes` a `[]byte`, `OptimizeFile` a local path. The image is read into memory first, since its exact size is sent with the request.
- **🛡️ Secure HMAC-SHA256 & Bearer Auth:** Tamper-proof payload verification.
- **✨ 4 Core Unified Methods:** `Optimize()`, `GetQuota()`, `PurgeCDN()`, and `ValidateKey()`.
- **🔄 Resilience & Fault Tolerance:** Automatic `Idempotency-Key` UUID injection, 30s timeouts, and exponential backoff with jitter on HTTP 429/5xx.
- **🔒 Zero-Leak Privacy:** API keys and credentials are automatically redacted from `Error()` and `String()` outputs.

---

## 📥 Installation

```bash
go get -u github.com/smallpict/smallpict-go
```

---

## 🚀 Quick Start

### 1. Minimal Example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/smallpict/smallpict-go"
)

func main() {
	client, err := smallpict.NewClient(
		smallpict.WithAPIKey(os.Getenv("SMALLPICT_API_KEY")),
		smallpict.WithSecretKey(os.Getenv("SMALLPICT_SECRET_KEY")), // Optional HMAC Secret
	)
	if err != nil {
		log.Fatalf("Failed to initialize SmallPict client: %v", err)
	}

	result, err := client.OptimizeFile(context.Background(), "hero-banner.png", &smallpict.OptimizeOptions{
		Format:   smallpict.FormatAVIF,
		Quality:  80,
		MaxWidth: 1920,
	})
	if err != nil {
		log.Fatalf("Optimization failed: %v", err)
	}

	fmt.Printf("Optimized URL: %s\n", result.URL)
	fmt.Printf("Original: %d B ➔ Compressed: %d B\n", result.OriginalSize, result.CompressedSize)
	fmt.Printf("Saved: %.2f%% (%d bytes)\n", result.SavingsPercentage, result.BytesSaved)
}
```

### What `Optimize` does

The API is asynchronous: `POST /v1/optimize` queues a job and returns a pre-signed upload URL. `Optimize`, `OptimizeBytes` and `OptimizeFile` run the whole flow for you: they upload the bytes to that URL (within the 30 seconds the worker waits for them), poll `GET /v1/optimize/status` every second, and return once the job is `completed` with `URL`, `Format`, `BytesSaved`, `ModeApplied`, `QualityApplied` and `Folder` from the job status. `OriginalSize` is the number of bytes sent and `CompressedSize` is `OriginalSize - BytesSaved`. `URL` for managed storage is a pre-signed link valid 10 minutes; `client.GetJobStatus(ctx, result.JobID)` returns a fresh one.

- A failed job returns an `*smallpict.Error` whose `Code` is `job_failed`, whose message is the server's (for example `client upload never arrived`) and whose `Details["job_id"]` names the job. Storage refusing the upload returns `Code` `upload_failed` with the HTTP status.
- A job still running after `WithWaitTimeout` (default 120 s) returns an error for which `smallpict.IsTimeout` is true, naming the job id; the job keeps running, so you can call `GetJobStatus` later. `WithPollInterval` (default 1 s) sets the polling interval. Cancelling `ctx` also stops the wait.
- Pass bytes or a local file. `OptimizeFile` with an `http://` or `https://` URL returns a `VALIDATION_FAILED` error before any request: the API does not fetch URLs, so download the image yourself first.

```go
client, err := smallpict.NewClient(
	smallpict.WithAPIKey(os.Getenv("SMALLPICT_API_KEY")),
	smallpict.WithPollInterval(500*time.Millisecond),
	smallpict.WithWaitTimeout(60*time.Second),
)
```

### 2. HTTP Web Handler with io.Reader Streaming

```go
package main

import (
	"net/http"

	"github.com/smallpict/smallpict-go"
)

func handleUpload(client *smallpict.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, header, err := r.FormFile("image")
		if err != nil {
			http.Error(w, "Invalid file upload", http.StatusBadRequest)
			return
		}
		defer file.Close()

		result, err := client.Optimize(r.Context(), file, &smallpict.OptimizeOptions{
			Filename: header.Filename,
			Format:   smallpict.FormatAuto,
			Quality:  85,
		})
		if err != nil {
			if smallpict.IsQuotaExceeded(err) {
				http.Error(w, "Monthly image optimization quota exceeded", http.StatusPaymentRequired)
				return
			}
			if smallpict.IsTimeout(err) {
				http.Error(w, "Optimization is still running, try again later", http.StatusGatewayTimeout)
				return
			}
			http.Error(w, "Optimization failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"url":"%s","format":"%s","savings":"%.2f%%"}`, result.URL, result.Format, result.SavingsPercentage)
	}
}
```

### 3. Compression Mode

`Mode` is `smallpict.ModeLossy` (server default), `smallpict.ModeExtreme`, `smallpict.ModeLossless` or `smallpict.ModeAuto`. `auto` picks, per image, the lowest quality that keeps the visual score (SSIMULACRA 2) at or above 70, and ignores `Quality`; it is on WP Pro and Business, Velocity, Momentum and sandbox keys (5 auto jobs per minute per sandbox key), and when it cannot search (PNG or GIF output, animation, time limit) the image is encoded `lossy` and `ModeApplied` says so. `extreme` gives the smallest files and is available on every plan; `lossless` needs a plan that includes it. Leave `Quality` at zero with `extreme` so the server picks its default for the output format; a quality you set is kept, or raised to that format's floor. The mode comparison is in the [image settings docs](https://smallpict.app/en/docs/image-settings), the per-format defaults in the API contract.

```go
result, err := client.OptimizeFile(ctx, "hero-banner.jpg", &smallpict.OptimizeOptions{
	Format: smallpict.FormatWebP,
	Mode:   smallpict.ModeExtreme, // no Quality: the server uses the WebP default for extreme
})
var apiErr *smallpict.Error
if smallpict.IsPermissionDenied(err) && errors.As(err, &apiErr) {
	log.Fatalf("mode not in plan, allowed: %v", apiErr.Details["allowed_modes"])
} else if err != nil {
	log.Fatal(err)
}

if result.QualityApplied != nil {
	fmt.Printf("%s at quality %d\n", result.ModeApplied, *result.QualityApplied)
}
```

---

## 📊 Checking Quota & Invalidate CDN Cache

```go
// 1. Get real-time quota usage
quota, err := client.GetQuota(ctx)
if err == nil {
	fmt.Printf("Plan: %s, Quota Used: %.2f%%\n", quota.Plan, quota.QuotaPercentage)
}

// 2. Invalidate CDN cache for updated images
purgeRes, err := client.PurgeCDN(ctx, []string{"https://cdn.smallpict.app/opt/hero-banner.avif"}, smallpict.PurgeTypeURL)
if err == nil {
	fmt.Println(purgeRes.Message)
}
```

### Folders

`OptimizeOptions.Folder` stores the optimized file under a folder of your account (1-3 segments of letters, digits, `_` and `-`, separated by `/`). The server lower-cases it and returns it as `Folder` on the result and the job status. Leave it empty to keep the default location.

```go
res, err := client.OptimizeFile(ctx, "shoe.png", &smallpict.OptimizeOptions{Folder: "products/shoes"})

// Purge every file delivered under that folder.
_, err = client.PurgeFolder(ctx, "products/shoes")
if smallpict.IsRateLimit(err) {
	// 429 purge_rate_limited: folder purges are limited per hour (all: 2, folder: 10).
	// The SDK does not retry this error; try again later.
}
```

Malformed folders (`a//b`, a leading or trailing `/`, `..`, more than 3 segments) fail with a validation error before any request; the server rejects the rest with `400 invalid_folder` (`IsValidationError`).

### Capacity and upgrade links

`GetQuota` returns `Capacity` for the product of your API key: `Storage` (`LimitBytes`, `UsedBytes`, `BaseBytes`, `AddonBlocks`), `CDN` (`LimitBytes`, `UsedBytes`, `Status`), booster balances in `Booster` (`Active`, `WPQuotaBytes`, `APITransforms`, `CDNBytes`, `NextExpiry`), the unpaid-account state `Lapse`, `PeriodEnd` and `ManageURL`. A member that does not apply to your plan is `nil`; `Capacity` itself is `nil` when the server could not read it and on older servers. Capacity is read-only here: storage add-ons and boosters are bought in the dashboard.

```go
quota, err := client.GetQuota(ctx)
if err == nil && quota.Capacity != nil && quota.Capacity.Storage != nil {
	fmt.Printf("Storage: %d / %d bytes\n", quota.Capacity.Storage.UsedBytes, quota.Capacity.Storage.LimitBytes)
}
```

Errors about capacity carry `UpgradeURL()`, the dashboard page where you add it (`""` when the error has none). `storage_quota_exceeded` and `account_read_only` satisfy `IsQuotaExceeded`; `mode_not_in_plan` satisfies `IsPermissionDenied` and lists the plan's modes in `AllowedModes()`. The SDK never opens these links; show them to your user.

```go
_, err := client.OptimizeFile(ctx, "hero.png", nil)
var spErr *smallpict.Error
if smallpict.IsQuotaExceeded(err) && errors.As(err, &spErr) && spErr.UpgradeURL() != "" {
	fmt.Println("Add capacity:", spErr.UpgradeURL())
}
```

---

## 🧪 Testing

```bash
go test -race -v -cover ./...
go vet ./...
```

---

## 🔒 Security & Schema Compatibility (2026)

- **MIMEType & ContentType Dual-Field Support**: `OptimizeOptions` supports both `MIMEType` and `ContentType` fields interchangeably. Outgoing conversion payloads send both `mime_type` and `content_type` ensuring 100% backward and forward compatibility with all SmallPict API backend versions without contract breaking changes.

---

## 📄 License

MIT © [SmallPict Engineering](https://smallpict.app)

