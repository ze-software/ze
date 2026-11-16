// Design: website/AI.md -- the approved blue and green design reaches every page.
package site

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestApprovedStyleIsTheSharedStylesheet pins the owner's approved design at
// the asset every regular page loads, rather than at the former review page.
func TestApprovedStyleIsTheSharedStylesheet(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	css, err := siteStylesheet(filepath.Join(root, "website"))
	if err != nil {
		t.Fatalf("render the shared stylesheet: %v", err)
	}
	stylesheet := string(css)
	for _, want := range []string{
		"--blue:#0963ae", "--blue:#71bcff",
		"--green:#137a50", "--green:#60d6a2",
		"--bg:#070d17", "--panel:#111d2d",
		"--ambient-blue:", "--ambient-green:",
		"radial-gradient(", "var(--ambient-blue)", "var(--ambient-green)",
	} {
		if !strings.Contains(stylesheet, want) {
			t.Errorf("the shared stylesheet omits approved design role %q", want)
		}
	}
	for _, pattern := range []string{
		`\.site-header\{[^}]*border-radius:999px`,
		`:root\[data-theme="dark"\]`,
		`@media\s*\(max-width:960px\)`,
	} {
		if !regexp.MustCompile(pattern).MatchString(stylesheet) {
			t.Errorf("the shared stylesheet omits approved design rule %q", pattern)
		}
	}
	for _, reviewOnly := range []string{".lust-review", "lust-review.css", "tools/lust-review"} {
		if strings.Contains(stylesheet, reviewOnly) {
			t.Errorf("the production style depends on review-only markup or assets: %q", reviewOnly)
		}
	}
	page := testShell().render("<p>Article content</p>")
	if !strings.Contains(page, `href="../../assets/site.css"`) {
		t.Error("an interior page does not load the shared stylesheet")
	}
}

// TestApprovedStyleStartsDarkAndKeepsTheSavedChoice checks both theme entry
// points: first paint from the page shell and the deferred toggle script.
// A stored light choice must remain valid; dark is only the absent-choice
// default, including when browser storage is unavailable.
func TestApprovedStyleStartsDarkAndKeepsTheSavedChoice(t *testing.T) {
	page := testShell().render("<p>Article content</p>")
	for _, want := range []string{
		`var t="dark"`,
		`localStorage.getItem("ze-theme")`,
		`s==="dark"||s==="light"?s:t`,
		`document.documentElement.setAttribute("data-theme",t)`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page's first paint omits theme contract %q", want)
		}
	}
	if strings.Contains(themeBootstrap, "prefers-color-scheme") {
		t.Error("the first paint still replaces the approved default with the system theme")
	}
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "website", "assets", "js", "site.js"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(content)
	for _, want := range []string{
		`return storedTheme() || "dark";`,
		`return value === "dark" || value === "light" ? value : null;`,
		`applyTheme(current === "dark" ? "light" : "dark", true);`,
		`window.localStorage.setItem(THEME_STORAGE_KEY, theme);`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("the deferred script omits theme contract %q", want)
		}
	}
}

// TestApprovedStylePrimaryActionsKeepReadableSmallText prevents the approved
// blue hover treatment from making a small white action label unreadable.
// It calculates contrast from the actual shared and hero declarations, in
// their normal and hover states, including each theme's foreground token.
func TestApprovedStylePrimaryActionsKeepReadableSmallText(t *testing.T) {
	rules := websiteStyleRules(t)
	for _, selector := range []string{
		".button.primary", ".button.primary:hover",
		".site-home .hero-actions .primary", ".site-home .hero-actions .primary:hover",
	} {
		t.Run(selector, func(t *testing.T) {
			var foreground, background string
			for _, rule := range rules {
				if !slices.Contains(rule.selectors, selector) {
					continue
				}
				if value := approvedStyleDeclaration(rule.body, "color"); value != "" {
					foreground = value
				}
				if value := approvedStyleDeclaration(rule.body, "background"); value != "" {
					background = value
				}
			}
			if foreground == "" || background == "" {
				t.Fatalf("%s must declare its foreground and solid background", selector)
			}
			for _, ink := range approvedStyleColorOptions(t, rules, foreground) {
				for _, fill := range approvedStyleColorOptions(t, rules, background) {
					first, second := approvedStyleLuminance(t, ink), approvedStyleLuminance(t, fill)
					ratio := (math.Max(first, second) + 0.05) / (math.Min(first, second) + 0.05)
					if ratio < 4.5 {
						t.Errorf("%s on %s gives %.3f:1 contrast; small action text needs at least 4.5:1", ink, fill, ratio)
					}
				}
			}
		})
	}
}

// TestApprovedStyleProductMapHoverRemainsReadableInLightTheme pins the backing
// behind the product-map links. These links are in a div, so a rule limited to
// hero paragraphs and captions cannot keep their white hover text readable.
func TestApprovedStyleProductMapHoverRemainsReadableInLightTheme(t *testing.T) {
	rules := websiteStyleRules(t)
	const link = ".site-home .hero-outcome-links a:hover"
	const lightLink = `:root[data-theme="light"] .site-home .hero-outcome-links a:hover`
	var foreground, background string
	for _, rule := range rules {
		if slices.Contains(rule.selectors, link) || slices.Contains(rule.selectors, lightLink) {
			if value := approvedStyleDeclaration(rule.body, "color"); value != "" {
				foreground = value
			}
		}
		if slices.Contains(rule.selectors, lightLink) {
			if value := approvedStyleDeclaration(rule.body, "background"); value != "" {
				background = value
			}
		}
	}
	if foreground == "" || background == "" {
		t.Fatal("the product-map hover needs its own light-theme backing for the white text")
	}
	for _, ink := range approvedStyleColorOptions(t, rules, foreground) {
		for _, fill := range approvedStyleColorOptions(t, rules, background) {
			first, second := approvedStyleLuminance(t, ink), approvedStyleLuminance(t, fill)
			if math.Abs(first-1) > 1e-9 {
				t.Errorf("the product-map hover text must remain white, got %s", ink)
			}
			ratio := (math.Max(first, second) + 0.05) / (math.Min(first, second) + 0.05)
			if ratio < 4.5 {
				t.Errorf("the light-theme product-map hover uses %s on %s, %.3f:1 contrast; need at least 4.5:1", ink, fill, ratio)
			}
		}
	}
}

// TestApprovedStyleFloatingHeaderHidesItsInset prevents the floating header's
// top inset from leaving a visible strip when the scroll handler hides it.
// The hidden translation must subtract the same inset that positions it.
func TestApprovedStyleFloatingHeaderHidesItsInset(t *testing.T) {
	rules := websiteStyleRules(t)
	var top, transform, headerBody string
	for _, rule := range rules {
		if slices.Contains(rule.selectors, ".site-header") {
			if value := approvedStyleDeclaration(rule.body, "top"); value != "" {
				top = value
				headerBody = rule.body
			}
		}
		if slices.Contains(rule.selectors, ".site-header.hidden") {
			if value := approvedStyleDeclaration(rule.body, "transform"); value != "" {
				transform = value
			}
		}
	}
	if !strings.HasPrefix(top, "var(") || !strings.HasSuffix(top, ")") {
		t.Fatalf("the floating header's top must use its shared inset token, got %q", top)
	}
	name := strings.TrimSuffix(strings.TrimPrefix(top, "var("), ")")
	if approvedStyleDeclaration(headerBody, name) == "" {
		t.Fatalf("the floating header does not declare its inset token %q", name)
	}
	compact := strings.Join(strings.Fields(transform), "")
	if want := "translateY(calc(-100%-" + top + "))"; compact != want {
		t.Errorf("the hidden header must move its full height plus top inset: got %q, want %q", transform, want)
	}
}

func approvedStyleDeclaration(body, name string) string {
	var value string
	for declaration := range strings.SplitSeq(body, ";") {
		key, candidate, ok := strings.Cut(declaration, ":")
		if ok && strings.TrimSpace(key) == name {
			value = strings.TrimSpace(candidate)
		}
	}
	return value
}

// approvedStyleColorOptions expands a theme token to every declared color, so
// a light-only or dark-only foreground change cannot escape the contrast test.
func approvedStyleColorOptions(t *testing.T, rules []cssRule, value string) []string {
	t.Helper()
	if !strings.HasPrefix(value, "var(") {
		return []string{value}
	}
	name := strings.TrimSuffix(strings.TrimPrefix(value, "var("), ")")
	var colors []string
	for _, rule := range rules {
		if color := approvedStyleDeclaration(rule.body, name); color != "" {
			colors = append(colors, color)
		}
	}
	if len(colors) == 0 {
		t.Fatalf("the action uses an undeclared color token %q", name)
	}
	return colors
}

// approvedStyleLuminance applies WCAG's sRGB transfer function to an opaque
// three- or six-digit CSS hex color. Other fills require a rendered-color test.
func approvedStyleLuminance(t *testing.T, color string) float64 {
	t.Helper()
	hex := strings.TrimPrefix(color, "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if !strings.HasPrefix(color, "#") || len(hex) != 6 {
		t.Fatalf("action color %q is not an opaque CSS hex color", color)
	}
	rgb, err := strconv.ParseUint(hex, 16, 24)
	if err != nil {
		t.Fatalf("parse action color %q: %v", color, err)
	}
	linear := func(channel uint64) float64 {
		value := float64(channel) / 255
		if value <= 0.04045 {
			return value / 12.92
		}
		return math.Pow((value+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(rgb>>16) + 0.7152*linear((rgb>>8)&0xff) + 0.0722*linear(rgb&0xff)
}
