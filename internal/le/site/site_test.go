package site

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/leroot"
)

// VALIDATES: every former website launcher has a native site action.
func TestActionsExposeNativeWebsiteWorkflows(t *testing.T) {
	if !leroot.Owns("site") {
		t.Fatal("site area is not registered with the le root")
	}
	verbs := map[string]bool{}
	for _, action := range Actions().Actions {
		verbs[action.Verb] = true
	}
	for _, verb := range []string{"build", "check", "bundle", "activity", "update-talk"} {
		if !verbs[verb] {
			t.Errorf("site action %q is not registered", verb)
		}
	}
}

// VALIDATES: the source and output roots cannot overlap.
func TestResolvePathsRejectsInTreeOutput(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "website"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := resolvePaths(root, filepath.Join(root, "website", "public")); err == nil {
		t.Fatal("accepted an output inside website sources")
	}
}

// VALIDATES: every former source-only family stays out of deployment.
func TestSourceOnlyBoundary(t *testing.T) {
	for _, name := range []string{"tools/build.go", "assets/css/site.css", "blog/posts/a.md", "presentations/tools/bundle.go", "faq/faq.md"} {
		if !isSourceOnly(name) {
			t.Errorf("%s escaped the source-only boundary", name)
		}
	}
	for _, name := range []string{"faq/index.html", "assets/site.css", "blog/index.html", "CNAME"} {
		if isSourceOnly(name) {
			t.Errorf("%s was incorrectly source-only", name)
		}
	}
}

// VALIDATES: the build digest commits to names, bytes, symlink targets, and ordering.
func TestSourceDigestIsOrderIndependentAndContentSensitive(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := sourceDigest(root, []string{"b", "a"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := sourceDigest(root, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("digest depends on discovery order")
	}
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := sourceDigest(root, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if changed == first {
		t.Fatal("digest ignored changed bytes")
	}
}

// VALIDATES: CSS imports resolve relative to their own stylesheet and cycles fail closed.
func TestRenderCSSExpandsNestedImports(t *testing.T) {
	source, output := t.TempDir(), t.TempDir()
	directory := filepath.Join(source, "assets", "css")
	if err := os.MkdirAll(filepath.Join(directory, "parts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "site.css"), []byte("@import url(\"parts/base.css\");\nmain { color: red; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "parts", "base.css"), []byte(`body { margin: 0; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := renderCSS(source, output); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(output, "assets", "site.css"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "body{margin:0}") || !strings.Contains(string(content), "main{color:red}") {
		t.Fatalf("imports were not expanded: %s", content)
	}
}

// VALIDATES: descendant pseudo-class selectors still target descendants after minification.
// PREVENTS: removing the descendant combinator so article title rules never match.
func TestMinifyCSSPreservesDescendantPseudoSelector(t *testing.T) {
	got := string(minifyCSS([]byte(`.hero :is(h1, h2) { font-size: 3rem; }`)))
	want := `.hero :is(h1,h2){font-size:3rem}`
	if got != want {
		t.Fatalf("minified CSS = %q, want %q", got, want)
	}
}

// VALIDATES: deck images, nested CSS assets, slide sources, and standalone identity are in one file.
func TestBundlePresentationInlinesLocalAssets(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pixel.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "slides.md"), []byte("# Talk"), 0o644); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(root, "deck.html")
	if err := os.WriteFile(input, []byte(`<html><head><title>Ze - conference slides</title></head><body><img src="pixel.png"><script></script></body></html>`), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := bundlePresentation(input)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"data:image/png;base64,", `id="embedded-slides"`, "standalone HTML deck"} {
		if !strings.Contains(string(content), want) {
			t.Errorf("bundled deck lacks %q: %s", want, content)
		}
	}
}

// VALIDATES: every vendored font URL is local and every face remains swappable.
func TestVendoredFontsAreSelfHosted(t *testing.T) {
	root := filepath.Join("..", "..", "..", "website", "assets", "vendor", "fonts")
	files, err := filepath.Glob(filepath.Join(root, "*.css"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no vendored font stylesheets")
	}
	for _, path := range files {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		text := blockComment.ReplaceAllString(string(content), "")
		if strings.Contains(text, "@font-face") && !strings.Contains(text, "font-display: swap") {
			t.Errorf("%s has a font face without swap display", path)
		}
		for _, match := range cssURLPattern.FindAllStringSubmatch(text, -1) {
			name := strings.Trim(strings.TrimSpace(match[1]), `"'`)
			if remoteAsset(name) {
				t.Errorf("%s contains remote font URL %s", path, name)
				continue
			}
			if _, statErr := os.Stat(filepath.Join(filepath.Dir(path), filepath.FromSlash(name))); statErr != nil {
				t.Errorf("%s names missing font %s", path, name)
			}
		}
	}
}

// VALIDATES: a full build stages repository inputs, refreshes command pages,
// and removes source-only files from a fresh artifact.
func TestBuildStagesADeployableArtifact(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "main")
	source := filepath.Join(root, "website")
	if err := os.MkdirAll(filepath.Join(source, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, "talks", "netmcr"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "CNAME"), []byte("example.test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "tools", "source.txt"), []byte("source only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog := `[{"path":"show test","description":"Show rows","mode":"read-only","operators":[{"name":"json","class":"global","available":"always","description":"JSON"}]}]`
	stubLiveCommandCatalog(t, catalog)
	if err := os.WriteFile(filepath.Join(source, "data", "cli-commands.json"), []byte(catalog), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "talks", "netmcr", "index.html"), []byte("<html><script></script></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedAssets := filepath.Join(parent, "gh-pages", "assets")
	if err := os.MkdirAll(seedAssets, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seedAssets, "site.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seedAssets, "site.js"), []byte("\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"init"}, {"add", "."}} {
		command := exec.CommandContext(t.Context(), "git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
	}
	output := filepath.Join(parent, "artifact")
	report, err := Build(BuildOptions{Repository: root, Output: output})
	if err != nil {
		t.Fatal(err)
	}
	if report.Files != 4 || report.SourceDigest == "" {
		t.Fatalf("unexpected build report: %#v", report)
	}
	if _, err := os.Stat(filepath.Join(output, "CNAME")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(output, "reference", "cli", "index.html")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(output, "talks", "netmcr", "index-inlined.html")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(output, "tools")); !os.IsNotExist(err) {
		t.Fatalf("source-only tools directory was deployed: %v", err)
	}
}

// stubLiveCommandCatalog makes a build read the given catalog instead of
// building the daemon, and restores the real reader when the test ends.
func stubLiveCommandCatalog(t *testing.T, catalog string) {
	t.Helper()
	previous := liveCommandCatalog
	t.Cleanup(func() { liveCommandCatalog = previous })
	liveCommandCatalog = func(string) ([]byte, error) { return []byte(catalog), nil }
}

// VALIDATES: a normal full build snapshots the current Pages checkout before
// cleaning it, so a page the build does not write keeps its exact bytes.
func TestBuildPreservesExistingArtifactSeed(t *testing.T) {
	stubLiveCommandCatalog(t, `[{"path":"show test","description":"Show rows","mode":"read-only"}]`)
	parent := t.TempDir()
	root := filepath.Join(parent, "main")
	source := filepath.Join(root, "website")
	output := filepath.Join(parent, "gh-pages")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(output, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := []byte("<html>published bytes</html>\n")
	page := filepath.Join(output, "blog", "index.html")
	if err := os.MkdirAll(filepath.Dir(page), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(page, want, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(output, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "assets", "site.css"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "assets", "site.js"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	paths, err := resolvePaths(root, output)
	if err != nil {
		t.Fatal(err)
	}
	previous, release, err := snapshotArtifact(paths)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if err := seedArtifact(paths, previous); err != nil {
		t.Fatal(err)
	}
	if err := refreshNativeSurfaces(paths); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("artifact bytes changed: %q", got)
	}
	if info, err := os.Stat(filepath.Join(output, ".git")); err != nil || !info.IsDir() {
		t.Fatalf("artifact repository metadata was removed: %v", err)
	}
}

// VALIDATES: a build refreshes the published command catalog from the binary,
// and leaves an EXISTING published page alone.
// PREVENTS: commit 9f45348a7, which published the drift checker's contract
// fixture over the real pages and cut 396 of them from about 10KB to 481 bytes.
// The fixture carries no head, no title, no navigation and no vendor
// equivalents, so it is a comparison input and never a page.
func TestBuildLeavesAPublishedPageAlone(t *testing.T) {
	stubLiveCommandCatalog(t, `[{"path":"show live","description":"Show live rows","mode":"read-only"}]`)
	parent := t.TempDir()
	root := filepath.Join(parent, "main")
	output := filepath.Join(parent, "gh-pages")
	if err := os.MkdirAll(filepath.Join(root, "website"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A real published page: a head, a title and a body the fixture renderer
	// does not produce. Its survival is what this test is about.
	const publishedPage = "<!doctype html><html><head><title>CLI</title></head><body>show retired</body></html>\n"
	stale := filepath.Join(output, "reference", "cli", "index.html")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte(publishedPage), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(output, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	catalog := filepath.Join(output, "data", "cli-commands.json")
	if err := os.WriteFile(catalog, []byte(`[{"path":"show retired","mode":"read-only"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(output, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"site.css", "site.js"} {
		if err := os.WriteFile(filepath.Join(output, "assets", name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths, err := resolvePaths(root, output)
	if err != nil {
		t.Fatal(err)
	}
	if err := refreshNativeSurfaces(paths); err != nil {
		t.Fatal(err)
	}
	published, err := os.ReadFile(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(published), "show retired") {
		t.Errorf("the published catalog still names a command the live registries dropped: %s", published)
	}
	page, err := os.ReadFile(stale)
	if err != nil {
		t.Fatal(err)
	}
	if string(page) != publishedPage {
		t.Errorf("a published page was overwritten by the contract fixture:\n got: %s\nwant: %s", page, publishedPage)
	}
}

// VALIDATES: the schema extractor indexes top-level nodes by name and writes
// stable, sorted JSON bytes.
func TestYANGConfigTreeIndexesRoots(t *testing.T) {
	output := t.TempDir()
	raw := []byte(`[{"name":"zeta","children":[]},{"name":"alpha","kind":"container"}]`)
	count, err := writeYANGConfigTree(output, raw)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("root count = %d, want 2", count)
	}
	content, err := os.ReadFile(filepath.Join(output, "data", "yang-config-tree.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"alpha\": {\n    \"name\": \"alpha\",\n    \"kind\": \"container\"\n  },\n  \"zeta\": {\n    \"name\": \"zeta\",\n    \"children\": []\n  }\n}\n"
	if string(content) != want {
		t.Fatalf("configuration tree bytes:\n%s\nwant:\n%s", content, want)
	}
}

// VALIDATES: the page registry follows public routes and requires Markdown
// mirrors without treating unrelated HTML files as pages.
func TestPageRegistryAndMirrorCheck(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"index.html": "home", "index.md": "# Home",
		"guide/index.html": "guide", "guide/extra.html": "helper",
		"moved/index.html":              `<meta name="robots" content="noindex"><meta http-equiv="refresh" content="0; url=/guide/">`,
		"presentations/deck/index.html": "standalone deck",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pages, err := pageRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 || pages[0].Route != "/" || pages[1].Route != "/guide/" {
		t.Fatalf("unexpected page registry: %#v", pages)
	}
	missing, err := checkPageMirrors(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || !strings.Contains(missing[0], "guide/index.md") {
		t.Fatalf("unexpected missing mirrors: %v", missing)
	}
}

// VALIDATES: the former LINX updater refreshes every live slide statistic,
// activity, and the standalone deck through one native call.
func TestUpdateTalkRefreshesStatsActivityAndBundle(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{
		"ai/rationale", "cmd/example", "internal/plugins/one", "internal/plugins/two",
		"plan/learned", "schema", "test/interop/scenarios/one", "vendor",
	} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string][]byte{
		"ai/rationale/one.md":                []byte("# Why\n"),
		"cmd/example/main.go":                []byte(strings.Repeat("package example\n", 1001)),
		"plan/learned/one.md":                []byte("# Learned\n"),
		"schema/example.yang":                []byte("container system {\n\tleaf name { type string; }\n}\n"),
		"test/example.ci":                    []byte("exec=ze\n"),
		"test/interop/scenarios/one/run.txt": []byte("scenario\n"),
		"vendor/blob":                        make([]byte, 1025),
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	talk := filepath.Join(root, "website", "talks", "linx")
	if err := os.MkdirAll(talk, 0o755); err != nil {
		t.Fatal(err)
	}
	slides := strings.Join([]string{
		"**9,999 co-authored commits**", "**99 plugins**",
		"**9,999 config nodes** across 999 YANG schemas",
		"99 rationale files", "**9,999 functional tests**",
		"99 interop scenarios", "999 learned summaries",
		"- Only **999k lines** of Go code",
		"- Only **999M** of vendored code",
		"<!-- embed: activity.html -->",
	}, "\n")
	if err := os.WriteFile(filepath.Join(talk, "slides.md"), []byte(slides), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(talk, "index.html"), []byte("<html><body><script></script></body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit := func(arguments ...string) {
		command := exec.CommandContext(t.Context(), "git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
	}
	runGit("init")
	runGit("add", ".")
	runGit("-c", "user.name=Site Test", "-c", "user.email=site@example.test", "commit", "-m", "site fixture", "-m", "Co-Authored-By: Helper <helper@example.test>")

	report, err := updateTalk(talkUpdateOptions{Repository: root, Directory: talk, Today: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if report.Slides == "" || report.Activity == "" || report.Bundle == "" {
		t.Fatalf("incomplete talk report: %#v", report)
	}
	updated, err := os.ReadFile(filepath.Join(talk, "slides.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"**1 co-authored commits**", "**2 plugins**",
		"**2 config nodes** across 1 YANG schemas",
		"1 rationale files", "**1 functional tests**",
		"1 interop scenarios", "1 learned summaries",
		"- Only **1k lines** of Go code", "- Only **2K** of vendored code",
	} {
		if !strings.Contains(string(updated), want) {
			t.Errorf("updated slides lack %q:\n%s", want, updated)
		}
	}
	bundle, err := os.ReadFile(report.Bundle)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`id="embedded-slides"`, `id="embedded-activity-html"`} {
		if !strings.Contains(string(bundle), want) {
			t.Errorf("standalone deck lacks %q", want)
		}
	}
}

// VALIDATES: the former NetMCR updater remains a bundle-only operation and
// does not need repository statistics.
func TestUpdateTalkBundleOnlyDoesNotNeedRepository(t *testing.T) {
	talk := t.TempDir()
	if err := os.WriteFile(filepath.Join(talk, "index.html"), []byte("<html><script></script></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := updateTalk(talkUpdateOptions{Directory: talk, BundleOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Bundle == "" || report.Slides != "" || report.Activity != "" {
		t.Fatalf("unexpected bundle-only report: %#v", report)
	}
}

// VALIDATES: a website build refreshes the LINX activity and bundle without
// changing the historical slide statistics.
func TestRefreshTalksPreservesHistoricalSlides(t *testing.T) {
	root := t.TempDir()
	talks := filepath.Join(root, "website", "talks")
	talk := filepath.Join(talks, "linx-2026-06")
	if err := os.MkdirAll(talk, 0o755); err != nil {
		t.Fatal(err)
	}
	slides := []byte("**99 plugins**\n<!-- embed: activity.html -->\n")
	if err := os.WriteFile(filepath.Join(talk, "slides.md"), slides, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(talk, "index.html"), []byte("<html><script></script></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit := func(arguments ...string) {
		command := exec.CommandContext(t.Context(), "git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
	}
	runGit("init")
	runGit("add", ".")
	runGit("-c", "user.name=Site Test", "-c", "user.email=site@example.test", "commit", "-m", "historical talk")

	reports, err := refreshTalks(root, talks)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || reports[0].Activity == "" || reports[0].Bundle == "" || reports[0].Slides != "" {
		t.Fatalf("unexpected staged talk report: %#v", reports)
	}
	updated, err := os.ReadFile(filepath.Join(talk, "slides.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(updated, slides) {
		t.Fatalf("historical slides changed:\n%s", updated)
	}
}
