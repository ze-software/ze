// Design: docs/architecture/testing/interop.md -- RFC 8654 directional matrix.
package bgp

const (
	peerFRRSink      = "frr-sink"
	extendedSinkHost = 15
)

// extendedMessageCase describes the OPENs as Ze sees them, never FRR's rewritten
// view. Source and sink are separate FRR daemons with distinct native router IDs.
type extendedMessageCase struct {
	sourceLocal  bool
	sourceRemote bool
	sinkLocal    bool
	sinkRemote   bool
}

var extendedMessageCases = map[string]extendedMessageCase{
	"bgp-extended-message-asymmetric-frr":         {sourceLocal: true, sinkRemote: true},
	"bgp-extended-message-bilateral-frr":          {sourceLocal: true, sourceRemote: true, sinkLocal: true, sinkRemote: true},
	"bgp-extended-message-send-denied-frr":        {sourceLocal: true, sinkLocal: true},
	"bgp-extended-message-remote-only-reject-frr": {sourceRemote: true, sinkRemote: true},
	"bgp-extended-message-neither-reject-frr":     {},
}

func init() {
	specialCheckers["bgp-extended-message-asymmetric-frr"] = checkExtendedAsymmetricFRR
	specialCheckers["bgp-extended-message-bilateral-frr"] = checkExtendedBilateralFRR
	specialCheckers["bgp-extended-message-send-denied-frr"] = checkExtendedSendDeniedFRR
	specialCheckers["bgp-extended-message-remote-only-reject-frr"] = checkExtendedRemoteOnlyRejectFRR
	specialCheckers["bgp-extended-message-neither-reject-frr"] = checkExtendedNeitherRejectFRR
}
