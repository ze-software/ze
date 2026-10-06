// Design: docs/architecture/testing/interop.md -- BGP peer-session observation.
package bgp

import (
	"context"
	"testing"
)

// VALIDATES: FRR continuity follows session establishment, not the derived epoch.
// PREVENTS: unchanged sessions failing policy reload when clock rounding changes.
func TestFRRSessionGenerationIgnoresDerivedEpoch(t *testing.T) {
	lab := &recordingLab{output: `{"172.30.0.2":{"bgpState":"Established","connectionsEstablished":1,"bgpTimerUpEstablishedEpoch":1790165799}}`}
	before, err := queryFRRSessionGeneration(t.Context(), lab, zeLabAddress)
	if err != nil {
		t.Fatal(err)
	}
	lab.output = `{"172.30.0.2":{"bgpState":"Established","connectionsEstablished":1,"bgpTimerUpEstablishedEpoch":1790165800}}`
	after, err := queryFRRSessionGeneration(t.Context(), lab, zeLabAddress)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("unchanged FRR session changed generation: %d -> %d", before, after)
	}
}

// VALIDATES: a reconnect requires Established and a greater lifetime count.
// PREVENTS: clock changes, disconnected peers, or reset counters passing a clear.
func TestFRRNewSessionRequiresEstablishedCountAdvance(t *testing.T) {
	for _, test := range []struct {
		name   string
		output string
		wantOK bool
	}{
		{"clock changed", `{"172.30.0.2":{"bgpState":"Established","connectionsEstablished":2,"bgpTimerUpEstablishedEpoch":1790165800}}`, false},
		{"reconnected in same second", `{"172.30.0.2":{"bgpState":"Established","connectionsEstablished":3,"bgpTimerUpEstablishedEpoch":1790165799}}`, true},
		{"disconnected after reconnect", `{"172.30.0.2":{"bgpState":"Active","connectionsEstablished":3}}`, false},
		{"counter reset", `{"172.30.0.2":{"bgpState":"Established","connectionsEstablished":1}}`, false},
		{"missing count", `{"172.30.0.2":{"bgpState":"Established","bgpTimerUpEstablishedEpoch":1790165800}}`, false},
		{"missing neighbor", `{}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			// Admit one real observation, then stop before another poll.
			lab := &recordingLab{output: test.output, afterQuery: cancel}
			err := waitFRRNewSession(ctx, lab, zeLabAddress, 2)
			if lab.reads != 1 {
				t.Fatalf("session observations = %d, want exactly one", lab.reads)
			}
			if (err == nil) != test.wantOK {
				t.Fatalf("re-establishment error = %v, want success %t", err, test.wantOK)
			}
		})
	}
}
