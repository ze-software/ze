// VALIDATES: the READER half of the declared-failure-group protocol. A stage
// that declares its groups is read back whole, a line that does not parse
// becomes a group saying so, and every doubt about the set refuses it rather
// than trusting a partial one.
// PREVENTS: a failure that vanishes between the producer and the ledger. A
// group the reader drops is a red nobody is charged for, which is the one
// outcome attribution must never produce (ai/rules/evidence.md).

package verifyengine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testArtifactTime is the fixed instant the index records, so the artifact is a
// pure function of the report the test built.
func testArtifactTime() time.Time {
	return time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
}

// declaredStage writes one stage log holding body and answers the report that
// names it, as writeRunArtifacts would see it.
func declaredStage(t *testing.T, body string) (string, StageReport) {
	t.Helper()
	root := t.TempDir()
	log := filepath.ToSlash(filepath.Join("tmp", "verify", "stage.log"))
	full := filepath.Join(root, filepath.FromSlash(log))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("create the stage log directory: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("write the stage log: %v", err)
	}
	return root, StageReport{Identity: Identity{Name: "doc wiring"}, Code: 1, Log: log}
}

// groupLine renders one declared group the way a producer emits it.
func groupLine(id, kind string, related ...string) string {
	quoted := make([]string, 0, len(related))
	for _, path := range related {
		quoted = append(quoted, `"`+path+`"`)
	}
	return `VERIFY FAILURE GROUP: {"stage":"doc wiring","group-id":"` + id +
		`","kind":"` + kind + `","related":[` + strings.Join(quoted, ",") +
		`],"summary":"a fixture failure","rerun":"le doc wiring"}`
}

func TestDeclaredGroupsAreReadBackWhenTheCountAgrees(t *testing.T) {
	root, stage := declaredStage(t, strings.Join([]string{
		"Running wiring...",
		groupLine("files:wiring", "files", "mine.go", "theirs.go"),
		groupLine("subcheck:ratchet", "subcheck"),
		"VERIFY FAILURE GROUPS COMPLETE: 2",
	}, "\n")+"\n")

	groups, complete := DeclaredGroups(root, stage)

	if !complete {
		t.Fatalf("a log whose count agrees was refused: %d group(s)", len(groups))
	}
	if len(groups) != 2 {
		t.Fatalf("read %d group(s), want 2", len(groups))
	}
	if groups[0].Kind != "files" || len(groups[0].Related) != 2 {
		t.Errorf("the first group is %#v, want the two files it named", groups[0])
	}
	if groups[1].Kind != "subcheck" || len(groups[1].Related) != 0 {
		t.Errorf("the second group is %#v, want a group naming no file", groups[1])
	}
	for index, group := range groups {
		if group.DetailLog != stage.Log {
			t.Errorf("group %d names detail log %q, want the stage's own %q", index, group.DetailLog, stage.Log)
		}
		if group.Parallel == "" {
			t.Errorf("group %d carries no rerun advice", index)
		}
	}
}

func TestAMalformedGroupLineBecomesAGroupThatSaysSo(t *testing.T) {
	root, stage := declaredStage(t, strings.Join([]string{
		"VERIFY FAILURE GROUP: {this is not json",
		groupLine("files:wiring", "files", "mine.go"),
		"VERIFY FAILURE GROUPS COMPLETE: 2",
	}, "\n")+"\n")

	groups, complete := DeclaredGroups(root, stage)

	if !complete || len(groups) != 2 {
		t.Fatalf("a malformed line lost its group: complete=%v groups=%d", complete, len(groups))
	}
	unparsed := groups[0]
	if unparsed.Kind != "unparsed" {
		t.Fatalf("the malformed line produced kind %q, want unparsed", unparsed.Kind)
	}
	// The kind matters as much as the group's existence: `unparsed` is outside
	// the path-bearing set groupRelatedPaths admits (internal/le/commit/
	// verification.go), so the stage is CHARGED rather than dropped.
	if len(unparsed.Excerpt) != 1 || !strings.Contains(unparsed.Excerpt[0], "not json") {
		t.Errorf("the unparsed group does not carry the line it could not read: %#v", unparsed)
	}
	if unparsed.Summary == "" {
		t.Errorf("the unparsed group says nothing about why it could not be read")
	}
}

func TestEveryDoubtAboutTheDeclaredSetRefusesIt(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			// The producer emitted fewer lines than it counted, which is what a
			// truncated stage log looks like from here. Trusting the survivors
			// would attribute the red to the files that happened to reach disk.
			name: "the count is larger than the set",
			body: groupLine("files:wiring", "files", "mine.go") + "\nVERIFY FAILURE GROUPS COMPLETE: 3\n",
		},
		{
			name: "no count at all",
			body: groupLine("files:wiring", "files", "mine.go") + "\n",
		},
		{
			name: "two counts, so neither is the run's own",
			body: groupLine("files:wiring", "files", "mine.go") +
				"\nVERIFY FAILURE GROUPS COMPLETE: 1\nVERIFY FAILURE GROUPS COMPLETE: 1\n",
		},
		{
			name: "the count is not a number",
			body: groupLine("files:wiring", "files", "mine.go") + "\nVERIFY FAILURE GROUPS COMPLETE: many\n",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root, stage := declaredStage(t, testCase.body)

			if _, complete := DeclaredGroups(root, stage); complete {
				t.Error("the declared set was accepted, so a partial set would be trusted")
			}
		})
	}
}

func TestAStageLogThatCannotBeReadDeclaresNothing(t *testing.T) {
	root := t.TempDir()
	stage := StageReport{Identity: Identity{Name: "doc wiring"}, Code: 1, Log: "tmp/verify/absent.log"}

	groups, complete := DeclaredGroups(root, stage)

	if complete || len(groups) != 0 {
		t.Fatalf("an unreadable stage log answered %d group(s), complete=%v", len(groups), complete)
	}
}

func TestAFailingStageWithNoDeclaredGroupsGetsOneThatNamesNoFile(t *testing.T) {
	root, stage := declaredStage(t, "Running wiring...\nwiring failed\n")
	report := Report{
		Mode: Mode, Code: 1, LogDir: filepath.ToSlash(filepath.Join("tmp", "verify")),
		Stages: []StageReport{stage}, Console: "",
	}

	if err := writeRunArtifacts(root, report, testArtifactTime()); err != nil {
		t.Fatalf("write the run artifacts: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(FailuresJSONPath)))
	if err != nil {
		t.Fatalf("read the failure index: %v", err)
	}
	index := string(body)
	// "generic" is outside the path-bearing set, and structuralGateReds counts a
	// stage with no attributable group as blind and CHARGES it. A red stage that
	// declared nothing must never reach the ledger with an empty group list,
	// which reads as "every group's files are foreign".
	if !strings.Contains(index, `"kind": "generic"`) {
		t.Fatalf("a failing stage that declared no group got no generic group:\n%s", index)
	}
	if !strings.Contains(index, `"related": []`) {
		t.Errorf("the generic group names a file it cannot know:\n%s", index)
	}
}
