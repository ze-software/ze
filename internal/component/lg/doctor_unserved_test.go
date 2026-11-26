// Design: docs/architecture/pki/tls-listeners.md -- the looking-glass certificate reference finding says what ze does with the name

package lg

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestLGTLSDoctorSaysWhichServersStayDown(t *testing.T) {
	// VALIDATES: the doctor finding of a looking-glass certificate reference
	// that cannot serve TLS says what ze does with it, which is to start, leave
	// down each server that asks for TLS, serve the `tls false` servers and
	// refuse a reload, and carries the fix with the name the operator
	// configured.
	// PREVENTS: a finding that reads as a refused start, which ze no longer
	// does, and a finding with no fix.
	now := time.Now()
	far := now.Add(365 * 24 * time.Hour)

	diags := lgTLSDiagnostic(lgDoctorTree(t, "true", "typo", far), now)
	if got := lgDiagCodes(diags); !slices.Equal(got, []string{"doctor-tls-reference"}) {
		t.Fatalf("codes: got %v, want [doctor-tls-reference]", got)
	}
	for _, want := range []string{
		"ze starts and leaves down each looking-glass server that asks for TLS",
		"the servers that say `tls false` serve",
		"a reload that names this certificate is refused",
	} {
		if !strings.Contains(diags[0].Message, want) {
			t.Fatalf("message %q does not say %q", diags[0].Message, want)
		}
	}
	wantFix := "Define it with `edit pki certificate typo`, or delete `environment looking-glass certificate`, then restart ze."
	if diags[0].Fix != wantFix {
		t.Fatalf("fix = %q, want %q", diags[0].Fix, wantFix)
	}
}
