package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	ed, err := NewEditor(configPath)
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

	data, err := os.ReadFile(configPath) //nolint:gosec // test-owned temp path
	require.NoError(t, err)
	committed := string(data)

	assert.Contains(t, committed, "bgp peer peer1 attach process alpha receive state",
		"the first attachment lost its body:\n%s", committed)
	assert.Contains(t, committed, "bgp peer peer1 attach process beta send update",
		"the second attachment lost its body:\n%s", committed)
	assert.Contains(t, committed, "9.9.9.9", "the edit must reach the committed file")
}

// TestExtractConfigKeyKeysFlattenedBlock pins the producing function R-13 names.
// A ze:flatten container writes its own name in front of the block keyword, and
// a key built from the first word alone is the same for every attachment.
//
// VALIDATES: R-13's mitigation, at the function that builds the key.
// PREVENTS: a load merge treating a peer's second attachment as a duplicate.
func TestExtractConfigKeyKeysFlattenedBlock(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"attach process alpha {", "attach process alpha"},
		{"\tattach process beta {", "attach process beta"},
		{"peer 1.1.1.1 {", "peer 1.1.1.1"},
		{"process alpha {", "process alpha"},
		{"router-id 1.2.3.4;", "router-id"},
		{"local ip 1.2.3.4;", "local"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, extractConfigKey(c.line), "line %q", c.line)
	}

	// The keys of two attachments on one peer must differ, or a merge drops one.
	first := extractConfigKey("attach process alpha {")
	second := extractConfigKey("attach process beta {")
	assert.NotEqual(t, first, second, "two attachments must not share a key")
}

// TestMergeConfigsKeepsEveryAttachedProcess covers the merge path that reads
// the key: a fragment naming two attachments must not lose one to duplicate
// detection.
func TestMergeConfigsKeepsEveryAttachedProcess(t *testing.T) {
	current := `peer peer1 {
	attach process alpha {
		receive [ state ]
	}
}
`
	merge := `peer peer1 {
	attach process beta {
		send [ update ]
	}
	attach process gamma {
		receive [ update ]
	}
}
`
	out := mergeConfigs(current, merge)
	for _, want := range []string{"attach process alpha", "attach process beta", "attach process gamma"} {
		assert.Equal(t, 1, strings.Count(out, want), "%s survives exactly once in:\n%s", want, out)
	}
}
