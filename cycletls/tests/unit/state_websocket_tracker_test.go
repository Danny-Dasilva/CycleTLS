//go:build !integration

package unit

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Danny-Dasilva/CycleTLS/cycletls/state"
)

// mockWSConnection is a simple struct used as a mock WebSocket connection for testing.
type mockWSConnection struct {
	ID   string
	Data string
}

func TestRegisterAndGetWebSocket(t *testing.T) {
	t.Parallel()

	conn := &mockWSConnection{ID: "test-ws-1", Data: "test data"}
	wsID := "ws-test-1"

	state.RegisterWebSocket(wsID, conn)
	defer state.UnregisterWebSocket(wsID)

	retrieved, exists := state.GetWebSocket(wsID)

	if !exists {
		t.Fatal("GetWebSocket: expected exists to be true for registered connection")
	}

	if retrieved != conn {
		t.Fatal("GetWebSocket: retrieved connection does not match registered connection")
	}

	// Verify type assertion works
	typedConn, ok := retrieved.(*mockWSConnection)
	if !ok {
		t.Fatal("GetWebSocket: could not type-assert retrieved connection")
	}

	if typedConn.ID != "test-ws-1" || typedConn.Data != "test data" {
		t.Fatalf("GetWebSocket: type-asserted connection has wrong data: got ID=%s, Data=%s", typedConn.ID, typedConn.Data)
	}
}

func TestGetWebSocket_NotExists(t *testing.T) {
	t.Parallel()

	retrieved, exists := state.GetWebSocket("ws-non-existent-id")

	if exists {
		t.Fatal("GetWebSocket: expected exists to be false for non-existent connection")
	}

	if retrieved != nil {
		t.Fatal("GetWebSocket: expected nil for non-existent connection")
	}
}

func TestUnregisterWebSocket(t *testing.T) {
	t.Parallel()

	conn := &mockWSConnection{ID: "test-ws-2", Data: "data to remove"}
	wsID := "ws-test-2"

	state.RegisterWebSocket(wsID, conn)

	// Verify it exists before unregistering
	_, exists := state.GetWebSocket(wsID)
	if !exists {
		t.Fatal("UnregisterWebSocket: connection should exist before unregistering")
	}

	state.UnregisterWebSocket(wsID)

	// Verify it no longer exists
	_, exists = state.GetWebSocket(wsID)
	if exists {
		t.Fatal("UnregisterWebSocket: connection should not exist after unregistering")
	}
}

func TestUnregisterWebSocket_NonExistent(t *testing.T) {
	t.Parallel()

	// Should not panic when unregistering non-existent ID
	state.UnregisterWebSocket("ws-does-not-exist")
}

func TestMultipleWebSocketConnections(t *testing.T) {
	t.Parallel()

	connections := make(map[string]*mockWSConnection)
	for i := 0; i < 10; i++ {
		id := "ws-multi-test-" + string(rune('a'+i))
		conn := &mockWSConnection{ID: id, Data: "data-" + id}
		connections[id] = conn
		state.RegisterWebSocket(id, conn)
	}

	// Cleanup at end
	defer func() {
		for id := range connections {
			state.UnregisterWebSocket(id)
		}
	}()

	// Verify all connections can be retrieved
	for id, expectedConn := range connections {
		retrieved, exists := state.GetWebSocket(id)
		if !exists {
			t.Fatalf("MultipleConnections: connection %s should exist", id)
		}
		if retrieved != expectedConn {
			t.Fatalf("MultipleConnections: connection %s does not match", id)
		}
	}

	// Unregister some and verify others still exist
	state.UnregisterWebSocket("ws-multi-test-a")
	state.UnregisterWebSocket("ws-multi-test-c")
	state.UnregisterWebSocket("ws-multi-test-e")

	_, exists := state.GetWebSocket("ws-multi-test-a")
	if exists {
		t.Fatal("MultipleConnections: ws-multi-test-a should be unregistered")
	}

	_, exists = state.GetWebSocket("ws-multi-test-b")
	if !exists {
		t.Fatal("MultipleConnections: ws-multi-test-b should still exist")
	}

	_, exists = state.GetWebSocket("ws-multi-test-d")
	if !exists {
		t.Fatal("MultipleConnections: ws-multi-test-d should still exist")
	}
}

func TestWebSocketConcurrentAccess(t *testing.T) {
	t.Parallel()

	const numGoroutines = 20
	const numOperations = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2)

	// Concurrent registrations
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				id := "ws-concurrent-" + string(rune('a'+idx%26)) + "-" + string(rune('0'+j%10))
				conn := &mockWSConnection{ID: id, Data: "concurrent data"}
				state.RegisterWebSocket(id, conn)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				id := "ws-concurrent-" + string(rune('a'+idx%26)) + "-" + string(rune('0'+j%10))
				state.GetWebSocket(id)
			}
		}(i)
	}

	wg.Wait()
}

func TestWebSocketConcurrentRegisterUnregister(t *testing.T) {
	t.Parallel()

	const numGoroutines = 10
	const numOperations = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2)

	// Concurrent registrations
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				id := "ws-reg-unreg-" + string(rune('a'+idx%26))
				conn := &mockWSConnection{ID: id}
				state.RegisterWebSocket(id, conn)
			}
		}(i)
	}

	// Concurrent unregistrations
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				id := "ws-reg-unreg-" + string(rune('a'+idx%26))
				state.UnregisterWebSocket(id)
			}
		}(i)
	}

	wg.Wait()
}

func TestRegisterWebSocketOverwrite(t *testing.T) {
	t.Parallel()

	conn1 := &mockWSConnection{ID: "original", Data: "original data"}
	conn2 := &mockWSConnection{ID: "replacement", Data: "replacement data"}
	wsID := "ws-overwrite-test"

	state.RegisterWebSocket(wsID, conn1)
	defer state.UnregisterWebSocket(wsID)

	// Verify first connection
	retrieved, _ := state.GetWebSocket(wsID)
	typedConn := retrieved.(*mockWSConnection)
	if typedConn.Data != "original data" {
		t.Fatal("RegisterOverwrite: first registration should store original data")
	}

	// Overwrite with new connection
	state.RegisterWebSocket(wsID, conn2)

	// Verify overwrite
	retrieved, _ = state.GetWebSocket(wsID)
	typedConn = retrieved.(*mockWSConnection)
	if typedConn.Data != "replacement data" {
		t.Fatal("RegisterOverwrite: second registration should overwrite with replacement data")
	}
}

// Issue #12 (MAJOR): Thread-safe concurrent access on the SAME key
func TestWebSocketConcurrentSameKey(t *testing.T) {
	t.Parallel()

	const numGoroutines = 50
	const opsPerGoroutine = 100
	wsID := "ws-same-key-concurrent"

	var wg sync.WaitGroup
	var panicked atomic.Int64

	wg.Add(numGoroutines * 3)

	// Registerers
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicked.Add(1)
				}
			}()
			for j := 0; j < opsPerGoroutine; j++ {
				conn := &mockWSConnection{ID: wsID, Data: "data"}
				state.RegisterWebSocket(wsID, conn)
			}
		}(i)
	}

	// Readers
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicked.Add(1)
				}
			}()
			for j := 0; j < opsPerGoroutine; j++ {
				conn, exists := state.GetWebSocket(wsID)
				if exists && conn != nil {
					// Verify type assertion safety under concurrency
					if _, ok := conn.(*mockWSConnection); !ok {
						t.Errorf("Type assertion failed on concurrent read")
					}
				}
			}
		}(i)
	}

	// Unregisterers
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicked.Add(1)
				}
			}()
			for j := 0; j < opsPerGoroutine; j++ {
				state.UnregisterWebSocket(wsID)
			}
		}(i)
	}

	wg.Wait()

	if p := panicked.Load(); p > 0 {
		t.Errorf("Concurrent same-key operations panicked %d times", p)
	}
}

// Issue #12 (MAJOR): Register->Get->Unregister cycle under concurrent load
func TestWebSocketConcurrentRegisterGetUnregisterCycle(t *testing.T) {
	t.Parallel()

	const numGoroutines = 30
	const opsPerGoroutine = 50

	var wg sync.WaitGroup
	var panicked atomic.Int64
	var getSuccesses atomic.Int64

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicked.Add(1)
				}
			}()
			for j := 0; j < opsPerGoroutine; j++ {
				id := "ws-cycle-" + string(rune('a'+idx%26))
				conn := &mockWSConnection{ID: id}

				// Full lifecycle: register -> get -> unregister
				state.RegisterWebSocket(id, conn)

				retrieved, exists := state.GetWebSocket(id)
				if exists && retrieved != nil {
					getSuccesses.Add(1)
				}

				state.UnregisterWebSocket(id)
			}
		}(i)
	}

	wg.Wait()

	if p := panicked.Load(); p > 0 {
		t.Errorf("Concurrent register-get-unregister cycle panicked %d times", p)
	}
	t.Logf("Successful gets during concurrent cycles: %d", getSuccesses.Load())
}
