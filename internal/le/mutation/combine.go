package mutation

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	combinedReportRel = "tmp/mutation-report.json"
	reportPatternRel  = "tmp/mutation-report-*.json"
)

// CombineReport is the structured answer from `le mutation combine`.
type CombineReport struct {
	Reports   []string `json:"reports"`
	Output    string   `json:"output,omitempty"`
	Total     int      `json:"total"`
	Killed    int      `json:"killed"`
	Survived  int      `json:"survived"`
	Score     float64  `json:"score"`
	Generated bool     `json:"generated"`
}

// Text preserves the two messages mutation_combine.py printed.
func (r CombineReport) Text() string {
	if !r.Generated {
		return "No reports generated\n"
	}
	var output textbuf.Buffer
	return output.Str("Combined: ").Int(int64(r.Killed)).Byte('/').Int(int64(r.Total)).
		Str(" killed (").Str(pythonScore(r.Killed, r.Total)).Str("%), ").Int(int64(r.Survived)).Str(" survived\n").String()
}

// Combine discovers every package report, combines it in lexical filename
// order, publishes the complete report, and only then removes its inputs.
func Combine(root string) (CombineReport, error) {
	paths, err := filepath.Glob(filepath.Join(root, reportPatternRel))
	if err != nil {
		return CombineReport{}, fmt.Errorf("discover mutation reports: %w", err)
	}
	slices.Sort(paths)
	if len(paths) == 0 {
		return CombineReport{Reports: []string{}}, nil
	}

	results := make([]jsonValue, 0)
	reports := make([]string, 0, len(paths))
	for _, path := range paths {
		values, err := readResults(path)
		if err != nil {
			return CombineReport{}, err
		}
		results = append(results, values...)
		reports = append(reports, relativeSlash(root, path))
	}

	killed := 0
	for index, result := range results {
		if result.kind != jsonObject {
			return CombineReport{}, fmt.Errorf("mutation result %d is not a JSON object", index+1)
		}
		status, ok := result.member("status")
		if !ok {
			return CombineReport{}, fmt.Errorf("mutation result %d has no status", index+1)
		}
		if status.kind != jsonString || status.text != "SURVIVED" {
			killed++
		}
	}
	survived := len(results) - killed
	score := pythonScore(killed, len(results))
	scoreValue, err := strconv.ParseFloat(score, 64)
	if err != nil {
		return CombineReport{}, fmt.Errorf("parse mutation score %q: %w", score, err)
	}

	document := jsonValue{kind: jsonObject, members: []jsonMember{
		{name: "results", value: jsonValue{kind: jsonArray, values: results}},
		{name: "summary", value: jsonValue{kind: jsonObject, members: []jsonMember{
			{name: "total", value: jsonValue{kind: jsonNumber, text: fmt.Sprint(len(results))}},
			{name: "killed", value: jsonValue{kind: jsonNumber, text: fmt.Sprint(killed)}},
			{name: "survived", value: jsonValue{kind: jsonNumber, text: fmt.Sprint(survived)}},
			{name: "score", value: jsonValue{kind: jsonNumber, text: score}},
		}}},
	}}
	outputPath := filepath.Join(root, combinedReportRel)
	if err := writeAtomic(outputPath, marshalIndented(document)); err != nil {
		return CombineReport{}, err
	}

	report := CombineReport{
		Reports: reports, Output: combinedReportRel, Total: len(results), Killed: killed,
		Survived: survived, Score: scoreValue, Generated: true,
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			return report, fmt.Errorf("remove combined mutation report %s: %w", relativeSlash(root, path), err)
		}
	}
	return report, nil
}

func readResults(path string) ([]jsonValue, error) {
	document, err := readReport(path)
	if err != nil {
		return nil, err
	}
	return resultsOf(document, path)
}

func readReport(path string) (jsonValue, error) {
	content, err := os.ReadFile(path) //nolint:gosec // the operator selected this local report
	if err != nil {
		return jsonValue{}, fmt.Errorf("read mutation report %s: %w", path, err)
	}
	document, err := decodeJSON(content)
	if err != nil {
		return jsonValue{}, fmt.Errorf("decode mutation report %s: %w", path, err)
	}
	return document, nil
}

func resultsOf(document jsonValue, path string) ([]jsonValue, error) {
	if document.kind != jsonObject {
		return nil, fmt.Errorf("mutation report %s is not a JSON object", path)
	}
	results, present := document.member("results")
	if !present {
		return []jsonValue{}, nil
	}
	if results.kind != jsonArray {
		return nil, fmt.Errorf("mutation report %s results is not an array", path)
	}
	return results.values, nil
}

func relativeSlash(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}
