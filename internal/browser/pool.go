package browser

import (
	"context"
	"fmt"
	"net/http"
	"sync"
)

// Pool manages a pool of browser instances.
type Pool struct {
	mu       sync.Mutex
	browsers []*Browser
	config   Config
	size     int
	current  int
	closed   bool
	sem      chan struct{}
}

// NewPool creates a new browser pool.
func NewPool(config Config) (*Pool, error) {
	if config.PoolSize < 1 {
		config.PoolSize = 1
	}

	pool := &Pool{
		browsers: make([]*Browser, config.PoolSize),
		config:   config,
		size:     config.PoolSize,
		sem:      make(chan struct{}, config.PoolSize),
	}

	// Initialize semaphore
	for i := 0; i < config.PoolSize; i++ {
		pool.sem <- struct{}{}
	}

	// Pre-create browsers
	for i := 0; i < config.PoolSize; i++ {
		browser, err := New(config)
		if err != nil {
			pool.Close()
			return nil, fmt.Errorf("failed to create browser %d: %w", i, err)
		}
		pool.browsers[i] = browser
	}

	return pool, nil
}

// Acquire gets a browser from the pool.
func (p *Pool) Acquire(ctx context.Context) (*Browser, error) {
	// Wait for available slot
	select {
	case <-p.sem:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		p.sem <- struct{}{} // Return token
		return nil, fmt.Errorf("pool is closed")
	}

	// Get next browser
	browser := p.browsers[p.current]
	p.current = (p.current + 1) % p.size

	// Check if browser needs recycling
	if browser.NeedsRecycle() {
		browser.Close()
		newBrowser, err := New(p.config)
		if err != nil {
			p.sem <- struct{}{} // Return token
			return nil, fmt.Errorf("failed to recycle browser: %w", err)
		}
		p.browsers[p.current] = newBrowser
		browser = newBrowser
	}

	return browser, nil
}

// Release returns a browser to the pool.
func (p *Pool) Release(browser *Browser) {
	p.sem <- struct{}{}
}

// Visit acquires a browser, visits the URL, and releases it.
func (p *Pool) Visit(ctx context.Context, url string, headers map[string]string, cookies []*http.Cookie) (*PageResult, error) {
	browser, err := p.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer p.Release(browser)

	return browser.Visit(ctx, url, headers, cookies)
}

// VisitWithOptions acquires a browser, visits the URL with options, and releases it.
func (p *Pool) VisitWithOptions(ctx context.Context, url string, headers map[string]string, cookies []*http.Cookie, opts VisitOptions) (*PageResult, error) {
	browser, err := p.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer p.Release(browser)

	return browser.VisitWithOptions(ctx, url, headers, cookies, opts)
}

// VisitHashRoute visits a hash-based route within an SPA.
func (p *Pool) VisitHashRoute(ctx context.Context, baseURL string, hashRoute string, headers map[string]string, cookies []*http.Cookie) (*PageResult, error) {
	browser, err := p.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer p.Release(browser)

	return browser.VisitHashRoute(ctx, baseURL, hashRoute, headers, cookies)
}

// VisitHashRouteWithOptions visits a hash-based route with options.
func (p *Pool) VisitHashRouteWithOptions(ctx context.Context, baseURL string, hashRoute string, headers map[string]string, cookies []*http.Cookie, opts VisitOptions) (*PageResult, error) {
	browser, err := p.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer p.Release(browser)

	return browser.VisitHashRouteWithOptions(ctx, baseURL, hashRoute, headers, cookies, opts)
}

// Close closes all browsers in the pool.
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true

	var lastErr error
	for _, browser := range p.browsers {
		if browser != nil {
			if err := browser.Close(); err != nil {
				lastErr = err
			}
		}
	}

	close(p.sem)
	return lastErr
}

// Size returns the pool size.
func (p *Pool) Size() int {
	return p.size
}

// Stats returns pool statistics.
type PoolStats struct {
	Size       int `json:"size"`
	Available  int `json:"available"`
	TotalPages int `json:"total_pages"`
}

// Stats returns pool statistics.
func (p *Pool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	totalPages := 0
	for _, b := range p.browsers {
		if b != nil {
			totalPages += b.PageCount()
		}
	}

	return PoolStats{
		Size:       p.size,
		Available:  len(p.sem),
		TotalPages: totalPages,
	}
}
