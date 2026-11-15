package ruleset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeAgentDefinition writes one mirrored definition named agentType.md
// with the given frontmatter, and answers the checkout root.
func writeAgentDefinition(t *testing.T, agentType, frontmatter string) string {
	t.Helper()
	return fixtureTree(t, map[string]string{
		AgentDefinitionsDir + "/" + agentType + ".md": "---\n" + frontmatter + "---\n\nBody.\n",
	})
}

// TestAgentOmitsInstructionsReadsTheFrontmatterTheHarnessReads holds the
// frontmatter reader to the values YAML gives the harness.
//
// VALIDATES: a trailing comment, a capitalised or quoted true, a quoted name,
// and an indented key inside a multi-line description (for name and for
// omitClaudeMd) are each read as an agent that omits the instruction files;
// so are an indented "---" inside a description, which does not close the
// block, a quoted key, and a repeated key any occurrence of which is not a
// bare false; so is a definition at the type's path whose name the reader
// cannot match (another name, a name only indented, uniformly indented
// frontmatter, no name), whatever its omitClaudeMd, and that one is answered
// AgentStartUnread, the safe side a report names; absent and false under a
// matching name, with or without a comment or repeated, are read as loading
// them. Method: each row writes one definition and asks ReadAgentStart, so a
// reader that took one occurrence of a repeated key, read an indented line as
// a key or as the closing line, kept a key's quotes or compared the raw text
// would turn a row red.
// PREVENTS: an agent that starts without the instruction files given no ban
// because its definition spells the value in another valid way.
func TestAgentOmitsInstructionsReadsTheFrontmatterTheHarnessReads(t *testing.T) {
	const key = OmitInstructionsKey
	tests := []struct {
		name        string
		frontmatter string
		start       AgentStart
	}{
		{"comment-after-true", "name: ze-read\n" + key + ": true # see the spec\n", AgentStartOmits},
		{"capitalised-true", "name: ze-read\n" + key + ": True\n", AgentStartOmits},
		{"quoted-true", "name: ze-read\n" + key + ": \"true\"\n", AgentStartOmits},
		{"quoted-name", "name: \"ze-read\"\n" + key + ": true\n", AgentStartOmits},
		{"single-quoted-name", "name: 'ze-read'\n" + key + ": true\n", AgentStartOmits},
		{"indented-name-in-description", "name: ze-read\ndescription: |\n  Reads.\n  name: x\n" + key + ": true\n", AgentStartOmits},
		{"indented-false-in-description", key + ": true\nname: ze-read\ndescription: >\n  Reads.\n  " + key + ": false\n", AgentStartOmits},
		{"tab-indented-false", "name: ze-read\n" + key + ": true\n\t" + key + ": false\n", AgentStartOmits},
		{"repeated-key-true-then-false", "name: ze-read\n" + key + ": true\n" + key + ": false\n", AgentStartOmits},
		{"repeated-key-false-then-true", "name: ze-read\n" + key + ": false\n" + key + ": true\n", AgentStartOmits},
		{"indented-closing-line-in-description", "name: ze-read\ndescription: |\n  a\n  ---\n" + key + ": true\n", AgentStartOmits},
		{"quoted-key", "name: ze-read\n\"" + key + "\": true\n", AgentStartOmits},
		{"single-quoted-key", "name: ze-read\n'" + key + "': true\n", AgentStartOmits},
		{"repeated-false", "name: ze-read\n" + key + ": false\n" + key + ": false\n", AgentStartLoads},
		{"crlf-lines", "name: ze-read\r\n" + key + ": true\r\n", AgentStartOmits},
		{"absent", "name: ze-read\n", AgentStartLoads},
		{"false", "name: ze-read\n" + key + ": false\n", AgentStartLoads},
		{"false-with-comment", "name: ze-read\n" + key + ": false # loads them\n", AgentStartLoads},
		// A definition at the type's path whose name the reader cannot match
		// cannot be read for sure, so it takes the side that sends the rule set.
		{"name-differs", "name: other\n" + key + ": false\n", AgentStartUnread},
		{"indented-name-only", "description: |\n  name: ze-read\n" + key + ": false\n", AgentStartUnread},
		{"uniformly-indented-frontmatter", "  name: ze-read\n  " + key + ": true\n", AgentStartUnread},
		{"no-name", key + ": false\n", AgentStartUnread},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := writeAgentDefinition(t, "ze-read", test.frontmatter)
			start, err := ReadAgentStart(root, "ze-read")
			if err != nil {
				t.Fatalf("ReadAgentStart: %v", err)
			}
			if start != test.start {
				t.Errorf("ReadAgentStart = %d, want %d for:\n%s", start, test.start, test.frontmatter)
			}
			want := test.start != AgentStartLoads
			if start.Omits() != want {
				t.Errorf("Omits = %v, want %v for:\n%s", start.Omits(), want, test.frontmatter)
			}
		})
	}
}

// TestReadAgentStartAnswersUndefinedOrAnErrorWithoutADefinition holds the
// answers that come from no frontmatter at all.
//
// VALIDATES: a type with no definition in the mirror, and a type that cannot
// name a file there, answer AgentStartUndefined, which does not omit; a
// definition path that exists and cannot be read as a file answers an error.
// PREVENTS: a definition the reader could not open read as one that loads the
// instruction files, so the hook sends no rule set and the sync check prints
// the always-loaded payload for an agent nobody could tell.
func TestReadAgentStartAnswersUndefinedOrAnErrorWithoutADefinition(t *testing.T) {
	root := writeAgentDefinition(t, "ze-read", "name: ze-read\n"+OmitInstructionsKey+": true\n")
	for _, agentType := range []string{"general-purpose", "plugin:ze-read", "../" + AgentDefinitionsDir + "/ze-read"} {
		start, err := ReadAgentStart(root, agentType)
		if err != nil {
			t.Fatalf("ReadAgentStart(%q): %v", agentType, err)
		}
		if start != AgentStartUndefined {
			t.Errorf("ReadAgentStart(%q) = %d, want AgentStartUndefined", agentType, start)
		}
		if start.Omits() {
			t.Errorf("ReadAgentStart(%q) omits, want an undefined agent sent no rule set", agentType)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, AgentDefinitionsDir, "ze-dir.md"), 0o750); err != nil {
		t.Fatal(err)
	}
	if start, err := ReadAgentStart(root, "ze-dir"); err == nil {
		t.Errorf("ReadAgentStart over a directory = %d with no error, want an error", start)
	}
}

// TestValidAgentTypeAdmitsOnlyAFileName holds the guard that keeps a payload's
// agent type from naming a path outside the mirror.
//
// VALIDATES: letters, digits, '-' and '_' up to agentTypeLengthMax are
// admitted; a dot segment, a separator, a plugin scope, non-ASCII, one
// character over the bound and the empty type are refused.
// PREVENTS: a hostile agent type reading a file outside .claude/agents, or a
// type the harness could not report reaching the reader.
func TestValidAgentTypeAdmitsOnlyAFileName(t *testing.T) {
	tests := []struct {
		agentType string
		valid     bool
	}{
		{"ze-read", true},
		{"ze_work2", true},
		{"ZE-Read", true},
		{strings.Repeat("a", agentTypeLengthMax), true},
		{"", false},
		{strings.Repeat("a", agentTypeLengthMax+1), false},
		{"..", false},
		{"ze.read", false},
		{"../ze-read", false},
		{"agents/ze-read", false},
		{"ze\\read", false},
		{"plugin:ze-read", false},
		{"zé-read", false},
		{"ze read", false},
	}
	for _, test := range tests {
		if got := validAgentType(test.agentType); got != test.valid {
			t.Errorf("validAgentType(%q) = %v, want %v", test.agentType, got, test.valid)
		}
	}
}
