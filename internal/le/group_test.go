// VALIDATES: le's own shape -- every command declares a group help renders it
// under, each group's promise holds, and a command's package sits at the path
// its name predicts.
// PREVENTS: a tool that registers into no section of the help page. A gate
// stage filed where a reader would not look for it. A generator that cannot
// regenerate. A report that writes. A package a reader cannot find from the
// command name.
package le

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// writesMarker is what an area's Subs line prints beside an action that changes
// the tree. leaction.Area.Subs writes it, and an area with a private action
// table renders the same word, so reading the registry covers both.
const writesMarker = "(writes)"

func TestEveryToolDeclaresARenderedGroup(t *testing.T) {
	if len(commandsAtStart) == 0 {
		t.Fatal("le registered no local-data command")
	}
	populated := make(map[leroot.Group]int, len(leroot.Groups()))
	for _, tool := range commandsAtStart {
		group, ok := leroot.GroupOf(tool.Name)
		if !ok {
			t.Errorf("tool %q declared no group, so help prints it under Ungrouped", tool.Name)
			continue
		}
		if !leroot.KnownGroup(group) {
			t.Errorf("tool %q declared group %q, which help has no title for", tool.Name, group)
			continue
		}
		populated[group]++
	}
	for _, group := range leroot.Groups() {
		if populated[group] == 0 {
			t.Errorf("group %q holds no command, so help renders an order nothing uses", group)
		}
	}
}

// TestGateStagesAreNotWorkflowOrReport ties the help taxonomy to the pre-commit
// population. A command the gate runs judges the tree, rewrites an artifact the
// gate compares, or runs a suite. It is never something a person merely types to
// move their own work along, and it never gates nothing.
func TestGateStagesAreNotWorkflowOrReport(t *testing.T) {
	stages := verifyengine.StagesForMode(verifyengine.Mode)
	if len(stages) == 0 {
		t.Fatal("full verification declares no stage")
	}
	for _, stage := range stages {
		name := stage.Identity.Command
		group, ok := leroot.GroupOf(name)
		if !ok {
			t.Errorf("stage %q names a command that declared no group", name)
			continue
		}
		switch group {
		case leroot.GroupGate, leroot.GroupGenerate, leroot.GroupSuite:
		default:
			t.Errorf("stage %q is declared %q; a gate stage is a gate, a generator, or a suite", name, group)
		}
	}
}

// TestGenerateAreasCanRewriteWhatTheyCheck holds the promise the section title
// makes. "Generated artifacts (check one, or rewrite it)" is a lie for an area
// that offers no rewrite, and a reader who reached that section for the repair
// verb would find none.
func TestGenerateAreasCanRewriteWhatTheyCheck(t *testing.T) {
	for _, tool := range commandsAtStart {
		if group, _ := leroot.GroupOf(tool.Name); group != leroot.GroupGenerate {
			continue
		}
		if !strings.Contains(tool.Meta.ResolveSubs(), writesMarker) {
			t.Errorf("%q is filed under generated artifacts and declares no writing action", tool.Name)
		}
	}
}

// TestReportAreasWriteNothing holds the other title's promise. "Reports (read
// the tree, gate nothing)" must not hide a verb that changes the tree.
func TestReportAreasWriteNothing(t *testing.T) {
	for _, tool := range commandsAtStart {
		if group, _ := leroot.GroupOf(tool.Name); group != leroot.GroupReport {
			continue
		}
		if strings.Contains(tool.Meta.ResolveSubs(), writesMarker) {
			t.Errorf("%q is filed under reports and declares a writing action", tool.Name)
		}
	}
}

// directoryFor answers the path a command name predicts, relative to
// internal/le. A space is a level, and a hyphen inside a level joins words
// naming one thing: `verify lint` predicts verify/lint, and
// `repository tracked-build` predicts repository/trackedbuild.
//
// This is the whole naming rule, and it is one function so the test and the
// reader read the same statement of it.
func directoryFor(name string) string {
	words := strings.Fields(name)
	for index, word := range words {
		words[index] = strings.ReplaceAll(word, "-", "")
	}
	return filepath.Join(words...)
}

// TestDirectoryForReadsBothHalvesOfTheNamingRule pins the rule itself. Until a
// family is split, every registered name is one word, so the nested branch
// would go unexercised by the tree and the structural test would pass without
// ever reading it.
func TestDirectoryForReadsBothHalvesOfTheNamingRule(t *testing.T) {
	for _, row := range []struct{ name, want string }{
		{"verify", "verify"},
		{"verify lint", "verify/lint"},
		{"dash-stdio", "dashstdio"},
		{"repository tracked-build", "repository/trackedbuild"},
		{"yang leaf-mentions", "yang/leafmentions"},
	} {
		if got := directoryFor(row.name); got != row.want {
			t.Errorf("directoryFor(%q) = %q, want %q", row.name, got, row.want)
		}
	}
}

// TestEveryCommandIsFoundAtThePathItsNamePredicts is the structural rule. It
// holds in both directions. `le spec session` lives at
// internal/le/spec/session, and every directory that registers a command is
// reached by some command name.
//
// The rule is mechanical on purpose. A reader who knows the command knows the
// directory. There is no table to consult. Its cost is that a long command
// name makes a long directory name. The answer to that is to rename the
// command, which is the honest coupling.
func TestEveryCommandIsFoundAtThePathItsNamePredicts(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find checkout: %v", err)
	}
	tools := filepath.Join(root, "internal", "le")

	claimed := make(map[string]string, len(commandsAtStart))
	for _, tool := range commandsAtStart {
		directory := directoryFor(tool.Name)
		claimed[directory] = tool.Name
		if _, statErr := os.Stat(filepath.Join(tools, directory, "register.go")); statErr != nil {
			t.Errorf("command %q predicts internal/le/%s, which registers nothing: %v",
				tool.Name, directory, statErr)
		}
	}

	for _, held := range registeringDirectories(t, tools) {
		if _, ok := claimed[held]; !ok {
			t.Errorf("internal/le/%s registers a command that no command name predicts", held)
		}
	}
}

// registeringDirectories answers every directory under internal/le holding a
// register.go, as a path relative to internal/le.
//
// It walks two levels, because a namespace member sits one level below its
// object and two words is the whole grammar. A library package registers
// nothing, so it is not walked into and names nothing.
func registeringDirectories(t *testing.T, tools string) []string {
	t.Helper()

	registers := func(dir string) bool {
		_, err := os.Stat(filepath.Join(dir, "register.go"))
		return err == nil
	}

	found := make([]string, 0, len(commandsAtStart))
	outer, err := os.ReadDir(tools)
	if err != nil {
		t.Fatalf("read internal/le: %v", err)
	}
	for _, object := range outer {
		if !object.IsDir() {
			continue
		}
		if registers(filepath.Join(tools, object.Name())) {
			found = append(found, object.Name())
		}
		inner, readErr := os.ReadDir(filepath.Join(tools, object.Name()))
		if readErr != nil {
			continue
		}
		for _, member := range inner {
			if !member.IsDir() {
				continue
			}
			if registers(filepath.Join(tools, object.Name(), member.Name())) {
				found = append(found, filepath.Join(object.Name(), member.Name()))
			}
		}
	}
	return found
}

// TestNoRegisteredLeCommandExceedsTwoWords holds the bound Dispatch enforces.
// A three-word command would register at a path the resolver never offers the
// matcher, so it would be unreachable from argv while looking registered.
func TestNoRegisteredLeCommandExceedsTwoWords(t *testing.T) {
	for _, tool := range commandsAtStart {
		if words := len(strings.Fields(tool.Name)); words > 2 {
			t.Errorf("command %q is %d words; dispatch offers the matcher two", tool.Name, words)
		}
	}
}

// TestNoMemberShadowsItsNamespaceRootVerb keeps one word from meaning two
// things in one position. `le verify list` is a verb of the verify command; a
// member named list would occupy the same slot, and the longer path wins, so
// the verb would silently stop running.
func TestNoMemberShadowsItsNamespaceRootVerb(t *testing.T) {
	verbs := make(map[string]map[string]bool, len(commandsAtStart))
	for _, tool := range commandsAtStart {
		if strings.ContainsRune(tool.Name, ' ') {
			continue
		}
		held := make(map[string]bool)
		for verb := range strings.SplitSeq(tool.Meta.ResolveSubs(), "|") {
			verb = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(verb), writesMarker))
			if verb != "" {
				held[strings.TrimSpace(verb)] = true
			}
		}
		verbs[tool.Name] = held
	}

	for _, tool := range commandsAtStart {
		object, member, found := strings.Cut(tool.Name, " ")
		if !found {
			continue
		}
		if verbs[object][member] {
			t.Errorf("member %q shadows the verb %q of the %q command, and the longer path wins",
				tool.Name, member, object)
		}
	}
}

// TestEveryCommandRegistersItsOwnAnswerShape is not the same claim as asking
// the registry whether a shape resolves. ShapeForCommand answers by the
// longest registered PREFIX, so a member that declares none inherits its
// namespace root's shape and reports one. The declaration is what this checks,
// at its source, where inheritance cannot hide the omission.
func TestEveryCommandRegistersItsOwnAnswerShape(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find checkout: %v", err)
	}
	tools := filepath.Join(root, "internal", "le")

	for _, held := range registeringDirectories(t, tools) {
		source, readErr := os.ReadFile(filepath.Join(tools, held, "register.go")) //nolint:gosec // a test reads the checkout it runs in
		if readErr != nil {
			t.Errorf("read internal/le/%s/register.go: %v", held, readErr)
			continue
		}
		if !strings.Contains(string(source), "leroot.RegisterShape(") {
			t.Errorf("internal/le/%s declares no answer shape of its own, so it inherits one", held)
		}
	}
}
