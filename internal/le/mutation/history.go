package mutation

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	defaultReportRel = "tmp/mutation-report.json"
	historyRel       = "test/mutation/history.ndjson"
)

type packageStats struct {
	Mutants int
	Killed  int
}

// PackageHistory is one package sample appended to the mutation history.
type PackageHistory struct {
	Package string  `json:"package"`
	Mutants int     `json:"mutants"`
	Killed  int     `json:"killed"`
	Score   float64 `json:"score"`
}

// historyReport is the structured answer from `le mutation record-history`.
type historyReport struct {
	History    string           `json:"history"`
	Recorded   int              `json:"recorded"`
	Packages   []PackageHistory `json:"packages"`
	CannotRead string           `json:"cannot_read,omitempty"`
}

// Text preserves mutation_history.py's successful stdout. An advisory read
// failure belongs on stderr and is emitted by the action boundary.
func (r historyReport) Text() string {
	if r.CannotRead != "" {
		return ""
	}
	if r.Recorded == 0 {
		return "mutation history: no results in report, nothing recorded\n"
	}
	var output textbuf.Buffer
	return output.Str("mutation history: recorded ").Int(int64(r.Recorded)).
		Str(" package(s) in ").Str(r.History).Byte('\n').String()
}

type gitRunner func(dir string, argv ...string) (string, error)

type historyRecorder struct {
	runGit gitRunner
	now    func() time.Time
}

func productionRecorder() historyRecorder {
	return historyRecorder{runGit: runGit, now: time.Now}
}

func runGit(dir string, argv ...string) (string, error) {
	command := exec.Command("git", argv...) //nolint:gosec,noctx // argv is the producer's two fixed repository queries
	if dir != "" {
		command.Dir = dir
	}
	output, err := command.Output()
	return strings.TrimSpace(string(output)), err
}

// recordHistory preserves the optional report path accepted by the producer.
func recordHistory(reportPath string) (historyReport, error) {
	return productionRecorder().record(reportPath)
}

func (recorder historyRecorder) record(reportPath string) (historyReport, error) {
	root, err := recorder.runGit("", "rev-parse", "--show-toplevel")
	if err != nil {
		return historyReport{}, fmt.Errorf("discover repository root: %w", err)
	}
	if root == "" {
		return historyReport{}, fmt.Errorf("discover repository root: git returned an empty path")
	}
	if reportPath == "" {
		reportPath = defaultReportRel
	}
	openPath := reportPath
	if !filepath.IsAbs(openPath) {
		openPath = filepath.Join(root, openPath)
	}
	document, err := readReport(openPath)
	if err != nil {
		return historyReport{
			History: historyRel, Packages: []PackageHistory{},
			CannotRead: fmt.Sprintf("mutation history: cannot read %s: %v", reportPath, unwrapReadError(err)),
		}, nil
	}
	values, err := resultsOf(document, openPath)
	if err != nil {
		return historyReport{}, err
	}

	stats, err := collectPackages(values, root)
	if err != nil {
		return historyReport{}, err
	}
	if len(stats) == 0 {
		return historyReport{History: historyRel, Packages: []PackageHistory{}}, nil
	}

	sha, err := recorder.runGit(root, "rev-parse", "--short", "HEAD")
	if err != nil || sha == "" {
		sha = "unknown"
	}
	timestamp := recorder.now().UTC().Format("2006-01-02T15:04:05Z")
	packages := make([]string, 0, len(stats))
	for name := range stats {
		packages = append(packages, name)
	}
	slices.Sort(packages)

	var appended bytes.Buffer
	reportPackages := make([]PackageHistory, 0, len(packages))
	for _, name := range packages {
		entry := stats[name]
		scoreText := pythonScore(entry.Killed, entry.Mutants)
		appendHistoryLine(&appended, timestamp, sha, name, entry, scoreText)
		score, err := strconv.ParseFloat(scoreText, 64)
		if err != nil {
			return historyReport{}, fmt.Errorf("parse mutation score %q: %w", scoreText, err)
		}
		reportPackages = append(reportPackages, PackageHistory{
			Package: name, Mutants: entry.Mutants, Killed: entry.Killed, Score: score,
		})
	}

	historyPath := filepath.Join(root, historyRel)
	if err := os.MkdirAll(filepath.Dir(historyPath), 0o750); err != nil {
		return historyReport{}, fmt.Errorf("create mutation history directory: %w", err)
	}
	existing, err := os.ReadFile(historyPath) //nolint:gosec // committed local history
	if err != nil && !os.IsNotExist(err) {
		return historyReport{}, fmt.Errorf("read mutation history: %w", err)
	}
	content := make([]byte, 0, len(existing)+appended.Len())
	content = append(content, existing...)
	content = append(content, appended.Bytes()...)
	if err := writeAtomic(historyPath, content); err != nil {
		return historyReport{}, err
	}
	return historyReport{
		History: historyRel, Recorded: len(packages), Packages: reportPackages,
	}, nil
}

func unwrapReadError(err error) error {
	for {
		unwrapped, ok := err.(interface{ Unwrap() error })
		if !ok {
			return err
		}
		next := unwrapped.Unwrap()
		if next == nil {
			return err
		}
		err = next
	}
}

func collectPackages(results []jsonValue, root string) (map[string]packageStats, error) {
	stats := make(map[string]packageStats)
	for index, result := range results {
		if result.kind != jsonObject {
			return nil, fmt.Errorf("mutation result %d is not a JSON object", index+1)
		}
		filePath, err := mutantFilePath(result)
		if err != nil {
			return nil, fmt.Errorf("mutation result %d: %w", index+1, err)
		}
		name := packageOf(filePath, root)
		entry := stats[name]
		entry.Mutants++
		status, present := result.member("status")
		if !present || status.kind != jsonString || status.text != "SURVIVED" {
			entry.Killed++
		}
		stats[name] = entry
	}
	return stats, nil
}

func mutantFilePath(result jsonValue) (string, error) {
	mutant, present := result.member("mutant")
	if !present || !pythonTruthy(mutant) {
		return "", nil
	}
	if mutant.kind != jsonObject {
		return "", fmt.Errorf("mutant is not a JSON object")
	}
	filePath, present := mutant.member("filePath")
	if !present {
		return "", nil
	}
	if filePath.kind != jsonString {
		return "", fmt.Errorf("mutant filePath is not a string")
	}
	return filePath.text, nil
}

func pythonTruthy(value jsonValue) bool {
	switch value.kind {
	case jsonNull:
		return false
	case jsonBool:
		return value.boolean
	case jsonNumber:
		return value.text != "0" && value.text != "0.0" && value.text != "-0.0"
	case jsonString:
		return value.text != ""
	case jsonArray:
		return len(value.values) != 0
	case jsonObject:
		return len(value.members) != 0
	default:
		return false
	}
}

func packageOf(filePath, root string) string {
	relative := filePath
	if rest, ok := strings.CutPrefix(relative, root); ok {
		relative = strings.TrimLeft(rest, "/")
	}
	name := posixDirname(relative)
	if name == "" {
		return "."
	}
	return name
}

// posixDirname is the lexical os.path.dirname used by the Python producer.
// filepath.Dir cleans `..` components, which would silently merge two package
// keys that the report recorded separately.
func posixDirname(path string) string {
	lastSlash := strings.LastIndexByte(path, '/')
	if lastSlash < 0 {
		return ""
	}
	head := path[:lastSlash+1]
	if strings.Trim(head, "/") != "" {
		head = strings.TrimRight(head, "/")
	}
	return head
}

func appendHistoryLine(output *bytes.Buffer, timestamp, sha, name string, stats packageStats, score string) {
	output.WriteByte('{')
	output.WriteString("\"ts\":")
	appendPythonString(output, timestamp)
	output.WriteString(",\"sha\":")
	appendPythonString(output, sha)
	output.WriteString(",\"package\":")
	appendPythonString(output, name)
	fmt.Fprintf(output, ",\"mutants\":%d,\"killed\":%d,\"score\":%s}\n", stats.Mutants, stats.Killed, score)
}
