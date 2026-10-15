// Design: docs/guide/developer-setup.md -- the Go bin directory must be on PATH
// Related: setup.go -- visitTool reports a Go tool that landed off PATH
//
// `go install` writes to GOBIN when it is set, and otherwise to the bin
// directory under the first GOPATH entry. A fresh host has neither on PATH: the
// go.dev tarball asks the developer to add /usr/local/go/bin, and nothing adds
// the install directory. A tool built there is on the disk and out of reach of
// every shell, so the report names the real directory and the exact line that
// puts it on PATH. It never answers "~/go/bin" by assumption, because GOBIN and
// GOPATH can both point elsewhere.

package setup

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// goEnvTimeout bounds `go env`, which reads its configuration and exits.
const goEnvTimeout = 30 * time.Second

// goEnvSubcommand is the go subcommand that prints its configuration.
const goEnvSubcommand = "env"

// goBinDir answers the directory `go install` writes to, and whether go could
// say. GOBIN wins when it is set; otherwise it is the bin directory under the
// first GOPATH entry, which is the rule `go help gopath` states.
func (s *Setup) goBinDir() (string, bool) {
	result := s.Shell.Run(Cmd{
		Argv:    []string{toolGo, goEnvSubcommand, "GOBIN", "GOPATH"},
		Dir:     s.Root,
		Timeout: goEnvTimeout,
	})
	if !result.OK() {
		return "", false
	}
	// `go env A B` prints one value per line, an empty line for an unset A.
	lines := strings.Split(strings.TrimRight(result.Out, "\n"), "\n")
	if len(lines) != 2 {
		return "", false
	}
	if gobin := strings.TrimSpace(lines[0]); gobin != "" {
		return gobin, true
	}
	gopath := filepath.SplitList(strings.TrimSpace(lines[1]))
	if len(gopath) == 0 {
		return "", false
	}
	if gopath[0] == "" {
		return "", false
	}
	return filepath.Join(gopath[0], "bin"), true
}

// onPath reports whether dir is one of the PATH entries this run sees.
func (s *Setup) onPath(dir string) bool {
	want := filepath.Clean(dir)
	for _, entry := range filepath.SplitList(s.env("PATH")) {
		if entry == "" {
			continue
		}
		if filepath.Clean(entry) == want {
			return true
		}
	}
	return false
}

// shellProfile names the startup file that gives every new shell the PATH
// line. zsh reads ~/.zshenv in every shell, interactive or not. Every other
// shell gets ~/.profile, which the Go install page names and a login shell
// reads; an interactive bash ~/.bashrc returns before a line appended to it
// when the shell is not interactive, so `ssh host command` would miss it.
func (s *Setup) shellProfile() string {
	if filepath.Base(s.env("SHELL")) == "zsh" {
		return "~/.zshenv"
	}
	return "~/.profile"
}

// goToolOffPath answers the outcome for a Go tool that sits in the Go install
// directory while that directory is not on PATH, and whether that is the case.
// Any other reason the probe failed answers false, and the caller reports it
// as before.
//
// The outcome is PENDING in both modes: the tool is built, and the one step left
// is the PATH line, which only the developer can put in a shell profile. It is
// never PRESENT, because no shell the developer opens can run it yet.
//
// The PATH line is printed once per run, before the first such row, so five
// tools in one directory read as one fix.
func (s *Setup) goToolOffPath(report *Report, tool Tool) (Outcome, bool) {
	if tool.GoInstall == "" {
		return Outcome{}, false
	}
	dir, ok := s.goBinDir()
	if !ok {
		return Outcome{}, false
	}
	if s.onPath(dir) {
		return Outcome{}, false
	}
	if !isFile(filepath.Join(dir, tool.Probe[0])) {
		return Outcome{}, false
	}

	if !s.goBinPathNoted {
		s.goBinPathNoted = true
		var run textbuf.Buffer
		report.Note(run.Str("  Run: export PATH=\"$PATH\":").Str(shellSingleQuote(dir)).String())
		var why textbuf.Buffer
		report.Note(why.Str("    `go install` puts tools in ").Str(dir).
			Str(", which is not on PATH; add that line to ").Str(s.shellProfile()).
			Str(" too, so every new shell finds them").String())
	}

	var detail textbuf.Buffer
	detail.Str("in ").Str(dir).Str(", which is not on PATH")
	return Outcome{Name: tool.Name, State: StatePending, Detail: detail.String()}, true
}

// shellSingleQuote answers dir as one POSIX sh single-quoted word, so a
// directory holding $, a backtick, a double quote or a space reaches PATH
// verbatim when the printed line is pasted. A single quote cannot appear inside
// single quotes, so each one closes the word, adds an escaped quote, and
// reopens it.
func shellSingleQuote(dir string) string {
	return "'" + strings.ReplaceAll(dir, "'", `'\''`) + "'"
}
