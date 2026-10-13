# SmallPict Go SDK

Official idiomatic Go SDK for the [SmallPict Image Optimization API](https://smallpict.app) — high-performance next-gen image transcoding (AVIF, WebP), smart compression, Edge CDN delivery, and cache purging.

[![Go Reference](https://pkg.go.dev/badge/github.com/smallpict/smallpict-go.svg)](https://pkg.go.dev/github.com/smallpict/smallpict-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/smallpict/smallpict-go)](https://goreportcard.com/report/github.com/smallpict/smallpict-go)

---

## ⚡ Features

- **🚀 100% Standard Library:** Zero external third-party HTTP dependencies.
- **⏱️ Full `context.Context` Propagation:** Seamless timeout and cancellation control on all network calls.
- **🌊 Zero-Copy Streaming:** Native `io.Reader` and `io.Writer` support for memory-efficient handling of large image buffers.
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

	result, err := smallpict.OptimizeFile(context.Background(), "hero-banner.png", &smallpict.OptimizeOptions{
		Format:   smallpict.FormatAVIF,
		Quality:  80,
		MaxWidth: 1920,
	})
	if err != nil {
		log.Fatalf("Optimization failed: %v", err)
	}

	fmt.Printf("Optimized CDN URL: %s\n", result.URL)
	fmt.Printf("Original: %d B ➔ Compressed: %d B\n", result.OriginalSize, result.CompressedSize)
	fmt.Printf("Saved: %.2f%% (%d bytes)\n", result.SavingsPercentage, result.BytesSaved)
}
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
			http.Error(w, "Optimization failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"url":"%s","format":"%s","savings":"%.2f%%"}`, result.URL, result.Format, result.SavingsPercentage)
	}
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

