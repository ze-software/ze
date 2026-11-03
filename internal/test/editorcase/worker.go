// Design: docs/architecture/testing/runner-architecture.md -- editor child FD protocol.

// Package editorcase owns the editor subprocess protocol. A result is a child
// report, not proof of reaping: the parent MUST wait and clean up before publishing it.
package editorcase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/pprof"
	"slices"
	"time"

	"golang.org/x/sys/unix"

	editortest "github.com/ze-software/ze/internal/component/cli/testing"
)

// request is an untrusted wire value; readRequest validates it before execution.
// Content is a pointer so an omitted authored snapshot cannot become an empty case.
type request struct {
	Content   *string   `json:"content"`
	Deadline  time.Time `json:"deadline"`
	Directory string    `json:"directory"`
}

const messageBytesMax = 64 << 20

// Main is the hidden le mock editor-case entry. The parent MUST supply regular
// request/result/diagnostic files at FDs 3/4/5 and a control pipe at FD 6.
// Stdout and stderr are the parent's log handles, never the result transport.
func Main(args []string) int {
	if len(args) == 1 {
		if slices.Contains([]string{"help", "-h", "--help"}, args[0]) {
			os.Stdout.WriteString("le mock editor-case\n\nInternal editor worker: inherited FDs 3=request, 4=result, 5=diagnostics, 6=control pipe.\nThe parent owns stdout/stderr logs and the working directory. Run le test editor for the suite.\n") //nolint:errcheck // CLI help output.
			return 0
		}
	}
	if len(args) != 0 {
		os.Stderr.WriteString("le mock editor-case takes no arguments; run le test editor to own its inherited descriptors\n") //nolint:errcheck // Final CLI diagnostic.
		return 2
	}
	if err := mainFiles(); err != nil {
		os.Stderr.WriteString("editor child failed: " + err.Error() + "; run le test editor with owned child descriptors\n") //nolint:errcheck // Final CLI diagnostic.
		return 1
	}
	return 0
}

func mainFiles() (err error) {
	// A pollable inherited pipe lets the owner close and join the reader even
	// when the parent has sent no cancellation byte.
	if err := unix.SetNonblock(6, true); err != nil {
		return fmt.Errorf("control descriptor: %w", err)
	}
	files := make([]*os.File, 0, 4)
	defer func() {
		for _, file := range files {
			err = errors.Join(err, file.Close())
		}
	}()
	for descriptor := 3; descriptor <= 6; descriptor++ {
		file := os.NewFile(uintptr(descriptor), "editor-child-ipc")
		if file == nil {
			return fmt.Errorf("missing inherited descriptor %d", descriptor)
		}
		files = append(files, file)
		stat, err := file.Stat()
		if err != nil {
			return err
		}
		if descriptor == 6 {
			if stat.Mode()&os.ModeNamedPipe == 0 {
				return errors.New("control descriptor must be a pipe")
			}
			continue
		}
		if !stat.Mode().IsRegular() {
			return errors.New("request, result and diagnostic descriptors must be regular files")
		}
	}
	return run(files[0], files[1], files[2], files[3])
}

func readRequest(file *os.File) (*request, error) {
	var req request
	if err := readMessage(file, &req); err != nil {
		return nil, fmt.Errorf("editor request: %w", err)
	}
	if req.Content == nil {
		return nil, errors.New("editor request lacks authored content")
	}
	if req.Deadline.IsZero() {
		return nil, errors.New("editor request lacks an absolute deadline")
	}
	if !filepath.IsAbs(req.Directory) {
		return nil, errors.New("editor request needs an absolute parent-owned directory")
	}
	stat, err := os.Stat(req.Directory)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, errors.New("editor request working directory is not a directory")
	}
	return &req, nil
}

// readMessage rejects an absent, partial, unknown-field or trailing message.
// Its byte bound is checked on the regular file before decoding.
func readMessage(file *os.File, target any) error {
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	if stat.Size() > messageBytesMax {
		return errors.New("editor IPC exceeds 64 MiB")
	}
	decoder := json.NewDecoder(io.NewSectionReader(file, 0, stat.Size()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("editor IPC has trailing data")
	}
	return nil
}

func run(input, output, diagnostic, control *os.File) error {
	req, err := readRequest(input)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithDeadline(context.Background(), req.Deadline)
	defer cancel()
	if err := control.SetReadDeadline(req.Deadline); err != nil {
		return fmt.Errorf("control pipe deadline: %w", err)
	}
	joined := make(chan error, 1)
	finished := make(chan struct{})
	go watchControl(control, diagnostic, cancel, req.Deadline, finished, joined)

	result := editortest.RunETContentIn(ctx, *req.Content, req.Directory)
	// Waking the reader ends its one lifetime on normal completion too. The
	// caller MUST join it before writing a result; watchControl MUST finish
	// diagnostics before sending its join answer.
	close(finished)
	closeErr := control.SetReadDeadline(time.Now())
	watchErr := <-joined
	if closeErr != nil {
		return errors.Join(closeErr, watchErr)
	}
	if watchErr != nil {
		return watchErr
	}
	if err := ctx.Err(); err != nil {
		result.Passed = false
		result.Error = errors.Join(errors.New(result.Error), err).Error()
	}
	if !time.Now().Before(req.Deadline) {
		result.Passed = false
		result.Error = errors.Join(errors.New(result.Error), context.DeadlineExceeded).Error()
	}
	return json.NewEncoder(output).Encode(result)
}

// watchControl owns one read, bounded by the case deadline or the driver's
// final read deadline. Any byte or parent EOF requests cooperative shutdown.
func watchControl(control, diagnostic *os.File, cancel context.CancelFunc, deadline time.Time, finished <-chan struct{}, joined chan<- error) {
	var stop [1]byte
	_, err := control.Read(stop[:])
	if errors.Is(err, os.ErrDeadlineExceeded) {
		if time.Now().Before(deadline) {
			select {
			case <-finished:
				joined <- nil
				return
			default:
			}
		}
	}
	cancel()
	if err != nil {
		if !errors.Is(err, io.EOF) {
			if !errors.Is(err, os.ErrDeadlineExceeded) {
				joined <- err
				return
			}
		}
	}
	joined <- pprof.Lookup("goroutine").WriteTo(diagnostic, 2)
}
