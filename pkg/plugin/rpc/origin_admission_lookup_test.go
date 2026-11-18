// Design: docs/architecture/api/process-protocol.md -- exact original accepted-record lookup.
package rpc

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func originAdmissionLookupFixture() (OriginAdmissionLookupInput, OriginAdmissionLookupOutput) {
	input := OriginAdmissionLookupInput{AdmissionID: 91, Family: "ipv4/unicast", Prefix: "198.51.100.0/24",
		Destination: "192.0.2.7", PeerOwner: 17, Operation: OriginIntakeAnnounce,
		DeadlineMS: time.Now().Add(time.Second).UnixMilli()}
	output := OriginAdmissionLookupOutput{AdmissionID: input.AdmissionID, Family: input.Family, Prefix: input.Prefix,
		Destination: input.Destination, PeerOwner: input.PeerOwner, Operation: input.Operation, State: OriginAdmissionCurrent,
		Snapshot: OriginAdmissionSnapshot{Protocol: "static", SourceInstance: 3, Table: 0, SourceMetric: 7,
			DistanceOverride: 0, HasDistanceOverride: true, NextHop: "192.0.2.9", OriginASN: 64496,
			Community: []uint32{64512<<16 | 9}, Tag: 42, Metric: 303, HasMetric: true, RuleGeneration: 0,
			PeerScope: OriginAdmissionAllPeers}}
	return input, output
}

// Exact echoes retain original preference separately from transformed MED,
// including explicit valid zero distance and initial evaluator generation.
func TestOriginAdmissionLookupKeepsExactOriginalSnapshot(t *testing.T) {
	input, output := originAdmissionLookupFixture()
	require.NoError(t, input.Validate(time.Now()))
	require.NoError(t, output.Validate(input))
	data, err := json.Marshal(output)
	require.NoError(t, err)
	var decoded OriginAdmissionLookupOutput
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.NoError(t, decoded.Validate(input))
	require.Equal(t, output, decoded)
	require.NotEqual(t, decoded.Snapshot.SourceMetric, decoded.Snapshot.Metric)
	output.Snapshot.PeerScope, output.Snapshot.Peer = OriginAdmissionExactPeer, input.Destination
	require.NoError(t, output.Validate(input))
	output.Family, input.Family = "ipv6/unicast", "ipv6/unicast"
	output.Prefix, input.Prefix = "2001:db8::/48", "2001:db8::/48"
	require.NoError(t, input.Validate(time.Now()))
	require.NoError(t, output.Validate(input))
}

// Every input discriminator is checked before a raw or typed callback enters.
func TestOriginAdmissionLookupRejectsMalformedInput(t *testing.T) {
	changes := map[string]func(*OriginAdmissionLookupInput){
		"missing-id":        func(value *OriginAdmissionLookupInput) { value.AdmissionID = 0 },
		"missing-owner":     func(value *OriginAdmissionLookupInput) { value.PeerOwner = 0 },
		"invalid-peer":      func(value *OriginAdmissionLookupInput) { value.Destination = "*" },
		"unknown-operation": func(value *OriginAdmissionLookupInput) { value.Operation = 99 },
		"zero-operation":    func(value *OriginAdmissionLookupInput) { value.Operation = OriginIntakeUnspecified },
		"expired":           func(value *OriginAdmissionLookupInput) { value.DeadlineMS = time.Now().Add(-time.Second).UnixMilli() },
		"unbounded":         func(value *OriginAdmissionLookupInput) { value.DeadlineMS = time.Now().Add(time.Minute).UnixMilli() },
		"host-bits":         func(value *OriginAdmissionLookupInput) { value.Prefix = "198.51.100.1/24" },
		"wrong-family":      func(value *OriginAdmissionLookupInput) { value.Family = "ipv6/unicast" },
		"unknown-family":    func(value *OriginAdmissionLookupInput) { value.Family = "ipv4 labeled-unicast" },
		"missing-prefix":    func(value *OriginAdmissionLookupInput) { value.Prefix = "" },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			value, _ := originAdmissionLookupFixture()
			require.NoError(t, value.Validate(time.Now()))
			change(&value)
			require.Error(t, value.Validate(time.Now()))
		})
	}
}

// A changed identity, unknown class or implicit wildcard cannot substitute for
// the exact record. Each refusal starts beside the same useful valid snapshot.
func TestOriginAdmissionLookupRejectsReboundOrUnknownSnapshot(t *testing.T) {
	input, _ := originAdmissionLookupFixture()
	changes := map[string]func(*OriginAdmissionLookupOutput){
		"id":                func(value *OriginAdmissionLookupOutput) { value.AdmissionID++ },
		"family":            func(value *OriginAdmissionLookupOutput) { value.Family = "ipv6/unicast" },
		"prefix":            func(value *OriginAdmissionLookupOutput) { value.Prefix = "203.0.113.0/24" },
		"destination":       func(value *OriginAdmissionLookupOutput) { value.Destination = "192.0.2.8" },
		"owner":             func(value *OriginAdmissionLookupOutput) { value.PeerOwner++ },
		"operation":         func(value *OriginAdmissionLookupOutput) { value.Operation = OriginIntakeWithdraw },
		"missing-state":     func(value *OriginAdmissionLookupOutput) { value.State = OriginAdmissionRecordUnspecified },
		"unknown-state":     func(value *OriginAdmissionLookupOutput) { value.State = 99 },
		"empty-snapshot":    func(value *OriginAdmissionLookupOutput) { value.Snapshot = OriginAdmissionSnapshot{} },
		"missing-protocol":  func(value *OriginAdmissionLookupOutput) { value.Snapshot.Protocol = "" },
		"control-protocol":  func(value *OriginAdmissionLookupOutput) { value.Snapshot.Protocol = "static\n" },
		"unknown-class":     func(value *OriginAdmissionLookupOutput) { value.Snapshot.BGPClass = 99 },
		"class-on-static":   func(value *OriginAdmissionLookupOutput) { value.Snapshot.BGPClass = OriginAdmissionEBGP },
		"missing-bgp-class": func(value *OriginAdmissionLookupOutput) { value.Snapshot.Protocol = "bgp" },
		"missing-scope": func(value *OriginAdmissionLookupOutput) {
			value.Snapshot.PeerScope = OriginAdmissionPeerScopeUnspecified
		},
		"unknown-scope":      func(value *OriginAdmissionLookupOutput) { value.Snapshot.PeerScope = 99 },
		"all-with-peer":      func(value *OriginAdmissionLookupOutput) { value.Snapshot.Peer = input.Destination },
		"exact-without-peer": func(value *OriginAdmissionLookupOutput) { value.Snapshot.PeerScope = OriginAdmissionExactPeer },
		"another-exact-peer": func(value *OriginAdmissionLookupOutput) {
			value.Snapshot.PeerScope = OriginAdmissionExactPeer
			value.Snapshot.Peer = "192.0.2.8"
		},
		"invalid-next-hop": func(value *OriginAdmissionLookupOutput) { value.Snapshot.NextHop = "invented" },
		"oversized":        func(value *OriginAdmissionLookupOutput) { value.Snapshot.Community = make([]uint32, MaxMessageSize) },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			_, value := originAdmissionLookupFixture()
			require.NoError(t, value.Validate(input))
			change(&value)
			require.Error(t, value.Validate(input))
		})
	}
	var missing *OriginAdmissionLookupOutput
	require.Error(t, missing.Validate(input))
	for _, class := range []OriginAdmissionBGPClass{OriginAdmissionIBGP, OriginAdmissionEBGP} {
		_, value := originAdmissionLookupFixture()
		value.Snapshot.Protocol, value.Snapshot.BGPClass = "bgp", class
		require.NoError(t, value.Validate(input))
	}
}

// The exact retiring record remains useful for cleanup but cannot authorize a
// fresh announcement. Current records still answer either exact operation.
func TestOriginAdmissionLookupRetiringIsWithdrawalOnly(t *testing.T) {
	input, output := originAdmissionLookupFixture()
	output.State = OriginAdmissionRetiring
	require.ErrorContains(t, output.Validate(input), "retiring")
	input.Operation, output.Operation = OriginIntakeWithdraw, OriginIntakeWithdraw
	require.NoError(t, output.Validate(input))
	output.State = OriginAdmissionCurrent
	require.NoError(t, output.Validate(input))
}

// Explicit zero/false facts survive decode; omitting or nulling any required
// field refuses without destroying the receiver's previous usable value.
func TestOriginAdmissionLookupRequiresExplicitSourceFacts(t *testing.T) {
	input, output := originAdmissionLookupFixture()
	data, err := json.Marshal(output)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &fields))
	var snapshot map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(fields["snapshot"], &snapshot))
	for _, name := range []string{"source-instance", "table", "source-metric", "distance-override", "has-distance-override", "next-hop", "origin-asn", "tag", "metric", "has-metric", "rule-generation", "peer-scope", "peer", "bgp-class", "protocol"} {
		for _, missing := range []bool{false, true} {
			saved := snapshot[name]
			if missing {
				delete(snapshot, name)
			} else {
				snapshot[name] = json.RawMessage("null")
			}
			fields["snapshot"], err = json.Marshal(snapshot)
			require.NoError(t, err)
			invalid, err := json.Marshal(fields)
			require.NoError(t, err)
			value := output
			require.Error(t, json.Unmarshal(invalid, &value), name)
			require.Equal(t, output, value, name)
			snapshot[name] = saved
		}
	}
	for _, name := range []string{"admission-id", "family", "prefix", "destination", "peer-owner", "operation", "deadline-ms"} {
		encoded, err := json.Marshal(input)
		require.NoError(t, err)
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(encoded, &raw))
		delete(raw, name)
		invalid, err := json.Marshal(raw)
		require.NoError(t, err)
		var decoded OriginAdmissionLookupInput
		require.Error(t, json.Unmarshal(invalid, &decoded), name)
	}
}
