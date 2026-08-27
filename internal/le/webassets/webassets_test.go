// Related: webassets.go -- the generator these tests call as a function

package webassets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The three surfaces the generator derives are packages of the real tree, so a
// fixture stands one of them up under its own path. web is the one with a templ
// component graph and several pages.
const webDir = "internal/component/web"

const shellTempl = `package web

//ze:page
templ peers() {
	<div hx-sse:connect="/events">peers</div>
	@row()
}

templ row() {
	<span hx-get="/row">row</span>
}

//ze:page
templ quiet() {
	<p>nothing moves here</p>
}

templ layout(v View) {
	<html>
		<head>
			@assetTags(pageAssets(v.Page))
		</head>
	</html>
}

templ assetTags(assets []string) {
	<span>tags</span>
}
`

func write(t *testing.T, root, rel, content string) {
	t.Helper()

	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create the fixture directory for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write the fixture %s: %v", rel, err)
	}
}

// onlyWeb narrows the surface table to the one a fixture stands up, so a test
// does not have to build all three.
func onlyWeb(t *testing.T) {
	t.Helper()

	saved := surfaces
	t.Cleanup(func() { surfaces = saved })

	for _, one := range saved {
		if one.Dir == webDir {
			surfaces = []surface{one}
			return
		}
	}
	t.Fatalf("no surface is declared for %s", webDir)
}

// fixture stands up one templ surface holding two pages, one of which streams.
func fixture(t *testing.T) string {
	t.Helper()

	onlyWeb(t)

	root := t.TempDir()
	write(t, root, "go.mod", "module example.test/assets\n\ngo 1.26\n")
	write(t, root, "feature-gates.txt", "")
	write(t, root, filepath.Join(webDir, "layout.templ"), shellTempl)

	return root
}

// VALIDATES: a page's asset set is the closure over the components its shell
// reaches, and a page that renders no htmx attribute imports nothing.
// PREVENTS: the union shipping to every page, and a streaming page loading the
// SSE extension without the core it calls into.
func TestEachPageGetsTheAssetsItsMarkupReaches(t *testing.T) {
	root := fixture(t)

	sets, _, err := derive(root)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}

	peers := sets[webDir+"/layout.templ#peers"]
	if strings.Join(peers, ",") != htmxAsset+","+sseAsset {
		t.Errorf("the streaming page loads %v, want the core then the extension", peers)
	}

	quiet := sets[webDir+"/layout.templ#quiet"]
	if len(quiet) != 0 {
		t.Errorf("the page that renders no htmx attribute loads %v, want nothing", quiet)
	}
}

// VALIDATES: the core is loaded before any extension, whatever the names sort
// to.
// PREVENTS: an extension script that runs before htmx.registerExtension exists,
// which throws in the browser and leaves the page rendering correctly and doing
// nothing.
func TestTheCoreIsLoadedFirst(t *testing.T) {
	got := loadOrder([]string{sseAsset, htmxAsset, sseAsset})
	if strings.Join(got, ",") != htmxAsset+","+sseAsset {
		t.Errorf("the load order is %v, want the core first and no repeat", got)
	}

	// The two real names already sort into the right order, so the pair above
	// would pass with nothing ordering them at all: htmx.min.js sorts before
	// hx-sse.min.js. That accident is what this function exists to stop relying
	// on, so the discriminating case is an extension whose name sorts BEFORE
	// the core.
	renamed := loadOrder([]string{"aa-sse.min.js", htmxAsset})
	if strings.Join(renamed, ",") != htmxAsset+",aa-sse.min.js" {
		t.Errorf("the load order is %v, want the core first whatever the names sort to", renamed)
	}

	if empty := loadOrder(nil); empty == nil || len(empty) != 0 {
		t.Errorf("an empty set is %v, want an empty list rather than nil", empty)
	}
}

// VALIDATES: check names the generated file that disagrees with the markup, and
// answers 1.
// PREVENTS: a page whose head block loads an asset its markup no longer uses, or
// drops one it now needs, which is invisible everywhere but the browser.
func TestCheckReportsAStaleGeneratedFile(t *testing.T) {
	root := fixture(t)
	write(t, root, filepath.Join(webDir, generatedFile), "package web\n\n// hand-edited\n")

	report, code := runCheck(root)
	if code != 1 {
		t.Fatalf("check over a stale tree answered %d, want 1", code)
	}
	if report.Stale != webDir+"/"+generatedFile {
		t.Errorf("check named %q stale, want the generated file", report.Stale)
	}
	if !strings.Contains(report.Text(), "is stale: it disagrees with the markup its package renders") {
		t.Errorf("the verdict does not say why:\n%s", report.Text())
	}
}

// VALIDATES: a generated file that is absent stops the run rather than reading
// as a pass.
// PREVENTS: the fail-open a --check invites. A file nobody can read and a file
// that agrees are the same silence, and only one of them is current.
func TestAnAbsentGeneratedFileStopsTheRun(t *testing.T) {
	root := fixture(t)

	if _, err := Check(root); err == nil {
		t.Fatal("check answered no error for a generated file that is not there")
	}
	if _, code := runCheck(root); code != 1 {
		t.Error("check answered 0 for a generated file that is not there")
	}
}

// VALIDATES: write generates the file, check then passes, and the generated Go
// carries the constant and the set for each page.
// PREVENTS: a generator whose own check rejects its output, and one whose
// output does not compile as the package it declares.
func TestWriteThenCheckAgree(t *testing.T) {
	root := fixture(t)

	written, code := runWrite(root)
	if code != 0 {
		t.Fatalf("write answered %d, want 0", code)
	}
	if len(written.Written) != 1 {
		t.Fatalf("write reported %v, want the one generated file", written.Written)
	}

	body, err := os.ReadFile(filepath.Join(root, webDir, "page_assets.go"))
	if err != nil {
		t.Fatalf("read the generated file: %v", err)
	}
	for _, want := range []string{
		"package web",
		"pgPeers pageID = \"peers\"",
		"pgPeers: {\"/assets/htmx.min.js\", \"/assets/hx-sse.min.js\"}",
		"pgQuiet: {}",
		"func pageAssets(page pageID) []string {",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the generated file does not carry %q:\n%s", want, body)
		}
	}

	if report, code := runCheck(root); code != 0 {
		t.Fatalf("check after write answered %d; stale: %q", code, report.Stale)
	}
}

// VALIDATES: the pages action answers the derived sets and writes nothing.
// PREVENTS: a query that rewrites the tree by the act of reading it. A test asks
// for these sets while it runs, and it must not leave the working tree changed.
func TestPagesAnswersTheSetsAndWritesNothing(t *testing.T) {
	root := fixture(t)

	sets, code := runPages(root)
	if code != 0 {
		t.Fatalf("pages answered %d, want 0", code)
	}
	if len(sets) != 2 {
		t.Errorf("pages answered %d sets, want 2: %v", len(sets), sets)
	}
	if _, err := os.Stat(filepath.Join(root, webDir, "page_assets.go")); !os.IsNotExist(err) {
		t.Errorf("pages wrote the generated file: %v", err)
	}
}

// VALIDATES: a surface whose markup the walk cannot parse stops the run.
// PREVENTS: the fail-open this generator was designed against. A templ release
// that changes a declaration leaves the walk finding an empty graph, every set
// empty, and every head block loading nothing.
func TestASurfaceThatParsesToNothingStopsTheRun(t *testing.T) {
	// Each case names the message its OWN guard produces. Several of these trees
	// would also trip a later guard, so asserting only that an error came back
	// would leave every guard but the last one untested.
	cases := map[string]struct{ body, says string }{
		"no templ component at all": {
			"package web\n\nfunc nothing() {}\n",
			"holds no templ component",
		},
		"a component with no brace": {
			"package web\n\ntempl pgOpen() {\n\t<div>never closed</div>\n",
			"has no closing brace at column 0",
		},
		"a head block that hand-writes its tags": {
			"package web\n\ntempl layout() {\n\t<html><head><script src=\"/assets/htmx.min.js\"></script></head></html>\n}\n",
			"still hand-writes its script tags",
		},
		"a page marker no shell serves": {
			"package web\n\n//ze:page\ntempl pgOrphan() {\n\t<div hx-get=\"/x\">x</div>\n}\n\n" +
				"templ shell() {\n\t<html><head>@assetTags(pageAssets(pgShell))</head></html>\n}\n",
			"and no shell of the package asks for a page at render time",
		},
	}

	for name, one := range cases {
		t.Run(name, func(t *testing.T) {
			onlyWeb(t)
			root := t.TempDir()
			write(t, root, "go.mod", "module example.test/assets\n\ngo 1.26\n")
			write(t, root, filepath.Join(webDir, "layout.templ"), one.body)

			_, _, err := derive(root)
			if err == nil {
				t.Fatal("derive answered no error for markup it could not read")
			}
			if !strings.Contains(err.Error(), one.says) {
				t.Errorf("derive answered %q, want it to say %q", err, one.says)
			}
			if _, code := runCheck(root); code != 1 {
				t.Error("check answered 0 for markup it could not read")
			}
		})
	}
}

// VALIDATES: a shell that names a page for itself must name its OWN page.
// PREVENTS: a head block asking for another page's set, which ships that page's
// assets to this one and leaves this page's set derived by nothing.
func TestAShellMustNameItsOwnPage(t *testing.T) {
	onlyWeb(t)
	root := t.TempDir()
	write(t, root, "go.mod", "module example.test/assets\n\ngo 1.26\n")
	write(t, root, filepath.Join(webDir, "layout.templ"),
		"package web\n\ntempl list() {\n\t<html><head>@assetTags(pageAssets(pgOther))</head></html>\n}\n")

	if _, _, err := derive(root); err == nil {
		t.Fatal("derive accepted a shell naming another page")
	}
}

// VALIDATES: a component rendering content the walk cannot follow contributes
// everything the package renders.
// PREVENTS: an under-approximation. A page whose body arrives as a
// templ.Component parameter would otherwise load nothing, and an asset needed
// and absent costs more than one shipped and unused.
func TestAnOpaqueComponentTakesTheWholeSurface(t *testing.T) {
	onlyWeb(t)
	root := t.TempDir()
	write(t, root, "go.mod", "module example.test/assets\n\ngo 1.26\n")
	write(t, root, filepath.Join(webDir, "layout.templ"),
		"package web\n\ntempl shell(body templ.Component) {\n"+
			"\t<html><head>@assetTags(pageAssets(pgShell))</head>@body</html>\n}\n\n"+
			"templ elsewhere() {\n\t<div hx-sse:connect=\"/events\">x</div>\n}\n")

	sets, _, err := derive(root)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	got := sets[webDir+"/layout.templ"]
	if strings.Join(got, ",") != htmxAsset+","+sseAsset {
		t.Errorf("the opaque shell loads %v, want everything the package renders", got)
	}
}
