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
	// PurgeTypeFolder purges one upload folder; use PurgeFolder, which sends it.
	PurgeTypeFolder = "folder"
)

// CompressionMode selects how POST /v1/optimize compresses the image.
// The server checks it against the plan; ModeLossy is the server default.
type CompressionMode string

// Compression modes accepted by POST /v1/optimize (options.mode).
const (
	ModeLossy    CompressionMode = "lossy"
	ModeExtreme  CompressionMode = "extreme"
	ModeLossless CompressionMode = "lossless"
	// ModeAuto lets the server pick the quality per image from a visual score (CMP-07).
	ModeAuto CompressionMode = "auto"
)

// OptimizeOptions contains transformation and compression parameters for an image.
type OptimizeOptions struct {
	Format string `json:"format,omitempty"`
	// Quality is the encoder quality (1-100). Zero leaves it to the server,
	// which picks a default per mode and output format.
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
	// Folder stores the optimized file under this folder of the account
	// (1-3 segments separated by "/"), sent as options.folder. Empty keeps
	// the default location.
	Folder string `json:"folder,omitempty"`
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
	// Folder is the folder stored with the job, normalized by the server
	// (lower case); empty without one.
	Folder string `json:"folder,omitempty"`
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
	// Capacity is the read-only capacity of the key's product. It is nil when
	// the server could not read it ("capacity": null) and on older servers that
	// do not send it.
	Capacity *Capacity `json:"capacity,omitempty"`
}

// Capacity is the read-only "capacity" object of GET /v1/quota. A member that
// does not apply to the product or plan is nil. Capacity is bought only in the
// dashboard; the SDK never opens or fetches ManageURL.
type Capacity struct {
	PeriodEnd *time.Time       `json:"period_end"`
	Storage   *CapacityStorage `json:"storage"`
	CDN       *CapacityCDN     `json:"cdn"`
	Booster   CapacityBooster  `json:"booster"`
	Lapse     *CapacityLapse   `json:"lapse"`
	ManageURL string           `json:"manage_url"`
}

// CapacityStorage is the smallPict-managed storage of the account.
type CapacityStorage struct {
	LimitBytes  int64 `json:"limit_bytes"`
	UsedBytes   int64 `json:"used_bytes"`
	BaseBytes   int64 `json:"base_bytes"`
	AddonBlocks int64 `json:"addon_blocks"`
}

// CapacityCDN is the managed CDN usage of the current cycle. Status is one of
// "ok", "warn_80", "exceeded_grace" or "hard_block".
type CapacityCDN struct {
	LimitBytes int64  `json:"limit_bytes"`
	UsedBytes  int64  `json:"used_bytes"`
	Status     string `json:"status"`
}

// CapacityBooster holds the one-time booster balances left. A balance that does
// not apply to the product is nil.
type CapacityBooster struct {
	Active        bool                `json:"active"`
	WPQuotaBytes  *int64              `json:"wp_quota_bytes"`
	APITransforms *int64              `json:"api_transforms"`
	CDNBytes      *int64              `json:"cdn_bytes"`
	NextExpiry    *CapacityNextExpiry `json:"next_expiry"`
}

// CapacityNextExpiry is the booster lot that expires first. Kind is one of
// "wp_quota_bytes", "api_transforms" or "cdn_bytes".
type CapacityNextExpiry struct {
	Kind      string    `json:"kind"`
	Units     int64     `json:"units"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CapacityLapse is the unpaid-account state. Stage is one of "grace",
// "blocked" or "deleting".
type CapacityLapse struct {
	Stage       string     `json:"stage"`
	DeleteAfter *time.Time `json:"delete_after"`
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
	// Folder is the folder stored with the job (lower case); empty without one.
	Folder string `json:"folder,omitempty"`
}
