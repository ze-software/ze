package wikicatalog

import (
	"bytes"
	"os"
	"path/filepath"
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
