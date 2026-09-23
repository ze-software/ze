package l2tpauthradius

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/radius"
)

// Backend registration and display baselines have no unregister API. Keep them
// in a joined child, leaving the parent's backend and acctGetStats untouched.
func isolatedAcctCounters(t *testing.T) bool {
	t.Helper()
	const key = "ZE_RADIUS_ACCT_COUNTER_TEST"
	if os.Getenv(key) == t.Name() {
		return true
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+regexp.QuoteMeta(t.Name())+"$", "-test.count=1", "-test.timeout=25s")
	cmd.Env = append(os.Environ(), key+"="+t.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("isolated accounting counters: %v\n%s", err, output)
	}
	return false
}

// Like the Linux backend, GetStats returns raw totals and ResetCounters cannot
// physically clear them. Real iface dispatch owns the display-only baseline.
// Embedding Backend makes an unexpected operation fail rather than fake success.
type acctCounterBackend struct {
	iface.Backend
	mu    sync.Mutex
	stats iface.InterfaceStats
	err   error
}

func (b *acctCounterBackend) set(stats iface.InterfaceStats, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stats, b.err = stats, err
}

func (b *acctCounterBackend) GetStats(name string) (*iface.InterfaceStats, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if name != "ppp4242" {
		return nil, fmt.Errorf("unknown counter interface %q", name)
	}
	if b.err != nil {
		return nil, b.err
	}
	stats := b.stats // dispatch may subtract its baseline in place
	return &stats, nil
}

func (b *acctCounterBackend) GetInterface(name string) (*iface.InterfaceInfo, error) {
	if name != "ppp4242" {
		return nil, fmt.Errorf("unknown counter interface %q", name)
	}
	return &iface.InterfaceInfo{Name: name, OsName: name, Index: 4242}, nil
}

func (b *acctCounterBackend) ResetCounters(string) error { return iface.ErrCountersNotResettable }
func (b *acctCounterBackend) Close() error               { return nil }

func installAcctCounterBackend(t *testing.T) *acctCounterBackend {
	t.Helper()
	b := &acctCounterBackend{}
	const name = "radius-acct-counter-test"
	if err := iface.RegisterBackend(name, func() (iface.Backend, error) { return b, nil }); err != nil {
		t.Fatal(err)
	}
	if err := iface.LoadBackend(name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := iface.CloseBackend(); err != nil {
			t.Error(err)
		}
	})
	return b
}

func newCounterAccounting(t *testing.T, interval time.Duration) (*radiusAcct, *acctCapture) {
	t.Helper()
	sharedKey := []byte("counter-lifetime")
	capture := newAcctCapture()
	conn, addr := startAcctServer(t, sharedKey, capture)
	t.Cleanup(func() { _ = conn.Close() })
	client, err := radius.NewClient(radius.ClientConfig{
		Servers: []radius.Server{{Address: addr, SharedKey: sharedKey}},
		Timeout: time.Second, Retries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	acct := newRADIUSAcct()
	acct.setClient(client, "counter-nas", interval, addr, nil, "")
	t.Cleanup(acct.Stop)
	return acct, capture
}

// Scan received records, not elapsed time. A timer tick racing a stats update
// may legitimately carry the previous snapshot; a later matching Interim is
// the barrier before Stop. The deadline also bounds a missing/wrong record.
func (c *acctCapture) waitForCounterRecord(t *testing.T, description string, match func(capturedAcct) bool) capturedAcct {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		c.mu.Lock()
		for _, packet := range c.packets {
			if match(packet) {
				c.mu.Unlock()
				return packet
			}
		}
		c.mu.Unlock()
		select {
		case <-c.waitCh:
		case <-deadline.C:
			c.mu.Lock()
			packets := append([]capturedAcct(nil), c.packets...)
			c.mu.Unlock()
			t.Fatalf("timed out waiting for %s; received %+v", description, packets)
		}
	}
}

func counterLifetimeStart(t *testing.T, acct *radiusAcct, capture *acctCapture, username string) capturedAcct {
	t.Helper()
	acct.onSessionIPAssigned(&events.SessionIPAssignedPayload{
		TunnelID: 2030, SessionID: 49, Username: username,
		PeerAddr: "198.51.100.42", PppInterface: "ppp4242",
	})
	start := capture.waitForCounterRecord(t, username+" Start", func(p capturedAcct) bool {
		return p.statusType == radius.AcctStatusStart && p.username == username
	})
	if start.sessionID == "" {
		t.Fatal("Start lacks accounting identity")
	}
	return start
}

func counterLifetimeStop(t *testing.T, acct *radiusAcct, capture *acctCapture, start capturedAcct) capturedAcct {
	t.Helper()
	acct.onSessionDown(&events.SessionDownPayload{
		TunnelID: 2030, SessionID: 49, Cause: events.TerminateCauseUserRequest,
	})
	stop := capture.waitForCounterRecord(t, start.username+" Stop", func(p capturedAcct) bool {
		return p.statusType == radius.AcctStatusStop && p.sessionID == start.sessionID
	})
	if stop.username != start.username || stop.peerAddr != start.peerAddr || stop.terminateCause != uint32(events.TerminateCauseUserRequest) {
		t.Fatalf("Stop lost known subscriber/termination facts: %+v", stop)
	}
	if value := stop.numericAttrs[radius.AttrAcctSessionTime]; len(value) != 4 {
		t.Fatalf("Stop lost session time: %x", value)
	}
	return stop
}

func counterAttrs(stats iface.InterfaceStats) map[uint8]uint32 {
	attrs := map[uint8]uint32{
		radius.AttrAcctInputOctets: uint32(stats.RxBytes), radius.AttrAcctOutputOctets: uint32(stats.TxBytes),
		radius.AttrAcctInputPackets: uint32(stats.RxPackets), radius.AttrAcctOutputPackets: uint32(stats.TxPackets),
	}
	if stats.RxBytes>>32 != 0 {
		attrs[radius.AttrAcctInputGigawords] = uint32(stats.RxBytes >> 32)
	}
	if stats.TxBytes>>32 != 0 {
		attrs[radius.AttrAcctOutputGigawords] = uint32(stats.TxBytes >> 32)
	}
	return attrs
}

func hasCounterAttrs(packet capturedAcct, want map[uint8]uint32) bool {
	for _, attr := range []uint8{
		radius.AttrAcctInputOctets, radius.AttrAcctOutputOctets,
		radius.AttrAcctInputPackets, radius.AttrAcctOutputPackets,
		radius.AttrAcctInputGigawords, radius.AttrAcctOutputGigawords,
	} {
		value, present := packet.numericAttrs[attr]
		expected, required := want[attr]
		if present != required || (required && (len(value) != 4 || binary.BigEndian.Uint32(value) != expected)) {
			return false
		}
	}
	return true
}

func assertCounterAttrs(t *testing.T, packet capturedAcct, want map[uint8]uint32) {
	t.Helper()
	if !hasCounterAttrs(packet, want) {
		t.Errorf("wire counters for %s status %d = %v, want %v (unlisted counters absent)", packet.username, packet.statusType, packet.numericAttrs, want)
	}
}

// RFC 2866 Sections 5.3/5.4 count octets over this service, not the retained
// PPP unit's lifetime. RFC 2869 Sections 5.1/5.2 count wraps over that service.
// These UDP Stop assertions include a borrow across the low 32-bit boundary.
func TestAccountingCountersRetainedPPPUnit(t *testing.T) {
	if !isolatedAcctCounters(t) {
		return
	}
	backend := installAcctCounterBackend(t)
	acct, capture := newCounterAccounting(t, time.Hour)
	backend.set(iface.InterfaceStats{RxBytes: 100, TxBytes: 200, RxPackets: 5, TxPackets: 7}, nil)
	first := counterLifetimeStart(t, acct, capture, "alice")
	retained := iface.InterfaceStats{RxBytes: 1<<32 - 16, TxBytes: 1<<32 - 32, RxPackets: 71, TxPackets: 93}
	backend.set(retained, nil)
	assertCounterAttrs(t, counterLifetimeStop(t, acct, capture, first), counterAttrs(iface.InterfaceStats{
		RxBytes: 1<<32 - 116, TxBytes: 1<<32 - 232, RxPackets: 66, TxPackets: 86,
	}))

	idle := counterLifetimeStart(t, acct, capture, "bob-idle")
	assertCounterAttrs(t, counterLifetimeStop(t, acct, capture, idle), counterAttrs(iface.InterfaceStats{}))

	busy := counterLifetimeStart(t, acct, capture, "carol-busy")
	backend.set(iface.InterfaceStats{RxBytes: 2<<32 + 16, TxBytes: 3<<32 + 32, RxPackets: 171, TxPackets: 293}, nil)
	assertCounterAttrs(t, counterLifetimeStop(t, acct, capture, busy), counterAttrs(iface.InterfaceStats{
		RxBytes: 1<<32 + 32, TxBytes: 2<<32 + 64, RxPackets: 100, TxPackets: 200,
	}))
	if first.sessionID == idle.sessionID || first.sessionID == busy.sessionID || idle.sessionID == busy.sessionID {
		t.Fatalf("replacement reused accounting identity: %q, %q, %q", first.sessionID, idle.sessionID, busy.sessionID)
	}
}

// Drive ResetCounters and GetStats through real iface dispatch. Accounting's
// default producer must bypass that display baseline, including for Interim.
func TestAccountingCountersIgnoreDisplayClear(t *testing.T) {
	if !isolatedAcctCounters(t) {
		return
	}
	backend := installAcctCounterBackend(t)
	acct, capture := newCounterAccounting(t, 200*time.Millisecond)
	backend.set(iface.InterfaceStats{RxBytes: 1000, TxBytes: 2000, RxPackets: 10, TxPackets: 20}, nil)
	start := counterLifetimeStart(t, acct, capture, "display-clear")
	backend.set(iface.InterfaceStats{RxBytes: 1600, TxBytes: 2800, RxPackets: 16, TxPackets: 28}, nil)
	if err := iface.ResetCounters("ppp4242"); err != nil {
		t.Fatal(err)
	}
	backend.set(iface.InterfaceStats{RxBytes: 2100, TxBytes: 3500, RxPackets: 21, TxPackets: 35}, nil)
	display, err := iface.GetStats("ppp4242")
	if err != nil {
		t.Fatal(err)
	}
	if display.RxBytes != 500 || display.TxBytes != 700 || display.RxPackets != 5 || display.TxPackets != 7 {
		t.Fatalf("display clear did not take effect: %+v", display)
	}
	want := counterAttrs(iface.InterfaceStats{RxBytes: 1100, TxBytes: 1500, RxPackets: 11, TxPackets: 15})
	capture.waitForCounterRecord(t, "Interim with lifetime totals despite display clear", func(p capturedAcct) bool {
		return p.statusType == radius.AcctStatusInterimUpdate && p.sessionID == start.sessionID && hasCounterAttrs(p, want)
	})
	assertCounterAttrs(t, counterLifetimeStop(t, acct, capture, start), want)
}

func TestAccountingCountersUnavailableSnapshots(t *testing.T) {
	if !isolatedAcctCounters(t) {
		return
	}
	backend := installAcctCounterBackend(t)
	acct, capture := newCounterAccounting(t, 200*time.Millisecond)
	t.Run("baseline-read-fails", func(t *testing.T) {
		// A successful final read cannot reconstruct an unavailable baseline.
		backend.set(iface.InterfaceStats{}, errors.New("raw baseline unavailable"))
		start := counterLifetimeStart(t, acct, capture, "missing-baseline")
		backend.set(iface.InterfaceStats{RxBytes: 5000, TxBytes: 7000, RxPackets: 50, TxPackets: 70}, nil)
		assertCounterAttrs(t, counterLifetimeStop(t, acct, capture, start), nil)
	})
	t.Run("counter-decreases", func(t *testing.T) {
		// One regressing field invalidates the snapshot; unsigned subtraction
		// must not manufacture a huge bill while the other counters increase.
		backend.set(iface.InterfaceStats{RxBytes: 5000, TxBytes: 7000, RxPackets: 50, TxPackets: 70}, nil)
		start := counterLifetimeStart(t, acct, capture, "decreased-counter")
		backend.set(iface.InterfaceStats{RxBytes: 4999, TxBytes: 8000, RxPackets: 60, TxPackets: 80}, nil)
		capture.waitForCounterRecord(t, "Interim omitting decreased snapshot", func(p capturedAcct) bool {
			return p.statusType == radius.AcctStatusInterimUpdate && p.sessionID == start.sessionID && hasCounterAttrs(p, nil)
		})
		// Numerical catch-up cannot recover continuity after a reset.
		backend.set(iface.InterfaceStats{RxBytes: 6000, TxBytes: 9000, RxPackets: 70, TxPackets: 90}, nil)
		assertCounterAttrs(t, counterLifetimeStop(t, acct, capture, start), nil)
	})
}
