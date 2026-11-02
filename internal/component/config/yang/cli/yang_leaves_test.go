// Design: docs/architecture/api/commands.md — a flag that selects the answer is a leaf

package cli

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	_ "github.com/ze-software/ze/internal/plugins/config-yang/yang" // Register ze-yang-cli-cmd, the model the routes judge against.
)

// yangAnswerOn validates tokens against the command's model, as every route
// does, and runs the registered handler on the local or the daemon route.
func yangAnswerOn(t *testing.T, path string, tokens []string, remote bool) command.LocalAnswer {
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

// yangEncoded is the answer as a caller compares it: code, payload JSON and
// diagnostic.
func yangEncoded(t *testing.T, answer command.LocalAnswer) string {
	t.Helper()
	payload, err := json.Marshal(answer.Payload)
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}
	return strconv.Itoa(answer.Code) + "\n" + string(payload) + "\n" + answer.Diagnostic
}

// TestShowYangSelectingLeaves proves the choices `show yang tree`,
// `show yang completion` and `show yang doc` made only through a local flag
// are declared leaves, so every surface can send them.
//
// Goal: AC-8 part 2. On the daemon route, where a flag never reaches the
// handler, the leaf answers exactly what the operator's own `ze` answers for
// the flag. Method: each call runs through the model validator on both
// routes, and its answer is compared with the local flag spelling's.
func TestShowYangSelectingLeaves(t *testing.T) {
	cases := []struct {
		path string
		leaf []string
		flag []string
	}{
		{"show yang tree", []string{"commands"}, []string{"--commands"}},
		{"show yang tree", []string{"config"}, []string{"--config"}},
		{"show yang completion", []string{"min-prefix", "2"}, []string{"--min-prefix", "2"}},
		{"show yang doc", []string{"list"}, []string{"--list"}},
		{"show yang doc", []string{"show", "bgp", "peer", "list"}, []string{"show", "bgp", "peer", "list"}},
		{"show yang doc", []string{"command", "show", "bgp", "peer", "list"}, []string{"show", "bgp", "peer", "list"}},
	}
	for _, tc := range cases {
		t.Run(tc.path+" "+strings.Join(tc.leaf, " "), func(t *testing.T) {
			want := yangEncoded(t, yangAnswerOn(t, tc.path, tc.flag, false))
			if !strings.HasPrefix(want, "0\n") || strings.HasPrefix(want, "0\nnull") {
				t.Fatalf("the local flag spelling %q answered nothing to compare:\n%s", tc.flag, want)
			}
			for _, remote := range []bool{false, true} {
				if got := yangEncoded(t, yangAnswerOn(t, tc.path, tc.leaf, remote)); got != want {
					t.Errorf("remote=%v: %q answered\n%s\nwant what %q answers\n%s", remote, tc.leaf, got, tc.flag, want)
				}
			}
		})
	}

	// The two halves differ, so each leaf selects its own half.
	commands := yangEncoded(t, yangAnswerOn(t, "show yang tree", []string{"commands"}, true))
	config := yangEncoded(t, yangAnswerOn(t, "show yang tree", []string{"config"}, true))
	if commands == config {
		t.Error("commands and config answered the same tree")
	}
}

// TestShowYangLeafAndFlagRefused proves one choice made twice, by the leaf and
// by its local option, is refused rather than resolved by a silent winner.
func TestShowYangLeafAndFlagRefused(t *testing.T) {
	for _, tc := range []struct {
		path   string
		tokens []string
	}{
		{"show yang tree", []string{"config", "--commands"}},
		{"show yang tree", []string{"commands", "config"}},
		{"show yang completion", []string{"min-prefix", "2", "--min-prefix=3"}},
		{"show yang doc", []string{"list", "--list"}},
	} {
		answer := yangAnswerOn(t, tc.path, tc.tokens, false)
		if answer.Code == 0 || answer.Payload != nil || !strings.Contains(answer.Diagnostic, "make the same choice") {
			t.Errorf("%s %q: code %d payload %v diagnostic %q, want the twice-made choice refused",
				tc.path, tc.tokens, answer.Code, answer.Payload, answer.Diagnostic)
		}
	}
}

// TestShowYangCompletionDepthRangeJudgedByModel proves the depth leaf's range
// is the model's: 11 never reaches the handler.
func TestShowYangCompletionDepthRangeJudgedByModel(t *testing.T) {
	if _, err := command.ValidateModelArgs("show yang completion", []string{"min-prefix", "11"}, nil); err == nil {
		t.Error("min-prefix 11 passed the model, want the 1..10 range to refuse it")
	}
}
