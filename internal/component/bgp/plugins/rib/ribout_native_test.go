// Design: docs/architecture/plugin/rib-storage-design.md -- native sent inventory.
package rib

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"strings"
	"testing"

	bgp "github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSentNativeInventoryRoundTrip drives both sent entry points, then replay,
// show and withdrawal through the actual registered family splitters.
func TestSentNativeInventoryRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		fam  family.Family
		raw  []byte
		cidr bool
	}{
		{"ipv4", family.IPv4Unicast, []byte{24, 192, 0, 2}, true},
		{"ipv6", family.IPv6Unicast, []byte{32, 0x20, 1, 0x0d, 0xb8}, true},
		{"evpn", family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}, nativeEVPNRoute(), false},
		{"mvpn", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMVPN}, []byte{1, 12, 0, 0, 0, 0, 0, 0, 0, 1, 192, 0, 2, 1}, false},
		{"rtc", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIRTC}, []byte{96, 0, 0, 0xfd, 0xe8, 0, 2, 0xfd, 0xe8, 0, 0, 0, 1}, false},
		{"flowspec", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}, []byte{5, 1, 24, 192, 0, 2}, false},
		{"labeled", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}, []byte{48, 0, 6, 0x41, 192, 0, 2}, false},
	}
	for _, tc := range cases {
		for _, structured := range []bool{false, true} {
			for _, addPath := range []bool{false, true} {
				name := tc.name + "/json"
				if structured {
					name = tc.name + "/structured"
				}
				if addPath {
					name += "/addpath-zero"
				}
				t.Run(name, func(t *testing.T) {
					if nlrisplit.Get(tc.fam) == nil {
						t.Fatal("real family splitter is not registered")
					}
					r := newTestRIBManager(t)
					peer := netip.MustParseAddr("192.0.2.20")
					raw := bytes.Clone(tc.raw)
					if addPath {
						raw = append([]byte{0, 0, 0, 0}, raw...)
					}
					attrs := nativeSentAttrs()
					if structured {
						attrs = feedNativeSent(t, r, peer, tc.fam, raw, attrs, addPath, false)
					} else {
						r.handleSent(nativeSentEvent(t, peer, tc.fam, raw, attrs, addPath, false))
					}
					key, ok := ribOutRouteKey(tc.fam, raw, addPath)
					if !ok {
						t.Fatal("native key rejected")
					}
					entries := r.ribOut[peer][tc.fam]
					if len(entries) != 1 {
						t.Fatalf("stored %d entries, want one", len(entries))
					}
					entry, ok := entries[key]
					if !ok {
						t.Fatal("stored key differs from helper key")
					}
					if entry.AddPath != addPath {
						t.Fatalf("ADD-PATH presence = %v", entry.AddPath)
					}
					if tc.cidr {
						if key.Native != "" || entry.NativeNLRI != "" {
							t.Fatal("CIDR path allocated native storage")
						}
					} else if key.Native == "" || entry.NativeNLRI != string(raw) {
						t.Fatal("opaque key or exact advertised bytes missing")
					}
					if source := entry.SourcePeer; source != "192.0.2.10" {
						t.Fatalf("source = %q", source)
					}
					groups := r.collectGroupedRibOutRoutesForFamily(peer, tc.fam)
					if len(groups) != 1 {
						t.Fatalf("replay groups = %d", len(groups))
					}
					commands := formatCursorCommands(&groups[0], nil)
					if len(commands) != 1 {
						t.Fatalf("replay commands = %d", len(commands))
					}
					if !strings.Contains(commands[0], "attr set "+hex.EncodeToString(attrs)) {
						t.Fatalf("attributes changed: %s", commands[0])
					}
					if !strings.HasSuffix(commands[0], " add "+hex.EncodeToString(raw)) {
						t.Fatalf("NLRI changed: %s", commands[0])
					}
					if strings.Contains(commands[0], " addpath") != addPath {
						t.Fatalf("ADD-PATH flag lost: %s", commands[0])
					}
					routes := r.collectRibOutRoutes(peer, tc.fam)
					if len(routes) != 1 {
						t.Fatalf("refresh routes = %d", len(routes))
					}
					if got := bgp.FormatAnnounceCommand(routes[0]); got != commands[0] {
						t.Fatalf("refresh and replay disagree:\n%s\n%s", got, commands[0])
					}
					row := serializeRouteItem(RouteItem{Family: tc.fam, Prefix: routes[0].Prefix, OutRoute: routes[0]})
					if !tc.cidr && row["raw-nlri"] != hex.EncodeToString(raw) {
						t.Fatalf("opaque show row = %#v", row)
					}
					if row["prefix"] == "invalid Prefix" {
						t.Fatal("show contains invalid CIDR placeholder")
					}
					if addPath && row["path-id"] != uint32(0) {
						t.Fatalf("zero path ID missing from show: %#v", row)
					}
					if structured {
						feedNativeSent(t, r, peer, tc.fam, raw, nil, addPath, true)
					} else {
						r.handleSent(nativeSentEvent(t, peer, tc.fam, raw, nil, addPath, true))
					}
					if len(r.ribOut[peer][tc.fam]) != 0 {
						t.Fatal("withdrawal retained sent entry")
					}
					if r.ribOut[peer][tc.fam][key].SourcePeer != "" {
						t.Fatal("withdrawal retained source reference")
					}
				})
			}
		}
	}
}

// TestSentNativeIdentityRetainsLatestWire separates EVPN semantic identity from
// non-key forwarding bytes, with two ADD-PATH identifiers including zero.
func TestSentNativeIdentityRetainsLatestWire(t *testing.T) {
	r := newTestRIBManager(t)
	peer := netip.MustParseAddr("192.0.2.20")
	other := netip.MustParseAddr("192.0.2.21")
	fam := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	raw := append([]byte{0, 0, 0, 0}, nativeEVPNRoute()...)
	handle, err := pool.RibOut.Intern(nativeSentAttrs())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pool.RibOut.Release(handle) }()
	entry := ribOutEntry{AttrHandle: handle}
	r.storeSentEntries(peer, fam, raw, true, entry, "192.0.2.10", false, nil, 0)
	r.storeSentEntries(other, fam, raw, true, entry, "192.0.2.10", false, nil, 0)
	key, ok := ribOutRouteKey(fam, raw, true)
	if !ok {
		t.Fatal("key rejected")
	}
	updated := bytes.Clone(raw)
	updated[len(updated)-2] = 9 // Label is forwarding data, not identity.
	updatedKey, ok := ribOutRouteKey(fam, updated, true)
	if !ok || updatedKey != key {
		t.Fatal("label update changed semantic identity")
	}
	r.storeSentEntries(peer, fam, updated, true, entry, "192.0.2.10", false, nil, 0)
	second := bytes.Clone(updated)
	binary.BigEndian.PutUint32(second[:4], 7)
	r.storeSentEntries(peer, fam, second, true, entry, "192.0.2.10", false, nil, 0)
	if len(r.ribOut[peer][fam]) != 2 {
		t.Fatal("ADD-PATH identifiers collapsed")
	}
	if r.ribOut[peer][fam][key].NativeNLRI != string(updated) {
		t.Fatal("semantic replacement lost latest label")
	}
	groups := r.collectGroupedRibOutRoutesForFamily(peer, fam)
	if len(groups) != 2 {
		t.Fatal("replay grouped distinct path identifiers together")
	}
	seen := make(map[string]bool)
	for i := range groups {
		commands := formatCursorCommands(&groups[i], nil)
		if len(commands) != 1 {
			t.Fatal("single native route was not replayed")
		}
		seen[commands[0]] = true
	}
	for _, wire := range [][]byte{updated, second} {
		found := false
		for command := range seen {
			if strings.HasSuffix(command, " addpath add "+hex.EncodeToString(wire)) {
				found = true
			}
		}
		if !found {
			t.Fatalf("replay lost path or latest label: %x", wire)
		}
	}
	r.removeSentNLRIs(peer, fam, raw, true)
	if len(r.ribOut[peer][fam]) != 1 {
		t.Fatal("withdrawal removed another path identifier")
	}
	// A duplicate withdrawal must not consume the other destination's source ref.
	r.removeSentNLRIs(peer, fam, raw, true)
	if r.ribOut[other][fam][key].SourcePeer != "192.0.2.10" {
		t.Fatal("duplicate withdrawal released another destination's source")
	}
	r.removeSentNLRIs(other, fam, raw, true)
	if r.ribOut[other][fam][key].SourcePeer != "" {
		t.Fatal("final destination did not release source")
	}
	r.removeSentNLRIs(peer, fam, second, true)
}

// TestSentLabeledWithdrawalCompatibility uses the registered withdrawal framing,
// whose three-octet compatibility field need not carry a bottom-of-stack bit.
func TestSentLabeledWithdrawalCompatibility(t *testing.T) {
	r := newTestRIBManager(t)
	peer := netip.MustParseAddr("192.0.2.20")
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}
	r.storeSentEntries(peer, fam, []byte{32, 0, 6, 0x41, 10}, false, ribOutEntry{}, "192.0.2.10", false, nil, 0)
	r.removeSentNLRIs(peer, fam, []byte{32, 0x80, 0, 0, 10}, false)
	if len(r.ribOut[peer][fam]) != 0 {
		t.Fatal("compatibility withdrawal missed labeled identity")
	}
}

// TestSentCIDRKeyNoAllocation pins the CIDR hot path independently of the map
// allocation required when a new destination first advertises a family.
func TestSentCIDRKeyNoAllocation(t *testing.T) {
	raw := []byte{0, 0, 0, 0, 25, 192, 0, 2, 255}
	var key ribOutKey
	var ok bool
	allocations := testing.AllocsPerRun(1000, func() {
		key, ok = ribOutRouteKey(family.IPv4Unicast, raw, true)
	})
	if allocations != 0 {
		t.Fatalf("CIDR key allocated %v times", allocations)
	}
	if !ok {
		t.Fatal("CIDR key rejected")
	}
	if key.Prefix != netip.MustParsePrefix("192.0.2.128/25") {
		t.Fatalf("CIDR padding not normalized: %v", key)
	}
}

// TestSentRawReplayBatchesKeepAttrs ensures every independently dispatched batch
// retains unknown attributes and every CIDR prefix is encoded exactly once.
func TestSentRawReplayBatchesKeepAttrs(t *testing.T) {
	route := &Route{Family: family.IPv4Unicast, RawAttrs: hex.EncodeToString(nativeSentAttrs()), NextHop: "192.0.2.10"}
	group := replayGroup{Route: route, Family: route.Family}
	for i := range 1000 {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i >> 8), byte(i), 0}), 24)
		group.Prefixes = append(group.Prefixes, prefix.String())
	}
	commands := formatCursorCommands(&group, nil)
	if len(commands) < 2 {
		t.Fatal("large group was not split")
	}
	got := make(map[string]bool)
	for _, command := range commands {
		if !strings.HasPrefix(command, "update hex attr set "+route.RawAttrs) {
			t.Fatal("split batch lost immutable attributes")
		}
		_, entries, ok := strings.Cut(command, " add ")
		if !ok {
			t.Fatal("split batch has no NLRI action")
		}
		for raw := range strings.FieldsSeq(entries) {
			if got[raw] {
				t.Fatalf("prefix replayed twice: %s", raw)
			}
			got[raw] = true
		}
	}
	if len(got) != len(group.Prefixes) {
		t.Fatalf("replayed %d prefixes, want %d", len(got), len(group.Prefixes))
	}
	for _, prefix := range group.Prefixes {
		route.Prefix = prefix
		if !got[bgp.RouteNLRIHex(route)] {
			t.Fatalf("replay omitted %s", prefix)
		}
	}
}

// TestSentJSONTextKeepsZeroPathID covers the parsed JSON fallback when a producer
// supplies no raw NLRI projection but explicitly names ADD-PATH identifier zero.
func TestSentJSONTextKeepsZeroPathID(t *testing.T) {
	r := newTestRIBManager(t)
	peer := netip.MustParseAddr("192.0.2.20")
	event := nativeSentEvent(t, peer, family.IPv4Unicast, nil, nativeSentAttrs(), false, false)
	event.RawNLRI = nil
	event.FamilyOps[family.IPv4Unicast][0].NLRIs = []any{map[string]any{"prefix": "192.0.2.0/24", "path-id": float64(0)}}
	r.handleSent(event)
	key := ribOutKey{Prefix: netip.MustParsePrefix("192.0.2.0/24")}
	if !r.ribOut[peer][family.IPv4Unicast][key].AddPath {
		t.Fatal("parsed JSON lost explicit path ID zero")
	}
	groups := r.collectGroupedRibOutRoutes(peer)
	if len(groups) != 1 {
		t.Fatal("parsed JSON route absent from replay")
	}
	commands := formatCursorCommands(&groups[0], nil)
	if len(commands) != 1 {
		t.Fatal("parsed JSON route absent from commands")
	}
	if !strings.HasSuffix(commands[0], " addpath add 0000000018c00002") {
		t.Fatalf("zero identifier lost: %s", commands[0])
	}
	r.removeSentNLRIs(peer, family.IPv4Unicast, []byte{0, 0, 0, 0, 24, 192, 0, 2}, true)
}

// TestSentNativeReplayCarriesStaleMetadata observes both actual dispatch loops,
// rather than only checking the formatter's intermediate command strings.
func TestSentNativeReplayCarriesStaleMetadata(t *testing.T) {
	r := newTestRIBManager(t)
	peer := netip.MustParseAddr("192.0.2.20")
	fam := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
	raw := nativeEVPNRoute()
	r.handleSent(nativeSentEvent(t, peer, fam, raw, nativeSentAttrs(), false, false))
	key, ok := ribOutRouteKey(fam, raw, false)
	if !ok {
		t.Fatal("native key rejected")
	}
	entry := r.ribOut[peer][fam][key]
	entry.StaleLevel = 2
	r.ribOut[peer][fam][key] = entry
	groups := r.collectGroupedRibOutRoutes(peer)
	announcements := 0
	ready := 0
	r.updateHook = func(command string, meta map[string]any) {
		if command == "update cursor done" {
			return
		}
		announcements++
		if !strings.Contains(command, "update hex attr set ") {
			t.Fatalf("opaque replay used text: %s", command)
		}
		if meta["stale"] != uint8(2) {
			t.Fatalf("stale metadata = %#v", meta)
		}
	}
	r.dispatchHook = func(command string) {
		if strings.Contains(command, "plugin session ready") {
			ready++
		}
	}
	r.replayRoutesWithCursor(peer.String(), groups, 1)
	if announcements != 1 {
		t.Fatalf("peer-up replay sent %d announcements", announcements)
	}
	if ready != 1 {
		t.Fatalf("peer-up replay sent %d ready markers", ready)
	}
	if count := r.resendRoutesWithCursor(peer.String(), groups); count != 1 {
		t.Fatalf("resend count = %d", count)
	}
	if announcements != 2 {
		t.Fatalf("resend sent %d total announcements", announcements)
	}
	if ready != 1 {
		t.Fatal("resend emitted a peer-up ready marker")
	}
	r.removeSentNLRIs(peer, fam, raw, false)
}

// nativeEVPNRoute is an Ethernet Auto-Discovery NLRI with a label-bearing body.
func nativeEVPNRoute() []byte {
	raw := make([]byte, 27)
	raw[0], raw[1], raw[9], raw[26] = 1, 25, 1, 1
	return raw
}

// nativeSentAttrs includes an AS_SET and an unknown transitive attribute so the
// replay assertion cannot pass through a parsed-field attribute whitelist.
func nativeSentAttrs() []byte {
	return []byte{0x40, 1, 1, 0, 0x40, 2, 6, 1, 1, 0, 0, 0xfd, 0xe8, 0xc0, 250, 2, 0xbe, 0xef}
}

func nativeSentEvent(t *testing.T, peer netip.Addr, fam family.Family, raw, attrs []byte, addPath, withdraw bool) *Event {
	t.Helper()
	event := &Event{
		Peer:          mustMarshal(t, map[string]any{"remote": map[string]any{"address": peer.String(), "as": 65002}}),
		RawAttributes: hex.EncodeToString(attrs),
		AddPath:       map[family.Family]bool{fam: addPath},
		RouteMeta:     map[string]any{"source-peer": "192.0.2.10"},
	}
	if withdraw {
		event.RawWithdrawn = map[family.Family]string{fam: hex.EncodeToString(raw)}
	} else {
		event.RawNLRI = map[family.Family]string{fam: hex.EncodeToString(raw)}
		event.FamilyOps = map[family.Family][]FamilyOperation{fam: {{Action: routeaction.Add, NextHop: "192.0.2.10"}}}
	}
	return event
}

func feedNativeSent(t *testing.T, r *RIBManager, peer netip.Addr, fam family.Family, raw, attrs []byte, addPath, withdraw bool) []byte {
	t.Helper()
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{fam: addPath}))
	if err != nil {
		t.Fatal(err)
	}
	mp := []byte{byte(fam.AFI >> 8), byte(fam.AFI), byte(fam.SAFI)}
	code := attribute.AttrMPUnreachNLRI
	if !withdraw {
		code = attribute.AttrMPReachNLRI
		mp = append(mp, 4, 192, 0, 2, 10, 0)
	}
	mp = append(mp, raw...)
	attrs = appendAttr(bytes.Clone(attrs), byte(code), 0x80, mp)
	body := []byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}
	body = append(body, attrs...)
	wu := wireu.NewWireUpdate(body, ctxID)
	wireAttrs, err := wu.Attrs()
	if err != nil {
		t.Fatal(err)
	}
	r.handleSentStructured(&rpc.StructuredEvent{
		PeerAddress: peer.String(), SourcePeerStr: "192.0.2.10",
		RawMessage: &bgptypes.RawMessage{WireUpdate: wu, AttrsWire: wireAttrs},
	})
	return attrs
}
