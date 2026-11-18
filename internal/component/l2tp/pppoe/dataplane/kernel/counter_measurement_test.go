// Design: docs/research/l2tpv2-ze-integration.md -- native diagnostic counter measurements
// Related: factory_counter_created_integration_linux_test.go -- original reader measurement fault port

//go:build integration && linux

package kernel

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestKernelCounterMeasurements checks all eight supplied native totals at the
// numeric adapter boundary. It does not establish native session identity.
func TestKernelCounterMeasurements(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	native := &iface.InterfaceStats{
		RxBytes: 1<<40 + 11, TxBytes: 1<<41 + 13,
		RxPackets: 1<<42 + 17, TxPackets: 1<<43 + 19,
		RxErrors: 1<<44 + 23, TxErrors: 1<<45 + 29,
		RxDropped: 1<<46 + 31, TxDropped: 1<<47 + 37,
	}
	backend, opened := newFactoryCounterSession(t, native)
	counters, err := opened.Dataplane().Counters()
	if err != nil {
		t.Fatal(err)
	}
	if backend.statsName != opened.Dataplane().Identity().InterfaceName {
		t.Fatalf("counter creation device=%q, want original %q", backend.statsName, opened.Dataplane().Identity().InterfaceName)
	}
	for _, field := range []struct {
		name string
		got  uint64
		want uint64
	}{
		{"rx-bytes", counters.RxBytes, native.RxBytes},
		{"tx-bytes", counters.TxBytes, native.TxBytes},
		{"rx-packets", counters.RxPackets, native.RxPackets},
		{"tx-packets", counters.TxPackets, native.TxPackets},
	} {
		if field.got != field.want {
			t.Errorf("%s=%d, want raw total %d", field.name, field.got, field.want)
		}
	}
	for _, field := range []struct {
		name    string
		reading ppp.CounterReading
		want    uint64
	}{
		{"rx-errors", counters.RxErrors, native.RxErrors},
		{"tx-errors", counters.TxErrors, native.TxErrors},
		{"rx-dropped", counters.RxDropped, native.RxDropped},
		{"tx-dropped", counters.TxDropped, native.TxDropped},
	} {
		got, err := field.reading.Value()
		if err != nil {
			t.Errorf("%s native measurement unavailable: %v", field.name, err)
			continue
		}
		if got != field.want {
			t.Errorf("%s=%d, want raw total %d", field.name, got, field.want)
		}
	}
}

// TestKernelCounterZeroMeasurements distinguishes a supplied zero snapshot from
// unavailable diagnostics by reading each counter through its public contract.
func TestKernelCounterZeroMeasurements(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	_, opened := newFactoryCounterSession(t, &iface.InterfaceStats{})
	counters, err := opened.Dataplane().Counters()
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []uint64{
		counters.RxBytes, counters.TxBytes, counters.RxPackets, counters.TxPackets,
	} {
		if value != 0 {
			t.Errorf("zero snapshot returned traffic total %d", value)
		}
	}
	for _, field := range []struct {
		name    string
		reading ppp.CounterReading
	}{
		{"rx-errors", counters.RxErrors},
		{"tx-errors", counters.TxErrors},
		{"rx-dropped", counters.RxDropped},
		{"tx-dropped", counters.TxDropped},
	} {
		got, err := field.reading.Value()
		if err != nil {
			t.Errorf("%s measured zero unavailable: %v", field.name, err)
			continue
		}
		if got != 0 {
			t.Errorf("%s=%d, want measured zero", field.name, got)
		}
	}
}

// TestKernelCounterProviderError checks that a native read failure stays
// identifiable and cannot publish the accompanying snapshot as a valid sample.
func TestKernelCounterProviderError(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	want := errors.New("native counter read failed")
	backend, opened := newFactoryCounterSession(t, &iface.InterfaceStats{RxBytes: 101, RxErrors: 103})
	backend.statsErr = want
	counters, err := opened.Dataplane().Counters()
	if !errors.Is(err, want) {
		t.Fatalf("counter error=%v, want native provider failure %v", err, want)
	}
	if counters != (ppp.SessionCounters{}) {
		t.Fatalf("failed read returned a counter sample: %+v", counters)
	}
}

// TestKernelCounterMissingSnapshot checks that a nil native snapshot is refused
// rather than converted into a successful zero sample.
func TestKernelCounterMissingSnapshot(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	_, opened := newFactoryCounterSession(t, nil)
	counters, err := opened.Dataplane().Counters()
	if err == nil {
		t.Fatal("missing native snapshot reported success")
	}
	if counters != (ppp.SessionCounters{}) {
		t.Fatalf("missing snapshot returned a counter sample: %+v", counters)
	}
}
