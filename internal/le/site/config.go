// Design: website/AI.md -- the configuration reference is the live YANG tree, browsed
// Detail: build.go publishes the tree and the plugin registry this producer reads;
// configscript.go walks the tree in the browser; plugins.go links each config root here.
package site

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// The configuration reference registers from here.
func init() {
	registerProducer(Producer{Name: "configuration", Render: renderConfiguration})
}

// The reference's own addresses in the artifact. plugins.go names
// configurationRoute too: every config root a plugin declares links here.
const (
	configurationRoute = "reference/configuration/"
	configurationDest  = configurationRoute + pageIndexFile
	configurationURL   = "/" + configurationRoute
	configurationRoot  = "../../"
	configurationGuide = "features/bgp-configuration/"
)

// repositoryBlob is where a YANG file's own source is read.
const repositoryBlob = repositoryURL + "/blob/main"

// configOwner names one plugin that provides a config path, and the YANG files
// it provides it from.
type configOwner struct {
	Name string          `json:"name"`
	YANG []configYANGRef `json:"yang"`
}

// configYANGRef is one YANG file, as the browser shows it: the name a reader
// recognizes and the address of the file itself.
type configYANGRef struct {
	File string `json:"file"`
	Href string `json:"href"`
}

// configOwnership is what the browser annotates one config path with.
type configOwnership struct {
	Label   string        `json:"label"`
	Plugins []configOwner `json:"plugins"`
}

// renderConfiguration publishes the configuration reference and answers the one
// route it wrote.
func renderConfiguration(paths Paths) ([]string, error) {
	runtime, err := runtimePlugins(paths.Output)
	if err != nil {
		return nil, err
	}
	raw, tree, err := readConfigTree(paths.Output)
	if err != nil {
		return nil, err
	}

	owners := configOwnerMap(runtime)
	if err := refuseOrphanConfigRoots(tree, owners); err != nil {
		return nil, err
	}

	links, err := loadPageLinks(paths.Source)
	if err != nil {
		return nil, err
	}
	sections := sortedTreeSections(tree)
	owned := 0
	for _, name := range sections {
		if len(owners[name]) != 0 {
			owned++
		}
	}

	body, err := configurationBody(raw, sections, owners, owned)
	if err != nil {
		return nil, err
	}
	shell := pageShell{
		Title: "Configuration Reference - Ze",
		Description: "Browse the whole Ze configuration -- every level shown the same way, " +
			"generated live from the YANG schema.",
		Root:    configurationRoot,
		Path:    configurationDest,
		Sidebar: pageSidebar(configurationRoot, configurationRoute, links),
	}
	mirror := configurationMirror(tree, sections, owners, owned)
	if err := writePublishedPage(paths.Output, configurationDest, shell.render(body), mirror); err != nil {
		return nil, err
	}
	return []string{configurationURL}, nil
}

// runtimePlugins answers the published registrations a reader can deploy.
//
// A test fixture is left out: this page is the configuration an operator writes,
// and a fixture's config root belongs to the test suite rather than to a
// shipped daemon.
func runtimePlugins(output string) ([]registryPlugin, error) {
	var published []registryPlugin
	if err := readArtifactJSON(output, pluginFile, &published); err != nil {
		return nil, err
	}
	runtime := make([]registryPlugin, 0, len(published))
	for _, plugin := range published {
		if strings.HasPrefix(plugin.SourceDir, testPluginPrefix) {
			continue
		}
		runtime = append(runtime, plugin)
	}
	return runtime, nil
}

// readConfigTree answers the published configuration tree twice: the bytes the
// page embeds for the browser, and the decoded tree the mirror walks.
//
// The bytes are the published file's own, compacted rather than re-encoded, so
// the page states the schema in the order the file states it and a second build
// over an unchanged tree writes the same page.
func readConfigTree(output string) ([]byte, map[string]configNode, error) {
	path := filepath.Join(output, filepath.FromSlash(configTreeFile))
	content, err := os.ReadFile(path) //nolint:gosec // a site build reads the artifact it was pointed at
	if err != nil {
		return nil, nil, fmt.Errorf("read the published %s: %w", configTreeFile, err)
	}
	var tree map[string]configNode
	if err := json.Unmarshal(content, &tree); err != nil {
		return nil, nil, fmt.Errorf("read the published %s: %w", configTreeFile, err)
	}
	if len(tree) == 0 {
		return nil, nil, fmt.Errorf("the published %s names no configuration section", configTreeFile)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, content); err != nil {
		return nil, nil, fmt.Errorf("read the published %s: %w", configTreeFile, err)
	}
	return compact.Bytes(), tree, nil
}

// sortedTreeSections answers the top-level configuration sections in name
// order, which is the order the page and its mirror both walk.
func sortedTreeSections(tree map[string]configNode) []string {
	names := make([]string, 0, len(tree))
	for name := range tree {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// configOwnerMap answers, for each config path a plugin declares, the plugins
// that declare it. The path is the raw config root: a top-level section such as
// `bgp`, or a nested path such as `fib/kernel`.
func configOwnerMap(plugins []registryPlugin) map[string][]registryPlugin {
	owners := make(map[string][]registryPlugin)
	for _, plugin := range plugins {
		for _, root := range plugin.ConfigRoots {
			owners[root] = append(owners[root], plugin)
		}
	}
	return owners
}

// refuseOrphanConfigRoots stops the build when a plugin declares a config root
// the schema has no node for.
//
// The plugin's section would then be published as core, with its owner and its
// YANG source silently absent, and the usual cause is a plugin whose declared
// root and the container it augments have drifted apart. The retired renderer
// warned, and the build it ran under exited non-zero on any warning, so such a
// tree never reached a reader either.
func refuseOrphanConfigRoots(tree map[string]configNode, owners map[string][]registryPlugin) error {
	var orphans []string
	for root, plugins := range owners {
		if configPathExists(tree, root) {
			continue
		}
		names := make([]string, 0, len(plugins))
		for _, plugin := range plugins {
			names = append(names, plugin.Name)
		}
		orphans = append(orphans, root+" (declared by "+strings.Join(names, ", ")+")")
	}
	if len(orphans) == 0 {
		return nil
	}
	sort.Strings(orphans)
	return fmt.Errorf("config %s resolves to no node in the YANG configuration tree: %s",
		plural(len(orphans), "root"), strings.Join(orphans, "; "))
}

// configPathExists reports whether a config root names a real node.
func configPathExists(tree map[string]configNode, path string) bool {
	segments := strings.Split(path, "/")
	node, found := tree[segments[0]]
	if !found {
		return false
	}
	for _, segment := range segments[1:] {
		child, found := configChild(&node, segment)
		if !found {
			return false
		}
		node = child
	}
	return true
}

// configChild answers one named child of a node.
func configChild(node *configNode, name string) (configNode, bool) {
	for _, child := range node.Children {
		if child.Name == name {
			return child, true
		}
	}
	return configNode{}, false
}

// configLead is the sentence the page and its mirror both open with, differing
// only in how each spells a link and an emphasis.
func configLead(sections, owned int, guide string) string {
	return "The complete Ze configuration in one place: <strong>" + strconv.Itoa(sections) +
		" sections</strong> (" + strconv.Itoa(owned) + " provided by plugins, the rest core), " +
		"generated live from the YANG schema with <code>ze yang tree</code>. Every level -- " +
		"sections and the containers inside them -- is browsed the same way: pick a setting to " +
		"step into it, or search across the whole configuration. Where a setting is provided by " +
		`a plugin, its owner and YANG source are shown. See the <a href="` + guide +
		`">Configuration guide</a> for a narrative walkthrough of BGP peer config specifically.`
}

// configurationBody renders the reference under <main>: the hero, the browser's
// own controls, the two JSON payloads it reads, and the script that walks them.
func configurationBody(tree []byte, sections []string,
	owners map[string][]registryPlugin, owned int,
) (string, error) {
	ownership, err := json.Marshal(configOwnershipData(owners))
	if err != nil {
		return "", fmt.Errorf("publish the configuration ownership: %w", err)
	}

	var body strings.Builder
	body.WriteString(`            <section aria-labelledby="config-ref-title" class="md-content reveal cat-operate">` + "\n")
	body.WriteString(pageHero("Configuration Reference",
		configLead(len(sections), owned, configurationRoot+configurationGuide),
		"Reference", ` id="config-ref-title"`, "journey-hero reveal") + "\n")
	body.WriteString(`                <div class="config-explorer" data-config-explorer>` + "\n")
	body.WriteString(`<script>document.documentElement.classList.add("config-js")</script>` + "\n")
	body.WriteString(`                <input id="config-search" type="search" ` +
		`placeholder="Search the whole configuration (setting, type, plugin)..." ` +
		`aria-label="Search the configuration" />` + "\n")
	body.WriteString(`                <nav class="config-crumbs" aria-label="Breadcrumb"></nav>` + "\n")
	body.WriteString(`                <div class="config-level"></div>` + "\n")
	body.WriteString(`                <noscript><p class="config-noscript">This config browser ` +
		`needs JavaScript. The whole configuration is also available as ` +
		`<a href="` + pageMirrorFile + `">plain text</a>.</p></noscript>` + "\n")
	body.WriteString("                </div>\n")
	body.WriteString(`                <script id="config-tree" type="application/json">` +
		escapeEmbeddedJSON(tree) + "</script>\n")
	body.WriteString(`                <script id="config-owners" type="application/json">` +
		escapeEmbeddedJSON(ownership) + "</script>\n")
	body.WriteString("            </section>\n")
	body.WriteString(configBrowserScript)
	return body.String(), nil
}

// escapeEmbeddedJSON makes one JSON payload safe inside its own script element.
//
// A "</script>" sequence inside a string would close the element early and the
// rest of the payload would be read as markup, so the opening angle bracket is
// written as its escape. Go's encoder already escapes it in what IT produces;
// the configuration tree is the published file's own bytes and is not.
func escapeEmbeddedJSON(payload []byte) string {
	return strings.ReplaceAll(string(payload), "<", `\u003c`)
}

// configOwnershipData is what the browser annotates each config path with: the
// short label a row shows, and every owning plugin with its YANG sources.
func configOwnershipData(owners map[string][]registryPlugin) map[string]configOwnership {
	data := make(map[string]configOwnership, len(owners))
	for path, plugins := range owners {
		entry := configOwnership{Label: configOwnerLabel(plugins)}
		for _, plugin := range plugins {
			owner := configOwner{Name: plugin.Name, YANG: []configYANGRef{}}
			for _, file := range plugin.YangFiles {
				owner.YANG = append(owner.YANG, configYANGRef{
					File: filepath.Base(file),
					Href: repositoryBlob + "/" + file,
				})
			}
			entry.Plugins = append(entry.Plugins, owner)
		}
		data[path] = entry
	}
	return data
}

// configOwnerLabel is the short answer to "who provides this": the plugin's own
// name when one does, a count when several do, and "core" when none does.
func configOwnerLabel(plugins []registryPlugin) string {
	switch len(plugins) {
	case 0:
		return "core"
	case 1:
		return plugins[0].Name
	default:
		return plural(len(plugins), "plugin")
	}
}

// configurationMirror renders the Markdown sibling: the whole configuration as
// one nested list, section by section, for a reader with no JavaScript.
func configurationMirror(tree map[string]configNode, sections []string,
	owners map[string][]registryPlugin, owned int,
) string {
	var mirror strings.Builder
	mirror.WriteString("# Configuration Reference\n\n")
	mirror.WriteString("The complete Ze configuration as one tree: " + strconv.Itoa(len(sections)) +
		" top-level sections (" + strconv.Itoa(owned) + " provided by plugins, the rest core), " +
		"generated live from the YANG schema with `ze yang tree`. This is about the structure of " +
		"the configuration -- every section, searchable and inspectable. See " +
		"[the Configuration guide](" + siteBase + configurationGuide + ") for a narrative " +
		"walkthrough of BGP peer config specifically.\n\n")
	for _, name := range sections {
		node := tree[name]
		mirror.WriteString("## " + name + "\n\n")
		if line := configOwnerMirrorLine(owners[name]); line != "" {
			mirror.WriteString(line + "\n\n")
		}
		if node.Description != "" {
			mirror.WriteString(collapseWhitespace(node.Description) + "\n\n")
		}
		for _, child := range node.Children {
			writeConfigChildMirror(&mirror, &child, name+"/"+child.Name, owners, 0)
		}
		mirror.WriteString("\n")
	}
	return strings.TrimRight(mirror.String(), "\n") + "\n"
}

// writeConfigChildMirror writes one node and everything under it.
//
// The recursion is over the YANG schema this repository compiles, which is a
// finite tree fixed at build time and nothing an external peer can deepen. Its
// depth is the schema's own nesting, eleven levels at the deepest measured
// today, so the stack is bounded by the committed schema.
func writeConfigChildMirror(mirror *strings.Builder, node *configNode, path string,
	owners map[string][]registryPlugin, depth int,
) {
	indent := strings.Repeat("  ", depth)
	mirror.WriteString(indent + "- **" + configNodeHead(node) + "**")
	if badge := configNodeBadge(node); badge != "" {
		mirror.WriteString(" `" + badge + "`")
	}
	mirror.WriteString("\n")
	if line := configOwnerMirrorLine(owners[path]); line != "" {
		mirror.WriteString(indent + "  " + line + "\n")
	}
	if node.Description != "" {
		mirror.WriteString(indent + "  " + collapseWhitespace(node.Description) + "\n")
	}
	for _, child := range node.Children {
		writeConfigChildMirror(mirror, &child, path+"/"+child.Name, owners, depth+1)
	}
}

// configOwnerMirrorLine states who provides one node, with each owner's YANG
// files linked to their own source. A core node states nothing.
func configOwnerMirrorLine(plugins []registryPlugin) string {
	if len(plugins) == 0 {
		return ""
	}
	stated := make([]string, 0, len(plugins))
	for _, plugin := range plugins {
		links := make([]string, 0, len(plugin.YangFiles))
		for _, file := range plugin.YangFiles {
			links = append(links, "["+filepath.Base(file)+"]("+repositoryBlob+"/"+file+")")
		}
		one := "`" + plugin.Name + "`"
		if len(links) != 0 {
			one += " (" + strings.Join(links, ", ") + ")"
		}
		stated = append(stated, one)
	}
	return "*Provided by " + strings.Join(stated, "; ") + "*"
}

// configNodeHead is a node's own line, in the shape an operator would type: a
// keyed list reads as "peer <name>", and everything else as its own name.
func configNodeHead(node *configNode) string {
	if key, keyed := strings.CutPrefix(node.Kind, "list["); keyed {
		if key, closed := strings.CutSuffix(key, "]"); closed {
			return node.Name + " <" + key + ">"
		}
	}
	return node.Name
}

// configNodeBadge is the short type a node carries beside its name.
func configNodeBadge(node *configNode) string {
	if node.Type != "" {
		switch node.Kind {
		case "leaf":
			return node.Type
		case "leaf-list":
			return node.Type + "[]"
		}
	}
	if strings.HasPrefix(node.Kind, "list[") {
		return "list"
	}
	return node.Kind
}

// collapseWhitespace folds every run of whitespace into one space, so a
// description written over several lines reads as one.
func collapseWhitespace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
