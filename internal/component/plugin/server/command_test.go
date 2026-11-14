package server

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/redact"
)

// handlerMarking returns a handler that records it ran in *ran.
func handlerMarking(ran *bool) Handler {
	return func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		*ran = true
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}
}

// TestDispatcherRefusesADuplicateName proves a second builtin cannot take a
// command name another builtin holds.
//
// VALIDATES: AC-11. RegisterWithOptions returns ErrCommandHeld for a name
// already held, case-insensitively, the message names the holding owner, and
// the holder's handler stays the one that runs. Register refuses the same way
// and names a holder that declared no owner as such.
// PREVENTS: the silent last-init()-wins overwrite of d.commands, where one
// component's handler replaced another's with no error and no log.
func TestDispatcherRefusesADuplicateName(t *testing.T) {
	d := NewDispatcher()

	var firstRan, secondRan bool
	if err := d.RegisterWithOptions("show alpha", handlerMarking(&firstRan), "", RegisterOptions{Owner: "ze-alpha:show"}); err != nil {
		t.Fatalf("first registration refused: %v", err)
	}

	err := d.RegisterWithOptions("Show Alpha", handlerMarking(&secondRan), "", RegisterOptions{Owner: "ze-intruder:show"})
	if !errors.Is(err, ErrCommandHeld) {
		t.Fatalf("second registration of a held name: got %v, want ErrCommandHeld", err)
	}
	if !strings.Contains(err.Error(), "ze-alpha:show") {
		t.Errorf("refusal %q does not name the holder ze-alpha:show", err)
	}

	held := d.commands["show alpha"]
	if held.Owner != "ze-alpha:show" {
		t.Errorf("holder after refusal = %q, want ze-alpha:show", held.Owner)
	}
	if _, err := held.Handler(nil, commandtest.Args()); err != nil {
		t.Fatalf("holder handler: %v", err)
	}
	if !firstRan {
		t.Error("the holder's handler did not run")
	}
	if secondRan {
		t.Error("the refused handler ran: the second registration overwrote the first")
	}

	if err := d.Register("show beta", handlerMarking(&firstRan), "beta"); err != nil {
		t.Fatalf("first Register refused: %v", err)
	}
	err = d.Register("show beta", handlerMarking(&secondRan), "beta again")
	if !errors.Is(err, ErrCommandHeld) {
		t.Fatalf("second Register of a held name: got %v, want ErrCommandHeld", err)
	}
	if !strings.Contains(err.Error(), "declared no owner") {
		t.Errorf("refusal %q does not say the holder declared no owner", err)
	}
}

// TestLoadBuiltinsRefusesADuplicateWireMethod proves the builtin load refuses
// two linked registrations that carry one wire method.
//
// VALIDATES: AC-12, wire-method half. loadBuiltinsWithAliases returns
// ErrWireMethodHeld naming the method, before it registers any path.
// PREVENTS: the bare wireToHandler assignment that kept whichever handler came
// last and served it under both owners' names.
func TestLoadBuiltinsRefusesADuplicateWireMethod(t *testing.T) {
	var ran bool
	regs := []RPCRegistration{
		{WireMethod: "ze-alpha:dup", Handler: handlerMarking(&ran)},
		{WireMethod: "ze-alpha:dup", Handler: handlerMarking(&ran)},
	}
	wireToPaths := map[string][]string{"ze-alpha:dup": {"show alpha dup"}}

	d := NewDispatcher()
	err := loadBuiltinsWithAliases(d, regs, wireToPaths, nil, nil, nil, nil)
	if !errors.Is(err, ErrWireMethodHeld) {
		t.Fatalf("got %v, want ErrWireMethodHeld", err)
	}
	if !strings.Contains(err.Error(), "ze-alpha:dup") {
		t.Errorf("refusal %q does not name the wire method", err)
	}
	if len(d.commands) != 0 {
		t.Errorf("refused load registered %d commands, want 0", len(d.commands))
	}
}

// TestLoadBuiltinsRefusesTwoOwnersOnOnePath proves two wire methods that reach
// one command path cannot both register it.
//
// VALIDATES: AC-12, name half. The second owner gets ErrCommandHeld naming the
// first owner's wire method, and the alias load stops there.
// PREVENTS: one owner taking another's command because its init() ran later.
func TestLoadBuiltinsRefusesTwoOwnersOnOnePath(t *testing.T) {
	var ran bool
	regs := []RPCRegistration{
		{WireMethod: "ze-alpha:one", Handler: handlerMarking(&ran)},
		{WireMethod: "ze-beta:one", Handler: handlerMarking(&ran)},
	}
	wireToPaths := map[string][]string{
		"ze-alpha:one": {"show shared"},
		"ze-beta:one":  {"show shared"},
	}

	err := loadBuiltinsWithAliases(NewDispatcher(), regs, wireToPaths, nil, nil, nil, nil)
	if !errors.Is(err, ErrCommandHeld) {
		t.Fatalf("got %v, want ErrCommandHeld", err)
	}
	if !strings.Contains(err.Error(), "ze-alpha:one") {
		t.Errorf("refusal %q does not name the holder ze-alpha:one", err)
	}
}

// TestNewServerRefusesABuiltinCollision proves the server refuses to serve
// when two linked builtins carry one wire method.
//
// VALIDATES: AC-12 at the real startup path: NewServer returns the
// ErrWireMethodHeld refusal instead of a server.
// PREVENTS: a daemon that starts with one owner's command silently served by
// another owner's handler.
func TestNewServerRefusesABuiltinCollision(t *testing.T) {
	saved := registeredRPCs
	t.Cleanup(func() { registeredRPCs = saved })

	var held RPCRegistration
	for _, reg := range saved {
		if reg.WireMethod == "ze-plugin:system-daemon-reload" {
			held = reg
		}
	}
	if held.WireMethod == "" {
		t.Fatal("ze-plugin:system-daemon-reload is not linked; the test needs a registration the server package owns")
	}
	registeredRPCs = append(append([]RPCRegistration(nil), saved...), held)

	s, err := NewServer(&ServerConfig{}, nil)
	if !errors.Is(err, ErrWireMethodHeld) {
		t.Fatalf("NewServer with a duplicated wire method: got (%v, %v), want ErrWireMethodHeld", s, err)
	}
}

// TestLoadBuiltinsServesLocalCommands proves the dispatcher serves every
// local-data command a YANG ze:command node declares, derived from the
// registries rather than from a list.
//
// VALIDATES: AC-4, AC-11 (bridge half). For each local-data registration
// linked into this binary whose exact path a YANG node declares, the
// dispatcher holds a command under that path, owned by the node's wire
// method, ReadOnly as the verb says unless the registration declares
// NotReadOnly, with the node's argument definitions. At least one is checked,
// so the test cannot pass on an empty population.
// PREVENTS: a local-data command answering only on `ze` while every daemon
// surface (MCP, REST, gRPC, SSH, web) reports it unknown.
func TestLoadBuiltinsServesLocalCommands(t *testing.T) {
	d, wireToPaths := localBridgeDispatcher(t)
	declared := make(map[string]string)
	for wire, paths := range wireToPaths {
		for _, path := range paths {
			declared[path] = wire
		}
	}
	loader, err := yang.DefaultLoader()
	if err != nil {
		t.Fatal(err)
	}
	argDefs := yang.PathToArgDefs(loader)

	checked := 0
	for _, entry := range command.ListLocal() {
		wire, hasNode := declared[entry.Path]
		if !hasNode {
			continue
		}
		handler, rest := command.LookupLocalData(strings.Fields(entry.Path))
		if handler == nil || len(rest) != 0 {
			continue
		}
		checked++
		cmd := d.Lookup(entry.Path)
		if cmd == nil {
			t.Errorf("%q: local-data command with YANG node %s is not served by the dispatcher", entry.Path, wire)
			continue
		}
		if cmd.Name != entry.Path {
			t.Errorf("%q: served under %q, want its own path", entry.Path, cmd.Name)
		}
		if cmd.Owner != wire {
			t.Errorf("%q: owner %q, want the node's wire method %q", entry.Path, cmd.Owner, wire)
		}
		wantReadOnly := IsReadOnlyPath(entry.Path) && !entry.Meta.NotReadOnly
		if cmd.ReadOnly != wantReadOnly {
			t.Errorf("%q: ReadOnly %v, want %v", entry.Path, cmd.ReadOnly, wantReadOnly)
		}
		if len(cmd.ArgDefs) != len(argDefs[entry.Path]) {
			t.Errorf("%q: %d argument definitions, want the node's %d", entry.Path, len(cmd.ArgDefs), len(argDefs[entry.Path]))
		}
	}
	if checked == 0 {
		t.Fatal("no local-data command with a YANG node is linked into this binary: the check proved nothing")
	}
	t.Logf("%d local-data commands served by the dispatcher", checked)
}

// TestAccountingRecordsMaskedContent proves the accounting record of a command
// line that carries caller bytes holds the placeholder in place of them.
//
// VALIDATES: AC-8 of spec-data-store-maintenance-through-the-daemon, its
// accounting half. The START and STOP records of `update data key <key>
// content <base64>` and `update data config name <name> content <base64>` hold
// the command path, the key or name, and redact.Placeholder, on the text
// accountant, on the typed accountant, and on the typed inter-plugin route.
// PREVENTS: a credential written through the daemon reaching the TACACS+
// accounting server in clear.
// Method: dispatch each line with a capturing accountant and compare the
// records with the masked line. Accounting runs at admission, before lookup and
// validation (BeginAccounting), so the test registers no handler and does not
// judge the dispatch result: phase 5 builds the commands themselves.
func TestAccountingRecordsMaskedContent(t *testing.T) {
	const content = "c2VjcmV0LWNyZWRlbnRpYWw="
	cases := []struct {
		command string
		args    []string
	}{
		{command: "update data key", args: []string{"meta/ddos-flowspec/announcement", "content", content}},
		{command: "update data config", args: []string{"name", "router.conf", "content", content}},
	}
	for _, c := range cases {
		line := c.command + " " + strings.Join(c.args, " ")
		masked := strings.TrimSuffix(line, content) + redact.Placeholder
		maskedTokens := strings.Fields(masked)

		t.Run(c.command+" text accountant", func(t *testing.T) {
			d := NewDispatcher()
			acct := &fakeAccountant{}
			d.SetAccountingHook(acct)
			// The dispatch result is not this test's question: see Method.
			_, _ = d.Dispatch(&CommandContext{Username: "alice"}, line)
			assertRecords(t, "START", acct.starts, masked, content)
			assertRecords(t, "STOP", acct.stops, masked, content)
		})

		t.Run(c.command+" typed accountant", func(t *testing.T) {
			d := NewDispatcher()
			acct := &typedAccountingCapture{}
			d.SetAccountingHook(acct)
			// The dispatch result is not this test's question: see Method.
			_, _ = d.Dispatch(&CommandContext{Username: "alice"}, line)
			assertTokenRecords(t, "START", acct.startTokens, maskedTokens)
			assertTokenRecords(t, "STOP", acct.stopTokens, maskedTokens)
		})

		t.Run(c.command+" typed inter-plugin route", func(t *testing.T) {
			d := NewDispatcher()
			acct := &typedAccountingCapture{}
			d.SetAccountingHook(acct)
			stop := d.beginAccountingArgs(&CommandContext{Username: "alice"}, c.command, c.args, "")
			stop()
			assertTokenRecords(t, "START", acct.startTokens, maskedTokens)
			assertTokenRecords(t, "STOP", acct.stopTokens, maskedTokens)
		})
	}
}

// assertRecords checks that a text accountant recorded exactly one line, the
// masked one, and that the content appears in no record.
func assertRecords(t *testing.T, kind string, got []string, want, content string) {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("%s records = %q, want one", kind, got)
	}
	if got[0] != want {
		t.Errorf("%s record = %q, want %q", kind, got[0], want)
	}
	if strings.Contains(got[0], content) {
		t.Errorf("%s record carries the content: %q", kind, got[0])
	}
}

// assertTokenRecords checks that a typed accountant recorded exactly one token
// list, the masked one.
func assertTokenRecords(t *testing.T, kind string, got [][]string, want []string) {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("%s records = %q, want one", kind, got)
	}
	if strings.Join(got[0], "\x00") != strings.Join(want, "\x00") {
		t.Errorf("%s tokens = %q, want %q", kind, got[0], want)
	}
}
