// Design: docs/architecture/config/yang-config-design.md -- Layer 4, the BGP peer pipeline
package web

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
	configcli "github.com/ze-software/ze/internal/component/config/cli"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// commitTestConfig is a committed config carrying a bgp container, which is
// what makes runValidation reach the BGP peer pipeline at all.
const commitTestConfig = `bgp {
	router-id 1.2.3.4
	session {
		asn {
			local 65000
		}
	}
}
`

// validatingEditorFactory mirrors the production web wiring in
// cmd/ze/hub/editor_adapter.go (newEditorFactory): every editor the web hands
// out carries config/cli.ValidateContent as its pre-commit validator, and that
// is the one path by which the web reaches infra.ValidateBGPPeers.
func validatingEditorFactory() contract.EditorFactory {
	return func(storeAny any, configPath string) (contract.Editor, error) {
		store, ok := storeAny.(storage.Storage)
		if !ok {
			return nil, errors.New("expected storage.Storage")
		}
		ed, err := cli.NewEditorWithStorage(store, configPath)
		if err != nil {
			return nil, err
		}
		ed.SetPreCommitValidate(func(candidate string) error {
			return configcli.ValidateContent(candidate, configPath)
		})
		return &testEditorAdapter{ed: ed}, nil
	}
}

// installPeerValidator fills the two infra seams the BGP engine fills at init,
// so ValidateContent reaches the peer pipeline in a binary that links no engine.
// The tree resolver is stubbed because ResolveBGPTree fails closed on a bgp{}
// block with no resolver registered, which would refuse every config here
// before the peer validator was ever asked.
//
// Both seams are restored to nil, which is their starting value in this test
// binary: it does not link internal/component/bgp/config, so nothing else
// fills them.
func installPeerValidator(t *testing.T, validate infra.BGPPeerValidator) {
	t.Helper()
	infra.SetBGPTreeResolver(func(*config.Tree) (map[string]any, error) {
		return map[string]any{}, nil
	})
	infra.SetBGPPeerValidator(validate)
	t.Cleanup(func() {
		infra.SetBGPTreeResolver(nil)
		infra.SetBGPPeerValidator(nil)
	})
}

// refuseUnlessRouterID installs a peer validator that accepts exactly one
// router-id and refuses every other tree. The router-id is a stand-in for any
// peer-pipeline rule: what the test needs is a verdict that DIFFERS between the
// draft tree and the committed tree.
func refuseUnlessRouterID(t *testing.T, accepted string) {
	t.Helper()
	installPeerValidator(t, func(tree *config.Tree) error {
		id := ""
		if bgp := tree.GetContainer("bgp"); bgp != nil {
			id, _ = bgp.Get("router-id")
		}
		if id == accepted {
			return nil
		}
		return errors.New("peer peer1: refused, router-id " + id)
	})
}

// newCommitTestManager returns an EditorManager over a committed config on a
// filesystem store, wired exactly as the daemon wires it.
func newCommitTestManager(t *testing.T) (*EditorManager, string) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(commitTestConfig), 0o600))
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	mgr := NewEditorManager(storage.NewFilesystem(), configPath, schema,
		validatingEditorFactory(), testEditSessionFactory())
	return mgr, configPath
}

// TestWebCommitBlocksWhenThePeerPipelineRefuses drives the web commit entry
// point over a config the BGP peer pipeline refuses.
//
// VALIDATES: EditorManager.Commit returns the refusal as an error, which all
// three web commit surfaces render (handleCommitPost, handleCLICommit,
// executeTerminalCommit), and config.conf is untouched.
// PREVENTS: the web writing a config the daemon refuses and reporting the
// refusal one step later, at reload, about a file already written.
func TestWebCommitBlocksWhenThePeerPipelineRefuses(t *testing.T) {
	refuseUnlessRouterID(t, "9.9.9.9")
	mgr, configPath := newCommitTestManager(t)

	require.NoError(t, mgr.SetValue("bob", []string{"bgp", "session", "asn"}, "local", "65001"))
	_, err := mgr.Commit("bob")
	require.Error(t, err, "the refused config must not reach .conf")
	assert.Contains(t, err.Error(), "peer1", "the operator is told which peer")

	data, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.NotContains(t, string(data), "65001", "nothing was staged")
}

// TestWebCommitValidatesTheTreeItStagesNotTheDraft is the same refusal reached
// through the divergence between the two trees a commit handles.
//
// The editor validated inside SaveDraft, over the DRAFT tree: the shared draft
// base plus this session's entries. A commit writes a DIFFERENT tree: the
// COMMITTED file plus this session's entries. The moment another session has a
// saved draft the two disagree, so the check answered about a config that is
// never written while the config that IS written went unchecked.
//
// VALIDATES: the validated tree and the staged tree are the same tree.
// PREVENTS: a second operator's saved draft carrying another operator's commit
// past the peer pipeline and into config.conf.
func TestWebCommitValidatesTheTreeItStagesNotTheDraft(t *testing.T) {
	refuseUnlessRouterID(t, "9.9.9.9")
	mgr, configPath := newCommitTestManager(t)

	// Alice makes the config acceptable and saves a draft without committing.
	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "9.9.9.9"))
	require.NoError(t, mgr.SaveDraft("alice"))

	// Bob commits his own unrelated change. The committed file still carries
	// router-id 1.2.3.4, so the tree Bob stages is one the pipeline refuses.
	require.NoError(t, mgr.SetValue("bob", []string{"bgp", "session", "asn"}, "local", "65001"))
	_, err := mgr.Commit("bob")
	require.Error(t, err, "the staged tree is refused, so the commit is refused")
	assert.Contains(t, err.Error(), "1.2.3.4", "the refusal names the tree that was staged")

	data, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.NotContains(t, string(data), "65001", "nothing was staged")
}

// TestWebCommitCandidateValidatesTheTreeItStages is the other half of the pair.
//
// EditorManager.Commit takes CommitSession when no reload hook is installed and
// CommitSessionCandidate when one is, and the daemon always installs one
// (cmd/ze/hub/service_web.go). A guard on one branch and not the other leaves
// the deployed path unguarded.
//
// VALIDATES: the candidate path refuses the same staged tree, writes no
// candidate version, and never calls the reload hook.
// PREVENTS: the pair defect recorded in
// plan/journal/guard-added-to-one-half-of-a-pair.md.
func TestWebCommitCandidateValidatesTheTreeItStages(t *testing.T) {
	refuseUnlessRouterID(t, "9.9.9.9")
	mgr, configPath := newCommitTestManager(t)

	reloaded := false
	mgr.SetCommitHook(func() error {
		reloaded = true
		return nil
	})

	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "9.9.9.9"))
	require.NoError(t, mgr.SaveDraft("alice"))

	require.NoError(t, mgr.SetValue("bob", []string{"bgp", "session", "asn"}, "local", "65001"))
	_, err := mgr.Commit("bob")
	require.Error(t, err, "the staged candidate is refused")
	assert.Contains(t, err.Error(), "1.2.3.4", "the refusal names the tree that was staged")
	assert.False(t, reloaded, "the commit must not reach the daemon")

	_, _, hasCandidate, candErr := storage.ReadCandidateConfig(storage.NewFilesystem(), configPath)
	require.NoError(t, candErr)
	assert.False(t, hasCandidate, "no candidate version was staged")
}

// TestWebCommitPassesWhenThePeerPipelineAccepts is the positive half.
//
// VALIDATES: the check adds no refusal of its own; an accepted tree still
// commits and reaches config.conf.
// PREVENTS: every web commit blocking because a nil error was read as a
// failure, which would make the web editor unusable rather than merely late.
func TestWebCommitPassesWhenThePeerPipelineAccepts(t *testing.T) {
	asked := false
	installPeerValidator(t, func(*config.Tree) error {
		asked = true
		return nil
	})

	mgr, configPath := newCommitTestManager(t)
	require.NoError(t, mgr.SetValue("bob", []string{"bgp", "session", "asn"}, "local", "65001"))
	result, err := mgr.Commit("bob")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Conflicts)
	assert.True(t, asked, "the web commit consults the peer pipeline")

	data, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.True(t, strings.Contains(string(data), "65001"), "the accepted config is committed")
}
