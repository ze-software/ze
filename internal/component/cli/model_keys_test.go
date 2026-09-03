// VALIDATES: AC-1, AC-4, AC-5 to AC-8, AC-10 — what Tab reveals, what every key
// does at each reveal level, and what Tab refuses to invent
// PREVENTS: the declared explanation staying unreachable from the CLI, Tab
// inventing text for a command that declares none, and a reveal that swallows
// the keystroke which dismissed it

package cli

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// peerListHelp is the long explanation the test tree declares for "peer list".
// It carries an authored newline, because a declared explanation holds the line
// breaks its author wrote.
const peerListHelp = "List every peer this daemon knows.\nOne row per peer, in configuration order."

// peerResetHelp is the long explanation the test tree declares for "peer reset".
// Two siblings declare different explanations, so a test reads which candidate
// an explanation came from.
const peerResetHelp = "Reset the session of one peer.\nThe peer reconnects on its own timer."

// tabTestModel returns an operational-mode model over a three-command tree.
// "peer list" and "peer reset" declare different long explanations, and "peer
// lock" declares none. One tree therefore serves the reveal tests, the tests
// that read WHICH candidate an explanation came from, and the
// nothing-invented tests.
func tabTestModel(t *testing.T) *Model {
	t.Helper()
	m := newTestModel(t)
	m.SetCommandCompleter(NewCommandCompleter(&commandNode{
		Children: map[string]*commandNode{
			"peer": {Name: "peer", Description: "Peer operations", Children: map[string]*commandNode{
				"list":  {Name: "list", Description: "List peers", LongHelp: peerListHelp},
				"lock":  {Name: "lock", Description: "Lock a peer"},
				"reset": {Name: "reset", Description: "Reset a peer", LongHelp: peerResetHelp},
			}},
		},
	}))
	m.switchMode(ModeOperational)
	return m
}

// pressTab presses Tab once and returns the model the key handler produced.
func pressTab(t *testing.T, m *Model) Model {
	t.Helper()
	next, _ := m.handleTab()
	updated, ok := next.(Model)
	if !ok {
		t.Fatal("handleTab must return a Model")
	}
	return updated
}

// TestTabOnExhaustedCompletionRevealsTheExplanation covers AC-4.
//
// VALIDATES: Tab with nothing left to complete puts the command's declared long
// explanation on the screen.
// PREVENTS: the long explanation staying unreachable, which is the defect this
// spec exists to fix.
func TestTabOnExhaustedCompletionRevealsTheExplanation(t *testing.T) {
	m := tabTestModel(t)
	m.textInput.SetValue("peer list ")
	m.updateCompletions()
	if len(m.Completions()) != 0 {
		t.Fatalf("precondition: the completion list must be exhausted, got %d entries", len(m.Completions()))
	}

	updated := pressTab(t, m)

	if updated.Explanation() != peerListHelp {
		t.Errorf("explanation = %q, want %q", updated.Explanation(), peerListHelp)
	}
	if updated.revealLevel() != revealExplanation {
		t.Errorf("reveal level = %d, want revealExplanation", updated.revealLevel())
	}
}

// TestTabRevealsTheExplanationBehindTheRunPrefix covers AC-4 in config mode.
//
// VALIDATES: the text Tab explains is the text updateCompletions completes, so
// the "run " prefix is stripped on both paths.
// PREVENTS: an explanation looked up for "run peer list", which names no command.
func TestTabRevealsTheExplanationBehindTheRunPrefix(t *testing.T) {
	m := tabTestModel(t)
	m.switchMode(ModeConfig)
	m.textInput.SetValue("run peer list ")
	m.updateCompletions()

	updated := pressTab(t, m)

	if updated.Explanation() != peerListHelp {
		t.Errorf("explanation = %q, want %q", updated.Explanation(), peerListHelp)
	}
}

// TestTabStillCompletesWhenThereIsMoreToAdd covers AC-1.
//
// VALIDATES: Tab completes exactly as it did before this spec while the typed
// path still has something to add, and reveals nothing.
// PREVENTS: the reveal stealing the keystroke that completes a command.
func TestTabStillCompletesWhenThereIsMoreToAdd(t *testing.T) {
	m := tabTestModel(t)
	m.textInput.SetValue("peer lis")
	m.updateCompletions()

	updated := pressTab(t, m)

	if updated.InputValue() != "peer list " {
		t.Errorf("input = %q, want %q", updated.InputValue(), "peer list ")
	}
	if updated.Explanation() != "" {
		t.Errorf("explanation = %q, want none while a completion remains", updated.Explanation())
	}
	if updated.revealLevel() != revealNothing {
		t.Errorf("reveal level = %d, want revealNothing", updated.revealLevel())
	}
}

// TestTabOnACommandWithNoExplanationInventsNothing covers AC-10.
//
// VALIDATES: a command that declares no long explanation keeps the reveal level
// where it is, and the message line says none is declared.
// PREVENTS: a silent no-op, which leaves the operator unable to tell "no
// explanation exists" from "the key did nothing".
func TestTabOnACommandWithNoExplanationInventsNothing(t *testing.T) {
	m := tabTestModel(t)
	m.textInput.SetValue("peer lock ")
	m.updateCompletions()

	updated := pressTab(t, m)

	if updated.Explanation() != "" {
		t.Errorf("explanation = %q, want none for a command that declares none", updated.Explanation())
	}
	if updated.revealLevel() != revealNothing {
		t.Errorf("reveal level = %d, want revealNothing", updated.revealLevel())
	}
	hint := updated.MessageHint()
	if !strings.Contains(hint, "peer lock") {
		t.Errorf("message line = %q, want it to name the command the operator typed", hint)
	}
	if !strings.Contains(hint, "no explanation") {
		t.Errorf("message line = %q, want it to say no explanation is declared", hint)
	}
}

// TestTabRevealsAfterTheFinalCompletionRefresh guards the write order.
//
// updateCompletions calls dismissReveal whenever one candidate or none is left,
// which is the state that sends Tab to the reveal. handleTab refreshes an empty
// completion list before it decides, so a reveal written before that refresh is
// erased by it. This test enters handleTab with no completions computed at all,
// which forces the refresh to run after the model is built.
//
// VALIDATES: the explanation is written after the last updateCompletions call.
// PREVENTS: a reveal that the refresh below it clears, which shows nothing and
// looks like a dead key.
func TestTabRevealsAfterTheFinalCompletionRefresh(t *testing.T) {
	m := tabTestModel(t)
	m.textInput.SetValue("peer list ")
	if len(m.Completions()) != 0 {
		t.Fatalf("precondition: no completions are computed yet, got %d", len(m.Completions()))
	}

	updated := pressTab(t, m)

	if updated.Explanation() != peerListHelp {
		t.Errorf("explanation = %q, want %q", updated.Explanation(), peerListHelp)
	}
}

// pressKey sends one key through the whole dispatch, which is the path a real
// keystroke takes. It returns the model the dispatch produced and the command
// it asked the runtime to run.
func pressKey(t *testing.T, m *Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.handleKeyMsg(msg)
	updated, ok := next.(Model)
	if !ok {
		t.Fatal("handleKeyMsg must return a Model")
	}
	return updated, cmd
}

// revealedModel returns a model showing the long explanation of "peer list".
func revealedModel(t *testing.T) Model {
	t.Helper()
	m := tabTestModel(t)
	m.width = 100
	m.height = 24
	m.textInput.SetValue("peer list ")
	m.updateCompletions()
	revealed := pressTab(t, m)
	if revealed.revealLevel() != revealExplanation {
		t.Fatalf("precondition: reveal level = %d, want revealExplanation", revealed.revealLevel())
	}
	return revealed
}

// menuModel returns a model showing the completion menu for "peer ".
func menuModel(t *testing.T) Model {
	t.Helper()
	m := tabTestModel(t)
	m.width = 100
	m.height = 24
	m.textInput.SetValue("peer ")
	m.updateCompletions()
	menu := pressTab(t, m)
	if menu.revealLevel() != revealCandidates {
		t.Fatalf("precondition: reveal level = %d, want revealCandidates", menu.revealLevel())
	}
	return menu
}

// TestEscapeDescendsOneLevelPerPress covers AC-5 and AC-6.
//
// VALIDATES: one Escape takes off one reveal, and the words the operator typed
// to reach that reveal stay in the input.
// PREVENTS: Escape at the explanation clearing the whole input, which throws
// away the command the explanation describes.
func TestEscapeDescendsOneLevelPerPress(t *testing.T) {
	t.Run("from the explanation", func(t *testing.T) {
		revealed := revealedModel(t)

		first, _ := pressKey(t, &revealed, tea.KeyPressMsg{Code: tea.KeyEscape})

		if first.InputValue() != "peer list " {
			t.Errorf("input = %q, want the typed command to stay", first.InputValue())
		}
		if first.Explanation() != "" {
			t.Errorf("explanation = %q, want it taken off", first.Explanation())
		}
		if first.revealLevel() != revealNothing {
			t.Errorf("reveal level = %d, want revealNothing", first.revealLevel())
		}
		if first.ShowDropdown() != revealed.ShowDropdown() {
			t.Errorf("menu open = %v, want %v", first.ShowDropdown(), revealed.ShowDropdown())
		}
		if first.SelectedIndex() != revealed.SelectedIndex() {
			t.Errorf("selection = %d, want %d", first.SelectedIndex(), revealed.SelectedIndex())
		}

		second, _ := pressKey(t, &first, tea.KeyPressMsg{Code: tea.KeyEscape})

		if second.InputValue() != "" {
			t.Errorf("input = %q, want the second Escape to clear it", second.InputValue())
		}
	})

	t.Run("from the menu", func(t *testing.T) {
		menu := menuModel(t)
		if !strings.Contains(menu.MessageHint(), "List peers") {
			t.Fatalf("precondition: message line = %q, want the selected summary", menu.MessageHint())
		}

		next, _ := pressKey(t, &menu, tea.KeyPressMsg{Code: tea.KeyEscape})

		if next.ShowDropdown() {
			t.Error("menu is still open, want Escape to close it")
		}
		if strings.Contains(next.MessageHint(), "List peers") {
			t.Errorf("message line = %q, want the summary gone with the menu", next.MessageHint())
		}
		if next.InputValue() != "peer " {
			t.Errorf("input = %q, want the typed words to stay", next.InputValue())
		}
		if next.revealLevel() != revealNothing {
			t.Errorf("reveal level = %d, want revealNothing", next.revealLevel())
		}
	})
}

// TestTypingDismissesHelpAndReachesTheInput covers AC-7.
//
// VALIDATES: a key pressed at the explanation both ends the reveal and reaches
// the input. The second half is what makes the reveal safe to leave on screen.
// PREVENTS: a reveal that eats the keystroke which dismissed it, so the
// operator types a rune and the prompt does not move.
func TestTypingDismissesHelpAndReachesTheInput(t *testing.T) {
	t.Run("a rune", func(t *testing.T) {
		revealed := revealedModel(t)

		typed, _ := pressKey(t, &revealed, tea.KeyPressMsg{Code: 'x', Text: "x"})

		if typed.InputValue() != "peer list x" {
			t.Errorf("input = %q, want the rune to land in it", typed.InputValue())
		}
		if typed.revealLevel() != revealNothing {
			t.Errorf("reveal level = %d, want revealNothing", typed.revealLevel())
		}
	})

	t.Run("a rune at the menu", func(t *testing.T) {
		menu := menuModel(t)
		// ? reveals the explanation of the highlighted candidate over the menu.
		// The menu still holds three candidates, so updateCompletions leaves
		// that reveal alone and only the key handler can take it off.
		asked := pressQuestionMark(t, &menu)
		if asked.Explanation() != peerListHelp {
			t.Fatalf("precondition: explanation = %q, want the candidate's", asked.Explanation())
		}

		typed, _ := pressKey(t, &asked, tea.KeyPressMsg{Code: 'l', Text: "l"})

		if typed.Explanation() != "" {
			t.Errorf("explanation = %q, want it dismissed by the rune", typed.Explanation())
		}

		if typed.InputValue() != "peer l" {
			t.Errorf("input = %q, want the rune to land in it", typed.InputValue())
		}
		// The menu is a live view of what the input matches, so a rune that
		// still matches narrows it rather than closing it.
		if !typed.ShowDropdown() {
			t.Error("the menu closed, want it to follow the narrowed input")
		}
	})

	t.Run("backspace", func(t *testing.T) {
		revealed := revealedModel(t)

		typed, _ := pressKey(t, &revealed, tea.KeyPressMsg{Code: tea.KeyBackspace})

		if typed.InputValue() != "peer list" {
			t.Errorf("input = %q, want backspace to reach it", typed.InputValue())
		}
		if typed.revealLevel() != revealNothing {
			t.Errorf("reveal level = %d, want revealNothing", typed.revealLevel())
		}
	})

	t.Run("ctrl-c", func(t *testing.T) {
		revealed := revealedModel(t)

		quit, _ := pressKey(t, &revealed, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

		if !quit.confirmQuit {
			t.Error("ctrl-c did not ask for quit confirmation")
		}
		if quit.revealLevel() != revealNothing {
			t.Errorf("reveal level = %d, want revealNothing", quit.revealLevel())
		}
		if strings.Contains(quit.View().Content, "One row per peer") {
			t.Error("the explanation box is still drawn over the quit confirmation")
		}
	})
}

// TestEnterRunsTheCommandFromEveryLevel covers AC-8.
//
// VALIDATES: what Enter runs is the command in the input, whatever the reveal
// level was. At the menu Enter is the only key that accepts the highlighted
// candidate, so it accepts first and runs on the next press.
// PREVENTS: a reveal that changes the command an operator runs, and a menu with
// no way to accept the candidate it highlights.
func TestEnterRunsTheCommandFromEveryLevel(t *testing.T) {
	t.Run("plain prompt", func(t *testing.T) {
		m, ran := enterTestModel(t)
		m.textInput.SetValue("peer list")

		_, cmd := pressKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
		runCommand(t, cmd)

		if *ran != "peer list" {
			t.Errorf("ran %q, want %q", *ran, "peer list")
		}
	})

	t.Run("explanation", func(t *testing.T) {
		m, ran := enterTestModel(t)
		m.textInput.SetValue("peer list ")
		m.updateCompletions()
		revealed := pressTab(t, m)
		if revealed.revealLevel() != revealExplanation {
			t.Fatalf("precondition: reveal level = %d, want revealExplanation", revealed.revealLevel())
		}

		after, cmd := pressKey(t, &revealed, tea.KeyPressMsg{Code: tea.KeyEnter})
		runCommand(t, cmd)

		if *ran != "peer list" {
			t.Errorf("ran %q, want %q", *ran, "peer list")
		}
		if after.revealLevel() != revealNothing {
			t.Errorf("reveal level = %d, want revealNothing", after.revealLevel())
		}
	})

	t.Run("menu", func(t *testing.T) {
		m, ran := enterTestModel(t)
		m.textInput.SetValue("peer ")
		m.updateCompletions()
		menu := pressTab(t, m)
		if menu.revealLevel() != revealCandidates {
			t.Fatalf("precondition: reveal level = %d, want revealCandidates", menu.revealLevel())
		}

		accepted, cmd := pressKey(t, &menu, tea.KeyPressMsg{Code: tea.KeyEnter})
		runCommand(t, cmd)

		if *ran != "" {
			t.Errorf("ran %q, want the first Enter to accept the candidate only", *ran)
		}
		if accepted.InputValue() != "peer list " {
			t.Errorf("input = %q, want the highlighted candidate accepted", accepted.InputValue())
		}

		_, cmd = pressKey(t, &accepted, tea.KeyPressMsg{Code: tea.KeyEnter})
		runCommand(t, cmd)

		if *ran != "peer list" {
			t.Errorf("ran %q, want %q", *ran, "peer list")
		}
	})
}

// enterTestModel returns a tab test model whose executor records the command
// string it is asked to run, so a test reads what reached the daemon.
func enterTestModel(t *testing.T) (*Model, *string) {
	t.Helper()
	m := tabTestModel(t)
	m.width = 100
	m.height = 24
	ran := new(string)
	m.SetCommandExecutor(func(input string) (CommandOutput, error) {
		*ran = input
		return CommandOutput{}, nil
	})
	return m, ran
}

// runCommand runs the command the dispatch returned, which is where the
// executor is called.
func runCommand(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	cmd()
}

// pressQuestionMark presses ? once and returns the model the dispatch produced.
func pressQuestionMark(t *testing.T, m *Model) Model {
	t.Helper()
	asked, _ := pressKey(t, m, tea.KeyPressMsg{Code: '?', Text: "?"})
	return asked
}

// TestQuestionMarkRevealsTheSelectedCandidateExplanation covers what an operator
// asks for when they highlight a name and press ?.
//
// VALIDATES: ? opens the explanation box on the CANDIDATE the selection points
// at, and the selection decides which text arrives.
// PREVENTS: the explanation of the typed path, or of another candidate, being
// shown for the name the operator highlighted.
func TestQuestionMarkRevealsTheSelectedCandidateExplanation(t *testing.T) {
	t.Run("the first candidate", func(t *testing.T) {
		menu := menuModel(t)

		asked := pressQuestionMark(t, &menu)

		if asked.Explanation() != peerListHelp {
			t.Errorf("explanation = %q, want %q", asked.Explanation(), peerListHelp)
		}
		if asked.revealLevel() != revealExplanation {
			t.Errorf("reveal level = %d, want revealExplanation", asked.revealLevel())
		}
	})

	t.Run("a later candidate", func(t *testing.T) {
		menu := menuModel(t)
		// Tab cycles the selection, so two presses move it from "list" to
		// "reset". Both declare an explanation, and they declare different ones.
		moved := pressTab(t, &menu)
		moved = pressTab(t, &moved)
		if moved.Completions()[moved.SelectedIndex()].Text != "reset" {
			t.Fatalf("precondition: selection is %q, want reset",
				moved.Completions()[moved.SelectedIndex()].Text)
		}

		asked := pressQuestionMark(t, &moved)

		if asked.Explanation() != peerResetHelp {
			t.Errorf("explanation = %q, want %q", asked.Explanation(), peerResetHelp)
		}
		if asked.Explanation() == peerListHelp {
			t.Error("explanation belongs to another candidate")
		}
	})
}

// TestQuestionMarkOnACandidateWithNoExplanationInventsNothing covers the
// candidate whose author wrote no long text.
//
// VALIDATES: the level stays at the menu, and the message line says no
// explanation is declared.
// PREVENTS: an empty box, which reads as an author who wrote nothing rather
// than as an undeclared explanation.
func TestQuestionMarkOnACandidateWithNoExplanationInventsNothing(t *testing.T) {
	menu := menuModel(t)
	moved := pressTab(t, &menu)
	if moved.Completions()[moved.SelectedIndex()].Text != "lock" {
		t.Fatalf("precondition: selection is %q, want lock",
			moved.Completions()[moved.SelectedIndex()].Text)
	}

	asked := pressQuestionMark(t, &moved)

	if asked.Explanation() != "" {
		t.Errorf("explanation = %q, want none for a candidate that declares none", asked.Explanation())
	}
	if asked.revealLevel() != revealCandidates {
		t.Errorf("reveal level = %d, want revealCandidates", asked.revealLevel())
	}
	hint := asked.MessageHint()
	if !strings.Contains(hint, "peer lock") {
		t.Errorf("message line = %q, want it to name the candidate", hint)
	}
	if !strings.Contains(hint, "no explanation") {
		t.Errorf("message line = %q, want it to say no explanation is declared", hint)
	}
}

// TestQuestionMarkOnASingleGhostMatchRevealsItsExplanation covers the reveal
// with no menu on the screen.
//
// VALIDATES: ? reads the one candidate ghost text is offering, and explains the
// command that candidate would produce.
// PREVENTS: an explanation read for the half-typed word, which names no command.
func TestQuestionMarkOnASingleGhostMatchRevealsItsExplanation(t *testing.T) {
	m := tabTestModel(t)
	m.width = 100
	m.height = 24
	m.textInput.SetValue("peer lis")
	m.updateCompletions()
	if len(m.Completions()) != 1 || m.GhostText() == "" {
		t.Fatalf("precondition: %d candidates and ghost text %q, want one and some",
			len(m.Completions()), m.GhostText())
	}

	asked := pressQuestionMark(t, m)

	if asked.Explanation() != peerListHelp {
		t.Errorf("explanation = %q, want %q", asked.Explanation(), peerListHelp)
	}
}

// TestQuestionMarkWithNoCandidateCompletes covers the fall-through.
//
// VALIDATES: with no candidate singled out, ? does what Tab does.
// PREVENTS: ? becoming a dead key at the state where the operator still has
// something to complete.
func TestQuestionMarkWithNoCandidateCompletes(t *testing.T) {
	m := tabTestModel(t)
	m.width = 100
	m.height = 24
	m.textInput.SetValue("peer ")
	m.updateCompletions()
	if m.ShowDropdown() {
		t.Fatal("precondition: the menu must be closed")
	}

	asked := pressQuestionMark(t, m)

	if !asked.ShowDropdown() {
		t.Error("the menu is closed, want ? to open it as Tab does")
	}
	if asked.SelectedIndex() != 0 {
		t.Errorf("selection = %d, want the first candidate", asked.SelectedIndex())
	}
}

// TestEscapeAtTheExplanationOverTheMenuLeavesTheMenu covers AC-5 in the state
// the operator reaches with ?.
//
// VALIDATES: one Escape takes off the explanation alone, and the menu it was
// drawn over is still open.
// PREVENTS: Escape closing both, which costs the operator the candidate list
// they were reading.
func TestEscapeAtTheExplanationOverTheMenuLeavesTheMenu(t *testing.T) {
	menu := menuModel(t)
	asked := pressQuestionMark(t, &menu)
	if asked.revealLevel() != revealExplanation {
		t.Fatalf("precondition: reveal level = %d, want revealExplanation", asked.revealLevel())
	}

	next, _ := pressKey(t, &asked, tea.KeyPressMsg{Code: tea.KeyEscape})

	if next.Explanation() != "" {
		t.Errorf("explanation = %q, want it taken off", next.Explanation())
	}
	if !next.ShowDropdown() {
		t.Error("the menu closed, want Escape to take off the explanation alone")
	}
	if next.revealLevel() != revealCandidates {
		t.Errorf("reveal level = %d, want revealCandidates", next.revealLevel())
	}
}

// showTestModel returns an operational-mode model whose tree is rooted at a verb
// the config editor also owns. `show` means the same thing in both modes, so an
// editor-backed model must still complete it from the COMMAND tree.
func showTestModel(t *testing.T) *Model {
	t.Helper()
	m := newTestModel(t)
	m.SetCommandCompleter(NewCommandCompleter(&commandNode{
		Children: map[string]*commandNode{
			"show": {Name: "show", Description: "Show operational state", Children: map[string]*commandNode{
				"version": {Name: "version", Description: "Show the version", LongHelp: "Prints the release this daemon runs."},
			}},
		},
	}))
	m.switchMode(ModeOperational)
	return m
}

// TestRevealWorksForAVerbTheConfigEditorAlsoOwns covers the regression an editor
// on the attached console exposed.
//
// VALIDATES: completesFromYANG subtracts the verbs that mean the same thing in
// both modes, so `show ...` reaches the command completer and its explanation is
// reachable.
// PREVENTS: ? and Tab returning in silence for every `show` command the moment a
// model carries a config completer, which is what `ze start --cli` gained.
func TestRevealWorksForAVerbTheConfigEditorAlsoOwns(t *testing.T) {
	m := showTestModel(t)
	if m.completer == nil {
		t.Fatal("precondition: this model must carry a config completer")
	}

	m.textInput.SetValue("show version ")
	if _, ok := m.commandCompleterInput(); !ok {
		t.Fatal("the command completer must answer for a show command")
	}
	m.updateCompletions()

	updated := pressTab(t, m)

	if updated.Explanation() != "Prints the release this daemon runs." {
		t.Errorf("explanation = %q, want the declared text", updated.Explanation())
	}
}

// TestRevealCandidateExplanationUsesLongHelp covers the config editor, where the
// two texts reach two surfaces.
//
// A config node declares the same pair a command declares. The YANG description
// is the summary the one-line message row shows, and the ze:help extension is
// the explanation. The box takes the explanation, so pressing ? adds the text
// the row cannot hold.
//
// VALIDATES: AC-11 — ? on a config candidate puts that node's ze:help in the
// box, whole, and never its description.
// PREVENTS: the box repeating the row, which is what it did while the config
// branch passed the description.
func TestRevealCandidateExplanationUsesLongHelp(t *testing.T) {
	const summary = "Classical admin distance stamped on BGP best-paths."
	const explanation = "RFC 4271 leaves the preference between protocols to the implementation.\n" +
		"Ze stamps this value on a best-path so the RIB can rank it against a route\n" +
		"another protocol installed."

	m := newTestModel(t)
	if m.mode != ModeConfig {
		t.Fatalf("precondition: mode = %v, want config", m.mode)
	}
	m.textInput.SetValue("set bgp ")
	m.completions = []Completion{{
		Text:        "admin-distance",
		Description: summary,
		LongHelp:    explanation,
		Type:        completionKeyword,
	}}
	m.showDropdown = true
	m.selected = 0

	revealed, _ := pressKey(t, m, tea.KeyPressMsg{Code: '?', Text: "?"})

	if revealed.Explanation() != explanation {
		t.Errorf("explanation = %q, want the declared ze:help", revealed.Explanation())
	}
	if strings.Contains(revealed.Explanation(), summary) {
		t.Errorf("explanation = %q, want the summary to stay on the message row alone", revealed.Explanation())
	}
	if revealed.revealLevel() != revealExplanation {
		t.Errorf("reveal level = %d, want revealExplanation", revealed.revealLevel())
	}
	if !strings.Contains(revealed.View().Content, "admin-distance") {
		t.Error("the box does not name the config path it explains")
	}
}

// TestRevealCandidateExplanationSaysNothingIsDeclared is the other half of the
// branch above.
//
// VALIDATES: AC-11 — a config candidate that declares no ze:help leaves the
// level where it is and says so, and its description does not stand in for the
// explanation it has none of.
// PREVENTS: a key that does nothing and states nothing, and the fallback that
// would put the row's own sentence back in the box.
func TestRevealCandidateExplanationSaysNothingIsDeclared(t *testing.T) {
	const summary = "The leaf nobody has explained yet."

	m := newTestModel(t)
	m.textInput.SetValue("set bgp ")
	m.completions = []Completion{{Text: "undocumented", Description: summary, Type: completionKeyword}}
	m.showDropdown = true
	m.selected = 0

	revealed, _ := pressKey(t, m, tea.KeyPressMsg{Code: '?', Text: "?"})

	if revealed.Explanation() != "" {
		t.Errorf("explanation = %q, want none for a node that declares none", revealed.Explanation())
	}
	hint := revealed.MessageHint()
	if !strings.Contains(hint, "undocumented") || !strings.Contains(hint, "no explanation") {
		t.Errorf("message line = %q, want it to name the path and say none is declared", hint)
	}
	if strings.Contains(hint, summary) {
		t.Errorf("message line = %q, want no fallback to the description", hint)
	}
}
