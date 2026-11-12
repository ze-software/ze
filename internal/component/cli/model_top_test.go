package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// topPeerContext is the context the `top <command>` tests sit in.
var topPeerContext = []string{"bgp", "peer", "peer1"}

// topTestModel returns an editor Model over a config holding peer peer1, at
// the root, with an executor that records the operational command it ran.
func topTestModel(t *testing.T) (Model, *string) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(testValidBGPConfigWithPeer), 0o600))
	ed, err := NewLooseFileEditor(nil, configPath)
	require.NoError(t, err)
	t.Cleanup(func() { ed.Close() }) //nolint:errcheck,gosec // test cleanup
	m, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)
	m.width, m.height = 100, 24
	ran := new(string)
	m.SetCommandExecutor(func(input string) (CommandOutput, error) {
		*ran = input
		return CommandOutput{Text: "ran " + input}, nil
	})
	return m, ran
}

// topInPeer returns topTestModel moved into `edit bgp peer peer1`, after the
// setup lines ran at the root.
func topInPeer(t *testing.T, setup ...string) (Model, *string) {
	t.Helper()
	m, ran := topTestModel(t)
	for _, line := range setup {
		m = enterLine(t, m, line)
		require.NoError(t, m.err, "setup line %q", line)
	}
	m = enterLine(t, m, "edit bgp peer peer1")
	require.Equal(t, topPeerContext, m.contextPath)
	return m, ran
}

// enterLine types line, presses Enter and applies what the command answers:
// the path a typed line and its result take through Update.
func enterLine(t *testing.T, m Model, line string) Model {
	t.Helper()
	out, _ := enterLineResult(t, m, line)
	return out
}

// enterLineResult is enterLine that also answers the command's result.
func enterLineResult(t *testing.T, m Model, line string) (Model, commandResult) {
	t.Helper()
	m.textInput.SetValue(line)
	next, cmd := m.handleEnter()
	entered, ok := next.(Model)
	require.True(t, ok, "handleEnter must answer a Model")
	if cmd == nil {
		return entered, commandResult{}
	}
	msg, ok := cmd().(commandResultMsg)
	if !ok {
		return entered, commandResult{}
	}
	applied, _ := entered.handleCommandResult(msg)
	out, ok := applied.(Model)
	require.True(t, ok, "handleCommandResult must answer a Model")
	return out, msg.result
}

// TestTopKeepsPositionUnlessInnerMoves proves `top <command>` runs the command
// at the root and leaves the operator where they were, unless the command
// itself moves them.
//
// VALIDATES: AC-45, AC-47, A-11: only edit, up and top move the position
// behind `top`; `top` alone still moves to the root.
// PREVENTS: cmdTop leaving the real position at the emptied root.
func TestTopKeepsPositionUnlessInnerMoves(t *testing.T) {
	cases := []struct {
		line string
		want []string
	}{
		{"top set bgp router-id 9.9.9.9", topPeerContext},
		{"top delete bgp router-id", topPeerContext},
		{"top show", topPeerContext},
		{"top commit verify", topPeerContext},
		{"top option errors hints", topPeerContext},
		{"top help", topPeerContext},
		{"top up", topPeerContext},
		{"top nosuchverb", topPeerContext},
		{"top edit bgp", []string{"bgp"}},
		{"top top edit bgp", []string{"bgp"}},
		{"top edit bgp peer peer1", topPeerContext},
		{"top", nil},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			m, _ := topInPeer(t)
			out := enterLine(t, m, tc.line)
			assert.Equal(t, tc.want, out.contextPath)
		})
	}

	t.Run("the set lands at the root path", func(t *testing.T) {
		m, _ := topInPeer(t)
		out := enterLine(t, m, "top set bgp router-id 9.9.9.9")
		require.NoError(t, out.err)
		assert.Contains(t, out.editor.ContentAtPath([]string{"bgp"}), "router-id 9.9.9.9")
		assert.NotContains(t, out.editor.ContentAtPath(topPeerContext), "9.9.9.9")
	})
}

// TestTopSavesTheOperatorPositionOnTheCopy proves cmdTop keeps the operator's
// position on the per-command copy, outermost `top` only, and never on the
// real Model.
//
// VALIDATES: Data Flow 7, the topOrigin/underTop pair read by
// operatorPosition.
// PREVENTS: a nested `top` overwriting the origin with the root, or the real
// Model carrying a stale origin after the command.
func TestTopSavesTheOperatorPositionOnTheCopy(t *testing.T) {
	m, _ := topInPeer(t)
	cp := m
	_, err := cp.cmdTop([]string{"top", "show"})
	require.NoError(t, err)
	assert.Equal(t, topPeerContext, cp.operatorPosition(), "the copy keeps the outermost origin")
	assert.Nil(t, cp.contextPath, "the copy resolves at the root")
	assert.False(t, m.underTop, "the real Model holds no origin")
	assert.Equal(t, topPeerContext, m.operatorPosition())
}

// TestTopShowShowsTheRoot proves `top show` inside a context draws exactly
// the view `show` draws at the root, diff annotator included.
//
// VALIDATES: AC-46, R-18.
// PREVENTS: the root view drawn with the line gutter because the model sits
// inside a context.
func TestTopShowShowsTheRoot(t *testing.T) {
	setup := []string{
		"set bgp peer peer2 connection remote ip 2.2.2.2",
		"set bgp peer peer2 session asn remote 65002",
	}
	root, _ := topTestModel(t)
	for _, line := range setup {
		root = enterLine(t, root, line)
		require.NoError(t, root.err)
	}
	view := root.configViewAtPath(nil)
	tree, _ := annotateContentWithTreeDiff(view.originalContent, view.content, root.editor.schema)
	gutter, _ := annotateContentWithGutter(view.originalContent, view.content)
	require.NotEqual(t, tree, gutter, "precondition: the two annotators must draw this change differently")

	root = enterLine(t, root, "show")
	require.NoError(t, root.err)

	m, _ := topInPeer(t, setup...)
	out := enterLine(t, m, "top show")
	require.NoError(t, out.err)
	assert.Equal(t, root.viewportContent, out.viewportContent, "top show draws the root view")
	assert.Equal(t, topPeerContext, out.contextPath)
}

// TestShowPathAnnotatesAtItsOwnPath proves a view is annotated for the path
// it was built at, whatever the model's position.
//
// VALIDATES: Data Flow 7 "The annotator follows the view's own path".
// PREVENTS: `show <path>` at the root drawing a sub-view with the root's tree
// diff, and `top show` drawing a root view with the line gutter.
func TestShowPathAnnotatesAtItsOwnPath(t *testing.T) {
	m, _ := topTestModel(t)
	for _, line := range []string{
		"set bgp peer peer2 connection remote ip 2.2.2.2",
		"set bgp peer peer2 session asn remote 65002",
	} {
		m = enterLine(t, m, line)
		require.NoError(t, m.err)
	}

	_, shown := enterLineResult(t, m, "show bgp")
	require.NotNil(t, shown.configView)
	assert.Equal(t, []string{"bgp"}, shown.configView.path, "show <path> records its path")

	// A view the two annotators draw differently, recorded at bgp.
	view := *m.configViewAtPath(nil)
	tree, _ := annotateContentWithTreeDiff(view.originalContent, view.content, m.editor.schema)
	gutter, _ := annotateContentWithGutter(view.originalContent, view.content)
	require.NotEqual(t, tree, gutter, "precondition: the two annotators must draw this change differently")
	view.path = []string{"bgp"}

	atRoot := m
	atRoot.contextPath = nil
	atRoot.setViewportData(view)
	inBGP := m
	inBGP.contextPath = []string{"bgp"}
	inBGP.setViewportData(view)
	assert.Equal(t, inBGP.viewportContent, atRoot.viewportContent, "the view's path picks the annotator")
}

// TestTopSetRefreshesAtThePosition proves a change made behind `top` redraws
// the context the prompt names, not the root the command ran at.
//
// VALIDATES: AC-45, R-18.
// PREVENTS: an incidental view built on the copy at the root.
func TestTopSetRefreshesAtThePosition(t *testing.T) {
	m, _ := topInPeer(t)
	out, result := enterLineResult(t, m, "top set bgp router-id 9.9.9.9")
	require.NoError(t, out.err)
	assert.True(t, result.refreshConfig, "set answers a refresh")
	assert.Nil(t, result.configView, "set builds no view on the copy")

	ref := out
	ref.setViewportData(*ref.configViewAtPath(topPeerContext))
	assert.Equal(t, ref.viewportContent, out.viewportContent, "the view is the peer's")
	assert.Equal(t, topPeerContext, out.contextPath)
}

// TestTopFailingCommandKeepsPosition proves a refused command behind `top`
// answers its own refusal and changes nothing.
//
// VALIDATES: AC-48 (the refusal; its fix framing is a later phase).
// PREVENTS: `top` swallowing the inner error or moving the operator.
func TestTopFailingCommandKeepsPosition(t *testing.T) {
	root, _ := topTestModel(t)
	root = enterLine(t, root, "set nosuchleaf 1")
	require.Error(t, root.err, "precondition: the root refuses the line")

	m, _ := topInPeer(t)
	before := m.editor.WorkingContent()
	out := enterLine(t, m, "top set nosuchleaf 1")
	require.Error(t, out.err)
	assert.Equal(t, root.err.Error(), out.err.Error(), "the inner command's own refusal")
	assert.Equal(t, before, out.editor.WorkingContent(), "nothing stored")
	assert.Equal(t, topPeerContext, out.contextPath)

	out = enterLine(t, m, "top nosuchverb")
	require.Error(t, out.err)
	assert.Equal(t, "unknown command: nosuchverb", out.err.Error())
	assert.Equal(t, topPeerContext, out.contextPath)
}

// topObserved is what an operator sees after a line: mode, prompts, viewport,
// position and what reached the daemon.
type topObserved struct {
	mode        EditorMode
	quitting    bool
	confirmQuit bool
	confirmExit bool
	confirmStop bool
	restart     bool
	status      string
	viewport    string
	ran         string
	err         string
	position    []string
}

func observeTop(m Model, ran *string) topObserved {
	o := topObserved{
		mode: m.mode, quitting: m.quitting, confirmQuit: m.confirmQuit,
		confirmExit: m.confirmExitConfig, confirmStop: m.confirmStop, restart: m.confirmRestart,
		status: m.statusMessage, viewport: m.viewportContent, ran: *ran, position: m.contextPath,
	}
	if m.err != nil {
		o.err = m.err.Error()
	}
	return o
}

// TestTopInterceptedVerbsBehaveAsWithout proves the verbs handleEnter takes
// before dispatch behave behind `top` exactly as without it.
//
// VALIDATES: AC-47, AC-48 (`top exit` with a pending change), A-13, R-19.
// PREVENTS: `top exit` or `top run ...` reaching dispatch as an unknown
// command.
func TestTopInterceptedVerbsBehaveAsWithout(t *testing.T) {
	cases := []struct {
		verb  string
		setup []string
	}{
		{verb: "exit"},
		{verb: "quit"},
		{verb: "stop"},
		{verb: "restart"},
		{verb: "clear"},
		{verb: "run"},
		{verb: "run show version"},
		{verb: "exit", setup: []string{"set bgp router-id 9.9.9.9"}},
	}
	for _, tc := range cases {
		for _, prefix := range []string{"top ", "top top "} {
			t.Run(prefix+tc.verb, func(t *testing.T) {
				plain, plainRan := topInPeer(t, tc.setup...)
				plain = enterLine(t, plain, tc.verb)
				behind, behindRan := topInPeer(t, tc.setup...)
				behind = enterLine(t, behind, prefix+tc.verb)
				assert.Equal(t, observeTop(plain, plainRan), observeTop(behind, behindRan))
			})
		}
	}
}

// TestTopKeepsHistoryAsTyped proves history records a line behind `top` as
// typed, and records nothing where the plain verb records nothing.
//
// VALIDATES: AC-45, AC-54 (history).
// PREVENTS: the strip in handleEnter rewriting the recorded line.
func TestTopKeepsHistoryAsTyped(t *testing.T) {
	m, ran := topInPeer(t)
	for _, line := range []string{"top set bgp router-id 9.9.9.9", "top run show version", "top clear", "top exit"} {
		m = enterLine(t, m, line)
	}
	require.Equal(t, "show version", *ran)
	entries := m.history.Entries()
	for _, typed := range []string{"top set bgp router-id 9.9.9.9", "top run show version", "top clear"} {
		assert.Contains(t, entries, typed)
	}
	for _, stripped := range []string{"set bgp router-id 9.9.9.9", "run show version", "clear", "exit", "top exit"} {
		assert.NotContains(t, entries, stripped)
	}
}

// TestTopPipedShowRunsAtTheRoot proves `top` is taken before the pipe split.
//
// VALIDATES: AC-53 (`top show | errors` is not refused as unpipeable), R-21.
// PREVENTS: `command 'top' does not support piping`.
func TestTopPipedShowRunsAtTheRoot(t *testing.T) {
	root, _ := topTestModel(t)
	root = enterLine(t, root, "show | errors")
	require.NoError(t, root.err)

	m, _ := topInPeer(t)
	out := enterLine(t, m, "top show | errors")
	require.NoError(t, out.err)
	assert.Equal(t, root.viewportContent, out.viewportContent)
	assert.Equal(t, root.statusMessage, out.statusMessage)
	assert.Equal(t, topPeerContext, out.contextPath)
}

// TestTopLoadTerminalPastesAtTheRoot proves the paste of a
// `top load terminal relative ...` lands at the root, and a plain one at the
// context, with the position kept.
//
// VALIDATES: AC-52, R-19 (multi-step commands).
// PREVENTS: finishPasteMode applying the paste at the real position after
// the copy that ran `load` has gone.
func TestTopLoadTerminalPastesAtTheRoot(t *testing.T) {
	t.Run("behind top", func(t *testing.T) {
		m, _ := topInPeer(t)
		m = enterLine(t, m, "top load terminal relative merge")
		require.True(t, m.pasteMode)
		m.pasteBuffer.WriteString("bgp { router-id 9.9.9.9; }")
		next, _ := m.finishPasteMode()
		out, ok := next.(Model)
		require.True(t, ok)
		require.NoError(t, out.err)
		assert.Contains(t, out.editor.ContentAtPath([]string{"bgp"}), "router-id 9.9.9.9")
		assert.NotContains(t, out.editor.ContentAtPath(topPeerContext), "9.9.9.9")
		assert.Equal(t, topPeerContext, out.contextPath)
	})
	t.Run("without top", func(t *testing.T) {
		m, _ := topInPeer(t)
		m = enterLine(t, m, "load terminal relative merge")
		require.True(t, m.pasteMode)
		m.pasteBuffer.WriteString("timer { receive-hold-time 60; }")
		next, _ := m.finishPasteMode()
		out, ok := next.(Model)
		require.True(t, ok)
		require.NoError(t, out.err)
		assert.Contains(t, out.editor.ContentAtPath(topPeerContext), "receive-hold-time 60")
		assert.Equal(t, topPeerContext, out.contextPath)
	})
}

// TestTopCompletesFromTheRoot proves completion after `top ` offers what the
// root offers at the same point, and the help table carries the form.
//
// VALIDATES: AC-49.
// PREVENTS: the context's children offered after `top set `.
func TestTopCompletesFromTheRoot(t *testing.T) {
	for _, rest := range []string{"", "set ", "se", "show "} {
		t.Run("top "+rest, func(t *testing.T) {
			root, _ := topTestModel(t)
			root.textInput.SetValue(rest)
			root.updateCompletions()

			m, _ := topInPeer(t)
			m.textInput.SetValue("top " + rest)
			m.updateCompletions()
			assert.Equal(t, completionTexts(root.completions), completionTexts(m.completions))
		})
	}

	t.Run("the context completes otherwise", func(t *testing.T) {
		root, _ := topTestModel(t)
		root.textInput.SetValue("set ")
		root.updateCompletions()
		m, _ := topInPeer(t)
		m.textInput.SetValue("set ")
		m.updateCompletions()
		require.NotEqual(t, completionTexts(root.completions), completionTexts(m.completions),
			"precondition: the context offers other children than the root")
	})

	t.Run("the help table names the form", func(t *testing.T) {
		var forms []string
		for _, e := range EditorHelpEntries() {
			forms = append(forms, e.Name)
		}
		assert.Contains(t, forms, "top <command>")
		assert.Contains(t, forms, "top")
	})
}
