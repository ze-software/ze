// Design: website/AI.md -- one header mount and one page sidebar for every page
// Detail: shell.go places both; website/data/page-links.json states the sidebar.
package site

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// noscriptHubLinks are the section hubs a reader reaches when JavaScript is
// off. site.js replaces the mount's content with the full menu when it runs, so
// this list stays at the top-level hubs: adding a dropdown item to nav.json
// would otherwise rewrite this fallback into every page, which is the per-page
// churn the mounted header exists to avoid.
var noscriptHubLinks = []struct {
	Href  string
	Label string
}{
	{"index.html#top", "Home"},
	{"docs/", "Docs"},
	{sectionFeatures, "Features"},
	{"compare/", "Compare"},
	{"project/roadmap/", "Roadmap"},
	{"project/changes/", "Changes"},
	{"faq/", labelFAQ},
	{"contribute/", "Contribute"},
}

// rootedHref answers a site-root-relative href that never spells a directory
// index. The homepage is linked as "../" rather than "../index.html", so a
// crawler and a reader see one URL for one page.
func rootedHref(root, target string) string {
	if target == pageIndexFile {
		if root == "" {
			return "./"
		}
		return root
	}
	if fragment, isHome := strings.CutPrefix(target, pageIndexFile); isHome {
		return root + fragment
	}
	return root + target
}

// headerMount renders the per-page mount the shared header fragment loads into.
//
// The mount carries a noscript fallback so a page is never a dead end without
// JavaScript. The fallback nests no further div, which keeps the mount one
// element a later pass can find and replace.
func headerMount(root string) string {
	var mount strings.Builder
	mount.WriteString(`        <div id="site-header-mount" data-header-src="` +
		html.EscapeString(root) + `assets/header.html" data-site-root="` +
		html.EscapeString(root) + "\">\n")
	mount.WriteString("            <noscript>\n")
	mount.WriteString("                <nav class=\"site-header-fallback\" aria-label=\"Site navigation\">\n")
	mount.WriteString(`                    <a class="brand" href="` +
		html.EscapeString(rootedHref(root, "index.html#top")) + "\">Ze</a>\n")
	for _, link := range noscriptHubLinks {
		mount.WriteString(`                    <a href="` +
			html.EscapeString(rootedHref(root, link.Href)) + `">` +
			html.EscapeString(link.Label) + "</a>\n")
	}
	mount.WriteString("                </nav>\n")
	mount.WriteString("            </noscript>\n")
	mount.WriteString("        </div>")
	return mount.String()
}

// The shared header fragment is registered from here. It answers no route: it is
// one file every page loads, and namedArtifacts is what refuses a build that
// stops writing it.
func init() {
	registerProducer(Producer{Name: "header", Render: renderSharedHeader})
}

// Where the shared header is published, what it reads, and the placeholder it
// spells the site root with.
//
// assets/site.js substitutes the placeholder for each page's own root when it loads
// the fragment, so ONE file serves a page at any depth. A resolved root would
// need one fragment for each depth, which is the per-page churn the mount
// exists to remove.
const (
	sharedHeaderFile            = "assets/header.html"
	sharedHeaderRootPlaceholder = "__ZE_SITE_ROOT__"
	navDataFile                 = "nav.json"
)

// siteNav is website/data/nav.json: the curated menu every page shows.
//
// It is ONE model of that file, read here to render the menu and read by
// derived.go to write llms.txt's page map. Two models of one file drift, and
// the narrower of the two is the one that silently drops a field: this model
// was declared for llms.txt alone, which reads neither the icon nor the top
// links, so a header rendered from it would have published a menu of unmarked
// entries.
type siteNav struct {
	TopLinks      []navLink     `json:"top_links"`
	Dropdowns     []navDropdown `json:"dropdowns"`
	TrailingLinks []navLink     `json:"trailing_links"`
}

// navLink is one bare link of the menu bar.
type navLink struct {
	Href  string `json:"href"`
	Label string `json:"label"`
}

// navDropdown is one labeled panel of the menu, holding its entries in columns.
type navDropdown struct {
	Label   string       `json:"label"`
	Columns [][]navEntry `json:"columns"`
}

// navEntry is one line of a dropdown column: a column heading when LabelOnly is
// set, and a link to a page otherwise. Feature marks the one entry of a column
// a reader is meant to read first, which the menu draws differently.
type navEntry struct {
	LabelOnly string `json:"label_only"`
	Href      string `json:"href"`
	Icon      string `json:"icon"`
	Title     string `json:"title"`
	Desc      string `json:"desc"`
	Feature   bool   `json:"feature"`
}

// renderSharedHeader publishes assets/header.html, the one navigation fragment
// every page of the site loads into its header mount.
//
// It reads the menu from the committed website/data/nav.json and the four live
// counts and the star number from the facts snapshot refreshNativeSurfaces
// wrote before any producer ran.
func renderSharedHeader(paths Paths) ([]string, error) {
	var data siteNav
	if err := readSourceJSON(paths.Source, navDataFile, &data); err != nil {
		return nil, err
	}
	var facts siteFacts
	if err := readArtifactJSON(paths.Output, factsFile, &facts); err != nil {
		return nil, err
	}
	header, err := sharedHeaderHTML(data, &facts)
	if err != nil {
		return nil, err
	}
	return nil, writeNamedArtifact(paths.Output, sharedHeaderFile, header)
}

// sharedHeaderHTML renders the whole fragment: the brand, the menu button and
// the navigation links, with the site root left as the token.
func sharedHeaderHTML(data siteNav, facts *siteFacts) (string, error) {
	root := sharedHeaderRootPlaceholder
	links, err := navLinksHTML(root, data, facts)
	if err != nil {
		return "", err
	}
	var header strings.Builder
	header.WriteString("        <header class=\"site-header\">\n")
	header.WriteString("            <nav class=\"nav\" aria-label=\"Main navigation\">\n")
	header.WriteString(`                <a class="brand" href="` +
		html.EscapeString(rootedHref(root, "index.html#top")) + "\" aria-label=\"Ze home\">\n")
	header.WriteString(`                    <img src="` + html.EscapeString(root) +
		"assets/ze.svg\" alt=\"\" width=\"32\" height=\"32\" />\n")
	header.WriteString("                    <span>Ze</span>\n")
	header.WriteString("                </a>\n")
	header.WriteString(`                <button class="nav-menu-toggle" type="button" ` +
		"aria-controls=\"site-nav-links\" aria-expanded=\"false\">\n")
	header.WriteString("                    <span class=\"nav-menu-toggle-bars\" aria-hidden=\"true\"></span>\n")
	header.WriteString("                    <span>Menu</span>\n")
	header.WriteString("                </button>\n")
	header.WriteString(links)
	header.WriteString("\n            </nav>\n        </header>\n")
	return header.String(), nil
}

// navLinksHTML renders the links block: the bare links, the dropdowns, the
// badges and the theme toggle, in the order nav.json states them.
func navLinksHTML(root string, data siteNav, facts *siteFacts) (string, error) {
	counts := navCounts(facts)
	var links strings.Builder
	links.WriteString("                <div id=\"site-nav-links\" class=\"nav-links\">\n")
	for _, link := range data.TopLinks {
		links.WriteString(navBarLink(root, link))
	}
	for _, dropdown := range data.Dropdowns {
		panel, err := navDropdownHTML(root, dropdown, counts)
		if err != nil {
			return "", err
		}
		links.WriteString(panel)
	}
	for _, link := range data.TrailingLinks {
		links.WriteString(navBarLink(root, link))
	}
	links.WriteString(navSearchBadge(root))
	links.WriteString(navBadge(discordInvite, "Ze Discord", "0 0 640 512", discordIconPath, "Discord"))
	links.WriteString(navBadge(repositoryURL, "Ze on GitHub, "+strconv.Itoa(facts.GitHubStars)+" stars",
		"0 0 496 512", githubIconPath, strconv.Itoa(facts.GitHubStars)))
	links.WriteString(navThemeToggle)
	links.WriteString("                </div>")
	return links.String(), nil
}

// navCounts answers the live numbers a menu description states, keyed by the
// placeholder nav.json spells each one with.
//
// The six are the whole set a description may name. A description naming
// anything else is refused rather than published with the placeholder showing,
// which is what navEntryDescription is for.
func navCounts(facts *siteFacts) *strings.Replacer {
	return strings.NewReplacer(
		"%(features)s", strconv.Itoa(facts.Features.CoreExperimental),
		"%(cli_commands)s", strconv.Itoa(facts.CLICommands),
		"%(config_sections)s", strconv.Itoa(facts.ConfigSections),
		"%(dependencies)s", strconv.Itoa(facts.Dependencies),
		"%(changes)s", strconv.Itoa(facts.Changes),
		"%(articles)s", strconv.Itoa(facts.BlogArticles),
	)
}

// navBarLink renders one bare link of the menu bar.
func navBarLink(root string, link navLink) string {
	return `                    <a href="` + html.EscapeString(rootedHref(root, link.Href)) +
		`">` + html.EscapeString(link.Label) + "</a>\n"
}

// navDropdownHTML renders one dropdown: its trigger, its panel, and one column
// for each column nav.json states.
//
// A dropdown with no entry is REFUSED by name. The retired renderer had one
// dynamic dropdown whose entries were generated rather than declared, and a
// dropdown whose generator went away renders as an empty panel a reader can
// open and find nothing in.
func navDropdownHTML(root string, dropdown navDropdown, counts *strings.Replacer) (string, error) {
	if dropdown.Label == "" {
		return "", fmt.Errorf("data/%s: a dropdown states no label", navDataFile)
	}
	panelID := "nav-panel-" + navSlug(dropdown.Label)
	var panel strings.Builder
	panel.WriteString("                    <div class=\"nav-dropdown\">\n")
	panel.WriteString(`                    <button class="nav-dropdown-trigger" type="button" ` +
		`aria-haspopup="true" aria-expanded="false" aria-controls="` + panelID + `">` +
		html.EscapeString(dropdown.Label) + "\n")
	panel.WriteString("                        " + navChevron + "\n")
	panel.WriteString("                    </button>\n")
	panel.WriteString(`                    <div class="nav-dropdown-panel" id="` + panelID + "\">\n")
	entries := 0
	for _, column := range dropdown.Columns {
		panel.WriteString("                    <div class=\"nav-dropdown-col\">\n")
		for _, entry := range column {
			rendered, err := navEntryHTML(root, dropdown.Label, entry, counts)
			if err != nil {
				return "", err
			}
			panel.WriteString(rendered)
			entries++
		}
		panel.WriteString("                    </div>\n")
	}
	if entries == 0 {
		return "", fmt.Errorf("data/%s: the %s dropdown states no entry", navDataFile, dropdown.Label)
	}
	panel.WriteString("                    </div>\n")
	panel.WriteString("                    </div>\n")
	return panel.String(), nil
}

// navEntryHTML renders one line of a dropdown column.
func navEntryHTML(root, dropdown string, entry navEntry, counts *strings.Replacer) (string, error) {
	if entry.LabelOnly != "" {
		return `                        <span class="nav-dropdown-label">` +
			html.EscapeString(entry.LabelOnly) + "</span>\n", nil
	}
	if entry.Href == "" || entry.Title == "" {
		return "", fmt.Errorf("data/%s: an entry of the %s dropdown states no href or no title",
			navDataFile, dropdown)
	}
	description, err := navEntryDescription(dropdown, entry, counts)
	if err != nil {
		return "", err
	}
	class := "nav-dropdown-item"
	if entry.Feature {
		class += " nav-dropdown-feature"
	}
	return `                        <a class="` + class + `" href="` +
		html.EscapeString(root+entry.Href) + "\">\n" +
		`                            <span class="nav-dropdown-icon" aria-hidden="true">` +
		html.EscapeString(entry.Icon) + "</span>\n" +
		"                            <span><strong>" + html.EscapeString(entry.Title) +
		"</strong><small>" + html.EscapeString(description) + "</small></span>\n" +
		"                        </a>\n", nil
}

// navEntryDescription answers one entry's description with the live counts put
// in, and refuses a placeholder this build cannot answer.
//
// The refusal matters because the failure is silent otherwise: the reader gets
// "%(peers)s peers" in the menu of every page, and no check reads a menu.
func navEntryDescription(dropdown string, entry navEntry, counts *strings.Replacer) (string, error) {
	description := counts.Replace(entry.Desc)
	if strings.Contains(description, "%(") {
		return "", fmt.Errorf("data/%s: the %s entry %q states a count this build cannot answer: %s",
			navDataFile, dropdown, entry.Title, entry.Desc)
	}
	return description, nil
}

// navSlugPunctuation matches every run of characters a panel id cannot carry.
var navSlugPunctuation = regexp.MustCompile(`[^a-z0-9]+`)

// navSlug answers the id fragment one dropdown label is addressed by. A label
// of punctuation alone answers "menu", so the id is never empty.
func navSlug(label string) string {
	slug := strings.Trim(navSlugPunctuation.ReplaceAllString(strings.ToLower(label), "-"), "-")
	if slug == "" {
		return "menu"
	}
	return slug
}

// navSearchBadge renders the badge that opens the search page. It is the one
// badge that stays on this site, so it takes no target attribute.
func navSearchBadge(root string) string {
	return "                    <a\n" +
		"                        class=\"nav-badge nav-badge-search\"\n" +
		`                        href="` + html.EscapeString(root) + "search/\"\n" +
		"                        aria-label=\"Search the site\"\n" +
		"                        aria-expanded=\"false\"\n" +
		"                    >\n" +
		`                        <span class="nav-badge-icon"><svg viewBox="0 0 512 512" ` +
		`fill="currentColor" aria-hidden="true"><path d="` + searchIconPath + `"/></svg></span>` + "\n" +
		`                        <span class="nav-badge-count nav-badge-search-label">Search ` +
		`<span class="search-shortcut-hint" aria-hidden="true"><kbd>` + "\u2318" + `K</kbd><span>/</span></span></span>` + "\n" +
		"                    </a>\n"
}

// navBadge renders one badge linking off this site: the Discord invite and the
// code host, each with the count or the word it shows.
func navBadge(href, ariaLabel, viewBox, iconPath, count string) string {
	return "                    <a\n" +
		"                        class=\"nav-badge\"\n" +
		`                        href="` + html.EscapeString(href) + `" target="_blank" rel="noopener"` + "\n" +
		`                        aria-label="` + html.EscapeString(ariaLabel) + "\"\n" +
		"                    >\n" +
		`                        <span class="nav-badge-icon"><svg viewBox="` + viewBox +
		`" fill="currentColor" aria-hidden="true"><path d="` + iconPath + `"/></svg></span>` + "\n" +
		`                        <span class="nav-badge-count">` + html.EscapeString(count) + "</span>\n" +
		"                    </a>\n"
}

// navChevron is the arrow a dropdown trigger draws.
const navChevron = `<svg viewBox="0 0 12 8" fill="none" aria-hidden="true">` +
	`<path d="M1 1l5 5 5-5" stroke="currentColor" stroke-width="1.6" ` +
	`stroke-linecap="round" stroke-linejoin="round"/></svg>`

// navThemeToggle is the button that switches the page between the light and the
// dark theme. assets/site.js binds it and states which theme it moves to, so
// the markup is fixed and carries no data of its own.
const navThemeToggle = `                    <button
                        class="theme-toggle"
                        type="button"
                        data-theme-toggle
                        aria-label="Use dark theme"
                        aria-pressed="false"
                        title="Use dark theme"
                    >
                        <span class="theme-toggle-icon" aria-hidden="true">
                            <svg class="theme-icon-moon" viewBox="0 0 24 24" fill="none"><path d="M20.2 15.1A8.5 8.5 0 0 1 8.9 3.8 8.5 8.5 0 1 0 20.2 15.1Z" fill="currentColor"/></svg>
                            <svg class="theme-icon-sun" viewBox="0 0 24 24" fill="none"><circle cx="12" cy="12" r="4" fill="currentColor"/><path d="M12 2V5M12 19V22M2 12H5M19 12H22M4.9 4.9L7 7M17 17L19.1 19.1M19.1 4.9L17 7M7 17L4.9 19.1" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
                        </span>
                    </button>
`

// searchIconPath, discordIconPath and githubIconPath are the three icons the
// navigation badges draw. Each is the path data of one Font Awesome glyph, and
// the site draws it inline so a badge needs no second request.
const searchIconPath = "M416 208c0 45.9-14.9 88.3-40 122.7L502.6 457.4c12.5 12.5 12.5 32.8 0 45.3s-32.8 12.5-45." +
	"3 0L330.7 376c-34.4 25.2-76.8 40-122.7 40C93.1 416 0 322.9 0 208S93.1 0 208 0S416 93.1 4" +
	"16 208zM208 352a144 144 0 1 0 0-288 144 144 0 1 0 0 288z"

const discordIconPath = "M524.531,69.836a1.5,1.5,0,0,0-.764-.7A485.065,485.065,0,0,0,404.081,32.03a1.816,1.816,0," +
	"0,0-1.923.91,337.461,337.461,0,0,0-14.9,30.6,447.848,447.848,0,0,0-134.426,0,309.541,309" +
	".541,0,0,0-15.135-30.6,1.89,1.89,0,0,0-1.924-.91A483.689,483.689,0,0,0,116.085,69.137a1." +
	"712,1.712,0,0,0-.788.676C39.068,183.651,18.186,294.69,28.43,404.354a2.016,2.016,0,0,0,.7" +
	"65,1.375A487.666,487.666,0,0,0,176.02,479.918a1.9,1.9,0,0,0,2.063-.676A348.2,348.2,0,0,0" +
	",208.12,430.4a1.86,1.86,0,0,0-1.019-2.588,321.173,321.173,0,0,1-45.868-21.853,1.885,1.88" +
	"5,0,0,1-.185-3.126c3.082-2.309,6.166-4.711,9.109-7.137a1.819,1.819,0,0,1,1.9-.256c96.229" +
	",43.917,200.41,43.917,295.5,0a1.812,1.812,0,0,1,1.924.233c2.944,2.426,6.027,4.851,9.132," +
	"7.16a1.884,1.884,0,0,1-.162,3.126,301.407,301.407,0,0,1-45.89,21.83,1.875,1.875,0,0,0-1," +
	"2.611,391.055,391.055,0,0,0,30.014,48.815,1.864,1.864,0,0,0,2.063.7A486.048,486.048,0,0," +
	"0,610.7,405.729a1.882,1.882,0,0,0,.765-1.352C623.729,277.594,590.933,167.465,524.531,69." +
	"836ZM222.491,337.58c-28.972,0-52.844-26.587-52.844-59.239S193.056,219.1,222.491,219.1c29" +
	".665,0,53.306,26.82,52.843,59.239C275.334,310.993,251.924,337.58,222.491,337.58Zm195.38," +
	"0c-28.971,0-52.843-26.587-52.843-59.239S388.437,219.1,417.871,219.1c29.667,0,53.307,26.8" +
	"2,52.844,59.239C470.715,310.993,447.538,337.58,417.871,337.58Z"

const githubIconPath = "M165.9 397.4c0 2-2.3 3.6-5.2 3.6-3.3.3-5.6-1.3-5.6-3.6 0-2 2.3-3.6 5.2-3.6 3-.3 5.6 1.3 " +
	"5.6 3.6zm-31.1-4.5c-.7 2 1.3 4.3 4.3 4.9 2.6 1 5.6 0 6.2-2s-1.3-4.3-4.3-5.2c-2.6-.7-5.5." +
	"3-6.2 2.3zm44.2-1.7c-2.9.7-4.9 2.6-4.6 4.9.3 2 2.9 3.3 5.9 2.6 2.9-.7 4.9-2.6 4.6-4.6-.3" +
	"-1.9-3-3.2-5.9-2.9zM244.8 8C106.1 8 0 113.3 0 252c0 110.9 69.8 205.8 169.5 239.2 12.8 2." +
	"3 17.3-5.6 17.3-12.1 0-6.2-.3-40.4-.3-61.4 0 0-70 15-84.7-29.8 0 0-11.4-29.1-27.8-36.6 0" +
	" 0-22.9-15.7 1.6-15.4 0 0 24.9 2 38.6 25.8 21.9 38.6 58.6 27.5 72.9 20.9 2.3-16 8.8-27.1" +
	" 16-33.7-55.9-6.2-112.3-14.3-112.3-110.5 0-27.5 7.6-41.3 23.6-58.9-2.6-6.5-11.1-33.3 2.6" +
	"-67.9 20.9-6.5 69 27 69 27 20-5.6 41.5-8.5 62.8-8.5s42.8 2.9 62.8 8.5c0 0 48.1-33.6 69-2" +
	"7 13.7 34.7 5.2 61.4 2.6 67.9 16 17.7 25.8 31.5 25.8 58.9 0 96.5-58.9 104.2-114.8 110.5 " +
	"9.2 7.9 17 22.9 17 46.4 0 33.7-.3 75.4-.3 83.6 0 6.5 4.6 14.4 17.3 12.1C428.2 457.8 496 " +
	"362.9 496 252 496 113.3 383.5 8 244.8 8zM97.2 352.9c-1.3 1-1 3.3.7 5.2 1.6 1.6 3.9 2.3 5" +
	".2 1 1.3-1 1-3.3-.7-5.2-1.6-1.6-3.9-2.3-5.2-1zm-10.8-8.1c-.7 1.3.3 2.9 2.3 3.9 1.6 1 3.6" +
	".7 4.3-.7.7-1.3-.3-2.9-2.3-3.9-2-.6-3.6-.3-4.3.7zm32.4 35.6c-1.6 1.3-1 4.3 1.3 6.2 2.3 2" +
	".3 5.2 2.6 6.5 1 1.3-1.3.7-4.3-1.3-6.2-2.2-2.3-5.2-2.6-6.5-1zm-11.4-14.7c-1.6 1-1.6 3.6 " +
	"0 5.9 1.6 2.3 4.3 3.3 5.6 2.3 1.6-1.3 1.6-3.9 0-6.2-1.4-2.3-4-3.3-5.6-2z"

// pageLinks is website/data/page-links.json: the sidebar each page carries.
type pageLinks struct {
	External map[string]pageLinkTarget `json:"external"`
	Pages    map[string]pageLinkSpec   `json:"pages"`
	Patterns []pageLinkPattern         `json:"patterns"`
}

// pageLinkTarget is one link to a site outside this repository, named once and
// referenced by key so its URL is stated in one place.
type pageLinkTarget struct {
	Label string `json:"label"`
	URL   string `json:"url"`
	Desc  string `json:"desc"`
}

// pageLinkSpec is the sidebar of one page: an eyebrow and its link groups.
type pageLinkSpec struct {
	Eyebrow string          `json:"eyebrow"`
	Groups  []pageLinkGroup `json:"groups"`
}

// pageLinkPattern gives every page under one prefix the same sidebar, minus the
// pages the exclusion list names.
type pageLinkPattern struct {
	Prefix  string   `json:"prefix"`
	Exclude []string `json:"exclude"`
	pageLinkSpec
}

// pageLinkGroup is one titled block of the sidebar.
type pageLinkGroup struct {
	Title string     `json:"title"`
	Links []pageLink `json:"links"`
}

// pageLink is one sidebar entry. External names a key of pageLinks.External;
// Href names a page of this site. Exactly one of the two is set.
type pageLink struct {
	Href     string `json:"href"`
	External string `json:"external"`
	Label    string `json:"label"`
	Desc     string `json:"desc"`
}

// loadPageLinks reads the sidebar data from one website source tree. A source
// with no such file answers an empty set rather than an error, so a checkout
// that has not written it yet still builds pages with no sidebar.
func loadPageLinks(source string) (pageLinks, error) {
	var links pageLinks
	content, err := os.ReadFile(filepath.Join(source, "data", "page-links.json")) //nolint:gosec // a site build reads the checkout it was pointed at
	if os.IsNotExist(err) {
		return links, nil
	}
	if err != nil {
		return links, err
	}
	if err := json.Unmarshal(content, &links); err != nil {
		return links, fmt.Errorf("read data/page-links.json: %w", err)
	}
	return links, nil
}

// normalizePageKey answers the key page-links.json states one page under: a
// directory path with a trailing slash, and the empty string for the homepage.
func normalizePageKey(key string) string {
	key = strings.TrimPrefix(filepath.ToSlash(key), "/")
	if key == "" || key == "." || key == pageIndexFile {
		return ""
	}
	if trimmed, isIndex := strings.CutSuffix(key, "/"+pageIndexFile); isIndex {
		return trimmed + "/"
	}
	if !strings.HasSuffix(key, "/") && !strings.Contains(path.Base(key), ".") {
		return key + "/"
	}
	return key
}

// specFor answers the sidebar one page carries, by its own key first and by the
// first matching prefix pattern second.
func (links pageLinks) specFor(pageKey string) (pageLinkSpec, bool) {
	key := normalizePageKey(pageKey)
	if spec, named := links.Pages[key]; named {
		return spec, true
	}
	for _, pattern := range links.Patterns {
		prefix := normalizePageKey(pattern.Prefix)
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if pattern.excludes(key) {
			continue
		}
		return pattern.pageLinkSpec, true
	}
	return pageLinkSpec{}, false
}

// excludes reports whether one pattern refuses one page key.
func (pattern pageLinkPattern) excludes(key string) bool {
	for _, excluded := range pattern.Exclude {
		if normalizePageKey(excluded) == key {
			return true
		}
	}
	return false
}

// pageSidebar renders the aside one page carries, or the empty string when it
// carries none.
//
// A link that resolves to the page itself is dropped, a group left with no link
// is dropped, and a sidebar left with no group is empty. The emptiness is
// load-bearing: shell.go reads it to decide the class on <main>, so a sidebar
// that renders nothing must answer nothing rather than an empty aside.
func pageSidebar(root, pageKey string, links pageLinks) string {
	spec, found := links.specFor(pageKey)
	if !found {
		return ""
	}
	current := normalizePageKey(pageKey)
	var groups []string
	for _, group := range spec.Groups {
		var entries strings.Builder
		for _, link := range group.Links {
			if link.External == "" && normalizePageKey(link.Href) == current {
				continue
			}
			entries.WriteString(sidebarLink(root, link, links.External))
		}
		if entries.Len() == 0 {
			continue
		}
		groups = append(groups, "                <section class=\"page-sidebar-group\">\n"+
			"                    <h2>"+html.EscapeString(group.Title)+"</h2>\n"+
			entries.String()+
			"                </section>\n")
	}
	if len(groups) == 0 {
		return ""
	}
	var sidebar strings.Builder
	sidebar.WriteString("            <aside class=\"page-sidebar\" aria-label=\"Related page links\">\n")
	if spec.Eyebrow != "" {
		sidebar.WriteString(`                <p class="page-sidebar-eyebrow">` +
			html.EscapeString(spec.Eyebrow) + "</p>\n")
	}
	sidebar.WriteString("                <nav class=\"page-sidebar-nav\" aria-label=\"Related links\">\n")
	for _, group := range groups {
		sidebar.WriteString(group)
	}
	sidebar.WriteString("                </nav>\n")
	sidebar.WriteString("            </aside>\n")
	return sidebar.String()
}

// sidebarLink renders one sidebar entry. A link that leaves this site opens in
// a new tab and states rel="noopener", whichever way it was declared.
func sidebarLink(root string, link pageLink, external map[string]pageLinkTarget) string {
	href, label, description := link.Href, link.Label, link.Desc
	if link.External != "" {
		target := external[link.External]
		href = target.URL
		if label == "" {
			label = target.Label
		}
		if description == "" {
			description = target.Desc
		}
	} else if !hasURLScheme(href) {
		href = rootedHref(root, href)
	}
	attributes := ""
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		attributes = ` target="_blank" rel="noopener"`
	}
	entry := `                    <a class="page-sidebar-link" href="` +
		html.EscapeString(href) + `"` + attributes + ">\n" +
		`                        <span class="page-sidebar-link-label">` +
		html.EscapeString(label) + "</span>\n"
	if description != "" {
		entry += "                        <small>" + html.EscapeString(description) + "</small>\n"
	}
	return entry + "                    </a>\n"
}

// hasURLScheme reports whether an href already names where it points, so the
// site root prefix must not be put in front of it.
func hasURLScheme(href string) bool {
	return strings.HasPrefix(href, "http://") ||
		strings.HasPrefix(href, "https://") ||
		strings.HasPrefix(href, "mailto:") ||
		strings.HasPrefix(href, "#")
}
