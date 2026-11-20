// VALIDATES: DesignHeader reads every `// Design:` page of a file's leading
// run of comment and blank lines, wherever in that run the line sits, and no
// line after it, with the topic each line gives its pages.
// PREVENTS: the readers of one header disagreeing, as a 25-line bound did for
// internal/component/command/registry/registry.go, whose `// Design:` line is
// line 27: `./le doc consistency` judged its page while the edit gate and
// `./le spec design documents` owed nothing, and ai/DOCS-TO-CODE.md left the
// file out.
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
				{Page: "docs/a.md", Topic: "a", Number: 3, Line: "// Design: docs/a.md -- a"},
				{Page: "docs/b.md", Topic: "two", Number: 4, Line: "// Design: docs/b.md, docs/c.md -- two"},
				{Page: "docs/c.md", Topic: "two", Number: 4, Line: "// Design: docs/b.md, docs/c.md -- two"},
			},
		},
		{
			name: "a line past line 25 of a long package comment",
			text: deep + "// Design: docs/deep.md -- deep\npackage x\n",
			want: []DesignLink{{Page: "docs/deep.md", Topic: "deep", Number: 27, Line: "// Design: docs/deep.md -- deep"}},
		},
		{
			name: "the package clause ends the header",
			text: "// Design: docs/a.md -- a\npackage x\n\n// Design: docs/late.md -- late\n",
			want: []DesignLink{{Page: "docs/a.md", Topic: "a", Number: 1, Line: "// Design: docs/a.md -- a"}},
		},
		{
			name: "a YANG module statement ends the header",
			text: "// Design: docs/a.md -- a\r\nmodule ze-x {\r\n// Design: docs/late.md -- late\r\n",
			want: []DesignLink{{Page: "docs/a.md", Topic: "a", Number: 1, Line: "// Design: docs/a.md -- a"}},
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

// TestDesignTargetsReadsEachTopicSeparator is the topic rule the
// ai/DOCS-TO-CODE.md generator renders. Method: each case is one header line
// and the pages and topic designTargets answers for it, one case per separator
// a citation writes between its pages and its topic.
func TestDesignTargetsReadsEachTopicSeparator(t *testing.T) {
	for _, tc := range []struct {
		line  string
		pages []string
		topic string
	}{
		{"// Design: docs/a/b.md -- dashes", []string{"docs/a/b.md"}, "dashes"},
		{"// Design: docs/a/b.md - dash", []string{"docs/a/b.md"}, "dash"},
		{"// Design: docs/a/b.md — em dash", []string{"docs/a/b.md"}, "em dash"},
		{"// Design: docs/a/b.md : colon", []string{"docs/a/b.md"}, "colon"},
		{"// Design: docs/a/b.md: glued colon", []string{"docs/a/b.md"}, "glued colon"},
		{"// Design: docs/a/b.md plain", []string{"docs/a/b.md"}, "plain"},
		{"// Design: docs/a/b.md plain -- and more", []string{"docs/a/b.md"}, "plain -- and more"},
		{"// Design: docs/a/b.md, docs/c.md -- two pages", []string{"docs/a/b.md", "docs/c.md"}, "two pages"},
		{"// Design: docs/a/b.md", []string{"docs/a/b.md"}, ""},
		{"// Design: docs/a/b.md -- ", []string{"docs/a/b.md"}, ""},
	} {
		pages, topic := designTargets(tc.line)
		if !slices.Equal(pages, tc.pages) || topic != tc.topic {
			t.Errorf("designTargets(%q) = (%q, %q), want (%q, %q)", tc.line, pages, topic, tc.pages, tc.topic)
		}
	}
}
