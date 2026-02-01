package errors

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

// =============================================================================
// ErrorType Tests
// =============================================================================

func TestErrorType_String(t *testing.T) {
	tests := []struct {
		errType ErrorType
		want    string
	}{
		{Unknown, "unknown"},
		{Network, "network"},
		{Timeout, "timeout"},
		{RateLimit, "rate_limit"},
		{Auth, "auth"},
		{NotFound, "not_found"},
		{ServerError, "server_error"},
		{ClientError, "client_error"},
		{Parse, "parse"},
		{Browser, "browser"},
		{Scope, "scope"},
		{Cancelled, "cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.errType.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorType_IsRetryable(t *testing.T) {
	tests := []struct {
		errType   ErrorType
		retryable bool
	}{
		{Network, true},
		{Timeout, true},
		{RateLimit, true},
		{ServerError, true},
		{Auth, false},
		{NotFound, false},
		{ClientError, false},
		{Parse, false},
		{Scope, false},
		{Cancelled, false},
		{Unknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.errType.String(), func(t *testing.T) {
			if got := tt.errType.IsRetryable(); got != tt.retryable {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.retryable)
			}
		})
	}
}

// =============================================================================
// CrawlError Tests
// =============================================================================

func TestCrawlError_Error(t *testing.T) {
	err := NewCrawlError(Network, "https://example.com", "fetch", "connection failed", nil)

	errStr := err.Error()
	if errStr == "" {
		t.Error("Error() should not return empty string")
	}
	if !containsAll(errStr, "network", "fetch", "https://example.com", "connection failed") {
		t.Errorf("Error() = %s, should contain relevant info", errStr)
	}
}

func TestCrawlError_Error_WithCause(t *testing.T) {
	cause := errors.New("underlying error")
	err := NewCrawlError(Network, "https://example.com", "fetch", "connection failed", cause)

	errStr := err.Error()
	if !containsAll(errStr, "underlying error") {
		t.Errorf("Error() = %s, should contain cause", errStr)
	}
}

func TestCrawlError_Unwrap(t *testing.T) {
	cause := errors.New("root cause")
	err := NewCrawlError(Network, "https://example.com", "fetch", "failed", cause)

	if err.Unwrap() != cause {
		t.Error("Unwrap() should return the cause")
	}
}

func TestCrawlError_Is(t *testing.T) {
	err1 := NewCrawlError(Network, "https://example.com", "fetch", "failed", nil)
	err2 := NewCrawlError(Network, "https://other.com", "request", "timeout", nil)
	err3 := NewCrawlError(Timeout, "https://example.com", "fetch", "timeout", nil)

	if !errors.Is(err1, err2) {
		t.Error("Errors with same type should match")
	}
	if errors.Is(err1, err3) {
		t.Error("Errors with different types should not match")
	}
}

// =============================================================================
// Error Constructor Tests
// =============================================================================

func TestNewNetworkError(t *testing.T) {
	err := NewNetworkError("https://example.com", "connect", nil)

	if err.Type != Network {
		t.Errorf("Type = %v, want Network", err.Type)
	}
	if !err.Retryable {
		t.Error("Network errors should be retryable")
	}
}

func TestNewTimeoutError(t *testing.T) {
	err := NewTimeoutError("https://example.com", "request", nil)

	if err.Type != Timeout {
		t.Errorf("Type = %v, want Timeout", err.Type)
	}
	if !err.Retryable {
		t.Error("Timeout errors should be retryable")
	}
}

func TestNewRateLimitError(t *testing.T) {
	err := NewRateLimitError("https://example.com", 60)

	if err.Type != RateLimit {
		t.Errorf("Type = %v, want RateLimit", err.Type)
	}
	if err.StatusCode != 429 {
		t.Errorf("StatusCode = %d, want 429", err.StatusCode)
	}
	if !err.Retryable {
		t.Error("Rate limit errors should be retryable")
	}
}

func TestNewAuthError(t *testing.T) {
	err := NewAuthError("https://example.com", 401, "unauthorized")

	if err.Type != Auth {
		t.Errorf("Type = %v, want Auth", err.Type)
	}
	if err.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", err.StatusCode)
	}
	if err.Retryable {
		t.Error("Auth errors should not be retryable")
	}
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("https://example.com/missing")

	if err.Type != NotFound {
		t.Errorf("Type = %v, want NotFound", err.Type)
	}
	if err.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", err.StatusCode)
	}
	if err.Retryable {
		t.Error("NotFound errors should not be retryable")
	}
}

func TestNewServerError(t *testing.T) {
	err := NewServerError("https://example.com", 503, "service unavailable")

	if err.Type != ServerError {
		t.Errorf("Type = %v, want ServerError", err.Type)
	}
	if err.StatusCode != 503 {
		t.Errorf("StatusCode = %d, want 503", err.StatusCode)
	}
	if !err.Retryable {
		t.Error("Server errors should be retryable")
	}
}

func TestNewClientError(t *testing.T) {
	err := NewClientError("https://example.com", 400, "bad request")

	if err.Type != ClientError {
		t.Errorf("Type = %v, want ClientError", err.Type)
	}
	if err.Retryable {
		t.Error("Client errors should not be retryable")
	}
}

func TestNewParseError(t *testing.T) {
	err := NewParseError("https://example.com", "html_parse", nil)

	if err.Type != Parse {
		t.Errorf("Type = %v, want Parse", err.Type)
	}
	if err.Retryable {
		t.Error("Parse errors should not be retryable")
	}
}

func TestNewBrowserError(t *testing.T) {
	err := NewBrowserError("https://example.com", "navigate", nil)

	if err.Type != Browser {
		t.Errorf("Type = %v, want Browser", err.Type)
	}
}

func TestNewScopeError(t *testing.T) {
	err := NewScopeError("https://external.com", "out of scope")

	if err.Type != Scope {
		t.Errorf("Type = %v, want Scope", err.Type)
	}
	if err.Retryable {
		t.Error("Scope errors should not be retryable")
	}
}

func TestNewCancelledError(t *testing.T) {
	err := NewCancelledError("https://example.com", "crawl")

	if err.Type != Cancelled {
		t.Errorf("Type = %v, want Cancelled", err.Type)
	}
	if err.Retryable {
		t.Error("Cancelled errors should not be retryable")
	}
}

// =============================================================================
// Categorize Tests
// =============================================================================

func TestCategorize_CrawlError(t *testing.T) {
	original := NewNetworkError("https://example.com", "fetch", nil)
	categorized := Categorize(original, "https://example.com")

	if categorized != original {
		t.Error("Should return same CrawlError")
	}
}

func TestCategorize_Nil(t *testing.T) {
	categorized := Categorize(nil, "https://example.com")

	if categorized != nil {
		t.Error("Should return nil for nil error")
	}
}

func TestCategorize_ContextCanceled(t *testing.T) {
	err := errors.New("context canceled")
	categorized := Categorize(err, "https://example.com")

	if categorized.Type != Cancelled {
		t.Errorf("Type = %v, want Cancelled", categorized.Type)
	}
}

func TestCategorize_Unknown(t *testing.T) {
	err := errors.New("some random error")
	categorized := Categorize(err, "https://example.com")

	if categorized.Type != Unknown {
		t.Errorf("Type = %v, want Unknown", categorized.Type)
	}
}

// =============================================================================
// CategorizeHTTPStatus Tests
// =============================================================================

func TestCategorizeHTTPStatus(t *testing.T) {
	tests := []struct {
		status   int
		wantType ErrorType
		wantNil  bool
	}{
		{200, Unknown, true},
		{201, Unknown, true},
		{301, Unknown, true},
		{401, Auth, false},
		{403, Auth, false},
		{404, NotFound, false},
		{429, RateLimit, false},
		{400, ClientError, false},
		{418, ClientError, false},
		{500, ServerError, false},
		{502, ServerError, false},
		{503, ServerError, false},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.status)), func(t *testing.T) {
			err := CategorizeHTTPStatus(tt.status, "https://example.com")
			if tt.wantNil {
				if err != nil {
					t.Errorf("CategorizeHTTPStatus(%d) should return nil", tt.status)
				}
				return
			}
			if err == nil {
				t.Errorf("CategorizeHTTPStatus(%d) should not return nil", tt.status)
				return
			}
			if err.Type != tt.wantType {
				t.Errorf("CategorizeHTTPStatus(%d).Type = %v, want %v", tt.status, err.Type, tt.wantType)
			}
		})
	}
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"nil", nil, false},
		{"network error", NewNetworkError("url", "op", nil), true},
		{"timeout error", NewTimeoutError("url", "op", nil), true},
		{"auth error", NewAuthError("url", 401, "unauth"), false},
		{"not found", NewNotFoundError("url"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.retryable {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.retryable)
			}
		})
	}
}

func TestIsAuthError(t *testing.T) {
	authErr := NewAuthError("url", 401, "unauthorized")
	networkErr := NewNetworkError("url", "op", nil)

	if !IsAuthError(authErr) {
		t.Error("Should identify auth error")
	}
	if IsAuthError(networkErr) {
		t.Error("Should not identify network error as auth error")
	}
	if IsAuthError(nil) {
		t.Error("Should return false for nil")
	}
}

func TestIsRateLimitError(t *testing.T) {
	rateLimitErr := NewRateLimitError("url", 60)
	networkErr := NewNetworkError("url", "op", nil)

	if !IsRateLimitError(rateLimitErr) {
		t.Error("Should identify rate limit error")
	}
	if IsRateLimitError(networkErr) {
		t.Error("Should not identify network error as rate limit error")
	}
}

func TestGetStatusCode(t *testing.T) {
	err := NewServerError("url", 503, "unavailable")

	if code := GetStatusCode(err); code != 503 {
		t.Errorf("GetStatusCode() = %d, want 503", code)
	}
	if code := GetStatusCode(nil); code != 0 {
		t.Errorf("GetStatusCode(nil) = %d, want 0", code)
	}
}

func TestGetErrorType(t *testing.T) {
	err := NewTimeoutError("url", "op", nil)

	if errType := GetErrorType(err); errType != Timeout {
		t.Errorf("GetErrorType() = %v, want Timeout", errType)
	}
	if errType := GetErrorType(nil); errType != Unknown {
		t.Errorf("GetErrorType(nil) = %v, want Unknown", errType)
	}
}

// =============================================================================
// Retry Tests
// =============================================================================

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.InitialDelay != 500*time.Millisecond {
		t.Errorf("InitialDelay = %v, want 500ms", cfg.InitialDelay)
	}
	if len(cfg.RetryableTypes) == 0 {
		t.Error("RetryableTypes should not be empty")
	}
}

func TestRetrier_Do_Success(t *testing.T) {
	r := NewDefaultRetrier()
	calls := 0

	result := r.Do(context.Background(), "test", "url", func(ctx context.Context) error {
		calls++
		return nil
	})

	if !result.Success {
		t.Error("Should succeed")
	}
	if result.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", result.Attempts)
	}
	if calls != 1 {
		t.Errorf("Function called %d times, want 1", calls)
	}
}

func TestRetrier_Do_RetryOnError(t *testing.T) {
	r := NewRetrier(RetryConfig{
		MaxRetries:     2,
		InitialDelay:   1 * time.Millisecond,
		MaxDelay:       10 * time.Millisecond,
		Multiplier:     2.0,
		RetryableTypes: []ErrorType{Network},
	})

	calls := 0
	result := r.Do(context.Background(), "test", "url", func(ctx context.Context) error {
		calls++
		if calls < 3 {
			return NewNetworkError("url", "op", nil)
		}
		return nil
	})

	if !result.Success {
		t.Error("Should succeed after retries")
	}
	if result.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3", result.Attempts)
	}
}

func TestRetrier_Do_MaxRetriesExceeded(t *testing.T) {
	r := NewRetrier(RetryConfig{
		MaxRetries:     2,
		InitialDelay:   1 * time.Millisecond,
		MaxDelay:       10 * time.Millisecond,
		Multiplier:     2.0,
		RetryableTypes: []ErrorType{Network},
	})

	result := r.Do(context.Background(), "test", "url", func(ctx context.Context) error {
		return NewNetworkError("url", "op", nil)
	})

	if result.Success {
		t.Error("Should fail after max retries")
	}
	if result.Attempts != 3 { // 1 initial + 2 retries
		t.Errorf("Attempts = %d, want 3", result.Attempts)
	}
	if result.LastError == nil {
		t.Error("LastError should be set")
	}
}

func TestRetrier_Do_NoRetryForNonRetryable(t *testing.T) {
	r := NewDefaultRetrier()
	calls := 0

	result := r.Do(context.Background(), "test", "url", func(ctx context.Context) error {
		calls++
		return NewNotFoundError("url") // Not retryable
	})

	if result.Success {
		t.Error("Should fail")
	}
	if calls != 1 {
		t.Errorf("Function called %d times, want 1 (no retry)", calls)
	}
}

func TestRetrier_Do_ContextCancellation(t *testing.T) {
	r := NewRetrier(RetryConfig{
		MaxRetries:     5,
		InitialDelay:   100 * time.Millisecond,
		MaxDelay:       1 * time.Second,
		Multiplier:     2.0,
		RetryableTypes: []ErrorType{Network},
	})

	ctx, cancel := context.WithCancel(context.Background())
	calls := 0

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result := r.Do(ctx, "test", "url", func(ctx context.Context) error {
		calls++
		return NewNetworkError("url", "op", nil)
	})

	if result.Success {
		t.Error("Should fail on cancellation")
	}
	if result.LastError == nil {
		t.Error("LastError should be set")
	}
}

func TestBackoffDuration(t *testing.T) {
	tests := []struct {
		attempt    int
		initial    time.Duration
		max        time.Duration
		multiplier float64
		want       time.Duration
	}{
		{0, time.Second, 10 * time.Second, 2.0, time.Second},
		{1, time.Second, 10 * time.Second, 2.0, time.Second},
		{2, time.Second, 10 * time.Second, 2.0, 2 * time.Second},
		{3, time.Second, 10 * time.Second, 2.0, 4 * time.Second},
		{4, time.Second, 10 * time.Second, 2.0, 8 * time.Second},
		{5, time.Second, 10 * time.Second, 2.0, 10 * time.Second}, // Capped at max
	}

	for _, tt := range tests {
		got := BackoffDuration(tt.attempt, tt.initial, tt.max, tt.multiplier)
		if got != tt.want {
			t.Errorf("BackoffDuration(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestExponentialBackoff(t *testing.T) {
	delays := ExponentialBackoff(5, time.Second, 10*time.Second, 2.0)

	if len(delays) != 5 {
		t.Errorf("len(delays) = %d, want 5", len(delays))
	}

	expected := []time.Duration{
		time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		10 * time.Second, // Capped
	}

	for i, d := range delays {
		if d != expected[i] {
			t.Errorf("delays[%d] = %v, want %v", i, d, expected[i])
		}
	}
}

// =============================================================================
// Circuit Breaker Tests
// =============================================================================

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state CircuitState
		want  string
	}{
		{Closed, "closed"},
		{Open, "open"},
		{HalfOpen, "half-open"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("String() = %v, want %v", got, tt.want)
		}
	}
}

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := NewDefaultCircuitBreaker()

	if cb.State() != Closed {
		t.Errorf("Initial state = %v, want Closed", cb.State())
	}
}

func TestCircuitBreaker_OpenAfterFailures(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		Timeout:          time.Second,
	})

	// Record failures
	for i := 0; i < 3; i++ {
		cb.Allow()
		cb.RecordFailure()
	}

	if cb.State() != Open {
		t.Errorf("State after %d failures = %v, want Open", 3, cb.State())
	}
}

func TestCircuitBreaker_BlockWhenOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		Timeout:          time.Hour, // Long timeout
	})

	cb.Allow()
	cb.RecordFailure()

	if cb.Allow() {
		t.Error("Should not allow requests when open")
	}
}

func TestCircuitBreaker_HalfOpenAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		Timeout:          10 * time.Millisecond,
	})

	cb.Allow()
	cb.RecordFailure()

	time.Sleep(20 * time.Millisecond)

	if !cb.Allow() {
		t.Error("Should allow request after timeout")
	}
	if cb.State() != HalfOpen {
		t.Errorf("State after timeout = %v, want HalfOpen", cb.State())
	}
}

func TestCircuitBreaker_CloseAfterSuccesses(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 2,
		Timeout:          1 * time.Millisecond,
		MaxConcurrent:    10,
	})

	cb.Allow()
	cb.RecordFailure()

	time.Sleep(5 * time.Millisecond)

	// Successes in half-open
	for i := 0; i < 2; i++ {
		cb.Allow()
		cb.RecordSuccess()
	}

	if cb.State() != Closed {
		t.Errorf("State after successes = %v, want Closed", cb.State())
	}
}

func TestCircuitBreaker_ReopenOnFailureInHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 2,
		Timeout:          1 * time.Millisecond,
	})

	cb.Allow()
	cb.RecordFailure()

	time.Sleep(5 * time.Millisecond)

	cb.Allow()
	cb.RecordFailure() // Failure in half-open

	if cb.State() != Open {
		t.Errorf("State after failure in half-open = %v, want Open", cb.State())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		Timeout:          time.Hour,
	})

	cb.Allow()
	cb.RecordFailure()

	cb.Reset()

	if cb.State() != Closed {
		t.Errorf("State after reset = %v, want Closed", cb.State())
	}
}

func TestCircuitBreaker_Execute(t *testing.T) {
	cb := NewDefaultCircuitBreaker()
	calls := 0

	err := cb.Execute(func() error {
		calls++
		return nil
	})

	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if calls != 1 {
		t.Errorf("Function called %d times, want 1", calls)
	}
}

func TestCircuitBreaker_Execute_BlockedWhenOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		Timeout:          time.Hour,
	})

	cb.Allow()
	cb.RecordFailure()

	err := cb.Execute(func() error {
		return nil
	})

	var circuitErr *CircuitOpenError
	if !errors.As(err, &circuitErr) {
		t.Errorf("Execute() should return CircuitOpenError, got %v", err)
	}
}

func TestCircuitBreaker_Stats(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 5,
		Timeout:          time.Second,
	})

	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	stats := cb.Stats()

	if stats.Failures != 2 {
		t.Errorf("Failures = %d, want 2", stats.Failures)
	}
	if stats.State != Closed {
		t.Errorf("State = %v, want Closed", stats.State)
	}
}

func TestHostCircuitBreakers(t *testing.T) {
	hcb := NewHostCircuitBreakers(DefaultCircuitBreakerConfig())

	cb1 := hcb.Get("host1.com")
	cb2 := hcb.Get("host2.com")
	cb1Again := hcb.Get("host1.com")

	if cb1 == cb2 {
		t.Error("Different hosts should have different breakers")
	}
	if cb1 != cb1Again {
		t.Error("Same host should return same breaker")
	}
}

func TestHostCircuitBreakers_AllStats(t *testing.T) {
	hcb := NewHostCircuitBreakers(DefaultCircuitBreakerConfig())

	hcb.Get("host1.com")
	hcb.Get("host2.com")

	stats := hcb.AllStats()

	if len(stats) != 2 {
		t.Errorf("AllStats() returned %d entries, want 2", len(stats))
	}
}

// Helper function
func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Mock net.Error for testing
type mockNetError struct {
	timeout   bool
	temporary bool
}

func (e *mockNetError) Error() string   { return "mock net error" }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return e.temporary }

var _ net.Error = (*mockNetError)(nil)
