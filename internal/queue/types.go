package queue

import "time"

// QueueItem represents an item in the crawl queue.
type QueueItem struct {
	URL       string
	Method    string
	Depth     int
	ParentURL string
	Headers   map[string]string
	Body      []byte
	Priority  int
	Timestamp time.Time
}
