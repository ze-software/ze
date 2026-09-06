// Design: docs/architecture/diagnostics/crash-capture.md -- crash capture doctor checks
//
// VALIDATES: AC-9: each of the three runtime dependencies has a registered check
//            that fires with its own diagnostic code, and stays silent on a box
//            that did not ask for kernel capture.
// PREVENTS:  a check that is written and never registered, which reports
//            nothing while looking like coverage, and a check that warns every
//            operator about a feature they never enabled.

package crashes

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// stubReadiness points the three checks at a fixed answer, so each branch is
// reachable without a reserved region or a mounted pstore.
func stubReadiness(t *testing.T, readiness crashlog.Readiness) {
	t.Helper()
	previous := crashDoctorReadiness
	crashDoctorReadiness = func(diagnostic.DoctorCheckContext) crashlog.Readiness { return readiness }
	t.Cleanup(func() { crashDoctorReadiness = previous })
}

func codesOf(diags []diagnostic.Diagnostic) []string {
	codes := make([]string, 0, len(diags))
	for i := range diags {
		codes = append(codes, diags[i].Code)
	}
	return codes
}

func TestDoctorReportsCrashCaptureReadiness(t *testing.T) {
	cases := []struct {
		name      string
		readiness crashlog.Readiness
		want      []string
	}{
		{
			name: "configured with no reservation on the running kernel",
			readiness: crashlog.Readiness{
				Configured:        true,
				Directory:         "/perm/ze/crash",
				DirectoryWritable: true,
				Reason:            "reboot to arm it",
			},
			want: []string{"doctor-crash-capture-unarmed"},
		},
		{
			name: "reservation present and the record store unreadable",
			readiness: crashlog.Readiness{
				Configured:        true,
				Region:            crashlog.ReserveRegionName,
				DirectoryWritable: true,
				Reason:            "mount pstore: operation not permitted",
			},
			want: []string{"doctor-crash-capture-pstore"},
		},
		{
			name: "armed and healthy",
			readiness: crashlog.Readiness{
				Configured:        true,
				Armed:             true,
				Region:            crashlog.ReserveRegionName,
				PstoreAvailable:   true,
				DirectoryWritable: true,
			},
			want: nil,
		},
		{
			name:      "no crash directory anywhere",
			readiness: crashlog.Readiness{},
			want:      []string{"doctor-crash-directory-unwritable"},
		},
		{
			name: "capture never enabled on a box with a writable directory",
			readiness: crashlog.Readiness{
				Directory:         "/perm/ze/crash",
				DirectoryWritable: true,
			},
			want: nil,
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.name, func(t *testing.T) {
			stubReadiness(t, tc.readiness)

			var diags []diagnostic.Diagnostic
			ctx := diagnostic.DoctorCheckContext{}
			diags = append(diags, checkCrashCaptureArmed(ctx)...)
			diags = append(diags, checkCrashCapturePstore(ctx)...)
			diags = append(diags, checkCrashDirectory(ctx)...)

			got := codesOf(diags)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("codes = %v, want %v", got, tc.want)
			}
			for j := range diags {
				if diags[j].Message == "" {
					t.Fatalf("diagnostic %s carries no message", diags[j].Code)
				}
			}
		})
	}
}

func TestCrashDoctorChecksAreRegistered(t *testing.T) {
	// A check that exists and is not registered runs never. The registry is the
	// only thing that makes `ze doctor` reach it, so the registration is what
	// this asserts, not the function's existence.
	names := diagnostic.DoctorCheckNames()
	for _, name := range []string{"crash-capture-armed", "crash-capture-pstore", "crash-directory-writable"} {
		if !slices.Contains(names, name) {
			t.Fatalf("doctor check %q is not registered: %v", name, names)
		}
	}
}

func TestCrashDoctorCodesAreRegistered(t *testing.T) {
	// Every validation error an agent sees carries a stable code with an
	// explanation behind it, so `ze explain <code>` answers.
	diagnostic.RegisterBuiltinCodes()
	for _, code := range []string{
		"doctor-crash-capture-unarmed",
		"doctor-crash-capture-pstore",
		"doctor-crash-directory-unwritable",
	} {
		meta := diagnostic.Lookup(code)
		if meta == nil {
			t.Fatalf("diagnostic code %q has no registered explanation", code)
		}
		if meta.Title == "" || meta.Description == "" {
			t.Fatalf("diagnostic code %q has an empty title or description", code)
		}
	}
}
