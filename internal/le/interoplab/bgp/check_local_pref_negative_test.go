// Design: docs/architecture/testing/interop.md -- actual-destination LOCAL_PREF proof.
package bgp

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestLocalPrefExternalWireViolationRefused isolates forbidden LOCAL_PREF on
// an otherwise complete external-session capture. The same checker accepts the
// stripped control, then rejects both a nonzero and zero attribute at assertion
// 13 with the wire semantic cause. Normalized RIB answers remain identical.
// These fixtures judge the checker, not a newly observed native daemon run.
// RFC requirement: RFC4271-5.1.5-2 negative -- LOCAL_PREF appearing on the
// external destination wire is refused by requireLocalPrefInterval, including
// value zero, after the original-session interval has been validated.
func TestLocalPrefExternalWireViolationRefused(t *testing.T) {
	for _, tc := range []struct {
		name       string
		preference int
	}{
		{"stripped", -1},
		{"leaked200", 200},
		{"leaked0", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			lab := &localPrefWireLab{
				wire: localPrefTestCapture(t, tc.preference), reverse: localPrefTestReverse(t),
			}
			err := checkLocalPrefStrip(ctx, &interoplab.CheckContext{
				Network: interoplab.Network{IPv4: netip.MustParsePrefix("172.30.0.0/24")}, Lab: lab,
			})
			if lab.releases != 1 {
				t.Fatalf("injector releases=%d, want one after healthy session controls", lab.releases)
			}
			if lab.wireReads != 2 {
				t.Fatalf("wire reads=%d, want both original-session directions", lab.wireReads)
			}
			if tc.preference < 0 {
				if err != nil {
					t.Fatalf("stripped baseline refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("external wire carrying LOCAL_PREF %d accepted", tc.preference)
			}
			if !strings.HasPrefix(err.Error(), "scenario bgp-local-pref-strip-gobgp assertion 13:") {
				t.Fatalf("wrong assertion rejected LOCAL_PREF %d: %v", tc.preference, err)
			}
			cause, ok := errors.AsType[localPrefWireSemanticError](err)
			if !ok {
				t.Fatalf("refusal lacks the LOCAL_PREF wire cause: %v", err)
			}
			if cause.Error() != "LOCAL_PREF appeared on the actual external destination wire" {
				t.Fatalf("wrong wire refusal: %v", cause)
			}
		})
	}
}
