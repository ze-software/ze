// Design: website/AI.md -- the homepage project metrics bind the published snapshot.
package site

import (
	"encoding/json"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"
)

// metricHomeFixture changes the snapshot inputs, never the template under test.
func metricHomeFixture(t *testing.T) Paths {
	t.Helper()
	paths := homeFixture(t)
	var snapshot map[string]any
	if err := json.Unmarshal([]byte(publishedFactsSnapshot), &snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot["repo"] = map[string]any{
		"go_source_lines": 2_345_678, "go_source_lines_display": "2.35 million",
		"go_test_lines": 3_456_789, "go_test_lines_display": "3.46 million",
	}
	snapshot["plugin_registrations"] = 87
	snapshot["cli_commands"] = 612
	content, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	writeArtifactFile(t, paths.Output, factsFile, string(content))
	return paths
}

// TestHomeProjectMetricsUseSnapshotValues exercises the registered homepage's
// actual renderer with four independent values and checks each destination.
func TestHomeProjectMetricsUseSnapshotValues(t *testing.T) {
	paths := metricHomeFixture(t)
	if _, err := renderHome(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, homeDest)
	root, err := xhtml.Parse(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	wants := map[string]struct{ value, label, href string }{
		"repo.go_source_lines_display": {"2.35 million", "Go source lines", "project/activity/"},
		"repo.go_test_lines_display":   {"3.46 million", "Go test lines", "project/activity/"},
		"plugin_registrations":         {"87", "Plugins", "reference/plugins/"},
		"cli_commands":                 {"612", "CLI commands", "reference/cli/"},
	}
	seen := make(map[string]bool)
	var visit func(*xhtml.Node)
	visit = func(node *xhtml.Node) {
		for _, attr := range node.Attr {
			want, relevant := wants[attr.Val]
			if attr.Key != "data-ze-stat" || !relevant {
				continue
			}
			if got := metricNodeText(node); got != want.value {
				t.Errorf("%s = %q, want %q", attr.Val, got, want.value)
			}
			for parent := node.Parent; parent != nil; parent = parent.Parent {
				if parent.Type != xhtml.ElementNode || parent.Data != "a" {
					continue
				}
				for _, link := range parent.Attr {
					if link.Key == "href" && link.Val == want.href && strings.Contains(metricNodeText(parent), want.label) {
						seen[attr.Val] = true
					}
				}
				break
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	for key := range wants {
		if !seen[key] {
			t.Errorf("homepage lacks metric %s with its label and destination", key)
		}
	}
}

// metricNodeText reads rendered text rather than markup or attribute strings.
func metricNodeText(node *xhtml.Node) string {
	if node.Type == xhtml.TextNode {
		return node.Data
	}
	var text strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		text.WriteString(metricNodeText(child))
	}
	return text.String()
}

// TestHomeProjectMetricsPreserveNavigationAndRelease gives each remaining
// reader purpose an explicit path while removing only the repeated hero tiles.
func TestHomeProjectMetricsPreserveNavigationAndRelease(t *testing.T) {
	paths := metricHomeFixture(t)
	if _, err := renderHome(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, homeDest)
	for _, want := range []string{
		`href="project/why-ze/"`, `href="https://discord.gg/T8s7CjPDne"`,
		`No stable release yet.`, `id="proof"`, `data-terminal-demo-player`,
		`Includes comments and blank lines; excludes third-party dependencies.`,
		`Current first-party Go inventory.`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("homepage lacks preserved path or metric definition %s", want)
		}
	}
	if strings.Contains(page, `class="hero-primary-actions"`) {
		t.Error("homepage still publishes the four repeated hero actions")
	}
}

// TestHomeProjectMetricsMirrorIsReadable proves the non-browser answer carries
// the figures together with their definitions and useful next steps.
func TestHomeProjectMetricsMirrorIsReadable(t *testing.T) {
	paths := metricHomeFixture(t)
	if _, err := renderHome(paths); err != nil {
		t.Fatal(err)
	}
	mirror := readArtifact(t, paths.Output, "index.md")
	for _, want := range []string{
		"2.35 million", "Go source lines", "3.46 million", "Go test lines", "87", "Plugins", "612", "CLI commands",
		"Includes comments and blank lines; excludes third-party dependencies.", "project/activity/", "project/why-ze/", "discord.gg/T8s7CjPDne",
		"Current first-party Go inventory.",
	} {
		if !strings.Contains(mirror, want) {
			t.Errorf("homepage Markdown lacks %q", want)
		}
	}
}
