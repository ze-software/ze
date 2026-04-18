package bridge

import (
	"strings"
	"testing"
)

// TestAckModeDefaultEnabled verifies the default (env unset) is ack enabled.
//
// VALIDATES: AC-16 default case: ze.bgp is backward-compatible with ExaBGP clients
// expecting done/error lines after each dispatched command.
func TestAckModeDefaultEnabled(t *testing.T) {
	t.Setenv(ackEnvKey, "")
	m := newAckMode()
	if !m.enabled {
		t.Errorf("default ack should be enabled, got disabled")
	}
}

// TestAckModeDisabled verifies explicit `false`/`0`/etc disable the ack.
//
// VALIDATES: AC-17 exabgp.api.ack=false silences done/error emission.
func TestAckModeDisabled(t *testing.T) {
	for _, raw := range []string{"false", "0", "no", "off", "disable", "disabled", "FALSE"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv(ackEnvKey, raw)
			m := newAckMode()
			if m.enabled {
				t.Errorf("ack with %q should be disabled, got enabled", raw)
			}
		})
	}
}

// TestAckModeExplicitEnabled verifies explicit truthy values keep ack on.
func TestAckModeExplicitEnabled(t *testing.T) {
	for _, raw := range []string{"true", "1", "yes", "on", "enable"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv(ackEnvKey, raw)
			m := newAckMode()
			if !m.enabled {
				t.Errorf("ack with %q should be enabled", raw)
			}
		})
	}
}

// TestAckWriteAckEmitsDone verifies writeAck emits exactly `done\n`.
//
// VALIDATES: AC-16 emitted frame is a single line, newline-terminated.
func TestAckWriteAckEmitsDone(t *testing.T) {
	var buf strings.Builder
	m := ackMode{enabled: true}
	m.writeAck(&buf)
	if got := buf.String(); got != "done\n" {
		t.Errorf("writeAck = %q, want %q", got, "done\n")
	}
}

// TestAckWriteAckDisabledNoOutput verifies writeAck is a no-op when disabled.
//
// VALIDATES: AC-17 disabled mode writes zero bytes.
func TestAckWriteAckDisabledNoOutput(t *testing.T) {
	var buf strings.Builder
	m := ackMode{enabled: false}
	m.writeAck(&buf)
	if got := buf.String(); got != "" {
		t.Errorf("disabled writeAck wrote %q, want empty", got)
	}
}

// TestAckWriteErrorSanitizes verifies multi-line / CR characters in the error
// text are flattened so they cannot inject extra framing.
//
// VALIDATES: Security review item (AC-18 variant): error message cannot inject
// additional lines.
func TestAckWriteErrorSanitizes(t *testing.T) {
	var buf strings.Builder
	m := ackMode{enabled: true}
	m.writeError(&buf, "bad\r\nerror: oops")
	got := buf.String()
	if !strings.HasPrefix(got, "error ") {
		t.Errorf("writeError output %q should start with 'error '", got)
	}
	if strings.Count(got, "\n") != 1 {
		t.Errorf("writeError output %q should have exactly one newline", got)
	}
}

// TestAckWriteErrorTruncates verifies the error message is bounded in length.
//
// VALIDATES: Security review item: DoS via very long error messages is bounded.
func TestAckWriteErrorTruncates(t *testing.T) {
	var buf strings.Builder
	m := ackMode{enabled: true}
	long := strings.Repeat("x", maxAckMessageLen*2)
	m.writeError(&buf, long)
	if len(buf.String()) > maxAckMessageLen+len("error \n") {
		t.Errorf("writeError did not truncate: %d bytes", len(buf.String()))
	}
}
