package hub

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// matchUndeclaredWord is a word no command declares as a keyword, so a call
// carrying it is refused by the argument validator on both routes.
const matchUndeclaredWord = "ze-match-no-such-word"

// TestLocalCommandDaemonMatchesLocal runs every local-data command the daemon
// serves through the local route and through the daemon dispatcher, and
// compares what an operator reads.
//
// VALIDATES: AC-4. The commands are the local-data registrations whose path
// the dispatcher pluginserver.NewServer builds serves under the same path, so
// a new served command is checked with no edit here. Each is called with the
// arguments its YANG leaves admit (every mandatory leaf gets a value its
// declaration accepts), once through command.ServeLocal with `| json` and
// once through Dispatcher.Dispatch, whose payload is rendered by the same
// `| json` chain. Either both routes answer and the JSON is identical, or
// both refuse and the refusal reads the same bytes: the local Diagnostic on
// stderr against the SSH rendering of the dispatch error. A second call per
// command carries a word no leaf declares, and both routes must refuse it in
// the same bytes.
// PREVENTS: the daemon route answering a command other than the local route
// does, or refusing it in other words.
func TestLocalCommandDaemonMatchesLocal(t *testing.T) {
	// The commands run for real, and some write state (delete debug profile
	// name opens debug.zefs in the working directory); a directory of the
	// test's own keeps that out of the checkout. It is also the directory the
	// daemon route confines file arguments to.
	t.Chdir(t.TempDir())

	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	if err != nil {
		t.Fatalf("daemon dispatcher: %v", err)
	}
	t.Cleanup(server.Stop)
	d := server.Dispatcher()

	checked := 0
	for _, entry := range command.ListLocal() {
		served := d.Lookup(entry.Path)
		if served == nil || served.Name != entry.Path {
			continue
		}
		handler, rest := command.LookupLocalData(strings.Fields(entry.Path))
		if handler == nil || len(rest) != 0 {
			continue
		}
		checked++
		input := entry.Path
		if args := matchMandatoryArgs(served.ArgDefs); args != "" {
			input += " " + args
		}
		t.Run(entry.Path, func(t *testing.T) {
			matchRoutes(t, d, input)
			matchRoutes(t, d, input+" "+matchUndeclaredWord)
		})
	}
	if checked == 0 {
		t.Fatal("the dispatcher serves no local-data command: the bridge loaded nothing")
	}
}

// matchRoutes runs input on both routes and fails when an operator would read
// a different answer or a different refusal.
func matchRoutes(t *testing.T, d *pluginserver.Dispatcher, input string) {
	t.Helper()
	piped := input + " | json"

	stdout, stderr := redirectProcessOutput(t)
	localAnswer, localCode, localServed := command.ServeLocal(piped, "")
	localWritten := restoreProcessOutput(t, stdout, stderr)
	if !localServed {
		t.Fatalf("%q: the local route does not serve it", input)
	}

	resp, dispatchErr := d.Dispatch(&pluginserver.CommandContext{}, input)
	remoteAnswer, remoteRefusal := matchRemoteRendering(t, piped, resp, dispatchErr)

	if (localCode == 0) != (dispatchErr == nil) {
		t.Errorf("%q: local exit %d (stderr %q), daemon error %v: one route answered and the other refused", input, localCode, localWritten, dispatchErr)
		return
	}
	if strings.TrimRight(localAnswer, "\n") != strings.TrimRight(remoteAnswer, "\n") {
		t.Errorf("%q: answers differ under | json, first at byte %d\nlocal:  %s\ndaemon: %s", input, matchFirstDifference(localAnswer, remoteAnswer), matchAround(localAnswer, remoteAnswer), matchAround(remoteAnswer, localAnswer))
	}
	if dispatchErr != nil && remoteRefusal != localWritten {
		t.Errorf("%q: refusals differ\nlocal:  %q\ndaemon: %q", input, localWritten, remoteRefusal)
	}
}

// matchRemoteRendering renders a dispatch result the way the SSH exec
// channel does: the payload through the input's pipe chain, and an error as
// the operator line (the refusal as written, or "error: " and the text).
func matchRemoteRendering(t *testing.T, piped string, resp *plugin.Response, dispatchErr error) (answer, refusal string) {
	t.Helper()
	if dispatchErr != nil {
		if line, refused := plugin.RefusalLine(dispatchErr); refused {
			refusal = line + "\n"
		} else {
			refusal = "error: " + dispatchErr.Error() + "\n"
		}
	}
	if resp == nil || resp.Data == nil {
		return "", refusal
	}
	_, format, pipeErr := command.ProcessPipesDefaultFormatChecked(piped, "")
	if pipeErr != "" {
		t.Fatalf("%q: pipe chain refused: %s", piped, pipeErr)
	}
	encoded, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("%q: daemon payload: %v", piped, err)
	}
	return format(string(encoded)), refusal
}

// matchMandatoryArgs answers a value for every mandatory leaf, in declared
// order, that its declaration accepts: the first enum value, the low end of
// the first range, or a word inside the declared length. A leaf a value must
// name state for (a file, a key) gets that word, and the routes are then
// compared on how they refuse it.
func matchMandatoryArgs(defs []command.ArgDef) string {
	var words []string
	for _, def := range defs {
		if !def.Mandatory() {
			continue
		}
		words = append(words, matchValue(def))
	}
	return strings.Join(words, " ")
}

// matchValue answers one value the leaf's declaration accepts.
func matchValue(def command.ArgDef) string {
	switch def.Kind() {
	case command.ArgEnum, command.ArgUnion:
		values := slices.Collect(def.EnumValues())
		if len(values) > 0 {
			return values[0]
		}
		return "x"
	case command.ArgUint:
		for r := range def.Ranges() {
			return strconv.FormatUint(r.Min, 10)
		}
		return "0"
	case command.ArgString:
		return "x"
	}
	return "x"
}

// matchFirstDifference answers the offset of the first byte a and b differ at.
func matchFirstDifference(a, b string) int {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// matchAround answers the part of a around its first difference from b, so a
// failure names what differs rather than printing two whole answers.
func matchAround(a, b string) string {
	at := matchFirstDifference(a, b)
	return a[max(0, at-120):min(len(a), at+120)]
}
