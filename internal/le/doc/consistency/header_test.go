// VALIDATES: DesignHeader reads every `// Design:` page of a file's leading
// run of comment and blank lines, wherever in that run the line sits, and no
// line after it.
// PREVENTS: three readers of one header disagreeing, as a 25-line bound did for
// internal/component/command/registry/registry.go, whose `// Design:` line is
// line 27: `./le doc consistency` judged its page while the edit gate and
// `./le spec design documents` owed nothing.
package docconsistency

import (
	"slices"
	"strings"
	"testing"
)

// TestDesignHeaderReadsTheLeadingComments is the header rule. Method: each case
// is a file text and the pages, line numbers and quoted lines DesignHeader
// answers for it.
func TestDesignHeaderReadsTheLeadingComments(t *testing.T) {
	deep := strings.Repeat("// Package widget is described at length.\n", 25) + "//\n"
	cases := []struct {
		name string
		text string
		want []DesignLink
	}{
		{
			name: "every page of every header line, in order",
			text: "//go:build linux\n\n// Design: docs/a.md -- a\n// Design: docs/b.md, docs/c.md -- two\n// Related: x.go -- no page\n\npackage x\n",
			want: []DesignLink{
				{Page: "docs/a.md", Number: 3, Line: "// Design: docs/a.md -- a"},
				{Page: "docs/b.md", Number: 4, Line: "// Design: docs/b.md, docs/c.md -- two"},
				{Page: "docs/c.md", Number: 4, Line: "// Design: docs/b.md, docs/c.md -- two"},
			},
		},
		{
			name: "a line past line 25 of a long package comment",
			text: deep + "// Design: docs/deep.md -- deep\npackage x\n",
			want: []DesignLink{{Page: "docs/deep.md", Number: 27, Line: "// Design: docs/deep.md -- deep"}},
		},
		{
			name: "the package clause ends the header",
			text: "// Design: docs/a.md -- a\npackage x\n\n// Design: docs/late.md -- late\n",
			want: []DesignLink{{Page: "docs/a.md", Number: 1, Line: "// Design: docs/a.md -- a"}},
		},
		{
			name: "a YANG module statement ends the header",
			text: "// Design: docs/a.md -- a\r\nmodule ze-x {\r\n// Design: docs/late.md -- late\r\n",
			want: []DesignLink{{Page: "docs/a.md", Number: 1, Line: "// Design: docs/a.md -- a"}},
		},
		{
			name: "a line naming no page",
			text: "// Design: (none -- a generated file)\npackage x\n",
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DesignHeader(tc.text); !slices.Equal(got, tc.want) {
				t.Fatalf("DesignHeader = %#v, want %#v", got, tc.want)
			}
		})
	}
}
