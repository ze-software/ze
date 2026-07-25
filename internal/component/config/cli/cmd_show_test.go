// Design: docs/architecture/config/syntax.md — ze config show tests

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
)

const showTestConfig = `bgp {
	session {
		asn {
			local 65000;
		}
	}
	router-id 1.2.3.4;
	peer peer1 {
		connection {
			remote {
				ip 1.1.1.1;
			}
		}
		session {
			asn {
				remote 65001;
			}
		}
	}
}
`

// TestConfigShowFullTree verifies `ze config show <file>` (no path) prints the
// whole parsed tree. VALIDATES AC-9 (one-shot config inspection).
func TestConfigShowFullTree(t *testing.T) {
	configPath := writeTestConfig(t, showTestConfig)
	var buf bytes.Buffer
	rc := showConfig(&buf, storage.NewFilesystem(), []string{configPath})
	if rc != exitOK {
		t.Fatalf("exit code = %d, want %d\nstdout:\n%s", rc, exitOK, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "router-id") || !strings.Contains(out, "peer") {
		t.Fatalf("full tree missing expected content:\n%s", out)
	}
}

// TestConfigShowAtPath verifies path-scoped output. A nested list-keyed path
// (bgp peer peer1) resolves and prints only that subtree.
func TestConfigShowAtPath(t *testing.T) {
	configPath := writeTestConfig(t, showTestConfig)
	var buf bytes.Buffer
	rc := showConfig(&buf, storage.NewFilesystem(), []string{configPath, "bgp", "peer", "peer1"})
	if rc != exitOK {
		t.Fatalf("exit code = %d, want %d", rc, exitOK)
	}
	out := buf.String()
	if !strings.Contains(out, "1.1.1.1") {
		t.Fatalf("subtree missing peer remote ip:\n%s", out)
	}
	// The scoped view must NOT include a sibling outside the path.
	if strings.Contains(out, "router-id") {
		t.Fatalf("subtree leaked a sibling (router-id) outside the path:\n%s", out)
	}
}

// TestConfigShowPathNotFound verifies a missing path exits non-zero with no
// output (not a silent fall-back to the whole tree).
func TestConfigShowPathNotFound(t *testing.T) {
	configPath := writeTestConfig(t, showTestConfig)
	var buf bytes.Buffer
	rc := showConfig(&buf, storage.NewFilesystem(), []string{configPath, "no", "such", "path"})
	if rc == exitOK {
		t.Fatalf("expected non-zero exit for missing path")
	}
	if buf.Len() != 0 {
		t.Fatalf("missing path produced output:\n%s", buf.String())
	}
}

// TestConfigShowJSON verifies `--json` emits a JSON object for the subtree.
func TestConfigShowJSON(t *testing.T) {
	configPath := writeTestConfig(t, showTestConfig)
	var buf bytes.Buffer
	rc := showConfig(&buf, storage.NewFilesystem(), []string{"--json", configPath, "bgp"})
	if rc != exitOK {
		t.Fatalf("exit code = %d, want %d", rc, exitOK)
	}
	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if _, ok := decoded["peer"]; !ok {
		t.Fatalf("JSON subtree missing 'peer' key: %v", decoded)
	}
}

// TestConfigShowMissingFile verifies the usage check for a missing file arg.
func TestConfigShowMissingFile(t *testing.T) {
	var buf bytes.Buffer
	rc := showConfig(&buf, storage.NewFilesystem(), []string{})
	if rc == exitOK {
		t.Fatalf("expected non-zero exit when file arg missing")
	}
}

// TestConfigShowRegistered verifies the subcommand is wired into dispatch.
func TestConfigShowRegistered(t *testing.T) {
	if _, ok := subcommandHandlers["show"]; !ok {
		t.Fatalf("`show` not registered in subcommandHandlers")
	}
}

// TestShowConfigDash verifies `ze config show -` reads the config from stdin and
// produces output byte-identical to the on-disk invocation, both for the whole
// tree (AC-1) and for a path scoped after "-" (AC-2).
func TestShowConfigDash(t *testing.T) {
	configPath := writeTestConfig(t, showTestConfig)

	run := func(args []string, stdin string) (int, string) {
		restore := cliio.SwapStreams(strings.NewReader(stdin), &bytes.Buffer{})
		defer restore()
		var buf bytes.Buffer
		rc := showConfig(&buf, storage.NewFilesystem(), args)
		return rc, buf.String()
	}

	// AC-1: whole tree from stdin == whole tree from disk.
	rcDisk, wantWhole := run([]string{configPath}, "")
	if rcDisk != exitOK {
		t.Fatalf("disk whole-tree exit = %d", rcDisk)
	}
	rcStdin, gotWhole := run([]string{"-"}, showTestConfig)
	if rcStdin != exitOK {
		t.Fatalf("stdin whole-tree exit = %d\n%s", rcStdin, gotWhole)
	}
	if gotWhole != wantWhole {
		t.Fatalf("stdin whole tree != disk whole tree\nstdin:\n%s\ndisk:\n%s", gotWhole, wantWhole)
	}

	// AC-2: a path after "-" does not disturb positional parsing.
	_, wantSub := run([]string{configPath, "bgp", "peer", "peer1"}, "")
	rcSub, gotSub := run([]string{"-", "bgp", "peer", "peer1"}, showTestConfig)
	if rcSub != exitOK {
		t.Fatalf("stdin subtree exit = %d\n%s", rcSub, gotSub)
	}
	if gotSub != wantSub {
		t.Fatalf("stdin subtree != disk subtree\nstdin:\n%s\ndisk:\n%s", gotSub, wantSub)
	}
	if strings.Contains(gotSub, "router-id") {
		t.Fatalf("subtree from stdin leaked a sibling:\n%s", gotSub)
	}
}
