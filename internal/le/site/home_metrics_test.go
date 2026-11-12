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
		"go_vendor_lines": 4_567_890, "go_vendor_lines_display": "4.57 million",
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
// actual renderer with five independent values and checks each destination.
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
		"repo.go_source_lines_display": {"2.35 million", "lines of Go source code", "project/activity/"},
		"repo.go_test_lines_display":   {"3.46 million", "lines of Go test code", "project/activity/"},
		"repo.go_vendor_lines_display": {"4.57 million", "lines of third-party Go code", "project/activity/"},
		"plugin_registrations":         {"87", "Plugins", "reference/plugins/"},
		"cli_commands":                 {"612", "CLI commands", "reference/cli/"},
	}
	seen := make(map[string]bool)
	var order []string
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
						if !seen[attr.Val] {
							order = append(order, attr.Val)
						}
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
	if got := strings.Join(order, ","); got != "repo.go_source_lines_display,repo.go_test_lines_display,repo.go_vendor_lines_display,plugin_registrations,cli_commands" {
		t.Errorf("homepage metric order = %s, want source, tests, third-party, plugins, commands", got)
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
	} {
		if !strings.Contains(page, want) {
			t.Errorf("homepage lacks preserved path or metric definition %s", want)
		}
	}
	if strings.Contains(page, `class="hero-primary-actions"`) {
		t.Error("homepage still publishes the four repeated hero actions")
	}
	if strings.Contains(page, `class="hero-metrics-note"`) {
		t.Error("homepage still publishes the removed inventory note")
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
		"2.35 million", "lines of Go source code", "3.46 million", "lines of Go test code", "87", "Plugins", "612", "CLI commands",
		"4.57 million", "lines of third-party Go code",
		"project/activity/", "project/why-ze/", "discord.gg/T8s7CjPDne",
	} {
		if !strings.Contains(mirror, want) {
			t.Errorf("homepage Markdown lacks %q", want)
		}
	}
	if strings.Contains(mirror, "Current first-party Go inventory.") || strings.Contains(mirror, "Includes comments and blank lines; excludes third-party dependencies.") {
		t.Error("homepage Markdown still publishes the removed inventory note")
	}
}

// TestHomeOpeningKeepsLinkedCapabilities requires the complete existing
// capability information in the opening composition beside the demonstration.
func TestHomeOpeningKeepsLinkedCapabilities(t *testing.T) {
	paths := metricHomeFixture(t)
	if _, err := renderHome(paths); err != nil {
		t.Fatal(err)
	}
	root, err := xhtml.Parse(strings.NewReader(readArtifact(t, paths.Output, homeDest)))
	if err != nil {
		t.Fatal(err)
	}
	var opening *xhtml.Node
	var find func(*xhtml.Node)
	find = func(node *xhtml.Node) {
		for _, attr := range node.Attr {
			if attr.Key == "class" && strings.Contains(" "+attr.Val+" ", " hero-start-intro ") {
				opening = node
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			find(child)
		}
	}
	find(root)
	if opening == nil {
		t.Fatal("homepage has no opening composition")
	}
	links := make(map[string]bool)
	var collect func(*xhtml.Node)
	collect = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode && node.Data == "a" {
			for _, attr := range node.Attr {
				if attr.Key == "href" {
					links[attr.Val] = true
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			collect(child)
		}
	}
	collect(opening)
	for _, href := range []string{
		"guides/bgp-peering/", "guides/isis/", "guides/ospf/", "features/interfaces/",
		"guides/ipsec/", "features/interfaces/#wireguard-configuration", "reference/plugins/fib-kernel/",
		"features/bgp-configuration/", "features/web-interface/", "features/cli-commands/",
		"features/mcp-integration/", "features/api-commands/", "guides/api/#rest-endpoints",
		"guides/api/#grpc-services", "guides/gnmi/", "architecture/", "guides/appliance/",
	} {
		if !links[href] {
			t.Errorf("opening lacks useful capability link %s", href)
		}
	}
	for _, text := range []string{"Manage one configuration", "plugins for automation and services of your own"} {
		if !strings.Contains(metricNodeText(opening), text) {
			t.Errorf("opening lacks capability information %q", text)
		}
	}
}

// TestHomeIntroMergesLinkedRouting holds the routing and management summary to
// its useful destinations after the separate routing definition is merged.
func TestHomeIntroMergesLinkedRouting(t *testing.T) {
	paths := metricHomeFixture(t)
	if _, err := renderHome(paths); err != nil {
		t.Fatal(err)
	}
	root, err := xhtml.Parse(strings.NewReader(readArtifact(t, paths.Output, homeDest)))
	if err != nil {
		t.Fatal(err)
	}
	intro := metricNodeWithClass(root, "hero-intro")
	if intro == nil {
		t.Fatal("homepage has no introduction")
	}
	links := make(map[string]bool)
	var collect func(*xhtml.Node)
	collect = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode && node.Data == "a" {
			for _, attr := range node.Attr {
				if attr.Key == "href" {
					links[attr.Val] = true
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			collect(child)
		}
	}
	collect(intro)
	for _, href := range []string{
		"guides/bgp-peering/", "guides/ospf/", "guides/isis/", "features/interfaces/",
		"guides/ipsec/", "features/interfaces/#wireguard-configuration", "reference/plugins/fib-kernel/",
		"reference/configuration/", "features/cli-commands/", "features/web-interface/", "features/api-commands/",
		"reference/plugins/fib-vpp/",
	} {
		if !links[href] {
			t.Errorf("introduction lacks useful link %s", href)
		}
	}
	for _, text := range []string{"Linux", "services", "tunnels", "FIB or VPP forwarding"} {
		if !strings.Contains(metricNodeText(intro), text) {
			t.Errorf("introduction lacks routing or management information %q", text)
		}
	}
	if strings.Contains(metricNodeText(root), "Route and connect") {
		t.Error("homepage retains the routing heading after its content was merged")
	}
}

// TestHomeReleaseNamesInitialVersion checks the release title followed by the
// linked bird and the two lines that name the initial version and its quarter.
func TestHomeReleaseNamesInitialVersion(t *testing.T) {
	paths := metricHomeFixture(t)
	if _, err := renderHome(paths); err != nil {
		t.Fatal(err)
	}
	root, err := xhtml.Parse(strings.NewReader(readArtifact(t, paths.Output, homeDest)))
	if err != nil {
		t.Fatal(err)
	}
	callout := metricNodeWithClass(root, "hero-release-callout")
	if callout == nil {
		t.Fatal("homepage has no release callout")
	}
	title := metricNodeWithClass(callout, "hero-release-title")
	if title == nil || strings.TrimSpace(metricNodeText(title)) != "Planned release" {
		t.Fatal("release callout lacks its Planned release title")
	}
	details := metricNodeWithClass(callout, "hero-release-details")
	if details == nil {
		t.Fatal("release callout lacks the bird and version row")
	}
	var tags []string
	var lines []string
	breaks := 0
	for child := details.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != xhtml.ElementNode {
			continue
		}
		tags = append(tags, child.Data)
		if child.Data == "p" {
			for line := child.FirstChild; line != nil; line = line.NextSibling {
				if line.Type == xhtml.ElementNode && line.Data == "br" {
					breaks++
				}
				if line.Type == xhtml.ElementNode && line.Data == "span" {
					lines = append(lines, strings.TrimSpace(metricNodeText(line)))
				}
			}
		}
	}
	if got := strings.Join(tags, ","); got != "a,p" {
		t.Errorf("release details order = %s, want linked bird before version text", got)
	}
	if metricNodeWithClass(details, "hero-mascot") == nil || metricNodeWithClass(details, "hero-release-speaker") == nil {
		t.Error("release details lost the linked bird")
	}
	if got := strings.Join(lines, ","); got != "Initial version,Q4 2026" || breaks != 1 {
		t.Errorf("release lines = %q with %d breaks, want Initial version then Q4 2026", got, breaks)
	}
}

// metricNodeWithClass finds a test target in the rendered DOM by a full class.
func metricNodeWithClass(node *xhtml.Node, class string) *xhtml.Node {
	for _, attr := range node.Attr {
		if attr.Key == "class" && strings.Contains(" "+attr.Val+" ", " "+class+" ") {
			return node
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := metricNodeWithClass(child, class); found != nil {
			return found
		}
	}
	return nil
}
