// Design: docs/architecture/testing/verify-freshness-scope.md -- the suite map this reduction feeds
// Detail: run.go -- the gating run that reduces each suite as it finishes
// Overview: suitemap.go -- the artifact these package sets are published to
//
// reach.go answers one question about a suite that has finished running: which
// Go packages did that suite REACH?
//
// A package is REACHED when the suite covered one block that is neither in
// register.go nor inside a func init() body. Every other covered block is what
// ANY process runs on ANY start, because Ze registers its components by running
// each package's init() (ai/patterns/registration.md), so counting those blocks
// answers "which packages does this binary LINK" instead.
//
// The difference is what makes the map worth having. Phase 1 of
// plan/spec-verify-scope-5-suite-coverage-map.md counted every covered block
// and measured a three-suite intersection of 443 packages of 646, with `ze show
// version` alone counting 435. Phase 1b counted reaches over the same profiles
// and measured 126, with `ze show version` at 115.
//
// Every uncertainty here WIDENS: a file that will not parse contributes its
// blocks, so a package is called reached rather than quietly dropped, and the
// suite that reached it runs more often rather than less.

package functional

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
)

// coverageReduceTimeout bounds the go tool invocation for one suite. The
// largest profile phase 1b reduced was 13.7 MB over 144 counter files, which
// covdata wrote in under a second. A run past this bound is wedged.
const coverageReduceTimeout = 5 * time.Minute

// modulePath is this module, and modulePrefix is what the import path of a file
// inside it opens with. A covered file outside the module cannot appear in a
// change-set answer, so it names no package the map can be asked about.
const (
	modulePath   = "github.com/ze-software/ze"
	modulePrefix = modulePath + "/"
)

// registrationFile is the file Ze declares a package's registration in. Its
// blocks run on any process start, so they say nothing about what a suite did.
const registrationFile = "register.go"

// lineRange is the inclusive first and last line of one func init() body.
type lineRange struct {
	first int
	last  int
}

// reachedPackages answers the packages one finished suite reached, from the raw
// coverage directory that suite wrote.
//
// The answer is empty, with no error, for a suite that recorded nothing. That
// is a real state and four suites are always in it: editor, web, runner and
// policy run the harness or skip unprivileged, so no instrumented ze runs at
// all. The caller omits such a suite from the map rather than publishing an
// empty set for it (publish, suitemap.go).
func reachedPackages(tc gotoolchain.Toolchain, cover string) ([]string, error) {
	profile, err := textfmtProfile(tc, cover)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := os.Remove(profile); err != nil {
			gaterun.Note(reportLine(err))
		}
	}()
	return packagesInProfile(tc.Root, profile)
}

// textfmtProfile reduces a raw coverage directory to one text profile and
// answers where it wrote it.
//
// The profile is a temporary beside the coverage root, and the caller removes
// it once it has been read. One suite's profile therefore exists at a time,
// which bounds what an instrumented gating run costs on disk: phase 1b measured
// 13.7 MB for the largest of the three suites it reduced.
func textfmtProfile(tc gotoolchain.Toolchain, cover string) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(cover), ".textfmt-*")
	if err != nil {
		return "", err
	}
	profile := file.Name()
	if err := file.Close(); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), coverageReduceTimeout)
	defer cancel()
	var input, output textbuf.Buffer
	//nolint:gosec // the go tool, over two paths this run created
	command := exec.CommandContext(ctx, "go", "tool", "covdata", "textfmt",
		input.Str("-i=").Str(cover).String(), output.Str("-o=").Str(profile).String())
	command.Dir = tc.Root
	command.Env = tc.Environment(gotoolchain.EnvOptions{})
	if out, err := command.CombinedOutput(); err != nil {
		_ = os.Remove(profile)
		return "", fmt.Errorf("go tool covdata textfmt over %s: %w: %s", cover, err, strings.TrimSpace(string(out)))
	}
	return profile, nil
}

// packagesInProfile reads one text coverage profile and answers the packages it
// says were reached, spelled the way the change-set selector spells one
// ("./internal/component/ssh"), sorted.
//
// A row of the profile is "<import-path>/<file>:<start>.<col>,<end>.<col> <statements> <count>".
// A count of zero is a block the suite never ran.
func packagesInProfile(root, profile string) ([]string, error) {
	handle, err := os.Open(profile) //nolint:gosec // a temporary this package wrote
	if err != nil {
		return nil, err
	}
	defer handle.Close() //nolint:errcheck // nothing is written to it

	initRanges := map[string][]lineRange{}
	reached := map[string]bool{}

	// A profile row is one file path plus two short numbers, and the longest
	// import path in this module is under 200 bytes. The bound is generous
	// rather than tight so a rename never turns a profile into a parse error.
	scanner := bufio.NewScanner(handle)
	scanner.Buffer(make([]byte, 0, 4096), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		importFile, block, count, err := profileRow(line)
		if err != nil {
			return nil, err
		}
		if count == 0 {
			continue
		}
		name, inside := selectorSpelling(path.Dir(importFile))
		if !inside {
			continue
		}
		if reached[name] {
			continue
		}
		if path.Base(importFile) == registrationFile {
			continue
		}
		ranges, known := initRanges[importFile]
		if !known {
			ranges = initLineRanges(root, importFile)
			initRanges[importFile] = ranges
		}
		if block.within(ranges) {
			continue
		}
		reached[name] = true
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read the coverage profile %s: %w", profile, err)
	}

	names := make([]string, 0, len(reached))
	for name := range reached {
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}

// profileRow splits one profile row into the file it covers, the lines the
// block spans, and how many times the suite ran it.
//
// A row this cannot read is an error rather than a skipped line, because the
// profile is written by the go tool and a shape change here would otherwise
// shrink every recorded set in silence.
func profileRow(line string) (importFile string, block lineRange, count int, err error) {
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return "", lineRange{}, 0, fmt.Errorf("a coverage profile row has %d fields, want 3: %q", len(fields), line)
	}
	count, err = strconv.Atoi(fields[2])
	if err != nil {
		return "", lineRange{}, 0, fmt.Errorf("a coverage profile row carries no count: %q", line)
	}

	colon := strings.LastIndex(fields[0], ":")
	if colon < 0 {
		return "", lineRange{}, 0, fmt.Errorf("a coverage profile row names no block: %q", line)
	}
	importFile = fields[0][:colon]
	first, last, found := strings.Cut(fields[0][colon+1:], ",")
	if !found {
		return "", lineRange{}, 0, fmt.Errorf("a coverage profile block has no end: %q", line)
	}
	block.first, err = lineNumber(first)
	if err != nil {
		return "", lineRange{}, 0, fmt.Errorf("%w: %q", err, line)
	}
	block.last, err = lineNumber(last)
	if err != nil {
		return "", lineRange{}, 0, fmt.Errorf("%w: %q", err, line)
	}
	return importFile, block, count, nil
}

// lineNumber reads the line out of one "<line>.<column>" position.
func lineNumber(position string) (int, error) {
	line, _, _ := strings.Cut(position, ".")
	number, err := strconv.Atoi(line)
	if err != nil {
		return 0, errors.New("a coverage profile position names no line")
	}
	return number, nil
}

// within reports whether this block sits inside one of the ranges.
func (r lineRange) within(ranges []lineRange) bool {
	for _, one := range ranges {
		if r.first >= one.first && r.last <= one.last {
			return true
		}
	}
	return false
}

// initLineRanges answers the line range of every func init() body one file
// declares. A file that declares several init functions yields several ranges.
//
// A file that will not parse yields none, so its blocks all count and the
// package reads as reached. That is the widening direction: the suite then runs
// on a change to that package, which is what an unselected run does today.
func initLineRanges(root, importFile string) []lineRange {
	relative, inside := strings.CutPrefix(importFile, modulePrefix)
	if !inside {
		return nil
	}
	source := filepath.Join(root, filepath.FromSlash(relative))
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, source, nil, parser.SkipObjectResolution)
	if err != nil {
		var tb textbuf.Buffer
		gaterun.Note(tb.Str("coverage: ").Str(source).
			Str(" would not parse, so its package counts as reached: ").Err(err).String())
		return nil
	}

	var ranges []lineRange
	for _, declared := range file.Decls {
		function, isFunction := declared.(*ast.FuncDecl)
		if !isFunction {
			continue
		}
		if function.Recv != nil {
			continue
		}
		if function.Name.Name != "init" {
			continue
		}
		ranges = append(ranges, lineRange{
			first: fset.Position(function.Pos()).Line,
			last:  fset.Position(function.End()).Line,
		})
	}
	return ranges
}

// selectorSpelling turns one import path into the spelling the change-set
// selector answers with, and says when the path names no package of this
// module.
//
// The two sides must agree letter for letter, because phase 4 compares a
// recorded set with `./le changed packages` without either side normalizing the
// other.
func selectorSpelling(importPath string) (name string, inside bool) {
	if importPath == modulePath {
		return ".", true
	}
	relative, within := strings.CutPrefix(importPath, modulePrefix)
	if !within {
		return "", false
	}
	var tb textbuf.Buffer
	return tb.Str("./").Str(relative).String(), true
}
