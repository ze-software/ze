// Design: docs/architecture/testing/interop.md -- RTR cache preference observations.
package bgp

import (
	"strings"
	"testing"
)

// TestRPKIPreferencePhaseError checks the rtr-stayrtr-preference predicate
// against hand-built observations: each phase's correct reading passes, and
// each way a broken client could look (the wrong cache in use, both caches
// claiming data, a union of both VRP sets, the prefix leaked or lost at BIRD)
// fails. Method: one valid observation per phase, then one mutation per case.
func TestRPKIPreferencePhaseError(t *testing.T) {
	const preferred, standby = "172.30.0.12", "172.30.0.14"
	const control = "10.59.0.0/16 unicast [ze_peer 12:00:00] * (100)\n\tbgp_path: 65000 65001\n"
	const prefix = "9.59.0.0/24 unicast [ze_peer 12:00:00] * (100)\n\tbgp_path: 65000 65001\n"
	onPreferred := rpkiPreferencePhase{"preferred", preferred, standby, rpkiPreferredVRPs, true}
	onStandby := rpkiPreferencePhase{"standby", standby, preferred, rpkiStandbyVRPs, false}
	caches := func(live string) []rpkiPreferenceCache {
		return []rpkiPreferenceCache{{standby, live == standby}, {preferred, live == preferred}}
	}
	valid := map[string]struct {
		phase    rpkiPreferencePhase
		observed rpkiPreferenceObservation
	}{
		"preferred": {onPreferred, rpkiPreferenceObservation{caches(preferred), rpkiPreferredVRPs, control + prefix}},
		"standby":   {onStandby, rpkiPreferenceObservation{caches(standby), rpkiStandbyVRPs, control}},
	}
	for name, test := range valid {
		if err := rpkiPreferencePhaseError(test.observed, test.phase); err != nil {
			t.Fatalf("%s: correct observation refused: %v", name, err)
		}
	}
	union := append(append([]rpkiPreferenceVRP(nil), rpkiPreferredVRPs...), rpkiStandbyVRPs[0])
	for _, test := range []struct {
		name   string
		phase  rpkiPreferencePhase
		mutate func(*rpkiPreferenceObservation)
	}{
		{"standby in use on preferred phase", onPreferred, func(o *rpkiPreferenceObservation) { o.Caches = caches(standby) }},
		{"both caches synced", onPreferred, func(o *rpkiPreferenceObservation) { o.Caches[0].Synced = true }},
		{"no cache synced", onStandby, func(o *rpkiPreferenceObservation) { o.Caches[0].Synced = false }},
		{"preferred cache unlisted", onPreferred, func(o *rpkiPreferenceObservation) { o.Caches = o.Caches[1:2] }},
		{"standby cache unlisted", onPreferred, func(o *rpkiPreferenceObservation) { o.Caches = o.Caches[0:1] }},
		{"union of both sets", onPreferred, func(o *rpkiPreferenceObservation) { o.VRPs = union }},
		{"standby set on preferred phase", onPreferred, func(o *rpkiPreferenceObservation) { o.VRPs = rpkiStandbyVRPs }},
		{"preferred set on standby phase", onStandby, func(o *rpkiPreferenceObservation) { o.VRPs = rpkiPreferredVRPs }},
		{"duplicate VRP", onStandby, func(o *rpkiPreferenceObservation) {
			o.VRPs = []rpkiPreferenceVRP{rpkiStandbyVRPs[1], rpkiStandbyVRPs[1]}
		}},
		{"prefix not exported", onPreferred, func(o *rpkiPreferenceObservation) { o.Export = control }},
		{"prefix leaked after failover", onStandby, func(o *rpkiPreferenceObservation) { o.Export = control + prefix }},
		{"control lost", onStandby, func(o *rpkiPreferenceObservation) { o.Export = "" }},
		{"prefix from another origin", onPreferred, func(o *rpkiPreferenceObservation) {
			o.Export = control + strings.Replace(prefix, "65000 65001", "65000 65003", 1)
		}},
		{"covering route only", onPreferred, func(o *rpkiPreferenceObservation) {
			o.Export = control + strings.Replace(prefix, "9.59.0.0/24", "9.59.0.0/16", 1)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			observed := valid[test.phase.name].observed
			observed.Caches = append([]rpkiPreferenceCache(nil), observed.Caches...)
			test.mutate(&observed)
			if err := rpkiPreferencePhaseError(observed, test.phase); err == nil {
				t.Fatalf("observation accepted: %+v", observed)
			}
		})
	}
}
