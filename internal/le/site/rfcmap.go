// Design: docs/contributing/gh-pages.md -- registered standalone RFC planning page over recorded data
// Related: rfcmap_model.go owns input validation and associations; the page script owns graph interaction.
package site

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	rfcMapProducerName = "rfc-map"
	rfcMapDestination  = "quality/rfc-map/index.html"
	rfcMapRoute        = "/quality/rfc-map/"
	rfcMapRoot         = "../../"
	rfcMapDataFile     = "data/rfc-map.json"
)

func init() {
	registerProducer(Producer{Name: rfcMapProducerName, Render: renderRFCMap})
}

// renderRFCMap publishes only this page and its own assets. It reads recorded
// inputs through the model. This producer calls no RFC check or collector.
func renderRFCMap(paths Paths) ([]string, error) {
	data, err := loadRFCMapData(paths)
	if err != nil {
		return nil, err
	}
	assets := []struct {
		source  string
		target  string
		content []byte
	}{
		{source: "assets/rfc-map.css", target: "assets/rfc-map.css"},
		{source: "assets/js/rfc-map.js", target: "assets/rfc-map.js"},
	}
	// Read every required asset before publishing, so an unfinished source
	// asset cannot replace the page with a graph the browser cannot load.
	for index := range assets {
		asset := &assets[index]
		asset.content, err = os.ReadFile(filepath.Join(paths.Source, asset.source)) //nolint:gosec // This producer reads its two fixed page-owned source assets.
		if err != nil {
			return nil, fmt.Errorf("RFC map needs website/%s before it can render: %w", asset.source, err)
		}
		if len(asset.content) == 0 {
			return nil, fmt.Errorf("RFC map asset website/%s is empty; complete that asset before rendering rfc-map", asset.source)
		}
	}
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode RFC map: %w", err)
	}
	shell := pageShell{
		Title:       "RFC dependency map - Ze",
		Description: "Explore RFC dependencies, proposed release layers and the specifications associated with each requirement.",
		Root:        rfcMapRoot,
		Path:        rfcMapDestination,
		Wide:        true,
		ExtraHead: "        <link rel=\"stylesheet\" href=\"../../assets/rfc-map.css\" />\n" +
			"        <script src=\"../../assets/rfc-map.js\" defer></script>\n",
	}
	page := shell.render(rfcMapBody(&data))
	mirror := rfcMapMirror(&data)
	if err := writeNamedArtifact(paths.Output, rfcMapDataFile, string(encoded)+"\n"); err != nil {
		return nil, err
	}
	for index := range assets {
		asset := &assets[index]
		if err := writeNamedArtifact(paths.Output, asset.target, string(asset.content)); err != nil {
			return nil, err
		}
	}
	if err := writePublishedPage(paths.Output, rfcMapDestination, page, mirror); err != nil {
		return nil, err
	}
	return []string{rfcMapRoute}, nil
}

func rfcMapBody(data *rfcMapData) string {
	var out textbuf.Buffer
	out.Str("<section class=\"hero hero-inner\" aria-labelledby=\"top\">\n")
	out.Str("<div class=\"hero-copy\"><span class=\"journey-eyebrow\">Release planning</span>\n")
	out.Str("<h1 id=\"top\">RFC dependency map.</h1>\n")
	out.Str("<p>Start with base BGP, explore the capabilities around it, and find the specifications associated with each requirement.</p>\n")
	out.Str("<p>Release layers are proposals. A filter changes this view; it does not disable a feature or approve a release.</p>\n")
	out.Str("<nav aria-label=\"Related standards pages\"><a href=\"../rfc-compliance/\">RFC compliance</a> · <a href=\"../rfc-compliance/profiles/\">Completion by use case</a> · <a href=\"../../project/roadmap/\">Roadmap</a></nav></div>\n</section>\n")
	out.Str("<section id=\"rfc-map\" data-rfc-map data-source=\"../../data/rfc-map.json\" data-rfc-base=\"../../data/rfc/\" data-roadmap=\"../../data/release-roadmap.json\" aria-label=\"RFC dependencies and release filters\">\n")
	out.Str("<div class=\"rfc-map-fallback\"><h2>Explore the recorded evidence</h2>\n")
	out.Str("<p>The interactive graph is loading. The recorded RFC list and evidence links below remain available if JavaScript is disabled or the graph cannot load.</p>\n")
	out.Str("<p><a href=\"../rfc-compliance/\">RFC evidence</a> · <a href=\"../rfc-compliance/profiles/\">Existing release profiles</a> · <a href=\"../../project/roadmap/\">Specification inventory</a></p>\n")
	out.Str("<p>RFC evidence revision and age are unknown where the recorded input supplies neither. Specification snapshot: <code>")
	out.Str(html.EscapeString(data.RoadmapRevision)).Str("</code>. These inputs do not certify one release candidate.</p>\n")
	out.Str("<noscript><p>The graph needs JavaScript. Use the recorded list below or the existing evidence pages to continue.</p></noscript>\n")
	out.Str("<details><summary>Recorded RFC list</summary><ul>\n")
	for index := range data.Nodes {
		node := &data.Nodes[index]
		out.Str("<li>")
		if node.Summary {
			out.Str("<a href=\"../rfc-compliance/").Str(html.EscapeString(node.Stem)).Str("/\">")
		}
		out.Str("<code>").Str(html.EscapeString(node.Stem)).Str("</code>")
		if node.Summary {
			out.Str("</a>")
		}
		if node.Title != "" {
			out.Str(" — ").Str(html.EscapeString(node.Title))
		}
		if !node.Summary {
			out.Str(" — no recorded summary")
		}
		out.Str("</li>\n")
	}
	out.Str("</ul></details>\n</div>\n</section>\n")
	return out.String()
}

func rfcMapMirror(data *rfcMapData) string {
	var out textbuf.Buffer
	out.Str("# RFC dependency map\n\n")
	out.Str("Explore RFC dependencies, proposed release layers and the specifications associated with each requirement. ")
	out.Str("Layer filters are planning proposals; they do not disable features or approve a release.\n\n")
	out.Str("The interactive graph is available on the [RFC map page](./).\n\n")
	out.Str("## Recorded inputs\n\n")
	out.Str("RFC evidence revision and age are unknown where the recorded input supplies neither. ")
	out.Str("Specification snapshot: `").Str(data.RoadmapRevision).Str("`. These inputs do not certify one release candidate.\n\n")
	for index := range data.Inputs {
		input := &data.Inputs[index]
		out.Str("- ").Str(rfcMapMirrorText(input.Label)).Str(": `").Str(input.Path).
			Str("`, SHA-256 `").Str(input.SHA256).Str("`")
		if input.Revision != "" {
			out.Str(", revision `").Str(input.Revision).Str("`")
		}
		out.Byte('\n')
	}
	out.Str("\n## Proposed release layers\n\n")
	for index := range data.Layers {
		layer := &data.Layers[index]
		out.Str("- ").Str(rfcMapMirrorText(layer.Title)).Str(": ")
		out.Str(strings.Join(layer.Stems, ", ")).Byte('\n')
	}
	out.Str("\n## Recorded RFCs\n\n")
	for index := range data.Nodes {
		node := &data.Nodes[index]
		out.Str("- ")
		if node.Summary {
			out.Str("[").Str(node.Stem).Str("](../rfc-compliance/").Str(node.Stem).Str("/)")
		} else {
			out.Str(node.Stem).Str(" (no recorded summary)")
		}
		if node.Title != "" {
			out.Str(" — ").Str(rfcMapMirrorText(node.Title))
		}
		out.Byte('\n')
	}
	out.Str("\n[Full RFC evidence](../rfc-compliance/) · [Existing profiles](../rfc-compliance/profiles/) · ")
	out.Str("[Specification inventory](../../project/roadmap/) · [Map data](../../data/rfc-map.json)\n")
	return out.String()
}

func rfcMapMirrorText(value string) string {
	return strings.NewReplacer("\n", " ", "\r", " ", "[", `\[`, "]", `\]`).Replace(value)
}
