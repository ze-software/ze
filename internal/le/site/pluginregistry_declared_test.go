package site

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	repoinventory "github.com/ze-software/ze/internal/le/repo/inventory"
)

// proofRefusingRegistryFixture is committedRegistryFixture with one more
// commit: the plugin's init calls the standard library. The inventory holds no
// standard-library source and the fixture retains no YANG sink, so the
// external-effect proof cannot certify that call and refuses, while the
// registration the source declares is unchanged.
func proofRefusingRegistryFixture(t *testing.T) string {
	t.Helper()
	root := committedRegistryFixture(t)
	headCommand := exec.CommandContext(t.Context(), "git", "-C", root, "rev-parse", "HEAD")
	head, err := headCommand.Output()
	if err != nil {
		t.Fatal(err)
	}
	body := "package example\nimport (\n\t\"strings\"\n\tr \"github.com/ze-software/ze/internal/component/plugin/registry\"\n)\n" +
		"func engine() {}\n" +
		"func init() {\n\t_ = strings.ToUpper(\"probe\")\n\t_ = r.Register(r.Registration{Name: \"committed-linux\", Description: \"committed description\", RunEngine: engine})\n}\n"
	stream := "commit refs/heads/main\ncommitter Fixture <fixture@example.invalid> 1700000001 +0000\ndata 5\nproof\nfrom " +
		strings.TrimSpace(string(head)) + "\nM 100644 inline internal/plugins/example/register_linux.go\ndata " +
		strconv.Itoa(len(body)) + "\n" + body + "\n\ndone\n"
	command := exec.CommandContext(t.Context(), "git", "-C", root, "fast-import", "--quiet")
	command.Stdin = strings.NewReader(stream)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	if raw, err := command.CombinedOutput(); err != nil {
		t.Fatalf("proof-refusing committed fixture: %v: %s", err, raw)
	}
	return root
}

// TestSiteRegistryRunsNoExternalEffectProof proves the website build publishes
// the declared plugin list without running the external-effect proof, which is
// a check (CI, release) and never a step of `./le site build`. Method: a
// committed tree where repoinventory.Plugins, the proof, refuses; the site's
// producer must still answer the declared registration and publish it.
//
// MUTATION: pointing committedPluginRegistry back at repoinventory.Plugins
// turns this red, because the producer then inherits the proof's refusal.
func TestSiteRegistryRunsNoExternalEffectProof(t *testing.T) {
	root := proofRefusingRegistryFixture(t)

	// Precondition: the fixture is one the proof refuses, so an answer below
	// can only come from a reader that did not run it.
	proved, err := repoinventory.Plugins(root)
	if err == nil {
		t.Fatalf("the external-effect proof accepted the fixture (%+v); the test proves nothing", proved)
	}
	if !strings.Contains(err.Error(), "external") {
		t.Fatalf("the fixture refused before the external-effect proof: %v", err)
	}

	plugins, err := committedPluginRegistry(root)
	if err != nil {
		t.Fatalf("the site registry ran a check: %v", err)
	}
	if len(plugins) != 1 || plugins[0].Name != "committed-linux" || plugins[0].Description != "committed description" {
		t.Fatalf("the site registry answered %+v, want the one declared registration", plugins)
	}

	output := t.TempDir()
	if err := publishPluginRegistry(Paths{Repository: root, Output: output}); err != nil {
		t.Fatalf("the build refused to publish the declared registry: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(output, "data", "plugin-registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	var published []registryPlugin
	if err := json.Unmarshal(raw, &published); err != nil {
		t.Fatal(err)
	}
	if len(published) != 1 || published[0].Name != "committed-linux" {
		t.Fatalf("published registry %s, want the one declared registration", raw)
	}
}
