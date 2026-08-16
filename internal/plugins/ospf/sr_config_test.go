// VALIDATES: spec-ospf-ext-5 config resolve + AC-1/AC-2/AC-3/AC-20/AC-21 -- the
// `segment-routing` container resolves into sr.SRConfig for both address families,
// enabling/disabling SR and populating the SRGB/SRLB ranges and node Prefix-SIDs.
// PREVENTS: an empty block enabling SR; SRGB/SRLB overlap passing validation.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
)

func TestParseSegmentRoutingResolves(t *testing.T) {
	m := map[string]any{
		"enable":          true,
		"srgb":            map[string]any{"lower-bound": float64(16000), "upper-bound": float64(23999)},
		"srlb":            map[string]any{"lower-bound": float64(40000), "upper-bound": float64(40999)},
		"srms-preference": float64(128),
		"prefix-sid": map[string]any{
			"10.0.0.1/32": map[string]any{"index": float64(1), "node-sid": true, "no-php": true},
		},
	}
	cfg := parseSegmentRouting(m)
	if cfg == nil || !cfg.Enabled {
		t.Fatalf("SR must be enabled: %+v", cfg)
	}
	if len(cfg.SRGB) != 1 || cfg.SRGB[0].Base != 16000 || cfg.SRGB[0].Size != 8000 {
		t.Fatalf("SRGB = %+v want base 16000 size 8000", cfg.SRGB)
	}
	if len(cfg.SRLB) != 1 || cfg.SRLB[0].Base != 40000 || cfg.SRLB[0].Size != 1000 {
		t.Fatalf("SRLB = %+v want base 40000 size 1000", cfg.SRLB)
	}
	if !cfg.HasSRMS || cfg.SRMSPreference != 128 {
		t.Fatalf("SRMS = %v/%d", cfg.HasSRMS, cfg.SRMSPreference)
	}
	if len(cfg.Prefixes) != 1 {
		t.Fatalf("prefixes = %+v", cfg.Prefixes)
	}
	p := cfg.Prefixes[0]
	if p.Prefix != netip.MustParsePrefix("10.0.0.1/32") || p.Index != 1 || !p.NodeSID || !p.NoPHP {
		t.Fatalf("prefix-SID = %+v", p)
	}
}

func TestParseSegmentRoutingEmptyIsInert(t *testing.T) {
	// An absent or empty block yields nil (SR off, AC-20).
	if parseSegmentRouting(nil) != nil {
		t.Fatalf("nil map must yield nil SR config")
	}
	if parseSegmentRouting(map[string]any{}) != nil {
		t.Fatalf("empty map must yield nil SR config")
	}
}

func TestValidateSRConfigRejectsOverlap(t *testing.T) {
	// SRGB overlapping SRLB is rejected (AC-21, R-5).
	cfg := &sr.SRConfig{
		Enabled: true,
		SRGB:    []sr.LabelRange{{Base: 16000, Size: 8000}},
		SRLB:    []sr.LabelRange{{Base: 20000, Size: 1000}},
	}
	if err := cfg.Validate(nil); err == nil {
		t.Fatalf("overlapping SRGB/SRLB must fail validation")
	}
}

func TestApplySRConfigPopulatesStore(t *testing.T) {
	srTestReset(t)
	router := types.RouterID{10, 0, 0, 1}
	sections := []configSection{{Root: "ospf", Data: `{"ospf":{"segment-routing":{"enable":true,"srgb":{"lower-bound":16000,"upper-bound":23999}}}}`}}
	cfg := ospfConfig{RouterID: router}
	applySRConfig(sections, cfg)
	got, ok := srWire.get(router)
	if !ok || !got.Enabled || len(got.SRGB) != 1 || got.SRGB[0].Base != 16000 {
		t.Fatalf("srWire not populated: %+v ok=%v", got, ok)
	}
	// Removing SR from config clears the store entry (AC-20).
	empty := []configSection{{Root: "ospf", Data: `{"ospf":{}}`}}
	applySRConfig(empty, cfg)
	if _, ok := srWire.get(router); ok {
		t.Fatalf("srWire must be cleared when SR config removed")
	}
}

func TestApplySRConfigIPv6Block(t *testing.T) {
	srTestReset(t)
	router := types.RouterID{10, 0, 0, 2}
	// Only the IPv6 address-family enables SR; the shared RI capabilities use it.
	sections := []configSection{{Root: "ospf", Data: `{"ospf":{"address-family":{"ipv6":{"segment-routing":{"enable":true,"srgb":{"lower-bound":18000,"upper-bound":18099}}}}}}`}}
	cfg := ospfConfig{RouterID: router}
	applySRConfig(sections, cfg)
	got, ok := srWire.get(router)
	if !ok || !got.Enabled || got.SRGB[0].Base != 18000 {
		t.Fatalf("IPv6 SR block not applied: %+v ok=%v", got, ok)
	}
}

func TestApplySRConfigDualStackOriginatesBothPrefixSIDs(t *testing.T) {
	// FIX-2 regression: when BOTH address families configure SR, the store must keep the
	// IPv6 block's own config so the IPv6 node Prefix-SID is originated in the
	// E-Intra-Area-Prefix-LSA, while the IPv4 Prefix-SID still originates from the shared
	// (IPv4-preferred) config (RFC 8666 §6 -- each family advertises its own prefixes).
	srTestReset(t)
	router := types.RouterID{10, 0, 0, 7}
	v4loop := netip.MustParsePrefix("10.0.0.7/32")
	v6loop := netip.MustParsePrefix("2001:db8::7/128")
	sections := []configSection{{Root: "ospf", Data: `{"ospf":{` +
		`"segment-routing":{"enable":true,"srgb":{"lower-bound":16000,"upper-bound":23999},` +
		`"prefix-sid":{"10.0.0.7/32":{"index":1,"node-sid":true}}},` +
		`"address-family":{"ipv6":{"segment-routing":{"enable":true,"srgb":{"lower-bound":18000,"upper-bound":18099},` +
		`"prefix-sid":{"2001:db8::7/128":{"index":2,"node-sid":true}}}}}}}`}}
	applySRConfig(sections, ospfConfig{RouterID: router})

	// IPv4 Prefix-SID still originates from the shared config.
	v4subs := srBuildPrefixSID(extSubTLVContext{Router: router, Prefix: v4loop})
	if len(v4subs) != 1 {
		t.Fatalf("IPv4 Prefix-SID sub-TLV must still originate, got %+v", v4subs)
	}
	if ps, err := sr.DecodePrefixSIDValue(v4subs[0].Value); err != nil || ps.Index != 1 {
		t.Fatalf("IPv4 Prefix-SID index = %+v, err %v (want index 1)", ps, err)
	}

	// The shared config alone (the pre-fix path) carries no IPv6 prefix; the IPv6 override
	// is what makes the IPv6 node Prefix-SID originate.
	if shared, _ := srWire.get(router); len(v6NodePrefixSIDs(shared)) != 0 {
		t.Fatalf("shared config must not carry the IPv6 prefix (that is the IPv4 block)")
	}
	v6cfg, ok := srWire.getAF(router, true)
	if !ok || !v6cfg.Enabled {
		t.Fatalf("IPv6 SR config override missing: ok=%v cfg=%+v", ok, v6cfg)
	}
	v6sids := v6NodePrefixSIDs(v6cfg)
	if len(v6sids) != 1 || v6sids[0].Prefix != v6loop || v6sids[0].Index != 2 {
		t.Fatalf("IPv6 node Prefix-SIDs = %+v, want [%v idx 2]", v6sids, v6loop)
	}

	// The v6 Prefix-SID is carried in the E-Intra-Area-Prefix-LSA body.
	body := v6EIntraAreaPrefixBody(router, v6sids)
	ext, err := ospfv3packet.DecodeExtendedLSABody(body[eIntraPrefixHeaderLen:])
	if err != nil || len(ext.TLVs) != 1 {
		t.Fatalf("E-Intra-Area-Prefix body TLVs = %+v, err %v", ext.TLVs, err)
	}
	gotPfx, ps, ok := v6PrefixSIDFromTLV(ext.TLVs[0])
	if !ok || gotPfx != v6loop || ps.Index != 2 {
		t.Fatalf("originated IPv6 Prefix-SID = %v/%+v ok=%v, want %v idx 2", gotPfx, ps, ok, v6loop)
	}
}

func TestApplySRConfigClearsStaleV6OverrideOnReconfigure(t *testing.T) {
	// FIX-2 regression (re-review finding): configuring dual-stack SR stores an IPv6
	// override; later removing ONLY the IPv6 SR block while IPv4 SR stays enabled must
	// clear that override, otherwise getAF(v6) keeps returning the withdrawn IPv6 node
	// Prefix-SID and the E-Intra-Area-Prefix-LSA keeps advertising it. This reconfigures
	// in place (NO srTestReset between the two applies) because a reset would mask the bug.
	srTestReset(t)
	router := types.RouterID{10, 0, 0, 7}
	v6loop := netip.MustParsePrefix("2001:db8::7/128")

	dualStack := []configSection{{Root: "ospf", Data: `{"ospf":{` +
		`"segment-routing":{"enable":true,"srgb":{"lower-bound":16000,"upper-bound":23999},` +
		`"prefix-sid":{"10.0.0.7/32":{"index":1,"node-sid":true}}},` +
		`"address-family":{"ipv6":{"segment-routing":{"enable":true,"srgb":{"lower-bound":18000,"upper-bound":18099},` +
		`"prefix-sid":{"2001:db8::7/128":{"index":2,"node-sid":true}}}}}}}`}}
	applySRConfig(dualStack, ospfConfig{RouterID: router})

	// Precondition: the dual-stack override is present.
	if v6cfg, ok := srWire.getAF(router, true); !ok || len(v6NodePrefixSIDs(v6cfg)) != 1 {
		t.Fatalf("precondition: dual-stack must store the IPv6 override, got ok=%v cfg=%+v", ok, v6cfg)
	}

	// Reconfigure IN PLACE: IPv4 SR stays, the IPv6 SR block is removed.
	v4only := []configSection{{Root: "ospf", Data: `{"ospf":{` +
		`"segment-routing":{"enable":true,"srgb":{"lower-bound":16000,"upper-bound":23999},` +
		`"prefix-sid":{"10.0.0.7/32":{"index":1,"node-sid":true}}}}}`}}
	applySRConfig(v4only, ospfConfig{RouterID: router})

	// The stale IPv6 override must be gone: getAF(v6) now falls back to the shared IPv4
	// block, which carries no IPv6 prefix.
	v6cfg, ok := srWire.getAF(router, true)
	if !ok {
		t.Fatalf("IPv4 SR still enabled, getAF(v6) must return the shared config")
	}
	if got := v6NodePrefixSIDs(v6cfg); len(got) != 0 {
		t.Fatalf("stale IPv6 override not cleared: getAF(v6) still yields %+v (want none)", got)
	}

	// The originated E-Intra-Area-Prefix-LSA must no longer advertise the withdrawn v6 SID.
	body := v6EIntraAreaPrefixBody(router, v6NodePrefixSIDs(v6cfg))
	ext, err := ospfv3packet.DecodeExtendedLSABody(body[eIntraPrefixHeaderLen:])
	if err != nil {
		t.Fatalf("decode E-Intra-Area-Prefix body: %v", err)
	}
	for _, tlv := range ext.TLVs {
		if gotPfx, _, ok := v6PrefixSIDFromTLV(tlv); ok && gotPfx == v6loop {
			t.Fatalf("withdrawn IPv6 Prefix-SID %v still originated after removing the IPv6 SR block", v6loop)
		}
	}
}

func TestValidateSRConfigSectionsRejectsBadRange(t *testing.T) {
	sections := []configSection{{Root: "ospf", Data: `{"ospf":{"segment-routing":{"enable":true,"srgb":{"lower-bound":16000,"upper-bound":23999},"srlb":{"lower-bound":20000,"upper-bound":20999}}}}`}}
	if err := validateSRConfig(sections); err == nil {
		t.Fatalf("overlapping SRGB/SRLB in config must be rejected")
	}
}
