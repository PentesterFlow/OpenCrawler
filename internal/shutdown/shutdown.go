// Package shutdown provides graceful shutdown handling for the DAST crawler.
package shutdown

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Handler manages graceful shutdown.
type Handler struct {
	mu sync.Mutex

	// Callbacks
	callbacks     []ShutdownCallback
	callbackNames []string

	// State
	isShuttingDown atomic.Bool
	done           chan struct{}
	timeout        time.Duration

	// Context
	ctx    context.Context
	cancel context.CancelFunc

	// Signal handling
	sigChan chan os.Signal

	// Notification
	onShutdownStart func()
	onShutdownDone  func(elapsed time.Duration, errors []error)
}

// ShutdownCallback is a function called during shutdown.
type ShutdownCallback func(ctx context.Context) error

// Config holds shutdown configuration.
type Config struct {
	Timeout         time.Duration
	Signals         []os.Signal
	OnShutdownStart func()
	OnShutdownDone  func(elapsed time.Duration, errors []error)
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		Timeout: 30 * time.Second,
		Signals: []os.Signal{syscall.SIGINT, syscall.SIGTERM},
	}
}

// New creates a new shutdown handler.
func New(cfg Config) *Handler {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if len(cfg.Signals) == 0 {
		cfg.Signals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	}

	ctx, cancel := context.WithCancel(context.Background())

	h := &Handler{
		callbacks:       make([]ShutdownCallback, 0),
		callbackNames:   make([]string, 0),
		done:            make(chan struct{}),
		timeout:         cfg.Timeout,
		ctx:             ctx,
		cancel:          cancel,
		sigChan:         make(chan os.Signal, 1),
		onShutdownStart: cfg.OnShutdownStart,
		onShutdownDone:  cfg.OnShutdownDone,
	}

	signal.Notify(h.sigChan, cfg.Signals...)

	return h
}

// NewDefault creates a handler with default configuration.
func NewDefault() *Handler {
	return New(DefaultConfig())
}

// Register registers a shutdown callback with a name.
func (h *Handler) Register(name string, callback ShutdownCallback) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.callbacks = append(h.callbacks, callback)
	h.callbackNames = append(h.callbackNames, name)
}

// RegisterFunc registers a simple cleanup function.
func (h *Handler) RegisterFunc(name string, fn func()) {
	h.Register(name, func(ctx context.Context) error {
		fn()
		return nil
	})
}

// Context returns the shutdown context.
// This context is cancelled when shutdown begins.
func (h *Handler) Context() context.Context {
	return h.ctx
}

// IsShuttingDown returns whether shutdown is in progress.
func (h *Handler) IsShuttingDown() bool {
	return h.isShuttingDown.Load()
}

// Done returns a channel that is closed when shutdown completes.
func (h *Handler) Done() <-chan struct{} {
	return h.done
}

// Wait blocks until a shutdown signal is received.
func (h *Handler) Wait() {
	select {
	case <-h.sigChan:
		h.Shutdown()
	case <-h.ctx.Done():
		// Already shutting down
	}
}

// WaitWithContext waits for shutdown or context cancellation.
func (h *Handler) WaitWithContext(ctx context.Context) {
	select {
	case <-h.sigChan:
		h.Shutdown()
	case <-ctx.Done():
		h.Shutdown()
	case <-h.ctx.Done():
		// Already shutting down
	}
}

// ListenAndShutdown starts listening for signals and handles shutdown.
// Returns a channel that receives when shutdown is complete.
func (h *Handler) ListenAndShutdown() <-chan struct{} {
	go h.Wait()
	return h.done
}

// Shutdown initiates graceful shutdown.
func (h *Handler) Shutdown() {
	if !h.isShuttingDown.CompareAndSwap(false, true) {
		// Already shutting down
		return
	}

	start := time.Now()

	// Notify start
	if h.onShutdownStart != nil {
		h.onShutdownStart()
	}

	// Cancel context to signal all operations to stop
	h.cancel()

	// Create timeout context for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), h.timeout)
	defer shutdownCancel()

	// Execute callbacks in reverse order (LIFO)
	var errors []error
	h.mu.Lock()
	callbacks := make([]ShutdownCallback, len(h.callbacks))
	names := make([]string, len(h.callbackNames))
	copy(callbacks, h.callbacks)
	copy(names, h.callbackNames)
	h.mu.Unlock()

	for i := len(callbacks) - 1; i >= 0; i-- {
		err := h.executeCallback(shutdownCtx, names[i], callbacks[i])
		if err != nil {
			errors = append(errors, err)
		}
	}

	elapsed := time.Since(start)

	// Notify done
	if h.onShutdownDone != nil {
		h.onShutdownDone(elapsed, errors)
	}

	close(h.done)
}

// executeCallback executes a shutdown callback with timeout handling.
func (h *Handler) executeCallback(ctx context.Context, name string, callback ShutdownCallback) error {
	done := make(chan error, 1)

	go func() {
		done <- callback(ctx)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return &TimeoutError{CallbackName: name}
	}
}

// ShutdownNow initiates immediate shutdown without waiting for callbacks.
func (h *Handler) ShutdownNow() {
	if !h.isShuttingDown.CompareAndSwap(false, true) {
		return
	}

	h.cancel()
	close(h.done)
}

// Trigger manually triggers shutdown (for testing or programmatic shutdown).
func (h *Handler) Trigger() {
	select {
	case h.sigChan <- syscall.SIGTERM:
	default:
		// Signal already pending
	}
}

// TimeoutError is returned when a callback times out.
type TimeoutError struct {
	CallbackName string
}

func (e *TimeoutError) Error() string {
	return "shutdown callback timed out: " + e.CallbackName
}

// ShutdownResult holds the result of a shutdown operation.
type ShutdownResult struct {
	Elapsed time.Duration
	Errors  []error
}

// HasErrors returns whether any errors occurred during shutdown.
func (r *ShutdownResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// GracefulServer wraps a component that needs graceful shutdown.
type GracefulServer interface {
	Shutdown(ctx context.Context) error
}

// RegisterServer registers a GracefulServer for shutdown.
func (h *Handler) RegisterServer(name string, server GracefulServer) {
	h.Register(name, server.Shutdown)
}

// Coordinator coordinates multiple shutdown handlers.
type Coordinator struct {
	mu       sync.Mutex
	handlers []*Handler
}

// NewCoordinator creates a new coordinator.
func NewCoordinator() *Coordinator {
	return &Coordinator{
		handlers: make([]*Handler, 0),
	}
}

// Add adds a handler to the coordinator.
func (c *Coordinator) Add(h *Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers = append(c.handlers, h)
}

// ShutdownAll triggers shutdown on all handlers.
func (c *Coordinator) ShutdownAll() {
	c.mu.Lock()
	handlers := make([]*Handler, len(c.handlers))
	copy(handlers, c.handlers)
	c.mu.Unlock()

	var wg sync.WaitGroup
	for _, h := range handlers {
		wg.Add(1)
		go func(handler *Handler) {
			defer wg.Done()
			handler.Shutdown()
		}(h)
	}
	wg.Wait()
}

// WaitAll waits for all handlers to complete shutdown.
func (c *Coordinator) WaitAll() {
	c.mu.Lock()
	handlers := make([]*Handler, len(c.handlers))
	copy(handlers, c.handlers)
	c.mu.Unlock()

	for _, h := range handlers {
		<-h.Done()
	}
}

// Global shutdown handler.
var globalHandler *Handler
var globalOnce sync.Once

// Global returns the global shutdown handler.
func Global() *Handler {
	globalOnce.Do(func() {
		globalHandler = NewDefault()
	})
	return globalHandler
}

// SetGlobal sets the global shutdown handler.
func SetGlobal(h *Handler) {
	globalHandler = h
}

// Register registers a callback with the global handler.
func Register(name string, callback ShutdownCallback) {
	Global().Register(name, callback)
}

// RegisterFunc registers a function with the global handler.
func RegisterFunc(name string, fn func()) {
	Global().RegisterFunc(name, fn)
}

// Shutdown triggers shutdown on the global handler.
func Shutdown() {
	Global().Shutdown()
}
