// Design: docs/architecture/core-design.md -- running commands, and reaching root without a prompt
//
// Every external command uses Shell.Run.
// Commands that require root use Shell.runPrivileged. runPrivileged first
// selects the privilege method and then calls Run.
//
// THE RULE THE PRIVILEGED HALF EXISTS TO KEEP: a setup program must never wait
// for a password when nobody can type it. sudo reads its prompt from inherited
// stdin. Thus, a run without a terminal can wait indefinitely. Also, `sudo tee`
// can send a config line from stdin to the prompt instead of to the file. The
// code decides privilege BEFORE a command starts and always gives -n to sudo.
//
// Only `sudo -v` asks for a password, and only when an attached terminal can
// receive the prompt. In every other case, the program records the command for
// a person and reports failure. A setup program that states the command permits
// recovery. A program that stops without output does not.
//
// EVERY SEAM IS A FIELD. Python tests replaced module functions. A Go test sets
// Look, Exec, Euid, or Tty and runs the same code as the command. A nil field
// uses the production implementation. Therefore, the command sets no fields.

package devsetup

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// sudoProbeTimeout limits `sudo -n true`. The command accesses no network and
// reads a local timestamp file. One second is already an abnormal duration. The
// timeout only prevents a blocked sudo from stopping the run. An unreachable
// LDAP sudoers source is a usual cause.
const sudoProbeTimeout = 15 * time.Second

// sudoBin is the one route to root this tool takes when it is not already root.
const sudoBin = "sudo"

// exitCannotStart is the shell status for a command that failed to start. It
// distinguishes this condition from a command that started and failed.
const exitCannotStart = 127

// exitTimedOut is what a command that outran its budget answers. It is the code
// `timeout(1)` uses, and Python's process.py chose it for the same reason.
const exitTimedOut = 124

// Cmd is one command to run, and everything about how to run it.
type Cmd struct {
	// Argv is the command and its arguments. Argv[0] is looked up on PATH.
	Argv []string
	// Dir is the working directory. Empty means this process's own.
	Dir string
	// Stdin is fed to the command. Nil means an empty stdin, never the
	// terminal's: a command that reads a prompt must not find one.
	Stdin []byte
	// Env replaces the whole environment when it is non-nil, the way
	// os/exec reads Cmd.Env.
	Env []string
	// Timeout bounds the run. Zero means no bound.
	Timeout time.Duration
}

// Result is what a command did.
//
// Out and Err are text because every caller wants text, and an invalid UTF-8
// complaint is still worth showing.
type Result struct {
	Argv []string
	Code int
	Out  string
	Err  string
}

// OK reports whether the command succeeded.
func (r Result) OK() bool { return r.Code == 0 }

// complaint answers the first line worth showing a human when this failed.
//
// stderr first, then stdout: a tool that failed usually says why on stderr, and
// the ones that do not have already put it on stdout.
func (r Result) complaint() string {
	for _, stream := range [...]string{r.Err, r.Out} {
		trimmed := strings.TrimSpace(stream)
		if trimmed == "" {
			continue
		}
		first, _, _ := strings.Cut(trimmed, "\n")
		return first
	}
	var tb textbuf.Buffer
	return tb.Str("exit ").Int(int64(r.Code)).String()
}

// Privilege is how this process can run a root command right now.
type Privilege string

const (
	// PrivilegeRoot means already root, so no sudo is used and none needs to
	// be installed.
	PrivilegeRoot Privilege = "root"
	// PrivilegeSudo means sudo acts with no password: NOPASSWD, or a live
	// timestamp.
	PrivilegeSudo Privilege = "sudo"
	// PrivilegePrompt means sudo wants a password and a terminal is attached
	// to type it on.
	PrivilegePrompt Privilege = "sudo-prompt"
	// PrivilegeNone means no route to root that would not block. The caller
	// records the command instead of running it.
	PrivilegeNone Privilege = "none"
)

// Prefix answers what a human would have to type in front of the command.
func (p Privilege) Prefix() string {
	if p == PrivilegeRoot {
		return ""
	}
	return "sudo "
}

// Shell finds and runs commands. Each field is a test seam that a test can
// replace. A nil field uses the production implementation.
type Shell struct {
	// Look answers the path to a command on PATH, and whether it is there.
	Look func(name string) (string, bool)
	// Exec runs one command and answers what it did.
	Exec func(ctx context.Context, cmd Cmd) Result
	// Euid answers this process's effective user id.
	Euid func() int
	// Tty reports whether a terminal is attached to answer a password prompt.
	Tty func() bool
	// Ctx cancels a running command. A nil context means the background one:
	// the script had no ceiling either, and each command that can hang
	// carries its own Timeout.
	Ctx context.Context
}

// Which answers the path to name on PATH, and whether it is there.
func (s *Shell) Which(name string) (string, bool) {
	if s.Look != nil {
		return s.Look(name)
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	return path, true
}

// Present reports whether name is on PATH.
func (s *Shell) Present(name string) bool {
	_, ok := s.Which(name)
	return ok
}

// context answers the context every command runs under.
func (s *Shell) context() context.Context {
	if s.Ctx != nil {
		return s.Ctx
	}
	return context.Background()
}

// Run runs one command and captures its output.
//
// It does not return an error for a command that starts and fails. Instead, it
// returns a Result with a non-zero code, which each caller must handle. A
// command that fails to start or exceeds its Timeout also returns a Result. Err
// contains the reason. Thus, callers examine one Result instead of separate
// return and failure paths.
func (s *Shell) Run(cmd Cmd) Result {
	if s.Exec != nil {
		return s.Exec(s.context(), cmd)
	}
	return realExec(s.context(), cmd)
}

// realExec is the fork Run takes when no Exec seam is set.
func realExec(ctx context.Context, cmd Cmd) Result {
	if len(cmd.Argv) == 0 {
		return Result{Argv: cmd.Argv, Code: exitCannotStart, Err: "no command to run"}
	}

	timedOut := false
	if cmd.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cmd.Timeout)
		defer cancel()
	}

	run := exec.CommandContext(ctx, cmd.Argv[0], cmd.Argv[1:]...) //nolint:gosec // the argv is built here from a fixed table
	run.Dir = cmd.Dir
	run.Env = cmd.Env
	if cmd.Stdin != nil {
		run.Stdin = strings.NewReader(string(cmd.Stdin))
	}

	var out, errOut textbuf.Buffer
	run.Stdout = &out
	run.Stderr = &errOut

	err := run.Run()
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		timedOut = true
	}

	switch {
	case timedOut:
		var tb textbuf.Buffer
		why := tb.Str("no reply within ").Int(int64(cmd.Timeout / time.Second)).Str("s").String()
		return Result{Argv: cmd.Argv, Code: exitTimedOut, Err: why}
	case err != nil && run.ProcessState == nil:
		return Result{Argv: cmd.Argv, Code: exitCannotStart, Err: err.Error()}
	}

	code := 0
	if run.ProcessState != nil {
		code = run.ProcessState.ExitCode()
	}
	return Result{Argv: cmd.Argv, Code: code, Out: out.String(), Err: errOut.String()}
}

// Privilege selects the route to root but does not get privilege.
func (s *Shell) Privilege() Privilege {
	euid := os.Geteuid
	if s.Euid != nil {
		euid = s.Euid
	}
	if euid() == 0 {
		return PrivilegeRoot
	}
	if !s.Present(sudoBin) {
		return PrivilegeNone
	}
	if s.Run(Cmd{Argv: []string{sudoBin, "-n", "true"}, Timeout: sudoProbeTimeout}).OK() {
		return PrivilegeSudo
	}
	if s.terminal() {
		return PrivilegePrompt
	}
	return PrivilegeNone
}

// terminal reports whether a password prompt has somebody to answer it.
func (s *Shell) terminal() bool {
	if s.Tty != nil {
		return s.Tty()
	}
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// sudoPlaceholder marks the position of `sudo ` in a displayed command. This
// keeps the recorded line correct during a root run, which does not use sudo.
// The code replaces the placeholder instead of using a formatter. A brace in
// other displayed text, such as a shell expansion or an awk program, causes
// a formatter to fail.
const sudoPlaceholder = "{sudo}"

// runPrivileged runs one command as root and reports whether it succeeded. It
// also returns details for the person who must correct a failure.
//
// shown replaces the recorded command when argv differs from the command that
// a person would copy. For example, a pipe into tee writes a config drop-in.
// The argv alone shows tee without the content. If shown is empty, the code uses
// the argv and adds the prefix that a person must type.
func (s *Shell) runPrivileged(report *Report, argv []string, stdin []byte, shown string) (bool, string) {
	mode := s.Privilege()

	label := shown
	if label == "" {
		var tb textbuf.Buffer
		label = tb.Str(mode.Prefix()).Join(argv, " ").String()
	} else {
		label = strings.ReplaceAll(label, sudoPlaceholder, mode.Prefix())
	}

	if mode == PrivilegeNone {
		var tb textbuf.Buffer
		return false, tb.Str("no password-free route to root for `").Str(label).Str("`").String()
	}

	if mode == PrivilegePrompt {
		report.Note("  sudo needs your password")
		if !s.Run(Cmd{Argv: []string{sudoBin, "-v"}}).OK() {
			var tb textbuf.Buffer
			return false, tb.Str("sudo could not authenticate for `").Str(label).Str("`").String()
		}
	}

	// -n even after `sudo -v` has cached the timestamp: the prompt is what
	// eats a piped stdin, so this way no code path can reach one.
	full := argv
	if mode != PrivilegeRoot {
		full = append([]string{sudoBin, "-n"}, argv...)
	}

	var line textbuf.Buffer
	report.Note(line.Str("  Run: ").Str(label).String())

	result := s.Run(Cmd{Argv: full, Stdin: stdin})
	if !result.OK() {
		var tb textbuf.Buffer
		return false, tb.Str(label).Str(": ").Str(result.complaint()).String()
	}
	return true, label
}
