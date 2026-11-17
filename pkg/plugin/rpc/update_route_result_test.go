// Design: docs/architecture/api/process-protocol.md -- origin admission results.
package rpc

import (
	"strings"
	"testing"
)

func admissionResultFixture() *UpdateRouteOutput {
	return &UpdateRouteOutput{Admissions: []RouteAdmission{{Family: "ipv4/unicast",
		KeyNLRI: []byte{24, 10, 0, 0}, Destination: "192.0.2.7", PeerOwner: 17,
		Generation: 3, Outcome: RouteAdmissionRetained, Disposition: RouteAdmissionSuppressedSelection}}}
}

// An explicit retained loser is success with no announcement; absence is never
// accepted by accident, and malformed receipt facts are refused.
func TestUpdateRouteAdmissionResultValidity(t *testing.T) {
	out := admissionResultFixture()
	if err := out.Err(); err != nil {
		t.Fatal(err)
	}
	if !out.Retained() {
		t.Fatal("retained loser lost its admission")
	}
	if out.Announced != 0 {
		t.Fatal("retained loser counted an announcement")
	}
	for _, change := range []struct {
		name   string
		change func(*RouteAdmission)
	}{
		{"outcome", func(a *RouteAdmission) { a.Outcome = RouteAdmissionUnspecified }},
		{"unknown-outcome", func(a *RouteAdmission) { a.Outcome = RouteAdmissionOutcome(255) }},
		{"generation", func(a *RouteAdmission) { a.Generation = 0 }},
		{"scope", func(a *RouteAdmission) { a.PeerOwner = 0 }},
		{"destination", func(a *RouteAdmission) { a.Destination = "not-a-peer" }},
		{"key", func(a *RouteAdmission) { a.KeyNLRI = nil }},
		{"family", func(a *RouteAdmission) { a.Family = "" }},
		{"disposition", func(a *RouteAdmission) { a.Disposition = RouteAdmissionDispositionUnspecified }},
		{"unknown-disposition", func(a *RouteAdmission) { a.Disposition = RouteAdmissionDisposition(255) }},
	} {
		t.Run(change.name, func(t *testing.T) {
			bad := admissionResultFixture()
			change.change(&bad.Admissions[0])
			if err := bad.Err(); err == nil {
				t.Fatal("malformed admission was accepted")
			}
			if bad.Retained() {
				t.Fatal("malformed admission implies retention")
			}
		})
	}
	var missing *UpdateRouteOutput
	if err := missing.Err(); err == nil {
		t.Fatal("nil result was accepted")
	}
	if missing.Retained() {
		t.Fatal("nil result implies retention")
	}
	zero := &UpdateRouteOutput{}
	if err := zero.Err(); err != nil {
		t.Fatal("explicit control result refused", err)
	}
	if zero.Retained() {
		t.Fatal("zero counts imply retention")
	}
}

// Failure does not erase earlier command counts or warning reasons.
func TestUpdateRouteAdmissionFailurePreservesPartialResult(t *testing.T) {
	for _, outcome := range []RouteAdmissionOutcome{RouteAdmissionRefused, RouteAdmissionFailed} {
		out := admissionResultFixture()
		out.Announced, out.Withdrawn = 2, 1
		out.Warnings = []string{"withdrawal withheld at 192.0.2.8"}
		out.Admissions[0].Outcome, out.Admissions[0].Generation = outcome, 0
		out.Admissions[0].Disposition = RouteAdmissionDispositionUnspecified
		out.Admissions[0].Reason = "origin admission denied"
		if err := out.Err(); err == nil || err.Error() != "origin admission denied" {
			t.Fatalf("failure lost: %v", err)
		}
		if out.Retained() {
			t.Fatal("refusal implies retention")
		}
		if out.Announced != 2 || out.Withdrawn != 1 || len(out.Warnings) != 1 {
			t.Fatalf("partial result lost: %+v", out)
		}
		out.Admissions[0].Reason = ""
		if err := out.Err(); err == nil {
			t.Fatal("failure without its reason accepted")
		}
	}
}

// Native replies honor the same maximum framed result size as socket replies.
func TestUpdateRouteResultFrameLimit(t *testing.T) {
	output := &UpdateRouteOutput{Warnings: []string{strings.Repeat("x", MaxMessageSize)}}
	if err := output.Validate(); err == nil {
		t.Fatal("oversized result accepted by native adapter")
	}
	output.Warnings = []string{"warning reason"}
	if err := output.Validate(); err != nil {
		t.Fatal(err)
	}
}
