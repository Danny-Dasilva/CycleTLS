package cycletls

import (
	"sync"
	"time"

	utls "github.com/refraction-networking/utls"
)

const (
	// sessionCacheHosts is how many server names one session cache remembers.
	sessionCacheHosts = 64
	// maxSessionCaches bounds the registry below, like maxClientPoolSize
	// bounds the client pool.
	maxSessionCaches = maxClientPoolSize
)

// newSessionCache returns an LRU TLS session cache. utls keys entries by the
// SNI it sends (or by the remote address when there is none).
func newSessionCache() utls.ClientSessionCache {
	return utls.NewLRUClientSessionCache(sessionCacheHosts)
}

type sessionCacheEntry struct {
	cache    utls.ClientSessionCache
	lastUsed time.Time
}

// sessionCaches holds one TLS session cache per client configuration.
//
// The cache cannot live on the roundTripper alone: with connection reuse off,
// createNewClient builds a new roundTripper for every request, so a
// per-roundTripper cache would always be empty and nothing would ever resume.
// Keeping it on the pooled client would not help either, because that pool is
// skipped in the same case. So the caches are kept here, keyed by the
// configuration that decides what a TLS session may be shared with (see
// sessionCacheKey), and bounded with least-recently-used eviction.
var sessionCaches = struct {
	sync.Mutex
	byKey map[string]*sessionCacheEntry
}{byKey: make(map[string]*sessionCacheEntry)}

// sharedSessionCache returns the session cache for a configuration key,
// creating it on first use.
func sharedSessionCache(key string) utls.ClientSessionCache {
	sessionCaches.Lock()
	defer sessionCaches.Unlock()

	now := time.Now()
	if entry, ok := sessionCaches.byKey[key]; ok {
		entry.lastUsed = now
		return entry.cache
	}

	for len(sessionCaches.byKey) >= maxSessionCaches {
		var oldestKey string
		var oldest time.Time
		first := true
		for k, entry := range sessionCaches.byKey {
			if first || entry.lastUsed.Before(oldest) {
				oldestKey, oldest, first = k, entry.lastUsed, false
			}
		}
		delete(sessionCaches.byKey, oldestKey)
	}

	entry := &sessionCacheEntry{cache: newSessionCache(), lastUsed: now}
	sessionCaches.byKey[key] = entry
	return entry.cache
}

// clearSessionCaches drops every cached TLS session (test isolation).
func clearSessionCaches() {
	sessionCaches.Lock()
	defer sessionCaches.Unlock()
	sessionCaches.byKey = make(map[string]*sessionCacheEntry)
}

// sessionCacheKey names the session cache a client configuration uses.
//
// Two requests share TLS sessions only when everything that shapes the
// handshake or the route is the same: fingerprint, user agent, SNI, proxy,
// verification settings, pins and CAs. A session is never offered through a
// different proxy or under a different fingerprint, which would link the two.
// It reuses generateClientKey so that a field added there is covered here too,
// with the parts that do not touch TLS (cookies, header order, HTTP/2
// fingerprint, timeout, redirects) blanked so they do not split the cache.
func sessionCacheKey(browser Browser, proxyURL ...string) string {
	proxy := ""
	if len(proxyURL) > 0 {
		proxy = proxyURL[0]
	}
	browser.Cookies = nil
	browser.HeaderOrder = nil
	browser.HTTP2Fingerprint = ""
	return generateClientKey(browser, 0, false, proxy)
}
