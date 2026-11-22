package hub

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestLogDaemonCapabilitiesNeverWarns logs each inventory the daemon can take of
// its capabilities at start: a set it lacks part of, a set it cannot read, and a
// set it holds whole.
//
// VALIDATES: the generic inventory never reaches the hub log at warn or error,
// whatever it holds, so a config that uses nothing a missing capability serves
// starts with no warning and no advice to reinstall the systemd unit.
// PREVENTS: the start-time warning that told an interfaces-and-SSH daemon,
// running correctly with CAP_NET_ADMIN alone, to reinstall its unit.
// METHOD: read the global log ring back for the hub component after each call
// and refuse any warn or error entry this test produced.
func TestLogDaemonCapabilitiesNeverWarns(t *testing.T) {
	started := time.Now()
	bpf := privilege.Capability{Name: "CAP_BPF", Bit: 39, Use: "eBPF"}
	logDaemonCapabilities([]privilege.Capability{bpf}, nil)
	logDaemonCapabilities(nil, errors.New("CapEff not found in /proc/self/status"))
	logDaemonCapabilities(nil, nil)

	entries, err := slogutil.GlobalLogRing().Snapshot(0, "", "hub")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Timestamp.Before(started) {
			continue
		}
		if !strings.Contains(entry.Message, "capabilit") {
			continue
		}
		if entry.Level == "warn" || entry.Level == "error" {
			t.Errorf("the capability inventory logged at %s: %q", entry.Level, entry.Message)
		}
	}
}
