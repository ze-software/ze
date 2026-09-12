// VALIDATES: that the SAD and SPD read paths report what a REAL kernel holds --
// an SA and a policy this test installed appear in the dump with their own
// values, and disappear from it when removed.
// PREVENTS: the vacuity trap the older TestXFRMListSAs falls into. That test
// passes on an empty kernel with the whole read body deleted, because it asserts
// only that whatever came back is well formed. Every assertion below names an
// object this test installed, and asserts a TRANSITION (absent, present, absent)
// rather than a state (ai/rules/interop-and-goal-validation.md).
//
// Design: docs/architecture/ike/ipsec-dataplane-inspection.md -- kernel dataplane read surface
// Related: xfrm_readback_linux_test.go -- the same mapping driven from fixtures

//go:build integration && linux

package dataplane

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// readbackSPI is chosen high and distinctive so a stray SA on the test host
// cannot be mistaken for the one under test.
const (
	readbackSPI   uint32 = 0x5ead0001
	readbackReqID uint32 = 0x5ead
)

// findSA returns the dumped SA with this SPI, or nil.
func findSA(t *testing.T, b *xfrmBackend, spi uint32) *SAInfo {
	t.Helper()
	sas, err := b.ListSAs(0)
	if err != nil {
		skipUnprivileged(t, err)
		t.Fatalf("ListSAs: %v", err)
	}
	for i := range sas {
		if sas[i].SPI == spi {
			return &sas[i]
		}
	}
	return nil
}

// TestXFRMReadbackShowsInstalledSA is the transition assertion for the SAD.
func TestXFRMReadbackShowsInstalledSA(t *testing.T) {
	b := &xfrmBackend{}
	src := net.ParseIP("10.88.0.1")
	dst := net.ParseIP("10.88.0.2")

	if sa := findSA(t, b, readbackSPI); sa != nil {
		t.Fatalf("SPI %#x is already installed before the test ran: %+v", readbackSPI, sa)
	}

	err := b.InstallSA(SAParams{
		SPI: readbackSPI, Src: src, Dst: dst, Proto: ProtoESP, Mode: ModeTunnel,
		ReqID: readbackReqID, ReplayWin: 64,
		EncAlgo: "aes256", EncKey: make([]byte, 32),
		AuthAlgo: "sha256", AuthKey: make([]byte, 32),
	})
	if err != nil {
		skipUnprivileged(t, err)
		t.Fatalf("InstallSA: %v", err)
	}
	t.Cleanup(func() { _ = b.RemoveSA(readbackSPI, dst, ProtoESP) })

	sa := findSA(t, b, readbackSPI)
	if sa == nil {
		t.Fatalf("SPI %#x was installed and the SAD dump does not report it", readbackSPI)
	}

	// Every field below was set by THIS test, so a dump that dropped or
	// mistranslated one is caught here rather than by inspection.
	if !sa.Src.Equal(src) || !sa.Dst.Equal(dst) {
		t.Errorf("addresses = %v -> %v, want %v -> %v", sa.Src, sa.Dst, src, dst)
	}
	if sa.Mode != ModeTunnel {
		t.Errorf("Mode = %d, want ModeTunnel (%d): the kernel numbers tunnel one lower than ze does",
			sa.Mode, ModeTunnel)
	}
	if sa.ReqID != readbackReqID {
		t.Errorf("ReqID = %d, want %d", sa.ReqID, readbackReqID)
	}
	if sa.ReplayWindow != 64 {
		t.Errorf("ReplayWindow = %d, want 64", sa.ReplayWindow)
	}
	if sa.Proto != ProtoESP {
		t.Errorf("Proto = %d, want ESP (%d)", sa.Proto, ProtoESP)
	}
	// A-1: the dump populates the algorithm arms and the counters without a
	// per-SA XFRM_MSG_GETSA round trip.
	if sa.Encryption == "" {
		t.Error("Encryption is empty: the dump did not carry the cipher name")
	}
	if sa.EncryptionKeyBits != 256 {
		t.Errorf("EncryptionKeyBits = %d, want 256", sa.EncryptionKeyBits)
	}
	if sa.Integrity == "" {
		t.Error("Integrity is empty: the dump did not carry the integrity name")
	}
	if sa.AddedAt.IsZero() {
		t.Error("AddedAt is the zero time: the kernel records an add time for every SA it accepts")
	}
	// A fresh SA has carried nothing, so UsedAt must be the zero time rather
	// than 1970.
	if !sa.UsedAt.IsZero() {
		t.Errorf("UsedAt = %v, want the zero time for an SA that has carried no packet", sa.UsedAt)
	}
	if sa.BytesHard != 0 || sa.PacketsHard != 0 {
		t.Errorf("unlimited hard limits = %d/%d, want 0/0", sa.BytesHard, sa.PacketsHard)
	}

	// The removal half. Without it the test would prove only that a dump lists
	// something, not that it TRACKS the kernel.
	if err := b.RemoveSA(readbackSPI, dst, ProtoESP); err != nil {
		t.Fatalf("RemoveSA: %v", err)
	}
	if sa := findSA(t, b, readbackSPI); sa != nil {
		t.Errorf("SPI %#x was removed and the SAD dump still reports it: %+v", readbackSPI, sa)
	}
}

// TestXFRMReadbackIfIDFilter proves the filter argument against a real dump.
func TestXFRMReadbackIfIDFilter(t *testing.T) {
	b := &xfrmBackend{}
	src := net.ParseIP("10.88.1.1")
	dst := net.ParseIP("10.88.1.2")
	const spi uint32 = 0x5ead0002
	const ifID uint32 = 0x5e01

	err := b.InstallSA(SAParams{
		SPI: spi, Src: src, Dst: dst, Proto: ProtoESP, Mode: ModeTunnel,
		ReqID: readbackReqID, IfID: ifID,
		EncAlgo: "aes256", EncKey: make([]byte, 32),
		AuthAlgo: "sha256", AuthKey: make([]byte, 32),
	})
	if err != nil {
		skipUnprivileged(t, err)
		t.Fatalf("InstallSA: %v", err)
	}
	t.Cleanup(func() { _ = b.RemoveSA(spi, dst, ProtoESP) })

	matching, err := b.ListSAs(ifID)
	if err != nil {
		t.Fatalf("ListSAs(%d): %v", ifID, err)
	}
	var seen bool
	for i := range matching {
		if matching[i].SPI == spi {
			seen = true
		}
		if matching[i].IfID != ifID {
			t.Errorf("ListSAs(%d) returned an SA with if_id %d", ifID, matching[i].IfID)
		}
	}
	if !seen {
		t.Fatalf("ListSAs(%d) did not return the SA installed with that if_id", ifID)
	}

	// A DIFFERENT if_id must not return it. Without this the filter could be a
	// no-op and the assertion above would still hold.
	other, err := b.ListSAs(ifID + 1)
	if err != nil {
		t.Fatalf("ListSAs(%d): %v", ifID+1, err)
	}
	for i := range other {
		if other[i].SPI == spi {
			t.Errorf("ListSAs(%d) returned the SA installed at if_id %d, so the filter does nothing", ifID+1, ifID)
		}
	}
}

// TestXFRMReadbackPolicyNoIfID is A-5's evidence: the SPD dump is NOT limited to
// xfrm-interface peers, so it reports the site-to-site policies IKE installs at
// if_id 0. If it were limited, the dump would miss most policies in the field.
func TestXFRMReadbackPolicyNoIfID(t *testing.T) {
	b := &xfrmBackend{}
	selectorSrc := mustCIDR(t, "10.88.2.0/24")
	selectorDst := mustCIDR(t, "10.88.3.0/24")

	params := SPParams{
		Src: selectorSrc, Dst: selectorDst, Dir: SADirOut,
		Proto: ProtoESP, Mode: ModeTunnel,
		ReqID:     readbackReqID,
		TunnelSrc: net.ParseIP("10.88.2.1"),
		TunnelDst: net.ParseIP("10.88.3.1"),
		Owner:     "readback-peer",
	}
	if err := b.InstallPolicy(params); err != nil {
		skipUnprivileged(t, err)
		t.Fatalf("InstallPolicy: %v", err)
	}
	t.Cleanup(func() { _ = b.RemovePolicyParams(params) })

	policies, err := b.ListPolicies()
	if err != nil {
		t.Fatalf("ListPolicies: %v", err)
	}
	var found *PolicyInfo
	for i := range policies {
		p := &policies[i]
		if p.Src != nil && p.Dst != nil &&
			p.Src.String() == selectorSrc.String() && p.Dst.String() == selectorDst.String() &&
			p.Dir == SADirOut {
			found = p
		}
	}
	if found == nil {
		t.Fatalf("a policy installed at if_id 0 is absent from the SPD dump, so the dump misses site-to-site policies")
	}
	if found.IfID != 0 {
		t.Errorf("IfID = %d, want 0", found.IfID)
	}
	if found.Mode != ModeTunnel {
		t.Errorf("Mode = %d, want ModeTunnel", found.Mode)
	}
	if found.ReqID != readbackReqID {
		t.Errorf("ReqID = %d, want %d", found.ReqID, readbackReqID)
	}
	if !found.TunnelSrc.Equal(params.TunnelSrc) || !found.TunnelDst.Equal(params.TunnelDst) {
		t.Errorf("template endpoints = %v -> %v, want %v -> %v",
			found.TunnelSrc, found.TunnelDst, params.TunnelSrc, params.TunnelDst)
	}
	// The owner join across a REAL kernel round trip. This is A-7 proven against
	// the kernel's own normalization rather than against a fixture.
	if !found.OwnerKnown || found.Owner != "readback-peer" {
		t.Errorf("owner = %q (known=%v), want readback-peer: the kernel row did not resolve back to the policy ze installed",
			found.Owner, found.OwnerKnown)
	}

	if err := b.RemovePolicyParams(params); err != nil {
		t.Fatalf("RemovePolicyParams: %v", err)
	}
	after, err := b.ListPolicies()
	if err != nil {
		t.Fatalf("ListPolicies after remove: %v", err)
	}
	for i := range after {
		p := &after[i]
		if p.Src != nil && p.Src.String() == selectorSrc.String() && p.Dir == SADirOut {
			t.Errorf("the policy was removed and the SPD dump still reports it: %+v", p)
		}
	}
}

// VALIDATES: IPv4 selectors retain their IPv6 tunnel endpoints, including
// addresses whose zero tails would otherwise be decoded as IPv4.
func TestXFRMReadbackPolicyTunnelAddressFamily(t *testing.T) {
	const ifID = 0x5ead1004
	b := &xfrmBackend{}
	params := SPParams{
		Src: mustCIDR(t, "10.88.8.0/24"), Dst: mustCIDR(t, "10.88.9.0/24"),
		Dir: SADirOut, Proto: ProtoESP, Mode: ModeTunnel,
		IfID: ifID, ReqID: readbackReqID, Owner: "readback-ipv6-tunnel",
		TunnelSrc: net.ParseIP("2001:db9::"),
		TunnelDst: net.ParseIP("2001:db8::"),
	}
	if err := b.InstallPolicy(params); err != nil {
		skipUnprivileged(t, err)
		t.Fatalf("InstallPolicy: %v", err)
	}
	t.Cleanup(func() { _ = b.RemovePolicyParams(params) })

	policies, err := b.ListPolicies()
	if err != nil {
		t.Fatalf("ListPolicies: %v", err)
	}
	var found *PolicyInfo
	for i := range policies {
		if policies[i].IfID == ifID {
			found = &policies[i]
		}
	}
	if found == nil {
		t.Fatal("installed IPv4 policy with IPv6 tunnel endpoints is absent from the SPD dump")
	}
	if found.Src == nil || found.Dst == nil {
		t.Fatalf("policy lost its selectors: %+v", found)
	}
	if found.Src.String() != params.Src.String() || found.Dst.String() != params.Dst.String() {
		t.Errorf("selectors = %s -> %s, want %s -> %s",
			found.Src, found.Dst, params.Src, params.Dst)
	}
	if !found.TunnelSrc.Equal(params.TunnelSrc) || !found.TunnelDst.Equal(params.TunnelDst) {
		t.Errorf("template endpoints = %v -> %v, want %v -> %v",
			found.TunnelSrc, found.TunnelDst, params.TunnelSrc, params.TunnelDst)
	}

	if err := b.RemovePolicyParams(params); err != nil {
		t.Fatalf("RemovePolicyParams: %v", err)
	}
	after, err := b.ListPolicies()
	if err != nil {
		t.Fatalf("ListPolicies after remove: %v", err)
	}
	for _, policy := range after {
		if policy.IfID == ifID {
			t.Errorf("removed policy remains in the SPD dump: %+v", policy)
		}
	}
}

// VALIDATES: an all-family kernel dump preserves each wildcard family and joins
// explicit full-tunnel selectors to their installed owner.
func TestXFRMReadbackWildcardOwners(t *testing.T) {
	const ifID = 0x5ead1001
	b := &xfrmBackend{}
	params := []SPParams{
		{
			Src: mustCIDR(t, "0.0.0.0/0"), Dst: mustCIDR(t, "0.0.0.0/0"),
			Dir: SADirOut, Proto: ProtoESP, Mode: ModeTransport,
			IfID: ifID, ReqID: readbackReqID, Owner: "ipv4-full-tunnel",
		},
		{
			Src: mustCIDR(t, "::/0"), Dst: mustCIDR(t, "::/0"),
			Dir: SADirOut, Proto: ProtoESP, Mode: ModeTransport,
			IfID: ifID, ReqID: readbackReqID, Owner: "ipv6-full-tunnel",
		},
		{
			Dst: mustCIDR(t, "2001:db8:1::/64"),
			Dir: SADirIn, Proto: ProtoESP, Mode: ModeTransport,
			IfID: ifID, ReqID: readbackReqID, Owner: "ipv6-nil-source",
		},
		{
			Dir: SADirIn, Proto: ProtoESP, Mode: ModeTransport,
			IfID: ifID, ReqID: readbackReqID, Owner: "ipv4-nil-selectors",
		},
		{
			Src: mustCIDR(t, "::/0"), Dst: mustCIDR(t, "2001:db8::/32"),
			Dir: SADirOut, Proto: ProtoESP, Mode: ModeTransport,
			IfID: ifID, ReqID: readbackReqID, Owner: "ipv6-zero-tail",
		},
		{
			Src: mustCIDR(t, "0.0.0.0/0"), Dst: mustCIDR(t, "32.1.13.184/32"),
			Dir: SADirOut, Proto: ProtoESP, Mode: ModeTransport,
			IfID: ifID, ReqID: readbackReqID, Owner: "ipv4-same-union-bytes",
		},
	}
	for _, p := range params {
		if err := b.InstallPolicy(p); err != nil {
			skipUnprivileged(t, err)
			t.Fatalf("InstallPolicy(%s): %v", p.Owner, err)
		}
		t.Cleanup(func() { _ = b.RemovePolicyParams(p) })
	}
	policies, err := b.ListPolicies()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range params {
		found := false
		for _, got := range policies {
			if got.IfID != ifID || got.Dir != want.Dir || got.Dst == nil {
				continue
			}
			dst := want.Dst
			if dst == nil {
				dst = mustCIDR(t, "0.0.0.0/0")
			}
			if got.Dst.String() != dst.String() {
				continue
			}
			found = true
			if !got.OwnerKnown || got.Owner != want.Owner {
				t.Errorf("%s owner = %q (known=%v)", dst, got.Owner, got.OwnerKnown)
			}
			if got.Src == nil || (got.Src.IP.To4() == nil) != (dst.IP.To4() == nil) {
				t.Errorf("%s source lost its family: %v", dst, got.Src)
			}
		}
		if !found {
			t.Errorf("installed policy %s missing from all-family dump", want.Owner)
		}
	}
	for _, p := range params {
		if err := b.RemovePolicyParams(p); err != nil {
			t.Fatal(err)
		}
	}
	policies, err = b.ListPolicies()
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range policies {
		if got.IfID == ifID {
			t.Errorf("removed policy remains in dump: %+v", got)
		}
	}
}

// VALIDATES: raw foreign selectors cross the kernel and vendored parser before
// reaching PolicyInfo. Ze's restricted policy writer cannot create these masks.
func TestXFRMReadbackRawPortMasks(t *testing.T) {
	const ifID = 0x5ead1002
	b := &xfrmBackend{}
	cases := []struct {
		src, dst PortMatch
	}{
		{PortMatch{Port: 0x1200, Mask: 0xff00}, ExactPortMatch(0)},
		{ExactPortMatch(0), PortMatch{Port: 0x3400, Mask: 0xff00}},
	}
	for _, tc := range cases {
		// A claim for the values with Ze's writable masks must not own the
		// foreign selector, even though all its other key fields agree.
		claim := SPParams{
			Src: mustCIDR(t, "10.88.6.0/24"), Dst: mustCIDR(t, "10.88.7.0/24"),
			Dir: SADirOut, UpperProto: 17, IfID: ifID, Owner: "nearby-peer",
		}
		if tc.src.Port != 0 {
			claim.SrcPort = ExactPortMatch(tc.src.Port)
		}
		if tc.dst.Port != 0 {
			claim.DstPort = ExactPortMatch(tc.dst.Port)
		}
		if _, err := b.policies.claim(claim); err != nil {
			t.Fatal(err)
		}
		msg := &nl.XfrmUserpolicyInfo{}
		msg.Sel.Family = unix.AF_INET
		msg.Sel.Saddr.FromIP(net.ParseIP("10.88.6.0"))
		msg.Sel.Daddr.FromIP(net.ParseIP("10.88.7.0"))
		msg.Sel.PrefixlenS, msg.Sel.PrefixlenD = 24, 24
		msg.Sel.Proto = 17
		msg.Sel.Sport, msg.Sel.SportMask = nl.Swap16(tc.src.Port), nl.Swap16(tc.src.Mask)
		msg.Sel.Dport, msg.Sel.DportMask = nl.Swap16(tc.dst.Port), nl.Swap16(tc.dst.Mask)
		msg.Dir = uint8(netlink.XFRM_DIR_OUT)
		msg.Action = uint8(netlink.XFRM_POLICY_BLOCK)
		msg.Lft.SoftByteLimit, msg.Lft.HardByteLimit = nl.XFRM_INF, nl.XFRM_INF
		msg.Lft.SoftPacketLimit, msg.Lft.HardPacketLimit = nl.XFRM_INF, nl.XFRM_INF
		req := nl.NewNetlinkRequest(nl.XFRM_MSG_NEWPOLICY, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)
		req.AddData(msg)
		req.AddData(nl.NewRtAttr(nl.XFRMA_IF_ID, nl.Uint32Attr(ifID)))
		if _, err := req.Execute(unix.NETLINK_XFRM, 0); err != nil {
			skipUnprivileged(t, err)
			t.Fatalf("raw policy add: %v", err)
		}
		remove := func() error {
			del := nl.NewNetlinkRequest(nl.XFRM_MSG_DELPOLICY, unix.NLM_F_ACK)
			del.AddData(&nl.XfrmUserpolicyId{Sel: msg.Sel, Dir: msg.Dir})
			del.AddData(nl.NewRtAttr(nl.XFRMA_IF_ID, nl.Uint32Attr(ifID)))
			_, err := del.Execute(unix.NETLINK_XFRM, 0)
			return err
		}
		t.Cleanup(func() { _ = remove() })
		policies, err := b.ListPolicies()
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, got := range policies {
			if got.IfID != ifID {
				continue
			}
			found = true
			if got.SrcPort != tc.src || got.DstPort != tc.dst {
				t.Errorf("raw port matches = %+v/%+v, want %+v/%+v", got.SrcPort, got.DstPort, tc.src, tc.dst)
			}
			if got.OwnerKnown || got.Owner != "" {
				t.Errorf("foreign policy acquired owner %q", got.Owner)
			}
		}
		if !found {
			t.Fatal("raw policy missing from dump")
		}
		if err := remove(); err != nil {
			t.Fatalf("raw policy delete: %v", err)
		}
		policies, err = b.ListPolicies()
		if err != nil {
			t.Fatal(err)
		}
		for _, got := range policies {
			if got.IfID == ifID {
				t.Errorf("removed raw policy remains in dump: %+v", got)
			}
		}
	}
}

// VALIDATES: the SAD decoder respects the message family even when an IPv6
// address has the same union bytes as an IPv4 address. Both SAs share every
// other identity field, so guessing the family would merge their observations.
func TestXFRMReadbackSAAddressFamilies(t *testing.T) {
	const (
		spi  = 0x5ead0003
		ifID = 0x5ead1003
	)
	b := &xfrmBackend{}
	cases := []struct {
		src, dst                 string
		selectorSrc, selectorDst string
	}{
		{"2001:db9::", "2001:db8::", "2001:db9::/128", "2001:db8::/128"},
		{"32.1.13.185", "32.1.13.184", "32.1.13.185/32", "32.1.13.184/32"},
	}
	states := make([]*netlink.XfrmState, 0, len(cases))
	for _, tc := range cases {
		state, err := xfrmStateFromParams(SAParams{
			SPI: spi, Src: net.ParseIP(tc.src), Dst: net.ParseIP(tc.dst),
			Proto: ProtoESP, Mode: ModeTunnel, IfID: ifID, ReqID: readbackReqID,
			EncAlgo: "aes256", EncKey: make([]byte, 32),
			AuthAlgo: "sha256", AuthKey: make([]byte, 32),
			Sel: &SASelector{
				Src: mustCIDR(t, tc.selectorSrc), Dst: mustCIDR(t, tc.selectorDst),
				UpperProto: 17,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		// Ze's state selector API has no port fields; the netlink writer
		// supplies exact ports here to exercise the nested selector decoder.
		state.Selector.SrcPort, state.Selector.DstPort = 4500, 500
		if err := netlink.XfrmStateAdd(state); err != nil {
			skipUnprivileged(t, err)
			t.Fatalf("install %s SA: %v", tc.dst, err)
		}
		t.Cleanup(func() { _ = netlink.XfrmStateDel(state) })
		states = append(states, state)
	}
	check := func(want []*netlink.XfrmState) {
		t.Helper()
		observed, err := b.ListSAs(ifID)
		if err != nil {
			t.Fatal(err)
		}
		byIdentity := make(map[SAIdentity]SAInfo, len(observed))
		for _, got := range observed {
			key := IdentityOf(got.SPI, got.Dst, got.Proto, got.IfID)
			if _, exists := byIdentity[key]; exists {
				t.Errorf("distinct kernel SAs collapsed to identity %+v", key)
			}
			byIdentity[key] = got
		}
		if len(byIdentity) != len(want) {
			t.Errorf("SAD identities = %d, want %d", len(byIdentity), len(want))
		}
		for _, state := range want {
			key := IdentityOf(uint32(state.Spi), state.Dst, uint8(state.Proto), uint32(state.Ifid))
			got, ok := byIdentity[key]
			if !ok {
				t.Errorf("installed SA identity missing: %+v", key)
				continue
			}
			if !got.Src.Equal(state.Src) {
				t.Errorf("SA %s source = %s, want %s", state.Dst, got.Src, state.Src)
			}
		}
	}
	check(states)
	decoded, err := netlink.XfrmStateList(netlink.FAMILY_ALL)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range states {
		found := false
		for _, got := range decoded {
			if got.Spi != spi || got.Ifid != ifID || !got.Dst.Equal(want.Dst) {
				continue
			}
			found = true
			sel := got.Selector
			if sel == nil || sel.Src == nil || sel.Dst == nil {
				t.Fatalf("SA %s lost its nested selector", want.Dst)
			}
			if sel.Src.String() != want.Selector.Src.String() || sel.Dst.String() != want.Selector.Dst.String() {
				t.Errorf("SA %s selector = %s -> %s, want %s -> %s",
					want.Dst, sel.Src, sel.Dst, want.Selector.Src, want.Selector.Dst)
			}
			if sel.SrcPort != 4500 || sel.SrcPortMask != 0xffff || sel.DstPort != 500 || sel.DstPortMask != 0xffff {
				t.Errorf("SA %s nested ports = %d/%#x -> %d/%#x", want.Dst, sel.SrcPort, sel.SrcPortMask, sel.DstPort, sel.DstPortMask)
			}
		}
		if !found {
			t.Errorf("SA %s missing from decoded dump", want.Dst)
		}
	}
	if err := netlink.XfrmStateDel(states[0]); err != nil {
		t.Fatal(err)
	}
	check(states[1:])
	if err := netlink.XfrmStateDel(states[1]); err != nil {
		t.Fatal(err)
	}
	check(nil)
}
