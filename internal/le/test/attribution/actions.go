// Design: docs/contributing/running-commands.md -- le test attribution report
// Overview: attribution.go -- the comparison these wire to the real world
//
// actions.go is the command surface of `le test attribution`, and the real
// Runner, HeadReader and OverlayWriter: job admission, git and the shared
// overlay writer.

package testattribution

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gitenv"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/job"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/runlog"
	testoverlay "github.com/ze-software/ze/internal/le/test/overlay"
)

const area = "test attribution"

// The keywords the report takes. A value never stands alone (ai/rules/cli.md).
const (
	keyLog     = "log"
	keyPackage = "package"
	keyTags    = "tags"
	keyFile    = "file"
)

// overlayArea is the session scratch subdirectory the HEAD overlays live in.
const overlayArea = "attribution"

// jobLabel is the admission label of each re-run.
const jobLabel = "attribution"

// gitDeadline bounds one git read of HEAD.
const gitDeadline = 30 * time.Second

var actions = leaction.New(area, leaction.Action{
	Verb:  "report",
	Heavy: true,
	// admittedRunner takes one slot per re-run, so the report as a whole
	// holds none.
	AdmitsItself: true,
	Default:      true,
	Summary:      "tell which failing tests your own files cause",
	Why: "re-run each test a package log reports failed, once as on disk and once with the named files at " +
		"HEAD through a Go overlay, and classify it: caused, added, not-caused, undetermined, " +
		"red-at-head-only, not-reproduced or not-run. Exit 1 when a row is caused or added, 2 when a " +
		"row reached no verdict, 0 otherwise. Tests the log reports passed are not re-run",
	Parameters: []leaction.Parameter{
		{Keyword: keyLog, Kind: leaction.KindPath, Value: "path", Requirement: leaction.Required,
			Help: "the go test log a quiet package run wrote"},
		{Keyword: keyPackage, Kind: leaction.KindPath, Value: "dir", Requirement: leaction.Required,
			Help: "the package directory the log ran"},
		{Keyword: keyTags, Kind: leaction.KindString, Value: "tags", Requirement: leaction.Required,
			Help: "the build tags the log's run used"},
		{Keyword: keyFile, Kind: leaction.KindPath, Value: "path", Requirement: leaction.Required, Repeat: true,
			Help: "one of your session's files, put back to HEAD"},
	},
	AnswerArgs: answerReport,
})

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le test attribution` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// answerReport reads the log, resolves the paths, and runs the attribution
// with the real runner. Anything that stops the comparison answers 2.
func answerReport(args leaction.Arguments) (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		return nil, refuse(err)
	}
	request, err := buildRequest(tree, args)
	if err != nil {
		return nil, refuse(err)
	}
	toolchain, err := gotoolchain.New(tree)
	if err != nil {
		return nil, refuse(err)
	}
	attributor := Attributor{
		Run:     admittedRunner(tree, testoverlay.UnitEnvironment(toolchain)),
		Head:    headReader(tree, "HEAD"),
		Write:   func(files []testoverlay.File) (string, error) { return testoverlay.Write(tree, overlayArea, files...) },
		Timeout: toolchain.Timeout,
	}
	report, err := attributor.Attribute(request)
	if err != nil {
		return nil, refuse(err)
	}
	return report, report.ExitCode()
}

// refuse reports why no attribution was made and answers 2.
func refuse(err error) int {
	var tb textbuf.Buffer
	leaction.ReportError(errors.New(tb.Str("test attribution: ").Err(err).String()))
	return exitNoVerdict
}

// buildRequest reads the failing tests from the log and resolves the package
// and the files against the tree.
func buildRequest(tree string, args leaction.Arguments) (Request, error) {
	logPath := args.One(keyLog)
	failed, err := readFailed(logPath)
	if err != nil {
		return Request{}, err
	}
	pkg, err := CleanPath(args.One(keyPackage))
	if err != nil {
		return Request{}, err
	}
	if strings.Contains(pkg, "...") {
		return Request{}, errors.New(pkg + ": name one package directory, not a pattern; " +
			"run the report once for each package the log failed")
	}
	info, err := os.Stat(filepath.Join(tree, filepath.FromSlash(pkg)))
	if err != nil {
		return Request{}, errors.New(pkg + ": no such package directory in this checkout; " +
			"name the directory relative to the checkout root, as internal/le/commit")
	}
	if !info.IsDir() {
		return Request{}, errors.New(pkg + ": is a file; name the package directory that holds it")
	}
	files := make([]string, 0, len(args.Values(keyFile)))
	for _, raw := range args.Values(keyFile) {
		rel, err := CleanPath(raw)
		if err != nil {
			return Request{}, err
		}
		files = append(files, rel)
	}
	return Request{Log: logPath, Failed: failed, Package: pkg, Tags: args.One(keyTags), Files: files}, nil
}

// readFailed answers the failing tests of one log through runlog, which owns
// what a failure line looks like.
func readFailed(logPath string) ([]string, error) {
	file, err := os.Open(logPath) //nolint:gosec // the caller names the log it wants read
	if err != nil {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("cannot read the log ").Str(logPath).Str(": ").Err(err).
			Str("; pass the log path a ./le job run ... quiet line printed").String())
	}
	defer file.Close() //nolint:errcheck // read-only file, nothing to flush

	return runlog.FailedTests(file)
}

// headReader answers the HeadReader that reads files at revision through git.
// The command passes HEAD; a test passes a tree object it wrote.
func headReader(tree, revision string) HeadReader {
	return func(rel string) (testoverlay.File, error) {
		listed, err := gitOutput(tree, "ls-tree", revision, "--", rel)
		if err != nil {
			return testoverlay.File{}, err
		}
		if listed == "" {
			if _, statErr := os.Stat(filepath.Join(tree, filepath.FromSlash(rel))); statErr != nil {
				return testoverlay.File{}, errors.New(rel + ": no such file on disk or at HEAD; " +
					"name a file your session changed, relative to the checkout root")
			}
			// HEAD lacks the file: the session added it, so the HEAD side
			// builds as if it did not exist.
			return testoverlay.File{Rel: rel, Absent: true}, nil
		}
		fields := strings.Fields(listed)
		if len(fields) < 2 {
			return testoverlay.File{}, errors.New(rel + ": git ls-tree answered a line with no object type: " + listed)
		}
		if fields[1] != "blob" {
			return testoverlay.File{}, errors.New(rel + ": names a " + fields[1] + " at HEAD, not a file; " +
				"name each file with its own file <path>")
		}
		content, err := gitRaw(tree, "show", revision+":"+rel)
		if err != nil {
			return testoverlay.File{}, err
		}
		return testoverlay.File{Rel: rel, Content: content}, nil
	}
}

// gitOutput runs one read-only git command in tree and answers its trimmed
// output.
func gitOutput(tree string, args ...string) (string, error) {
	out, err := gitRaw(tree, args...)
	return strings.TrimSpace(out), err
}

// gitRaw runs one read-only git command in tree and answers its output as is,
// so a file read from HEAD keeps its last newline.
func gitRaw(tree string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitDeadline)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", tree}, args...)...) //nolint:gosec // fixed git verbs; the path is an argv operand
	// The deadline SIGKILLs git; a read-only environment keeps a killed git
	// from leaving an index lock behind.
	cmd.Env = gitenv.ReadOnly(os.Environ())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("git ").Str(strings.Join(args, " ")).Str(": ").Err(err).Str(": ").
			Str(strings.TrimSpace(stderr.String())).String())
	}
	return string(out), nil
}

// admittedRunner answers the Runner that runs each `go test` through job
// admission in tree, like every other le child, with its output captured.
func admittedRunner(tree string, environ []string) Runner {
	return func(argv []string) (bool, string, error) {
		admission, err := job.NewIn(tree)
		if err != nil {
			return false, "", err
		}
		// The two runs of one test differ in one flag, and a verdict borrowed
		// from another job would answer another question.
		admission.MayAttach = false
		var output bytes.Buffer
		admission.Out = &output
		report, code := admission.Run(jobLabel, argv, tree, environ)
		if report.Admission == job.KindUnspecified.String() {
			return false, output.String(), errors.New("job admission refused the run, so no test was " +
				"re-run; admission printed its reason above: fix it, then run the report again")
		}
		return code == 0, output.String(), nil
	}
}
