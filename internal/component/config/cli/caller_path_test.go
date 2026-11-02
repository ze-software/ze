// Design: docs/architecture/api/commands.md — path arguments and stdin on the daemon route

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	_ "github.com/ze-software/ze/internal/plugins/config-cli/yang" // Register ze-config-cli-cmd, the model the routes judge against.
)

// testConfigPlain is a configuration with no secret value in it.
const testConfigPlain = `
bgp {
    router-id 1.1.1.1;
    session {
        asn {
            local 65000;
        }
    }
    peer peer1 {
        connection {
            remote {
                ip 10.0.0.1;
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

// confinedConfigDir points the configuration search directory at a fresh
// directory, writes content there as router.conf and answers its path. The
// daemon route reads only inside that directory (callerPathDirs).
func confinedConfigDir(t *testing.T, content string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	dir := filepath.Join(home, "ze")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "router.conf")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// answerOn validates tokens against the command's model, as every route does,
// and runs the registered handler on the local or the daemon route.
func answerOn(t *testing.T, path string, tokens []string, remote bool) command.LocalAnswer {
	t.Helper()
	handler, rest := command.LookupLocalData(strings.Fields(path))
	if handler == nil || len(rest) != 0 {
		t.Fatalf("%s has no local-data handler", path)
	}
	validated, err := command.ValidateModelArgs(path, tokens, nil)
	if err != nil {
		t.Fatalf("the model refused %s %q: %v", path, tokens, err)
	}
	if remote {
		return command.AnswerRemote(handler, validated)
	}
	return command.AnswerLocal(handler, validated)
}

// encodedAnswer is the answer as a caller compares it: code, payload JSON and
// diagnostic.
func encodedAnswer(t *testing.T, answer command.LocalAnswer) string {
	t.Helper()
	payload, err := json.Marshal(answer.Payload)
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}
	return strings.Join([]string{strconv.Itoa(answer.Code), string(payload), answer.Diagnostic}, "\n")
}

// TestConfigCommandsPathConfinedOnDaemonRoute proves every daemon-served
// config command that reads a caller-named file confines it.
//
// Goal: on the daemon route a file inside the configuration directory answers
// exactly what the operator's own `ze` answers, and `/etc/passwd`, a `..`
// escape and `-` are refused with the local command to run.
// Method: each registered handler runs on both routes through the model
// validator, with the search directory pointed at a test directory.
func TestConfigCommandsPathConfinedOnDaemonRoute(t *testing.T) {
	// No secret: a masked secret is re-encrypted with a fresh salt on each
	// answer, so two answers would differ while both are right.
	file := confinedConfigDir(t, testConfigPlain)
	escape := filepath.Join(filepath.Dir(file), "..", "..", "..", "etc", "passwd")

	for _, path := range []string{
		"show config dump", "show config validation", "show config graph", "show config fmt",
	} {
		t.Run(path, func(t *testing.T) {
			local := answerOn(t, path, []string{file}, false)
			remote := answerOn(t, path, []string{file}, true)
			if encodedAnswer(t, remote) != encodedAnswer(t, local) {
				t.Errorf("inside the config directory the daemon route answered\n%s\nthe local route\n%s",
					encodedAnswer(t, remote), encodedAnswer(t, local))
			}
			for _, target := range []string{"/etc/passwd", escape, "-"} {
				refusal := answerOn(t, path, []string{target}, true)
				if refusal.Code == 0 || refusal.Payload != nil {
					t.Errorf("daemon route read %q: code %d payload %v", target, refusal.Code, refusal.Payload)
					continue
				}
				want := "run 'ze " + path + " " + target + "' on the host that holds the file"
				if !strings.Contains(refusal.Diagnostic, want) {
					t.Errorf("refusal of %q is %q, want it to name %q", target, refusal.Diagnostic, want)
				}
			}
		})
	}

	t.Run("show config diff", func(t *testing.T) {
		tokens := []string{file, file}
		local := answerOn(t, "show config diff", tokens, false)
		remote := answerOn(t, "show config diff", tokens, true)
		if encodedAnswer(t, remote) != encodedAnswer(t, local) {
			t.Errorf("diff inside the config directory differs by route:\n%s\n%s",
				encodedAnswer(t, remote), encodedAnswer(t, local))
		}
		refusal := answerOn(t, "show config diff", []string{file, "/etc/passwd"}, true)
		if refusal.Code == 0 || !strings.Contains(refusal.Diagnostic, "on the host that holds the file") {
			t.Errorf("diff against /etc/passwd on the daemon route: code %d, %q", refusal.Code, refusal.Diagnostic)
		}
	})

	t.Run("show config history", func(t *testing.T) {
		refusal := answerOn(t, "show config history", []string{"/etc/passwd"}, true)
		if refusal.Code == 0 || !strings.Contains(refusal.Diagnostic, "on the host that holds the file") {
			t.Errorf("history beside /etc/passwd on the daemon route: code %d, %q", refusal.Code, refusal.Diagnostic)
		}
	})
}

// TestShowConfigFmtWriteRefusedOnDaemonRoute proves `fmt -w` never rewrites a
// file for a caller on another surface, even one inside the configuration
// directory, and names the local command that does.
func TestShowConfigFmtWriteRefusedOnDaemonRoute(t *testing.T) {
	file := confinedConfigDir(t, "system { host-name   a; }\n")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	refusal := answerOn(t, "show config fmt", []string{"-w", file}, true)
	if refusal.Code == 0 {
		t.Fatalf("fmt -w on the daemon route succeeded: %q", refusal.Diagnostic)
	}
	want := "run 'ze show config fmt -w " + file + "' on the host that holds the file"
	if !strings.Contains(refusal.Diagnostic, want) {
		t.Errorf("refusal %q does not name %q", refusal.Diagnostic, want)
	}
	after, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Errorf("fmt -w on the daemon route rewrote the file:\n%s", after)
	}
}

// TestShowConfigDumpStripPrivateLeaf proves the declared leaf asks for what
// the local flag asks for.
//
// Goal: `strip-private`, which every surface can send, and the local
// `--strip-private` both pass the model and answer the masked tree, on both
// routes; without either the placeholder is absent.
// Method: each spelling through command.ValidateModelArgs and the registered
// handler, comparing the JSON payloads.
func TestShowConfigDumpStripPrivateLeaf(t *testing.T) {
	file := confinedConfigDir(t, testConfigSecretBase)

	masked := encodedAnswer(t, answerOn(t, "show config dump", []string{"--strip-private", file}, false))
	if !strings.Contains(masked, config.SecretDataPlaceholder) {
		t.Fatalf("--strip-private answers no placeholder, so this test proves nothing: %s", masked)
	}
	for _, tc := range []struct {
		tokens []string
		remote bool
	}{
		{[]string{"strip-private", file}, false},
		{[]string{file, "strip-private"}, false},
		{[]string{"strip-private", file}, true},
	} {
		got := encodedAnswer(t, answerOn(t, "show config dump", tc.tokens, tc.remote))
		if got != masked {
			t.Errorf("show config dump %q (remote %v) answered\n%s\nwant\n%s", tc.tokens, tc.remote, got, masked)
		}
	}
	plain := encodedAnswer(t, answerOn(t, "show config dump", []string{file}, false))
	if strings.Contains(plain, config.SecretDataPlaceholder) {
		t.Errorf("show config dump without the leaf masked the tree: %s", plain)
	}
}

// TestPluginDeclarationsConfigPathConfined proves `show plugin declarations
// config <file>` confines its file the same way, judged before the file is
// looked at: inside the configuration directory both routes answer alike,
// and `/etc/passwd` is refused on the daemon route with the local command.
func TestPluginDeclarationsConfigPathConfined(t *testing.T) {
	file := confinedConfigDir(t, testConfigPlain)
	const path = "show plugin declarations config"

	local := encodedAnswer(t, answerOn(t, path, []string{file}, false))
	remote := encodedAnswer(t, answerOn(t, path, []string{file}, true))
	if remote != local {
		t.Errorf("inside the config directory the daemon route answered\n%s\nthe local route\n%s", remote, local)
	}
	refusal := answerOn(t, path, []string{"/etc/passwd"}, true)
	want := "run 'ze " + path + " /etc/passwd' on the host that holds the file"
	if refusal.Code == 0 || !strings.Contains(refusal.Diagnostic, want) {
		t.Errorf("daemon route on /etc/passwd: code %d, %q, want it to name %q", refusal.Code, refusal.Diagnostic, want)
	}
}

// TestConfigIncludeReadsNothingOutsideTheConfinedDir proves a confined file
// cannot pull a file from outside the configuration directories into a
// daemon-route answer.
//
// Goal: AC-8. The file the caller names is confined (command.CallerPath), so
// the only other way to reach a file is a statement inside it that names one.
// Ze's configuration grammar has no include statement: a file that writes one
// is answered from its own bytes alone, whether the statement names the file
// outside directly or through a symlink placed inside the directory.
// Method: every daemon-served command that parses a caller-named config runs
// on the daemon route over a confined file whose include names a marked file
// outside the directory, `/etc/passwd`, and a symlink to each; no answer may
// carry a byte of either target.
func TestConfigIncludeReadsNothingOutsideTheConfinedDir(t *testing.T) {
	const marker = "zesecretmarker"
	outside := filepath.Join(t.TempDir(), "secret.conf")
	secret := "# " + marker + "\nbgp {\n    peer " + marker + " {\n        connection {\n            remote {\n                ip 10.9.9.9;\n            }\n        }\n    }\n}\n"
	if err := os.WriteFile(outside, []byte(secret), 0o600); err != nil {
		t.Fatal(err)
	}
	forbidden := []string{marker, "10.9.9.9"}
	if passwd, err := os.ReadFile("/etc/passwd"); err == nil {
		if first, _, _ := strings.Cut(string(passwd), "\n"); first != "" {
			forbidden = append(forbidden, first)
		}
	}

	file := confinedConfigDir(t, testConfigPlain)
	dir := filepath.Dir(file)
	for name, target := range map[string]string{"secret-link": outside, "passwd-link": "/etc/passwd"} {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}

	for _, include := range []string{outside, "/etc/passwd", filepath.Join(dir, "secret-link"), "passwd-link"} {
		content := "include \"" + include + "\";\n" + testConfigPlain
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		for path, tokens := range map[string][]string{
			"show config dump":                {file},
			"show config validation":          {file},
			"show config graph":               {file},
			"show config fmt":                 {file},
			"show config diff":                {file, file},
			"show plugin declarations config": {file},
		} {
			answer := answerOn(t, path, tokens, true)
			seen := encodedAnswer(t, answer)
			if blob, ok := answer.Payload.(command.Blob); ok {
				seen += "\n" + string(blob)
			}
			for _, bytesOutside := range forbidden {
				if strings.Contains(seen, bytesOutside) {
					t.Errorf("%s over a file including %q answered %q from outside the configuration directory:\n%s",
						path, include, bytesOutside, seen)
				}
			}
		}
	}
}
