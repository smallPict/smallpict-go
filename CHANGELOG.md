# Changelog

All notable changes to the `github.com/smallpict/smallpict-go` package will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- Module path moved to `github.com/smallpict/smallpict-go` after the repository was transferred to the smallPict GitHub organization. Update imports from `github.com/tuxnoob/smallpict-go` to the new path.

## [1.0.0] - 2026-08-22

### Added
- Official Go SDK implementation for SmallPict OpenAPI 3.1.0 API.
- Zero external dependencies using Go standard library exclusively (`net/http`, `crypto/hmac`, `crypto/sha256`).
- Full `context.Context` propagation across all network methods.
- Functional options pattern: `NewClient(WithAPIKey(...), ...)`.
- 4 unified core client methods: `Optimize()`, `GetQuota()`, `PurgeCDN()`, and `ValidateKey()`.
- Helper `GetJobStatus()` for polling asynchronous image conversion tasks.
- Stream-safe operations: `Optimize(ctx, io.Reader)`, `OptimizeBytes(ctx, []byte)`, `OptimizeFile(ctx, filepath)`.
- Automatic secret masking in `Error()` string and `fmt.Stringer`.
- Resilient HTTP transport with 30s timeouts, exponential backoff, jitter, and automatic `Idempotency-Key` UUID injection.
- Optional `WithFallbackMode(FallbackPassthrough)` for uninterrupted availability on quota limit exhaustion.
