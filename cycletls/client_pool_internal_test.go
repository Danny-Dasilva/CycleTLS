//go:build !integration

package cycletls

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestGetOrCreateClient_ConcurrentReuse hammers getOrCreateClient from many
// goroutines with a mix of one shared config and per-goroutine distinct configs.
// It must be race-free (run under -race), reuse a single *entry for the shared
// key, and allocate a distinct entry per distinct key.
func TestGetOrCreateClient_ConcurrentReuse(t *testing.T) {
	clearAllConnections()
	defer clearAllConnections()

	const goroutines = 64
	transports := make([]interface{}, goroutines)

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			b := Browser{UserAgent: "ua"}
			if idx%2 == 0 {
				b.JA3 = "SHARED" // even goroutines all share one config
			} else {
				b.JA3 = fmt.Sprintf("distinct-%d", idx) // odd goroutines are unique
			}
			c, err := getOrCreateClient(b, 30, false, "ua", true)
			if err != nil {
				t.Errorf("getOrCreateClient error: %v", err)
				return
			}
			transports[idx] = c.Transport
		}(i)
	}
	wg.Wait()

	// Every shared-config goroutine must have received the SAME transport pointer.
	var sharedTransport interface{}
	for i := 0; i < goroutines; i += 2 {
		if sharedTransport == nil {
			sharedTransport = transports[i]
			continue
		}
		if transports[i] != sharedTransport {
			t.Errorf("shared-config goroutine %d got a different transport; reuse broken", i)
		}
	}

	// Distinct-config goroutines must each hold a unique transport.
	seen := map[interface{}]int{}
	for i := 1; i < goroutines; i += 2 {
		seen[transports[i]]++
	}
	for tr, n := range seen {
		if n != 1 {
			t.Errorf("distinct-config transport reused %d times: %v", n, tr)
		}
	}

	// Pool holds exactly: 1 shared entry + one per distinct odd goroutine.
	wantDistinct := goroutines / 2
	advancedClientPoolMutex.RLock()
	got := len(advancedClientPool)
	advancedClientPoolMutex.RUnlock()
	if got != wantDistinct+1 {
		t.Errorf("pool size = %d, want %d (1 shared + %d distinct)", got, wantDistinct+1, wantDistinct)
	}
}

// TestClientPool_BoundedAtInsert inserts far more distinct clients than the
// bound and asserts the pool never exceeds maxClientPoolSize (LRU eviction at
// insert), so it cannot grow without limit (M3).
func TestClientPool_BoundedAtInsert(t *testing.T) {
	clearAllConnections()
	defer clearAllConnections()

	for i := 0; i < maxClientPoolSize*2; i++ {
		b := Browser{JA3: fmt.Sprintf("bound-%d", i), UserAgent: "ua"}
		if _, err := getOrCreateClient(b, 30, false, "ua", true); err != nil {
			t.Fatalf("getOrCreateClient error: %v", err)
		}
		advancedClientPoolMutex.RLock()
		n := len(advancedClientPool)
		advancedClientPoolMutex.RUnlock()
		if n > maxClientPoolSize {
			t.Fatalf("pool exceeded bound: size %d > %d after %d inserts", n, maxClientPoolSize, i+1)
		}
	}
}

// TestCleanupClientPool_RemovesStale verifies the maxAge cleanup path evicts
// entries idle beyond maxAge while retaining fresh ones.
func TestCleanupClientPool_RemovesStale(t *testing.T) {
	clearAllConnections()
	defer clearAllConnections()

	fresh := Browser{JA3: "fresh", UserAgent: "ua"}
	stale := Browser{JA3: "stale", UserAgent: "ua"}
	if _, err := getOrCreateClient(fresh, 30, false, "ua", true); err != nil {
		t.Fatal(err)
	}
	if _, err := getOrCreateClient(stale, 30, false, "ua", true); err != nil {
		t.Fatal(err)
	}

	staleKey := generateClientKey(stale, 30, false, "")
	advancedClientPoolMutex.Lock()
	advancedClientPool[staleKey].LastUsed = time.Now().Add(-2 * time.Hour)
	advancedClientPoolMutex.Unlock()

	cleanupClientPool(1 * time.Hour)

	advancedClientPoolMutex.RLock()
	_, staleExists := advancedClientPool[staleKey]
	size := len(advancedClientPool)
	advancedClientPoolMutex.RUnlock()

	if staleExists {
		t.Error("cleanupClientPool did not remove the stale entry")
	}
	if size != 1 {
		t.Errorf("expected only the fresh entry to remain, pool size = %d", size)
	}
}
