// Design: (none -- shell completion of a root's registered sub-verbs)
// Related: root_commands.go -- shellRootsWithSubVerbs, fishRootSubVerbLines
// Related: words.go -- rootSubVerbs, the one parser of a root's Subs hint

package completion

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

// subVerbProbeRoot is a registered root with sub-verbs and no hand-written
// arm in any generator: `ze secret` registers decode and encode as its Subs.
const subVerbProbeRoot = "secret"

// VALIDATES: a root that registers sub-verbs gets its second-word arm in each
// generated shell script, and the words that arm asks for at tab time are the
// verbs the root registered.
// PREVENTS: a new root completing its own name but none of its verbs until
// someone edits each generator, the central enumeration of special-cased
// roots the generators carried.
// Method: generate the four scripts, find the arm that serves the probe root
// in each, then run the words query that arm makes and read its verbs.
func TestScriptsOfferRegisteredSubVerbs(t *testing.T) {
	if !slices.Contains(shellRootsWithSubVerbs(), subVerbProbeRoot) {
		t.Fatalf("%s is not a shell-visible root with sub-verbs; the probe needs one", subVerbProbeRoot)
	}

	arms := map[string]string{
		// The default case arm serves every root with no hand-written arm.
		"bash": "        *)\n" +
			"            # Every other root: the second word from the sub-verbs it\n" +
			"            # registers, so a new root completes with no edit here.\n" +
			"            if [[ ${depth} -eq 1 ]]; then\n" +
			"                local completions\n" +
			"                completions=$(ze completion words \"${subcmd}\" 2>/dev/null | cut -f1)\n",
		"zsh": "                *)\n" +
			"                    # Every other root: the second word from the sub-verbs it\n" +
			"                    # registers, so a new root completes with no edit here.\n" +
			"                    if (( CURRENT == 2 )); then\n" +
			"                        local -a root_verbs\n" +
			"                        root_verbs=(${(f)\"$(ze completion words ${words[1]} 2>/dev/null",
		"fish":    "complete -c ze -n 'test (__ze_depth secret) = 0' -a '(__ze_complete_dynamic secret)'\n",
		"nushell": "(^ze completion words $subcmd ...$rest",
	}
	for _, shell := range shells {
		var script bytes.Buffer
		if code := generate(shell, &script); code != 0 {
			t.Fatalf("%s: exit %d", shell, code)
		}
		arm, ok := arms[shell]
		if !ok {
			t.Fatalf("%s: no expected arm declared for this shell", shell)
		}
		if !strings.Contains(script.String(), arm) {
			t.Errorf("%s: no second-word arm serving %s:\n%s", shell, subVerbProbeRoot, arm)
		}
		// A case arm label ends its line; the fish rule's `secret)` does not.
		if strings.Contains(script.String(), " "+subVerbProbeRoot+")\n") {
			t.Errorf("%s: %s has a hand-written arm, so the probe does not reach the derived one", shell, subVerbProbeRoot)
		}
	}

	var words bytes.Buffer
	if code := writeWords(&words, []string{subVerbProbeRoot}); code != 0 {
		t.Fatalf("ze completion words %s: exit %d", subVerbProbeRoot, code)
	}
	for _, verb := range []string{"decode", "encode"} {
		if !strings.Contains(words.String(), verb+"\t") {
			t.Errorf("ze completion words %s does not offer %q:\n%s", subVerbProbeRoot, verb, words.String())
		}
	}
}

// VALIDATES: rootSubVerbs keeps the verbs of a Subs hint and drops what an
// operator never types as a second word.
// PREVENTS: a completion offering `<plugin-name>`, `-c` or the `1` of a
// sentence as a word.
func TestRootSubVerbsKeepsOnlyVerbs(t *testing.T) {
	cases := []struct {
		subs string
		want []string
	}{
		{"decode <hex> | decode -, encode <route>, plugin", []string{"decode", "encode", "plugin"}},
		{"<plugin-name> for plugin CLI, test for debugging", []string{"test"}},
		{"-c <cmd> for single command", nil},
		{"--managed --force, [--json]", nil},
		{"kernel-capabilities, config <file>, 1 = not", []string{"kernel-capabilities", "config"}},
		{"", nil},
	}
	for _, c := range cases {
		var got []string
		for _, verb := range rootSubVerbs(c.subs) {
			got = append(got, verb.Name)
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("rootSubVerbs(%q) = %v, want %v", c.subs, got, c.want)
		}
	}
}
