// Design: docs/contributing/rule-authoring.md -- directive metadata and body agreement.
package airules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRuleMetadataLevelFollowsBody reads actual point files through the corpus
// checker: MUST metadata cannot silently upgrade a SHOULD body, and quoted
// examples cannot upgrade the body's obligation either.
func TestRuleMetadataLevelFollowsBody(t *testing.T) {
	for _, tc := range []struct {
		name, declared, body string
		mismatch             bool
	}{
		{"must-metadata-should-body", "MUST", "An author SHOULD name the source.", true},
		{"should-metadata-must-body", "SHOULD", "An author MUST name the source.", true},
		{"matching-should", "SHOULD", "An author SHOULD name the source.", false},
		{"matching-must", "MUST", "An author MUST name the source.", false},
		{"quoted-must-is-not-obligation", "SHOULD", "An author SHOULD name the source.\n\n> An example MUST stay quoted.\n", false},
		{"must-metadata-must-not-body", "MUST", "An author MUST NOT regenerate a tagged test.", true},
		{"matching-must-not", "MUST NOT", "An author MUST NOT regenerate a tagged test.", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			points := filepath.Join(root, filepath.FromSlash(pointsRel))
			if err := os.MkdirAll(points, 0o750); err != nil {
				t.Fatal(err)
			}
			text := "---\nkind: directive\nlevel: " + tc.declared + "\n---\n" + tc.body + "\n"
			if err := os.WriteFile(filepath.Join(points, "source.md"), []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
			count, violations, err := checkPointFiles(root, points)
			if err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatalf("point population = %d, want the authored point", count)
			}
			if !tc.mismatch {
				if len(violations) != 0 {
					t.Fatalf("matching metadata refused: %+v", violations)
				}
				return
			}
			if len(violations) != 1 {
				t.Fatalf("mismatched metadata violations = %+v, want one named point", violations)
			}
			if violations[0].File != pointsRel+"/source.md" {
				t.Fatalf("mismatch names %q, want the authored point", violations[0].File)
			}
			if len(violations[0].Problems) != 1 || !strings.Contains(violations[0].Problems[0], "disagrees with the body") {
				t.Fatalf("mismatch cause = %v, want metadata/body disagreement", violations[0].Problems)
			}
		})
	}
}
