// Design: docs/architecture/l2tp/subscriber-session-model.md -- accounting lifetime and immutable measurement boundary.
package events

import (
	"encoding/json"
	"errors"
	"math"
	"testing"
)

// One logical Birth's per-network sequence is never zero, reused or wrapped.
func TestNetworkLifetimeCanonicalSequenceDoesNotWrap(t *testing.T) {
	var absent NetworkLifetime
	if absent.IsValid() {
		t.Fatal("zero sequence identifies a network")
	}
	first, err := absent.Next()
	if err != nil {
		t.Fatal(err)
	}
	second, err := first.Next()
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("replacement network reused its predecessor")
	}
	text, err := second.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	var decoded NetworkLifetime
	if err := decoded.UnmarshalText(text); err != nil {
		t.Fatal(err)
	}
	if decoded != second {
		t.Fatal("EventBus text changed the network lifetime")
	}
	for _, invalid := range []string{"", "0", "01", "+1", "-1", "18446744073709551616"} {
		if err := decoded.UnmarshalText([]byte(invalid)); err == nil {
			t.Fatalf("accepted %q", invalid)
		}
		if decoded != second {
			t.Fatalf("refused %q changed the original lifetime", invalid)
		}
	}
	if _, err := absent.MarshalText(); err == nil {
		t.Fatal("an absent network was encoded as real")
	}
	exhausted := NetworkLifetime{sequence: math.MaxUint64}
	if next, err := exhausted.Next(); err == nil || next.IsValid() {
		t.Fatalf("exhaustion=%v,%v", next, err)
	}
}

// A JSON event must preserve measured zero and every raw total, while a missing
// measurement retains its cause and cannot become an available zero sample.
func TestTrafficSampleJSONPreservesMeasurementAndUnknown(t *testing.T) {
	totals := TrafficTotals{RxBytes: math.MaxUint64, TxBytes: 23, RxPackets: 29, TxPackets: 31}
	for _, original := range []TrafficSample{MeasuredTrafficCounters(TrafficTotals{}), MeasuredTrafficCounters(totals)} {
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatal(err)
		}
		var received TrafficSample
		if err := json.Unmarshal(data, &received); err != nil {
			t.Fatal(err)
		}
		want, err := original.Counters()
		if err != nil {
			t.Fatal(err)
		}
		got, err := received.Counters()
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("event totals=%+v,want%+v", got, want)
		}
		got.RxBytes++
		preserved, err := received.Counters()
		if err != nil {
			t.Fatal(err)
		}
		if preserved != want {
			t.Fatal("a returned copy mutated its original measurement")
		}
	}
	absentCause := errors.New("original unit counter source unavailable")
	unavailable := UnavailableTrafficCounters(absentCause)
	data, err := json.Marshal(unavailable)
	if err != nil {
		t.Fatal(err)
	}
	var received TrafficSample
	if err := json.Unmarshal(data, &received); err != nil {
		t.Fatal(err)
	}
	if _, err := received.Counters(); err == nil || err.Error() != absentCause.Error() {
		t.Fatalf("unavailability cause=%v", err)
	}
	for _, unknown := range []TrafficSample{{}, UnavailableTrafficCounters(nil)} {
		if _, err := unknown.Counters(); err == nil {
			t.Fatal("unknown sample invented measured zero")
		}
	}
}

// Malformed/null/partial event fields cannot manufacture the omitted counters;
// a refusal preserves the old confirmed snapshot rather than partially decoding.
func TestTrafficSampleJSONRejectsInventedMeasurements(t *testing.T) {
	original := MeasuredTrafficCounters(TrafficTotals{RxBytes: 11, TxBytes: 13, RxPackets: 17, TxPackets: 19})
	want, err := original.Counters()
	if err != nil {
		t.Fatal(err)
	}
	for _, malformed := range []string{
		`null`, `{}`, `{"other":1}`, `{"totals":null}`, `{"totals":{}}`,
		`{"unavailable":""}`, `{"unavailable":null}`, `{"totals":{},"unavailable":"lost"}`,
		`{"totals":{"rx-bytes":0,"tx-bytes":0,"rx-packets":0}}`,
		`{"totals":{"rx-bytes":null,"tx-bytes":0,"rx-packets":0,"tx-packets":0}}`,
		`{"totals":{"rx-bytes":-1,"tx-bytes":0,"rx-packets":0,"tx-packets":0}}`,
		`{"totals":{"rx-bytes":18446744073709551616,"tx-bytes":0,"rx-packets":0,"tx-packets":0}}`,
	} {
		received := original
		if err := json.Unmarshal([]byte(malformed), &received); err == nil {
			t.Fatalf("accepted %s", malformed)
		}
		got, err := received.Counters()
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("refusal %s changed the original snapshot", malformed)
		}
	}
}
