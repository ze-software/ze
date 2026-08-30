// Design: website/AI.md -- the plugin catalog is the live plugin registry, rendered
// Detail: build.go publishes data/plugin-registry.json that this producer reads;
// shell.go wraps each page; llmsdata.go reads the same file for one llms.txt section.
package site

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/inventory"
)

// The plugin catalog registers from here. A build discovers it through the
// registry rather than through a call it states by name.
func init() {
	registerProducer(Producer{Name: pluginProducerName, Render: renderPluginCatalog})
}

// The catalog's own addresses in the artifact.
const (
	pluginsDirectory = "reference/" + pluginProducerName
	pluginsDest      = pluginsDirectory + "/" + pageIndexFile
	pluginsRoute     = "/" + pluginsDirectory + "/"
	// pluginsRoot reaches the site root from the catalog, and pluginDetailRoot
	// from one plugin's detail page a level below it.
	pluginsRoot      = "../../"
	pluginDetailRoot = "../../../"
)

// testPluginPrefix is where a plugin that exists for the test suite lives. The
// catalog groups those apart and counts them apart, because a reader choosing
// what to deploy must not meet a fixture beside a runtime plugin.
const testPluginPrefix = "internal/test/"

// pluginAcronyms keep a group label readable. The grouping itself still comes
// from config roots and source paths; this only spells a label.
var pluginAcronyms = map[string]string{
	"afi": "AFI", "api": "API", "as112": "AS112", "bfd": "BFD", bgpGroup: "BGP",
	"copp": "CoPP", "ddos": "DDoS", "dhcp": "DHCP", "fib": "FIB", "igp": "IGP",
	"ike": "IKE", "ip": "IP", "ipfix": "IPFIX", "irr": "IRR", "isis": "IS-IS",
	"l2tp": "L2TP", "ldp": "LDP", "mrt": "MRT", "nat": "NAT", "nlri": "NLRI",
	"ntp": "NTP", "ospf": "OSPF", "p4": "P4", "pki": "PKI", "qos": "QoS",
	"rib": "RIB", "rpki": "RPKI", "rsvp": "RSVP", "vpn": "VPN", "safi": "SAFI",
	"sr": "SR", "tc": "TC", "te": "TE", "vpp": "VPP", "yang": "YANG",
}

// pluginGroupAliases map a package's code name onto the public noun. The
// implementation package for interfaces is `iface`, because `interface` is a Go
// keyword, and a reader of the catalog wants the feature's own name.
var pluginGroupAliases = map[string]string{"iface": "interface"}

// The coarse buckets the category filter offers, in the order it writes them,
// and the label each one carries.
const (
	bucketRouting   = "routing"
	bucketSecurity  = "security"
	bucketTunneling = "tunneling"
	bucketServices  = "services"
	bucketDataplane = "dataplane"
	bucketTelemetry = "telemetry"
	bucketSystem    = "system"
	bucketTest      = "test"
)

var pluginBucketOrder = []string{
	bucketRouting, bucketSecurity, bucketTunneling, bucketServices,
	bucketDataplane, bucketTelemetry, bucketSystem, bucketTest,
}

var pluginBucketLabels = map[string]string{
	bucketRouting:   "Routing",
	bucketSecurity:  "Policy & security",
	bucketTunneling: "Tunneling",
	bucketServices:  "Network services",
	bucketDataplane: "Interfaces & dataplane",
	bucketTelemetry: "Telemetry & operations",
	bucketSystem:    "System",
	bucketTest:      "Test fixtures",
}

// The group identifiers the grouping rules spell more than once: the area a
// test fixture lands in, the BGP engine's own area, and the token that files a
// package under the filter sub-area.
const (
	testHarnessGroup = "test-harness"
	bgpGroup         = "bgp"
	filterSegment    = "filter"
)

// pluginProducerName is what the coverage answer calls this producer, and the
// directory the catalog publishes under carries the same word.
const pluginProducerName = "plugins"

// marshalPluginRegistry answers the published form of the live registrations.
//
// The published shape is its own, not the inventory's: it is what the retired
// extractor wrote, what llmsdata.go's registryPlugin already reads, and what a
// reader of data/plugin-registry.json gets. An absent list is published as an
// empty array rather than as null, because a reader of JSON should not have to
// tell those apart.
func marshalPluginRegistry(plugins []inventory.Plugin) (string, error) {
	published := make([]registryPlugin, 0, len(plugins))
	for index := range plugins {
		plugin := &plugins[index]
		published = append(published, registryPlugin{
			Name:                 plugin.Name,
			Description:          plugin.Description,
			ConfigRoots:          orEmpty(plugin.ConfigRoots),
			Dependencies:         orEmpty(plugin.Dependencies),
			OptionalDependencies: orEmpty(plugin.OptionalDependencies),
			SourceDir:            plugin.SourceDir,
			YangFiles:            orEmpty(plugin.YANGFiles),
		})
	}
	// The registry answers in name order and the file keeps it, so a build that
	// changes no registration writes the same bytes.
	sort.SliceStable(published, func(left, right int) bool { return published[left].Name < published[right].Name })
	content, err := json.MarshalIndent(published, "", "  ")
	if err != nil {
		return "", fmt.Errorf("publish the plugin registry: %w", err)
	}
	return string(content) + "\n", nil
}

// orEmpty answers an empty slice for an absent one, so a JSON reader meets [].
func orEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// pluginEntry is one plugin as the catalog holds it: what the registry states,
// plus the slug its page sits at and the group it is filed under.
type pluginEntry struct {
	registryPlugin
	Slug  string
	Group string
}

// isTest reports whether this plugin exists for the test suite.
func (entry *pluginEntry) isTest() bool {
	return strings.HasPrefix(entry.SourceDir, testPluginPrefix)
}

// pluginGroup is one area of the catalog: the plugins filed under it, and what
// the filter controls say about it.
type pluginGroup struct {
	ID      string
	Label   string
	Short   string
	Tone    string
	Bucket  string
	Roots   []string
	Sources []string
	Plugins []*pluginEntry
}

// renderPluginCatalog publishes the catalog and one detail page for each
// registered plugin, and answers every route it wrote.
func renderPluginCatalog(paths Paths) ([]string, error) {
	var published []registryPlugin
	if err := readArtifactJSON(paths.Output, pluginFile, &published); err != nil {
		return nil, err
	}
	if len(published) == 0 {
		return nil, fmt.Errorf("the published %s names no plugin", pluginFile)
	}

	entries, err := pluginEntries(published)
	if err != nil {
		return nil, err
	}
	groups := pluginGroups(entries)
	links, err := loadPageLinks(paths.Source)
	if err != nil {
		return nil, err
	}

	if err := removeRetiredPluginPages(paths.Output, entries); err != nil {
		return nil, err
	}

	routes := make([]string, 0, len(entries)+1)
	route, err := writePluginCatalogPage(paths.Output, entries, groups, links)
	if err != nil {
		return nil, err
	}
	routes = append(routes, route)

	dependents := pluginDependents(entries)
	byName := make(map[string]*pluginEntry, len(entries))
	for _, entry := range entries {
		byName[entry.Name] = entry
	}
	byGroup := make(map[string]*pluginGroup, len(groups))
	for index := range groups {
		byGroup[groups[index].ID] = groups[index]
	}
	for _, entry := range entries {
		group, filed := byGroup[entry.Group]
		if !filed {
			return nil, fmt.Errorf("plugin %q is filed under area %q, which no group carries",
				entry.Name, entry.Group)
		}
		route, err := writePluginDetailPage(paths.Output, entry, group, byName, dependents, links)
		if err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}
	return routes, nil
}

// pluginEntries files every published registration under its group and gives it
// the slug its page sits at.
func pluginEntries(published []registryPlugin) ([]*pluginEntry, error) {
	entries := make([]*pluginEntry, 0, len(published))
	for index := range published {
		plugin := published[index]
		if plugin.Name == "" {
			return nil, fmt.Errorf("the published %s carries a plugin with no name", pluginFile)
		}
		if plugin.SourceDir == "" {
			return nil, fmt.Errorf("the published %s states no source directory for %q", pluginFile, plugin.Name)
		}
		entries = append(entries, &pluginEntry{registryPlugin: plugin, Group: pluginGroupOf(&plugin)})
	}
	assignPluginSlugs(entries)
	// The catalog lists by group label and then by name, which is the order the
	// page and its mirror both walk.
	sort.SliceStable(entries, func(left, right int) bool {
		leftLabel, rightLabel := pluginGroupLabel(entries[left].Group), pluginGroupLabel(entries[right].Group)
		if leftLabel != rightLabel {
			return leftLabel < rightLabel
		}
		return entries[left].Name < entries[right].Name
	})
	return entries, nil
}

// assignPluginSlugs gives each plugin the directory its detail page sits in.
//
// The walk is by NAME rather than by the catalog's own order, so the slug a
// plugin gets does not move when its group label changes. A second plugin
// reaching one slug takes a numbered one, which is why the walk order matters.
func assignPluginSlugs(entries []*pluginEntry) {
	byName := append([]*pluginEntry{}, entries...)
	sort.SliceStable(byName, func(left, right int) bool { return byName[left].Name < byName[right].Name })
	used := make(map[string]int, len(byName))
	for _, entry := range byName {
		base := pluginSlug(entry.Name)
		count := used[base]
		used[base] = count + 1
		entry.Slug = base
		if count > 0 {
			entry.Slug = base + "-" + strconv.Itoa(count+1)
		}
	}
}

// pluginSlug folds a plugin name into the directory its page sits in.
func pluginSlug(name string) string {
	var slug strings.Builder
	previousDash := true
	for _, letter := range strings.ToLower(name) {
		if (letter >= 'a' && letter <= 'z') || (letter >= '0' && letter <= '9') {
			slug.WriteRune(letter)
			previousDash = false
			continue
		}
		if !previousDash {
			slug.WriteByte('-')
			previousDash = true
		}
	}
	folded := strings.Trim(slug.String(), "-")
	if folded == "" {
		return "plugin"
	}
	return folded
}

// pluginGroupOf answers the area a plugin is filed under, derived from the
// repository's own layout and the plugin's config roots rather than from a
// hand-written map of plugin names.
func pluginGroupOf(plugin *registryPlugin) string {
	parts := pathSegments(plugin.SourceDir)
	if strings.HasPrefix(plugin.SourceDir, testPluginPrefix) {
		return testHarnessGroup
	}

	// The BGP engine registers many plugins with no config root at all, so its
	// sub-areas come from the source layout: the filters, the NLRI codecs and
	// the two redistribution halves each read as their own area.
	if len(parts) >= 3 && parts[0] == "internal" && parts[1] == "component" && parts[2] == bgpGroup {
		return bgpGroupOf(parts)
	}

	if root := topConfigRoot(plugin); root != "" {
		return aliasPluginGroup(root)
	}
	if len(parts) >= 3 && parts[0] == "internal" && (parts[1] == "component" || parts[1] == "plugins") {
		return aliasPluginGroup(parts[2])
	}
	first, _, _ := strings.Cut(pluginSlug(plugin.Name), "-")
	return aliasPluginGroup(first)
}

// bgpGroupOf answers the BGP sub-area a package under the engine belongs to.
func bgpGroupOf(parts []string) string {
	if len(parts) >= 5 && parts[3] == "plugins" {
		sub := parts[4]
		prefix, _, _ := strings.Cut(sub, "_")
		if sub == "nlri" && len(parts) >= 6 {
			return "bgp-nlri"
		}
		if prefix == filterSegment || prefix == "redistribute" {
			return bgpGroup + "-" + prefix
		}
	}
	if slices.Contains(parts, filterSegment) {
		return bgpGroup + "-" + filterSegment
	}
	return bgpGroup
}

// aliasPluginGroup maps a package's code name onto the public noun.
func aliasPluginGroup(id string) string {
	if alias, found := pluginGroupAliases[id]; found {
		return alias
	}
	return id
}

// topConfigRoot answers the top-level config section a plugin owns, or the
// empty string when it declares none.
func topConfigRoot(plugin *registryPlugin) string {
	if len(plugin.ConfigRoots) == 0 {
		return ""
	}
	top, _, _ := strings.Cut(plugin.ConfigRoots[0], "/")
	return top
}

// pathSegments splits a slash path, dropping the empty segments a leading or
// doubled separator leaves.
func pathSegments(path string) []string {
	segments := make([]string, 0, 6)
	for part := range strings.SplitSeq(path, "/") {
		if part != "" {
			segments = append(segments, part)
		}
	}
	return segments
}

// pluginGroupLabel spells a group identifier for a reader.
func pluginGroupLabel(id string) string {
	var label strings.Builder
	for _, token := range strings.FieldsFunc(id, func(letter rune) bool { return letter == '-' || letter == '_' }) {
		if label.Len() > 0 {
			label.WriteByte(' ')
		}
		label.WriteString(pluginLabelWord(token))
	}
	return label.String()
}

// pluginLabelWord spells one token of a group label.
func pluginLabelWord(token string) string {
	if acronym, found := pluginAcronyms[token]; found {
		return acronym
	}
	switch token {
	case "and", "for", "of", "to":
		return token
	}
	return strings.ToUpper(token[:1]) + token[1:]
}

// pluginGroupShort is the label the card corner carries, cut to fit.
func pluginGroupShort(id string) string {
	label := pluginGroupLabel(id)
	if len(label) <= 18 {
		return label
	}
	return strings.TrimRight(label[:16], " ") + "..."
}

// pluginGroupTone picks one of the presentation tones for a group, from the
// group's own identifier, so a group keeps its color between builds.
func pluginGroupTone(id string) string {
	sum := 0
	for _, letter := range id {
		sum += int(letter)
	}
	return presentationTones[sum%len(presentationTones)]
}

// pluginGroups files every plugin under its area and answers the areas in the
// order the page writes them: by label, with the test fixtures last.
func pluginGroups(entries []*pluginEntry) []*pluginGroup {
	byID := make(map[string]*pluginGroup)
	roots := make(map[string]map[string]bool)
	sources := make(map[string]map[string]bool)
	var groups []*pluginGroup
	for _, entry := range entries {
		group, found := byID[entry.Group]
		if !found {
			group = &pluginGroup{
				ID:    entry.Group,
				Label: pluginGroupLabel(entry.Group),
				Short: pluginGroupShort(entry.Group),
				Tone:  pluginGroupTone(entry.Group),
			}
			byID[entry.Group] = group
			roots[entry.Group] = make(map[string]bool)
			sources[entry.Group] = make(map[string]bool)
			groups = append(groups, group)
		}
		group.Plugins = append(group.Plugins, entry)
		for _, root := range entry.ConfigRoots {
			top, _, _ := strings.Cut(root, "/")
			roots[entry.Group][top] = true
		}
		segments := pathSegments(entry.SourceDir)
		if len(segments) >= 3 {
			sources[entry.Group][strings.Join(segments[:3], "/")] = true
		} else {
			sources[entry.Group][entry.SourceDir] = true
		}
	}

	for _, group := range groups {
		sort.SliceStable(group.Plugins, func(left, right int) bool {
			return group.Plugins[left].Name < group.Plugins[right].Name
		})
		group.Roots = sortedSetKeys(roots[group.ID])
		group.Sources = sortedSetKeys(sources[group.ID])
		group.Bucket = pluginBucketOf(group)
	}
	// The test harness sorts last whatever its label is, so a reader scanning
	// the page reaches every runtime area before the first fixture.
	sort.SliceStable(groups, func(left, right int) bool {
		leftTest, rightTest := groups[left].ID == testHarnessGroup, groups[right].ID == testHarnessGroup
		if leftTest != rightTest {
			return rightTest
		}
		return groups[left].Label < groups[right].Label
	})
	return groups
}

// sortedSetKeys answers a set's members in order, so no output order comes from
// a Go map.
func sortedSetKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// pluginBucketOf answers the coarse category filter bucket a group falls in.
//
// The tests are ordered and the first match wins, so a group naming both a
// tunnel and a filter reads as tunneling. The words are matched against the
// group's own generated identity: its id, its label, its config roots and its
// source areas, never against a list of plugin names.
func pluginBucketOf(group *pluginGroup) string {
	if group.ID == testHarnessGroup {
		return bucketTest
	}
	text := strings.ToLower(strings.Join(append([]string{group.ID, group.Label},
		append(append([]string{}, group.Roots...), group.Sources...)...), " "))
	for _, rule := range []struct {
		bucket string
		tokens []string
	}{
		{bucketTunneling, []string{"l2tp", "ppp", "pppoe", "ipsec", "ike", "vpn", "ldp", "rsvp", "mpls"}},
		{bucketSecurity, []string{"firewall", "ddos", "anomaly", "policy", "filter", "rpki", "copp", "aaa", "tacacs", "pki"}},
		{bucketServices, []string{"service", "dhcp", "dns", "ntp", "as112", "image"}},
		{bucketDataplane, []string{"iface", "interface", "fib", "vpp", "kernel", "sysctl", "bfd"}},
		{bucketTelemetry, []string{"flow", "mrt", "traffic", "telemetry", "monitor", "watchdog"}},
		{bucketRouting, []string{bgpGroup, "ospf", "isis", "rib", "route", "routing", "static", "connected", "redistribute"}},
	} {
		for _, token := range rule.tokens {
			if strings.Contains(text, token) {
				return rule.bucket
			}
		}
	}
	return bucketSystem
}

// pluginRelations names, for one plugin, every plugin that depends on it.
type pluginRelations struct {
	Required []*pluginEntry
	Optional []*pluginEntry
}

// pluginDependents inverts the dependency declarations, so a detail page can
// say who uses this plugin as well as what it uses.
func pluginDependents(entries []*pluginEntry) map[string]*pluginRelations {
	byName := make(map[string]*pluginEntry, len(entries))
	relations := make(map[string]*pluginRelations, len(entries))
	for _, entry := range entries {
		byName[entry.Name] = entry
		relations[entry.Name] = &pluginRelations{}
	}
	for _, entry := range entries {
		for _, dependency := range entry.Dependencies {
			if _, found := byName[dependency]; found {
				relations[dependency].Required = append(relations[dependency].Required, entry)
			}
		}
		for _, dependency := range entry.OptionalDependencies {
			if _, found := byName[dependency]; found {
				relations[dependency].Optional = append(relations[dependency].Optional, entry)
			}
		}
	}
	for _, relation := range relations {
		sort.SliceStable(relation.Required, func(left, right int) bool {
			return relation.Required[left].Name < relation.Required[right].Name
		})
		sort.SliceStable(relation.Optional, func(left, right int) bool {
			return relation.Optional[left].Name < relation.Optional[right].Name
		})
	}
	return relations
}

// removeRetiredPluginPages deletes the page of a plugin this build no longer
// publishes, so a renamed or withdrawn plugin stops being served.
//
// The retired renderer cleared every detail directory on each run. The effect
// is the same and the reason is the one AC-2 states: a page that survives on
// the seed alone is frozen content with a fresh timestamp.
func removeRetiredPluginPages(output string, entries []*pluginEntry) error {
	live := make(map[string]bool, len(entries))
	for _, entry := range entries {
		live[entry.Slug] = true
	}
	root := filepath.Join(output, filepath.FromSlash(pluginsDirectory))
	directory, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range directory {
		if !entry.IsDir() || live[entry.Name()] {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// writePluginCatalogPage publishes the catalog and its mirror, and answers the
// route it wrote.
func writePluginCatalogPage(output string, entries []*pluginEntry, groups []*pluginGroup, links pageLinks) (string, error) {
	shell := pageShell{
		Title:       "Plugin Catalog - Ze",
		Description: "Search every Ze runtime plugin by purpose, config root, dependency, and source.",
		Root:        pluginsRoot,
		Path:        pluginsDest,
		Sidebar:     pageSidebar(pluginsRoot, pluginsRoute[1:], links),
	}
	if err := writePublishedPage(output, pluginsDest,
		shell.render(pluginCatalogBody(entries, groups)), pluginCatalogMirror(entries, groups)); err != nil {
		return "", err
	}
	return pluginsRoute, nil
}

// pluginCounts answers the four numbers the catalog states about itself.
type pluginCounts struct {
	Total      int
	Runtime    int
	Fixtures   int
	Configured int
	Dependent  int
	WithYANG   int
}

// countPlugins counts the catalog, keeping runtime plugins and test fixtures
// apart: every derived number describes the runtime set, which is what a reader
// choosing what to deploy is counting.
func countPlugins(entries []*pluginEntry) pluginCounts {
	counts := pluginCounts{Total: len(entries)}
	for _, entry := range entries {
		if entry.isTest() {
			counts.Fixtures++
			continue
		}
		counts.Runtime++
		if len(entry.ConfigRoots) != 0 {
			counts.Configured++
		}
		if len(entry.Dependencies) != 0 || len(entry.OptionalDependencies) != 0 {
			counts.Dependent++
		}
		if len(entry.YangFiles) != 0 {
			counts.WithYANG++
		}
	}
	return counts
}

// fixtureClause states how many test fixtures the catalog carries, after the
// words that join it to the sentence, and states nothing when it carries none.
//
// The composition root the registry is read from excludes internal/test/plugins
// by design, so a build of this checkout counts zero. The clause stays because
// the count is derived rather than assumed, and reads correctly either way.
func fixtureClause(conjunction string, fixtures int) string {
	if fixtures == 0 {
		return ""
	}
	return conjunction + plural(fixtures, "test fixture")
}

// pluginCatalogBody renders the catalog under <main>: the hero, the counts, the
// search console, and one section for each area.
func pluginCatalogBody(entries []*pluginEntry, groups []*pluginGroup) string {
	counts := countPlugins(entries)
	var body strings.Builder
	body.WriteString(`            <section class="md-content reveal cat-automate plugin-catalog" data-plugin-catalog aria-labelledby="plugin-catalog-title">` + "\n")
	body.WriteString(pageHero("Plugin catalog",
		"Ze features are composed from plugins. This catalog is generated from the live registry "+
			"and explains what each plugin is for, what it configures, and which other plugins it "+
			"relies on. Click any plugin to open a local detail page.",
		"Plugins", ` id="plugin-catalog-title"`, "journey-hero reveal") + "\n")
	body.WriteString(`                <p class="plugin-summary">Generated from ` +
		strconv.Itoa(counts.Total) + " registry entries: " + plural(counts.Runtime, "runtime plugin") +
		fixtureClause(" and ", counts.Fixtures) +
		". Among runtime plugins, " + strconv.Itoa(counts.Configured) +
		" declare configuration roots, " + strconv.Itoa(counts.Dependent) +
		" declare dependencies, and " + strconv.Itoa(counts.WithYANG) + " ship YANG modules.</p>\n")
	body.WriteString(`                <div class="plugin-console" role="search" aria-label="Search plugins">` + "\n")
	body.WriteString(`                    <label for="plugin-search">Search by feature, protocol, config root, dependency, or source path</label>` + "\n")
	body.WriteString(`                    <input id="plugin-search" type="search" autocomplete="off" placeholder="Try RPKI, FlowSpec, FIB, DHCP, DDoS, l2tp, bgp-filter..." />` + "\n")
	body.WriteString(pluginFilterControls(groups) + "\n")
	body.WriteString(`                    <p id="plugin-status" class="plugin-status search-status" aria-live="polite"></p>` + "\n")
	body.WriteString("                </div>\n")
	body.WriteString(`                <p class="plugin-empty" hidden>No plugins match this search.</p>` + "\n")
	body.WriteString(`                <div class="plugin-groups">` + "\n")
	for _, group := range groups {
		body.WriteString(pluginGroupHTML(group))
	}
	body.WriteString("                </div>\n")
	body.WriteString("            </section>\n")
	body.WriteString(pluginCatalogScript)
	return body.String()
}

// pluginFilterControls renders the two selects that narrow the catalog: one
// over the coarse buckets and one over the areas inside the chosen bucket.
func pluginFilterControls(groups []*pluginGroup) string {
	var categories, areas strings.Builder
	categories.WriteString(`<option value="">All categories</option>`)
	areas.WriteString(`<option value="" data-label="All areas">All areas</option>`)
	for _, bucket := range pluginBucketOrder {
		filed := make([]*pluginGroup, 0, len(groups))
		count := 0
		for _, group := range groups {
			if group.Bucket == bucket {
				filed = append(filed, group)
				count += len(group.Plugins)
			}
		}
		if len(filed) == 0 {
			continue
		}
		label := html.EscapeString(pluginBucketLabels[bucket])
		categories.WriteString("\n                                    " +
			`<option value="` + html.EscapeString(bucket) + `" data-label="` + label + `">` +
			label + " (" + strconv.Itoa(count) + ")</option>")
		areas.WriteString("\n                                    <optgroup label=\"" + label + "\">")
		for _, group := range filed {
			groupLabel := html.EscapeString(group.Label)
			areas.WriteString("\n                                    " +
				`<option value="` + html.EscapeString(group.ID) + `" data-category="` + html.EscapeString(bucket) +
				`" data-label="` + groupLabel + `">` + groupLabel + " (" +
				strconv.Itoa(len(group.Plugins)) + ")</option>")
		}
		areas.WriteString("\n                                    </optgroup>")
	}
	return "\n" +
		`                        <div class="plugin-filter-controls">` + "\n" +
		`                            <label for="plugin-category">Category` + "\n" +
		`                                <select id="plugin-category" autocomplete="off">` + "\n" +
		"                                    " + categories.String() + "\n" +
		"                                </select>\n" +
		"                            </label>\n" +
		`                            <label for="plugin-family">Area` + "\n" +
		`                                <select id="plugin-family" autocomplete="off">` + "\n" +
		"                                    " + areas.String() + "\n" +
		"                                </select>\n" +
		"                            </label>\n" +
		"                        </div>"
}

// pluginGroupHTML renders one area of the catalog: its heading, the sentence
// that says how the area was derived, and one card for each plugin in it.
func pluginGroupHTML(group *pluginGroup) string {
	var cards strings.Builder
	for index, entry := range group.Plugins {
		if index > 0 {
			cards.WriteString("\n")
		}
		cards.WriteString(pluginCardHTML(entry, group))
	}
	return "\n" +
		`                <section class="plugin-group" data-plugin-group data-family="` + html.EscapeString(group.ID) +
		`" data-category="` + html.EscapeString(group.Bucket) + `" aria-labelledby="plugin-group-` + html.EscapeString(group.ID) + `">` + "\n" +
		`                    <div class="plugin-group-head tone-` + group.Tone + `">` + "\n" +
		`                        <h2 id="plugin-group-` + html.EscapeString(group.ID) + `">` + html.EscapeString(group.Label) + "</h2>\n" +
		"                        <span>" + plural(len(group.Plugins), "plugin") + "</span>\n" +
		"                    </div>\n" +
		"                    <p>" + inlineMarkup(pluginGroupDeck(group)) + "</p>\n" +
		`                    <div class="cards plugin-grid">` + "\n" +
		cards.String() + "\n" +
		"                    </div>\n" +
		"                </section>"
}

// pluginGroupDeck states how one area was derived, in the marker vocabulary the
// data pages use: backticks for a config root or a source path.
func pluginGroupDeck(group *pluginGroup) string {
	deck := "Generated group for registry entries mapped to the " + group.Label + " area."
	if len(group.Roots) != 0 {
		deck += " Config roots: " + codeMarkerList(group.Roots) + "."
	}
	if len(group.Sources) != 0 {
		sources := group.Sources
		if len(sources) > 3 {
			sources = sources[:3]
		}
		deck += " Source area: " + codeMarkerList(sources) + "."
	}
	return deck
}

// codeMarkerList joins values as backtick-marked code, comma separated.
func codeMarkerList(values []string) string {
	marked := make([]string, 0, len(values))
	for _, value := range values {
		marked = append(marked, "`"+value+"`")
	}
	return strings.Join(marked, ", ")
}

// pluginCardHTML renders one plugin's card in the catalog.
func pluginCardHTML(entry *pluginEntry, group *pluginGroup) string {
	test := ""
	if entry.isTest() {
		test = ` data-test="true"`
	}
	return "\n" +
		`                    <article class="card plugin-card tone-` + group.Tone + `" id="plugin-` + html.EscapeString(entry.Slug) +
		`" data-plugin-card data-family="` + html.EscapeString(group.ID) + `" data-category="` + html.EscapeString(group.Bucket) +
		`" data-search="` + html.EscapeString(pluginSearchText(entry, group)) + `"` + test + ">\n" +
		`                        <span class="cat">` + html.EscapeString(group.Short) + "</span>\n" +
		`                        <h3><a href="` + html.EscapeString(entry.Slug) + `/"><code>` + html.EscapeString(entry.Name) + "</code></a></h3>\n" +
		`                        <p class="plugin-desc">` + html.EscapeString(entry.Description) + "</p>\n" +
		`                        <div class="chips">` + "\n" +
		"                            " + strings.Join(pluginChips(entry), "\n                            ") + "\n" +
		"                        </div>\n" +
		"                        " + pluginMetaHTML(entry) + "\n" +
		"                    </article>"
}

// pluginSearchText is what the browser-side search matches a query against: the
// plugin's name, its purpose, where it lives, its area, and every config root,
// dependency and YANG file it names.
func pluginSearchText(entry *pluginEntry, group *pluginGroup) string {
	return strings.Join([]string{
		entry.Name,
		entry.Description,
		entry.Description,
		entry.SourceDir,
		group.Label,
		"",
		"",
		strings.Join(entry.ConfigRoots, " "),
		strings.Join(entry.Dependencies, " "),
		strings.Join(entry.OptionalDependencies, " "),
		strings.Join(entry.YangFiles, " "),
	}, " ")
}

// pluginChips are the short badges on a card. The first three dependencies are
// named and the rest are counted, so a card stays one glance wide.
func pluginChips(entry *pluginEntry) []string {
	var chips []string
	for _, root := range entry.ConfigRoots {
		chips = append(chips, `<span class="chip mode">`+html.EscapeString("config:"+root)+"</span>")
	}
	named := entry.Dependencies
	if len(named) > 3 {
		named = named[:3]
	}
	for _, dependency := range named {
		chips = append(chips, `<span class="chip">`+html.EscapeString("needs:"+dependency)+"</span>")
	}
	if len(entry.Dependencies) > 3 {
		chips = append(chips, `<span class="chip">`+html.EscapeString("+"+strconv.Itoa(len(entry.Dependencies)-3)+" deps")+"</span>")
	}
	if len(entry.OptionalDependencies) != 0 {
		chips = append(chips, `<span class="chip">`+html.EscapeString("optional:"+entry.OptionalDependencies[0])+"</span>")
	}
	if len(entry.YangFiles) != 0 {
		chips = append(chips, `<span class="chip">`+html.EscapeString("YANG:"+strconv.Itoa(len(entry.YangFiles)))+"</span>")
	}
	if len(chips) == 0 {
		chips = append(chips, `<span class="chip">no config</span>`)
	}
	return chips
}

// pluginMetaHTML is the definition list under a card's chips. A plugin that
// declares nothing states "None" rather than showing an empty list.
func pluginMetaHTML(entry *pluginEntry) string {
	var rows strings.Builder
	if len(entry.ConfigRoots) != 0 {
		rows.WriteString(pluginMetaRow("Config", codeList(entry.ConfigRoots)))
	}
	if len(entry.Dependencies) != 0 {
		rows.WriteString(pluginMetaRow("Needs", codeList(entry.Dependencies)))
	}
	if len(entry.OptionalDependencies) != 0 {
		rows.WriteString(pluginMetaRow("Optional", codeList(entry.OptionalDependencies)))
	}
	if len(entry.YangFiles) != 0 {
		rows.WriteString(pluginMetaRow("YANG", plural(len(entry.YangFiles), "module")))
	}
	if rows.Len() == 0 {
		rows.WriteString(pluginMetaRow("Config", nothingDeclared))
	}
	return `<dl class="plugin-meta">` + rows.String() + "</dl>"
}

// pluginMetaRow is one term and its value in a card's definition list.
func pluginMetaRow(term, value string) string {
	return "<div><dt>" + html.EscapeString(term) + "</dt><dd>" + value + "</dd></div>"
}

// codeList marks each value as code and joins them with commas.
func codeList(values []string) string {
	marked := make([]string, 0, len(values))
	for _, value := range values {
		marked = append(marked, "<code>"+html.EscapeString(value)+"</code>")
	}
	return strings.Join(marked, ", ")
}

// pluginCatalogMirror renders the Markdown sibling of the catalog: the same
// areas in the same order, each as one table.
func pluginCatalogMirror(entries []*pluginEntry, groups []*pluginGroup) string {
	counts := countPlugins(entries)
	var mirror strings.Builder
	mirror.WriteString("# Plugin catalog\n\n")
	mirror.WriteString(plural(counts.Runtime, "runtime plugin") +
		" generated from `" + pluginFile + "`" + fixtureClause(", plus ", counts.Fixtures) + ". " +
		strconv.Itoa(counts.Configured) + " runtime plugins declare configuration roots and " +
		strconv.Itoa(counts.WithYANG) + " ship YANG modules.\n\n")
	mirror.WriteString("The HTML page includes browser-side search across name, purpose, config roots, " +
		"dependencies, YANG files, and source directories. Clicking a plugin opens its generated " +
		"local detail page.\n\n")
	for _, group := range groups {
		if len(group.Plugins) == 0 {
			continue
		}
		mirror.WriteString("## " + group.Label + "\n\n" + pluginGroupDeck(group) + "\n\n")
		mirror.WriteString("| Plugin | Used for | Config | Depends on | Source path |\n")
		mirror.WriteString("|--------|----------|--------|------------|-------------|\n")
		for _, entry := range group.Plugins {
			mirror.WriteString("| [`" + entry.Name + "`](" + entry.Slug + "/" + pageMirrorFile + ") | " +
				strings.ReplaceAll(entry.Description, "|", `\|`) + " | " +
				pluginOrNone(codeMarkerList(entry.ConfigRoots)) + " | " +
				pluginOrNone(codeMarkerList(entry.Dependencies)) + " | `" + entry.SourceDir + "` |\n")
		}
		mirror.WriteString("\n")
	}
	return strings.TrimRight(mirror.String(), "\n") + "\n"
}

// nothingDeclared is what the catalog writes where a plugin declares no config
// root, no dependency and no YANG file. The command surfaces spell the same
// idea in lower case, on their own pages.
const nothingDeclared = "None"

// pluginOrNone answers nothingDeclared for an empty cell, which is what a
// reader of a catalog mirror needs to tell "nothing declared" from a rendering
// fault.
func pluginOrNone(value string) string {
	if value == "" {
		return nothingDeclared
	}
	return value
}
