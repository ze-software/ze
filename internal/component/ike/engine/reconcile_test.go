package engine

import (
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

func testIPsecConfig(peers ...ipsec.SiteToSitePeer) *ipsec.IPsecConfig {
	cfg := &ipsec.IPsecConfig{
		ESPGroups: map[string]ipsec.ESPGroup{
			"test-esp": testESPGroup(),
		},
		IKEGroups: map[string]ipsec.IKEGroup{
			"test-ike": testIKEGroup(),
		},
		Peers: make(map[string]ipsec.SiteToSitePeer),
	}
	for i := range peers {
		cfg.Peers[peers[i].Name] = peers[i]
	}
	return cfg
}

func TestReconcilePeersAdded(t *testing.T) {
	active := make(map[string]*PeerSession)
	table := NewSATable()
	log := slog.Default()

	peer := testPeer()
	cfg := testIPsecConfig(peer)

	reconcilePeers(cfg, nil, active, table, nil, nil, nil, log)

	if len(active) != 1 {
		t.Fatalf("expected 1 active peer, got %d", len(active))
	}
	if _, ok := active["test-peer"]; !ok {
		t.Fatal("expected test-peer in active map")
	}

	// Cleanup.
	for _, ps := range active {
		ps.Stop()
	}
}

func TestReconcilePeersRemoved(t *testing.T) {
	active := make(map[string]*PeerSession)
	table := NewSATable()
	log := slog.Default()

	peer := testPeer()
	cfg := testIPsecConfig(peer)
	reconcilePeers(cfg, nil, active, table, nil, nil, nil, log)

	if len(active) != 1 {
		t.Fatalf("setup: expected 1 active peer, got %d", len(active))
	}

	emptyCfg := testIPsecConfig()
	reconcilePeers(emptyCfg, cfg, active, table, nil, nil, nil, log)

	if len(active) != 0 {
		t.Fatalf("expected 0 active peers after removal, got %d", len(active))
	}
}

func TestReconcilePeersChanged(t *testing.T) {
	active := make(map[string]*PeerSession)
	table := NewSATable()
	log := slog.Default()

	peer := testPeer()
	cfg := testIPsecConfig(peer)
	reconcilePeers(cfg, nil, active, table, nil, nil, nil, log)

	oldPS := active["test-peer"]

	changedPeer := peer
	changedPeer.RemoteAddress = "198.51.100.1"
	newCfg := testIPsecConfig(changedPeer)
	reconcilePeers(newCfg, cfg, active, table, nil, nil, nil, log)

	if len(active) != 1 {
		t.Fatalf("expected 1 active peer, got %d", len(active))
	}
	newPS := active["test-peer"]
	if newPS == oldPS {
		t.Fatal("expected new PeerSession after config change")
	}

	// Cleanup.
	for _, ps := range active {
		ps.Stop()
	}
}

func TestReconcilePeersUnchanged(t *testing.T) {
	active := make(map[string]*PeerSession)
	table := NewSATable()
	log := slog.Default()

	peer := testPeer()
	cfg := testIPsecConfig(peer)
	reconcilePeers(cfg, nil, active, table, nil, nil, nil, log)

	oldPS := active["test-peer"]

	reconcilePeers(cfg, cfg, active, table, nil, nil, nil, log)

	if active["test-peer"] != oldPS {
		t.Fatal("unchanged peer should keep the same PeerSession")
	}

	// Cleanup.
	for _, ps := range active {
		ps.Stop()
	}
}

// VALIDATES: AC-1 (clear triggers re-initiation). TerminateAllSAs stops each session,
// removes it from the SATable, deletes it from the active map, and calls reEstablishFn;
// because activePeersMap and reconcilePeers' `active` are the SAME map object (A-1's
// load-bearing identity), the reEstablish closure sees the peer absent and starts a
// fresh session. A refactor that copies the map would silently break `clear`.
func TestTerminateAllSAsReinitiates(t *testing.T) {
	log := slog.Default()
	active := make(map[string]*PeerSession)
	setActivePeers(active)
	t.Cleanup(func() { setActivePeers(nil) })
	table := NewSATable()
	SetActiveTableForTest(table)
	t.Cleanup(func() { SetActiveTableForTest(nil) })

	peer := testPeer() // connection-type initiate, so it re-initiates on reconcile
	cfg := testIPsecConfig(peer)
	reconcilePeers(cfg, nil, active, table, nil, nil, nil, log)
	first := active[peer.Name]
	if first == nil {
		t.Fatal("setup: peer session was not started")
	}

	// reEstablish re-runs reconcile against the SAME active map (as runEngine's closure
	// does), so a peer TerminateAllSAs removed is started again.
	reEst := func() { reconcilePeers(cfg, nil, active, table, nil, nil, nil, log) }
	reEstablishFn.Store(&reEst)
	t.Cleanup(func() { reEstablishFn.Store(nil) })

	count := TerminateAllSAs()
	if count != 1 {
		t.Fatalf("TerminateAllSAs terminated %d, want 1", count)
	}

	second := active[peer.Name]
	if second == nil {
		t.Fatal("clear did not re-initiate the peer (no session after reEstablish)")
	}
	if second == first {
		t.Error("re-initiation must be a fresh session, not the terminated one")
	}

	// Cleanup.
	for _, ps := range active {
		ps.Stop()
	}
}

// testSelectorPolicy builds one traffic-selector policy row in the shape
// parseTrafficSelectors produces, with its own *net.IPNet allocations. Two calls with the
// same arguments therefore give EQUAL policies behind DIFFERENT pointers, which is what a
// reload gives: two independent parses of one config file.
func testSelectorPolicy(t testing.TB, number, localPrefix, remotePrefix string) ipsec.TrafficSelectorPolicy {
	t.Helper()
	_, local, err := net.ParseCIDR(localPrefix)
	if err != nil {
		t.Fatalf("local prefix %q: %v", localPrefix, err)
	}
	_, remote, err := net.ParseCIDR(remotePrefix)
	if err != nil {
		t.Fatalf("remote prefix %q: %v", remotePrefix, err)
	}
	return ipsec.TrafficSelectorPolicy{
		Number:       number,
		LocalPrefix:  local,
		LocalPort:    ipsec.PortSelector{Form: ipsec.PortSingle, Port: 179},
		RemotePrefix: remote,
		RemotePort:   ipsec.AnyPort(),
		Protocol:     6,
	}
}

// testPeerEveryMember is a peer with every member of ipsec.SiteToSitePeer, and every
// member of its Auth, set to a value that is not the Go zero value. It is rebuilt on each
// call, so two calls give equal values behind different pointers and different slice
// backing arrays.
//
// The zero values are excluded on purpose. A member left at zero here would let
// TestPeerConfigChangedNoEditNoChange pass without the comparison ever reaching it, so a
// member added to ipsec.SiteToSitePeer belongs in this builder on the day it is added.
func testPeerEveryMember(t testing.TB) ipsec.SiteToSitePeer {
	t.Helper()
	return ipsec.SiteToSitePeer{
		Name:           "branch-1",
		IKEGroup:       "test-ike",
		ESPGroup:       "test-esp",
		ConnectionType: ipsec.ConnectionRespond,
		LocalAddress:   "192.0.2.10",
		RemoteAddress:  "192.0.2.1",
		Auth: ipsec.AuthConfig{
			Mode:                ipsec.AuthPreSharedSecret,
			PSK:                 "test-secret",
			LocalID:             "ze.example.net",
			RemoteID:            "peer.example.net",
			CACertificate:       "test-ca",
			Certificate:         "test-cert",
			RemoteIDType:        2,
			CertificateCount:    4,
			HashAndURL:          true,
			CertificateURL:      "http://cert.example.net/ze.der",
			CertificateURLAllow: []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")},
		},
		VTIBind:           "vti0",
		IfID:              42,
		TrafficSelectors:  []ipsec.TrafficSelectorPolicy{testSelectorPolicy(t, "1", "10.1.0.0/16", "10.2.0.0/16")},
		Mode:              dataplane.ModeTunnel,
		TransportRequired: true,
	}
}

// mutateForTest changes v to a value it does not already hold and reports whether it
// could. A kind it does not know draws false, and TestPeerConfigChangedIsFailClosed turns
// that false into a failure: an unhandled kind is exactly the member nobody classified.
//
// The recursion is over a Go struct type fixed at compile time, so the depth is bounded by
// the nesting of ipsec.SiteToSitePeer and no peer-controlled input reaches it.
func mutateForTest(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		v.SetString(v.String() + "-mutated")
		return true
	case reflect.Bool:
		v.SetBool(!v.Bool())
		return true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(v.Int() + 1)
		return true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(v.Uint() + 1)
		return true
	case reflect.Slice:
		v.Set(reflect.Append(v, reflect.New(v.Type().Elem()).Elem()))
		return true
	case reflect.Pointer:
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
			return true
		}
		return mutateForTest(v.Elem())
	case reflect.Struct:
		for _, field := range reflect.VisibleFields(v.Type()) {
			if len(field.Index) != 1 {
				continue // promoted from an embedded struct; reached through its own member
			}
			member := v.FieldByIndex(field.Index)
			if member.CanSet() && mutateForTest(member) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// VALIDATES: AC-3 -- a member added to ipsec.SiteToSitePeer and classified nowhere makes
// the guard report a change, so the conservative action is what an unclassified member
// draws. The walk is over reflect.VisibleFields rather than a written list, which is what
// covers the member added TOMORROW: nobody has to remember this test exists.
//
// PREVENTS: the shape the guard had before this spec. It named eight members of the twelve
// and ignored the rest by omission, so an operator's edit to a member nobody had listed
// committed successfully and reached no wire.
func TestPeerConfigChangedIsFailClosed(t *testing.T) {
	base := testPeerEveryMember(t)

	for _, field := range reflect.VisibleFields(reflect.TypeOf(base)) {
		if len(field.Index) != 1 {
			continue // promoted from an embedded struct; reached through its own member
		}
		t.Run(field.Name, func(t *testing.T) {
			edited := testPeerEveryMember(t)
			target := reflect.ValueOf(&edited).Elem().FieldByIndex(field.Index)
			if !mutateForTest(target) {
				t.Fatalf("member %s (%s) has no mutation this test knows: either classify it by name in ipsec.SiteToSitePeer.Equal, or teach mutateForTest its kind",
					field.Name, field.Type)
			}
			ps := &PeerSession{peerCfg: base, ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
			if !peerConfigChanged(ps, edited, testIKEGroup(), testESPGroup()) {
				t.Errorf("member %s changed and the guard reported no change", field.Name)
			}
		})
	}
}

// VALIDATES: AC-1 -- the members the previous guard omitted each force the peer to
// restart, named one per row. The reflection walk above proves the same property
// mechanically; this table is what a reader of the diff can check, and it is the
// regression test for the defect this spec exists to remove.
//
// A member that stops forcing a restart has to DELETE its row here and state the reason in
// ipsec.SiteToSitePeer.Equal, which is the difference between a decision and an omission.
func TestPeerConfigChangedIgnoresNothingSilently(t *testing.T) {
	cases := []struct {
		member string
		edit   func(t *testing.T, p *ipsec.SiteToSitePeer)
		means  string
	}{
		{"TrafficSelectors", func(t *testing.T, p *ipsec.SiteToSitePeer) {
			p.TrafficSelectors = []ipsec.TrafficSelectorPolicy{testSelectorPolicy(t, "1", "10.1.0.0/24", "10.2.0.0/16")}
		}, "the operator narrowed which traffic the tunnel carries"},
		{"TrafficSelectors added", func(t *testing.T, p *ipsec.SiteToSitePeer) {
			p.TrafficSelectors = append(p.TrafficSelectors, testSelectorPolicy(t, "2", "10.3.0.0/24", "10.4.0.0/24"))
		}, "the operator added a second selector row"},
		{"TrafficSelectors removed", func(_ *testing.T, p *ipsec.SiteToSitePeer) {
			p.TrafficSelectors = nil
		}, "the operator deleted the list, so the peer accepts whatever is proposed"},
		{"Mode", func(_ *testing.T, p *ipsec.SiteToSitePeer) { p.Mode = dataplane.ModeTransport }, "tunnel mode became transport mode"},
		{"TransportRequired", func(_ *testing.T, p *ipsec.SiteToSitePeer) { p.TransportRequired = false }, "a silent downgrade to tunnel mode became acceptable"},
		{"VTIBind", func(_ *testing.T, p *ipsec.SiteToSitePeer) { p.VTIBind = "vti1" }, "the tunnel binds to another interface"},
		{"IfID", func(_ *testing.T, p *ipsec.SiteToSitePeer) { p.IfID = 43 }, "the XFRM if_id moved, so the SA binds elsewhere"},
		{"Auth.LocalID", func(_ *testing.T, p *ipsec.SiteToSitePeer) { p.Auth.LocalID = "other.example.net" }, "ze asserts another identity"},
		{"Auth.RemoteID", func(_ *testing.T, p *ipsec.SiteToSitePeer) { p.Auth.RemoteID = "other.example.net" }, "the operator pinned the peer to another identity"},
		{"Auth.RemoteIDType", func(_ *testing.T, p *ipsec.SiteToSitePeer) { p.Auth.RemoteIDType = 3 }, "the peer must now assert another ID type"},
		{"Auth.CACertificate", func(_ *testing.T, p *ipsec.SiteToSitePeer) { p.Auth.CACertificate = "other-ca" }, "another trust anchor validates the peer"},
		{"Auth.CertificateCount", func(_ *testing.T, p *ipsec.SiteToSitePeer) { p.Auth.CertificateCount = 2 }, "the accepted chain length changed"},
		{"Auth.HashAndURL", func(_ *testing.T, p *ipsec.SiteToSitePeer) { p.Auth.HashAndURL = false }, "hash-and-url certificates were turned off"},
		{"Auth.CertificateURL", func(_ *testing.T, p *ipsec.SiteToSitePeer) {
			p.Auth.CertificateURL = "http://cert.example.net/other.der"
		}, "ze publishes its certificate elsewhere"},
		{"Auth.CertificateURLAllow", func(_ *testing.T, p *ipsec.SiteToSitePeer) {
			p.Auth.CertificateURLAllow = []netip.Prefix{netip.MustParsePrefix("198.51.100.0/24")}
		}, "the fetcher's destination allow list changed"},
	}

	for _, tc := range cases {
		t.Run(tc.member, func(t *testing.T) {
			ps := &PeerSession{peerCfg: testPeerEveryMember(t), ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
			edited := testPeerEveryMember(t)
			tc.edit(t, &edited)
			if !peerConfigChanged(ps, edited, testIKEGroup(), testESPGroup()) {
				t.Errorf("%s changed (%s) and the guard reported no change", tc.member, tc.means)
			}
		})
	}
}

// VALIDATES: AC-4 -- a reload that changes one of the eight members the guard already
// named takes the action it took before this spec, which is a restart. The subtraction
// shape widens what forces a restart; it must not narrow it.
func TestPeerConfigChangedKeepsTheMembersItAlreadyNamed(t *testing.T) {
	cases := []struct {
		member string
		edit   func(p *ipsec.SiteToSitePeer)
	}{
		{"RemoteAddress", func(p *ipsec.SiteToSitePeer) { p.RemoteAddress = "198.51.100.1" }},
		{"LocalAddress", func(p *ipsec.SiteToSitePeer) { p.LocalAddress = "198.51.100.10" }},
		{"IKEGroup", func(p *ipsec.SiteToSitePeer) { p.IKEGroup = "other-ike" }},
		{"ESPGroup", func(p *ipsec.SiteToSitePeer) { p.ESPGroup = "other-esp" }},
		{"ConnectionType", func(p *ipsec.SiteToSitePeer) { p.ConnectionType = ipsec.ConnectionInitiate }},
		{"Auth.Mode", func(p *ipsec.SiteToSitePeer) { p.Auth.Mode = ipsec.AuthX509 }},
		{"Auth.PSK", func(p *ipsec.SiteToSitePeer) { p.Auth.PSK = "other-secret" }},
		{"Auth.Certificate", func(p *ipsec.SiteToSitePeer) { p.Auth.Certificate = "other-cert" }},
	}

	for _, tc := range cases {
		t.Run(tc.member, func(t *testing.T) {
			ps := &PeerSession{peerCfg: testPeerEveryMember(t), ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
			edited := testPeerEveryMember(t)
			tc.edit(&edited)
			if !peerConfigChanged(ps, edited, testIKEGroup(), testESPGroup()) {
				t.Errorf("%s changed and the guard reported no change", tc.member)
			}
		})
	}
}

// VALIDATES: AC-2 -- a reload that changes no member restarts nothing. The two peers are
// built by SEPARATE calls, so their prefixes and slices are equal values behind different
// pointers, which is what a reload hands the guard: two independent parses of one config
// file. A comparison that answered on pointer identity would fail here.
//
// This is R-1's early signal. A guard that reports a change over a whole struct restarts
// every peer on every commit unless every member is stable across a parse, and that would
// be a worse defect than the one this spec removes.
func TestPeerConfigChangedNoEditNoChange(t *testing.T) {
	ps := &PeerSession{peerCfg: testPeerEveryMember(t), ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
	if peerConfigChanged(ps, testPeerEveryMember(t), testIKEGroup(), testESPGroup()) {
		t.Error("a reload that edited nothing reported a change, so every commit would bounce the tunnel")
	}
}

// VALIDATES: AC-1 end to end inside the engine -- an operator edits ONLY the traffic
// selectors and commits. reconcilePeers must stop the running session and start a fresh
// one, and the fresh session must carry the EDITED selectors in ps.peerCfg.
//
// The second half is the half the guard alone does not give. startPeerSession is the only
// writer of ps.peerCfg, and initiator.go and responder.go copy that field into sa.PeerCfg,
// which proposeChildTSPayloads (rekey.go) reads to build the TSi and TSr of the next
// CREATE_CHILD_SA. A session left running keeps proposing the selectors it was born with.
func TestStartPeerSessionWritesTheFreshConfig(t *testing.T) {
	active := make(map[string]*PeerSession)
	table := NewSATable()
	log := slog.Default()

	peer := testPeer()
	peer.TrafficSelectors = []ipsec.TrafficSelectorPolicy{testSelectorPolicy(t, "1", "10.1.0.0/16", "10.2.0.0/16")}
	cfg := testIPsecConfig(peer)
	reconcilePeers(cfg, nil, active, table, nil, nil, nil, log)

	before := active["test-peer"]
	if before == nil {
		t.Fatal("setup: peer session was not started")
	}

	narrowed := testPeer()
	narrowed.TrafficSelectors = []ipsec.TrafficSelectorPolicy{testSelectorPolicy(t, "1", "10.1.0.0/24", "10.2.0.0/16")}
	newCfg := testIPsecConfig(narrowed)
	reconcilePeers(newCfg, cfg, active, table, nil, nil, nil, log)

	after := active["test-peer"]
	if after == nil {
		t.Fatal("the peer disappeared instead of restarting")
	}
	if after == before {
		t.Fatal("a traffic-selector edit left the running session in place, so the tunnel keeps the selectors it was born with")
	}
	if !reflect.DeepEqual(after.peerCfg.TrafficSelectors, narrowed.TrafficSelectors) {
		t.Errorf("restarted session carries selectors %v, want the edited %v",
			after.peerCfg.TrafficSelectors, narrowed.TrafficSelectors)
	}

	// Cleanup.
	for _, ps := range active {
		ps.Stop()
	}
}

// VALIDATES: the reload apply FAILS CLOSED. An apply that reaches the engine with nothing
// staged returns errIKEApplyWithoutVerify and applies nothing, so a transaction can never
// be reported as committed while the engine keeps the configuration it was already
// running.
//
// PREVENTS: this spec's own defect, one layer along. The engine used to answer every
// config-apply with OK because it registered no handler at all, and an apply handler that
// answered OK on an empty stash would restore exactly that: a commit that succeeds, a
// `sighup reload complete`, and no change on the wire. runVerify and runApply pick their
// participants with the same predicate (filterDiffs, config/transaction/orchestrator.go),
// so an apply without a verify is a protocol violation and not a state to shrug at.
func TestIKEConfigApplyWithoutVerifyIsRefused(t *testing.T) {
	var applied []*ipsec.IPsecConfig
	staging := &ikeConfigStaging{apply: func(cfg *ipsec.IPsecConfig) error {
		applied = append(applied, cfg)
		return nil
	}}

	if err := staging.commit(); !errors.Is(err, errIKEApplyWithoutVerify) {
		t.Errorf("apply with nothing staged returned %v, want errIKEApplyWithoutVerify", err)
	}
	if len(applied) != 0 {
		t.Errorf("apply ran %d times with nothing staged, want 0", len(applied))
	}

	first := testIPsecConfig(testPeer())
	staging.stage(first)
	if err := staging.commit(); err != nil {
		t.Fatalf("apply after a verify returned %v, want nil", err)
	}
	if len(applied) != 1 || applied[0] != first {
		t.Fatalf("apply ran %d times, want 1 with the staged config", len(applied))
	}

	// The stash is SINGLE USE. A second apply in the same transaction has no config of
	// its own, and re-applying the one the coordinator already committed would hide the
	// protocol violation rather than report it.
	if err := staging.commit(); !errors.Is(err, errIKEApplyWithoutVerify) {
		t.Errorf("a second apply returned %v, want errIKEApplyWithoutVerify", err)
	}
	if len(applied) != 1 {
		t.Errorf("apply ran %d times across two commits, want 1", len(applied))
	}
}

// VALIDATES: an operator rotating a cipher restarts the peer, even though the peer block
// itself is byte-identical. A peer names its groups; the crypto lives in the group.
//
// PREVENTS: the same defect this spec removes, one indirection along. startPeerSession
// copies the RESOLVED ike-group and esp-group onto the PeerSession and nothing refreshes
// them, and ipsec.SiteToSitePeer holds only the group NAMES. A guard that compared the
// peer alone would report "nothing changed" for `ike-group IKE-1 { proposal 1 {
// encryption ... } }`, the commit would succeed, and the tunnel would keep negotiating the
// algorithm the operator replaced. It was unreachable while reload applied nothing at all.
func TestPeerConfigChangedSeesTheResolvedGroups(t *testing.T) {
	rotatedIKE := func() ipsec.IKEGroup {
		g := testIKEGroup()
		g.Proposals[0].Encryption = ipsec.EncryptionAES128
		return g
	}
	rotatedESP := func() ipsec.ESPGroup {
		g := testESPGroup()
		g.Proposals[0].Encryption = ipsec.EncryptionAES128
		return g
	}

	cases := []struct {
		what string
		ike  ipsec.IKEGroup
		esp  ipsec.ESPGroup
		want bool
	}{
		{"neither group moved", testIKEGroup(), testESPGroup(), false},
		{"the IKE group's cipher rotated", rotatedIKE(), testESPGroup(), true},
		{"the ESP group's cipher rotated", testIKEGroup(), rotatedESP(), true},
		{"the groups were deleted from the config", ipsec.IKEGroup{}, ipsec.ESPGroup{}, true},
	}

	for _, tc := range cases {
		t.Run(tc.what, func(t *testing.T) {
			ps := &PeerSession{peerCfg: testPeerEveryMember(t), ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
			if got := peerConfigChanged(ps, testPeerEveryMember(t), tc.ike, tc.esp); got != tc.want {
				t.Errorf("%s: guard reported changed=%v, want %v", tc.what, got, tc.want)
			}
		})
	}
}

// VALIDATES: the same rotation driven through reconcilePeers, which is the entry point a
// commit reaches. The peer block does not move; only the group the peer names does. The
// session must be replaced and the fresh one must carry the rotated group.
func TestReconcilePeersRestartsOnGroupRotation(t *testing.T) {
	active := make(map[string]*PeerSession)
	table := NewSATable()
	log := slog.Default()

	cfg := testIPsecConfig(testPeer())
	reconcilePeers(cfg, nil, active, table, nil, nil, nil, log)
	before := active["test-peer"]
	if before == nil {
		t.Fatal("setup: peer session was not started")
	}

	rotated := testIPsecConfig(testPeer())
	group := rotated.IKEGroups["test-ike"]
	group.Proposals[0].Encryption = ipsec.EncryptionAES128
	rotated.IKEGroups["test-ike"] = group

	reconcilePeers(rotated, cfg, active, table, nil, nil, nil, log)

	after := active["test-peer"]
	if after == nil {
		t.Fatal("the peer disappeared instead of restarting")
	}
	if after == before {
		t.Fatal("a cipher rotation left the running session in place, so the tunnel keeps negotiating the old algorithm")
	}
	if after.ikeGroup.Proposals[0].Encryption != ipsec.EncryptionAES128 {
		t.Errorf("restarted session carries IKE encryption %s, want the rotated aes128", after.ikeGroup.Proposals[0].Encryption)
	}

	// Cleanup.
	for _, ps := range active {
		ps.Stop()
	}
}

// VALIDATES: an apply whose own error reaches the caller is not swallowed by the staging
// layer. The reload path returns it so the transaction rolls back, and the peers this
// engine is running are left alone.
//
// PREVENTS: the interface-read regression. `interface` supplies the local address of every
// peer that names none, so a transient read failure on reload gives those peers an EMPTY
// LocalAddress. peerConfigChanged compares that against the address the running sessions
// resolved at startup, and would stop and restart EVERY one of them into a state the
// engine's own message calls "will fail". This was unreachable while reload applied
// nothing.
func TestIKEConfigApplyPropagatesTheApplyError(t *testing.T) {
	wantErr := errors.New("interface read failed")
	staging := &ikeConfigStaging{apply: func(*ipsec.IPsecConfig) error { return wantErr }}

	staging.stage(testIPsecConfig(testPeer()))
	if err := staging.commit(); !errors.Is(err, wantErr) {
		t.Errorf("commit returned %v, want the apply's own error", err)
	}
}

// VALIDATES: the predicate that decides whether a failed interface read is a refusal or a
// warning. A configuration whose every peer carries its own local-address does not depend
// on the interface, so refusing it would refuse a config that works.
func TestPeersNeedInterfaceAddress(t *testing.T) {
	withAddress := testPeer()
	withAddress.LocalAddress = "192.0.2.10"
	withoutAddress := testPeer()
	withoutAddress.Name = "other-peer"
	withoutAddress.LocalAddress = ""

	if peersNeedInterfaceAddress(testIPsecConfig()) {
		t.Error("a config with no peers needs no interface address")
	}
	if peersNeedInterfaceAddress(testIPsecConfig(withAddress)) {
		t.Error("a peer carrying its own local-address does not depend on the interface")
	}
	if !peersNeedInterfaceAddress(testIPsecConfig(withoutAddress)) {
		t.Error("a peer with no local-address depends on the interface")
	}
	if got := peersWithoutLocalAddress(testIPsecConfig(withAddress, withoutAddress)); got != 1 {
		t.Errorf("peersWithoutLocalAddress = %d, want 1", got)
	}
}

// VALIDATES: unbindablePeers answers on the lookup RESULT it is handed, names the failure
// that actually happened, and stays silent for a configuration the interface cannot
// affect. It is the condition a reload refuses and startup only warns about.
//
// PREVENTS: two failures that look opposite and are the same mistake. Refusing a
// configuration whose every peer carries its own local-address would deny a config that
// works. Reporting "no IPv4 address" for a lookup that never RAN would send the operator
// to the interface configuration for a fault that is not in it.
func TestUnbindablePeersReportsOnlyTheDependentCase(t *testing.T) {
	withAddress := testPeer()
	withAddress.LocalAddress = "192.0.2.10"
	withoutAddress := testPeer()
	withoutAddress.Name = "other-peer"
	withoutAddress.LocalAddress = ""

	lookupErr := errors.New("iface: no backend loaded")

	independent := testIPsecConfig(withAddress)
	independent.Interface = "eth0"
	if err := unbindablePeers(independent, lookupErr); err != nil {
		t.Errorf("a config whose every peer carries its own local-address was refused: %v", err)
	}
	if err := unbindablePeers(independent, nil); err != nil {
		t.Errorf("the same config with a lookup that found no IPv4 was refused: %v", err)
	}

	dependent := testIPsecConfig(withAddress, withoutAddress)
	dependent.Interface = "eth0"

	err := unbindablePeers(dependent, lookupErr)
	if err == nil {
		t.Fatal("a peer with no local-address was accepted over an interface that could not be read")
	}
	if !errors.Is(err, lookupErr) {
		t.Errorf("the refusal dropped the lookup failure it is reporting: %v", err)
	}
	for _, want := range []string{"eth0", "cannot read addresses", "1 peer(s)"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %q", err, want)
		}
	}

	err = unbindablePeers(dependent, nil)
	if err == nil {
		t.Fatal("a peer with no local-address was accepted over an interface holding no IPv4 address")
	}
	if !strings.Contains(err.Error(), "no IPv4 address") {
		t.Errorf("refusal %q does not say the interface holds no IPv4 address", err)
	}
	if errors.Is(err, lookupErr) {
		t.Errorf("refusal %q blames a lookup failure that did not happen", err)
	}
}
