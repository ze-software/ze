package reactor

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"slices"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

func pathsLimitSession(t *testing.T, limits map[family.Family]uint16) (*Session, *recordingConn) {
	t.Helper()
	peer, conn := newAnnouncePeer(t, "192.0.2.2")
	s := peer.session
	local := []capability.Capability{&capability.ASN4{ASN: 65000}}
	remote := []capability.Capability{&capability.ASN4{ASN: 65001}}
	ap := &capability.AddPath{}
	pl := &capability.PathsLimit{}
	for fam, limit := range limits {
		mp := &capability.Multiprotocol{AFI: fam.AFI, SAFI: fam.SAFI}
		local = append(local, mp)
		remote = append(remote, mp)
		ap.Families = append(ap.Families, capability.AddPathFamily{AFI: fam.AFI, SAFI: fam.SAFI, Mode: capability.AddPathBoth})
		pl.Entries = append(pl.Entries, capability.PathsLimitEntry{AFI: fam.AFI, SAFI: fam.SAFI, Limit: limit})
	}
	local = append(local, ap)
	remote = append(remote, ap, pl)
	s.localOpen = &message.Open{HoldTime: 90}
	s.peerOpen = &message.Open{ASN4: 65001, HoldTime: 90}
	s.negotiateWith(local, remote)
	return s, conn
}

func pathsLimitNLRI(id uint32, prefix string) []byte {
	p := netip.MustParsePrefix(prefix).Masked()
	b := make([]byte, 5+(p.Bits()+7)/8)
	binary.BigEndian.PutUint32(b, id)
	b[4] = byte(p.Bits())
	copy(b[5:], p.Addr().AsSlice())
	return b
}

func pathsLimitUpdate(fam family.Family, withdraw bool, routes ...[]byte) *message.Update {
	data := bytes.Join(routes, nil)
	u := &message.Update{}
	if fam == family.IPv4Unicast {
		if withdraw {
			u.WithdrawnRoutes = data
		} else {
			u.NLRI = data
		}
		return u
	}
	code := byte(attribute.AttrMPUnreachNLRI)
	value := []byte{byte(fam.AFI >> 8), byte(fam.AFI), byte(fam.SAFI)}
	if !withdraw {
		code = byte(attribute.AttrMPReachNLRI)
		value = append(value, 16)
		value = append(value, netip.MustParseAddr("2001:db8::1").AsSlice()...)
		value = append(value, 0)
	}
	value = append(value, data...)
	u.PathAttributes = []byte{0x90, code, byte(len(value) >> 8), byte(len(value))}
	u.PathAttributes = append(u.PathAttributes, value...)
	return u
}

// pathsLimitReceived reads complete frames and each registered family from the
// socket's captured bytes, rather than inspecting the admission table.
func pathsLimitReceived(t *testing.T, frames []byte, fam family.Family, withdraw bool) []uint32 {
	t.Helper()
	var ids []uint32
	for len(frames) != 0 {
		if len(frames) < message.HeaderLen {
			t.Fatal("truncated BGP header")
		}
		n := int(binary.BigEndian.Uint16(frames[16:]))
		if n < message.HeaderLen || n > len(frames) {
			t.Fatalf("invalid BGP length %d", n)
		}
		body := frames[message.HeaderLen:n]
		sections, err := wire.ParseUpdateSections(body)
		if err != nil {
			t.Fatal(err)
		}
		data := sections.NLRI(body)
		if withdraw {
			data = sections.Withdrawn(body)
		}
		if fam != family.IPv4Unicast {
			data = nil
			iter := attribute.NewAttrIterator(sections.Attrs(body))
			for code, _, value, ok := iter.Next(); ok; code, _, value, ok = iter.Next() {
				want := attribute.AttrMPReachNLRI
				if withdraw {
					want = attribute.AttrMPUnreachNLRI
				}
				if code != want {
					continue
				}
				f, off, err := pathsLimitMP(value, !withdraw)
				if err != nil {
					t.Fatal(err)
				}
				if f == fam {
					data = value[off:]
				}
			}
		}
		split := nlrisplit.Get(fam)
		if withdraw {
			split = nlrisplit.GetWithdraw(fam)
		}
		if _, err := split(data, true, func(raw []byte) { ids = append(ids, binary.BigEndian.Uint32(raw)) }); err != nil {
			t.Fatal(err)
		}
		frames = frames[n:]
	}
	return ids
}

// New paths are capped across writes, while replacements and withdrawals keep
// working at capacity. A withheld batch must not turn into a false EOR.
//
// RFC requirement: DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-6 positive -- paths within the peer's limit reach the wire; replacements and reuse after withdrawal remain allowed across UPDATEs.
// RFC requirement: DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-6 negative -- excess paths for the same prefix never reach the wire, including a later all-withheld batch and a retry after an unknown withdrawal.
func TestPathsLimitSessionAcrossUpdates(t *testing.T) {
	for _, fam := range []family.Family{family.IPv4Unicast, family.IPv6Unicast} {
		t.Run(fam.String(), func(t *testing.T) {
			s, conn := pathsLimitSession(t, map[family.Family]uint16{fam: 2})
			prefix := "198.51.100.0/24"
			if fam == family.IPv6Unicast {
				prefix = "2001:db8:1::/48"
			}
			send := func(withdraw bool, ids ...uint32) {
				t.Helper()
				var routes [][]byte
				for _, id := range ids {
					routes = append(routes, pathsLimitNLRI(id, prefix))
				}
				if err := s.SendUpdate(pathsLimitUpdate(fam, withdraw, routes...)); err != nil {
					t.Fatal(err)
				}
			}
			send(false, 0, 1, 2)
			before := len(conn.written())
			send(false, 3)
			if len(conn.written()) != before {
				t.Fatal("over-limit batch reached wire (or became a false EOR)")
			}
			send(false, 1)
			send(true, 99) // Unknown withdrawal does not free a slot.
			send(false, 4)
			send(true, 0)
			send(false, 4)
			if got := pathsLimitReceived(t, conn.written(), fam, false); !slices.Equal(got, []uint32{0, 1, 1, 4}) {
				t.Fatalf("announced IDs = %v, want [0 1 1 4]", got)
			}
			if got := pathsLimitReceived(t, conn.written(), fam, true); !slices.Equal(got, []uint32{99, 0}) {
				t.Fatalf("withdrawn IDs = %v, want [99 0]", got)
			}
		})
	}
}

// Raw and re-encoded forwarding must share the same per-session budget.
//
// RFC requirement: DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-6 positive -- raw forwarding admits the first path and re-encoded forwarding admits a different prefix under the same session limit.
// RFC requirement: DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-6 negative -- re-encoded forwarding cannot exceed a prefix's limit after raw forwarding has consumed its slot.
func TestPathsLimitForwardWritersShareState(t *testing.T) {
	s, conn := pathsLimitSession(t, map[family.Family]uint16{family.IPv4Unicast: 1})
	first := pathsLimitUpdate(family.IPv4Unicast, false, pathsLimitNLRI(5, "198.51.100.0/24"))
	body := message.PackTo(first, nil)[message.HeaderLen:]
	original := slices.Clone(body)
	if err := s.sendRawUpdateBody(body); err != nil {
		t.Fatal(err)
	}
	s.writeMu.Lock()
	err := s.writeUpdatePreFiltered(pathsLimitUpdate(family.IPv4Unicast, false,
		pathsLimitNLRI(6, "198.51.100.0/24"), pathsLimitNLRI(7, "203.0.113.0/24")))
	if err == nil {
		err = s.flushWrites()
	}
	s.writeMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, original) {
		t.Fatal("shared forwarding input was mutated")
	}
	if got := pathsLimitReceived(t, conn.written(), family.IPv4Unicast, false); !slices.Equal(got, []uint32{5, 7}) {
		t.Fatalf("forwarded IDs = %v, want [5 7]", got)
	}
}

// Concurrent plugin writers must not admit two new paths against one slot.
func TestPathsLimitConcurrentWriters(t *testing.T) {
	s, conn := pathsLimitSession(t, map[family.Family]uint16{family.IPv4Unicast: 1})
	var wg sync.WaitGroup
	for id := range uint32(32) {
		wg.Go(func() {
			if err := s.SendUpdate(pathsLimitUpdate(family.IPv4Unicast, false, pathsLimitNLRI(id, "198.51.100.0/24"))); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if got := pathsLimitReceived(t, conn.written(), family.IPv4Unicast, false); len(got) != 1 {
		t.Fatalf("concurrent writers advertised %v, want exactly one path", got)
	}
}

// A malformed tail must not reserve capacity or release a path whose withdrawal
// never reached the peer. The next valid UPDATE observes the original state.
func TestPathsLimitMalformedUpdateRollsBack(t *testing.T) {
	s, conn := pathsLimitSession(t, map[family.Family]uint16{family.IPv4Unicast: 1})
	route := pathsLimitNLRI(0, "198.51.100.0/24")
	if err := s.SendUpdate(pathsLimitUpdate(family.IPv4Unicast, false, route)); err != nil {
		t.Fatal(err)
	}
	before := len(conn.written())
	bad := &message.Update{WithdrawnRoutes: route, NLRI: []byte{0, 0, 0, 1, 24, 198}}
	if err := s.SendUpdate(bad); err == nil {
		t.Fatal("malformed NLRI was accepted")
	}
	if len(conn.written()) != before {
		t.Fatal("partial malformed UPDATE reached wire")
	}
	if err := s.SendUpdate(pathsLimitUpdate(family.IPv4Unicast, false, pathsLimitNLRI(1, "198.51.100.0/24"))); err != nil {
		t.Fatal(err)
	}
	if len(conn.written()) != before {
		t.Fatal("failed withdrawal freed a slot")
	}

	bad = &message.Update{NLRI: append(pathsLimitNLRI(2, "203.0.113.0/24"), 0, 0, 0)}
	if err := s.SendUpdate(bad); err == nil {
		t.Fatal("malformed trailing identifier was accepted")
	}
	if err := s.SendUpdate(pathsLimitUpdate(family.IPv4Unicast, false, pathsLimitNLRI(3, "203.0.113.0/24"))); err != nil {
		t.Fatal(err)
	}
	if got := pathsLimitReceived(t, conn.written(), family.IPv4Unicast, false); !slices.Equal(got, []uint32{0, 3}) {
		t.Fatalf("announced IDs after rollback = %v, want [0 3]", got)
	}
}

// Capacity is per family and per connection. Explicit EORs still reach a peer
// whose route budget is full, and limit zero leaves ADD-PATH unrestricted.
func TestPathsLimitFamiliesEORAndNewSession(t *testing.T) {
	s, conn := pathsLimitSession(t, map[family.Family]uint16{family.IPv4Unicast: 1, family.IPv6Unicast: 0})
	for _, fam := range []family.Family{family.IPv4Unicast, family.IPv6Unicast} {
		prefix := "198.51.100.0/24"
		if fam == family.IPv6Unicast {
			prefix = "2001:db8:1::/48"
		}
		if err := s.SendUpdate(pathsLimitUpdate(fam, false, pathsLimitNLRI(1, prefix), pathsLimitNLRI(2, prefix))); err != nil {
			t.Fatal(err)
		}
		before := len(conn.written())
		eor := message.PackTo(message.BuildEOR(fam), nil)
		if err := s.SendUpdate(message.BuildEOR(fam)); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(conn.written()[before:], eor) {
			t.Fatal("EOR changed at capacity")
		}
	}
	if got := pathsLimitReceived(t, conn.written(), family.IPv4Unicast, false); !slices.Equal(got, []uint32{1}) {
		t.Fatalf("IPv4 IDs = %v", got)
	}
	if got := pathsLimitReceived(t, conn.written(), family.IPv6Unicast, false); !slices.Equal(got, []uint32{1, 2}) {
		t.Fatalf("unlimited IPv6 IDs = %v", got)
	}
	next, nextConn := pathsLimitSession(t, map[family.Family]uint16{family.IPv4Unicast: 1})
	if err := next.SendUpdate(pathsLimitUpdate(family.IPv4Unicast, false, pathsLimitNLRI(2, "198.51.100.0/24"))); err != nil {
		t.Fatal(err)
	}
	if got := pathsLimitReceived(t, nextConn.written(), family.IPv4Unicast, false); !slices.Equal(got, []uint32{2}) {
		t.Fatalf("new connection retained old budget: %v", got)
	}
}

// The one-route writer and its withdrawal share admission with batch UPDATEs.
func TestPathsLimitSingleRouteWithdrawal(t *testing.T) {
	s, conn := pathsLimitSession(t, map[family.Family]uint16{family.IPv4Unicast: 1})
	route := pathsLimitNLRI(0, "198.51.100.0/24")
	if err := s.SendUpdate(pathsLimitUpdate(family.IPv4Unicast, false, route)); err != nil {
		t.Fatal(err)
	}
	if err := s.sendWithdraw(netip.MustParsePrefix("198.51.100.0/24"), true); err != nil {
		t.Fatal(err)
	}
	if err := s.SendUpdate(pathsLimitUpdate(family.IPv4Unicast, false, pathsLimitNLRI(1, "198.51.100.0/24"))); err != nil {
		t.Fatal(err)
	}
	if got := pathsLimitReceived(t, conn.written(), family.IPv4Unicast, false); !slices.Equal(got, []uint32{0, 1}) {
		t.Fatalf("IDs = %v, want [0 1]", got)
	}
}

// Labels must not create new prefixes. VPN RDs, in contrast, separate prefixes.
func TestPathsLimitLabeledPrefixIdentity(t *testing.T) {
	for _, safi := range []family.SAFI{family.SAFIMPLSLabel, family.SAFIVPN} {
		fam := family.Family{AFI: family.AFIIPv4, SAFI: safi}
		t.Run(fam.String(), func(t *testing.T) {
			s, conn := pathsLimitSession(t, map[family.Family]uint16{fam: 1})
			labeled := func(id uint32, label byte, rd byte) []byte {
				raw := make([]byte, 4)
				binary.BigEndian.PutUint32(raw, id)
				bits := byte(48)
				if safi == family.SAFIVPN {
					bits += 64
				}
				raw = append(raw, bits, 0, label, 1)
				if safi == family.SAFIVPN {
					raw = append(raw, 0, 0, 0, 1, 0, 0, 0, rd)
				}
				return append(raw, 198, 51, 100)
			}
			for _, raw := range [][]byte{labeled(0, 1, 1), labeled(1, 2, 1), labeled(0, 3, 1), labeled(2, 4, 2)} {
				if err := s.SendUpdate(pathsLimitUpdate(fam, false, raw)); err != nil {
					t.Fatal(err)
				}
			}
			want := []uint32{0, 0}
			if safi == family.SAFIVPN {
				want = append(want, 2)
			}
			if got := pathsLimitReceived(t, conn.written(), fam, false); !slices.Equal(got, want) {
				t.Fatalf("IDs = %v, want %v", got, want)
			}
			if err := s.SendUpdate(pathsLimitUpdate(fam, true, labeled(0, 9, 1))); err != nil {
				t.Fatal(err)
			}
			if err := s.SendUpdate(pathsLimitUpdate(fam, false, labeled(3, 10, 1))); err != nil {
				t.Fatal(err)
			}
			want = append(want, 3)
			if got := pathsLimitReceived(t, conn.written(), fam, false); !slices.Equal(got, want) {
				t.Fatalf("label-changing withdrawal failed: %v", got)
			}
		})
	}
}

// Host bits outside a prefix cannot bypass the cap. A mixed UPDATE can replace
// a withdrawn path immediately, rather than waiting for a second write.
func TestPathsLimitPaddingAndMixedReplacement(t *testing.T) {
	s, conn := pathsLimitSession(t, map[family.Family]uint16{family.IPv4Unicast: 1})
	first := pathsLimitNLRI(1, "198.51.100.0/25")
	if err := s.SendUpdate(pathsLimitUpdate(family.IPv4Unicast, false, first)); err != nil {
		t.Fatal(err)
	}
	second := pathsLimitNLRI(2, "198.51.100.0/25")
	second[len(second)-1] |= 0x7f
	if err := s.SendUpdate(pathsLimitUpdate(family.IPv4Unicast, false, second)); err != nil {
		t.Fatal(err)
	}
	if err := s.SendUpdate(&message.Update{WithdrawnRoutes: first, NLRI: second}); err != nil {
		t.Fatal(err)
	}
	if got := pathsLimitReceived(t, conn.written(), family.IPv4Unicast, false); !slices.Equal(got, []uint32{1, 2}) {
		t.Fatalf("IDs = %v, want [1 2]", got)
	}
}

func pathsLimitAPIPeer(t *testing.T) (*Peer, *recordingConn) {
	t.Helper()
	session, conn := pathsLimitSession(t, map[family.Family]uint16{family.IPv4Unicast: 1})
	peer := NewPeer(session.settings)
	peer.session = session
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(NewNegotiatedCapabilities(session.negotiated))
	peer.setEncodingContexts(session.negotiated)
	t.Cleanup(peer.clearEncodingContexts)
	return peer, conn
}

// A withheld path must not enter the API's duplicate cache: the same command
// must be able to retry it after a withdrawal, without requiring changed attrs.
func TestPathsLimitAPIReattemptAfterWithdrawal(t *testing.T) {
	peer, conn := pathsLimitAPIPeer(t)
	adapter := groupUpdatesReactor([]*Peer{peer}, false)
	batch := func(ids ...uint32) bgptypes.NLRIBatch {
		b := adjOutBatch("198.51.100.0/24", "192.0.2.1")
		b.NLRIs = nil
		for _, id := range ids {
			b.NLRIs = append(b.NLRIs, nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), id))
		}
		return b
	}
	if err := adapter.AnnounceNLRIBatch(selector.All(), batch(1, 2), plugin.OperatorSender()); err != nil {
		t.Fatal(err)
	}
	if err := adapter.AnnounceNLRIBatch(selector.All(), batch(2), plugin.OperatorSender()); err != nil {
		t.Fatal(err)
	}
	if err := adapter.WithdrawNLRIBatch(selector.All(), batch(1), plugin.OperatorSender()); err != nil {
		t.Fatal(err)
	}
	if err := adapter.AnnounceNLRIBatch(selector.All(), batch(2), plugin.OperatorSender()); err != nil {
		t.Fatal(err)
	}
	if got := pathsLimitReceived(t, conn.written(), family.IPv4Unicast, false); !slices.Equal(got, []uint32{1, 2}) {
		t.Fatalf("API announced IDs = %v, want [1 2]", got)
	}
}

// Named commits share the session budget and report actual sends. Withdrawals
// in the same commit free capacity before new paths are offered.
func TestPathsLimitCommitsAndResultCounts(t *testing.T) {
	peer, conn := pathsLimitAPIPeer(t)
	adapter := groupUpdatesReactor([]*Peer{peer}, false)
	route := func(id uint32) *rib.Route {
		n := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("198.51.100.0/24"), id)
		return rib.NewRouteWithASPath(n, netip.MustParseAddr("192.0.2.1"), nil, nil)
	}
	first := adapter.commitToPeer(peer, []*rib.Route{route(1), route(2)}, nil, []family.Family{family.IPv4Unicast}, false)
	if first.RoutesAnnounced != 1 || !slices.Contains(first.Reasons, bgptypes.CommitReasonRoutesDropped) {
		t.Fatalf("first commit result = %+v, want one announced route and dropped reason", first)
	}
	second := adapter.commitToPeer(peer, []*rib.Route{route(2)}, nil, []family.Family{family.IPv4Unicast}, false)
	if second.RoutesAnnounced != 0 || second.UpdatesSent != 0 {
		t.Fatalf("withheld commit reported sends: %+v", second)
	}
	third := adapter.commitToPeer(peer, []*rib.Route{route(2)}, []nlri.NLRI{route(1).NLRI()}, []family.Family{family.IPv4Unicast}, false)
	if third.RoutesAnnounced != 1 || third.RoutesWithdrawn != 1 {
		t.Fatalf("replacement commit result = %+v", third)
	}
	if got := pathsLimitReceived(t, conn.written(), family.IPv4Unicast, false); !slices.Equal(got, []uint32{1, 2}) {
		t.Fatalf("commit wire IDs = %v", got)
	}
}

// RFC 8277 withdrawals carry one ignored compatibility field, even when the
// announcement had multiple labels. Neither its value nor its S bit is a key.
func TestPathsLimitLabeledCompatibilityWithdrawal(t *testing.T) {
	for _, safi := range []family.SAFI{family.SAFIMPLSLabel, family.SAFIVPN} {
		fam := family.Family{AFI: family.AFIIPv4, SAFI: safi}
		t.Run(fam.String(), func(t *testing.T) {
			s, conn := pathsLimitSession(t, map[family.Family]uint16{fam: 1})
			route := func(id uint32, labels []byte) []byte {
				raw := make([]byte, 5)
				binary.BigEndian.PutUint32(raw, id)
				raw[4] = byte(len(labels)*8 + 8)
				raw = append(raw, labels...)
				if safi == family.SAFIVPN {
					raw[4] += 64
					raw = append(raw, 0, 0, 0, 1, 0, 0, 0, 2)
				}
				return append(raw, 10)
			}
			for i, compatibility := range [][]byte{{0x80, 0, 0}, {0xde, 0xad, 0xbe}} {
				id := uint32(i + 1)
				if err := s.SendUpdate(pathsLimitUpdate(fam, false, route(id, []byte{0, 1, 0, 0, 2, 1}))); err != nil {
					t.Fatal(err)
				}
				if err := s.SendUpdate(pathsLimitUpdate(fam, true, route(id, compatibility))); err != nil {
					t.Fatal(err)
				}
			}
			if got := pathsLimitReceived(t, conn.written(), fam, false); !slices.Equal(got, []uint32{1, 2}) {
				t.Fatalf("announced IDs = %v, want [1 2]", got)
			}
			if got := pathsLimitReceived(t, conn.written(), fam, true); !slices.Equal(got, []uint32{1, 2}) {
				t.Fatalf("withdrawn IDs = %v, want [1 2]", got)
			}
		})
	}
}

func TestPathsLimitEVPNForwardingFieldsAreNotPrefixKeys(t *testing.T) {
	fam := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	s, conn := pathsLimitSession(t, map[family.Family]uint16{fam: 1})
	route := func(id uint32, rd, forwarding byte) []byte {
		raw := make([]byte, 4+2+34)
		binary.BigEndian.PutUint32(raw, id)
		raw[4], raw[5] = 5, 34
		data := raw[6:]
		data[7], data[17], data[21] = rd, forwarding, 10
		data[22] = 24
		copy(data[23:27], []byte{198, 51, 100, 0})
		copy(data[27:31], []byte{192, 0, 2, forwarding})
		data[33] = forwarding
		return raw
	}
	for _, raw := range [][]byte{route(1, 1, 1), route(2, 1, 2), route(1, 1, 3)} {
		if err := s.SendUpdate(pathsLimitUpdate(fam, false, raw)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SendUpdate(pathsLimitUpdate(fam, true, route(1, 1, 99))); err != nil {
		t.Fatal(err)
	}
	if err := s.SendUpdate(pathsLimitUpdate(fam, false, route(2, 1, 2), route(3, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if got := pathsLimitReceived(t, conn.written(), fam, false); !slices.Equal(got, []uint32{1, 1, 2, 3}) {
		t.Fatalf("announced IDs = %v, want [1 1 2 3]", got)
	}
	if got := pathsLimitReceived(t, conn.written(), fam, true); !slices.Equal(got, []uint32{1}) {
		t.Fatalf("withdrawn IDs = %v, want [1]", got)
	}
}
