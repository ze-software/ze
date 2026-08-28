package wikicatalog

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckCatalogUsesExactBytesAndTreatsMissingAsStale(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "command-catalog.md")
	entries := []Entry{{Path: "show alpha", Mode: "daemon"}}
	markdown := []byte("catalog\n")

	report, err := checkCatalog(destination, entries, markdown)
	if err != nil {
		t.Fatalf("check missing catalog: %v", err)
	}
	if !report.Stale || report.Written || report.Bytes != len(markdown) {
		t.Fatalf("missing catalog report = %#v", report)
	}

	if err := os.WriteFile(destination, markdown, 0o644); err != nil {
		t.Fatalf("write exact catalog: %v", err)
	}
	report, err = checkCatalog(destination, entries, markdown)
	if err != nil {
		t.Fatalf("check exact catalog: %v", err)
	}
	if report.Stale {
		t.Fatalf("exact catalog reported stale: %#v", report)
	}

	if err := os.WriteFile(destination, []byte("catalog\r\n"), 0o644); err != nil {
		t.Fatalf("write different catalog: %v", err)
	}
	report, err = checkCatalog(destination, entries, markdown)
	if err != nil {
		t.Fatalf("check different catalog: %v", err)
	}
	if !report.Stale {
		t.Fatalf("byte-different catalog reported current: %#v", report)
	}
}

func TestUpdateCatalogReturnsWriteErrorsWithUnwrittenReport(t *testing.T) {
	boom := errors.New("disk full")
	entries := []Entry{{Path: "show alpha", Mode: "daemon"}}
	markdown := []byte("catalog\n")
	report, err := updateCatalog("chosen.md", entries, markdown, func(destination string, content []byte) error {
		if destination != "chosen.md" || !bytes.Equal(content, markdown) {
			t.Fatalf("writer received %q and %q", destination, content)
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("write error = %v, want wrapped disk error", err)
	}
	if report.File != "chosen.md" || report.Bytes != len(markdown) || report.Written {
		t.Fatalf("write-error report = %#v", report)
	}
	if len(report.Commands) != 1 || report.Commands[0].Path != "show alpha" {
		t.Fatalf("write-error commands = %#v", report.Commands)
	}
}

func TestAtomicWritePublishesSelectedDestinationAndPreservesMode(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "selected.md")
	if err := os.WriteFile(destination, []byte("old\n"), 0o600); err != nil {
		t.Fatalf("write old destination: %v", err)
	}
	if err := atomicWrite(destination, []byte("new\n")); err != nil {
		t.Fatalf("replace destination: %v", err)
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read replaced destination: %v", err)
	}
	if !bytes.Equal(content, []byte("new\n")) {
		t.Fatalf("destination content = %q", content)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatalf("stat replaced destination: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("destination mode = %04o, want 0600", info.Mode().Perm())
	}
}

func TestAtomicWriteReportsMissingParentWithoutCreatingDestination(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "missing", "catalog.md")
	if err := atomicWrite(destination, []byte("catalog\n")); err == nil {
		t.Fatal("atomicWrite succeeded with a missing parent")
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination exists after failed write: %v", err)
	}
}

func TestReportTextSummarizesEachVerdict(t *testing.T) {
	commands := []Entry{{Path: "show alpha"}}
	tests := []struct {
		name   string
		report Report
		want   string
	}{
		{name: "current", report: Report{File: "catalog.md", Commands: commands}, want: "checked 1 commands, catalog.md up to date\n"},
		{name: "stale", report: Report{File: "catalog.md", Commands: commands, Stale: true}, want: "catalog.md is stale\n"},
		{name: "written", report: Report{File: "catalog.md", Commands: commands, Written: true}, want: "wrote catalog.md (1 commands)\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.report.Text(); got != test.want {
				t.Fatalf("Text() = %q, want %q", got, test.want)
			}
		})
	}
}
