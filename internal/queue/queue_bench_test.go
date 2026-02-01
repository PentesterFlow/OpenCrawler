package queue

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func BenchmarkMemoryQueuePush(b *testing.B) {
	q := NewMemoryQueue(b.N + 1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Push(&QueueItem{
			URL:       fmt.Sprintf("https://example.com/page/%d", i),
			Timestamp: time.Now(),
		})
	}
}

func BenchmarkFastQueuePush(b *testing.B) {
	q := NewFastQueue(b.N + 1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Push(&QueueItem{
			URL:       fmt.Sprintf("https://example.com/page/%d", i),
			Timestamp: time.Now(),
		})
	}
}

func BenchmarkMemoryQueuePop(b *testing.B) {
	q := NewMemoryQueue(b.N + 1000)
	for i := 0; i < b.N; i++ {
		q.Push(&QueueItem{
			URL:       fmt.Sprintf("https://example.com/page/%d", i),
			Timestamp: time.Now(),
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Pop()
	}
}

func BenchmarkFastQueuePop(b *testing.B) {
	q := NewFastQueue(b.N + 1000)
	for i := 0; i < b.N; i++ {
		q.Push(&QueueItem{
			URL:       fmt.Sprintf("https://example.com/page/%d", i),
			Timestamp: time.Now(),
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Pop()
	}
}

func BenchmarkFastQueuePopBatch(b *testing.B) {
	q := NewFastQueue(b.N + 1000)
	for i := 0; i < b.N; i++ {
		q.Push(&QueueItem{
			URL:       fmt.Sprintf("https://example.com/page/%d", i),
			Timestamp: time.Now(),
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i += 10 {
		q.PopBatch(10)
	}
}

func BenchmarkMemoryQueueConcurrent(b *testing.B) {
	q := NewMemoryQueue(b.N*10 + 1000)
	numWriters := 10
	numReaders := 10

	b.ResetTimer()
	var wg sync.WaitGroup

	// Writers
	for w := 0; w < numWriters; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < b.N/numWriters; i++ {
				q.Push(&QueueItem{
					URL:       fmt.Sprintf("https://example.com/writer/%d/page/%d", id, i),
					Timestamp: time.Now(),
				})
			}
		}(w)
	}

	// Readers
	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < b.N/numReaders; i++ {
				q.Pop()
			}
		}()
	}

	wg.Wait()
}

func BenchmarkFastQueueConcurrent(b *testing.B) {
	q := NewFastQueue(b.N*10 + 1000)
	numWriters := 10
	numReaders := 10

	b.ResetTimer()
	var wg sync.WaitGroup

	// Writers
	for w := 0; w < numWriters; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < b.N/numWriters; i++ {
				q.Push(&QueueItem{
					URL:       fmt.Sprintf("https://example.com/writer/%d/page/%d", id, i),
					Timestamp: time.Now(),
				})
			}
		}(w)
	}

	// Readers
	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < b.N/numReaders; i++ {
				q.Pop()
			}
		}()
	}

	wg.Wait()
}

func BenchmarkFastQueueBatchPush(b *testing.B) {
	q := NewFastQueue(b.N + 1000)
	batchSize := 100
	batches := b.N / batchSize

	b.ResetTimer()
	for i := 0; i < batches; i++ {
		items := make([]*QueueItem, batchSize)
		for j := 0; j < batchSize; j++ {
			items[j] = &QueueItem{
				URL:       fmt.Sprintf("https://example.com/batch/%d/page/%d", i, j),
				Timestamp: time.Now(),
			}
		}
		q.PushBatch(items)
	}
}

func BenchmarkFastQueueHighContention(b *testing.B) {
	q := NewFastQueue(b.N*100 + 1000)
	numGoroutines := 100

	b.ResetTimer()
	var wg sync.WaitGroup

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < b.N/numGoroutines; i++ {
				q.Push(&QueueItem{
					URL:       fmt.Sprintf("https://example.com/g/%d/page/%d", id, i),
					Timestamp: time.Now(),
				})
				q.Pop()
			}
		}(g)
	}

	wg.Wait()
}
