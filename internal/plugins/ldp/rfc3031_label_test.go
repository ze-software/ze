// Design: docs/architecture/ldp/mpls-ldp.md -- RFC 3031 label uniqueness and penultimate hop popping
//
// RFC 3031 Section 3.14: "In all other cases, Rd MUST NOT distribute to Ru
// bindings of the same label value to two different FECs." Ze uses one
// per-platform label space, so the bindings it distributes to one peer carry a
// distinct label for each FEC. The Section 3.14 tests drive runSession to the
// operational state over a net.Pipe and decode the Label Mappings ze writes.
// The Section 3.16 tests at the end of the file moved here from fib_test.go
// unchanged, so the file holding their tags is named for the RFC they cite.
package ldp

import (
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// readLocalMappings reads n Label Mapping PDUs from remote and returns the label
// each FEC was bound to. It fails the test on any other message, and on a FEC
// mapped twice, which a single advertisement pass never does.
func readLocalMappings(t *testing.T, remote net.Conn, n int) map[netip.Prefix]uint32 {
	t.Helper()
	got := make(map[netip.Prefix]uint32, n)
	for range n {
		_, msgHdr, body := readLDPPDU(t, remote)
		if msgHdr.Type != MsgTypeLabelMapping {
			t.Fatalf("message = %#x, want Label Mapping (%#x)", msgHdr.Type, MsgTypeLabelMapping)
		}
		lm, err := decodeLabelMapping(msgHdr.MessageID, body)
		if err != nil {
			t.Fatalf("decodeLabelMapping: %v", err)
		}
		if _, dup := got[lm.FEC.Prefix]; dup {
			t.Fatalf("FEC %s mapped twice in one advertisement pass", lm.FEC.Prefix)
		}
		got[lm.FEC.Prefix] = lm.Label
	}
	return got
}

// requireDistinctLabels fails the test when two FECs in mappings share a label.
func requireDistinctLabels(t *testing.T, mappings map[netip.Prefix]uint32) {
	t.Helper()
	owner := make(map[uint32]netip.Prefix, len(mappings))
	for fec, label := range mappings {
		if other, taken := owner[label]; taken {
			t.Errorf("label %d distributed for two FECs, %s and %s", label, other, fec)
			continue
		}
		owner[label] = fec
	}
}

// RFC requirement: RFC3031-3.14-1 positive -- every local FEC ze originates is
// distributed to one peer with its own label: N FECs bound through EnsureLocal
// reach the peer as N Label Mappings carrying N distinct labels, each the label
// EnsureLocal bound to that FEC.
func TestRFC3031LocalFECsGetDistinctLabels(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	sess := rfcTestSession(local)
	fecs := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.1/32"),
		netip.MustParsePrefix("10.1.0.0/24"),
		netip.MustParsePrefix("10.2.0.0/24"),
		netip.MustParsePrefix("192.0.2.0/24"),
	}
	want := make(map[netip.Prefix]uint32, len(fecs))
	for _, fec := range fecs {
		want[fec] = sess.lib.EnsureLocal(fec).Label
	}

	stop := readSessionUp(t, sess, remote)
	defer stop()

	got := readLocalMappings(t, remote, len(fecs))
	requireDistinctLabels(t, got)
	for _, fec := range fecs {
		if got[fec] != want[fec] {
			t.Errorf("FEC %s mapped with label %d, want the bound label %d", fec, got[fec], want[fec])
		}
	}
}

// RFC requirement: RFC3031-3.14-1 negative -- once the local label counter wraps
// from MaxLabel back to 16, a label still bound to a FEC is never bound to a
// second one, and a FEC bound again keeps its label: the mappings sent to one
// peer after the wrap carry no label twice.
func TestRFC3031WrappedAllocatorNeverReissuesALabel(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	sess := rfcTestSession(local)
	early := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.1/32"),
		netip.MustParsePrefix("10.1.0.0/24"),
		netip.MustParsePrefix("10.2.0.0/24"),
	}
	earlyLabels := make(map[netip.Prefix]uint32, len(early))
	for _, fec := range early {
		earlyLabels[fec] = sess.lib.EnsureLocal(fec).Label // 16, 17, 18
	}

	// Move the counter to the top of the label space, so the next allocations
	// run past MaxLabel and wrap onto the labels the early FECs still hold.
	sess.lib.mu.Lock()
	sess.lib.nextLabel = MaxLabel - 1
	sess.lib.mu.Unlock()

	late := []netip.Prefix{
		netip.MustParsePrefix("10.3.0.0/24"),
		netip.MustParsePrefix("10.4.0.0/24"),
		netip.MustParsePrefix("10.5.0.0/24"),
		netip.MustParsePrefix("10.6.0.0/24"),
	}
	for _, fec := range late {
		sess.lib.EnsureLocal(fec)
	}
	for _, fec := range early {
		if again := sess.lib.EnsureLocal(fec).Label; again != earlyLabels[fec] {
			t.Errorf("FEC %s rebound to label %d after the wrap, want its label %d", fec, again, earlyLabels[fec])
		}
	}

	stop := readSessionUp(t, sess, remote)
	defer stop()

	got := readLocalMappings(t, remote, len(early)+len(late))
	requireDistinctLabels(t, got)
	for _, fec := range early {
		if got[fec] != earlyLabels[fec] {
			t.Errorf("FEC %s mapped with label %d, want its label %d", fec, got[fec], earlyLabels[fec])
		}
	}
}

// RFC 3031 Section 3.16: "An LSR which is capable of popping the label stack at
// all MUST do penultimate hop popping when so requested by its downstream label
// distribution peer." The downstream peer requests it by binding the FEC to
// Implicit NULL (label 3). Ze has no transit swap, so the request is honored at
// ingress only: ze imposes no label and forwards the FEC as plain IP. The two
// tests below check the (mpls-fib, entry) event ze hands fib-kernel, not the
// kernel route fib-kernel installs from it.

// RFC requirement: RFC3031-3.16-1 negative -- with no penultimate-hop-popping
// request, ze as ingress keeps the label: a real label (17000) learned from the
// downstream peer produces a push entry imposing exactly that label.
// VALIDATES: AC-4 -- a real label learned from a peer becomes an ingress push.
func TestApplyRemoteBindingRealLabel(t *testing.T) {
	bus := &captureBus{}
	fib := newLDPFIB(bus, slogutil.DiscardLogger())
	fec := netip.MustParsePrefix("10.9.0.0/24")
	peer := netip.MustParseAddr("10.0.0.1")

	applyRemoteBinding(fib, fec, 17000, peer, slogutil.DiscardLogger())

	e := bus.lastEntry(t)
	assert.Equal(t, mplsfibevents.OpPush, e.Op)
	assert.Equal(t, []uint32{17000}, e.OutLabels)
}

// RFC requirement: RFC3031-3.16-1 positive -- when the downstream peer requests
// penultimate hop popping by binding the FEC to Implicit NULL, ze as ingress
// imposes no label: no push entry is emitted for the FEC.
// VALIDATES: implicit-null (3) means forward as plain IP -- ze must NOT push a
// label (it would otherwise impose label 3 on the wire, breaking forwarding).
func TestApplyRemoteBindingImplicitNull(t *testing.T) {
	bus := &captureBus{}
	fib := newLDPFIB(bus, slogutil.DiscardLogger())
	fec := netip.MustParsePrefix("10.9.0.0/24")
	peer := netip.MustParseAddr("10.0.0.1")

	applyRemoteBinding(fib, fec, ImplicitNull, peer, slogutil.DiscardLogger())

	assert.Empty(t, bus.emits, "implicit-null must not program a push entry")
}
