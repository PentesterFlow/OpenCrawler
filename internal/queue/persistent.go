package queue

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketQueue   = []byte("queue")
	bucketVisited = []byte("visited")
)

// PersistentQueue is a disk-backed queue using BoltDB.
type PersistentQueue struct {
	mu       sync.RWMutex
	db       *bolt.DB
	memory   *MemoryQueue // In-memory buffer for fast access
	closed   bool
	dbPath   string
	maxMem   int // Max items to keep in memory
}

// NewPersistentQueue creates a new persistent queue.
func NewPersistentQueue(dbPath string, maxMemory int) (*PersistentQueue, error) {
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{
		Timeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create buckets
	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketQueue); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketVisited); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create buckets: %w", err)
	}

	pq := &PersistentQueue{
		db:     db,
		memory: NewMemoryQueue(maxMemory),
		dbPath: dbPath,
		maxMem: maxMemory,
	}

	// Load existing items into memory
	if err := pq.loadFromDisk(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to load from disk: %w", err)
	}

	return pq, nil
}

// loadFromDisk loads queue items from disk into memory.
func (pq *PersistentQueue) loadFromDisk() error {
	return pq.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketQueue)
		if b == nil {
			return nil
		}

		count := 0
		return b.ForEach(func(k, v []byte) error {
			if count >= pq.maxMem {
				return nil
			}

			var item QueueItem
			if err := json.Unmarshal(v, &item); err != nil {
				return nil // Skip invalid items
			}

			pq.memory.Push(&item)
			count++
			return nil
		})
	})
}

// Push adds an item to the queue.
func (pq *PersistentQueue) Push(item *QueueItem) error {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.closed {
		return ErrQueueClosed
	}

	// Check if already visited
	if pq.isVisited(item.URL) {
		return nil
	}

	// Persist to disk
	if err := pq.persistItem(item); err != nil {
		return err
	}

	// Try to add to memory queue
	if pq.memory.Len() < pq.maxMem {
		pq.memory.Push(item)
	}

	return nil
}

// persistItem saves an item to disk.
func (pq *PersistentQueue) persistItem(item *QueueItem) error {
	return pq.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketQueue)
		if b == nil {
			return errors.New("queue bucket not found")
		}

		data, err := json.Marshal(item)
		if err != nil {
			return err
		}

		return b.Put([]byte(item.URL), data)
	})
}

// Pop removes and returns the next item from the queue.
func (pq *PersistentQueue) Pop() (*QueueItem, error) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.closed {
		return nil, ErrQueueClosed
	}

	// Try memory first
	item, err := pq.memory.Pop()
	if err == nil {
		// Remove from disk and mark as visited
		pq.removeFromDisk(item.URL)
		pq.markVisited(item.URL)
		return item, nil
	}

	// Try loading more from disk
	if err := pq.refillMemory(); err != nil {
		return nil, err
	}

	item, err = pq.memory.Pop()
	if err != nil {
		return nil, ErrQueueEmpty
	}

	pq.removeFromDisk(item.URL)
	pq.markVisited(item.URL)
	return item, nil
}

// refillMemory loads more items from disk into memory.
func (pq *PersistentQueue) refillMemory() error {
	return pq.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketQueue)
		if b == nil {
			return nil
		}

		count := 0
		return b.ForEach(func(k, v []byte) error {
			if count >= pq.maxMem {
				return nil
			}

			// Skip if already in memory
			if pq.memory.Contains(string(k)) {
				return nil
			}

			var item QueueItem
			if err := json.Unmarshal(v, &item); err != nil {
				return nil
			}

			pq.memory.Push(&item)
			count++
			return nil
		})
	})
}

// removeFromDisk removes an item from disk storage.
func (pq *PersistentQueue) removeFromDisk(url string) error {
	return pq.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketQueue)
		if b == nil {
			return nil
		}
		return b.Delete([]byte(url))
	})
}

// markVisited marks a URL as visited.
func (pq *PersistentQueue) markVisited(url string) error {
	return pq.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketVisited)
		if b == nil {
			return nil
		}
		return b.Put([]byte(url), []byte{1})
	})
}

// isVisited checks if a URL has been visited.
func (pq *PersistentQueue) isVisited(url string) bool {
	var visited bool
	pq.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketVisited)
		if b == nil {
			return nil
		}
		visited = b.Get([]byte(url)) != nil
		return nil
	})
	return visited
}

// Peek returns the next item without removing it.
func (pq *PersistentQueue) Peek() (*QueueItem, error) {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	if pq.closed {
		return nil, ErrQueueClosed
	}

	return pq.memory.Peek()
}

// Len returns the approximate number of items in the queue.
func (pq *PersistentQueue) Len() int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	var count int
	pq.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketQueue)
		if b != nil {
			count = b.Stats().KeyN
		}
		return nil
	})
	return count
}

// IsEmpty returns true if the queue is empty.
func (pq *PersistentQueue) IsEmpty() bool {
	return pq.Len() == 0
}

// Clear removes all items from the queue.
func (pq *PersistentQueue) Clear() error {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	pq.memory.Clear()

	return pq.db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(bucketQueue); err != nil && err != bolt.ErrBucketNotFound {
			return err
		}
		if _, err := tx.CreateBucket(bucketQueue); err != nil {
			return err
		}
		return nil
	})
}

// Close closes the queue and database.
func (pq *PersistentQueue) Close() error {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	pq.closed = true
	pq.memory.Close()
	return pq.db.Close()
}

// Contains checks if a URL is in the queue or has been visited.
func (pq *PersistentQueue) Contains(url string) bool {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	if pq.memory.Contains(url) {
		return true
	}

	if pq.isVisited(url) {
		return true
	}

	var found bool
	pq.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketQueue)
		if b != nil {
			found = b.Get([]byte(url)) != nil
		}
		return nil
	})
	return found
}

// Stats returns queue statistics.
func (pq *PersistentQueue) Stats() (queueSize, visitedCount int) {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	pq.db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket(bucketQueue); b != nil {
			queueSize = b.Stats().KeyN
		}
		if b := tx.Bucket(bucketVisited); b != nil {
			visitedCount = b.Stats().KeyN
		}
		return nil
	})
	return
}
