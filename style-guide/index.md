# Style guide.

The palette, components, and content rules this site follows -- so new pages stay consistent without re-deriving them from scratch.

## Palette: vivid candy claymation

Seven candy hues on a calm blush base.

Each hue ships four tones in `assets/site.css?v=e42f60ed09`: `-base` (the vivid candy color), `-deep` (text-safe dark), `-chip` (pill background), and `-tint` (card surface), plus a `-glow` for hue-tinted shadows. Never mix tones across hues on one component.

 Operate

### Sky

 `--sky-*`

Operator surfaces: SSH CLI, YANG configuration, web workbench, looking glass.

 Routing

### Tangerine

 `--tangerine-*`

Control plane: BGP engine, static routes, BFD, MRT, MPLS signaling.

 Services

### Teal

 `--teal-*`

Network services and data plane: VPN, BNG, firewall, interfaces, VPP, DNS.

 Automate

### Grape

 `--grape-*`

Programmability: plugins, APIs, MCP, ExaBGP migration, fleet management.

 Observe

### Mint

 `--mint-*`

Visibility and evidence: telemetry, health, diagnostics, testing proof.

 Secure

### Pink

 `--pink-*`

Security: SSH and RBAC, RPKI, TACACS+, audit trail, PKI store.

 Platform

### Lemon

 `--lemon-*`

Packaging and lifecycle: appliance and daemon targets, install, Docker, tunables.

### Neutrals and brand

 `--bg / --text / --purple`

A flat dimmed-blush page background, deep plum text, grape-violet headings. Surfaces stay lighter than the page so cards float. Cards without a category stay blush-white.

## Color means category

On feature cards, color is information, never decoration.

A `cat-*` class on the card sets one accent hue for everything on it: the category chip, the title, the tech chips, the bullet markers, the bold words, and the hover glow. Do not color cards by position (`nth-child` cycling is banned for cards) and do not mix hues within one card. Decorative variety is fine only where order carries no meaning -- the homepage proof strip and audience trio.

 Routing

### A well-formed card

 `RFC 0000` `Tech`

 - One hue, **every element**
 - Chips name **technologies**
 - Bullets state **facts**

 Platform

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

### .md-content

Wraps markdown rendered by tools/render-doc.py -- headings, tables, blockquotes, code, all styled to match the rest of the site.

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

New substantial content gets its own page and a nav/footer link, not another section bolted onto index.html.

### Markdown-sourced where it can be

Content that changes often (like Compare) lives in a .md file rendered by tools/render-doc.py, so updating it doesn't mean hand-editing HTML tables.
