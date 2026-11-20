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
