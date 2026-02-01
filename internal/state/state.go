// Package state provides state management for the crawler.
package state

import (
	"time"
)

// Manager handles crawler state.
type Manager struct {
	store         Store
	dedup         *Deduplicator
	hashDedup     *HashAwareDeduplicator
	stats         *CrawlStats
	startTime     time.Time
	target        string
	softErrorURLs map[string]string // URL -> error message for soft 404s
}

// NewManager creates a new state manager.
func NewManager(store Store, estimatedURLs int) *Manager {
	return &Manager{
		store:         store,
		dedup:         NewDeduplicator(estimatedURLs),
		hashDedup:     NewHashAwareDeduplicator(estimatedURLs),
		stats:         &CrawlStats{},
		startTime:     time.Now(),
		softErrorURLs: make(map[string]string),
	}
}

// SetTarget sets the crawl target.
func (m *Manager) SetTarget(target string) {
	m.target = target
}

// Start initializes the state for a new crawl.
func (m *Manager) Start(target string) {
	m.target = target
	m.startTime = time.Now()
	m.stats = &CrawlStats{}
}

// MarkVisited marks a URL as visited.
func (m *Manager) MarkVisited(url string) bool {
	if m.dedup.HasSeen(url) {
		return false
	}
	m.dedup.Add(url)
	m.stats.PagesCrawled++
	return true
}

// HasVisited checks if a URL has been visited.
func (m *Manager) HasVisited(url string) bool {
	return m.dedup.HasSeen(url)
}

// AddDiscoveredURL increments the discovered URL counter.
func (m *Manager) AddDiscoveredURL() {
	m.stats.URLsDiscovered++
}

// AddForm increments the form counter.
func (m *Manager) AddForm() {
	m.stats.FormsFound++
}

// AddAPIEndpoint increments the API endpoint counter.
func (m *Manager) AddAPIEndpoint() {
	m.stats.APIEndpoints++
}

// AddWebSocket increments the WebSocket endpoint counter.
func (m *Manager) AddWebSocket() {
	m.stats.WebSocketEndpoints++
}

// AddError increments the error counter.
func (m *Manager) AddError() {
	m.stats.ErrorCount++
}

// AddBytes adds to the bytes transferred counter.
func (m *Manager) AddBytes(n int64) {
	m.stats.BytesTransferred += n
}

// GetStats returns the current statistics.
func (m *Manager) GetStats() CrawlStats {
	stats := *m.stats
	stats.Duration = time.Since(m.startTime)
	return stats
}

// Save saves the current state.
func (m *Manager) Save(state *CrawlerState) error {
	if m.store == nil {
		return nil
	}
	state.UpdatedAt = time.Now()
	return m.store.Save(state)
}

// Load loads the state from storage.
func (m *Manager) Load() (*CrawlerState, error) {
	if m.store == nil {
		return nil, nil
	}
	return m.store.Load()
}

// Reset resets the state manager.
func (m *Manager) Reset() {
	m.dedup.Reset()
	m.stats = &CrawlStats{}
	m.startTime = time.Now()
}

// GetDeduplicator returns the deduplicator.
func (m *Manager) GetDeduplicator() *Deduplicator {
	return m.dedup
}

// GetHashDeduplicator returns the hash-aware deduplicator.
func (m *Manager) GetHashDeduplicator() *HashAwareDeduplicator {
	return m.hashDedup
}

// HasDuplicateContent checks if the content hash has been seen before.
// Returns true if duplicate, along with the URL that had the same content.
func (m *Manager) HasDuplicateContent(url, contentHash string) (bool, string) {
	if m.hashDedup == nil || contentHash == "" {
		return false, ""
	}
	return m.hashDedup.HasDuplicateContent(url, contentHash)
}

// SetContentHash stores the content hash for a URL.
func (m *Manager) SetContentHash(url, contentHash string) {
	if m.hashDedup == nil || contentHash == "" {
		return
	}
	m.hashDedup.SetContentHash(url, contentHash)
}

// MarkSoftError records a URL as a soft 404.
func (m *Manager) MarkSoftError(url, errorMsg string) {
	if m.softErrorURLs == nil {
		m.softErrorURLs = make(map[string]string)
	}
	m.softErrorURLs[url] = errorMsg
}

// IsSoftError checks if a URL was marked as a soft 404.
func (m *Manager) IsSoftError(url string) (bool, string) {
	if m.softErrorURLs == nil {
		return false, ""
	}
	msg, exists := m.softErrorURLs[url]
	return exists, msg
}

// GetSoftErrors returns all soft error URLs.
func (m *Manager) GetSoftErrors() map[string]string {
	if m.softErrorURLs == nil {
		return make(map[string]string)
	}
	result := make(map[string]string, len(m.softErrorURLs))
	for k, v := range m.softErrorURLs {
		result[k] = v
	}
	return result
}

// ShouldSkipFragment checks if a hash fragment should be skipped (UI state).
func (m *Manager) ShouldSkipFragment(fragment string) bool {
	if m.hashDedup == nil {
		return false
	}
	return m.hashDedup.ShouldSkipFragment(fragment)
}

// NormalizeURL normalizes a URL for deduplication (handles hash-based SPAs).
func (m *Manager) NormalizeURL(url string) string {
	if m.hashDedup == nil {
		return url
	}
	return m.hashDedup.NormalizeURL(url)
}

// Store defines the interface for state storage.
type Store interface {
	Save(state *CrawlerState) error
	Load() (*CrawlerState, error)
	Close() error
}
