//go:build !integration

package unit

import (
	"testing"
)

// Issue #4 (CRITICAL): Fixed panic test logic error.
// Previously, t.Error("Should have panicked") at the end was unreachable because the panic
// on the line above would always jump to the defer/recover. The test always passed vacuously.
// Fix: Use a `panicked` boolean flag set inside defer/recover, and assert it after the
// function that should panic returns.
func TestUncheckedTypeAssertion_Panics(t *testing.T) {
	t.Parallel()

	// Simulate the state.GetWebSocket pattern: returns interface{}
	var wsConnInterface interface{} = "not a WebSocketConnection"

	type FakeWSConn struct{ ID string }

	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
				t.Logf("Unchecked type assertion panicked as expected: %v", r)
			}
		}()
		// This simulates the bug - unchecked assertion on wrong type panics
		_ = wsConnInterface.(*FakeWSConn)
	}()

	if !panicked {
		t.Error("Expected unchecked type assertion to panic, but it did not")
	}
}

// TestCheckedTypeAssertion_Safe demonstrates the fix: comma-ok pattern
// gracefully handles wrong types without panicking.
func TestCheckedTypeAssertion_Safe(t *testing.T) {
	t.Parallel()

	type FakeWSConn struct{ ID string }

	// Case 1: wrong type
	var wsConnInterface interface{} = "not a connection"
	conn, ok := wsConnInterface.(*FakeWSConn)
	if ok {
		t.Error("Should return ok=false for wrong type")
	}
	if conn != nil {
		t.Error("Should return nil for wrong type")
	}

	// Case 2: correct type
	expected := &FakeWSConn{ID: "test-123"}
	wsConnInterface = expected
	conn, ok = wsConnInterface.(*FakeWSConn)
	if !ok {
		t.Error("Should return ok=true for correct type")
	}
	if conn != expected {
		t.Error("Should return the same pointer")
	}
	if conn.ID != "test-123" {
		t.Errorf("ID mismatch: got %s, expected test-123", conn.ID)
	}

	// Case 3: nil interface
	wsConnInterface = nil
	conn, ok = wsConnInterface.(*FakeWSConn)
	if ok {
		t.Error("Should return ok=false for nil interface")
	}
	if conn != nil {
		t.Error("Should return nil for nil interface")
	}
}

// TestTypeAssertionOnNilPointer tests comma-ok with a typed nil pointer in interface.
func TestTypeAssertionOnNilPointer(t *testing.T) {
	t.Parallel()

	type FakeWSConn struct{ ID string }

	// A typed nil pointer IS a valid *FakeWSConn, so ok should be true
	var nilConn *FakeWSConn
	var wsConnInterface interface{} = nilConn

	conn, ok := wsConnInterface.(*FakeWSConn)
	if !ok {
		t.Error("Typed nil pointer should pass type assertion (ok=true)")
	}
	if conn != nil {
		t.Error("Typed nil pointer should yield nil conn value")
	}
}

// TestTypeAssertionMultipleTypes tests comma-ok across several concrete types.
func TestTypeAssertionMultipleTypes(t *testing.T) {
	t.Parallel()

	type ConnA struct{ Name string }
	type ConnB struct{ Name string }

	var iface interface{} = &ConnA{Name: "alpha"}

	if _, ok := iface.(*ConnA); !ok {
		t.Error("Should match *ConnA")
	}
	if _, ok := iface.(*ConnB); ok {
		t.Error("Should NOT match *ConnB")
	}
	if _, ok := iface.(string); ok {
		t.Error("Should NOT match string")
	}
	if _, ok := iface.(int); ok {
		t.Error("Should NOT match int")
	}
}
