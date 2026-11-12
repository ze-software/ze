package reporewrite

import "testing"

// TestGoLineCountsArePinned pins what CountGoLines answers on a fixed corpus,
// so a change to the counting rules cannot reach recorded history unnoticed.
//
// The method writes each source in the test and compares the whole GoLines
// value with counts made by hand: code, blank and line-comment lines, a block
// comment over several lines, code after a block comment closes, comment
// markers inside a string, CRLF endings, an unterminated last line, an empty
// source and invalid UTF-8. A red case is either a defect or a deliberate rule
// change, and a deliberate change owes a raise of GoLineRulesVersion.
func TestGoLineCountsArePinned(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   GoLines
	}{
		{"empty source", "", GoLines{}},
		{"one newline", "\n", GoLines{Total: 1, Blank: 1}},
		{"code lines", "package x\nfunc f() {}\n", GoLines{Total: 2, Code: 2}},
		{"blank lines, spaces and tabs", "\n\n  \t\n", GoLines{Total: 3, Blank: 3}},
		{"line comments and a trailing comment", "// a\n  // b\nx := 1 // trailing\n", GoLines{Total: 3, Code: 1, Comment: 2}},
		{"block comment over three lines", "/* start\n middle\n end */\n", GoLines{Total: 3, Comment: 3}},
		{"code after a block comment closes", "/* a\n b */ x := 1\n", GoLines{Total: 2, Code: 1, Comment: 1}},
		{"two block comments on one line", "/* a */ /* b */\n", GoLines{Total: 1, Comment: 1}},
		{"code after a one-line block comment", "/* a */ x\n", GoLines{Total: 1, Code: 1}},
		{"block opened after code is not seen", "x /* c\n */\n", GoLines{Total: 2, Code: 2}},
		{"block comment never closed", "/* open\nstill open", GoLines{Total: 2, Comment: 2}},
		{"comment markers inside strings", "s := \"http://x\"\nt := \"/*\"\ny := 2\n", GoLines{Total: 3, Code: 3}},
		{"CRLF endings", "package x\r\n\r\n// c\r\n", GoLines{Total: 3, Code: 1, Blank: 1, Comment: 1}},
		{"unterminated last code line", "a\nb", GoLines{Total: 2, Code: 2}},
		{"unterminated last comment line", "// c", GoLines{Total: 1, Comment: 1}},
		{"invalid UTF-8", "\xff\xfe\n// \xff\n\n", GoLines{Total: 3, Code: 1, Blank: 1, Comment: 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CountGoLines([]byte(tc.source))
			if got != tc.want {
				t.Fatalf("CountGoLines(%q) = %+v, want %+v; a deliberate change to the Go "+
					"counting rules owes a raise of GoLineRulesVersion (now %d), because "+
					"recorded history was measured under the old rules",
					tc.source, got, tc.want, GoLineRulesVersion)
			}
		})
	}
}

// TestCountVendorModulesCountsModuleHeaders asks what CountVendorModules
// counts in a vendor/modules.txt. Method: counts written by hand over an
// empty file, one module, a "## explicit" annotation and package lines that
// are no module, an unterminated last header, and a "#" with no space.
func TestCountVendorModulesCountsModuleHeaders(t *testing.T) {
	cases := []struct {
		name    string
		modules string
		want    int
	}{
		{"empty file", "", 0},
		{"one module", "# example.com/a v1.0.0\n", 1},
		{"annotations and packages are no modules", "# example.com/a v1.0.0\n## explicit; go 1.22\nexample.com/a\nexample.com/a/sub\n", 1},
		{"an unterminated last header counts", "# example.com/a v1.0.0\n## explicit\n# example.com/b v0.1.0", 2},
		{"a hash with no space is no module", "#example.com/a v1.0.0\n", 0},
	}
	for _, tc := range cases {
		if got := CountVendorModules([]byte(tc.modules)); got != tc.want {
			t.Errorf("%s: CountVendorModules(%q) = %d, want %d", tc.name, tc.modules, got, tc.want)
		}
	}
}
