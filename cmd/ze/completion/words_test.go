package completion

import (
	"bytes"
	"strings"
	"testing"
)

func TestWordsShowProducesTabSeparatedOutput(t *testing.T) {
	var buf bytes.Buffer
	code := writeWords(&buf, []string{"show"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("expected non-empty output for 'show' tree")
	}

	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			t.Errorf("expected tab-separated pair, got: %q", line)
		}
		if parts[0] == "" {
			t.Errorf("empty word in line: %q", line)
		}
		if parts[1] == "" {
			t.Errorf("empty description in line: %q", line)
		}
	}
}

func TestWordsRunHasMoreThanShow(t *testing.T) {
	var showBuf, runBuf bytes.Buffer

	writeWords(&showBuf, []string{"show"})
	writeWords(&runBuf, []string{"run"})

	showLines := strings.Count(strings.TrimSpace(showBuf.String()), "\n")
	runLines := strings.Count(strings.TrimSpace(runBuf.String()), "\n")

	if runLines < showLines {
		t.Errorf("expected run (%d entries) >= show (%d entries)", runLines, showLines)
	}
}

func TestWordsDeepPath(t *testing.T) {
	var buf bytes.Buffer
	code := writeWords(&buf, []string{"show", "peer"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("expected non-empty output for 'show peer' subcommands")
	}

	if !strings.Contains(output, "list\t") {
		t.Errorf("expected 'list' in peer subcommands, got: %s", output)
	}
}

func TestWordsInvalidPath(t *testing.T) {
	var buf bytes.Buffer
	code := writeWords(&buf, []string{"show", "nonexistent"})
	if code != 0 {
		t.Fatalf("expected exit 0 for invalid path, got %d", code)
	}

	if buf.String() != "" {
		t.Errorf("expected empty output for invalid path, got: %q", buf.String())
	}
}

func TestWordsEmptyArgs(t *testing.T) {
	var buf bytes.Buffer
	code := writeWords(&buf, nil)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestWordsUnknownContext(t *testing.T) {
	var buf bytes.Buffer
	code := writeWords(&buf, []string{"unknown"})
	if code != 0 {
		t.Fatalf("expected exit 0 for unknown context, got %d", code)
	}

	if buf.String() != "" {
		t.Errorf("expected empty output for unknown context, got: %q", buf.String())
	}
}
