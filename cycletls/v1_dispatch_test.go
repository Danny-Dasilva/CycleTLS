package cycletls

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// C1 regression: the V1 dispatch path routes response/data/end frames through the
// blocking, context-aware writeBlockingCtx. Under concurrency with a slow consumer,
// no critical frame may be dropped and per-stream ordering must be preserved.
//
// This exercises the safeChannelWriter mechanism directly (as dispatcherAsync uses
// it): many goroutines each push an ordered sequence of frames for a distinct
// stream through one buffered channel drained by a deliberately slow consumer.
func TestSafeChannelWriter_BlockingNoDropUnderConcurrency(t *testing.T) {
	const streams = 16
	const framesPerStream = 50 // response + data chunks + end, encoded as seq 0..49

	ch := make(chan []byte, 4) // small buffer forces real backpressure
	scw := newSafeChannelWriter(ch)

	received := make(map[byte][]byte) // streamID -> ordered seqs received
	var mu sync.Mutex
	var total int64
	consumerDone := make(chan struct{})

	go func() {
		defer close(consumerDone)
		for buf := range ch {
			// Simulate a slow network write so senders must block, not drop.
			time.Sleep(50 * time.Microsecond)
			mu.Lock()
			received[buf[0]] = append(received[buf[0]], buf[1])
			mu.Unlock()
			if atomic.AddInt64(&total, 1) == int64(streams*framesPerStream) {
				return
			}
		}
	}()

	var wg sync.WaitGroup
	var dropped int64
	for s := 0; s < streams; s++ {
		wg.Add(1)
		go func(id byte) {
			defer wg.Done()
			for seq := 0; seq < framesPerStream; seq++ {
				frame := []byte{id, byte(seq)}
				if !scw.writeBlockingCtx(frame, context.Background()) {
					atomic.AddInt64(&dropped, 1)
				}
			}
		}(byte(s))
	}
	wg.Wait()
	<-consumerDone

	if dropped != 0 {
		t.Fatalf("blocking writer dropped %d frames; response/data/end must never drop", dropped)
	}
	if got := atomic.LoadInt64(&total); got != int64(streams*framesPerStream) {
		t.Fatalf("consumer received %d frames, want %d", got, streams*framesPerStream)
	}

	mu.Lock()
	defer mu.Unlock()
	for s := 0; s < streams; s++ {
		seqs := received[byte(s)]
		if len(seqs) != framesPerStream {
			t.Fatalf("stream %d: got %d frames, want %d", s, len(seqs), framesPerStream)
		}
		for i, v := range seqs {
			if int(v) != i {
				t.Fatalf("stream %d: frame at position %d out of order, got seq %d", s, i, v)
			}
		}
	}
}

// C1: a cancelled request context must unblock a pending critical send so a dead
// client can never wedge a dispatcher goroutine forever.
func TestSafeChannelWriter_ContextCancelUnblocks(t *testing.T) {
	ch := make(chan []byte) // unbuffered, no consumer -> send blocks
	scw := newSafeChannelWriter(ch)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan bool, 1)
	go func() {
		done <- scw.writeBlockingCtx([]byte{1}, ctx)
	}()

	time.Sleep(20 * time.Millisecond) // let the send block
	cancel()

	select {
	case ok := <-done:
		if ok {
			t.Fatal("expected writeBlockingCtx to return false on context cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("writeBlockingCtx did not unblock on context cancel (dead client would hang)")
	}
}

// C1: writer shutdown (readSocket exit) must unblock pending sends and further
// sends must return false without panicking. The underlying channel is never
// closed, so a send-to-closed-channel panic is structurally impossible.
func TestSafeChannelWriter_ShutdownUnblocks(t *testing.T) {
	ch := make(chan []byte) // unbuffered, no consumer -> send blocks
	scw := newSafeChannelWriter(ch)

	done := make(chan bool, 1)
	go func() {
		done <- scw.writeBlockingCtx([]byte{1}, context.Background())
	}()

	time.Sleep(20 * time.Millisecond) // let the send block
	scw.setClosed()                   // simulate readSocket exit / writer shutdown

	select {
	case ok := <-done:
		if ok {
			t.Fatal("expected writeBlockingCtx to return false after setClosed")
		}
	case <-time.After(time.Second):
		t.Fatal("writeBlockingCtx did not unblock on setClosed")
	}

	if scw.writeBlockingCtx([]byte{2}, context.Background()) {
		t.Fatal("writeBlockingCtx after setClosed should return false")
	}
}

// Contrast with the old behavior that caused C1: the non-blocking write() drops
// when the consumer is not keeping up. This is exactly why critical frames were
// moved to the blocking path above.
func TestSafeChannelWriter_NonBlockingWriteDropsUnderPressure(t *testing.T) {
	ch := make(chan []byte) // unbuffered, no ready receiver
	scw := newSafeChannelWriter(ch)

	if scw.write([]byte{1}) {
		t.Fatal("expected non-blocking write to drop when no receiver is ready")
	}
}
