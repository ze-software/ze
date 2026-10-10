// Design: website/AI.md -- the homepage is authored copy around six data slots
package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"
)

// homeFixture provides stable structured data, controlled articles and weeks,
// and recorded hero media. Authored website copy is not a golden oracle.
func homeFixture(t *testing.T) Paths {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "website")
	repository := repositoryRoot(t)

	copyFixture(t, filepath.Join("testdata", "published-audience.json"),
		filepath.Join(source, "data", audienceDataFile))
	copyFixture(t, filepath.Join("testdata", "published-whats-new.json"),
		filepath.Join(source, "data", whatsNewDataFile))
	copyFixture(t, filepath.Join("testdata", "published-features.json"),
		filepath.Join(source, "data", featuresDataFile))
	copyDeclarations(t, root)
	copyFixture(t, filepath.Join(repository, "website", "data", "topics.json"),
		filepath.Join(source, "data", "topics.json"))
	writeArtifactFile(t, source, blogSourceDirectory+"/reference-from-the-system.md", blogRenderingSource)
	writeArtifactFile(t, source, blogSourceDirectory+"/older.md",
		blogPostSource("Older article", "2026-08-04"))
	for _, week := range append(homeFixtureWeeks(), "2026-07-27") {
		intro := "Changes for " + week + "."
		if week == "2026-08-17" {
			intro = homeLongWeekIntro
		}
		writeArtifactFile(t, source, changesSourceDirectory+"/"+week+markdownExtension,
			strings.Replace(weekSource(week, "BGP, CLI"), "The intro.", intro, 1))
	}

	output := t.TempDir()
	writeHeroDemo(t, root, output)
	writeArtifactFile(t, output, factsFile, publishedFactsSnapshot)
	return Paths{Repository: root, Source: source, Output: output}
}

// homeLongWeekIntro crosses the summary boundary before the eighteenth word.
// The expected clipped text is stated independently at the rendered consumer.
const homeLongWeekIntro = "One two three four five six seven eight nine ten eleven twelve " +
	"thirteen fourteen fifteen sixteen seventeen eighteen nineteen twenty."

// homeFixtureWeeks are the three weeks the controlled homepage must tease,
// excluding the fourth, older input.
func homeFixtureWeeks() []string {
	return []string{"2026-08-17", "2026-08-10", "2026-08-03"}
}

// publishedFactsSnapshot is data/site-facts.json as gh-pages 2fa8fa2ad
// published it, cut to the numbers the proof strip shows.
//
// These are rendering inputs, not claims about the checkout's current counts.
// Facts freshness is checked against re-derivation elsewhere.
//
// The five proof-share keys are the exception: that build published two
// absolute counts and no share at all, so there is nothing to carry forward.
// The five are internally consistent with the published ones beside them: 1,609
// proven of 2,966 gated MUSTs over the 140 RFCs Ze implements, inside the 2,972
// that build gated across its 171 enrolled RFCs.
const publishedFactsSnapshot = `{
  "interop": {"target_display": "9"},
  "rfc": {"enrolled_display": "171", "gated_must_display": "2,972",
          "gated_implemented_display": "2,966", "implemented_display": "140",
          "inspected_display": "171", "proven_display": "1,609",
          "proven_percent": "54.2"},
  "tests": {"e2e_display": "1,700+", "fuzz_display": "78", "unit_display": "23,700+"}
}`

// writeHeroDemo lays down the recorded demonstration the homepage hero replays,
// under the id the page names. The media goes in the artifact tree, which is
// where a render writes it and where the catalog reads it.
func writeHeroDemo(t *testing.T, root, output string) {
	t.Helper()
	media := filepath.Join(output, "assets", "demos")
	if err := os.MkdirAll(media, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "demos", "terminal"), 0o755); err != nil {
		t.Fatal(err)
	}
	const cast = `{"version":2,"width":137,"height":36}` + "\n" +
		`[0.5,"o","ze> "]` + "\n" +
		`[12.5,"o","done\r\n"]` + "\n"
	writeDemoAsset(t, media, homeHeroDemo+".cast", cast)
	writeDemoAsset(t, media, homeHeroDemo+".txt", "ze> show bgp summary\nPeer 192.0.2.1 established\n")
	writeDemoJSON(t, filepath.Join(root, "demos", "terminal", "manifest.json"), map[string]any{
		"schema":       2,
		"renderer":     map[string]any{"name": "ze-demo", "version": "3", "image": "img", "platform": "linux/native"},
		"gallery-page": "guide/terminal-demonstrations.md",
		"demos": []any{map[string]any{
			"id": homeHeroDemo, "title": "Live BGP dashboard", "description": "Operate BGP from the dashboard.",
			"page": "guide/quickstart.md", "anchor": "dashboard", "platform": "portable",
			"kind": "terminal", "engine": "ze-demo", "source": "tape", "validate": "check",
		}, map[string]any{
			"id": homeTourDemo, "title": "Web tour", "description": "Commit from the browser.",
			"page": "guide/quickstart.md", "anchor": "web-tour", "platform": "portable",
			"kind": "browser", "engine": "playwright", "source": "run.cjs", "validate": "check",
		}},
	})
	writeDemoJSON(t, filepath.Join(media, "manifest.json"), map[string]any{
		"schema":   2,
		"renderer": map[string]any{"name": "ze-demo", "version": "3", "image": "img", "platform": "linux/native"},
		"demos": map[string]any{homeHeroDemo: map[string]any{
			"release": "26.08.27",
			"assets": map[string]any{
				"cast":       demoAssetRecord(t, media, homeHeroDemo+".cast"),
				"transcript": demoAssetRecord(t, media, homeHeroDemo+".txt"),
			},
		}},
	})
}

// renderHomeFixture builds the homepage and answers the page and its mirror.
func renderHomeFixture(t *testing.T) (page, mirror string) {
	t.Helper()
	paths := homeFixture(t)
	routes, err := renderHome(paths)
	if err != nil {
		t.Fatalf("render the homepage: %v", err)
	}
	if len(routes) != 1 || routes[0] != homeRoute {
		t.Fatalf("the homepage claimed %v, want [%s]", routes, homeRoute)
	}
	return readArtifact(t, paths.Output, homeDest), readArtifact(t, paths.Output, pageMirrorFile)
}

// VALIDATES: the full homepage wires its generated article/week links into
// both HTML and Markdown, without allowing editorial copy to become a golden.
func TestTheHomepageLinksItsGeneratedNews(t *testing.T) {
	page, mirror := renderHomeFixture(t)
	body, err := extractMain(page)
	if err != nil {
		t.Fatal(err)
	}
	targets := linkTargets(body)
	for _, route := range []string{
		"blog/reference-from-the-system/",
		"project/changes/2026-08-17/",
		"project/changes/2026-08-10/",
		"project/changes/2026-08-03/",
	} {
		if !slices.Contains(targets, route) {
			t.Errorf("the homepage does not link %s", route)
		}
		if !strings.Contains(mirror, "]("+siteBase+route+")") {
			t.Errorf("the homepage mirror does not preserve the absolute link for %s", route)
		}
	}
	for _, route := range []string{"blog/older/", "project/changes/2026-07-27/"} {
		if slices.Contains(targets, route) {
			t.Errorf("the homepage selected an older input %s", route)
		}
	}
	if strings.Contains(body, "<renderer>") {
		t.Error("the authored article title became homepage markup")
	}
}

// demoAssetURLPattern matches the version query one demo asset URL carries. The
// value is the first ten characters of the recording's digest, so it moves with
// the recording rather than with the page.
var demoAssetURLPattern = regexp.MustCompile(`(assets/demos/[a-z0-9-]+\.(?:cast|txt))\?v=[0-9a-f]+`)

// linkTargets answers every address one fragment links, in document order, with
// the demo recordings' digests normalised away.
func linkTargets(fragment string) []string {
	var targets []string
	tokenizer := xhtml.NewTokenizer(strings.NewReader(fragment))
	for {
		//exhaustive:ignore // Projects hrefs from opening anchors; other tokens cannot contribute link targets.
		switch tokenizer.Next() {
		case xhtml.ErrorToken:
			return targets
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			name, hasAttributes := tokenizer.TagName()
			if string(name) != "a" {
				continue
			}
			for hasAttributes {
				var key, value []byte
				key, value, hasAttributes = tokenizer.TagAttr()
				if string(key) == "href" {
					targets = append(targets, demoAssetURLPattern.ReplaceAllString(string(value), "$1"))
				}
			}
		case xhtml.TextToken, xhtml.EndTagToken, xhtml.CommentToken, xhtml.DoctypeToken:
			// A link is an opening tag and nothing else carries an href.
		}
	}
}

// firstMismatch answers the two strings cut to where they first differ, with
// enough after the cut to read the mismatch.
//
// It works on characters rather than on lines, because visibleText answers one
// long line: mirror_test.go's firstDifference would report "line 1" and print
// both whole pages.
func firstMismatch(got, want string) (string, string) {
	limit := min(len(got), len(want))
	for index := range limit {
		if got[index] != want[index] {
			start := max(index-60, 0)
			return got[start:min(index+80, len(got))], want[start:min(index+80, len(want))]
		}
	}
	return got[min(limit, len(got)):], want[min(limit, len(want)):]
}

// VALIDATES: the proof strip carries one span for each published number, each
// naming the fact it shows and carrying that fact's value, and the RFC card
// states its share with both populations it is drawn from.
//
// The attribute is what lets a reader check the number against the snapshot it
// came from, and visibleText cannot see an attribute, so this reads the markup.
// PREVENTS: the card publishing an absolute count with no denominator, which
// counts obligations JUDGED and is read as obligations MET (owner directive,
// 2026-09-01).
func TestTheHomepageProofStripCarriesItsStatSpans(t *testing.T) {
	page, _ := renderHomeFixture(t)
	spans := []string{
		`<span data-ze-stat="tests.unit_display">23,700+</span>`,
		`<span data-ze-stat="tests.e2e_display">1,700+</span>`,
		`<span data-ze-stat="tests.fuzz_display">78</span>`,
		`<span data-ze-stat="interop.target_display">9</span>`,
		`<span data-ze-stat="rfc.proven_display">1,609</span>`,
		`<span data-ze-stat="rfc.gated_implemented_display">2,966</span>`,
		`<span data-ze-stat="rfc.proven_percent">54.2</span>`,
		`<span data-ze-stat="rfc.implemented_display">140</span>`,
	}
	for _, span := range spans {
		if !strings.Contains(page, span) {
			t.Errorf("the proof strip does not carry %s", span)
		}
	}
	if count := strings.Count(page, `data-ze-stat=`); count != len(spans) {
		t.Errorf("the homepage carries %d stat spans, want %d", count, len(spans))
	}
}

// VALIDATES: a fact the snapshot does not state stops the build by name, rather
// than publishing a blank where a number belongs.
func TestAFactTheSnapshotLostStopsTheHomepage(t *testing.T) {
	paths := homeFixture(t)
	writeArtifactFile(t, paths.Output, factsFile,
		`{"interop":{"target_display":"9"},
		  "rfc":{"enrolled_display":"171","gated_must_display":"2,972",
		         "gated_implemented_display":"2,966","implemented_display":"140",
		         "inspected_display":"171","proven_percent":"54.2"},
		  "tests":{"e2e_display":"1,700+","fuzz_display":"78","unit_display":"23,700+"}}`)
	_, err := renderHome(paths)
	if err == nil {
		t.Fatal("a snapshot with no proven count published a homepage with a blank")
	}
	if !strings.Contains(err.Error(), "rfc.proven_display") {
		t.Errorf("the refusal says %q, which does not name the fact it is missing", err)
	}
}

// VALIDATES: both card grids keep the data file's own order, and every card
// carries its category, its chips and its call to action.
//
// The order is the author's argument about what a reader should try first. A
// sort here would reorder it whenever a card's title changed.
func TestTheHomepageCardsKeepTheDataFilesOwnOrder(t *testing.T) {
	paths := homeFixture(t)
	page, _ := renderHomeFixture(t)
	var audience audienceData
	if err := readSourceJSON(paths.Source, audienceDataFile, &audience); err != nil {
		t.Fatal(err)
	}
	if len(audience.Run) != 3 || len(audience.Who) != 7 {
		t.Fatalf("the published audience file holds %d run cards and %d use cases, want 3 and 7",
			len(audience.Run), len(audience.Who))
	}
	previous := 0
	for _, card := range append(append([]audienceCard{}, audience.Run...), audience.Who...) {
		at := strings.Index(page, ">"+card.Title+"</a></h3>")
		if at < 0 {
			t.Fatalf("the homepage shows no card titled %q", card.Title)
		}
		if at < previous {
			t.Errorf("the card %q is published before the one that precedes it in the data file", card.Title)
		}
		previous = at
	}
	if got := strings.Count(page, `class="card audience-card`); got != 10 {
		t.Errorf("the homepage shows %d audience cards, want 10", got)
	}
}

// VALIDATES: the feature category buttons follow the PAGE's own category order
// and each states the number of shipped and experimental cards it holds.
//
// A list derived from the data reorders its buttons whenever a card moves, and
// drops a category no card carries.
func TestTheFeatureCategoryLinksFollowThePagesOwnOrder(t *testing.T) {
	paths := homeFixture(t)
	var features featureData
	if err := readSourceJSON(paths.Source, featuresDataFile, &features); err != nil {
		t.Fatal(err)
	}
	links := featureCategoryLinks(features)
	want := []string{"Operate (6)", "Routing (9)", "Services (9)", "Automate (7)",
		"Observe (9)", "Secure (6)", "Platform (6)"}
	previous := 0
	for _, label := range want {
		at := strings.Index(links, ">"+label+"</a>")
		if at < 0 {
			t.Fatalf("the features summary shows no %q button:\n%s", label, links)
		}
		if at < previous {
			t.Errorf("the %q button is out of the page's own category order", label)
		}
		previous = at
	}
	if got := strings.Count(links, "<a class="); got != len(legendCategories) {
		t.Errorf("the features summary shows %d buttons, want %d", got, len(legendCategories))
	}
}

// VALIDATES: the Latest news band takes the newest article and the newest week,
// and cuts each summary to one line on a word boundary.
func TestTheLatestNewsBandTakesTheNewestArticleAndWeek(t *testing.T) {
	page, mirror := renderHomeFixture(t)
	band := sliceBetween(t, page, `<section class="whats-new reveal"`, "</section>")
	const clipped = "One two three four five six seven eight nine ten eleven twelve " +
		"thirteen fourteen fifteen sixteen seventeen…"
	for _, want := range []string{
		`<span class="whats-new-label">Engineering note</span>`,
		`<h3><a href="blog/reference-from-the-system/">A &lt;renderer&gt; &amp; its sources</a></h3>`,
		`<span class="whats-new-label">Recently shipped</span>`,
		`<h3><a href="project/changes/2026-08-17/">Week of 2026-08-17</a></h3>`,
		"<p>" + clipped + "</p>",
		`<p>An index description, not the deck.</p>`,
		`<span class="whats-new-label">RFC compliance progress</span>`,
	} {
		if !strings.Contains(band, want) {
			t.Errorf("the Latest news band does not carry %s", want)
		}
	}
	if strings.Contains(band, homeLongWeekIntro) {
		t.Error("the Latest news band published the unclipped weekly intro")
	}
	if !strings.Contains(mirror, clipped) {
		t.Error("the homepage mirror lost the clipped weekly summary")
	}
}

// VALIDATES: a summary is cut on a word boundary, in runes, and a summary that
// already fits is left alone.
func TestASummaryIsCutOnAWordBoundary(t *testing.T) {
	short := "One line that already fits."
	if got := clipSummary(short); got != short {
		t.Errorf("a short summary was changed: got %q", got)
	}
	if got := clipSummary("  spaced   out\n text "); got != "spaced out text" {
		t.Errorf("whitespace was not collapsed: got %q", got)
	}
	long := strings.Repeat("mesure ", 30)
	cut := clipSummary(long)
	if !strings.HasSuffix(cut, "…") {
		t.Errorf("a long summary was not marked as cut: %q", cut)
	}
	if runes := len([]rune(cut)); runes > homeSummaryRunes+1 {
		t.Errorf("a cut summary is %d runes long, want at most %d", runes, homeSummaryRunes+1)
	}
	if strings.Contains(strings.TrimSuffix(cut, "…"), "mesur\u00a0") {
		t.Errorf("the cut fell inside a word: %q", cut)
	}
	accented := strings.Repeat("é", homeSummaryRunes+20)
	if runes := len([]rune(clipSummary(accented))); runes > homeSummaryRunes+1 {
		t.Errorf("an accented summary was cut by bytes rather than by runes: %d runes", runes)
	}
}

// VALIDATES: a teaser shows one tag per category first, so a card states the
// breadth of a week rather than four shades of one subject.
func TestATeaserShowsOneTagPerCategoryFirst(t *testing.T) {
	topics := []changeTopic{
		{Key: "BGP", Label: "BGP", Category: categoryRouting},
		{Key: "Route Server", Label: "Route Server", Category: categoryRouting},
		{Key: "RPKI", Label: "RPKI", Category: categoryRouting},
		{Key: "CLI", Label: "CLI", Category: categoryOperate},
		{Key: "IPsec", Label: "IPsec", Category: categorySecure},
	}
	picked := pickHomeUpdateTopics(topics)
	if len(picked) != homeUpdateTags {
		t.Fatalf("a teaser shows %d tags, want %d", len(picked), homeUpdateTags)
	}
	if picked[0].Key != "BGP" || picked[1].Key != "CLI" || picked[2].Key != "IPsec" {
		t.Errorf("the first three tags are %q, %q, %q, want one per category",
			picked[0].Key, picked[1].Key, picked[2].Key)
	}
	if picked[3].Key != "Route Server" {
		t.Errorf("the fourth tag is %q, want the week's next tag in its own order", picked[3].Key)
	}
	if got := len(pickHomeUpdateTopics(topics[:1])); got != 1 {
		t.Errorf("a week with one tag shows %d, want 1", got)
	}
	if got := len(pickHomeUpdateTopics(nil)); got != 0 {
		t.Errorf("a week with no tag shows %d, want none", got)
	}
}

// VALIDATES: the three teasers are the three newest weeks, numbered from one
// and toned by position.
func TestTheTeasersAreTheThreeNewestWeeks(t *testing.T) {
	page, _ := renderHomeFixture(t)
	for index, week := range homeFixtureWeeks() {
		card := `<article class="card card-post home-update-card tone-` + homeTeaserTones[index] + `">`
		at := strings.Index(page, card)
		if at < 0 {
			t.Fatalf("teaser %d carries no %s tone", index+1, homeTeaserTones[index])
		}
		rest := page[at:]
		if !strings.Contains(rest[:600], `<span class="home-update-number">0`+strconv.Itoa(index+1)+`</span>`) {
			t.Errorf("teaser %d is not numbered %02d", index+1, index+1)
		}
		if !strings.Contains(rest[:600], "Week of "+week) {
			t.Errorf("teaser %d does not name the week %s", index+1, week)
		}
	}
	if got := strings.Count(page, `class="card card-post home-update-card`); got != homeTeasers {
		t.Errorf("the homepage shows %d teasers, want %d", got, homeTeasers)
	}
}

// VALIDATES: a homepage no page can be made from is refused by name, rather
// than published with a hole a reader meets before anything else on the site.
func TestAHomepageInputNoPageCanBeMadeFromIsRefused(t *testing.T) {
	for _, refusal := range []struct {
		what   string
		break_ func(t *testing.T, paths Paths)
		says   string
	}{
		{"an audience file with no run path", func(t *testing.T, paths Paths) {
			writeFixtureFile(t, filepath.Join(paths.Source, "data", audienceDataFile),
				`{"run":[],"who":[{"title":"T","body":"B"}]}`)
		}, "states no run path"},
		{"a card with no body", func(t *testing.T, paths Paths) {
			writeFixtureFile(t, filepath.Join(paths.Source, "data", audienceDataFile),
				`{"run":[{"title":"Run it","body":""}],"who":[{"title":"T","body":"B"}]}`)
		}, "states no body"},
		{"a card whose link has no label", func(t *testing.T, paths Paths) {
			writeFixtureFile(t, filepath.Join(paths.Source, "data", audienceDataFile),
				`{"run":[{"title":"Run it","body":"B","link":{"href":"labs/"}}],
				  "who":[{"title":"T","body":"B"}]}`)
		}, "no label"},
		{"a Latest news file with no heading", func(t *testing.T, paths Paths) {
			writeFixtureFile(t, filepath.Join(paths.Source, "data", whatsNewDataFile), `{"note":null}`)
		}, "states no title"},
		{"a hero recording the manifest does not name", func(t *testing.T, paths Paths) {
			writeDemoJSON(t, filepath.Join(paths.Repository, "demos", "terminal", "manifest.json"), map[string]any{
				"schema":       2,
				"renderer":     map[string]any{"name": "ze-demo", "version": "3", "image": "img", "platform": "linux/native"},
				"gallery-page": "guide/terminal-demonstrations.md",
				"demos": []any{map[string]any{
					"id": "other", "title": "Other", "description": "Other.",
					"page": "guide/quickstart.md", "anchor": "other", "platform": "portable",
					"kind": "terminal", "engine": "ze-demo", "source": "tape", "validate": "check",
				}},
			})
		}, "unknown terminal demo: " + homeHeroDemo},
	} {
		t.Run(refusal.what, func(t *testing.T) {
			paths := homeFixture(t)
			refusal.break_(t, paths)
			if _, err := renderHome(paths); err == nil {
				t.Fatalf("%s published a homepage instead of stopping the build", refusal.what)
			} else if !strings.Contains(err.Error(), refusal.says) {
				t.Errorf("%s said %q, which does not name %q", refusal.what, err, refusal.says)
			}
		})
	}
}

// VALIDATES: the template and the producer agree about the slots, in both
// directions, and a reader's own braces are not mistaken for one.
//
// A slot nobody fills publishes its own name to a reader. A slot the template
// lost leaves its content off the page with nothing to notice, which is the
// shape of the defect this whole spec exists to fix.
func TestTheTemplateAndTheProducerAgreeAboutEverySlot(t *testing.T) {
	// Fourteen since 2026-09-21, when 02b164de51 rewrote the RFC proof card
	// again and deleted {rfc_walked} from the template and from the producer
	// together. It left this pin at fifteen, the count 5fc257bde3 set on
	// 2026-09-09 when it deleted {rfc_gated_must}.
	names := homeSlotPattern.FindAllString(homeTemplate, -1)
	if len(names) != 14 {
		t.Errorf("the homepage template has %d slots, want 14: %v", len(names), names)
	}
	pairs := make([]string, 0, 2*len(names))
	for _, name := range names {
		pairs = append(pairs, name, "")
	}
	if err := everySlotFilled(pairs); err != nil {
		t.Errorf("the template's own slots were refused: %v", err)
	}
	if err := everySlotFilled([]string{"{hero_demo}", ""}); err == nil {
		t.Error("a template slot nothing fills was published as its own name")
	}
	if err := everySlotFilled(append(append([]string{}, pairs...), "{invented}", "")); err == nil {
		t.Error("a slot the template does not have was accepted")
	}

	// A reader's own braces are text, not a slot.
	paths := homeFixture(t)
	writeFixtureFile(t, filepath.Join(paths.Source, "data", whatsNewDataFile), `{"title":"Latest news",
		"link":{"href":"project/changes/","label":"All updates"},
		"note":{"label":"Note","title":"A {template} in prose","body":"Braces {like_this} are text."}}`)
	if _, err := renderHome(paths); err != nil {
		t.Errorf("a note carrying braces stopped the build: %v", err)
	}
	page := readArtifact(t, paths.Output, homeDest)
	if !strings.Contains(page, "A {template} in prose") {
		t.Error("the note's braces did not reach the reader")
	}
}

// VALIDATES: the hero replays the recording the manifest names, sized from the
// recording itself, and a recording that is not a terminal one is refused.
//
// The hero used to spell its asset paths and digests by hand, which made it a
// fourth place the asset set was written down and the only one no render could
// correct.
func TestTheHeroReplaysTheRecordingTheManifestNames(t *testing.T) {
	paths := homeFixture(t)
	mount, err := newDemoCatalog(paths).heroMount(homeHeroDemo, homeRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-cols="137"`, `data-rows="36"`,
		`aria-label="Live BGP dashboard"`,
		`assets/demos/` + homeHeroDemo + `.cast?v=`,
		`assets/demos/` + homeHeroDemo + `.txt?v=`,
	} {
		if !strings.Contains(mount, want) {
			t.Errorf("the hero mount does not carry %s:\n%s", want, mount)
		}
	}

	video := homeFixture(t)
	manifest := filepath.Join(video.Repository, "demos", "terminal", "manifest.json")
	content, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatal(err)
	}
	demos, listed := decoded["demos"].([]any)
	if !listed || len(demos) == 0 {
		t.Fatalf("the fixture manifest lists no demonstration: %v", decoded["demos"])
	}
	first, isObject := demos[0].(map[string]any)
	if !isObject {
		t.Fatalf("the fixture manifest's first demonstration is not an object: %v", demos[0])
	}
	first["kind"] = "video"
	writeDemoJSON(t, manifest, decoded)
	if _, err := newDemoCatalog(video).heroMount(homeHeroDemo, homeRoot); err == nil {
		t.Error("a video recording was mounted in the hero's terminal frame")
	} else if !strings.Contains(err.Error(), "replays a terminal") {
		t.Errorf("the refusal says %q, which does not say the frame replays a terminal", err)
	}
}

// VALIDATES: the homepage carries the shared page shell and a Markdown mirror
// beside it, which the retired build published for every route but this one.
//
// `./le site check` refuses a published route with no mirror, so the homepage
// was the one route the mirror contract never covered.
func TestTheHomepageCarriesTheShellAndItsMirror(t *testing.T) {
	page, mirror := renderHomeFixture(t)
	for _, want := range []string{
		"<!doctype html>", `<script id="theme-bootstrap">`,
		`<title>Ze - Configuration and Protocol Engine for Internet Infrastructure</title>`,
		`<link rel="canonical" href="https://ze-software.net/" />`,
		`<meta property="og:url" content="https://ze-software.net/" />`,
		`<link rel="stylesheet" href="assets/site.css" />`,
		`<link rel="stylesheet" href="assets/vendor/asciinema-player.css" />`,
		`<div id="site-header-mount" data-header-src="assets/header.html"`,
		"<noscript>", `<main id="top" tabindex="-1">`,
		"<footer>", `<span class="footer-published">`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the homepage carries no %s", want)
		}
	}
	if strings.Contains(page, `<main id="top" tabindex="-1" class=`) {
		t.Error("the homepage opened <main> with a class; it carries no sidebar and asks for no width")
	}
	for _, want := range []string{
		"A <renderer> & its sources", "Week of 2026-08-17",
	} {
		if !strings.Contains(mirror, want) {
			t.Errorf("the homepage mirror does not carry %q", want)
		}
	}
	if strings.Contains(mirror, "<article") || strings.Contains(mirror, "<section") {
		t.Error("the homepage mirror carries page markup rather than Markdown")
	}
}

// VALIDATES: the home producer claims exactly the root it wrote, with both
// page and mirror artifacts. renderHomeFixture checks this producer answer.
func TestTheHomeProducerClaimsTheSiteRoot(t *testing.T) {
	renderHomeFixture(t)
}

// VALIDATES: spec-terminal-demo-showcase AC-8. The hero's file name, caption,
// player label and transcript link, and the tour link beside it, come from
// homeHeroDemo and homeTourDemo plus their manifest entries: a fixture manifest
// with its own title, page and anchor reaches the rendered homepage, and the
// authored template names no demonstration id or anchor of the real manifest.
// PREVENTS: a hero pointed at a new recording that still shows the old one's
// caption and links to the old one's transcript.
func TestHomeHeroDerivesFromTheManifest(t *testing.T) {
	paths := homeFixture(t)
	manifestPath := filepath.Join(paths.Repository, "demos", "terminal", "manifest.json")
	var manifest map[string]any
	readDemoJSON(t, manifestPath, &manifest)
	hero, isObject := manifest["demos"].([]any)[0].(map[string]any)
	if !isObject {
		t.Fatal("the fixture manifest's first demonstration is not an object")
	}
	hero["title"] = "Fixture hero caption"
	hero["anchor"] = "fixture-hero-anchor"
	writeDemoJSON(t, manifestPath, manifest)

	if _, err := renderHome(paths); err != nil {
		t.Fatalf("render the homepage: %v", err)
	}
	page := readArtifact(t, paths.Output, homeDest)
	destination, err := docsDestination("guide/quickstart.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<span>" + homeHeroDemo + ".terminal</span>",
		`<strong id="hero-demo-title">Fixture hero caption</strong>`,
		`aria-label="Fixture hero caption"`,
		`<a href="` + destination + `/#fixture-hero-anchor">Read transcript</a>`,
		`<a class="hero-start-action" href="` + destination + `/#web-tour">`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the homepage carries no %s", want)
		}
	}

	var real struct {
		Demos []struct{ ID, Anchor string } `json:"demos"`
	}
	readDemoJSON(t, filepath.Join(repositoryRoot(t), "demos", "terminal", "manifest.json"), &real)
	if strings.Contains(homeTemplate, "demos/terminal/#") {
		t.Error("the homepage template links a gallery anchor literally")
	}
	for _, demo := range real.Demos {
		if strings.Contains(homeTemplate, demo.ID) {
			t.Errorf("the homepage template names demonstration %s literally", demo.ID)
		}
		if strings.Contains(homeTemplate, "#"+demo.Anchor) {
			t.Errorf("the homepage template names anchor %s literally", demo.Anchor)
		}
	}
}

// readDemoJSON decodes one manifest into value.
func readDemoJSON(t *testing.T, path string, value any) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, value); err != nil {
		t.Fatal(err)
	}
}
