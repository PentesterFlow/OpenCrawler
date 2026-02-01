package queue

import (
	"container/heap"
	"sync"
	"sync/atomic"
)

// FastQueue is a high-performance queue optimized for concurrent access.
// It uses sharding to reduce lock contention and supports batch operations.
type FastQueue struct {
	shards    []*queueShard
	numShards int
	counter   uint64 // For round-robin distribution
	closed    atomic.Bool
	totalLen  atomic.Int64
	capacity  int

	// Deduplication using sharded bloom filter + map for accuracy
	dedupShards []*dedupShard
}

type queueShard struct {
	mu   sync.Mutex
	pq   priorityQueue
	cond *sync.Cond
}

type dedupShard struct {
	mu     sync.RWMutex
	urlSet map[string]struct{}
}

// NewFastQueue creates a new high-performance queue.
func NewFastQueue(capacity int) *FastQueue {
	numShards := 16 // Power of 2 for fast modulo
	if capacity < 10000 {
		numShards = 4
	} else if capacity > 100000 {
		numShards = 32
	}

	fq := &FastQueue{
		shards:      make([]*queueShard, numShards),
		dedupShards: make([]*dedupShard, numShards),
		numShards:   numShards,
		capacity:    capacity,
	}

	shardCap := capacity / numShards
	if shardCap < 1000 {
		shardCap = 1000
	}

	for i := 0; i < numShards; i++ {
		shard := &queueShard{
			pq: make(priorityQueue, 0, shardCap),
		}
		shard.cond = sync.NewCond(&shard.mu)
		heap.Init(&shard.pq)
		fq.shards[i] = shard

		fq.dedupShards[i] = &dedupShard{
			urlSet: make(map[string]struct{}, shardCap),
		}
	}

	return fq
}

// getShardIndex returns shard index for a URL using FNV-1a hash.
func (fq *FastQueue) getShardIndex(url string) int {
	// FNV-1a hash - fast and good distribution
	h := uint64(14695981039346656037)
	for i := 0; i < len(url); i++ {
		h ^= uint64(url[i])
		h *= 1099511628211
	}
	return int(h & uint64(fq.numShards-1))
}

// Push adds an item to the queue.
func (fq *FastQueue) Push(item *QueueItem) error {
	if fq.closed.Load() {
		return ErrQueueClosed
	}

	// Check capacity
	if fq.capacity > 0 && fq.totalLen.Load() >= int64(fq.capacity) {
		return nil // Silently drop when at capacity
	}

	// Check dedup first (separate lock from queue)
	shardIdx := fq.getShardIndex(item.URL)
	dedupShard := fq.dedupShards[shardIdx]

	dedupShard.mu.Lock()
	if _, exists := dedupShard.urlSet[item.URL]; exists {
		dedupShard.mu.Unlock()
		return nil // Already in queue
	}
	dedupShard.urlSet[item.URL] = struct{}{}
	dedupShard.mu.Unlock()

	// Add to queue shard (use round-robin for better distribution)
	queueIdx := int(atomic.AddUint64(&fq.counter, 1) & uint64(fq.numShards-1))
	shard := fq.shards[queueIdx]

	shard.mu.Lock()
	heap.Push(&shard.pq, item)
	shard.cond.Signal()
	shard.mu.Unlock()

	fq.totalLen.Add(1)
	return nil
}

// PushBatch adds multiple items to the queue efficiently.
func (fq *FastQueue) PushBatch(items []*QueueItem) (int, error) {
	if fq.closed.Load() {
		return 0, ErrQueueClosed
	}

	added := 0
	// Group items by shard
	shardItems := make([][]*QueueItem, fq.numShards)
	for i := range shardItems {
		shardItems[i] = make([]*QueueItem, 0, len(items)/fq.numShards+1)
	}

	// First pass: dedup check and grouping
	for _, item := range items {
		if fq.capacity > 0 && fq.totalLen.Load()+int64(added) >= int64(fq.capacity) {
			break
		}

		shardIdx := fq.getShardIndex(item.URL)
		dedupShard := fq.dedupShards[shardIdx]

		dedupShard.mu.Lock()
		if _, exists := dedupShard.urlSet[item.URL]; exists {
			dedupShard.mu.Unlock()
			continue
		}
		dedupShard.urlSet[item.URL] = struct{}{}
		dedupShard.mu.Unlock()

		// Round-robin queue distribution
		queueIdx := int(atomic.AddUint64(&fq.counter, 1) & uint64(fq.numShards-1))
		shardItems[queueIdx] = append(shardItems[queueIdx], item)
		added++
	}

	// Second pass: batch add to each shard
	for i, items := range shardItems {
		if len(items) == 0 {
			continue
		}
		shard := fq.shards[i]
		shard.mu.Lock()
		for _, item := range items {
			heap.Push(&shard.pq, item)
		}
		shard.cond.Broadcast() // Wake all waiters
		shard.mu.Unlock()
	}

	fq.totalLen.Add(int64(added))
	return added, nil
}

// Pop removes and returns the next item (non-blocking).
func (fq *FastQueue) Pop() (*QueueItem, error) {
	if fq.closed.Load() {
		return nil, ErrQueueClosed
	}

	// Try each shard starting from a random one
	startIdx := int(atomic.AddUint64(&fq.counter, 1) & uint64(fq.numShards-1))

	for i := 0; i < fq.numShards; i++ {
		idx := (startIdx + i) & (fq.numShards - 1)
		shard := fq.shards[idx]

		shard.mu.Lock()
		if len(shard.pq) > 0 {
			item := heap.Pop(&shard.pq).(*QueueItem)
			shard.mu.Unlock()

			// Remove from dedup
			dedupIdx := fq.getShardIndex(item.URL)
			dedupShard := fq.dedupShards[dedupIdx]
			dedupShard.mu.Lock()
			delete(dedupShard.urlSet, item.URL)
			dedupShard.mu.Unlock()

			fq.totalLen.Add(-1)
			return item, nil
		}
		shard.mu.Unlock()
	}

	return nil, ErrQueueEmpty
}

// PopBatch removes and returns up to n items.
func (fq *FastQueue) PopBatch(n int) ([]*QueueItem, error) {
	if fq.closed.Load() {
		return nil, ErrQueueClosed
	}

	items := make([]*QueueItem, 0, n)
	startIdx := int(atomic.AddUint64(&fq.counter, 1) & uint64(fq.numShards-1))

	for i := 0; i < fq.numShards && len(items) < n; i++ {
		idx := (startIdx + i) & (fq.numShards - 1)
		shard := fq.shards[idx]

		shard.mu.Lock()
		for len(shard.pq) > 0 && len(items) < n {
			item := heap.Pop(&shard.pq).(*QueueItem)
			items = append(items, item)
		}
		shard.mu.Unlock()
	}

	// Batch remove from dedup
	for _, item := range items {
		dedupIdx := fq.getShardIndex(item.URL)
		dedupShard := fq.dedupShards[dedupIdx]
		dedupShard.mu.Lock()
		delete(dedupShard.urlSet, item.URL)
		dedupShard.mu.Unlock()
	}

	fq.totalLen.Add(-int64(len(items)))

	if len(items) == 0 {
		return nil, ErrQueueEmpty
	}
	return items, nil
}

// PopWait removes and returns the next item, blocking if empty.
func (fq *FastQueue) PopWait() (*QueueItem, error) {
	// First try non-blocking
	if item, err := fq.Pop(); err == nil {
		return item, nil
	}

	// If empty, wait on shard 0
	shard := fq.shards[0]
	shard.mu.Lock()
	for fq.totalLen.Load() == 0 && !fq.closed.Load() {
		shard.cond.Wait()
	}
	shard.mu.Unlock()

	if fq.closed.Load() && fq.totalLen.Load() == 0 {
		return nil, ErrQueueClosed
	}

	return fq.Pop()
}

// Len returns the total number of items across all shards.
func (fq *FastQueue) Len() int {
	return int(fq.totalLen.Load())
}

// IsEmpty returns true if the queue is empty.
func (fq *FastQueue) IsEmpty() bool {
	return fq.totalLen.Load() == 0
}

// Contains checks if a URL is in the queue.
func (fq *FastQueue) Contains(url string) bool {
	shardIdx := fq.getShardIndex(url)
	dedupShard := fq.dedupShards[shardIdx]
	dedupShard.mu.RLock()
	_, exists := dedupShard.urlSet[url]
	dedupShard.mu.RUnlock()
	return exists
}

// Close closes the queue and wakes all waiters.
func (fq *FastQueue) Close() error {
	fq.closed.Store(true)
	for _, shard := range fq.shards {
		shard.mu.Lock()
		shard.cond.Broadcast()
		shard.mu.Unlock()
	}
	return nil
}

// Clear removes all items from the queue.
func (fq *FastQueue) Clear() error {
	for i := 0; i < fq.numShards; i++ {
		shard := fq.shards[i]
		shard.mu.Lock()
		shard.pq = make(priorityQueue, 0, fq.capacity/fq.numShards)
		heap.Init(&shard.pq)
		shard.mu.Unlock()

		dedupShard := fq.dedupShards[i]
		dedupShard.mu.Lock()
		dedupShard.urlSet = make(map[string]struct{})
		dedupShard.mu.Unlock()
	}
	fq.totalLen.Store(0)
	return nil
}

// URLs returns all URLs currently in the queue.
func (fq *FastQueue) URLs() []string {
	urls := make([]string, 0, fq.Len())
	for i := 0; i < fq.numShards; i++ {
		dedupShard := fq.dedupShards[i]
		dedupShard.mu.RLock()
		for url := range dedupShard.urlSet {
			urls = append(urls, url)
		}
		dedupShard.mu.RUnlock()
	}
	return urls
}

// Stats returns queue statistics.
type QueueStats struct {
	TotalItems   int
	ShardSizes   []int
	DedupEntries int
}

func (fq *FastQueue) Stats() QueueStats {
	stats := QueueStats{
		TotalItems: int(fq.totalLen.Load()),
		ShardSizes: make([]int, fq.numShards),
	}

	for i := 0; i < fq.numShards; i++ {
		shard := fq.shards[i]
		shard.mu.Lock()
		stats.ShardSizes[i] = len(shard.pq)
		shard.mu.Unlock()

		dedupShard := fq.dedupShards[i]
		dedupShard.mu.RLock()
		stats.DedupEntries += len(dedupShard.urlSet)
		dedupShard.mu.RUnlock()
	}

	return stats
}
