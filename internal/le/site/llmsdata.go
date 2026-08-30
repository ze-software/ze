// Design: website/AI.md -- llms.txt reads the same data files the pages read
// Detail: derived.go writes the sections; this file loads and shapes their inputs.
// Related: catalog.go and equivalents.go read two of the same files.
package site

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The site's own addresses, named once because llms.txt states each of them.
const (
	repositoryURL = "https://github.com/ze-software/ze"
	discordInvite = "https://discord.gg/T8s7CjPDne"
)

// The artifact-relative data files llms.txt reads, beside the command catalog
// and the vendor map that catalog.go and equivalents.go already name.
const (
	factsFile      = "data/site-facts.json"
	pluginFile     = "data/plugin-registry.json"
	featuresFile   = "data/features.json"
	configTreeFile = "data/yang-config-tree.json"
	dependencyFile = "data/dependencies.json"
	navFile        = "data/nav.json"
)

// registryPlugin is one runtime plugin registration, as the catalog holds it.
type registryPlugin struct {
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	ConfigRoots          []string `json:"config_roots"`
	Dependencies         []string `json:"dependencies"`
	OptionalDependencies []string `json:"optional_dependencies"`
	SourceDir            string   `json:"source_dir"`
	YangFiles            []string `json:"yang_files"`
}

// configNode is one node of the YANG-derived configuration tree.
type configNode struct {
	Description string `json:"description"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	// Type is the YANG type a leaf or a leaf-list holds, and is empty for a
	// container and for a list. The configuration reference shows it as the
	// node's badge, which is the one place a reader learns what to write.
	Type     string       `json:"type"`
	Children []configNode `json:"children"`
}

// llmsInputs is everything one llms.txt render reads, loaded once.
//
// They are loaded together rather than section by section, so a build that is
// missing one input fails before it writes half a file. A partial llms.txt is
// worse than none: it reads complete and states less than it claims.
type llmsInputs struct {
	Facts        siteFacts
	Plugins      []registryPlugin
	Features     featureData
	ConfigTree   map[string]configNode
	Dependencies dependencyData
	Nav          siteNav
	Commands     []catalogCommand
	Equivalents  *equivalentMapping
}

// loadLLMSInputs reads every input llms.txt states a fact from.
func loadLLMSInputs(paths Paths) (*llmsInputs, error) {
	inputs := &llmsInputs{}
	for _, load := range []struct {
		file  string
		value any
	}{
		{factsFile, &inputs.Facts},
		{pluginFile, &inputs.Plugins},
		{featuresFile, &inputs.Features},
		{configTreeFile, &inputs.ConfigTree},
		{dependencyFile, &inputs.Dependencies},
		{navFile, &inputs.Nav},
	} {
		if err := readArtifactJSON(paths.Output, load.file, load.value); err != nil {
			return nil, err
		}
	}
	commands, err := loadCommandCatalog(paths.Output)
	if err != nil {
		return nil, err
	}
	inputs.Commands = commands
	mapping, err := loadEquivalentMapping(paths.Output)
	if err != nil {
		return nil, err
	}
	inputs.Equivalents = mapping
	return inputs, nil
}

// readArtifactJSON decodes one published data file into value.
func readArtifactJSON(output, name string, value any) error {
	path := filepath.Join(output, filepath.FromSlash(name))
	content, err := os.ReadFile(path) //nolint:gosec // a site build reads the artifact it was pointed at
	if err != nil {
		return fmt.Errorf("read the published %s: %w", name, err)
	}
	if err := json.Unmarshal(content, value); err != nil {
		return fmt.Errorf("read the published %s: %w", name, err)
	}
	return nil
}

// The patterns that turn authored prose into one plain line an inventory can
// hold: markup, an inline link, emphasis, and the punctuation a plain-text
// reader has no font for.
var (
	inlineTag        = regexp.MustCompile(`<[^>]+>`)
	inlineLink       = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	inlineEmphasis   = regexp.MustCompile(`\*\*|__`)
	inlineWhitespace = regexp.MustCompile(`\s+`)
)

// cleanInline folds one authored value into a single plain line.
func cleanInline(value string) string {
	value = inlineTag.ReplaceAllString(value, " ")
	value = inlineLink.ReplaceAllString(value, "$1")
	value = inlineEmphasis.ReplaceAllString(value, "")
	replacer := strings.NewReplacer("—", "-", "–", "-", "…", "...")
	value = replacer.Replace(value)
	return strings.TrimSpace(inlineWhitespace.ReplaceAllString(value, " "))
}

// trimInline folds one authored value into a single plain line and cuts it at
// the last word boundary inside limit, so an inventory line stays scannable.
func trimInline(value string, limit int) string {
	text := cleanInline(value)
	if len(text) <= limit {
		return text
	}
	cut := text[:limit]
	if space := strings.LastIndex(cut, " "); space > 0 {
		cut = cut[:space]
	}
	return strings.TrimRight(cut, ".,;:") + "..."
}

// mirrorURL answers the absolute URL of one page's Markdown mirror, which is
// what a machine reader wants: the same content without the chrome.
func mirrorURL(href string) string {
	if hasURLScheme(href) {
		return href
	}
	href = strings.TrimPrefix(href, "/")
	path, fragment, hasFragment := strings.Cut(href, "#")
	switch {
	case path == "" || path == pageIndexFile:
		path = pageMirrorFile
	case strings.HasSuffix(path, "/"):
		path += pageMirrorFile
	case !strings.HasSuffix(path, ".md") && !strings.HasSuffix(path, ".html"):
		path += "/" + pageMirrorFile
	}
	if hasFragment {
		return siteBase + path + "#" + fragment
	}
	return siteBase + path
}

// pageURL answers the absolute URL a person opens for the same page.
func pageURL(href string) string {
	if hasURLScheme(href) {
		return href
	}
	return siteBase + strings.TrimPrefix(href, "/")
}

// writeLLMSDocumentation lists every hand-authored documentation page, with the
// title and the opening paragraph read from the page's own source.
//
// It reads the SOURCES rather than the rendered pages, because a summary of a
// rendered page is a summary of the chrome around it. The list is the docs
// producer's own manifest and use-case family, so a page that stops being
// published stops being listed here in the same edit.
func writeLLMSDocumentation(out *strings.Builder, paths Paths) error {
	out.WriteString("## Complete documentation index\n\n")
	for _, row := range docsManifest {
		directory, err := docsDestination(row.Source)
		if err != nil {
			return err
		}
		if err := writeDocumentationEntry(out, paths, "docs/"+row.Source, directory+"/"); err != nil {
			return err
		}
	}
	for _, page := range useCasePages {
		href := strings.TrimSuffix(page.Dest, pageIndexFile)
		if err := writeDocumentationEntry(out, paths, page.Source, href); err != nil {
			return err
		}
	}
	out.WriteString("\n")
	return nil
}

// writeDocumentationEntry writes one documentation page as one line.
func writeDocumentationEntry(out *strings.Builder, paths Paths, source, href string) error {
	title, summary, err := markdownTitleAndSummary(filepath.Join(paths.Repository, filepath.FromSlash(source)))
	if err != nil {
		return err
	}
	out.WriteString("- [" + title + "](" + mirrorURL(href) + "): " + summary +
		" (web: " + pageURL(href) + ")\n")
	return nil
}

// markdownHeading matches the first level-one heading of a source.
var markdownHeading = regexp.MustCompile(`(?m)^#\s+(.+)$`)

// markdownComment matches an HTML comment, which carries no prose for a reader.
var markdownComment = regexp.MustCompile(`(?s)<!--.*?-->`)

// markdownListItem matches the marker a list item opens with.
var markdownListItem = regexp.MustCompile(`^(?:[-+*]|\d+[.)])\s+`)

// markdownTitleAndSummary answers one source's heading and its opening prose.
//
// The summary is the first paragraph that is prose: a fence, a heading, a
// quote, a table row and a block of HTML each say nothing about the page. A
// source whose first content is a list falls back to that list's first item,
// which is what a reader would skim first anyway.
func markdownTitleAndSummary(path string) (string, string, error) {
	content, err := os.ReadFile(path) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return "", "", err
	}
	_, body, err := parseFrontMatter(content)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", path, err)
	}
	text := string(body)
	title := strings.TrimSuffix(filepath.Base(path), ".md")
	if match := markdownHeading.FindStringSubmatch(text); match != nil {
		title = cleanInline(match[1])
	}
	text = markdownComment.ReplaceAllString(text, "")
	listFallback := ""
	for block := range strings.SplitSeq(text, "\n\n") {
		lines := make([]string, 0, 8)
		for line := range strings.SplitSeq(block, "\n") {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				lines = append(lines, trimmed)
			}
		}
		if len(lines) == 0 {
			continue
		}
		first := lines[0]
		if strings.HasPrefix(first, "```") || strings.HasPrefix(first, "~~~") ||
			strings.HasPrefix(first, "#") || strings.HasPrefix(first, ">") ||
			strings.HasPrefix(first, "|") || strings.HasPrefix(first, "<") {
			continue
		}
		if markdownListItem.MatchString(first) {
			if listFallback == "" {
				listFallback = trimInline(markdownListItem.ReplaceAllString(first, ""), summaryLimit)
			}
			continue
		}
		return title, trimInline(strings.Join(lines, " "), summaryLimit), nil
	}
	return title, listFallback, nil
}

// summaryLimit bounds one documentation summary, so the index stays a list a
// reader scans rather than a second copy of every page.
const summaryLimit = 220
