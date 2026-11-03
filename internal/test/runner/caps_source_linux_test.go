// Design: docs/architecture/testing/ci-format.md -- capability probe uncertainty.

package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProbeCapsSourceRefusesUncertainty reads actual files rather than replacing
// the probe verdict. An unreadable or invalid source cannot grant a capability.
// MUTATION: return true with no error from any probe source failure.
func TestProbeCapsSourceRefusesUncertainty(t *testing.T) {
	for _, source := range []struct{ name, content, cause string }{
		{"unreadable", "", "cannot read"},
		{"malformed", "CapEff:\tnot-hex\n", "cannot parse CapEff"},
		{"overflow", "CapEff:\t10000000000000000\n", "cannot parse CapEff"},
		{"missing", "Name:\tfixture\n", "no CapEff line"},
	} {
		t.Run(source.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "status")
			if source.name != "unreadable" {
				if err := os.WriteFile(path, []byte(source.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			present, err := probeCapsFile(path, capNetAdmin)
			if present {
				t.Fatal("uncertain capability source granted admission")
			}
			if err == nil {
				t.Fatal("uncertainty was reported as capability absence")
			}
			if !strings.Contains(err.Error(), source.cause) {
				t.Fatal(err)
			}
			if !strings.Contains(err.Error(), path) {
				t.Fatal("probe error lost source path")
			}

			original := hasCaps
			t.Cleanup(func() { hasCaps = original })
			hasCaps = func(tokens []string) (bool, error) {
				if len(tokens) != 1 || tokens[0] != capsNetAdmin {
					t.Fatalf("unexpected tokens: %v", tokens)
				}
				return probeCapsFile(path, capNetAdmin)
			}
			var admission Record
			_, parseErr := ParseResourceOption(&admission, "needs-linux:caps=net-admin")
			if parseErr == nil {
				t.Fatal("shared parser admitted unknown capabilities")
			}
			if !strings.Contains(parseErr.Error(), source.cause) {
				t.Fatal(parseErr)
			}
			if admission.SkipReason != "" {
				t.Fatalf("probe error became skip: %s", admission.SkipReason)
			}

			ciRoot := t.TempDir()
			ci := filepath.Join(ciRoot, "uncertain.ci")
			if err := os.WriteFile(ci, []byte("option=needs-linux:caps=net-admin\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ParseCIFile(ci); err == nil || !strings.Contains(err.Error(), source.cause) {
				t.Fatalf("ParseCIFile lost uncertainty: %v", err)
			}
			if guest, err := NeedsGuest(ci); guest || err == nil || !strings.Contains(err.Error(), source.cause) {
				t.Fatalf("NeedsGuest guessed placement: %v, %v", guest, err)
			}
			tests := NewEncodingTests(ciRoot)
			if err := tests.Discover(ciRoot); err != nil {
				t.Fatal(err)
			}
			if _, err := tests.Select(Selection{All: true}); err != nil {
				t.Fatal(err)
			}
			selected := tests.Selected()
			if len(selected) != 1 {
				t.Fatalf("lost failed record: %d", len(selected))
			}
			record := selected[0]
			if !record.ParseFailed {
				t.Fatal("discovery did not retain admission failure")
			}
			if !strings.Contains(record.Error.Error(), source.cause) {
				t.Fatal(record.Error)
			}
			pr := NewParallelRunner[int](NewColors())
			pr.SetQuiet(true)
			scheduled := pr.AddTestWithNick("uncertain", "uncertain", 0, func(context.Context, int) (bool, error) {
				t.Error("uncertain discovery reached execution")
				return true, nil
			})
			scheduled.ParseFailed, scheduled.Error = record.ParseFailed, record.Error
			if pr.Run(context.Background()) {
				t.Fatal("uncertain discovery passed")
			}
		})
	}
}

// TestProbeCapsSourceExactBits distinguishes a valid absent bit from uncertainty,
// and requires every selected bit without using the host's real privileges.
func TestProbeCapsSourceExactBits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "status")
	if err := os.WriteFile(path, []byte("Name:\tfixture\nCapEff:\t0000008000001000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		bits []int
		want bool
	}{
		{"exact", []int{capNetAdmin, capBPF}, true},
		{"absent", []int{capSysTime}, false},
		{"mixed", []int{capNetAdmin, capSysTime}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := probeCapsFile(path, tc.bits...)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("capability answer %v, want %v", got, tc.want)
			}
		})
	}
	if present, err := hasCaps([]string{"unknown-capability"}); present || err == nil {
		t.Fatalf("unknown token accepted: %v, %v", present, err)
	}
	if present, err := hasCaps(nil); !present || err != nil {
		t.Fatalf("empty requirements: %v, %v", present, err)
	}
}
