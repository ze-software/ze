package airulesreport

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	// The `ai` namespace registers its summary there; le refuses to dispatch
	// while a namespace it holds has none.
	_ "github.com/ze-software/ze/internal/le/ai"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// captureOutput runs fn with os.Stdout and os.Stderr replaced by pipes and
// answers what fn wrote to each.
func captureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()
	redirect := func(target **os.File) func() string {
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatalf("pipe: %v", err)
		}
		saved := *target
		*target = writer
		done := make(chan string, 1)
		go func() {
			var buffer bytes.Buffer
			_, _ = buffer.ReadFrom(reader)
			done <- buffer.String()
		}()
		return func() string {
			_ = writer.Close()
			*target = saved
			out := <-done
			_ = reader.Close()
			return out
		}
	}
	stopOut := redirect(&os.Stdout)
	stopErr := redirect(&os.Stderr)
	fn()
	return stopOut(), stopErr()
}

// VALIDATES: `le ai rules report` is a registered three-word area holding
// gate-map, payload, router and coverage, and `le ai rules report <verb>
// help` resolves through Dispatch to each action's own page (D-P4-depth3).
// PREVENTS: a sub-area whose actions are registered but that argv cannot
// reach, because the dispatcher reads fewer words than the name has, or an
// action that answers the area's page instead of its own.
func TestReportActionsDispatch(t *testing.T) {
	if !leroot.Owns(area) {
		t.Fatalf("leroot does not own %q", area)
	}
	list, found := leroot.ActionsOf(area)
	if !found {
		t.Fatalf("no actions registered for %q", area)
	}
	verbs := make([]string, 0, len(list.Actions))
	for _, row := range list.Actions {
		verbs = append(verbs, row.Verb)
	}
	if !slices.Equal(verbs, []string{"gate-map", "payload", "router", "coverage"}) {
		t.Fatalf("verbs %v, want gate-map, payload, router, coverage", verbs)
	}

	for _, row := range list.Actions {
		argv := append(strings.Fields(area), row.Verb, "help")
		code := 0
		page, stderr := captureOutput(t, func() { code = leroot.Dispatch("le", argv) })
		if code != 0 {
			t.Errorf("le %s answered %d, want 0; stderr: %s", strings.Join(argv, " "), code, stderr)
		}
		heading := "le " + area + " " + row.Verb + " - " + row.Summary
		if !strings.Contains(page, heading) {
			t.Errorf("le %s does not print the action's page %q:\n%s", strings.Join(argv, " "), heading, page)
		}
	}

	code := 0
	_, stderr := captureOutput(t, func() { code = leroot.Dispatch("le", append(strings.Fields(area), "nosuch")) })
	if code != 2 {
		t.Errorf("le %s nosuch answered %d, want 2 (usage); stderr: %s", area, code, stderr)
	}
}
