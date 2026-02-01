package errors

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// RetryConfig configures retry behavior.
type RetryConfig struct {
	MaxRetries      int           // Maximum number of retries (0 = no retries)
	InitialDelay    time.Duration // Initial delay before first retry
	MaxDelay        time.Duration // Maximum delay between retries
	Multiplier      float64       // Delay multiplier for exponential backoff
	Jitter          float64       // Random jitter factor (0-1)
	RetryableTypes  []ErrorType   // Error types that should be retried
}

// DefaultRetryConfig returns sensible defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.2,
		RetryableTypes: []ErrorType{
			Network,
			Timeout,
			RateLimit,
			ServerError,
		},
	}
}

// Retrier implements retry logic with exponential backoff.
type Retrier struct {
	config RetryConfig
	rng    *rand.Rand
}

// NewRetrier creates a new retrier.
func NewRetrier(config RetryConfig) *Retrier {
	return &Retrier{
		config: config,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NewDefaultRetrier creates a retrier with default configuration.
func NewDefaultRetrier() *Retrier {
	return NewRetrier(DefaultRetryConfig())
}

// RetryFunc is a function that can be retried.
type RetryFunc func(ctx context.Context) error

// RetryResult holds the result of a retry operation.
type RetryResult struct {
	Attempts  int           // Number of attempts made
	LastError error         // The last error encountered
	Duration  time.Duration // Total time spent retrying
	Success   bool          // Whether the operation succeeded
}

// Do executes the function with retries.
func (r *Retrier) Do(ctx context.Context, operation string, url string, fn RetryFunc) *RetryResult {
	result := &RetryResult{
		Attempts: 0,
	}
	start := time.Now()

	var lastErr error
	delay := r.config.InitialDelay

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		result.Attempts++

		// Execute the function
		err := fn(ctx)
		if err == nil {
			result.Success = true
			result.Duration = time.Since(start)
			return result
		}

		lastErr = err

		// Check if context is cancelled
		if ctx.Err() != nil {
			result.LastError = NewCancelledError(url, operation)
			result.Duration = time.Since(start)
			return result
		}

		// Check if we should retry
		if attempt >= r.config.MaxRetries {
			break
		}

		if !r.shouldRetry(err) {
			break
		}

		// Calculate delay with jitter
		actualDelay := r.calculateDelay(delay)

		// Wait before retry
		select {
		case <-ctx.Done():
			result.LastError = NewCancelledError(url, operation)
			result.Duration = time.Since(start)
			return result
		case <-time.After(actualDelay):
		}

		// Increase delay for next retry
		delay = r.nextDelay(delay)
	}

	result.LastError = lastErr
	result.Duration = time.Since(start)
	return result
}

// shouldRetry checks if an error should be retried.
func (r *Retrier) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	errType := GetErrorType(err)

	// Check against configured retryable types
	for _, t := range r.config.RetryableTypes {
		if errType == t {
			return true
		}
	}

	// Fall back to generic retryable check
	return IsRetryable(err)
}

// calculateDelay calculates the actual delay with jitter.
func (r *Retrier) calculateDelay(baseDelay time.Duration) time.Duration {
	if r.config.Jitter <= 0 {
		return baseDelay
	}

	// Add random jitter
	jitter := r.config.Jitter * float64(baseDelay)
	jitterRange := 2 * jitter
	randomJitter := (r.rng.Float64() * jitterRange) - jitter

	return time.Duration(float64(baseDelay) + randomJitter)
}

// nextDelay calculates the next delay using exponential backoff.
func (r *Retrier) nextDelay(currentDelay time.Duration) time.Duration {
	nextDelay := time.Duration(float64(currentDelay) * r.config.Multiplier)

	if nextDelay > r.config.MaxDelay {
		return r.config.MaxDelay
	}

	return nextDelay
}

// DoWithResult executes a function that returns a value and error.
func DoWithResult[T any](ctx context.Context, r *Retrier, operation, url string, fn func(ctx context.Context) (T, error)) (T, *RetryResult) {
	var result T
	var lastErr error

	retryResult := r.Do(ctx, operation, url, func(ctx context.Context) error {
		var err error
		result, err = fn(ctx)
		lastErr = err
		return err
	})

	if !retryResult.Success {
		retryResult.LastError = lastErr
	}

	return result, retryResult
}

// BackoffDuration calculates the backoff duration for a given attempt.
func BackoffDuration(attempt int, initial, max time.Duration, multiplier float64) time.Duration {
	if attempt <= 0 {
		return initial
	}

	delay := float64(initial) * math.Pow(multiplier, float64(attempt-1))
	if delay > float64(max) {
		return max
	}

	return time.Duration(delay)
}

// ExponentialBackoff returns a sequence of backoff durations.
func ExponentialBackoff(maxRetries int, initial, max time.Duration, multiplier float64) []time.Duration {
	delays := make([]time.Duration, maxRetries)

	for i := 0; i < maxRetries; i++ {
		delays[i] = BackoffDuration(i+1, initial, max, multiplier)
	}

	return delays
}
