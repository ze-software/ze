// VALIDATES: spec-le-is-a-ze-binary native module move and module rename actions.
// PREVENTS: partial rewrites, boundary mistakes, silent binary/generated-file edits,
// collision damage, and preview modes that mutate the checkout.
package module

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	testOldModule = "oldforge.example/owner/project"
	testNewModule = "newforge.example/other/project"
)

func TestModuleActionsExposeKeywordFirstNativeWorkflows(t *testing.T) {
	list := Actions()
	if list.Area != "module" || len(list.Actions) != 2 {
		t.Fatalf("action list = %+v", list)
	}
	if list.Actions[0].Verb != "move" || list.Actions[1].Verb != "rename" {
		t.Fatalf("verbs = %+v", list.Actions)
	}
	if _, code := Answer([]string{"move", "edge"}); code != 2 {
		t.Fatalf("bare source bypassed keyword grammar with code %d", code)
	}
	if _, code := Answer([]string{"rename", testNewModule}); code != 2 {
		t.Fatalf("bare module bypassed keyword grammar with code %d", code)
	}
}

func TestMovePreviewIsBoundarySafeDeterministicAndReadOnly(t *testing.T) {
	root := moveFixture(t)
	writeFixture(t, root, "internal/component/edge/edge.go", "package edge\nimport _ \""+testOldModule+"/internal/component/edge/sub\"\n")
	writeFixture(t, root, "internal/component/edge/sub/sub.go", "package sub\n")
	writeFixture(t, root, "internal/plugins/client/client.go", "package client\nimport _ \""+testOldModule+"/internal/component/edge\"\n")
	writeFixture(t, root, "internal/component/edge2/edge.go", "package edge2\nimport _ \""+testOldModule+"/internal/component/edge2\"\n")
	writeFixture(t, root, "docs/edge.md", "see internal/component/edge/sub\n")

	before := mustRead(t, treePath(root, "internal/plugins/client/client.go"))
	report, err := Move(root, MoveOptions{Source: "edge", Destination: "plugins"})
	if err != nil {
		t.Fatalf("preview move: %v", err)
	}
	if report.Apply || report.Source != "internal/component/edge" || report.Destination != "internal/plugins/edge" {
		t.Fatalf("unexpected preview identity: %+v", report)
	}
	wantEdits := []CountedPath{
		{Path: "internal/component/edge/edge.go", Count: 1},
		{Path: "internal/plugins/client/client.go", Count: 1},
	}
	if !reflect.DeepEqual(report.ImportEdits, wantEdits) {
		t.Fatalf("import edits = %#v, want %#v", report.ImportEdits, wantEdits)
	}
	if !reflect.DeepEqual(report.Residual, []CountedPath{{Path: "docs/edge.md", Count: 1}}) {
		t.Fatalf("residual = %#v", report.Residual)
	}
	if got := mustRead(t, treePath(root, "internal/plugins/client/client.go")); got != before {
		t.Fatalf("preview changed importer:\n%s", got)
	}
	if _, err := os.Stat(treePath(root, "internal/component/edge")); err != nil {
		t.Fatalf("preview moved source: %v", err)
	}
}

func TestMoveApplyRelocatesTreeRewritesImportsAndPluginDiscovery(t *testing.T) {
	root := moveFixture(t)
	writeFixture(t, root, "internal/component/edge/edge.go", "package edge\nimport _ \""+testOldModule+"/internal/component/edge/sub\"\n")
	writeFixture(t, root, "internal/component/edge/sub/sub.go", "package sub\n")
	writeFixture(t, root, "internal/plugins/client/client.go", "package client\nimport _ \""+testOldModule+"/internal/component/edge\"\n")

	oldGenerator, oldGoimports := executePluginGenerator, executeMoveGoimports
	executePluginGenerator = func(string) int { return 0 }
	executeMoveGoimports = func(string, movePlan) string { return "test stub" }
	t.Cleanup(func() {
		executePluginGenerator = oldGenerator
		executeMoveGoimports = oldGoimports
	})

	report, err := Move(root, MoveOptions{Source: "edge", Destination: "plugins", Apply: true})
	if err != nil {
		t.Fatalf("apply move: %v; report=%+v", err, report)
	}
	if report.Code != 0 || report.Goimports != "test stub" || !report.Registrations.Preserved {
		t.Fatalf("unexpected apply report: %+v", report)
	}
	if _, err := os.Stat(treePath(root, "internal/component/edge")); !os.IsNotExist(err) {
		t.Fatalf("old source still exists: %v", err)
	}
	moved := mustRead(t, treePath(root, "internal/plugins/edge/edge.go"))
	if !strings.Contains(moved, testOldModule+"/internal/plugins/edge/sub") {
		t.Fatalf("moved import was not rewritten: %s", moved)
	}
	client := mustRead(t, treePath(root, "internal/plugins/client/client.go"))
	if !strings.Contains(client, testOldModule+"/internal/plugins/edge") {
		t.Fatalf("external import was not rewritten: %s", client)
	}
	generator := mustRead(t, filepath.Join(root, pluginGenerator))
	if strings.Contains(generator, "internal/component/edge") || !strings.Contains(generator, "\"internal/plugins\"") {
		t.Fatalf("plugin discovery edit is wrong:\n%s", generator)
	}
}

func TestMoveApplyRefusesRPCHazardAndCollisionBeforeMutation(t *testing.T) {
	t.Run("rpc hazard", func(t *testing.T) {
		root := moveFixture(t)
		writeFixture(t, root, "internal/component/edge/rpc.go", "package edge\nfunc register(){ pluginserver.RegisterRPCs() }\n")
		report, err := Move(root, MoveOptions{Source: "edge", Destination: "plugins", Apply: true})
		if err == nil || report.Code != 3 || !report.RPCHazard {
			t.Fatalf("RPC refusal = report %+v, err %v", report, err)
		}
		if _, statErr := os.Stat(treePath(root, "internal/component/edge/rpc.go")); statErr != nil {
			t.Fatalf("RPC refusal mutated source: %v", statErr)
		}
	})

	t.Run("merge collision", func(t *testing.T) {
		root := moveFixture(t)
		writeFixture(t, root, "internal/component/edge/same.go", "package edge\n")
		writeFixture(t, root, "internal/plugins/edge/same.go", "package edge\nconst destination = true\n")
		report, err := Move(root, MoveOptions{Source: "internal/component/edge", Destination: "internal/plugins/edge", Apply: true})
		if err == nil || report.Code != 2 || !reflect.DeepEqual(report.Conflicts, []string{"same.go"}) {
			t.Fatalf("collision refusal = report %+v, err %v", report, err)
		}
		if got := mustRead(t, treePath(root, "internal/plugins/edge/same.go")); !strings.Contains(got, "destination") {
			t.Fatalf("collision overwrote destination: %s", got)
		}
	})
}

func TestRenamePlanClassifiesEveryTextOccurrenceAndIgnoresBinary(t *testing.T) {
	root := t.TempDir()
	files := []string{
		".claude/settings.local.json", "api/ze.pb.go", "api/ze.proto", "blob.bin",
		"go.mod", "rfc/audit/rfc9999.json", "vendor/example/x.go",
	}
	writeFixture(t, root, "go.mod", "module "+testOldModule+"\n")
	writeFixture(t, root, "api/ze.proto", "option go_package = \""+testOldModule+"/api\";\n")
	writeFixture(t, root, "api/ze.pb.go", "var raw = \""+testOldModule+"/api\"\n")
	writeFixture(t, root, "vendor/example/x.go", "import \""+testOldModule+"/x\"\n")
	writeFixture(t, root, "rfc/audit/rfc9999.json", "{\"note\":\""+testOldModule+"\"}\n")
	writeFixture(t, root, ".claude/settings.local.json", "/Users/t/Code/"+testOldModule+"/\n")
	writeBytesFixture(t, root, "blob.bin", append([]byte{0}, []byte(testOldModule)...))

	plan, err := buildRenamePlan(root, files, testOldModule, testNewModule)
	if err != nil {
		t.Fatal(err)
	}
	if got := countedEdits(plan.edits); !reflect.DeepEqual(got, []CountedPath{{Path: "api/ze.proto", Count: 1}, {Path: "go.mod", Count: 1}}) {
		t.Fatalf("edits = %#v", got)
	}
	if !reflect.DeepEqual(plan.regenerate, []CountedPath{{Path: "api/ze.pb.go", Count: 1}}) {
		t.Fatalf("regenerate = %#v", plan.regenerate)
	}
	wantSkipped := []SkippedPath{
		{Path: ".claude/settings.local.json", Count: 1, Reason: "not-a-module-path"},
		{Path: "rfc/audit/rfc9999.json", Count: 1, Reason: "rfc/audit"},
		{Path: "vendor/example/x.go", Count: 1, Reason: "vendor"},
	}
	if !reflect.DeepEqual(plan.skipped, wantSkipped) {
		t.Fatalf("skipped = %#v, want %#v", plan.skipped, wantSkipped)
	}
}

func TestRenameTextHandlesLiteralAndSegmentedFormsWithoutGuessingDepth(t *testing.T) {
	text := "import \"" + testOldModule + "/x\"\nJoin(\n\t\"oldforge.example\",\n\t\"owner\",\n\t\"project\",\n)\n"
	updated, count := rewriteModuleText(text, testOldModule, testNewModule)
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	if !strings.Contains(updated, "\"newforge.example\",\n\t\"other\",\n\t\"project\"") {
		t.Fatalf("segmented spelling was not preserved and renamed:\n%s", updated)
	}
	depthChange := "Join(\"oldforge.example\", \"owner\", \"project\")"
	if got, count := rewriteModuleText(depthChange, testOldModule, "newforge.example/project"); got != depthChange || count != 0 {
		t.Fatalf("depth-changing segmented rewrite guessed: %q, %d", got, count)
	}
}

func TestRenamePreviewAndApplyUseTrackedFilesAndMoveMirroredPath(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "go.mod", "module "+testOldModule+"\n\ngo 1.26\n")
	writeFixture(t, root, "internal/a/a.go", "package a\nimport _ \""+testOldModule+"/internal/x\"\n")
	writeFixture(t, root, "gokrazy/ze/builddir/"+testOldModule+"/go.mod", "require "+testOldModule+" v0.0.0\n")
	writeFixture(t, root, ".gitignore", "ignored.go\n")
	writeFixture(t, root, "ignored.go", "import \""+testOldModule+"/ignored\"\n")
	gitInitAndAdd(t, root, []string{"go.mod", "internal/a/a.go", "gokrazy/ze/builddir/" + testOldModule + "/go.mod"})

	before := mustRead(t, filepath.Join(root, "go.mod"))
	preview, err := Rename(root, RenameOptions{New: testNewModule})
	if err != nil {
		t.Fatalf("preview rename: %v", err)
	}
	if preview.Apply || preview.Occurrences != 3 || len(preview.Moves) != 1 {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if got := mustRead(t, filepath.Join(root, "go.mod")); got != before {
		t.Fatalf("preview rewrote go.mod: %s", got)
	}

	report, err := Rename(root, RenameOptions{New: testNewModule, Apply: true, NoGoimports: true, NoReseal: true})
	if err != nil {
		t.Fatalf("apply rename: %v; report=%+v", err, report)
	}
	if report.Code != 0 || report.ChangedFiles != 3 || report.MovedDirs != 1 || report.Goimports != "skipped" {
		t.Fatalf("unexpected apply report: %+v", report)
	}
	if got := mustRead(t, filepath.Join(root, "go.mod")); !strings.HasPrefix(got, "module "+testNewModule) {
		t.Fatalf("go.mod not renamed: %s", got)
	}
	newMirror := filepath.Join(root, filepath.FromSlash("gokrazy/ze/builddir/"+testNewModule+"/go.mod"))
	if _, err := os.Stat(newMirror); err != nil {
		t.Fatalf("mirrored path not moved: %v", err)
	}
	if got := mustRead(t, filepath.Join(root, "ignored.go")); !strings.Contains(got, testOldModule) {
		t.Fatalf("untracked file was rewritten: %s", got)
	}
}

func TestRenameCollisionRefusesBeforeRewritingFiles(t *testing.T) {
	root := t.TempDir()
	oldMirror := "gokrazy/ze/builddir/" + testOldModule + "/go.mod"
	newMirror := "gokrazy/ze/builddir/" + testNewModule + "/occupied"
	writeFixture(t, root, "go.mod", "module "+testOldModule+"\n")
	writeFixture(t, root, oldMirror, "require "+testOldModule+" v0.0.0\n")
	writeFixture(t, root, newMirror, "occupied\n")
	gitInitAndAdd(t, root, []string{"go.mod", oldMirror, newMirror})

	before := mustRead(t, filepath.Join(root, "go.mod"))
	report, err := Rename(root, RenameOptions{New: testNewModule, Apply: true, NoGoimports: true, NoReseal: true})
	if err == nil || report.Code != 2 {
		t.Fatalf("collision was not refused: report=%+v err=%v", report, err)
	}
	if got := mustRead(t, filepath.Join(root, "go.mod")); got != before {
		t.Fatalf("collision refusal rewrote go.mod: %s", got)
	}
}

func TestRenameOnlyProofAcceptsFormattingAndRejectsAssertionChanges(t *testing.T) {
	root := t.TempDir()
	relative := "x_test.go"
	writeFixture(t, root, relative, "import (\n\t\""+testOldModule+"/a\"\n\n\t\""+testOldModule+"/b\"\n)\nrequire.Equal(t, 3, got)\n")
	gitInitAndAdd(t, root, []string{relative})
	gitCommitFixture(t, root)

	writeFixture(t, root, relative, "import (\n    \""+testNewModule+"/a\"\n    \""+testNewModule+"/b\"\n)\nrequire.Equal(t, 3, got)\n")
	if !renameOnlySinceHead(root, relative, testOldModule, testNewModule) {
		t.Fatal("pure rename plus fingerprint-equivalent formatting was refused")
	}
	writeFixture(t, root, relative, "import \""+testNewModule+"/a\"\nrequire.Equal(t, 4, got)\n")
	if renameOnlySinceHead(root, relative, testOldModule, testNewModule) {
		t.Fatal("assertion change was accepted as a pure rename")
	}
}

func TestRenameReportTextIsExactAndHonorsTheRowLimit(t *testing.T) {
	report := RenameReport{
		Old: testOldModule, New: testNewModule, Limit: 1, Occurrences: 2,
		Edits: []CountedPath{{Path: "a.go", Count: 1}, {Path: "b.go", Count: 1}},
	}
	want := "rename " + testOldModule + "\n" +
		"    -> " + testNewModule + "\n" +
		"2 occurrence(s) in 2 file(s), 0 directory move(s)\n" +
		"rewrite (2):\n" +
		"  1  a.go\n" +
		"  ... and 1 more\n" +
		"dry run -- nothing changed. Re-run with apply.\n"
	if got := report.Text(); got != want {
		t.Fatalf("report text differs\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestAtomicWriterPreservesPermissionsAndLeavesNoStagingFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "tool")
	if err := os.WriteFile(path, []byte("old"), 0o751); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(path, []byte("new"), 0o751); err != nil {
		t.Fatal(err)
	}
	if got := mustRead(t, path); got != "new" {
		t.Fatalf("contents = %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o751 {
		t.Fatalf("permissions = %o", info.Mode().Perm())
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "tool" {
		t.Fatalf("staging file leaked: %v", entries)
	}
}

func moveFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, "go.mod", "module "+testOldModule+"\n")
	writeFixture(t, root, pluginGenerator, "package main\nvar pluginDirs = []string{\n\t\"internal/component/edge\",\n\t\"internal/plugins\",\n}\nvar nestedPluginDomains = []string{\n}\n")
	writeFixture(t, root, generatedAll, "package all\n")
	return root
}

func countedEdits(edits []textEdit) []CountedPath {
	out := make([]CountedPath, 0, len(edits))
	for _, edit := range edits {
		out = append(out, CountedPath{Path: edit.Relative, Count: edit.Count})
	}
	return out
}

func writeFixture(t *testing.T, root, relative, contents string) {
	t.Helper()
	writeBytesFixture(t, root, relative, []byte(contents))
}

func writeBytesFixture(t *testing.T, root, relative string, contents []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

func gitInitAndAdd(t *testing.T, root string, files []string) {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", "init", "-q")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	args := append([]string{"add", "--"}, files...)
	command = exec.CommandContext(t.Context(), "git", args...)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, output)
	}
}

func gitCommitFixture(t *testing.T, root string) {
	t.Helper()
	command := exec.CommandContext(t.Context(),
		"git", "-c", "user.email=test@example.invalid", "-c", "user.name=modulemigration-test",
		"commit", "-q", "-m", "fixture",
	)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
}

// treePath answers one checkout-relative slash path inside root.
func treePath(root, relative string) string {
	return filepath.Join(root, filepath.FromSlash(relative))
}
