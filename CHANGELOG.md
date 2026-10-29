# Changelog

All notable changes to the `github.com/smallpict/smallpict-go` package will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `OptimizeOptions.Mode` with the typed `CompressionMode` values `ModeLossy` (server default), `ModeExtreme` and `ModeLossless`, sent as `options.mode` (FND-07). The server checks the mode against the plan: `403 mode_not_in_plan` is an `*Error` for which `IsPermissionDenied` is true, with `Details["mode"]` and `Details["allowed_modes"]`; `400 invalid_mode` satisfies `IsValidationError`.
- `JobStatusResult.ModeApplied` and `JobStatusResult.QualityApplied`, read from `mode_applied` and `quality_applied` of the job status. They stay empty (`""`, `nil`) for jobs created before the server reported them.

### Deprecated
- `OptimizeOptions.Lossless`: use `Mode: smallpict.ModeLossless`. `Lossless: true` is now sent as `mode: "lossless"`; with a different `Mode` it returns a `VALIDATION_FAILED` error before any request. It will not be removed before the next major version.

### Fixed
- `Lossless: true` used to be sent as the `lossless` key, which the server ignored; it is now honoured as `mode: "lossless"`. The SDK no longer sends the `lossless` key.

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
