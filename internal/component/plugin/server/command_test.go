package server

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// handlerMarking returns a handler that records it ran in *ran.
func handlerMarking(ran *bool) Handler {
	return func(_ *CommandContext, _ []string) (*plugin.Response, error) {
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
	if _, err := held.Handler(nil, nil); err != nil {
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
		if reg.WireMethod == "ze-system:daemon-reload" {
			held = reg
		}
	}
	if held.WireMethod == "" {
		t.Fatal("ze-system:daemon-reload is not linked; the test needs a registration the server package owns")
	}
	registeredRPCs = append(append([]RPCRegistration(nil), saved...), held)

	s, err := NewServer(&ServerConfig{}, nil)
	if !errors.Is(err, ErrWireMethodHeld) {
		t.Fatalf("NewServer with a duplicated wire method: got (%v, %v), want ErrWireMethodHeld", s, err)
	}
}
