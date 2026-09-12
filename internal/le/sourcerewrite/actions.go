// Design: docs/architecture/core-design.md -- one command area, separate native actions
package sourcerewrite

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const area = "source-rewrite"

var actions = leaction.New(area,
	leaction.Action{
		Verb:       "rules-reformat",
		Why:        "bring ai/rules/*.md into the canonical metadata and Directives format",
		Writes:     true,
		Parameters: []leaction.Parameter{{Keyword: "dry-run"}},
		AnswerArgs: rulesReformatAnswer,
	},
	leaction.Action{
		Verb:   "reorder-attr-expectations",
		Why:    "permute committed BGP UPDATE attributes without changing their bytes",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: "files", Value: "comma-separated-paths", Requirement: leaction.Required},
			{Keyword: "check"},
			{Keyword: "write"},
		},
		AnswerArgs: reorderExpectationsAnswer,
	},
	leaction.Action{
		Verb:   "replace",
		Why:    "preview or apply one deterministic literal or regular-expression replacement",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: "file", Value: valuePath, Requirement: leaction.Required},
			{Keyword: "old", Value: "text", Requirement: leaction.Required},
			{Keyword: "new", Value: "text", Requirement: leaction.Required},
			{Keyword: "regex"}, {Keyword: "all"}, {Keyword: "apply"},
		},
		AnswerArgs: replaceAnswer,
	},
	leaction.Action{
		Verb:   "loc-activity",
		Why:    "render or serve the self-contained Git activity and Go source dashboard",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: "repo", Value: valuePath, Requirement: leaction.Optional},
			{Keyword: "days", Value: "count", Requirement: leaction.Optional},
			{Keyword: "output", Value: valuePath, Requirement: leaction.Optional},
			{Keyword: "ref", Value: "git-ref", Requirement: leaction.Optional},
			{Keyword: "all"},
			{Keyword: "all-files"},
			{Keyword: "extensions", Value: "comma-separated-extensions", Requirement: leaction.Optional},
			{Keyword: "exclude", Value: "comma-separated-patterns", Requirement: leaction.Optional},
			{Keyword: "author", Value: "git-author-regex", Requirement: leaction.Optional},
			{Keyword: "serve"},
			{Keyword: "address", Value: "host:port", Requirement: leaction.Optional},
			{Keyword: "open"},
		},
		AnswerArgs: activityAnswer,
	},
)

// Actions returns the four native source-maintenance workflows.
func Actions() leaction.List {
	return actions.Actions()
}

// Subs returns the action hint rendered by command help.
func Subs() string {
	return actions.Subs()
}

// Answer dispatches le source-rewrite through the closed action grammar.
func Answer(args []string) (any, int) {
	return actions.Answer(args)
}

func treeRoot() (string, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return "", 2
	}
	return root, 0
}

func rulesReformatAnswer(args leaction.Arguments) (any, int) {
	root, code := treeRoot()
	if code != 0 {
		return nil, code
	}
	report, err := reformatRules(root, args.Has("dry-run"))
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, 0
}

func reorderExpectationsAnswer(args leaction.Arguments) (any, int) {
	files := args.One("files")
	if files == "" {
		_, _ = fmt.Fprintln(os.Stderr, "error: files is required")
		return nil, 2
	}
	if args.Has("check") == args.Has("write") {
		_, _ = fmt.Fprintln(os.Stderr, "error: name exactly one of check or write")
		return nil, 2
	}
	root, code := treeRoot()
	if code != 0 {
		return nil, code
	}
	paths := splitComma(files)
	displayPaths := append([]string(nil), paths...)
	for index, path := range paths {
		if !filepath.IsAbs(path) {
			paths[index] = filepath.Join(root, filepath.FromSlash(path))
		}
	}
	report, err := reorderExpectationFiles(paths, args.Has("write"))
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	for index := range report.Files {
		report.Files[index].File = displayPaths[index]
	}
	for index, warning := range report.Warnings {
		for pathIndex, path := range paths {
			warning = strings.Replace(warning, path, displayPaths[pathIndex], 1)
		}
		report.Warnings[index] = warning
	}
	for _, warning := range report.Warnings {
		_, _ = fmt.Fprintln(os.Stderr, warning)
	}
	if report.LeftAlone > 0 {
		return report, 1
	}
	return report, 0
}

func replaceAnswer(args leaction.Arguments) (any, int) {
	file := args.One("file")
	old := args.One("old")
	newText := args.One("new")
	if !args.Has("file") || !args.Has("old") || !args.Has("new") {
		_, _ = fmt.Fprintln(os.Stderr, "error: file, old, and new are required")
		return nil, 2
	}
	root, code := treeRoot()
	if code != 0 {
		return nil, code
	}
	displayFile := file
	if !filepath.IsAbs(file) {
		file = filepath.Join(root, filepath.FromSlash(file))
	}
	report, err := replaceFile(file, old, newText, args.Has("regex"), args.Has("all"), args.Has("apply"))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		if isReplaceInputError(err) {
			return nil, 2
		}
		return nil, 1
	}
	report.File = displayFile
	report.Diff = strings.Replace(report.Diff, "a/"+file, "a/"+displayFile, 1)
	report.Diff = strings.Replace(report.Diff, "b/"+file, "b/"+displayFile, 1)
	if report.Count == 0 {
		_, _ = fmt.Fprintln(os.Stderr, "no changes")
		return nil, 1
	}
	_, _ = fmt.Fprintf(os.Stderr, "\n--- %d replacement(s) ---\n", report.Count)
	if report.Applied {
		_, _ = fmt.Fprintf(os.Stderr, "applied to %s\n", report.File)
	}
	return report, 0
}

func activityAnswer(args leaction.Arguments) (any, int) {
	root, code := treeRoot()
	if code != 0 {
		return nil, code
	}
	options := defaultActivityOptions(root)
	if args.Has("repo") {
		options.Repo = args.One("repo")
	}
	if args.Has("days") {
		days, err := strconv.Atoi(args.One("days"))
		if err != nil || days <= 0 {
			_, _ = fmt.Fprintln(os.Stderr, "error: --days must be positive")
			return nil, 2
		}
		options.Days = days
	}
	if args.Has("output") {
		options.Output = args.One("output")
	}
	if args.Has("ref") {
		options.Ref = args.One("ref")
	}
	options.AllRefs = args.Has("all")
	options.AllFiles = args.Has("all-files")
	if args.Has("extensions") {
		extensions, err := parseExtensions(args.One("extensions"))
		if err != nil {
			leaction.ReportError(err)
			return nil, 2
		}
		options.Extensions = extensions
	}
	if args.Has("exclude") {
		options.Excludes = append(options.Excludes, splitComma(args.One("exclude"))...)
	}
	if args.Has("author") {
		options.Author = args.One("author")
	}
	options.Open = args.Has("open")
	if args.Has("address") && !args.Has("serve") {
		_, _ = fmt.Fprintln(os.Stderr, "error: address requires serve")
		return nil, 2
	}
	if args.Has("serve") {
		address := defaultActivityServe
		if args.Has("address") {
			address = args.One("address")
		}
		if err := serveActivity(options, address); err != nil {
			leaction.ReportError(err)
			return nil, 1
		}
		return nil, 0
	}
	report, err := writeActivity(options)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, 0
}

func splitComma(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
