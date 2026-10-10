// Design: docs/contributing/terminal-demos.md -- a card explains each section as it loads
// Related: pty.go -- the tape parser and the cast writer this directive extends
// Related: runtime.go -- printCard, the outer-shell card the overview uses

package siteterminaldemo

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// tapeCardCommand names the tape directive that shows a card over a program
// that holds the terminal. `ze-demo card` prints from the outer shell, so it
// cannot run while the SSH editor is open; the recorder writes this card into
// the cast instead, and the editor never sees it.
const tapeCardCommand = "Card"

const (
	// cardHoldBase and cardHoldPerWord set how long a card stays on screen:
	// a fixed moment to notice the card, then 200 words a minute to read it.
	cardHoldBase    = 2 * time.Second
	cardHoldPerWord = 300 * time.Millisecond

	// cardClear resets the colors, homes the cursor and erases the screen, so
	// a card and the repaint after it each start from a blank screen.
	cardClear = "\x1b[0m\x1b[H\x1b[2J"
)

// sectionLoadPattern matches a typed command that loads one configuration
// section into the editor's candidate: `load file absolute merge <file>` and
// every other `load ... merge` form.
var sectionLoadPattern = regexp.MustCompile(`^load\s.*\bmerge\s`)

// tapeCard resolves a `Card DEMO PHASE` argument to the card's text, from the
// same cards.json that `ze-demo card DEMO PHASE` prints, and to the time the
// card is held on screen.
func tapeCard(argument string) (string, time.Duration, error) {
	fields := strings.Fields(argument)
	if len(fields) != 2 {
		return "", 0, fmt.Errorf("needs a demo and a phase, as `Card topic-irr server`, got %q", argument)
	}
	key := fields[0] + ":" + fields[1]
	text, ok := cards[key]
	if !ok {
		return "", 0, fmt.Errorf("names %q, which internal/le/site/terminaldemo/cards.json does not hold; add the card there", key)
	}
	return text, cardHold(text), nil
}

// cardHold answers how long a card stays on screen, from its word count.
func cardHold(text string) time.Duration {
	return cardHoldBase + time.Duration(len(strings.Fields(text)))*cardHoldPerWord
}

// requireSectionCards refuses a tape that loads a configuration section with no
// Card since the previous section load. The owner's first decision is that each
// section is explained as it is loaded (docs/contributing/terminal-demos.md),
// so a section without its card is a recording that breaks that decision.
func requireSectionCards(actions []tapeAction) error {
	carded := false
	for _, action := range actions {
		if action.name == tapeCardCommand {
			carded = true
			continue
		}
		if action.name != tapeTypeCommand {
			continue
		}
		if !sectionLoadPattern.MatchString(action.text) {
			continue
		}
		if !carded {
			return fmt.Errorf("%s: %q loads a section with no Card before it; each section is explained as it loads, so add `Card <demo> <phase>` above this Type", action.where, action.text)
		}
		carded = false
	}
	return nil
}

// card shows a card over whatever program holds the terminal, holds it while
// the program keeps running, then repaints the program's screen. The card and
// the repaint go into the cast only: the program never sees them, so the
// editor's candidate and its screen state are untouched.
func (session *tapeSession) card(action tapeAction) error {
	if err := session.writer.showCard(action.text); err != nil {
		return err
	}
	if err := session.pump(action.duration); err != nil {
		return err
	}
	return session.writer.restoreScreen()
}

// paint emits live terminal output and keeps it for the repaint after a card.
func (writer *castWriter) paint(text string) error {
	writer.remember(text)
	return writer.emit(text)
}

// remember keeps the live output written since the last screen erase. Replayed
// onto a blank screen it redraws what the program painted, colors included,
// because it is the same byte stream the terminal drew it from. The replay is
// bounded by the recording: an erase restarts it.
func (writer *castWriter) remember(text string) {
	matches := erasePattern.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		writer.replay = append(writer.replay, text)
		return
	}
	writer.replay = []string{text[matches[len(matches)-1][0]:]}
}

// showCard covers the screen with a card. MUST be followed by restoreScreen.
func (writer *castWriter) showCard(text string) error {
	if writer.carded {
		return errors.New("a card is already on screen")
	}
	writer.carded = true
	// The cast holds no terminal driver, so the line feeds a printed card gets
	// from the pty's output processing are written here.
	return writer.emit(cardClear + strings.ReplaceAll(text, "\n", "\r\n"))
}

// restoreScreen ends a card. MUST be called after showCard: it erases the card
// and replays the program's screen, with what the program wrote while the card
// was up.
func (writer *castWriter) restoreScreen() error {
	if !writer.carded {
		return errors.New("no card is on screen to remove")
	}
	writer.carded = false
	return writer.emit(cardClear + strings.Join(writer.replay, ""))
}
