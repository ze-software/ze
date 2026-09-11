// Design: website/AI.md -- every Markdown page of the site is published by one producer
// Detail: docsmanifest.go carries the recovered page registry; doctransform.go the body passes.
// Related: markdown.go renders the body, shell.go wraps it, mirror.go writes the sibling.
package site

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/le/rfc"
)

// The docs producer is registered from here, so a build discovers it through
// the registry rather than through a call the build states by name.
func init() {
	registerProducer(Producer{Name: "docs", Render: renderDocsPages})
}

// sitePage is one Markdown source and the page it publishes.
type sitePage struct {
	// Source is the path of the Markdown source, relative to the checkout.
	Source string
	// Dest is the published page, relative to the artifact, ending index.html.
	Dest string
	// Desc is the meta description, which overrides the source's own.
	Desc string
	// Category is one of the site's seven topic hues, or empty for the
	// neutral heading color. It overrides the source's own.
	Category string
	// Journey is the eyebrow above the page title. An empty field derives the
	// label from DocRel, or from the destination when there is none.
	Journey string
	// DocRel is the source's path relative to docs/, set only for a page the
	// manifest names. It turns on cross-document link rewriting, which
	// resolves a link to another manifest page as a site link and every other
	// link to the code host, and it selects the eyebrow label.
	DocRel string
}

// destinationDirectory answers the page's own directory, relative to the
// artifact root: "guides/anomaly" for "guides/anomaly/index.html".
func (page sitePage) destinationDirectory() string {
	return path.Dir(page.Dest)
}

// root answers the relative path from this page back to the site root.
//
// The depth counts the directory segments above the file rather than the file
// name, so a page at the artifact root answers the empty string and any other
// page answers one "../" for each directory it sits under.
func (page sitePage) root() string {
	return strings.Repeat("../", strings.Count(page.Dest, "/"))
}

// docsSourceRoot is the directory a manifest row's Markdown lives in.
const docsSourceRoot = "docs/"

// rfcStatusSource is the published source whose Markdown the site derives.
const rfcStatusSource = docsSourceRoot + docsSourceRFCStatus

// liveDocSources names each published source whose Markdown is DERIVED, with
// the producer that answers it.
//
// docs/features/rfc-status.md is rendered from the `## Meta` table of each
// rfc/short/ summary and is not tracked. A build reading it off disk would
// publish whatever the last regeneration left behind, and would fail in a
// checkout that holds none. Every other published source is authored, and is
// read from the tree.
var liveDocSources = map[string]func(repository string) (string, error){
	rfcStatusSource: rfc.StatusPage,
}

// docsSourceText answers the Markdown one published source is rendered from.
//
// A producer that answers nothing is refused rather than published: an empty
// page under a live route reads as a page with nothing to say, which is not
// what a failed derivation means.
func docsSourceText(repository, source string) ([]byte, error) {
	produce, derived := liveDocSources[source]
	if !derived {
		return os.ReadFile(filepath.Join(repository, filepath.FromSlash(source))) //nolint:gosec // a site build reads the checkout it was pointed at
	}
	text, err := produce(repository)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", source, err)
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("%s answered no Markdown, so its page would publish empty", source)
	}
	return []byte(text), nil
}

// docsProducerPages answers every page this producer publishes, in a fixed
// order: the manifest first, in its own declared order, then each family.
func docsProducerPages() ([]sitePage, error) {
	pages := make([]sitePage, 0, len(docsManifest)+32)
	for _, row := range docsManifest {
		directory, err := docsDestination(row.Source)
		if err != nil {
			return nil, err
		}
		pages = append(pages, sitePage{
			Source:   docsSourceRoot + row.Source,
			Dest:     directory + "/" + pageIndexFile,
			Desc:     "Ze documentation: " + row.Source,
			Category: row.Category,
			DocRel:   row.Source,
		})
	}
	for _, family := range [][]sitePage{hubPages, useCasePages, labDetailPages, comparePages, qualityPages, oneShotPages} {
		pages = append(pages, family...)
	}
	return pages, nil
}

// renderDocsPages publishes every page of the docs pipeline and answers the
// route of each one.
func renderDocsPages(paths Paths) ([]string, error) {
	pages, err := docsProducerPages()
	if err != nil {
		return nil, err
	}
	renderer, err := newDocsRenderer(paths)
	if err != nil {
		return nil, err
	}
	routes := make([]string, 0, len(pages))
	for _, page := range pages {
		if err := renderer.render(page); err != nil {
			return nil, fmt.Errorf("%s: %w", page.Source, err)
		}
		routes = append(routes, "/"+strings.TrimSuffix(page.Dest, pageIndexFile))
	}
	return routes, nil
}

// docsRenderer holds what every page of one build shares: where it reads and
// writes, the sidebar data, the cross-document link manifest, the number
// tokens, and the terminal demo catalog.
//
// The state sits here rather than in a parameter list because one build reads
// each of these once and every page uses them unchanged.
type docsRenderer struct {
	paths    Paths
	links    pageLinks
	manifest map[string]string
	tokens   numberTokens
	demos    *demoCatalog
}

// newDocsRenderer reads the inputs every page of one build shares.
func newDocsRenderer(paths Paths) (*docsRenderer, error) {
	links, err := loadPageLinks(paths.Source)
	if err != nil {
		return nil, err
	}
	manifest, err := docsLinkManifest()
	if err != nil {
		return nil, err
	}
	tokens, err := loadNumberTokens(paths.Output)
	if err != nil {
		return nil, err
	}
	return &docsRenderer{paths: paths, links: links, manifest: manifest, tokens: tokens, demos: newDemoCatalog(paths)}, nil
}

// render publishes one page: its HTML, its Markdown mirror, and the images it
// references from beside its source.
//
// The passes run in the order the retired renderer ran them, and the order is
// load-bearing twice. The mirror is converted back from the body BEFORE the
// terminal demos expand, so a demo reaches the mirror as Markdown rather than
// as the player's markup. The contents list is spliced LAST, so it lands after
// the hero the journey pass wrote rather than inside it.
func (renderer *docsRenderer) render(page sitePage) error {
	sourcePath := filepath.Join(renderer.paths.Repository, filepath.FromSlash(page.Source))
	source, err := docsSourceText(renderer.paths.Repository, page.Source)
	if err != nil {
		return err
	}
	metadata, body, err := parseFrontMatter(source)
	if err != nil {
		return err
	}

	// The number tokens resolve twice: plain for the Markdown mirror, and as
	// data-ze-stat spans for the HTML, which is what lets a build check a
	// published number against the facts snapshot.
	plain, err := renderer.tokens.substitute(string(body), false)
	if err != nil {
		return err
	}
	marked, err := renderer.tokens.substitute(string(body), true)
	if err != nil {
		return err
	}

	title := pageTitle(metadata, plain)
	description := pageDescription(page, metadata)
	category, err := pageCategory(page, metadata)
	if err != nil {
		return err
	}
	tableColumns, err := tableColumnsEnabled(metadata)
	if err != nil {
		return err
	}

	bodyHTML, headings, err := renderMarkdown([]byte(marked))
	if err != nil {
		return err
	}
	contents := renderDocTOC(headings)
	mirror := plain
	if page.DocRel != "" {
		bodyHTML = rewriteDocLinks(bodyHTML, page.DocRel, renderer.manifest, page.destinationDirectory())
		mirror = rewriteDocLinksMarkdown(mirror, page.DocRel, renderer.manifest, page.destinationDirectory())
	}
	if containsBlockHTML(marked) {
		mirror, err = htmlToMarkdown(bodyHTML, pageCanonicalURL(page.Dest))
		if err != nil {
			return err
		}
	}
	bodyHTML, mirror, demoHead, err := renderer.demos.expand(bodyHTML, mirror, page.root(), page.DocRel)
	if err != nil {
		return err
	}
	bodyHTML = relayoutEvidenceCells(bodyHTML)
	bodyHTML = colorCodeCells(bodyHTML)
	bodyHTML = patchExternalLinkTargets(bodyHTML)
	bodyHTML = wrapJourneyHero(bodyHTML, journeyLabel(page, metadata))
	bodyHTML = insertDocTOC(bodyHTML, contents)

	shell := pageShell{
		Title:       title + " - Ze",
		Description: description,
		Root:        page.root(),
		Path:        page.Dest,
		ExtraHead:   demoHead,
		Sidebar:     pageSidebar(page.root(), page.Dest, renderer.links),
	}
	destination := filepath.Join(renderer.paths.Output, filepath.FromSlash(page.Dest))
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return err
	}
	content := shell.render(sectionOpen(category, tableColumns) + bodyHTML + "\n            </section>\n")
	if err := os.WriteFile(destination, []byte(content), 0o644); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return fmt.Errorf("write %s: %w", destination, err)
	}
	if err := writeMarkdownMirror(destination, mirror); err != nil {
		return err
	}
	return copyReferencedImages(sourcePath, destination, string(body))
}

// sectionOpen answers the element the page body opens with. The category
// colors the title, and a source that turned table columns off says so on the
// element rather than in a class.
func sectionOpen(category string, tableColumns bool) string {
	classes := "md-content reveal"
	if category != "" {
		classes += " cat-" + category
	}
	attribute := ""
	if !tableColumns {
		attribute = ` data-table-columns="off"`
	}
	return `            <section class="` + classes + `"` + attribute + ">\n"
}
