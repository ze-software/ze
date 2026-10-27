// Design: docs/guide/add-path.md -- the receive ceiling under Ze's own PATHS-LIMIT
// RFC: rfc/drafts/draft-abraitis-idr-addpath-paths-limit.txt -- Section 3
// Related: session_paths_limit.go -- the admission state, section filter and journal shared with send
package reactor

import (
	"encoding/hex"
)

// pathsLimitRefusedHeadroom is how many refused identifiers one route tracks
// beyond its admitted ones. A peer that ignores the limit cannot decide how
// much memory the audit costs: the tracked set per route never exceeds the
// limit plus this headroom (ExaBGP's IncomingRIB.track_path uses the same 1).
const pathsLimitRefusedHeadroom = 1

// reportPathsLimitRefusals counts every path the receive ceiling refused in one
// message and warns once for each route whose refused set was empty before it.
// The receive caller MUST call it once, after the message's admission is final
// and before releasing the journal. It MUST NOT be called after rollback: a
// rolled-back refusal is not final and its route warns again on replay.
//
// draft-abraitis-idr-addpath-paths-limit Section 3: "An implementation SHOULD
// provide a configuration knob to specify the maximum number of paths to
// accept from a sender."
func (s *Session) reportPathsLimitRefusals(changes *pathsLimitChanges) {
	if changes.refusals == 0 {
		return
	}
	for i := range changes.entries[:changes.n] {
		change := &changes.entries[i]
		if change.kind != pathsLimitRefusalTracked && change.kind != pathsLimitRefusalCounted {
			continue
		}
		if s.prefixMetrics != nil {
			s.prefixMetrics.pathsLimitRefused.With(s.peerLabel(), change.family.family.String()).Inc()
		}
		if !change.warn {
			continue
		}
		sessionLogger().Warn("PATHS-LIMIT: peer sent more paths for one route than Ze advertised it would accept; the excess path is dropped",
			"peer", s.settings.Address,
			"family", change.family.family.String(),
			"limit", change.family.limit,
			"route-key", hex.EncodeToString([]byte(change.prefix.key)),
			"path-id", change.id,
			"fix", "raise 'add-path limit receive' for this family, or have the peer honor the PATHS-LIMIT capability",
			"repeats", "suppressed for this route until the peer withdraws the refused path")
	}
}
