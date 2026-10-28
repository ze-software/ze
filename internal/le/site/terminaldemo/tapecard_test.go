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

// castOutputs returns the text of every output event in a recorded cast, in
// order, after its header line.
func castOutputs(t *testing.T, path string) []string {
	t.Helper()
	content, err := os.ReadFile(path) //nolint:gosec // the test's own temporary cast
	if err != nil {
		t.Fatalf("read cast: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	outputs := make([]string, 0, len(lines))
	for _, line := range lines[1:] {
		var event []any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("cast event %q: %v", line, err)
		}
		text, ok := event[2].(string)
		if !ok {
			t.Fatalf("cast event %q carries no text", line)
		}
		outputs = append(outputs, text)
	}
	return outputs
}

// TestCardCoversTheScreenThenRepaintsIt proves the cast a card produces: the
// card's text, then a repaint of the program's screen, and output the program
// wrote while the card was up held back until the repaint.
// Method: drive the cast writer directly with an editor screen, a card, output
// arriving under the card, and the end of the card; read the events back.
func TestCardCoversTheScreenThenRepaintsIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "card.cast")
	writer, err := newCastWriter(path, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	steps := []func() error{
		func() error { return writer.write("stale\x1b[2J\x1b[31mze#\x1b[0m load") },
		func() error { return writer.showCard("\n  Title\n  body\n") },
		func() error { return writer.write(" merged") },
		writer.restoreScreen,
	}
	for _, step := range steps {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.file.Close(); err != nil {
		t.Fatal(err)
	}
	outputs := castOutputs(t, path)
	if len(outputs) != 3 {
		t.Fatalf("events %q, want the screen, the card, the repaint", outputs)
	}
	if outputs[1] != cardClear+"\r\n  Title\r\n  body\r\n" {
		t.Fatalf("card event %q, want the card on a cleared screen with CR LF line ends", outputs[1])
	}
	if strings.Contains(outputs[1], "merged") {
		t.Fatalf("card event %q carries output written under the card", outputs[1])
	}
	want := cardClear + "\x1b[2J\x1b[31mze#\x1b[0m load merged"
	if outputs[2] != want {
		t.Fatalf("repaint %q, want %q: the screen since its last erase, then what arrived under the card", outputs[2], want)
	}
}

// TestCardRefusesAnUnpairedEnd proves the writer refuses a card shown twice and
// a repaint with no card, so a misordered driver fails rather than recording a
// repaint over a live screen.
func TestCardRefusesAnUnpairedEnd(t *testing.T) {
	writer, err := newCastWriter(filepath.Join(t.TempDir(), "card.cast"), 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = writer.file.Close() }()
	if err := writer.restoreScreen(); err == nil {
		t.Fatal("a repaint with no card on screen was accepted")
	}
	if err := writer.showCard("one"); err != nil {
		t.Fatal(err)
	}
	if err := writer.showCard("two"); err == nil {
		t.Fatal("a second card over the first was accepted")
	}
}

// TestRecorderShowsACardOverARunningProgram proves the Card directive end to
// end through a real pty: the card appears after the program's output, the
// program's screen is repainted right after it, and the program keeps working.
// Method: drive bash with a tape holding a Card between two commands.
func TestRecorderShowsACardOverARunningProgram(t *testing.T) {
	output := filepath.Join(t.TempDir(), "card.cast")
	settings := defaultTapeSettings()
	settings.shell = hermeticShell(t)
	settings.typing = time.Millisecond
	settings.wait = shellWaitTimeout
	// The reading floor has its own test; here it would only slow the run.
	settings.resultHold = 0
	tape := terminalTape{settings: settings, actions: []tapeAction{
		{name: tapeTypeCommand, text: "echo $((40+2))", where: "t:1"},
		{name: "Enter", where: "t:2"},
		{name: tapeWaitCommand, pattern: regexp.MustCompile(`(?m)^42`), where: "t:3"},
		{name: tapeCardCommand, text: "CARD\nTEXT", duration: 200 * time.Millisecond, where: "t:4"},
		{name: tapeTypeCommand, text: "echo $((50+5))", where: "t:5"},
		{name: "Enter", where: "t:6"},
		{name: tapeWaitCommand, pattern: regexp.MustCompile(`(?m)^55`), where: "t:7"},
	}}
	if _, err := driveTape(tape, output, 80, 24); err != nil {
		t.Fatalf("drive tape: %v", err)
	}
	outputs := castOutputs(t, output)
	cardAt := -1
	for index, text := range outputs {
		if strings.Contains(text, "CARD\r\nTEXT") {
			cardAt = index
			break
		}
	}
	if cardAt < 1 {
		t.Fatalf("no card event after the first output in %q", outputs)
	}
	if !strings.Contains(strings.Join(outputs[:cardAt], ""), "42") {
		t.Fatalf("the program's 42 was not on screen before the card: %q", outputs[:cardAt])
	}
	if cardAt+1 >= len(outputs) {
		t.Fatalf("nothing after the card: %q", outputs)
	}
	repaint := outputs[cardAt+1]
	if !strings.HasPrefix(repaint, cardClear) {
		t.Fatalf("event after the card %q is not a repaint", repaint)
	}
	if !strings.Contains(repaint, "42") {
		t.Fatalf("repaint %q does not redraw the program's screen", repaint)
	}
	if !strings.Contains(strings.Join(outputs[cardAt+1:], ""), "55") {
		t.Fatalf("the program did not answer after the card: %q", outputs[cardAt+1:])
	}
}

// TestTapeParserHoldsTheCardRules proves the parser's card rules: a section
// load needs a Card since the previous load, a Card names a card cards.json
// holds, and a Card inside Hide is refused because nobody would see it.
// Method: parse one tape per rule, and one that keeps every rule.
func TestTapeParserHoldsTheCardRules(t *testing.T) {
	load := `Type "load file absolute merge /src/a.conf"`
	for _, testCase := range []struct {
		name string
		text string
		want string
	}{
		{name: "section without card", text: load + "\nEnter\n", want: "loads a section with no Card"},
		{name: "second section reuses the first card", text: "Card topic-irr server\n" + load + "\nEnter\n" + load + "\n", want: "loads a section with no Card"},
		{name: "unknown card", text: "Card topic-irr nothing\n", want: "does not hold"},
		{name: "card needs a phase", text: "Card topic-irr\n", want: "needs a demo and a phase"},
		{name: "card while hidden", text: "Hide\nCard topic-irr server\nShow\n", want: "never seen"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "demo.tape")
			mustWrite(t, path, testCase.text)
			_, err := parseTape(path, "")
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("parse error = %v, want %q", err, testCase.want)
			}
		})
	}
	path := filepath.Join(t.TempDir(), "demo.tape")
	mustWrite(t, path, "Card topic-irr server\n"+load+"\nEnter\nCard topic-irr as-set\n"+load+"\n")
	tape, err := parseTape(path, "")
	if err != nil {
		t.Fatalf("a carded section tape was refused: %v", err)
	}
	if tape.actions[0].text != cards["topic-irr:server"] || tape.actions[0].duration != cardHold(cards["topic-irr:server"]) {
		t.Fatalf("card action %#v does not carry the card's text and hold", tape.actions[0])
	}
}

// TestCardHoldGrowsWithTheWords proves a card is held long enough to read: the
// base moment plus a fixed time per word.
func TestCardHoldGrowsWithTheWords(t *testing.T) {
	if got := cardHold(""); got != cardHoldBase {
		t.Fatalf("empty card held %v, want %v", got, cardHoldBase)
	}
	if got := cardHold("  one two\nthree "); got != cardHoldBase+3*cardHoldPerWord {
		t.Fatalf("three-word card held %v, want %v", got, cardHoldBase+3*cardHoldPerWord)
	}
}

// TestTopicOverviewCardNamesEverySection proves each showcase topic's overview
// card, printed before the editor opens, names every section the topic's
// fragments load, by the title of that section's card.
// Method: for each directory under demos/terminal/topics, parse every fragment,
// collect its Card actions, and find each card's title line in
// cards["topic-<dir>:overview"].
func TestTopicOverviewCardNamesEverySection(t *testing.T) {
	demoTreePath := filepath.Join("..", "..", "..", "..", "demos", "terminal")
	topics, err := filepath.Glob(filepath.Join(demoTreePath, "topics", "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(topics) == 0 {
		t.Fatalf("no topic under %s", demoTreePath)
	}
	for _, topic := range topics {
		name := "topic-" + filepath.Base(topic)
		overview, ok := cards[name+":overview"]
		if !ok {
			t.Errorf("%s: no %s:overview card in cards.json", topic, name)
			continue
		}
		fragments, err := filepath.Glob(filepath.Join(topic, "*.tape"))
		if err != nil {
			t.Fatal(err)
		}
		sections := 0
		for _, fragment := range fragments {
			tape, err := parseTape(fragment, demoTreePath)
			if err != nil {
				t.Errorf("%s: %v", fragment, err)
				continue
			}
			for _, action := range tape.actions {
				if action.name != tapeCardCommand {
					continue
				}
				sections++
				title := strings.TrimSpace(strings.SplitN(strings.TrimSpace(action.text), "\n", 2)[0])
				if !strings.Contains(overview, title) {
					t.Errorf("%s: the %s overview card does not name the section %q", action.where, name, title)
				}
			}
		}
		if sections == 0 {
			t.Errorf("%s: no fragment carries a section Card", topic)
		}
	}
}
