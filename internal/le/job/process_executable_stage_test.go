// Design: docs/contributing/running-commands.md -- portable executable artifacts.
package job

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/seedinit"
	"github.com/ze-software/ze/internal/le/gaterun"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

func TestPortableHeldExecutable(t *testing.T) {
	scratch := portableScratch(t)
	path := filepath.Join(t.TempDir(), "program")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf 'original:%s:%s' \"$1\" \"$2\"\nexit 23\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close() //nolint:errcheck // Borrowed test input.
	if _, err := file.Seek(7, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+".held"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 99\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	argv := []string{path, "one two", "three"}
	cmd := exec.CommandContext(t.Context(), argv[0], argv[1:]...)
	cleanup, err := stageProcessExecutable(cmd, file)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cleanup != nil {
			_ = cleanup()
		}
	})
	if filepath.Dir(cmd.Path) != scratch {
		t.Fatalf("stage escaped managed scratch: %s", cmd.Path)
	}
	if !reflect.DeepEqual(cmd.Args, argv) {
		t.Fatalf("argv changed: %q", cmd.Args)
	}
	info, err := os.Stat(cmd.Path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("stage mode: %v", info.Mode())
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cmd.Path); err != nil {
		t.Fatalf("stage removed before reap: %v", err)
	}
	err = cmd.Wait()
	if err == nil || cmd.ProcessState.ExitCode() != 23 {
		t.Fatalf("exit: %v, %v", cmd.ProcessState, err)
	}
	if output.String() != "original:one two:three" {
		t.Fatalf("output: %q", output.String())
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	cleanup = nil
	if _, err := os.Stat(cmd.Path); !os.IsNotExist(err) {
		t.Fatalf("stage not removed: %v", err)
	}
	if offset, err := file.Seek(0, io.SeekCurrent); err != nil || offset != 7 {
		t.Fatalf("borrowed offset=%d err=%v", offset, err)
	}
}

func portableScratch(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("ZE_REPO_ROOT", root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "portable-executable-fixture")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	paths, err := lepath.ResolveSession(root, true)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, paths.Scratch)
}

func TestPortableExecutableLifecycle(t *testing.T) {
	for _, outcome := range []string{"success", "exit", "start", "environment", "cleanup"} {
		t.Run(outcome, func(t *testing.T) {
			scratch := portableScratch(t)
			path := filepath.Join(t.TempDir(), "program")
			body := "#!/bin/sh\nexit 0\n"
			if outcome == "exit" {
				body = "#!/bin/sh\nexit 23\n"
			}
			if outcome == "start" {
				body = "not an executable\n"
			}
			if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close() //nolint:errcheck // Test input.
			streams := ProcessIO{Executable: file}
			if outcome == "environment" {
				streams.SeedDirectory, err = os.Open(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				defer streams.SeedDirectory.Close() //nolint:errcheck // Test input.
				streams.Environ = []string{seedinit.DirectoryKey + "=conflict"}
			}
			cleanupFailure := errors.New("test cleanup failure")
			cleaned := false
			prepare := func(cmd *exec.Cmd, file *os.File) (func() error, error) {
				cleanup, err := stageProcessExecutable(cmd, file)
				if err != nil {
					return nil, err
				}
				return func() error {
					if outcome != "start" && outcome != "environment" && cmd.ProcessState == nil {
						t.Error("cleanup before child reap")
					}
					cleaned = true
					err := cleanup()
					if outcome == "cleanup" {
						return errors.Join(err, cleanupFailure)
					}
					return err
				}, nil
			}
			code, err := runProcess([]string{path}, streams, prepare)
			switch outcome {
			case "success":
				if code != 0 || err != nil {
					t.Fatalf("code=%d err=%v", code, err)
				}
			case "exit":
				if code != 23 || err != nil {
					t.Fatalf("code=%d err=%v", code, err)
				}
			case "start", "environment":
				if code != gaterun.CannotStart || err == nil {
					t.Fatalf("code=%d err=%v", code, err)
				}
			case "cleanup":
				if !errors.Is(err, cleanupFailure) {
					t.Fatalf("lost cleanup failure: %v", err)
				}
			}
			if !cleaned {
				t.Fatal("stage cleanup not called")
			}
			entries, err := os.ReadDir(scratch)
			if err != nil || len(entries) != 0 {
				t.Fatalf("scratch leftovers: %v err=%v", entries, err)
			}
			if _, err := file.Stat(); err != nil {
				t.Fatalf("borrowed input closed: %v", err)
			}
		})
	}
}

func TestPortableExecutableCopyFailure(t *testing.T) {
	scratch := portableScratch(t)
	file, err := os.OpenFile(filepath.Join(t.TempDir(), "write-only"), os.O_CREATE|os.O_WRONLY, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close() //nolint:errcheck // Test input.
	if _, err := file.WriteString("unreadable"); err != nil {
		t.Fatal(err)
	}
	if cleanup, err := stageProcessExecutable(exec.CommandContext(t.Context(), file.Name()), file); err == nil || cleanup != nil {
		t.Fatalf("copy accepted: cleanup=%v err=%v", cleanup != nil, err)
	}
	entries, err := os.ReadDir(scratch)
	if err != nil || len(entries) != 0 {
		t.Fatalf("partial artifact leaked: %v err=%v", entries, err)
	}
}

func TestPortableExecutableNoDescriptor(t *testing.T) {
	scratch := portableScratch(t)
	cmd := exec.CommandContext(t.Context(), "/bin/sh", "-c", "exit 0")
	before := cmd.Path
	cleanup, err := stageProcessExecutable(cmd, nil)
	if err != nil || cleanup != nil || cmd.Path != before || len(cmd.ExtraFiles) != 0 {
		t.Fatalf("ordinary execution changed: %v", err)
	}
	entries, err := os.ReadDir(scratch)
	if err != nil || len(entries) != 0 {
		t.Fatalf("ordinary launch staged: %v err=%v", entries, err)
	}
}

func TestPortableNativeArgv(t *testing.T) {
	if os.Args[len(os.Args)-1] == "--portable-native-child" {
		fmt.Print(os.Args[0])
		os.Exit(0)
	}
	portableScratch(t)
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close() //nolint:errcheck // Borrowed test executable.
	var output bytes.Buffer
	argv0 := "/absent/original/session/ze"
	code, err := runProcess([]string{argv0, "-test.run=^TestPortableNativeArgv$", "--", "--portable-native-child"}, ProcessIO{Executable: file, Stdout: &output}, stageProcessExecutable)
	if code != 0 || err != nil || output.String() != argv0 {
		t.Fatalf("code=%d err=%v argv0=%q", code, err, output.String())
	}
}

// A cleanup failure must be returned, never expanded into recursive removal
// of another artifact that happens to occupy the stage's former name.
func TestPortableExecutableCleanupOwnFile(t *testing.T) {
	portableScratch(t)
	file, err := os.Open("/bin/sh")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close() //nolint:errcheck // Borrowed executable.
	cmd := exec.CommandContext(t.Context(), "/bin/sh", "-c", "exit 0")
	cleanup, err := stageProcessExecutable(cmd, file)
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(cmd.Path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(cmd.Path, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(cmd.Path, "other-artifact")
	if err := os.WriteFile(sentinel, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cleanup(); err == nil {
		t.Fatal("cleanup failure hidden")
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "preserve" {
		t.Fatalf("unrelated artifact removed: %q %v", data, err)
	}
}
