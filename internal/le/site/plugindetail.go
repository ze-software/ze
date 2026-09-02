// Design: website/AI.md -- one page for each registered plugin
// Detail: plugins.go groups and counts them; this file renders one.
package site

import (
	"html"
	"net/url"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The two kinds a detail page names, which are the two halves the catalog
// counts apart.
const (
	pluginKindRuntime = "Runtime plugin"
	pluginKindFixture = "Test fixture"
)

// pluginMetadataSource is where a detail page's facts came from. The retired
// renderer could also read a PLUGIN.md beside the package; no such file has
// ever existed, so the registration is the only source and it is stated.
const pluginMetadataSource = "Registration"

// writePluginDetailPage publishes one plugin's page and its mirror, and answers
// the route it wrote.
func writePluginDetailPage(output string, entry *pluginEntry, group *pluginGroup,
	byName map[string]*pluginEntry, dependents map[string]*pluginRelations, links pageLinks,
) (string, error) {
	route := pluginsRoute + entry.Slug + "/"
	destination := pluginsDirectory + "/" + entry.Slug + "/" + pageIndexFile
	title := entry.Name + " plugin - Ze"
	description := entry.Name + " plugin: " + entry.Description
	shell := pageShell{
		Title:       title,
		Description: description,
		Root:        pluginDetailRoot,
		Path:        destination,
		Sidebar:     pageSidebar(pluginDetailRoot, route[1:], links),
	}
	body := pluginDetailBody(entry, group, byName, dependents[entry.Name])
	mirror := pluginDetailMirror(entry, group, byName, dependents[entry.Name])
	if err := writePublishedPage(output, destination, shell.render(body), mirror); err != nil {
		return "", err
	}
	return route, nil
}

// pluginKind answers whether this plugin ships to an operator or exists for the
// test suite.
func pluginKind(entry *pluginEntry) string {
	if entry.isTest() {
		return pluginKindFixture
	}
	return pluginKindRuntime
}

// pluginDetailBody renders one plugin's page under <main>: the hero, and five
// panels stating what it is, what it configures, what it needs, what needs it,
// and where its code and schema live.
func pluginDetailBody(entry *pluginEntry, group *pluginGroup,
	byName map[string]*pluginEntry, relations *pluginRelations,
) string {
	var body textbuf.Buffer
	body.Reset().Str(`            <section class="md-content reveal cat-automate plugin-detail" aria-labelledby="plugin-detail-title">`).Byte('\n')
	body.Str(pageHero("<code>"+html.EscapeString(entry.Name)+"</code>",
		html.EscapeString(entry.Description), group.Label,
		` id="plugin-detail-title"`, "journey-hero reveal cat-automate")).Byte('\n')
	body.Str(`                <div class="plugin-detail-grid">`).Byte('\n')

	body.Str(pluginPanel("At a glance", `                        <dl class="plugin-detail-facts">`+"\n"+
		pluginFactRow("Registry area", html.EscapeString(group.Label))+
		pluginFactRow("Kind", pluginKind(entry))+
		pluginFactRow("Source path", "<code>"+html.EscapeString(entry.SourceDir)+"</code>")+
		pluginFactRow("YANG modules", strconv.Itoa(len(entry.YangFiles)))+
		pluginFactRow("Metadata source", pluginMetadataSource)+
		"                        </dl>", ""))

	configuration := make([]string, 0, len(entry.ConfigRoots))
	for _, root := range entry.ConfigRoots {
		configuration = append(configuration, `<a href="`+html.EscapeString(pluginConfigHref(root))+
			`"><code>`+html.EscapeString(root)+"</code></a>")
	}
	body.Str(pluginPanel("Configuration", "                        "+pluginLinkList(configuration), ""))

	body.Str(pluginPanel("Dependencies",
		"                        <h3>Required</h3>\n"+
			"                        "+pluginLinkList(pluginDependencyLinks(entry.Dependencies, byName))+"\n"+
			"                        <h3>Optional</h3>\n"+
			"                        "+pluginLinkList(pluginDependencyLinks(entry.OptionalDependencies, byName)), ""))

	body.Str(pluginPanel("Used by",
		"                        <h3>Required dependency for</h3>\n"+
			"                        "+pluginLinkList(pluginEntryLinks(relations.Required))+"\n"+
			"                        <h3>Optional dependency for</h3>\n"+
			"                        "+pluginLinkList(pluginEntryLinks(relations.Optional)), ""))

	yang := make([]string, 0, len(entry.YangFiles))
	for _, path := range entry.YangFiles {
		yang = append(yang, "<code>"+html.EscapeString(path)+"</code>")
	}
	body.Str(pluginPanel("Repository artifacts",
		"                        <p>These paths come from the registry extraction and are shown locally so the detail page stays on the site.</p>\n"+
			`                        <dl class="plugin-detail-facts">`+"\n"+
			"                            <div><dt>Package</dt><dd><code>"+html.EscapeString(entry.SourceDir)+"</code></dd></div>\n"+
			"                        </dl>\n"+
			"                        <h3>YANG files</h3>\n"+
			"                        "+pluginLinkList(yang), " plugin-detail-panel-wide"))

	body.Str("                </div>\n")
	body.Str("            </section>\n")
	return body.String()
}

// pluginPanel wraps one heading and its content as a panel of the detail grid.
func pluginPanel(heading, content, extraClass string) string {
	return `                    <article class="plugin-detail-panel` + extraClass + `">` + "\n" +
		"                        <h2>" + html.EscapeString(heading) + "</h2>\n" +
		content + "\n" +
		"                    </article>\n"
}

// pluginFactRow is one term and its value in a detail page's fact list.
func pluginFactRow(term, value string) string {
	return "                            <div><dt>" + html.EscapeString(term) + "</dt><dd>" + value + "</dd></div>\n"
}

// pluginLinkList renders a list of already-escaped items, and says so plainly
// when there are none. An empty <ul> would read as a rendering fault.
func pluginLinkList(items []string) string {
	if len(items) == 0 {
		return `<p class="plugin-detail-empty">None declared.</p>`
	}
	var list textbuf.Buffer
	list.Reset().Str("<ul>")
	for _, item := range items {
		list.Str("<li>").Str(item).Str("</li>")
	}
	list.Str("</ul>")
	return list.String()
}

// pluginConfigHref points at the config root's own place in the configuration
// reference.
func pluginConfigHref(root string) string {
	return pluginDetailRoot + configurationRoute + "#" + escapeFragmentPath(root)
}

// escapeFragmentPath percent-encodes a config path for a URL fragment, leaving
// the separators between its segments readable.
func escapeFragmentPath(path string) string {
	segments := strings.Split(path, "/")
	for index, segment := range segments {
		segments[index] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}

// pluginDependencyLinks renders each named dependency, linking the ones this
// catalog has a page for and naming the rest.
//
// A dependency with no page is a name the engine resolves at run time that no
// registration declares, so it is stated without a link rather than dropped.
func pluginDependencyLinks(names []string, byName map[string]*pluginEntry) []string {
	links := make([]string, 0, len(names))
	for _, name := range names {
		entry, found := byName[name]
		if !found {
			links = append(links, "<code>"+html.EscapeString(name)+"</code>")
			continue
		}
		links = append(links, `<a href="`+html.EscapeString(pluginSiblingHref(entry))+
			`"><code>`+html.EscapeString(name)+"</code></a>")
	}
	return links
}

// pluginEntryLinks renders a link to each plugin in the list.
func pluginEntryLinks(entries []*pluginEntry) []string {
	links := make([]string, 0, len(entries))
	for _, entry := range entries {
		links = append(links, `<a href="`+html.EscapeString(pluginSiblingHref(entry))+
			`"><code>`+html.EscapeString(entry.Name)+"</code></a>")
	}
	return links
}

// pluginSiblingHref reaches another plugin's page from one detail page.
func pluginSiblingHref(entry *pluginEntry) string {
	return "../" + entry.Slug + "/"
}

// pluginSiblingMirrorHref reaches another plugin's mirror from one mirror.
func pluginSiblingMirrorHref(entry *pluginEntry) string {
	return "../" + entry.Slug + "/" + pageMirrorFile
}

// pluginDetailMirror renders one plugin's Markdown sibling, which states the
// same facts as the page in the order the page states them.
func pluginDetailMirror(entry *pluginEntry, group *pluginGroup,
	byName map[string]*pluginEntry, relations *pluginRelations,
) string {
	var mirror textbuf.Buffer
	mirror.Reset().Str("# `").Str(entry.Name).Str("` plugin\n\n")
	mirror.Str(entry.Description).Str("\n\n")
	mirror.Str("## At a glance\n\n")
	mirror.Str("| Field | Value |\n|-------|-------|\n")
	mirror.Str("| Registry area | ").Str(group.Label).Str(" |\n")
	mirror.Str("| Kind | ").Str(pluginKind(entry)).Str(" |\n")
	mirror.Str("| Source path | `").Str(entry.SourceDir).Str("` |\n")
	mirror.Str("| YANG modules | ").Int(int64(len(entry.YangFiles))).Str(" |\n\n")
	mirror.Str("## Configuration\n\n").Str(pluginOrNone(codeMarkerList(entry.ConfigRoots))).Str("\n\n")
	mirror.Str("## Dependencies\n\n")
	mirror.Str("- Required: ").Str(pluginDependencyMirrorList(entry.Dependencies, byName)).Byte('\n')
	mirror.Str("- Optional: ").Str(pluginDependencyMirrorList(entry.OptionalDependencies, byName)).Str("\n\n")
	mirror.Str("## Used by\n\n")
	mirror.Str("- Required dependency for: ").Str(pluginEntryMirrorList(relations.Required)).Byte('\n')
	mirror.Str("- Optional dependency for: ").Str(pluginEntryMirrorList(relations.Optional)).Str("\n\n")
	mirror.Str("## Repository artifacts\n\n")
	mirror.Str("Package: `").Str(entry.SourceDir).Str("`\n\n")
	mirror.Str("YANG files: ").Str(pluginOrNone(codeMarkerList(entry.YangFiles))).Byte('\n')
	mirror.Str("Metadata source: `").Str(pluginMetadataSource).Str("`\n")
	return mirror.String()
}

// pluginDependencyMirrorList names each dependency in the mirror, linking the
// ones this catalog has a page for.
func pluginDependencyMirrorList(names []string, byName map[string]*pluginEntry) string {
	if len(names) == 0 {
		return nothingDeclared
	}
	parts := make([]string, 0, len(names))
	for _, name := range names {
		entry, found := byName[name]
		if !found {
			parts = append(parts, "`"+name+"`")
			continue
		}
		parts = append(parts, "[`"+name+"`]("+pluginSiblingMirrorHref(entry)+")")
	}
	return strings.Join(parts, ", ")
}

// pluginEntryMirrorList links each plugin in the list from one mirror.
func pluginEntryMirrorList(entries []*pluginEntry) string {
	if len(entries) == 0 {
		return nothingDeclared
	}
	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		parts = append(parts, "[`"+entry.Name+"`]("+pluginSiblingMirrorHref(entry)+")")
	}
	return strings.Join(parts, ", ")
}
