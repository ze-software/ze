// Design: docs/architecture/testing/verify-freshness-scope.md -- each stage runs as its own le process
//
// Package verifydispatch connects verifyworktree to le's local-data registry.
// It owns no command table: verifyengine.Identity names the tool and exact arguments.
package verifydispatch

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// outputDrainMax bounds how long a stage's log waits, once the stage process
// has exited, for a process it started and left running to close the output
// it inherited, and how long a canceled stage has between SIGTERM and SIGKILL.
// A test daemon that outlives its stage would otherwise hold the verify run
// forever. It is a variable only so a test can shorten the wait.
var outputDrainMax = 10 * time.Second

// spawnFunc builds the process that runs the le command line words. It MUST
// NOT start it.
type spawnFunc func(ctx context.Context, words []string) (*exec.Cmd, error)

// RunAction runs one registered le action as its own le process inside the
// requested checkout root.
//
// A stage runs in a child process rather than in this one so its log holds
// every byte the stage wrote, whatever stream it wrote it to. In process, a
// handler that wrote its reason straight to os.Stderr and returned a failing
// code left a stage log with no cause in it (`le arch tier check`, the
// staticcheck deadline, `le arch iface-resolution`), and no writer passed to
// the handler can catch a library, a child, or a helper that reaches the
// process's own streams. The process boundary catches all of them, and the
// log is then what an operator who typed the same line would have seen.
func RunAction(ctx context.Context, root string, identity verifyengine.Identity) verifyengine.ActionResult {
	return dispatch(ctx, root, identity, leroot.Owns, leroot.LookupCommand, stageProcess)
}

// stageProcess starts this binary again, under the name this process was
// started as, so the child takes the same personality and the same build name
// (cmd/ze/le_build_name.go) the parent did.
func stageProcess(ctx context.Context, words []string) (*exec.Cmd, error) {
	executable, err := stageExecutable()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, executable) // #nosec G204 -- this binary, started again with a registered command line.
	cmd.Args = stageArgv(os.Args[0], leroot.InvocationName(), words)
	return cmd, nil
}

// stageExecutable answers the image a stage process runs. On Linux it is
// /proc/self/exe, which execve resolves in the forked child to the image this
// process runs, so a bin/le replaced mid-run (`./le --update`) cannot put
// different code under later stages. Darwin has no procfs and no other handle
// that execs the running image, so there it is the binary's path, and a binary
// replaced mid-run runs its new code in the stages after the replacement.
func stageExecutable() (string, error) {
	if runtime.GOOS == "linux" {
		return "/proc/self/exe", nil
	}
	return os.Executable()
}

// stageArgv answers the argv of a stage process: argv0 as this process got it,
// then the le root word when program is a tagged ze build ("ze le"), then the
// command line words.
func stageArgv(argv0, program string, words []string) []string {
	personality := strings.Fields(program)[1:]
	argv := make([]string, 0, 1+len(personality)+len(words))
	argv = append(argv, argv0)
	argv = append(argv, personality...)
	return append(argv, words...)
}

func dispatch(
	ctx context.Context,
	root string,
	identity verifyengine.Identity,
	owns func(string) bool,
	lookup func(string) command.LocalDataHandler,
	spawn spawnFunc,
) (result verifyengine.ActionResult) {
	identity.Args = slices.Clone(identity.Args)
	var text textbuf.Buffer
	result.Identity = identity
	if err := ctx.Err(); err != nil {
		return refused(result, "interrupted", verifyengine.Interrupted, err.Error())
	}
	if strings.TrimSpace(root) == "" {
		return refused(result, "root-missing", 2, "action runner received no checkout root")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return refused(result, "root-missing", 2, messageWithError("resolve action root: ", err))
	}
	info, err := os.Stat(absoluteRoot)
	if err != nil {
		return refused(result, "root-missing", 2, messageWithError("open action root: ", err))
	}
	if !info.IsDir() {
		return refused(result, "root-missing", 2, "action root is not a directory")
	}
	if !owns(identity.Command) {
		return refused(result, "unowned-root", 2,
			text.Reset().Str("le does not own root ").Quoted(identity.Command).String())
	}
	if lookup(identity.Command) == nil {
		return refused(result, "missing-handler", 2,
			text.Reset().Str("root ").Quoted(identity.Command).
				Str(" has no registered handler").String())
	}
	result.Registered = true

	words := append(strings.Fields(identity.Command), identity.Args...)
	cmd, err := spawn(ctx, words)
	if err != nil {
		return refused(result, "spawn", 2, messageWithError("build stage process: ", err))
	}
	// The child judges the requested checkout: lepath.Root reads the root key
	// before anything else, and the working directory agrees with it.
	cmd.Dir = absoluteRoot
	cmd.Env = append(env.Without(os.Environ(), lepath.RootKey), lepath.RootKey+"="+absoluteRoot)
	// A canceled stage is asked to stop, so it can release the job slot, port
	// lock, temporary directory or namespace it holds and stop its own
	// children; WaitDelay kills it when it has not exited by then. It stays in
	// this process group on purpose: the job registry's stall breaker kills
	// verify's whole group and Ctrl-C reaches the whole foreground group, and a
	// group of its own would let a stage outlive both.
	// cutShort records that the cancel's signal, not the stage, ended the
	// process (cancelStage); Wait returns only after Cancel has.
	var cutShort atomic.Bool
	cmd.Cancel = cancelStage(cmd, &cutShort)
	cmd.WaitDelay = outputDrainMax
	// The Go runtime in the stage writes a crash to this file and to nothing
	// else (armStageCrashOutput), so a crash is told from a stage that only
	// relays another program's panic text.
	crashFile, err := stageCrashFile(absoluteRoot)
	if err != nil {
		return refused(result, "spawn", 2, messageWithError("create stage crash file: ", err))
	}
	defer discardCrashFile(crashFile)
	cmd.ExtraFiles = []*os.File{crashFile}
	cmd.Env = append(cmd.Env, crashFDKey+"="+strconv.Itoa(stageCrashFD))

	var output textbuf.Buffer
	output.Reset()
	held, runErr := runStage(cmd, &output)
	if held {
		output.Str("verify: a process this stage started still held its output open ").
			Str(outputDrainMax.String()).Str(" after the stage exited; what it wrote later is not in this log\n")
	}
	result.Output = output.String()
	// Only a stage the cancel cut short is interrupted. One that exited on its
	// own before the context was done answered, and its exit is a verdict even
	// when the interrupt has landed by now (the run loop stops after it).
	if cutShort.Load() {
		return refused(result, "interrupted", verifyengine.Interrupted,
			"the run was interrupted while the stage process ran: "+context.Cause(ctx).Error())
	}
	var exitErr *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exitErr) && !errors.Is(runErr, exec.ErrWaitDelay) {
		return refused(result, "spawn", 2, messageWithError("run stage process: ", runErr))
	}
	state := cmd.ProcessState
	if !state.Exited() {
		if stoppedByInterrupt(state) {
			return refused(result, "interrupted", verifyengine.Interrupted,
				text.Reset().Str("stage process stopped by ").Str(state.String()).String())
		}
		return refused(result, "killed", 2,
			text.Reset().Str("stage process ended without an exit status: ").Str(state.String()).String())
	}
	result.Code = state.ExitCode()
	result.Completed = true
	// A crash is a red stage, never the "could not judge" its exit status 2
	// reads as: the stage reached a defect, and the trace is in its log.
	if crashReported(crashFile) {
		result.Code = 1
		result.Failure = &verifyengine.Failure{Kind: "crashed", Stage: identity.Name,
			Message: "stage process died of a Go panic or fatal error; the trace is in its log"}
	}
	return result
}

// cancelStage answers the Cancel function for a stage process: it asks the
// process to stop with SIGTERM and records in cutShort that the cancel ended
// it.
//
// exec's context watcher can call Cancel after Wait has already reaped the
// process, when the context is done in the moment the stage exits. The signal
// then answers os.ErrProcessDone and reached nothing: the stage exited on its
// own, so its exit is a verdict and cutShort stays false. exec itself treats
// that error as no failure.
func cancelStage(cmd *exec.Cmd, cutShort *atomic.Bool) func() error {
	return func() error {
		err := cmd.Process.Signal(syscall.SIGTERM)
		if errors.Is(err, os.ErrProcessDone) {
			return err
		}
		cutShort.Store(true)
		return err
	}
}

// stageCrashFD is the descriptor the stage process holds its crash file at:
// ExtraFiles[0] is always descriptor 3 in the child.
const stageCrashFD = 3

// stageCrashFile creates the empty file a stage's Go runtime writes its crash
// to, under the checkout's tmp/, which verify already writes.
func stageCrashFile(root string) (*os.File, error) {
	dir := filepath.Join(root, "tmp")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return os.CreateTemp(dir, "verify-stage-crash-*")
}

// crashReported answers whether the stage's runtime wrote a crash: the file
// was created empty and nothing else writes to it.
func crashReported(crashFile *os.File) bool {
	info, err := crashFile.Stat()
	if err != nil {
		return false
	}
	return info.Size() > 0
}

// discardCrashFile removes the crash file once the stage's verdict is read. The
// crash text itself is already in the stage log, because the runtime writes it
// to stderr as well.
func discardCrashFile(crashFile *os.File) {
	crashFile.Close()           //nolint:errcheck // read only through Stat, nothing buffered
	os.Remove(crashFile.Name()) //nolint:errcheck // a leftover empty file under tmp/ costs nothing
}

// stoppedByInterrupt answers whether the stage process died of SIGINT or
// SIGTERM, an interrupt that reached it directly (Ctrl-C reaches the whole
// foreground process group) rather than through this run's context.
func stoppedByInterrupt(state *os.ProcessState) bool {
	status, ok := state.Sys().(syscall.WaitStatus)
	if !ok {
		return false
	}
	if !status.Signaled() {
		return false
	}
	return status.Signal() == syscall.SIGINT || status.Signal() == syscall.SIGTERM
}

// runStage runs cmd with one pipe as both of its output streams, so the log
// keeps the order they were written in, and copies that pipe into output. It
// answers whether a process the stage left running still held the pipe
// outputDrainMax after the stage exited, whatever status the stage exited
// with, and the error starting or waiting for the stage answered.
//
// One goroutine (drainStage) lives for the stage process: it ends when every
// holder of the pipe has closed it, or when runStage closes the pipe under it.
func runStage(cmd *exec.Cmd, output *textbuf.Buffer) (held bool, err error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return false, err
	}
	defer reader.Close() //nolint:errcheck // read side, closed once the copy is over

	cmd.Stdout = writer
	cmd.Stderr = writer
	startErr := cmd.Start()
	writer.Close() //nolint:errcheck // this process's copy; the stage holds its own
	if startErr != nil {
		return false, startErr
	}
	copied := make(chan struct{})
	go drainStage(reader, output, copied)
	waitErr := cmd.Wait()

	select {
	case <-copied:
	case <-time.After(outputDrainMax):
		held = true
		reader.Close() //nolint:errcheck // unblocks the copy; the close is the point
		<-copied
	}
	return held, waitErr
}

// drainStage copies a stage's output into output until the pipe ends, then
// closes copied. It MUST be the only writer to output until copied is closed.
func drainStage(reader io.Reader, output *textbuf.Buffer, copied chan<- struct{}) {
	defer close(copied)
	io.Copy(output, reader) //nolint:errcheck // a pipe closed under the copy is how a held log ends
}

func refused(result verifyengine.ActionResult, kind string, code int, message string) verifyengine.ActionResult {
	result.Code = code
	result.Completed = false
	result.Failure = &verifyengine.Failure{Kind: kind, Stage: result.Identity.Name, Message: message}
	return result
}

func messageWithError(prefix string, err error) string {
	var text textbuf.Buffer
	return text.Str(prefix).Err(err).String()
}
