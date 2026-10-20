// VALIDATES: writeDesignDoorsRead refuses a code write until the session, or the
// subagent, read ai/doors/system.md and every door covering the file, each in
// full, through Read or a Bash print.
// PREVENTS: code written from habit by a context that never read the design it
// must respect, or read only part of it.
package hookruntime

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// doorGateRefusal is the text only writeDesignDoorsRead writes, so a test can
// tell its refusal from any other check's.
const doorGateRefusal = "has not read the design doors"

// Door pages of the fixture checkout. The bases copy the shape of the real
// headers: bgp-plugins sits inside bgp-engine's base, and plugin-infra and
// hub-engine both claim pkg/ze.
const (
	doorSystem      = "ai/doors/system.md"
	doorBGPEngine   = "ai/doors/bgp-engine.md"
	doorBGPPlugins  = "ai/doors/bgp-plugins.md"
	doorPluginInfra = "ai/doors/plugin-infra.md"
	doorHubEngine   = "ai/doors/hub-engine.md"
	doorReadme      = "ai/doors/README.md"
)

// doorPageLines is how many lines each fixture door holds.
const doorPageLines = 6

// writeDoors lays out a fixture ai/doors directory under root. README.md
// names the header inside its prose, as the real index does, and carries none.
func writeDoors(t *testing.T, root string) {
	t.Helper()
	pages := map[string]string{
		doorSystem:      "pkg cmd/ze internal",
		doorBGPEngine:   "internal/component/bgp internal/core/bgp",
		doorBGPPlugins:  "internal/component/bgp/plugins",
		doorPluginInfra: "internal/component/plugin pkg/plugin pkg/ze",
		doorHubEngine:   "cmd/ze internal/component/engine pkg/ze/",
	}
	for page, bases := range pages {
		body := "# Door\n\n<!-- door-base: " + bases + " -->\n" + strings.Repeat("A line of the door.\n", doorPageLines-3)
		writeFixtureFile(t, root, page, body)
	}
	writeFixtureFile(t, root, doorReadme, "# Doors\n\nEach door carries a `<!-- door-base: ... -->` header.\n")
}

func writeFixtureFile(t *testing.T, root, path, body string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// readDoor runs the Read hook over one door, as the harness does after a Read
// tool call, with window adding the call's offset and limit.
func readDoor(t *testing.T, root, session, agent, door string, window map[string]any) {
	t.Helper()
	input := map[string]any{"file_path": filepath.Join(root, filepath.FromSlash(door))}
	maps.Copy(input, window)
	payload := map[string]any{"session_id": session, "tool_name": "Read", "tool_input": input}
	if agent != "" {
		payload["agent_id"] = agent
	}
	if code, _, message := runHook(t, root, "mark-source-read", payload); code != 0 {
		t.Fatalf("mark-source-read: code=%d message=%q", code, message)
	}
}

// readEveryDoor lays out the fixture doors under root and reads each one whole
// for session, so a test judging another check runs past the door gate.
func readEveryDoor(t *testing.T, root, session string) {
	t.Helper()
	writeDoors(t, root)
	for _, door := range []string{doorSystem, doorBGPEngine, doorBGPPlugins, doorPluginInfra, doorHubEngine} {
		readDoor(t, root, session, "", door, nil)
	}
}

// runBash runs the Bash pre-hook over one command.
func runBash(t *testing.T, root, session, agent, command string) {
	t.Helper()
	payload := map[string]any{"session_id": session, "tool_name": "Bash", "tool_input": map[string]any{"command": command}}
	if agent != "" {
		payload["agent_id"] = agent
	}
	runHook(t, root, "pretool-bash", payload)
}

// codeEditPayload proposes one tool call writing file.
func codeEditPayload(session, agent, tool, file string) map[string]any {
	payload := map[string]any{
		"session_id": session,
		"tool_name":  tool,
		"tool_input": map[string]any{"file_path": file, "old_string": "a", "new_string": "b", "content": "b\n"},
	}
	if agent != "" {
		payload["agent_id"] = agent
	}
	return payload
}

// requireDoorGate asserts that the pre-write hook refuses the payload exactly
// when unread is not empty, and that the refusal lists every unread door and
// no other.
func requireDoorGate(t *testing.T, root string, payload map[string]any, unread []string) {
	t.Helper()
	code, _, message := runHook(t, root, "pretool-writeedit", payload)
	refused := strings.Contains(message, doorGateRefusal)
	if refused != (len(unread) != 0) {
		t.Fatalf("door gate refused = %v (code %d), want unread %v: %q", refused, code, unread, message)
	}
	if !refused {
		return
	}
	if code != 2 {
		t.Fatalf("a refusal must exit 2, got %d: %q", code, message)
	}
	for _, door := range []string{doorSystem, doorBGPEngine, doorBGPPlugins, doorPluginInfra, doorHubEngine, doorReadme} {
		listed := strings.Contains(message, "  - "+door+"\n")
		if listed != slices.Contains(unread, door) {
			t.Fatalf("refusal lists %s = %v, want unread %v: %q", door, listed, unread, message)
		}
	}
	if !strings.Contains(message, "Read tool") {
		t.Fatalf("the refusal does not name the fix: %q", message)
	}
}

// TestDoorGateRequiresSystemAndTheCoveringDoor is the table: which doors a
// context must have read before it writes each kind of file.
func TestDoorGateRequiresSystemAndTheCoveringDoor(t *testing.T) {
	cases := []struct {
		name   string
		file   string
		tool   string
		read   []string
		unread []string
	}{
		{"nothing-read", "internal/component/bgp/plugins/rib/rib.go", "Edit", nil, []string{doorSystem, doorBGPPlugins}},
		{"system-read-door-unread", "internal/component/bgp/plugins/rib/rib.go", "Edit", []string{doorSystem}, []string{doorBGPPlugins}},
		{"door-read-system-unread", "internal/component/bgp/plugins/rib/rib.go", "Edit", []string{doorBGPPlugins}, []string{doorSystem}},
		{"longest-prefix-wins", "internal/component/bgp/plugins/rib/rib.go", "Edit", []string{doorSystem, doorBGPEngine}, []string{doorBGPPlugins}},
		{"longest-prefix-read", "internal/component/bgp/plugins/rib/rib.go", "Write", []string{doorSystem, doorBGPPlugins}, nil},
		{"engine-file-takes-engine-door", "internal/component/bgp/reactor/peer.go", "MultiEdit", []string{doorSystem}, []string{doorBGPEngine}},
		{"engine-file-read", "internal/component/bgp/reactor/peer.go", "Edit", []string{doorSystem, doorBGPEngine}, nil},
		{"tie-requires-both", "pkg/ze/engine.go", "Edit", []string{doorSystem, doorPluginInfra}, []string{doorHubEngine}},
		{"tie-trailing-slash-base", "pkg/ze/engine.go", "Edit", []string{doorSystem}, []string{doorPluginInfra, doorHubEngine}},
		{"tie-both-read", "pkg/ze/engine.go", "Edit", []string{doorSystem, doorPluginInfra, doorHubEngine}, nil},
		{"system-tie-on-its-own-base", "cmd/ze/main.go", "Edit", []string{doorSystem}, []string{doorHubEngine}},
		{"uncovered-needs-system-only", "internal/component/foo/foo.go", "Edit", nil, []string{doorSystem}},
		{"uncovered-system-read", "internal/component/foo/foo.go", "Edit", []string{doorSystem}, nil},
		{"outside-every-base", "tools/x.go", "Edit", nil, []string{doorSystem}},
		{"whole-component-prefix", "internal/component/bgpx/foo.go", "Edit", []string{doorSystem}, nil},
		{"test-file-is-code", "internal/component/bgp/plugins/rib/rib_test.go", "Edit", nil, []string{doorSystem, doorBGPPlugins}},
		{"yang-is-code", "internal/component/bgp/plugins/rib/schema/ze-rib.yang", "Edit", []string{doorSystem}, []string{doorBGPPlugins}},
		{"templ-is-code", "internal/component/bgp/plugins/rib/page.templ", "Write", []string{doorSystem}, []string{doorBGPPlugins}},
		{"markdown-never-refused", "internal/component/bgp/plugins/rib/README.md", "Write", nil, nil},
		{"ci-never-refused", "test/plugin/rib.ci", "Write", nil, nil},
		{"doc-never-refused", "docs/architecture/core-design.md", "Edit", nil, nil},
		{"read-tool-never-refused", "internal/component/bgp/plugins/rib/rib.go", "Read", nil, nil},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeDoors(t, root)
			for _, door := range test.read {
				readDoor(t, root, "sess-a", "", door, nil)
			}
			requireDoorGate(t, root, codeEditPayload("sess-a", "", test.tool, test.file), test.unread)
		})
	}
}

// TestDoorGateMatchesAnAbsolutePath holds the gate to the repository-relative
// path when the tool names the file absolutely, as the harness does.
func TestDoorGateMatchesAnAbsolutePath(t *testing.T) {
	root := t.TempDir()
	writeDoors(t, root)
	readDoor(t, root, "sess-a", "", doorSystem, nil)
	file := filepath.Join(root, "internal", "component", "bgp", "plugins", "rib", "rib.go")
	requireDoorGate(t, root, codeEditPayload("sess-a", "", "Edit", file), []string{doorBGPPlugins})
}

// TestDoorGateCountsOnlyAWholeRead holds the Read hook to "in full": an offset
// past the first line or a limit short of the last line records nothing.
func TestDoorGateCountsOnlyAWholeRead(t *testing.T) {
	partial := []map[string]any{
		{"offset": float64(3)},
		{"limit": float64(doorPageLines - 1)},
		{"limit": "all"},
	}
	for _, window := range partial {
		root := t.TempDir()
		writeDoors(t, root)
		readDoor(t, root, "sess-a", "", doorSystem, window)
		requireDoorGate(t, root, codeEditPayload("sess-a", "", "Edit", "internal/component/foo/foo.go"), []string{doorSystem})
	}
	root := t.TempDir()
	writeDoors(t, root)
	readDoor(t, root, "sess-a", "", doorSystem, map[string]any{"offset": float64(1), "limit": float64(doorPageLines)})
	requireDoorGate(t, root, codeEditPayload("sess-a", "", "Edit", "internal/component/foo/foo.go"), nil)
}

// TestDoorGateCountsABashRead drives the Bash pre-hook: a cat of the doors with
// no pipe records each door it prints, and a piped, redirected or ranged print
// records nothing.
func TestDoorGateCountsABashRead(t *testing.T) {
	file := "internal/component/bgp/plugins/rib/rib.go"
	records := []string{
		"cat " + doorSystem + " " + doorBGPPlugins,
		"cd /repo && cat -n /repo/" + doorSystem + " && nl ./" + doorBGPPlugins,
	}
	for _, command := range records {
		root := t.TempDir()
		writeDoors(t, root)
		runBash(t, root, "sess-a", "", command)
		requireDoorGate(t, root, codeEditPayload("sess-a", "", "Edit", file), nil)
	}
	ignored := []string{
		"cat " + doorSystem + " " + doorBGPPlugins + " | head",
		"cat " + doorSystem + " " + doorBGPPlugins + " > /dev/null",
		"head -n 400 " + doorSystem + " " + doorBGPPlugins,
		"sed -n '1,400p' " + doorSystem + " " + doorBGPPlugins,
		"grep -n door " + doorSystem + " " + doorBGPPlugins,
		"less +3 " + doorSystem + " " + doorBGPPlugins,
		"bat -r 1:4 " + doorSystem + " " + doorBGPPlugins,
		"cat ai/doors/*.md",
	}
	for _, command := range ignored {
		root := t.TempDir()
		writeDoors(t, root)
		runBash(t, root, "sess-a", "", command)
		requireDoorGate(t, root, codeEditPayload("sess-a", "", "Edit", file), []string{doorSystem, doorBGPPlugins})
	}
}

// TestDoorGateHoldsEachSubagentToItsOwnRead: the parent's read does not open
// the gate for a subagent, and a subagent's read does not open it for the
// parent or a sibling.
func TestDoorGateHoldsEachSubagentToItsOwnRead(t *testing.T) {
	root := t.TempDir()
	writeDoors(t, root)
	file := "internal/component/foo/foo.go"
	readDoor(t, root, "sess-a", "", doorSystem, nil)
	requireDoorGate(t, root, codeEditPayload("sess-a", "agent1", "Edit", file), []string{doorSystem})
	runBash(t, root, "sess-a", "agent1", "cat "+doorSystem)
	requireDoorGate(t, root, codeEditPayload("sess-a", "agent1", "Edit", file), nil)
	requireDoorGate(t, root, codeEditPayload("sess-a", "agent2", "Edit", file), []string{doorSystem})
	requireDoorGate(t, root, codeEditPayload("sess-b", "", "Edit", file), []string{doorSystem})
}

// TestDoorGateFailsClosed refuses the edit when no marker can be named, and
// when the checkout holds no doors to read, rather than reading either as a
// pass.
func TestDoorGateFailsClosed(t *testing.T) {
	root := t.TempDir()
	writeDoors(t, root)
	code, _, message := runHook(t, root, "pretool-writeedit", codeEditPayload("../escape", "", "Edit", "internal/component/foo/foo.go"))
	if code != 2 || !strings.Contains(message, "no usable id, so no read of "+doorSystem) {
		t.Fatalf("code=%d message=%q, want a fail-closed refusal", code, message)
	}

	bare := t.TempDir()
	code, _, message = runHook(t, bare, "pretool-writeedit", codeEditPayload("sess-a", "", "Edit", "internal/component/foo/foo.go"))
	if code != 2 || !strings.Contains(message, "cannot be read") || !strings.Contains(message, doorSystem) {
		t.Fatalf("code=%d message=%q, want a refusal naming the missing doors", code, message)
	}

	broken := t.TempDir()
	writeDoors(t, broken)
	writeFixtureFile(t, broken, "ai/doors/broken.md", "# Door\n\n<!-- door-base: internal/component/foo\n")
	code, _, message = runHook(t, broken, "pretool-writeedit", codeEditPayload("sess-a", "", "Edit", "internal/component/foo/foo.go"))
	if code != 2 || !strings.Contains(message, "ai/doors/broken.md") {
		t.Fatalf("code=%d message=%q, want a refusal naming the malformed header", code, message)
	}
}

// TestDoorGateRefusesADoorWithoutAUsableHeader: every page under ai/doors but
// README.md owes a well-formed header. A page that lost it, or misspells the
// opener, refuses the edit and is named, rather than covering nothing.
func TestDoorGateRefusesADoorWithoutAUsableHeader(t *testing.T) {
	bodies := map[string]string{
		"no-header":     "# Door\n\nProse only.\n",
		"no-space":      "# Door\n\n<!--door-base: internal/component/foo -->\n",
		"names-no-path": "# Door\n\n<!-- door-base: -->\n",
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			readEveryDoor(t, root, "sess-a")
			writeFixtureFile(t, root, "ai/doors/foo.md", body)
			code, _, message := runHook(t, root, "pretool-writeedit", codeEditPayload("sess-a", "", "Edit", "internal/component/foo/foo.go"))
			if code != 2 || !strings.Contains(message, "ai/doors/foo.md is not a usable design door") {
				t.Fatalf("code=%d message=%q, want a refusal naming the headerless door", code, message)
			}
		})
	}
	root := t.TempDir()
	readEveryDoor(t, root, "sess-a")
	requireDoorGate(t, root, codeEditPayload("sess-a", "", "Edit", "internal/component/foo/foo.go"), nil)
}

// TestDoorGateResolvesTheCheckoutBoundary: a file reached through a symlink
// alias of the checkout is gated like any other file in it, and a file outside
// the checkout (scratch, another checkout) is not gated by this one's doors.
func TestDoorGateResolvesTheCheckoutBoundary(t *testing.T) {
	root := t.TempDir()
	writeDoors(t, root)
	readDoor(t, root, "sess-a", "", doorSystem, nil)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	aliased := filepath.Join(alias, "internal", "component", "bgp", "plugins", "rib", "new.go")
	requireDoorGate(t, root, codeEditPayload("sess-a", "", "Write", aliased), []string{doorBGPPlugins})

	outside := filepath.Join(t.TempDir(), "internal", "component", "bgp", "plugins", "rib", "probe.go")
	requireDoorGate(t, root, codeEditPayload("sess-b", "", "Write", outside), nil)
}
