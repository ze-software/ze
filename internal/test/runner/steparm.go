// Design: docs/architecture/testing/ci-format.md -- cmd= exit= assertion
// Related: runner_exec.go -- runOrchestrated dispatches each cmd= step on its arm
// Related: record_parse.go -- parseCmd refuses exit= where the arm does not check it

package runner

import (
	"fmt"
)

// stepArm names the arm of runOrchestrated that runs one cmd=background or
// cmd=foreground step. The parser and the dispatcher both derive it from
// classifyStepArm, so which steps have their exit= checked is declared once,
// in assertsExit, rather than once in each reader. The zero value names no
// arm: classifyStepArm answers it only for a mode that is neither.
type stepArm uint8

const (
	stepArmUnspecified stepArm = iota
	// stepArmBackground is started and tracked, and never awaited for its own
	// exit status: the runner kills it at teardown.
	stepArmBackground
	// stepArmHelper is a foreground program that is neither ze nor the
	// harness peer. The runner awaits it before the next step.
	stepArmHelper
	// stepArmQuickZe is a foreground ze subcommand that exits by itself
	// (isQuickExitZeCommand). The runner awaits it before the next step.
	stepArmQuickZe
	// stepArmDaemon is a foreground ze daemon or a foreground le test peer.
	// The runner synchronizes on it and tears it down at the end of the test.
	stepArmDaemon
)

// stepRole is the role name runOrchestrated uses for a step: an `le test peer`
// launch takes binNamePeer, every other step its first word.
func stepRole(cmdParts []string) string {
	if len(cmdParts) == 0 {
		return ""
	}
	if cmdParts[0] == binNameLE && launchesPeer(cmdParts) {
		return binNamePeer
	}
	return cmdParts[0]
}

// classifyStepArm answers which arm of runOrchestrated runs a step of the
// given mode whose exec= split into cmdParts. Port placeholders do not change
// the answer: every test it makes reads flags and verbs, never their values.
func classifyStepArm(mode string, cmdParts []string) stepArm {
	switch mode {
	case modeBackground:
		return stepArmBackground
	case modeForeground:
		// Classified below.
	default:
		return stepArmUnspecified
	}
	role := stepRole(cmdParts)
	if role != binNameZe && role != binNamePeer {
		return stepArmHelper
	}
	if role == binNameZe && isQuickExitZeCommand(cmdParts[1:]) {
		return stepArmQuickZe
	}
	return stepArmDaemon
}

// assertsExit reports whether the arm compares the step's exit status with
// its exit=. It MUST stay true only for the arms in runOrchestrated that
// await the process and compare cmd.ExitCode. runOrchestrated calls
// checkExitAssertable before it dispatches, so an arm that drops exit= is
// refused there too rather than passing in silence.
func (a stepArm) assertsExit() bool {
	switch a {
	case stepArmHelper, stepArmQuickZe:
		return true
	case stepArmUnspecified, stepArmBackground, stepArmDaemon:
		return false
	}
	panic("BUG: assertsExit on an unknown stepArm")
}

// exitNotCheckedError explains why an exit= on a step of this arm would never
// be compared, and what the author writes instead.
func (a stepArm) exitNotCheckedError(directive string) error {
	switch a {
	case stepArmBackground:
		return fmt.Errorf("%s exit= is never checked on a background command: the runner does not wait for a background process to exit, "+
			"it kills it at teardown, so the assertion would pass whatever the command did. "+
			"Fix: run the command as cmd=foreground, or remove exit= and assert what the process produced with expect=", directive)
	case stepArmDaemon:
		return fmt.Errorf("%s exit= is never checked on a foreground ze daemon or le test peer: the runner synchronizes on these "+
			"and tears them down at the end of the test, so their exit status is not compared per command. "+
			"Fix: remove exit= and assert the test's exit status with expect=exit:code=", directive)
	case stepArmUnspecified:
		return fmt.Errorf("%s exit= is only checked on cmd=foreground: no other mode waits for the command to exit. "+
			"Fix: remove exit=", directive)
	case stepArmHelper, stepArmQuickZe:
		panic("BUG: exitNotCheckedError on an arm that checks exit=")
	}
	panic("BUG: exitNotCheckedError on an unknown stepArm")
}

// checkExitAssertable refuses an exit= the step's arm would drop. The parser
// calls it so the author is told when the file is read, and runOrchestrated
// calls it again so a record built without the parser cannot slip past.
func checkExitAssertable(rc *RunCommand) error {
	if rc.ExitCode == nil {
		return nil
	}
	cmdParts, err := SplitCommand(rc.Exec)
	if err != nil {
		return fmt.Errorf("%s exit=: %w", cmdDirective(rc.Mode), err)
	}
	arm := classifyStepArm(rc.Mode, cmdParts)
	if arm.assertsExit() {
		return nil
	}
	return arm.exitNotCheckedError(cmdDirective(rc.Mode))
}

// selectForegroundProcess names the process the end-of-test arms synchronize
// on: the last started process that is either a non-peer or the cmd=foreground
// daemon arm (stepArmDaemon). It answers nil when the file started neither.
//
// peerProcs holds every le test peer, foreground or background, because the
// peers are waited and drained on their own. A cmd=foreground le test peer
// is still the process the file synchronizes on, and its exit is the one
// expect=exit:code= asserts (ci-format.md, "exit= vs expect=exit:code=").
// Skipping it as a peer made the exit-code arm wait on a background ze, which
// never exits on its own, so the test ran to its timeout.
func selectForegroundProcess(started []*ownedProcess, peerProcs map[*ownedProcess]bool, foregroundDaemon *ownedProcess) *ownedProcess {
	var selected *ownedProcess
	for _, p := range started {
		if p == foregroundDaemon {
			selected = p
			continue
		}
		if !peerProcs[p] {
			selected = p
		}
	}
	return selected
}
