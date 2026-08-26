// Design: docs/architecture/core-design.md -- the fingerprint of the work a job does
//
// A label names the TARGET, and a parameterized target represents many jobs
// under one name. `make ze-unit-pkg-test PKG=./a` and the same target on `./b`
// pass the same label and command to the wrapper because PKG reaches only the
// `_ze-unit-pkg-test-impl` half. Two sessions that tested different packages
// therefore matched. The second session reported the first run's exit code as
// its own. On 2026-08-19, a package that was never compiled read as passing
// (plan/journal/stale-artifact-reused.md).
//
// The key fingerprints what the job DOES: the command and the make command-line
// variables that the caller typed. This set is DERIVED rather than declared.
// GNU make puts every `VAR=value` from the command line into MAKEFLAGS. If a
// target gets a parameter later, the key includes it without an edit here or
// in its recipe. A recipe cannot omit a declaration because no declaration is
// required.
//
// This boundary is explicit because this package must refuse a guard that fails
// open silently. A parameter supplied through the ENVIRONMENT
// (`PKG=./a make ze-unit-pkg-test`) reaches make as a variable but does not
// reach MAKEFLAGS. No rule, doc or recipe in this repository uses that form.
//
// It is indistinguishable from the ambient environment, which differs between
// sessions. Including it in the key would prevent all sharing. A recipe that
// needs such a value in the key passes it in the command that it gives to the
// wrapper. The key already reads that command.

package lejob

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// admissionKnobs contains the four names excluded from a key. All four control
// how a job is ADMITTED, not what it judges. The Makefile documents
// `make <target> ZE_RUN_SLOTS=1`, and ai/rules/git-safety.md documents raising
// the stall window. A session that follows either instruction must still share
// a running job instead of running a duplicate.
//
// MAY_ATTACH is excluded for another reason. It says that this job will not
// use another job's verdict. Including it in the key would also prevent other
// jobs from using THIS job's verdict.
var admissionKnobs = [...]string{
	"ZE_RUN_SLOTS=",
	"ZE_JOB_STALL_SECONDS=",
	"ZE_VERIFY_MAX_LOCK_AGE=",
	"MAY_ATTACH=",
}

// MakeParams returns the make command-line variable definitions that this
// process received, in sorted order.
//
// Make lists the definitions in the caller's order. However, sessions that type
// `PKG=./a RUN=X` and `RUN=X PKG=./a` request the same work, so the key uses the
// SET. Sorting uses byte order, which matches the order that `LC_ALL=C sort`
// gives to the shell half when it reads the same variable.
//
// MAKEFLAGS is make's own variable rather than a Ze variable. Therefore,
// MakeParams reads it directly from the process environment.
func MakeParams() []string {
	flags := os.Getenv("MAKEFLAGS")

	// The `--` separator is removed when make writes it. Its absence does NOT
	// mean "this job was given no parameters". GNU make emits the separator
	// only when the command line ALSO contains flags. Thus,
	// `make ze-unit-pkg-test PKG=./a` has the bare `PKG=./a` in MAKEFLAGS.
	//
	// A key that required the separator detected no parameters and used only the
	// command. This recreates the 2026-08-19 defect on a host whose make writes
	// no separator. GNU Make 3.81, which macOS ships, is such a host.
	if _, after, found := strings.Cut(flags, " -- "); found {
		flags = after
	}

	var params []string
	for _, word := range strings.FieldsFunc(flags, isBlank) {
		if !isDefinition(word) || isAdmissionKnob(word) {
			continue
		}
		params = append(params, word)
	}
	slices.Sort(params)
	return params
}

// isBlank reports whether a rune separates two words of MAKEFLAGS. Make writes
// spaces. It also accepts a tab because the splitter in the shell half accepts
// a tab.
func isBlank(r rune) bool {
	return r == ' ' || r == '\t'
}

// isDefinition reports whether a word of MAKEFLAGS is a variable definition. A
// valid name starts with a letter or an underscore, continues with letters,
// digits, underscores and dots, and ends at an `=`.
//
// The SHAPE alone determines the result, not a list of known names. `-j4` and
// `--jobserver-auth=fifo:/tmp/x` are excluded because neither changes a verdict.
// The jobserver address also differs on every invocation. Including it in the
// key would prevent all sharing.
func isDefinition(word string) bool {
	name, _, found := strings.Cut(word, "=")
	if !found || name == "" {
		return false
	}
	for i := range len(name) {
		c := name[i]
		switch {
		case c >= 'A' && c <= 'Z':
		case c >= 'a' && c <= 'z':
		case c == '_':
		case i > 0 && c >= '0' && c <= '9':
		case i > 0 && c == '.':
		default:
			return false
		}
	}
	return true
}

// isAdmissionKnob reports whether a definition is one of the four that choose
// how a job is admitted.
func isAdmissionKnob(word string) bool {
	for _, knob := range admissionKnobs {
		if strings.HasPrefix(word, knob) {
			return true
		}
	}
	return false
}

// JobKey returns the fingerprint of what this job DOES.
//
// The bytes contain the command on a CMD= line, followed by one parameter per
// line. A newline ends each line. This stream is the contract between this half
// and scripts/dev/ze-run.sh. A job admitted by either half attaches to a job
// that the other half runs only when both halves compute the same key.
func JobKey(argv, params []string) string {
	var tb textbuf.Buffer
	tb.Str("CMD=").Join(argv, " ").Byte('\n')
	for _, param := range params {
		tb.Str(param).Byte('\n')
	}

	sum := sha256.Sum256([]byte(tb.String()))
	return hex.EncodeToString(sum[:])
}
