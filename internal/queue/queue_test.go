package queue

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// QueueItem Tests
// =============================================================================

func TestQueueItem_Creation(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		method   string
		depth    int
		priority int
	}{
		{"basic item", "https://example.com", "GET", 0, 0},
		{"with depth", "https://example.com/page", "GET", 5, 0},
		{"with priority", "https://example.com/important", "POST", 1, 10},
		{"empty url", "", "GET", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &QueueItem{
				URL:       tt.url,
				Method:    tt.method,
				Depth:     tt.depth,
				Priority:  tt.priority,
				Timestamp: time.Now(),
			}

			if item.URL != tt.url {
				t.Errorf("URL = %v, want %v", item.URL, tt.url)
			}
			if item.Method != tt.method {
				t.Errorf("Method = %v, want %v", item.Method, tt.method)
			}
			if item.Depth != tt.depth {
				t.Errorf("Depth = %v, want %v", item.Depth, tt.depth)
			}
			if item.Priority != tt.priority {
				t.Errorf("Priority = %v, want %v", item.Priority, tt.priority)
			}
		})
	}
}

// =============================================================================
// MemoryQueue Tests
// =============================================================================

func TestMemoryQueue_NewMemoryQueue(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
	}{
		{"small capacity", 10},
		{"medium capacity", 1000},
		{"large capacity", 100000},
		{"zero capacity", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := NewMemoryQueue(tt.capacity)
			if q == nil {
				t.Fatal("NewMemoryQueue returned nil")
			}
			if q.Len() != 0 {
				t.Errorf("New queue length = %v, want 0", q.Len())
			}
			if !q.IsEmpty() {
				t.Error("New queue should be empty")
			}
		})
	}
}

func TestMemoryQueue_Push(t *testing.T) {
	q := NewMemoryQueue(100)

	tests := []struct {
		name    string
		item    *QueueItem
		wantErr bool
	}{
		{
			name:    "push valid item",
			item:    &QueueItem{URL: "https://example.com/1", Method: "GET", Timestamp: time.Now()},
			wantErr: false,
		},
		{
			name:    "push second item",
			item:    &QueueItem{URL: "https://example.com/2", Method: "GET", Timestamp: time.Now()},
			wantErr: false,
		},
		{
			name:    "push duplicate (should be ignored)",
			item:    &QueueItem{URL: "https://example.com/1", Method: "GET", Timestamp: time.Now()},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := q.Push(tt.item)
			if (err != nil) != tt.wantErr {
				t.Errorf("Push() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	// Should have 2 items (duplicate ignored)
	if q.Len() != 2 {
		t.Errorf("Queue length = %v, want 2", q.Len())
	}
}

func TestMemoryQueue_Pop(t *testing.T) {
	q := NewMemoryQueue(100)

	// Pop from empty queue
	_, err := q.Pop()
	if err != ErrQueueEmpty {
		t.Errorf("Pop from empty queue error = %v, want ErrQueueEmpty", err)
	}

	// Add items with different depths
	items := []*QueueItem{
		{URL: "https://example.com/deep", Depth: 5, Timestamp: time.Now()},
		{URL: "https://example.com/shallow", Depth: 1, Timestamp: time.Now()},
		{URL: "https://example.com/medium", Depth: 3, Timestamp: time.Now()},
	}

	for _, item := range items {
		q.Push(item)
	}

	// Should pop in breadth-first order (lowest depth first)
	item, err := q.Pop()
	if err != nil {
		t.Fatalf("Pop() error = %v", err)
	}
	if item.Depth != 1 {
		t.Errorf("First pop depth = %v, want 1 (breadth-first)", item.Depth)
	}

	item, err = q.Pop()
	if err != nil {
		t.Fatalf("Pop() error = %v", err)
	}
	if item.Depth != 3 {
		t.Errorf("Second pop depth = %v, want 3", item.Depth)
	}
}

func TestMemoryQueue_Capacity(t *testing.T) {
	capacity := 5
	q := NewMemoryQueue(capacity)

	// Fill to capacity
	for i := 0; i < capacity; i++ {
		err := q.Push(&QueueItem{
			URL:       "https://example.com/" + string(rune('a'+i)),
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Fatalf("Push %d failed: %v", i, err)
		}
	}

	// Next push should fail or be ignored
	err := q.Push(&QueueItem{URL: "https://example.com/overflow", Timestamp: time.Now()})
	if err == nil {
		// Check if it was silently ignored by checking length
		if q.Len() > capacity {
			t.Error("Queue exceeded capacity")
		}
	}
}

func TestMemoryQueue_Contains(t *testing.T) {
	q := NewMemoryQueue(100)

	url := "https://example.com/test"
	if q.Contains(url) {
		t.Error("Empty queue should not contain URL")
	}

	q.Push(&QueueItem{URL: url, Timestamp: time.Now()})

	if !q.Contains(url) {
		t.Error("Queue should contain pushed URL")
	}

	q.Pop()

	if q.Contains(url) {
		t.Error("Queue should not contain popped URL")
	}
}

func TestMemoryQueue_Close(t *testing.T) {
	q := NewMemoryQueue(100)
	q.Push(&QueueItem{URL: "https://example.com", Timestamp: time.Now()})

	err := q.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Operations after close should fail
	err = q.Push(&QueueItem{URL: "https://example.com/new", Timestamp: time.Now()})
	if err != ErrQueueClosed {
		t.Errorf("Push after close error = %v, want ErrQueueClosed", err)
	}

	_, err = q.Pop()
	if err != ErrQueueClosed {
		t.Errorf("Pop after close error = %v, want ErrQueueClosed", err)
	}
}

func TestMemoryQueue_Clear(t *testing.T) {
	q := NewMemoryQueue(100)

	for i := 0; i < 10; i++ {
		q.Push(&QueueItem{URL: "https://example.com/" + string(rune('0'+i)), Timestamp: time.Now()})
	}

	if q.Len() != 10 {
		t.Fatalf("Queue length = %v, want 10", q.Len())
	}

	q.Clear()

	if q.Len() != 0 {
		t.Errorf("Queue length after clear = %v, want 0", q.Len())
	}
	if !q.IsEmpty() {
		t.Error("Queue should be empty after clear")
	}
}

func TestMemoryQueue_URLs(t *testing.T) {
	q := NewMemoryQueue(100)

	urls := []string{
		"https://example.com/1",
		"https://example.com/2",
		"https://example.com/3",
	}

	for _, url := range urls {
		q.Push(&QueueItem{URL: url, Timestamp: time.Now()})
	}

	gotURLs := q.URLs()
	if len(gotURLs) != len(urls) {
		t.Errorf("URLs() returned %d items, want %d", len(gotURLs), len(urls))
	}

	// Check all URLs are present
	urlMap := make(map[string]bool)
	for _, u := range gotURLs {
		urlMap[u] = true
	}
	for _, u := range urls {
		if !urlMap[u] {
			t.Errorf("URL %s not found in URLs()", u)
		}
	}
}

func TestMemoryQueue_Concurrent(t *testing.T) {
	q := NewMemoryQueue(10000)
	var wg sync.WaitGroup
	numGoroutines := 10
	itemsPerGoroutine := 100

	// Concurrent pushes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < itemsPerGoroutine; j++ {
				q.Push(&QueueItem{
					URL:       "https://example.com/" + string(rune('a'+id)) + "/" + string(rune('0'+j%10)),
					Timestamp: time.Now(),
				})
			}
		}(i)
	}
	wg.Wait()

	// Concurrent pops
	var popCount int64
	var mu sync.Mutex
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				_, err := q.Pop()
				if err == ErrQueueEmpty {
					return
				}
				if err != nil {
					t.Errorf("Pop error: %v", err)
					return
				}
				mu.Lock()
				popCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if q.Len() != 0 {
		t.Errorf("Queue not empty after concurrent pops, len = %d", q.Len())
	}
}

// =============================================================================
// FastQueue Tests
// =============================================================================

func TestFastQueue_NewFastQueue(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
	}{
		{"small capacity", 100},
		{"medium capacity", 10000},
		{"large capacity", 100000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := NewFastQueue(tt.capacity)
			if q == nil {
				t.Fatal("NewFastQueue returned nil")
			}
			if q.Len() != 0 {
				t.Errorf("New queue length = %v, want 0", q.Len())
			}
			if !q.IsEmpty() {
				t.Error("New queue should be empty")
			}
		})
	}
}

func TestFastQueue_Push(t *testing.T) {
	q := NewFastQueue(100)

	err := q.Push(&QueueItem{URL: "https://example.com/1", Timestamp: time.Now()})
	if err != nil {
		t.Errorf("Push() error = %v", err)
	}

	if q.Len() != 1 {
		t.Errorf("Queue length = %v, want 1", q.Len())
	}

	// Push duplicate
	err = q.Push(&QueueItem{URL: "https://example.com/1", Timestamp: time.Now()})
	if err != nil {
		t.Errorf("Push duplicate error = %v", err)
	}

	// Length should still be 1 (duplicate ignored)
	if q.Len() != 1 {
		t.Errorf("Queue length after duplicate = %v, want 1", q.Len())
	}
}

func TestFastQueue_PushBatch(t *testing.T) {
	q := NewFastQueue(1000)

	items := make([]*QueueItem, 100)
	for i := 0; i < 100; i++ {
		items[i] = &QueueItem{
			URL:       "https://example.com/page/" + fmt.Sprintf("%d", i),
			Timestamp: time.Now(),
		}
	}

	added, err := q.PushBatch(items)
	if err != nil {
		t.Errorf("PushBatch() error = %v", err)
	}

	if added != 100 {
		t.Errorf("PushBatch() added = %v, want 100", added)
	}

	if q.Len() != 100 {
		t.Errorf("Queue length = %v, want 100", q.Len())
	}

	// Push batch with duplicates
	added, err = q.PushBatch(items[:10])
	if err != nil {
		t.Errorf("PushBatch duplicates error = %v", err)
	}

	if added != 0 {
		t.Errorf("PushBatch duplicates added = %v, want 0", added)
	}
}

func TestFastQueue_Pop(t *testing.T) {
	q := NewFastQueue(100)

	// Pop from empty
	_, err := q.Pop()
	if err != ErrQueueEmpty {
		t.Errorf("Pop from empty error = %v, want ErrQueueEmpty", err)
	}

	q.Push(&QueueItem{URL: "https://example.com/test", Timestamp: time.Now()})

	item, err := q.Pop()
	if err != nil {
		t.Errorf("Pop() error = %v", err)
	}
	if item.URL != "https://example.com/test" {
		t.Errorf("Pop() URL = %v, want https://example.com/test", item.URL)
	}

	if !q.IsEmpty() {
		t.Error("Queue should be empty after pop")
	}
}

func TestFastQueue_PopBatch(t *testing.T) {
	q := NewFastQueue(1000)

	// PopBatch from empty
	_, err := q.PopBatch(10)
	if err != ErrQueueEmpty {
		t.Errorf("PopBatch from empty error = %v, want ErrQueueEmpty", err)
	}

	// Add 50 items
	for i := 0; i < 50; i++ {
		q.Push(&QueueItem{
			URL:       fmt.Sprintf("https://example.com/batch/%d", i),
			Timestamp: time.Now(),
		})
	}

	// Pop batch of 10
	items, err := q.PopBatch(10)
	if err != nil {
		t.Errorf("PopBatch() error = %v", err)
	}

	if len(items) != 10 {
		t.Errorf("PopBatch() returned %d items, want 10", len(items))
	}

	if q.Len() != 40 {
		t.Errorf("Queue length after PopBatch = %v, want 40", q.Len())
	}

	// Pop more than available
	items, err = q.PopBatch(100)
	if err != nil {
		t.Errorf("PopBatch() error = %v", err)
	}

	if len(items) != 40 {
		t.Errorf("PopBatch() returned %d items, want 40", len(items))
	}
}

func TestFastQueue_Contains(t *testing.T) {
	q := NewFastQueue(100)

	url := "https://example.com/test"

	if q.Contains(url) {
		t.Error("Empty queue should not contain URL")
	}

	q.Push(&QueueItem{URL: url, Timestamp: time.Now()})

	if !q.Contains(url) {
		t.Error("Queue should contain pushed URL")
	}

	q.Pop()

	if q.Contains(url) {
		t.Error("Queue should not contain popped URL")
	}
}

func TestFastQueue_Close(t *testing.T) {
	q := NewFastQueue(100)
	q.Push(&QueueItem{URL: "https://example.com", Timestamp: time.Now()})

	err := q.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	err = q.Push(&QueueItem{URL: "https://example.com/new", Timestamp: time.Now()})
	if err != ErrQueueClosed {
		t.Errorf("Push after close error = %v, want ErrQueueClosed", err)
	}
}

func TestFastQueue_Stats(t *testing.T) {
	q := NewFastQueue(1000)

	for i := 0; i < 100; i++ {
		q.Push(&QueueItem{
			URL:       fmt.Sprintf("https://example.com/stats/%d", i),
			Timestamp: time.Now(),
		})
	}

	stats := q.Stats()

	if stats.TotalItems != 100 {
		t.Errorf("Stats.TotalItems = %v, want 100", stats.TotalItems)
	}

	// Check shards have items distributed
	totalInShards := 0
	for _, size := range stats.ShardSizes {
		totalInShards += size
	}

	if totalInShards != 100 {
		t.Errorf("Total in shards = %v, want 100", totalInShards)
	}
}

func TestFastQueue_Concurrent(t *testing.T) {
	q := NewFastQueue(100000)
	var wg sync.WaitGroup
	numGoroutines := 50
	itemsPerGoroutine := 100

	// Concurrent pushes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < itemsPerGoroutine; j++ {
				q.Push(&QueueItem{
					URL:       "https://example.com/g" + string(rune('a'+id%26)) + "/p" + string(rune('0'+j%10)),
					Timestamp: time.Now(),
				})
			}
		}(i)
	}
	wg.Wait()

	initialLen := q.Len()
	t.Logf("Queue length after concurrent pushes: %d", initialLen)

	// Concurrent pops
	var popCount int64
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				_, err := q.Pop()
				if err == ErrQueueEmpty {
					return
				}
				if err != nil && err != ErrQueueEmpty {
					t.Errorf("Pop error: %v", err)
					return
				}
				mu.Lock()
				popCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if q.Len() != 0 {
		t.Errorf("Queue not empty after concurrent pops, len = %d", q.Len())
	}
}

func TestFastQueue_ConcurrentPushPop(t *testing.T) {
	q := NewFastQueue(10000)
	var wg sync.WaitGroup
	done := make(chan struct{})

	// Concurrent pushers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				select {
				case <-done:
					return
				default:
					q.Push(&QueueItem{
						URL:       "https://example.com/" + string(rune('a'+id)) + "/" + string(rune('0'+j%10)),
						Timestamp: time.Now(),
					})
				}
			}
		}(i)
	}

	// Concurrent poppers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				select {
				case <-done:
					return
				default:
					q.Pop()
					time.Sleep(time.Microsecond)
				}
			}
		}()
	}

	wg.Wait()
	close(done)

	// Queue should be in consistent state
	_ = q.Len() // Should not panic
}

// =============================================================================
// Priority Queue Tests
// =============================================================================

func TestPriorityQueue_BreadthFirst(t *testing.T) {
	q := NewMemoryQueue(100)

	// Add items in random depth order
	items := []struct {
		url   string
		depth int
	}{
		{"https://example.com/d3", 3},
		{"https://example.com/d1", 1},
		{"https://example.com/d5", 5},
		{"https://example.com/d2", 2},
		{"https://example.com/d4", 4},
	}

	for _, item := range items {
		q.Push(&QueueItem{URL: item.url, Depth: item.depth, Timestamp: time.Now()})
	}

	// Pop should return in breadth-first order (depth 1, 2, 3, 4, 5)
	expectedDepths := []int{1, 2, 3, 4, 5}
	for i, expectedDepth := range expectedDepths {
		item, err := q.Pop()
		if err != nil {
			t.Fatalf("Pop %d error: %v", i, err)
		}
		if item.Depth != expectedDepth {
			t.Errorf("Pop %d depth = %v, want %v", i, item.Depth, expectedDepth)
		}
	}
}

func TestPriorityQueue_PriorityWithinDepth(t *testing.T) {
	q := NewMemoryQueue(100)

	// Add items with same depth but different priorities
	items := []struct {
		url      string
		priority int
	}{
		{"https://example.com/low", 1},
		{"https://example.com/high", 10},
		{"https://example.com/medium", 5},
	}

	for _, item := range items {
		q.Push(&QueueItem{URL: item.url, Depth: 1, Priority: item.priority, Timestamp: time.Now()})
	}

	// Higher priority should come first
	item, _ := q.Pop()
	if item.Priority != 10 {
		t.Errorf("First pop priority = %v, want 10", item.Priority)
	}

	item, _ = q.Pop()
	if item.Priority != 5 {
		t.Errorf("Second pop priority = %v, want 5", item.Priority)
	}
}
