package site

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// modelFactKeys answers every dotted key the siteFacts model's JSON tags name
// down to a scalar leaf, read from the type rather than from a file.
//
// It is the test's own derivation, independent of the snapshot walk the
// producers use, so a key the model names and the walk misses is a red here. A
// map-typed section names its keys only in a value, so it is not walked.
func modelFactKeys(model reflect.Type, prefix string) []string {
	var keys []string
	for field := range model.Fields() {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		key := prefix + name
		switch field.Type.Kind() {
		case reflect.Struct:
			keys = append(keys, modelFactKeys(field.Type, key+".")...)
		case reflect.Map:
			continue
		default:
			keys = append(keys, key)
		}
	}
	return keys
}

// VALIDATES: AC-3. A {{ze:<dotted.key>}} token resolves for every key the
// snapshot model names, with no name table between the token and the key.
//
// The method writes the snapshot a build writes, then asks for every key the
// siteFacts JSON tags name, through the same substitution a page uses, in both
// the marked and the plain form.
func TestATokenResolvesAnyKeyTheSnapshotNames(t *testing.T) {
	paths := factsFixture(t)
	if err := publishSiteFacts(paths); err != nil {
		t.Fatalf("publish the facts snapshot: %v", err)
	}
	snapshot, err := loadSiteFactSnapshot(paths.Output)
	if err != nil {
		t.Fatal(err)
	}
	keys := modelFactKeys(reflect.TypeFor[siteFacts](), "")
	if len(keys) < 40 {
		t.Fatalf("the model walk found %d keys; the siteFacts model names more", len(keys))
	}
	for _, key := range keys {
		value, found := snapshot.value(key)
		if !found {
			t.Errorf("the snapshot this build wrote answers nothing for %s", key)
			continue
		}
		plain, err := snapshot.substitute("A {{ze:"+key+"}} B", false)
		if err != nil || plain != "A "+value+" B" {
			t.Errorf("{{ze:%s}} reads %q in a mirror (%v), want %q", key, plain, err, value)
		}
		marked, err := snapshot.substitute("A {{ze:"+key+"}} B", true)
		if err != nil || marked != "A "+markedFact(key, value)+" B" {
			t.Errorf("{{ze:%s}} reads %q on a page (%v)", key, marked, err)
		}
		if !slices.Contains(snapshot.keys(), key) {
			t.Errorf("the published key set omits %s", key)
		}
	}
}

// VALIDATES: AC-3. A token naming a key the snapshot does not hold stops the
// build, and the error names the token and how to list the valid keys.
func TestAnUnknownTokenIsRefusedByName(t *testing.T) {
	snapshot, err := parseSiteFactSnapshot([]byte(`{"interop":{"scenarios":137,"scenarios_display":"130+"}}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{
		"Ze runs {{ze:interop-scenarios}} scenarios.",
		"Ze runs {{ze:interop}} scenarios.",
		"Ze runs {{ze:interop.scenarios_display.more}} scenarios.",
	} {
		_, err := snapshot.substitute(source, true)
		if err == nil {
			t.Errorf("%q resolved, but the snapshot names no such value", source)
			continue
		}
		token := source[strings.Index(source, "{{") : strings.Index(source, "}}")+2]
		for _, want := range []string{token, "data/site-facts.json", "jq -r"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the refusal of %s does not say %q: %v", token, want, err)
			}
		}
		if _, err := snapshot.substituteHTML("<html><body><p>" + source + "</p></body></html>"); err == nil {
			t.Errorf("an HTML page resolved %s", token)
		}
	}
	_, err = snapshot.substitute("Ze runs {{ze:interop-scenarios}} scenarios.", true)
	if err == nil || !strings.Contains(err.Error(), "interop.scenarios") {
		t.Errorf("the refusal suggests no key the snapshot holds: %v", err)
	}
}

// VALIDATES: AC-4, risk R-1. In hand-written HTML a token in a text node of the
// body becomes a marked span, and the same token in an attribute, in the head
// or in a raw-text element becomes the plain value, so no span lands inside
// markup a browser reads as text.
func TestATokenInAnAttributeOrHeadStaysPlain(t *testing.T) {
	snapshot, err := parseSiteFactSnapshot([]byte(`{"cli_commands":490,"tests":{"unit_display":"23,700+"}}`))
	if err != nil {
		t.Fatal(err)
	}
	source := strings.Join([]string{
		`<!doctype html><html><head>`,
		`<title>Ze answers {{ze:cli_commands}} commands</title>`,
		`<meta name="description" content="{{ze:cli_commands}} commands">`,
		`</head><body><main>`,
		`<p title="{{ze:tests.unit_display}} tests">Ze runs {{ze:tests.unit_display}} unit tests.</p>`,
		`<script>const count = "{{ze:cli_commands}}";</script>`,
		`</main></body></html>`,
	}, "\n")
	page, err := snapshot.substituteHTML(source)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		`<!doctype html><html><head>`,
		`<title>Ze answers 490 commands</title>`,
		`<meta name="description" content="490 commands">`,
		`</head><body><main>`,
		`<p title="23,700+ tests">Ze runs <span data-ze-stat="tests.unit_display">23,700+</span> unit tests.</p>`,
		`<script>const count = "490";</script>`,
		`</main></body></html>`,
	}, "\n")
	if page != want {
		t.Errorf("the page reads\n%s\nwant\n%s", page, want)
	}
	hostile, err := parseSiteFactSnapshot([]byte(`{"name":"<b>&"}`))
	if err != nil {
		t.Fatal(err)
	}
	escaped, err := hostile.substituteHTML(`<p a="{{ze:name}}">{{ze:name}}</p>`)
	if err != nil {
		t.Fatal(err)
	}
	if escaped != `<p a="&lt;b&gt;&amp;">`+markedFact("name", "<b>&")+`</p>` || strings.Contains(escaped, "<b>") {
		t.Errorf("a value reached the page unescaped: %s", escaped)
	}
}

// VALIDATES: AC-5. A first build, with no snapshot in the artifact yet, leaves
// every token alone in Markdown and in HTML and refuses none of them.
func TestTokensWaitForTheFirstSnapshot(t *testing.T) {
	snapshot, err := loadSiteFactSnapshot(t.TempDir())
	if err != nil {
		t.Fatalf("an artifact with no snapshot is a first build, not an error: %v", err)
	}
	for _, source := range []string{"Ze runs {{ze:tests.unit_display}} tests.", "Ze has {{ze:invented-count}}."} {
		for _, marked := range []bool{true, false} {
			if got, err := snapshot.substitute(source, marked); err != nil || got != source {
				t.Errorf("a first build changed %q into %q (%v)", source, got, err)
			}
		}
		page := "<html><body><p>" + source + "</p></body></html>"
		if got, err := snapshot.substituteHTML(page); err != nil || got != page {
			t.Errorf("a first build changed %q into %q (%v)", page, got, err)
		}
	}
}

// VALIDATES: AC-1, the wiring row for every producer. Each site-wide producer
// outside the RFC section publishes the snapshot's values only inside
// data-ze-stat, whose text equals the snapshot's value for that key.
//
// The method renders each producer over its fixture with a stated snapshot,
// then asks two things of what it wrote: the plain-text guard finds no figure
// outside a marker, and every key the producer shows is a marked element
// holding the snapshot's value. It is red until the producers emit their
// values through markedFact (phase 2 and 3 of
// plan/release/validation/spec-site-values-from-one-file.md).
func TestEverySiteWideValueIsPublishedInsideItsMarker(t *testing.T) {
	headerSnapshot, err := json.Marshal(publishedHeaderFacts)
	if err != nil {
		t.Fatal(err)
	}
	header := renderSiteHeader(t)
	assertMarkedValues(t, "assets/header.html", header, headerSnapshot,
		[]string{"cli_commands", "config_sections", "dependencies", "github_stars"})

	commands := commandSurfacePaths(t)
	copyFixture(t, filepath.Join("testdata", "published-site-facts.json"), filepath.Join(commands.Output, factsFile))
	if _, err := renderCLIReference(commands); err != nil {
		t.Fatal(err)
	}
	if _, err := renderCommandEquivalents(commands); err != nil {
		t.Fatal(err)
	}
	commandSnapshot := []byte(readArtifact(t, commands.Output, factsFile))
	assertMarkedValues(t, "reference/cli/", readArtifact(t, commands.Output, "reference/cli/"+pageIndexFile),
		commandSnapshot, []string{"cli_commands"})
	assertMarkedValues(t, "reference/command-equivalents/",
		readArtifact(t, commands.Output, "reference/command-equivalents/"+pageIndexFile),
		commandSnapshot, []string{"cli_commands"})

	home := homeFixture(t)
	if _, err := renderHome(home); err != nil {
		t.Fatal(err)
	}
	assertMarkedValues(t, "index.html", readArtifact(t, home.Output, pageIndexFile),
		[]byte(publishedFactsSnapshot), []string{"tests.unit_display", "tests.e2e_display", "tests.fuzz_display"})
}

// assertMarkedValues fails the test when a page shows a snapshot value outside
// its marker, or shows a named key nowhere inside one.
func assertMarkedValues(t *testing.T, page, content string, snapshotJSON []byte, keys []string) {
	t.Helper()
	snapshot, err := parseSiteFactSnapshot(snapshotJSON)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := plainFigures(page, content, publishedFigures(snapshot))
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range findings {
		t.Errorf("%s shows %q (%s) outside its marker", finding.Page, finding.Figure, finding.Key)
	}
	for _, key := range keys {
		value, found := snapshot.value(key)
		if !found {
			t.Fatalf("the stated snapshot holds no %s", key)
		}
		if !strings.Contains(content, markedFact(key, value)) {
			t.Errorf("%s shows no %s", page, markedFact(key, value))
		}
	}
}
