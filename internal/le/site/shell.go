// Design: website/AI.md -- one page shell wraps every published page
// Detail: nav.go renders the header mount and the sidebar; footer.go the stamp.
package site

import (
	"html"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The site's own addresses and the assets every page links.
const (
	siteBase       = "https://ze-software.net/"
	socialImage    = siteBase + "assets/social-card.png"
	fontStylesheet = "assets/vendor/fonts/fonts.css"
)

// themeBootstrap sets data-theme before the first paint, so a reader who chose
// the dark theme never sees a light flash. It runs inline in the head for that
// reason and must not move into the deferred script.
const themeBootstrap = `        <script id="theme-bootstrap">(function(){var t="light";try{var s=localStorage.getItem("ze-theme");t=s==="dark"||s==="light"?s:window.matchMedia&&window.matchMedia("(prefers-color-scheme: dark)").matches?"dark":"light"}catch(e){t=window.matchMedia&&window.matchMedia("(prefers-color-scheme: dark)").matches?"dark":"light"}document.documentElement.setAttribute("data-theme",t)})();</script>`

// structuredData is the schema.org graph every page carries.
//
// It is written out rather than marshaled from a Go value for two reasons: the
// key order is part of what is published and a Go map states no order, and the
// encoder escapes "<" into a unicode escape unless SetEscapeHTML is turned off.
// The content is the same on every page, so there is nothing here to compute.
const structuredData = `        <script type="application/ld+json">{"@context":"https://schema.org","@graph":[{"@type":"WebSite","name":"Ze","url":"https://ze-software.net/","description":"Open-source configuration and protocol engine for Linux routing, with YANG-modeled plugins, operator interfaces, telemetry, and MCP.","inLanguage":"en"},{"@type":"SoftwareSourceCode","name":"Ze","description":"Open-source configuration and protocol engine for Linux routing, used to build a plugin-based network operating system.","codeRepository":"https://github.com/ze-software/ze","license":"https://www.gnu.org/licenses/agpl-3.0.en.html","programmingLanguage":"Go","runtimePlatform":"Linux","applicationCategory":"Network operating system","isAccessibleForFree":true}]}</script>` + "\n"

// pageShell is the chrome around one published page, held as ONE value.
//
// The retired renderer split it into a head call and a foot call joined by a
// module global: the head computed the page sidebar, stashed it, and the foot
// read it back. Two calls in Go would let a page open <main> with the
// has-page-sidebar class and then write no sidebar, or the reverse, with
// nothing to notice. One value decides both, so they cannot disagree.
//
// Body is not a field. A producer renders its own body and passes it to render,
// which splices it at column zero into an indentation-sensitive shell: a real
// page therefore mixes indented chrome and an unindented body, and re-indenting
// the body to look tidy would change what a <pre> block says.
type pageShell struct {
	// Title is the whole <title>, suffix included. The shell adds nothing.
	Title string
	// Description is the meta description and the default social description.
	Description string
	// SocialTitle and SocialDescription override Title and Description in the
	// Open Graph and Twitter tags. An empty field takes the page's own.
	SocialTitle       string
	SocialDescription string
	// Root is the relative prefix that reaches the site root from this page,
	// empty for the site root itself.
	Root string
	// Path is this page's artifact-relative POSIX path, which the canonical
	// URL is derived from.
	Path string
	// ExtraHead is markup a producer needs in the head, already indented and
	// ending with a newline. A page that needs none leaves it empty.
	ExtraHead string
	// Sidebar is the rendered page sidebar, or the empty string when the page
	// carries none. nav.pageSidebar answers the empty string for a sidebar
	// whose every group emptied, which is what makes the class below correct.
	Sidebar string
	// Wide widens <main> on a page that carries no sidebar. A sidebar wins.
	Wide bool
}

// pageCanonicalURL answers the absolute URL of one published page from its
// artifact-relative POSIX path: the site root for the homepage, the directory
// URL for any other index.html, and the file URL for anything else.
func pageCanonicalURL(relative string) string {
	relative = filepath.ToSlash(relative)
	if relative == pageIndexFile {
		return siteBase
	}
	if directory, isIndex := strings.CutSuffix(relative, pageIndexFile); isIndex {
		return siteBase + directory
	}
	return siteBase + relative
}

// mainClass answers the class attribute <main> opens with, including its
// leading space, or the empty string when it takes no class.
//
// A sidebar wins over a wide request, because a page cannot be both. This is
// the one decision the retired renderer could get wrong, and the sidebar it
// reads is the same string render writes.
func (shell pageShell) mainClass() string {
	if shell.Sidebar != "" {
		return ` class="has-page-sidebar"`
	}
	if shell.Wide {
		return ` class="site-main-wide"`
	}
	return ""
}

// socialTitle and socialDescription answer what the Open Graph and Twitter tags
// state, falling back to the page's own title and description.
func (shell pageShell) socialTitle() string {
	if shell.SocialTitle != "" {
		return shell.SocialTitle
	}
	return shell.Title
}

func (shell pageShell) socialDescription() string {
	if shell.SocialDescription != "" {
		return shell.SocialDescription
	}
	return shell.Description
}

// render answers the whole page: the head, the chrome, the body spliced in at
// column zero, and the footer carrying this build's publication stamp.
//
// The stamp is provisional. stampArtifact rewrites the footer of every public
// page after the producers run, and carryPublicationStamps then gives an
// unchanged page its previous stamp back, so the line reads as when the page
// last changed rather than as when a build last ran.
func (shell pageShell) render(body string) string {
	canonical := html.EscapeString(pageCanonicalURL(shell.Path))
	root := html.EscapeString(shell.Root)

	var page textbuf.Buffer
	page.Reset().Str("<!doctype html>\n<html lang=\"en\">\n    <head>\n")
	page.Str("        <meta charset=\"utf-8\" />\n")
	page.Str("        <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\" />\n")
	page.Str(themeBootstrap).Byte('\n')
	page.Str("        <title>").Str(html.EscapeString(shell.Title)).Str("</title>\n")
	page.Str(metaTag("name", "description", shell.Description))
	page.Str(metaTag("property", "og:title", shell.socialTitle()))
	page.Str(metaTag("property", "og:description", shell.socialDescription()))
	page.Str(metaTag("property", "og:type", "website"))
	page.Str(metaTag("property", "og:image", socialImage))
	page.Str(metaTag("property", "og:image:width", "1200"))
	page.Str(metaTag("property", "og:image:height", "630"))
	page.Str(metaTag("property", "og:image:alt", "Ze, an open-source configuration and protocol engine"))
	page.Str(metaTag("name", "twitter:card", "summary_large_image"))
	page.Str(metaTag("name", "twitter:title", shell.socialTitle()))
	page.Str(metaTag("name", "twitter:description", shell.socialDescription()))
	page.Str(metaTag("name", "twitter:image", socialImage))
	page.Str("        <link rel=\"icon\" href=\"").Str(root).Str("assets/ze.svg\" type=\"image/svg+xml\" />\n")
	page.Str("        <link rel=\"stylesheet\" href=\"").Str(root).Str(fontStylesheet).Str("\" />\n")

	// The canonical link and og:url sit immediately before the stylesheet
	// link. The retired renderer emitted neither, and a later pass anchored
	// both on the site.css link and inserted them before it, so this is where
	// every published page carries them.
	page.Str("        <link rel=\"canonical\" href=\"").Str(canonical).Str("\" />\n")
	page.Str("        <meta property=\"og:url\" content=\"").Str(canonical).Str("\" />\n")
	page.Str("        <link rel=\"stylesheet\" href=\"").Str(root).Str("assets/site.css\" />\n")
	page.Str(structuredData)
	page.Str(shell.ExtraHead)
	page.Str("    </head>\n    <body>\n")
	page.Str("        <a class=\"skip-link\" href=\"#top\">Skip to main content</a>\n")
	page.Str(headerMount(shell.Root)).Byte('\n')
	page.Str("\n        <main id=\"top\"").Str(shell.mainClass()).Str(" tabindex=\"-1\">\n")

	page.Str(body)

	page.Str(shell.Sidebar).Str("        </main>\n")
	page.Str("\n        <script src=\"").Str(root).Str("assets/site.js\" defer></script>\n\n")
	page.Str(footerHTML(shell.Root, buildClock())).Byte('\n')
	page.Str("    </body>\n</html>\n")
	return page.String()
}

// metaTag renders one meta element of the head. The attribute naming the tag is
// "name" or "property", which is why it is a parameter rather than a constant.
func metaTag(attribute, name, content string) string {
	return "        <meta " + attribute + "=\"" + name + "\" content=\"" +
		html.EscapeString(content) + "\" />\n"
}
