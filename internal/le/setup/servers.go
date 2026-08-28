// Design: docs/architecture/core-design.md -- whether a language server ANSWERS
//
// These probes are the authoritative checks for language-server behavior.
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
// The former tools had the same problem. The measured transcript store contained
// 405 reads of complete development-tool and hook files when the question
// concerned one symbol (ai/rules/context-economy.md).

package setup

import (
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

// serverAnswer is what a server probe found, and a sentence for whoever must
// fix it.
type serverAnswer struct {
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

// runGoplsProbe asks the language server for the symbols of one small file.
//
// Split out from goplsHealth so a test can fake the answer instead of shelling
// out to a real server it cannot control.
func (s *Setup) runGoplsProbe() Result {
	return s.Shell.Run(Cmd{
		Argv:    []string{toolGopls, "symbols", goplsProbeFile},
		Dir:     s.Root,
		Timeout: goplsProbeTimeout,
	})
}

// goplsHealth answers whether gopls answers, and why not when it does not.
func (s *Setup) goplsHealth() serverAnswer {
	if !s.Shell.Present(toolGopls) {
		return serverAnswer{Health: HealthAbsent, Detail: goplsNotInstalled}
	}
	if !isFile(filepath.Join(s.Root, goplsProbeFile)) {
		var tb textbuf.Buffer
		return serverAnswer{Health: HealthNA, Detail: tb.Str("no ").Str(goplsProbeFile).Str(" to probe").String()}
	}

	result := s.goplsProbe()
	if !result.OK() {
		var tb textbuf.Buffer
		why := tb.Str(goplsNotAnswering).Str(": ").Str(result.complaint()).String()
		return serverAnswer{Health: HealthBroken, Detail: why}
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
		return serverAnswer{Health: HealthBroken, Detail: why}
	}

	var tb textbuf.Buffer
	detail := tb.Int(int64(symbols)).Str(" symbols from ").Str(goplsProbeFile).String()
	return serverAnswer{Health: HealthOK, Detail: detail}
}

// goplsProbe takes the seam when a test set one.
func (s *Setup) goplsProbe() Result {
	if s.Gopls != nil {
		return s.Gopls()
	}
	return s.runGoplsProbe()
}
