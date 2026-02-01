package state

import (
	"sync"

	"github.com/bits-and-blooms/bloom/v3"
)

// Deduplicator handles URL deduplication using a Bloom filter.
type Deduplicator struct {
	mu     sync.RWMutex
	filter *bloom.BloomFilter
	exact  map[string]struct{} // For exact matching when Bloom filter might give false positives
	count  int
	fpRate float64
}

// NewDeduplicator creates a new deduplicator.
func NewDeduplicator(estimatedItems int) *Deduplicator {
	if estimatedItems < 1000 {
		estimatedItems = 1000
	}

	fpRate := 0.001 // 0.1% false positive rate

	return &Deduplicator{
		filter: bloom.NewWithEstimates(uint(estimatedItems), fpRate),
		exact:  make(map[string]struct{}),
		fpRate: fpRate,
	}
}

// Add adds a URL to the deduplicator.
func (d *Deduplicator) Add(url string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Only increment count if URL is new
	if _, exists := d.exact[url]; !exists {
		d.filter.AddString(url)
		d.exact[url] = struct{}{}
		d.count++
	}
}

// HasSeen checks if a URL has been seen before.
func (d *Deduplicator) HasSeen(url string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Fast check with Bloom filter
	if !d.filter.TestString(url) {
		return false
	}

	// Exact check for potential false positives
	_, exists := d.exact[url]
	return exists
}

// Count returns the number of unique URLs seen.
func (d *Deduplicator) Count() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.count
}

// Reset resets the deduplicator.
func (d *Deduplicator) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.filter.ClearAll()
	d.exact = make(map[string]struct{})
	d.count = 0
}

// GetAll returns all URLs in the deduplicator.
func (d *Deduplicator) GetAll() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	urls := make([]string, 0, len(d.exact))
	for url := range d.exact {
		urls = append(urls, url)
	}
	return urls
}

// AddBatch adds multiple URLs at once.
func (d *Deduplicator) AddBatch(urls []string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, url := range urls {
		if _, exists := d.exact[url]; !exists {
			d.filter.AddString(url)
			d.exact[url] = struct{}{}
			d.count++
		}
	}
}

// Merge merges another deduplicator into this one.
func (d *Deduplicator) Merge(other *Deduplicator) {
	other.mu.RLock()
	urls := make([]string, 0, len(other.exact))
	for url := range other.exact {
		urls = append(urls, url)
	}
	other.mu.RUnlock()

	d.AddBatch(urls)
}

// FalsePositiveRate returns the current estimated false positive rate.
func (d *Deduplicator) FalsePositiveRate() float64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	// The bloom filter library calculates FPR based on its internal state
	return d.fpRate
}
