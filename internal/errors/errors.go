// Package errors provides error types and handling for the DAST crawler.
package errors

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
)

// ErrorType categorizes errors for handling decisions.
type ErrorType int

const (
	// Unknown is an uncategorized error.
	Unknown ErrorType = iota
	// Network represents network-related errors (DNS, connection).
	Network
	// Timeout represents timeout errors.
	Timeout
	// RateLimit represents rate limiting (429) errors.
	RateLimit
	// Auth represents authentication/authorization errors (401, 403).
	Auth
	// NotFound represents 404 errors.
	NotFound
	// ServerError represents 5xx errors.
	ServerError
	// ClientError represents 4xx errors (except 401, 403, 404, 429).
	ClientError
	// Parse represents parsing errors (HTML, JSON, etc.).
	Parse
	// Browser represents browser/CDP errors.
	Browser
	// Scope represents scope violation errors.
	Scope
	// Cancelled represents context cancellation.
	Cancelled
)

// String returns the string representation of ErrorType.
func (t ErrorType) String() string {
	switch t {
	case Network:
		return "network"
	case Timeout:
		return "timeout"
	case RateLimit:
		return "rate_limit"
	case Auth:
		return "auth"
	case NotFound:
		return "not_found"
	case ServerError:
		return "server_error"
	case ClientError:
		return "client_error"
	case Parse:
		return "parse"
	case Browser:
		return "browser"
	case Scope:
		return "scope"
	case Cancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// IsRetryable returns whether errors of this type should be retried.
func (t ErrorType) IsRetryable() bool {
	switch t {
	case Network, Timeout, RateLimit, ServerError:
		return true
	default:
		return false
	}
}

// CrawlError represents a categorized crawl error.
type CrawlError struct {
	Type       ErrorType
	URL        string
	Operation  string
	Message    string
	Cause      error
	StatusCode int
	Retryable  bool
}

// Error implements the error interface.
func (e *CrawlError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s error during %s on %s: %s (caused by: %v)",
			e.Type.String(), e.Operation, e.URL, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s error during %s on %s: %s",
		e.Type.String(), e.Operation, e.URL, e.Message)
}

// Unwrap returns the underlying error.
func (e *CrawlError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches a target.
func (e *CrawlError) Is(target error) bool {
	t, ok := target.(*CrawlError)
	if !ok {
		return false
	}
	return e.Type == t.Type
}

// NewCrawlError creates a new CrawlError.
func NewCrawlError(errType ErrorType, url, operation, message string, cause error) *CrawlError {
	return &CrawlError{
		Type:      errType,
		URL:       url,
		Operation: operation,
		Message:   message,
		Cause:     cause,
		Retryable: errType.IsRetryable(),
	}
}

// NewNetworkError creates a network error.
func NewNetworkError(url, operation string, cause error) *CrawlError {
	return NewCrawlError(Network, url, operation, "network failure", cause)
}

// NewTimeoutError creates a timeout error.
func NewTimeoutError(url, operation string, cause error) *CrawlError {
	return NewCrawlError(Timeout, url, operation, "request timed out", cause)
}

// NewRateLimitError creates a rate limit error.
func NewRateLimitError(url string, retryAfter int) *CrawlError {
	err := NewCrawlError(RateLimit, url, "request", fmt.Sprintf("rate limited, retry after %ds", retryAfter), nil)
	err.StatusCode = 429
	return err
}

// NewAuthError creates an authentication error.
func NewAuthError(url string, statusCode int, message string) *CrawlError {
	err := NewCrawlError(Auth, url, "request", message, nil)
	err.StatusCode = statusCode
	err.Retryable = false
	return err
}

// NewNotFoundError creates a not found error.
func NewNotFoundError(url string) *CrawlError {
	err := NewCrawlError(NotFound, url, "request", "page not found", nil)
	err.StatusCode = 404
	err.Retryable = false
	return err
}

// NewServerError creates a server error.
func NewServerError(url string, statusCode int, message string) *CrawlError {
	err := NewCrawlError(ServerError, url, "request", message, nil)
	err.StatusCode = statusCode
	return err
}

// NewClientError creates a client error.
func NewClientError(url string, statusCode int, message string) *CrawlError {
	err := NewCrawlError(ClientError, url, "request", message, nil)
	err.StatusCode = statusCode
	err.Retryable = false
	return err
}

// NewParseError creates a parse error.
func NewParseError(url, operation string, cause error) *CrawlError {
	err := NewCrawlError(Parse, url, operation, "parsing failed", cause)
	err.Retryable = false
	return err
}

// NewBrowserError creates a browser error.
func NewBrowserError(url, operation string, cause error) *CrawlError {
	return NewCrawlError(Browser, url, operation, "browser operation failed", cause)
}

// NewScopeError creates a scope error.
func NewScopeError(url, reason string) *CrawlError {
	err := NewCrawlError(Scope, url, "scope_check", reason, nil)
	err.Retryable = false
	return err
}

// NewCancelledError creates a cancelled error.
func NewCancelledError(url, operation string) *CrawlError {
	err := NewCrawlError(Cancelled, url, operation, "operation cancelled", nil)
	err.Retryable = false
	return err
}

// Categorize determines the error type from a generic error.
func Categorize(err error, url string) *CrawlError {
	if err == nil {
		return nil
	}

	// Already a CrawlError
	var crawlErr *CrawlError
	if errors.As(err, &crawlErr) {
		return crawlErr
	}

	// Check for context cancellation
	if errors.Is(err, context_Canceled) || strings.Contains(err.Error(), "context canceled") {
		return NewCancelledError(url, "request")
	}

	// Check for timeout
	if isTimeout(err) {
		return NewTimeoutError(url, "request", err)
	}

	// Check for network errors
	if isNetworkError(err) {
		return NewNetworkError(url, "request", err)
	}

	// Default to unknown
	return NewCrawlError(Unknown, url, "request", err.Error(), err)
}

// CategorizeHTTPStatus creates an error from HTTP status code.
func CategorizeHTTPStatus(statusCode int, url string) *CrawlError {
	switch {
	case statusCode == 401:
		return NewAuthError(url, statusCode, "unauthorized")
	case statusCode == 403:
		return NewAuthError(url, statusCode, "forbidden")
	case statusCode == 404:
		return NewNotFoundError(url)
	case statusCode == 429:
		return NewRateLimitError(url, 60)
	case statusCode >= 500:
		return NewServerError(url, statusCode, fmt.Sprintf("server returned %d", statusCode))
	case statusCode >= 400:
		return NewClientError(url, statusCode, fmt.Sprintf("client error %d", statusCode))
	default:
		return nil
	}
}

// isTimeout checks if an error is a timeout.
func isTimeout(err error) bool {
	if err == nil {
		return false
	}

	// Check for net.Error timeout
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Check error message
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "context deadline")
}

// isNetworkError checks if an error is network-related.
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific network errors
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	// Check for syscall errors
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ETIMEDOUT) ||
		errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ENETUNREACH) {
		return true
	}

	// Check error message
	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "dial tcp")
}

// Sentinel errors for context (avoid import cycle).
var context_Canceled = errors.New("context canceled")

// IsRetryable checks if an error should be retried.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	var crawlErr *CrawlError
	if errors.As(err, &crawlErr) {
		return crawlErr.Retryable
	}

	// Check for temporary errors
	var tempErr interface{ Temporary() bool }
	if errors.As(err, &tempErr) && tempErr.Temporary() {
		return true
	}

	return isTimeout(err) || isNetworkError(err)
}

// IsAuthError checks if an error is authentication-related.
func IsAuthError(err error) bool {
	var crawlErr *CrawlError
	if errors.As(err, &crawlErr) {
		return crawlErr.Type == Auth
	}
	return false
}

// IsRateLimitError checks if an error is rate limiting.
func IsRateLimitError(err error) bool {
	var crawlErr *CrawlError
	if errors.As(err, &crawlErr) {
		return crawlErr.Type == RateLimit
	}
	return false
}

// GetStatusCode extracts the status code from an error.
func GetStatusCode(err error) int {
	var crawlErr *CrawlError
	if errors.As(err, &crawlErr) {
		return crawlErr.StatusCode
	}
	return 0
}

// GetErrorType extracts the error type from an error.
func GetErrorType(err error) ErrorType {
	var crawlErr *CrawlError
	if errors.As(err, &crawlErr) {
		return crawlErr.Type
	}
	return Unknown
}
