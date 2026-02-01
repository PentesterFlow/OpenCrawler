package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// =============================================================================
// Deduplicator Tests
// =============================================================================

func TestDeduplicator_New(t *testing.T) {
	tests := []struct {
		name         string
		estimatedURLs int
	}{
		{"small", 100},
		{"medium", 10000},
		{"large", 100000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDeduplicator(tt.estimatedURLs)
			if d == nil {
				t.Fatal("NewDeduplicator returned nil")
			}
			if d.Count() != 0 {
				t.Errorf("New deduplicator count = %v, want 0", d.Count())
			}
		})
	}
}

func TestDeduplicator_Add(t *testing.T) {
	d := NewDeduplicator(1000)

	url := "https://example.com/test"

	// First add should succeed
	if d.HasSeen(url) {
		t.Error("URL should not be seen before adding")
	}

	d.Add(url)

	if !d.HasSeen(url) {
		t.Error("URL should be seen after adding")
	}

	if d.Count() != 1 {
		t.Errorf("Count = %v, want 1", d.Count())
	}
}

func TestDeduplicator_Duplicates(t *testing.T) {
	d := NewDeduplicator(1000)

	url := "https://example.com/test"

	d.Add(url)
	d.Add(url) // Add same URL again

	if d.Count() != 1 {
		t.Errorf("Count after duplicate = %v, want 1", d.Count())
	}
}

func TestDeduplicator_MultipleURLs(t *testing.T) {
	d := NewDeduplicator(1000)

	urls := []string{
		"https://example.com/page1",
		"https://example.com/page2",
		"https://example.com/page3",
		"https://different.com/page",
	}

	for _, url := range urls {
		d.Add(url)
	}

	if d.Count() != len(urls) {
		t.Errorf("Count = %v, want %v", d.Count(), len(urls))
	}

	for _, url := range urls {
		if !d.HasSeen(url) {
			t.Errorf("URL %s should be seen", url)
		}
	}
}

func TestDeduplicator_Reset(t *testing.T) {
	d := NewDeduplicator(1000)

	d.Add("https://example.com/1")
	d.Add("https://example.com/2")

	if d.Count() != 2 {
		t.Fatalf("Count before reset = %v, want 2", d.Count())
	}

	d.Reset()

	if d.Count() != 0 {
		t.Errorf("Count after reset = %v, want 0", d.Count())
	}

	if d.HasSeen("https://example.com/1") {
		t.Error("URL should not be seen after reset")
	}
}

func TestDeduplicator_GetAll(t *testing.T) {
	d := NewDeduplicator(1000)

	urls := []string{
		"https://example.com/a",
		"https://example.com/b",
		"https://example.com/c",
	}

	for _, url := range urls {
		d.Add(url)
	}

	all := d.GetAll()
	if len(all) != len(urls) {
		t.Errorf("GetAll() returned %d items, want %d", len(all), len(urls))
	}

	// Check all URLs are present
	urlMap := make(map[string]bool)
	for _, u := range all {
		urlMap[u] = true
	}
	for _, u := range urls {
		if !urlMap[u] {
			t.Errorf("URL %s not found in GetAll()", u)
		}
	}
}

func TestDeduplicator_AddBatch(t *testing.T) {
	d := NewDeduplicator(1000)

	urls := []string{
		"https://example.com/1",
		"https://example.com/2",
		"https://example.com/3",
	}

	d.AddBatch(urls)

	if d.Count() != len(urls) {
		t.Errorf("Count after AddBatch = %v, want %v", d.Count(), len(urls))
	}

	for _, url := range urls {
		if !d.HasSeen(url) {
			t.Errorf("URL %s should be seen after AddBatch", url)
		}
	}
}

// =============================================================================
// HashAwareDeduplicator Tests
// =============================================================================

func TestHashAwareDeduplicator_New(t *testing.T) {
	d := NewHashAwareDeduplicator(1000)
	if d == nil {
		t.Fatal("NewHashAwareDeduplicator returned nil")
	}
}

func TestHashAwareDeduplicator_NormalizeURL(t *testing.T) {
	d := NewHashAwareDeduplicator(1000)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "add trailing slash normalization",
			input:    "https://example.com/path",
			expected: "https://example.com/path",
		},
		{
			name:     "remove default http port",
			input:    "http://example.com:80/path",
			expected: "http://example.com/path",
		},
		{
			name:     "remove default https port",
			input:    "https://example.com:443/path",
			expected: "https://example.com/path",
		},
		{
			name:     "lowercase scheme",
			input:    "HTTPS://example.com/path",
			expected: "https://example.com/path",
		},
		{
			name:     "lowercase host",
			input:    "https://EXAMPLE.COM/path",
			expected: "https://example.com/path",
		},
		{
			name:     "remove utm parameters",
			input:    "https://example.com/path?utm_source=test&id=1",
			expected: "https://example.com/path?id=1",
		},
		{
			name:     "sort query parameters",
			input:    "https://example.com/path?z=1&a=2&m=3",
			expected: "https://example.com/path?a=2&m=3&z=1",
		},
		{
			name:     "normalize hash route",
			input:    "https://example.com/#/dashboard",
			expected: "https://example.com/#/dashboard",
		},
		{
			name:     "remove hashbang prefix",
			input:    "https://example.com/#!/route",
			expected: "https://example.com/#/route",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.NormalizeURL(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeURL(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHashAwareDeduplicator_MarkVisited(t *testing.T) {
	d := NewHashAwareDeduplicator(1000)

	url := "https://example.com/test"

	if d.HasVisited(url) {
		t.Error("URL should not be visited initially")
	}

	d.MarkVisited(url)

	if !d.HasVisited(url) {
		t.Error("URL should be visited after marking")
	}
}

func TestHashAwareDeduplicator_ContentHash(t *testing.T) {
	d := NewHashAwareDeduplicator(1000)

	url := "https://example.com/page"
	contentHash := "abc123hash"

	// Initially no content hash
	_, exists := d.GetContentHash(url)
	if exists {
		t.Error("Content hash should not exist initially")
	}

	d.SetContentHash(url, contentHash)

	hash, exists := d.GetContentHash(url)
	if !exists {
		t.Error("Content hash should exist after setting")
	}
	if hash != contentHash {
		t.Errorf("Content hash = %s, want %s", hash, contentHash)
	}
}

func TestHashAwareDeduplicator_DuplicateContent(t *testing.T) {
	d := NewHashAwareDeduplicator(1000)

	url1 := "https://example.com/page1"
	url2 := "https://example.com/page2"
	sameContentHash := "samehash123"

	d.SetContentHash(url1, sameContentHash)

	// Check if url2 has duplicate content
	isDupe, dupeURL := d.HasDuplicateContent(url2, sameContentHash)
	if !isDupe {
		t.Error("Should detect duplicate content")
	}
	if dupeURL != d.NormalizeURL(url1) {
		t.Errorf("Duplicate URL = %s, want %s", dupeURL, url1)
	}
}

func TestHashAwareDeduplicator_ShouldSkipFragment(t *testing.T) {
	d := NewHashAwareDeduplicator(1000)

	tests := []struct {
		fragment   string
		shouldSkip bool
	}{
		{"modal-123", true},
		{"popup-open", true},
		{"tab-settings", true},
		{"scroll123", true},
		{"page-5", true},
		{"12345", true},
		{"/dashboard", false},
		{"users/profile", false},
		{"settings.account", false},
	}

	for _, tt := range tests {
		t.Run(tt.fragment, func(t *testing.T) {
			result := d.ShouldSkipFragment(tt.fragment)
			if result != tt.shouldSkip {
				t.Errorf("ShouldSkipFragment(%s) = %v, want %v", tt.fragment, result, tt.shouldSkip)
			}
		})
	}
}

func TestHashAwareDeduplicator_Stats(t *testing.T) {
	d := NewHashAwareDeduplicator(1000)

	d.MarkVisited("https://example.com/1")
	d.MarkVisited("https://example.com/2")
	d.SetContentHash("https://example.com/1", "hash1")

	stats := d.Stats()

	if stats["visited_urls"] != 2 {
		t.Errorf("visited_urls = %v, want 2", stats["visited_urls"])
	}
	if stats["content_hashes"] != 1 {
		t.Errorf("content_hashes = %v, want 1", stats["content_hashes"])
	}
}

func TestComputeContentHash(t *testing.T) {
	content := "Hello, World!"
	hash := ComputeContentHash(content)

	if hash == "" {
		t.Error("ComputeContentHash returned empty string")
	}

	// Same content should produce same hash
	hash2 := ComputeContentHash(content)
	if hash != hash2 {
		t.Error("Same content should produce same hash")
	}

	// Different content should produce different hash
	hash3 := ComputeContentHash("Different content")
	if hash == hash3 {
		t.Error("Different content should produce different hash")
	}
}

// =============================================================================
// Manager Tests
// =============================================================================

func TestManager_New(t *testing.T) {
	m := NewManager(nil, 1000)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestManager_MarkVisited(t *testing.T) {
	m := NewManager(nil, 1000)

	url := "https://example.com/test"

	if m.HasVisited(url) {
		t.Error("URL should not be visited initially")
	}

	result := m.MarkVisited(url)
	if !result {
		t.Error("MarkVisited should return true for new URL")
	}

	if !m.HasVisited(url) {
		t.Error("URL should be visited after marking")
	}

	// Marking again should return false
	result = m.MarkVisited(url)
	if result {
		t.Error("MarkVisited should return false for duplicate URL")
	}
}

func TestManager_Stats(t *testing.T) {
	m := NewManager(nil, 1000)

	m.MarkVisited("https://example.com/1")
	m.MarkVisited("https://example.com/2")
	m.AddDiscoveredURL()
	m.AddDiscoveredURL()
	m.AddForm()
	m.AddAPIEndpoint()
	m.AddWebSocket()
	m.AddError()
	m.AddBytes(1024)

	stats := m.GetStats()

	if stats.PagesCrawled != 2 {
		t.Errorf("PagesCrawled = %v, want 2", stats.PagesCrawled)
	}
	if stats.URLsDiscovered != 2 {
		t.Errorf("URLsDiscovered = %v, want 2", stats.URLsDiscovered)
	}
	if stats.FormsFound != 1 {
		t.Errorf("FormsFound = %v, want 1", stats.FormsFound)
	}
	if stats.APIEndpoints != 1 {
		t.Errorf("APIEndpoints = %v, want 1", stats.APIEndpoints)
	}
	if stats.WebSocketEndpoints != 1 {
		t.Errorf("WebSocketEndpoints = %v, want 1", stats.WebSocketEndpoints)
	}
	if stats.ErrorCount != 1 {
		t.Errorf("ErrorCount = %v, want 1", stats.ErrorCount)
	}
	if stats.BytesTransferred != 1024 {
		t.Errorf("BytesTransferred = %v, want 1024", stats.BytesTransferred)
	}
}

func TestManager_Reset(t *testing.T) {
	m := NewManager(nil, 1000)

	m.MarkVisited("https://example.com/1")
	m.AddForm()
	m.AddError()

	m.Reset()

	if m.HasVisited("https://example.com/1") {
		t.Error("URL should not be visited after reset")
	}

	stats := m.GetStats()
	if stats.FormsFound != 0 || stats.ErrorCount != 0 {
		t.Error("Stats should be reset")
	}
}

func TestManager_GetDeduplicator(t *testing.T) {
	m := NewManager(nil, 1000)

	d := m.GetDeduplicator()
	if d == nil {
		t.Error("GetDeduplicator should not return nil")
	}
}

func TestManager_GetHashDeduplicator(t *testing.T) {
	m := NewManager(nil, 1000)

	d := m.GetHashDeduplicator()
	if d == nil {
		t.Error("GetHashDeduplicator should not return nil")
	}
}

func TestManager_SoftErrors(t *testing.T) {
	m := NewManager(nil, 1000)

	url := "https://example.com/notfound"
	errorMsg := "Page not found"

	// Initially no soft error
	isSoftError, _ := m.IsSoftError(url)
	if isSoftError {
		t.Error("URL should not be soft error initially")
	}

	m.MarkSoftError(url, errorMsg)

	isSoftError, msg := m.IsSoftError(url)
	if !isSoftError {
		t.Error("URL should be soft error after marking")
	}
	if msg != errorMsg {
		t.Errorf("Error message = %s, want %s", msg, errorMsg)
	}

	softErrors := m.GetSoftErrors()
	if len(softErrors) != 1 {
		t.Errorf("GetSoftErrors() returned %d items, want 1", len(softErrors))
	}
}

func TestManager_ContentDedup(t *testing.T) {
	m := NewManager(nil, 1000)

	url := "https://example.com/page"
	contentHash := "hash123"

	// Initially no duplicate
	isDupe, _ := m.HasDuplicateContent(url, contentHash)
	if isDupe {
		t.Error("Should not be duplicate initially")
	}

	m.SetContentHash(url, contentHash)

	// Same URL is not a duplicate of itself
	isDupe, _ = m.HasDuplicateContent(url, contentHash)
	if isDupe {
		t.Error("URL should not be duplicate of itself")
	}

	// Different URL with same content is a duplicate
	isDupe, dupeURL := m.HasDuplicateContent("https://example.com/other", contentHash)
	if !isDupe {
		t.Error("Should detect duplicate content")
	}
	if dupeURL == "" {
		t.Error("Should return duplicate URL")
	}
}

// =============================================================================
// BoltStore Tests
// =============================================================================

func TestBoltStore_NewAndClose(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewBoltStore(dbPath)
	if err != nil {
		t.Fatalf("NewBoltStore() error = %v", err)
	}

	if store == nil {
		t.Fatal("NewBoltStore returned nil")
	}

	err = store.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}
}

func TestBoltStore_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewBoltStore(dbPath)
	if err != nil {
		t.Fatalf("NewBoltStore() error = %v", err)
	}
	defer store.Close()

	// Create state
	state := &CrawlerState{
		Target:      "https://example.com",
		StartedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		QueueURLs:   []string{"https://example.com/1", "https://example.com/2"},
		VisitedURLs: []string{"https://example.com/visited"},
		Stats: CrawlStats{
			PagesCrawled:   10,
			URLsDiscovered: 50,
			FormsFound:     5,
		},
	}

	// Save
	err = store.Save(state)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded == nil {
		t.Fatal("Load() returned nil")
	}

	// Verify
	if loaded.Target != state.Target {
		t.Errorf("Target = %s, want %s", loaded.Target, state.Target)
	}
	if len(loaded.QueueURLs) != len(state.QueueURLs) {
		t.Errorf("QueueURLs length = %d, want %d", len(loaded.QueueURLs), len(state.QueueURLs))
	}
	if loaded.Stats.PagesCrawled != state.Stats.PagesCrawled {
		t.Errorf("PagesCrawled = %d, want %d", loaded.Stats.PagesCrawled, state.Stats.PagesCrawled)
	}
}

func TestBoltStore_LoadEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "empty.db")

	store, err := NewBoltStore(dbPath)
	if err != nil {
		t.Fatalf("NewBoltStore() error = %v", err)
	}
	defer store.Close()

	// Load from empty store
	state, err := store.Load()
	if err != nil {
		t.Errorf("Load() error = %v", err)
	}

	if state != nil {
		t.Error("Load() from empty store should return nil")
	}
}

// =============================================================================
// FileStore Tests
// =============================================================================

func TestFileStore_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "state.json")

	store := NewFileStore(filePath, false)

	state := &CrawlerState{
		Target:    "https://example.com",
		StartedAt: time.Now(),
		Stats: CrawlStats{
			PagesCrawled:   5,
			URLsDiscovered: 20,
		},
	}

	// Save
	err := store.Save(state)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Target != state.Target {
		t.Errorf("Target = %s, want %s", loaded.Target, state.Target)
	}
	if loaded.Stats.PagesCrawled != state.Stats.PagesCrawled {
		t.Errorf("PagesCrawled = %d, want %d", loaded.Stats.PagesCrawled, state.Stats.PagesCrawled)
	}
}

func TestFileStore_LoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nonexistent.json")

	store := NewFileStore(filePath, false)

	state, err := store.Load()
	if err != nil {
		t.Errorf("Load() error = %v", err)
	}
	if state != nil {
		t.Error("Load() from non-existent file should return nil")
	}
}

func TestFileStore_Compressed(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "state")

	store := NewFileStore(filePath, true)

	state := &CrawlerState{
		Target:    "https://example.com",
		StartedAt: time.Now(),
		Stats: CrawlStats{
			PagesCrawled: 10,
		},
	}

	// Save compressed
	err := store.Save(state)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify .gz file was created
	if _, err := os.Stat(filePath + ".gz"); os.IsNotExist(err) {
		t.Error("Compressed file was not created")
	}

	// Load compressed
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Target != state.Target {
		t.Errorf("Target = %s, want %s", loaded.Target, state.Target)
	}
}

func TestFileStore_Close(t *testing.T) {
	store := NewFileStore("/tmp/test.json", false)
	err := store.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// =============================================================================
// MemoryStore Tests
// =============================================================================

func TestMemoryStore_SaveAndLoad(t *testing.T) {
	store := NewMemoryStore()

	state := &CrawlerState{
		Target: "https://example.com",
		Stats: CrawlStats{
			PagesCrawled: 3,
		},
	}

	err := store.Save(state)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded != state {
		t.Error("Load() should return the same pointer")
	}
}

func TestMemoryStore_LoadEmpty(t *testing.T) {
	store := NewMemoryStore()

	state, err := store.Load()
	if err != nil {
		t.Errorf("Load() error = %v", err)
	}
	if state != nil {
		t.Error("Load() from empty store should return nil")
	}
}

func TestMemoryStore_Close(t *testing.T) {
	store := NewMemoryStore()
	err := store.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// =============================================================================
// Manager Additional Tests
// =============================================================================

func TestManager_ShouldSkipFragment(t *testing.T) {
	m := NewManager(nil, 1000)

	// UI state fragments should be skipped
	if !m.ShouldSkipFragment("modal-123") {
		t.Error("modal fragment should be skipped")
	}

	// Route fragments should not be skipped
	if m.ShouldSkipFragment("/dashboard") {
		t.Error("route fragment should not be skipped")
	}
}

func TestManager_NormalizeURL(t *testing.T) {
	m := NewManager(nil, 1000)

	normalized := m.NormalizeURL("HTTPS://EXAMPLE.COM/path")
	if normalized != "https://example.com/path" {
		t.Errorf("NormalizeURL() = %s, want https://example.com/path", normalized)
	}
}

func TestManager_SetTarget(t *testing.T) {
	m := NewManager(nil, 1000)
	m.SetTarget("https://example.com")
	// Just verify no panic
}

func TestManager_Start(t *testing.T) {
	m := NewManager(nil, 1000)
	m.Start("https://example.com")
	// Just verify no panic and stats are reset
	stats := m.GetStats()
	if stats.PagesCrawled != 0 {
		t.Error("Stats should be reset after Start")
	}
}

func TestManager_SaveWithStore(t *testing.T) {
	store := NewMemoryStore()
	m := NewManager(store, 1000)

	state := &CrawlerState{
		Target: "https://example.com",
	}

	err := m.Save(state)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Target != state.Target {
		t.Errorf("Target = %s, want %s", loaded.Target, state.Target)
	}
}

func TestManager_LoadWithStore(t *testing.T) {
	store := NewMemoryStore()
	m := NewManager(store, 1000)

	// Initially empty
	state, err := m.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if state != nil {
		t.Error("Load() should return nil initially")
	}
}

// =============================================================================
// Deduplicator Concurrency Tests
// =============================================================================

func TestDeduplicator_Concurrent(t *testing.T) {
	d := NewDeduplicator(10000)

	// Concurrent adds
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				url := "https://example.com/" + string(rune('a'+id)) + "/" + string(rune('0'+j%10))
				d.Add(url)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Count should be less than or equal to 1000 (some duplicates due to limited URL space)
	if d.Count() > 1000 {
		t.Errorf("Count = %d, expected <= 1000", d.Count())
	}
}

func TestDeduplicator_Merge(t *testing.T) {
	d1 := NewDeduplicator(1000)
	d2 := NewDeduplicator(1000)

	d1.Add("https://example.com/1")
	d1.Add("https://example.com/2")

	d2.Add("https://example.com/3")
	d2.Add("https://example.com/4")

	d1.Merge(d2)

	if d1.Count() != 4 {
		t.Errorf("Count after merge = %d, want 4", d1.Count())
	}

	// All URLs should be present
	for _, url := range []string{"1", "2", "3", "4"} {
		if !d1.HasSeen("https://example.com/" + url) {
			t.Errorf("URL %s should be in merged deduplicator", url)
		}
	}
}

func TestDeduplicator_FalsePositiveRate(t *testing.T) {
	d := NewDeduplicator(1000)

	rate := d.FalsePositiveRate()
	if rate != 0.001 {
		t.Errorf("FalsePositiveRate = %v, want 0.001", rate)
	}
}
