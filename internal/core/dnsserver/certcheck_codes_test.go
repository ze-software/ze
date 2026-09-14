package dnsserver

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestCertMaterialCodesRegistered drives CheckCertMaterial into its refusals
// and looks up the code each one carries.
//
// The codes come from the producer rather than from a list here, so a refusal
// that starts answering a code the registry does not hold turns this red.
//
// VALIDATES: `ze explain <code>` resolves every code the DoT/DoH certificate
// check reports.
// PREVENTS: a diagnostic an operator cannot look up.
func TestCertMaterialCodesRegistered(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()

	now := time.Now()
	refusals := [][]CertProblem{
		CheckCertMaterial("cert.pem", "", now),
		CheckCertMaterial(t.TempDir()+"/absent.pem", t.TempDir()+"/absent.key", now),
	}
	seen := 0
	for _, problems := range refusals {
		for _, problem := range problems {
			seen++
			if diagnostic.Lookup(problem.Code) == nil {
				t.Errorf("CheckCertMaterial answers code %q, which the code registry does not hold", problem.Code)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no refusal was produced, so no code was judged")
	}
}
