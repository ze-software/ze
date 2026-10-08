package yang

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// publishCmdModule points two nodes at the fixture -api module's socket-list
// rpc under two wire methods, and points no node at socket-clear.
const publishCmdModule = `
module ze-fixture-cmd {
    namespace "urn:ze:fixture:cmd";
    prefix zefc;
    import ze-extensions { prefix ze; }

    container show {
        config false;
        container sockets {
            config false;
            ze:command "ze-show:sockets";
            ze:rpc "ze-fixture-api:socket-list";
        }
        container socket-table {
            config false;
            ze:command "ze-fixture:socket-table";
            ze:rpc "ze-fixture-api:socket-list";
        }
        container plain {
            config false;
            ze:command "ze-show:plain";
        }
    }
}
`

// publishLoader loads the fixture -api module and the given -cmd module text.
func publishLoader(t *testing.T, cmdModule string) *Loader {
	t.Helper()
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.AddModuleFromText("ze-fixture-api.yang", rpcHelpModule))
	require.NoError(t, loader.AddModuleFromText("ze-fixture-cmd.yang", cmdModule))
	require.NoError(t, loader.Resolve())
	return loader
}

// TestPublishedRPCsTakeTheMethodOfThePointingNode proves an rpc is published
// under the wire method of each ze:command node that points at it, and under
// no method built from its module's file name.
//
// VALIDATES: AC-8, the node is the one declaration of the method; AC-9, an rpc
// no node points at and that declares no ze:method is named as unnamed.
// PREVENTS: `ze-fixture-api` publishing `ze-fixture:socket-list`, a name no node
// declares and no handler answers.
func TestPublishedRPCsTakeTheMethodOfThePointingNode(t *testing.T) {
	pub, err := PublishedRPCs(publishLoader(t, publishCmdModule))
	require.NoError(t, err)

	var methods []string
	for _, rpc := range pub.Commands {
		if rpc.Module == "ze-fixture-api" {
			methods = append(methods, rpc.WireMethod)
			assert.Equal(t, "socket-list", rpc.Name)
			assert.Equal(t, "List the open sockets.", rpc.ShortHelp, "the pointed rpc keeps its own texts")
		}
	}
	assert.Equal(t, []string{"ze-fixture:socket-table", "ze-show:sockets"}, methods)

	var unnamed []string
	for _, rpc := range pub.Unnamed {
		if rpc.Module == "ze-fixture-api" {
			unnamed = append(unnamed, rpc.Name)
		}
	}
	assert.Equal(t, []string{"socket-clear"}, unnamed, "an rpc no node points at is published under no name")
}

// TestPublishedRPCsSetAsideAPointerAtAnUnlinkedModule proves a pointer at a
// module this process did not load is reported as unlinked, not refused, and
// publishes nothing.
//
// VALIDATES: a binary that links a -cmd module without the -api module it
// points at still builds its schema registry.
// PREVENTS: `ze schema` failing in a build that leaves one component out.
func TestPublishedRPCsSetAsideAPointerAtAnUnlinkedModule(t *testing.T) {
	module := strings.Replace(publishCmdModule, `ze:rpc "ze-fixture-api:socket-list";`, `ze:rpc "ze-absent-api:socket-list";`, 1)
	pub, err := PublishedRPCs(publishLoader(t, module))
	require.NoError(t, err)
	assert.Contains(t, pub.Unlinked, "ze-show:sockets -> ze-absent-api:socket-list")
}

// TestPublishedRPCsRefuseABrokenPointer proves each pointer the schema cannot
// honor is refused by name rather than dropped.
//
// VALIDATES: a pointer to an rpc no module declares, one method pointing at
// two rpcs, and a target with no module are each an ErrRPCPointer.
// PREVENTS: a mistyped ze:rpc publishing nothing in silence.
func TestPublishedRPCsRefuseABrokenPointer(t *testing.T) {
	cases := map[string]string{
		"missing rpc": `container a { config false; ze:command "ze-show:a"; ze:rpc "ze-fixture-api:socket-gone"; }`,
		"two targets": `container a { config false; ze:command "ze-show:a"; ze:rpc "ze-fixture-api:socket-list"; }
        container b { config false; ze:command "ze-show:a"; ze:rpc "ze-fixture-api:socket-clear"; }`,
		"malformed":  `container a { config false; ze:command "ze-show:a"; ze:rpc "socket-list"; }`,
		"no command": `container a { config false; ze:rpc "ze-fixture-api:socket-list"; }`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			module := "module ze-fixture-cmd { namespace \"urn:ze:fixture:cmd\"; prefix zefc; import ze-extensions { prefix ze; } container show { config false; " + body + " } }"
			_, err := PublishedRPCs(publishLoader(t, module))
			require.ErrorIs(t, err, ErrRPCPointer)
		})
	}
}

// TestExtractRPCsNonexistentModule verifies graceful handling of missing modules.
//
// VALIDATES: Returns empty slice for nonexistent module.
// PREVENTS: Nil pointer panic when module doesn't exist.
func TestExtractRPCsNonexistentModule(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.Resolve())

	rpcs := ExtractRPCs(loader, "nonexistent-module")
	assert.Empty(t, rpcs, "should return empty for nonexistent module")
}

// TestExtractNotificationsNonexistentModule verifies graceful handling of missing modules.
//
// VALIDATES: Returns empty slice for nonexistent module.
// PREVENTS: Nil pointer panic when module doesn't exist.
func TestExtractNotificationsNonexistentModule(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.Resolve())

	notifs := ExtractNotifications(loader, "nonexistent-module")
	assert.Empty(t, notifs, "should return empty for nonexistent module")
}

// rpcHelpModule declares two RPCs. The first carries both help texts, and its
// description argument spans three lines, because a long explanation is what
// the statement carries and goyang has to return it whole. The second carries
// a ze:help summary alone, which is what every unconverted RPC looks like.
const rpcHelpModule = `
module ze-fixture-api {
    namespace "urn:ze:fixture:api";
    prefix zefa;
    import ze-extensions { prefix ze; }

    rpc socket-list {
        ze:help "List the open sockets.";
        description "One row is written for each socket the daemon holds open.

                 The state column names the TCP state.";
        output {
            leaf count {
                type uint32;
                ze:help "How many sockets are open.";
            }
        }
    }

    rpc socket-clear {
        ze:help "Close every idle socket.";
    }
}
`

// TestRPCDescriptionCarriesSummaryAndHelp reads an RPC declaring both help
// texts and asserts each reaches its own field on the extracted metadata.
//
// VALIDATES: goyang exposes the extension statements of an rpc, so an RPC
// declares its one-line summary through the same ze:help the command tree uses,
// and the description carries the long explanation.
// PREVENTS: a second mechanism for the long form on the RPC side, and an RPC
// whose summary and explanation share one string, which is the state every
// renderer guesses its way out of (AC-16).
func TestRPCDescriptionCarriesSummaryAndHelp(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.AddModuleFromText("ze-fixture-api.yang", rpcHelpModule))
	require.NoError(t, loader.Resolve())

	rpcs := ExtractRPCs(loader, "ze-fixture-api")
	require.Len(t, rpcs, 2)

	byName := map[string]RPCMeta{}
	for _, rpc := range rpcs {
		byName[rpc.Name] = rpc
	}

	declared := byName["socket-list"]
	assert.Equal(t, "List the open sockets.", declared.ShortHelp)
	assert.Contains(t, declared.Description, "One row is written for each socket")
	assert.Contains(t, declared.Description, "The state column names the TCP state.")
	assert.Contains(t, declared.Description, "\n", "a long explanation keeps the line breaks its author wrote")
	assert.NotContains(t, declared.ShortHelp, declared.Description, "neither field is derived from the other")

	silent := byName["socket-clear"]
	assert.Equal(t, "Close every idle socket.", silent.ShortHelp)
	assert.Empty(t, silent.Description, "no description statement means no long explanation")
}

// TestExtractRPCsCarriesBothLeafTexts proves an rpc input leaf, an rpc output
// leaf and a notification leaf each carry LeafMeta.ShortHelp from ze:help and
// LeafMeta.Description from description, and that a leaf declaring one text
// leaves the other empty.
func TestExtractRPCsCarriesBothLeafTexts(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())

	module := `
module ze-leaftexts-api {
    namespace "urn:test:leaftexts";
    prefix lt;

    import ze-extensions { prefix ze; }

    rpc socket-open {
        ze:help "Open a socket.";
        input {
            leaf port {
                type uint16;
                mandatory true;
                ze:help "The TCP port to listen on";
                description "The port the socket binds.";
            }
        }
        output {
            leaf fd {
                type int32;
                ze:help "The descriptor of the open socket";
                description "A descriptor the caller closes when it is done.";
            }
        }
    }

    notification socket-closed {
        ze:help "A socket closed.";
        leaf reason {
            type string;
            ze:help "Why the socket closed";
        }
    }
}
`
	require.NoError(t, loader.AddModuleFromText("ze-leaftexts-api.yang", module))
	require.NoError(t, loader.Resolve())

	rpcs := ExtractRPCs(loader, "ze-leaftexts-api")
	require.Len(t, rpcs, 1)
	require.Len(t, rpcs[0].Input, 1)
	assert.Equal(t, "The TCP port to listen on", rpcs[0].Input[0].ShortHelp)
	assert.Equal(t, "The port the socket binds.", rpcs[0].Input[0].Description)
	require.Len(t, rpcs[0].Output, 1)
	assert.Equal(t, "The descriptor of the open socket", rpcs[0].Output[0].ShortHelp)
	assert.Equal(t, "A descriptor the caller closes when it is done.", rpcs[0].Output[0].Description)

	notifs := ExtractNotifications(loader, "ze-leaftexts-api")
	require.Len(t, notifs, 1)
	require.Len(t, notifs[0].Leaves, 1)
	assert.Equal(t, "Why the socket closed", notifs[0].Leaves[0].ShortHelp)
	assert.Empty(t, notifs[0].Leaves[0].Description, "no description statement means no explanation")
}

// TestRPCInputKeepsTheOrderTheModuleDeclared pins the order an RPC's
// parameters come back in.
//
// VALIDATES: the leaves arrive in source order, and a leaf reached through a
// `uses` arrives after them by name rather than wherever a map put it.
// PREVENTS: an order decided by Go's map seed. Entry.Dir is a map, and ranging
// it was the whole source of this order, so the same schema described its own
// arguments differently on each process: the MCP tool's parameter list and the
// generated command help both read it. It also made a test of the first
// parameter pass or fail on the toss of that seed, which is how this was found
// (cmd/ze/hub, TestBuildParamMetaCarriesBothLeafTexts).
//
// The four declared names are in neither sorted nor reverse-sorted order, so a
// map iteration agreeing with the assertion by chance is a 1-in-24 event, and
// the loop repeats it until that is not worth arguing about.
func TestRPCInputKeepsTheOrderTheModuleDeclared(t *testing.T) {
	const module = `module ze-order-api {
  namespace "urn:ze:order"; prefix zo;
  grouping shared { leaf alpha { type string; } }
  rpc show-order {
    input {
      leaf zulu { type string; mandatory true; }
      leaf mike { type uint16; }
      leaf bravo { type string; }
      uses shared;
    }
  }
}
`
	want := []string{"zulu", "mike", "bravo", "alpha"}
	for range 20 {
		loader := NewLoader()
		require.NoError(t, loader.AddModuleFromText("ze-order-api", module))
		require.NoError(t, loader.Resolve())

		rpcs := ExtractRPCs(loader, "ze-order-api")
		require.Len(t, rpcs, 1, "the module declares one rpc")

		got := make([]string, 0, len(rpcs[0].Input))
		for _, leaf := range rpcs[0].Input {
			got = append(got, leaf.Name)
		}
		require.Equal(t, want, got,
			"declared order first, then what only the entry tree holds, by name")
	}
}
