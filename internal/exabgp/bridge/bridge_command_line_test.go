// GOAL: JoinCommandLine and SplitCommandLine are inverses over any argv.
//
// METHOD: join argvs holding the words that need quoting (blank, apostrophe,
// double quote, backslash, dollar, empty) and split the line back.
//
// PREVENTS: a migrated ExaBGP argument reaching the script changed, split in
// two, or dropped.

package bridge

import (
	"slices"
	"testing"
)

// TestJoinCommandLineRoundTrips checks each argv survives join then split, and
// that a word in the safe set is written bare.
func TestJoinCommandLineRoundTrips(t *testing.T) {
	argvs := [][]string{
		{"/opt/inject.py"},
		{"python3", "/opt/inject.py", "--label=a,b:c@d%e+f"},
		{"cmd", "two words", "it's", `say "hi"`, `back\slash`, "$HOME", ""},
		{"cmd", "tab\there", "'", "''"},
	}
	for _, argv := range argvs {
		line := JoinCommandLine(argv)
		got, err := SplitCommandLine(line)
		if err != nil {
			t.Fatalf("split %q: %v", line, err)
		}
		if !slices.Equal(got, argv) {
			t.Errorf("argv %q joined to %q split back as %q", argv, line, got)
		}
	}
	if line := JoinCommandLine([]string{"python3", "/opt/inject.py", "--flag"}); line != "python3 /opt/inject.py --flag" {
		t.Errorf("safe words quoted: %q", line)
	}
}
