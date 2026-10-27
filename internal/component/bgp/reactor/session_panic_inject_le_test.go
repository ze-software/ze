//go:build ze_le

package reactor

import (
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// VALIDATES: the ze_le read-panic seam fires once per trigger file, for the
// named peer only, and removes the file before it panics.
// PREVENTS: an interop scenario (bgp-panic-cease) whose trigger never fires, or
// fires on every UPDATE after the first, which would turn one contained panic
// into a reconnect loop the scenario could not tell from a defect.
func TestInjectReadPanicFiresOncePerTrigger(t *testing.T) {
	directory := t.TempDir()
	saved := readPanicTriggerDir
	readPanicTriggerDir = directory
	t.Cleanup(func() { readPanicTriggerDir = saved })

	target := netip.MustParseAddr("192.0.2.1")
	other := netip.MustParseAddr("192.0.2.2")
	trigger := filepath.Join(directory, readPanicTriggerPrefix+target.String())

	if panicked(func() { injectReadPanic(target) }) {
		t.Fatal("no trigger file exists, yet the seam panicked")
	}
	if err := os.WriteFile(trigger, nil, 0o600); err != nil {
		t.Fatalf("write the trigger file: %v", err)
	}
	if panicked(func() { injectReadPanic(other) }) {
		t.Fatalf("the trigger names %s, yet the seam panicked for %s", target, other)
	}
	if !panicked(func() { injectReadPanic(target) }) {
		t.Fatalf("the trigger file %s exists, yet the seam did not panic", trigger)
	}
	if _, err := os.Stat(trigger); !os.IsNotExist(err) {
		t.Fatalf("the trigger file must be removed before the panic, stat answered %v", err)
	}
	if panicked(func() { injectReadPanic(target) }) {
		t.Fatal("one trigger file raised a second panic")
	}
}

// VALIDATES: both readers reach the seam for a plain IPv4 announcement, the
// UPDATE an interop peer sends when it redistributes one route.
// PREVENTS: the seam sitting on processMessage alone, which the coalescing
// reader bypasses for such an UPDATE: bgp-panic-cease then armed a trigger
// that no UPDATE from FRR ever consumed, and no Cease went out.
// METHOD: arm the trigger for the session's peer, feed one announcement
// through each reader, and require the read to panic and consume the file.
func TestInjectReadPanicReachedByBothReaders(t *testing.T) {
	for _, coalesced := range []bool{false, true} {
		t.Run("coalesced="+strconv.FormatBool(coalesced), func(t *testing.T) {
			directory := t.TempDir()
			saved := readPanicTriggerDir
			readPanicTriggerDir = directory
			t.Cleanup(func() { readPanicTriggerDir = saved })

			session, client, _, cleanup := setupCapturingSession(t, 65002, false, false)
			defer cleanup()
			trigger := filepath.Join(directory, readPanicTriggerPrefix+session.settings.Address.String())
			if err := os.WriteFile(trigger, nil, 0o600); err != nil {
				t.Fatalf("write the trigger file: %v", err)
			}

			frame := buildUpdateMsg(receivedUpdateBody(fatalLengthAnnouncement().PathAttributes, ipv4Slash24s(1)))
			go func() {
				_, _ = client.Write(frame) //nolint:errcheck // cleanup closes the pipe; the read below is the assertion
			}()
			raised := panicked(func() {
				if coalesced {
					_ = session.readAndProcessCoalesced(session.Conn(), session.bufReader) //nolint:errcheck // only the panic is judged
					return
				}
				_ = session.ReadAndProcess() //nolint:errcheck // only the panic is judged
			})
			if !raised {
				t.Fatalf("an UPDATE from %s with %s armed did not reach the seam", session.settings.Address, trigger)
			}
			if _, err := os.Stat(trigger); !os.IsNotExist(err) {
				t.Fatalf("the trigger file must be consumed by the panic, stat answered %v", err)
			}
		})
	}
}

// panicked answers whether fn panicked.
func panicked(fn func()) (raised bool) {
	defer func() {
		if recover() != nil {
			raised = true
		}
	}()
	fn()
	return false
}
