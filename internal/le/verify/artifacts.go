// Design: docs/functional-tests.md -- published verification failure protocol
package verify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// CombinedLogPath is the stable reader-facing log of the latest run.
	CombinedLogPath = "tmp/ze-verify.log"
	// FailuresLogPath is the stable human failure index of the latest run.
	FailuresLogPath = "tmp/ze-verify-failures.log"
	// FailuresJSONPath is the stable machine failure index of the latest run.
	FailuresJSONPath = "tmp/ze-verify-failures.json"
	// FullJSONPath preserves the latest full-mode machine index when a changed
	// run publishes its cheaper result.
	FullJSONPath = "tmp/ze-verify-full.json"
)

type artifactGroup struct {
	Stage     string   `json:"stage"`
	GroupID   string   `json:"group-id"`
	Kind      string   `json:"kind"`
	Related   []string `json:"related"`
	Summary   string   `json:"summary"`
	Rerun     string   `json:"rerun"`
	DetailLog string   `json:"detail-log"`
	Parallel  string   `json:"parallel"`
	Excerpt   []string `json:"excerpt,omitempty"`
}

type artifactStage struct {
	Stage     string          `json:"stage"`
	ExitCode  int             `json:"exit-code"`
	DetailLog string          `json:"detail-log"`
	Groups    []artifactGroup `json:"groups,omitempty"`
}

type artifactIndex struct {
	Mode        string          `json:"mode"`
	ExitCode    int             `json:"exit-code"`
	RunDir      string          `json:"run-dir"`
	CombinedLog string          `json:"combined-log"`
	GeneratedAt string          `json:"generated-at"`
	Stages      []artifactStage `json:"stages"`
}

func writeRunArtifacts(root string, report Report, at time.Time) error {
	combinedLog := filepath.ToSlash(filepath.Join(report.LogDir, "ze-verify.log"))
	index := artifactIndex{
		Mode: report.Mode, ExitCode: report.Code, RunDir: report.LogDir,
		CombinedLog: combinedLog, GeneratedAt: at.UTC().Format(time.RFC3339),
		Stages: make([]artifactStage, 0, len(report.Stages)),
	}
	for _, stage := range report.Stages {
		result := artifactStage{
			Stage: stage.Identity.Name, ExitCode: stage.Code, DetailLog: stage.Log,
		}
		if stage.Code != 0 {
			if declared, complete := declaredGroups(root, stage); complete {
				result.Groups = declared
			} else {
				summary := fmt.Sprintf("stage exited %d", stage.Code)
				if stage.Failure != nil && stage.Failure.Message != "" {
					summary = stage.Failure.Message
				}
				result.Groups = []artifactGroup{{
					Stage: stage.Identity.Name, GroupID: "stage:" + stage.Identity.Name,
					Kind: "generic", Related: []string{}, Summary: summary,
					Rerun: "le " + invocation(stage.Identity), DetailLog: stage.Log,
					Parallel: "independent; rerun this stage alone",
				}}
			}
		}
		index.Stages = append(index.Stages, result)
	}
	failureText := failureIndexText(index)
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal verify failure index: %w", err)
	}
	data = append(data, '\n')
	for _, artifact := range []struct {
		path    string
		content []byte
	}{
		{CombinedLogPath, []byte(report.Console)},
		{filepath.ToSlash(filepath.Join("tmp", "verify", "ze-verify.log")), []byte(report.Console)},
		{FailuresLogPath, []byte(failureText)},
		{FailuresJSONPath, data},
	} {
		if err := atomicWrite(root, artifact.path, artifact.content, 0o600); err != nil {
			return fmt.Errorf("write verify artifact %s: %w", artifact.path, err)
		}
	}
	if report.Mode == Mode {
		if err := atomicWrite(root, FullJSONPath, data, 0o600); err != nil {
			return fmt.Errorf("write full verify artifact: %w", err)
		}
	}
	return nil
}

func declaredGroups(root string, stage StageReport) ([]artifactGroup, bool) {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(stage.Log)))
	if err != nil {
		return nil, false
	}
	var groups []artifactGroup
	var counts []string
	for _, line := range strings.Split(string(content), "\n") {
		if _, payload, found := strings.Cut(line, "VERIFY FAILURE GROUP:"); found {
			var group artifactGroup
			if err := json.Unmarshal([]byte(strings.TrimSpace(payload)), &group); err != nil {
				groups = append(groups, artifactGroup{
					GroupID: "unparsed-group:" + strconv.Itoa(len(groups)), Kind: "unparsed",
					Related:   []string{"unparsed-group"},
					Summary:   "a declared failure group line did not parse: " + err.Error(),
					DetailLog: stage.Log, Parallel: "independent; rerun this group alone",
					Excerpt: []string{line},
				})
				continue
			}
			if group.DetailLog == "" {
				group.DetailLog = stage.Log
			}
			if group.Parallel == "" {
				group.Parallel = "independent; rerun this group alone"
			}
			groups = append(groups, group)
			continue
		}
		if _, count, found := strings.Cut(line, "VERIFY FAILURE GROUPS COMPLETE:"); found {
			counts = append(counts, strings.TrimSpace(count))
		}
	}
	if len(counts) != 1 {
		return groups, false
	}
	count, err := strconv.Atoi(counts[0])
	return groups, err == nil && count == len(groups)
}

func failureIndexText(index artifactIndex) string {
	var text strings.Builder
	fmt.Fprintf(&text, "# Ze verify failure index\n")
	fmt.Fprintf(&text, "Generated: %s\n", index.GeneratedAt)
	fmt.Fprintf(&text, "Mode: %s\n", index.Mode)
	fmt.Fprintf(&text, "Run directory: %s\n", index.RunDir)
	fmt.Fprintf(&text, "Combined log: %s\n\n", index.CombinedLog)
	failed := 0
	for _, stage := range index.Stages {
		if stage.ExitCode == 0 {
			continue
		}
		failed++
		fmt.Fprintf(&text, "## Stage: %s\n", stage.Stage)
		fmt.Fprintf(&text, "Exit: %d\n", stage.ExitCode)
		fmt.Fprintf(&text, "Detail log: %s\n\n", stage.DetailLog)
		for _, group := range stage.Groups {
			fmt.Fprintf(&text, "### Group: %s\n", group.GroupID)
			fmt.Fprintf(&text, "Stage: %s\n", group.Stage)
			fmt.Fprintf(&text, "Kind: %s\n", group.Kind)
			fmt.Fprintf(&text, "Related: %s\n", strings.Join(group.Related, ", "))
			fmt.Fprintf(&text, "Summary: %s\n", group.Summary)
			fmt.Fprintf(&text, "Rerun: %s\n", group.Rerun)
			fmt.Fprintf(&text, "Detail log: %s\n", group.DetailLog)
			fmt.Fprintf(&text, "Parallel: %s\n\n", group.Parallel)
		}
	}
	if failed == 0 {
		text.WriteString("No failures.\n")
	}
	return text.String()
}
