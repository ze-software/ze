package web

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/golden"
)

// SCAFFOLDING, deleted with templ_fidelity_test.go when phase 3b closes.
//
// The unit comparison in templ_fidelity_test.go renders one component against
// one template. It cannot see the composition layer: RenderLayout,
// RenderWorkbench and the handlers that build a view model and write a whole
// response. This sweep covers that layer by comparing the HANDLER capture
// against the bytes committed at HEAD.
//
// It reads the pre-port side out of git rather than from a copy, so nothing
// here can fossilize into a baseline that outlives the port.

// TestTemplPortHandlerFidelity compares every handler fixture against its
// committed bytes under golden.NormalizeHTML.
//
// VALIDATES: AC-2 at the composition layer. A whole HTTP response is the same
// page after the port as before it.
// PREVENTS: a port that keeps every unit faithful and still drops, duplicates
// or reorders a region when the page is assembled. The unit comparison is blind
// to that, because it never renders a page.
func TestTemplPortHandlerFidelity(t *testing.T) {
	root := filepath.Join("testdata", "handler")

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}

	compared := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}

		name := entry.Name()

		t.Run(strings.TrimSuffix(name, ".txt"), func(t *testing.T) {
			path := filepath.Join(root, name)

			before, ok := committedFixture(t, path)
			if !ok {
				t.Skipf("%s is new since HEAD; it has no pre-port bytes to compare against", path)
			}

			after, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("read %s: %v", path, readErr)
			}

			beforeHead, beforeBody := splitFixture(string(before))
			afterHead, afterBody := splitFixture(string(after))

			if beforeHead != afterHead {
				t.Errorf("response head moved for %s\n--- before\n%s\n--- after\n%s", name, beforeHead, afterHead)
			}

			want := intendedChange(name, templSpelling(golden.NormalizeHTML(beforeBody)))
			have := golden.NormalizeHTML(afterBody)

			if want != have {
				t.Errorf("response body is not the same page for %s\n--- first difference at %d\n--- before\n%s\n--- after\n%s",
					name, firstDifference(want, have), want, have)
			}
		})

		compared++
	}

	// A sweep that compares nothing reports green. The floor is the count the
	// phase-1 capture recorded, so a fixture tree that empties fails here.
	if compared < 100 {
		t.Errorf("compared %d handler fixtures; the capture holds far more", compared)
	}
}

// intendedChange rewrites the pre-port side of a response this phase MEANT to
// change. The sweep then compares the rest of the page byte for byte.
//
// The one entry is the double escape. handleDashboardEventsPage
// (page_dashboard.go) ran each cell through template.HTMLEscapeString, and
// html/template escaped the result again. A namespace holding a quote reached
// the operator as the literal text &#34; rather than as ".
//
// The hand escape is deleted at every ported call site (R-5 and AC-5 of
// plan/spec-web-templ-migration.md). This states which bytes that moved.
// Everything else in the response must still match.
//
// A test that SKIPPED the fixture would prove nothing about the other 10 KiB
// of that page.
func intendedChange(fixture, before string) string {
	if fixture != "nav-show-events.txt" {
		return before
	}

	return strings.ReplaceAll(before, "&amp;#34;", "&#34;")
}

// committedFixture reads one fixture as HEAD holds it. A fixture added since
// HEAD has no pre-port side, and the caller skips it.
func committedFixture(t *testing.T, path string) ([]byte, bool) {
	t.Helper()

	rel := filepath.ToSlash(filepath.Join("internal", "component", "web", path))

	out, err := exec.CommandContext(context.Background(), "git", "show", "HEAD:"+rel).Output()
	if err != nil {
		return nil, false
	}

	return out, true
}

// splitFixture separates a handler fixture's status and header block from the
// response body. The two are compared by different rules: the head byte for
// byte, the body as a page.
func splitFixture(src string) (head, body string) {
	before, after, ok := strings.Cut(src, "\n\n")
	if !ok {
		return src, ""
	}

	return before, after
}
