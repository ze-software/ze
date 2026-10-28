package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConfigFmtSetInputRefused runs `ze config fmt` on a set-form and a
// set-meta-form file through cmdFmt, the function an operator reaches.
//
// VALIDATES: AC-12: exit 2, the refusal names the form the file is in and the
// command that converts it, with the operator's own path in that command, and
// the file is left untouched by -w.
// PREVENTS: the hierarchical parser's "unknown top-level keyword: set", which
// names neither the form nor the fix.
func TestConfigFmtSetInputRefused(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		formName string
	}{
		{"set", "set bgp router-id 1.2.3.4\n", "set form"},
		{"set-meta", "#operator set bgp router-id 1.2.3.4\n", "set-meta form"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "stored.conf")
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}

			for _, args := range [][]string{{path}, {"-w", path}, {"--check", path}} {
				rc, stderr := captureStderr(t, func() int { return cmdFmt(args) })
				if rc != exitError {
					t.Errorf("%v: exit %d, want %d; stderr:\n%s", args, rc, exitError, stderr)
				}
				if !strings.Contains(stderr, tc.formName) {
					t.Errorf("%v: stderr does not name the %s:\n%s", args, tc.formName, stderr)
				}
				fix := "ze config migrate format hierarchical " + path
				if !strings.Contains(stderr, fix) {
					t.Errorf("%v: stderr does not give the fix %q:\n%s", args, fix, stderr)
				}
				if strings.Contains(stderr, "unknown top-level keyword") {
					t.Errorf("%v: stderr still carries the parser refusal:\n%s", args, stderr)
				}
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != tc.content {
				t.Errorf("file changed by a refused fmt:\n%s", data)
			}
		})
	}
}

// TestConfigFmtKeepsUserOrderedEntries formats a config through configFmtBytes,
// the whole of `ze config fmt`, with the terms of a firewall chain (YANG
// `ordered-by user`) written out of key order, then formats the output again.
// The firewall needs no feature tag, so the test runs in every build.
//
// VALIDATES: the terms print in the operator's order, which holds only when the
// schema marks the list UserOrdered from its YANG, and the second fmt changes
// nothing.
// PREVENTS: fmt sorting the entries by key, which reorders a first-match list:
// `term allow-ssh` would move ahead of `term drop-all` and the chain would
// match a different term first.
func TestConfigFmtKeepsUserOrderedEntries(t *testing.T) {
	input := `firewall {
	table wan {
		family inet
		chain input {
			type filter
			hook input
			priority 0
			policy accept
			term drop-all {
				then {
					drop
				}
			}
			term allow-ssh {
				from {
					destination-port 22
				}
				then {
					accept
				}
			}
		}
	}
}
`
	formatted, _, err := configFmtBytes("test.conf", []byte(input))
	if err != nil {
		t.Fatalf("fmt: %v", err)
	}
	first, second := strings.Index(formatted, "term drop-all"), strings.Index(formatted, "term allow-ssh")
	if first < 0 {
		t.Fatalf("output lacks term drop-all:\n%s", formatted)
	}
	if second < 0 {
		t.Fatalf("output lacks term allow-ssh:\n%s", formatted)
	}
	if first > second {
		t.Errorf("fmt moved term allow-ssh ahead of term drop-all:\n%s", formatted)
	}

	again, changed, err := configFmtBytes("test.conf", []byte(formatted))
	if err != nil {
		t.Fatalf("second fmt: %v", err)
	}
	if changed {
		t.Errorf("second fmt changed the output:\n%s\nvs\n%s", formatted, again)
	}
}

// operatorOrderPeers is a BGP config whose peers are written zulu, alpha,
// mike: an order the operator chose, and not key order.
const operatorOrderPeers = `bgp {
	router-id 10.0.0.1
	session {
		asn {
			local 65000
		}
	}
	peer zulu {
		connection {
			remote {
				ip 192.0.2.26
			}
		}
		session {
			asn {
				remote 65026
			}
		}
	}
	peer alpha {
		connection {
			remote {
				ip 192.0.2.1
			}
		}
		session {
			asn {
				remote 65001
			}
		}
	}
	peer mike {
		connection {
			remote {
				ip 192.0.2.13
			}
		}
		session {
			asn {
				remote 65013
			}
		}
	}
}
`

// TestConfigFmtKeepsOperatorOrderedEntries proves AC-1 through the function
// `ze config fmt` runs: a list that is not `ordered-by user` keeps the entry
// order the operator wrote, and formatting the output again changes nothing.
// The method formats operatorOrderPeers and compares where each peer prints.
func TestConfigFmtKeepsOperatorOrderedEntries(t *testing.T) {
	formatted, _, err := configFmtBytes("test.conf", []byte(operatorOrderPeers))
	if err != nil {
		t.Fatalf("fmt: %v", err)
	}
	names := []string{"peer zulu", "peer alpha", "peer mike"}
	positions := make([]int, len(names))
	for i, name := range names {
		positions[i] = strings.Index(formatted, name)
		if positions[i] < 0 {
			t.Fatalf("output lacks %s:\n%s", name, formatted)
		}
	}
	for i := 1; i < len(names); i++ {
		if positions[i-1] > positions[i] {
			t.Errorf("fmt moved %s ahead of %s, want the order written (zulu, alpha, mike):\n%s",
				names[i], names[i-1], formatted)
		}
	}

	again, changed, err := configFmtBytes("test.conf", []byte(formatted))
	if err != nil {
		t.Fatalf("second fmt: %v", err)
	}
	if changed {
		t.Errorf("second fmt changed the output:\n%s\nvs\n%s", formatted, again)
	}
}

// TestConfigFmtWriteKeepsValueMeaning runs `ze config fmt -w` then
// `ze config fmt --check` through cmdFmt on values that today print bare.
//
// VALIDATES: AC-9 through the command: -w exits 0, the written file holds
// each value quoted, and --check on it exits 0, so the written file parses
// back to the same text.
// PREVENTS: a value holding `]`, `[`, a newline, a carriage return, or a
// leading `(` or `)` written bare, which the next fmt refuses (exit 2).
func TestConfigFmtWriteKeepsValueMeaning(t *testing.T) {
	cases := []struct {
		name    string
		written string // the value as the input file spells it
		printed string // the value as fmt must print it
	}{
		{"close-bracket", `"x]y(z"`, `"x]y(z"`},
		{"open-bracket", `"a[b"`, `"a[b"`},
		{"newline", `"l1\nl2"`, `"l1\nl2"`},
		{"carriage-return", "\"a\rb\"", "\"a\rb\""},
		{"leading-paren", `"(open"`, `"(open"`},
		{"leading-close-paren", `")close"`, `")close"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "meaning.conf")
			input := "bgp {\n\tpeer p1 {\n\t\tdescription " + tc.written +
				";\n\t\tconnection {\n\t\t\tremote {\n\t\t\t\tip 192.0.2.1;\n\t\t\t}\n\t\t}\n" +
				"\t\tsession {\n\t\t\tasn {\n\t\t\t\tremote 65001;\n\t\t\t}\n\t\t}\n\t}\n}\n"
			if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
				t.Fatal(err)
			}

			rc, stderr := captureStderr(t, func() int { return cmdFmt([]string{"-w", path}) })
			if rc != exitOK {
				t.Fatalf("fmt -w: exit %d, stderr:\n%s", rc, stderr)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), "description "+tc.printed+"\n") {
				t.Errorf("written file does not hold description %s:\n%s", tc.printed, data)
			}

			rc, stderr = captureStderr(t, func() int { return cmdFmt([]string{"--check", path}) })
			if rc != exitOK {
				t.Errorf("fmt --check on the written file: exit %d, stderr:\n%s", rc, stderr)
			}
		})
	}
}
