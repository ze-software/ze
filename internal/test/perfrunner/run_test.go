package perfrunner

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGenerateToFileNeverDestroysTheExistingReport(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "report.html")
	if err := os.WriteFile(destination, []byte("OLD\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	failed := func(_ context.Context, stdout, _ io.Writer, _ string, _ []string, _ []string) error {
		_, _ = io.WriteString(stdout, "PARTIAL\n")
		return errors.New("generator failed")
	}
	var stderr strings.Builder
	if generateToFile(context.Background(), failed, []string{"ze-perf", "report"}, destination, &stderr) {
		t.Fatal("failing generator reported success")
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "OLD\n" {
		t.Fatalf("destination = %q, want original bytes", content)
	}
	if _, err := os.Stat(destination + ".new"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary file remains: %v", err)
	}
}

func TestGenerateToFilePublishesOnlyACompleteResult(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "report.html")
	succeeded := func(_ context.Context, stdout, _ io.Writer, _ string, _ []string, _ []string) error {
		_, _ = io.WriteString(stdout, "HTML\n")
		return nil
	}
	if !generateToFile(context.Background(), succeeded, []string{"ze-perf", "report"}, destination, io.Discard) {
		t.Fatal("successful generator reported failure")
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "HTML\n" {
		t.Fatalf("destination = %q", content)
	}
}

func TestUnmeasuredDUTsReportsPartialFleetRuns(t *testing.T) {
	if got, want := unmeasuredDUTs([]string{"/tmp/ze.json", "/tmp/ze-propagation.json", "/tmp/bird.json"}), []string{"frr", "gobgp", "rustbgpd", "rustybgp", "freertr", "openbgpd"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("missing DUTs = %q, want %q", got, want)
	}
	all := make([]string, 0, len(DUTs()))
	for _, dut := range DUTs() {
		all = append(all, dut.Name+".json")
	}
	if got := unmeasuredDUTs(all); len(got) != 0 {
		t.Fatalf("whole fleet reports missing DUTs: %q", got)
	}
}

func TestConfigOverlayWinsPerFileWithoutReplacingDefaults(t *testing.T) {
	root := t.TempDir()
	runner := New(root, io.Discard, io.Discard)
	overlay := filepath.Join(root, "overlay")
	if err := os.MkdirAll(overlay, 0o750); err != nil {
		t.Fatal(err)
	}
	runner.ConfigOverlay = overlay
	if err := os.WriteFile(filepath.Join(overlay, "ze.conf"), []byte("filter"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := runner.config("ze.conf"); got != filepath.Join(overlay, "ze.conf") {
		t.Fatalf("overlay config = %s", got)
	}
	if got := runner.config("bird.conf"); got != filepath.Join(root, "test", "perf", "configs", "bird.conf") {
		t.Fatalf("fallback config = %s", got)
	}
}
