// Design: website/AI.md -- owner rulings on the website's look are pinned by a test.
package site

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// A cssRule is one selector list and its declarations, as the stylesheets
// under website/assets/css write it.
type cssRule struct {
	file      string
	selectors []string
	body      string
}

var (
	cssCommentPattern   = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssClassPattern     = regexp.MustCompile(`\.([A-Za-z_][A-Za-z0-9_-]*)`)
	cssUnderlinePattern = regexp.MustCompile(`text-decoration(-line)?\s*:[^;]*underline`)
	cssNoUnderline      = regexp.MustCompile(`text-decoration\s*:\s*none`)
	cssColorPattern     = regexp.MustCompile(`(^|[;\s])color\s*:\s*[^;\s]`)
	htmlClassPattern    = regexp.MustCompile(`class="([^"]+)"`)
	cssSpacePattern     = regexp.MustCompile(`\s+`)
)

// websiteStyleRules reads every stylesheet the site publishes and answers its
// rules. A rule inside an at-rule block is answered like any other: the
// selector is the text between the last two opening braces.
func websiteStyleRules(t *testing.T) []cssRule {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	files, err := filepath.Glob(filepath.Join(root, "website", "assets", "css", "*.css"))
	if err != nil {
		t.Fatalf("list the website stylesheets: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no stylesheet under %s/website/assets/css", root)
	}
	var rules []cssRule
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		text := cssCommentPattern.ReplaceAllString(string(content), "")
		for chunk := range strings.SplitSeq(text, "}") {
			parts := strings.Split(chunk, "{")
			if len(parts) < 2 {
				continue
			}
			selector := cssSpacePattern.ReplaceAllString(strings.TrimSpace(parts[len(parts)-2]), " ")
			var selectors []string
			for one := range strings.SplitSeq(selector, ",") {
				selectors = append(selectors, strings.TrimSpace(one))
			}
			rules = append(rules, cssRule{
				file:      filepath.Base(file),
				selectors: selectors,
				body:      parts[len(parts)-1],
			})
		}
	}
	return rules
}

// homeHeroClasses answers every class the homepage hero's markup uses.
func homeHeroClasses(t *testing.T) []string {
	t.Helper()
	start := strings.Index(homeTemplate, `<section class="hero"`)
	if start < 0 {
		t.Fatal("the homepage template holds no hero section")
	}
	end := strings.Index(homeTemplate[start:], "</section>")
	if end < 0 {
		t.Fatal("the homepage hero section is not closed")
	}
	var classes []string
	for _, match := range htmlClassPattern.FindAllStringSubmatch(homeTemplate[start:start+end], -1) {
		classes = append(classes, strings.Fields(match[1])...)
	}
	slices.Sort(classes)
	return slices.Compact(classes)
}

// underlinesInHero reports whether a selector that underlines can match inside
// the hero: it names a hero class, and every class it names is one the hero
// uses, so a rule scoped to another part of the site is left alone.
func underlinesInHero(selector string, inHero []string) bool {
	reachesHero := false
	for _, class := range cssClassPattern.FindAllStringSubmatch(selector, -1) {
		if !slices.Contains(inHero, class[1]) {
			return false
		}
		if class[1] != "site-home" {
			reachesHero = true
		}
	}
	return reachesHero
}

// TestHeroLinksAreNeverUnderlined pins the owner ruling that no link in the
// homepage hero is underlined, in any state, and that a text link stands out
// by color instead (website/AI.md, "Owner rulings").
//
// It reads the published stylesheets and the hero's own markup. It fails when
// either hero-wide rule is gone, and when any rule that can match inside the
// hero underlines.
//
// PREVENTS: an edit that adds or renames a hero element bringing the browser's
// underline back, which happened each time the hero was touched while the
// underline was removed one element at a time.
func TestHeroLinksAreNeverUnderlined(t *testing.T) {
	rules := websiteStyleRules(t)
	inHero := append(homeHeroClasses(t), "site-home")

	var coversEveryLink, colorsTextLinks bool
	for _, rule := range rules {
		if slices.Contains(rule.selectors, ".site-home .hero a") &&
			slices.Contains(rule.selectors, ".site-home .hero a:hover") &&
			slices.Contains(rule.selectors, ".site-home .hero a:focus-visible") &&
			cssNoUnderline.MatchString(rule.body) {
			coversEveryLink = true
		}
		// A selector list is split on commas, so `:where(p, figcaption)`
		// arrives as two entries.
		if slices.Contains(rule.selectors, ".site-home .hero :where(p") &&
			slices.Contains(rule.selectors, "figcaption) a") &&
			cssColorPattern.MatchString(rule.body) && !strings.Contains(rule.body, "inherit") {
			colorsTextLinks = true
		}
	}
	if !coversEveryLink {
		t.Error("no rule removes the underline from every hero link: " +
			"`.site-home .hero a` with its :hover and :focus-visible states must declare `text-decoration: none`")
	}
	if !colorsTextLinks {
		t.Error("no rule colors every hero text link: " +
			"`.site-home .hero :where(p, figcaption) a` must declare a color of its own")
	}

	for _, rule := range rules {
		if !cssUnderlinePattern.MatchString(rule.body) {
			continue
		}
		for _, selector := range rule.selectors {
			if underlinesInHero(selector, inHero) {
				t.Errorf("%s: `%s` underlines inside the homepage hero; the owner ruled that hero links "+
					"are never underlined and stand out by color", rule.file, selector)
			}
		}
	}
}

// TestHeroSaysZeIsSelfContained pins the owner ruling that the front page
// says Ze is self-contained, as a key point beside the other three.
//
// Method: it reads the hero's key-point row out of the homepage template and
// requires "Self-contained" as its first point, so dropping, renaming or
// moving the point turns it red.
func TestHeroSaysZeIsSelfContained(t *testing.T) {
	const opening = `<p class="hero-start-points" aria-label="Key capabilities">`
	start := strings.Index(homeTemplate, opening)
	if start < 0 {
		t.Fatal("the homepage template holds no hero key-point row")
	}
	end := strings.Index(homeTemplate[start:], "</p>")
	if end < 0 {
		t.Fatal("the hero key-point row is not closed")
	}
	points := regexp.MustCompile(`<span>([^<]*)</span>`).FindAllStringSubmatch(homeTemplate[start:start+end], -1)
	if len(points) == 0 || points[0][1] != "Self-contained" {
		t.Errorf("the hero's first key point is not %q (owner ruling, website/AI.md): %v",
			"Self-contained", points)
	}
}

// TestHomeRFCPanelStatesBothSharesAndWhatEachCounts pins the owner ruling that
// the homepage RFC panel gives two shares, each saying what it counts, and
// links the RFC dependency map beside the evidence link.
//
// Method: it renders the homepage from the fixture snapshot and reads the RFC
// panel alone. The first share must be rfc.proven_percent over every MUST for
// Ze or the OS, naming the not built rows it holds and that they need no test
// until built; the second must be rfc.proven_built_percent over the MUSTs for
// what Ze builds, saying it shows how well the shipped features are tested.
// Dropping either figure, either explanation, the order of the two, or the map
// link turns it red. The arithmetic of the second share is
// TestTheHomeCardStatesProvenOverWhatZeBuilds's.
func TestHomeRFCPanelStatesBothSharesAndWhatEachCounts(t *testing.T) {
	page, mirror := renderHomeFixture(t)
	panel := sliceBetween(t, page, `aria-labelledby="index4-rfc-title"`, `</article>`)

	everyMUST := strings.Index(panel, markedFact("rfc.proven_percent", "54.2"))
	builtMUST := strings.Index(panel, markedFact("rfc.proven_built_percent", "80.5"))
	if everyMUST < 0 {
		t.Fatal("the RFC panel does not show the share of every MUST (rfc.proven_percent)")
	}
	if builtMUST < 0 {
		t.Fatal("the RFC panel does not show the share of the MUSTs for what Ze builds (rfc.proven_built_percent)")
	}
	if builtMUST < everyMUST {
		t.Error("the RFC panel shows the share of what Ze builds before the share of every MUST")
	}

	text := visibleText(panel)
	for _, says := range []string{
		"54.2% 1,609 of all 2,966 MUSTs for Ze or the OS, including the 966 for parts of an RFC Ze does not " +
			"build today, which need no test until they are built.",
		"80.5% 1,609 of the 2,000 MUSTs for what Ze builds. This shows how well the features Ze ships are tested.",
		"The RFC dependency map shows each standard and its coverage visually.",
	} {
		if !strings.Contains(text, says) {
			t.Errorf("the RFC panel does not say %q (owner ruling, website/AI.md); it reads %q", says, text)
		}
	}
	if !strings.Contains(panel, `<a href="quality/rfc-map/">RFC dependency map</a>`) {
		t.Error("the RFC panel does not link the RFC dependency map at quality/rfc-map/")
	}
	if !strings.Contains(panel, `href="quality/rfc-compliance/"`) {
		t.Error("the RFC panel lost its link to the standards requirements and test results")
	}
	for _, says := range []string{"for parts of an RFC Ze does not build today", "MUSTs for what Ze builds",
		"RFC dependency map"} {
		if !strings.Contains(mirror, says) {
			t.Errorf("the homepage mirror does not say %q", says)
		}
	}
}
