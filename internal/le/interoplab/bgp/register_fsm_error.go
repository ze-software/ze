// Design: docs/architecture/testing/interop.md -- RFC 6608 native-peer relay scenarios.
package bgp

import (
	"context"
	"io"

	"github.com/ze-software/ze/internal/le/interoplab"
)

type fsmRelayCase struct {
	scenario, personality string
	subcode, message      byte
}

// Subcode zero names the valid-handshake control, never a generated error.
var fsmRelayCases = []fsmRelayCase{
	{"bgp-fsm-error-opensent-frr", "fsm-error-opensent", 1, bgpKeepalive},
	{"bgp-fsm-error-openconfirm-frr", "fsm-error-openconfirm", 2, bgpOpen},
	{"bgp-fsm-error-established-frr", "fsm-error-established", 3, bgpOpen},
	{"bgp-fsm-valid-handshake-frr", "fsm-valid-handshake", 0, 0},
}

func init() {
	for _, test := range fsmRelayCases {
		specialCheckers[test.scenario] = func(ctx context.Context, check *interoplab.CheckContext) error {
			return checkFSMErrorFRR(ctx, check, test)
		}
		speakerRelayRunners[test.personality] = func(options speakerOptions, output io.Writer) error {
			return runFSMErrorRelay(options, output, test)
		}
	}
}
