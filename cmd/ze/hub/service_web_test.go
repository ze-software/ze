//go:build ze_web

package hub

// VALIDATES: the ze_web-gated web factory is registered through the service
// construction registry and can build a running web service from generic deps.
// PREVENTS: reintroducing a direct always-on web.NewWebServer construction path.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func TestServiceRegistry_BuildsWeb(t *testing.T) {
	withCleanRegistry(t)
	registerService("web", buildWebService, nil)

	store, err := storage.Create(t.TempDir())
	if err != nil {
		t.Fatalf("blob storage: %v", err)
	}

	services := buildServices(&serviceDeps{
		Store:      store,
		ConfigPath: "config.conf",
		Dispatch: func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
			return plugin.NewResponse(plugin.StatusDone, nil), nil
		},
		WebEnabled:  true,
		WebAddrs:    []string{"127.0.0.1:0"},
		InsecureWeb: true,
	})
	if len(services) != 1 {
		t.Fatalf("buildServices built %d services, want 1", len(services))
	}
	if services[0].Name() != "web" {
		t.Fatalf("service name = %q, want web", services[0].Name())
	}
	if len(services[0].Addresses()) != 1 {
		t.Fatalf("web service addresses = %v, want one bound address", services[0].Addresses())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := services[0].Shutdown(ctx); err != nil {
		t.Fatalf("shutdown web service: %v", err)
	}
}

// TestAdminTreeMergesVisiblePluginCommands checks the web admin console's tree
// producer against a live plugin registry: a plugin command with no YANG node
// appears in the tree answered for the next request, a hidden one never does,
// the shared YANG tree is never written, and a command the plugin no longer
// registers is gone from the next answer.
//
// VALIDATES: G2 of spec-command-surface-parity: adminCommandTrees overlays the
// registry per request (command.OverlayCommandPaths of VisibleCommandEntries).
// PREVENTS: a plugin command running from every other surface and missing from
// the admin console, and a startup-built tree that ignores later plugins.
func TestAdminTreeMergesVisiblePluginCommands(t *testing.T) {
	const visible = "show zz-admin-fixture visible"
	const hidden = "show zz-admin-fixture hidden"
	base := &command.Node{Children: map[string]*command.Node{
		"show": {Name: "show", Children: map[string]*command.Node{"version": {Name: "version", WireMethod: "ze-system:version"}}},
	}}
	d := pluginserver.NewDispatcher()
	trees := adminCommandTrees(base, func() []command.CommandEntry { return d.Registry().VisibleCommandEntries() })

	if hasAdminPath(trees(), visible) {
		t.Fatal("the fixture command is in the tree before any plugin registered it")
	}
	fixture := process.NewProcess(plugin.PluginConfig{Name: "zz-admin-fixture"})
	for _, res := range d.Registry().Register(fixture, []pluginserver.CommandDef{
		{Name: visible, ShortHelp: "A fixture command with no YANG node."},
		{Name: hidden, ShortHelp: "A hidden fixture command.", Hidden: true},
	}) {
		if !res.OK {
			t.Fatalf("fixture command %q: %s", res.Name, res.Error)
		}
	}

	tree := trees()
	if !hasAdminPath(tree, visible) {
		t.Errorf("%q is missing from the admin tree after its plugin registered it", visible)
	}
	if hasAdminPath(tree, hidden) {
		t.Errorf("hidden %q is in the admin tree", hidden)
	}
	if !hasAdminPath(tree, "show version") {
		t.Error("the YANG command show version is missing from the overlaid tree")
	}
	if _, written := base.Children["show"].Children["zz-admin-fixture"]; written {
		t.Error("the overlay wrote the plugin command into the shared YANG tree")
	}

	d.Registry().Unregister(fixture, []string{visible, hidden})
	if hasAdminPath(trees(), visible) {
		t.Errorf("%q stayed in the admin tree after its plugin went away", visible)
	}

	if static := adminCommandTrees(base, nil)(); static != base {
		t.Error("with no plugin registry the producer did not answer the YANG tree itself")
	}
}

// hasAdminPath answers whether each word of path is a child of the one before.
func hasAdminPath(tree *command.Node, path string) bool {
	node := tree
	for _, word := range strings.Fields(path) {
		if node == nil {
			return false
		}
		node = node.Children[word]
	}
	return node != nil
}
