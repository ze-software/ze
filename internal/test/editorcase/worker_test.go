// Design: docs/architecture/testing/runner-architecture.md -- inherited editor IPC.

package editorcase

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	editortest "github.com/ze-software/ze/internal/component/cli/testing"
)

// TestEditorChildHelp checks that the forwarding registration's help forms
// answer before inspecting inherited descriptors.
func TestEditorChildHelp(t *testing.T) {
	for _, word := range []string{"help", "-h", "--help"} {
		if code := Main([]string{word}); code != 0 {
			t.Fatalf("%s: exit %d", word, code)
		}
	}
}

// TestEditorChildProtocol drives real files and a control pipe, including
// malformed request frames, authored parse failure and cooperative deadline.
func TestEditorChildProtocol(t *testing.T) {
	for _, name := range []string{"success", "parse", "expired", "wait", "stop", "missing", "partial", "trailing", "unknown", "no-content", "no-deadline", "relative-directory"} {
		t.Run(name, func(t *testing.T) {
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
			input, output, diagnostic := open("request"), open("result"), open("diagnostic")
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
			content := "option=mode:value=command\n"
			req := request{Content: &content, Deadline: time.Now().Add(5 * time.Second), Directory: directory}
			badRequest := false
			switch name {
			case "parse":
				content = "not-an-editor-directive\n"
			case "expired":
				req.Deadline = time.Now().Add(-time.Second)
			case "wait":
				content += "wait=ms:60000\n"
				req.Deadline = time.Now().Add(500 * time.Millisecond)
			case "stop":
				content += "wait=ms:60000\n"
				if _, err := parent.Write([]byte{1}); err != nil {
					t.Fatal(err)
				}
			case "no-content":
				req.Content = nil
				badRequest = true
			case "no-deadline":
				req.Deadline = time.Time{}
				badRequest = true
			case "relative-directory":
				req.Directory = "relative"
				badRequest = true
			}
			body, err := json.Marshal(req)
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "missing":
				body = nil
				badRequest = true
			case "partial":
				body = []byte(`{"content":`)
				badRequest = true
			case "trailing":
				body = append(body, []byte(` {}`)...)
				badRequest = true
			case "unknown":
				body = []byte(`{"unexpected":1}`)
				badRequest = true
			}
			if _, err := input.Write(body); err != nil {
				t.Fatal(err)
			}
			err = run(input, output, diagnostic, control)
			if badRequest {
				if err == nil {
					t.Fatal("accepted invalid IPC")
				}
				stat, statErr := output.Stat()
				if statErr != nil {
					t.Fatal(statErr)
				}
				if stat.Size() != 0 {
					t.Fatal("invalid request published a result")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var result *editortest.TestResult
			if err := readMessage(output, &result); err != nil {
				t.Fatal(err)
			}
			if result == nil {
				t.Fatal("missing result")
			}
			if result.Passed != (name == "success") {
				t.Fatalf("unexpected verdict: %+v", result)
			}
			if name == "wait" {
				if !strings.Contains(result.Error, "(wait)") {
					t.Fatalf("did not reach authored wait: %+v", result)
				}
			}
			if name != "success" && result.Error == "" {
				t.Fatal("failure lost its cause")
			}
			if name == "stop" {
				stat, err := diagnostic.Stat()
				if err != nil {
					t.Fatal(err)
				}
				if stat.Size() == 0 {
					t.Fatal("stop returned before diagnostics")
				}
			}
		})
	}
}
