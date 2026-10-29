package fixture

// Related: register_le_menu_and_prompt.go -- the registration.
// Related: ui_fixture_le_completion_shells.go -- the terminal session and readUntil this driver reuses.
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

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// leMenuAndPromptSpec is the spec file the prompt test completes to. It lives
// in a directory that is not a Ze checkout, so the claim it runs is refused
// and nothing in any real checkout changes.
const leMenuAndPromptSpec = "plan/probe/spec-menu-probe.md"

// leMenuAndPromptHint is the last line of every menu screen.
var leMenuAndPromptHint = regexp.MustCompile(`enter select .* esc quit`)

// leMenuAndPromptNever matches no text, so a read with it lasts until the
// program exits.
var leMenuAndPromptNever = regexp.MustCompile(`[^\s\S]`)

// leMenuAndPromptRun is one program started at a pseudo-terminal. The slave
// stays open in the fixture until close, so the terminal keeps every byte the
// program wrote after it exits.
type leMenuAndPromptRun struct {
	session *leCompletionShellsSession
	slave   *os.File
}

// leMenuAndPrompt builds the le personality and drives it at a
// pseudo-terminal: bare le opens the menu, typing filters, Enter drills into a
// namespace and an action and runs it after echoing the line, q quits with 0;
// `le spec claim` asks for its spec, Tab completes the path, Enter echoes and
// runs the line; Ctrl-C at the prompt runs nothing and exits 2; with only one
// of stdin and stdout a terminal, le prints the root page and exits 0; and
// `ze le` opens the same menu under its own name.
func leMenuAndPrompt(ctx context.Context) error {
	if os.Getenv("ZE_REPO_ROOT") == "" {
		return errors.New("ZE_REPO_ROOT is not set")
	}
	root, err := filepath.Abs(os.Getenv("ZE_REPO_ROOT"))
	if err != nil {
		return fmt.Errorf("resolve ZE_REPO_ROOT: %w", err)
	}
	work, err := os.MkdirTemp("", "le-menu-and-prompt-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup

	leMenuAndPromptProgress("build the le personality")
	started := time.Now()
	lePath, zePath, err := leHelpPagesBuild(ctx, root, work)
	if err != nil {
		return err
	}
	leMenuAndPromptProgress("built in " + time.Since(started).Round(time.Second).String())
	dir := filepath.Join(work, "checkout")
	if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(leMenuAndPromptSpec)), 0o750); err != nil {
		return fmt.Errorf("lay out the spec directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, leMenuAndPromptSpec), []byte("# probe\n"), 0o600); err != nil {
		return fmt.Errorf("write the probe spec: %w", err)
	}
	// le finds its checkout from ZE_REPO_ROOT, then the working directory's
	// ancestors, then the executable's. ZE_REPO_ROOT is cleared, and both
	// directories must lie outside every Ze checkout, so the `spec claim` the
	// prompt step runs is refused and can claim nothing in a real one. A
	// TMPDIR inside a checkout would break that, so it is refused here.
	for _, place := range []string{dir, filepath.Dir(lePath)} {
		if checkout, inside := lepath.CheckoutContaining(place); inside {
			return fmt.Errorf("the fixture directory %s lies inside the Ze checkout %s, so the claim it runs could change that checkout: set TMPDIR outside it and run the test again", place, checkout)
		}
	}
	// The session id is a throwaway one as well, so even a claim that found a
	// checkout would not land on the session running this test.
	env := childEnvironment(os.Environ(), map[string]string{
		"ZE_REPO_ROOT": "", "HOME": work, envTerm: leCompletionShellsTerm,
		"CLAUDE_CODE_SESSION_ID": "le-menu-and-prompt-probe",
	})

	for _, step := range []struct {
		name string
		run  func(context.Context, string, []string, string, string) error
	}{
		{"menu drill-down, echo and run", leMenuAndPromptMenu},
		{"q quits the menu with 0", leMenuAndPromptQuit},
		{"claim prompt, Tab, echo and run", leMenuAndPromptClaim},
		{"Ctrl-C at the prompt runs nothing", leMenuAndPromptCancel},
		{"half terminal prints the root page", leMenuAndPromptHalfTerminal},
		{"ze le opens the menu", leMenuAndPromptZeLe},
	} {
		if err := leMenuAndPromptStep(ctx, step.name, func(stepCtx context.Context) error {
			return step.run(stepCtx, dir, env, lePath, zePath)
		}); err != nil {
			return err
		}
	}
	// The .ci reads this line; an exit code of 0 alone would also pass a
	// driver that returned before its checks ran.
	fmt.Println("OK")
	return nil
}

// leMenuAndPromptStepBudget is the most one step may take. Every read in a
// step stops at ten seconds and every exit wait at one, so a step that reaches
// this budget is blocked somewhere no read deadline covers (a child writing a
// terminal nobody reads, a wait with no timeout). Its context then ends, which
// kills every program the step started.
const leMenuAndPromptStepBudget = 60 * time.Second

// leMenuAndPromptStep runs one named step under the step budget, printing a
// line to stderr as it starts and as it ends, so a step that hangs names
// itself in the test log rather than leaving only the runner's timeout.
func leMenuAndPromptStep(ctx context.Context, name string, run func(context.Context) error) error {
	leMenuAndPromptProgress("step: " + name)
	stepCtx, cancel := context.WithTimeout(ctx, leMenuAndPromptStepBudget)
	defer cancel()
	started := time.Now()
	err := run(stepCtx)
	if errors.Is(stepCtx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("step %q did not finish within %s, so the fixture stopped it: %w", name, leMenuAndPromptStepBudget, err)
	}
	if err != nil {
		return fmt.Errorf("step %q: %w", name, err)
	}
	leMenuAndPromptProgress("passed in " + time.Since(started).Round(time.Millisecond).String() + ": " + name)
	return nil
}

// leMenuAndPromptProgress writes one progress line to stderr, which the
// runner keeps in the test log; stdout carries only the final OK.
func leMenuAndPromptProgress(line string) {
	_, _ = os.Stderr.WriteString("le-menu-and-prompt: " + line + "\n")
}

// leMenuAndPromptMenu checks AC-42: the menu drills from spec into spec
// current and runs its action after echoing the whole line. It checks AC-53 on
// the way: the spec row says what the namespace is for, and AC-55: entering
// spec heads its members with the namespace's long description.
func leMenuAndPromptMenu(ctx context.Context, dir string, env []string, lePath, _ string) error {
	run, err := leMenuAndPromptStart(ctx, dir, env, lePath)
	if err != nil {
		return err
	}
	defer run.close()
	for _, step := range []struct {
		keys string
		want *regexp.Regexp
	}{
		{"", leMenuAndPromptHint},
		// The renderer redraws only the cells that changed, so each step
		// matches text the step newly draws, never the cursor marker.
		// The filtered spec row carries what the namespace is for (AC-53).
		{"spec", regexp.MustCompile(`(?s)filter: spec.*the spec workflow|the spec workflow.*filter: spec`)},
		// Enter opens spec's members under a header holding its long
		// description (D-P7-short-long). The capital T is the long text's:
		// the short in the row above reads "the spec workflow".
		{"\r", regexp.MustCompile(`(?s)The spec workflow: claim, track, review.*current`)},
		{"cu", regexp.MustCompile(`filter: cu`)},
		{"\r", regexp.MustCompile(`show`)},
		{"\r", regexp.MustCompile(`running: le spec current show\n`)},
	} {
		if err := run.press(ctx, step.keys, step.want); err != nil {
			return fmt.Errorf("menu: %w", err)
		}
	}
	return nil
}

// leMenuAndPromptQuit checks AC-42's quit: q at the first screen runs nothing
// and exits 0.
func leMenuAndPromptQuit(ctx context.Context, dir string, env []string, lePath, _ string) error {
	run, err := leMenuAndPromptStart(ctx, dir, env, lePath)
	if err != nil {
		return err
	}
	defer run.close()
	if err := run.press(ctx, "", leMenuAndPromptHint); err != nil {
		return fmt.Errorf("quit: %w", err)
	}
	screen, code, err := run.exit(ctx, "q")
	if err != nil {
		return fmt.Errorf("quit: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("q at the menu exited %d and printed %q, want 0 and nothing run", code, screen)
	}
	if strings.Contains(screen, "running:") {
		return fmt.Errorf("q at the menu exited %d and printed %q, want 0 and nothing run", code, screen)
	}
	return nil
}

// leMenuAndPromptClaim checks AC-44: `le spec claim` asks for its spec, Tab
// completes the file path, and Enter echoes the full line and runs it.
func leMenuAndPromptClaim(ctx context.Context, dir string, env []string, lePath, _ string) error {
	run, err := leMenuAndPromptStart(ctx, dir, env, lePath, "spec", "claim")
	if err != nil {
		return err
	}
	defer run.close()
	for _, step := range []struct {
		keys string
		want *regexp.Regexp
	}{
		{"", regexp.MustCompile(`spec \(the spec file this session takes\):`)},
		{"plan/probe/spec-m\t", regexp.MustCompile(regexp.QuoteMeta(leMenuAndPromptSpec))},
	} {
		if err := run.press(ctx, step.keys, step.want); err != nil {
			return fmt.Errorf("prompt: %w", err)
		}
	}
	screen, code, err := run.exit(ctx, "\r")
	if err != nil {
		return fmt.Errorf("prompt: %w", err)
	}
	echo := "running: le spec claim " + leMenuAndPromptSpec + "\n"
	_, after, echoed := strings.Cut(screen, echo)
	if !echoed {
		return fmt.Errorf("prompt: Enter did not echo %q:\n%s", echo, screen)
	}
	// The line ran: le, finding no checkout, refuses the claim itself, so the
	// code is not 0 and the no-checkout refusal follows the echo. That
	// refusal is also the proof the claim took nothing in any checkout.
	if code == 0 {
		return fmt.Errorf("prompt: the echoed line exited %d with %q after the echo, want the claim refused with %q", code, after, lepath.ErrNoCheckout.Error())
	}
	if !strings.Contains(after, lepath.ErrNoCheckout.Error()) {
		return fmt.Errorf("prompt: the echoed line exited %d with %q after the echo, want the claim refused with %q", code, after, lepath.ErrNoCheckout.Error())
	}
	return nil
}

// leMenuAndPromptCancel checks AC-46: Ctrl-C at the prompt runs nothing and
// exits 2.
func leMenuAndPromptCancel(ctx context.Context, dir string, env []string, lePath, _ string) error {
	run, err := leMenuAndPromptStart(ctx, dir, env, lePath, "spec", "claim")
	if err != nil {
		return err
	}
	defer run.close()
	if err := run.press(ctx, "plan", regexp.MustCompile(`: plan`)); err != nil {
		return fmt.Errorf("cancel: %w", err)
	}
	screen, code, err := run.exit(ctx, "\x03")
	if err != nil {
		return fmt.Errorf("cancel: %w", err)
	}
	if code != 2 {
		return fmt.Errorf("Ctrl-C at the prompt exited %d and printed %q, want 2, nothing ran", code, screen)
	}
	if strings.Contains(screen, "running:") {
		return fmt.Errorf("Ctrl-C at the prompt exited %d and printed %q, want 2, nothing ran", code, screen)
	}
	if !strings.Contains(screen, "nothing ran") {
		return fmt.Errorf("Ctrl-C at the prompt exited %d and printed %q, want 2, nothing ran", code, screen)
	}
	return nil
}

// leMenuAndPromptHalfTerminal checks AC-43: with stdin a terminal and stdout a
// pipe, and the reverse, bare le prints the root page and exits 0.
func leMenuAndPromptHalfTerminal(ctx context.Context, dir string, env []string, lePath, _ string) error {
	for _, stdinTerminal := range []bool{true, false} {
		master, slave, err := displayFillOpenPTY()
		if err != nil {
			return fmt.Errorf("open pseudo-terminal: %w", err)
		}
		cmd := exec.CommandContext(ctx, lePath) //nolint:gosec // the fixture built the program
		cmd.Dir, cmd.Env = dir, env
		cmd.WaitDelay = time.Second
		var piped bytes.Buffer
		cmd.Stderr = &piped
		if stdinTerminal {
			cmd.Stdin, cmd.Stdout = slave, &piped
		} else {
			cmd.Stdin, cmd.Stdout = strings.NewReader(""), slave
		}
		process, err := displayFillStart(cmd)
		if err != nil {
			_ = slave.Close()
			_ = master.Close()
			return fmt.Errorf("start bare le with stdin terminal %v: %w", stdinTerminal, err)
		}
		// With stdout at the terminal, the terminal is read WHILE le runs,
		// never after: the root page is larger than a pseudo-terminal's
		// buffer, so a le whose terminal nobody reads blocks in its write and
		// never exits. The read lasts until le exits or ten seconds pass.
		session := &leCompletionShellsSession{master: master, process: process}
		var screen string
		if !stdinTerminal {
			screen, _ = session.readUntil(ctx, leMenuAndPromptNever)
		}
		exited := displayFillWait(process, 10*time.Second)
		session.close()
		_ = slave.Close()
		if !exited {
			return fmt.Errorf("bare le with stdin terminal %v did not exit within ten seconds, so the fixture killed it; it printed %q", stdinTerminal, piped.String()+screen)
		}
		page := piped.String() + screen
		if process.waitErr != nil {
			return fmt.Errorf("bare le with stdin terminal %v printed %q and answered %w, want 0 and the root page", stdinTerminal, page, process.waitErr)
		}
		if strings.Contains(page, "esc quit") {
			return fmt.Errorf("bare le with stdin terminal %v printed %q, want the root page and no menu", stdinTerminal, page)
		}
		if !strings.Contains(page, "\nspec: the spec workflow: ") {
			return fmt.Errorf("bare le with stdin terminal %v printed %q, want the root page and no menu", stdinTerminal, page)
		}
	}
	return nil
}

// leMenuAndPromptZeLe checks AC-48's menu half: `ze le` opens the same menu,
// titled with its own name, and q quits it with 0.
func leMenuAndPromptZeLe(ctx context.Context, dir string, env []string, _, zePath string) error {
	run, err := leMenuAndPromptStart(ctx, dir, env, zePath, "le")
	if err != nil {
		return err
	}
	defer run.close()
	if err := run.press(ctx, "", regexp.MustCompile(`ze le[\s\S]*spec[\s\S]*esc quit`)); err != nil {
		return fmt.Errorf("ze le: %w", err)
	}
	_, code, err := run.exit(ctx, "q")
	if err != nil {
		return fmt.Errorf("ze le: q, want exit 0: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("ze le: q exited %d, want 0", code)
	}
	return nil
}

// leMenuAndPromptStart starts program with args at a fresh pseudo-terminal.
func leMenuAndPromptStart(ctx context.Context, dir string, env []string, program string, args ...string) (*leMenuAndPromptRun, error) {
	master, slave, err := displayFillOpenPTY()
	if err != nil {
		return nil, fmt.Errorf("open pseudo-terminal: %w", err)
	}
	cmd := exec.CommandContext(ctx, program, args...) //nolint:gosec // the fixture built the program
	cmd.Dir, cmd.Env = dir, env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	process, err := displayFillStart(cmd)
	if err != nil {
		_ = master.Close()
		_ = slave.Close()
		return nil, fmt.Errorf("start %s: %w", program, err)
	}
	return &leMenuAndPromptRun{session: &leCompletionShellsSession{master: master, process: process}, slave: slave}, nil
}

// press types keys and reads the terminal until want matches.
func (r *leMenuAndPromptRun) press(ctx context.Context, keys string, want *regexp.Regexp) error {
	if keys != "" {
		if err := displayFillWrite(ctx, r.session.master, []byte(keys)); err != nil {
			return fmt.Errorf("type %q: %w", keys, err)
		}
	}
	screen, err := r.session.readUntil(ctx, want)
	if err != nil {
		return fmt.Errorf("after %q: %w\n%s", keys, err, screen)
	}
	return nil
}

// exit types keys and answers what the program printed until it exited, and
// its exit code.
func (r *leMenuAndPromptRun) exit(ctx context.Context, keys string) (string, int, error) {
	if err := displayFillWrite(ctx, r.session.master, []byte(keys)); err != nil {
		return "", 0, fmt.Errorf("type %q: %w", keys, err)
	}
	// The terminal is drained until the program exits: a program whose
	// output nobody reads blocks writing its terminal restore into a full
	// pseudo-terminal buffer, and never exits.
	screen, _ := r.session.readUntil(ctx, leMenuAndPromptNever)
	if !displayFillWait(r.session.process, time.Second) {
		return screen, 0, fmt.Errorf("the program did not exit within ten seconds of %q", keys)
	}
	return screen, r.session.process.cmd.ProcessState.ExitCode(), nil
}

// close stops the program and releases its terminal.
func (r *leMenuAndPromptRun) close() {
	r.session.close()
	_ = r.slave.Close()
}
