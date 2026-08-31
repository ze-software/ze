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
					Description:   "Zeta | first",
					LongHelp:      "Zeta details\nZeta second line",
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
				{Path: "show alpha", Description: "Alpha", Mode: "daemon", WireMethod: "show_alpha"},
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
		Description:   "*summary* | literal",
		LongHelp:      "_detail_\n**second**",
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
		"\\_detail\\_\n\\*\\*second\\*\\*",
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

func TestRenderZeroSupportCommandsHaveCanonicalPipeVerdict(t *testing.T) {
	rendered, err := Render([]Entry{
		{
			Path:        "clear minimal",
			Mode:        "offline",
			Description: "Minimal offline command",
		},
		{
			Path:        "show wire-backed",
			Mode:        "daemon",
			Description: "Wire-backed command",
			WireMethod:  "show_wire_backed",
		},
	})
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	content := string(rendered)
	if count := strings.Count(content, "**Pipes:** not available"); count != 2 {
		t.Fatalf("canonical no-support verdict count = %d, want 2:\n%s", count, content)
	}
	if count := strings.Count(content, "This command runs offline."); count != 1 {
		t.Fatalf("offline explanation count = %d, want 1:\n%s", count, content)
	}
	_, wireDetail, found := strings.Cut(content, "### `show wire-backed`")
	if !found {
		t.Fatalf("wire-backed command has no detail:\n%s", content)
	}
	if strings.Contains(wireDetail, "offline") {
		t.Fatalf("wire-backed command was described as offline:\n%s", wireDetail)
	}
}

func TestRenderCurrentCatalogAnswersPipeSupportForEveryCommand(t *testing.T) {
	entries := Collect()
	rendered, err := Render(entries)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	zeroSupport := 0
	for _, entry := range entries {
		if len(entry.Operators) == 0 && len(entry.Pipes) == 0 &&
			len(entry.Aliases) == 0 {
			zeroSupport++
		}
	}
	content := string(rendered)
	if details := strings.Count(content, "\n### "); details != len(entries) {
		t.Fatalf("command detail count = %d, want %d", details, len(entries))
	}
	if verdicts := strings.Count(
		content, "\n**Pipes:** not available\n",
	); verdicts != zeroSupport {
		t.Fatalf("no-support verdict count = %d, want %d", verdicts, zeroSupport)
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
		{Path: "!!! route", Mode: "read-only", Description: "Empty ASCII anchor"},
		{Path: "`show` route", Mode: "read-only", Description: "Backtick"},
		{Path: "contents route", Mode: "read-only", Description: "Reserved"},
		{Path: "show! route", Mode: "read-only", Description: "Punctuation"},
		{Path: "show? route", Mode: "read-only", Description: "Collision"},
		{Path: "show-1 route", Mode: "read-only", Description: "Slug collision"},
		{Path: "表示 route", Mode: "read-only", Description: "Unicode"},
		{Path: "u--212121 route", Mode: "read-only", Description: "Reserved collision"},
		{Path: "！！！ route", Mode: "read-only", Description: "Empty Unicode anchor"},
	})
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	content := string(rendered)
	for _, want := range []string{
		"- [\\!\\!\\!](#u--212121) (1)",
		"- [\\`show\\`](#show) (1)",
		"- [contents](#contents-1) (1)",
		"- [show\\!](#show-1) (1)",
		"- [show\\?](#show-2) (1)",
		"- [show\\-1](#show-1-1) (1)",
		"## show\\-1",
		"- [表示](#表示) (1)",
		"- [u\\-\\-212121](#u--212121-1) (1)",
		"- [！！！](#u--efbc81efbc81efbc81) (1)",
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

// VALIDATES: a carriage return in either declared half is normalized, and the
// caller's own entries are left as they were.
//
// A summary is one line by declaration, so the line break this normalizes is
// the long form's. A summary that still carries one is joined with a space,
// because a Markdown table cell cannot hold a line break.
func TestRenderNormalizesDescriptionLineBreaks(t *testing.T) {
	entries := []Entry{
		{Path: "show crlf", Mode: "read-only", Description: "one line", LongHelp: "first\r\nsecond"},
		{Path: "clear cr", Mode: "offline", Description: "one line", LongHelp: "first\rsecond"},
		{Path: "set mixed", Mode: "offline", Description: "first\r\nsecond", LongHelp: "third\nfourth"},
	}
	rendered, err := Render(entries)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	content := string(rendered)
	for _, want := range []string{
		"| `show crlf` | read-only | one line |",
		"### `show crlf`\n\nfirst\nsecond\n\nMode: read-only",
		"| `clear cr` | offline | one line |",
		"### `clear cr`\n\nfirst\nsecond\n\nMode: offline",
		"| `set mixed` | offline | first second |",
		"### `set mixed`\n\nthird\nfourth\n\nMode: offline",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("normalized catalog omitted %q:\n%s", want, content)
		}
	}
	if strings.ContainsRune(content, '\r') {
		t.Fatalf("normalized catalog retained carriage returns:\n%q", content)
	}
	if entries[0].LongHelp != "first\r\nsecond" {
		t.Fatalf("Render() mutated caller-owned entries: %q", entries[0].LongHelp)
	}
}

// VALIDATES: the summary column takes the declared summary verbatim and the
// detail block takes the declared long form (AC-10).
//
// The method is to declare two halves that share no substring, then look for
// each half where its own surface renders it and refuse it on the other. A
// renderer that cut one authored string in two would put the same words in
// both places, so the two negative assertions are what discriminate.
func TestWikiCatalogRendersDeclaredSummary(t *testing.T) {
	entries := []Entry{
		{
			Path:        "show declared",
			Mode:        "read-only",
			Description: "Show what the node declares as its summary.",
			LongHelp:    "The explanation runs over two lines.\nIt reaches the detail block alone.",
		},
		{
			Path:        "show terse",
			Mode:        "read-only",
			Description: "Show a node that declares no long form.",
		},
	}
	rendered, err := Render(entries)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	content := string(rendered)

	for _, want := range []string{
		"| `show declared` | read-only | Show what the node declares as its summary\\. |",
		"### `show declared`\n\nThe explanation runs over two lines\\.\n" +
			"It reaches the detail block alone\\.\n\nMode: read-only",
		"| `show terse` | read-only | Show a node that declares no long form\\. |",
		"### `show terse`\n\nMode: read-only",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("the catalog omits %q:\n%s", want, content)
		}
	}

	summaryRow, _, found := strings.Cut(content, "### `show declared`")
	if !found {
		t.Fatalf("the catalog has no detail block for show declared:\n%s", content)
	}
	if strings.Contains(summaryRow, "detail block alone") {
		t.Error("the summary table carries the long form, which belongs to the detail block alone")
	}
	detail, _, _ := strings.Cut(content, "### `show terse`")
	_, detail, _ = strings.Cut(detail, "### `show declared`")
	if strings.Contains(detail, "declares as its summary") {
		t.Error("the detail block repeats the summary, which the table row already carries")
	}
}
