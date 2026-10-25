// Design: docs/architecture/cli/tui-launcher.md -- prompting for a missing required value
// Related: menu.go -- the menu that hands a chosen command to the prompt

package menu

import (
	"errors"
	"fmt"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ErrCanceled is the reader leaving a prompt with Ctrl-C or Ctrl-D. A caller
// MUST run nothing on it: the line it was filling is incomplete.
var ErrCanceled = errors.New("canceled at the prompt, so nothing ran")

// Prompt declares one value a program asks for at a terminal.
type Prompt struct {
	// Label names the value, as the command's usage renders it.
	Label string
	// Help is the one line that says what the value is.
	Help string
	// Complete answers the candidates for the text typed so far, and is asked
	// on each Tab. Nil offers nothing. It MUST NOT run anything, because a
	// keystroke calls it.
	Complete func(partial string) []string
}

// promptModel is one prompt's state. Like Model it is a value: every key
// answers a new promptModel.
type promptModel struct {
	prompt   Prompt
	text     string
	offered  []string // the candidates the last Tab could not narrow to one
	done     bool
	canceled bool
}

// Ask asks for one value on in and out, and answers it once the reader
// presses Enter on a non-empty line. An empty line is not an answer: the value
// is required, so Enter on nothing keeps asking. Ctrl-C or Ctrl-D answers
// ErrCanceled. Any other error is the terminal program's failure to run.
func Ask(prompt Prompt, in io.Reader, out io.Writer) (string, error) {
	result, err := tea.NewProgram(promptModel{prompt: prompt}, tea.WithInput(in), tea.WithOutput(out)).Run()
	if err != nil {
		return "", fmt.Errorf("prompt: %w", err)
	}
	final, ok := result.(promptModel)
	if !ok {
		return "", fmt.Errorf("prompt: the terminal program answered a %T, not the prompt", result)
	}
	if final.canceled {
		return "", ErrCanceled
	}
	if !final.done {
		return "", errors.New("prompt: the terminal closed before a value was given")
	}
	return final.text, nil
}

// Init starts the prompt with nothing to run.
func (p promptModel) Init() tea.Cmd {
	return nil
}

// Update answers the prompt after one message. Only a key press changes it.
func (p promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, isKey := msg.(tea.KeyPressMsg)
	if !isKey {
		return p, nil
	}
	switch km.String() {
	case "ctrl+c", "ctrl+d":
		p.canceled = true
		return p, tea.Quit
	case "enter":
		if strings.TrimSpace(p.text) == "" {
			return p, nil
		}
		p.done = true
		return p, tea.Quit
	case "tab":
		p.text, p.offered = completed(p.text, p.candidates())
		return p, nil
	case "backspace":
		if p.text != "" {
			runes := []rune(p.text)
			p.text = string(runes[:len(runes)-1])
		}
		p.offered = nil
		return p, nil
	}
	if km.Text != "" && km.Text[0] >= ' ' {
		p.text += km.Text
		p.offered = nil
	}
	return p, nil
}

// candidates asks the prompt's completer for the text typed so far.
func (p promptModel) candidates() []string {
	if p.prompt.Complete == nil {
		return nil
	}
	return p.prompt.Complete(p.text)
}

// completed answers the text after a Tab over candidates: the one candidate
// when only one matches, else the longest prefix they share, with all of them
// offered so the reader sees what to type next.
func completed(text string, candidates []string) (string, []string) {
	switch len(candidates) {
	case 0:
		return text, nil
	case 1:
		return candidates[0], nil
	}
	shared := candidates[0]
	for _, candidate := range candidates[1:] {
		for !strings.HasPrefix(candidate, shared) {
			shared = shared[:len(shared)-1]
		}
	}
	if len(shared) < len(text) {
		shared = text
	}
	return shared, candidates
}

var promptLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("73"))

// View draws the prompt inline, not on the alternate screen, so the question
// and the answer stay in the reader's scrollback above the command that runs.
func (p promptModel) View() tea.View {
	if p.done || p.canceled {
		return tea.NewView("")
	}
	var b textbuf.Buffer
	b.Str(promptLabelStyle.Render(p.prompt.Label))
	if p.prompt.Help != "" {
		b.Str(menuHintStyle.Render(" (" + p.prompt.Help + ")"))
	}
	b.Str(": ").Str(p.text)
	if len(p.offered) != 0 {
		b.Byte('\n').Str(menuHintStyle.Render(strings.Join(p.offered, "  ")))
	}
	return tea.NewView(b.String())
}
