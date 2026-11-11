// Design: docs/architecture/ike/ipsec-10-cli-diag.md -- ipsec-peer-malformed-message
// RFC: rfc/short/rfc7296.md -- Section 3.1, no answer to a response
// Related: inbound.go -- handleOwnedInbound, the three sites that count on the SA
// Related: fsm.go -- handleAuthResponse, the site that counts in the RFC 4739 second exchange
// Related: doctor_unauthenticated_input.go -- the sibling for messages that fail their integrity check

package engine

import (
	"slices"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// peerMalformed is one peer whose live IKE SAs received authenticated messages that do
// not parse, and how many.
type peerMalformed struct {
	peer     string
	messages uint64
}

// peerMalformedByPeer totals the authenticated malformed messages of sas by peer,
// sorted by peer name, leaving out every peer with none.
func peerMalformedByPeer(sas []*SA) []peerMalformed {
	byPeer := map[string]uint64{}
	for _, sa := range sas {
		if n := sa.peerMalformedMessageCount(); n > 0 {
			byPeer[sa.PeerName] += n
		}
	}
	names := make([]string, 0, len(byPeer))
	for name := range byPeer {
		names = append(names, name)
	}
	slices.Sort(names)
	out := make([]peerMalformed, 0, len(names))
	for _, name := range names {
		out = append(out, peerMalformed{peer: name, messages: byPeer[name]})
	}
	return out
}

// checkIPsecPeerMalformedMessage warns once for each peer whose live IKE SA received
// messages whose integrity check verified and whose payloads do not parse, which
// handleOwnedInbound drops without an answer on an established SA, and
// handleAuthResponse drops on the handshake SA in the RFC 4739 second exchange. Only the
// peer holds the keys, so the
// defect is the peer's, and the WARN line of each drop is otherwise its only record.
//
// The count lives on the SA, so it clears when the SA ends or is rekeyed. With no
// engine running the check reports nothing.
func checkIPsecPeerMalformedMessage(_ registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	table := ActiveTable()
	if table == nil {
		return nil
	}
	found := peerMalformedByPeer(table.All())
	out := make([]rpc.DoctorCheckDiagnostic, 0, len(found))
	for _, f := range found {
		var tb textbuf.Buffer
		tb.Str("ipsec peer ").Str(f.peer).Str(" sent ").Int(int64(f.messages)).
			Str(" IKE messages on its live IKE SA whose integrity check verified and whose " +
				"payloads do not parse. Only the peer holds the keys of the SA, and the check " +
				"covers every octet, so no path damaged them: the defect is in the peer's IKE " +
				"implementation. Ze dropped them and kept the SA, but the exchange each one " +
				"carried was lost, so a rekey, a liveness check, an address update or a second " +
				"authentication exchange can fail. " +
				"Collect the peer's IKE log for the same time and report the defect to its vendor.")
		out = append(out, rpc.DoctorCheckDiagnostic{
			Code:     diagnostic.CodeDoctorIPsecPeerMalformedMessage,
			Severity: severityWarning,
			Message:  tb.String(),
			Fix:      "show vpn ipsec sa",
		})
	}
	return out
}
