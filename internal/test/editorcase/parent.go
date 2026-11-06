// Design: docs/architecture/testing/runner-architecture.md -- editor process ownership.

package editorcase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	editortest "github.com/ze-software/ze/internal/component/cli/testing"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	cooperativeGrace = 2 * time.Second
	childPipeWait    = 2 * time.Second
	reapNoticeAfter  = 2 * time.Second
)

// Run owns one editor process and all its temporary resources. It MUST remain
// attached until its sole Wait owner finishes, including after a hard kill.
// A kernel-stuck child is reported as cleanup-incomplete while this call retains
// ownership; there is no detached custodian or return that releases a live lease.
// Failure diagnostics are retained outside work, with their path in Error.
func Run(ctx context.Context, executable, content, scratch string, headroom int) (result *editortest.TestResult) {
	start := time.Now()
	result = &editortest.TestResult{}
	defer func() { result.Duration = time.Since(start) }()
	budget, err := editortest.ContentBudget(content, headroom)
	if err != nil {
		failResult(result, err)
		return result
	}
	caseCtx, cancel := context.WithDeadline(ctx, start.Add(budget))
	defer cancel()
	if err := caseCtx.Err(); err != nil {
		failResult(result, err)
		return result
	}
	files, err := newCaseFiles(scratch)
	if err != nil {
		failResult(result, err)
		return result
	}
	defer func() { files.finish(result) }()
	deadline, ok := caseCtx.Deadline()
	if !ok {
		failResult(result, errors.New("editor case has no deadline"))
		return result
	}
	req := request{Content: &content, Deadline: deadline, Directory: files.work}
	if err := json.NewEncoder(files.handles[0]).Encode(req); err != nil {
		failResult(result, fmt.Errorf("write editor request: %w", err))
		return result
	}
	if err := caseCtx.Err(); err != nil {
		failResult(result, err)
		return result
	}
	child, err := files.start(executable)
	if err != nil {
		failResult(result, err)
		return result
	}
	// This defer also covers panic paths. close MUST join before finish may
	// close descriptors or remove resources; repeated joins read a closed channel.
	defer child.close()
	waitErr := child.wait(caseCtx, files)
	decoded, err := readResult(files.handles[1], files.work)
	if err == nil {
		result = decoded
	} else {
		failResult(result, err)
	}
	if waitErr != nil {
		failResult(result, fmt.Errorf("editor child pid %d: %w", child.pid, waitErr))
	}
	if !time.Now().Before(deadline) {
		failResult(result, fmt.Errorf("editor timed out after %s: %w", budget, context.DeadlineExceeded))
	}
	return result
}

// caseFiles owns handles in child FD order, followed by the log and control
// writer. Its zero supports finish; only newCaseFiles returns one ready to start.
// Not safe for concurrent use. The owner MUST call finish after any child joins.
type caseFiles struct {
	root    string
	work    string
	handles [6]*os.File // request, result, diagnostics, control reader, log, control writer.
}

func newCaseFiles(scratch string) (*caseFiles, error) {
	if scratch == "" {
		scratch = os.TempDir()
	}
	scratch, err := filepath.Abs(scratch)
	if err != nil {
		return nil, err
	}
	root, err := os.MkdirTemp(scratch, "ze-editor-case-")
	if err != nil {
		return nil, err
	}
	files := &caseFiles{root: root, work: filepath.Join(root, "work")}
	if err := files.create(); err != nil {
		result := &editortest.TestResult{Error: err.Error()}
		files.finish(result)
		return nil, errors.New(result.Error)
	}
	return files, nil
}

func (files *caseFiles) create() error {
	if err := os.Mkdir(files.work, 0o700); err != nil {
		return err
	}
	for i, name := range []string{"request.json", "result.json", "stacks.txt"} {
		file, err := os.Create(filepath.Join(files.root, name)) //nolint:gosec // Fixed artifact names in the case's private directory.
		if err != nil {
			return err
		}
		files.handles[i] = file
	}
	log, err := os.Create(filepath.Join(files.root, "child.log"))
	if err != nil {
		return err
	}
	files.handles[4] = log
	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	files.handles[3], files.handles[5] = reader, writer
	return nil
}

func (files *caseFiles) stop() error {
	if files.handles[5] == nil {
		// refuse-by-default: guards stop -- nil means the pipe was already closed.
		return nil
	}
	err := files.handles[5].Close()
	files.handles[5] = nil
	return err
}

// finish MUST run only after the child is joined (or start failed). Artifacts
// of a failure intentionally survive; work never survives a successful cleanup.
func (files *caseFiles) finish(result *editortest.TestResult) {
	for i, file := range files.handles {
		if file == nil {
			continue
		}
		if err := file.Close(); err != nil {
			failResult(result, fmt.Errorf("close editor descriptor: %w", err))
		}
		files.handles[i] = nil
	}
	if err := os.RemoveAll(files.work); err != nil {
		result.TempDir = files.work
		failResult(result, fmt.Errorf("editor cleanup-incomplete: %w", err))
	} else {
		result.TempDir = ""
	}
	if result.Passed {
		if err := os.RemoveAll(files.root); err != nil {
			failResult(result, fmt.Errorf("editor artifact cleanup-incomplete: %w", err))
		}
	}
	if !result.Passed {
		result.Error += "; goroutine stacks: " + filepath.Join(files.root, "stacks.txt") + "; artifacts: " + files.root
	}
}

// runningChild has exactly one Wait worker. err is readable only after done
// closes. The owner MUST call close, even after wait has returned.
type runningChild struct {
	pid   int
	done  chan struct{}
	err   error
	force context.CancelFunc
}

func (files *caseFiles) start(executable string) (*runningChild, error) {
	hardCtx, force := context.WithCancel(context.Background())
	cmd := exec.CommandContext(hardCtx, executable, "mock", "editor-case")
	plugin.KillGroupOnCancel(cmd)
	cmd.WaitDelay = childPipeWait
	cmd.ExtraFiles = files.handles[:4]
	cmd.Stdout, cmd.Stderr = files.handles[4], files.handles[4]
	child := &runningChild{done: make(chan struct{}), force: force}
	if err := plugin.StartChild(cmd); err != nil {
		force()
		return nil, fmt.Errorf("start editor child: %w", err)
	}
	child.pid = cmd.Process.Pid
	go child.reap(cmd)
	return child, nil
}

func (child *runningChild) reap(cmd *exec.Cmd) {
	child.err = cmd.Wait()
	close(child.done)
}

// close MUST be called by the foreground owner, never a detached custodian.
func (child *runningChild) close() {
	child.force()
	<-child.done
}

func (child *runningChild) wait(ctx context.Context, files *caseFiles) error {
	select {
	case <-child.done:
		return errors.Join(child.err, ctx.Err())
	case <-ctx.Done():
	}
	stopErr := files.stop()
	grace := time.NewTimer(cooperativeGrace)
	defer grace.Stop()
	select {
	case <-child.done:
		return errors.Join(ctx.Err(), stopErr, child.err)
	case <-grace.C:
	}
	child.force()
	notice := time.NewTimer(reapNoticeAfter)
	defer notice.Stop()
	select {
	case <-child.done:
		return errors.Join(ctx.Err(), stopErr, child.err)
	case <-notice.C:
	}
	var message textbuf.Buffer
	message.Str("editor cleanup-incomplete: killed child is not reaped; retaining its work directory and scheduler slot: ").Str(files.work).Byte('\n')
	_, reportErr := os.Stderr.WriteString(message.String())
	// A kernel-uninterruptible child has no safe bounded reap. Stay attached:
	// no publication, resource deletion or lease release is allowed here.
	<-child.done
	return errors.Join(ctx.Err(), stopErr, child.err, reportErr, errors.New("editor cleanup was incomplete after forced cancellation"))
}

func failResult(result *editortest.TestResult, err error) {
	result.Passed = false
	if result.Error == "" {
		result.Error = err.Error()
		return
	}
	result.Error = errors.Join(errors.New(result.Error), err).Error()
}
