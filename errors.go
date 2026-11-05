package smallpict

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	keyPattern    = regexp.MustCompile(`sp_(test|sdk|wp)_[a-zA-Z0-9_-]{10,}`)
	secretPattern = regexp.MustCompile(`(?i)(sec|secret)_[a-zA-Z0-9_-]{8,}`)
	bearerPattern = regexp.MustCompile(`(?i)Bearer\s+[a-zA-Z0-9._-]+`)
)

// SanitizeMessage masks API keys, HMAC secrets, and bearer tokens from error strings.
func SanitizeMessage(msg string) string {
	if msg == "" {
		return msg
	}
	msg = keyPattern.ReplaceAllStringFunc(msg, func(match string) string {
		if len(match) > 14 {
			return match[:10] + "..." + match[len(match)-4:]
		}
		return match[:6] + "..."
	})
	msg = secretPattern.ReplaceAllString(msg, "***REDACTED***")
	msg = bearerPattern.ReplaceAllString(msg, "Bearer ***REDACTED***")
	return msg
}

// Error represents a structured API or client error returned by the SmallPict SDK.
type Error struct {
	Code       string                 `json:"code"`
	StatusCode int                    `json:"status_code,omitempty"`
	RequestID  string                 `json:"request_id,omitempty"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Err        error                  `json:"-"`
}

func (e *Error) Error() string {
	safeMsg := SanitizeMessage(e.Message)
	if e.StatusCode > 0 {
		return fmt.Sprintf("[%s] HTTP %d: %s", e.Code, e.StatusCode, safeMsg)
	}
	return fmt.Sprintf("[%s]: %s", e.Code, safeMsg)
}

func (e *Error) String() string {
	return e.Error()
}

func (e *Error) Unwrap() error {
	return e.Err
}

// UpgradeURL returns details.upgrade_url, the dashboard page where the missing
// capacity is added, or "" when the error has none. Show it to the user; the
// SDK never opens or fetches it.
func (e *Error) UpgradeURL() string {
	u, _ := e.Details["upgrade_url"].(string)
	return u
}

// AllowedModes returns details.allowed_modes of a 403 mode_not_in_plan error,
// or nil when the error has none.
func (e *Error) AllowedModes() []string {
	raw, ok := e.Details["allowed_modes"].([]interface{})
	if !ok {
		return nil
	}
	modes := make([]string, 0, len(raw))
	for _, m := range raw {
		if s, ok := m.(string); ok {
			modes = append(modes, s)
		}
	}
	return modes
}

// Helper predicates for error classification.

func IsValidationError(err error) bool {
	var e *Error
	return errors.As(err, &e) && (e.StatusCode == 400 || e.Code == "VALIDATION_FAILED")
}

func IsUnauthorized(err error) bool {
	var e *Error
	return errors.As(err, &e) && (e.StatusCode == 401 || e.Code == "UNAUTHORIZED")
}

func IsQuotaExceeded(err error) bool {
	var e *Error
	return errors.As(err, &e) && (e.StatusCode == 402 || e.Code == "QUOTA_EXCEEDED" || e.Code == "OVERAGE_EXCEEDED")
}

func IsPermissionDenied(err error) bool {
	var e *Error
	return errors.As(err, &e) && (e.StatusCode == 403 || e.Code == "FORBIDDEN")
}

func IsNotFound(err error) bool {
	var e *Error
	return errors.As(err, &e) && (e.StatusCode == 404 || e.Code == "NOT_FOUND")
}

func IsRateLimit(err error) bool {
	var e *Error
	return errors.As(err, &e) && (e.StatusCode == 429 || e.Code == "RATE_LIMIT_EXCEEDED")
}

func IsServer(err error) bool {
	var e *Error
	return errors.As(err, &e) && (e.StatusCode >= 500 || e.Code == "INTERNAL_ERROR")
}

// IsTimeout reports a TIMEOUT_ERROR: Optimize waited past WithWaitTimeout, or the context ended.
func IsTimeout(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == "TIMEOUT_ERROR"
}
