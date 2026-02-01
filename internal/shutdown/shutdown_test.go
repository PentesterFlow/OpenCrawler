package shutdown

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	h := New(DefaultConfig())
	if h == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNewDefault(t *testing.T) {
	h := NewDefault()
	if h == nil {
		t.Fatal("NewDefault() returned nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
	}
	if len(cfg.Signals) != 2 {
		t.Errorf("Signals length = %d, want 2", len(cfg.Signals))
	}
}

func TestHandler_Register(t *testing.T) {
	h := NewDefault()
	called := false

	h.Register("test", func(ctx context.Context) error {
		called = true
		return nil
	})

	h.Shutdown()
	<-h.Done()

	if !called {
		t.Error("Callback was not called")
	}
}

func TestHandler_RegisterFunc(t *testing.T) {
	h := NewDefault()
	called := false

	h.RegisterFunc("test", func() {
		called = true
	})

	h.Shutdown()
	<-h.Done()

	if !called {
		t.Error("Function was not called")
	}
}

func TestHandler_Context(t *testing.T) {
	h := NewDefault()
	ctx := h.Context()

	if ctx == nil {
		t.Fatal("Context() returned nil")
	}

	// Context should not be done initially
	select {
	case <-ctx.Done():
		t.Error("Context should not be done initially")
	default:
		// OK
	}

	h.Shutdown()

	// Context should be done after shutdown
	select {
	case <-ctx.Done():
		// OK
	case <-time.After(time.Second):
		t.Error("Context should be done after shutdown")
	}
}

func TestHandler_IsShuttingDown(t *testing.T) {
	h := NewDefault()

	if h.IsShuttingDown() {
		t.Error("Should not be shutting down initially")
	}

	h.Shutdown()

	if !h.IsShuttingDown() {
		t.Error("Should be shutting down after Shutdown()")
	}
}

func TestHandler_Done(t *testing.T) {
	h := NewDefault()

	// Should not be closed initially
	select {
	case <-h.Done():
		t.Error("Done channel should not be closed initially")
	default:
		// OK
	}

	h.Shutdown()

	// Should be closed after shutdown
	select {
	case <-h.Done():
		// OK
	case <-time.After(time.Second):
		t.Error("Done channel should be closed after shutdown")
	}
}

func TestHandler_Shutdown_LIFO(t *testing.T) {
	h := NewDefault()
	order := make([]int, 0, 3)

	h.Register("first", func(ctx context.Context) error {
		order = append(order, 1)
		return nil
	})
	h.Register("second", func(ctx context.Context) error {
		order = append(order, 2)
		return nil
	})
	h.Register("third", func(ctx context.Context) error {
		order = append(order, 3)
		return nil
	})

	h.Shutdown()
	<-h.Done()

	// Should be LIFO order
	if len(order) != 3 {
		t.Fatalf("Expected 3 callbacks, got %d", len(order))
	}
	if order[0] != 3 || order[1] != 2 || order[2] != 1 {
		t.Errorf("Order = %v, want [3, 2, 1] (LIFO)", order)
	}
}

func TestHandler_Shutdown_MultipleCallsIdempotent(t *testing.T) {
	h := NewDefault()
	callCount := 0

	h.Register("test", func(ctx context.Context) error {
		callCount++
		return nil
	})

	// Call shutdown multiple times
	h.Shutdown()
	h.Shutdown()
	h.Shutdown()

	<-h.Done()

	if callCount != 1 {
		t.Errorf("Callback called %d times, want 1", callCount)
	}
}

func TestHandler_Shutdown_WithCallbacks(t *testing.T) {
	startCalled := false
	doneCalled := false
	var doneElapsed time.Duration
	var doneErrors []error

	h := New(Config{
		Timeout: 5 * time.Second,
		OnShutdownStart: func() {
			startCalled = true
		},
		OnShutdownDone: func(elapsed time.Duration, errors []error) {
			doneCalled = true
			doneElapsed = elapsed
			doneErrors = errors
		},
	})

	h.Shutdown()
	<-h.Done()

	if !startCalled {
		t.Error("OnShutdownStart was not called")
	}
	if !doneCalled {
		t.Error("OnShutdownDone was not called")
	}
	if doneElapsed <= 0 {
		t.Error("Elapsed time should be positive")
	}
	if len(doneErrors) != 0 {
		t.Errorf("Expected no errors, got %v", doneErrors)
	}
}

func TestHandler_Shutdown_WithErrors(t *testing.T) {
	var doneErrors []error

	h := New(Config{
		Timeout: 5 * time.Second,
		OnShutdownDone: func(elapsed time.Duration, errors []error) {
			doneErrors = errors
		},
	})

	testErr := errors.New("test error")
	h.Register("failing", func(ctx context.Context) error {
		return testErr
	})

	h.Shutdown()
	<-h.Done()

	if len(doneErrors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(doneErrors))
	}
}

func TestHandler_ShutdownNow(t *testing.T) {
	h := NewDefault()
	called := false

	h.Register("test", func(ctx context.Context) error {
		called = true
		time.Sleep(time.Second) // Slow callback
		return nil
	})

	h.ShutdownNow()

	select {
	case <-h.Done():
		// OK - should complete immediately
	case <-time.After(100 * time.Millisecond):
		t.Error("ShutdownNow should complete immediately")
	}

	if called {
		t.Error("Callback should not be called with ShutdownNow")
	}
}

func TestHandler_Trigger(t *testing.T) {
	h := NewDefault()

	go func() {
		time.Sleep(10 * time.Millisecond)
		h.Trigger()
	}()

	h.Wait()

	if !h.IsShuttingDown() {
		t.Error("Should be shutting down after Trigger()")
	}
}

func TestHandler_WaitWithContext(t *testing.T) {
	h := NewDefault()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	go h.WaitWithContext(ctx)

	select {
	case <-h.Done():
		// OK
	case <-time.After(time.Second):
		t.Error("Should shutdown after context timeout")
	}
}

func TestHandler_ListenAndShutdown(t *testing.T) {
	h := NewDefault()

	done := h.ListenAndShutdown()

	// Trigger shutdown
	h.Trigger()

	select {
	case <-done:
		// OK
	case <-time.After(time.Second):
		t.Error("ListenAndShutdown should complete")
	}
}

func TestHandler_Timeout(t *testing.T) {
	h := New(Config{
		Timeout: 50 * time.Millisecond,
	})

	h.Register("slow", func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
			return nil
		}
	})

	start := time.Now()
	h.Shutdown()
	<-h.Done()
	elapsed := time.Since(start)

	if elapsed > 200*time.Millisecond {
		t.Errorf("Shutdown took %v, should timeout faster", elapsed)
	}
}

func TestTimeoutError(t *testing.T) {
	err := &TimeoutError{CallbackName: "test"}

	if err.Error() != "shutdown callback timed out: test" {
		t.Errorf("Error() = %s", err.Error())
	}
}

func TestShutdownResult(t *testing.T) {
	r1 := &ShutdownResult{
		Elapsed: time.Second,
		Errors:  nil,
	}
	if r1.HasErrors() {
		t.Error("HasErrors() should be false with no errors")
	}

	r2 := &ShutdownResult{
		Elapsed: time.Second,
		Errors:  []error{errors.New("test")},
	}
	if !r2.HasErrors() {
		t.Error("HasErrors() should be true with errors")
	}
}

func TestHandler_RegisterServer(t *testing.T) {
	h := NewDefault()
	server := &mockServer{}

	h.RegisterServer("mock", server)

	h.Shutdown()
	<-h.Done()

	if !server.shutdownCalled {
		t.Error("Server.Shutdown was not called")
	}
}

type mockServer struct {
	shutdownCalled bool
}

func (s *mockServer) Shutdown(ctx context.Context) error {
	s.shutdownCalled = true
	return nil
}

func TestCoordinator(t *testing.T) {
	c := NewCoordinator()

	h1 := NewDefault()
	h2 := NewDefault()

	var h1Called, h2Called atomic.Bool

	h1.Register("h1", func(ctx context.Context) error {
		h1Called.Store(true)
		return nil
	})
	h2.Register("h2", func(ctx context.Context) error {
		h2Called.Store(true)
		return nil
	})

	c.Add(h1)
	c.Add(h2)

	c.ShutdownAll()
	c.WaitAll()

	if !h1Called.Load() {
		t.Error("h1 callback was not called")
	}
	if !h2Called.Load() {
		t.Error("h2 callback was not called")
	}
}

func TestGlobal(t *testing.T) {
	h := Global()
	if h == nil {
		t.Fatal("Global() returned nil")
	}

	// Should return same instance
	h2 := Global()
	if h != h2 {
		t.Error("Global() should return same instance")
	}
}

func TestSetGlobal(t *testing.T) {
	original := Global()
	defer SetGlobal(original)

	newHandler := NewDefault()
	SetGlobal(newHandler)

	if Global() != newHandler {
		t.Error("SetGlobal() did not set the handler")
	}
}

func TestGlobalRegister(t *testing.T) {
	// Create a fresh handler for this test
	h := NewDefault()
	SetGlobal(h)
	defer SetGlobal(NewDefault())

	called := false
	Register("global-test", func(ctx context.Context) error {
		called = true
		return nil
	})

	Shutdown()
	<-h.Done()

	if !called {
		t.Error("Global Register callback was not called")
	}
}

func TestGlobalRegisterFunc(t *testing.T) {
	// Create a fresh handler for this test
	h := NewDefault()
	SetGlobal(h)
	defer SetGlobal(NewDefault())

	called := false
	RegisterFunc("global-test-func", func() {
		called = true
	})

	Shutdown()
	<-h.Done()

	if !called {
		t.Error("Global RegisterFunc callback was not called")
	}
}

func TestHandler_Concurrent(t *testing.T) {
	h := NewDefault()
	var callCount atomic.Int64

	for i := 0; i < 10; i++ {
		h.Register("callback", func(ctx context.Context) error {
			callCount.Add(1)
			return nil
		})
	}

	// Trigger concurrent shutdowns
	for i := 0; i < 5; i++ {
		go h.Shutdown()
	}

	<-h.Done()

	if callCount.Load() != 10 {
		t.Errorf("CallCount = %d, want 10", callCount.Load())
	}
}
