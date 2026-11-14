// Design: docs/contributing/terminal-demos.md -- runtime evidence must reject false greens
// Related: runtime.go -- waitForCommandText and runCommand, the polls and commands these judge
// Related: validate_showcase_topics.go -- showcaseTopicOrder, the chapter order read from the tape

package siteterminaldemo

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// The lines `ip -o link show` prints for a dummy link in each state. The
// format is iproute2's, written from knowledge: the repository holds no
// recorded line of a down link. The up line agrees with the flags
// internal/le/test/deployment/l2tpppp_test.go records
// (`<POINTOPOINT,MULTICAST,NOARP,UP,LOWER_UP>`).
const (
	linkLineDown      = `7: zetrk1: <BROADCAST,NOARP> mtu 1500 qdisc noqueue state DOWN mode DEFAULT group default qlen 1000\    link/ether 6a:5b:1c:9d:2e:3f brd ff:ff:ff:ff:ff:ff`
	linkLineNoCarrier = `7: zetrk1: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc noqueue state DOWN mode DEFAULT group default qlen 1000\    link/ether 6a:5b:1c:9d:2e:3f brd ff:ff:ff:ff:ff:ff`
	linkLineUp        = `7: zetrk1: <BROADCAST,NOARP,UP,LOWER_UP> mtu 1500 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000\    link/ether 6a:5b:1c:9d:2e:3f brd ff:ff:ff:ff:ff:ff`
)

func answerOf(text string) func() (string, error) {
	return func() (string, error) { return text, nil }
}

// PREVENTS: a readiness word satisfied by a lower-case look-alike. `UP` was
// found in `group default`, which a down link prints too.
// Method: one attempt over a down link's line, waiting for `UP`.
// MUTATION: comparing `strings.ToLower(output)` with `strings.ToLower(expected)`
// in waitForCommandText turns this red.
func TestWaitForCommandTextIsCaseSensitive(t *testing.T) {
	if _, err := waitForCommandText(1, "UP", answerOf(linkLineDown)); err == nil {
		t.Fatal("a down link's line satisfied a wait for UP")
	}
	if _, err := waitForCommandText(1, "state DOWN", answerOf(linkLineDown)); err != nil {
		t.Fatalf("text present in the answer was not found: %v", err)
	}
}

// PREVENTS: the tracked uplink reported ready while it is down, which made the
// showcase-lab check after `tracked-uplink down` and `up` prove nothing.
// Method: the fixture's own readiness target against the line of a down link,
// of a link with no carrier, and of a link that is up.
// MUTATION: the target `UP` in showcaseTrackedFixture turns the no-carrier case
// red; with the folded comparison it turns the down case red too.
func TestTrackedUplinkReadinessNeedsAnUpLink(t *testing.T) {
	ready := showcaseTrackedFixture().ready
	if len(ready) != 1 {
		t.Fatalf("tracked uplink declares %d readiness observations, want 1", len(ready))
	}
	for name, line := range map[string]string{"down": linkLineDown, "no carrier": linkLineNoCarrier} {
		if _, err := waitForCommandText(1, ready[0].target, answerOf(line)); err == nil {
			t.Errorf("%s: target %q accepted the line %s", name, ready[0].target, line)
		}
	}
	if _, err := waitForCommandText(1, ready[0].target, answerOf(linkLineUp)); err != nil {
		t.Errorf("target %q refused an up link: %v", ready[0].target, err)
	}
}

// PREVENTS: a hung command stalling a check until the container's one-hour
// limit, with no message naming it.
// Method: run `sleep 30` under a 200 millisecond deadline and read the error.
// MUTATION: `context.Background()` in place of the `context.WithTimeout` in
// beginCommand lets the sleep finish and turns this red.
func TestRunCommandStopsAtItsDeadline(t *testing.T) {
	started := time.Now()
	_, err := runCommand("sleep", []string{"30"}, commandOptions{deadline: 200 * time.Millisecond})
	if !errors.Is(err, errCommandDeadline) {
		t.Fatalf("error = %v, want the deadline error", err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("the command ran %s past a 200ms deadline", elapsed)
	}
	for _, part := range []string{`"sleep 30"`, "200ms", "./le site terminal-demo check validation"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("deadline error does not name %q: %v", part, err)
		}
	}
}

// PREVENTS: a poll retrying a hung command for every one of its attempts.
// Method: an answer that always reports the deadline error, counted.
// MUTATION: removing the `errors.Is(err, errCommandDeadline)` return from
// waitForCommandText runs all five attempts and turns this red.
func TestWaitForCommandTextEndsOnAHungCommand(t *testing.T) {
	attempts := 0
	_, err := waitForCommandText(5, "ready", func() (string, error) {
		attempts++
		return "", errCommandDeadline
	})
	if !errors.Is(err, errCommandDeadline) {
		t.Fatalf("error = %v, want the deadline error", err)
	}
	if attempts != 1 {
		t.Fatalf("a hung command was run %d times, want 1", attempts)
	}
}

// PREVENTS: a poll built on awaitCheck retrying a hung command for every one
// of its attempts (600 for the IPsec topic), each one a full command deadline.
// Method: a check that always reports the deadline error, counted, then a
// check that reports an ordinary error, which must use every attempt.
// MUTATION: removing the `errors.Is(last, errCommandDeadline)` return from
// awaitCheck runs all five attempts and turns this red.
func TestAwaitCheckEndsOnAHungCommand(t *testing.T) {
	attempts := 0
	err := awaitCheck(5, func() error {
		attempts++
		return errCommandDeadline
	})
	if !errors.Is(err, errCommandDeadline) {
		t.Fatalf("error = %v, want the deadline error", err)
	}
	if attempts != 1 {
		t.Fatalf("a hung command was run %d times, want 1", attempts)
	}
	notYet := errors.New("not converged")
	attempts = 0
	err = awaitCheck(2, func() error {
		attempts++
		return notYet
	})
	if !errors.Is(err, notYet) {
		t.Fatalf("error = %v, want the last ordinary error", err)
	}
	if attempts != 2 {
		t.Fatalf("an ordinary failure used %d attempts, want both of 2", attempts)
	}
	attempts = 0
	err = awaitCheck(5, func() error {
		attempts++
		if attempts < 2 {
			return notYet
		}
		return nil
	})
	if err != nil || attempts != 2 {
		t.Fatalf("a check that passed on its second attempt: error %v after %d attempts", err, attempts)
	}
}

// PREVENTS: a poll that ignores a failed read missing that the read hung, and
// the opposite, an ordinary failed read ending a poll that should retry.
// Method: errors with and without the deadline mark, wrapped as runCommand
// wraps it.
// MUTATION: `return nil` in place of the `errors.Is` return in hungCommand
// turns the deadline cases red; returning the first non-nil error turns the
// ordinary case red.
func TestHungCommandFindsTheDeadlineError(t *testing.T) {
	ordinary := errors.New("exit status 1")
	// The mark as runCommand's callers hand it on, under their own context.
	hung := errors.Join(errors.New("read peers"), errCommandDeadline)
	if err := hungCommand(); err != nil {
		t.Fatalf("no error at all was read as a hung command: %v", err)
	}
	if err := hungCommand(nil, ordinary); err != nil {
		t.Fatalf("an ordinary failure was read as a hung command: %v", err)
	}
	for name, errs := range map[string][]error{
		"deadline after an ordinary failure": {ordinary, hung},
		"deadline after a read that passed":  {nil, hung},
		"deadline first":                     {hung, ordinary},
	} {
		err := hungCommand(errs...)
		if !errors.Is(err, errCommandDeadline) {
			t.Errorf("%s: answer = %v, want the deadline error", name, err)
			continue
		}
		if !strings.Contains(err.Error(), "read peers") {
			t.Errorf("%s: the answer lost the caller's context: %v", name, err)
		}
	}
}

// PREVENTS: a validator building a directive the recorder does not read as
// one, which it then types into the session as plain text.
// Method: each builder's output through the recorder's own reader
// (splitPTYDirective, validatePTYCommand, parsePTYOptions), then every command
// of the two dashboard sessions the validators build.
// MUTATION: a literal of another spelling in a builder (`"@pause "` in
// ptySleep) turns its case red; a builder that answers its argument alone
// turns its case red; a ptyKeys entry spelled other than keyEnter turns the
// Enter case and both sessions red.
func TestBuiltDirectivesAreTheOnesTheRecorderReads(t *testing.T) {
	for _, tc := range []struct {
		name, built, directive, argument string
	}{
		{"wait", ptyWait("Commit accepted"), tapeWaitDirective, "Commit accepted"},
		{"type", ptyType("show"), tapeTypeDirective, "show"},
		{"enter key", ptyKey(keyEnter), tapeKeyDirective, keyEnter},
		{"down key", ptyKey(ptyKeyDown), tapeKeyDirective, ptyKeyDown},
		{"sleep", ptySleep("16"), tapeSleepDirective, "16"},
	} {
		directive, argument := splitPTYDirective(tc.built)
		if directive != tc.directive || argument != tc.argument {
			t.Errorf("%s: %q reads as directive %q with argument %q, want %q and %q",
				tc.name, tc.built, directive, argument, tc.directive, tc.argument)
		}
		if err := validatePTYCommand(tc.built); err != nil {
			t.Errorf("%s: the recorder refuses %q: %v", tc.name, tc.built, err)
		}
	}
	if enter := ptyKeys[keyEnter]; !slices.Equal(enter, []byte{'\r'}) {
		t.Errorf("the Enter key sends %q, want a carriage return", enter)
	}
	// The recorder refuses a session that ends on a sleep or a wait, so a
	// refusal here shows it read the built command as that directive.
	for name, last := range map[string]string{"sleep": ptySleep("1"), "wait": ptyWait("ready")} {
		_, err := parsePTYOptions([]string{flagCommand, "show version", flagCommand, last, "--", "ze"})
		if err == nil {
			t.Errorf("a session ending on a built %s was accepted, so it was read as plain text", name)
		}
	}
	for name, arguments := range map[string][]string{
		"showcase":      showcaseDashboardSession(),
		"cli-dashboard": cliDashboardSession(),
	} {
		options, err := parsePTYOptions(arguments)
		if err != nil {
			t.Errorf("%s: the recorder refuses the session the validator builds: %v", name, err)
			continue
		}
		directives := 0
		for _, command := range options.commands {
			if directive, _ := splitPTYDirective(command); directive != "" {
				directives++
			}
		}
		if directives == 0 {
			t.Errorf("%s: the session holds no directive, so this judged nothing: %q", name, options.commands)
		}
	}
}

// PREVENTS: a timeout that ends in a blank line, which cannot be told from a
// message that dropped the command's answer.
// Method: one attempt over an empty answer, then over an answer without the text.
func TestWaitForCommandTextTimeoutSaysWhatItSaw(t *testing.T) {
	_, err := waitForCommandText(1, "0.0.0.0:500", answerOf("\n"))
	if err == nil || !strings.Contains(err.Error(), "printed nothing") {
		t.Fatalf("empty answer: error = %v, want it to say the command printed nothing", err)
	}
	_, err = waitForCommandText(1, "0.0.0.0:500", answerOf("UNCONN 0 0 *:4500 *:*"))
	if err == nil || !strings.Contains(err.Error(), "*:4500") {
		t.Fatalf("other answer: error = %v, want the last answer in it", err)
	}
}

// PREVENTS: the hidden router started under `ip netns exec`, whose fresh /sys
// hides cgroup2, so its IPsec override guard cannot attach and it never
// listens for IKE.
// Method: read the start command the VPN fixture builds.
// MUTATION: `ip netns exec showcase-vpn ze start` in showcaseVPNDaemonCommand
// turns this red.
func TestShowcaseVPNDaemonEntersItsNamespaceWithNsenter(t *testing.T) {
	name, args := showcaseVPNDaemonCommand()
	want := append([]string{"--net=/run/netns/" + showcaseVPNNamespace, "ze"}, daemonArgs()...)
	if name != "nsenter" {
		t.Fatalf("the hidden router starts under %q, want nsenter", name)
	}
	if !slices.Equal(args, want) {
		t.Fatalf("arguments = %q, want %q", args, want)
	}
}

// PREVENTS: counting the open draft as a stored revision.
// Method: the JSON answer shape of `show config history`, a draft row first.
func TestCountConfigRevisions(t *testing.T) {
	const answer = `[{"revision": "draft", "state": "editing in progress"},
 {"revision": 1, "timestamp": "2026-11-14 10:00:00", "path": "a"},
 {"revision": 2, "timestamp": "2026-11-14 10:00:15", "path": "b"}]`
	revisions, err := countConfigRevisions(answer)
	if err != nil {
		t.Fatal(err)
	}
	if revisions != 2 {
		t.Fatalf("counted %d revisions, want 2", revisions)
	}
	if empty, err := countConfigRevisions("[]"); err != nil || empty != 0 {
		t.Fatalf("an empty history counted %d revisions, error %v", empty, err)
	}
	if _, err := countConfigRevisions("revisions:"); err == nil {
		t.Fatal("an answer that is not JSON was counted")
	}
}

// PREVENTS: a confirmed trial that never reached the store passing the history
// check. The former floor of two revisions was met before the trial started.
// Method: revision counts before the trial and after its rollback.
// MUTATION: `after >= 2` in requireTrialRecorded accepts the first two cases
// and turns this red.
func TestRequireTrialRecorded(t *testing.T) {
	for _, tc := range []struct {
		name          string
		before, after int
		accepted      bool
	}{
		{"trial never applied", 3, 3, false},
		{"trial applied and never reverted", 3, 4, false},
		{"trial applied and reverted", 3, 5, true},
		{"first commits of a new store", 0, 2, true},
	} {
		err := requireTrialRecorded(tc.before, tc.after)
		if tc.accepted {
			if err != nil {
				t.Errorf("%s: refused: %v", tc.name, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: %d revisions before and %d after were accepted", tc.name, tc.before, tc.after)
		}
	}
}

// PREVENTS: a ping that could not run standing in for hosts that cannot
// communicate. The base check accepted any error from the probe.
// Method: the three outcomes of a probe, judged from its answer and error.
// MUTATION: `return nil` in place of the `0 received` requirement in
// requireNoPingReply accepts the probe that could not run.
func TestRequireNoPingReply(t *testing.T) {
	failed := errors.New("exit status 1")
	const unanswered = "PING 10.78.2.2 (10.78.2.2) 56(84) bytes of data.\n\n--- 10.78.2.2 ping statistics ---\n1 packets transmitted, 0 received, 100% packet loss, time 0ms\n"
	if err := requireNoPingReply(unanswered, failed); err != nil {
		t.Errorf("an unanswered probe was refused: %v", err)
	}
	if err := requireNoPingReply("", failed); err == nil {
		t.Error("a probe that printed no summary was accepted as unanswered")
	}
	const answered = "1 packets transmitted, 1 received, 0% packet loss, time 0ms\n"
	if err := requireNoPingReply(answered, nil); err == nil {
		t.Error("an answered probe was accepted")
	}
}

// PREVENTS: a new live topic whose commit adds a new top-level container
// being accepted as a clean base because nobody edited the base guard.
// Method: a registry holding one topic the guard has never heard of, over a
// tree without and then with the container that topic declares.
// MUTATION: a fixed path list in place of the registry walk in
// requireLiveTopicsAbsent accepts the second tree.
func TestBaseGuardRunsEveryRegisteredTopicDeclaration(t *testing.T) {
	pass := func() error { return nil }
	registry := scenarioRegistry{
		"not-a-topic": {validate: pass},
		"new-topic": {validate: pass, live: &liveTopic{
			fragment: "topics/new/configure.tape", check: pass, absent: topicContainersAbsent("new-feature"),
		}},
	}
	tree := config.NewTree()
	if err := requireLiveTopicsAbsent(registry, tree); err != nil {
		t.Fatalf("a tree without the topic's container was refused: %v", err)
	}
	tree.GetOrCreateContainer("new-feature")
	err := requireLiveTopicsAbsent(registry, tree)
	if err == nil {
		t.Fatal("a tree that already holds the new topic's container was accepted")
	}
	for _, part := range []string{"live topic new-topic", "topic new-feature"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("refusal does not name %q: %v", part, err)
		}
	}
}

// PREVENTS: a live topic registered with no declaration of what its commit
// adds, which the base guard could then never refuse.
// Method: register with a nil declaration and expect the init-time panic; the
// panic comes before the registry is written, so the registry is unchanged.
func TestRegisterLiveTopicRequiresAnAbsenceDeclaration(t *testing.T) {
	pass := func() error { return nil }
	defer func() {
		if recover() == nil {
			t.Fatal("a live topic with no absence declaration was registered")
		}
		if _, err := scenarios.lookup("undeclared-topic"); err == nil {
			t.Fatal("the refused topic reached the registry")
		}
	}()
	registerLiveTopic("undeclared-topic", "topics/undeclared/configure.tape", nil, pass, func(string) error { return nil })
}

// ipsecPolicyRows answers the rows `show vpn ipsec dataplane policy` holds at
// the VPN interface's if_id on the lab's Linux XFRM backend: the Child SA's
// three protect entries, and the interface SPD's final discard for in and out
// in both families. The roles are the producer's own names
// (dataplane.PolicyRole.String); the other words are what policyInfoRow
// renders (internal/component/ike/cmd/show_dataplane.go). Each call answers
// fresh slices, so a case edits its own copy.
func ipsecPolicyRows() (protect, catchAll []topicIPsecPolicy) {
	childSA := dataplane.PolicyRoleChildSA.String()
	inbound := topicIPsecPolicy{IfID: topicIPsecIfID, Direction: ipsecDirectionIn, Role: childSA,
		Source: "10.78.2.0/24", Target: "10.78.1.0/24",
		TunnelSrc: showcaseVPNPeerEndpoint, TunnelDst: showcaseVPNZeEndpoint,
		Action: "protect", Mode: "tunnel", Owner: ipsecResponderPeer, OwnerKnown: true}
	forward := inbound
	forward.Direction = ipsecDirectionForward
	outbound := topicIPsecPolicy{IfID: topicIPsecIfID, Direction: ipsecDirectionOut, Role: childSA,
		Source: "10.78.1.0/24", Target: "10.78.2.0/24",
		TunnelSrc: showcaseVPNZeEndpoint, TunnelDst: showcaseVPNPeerEndpoint,
		Action: "protect", Mode: "tunnel", Owner: ipsecResponderPeer, OwnerKnown: true}
	protect = []topicIPsecPolicy{inbound, forward, outbound}
	for _, direction := range ipsecFinalDirections {
		for _, everything := range ipsecWildcardSelectors {
			catchAll = append(catchAll, ipsecFinalDiscardRow(direction, everything))
		}
	}
	return protect, catchAll
}

// ipsecFinalDiscardRow answers one final discard row of the interface's SPD.
func ipsecFinalDiscardRow(direction, everything string) topicIPsecPolicy {
	return topicIPsecPolicy{IfID: topicIPsecIfID, Direction: direction,
		Role:   dataplane.PolicyRoleInterfaceCatchAll.String(),
		Source: everything, Target: everything,
		Action: ipsecActionDiscard, Mode: "unknown", Owner: "unknown"}
}

// PREVENTS: the IPsec check reading the interface SPD's final discard entries
// as protect entries that differ, which failed the healthy stage once the
// engine installed them, and the opposite error, a check that passes with a
// protect entry or a final discard missing or wrong.
// Method: the rows of a healthy and of a refused Child SA, in either order,
// then one defect at a time, each judged by the message it must produce.
// MUTATION: removing the `got.Role == dataplane.PolicyRoleInterfaceCatchAll.String()`
// branch in checkTopicIPsecPolicies turns every accepted case red. Removing
// the `ipsecFinalDirections` loop turns the missing-discard cases red.
// Removing the `got.Action != ipsecActionDiscard` guard in
// checkTopicIPsecCatchAll turns the bypass case red. Removing the
// `ipsecProtectDirections` loop turns the missing-protect cases red. Replacing
// `*got != want` with a comparison of the selectors alone turns the tunnel
// endpoint, mode, action, owner and role cases red. Removing the `!protected`
// refusal turns the last case red.
func TestCheckTopicIPsecPoliciesTellsProtectFromFinalDiscard(t *testing.T) {
	type policyCase struct {
		name      string
		rows      func() []topicIPsecPolicy
		protected bool
		refusal   string // empty when the rows must be accepted
	}
	// edited answers the healthy rows, protect first, after one edit.
	edited := func(edit func(protect, catchAll []topicIPsecPolicy) (p, c []topicIPsecPolicy)) func() []topicIPsecPolicy {
		return func() []topicIPsecPolicy {
			protect, catchAll := edit(ipsecPolicyRows())
			return slices.Concat(protect, catchAll)
		}
	}
	protectEdit := func(index int, change func(*topicIPsecPolicy)) func() []topicIPsecPolicy {
		return edited(func(protect, catchAll []topicIPsecPolicy) (p, c []topicIPsecPolicy) {
			change(&protect[index])
			return protect, catchAll
		})
	}
	cases := []policyCase{
		{name: "final discards listed before the protect entries", protected: true,
			rows: func() []topicIPsecPolicy {
				protect, catchAll := ipsecPolicyRows()
				nodeWide := topicIPsecPolicy{Direction: ipsecDirectionOut, Role: dataplane.PolicyRoleCatchAll.String(),
					Source: "0.0.0.0/0", Target: "0.0.0.0/0", Action: "bypass", Mode: "unknown", Owner: "unknown"}
				return slices.Concat([]topicIPsecPolicy{nodeWide}, catchAll, protect)
			}},
		{name: "final discards listed after the protect entries", protected: true,
			rows: edited(func(protect, catchAll []topicIPsecPolicy) (p, c []topicIPsecPolicy) { return protect, catchAll })},
		{name: "final discards alone under a refused proposal",
			rows: edited(func(_, catchAll []topicIPsecPolicy) (p, c []topicIPsecPolicy) { return nil, catchAll })},
		{name: "a backend that lists forward final discards too", protected: true,
			rows: edited(func(protect, catchAll []topicIPsecPolicy) (p, c []topicIPsecPolicy) {
				for _, everything := range ipsecWildcardSelectors {
					catchAll = append(catchAll, ipsecFinalDiscardRow(ipsecDirectionForward, everything))
				}
				return protect, catchAll
			})},
		{name: "final discards alone when a Child SA exists", protected: true,
			refusal: "kernel lacks the protected in policy",
			rows:    edited(func(_, catchAll []topicIPsecPolicy) (p, c []topicIPsecPolicy) { return nil, catchAll })},
		{name: "protect entries with no final discard", protected: true,
			refusal: "lacks its final discard entry for in 0.0.0.0/0",
			rows:    edited(func(protect, _ []topicIPsecPolicy) (p, c []topicIPsecPolicy) { return protect, nil })},
		{name: "a final discard that lets traffic through", protected: true,
			refusal: "has action bypass, not discard",
			rows: edited(func(protect, catchAll []topicIPsecPolicy) (p, c []topicIPsecPolicy) {
				catchAll[len(catchAll)-1].Action = "bypass"
				return protect, catchAll
			})},
		{name: "a final discard that does not match everything", protected: true,
			refusal: "final in entry does not match everything",
			rows: edited(func(protect, catchAll []topicIPsecPolicy) (p, c []topicIPsecPolicy) {
				catchAll[0].Source, catchAll[0].Target = "10.0.0.0/8", "10.0.0.0/8"
				return protect, catchAll
			})},
		{name: "a forward final discard that lets traffic through", protected: true,
			refusal: "has action bypass, not discard",
			rows: edited(func(protect, catchAll []topicIPsecPolicy) (p, c []topicIPsecPolicy) {
				forward := ipsecFinalDiscardRow(ipsecDirectionForward, "::/0")
				forward.Action = "bypass"
				return protect, append(catchAll, forward)
			})},
		{name: "out protect entry to another network", protected: true,
			refusal: "out policy differs from the protected peer",
			rows:    protectEdit(2, func(row *topicIPsecPolicy) { row.Target = "10.78.3.0/24" })},
		{name: "out protect entry to another tunnel endpoint", protected: true,
			refusal: "out policy differs from the protected peer",
			rows:    protectEdit(2, func(row *topicIPsecPolicy) { row.TunnelDst = "172.30.0.3" })},
		{name: "in protect entry from another tunnel endpoint", protected: true,
			refusal: "in policy differs from the protected peer",
			rows:    protectEdit(0, func(row *topicIPsecPolicy) { row.TunnelSrc = "172.30.0.3" })},
		{name: "in protect entry in transport mode", protected: true,
			refusal: "in policy differs from the protected peer",
			rows:    protectEdit(0, func(row *topicIPsecPolicy) { row.Mode = "transport" })},
		{name: "fwd entry that bypasses", protected: true,
			refusal: "fwd policy differs from the protected peer",
			rows:    protectEdit(1, func(row *topicIPsecPolicy) { row.Action = "bypass" })},
		{name: "in protect entry of another peer", protected: true,
			refusal: "in policy differs from the protected peer",
			rows:    protectEdit(0, func(row *topicIPsecPolicy) { row.Owner = "initiator" })},
		{name: "in protect entry with no known owner", protected: true,
			refusal: "in policy differs from the protected peer",
			rows:    protectEdit(0, func(row *topicIPsecPolicy) { row.OwnerKnown = false })},
		{name: "protect entry an operator installed", protected: true,
			refusal: "out policy differs from the protected peer",
			rows:    protectEdit(2, func(row *topicIPsecPolicy) { row.Role = dataplane.PolicyRoleOperator.String() })},
		{name: "an extra entry Ze holds no role for", protected: true,
			refusal: "out policy differs from the protected peer",
			rows: edited(func(protect, catchAll []topicIPsecPolicy) (p, c []topicIPsecPolicy) {
				foreign := ipsecFinalDiscardRow(ipsecDirectionOut, "0.0.0.0/0")
				foreign.Role = dataplane.PolicyRoleUnspecified.String()
				return append(protect, foreign), catchAll
			})},
		{name: "an entry in a direction the row cannot name", protected: true,
			refusal: `unknown direction "unknown"`,
			rows:    protectEdit(0, func(row *topicIPsecPolicy) { row.Direction = "unknown" })},
		{name: "a protect entry left under a refused proposal",
			refusal: "refused ESP left kernel in policy installed",
			rows:    edited(func(protect, catchAll []topicIPsecPolicy) (p, c []topicIPsecPolicy) { return protect[:1], catchAll })},
	}
	for index, direction := range ipsecProtectDirections {
		cases = append(cases, policyCase{name: "no " + direction + " protect entry", protected: true,
			refusal: "kernel lacks the protected " + direction + " policy",
			rows: edited(func(protect, catchAll []topicIPsecPolicy) (p, c []topicIPsecPolicy) {
				return slices.Delete(protect, index, index+1), catchAll
			})})
	}
	_, finals := ipsecPolicyRows()
	for index := range finals {
		missing := finals[index].Direction + " " + finals[index].Source
		cases = append(cases, policyCase{name: "no final discard for " + missing, protected: true,
			refusal: "lacks its final discard entry for " + missing,
			rows: edited(func(protect, catchAll []topicIPsecPolicy) (p, c []topicIPsecPolicy) {
				return protect, slices.Delete(catchAll, index, index+1)
			})})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkTopicIPsecPolicies(tc.rows(), tc.protected)
			if tc.refusal == "" {
				if err != nil {
					t.Fatalf("rows refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("rows accepted, want a refusal holding %q", tc.refusal)
			}
			if !strings.Contains(err.Error(), tc.refusal) {
				t.Fatalf("refusal %q does not hold %q", err, tc.refusal)
			}
		})
	}
}

// liveTopicFragments answers the registered live topics and their fragments,
// in id order.
func liveTopicFragments(t *testing.T) (ids, fragments []string) {
	t.Helper()
	ids = liveTopicIDs(scenarios)
	if len(ids) < 2 {
		t.Fatalf("%d live topics are registered, the order tests need two", len(ids))
	}
	for _, id := range ids {
		fragments = append(fragments, scenarios[id].live.fragment)
	}
	return ids, fragments
}

// writeShowcaseTape points the demo root at a private tree whose showcase tape
// sources the given fragments, in order.
func writeShowcaseTape(t *testing.T, fragments []string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("ZE_DEMO_ROOT", root)
	directory := filepath.Join(root, "demos", "terminal", "showcase")
	if err := os.MkdirAll(directory, 0o750); err != nil {
		t.Fatal(err)
	}
	lines := []string{"Output artifacts/showcase.cast"}
	for _, fragment := range fragments {
		lines = append(lines, tapeSourceCommand+" "+fragment)
	}
	if err := os.WriteFile(filepath.Join(directory, "demo.tape"), []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: the showcase validator runs the chapters in the order its tape
// sources them, not in an order of its own.
// Method: a tape that sources every live topic, then the same tape reversed.
func TestShowcaseTopicOrderFollowsTheTape(t *testing.T) {
	ids, fragments := liveTopicFragments(t)
	writeShowcaseTape(t, fragments)
	order, err := showcaseTopicOrder()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(order, ids) {
		t.Fatalf("order = %q, want %q", order, ids)
	}
	slices.Reverse(ids)
	slices.Reverse(fragments)
	writeShowcaseTape(t, fragments)
	order, err = showcaseTopicOrder()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(order, ids) {
		t.Fatalf("reversed tape: order = %q, want %q", order, ids)
	}
}

// PREVENTS: a showcase that drops a chapter passing validation.
// Method: a tape that sources every live topic but the first.
// MUTATION: removing the `!seen[id]` refusal from showcaseTopicOrder turns
// this red.
func TestShowcaseTopicOrderRefusesAMissingChapter(t *testing.T) {
	ids, fragments := liveTopicFragments(t)
	writeShowcaseTape(t, fragments[1:])
	_, err := showcaseTopicOrder()
	if err == nil {
		t.Fatalf("a showcase without chapter %s was accepted", ids[0])
	}
	if want := "does not source live topic " + ids[0] + " "; !strings.Contains(err.Error(), want) {
		t.Fatalf("refusal %q does not name %q", err, want)
	}
}

// PREVENTS: a showcase that plays one chapter twice passing validation, whose
// second run would start from a lab the first already changed.
// Method: a tape that sources every live topic, and the first one again.
// MUTATION: removing the `seen[id]` refusal from showcaseTopicOrder turns
// this red.
func TestShowcaseTopicOrderRefusesARepeatedChapter(t *testing.T) {
	ids, fragments := liveTopicFragments(t)
	writeShowcaseTape(t, append(fragments, fragments[0]))
	_, err := showcaseTopicOrder()
	if err == nil {
		t.Fatalf("a showcase that repeats chapter %s was accepted", ids[0])
	}
	if want := "sources live topic " + ids[0] + " more than once"; !strings.Contains(err.Error(), want) {
		t.Fatalf("refusal %q does not name %q", err, want)
	}
}
