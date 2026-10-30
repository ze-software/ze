// Design: docs/architecture/rib/unified-locrib.md -- the import policy gate on the Loc-RIB
// Related: policy.go -- admit, which hides a path an import chain rejects
// Related: entry.go -- upsert and remove, which account each stored path
// Related: distance.go -- Reselect, which re-judges every stored path

package locrib

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// policySourcesMax bounds the ProtocolIDs the policy metrics track. Protocol
// IDs are assigned in registration order from 1, and a binary registers a few
// dozen, so 256 leaves room. A source at or above the bound is counted by
// neither metric, and SetPolicyMetricsRegistry binds no series for it, so the
// exposition omits it rather than reporting a wrong number.
const policySourcesMax = 256

// hiddenPaths counts the hidden paths each source holds in the process-wide
// Loc-RIB, whether or not a metrics registry is bound, so a registry bound
// after the first insert still publishes the true count. Indexed by
// ProtocolID. Safe for concurrent use.
var hiddenPaths [policySourcesMax]atomic.Int64

// policyMetrics holds the series handles, resolved once when the registry is
// bound, so an insert never formats a label or allocates a series.
type policyMetrics struct {
	// hidden is ze_rib_policy_hidden_paths{source}, indexed by ProtocolID.
	hidden [policySourcesMax]metrics.Gauge
	// rejected is ze_rib_policy_rejected_total{source,chain}, indexed by
	// ProtocolID then ChainRole.
	rejected [policySourcesMax][ribpolicy.ChainRoleCount]metrics.Counter
}

var policyMetricsPtr atomic.Pointer[policyMetrics]

// SetPolicyMetricsRegistry registers the import policy metrics in reg and
// binds one series per registered source (and per chain for the counter). A
// nil reg unbinds them. The policy component calls it from its
// ConfigureMetrics hook; every protocol registers in init(), before that hook
// runs, so every source is bound. Safe for concurrent use.
func SetPolicyMetricsRegistry(reg metrics.Registry) {
	if reg == nil {
		policyMetricsPtr.Store(nil)
		return
	}
	hiddenVec := reg.GaugeVec("ze_rib_policy_hidden_paths",
		"Paths an import chain hid in the Loc-RIB, by the source that offered them.",
		[]string{"source"})
	rejectedVec := reg.CounterVec("ze_rib_policy_rejected_total",
		"Paths an import chain rejected when the Loc-RIB stored or re-judged them, by source and by the chain that rejected (source or central).",
		[]string{"source", "chain"})
	m := &policyMetrics{}
	for _, name := range redistevents.ProtocolNames() {
		id, ok := redistevents.ProtocolIDOf(name)
		if !ok {
			continue
		}
		if int(id) >= policySourcesMax {
			continue
		}
		m.hidden[id] = hiddenVec.With(name)
		m.hidden[id].Set(float64(hiddenPaths[id].Load()))
		for _, role := range [...]ribpolicy.ChainRole{ribpolicy.ChainRoleSource, ribpolicy.ChainRoleCentral} {
			m.rejected[id][role] = rejectedVec.With(name, role.String())
		}
	}
	policyMetricsPtr.Store(m)
}

// accountStored records that stored replaced old (nil for a new path) in a
// PathGroup: it moves the source's hidden count and, when stored is hidden,
// counts one rejection by the chain that hid it. A producer's offer that is
// rejected always counts. A Reselect re-judgement (rejudged) counts only when it
// changes the verdict, so a reload does not count again every path an
// unchanged chain already hid. Called under the shard lock, once for every
// path upsert and Reselect store. Allocates nothing.
func accountStored(old, stored *Path, rejudged bool) {
	if stored.Hidden {
		if !rejudged {
			countRejected(stored.Source, stored.HiddenBy)
		} else if old == nil || !old.Hidden || old.HiddenBy != stored.HiddenBy {
			countRejected(stored.Source, stored.HiddenBy)
		}
	}
	wasHidden := old != nil && old.Hidden
	if wasHidden == stored.Hidden {
		return
	}
	if stored.Hidden {
		moveHidden(stored.Source, 1)
		return
	}
	moveHidden(old.Source, -1)
}

// accountRemoved records that p left its PathGroup.
func accountRemoved(p *Path) {
	if p.Hidden {
		moveHidden(p.Source, -1)
	}
}

// moveHidden adds delta to source's hidden count and publishes it.
func moveHidden(source redistevents.ProtocolID, delta int64) {
	if int(source) >= policySourcesMax {
		return
	}
	n := hiddenPaths[source].Add(delta)
	m := policyMetricsPtr.Load()
	if m == nil {
		return
	}
	if g := m.hidden[source]; g != nil {
		g.Set(float64(n))
	}
}

// countRejected counts one rejection of a source's path by role's chain.
func countRejected(source redistevents.ProtocolID, role ribpolicy.ChainRole) {
	if int(source) >= policySourcesMax {
		return
	}
	if int(role) >= ribpolicy.ChainRoleCount {
		return
	}
	m := policyMetricsPtr.Load()
	if m == nil {
		return
	}
	if c := m.rejected[source][role]; c != nil {
		c.Inc()
	}
}
