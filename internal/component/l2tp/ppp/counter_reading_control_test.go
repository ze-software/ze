// Design: docs/research/l2tpv2-ze-integration.md -- explicit per-counter availability
// Related: counter_reading_test.go -- independent value semantics

package ppp

import "testing"

// The actual admitted worker returns the original handle's optional readings intact.
func TestSessionControlPreservesEightCounterReadings(t *testing.T) {
	_, owner, control := admitNativeBudgetSession(t, nil, 71)
	want := SessionCounters{
		RxBytes: 11, TxBytes: 13, RxPackets: 17, TxPackets: 19,
		RxErrors: MeasuredCounter(0), TxErrors: MeasuredCounter(29),
		RxDropped: MeasuredCounter(31), TxDropped: MeasuredCounter(37),
	}
	owner.counterRead = func() (SessionCounters, error) { return want, nil }
	got, err := control.Counters()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("captured worker changed original readings: %+v", got)
	}
}
