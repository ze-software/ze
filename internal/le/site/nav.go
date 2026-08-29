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
	{"features/", "Features"},
	{"compare/", "Compare"},
	{"project/roadmap/", "Roadmap"},
	{"project/changes/", "Changes"},
	{"faq/", "FAQ"},
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
