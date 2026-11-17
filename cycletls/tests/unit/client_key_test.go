//go:build !integration

package unit

// The authoritative tests for generateClientKey and the client pool are
// white-box tests in package cycletls (client_key_internal_test.go and
// client_pool_internal_test.go), because generateClientKey, getOrCreateClient
// and the pool are unexported and cannot be reached from this black-box package.
// This file keeps a self-contained sanity check of the FNV-1a canonical-key
// approach those tests exercise.

import (
	"fmt"
	"hash/fnv"
	"testing"
)

func fnvKey(configStr string) string {
	h := fnv.New64a()
	h.Write([]byte(configStr))
	return fmt.Sprintf("%016x", h.Sum64())
}

// TestFNVKey_DeterministicAndDistinct verifies the properties generateClientKey
// relies on: identical config strings hash to the same 16-char hex key, and a
// config differing in any field (here, header order) hashes to a different key.
func TestFNVKey_DeterministicAndDistinct(t *testing.T) {
	a := "ja3:771|ua:Chrome|headerorder:host,user-agent"
	b := "ja3:771|ua:Chrome|headerorder:user-agent,host" // differs only in header order

	if fnvKey(a) != fnvKey(a) {
		t.Error("FNV-1a key is not deterministic")
	}
	if fnvKey(a) == fnvKey(b) {
		t.Error("configs differing only in header order must produce distinct keys")
	}
	if got := fnvKey(a); len(got) != 16 {
		t.Errorf("expected 16-char hex key, got %q (%d chars)", got, len(got))
	}
}
