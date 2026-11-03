// Design: docs/architecture/testing/runner-architecture.md -- registered editor child.

package editorcase

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	editortest "github.com/ze-software/ze/internal/component/cli/testing"
	"github.com/ze-software/ze/internal/component/plugin"
)

var nativeExecutable = flag.String("editor-child", "", "absolute path to the named le build for the native editor child proof")

// TestEditorChildNativeEntry proves the registered command consumes authored
// content through inherited FDs and executes real editor assertions.
func TestEditorChildNativeEntry(t *testing.T) {
	directory := t.TempDir()
	open := func(name string) *os.File {
		file, err := os.Create(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := file.Close(); err != nil {
				t.Error(err)
			}
		})
		return file
	}
	input, output, diagnostic, log := open("request"), open("result"), open("diagnostic"), open("log")
	executable := *nativeExecutable
	arguments := []string{"mock", "editor-case"}
	if executable == "" {
		// Standalone package runs build their own named artifact. A targeted
		// invocation may supply the named build its admission already built.
		root, err := filepath.Abs("../../..")
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()
		build := exec.CommandContext(ctx, filepath.Join(root, "le"), "--name", "editor-child-native-proof", "help")
		build.Dir = root
		build.Env = append(os.Environ(), "CGO_ENABLED=0")
		build.Stdout, build.Stderr = log, log
		plugin.KillGroupOnCancel(build)
		if err := plugin.StartChild(build); err != nil {
			t.Fatal(err)
		}
		if err := build.Wait(); err != nil {
			t.Fatalf("named native fixture build: %v; log %s", err, log.Name())
		}
		executable = filepath.Join(root, "le")
		arguments = []string{"--name", "editor-child-native-proof", "mock", "editor-case"}
	}
	if !filepath.IsAbs(executable) {
		t.Fatal("-editor-child must name an absolute path")
	}
	control, parent, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := control.Close(); err != nil {
			t.Error(err)
		}
		if err := parent.Close(); err != nil {
			t.Error(err)
		}
	})
	content := "tmpfs=test.conf:terminator=END\nbgp { router-id 1.2.3.4; }\nEND\noption=file:path=test.conf\ninput=type:text=set bgp router-id 5.6.7.8\ninput=enter\nexpect=dirty:true\n"
	req := request{Content: &content, Deadline: time.Now().Add(10 * time.Second), Directory: directory}
	if err := json.NewEncoder(input).Encode(req); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, arguments...)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	plugin.KillGroupOnCancel(cmd)
	cmd.WaitDelay = 2 * time.Second
	cmd.ExtraFiles = []*os.File{input, output, diagnostic, control}
	cmd.Stdout, cmd.Stderr = log, log
	if err := plugin.StartChild(cmd); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		body, readErr := os.ReadFile(log.Name())
		if readErr != nil {
			t.Fatal(readErr)
		}
		t.Fatalf("native child: %v\n%s", err, body)
	}
	var result editortest.TestResult
	if err := readMessage(output, &result); err != nil {
		t.Fatal(err)
	}
	if !result.Passed {
		t.Fatalf("native authored run failed: %+v", result)
	}
	if len(result.Steps) != 3 {
		t.Fatalf("authored steps did not execute: %+v", result)
	}
	if result.TempDir != directory {
		t.Fatalf("child falsely reported parent resources cleaned: %+v", result)
	}
	if _, err := os.Stat(directory); err != nil {
		t.Fatalf("child removed parent-owned directory: %v", err)
	}
}
