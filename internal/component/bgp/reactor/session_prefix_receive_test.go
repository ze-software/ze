// Design: docs/architecture/behavior/fsm-established.md -- receive prefix limits.
// Related: session_prefix.go -- registered framing and installed route identity.
package reactor

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestReceivePrefixLimitCompatibilityWithdrawal passes complete UPDATE bodies
// through the established receive path. A changed Compatibility field must free
// a full installed slot, and offered counting must frame that withdrawal too.
// RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility field
// MUST be ignored.".
func TestReceivePrefixLimitCompatibilityWithdrawal(t *testing.T) {
	for _, fam := range []family.Family{
		{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel},
		{AFI: family.AFIIPv6, SAFI: family.SAFIMPLSLabel},
		{AFI: family.AFIIPv4, SAFI: family.SAFIVPN},
		{AFI: family.AFIIPv6, SAFI: family.SAFIVPN},
	} {
		for _, mode := range []PrefixCountMode{PrefixCountInstalled, PrefixCountOffered} {
			for _, addPath := range []bool{false, true} {
				name := fam.String() + "/" + mode.String() + "/base"
				if addPath {
					name = fam.String() + "/" + mode.String() + "/add-path-zero"
				}
				t.Run(name, func(t *testing.T) {
					for _, compatibility := range []uint32{0x800000, 0, 0xffffff, 0x123456} {
						s := prefixReceiveSession(t, fam, mode, 1, addPath)
						announcement := prefixReceiveNLRI(fam, addPath, 0, 1, 10, 0x001001)
						withdrawal := prefixReceiveNLRI(fam, addPath, 0, 1, 10, compatibility)
						prefixReceive(t, s, prefixReceiveBody(fam, announcement, nil), true)
						prefixReceiveCount(t, s, fam, 1)
						prefixReceive(t, s, prefixReceiveBody(fam, nil, withdrawal), true)
						prefixReceiveCount(t, s, fam, 0)
						prefixReceive(t, s, prefixReceiveBody(fam,
							prefixReceiveNLRI(fam, addPath, 0, 1, 11, 0x002001), nil), true)
						prefixReceiveCount(t, s, fam, 1)
					}
				})
			}
		}
	}
}

// TestReceivePrefixLimitLabelChurn keeps one logical VPN route at its limit
// while labels change, then checks independent RD and ADD-PATH identities.
// RFC 8277 Section 2.5 replaces the binding when the prefix and Path Identifier
// match, not when their labels match.
func TestReceivePrefixLimitLabelChurn(t *testing.T) {
	for _, afi := range []family.AFI{family.AFIIPv4, family.AFIIPv6} {
		fam := family.Family{AFI: afi, SAFI: family.SAFIVPN}
		t.Run(fam.String(), func(t *testing.T) {
			s := prefixReceiveSession(t, fam, PrefixCountInstalled, 3, true)
			for _, route := range []struct{ path, rd uint32 }{{0, 1}, {17, 1}, {0, 2}} {
				prefixReceive(t, s, prefixReceiveBody(fam,
					prefixReceiveNLRI(fam, true, route.path, route.rd, 10, 0x001001), nil), true)
			}
			prefixReceiveCount(t, s, fam, 3)
			for _, label := range []uint32{0x002001, 0x003001, 0x004001} {
				prefixReceive(t, s, prefixReceiveBody(fam,
					prefixReceiveNLRI(fam, true, 0, 1, 10, label), nil), true)
				prefixReceiveCount(t, s, fam, 3)
			}
			prefixReceive(t, s, prefixReceiveBody(fam, nil,
				prefixReceiveNLRI(fam, true, 99, 1, 10, 0)), true)
			prefixReceive(t, s, prefixReceiveBody(fam, nil,
				prefixReceiveNLRI(fam, true, 0, 99, 10, 0)), true)
			prefixReceiveCount(t, s, fam, 3)
			for _, route := range []struct{ path, rd uint32 }{{0, 1}, {17, 1}, {0, 2}} {
				prefixReceive(t, s, prefixReceiveBody(fam, nil,
					prefixReceiveNLRI(fam, true, route.path, route.rd, 10, 0x123456)), true)
				prefixReceiveCount(t, s, fam, 2)
				// Repeating the same withdrawal cannot consume the other RD or path.
				prefixReceive(t, s, prefixReceiveBody(fam, nil,
					prefixReceiveNLRI(fam, true, route.path, route.rd, 10, 0xffffff)), true)
				prefixReceiveCount(t, s, fam, 2)
				prefixReceive(t, s, prefixReceiveBody(fam,
					prefixReceiveNLRI(fam, true, route.path, route.rd, 10, 0x005001), nil), true)
				prefixReceiveCount(t, s, fam, 3)
			}
		})
	}
}

// TestReceivePrefixLimitFamilyIsolation announces byte-identical VPN NLRIs in
// both AFIs, then withdraws one. The other family's occupied slot must survive.
func TestReceivePrefixLimitFamilyIsolation(t *testing.T) {
	v4 := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}
	v6 := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}
	s := prefixReceiveSession(t, v4, PrefixCountInstalled, 1, true, v6)
	for _, fam := range []family.Family{v4, v6} {
		prefixReceive(t, s, prefixReceiveBody(fam,
			prefixReceiveNLRI(fam, true, 0, 1, 10, 0x001001), nil), true)
		prefixReceiveCount(t, s, fam, 1)
	}
	prefixReceive(t, s, prefixReceiveBody(v4, nil,
		prefixReceiveNLRI(v4, true, 0, 1, 10, 0x123456)), true)
	prefixReceiveCount(t, s, v4, 0)
	prefixReceiveCount(t, s, v6, 1)
	prefixReceive(t, s, prefixReceiveBody(v6,
		prefixReceiveNLRI(v6, true, 0, 1, 11, 0x001001), nil), false)
	prefixReceiveCount(t, s, v6, 1)
	prefixReceive(t, s, prefixReceiveBody(v6, nil,
		prefixReceiveNLRI(v6, true, 0, 1, 10, 0)), true)
	prefixReceiveCount(t, s, v6, 0)
}

// TestReceivePrefixLimitSemanticRollback rejects a whole mixed UPDATE after
// withdrawing and replacing a route under different label encodings. The saved
// inventory must restore both original routes, not reused key scratch bytes.
func TestReceivePrefixLimitSemanticRollback(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}
	s := prefixReceiveSession(t, fam, PrefixCountInstalled, 2, true)
	first := prefixReceiveNLRI(fam, true, 0, 1, 10, 0x001001)
	second := prefixReceiveNLRI(fam, true, 17, 2, 10, 0x001001)
	prefixReceive(t, s, prefixReceiveBody(fam, append(first, second...), nil), true)
	prefixReceiveCount(t, s, fam, 2)
	announcements := prefixReceiveNLRI(fam, true, 0, 1, 10, 0x002001)
	announcements = append(announcements, prefixReceiveNLRI(fam, true, 0, 3, 11, 0x003001)...)
	announcements = append(announcements, prefixReceiveNLRI(fam, true, 0, 4, 12, 0x004001)...)
	prefixReceive(t, s, prefixReceiveBody(fam, announcements,
		prefixReceiveNLRI(fam, true, 0, 1, 10, 0x123456)), false)
	prefixReceiveCount(t, s, fam, 2)
	for _, rd := range []uint32{3, 4} {
		prefixReceive(t, s, prefixReceiveBody(fam, nil,
			prefixReceiveNLRI(fam, true, 0, rd, byte(rd+8), 0)), true)
		prefixReceiveCount(t, s, fam, 2)
	}
	prefixReceive(t, s, prefixReceiveBody(fam, nil,
		prefixReceiveNLRI(fam, true, 0, 1, 10, 0)), true)
	prefixReceiveCount(t, s, fam, 1)
	prefixReceive(t, s, prefixReceiveBody(fam, nil,
		prefixReceiveNLRI(fam, true, 17, 2, 10, 0xffffff)), true)
	prefixReceiveCount(t, s, fam, 0)
}

// TestReceivePrefixLimitShortAddPathEntry rejects an incomplete Path Identifier
// at the generic inventory boundary without trusting a registered splitter to
// have checked it. The SR-Policy splitter currently visits 0800 as a two-octet
// entry even with ADD-PATH enabled; this test does not claim SR-Policy support.
func TestReceivePrefixLimitShortAddPathEntry(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFISRPolicy}
	s := prefixReceiveSession(t, fam, PrefixCountInstalled, 1, true)
	prefixReceive(t, s, prefixReceiveBody(fam, nil, []byte{8, 0}), true)
	prefixReceiveCount(t, s, fam, 0)
	if len(s.prefixCounts.sets[familyKey(fam)]) != 0 {
		t.Fatal("incomplete ADD-PATH entry entered the installed inventory")
	}
}

// TestInstalledPrefixIdentityReannounceAllocations checks the warmed inventory
// walk after a real receive publication. Reusing canonical scratch must not add
// an allocation for every label-bearing route on a table refresh.
func TestInstalledPrefixIdentityReannounceAllocations(t *testing.T) {
	for _, safi := range []family.SAFI{family.SAFIMPLSLabel, family.SAFIVPN} {
		for _, addPath := range []bool{false, true} {
			fam := family.Family{AFI: family.AFIIPv4, SAFI: safi}
			s := prefixReceiveSession(t, fam, PrefixCountInstalled, 2, addPath)
			body := prefixReceiveBody(fam, prefixReceiveNLRI(fam, addPath, 0, 1, 10, 0x001001), nil)
			prefixReceive(t, s, body, true)
			update := wireu.NewWireUpdate(body, s.recvCtxID)
			allocations := testing.AllocsPerRun(100, func() {
				notif, drop := s.checkPrefixLimits(update)
				if notif != nil {
					t.Fatal("reannouncement triggered teardown")
				}
				if drop {
					t.Fatal("reannouncement was dropped")
				}
			})
			if allocations != 0 {
				t.Fatalf("%s ADD-PATH=%v: reannouncement allocated %g times", fam, addPath, allocations)
			}
		}
	}
}

// prefixReceiveSession uses actual negotiated family and ADD-PATH receive
// contexts; only the already-established transport is supplied by the fixture.
func prefixReceiveSession(t *testing.T, fam family.Family, mode PrefixCountMode, maximum uint32, addPath bool, others ...family.Family) *Session {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65001, 0x01020301)
	settings.PrefixMaximum = make(map[string]uint32)
	settings.PrefixCount = make(map[string]PrefixCountMode)
	settings.PrefixTeardown = make(map[string]bool)
	caps := []capability.Capability{&capability.ASN4{ASN: settings.LocalAS}}
	var paths []capability.AddPathFamily
	for _, configured := range append([]family.Family{fam}, others...) {
		settings.PrefixMaximum[configured.String()] = maximum
		settings.PrefixCount[configured.String()] = mode
		settings.PrefixTeardown[configured.String()] = false
		caps = append(caps, &capability.Multiprotocol{AFI: configured.AFI, SAFI: configured.SAFI})
		if addPath {
			paths = append(paths, capability.AddPathFamily{
				AFI: configured.AFI, SAFI: configured.SAFI, Mode: capability.AddPathBoth,
			})
		}
	}
	if addPath {
		caps = append(caps, &capability.AddPath{Families: paths})
	}
	s := NewSession(settings)
	t.Cleanup(s.timers.StopAll)
	s.negotiated = capability.Negotiate(caps, caps,
		capability.PeerIdentity{LocalASN: settings.LocalAS, PeerASN: settings.PeerAS})
	ctxID, err := bgpctx.Registry.Register(bgpctx.FromNegotiatedRecv(s.negotiated))
	if err != nil {
		t.Fatal(err)
	}
	s.recvCtxID = ctxID
	for _, event := range []fsm.Event{
		fsm.EventManualStart, fsm.EventTCPConnectionConfirmed, fsm.EventBGPOpen, fsm.EventKeepaliveMsg,
	} {
		if err := s.fsm.Event(event); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

// prefixReceive checks publication, not just the local tally. Exact body equality
// also rejects an earlier receive stage silently turning announcements into withdrawals.
func prefixReceive(t *testing.T, s *Session, body []byte, accepted bool) {
	t.Helper()
	calls := 0
	s.onMessageReceived = func(_ netip.Addr, _ msgtype.MessageType, _ []byte,
		wu *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string, _ uint64) bool {
		calls++
		if !bytes.Equal(wu.Payload(), body) {
			t.Fatalf("receive changed UPDATE: got %x, want %x", wu.Payload(), body)
		}
		return false
	}
	header := message.Header{Type: msgtype.TypeUPDATE, Length: uint16(message.HeaderLen + len(body))}
	if err, kept := s.processMessage(&header, body, BufHandle{ID: noPoolBufID, Buf: body}); err != nil || kept {
		t.Fatalf("receive returned err=%v kept=%v", err, kept)
	}
	want := 0
	if accepted {
		want = 1
	}
	if calls != want {
		t.Fatalf("receive publications = %d, want %d; UPDATE %x", calls, want, body)
	}
	if s.State() != fsm.StateEstablished {
		t.Fatalf("session left Established: %v", s.State())
	}
}

func prefixReceiveCount(t *testing.T, s *Session, fam family.Family, want int64) {
	t.Helper()
	if got := s.prefixCounts.counts[familyKey(fam)]; got != want {
		t.Fatalf("%s prefix count = %d, want %d", fam, got, want)
	}
}

// prefixReceiveNLRI encodes RFC 8277 Sections 2.2/2.4: optional Path ID at
// [0:4], Length at [0], label/Compatibility at [1:4], then RD (VPN only)
// and one significant prefix octet. This includes the short SAFI-4 208000000a.
func prefixReceiveNLRI(fam family.Family, addPath bool, pathID, rd uint32, prefix byte, label uint32) []byte {
	var out []byte
	if addPath {
		out = binary.BigEndian.AppendUint32(out, pathID)
	}
	bits := byte(32)
	if fam.SAFI == family.SAFIVPN {
		bits += 64
	}
	out = append(out, bits, byte(label>>16), byte(label>>8), byte(label))
	if fam.SAFI == family.SAFIVPN {
		out = append(out, 0, 0, 0xfd, 0xe9)
		out = binary.BigEndian.AppendUint32(out, rd)
	}
	return append(out, prefix)
}

// prefixReceiveBody supplies the MP family-native next-hop shape and mandatory
// iBGP path attributes, so receive validation precedes the prefix-limit check.
func prefixReceiveBody(fam family.Family, announced, withdrawn []byte) []byte {
	var attrs []byte
	if len(withdrawn) > 0 {
		attrs = append(attrs, mpUnreachAttrs(fam, withdrawn)...)
	}
	if len(announced) > 0 {
		nextHop := []byte{192, 0, 2, 1}
		if fam.AFI == family.AFIIPv6 {
			nextHop = []byte{0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
		}
		if fam.SAFI == family.SAFIVPN {
			nextHop = append(make([]byte, 8), nextHop...)
		}
		mp := []byte{byte(fam.AFI >> 8), byte(fam.AFI), byte(fam.SAFI), byte(len(nextHop))}
		mp = append(mp, nextHop...)
		mp = append(mp, 0)
		mp = append(mp, announced...)
		attrs = append(attrs, 0x80, 14, byte(len(mp)))
		attrs = append(attrs, mp...)
		attrs = append(attrs, 0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 5, 4, 0, 0, 0, 100)
	}
	return makeUpdateBody(nil, attrs, nil)
}
