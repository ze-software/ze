package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

const twoBusEndpointsConfig = `bus {
	endpoint alpha {
		internal bgp
	}
	endpoint beta {
		internal static
	}
}
bgp {
	router-id 1.2.3.4
	session {
		asn {
			local 65000
		}
	}
	peer peer1 {
		connection {
			remote {
				ip 1.1.1.1
			}
		}
		session {
			asn {
				remote 65001
			}
		}
		timer { receive-hold-time 90; }
		bus {
			send {
				to [ /bus/endpoint/alpha ]
				allow [ bgp:state ]
			}
			receive {
				from [ /bus/endpoint/beta ]
				allow [ bgp:update ]
			}
		}
	}
}
`

// TestLoadEditCommitKeepsBothAttachedProcesses drives the round trip AC-13
// names: a peer narrows bus delivery to two endpoints, the config is loaded,
// edited and committed through the editor, and both endpoints and the peer bus
// rules survive with their own names and bodies.
//
// VALIDATES: AC-13.
// PREVENTS: editor load/edit/commit collapsing or dropping bus endpoints or
// peer bus delivery rules.
func TestLoadEditCommitKeepsBothAttachedProcesses(t *testing.T) {
	configPath := writeTestConfig(t, twoBusEndpointsConfig)

	store := newTestTreeStore(t, configPath)
	ed, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	ed.SetSession(NewEditSession("thomas", "local"))

	// Both endpoints and peer bus rules are in the loaded tree.
	loaded := ed.WorkingContent()
	assert.Contains(t, loaded, "alpha", "the first endpoint must load")
	assert.Contains(t, loaded, "beta", "the second endpoint must load")

	require.NoError(t, ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"))

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)

	data, err := store.ReadFile(configPath) //nolint:gosec // test-owned temp path
	require.NoError(t, err)
	committed := string(data)

	assert.Contains(t, committed, "bgp peer peer1 bus send to /bus/endpoint/alpha",
		"the peer send target lost its body:\n%s", committed)
	assert.Contains(t, committed, "bgp peer peer1 bus send allow bgp:state",
		"the peer send allow list lost its body:\n%s", committed)
	assert.Contains(t, committed, "bgp peer peer1 bus receive from /bus/endpoint/beta",
		"the peer receive source lost its body:\n%s", committed)
	assert.Contains(t, committed, "bgp peer peer1 bus receive allow bgp:update",
		"the peer receive allow list lost its body:\n%s", committed)
	assert.Contains(t, committed, "9.9.9.9", "the edit must reach the committed file")
}

// TestMergeConfigsKeepsEveryAttachedProcess covers the load merge of a bus
// endpoint list gaining two entries: a fragment naming two endpoints must not
// lose one, nor drop the endpoint the config already holds.
func TestMergeConfigsKeepsEveryAttachedProcess(t *testing.T) {
	ed, _, _ := newLoadSessionEditor(t, validBGPConfig)
	ed.SetSession(nil)
	require.NoError(t, ed.LoadMerge(nil, parseLoadInput(t, ed, `bus {
	endpoint alpha {
		external {
			run ./alpha.py
		}
	}
}`)))
	merge := `bus {
	endpoint beta {
		external {
			run ./beta.py
		}
	}
	endpoint gamma {
		external {
			run ./gamma.py
		}
	}
}`
	require.NoError(t, ed.LoadMerge(nil, parseLoadInput(t, ed, merge)))
	out := config.Serialize(ed.tree, ed.schema)
	for _, want := range []string{"endpoint alpha", "endpoint beta", "endpoint gamma"} {
		assert.Equal(t, 1, strings.Count(out, want), "%s survives exactly once in:\n%s", want, out)
	}
}
