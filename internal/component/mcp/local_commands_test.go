package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	_ "github.com/ze-software/ze/internal/component/config/storage/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/statestore"
	_ "github.com/ze-software/ze/internal/plugins/config-storage/yang"
	_ "github.com/ze-software/ze/internal/plugins/support"
	_ "github.com/ze-software/ze/internal/plugins/support/yang"
)

// mcpText answers the one text item of a tool result, failing on an error
// result.
func mcpText(t *testing.T, result map[string]any) string {
	t.Helper()
	content, ok := result["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("tool result has no content: %#v", result)
	}
	text, isText := content[0][contentTypeText].(string)
	if !isText {
		t.Fatalf("first content item is not text: %#v", content[0])
	}
	if result["isError"] == true {
		t.Fatalf("tool refused: %s", text)
	}
	return text
}

// TestMCPSupportArchiveRoundTrip collects a support archive through MCP and
// reads it back through MCP, on the daemon's real dispatcher.
//
// VALIDATES: AC-15, goal 4. `request support archive module version` through
// the MCP tool path is answered by the daemon: the archive goes into the
// daemon's store and the answer names its key and size; `show data cat <key>`
// through the same MCP caller answers exactly the stored bytes (a tar.gz,
// carried base64 because it is not UTF-8). `--output` through MCP is refused
// and names --output.
// PREVENTS: an AI connected to the daemon having no way to collect the
// archive an operator would send to support.
func TestMCPSupportArchiveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ze.config.dir", dir)
	// env caches what it read first; `show data cat` resolves the store from
	// ze.config.dir, so the cache must see this test's directory.
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	store, err := storage.Create(dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	previous := statestore.Store()
	statestore.SetStore(store)
	t.Cleanup(func() {
		statestore.SetStore(previous)
		store.Close() //nolint:errcheck // test cleanup
	})

	daemon, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	if err != nil {
		t.Fatalf("daemon dispatcher: %v", err)
	}
	t.Cleanup(daemon.Stop)
	d := daemon.Dispatcher()
	runner := &server{
		username: "ai",
		dispatch: func(ctx context.Context, _ plugin.CallerIdentity, input string) (*plugin.Response, error) {
			return d.Dispatch(&pluginserver.CommandContext{RequestContext: ctx}, input)
		},
	}

	var manifest struct {
		ArchiveKey    string `json:"archive-key"`
		ArchiveOctets int    `json:"archive-octets"`
		Reason        string `json:"reason"`
	}
	answer := mcpText(t, runner.run("request support archive module version reason ai-case"))
	if err := json.Unmarshal([]byte(answer), &manifest); err != nil {
		t.Fatalf("archive answer is not the manifest: %v\n%s", err, answer)
	}
	if manifest.ArchiveKey == "" || manifest.ArchiveOctets == 0 {
		t.Fatalf("archive answer names no key or size: %s", answer)
	}
	if manifest.Reason != "ai-case" {
		t.Errorf("reason %q, want the reason leaf", manifest.Reason)
	}
	stored, found, err := statestore.Read(manifest.ArchiveKey)
	if err != nil || !found {
		t.Fatalf("store holds nothing under %q: found=%v err=%v", manifest.ArchiveKey, found, err)
	}

	catted := mcpText(t, runner.run("show data cat "+manifest.ArchiveKey))
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(catted), &envelope); err != nil {
		t.Fatalf("show data cat answer is not JSON: %v\n%.200s", err, catted)
	}
	blob, err := command.DecodeBlob(envelope["blob"])
	if err != nil {
		t.Fatalf("show data cat answer is not a blob: %v\n%.200s", err, catted)
	}
	if string(blob) != string(stored) || len(blob) != manifest.ArchiveOctets {
		t.Errorf("show data cat answered %d bytes, the store holds %d and the manifest says %d",
			len(blob), len(stored), manifest.ArchiveOctets)
	}
	if !strings.Contains(catted, `"encoding":"base64"`) {
		t.Errorf("a tar.gz travels base64, got %.120s", catted)
	}

	refused := runner.run("request support archive --output /tmp")
	if refused["isError"] != true {
		t.Fatalf("--output through MCP answered %#v, want a refusal", refused)
	}
	content, ok := refused["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("--output refusal has no content: %#v", refused)
	}
	if text, _ := content[0][contentTypeText].(string); !strings.Contains(text, "--output") {
		t.Errorf("--output refusal %q does not name --output", text)
	}
}
