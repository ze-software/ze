// Design: docs/architecture/core-design.md -- one declaration of the Go version, and the carriers that copy it
// Detail: dockerfile.go -- how a build stage is read out of a Dockerfile
// Detail: report.go -- what `le go-version check` answers
//
// Package goversion pins every build carrier in this repository to the Go minor
// version the `go` directive of go.mod declares.
//
// The version is DECLARED once, in go.mod, and COPIED into each carrier that
// builds this module: the `FROM golang:<version>` line of a Dockerfile, and the
// container image a Go tool names for a build it runs. Nothing compared the two
// until this gate, and the drift has cost the tree twice
// (plan/journal/stale-artifact-reused.md, the 2026-08-27 and 2026-09-05 rows).
// A carrier a minor behind either fails at its `go mod download` layer or
// downloads a toolchain nobody chose.
//
// WHICH CARRIERS ARE JUDGED is DERIVED from what each one does, never from a
// list of names here, because a list is the second declaration this gate exists
// to remove:
//
//   - A Dockerfile stage is judged when it copies THIS MODULE in from the build
//     context, which is a `COPY go.mod ...` or a `COPY . .`. Only such a stage
//     compiles Ze, so only such a stage owes Ze's Go version.
//   - A Go string literal naming a `golang:` image is judged, because a Go tool
//     that names one runs a build of this module inside it (DefaultImage,
//     internal/le/evidence/evidence.go).
//
// A stage that builds OTHER sources is excluded by the same derivation, and it
// is REPORTED rather than dropped: a `go install` of a remote module
// (Dockerfile.gobgp, Dockerfile.stayrtr) and a copy of one standalone file (the
// fixture-builder stage of Dockerfile.freertr) each carry their own Go minimum
// and copy no module in. There is no allowlist to fall out of, and no marker to
// write. A carrier this derivation cannot excuse states its case in the file it
// lives in, where the reader who can judge it is already looking.
//
// The gate FAILS CLOSED. A judged stage whose image tag names no
// `<major>.<minor>` is a finding rather than a skip, and so is one whose base is
// not a golang image at all: both build this module with a Go version the gate
// cannot read, which is the same drift wearing another face. A run that judged
// no carrier is an error rather than a pass.
//
// Two trees are outside the walk, each for what the directory IS. vendor/ holds
// third-party source this module does not build, and testdata/ holds the
// fixtures this gate's own tests are written against.
package goversion

import (
	"context"
	"errors"
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const (
	// goModFile is the one declaration every carrier copies.
	goModFile = "go.mod"
	// moduleImage is the image name a carrier of this module builds on.
	moduleImage = "golang"
	// imagePrefix is the cheap test that keeps the Go walk off every string
	// literal in the repository. minorOf is what decides the rest.
	imagePrefix = moduleImage + ":"
	// wholeContext is the COPY source that brings the whole build context, and
	// with it this module, into a stage.
	wholeContext = "."
	// subprocessDeadline bounds the one git call. A hung `git ls-files` would
	// otherwise hang a verify run with no diagnostic.
	subprocessDeadline = 120 * time.Second
)

// excludedDirectories are the path components the walk does not enter, each for
// what the directory IS rather than for what it happens to hold today. vendor/
// is third-party source this module does not build. testdata/ is the fixture
// tree this package's own tests judge, and judging it here would make a fixture
// that must drift into a finding about the checkout.
var excludedDirectories = map[string]bool{"vendor": true, "testdata": true}

var (
	// goDirectiveRe reads the `go` directive of a go.mod. The anchor at column
	// zero is what separates the directive from a `require` line, which git
	// holds indented, and from the `toolchain` line, which starts with another
	// word.
	goDirectiveRe = regexp.MustCompile(`(?m)^go[ \t]+(\d+)\.(\d+)(?:\.\d+)?[ \t]*$`)
	// imageVersionRe reads the version part of an image tag, which is the tag up
	// to its first hyphen: 1.27, 1.27.1, and the 1.27 of 1.27-alpine.
	imageVersionRe = regexp.MustCompile(`^(\d+)\.(\d+)(?:\.\d+)?$`)
)

// Declared answers the Go minor version go.mod declares, as `<major>.<minor>`.
//
// Two go directives, and none at all, are both errors rather than a guess. This
// value is the whole standard every carrier is judged against, so a read that
// did not reach it must not answer an empty string a comparison would then
// treat as a version nothing matches.
func Declared(root string) (string, error) {
	body, err := readFileString(filepath.Join(root, goModFile))
	if err != nil {
		return "", fmt.Errorf("read %s: %w", goModFile, err)
	}
	return declaredMinor(body)
}

// declaredMinor reads the minor version out of a go.mod text.
func declaredMinor(body string) (string, error) {
	matches := goDirectiveRe.FindAllStringSubmatch(body, -1)
	if len(matches) != 1 {
		return "", fmt.Errorf("%s declares %d `go <major>.<minor>` directives, want 1", goModFile, len(matches))
	}
	return matches[0][1] + "." + matches[0][2], nil
}

// minorOf answers the Go minor version an image reference names.
//
// isGolang says the reference names the golang image, which is what makes it a
// carrier of this module's Go version at all. readable says its tag carries a
// `<major>.<minor>`: `golang:latest`, `golang:alpine` and `golang:${GO_VERSION}`
// each build with a Go nobody in this tree declared, so the caller reports them
// rather than reading a zero as agreement.
func minorOf(reference string) (minor string, isGolang, readable bool) {
	// A digest pin sits after the tag and says nothing about the version.
	image, _, _ := strings.Cut(reference, "@")

	name := image
	tag := ""
	if colon := strings.LastIndex(image, ":"); colon > strings.LastIndex(image, "/") {
		name, tag = image[:colon], image[colon+1:]
	}
	if path.Base(name) != moduleImage {
		return "", false, false
	}

	// The tag is <version> or <version>-<variant>, so the version ends at the
	// first hyphen: 1.27-alpine and 1.27-bookworm are both the 1.27 minor.
	version, _, _ := strings.Cut(tag, "-")
	parts := imageVersionRe.FindStringSubmatch(version)
	if parts == nil {
		return "", true, false
	}
	return parts[1] + "." + parts[2], true, true
}

// Check judges every build carrier under root against the declared minor.
//
// files is the tracked population, as repository-relative paths. runCheck reads
// it from git's index and a test hands it a walk of a fixture tree, so the
// judgement itself has one implementation and both callers reach it.
//
// The error is about the READ rather than about the tree: a carrier that could
// not be read, or a walk that judged nothing. A carrier that disagrees is a
// FINDING, because a disagreement is what this gate exists to answer.
func Check(root string, files []string, declared string) (Result, error) {
	if declared == "" {
		return Result{}, errors.New("no declared Go minor version to judge against")
	}

	result := Result{Declared: declared}
	// The walk is a copy, because ordering the answer must not reorder the
	// caller's own population.
	walk := append([]string(nil), files...)
	sort.Strings(walk)

	for _, rel := range walk {
		if !walked(rel) {
			continue
		}
		body, err := readFileString(filepath.Join(root, rel))
		if errors.Is(err, os.ErrNotExist) {
			// git holds a file the working tree has already deleted. There is
			// no content left to judge, and the commit that stages the deletion
			// removes the entry.
			continue
		}
		if err != nil {
			return Result{}, fmt.Errorf("read %s: %w", rel, err)
		}
		if isDockerfile(rel) {
			result.judgeDockerfile(rel, body)
			continue
		}
		result.judgeGoSource(rel, body)
	}

	if result.Carriers == 0 {
		return Result{}, errors.New("the walk judged no build carrier, so a clean answer would be a walk that read nothing")
	}
	result.Valid = len(result.Findings) == 0
	return result, nil
}

// walked reports whether a tracked path is in this gate's walk at all: a
// container build file or Go source, outside the two excluded trees.
func walked(rel string) bool {
	for part := range strings.SplitSeq(filepath.ToSlash(rel), "/") {
		if excludedDirectories[part] {
			return false
		}
	}
	if isDockerfile(rel) {
		return true
	}
	return strings.HasSuffix(rel, ".go")
}

// isDockerfile reports whether a path names a container build file. The suffix
// carries the variant (`Dockerfile.ze`, `Dockerfile.lab`), so the prefix is what
// identifies the kind.
func isDockerfile(rel string) bool {
	return strings.HasPrefix(path.Base(filepath.ToSlash(rel)), "Dockerfile")
}

// judgeDockerfile reads every stage of one Dockerfile and records what each one
// is: a judged carrier, an excluded golang stage, or neither.
func (r *Result) judgeDockerfile(rel, body string) {
	for _, current := range stagesOf(body) {
		minor, isGolang, readable := minorOf(current.Base)
		if !current.Copies {
			if isGolang {
				r.Excluded = append(r.Excluded, Excluded{
					Carrier: rel, Line: current.Line, Names: current.Base, Reason: ExcludedNoModuleCopy,
				})
			}
			continue
		}

		r.Carriers++
		switch {
		case !isGolang:
			r.Findings = append(r.Findings, Finding{
				Carrier: rel, Line: current.Line, Names: current.Base,
				Declared: r.Declared, Reason: ReasonUnreadableBase,
			})
		case !readable:
			r.Findings = append(r.Findings, Finding{
				Carrier: rel, Line: current.Line, Names: current.Base,
				Declared: r.Declared, Reason: ReasonUnreadableTag,
			})
		case minor != r.Declared:
			r.Findings = append(r.Findings, Finding{
				Carrier: rel, Line: current.Line, Names: current.Base,
				Declared: r.Declared, Reason: ReasonMismatch,
			})
		}
	}
}

// judgeGoSource reads every golang image literal in one Go file. A Go tool that
// names such an image runs a build of this module inside it, so the literal is
// a carrier of the same declaration a Dockerfile stage carries.
func (r *Result) judgeGoSource(rel, body string) {
	for _, current := range imageLiterals(rel, body) {
		minor, isGolang, readable := minorOf(current.Value)
		if !isGolang {
			continue
		}

		r.Carriers++
		if !readable {
			r.Findings = append(r.Findings, Finding{
				Carrier: rel, Line: current.Line, Names: current.Value,
				Declared: r.Declared, Reason: ReasonUnreadableTag,
			})
			continue
		}
		if minor != r.Declared {
			r.Findings = append(r.Findings, Finding{
				Carrier: rel, Line: current.Line, Names: current.Value,
				Declared: r.Declared, Reason: ReasonMismatch,
			})
		}
	}
}

// literal is one Go string literal that names an image, with the line it sits on.
type literal struct {
	Value string
	Line  int
}

// imageLiterals answers every STRING token of a Go file whose value names an
// image, in file order.
//
// The reader is go/scanner rather than a regular expression, and rather than
// go/parser, for one reason each. A regular expression cannot tell a string
// literal from a sentence in a comment, and this repository's comments quote an
// image reference often. go/parser needs a whole well-formed file, and several
// sessions edit this checkout at once, so a peer's half-written source would
// turn the gate red for something that is not a carrier at all. The scanner
// takes a nil error handler, recovers, and keeps answering the tokens it can
// read, which is what a gate over somebody else's work in progress owes.
func imageLiterals(rel, body string) []literal {
	set := token.NewFileSet()
	file := set.AddFile(rel, set.Base(), len(body))

	var read scanner.Scanner
	// Mode 0 drops comments, which is the whole reason this is a scanner.
	read.Init(file, []byte(body), nil, 0)

	var out []literal
	for {
		position, kind, text := read.Scan()
		if kind == token.EOF {
			break
		}
		if kind != token.STRING {
			continue
		}
		value, err := strconv.Unquote(text)
		if err != nil {
			// A literal the scanner recovered from mid-edit. It names no
			// readable image, so there is nothing here to judge.
			continue
		}
		if !strings.Contains(value, imagePrefix) {
			continue
		}
		out = append(out, literal{Value: value, Line: set.Position(position).Line})
	}
	return out
}

// readFileString answers one file's whole text. The parameter is named `file`
// rather than `path`, which this package imports.
func readFileString(file string) (string, error) {
	body, err := os.ReadFile(file) //nolint:gosec // paths inside the checkout lepath answers
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// trackedFiles answers the repository-relative paths git holds in its index.
//
// It FAILS CLOSED twice, the same two ways every population read in this
// repository does. A git that cannot run is a refusal rather than an empty set,
// and an index that lists nothing is a broken query rather than an empty
// repository: this gate is a filter over that set, so an empty answer judges
// nothing and prints what a clean tree prints.
func trackedFiles(root string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), subprocessDeadline)
	defer cancel()

	command := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "-z") // #nosec G204 -- the checkout path lepath answered
	out, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files in %s: %w", root, err)
	}

	var files []string
	for name := range strings.SplitSeq(string(out), "\x00") {
		if name != "" {
			files = append(files, name)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("git ls-files listed nothing in %s", root)
	}
	return files, nil
}

// runCheck is the `check` action: read the checkout and answer what drifted.
func runCheck() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	declared, err := Declared(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	files, err := trackedFiles(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	result, err := Check(tree, files, declared)
	if err != nil {
		// 2 rather than 1: a read that did not complete is a different fact
		// from a carrier that drifted, and a caller reads the two apart.
		leaction.ReportError(err)
		return nil, 2
	}
	if !result.Valid {
		return result, 1
	}
	return result, 0
}
