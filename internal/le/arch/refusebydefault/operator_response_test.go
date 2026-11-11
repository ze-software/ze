// VALIDATES: canonical plugin operator-error responses carry a refusal even
// with a nil Go error; unrelated helpers and shadowed imports remain findings.
// PREVENTS: a command's explicit refusal counted as success, or a same-spelled
// helper hiding an actual miss from the refuse-by-default report.
package archrefusebydefault

import (
	"slices"
	"testing"

	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// TestOperatorErrorResponseCarriesRefusal holds the whole finding set to the
// fixtures' markers, including bad neighbors beside each accepted response.
func TestOperatorErrorResponseCarriesRefusal(t *testing.T) {
	for _, name := range []string{"operator_response.go", "operator_response_alias.go", "operator_response_impostor.go"} {
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
				t.Fatalf("canonical refusal precision must not annotate findings: %v", report.Annotated)
			}
		})
	}
}
