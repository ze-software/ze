// Design: docs/architecture/api/commands.md -- the pipe chain on the stream route.

package command

import (
	"bytes"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// streamOpNames names each operation of ops by its catalog entry.
func streamOpNames(t *testing.T, ops []pipeOp) []string {
	t.Helper()
	names := make([]string, 0, len(ops))
	for _, op := range ops {
		entry, known := lookupPipeOperatorByKind(op.kind)
		if !known {
			t.Fatalf("operation kind %d is not in the catalog", op.kind)
		}
		names = append(names, entry.Name)
	}
	return names
}

// TestPresentationSuffixSameOnClientAndDaemon asks whether the daemon and a
// client that set the rendering key split each line of the design's table in
// one place: the daemon applies everything before the presentation suffix,
// the client the suffix alone, so no operation is applied twice or by nobody
// (AC-36, R-10). The alias row and a name the client has never heard of are
// included; the unknown-name row shows the client refuses nothing the daemon
// classifies.
//
// Method: ReadStreamLine under StreamRenderingClientPresentation gives the
// daemon's part; presentationSuffixLength over the typed chain gives the
// client's. Their names, joined, must equal the residual chain.
func TestPresentationSuffixSameOnClientAndDaemon(t *testing.T) {
	const prefix = "monitor test presentation-suffix"
	t.Cleanup(ResetPipeFiltersForTest)
	RegisterPipeFilters([]string{prefix},
		PipeFilter{Name: "top-talkers", Description: "The busiest peers", Argument: ArgNone})
	RegisterAliases([]string{prefix}, Alias{Name: "brief", Description: "Short events", Expansion: "json"})
	t.Cleanup(func() { RegisterAliases([]string{prefix}) })
	save := filepath.Join(t.TempDir(), "f")

	rows := []struct {
		chain  string
		daemon []string
		client []string
	}{
		{"| match up | save " + save, []string{"match"}, []string{"save"}},
		{"| match up | table | save " + save, []string{"match"}, []string{"table", "save"}},
		{"| json | match up", []string{"json", "match"}, nil},
		{"| top-talkers | save " + save, nil, []string{"save"}},
		{"| brief | save " + save, []string{"json"}, []string{"save"}},
	}
	for _, row := range rows {
		t.Run(row.chain, func(t *testing.T) {
			input := prefix + " " + row.chain
			line, err := ReadStreamLine(input, StreamRenderingClientPresentation)
			if err != nil {
				t.Fatalf("the daemon refused %q: %v", input, err)
			}
			_, chain, err := scanExecution(input)
			if err != nil {
				t.Fatal(err)
			}
			typed := chain.operations
			var client []string
			for _, op := range typed[len(typed)-presentationSuffixLength(typed):] {
				client = append(client, op[0])
			}
			daemon := streamOpNames(t, line.applied)
			if !slices.Equal(daemon, row.daemon) {
				t.Errorf("daemon applies %q, want %q", daemon, row.daemon)
			}
			if !slices.Equal(client, row.client) {
				t.Errorf("client presents %q, want %q", client, row.client)
			}
			if whole := streamOpNames(t, line.part.residual); !slices.Equal(append(daemon, client...), whole) {
				t.Errorf("daemon %q + client %q is not the residual chain %q", daemon, client, whole)
			}
		})
	}

	t.Run("a name the client does not know", func(t *testing.T) {
		input := prefix + " | bogus | save " + save
		if _, err := ReadStreamLine(input, StreamRenderingClientPresentation); err == nil {
			t.Error("the daemon accepted an operation the monitor does not declare")
		}
		_, saves, msg := PresentStreamSuffix(input, func(s string) string { return s })
		if msg != "" {
			t.Errorf("the client refused a name it leaves to the daemon: %s", msg)
		}
		if saves != nil {
			if err := saves.Abort(); err != nil {
				t.Fatal(err)
			}
		}
	})
}

// TestStreamLineAppliesChainAndConsumesLog asks whether the daemon's part of
// a monitor chain renders every event once, `| log` consumed, when the
// producer splits an event across writes. It is the formatting half the SSH
// exec channel ran before the chain moved to the daemon.
func TestStreamLineAppliesChainAndConsumesLog(t *testing.T) {
	line, err := ReadStreamLine("monitor test stream-line peer Blue | json | log", StreamRenderingTerminal)
	if err != nil {
		t.Fatal(err)
	}
	if got := line.Words(); !slices.Equal(got, []string{"monitor", "test", "stream-line", "peer", "Blue"}) {
		t.Errorf("command words = %q", got)
	}
	var output bytes.Buffer
	err = line.Run(&output, func(w io.Writer) error {
		if _, err := io.WriteString(w, `{"event":"fir`); err != nil {
			return err
		}
		_, err := io.WriteString(w, "st\"}\n{\"event\":\"second\"}")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := "{\n  \"event\": \"first\"\n}\n{\n  \"event\": \"second\"\n}\n"; output.String() != want {
		t.Errorf("output = %q, want %q", output.String(), want)
	}
}

// TestStreamLineMatchKeepsWholeEvents asks whether `| match` on a stream
// treats each event as one row of the stream: a monitor event that is one
// document (the BGP OPEN and state events, internal/component/bgp/format
// text_json.go) is kept whole or removed whole, where the same document as a
// one-shot answer is refused, and the header line is matched as text.
//
// Method: the real ReadStreamLine and Run over the three lines `monitor event
// include open,state` writes, under the rendering key (the daemon applies
// `match` alone, so the kept events stay data, one line each) and on a terminal
// (the default format follows). MUTATION: drop the pipeSurfaceStream arm of
// the match case in applyPipesWithRequest; the first JSON event is refused
// with "match needs rows ... it holds one document" and Run fails.
func TestStreamLineMatchKeepsWholeEvents(t *testing.T) {
	const (
		header = "monitoring: include=open,state"
		open   = `{"type":"bgp","bgp":{"message":{"type":"open"},"peer":{"address":"127.0.0.1"},"open":{"asn":65001}}}`
		state  = `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"127.0.0.1"},"state":"up"}}`
	)
	produce := func(w io.Writer) error {
		_, err := io.WriteString(w, header+"\n"+open+"\n"+state+"\n")
		return err
	}
	const input = "monitor event include open,state | match state"

	presented, err := ReadStreamLine(input, StreamRenderingClientPresentation)
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := presented.Run(&data, produce); err != nil {
		t.Fatalf("the presenting client's stream failed: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(data.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("presented stream = %q, want the header and the state event", data.String())
	}
	if lines[0] != header {
		t.Errorf("presented header = %q, want %q", lines[0], header)
	}
	if !strings.Contains(lines[1], `"state":"up"`) {
		t.Errorf("presented event = %q, want the state event as data", lines[1])
	}
	if strings.Contains(data.String(), "65001") {
		t.Errorf("presented stream %q kept the OPEN event match removed", data.String())
	}

	rendered, err := ReadStreamLine(input, StreamRenderingTerminal)
	if err != nil {
		t.Fatal(err)
	}
	var text bytes.Buffer
	if err := rendered.Run(&text, produce); err != nil {
		t.Fatalf("the terminal stream failed: %v", err)
	}
	if !strings.Contains(text.String(), "up") {
		t.Errorf("terminal stream %q lost the state event", text.String())
	}
	if strings.Contains(text.String(), "65001") {
		t.Errorf("terminal stream %q kept the OPEN event match removed", text.String())
	}

	// The negative: the same document answered by a one-shot command is the
	// whole answer, not a row, and match still refuses it by name.
	if _, msg := applyMatch(state, "state"); !strings.Contains(msg, "it holds one document") {
		t.Errorf("one-shot match over one document = %q, want the refusal", msg)
	}
}

// TestStreamLineMatchNeverSplitsAnEvent asks whether `| match` on a stream
// keeps an event whole even when its body reads as rows: a BGP OPEN event's
// body is a map whose values are all objects, which the one-shot row finder
// takes for rows keyed by identity, and an event holding a real row set is
// still one row of the stream.
//
// Method: the real ReadStreamLine and Run under the rendering key, so the kept
// events stay data. MUTATION: match an event's rows through rowsInKeyed and
// matchRows in applyEventMatch; `| match 127.0.0.1` answers the OPEN event
// without its message and open members, and `| match up` sends the `down`
// event as an empty peer list.
func TestStreamLineMatchNeverSplitsAnEvent(t *testing.T) {
	const (
		open = `{"type":"bgp","bgp":{"message":{"type":"open"},"peer":{"address":"127.0.0.1"},"open":{"asn":65001}}}`
		up   = `{"peers":[{"state":"up"}]}`
		down = `{"peers":[{"state":"down"}]}`
	)
	run := func(input, events string) string {
		t.Helper()
		presented, err := ReadStreamLine(input, StreamRenderingClientPresentation)
		if err != nil {
			t.Fatal(err)
		}
		var data bytes.Buffer
		if err := presented.Run(&data, func(w io.Writer) error {
			_, err := io.WriteString(w, events)
			return err
		}); err != nil {
			t.Fatalf("%s: the stream failed: %v", input, err)
		}
		return data.String()
	}

	kept := run("monitor event | match 127.0.0.1", open+"\n")
	for _, member := range []string{`"type":"open"`, `"asn":65001`, `"address":"127.0.0.1"`} {
		if !strings.Contains(kept, member) {
			t.Errorf("match kept %q, want the whole OPEN event with %s", kept, member)
		}
	}

	selected := run("monitor event | match up", up+"\n"+down+"\n")
	if strings.Count(strings.TrimSuffix(selected, "\n"), "\n") != 0 {
		t.Fatalf("match up answered %q, want the up event alone", selected)
	}
	if !strings.Contains(selected, `"state":"up"`) {
		t.Errorf("match up answered %q, want the up event", selected)
	}
}

// TestStreamLineRefusesDaemonSave asks whether `| save` is refused on the
// daemon's part of a stream, and left to a client that presents (AC-22).
func TestStreamLineRefusesDaemonSave(t *testing.T) {
	input := "monitor test stream-line | save " + filepath.Join(t.TempDir(), "f")
	for _, rendering := range []StreamRendering{StreamRenderingEngine, StreamRenderingTerminal} {
		_, err := ReadStreamLine(input, rendering)
		if err == nil || !strings.Contains(err.Error(), "save") {
			t.Errorf("rendering %d: err = %v, want the save refused", rendering, err)
		}
	}
	if _, err := ReadStreamLine(input, StreamRenderingClientPresentation); err != nil {
		t.Errorf("the presenting client's save was refused: %v", err)
	}
}

// TestStreamLineWithoutChain asks what each rendering does to an event when
// the line has no chain: the engine passes it unchanged, a terminal renders
// it in the default format.
func TestStreamLineWithoutChain(t *testing.T) {
	const event = "{\"peers\":[{\"state\":\"up\"}]}"
	for _, row := range []struct {
		rendering StreamRendering
		unchanged bool
	}{
		{StreamRenderingEngine, true},
		{StreamRenderingClientPresentation, true},
		{StreamRenderingTerminal, false},
	} {
		line, err := ReadStreamLine("monitor test stream-line", row.rendering)
		if err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		if err := line.Run(&output, func(w io.Writer) error {
			_, err := io.WriteString(w, event+"\n")
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if unchanged := output.String() == event+"\n"; unchanged != row.unchanged {
			t.Errorf("rendering %d: output %q, unchanged = %v, want %v", row.rendering, output.String(), unchanged, row.unchanged)
		}
	}
}

// TestStreamLineHoldsOverrideEventsToShape asks whether the stream route
// holds each event of a command's override to the shape of the generic it
// overrides, as Dispatch and ServeLocal hold the one answer (AC-17): a
// `count` override's event is one object holding an integer count, a rows
// override's event is one JSON document, and an event of another shape stops
// the stream with a refusal naming the command, what the event was and the
// command that explains the override, the wrong event never written. An override followed by an operation of the command's
// own, and a line with no declared operation, are not judged.
//
// Method: the real ReadStreamLine and Run, under every rendering, over a
// producer that writes one event of the right shape and then one of the
// wrong shape. Under the rendering key the daemon applies nothing, so the
// output is exactly the events that passed.
// MUTATION: make StreamLine.Run call produce without overrideEventJudge, or
// make the judge answer nil, and the wrong events are written with no error.
func TestStreamLineHoldsOverrideEventsToShape(t *testing.T) {
	const prefix = "monitor test override-shape"
	t.Cleanup(ResetPipeFiltersForTest)
	RegisterPipeFilters([]string{prefix},
		PipeFilter{Name: "count", Description: "Count the events at the source"},
		PipeFilter{Name: "match", Description: "Match at the source", Argument: ArgText},
		PipeFilter{Name: "first", Description: "The first events at the source", Argument: ArgCount},
		PipeFilter{Name: "top-talkers", Description: "The busiest peers", Argument: ArgNone})

	renderings := []StreamRendering{StreamRenderingEngine, StreamRenderingTerminal, StreamRenderingClientPresentation}
	for _, row := range []struct {
		name     string
		chain    string
		good     string
		wrong    string
		refused  bool
		answered string
	}{
		{name: "count, a route event", chain: "| count", good: `{"count":3}`, wrong: `{"prefix":"10.0.0.0/8"}`, refused: true, answered: "an event with no integer count"},
		{name: "count, a fraction", chain: "| count", good: `{"count":3}`, wrong: `{"count":1.5}`, refused: true, answered: "an event with no integer count"},
		{name: "count, an array", chain: "| count", good: `{"count":3}`, wrong: `[3]`, refused: true, answered: "an event that is not one object"},
		{name: "count, a sentence", chain: "| count", good: `{"count":3}`, wrong: `three events`, refused: true, answered: "an event that is not one JSON document"},
		{name: "match, a sentence", chain: "| match up", good: `{"peer":"Blue","state":"up"}`, wrong: `peer Blue is up`, refused: true, answered: "an event that is not one JSON document"},
		{name: "first, two documents in one event", chain: "| first 1", good: `{"peer":"Blue"}`, wrong: `{"peer":"Blue"} {"peer":"Red"}`, refused: true, answered: "an event that is not one JSON document"},
		{name: "first, more events than asked", chain: "| first 1", good: `{"peer":"Blue"}`, wrong: `{"peer":"Red"}`},
		{name: "count followed by an own operation", chain: "| count | top-talkers", good: `{"count":3}`, wrong: `{"prefix":"10.0.0.0/8"}`},
		{name: "no declared operation", chain: "", good: `{"count":3}`, wrong: `{"prefix":"10.0.0.0/8"}`},
	} {
		for _, rendering := range renderings {
			line, err := ReadStreamLine(prefix+" "+row.chain, rendering)
			if err != nil {
				t.Fatalf("%s, rendering %d: %v", row.name, rendering, err)
			}
			var output bytes.Buffer
			err = line.Run(&output, func(w io.Writer) error {
				_, writeErr := io.WriteString(w, row.good+"\n"+row.wrong+"\n")
				return writeErr
			})
			written := output.String()
			if !row.refused {
				if err != nil {
					t.Errorf("%s, rendering %d: refused: %v", row.name, rendering, err)
				}
				if rendering == StreamRenderingClientPresentation && written != row.good+"\n"+row.wrong+"\n" {
					t.Errorf("%s: output %q, want both events unchanged", row.name, written)
				}
				continue
			}
			if err == nil {
				t.Errorf("%s, rendering %d: the wrong event was accepted, output %q", row.name, rendering, written)
				continue
			}
			text := err.Error()
			if !strings.Contains(text, prefix) || !strings.Contains(text, row.answered) {
				t.Errorf("%s, rendering %d: refusal %q does not name the command and %q", row.name, rendering, text, row.answered)
			}
			if !strings.Contains(text, `show command help "`+prefix+`"`) {
				t.Errorf("%s, rendering %d: refusal %q does not end in the command that explains the override", row.name, rendering, text)
			}
			if rendering == StreamRenderingClientPresentation && written != row.good+"\n" {
				t.Errorf("%s: output %q, want only the event that kept the shape", row.name, written)
			}
		}
	}
}

// TestStreamLineOverrideRefusalNamesDeclaredPath asks whether the refusal of
// an override's event names the path the operation is declared at, when the
// line carries arguments after it: the fix line is a command an operator can
// run, and the arguments typed are not part of the command's name.
func TestStreamLineOverrideRefusalNamesDeclaredPath(t *testing.T) {
	const prefix = "monitor test override-path"
	t.Cleanup(ResetPipeFiltersForTest)
	RegisterPipeFilters([]string{prefix}, PipeFilter{Name: "count", Description: "Count the events at the source"})
	line, err := ReadStreamLine(prefix+" peer Blue | count", StreamRenderingClientPresentation)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err = line.Run(&output, func(w io.Writer) error {
		_, writeErr := io.WriteString(w, "{\"total\":5}\n")
		return writeErr
	})
	if err == nil {
		t.Fatalf("a count event with no count was accepted, output %q", output.String())
	}
	if !strings.Contains(err.Error(), `show command help "`+prefix+`"`) {
		t.Errorf("refusal %q does not name the declared path %q in its fix", err, prefix)
	}
	if output.Len() != 0 {
		t.Errorf("output %q, want nothing written", output.String())
	}
}

// presentedBy names which formatter PresentStreamSuffix answered for a line.
type presentedBy uint8

const (
	presentedByUnspecified presentedBy = iota
	// presentedByDefault: the client's own default formatter.
	presentedByDefault
	// presentedBySuffix: the format the suffix names, never the default.
	presentedBySuffix
	// presentedByIdentity: the event unchanged, because the daemon already
	// rendered the format typed before the suffix.
	presentedByIdentity
)

// TestPresentStreamSuffixChoosesTheFormatter asks which formatter a client
// that presents a stream applies to each event: its default when the line
// names no format, the suffix's format when the suffix names one, and the
// event unchanged when a format typed before the suffix was rendered by the
// daemon. A selecting operation alone (`| match`) leaves the default.
//
// Method: PresentStreamSuffix over each line with a marking default; the
// answer for a sample event is compared with the default's mark, with the
// event itself, and with neither.
func TestPresentStreamSuffixChoosesTheFormatter(t *testing.T) {
	const event = `{"items":[{"key":"value"}]}`
	const mark = "DEFAULT:"
	defaultFn := func(s string) string { return mark + s }

	for _, row := range []struct {
		input string
		want  presentedBy
	}{
		{"monitor event", presentedByDefault},
		{"monitor event | json", presentedBySuffix},
		{"monitor event | table", presentedBySuffix},
		{"monitor event | text", presentedBySuffix},
		{"monitor event | yaml", presentedBySuffix},
		{"monitor event | match state", presentedByDefault},
		{"monitor event | json | match up", presentedByIdentity},
	} {
		t.Run(row.input, func(t *testing.T) {
			format, saves, msg := PresentStreamSuffix(row.input, defaultFn)
			if msg != "" {
				t.Fatalf("the client refused %q: %s", row.input, msg)
			}
			t.Cleanup(func() {
				if err := saves.Abort(); err != nil {
					t.Error(err)
				}
			})
			got := format(event)
			switch row.want {
			case presentedByDefault:
				if got != mark+event {
					t.Errorf("presented %q, want the default formatter's %q", got, mark+event)
				}
			case presentedBySuffix:
				if strings.HasPrefix(got, mark) {
					t.Errorf("presented %q through the default, want the suffix's format", got)
				}
				if got == event {
					t.Errorf("presented the event unchanged, want the suffix's format")
				}
			case presentedByIdentity:
				if got != event {
					t.Errorf("presented %q, want the daemon's rendering %q unchanged", got, event)
				}
			case presentedByUnspecified:
				t.Fatal("BUG: a row states no expected formatter")
			}
		})
	}
}
