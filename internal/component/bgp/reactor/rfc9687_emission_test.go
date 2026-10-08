package reactor

import (
	"bufio"
	"bytes"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/test/sim"
)

// rfc9687EmissionConn records the transport Close called by the real timer path.
// The fixture and its fake-clock callbacks run synchronously.
type rfc9687EmissionConn struct {
	recordingConn
	closed bool
}

func (c *rfc9687EmissionConn) Close() error {
	c.closed = true
	return nil
}

// rfc9687EmissionSession negotiates a real session over the synchronous recording
// transport. The fake clock fires the actual timer callbacks, not a test echo.
func rfc9687EmissionSession(t *testing.T) (*Session, *rfc9687EmissionConn, *sim.FakeClock) {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = 90 * time.Second
	settings.SendHoldTime = rfc9687SendHold
	s := NewSession(settings)
	clock := sim.NewFakeClock(time.Now())
	s.SetClock(clock)
	conn := &rfc9687EmissionConn{}
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if err := s.Accept(conn); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		s.timers.StopAll()
		s.stopSendHoldTimer()
		s.closeConn()
	})
	if err := s.handleOpen(rfc9687PeerOpen(90)[message.HeaderLen:]); err != nil {
		t.Fatal(err)
	}
	if err := s.handleKeepalive(); err != nil {
		t.Fatal(err)
	}
	if s.State() != fsm.StateEstablished || s.sendHoldDeadline.Load() == 0 {
		t.Fatal("precondition: session must be Established with SendHold armed")
	}
	return s, conn, clock
}

// TestRFC9687SuppressedAttemptsDoNotRestartSendHold repeatedly invokes real
// writer rails at 0.3, 0.6 and 0.9 SendHoldTime. The transport must receive no
// bytes, the exact deadline must not change, and Event 29 must close at expiry.
// MUTATION: reset SendHold unconditionally after an empty successful flush.
// RFC requirement: RFC9687-4.3-8 negative -- policy-suppressed, duplicate, stale and empty write attempts send no BGP message, cannot move the deadline, and cannot prevent SendHold expiry.
func TestRFC9687SuppressedAttemptsDoNotRestartSendHold(t *testing.T) {
	for _, name := range []string{
		"policy-update", "policy-held", "policy-batch", "duplicate-update",
		"duplicate-held", "duplicate-announce", "duplicate-body", "duplicate-message",
		"stale-batch-raw", "stale-batch-parsed", "empty-dirty-flush", "empty-raw", "paths-limit",
	} {
		t.Run(name, func(t *testing.T) {
			s, conn, clock := rfc9687EmissionSession(t)
			update := &message.Update{
				PathAttributes: []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 99},
				NLRI: []byte{24, 198, 51, 100},
			}
			route := bgptypes.RouteSpec{Prefix: netip.MustParsePrefix("198.51.100.0/24"), NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.99"))}
			peer := NewPeer(s.settings)
			peer.session = s
			item := fwdItem{peer: peer, session: s, originated: true, updates: []*message.Update{update}}
			switch name {
			case "policy-update", "policy-held", "policy-batch":
				s.egressRouteFilter = func([]byte) (bool, []byte) { return true, nil }
			case "duplicate-update", "duplicate-held", "duplicate-body", "duplicate-message":
				if err := s.SendUpdate(update); err != nil {
					t.Fatal(err)
				}
			case "duplicate-announce":
				if err := s.SendAnnounce(route, netip.Addr{}, 65001, false, true, false); err != nil {
					t.Fatal(err)
				}
			case "stale-batch-raw", "stale-batch-parsed":
				item.session = NewSession(s.settings)
				if name == "stale-batch-raw" {
					item.rawBodies = [][]byte{message.PackTo(update, nil)[message.HeaderLen:]}
					item.updates = nil
				}
			case "paths-limit":
				// Keep the negotiated nonzero hold time; UPDATE-only fixtures
				// intentionally have zero timers and cannot prove liveness.
				caps := []capability.Capability{
					&capability.ASN4{ASN: 65001},
					&capability.Multiprotocol{AFI: 1, SAFI: 1},
					&capability.AddPath{Families: []capability.AddPathFamily{{AFI: 1, SAFI: 1, Mode: capability.AddPathBoth}}},
				}
				s.negotiateWith(caps, append(caps, &capability.PathsLimit{
					Entries: []capability.PathsLimitEntry{{AFI: 1, SAFI: 1, Limit: 1}},
				}))
				peer.setEncodingContexts(s.Negotiated())
				t.Cleanup(peer.clearEncodingContexts)
				if err := s.SendUpdate(pathsLimitUpdate(family.IPv4Unicast, false, pathsLimitNLRI(1, "198.51.100.0/24"))); err != nil {
					t.Fatal(err)
				}
				update = pathsLimitUpdate(family.IPv4Unicast, false, pathsLimitNLRI(2, "198.51.100.0/24"))
			}
			before := len(conn.written())
			deadline := s.sendHoldDeadline.Load()
			if want := clock.Now().Add(rfc9687SendHold).UnixNano(); deadline != want {
				t.Fatalf("precondition: initial deadline = %d, want %d", deadline, want)
			}
			for range 3 {
				clock.Add(3 * rfc9687SendHold / 10)
				var err error
				switch name {
				case "policy-update", "duplicate-update", "paths-limit":
					err = s.SendUpdate(update)
				case "policy-held", "duplicate-held":
					s.HoldWrites()
					err = s.SendUpdateHeld(update)
					s.releaseWrites()
				case "policy-batch", "stale-batch-raw", "stale-batch-parsed":
					fwdBatchHandler(fwdKey{}, []fwdItem{item})
				case "duplicate-announce":
					err = s.SendAnnounce(route, netip.Addr{}, 65001, false, true, false)
				case "duplicate-body":
					err = s.sendRawUpdateBody(message.PackTo(update, nil)[message.HeaderLen:])
				case "duplicate-message":
					err = s.writeMessage(conn, update)
				case "empty-dirty-flush":
					s.appendFwdDirty(s)
					s.flushFwdDirty()
				case "empty-raw":
					err = s.SendRawMessage(0, nil)
				}
				if err != nil {
					t.Fatal(err)
				}
				if got := len(conn.written()); got != before {
					t.Fatalf("no-emission attempt wrote %d additional bytes", got-before)
				}
				if got := s.sendHoldDeadline.Load(); got != deadline {
					t.Errorf("no-emission attempt moved deadline from %d to %d", deadline, got)
				}
			}
			clock.Add(rfc9687SendHold/10 - time.Nanosecond)
			if s.State() != fsm.StateEstablished {
				t.Fatal("session expired before the exact SendHold deadline")
			}
			clock.Add(time.Nanosecond)
			if s.State() != fsm.StateIdle || s.Conn() != nil || s.sendHoldDeadline.Load() != 0 {
				t.Fatal("no-emission attempts prevented SendHold expiry and connection release")
			}
			if !conn.closed {
				t.Fatal("SendHold expiry did not close the transport")
			}
			want := message.PackTo(&message.Notification{ErrorCode: message.NotifySendHoldTimerExpired}, nil)
			if got := conn.written()[before:]; !bytes.Equal(got, want) {
				t.Fatalf("expiry wire = %x, want only SendHold notification %x", got, want)
			}
		})
	}
}

// TestRFC9687EmissionRestartsExactSendHoldDeadline compares emitted frames with
// the transport capture, including a bufio direct write with nothing left to
// flush. A later empty flush must not claim that old emission a second time.
// MUTATION: reset only when bufWriter.Buffered() is nonzero, or keep pending emission after flush.
// RFC requirement: RFC9687-4.3-8 positive -- genuine UPDATE, KEEPALIVE, refresh, BoRR and EoRR frames restart the exact deadline; only a new emitted frame can restart it again.
func TestRFC9687EmissionRestartsExactSendHoldDeadline(t *testing.T) {
	for _, name := range []string{"update", "auto-flush", "dirty-flush", "keepalive", "refresh", "borr", "eorr"} {
		t.Run(name, func(t *testing.T) {
			s, conn, clock := rfc9687EmissionSession(t)
			var packet message.Message = &message.Update{WithdrawnRoutes: []byte{24, 10, 0, 0}}
			switch name {
			case "auto-flush":
				s.bufWriter = bufio.NewWriterSize(conn, 1)
			case "keepalive":
				packet = message.NewKeepalive()
			case "refresh":
				packet = &message.RouteRefresh{AFI: 1, SAFI: 1, Subtype: message.RouteRefreshNormal}
			case "borr":
				packet = &message.RouteRefresh{AFI: 1, SAFI: 1, Subtype: message.RouteRefreshBoRR}
			case "eorr":
				packet = &message.RouteRefresh{AFI: 1, SAFI: 1, Subtype: message.RouteRefreshEoRR}
			}
			before := len(conn.written())
			initialDeadline := s.sendHoldDeadline.Load()
			clock.Add(6 * rfc9687SendHold / 10)
			if name == "dirty-flush" {
				s.writeMu.Lock()
				err := s.writeRawUpdateBody(message.PackTo(packet, nil)[message.HeaderLen:])
				s.writeMu.Unlock()
				if err != nil {
					t.Fatal(err)
				}
				if len(conn.written()) != before {
					t.Fatal("buffered UPDATE reached transport before flush")
				}
				if got := s.sendHoldDeadline.Load(); got != initialDeadline {
					t.Fatalf("buffering alone moved deadline from %d to %d", initialDeadline, got)
				}
				clock.Add(time.Second)
				s.appendFwdDirty(s)
				s.flushFwdDirty()
			} else if err := s.writeMessage(conn, packet); err != nil {
				t.Fatal(err)
			}
			if got, want := conn.written()[before:], message.PackTo(packet, nil); !bytes.Equal(got, want) {
				t.Fatalf("sent frame = %x, want %x", got, want)
			}
			deadline := clock.Now().Add(rfc9687SendHold).UnixNano()
			if got := s.sendHoldDeadline.Load(); got != deadline {
				t.Fatalf("emitted frame deadline = %d, want %d", got, deadline)
			}
			clock.Add(6 * rfc9687SendHold / 10)
			s.appendFwdDirty(s)
			s.flushFwdDirty()
			if got := s.sendHoldDeadline.Load(); got != deadline {
				t.Errorf("empty flush reused old emission: deadline = %d, want %d", got, deadline)
			}
			clock.Add(4*rfc9687SendHold/10 - time.Nanosecond)
			if s.State() != fsm.StateEstablished {
				t.Fatal("genuine emission did not extend session to exact new deadline")
			}
			clock.Add(time.Nanosecond)
			if s.State() != fsm.StateIdle || s.Conn() != nil {
				t.Fatal("session did not expire at the exact restarted deadline")
			}
			if !conn.closed {
				t.Fatal("restarted SendHold expiry did not close the transport")
			}
		})
	}
}
