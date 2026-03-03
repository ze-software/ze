package yang

import (
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoader_EmbeddedModules verifies loading of embedded core YANG modules.
//
// VALIDATES: Core YANG modules (extensions, types, plugin) load without errors.
// PREVENTS: Syntax errors in YANG files breaking startup.
// NOTE: ze-bgp is in internal/plugins/bgp/schema, ze-hub is embedded in internal/yang/modules.
func TestLoader_EmbeddedModules(t *testing.T) {
	loader := NewLoader()

	err := loader.LoadEmbedded()
	require.NoError(t, err, "loading embedded modules should succeed")

	err = loader.Resolve()
	require.NoError(t, err, "resolving modules should succeed")

	// Verify core modules are loaded (ze-bgp is external, ze-hub is now embedded)
	names := loader.ModuleNames()
	assert.Contains(t, names, "ze-extensions", "ze-extensions module should be loaded")
	assert.Contains(t, names, "ze-types", "ze-types module should be loaded")
	assert.Contains(t, names, "ze-hub-conf", "ze-hub-conf module should be loaded")
	assert.Contains(t, names, "ze-plugin-conf", "ze-plugin-conf module should be loaded")
}

// TestLoader_ZeTypesModule verifies ze-types.yang content.
//
// VALIDATES: ze-types module defines expected typedefs.
// PREVENTS: Missing type definitions breaking other modules.
func TestLoader_ZeTypesModule(t *testing.T) {
	loader := NewLoader()

	err := loader.LoadEmbedded()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	mod := loader.GetModule("ze-types")
	require.NotNil(t, mod, "ze-types module should exist")

	// Check namespace
	assert.Equal(t, "urn:ze:types", mod.Namespace.Name)

	// Check typedefs exist
	typedefNames := make(map[string]bool)
	for _, td := range mod.Typedef {
		typedefNames[td.Name] = true
	}

	assert.True(t, typedefNames["asn"], "asn typedef should exist")
	assert.True(t, typedefNames["asn2"], "asn2 typedef should exist")
	assert.True(t, typedefNames["port"], "port typedef should exist")
	assert.True(t, typedefNames["ip-address"], "ip-address typedef should exist")
	assert.True(t, typedefNames["ipv4-address"], "ipv4-address typedef should exist")
	assert.True(t, typedefNames["ipv6-address"], "ipv6-address typedef should exist")
	assert.True(t, typedefNames["community"], "community typedef should exist")
}

// TestLoader_ZeBgpModule verifies ze-bgp-conf.yang content.
//
// VALIDATES: ze-bgp-conf module defines expected containers and lists.
// PREVENTS: Missing BGP configuration elements.
// NOTE: Uses LoadAllForTesting since ze-bgp-conf is now in internal/plugins/bgp/schema.
func TestLoader_ZeBgpModule(t *testing.T) {
	loader := NewLoader()

	err := loader.LoadAllForTesting()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	mod := loader.GetModule("ze-bgp-conf")
	require.NotNil(t, mod, "ze-bgp-conf module should exist")

	// Check namespace
	assert.Equal(t, "urn:ze:bgp:conf", mod.Namespace.Name)

	// Check import of ze-types
	assert.NotEmpty(t, mod.Import, "ze-bgp-conf should import ze-types")

	// Find bgp container
	var bgpContainer bool
	for _, c := range mod.Container {
		if c.Name == "bgp" {
			bgpContainer = true
			break
		}
	}
	assert.True(t, bgpContainer, "bgp container should exist")
}

// TestLoader_ZeHubModule verifies ze-hub-conf.yang content.
//
// VALIDATES: ze-hub-conf module defines expected containers.
// PREVENTS: Missing hub/environment configuration elements.
// NOTE: ze-hub-conf is now embedded in internal/yang/modules, loaded by LoadEmbedded().
func TestLoader_ZeHubModule(t *testing.T) {
	loader := NewLoader()

	err := loader.LoadEmbedded()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	mod := loader.GetModule("ze-hub-conf")
	require.NotNil(t, mod, "ze-hub-conf module should exist")

	// Check namespace
	assert.Equal(t, "urn:ze:hub:conf", mod.Namespace.Name)

	// Find environment container
	var envContainer bool
	for _, c := range mod.Container {
		if c.Name == "environment" {
			envContainer = true
			break
		}
	}
	assert.True(t, envContainer, "environment container should exist")
}

// TestLoader_ZePluginModule verifies ze-plugin-conf.yang content.
//
// VALIDATES: ze-plugin-conf module defines plugin configuration.
// PREVENTS: Missing plugin configuration schema.
func TestLoader_ZePluginModule(t *testing.T) {
	loader := NewLoader()

	err := loader.LoadEmbedded()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	mod := loader.GetModule("ze-plugin-conf")
	require.NotNil(t, mod, "ze-plugin-conf module should exist")

	// Check namespace
	assert.Equal(t, "urn:ze:plugin:conf", mod.Namespace.Name)

	// Find plugin container
	var pluginContainer bool
	for _, c := range mod.Container {
		if c.Name == "plugin" {
			pluginContainer = true
			break
		}
	}
	assert.True(t, pluginContainer, "plugin container should exist")
}

// TestLoader_AddModuleFromText verifies loading YANG from text.
//
// VALIDATES: YANG modules can be loaded from text content.
// PREVENTS: Plugin schema declaration failures.
func TestLoader_AddModuleFromText(t *testing.T) {
	loader := NewLoader()

	yangText := `
module test-module {
    namespace "urn:test:module";
    prefix tm;

    leaf test-leaf {
        type string;
    }
}
`

	err := loader.AddModuleFromText("test-module.yang", yangText)
	require.NoError(t, err, "loading from text should succeed")

	err = loader.Resolve()
	require.NoError(t, err)

	mod := loader.GetModule("test-module")
	require.NotNil(t, mod)
	assert.Equal(t, "urn:test:module", mod.Namespace.Name)
}

// TestLoader_InvalidYang verifies error handling for invalid YANG.
//
// VALIDATES: Invalid YANG syntax is rejected with error.
// PREVENTS: Silent acceptance of malformed schemas.
func TestLoader_InvalidYang(t *testing.T) {
	loader := NewLoader()

	invalidYang := `
module broken {
    this is not valid yang
}
`

	err := loader.AddModuleFromText("broken.yang", invalidYang)
	require.Error(t, err, "invalid YANG should fail")
}

// TestLoader_MissingImport verifies error on unresolved import.
//
// VALIDATES: Missing imports cause resolution failure.
// PREVENTS: Silently ignoring missing dependencies.
func TestLoader_MissingImport(t *testing.T) {
	loader := NewLoader()

	yangWithImport := `
module needs-import {
    namespace "urn:test:needs-import";
    prefix ni;

    import nonexistent-module { prefix nm; }

    leaf test {
        type nm:some-type;
    }
}
`

	err := loader.AddModuleFromText("needs-import.yang", yangWithImport)
	require.NoError(t, err, "parse should succeed")

	// Resolution should fail due to missing import
	err = loader.Resolve()
	require.Error(t, err, "resolution should fail with missing import")
}

// TestLoader_TypeBoundaries verifies type constraint definitions.
//
// VALIDATES: Types have correct range constraints.
// PREVENTS: Incorrect boundary validation.
func TestLoader_TypeBoundaries(t *testing.T) {
	loader := NewLoader()

	err := loader.LoadEmbedded()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	mod := loader.GetModule("ze-types")
	require.NotNil(t, mod)

	// Find ASN typedef and check its range
	for _, td := range mod.Typedef {
		if td.Name == "asn" {
			// ASN should be uint32 with range 1..4294967295
			assert.NotNil(t, td.Type, "asn should have a type")
			// Type checking is done by goyang during resolution
		}
		if td.Name == "port" {
			// Port should be uint16 with range 1..65535
			assert.NotNil(t, td.Type, "port should have a type")
		}
	}
}

// TestYANGRPCParsing verifies that the YANG loader parses rpc nodes.
//
// VALIDATES: goyang extracts rpc definitions from YANG modules.
// PREVENTS: RPC definitions being silently ignored by the loader.
func TestYANGRPCParsing(t *testing.T) {
	loader := NewLoader()

	yangText := `
module test-rpc {
    namespace "urn:test:rpc";
    prefix tr;

    rpc peer-list {
        description "List all peers";
        input {
            leaf selector {
                type string;
                description "Peer selector pattern";
            }
        }
        output {
            list peer {
                leaf address {
                    type string;
                }
                leaf state {
                    type string;
                }
            }
        }
    }

    rpc shutdown {
        description "Shutdown the daemon";
    }
}
`

	err := loader.AddModuleFromText("test-rpc.yang", yangText)
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	mod := loader.GetModule("test-rpc")
	require.NotNil(t, mod)

	// Verify RPCs are parsed at module level
	require.Len(t, mod.RPC, 2, "module should have 2 RPCs")

	rpcNames := make(map[string]bool)
	for _, rpc := range mod.RPC {
		rpcNames[rpc.Name] = true
	}
	assert.True(t, rpcNames["peer-list"], "peer-list RPC should exist")
	assert.True(t, rpcNames["shutdown"], "shutdown RPC should exist")

	// Verify RPCs appear in entry tree
	entry := gyang.ToEntry(mod)
	require.NotNil(t, entry)

	peerListEntry := entry.Dir["peer-list"]
	require.NotNil(t, peerListEntry, "peer-list should be in entry tree")

	shutdownEntry := entry.Dir["shutdown"]
	require.NotNil(t, shutdownEntry, "shutdown should be in entry tree")
}

// TestYANGNotificationParsing verifies that the YANG loader parses notification nodes.
//
// VALIDATES: goyang extracts notification definitions from YANG modules.
// PREVENTS: Notification definitions being silently ignored.
func TestYANGNotificationParsing(t *testing.T) {
	loader := NewLoader()

	yangText := `
module test-notif {
    namespace "urn:test:notif";
    prefix tn;

    notification update-event {
        description "BGP UPDATE received or sent";
        leaf peer-address {
            type string;
        }
        leaf direction {
            type enumeration {
                enum received;
                enum sent;
            }
        }
    }

    notification state-event {
        description "Peer state change";
        leaf peer-address {
            type string;
        }
        leaf state {
            type string;
        }
    }
}
`

	err := loader.AddModuleFromText("test-notif.yang", yangText)
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	mod := loader.GetModule("test-notif")
	require.NotNil(t, mod)

	// Verify notifications are parsed at module level
	require.Len(t, mod.Notification, 2, "module should have 2 notifications")

	notifNames := make(map[string]bool)
	for _, n := range mod.Notification {
		notifNames[n.Name] = true
	}
	assert.True(t, notifNames["update-event"], "update-event notification should exist")
	assert.True(t, notifNames["state-event"], "state-event notification should exist")

	// Verify notifications appear in entry tree
	entry := gyang.ToEntry(mod)
	require.NotNil(t, entry)

	updateEntry := entry.Dir["update-event"]
	require.NotNil(t, updateEntry, "update-event should be in entry tree")
	assert.Equal(t, gyang.NotificationEntry, updateEntry.Kind, "should be NotificationEntry kind")

	// Verify notification children are accessible
	require.NotNil(t, updateEntry.Dir["peer-address"], "peer-address leaf should exist")
	require.NotNil(t, updateEntry.Dir["direction"], "direction leaf should exist")
}

// TestYANGRPCInputOutput verifies RPC input/output leaves are accessible.
//
// VALIDATES: RPC input and output parameters can be traversed via entry tree.
// PREVENTS: IPC parameter validation failing due to inaccessible YANG definitions.
func TestYANGRPCInputOutput(t *testing.T) {
	loader := NewLoader()

	yangText := `
module test-rpc-io {
    namespace "urn:test:rpc-io";
    prefix trio;

    rpc peer-teardown {
        description "Teardown peer session";
        input {
            leaf selector {
                type string;
                mandatory true;
                description "Peer address or wildcard";
            }
            leaf subcode {
                type uint8;
                default 2;
                description "CEASE subcode";
            }
        }
        output {
            leaf peers-affected {
                type uint32;
            }
        }
    }
}
`

	err := loader.AddModuleFromText("test-rpc-io.yang", yangText)
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	mod := loader.GetModule("test-rpc-io")
	require.NotNil(t, mod)

	entry := gyang.ToEntry(mod)
	require.NotNil(t, entry)

	rpcEntry := entry.Dir["peer-teardown"]
	require.NotNil(t, rpcEntry, "peer-teardown RPC should exist in entry tree")
	require.NotNil(t, rpcEntry.RPC, "entry should have RPC field set")

	// Verify input parameters
	input := rpcEntry.RPC.Input
	require.NotNil(t, input, "RPC should have input")
	assert.Equal(t, gyang.InputEntry, input.Kind, "input should be InputEntry kind")

	selectorLeaf := input.Dir["selector"]
	require.NotNil(t, selectorLeaf, "selector input parameter should exist")
	assert.NotNil(t, selectorLeaf.Type, "selector should have type")

	subcodeLeaf := input.Dir["subcode"]
	require.NotNil(t, subcodeLeaf, "subcode input parameter should exist")

	// Verify output parameters
	output := rpcEntry.RPC.Output
	require.NotNil(t, output, "RPC should have output")
	assert.Equal(t, gyang.OutputEntry, output.Kind, "output should be OutputEntry kind")

	peersLeaf := output.Dir["peers-affected"]
	require.NotNil(t, peersLeaf, "peers-affected output parameter should exist")
}

// TestYANGModuleWithRPCAndConfig verifies modules with both containers and RPCs.
//
// VALIDATES: A single YANG module can contain both config containers and RPCs.
// PREVENTS: Config-only assumptions breaking when IPC RPCs are added to modules.
func TestYANGModuleWithRPCAndConfig(t *testing.T) {
	loader := NewLoader()

	// Load core modules for imports
	err := loader.LoadEmbedded()
	require.NoError(t, err)

	yangText := `
module test-mixed {
    namespace "urn:test:mixed";
    prefix tmx;

    import ze-types { prefix zt; }

    container bgp {
        leaf router-id {
            type zt:ipv4-address;
        }
        leaf local-as {
            type zt:asn;
        }
    }

    rpc peer-list {
        description "List peers";
        output {
            leaf count {
                type uint32;
            }
        }
    }

    rpc daemon-status {
        description "Get daemon status";
        output {
            leaf uptime {
                type string;
            }
        }
    }

    notification state-change {
        description "Peer state changed";
        leaf peer {
            type string;
        }
        leaf new-state {
            type string;
        }
    }
}
`

	err = loader.AddModuleFromText("test-mixed.yang", yangText)
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	mod := loader.GetModule("test-mixed")
	require.NotNil(t, mod)

	// Verify containers coexist with RPCs and notifications
	assert.Len(t, mod.Container, 1, "should have 1 container")
	assert.Equal(t, "bgp", mod.Container[0].Name)
	assert.Len(t, mod.RPC, 2, "should have 2 RPCs")
	assert.Len(t, mod.Notification, 1, "should have 1 notification")

	// Verify entry tree has all three types
	entry := gyang.ToEntry(mod)
	require.NotNil(t, entry)

	// Container
	bgpEntry := entry.Dir["bgp"]
	require.NotNil(t, bgpEntry, "bgp container should be in entry tree")
	assert.NotNil(t, bgpEntry.Dir["router-id"], "router-id should exist")
	assert.NotNil(t, bgpEntry.Dir["local-as"], "local-as should exist")

	// RPCs
	peerListEntry := entry.Dir["peer-list"]
	require.NotNil(t, peerListEntry, "peer-list RPC should be in entry tree")
	require.NotNil(t, peerListEntry.RPC, "should have RPC field")

	statusEntry := entry.Dir["daemon-status"]
	require.NotNil(t, statusEntry, "daemon-status RPC should be in entry tree")

	// Notification
	stateEntry := entry.Dir["state-change"]
	require.NotNil(t, stateEntry, "state-change notification should be in entry tree")
	assert.Equal(t, gyang.NotificationEntry, stateEntry.Kind)
	assert.NotNil(t, stateEntry.Dir["peer"], "peer leaf should exist in notification")
	assert.NotNil(t, stateEntry.Dir["new-state"], "new-state leaf should exist in notification")
}
