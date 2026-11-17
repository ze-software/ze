//go:build linux || darwin

// Design: docs/contributing/committing.md -- pending judge rows survive failed and concurrent writers
// Related: approve_test.go -- real approval identity and namespace fixture
package rfc

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const approvalWriterChildMark = "approval-writer-race-child"
const approvalWriterReady = "APPROVAL-WRITER-READY"

// Goal: a failed real approval write preserves every earlier pending row.
// Method: a child limits regular-file growth only around Approve; EFBIG must
// leave the parent's existing ledger byte-exact, rather than truncate it.
func TestApproveWriteFailurePreservesExistingRows(t *testing.T) {
	if root, unit, child := approvalWriterChildArgs(t, "limited"); child {
		approvalWriterLimitedCall(t, root, unit)
		return
	}
	root := approvalWriterFixture(t)
	before := readApprovals(t, root)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable,
		"-test.run=^TestApproveWriteFailurePreservesExistingRows$", "-test.timeout=10s",
		"--", approvalWriterChildMark, "limited", root, "pkg.TestY")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("limited approval child: %v\n%s", err, output)
	}
	if after := readApprovals(t, root); after != before {
		t.Fatalf("failed approval changed previously recorded rows:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// Goal: two successful native approvals in separate processes retain both
// fresh verdicts and an unrelated prior row. Method: both children reach an
// explicit stdin gate before the parent releases either writer.
// This is the API-level positive control, not a timing-based discriminator.
func TestConcurrentApproveCallsRetainBothFreshRows(t *testing.T) {
	if root, unit, child := approvalWriterChildArgs(t, "concurrent"); child {
		if _, err := fmt.Fprintln(os.Stdout, approvalWriterReady); err != nil {
			t.Fatal(err)
		}
		if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
			t.Fatal(err)
		}
		if _, err := Approve(root, approvalSession, unit, "fresh concurrent verdict for "+unit); err != nil {
			t.Fatal(err)
		}
		return
	}
	root := approvalWriterFixture(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	var children []*approvalWriterChild
	t.Cleanup(func() {
		cancel()
		for _, child := range children {
			if child.joined {
				continue
			}
			if err := child.command.Wait(); err != nil {
				t.Logf("joined canceled approval child: %v", err)
			}
			child.joined = true
		}
	})
	for _, unit := range []string{"pkg.TestX", "pkg.TestY"} {
		child := &approvalWriterChild{command: exec.CommandContext(ctx, executable,
			"-test.run=^TestConcurrentApproveCallsRetainBothFreshRows$", "-test.timeout=10s",
			"--", approvalWriterChildMark, "concurrent", root, unit)}
		child.command.Stderr = &child.errors
		child.input, err = child.command.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		output, err := child.command.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		child.output = bufio.NewReader(output)
		if err := child.command.Start(); err != nil {
			t.Fatal(err)
		}
		children = append(children, child)
		approvalWriterAwaitReady(t, child)
	}
	for _, child := range children {
		if _, err := io.WriteString(child.input, "write\n"); err != nil {
			t.Fatal(err)
		}
		if err := child.input.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for _, child := range children {
		output, readErr := io.ReadAll(child.output)
		waitErr := child.command.Wait()
		child.joined = true
		if readErr != nil {
			t.Fatal(readErr)
		}
		if waitErr != nil {
			t.Fatalf("concurrent approval child: %v\n%s\n%s", waitErr, output, child.errors.String())
		}
	}
	content := readApprovals(t, root)
	for _, unit := range []string{"pkg.TestSeed", "pkg.TestX", "pkg.TestY"} {
		if count := strings.Count(content, "| "+unit+" |"); count != 1 {
			t.Fatalf("approval %s has %d rows after both successful writers:\n%s", unit, count, content)
		}
	}
	if !strings.Contains(content, ApprovalTableHeader) {
		t.Fatalf("concurrent approvals lost the table header:\n%s", content)
	}
}

// approvalWriterChild owns one subprocess lifecycle. Its parent MUST either
// join it normally or cancel and join it in test cleanup. Output is read only
// by the parent; the stderr buffer is inspected only after Wait joins copying.
type approvalWriterChild struct {
	command *exec.Cmd
	input   io.WriteCloser
	output  *bufio.Reader
	errors  bytes.Buffer
	joined  bool
}

func approvalWriterFixture(t *testing.T) string {
	t.Helper()
	root, _ := fixtureCommit(t, map[string]string{
		"pkg/x_test.go": "package pkg\nimport \"testing\"\nfunc TestSeed(t *testing.T) {}\nfunc TestX(t *testing.T) {}\nfunc TestY(t *testing.T) {}\n",
	})
	if _, err := Approve(root, approvalSession, "pkg.TestSeed", "retained prior verdict"); err != nil {
		t.Fatal(err)
	}
	return root
}

func approvalWriterChildArgs(t *testing.T, kind string) (string, string, bool) {
	t.Helper()
	args := flag.Args()
	if len(args) == 0 {
		return "", "", false
	}
	if args[0] != approvalWriterChildMark {
		return "", "", false
	}
	if len(args) != 4 {
		t.Fatal("malformed approval child arguments")
	}
	if args[1] != kind {
		t.Fatal("approval child selected the wrong question")
	}
	return args[2], args[3], true
}

func approvalWriterLimitedCall(t *testing.T, root, unit string) {
	t.Helper()
	var previous syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &previous); err != nil {
		t.Fatal(err)
	}
	signal.Ignore(syscall.SIGXFSZ)
	defer signal.Reset(syscall.SIGXFSZ)
	limited := previous
	limited.Cur = 0
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limited); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &previous); err != nil {
			t.Errorf("restore child file limit: %v", err)
		}
	}()
	_, err := Approve(root, approvalSession, unit, "a verdict whose write must fail")
	if !errors.Is(err, syscall.EFBIG) {
		t.Fatalf("approval error = %v, want the actual regular-file write's EFBIG", err)
	}
	// An existing ledger must remain addressable even after this failed write.
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(ApprovalPath(approvalSession)))); err != nil {
		t.Fatalf("failed approval removed its existing ledger: %v", err)
	}
}

func approvalWriterAwaitReady(t *testing.T, child *approvalWriterChild) {
	t.Helper()
	// The Go test harness can print its RUN line before the child marker.
	// Four lines admit that preamble but refuse unrelated unbounded output.
	for range 4 {
		line, err := child.output.ReadString('\n')
		if err != nil {
			t.Fatalf("approval child did not reach its start gate: %v", err)
		}
		if strings.TrimSpace(line) == approvalWriterReady {
			return
		}
	}
	t.Fatal("approval child output exceeded its readiness preamble")
}
