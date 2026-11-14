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
