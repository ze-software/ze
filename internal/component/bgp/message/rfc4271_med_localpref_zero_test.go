package message

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// findPathAttr walks a path attribute block and returns the value of the first
// attribute with the given type code, and whether one was present.
func findPathAttr(t *testing.T, attrs []byte, code attribute.AttributeCode) ([]byte, bool) {
	t.Helper()
	for off := 0; off < len(attrs); {
		if off+3 > len(attrs) {
			t.Fatalf("truncated attribute header at offset %d in %x", off, attrs)
		}
		flags, typ := attrs[off], attrs[off+1]
		hdr, n := 3, int(attrs[off+2])
		if flags&0x10 != 0 {
			if off+4 > len(attrs) {
				t.Fatalf("truncated extended-length header at offset %d in %x", off, attrs)
			}
			hdr, n = 4, int(binary.BigEndian.Uint16(attrs[off+2:]))
		}
		if off+hdr+n > len(attrs) {
			t.Fatalf("attribute %d overruns the block at offset %d in %x", typ, off, attrs)
		}
		if attribute.AttributeCode(typ) == code {
			return attrs[off+hdr : off+hdr+n], true
		}
		off += hdr + n
	}
	return nil, false
}

// medLocalPrefCase is one presence combination and the wire it owes.
type medLocalPrefCase struct {
	name      string
	med       uint32
	hasMED    bool
	localPref uint32
	hasLP     bool
	wantMED   []byte // nil = MULTI_EXIT_DISC absent
	wantLP    []byte // LOCAL_PREF on the iBGP session
}

var medLocalPrefCases = []medLocalPrefCase{
	{name: "local-preference 0", hasLP: true, wantLP: []byte{0, 0, 0, 0}},
	{name: "med 0", hasMED: true, wantMED: []byte{0, 0, 0, 0}, wantLP: []byte{0, 0, 0, 100}},
	{name: "neither set", wantLP: []byte{0, 0, 0, 100}},
	{name: "both set", med: 7, hasMED: true, localPref: 250, hasLP: true, wantMED: []byte{0, 0, 0, 7}, wantLP: []byte{0, 0, 0, 250}},
}

// checkMEDLocalPref asserts the MULTI_EXIT_DISC and LOCAL_PREF one builder wrote.
func checkMEDLocalPref(t *testing.T, tc medLocalPrefCase, attrs []byte, checkMED bool) {
	t.Helper()
	if checkMED {
		med, ok := findPathAttr(t, attrs, attribute.AttrMED)
		if tc.wantMED == nil {
			if ok {
				t.Errorf("MULTI_EXIT_DISC %x present, want absent", med)
			}
		} else if !ok || !bytes.Equal(med, tc.wantMED) {
			t.Errorf("MULTI_EXIT_DISC = %x (present %v), want %x", med, ok, tc.wantMED)
		}
	}
	lp, ok := findPathAttr(t, attrs, attribute.AttrLocalPref)
	if !ok || !bytes.Equal(lp, tc.wantLP) {
		t.Errorf("LOCAL_PREF = %x (present %v), want %x", lp, ok, tc.wantLP)
	}
}

// TestBuilders_MEDAndLocalPrefZeroAreValues proves that every UPDATE builder
// reads presence from the Has flags and never from the value.
//
// VALIDATES: RFC 4271 Sections 5.1.4 and 5.1.5. A configured LOCAL_PREF of 0
// reaches an iBGP peer as 0, a configured MED of 0 is emitted, an unset
// LOCAL_PREF is the default 100 and an unset MED is absent.
// PREVENTS: a builder treating 0 as "not configured", which sent LOCAL_PREF 0
// as 100 and dropped MED 0 (found by comparison with VyOS T9404).
func TestBuilders_MEDAndLocalPrefZeroAreValues(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	nextHop := netip.MustParseAddr("192.168.1.1")

	for _, tc := range medLocalPrefCases {
		t.Run("unicast/"+tc.name, func(t *testing.T) {
			ub := NewUpdateBuilder(65001, true, true, false)
			upd := mustBuildUnicast(t, ub, &UnicastParams{
				Prefix: prefix, NextHop: nextHop, Origin: attribute.OriginIGP,
				MED: tc.med, HasMED: tc.hasMED,
				LocalPreference: tc.localPref, HasLocalPreference: tc.hasLP,
			})
			checkMEDLocalPref(t, tc, upd.PathAttributes, true)
		})
		t.Run("grouped/"+tc.name, func(t *testing.T) {
			ub := NewUpdateBuilder(65001, true, true, false)
			upd := mustBuildGrouped(t, ub, []UnicastParams{{
				Prefix: prefix, NextHop: nextHop, Origin: attribute.OriginIGP,
				MED: tc.med, HasMED: tc.hasMED,
				LocalPreference: tc.localPref, HasLocalPreference: tc.hasLP,
			}})
			checkMEDLocalPref(t, tc, upd.PathAttributes, true)
		})
		t.Run("labeled/"+tc.name, func(t *testing.T) {
			ub := NewUpdateBuilder(65001, true, true, false)
			upd := ub.BuildLabeledUnicast(&LabeledUnicastParams{
				Prefix: prefix, NextHop: nextHop, Origin: attribute.OriginIGP, Labels: []uint32{100},
				MED: tc.med, HasMED: tc.hasMED,
				LocalPreference: tc.localPref, HasLocalPreference: tc.hasLP,
			})
			checkMEDLocalPref(t, tc, upd.PathAttributes, true)
		})
		t.Run("vpn/"+tc.name, func(t *testing.T) {
			ub := NewUpdateBuilder(65001, true, true, false)
			upd := ub.BuildVPN(&VPNParams{
				Prefix: prefix, NextHop: nextHop, Origin: attribute.OriginIGP, Labels: []uint32{100},
				RDBytes: [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
				MED:     tc.med, HasMED: tc.hasMED,
				LocalPreference: tc.localPref, HasLocalPreference: tc.hasLP,
			})
			checkMEDLocalPref(t, tc, upd.PathAttributes, true)
		})
		t.Run("plugin/"+tc.name, func(t *testing.T) {
			ub := NewUpdateBuilder(65001, true, true, false)
			upd := ub.BuildPlugin(PluginParams{
				AFI: 1, SAFI: 5, NLRI: []byte{1, 2, 3}, NextHop: nextHop,
				LocalPreference: tc.localPref, HasLocalPreference: tc.hasLP,
			})
			// The plugin rail carries MED in RawAttrs, built by the family plugin.
			checkMEDLocalPref(t, tc, upd.PathAttributes, false)
		})
	}
}

// TestBuilders_LocalPrefZeroStaysOffEBGP proves that a configured LOCAL_PREF,
// 0 included, still never reaches an external peer.
//
// VALIDATES: RFC 4271 Section 5.1.5, "A BGP speaker MUST NOT include this
// attribute in UPDATE messages it sends to external peers".
// PREVENTS: the presence flag being read as permission to send on eBGP.
func TestBuilders_LocalPrefZeroStaysOffEBGP(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)
	upd := mustBuildUnicast(t, ub, &UnicastParams{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("192.168.1.1"),
		Origin:  attribute.OriginIGP, HasLocalPreference: true,
		HasMED: true,
	})
	if lp, ok := findPathAttr(t, upd.PathAttributes, attribute.AttrLocalPref); ok {
		t.Errorf("LOCAL_PREF %x sent to an external peer", lp)
	}
	if med, ok := findPathAttr(t, upd.PathAttributes, attribute.AttrMED); !ok || !bytes.Equal(med, []byte{0, 0, 0, 0}) {
		t.Errorf("MULTI_EXIT_DISC = %x (present %v), want 00000000", med, ok)
	}
}
