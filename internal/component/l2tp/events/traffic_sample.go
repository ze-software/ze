// Design: docs/architecture/l2tp/subscriber-session-model.md -- immutable accounting measurements.
package events

import (
	"encoding/json"
	"errors"
)

// TrafficTotals are raw cumulative counters read from one original PPP owner.
// They carry no identity; their event MUST retain its Birth and NetworkLifetime.
type TrafficTotals struct {
	RxBytes   uint64 `json:"rx-bytes"`
	TxBytes   uint64 `json:"tx-bytes"`
	RxPackets uint64 `json:"rx-packets"`
	TxPackets uint64 `json:"tx-packets"`
}

// TrafficSample is a self-contained immutable measurement or unavailability
// cause. Its zero value is Unknown; consumers MUST check Counters, including
// after JSON decoding. No live interface, function or ownership pointer crosses
// the EventBus. Safe to copy and read concurrently.
type TrafficSample struct {
	totals      *TrafficTotals
	unavailable string
}

var errTrafficSampleAbsent = errors.New("PPP traffic counters were not captured")

// MeasuredTrafficCounters retains a confirmed raw snapshot, including zero.
// The producer MUST call this only after the original owner read succeeds.
func MeasuredTrafficCounters(totals TrafficTotals) TrafficSample {
	return TrafficSample{totals: &totals}
}

// UnavailableTrafficCounters preserves the cause rather than inventing zero.
// An absent cause describes an absent observation, never a measurement.
func UnavailableTrafficCounters(cause error) TrafficSample {
	if cause == nil {
		cause = errTrafficSampleAbsent
	}
	return TrafficSample{unavailable: cause.Error()}
}

// Counters returns only a confirmed measurement; a zero sample refuses.
func (sample TrafficSample) Counters() (TrafficTotals, error) {
	if sample.totals != nil {
		return *sample.totals, nil
	}
	if sample.unavailable == "" {
		return TrafficTotals{}, errTrafficSampleAbsent
	}
	return TrafficTotals{}, errors.New(sample.unavailable)
}

// MarshalJSON encodes either all four measured totals or an explicit cause.
func (sample TrafficSample) MarshalJSON() ([]byte, error) {
	if sample.totals != nil {
		return json.Marshal(struct {
			Totals TrafficTotals `json:"totals"`
		}{Totals: *sample.totals})
	}
	cause := sample.unavailable
	if cause == "" {
		cause = errTrafficSampleAbsent.Error()
	}
	return json.Marshal(struct {
		Unavailable string `json:"unavailable"`
	}{Unavailable: cause})
}

// UnmarshalJSON requires exactly one variant, with all four counter fields for
// a measurement. Missing/null fields and malformed variants leave the original
// sample unchanged. Input size is bounded by the existing EventBus frame.
func (sample *TrafficSample) UnmarshalJSON(data []byte) error {
	if sample == nil {
		return errors.New("l2tp: nil traffic sample receiver")
	}
	var variant map[string]json.RawMessage
	if err := json.Unmarshal(data, &variant); err != nil {
		return err
	}
	if len(variant) != 1 {
		return errors.New("l2tp: traffic sample requires one measured or unavailable variant")
	}
	if raw, measured := variant["totals"]; measured {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return err
		}
		if len(fields) != 4 {
			return errors.New("l2tp: measured traffic sample requires four counters")
		}
		var totals struct {
			RxBytes   *uint64 `json:"rx-bytes"`
			TxBytes   *uint64 `json:"tx-bytes"`
			RxPackets *uint64 `json:"rx-packets"`
			TxPackets *uint64 `json:"tx-packets"`
		}
		if err := json.Unmarshal(raw, &totals); err != nil {
			return err
		}
		if totals.RxBytes == nil {
			return errors.New("l2tp: measured traffic sample has no rx-bytes")
		}
		if totals.TxBytes == nil {
			return errors.New("l2tp: measured traffic sample has no tx-bytes")
		}
		if totals.RxPackets == nil {
			return errors.New("l2tp: measured traffic sample has no rx-packets")
		}
		if totals.TxPackets == nil {
			return errors.New("l2tp: measured traffic sample has no tx-packets")
		}
		*sample = MeasuredTrafficCounters(TrafficTotals{RxBytes: *totals.RxBytes, TxBytes: *totals.TxBytes, RxPackets: *totals.RxPackets, TxPackets: *totals.TxPackets})
		return nil
	}
	if raw, unavailable := variant["unavailable"]; unavailable {
		var cause string
		if err := json.Unmarshal(raw, &cause); err != nil {
			return err
		}
		if cause == "" {
			return errors.New("l2tp: unavailable traffic sample has no cause")
		}
		*sample = UnavailableTrafficCounters(errors.New(cause))
		return nil
	}
	return errors.New("l2tp: unknown traffic sample variant")
}
