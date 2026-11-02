// Design: docs/architecture/bgp/structural-forwarding.md -- reactor-native RS forwarding
// Related: forward_rs.go -- reactorForwardRSSection, which fills and counts the edits
// Related: reactor_api_forward.go -- the general rail, which counts the same edits

package reactor

// rsCommunityEdits is what the route-server control communities of one received
// UPDATE changed in what one client is written. Both forward rails fill it while
// they record the edits, and recordRSCommunityEdits counts it each time an
// UPDATE carrying them is queued for the client, never where the edit is only
// computed: a destination the egress filters, a failed rebuild or a withdrawal
// then suppress counts nothing.
type rsCommunityEdits struct {
	prepended           bool // the route asked for an AS_PATH prepend toward this client
	largeStrippedValues int  // LARGE_COMMUNITY values in the route server's namespace removed
}

// recordRSCommunityEdits counts one UPDATE queued for a client with its
// control-community edits applied.
//
// It counts every UPDATE written, so a received UPDATE split into several
// sections counts once per section: each section carries the prepend and the
// strip. Counting per received UPDATE instead would need state shared across
// the split loops of both rails (forwardUpdateSelected, reactorForwardRS), and
// each UPDATE written is where the edit takes effect. The caller MUST call it
// only for a delivery that is not a withdrawal: a withdrawal carries no path
// attribute, so no edit reaches the client.
//
// Telemetry can be off, so a reactor with no registry (rmetrics nil) counts
// nothing: counting never changes what the client is written.
func (r *Reactor) recordRSCommunityEdits(edits rsCommunityEdits) {
	if r == nil {
		return
	}
	if r.rmetrics == nil {
		return
	}
	if edits.prepended {
		r.rmetrics.rsPrependApplied.Inc()
	}
	if edits.largeStrippedValues > 0 {
		r.rmetrics.rsLargeControlStripped.Add(float64(edits.largeStrippedValues))
	}
}
