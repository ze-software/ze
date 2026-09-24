// Design: docs/architecture/testing/interop.md -- fail-closed MOBIKE scenario verdicts.
// Related: mobike.go -- real daemon movement and kernel observations.
package ipsec

import (
	"strings"
	"testing"
)

// TestMOBIKERejectsReplacementSAs distinguishes migration from successful fresh
// establishment, and from a rekey of either simplex Child SA.
func TestMOBIKERejectsReplacementSAs(t *testing.T) {
	original := mobikeIdentity{
		initiatorSPI: "0123456789abcdef", responderSPI: "fedcba9876543210",
		inboundSPI: 0x12345678, outboundSPI: 0x87654321,
	}
	if err := requireMOBIKEIdentity(original, original); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"initiator SPI", "responder SPI", "inbound ESP", "outbound ESP"} {
		t.Run(field, func(t *testing.T) {
			replacement := original
			switch field {
			case "initiator SPI":
				replacement.initiatorSPI = "1111111111111111"
			case "responder SPI":
				replacement.responderSPI = "2222222222222222"
			case "inbound ESP":
				replacement.inboundSPI++
			case "outbound ESP":
				replacement.outboundSPI++
			}
			if err := requireMOBIKEIdentity(original, replacement); err == nil {
				t.Fatal("a replacement SA passed as a migrated SA")
			}
		})
	}
}

// TestMOBIKERequiresBothMigratedDirections rejects partial migration and stale
// duplicates even when the unordered SPI set still matches the original tunnel.
func TestMOBIKERequiresBothMigratedDirections(t *testing.T) {
	identity := mobikeIdentity{inboundSPI: 1, outboundSPI: 2}
	outbound := readbackSAKey{source: mobikeZeMoved, target: swanIP, spi: 2}
	inbound := readbackSAKey{source: swanIP, target: mobikeZeMoved, spi: 1}
	oldInbound := readbackSAKey{source: swanIP, target: zeIP, spi: 1}
	for _, test := range []struct {
		name   string
		states map[readbackSAKey]espLifetime
		accept bool
	}{
		{name: "both migrated", states: map[readbackSAKey]espLifetime{outbound: {}, inbound: {}}, accept: true},
		{name: "no states"},
		{name: "outbound only", states: map[readbackSAKey]espLifetime{outbound: {}}},
		{name: "inbound stale", states: map[readbackSAKey]espLifetime{outbound: {}, oldInbound: {}}},
		{name: "stale duplicate", states: map[readbackSAKey]espLifetime{outbound: {}, inbound: {}, oldInbound: {}}},
		{name: "SPIs reversed", states: map[readbackSAKey]espLifetime{
			{source: mobikeZeMoved, target: swanIP, spi: 1}: {},
			{source: swanIP, target: mobikeZeMoved, spi: 2}: {},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := requireMOBIKEEndpoints(test.states, mobikeZeMoved, swanIP, identity)
			if (err == nil) != test.accept {
				t.Fatalf("migration verdict: %v; accept=%v", err, test.accept)
			}
		})
	}
}

// TestMOBIKEReadsStrongSwanEndpoints requires the independent peer to retain
// the same IKE SPIs, role, installed Child SA and new UDP/4500 endpoint.
func TestMOBIKEReadsStrongSwanEndpoints(t *testing.T) {
	identity := mobikeIdentity{
		initiatorSPI: "0123456789abcdef", responderSPI: "fedcba9876543210",
		inboundSPI: 1, outboundSPI: 2,
	}
	const answer = "ze: #1, ESTABLISHED, IKEv2, 0123456789abcdef_i fedcba9876543210_r*\n" +
		"  local  '172.28.0.3' @ 172.28.0.3[4500]\n" +
		"  remote '172.28.0.2' @ 172.28.0.8[4500]\n" +
		"  ze-child: #1, reqid 1, INSTALLED, TUNNEL, ESP:AES_GCM_16-256\n"
	for _, test := range []struct {
		name   string
		answer string
		accept bool
	}{
		{name: "moved", answer: answer, accept: true},
		{name: "no answer"},
		{name: "old endpoint", answer: strings.ReplaceAll(answer, "@ 172.28.0.8", "@ 172.28.0.2")},
		{name: "port not floated", answer: strings.ReplaceAll(answer, "[4500]", "[500]")},
		{name: "replacement IKE", answer: strings.ReplaceAll(answer, "0123456789abcdef", "1111111111111111")},
		{name: "wrong role", answer: strings.ReplaceAll(answer, "_i fedcba9876543210_r*", "_i* fedcba9876543210_r")},
		{name: "missing Child", answer: strings.ReplaceAll(answer, "INSTALLED", "DELETING")},
		{name: "transport Child", answer: strings.ReplaceAll(answer, "TUNNEL", "TRANSPORT")},
		{name: "overlapping IKE", answer: answer + answer},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := requireMOBIKESwan(test.answer, mobikeZeMoved, swanIP, true, identity)
			if (err == nil) != test.accept {
				t.Fatalf("strongSwan verdict: %v; accept=%v", err, test.accept)
			}
		})
	}
}

// TestMOBIKERequiresLiveZeIdentity rejects absent or partial CLI answers rather
// than allowing two zero-valued identity snapshots to compare equal.
func TestMOBIKERequiresLiveZeIdentity(t *testing.T) {
	const answer = `[{"peer-name":"swan","state":"established","is-initiator":true,` +
		`"initiator-spi":"0123456789abcdef","responder-spi":"fedcba9876543210",` +
		`"child-sa":{"inbound-spi":1,"outbound-spi":2,"mode":"tunnel",` +
		`"remote-address":"172.28.0.3","ts-local":"10.10.0.1/32","ts-remote":"10.20.0.1/32"}}]`
	for _, test := range []struct {
		name   string
		answer string
		accept bool
	}{
		{name: "established", answer: answer, accept: true},
		{name: "empty", answer: "[]"},
		{name: "partial", answer: "[{}]"},
		{name: "not established", answer: strings.ReplaceAll(answer, "established", "dead")},
		{name: "zero IKE SPI", answer: strings.ReplaceAll(answer, "0123456789abcdef", "0000000000000000")},
		{name: "zero Child SPI", answer: strings.ReplaceAll(answer, `"outbound-spi":2`, `"outbound-spi":0`)},
		{name: "outer selector substituted", answer: strings.ReplaceAll(answer, "10.10.0.1/32", "172.28.0.8/32")},
		{name: "stale Child endpoint", answer: strings.ReplaceAll(answer, "172.28.0.3", "172.28.0.9")},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := mobikeZeIdentity(test.answer, swanIP, true)
			if (err == nil) != test.accept {
				t.Fatalf("Ze verdict: %v; accept=%v", err, test.accept)
			}
		})
	}
}
