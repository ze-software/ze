package mutation

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecordHistoryAppendsSortedPackagesWithFixedClockAndSHA(t *testing.T) {
	root := t.TempDir()
	reportPath := filepath.Join(root, "tmp/report.json")
	absoluteB := filepath.ToSlash(filepath.Join(root, "internal/b/b.go"))
	mustWriteMutationFile(t, reportPath, []byte(`{"results":[`+
		`{"status":"SURVIVED"},`+
		`{"status":"SURVIVED","mutant":{"filePath":"`+absoluteB+`"}},`+
		`{"status":"KILLED","mutant":{"filePath":"internal/a/a.go"}},`+
		`{"status":"TIMED_OUT","mutant":{"filePath":"`+absoluteB+`"}}`+
		`]}`))
	historyPath := filepath.Join(root, historyRel)
	mustWriteMutationFile(t, historyPath, []byte("prior\n"))

	recorder := historyRecorder{
		now: func() time.Time {
			return time.Date(2026, time.August, 27, 15, 4, 5, 123, time.FixedZone("west", -7*60*60))
		},
		runGit: func(dir string, argv ...string) (string, error) {
			switch strings.Join(argv, " ") {
			case "rev-parse --show-toplevel":
				if dir != "" {
					t.Fatalf("root query directory = %q", dir)
				}
				return root, nil
			case "rev-parse --short HEAD":
				if dir != root {
					t.Fatalf("SHA query directory = %q, want %q", dir, root)
				}
				return "abc123", nil
			default:
				t.Fatalf("unexpected git query: %v", argv)
				return "", nil
			}
		},
	}

	report, err := recorder.record("tmp/report.json")
	if err != nil {
		t.Fatal(err)
	}
	if report.Text() != "mutation history: recorded 3 package(s) in test/mutation/history.ndjson\n" {
		t.Fatalf("record text = %q", report.Text())
	}
	if report.History != historyRel || report.Recorded != 3 || report.CannotRead != "" || len(report.Packages) != 3 {
		t.Fatalf("record structured answer = %#v", report)
	}
	wantPackages := []PackageHistory{
		{Package: ".", Mutants: 1, Killed: 0, Score: 0},
		{Package: "internal/a", Mutants: 1, Killed: 1, Score: 100},
		{Package: "internal/b", Mutants: 2, Killed: 1, Score: 50},
	}
	for index, want := range wantPackages {
		if got := report.Packages[index]; got != want {
			t.Fatalf("package %d = %#v, want %#v", index, got, want)
		}
	}

	got, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "prior\n" +
		"{\"ts\":\"2026-08-27T22:04:05Z\",\"sha\":\"abc123\",\"package\":\".\",\"mutants\":1,\"killed\":0,\"score\":0.0}\n" +
		"{\"ts\":\"2026-08-27T22:04:05Z\",\"sha\":\"abc123\",\"package\":\"internal/a\",\"mutants\":1,\"killed\":1,\"score\":100.0}\n" +
		"{\"ts\":\"2026-08-27T22:04:05Z\",\"sha\":\"abc123\",\"package\":\"internal/b\",\"mutants\":2,\"killed\":1,\"score\":50.0}\n"
	if string(got) != want {
		t.Fatalf("history bytes differ\n got: %q\nwant: %q", got, want)
	}
}

func TestRecordHistoryReadFailuresAreAdvisoryAndDoNotWriteHistory(t *testing.T) {
	root := t.TempDir()
	recorder := historyRecorder{
		now:    time.Now,
		runGit: func(_ string, _ ...string) (string, error) { return root, nil },
	}

	for _, test := range []struct {
		name    string
		path    string
		content []byte
	}{
		{name: "missing", path: "tmp/missing.json"},
		{name: "malformed", path: "tmp/malformed.json", content: []byte(`{"results":[`)},
		{name: "invalid UTF-8", path: "tmp/invalid-utf8.json", content: []byte{0xff}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.content != nil {
				mustWriteMutationFile(t, filepath.Join(root, test.path), test.content)
			}
			report, err := recorder.record(test.path)
			if err != nil {
				t.Fatalf("advisory read returned error: %v", err)
			}
			prefix := "mutation history: cannot read " + test.path + ": "
			if !strings.HasPrefix(report.CannotRead, prefix) || report.Text() != "" || report.Recorded != 0 {
				t.Fatalf("advisory report = %#v", report)
			}
			if _, err := os.Stat(filepath.Join(root, historyRel)); !os.IsNotExist(err) {
				t.Fatalf("advisory read created history: %v", err)
			}
		})
	}
}

func TestRecordHistoryEmptyReportDoesNotQuerySHAOrTouchHistory(t *testing.T) {
	root := t.TempDir()
	mustWriteMutationFile(t, filepath.Join(root, defaultReportRel), []byte(`{"results":[]}`))
	queries := 0
	recorder := historyRecorder{
		now: time.Now,
		runGit: func(_ string, argv ...string) (string, error) {
			queries++
			if strings.Join(argv, " ") != "rev-parse --show-toplevel" {
				t.Fatalf("empty report made unexpected query: %v", argv)
			}
			return root, nil
		},
	}

	report, err := recorder.record("")
	if err != nil {
		t.Fatal(err)
	}
	if queries != 1 {
		t.Fatalf("empty report made %d git queries, want 1", queries)
	}
	if report.Text() != "mutation history: no results in report, nothing recorded\n" || report.Recorded != 0 {
		t.Fatalf("empty report answer = %#v, text %q", report, report.Text())
	}
	if _, err := os.Stat(filepath.Join(root, historyRel)); !os.IsNotExist(err) {
		t.Fatalf("empty report created history: %v", err)
	}
}

func TestRecordHistoryRejectsValidJSONWithInvalidReportSchema(t *testing.T) {
	root := t.TempDir()
	mustWriteMutationFile(t, filepath.Join(root, defaultReportRel), []byte(`{"results":null}`))
	recorder := historyRecorder{
		now:    time.Now,
		runGit: func(_ string, _ ...string) (string, error) { return root, nil },
	}

	_, err := recorder.record("")
	if err == nil || !strings.Contains(err.Error(), "results is not an array") {
		t.Fatalf("invalid schema error = %v", err)
	}
}

func TestRecordHistoryUsesUnknownWhenSHAQueryFails(t *testing.T) {
	root := t.TempDir()
	mustWriteMutationFile(t, filepath.Join(root, defaultReportRel), []byte(`{"results":[{"status":"KILLED"}]}`))
	recorder := historyRecorder{
		now: func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) },
		runGit: func(_ string, argv ...string) (string, error) {
			if strings.Join(argv, " ") == "rev-parse --show-toplevel" {
				return root, nil
			}
			return "", errors.New("no HEAD")
		},
	}

	_, err := recorder.record("")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, historyRel))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"sha":"unknown"`) {
		t.Fatalf("failed SHA query wrote %q", got)
	}
}
