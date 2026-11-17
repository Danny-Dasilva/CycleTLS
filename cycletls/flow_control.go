package cycletls

import (
	"context"
	"errors"
	"math"
	"sync"
)

// ErrWindowClosed is returned when trying to acquire from a closed credit window.
var ErrWindowClosed = errors.New("credit window closed")

// -----------------------------------------------------------------------------
// Frame sender
// -----------------------------------------------------------------------------

// frameSender sends byte buffers through a channel with context cancellation support.
type frameSender struct {
	ctx context.Context
	out chan<- []byte
}

// newFrameSender creates a new frame sender.
func newFrameSender(ctx context.Context, out chan<- []byte) *frameSender {
	return &frameSender{
		ctx: ctx,
		out: out,
	}
}

// send transmits the buffer as-is.
// The caller must not modify buf after calling send.
func (s *frameSender) send(buf []byte) bool {
	select {
	case <-s.ctx.Done():
		return false
	case s.out <- buf:
		return true
	}
}

// -----------------------------------------------------------------------------
// Credit window (weighted semaphore)
// -----------------------------------------------------------------------------

// creditWindow is a weighted semaphore for flow control.
// A nil pointer represents infinite credit capacity.
//
// Uses a channel-based broadcast mechanism instead of sync.Cond to allow
// integration with context cancellation without spawning per-Acquire goroutines.
type creditWindow struct {
	mu     sync.Mutex
	window int64
	closed bool
	// broadcast is a channel that gets closed to wake all waiters (replaces sync.Cond).
	// After closing, a new channel is created for the next round of waiters.
	broadcast chan struct{}
}

// newCreditWindow creates a new credit window with the specified initial capacity.
func newCreditWindow(initial int64) *creditWindow {
	if initial < 0 {
		panic("creditWindow: negative initial window")
	}

	return &creditWindow{
		window:    initial,
		broadcast: make(chan struct{}),
	}
}

// guard checks if the request is trivial or infinite.
func (cw *creditWindow) guard(n int64) bool {
	return cw == nil || n <= 0
}

// wake closes the current broadcast channel and creates a new one.
// Caller must hold cw.mu.
func (cw *creditWindow) wake() {
	close(cw.broadcast)
	cw.broadcast = make(chan struct{})
}

// Acquire blocks until n credits are available, then consumes them atomically.
// The wait is cancellable via the context. No goroutines are spawned.
func (cw *creditWindow) Acquire(ctx context.Context, n int64) error {
	if cw.guard(n) {
		return nil
	}

	cw.mu.Lock()
	for {
		if cw.closed {
			cw.mu.Unlock()
			return ErrWindowClosed
		}

		if cw.window >= n {
			cw.window -= n
			cw.mu.Unlock()
			return nil
		}

		if err := ctx.Err(); err != nil {
			cw.mu.Unlock()
			return err
		}

		// Grab the current broadcast channel while holding the lock.
		// When Add or Close is called, this channel will be closed.
		ch := cw.broadcast
		cw.mu.Unlock()

		// Wait for either a broadcast or context cancellation without holding the lock.
		select {
		case <-ch:
			// Credits may have been added or window closed - re-check
		case <-ctx.Done():
			return ctx.Err()
		}

		cw.mu.Lock()
	}
}

// TryAcquire attempts to consume n credits without blocking.
func (cw *creditWindow) TryAcquire(n int64) bool {
	if cw.guard(n) {
		return true
	}

	cw.mu.Lock()
	defer cw.mu.Unlock()

	if cw.closed || cw.window < n {
		return false
	}

	cw.window -= n
	return true
}

// Add adds credits and wakes up waiting goroutines.
// Includes overflow protection: saturates at MaxInt64 instead of wrapping.
// Panics on negative values. Zero is a no-op.
func (cw *creditWindow) Add(n int64) {
	if cw == nil {
		return
	}
	if n < 0 {
		panic("creditWindow: negative credit value")
	}
	if n == 0 {
		return
	}

	cw.mu.Lock()
	defer cw.mu.Unlock()

	if cw.closed {
		return
	}

	// Overflow protection: saturate at MaxInt64 instead of wrapping to negative
	if n > 0 && cw.window > math.MaxInt64-n {
		cw.window = math.MaxInt64
	} else {
		cw.window += n
	}
	cw.wake()
}

// Close logically closes the window and wakes all waiters.
func (cw *creditWindow) Close() {
	if cw == nil {
		return
	}

	cw.mu.Lock()
	defer cw.mu.Unlock()

	if cw.closed {
		return
	}

	cw.closed = true
	cw.wake()
}
