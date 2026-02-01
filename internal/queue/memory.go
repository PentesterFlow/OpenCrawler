package queue

import (
	"container/heap"
	"errors"
	"sync"
)

var (
	ErrQueueEmpty  = errors.New("queue is empty")
	ErrQueueClosed = errors.New("queue is closed")
)

// priorityQueue implements heap.Interface for QueueItem.
type priorityQueue []*QueueItem

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	// Lower depth = higher priority (breadth-first)
	if pq[i].Depth != pq[j].Depth {
		return pq[i].Depth < pq[j].Depth
	}
	// Higher priority value = higher priority
	return pq[i].Priority > pq[j].Priority
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *priorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*QueueItem))
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*pq = old[0 : n-1]
	return item
}

// MemoryQueue is a thread-safe in-memory priority queue.
type MemoryQueue struct {
	mu       sync.RWMutex
	pq       priorityQueue
	urlSet   map[string]struct{}
	closed   bool
	cond     *sync.Cond
	capacity int
}

// NewMemoryQueue creates a new in-memory queue.
func NewMemoryQueue(capacity int) *MemoryQueue {
	mq := &MemoryQueue{
		pq:       make(priorityQueue, 0),
		urlSet:   make(map[string]struct{}),
		capacity: capacity,
	}
	mq.cond = sync.NewCond(&mq.mu)
	heap.Init(&mq.pq)
	return mq
}

// Push adds an item to the queue.
func (mq *MemoryQueue) Push(item *QueueItem) error {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	if mq.closed {
		return ErrQueueClosed
	}

	// Check capacity
	if mq.capacity > 0 && len(mq.pq) >= mq.capacity {
		return errors.New("queue at capacity")
	}

	// Check for duplicates
	if _, exists := mq.urlSet[item.URL]; exists {
		return nil // Silently ignore duplicates
	}

	mq.urlSet[item.URL] = struct{}{}
	heap.Push(&mq.pq, item)
	mq.cond.Signal()
	return nil
}

// Pop removes and returns the next item from the queue.
func (mq *MemoryQueue) Pop() (*QueueItem, error) {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	if mq.closed {
		return nil, ErrQueueClosed
	}

	if len(mq.pq) == 0 {
		return nil, ErrQueueEmpty
	}

	item := heap.Pop(&mq.pq).(*QueueItem)
	delete(mq.urlSet, item.URL)
	return item, nil
}

// PopWait removes and returns the next item, blocking if empty.
func (mq *MemoryQueue) PopWait() (*QueueItem, error) {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	for len(mq.pq) == 0 && !mq.closed {
		mq.cond.Wait()
	}

	if mq.closed && len(mq.pq) == 0 {
		return nil, ErrQueueClosed
	}

	item := heap.Pop(&mq.pq).(*QueueItem)
	delete(mq.urlSet, item.URL)
	return item, nil
}

// Peek returns the next item without removing it.
func (mq *MemoryQueue) Peek() (*QueueItem, error) {
	mq.mu.RLock()
	defer mq.mu.RUnlock()

	if mq.closed {
		return nil, ErrQueueClosed
	}

	if len(mq.pq) == 0 {
		return nil, ErrQueueEmpty
	}

	return mq.pq[0], nil
}

// Len returns the number of items in the queue.
func (mq *MemoryQueue) Len() int {
	mq.mu.RLock()
	defer mq.mu.RUnlock()
	return len(mq.pq)
}

// IsEmpty returns true if the queue is empty.
func (mq *MemoryQueue) IsEmpty() bool {
	return mq.Len() == 0
}

// Clear removes all items from the queue.
func (mq *MemoryQueue) Clear() error {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	mq.pq = make(priorityQueue, 0)
	mq.urlSet = make(map[string]struct{})
	heap.Init(&mq.pq)
	return nil
}

// Close closes the queue.
func (mq *MemoryQueue) Close() error {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	mq.closed = true
	mq.cond.Broadcast()
	return nil
}

// Contains checks if a URL is in the queue.
func (mq *MemoryQueue) Contains(url string) bool {
	mq.mu.RLock()
	defer mq.mu.RUnlock()
	_, exists := mq.urlSet[url]
	return exists
}

// URLs returns all URLs currently in the queue.
func (mq *MemoryQueue) URLs() []string {
	mq.mu.RLock()
	defer mq.mu.RUnlock()

	urls := make([]string, 0, len(mq.urlSet))
	for url := range mq.urlSet {
		urls = append(urls, url)
	}
	return urls
}
