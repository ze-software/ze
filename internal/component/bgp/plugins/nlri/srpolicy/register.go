// Design: docs/architecture/wire/nlri.md -- SR-Policy NLRI plugin registration
// RFC: rfc/short/rfc9830.md -- SAFI 73 (SR-Policy)
// Related: types.go -- SRPolicy NLRI type
// Related: split.go -- NLRI splitter

package srpolicy

import (
	"log/slog"
	"net"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// Family registrations for SR-Policy (SAFI 73).
var (
	IPv4SRPolicy = family.MustRegister(family.AFIIPv4, family.SAFISRPolicy, "ipv4", "sr-policy")
	IPv6SRPolicy = family.MustRegister(family.AFIIPv6, family.SAFISRPolicy, "ipv6", "sr-policy")
)

func init() {
	nlrisplit.Register(IPv4SRPolicy, SplitSRPolicy)
	nlrisplit.Register(IPv6SRPolicy, SplitSRPolicy)

	reg := registry.Registration{
		Name:                       "bgp-nlri-srpolicy",
		Description:                "SR-Policy family plugin (RFC 9830, SAFI 73)",
		SupportsNLRI:               true,
		Features:                   "nlri",
		Families:                   []string{"ipv4/sr-policy", "ipv6/sr-policy"},
		InProcessNLRIDecoder:       DecodeNLRIHex,
		InProcessNLRIEncoder:       EncodeNLRIHex,
		InProcessRouteEncoder:      EncodeRoute,
		InProcessConfigRouteParser: parseConfigRoute,
		RunEngine:                  func(net.Conn) int { return 0 },
		CLIHandler:                 func([]string) int { return 0 },
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("srpolicy: registration failed", "error", err)
		os.Exit(1)
	}
}
