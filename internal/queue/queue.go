// Package queue provides URL queue implementations for the crawler.
package queue

// Queue defines the interface for URL queues.
type Queue interface {
	// Push adds an item to the queue
	Push(item *QueueItem) error

	// Pop removes and returns the next item from the queue
	Pop() (*QueueItem, error)

	// Peek returns the next item without removing it
	Peek() (*QueueItem, error)

	// Len returns the number of items in the queue
	Len() int

	// IsEmpty returns true if the queue is empty
	IsEmpty() bool

	// Clear removes all items from the queue
	Clear() error

	// Close closes the queue and releases resources
	Close() error

	// Contains checks if a URL is already in the queue
	Contains(url string) bool
}
