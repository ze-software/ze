// Design: docs/architecture/core-design.md -- a data-plane-mismatched feature stays off at daemon start and is reported under doctor-dataplane-mismatch
// Related: dataplane_start_exclude.go -- excludeDataPlaneMismatches and ExcludedDataPlane
// Related: loader.go -- LoadConfigForStart vs strict LoadConfig and ReadConfig

package config

import (
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/configvalue"
)

const startExcludeTestSchemaYANG = `module ze-test-dataplane-start-exclude-conf {
    yang-version 1.1;
    namespace "urn:ze:test:dataplane-start-exclude";
    prefix dse;

    import ze-extensions { prefix ze; }
    import ze-hub-conf { prefix hub; }
    import ze-fib-conf { prefix fib; }
    import ze-iface-conf { prefix iface; }

    container firewall {
        leaf backend {
            type string;
            default "kernel";
        }
        list table {
            key "name";
            leaf name { type string; }
        }
    }

    container traffic {
        container control {
            leaf backend {
                type string;
                default "kernel";
            }
        }
    }

    container vpn {
        container ipsec {
            leaf enabled {
                type boolean;
                default "true";
            }
        }
    }

    container l2tp {
        leaf enabled {
            type boolean;
            default "true";
        }
        leaf pool {
            type string;
        }
    }

    container flow-export {
        container collector {
            leaf address { type string; }
        }
        container sampling {
            leaf rate { type uint32; }
        }
        container conntrack {
            leaf enabled {
                type boolean;
                default "false";
            }
        }
    }

    augment "/hub:environment" {
        container l2tp {
            list server {
                key "name";
                leaf name { type string; }
                leaf listen { type string; }
            }
        }
    }

    augment "/fib:fib" {
        container vpp {
            leaf enabled {
                type boolean;
                default "true";
            }
        }
        container dummy {
            leaf enabled {
                type boolean;
                default "true";
            }
        }
    }

    augment "/iface:interface/iface:ethernet" {
        container vrrp {
            ze:backend "kernel";
            leaf vrid { type uint8; }
        }
    }
}`

func setupStartExcludeTest(t *testing.T) {
	t.Helper()
	t.Cleanup(ResetDeclaredSchemasForTest())
	t.Cleanup(configyang.RegisterModuleForTest("ze-test-dataplane-start-exclude-conf.yang", startExcludeTestSchemaYANG))
	isolateRegistry(t)
	savedCLI := CommandLinePlugins()
	t.Cleanup(func() { RecordCommandLinePlugins(savedCLI) })
	RecordCommandLinePlugins(nil)

	registerTestSelectors(t, []string{"kernel", "vpp"}, []string{"kernel", "vpp"}, []string{"kernel", "vpp"})
}

func hasPluginNamed(plugins []plugin.PluginConfig, name string) bool {
	for _, p := range plugins {
		if p.Name == name || p.CompiledPlugin == name {
			return true
		}
	}
	return false
}

func TestRemoveWildcardPathAndSetLeafPath(t *testing.T) {
	// VALIDATES: Tree.RemoveWildcardPath deletes concrete paths, single-wildcard
	// paths, and nested-wildcard paths across every list entry, while
	// Tree.SetLeafPath updates a leaf only when its parent container already
	// exists and clears any inactive marker on that leaf.
	// PREVENTS: partial wildcard deletion leaving later entries behind, or
	// SetLeafPath fabricating an empty container that was never configured.
	tree := NewTree()
	bgp := tree.GetOrCreateContainer("bgp")

	p1 := NewTree()
	p1.GetOrCreateContainer("connection").GetOrCreateContainer("ttl").Set("min", "255")
	p1.Set("description", "peer-one")
	bgp.AddListEntry("peer", "192.0.2.1", p1)

	p2 := NewTree()
	p2.GetOrCreateContainer("connection").GetOrCreateContainer("ttl").Set("max", "1")
	p2.Set("description", "peer-two")
	bgp.AddListEntry("peer", "192.0.2.2", p2)

	grp := NewTree()
	grp.GetOrCreateContainer("connection").GetOrCreateContainer("ttl").Set("set", "255")
	gp := NewTree()
	gp.GetOrCreateContainer("connection").GetOrCreateContainer("ttl").Set("min", "255")
	gp.Set("description", "group-peer")
	grp.AddListEntry("peer", "192.0.2.3", gp)
	bgp.AddListEntry("group", "edge", grp)

	require.True(t, tree.RemoveWildcardPath("bgp/peer[*]/connection/ttl"))
	require.False(t, tree.RemoveWildcardPath("bgp/peer[*]/connection/ttl"), "second removal is a no-op")
	require.Nil(t, p1.GetContainer("connection").GetContainer("ttl"))
	require.Nil(t, p2.GetContainer("connection").GetContainer("ttl"))
	desc, ok := p1.Get("description")
	require.True(t, ok)
	require.Equal(t, "peer-one", desc)

	require.True(t, tree.RemoveWildcardPath("bgp/group[*]/connection/ttl"))
	require.Nil(t, grp.GetContainer("connection").GetContainer("ttl"))

	require.True(t, tree.RemoveWildcardPath("bgp/group[*]/peer[*]/connection/ttl"))
	require.Nil(t, gp.GetContainer("connection").GetContainer("ttl"))
	gDesc, gOK := gp.Get("description")
	require.True(t, gOK)
	require.Equal(t, "group-peer", gDesc)

	// SetLeafPath does not create missing parent containers.
	require.False(t, tree.SetLeafPath("l2tp/enabled", "false"))
	require.Nil(t, tree.GetContainer("l2tp"))

	// SetLeafPath writes when the parent exists and clears inactive state.
	l2tp := tree.GetOrCreateContainer("l2tp")
	l2tp.Set("enabled", "true")
	l2tp.SetLeafInactive("enabled", true)
	require.True(t, tree.SetLeafPath("l2tp/enabled", "false"))
	require.False(t, l2tp.IsLeafInactive("enabled"))
	val, valOK := l2tp.Get("enabled")
	require.True(t, valOK)
	require.Equal(t, "false", val)
}

func TestLoadConfigForStartExcludesKernelFirewallOnVPP(t *testing.T) {
	// VALIDATES: LoadConfigForStart leaves a kernel firewall off under
	// interface backend vpp, removes firewall from result.Tree and
	// result.Plugins, and records the mismatch with `set firewall backend vpp`
	// while LoadConfig and ReadConfig stay strictly refusing.
	// PREVENTS: a kernel firewall blocking VPP daemon start or silently
	// starting on Linux nftables beside VPP interfaces.
	setupStartExcludeTest(t)

	const cfg = `
interface {
	backend vpp;
}
firewall {
	table wan {
	}
}
`
	result, err := LoadConfigForStart(cfg, "test.conf", nil)
	require.NoError(t, err)
	require.Nil(t, result.Tree.GetContainer("firewall"), "firewall subtree must be pruned from start tree")
	require.False(t, hasPluginNamed(result.Plugins, "firewall"), "firewall plugin must not start")
	require.False(t, hasPluginNamed(result.CandidateSelection, "firewall"))
	require.Len(t, result.ExcludedDataPlane, 1)
	require.Contains(t, result.ExcludedDataPlane[0].Problem, "firewall")
	require.Contains(t, result.ExcludedDataPlane[0].Fix, "set firewall backend vpp")

	_, loadErr := LoadConfig(cfg, "test.conf", nil)
	require.Error(t, loadErr, "LoadConfig must remain strict")
	require.Contains(t, loadErr.Error(), "set firewall backend vpp")

	_, readErr := ReadConfig(cfg, "test.conf", nil)
	require.Error(t, readErr, "ReadConfig must remain strict")
	require.Contains(t, readErr.Error(), "set firewall backend vpp")
}

func TestLoadConfigForStartExcludesVPNIPsecOnVPP(t *testing.T) {
	// VALIDATES: LoadConfigForStart excludes vpn ipsec under interface backend
	// vpp, removes vpn from result.Tree, omits ike from result.Plugins, and
	// reports `vpn ipsec` with `set interface backend kernel`.
	// PREVENTS: IPsec stopping VPP daemon start or programming kernel XFRM
	// tunnels on a VPP router.
	setupStartExcludeTest(t)

	require.NoError(t, registry.Register(registry.Registration{
		Name:        "ike",
		Description: "test IKE/IPsec plugin",
		ConfigRoots: []string{"vpn"},
		DataPlaneFeatures: []registry.DataPlaneFeature{{
			ConfigPath: "vpn ipsec",
			Why:        "IPsec programs kernel XFRM only",
			Planes:     []string{"kernel"},
			InUseKind:  registry.DataPlaneInUseConfigRoot,
		}},
		RunEngine:  func(net.Conn) int { return 0 },
		CLIHandler: func([]string) int { return 0 },
	}))

	const cfg = `
interface {
	backend vpp;
}
vpn {
	ipsec {
		enabled true;
	}
}
`
	result, err := LoadConfigForStart(cfg, "test.conf", nil)
	require.NoError(t, err)
	require.Nil(t, result.Tree.GetContainer("vpn"), "vpn subtree must be pruned from start tree")
	require.False(t, hasPluginNamed(result.Plugins, "ike"), "ike plugin must not start")
	require.Len(t, result.ExcludedDataPlane, 1)
	require.Contains(t, result.ExcludedDataPlane[0].Problem, "vpn ipsec")
	require.Contains(t, result.ExcludedDataPlane[0].Fix, "set interface backend kernel")

	_, loadErr := LoadConfig(cfg, "test.conf", nil)
	require.Error(t, loadErr)
	_, readErr := ReadConfig(cfg, "test.conf", nil)
	require.Error(t, readErr)
}

func TestLoadConfigForStartExcludesFIBKernelAloneOnVPP(t *testing.T) {
	// VALIDATES: LoadConfigForStart excludes fib kernel when configured alone
	// under interface backend vpp, removes fib/kernel, omits fib-kernel from
	// result.Plugins, and reports `fib kernel` with `set fib vpp`.
	// PREVENTS: fib kernel alone blocking VPP daemon start or loading
	// fib-kernel without fib-vpp.
	setupStartExcludeTest(t)

	require.NoError(t, registry.Register(registry.Registration{
		Name:              "fib-kernel",
		Description:       "test kernel FIB writer",
		ConfigRoots:       []string{"fib/kernel"},
		ProgramsFIB:       true,
		DataPlane:         "kernel",
		HostSideDataPlane: true,
		RunEngine:         func(net.Conn) int { return 0 },
		CLIHandler:        func([]string) int { return 0 },
	}))
	require.NoError(t, registry.Register(registry.Registration{
		Name:        "fib-vpp",
		Description: "test VPP FIB writer",
		ConfigRoots: []string{"fib/vpp"},
		ProgramsFIB: true,
		DataPlane:   "vpp",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	}))

	const cfg = `
interface {
	backend vpp;
}
fib {
	kernel {
	}
}
`
	result, err := LoadConfigForStart(cfg, "test.conf", nil)
	require.NoError(t, err)
	if fibRoot := result.Tree.GetContainer("fib"); fibRoot != nil {
		require.Nil(t, fibRoot.GetContainer("kernel"), "fib/kernel must be pruned from start tree")
	}
	require.False(t, hasPluginNamed(result.Plugins, "fib-kernel"), "fib-kernel plugin must not start")
	require.Len(t, result.ExcludedDataPlane, 1)
	require.Contains(t, result.ExcludedDataPlane[0].Problem, "fib kernel")
	require.Contains(t, result.ExcludedDataPlane[0].Fix, "set fib vpp")

	_, loadErr := LoadConfig(cfg, "test.conf", nil)
	require.Error(t, loadErr)
	_, readErr := ReadConfig(cfg, "test.conf", nil)
	require.Error(t, readErr)
}

func TestLoadConfigForStartExcludesCLIPluginFlowspecOnVPP(t *testing.T) {
	// VALIDATES: LoadConfigForStart strips a mismatched --plugin selector
	// (ze.flowspec-firewall) from result.Plugins, result.CLIPlugins, and
	// result.CandidateSelection even when CommandLinePlugins also records it,
	// while leaving CommandLinePlugins intact so strict LoadConfig and
	// ReadConfig still refuse.
	// PREVENTS: a --plugin flag blocking VPP start, starting flowspec-firewall
	// anyway, or clearing the process-wide CLI plugin record used by reloads.
	setupStartExcludeTest(t)
	registerTestFlowspecProducer(t)

	const cfg = `
interface {
	backend vpp;
}
`
	cliArgs := []string{"ze.flowspec-firewall"}
	RecordCommandLinePlugins(cliArgs)

	result, err := LoadConfigForStart(cfg, "test.conf", cliArgs)
	require.NoError(t, err)
	require.False(t, hasPluginNamed(result.Plugins, "flowspec-firewall"), "flowspec-firewall must not start")
	require.False(t, hasPluginNamed(result.CandidateSelection, "flowspec-firewall"), "flowspec-firewall must leave CandidateSelection")
	require.Empty(t, result.CLIPlugins)
	require.Len(t, result.ExcludedDataPlane, 1)
	require.Contains(t, result.ExcludedDataPlane[0].Problem, "--plugin ze.flowspec-firewall")
	require.Contains(t, result.ExcludedDataPlane[0].Fix, "set firewall backend vpp")

	// CommandLinePlugins MUST remain intact for subsequent reloads/commits.
	require.Equal(t, cliArgs, CommandLinePlugins())
	_, loadErr := LoadConfig(cfg, "test.conf", cliArgs)
	require.Error(t, loadErr)
	_, readErr := ReadConfig(cfg, "test.conf", nil)
	require.Error(t, readErr)
}

func TestLoadConfigForStartExcludesL2TPPreservingSharedBlock(t *testing.T) {
	// VALIDATES: a predicate feature declaring DisablePaths and DisableLeaves
	// (L2TP) removes environment/l2tp and sets l2tp/enabled to "false" while
	// keeping sibling leaves under l2tp (shared with PPPoE) intact.
	// PREVENTS: deleting the whole l2tp container and breaking PPPoE pools/auth
	// on VPP, or leaving l2tp enabled at start.
	setupStartExcludeTest(t)

	require.NoError(t, registry.RegisterDataPlaneFeature(registry.DataPlaneFeature{
		Name:       "l2tp",
		ConfigPath: "environment l2tp server",
		Why:        "L2TP programs Linux PPP interfaces in the kernel only",
		Planes:     []string{"kernel"},
		InUseKind:  registry.DataPlaneInUsePredicate,
		InUse: func(tree map[string]any) registry.DataPlaneFeatureUse {
			envL2TP := configvalue.Section("environment/l2tp", tree)
			if envL2TP == nil || len(configvalue.ListEntries(envL2TP["server"])) == 0 {
				return registry.DataPlaneFeatureUse{}
			}
			if root := configvalue.Section("l2tp", tree); root != nil {
				if enabled, ok := configvalue.Bool(root["enabled"]); ok && !enabled {
					return registry.DataPlaneFeatureUse{}
				}
			}
			return registry.DataPlaneFeatureUse{InUse: true}
		},
		DisablePaths:  []string{"environment/l2tp"},
		DisableLeaves: map[string]string{"l2tp/enabled": "false"},
	}))

	const cfg = `
interface {
	backend vpp;
}
l2tp {
	enabled true;
	pool shared-bng;
}
environment {
	l2tp {
		server main {
			listen 127.0.0.1:1701;
		}
	}
}
`
	result, err := LoadConfigForStart(cfg, "test.conf", nil)
	require.NoError(t, err)
	require.Len(t, result.ExcludedDataPlane, 1)
	require.Contains(t, result.ExcludedDataPlane[0].Problem, "environment l2tp server")

	envRoot := result.Tree.GetContainer("environment")
	if envRoot != nil {
		require.Nil(t, envRoot.GetContainer("l2tp"), "environment/l2tp must be removed")
	}
	l2tpRoot := result.Tree.GetContainer("l2tp")
	require.NotNil(t, l2tpRoot, "shared l2tp container must survive for PPPoE")
	enabled, ok := l2tpRoot.Get("enabled")
	require.True(t, ok)
	require.Equal(t, "false", enabled)
	pool, poolOK := l2tpRoot.Get("pool")
	require.True(t, poolOK)
	require.Equal(t, "shared-bng", pool)

	// Without an l2tp container in the config, SetLeafPath does not invent one.
	const envOnlyCfg = `
interface {
	backend vpp;
}
environment {
	l2tp {
		server main {
			listen 127.0.0.1:1701;
		}
	}
}
`
	envOnlyRes, envOnlyErr := LoadConfigForStart(envOnlyCfg, "test.conf", nil)
	require.NoError(t, envOnlyErr)
	require.Nil(t, envOnlyRes.Tree.GetContainer("l2tp"), "absent l2tp container must not be created")
	require.Len(t, envOnlyRes.ExcludedDataPlane, 1)
}

func TestLoadConfigForStartExcludesGTSMAndFlowExportPredicates(t *testing.T) {
	// VALIDATES: wildcard DisablePaths disable GTSM across top-level peers and
	// group peers while keeping BGP peers intact, and concrete DisablePaths
	// disable flow-export sampling and conntrack while keeping collectors.
	// PREVENTS: GTSM or flow-export kernel sub-features blocking VPP start or
	// dropping the healthy BGP peer / counter collector siblings.
	setupStartExcludeTest(t)

	require.NoError(t, registry.RegisterDataPlaneFeature(registry.DataPlaneFeature{
		Name:       "gtsm",
		ConfigPath: "bgp",
		Why:        "GTSM programs kernel routes and nftables",
		Planes:     []string{"kernel"},
		InUseKind:  registry.DataPlaneInUsePredicate,
		InUse: func(tree map[string]any) registry.DataPlaneFeatureUse {
			bgp := configvalue.Section("bgp", tree)
			if bgp == nil {
				return registry.DataPlaneFeatureUse{}
			}
			hasTTL := func(m map[string]any) bool {
				conn, ok := m["connection"].(map[string]any)
				if !ok {
					return false
				}
				ttl, ok := conn["ttl"].(map[string]any)
				return ok && len(ttl) > 0
			}
			for _, peer := range configvalue.ListEntries(bgp["peer"]) {
				if hasTTL(peer.Fields) {
					return registry.DataPlaneFeatureUse{InUse: true, ConfigPath: "bgp peer " + peer.Key + " connection ttl"}
				}
			}
			for _, grp := range configvalue.ListEntries(bgp["group"]) {
				if hasTTL(grp.Fields) {
					return registry.DataPlaneFeatureUse{InUse: true, ConfigPath: "bgp group " + grp.Key + " connection ttl"}
				}
				for _, peer := range configvalue.ListEntries(grp.Fields["peer"]) {
					if hasTTL(peer.Fields) {
						return registry.DataPlaneFeatureUse{InUse: true, ConfigPath: "bgp group " + grp.Key + " peer " + peer.Key + " connection ttl"}
					}
				}
			}
			return registry.DataPlaneFeatureUse{}
		},
		DisablePaths: []string{
			"bgp/peer[*]/connection/ttl",
			"bgp/group[*]/connection/ttl",
			"bgp/group[*]/peer[*]/connection/ttl",
		},
	}))

	require.NoError(t, registry.Register(registry.Registration{
		Name:        "flow-export",
		Description: "test flow export",
		ConfigRoots: []string{"flow-export"},
		DataPlaneFeatures: []registry.DataPlaneFeature{
			{
				ConfigPath: "flow-export sampling",
				Why:        "sampling installs a kernel tc filter",
				Planes:     []string{"kernel"},
				InUseKind:  registry.DataPlaneInUsePredicate,
				InUse: func(tree map[string]any) registry.DataPlaneFeatureUse {
					if configvalue.Section("flow-export/sampling", tree) != nil {
						return registry.DataPlaneFeatureUse{InUse: true}
					}
					return registry.DataPlaneFeatureUse{}
				},
				DisablePaths: []string{"flow-export/sampling"},
			},
			{
				ConfigPath: "flow-export conntrack",
				Why:        "conntrack sees no VPP flows",
				Planes:     []string{"kernel"},
				InUseKind:  registry.DataPlaneInUsePredicate,
				InUse: func(tree map[string]any) registry.DataPlaneFeatureUse {
					ct := configvalue.Section("flow-export/conntrack", tree)
					if ct == nil {
						return registry.DataPlaneFeatureUse{}
					}
					if enabled, ok := configvalue.Bool(ct["enabled"]); ok && enabled {
						return registry.DataPlaneFeatureUse{InUse: true}
					}
					return registry.DataPlaneFeatureUse{}
				},
				DisablePaths: []string{"flow-export/conntrack"},
			},
		},
		RunEngine:  func(net.Conn) int { return 0 },
		CLIHandler: func([]string) int { return 0 },
	}))

	const cfg = `
interface {
	backend vpp;
}
bgp {
	router-id 192.0.2.1;
	session {
		asn {
			local 65001;
		}
	}
	peer p1 {
		connection {
			ttl {
				min 255;
			}
		}
	}
	group g1 {
		peer p2 {
			connection {
				ttl {
					max 1;
				}
			}
		}
	}
}
flow-export {
	collector {
		address 192.0.2.10;
	}
	sampling {
		rate 1000;
	}
	conntrack {
		enabled true;
	}
}
`
	result, err := LoadConfigForStart(cfg, "test.conf", nil)
	require.NoError(t, err)
	require.Len(t, result.ExcludedDataPlane, 3)

	bgpRoot := result.Tree.GetContainer("bgp")
	require.NotNil(t, bgpRoot)
	p1 := bgpRoot.GetList("peer")["p1"]
	require.NotNil(t, p1, "peer p1 must survive")
	require.Nil(t, p1.GetContainer("connection").GetContainer("ttl"), "p1 ttl must be removed")

	g1 := bgpRoot.GetList("group")["g1"]
	require.NotNil(t, g1, "group g1 must survive")
	p2 := g1.GetList("peer")["p2"]
	require.NotNil(t, p2, "group peer p2 must survive")
	require.Nil(t, p2.GetContainer("connection").GetContainer("ttl"), "p2 ttl must be removed")

	feRoot := result.Tree.GetContainer("flow-export")
	require.NotNil(t, feRoot, "flow-export must survive for counter collectors")
	require.NotNil(t, feRoot.GetContainer("collector"))
	require.Nil(t, feRoot.GetContainer("sampling"))
	require.Nil(t, feRoot.GetContainer("conntrack"))
}

func TestLoadConfigForStartExcludesBackendAnnotatedNodes(t *testing.T) {
	// VALIDATES: a ze:backend "kernel" container (ipv4 dhcp under an interface
	// unit) deletes only that container while the interface and unit survive,
	// and a ze:backend "kernel" leaf inside a list entry (tunnel gre ttl)
	// deletes the entire enclosing tunnel list entry while leaving sibling
	// tunnels intact.
	// PREVENTS: deleting the whole interface when only a kernel-only child
	// container mismatches, or approximating a tunnel by stripping only its
	// ttl leaf.
	setupStartExcludeTest(t)

	const cfg = `
interface {
	backend vpp;
	ethernet eth0 {
		mtu 1500;
		unit 0 {
			ipv4 {
				dhcp {
					enabled true;
				}
			}
		}
	}
	tunnel tun0 {
		encapsulation {
			gre {
				local {
					ip 192.0.2.1;
				}
				remote {
					ip 192.0.2.2;
				}
				ttl 64;
			}
		}
	}
	tunnel tun1 {
		encapsulation {
			gre {
				local {
					ip 192.0.2.3;
				}
				remote {
					ip 192.0.2.4;
				}
			}
		}
	}
}
`
	result, err := LoadConfigForStart(cfg, "test.conf", nil)
	require.NoError(t, err)
	require.Len(t, result.ExcludedDataPlane, 2)

	ifaceRoot := result.Tree.GetContainer("interface")
	require.NotNil(t, ifaceRoot)

	eth0 := ifaceRoot.GetList("ethernet")["eth0"]
	require.NotNil(t, eth0, "ethernet eth0 must survive when only ipv4 dhcp is excluded")
	unit0 := eth0.GetList("unit")["0"]
	require.NotNil(t, unit0, "unit 0 must survive when only ipv4 dhcp is excluded")
	require.Nil(t, unit0.GetContainer("ipv4").GetContainer("dhcp"), "dhcp container must be removed")
	mtu, ok := eth0.Get("mtu")
	require.True(t, ok)
	require.Equal(t, "1500", mtu)

	tunnels := ifaceRoot.GetList("tunnel")
	require.Nil(t, tunnels["tun0"], "tunnel tun0 with kernel-only gre ttl leaf must be removed whole")
	require.NotNil(t, tunnels["tun1"], "valid tunnel tun1 must survive")

	_, loadErr := LoadConfig(cfg, "test.conf", nil)
	require.Error(t, loadErr, "LoadConfig must still refuse ze:backend mismatches")
}

func TestLoadConfigForStartStillRefusesHardFailures(t *testing.T) {
	// VALIDATES: retired backend names (netlink, nft, tc), fib dummy beside
	// another FIB writer, and a predicate feature lacking DisablePaths /
	// DisableLeaves all still fail LoadConfigForStart closed.
	// PREVENTS: start-time exclusion silently bypassing unknown selectors,
	// contradictory fib dummy choices, or un-disableable mismatched features.
	t.Run("retired backend names refuse start", func(t *testing.T) {
		setupStartExcludeTest(t)

		const cfg = `
interface {
	backend netlink;
}
firewall {
	backend nft;
}
traffic {
	control {
		backend tc;
	}
}
`
		_, err := LoadConfigForStart(cfg, "test.conf", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), `interface backend "netlink"`)
		require.Contains(t, err.Error(), `firewall backend "nft"`)
		require.Contains(t, err.Error(), `traffic control backend "tc"`)
	})

	t.Run("fib dummy beside fib kernel refuses start", func(t *testing.T) {
		setupStartExcludeTest(t)

		require.NoError(t, registry.Register(registry.Registration{
			Name:              "fib-kernel",
			Description:       "test kernel FIB writer",
			ConfigRoots:       []string{"fib/kernel"},
			ProgramsFIB:       true,
			DataPlane:         "kernel",
			HostSideDataPlane: true,
			RunEngine:         func(net.Conn) int { return 0 },
			CLIHandler:        func([]string) int { return 0 },
		}))
		require.NoError(t, registry.Register(registry.Registration{
			Name:              "fib-dummy",
			Description:       "test no-op FIB writer",
			ConfigRoots:       []string{"fib/dummy"},
			ProgramsFIB:       true,
			NoDataPlaneChoice: true,
			RunEngine:         func(net.Conn) int { return 0 },
			CLIHandler:        func([]string) int { return 0 },
		}))

		const cfg = `
interface {
	backend kernel;
}
fib {
	kernel {
	}
	dummy {
	}
}
`
		_, err := LoadConfigForStart(cfg, "test.conf", nil)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "fib dummy"), "got: %v", err)
	})

	t.Run("predicate without disable metadata fails closed", func(t *testing.T) {
		setupStartExcludeTest(t)

		require.NoError(t, registry.RegisterDataPlaneFeature(registry.DataPlaneFeature{
			Name:       "unprunable-predicate",
			ConfigPath: "vpn ipsec",
			Why:        "cannot be pruned without DisablePaths",
			Planes:     []string{"kernel"},
			InUseKind:  registry.DataPlaneInUsePredicate,
			InUse: func(map[string]any) registry.DataPlaneFeatureUse {
				return registry.DataPlaneFeatureUse{InUse: true}
			},
		}))

		const cfg = `
interface {
	backend vpp;
}
`
		_, err := LoadConfigForStart(cfg, "test.conf", nil)
		require.Error(t, err, "re-check must fail closed when feature stays active after exclusion")
		require.Contains(t, err.Error(), "vpn ipsec")
	})
}
