// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- receive-path validation
// Related: instance.go -- onPacket and noteOwnerConflict
package vrrp

import (
	"bytes"
	"log/slog"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
	"github.com/ze-software/ze/internal/plugins/vrrp/transport"
)

// ownerConflictLogLine is the text noteOwnerConflict logs, matched by the
// tests below to count log lines.
const ownerConflictLogLine = "advertisement received by the address owner"

// v3AdvertItem encodes a VRRPv3 IPv4 advertisement for VRID 10 from source
// and wraps it as a received datagram.
func v3AdvertItem(t *testing.T, priority uint8, source netip.Addr, vips ...netip.Addr) transport.RxItem {
	t.Helper()
	adv := packet.Advertisement{
		Version:         packet.VersionV3,
		Family:          packet.V4,
		VRID:            10,
		Priority:        priority,
		AdverIntervalMS: 1000,
		VIPs:            vips,
	}
	var buf [packet.MaxLenV3v4]byte
	n := adv.WriteTo(buf[:], 0)
	packet.FillChecksum(buf[:], 0, n, source, packet.MulticastV4)
	return transport.RxItem{
		Meta:    packet.RxMeta{TTL: 255, Family: packet.V4, Src: source, Dst: packet.MulticastV4},
		Payload: append([]byte(nil), buf[:n]...),
	}
}

// captureVRRPLog routes the package logger into a buffer for one test and
// restores the previous logger after it. The package tests run serially, so
// no other test writes to the buffer.
func captureVRRPLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logBuf bytes.Buffer
	previous := logger()
	setLogger(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { setLogger(previous) })
	return &logBuf
}

// TestInstanceV3OwnerConflictLoggedAndProcessed proves the VRRPv3 address
// owner checks every advertisement it receives, counts and logs the conflict
// under a rate limit, and still processes the advertisement.
//
// Method: a v3 owner receives Priority 255 advertisements from a greater
// address. Each is counted under the owner-conflict reason and reaches the
// FSM. The first logs; the second, 30 s later, does not; the third, past the
// one-minute window, logs again. A v3 non-owner receiving the same
// advertisement records nothing and logs nothing.
func TestInstanceV3OwnerConflictLoggedAndProcessed(t *testing.T) {
	// RFC requirement: RFC9568-7.1-12 positive -- a VRRPv3 advertisement received by the address owner (local Priority 255) is detected: counted under the owner-conflict reason every time, logged at most once a minute, and still delivered to the FSM (noteOwnerConflict instance.go).
	// RFC requirement: RFC9568-7.1-12 negative -- the same advertisement received by a v3 router that is not the address owner raises no owner-conflict count and no log line (onPacket instance.go).
	logBuf := captureVRRPLog(t)
	vip := netip.MustParseAddr("192.0.2.1")
	greater := netip.MustParseAddr("192.0.2.200")

	spec := testSpec()
	spec.IsOwner = true
	spec.Priority = ownerPriority
	owner, ownerDeps, clk := newTestInstance(t, spec)
	owner.dispatch(fsm.Startup{Config: owner.fsmConfig()})

	for i, step := range []struct {
		advance time.Duration
		logs    int
	}{{0, 1}, {30 * time.Second, 1}, {31 * time.Second, 2}} {
		clk.Add(step.advance)
		owner.onPacket(v3AdvertItem(t, ownerPriority, greater, vip))
		got := ownerDeps.snapshot()
		if len(got.rxErrors) != i+1 || got.rxErrors[i] != packet.ReasonOwnerConflict {
			t.Fatalf("advert %d: the v3 owner must count %q for every advert, got %+v", i+1, packet.ReasonOwnerConflict, got.rxErrors)
		}
		if lines := strings.Count(logBuf.String(), ownerConflictLogLine); lines != step.logs {
			t.Fatalf("advert %d: owner-conflict log lines = %d, want %d (rate limit)\n%s", i+1, lines, step.logs, logBuf.String())
		}
		if !drainAdvertReceived(owner) {
			t.Fatalf("advert %d: the v3 owner must still process the advert (erratum 8298), but the FSM saw no AdvertReceived", i+1)
		}
	}

	logBuf.Reset()
	spec.IsOwner = false
	spec.Priority = 200
	backup, backupDeps, _ := newTestInstance(t, spec)
	backup.dispatch(fsm.Startup{Config: backup.fsmConfig()})
	backup.onPacket(v3AdvertItem(t, ownerPriority, greater, vip))
	if got := backupDeps.snapshot(); len(got.rxErrors) != 0 {
		t.Fatalf("a v3 non-owner must not flag an owner conflict, got %+v", got.rxErrors)
	}
	if strings.Contains(logBuf.String(), ownerConflictLogLine) {
		t.Fatalf("a v3 non-owner must not log an owner conflict:\n%s", logBuf.String())
	}
	if !drainAdvertReceived(backup) {
		t.Fatal("a v3 non-owner delivered no AdvertReceived to the FSM")
	}
}

// drainAdvertReceived empties the instance's event queue and answers whether
// an AdvertReceived was among the events. Advancing the fake clock fires the
// Master's advertisement timer, so the queue can also hold its expiry.
func drainAdvertReceived(in *instance) bool {
	found := false
	for {
		select {
		case ev := <-in.events:
			if _, ok := ev.(fsm.AdvertReceived); ok {
				found = true
			}
		default:
			return found
		}
	}
}
