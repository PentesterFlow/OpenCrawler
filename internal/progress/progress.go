// Package progress provides progress bar display for the crawler.
package progress

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Display manages progress bar display during crawling.
type Display struct {
	mu      sync.Mutex
	started bool
	stopped bool

	// Stats
	urlsDiscovered  atomic.Int64
	pagesCrawled    atomic.Int64
	formsFound      atomic.Int64
	apiEndpoints    atomic.Int64
	wsEndpoints     atomic.Int64
	errors          atomic.Int64
	queueSize       atomic.Int64

	// Timing
	startTime  time.Time
	lastUpdate time.Time
	target     string

	// Display
	lastLine string
}

// New creates a new progress display.
func New() *Display {
	return &Display{}
}

// Start begins the progress display.
func (d *Display) Start(target string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.started {
		return
	}

	d.started = true
	d.startTime = time.Now()
	d.lastUpdate = time.Now()
	d.target = target
}

// Update updates the progress display with current stats.
func (d *Display) Update(urlsDiscovered, pagesCrawled, formsFound, apiEndpoints, wsEndpoints, errors, queueSize int) {
	d.urlsDiscovered.Store(int64(urlsDiscovered))
	d.pagesCrawled.Store(int64(pagesCrawled))
	d.formsFound.Store(int64(formsFound))
	d.apiEndpoints.Store(int64(apiEndpoints))
	d.wsEndpoints.Store(int64(wsEndpoints))
	d.errors.Store(int64(errors))
	d.queueSize.Store(int64(queueSize))

	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.started || d.stopped {
		return
	}

	// Calculate progress percentage
	total := urlsDiscovered
	if total == 0 {
		total = 1
	}

	progress := 0
	if queueSize == 0 && pagesCrawled > 0 {
		progress = 100
	} else if total > 0 {
		progress = int((float64(pagesCrawled) / float64(total)) * 100)
		if progress > 99 {
			progress = 99
		}
	}

	// Calculate speed
	elapsed := time.Since(d.startTime)
	speed := float64(0)
	if elapsed.Seconds() > 0 {
		speed = float64(pagesCrawled) / elapsed.Seconds()
	}

	// Build progress bar
	barWidth := 30
	filled := int(float64(progress) / 100 * float64(barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	// Build status line
	line := fmt.Sprintf("\r[%s] %3d%% | Pages: %d | Queue: %d | APIs: %d | Forms: %d | %.1f p/s | %s",
		bar, progress, pagesCrawled, queueSize, apiEndpoints, formsFound, speed, formatDuration(elapsed))

	// Clear previous line and print new one
	if len(line) < len(d.lastLine) {
		fmt.Fprint(os.Stderr, "\r"+strings.Repeat(" ", len(d.lastLine)))
	}
	fmt.Fprint(os.Stderr, line)
	d.lastLine = line
	d.lastUpdate = time.Now()
}

// Stop stops the progress display.
func (d *Display) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped || !d.started {
		return
	}

	d.stopped = true

	// Print newline to move past progress bar
	fmt.Fprintln(os.Stderr)
}

// PrintSummary prints a final summary after crawling.
func (d *Display) PrintSummary() {
	duration := time.Since(d.startTime)

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                       Crawl Complete                         ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  Target:              %s\n", truncateURL(d.target, 50))
	fmt.Printf("  Duration:            %s\n", formatDuration(duration))
	fmt.Printf("  URLs Discovered:     %d\n", d.urlsDiscovered.Load())
	fmt.Printf("  Pages Crawled:       %d\n", d.pagesCrawled.Load())
	fmt.Printf("  Forms Found:         %d\n", d.formsFound.Load())
	fmt.Printf("  API Endpoints:       %d\n", d.apiEndpoints.Load())
	fmt.Printf("  WebSocket Endpoints: %d\n", d.wsEndpoints.Load())
	fmt.Printf("  Errors:              %d\n", d.errors.Load())
	fmt.Println()

	// Calculate rates
	if duration.Seconds() > 0 {
		pagesPerSec := float64(d.pagesCrawled.Load()) / duration.Seconds()
		fmt.Printf("  Average Speed:       %.1f pages/sec\n", pagesPerSec)
		fmt.Println()
	}
}

// Stats returns current crawl statistics.
func (d *Display) Stats() (urlsDiscovered, pagesCrawled, formsFound, apiEndpoints, wsEndpoints, errors int64) {
	return d.urlsDiscovered.Load(),
		d.pagesCrawled.Load(),
		d.formsFound.Load(),
		d.apiEndpoints.Load(),
		d.wsEndpoints.Load(),
		d.errors.Load()
}

// truncateURL truncates a URL to maxLen characters.
func truncateURL(url string, maxLen int) string {
	if len(url) <= maxLen {
		return url
	}
	return url[:maxLen-3] + "..."
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
