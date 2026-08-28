package mutation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCombineNoReportsLeavesExistingOutputUntouched(t *testing.T) {
	root := t.TempDir()
	mustWriteMutationFile(t, filepath.Join(root, combinedReportRel), []byte("old report"))

	report, err := Combine(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.Generated || report.Text() != "No reports generated\n" {
		t.Fatalf("no-report answer = %#v, text %q", report, report.Text())
	}
	if len(report.Reports) != 0 || report.Score != 0 {
		t.Fatalf("no-report structured answer = %#v", report)
	}
	got, err := os.ReadFile(filepath.Join(root, combinedReportRel))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old report" {
		t.Fatalf("no-report run changed existing output to %q", got)
	}
}

func TestCombineOrdersReportsPreservesResultsAndPinsOutputBytes(t *testing.T) {
	root := t.TempDir()
	mustWriteMutationFile(t, filepath.Join(root, "tmp/mutation-report-z.json"), []byte(`{"results":[{"status":"KILLED","n":1.2300}]}`))
	mustWriteMutationFile(t, filepath.Join(root, "tmp/mutation-report-a.json"), []byte(`{"results":[{"status":"SURVIVED","label":"é","mutant":{"filePath":"internal/a.go"}}]}`))

	report, err := Combine(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.Text() != "Combined: 1/2 killed (50.0%), 1 survived\n" {
		t.Fatalf("combined text = %q", report.Text())
	}
	if strings.Join(report.Reports, ",") != "tmp/mutation-report-a.json,tmp/mutation-report-z.json" {
		t.Fatalf("report discovery order = %v", report.Reports)
	}
	if report.Output != combinedReportRel || report.Total != 2 || report.Killed != 1 || report.Survived != 1 || report.Score != 50 || !report.Generated {
		t.Fatalf("combined structured answer = %#v", report)
	}

	got, err := os.ReadFile(filepath.Join(root, combinedReportRel))
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n" +
		"  \"results\": [\n" +
		"    {\n" +
		"      \"status\": \"SURVIVED\",\n" +
		"      \"label\": \"\\u00e9\",\n" +
		"      \"mutant\": {\n" +
		"        \"filePath\": \"internal/a.go\"\n" +
		"      }\n" +
		"    },\n" +
		"    {\n" +
		"      \"status\": \"KILLED\",\n" +
		"      \"n\": 1.23\n" +
		"    }\n" +
		"  ],\n" +
		"  \"summary\": {\n" +
		"    \"total\": 2,\n" +
		"    \"killed\": 1,\n" +
		"    \"survived\": 1,\n" +
		"    \"score\": 50.0\n" +
		"  }\n" +
		"}"
	if string(got) != want {
		t.Fatalf("combined bytes differ\n got: %q\nwant: %q", got, want)
	}
	for _, input := range []string{"tmp/mutation-report-a.json", "tmp/mutation-report-z.json"} {
		if _, err := os.Stat(filepath.Join(root, input)); !os.IsNotExist(err) {
			t.Fatalf("combined input %s still exists or stat failed: %v", input, err)
		}
	}
}

func TestCombineFailureKeepsEveryInputAndPublishedReport(t *testing.T) {
	root := t.TempDir()
	paths := []string{"tmp/mutation-report-a.json", "tmp/mutation-report-z.json"}
	mustWriteMutationFile(t, filepath.Join(root, paths[0]), []byte(`{"results":[{"status":"KILLED"}]}`))
	mustWriteMutationFile(t, filepath.Join(root, paths[1]), []byte(`{"results":[`))
	mustWriteMutationFile(t, filepath.Join(root, combinedReportRel), []byte("previous"))

	_, err := Combine(root)
	if err == nil || !strings.Contains(err.Error(), "decode mutation report") {
		t.Fatalf("malformed report error = %v", err)
	}
	for _, input := range paths {

		if _, err := os.Stat(filepath.Join(root, input)); err != nil {
			t.Fatalf("failed combine removed %s: %v", input, err)
		}
	}
	got, err := os.ReadFile(filepath.Join(root, combinedReportRel))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "previous" {
		t.Fatalf("failed combine replaced published report with %q", got)
	}
}
func TestCombineReportsWithNoResultsUseIntegerZeroScore(t *testing.T) {
	root := t.TempDir()
	mustWriteMutationFile(t, filepath.Join(root, "tmp/mutation-report-empty.json"), []byte(`{}`))

	report, err := Combine(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.Text() != "Combined: 0/0 killed (0%), 0 survived\n" {
		t.Fatalf("empty combined text = %q", report.Text())
	}
	got, err := os.ReadFile(filepath.Join(root, combinedReportRel))
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n" +
		"  \"results\": [],\n" +
		"  \"summary\": {\n" +
		"    \"total\": 0,\n" +
		"    \"killed\": 0,\n" +
		"    \"survived\": 0,\n" +
		"    \"score\": 0\n" +
		"  }\n" +
		"}"
	if string(got) != want {
		t.Fatalf("empty combined bytes = %q, want %q", got, want)
	}
}

func TestCombineRejectsResultWithoutStatusBeforeCleanup(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "tmp/mutation-report-a.json")
	mustWriteMutationFile(t, input, []byte(`{"results":[{}]}`))

	_, err := Combine(root)
	if err == nil || !strings.Contains(err.Error(), "mutation result 1 has no status") {
		t.Fatalf("missing status error = %v", err)
	}
	if _, err := os.Stat(input); err != nil {
		t.Fatalf("failed combine removed its input: %v", err)
	}
}

func mustWriteMutationFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}
