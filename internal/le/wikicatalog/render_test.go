package wikicatalog

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderGoldenCatalogs(t *testing.T) {
	tests := []struct {
		name    string
		entries []Entry
		golden  string
	}{
		{name: "empty", golden: "empty.md"},
		{
			name: "multiple commands are escaped and sorted",
			entries: []Entry{
				{
					Path:          "show zeta",
					Description:   "Zeta | first\nZeta details",
					Mode:          "read-only",
					WireMethod:    "show_zeta",
					AnswerShape:   "tab",
					AddressFields: []string{"address"},
					Backend:       []string{"rib"},
					TaskSupport:   "stream",
					Args: []Argument{
						{Name: "family", Type: "enum", Values: []string{"blue", "red"}, Mandatory: true},
					},
					Pipes: []Pipe{
						{Name: "detail", Description: "show detail", TakesArg: true},
					},
					Operators: []Operator{
						{Name: "json", Available: "always"},
						{Name: "match", Available: "with-rows"},
						{Name: "log", Available: "when-streaming"},
						{Name: "save", Available: "always", LocalOnly: true},
					},
					Aliases: []Alias{
						{Name: "quick", Description: "short view", Expansion: "match up | count"},
					},
					Subcommands: []string{"brief", "detail"},
				},
				{Path: "clear beta", Description: "Clear | beta", Mode: "offline"},
				{Path: "show alpha", Description: "Alpha", Mode: "daemon"},
			},
			golden: "multi.md",
		},
		{
			name: "literal verb labels and anchors",
			entries: []Entry{
				{Path: "`show` route", Mode: "read-only", Description: "Backtick"},
				{Path: "contents route", Mode: "read-only", Description: "Reserved"},
				{Path: "show! route", Mode: "read-only", Description: "Punctuation"},
				{Path: "show? route", Mode: "read-only", Description: "Collision"},
				{Path: "表示 route", Mode: "read-only", Description: "Unicode"},
			},
			golden: "literal-verbs.md",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", test.golden))
			if err != nil {
				t.Fatalf("read golden fixture: %v", err)
			}
			got, err := Render(test.entries)
			if err != nil {
				t.Fatalf("render catalog: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("rendered Markdown differs from %s\nwant:\n%s\ngot:\n%s", test.golden, want, got)
			}
		})
	}
}

func TestRenderRejectsUnknownOperatorAvailability(t *testing.T) {
	_, err := Render([]Entry{{
		Path: "show bad", Mode: "daemon",
		Operators: []Operator{{Name: "mystery", Available: "sometimes"}},
	}})
	if err == nil || err.Error() != "unknown operator availability for mystery" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderLiteralProseAndDynamicCodeSpans(t *testing.T) {
	rendered, err := Render([]Entry{{
		Path:          "show `tick`",
		Mode:          "read-only",
		Description:   "*summary* | literal\n_detail_",
		WireMethod:    "`wire`",
		AnswerShape:   "`shape`",
		AddressFields: []string{"`address`"},
		Aliases: []Alias{{
			Name:        "`quick`",
			Description: "**literal**",
			Expansion:   "match `` value",
		}},
	}})
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	content := string(rendered)
	for _, want := range []string{
		"| `` show `tick` `` | read-only | \\*summary\\* \\| literal |",
		"### `` show `tick` ``",
		"\\*summary\\* \\| literal\n\\_detail\\_",
		"Mode: read-only | Wire: `` `wire` ``",
		"Answer shape: `` `shape` ``",
		"Address fields: `` `address` ``",
		"- `` `quick` `` -- \\*\\*literal\\*\\* (```match `` value```)",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("rendered catalog omitted %q:\n%s", want, content)
		}
	}
}

func TestNeedsDetailForEveryOptionalContractField(t *testing.T) {
	for _, entry := range []Entry{
		{Aliases: []Alias{{Name: "summary"}}},
		{WireMethod: "show"},
		{AnswerShape: "tab"},
		{AddressFields: []string{"address"}},
	} {
		if !needsDetail(entry) {
			t.Errorf("needsDetail(%+v) = false", entry)
		}
	}
}

func TestRenderAliasOnlyEntryIncludesDetail(t *testing.T) {
	rendered, err := Render([]Entry{{
		Path:        "show alias",
		Mode:        "read-only",
		Description: "Alias command",
		Aliases: []Alias{{
			Name:        "summary",
			Description: "Show a summary",
			Expansion:   "display address",
		}},
	}})
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	content := string(rendered)
	for _, want := range []string{
		"### `show alias`",
		"Named chains:\n- `summary` -- Show a summary (`display address`)",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("alias-only catalog omitted %q:\n%s", want, content)
		}
	}
}

func TestRenderLiteralVerbLabelsAndHeadingAnchors(t *testing.T) {
	rendered, err := Render([]Entry{
		{Path: "`show` route", Mode: "read-only", Description: "Backtick"},
		{Path: "contents route", Mode: "read-only", Description: "Reserved"},
		{Path: "show! route", Mode: "read-only", Description: "Punctuation"},
		{Path: "show? route", Mode: "read-only", Description: "Collision"},
		{Path: "show-1 route", Mode: "read-only", Description: "Slug collision"},
		{Path: "表示 route", Mode: "read-only", Description: "Unicode"},
	})
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	content := string(rendered)
	for _, want := range []string{
		"- [\\`show\\`](#show) (1)",
		"- [contents](#contents-1) (1)",
		"- [show\\!](#show-1) (1)",
		"- [show\\?](#show-2) (1)",
		"- [show\\-1](#show-1-1) (1)",
		"## show\\-1",
		"- [表示](#表示) (1)",
		"## \\`show\\`",
		"## contents",
		"## show\\!",
		"## show\\?",
		"## 表示",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("literal-verb catalog omitted %q:\n%s", want, content)
		}
	}
}
