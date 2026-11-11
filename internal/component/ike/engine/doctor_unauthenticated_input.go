// Design: docs/architecture/ike/ipsec-10-cli-diag.md -- ipsec-unauthenticated-input
// RFC: rfc/short/rfc7296.md -- RFC7296-2.4-3
// Related: protected_input.go -- dropUnauthenticated, which counts each drop on the SA
// Related: doctor_virtual_ip.go -- the sibling check over the running engine this one is shaped after
// Related: doctor_peer_malformed.go -- the sibling for authenticated messages that do not parse

package engine

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// handshakeDrops is one peer whose handshake in progress dropped messages that failed
// their integrity check: the total over its handshake SAs and their states.
type handshakeDrops struct {
	peer   string
	drops  uint64
	states []string
}

// inHandshake reports whether an SA in state s is negotiating, the only states the
// ipsec-unauthenticated-input check reports. The set of states is closed: only Ze
// assigns one.
func inHandshake(s SAState) bool {
	switch s {
	case StateSAInitSent, StateSAInitReceived, StateAuthSent, StateAuthReceived, StateEAPInProgress:
		return true
	case StateIdle, StateEstablished, StateDead:
		return false
	default:
		panic("BUG: unknown IKE SA state")
	}
}

// unauthenticatedInputByPeer groups the handshake SAs of sas that dropped at least one
// message by peer, sorted by peer name. A peer that holds an established IKE SA is left
// out: its tunnel is up, so a forgery aimed at a parallel handshake changed nothing the
// operator has.
func unauthenticatedInputByPeer(sas []*SA) []handshakeDrops {
	established := map[string]bool{}
	for _, sa := range sas {
		if sa.State == StateEstablished {
			established[sa.PeerName] = true
		}
	}
	byPeer := map[string]*handshakeDrops{}
	for _, sa := range sas {
		drops := sa.unauthenticatedDropCount()
		if drops == 0 {
			continue
		}
		if !inHandshake(sa.State) {
			continue
		}
		if established[sa.PeerName] {
			continue
		}
		entry := byPeer[sa.PeerName]
		if entry == nil {
			entry = &handshakeDrops{peer: sa.PeerName}
			byPeer[sa.PeerName] = entry
		}
		entry.drops += drops
		if state := sa.State.String(); !slices.Contains(entry.states, state) {
			entry.states = append(entry.states, state)
		}
	}
	out := make([]handshakeDrops, 0, len(byPeer))
	for _, entry := range byPeer {
		slices.Sort(entry.states)
		out = append(out, *entry)
	}
	slices.SortFunc(out, func(a, b handshakeDrops) int { return strings.Compare(a.peer, b.peer) })
	return out
}

// checkIPsecUnauthenticatedInput warns once for each configured peer whose handshake in
// progress dropped messages that failed their integrity check. openProtected drops such
// a message and keeps the handshake (RFC 7296 Section 2.4), and logs it at debug only,
// so a tunnel held up by forgeries or by a path that damages datagrams shows no cause.
//
// The count lives on the SA, so the warning clears when the handshake establishes or a
// new cycle builds a new SA. With no engine running the check reports nothing.
func checkIPsecUnauthenticatedInput(_ registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	table := ActiveTable()
	if table == nil {
		return nil
	}
	found := unauthenticatedInputByPeer(table.All())
	out := make([]rpc.DoctorCheckDiagnostic, 0, len(found))
	for _, f := range found {
		var tb textbuf.Buffer
		tb.Str("ipsec peer ").Str(f.peer).Str(": its handshake in progress (state ")
		for i, state := range f.states {
			if i > 0 {
				tb.Str(", ")
			}
			tb.Str(state)
		}
		tb.Str(") dropped ").Int(int64(f.drops)).
			Str(" IKE messages that failed their integrity check. They carry no proof that " +
				"they came from the peer, so Ze kept waiting for the peer's answer and did not " +
				"end the handshake. The tunnel comes up late, or not at all if they keep arriving " +
				"until the retransmissions run out. Someone sends datagrams under the SPIs of this " +
				"handshake, or the path to the peer damages them: check that path on UDP 500 and 4500. " +
				"Each drop is counted in the handshake-unauthenticated series.")
		out = append(out, rpc.DoctorCheckDiagnostic{
			Code:     diagnostic.CodeDoctorIPsecUnauthenticatedInput,
			Severity: severityWarning,
			Message:  tb.String(),
			Fix:      "show metrics values | match handshake-unauthenticated",
		})
	}
	return out
}
