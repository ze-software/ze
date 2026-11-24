// Design: docs/guide/config-editor.md — live committed baseline for compare.
// Related: model_commands_show.go.

package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestCompareCommittedUsesLiveStorageBaseline verifies that explicit
// compare committed, compare confirmed, show | compare | format config, and
// show confirmed read the current committed configuration from storage rather
// than a stale in-memory OriginalContent snapshot.
//
// VALIDATES: An external commit promoted into storage immediately updates the
// baseline for compare committed, compare confirmed, format config compare,
// and show confirmed.
// PREVENTS: Stale OriginalContent hiding external commits or reporting
// already-committed changes as pending.
func TestCompareCommittedUsesLiveStorageBaseline(t *testing.T) {
	const initial = `bgp {
	router-id 1.2.3.4
	session {
		asn {
			local 65000
		}
	}
}
`
	const externallyCommitted = `bgp {
	router-id 10.20.30.40
	session {
		asn {
			local 65000
		}
	}
}
`

	ed, _, path := newLoadSessionEditor(t, initial)
	m, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, m.Close()) })

	// Another user/session commits a new router-id directly to storage.
	_, err = storage.WriteCandidateVersion(ed.store, path, []byte(externallyCommitted), time.Now())
	require.NoError(t, err)
	require.NoError(t, storage.PromoteCandidate(ed.store, path))

	// Local session makes an unrelated edit; its in-memory originalContent is still 1.2.3.4.
	require.NoError(t, ed.SetValue([]string{"bgp", "session", "asn"}, "local", "65099"))

	// show confirmed must reflect the live committed router-id.
	confirmedRes, err := m.cmdShow([]string{srcConfirmed})
	require.NoError(t, err)
	require.NotNil(t, confirmedRes.configView)
	assert.Contains(t, confirmedRes.configView.content, "10.20.30.40")
	assert.NotContains(t, confirmedRes.configView.content, "1.2.3.4")

	// Explicit compare committed and compare confirmed must diff against 10.20.30.40.
	for _, target := range []string{"committed", srcConfirmed} {
		t.Run("compare "+target, func(t *testing.T) {
			res, cmpErr := m.cmdShowPipe(nil, []PipeFilter{{Type: cmdCompare, Arg: target}})
			require.NoError(t, cmpErr)
			require.NotNil(t, res.configView)
			assert.Contains(t, res.configView.originalContent, "10.20.30.40",
				"baseline must come from live storage, not stale OriginalContent")
			assert.Contains(t, res.configView.content, "65099")
		})
	}

	// Explicit format config pipe also uses the tree compare baseline.
	fmtRes, err := m.cmdShowPipe(nil, []PipeFilter{
		{Type: cmdCompare, Arg: ""},
		{Type: cmdFormat, Arg: fmtConfig},
	})
	require.NoError(t, err)
	require.NotNil(t, fmtRes.configView)
	assert.Contains(t, fmtRes.configView.originalContent, "10.20.30.40")
	assert.Contains(t, fmtRes.configView.content, "65099")

	// When local candidate matches the external commit on router-id, router-id has no diff.
	require.NoError(t, ed.SetValue([]string{"bgp"}, "router-id", "10.20.30.40"))
	sameRes, err := m.cmdShowPipe(nil, []PipeFilter{{Type: cmdCompare, Arg: "committed"}})
	require.NoError(t, err)
	require.NotNil(t, sameRes.configView)
	assert.NotContains(t, sameRes.configView.originalContent, "1.2.3.4")
	assert.NotContains(t, sameRes.configView.content, "10.20.30.40",
		"router-id matching live committed config must be pruned as unchanged")
	assert.Contains(t, sameRes.configView.content, "65099")
}
