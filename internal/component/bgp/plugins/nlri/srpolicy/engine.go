// Design: docs/architecture/wire/nlri.md -- SR-Policy NLRI plugin engine
// RFC: rfc/short/rfc9830.md -- SAFI 73 (SR-Policy)
// Related: register.go -- plugin registration that names runSRPolicyPlugin

package srpolicy

import (
	"log/slog"
	"net"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// familyModeDecode declares a family this plugin decodes. Encoding stays on
// the in-process encoders register.go names, as for every NLRI plugin but
// FlowSpec. The plugin server reads it as sdk.FamilyDecl.Mode.
const familyModeDecode = "decode"

var logger = slogutil.DiscardLogger()

// SetLogger sets the package-level logger.
func SetLogger(l *slog.Logger) {
	if l != nil {
		logger = l
	}
}

// familyDecl declares fam to the plugin server, taking its name and its AFI
// and SAFI numbers from the family.MustRegister call in register.go.
func familyDecl(fam family.Family) sdk.FamilyDecl {
	return sdk.FamilyDecl{
		Name: fam.String(),
		Mode: familyModeDecode,
		AFI:  uint16(fam.AFI),
		SAFI: uint8(fam.SAFI),
	}
}

// runSRPolicyPlugin runs the SR-Policy plugin over the SDK RPC protocol.
//
// The daemon starts it whenever a peer configures an SR-Policy family that no
// explicit plugin claims (getUnclaimedFamilyPlugins), so it MUST complete the
// startup handshake. An engine that returns at once closes the connection, and
// the daemon then fails the plugin at Stage 1.
func runSRPolicyPlugin(conn net.Conn) int {
	logger.Debug("srpolicy plugin starting (RPC)")

	p := sdk.NewWithConn("bgp-nlri-srpolicy", conn)
	defer func() { _ = p.Close() }()

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, &sdk.Registration{
		Families: []sdk.FamilyDecl{
			familyDecl(IPv4SRPolicy),
			familyDecl(IPv6SRPolicy),
		},
	})
	if err != nil {
		logger.Error("srpolicy plugin failed", "error", err)
		return 1
	}

	return 0
}
