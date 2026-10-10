package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

const twoAttachedProcessesConfig = `plugin {
	external alpha {
		run ./alpha.py
	}
	external beta {
		run ./beta.py
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
		attach process alpha {
			receive [ state ]
		}
		attach process beta {
			send [ update ]
		}
	}
}
`

// TestLoadEditCommitKeepsBothAttachedProcesses drives the round trip AC-13
// names: a peer attaches two processes, the config is loaded, edited and
// committed through the editor, and both attachments survive with their own
// name and their own body.
//
// VALIDATES: AC-13.
// PREVENTS: R-13 — the two-word keyword collapsing every attachment of a peer
// onto one key, which drops all but the first.
func TestLoadEditCommitKeepsBothAttachedProcesses(t *testing.T) {
	configPath := writeTestConfig(t, twoAttachedProcessesConfig)

	store := newTestTreeStore(t, configPath)
	ed, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck // test cleanup

	ed.SetSession(NewEditSession("thomas", "local"))

	// Both attachments are in the loaded tree, each keyed by its own name.
	loaded := ed.WorkingContent()
	assert.Contains(t, loaded, "alpha", "the first attachment must load")
	assert.Contains(t, loaded, "beta", "the second attachment must load")

	require.NoError(t, ed.SetValue([]string{"bgp"}, "router-id", "9.9.9.9"))

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)

	data, err := store.ReadFile(configPath) //nolint:gosec // test-owned temp path
	require.NoError(t, err)
	committed := string(data)

	assert.Contains(t, committed, "bgp peer peer1 attach process alpha receive state",
		"the first attachment lost its body:\n%s", committed)
	assert.Contains(t, committed, "bgp peer peer1 attach process beta send update",
		"the second attachment lost its body:\n%s", committed)
	assert.Contains(t, committed, "9.9.9.9", "the edit must reach the committed file")
}

// TestMergeConfigsKeepsEveryAttachedProcess covers the load merge of a peer
// whose ze:flatten attach list gains two entries: a fragment naming two
// attachments must not lose one, nor drop the one the peer already holds.
func TestMergeConfigsKeepsEveryAttachedProcess(t *testing.T) {
	ed, _, _ := newLoadSessionEditor(t, validBGPConfig)
	ed.SetSession(nil)
	require.NoError(t, ed.LoadMerge(nil, parseLoadInput(t, ed, `bgp { peer peer1 {
	attach process alpha {
		receive [ state ]
	}
} }`)))
	merge := `bgp { peer peer1 {
	attach process beta {
		send [ update ]
	}
	attach process gamma {
		receive [ update ]
	}
} }`
	require.NoError(t, ed.LoadMerge(nil, parseLoadInput(t, ed, merge)))
	out := config.Serialize(ed.tree, ed.schema)
	for _, want := range []string{"attach process alpha", "attach process beta", "attach process gamma"} {
		assert.Equal(t, 1, strings.Count(out, want), "%s survives exactly once in:\n%s", want, out)
	}
}
