package smallpict

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Client is the official SmallPict API client.
type Client struct {
	apiKey       string
	secretKey    string
	baseURL      string
	timeout      time.Duration
	maxRetries   int
	fallbackMode string
	httpClient   *http.Client
	pollInterval time.Duration
	waitTimeout  time.Duration
}

// NewClient initializes a new SmallPict Client with functional options.
func NewClient(opts ...Option) (*Client, error) {
	c := &Client{
		apiKey:       os.Getenv("SMALLPICT_API_KEY"),
		secretKey:    os.Getenv("SMALLPICT_SECRET_KEY"),
		baseURL:      "https://api.smallpict.app",
		timeout:      30 * time.Second,
		maxRetries:   3,
		fallbackMode: FallbackThrow,
		pollInterval: time.Second,
		waitTimeout:  120 * time.Second,
	}

	if envBase := os.Getenv("SMALLPICT_BASE_URL"); envBase != "" {
		c.baseURL = strings.TrimRight(envBase, "/")
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.apiKey == "" {
		return nil, &Error{
			Code:    "VALIDATION_FAILED",
			Message: "Missing required SmallPict API key. Pass WithAPIKey(...) or set SMALLPICT_API_KEY environment variable.",
		}
	}

	if c.httpClient == nil {
		c.httpClient = &http.Client{
			Timeout: c.timeout,
		}
	}

	return c, nil
}

// Optimize reads the image from source, queues the job, uploads the bytes to
// the job's upload URL and waits until the job is completed. It returns a
// job_failed *Error when the job fails and a TIMEOUT_ERROR (IsTimeout) when it
// is still running after WithWaitTimeout.
func (c *Client) Optimize(ctx context.Context, source io.Reader, opts *OptimizeOptions) (*OptimizeResult, error) {
	if opts == nil {
		opts = &OptimizeOptions{
			Format:        FormatAuto,
			Fit:           FitCover,
			StripMetadata: true,
		}
	}

	mode := opts.Mode
	if opts.Lossless {
		if mode != "" && mode != ModeLossless {
			return nil, &Error{Code: "VALIDATION_FAILED", Message: fmt.Sprintf("Lossless contradicts Mode %q; use Mode only", mode)}
		}
		mode = ModeLossless
	}

	if opts.Folder != "" {
		if err := validateFolder(opts.Folder); err != nil {
			return nil, err
		}
	}

	var data []byte
	var err error
	if source != nil {
		data, err = io.ReadAll(source)
		if err != nil {
			return nil, &Error{Code: "NETWORK_ERROR", Message: fmt.Sprintf("Failed to read image source stream: %v", err), Err: err}
		}
	}

	filename := opts.Filename
	if filename == "" {
		filename = "image.jpg"
	}
	mimeType := opts.MIMEType
	if mimeType == "" && opts.ContentType != "" {
		mimeType = opts.ContentType
	}
	if mimeType == "" {
		mimeType = "image/jpeg"
	}

	options := map[string]interface{}{
		"format":         opts.Format,
		"max_width":      opts.MaxWidth,
		"max_height":     opts.MaxHeight,
		"max_dimension":  opts.MaxDimension,
		"fit":            opts.Fit,
		"strip_metadata": opts.StripMetadata,
	}
	// Zero quality is omitted so the server picks its default for the mode and format.
	if opts.Quality != 0 {
		options["quality"] = opts.Quality
	}
	if mode != "" {
		options["mode"] = string(mode)
	}
	if opts.Folder != "" {
		options["folder"] = opts.Folder
	}
	payload := map[string]interface{}{
		"filename":     filename,
		"mime_type":    mimeType,
		"content_type": mimeType,
		"filesize":     len(data),
		"options":      options,
	}

	resMap, err := c.request(ctx, http.MethodPost, "/v1/optimize", payload, opts.IdempotencyKey)
	if err != nil {
		if IsQuotaExceeded(err) && c.fallbackMode == FallbackPassthrough {
			return &OptimizeResult{
				JobID:             "fallback-passthrough",
				Status:            "completed",
				Format:            strings.TrimPrefix(mimeType, "image/"),
				OriginalSize:      int64(len(data)),
				CompressedSize:    int64(len(data)),
				BytesSaved:        0,
				SavingsPercentage: 0.0,
				Data:              data,
			}, nil
		}
		return nil, err
	}

	jobID := getString(resMap, "job_id", "")
	uploadURL := getString(resMap, "upload_url", "")
	if jobID == "" || uploadURL == "" {
		return nil, &Error{Code: "INTERNAL_ERROR", Message: "optimize response has no job_id or upload_url"}
	}
	if err := c.upload(ctx, uploadURL, data, mimeType); err != nil {
		return nil, err
	}
	st, err := c.waitForJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	origSize := int64(len(data))
	var saved int64
	if st.BytesSaved != nil {
		saved = *st.BytesSaved
	}
	var savingsPct float64
	if origSize > 0 {
		savingsPct = math.Round(float64(saved)/float64(origSize)*100*100) / 100
	}
	return &OptimizeResult{
		JobID:             st.JobID,
		Status:            st.Status,
		URL:               st.URL,
		Format:            st.Format,
		OriginalSize:      origSize,
		CompressedSize:    origSize - saved,
		BytesSaved:        saved,
		SavingsPercentage: savingsPct,
		ModeApplied:       st.ModeApplied,
		QualityApplied:    st.QualityApplied,
		Folder:            st.Folder,
	}, nil
}

// upload PUTs the image to the pre-signed upload_url. Only Content-Type and
// Content-Length go with it: S3 refuses extra signed headers with 403. No retry,
// since the worker gives up 30 seconds after the optimize request.
func (c *Client) upload(ctx context.Context, uploadURL string, data []byte, mimeType string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, bytes.NewReader(data))
	if err != nil {
		return &Error{Code: "NETWORK_ERROR", Message: fmt.Sprintf("Failed to create upload request: %v", err), Err: err}
	}
	req.Header.Set("Content-Type", mimeType)
	req.ContentLength = int64(len(data))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &Error{Code: "NETWORK_ERROR", Message: fmt.Sprintf("Upload to storage failed: %v", err), Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		text, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &Error{
			Code:       "upload_failed",
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("Upload to storage failed with HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(text))),
		}
	}
	return nil
}

// waitForJob polls GetJobStatus every pollInterval until the job is completed
// or failed, for at most waitTimeout. The first poll comes after one interval.
func (c *Client) waitForJob(ctx context.Context, jobID string) (*JobStatusResult, error) {
	deadline := time.Now().Add(c.waitTimeout)
	for {
		timer := time.NewTimer(c.pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, &Error{Code: "TIMEOUT_ERROR", Message: fmt.Sprintf("Stopped waiting for job %s: %v", jobID, ctx.Err()), Err: ctx.Err()}
		case <-timer.C:
		}

		st, err := c.GetJobStatus(ctx, jobID)
		if err != nil {
			return nil, err
		}
		switch st.Status {
		case "completed":
			return st, nil
		case "failed":
			msg, _ := st.Error["message"].(string)
			if msg == "" {
				msg = "The optimization job failed."
			}
			return nil, &Error{Code: "job_failed", Message: msg, Details: map[string]interface{}{"job_id": jobID}}
		}
		if !time.Now().Before(deadline) {
			return nil, &Error{
				Code:    "TIMEOUT_ERROR",
				Message: fmt.Sprintf("Job %s did not finish within %s; it keeps running on the server, call GetJobStatus later", jobID, c.waitTimeout),
			}
		}
	}
}

// OptimizeBytes compresses an in-memory byte slice.
func (c *Client) OptimizeBytes(ctx context.Context, data []byte, opts *OptimizeOptions) (*OptimizeResult, error) {
	return c.Optimize(ctx, bytes.NewReader(data), opts)
}

// OptimizeFile reads and optimizes a local file from disk. An http(s) URL is
// rejected: the API does not fetch URLs.
func (c *Client) OptimizeFile(ctx context.Context, filePath string, opts *OptimizeOptions) (*OptimizeResult, error) {
	if lower := strings.ToLower(filePath); strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return nil, &Error{Code: "VALIDATION_FAILED", Message: "the API does not fetch URLs; download the image and pass its bytes"}
	}
	f, err := os.Open(filePath)
	if err != nil {
		return nil, &Error{Code: "VALIDATION_FAILED", Message: fmt.Sprintf("Failed to open file: %v", err), Err: err}
	}
	defer f.Close()

	if opts == nil {
		opts = &OptimizeOptions{
			Format:        FormatAuto,
			Fit:           FitCover,
			StripMetadata: true,
		}
	}
	if opts.Filename == "" {
		opts.Filename = filepath.Base(filePath)
	}

	return c.Optimize(ctx, f, opts)
}

// GetQuota retrieves real-time account storage quota and CDN egress bandwidth metrics.
func (c *Client) GetQuota(ctx context.Context) (*QuotaResponse, error) {
	resMap, err := c.request(ctx, http.MethodGet, "/v1/quota", nil, "")
	if err != nil {
		return nil, err
	}

	bytesUsed := getInt64(resMap, "bytes_used", 0)
	quotaLimit := getInt64(resMap, "quota_limit", 0)
	pct := getFloat64(resMap, "quota_percentage", 0.0)
	if pct == 0 && quotaLimit > 0 {
		pct = math.Round((float64(bytesUsed)/float64(quotaLimit)*100)*100) / 100
	}

	quota := &QuotaResponse{
		Plan:            getString(resMap, "plan", "free"),
		BytesUsed:       bytesUsed,
		QuotaLimit:      quotaLimit,
		QuotaPercentage: pct,
	}

	if v, ok := resMap["cdn_egress_used_bytes"].(float64); ok {
		val := int64(v)
		quota.CDNEgressUsedBytes = &val
	}
	if v, ok := resMap["cdn_egress_quota_bytes"].(float64); ok {
		val := int64(v)
		quota.CDNEgressQuotaBytes = &val
	}
	if v, ok := resMap["active_keys_count"].(float64); ok {
		val := int(v)
		quota.ActiveKeysCount = &val
	}
	if v, ok := resMap["active_sites_count"].(float64); ok {
		val := int(v)
		quota.ActiveSitesCount = &val
	}
	// Absent (older server) and null leave Capacity nil. A capacity that does
	// not decode also stays nil, so GetQuota and ValidateKey keep working.
	if raw, ok := resMap["capacity"].(map[string]interface{}); ok {
		var capacity Capacity
		if b, err := json.Marshal(raw); err == nil && json.Unmarshal(b, &capacity) == nil {
			quota.Capacity = &capacity
		}
	}

	return quota, nil
}

// PurgeCDN invalidates cached assets across global CDN edge points of presence.
func (c *Client) PurgeCDN(ctx context.Context, urls []string, purgeType string) (*PurgeResponse, error) {
	if purgeType == "" {
		purgeType = PurgeTypeURL
	}
	if purgeType == PurgeTypeFolder {
		return nil, &Error{Code: "VALIDATION_FAILED", Message: "purge type folder requires a folder; use PurgeFolder"}
	}
	return c.purge(ctx, map[string]interface{}{
		"purge_type": purgeType,
		"urls":       urls,
	})
}

// PurgeFolder invalidates every file the account delivered under folder.
// Folder purges are limited per hour: a 429 purge_rate_limited error
// (IsRateLimit) is returned without automatic retry.
func (c *Client) PurgeFolder(ctx context.Context, folder string) (*PurgeResponse, error) {
	if err := validateFolder(folder); err != nil {
		return nil, err
	}
	return c.purge(ctx, map[string]interface{}{
		"purge_type": PurgeTypeFolder,
		"folder":     folder,
	})
}

func (c *Client) purge(ctx context.Context, payload map[string]interface{}) (*PurgeResponse, error) {
	resMap, err := c.request(ctx, http.MethodPost, "/v1/purge", payload, "")
	if err != nil {
		return nil, err
	}

	return &PurgeResponse{
		Message: getString(resMap, "message", "Purge job enqueued"),
	}, nil
}

// validateFolder rejects obviously malformed folders before any request.
// The server owns the full rule (400 invalid_folder).
func validateFolder(folder string) error {
	segments := strings.Split(folder, "/")
	if len(segments) > 3 {
		return &Error{Code: "VALIDATION_FAILED", Message: "folder has more than 3 segments"}
	}
	for _, seg := range segments {
		if seg == "" {
			return &Error{Code: "VALIDATION_FAILED", Message: "folder must not be empty or have empty segments (//, leading or trailing /)"}
		}
		if seg == ".." {
			return &Error{Code: "VALIDATION_FAILED", Message: "folder must not contain .. segments"}
		}
	}
	return nil
}

// ValidateKey checks if the configured API key is valid and active.
func (c *Client) ValidateKey(ctx context.Context) bool {
	_, err := c.GetQuota(ctx)
	return err == nil
}

// GetJobStatus polls the progress of an asynchronous image conversion task.
func (c *Client) GetJobStatus(ctx context.Context, jobID string) (*JobStatusResult, error) {
	if jobID == "" {
		return nil, &Error{Code: "VALIDATION_FAILED", Message: "jobID parameter is required"}
	}

	resMap, err := c.request(ctx, http.MethodGet, fmt.Sprintf("/v1/optimize/status?job_id=%s", url.QueryEscape(jobID)), nil, "")
	if err != nil {
		return nil, err
	}

	result := &JobStatusResult{
		JobID:  getString(resMap, "job_id", jobID),
		Status: getString(resMap, "status", "processing"),
		URL:    getString(resMap, "url", ""),
		Format: getString(resMap, "format", ""),
	}

	if v, ok := resMap["bytes_saved"].(float64); ok {
		val := int64(v)
		result.BytesSaved = &val
	}
	if v, ok := resMap["error"].(map[string]interface{}); ok {
		result.Error = v
	}
	result.ModeApplied = getString(resMap, "mode_applied", "")
	result.Folder = getString(resMap, "folder", "")
	if v, ok := resMap["quality_applied"].(float64); ok {
		q := int(v)
		result.QualityApplied = &q
	}

	return result, nil
}

func (c *Client) request(
	ctx context.Context,
	method, path string,
	reqBody interface{},
	idempotencyKey string,
) (map[string]interface{}, error) {
	cleanPath := path
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}
	if !strings.HasPrefix(cleanPath, "/v1/") && !strings.HasPrefix(cleanPath, "/v2/") {
		cleanPath = "/v1" + cleanPath
	}

	endpointURL := fmt.Sprintf("%s%s", c.baseURL, cleanPath)

	var bodyBytes []byte
	bodyHash := EmptySHA256

	if reqBody != nil {
		var err error
		bodyBytes, err = json.Marshal(reqBody)
		if err != nil {
			return nil, &Error{Code: "VALIDATION_FAILED", Message: fmt.Sprintf("Failed to encode JSON payload: %v", err), Err: err}
		}
		bodyHash = SHA256Hex(bodyBytes)
	}

	attempt := 0
	baseDelay := 250 * time.Millisecond

	for attempt <= c.maxRetries {
		attempt++

		if err := ctx.Err(); err != nil {
			return nil, &Error{Code: "TIMEOUT_ERROR", Message: fmt.Sprintf("Context canceled: %v", err), Err: err}
		}

		timestamp := strconv.FormatInt(time.Now().Unix(), 10)

		var bodyReader io.Reader
		if len(bodyBytes) > 0 {
			bodyReader = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, endpointURL, bodyReader)
		if err != nil {
			return nil, &Error{Code: "NETWORK_ERROR", Message: fmt.Sprintf("Failed to create HTTP request: %v", err), Err: err}
		}

		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-API-Key", c.apiKey)

		if len(bodyBytes) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}

		if c.secretKey != "" {
			stringToSign := BuildStringToSign(method, cleanPath, timestamp, bodyHash)
			signature := HMACSHA256Hex(c.secretKey, stringToSign)
			req.Header.Set("X-Timestamp", timestamp)
			req.Header.Set("X-Signature", signature)
		} else {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}

		if method == http.MethodPost || method == http.MethodPatch || method == http.MethodDelete {
			if idempotencyKey == "" {
				idempotencyKey = generateUUID()
			}
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if attempt <= c.maxRetries {
				delay := baseDelay * time.Duration(1<<(attempt-1))
				time.Sleep(delay)
				continue
			}
			return nil, &Error{Code: "NETWORK_ERROR", Message: fmt.Sprintf("HTTP request failed: %v", err), Err: err}
		}

		respBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		requestID := resp.Header.Get("X-Request-ID")
		retryAfterHeader := resp.Header.Get("Retry-After")

		var result map[string]interface{}
		_ = json.Unmarshal(respBytes, &result)

		// Retry on 429 and transient 5xx. purge_rate_limited is an hourly
		// limit, so retrying within seconds cannot succeed.
		retryable := resp.StatusCode == http.StatusTooManyRequests || (resp.StatusCode >= 500 && resp.StatusCode <= 504)
		if retryable && errorCode(result) == "purge_rate_limited" {
			retryable = false
		}
		if retryable {
			if attempt <= c.maxRetries {
				delay := baseDelay * time.Duration(1<<(attempt-1))
				if retryAfterHeader != "" {
					if sec, parseErr := strconv.Atoi(retryAfterHeader); parseErr == nil && sec > 0 {
						delay = time.Duration(sec) * time.Second
					}
				}
				jitter := time.Duration(cryptoRandomInt(0, 100)) * time.Millisecond
				time.Sleep(delay + jitter)
				continue
			}
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			msg := fmt.Sprintf("API request failed with HTTP %d", resp.StatusCode)
			code := "INTERNAL_ERROR"
			var details map[string]interface{}

			if result != nil {
				if errObj, ok := result["error"].(map[string]interface{}); ok {
					if m, ok := errObj["message"].(string); ok {
						msg = m
					}
					if c, ok := errObj["code"].(string); ok {
						code = c
					}
					if d, ok := errObj["details"].(map[string]interface{}); ok {
						details = d
					}
				} else if m, ok := result["message"].(string); ok {
					msg = m
				}
			}

			return nil, &Error{
				Code:       code,
				StatusCode: resp.StatusCode,
				RequestID:  requestID,
				Message:    msg,
				Details:    details,
			}
		}

		if result == nil {
			result = make(map[string]interface{})
		}
		return result, nil
	}

	return nil, &Error{Code: "NETWORK_ERROR", Message: "Request failed after maximum retry attempts"}
}

func errorCode(result map[string]interface{}) string {
	if errObj, ok := result["error"].(map[string]interface{}); ok {
		if code, ok := errObj["code"].(string); ok {
			return code
		}
	}
	return ""
}

func getString(m map[string]interface{}, key, defaultVal string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return defaultVal
}

func getInt64(m map[string]interface{}, key string, defaultVal int64) int64 {
	if v, ok := m[key].(float64); ok {
		return int64(v)
	}
	return defaultVal
}

func getFloat64(m map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return defaultVal
}

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func cryptoRandomInt(min, max int64) int64 {
	nBig, err := rand.Int(rand.Reader, big.NewInt(max-min+1))
	if err != nil {
		return min
	}
	return nBig.Int64() + min
}
