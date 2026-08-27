// Design: docs/architecture/core-design.md -- whether a language server ANSWERS
//
// servers.go is a Go port of scripts/le/devtools/servers.py.
//
// A binary on PATH does not prove that a language server works. The problem
// becomes visible only when a client makes a call. On one of the two dev
// machines, gopls was absent for weeks. The BLOCKING "load LSP first" rule was
// satisfied during every session because the query text disables that rule.
// No client made a call, and every call would have returned ENOENT.
//
// A presence check would not have detected the problem. Therefore, these checks
// run the server and require an answer.
//
// The Python tools had the same problem. The measured transcript store contained
// 405 reads of scripts/dev/*.py and .claude/hooks/*.py. The machine had no symbol
// server, so each read loaded a complete file when the question concerned a
// symbol (ai/rules/context-economy.md).

package devsetup

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Health says whether a language server answered, and if not, whose problem it
// is.
type Health string

const (
	// HealthOK means the server answered.
	HealthOK Health = "ok"
	// HealthAbsent means nothing on PATH. A DIFFERENT problem with a different
	// fix, and the tool table already installs it.
	HealthAbsent Health = "absent"
	// HealthBroken means that the server ran but returned no usable result. A
	// timeout, a non-zero exit, or a reply without an answer causes this state.
	// A broken module cache, a package that does not build, and a version mismatch
	// require corrections other than a server reinstall.
	HealthBroken Health = "broken"
	// HealthNA means there is nothing here to ask about, so the question does
	// not apply.
	HealthNA Health = "na"
)

// ServerAnswer is what a server probe found, and a sentence for whoever must
// fix it.
type ServerAnswer struct {
	Health Health
	Detail string
}

// goplsProbeFile is internal/core/clock/clock.go. It has 128 lines, and its
// package imports only time. This package has the smallest dependency footprint
// in the repository. gopls must load and type-check the package before it can
// answer. A package with a large import graph would test the graph instead of
// the server.
const goplsProbeFile = "internal/core/clock/clock.go"

// goplsProbeTimeout is approximately 35 times the measured warm duration of
// 3.4s on this machine. gopls type-checks the package before it answers. Thus,
// the first run with a cold Go build cache also processes the standard library.
// This timeout permits a cold-cache run but does not let a hung server stop the
// run indefinitely. A false failure causes someone to disable a check.
const goplsProbeTimeout = 120 * time.Second

// goplsSymbolLine matches one line of `gopls symbols` output: a name, a symbol
// kind, and a range.
var goplsSymbolLine = regexp.MustCompile(`\b[A-Z][A-Za-z]+\s+\d+:\d+-\d+:\d+`)

// The two sentences a gopls failure is reported with.
const goplsNotInstalled = "gopls is not installed; the gopls row above installs it"

const goplsNotAnswering = "gopls is present but not answering"

// GoplsProbe asks the language server for the symbols of one small file.
//
// Split out from GoplsHealth so a test can fake the answer instead of shelling
// out to a real server it cannot control.
func (s *Setup) GoplsProbe() Result {
	return s.Shell.Run(Cmd{
		Argv:    []string{toolGopls, "symbols", goplsProbeFile},
		Dir:     s.Root,
		Timeout: goplsProbeTimeout,
	})
}

// GoplsHealth answers whether gopls answers, and why not when it does not.
func (s *Setup) GoplsHealth() ServerAnswer {
	if !s.Shell.Present(toolGopls) {
		return ServerAnswer{Health: HealthAbsent, Detail: goplsNotInstalled}
	}
	if !isFile(filepath.Join(s.Root, goplsProbeFile)) {
		var tb textbuf.Buffer
		return ServerAnswer{Health: HealthNA, Detail: tb.Str("no ").Str(goplsProbeFile).Str(" to probe").String()}
	}

	result := s.goplsProbe()
	if !result.OK() {
		var tb textbuf.Buffer
		why := tb.Str(goplsNotAnswering).Str(": ").Str(result.Complaint()).String()
		return ServerAnswer{Health: HealthBroken, Detail: why}
	}

	symbols := 0
	for line := range strings.SplitSeq(result.Out, "\n") {
		if goplsSymbolLine.MatchString(line) {
			symbols++
		}
	}
	if symbols == 0 {
		var tb textbuf.Buffer
		why := tb.Str(goplsNotAnswering).Str(": no symbol in its reply for ").Str(goplsProbeFile).String()
		return ServerAnswer{Health: HealthBroken, Detail: why}
	}

	var tb textbuf.Buffer
	detail := tb.Int(int64(symbols)).Str(" symbols from ").Str(goplsProbeFile).String()
	return ServerAnswer{Health: HealthOK, Detail: detail}
}

// goplsProbe takes the seam when a test set one.
func (s *Setup) goplsProbe() Result {
	if s.Gopls != nil {
		return s.Gopls()
	}
	return s.GoplsProbe()
}

// pyrightProbeFile is the Python original of this module. GoplsHealth has an NA
// state for cases in which its probe file was renamed. A probe file inside the
// running program cannot cause that state. Therefore, PyrightHealth never
// returns NA. If this constant names another file, restore the NA branch.
const pyrightProbeFile = "scripts/le/devtools/servers.py"

// pyrightProbeTimeout is large for the same reason as goplsProbeTimeout. The
// measured warm duration is 0.5s on this machine. The first run after
// installation downloads a node runtime.
const pyrightProbeTimeout = 120 * time.Second

// The two sentences a pyright failure is reported with.
const pyrightNotInstalled = "pyright is not installed; the pyright row above installs it"

const pyrightNotAnswering = "pyright is present but not answering"

// PyrightProbe asks the language server to analyze one file and report what it
// did.
func (s *Setup) PyrightProbe() Result {
	return s.Shell.Run(Cmd{
		Argv:    []string{toolPyright, "--outputjson", pyrightProbeFile},
		Dir:     s.Root,
		Timeout: pyrightProbeTimeout,
	})
}

// PyrightSummary returns the analysis summary and reports whether it found one.
// A bootstrap preamble sometimes precedes the reply.
//
// Decoding the complete reply failed on exactly one run: the first run. If no
// global node is on PATH, the pipx wrapper installs one. _install_node_env
// (pyright/node.py) runs nodeenv through subprocess.run(args, check=True)
// without redirection. Therefore, this probe captures the nodeenv progress on
// stdout. Only the npm path is silent (install_pyright, pyright/_utils.py).
//
// Valid JSON follows the progress text. Decoding the complete output fails, and
// setup fails on the fresh Linux box that it must prepare. The second run
// passes, so the failure appears intermittent.
//
// The preamble is not valid JSON. nodeenv prints a Python dict repr with single
// quotes. Therefore, the function scans for the first object that contains a
// summary. This method identifies the reply and does not guess its start.
func PyrightSummary(out string) (map[string]any, bool) {
	for start := strings.IndexByte(out, '{'); start != -1; {
		decoder := json.NewDecoder(strings.NewReader(out[start:]))
		var value map[string]any
		if err := decoder.Decode(&value); err == nil {
			if summary, ok := value["summary"].(map[string]any); ok {
				return summary, true
			}
		}
		next := strings.IndexByte(out[start+1:], '{')
		if next == -1 {
			return nil, false
		}
		start += next + 1
	}
	return nil, false
}

// PyrightHealth reports whether pyright answers. It also gives the reason when
// pyright does not answer.
//
// One difference from gopls determines the remaining behavior. pyright exits 1
// when it finds a type error. Therefore, the exit code reports whether the CODE
// is clean, not whether the SERVER worked. An exit-code check fails when a
// script first receives a diagnostic. A false failure causes someone to disable
// a check. The filesAnalyzed field in the summary answers the required question.
func (s *Setup) PyrightHealth() ServerAnswer {
	if !s.Shell.Present(toolPyright) {
		return ServerAnswer{Health: HealthAbsent, Detail: pyrightNotInstalled}
	}

	result := s.pyrightProbe()
	analyzed, ok := analyzedFiles(result.Out)
	if !ok {
		why := ""
		if strings.TrimSpace(result.Err) != "" {
			why = result.Complaint()
		}
		if why == "" {
			var tb textbuf.Buffer
			why = tb.Str("no summary in its reply for ").Str(pyrightProbeFile).String()
		}
		var tb textbuf.Buffer
		return ServerAnswer{Health: HealthBroken, Detail: tb.Str(pyrightNotAnswering).Str(": ").Str(why).String()}
	}

	if analyzed < 1 {
		var tb textbuf.Buffer
		// The two wordings below are the SCRIPT's. Keep them unchanged so that
		// the pages compare byte for byte until step 14 deletes the script. US
		// English governs text that this repository AUTHORS. A migration fixture
		// is not authored text, and step 14 changes both spellings.
		why := tb.Str(pyrightNotAnswering).Str(": analysed no file for ").Str(pyrightProbeFile).String() //nolint:misspell // the script's wording, held until the swap
		return ServerAnswer{Health: HealthBroken, Detail: why}
	}

	var tb textbuf.Buffer
	detail := tb.Int(int64(analyzed)).Str(" file analysed from ").Str(pyrightProbeFile).String() //nolint:misspell // the script's wording, held until the swap
	return ServerAnswer{Health: HealthOK, Detail: detail}
}

// pyrightProbe takes the seam when a test set one.
func (s *Setup) pyrightProbe() Result {
	if s.Pyright != nil {
		return s.Pyright()
	}
	return s.PyrightProbe()
}

// analyzedFiles answers how many files the reply says were analyzed, and
// whether the reply said so at all. A summary whose field is missing or is not
// a number is no answer, which is the state the caller reports as BROKEN.
func analyzedFiles(out string) (int, bool) {
	summary, ok := PyrightSummary(out)
	if !ok {
		return 0, false
	}
	count, ok := summary["filesAnalyzed"].(float64)
	if !ok {
		return 0, false
	}
	return int(count), true
}
