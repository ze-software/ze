Design system

# Style guide.

The palette, components, and content rules this site follows -- so new pages stay consistent without re-deriving them from scratch.

## Design principles

Preserve what helps readers understand Ze, and make it easier to explore.

Thomas approved these principles on 23 October 2026 and gave this instruction:

> Yes, put these rules into the website design guide, then use them to produce a new mockup. Keep the useful information from the current site and show me how the new design presents it better.

Apply these principles to website proposals and implementation. The palette and component sections below describe the blue design Thomas approved on 23 October 2026 after reviewing the homepage and representative inner pages. Later proposals need review against these principles before implementation. The guide changes with the implementation it describes.

Thomas approved implementation on 23 October 2026:

> This looks good. Please apply this style across the GitHub Pages website, keeping all the existing information and functionality.

Thomas approved the management walkthrough layout on 24 October 2026:

> Yes, put the SSH walkthrough on the right and make the existing content more compact on the left. Regenerate the site in ../gh-pages so I can review it.

Thomas refined the card color rule on 24 October 2026:

> It's very nice to have different cards with different colors, but the excessive colors on the card make it look less professional. Perhaps the border shouldn't be colored, only the title should have a color. The color brings differentiation, but it should be a bit more subdued.

Thomas requested a different Routing color on 24 October 2026:

> The color for routing is ugly.

Thomas requested a review of the complete category palette on 24 October 2026:

> Can you review all the category colours together so they are clearly different, but still subdued and consistent across the site?

Thomas kept the current colors and renamed the category on 24 October 2026:

> keep the color for the time being.
> can you change "Deploy and Configure" to "Setup"

Thomas requested neutral gray for Observe on 24 October 2026:

> Still not nice for Observe, a Grey would be better

Thomas approved the resulting palette on 24 October 2026:

> good change

1. ### Preserve useful information

 Start with an inventory of existing capabilities, differentiators, demonstrations, evidence, deployment options, use cases, updates and release status. Give every item an explicit place in the proposal. Explain each move or reduction in detail. Removing useful information requires the owner's agreement.
2. ### Make the product and its value immediately clear

 The opening explains what Ze is, who it serves and why an operator would investigate it. Support a memorable headline with concrete information. A generic slogan does not explain the product's value.
3. ### Explain advantages through their consequences

 Connect each technical capability to what the reader can accomplish. Explain the practical value of shared configuration, external plugins and ExaBGP migration with examples and their limitations. A protocol list establishes breadth, but cannot carry the whole argument.
4. ### Show the product doing useful work

 Use authentic interfaces, readable configuration examples, demonstrations and explanatory diagrams. Each visual makes a clear point and has enough space to show it. A terminal thumbnail that cannot be read provides little evidence. Choose useful preview frames, explain them with captions and provide access to the full recording.
5. ### Create hierarchy without emptying the page

 Let readers understand the proposition at a glance, scan the reasons to explore, then examine the details. Use scale, grouping, alignment, contrast and spacing to distinguish these levels. A long page is useful when each section gives readers a reason to continue.
6. ### Use visual richness deliberately

 Color, gradients, shadows, layered surfaces and illustration can give the site character and depth. Use them consistently to establish emphasis and relationships. Give demonstrations, evidence, feature comparisons and articles compositions suited to their content instead of putting everything into identical cards. Use white space to aid comprehension, rather than merely enlarge the page.
7. ### Give every section a reader purpose and a useful next step

 Each section helps readers understand Ze, assess its fit, inspect evidence or choose how to try it. Keep documentation, migration, appliance, developer and community paths discoverable. Label links with the destination or the action they offer.
8. ### Make credibility visible and specific

 Retain meaningful statistics, interoperability evidence, source links and updates. Explain what the numbers measure and distinguish capabilities from test evidence and production maturity. Keep the pre-release status clear. Place evidence with the claim it supports, rather than using counters as decoration.
9. ### Use a coherent, accessible system

 Share typography, spacing, color roles, elevation, controls and interaction states. Adapt layouts to homepages, references, documentation and articles while preserving Ze's identity. Check keyboard access, contrast, zoom, mobile layouts and reduced motion. Use meaningful headings and document structure.
10. ### Judge each proposal against these principles

 Compare the proposal with the current site using the same content and viewport sizes. Include an information-preservation map and explain material changes. An unfamiliar reader must be able to explain what Ze offers, identify concrete reasons to evaluate it, find evidence and choose a next step. Visual polish alone is insufficient.

### Review before design approval

Record each inventory item's source and current location, its proposed location and any change in detail. Explain why each material change improves the reader's understanding. Identify any proposed removal explicitly so the owner can decide it.

Show the same pages at matching desktop and mobile sizes, with equivalent content available in both versions. Include representative documentation, an article and a dense reference or comparison alongside the homepage. Inspect light and dark themes, keyboard controls, zoom, reduced motion and content without JavaScript. Record which views and interactions the prototype demonstrates and which still need review.

For each section, identify the reader's purpose, the evidence or example it presents and the next useful destination. Check that the reader can understand Ze's offer, reasons to evaluate it, supporting evidence and next steps before judging the visual finish.

## Palette: brand, subjects and status

Logo blue identifies Ze. Seven distinct subject colors connect related content across the website.

`assets/css/00-tokens.css` owns the palette for both themes. `--brand` is the logo blue (#0069b4), `--brand-bright` is cyan (#009ee2), and `--brand-indigo` is indigo (#312782). Navy heroes use `--hero-bg`; body surfaces use `--bg`, `--panel` and `--bg-soft`. Text and controls use semantic tokens so the same content remains readable in either theme. Cyan is an accent on dark surfaces; small links on light surfaces use the darker blue.

Category tokens retain their existing names for authored pages. Cards use `-deep` for readable titles and compact category labels on neutral surfaces. Selection controls can use `-base`, `-chip` and `-tint` to express their category and active state. Despite its name, `-deep` becomes lighter in the dark theme. Do not put white text on a bright accent without checking contrast. Normal text needs at least 4.5:1 in both themes.

The category identities are fixed: Operate is blue, Routing indigo, Services violet, Automate magenta, Observe gray, Secure pink and Setup teal. Broad project updates and cross-topic evidence are neutral. Keep these identities in explicit and automatic dark mode. Status colors remain independent of this subject palette.

Services uses violet and Automate uses magenta. Observe uses neutral gray so it stays distinct from Setup's teal. Keep the approved text colors on neutral panel surfaces; selected controls use subtle fills from the same family. Review the complete palette together in light and dark themes before changing a category color.

Small category labels use `--panel`, including hover and focus states. The approved gray and pink do not reach 4.5:1 on the light `--bg-soft` surface. Use a panel backing instead of darkening the approved text. Check each selected-control fill against its text; the light gray and pink fills stay close to white. Plugin totals and status labels keep their independent neutral and status roles.

| Category | Light | Dark |
| --- | --- | --- |
| Operate | `#0069b4` | `#66bfff` |
| Routing | `#4f46e5` | `#a5b4fc` |
| Services | `#9333ea` | `#d8b4fe` |
| Automate | `#b322a0` | `#f5a1e6` |
| Observe | `#737373` | `#c4c4c4` |
| Secure | `#db2777` | `#f9a8d4` |
| Setup | `#0f766e` | `#5eead4` |

 Operate Routing Services Automate Observe Secure Setup Neutral

**Operate**

### Blue

`--sky-*`

Operator surfaces: SSH CLI, YANG configuration, web workbench, looking glass.

**Routing**

### Indigo

`--lemon-*`

Control plane: BGP engine, static routes, BFD, MRT, RPKI, MPLS signaling.

**Services**

### Violet

`--gold-*`

Network services and dataplane: IPsec, BNG, firewall, interfaces, VPP, DNS.

**Automate**

### Magenta

`--grape-*`

Programmability: plugins, APIs, MCP, ExaBGP migration, fleet management.

**Observe**

### Gray

`--mint-*`

Visibility: telemetry, health and diagnostics.

**Secure**

### Pink

`--pink-*`

Security: SSH authentication, access control, TACACS+, audit trail and PKI store.

**Setup**

### Teal

`--teal-*`

Runtime and lifecycle: performance engineering, appliance and daemon targets, install, Docker, tunables.

**Neutral**

### Cross-topic content

`--neutral-*`

A pale blue-gray page with white surfaces and navy text in the light theme; deep navy surfaces and pale text in the dark theme. Blue links and primary actions identify a destination. Uncategorized cards use the shared neutral surface.

## Color means category

A subject keeps the same color wherever it appears. Color supplements visible category labels; readers do not need color alone to identify a subject.

A `cat-*` class sets the subject color for titles and compact category labels. Borders, surfaces, technology chips, body text and bullet markers stay neutral. The same rules cover homepage, feature, documentation, article, plugin, comparison, FAQ and evidence cards.

1. Choose the existing category that describes the card's primary subject. Keep it in the source record or an explicit semantic classification, not in a styling loop.
2. Use `cat-meta` for cross-topic content with no primary subject. Unclassified cards use the same neutral roles. Do not invent a category to create variety.
3. Pass authored categories through the shared generator vocabulary. Unknown values are errors, not permission to choose an arbitrary color.
4. Use `--acc-deep` for title and category-label text so it stays readable in both themes. Do not assign hues by position, hash or presentation tone, or override them for a page or theme.
5. Every subject card uses the shared `--panel` surface and a thin `--line` border on all sides. Hover borders use `--line-strong`. Keep shadows neutral and restrained. Do not add colored top bars, edge stripes, tinted washes, rings or glows.
6. Category labels use neutral `--panel` backgrounds. Keep technology and mode chips neutral; reserve stronger control color for selected filters and visible keyboard focus.
7. Keep status independent. Success, warning and failure use `--green`, `--amber` and `--danger-deep`, with words explaining the result. Neutral evidence measures make no verdict. Maturity keeps its explicit labels and border treatment.

**Routing**

### A well-formed card

`RFC 0000` `Tech`

- Subject color in **titles and labels**
- Chips name **technologies**
- Bullets state **facts**

**Setup**

### Mode chips

`Daemon only` `Example`

Features run in both daemon and appliance modes by default; a solid mode chip flags the exception. No "Both" pill on every card.

## Surfaces and elevation

Borders group content; restrained shadows distinguish interactive and raised surfaces.

Cards use `var(--panel)`, a thin `var(--line)` border and the shared radius. `--clay` remains the compatibility token for the subtle card shadow. `--shadow-lifted` marks a linked card on hover, and `--shadow-panel` separates a product image or dropdown from its surroundings. Informational cards remain still. Keyboard focus remains visible without depending on hover or color alone.

### Raised surface

The shared card surface uses a fine border and a small shadow to separate it from the page.

### Inset surface

A muted surface groups supporting information without adding another raised card.

## Page and card ornaments

Navy heroes, restrained blue light and distinct content layouts.

Page heroes and dispatch cards share one ornament language: subtle blue gradients behind the content, thin concentric circle lines in an open corner, and optional masked grid lines for large hero surfaces. Keep ornaments low contrast and behind the text. Avoid filled balls, opaque blobs, heavy badges, or one-off shapes that fight the content.

Wide product comparison tables must keep the existing page filter and add product column controls so readers can hide projects they are not comparing. Keep those controls visible, keyboard-accessible, and honest: hiding a column changes the view only, never the source evidence or generated text.

Use `.journey-hero` for the shared navy page opening. Two-choice dispatch pages, including Compare, use `.compare-dispatch` cards with subject category classes and restrained line decoration.

### Hero pages

A navy hero introduces the page with a clear title and useful context. Interior-page hero backgrounds reach the viewport edges while their content follows the shared page width. The homepage hero and proof panels stay within that width, with inner padding and rounded edges. Leave a gap before Latest news and distribute the product links equally across their row, using fewer columns on smaller screens.

The homepage hero uses the shared `--hero-panel-surface` navy gradient in both themes.

The homepage's four starting actions fill the space beside the release callout in equal columns. Center each title and its supporting text within its column. Align the actions and release notice at the top, with the notice text at the mascot's top edge. At widths of 850px or less, use two equal columns and keep the text centered.

Separate standard homepage sections with spacing and no horizontal rules, and omit the final footer separator on the homepage. Use 2rem of top and bottom padding, reduced to 1.5rem at widths of 560px or less, with no added top margin. Section titles, including the proof and Try safely titles, share a 2rem font size, 700 weight, 1.25 line height and 0.5rem bottom margin. Reduce their font size to 1.65rem at widths of 560px or less. Stacked section titles and introductions have a 0.5rem gap. Keep the compact Latest news title and the hero and proof panels' separate spacing.

Keep homepage section containers, including Latest news, shadowless. Cards within them retain their component shadows.

Proof cards share heading, detail and evidence rows. Align every evidence divider and the first line of its text to the same row boundary, even when another card's evidence wraps. Preserve the complete evidence wording.

The management section uses two equal columns: compact explanation, interface map and documentation links on the left; a real SSH recording on the right. Keep readable step labels beside the player, show its SSH introduction as the preview, start there on demand, and retain full-screen controls, the full session and the transcript. Stack the columns at widths of 960px or less.

### Dispatch cards

Use the subject color on each choice's title and compact category label. Keep the panel and border neutral, without decorative colored shapes.

### Generated pages

Start from existing components before adding CSS. If a page needs a new surface, document the rule here when the component ships.

## Component patterns to reuse

Don't invent new components for new pages -- these cover almost everything.

### Guide and article layouts

The navy `.journey-hero` introduces a guide. `.doc-layout` keeps its outline beside the complete source content. Articles use `.blog-article-shell` for the opening and `.blog-reading-layout` for a narrower reading column. On small screens, native outline disclosures start collapsed after JavaScript loads and related links follow the article outline. Without JavaScript, outlines remain open. Readers can expand or collapse them at every screen size.

### Catalog controls

Feature search combines category and maturity filters. The command reference combines text and group filters, preserves suggestions and command links, and offers expand, collapse and column controls. Reset restores the complete inventory. Without JavaScript, every card and command remains available, including category counts, arguments and pipe details.

### Product screenshots

The homepage groups authentic interfaces with captions, provenance and full-size links. Native buttons select the interface and expose their pressed state. Keep one shared media ratio and reserve the tallest caption so switching views never moves the hero. Inactive panels are invisible and inert. All panels remain visible when JavaScript is unavailable. Distinguish Web configuration from the CLI editor; the web tab carries a genuine recording with controls and a transcript and pauses when another tab is selected. Static CLI captures contain no player controls or play button.

### .status-panel / .status-table

Two-column fact panel: prose on the left, labeled rows on the right. Used for lab pages' "Proves / Requires / Source" facts.

### .terminal-panel

Terminal (exact commands and familiar traffic-light dots) + terminal-note (prerequisites, links) side by side. Used on every lab page and the homepage Try section.

### .cards / .card + cat-*

The feature-grid unit. A cat-* class sets the hue, .cat names the category, .chip pills tag technologies, .chip.mode flags Daemon/Appliance exceptions.

### .legend

Category filter buttons carrying the same cat-* classes as the cards they filter. One active category at a time; pressing again clears.

### .section-note

Full-width prose under a section head for anything longer than a sentence or two. The head's side text stays short so it never pushes the section's content down.

### Maturity tiers on cards

Shipped cards have a solid border. .card.experimental adds a dashed border and an "Experimental" status chip. Remaining release work items are listed on the [generated roadmap](https://ze-software.net/project/roadmap/), an inventory preview pending owner classification.

### .doc-diagram

Full-width documentation figure with a shared surface and border, caption, and horizontal overflow that keeps detailed diagrams readable on narrow screens. Link the image to its standalone asset and keep a text equivalent below it.

### .md-content

Wraps Markdown rendered by the native `./le site build` pipeline: headings, tables, blockquotes, and code all match the rest of the site.

## Navigation rules

The menu and related links come from shared data. The footer retains the license and publication stamp.

Use navigation to explain where the reader should go next, not as a dump of every page. The top mega-menu is the global map from `data/nav.json`. The right page menu is the local guide from `data/page-links.json`. The footer is the shared license line and publication stamp from `internal/le/site/footer.go`'s `footerHTML`. Do not hand-code equivalent link lists in page bodies.

### Top mega-menu

Every multi-column dropdown must have a label at the top of each column. Group the top menu by reader job: Start, Evaluate, Docs, Examples, Reference, Project. Do not let a dropdown clip outside the viewport.

Leave a 0.75rem gap below the trigger so an open panel does not crowd its label. On desktop, a transparent bridge preserves hover access across that gap. In the stacked mobile menu, keep the same spacing and remove the bridge.

### Right menu

Use `.page-sidebar` for nearby choices, related evidence, and next steps. Add entries to `data/page-links.json`. The `pageSidebar` function in `internal/le/site/nav.go` renders the menu, and the shared stylesheet controls its mobile layout.

### Reader path first

The right menu ranks what the reader likely wants next before external references. Research, debugging, comparison, and journey paths come before project homepages or vendor docs.

### External links

Every external link opens with `target="_blank"` and `rel="noopener"`. Never use a named target. Keep each external destination once in `data/page-links.json` instead of duplicating it in page bodies.

### Easy scanning

Prefer short labels, one-line descriptions, grouped links, and search boxes over long index pages. If a page becomes a catalog, move details to per-item pages and keep the index as a route.

## Source and evidence rules

Claims, counts, navigation, and AI mirrors must be generated from source.

### Evidence for claims

Every claim about a feature, protocol, lab, benchmark, command, dependency, plugin, or quality gate must trace to a source file, Markdown source, JSON data file, script, generated binary output, or external reference.

### Generated data first

Do not hand-maintain lists that can come from Markdown or structured data. Use website `data/*.json`, repository `docs/*.md`, `go.mod`, registry extraction, YANG extraction, or output from the daemon compiled by the site build.

### Markdown mirrors

Every published page needs an `index.md` sibling for AI and text readers. Render it from the same Markdown or data as the HTML; only hand-authored pages may use the build step's HTML-to-Markdown extraction.

### `llms.txt`

`llms.txt` is generated from `data/nav.json` and live counts. Each entry links to the Markdown URL first, then the human web page. Never edit it by hand.

## Content rules

Agreed while building this site. Apply them to anything added later.

### Honesty about shortcomings

State where Ze is behind plainly and up front (see Compare) -- not buried in a table, not omitted.

### No fabrication

Every claim about what a feature or lab does traces to a real source file, doc, or script. If it can't be cited, don't state it as fact.

### No naming other network OSes

The BGP-daemon comparison table is an established, separate category. Beyond that, don't name competing full network operating systems in Ze's own copy.

### Daemon / Appliance labeling

Features run in both modes unless a .chip.mode says otherwise. Labs cards still tag modes explicitly with card-label pills, since labs are mode-specific.

### Write little in section heads

The text beside a title is one or two sentences, max. Longer verbiage moves to a .section-note under the head.

### Dedicated pages over a single page

New substantial content gets its own page and a top-nav or right-menu path, not another section bolted onto index.html.

### Markdown-sourced where it can be

Content that changes often, such as Compare, lives in a Markdown file rendered by `./le site build`, so updating it does not require hand-editing HTML tables.
