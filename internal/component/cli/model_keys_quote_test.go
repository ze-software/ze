// VALIDATES: a ? typed inside an open double-quoted value lands in the input,
// and a ? outside one keeps its help meaning
// PREVENTS: an operator being unable to type a value that carries a ?, a URL
// with a query string being the common case

package cli

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// typeRunes presses each rune of text in turn through the key handler, the way
// a terminal delivers typed characters, and returns the resulting model.
func typeRunes(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		m, _ = pressKey(t, &m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return m
}

// TestQuestionMarkInsideQuotesIsTyped covers the defect: every keystroke of a
// quoted URL with a query string goes through the key handler, and the input
// is read back.
//
// VALIDATES: AC-1 — each ? inside the open quote reaches the input as a
// character, and no help is revealed on the way.
// PREVENTS: ? opening help or completing while the operator types a value.
func TestQuestionMarkInsideQuotesIsTyped(t *testing.T) {
	const line = `set bgp description "https://example.net/a?b=1&c=?"`

	m := newTestModel(t)
	if m.mode != ModeConfig {
		t.Fatalf("precondition: mode = %v, want config", m.mode)
	}

	typed := typeRunes(t, *m, line)

	if typed.InputValue() != line {
		t.Errorf("input = %q, want %q", typed.InputValue(), line)
	}
	if typed.revealLevel() != revealNothing {
		t.Errorf("reveal level = %d, want revealNothing", typed.revealLevel())
	}
}

// TestQuestionMarkInsideAnUnterminatedQuoteIsTyped covers the quote the
// operator has opened and not yet closed, which is the state every quoted
// value is in while it is being typed.
//
// VALIDATES: AC-2 — ? after an opening quote at the end of the line is
// inserted.
// PREVENTS: the rule only holding for a quote that is already closed.
func TestQuestionMarkInsideAnUnterminatedQuoteIsTyped(t *testing.T) {
	m := newTestModel(t)
	m.textInput.SetValue(`set bgp description "a`)
	m.textInput.CursorEnd()

	typed, _ := pressKey(t, m, tea.KeyPressMsg{Code: '?', Text: "?"})

	if typed.InputValue() != `set bgp description "a?` {
		t.Errorf("input = %q, want the ? inserted", typed.InputValue())
	}
}

// TestQuestionMarkFollowsTheCursorNotTheLine reads the quote state at the
// cursor, which is where the character would land.
//
// VALIDATES: AC-3, AC-4 — with the cursor moved back between a pair of quotes,
// ? is inserted there; with the cursor before the opening quote, ? inserts
// nothing.
// PREVENTS: the rule reading the whole line, which treats a closed pair as
// "no quote open" wherever the cursor sits, and an open quote after the cursor
// as open at it.
func TestQuestionMarkFollowsTheCursorNotTheLine(t *testing.T) {
	t.Run("between the quotes", func(t *testing.T) {
		m := newTestModel(t)
		m.textInput.SetValue(`set bgp description "ab"`)
		m.textInput.SetCursor(len(`set bgp description "a`))

		typed, _ := pressKey(t, m, tea.KeyPressMsg{Code: '?', Text: "?"})

		if typed.InputValue() != `set bgp description "a?b"` {
			t.Errorf("input = %q, want the ? inserted at the cursor", typed.InputValue())
		}
	})

	t.Run("before an opening quote", func(t *testing.T) {
		m := tabTestModel(t)
		m.width = 100
		m.height = 24
		m.textInput.SetValue(`peer "x`)
		m.textInput.SetCursor(len(`peer `))

		asked, _ := pressKey(t, m, tea.KeyPressMsg{Code: '?', Text: "?"})

		if asked.InputValue() != `peer "x` {
			t.Errorf("input = %q, want ? outside the quote to insert nothing", asked.InputValue())
		}
	})
}

// TestQuestionMarkOutsideQuotesStillRevealsHelp is the control for the tests
// above: the same key, outside a quote, keeps its help meaning.
//
// VALIDATES: AC-4 — after a closed pair of quotes ? inserts nothing, and on a
// line with no quote ? opens the candidate menu as Tab does.
// PREVENTS: the quote rule leaking past the closing quote and turning ? into
// a plain character everywhere.
func TestQuestionMarkOutsideQuotesStillRevealsHelp(t *testing.T) {
	t.Run("after a closing quote", func(t *testing.T) {
		m := newTestModel(t)
		m.textInput.SetValue(`set bgp description "a" `)
		m.textInput.CursorEnd()

		asked, _ := pressKey(t, m, tea.KeyPressMsg{Code: '?', Text: "?"})

		if asked.InputValue() != `set bgp description "a" ` {
			t.Errorf("input = %q, want ? outside the quote to insert nothing", asked.InputValue())
		}
	})

	t.Run("no quote on the line", func(t *testing.T) {
		m := tabTestModel(t)
		m.width = 100
		m.height = 24
		m.textInput.SetValue("peer ")
		m.textInput.CursorEnd()
		m.updateCompletions()

		asked := pressQuestionMark(t, m)

		if !asked.ShowDropdown() {
			t.Error("the menu is closed, want ? outside a quote to open it as Tab does")
		}
		if asked.InputValue() != "peer " {
			t.Errorf("input = %q, want ? to insert nothing", asked.InputValue())
		}
	})
}

// TestPastedQuestionMarkReachesTheInput covers the bracketed-paste route,
// which bubbletea delivers as one PasteMsg rather than as key presses.
//
// VALIDATES: AC-5 — a pasted value carrying ? arrives whole, unquoted.
// PREVENTS: a paste reaching the ? arm, which would swallow the character.
func TestPastedQuestionMarkReachesTheInput(t *testing.T) {
	m := newTestModel(t)

	next, _ := m.Update(tea.PasteMsg{Content: "https://example.net/a?b=1"})
	pasted, ok := next.(Model)
	if !ok {
		t.Fatal("Update must return a Model")
	}

	if pasted.InputValue() != "https://example.net/a?b=1" {
		t.Errorf("input = %q, want the pasted text whole", pasted.InputValue())
	}
}
