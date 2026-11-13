// Design: docs/architecture/config/yang-config-design.md -- Value refusals
// Related: completer.go -- ValidateValueAtPath
// Related: model_commands_edit.go -- cmdSet, cmdInsert

package cli

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestValidateValueAtPathKeepsParserVerdicts checks the shipped declarations
// behind the editor's value gate. Prefix members and hex keys stay accepted;
// declared range, length and pattern boundaries retain their typed refusals.
// The accepted controls prevent a validator that refuses every input passing.
func TestValidateValueAtPathKeepsParserVerdicts(t *testing.T) {
	completer := acceptanceCompleter(t)
	hexPath := []string{"ospf", "address-family", "ipv6", "interfaces", "interface", "eth0", "ipsec", "key"}
	for _, tc := range []struct {
		name     string
		path     []string
		accepted []string
		refused  []string
		kind     configyang.ErrorType
	}{
		{
			name:     "IPv4 prefix members",
			path:     []string{"interface", "ethernet", "eth0", "unit", "0", "ipv4", "address"},
			accepted: []string{"0.0.0.0/0", "192.0.2.1/24", "198.51.100.7/32"},
			refused:  []string{"192.0.2.1", "not-a-prefix"},
			kind:     configyang.ErrTypePattern,
		},
		{
			name:     "IPv6 prefix members",
			path:     []string{"interface", "ethernet", "eth0", "unit", "0", "ipv6", "address"},
			accepted: []string{"::/0", "2001:db8::1/64", "2001:db8::1/128"},
			refused:  []string{"2001:db8::1", "not-a-prefix"},
			kind:     configyang.ErrTypePattern,
		},
		{
			name:     "hex key format",
			path:     hexPath,
			accepted: []string{strings.Repeat("aB01", 16)},
			refused:  []string{strings.Repeat("g", 64)},
			kind:     configyang.ErrTypePattern,
		},
		{
			name:     "hex key length",
			path:     hexPath,
			accepted: []string{strings.Repeat("a", 40), strings.Repeat("B", 128)},
			refused:  []string{strings.Repeat("a", 39), strings.Repeat("B", 129)},
			kind:     configyang.ErrTypeLength,
		},
		{
			name:     "declared cost range",
			path:     []string{"ospf", "interfaces", "interface", "eth0", "cost"},
			accepted: []string{"1", "65535"},
			refused:  []string{"0", "65536"},
			kind:     configyang.ErrTypeRange,
		},
		{
			name:     "process command length",
			path:     []string{"exabgp", "bridge", "process", "p1", "run"},
			accepted: []string{"x", strings.Repeat("x", 4096)},
			refused:  []string{"", strings.Repeat("x", 4097)},
			kind:     configyang.ErrTypeLength,
		},
		{
			name:     "rate format",
			path:     []string{"l2tp", "shaper", "default-rate"},
			accepted: []string{"10mbit", "1gbps"},
			refused:  []string{"fast", "10"},
			kind:     configyang.ErrTypePattern,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for index, value := range tc.accepted {
				if err := completer.ValidateValueAtPath(tc.path, value); err != nil {
					t.Errorf("accepted input %d (%d bytes) was refused: %v", index, len(value), err)
				}
			}
			for _, value := range tc.refused {
				err := completer.ValidateValueAtPath(tc.path, value)
				acceptanceRefusal(t, err, tc.kind, tc.path)
			}
		})
	}
}

// TestModelPrefixMembersKeepParserVerdicts drives both editor verbs in file
// and session mode. Refused prefixes change neither candidate nor member
// order, while accepted set and insert values reach the stored leaf-list.
func TestModelPrefixMembersKeepParserVerdicts(t *testing.T) {
	for _, user := range []string{"", "alice"} {
		mode := "file"
		if user != "" {
			mode = "session"
		}
		for _, tc := range []struct {
			family string
			first  string
			next   string
			bad    string
		}{
			{family: "ipv4", first: "192.0.2.1/24", next: "198.51.100.1/24", bad: "192.0.2.9"},
			{family: "ipv6", first: "2001:db8::1/64", next: "2001:db8:1::1/64", bad: "2001:db8::9"},
		} {
			t.Run(mode+"/"+tc.family, func(t *testing.T) {
				model, _, _ := newCommitGrammarModel(t, user)
				path := []string{"interface", "ethernet", "eth0", "unit", "0", tc.family, "address"}
				setting := strings.Join(path, " ")
				if _, err := model.dispatchCommand("set " + setting + " " + tc.first); err != nil {
					t.Fatalf("set accepted prefix: %v", err)
				}
				before := model.editor.WorkingContent()
				for _, command := range []string{
					"set " + setting + " " + tc.bad,
					"insert " + setting + " " + tc.bad + " after " + tc.first,
				} {
					_, err := model.dispatchCommand(command)
					acceptanceRefusal(t, err, configyang.ErrTypePattern, path)
					if got := model.editor.WorkingContent(); got != before {
						t.Errorf("refused command %q changed the candidate:\n%s", command, got)
					}
				}
				if _, err := model.dispatchCommand("insert " + setting + " " + tc.next + " after " + tc.first); err != nil {
					t.Fatalf("insert accepted prefix: %v", err)
				}
				container := model.editor.WalkPath(path[:len(path)-1])
				if container == nil {
					t.Fatal("accepted prefix commands created no address container")
				}
				if got, want := container.GetMultiValues("address"), []string{tc.first, tc.next}; !slices.Equal(got, want) {
					t.Errorf("stored prefix order %q, want %q", got, want)
				}
			})
		}
	}
}

// TestModelHexKeyKeepsParserVerdicts checks the accepted hex spelling on the
// actual set route, beside equal-length nonhex and too-short hex refusals.
// Refusing either replacement must preserve the already stored valid key.
func TestModelHexKeyKeepsParserVerdicts(t *testing.T) {
	for _, user := range []string{"", "alice"} {
		mode := "file"
		if user != "" {
			mode = "session"
		}
		t.Run(mode, func(t *testing.T) {
			model, _, _ := newCommitGrammarModel(t, user)
			path := []string{"ospf", "address-family", "ipv6", "interfaces", "interface", "eth0", "ipsec", "key"}
			setting := strings.Join(path, " ")
			valid := strings.Repeat("aB01", 16)
			if _, err := model.dispatchCommand("set " + setting + " " + valid); err != nil {
				t.Fatalf("set accepted hex key: %v", err)
			}
			before := model.editor.WorkingContent()
			for _, tc := range []struct {
				value string
				kind  configyang.ErrorType
			}{
				{value: strings.Repeat("g", 64), kind: configyang.ErrTypePattern},
				{value: strings.Repeat("a", 39), kind: configyang.ErrTypeLength},
			} {
				_, err := model.dispatchCommand("set " + setting + " " + tc.value)
				acceptanceRefusal(t, err, tc.kind, path)
				if got := model.editor.WorkingContent(); got != before {
					t.Error("a refused replacement changed the stored hex key")
				}
			}
			container := model.editor.WalkPath(path[:len(path)-1])
			if container == nil {
				t.Fatal("accepted set created no IPsec container")
			}
			stored, found := container.Get("key")
			if !found {
				t.Fatal("accepted set stored no hex key")
			}
			if stored != valid {
				t.Error("the stored key differs from the accepted hex spelling")
			}
		})
	}
}

// acceptanceCompleter keeps the extra shipped declarations local to this
// test. The CLI unit composition does not import the bridge or L2TP YANG
// packages; registering them globally would change every other test's model.
func acceptanceCompleter(t *testing.T) *Completer {
	t.Helper()
	loader := configyang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("load embedded YANG: %v", err)
	}
	if err := loader.LoadRegistered(); err != nil {
		t.Fatalf("load registered YANG: %v", err)
	}
	for _, path := range []string{
		"../l2tp/yang/ze-l2tp-conf.yang",
		"../l2tp/plugins/shaper/yang/ze-l2tp-shaper-conf.yang",
		"../../plugins/exabgp/bridgeplugin/yang/ze-exabgp-bridge-conf.yang",
	} {
		name := filepath.Base(path)
		if slices.ContainsFunc(configyang.Modules(), func(module configyang.Module) bool { return module.Name == name }) {
			continue
		}
		if err := loader.AddModuleFromFile(path); err != nil {
			t.Fatalf("load shipped declaration %s: %v", name, err)
		}
	}
	resolved, err := loader.Resolve()
	if err != nil {
		t.Fatalf("resolve acceptance model: %v", err)
	}
	return &Completer{schema: resolved}
}

// acceptanceRefusal checks the kind and setting path so a path lookup error
// cannot stand in for the declaration's actual value refusal.
func acceptanceRefusal(t *testing.T, err error, kind configyang.ErrorType, path []string) {
	t.Helper()
	var refusal *configyang.ValidationError
	if !errors.As(err, &refusal) {
		t.Fatalf("value error %v does not retain a ValidationError", err)
	}
	if refusal.Type != kind {
		t.Errorf("refusal kind %v, want %v", refusal.Type, kind)
	}
	if got := refusal.Segments(); !slices.Equal(got, path) {
		t.Errorf("refusal setting %q, want %q", got, path)
	}
}
