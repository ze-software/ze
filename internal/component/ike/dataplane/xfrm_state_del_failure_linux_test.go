// VALIDATES: spec-fixit-ike-resource-lifetime-leaks AC-1 -- when XfrmStateDel returns an
// error inside RemoveSA, the SPI is absent from the ESP form registry and the error still
// reaches the caller unchanged.
// PREVENTS: the host-wide raw ESP socket that a forgotten SPI holds open. The Forget call
// sat BELOW the delete's error return, and the second teardown of one Child SA (RFC 7296
// Section 1.4 Delete processing, engine/delete.go) is ordinary rather than exotic: the
// state is already gone, the kernel answers ESRCH, and the SPI stayed watched forever.
// The receiver then keeps its raw IPPROTO_ESP socket, which makes the kernel clone every
// bare ESP packet on the box, long after the last tunnel closed.
//
// No kernel state is set up: a delete for a state that was never installed is REFUSED by
// every kernel (ESRCH as root, EPERM unprivileged), which is exactly the case under test.
// So this needs no netns and no CAP_NET_ADMIN, and is merge-gated rather than nightly.

//go:build linux

package dataplane

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/core/slogutil"
)

func TestRemoveSAForgetsESPFormWhenStateDeleteFails(t *testing.T) {
	const spi uint32 = 0xfeed1234

	b := &xfrmBackend{espForms: newESPFormReceiver(slogutil.DiscardLogger())}
	// The registry is seeded directly rather than through Watch: Watch opens the raw
	// sockets, which needs CAP_NET_RAW, and the membership is what RemoveSA must drop.
	b.espForms.reg.watch(spi, natTarget(netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("203.0.113.9")))

	err := b.RemoveSA(spi, net.ParseIP("203.0.113.9"), ProtoESP)

	if err == nil {
		t.Fatal("the kernel accepted a delete for a state that was never installed, so this test exercises the success path and proves nothing")
	}
	if !strings.Contains(err.Error(), "xfrm: state del") {
		t.Errorf("RemoveSA returned %v, want the state-del failure reported unchanged to the caller", err)
	}
	if b.espForms.reg.watching(spi) {
		t.Errorf("spi %#x is still watched after a failed RemoveSA: the receiver holds a host-wide raw ESP socket for a tunnel that is gone", spi)
	}
}

type xfrmRollbackLogCapture struct {
	mu      sync.Mutex
	records []slog.Record
}

func (c *xfrmRollbackLogCapture) Enabled(_ context.Context, _ slog.Level) bool { return true }

//nolint:gocritic // slog.Handler interface requires slog.Record by value
func (c *xfrmRollbackLogCapture) Handle(_ context.Context, r slog.Record) error {
	c.mu.Lock()
	c.records = append(c.records, r.Clone())
	c.mu.Unlock()
	return nil
}

func (c *xfrmRollbackLogCapture) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *xfrmRollbackLogCapture) WithGroup(string) slog.Handler      { return c }

func (c *xfrmRollbackLogCapture) warns() []slog.Record {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []slog.Record
	for i := range c.records {
		if c.records[i].Level == slog.LevelWarn {
			out = append(out, c.records[i])
		}
	}
	return out
}

func xfrmAttrString(r *slog.Record, key string) (string, bool) {
	var val string
	var found bool
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			val = a.Value.String()
			found = true
			return false
		}
		return true
	})
	return val, found
}

// VALIDATES: when InstallSA adds an inbound XFRM state, dual-form ESP setup
// fails, and the rollback XfrmStateDel also fails, InstallSA logs a Warn
// carrying CodeDoctorIPsecXFRMRollbackFailed, the fix command, and the sink
// report outcome, invokes the registered rollback sink non-blockingly, and
// joins both the install error and the rollback error in its return value.
// PREVENTS: a stranded kernel XFRM state from going invisible at Debug when
// rollback deletion fails.
func TestXFRMInstallSARollbackFailureWarnsAndJoinsError(t *testing.T) {
	const spi uint32 = 0xfeed5678

	origAdd := xfrmStateAdd
	origDel := xfrmStateDel
	t.Cleanup(func() {
		xfrmStateAdd = origAdd
		xfrmStateDel = origDel
	})

	watchErr := errors.New("simulated dual-form open refusal")
	rollbackErr := errors.New("simulated kernel rollback delete refusal")
	xfrmStateAdd = func(*netlink.XfrmState) error { return nil }
	xfrmStateDel = func(*netlink.XfrmState) error { return rollbackErr }

	var (
		sinkMu  sync.Mutex
		sinkEvs []XFRMRollbackFailed
	)
	RegisterXFRMRollbackFailed(func(ev XFRMRollbackFailed) slog.Attr {
		sinkMu.Lock()
		sinkEvs = append(sinkEvs, ev)
		sinkMu.Unlock()
		return slog.String("report", "queued")
	})
	t.Cleanup(func() { RegisterXFRMRollbackFailed(nil) })

	capture := &xfrmRollbackLogCapture{}
	rec := newESPFormReceiver(slog.New(capture))
	rec.openPaths = func() (espFormPaths, error) { return espFormPaths{}, watchErr }
	b := &xfrmBackend{espForms: rec}

	err := b.InstallSA(SAParams{
		SPI:                spi,
		Src:                net.ParseIP("198.51.100.7"),
		Dst:                net.ParseIP("203.0.113.9"),
		Proto:              ProtoESP,
		Mode:               ModeTunnel,
		Dir:                SADirIn,
		EncAlgo:            "aes128gcm",
		EncKey:             make([]byte, 20),
		IsAEAD:             true,
		UDPEncap:           true,
		UDPEncapSPort:      4500,
		UDPEncapDPort:      4500,
		AcceptBothESPForms: true,
	})
	if err == nil {
		t.Fatal("InstallSA returned nil, want joined install + rollback error")
	}
	if !errors.Is(err, watchErr) {
		t.Errorf("InstallSA error %v does not wrap watch failure %v", err, watchErr)
	}
	if !errors.Is(err, rollbackErr) {
		t.Errorf("InstallSA error %v does not wrap rollback failure %v", err, rollbackErr)
	}

	warns := capture.warns()
	if len(warns) != 1 {
		t.Fatalf("warn count = %d, want 1", len(warns))
	}
	if code, ok := xfrmAttrString(&warns[0], "code"); !ok || code != CodeDoctorIPsecXFRMRollbackFailed {
		t.Errorf("warn code = %q (present=%v), want %q", code, ok, CodeDoctorIPsecXFRMRollbackFailed)
	}
	if fix, ok := xfrmAttrString(&warns[0], "fix"); !ok || fix != "ze show vpn ipsec dataplane sa" {
		t.Errorf("warn fix = %q (present=%v), want %q", fix, ok, "ze show vpn ipsec dataplane sa")
	}
	if rep, ok := xfrmAttrString(&warns[0], "report"); !ok || rep != "queued" {
		t.Errorf("warn report = %q (present=%v), want %q", rep, ok, "queued")
	}

	sinkMu.Lock()
	defer sinkMu.Unlock()
	if len(sinkEvs) != 1 {
		t.Fatalf("sink event count = %d, want 1", len(sinkEvs))
	}
	if sinkEvs[0].SPI != spi || !errors.Is(sinkEvs[0].RollbackErr, rollbackErr) || !errors.Is(sinkEvs[0].InstallErr, watchErr) {
		t.Errorf("unexpected sink event: %+v", sinkEvs[0])
	}
}
