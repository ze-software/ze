// Design: docs/architecture/config/syntax.md — ze config show tests

package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
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

// showSecretToken is the value the fixture stores on a real ze:sensitive leaf.
const showSecretToken = "show-stored-api-token" //nolint:gosec // fixture value, not a credential

// showSecretConfig carries one secret leaf and one ordinary leaf.
// environment.api-server.token is marked ze:sensitive in
// internal/component/api/yang/ze-api-conf.yang, so one render answers both
// questions: the secret is hidden and the rest of the tree still prints.
const showSecretConfig = `bgp {
	router-id 1.2.3.4;
}
environment {
	api-server {
		token ` + showSecretToken + `;
	}
}
`

// TestConfigShowMasksASecret verifies `ze config show` hides every secret leaf,
// in the text form and in the JSON form, whole-tree and at a path.
//
// VALIDATES: no `ze config show` shape renders a value the schema marks.
// PREVENTS: a shipped command printing an API token, a shared secret or a
// pre-shared key to stdout. The parser decodes a $9$ value into the tree, so
// this command published in cleartext what its sibling `ze config dump` writes
// encoded. The JSON shape is the same disclosure to anything that reads it.
func TestConfigShowMasksASecret(t *testing.T) {
	configPath := writeTestConfig(t, showSecretConfig)

	stored, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if !strings.Contains(string(stored), showSecretToken) {
		t.Fatalf("the fixture holds no secret, so this test would prove nothing:\n%s", stored)
	}

	cases := []struct {
		name string
		args []string
		keep string
	}{
		{"text whole tree", []string{configPath}, "router-id"},
		{"text at path", []string{configPath, "environment", "api-server"}, "token"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if rc := showConfig(&buf, storage.NewFilesystem(), tc.args); rc != exitOK {
				t.Fatalf("exit code = %d, want %d\n%s", rc, exitOK, buf.String())
			}
			out := buf.String()
			if !strings.Contains(out, tc.keep) {
				t.Fatalf("render carried no configuration, so this case would prove nothing:\n%s", out)
			}
			if strings.Contains(out, showSecretToken) {
				t.Fatalf("published the stored secret:\n%s", out)
			}
			if !strings.Contains(out, config.SecretDataPlaceholder) {
				t.Fatalf("rendered no placeholder:\n%s", out)
			}
		})
	}
}

// TestConfigShowUnparsableConfig verifies the two answer shapes agree about a
// configuration that parses nowhere.
//
// VALIDATES: the data form reports the parse failure rather than answering `{}`.
// PREVENTS: a silent empty answer. newEditor substitutes an empty tree when
// nothing parses, so the data form said the configuration held nothing while
// the text form printed the file. The text form keeps that fallback for the
// whole configuration, because the operator must read the broken line to
// repair it.
func TestConfigShowUnparsableConfig(t *testing.T) {
	broken := "bgp {\n    router-id 1.2.3.4;\n"
	configPath := writeTestConfig(t, broken)

	var textOut bytes.Buffer
	if rc := showConfig(&textOut, storage.NewFilesystem(), []string{configPath}); rc != exitOK {
		t.Fatalf("text whole-tree exit = %d, want %d", rc, exitOK)
	}
	if !strings.Contains(textOut.String(), "router-id") {
		t.Fatalf("the text form must answer the file so the operator can repair it:\n%s", textOut.String())
	}

	var buf bytes.Buffer
	if rc := showConfig(&buf, storage.NewFilesystem(), []string{configPath, "bgp"}); rc == exitOK {
		t.Fatalf("the text form exited 0 at a path of a configuration that does not parse:\n%s", buf.String())
	}
	if buf.Len() != 0 {
		t.Fatalf("the text form wrote output for a configuration that does not parse:\n%s", buf.String())
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
