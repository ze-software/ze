// Related: script/script.go -- the generator le's scripts come from

package completion

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/plugins/completion/script"
)

// VALIDATES: R-12 -- ze's four completion scripts are byte-for-byte the
// scripts captured from HEAD before the le generator was added beside them.
// PREVENTS: sharing the generator with le changing what an operator's shell
// loads for ze.
func TestZeCompletionScriptsUnchanged(t *testing.T) {
	for _, shell := range shells {
		want, err := os.ReadFile("testdata/ze." + shell)
		if err != nil {
			t.Fatal(err)
		}
		var got bytes.Buffer
		if code := generate(shell, &got); code != 0 {
			t.Fatalf("%s: exit %d", shell, code)
		}
		if !bytes.Equal(got.Bytes(), want) {
			t.Errorf("ze completion %s differs from testdata/ze.%s", shell, shell)
		}
	}
}

// VALIDATES: AC-33, AC-34, AC-35 script side -- each le script registers both
// spellings, `le` and `./le`, and asks the binary beside the typed word,
// `bin/le completion words`, never the launcher, with stderr discarded.
// PREVENTS: a keystroke running the launcher, which checks staleness and can
// build, or a script that completes only one of the two spellings.
func TestLeCompletionScriptRegistersBothSpellings(t *testing.T) {
	program := script.Program{Name: "le", Spellings: []string{"le", "./le"}, Binary: "bin/le"}
	registrations := map[string][]string{
		script.ShellBash: {"complete -F _le le ./le\n"},
		script.ShellZsh:  {"#compdef le\n", "compdef _le le ./le\n"},
		script.ShellFish: {"complete -c le -f -a '(__le_complete)'\n", "complete -c ./le -f -a '(__le_complete)'\n"},
	}
	for shell, lines := range registrations {
		text, err := script.Generate(shell, program)
		if err != nil {
			t.Fatalf("%s: %v", shell, err)
		}
		for _, line := range lines {
			if !strings.Contains(text, line) {
				t.Errorf("%s: no %q in the script", shell, line)
			}
		}
		for _, want := range []string{"/bin/le", "completion words", "2>/dev/null", "'#!'"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s: the script holds no %q", shell, want)
			}
		}
	}
	if _, err := script.Generate("nushell", program); err == nil {
		t.Error("a shell with no le script was accepted")
	}
}
