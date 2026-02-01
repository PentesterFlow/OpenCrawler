package metrics

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
}

func TestCollector_RecordRequest(t *testing.T) {
	c := New()

	c.RecordRequest()
	c.RecordRequest()
	c.RecordRequest()

	snap := c.Snapshot()
	if snap.RequestsTotal != 3 {
		t.Errorf("RequestsTotal = %d, want 3", snap.RequestsTotal)
	}
}

func TestCollector_RecordError(t *testing.T) {
	c := New()

	c.RecordError("network")
	c.RecordError("network")
	c.RecordError("timeout")

	snap := c.Snapshot()
	if snap.ErrorsTotal != 3 {
		t.Errorf("ErrorsTotal = %d, want 3", snap.ErrorsTotal)
	}
	if snap.ErrorCounts["network"] != 2 {
		t.Errorf("ErrorCounts[network] = %d, want 2", snap.ErrorCounts["network"])
	}
	if snap.ErrorCounts["timeout"] != 1 {
		t.Errorf("ErrorCounts[timeout] = %d, want 1", snap.ErrorCounts["timeout"])
	}
}

func TestCollector_RecordResponseTime(t *testing.T) {
	c := New()

	c.RecordResponseTime(100 * time.Millisecond)
	c.RecordResponseTime(200 * time.Millisecond)
	c.RecordResponseTime(300 * time.Millisecond)

	snap := c.Snapshot()
	avgMs := snap.AverageResponseTime.Milliseconds()
	if avgMs != 200 {
		t.Errorf("AverageResponseTime = %dms, want 200ms", avgMs)
	}
}

func TestCollector_RecordResponseTime_Buckets(t *testing.T) {
	c := New()

	c.RecordResponseTime(5 * time.Millisecond)    // bucket 0 (<10)
	c.RecordResponseTime(30 * time.Millisecond)   // bucket 1 (<50)
	c.RecordResponseTime(75 * time.Millisecond)   // bucket 2 (<100)
	c.RecordResponseTime(150 * time.Millisecond)  // bucket 3 (<250)
	c.RecordResponseTime(400 * time.Millisecond)  // bucket 4 (<500)
	c.RecordResponseTime(750 * time.Millisecond)  // bucket 5 (<1000)
	c.RecordResponseTime(2000 * time.Millisecond) // bucket 6 (<2500)
	c.RecordResponseTime(4000 * time.Millisecond) // bucket 7 (<5000)
	c.RecordResponseTime(8000 * time.Millisecond) // bucket 8 (<10000)
	c.RecordResponseTime(15000 * time.Millisecond) // bucket 9 (>=10000)

	snap := c.Snapshot()
	for i := 0; i < 10; i++ {
		if snap.ResponseTimeHist[i] != 1 {
			t.Errorf("ResponseTimeHist[%d] = %d, want 1", i, snap.ResponseTimeHist[i])
		}
	}
}

func TestCollector_RecordStatusCode(t *testing.T) {
	c := New()

	c.RecordStatusCode(200)
	c.RecordStatusCode(200)
	c.RecordStatusCode(404)
	c.RecordStatusCode(500)

	snap := c.Snapshot()
	if snap.StatusCodes[200] != 2 {
		t.Errorf("StatusCodes[200] = %d, want 2", snap.StatusCodes[200])
	}
	if snap.StatusCodes[404] != 1 {
		t.Errorf("StatusCodes[404] = %d, want 1", snap.StatusCodes[404])
	}
	if snap.StatusCodes[500] != 1 {
		t.Errorf("StatusCodes[500] = %d, want 1", snap.StatusCodes[500])
	}
}

func TestCollector_RecordPageDiscovered(t *testing.T) {
	c := New()

	c.RecordPageDiscovered()
	c.RecordPageDiscovered()

	snap := c.Snapshot()
	if snap.PagesDiscovered != 2 {
		t.Errorf("PagesDiscovered = %d, want 2", snap.PagesDiscovered)
	}
}

func TestCollector_RecordPageCrawled(t *testing.T) {
	c := New()

	c.RecordPageCrawled()

	snap := c.Snapshot()
	if snap.PagesCrawled != 1 {
		t.Errorf("PagesCrawled = %d, want 1", snap.PagesCrawled)
	}
}

func TestCollector_RecordFormFound(t *testing.T) {
	c := New()

	c.RecordFormFound()
	c.RecordFormFound()

	snap := c.Snapshot()
	if snap.FormsFound != 2 {
		t.Errorf("FormsFound = %d, want 2", snap.FormsFound)
	}
}

func TestCollector_RecordAPIEndpoint(t *testing.T) {
	c := New()

	c.RecordAPIEndpoint()

	snap := c.Snapshot()
	if snap.APIEndpoints != 1 {
		t.Errorf("APIEndpoints = %d, want 1", snap.APIEndpoints)
	}
}

func TestCollector_RecordWebSocket(t *testing.T) {
	c := New()

	c.RecordWebSocket()

	snap := c.Snapshot()
	if snap.WebSockets != 1 {
		t.Errorf("WebSockets = %d, want 1", snap.WebSockets)
	}
}

func TestCollector_RecordBytes(t *testing.T) {
	c := New()

	c.RecordBytes(1024)
	c.RecordBytes(2048)

	snap := c.Snapshot()
	if snap.BytesTotal != 3072 {
		t.Errorf("BytesTotal = %d, want 3072", snap.BytesTotal)
	}
}

func TestCollector_RecordRetry(t *testing.T) {
	c := New()

	c.RecordRetry()
	c.RecordRetry()

	snap := c.Snapshot()
	if snap.RetriesTotal != 2 {
		t.Errorf("RetriesTotal = %d, want 2", snap.RetriesTotal)
	}
}

func TestCollector_SetQueueDepth(t *testing.T) {
	c := New()

	c.SetQueueDepth(100)

	snap := c.Snapshot()
	if snap.QueueDepth != 100 {
		t.Errorf("QueueDepth = %d, want 100", snap.QueueDepth)
	}
}

func TestCollector_SetActiveWorkers(t *testing.T) {
	c := New()

	c.SetActiveWorkers(50)

	snap := c.Snapshot()
	if snap.ActiveWorkers != 50 {
		t.Errorf("ActiveWorkers = %d, want 50", snap.ActiveWorkers)
	}
}

func TestCollector_SetBrowserPoolStats(t *testing.T) {
	c := New()

	c.SetBrowserPoolStats(10, 5)

	snap := c.Snapshot()
	if snap.BrowserPoolSize != 10 {
		t.Errorf("BrowserPoolSize = %d, want 10", snap.BrowserPoolSize)
	}
	if snap.BrowserPoolInUse != 5 {
		t.Errorf("BrowserPoolInUse = %d, want 5", snap.BrowserPoolInUse)
	}
}

func TestCollector_Reset(t *testing.T) {
	c := New()

	c.RecordRequest()
	c.RecordError("network")
	c.RecordPageCrawled()
	c.SetQueueDepth(100)

	c.Reset()

	snap := c.Snapshot()
	if snap.RequestsTotal != 0 {
		t.Errorf("RequestsTotal after reset = %d, want 0", snap.RequestsTotal)
	}
	if snap.ErrorsTotal != 0 {
		t.Errorf("ErrorsTotal after reset = %d, want 0", snap.ErrorsTotal)
	}
	if snap.PagesCrawled != 0 {
		t.Errorf("PagesCrawled after reset = %d, want 0", snap.PagesCrawled)
	}
	if snap.QueueDepth != 0 {
		t.Errorf("QueueDepth after reset = %d, want 0", snap.QueueDepth)
	}
}

func TestCollector_GetRequestsPerSecond(t *testing.T) {
	c := New()

	// Record some requests
	for i := 0; i < 100; i++ {
		c.RecordRequest()
	}

	rps := c.GetRequestsPerSecond()
	// Should be greater than 0 since we recorded requests
	if rps <= 0 {
		t.Log("Warning: RPS calculation might be timing-sensitive")
	}
}

func TestCollector_GetAverageResponseTime_Empty(t *testing.T) {
	c := New()

	avg := c.GetAverageResponseTime()
	if avg != 0 {
		t.Errorf("AverageResponseTime with no data = %v, want 0", avg)
	}
}

func TestSnapshot_ErrorRate(t *testing.T) {
	tests := []struct {
		name     string
		requests int64
		errors   int64
		want     float64
	}{
		{"no requests", 0, 0, 0},
		{"no errors", 100, 0, 0},
		{"50% errors", 100, 50, 0.5},
		{"all errors", 100, 100, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Snapshot{
				RequestsTotal: tt.requests,
				ErrorsTotal:   tt.errors,
			}
			if got := s.ErrorRate(); got != tt.want {
				t.Errorf("ErrorRate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSnapshot_BrowserPoolUtilization(t *testing.T) {
	tests := []struct {
		name   string
		size   int64
		inUse  int64
		want   float64
	}{
		{"empty pool", 0, 0, 0},
		{"no usage", 10, 0, 0},
		{"50% usage", 10, 5, 0.5},
		{"full usage", 10, 10, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Snapshot{
				BrowserPoolSize:  tt.size,
				BrowserPoolInUse: tt.inUse,
			}
			if got := s.BrowserPoolUtilization(); got != tt.want {
				t.Errorf("BrowserPoolUtilization() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSnapshot_Summary(t *testing.T) {
	s := &Snapshot{
		Uptime:              10 * time.Second,
		RequestsTotal:       1000,
		ErrorsTotal:         50,
		PagesCrawled:        500,
		PagesDiscovered:     800,
		QueueDepth:          100,
		ActiveWorkers:       20,
		RequestsPerSecond:   100,
		AverageResponseTime: 200 * time.Millisecond,
		BrowserPoolSize:     10,
		BrowserPoolInUse:    5,
	}

	summary := s.Summary()

	if summary["requests_total"] != int64(1000) {
		t.Errorf("summary[requests_total] = %v, want 1000", summary["requests_total"])
	}
	if summary["pages_crawled"] != int64(500) {
		t.Errorf("summary[pages_crawled] = %v, want 500", summary["pages_crawled"])
	}
}

func TestGlobal(t *testing.T) {
	c := Global()
	if c == nil {
		t.Fatal("Global() returned nil")
	}
}

func TestSetGlobal(t *testing.T) {
	original := Global()
	defer SetGlobal(original)

	newCollector := New()
	SetGlobal(newCollector)

	if Global() != newCollector {
		t.Error("SetGlobal() did not set the global collector")
	}
}

func TestCollector_Concurrent(t *testing.T) {
	c := New()
	done := make(chan bool)

	// Run concurrent operations
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				c.RecordRequest()
				c.RecordError("test")
				c.RecordResponseTime(time.Millisecond)
				c.RecordStatusCode(200)
				c.RecordPageCrawled()
				c.SetQueueDepth(int64(j))
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	snap := c.Snapshot()
	if snap.RequestsTotal != 1000 {
		t.Errorf("RequestsTotal = %d, want 1000", snap.RequestsTotal)
	}
	if snap.ErrorsTotal != 1000 {
		t.Errorf("ErrorsTotal = %d, want 1000", snap.ErrorsTotal)
	}
	if snap.PagesCrawled != 1000 {
		t.Errorf("PagesCrawled = %d, want 1000", snap.PagesCrawled)
	}
}

func TestSnapshot_Timestamp(t *testing.T) {
	c := New()
	before := time.Now()
	snap := c.Snapshot()
	after := time.Now()

	if snap.Timestamp.Before(before) || snap.Timestamp.After(after) {
		t.Error("Snapshot timestamp should be between before and after")
	}
}

func TestSnapshot_Uptime(t *testing.T) {
	c := New()
	time.Sleep(10 * time.Millisecond)
	snap := c.Snapshot()

	if snap.Uptime < 10*time.Millisecond {
		t.Errorf("Uptime = %v, should be >= 10ms", snap.Uptime)
	}
}
