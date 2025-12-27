package functional

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestExecRun verifies basic command execution.
//
// VALIDATES: Exec can run simple commands and capture output.
// PREVENTS: Broken process spawning or output capture.
func TestExecRun(t *testing.T) {
	e := NewExec()
	ctx := context.Background()

	err := e.Run(ctx, []string{"echo", "hello"}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Wait for process to complete
	err = e.Wait()
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	stdout := e.Stdout()
	if !strings.Contains(stdout, "hello") {
		t.Errorf("Stdout() = %q, want to contain %q", stdout, "hello")
	}
}

// TestExecRunWithEnv verifies environment variable passing.
//
// VALIDATES: Custom environment variables are passed to child process.
// PREVENTS: Missing or incorrect env vars in spawned processes.
func TestExecRunWithEnv(t *testing.T) {
	e := NewExec()
	ctx := context.Background()

	env := map[string]string{"TEST_VAR": "test_value"}
	err := e.Run(ctx, []string{"sh", "-c", "echo $TEST_VAR"}, env)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	err = e.Wait()
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	stdout := e.Stdout()
	if !strings.Contains(stdout, "test_value") {
		t.Errorf("Stdout() = %q, want to contain %q", stdout, "test_value")
	}
}

// TestExecTerminate verifies graceful termination.
//
// VALIDATES: Terminate() stops running process with SIGTERM then SIGKILL.
// PREVENTS: Orphaned processes after test cleanup.
func TestExecTerminate(t *testing.T) {
	e := NewExec()
	ctx := context.Background()

	// Start a long-running process
	err := e.Run(ctx, []string{"sleep", "60"}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Terminate should complete within reasonable time
	done := make(chan struct{})
	go func() {
		e.Terminate()
		close(done)
	}()

	select {
	case <-done:
		// Good, terminated
	case <-time.After(3 * time.Second):
		t.Fatal("Terminate() took too long")
	}
}

// TestExecReady verifies Ready() returns true when process exits.
//
// VALIDATES: Ready() correctly detects process completion.
// PREVENTS: Event loop spinning on Ready() forever.
func TestExecReady(t *testing.T) {
	e := NewExec()
	ctx := context.Background()

	err := e.Run(ctx, []string{"true"}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Wait for process to exit
	time.Sleep(100 * time.Millisecond)

	if !e.Ready() {
		t.Error("Ready() = false after process exited")
	}
}

// TestExecExitCode verifies ExitCode() returns correct value.
//
// VALIDATES: Exit code is captured from child process.
// PREVENTS: Wrong exit code breaking test result detection.
func TestExecExitCode(t *testing.T) {
	tests := []struct {
		name    string
		cmd     []string
		want    int
		wantErr bool
	}{
		{"success", []string{"true"}, 0, false},
		{"failure", []string{"false"}, 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewExec()
			ctx := context.Background()

			err := e.Run(ctx, tt.cmd, nil)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}

			err = e.Wait()
			if tt.wantErr && err == nil {
				t.Error("Wait() error = nil, want error")
			}

			if got := e.ExitCode(); got != tt.want {
				t.Errorf("ExitCode() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestExecCommand verifies Command() returns the executed command string.
//
// VALIDATES: Command string is captured for logging/debugging.
// PREVENTS: Missing command info in error reports.
func TestExecCommand(t *testing.T) {
	e := NewExec()
	ctx := context.Background()

	cmd := []string{"echo", "hello", "world"}
	err := e.Run(ctx, cmd, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cmdStr := e.Command()
	if !strings.Contains(cmdStr, "echo") {
		t.Errorf("Command() = %q, want to contain %q", cmdStr, "echo")
	}
}
