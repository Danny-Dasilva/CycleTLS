// Package state provides thread-safe state management for CycleTLS.
// This file handles WebSocket connection tracking.
package state

import (
	"log"
	"sync"
	"time"
)

const (
	// maxActiveWebSockets is the maximum number of tracked WebSocket connections.
	// When exceeded, the oldest entries are evicted to prevent unbounded growth.
	maxActiveWebSockets = 10000
)

// wsEntry stores a WebSocket connection alongside its registration time.
type wsEntry struct {
	conn         interface{}
	registeredAt time.Time
}

// activeWebSockets stores all active WebSocket connections by their ID.
// Uses interface{} to avoid circular imports - callers type-assert to *WebSocketConnection.
var activeWebSockets = make(map[string]wsEntry)

// activeWebSocketsMutex protects concurrent access to activeWebSockets.
// Uses RWMutex for read-heavy workload optimization.
var activeWebSocketsMutex sync.RWMutex

// WebSocketTrackerMaxSize returns the maximum number of tracked WebSocket connections.
func WebSocketTrackerMaxSize() int {
	return maxActiveWebSockets
}

// RegisterWebSocket adds a WebSocket connection to the active connections map.
// The connection should be a *WebSocketConnection from the cycletls package.
// If the tracker is at capacity, the oldest entry is evicted first.
func RegisterWebSocket(id string, conn interface{}) {
	activeWebSocketsMutex.Lock()
	defer activeWebSocketsMutex.Unlock()

	// Evict if at capacity
	if len(activeWebSockets) >= maxActiveWebSockets {
		evictOldestWebSocketLocked()
	}

	activeWebSockets[id] = wsEntry{
		conn:         conn,
		registeredAt: time.Now(),
	}
}

// GetWebSocket retrieves a WebSocket connection by its ID.
// Returns the connection and true if found, nil and false otherwise.
// Callers should type-assert the returned interface{} to *WebSocketConnection.
func GetWebSocket(id string) (interface{}, bool) {
	activeWebSocketsMutex.RLock()
	defer activeWebSocketsMutex.RUnlock()
	entry, exists := activeWebSockets[id]
	if !exists {
		return nil, false
	}
	return entry.conn, true
}

// UnregisterWebSocket removes a WebSocket connection from the active connections map.
func UnregisterWebSocket(id string) {
	activeWebSocketsMutex.Lock()
	defer activeWebSocketsMutex.Unlock()
	delete(activeWebSockets, id)
}

// CleanupStaleWebSockets removes entries older than maxAge.
// Returns the number of entries removed.
func CleanupStaleWebSockets(maxAge time.Duration) int {
	activeWebSocketsMutex.Lock()
	defer activeWebSocketsMutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, entry := range activeWebSockets {
		if entry.registeredAt.Before(cutoff) {
			delete(activeWebSockets, id)
			removed++
		}
	}

	if removed > 0 {
		log.Printf("state: cleaned up %d stale WebSocket entries (older than %v)", removed, maxAge)
	}

	return removed
}

// recordWebSocketTime sets the registration time for a WebSocket (used in tests).
func recordWebSocketTime(id string, t time.Time) {
	activeWebSocketsMutex.Lock()
	defer activeWebSocketsMutex.Unlock()
	if entry, exists := activeWebSockets[id]; exists {
		entry.registeredAt = t
		activeWebSockets[id] = entry
	}
}

// evictOldestWebSocketLocked removes the oldest entry from the map.
// Caller must hold activeWebSocketsMutex.
func evictOldestWebSocketLocked() {
	var oldestID string
	var oldestTime time.Time
	first := true

	for id, entry := range activeWebSockets {
		if first || entry.registeredAt.Before(oldestTime) {
			oldestID = id
			oldestTime = entry.registeredAt
			first = false
		}
	}

	if !first {
		delete(activeWebSockets, oldestID)
	}
}
