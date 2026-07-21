Design system

# Style guide.

The palette, components, and content rules this site follows -- so new pages stay consistent without re-deriving them from scratch.

## Palette: vivid candy claymation

Seven candy hues on a calm blush base.

Each hue ships four tones in `assets/site.css?v=079eb640b0`: `-base` (the vivid candy color), `-deep` (text-safe dark), `-chip` (pill background), and `-tint` (card surface), plus a `-glow` for hue-tinted shadows. Never mix tones across hues on one component.

**Operate**

### Sky

`--sky-*`

Operator surfaces: SSH CLI, YANG configuration, web workbench, looking glass.

**Routing**

### Tangerine

`--tangerine-*`

Control plane: BGP engine, static routes, BFD, MRT, MPLS signaling.

**Services**

### Teal

`--teal-*`

Network services and data plane: VPN, BNG, firewall, interfaces, VPP, DNS.

**Automate**

### Grape

`--grape-*`

Programmability: plugins, APIs, MCP, ExaBGP migration, fleet management.

**Observe**

### Mint

`--mint-*`

Visibility and evidence: telemetry, health, diagnostics, testing proof.

**Secure**

### Pink

`--pink-*`

Security: SSH and RBAC, RPKI, TACACS+, audit trail, PKI store.

**Platform**

### Lemon

`--lemon-*`

Packaging and lifecycle: appliance and daemon targets, install, Docker, tunables.

### Neutrals and brand

`--bg / --text / --purple`

A flat dimmed-blush page background, deep plum text, grape-violet headings. Surfaces stay lighter than the page so cards float. Cards without a category stay blush-white.

## Color means category

On feature cards, color is information, never decoration.

A `cat-*` class on the card sets one accent hue for everything on it: the category chip, the title, the tech chips, the bullet markers, the bold words, and the hover glow. Do not color cards by position (`nth-child` cycling is banned for cards) and do not mix hues within one card. Decorative variety is fine only where order carries no meaning -- the homepage proof strip and audience trio.

**Routing**

### A well-formed card

`RFC 0000` `Tech`

- One hue, **every element**
- Chips name **technologies**
- Bullets state **facts**

**Platform**

### Mode chips

`Daemon only` `Example`

Features run in both daemon and appliance modes by default; a solid mode chip flags the exception. No "Both" pill on every card.

## The clay look

Molded, not just rounded with a drop shadow.

Every card-like surface (`.card`, `.button`, `.proof`, `.audience-card`, `.bird-card`, `.status-panel`, `.terminal-note`, `.closing`, the `.eyebrow` badge) carries `box-shadow: var(--clay)` plus a 2px white "sugar coat" border: an inset top highlight, a soft inset bottom shade, and a diffuse outer shadow. Hovers lift with a bouncy cubic-bezier and a hue-tinted glow. Add all of it to any new card-like component; don't invent a one-off shadow.

### With clay

This card. Notice the white coat, soft top highlight, and bottom depth even before hover.

### Without clay

Flat by comparison -- a plain bordered rectangle, no material feel.

## Page and card ornaments

Light linework, candy washes, and clay depth.

Page heroes and dispatch cards share one ornament language: translucent candy radial washes behind the content, thin concentric circle lines in an open corner, and optional masked grid lines for large hero surfaces. Keep ornaments low contrast and behind the text. Avoid filled balls, opaque blobs, heavy badges, or one-off shapes that fight the content.

Wide product comparison tables must keep the existing page filter and add product column controls so readers can hide projects they are not comparing. Keep those controls visible, keyboard-accessible, and honest: hiding a column changes the view only, never the source evidence or generated text.

Reuse `.journey-hero::before` for masked grid texture and `.journey-hero::after` for circle line treatment. Two-choice dispatch pages, including Compare, use `.compare-dispatch` cards with page-specific accent classes and the same concentric ring language.

### Hero pages

Use a soft candy wash, a masked grid, one ring ornament, and clay shadow. The content owns the foreground.

### Dispatch cards

Use one accent palette per choice and a light circle-line ornament. Do not use filled decorative balls.

### Generated pages

Start from existing components before adding CSS. If a page needs a new surface, document the rule here when the component ships.

## Component patterns to reuse

Don't invent new components for new pages -- these cover almost everything.

### .status-panel / .status-table

Two-column fact panel: prose on the left, labeled rows on the right. Used for lab pages' "Proves / Requires / Source" facts.

### .terminal-panel

Terminal (exact commands, candy traffic-light dots) + terminal-note (prerequisites, links) side by side. Used on every lab page and the homepage Try section.

### .cards / .card + cat-*

The feature-grid unit. A cat-* class sets the hue, .cat names the category, .chip pills tag technologies, .chip.mode flags Daemon/Appliance exceptions.

### .legend

Category filter buttons carrying the same cat-* classes as the cards they filter. One active category at a time; pressing again clears.

### .section-note

Full-width prose under a section head for anything longer than a sentence or two. The head's side text stays short so it never pushes the section's content down.

### Maturity tiers on cards

Shipped cards are solid clay. .card.experimental adds a dashed border and an "Experimental" status chip. .card.aspiration is a flat dashed blueprint whose title links to its pending spec in the main repo's plan/ directory.

### .doc-diagram

Full-width documentation figure with a clay frame, caption, and horizontal overflow that keeps detailed diagrams readable on narrow screens. Link the image to its standalone asset and keep a text equivalent below it.

### .md-content

Wraps markdown rendered by tools/render-doc.py -- headings, tables, blockquotes, code, all styled to match the rest of the site.

## Navigation rules

Top menu and right menu come from shared data; footer is a license line.

Use navigation to explain where the reader should go next, not as a dump of every page. The top mega-menu is the global map from `data/nav.json`. The right page menu is the local guide from `data/page-links.json`. The footer is a shared license line from `sitelib.footer_html`. Do not hand-code equivalent link lists in page bodies.

### Top mega-menu

Every multi-column dropdown must have a label at the top of each column. Group the top menu by reader job: Start, Evaluate, Docs, Examples, Reference, Project. Do not let a dropdown clip outside the viewport.

### Right menu

Use `.page-sidebar` for nearby choices, related evidence, and next steps. Add entries to `data/page-links.json`; `sitelib.patch_page_sidebar` injects the menu and the mobile layout.

### Reader path first

The right menu ranks what the reader likely wants next before external references. Research, debugging, comparison, and journey paths come before project homepages or vendor docs.

### External links

Every external link opens with `target="_blank"` and `rel="noopener"`. Never use a named target. Do not add duplicate links to the same external page; validate with `tools/check-page-links.py`.

### Easy scanning

Prefer short labels, one-line descriptions, grouped links, and search boxes over long index pages. If a page becomes a catalogue, move details to per-item pages and keep the index as a route.

## Source and evidence rules

Claims, counts, navigation, and AI mirrors must be generated from source.

### Evidence for claims

Every claim about a feature, protocol, lab, benchmark, command, dependency, plugin, or quality gate must trace to a source file, Markdown source, JSON data file, script, generated binary output, or external reference.

### Generated data first

Do not hand-maintain lists that can come from Markdown or structured data. Use `data/*.json`, `../main/docs/*.md`, `go.mod`, registry extraction, YANG extraction, or live `bin/ze` output.

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

Content that changes often (like Compare) lives in a .md file rendered by tools/render-doc.py, so updating it doesn't mean hand-editing HTML tables.
