// Design: docs/architecture/api/architecture.md -- BGP feature requirement validation and dynamic group families.
// Related: bus_requirements.go -- validateBGPBusRequirements, checkRedistributionRequirements, checkFlowSpecRequirements, checkExplicitAIGPRequirements, checkFilterOwnerStateRequirements.
// Related: loader_create.go -- collectConfiguredFamilies.
package bgpconfig

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestNoPluginNameLiteralInBGPConfig verifies AC-14: production Go files in
// internal/component/bgp/config contain no hardcoded plugin-name string
// literals (such as bgp-rs, bgp-rr, bgp-rib, bgp-adj-rib-in, or
// redistribute-orchestrator). Every feature requirement discovers endpoints
// dynamically from compiled registrations.
//
// VALIDATES: AC-14 -- no production file in internal/component/bgp/config
// hardcodes a registered hyphenated plugin name as a string literal.
// PREVENTS: reintroducing hardcoded plugin-name literals in BGP config code.
func TestNoPluginNameLiteralInBGPConfig(t *testing.T) {
	banned := make(map[string]bool)
	for _, reg := range registry.All() {
		// Hyphenated names cover every BGP and redistribution plugin (bgp-rib,
		// bgp-rs, bgp-rr, bgp-adj-rib-in, bgp-filter-*, redistribute-orchestrator,
		// etc.) without matching YANG top-level root names like "bgp" or "policy".
		if strings.Contains(reg.Name, "-") {
			banned[reg.Name] = true
		}
	}
	require.NotEmpty(t, banned, "compiled-in plugin registry must be populated")

	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, parseErr, "parse %s", name)

		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			val, unquoteErr := strconv.Unquote(lit.Value)
			if unquoteErr != nil {
				return true
			}
			if banned[val] {
				pos := fset.Position(lit.Pos())
				t.Errorf("%s:%d: hardcoded plugin name literal %q is forbidden in internal/component/bgp/config", name, pos.Line, val)
			}
			return true
		})
	}
}

// TestDynamicGroupImpliedEndpoints verifies AC-36 and S22: a configuration
// whose address family appears ONLY on a dynamic group (with zero static peers)
// still surfaces that family in collectConfiguredFamilies so the implied NLRI
// family endpoint is auto-loaded and displayed.
//
// VALIDATES: AC-36, S22 -- dynamic groups contribute to ConfiguredFamilies.
// PREVENTS: a dynamic-group-only FlowSpec or EVPN router failing to load its
// NLRI codec endpoint because the collector walked only static peers.
func TestDynamicGroupImpliedEndpoints(t *testing.T) {
	const dynFlowCfg = `
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    group flow-members {
        connection {
            remote { ip dynamic; connect false; range 10.0.0.0/24; }
            local  { ip 10.0.0.1; accept true; }
        }
        session {
            asn { local 65000; }
            family { ipv4/flow { prefix { maximum 1000; } } }
        }
    }
}`
	res, err := config.LoadConfig(dynFlowCfg, "", nil)
	require.NoError(t, err)
	peers, dynGroups, err := peersAndDynamicGroups(res.Tree.Clone())
	require.NoError(t, err)
	require.Empty(t, peers, "fixture has no static peers")
	require.Len(t, dynGroups, 1)

	families := collectConfiguredFamilies(peers, dynGroups)
	require.Contains(t, families, "ipv4/flow", "dynamic group family must be collected")

	var implied []string
	for _, fam := range families {
		if plug := plugin.GetPluginForFamily(fam); plug != "" && !slices.Contains(implied, plug) {
			implied = append(implied, plug)
		}
	}
	require.Contains(t, implied, "bgp-nlri-flowspec",
		"dynamic group ipv4/flow must imply bgp-nlri-flowspec")

	// Negative check: dynamic group with unicast only does not imply bgp-nlri-flowspec.
	const dynUnicastCfg = `
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    group uni-members {
        connection {
            remote { ip dynamic; connect false; range 10.0.0.0/24; }
            local  { ip 10.0.0.1; accept true; }
        }
        session {
            asn { local 65000; }
            family { ipv4/unicast { prefix { maximum 1000; } } }
        }
    }
}`
	uniRes, err := config.LoadConfig(dynUnicastCfg, "", nil)
	require.NoError(t, err)
	uniPeers, uniGroups, err := peersAndDynamicGroups(uniRes.Tree.Clone())
	require.NoError(t, err)
	for _, fam := range collectConfiguredFamilies(uniPeers, uniGroups) {
		require.NotEqual(t, "bgp-nlri-flowspec", plugin.GetPluginForFamily(fam))
	}
}

// TestRequirementCheckRefusals verifies AC-14 (S18, S33, S34, S35): every BGP
// feature requirement rejects a bus override that removes a required message
// or violates an in-daemon constraint, returning a typed *opererr.Error carrying
// an actionable bus fix line.
//
// VALIDATES: AC-14 -- redistribution (source & destination), FlowSpec (feeds &
// in-daemon), explicit AIGP, filter-owner bgp:state, and capability-send checks
// all refuse invalid narrowed configs with a typed fix.
// PREVENTS: a narrowed bus silently disabling an active BGP feature.
func TestRequirementCheckRefusals(t *testing.T) {
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	require.NoError(t, registry.Register(registry.Registration{
		Name:        "req-test-filter",
		Description: "test filter plugin",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		ConfigRoots: []string{"bgp"},
		Receives:    []rpc.MessageReceiveDecl{{Message: bgpevents.Namespace + ":" + bgpevents.EventState}},
		Filters:     []rpc.FilterDecl{{Name: "check"}},
	}))

	cases := []struct {
		name        string
		cfg         string
		wantProblem string
	}{
		{
			name: "redistribution source bgp peer cuts Loc-RIB",
			cfg: `
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer p1 {
        connection { remote { ip 10.0.0.2; } local { ip 10.0.0.1; } }
        session { asn { local 65000; remote 65001; } }
        bus { send { to none; } }
    }
}
ospf { router-id 10.0.0.1; }
redistribute { destination ospf { import bgp; } }`,
			wantProblem: "redistribution",
		},
		{
			name: "redistribution destination bgp orchestrator cannot send",
			cfg: `
bus {
    endpoint redistribute-orchestrator {
        internal redistribute-orchestrator;
        send { allow none; }
    }
}
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer p1 {
        connection { remote { ip 10.0.0.2; } local { ip 10.0.0.1; } }
        session { asn { local 65000; remote 65001; } }
    }
}
isis { net 49.0001.0000.0000.0001.00; }
redistribute { destination bgp { import isis; } }`,
			wantProblem: "redistribution",
		},
		{
			name: "FlowSpec separate-process selecting RIB",
			cfg: `
bus {
    endpoint routes {
        internal bgp-rib;
        process separate;
    }
}
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer p1 {
        connection { remote { ip 10.0.0.2; } local { ip 10.0.0.1; } }
        session {
            asn { local 65000; remote 65001; }
            family { ipv4/flow { prefix { maximum 1000; } } }
        }
    }
}`,
			wantProblem: "in-daemon",
		},
		{
			name: "FlowSpec peer cuts selecting RIB",
			cfg: `
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer p1 {
        connection { remote { ip 10.0.0.2; } local { ip 10.0.0.1; } }
        session {
            asn { local 65000; remote 65001; }
            family { ipv4/flow { prefix { maximum 1000; } } }
        }
        bus { send { to none; } }
    }
}`,
			wantProblem: "FlowSpec",
		},
		{
			name: "explicit AIGP peer cuts selecting RIB",
			cfg: `
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer p1 {
        connection { remote { ip 10.0.0.2; } local { ip 10.0.0.1; } }
        session { asn { local 65000; remote 65001; } aigp { enabled true; } }
        bus { send { to none; } }
    }
}`,
			wantProblem: "AIGP",
		},
		{
			name: "plugin filter owner cut from bgp:state",
			cfg: `
bus {
    endpoint req-test-filter {
        internal req-test-filter;
        receive { allow none; }
    }
}
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer p1 {
        connection { remote { ip 10.0.0.2; } local { ip 10.0.0.1; } }
        session { asn { local 65000; remote 65001; } }
        filter { export { filters [ req-test-filter:check ]; final accept; } }
    }
}`,
			wantProblem: "req-test-filter",
		},
		{
			name: "route-refresh enabled with receive from none",
			cfg: `
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer p1 {
        connection { remote { ip 10.0.0.2; } local { ip 10.0.0.1; } }
        session {
            asn { local 65000; remote 65001; }
            capability { route-refresh enable; }
        }
        bus { receive { from none; } }
    }
}`,
			wantProblem: "route-refresh",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.LoadConfig(tc.cfg, "", nil)
			require.Error(t, err)
			var oe *opererr.Error
			require.ErrorAs(t, err, &oe)
			require.Contains(t, err.Error(), tc.wantProblem)
			require.NotEmpty(t, oe.Why)
			require.NotEmpty(t, oe.Fix)
		})
	}
}
