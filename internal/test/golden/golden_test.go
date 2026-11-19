package golden

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// TestBodyOutsideDefines covers the rule that decides whether a template file
// contributes a template named after itself.
//
// VALIDATES: bodyOutsideDefines separates a file that renders a body of its own
// from a file that only carries {{define}} blocks.
// PREVENTS: a file with BOTH a {{define}} and a body contributing a renderable
// template that lands in no fixture and raises no coverage error. The rule this
// replaces read the body only when the file defined nothing.
func TestBodyOutsideDefines(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{name: "body only", src: "<p>hello</p>\n", want: true},
		{name: "define only", src: `{{define "row"}}<td>{{.X}}</td>{{end}}` + "\n", want: false},
		{
			name: "define and body",
			src:  `{{define "row"}}<td>{{.X}}</td>{{end}}` + "\n<p>hello</p>\n",
			want: true,
		},
		{
			name: "define holding its own branch",
			src:  `{{define "row"}}{{if .X}}<td>{{.X}}</td>{{end}}{{end}}` + "\n",
			want: false,
		},
		{
			name: "two defines",
			src:  `{{define "a"}}A{{end}}` + "\n" + `{{define "b"}}B{{end}}` + "\n",
			want: false,
		},
		{name: "top-level branch", src: `{{if .X}}<p>{{.X}}</p>{{end}}` + "\n", want: true},
		{name: "top-level action", src: `{{template "row" .}}` + "\n", want: true},
		{name: "top-level field", src: "<p>{{.X}}</p>\n", want: true},
		{
			name: "comment beside a define",
			src:  "{{/* a note */}}\n" + `{{define "row"}}<td></td>{{end}}` + "\n",
			want: false,
		},
		{name: "empty", src: "", want: false},
		{name: "whitespace", src: "\n\n  \n", want: false},
		{
			name: "unbalanced end",
			src:  `{{define "row"}}<td></td>{{end}}` + "\n{{end}}\n",
			want: true,
		},
		{
			name: "text after the last define",
			src:  `{{define "row"}}<td></td>{{end}}` + "\ntrailing\n",
			want: true,
		},
		{
			name: "trim markers",
			src:  `{{- define "row" -}}<td></td>{{- end -}}` + "\n",
			want: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := bodyOutsideDefines(c.src); got != c.want {
				t.Errorf("bodyOutsideDefines(%q) = %v, want %v", c.src, got, c.want)
			}
		})
	}
}

// TestCoverageReportsAnUncapturedBody drives the whole coverage check over a
// file that carries a {{define}} and a body, with only the define captured.
//
// VALIDATES: Set.coverage names the file's own body as an uncaptured template.
// PREVENTS: the body rendering markup an operator receives while no fixture
// records it.
func TestCoverageReportsAnUncapturedBody(t *testing.T) {
	set := Set{
		FS: fstest.MapFS{
			"templates/page.html": &fstest.MapFile{
				Data: []byte(`{{define "row"}}<td></td>{{end}}` + "\n<p>body</p>\n"),
			},
		},
		Dir:     "templates",
		SpecVar: "testSpec",
		Spec: Spec{
			"templates/page.html": {{
				Name:     "row",
				Variants: []Variant{{Data: nil}},
			}},
		},
	}

	findings := set.coverage([]string{"templates/page.html"})

	if len(findings) != 1 {
		t.Fatalf("coverage findings = %v, want one", findings)
	}

	want := `template "page.html" defined in templates/page.html is not captured`
	if findings[0].Error() != want {
		t.Errorf("finding = %q, want %q", findings[0], want)
	}
}

// TestCoverageReportsASharedTemplateName drives the coverage check over two
// files in different directories that define the same template name.
//
// VALIDATES: Set.coverage names both files when one template name is captured
// twice.
// PREVENTS: two same-named templates reaching two fixture PATHS, where the
// clash check saw no clash. NewRenderer parses component/*.html and
// input/*.html into ONE set, so the later parse wins. One of the two templates
// is then never rendered, and both fixtures still look healthy.
func TestCoverageReportsASharedTemplateName(t *testing.T) {
	body := []byte(`{{define "row"}}<td></td>{{end}}` + "\n")

	set := Set{
		FS: fstest.MapFS{
			"templates/component/row.html": &fstest.MapFile{Data: body},
			"templates/input/row.html":     &fstest.MapFile{Data: body},
		},
		Dir:     "templates",
		SpecVar: "testSpec",
		Spec: Spec{
			"templates/component/row.html": {{Name: "row", Variants: []Variant{{Data: nil}}}},
			"templates/input/row.html":     {{Name: "row", Variants: []Variant{{Data: nil}}}},
		},
	}

	files := []string{"templates/component/row.html", "templates/input/row.html"}

	if set.FixturePath("testdata", files[0], "row") == set.FixturePath("testdata", files[1], "row") {
		t.Fatal("the two fixture paths are equal; the test no longer covers the hole it was written for")
	}

	findings := set.coverage(files)

	if len(findings) != 1 {
		t.Fatalf("coverage findings = %v, want one", findings)
	}

	want := `fixture "row" is captured for both templates/component/row.html and templates/input/row.html; ` +
		"a parsed set holds one template per name, so the later parse wins and one of the two is never rendered"
	if findings[0].Error() != want {
		t.Errorf("finding = %q, want %q", findings[0], want)
	}
}

// TestCoverageReportsAFileOnOneSideOnly drives the two directions of the
// spec-against-FS comparison.
//
// VALIDATES: Set.coverage names a template file the spec misses, and a spec
// entry the FS does not hold.
// PREVENTS: a capture that covers part of the tree reporting green while
// proving nothing about the rest, and a spec row that renders nothing because
// its file was renamed or deleted.
func TestCoverageReportsAFileOnOneSideOnly(t *testing.T) {
	set := Set{
		FS: fstest.MapFS{
			"templates/new.html": &fstest.MapFile{Data: []byte("<p>body</p>\n")},
		},
		Dir:     "templates",
		SpecVar: "testSpec",
		Spec: Spec{
			"templates/gone.html": {{Name: "gone.html", Variants: []Variant{{Data: nil}}}},
		},
	}

	findings := set.coverage([]string{"templates/new.html"})

	if len(findings) != 2 {
		t.Fatalf("coverage findings = %v, want two", findings)
	}

	want := []string{
		"template file templates/new.html has no golden coverage; add its templates to testSpec",
		"testSpec names templates/gone.html, which the embedded FS does not hold",
	}
	for i, w := range want {
		if findings[i].Error() != w {
			t.Errorf("finding %d = %q, want %q", i, findings[i], w)
		}
	}
}

// TestCoverageIsSilentWhenTheSpecMatches proves the checks above discriminate:
// the same shape with each template captured once reports nothing.
//
// VALIDATES: Set.coverage returns no finding for a spec that matches the FS.
// PREVENTS: a check that fails for every input. Such a check reports the holes
// above and proves nothing about them.
func TestCoverageIsSilentWhenTheSpecMatches(t *testing.T) {
	set := Set{
		FS: fstest.MapFS{
			"templates/component/row.html": &fstest.MapFile{
				Data: []byte(`{{define "row"}}<td></td>{{end}}` + "\n"),
			},
			"templates/input/cell.html": &fstest.MapFile{
				Data: []byte(`{{define "cell"}}<td></td>{{end}}` + "\n<p>body</p>\n"),
			},
		},
		Dir:     "templates",
		SpecVar: "testSpec",
		Spec: Spec{
			"templates/component/row.html": {{Name: "row", Variants: []Variant{{Data: nil}}}},
			"templates/input/cell.html": {
				{Name: "cell", Variants: []Variant{{Data: nil}}},
				{Name: "cell.html", Variants: []Variant{{Data: nil}}},
			},
		},
	}

	findings := set.coverage([]string{"templates/component/row.html", "templates/input/cell.html"})
	if len(findings) != 0 {
		t.Fatalf("coverage findings = %v, want none", findings)
	}
}

// TestFixtureNameAndPath pins the two rules that decide where a variant's bytes
// land.
//
// VALIDATES: Unit.FixtureName suffixes a named variant and leaves an unnamed
// one bare. Set.FixturePath mirrors the template tree below Dir and gives every
// fixture the .html suffix once.
// PREVENTS: the coverage check and the render loop writing and checking two
// different paths for one variant.
func TestFixtureNameAndPath(t *testing.T) {
	unit := Unit{Name: "login.html"}

	if got := unit.FixtureName(Variant{}); got != "login.html" {
		t.Errorf("FixtureName(unnamed) = %q, want login.html", got)
	}

	if got := unit.FixtureName(Variant{Name: "overlay"}); got != "login.html--overlay" {
		t.Errorf("FixtureName(overlay) = %q, want login.html--overlay", got)
	}

	set := Set{Dir: "templates"}

	cases := []struct {
		file    string
		fixture string
		want    string
	}{
		{file: "templates/page/login.html", fixture: "login.html--overlay", want: "testdata/page/login.html--overlay.html"},
		{file: "templates/flex.html", fixture: "flex.html", want: "testdata/flex.html"},
		{file: "templates/component/row.html", fixture: "row", want: "testdata/component/row.html"},
	}

	for _, c := range cases {
		if got := set.FixturePath("testdata", c.file, c.fixture); got != c.want {
			t.Errorf("FixturePath(%s, %s) = %q, want %q", c.file, c.fixture, got, c.want)
		}
	}
}

// TestFilesLeavesANestedPackageToItsOwnSet walks a templ tree that holds a
// nested Go package, a nested directory that is not a package, and a testdata
// directory.
//
// The fleet interface nests under the web package as a package of its own.
// Its components are that package's symbols, so the web package's spec cannot
// name them, and a walk that returned its sources demanded coverage no edit to
// that spec could give.
//
// VALIDATES: with Ext set, Set.Files leaves out a directory below Dir that
// holds a Go source file, with everything under it, and still returns the
// sources of the package itself and of a nested directory that is no package.
// Set.coverage then demands the nested non-package source.
// PREVENTS: a nested package's templ source reported against the spec of the
// package above it, and the opposite defect, where every nested directory is
// skipped and a template placed in one renders into no fixture.
func TestFilesLeavesANestedPackageToItsOwnSet(t *testing.T) {
	component := func(pkg, name string) *fstest.MapFile {
		return &fstest.MapFile{Data: []byte("package " + pkg + "\n\ntempl " + name + "() {\n\t<p></p>\n}\n")}
	}

	set := Set{
		FS: fstest.MapFS{
			"page.templ":    component("web", "page"),
			"page_templ.go": &fstest.MapFile{Data: []byte("package web\n")},
			// A nested package: a Go source file sits beside its templ source.
			"nested/shell.templ":    component("nested", "shell"),
			"nested/shell_templ.go": &fstest.MapFile{Data: []byte("package nested\n")},
			// Below the nested package, in a directory with no Go source. It
			// is under another package, so it is outside this tree too.
			"nested/partials/tab.templ": component("nested", "tab"),
			// A nested directory that is not a package.
			"partials/row.templ": component("web", "row"),
			// A fixture, which the walk always leaves out.
			"testdata/sample.templ": component("web", "sample"),
		},
		Dir: ".",
		Ext: ".templ",
		Spec: Spec{
			"page.templ": {{Name: "page", Variants: []Variant{{}}}},
		},
		SpecVar: "testSpec",
	}

	files := set.Files(t)

	want := []string{"page.templ", "partials/row.templ"}
	if !slices.Equal(files, want) {
		t.Fatalf("Files() = %v, want %v", files, want)
	}

	findings := set.coverage(files)
	if len(findings) != 1 {
		t.Fatalf("coverage findings = %v, want one", findings)
	}

	demanded := "template file partials/row.templ has no golden coverage; add its templates to testSpec"
	if findings[0].Error() != demanded {
		t.Errorf("finding = %q, want %q", findings[0], demanded)
	}
}

// TestFilesWalksEveryDirectoryOfATemplateFS walks a tree with Ext empty, the
// shape of an embedded FS that holds nothing but templates.
//
// VALIDATES: with Ext empty, Set.Files returns the files of every nested
// directory but testdata, a directory that holds a Go source file included.
// PREVENTS: the nested-package rule reaching a tree it was not written for,
// where it would drop a directory of templates from the capture with no
// finding.
func TestFilesWalksEveryDirectoryOfATemplateFS(t *testing.T) {
	body := &fstest.MapFile{Data: []byte("<p>body</p>\n")}

	set := Set{
		FS: fstest.MapFS{
			"templates/page/login.html":    body,
			"templates/component/row.html": body,
			"templates/component/doc.go":   &fstest.MapFile{Data: []byte("package component\n")},
			"templates/testdata/case.html": body,
		},
		Dir: "templates",
	}

	files := set.Files(t)

	want := []string{
		"templates/component/doc.go",
		"templates/component/row.html",
		"templates/page/login.html",
	}
	if !slices.Equal(files, want) {
		t.Fatalf("Files() = %v, want %v", files, want)
	}
}

// TestTemplSourcesDeriveTheirComponents drives the templ half of the coverage
// check. A templ file declares its units with the templ keyword, not with
// {{define}}. It also lives beside the package's Go files.
//
// VALIDATES: Set.Ext keeps the walk to the templ sources, definedNames reads
// every templ component out of one, and Unit.Fixture decouples the fixture
// stem from the component's Go name.
// PREVENTS: a templ component added later rendering into no fixture, which is
// the hole the {{define}} rule would leave over a templ tree.
func TestTemplSourcesDeriveTheirComponents(t *testing.T) {
	set := Set{
		FS: fstest.MapFS{
			"peers.templ": &fstest.MapFile{Data: []byte(
				"package lg\n\ntempl peersPage(v peersView) {\n\t<h1>Peers</h1>\n}\n\n" +
					"templ peersTableBody(rows []peerRow) {\n\t<tr></tr>\n}\n")},
			"peers_templ.go": &fstest.MapFile{Data: []byte("package lg\n")},
			"handler.go":     &fstest.MapFile{Data: []byte("package lg\n")},
		},
		Dir: ".",
		Ext: ".templ",
		Spec: Spec{
			"peers.templ": {
				{Name: "peersPage", Fixture: "peers", Variants: []Variant{{}}},
			},
		},
		SpecVar: "lgGoldenSpec",
	}

	files := set.Files(t)
	if len(files) != 1 || files[0] != "peers.templ" {
		t.Fatalf("Files() = %v, want only peers.templ; Ext did not keep the walk off the Go files", files)
	}

	findings := set.coverage(files)
	if len(findings) != 1 {
		t.Fatalf("coverage() reported %d findings, want 1: %v", len(findings), findings)
	}

	if got := findings[0].Error(); !strings.Contains(got, "peersTableBody") {
		t.Errorf("coverage() = %q, want it to name the uncaptured component", got)
	}

	// The captured unit writes the fixture its Fixture field names, not one
	// named after the Go identifier.
	unit := set.Spec["peers.templ"][0]
	if got := unit.FixtureName(Variant{}); got != "peers" {
		t.Errorf("FixtureName = %q, want peers", got)
	}

	if got := set.FixturePath("testdata/golden", "peers.templ", "peers"); got != "testdata/golden/peers.html" {
		t.Errorf("FixturePath = %q, want testdata/golden/peers.html", got)
	}
}
