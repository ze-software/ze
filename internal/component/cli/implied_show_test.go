// Design: docs/architecture/api/architecture.md -- editor show | implied pipe, plain-show notice, and scoping tests.
// Related: model_load.go -- ClassifyShowPipes and cmdShowPipe.
// Related: model_commands_show.go -- cmdShowImplied and attachImpliedPlainNotice.

package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
)

// TestClassifyShowPipesImplied verifies that ClassifyShowPipes parses bare
// `| implied`, combined `| implied | format config`, and `| implied | match`,
// rejects unexpected arguments to `| implied`, and exposes `implied` in pipe
// completion.
//
// VALIDATES: AC-23 (editor show | implied pipe classification and completion).
// PREVENTS: show | implied falling through as an unknown text filter.
func TestClassifyShowPipesImplied(t *testing.T) {
	bare, err := ClassifyShowPipes([]PipeFilter{{Type: cmdImplied}})
	require.NoError(t, err)
	assert.True(t, bare.Implied)
	assert.Equal(t, fmtTree, bare.Format)
	assert.Empty(t, bare.TextFilters)

	withSet, err := ClassifyShowPipes([]PipeFilter{
		{Type: cmdImplied},
		{Type: cmdFormat, Arg: fmtConfig},
	})
	require.NoError(t, err)
	assert.True(t, withSet.Implied)
	assert.Equal(t, fmtConfig, withSet.Format)
	assert.True(t, withSet.FormatExplicit)
	assert.Empty(t, withSet.TextFilters)

	withMatch, err := ClassifyShowPipes([]PipeFilter{
		{Type: cmdImplied},
		{Type: cmdMatch, Arg: "collector"},
	})
	require.NoError(t, err)
	assert.True(t, withMatch.Implied)
	assert.Equal(t, fmtTree, withMatch.Format)
	require.Len(t, withMatch.TextFilters, 1)
	assert.Equal(t, cmdMatch, withMatch.TextFilters[0].Type)
	assert.Equal(t, "collector", withMatch.TextFilters[0].Arg)

	_, err = ClassifyShowPipes([]PipeFilter{{Type: cmdImplied, Arg: "unexpected"}})
	require.Error(t, err, "show | implied takes no arguments")

	found := false
	for _, c := range showPipeFilters {
		if c.Text == cmdImplied {
			found = true
			break
		}
	}
	assert.True(t, found, "showPipeFilters must offer implied for tab completion")
}

// TestCmdShowImpliedTreeSetAndMatch verifies editor `show | implied`,
// `show | implied | format config`, `show | implied | match <pat>`, alternate
// sources, context scoping, and ExecuteConfigCommand parity.
//
// VALIDATES: AC-23 (editor show | implied renders written and implied entries
// with origins in tree and set formats and composes with text filters).
// PREVENTS: implied blocks missing origin comments, emitting ## at column 0,
// or bypassing text filters.
func TestCmdShowImpliedTreeSetAndMatch(t *testing.T) {
	restore := config.ResetImpliedProvidersForTest()
	t.Cleanup(restore)

	config.RegisterImpliedBusProvider(func(tree *config.Tree, _ config.ImpliedOptions) ([]config.ImpliedEndpoint, []config.ImpliedParticipant, error) {
		if tree == nil || tree.GetContainer("bgp") == nil {
			return nil, nil, nil
		}
		return []config.ImpliedEndpoint{
				{
					Name:           "synthetic-ep",
					CompiledPlugin: "synthetic-ep",
					ReasonKind:     "config-path",
					ReasonSource:   "bgp",
					ImpliedBy:      "config section bgp",
				},
				{
					Name:           "other-ep",
					CompiledPlugin: "other-ep",
					ReasonKind:     "config-path",
					ReasonSource:   "system/ntp",
					ImpliedBy:      "config section system/ntp",
				},
			},
			[]config.ImpliedParticipant{
				{
					Path:         "/bus/endpoint/synthetic-ep",
					EndpointName: "synthetic-ep",
					Receive: []config.ImpliedDirectionRule{
						{
							Message: "demo:event",
							Allowed: true,
							Origin:  config.ImpliedOriginRegistered,
							Source:  "registration of synthetic-ep",
							Targets: []string{"/bgp/peer/peer1"},
						},
					},
				},
			},
			nil
	})
	config.RegisterImpliedFilterProvider("synthetic-filter", func(tree *config.Tree) ([]config.ImpliedFilterMember, error) {
		if tree == nil || tree.GetContainer("bgp") == nil {
			return nil, nil
		}
		return []config.ImpliedFilterMember{
			{
				TargetPath: "/bgp/peer/peer1",
				Direction:  "import",
				Position:   "head",
				Ref:        "demo-step",
				ImpliedBy:  "default engine step",
			},
		}, nil
	})

	m := testShowModel(t)

	// 1. Hierarchical text: `show | implied` renders # implied by: and # implied: beside written config.
	treeRes, err := m.cmdShowPipe(nil, []PipeFilter{{Type: cmdImplied}})
	require.NoError(t, err)
	assert.Contains(t, treeRes.output, "bgp {")
	assert.Contains(t, treeRes.output, "# implied by: config section bgp")
	assert.Contains(t, treeRes.output, "endpoint synthetic-ep {")
	assert.Contains(t, treeRes.output, "# implied: import head demo-step (default engine step)")
	assert.Empty(t, treeRes.statusMessage, "show | implied must not emit a redundant plain-show notice")
	for line := range strings.SplitSeq(treeRes.output, "\n") {
		assert.False(t, strings.HasPrefix(line, "##"), "rendered implied line must never start with ##: %q", line)
	}

	// 2. Set-command format: `show | implied | format config`.
	setRes, err := m.cmdShowPipe(nil, []PipeFilter{
		{Type: cmdImplied},
		{Type: cmdFormat, Arg: fmtConfig},
	})
	require.NoError(t, err)
	assert.Contains(t, setRes.output, "set bgp ")
	assert.Contains(t, setRes.output, "# implied by: config section bgp")
	assert.Contains(t, setRes.output, "set bus endpoint synthetic-ep internal synthetic-ep")

	// 3. Text filter composition: `show | implied | match` filters both written and implied lines.
	matchImplied, err := m.cmdShowPipe(nil, []PipeFilter{
		{Type: cmdImplied},
		{Type: cmdMatch, Arg: "synthetic-ep"},
	})
	require.NoError(t, err)
	assert.Contains(t, matchImplied.output, "synthetic-ep")
	assert.NotContains(t, matchImplied.output, "router-id")

	matchWritten, err := m.cmdShowPipe(nil, []PipeFilter{
		{Type: cmdImplied},
		{Type: cmdMatch, Arg: "router-id"},
	})
	require.NoError(t, err)
	assert.Contains(t, matchWritten.output, "router-id")
	assert.NotContains(t, matchWritten.output, "synthetic-ep")

	// 4. Scoped view: `show bgp | implied` includes bgp-sourced items and excludes unrelated sections.
	scopedRes, err := m.cmdShowPipe([]string{"bgp"}, []PipeFilter{{Type: cmdImplied}})
	require.NoError(t, err)
	assert.Contains(t, scopedRes.output, "synthetic-ep")
	assert.Contains(t, scopedRes.output, "demo-step")
	assert.NotContains(t, scopedRes.output, "other-ep")

	// 5. Shared transport (used by web CLI and graphical inspection).
	answer, err := m.editor.ExecuteConfigCommand(contract.ConfigCommandRequest{
		Line: "show | implied | format config",
	})
	require.NoError(t, err)
	assert.Contains(t, answer.Output, "set bus endpoint synthetic-ep internal synthetic-ep")
	assert.Empty(t, answer.Feedback)
}

// TestCmdShowPlainImpliedNoticeAndPureOutput verifies that plain `show` keeps
// the displayed configuration text byte-pure while setting statusMessage to
// config.ImpliedPlainNotice when implied items exist, and leaving statusMessage
// empty when none exist.
//
// VALIDATES: AC-23 (plain-display notice in editor status bar, byte-pure config).
// PREVENTS: leaking `# implied` comments into plain `show` or hiding the notice
// when implied items are active.
func TestCmdShowPlainImpliedNoticeAndPureOutput(t *testing.T) {
	restore := config.ResetImpliedProvidersForTest()
	t.Cleanup(restore)

	m := testShowModel(t)

	// Without implied items, plain show has empty statusMessage.
	emptyRes, err := m.cmdShow(nil)
	require.NoError(t, err)
	require.NotNil(t, emptyRes.configView)
	assert.Empty(t, emptyRes.statusMessage, "plain show must have empty status when no implied items exist")
	assert.NotContains(t, emptyRes.configView.content, "# implied")

	// Register one implied endpoint; plain show stays byte-pure and sets the notice in statusMessage.
	config.RegisterImpliedBusProvider(func(_ *config.Tree, _ config.ImpliedOptions) ([]config.ImpliedEndpoint, []config.ImpliedParticipant, error) {
		return []config.ImpliedEndpoint{
			{
				Name:           "auto-ep",
				CompiledPlugin: "auto-ep",
				ReasonKind:     "config-path",
				ReasonSource:   "bgp",
				ImpliedBy:      "config section bgp",
			},
		}, nil, nil
	})

	activeRes, err := m.cmdShow(nil)
	require.NoError(t, err)
	require.NotNil(t, activeRes.configView)
	assert.Equal(t, emptyRes.configView.content, activeRes.configView.content, "plain show config text must stay byte-pure")
	assert.NotContains(t, activeRes.configView.content, "# implied")
	assert.Equal(t, "1 implied item(s) active — use | implied to view", activeRes.statusMessage)

	// Bare `show implied` redirects to `show | implied`.
	_, err = m.cmdShow([]string{cmdImplied})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "show | implied")
}
