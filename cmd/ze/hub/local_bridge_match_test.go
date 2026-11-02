package hub

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/statestore"
)

// matchUndeclaredWord is a word no command declares as a keyword, so a call
// carrying it is refused by the argument validator on both routes.
const matchUndeclaredWord = "ze-match-no-such-word"

// matchStringValue is the value a mandatory string leaf gets. The file
// commands (show config graph, validation, plugin declarations config) read
// it as a file name, so the test writes matchConfig under that name.
const matchStringValue = "x"

// matchConfig is a valid configuration, written under every word a file
// command reads as its file, so both routes read a real file and the answers
// compare rather than two refusals of a missing one.
const matchConfig = `
bgp {
    router-id 1.1.1.1;
    session {
        asn {
            local 65000;
        }
    }
    peer peer1 {
        connection {
            remote {
                ip 10.0.0.1;
            }
        }
        session {
            asn {
                remote 65001;
            }
        }
    }
}
`

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
//
// A command that produces a file (registry.Meta.Artifact) names it where each
// route is designed to keep it: the local route is given a directory of the
// test's own and the daemon a store of the test's own, and the two documents
// must agree on every field but those the declaration names (matchArtifact).
// PREVENTS: the daemon route answering a command other than the local route
// does, or refusing it in other words. A refusal may differ only where the
// local route refuses too and the daemon route names the local command to
// run instead (show bgp encode with no route reads standard input, which only
// the local process has): that difference is by design, and it is logged.
func TestLocalCommandDaemonMatchesLocal(t *testing.T) {
	// The commands run for real, and some write state (delete debug profile
	// name opens debug.zefs in the working directory); a directory of the
	// test's own keeps that out of the checkout. It is also a directory the
	// daemon route confines file arguments to, and XDG_CONFIG_HOME points at
	// another empty one, so the operator's own configuration directory is
	// neither read nor named in a refusal.
	//
	// ze.config.dir is pinned to that working directory as well: debug.zefs
	// and every other store a command opens resolve through it
	// (paths.DefaultConfigDir), and an earlier test whose run() published a
	// file source's directory there would otherwise send both routes into a
	// directory that test already removed, where each refusal names its own
	// random temp file and the comparison fails on a name, not on behavior.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	work := t.TempDir()
	t.Chdir(work)
	previousConfigDir := env.Get("ze.config.dir")
	if err := env.Set("ze.config.dir", work); err != nil {
		t.Fatalf("pin ze.config.dir: %v", err)
	}
	t.Cleanup(func() {
		if err := env.Set("ze.config.dir", previousConfigDir); err != nil {
			t.Errorf("restore ze.config.dir: %v", err)
		}
	})
	for _, name := range []string{matchStringValue, matchUndeclaredWord} {
		if err := os.WriteFile(name, []byte(matchConfig), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	if err != nil {
		t.Fatalf("daemon dispatcher: %v", err)
	}
	t.Cleanup(server.Stop)
	d := server.Dispatcher()

	checked := 0
	for _, entry := range command.ListLocal() {
		// A local command at an offline root is served by the daemon under
		// the path its registration declares (registry.Meta.DaemonPath), so
		// that is the path the daemon route is asked.
		daemonPath := entry.Path
		if entry.Meta.DaemonPath != "" {
			daemonPath = entry.Meta.DaemonPath
		}
		served := d.Lookup(daemonPath)
		if served == nil || served.Name != daemonPath {
			continue
		}
		handler, rest := command.LookupLocalData(strings.Fields(entry.Path))
		if handler == nil || len(rest) != 0 {
			continue
		}
		checked++
		args := matchMandatoryArgs(served.ArgDefs)
		local := matchInput(entry.Path, args)
		daemon := matchInput(daemonPath, args)
		t.Run(entry.Path, func(t *testing.T) {
			if artifact := entry.Meta.Artifact; artifact != nil {
				matchArtifact(t, d, local, artifact)
				matchArtifact(t, d, local+" "+matchUndeclaredWord, artifact)
				return
			}
			matchRoutes(t, d, local, daemon)
			matchRoutes(t, d, local+" "+matchUndeclaredWord, daemon+" "+matchUndeclaredWord)
		})
	}
	if checked == 0 {
		t.Fatal("the dispatcher serves no local-data command: the bridge loaded nothing")
	}
}

// matchInput is a command path followed by its mandatory arguments, if any.
func matchInput(path, args string) string {
	if args == "" {
		return path
	}
	return path + " " + args
}

// matchRoutes runs input on the local route and daemonInput on the daemon
// route, and fails when an operator would read a different answer or a
// different refusal. The two inputs differ only for a local command at an
// offline root, whose daemon path its registration declares.
func matchRoutes(t *testing.T, d *pluginserver.Dispatcher, input, daemonInput string) {
	t.Helper()
	piped := input + " | json"

	stdout, stderr := redirectProcessOutput(t)
	localAnswer, localCode, localServed := command.ServeLocal(piped, "")
	localWritten := restoreProcessOutput(t, stdout, stderr)
	if !localServed {
		t.Fatalf("%q: the local route does not serve it", input)
	}

	resp, dispatchErr := d.Dispatch(&pluginserver.CommandContext{}, daemonInput)
	remoteAnswer, remoteRefusal := matchRemoteRendering(t, daemonInput+" | json", resp, dispatchErr)

	if (localCode == 0) != (dispatchErr == nil) {
		t.Errorf("%q: local exit %d (stderr %q), daemon error %v: one route answered and the other refused", input, localCode, localWritten, dispatchErr)
		return
	}
	if strings.TrimRight(localAnswer, "\n") != strings.TrimRight(remoteAnswer, "\n") {
		t.Errorf("%q: answers differ under | json, first at byte %d\nlocal:  %s\ndaemon: %s", input, matchFirstDifference(localAnswer, remoteAnswer), matchAround(localAnswer, remoteAnswer), matchAround(remoteAnswer, localAnswer))
	}
	if dispatchErr != nil && remoteRefusal != localWritten {
		if strings.Contains(remoteRefusal, "'ze "+input) {
			t.Logf("%q: the daemon route defers to the local command, by design\nlocal:  %q\ndaemon: %q", input, localWritten, remoteRefusal)
			return
		}
		t.Errorf("%q: refusals differ\nlocal:  %q\ndaemon: %q", input, localWritten, remoteRefusal)
	}
}

// matchArtifact runs a command that produces a file on both routes, the way
// its registry.Artifact declares, and fails when either location is not the
// kind its route is designed to answer (a file under the directory the local
// route was given, a key `show data cat` reads on the daemon) or when the two
// documents differ in any field the declaration does not name. A refusal
// must read the same bytes on both routes.
func matchArtifact(t *testing.T, d *pluginserver.Dispatcher, input string, artifact *registry.Artifact) {
	t.Helper()
	// The daemon keeps the file in the store `show data cat` reads, the one
	// under ze.config.dir; a directory of this call's own holds it, so no
	// other command's answer sees the store or the file.
	configDir := t.TempDir()
	previousConfigDir := env.Get("ze.config.dir")
	if err := env.Set("ze.config.dir", configDir); err != nil {
		t.Fatalf("%q: pin ze.config.dir: %v", input, err)
	}
	t.Cleanup(func() {
		if err := env.Set("ze.config.dir", previousConfigDir); err != nil {
			t.Errorf("%q: restore ze.config.dir: %v", input, err)
		}
	})
	store, err := storage.Create(configDir)
	if err != nil {
		t.Fatalf("%q: daemon store: %v", input, err)
	}
	previous := statestore.Store()
	statestore.SetStore(store)
	t.Cleanup(func() {
		statestore.SetStore(previous)
		if err := store.Close(); err != nil {
			t.Errorf("%q: close daemon store: %v", input, err)
		}
	})
	destination := t.TempDir()

	stdout, stderr := redirectProcessOutput(t)
	localAnswer, localCode, localServed := command.ServeLocal(input+" "+artifact.LocalArgs+" "+destination+" | json", "")
	localWritten := restoreProcessOutput(t, stdout, stderr)
	if !localServed {
		t.Fatalf("%q: the local route does not serve it", input)
	}
	resp, dispatchErr := d.Dispatch(&pluginserver.CommandContext{}, input)
	remoteAnswer, remoteRefusal := matchRemoteRendering(t, input+" | json", resp, dispatchErr)
	if (localCode == 0) != (dispatchErr == nil) {
		t.Errorf("%q: local exit %d (stderr %q), daemon error %v: one route answered and the other refused", input, localCode, localWritten, dispatchErr)
		return
	}
	if dispatchErr != nil {
		if remoteRefusal != localWritten {
			t.Errorf("%q: refusals differ\nlocal:  %q\ndaemon: %q", input, localWritten, remoteRefusal)
		}
		return
	}

	local := matchArtifactDocument(t, input, "local", localAnswer)
	remote := matchArtifactDocument(t, input, "daemon", remoteAnswer)
	matchArtifactFile(t, input, destination, local[artifact.LocalField])
	matchArtifactKey(t, d, input, remote[artifact.DaemonField])
	delete(local, artifact.LocalField)
	delete(remote, artifact.DaemonField)
	matchDropFields(local, artifact.Volatile)
	matchDropFields(remote, artifact.Volatile)
	localJSON, err := json.Marshal(local)
	if err != nil {
		t.Fatalf("%q: local document: %v", input, err)
	}
	remoteJSON, err := json.Marshal(remote)
	if err != nil {
		t.Fatalf("%q: daemon document: %v", input, err)
	}
	if string(localJSON) != string(remoteJSON) {
		t.Errorf("%q: documents differ outside the declared fields\nlocal:  %s\ndaemon: %s", input, localJSON, remoteJSON)
	}
}

// matchArtifactDocument decodes one route's `| json` answer as an object. A
// command that answers its document as JSON text (a command.Blob, as the
// local route does under the declared LocalArgs) reaches `| json` wrapped as
// {"blob":{"content":...}}, and the document is that content.
func matchArtifactDocument(t *testing.T, input, route, answer string) map[string]any {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal([]byte(answer), &document); err != nil {
		t.Fatalf("%q: the %s answer is no JSON object: %v\n%s", input, route, err, answer)
	}
	blob, isBlob := document["blob"].(map[string]any)
	if !isBlob {
		return document
	}
	content, isText := blob["content"].(string)
	if !isText {
		t.Fatalf("%q: the %s answer is a blob with no text content: %s", input, route, answer)
	}
	var inner map[string]any
	if err := json.Unmarshal([]byte(content), &inner); err != nil {
		t.Fatalf("%q: the %s answer's blob is no JSON object: %v\n%s", input, route, err, content)
	}
	return inner
}

// matchArtifactFile fails unless location is the path of a regular file in
// destination, the directory the local route was given.
func matchArtifactFile(t *testing.T, input, destination string, location any) {
	t.Helper()
	path, isString := location.(string)
	if !isString {
		t.Errorf("%q: the local answer names no file: %#v", input, location)
		return
	}
	if filepath.Dir(path) != destination {
		t.Errorf("%q: the local route wrote %s, outside the directory it was given (%s)", input, path, destination)
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("%q: the local answer names %s, which cannot be read: %v", input, path, err)
		return
	}
	if !info.Mode().IsRegular() {
		t.Errorf("%q: the local answer names %s, which is no regular file", input, path)
	}
}

// matchArtifactKey fails unless location is a key the daemon's store holds,
// read back the way a remote caller is told to: `show data cat <key>`.
func matchArtifactKey(t *testing.T, d *pluginserver.Dispatcher, input string, location any) {
	t.Helper()
	key, isString := location.(string)
	if !isString {
		t.Errorf("%q: the daemon answer names no store key: %#v", input, location)
		return
	}
	resp, err := d.Dispatch(&pluginserver.CommandContext{}, "show data cat "+key)
	if err != nil {
		t.Errorf("%q: the daemon answer names %s, which show data cat refuses: %v", input, key, err)
		return
	}
	if resp == nil {
		t.Errorf("%q: show data cat %s answered nothing", input, key)
		return
	}
	if resp.Data == nil {
		t.Errorf("%q: show data cat %s answered no bytes", input, key)
	}
}

// matchDropFields removes every field named in names from value, at any
// depth. The recursion is bounded by the nesting of one command's answer,
// which the test itself just decoded.
func matchDropFields(value any, names []string) {
	switch node := value.(type) {
	case map[string]any:
		for _, name := range names {
			delete(node, name)
		}
		for _, child := range node {
			matchDropFields(child, names)
		}
	case []any:
		for _, child := range node {
			matchDropFields(child, names)
		}
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
	for i := range defs {
		if !defs[i].Mandatory() {
			continue
		}
		words = append(words, matchValue(defs[i]))
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
		return matchStringValue
	case command.ArgFlag:
		// A flag is the keyword alone (command.ArgFlag), so naming the leaf
		// is the whole argument.
		return def.Name()
	}
	panic("BUG: matchValue: argument kind " + strconv.Itoa(int(def.Kind())) + " has no matching value")
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
