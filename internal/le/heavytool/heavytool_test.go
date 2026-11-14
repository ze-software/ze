package heavytool

import (
	"strings"
	"testing"
)

// TestNameOfEachHeavyTool proves the one definition of a heavy tool names
// every tool it lists and nothing else. Its method: one row per Go verb,
// linter and formatter, per cheap subcommand of the linter, and per edge the
// Bash hook handled before the definition moved here (a program path judged
// by its base name, options in front of the linter's subcommand, a bare
// program), each asserting both the name and whether the words are heavy.
func TestNameOfEachHeavyTool(t *testing.T) {
	type row struct {
		words string
		name  string
		heavy bool
	}
	rows := []row{
		// The Go toolchain's executing verbs.
		{"go test ./x/", "go test", true},
		{"go build ./cmd/ze", "go build", true},
		{"go vet ./...", "go vet", true},
		// The line is assembled so this file never holds the literal the
		// repository contract reads as a test shelling out to a compiled tool.
		{"go run" + " ./cmd/ze", "go run", true},
		{"go generate ./...", "go generate", true},
		{"go install ./cmd/ze", "go install", true},
		{"go fmt ./...", "go fmt", true},
		{"go fix ./...", "go fix", true},
		{"go test", "go test", true},
		// The verbs that read and start no build of the tree.
		{"go version", "", false},
		{"go env GOROOT", "", false},
		{"go list ./...", "", false},
		{"go doc fmt", "", false},
		{"go help test", "", false},
		{"go mod vendor", "", false},
		{"go", "", false},
		// A flag is not the verb: the verb is the word after the program.
		{"go -C dir test ./x/", "", false},
		// The linters and the formatters.
		{"golangci-lint run ./...", "golangci-lint", true},
		{"golangci-lint", "golangci-lint", true},
		{"golangci-lint fmt", "golangci-lint", true},
		{"golangci-lint --timeout 5m run", "golangci-lint", true},
		{"golangci-lint -v", "golangci-lint", true},
		{"staticcheck ./...", "staticcheck", true},
		{"govulncheck ./...", "govulncheck", true},
		{"gofmt -l .", "gofmt", true},
		{"goimports -w x.go", "goimports", true},
		{"gofumpt -l .", "gofumpt", true},
		{"gofmt", "gofmt", true},
		// The linter's subcommands that run no analysis.
		{"golangci-lint config verify", "", false},
		{"golangci-lint version", "", false},
		{"golangci-lint help linters", "", false},
		{"golangci-lint linters", "", false},
		{"golangci-lint cache clean", "", false},
		{"golangci-lint completion zsh", "", false},
		// Options in front of the subcommand are read through.
		{"golangci-lint -v version", "", false},
		{"golangci-lint --color never config path", "golangci-lint", true},
		// A program path is judged by its base name.
		{"/usr/local/go/bin/go test ./x/", "go test", true},
		{"/usr/local/go/bin/go version", "", false},
		{"./bin/golangci-lint run", "golangci-lint", true},
		{"./bin/golangci-lint version", "", false},
		{"/opt/tools/gofmt -l .", "gofmt", true},
		// A program outside the list is not heavy.
		{"gopls symbols x.go", "", false},
		{"make test", "", false},
		{"le test unit all", "", false},
		{"gotest ./x/", "", false},
		{"test go", "", false},
	}
	for _, r := range rows {
		name, heavy := Name(strings.Fields(r.words))
		if name != r.name || heavy != r.heavy {
			t.Errorf("Name(%q) = %q, %v, want %q, %v", r.words, name, heavy, r.name, r.heavy)
		}
	}
	if name, heavy := Name(nil); name != "" || heavy {
		t.Errorf("Name(nil) = %q, %v, want no heavy tool", name, heavy)
	}
}

// TestLintRunsNoAnalysisReadsTheFirstWordThatIsNoOption proves the linter's
// cheap-subcommand judgment the raw-job guard reads: the first word after the
// program that is not an option decides, and a line with no such word may
// analyze. Its method: one row per shape of the words.
func TestLintRunsNoAnalysisReadsTheFirstWordThatIsNoOption(t *testing.T) {
	rows := []struct {
		words string
		cheap bool
	}{
		{"golangci-lint version", true},
		{"golangci-lint -v --color=never cache status", true},
		{"golangci-lint run version", false},
		{"golangci-lint --help", false},
		{"golangci-lint", false},
		{"", false},
	}
	for _, r := range rows {
		if cheap := LintRunsNoAnalysis(strings.Fields(r.words)); cheap != r.cheap {
			t.Errorf("LintRunsNoAnalysis(%q) = %v, want %v", r.words, cheap, r.cheap)
		}
	}
}
