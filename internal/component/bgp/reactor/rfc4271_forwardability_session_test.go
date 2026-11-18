// Design: docs/architecture/core-design.md -- Routing-Table selection and forwarding.
// Related: rfc4271_selected_advertisement_test.go -- ordinary selected output.
// Related: rfc4271_selected_boundaries_test.go -- ingress and capture fences.
package reactor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// TestRFC4271OrdinaryOutputRequiresForwardableRoute observes ordinary recipient
// writes while a retained route loses and regains independent interior reachability.
// Useful K output and actual event settlement bound the forbidden-output checks.
// RFC 4271 Section 9.1.3: "A route SHALL NOT be installed in the Adj-Rib-Out
// unless the destination, and NEXT_HOP described by this route, may be forwarded
// appropriately by the Routing Table."
// This uses the default forwarding policy, not the following paragraph's local
// policy exception. Its Routing-Table observations do not claim kernel programming.
func TestRFC4271OrdinaryOutputRequiresForwardableRoute(t *testing.T) {
	// RFC requirement: RFC4271-9.2-2 positive -- For ordinary IPv4 selected output to the internal recipient under default Routing-Table policy, every successful subject or control announcement has both its sampled destination host and emitted NEXT_HOP resolvable at one stable Routing-Table revision; useful output and independently captured announcement history prevent empty-output success.

	if ribpolicy.Current() != nil {
		t.Fatal("forwardability witness requires the default Routing-Table import policy")
	}
	settings := []*PeerSettings{
		acceptingLiveSettings(t, "192.0.2.231", 65000, 65001),
		acceptingLiveSettings(t, "192.0.2.232", 65000, 65002),
		acceptingLiveSettings(t, "192.0.2.233", 65000, 65000),
	}
	for _, setting := range settings {
		setting.NextHopMode = NextHopAuto
	}
	peers := lowLiveStart(t, lowLiveOptions{withoutRS: true}, settings...)
	loc := locrib.Default()
	if loc == nil {
		t.Fatal("running ordinary RIB has no daemon Routing Table")
	}
	subject := netip.MustParsePrefix("198.19.253.0/24")
	control := netip.MustParsePrefix("198.19.254.0/24")
	nextHop := settings[0].Address
	controlHop := settings[1].Address
	forwardabilityNoCover(t, loc, subject)
	forwardabilityNoCover(t, loc, control)
	if igpcost.Resolve(loc, nextHop).Resolved {
		t.Fatal("subject NEXT_HOP unexpectedly resolves before its interior route exists")
	}
	protocol := redistevents.RegisterProtocol("forwardability-session-test")
	host := netip.PrefixFrom(nextHop, 32)
	controlHost := netip.PrefixFrom(controlHop, 32)
	loc.Insert(family.IPv4Unicast, controlHost, locrib.Path{Source: protocol, Metric: 7})
	t.Cleanup(func() {
		loc.Remove(family.IPv4Unicast, host, protocol, 0)
		loc.Remove(family.IPv4Unicast, controlHost, protocol, 0)
	})
	if !igpcost.Resolve(loc, controlHop).Resolved {
		t.Fatal("independent control NEXT_HOP does not resolve")
	}
	a := lowLiveConnect(t, peers[0], 0x0a00fd01)
	d := lowLiveConnect(t, peers[1], 0x0a00fd02)
	c, observer := forwardabilityConnect(t, peers[2], loc, [2]netip.Prefix{subject, control})
	live := []*lowLivePeer{a, d, c}
	attrs := synthesizedNextHopAttrs(nextHop, 65001)
	controlAttrs := synthesizedNextHopAttrs(controlHop, 65002)
	// RFC 4271 Sections 4.3 and 9.1.3: real received input and useful output.
	ownershipLiveSend(t, d, nil, controlAttrs, control)
	ordinarySelectedStored(t, d, control)
	forwardabilityWaitRoute(t, c, control, controlHop)
	forwardabilitySettle(t, d, c)
	forwardabilityAssertRoute(t, ownershipLiveRead(t, c), control, controlHop, 65002)

	ownershipLiveSend(t, a, nil, attrs, subject)
	ordinarySelectedStored(t, a, subject)
	forwardabilitySettle(t, a, c)
	view := ownershipLiveRead(t, c)
	// RFC requirement: RFC4271-9.2-2 negative -- An initially retained IPv4 subject with no destination cover and an unresolved NEXT_HOP produces zero announcements to the ordinary internal recipient after event settlement, while the independently resolvable control remains advertised.

	if got := len(view.announcements[subject]); got != 0 {
		t.Errorf("unresolved subject was advertised %d times", got)
	}
	forwardabilityNoCover(t, loc, subject)
	forwardabilityAssertRoute(t, view, control, controlHop, 65002)
	observer.checkHistory(t, view)
	received := a.peer.Stats().UpdatesReceived

	// RFC 4271 Section 9.1.2: routing changes re-elect retained input without
	// another source UPDATE. The writer observer was armed before that input.
	loc.Insert(family.IPv4Unicast, host, locrib.Path{Source: protocol, Metric: 7})
	forwardabilityWaitRoute(t, c, subject, nextHop)
	forwardabilitySettle(t, a, c)
	view = ownershipLiveRead(t, c)
	forwardabilityAssertRoute(t, view, subject, nextHop, 65001)
	forwardabilityAssertRoute(t, view, control, controlHop, 65002)
	if got := len(view.announcements[subject]); got < 1 {
		t.Errorf("resolved subject announcements=%d, want useful output", got)
	}
	observer.checkHistory(t, view)
	beforeLoss := len(view.announcements[subject])

	if _, removed := loc.Remove(family.IPv4Unicast, host, protocol, 0); !removed {
		t.Fatal("subject interior route was not removed")
	}
	lowEventually(t, func() bool {
		_, found := loc.Best(family.IPv4Unicast, subject)
		return !found
	}, "unresolved retained subject removed from the Routing Table")
	forwardabilitySettle(t, a, c)
	view = ownershipLiveRead(t, c)
	ownershipLiveAbsent(t, view, subject)
	if got := view.withdrawals[subject]; got < 1 {
		t.Errorf("unresolved subject withdrawals=%d, want service withdrawal", got)
	}
	if got := len(view.announcements[subject]); got != beforeLoss {
		t.Errorf("NEXT_HOP loss changed subject announcement count from %d to %d", beforeLoss, got)
	}
	forwardabilityNoCover(t, loc, subject)
	forwardabilityAssertAnnouncements(t, view, subject, nextHop, 65001)
	forwardabilityAssertRoute(t, view, control, controlHop, 65002)
	observer.checkHistory(t, view)
	beforeRecovery := len(view.announcements[subject])
	withdrawalsBeforeRecovery := view.withdrawals[subject]

	loc.Insert(family.IPv4Unicast, host, locrib.Path{Source: protocol, Metric: 7})
	lowEventually(t, func() bool {
		return len(ownershipLiveRead(t, c).announcements[subject]) > beforeRecovery
	}, "retained subject recovered on the recipient without another UPDATE")
	forwardabilitySettle(t, a, c)
	view = ownershipLiveRead(t, c)
	forwardabilityAssertRoute(t, view, subject, nextHop, 65001)
	forwardabilityAssertRoute(t, view, control, controlHop, 65002)
	ownershipLiveWithdrawals(t, view, subject, withdrawalsBeforeRecovery)
	ownershipLiveWithdrawals(t, view, control, 0)
	if got := len(view.announcements[control]); got < 1 {
		t.Errorf("independent control announcements=%d, want useful output", got)
	}
	if got := a.peer.Stats().UpdatesReceived; got != received {
		t.Errorf("reachability lifecycle received %d source UPDATEs, started at %d", got, received)
	}
	ordinarySelectedStored(t, a, subject)
	ownershipLiveEstablished(t, live)
	observer.checkHistory(t, view)
}

func forwardabilityNoCover(t *testing.T, loc *locrib.RIB, prefix netip.Prefix) {
	t.Helper()
	host := prefix.Addr().Next()
	if _, cover, found := loc.LPM(family.IPv4Unicast, host); found {
		t.Fatalf("subject destination %s already has Routing-Table cover %s", host, cover)
	}
	if igpcost.Resolve(loc, host).Resolved {
		t.Fatalf("destination %s unexpectedly resolves without a cover", host)
	}
}

func forwardabilityWaitRoute(t *testing.T, peer *lowLivePeer, prefix netip.Prefix, nextHop netip.Addr) {
	t.Helper()
	lowEventually(t, func() bool {
		view := ownershipLiveRead(t, peer)
		return ordinarySelectedNextHop(t, view.routes[prefix]) == nextHop
	}, "forwardable ordinary recipient route "+prefix.String())
}

// forwardabilitySettle MUST run after the source input or observed routing
// transition. Application alone does not join deferred ordinary selected output.
// The caller MUST let capture run throughout settlement; no writer is paused.
func forwardabilitySettle(t *testing.T, source, target *lowLivePeer) {
	t.Helper()
	before := source.peer.Stats().KeepalivesReceived
	source.send(t, message.PackTo(message.NewKeepalive(), nil))
	lowEventually(t, func() bool {
		return source.peer.Stats().KeepalivesReceived > before
	}, "forwardability source ingress marker")
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := source.peer.receiveCut().wait(ctx); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bgp-rib", "bgp-adj-rib-in"} {
		process := source.peer.reactor.api.ProcessManager().GetProcess(name)
		if process == nil {
			t.Fatalf("forwardability process %s is missing", name)
		}
		if err := process.SettleEventsApplied(ctx); err != nil {
			t.Fatalf("settle %s: %v", name, err)
		}
	}
	forwardSocketBarrier(t, source.peer.reactor)
	ordinaryBoundaryCapture(t, target)
}

func forwardabilityAssertRoute(t *testing.T, view ownershipLiveProjection, prefix netip.Prefix, nextHop netip.Addr, sourceAS uint32) {
	t.Helper()
	forwardabilityAssertAnnouncements(t, view, prefix, nextHop, sourceAS)
	attrs, found := view.routes[prefix]
	if !found {
		t.Errorf("recipient has no useful route for %s", prefix)
		return
	}
	forwardabilityAssertAttributes(t, attrs, prefix, nextHop, sourceAS)
}

// forwardabilityAssertAnnouncements checks every emitted value, including any
// permitted duplicate, so a later useful route cannot hide an earlier bad one.
func forwardabilityAssertAnnouncements(t *testing.T, view ownershipLiveProjection, prefix netip.Prefix, nextHop netip.Addr, sourceAS uint32) {
	t.Helper()
	for _, attrs := range view.announcements[prefix] {
		forwardabilityAssertAttributes(t, attrs, prefix, nextHop, sourceAS)
	}
}

func forwardabilityAssertAttributes(t *testing.T, attrs []byte, prefix netip.Prefix, nextHop netip.Addr, sourceAS uint32) {
	t.Helper()
	want := synthesizedNextHopAttrs(nextHop, sourceAS)
	for _, code := range []attribute.AttributeCode{attribute.AttrOrigin, attribute.AttrASPath, attribute.AttrNextHop} {
		_, _, expected, _ := attribute.AttrFind(want, code)
		_, _, got, present := attribute.AttrFind(attrs, code)
		if !present {
			t.Errorf("recipient %s lost attribute %d", prefix, code)
			continue
		}
		if !bytes.Equal(got, expected) {
			t.Errorf("recipient %s attribute %d=%x, want %x", prefix, code, got, expected)
		}
	}
	_, _, preference, present := attribute.AttrFind(attrs, attribute.AttrLocalPref)
	if !present {
		t.Errorf("internal recipient %s has no LOCAL_PREF", prefix)
		return
	}
	if !bytes.Equal(preference, []byte{0, 0, 0, 100}) {
		t.Errorf("internal recipient %s LOCAL_PREF=%x, want 100", prefix, preference)
	}
}

// forwardabilityConnect wraps only the server endpoint before transport admission.
// Its cleanup MUST close both endpoints and join capture before reactor teardown.
func forwardabilityConnect(t *testing.T, peer *Peer, loc *locrib.RIB, prefixes [2]netip.Prefix) (*lowLivePeer, *forwardabilityWire) {
	t.Helper()
	lowEventually(t, func() bool {
		if peer.currentSession() == nil {
			return false
		}
		return peer.SessionState() == fsm.StateActive
	}, "forwardability recipient Active")
	server, remote := net.Pipe()
	observer := &forwardabilityWire{Conn: server, loc: loc, prefixes: prefixes}
	live := &lowLivePeer{peer: peer, remote: remote, stopped: make(chan struct{})}
	live.startCapture(t)
	t.Cleanup(func() {
		if err := remote.Close(); err != nil {
			t.Errorf("close forwardability remote: %v", err)
		}
		if err := server.Close(); err != nil {
			t.Errorf("close forwardability server: %v", err)
		}
		<-live.stopped
		observer.check(t)
	})
	if err := peer.acceptConnection(observer); err != nil {
		t.Fatal(err)
	}
	settings := peer.Settings()
	open := &message.Open{Version: 4, MyAS: uint16(settings.PeerAS), HoldTime: 90,
		BGPIdentifier:  0x0a00fd03,
		OptionalParams: []byte{2, 6, 65, 4, 0, 0, byte(settings.PeerAS >> 8), byte(settings.PeerAS), 2, 6, 1, 4, 0, 1, 0, 1}}
	live.send(t, message.PackTo(open, nil))
	lowEventually(t, func() bool {
		return peer.SessionState() == fsm.StateOpenConfirm
	}, "forwardability recipient OPEN accepted")
	live.send(t, message.PackTo(message.NewKeepalive(), nil))
	lowEventually(t, func() bool {
		if peer.State() != PeerStateEstablished {
			return false
		}
		return !peer.pendingSync()
	}, "forwardability recipient initial selected sync")
	return live, observer
}

const (
	forwardabilityObservationsMax = 64
	// The fixture negotiates no Extended Message capability. One incomplete
	// frame and a coalesced buffered write each fit the ordinary message bound.
	forwardabilityPendingMax = 2 * message.MaxMsgLen
)

// forwardabilityWire observes serialized successful writes without changing bytes
// or acknowledgments. Its zero value is invalid: forwardabilityConnect supplies
// a live transport, authoritative Routing Table and two IPv4 /24 subjects.
// Safe for concurrent checks. writeMu serializes transport writes; mu protects
// completed observations only. No Routing-Table lock survives snapshot().
type forwardabilityWire struct {
	net.Conn
	loc      *locrib.RIB
	prefixes [2]netip.Prefix
	writeMu  sync.Mutex
	pending  []byte
	mu       sync.Mutex
	observed []forwardabilityObservation
	failure  error
}

type forwardabilityObservation struct {
	prefix              netip.Prefix
	nextHop             netip.Addr
	destinationCover    netip.Prefix
	nextHopCover        netip.Prefix
	destinationVia      netip.Addr
	nextHopVia          netip.Addr
	destinationResolved bool
	nextHopResolved     bool
	revision            uint64
	frame               []byte
}

// Write samples complete frames before passing bytes to the real net.Pipe.
// Only a full successful underlying Write at the sampled Routing-Table revision
// commits its samples. A crossed revision is fixture-indeterminate. Short/error
// writes invalidate the scenario; neither can masquerade as forwarding evidence.
func (c *forwardabilityWire) Write(data []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	pending, observed, prepareErr := c.prepare(data)
	n, err := c.Conn.Write(data)
	if err != nil {
		c.fail(fmt.Errorf("recipient transport write: %w", err))
		return n, err
	}
	if n != len(data) {
		c.fail(io.ErrShortWrite)
		return n, err
	}
	if prepareErr != nil {
		c.fail(prepareErr)
		return n, err
	}
	c.pending = pending
	revision := c.loc.Revision()
	comparable := observed[:0]
	for sampleIndex := range observed {
		sample := &observed[sampleIndex]
		if sample.revision != revision {
			c.fail(fmt.Errorf("fixture-indeterminate recipient %s write crossed Routing-Table revisions %d to %d",
				sample.prefix, sample.revision, revision))
			continue
		}
		comparable = append(comparable, *sample)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.observed)+len(comparable) > forwardabilityObservationsMax {
		if c.failure == nil {
			c.failure = fmt.Errorf("recipient exceeded %d forwarding observations", forwardabilityObservationsMax)
		}
		return n, err
	}
	c.observed = append(c.observed, comparable...)
	return n, err
}

// prepare owns the incremental framing copy. The byte bound limits every walk;
// incomplete frames are retained until a later successful Write completes them.
func (c *forwardabilityWire) prepare(data []byte) ([]byte, []forwardabilityObservation, error) {
	if len(c.pending)+len(data) > forwardabilityPendingMax {
		return nil, nil, fmt.Errorf("recipient framing exceeded %d bytes", forwardabilityPendingMax)
	}
	remaining := append(bytes.Clone(c.pending), data...)
	var observed []forwardabilityObservation
	for len(remaining) >= message.HeaderLen {
		header, err := message.ParseHeader(remaining)
		if err != nil {
			return nil, nil, fmt.Errorf("recipient BGP header: %w", err)
		}
		length := int(header.Length)
		if length > message.MaxMsgLen {
			return nil, nil, fmt.Errorf("recipient unnegotiated BGP length %d", length)
		}
		if len(remaining) < length {
			break
		}
		frame := remaining[:length]
		remaining = remaining[length:]
		if header.Type != msgtype.TypeUPDATE {
			continue
		}
		// RFC 4271 Section 4.3: inspect actual framed legacy NLRI/NEXT_HOP.
		update, err := message.UnpackUpdate(frame[message.HeaderLen:])
		if err != nil {
			return nil, nil, fmt.Errorf("recipient UPDATE: %w", err)
		}
		prefixes, err := forwardabilityPrefixes(update.NLRI)
		if err != nil {
			return nil, nil, err
		}
		for _, prefix := range prefixes {
			for _, subject := range c.prefixes {
				if prefix != subject {
					continue
				}
				_, _, field, found := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
				if !found {
					return nil, nil, fmt.Errorf("recipient %s announcement has no NEXT_HOP", prefix)
				}
				if len(field) != 4 {
					return nil, nil, fmt.Errorf("recipient %s NEXT_HOP length %d", prefix, len(field))
				}
				nextHop := netip.AddrFrom4([4]byte(field))
				sample, err := c.snapshot(prefix, nextHop, frame)
				if err != nil {
					return nil, nil, err
				}
				observed = append(observed, sample)
			}
		}
	}
	return remaining, observed, nil
}

// forwardabilityPrefixes decodes only the negotiated non-ADD-PATH IPv4 framing.
// Its loop consumes at least one octet and is bounded by the actual NLRI field.
// RFC 4271 Section 4.3: "The Length field indicates the length in bits of the
// IP address prefix."
func forwardabilityPrefixes(field []byte) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	for len(field) != 0 {
		bits := int(field[0])
		if bits > 32 {
			return nil, fmt.Errorf("recipient IPv4 prefix length %d", bits)
		}
		octets := (bits + 7) / 8
		if len(field) < 1+octets {
			return nil, fmt.Errorf("recipient truncated IPv4 NLRI %x", field)
		}
		var address [4]byte
		copy(address[:], field[1:1+octets])
		prefixes = append(prefixes, netip.PrefixFrom(netip.AddrFrom4(address), bits).Masked())
		field = field[1+octets:]
	}
	return prefixes, nil
}

// snapshot samples one actual destination host and emitted next hop. Revision
// churn is an observation error, never a forwarding success. Eight attempts
// bound concurrent routing changes; each lookup releases its lock before return.
func (c *forwardabilityWire) snapshot(prefix netip.Prefix, nextHop netip.Addr, frame []byte) (forwardabilityObservation, error) {
	host := prefix.Addr().Next()
	for range 8 {
		revision := c.loc.Revision()
		destination, destinationCover, _ := c.loc.LPM(family.IPv4Unicast, host)
		hop, nextHopCover, _ := c.loc.LPM(family.IPv4Unicast, nextHop)
		destinationDistance := igpcost.Resolve(c.loc, host)
		nextHopDistance := igpcost.Resolve(c.loc, nextHop)
		if c.loc.Revision() != revision {
			continue
		}
		return forwardabilityObservation{
			prefix: prefix, nextHop: nextHop, destinationCover: destinationCover,
			nextHopCover: nextHopCover, destinationVia: destination.NextHop,
			nextHopVia: hop.NextHop, destinationResolved: destinationDistance.Resolved,
			nextHopResolved: nextHopDistance.Resolved, revision: revision,
			frame: bytes.Clone(frame),
		}, nil
	}
	return forwardabilityObservation{}, fmt.Errorf("recipient %s forwarding snapshot never stabilized", prefix)
}

func (c *forwardabilityWire) fail(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failure == nil {
		c.failure = err
	}
}

// check MUST run on the test goroutine and after final transport closure too.
// A later good snapshot cannot replace an earlier unresolved successful write.
func (c *forwardabilityWire) check(t *testing.T) {
	t.Helper()
	c.mu.Lock()
	failure := c.failure
	observed := append([]forwardabilityObservation(nil), c.observed...)
	c.mu.Unlock()
	if failure != nil {
		t.Errorf("forwardability observation failed: %v", failure)
	}
	for sampleIndex := range observed {
		sample := &observed[sampleIndex]
		if !sample.destinationResolved {
			t.Errorf("successful recipient announcement %s at Routing-Table revision %d has unforwardable destination: cover=%s via=%s NEXT_HOP=%s frame=%x",
				sample.prefix, sample.revision, sample.destinationCover, sample.destinationVia, sample.nextHop, sample.frame)
		}
		if !sample.nextHopResolved {
			t.Errorf("successful recipient announcement %s at Routing-Table revision %d has unforwardable NEXT_HOP %s: cover=%s via=%s frame=%x",
				sample.prefix, sample.revision, sample.nextHop, sample.nextHopCover, sample.nextHopVia, sample.frame)
		}
	}
}

// checkHistory pairs the Routing-Table oracle with the independently captured
// recipient history. Duplicate output is permitted, but missing observations
// cannot pass and every completed comparable write still gets its oracle.
func (c *forwardabilityWire) checkHistory(t *testing.T, view ownershipLiveProjection) {
	t.Helper()
	c.check(t)
	c.mu.Lock()
	counts := [2]int{}
	for sampleIndex := range c.observed {
		sample := &c.observed[sampleIndex]
		for index, prefix := range c.prefixes {
			if sample.prefix == prefix {
				counts[index]++
			}
		}
	}
	c.mu.Unlock()
	for index, prefix := range c.prefixes {
		if want := len(view.announcements[prefix]); counts[index] != want {
			t.Errorf("recipient forwarding observations for %s=%d, captured history has %d", prefix, counts[index], want)
		}
	}
}
