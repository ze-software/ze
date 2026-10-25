package fixture

// Related: register_le_completion_shells.go -- the registration.
// Related: ui_fixture_display_fill_completion.go -- the pseudo-terminal helpers this driver reuses.
// Related: ui_fixture_le_help_pages.go -- leHelpPagesBuild, the build this driver shares.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// leCompletionShellsTerm is the terminal the shells are told they drive: one
// whose line editor runs, so Tab reaches the completion function.
const leCompletionShellsTerm = "xterm"

// leCompletionShellsLine matches the line the echo wrapper prints: the
// command line as the shell held it after the Tabs, between LINE: and '|'.
var leCompletionShellsLine = regexp.MustCompile(`(?m)^LINE:(.*)\|$`)

// leCompletionShellsReady matches the line a shell prints once its setup ran.
var leCompletionShellsReady = regexp.MustCompile(`(?m)^READY$`)

// errLeCompletionShellsLost is a line that does not end with the @ typed after
// the Tabs.
var errLeCompletionShellsLost = errors.New("the probe character was not on the line")

// leCompletionShellsRetired are the commit create keywords that are retired,
// so completion never offers them (AC-33).
var leCompletionShellsRetired = []string{"push", "unverified", "missing-full-verify-ok"}

// leCompletionShellsShell is one interactive shell the driver types into.
type leCompletionShellsShell struct {
	name string
	// argv starts the shell interactive, with no user configuration.
	argv []string
	// setup loads le's script; it ends by printing READY.
	setup string
	// list is the keystrokes that list the candidates of an ambiguous word.
	list string
}

// leCompletionShellsBash is bash, loading the script binary prints. Readline
// lists an ambiguous word on the second Tab.
func leCompletionShellsBash(binary string) leCompletionShellsShell {
	const bash = "bash"
	return leCompletionShellsShell{
		name:  bash,
		argv:  []string{bash, "--norc", "--noprofile", "-i"},
		setup: `eval "$(` + binary + ` completion bash)"; echo READY` + "\r",
		list:  "\t\t",
	}
}

// leCompletionShellsSession is one shell running at a pseudo-terminal.
type leCompletionShellsSession struct {
	master  *os.File
	process *displayFillProcess
}

// leCompletionShells builds the le personality, lays it out as a checkout (the
// binary at bin/le, a launcher script at ./le that records any run), and types
// into bash and zsh at a pseudo-terminal after loading le's script: a namespace
// and a verb complete, commit create lists its live keywords and none of the
// retired ones, a used keyword is not offered again, an enum keyword's values
// complete, and the same through the word le on PATH. fish answers complete -C.
// The callback writes records only, ze le answers the same candidates, and with
// bin/le absent Tab completes nothing and the launcher never runs.
func leCompletionShells(ctx context.Context) error {
	if os.Getenv("ZE_REPO_ROOT") == "" {
		return errors.New("ZE_REPO_ROOT is not set")
	}
	root, err := filepath.Abs(os.Getenv("ZE_REPO_ROOT"))
	if err != nil {
		return fmt.Errorf("resolve ZE_REPO_ROOT: %w", err)
	}
	work, err := os.MkdirTemp("", "le-completion-shells-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup

	lePath, zePath, err := leHelpPagesBuild(ctx, root, work)
	if err != nil {
		return err
	}
	checkout, marker, err := leCompletionShellsCheckout(work, lePath)
	if err != nil {
		return err
	}
	env := childEnvironment(os.Environ(), map[string]string{
		"PATH":    checkout + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME":    work,
		"ZDOTDIR": work,
		"INPUTRC": os.DevNull,
		envTerm:   leCompletionShellsTerm,
		"PS1":     "$ ",
		"EDITOR":  "",
		"VISUAL":  "",
	})

	if err := leCompletionShellsWords(ctx, checkout, zePath); err != nil {
		return err
	}
	for _, shell := range []leCompletionShellsShell{
		leCompletionShellsBash("bin/le"),
		{name: "zsh", argv: []string{"zsh", "-f", "-i"}, setup: `setopt no_prompt_sp; bindkey -e; autoload -Uz compinit; compinit -u; source <(bin/le completion zsh); echo READY` + "\r", list: "\t"},
	} {
		if err := leCompletionShellsInteractive(ctx, checkout, env, shell); err != nil {
			return err
		}
	}
	if err := leCompletionShellsFish(ctx, checkout, env); err != nil {
		return err
	}
	if err := leCompletionShellsAbsent(ctx, checkout, env); err != nil {
		return err
	}
	if _, err := os.Stat(marker); err == nil {
		return errors.New("the launcher script ran during completion: a keystroke would have started a build")
	}
	// The .ci reads this line; an exit code of 0 alone would also pass a
	// driver that returned before its checks ran.
	fmt.Println("OK")
	return nil
}

// leCompletionShellsCheckout lays out a checkout under work: the built binary
// at bin/le and a launcher script at le that records that it ran.
func leCompletionShellsCheckout(work, lePath string) (string, string, error) {
	checkout := filepath.Join(work, "checkout")
	if err := os.MkdirAll(filepath.Join(checkout, "bin"), 0o755); err != nil {
		return "", "", fmt.Errorf("create the checkout: %w", err)
	}
	if err := os.Link(lePath, filepath.Join(checkout, "bin", "le")); err != nil {
		return "", "", fmt.Errorf("place bin/le: %w", err)
	}
	marker := filepath.Join(work, "launcher-ran")
	launcher := "#!/bin/sh\necho ran >> '" + marker + "'\nexit 3\n"
	if err := os.WriteFile(filepath.Join(checkout, "le"), []byte(launcher), 0o755); err != nil { //nolint:gosec // the launcher must be executable
		return "", "", fmt.Errorf("write the launcher script: %w", err)
	}
	return checkout, marker, nil
}

// leCompletionShellsWords checks AC-36 and the completion half of AC-48: the
// callback writes candidate records only and nothing on stderr, and ze le
// answers the same candidates.
func leCompletionShellsWords(ctx context.Context, checkout, zePath string) error {
	ask := func(program string, args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, program, args...) //nolint:gosec // the fixture chooses the program and its arguments
		cmd.Dir = checkout
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("%s %s: %w\n%s", filepath.Base(program), strings.Join(args, " "), err, stderr.String())
		}
		if stderr.Len() != 0 {
			return "", fmt.Errorf("%s %s wrote to stderr: %q", filepath.Base(program), strings.Join(args, " "), stderr.String())
		}
		return stdout.String(), nil
	}
	le, err := ask(filepath.Join(checkout, "bin", "le"), "completion", "words", "spec", "cl")
	if err != nil {
		return err
	}
	if !strings.HasPrefix(le, "claim\t") {
		return fmt.Errorf("le completion words spec cl answered %q, want the claim record", le)
	}
	for line := range strings.SplitSeq(strings.TrimSuffix(le, "\n"), "\n") {
		if !strings.Contains(line, "\t") {
			return fmt.Errorf("le completion words wrote %q, which is not a candidate record", line)
		}
	}
	ze, err := ask(zePath, "le", "completion", "words", "spec", "cl")
	if err != nil {
		return err
	}
	if ze != le {
		return fmt.Errorf("ze le completion words spec cl answered %q, le answered %q", ze, le)
	}
	return nil
}

// leCompletionShellsInteractive checks AC-33 (bash) and AC-34 (zsh) at a
// pseudo-terminal.
func leCompletionShellsInteractive(ctx context.Context, checkout string, env []string, shell leCompletionShellsShell) error {
	session, err := leCompletionShellsStart(ctx, checkout, env, shell)
	if err != nil {
		return err
	}
	defer displayFillStop(session.process, syscall.SIGKILL, 3*time.Second)
	defer session.master.Close() //nolint:errcheck // fixture teardown

	for _, check := range []struct{ typed, want string }{
		{"./le sp\t", "./le spec "},
		{"./le spec cl\t", "./le spec claim "},
		{"./le commit create subject x su\t", "./le commit create subject x su"},
		{"./le yang migration path-refactor operation ren\t", "./le yang migration path-refactor operation rename "},
		{"le sp\t", "le spec "},
	} {
		_, line, err := session.line(ctx, check.typed)
		if err != nil {
			return fmt.Errorf("%s: %w", shell.name, err)
		}
		if line != check.want {
			return fmt.Errorf("%s: Tab after %q left %q, want %q", shell.name, strings.TrimSuffix(check.typed, "\t"), line, check.want)
		}
	}
	screen, _, err := session.line(ctx, "./le commit create "+shell.list)
	if err != nil {
		return fmt.Errorf("%s: %w", shell.name, err)
	}
	for _, live := range []string{"subject", "body", "file-list", "dry-run"} {
		if !strings.Contains(screen, live) {
			return fmt.Errorf("%s: commit create does not list %q:\n%s", shell.name, live, screen)
		}
	}
	for _, retired := range leCompletionShellsRetired {
		if regexp.MustCompile(`(^|\s)` + regexp.QuoteMeta(retired) + `(\s|$)`).MatchString(screen) {
			return fmt.Errorf("%s: commit create lists the retired keyword %q:\n%s", shell.name, retired, screen)
		}
	}
	screen, _, err = session.line(ctx, "./le yang migration path-refactor operation "+shell.list)
	if err != nil {
		return fmt.Errorf("%s: %w", shell.name, err)
	}
	for _, value := range []string{"remove", "rename", "move"} {
		if !strings.Contains(screen, value) {
			return fmt.Errorf("%s: the operation keyword does not list %q:\n%s", shell.name, value, screen)
		}
	}
	return nil
}

// leCompletionShellsStart starts shell at a pseudo-terminal in dir and runs
// its setup, answering once the shell printed READY.
func leCompletionShellsStart(ctx context.Context, dir string, env []string, shell leCompletionShellsShell) (*leCompletionShellsSession, error) {
	if _, err := exec.LookPath(shell.argv[0]); err != nil {
		return nil, fmt.Errorf("%s is not installed, so its completion cannot be driven: install %s and run the test again", shell.name, shell.name)
	}
	master, slave, err := displayFillOpenPTY(50, 200)
	if err != nil {
		return nil, fmt.Errorf("open pseudo-terminal: %w", err)
	}
	cmd := exec.CommandContext(ctx, shell.argv[0], shell.argv[1:]...) //nolint:gosec // the fixture chooses the shell
	cmd.Dir = dir
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	process, err := displayFillStart(cmd)
	closeErr := slave.Close()
	if err != nil {
		_ = master.Close()
		return nil, fmt.Errorf("start %s: %w", shell.name, err)
	}
	session := &leCompletionShellsSession{master: master, process: process}
	if closeErr != nil {
		session.close()
		return nil, fmt.Errorf("close pseudo-terminal slave: %w", closeErr)
	}
	if err := displayFillWrite(ctx, master, []byte(shell.setup)); err != nil {
		session.close()
		return nil, fmt.Errorf("write the %s setup: %w", shell.name, err)
	}
	if screen, err := session.readUntil(ctx, leCompletionShellsReady); err != nil {
		session.close()
		return nil, fmt.Errorf("%s never finished its setup: %w\n%s", shell.name, err, screen)
	}
	return session, nil
}

// close stops the shell and releases its terminal.
func (s *leCompletionShellsSession) close() {
	displayFillStop(s.process, syscall.SIGKILL, 3*time.Second)
	_ = s.master.Close()
}

// line types typed (Tabs included) and one @, then wraps the edited line in an
// echo. The @ is a character a user would type next. zsh removes the space a
// completion inserted when the next key edits rather than inserts (Ctrl-A
// does), so without it the check would read a line no user sees. The echo
// is Ctrl-A, `echo "LINE:`, Ctrl-E, `|"`, Enter. The shell prints the line as it
// held it, so the check reads the completed text rather than the redraws.
// It answers the screen since typing and the line.
func (s *leCompletionShellsSession) line(ctx context.Context, typed string) (string, string, error) {
	if err := displayFillWrite(ctx, s.master, []byte(typed+"@")); err != nil {
		return "", "", fmt.Errorf("type %q: %w", typed, err)
	}
	if err := displayFillWrite(ctx, s.master, []byte("\x01echo \"LINE:\x05|\"\r")); err != nil {
		return "", "", fmt.Errorf("wrap %q in an echo: %w", typed, err)
	}
	screen, err := s.readUntil(ctx, leCompletionShellsLine)
	if err != nil {
		return screen, "", fmt.Errorf("after %q: %w\n%s", typed, err, screen)
	}
	line, typedNext := strings.CutSuffix(leCompletionShellsLine.FindStringSubmatch(screen)[1], "@")
	if !typedNext {
		return screen, "", fmt.Errorf("after %q the line lost the @ typed next: %w", typed, errLeCompletionShellsLost)
	}
	return screen, line, nil
}

// readUntil reads the terminal until want matches its text with escapes and
// carriage returns removed (a shell starts its output line with one), or ten
// seconds pass.
func (s *leCompletionShellsSession) readUntil(ctx context.Context, want *regexp.Regexp) (string, error) {
	deadline := time.Now().Add(10 * time.Second)
	var screen strings.Builder
	for time.Now().Before(deadline) {
		chunk, err := displayFillReadAvailable(ctx, s.master, deadline)
		if err != nil {
			return screen.String(), err
		}
		screen.WriteString(strings.ReplaceAll(displayFillANSI.ReplaceAllString(chunk, ""), "\r", ""))
		if want.MatchString(screen.String()) {
			return screen.String(), nil
		}
		if displayFillExited(s.process) {
			return screen.String(), errors.New("the shell exited")
		}
	}
	return screen.String(), fmt.Errorf("no line matched %s within ten seconds", want)
}

// leCompletionShellsFish checks AC-35: fish offers spec for ./le sp and le sp.
func leCompletionShellsFish(ctx context.Context, checkout string, env []string) error {
	if _, err := exec.LookPath("fish"); err != nil {
		return errors.New("fish is not installed, so its completion cannot be driven: install fish and run the test again")
	}
	script := filepath.Join(checkout, "..", "le.fish")
	generate := exec.CommandContext(ctx, filepath.Join(checkout, "bin", "le"), "completion", "fish") //nolint:gosec // the le binary this fixture built
	out, err := generate.Output()
	if err != nil {
		return fmt.Errorf("le completion fish: %w", err)
	}
	if err := os.WriteFile(script, out, 0o600); err != nil {
		return fmt.Errorf("write the fish script: %w", err)
	}
	for _, typed := range []string{"./le sp", "le sp"} {
		cmd := exec.CommandContext(ctx, "fish", "--no-config", "-c", "source "+script+"; complete -C '"+typed+"'") //nolint:gosec // the fixture chooses the program and its arguments
		cmd.Dir = checkout
		cmd.Env = env
		answer, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("fish complete -C %q: %w\n%s", typed, err, answer)
		}
		if !regexp.MustCompile(`(?m)^spec(\t|$)`).Match(answer) {
			return fmt.Errorf("fish complete -C %q answered %q, want spec", typed, answer)
		}
	}
	return nil
}

// leCompletionShellsAbsent checks AC-36's absent half: with bin/le gone, Tab
// after ./le completes nothing, and the launcher script is never called.
func leCompletionShellsAbsent(ctx context.Context, checkout string, env []string) error {
	binary := filepath.Join(checkout, "bin", "le")
	away := binary + ".away"
	if err := os.Rename(binary, away); err != nil {
		return fmt.Errorf("remove bin/le: %w", err)
	}
	defer os.Rename(away, binary) //nolint:errcheck // fixture cleanup; the directory is removed after
	shell := leCompletionShellsBash("bin/le.away")
	session, err := leCompletionShellsStart(ctx, checkout, env, shell)
	if err != nil {
		return err
	}
	defer session.close()
	_, line, err := session.line(ctx, "./le sp\t")
	if err != nil {
		return fmt.Errorf("bash with bin/le absent: %w", err)
	}
	if line != "./le sp" {
		return fmt.Errorf("bash with bin/le absent completed ./le sp to %q, want nothing", line)
	}
	return nil
}
