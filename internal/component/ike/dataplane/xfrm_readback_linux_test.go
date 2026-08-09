// VALIDATES: the mapping from a kernel XFRM record onto SAInfo and PolicyInfo --
// every field the read surface reports, the two inversions that are not plain
// copies (direction offset, wildcard prefix), and the rule that no key material
// crosses this boundary.
// PREVENTS: a dropped, mis-scaled or offset-by-one field reaching the operator as
// a confident wrong number, and a dump failure rendering as an empty table.
//
// Design: docs/architecture/ike/ipsec-dataplane-inspection.md -- kernel dataplane read surface
// Related: xfrm_linux.go -- saInfoFromState, policyInfoFromKernel

//go:build linux

package dataplane

import (
	"errors"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
)

// mustPrefix is named apart from the integration tier's mustCIDR on purpose:
// both files carry the linux tag, so under `-tags integration` they compile
// together and a shared name is a redeclaration.
func mustPrefix(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatalf("ParseCIDR(%q): %v", s, err)
	}
	return n
}

// withStateList swaps the kernel dump seam for the duration of one test.
func withStateList(t *testing.T, fn func(int) ([]netlink.XfrmState, error)) {
	t.Helper()
	old := xfrmStateList
	xfrmStateList = fn
	t.Cleanup(func() { xfrmStateList = old })
}

func withPolicyList(t *testing.T, fn func(int) ([]netlink.XfrmPolicy, error)) {
	t.Helper()
	old := xfrmPolicyList
	xfrmPolicyList = fn
	t.Cleanup(func() { xfrmPolicyList = old })
}

func TestSAInfoFromStateCarriesKernelFields(t *testing.T) {
	s := &netlink.XfrmState{
		Src:          net.ParseIP("10.0.0.1"),
		Dst:          net.ParseIP("10.0.0.2"),
		Proto:        netlink.XFRM_PROTO_ESP,
		Mode:         netlink.XFRM_MODE_TUNNEL,
		Spi:          0x12345678,
		Reqid:        42,
		ReplayWindow: 64,
		Ifid:         7,
		Limits: netlink.XfrmStateLimits{
			ByteHard:   1 << 40,
			PacketHard: 9000,
		},
		Statistics: netlink.XfrmStateStats{
			Bytes:   4096,
			Packets: 12,
			AddTime: 1700000000,
			UseTime: 1700000500,
		},
		Crypt: &netlink.XfrmStateAlgo{Name: "cbc(aes)", Key: make([]byte, 32)},
		Auth:  &netlink.XfrmStateAlgo{Name: "hmac(sha256)", Key: make([]byte, 32)},
	}

	got := saInfoFromState(s)

	if got.SPI != 0x12345678 {
		t.Errorf("SPI = %#x, want 0x12345678", got.SPI)
	}
	if !got.Src.Equal(s.Src) || !got.Dst.Equal(s.Dst) {
		t.Errorf("addresses = %v -> %v, want %v -> %v", got.Src, got.Dst, s.Src, s.Dst)
	}
	if got.IfID != 7 {
		t.Errorf("IfID = %d, want 7", got.IfID)
	}
	if got.Proto != 50 {
		t.Errorf("Proto = %d, want 50 (ESP)", got.Proto)
	}
	// The kernel numbers tunnel mode 1 and ze numbers it 2. A missing inverse
	// reports every tunnel SA as transport, which reads as a plausible answer.
	if got.Mode != ModeTunnel {
		t.Errorf("Mode = %d, want ModeTunnel (%d)", got.Mode, ModeTunnel)
	}
	if got.ReqID != 42 {
		t.Errorf("ReqID = %d, want 42", got.ReqID)
	}
	if got.ReplayWindow != 64 {
		t.Errorf("ReplayWindow = %d, want 64", got.ReplayWindow)
	}
	if got.BytesCurrent != 4096 || got.PacketsCurrent != 12 {
		t.Errorf("current counters = %d bytes / %d packets, want 4096 / 12", got.BytesCurrent, got.PacketsCurrent)
	}
	if got.BytesHard != 1<<40 || got.PacketsHard != 9000 {
		t.Errorf("hard limits = %d bytes / %d packets, want %d / 9000", got.BytesHard, got.PacketsHard, uint64(1)<<40)
	}
	if got.Encryption != "cbc(aes)" || got.EncryptionKeyBits != 256 {
		t.Errorf("encryption = %q/%d bits, want cbc(aes)/256", got.Encryption, got.EncryptionKeyBits)
	}
	if got.Integrity != "hmac(sha256)" || got.IntegrityKeyBits != 256 {
		t.Errorf("integrity = %q/%d bits, want hmac(sha256)/256", got.Integrity, got.IntegrityKeyBits)
	}
	if want := time.Unix(1700000000, 0).UTC(); !got.AddedAt.Equal(want) {
		t.Errorf("AddedAt = %v, want %v", got.AddedAt, want)
	}
	if want := time.Unix(1700000500, 0).UTC(); !got.UsedAt.Equal(want) {
		t.Errorf("UsedAt = %v, want %v", got.UsedAt, want)
	}
}

// TestSAInfoUnusedSAHasZeroUseTime keeps "never carried a packet" distinct from
// "last used in 1970". time.Unix(0,0) renders as a real date, so the wrong answer
// here is not obviously wrong to a reader.
func TestSAInfoUnusedSAHasZeroUseTime(t *testing.T) {
	got := saInfoFromState(&netlink.XfrmState{
		Statistics: netlink.XfrmStateStats{AddTime: 1700000000, UseTime: 0},
	})
	if !got.UsedAt.IsZero() {
		t.Errorf("UsedAt = %v, want the zero time for an SA that carried nothing", got.UsedAt)
	}
	if got.AddedAt.IsZero() {
		t.Error("AddedAt must still be set when only UseTime is zero")
	}
}

// TestSAInfoAEADLeavesIntegrityEmpty pins RFC 7296 Section 3.3.2: an AEAD
// transform carries integrity itself, so the kernel fills Aead and leaves Auth
// nil. An empty Integrity is the truth about the negotiation.
func TestSAInfoAEADLeavesIntegrityEmpty(t *testing.T) {
	got := saInfoFromState(&netlink.XfrmState{
		Aead: &netlink.XfrmStateAlgo{Name: "rfc4106(gcm(aes))", Key: make([]byte, 20), ICVLen: 128},
	})
	if got.Encryption != "rfc4106(gcm(aes))" {
		t.Errorf("Encryption = %q, want the AEAD transform name", got.Encryption)
	}
	if got.EncryptionKeyBits != 160 {
		t.Errorf("EncryptionKeyBits = %d, want 160", got.EncryptionKeyBits)
	}
	if got.Integrity != "" {
		t.Errorf("Integrity = %q, want empty: an AEAD transform negotiates no separate integrity", got.Integrity)
	}
}

// TestSAInfoCarriesNoKeyMaterial is a structural guard, not an example.
//
// The kernel returns the live encryption and integrity keys on EVERY dump. The
// read surface renders its result to a terminal, a log and a `| json` pipe, so a
// byte-slice field added to SAInfo later is how a session key would leak. This
// fails on the field's existence rather than on one rendering of it.
func TestSAInfoCarriesNoKeyMaterial(t *testing.T) {
	for f := range reflect.TypeFor[SAInfo]().Fields() {
		if f.Type.Kind() == reflect.Slice && f.Type.Elem().Kind() == reflect.Uint8 {
			// net.IP is a []byte and is an address, not a secret.
			if f.Type == reflect.TypeFor[net.IP]() {
				continue
			}
			t.Errorf("SAInfo.%s is a byte slice: key material must never cross this boundary, only algorithm names and key LENGTHS", f.Name)
		}
	}
}

func TestListSAsFiltersByIfID(t *testing.T) {
	withStateList(t, func(int) ([]netlink.XfrmState, error) {
		return []netlink.XfrmState{
			{Spi: 1, Ifid: 5},
			{Spi: 2, Ifid: 9},
			{Spi: 3, Ifid: 5},
		}, nil
	})
	b := &xfrmBackend{}

	all, err := b.ListSAs(0)
	if err != nil {
		t.Fatalf("ListSAs(0): %v", err)
	}
	if len(all) != 3 {
		t.Errorf("ListSAs(0) = %d SAs, want 3: zero means every if_id", len(all))
	}

	filtered, err := b.ListSAs(5)
	if err != nil {
		t.Fatalf("ListSAs(5): %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("ListSAs(5) = %d SAs, want 2", len(filtered))
	}
	for _, sa := range filtered {
		if sa.IfID != 5 {
			t.Errorf("ListSAs(5) returned if_id %d", sa.IfID)
		}
	}
}

// TestListSAsSurfacesDumpError is the fail-closed half: a dump that failed must
// not reach the operator as an empty table (ai/rules/evidence.md).
func TestListSAsSurfacesDumpError(t *testing.T) {
	sentinel := errors.New("operation not permitted")
	withStateList(t, func(int) ([]netlink.XfrmState, error) { return nil, sentinel })
	b := &xfrmBackend{}

	sas, err := b.ListSAs(0)
	if !errors.Is(err, sentinel) {
		t.Fatalf("ListSAs error = %v, want it to wrap %v", err, sentinel)
	}
	if sas != nil {
		t.Errorf("ListSAs entries = %v, want nil beside an error", sas)
	}
}

func TestListPoliciesSurfacesDumpError(t *testing.T) {
	sentinel := errors.New("operation not permitted")
	withPolicyList(t, func(int) ([]netlink.XfrmPolicy, error) { return nil, sentinel })
	b := &xfrmBackend{}

	pols, err := b.ListPolicies()
	if !errors.Is(err, sentinel) {
		t.Fatalf("ListPolicies error = %v, want it to wrap %v", err, sentinel)
	}
	if pols != nil {
		t.Errorf("ListPolicies entries = %v, want nil beside an error", pols)
	}
}

// TestPolicyDirectionIsOffsetFromKernel pins the second inversion that is not a
// copy. netlink numbers IN/OUT/FWD from 0 and ze numbers them from 1, so a
// missing offset silently reports every outbound policy as inbound.
func TestPolicyDirectionIsOffsetFromKernel(t *testing.T) {
	for _, tc := range []struct {
		kernel netlink.Dir
		want   SADir
	}{
		{netlink.XFRM_DIR_IN, SADirIn},
		{netlink.XFRM_DIR_OUT, SADirOut},
		{netlink.XFRM_DIR_FWD, SADirFwd},
	} {
		b := &xfrmBackend{}
		got := b.policyInfoFromKernel(&netlink.XfrmPolicy{Dir: tc.kernel})
		if got.Dir != tc.want {
			t.Errorf("kernel dir %d mapped to %d, want %d", tc.kernel, got.Dir, tc.want)
		}
	}
}

func TestPolicyInfoCarriesSelectorAndTemplate(t *testing.T) {
	b := &xfrmBackend{}
	got := b.policyInfoFromKernel(&netlink.XfrmPolicy{
		Src:      mustPrefix(t, "192.168.1.0/24"),
		Dst:      mustPrefix(t, "192.168.2.0/24"),
		Proto:    netlink.Proto(89),
		SrcPort:  0,
		DstPort:  4500,
		Dir:      netlink.XFRM_DIR_OUT,
		Priority: 1000,
		Ifid:     3,
		Tmpls: []netlink.XfrmPolicyTmpl{{
			Src:   net.ParseIP("10.0.0.1"),
			Dst:   net.ParseIP("10.0.0.2"),
			Mode:  netlink.XFRM_MODE_TUNNEL,
			Reqid: 77,
		}},
	})

	if got.Src.String() != "192.168.1.0/24" || got.Dst.String() != "192.168.2.0/24" {
		t.Errorf("selector = %v -> %v", got.Src, got.Dst)
	}
	if got.UpperProto != 89 {
		t.Errorf("UpperProto = %d, want 89", got.UpperProto)
	}
	if !got.SrcPort.IsAny() {
		t.Errorf("SrcPort = %+v, want any", got.SrcPort)
	}
	if got.DstPort != ExactPortMatch(4500) {
		t.Errorf("DstPort = %+v, want an exact 4500", got.DstPort)
	}
	if got.Priority != 1000 {
		t.Errorf("Priority = %d, want 1000", got.Priority)
	}
	if got.IfID != 3 {
		t.Errorf("IfID = %d, want 3", got.IfID)
	}
	if !got.TunnelSrc.Equal(net.ParseIP("10.0.0.1")) || !got.TunnelDst.Equal(net.ParseIP("10.0.0.2")) {
		t.Errorf("tunnel endpoints = %v -> %v", got.TunnelSrc, got.TunnelDst)
	}
	if got.Mode != ModeTunnel {
		t.Errorf("Mode = %d, want ModeTunnel", got.Mode)
	}
	if got.ReqID != 77 {
		t.Errorf("ReqID = %d, want 77", got.ReqID)
	}
	if got.Action != SPActionProtect {
		t.Errorf("Action = %d, want SPActionProtect: this policy carries a template", got.Action)
	}
}

// TestPolicyBypassRecognized pins RFC 4301 Section 4.4.1 BYPASS: ALLOW with no
// template. Reporting it as a protect policy would tell the operator traffic is
// encrypted when it is in the clear.
func TestPolicyBypassRecognized(t *testing.T) {
	b := &xfrmBackend{}
	got := b.policyInfoFromKernel(&netlink.XfrmPolicy{
		Action: netlink.XFRM_POLICY_ALLOW,
		Tmpls:  nil,
	})
	if got.Action != SPActionBypass {
		t.Errorf("Action = %d, want SPActionBypass for an ALLOW policy with no template", got.Action)
	}
}

// TestPolicyOwnerJoinRoundTrips is A-7's evidence: a policy ze installed is
// found again in the ownership registry after a round trip through the kernel's
// representation, for every selector shape ze can install.
//
// The kernel drops the port MASK, and the join survives that only because
// xfrmSelectorPort refuses to install a mask other than 0 or 0xffff. Each case
// below is one of the shapes that survives.
func TestPolicyOwnerJoinRoundTrips(t *testing.T) {
	cases := []struct {
		name   string
		params SPParams
		kernel netlink.XfrmPolicy
	}{
		{
			name: "explicit prefixes, any ports",
			params: SPParams{
				Src: mustPrefix(t, "192.168.1.0/24"),
				Dst: mustPrefix(t, "192.168.2.0/24"),
				Dir: SADirOut,
			},
			kernel: netlink.XfrmPolicy{
				Src: mustPrefix(t, "192.168.1.0/24"),
				Dst: mustPrefix(t, "192.168.2.0/24"),
				Dir: netlink.XFRM_DIR_OUT,
			},
		},
		{
			name: "exact ports and an upper protocol",
			params: SPParams{
				Src:        mustPrefix(t, "10.1.0.0/16"),
				Dst:        mustPrefix(t, "10.2.0.0/16"),
				Dir:        SADirIn,
				UpperProto: 17,
				SrcPort:    ExactPortMatch(500),
				DstPort:    ExactPortMatch(4500),
			},
			kernel: netlink.XfrmPolicy{
				Src:     mustPrefix(t, "10.1.0.0/16"),
				Dst:     mustPrefix(t, "10.2.0.0/16"),
				Dir:     netlink.XFRM_DIR_IN,
				Proto:   netlink.Proto(17),
				SrcPort: 500,
				DstPort: 4500,
			},
		},
		{
			// The site-to-site case, and the one that fails without
			// normalizeSelectorPrefix: ze installs a nil selector and the kernel
			// dumps a materialized 0.0.0.0/0.
			name: "wildcard selector installed as nil, dumped as 0.0.0.0/0",
			params: SPParams{
				Src: nil,
				Dst: nil,
				Dir: SADirOut,
			},
			kernel: netlink.XfrmPolicy{
				Src: mustPrefix(t, "0.0.0.0/0"),
				Dst: mustPrefix(t, "0.0.0.0/0"),
				Dir: netlink.XFRM_DIR_OUT,
			},
		},
		{
			name: "wildcard selector, IPv6",
			params: SPParams{
				Src: nil,
				Dst: nil,
				Dir: SADirIn,
			},
			kernel: netlink.XfrmPolicy{
				Src: mustPrefix(t, "::/0"),
				Dst: mustPrefix(t, "::/0"),
				Dir: netlink.XFRM_DIR_IN,
			},
		},
		{
			name: "per-interface policy carries if_id",
			params: SPParams{
				Src:  mustPrefix(t, "172.16.0.0/12"),
				Dst:  mustPrefix(t, "172.17.0.0/16"),
				Dir:  SADirOut,
				IfID: 42,
			},
			kernel: netlink.XfrmPolicy{
				Src:  mustPrefix(t, "172.16.0.0/12"),
				Dst:  mustPrefix(t, "172.17.0.0/16"),
				Dir:  netlink.XFRM_DIR_OUT,
				Ifid: 42,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := &xfrmBackend{}
			claim := tc.params
			claim.Owner = "peer-alpha"
			if _, err := b.policies.claim(claim); err != nil {
				t.Fatalf("claim: %v", err)
			}

			got := b.policyInfoFromKernel(&tc.kernel)
			if !got.OwnerKnown {
				t.Fatalf("OwnerKnown = false: the kernel row did not resolve back to the policy ze installed")
			}
			if got.Owner != "peer-alpha" {
				t.Errorf("Owner = %q, want peer-alpha", got.Owner)
			}
		})
	}
}

// TestPolicyOwnerUnknownForForeignPolicy is the fail-closed half of the join. A
// policy another daemon installed has no owner here, and that must read as
// "not ours" rather than as a blank cell or a nearest match.
func TestPolicyOwnerUnknownForForeignPolicy(t *testing.T) {
	b := &xfrmBackend{}
	if _, err := b.policies.claim(SPParams{
		Src:   mustPrefix(t, "192.168.1.0/24"),
		Dst:   mustPrefix(t, "192.168.2.0/24"),
		Dir:   SADirOut,
		Owner: "peer-alpha",
	}); err != nil {
		t.Fatalf("claim: %v", err)
	}

	// Same direction, a DIFFERENT destination prefix. A join that matched on
	// source alone would hand this row to peer-alpha.
	got := b.policyInfoFromKernel(&netlink.XfrmPolicy{
		Src: mustPrefix(t, "192.168.1.0/24"),
		Dst: mustPrefix(t, "192.168.9.0/24"),
		Dir: netlink.XFRM_DIR_OUT,
	})
	if got.OwnerKnown {
		t.Errorf("OwnerKnown = true with owner %q: a policy ze did not install must not be attributed", got.Owner)
	}
	if got.Owner != "" {
		t.Errorf("Owner = %q, want empty beside OwnerKnown false", got.Owner)
	}
}

// TestSAInfoCountersAreNotNarrowed is the boundary case the spec's numeric table
// names: a byte counter is uint64 and a device that has moved more than 4 GiB
// through one SA must not wrap.
func TestSAInfoCountersAreNotNarrowed(t *testing.T) {
	const beyond32Bits = uint64(1)<<32 + 12345
	got := saInfoFromState(&netlink.XfrmState{
		Statistics: netlink.XfrmStateStats{Bytes: beyond32Bits, Packets: beyond32Bits},
		Limits:     netlink.XfrmStateLimits{ByteHard: ^uint64(0), PacketHard: ^uint64(0)},
	})
	if got.BytesCurrent != beyond32Bits || got.PacketsCurrent != beyond32Bits {
		t.Errorf("counters narrowed: got %d/%d, want %d", got.BytesCurrent, got.PacketsCurrent, beyond32Bits)
	}
	if got.BytesHard != ^uint64(0) || got.PacketsHard != ^uint64(0) {
		t.Errorf("hard limits narrowed: got %d/%d, want %d", got.BytesHard, got.PacketsHard, ^uint64(0))
	}
}

// TestSAInfoMaxSPI covers the top of the SPI range. SPI is uint32 on the wire
// (RFC 4303 Section 2.1) and int on netlink.XfrmState, so the conversion is where
// the top of the range would be lost.
func TestSAInfoMaxSPI(t *testing.T) {
	got := saInfoFromState(&netlink.XfrmState{Spi: int(^uint32(0))})
	if got.SPI != ^uint32(0) {
		t.Errorf("SPI = %d, want %d", got.SPI, ^uint32(0))
	}
}
