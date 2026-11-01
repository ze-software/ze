package reactor

import (
	"bytes"
	"encoding/hex"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The intent test attachments and filter type: a member that reads prefix and
// asks whatever libraryIntentEval asks. The tests run one at a time and set
// libraryIntentEval before each run.
const (
	libraryIntentImport = "reactor-library-intent-test import"
	libraryIntentExport = "reactor-library-intent-test export"
	libraryTypeIntents  = "reactor-library-test-intents"
)

var libraryIntentEval func(*policyview.View, *policychain.Sets) error

func init() {
	reads := []routepolicy.Field{routepolicy.FieldPrefix}
	sets := []routepolicy.Field{routepolicy.FieldASPath, routepolicy.FieldPrefix, routepolicy.FieldFamily}
	routepolicy.MustRegisterAttachment("bgp", libraryIntentImport, reads, sets)
	routepolicy.MustRegisterAttachment("bgp", libraryIntentExport, reads, sets)
	routepolicy.MustRegisterFilterType(libraryTypeIntents, reads, sets)
	prefix, err := policyview.ReaderFor(routepolicy.FieldPrefix)
	if err != nil {
		panic("BUG: reactor test: no prefix reader: " + err.Error())
	}
	policychain.MustRegisterEvaluator(libraryTypeIntents, rpc.OnErrorReject, func(string, map[string]any, routepolicy.InstanceResolver) (policychain.Evaluator, error) {
		return func(view *policyview.View, sets *policychain.Sets) (policychain.Decision, error) {
			if _, err := view.Read(prefix); err != nil {
				return policychain.DecisionUnspecified, err
			}
			return policychain.DecisionPass, libraryIntentEval(view, sets)
		}, nil
	})
}

// dropCIDRs answers an evaluator body that drops, from the body NLRI and the
// MP_REACH_NLRI the view answers for prefix, every entry whose CIDR bytes are
// in drop (hex). It finds the offsets by walking the sections as a member
// would, so the test proves the offsets the view hands a member are the ones
// the reactor applies.
func dropCIDRs(t *testing.T, drop ...string) func(*policyview.View, *policychain.Sets) error {
	t.Helper()
	prefix, err := policyview.ReaderFor(routepolicy.FieldPrefix)
	if err != nil {
		t.Fatalf("prefix reader: %v", err)
	}
	walk := func(sets *policychain.Sets, section policychain.NLRISection, raw []byte) error {
		for off := 0; off < len(raw); {
			_, cidr, next, ok := wireu.CIDREntry(raw, off, false)
			if !ok {
				t.Fatalf("section %d does not frame at %d", section, off)
			}
			if slices.Contains(drop, hex.EncodeToString(cidr)) {
				if err := sets.DropNLRI(section, uint16(off)); err != nil {
					return err
				}
			}
			off = next
		}
		return nil
	}
	return func(view *policyview.View, sets *policychain.Sets) error {
		value, err := view.Read(prefix)
		if err != nil {
			return err
		}
		if err := walk(sets, policychain.NLRISectionAnnounce, value.Wire()); err != nil {
			return err
		}
		if mp := value.MPReach(); mp != nil {
			return walk(sets, policychain.NLRISectionMPReach, mp.NLRIBytes())
		}
		return nil
	}
}

// runLibraryImport runs the import step over body with one intent member.
func runLibraryImport(t *testing.T, body []byte) ingressStepResult {
	t.Helper()
	family.RegisterTestFamilies()
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
	addr := netip.MustParseAddr("10.0.0.4")
	peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001,
		ImportPolicy: libraryPolicy(t, libraryIntentImport, libraryTypeIntents, routepolicy.FinalAccept)})
	return r.runIngressPolicyChain(peer, addr, testWireUpdate(body), body)
}

// TestLibraryNLRIDropMatchesTextSubset proves the NLRI subset intent at UPDATE
// bytes: a library member dropping one IPv4 and one IPv6 prefix by offset
// makes the import step answer the same body the text path's prefix-list
// answer builds, MP_REACH_NLRI included, and the body each case states.
//
// PREVENTS: the library path rewriting only the body NLRI and copying
// MP_REACH_NLRI verbatim (plan/journal/guard-added-to-one-half-of-a-pair.md).
func TestLibraryNLRIDropMatchesTextSubset(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		drop     []string // prefixes, for the text path
		dropCIDR []string // the same prefixes as CIDR bytes, for the member
		want     string
	}{
		{
			name: "legacy-and-mp-ipv6",
			body: "0000 003a 40010100 40020602010000FDE9 40030401010101" +
				" 800E23 0002 01 10 20010db8000000000000000000000001 00 3020010db80001 3020010db80002" +
				" 180a0001 180a0009",
			drop:     []string{"2001:db8:1::/48", "10.0.9.0/24"},
			dropCIDR: []string{"3020010db80001", "180a0009"},
			want: "0000 0033 800E1c 0002 01 10 20010db8000000000000000000000001 00 3020010db80002" +
				" 40010100 40020602010000FDE9 40030401010101 180a0001",
		},
		{
			name: "mp-emptied-drops-attribute",
			body: "0000 0033 40010100 40020602010000FDE9 40030401010101" +
				" 800E1c 0002 01 10 20010db8000000000000000000000001 00 3020010db80001 180a0001",
			drop:     []string{"2001:db8:1::/48"},
			dropCIDR: []string{"3020010db80001"},
			want:     "0000 0014 40010100 40020602010000FDE9 40030401010101 180a0001",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := nlriHex(t, tt.body)
			want := nlriHex(t, tt.want)
			text := runImportDrop(t, body, tt.drop...)
			if !text.accept || !bytes.Equal(text.modifiedPayload, want) {
				t.Fatalf("text path = accept %v body %x, want %x", text.accept, text.modifiedPayload, want)
			}
			libraryIntentEval = dropCIDRs(t, tt.dropCIDR...)
			got := runLibraryImport(t, body)
			if !got.accept || !bytes.Equal(got.modifiedPayload, want) {
				t.Fatalf("library = accept %v body %x, want %x", got.accept, got.modifiedPayload, want)
			}
		})
	}
}

// TestLibraryNLRIDropRefusesStrayOffset proves an offset that starts no NLRI
// entry, and a drop from a section the UPDATE does not carry, drop the route
// rather than send prefixes the chain asked out.
func TestLibraryNLRIDropRefusesStrayOffset(t *testing.T) {
	body := nlriHex(t, "0000 0014 40010100 40020602010000FDE9 40030401010101 180a0001 180a0009")
	for name, ask := range map[string]func(*policychain.Sets) error{
		"inside-an-entry": func(s *policychain.Sets) error { return s.DropNLRI(policychain.NLRISectionAnnounce, 1) },
		"past-the-end":    func(s *policychain.Sets) error { return s.DropNLRI(policychain.NLRISectionAnnounce, 8) },
		"no-mp-section":   func(s *policychain.Sets) error { return s.DropNLRI(policychain.NLRISectionMPReach, 0) },
	} {
		t.Run(name, func(t *testing.T) {
			libraryIntentEval = func(_ *policyview.View, s *policychain.Sets) error { return ask(s) }
			if got := runLibraryImport(t, body); got.accept {
				t.Fatalf("import = %+v, want the route dropped", got)
			}
		})
	}
}

// TestFamilyStripIntent proves AC-26's in-process half: stripping one MP
// family of an UPDATE carrying two removes that family's MP_REACH_NLRI and
// MP_UNREACH_NLRI and keeps the other family, giving the body filter_family's
// raw rewrite (stripMPAttrs) produces for it: the same attributes with types
// 14 and 15 cut out and the attribute length recomputed. Stripping IPv4
// unicast instead empties the body NLRI and keeps the MP family.
func TestFamilyStripIntent(t *testing.T) {
	body := "0000 0040 40010100 40020602010000FDE9 40030401010101" +
		" 800E1c 0002 01 10 20010db8000000000000000000000001 00 3020010db80001" +
		" 800F0a 0002 01 3020010db80002" +
		" 180a0001"
	tests := []struct {
		name  string
		strip family.Family
		want  string
	}{
		{
			name:  "ipv6-unicast",
			strip: family.IPv6Unicast,
			want:  "0000 0014 40010100 40020602010000FDE9 40030401010101 180a0001",
		},
		{
			name:  "ipv4-unicast",
			strip: family.IPv4Unicast,
			// The builder writes the kept MP attributes first, as it does for
			// the text path's subset (RFC 7606 Section 5.1).
			want: "0000 0040 800F0a 0002 01 3020010db80002" +
				" 800E1c 0002 01 10 20010db8000000000000000000000001 00 3020010db80001" +
				" 40010100 40020602010000FDE9 40030401010101",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			libraryIntentEval = func(_ *policyview.View, s *policychain.Sets) error { return s.StripFamily(tt.strip) }
			got := runLibraryImport(t, nlriHex(t, body))
			want := strings.ToLower(strings.ReplaceAll(tt.want, " ", ""))
			if !got.accept || hex.EncodeToString(got.modifiedPayload) != want {
				t.Fatalf("import = accept %v body %x, want %s", got.accept, got.modifiedPayload, want)
			}
		})
	}
}

// TestLibraryASPathIntentsMatchText proves the AS-path intents at UPDATE
// bytes on both steps: remove-private strip and a prepend of 2 give the body
// the text delta's directives give through the same builder, with the path
// each case states; toward an internal peer on export the AS_PATH is left
// alone (RFC 4271 Section 5.1.2).
func TestLibraryASPathIntentsMatchText(t *testing.T) {
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
	libraryIntentEval = func(_ *policyview.View, s *policychain.Sets) error {
		if err := s.RemovePrivateAS(policychain.RemovePrivateStrip); err != nil {
			return err
		}
		return s.PrependASPath(2)
	}
	textBody := func(t *testing.T, received []byte, asn4 bool, localAS, peerAS uint32) []byte {
		t.Helper()
		attrs, _ := testWireUpdate(received).Attrs()
		var mods filterapi.ModAccumulator
		modAttrs := parseFilterAttrs("remove-private strip as-path-prepend 2")
		ExtractRemovePrivateASOps(newTestScratch(t), modAttrs, attrs, asn4, peerAS, &mods)
		ExtractASPathPrependOps(newTestScratch(t), modAttrs, attrs, asn4, localAS, &mods)
		body, _, fail := buildModifiedPayload(received, &mods, r.attrModHandlers, nil, nil)
		if fail.failed() {
			t.Fatalf("text-path build failed")
		}
		return body
	}

	t.Run("import-two-octet", func(t *testing.T) {
		// AS_PATH AS_SEQUENCE 3 64512 174 at two octets; 64512 is private.
		received := nlriHex(t, "0000 0016 40010100 4002080203 0003 fc00 00ae 40030401010101 180a0001")
		// AS_SEQUENCE 65000 65000 3 174: the prepend joins the leading
		// AS_SEQUENCE (RFC 4271 Section 5.1.2 b 1).
		want := nlriHex(t, "0000 0018 40010100 40020a 0204 fde8 fde8 0003 00ae 40030401010101 180a0001")
		if text := textBody(t, received, false, 65000, 65001); !bytes.Equal(text, want) {
			t.Fatalf("text path = %x, want %x", text, want)
		}
		got := runLibraryImport(t, received)
		if !got.accept || !bytes.Equal(got.modifiedPayload, want) {
			t.Fatalf("library = accept %v body %x, want %x", got.accept, got.modifiedPayload, want)
		}
	})

	exportPolicy := func(t *testing.T) filterapi.Policy {
		return libraryPolicy(t, libraryIntentExport, libraryTypeIntents, routepolicy.FinalAccept)
	}
	// AS_PATH AS_SEQUENCE 3356 4200000000 at four octets; the second is private.
	received := nlriHex(t, "0000 0018 40010100 40020a0202 00000d1c fa56ea00 40030401010101 180a0001")

	t.Run("export-external-four-octet", func(t *testing.T) {
		// AS_SEQUENCE 65000 65000 3356, one segment.
		want := nlriHex(t, "0000 001c 40010100 40020e 0203 0000fde8 0000fde8 00000d1c 40030401010101 180a0001")
		if text := textBody(t, received, true, 65000, 65002); !bytes.Equal(text, want) {
			t.Fatalf("text path = %x, want %x", text, want)
		}
		out := r.runEgressPolicyChainASN4(exportPolicy(t), "10.0.0.5", 65002, 65000, false, testWireUpdate(received), true)
		if !out.accept || out.wireOverride == nil || !bytes.Equal(out.wireOverride.Payload(), want) {
			t.Fatalf("export = %+v, want accept with %x", out, want)
		}
	})

	t.Run("export-internal-untouched", func(t *testing.T) {
		out := r.runEgressPolicyChainASN4(exportPolicy(t), "10.0.0.5", 65000, 65000, true, testWireUpdate(received), true)
		if !out.accept || out.wireOverride != nil {
			t.Fatalf("export = %+v, want accept with the AS_PATH unchanged", out)
		}
	})
}
