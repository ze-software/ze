// Design: website/AI.md -- Markdown command mirrors preserve the live contract.
// Related: command_surfaces.go -- equivalentMarkdownGroups reads displayed operator names.

package docyangcontract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEquivalentMarkdownEscapedOperatorGroupsKeepIndividualNames reads the
// escaped prose emitted by equivalentDetailMirror/markdownProse. Display escapes
// must not combine valid operator names or hide an additional unknown operator.
func TestEquivalentMarkdownEscapedOperatorGroupsKeepIndividualNames(t *testing.T) {
	commands, err := parseCommandCatalog("operator mirror fixture", []byte(renderedCommandCatalogFixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 1 {
		t.Fatalf("operator mirror fixture has %d commands, want 1", len(commands))
	}
	for _, test := range []struct {
		name         string
		value        string
		issueCount   int
		extraName    string
		missingNames []string
	}{
		{name: "escaped display separators", value: `json\, save`},
		{
			name: "escaped display retains unknown operator", value: `json\, save\, catalog\-absent`,
			issueCount: 1, extraName: "catalog-absent",
		},
		{
			name: "a code span retains its comma", value: "`json, save`",
			issueCount: 3, extraName: "json, save", missingNames: []string{"json", "save"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writePublishedCommandSurfaceFixture(t, root, false)
			path := filepath.Join(root, "website", "reference", "command-equivalents", "show-test", "index.md")
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			const original = "- Pipes, always: json, save"
			if strings.Count(string(body), original) != 1 {
				t.Fatal("operator mirror fixture must have exactly one always group")
			}
			content := strings.Replace(string(body), original, "- Pipes, always: "+test.value, 1)
			issues := validateEquivalentMarkdownContract(path, content, &commands[0])
			if len(issues) != test.issueCount {
				t.Fatalf("operator group has %d issues, want %d: %+v", len(issues), test.issueCount, issues)
			}
			var details strings.Builder
			for _, issue := range issues {
				details.WriteString(issue.Detail)
				details.WriteByte('\n')
			}
			if test.extraName != "" {
				want := `catalog-absent operator in always group "` + test.extraName + `"`
				if !strings.Contains(details.String(), want) {
					t.Fatalf("the unknown operator lost its own identity: %+v", issues)
				}
			}
			for _, name := range test.missingNames {
				want := `missing always availability for operator "` + name + `"`
				if !strings.Contains(details.String(), want) {
					t.Errorf("a code span supplied individual operator %q: %+v", name, issues)
				}
			}
		})
	}
}

// TestEquivalentMarkdownRefusesUnclosedOperatorCodeSpan prevents an incomplete
// code span from supplying even the valid operator names before it.
func TestEquivalentMarkdownRefusesUnclosedOperatorCodeSpan(t *testing.T) {
	values, valid := splitEquivalentMarkdownGroup("json, `save")
	if valid || values != nil {
		t.Fatalf("an unclosed operator code span supplied a group: %q, valid %v", values, valid)
	}

	const label = "Pipes, always"
	groups := equivalentMarkdownGroups("- Pipes, always: `json, save`", label)
	if len(groups) != 1 || len(groups[0]) != 1 || groups[0][0] != "json, save" {
		t.Fatalf("a closed code span lost its literal comma: %q", groups)
	}
}
