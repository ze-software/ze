// Design: ai/rules/plugins.md -- a plugin owns its whole surface, its rpcs included
// Related: ../../../../cmd/ze/hub/build_tag_health_rpcs_absent_test.go -- the same rpcs with neither owner
//
//go:build ze_l2tp && ze_bgp

package all

import (
	"strings"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// healthRPCOwner names where one health rpc lives and how the command that
// reaches it is spelled on the wire. The wire method and the rpc name stay as
// they were before the rpc moved, so no CLI path and no wire method changes.
type healthRPCOwner struct {
	method      string
	module      string
	rpc         string
	help        string
	description string // a prefix of the rpc's description
}

var healthRPCOwners = []healthRPCOwner{
	{
		method:      "ze-bgp:show-health",
		module:      "ze-bgp-api",
		rpc:         "bgp-health",
		help:        "Show BGP peer health sorted by state and activity.",
		description: "Each row carries the peer address, the state, the remote AS and",
	},
	{
		method:      "ze-l2tp:show-health",
		module:      "ze-l2tp-api",
		rpc:         "l2tp-health",
		help:        "Show L2TP session health sorted by echo loss.",
		description: "The rows are sorted by average round-trip time, highest first.",
	},
}

// TestHealthRPCsPublishedOnlyWithOwner proves each health rpc is declared by
// the API module of the component that answers it, and is published under the
// owner's wire method.
//
// VALIDATES: with ze_l2tp and ze_bgp linked, PublishedRPCs.Commands holds
// ze-l2tp:show-health pointing at ze-l2tp-api:l2tp-health and
// ze-bgp:show-health pointing at ze-bgp-api:bgp-health, with their help and
// description text, and ze-cli-show-api declares neither rpc.
// PREVENTS: an always-linked module declaring an rpc only a gated component
// answers, which a build without that component publishes under Unnamed.
func TestHealthRPCsPublishedOnlyWithOwner(t *testing.T) {
	loader := configyang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	if err := loader.LoadRegistered(); err != nil {
		t.Fatalf("LoadRegistered: %v", err)
	}
	schema, err := loader.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	pub, err := configyang.PublishedRPCs(schema)
	if err != nil {
		t.Fatalf("PublishedRPCs: %v", err)
	}

	commands := make(map[string]configyang.RPCMeta, len(pub.Commands))
	for _, meta := range pub.Commands {
		commands[meta.WireMethod] = meta
	}
	for _, owner := range healthRPCOwners {
		meta, ok := commands[owner.method]
		if !ok {
			t.Errorf("%s is not published as a command", owner.method)
			continue
		}
		if meta.Module != owner.module || meta.Name != owner.rpc {
			t.Errorf("%s points at %s:%s, want %s:%s", owner.method, meta.Module, meta.Name, owner.module, owner.rpc)
		}
		if meta.ShortHelp != owner.help {
			t.Errorf("%s help = %q, want %q", owner.method, meta.ShortHelp, owner.help)
		}
		if !strings.HasPrefix(meta.Description, owner.description) {
			t.Errorf("%s description = %q, want it to start %q", owner.method, meta.Description, owner.description)
		}
	}

	for _, meta := range configyang.ExtractRPCs(schema, "ze-cli-show-api") {
		for _, owner := range healthRPCOwners {
			if meta.Name == owner.rpc {
				t.Errorf("ze-cli-show-api declares %s, which %s owns", owner.rpc, owner.module)
			}
		}
	}
}
