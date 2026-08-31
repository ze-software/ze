// Design: website/AI.md -- one native command surface owns site build tools
package site

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/sourcerewrite"
)

const area = "site"

var actions = leaction.New(area,
	leaction.Action{Verb: "build", Why: "stage website sources and refresh the Pages artifact with native renderers", Writes: true,
		Parameters: []leaction.Parameter{{Keyword: keywordOutput, Value: "directory"}, {Keyword: "partial"}}, AnswerArgs: runBuild},
	leaction.Action{Verb: "check", Why: "verify that the Pages artifact contains no source-only website inputs", Answer: runCheck},
	leaction.Action{Verb: "bundle", Why: "turn one presentation deck into a self-contained HTML file", Writes: true,
		Parameters: []leaction.Parameter{{Keyword: keywordInput, Value: "html-file"}}, AnswerArgs: runBundle},
	leaction.Action{Verb: "activity", Why: "render repository line and commit activity as a presentation-ready HTML page", Writes: true,
		Parameters: []leaction.Parameter{{Keyword: keywordOutput, Value: "html-file"}, {Keyword: "days", Value: "count"}, {Keyword: "ref", Value: "revision"}, {Keyword: "today", Value: "date"}}, AnswerArgs: runActivity},
	leaction.Action{Verb: "update-talk", Why: "refresh one talk's live statistics, activity page, and standalone deck", Writes: true,
		Parameters: []leaction.Parameter{{Keyword: "talk", Value: "slug"}, {Keyword: "bundle-only"}}, AnswerArgs: runUpdateTalk},
	leaction.Action{Verb: "config-tree", Why: "extract the live YANG configuration tree for the public configuration reference", Writes: true,
		Parameters: []leaction.Parameter{{Keyword: keywordOutput, Value: "directory"}, {Keyword: "binary", Value: "ze-binary"}}, AnswerArgs: runConfigTree},
)

func Actions() leaction.List          { return actions.Actions() }
func Subs() string                    { return actions.Subs() }
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runBuild(arguments leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, err := Build(BuildOptions{Repository: root, Output: arguments[keywordOutput], Partial: arguments.Has("partial")})
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, 0
}

type checkReport struct {
	Output string `json:"output"`
	// MissingArtifacts names the published files that are not routes and are
	// absent. The coverage arithmetic answers for pages; nothing else answers
	// for these.
	MissingArtifacts []string `json:"missing-artifacts,omitempty"`
	SourceOnly       []string `json:"source-only,omitempty"`
	MissingMirrors   []string `json:"missing-mirrors,omitempty"`
	Coverage         Coverage `json:"coverage"`
}

// exit answers the status one check reports. An artifact is refused when it
// carries a source-only input, when a public route has no Markdown mirror, when
// a named non-route artifact is absent, and when a published route has no
// producer or has two.
func (report checkReport) exit() int {
	if len(report.SourceOnly) != 0 || len(report.MissingMirrors) != 0 {
		return 1
	}
	if len(report.MissingArtifacts) != 0 || report.Coverage.Red() {
		return 1
	}
	return 0
}

func runCheck() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	paths, err := resolvePaths(root, "")
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report := checkReport{Output: paths.Output}
	err = filepath.WalkDir(paths.Output, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == paths.Output {
			return nil
		}
		relative, relErr := filepath.Rel(paths.Output, path)
		if relErr != nil {
			return relErr
		}
		if isSourceOnly(relative) {
			report.SourceOnly = append(report.SourceOnly, filepath.ToSlash(relative))
			if entry.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report.MissingArtifacts = checkNamedArtifacts(paths.Output)
	report.MissingMirrors, err = checkPageMirrors(paths.Output)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report.Coverage, err = checkCoverage(paths)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, report.exit()
}

func runBundle(arguments leaction.Arguments) (any, int) {
	output, err := bundlePresentation(arguments["input"])
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return map[string]string{keywordOutput: output}, 0
}

func runActivity(arguments leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	days := sourcerewrite.ActivityDaysDefault
	if raw := arguments["days"]; raw != "" {
		days, err = strconv.Atoi(raw)
		if err != nil {
			leaction.ReportError(fmt.Errorf("invalid days %q: %w", raw, err))
			return nil, 1
		}
	}
	today := time.Now().UTC()
	if raw := arguments["today"]; raw != "" {
		today, err = time.Parse("2006-01-02", raw)
		if err != nil {
			leaction.ReportError(fmt.Errorf("invalid today %q: %w", raw, err))
			return nil, 1
		}
	}
	output := arguments[keywordOutput]
	if output == "" {
		output = filepath.Join(root, "tmp", "code-activity.html")
	}
	err = renderActivity(ActivityOptions{Repository: root, Ref: arguments["ref"], Output: output, Days: days, Today: today})
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return map[string]string{keywordOutput: output}, 0
}

func runUpdateTalk(arguments leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	slug := arguments["talk"]
	if slug == "" || slug == "." || slug == ".." || filepath.Base(slug) != slug {
		leaction.ReportError(fmt.Errorf("talk must name one directory under website/talks"))
		return nil, 1
	}
	report, err := updateTalk(talkUpdateOptions{
		Repository: root,
		Directory:  filepath.Join(root, "website", "talks", slug),
		BundleOnly: arguments.Has("bundle-only"),
	})
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, 0
}

func runConfigTree(arguments leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	paths, err := resolvePaths(root, arguments[keywordOutput])
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	count, err := extractYANGConfigTree(root, paths.Output, arguments["binary"])
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return map[string]any{
		"file":  filepath.Join(paths.Output, "data", "yang-config-tree.json"),
		"roots": count,
	}, 0
}
