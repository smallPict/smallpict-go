# Changelog

All notable changes to the `github.com/smallpict/smallpict-go` package will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `ModeAuto` (`"auto"`): the server picks the quality per image from a visual score (CMP-07). Plans: WP Pro and Business, Velocity, Momentum and sandbox; a server without auto enabled answers `400 invalid_mode` (`IsValidationError`).
- `OptimizeOptions.Mode` with the typed `CompressionMode` values `ModeLossy` (server default), `ModeExtreme` and `ModeLossless`, sent as `options.mode` (FND-07). The server checks the mode against the plan: `403 mode_not_in_plan` is an `*Error` for which `IsPermissionDenied` is true, with `Details["mode"]` and `Details["allowed_modes"]`; `400 invalid_mode` satisfies `IsValidationError`.
- `JobStatusResult.ModeApplied` and `JobStatusResult.QualityApplied`, read from `mode_applied` and `quality_applied` of the job status. They stay empty (`""`, `nil`) for jobs created before the server reported them.
- **Upload folders (FLD-03).** `OptimizeOptions.Folder` is sent as `options.folder` and omitted when empty. `OptimizeResult.Folder` and `JobStatusResult.Folder` return the folder the server stored (lower case), empty without one. Malformed folders (empty segments such as `a//b`, `/a` or `a/`, `..` segments, more than 3 segments) return a `VALIDATION_FAILED` error before any request; the server's `400 invalid_folder` satisfies `IsValidationError` and keeps the server's message.
- README section "Compression Mode" with a `ModeExtreme` example that leaves `Quality` unset (CMP-04).
- **Folder purge (FLD-03).** `Client.PurgeFolder(ctx, folder)` sends `{"purge_type": "folder", "folder": ...}` to `POST /v1/purge`, and `PurgeTypeFolder` names the type. `PurgeCDN` with `PurgeTypeFolder` returns a `VALIDATION_FAILED` error without a request, since it has no folder argument; `url` and `all` purges are unchanged.

- **Capacity in `GetQuota` (SDK-02).** `QuotaResponse.Capacity` returns the read-only storage, CDN and booster balances (with the lot that expires first, `NextExpiry`), the unpaid-account state (`Lapse`), `PeriodEnd` and `ManageURL` of the key's product. Members that do not apply are `nil`; `Capacity` is `nil` when the server sends `null` and on older servers without it, whose responses still parse.
- **Upgrade links on errors (SDK-02).** `(*Error).UpgradeURL()` returns `details.upgrade_url` (`""` when absent or not a string), and `(*Error).AllowedModes()` returns `details.allowed_modes` of `403 mode_not_in_plan` (`nil` when absent). `402 storage_quota_exceeded` and `account_read_only` satisfy `IsQuotaExceeded`; the 409 capacity codes stay a plain `*Error` with `StatusCode` 409. The SDK never opens these links.

### Deprecated
- `OptimizeOptions.Lossless`: use `Mode: smallpict.ModeLossless`. `Lossless: true` is now sent as `mode: "lossless"`; with a different `Mode` it returns a `VALIDATION_FAILED` error before any request. It will not be removed before the next major version.

### Fixed
- `Lossless: true` used to be sent as the `lossless` key, which the server ignored; it is now honoured as `mode: "lossless"`. The SDK no longer sends the `lossless` key.
- **`quality` is sent only when set (CMP-04).** `Optimize` and `OptimizeFile` used to send `quality: 80` with nil options and `quality: 0` when `Quality` was left at zero. A zero `Quality` is now omitted, so the server picks its default per mode and output format: `ModeExtreme` now gets its per-codec default instead of lossy-sized output at 80. Lossy results are unchanged, since the server default for lossy is 80.

### Changed
- Module path moved to `github.com/smallpict/smallpict-go` after the repository was transferred to the smallPict GitHub organization. Update imports from `github.com/tuxnoob/smallpict-go` to the new path.
- **`429 purge_rate_limited` is no longer retried.** The hourly limit on `folder` and `all` purges does not reset within the retry window, so the SDK returns it at once as an `*Error` for which `IsRateLimit` is true. Other `429` and `5xx` responses are retried as before.

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
