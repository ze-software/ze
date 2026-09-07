// The tool is called as a function here, which is the whole point of compiling
// it. Its predecessor, internal/le/command/list/commandlist_test.go, forked `go run` and
// asserted on the process's combined output; every case below says what the old
// assertion proved and where that proof now lives.

package commandlist

import (
	"net"
	"slices"
	"sort"
	"strings"
	"testing"

	pluginregistry "github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestClassifyVerb pins the taxonomy. The verb is the first word when it is one
// of the five, whatever its case, and "-" when it is not: a path whose first
// word is unrecognized must not be filed under a verb it does not have.
func TestClassifyVerb(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"show bgp peer", "show"},
		{"set interface", "set"},
		{"delete peer", "delete"},
		{"update policy", "update"},
		{"monitor bgp", "monitor"},
		{"SHOW bgp", "show"},
		{"peer-list", "-"},
		{"", "-"},
		{"showdown now", "-"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := classifyVerb(tc.path); got != tc.want {
				t.Errorf("classifyVerb(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

// TestCollectReadsTheLiveRegistry is what the old TestCommandInventoryRuns
// proved -- that the tool produces an inventory at all -- without forking a
// process to prove it. It also pins what the old test could not see: the row
// contract each entry satisfies.
func TestCollectReadsTheLiveRegistry(t *testing.T) {
	commands, err := Collect()
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(commands) == 0 {
		t.Fatal("the registry answered no command: the composition root registered nothing")
	}

	sources := map[string]bool{"builtin": true, "streaming": true, "cli": true, "plugin": true}
	for _, entry := range commands {
		if entry.Path == "" {
			t.Errorf("a %s command carries no path: %+v", entry.Source, entry)
		}
		if !sources[entry.Source] {
			t.Errorf("%q carries source %q, which names no registration kind", entry.Path, entry.Source)
		}
		if entry.Verb != classifyVerb(entry.Path) {
			t.Errorf("%q is filed under %q, and its path classifies as %q",
				entry.Path, entry.Verb, classifyVerb(entry.Path))
		}
	}
}

// TestCollectSortsByVerbThenPath pins the order the page is read in. It is part
// of the answer rather than an implementation detail: a reader scans the table
// by verb.
func TestCollectSortsByVerbThenPath(t *testing.T) {
	commands, err := Collect()
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	ordered := sort.SliceIsSorted(commands, func(i, j int) bool {
		if commands[i].Verb != commands[j].Verb {
			return commands[i].Verb < commands[j].Verb
		}
		return commands[i].Path < commands[j].Path
	})
	if !ordered {
		t.Error("the answer is not sorted by verb and then by path")
	}
}

// TestCollectNamesTheDashboard pins the one command reached through neither an
// RPC nor a streaming prefix. Dropping it would leave the inventory silent
// about a command an operator can type.
func TestCollectNamesTheDashboard(t *testing.T) {
	commands, err := Collect()
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if !hasPath(commands, dashboardPath, func(a, b string) bool { return a == b }) {
		t.Errorf("the inventory names no %q", dashboardPath)
	}
}

// TestCollectNamesEachPathOnce is the deduplication the collector does across
// its three sources. A streaming prefix that a builtin RPC already named must
// not appear twice, or every count derived from the answer doubles it.
func TestCollectNamesEachPathOnce(t *testing.T) {
	commands, err := Collect()
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	seen := make(map[string]string, len(commands))
	for _, entry := range commands {
		key := strings.ToLower(entry.Path)
		if first, repeat := seen[key]; repeat {
			t.Errorf("%q is listed twice: once from %s and once from %s", entry.Path, first, entry.Source)
			continue
		}
		seen[key] = entry.Source
	}
}

// TestHasPathComparesTheWayItsCallerNeeds pins the two equalities the collector
// uses. A streaming prefix is matched case-insensitively, because the wire
// method and the CLI path can differ in case; the dashboard path is matched
// exactly.
func TestHasPathComparesTheWayItsCallerNeeds(t *testing.T) {
	commands := Commands{{Path: "Show BGP Peer"}}
	insensitive := strings.EqualFold
	exact := func(a, b string) bool { return a == b }

	if !hasPath(commands, "show bgp peer", insensitive) {
		t.Error("a streaming prefix differing only in case was treated as new")
	}
	if hasPath(commands, "show bgp peer", exact) {
		t.Error("an exact comparison matched a path that differs in case")
	}
}

// TestTextRendersTheMarkdownTable is the other half of what the old
// TestCommandInventoryRuns proved: the output carries the page header and the
// total line. It reads the rendering directly rather than a subprocess's stdout.
func TestTextRendersTheMarkdownTable(t *testing.T) {
	commands := Commands{
		{
			Verb: "show", Path: "show bgp peer", WireMethod: "peer-list", Source: "builtin",
			Shape:         "tab",
			ColumnOrders:  [][]string{{"peer", "state"}, {"up"}},
			AddressFields: []string{"peer"},
			Aliases:       []Alias{{Name: "summary", Description: "the aggregate half", Expansion: "display router-id"}},
		},
		{Verb: "-", Path: "raw-thing", Source: "streaming"},
	}
	text := commands.Text()

	for _, want := range []string{
		"# Command Inventory",
		"| Verb | CLI Path | Wire Method | Source | Shape | Column Order | Address Fields | Aliases |",
		"| show | show bgp peer | peer-list | builtin | tab | peer, state; up | peer | summary |",
		"| - | raw-thing |  | streaming | - | - | - | - |",
		"Total: 2 commands",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the rendering is missing %q:\n%s", want, text)
		}
	}
	if !strings.HasSuffix(text, "\n") {
		t.Error("the rendering does not end in a newline")
	}
}

// TestTextCountsWhatItRendered guards the one number the page states about
// itself: a total that disagrees with the rows is worse than no total.
func TestTextCountsWhatItRendered(t *testing.T) {
	var commands Commands
	for range 7 {
		commands = append(commands, Command{Verb: "show", Path: "show thing", Source: "builtin"})
	}
	if !strings.Contains(commands.Text(), "Total: 7 commands") {
		t.Errorf("seven rows did not render a total of seven:\n%s", commands.Text())
	}
}

// TestAnswerRefusesArguments: the tree and the registry are this process's, so
// there is nothing for an argument to select. Accepting one silently would let
// `le command list --json` look like it worked while doing nothing.
func TestAnswerRefusesArguments(t *testing.T) {
	payload, code := Answer([]string{"--json"})
	if code == 0 {
		t.Error("an argument was accepted")
	}
	if payload != nil {
		t.Errorf("a refused call answered a payload: %v", payload)
	}
}

// TestAnswerAnswersRows is AC-7 at the tool's own boundary: the payload is the
// data, never a rendering of it. A string here would mean the tool formatted
// the answer and left the operator's pipe chain nothing to act on.
func TestAnswerAnswersRows(t *testing.T) {
	payload, code := Answer(nil)
	if code != 0 {
		t.Fatalf("collecting the inventory answered %d", code)
	}
	commands, ok := payload.(Commands)
	if !ok {
		t.Fatalf("the payload is %T, want Commands", payload)
	}
	if len(commands) == 0 {
		t.Error("the payload holds no command")
	}
}

// TestCommandListReportsAPluginDeclaredShape is the wiring test: the inventory
// must report the answer shape a PLUGIN declared, not only the shape of what
// the reader's own process compiled in. `show bgp adj-rib-in status` declares
// `doc` (internal/component/bgp/plugins/adj_rib_in, commandDecls), and that
// declaration reaches the daemon in the plugin's Stage 1 message, which no
// reader of the compiled tree ever sees.
func TestCommandListReportsAPluginDeclaredShape(t *testing.T) {
	commands, err := Collect()
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	byPath := make(map[string]Command, len(commands))
	for _, entry := range commands {
		byPath[entry.Path] = entry
	}

	const shapePath = "show bgp adj-rib-in status"
	entry, found := byPath[shapePath]
	if !found {
		t.Fatalf("the inventory names no %q, so no plugin declaration reached it", shapePath)
	}
	if entry.Shape != "doc" {
		t.Errorf("%q reports shape %q, and the plugin declares doc", shapePath, entry.Shape)
	}

	// The column order is the other half of AC-5, and this reader is the
	// fourth to publish one. The order below is neither alphabetical nor the
	// order the rows are built in, so it can only have come from the
	// declaration (internal/component/bgp/plugins/rpki/rpki.go, commandDecls).
	const orderPath = "show bgp rpki roa"
	entry, found = byPath[orderPath]
	if !found {
		t.Fatalf("the inventory names no %q, so no plugin declaration reached it", orderPath)
	}
	want := [][]string{{"prefix", "max-length", "asn"}}
	if !slices.EqualFunc(entry.ColumnOrders, want, slices.Equal) {
		t.Errorf("%q reports column orders %v, and the plugin declares %v",
			orderPath, entry.ColumnOrders, want)
	}
}

// TestRegistrationCarriesTheDeclaredCommands is AC-4 of
// spec-daemon-backed-command-catalog: a reader answers a plugin's
// command declarations, and answers them with NO engine started.
//
// The guard is the point of the test, so it is driven through Collect, the
// reader `./le command list` runs. A plugin's declarations used to exist only
// after its runner sent them in the Stage 1 registration message, so the
// rejected way to read them was to RUN the plugin -- which, for two of the 97
// runners, programs nftables or writes XFRM policy before it declares anything.
// Every runner in this process is replaced by one that records its own call,
// so a reader that goes back to that route fails here by name rather than by
// the host state it changed.
//
// registry.All answers POINTERS into the registry, so the stubs are shared with
// every other test in this binary until the cleanup restores them. This test
// therefore MUST NOT call t.Parallel, and neither may anything that reads
// RunEngine.
func TestRegistrationCarriesTheDeclaredCommands(t *testing.T) {
	var started []string

	registrations := pluginregistry.All()
	engines := make([]func(net.Conn) int, len(registrations))
	for index, reg := range registrations {
		engines[index] = reg.RunEngine
		name := reg.Name
		reg.RunEngine = func(net.Conn) int {
			started = append(started, name)
			return 1
		}
	}
	t.Cleanup(func() {
		for index, reg := range registrations {
			reg.RunEngine = engines[index]
		}
	})

	commands, err := Collect()
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	reported := make(map[string]bool, len(commands))
	for _, entry := range commands {
		reported[entry.Path] = true
	}

	// One plugin for each way a declaration reaches the registration:
	// bgp-adj-rib-in already had a commandDecls function, mrt's runner carried
	// a one-line literal, and sysctl's carried a six-entry block. Each is
	// asserted through the reader's own answer, so a declaration the reader
	// drops is a failure here and not only a missing field.
	for _, path := range []string{
		"show bgp adj-rib-in status",
		"request mrt dump-rib",
		"show sysctl",
	} {
		if !reported[path] {
			t.Errorf("the inventory names no %q, so a plugin's declaration did not reach the reader", path)
		}
	}

	if len(started) > 0 {
		t.Errorf("reading the declarations started %d engine(s): %v", len(started), started)
	}
}
