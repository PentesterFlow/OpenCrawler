// Package ratelimit provides rate limiting functionality for the crawler.
package ratelimit

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter implements rate limiting for crawling.
type Limiter struct {
	mu            sync.RWMutex
	limiter       *rate.Limiter
	perDomain     map[string]*rate.Limiter
	defaultRate   rate.Limit
	defaultBurst  int
	domainDelay   time.Duration
	lastRequest   map[string]time.Time
	robots        *RobotsManager
}

// NewLimiter creates a new rate limiter.
func NewLimiter(requestsPerSecond float64, burst int) *Limiter {
	return &Limiter{
		limiter:      rate.NewLimiter(rate.Limit(requestsPerSecond), burst),
		perDomain:    make(map[string]*rate.Limiter),
		defaultRate:  rate.Limit(requestsPerSecond),
		defaultBurst: burst,
		lastRequest:  make(map[string]time.Time),
		robots:       NewRobotsManager(),
	}
}

// Wait blocks until a request is allowed or context is cancelled.
func (l *Limiter) Wait(ctx context.Context) error {
	return l.limiter.Wait(ctx)
}

// WaitDomain blocks until a request to a specific domain is allowed.
func (l *Limiter) WaitDomain(ctx context.Context, domain string) error {
	// Global rate limit
	if err := l.limiter.Wait(ctx); err != nil {
		return err
	}

	// Per-domain rate limit
	l.mu.Lock()
	domainLimiter, exists := l.perDomain[domain]
	if !exists {
		domainLimiter = rate.NewLimiter(l.defaultRate, l.defaultBurst)
		l.perDomain[domain] = domainLimiter
	}

	// Check domain delay
	if l.domainDelay > 0 {
		if lastReq, ok := l.lastRequest[domain]; ok {
			elapsed := time.Since(lastReq)
			if elapsed < l.domainDelay {
				l.mu.Unlock()
				select {
				case <-time.After(l.domainDelay - elapsed):
				case <-ctx.Done():
					return ctx.Err()
				}
				l.mu.Lock()
			}
		}
		l.lastRequest[domain] = time.Now()
	}
	l.mu.Unlock()

	return domainLimiter.Wait(ctx)
}

// SetDomainRate sets a custom rate limit for a specific domain.
func (l *Limiter) SetDomainRate(domain string, requestsPerSecond float64, burst int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.perDomain[domain] = rate.NewLimiter(rate.Limit(requestsPerSecond), burst)
}

// SetDomainDelay sets the minimum delay between requests to the same domain.
func (l *Limiter) SetDomainDelay(delay time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.domainDelay = delay
}

// Allow checks if a request is allowed without blocking.
func (l *Limiter) Allow() bool {
	return l.limiter.Allow()
}

// AllowDomain checks if a request to a domain is allowed without blocking.
func (l *Limiter) AllowDomain(domain string) bool {
	if !l.limiter.Allow() {
		return false
	}

	l.mu.RLock()
	domainLimiter, exists := l.perDomain[domain]
	l.mu.RUnlock()

	if !exists {
		return true
	}

	return domainLimiter.Allow()
}

// Reserve reserves a token for later use.
func (l *Limiter) Reserve() *rate.Reservation {
	return l.limiter.Reserve()
}

// SetRate updates the global rate limit.
func (l *Limiter) SetRate(requestsPerSecond float64, burst int) {
	l.limiter.SetLimit(rate.Limit(requestsPerSecond))
	l.limiter.SetBurst(burst)
	l.defaultRate = rate.Limit(requestsPerSecond)
	l.defaultBurst = burst
}

// GetRobots returns the robots.txt manager.
func (l *Limiter) GetRobots() *RobotsManager {
	return l.robots
}

// IsAllowed checks if a URL is allowed by rate limit and robots.txt.
func (l *Limiter) IsAllowed(ctx context.Context, domain, path, userAgent string, respectRobots bool) bool {
	if respectRobots && !l.robots.IsAllowed(domain, path, userAgent) {
		return false
	}
	return true
}

// GetCrawlDelay returns the crawl delay for a domain from robots.txt.
func (l *Limiter) GetCrawlDelay(domain, userAgent string) time.Duration {
	return l.robots.GetCrawlDelay(domain, userAgent)
}

// Stats returns rate limiter statistics.
func (l *Limiter) Stats() LimiterStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return LimiterStats{
		DomainCount:  len(l.perDomain),
		DefaultRate:  float64(l.defaultRate),
		DefaultBurst: l.defaultBurst,
		DomainDelay:  l.domainDelay,
	}
}

// LimiterStats contains rate limiter statistics.
type LimiterStats struct {
	DomainCount  int           `json:"domain_count"`
	DefaultRate  float64       `json:"default_rate"`
	DefaultBurst int           `json:"default_burst"`
	DomainDelay  time.Duration `json:"domain_delay"`
}

// AdaptiveRateLimiter adjusts rate based on response times and errors.
type AdaptiveRateLimiter struct {
	*Limiter
	mu           sync.Mutex
	minRate      float64
	maxRate      float64
	currentRate  float64
	errorCount   int
	successCount int
	windowSize   int
}

// NewAdaptiveRateLimiter creates a new adaptive rate limiter.
func NewAdaptiveRateLimiter(minRate, maxRate float64, burst int) *AdaptiveRateLimiter {
	return &AdaptiveRateLimiter{
		Limiter:     NewLimiter(maxRate, burst),
		minRate:     minRate,
		maxRate:     maxRate,
		currentRate: maxRate,
		windowSize:  100,
	}
}

// RecordSuccess records a successful request.
func (a *AdaptiveRateLimiter) RecordSuccess() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.successCount++
	a.checkAndAdjust()
}

// RecordError records a failed request.
func (a *AdaptiveRateLimiter) RecordError() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.errorCount++
	a.checkAndAdjust()
}

// checkAndAdjust adjusts the rate based on success/error ratio.
func (a *AdaptiveRateLimiter) checkAndAdjust() {
	total := a.successCount + a.errorCount
	if total < a.windowSize {
		return
	}

	errorRate := float64(a.errorCount) / float64(total)

	if errorRate > 0.1 {
		// Too many errors, slow down
		a.currentRate = a.currentRate * 0.8
		if a.currentRate < a.minRate {
			a.currentRate = a.minRate
		}
	} else if errorRate < 0.01 {
		// Very few errors, speed up
		a.currentRate = a.currentRate * 1.1
		if a.currentRate > a.maxRate {
			a.currentRate = a.maxRate
		}
	}

	a.SetRate(a.currentRate, a.defaultBurst)

	// Reset counters
	a.successCount = 0
	a.errorCount = 0
}

// CurrentRate returns the current rate.
func (a *AdaptiveRateLimiter) CurrentRate() float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.currentRate
}
