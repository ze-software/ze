// Design: docs/contributing/documentation-testing.md -- added-symbol consumer census
// Related: wiring.go -- production references and export candidate scope
package docwiring

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWiringCountsPublicPackageAlias proves a public SDK alias consumes an
// internal export by judging the same declaration with each production tree.
func TestWiringCountsPublicPackageAlias(t *testing.T) {
	for _, consumer := range []string{"internal/client/alias.go", "cmd/client/alias.go", "pkg/sdk/alias.go"} {
		t.Run(consumer, func(t *testing.T) {
			issues := wiringPkgIssues(t, "internal/retained/holder.go", map[string]string{
				consumer: "package client\n\nimport \"example/internal/retained\"\n\ntype PublicHolder = retained.SeriesRetainer\n",
			})
			if len(issues) != 0 {
				t.Fatalf("production alias was refused: %q", issues)
			}
		})
	}
}

// TestWiringRefusesUnreferencedExports proves extending the consumer census
// preserves rejection when an internal or command export has no code reference.
// Each fixture places its only mention in a test or comment, or omits it.
func TestWiringRefusesUnreferencedExports(t *testing.T) {
	for _, source := range []string{"internal/retained/holder.go", "cmd/retained/holder.go"} {
		t.Run(source, func(t *testing.T) {
			for _, tc := range []struct {
				name string
				path string
				body string
			}{
				{name: "absent"},
				{name: "pkg-test", path: "pkg/sdk/alias_test.go", body: "package sdk\n\ntype PublicHolder = retained.SeriesRetainer\n"},
				{name: "pkg-comment", path: "pkg/sdk/alias.go", body: "package sdk\n\n// SeriesRetainer is not consumed.\n"},
				{name: "internal-test", path: "internal/client/alias_test.go", body: "package client\n\ntype PublicHolder = retained.SeriesRetainer\n"},
				{name: "internal-comment", path: "internal/client/alias.go", body: "package client\n\n// SeriesRetainer is not consumed.\n"},
				{name: "cmd-test", path: "cmd/client/alias_test.go", body: "package client\n\ntype PublicHolder = retained.SeriesRetainer\n"},
				{name: "cmd-comment", path: "cmd/client/alias.go", body: "package client\n\n// SeriesRetainer is not consumed.\n"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					consumers := make(map[string]string)
					if tc.path != "" {
						consumers[tc.path] = tc.body
					}
					issues := wiringPkgIssues(t, source, consumers)
					if len(issues) != 1 {
						t.Fatalf("got %q, want one unwired export", issues)
					}
					if !strings.Contains(issues[0], source+":3: exported type SeriesRetainer has no non-test reference") {
						t.Fatalf("wrong export refused: %q", issues)
					}
				})
			}
		})
	}
}

// TestWiringKeepsPublicExportsOutsideCandidatePopulation proves that recognizing
// pkg consumers does not introduce a new gate over public SDK declarations.
func TestWiringKeepsPublicExportsOutsideCandidatePopulation(t *testing.T) {
	issues := wiringPkgIssues(t, "pkg/sdk/holder.go", nil)
	if len(issues) != 0 {
		t.Fatalf("public declaration became an export candidate: %q", issues)
	}
}

// wiringPkgIssues adds a declaration and its consumers after the fixture's base
// commit, then judges that declaration through the real added-symbol checker.
func wiringPkgIssues(t *testing.T, source string, consumers map[string]string) []string {
	t.Helper()
	root := pushedTree(t, map[string]string{"README.md": "fixture\n"})
	files := map[string]string{source: "package retained\n\ntype SeriesRetainer interface{}\n"}
	for path, body := range consumers {
		files[path] = body
	}
	for path, body := range files {
		target := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			t.Fatalf("fixture directory: %v", err)
		}
		if err := os.WriteFile(target, []byte(body), 0o600); err != nil {
			t.Fatalf("fixture file: %v", err)
		}
	}
	issues, err := checkWiring(root, "HEAD", []string{source})
	if err != nil {
		t.Fatalf("checkWiring: %v", err)
	}
	return issues
}
