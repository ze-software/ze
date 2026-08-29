package yangmigration

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActionsExposeAllNativeWorkflows(t *testing.T) {
	list := Actions()
	if len(list.Actions) != 3 {
		t.Fatalf("actions = %+v", list.Actions)
	}
	for index, workflow := range []Workflow{WorkflowCommandsToPlugins, WorkflowPathRefactor, WorkflowSchemaToYang} {
		if list.Actions[index].Verb != string(workflow) || !list.Actions[index].Writes {
			t.Fatalf("action %d = %+v", index, list.Actions[index])
		}
	}
}

func TestCommandsToPluginsPreviewApplyAndIdempotence(t *testing.T) {
	root := fixtureTree(t, "commands/success")
	source := treePath(root, "internal/component/aaa/yang/ze-aaa-cmd.yang")
	before := mustRead(t, source)

	preview, err := commandsToPlugins(root, false)
	if err != nil || preview.Refused() {
		t.Fatalf("preview failed: report=%+v err=%v", preview, err)
	}
	if len(preview.Moves) != 2 || len(preview.Edits) != 1 {
		t.Fatalf("preview effects = %+v", preview)
	}
	if got := mustRead(t, source); got != before {
		t.Fatal("preview changed source")
	}

	report, err := commandsToPlugins(root, true)
	if err != nil || report.Refused() {
		t.Fatalf("apply failed: report=%+v err=%v", report, err)
	}
	mustNotExist(t, source)
	mustContain(t, treePath(root, "internal/plugins/aaa-cmd/yang/ze-aaa-cmd.yang"), "module ze-aaa-cmd")
	mustContain(t, treePath(root, "internal/component/cmd/show/yang/owner_test.go"), "internal/plugins/aaa-cmd/yang")

	second, err := commandsToPlugins(root, true)
	if err != nil || second.Refused() || second.Changed() {
		t.Fatalf("second apply is not idempotent: report=%+v err=%v", second, err)
	}
}

func TestCommandsToPluginsCoalescesIdenticalDestination(t *testing.T) {
	root := fixtureTree(t, "commands/identical")
	source := treePath(root, "internal/component/aaa/yang/ze-aaa-cmd.yang")
	report, err := commandsToPlugins(root, true)
	if err != nil || report.Refused() || len(report.Moves) != 1 || !report.Moves[0].Identical {
		t.Fatalf("identical collision = %+v, err=%v", report, err)
	}
	mustNotExist(t, source)
	mustExist(t, treePath(root, "internal/plugins/aaa-cmd/yang/ze-aaa-cmd.yang"))
}

func TestCommandsToPluginsRefusalsAreAtomic(t *testing.T) {
	for _, fixture := range []string{"commands/collision", "commands/malformed-yang", "commands/malformed-go"} {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			root := fixtureTree(t, fixture)
			source := treePath(root, "internal/component/aaa/yang/ze-aaa-cmd.yang")
			before := mustRead(t, source)
			report, err := commandsToPlugins(root, true)
			if err != nil {
				t.Fatal(err)
			}
			if !report.Refused() {
				t.Fatalf("unsafe fixture was accepted: %+v", report)
			}
			if got := mustRead(t, source); got != before {
				t.Fatal("refused apply changed source")
			}
		})
	}
}

func TestSchemaToYangPreviewApplyMergeAndIdempotence(t *testing.T) {
	root := fixtureTree(t, "schema/success")
	preview, err := schemaToYang(root, false)
	if err != nil || preview.Refused() {
		t.Fatalf("preview failed: report=%+v err=%v", preview, err)
	}
	if len(preview.Moves) != 5 || len(preview.Removals) != 3 {
		t.Fatalf("preview effects = %+v", preview)
	}
	mustExist(t, treePath(root, "internal/component/demo/schema/ze-demo.yang"))

	report, err := schemaToYang(root, true)
	if err != nil || report.Refused() {
		t.Fatalf("apply failed: report=%+v err=%v", report, err)
	}
	mustNotExist(t, treePath(root, "internal/component/demo/schema"))
	mustContain(t, treePath(root, "internal/component/demo/yang/model.go"), "package yang")
	mustContain(t, treePath(root, "internal/consumer/use.go"), `demoyang "github.com/ze-software/ze/internal/component/demo/yang"`)
	mustContain(t, treePath(root, "internal/consumer/use.go"), "demoyang.Value")
	mustContain(t, treePath(root, "docs/tree.md"), "internal/component/demo/yang/ze-demo.yang")
	mustNotExist(t, treePath(root, "internal/component/merged/schema"))
	mustExist(t, treePath(root, "internal/component/merged/yang/ze-merged.yang"))
	mustNotExist(t, treePath(root, "internal/component/merged/yang/embed.go"))

	second, err := schemaToYang(root, true)
	if err != nil || second.Refused() || second.Changed() {
		t.Fatalf("second apply is not idempotent: report=%+v err=%v", second, err)
	}
}

func TestSchemaToYangCoalescesPostTransformDestination(t *testing.T) {
	root := fixtureTree(t, "schema/post-transform-identical")
	report, err := schemaToYang(root, true)
	if err != nil || report.Refused() || len(report.Moves) != 2 {
		t.Fatalf("post-transform collision = %+v, err=%v", report, err)
	}
	for _, move := range report.Moves {
		if !move.Identical {
			t.Fatalf("move was not coalesced: %+v", move)
		}
	}
	mustNotExist(t, treePath(root, "internal/component/demo/schema"))
	mustContain(t, treePath(root, "internal/component/demo/yang/model.go"), "package yang")
}

func TestSchemaToYangRefusalsAreAtomic(t *testing.T) {
	for _, fixture := range []string{"schema/collision", "schema/destination-file", "schema/non-regular", "schema/malformed-go", "schema/malformed-yang"} {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			root := fixtureTree(t, fixture)
			report, err := schemaToYang(root, true)
			if err != nil {
				t.Fatal(err)
			}
			if !report.Refused() {
				t.Fatalf("unsafe fixture was accepted: %+v", report)
			}
			mustExist(t, treePath(root, "internal/component/demo/schema/ze-demo.yang"))
		})
	}
}

func TestPathRefactorRemovePreviewApplyAndIdempotence(t *testing.T) {
	root := fixtureTree(t, "path/success")
	op := pathOperation{Kind: PathRemove, Target: "connection", Under: "bgp/peer", ListNodes: defaultListNodes()}
	preview, err := refactorPaths(root, op, false)
	if err != nil || preview.Refused() || len(preview.Edits) != 4 || len(preview.Manual) != 2 {
		t.Fatalf("preview = %+v, err=%v", preview, err)
	}
	mustContain(t, treePath(root, "test/path.ci"), "connection {")

	report, err := refactorPaths(root, op, true)
	if err != nil || report.Refused() {
		t.Fatalf("apply = %+v, err=%v", report, err)
	}
	mustNotContain(t, treePath(root, "test/path.ci"), "connection {")
	mustContain(t, treePath(root, "test/path.ci"), "remote {")
	mustContain(t, treePath(root, "test/path.ci"), "--context bgp/peer/p1/remote")
	mustContain(t, treePath(root, "test/path.et"), "set bgp peer p1 remote ip")
	mustContain(t, treePath(root, "internal/path.go"), `peer.GetContainer("remote")`)
	mustContain(t, treePath(root, "internal/path.go"), `terminal := peer.GetContainer("connection")`)
	mustContain(t, treePath(root, "internal/model.yang"), `"remote/ip"`)

	second, err := refactorPaths(root, op, true)
	if err != nil || second.Refused() || second.Changed() {
		t.Fatalf("second apply is not idempotent: report=%+v err=%v", second, err)
	}
}

func TestPathRefactorRenameAndMovePreserveListKeys(t *testing.T) {
	rename := pathOperation{Kind: PathRename, Target: "session", Replacement: "protocol", Under: "bgp/peer", ListNodes: defaultListNodes()}
	if got, changed := transformSlashPath("bgp/peer/p1/session/asn", rename); !changed || got != "bgp/peer/p1/protocol/asn" {
		t.Fatalf("rename = %q, %v", got, changed)
	}
	move := pathOperation{Kind: PathMove, Source: "bgp/peer/session/capability", Destination: "bgp/peer/capability", ListNodes: defaultListNodes()}
	if got, changed := transformSlashPath("bgp/peer/p1/session/capability/graceful-restart", move); !changed || got != "bgp/peer/p1/capability/graceful-restart" {
		t.Fatalf("move = %q, %v", got, changed)
	}
}

func TestPathRefactorRenameWorkflow(t *testing.T) {
	root := fixtureTree(t, "path/rename-success")
	operation := pathOperation{Kind: PathRename, Target: "session", Replacement: "protocol", Under: "bgp/peer", ListNodes: defaultListNodes()}
	report, err := refactorPaths(root, operation, true)
	if err != nil || report.Refused() || len(report.Edits) != 2 {
		t.Fatalf("rename workflow = %+v, err=%v", report, err)
	}
	mustContain(t, treePath(root, "internal/path.go"), `GetContainer("protocol")`)
	mustContain(t, treePath(root, "test/path.ci"), "protocol {")
}

func TestPathRefactorMoveWorkflow(t *testing.T) {
	root := fixtureTree(t, "path/move-success")
	operation := pathOperation{Kind: PathMove, Source: "bgp/peer/session/capability", Destination: "bgp/peer/capability", ListNodes: defaultListNodes()}
	report, err := refactorPaths(root, operation, true)
	if err != nil || report.Refused() || len(report.Edits) != 1 {
		t.Fatalf("move workflow = %+v, err=%v", report, err)
	}
	mustContain(t, treePath(root, "internal/path.go"), `"bgp/peer/p1/capability/graceful-restart"`)
}

func TestPathOperationRefusesInvalidGrammar(t *testing.T) {
	operations := []pathOperation{
		{Kind: PathRemove, Target: "connection", ListNodes: defaultListNodes()},
		{Kind: PathRename, Target: "session", Replacement: "session", Under: "bgp/peer", ListNodes: defaultListNodes()},
		{Kind: PathMove, Source: "bgp/peer", Destination: "bgp/peer", ListNodes: defaultListNodes()},
		{Kind: PathMove, Source: "bgp//peer", Destination: "bgp/group", ListNodes: defaultListNodes()},
	}
	for _, operation := range operations {
		if err := operation.Validate(); err == nil {
			t.Fatalf("invalid operation was accepted: %+v", operation)
		}
	}
}

func TestPathRefactorRefusesMalformedSyntaxWithoutWrites(t *testing.T) {
	for _, fixture := range []string{"path/malformed-yang", "path/malformed-go", "path/unclosed-quote", "path/unclosed-brace"} {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			root := fixtureTree(t, fixture)
			op := pathOperation{Kind: PathRemove, Target: "connection", Under: "bgp/peer", ListNodes: defaultListNodes()}
			report, err := refactorPaths(root, op, true)
			if err != nil {
				t.Fatal(err)
			}
			if !report.Refused() {
				t.Fatalf("malformed fixture was accepted: %+v", report)
			}
		})
	}
}

func fixtureTree(t *testing.T, name string) string {
	t.Helper()
	source := filepath.Join("testdata", filepath.FromSlash(name))
	destination := t.TempDir()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relativePath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relativePath)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return destination
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func mustContain(t *testing.T, path, text string) {
	t.Helper()
	if content := mustRead(t, path); !strings.Contains(content, text) {
		t.Fatalf("%s does not contain %q:\n%s", path, text, content)
	}
}

func mustNotContain(t *testing.T, path, text string) {
	t.Helper()
	if content := mustRead(t, path); strings.Contains(content, text) {
		t.Fatalf("%s still contains %q:\n%s", path, text, content)
	}
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func mustNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("%s still exists (err=%v)", path, err)
	}
}

// treePath answers one checkout-relative slash path inside root.
func treePath(root, relative string) string {
	return filepath.Join(root, filepath.FromSlash(relative))
}
