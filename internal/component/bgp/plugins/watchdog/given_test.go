package watchdog

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// givenRecorder is a send hook that records each command with its selector,
// or, while refuse is set, answers refuse as the engine refusing the send.
type givenRecorder struct {
	sent   []string
	refuse error
}

func (r *givenRecorder) send(peer, cmd string) error {
	if r.refuse != nil {
		return r.refuse
	}
	r.sent = append(r.sent, peer+" | "+cmd)
	return nil
}

// errGivenRefused stands for the engine's answer to a peer that does not
// attach bgp-watchdog with `send [ update ]`.
var errGivenRefused = errors.New("send refused: this peer does not attach that process with the send permission it needs")

// mustRefuse runs a command that MUST answer the engine's refusal.
func mustRefuse(t *testing.T, s *watchdogServer, command string, args ...string) {
	t.Helper()
	status, _, err := s.handleCommand(command, args, "*")
	if status != statusError || !errors.Is(err, errGivenRefused) {
		t.Fatalf("%s %v answered %s, %v; want error wrapping the refusal", command, args, status, err)
	}
}

// TestGivenRouteRefusedSendIsAnError verifies that a given route the engine
// refuses to send answers an error to give, announce, withdraw and forget,
// never done, and that the refused route keeps its former state.
//
// VALIDATES: a process reads the refusal (api-watchdog-route.ci read done
// while nothing reached the peer).
// PREVENTS: a hook that only logs the refusal.
func TestGivenRouteRefusedSendIsAnError(t *testing.T) {
	s, rec := givenServer(t)

	rec.refuse = errGivenRefused
	mustRefuse(t, s, commandRequestWatchdogGive, givenArgs("helper", false)...)
	if s.givenHolds("dnsr") {
		t.Fatal("a route whose announce was refused is still held")
	}

	mustDone(t, s, commandRequestWatchdogGive, givenArgs("helper", true)...)
	mustRefuse(t, s, commandRequestWatchdogAnnounce, "dnsr")
	rec.refuse = nil
	mustDone(t, s, commandRequestWatchdogAnnounce, "dnsr")
	if want := []string{givenAnnounce}; !slices.Equal(rec.sent, want) {
		t.Fatalf("after a refused announce, announce sent %q, want %q", rec.sent, want)
	}

	rec.refuse = errGivenRefused
	mustRefuse(t, s, commandRequestWatchdogWithdraw, "dnsr")
	mustRefuse(t, s, commandRequestWatchdogForget, "helper")
}

const (
	givenAnnounce   = "* | update text nhop 1.1.1.1 nlri add 10.0.4.0/24"
	givenWithdrawal = "* | update text nhop 1.1.1.1 nlri del 10.0.4.0/24"
	givenRestore    = "* | update text restore-configured nhop 1.1.1.1 nlri del 10.0.4.0/24"
)

// givenArgs is the argument list the ExaBGP bridge writes for one route
// (watchdogGiveCommand, internal/exabgp/bridge/bridge_watchdog.go).
func givenArgs(owner string, held bool) []string {
	line := "dnsr owner " + owner + " selector *"
	if held {
		line += " withdraw"
	}
	line += " announce 7 update text nhop 1.1.1.1 nlri add 10.0.4.0/24 withdrawal update text nhop 1.1.1.1 nlri del 10.0.4.0/24"
	return strings.Fields(line)
}

func givenServer(t *testing.T) (*watchdogServer, *givenRecorder) {
	t.Helper()
	rec := &givenRecorder{}
	return newWatchdogServer(rec.send, nil), rec
}

func mustDone(t *testing.T, s *watchdogServer, command string, args ...string) {
	t.Helper()
	status, _, err := s.handleCommand(command, args, "*")
	if err != nil || status != statusDone {
		t.Fatalf("%s %v answered %s, %v", command, args, status, err)
	}
}

// TestGivenRouteHeldUntilAnnounce verifies that a route given with `withdraw`
// is not sent until its watchdog is announced, and is withdrawn with it.
//
// VALIDATES: c43f90f79 (ExaBGP f9bfb853b): an API route with `withdraw`
// waits for `announce watchdog <name>`; `withdraw watchdog <name>` withdraws it.
// Mirrors test_an_api_route_starting_withdrawn_waits_for_its_watchdog.
func TestGivenRouteHeldUntilAnnounce(t *testing.T) {
	s, rec := givenServer(t)
	mustDone(t, s, commandRequestWatchdogGive, givenArgs("helper", true)...)
	if len(rec.sent) != 0 {
		t.Fatalf("a held route was sent: %q", rec.sent)
	}
	mustDone(t, s, commandRequestWatchdogAnnounce, "dnsr")
	mustDone(t, s, commandRequestWatchdogAnnounce, "dnsr")
	mustDone(t, s, commandRequestWatchdogWithdraw, "dnsr")
	if want := []string{givenAnnounce, givenWithdrawal}; !slices.Equal(rec.sent, want) {
		t.Errorf("sent %q, want %q", rec.sent, want)
	}
}

// TestGivenRouteSentNowAndWithdrawnWithItsWatchdog verifies that a route given
// without `withdraw` is sent at once and withdrawn by its watchdog.
// Mirrors test_an_api_route_naming_a_watchdog_is_announced_and_withdrawn_with_it.
func TestGivenRouteSentNowAndWithdrawnWithItsWatchdog(t *testing.T) {
	s, rec := givenServer(t)
	mustDone(t, s, commandRequestWatchdogGive, givenArgs("helper", false)...)
	mustDone(t, s, commandRequestWatchdogWithdraw, "dnsr")
	if want := []string{givenAnnounce, givenWithdrawal}; !slices.Equal(rec.sent, want) {
		t.Errorf("sent %q, want %q", rec.sent, want)
	}
}

// TestForgetOwnerWithdrawsAndForgets verifies that forgetting an owner
// withdraws its announced routes, restoring a configured one, and that a later
// announce of the watchdog does not bring any of them back.
//
// VALIDATES: _forget_watchdog_owner and withdraw_owner at f9bfb853b.
// Mirrors test_the_routes_a_helper_gave_a_watchdog_go_with_the_helper and
// test_a_route_held_withdrawn_for_a_helper_which_exits_is_not_announced_later.
func TestForgetOwnerWithdrawsAndForgets(t *testing.T) {
	s, rec := givenServer(t)
	mustDone(t, s, commandRequestWatchdogGive, givenArgs("helper", false)...)
	mustDone(t, s, commandRequestWatchdogForget, "helper")
	mustDone(t, s, commandRequestWatchdogWithdraw, "dnsr")
	mustDone(t, s, commandRequestWatchdogAnnounce, "dnsr")
	if want := []string{givenAnnounce, givenRestore}; !slices.Equal(rec.sent, want) {
		t.Errorf("sent %q, want %q", rec.sent, want)
	}

	held, heldRec := givenServer(t)
	mustDone(t, held, commandRequestWatchdogGive, givenArgs("helper", true)...)
	mustDone(t, held, commandRequestWatchdogForget, "helper")
	mustDone(t, held, commandRequestWatchdogAnnounce, "dnsr")
	if len(heldRec.sent) != 0 {
		t.Errorf("a held route of a forgotten owner was sent: %q", heldRec.sent)
	}
	if held.givenCount != 0 || len(held.given) != 0 {
		t.Errorf("forget left %d routes in %d watchdogs", held.givenCount, len(held.given))
	}
}

// TestForgetLeavesOtherOwners verifies that forgetting one owner leaves the
// routes another owner gave, and that giving a route again hands it over.
func TestForgetLeavesOtherOwners(t *testing.T) {
	s, rec := givenServer(t)
	mustDone(t, s, commandRequestWatchdogGive, givenArgs("a", true)...)
	mustDone(t, s, commandRequestWatchdogGive, givenArgs("b", true)...)
	mustDone(t, s, commandRequestWatchdogForget, "a")
	mustDone(t, s, commandRequestWatchdogAnnounce, "dnsr")
	if want := []string{givenAnnounce}; !slices.Equal(rec.sent, want) {
		t.Errorf("sent %q, want b's route announced once", rec.sent)
	}
	if s.givenCount != 1 {
		t.Errorf("the route given twice counts %d, want 1", s.givenCount)
	}
}

// TestGivenSelectorScope verifies that a watchdog announce for one peer moves
// only the routes given for that selector, and answers done for them.
func TestGivenSelectorScope(t *testing.T) {
	s, rec := givenServer(t)
	mustDone(t, s, commandRequestWatchdogGive, givenArgs("helper", true)...)
	status, _, err := s.handleCommand(commandRequestWatchdogAnnounce, []string{"dnsr", "10.0.0.1"}, "*")
	if err == nil || status != statusError {
		t.Errorf("an announce for a peer with no route of the watchdog answered %s, %v", status, err)
	}
	if len(rec.sent) != 0 {
		t.Errorf("an announce for another selector sent %q", rec.sent)
	}
}

// TestGivenRefusals verifies that a malformed add is refused naming the
// expected form, and that a MED override on a watchdog holding given routes
// is refused rather than announcing them without it.
func TestGivenRefusals(t *testing.T) {
	s, _ := givenServer(t)
	for _, line := range []string{
		"dnsr",
		"dnsr owner a selector * announce 9 update text nlri add 10.0.4.0/24 withdrawal update text nlri del 10.0.4.0/24",
		"dnsr owner a selector * announce 0 withdrawal update text",
		"dnsr owner a selector * announce x update text withdrawal update text",
		"dnsr owner a selector * announce 2 update text nlri withdrawal",
		"dnsr who a selector * announce 2 update text withdrawal update text",
		"med owner a selector * announce 2 update text withdrawal update text",
	} {
		status, _, err := s.handleCommand(commandRequestWatchdogGive, strings.Fields(line), "*")
		if err == nil || status != statusError {
			t.Errorf("%q was accepted", line)
		}
	}
	status, _, err := s.handleCommand(commandRequestWatchdogForget, nil, "*")
	if err == nil || status != statusError {
		t.Error("forget with no owner was accepted")
	}

	mustDone(t, s, commandRequestWatchdogGive, givenArgs("helper", true)...)
	status, _, err = s.handleCommand(commandRequestWatchdogAnnounce, []string{"dnsr", "med", "10"}, "*")
	if err == nil || status != statusError {
		t.Error("a MED override on given routes was accepted")
	}
}
