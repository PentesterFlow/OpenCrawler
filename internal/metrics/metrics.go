// Package metrics provides metrics collection for the DAST crawler.
package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// Collector collects and aggregates metrics.
type Collector struct {
	mu sync.RWMutex

	// Counters
	requestsTotal   atomic.Int64
	errorsTotal     atomic.Int64
	pagesDiscovered atomic.Int64
	pagesCrawled    atomic.Int64
	formsFound      atomic.Int64
	apiEndpoints    atomic.Int64
	websockets      atomic.Int64
	bytesTotal      atomic.Int64
	retriesTotal    atomic.Int64

	// Rate tracking
	requestsInWindow atomic.Int64
	errorsInWindow   atomic.Int64
	windowStart      atomic.Int64

	// Response time tracking
	responseTimesSum atomic.Int64
	responseTimesNum atomic.Int64

	// Gauges
	queueDepth       atomic.Int64
	activeWorkers    atomic.Int64
	browserPoolSize  atomic.Int64
	browserPoolInUse atomic.Int64

	// Histograms (buckets for response times in ms)
	responseTimeBuckets [10]atomic.Int64 // <10, <50, <100, <250, <500, <1000, <2500, <5000, <10000, >=10000

	// Error breakdown
	errorCounts map[string]*atomic.Int64
	errorMu     sync.RWMutex

	// Status code breakdown
	statusCodes map[int]*atomic.Int64
	statusMu    sync.RWMutex

	// Start time
	startTime time.Time
}

// New creates a new metrics collector.
func New() *Collector {
	now := time.Now()
	c := &Collector{
		errorCounts: make(map[string]*atomic.Int64),
		statusCodes: make(map[int]*atomic.Int64),
		startTime:   now,
	}
	c.windowStart.Store(now.UnixNano())
	return c
}

// RecordRequest records an HTTP request.
func (c *Collector) RecordRequest() {
	c.requestsTotal.Add(1)
	c.requestsInWindow.Add(1)
}

// RecordError records an error.
func (c *Collector) RecordError(errorType string) {
	c.errorsTotal.Add(1)
	c.errorsInWindow.Add(1)

	c.errorMu.Lock()
	if c.errorCounts[errorType] == nil {
		c.errorCounts[errorType] = &atomic.Int64{}
	}
	c.errorCounts[errorType].Add(1)
	c.errorMu.Unlock()
}

// RecordResponseTime records a response time.
func (c *Collector) RecordResponseTime(d time.Duration) {
	ms := d.Milliseconds()
	c.responseTimesSum.Add(ms)
	c.responseTimesNum.Add(1)

	// Update histogram bucket
	bucket := c.getBucket(ms)
	c.responseTimeBuckets[bucket].Add(1)
}

// getBucket returns the histogram bucket for a given response time.
func (c *Collector) getBucket(ms int64) int {
	switch {
	case ms < 10:
		return 0
	case ms < 50:
		return 1
	case ms < 100:
		return 2
	case ms < 250:
		return 3
	case ms < 500:
		return 4
	case ms < 1000:
		return 5
	case ms < 2500:
		return 6
	case ms < 5000:
		return 7
	case ms < 10000:
		return 8
	default:
		return 9
	}
}

// RecordStatusCode records an HTTP status code.
func (c *Collector) RecordStatusCode(code int) {
	c.statusMu.Lock()
	if c.statusCodes[code] == nil {
		c.statusCodes[code] = &atomic.Int64{}
	}
	c.statusCodes[code].Add(1)
	c.statusMu.Unlock()
}

// RecordPageDiscovered increments discovered pages.
func (c *Collector) RecordPageDiscovered() {
	c.pagesDiscovered.Add(1)
}

// RecordPageCrawled increments crawled pages.
func (c *Collector) RecordPageCrawled() {
	c.pagesCrawled.Add(1)
}

// RecordFormFound increments found forms.
func (c *Collector) RecordFormFound() {
	c.formsFound.Add(1)
}

// RecordAPIEndpoint increments API endpoints.
func (c *Collector) RecordAPIEndpoint() {
	c.apiEndpoints.Add(1)
}

// RecordWebSocket increments WebSocket endpoints.
func (c *Collector) RecordWebSocket() {
	c.websockets.Add(1)
}

// RecordBytes records transferred bytes.
func (c *Collector) RecordBytes(n int64) {
	c.bytesTotal.Add(n)
}

// RecordRetry records a retry attempt.
func (c *Collector) RecordRetry() {
	c.retriesTotal.Add(1)
}

// SetQueueDepth sets the current queue depth.
func (c *Collector) SetQueueDepth(depth int64) {
	c.queueDepth.Store(depth)
}

// SetActiveWorkers sets the number of active workers.
func (c *Collector) SetActiveWorkers(n int64) {
	c.activeWorkers.Store(n)
}

// SetBrowserPoolStats sets browser pool statistics.
func (c *Collector) SetBrowserPoolStats(size, inUse int64) {
	c.browserPoolSize.Store(size)
	c.browserPoolInUse.Store(inUse)
}

// GetRequestsPerSecond returns the current requests per second rate.
func (c *Collector) GetRequestsPerSecond() float64 {
	return c.getRatePerSecond(&c.requestsInWindow)
}

// GetErrorsPerSecond returns the current errors per second rate.
func (c *Collector) GetErrorsPerSecond() float64 {
	return c.getRatePerSecond(&c.errorsInWindow)
}

// getRatePerSecond calculates rate per second with window rotation.
func (c *Collector) getRatePerSecond(counter *atomic.Int64) float64 {
	windowDuration := time.Duration(10) * time.Second
	now := time.Now().UnixNano()
	windowStart := c.windowStart.Load()

	elapsed := time.Duration(now - windowStart)
	if elapsed >= windowDuration {
		// Rotate window
		if c.windowStart.CompareAndSwap(windowStart, now) {
			c.requestsInWindow.Store(0)
			c.errorsInWindow.Store(0)
		}
		return 0
	}

	count := counter.Load()
	if elapsed <= 0 {
		return 0
	}

	return float64(count) / elapsed.Seconds()
}

// GetAverageResponseTime returns the average response time.
func (c *Collector) GetAverageResponseTime() time.Duration {
	sum := c.responseTimesSum.Load()
	num := c.responseTimesNum.Load()
	if num == 0 {
		return 0
	}
	return time.Duration(sum/num) * time.Millisecond
}

// Snapshot returns a point-in-time snapshot of all metrics.
func (c *Collector) Snapshot() *Snapshot {
	s := &Snapshot{
		Timestamp:           time.Now(),
		Uptime:              time.Since(c.startTime),
		RequestsTotal:       c.requestsTotal.Load(),
		ErrorsTotal:         c.errorsTotal.Load(),
		PagesDiscovered:     c.pagesDiscovered.Load(),
		PagesCrawled:        c.pagesCrawled.Load(),
		FormsFound:          c.formsFound.Load(),
		APIEndpoints:        c.apiEndpoints.Load(),
		WebSockets:          c.websockets.Load(),
		BytesTotal:          c.bytesTotal.Load(),
		RetriesTotal:        c.retriesTotal.Load(),
		QueueDepth:          c.queueDepth.Load(),
		ActiveWorkers:       c.activeWorkers.Load(),
		BrowserPoolSize:     c.browserPoolSize.Load(),
		BrowserPoolInUse:    c.browserPoolInUse.Load(),
		RequestsPerSecond:   c.GetRequestsPerSecond(),
		ErrorsPerSecond:     c.GetErrorsPerSecond(),
		AverageResponseTime: c.GetAverageResponseTime(),
		ErrorCounts:         make(map[string]int64),
		StatusCodes:         make(map[int]int64),
		ResponseTimeHist:    make([]int64, 10),
	}

	// Copy error counts
	c.errorMu.RLock()
	for k, v := range c.errorCounts {
		s.ErrorCounts[k] = v.Load()
	}
	c.errorMu.RUnlock()

	// Copy status codes
	c.statusMu.RLock()
	for k, v := range c.statusCodes {
		s.StatusCodes[k] = v.Load()
	}
	c.statusMu.RUnlock()

	// Copy histogram
	for i := 0; i < 10; i++ {
		s.ResponseTimeHist[i] = c.responseTimeBuckets[i].Load()
	}

	return s
}

// Reset resets all metrics.
func (c *Collector) Reset() {
	c.requestsTotal.Store(0)
	c.errorsTotal.Store(0)
	c.pagesDiscovered.Store(0)
	c.pagesCrawled.Store(0)
	c.formsFound.Store(0)
	c.apiEndpoints.Store(0)
	c.websockets.Store(0)
	c.bytesTotal.Store(0)
	c.retriesTotal.Store(0)
	c.requestsInWindow.Store(0)
	c.errorsInWindow.Store(0)
	c.responseTimesSum.Store(0)
	c.responseTimesNum.Store(0)
	c.queueDepth.Store(0)
	c.activeWorkers.Store(0)
	c.browserPoolSize.Store(0)
	c.browserPoolInUse.Store(0)

	for i := 0; i < 10; i++ {
		c.responseTimeBuckets[i].Store(0)
	}

	c.errorMu.Lock()
	c.errorCounts = make(map[string]*atomic.Int64)
	c.errorMu.Unlock()

	c.statusMu.Lock()
	c.statusCodes = make(map[int]*atomic.Int64)
	c.statusMu.Unlock()

	c.windowStart.Store(time.Now().UnixNano())
	c.startTime = time.Now()
}

// Snapshot represents a point-in-time view of metrics.
type Snapshot struct {
	Timestamp           time.Time         `json:"timestamp"`
	Uptime              time.Duration     `json:"uptime"`
	RequestsTotal       int64             `json:"requests_total"`
	ErrorsTotal         int64             `json:"errors_total"`
	PagesDiscovered     int64             `json:"pages_discovered"`
	PagesCrawled        int64             `json:"pages_crawled"`
	FormsFound          int64             `json:"forms_found"`
	APIEndpoints        int64             `json:"api_endpoints"`
	WebSockets          int64             `json:"websockets"`
	BytesTotal          int64             `json:"bytes_total"`
	RetriesTotal        int64             `json:"retries_total"`
	QueueDepth          int64             `json:"queue_depth"`
	ActiveWorkers       int64             `json:"active_workers"`
	BrowserPoolSize     int64             `json:"browser_pool_size"`
	BrowserPoolInUse    int64             `json:"browser_pool_in_use"`
	RequestsPerSecond   float64           `json:"requests_per_second"`
	ErrorsPerSecond     float64           `json:"errors_per_second"`
	AverageResponseTime time.Duration     `json:"average_response_time"`
	ErrorCounts         map[string]int64  `json:"error_counts"`
	StatusCodes         map[int]int64     `json:"status_codes"`
	ResponseTimeHist    []int64           `json:"response_time_histogram"`
}

// ErrorRate returns the error rate (errors/requests).
func (s *Snapshot) ErrorRate() float64 {
	if s.RequestsTotal == 0 {
		return 0
	}
	return float64(s.ErrorsTotal) / float64(s.RequestsTotal)
}

// BrowserPoolUtilization returns the browser pool utilization (0-1).
func (s *Snapshot) BrowserPoolUtilization() float64 {
	if s.BrowserPoolSize == 0 {
		return 0
	}
	return float64(s.BrowserPoolInUse) / float64(s.BrowserPoolSize)
}

// Summary returns a human-readable summary.
func (s *Snapshot) Summary() map[string]interface{} {
	return map[string]interface{}{
		"uptime":               s.Uptime.String(),
		"requests_total":       s.RequestsTotal,
		"errors_total":         s.ErrorsTotal,
		"error_rate":           s.ErrorRate(),
		"pages_crawled":        s.PagesCrawled,
		"pages_discovered":     s.PagesDiscovered,
		"queue_depth":          s.QueueDepth,
		"active_workers":       s.ActiveWorkers,
		"requests_per_second":  s.RequestsPerSecond,
		"avg_response_time_ms": s.AverageResponseTime.Milliseconds(),
		"browser_pool_util":    s.BrowserPoolUtilization(),
	}
}

// Global metrics collector.
var globalCollector = New()

// SetGlobal sets the global metrics collector.
func SetGlobal(c *Collector) {
	globalCollector = c
}

// Global returns the global metrics collector.
func Global() *Collector {
	return globalCollector
}
