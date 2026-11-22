// Design: docs/guide/graceful-restart.md -- consecutive restarts during the LLGR period
// RFC: rfc/short/rfc4724.md -- Section 4.2, Procedures for the Receiving Speaker
// RFC: rfc/short/rfc9494.md -- Section 4.2, Session Resets
// Overview: gr.go -- session-down dispatch and the LLGR callbacks that call into this file
// Related: gr_state.go -- onSessionDownPlan keeps running LLST timers across a restart
//
// A peer whose LLGR period is running can come back, refresh some routes, and
// go down again before End-of-RIB. Two connections then hold routes the helper
// retains for the same family and the same LLST timer: the earlier one still
// owns the stale routes that were not refreshed, and the later one owns the
// routes it sent. Every route lives under the connection that delivered it, in
// the RIB and in each store that follows the validation-cleanup event, so the
// plugin addresses each connection in turn. Which earlier connections still own
// routes is the RIB's fact: the plugin reads it from the retain-routes answer
// and keeps no list of its own beside it.

package gr

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// answerKeyEarlierConnections is the retain-routes answer key under which the
// RIB lists the earlier connections that still own retained routes
// (rib_source_commands.go, retainedSourceAnswer). The two plugins share no
// package, so the name is written on both sides of the command.
const answerKeyEarlierConnections = "earlier-connections"

// grEarlierRetained is the record of one retained cycle's earlier connections.
// It is valid only for the cycle addressed by current; a later cycle of the
// same peer replaces it.
type grEarlierRetained struct {
	current uint64     // the connection the retained cycle is addressed by
	sources []grSource // the earlier connections, oldest first; never empty
}

// retainedConnections returns every connection whose routes the retained cycle
// addressed by source holds: the earlier ones first, then source itself. A
// source that addresses no recorded cycle answers itself alone. Safe for
// concurrent use.
func (gp *grPlugin) retainedConnections(source grSource) []grSource {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	record, recorded := gp.earlierRetained[source.owner]
	if !recorded {
		return []grSource{source}
	}
	if record.current != source.connectionID {
		return []grSource{source}
	}
	return append(slices.Clone(record.sources), source)
}

// dispatchRetained sends one removal command to every connection the retained
// cycle addressed by source holds. mark-stale is never sent this way: the RIB
// accepts it for the cycle's own connection only, and an earlier connection's
// surviving routes were marked by the restart that retained them.
//
// RFC 9494 Section 4.2: "If the timer for the Long-Lived Stale Time for a given
// AFI/SAFI expires before the session is re-established, the helper MUST delete
// all stale routes of that AFI/SAFI from the neighbor that it is retaining."
// The routes are "from the neighbor", whichever connection delivered them.
func (gp *grPlugin) dispatchRetained(source grSource, command string, tail ...string) {
	for _, retained := range gp.retainedConnections(source) {
		gp.dispatchSource(retained, command, tail...)
	}
}

// recordEarlierRetained replaces the peer's record with what the RIB answered
// to the retention of source. previous is every connection the continued cycle
// retained before this DOWN; it is used only when the answer does not say.
//
// A retention the RIB did not apply retains nothing, so no earlier connection
// is recorded. An applied retention whose answer names no list (a test hook
// that returns nothing, or a RIB defect) leaves the membership unknown: every
// previous connection then stays addressed, because a purge that finds nothing
// is refused in the open while an unaddressed connection would keep its stale
// routes past the timer in silence. That path alone can grow by one connection
// per restart; the answered path is bounded by the RIB's own list.
func (gp *grPlugin) recordEarlierRetained(source grSource, previous []grSource, answer json.RawMessage, applied bool) {
	var earlier []grSource
	if applied {
		answered, said := earlierConnections(source, answer)
		earlier = answered
		if !said {
			earlier = previous
			if len(previous) > 0 {
				reportEarlierConnectionsUnknown(source, len(previous))
			}
		}
	}
	gp.mu.Lock()
	defer gp.mu.Unlock()
	if len(earlier) == 0 {
		delete(gp.earlierRetained, source.owner)
		return
	}
	if gp.earlierRetained == nil {
		gp.earlierRetained = make(map[uint64]grEarlierRetained)
	}
	gp.earlierRetained[source.owner] = grEarlierRetained{current: source.connectionID, sources: earlier}
}

// reportEarlierConnectionsUnknown tells the operator that the route store's
// retain-routes answer lacked the earlier-connection list, a defect between
// two Ze plugins and not a configuration error. It enters the same bounded
// recent-event report as a refused source command (validation_cleanup.go).
func reportEarlierConnectionsUnknown(source grSource, connections int) {
	failure := opererr.New("The route store did not name the earlier connections that still own retained routes of this peer",
		"GR keeps purging every earlier connection of the peer, so no stale route outlives its timer, but a purge of a connection that owns no route is refused and reported",
		"Report this as a Ze defect, then request reload")
	subject := "request bgp rib source retain-routes owner " + textbuf.StringUint(source.owner) + " connection " + textbuf.StringUint(source.connectionID)
	logger().LogAttrs(context.Background(), slog.LevelWarn, failure.Problem(),
		failure.LogAttrs(slog.String("code", diagnostic.CodeDoctorRecentEvents), slog.String("peer", source.peer()),
			slog.Int("connections", connections), slog.String("command", subject))...)
	report.RaiseError("bgp-gr", diagnostic.CodeDoctorRecentEvents, subject, failure.Error())
}

// earlierConnections reads the earlier-connection list from a retain-routes
// answer. False means the answer does not say: it is empty, is not the answer
// object, lacks the key, or names a connection that is not a nonzero decimal
// id. True with no source means the RIB said there is none.
func earlierConnections(source grSource, answer json.RawMessage) ([]grSource, bool) {
	if len(answer) == 0 {
		return nil, false
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(answer, &decoded); err != nil {
		return nil, false
	}
	raw, present := decoded[answerKeyEarlierConnections]
	if !present {
		return nil, false
	}
	var connections []string
	if err := json.Unmarshal(raw, &connections); err != nil {
		return nil, false
	}
	earlier := make([]grSource, 0, len(connections))
	for _, text := range connections {
		connectionID, err := strconv.ParseUint(text, 10, 64)
		if err != nil {
			return nil, false
		}
		retained, err := source.source(connectionID)
		if err != nil {
			return nil, false
		}
		earlier = append(earlier, retained)
	}
	return earlier, true
}

// forgetEarlierRetained drops the record once the cycle addressed by source
// has released its routes. A record of another cycle is left alone. Safe for
// concurrent use.
func (gp *grPlugin) forgetEarlierRetained(source grSource) {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	record, recorded := gp.earlierRetained[source.owner]
	if !recorded {
		return
	}
	if record.current != source.connectionID {
		return
	}
	delete(gp.earlierRetained, source.owner)
}

// dropEarlierRetained drops the peer's record whatever cycle it names: the
// peer retains nothing any more. Safe for concurrent use.
func (gp *grPlugin) dropEarlierRetained(owner uint64) {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	delete(gp.earlierRetained, owner)
}

// restalePreservedLLGR marks LLGR-stale the routes the connection that just
// ended delivered for the families whose LLGR period is still running. Those
// families start no new period, so nothing else marks these routes; the
// family's LLST timer is left as it is.
//
// RFC 4724 Section 4.2: "When the Receiving Speaker detects termination of the
// TCP session for a BGP session with a peer that has advertised the Graceful
// Restart Capability, it MUST retain the routes received from the peer for all
// the address families that were previously received in the Graceful Restart
// Capability and MUST mark them as stale routing information."
// RFC 9494 Section 4.2: "The helper router MUST attach the LLGR_STALE community
// to the stale routes being retained."
//
// The RFC does not say which period the routes of the second session join.
// BIRD answers it the same way: bgp_handle_graceful_restart, for a channel in
// BGP_GRS_LLGR, runs rt_refresh_begin then rt_modify_stale and leaves the stale
// timer running (proto/bgp/bgp.c).
//
// The level-2 mark comes first because the two community commands skip a fresh
// route. Unlike the LLGR entry, no command here is guarded on the family's
// timer: if it expired while this DOWN was being dispatched, its purge may have
// run before these routes were retained and marked, so the purge is sent again.
func (gp *grPlugin) restalePreservedLLGR(source grSource, families []family.Family) {
	restaled := make([]family.Family, 0, len(families))
	for _, fam := range families {
		famStr := fam.String()
		gp.markLLGRStale(source, famStr)
		// The two community commands also reach the earlier connections. Their
		// stale routes normally carry the marks already, and the commands then
		// change nothing; but this DOWN makes source the cycle's connection,
		// which stops an LLGR entry of an earlier connection that is still
		// dispatching (llgrFamilyActive) and can leave those routes stale
		// without LLGR_STALE while the family's timer runs. attach-community
		// raises a stale route to level 2 itself, so no mark is owed to them.
		for _, retained := range gp.retainedConnections(source) {
			gp.removeNoLLGR(retained, famStr)
			gp.attachLLGRStale(retained, famStr)
		}
		if gp.state.llgrFamilyActive(source, fam) {
			restaled = append(restaled, fam)
			continue
		}
		// RFC 9494 Section 4.2.
		gp.dispatchRetained(source, "request bgp rib source purge-stale", "family", famStr)
	}
	gp.readvertiseLLGR(source, restaled)
}

// removeNoLLGR deletes the stale routes of one family that carry NO_LLGR.
// RFC 9494 Section 4.2: "If any of the routes from the peer have been marked
// with the NO_LLGR community, either as sent by the peer or as the result of a
// configured policy, they MUST NOT be retained and MUST be removed as per the
// normal operation of [RFC4271]."
// NO_LLGR is 0xFFFF0007.
func (gp *grPlugin) removeNoLLGR(source grSource, famStr string) {
	gp.dispatchSource(source, "request bgp rib source delete-with-community", "family", famStr, "community", "ffff0007")
}

// attachLLGRStale attaches LLGR_STALE (0xFFFF0006) to the stale routes of one
// family. RFC 9494 Section 4.2.
func (gp *grPlugin) attachLLGRStale(source grSource, famStr string) {
	gp.dispatchSource(source, "request bgp rib source attach-community", "family", famStr, "community", "ffff0006")
}

// markLLGRStale raises one family's routes to stale level 2, the depreference
// threshold. The restart time is zero: the family's LLST timer bounds the
// retention, so the RIB arms no safety timer of its own.
func (gp *grPlugin) markLLGRStale(source grSource, famStr string) {
	gp.dispatchSource(source, "request bgp rib source mark-stale", "restart-time", "0", "stale-level", "2", "family", famStr)
}

// readvertiseLLGR resends, to every other peer, the routes of the families
// whose stale routes have just gained LLGR_STALE.
// RFC 9494 Section 4.2: "Note that this requirement implies that the routes
// would need to be readvertised in order to disseminate the modified
// community."
func (gp *grPlugin) readvertiseLLGR(source grSource, families []family.Family) {
	for _, fam := range families {
		gp.dispatchCommand("clear bgp rib out-excluding-owner", "owner", textbuf.StringUint(source.owner), "family", fam.String())
	}
}
