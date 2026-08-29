// Design: docs/architecture/core-design.md -- verify-class admission through the shared job registry
// Related: ../job/job.go -- the single admission, locking, and process implementation
package verifylock

import (
	"errors"
	"io"
	"os"

	"github.com/ze-software/ze/internal/le/job"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const (
	name           = "verify lock"
	runAction      = "run"
	labelKeyword   = "label"
	commandKeyword = "command"
	usage          = "run label <label> command <argv...>"
)

// Run executes one verify-class command through lejob's shared admission.
// The child's exit or signal status is returned unchanged.
func Run(root, label string, argv []string, stdout, stderr io.Writer) (job.Report, int, error) {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	admission, err := job.NewIn(root)
	if err != nil {
		return job.Report{}, 2, err
	}
	admission.Out = stdout
	admission.Err = stderr
	report, code := admission.Run(label, argv, "", nil)
	return report, code, nil
}

// Answer runs the native replacement for verify-lock.sh.
func Answer(args []string) (any, int) {
	if len(args) == 0 {
		return leaction.List{
			Area: name,
			Actions: []leaction.Row{{
				Verb: runAction,
				Why:  "admit and run one verify-class command, preserving its exit status",
			}},
		}, 0
	}
	label, argv, err := parse(args)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, code, err := Run(root, label, argv, os.Stdout, os.Stderr)
	if err != nil {
		leaction.ReportError(err)
		return nil, code
	}
	return report, code
}

func parse(args []string) (string, []string, error) {
	if len(args) == 0 || args[0] != runAction {
		return "", nil, errors.New("verify-lock: expected " + usage)
	}
	if len(args) < 3 || args[1] != labelKeyword || args[2] == "" {
		return "", nil, errors.New("verify-lock: expected " + usage)
	}
	if len(args) < 5 || args[3] != commandKeyword {
		return "", nil, errors.New("verify-lock: expected " + usage)
	}
	return args[2], args[4:], nil
}
