// RFC: rfc/short/rfc9252.md -- Section 7, best-path ineligibility of a path without a valid SRv6 SID
// Related: rib_bestchange.go -- isSRv6Ineligible and srv6SIDValidForFamily, the producers under test
// Related: rib_commands.go -- candidateAdmitted, the gate that calls them

package rib

import (
	"bytes"
	"log/slog"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// srv6PrefixSIDWithStructure builds a Prefix-SID value holding one SRv6 L3
// Service TLV whose SID Information carries a SID Structure Sub-Sub-TLV of
// the given fields (RFC 9252 Section 3.2.1: LBL, LNL, FL, AL, Transposition
// Length, Transposition Offset).
func srv6PrefixSIDWithStructure(lbl, lnl, fl, al, transposLen, transposOff byte) []byte {
	info := []byte{0}
	info = append(info, netip.MustParseAddr("2001:db8:1::").AsSlice()...)
	info = append(info, 0, 0, 0, 0, 1, 0, 6, lbl, lnl, fl, al, transposLen, transposOff)
	sub := append([]byte{1, byte(len(info) >> 8), byte(len(info))}, info...)
	service := append([]byte{0}, sub...)
	return append([]byte{5, byte(len(service) >> 8), byte(len(service))}, service...)
}

// TestIsSRv6Ineligible_InvalidSIDStructure drives a VPNv4 UPDATE whose SID
// Structure is well-formed on the wire but fails Section 3.2.1 (sum 200),
// and checks the stored path is kept out of best-path selection.
//
// VALIDATES: RFC 9252 Section 7: the SID "is invalid when ... any of the
// conditions for the fields of the Sub-Sub-TLV, as specified in Section
// 3.2.1, is not met", and "The path having any such Prefix-SID attribute
// without any valid SRv6 SID information MUST be considered ineligible
// during the selection of the best path for the corresponding prefix."
// PREVENTS: a path with an invalid SID winning, as it did when only the
// transposition was dropped and the SID used whole.
//
// RFC requirement: RFC9252-5-1 negative -- a path whose only SRv6 SID is made invalid by its SID Structure (LBL+LNL+FL+AL above 128) is not admitted to best-path selection.
func TestIsSRv6Ineligible_InvalidSIDStructure(t *testing.T) {
	r := newTestRIBManagerWithBus(newTestEventBus())
	peer := netip.MustParseAddr("192.0.2.4")
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}

	body, nlriKey := vpnv4SRv6Update(uint32(srv6FunctionBits)<<4, srv6TransposLen)
	// Patch LNL so LBL+LNL+FL+AL = 32 + 152 + 16 + 0 = 200: same framing.
	sound := []byte{1, 0, 6, 32, 16, srv6TransposLen, 0, srv6TransposLen, srv6TransposOffset}
	at := bytes.Index(body, sound)
	if at < 0 {
		t.Fatal("fixture: SID Structure not found in the UPDATE body")
	}
	body[at+4] = 152

	feedReceived(r, peer, ctxID, body)

	if storedPathAdmitted(t, r, fam, nlriKey, peer) {
		t.Error("path with an invalid SID Structure admitted to best-path selection, want excluded")
	}
}

// TestSRv6TranspositionAtLabelWidthIsEligible is the boundary: a VPNv4 path
// whose transposition fills the 20-bit label field exactly stays eligible
// and its SID is reconstructed.
//
// VALIDATES: RFC 9252 Section 5.1: "the Transposition Length MUST be less
// than or equal to 20", so 20 is valid.
// PREVENTS: an off-by-one excluding a conforming path.
//
// RFC requirement: RFC9252-5-1 positive -- a VPNv4 path whose SID Structure transposes exactly 20 bits keeps a valid SID and is admitted to best-path selection.
func TestSRv6TranspositionAtLabelWidthIsEligible(t *testing.T) {
	r := newTestRIBManagerWithBus(newTestEventBus())
	peer := netip.MustParseAddr("192.0.2.5")
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}

	body, nlriKey := vpnv4SRv6Update(0xABCDE, 20)
	feedReceived(r, peer, ctxID, body)

	if !storedPathAdmitted(t, r, fam, nlriKey, peer) {
		t.Error("path with a 20-bit transposition excluded, want admitted")
	}
	// Bits 48..67 of the SID come back from the label: 0xABCDE.
	want := netip.MustParseAddr("2001:db8:1:abcd:e000::")
	if sid := storedSRv6SID(t, r, fam, nlriKey, peer); sid != want {
		t.Errorf("SID = %v, want %v", sid, want)
	}
}

// TestSRv6TranspositionOnFamilyWithoutLabelIsIneligible pins Section 7's zero
// rule on the families the Transposition Scheme does not apply to.
//
// VALIDATES: RFC 9252 Section 7: "The transposition offset and length MUST be
// 0 when the Sub-Sub-TLV is advertised along with routes where the
// Transposition Scheme is not applicable (e.g., for global IPv6 service
// [RFC2545] where there is no label field)."
// PREVENTS: a SID read with bits missing on a family with no label to hold them.
//
// RFC requirement: RFC9252-3.2.1-2 negative -- on IPv6 unicast and IPv6 labeled unicast a SID Structure with a non-zero Transposition Length leaves the path without a valid SID: it is not admitted.
// RFC requirement: RFC9252-3.2.1-2 positive -- the same SID Structure with Transposition Offset and Length 0 keeps the path admitted.
func TestSRv6TranspositionOnFamilyWithoutLabelIsIneligible(t *testing.T) {
	for _, fam := range []family.Family{
		{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast},
		{AFI: family.AFIIPv6, SAFI: family.SAFIMPLSLabel},
	} {
		transposed := makeRouteEntryWithOtherAttrs(buildOtherAttrsWithPrefixSID(srv6PrefixSIDWithStructure(32, 16, 16, 0, 16, 48)))
		if !isSRv6Ineligible(fam, transposed) {
			t.Errorf("%s: transposition length 16 admitted, want excluded", fam)
		}
		transposed.Release()

		whole := makeRouteEntryWithOtherAttrs(buildOtherAttrsWithPrefixSID(srv6PrefixSIDWithStructure(32, 16, 16, 0, 0, 0)))
		if isSRv6Ineligible(fam, whole) {
			t.Errorf("%s: transposition length 0 excluded, want admitted", fam)
		}
		whole.Release()
	}
}

// TestIsSRv6IneligibleAllocatesNothing pins that the per-candidate gate
// allocates nothing on a path with a valid SID.
func TestIsSRv6IneligibleAllocatesNothing(t *testing.T) {
	entry := makeRouteEntryWithOtherAttrs(buildOtherAttrsWithPrefixSID(srv6PrefixSIDWithStructure(32, 16, 16, 0, 16, 48)))
	defer entry.Release()
	allocs := testing.AllocsPerRun(100, func() {
		if isSRv6Ineligible(ipv4VPN, entry) {
			t.Fatal("valid path excluded")
		}
	})
	if allocs != 0 {
		t.Errorf("isSRv6Ineligible allocates %.1f per call, want 0", allocs)
	}
}

// TestIsSRv6IneligibleUnreadableOtherAttrsFailsClosed pins that a path whose
// OtherAttrs handle no longer resolves is excluded, and that the failure is
// logged once, not once per candidate per run.
func TestIsSRv6IneligibleUnreadableOtherAttrsFailsClosed(t *testing.T) {
	var logs bytes.Buffer
	prev := logger()
	SetLogger(slog.New(slog.NewTextHandler(&logs, nil)))
	defer SetLogger(prev)
	srv6UnreadableLogged.Store(false)

	entry := makeRouteEntryWithOtherAttrs(buildOtherAttrsWithPrefixSID(append(buildValidSRv6PrefixSID(), 0x7f, 0, 1, 0xa5)))
	defer entry.Release()
	h := entry.GetBundle().OtherAttrs
	for range 64 {
		if pool.OtherAttrs.Release(h) != nil {
			break
		}
	}
	if _, err := pool.OtherAttrs.Get(h); err == nil {
		t.Fatal("fixture: the OtherAttrs handle still resolves")
	}

	for i := range 2 {
		if !isSRv6Ineligible(ipv4VPN, entry) {
			t.Errorf("call %d: unreadable path admitted, want excluded", i)
		}
	}
	if n := strings.Count(logs.String(), "cannot be read"); n != 1 {
		t.Errorf("%d log lines, want 1:\n%s", n, logs.String())
	}
}
