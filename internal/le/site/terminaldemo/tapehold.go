// Design: docs/contributing/terminal-demos.md -- every screen stays up long enough to read
// Related: pty.go -- driveTape, which calls these between actions
// Related: tapecard.go -- cardHold, the reading time a card earns from its words

package siteterminaldemo

import (
	"fmt"
	"regexp"
	"time"
)

// resultHold is the least time a result the tape waited for stays on screen
// before the next action may change it: long enough to see what changed and
// read its key line. Without it a section load or a commit result showed for
// about a second before the next card covered it. A tape Sleeps longer where
// a screen has more to read, and that Sleep counts toward the floor.
const resultHold = 4 * time.Second

// outerCardPattern matches the shell command that prints a card from
// cards.json in the outer shell, capturing its demo and phase.
var outerCardPattern = regexp.MustCompile(`^ze-demo card (\S+) (\S+)$`)

// outerCardHold answers the reading time of the card a typed shell command
// prints, or zero for a command that prints no card. A card cards.json does
// not hold is refused: `ze-demo card` would fail on it at recording time.
func outerCardHold(typed string) (time.Duration, error) {
	match := outerCardPattern.FindStringSubmatch(typed)
	if match == nil {
		return 0, nil
	}
	key := match[1] + ":" + match[2]
	text, ok := cards[key]
	if !ok {
		return 0, fmt.Errorf("prints the card %q, which internal/le/site/terminaldemo/cards.json does not hold; add the card there", key)
	}
	return cardHold(text), nil
}

// hidden answers whether a Hide is in force, so nothing reaches the viewer.
func (writer *castWriter) hidden() bool { return !writer.hiddenAt.IsZero() }

// holdResult starts the reading floor for what a Wait+Screen just saw on a
// visible screen: resultHold, or the card's own reading time when the typed
// command printed a card. Nothing is held while the recording is hidden,
// because the viewer never sees that screen.
func (session *tapeSession) holdResult() {
	hold := max(session.resultHold, session.cardHold)
	session.cardHold = 0
	if session.writer.hidden() {
		return
	}
	session.readyAt = time.Now().Add(hold)
}

// awaitReading keeps the screen as it is until the reading floor has passed.
// The program keeps running meanwhile, so its output still reaches the cast.
func (session *tapeSession) awaitReading() error {
	if session.writer.hidden() {
		return nil
	}
	remaining := time.Until(session.readyAt)
	if remaining <= 0 {
		return nil
	}
	return session.pump(remaining)
}
