//go:build docker

// Design: docs/architecture/testing/interop.md -- a package test's own server container (labtest)
// Related: ntp.go -- syncWorker.doSync, the client code these tests drive.

package ntp

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/beevik/ntp"

	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/interoplab/labtest"
)

const (
	// chronyImage is the real NTP server, pinned to one chrony release.
	chronyImage = "dockurr/chrony:4.9@sha256:f584829f268b26b26b351000b60aa7c3507933873b6a6d77ffe9dcabcfdc7025"
	// chronyLocalStratum is the stratum the synchronised server is configured
	// to serve (`local stratum 7`). Seven is neither chrony's default local
	// stratum (10) nor a value Ze could produce on its own.
	chronyLocalStratum = 7
	// chronyConfiguration is where the start script writes the test's own
	// chrony configuration; the image's entry point would otherwise write one
	// from its environment and ignore the command.
	chronyConfiguration = "/etc/chrony/chrony.conf"
	// chronyStartScript writes its first argument as the configuration, then
	// runs chronyd in the foreground (-d), never touching the container's
	// clock (-x), as root so it needs no chrony-owned run directory.
	chronyStartScript = `printf '%s\n' "$0" > ` + chronyConfiguration +
		` && exec chronyd -d -x -u root -f ` + chronyConfiguration
	// chronyReadyTimeout bounds the wait for the first answer, which includes
	// chrony switching to its local reference.
	chronyReadyTimeout = 60 * time.Second
	// chronyLogLines is the log tail a failed test receives.
	chronyLogLines = 200
	// probeTimeout bounds one direct NTP query the tests make themselves.
	probeTimeout = 2 * time.Second
	// offsetMax bounds the offset Ze may report: the container reads the
	// host's own kernel clock, so any real offset is query noise.
	offsetMax = 500 * time.Millisecond
	// rttMax bounds the round trip Ze may report over the host loopback.
	rttMax = 2 * time.Second
)

// chronySynchronised serves its local clock at chronyLocalStratum to any
// client, and opens no command port.
const chronySynchronised = "local stratum 7\nallow all\ncmdport 0"

// chronyUnsynchronised has one source that never answers (192.0.2.1 is
// TEST-NET-1) and no local reference, so chrony answers every client as
// unsynchronised: leap indicator 3 (alarm) and stratum 0 on the wire, which is
// how RFC 5905 Section 11.1 transmits MAXSTRAT (16): "Remember that MAXSTRAT
// is mapped to zero in the transmitted packet."
const chronyUnsynchronised = "server 192.0.2.1\nallow all\ncmdport 0"

// startChrony starts chrony with configuration and answers the
// `host:port` Ze's client is configured with, once ready accepts a reply.
func startChrony(t *testing.T, name, configuration string,
	ready func(context.Context, netip.AddrPort) error,
) string {
	t.Helper()
	address := labtest.Start(t, labtest.Spec{
		Name: name, Image: chronyImage, Entrypoint: "sh",
		Command:      []string{"-c", chronyStartScript, configuration},
		Port:         interoplab.UDPPort(123),
		Ready:        ready,
		ReadyTimeout: chronyReadyTimeout,
		LogLines:     chronyLogLines,
	})
	return address.String()
}

// queryDirect asks the server itself, outside Ze, so a test can state what
// the server served before it judges what Ze made of it.
func queryDirect(address string) (*ntp.Response, error) {
	return ntp.QueryWithOptions(address, ntp.QueryOptions{Timeout: probeTimeout})
}

// synchronisedReply is ready once the server answers as synchronised: chrony
// answers unsynchronised until its local reference takes over.
func synchronisedReply(_ context.Context, address netip.AddrPort) error {
	response, err := queryDirect(address.String())
	if err != nil {
		return err
	}
	if response.Leap == ntp.LeapNotInSync {
		return errors.New("chrony still answers unsynchronised")
	}
	return nil
}

// anyReply is ready once the server answers at all.
func anyReply(_ context.Context, address netip.AddrPort) error {
	_, err := queryDirect(address.String())
	return err
}

// syncFrom runs one sync of Ze's real client against address, with the real
// query (ntpQueryFn left at ntp.Query) and the clock setter replaced by a
// recorder, because a test MUST NOT step the host clock. It answers what
// doSync returned, the clock times Ze asked for, and the published state.
func syncFrom(t *testing.T, address string) (bool, []time.Time, *syncState) {
	t.Helper()
	restoreNTPSeams(t)
	var stepped []time.Time
	setClockFn = func(at time.Time) error {
		stepped = append(stepped, at)
		return nil
	}
	// SlewThresholdMs 0 steps through setClockFn rather than adjtimex, and an
	// empty PersistPath writes no state store.
	worker := newSyncWorker(ntpConfig{Enabled: true, Servers: []string{address}, IntervalSec: 3600}, nil)
	synced := worker.doSync(slog.New(slog.NewTextHandler(t.Output(), nil)))
	return synced, stepped, loadState()
}

// serverNamed answers the per-server state Ze published for address.
func serverNamed(t *testing.T, state *syncState, address string) serverState {
	t.Helper()
	if state == nil {
		t.Fatal("doSync published no state")
	}
	for _, server := range state.Servers {
		if server.Address == address {
			return server
		}
	}
	t.Fatalf("published state %+v holds no server %s", state.Servers, address)
	return serverState{}
}

// VALIDATES: AC-20, S-2: Ze's NTP client, querying a real chrony over UDP,
// accepts a synchronised reply and reports what the server serves: the
// configured stratum, an offset and round trip within the bounds a
// same-kernel clock allows, reach bit 0 set, the server as its source, and
// one clock step to that offset.
// PREVENTS: a client that never reached a real server (only the in-process
// fake of ntp_test.go), or one that decodes the reply but reports the wrong
// fields.
// Method: labtest starts chrony; the test confirms the reply chrony gives,
// then runs syncWorker.doSync with the real query and reads the published
// state. Not parallel: it swaps the package's clock seam.
func TestNTPClientSyncsFromChrony(t *testing.T) {
	address := startChrony(t, "chrony-synced", chronySynchronised, synchronisedReply)

	served, err := queryDirect(address)
	if err != nil {
		t.Fatalf("direct query of %s: %v", address, err)
	}
	if served.Stratum != chronyLocalStratum {
		t.Fatalf("premise: chrony serves stratum %d, configured %d", served.Stratum, chronyLocalStratum)
	}
	if served.Leap != ntp.LeapNoWarning {
		t.Fatalf("premise: chrony serves leap indicator %d, want %d (no warning)", served.Leap, ntp.LeapNoWarning)
	}

	before := time.Now()
	synced, stepped, state := syncFrom(t, address)
	after := time.Now()
	if !synced {
		t.Fatalf("doSync did not sync from %s; state %+v", address, state)
	}
	if !state.Synced {
		t.Error("published state is not synced")
	}
	if state.Source != address {
		t.Errorf("source = %q, want %q", state.Source, address)
	}
	if state.Stratum != chronyLocalStratum {
		t.Errorf("published stratum = %d, want %d", state.Stratum, chronyLocalStratum)
	}

	server := serverNamed(t, state, address)
	if server.LastError != "" {
		t.Errorf("last error = %q, want none", server.LastError)
	}
	if server.Reach != 1 {
		t.Errorf("reach = %08b, want 00000001 after one answered query", server.Reach)
	}
	if server.Stratum != chronyLocalStratum {
		t.Errorf("server stratum = %d, want %d", server.Stratum, chronyLocalStratum)
	}
	if server.Offset != state.Offset {
		t.Errorf("published offset %v differs from the server's %v", state.Offset, server.Offset)
	}
	if server.Offset < -offsetMax || server.Offset > offsetMax {
		t.Errorf("offset = %v, outside +-%v for a server on the same kernel clock", server.Offset, offsetMax)
	}
	if server.RTT <= 0 || server.RTT > rttMax {
		t.Errorf("rtt = %v, want in (0, %v]", server.RTT, rttMax)
	}
	if server.RootDelay != 0 {
		t.Errorf("root delay = %v, want 0 from a server on its local reference", server.RootDelay)
	}

	if len(stepped) != 1 {
		t.Fatalf("clock steps = %v, want exactly one", stepped)
	}
	if stepped[0].Before(before.Add(-offsetMax)) || stepped[0].After(after.Add(offsetMax)) {
		t.Errorf("clock stepped to %v, outside [%v, %v] widened by %v", stepped[0], before, after, offsetMax)
	}
}

// VALIDATES: AC-20, S-2: Ze's NTP client, querying a real chrony that has no
// synchronised source, rejects the reply (stratum 16, leap alarm) rather than
// accepting it: no clock step, not synced, no source, reach bit 0 clear, and
// a reason in the server's last error.
// PREVENTS: a client that sets the router's clock from a server that says
// it does not know the time.
// Method: as TestNTPClientSyncsFromChrony, against a chrony whose only
// source never answers.
func TestNTPClientRejectsUnsynchronisedChrony(t *testing.T) {
	address := startChrony(t, "chrony-unsynced", chronyUnsynchronised, anyReply)

	served, err := queryDirect(address)
	if err != nil {
		t.Fatalf("direct query of %s: %v", address, err)
	}
	if served.Stratum != 0 {
		t.Fatalf("premise: unsynchronised chrony serves stratum %d, want 0 (MAXSTRAT on the wire)", served.Stratum)
	}
	if served.Leap != ntp.LeapNotInSync {
		t.Fatalf("premise: unsynchronised chrony serves leap indicator %d, want %d (alarm)",
			served.Leap, ntp.LeapNotInSync)
	}

	synced, stepped, state := syncFrom(t, address)
	if synced {
		t.Error("doSync synced from an unsynchronised server")
	}
	if len(stepped) != 0 {
		t.Errorf("clock steps = %v, want none from an unsynchronised server", stepped)
	}
	if state.Synced {
		t.Error("published state is synced")
	}
	if state.Source != "" {
		t.Errorf("source = %q, want none", state.Source)
	}

	server := serverNamed(t, state, address)
	if server.Reach != 0 {
		t.Errorf("reach = %08b, want 00000000: the reply was rejected", server.Reach)
	}
	if server.LastError == "" {
		t.Error("last error is empty: the operator is not told why the server was rejected")
	}
	t.Logf("reference id %08x; Ze reports %q", served.ReferenceID, server.LastError)
}
