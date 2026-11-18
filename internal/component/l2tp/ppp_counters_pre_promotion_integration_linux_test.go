// Design: docs/research/l2tpv2-ze-integration.md -- original owner measurement conversion
// Related: ppp_counters_measurement_test.go -- retained portable questions
// Related: ppp_counters_created_integration_linux_test.go -- genuine original helper

//go:build integration && linux

package l2tp

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestKernelCreatedCounterMeasurementsPrePromotion retains every old numeric
// assertion on the genuine original reader before any portable root relocates.
func TestKernelCreatedCounterMeasurementsPrePromotion(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	native := &iface.InterfaceStats{
		RxBytes: 1<<40 + 11, TxBytes: 1<<41 + 13,
		RxPackets: 1<<42 + 17, TxPackets: 1<<43 + 19,
		RxErrors: 1<<44 + 23, TxErrors: 1<<45 + 29,
		RxDropped: 1<<46 + 31, TxDropped: 1<<47 + 37,
	}
	backend, handle := newKernelMeasurementSession(t, native)
	counters, err := handle.Counters()
	if err != nil {
		t.Fatal(err)
	}
	if backend.name != "ppp73" {
		t.Fatalf("counter device=%q, want ppp73", backend.name)
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

// TestKernelCreatedCounterZeroMeasurementsPrePromotion retains measured-zero
// and public availability assertions through the same genuine helper.
func TestKernelCreatedCounterZeroMeasurementsPrePromotion(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	_, handle := newKernelMeasurementSession(t, &iface.InterfaceStats{})
	counters, err := handle.Counters()
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

// TestKernelCreatedCounterProviderErrorPrePromotion retains both the populated
// snapshot and original failure sentinel; failure must publish no sample.
func TestKernelCreatedCounterProviderErrorPrePromotion(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	want := errors.New("native counter read failed")
	backend, handle := newKernelMeasurementSession(t, &iface.InterfaceStats{RxBytes: 101, RxErrors: 103})
	backend.err = want
	counters, err := handle.Counters()
	if !errors.Is(err, want) {
		t.Fatalf("counter error=%v, want native provider failure %v", err, want)
	}
	if counters != (ppp.SessionCounters{}) {
		t.Fatalf("failed read returned a counter sample: %+v", counters)
	}
}

// TestKernelCreatedCounterMissingSnapshotPrePromotion retains nil refusal and
// the empty-sample check through a live original owner.
func TestKernelCreatedCounterMissingSnapshotPrePromotion(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	_, handle := newKernelMeasurementSession(t, nil)
	counters, err := handle.Counters()
	if err == nil {
		t.Fatal("missing native snapshot reported success")
	}
	if counters != (ppp.SessionCounters{}) {
		t.Fatalf("missing snapshot returned a counter sample: %+v", counters)
	}
}
