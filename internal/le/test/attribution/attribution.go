// Design: docs/contributing/running-commands.md -- le test attribution report
// Related: observe.go -- what one `go test` run of one test answered
// Related: report.go -- the answer and its text rendering
// Related: actions.go -- the command surface and the real runner
// Related: register.go -- the command registration
//
// Package testattribution answers, for each test a package run reported red,
// whether the session's own files cause the red. Several sessions edit one
// checkout, so a package run judges every session's edits at once; this
// re-runs each failing test twice, once as the tree is on disk and once with
// the files the caller names put back to their HEAD content through a Go
// overlay, and compares the two. The tree on disk is never touched.
//
// The named files are the caller's to give, as `./le commit create` takes
// them: the changed-set selector cannot tell one session's edits from
// another's.
package testattribution

import (
	"errors"
	"path"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	testoverlay "github.com/ze-software/ze/internal/le/test/overlay"
)

// Runner runs one `go test` argv and answers whether it passed, with its
// combined output. An error says the run did not happen, which is never a red.
type Runner func(argv []string) (passed bool, output string, err error)

// HeadReader answers the overlay entry that puts one tree-relative file back
// to its HEAD content: the content when HEAD holds the file, Absent when it
// does not.
type HeadReader func(rel string) (testoverlay.File, error)

// OverlayWriter writes the overlay that replaces files and answers its path.
type OverlayWriter func(files []testoverlay.File) (string, error)

// Request is one attribution: the failing tests of one package run, and the
// files the caller says are the session's own.
type Request struct {
	// Log is the run log the failing tests were read from, as the caller named it.
	Log string
	// Failed are the top-level tests the log reports failed, each once.
	Failed []string
	// Package is the slash-separated package directory, relative to the tree.
	Package string
	// Tags are the build tags of the run, as the caller gave them.
	Tags string
	// Files are the slash-separated tree-relative paths of the session's files.
	Files []string
}

// Attributor holds what one attribution needs from the outside world, so a
// test can hand it fakes and the command hands it git, the overlay writer and
// job admission.
type Attributor struct {
	Run   Runner
	Head  HeadReader
	Write OverlayWriter
	// Timeout is the `go test -timeout` value each re-run carries.
	Timeout string
}

// Attribute re-runs each failing test as on disk and with the named files at
// HEAD, and classifies each one. Passed tests are not re-run: the question is
// about the reds only.
//
// An error says the comparison could not be made at all (no failing test, an
// unreadable file at HEAD, a run that never started), so no row is answered.
func (a Attributor) Attribute(request Request) (Report, error) {
	if len(request.Failed) == 0 {
		var tb textbuf.Buffer
		return Report{}, errors.New(tb.Str("the log ").Str(request.Log).
			Str(" names no failing test (no `--- FAIL:` line), so there is nothing to attribute; ").
			Str("a package that did not compile has no such line: read the log's first error instead").String())
	}
	if len(request.Files) == 0 {
		return Report{}, errors.New("attribution compares the named files at HEAD with the same files on disk, " +
			"and no file was named: pass each of the session's files with file <path>")
	}

	files := make([]testoverlay.File, 0, len(request.Files))
	for _, rel := range request.Files {
		file, err := a.Head(rel)
		if err != nil {
			return Report{}, err
		}
		files = append(files, file)
	}
	overlay, err := a.Write(files)
	if err != nil {
		return Report{}, err
	}

	report := Report{Log: request.Log, Package: request.Package, Tags: request.Tags,
		Files: request.Files, Compared: compared(request.Package, request.Files)}
	for _, name := range request.Failed {
		disk, err := a.observe(request, name, "")
		if err != nil {
			return Report{}, err
		}
		head, err := a.observe(request, name, overlay)
		if err != nil {
			return Report{}, err
		}
		class, detail := classify(disk, head)
		report.Rows = append(report.Rows, Row{Test: name, Class: class, Detail: detail,
			OnDisk: disk.outcome.String(), AtHead: head.outcome.String()})
	}
	return report, nil
}

// observe runs one test once, under the overlay when one is given.
func (a Attributor) observe(request Request, name, overlay string) (observation, error) {
	passed, output, err := a.Run(unitArgv(a.Timeout, request.Tags, request.Package, name, overlay))
	if err != nil {
		var tb textbuf.Buffer
		side := "as on disk"
		if overlay != "" {
			side = "with the named files at HEAD"
		}
		return observation{}, errors.New(tb.Str("the re-run of ").Str(name).Byte(' ').Str(side).
			Str(" did not run, so nothing is attributed: ").Err(err).String())
	}
	return observe(passed, output, name), nil
}

// unitArgv answers the `go test` command that runs one top-level test of one
// package with the caller's tags, under the overlay when one is given.
//
// The caller's tags rather than the feature manifest's, because the question
// is about the run the log came from. -v prints `=== RUN   <Name>`, which is
// what tells a test that ran from one the package does not hold.
func unitArgv(timeout, tags, pkg, name, overlay string) []string {
	var tb textbuf.Buffer
	argv := []string{"go", "test", "-timeout", timeout, "-tags", tags,
		"-run", tb.Byte('^').Str(name).Byte('$').String(), "-count=1", "-v"}
	if overlay != "" {
		argv = append(argv, "-overlay", overlay)
	}
	return append(argv, tb.Reset().Str("./").Str(pkg).String())
}

// compared states what the two runs differed in, which is all a verdict can
// claim (R-5 of the spec): a red the named files cause only together with
// another session's edit still reads as not caused.
func compared(pkg string, files []string) string {
	var tb textbuf.Buffer
	tb.Str("each failing test of ").Str(pkg).Str(" ran twice: once as on disk, once with ").
		Str(strings.Join(files, ", ")).
		Str(" at HEAD and every other file as on disk. The overlay changes only what the compiler reads, ").
		Str("so a test that opens a named file at run time reads the copy on disk, and a red the named ").
		Str("files cause only together with another session's edit reads as not-caused")
	return tb.String()
}

// CleanPath answers a caller's tree-relative path in slash form with no
// leading `./` or trailing slash, or an error for a path outside the tree.
func CleanPath(rel string) (string, error) {
	cleaned := path.Clean(strings.ReplaceAll(rel, "\\", "/"))
	if path.IsAbs(cleaned) {
		return "", errors.New(rel + ": name the path relative to the checkout root, not as an absolute path")
	}
	if cleaned == ".." {
		return "", errors.New(rel + ": the path leaves the checkout; name a path inside it")
	}
	if strings.HasPrefix(cleaned, "../") {
		return "", errors.New(rel + ": the path leaves the checkout; name a path inside it")
	}
	if cleaned == "." {
		return "", errors.New(rel + ": the path names the checkout root; name one file or package directory")
	}
	return cleaned, nil
}
