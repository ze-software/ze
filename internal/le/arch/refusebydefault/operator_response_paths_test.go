// VALIDATES: canonical operator-error responses remain refusals in switch
// defaults, unmatched-switch returns, and branches with quiet error logs.
// PREVENTS: correcting only lookup misses while another rule still counts
// the same typed refusal as successful; impostors must remain findings.
package archrefusebydefault

import (
	"slices"
	"testing"

	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// TestOperatorErrorResponseCarriesAllReturnedRefusals checks every returned-
// error detection path against canonical responses and bad neighbors.
func TestOperatorErrorResponseCarriesAllReturnedRefusals(t *testing.T) {
	for _, name := range []string{"operator_response_paths.go", "operator_response_paths_alias.go"} {
		t.Run(name, func(t *testing.T) {
			source := fixture(t, name)
			want := markers(t, source, "want")
			if len(want) == 0 {
				t.Fatal("fixture has no bad control")
			}
			tree := t.TempDir()
			rel := "internal/fixture/" + name
			writeSource(t, tree, rel, source)
			report, err := Check(tree, repochanged.ChangedLines{rel: every})
			if err != nil {
				t.Fatal(err)
			}
			if got := rows(report.Findings); !slices.Equal(got, want) {
				t.Fatalf("findings = %v, want %v", got, want)
			}
			if len(report.Annotated) != 0 {
				t.Fatalf("typed refusals must not require annotations: %v", report.Annotated)
			}
		})
	}
}
