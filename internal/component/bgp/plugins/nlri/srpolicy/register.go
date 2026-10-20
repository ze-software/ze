// Design: docs/architecture/wire/nlri.md -- SR-Policy NLRI plugin registration
// RFC: rfc/short/rfc9830.md -- SAFI 73 (SR-Policy)
// Related: types.go -- SRPolicy NLRI type
// Related: split.go -- NLRI splitter and receive syntax rule
// Related: engine.go -- plugin engine the daemon starts for SR-Policy families
// Related: attr_json.go -- Tunnel Encapsulation rendering for the offline decoder

package srpolicy

import (
	"log/slog"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisyntax"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// Family registrations for SR-Policy (SAFI 73).
var (
	IPv4SRPolicy = family.MustRegister(family.AFIIPv4, family.SAFISRPolicy, "ipv4", "sr-policy")
	IPv6SRPolicy = family.MustRegister(family.AFIIPv6, family.SAFISRPolicy, "ipv6", "sr-policy")
)

func init() {
	nlrisplit.Register(IPv4SRPolicy, SplitSRPolicy)
	nlrisplit.Register(IPv6SRPolicy, SplitSRPolicy)

	// RFC 9830 Section 5: the receive path judges every SAFI 73 NLRI section through
	// this rule (message.validateMPNLRISyntax) and removes the malformed NLRIs
	// (reactor.typedNLRIEdit). The NLRI Length is fixed per AFI (Section 2.1).
	for _, one := range []struct {
		fam      family.Family
		nlriBits int
	}{{IPv4SRPolicy, nlriBitsIPv4}, {IPv6SRPolicy, nlriBitsIPv6}} {
		if err := nlrisyntax.Register(one.fam, srPolicySyntaxRule(one.nlriBits)); err != nil {
			slog.Error("srpolicy: NLRI syntax rule registration failed", "family", one.fam, "error", err)
			os.Exit(1)
		}
	}

	reg := registry.Registration{
		Name:                 "bgp-nlri-srpolicy",
		Description:          "SR-Policy family plugin (RFC 9830, SAFI 73)",
		SupportsNLRI:         true,
		Features:             "nlri",
		Families:             []string{IPv4SRPolicy.String(), IPv6SRPolicy.String()},
		InProcessNLRIDecoder: DecodeNLRIHex,
		InProcessNLRIEncoder: EncodeNLRIHex,
		// Ze's own rule, not an RFC one: RFC 9830 says nothing about ADD-PATH,
		// and DecodeNLRIHex accepts a received path identifier (RFC 7911). The
		// SR-Policy route Ze builds carries none (SRPolicy.SupportsAddPath), so
		// `update text` refuses one rather than dropping it.
		NLRIAddPathRefusal:    "Ze builds SR-Policy routes without an ADD-PATH path identifier",
		InProcessRouteEncoder: EncodeRoute,
		// RFC 9830 Section 2.4.2 and Section 2.4.3: the Tunnel Encapsulation
		// attribute (type 23) carries the SR Policy's Binding SID sub-TLVs, and
		// this plugin renders them for the offline decoder, which names no plugin.
		AttributeJSONDecoders: map[uint8]registry.AttributeJSONDecoder{
			attrCodeTunnelEncap: {Key: "tunnel-encapsulation", Decode: TunnelEncapToJSON},
		},
		InProcessConfigRouteParser: parseConfigRoute,
		// RFC 9830 Section 2.2: "The content of the SR Policy CP is encoded in
		// the Tunnel Encapsulation Attribute", so an API section writes the
		// attribute's sub-TLVs after the NLRI fields and goes to
		// parseConfigRoute, the parser config uses.
		NLRISectionCarriesAttributes: true,
		RunEngine:                    runSRPolicyPlugin,
		CLIHandler:                   func([]string) int { return 0 },
		ConfigureEngineLogger: func(loggerName string) {
			SetLogger(slogutil.Logger(loggerName))
		},
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("srpolicy: registration failed", "error", err)
		os.Exit(1)
	}
}
