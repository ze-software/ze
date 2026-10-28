package siteterminaldemo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// castEvent is one output event of a cast: its time and its text.
type castEvent struct {
	at   float64
	text string
}

// castEvents reads the timed output events of a cast the test recorded.
func castEvents(t *testing.T, path string) []castEvent {
	t.Helper()
	content, err := os.ReadFile(path) //nolint:gosec // the test's own temporary cast
	if err != nil {
		t.Fatalf("read cast: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	events := make([]castEvent, 0, len(lines))
	for _, line := range lines[1:] {
		var event []any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("cast event %q: %v", line, err)
		}
		at, isTime := event[0].(float64)
		text, isText := event[2].(string)
		if !isTime || !isText {
			t.Fatalf("cast event %q is not [time, type, text]", line)
		}
		events = append(events, castEvent{at: at, text: text})
	}
	return events
}

// TestRecorderHoldsAWaitedResultBeforeTheNextCommand proves the reading floor
// end to end through a real pty: the next command is not typed until the
// result the tape waited for has been on screen for the floor. Before it, the
// recorder typed the next command as soon as the wait matched, so a section
// load or a commit result flashed past.
// Method: drive bash with two commands and no Sleep between them, then compare
// the time the first result was painted with the time the second was typed.
func TestRecorderHoldsAWaitedResultBeforeTheNextCommand(t *testing.T) {
	const hold = 700 * time.Millisecond
	output := filepath.Join(t.TempDir(), "hold.cast")
	settings := defaultTapeSettings()
	settings.shell = hermeticShell(t)
	settings.typing = time.Millisecond
	settings.wait = shellWaitTimeout
	settings.resultHold = hold
	tape := terminalTape{settings: settings, actions: []tapeAction{
		{name: tapeTypeCommand, text: "echo $((40+2))", where: "t:1"},
		{name: "Enter", where: "t:2"},
		{name: tapeWaitCommand, pattern: regexp.MustCompile(`(?m)^42`), where: "t:3"},
		{name: tapeTypeCommand, text: "echo $((50+5))", where: "t:4"},
		{name: "Enter", where: "t:5"},
		{name: tapeWaitCommand, pattern: regexp.MustCompile(`(?m)^55`), where: "t:6"},
	}}
	if _, err := driveTape(tape, output, 80, 24); err != nil {
		t.Fatalf("drive tape: %v", err)
	}
	painted, typed := -1.0, -1.0
	for _, event := range castEvents(t, output) {
		if painted < 0 && strings.Contains(event.text, "42") && !strings.Contains(event.text, "40+2") {
			painted = event.at
		}
		// Typing echoes one character an event, so the second command starts
		// with the event that opens with its first letter.
		if typed < 0 && painted >= 0 && event.at > painted && strings.HasPrefix(event.text, "e") {
			typed = event.at
		}
	}
	if painted < 0 || typed < 0 {
		t.Fatalf("the cast holds no painted 42 followed by typing: painted=%v typed=%v", painted, typed)
	}
	if gap := time.Duration((typed - painted) * float64(time.Second)); gap < hold {
		t.Fatalf("the next command was typed %v after the result was painted, want at least %v", gap, hold)
	}
}

// TestTypedCardEarnsItsReadingTime proves a `ze-demo card` typed in the outer
// shell earns the same reading time as the in-editor Card of that card, that a
// plain command earns none, and that a card cards.json does not hold is
// refused at parse time rather than failing mid-recording.
// Method: parse one tape per case and read the Type action's duration.
func TestTypedCardEarnsItsReadingTime(t *testing.T) {
	for _, testCase := range []struct {
		name string
		text string
		want time.Duration
	}{
		{name: "outer card", text: "ze-demo card topic-irr server", want: cardHold(cards["topic-irr:server"])},
		{name: "plain command", text: "echo ze-demo card topic-irr server", want: 0},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "demo.tape")
			mustWrite(t, path, `Type "`+testCase.text+`"`+"\n")
			tape, err := parseTape(path, "")
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := tape.actions[0].duration; got != testCase.want {
				t.Fatalf("hold = %v, want %v", got, testCase.want)
			}
		})
	}
	if cardHold(cards["topic-irr:server"]) <= resultHold {
		t.Fatal("the fixture card is too short to tell its own hold from the result floor")
	}
	path := filepath.Join(t.TempDir(), "demo.tape")
	mustWrite(t, path, `Type "ze-demo card topic-irr nothing"`+"\n")
	if _, err := parseTape(path, ""); err == nil || !strings.Contains(err.Error(), "does not hold") {
		t.Fatalf("parse error = %v, want the refusal naming the missing card", err)
	}
}
