// Design: docs/contributing/running-commands.md -- testing one package while you develop it
// Related: answer.go -- the grammar that reaches this
// Related: report.go -- the fields a quiet run fills, and how they render
//
// A session that runs a job to learn whether a package still passes does not
// want the child's whole output in its transcript. What it wants is the exit
// code, the failure lines, and a path to read when those are not enough.
//
// The pattern this replaces was four steps in one shell command: a scratch
// path from `le session scratch ensure`, a redirect of the job into it, an
// echo of the exit code, and a grep over the log for the lines that matter.
// Each step is here instead, so the answer is one command and every session
// gets the same one.
//
// The registry log cannot serve this: ticket.Release removes it when the job
// ends (registry.go), so a reader who arrives after the run finds nothing.
// This log lives in the session's own scratch directory and stays.

package job

import (
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/runlog"
)

// quietKeyLines is how many failure lines the summary carries. A reader who
// needs more opens the log, whose path the summary names.
const quietKeyLines = 20

// runQuiet runs one job with its output going to this session's scratch log
// rather than to the terminal, and answers the summary of what happened.
//
// The admission, the queueing and the exit code are the ordinary ones. Only
// the destination of the child's output changes, and Admission.Out is where
// every branch of Run sends it: the streamed child, the replay of a job this
// one attached to, and a stage that ran inside a parent's slot.
func runQuiet(adm *Admission, label string, argv []string) (Report, int) {
	report := Report{Label: label, Command: argv, Admission: KindUnspecified.String(), Quiet: true}

	// The log is named for the label, so the name is checked before it becomes
	// a path. Run checks it too, but only after this function has written.
	if !validLabel(label) {
		adm.note(errorLine(ErrLabel))
		report.Code = 2
		return report, report.Code
	}

	relative, err := quietLog(adm.Root, label)
	if err != nil {
		leaction.ReportError(err)
		report.Code = 2
		return report, report.Code
	}

	file, err := os.OpenFile(filepath.Join(adm.Root, relative), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644) //nolint:gosec // this session's own scratch log, named from a validated label
	if err != nil {
		leaction.ReportError(err)
		report.Code = 2
		return report, report.Code
	}

	adm.Out = file
	report, code := adm.Run(label, argv, "", nil)
	report.Quiet = true
	report.Log = relative

	if err := file.Close(); err != nil {
		adm.note(errorLine(err))
	}
	report.KeyLines = quietSummary(adm, filepath.Join(adm.Root, relative))
	return report, code
}

// quietLog answers the checkout-relative path of this session's log for one
// label. Two runs of one label in one session overwrite: the answer is about
// the run in hand, and the previous run's verdict was already read.
func quietLog(root, label string) (string, error) {
	paths, err := lepath.ResolveSession(root, true)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Join(paths.Scratch, "job-"+label+".log")), nil
}

// quietSummary reads back the lines of the log that say what broke. A log this
// process has just written and closed is readable, so a failure to read it is
// reported rather than hidden, and the summary is then empty.
func quietSummary(adm *Admission, path string) []string {
	file, err := os.Open(path) //nolint:gosec // the log this run just wrote
	if err != nil {
		adm.note(errorLine(err))
		return nil
	}
	defer file.Close() //nolint:errcheck // the log is only read

	lines, err := runlog.Key(file, quietKeyLines)
	if err != nil {
		adm.note(errorLine(err))
	}
	return lines
}
