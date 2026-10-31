// VALIDATES: writeSpecDoorsRead refuses a spec write until the session, or the
// subagent, read ai/doors/system.md and every door covering a path the spec
// names, each in full, and lets the same write through once it has.
// PREVENTS: a spec's design rewritten from habit by a context that never read
// the contracts its subsystems hold (third-party plugins in any language).
package hookruntime

import (
	"path/filepath"
	"slices"
	"testing"
)

// specFixturePath is a spec in a real bucket, so specpath.IsSpec answers yes.
const specFixturePath = "plan/post-release/engineering/spec-widget.md"

// specEditPayload proposes one tool call writing text into file.
func specEditPayload(session, tool, file, text string) map[string]any {
	return map[string]any{
		"session_id": session,
		"tool_name":  tool,
		"tool_input": map[string]any{"file_path": file, "old_string": "a", "new_string": text, "content": text},
	}
}

// TestSpecDoorGateRequiresTheDoorsOfNamedPaths is the table: which doors a
// context must have read before it writes a spec, from what the spec on disk
// and the proposed text name.
func TestSpecDoorGateRequiresTheDoorsOfNamedPaths(t *testing.T) {
	cases := []struct {
		name   string
		file   string
		tool   string
		disk   string
		text   string
		read   []string
		unread []string
	}{
		{"new-spec-no-path-needs-system", specFixturePath, "Write", "", "# Spec: widget\n", nil, []string{doorSystem}},
		{"new-spec-no-path-system-read", specFixturePath, "Write", "", "# Spec: widget\n", []string{doorSystem}, nil},
		{"bgp-path-needs-bgp-door", specFixturePath, "Write", "", "| `internal/component/bgp/reactor/peer.go` | change |\n", []string{doorSystem}, []string{doorBGPEngine}},
		{"bgp-path-nothing-read", specFixturePath, "Edit", "", "Touches internal/component/bgp/reactor/peer.go.", nil, []string{doorSystem, doorBGPEngine}},
		{"bgp-path-doors-read", specFixturePath, "Write", "", "- `internal/component/bgp/reactor/peer.go`\n", []string{doorSystem, doorBGPEngine}, nil},
		{"plugin-path-longest-prefix", specFixturePath, "MultiEdit", "", "internal/component/bgp/plugins/rib/rib.go", []string{doorSystem, doorBGPEngine}, []string{doorBGPPlugins}},
		{"path-on-disk-still-owed", specFixturePath, "Edit", "## Files to Modify\n- `pkg/ze/engine.go`\n", "status only", []string{doorSystem}, []string{doorPluginInfra, doorHubEngine}},
		{"directory-path-with-slash", specFixturePath, "Write", "", "Everything under ./internal/component/bgp/.", []string{doorSystem}, []string{doorBGPEngine}},
		{"bare-word-is-no-path", specFixturePath, "Write", "", "pkg and internal and bgp, and/or cmd", []string{doorSystem}, nil},
		{"draft-plan-not-a-spec", "plan/drafts/ze-plan-widget.md", "Write", "", "internal/component/bgp/reactor/peer.go", nil, nil},
		{"doc-not-a-spec", "docs/architecture/widget.md", "Write", "", "internal/component/bgp/reactor/peer.go", nil, nil},
		{"read-tool-never-refused", specFixturePath, "Read", "", "internal/component/bgp/reactor/peer.go", nil, nil},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeDoors(t, root)
			if test.disk != "" {
				writeFixtureFile(t, root, test.file, test.disk)
			}
			for _, door := range test.read {
				readDoor(t, root, "sess-a", door, nil)
			}
			requireDoorGate(t, root, specEditPayload("sess-a", test.tool, test.file, test.text), test.unread)
		})
	}
}

// TestSpecDoorGateClearsAfterTheReads walks one edit from refused to admitted:
// the same Write of a spec naming a BGP path is refused, then passes once the
// context has read system.md and the BGP door, whole.
func TestSpecDoorGateClearsAfterTheReads(t *testing.T) {
	root := t.TempDir()
	writeDoors(t, root)
	payload := specEditPayload("sess-a", "Write", specFixturePath, "## Files to Modify\n- `internal/component/bgp/reactor/peer.go`\n")
	requireDoorGate(t, root, payload, []string{doorSystem, doorBGPEngine})
	readDoor(t, root, "sess-a", doorSystem, nil)
	requireDoorGate(t, root, payload, []string{doorBGPEngine})
	readDoor(t, root, "sess-a", doorBGPEngine, nil)
	requireDoorGate(t, root, payload, nil)
}

// TestSpecDoorGateMatchesAbsolutePaths holds the gate to repository-relative
// paths when the tool names the spec absolutely, and when the spec names a
// file by its absolute path inside the checkout.
func TestSpecDoorGateMatchesAbsolutePaths(t *testing.T) {
	root := t.TempDir()
	writeDoors(t, root)
	readDoor(t, root, "sess-a", doorSystem, nil)
	spec := filepath.Join(root, filepath.FromSlash(specFixturePath))
	named := filepath.ToSlash(filepath.Join(root, "internal", "component", "bgp", "plugins", "rib", "rib.go"))
	requireDoorGate(t, root, specEditPayload("sess-a", "Edit", spec, "See "+named+"."), []string{doorBGPPlugins})
}

// TestSpecRepoPathsTrimsAndDeduplicates pins the path extraction: punctuation
// and `./` trimmed, a URL's host path and slashless words dropped, each path
// once.
func TestSpecRepoPathsTrimsAndDeduplicates(t *testing.T) {
	text := "Edit `./internal/a/b.go`, then internal/a/b.go. See https://example.com/x/y and the word test."
	got := specRepoPaths("/repo", text+" /repo/pkg/ze/z.go")
	want := []string{"internal/a/b.go", "pkg/ze/z.go"}
	if !slices.Equal(got, want) {
		t.Fatalf("specRepoPaths = %q, want %q", got, want)
	}
}
