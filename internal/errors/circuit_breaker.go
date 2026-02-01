package errors

import (
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	// Closed means the circuit is operating normally.
	Closed CircuitState = iota
	// Open means the circuit has tripped and requests are blocked.
	Open
	// HalfOpen means the circuit is testing if it can close again.
	HalfOpen
)

// String returns the string representation of CircuitState.
func (s CircuitState) String() string {
	switch s {
	case Closed:
		return "closed"
	case Open:
		return "open"
	case HalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig configures a circuit breaker.
type CircuitBreakerConfig struct {
	FailureThreshold int           // Number of failures before opening
	SuccessThreshold int           // Number of successes in half-open before closing
	Timeout          time.Duration // Time to wait before trying half-open
	MaxConcurrent    int           // Max concurrent requests in half-open (0 = 1)
}

// DefaultCircuitBreakerConfig returns sensible defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
		MaxConcurrent:    1,
	}
}

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	mu sync.RWMutex

	config CircuitBreakerConfig
	state  CircuitState

	failures         int
	successes        int
	lastFailureTime  time.Time
	halfOpenRequests int

	// Callbacks
	onStateChange func(from, to CircuitState)
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 1
	}

	return &CircuitBreaker{
		config: config,
		state:  Closed,
	}
}

// NewDefaultCircuitBreaker creates a circuit breaker with default configuration.
func NewDefaultCircuitBreaker() *CircuitBreaker {
	return NewCircuitBreaker(DefaultCircuitBreakerConfig())
}

// OnStateChange sets a callback for state changes.
func (cb *CircuitBreaker) OnStateChange(fn func(from, to CircuitState)) {
	cb.mu.Lock()
	cb.onStateChange = fn
	cb.mu.Unlock()
}

// State returns the current state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Allow checks if a request should be allowed.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case Closed:
		return true

	case Open:
		// Check if timeout has passed
		if time.Since(cb.lastFailureTime) >= cb.config.Timeout {
			cb.transitionTo(HalfOpen)
			cb.halfOpenRequests++
			return true
		}
		return false

	case HalfOpen:
		// Allow limited concurrent requests
		if cb.halfOpenRequests < cb.config.MaxConcurrent {
			cb.halfOpenRequests++
			return true
		}
		return false
	}

	return false
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case Closed:
		// Reset failure count on success
		cb.failures = 0

	case HalfOpen:
		cb.successes++
		cb.halfOpenRequests--
		if cb.halfOpenRequests < 0 {
			cb.halfOpenRequests = 0
		}

		// Check if we should close
		if cb.successes >= cb.config.SuccessThreshold {
			cb.transitionTo(Closed)
		}
	}
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailureTime = time.Now()

	switch cb.state {
	case Closed:
		cb.failures++
		if cb.failures >= cb.config.FailureThreshold {
			cb.transitionTo(Open)
		}

	case HalfOpen:
		cb.halfOpenRequests--
		if cb.halfOpenRequests < 0 {
			cb.halfOpenRequests = 0
		}
		// Any failure in half-open reopens the circuit
		cb.transitionTo(Open)
	}
}

// transitionTo transitions to a new state.
func (cb *CircuitBreaker) transitionTo(newState CircuitState) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState

	// Reset counters on state change
	switch newState {
	case Closed:
		cb.failures = 0
		cb.successes = 0
		cb.halfOpenRequests = 0
	case Open:
		cb.successes = 0
		cb.halfOpenRequests = 0
	case HalfOpen:
		cb.successes = 0
	}

	// Notify callback
	if cb.onStateChange != nil {
		cb.onStateChange(oldState, newState)
	}
}

// Reset resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = Closed
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenRequests = 0
}

// Stats returns current statistics.
func (cb *CircuitBreaker) Stats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		State:            cb.state,
		Failures:         cb.failures,
		Successes:        cb.successes,
		LastFailureTime:  cb.lastFailureTime,
		HalfOpenRequests: cb.halfOpenRequests,
	}
}

// CircuitBreakerStats holds circuit breaker statistics.
type CircuitBreakerStats struct {
	State            CircuitState
	Failures         int
	Successes        int
	LastFailureTime  time.Time
	HalfOpenRequests int
}

// Execute executes a function through the circuit breaker.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.Allow() {
		return &CircuitOpenError{State: cb.State()}
	}

	err := fn()
	if err != nil {
		cb.RecordFailure()
		return err
	}

	cb.RecordSuccess()
	return nil
}

// CircuitOpenError is returned when the circuit is open.
type CircuitOpenError struct {
	State CircuitState
}

// Error implements the error interface.
func (e *CircuitOpenError) Error() string {
	return "circuit breaker is " + e.State.String()
}

// HostCircuitBreakers manages circuit breakers per host.
type HostCircuitBreakers struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
	config   CircuitBreakerConfig
}

// NewHostCircuitBreakers creates a new host circuit breaker manager.
func NewHostCircuitBreakers(config CircuitBreakerConfig) *HostCircuitBreakers {
	return &HostCircuitBreakers{
		breakers: make(map[string]*CircuitBreaker),
		config:   config,
	}
}

// Get returns the circuit breaker for a host, creating one if needed.
func (hcb *HostCircuitBreakers) Get(host string) *CircuitBreaker {
	hcb.mu.RLock()
	cb, ok := hcb.breakers[host]
	hcb.mu.RUnlock()

	if ok {
		return cb
	}

	// Create new circuit breaker
	hcb.mu.Lock()
	defer hcb.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, ok = hcb.breakers[host]; ok {
		return cb
	}

	cb = NewCircuitBreaker(hcb.config)
	hcb.breakers[host] = cb
	return cb
}

// AllStats returns statistics for all hosts.
func (hcb *HostCircuitBreakers) AllStats() map[string]CircuitBreakerStats {
	hcb.mu.RLock()
	defer hcb.mu.RUnlock()

	stats := make(map[string]CircuitBreakerStats, len(hcb.breakers))
	for host, cb := range hcb.breakers {
		stats[host] = cb.Stats()
	}
	return stats
}

// Reset resets all circuit breakers.
func (hcb *HostCircuitBreakers) Reset() {
	hcb.mu.Lock()
	defer hcb.mu.Unlock()

	for _, cb := range hcb.breakers {
		cb.Reset()
	}
}
