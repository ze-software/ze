package labconfig

import (
	"net/netip"
	"testing"
)

// TestRenderLinkLocalsFromTheOneGrammar renders each link-local placeholder.
//
// VALIDATES: every placeholder in LinkLocals renders to its owner's address
// under the lab prefix, the answer records which placeholders the text
// carried, and a placeholder with no IPv6 prefix is refused.
// PREVENTS: the lab and the config sweep rendering a scenario differently, and
// an unrendered placeholder reaching a daemon as an address.
func TestRenderLinkLocalsFromTheOneGrammar(t *testing.T) {
	prefix := netip.MustParsePrefix("fd00:1e:7::/64")
	text := "local " + ZeLinkLocalPlaceholder + "; peer " + FRRLinkLocalPlaceholder + ";"
	got, carried, err := RenderLinkLocals(text, prefix)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	want := "local fe80::fd00:1e:7:2; peer fe80::fd00:1e:7:3;"
	if got != want {
		t.Errorf("rendered %q, want %q", got, want)
	}
	if !carried[ZeLinkLocal] || !carried[FRRLinkLocal] || carried[FRRSinkLinkLocal] {
		t.Errorf("carried = %v, want ze and frr only", carried)
	}
	if _, _, err := RenderLinkLocals(text, netip.Prefix{}); err == nil {
		t.Error("a placeholder rendered with no IPv6 prefix")
	}
	plain := "local 192.0.2.1;"
	if got, _, err := RenderLinkLocals(plain, netip.Prefix{}); err != nil || got != plain {
		t.Errorf("plain text = %q, %v; want it unchanged", got, err)
	}
}
