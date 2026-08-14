package golden

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNameCoverageReportsBothDirections drives the check a handler capture
// hangs its route list on.
//
// VALIDATES: nameCoverage names a live route no case reaches, and a case that
// names a route nothing serves.
// PREVENTS: a capture that requests 20 of 40 routes reporting green through a
// whole migration, and a case list that goes on naming a route after the route
// is deleted.
func TestNameCoverageReportsBothDirections(t *testing.T) {
	findings := nameCoverage("route", "testCases",
		[]string{"GET /a", "GET /b"},
		[]string{"GET /a", "GET /gone"})

	want := []string{
		`route "GET /b" is live but no golden case reaches it; add one to testCases`,
		`testCases names route "GET /gone", which is not live`,
	}

	if len(findings) != len(want) {
		t.Fatalf("findings = %v, want %d", findings, len(want))
	}

	for i, w := range want {
		if findings[i].Error() != w {
			t.Errorf("finding %d = %q, want %q", i, findings[i], w)
		}
	}
}

// TestNameCoverageIsSilentWhenTheSetsMatch proves the check above discriminates.
//
// VALIDATES: nameCoverage returns no finding when the capture reaches every
// live route and no other.
// PREVENTS: a check that fails for every input, which reports the holes above
// and proves nothing about them.
func TestNameCoverageIsSilentWhenTheSetsMatch(t *testing.T) {
	findings := nameCoverage("route", "testCases",
		[]string{"GET /a", "GET /b"},
		[]string{"GET /b", "GET /a", "GET /a"})
	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none", findings)
	}
}

// TestRepeatedNamesReportsAClash covers the fixture-name check.
//
// VALIDATES: repeatedNames names a fixture stem two cases share.
// PREVENTS: the second case overwriting the first one's fixture, which leaves
// the capture green over bytes nobody compared.
func TestRepeatedNamesReportsAClash(t *testing.T) {
	findings := repeatedNames("fixture", "testCases", []string{"one", "two", "one"})

	if len(findings) != 1 {
		t.Fatalf("findings = %v, want one", findings)
	}

	want := `testCases gives 2 cases the fixture name "one"; one fixture cannot hold two captures`
	if findings[0].Error() != want {
		t.Errorf("finding = %q, want %q", findings[0], want)
	}

	if rest := repeatedNames("fixture", "testCases", []string{"one", "two"}); len(rest) != 0 {
		t.Fatalf("findings = %v, want none", rest)
	}
}

// TestRoutePatternsReadsLiteralAndDynamic drives the source read that keeps a
// route list out of the test that captures it.
//
// VALIDATES: RoutePatterns returns every literal pattern a file registers,
// sorted, and reports a registration whose pattern the source computes.
// PREVENTS: a route added to a server passing uncaptured, which is what a
// hand-written route table in the test allows.
func TestRoutePatternsReadsLiteralAndDynamic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.go")

	source := `package server

func register(mux *ServeMux, routes []Route) {
	mux.HandleFunc("POST /login", login)
	mux.Handle("GET /assets/", assets)
	for _, r := range RegisteredWebRoutes() {
		mux.Handle(r.Pattern, r.Build())
	}
	other.Call("GET /not-a-route")
}
`

	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("write the probe source: %v", err)
	}

	literal, dynamic := RoutePatterns(t, path)

	if want := []string{"GET /assets/", "POST /login"}; !equalStrings(literal, want) {
		t.Errorf("literal = %v, want %v", literal, want)
	}

	if len(dynamic) != 1 {
		t.Fatalf("dynamic = %v, want one", dynamic)
	}

	if dynamic[0].Expr != "r.Pattern" {
		t.Errorf("dynamic expression = %q, want %q", dynamic[0].Expr, "r.Pattern")
	}

	if dynamic[0].RangeOver != "RegisteredWebRoutes()" {
		t.Errorf("range subject = %q, want %q", dynamic[0].RangeOver, "RegisteredWebRoutes()")
	}

	if !strings.HasSuffix(dynamic[0].Pos, "server.go:7:3") {
		t.Errorf("dynamic position = %q, want the call's line", dynamic[0].Pos)
	}
}

// TestRoutePatternsNamesTheSetALoopRegisters covers the field a caller reads to
// tell one registry from another.
//
// VALIDATES: RangeOver carries the enclosing range's subject, the innermost one
// when ranges nest, and is empty for a registration in no range.
// PREVENTS: a capture accepting a loop repointed at another registry. That
// edit leaves the pattern expression unchanged. A check reading the expression
// alone stays silent over a capture of a set the server no longer serves.
func TestRoutePatternsNamesTheSetALoopRegisters(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.go")

	source := `package server

func register(mux *ServeMux) {
	mux.Handle("GET /health", health)
	mux.Handle(computed(), handler)
	for _, r := range RegisteredWebRoutes() {
		for _, alias := range r.Aliases {
			mux.Handle(alias.Pattern, r.Build())
		}
	}
}
`

	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("write the probe source: %v", err)
	}

	_, dynamic := RoutePatterns(t, path)

	if len(dynamic) != 2 {
		t.Fatalf("dynamic = %v, want two", dynamic)
	}

	if dynamic[0].Expr != "computed()" || dynamic[0].RangeOver != "" {
		t.Errorf("top-level registration = %+v, want computed() in no range", dynamic[0])
	}

	if dynamic[1].Expr != "alias.Pattern" || dynamic[1].RangeOver != "r.Aliases" {
		t.Errorf("nested registration = %+v, want alias.Pattern under r.Aliases", dynamic[1])
	}
}

// TestRepoFileFindsATrackedFile covers the path rule a capture reading another
// package's source depends on.
//
// VALIDATES: RepoFile resolves a repository-relative path from the test's own
// working directory.
// PREVENTS: a capture reading a source file through a relative path that breaks
// the moment its own package moves.
func TestRepoFileFindsATrackedFile(t *testing.T) {
	path := RepoFile(t, filepath.Join("internal", "test", "golden", "routes.go"))

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("RepoFile returned %s, which is not there: %v", path, err)
	}

	if !filepath.IsAbs(path) {
		t.Errorf("RepoFile returned %q, want an absolute path", path)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}
