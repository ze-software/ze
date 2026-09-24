package reactor

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func flowForwardParts(t *testing.T, bodies [][]byte) (announced, withdrawn []byte) {
	t.Helper()
	for _, body := range bodies {
		wu := wireu.NewWireUpdate(body, 0)
		reach, err := wu.MPReach()
		if err != nil {
			t.Fatal(err)
		}
		if reach != nil {
			if len(reach.NextHopBytes()) != 0 {
				t.Fatal("FlowSpec replay created a next hop")
			}
			announced = append(announced, reach.NLRIBytes()...)
		}
		unreach, err := wu.MPUnreach()
		if err != nil {
			t.Fatal(err)
		}
		if unreach != nil {
			withdrawn = append(withdrawn, unreach.WithdrawnBytes()...)
		}
	}
	return announced, withdrawn
}

// A raw RS fast-path request must defer even when no RPKI validator is active.
// Cached forwarding and retained-route recovery both pass the mandatory gate,
// including withdrawal on loss and re-advertisement without a new UPDATE.
func TestFlowSpecForwardingRequiresAuthorizationAcrossRails(t *testing.T) {
	for _, rail := range []string{"cached", "batch"} {
		t.Run(rail, func(t *testing.T) {
			api, src, dst, capture := validationForwardFixture(t, false)
			fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
			for _, peer := range []*Peer{src, dst} {
				peer.negotiated.Store(&NegotiatedCapabilities{families: map[family.Family]bool{fam: true}})
				peer.refreshForwardFacts()
			}
			raw := []byte{5, 1, 24, 10, 0, 0}
			actions := []byte{0xc0, 16, 8, 0x80, 6, 0, 0, 0, 0, 0, 0}
			attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9}
			attrs = append(attrs, actions...)
			var mp [64]byte
			n := writeMPReach(mp[:], 0, fam, nil, raw)
			attrs = append(attrs, mp[:n]...)
			payload := []byte{0, 0}
			payload = binary.BigEndian.AppendUint16(payload, uint16(len(attrs)))
			payload = append(payload, attrs...)
			update, id := newLeakTestUpdate(t, api.r.recentUpdates, payload, src.recvCtxID)
			update.WireUpdate.SetSourceID(src.SourceID())
			ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil)
			t.Cleanup(func() { ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil) })
			if skipped, delivered := reactorForwardRS(api.r, update, id, src.Settings().Address, src); len(skipped) != 0 || delivered != 0 {
				t.Fatalf("raw fast path bypassed authorization: %v/%d", skipped, delivered)
			}
			if rail == "batch" {
				if err := api.ForwardUpdatesDirect([]uint64{id}, []netip.AddrPort{dst.Settings().PeerKey()}, "test-plugin", plugin.OperatorSender()); err != nil {
					t.Fatal(err)
				}
			} else {
				sel, err := selector.Parse(dst.Settings().Address.String())
				if err != nil {
					t.Fatal(err)
				}
				if err := api.ForwardUpdate(sel, id, "test-plugin", plugin.OperatorSender()); err != nil {
					t.Fatal(err)
				}
			}
			announced, withdrawn := flowForwardParts(t, validationBodies(t, api, capture))
			if len(announced) != 0 || !bytes.Equal(withdrawn, raw) {
				t.Fatalf("missing authorization = add %x del %x", announced, withdrawn)
			}
			key := ribevents.ValidationRoute{Peer: src.Settings().Address, Family: fam, NLRI: string(raw)}
			eligible, present := true, true
			generation := id
			ribevents.RegisterFlowSpecLookup(func(got ribevents.ValidationRoute, msgID uint64) bool {
				return got == key && eligible && (msgID == 0 || msgID == generation)
			}, func(got ribevents.ValidationRoute) bool { return got == key && present }, nil, nil)
			route := rpc.StoredRoute{SourcePeer: src.Settings().Address.String(), Family: fam.String(),
				NLRIHex: hex.EncodeToString(raw), AttrHex: hex.EncodeToString(attrs), MsgID: id, NLRIFraming: rpc.NLRIFramingPrefixOnly}
			relay := func() [][]byte {
				t.Helper()
				if err := api.RelayStoredRoute(dst.Settings().Address, []rpc.StoredRoute{route}, plugin.OperatorSender()); err != nil {
					t.Fatal(err)
				}
				return validationBodies(t, api, capture)
			}
			valid := relay()
			announced, withdrawn = flowForwardParts(t, valid)
			if !bytes.Equal(announced, raw) || len(withdrawn) != 0 || len(valid) != 1 || !bytes.Contains(valid[0], actions) {
				t.Fatalf("authorized retained actions not exported: %x", valid)
			}
			eligible = false
			announced, withdrawn = flowForwardParts(t, relay())
			if len(announced) != 0 || !bytes.Equal(withdrawn, raw) {
				t.Fatalf("loss of authorization = add %x del %x", announced, withdrawn)
			}
			if !validationRetainsPath(key.Peer, fam, 0, raw) {
				t.Fatal("infeasible retained route lost ADD-PATH ownership")
			}
			eligible = true
			announced, withdrawn = flowForwardParts(t, relay())
			if !bytes.Equal(announced, raw) || len(withdrawn) != 0 {
				t.Fatalf("recovered route = add %x del %x", announced, withdrawn)
			}
			generation++
			if stale := relay(); len(stale) != 0 {
				t.Fatalf("obsolete generation exported over replacement: %x", stale)
			}
			present = false
			if validationRetainsPath(key.Peer, fam, 0, raw) {
				t.Fatal("removed route retained ADD-PATH ownership")
			}
		})
	}
}

// Native FlowSpec length fields count octets, not prefix bits. Both forwarding
// encodings must preserve a long rule, reuse the same path identifier, and keep
// a retained path's identity when its withdrawal cache entry is evicted.
func TestFlowSpecAddPathNativeFraming(t *testing.T) {
	_, src, dst, _ := validationForwardFixture(t, false)
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	sourceCtx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{fam: true})
	sourceCtxID, err := bgpctx.Registry.Register(sourceCtx)
	if err != nil {
		t.Fatal(err)
	}
	dst.sendCtx.Store(sourceCtx)
	t.Cleanup(func() { fwdPathIDs.releaseSource(src.SourceID()) })

	short := []byte{5, 1, 24, 10, 0, 0}
	components := []byte{1, 24, 10, 1, 0, 5}
	for range 125 {
		components = append(components, 1, 80)
	}
	components[len(components)-2] |= 0x80
	long := []byte{0xf0 | byte(len(components)>>8), byte(len(components))}
	long = append(long, components...)
	native := append(bytes.Clone(short), long...)
	framed := binary.BigEndian.AppendUint32(nil, 7)
	framed = append(framed, short...)
	framed = binary.BigEndian.AppendUint32(framed, 7)
	framed = append(framed, long...)
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9}
	var mp [512]byte
	n := writeMPReach(mp[:], 0, fam, nil, framed)
	body := buildUpdatePayload(append(attrs, mp[:n]...), nil)
	wu := wireu.NewWireUpdate(body, sourceCtxID)
	wu.SetSourceID(src.SourceID())
	original := bytes.Clone(body)
	var advertised []byte

	for _, destination := range []struct {
		name          string
		asn4, addPath bool
	}{
		{"same-context", true, true},
		{"cross-context", false, true},
		{"without-add-path", false, false},
	} {
		t.Run(destination.name, func(t *testing.T) {
			destCtx := bgpctx.EncodingContextWithAddPath(destination.asn4, map[family.Family]bool{fam: destination.addPath})
			destCtxID, err := bgpctx.Registry.Register(destCtx)
			if err != nil {
				t.Fatal(err)
			}
			dst.sendCtx.Store(destCtx)
			var parsed fwdParseCache
			result, ok := buildFwdBody(wu, message.MaxMsgLen, destCtxID, dst, dst.Settings().Address, &parsed)
			if !ok {
				t.Fatal("native FlowSpec forward was refused")
			}
			defer ReturnReadBuffer(result.transcodeBuf)
			bodies := result.rawBodies
			for _, update := range result.updates {
				packet := message.PackTo(update, nil)
				bodies = append(bodies, packet[message.HeaderLen:])
			}
			got, withdrawn := flowForwardParts(t, bodies)
			if len(withdrawn) != 0 {
				t.Fatalf("announcement became a withdrawal: %x", withdrawn)
			}
			var rules []byte
			var identifiers []uint32
			_, err = nlrisplit.SplitFlowSpec(got, destination.addPath, func(raw []byte) {
				if destination.addPath {
					identifiers = append(identifiers, binary.BigEndian.Uint32(raw[:4]))
					raw = raw[4:]
				}
				rules = append(rules, raw...)
			})
			if err != nil || !bytes.Equal(rules, native) {
				t.Fatalf("forward changed native rule: %x, error %v", rules, err)
			}
			if destination.addPath {
				if len(identifiers) != 2 || identifiers[0] == identifiers[1] {
					t.Fatalf("distinct rules share an outgoing identity: %v", identifiers)
				}
				if advertised == nil {
					advertised = bytes.Clone(got)
				} else if !bytes.Equal(advertised, got) {
					t.Fatalf("cross-context forward changed path identities: %x -> %x", advertised, got)
				}
			}
		})
	}
	if !bytes.Equal(body, original) {
		t.Fatal("forwarding modified the received UPDATE")
	}

	// The equivalent extended encoding of the short rule must withdraw the
	// identifier assigned to its short encoding, not mint a second path.
	withdrawn := binary.BigEndian.AppendUint32(nil, 7)
	withdrawn = append(withdrawn, 0xf0, 5)
	withdrawn = append(withdrawn, short[1:]...)
	var memo fwdPathIDMemo
	memo.source = src.SourceID()
	wire := bytes.Clone(withdrawn)
	if err := fwdPatchPathIDs(wire, fam, &memo, true); err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint32(wire[:4]) != binary.BigEndian.Uint32(advertised[:4]) {
		t.Fatal("equivalent native framing changed withdrawal identity")
	}

	present := true
	ribevents.RegisterFlowSpecLookup(func(ribevents.ValidationRoute, uint64) bool { return false }, func(key ribevents.ValidationRoute) bool {
		return present && key.Peer == src.Settings().Address && key.Family == fam &&
			key.PathID == 7 && key.NLRI == string(short)
	}, nil, nil)
	t.Cleanup(func() { ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil) })
	if err := fwdReleaseSection(src.SourceID(), src.Settings().Address, fam, withdrawn, nil); err != nil {
		t.Fatal(err)
	}
	wire = bytes.Clone(withdrawn)
	if err := fwdPatchPathIDs(wire, fam, &memo, true); err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint32(wire[:4]) != binary.BigEndian.Uint32(advertised[:4]) {
		t.Fatal("retained FlowSpec path lost identity on cache eviction")
	}
	present = false
	if err := fwdReleaseSection(src.SourceID(), src.Settings().Address, fam, withdrawn, nil); err != nil {
		t.Fatal(err)
	}
	wire = bytes.Clone(withdrawn)
	if err := fwdPatchPathIDs(wire, fam, &memo, true); err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint32(wire[:4]) == binary.BigEndian.Uint32(advertised[:4]) {
		t.Fatal("removed FlowSpec path retained an obsolete identifier")
	}
}
