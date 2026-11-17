package state

import (
	"context"
	"log"
	"sync"
	"time"
)

const (
	// maxActiveRequests is the maximum number of tracked requests.
	// When exceeded, the oldest entries are evicted to prevent unbounded growth.
	maxActiveRequests = 10000
)

// requestEntry stores a cancel function alongside its registration time.
type requestEntry struct {
	cancel     context.CancelFunc
	registeredAt time.Time
}

// activeRequests maps request IDs to their entries
var activeRequests = make(map[string]requestEntry)

// activeRequestsMutex protects concurrent access to activeRequests
var activeRequestsMutex sync.Mutex

// RequestTrackerMaxSize returns the maximum number of tracked requests.
func RequestTrackerMaxSize() int {
	return maxActiveRequests
}

// RegisterRequest adds a request's cancel function to the tracker.
// This allows the request to be cancelled later by its ID.
// If the tracker is at capacity, stale entries are evicted first.
func RegisterRequest(id string, cancel context.CancelFunc) {
	activeRequestsMutex.Lock()

	// Evict if at capacity
	var evictedCancel context.CancelFunc
	if len(activeRequests) >= maxActiveRequests {
		evictedCancel = evictOldestRequestLocked()
	}

	activeRequests[id] = requestEntry{
		cancel:       cancel,
		registeredAt: time.Now(),
	}

	activeRequestsMutex.Unlock()

	// Call evicted cancel outside the lock
	if evictedCancel != nil {
		evictedCancel()
	}
}

// UnregisterRequest removes a request from the tracker.
// This should be called when a request completes normally.
func UnregisterRequest(id string) {
	activeRequestsMutex.Lock()
	defer activeRequestsMutex.Unlock()
	delete(activeRequests, id)
}

// CancelRequest cancels and removes a request from the tracker.
// Returns true if the request was found and cancelled, false otherwise.
// The cancel function is called after releasing the mutex to prevent deadlock
// if the cancel callback tries to interact with the tracker.
func CancelRequest(id string) bool {
	activeRequestsMutex.Lock()
	entry, exists := activeRequests[id]
	if exists {
		delete(activeRequests, id)
	}
	activeRequestsMutex.Unlock()

	if exists {
		entry.cancel()
		return true
	}
	return false
}

// CleanupStaleRequests removes entries older than maxAge and cancels them.
// Returns the number of entries removed.
// Cancel functions are called after releasing the mutex to prevent deadlock.
func CleanupStaleRequests(maxAge time.Duration) int {
	activeRequestsMutex.Lock()

	cutoff := time.Now().Add(-maxAge)
	var toCancel []context.CancelFunc

	for id, entry := range activeRequests {
		if entry.registeredAt.Before(cutoff) {
			toCancel = append(toCancel, entry.cancel)
			delete(activeRequests, id)
		}
	}

	activeRequestsMutex.Unlock()

	// Call cancel functions outside the lock
	for _, cancel := range toCancel {
		cancel()
	}

	if len(toCancel) > 0 {
		log.Printf("state: cleaned up %d stale requests (older than %v)", len(toCancel), maxAge)
	}

	return len(toCancel)
}

// recordRequestTime sets the registration time for a request (used in tests).
func recordRequestTime(id string, t time.Time) {
	activeRequestsMutex.Lock()
	defer activeRequestsMutex.Unlock()
	if entry, exists := activeRequests[id]; exists {
		entry.registeredAt = t
		activeRequests[id] = entry
	}
}

// evictOldestRequestLocked removes the oldest entry from the map and returns
// its cancel function. Caller must hold activeRequestsMutex.
// The caller should invoke the returned cancel function after releasing the mutex.
func evictOldestRequestLocked() context.CancelFunc {
	var oldestID string
	var oldestTime time.Time
	first := true

	for id, entry := range activeRequests {
		if first || entry.registeredAt.Before(oldestTime) {
			oldestID = id
			oldestTime = entry.registeredAt
			first = false
		}
	}

	if !first {
		if entry, exists := activeRequests[oldestID]; exists {
			delete(activeRequests, oldestID)
			return entry.cancel
		}
	}
	return nil
}
