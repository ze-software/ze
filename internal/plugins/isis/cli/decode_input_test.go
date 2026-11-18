// Design: docs/architecture/wire/isis.md -- offline decoder input contract.
package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

// TestISISDecodeReadsPathBeforeStdin gives the command a valid file and
// invalid stdin, then judges the decoded packet. Ignoring the path cannot pass.
func TestISISDecodeReadsPathBeforeStdin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "packet.hex")
	if err := os.WriteFile(path, []byte(buildLSPHex(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{path}, {"--pretty", path}} {
		output, err := runISISDecodeInput(t, args, strings.NewReader("invalid stdin"))
		if err != nil {
			t.Fatalf("decode %v: %v\n%s", args, err, output)
		}
		assertISISDecodeInput(t, output)
	}
}

// TestISISDecodeExplicitStdin preserves the conventional dash input,
// including pretty output, while file input is added.
func TestISISDecodeExplicitStdin(t *testing.T) {
	for _, args := range [][]string{{"-"}, {"--pretty", "-"}} {
		output, err := runISISDecodeInput(t, args, strings.NewReader(buildLSPHex(t)))
		if err != nil {
			t.Fatalf("decode %v: %v\n%s", args, err, output)
		}
		assertISISDecodeInput(t, output)
	}
}

// TestISISDecodePipedStdin preserves the input form used by the native
// wire fixtures: no operand with a real pipe still decodes a packet.
func TestISISDecodePipedStdin(t *testing.T) {
	output, err := runISISDecodeInput(t, nil, strings.NewReader(buildLSPHex(t)))
	if err != nil {
		t.Fatalf("decode piped stdin: %v\n%s", err, output)
	}
	assertISISDecodeInput(t, output)
}

// TestISISDecodeInputRefusals pairs path selection with invalid, missing
// and oversized inputs. A valid stdin packet cannot rescue a refused path.
func TestISISDecodeInputRefusals(t *testing.T) {
	tests := []struct {
		name string
		body string
		file bool
		args []string
		want string
	}{
		{name: "missing file", args: []string{filepath.Join(t.TempDir(), "missing.hex")}, want: "error:"},
		{name: "extra operands", args: []string{"-", "extra.hex"}, want: "usage:"},
		{name: "invalid file", body: "invalid packet", file: true, want: "error:"},
		{name: "oversized file", body: strings.Repeat("0", maxStdinBytes+1), file: true, want: "input exceeds"},
		{name: "invalid stdin", body: "invalid packet", args: []string{"-"}, want: "error:"},
		{name: "oversized stdin", body: strings.Repeat("0", maxStdinBytes+1), args: []string{"-"}, want: "input exceeds"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := tc.args
			input := tc.body
			if tc.file {
				path := filepath.Join(t.TempDir(), "packet.hex")
				if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
					t.Fatal(err)
				}
				args = []string{path}
				input = buildLSPHex(t)
			}
			if tc.body == "" {
				input = buildLSPHex(t)
			}
			output, err := runISISDecodeInput(t, args, strings.NewReader(input))
			if err == nil {
				t.Fatalf("decode accepted refused input: %s", output)
			}
			if !strings.Contains(string(output), tc.want) {
				t.Fatalf("refusal %q does not contain %q", output, tc.want)
			}
		})
	}
}

// TestISISDecodeTerminalWithoutInputRefuses attaches a real terminal and
// supplies no bytes. The subprocess deadline kills only this test's child if
// the command blocks; a successful refusal must itself return usage.
func TestISISDecodeTerminalWithoutInputRefuses(t *testing.T) {
	master, terminal, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := terminal.Close(); err != nil {
			t.Error(err)
		}
		if err := master.Close(); err != nil {
			t.Error(err)
		}
	})
	output, err := runISISDecodeInput(t, nil, terminal)
	if err == nil {
		t.Fatalf("terminal input was accepted: %s", output)
	}
	if !strings.Contains(string(output), "usage: ze isis decode") {
		t.Fatalf("terminal refusal contains no usage: %s", output)
	}
}

// TestISISDecodeInputProcess is the bounded subprocess entry point, so
// tests exercise cmdDecode's real process streams without rebinding globals.
func TestISISDecodeInputProcess(t *testing.T) {
	if os.Getenv("ZE_ISIS_DECODE_INPUT_CHILD") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Exit(cmdDecode(os.Args[i+1:]))
		}
	}
	t.Fatal("decode subprocess has no argument boundary")
}

func runISISDecodeInput(t *testing.T, args []string, input io.Reader) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	childArgs := append([]string{"-test.run=^TestISISDecodeInputProcess$", "--"}, args...)
	child := exec.CommandContext(ctx, os.Args[0], childArgs...)
	child.Env = append(os.Environ(), "ZE_ISIS_DECODE_INPUT_CHILD=1")
	child.Stdin = input
	output, err := child.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("decode blocked waiting for input: %v\n%s", ctx.Err(), output)
	}
	return output, err
}

func assertISISDecodeInput(t *testing.T, output []byte) {
	t.Helper()
	var decoded struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatalf("decode output is not JSON: %v\n%s", err, output)
	}
	if decoded.Type != "l2-lsp" {
		t.Fatalf("decoded type = %q, want l2-lsp", decoded.Type)
	}
}
