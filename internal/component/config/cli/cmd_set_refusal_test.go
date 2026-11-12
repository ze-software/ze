// Design: docs/guide/config-editor.md -- offline value refusals
// Related: cmd_set.go -- cmdSetImpl frames the command the operator typed

package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestCmdSetValueRefusal checks the offline writer's complete refusal and
// unchanged file, through the same command an operator runs (AC-6, AC-13).
func TestCmdSetValueRefusal(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		problem string
		why     []string
		fix     string
	}{
		{
			name:    "enum",
			args:    []string{"bgp", "peer", "peer1", "session", "capability", "route-refresh", "requre"},
			problem: `bgp peer peer1 session capability route-refresh: "requre" is not an accepted value (did you mean require?)`,
			why: []string{
				"route-refresh accepts only these values:",
				"  enable   Advertise the capability; no enforcement",
				"  disable  Do not advertise the capability; no enforcement",
				"  require  Advertise the capability; refuse the session when the peer does not offer it",
				"  refuse   Do not advertise the capability; refuse the session when the peer offers it",
			},
			fix: "set bgp peer peer1 session capability route-refresh require",
		},
		{
			name:    "union",
			args:    []string{"bgp", "peer", "peer1", "connection", "local", "ip", "autoo"},
			problem: `bgp peer peer1 connection local ip: "autoo" is not an accepted value (did you mean auto?)`,
			why: []string{
				"ip accepts one of these values:",
				"  auto  Let the operating system choose the source address of the connection.",
				"or a value of type ip-address",
			},
			fix: "set bgp peer peer1 connection local ip auto",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := writeTestConfig(t, setSecretConfig)
			before, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			code, stderr := captureStderr(t, func() int {
				return cmdSet(append([]string{file}, tc.args...))
			})
			if code != exitError {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, exitError, stderr)
			}
			want := "error: " + tc.problem + "\n" + strings.Join(tc.why, "\n") + "\nfix: " + tc.fix + "\n"
			if stderr != want {
				t.Errorf("refusal:\ngot:\n%s\nwant:\n%s", stderr, want)
			}
			after, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, before) {
				t.Errorf("refusal changed the file:\n%s", after)
			}
		})
	}
}

// TestCmdSetKeyRefusalKeepsTheWholeCommand checks that a new enum key is
// refused with the rest of the typed command in the fix (AC-28).
func TestCmdSetKeyRefusalKeepsTheWholeCommand(t *testing.T) {
	file := writeTestConfig(t, setSecretConfig)
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	code, stderr := captureStderr(t, func() int {
		return cmdSet([]string{file, "ospf", "redistribute", "bogus", "nssa-propagate", "true"})
	})
	if code != exitError {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, exitError, stderr)
	}
	want := "error: ospf redistribute bogus: \"bogus\" is not an accepted value\n" +
		"source accepts only these values:\n" +
		"  connected  Routes of the addresses configured on local interfaces.\n" +
		"  static     Routes the static plugin installs.\n" +
		"  kernel     Routes read from the kernel routing table.\n" +
		"  bgp        Routes learned by BGP.\n" +
		"  isis       Routes learned by IS-IS.\n" +
		"fix: set ospf redistribute <connected|static|kernel|bgp|isis> nssa-propagate true\n"
	if stderr != want {
		t.Errorf("refusal:\ngot:\n%s\nwant:\n%s", stderr, want)
	}
	after, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Errorf("refusal changed the file:\n%s", after)
	}
}

// TestCmdSetKeyRefusalMasksSensitiveSuffix protects the typed writer's full
// fix command when a refused ancestor key precedes a secret assignment.
func TestCmdSetKeyRefusalMasksSensitiveSuffix(t *testing.T) {
	t.Cleanup(configyang.RegisterModuleForTest("ze-offline-secret-probe-conf.yang", `module ze-offline-secret-probe-conf {
  yang-version 1.1;
  namespace "urn:ze:offline-secret-probe";
  prefix probe;
  import ze-extensions { prefix ze; }
  container offline-secret-probe {
    list lane {
      key "kind";
      leaf kind { type enumeration { enum blue; enum red; } }
      leaf credential { type string { length "1..max"; } ze:sensitive; }
    }
  }
}`))
	file := writeTestConfig(t, setSecretConfig)
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	const secret = "offline-secret-c5f98d2-tail"
	code, stderr := captureStderr(t, func() int {
		return cmdSet([]string{file, "offline-secret-probe", "lane", "bogus", "credential", secret})
	})
	if code != exitError {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, exitError, stderr)
	}
	if !strings.HasPrefix(stderr, `error: offline-secret-probe lane bogus: "bogus" is not an accepted value`) {
		t.Errorf("wrong refusal: %s", stderr)
	}
	wantFix := "\nfix: set offline-secret-probe lane <blue|red> credential " + config.SecretDataPlaceholder + "\n"
	if !strings.HasSuffix(stderr, wantFix) {
		t.Errorf("missing masked whole-command fix %q: %s", wantFix, stderr)
	}
	if strings.Contains(stderr, "c5f98d2-tail") {
		t.Errorf("refusal published the secret's tail: %s", stderr)
	}
	after, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Errorf("refusal changed the file:\n%s", after)
	}
}

// TestCmdSetAcceptsPrefixKeys proves both union prefix-key members still
// reach the entry creator through the offline writer (AC-32).
func TestCmdSetAcceptsPrefixKeys(t *testing.T) {
	for _, prefix := range []string{"10.0.0.0/8", "2001:db8::/32"} {
		t.Run(prefix, func(t *testing.T) {
			file := writeTestConfig(t, setSecretConfig)
			code, stderr := captureStderr(t, func() int {
				return cmdSet([]string{file, "policy", "prefix-list", "sample", "entry", prefix, "action", "accept"})
			})
			if code != exitOK {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, exitOK, stderr)
			}
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := config.YANGSchema()
			if err != nil {
				t.Fatal(err)
			}
			tree, err := config.NewParser(schema).Parse(string(data))
			if err != nil {
				t.Fatal(err)
			}
			policy := tree.GetContainer("policy")
			if policy == nil {
				t.Fatal("no policy container was created")
			}
			list := policy.GetList("prefix-list")["sample"]
			if list == nil {
				t.Fatal("no prefix-list entry was created")
			}
			entry := list.GetList("entry")[prefix]
			if entry == nil {
				t.Fatalf("no entry with key %q was created", prefix)
			}
			action, ok := entry.Get("action")
			if !ok {
				t.Fatal("the created entry has no action")
			}
			if action != "accept" {
				t.Errorf("action = %q, want accept", action)
			}
		})
	}
}
