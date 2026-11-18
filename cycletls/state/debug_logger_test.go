package state

import (
	"io"
	"os"
	"testing"
)

// The debug logger must stay quiet unless CYCLETLS_DEBUG is set: a parent
// process that does not read stdout would otherwise stall the server once the
// pipe is full.
func TestDebugOutputFollowsEnv(t *testing.T) {
	cases := []struct {
		value string
		want  io.Writer
	}{
		{"", io.Discard},
		{"0", io.Discard},
		{"false", io.Discard},
		{"1", os.Stdout},
		{"true", os.Stdout},
	}
	for _, tc := range cases {
		t.Setenv(DebugEnvVar, tc.value)
		if got := debugOutput(); got != tc.want {
			t.Errorf("%s=%q: debugOutput() = %v, want %v", DebugEnvVar, tc.value, got, tc.want)
		}
	}
}

func TestDebugLoggerSilentByDefault(t *testing.T) {
	if DebugEnabled() {
		t.Skipf("%s is set in this environment", DebugEnvVar)
	}
	if DebugLogger.Writer() != io.Discard {
		t.Fatalf("DebugLogger writes to %v with %s unset, want io.Discard", DebugLogger.Writer(), DebugEnvVar)
	}
}
