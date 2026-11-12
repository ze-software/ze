// Design: docs/functional-tests.md — process orchestration, waiting for a daemon state over SSH
// Related: register_daemon_await_answer.go — the registration of this barrier

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// awaitAnswerDelay spaces the attempts. Each one is a whole `ze cli` process
// and an SSH session, so a shorter delay only loads the daemon it waits on.
const awaitAnswerDelay = 250 * time.Millisecond

// awaitAnswerTailOctets bounds how much of the last answer the error quotes.
const awaitAnswerTailOctets = 2048

// daemonAwaitAnswer is a barrier on a daemon state that no output line orders
// against the steps after it: it runs `ze cli -c <command>` until the command
// exits zero with <needle> in its stdout, so the next step starts only once the
// daemon answers over SSH AND the answer shows the state.
//
// Arguments: <needle> <command>. The client takes its address and credentials
// from the environment the runner gave this process (option=env), exactly as a
// `cmd=foreground:exec=ze cli` step does. A refused connection, a nonzero exit
// and an answer without the needle are all "not yet": the wait ends only on the
// state, or on its budget, a share of the test's own (WaitBudget), with an
// error that quotes the last answer. It never reports success it did not see.
func daemonAwaitAnswer(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: daemon/await-answer <needle> <command>")
	}
	needle, command := args[0], args[1]
	if needle == "" {
		return errors.New("daemon/await-answer: an empty needle matches any answer")
	}
	waitCtx, cancel := context.WithTimeout(ctx, WaitBudget(50, 30*time.Second))
	defer cancel()

	attempts := WaitAttempts(50, awaitAnswerDelay, 120)
	environment := os.Environ()
	var last *awaitAnswerRun
	held := Poll(waitCtx, attempts, awaitAnswerDelay, func() bool {
		var run awaitAnswerRun
		run.code, run.stdout, run.stderr, run.err = runCaptured(waitCtx, environment, "", "ze", "cli", "-c", command)
		// A run the budget's end killed answered nothing: keep the last
		// complete one, which is what the failure quotes.
		if waitCtx.Err() != nil {
			return false
		}
		last = &run
		if run.err != nil {
			return false
		}
		if run.code != 0 {
			return false
		}
		return strings.Contains(run.stdout, needle)
	})
	if !held {
		return awaitAnswerFailure(needle, command, last)
	}
	fmt.Fprintln(os.Stderr, "OK: `ze cli -c "+command+"` answered with "+needle)
	return nil
}

// awaitAnswerRun is one completed `ze cli` run: its exit code and output, or
// the error that kept it from starting.
type awaitAnswerRun struct {
	code   int
	stdout string
	stderr string
	err    error
}

// awaitAnswerFailure names the needle, the command and the last answer seen,
// so the operator reads what the daemon said rather than a bare timeout. A nil
// last means no run completed inside the budget.
func awaitAnswerFailure(needle, command string, last *awaitAnswerRun) error {
	if last == nil {
		return fmt.Errorf("`ze cli -c %s` never answered with %q: no run completed inside the wait budget", command, needle)
	}
	if last.err != nil {
		return fmt.Errorf("`ze cli -c %s` never answered with %q: the last run did not start: %w", command, needle, last.err)
	}
	return fmt.Errorf("`ze cli -c %s` never answered with %q: the last run exited %d; stdout: %q; stderr: %q",
		command, needle, last.code, awaitAnswerTail(last.stdout), awaitAnswerTail(last.stderr))
}

// awaitAnswerTail keeps the last awaitAnswerTailOctets of text, where a
// client's refusal or the end of an answer sits.
func awaitAnswerTail(text string) string {
	if len(text) <= awaitAnswerTailOctets {
		return text
	}
	return text[len(text)-awaitAnswerTailOctets:]
}
