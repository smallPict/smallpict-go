package smallpict

import "time"

// Image formats supported for output transcoding.
const (
	FormatAuto = "auto"
	FormatAVIF = "avif"
	FormatWebP = "webp"
	FormatJPEG = "jpeg"
	FormatPNG  = "png"
)

// Resizing bounding box fit modes.
const (
	FitCover   = "cover"
	FitContain = "contain"
	FitInside  = "inside"
	FitOutside = "outside"
)

// Fallback behavior when account quota is exhausted.
const (
	FallbackThrow       = "throw"
	FallbackPassthrough = "passthrough"
)

// CDN cache invalidation scope.
const (
	PurgeTypeURL = "url"
	PurgeTypeAll = "all"
)

// CompressionMode selects how POST /v1/optimize compresses the image.
// The server checks it against the plan; ModeLossy is the server default.
type CompressionMode string

// Compression modes accepted by POST /v1/optimize (options.mode).
const (
	ModeLossy    CompressionMode = "lossy"
	ModeExtreme  CompressionMode = "extreme"
	ModeLossless CompressionMode = "lossless"
)

// OptimizeOptions contains transformation and compression parameters for an image.
type OptimizeOptions struct {
	Format       string `json:"format,omitempty"`
	Quality      int    `json:"quality,omitempty"`
	MaxWidth     int    `json:"max_width,omitempty"`
	MaxHeight    int    `json:"max_height,omitempty"`
	MaxDimension int    `json:"max_dimension,omitempty"`
	Fit          string `json:"fit,omitempty"`
	// Mode is the compression mode. Empty leaves it to the server (lossy).
	Mode CompressionMode `json:"mode,omitempty"`
	// Lossless requests lossless compression.
	//
	// Deprecated: use Mode. Lossless true is sent as Mode ModeLossless; it is
	// a validation error together with a different Mode.
	Lossless       bool   `json:"lossless,omitempty"`
	StripMetadata  bool   `json:"strip_metadata"`
	Filename       string `json:"filename,omitempty"`
	MIMEType       string `json:"mime_type,omitempty"`
	ContentType    string `json:"content_type,omitempty"`
	IdempotencyKey string `json:"-"`
}

// OptimizeResult represents the output from a successful image optimization.
type OptimizeResult struct {
	JobID             string  `json:"job_id"`
	Status            string  `json:"status"`
	URL               string  `json:"url"`
	Format            string  `json:"format"`
	OriginalSize      int64   `json:"original_size"`
	CompressedSize    int64   `json:"compressed_size"`
	BytesSaved        int64   `json:"bytes_saved"`
	SavingsPercentage float64 `json:"savings_percentage"`
	UploadURL         string  `json:"upload_url,omitempty"`
	Data              []byte  `json:"-"`
}

// QuotaResponse details the current monthly processing usage and account limits.
type QuotaResponse struct {
	Plan                string  `json:"plan"`
	BytesUsed           int64   `json:"bytes_used"`
	QuotaLimit          int64   `json:"quota_limit"`
	QuotaPercentage     float64 `json:"quota_percentage"`
	CDNEgressUsedBytes  *int64  `json:"cdn_egress_used_bytes,omitempty"`
	CDNEgressQuotaBytes *int64  `json:"cdn_egress_quota_bytes,omitempty"`
	ActiveKeysCount     *int    `json:"active_keys_count,omitempty"`
	ActiveSitesCount    *int    `json:"active_sites_count,omitempty"`
}

// PurgeResponse confirms enqueuing of a CDN cache invalidation request.
type PurgeResponse struct {
	Message string `json:"message"`
}

// JobStatusResult represents the current state of an asynchronous conversion job.
type JobStatusResult struct {
	JobID      string `json:"job_id"`
	Status     string `json:"status"`
	URL        string `json:"url,omitempty"`
	Format     string `json:"format,omitempty"`
	BytesSaved *int64 `json:"bytes_saved,omitempty"`
	// ModeApplied is the compression mode the server used; empty for older jobs.
	ModeApplied string `json:"mode_applied,omitempty"`
	// QualityApplied is the quality the encoder got; nil for older jobs.
	QualityApplied *int                   `json:"quality_applied,omitempty"`
	Error          map[string]interface{} `json:"error,omitempty"`
	CreatedAt      *time.Time             `json:"created_at,omitempty"`
	UpdatedAt      *time.Time             `json:"updated_at,omitempty"`
}
