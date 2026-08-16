#!/usr/bin/env -S uv run python3
"""Render index.html from a template plus data/audience.json.

Usage:
    tools/render-index.py

The hero, outcome strip, proof block, and status panel are bespoke homepage
copy and stay as a literal template here (there's nothing repeated to model as
data). The "Run paths" and "Use cases worth trying now" card grids are data
(data/audience.json) -- add or edit an audience card there instead of
hand-editing HTML.

The "Latest news" band is two generated slots (the newest blog article and
the newest weekly update) plus one freeform note from data/whats-new.json. It
is deliberately one shallow row: the proof cards under it have to stay compact
on a laptop-height viewport.
"""

import html
import json
import pathlib

import models
import sitelib
import sitefacts

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
DATA = GH_PAGES / "data" / "audience.json"
CHANGES_DATA = GH_PAGES / "data" / "changes.json"
WHATS_NEW_DATA = GH_PAGES / "data" / "whats-new.json"
DEST = GH_PAGES / "index.html"

# Homepage proof-strip numbers are regenerated from ../main whenever the
# homepage is rendered. The fallback is only for a damaged or unavailable
# source tree.
PROOF_STATS_FALLBACK = {
    "unit_tests": "17,300+",
    "e2e_tests": "1,300+",
    "fuzz_targets": "65+",
    "interop_targets": "7",
}


def proof_stats():
    try:
        facts = sitefacts.write_facts()
    except (OSError, KeyError, ValueError):
        return PROOF_STATS_FALLBACK
    return {
        "unit_tests": facts["tests"]["unit_display"],
        "e2e_tests": facts["tests"]["e2e_display"],
        "fuzz_targets": facts["tests"]["fuzz_display"],
        "interop_targets": facts["interop"]["target_display"],
    }


def esc(value):
    return html.escape(str(value), quote=True)


def render_chip_row(card):
    chips = card.get("chips", [])
    if not chips:
        return ""

    out = ['                        <div class="chips">']
    out.extend(
        '                            <span class="chip">%s</span>' % esc(chip)
        for chip in chips
    )
    out.append("                        </div>")
    return "\n".join(out) + "\n"


def render_audience_card(card):
    category = card.get("category", "platform")
    tone = card.get("tone")
    label = card.get("label", category.capitalize())
    link = card.get("link")
    title = esc(card["title"])
    if link:
        title = '<a href="%s">%s</a>' % (esc(link["href"]), title)
        cta = (
            '                        <span class="audience-card-cta">%s '
            "<small>%s</small></span>\n"
            % (esc(link["label"]), esc(link["sublabel"]))
        )
    else:
        cta = ""

    return """                    <article class="card audience-card cat-{category}{tone_class}">
                        <span class="cat">{label}</span>
                        <h3>{title}</h3>
                        <p>
                            {body}
                        </p>
{chips}{cta}                    </article>""".format(
        category=esc(category),
        tone_class=(" tone-%s" % esc(tone)) if tone else "",
        label=esc(label),
        title=title,
        body=esc(card["body"]),
        chips=render_chip_row(card),
        cta=cta,
    )


BLOG_TEASER_TONES = [
    "sky",
    "pink",
    "grape",
]


def pick_home_update_topics(topics, limit=4):
    picked = []
    seen_categories = set()
    for topic in topics:
        category = topic.get("category")
        if category in seen_categories:
            continue
        picked.append(topic)
        seen_categories.add(category)
        if len(picked) == limit:
            return picked
    for topic in topics:
        if topic in picked:
            continue
        picked.append(topic)
        if len(picked) == limit:
            break
    return picked


def render_home_update_tags(topics):
    picked = pick_home_update_topics(topics)
    if not picked:
        return ""
    out = ['                        <div class="home-update-tags">']
    for topic in picked:
        out.append(
            '                            <span class="home-update-tag cat-%s">%s</span>'
            % (esc(topic.get("category", "meta")), esc(topic.get("label", "")))
        )
    out.append("                        </div>")
    return "\n".join(out) + "\n"


def change_topics_by_slug():
    weeks = json.loads(CHANGES_DATA.read_text())
    return {week["slug"]: week.get("topics", []) for week in weeks}


def render_blog_teaser_card(post, i, topics):
    tone = BLOG_TEASER_TONES[i % len(BLOG_TEASER_TONES)]
    parts = [
        '                    <article class="card card-post home-update-card tone-%s">'
        % tone
    ]
    parts.append('                        <div class="home-update-head">')
    parts.append('                            <span class="cat">Update</span>')
    parts.append(
        '                            <span class="home-update-number">%02d</span>'
        % (i + 1)
    )
    parts.append("                        </div>")
    parts.append(
        '                        <p class="home-update-date">Week of %s</p>'
        % esc(post["slug"])
    )
    parts.append(
        '                        <h3><a href="changes/%s/">%s</a></h3>'
        % (esc(post["slug"]), esc(post["intro"] or "Weekly update"))
    )
    parts.append(render_home_update_tags(topics).rstrip())
    parts.append("                    </article>")
    return "\n".join(part for part in parts if part)


# The band's own slots: the newest article and the newest weekly update are
# generated, the third is whatever data/whats-new.json says. One line of
# summary each -- anything taller pushes the proof strip off a laptop screen.
WHATS_NEW_SUMMARY_CHARS = 108


def clip(text, limit=WHATS_NEW_SUMMARY_CHARS):
    """One line of summary, cut on a word boundary."""
    text = " ".join((text or "").split())
    if len(text) <= limit:
        return text
    return text[:limit].rsplit(" ", 1)[0].rstrip(",.;:") + "…"


def render_whats_new_item(label, tone, href, title, summary):
    return """                <article class="whats-new-item tone-{tone}">
                    <span class="whats-new-label">{label}</span>
                    <h3><a href="{href}">{title}</a></h3>
                    <p>{summary}</p>
                </article>""".format(
        tone=esc(tone),
        label=esc(label),
        href=esc(href),
        title=esc(title),
        summary=esc(summary),
    )


def render_whats_new(data):
    """The band above the proof strip: latest article, latest weekly update,
    and the freeform note. A missing article or note simply drops its slot,
    so the homepage still renders on a tree with no blog posts yet."""
    items = []

    articles = sitelib.blog_articles()
    if articles:
        article = articles[0]
        items.append(
            render_whats_new_item(
                "Engineering note",
                "grape",
                "blog/%s/" % article["slug"],
                article["title"],
                clip(article["description"]),
            )
        )

    weeks = sitelib.latest_blog_posts(1)
    if weeks:
        week = weeks[0]
        items.append(
            render_whats_new_item(
                "Recently shipped",
                "sky",
                "changes/%s/" % week["slug"],
                "Week of %s" % week["slug"],
                clip(week["intro"] or "What shipped that week."),
            )
        )

    note = data.get("note")
    if note:
        link = note.get("link")
        items.append(
            render_whats_new_item(
                note["label"],
                note.get("tone", "sky"),
                link["href"] if link else data["link"]["href"],
                note["title"],
                clip(note["body"]),
            )
        )

    if not items:
        return ""

    return """            <section class="whats-new reveal" aria-labelledby="whats-new-title">
                <div class="whats-new-head">
                    <h2 id="whats-new-title">{title}</h2>
                    <a href="{link_href}">{link_label}</a>
                </div>
{items}
            </section>
""".format(
        title=esc(data["title"]),
        link_href=esc(data["link"]["href"]),
        link_label=esc(data["link"]["label"]),
        items="\n".join(items),
    )


BODY = """            <section class="hero" aria-labelledby="hero-title">
                <div>
                    <aside class="hero-start-panel" aria-label="Start with Ze">
                        <div class="hero-start-intro">
                            <div class="hero-start-brand">
                                <a
                                    href="zeledon/"
                                    aria-label="Meet Zeledon, the Ze mascot"
                                >
                                    <img
                                        class="hero-mascot"
                                        src="assets/zeledon.svg"
                                        alt="Zeledon, the Ze bird mascot"
                                        width="134"
                                        height="134"
                                    />
                                </a>
                            </div>
                            <div class="hero-start-copy">
                                <div
                                    class="tagline-carousel"
                                    data-tagline-carousel
                                >
                                    <h1
                                        id="hero-title"
                                        class="hero-start-title"
                                    >
                                        <span data-tagline-text
                                            >Open routing for white-label
                                            hardware</span
                                        >
                                    </h1>
                                    <template data-tagline-options>
                                        <span
                                            >Open routing for white-label
                                            hardware</span
                                        >
                                        <span
                                            >Internet Configuration and Protocol
                                            Engine</span
                                        >
                                        <span>Immutable Linux appliance</span>
                                    </template>
                                    <div
                                        class="tagline-carousel-controls"
                                        aria-label="Choose a Ze tagline"
                                    >
                                        <button
                                            class="tagline-carousel-arrow"
                                            type="button"
                                            data-tagline-prev
                                            aria-label="Previous tagline"
                                        >
                                            <span aria-hidden="true">←</span>
                                        </button>
                                        <div
                                            class="tagline-carousel-dots"
                                            data-tagline-dots
                                        ></div>
                                        <button
                                            class="tagline-carousel-arrow"
                                            type="button"
                                            data-tagline-next
                                            aria-label="Next tagline"
                                        >
                                            <span aria-hidden="true">→</span>
                                        </button>
                                        <span
                                            class="sr-only"
                                            data-tagline-status
                                            aria-live="polite"
                                        ></span>
                                    </div>
                                </div>
                            </div>
                            <div class="hero-start-lead-wrap">
                                <p class="hero-start-lead">
                                    Ze starts with a small core: a supervisor,
                                    message bus, config provider, and plugin
                                    manager.
                                </p>
                                <p class="hero-start-lead">
                                    Protocols and services register themselves
                                    with <strong class="hl blue">YANG</strong>.
                                    <strong class="hl blue">BGP</strong>,
                                    interface management, FIB programming,
                                    <strong class="hl blue">CLI</strong>, web,
                                    API, and <strong class="hl blue">MCP</strong>
                                    then share the same model.
                                </p>
                                <p class="hero-release-badge">
                                    Expected Initial release: Q4 2026
                                </p>
                            </div>
                        </div>
                        <nav
                            class="hero-start-actions"
                            aria-label="Start with Ze shortcuts"
                        >
                            <a
                                class="hero-start-action"
                                href="labs/bgp-interop/"
                                ><strong>Run a BGP lab</strong
                                ><small
                                    >Exercise Ze against FRR, BIRD, and GoBGP in
                                    Docker.</small
                                ></a
                            >
                            <a
                                class="hero-start-action"
                                href="guides/quickstart/"
                                ><strong>Quickstart</strong
                                ><small
                                    >Bring up two BGP peers in under five
                                    minutes.</small
                                ></a
                            >
                            <a
                                class="hero-start-action search-trigger"
                                href="search/"
                                aria-expanded="false"
                                ><strong>Search the site</strong
                                ><small
                                    >Find commands, labs, guides, and generated
                                    references. <span class="search-shortcut-hint" aria-hidden="true"><kbd>⌘K</kbd><span>or</span><kbd>⌘/</kbd></span></small
                                ></a
                            >
                            <a
                                class="hero-start-action"
                                href="https://github.com/ze-software/ze"
                                target="_blank"
                                rel="noopener"
                                ><strong>GitHub</strong
                                ><small
                                    >Read the source, issues, tests, and release
                                    work.</small
                                ></a
                            >
                            <a
                                class="hero-start-action hero-start-action-primary"
                                href="https://discord.gg/T8s7CjPDne"
                                target="_blank"
                                rel="noopener"
                                ><strong>Join Discord</strong
                                ><small
                                    >Ask before spending a weekend on a
                                    build.</small
                                ></a
                            >
                            <a
                                class="hero-start-action"
                                href="guides/ze-install/"
                                ><strong>Install Ze</strong
                                ><small
                                    >Run it on Linux or as a bootable
                                    appliance.</small
                                ></a
                            >
                        </nav>
                    </aside>
                </div>
            </section>

            <section class="outcome-strip reveal" aria-label="Ze product map">
                <a href="architecture/">Protocol-agnostic core</a>
                <a href="reference/configuration/">YANG per subsystem</a>
                <a href="features/#routing">BGP, interfaces, FIB</a>
                <a href="features/ai-first/">CLI, SSH, web, API, MCP</a>
                <a href="reference/plugins/">Compiled or external plugins</a>
                <a href="license/">AGPLv3 source</a>
            </section>

{whats_new}

            <section id="proof" class="home-proof-block reveal" aria-labelledby="proof-title">
                <div class="home-proof-head">
                    <div>
                        <h2 id="proof-title">Tests, fuzzing, and interop before release claims.</h2>
                        <p>
                            These counts show what backs Ze before you spend
                            time on a lab.<br />The interop list names the peer
                            daemons used in the protocol checks.
                        </p>
                    </div>
                    <div class="home-proof-actions">
                        <a class="button primary" href="labs/bgp-interop/">Run a BGP lab</a>
                        <a class="button" href="quality/">Read the evidence map</a>
                    </div>
                </div>
                <div class="proof-strip" aria-label="Project evidence">
                    <a class="proof" href="quality/unit-fuzz-mutation/">
                        <strong
                            >{unit_tests} <span class="label">unit tests</span></strong
                        >
                        <ul>
                            <li>Wire encoding, parsing</li>
                            <li>Config, FSM, plugins</li>
                            <li>gomu mutates code to check assertions</li>
                        </ul>
                    </a>
                    <a class="proof" href="quality/functional-ci/">
                        <strong
                            >{e2e_tests}
                            <span class="label">end to end tests</span></strong
                        >
                        <ul>
                            <li>Peering, sessions, updates</li>
                            <li>Editor, commits, reloads</li>
                            <li>Commands checked as operators run them</li>
                        </ul>
                    </a>
                    <a class="proof" href="quality/unit-fuzz-mutation/#fuzz-targets-are-still-tests">
                        <strong>{fuzz_targets} <span class="label">fuzz targets</span></strong>
                        <ul>
                            <li>Parsers, external inputs</li>
                            <li>Wire formats, config files</li>
                            <li>Saved crashes become regression cases</li>
                        </ul>
                    </a>
                    <a class="proof" href="quality/qemu-interop-release/#docker-interop">
                        <strong
                            >{interop_targets} <span class="label">interop targets</span></strong
                        >
                        <ul>
                            <li>Real third-party daemons</li>
                            <li>BGP sessions in Docker</li>
                            <li>Routes checked by peer CLIs</li>
                        </ul>
                    </a>
                </div>
                <div class="interop-strip" aria-label="Tested BGP peer implementations">
                    <span class="interop-strip-label">Tested against routing stacks</span>
                    <a href="quality/qemu-interop-release/#docker-interop">FRR</a>
                    <a href="quality/qemu-interop-release/#docker-interop">BIRD</a>
                    <a href="quality/qemu-interop-release/#docker-interop">GoBGP</a>
                    <a href="quality/qemu-interop-release/#docker-interop">OpenBGPd</a>
                    <a href="quality/qemu-interop-release/#docker-interop">FreeRtr</a>
                    <a href="quality/qemu-interop-release/#docker-interop">RustyBGP</a>
                    <a href="quality/qemu-interop-release/#docker-interop">rustbgpd</a>
                    <a href="features/exabgp-compatibility/">ExaBGP migration path</a>
                </div>
            </section>

            <section class="home-section-panel home-section-panel-run" aria-labelledby="run-title">
                <div class="section-head reveal tone-teal">
                    <h2 id="run-title">
                        Run it as a lab,<br />daemon, or appliance.
                    </h2>
                    <p>
                        The same binary and configuration support each path,
                        from a <a href="labs/netlab/">netlab topology</a>
                        or BGP interop lab to spare hardware.
                    </p>
                </div>
                <div class="audience run-path-grid reveal">
{run_cards}
                </div>
            </section>

            <section id="why-ze" class="home-section-panel home-section-panel-why" aria-labelledby="why-title">
                <div class="section-head reveal tone-sky">
                    <h2 id="why-title">Why Ze.</h2>
                    <p>
                        Ze keeps the core protocol-agnostic. Subsystems bring
                        their own YANG, and the CLI, web editor, validation,
                        generated references, and MCP tools are derived from the
                        resulting schema.
                    </p>
                </div>
                <div class="cards usp-grid reveal" aria-label="Ze architectural arguments">
                    <article class="card usp-card tone-mint">
                        <span class="cat">Model</span>
                        <h3><a href="reference/configuration/">One model feeds every interface</a></h3>
                        <p>
                            Each subsystem declares YANG. The config tree,
                            validation, CLI completion, web editor, API, MCP,
                            docs, audit, and diagnostics read that model.
                        </p>
                    </article>
                    <article class="card usp-card cat-platform">
                        <span class="cat">Runtime</span>
                        <h3><a href="architecture/">Small core, registered subsystems</a></h3>
                        <p>
                            The core holds the supervisor, message bus, config
                            provider, and plugin manager. BGP and interface
                            management register into it.
                        </p>
                    </article>
                    <article class="card usp-card cat-routing">
                        <span class="cat">Routing</span>
                        <h3><a href="architecture/">Network OS built on the engine</a></h3>
                        <p>
                            The shipped daemon speaks BGP, manages Linux
                            interfaces, programs the FIB, and serves its
                            configuration through SSH and the web UI.
                        </p>
                    </article>
                    <article class="card usp-card tone-pink">
                        <span class="cat">Honest</span>
                        <h3><a href="reference/plugins/">Plugins keep their own contract</a></h3>
                        <p>
                            Plugins can be compiled Go modules or external
                            processes. Compiled modules load their YANG into the
                            daemon validator; external plugins can expose their
                            model through <code>ze schema</code>.
                        </p>
                    </article>
                </div>
            </section>

            <section id="features-summary" class="home-section-panel home-section-panel-map" aria-labelledby="core-title">
                <div class="section-head reveal">
                    <h2 id="core-title">Generated references are part of the product.</h2>
                    <p>Read the generated references before you run Ze.</p>
                </div>
                <div class="section-note reveal">
                    <p>
                        Ze is an open-source configuration and protocol engine.
                        The network operating system built on it speaks BGP,
                        manages Linux interfaces, programs the FIB, and serves
                        the same YANG-modeled configuration through SSH, web,
                        API, and MCP.
                    </p>
                </div>
                <div class="legend reveal" role="group" aria-label="Browse features by category">
{category_links}
                </div>
            </section>

            <section id="try" class="home-section-panel home-section-panel-try" aria-labelledby="try-title">
                <div class="section-head reveal tone-grape">
                    <h2 id="try-title">First paths for routing feedback.</h2>
                    <p>The BGP lab, ExaBGP migration, and appliance install are good starting points.</p>
                </div>
                <div class="terminal-panel reveal">
                    <div class="terminal" aria-label="Quick start commands">
                        <div class="terminal-dots">
                            <span></span><span></span><span></span>
                            <span class="terminal-title">quickstart.sh</span>
                        </div>
                        <div class="terminal-body">
                            <pre><span class="term-comment"># build from source</span>
<span class="term-prompt">$</span> <span class="term-cmd">git clone</span> https://github.com/ze-software/ze.git
<span class="term-prompt">$</span> <span class="term-cmd">cd</span> ze && <span class="term-cmd">make</span> build

<span class="term-comment"># set up credentials and configure</span>
<span class="term-prompt">$</span> bin/ze <span class="term-cmd">init</span>
<span class="term-prompt">$</span> bin/ze <span class="term-cmd">config import</span> router.conf

<span class="term-comment"># start</span>
<span class="term-prompt">$</span> bin/ze <span class="term-cmd">start</span>

<span class="term-comment"># from another terminal</span>
<span class="term-prompt">$</span> bin/ze <span class="term-cmd">cli</span> -c "show bgp peer list"
<span class="term-prompt">$</span> bin/ze <span class="term-cmd">cli</span> -c "monitor event"</pre>
                        </div>
                    </div>
                    <div class="terminal-note">
                        <span class="tag">Good starting points</span>
                        <p>
                            A lab peer, a migrated ExaBGP config, or a
                            looking-glass instance can produce useful reports
                            from people who know routing operations.
                        </p>
                        <div class="link-list">
                            <a href="labs/bgp-interop/"
                                >BGP interop lab
                                <span>FRR, BIRD, and GoBGP in Docker</span></a
                            >
                            <a href="use-cases/exabgp-migration/"
                                >ExaBGP migration
                                <span>try an existing config and process script</span></a
                            >
                            <a href="guides/appliance/"
                                >Appliance install
                                <span>ISO media, PXE provisioning, spare hardware</span></a
                            >
                            <a href="guides/public-looking-glass/"
                                >Looking glass
                                <span>publish read-only BGP visibility</span></a
                            >
                            <a href="features/ai-first/"
                                >AI-assisted operations
                                <span>MCP exposes Ze commands to tools</span></a
                            >
                        </div>
                        <div class="link-list home-secondary-routes">
                            <a href="faq/"
                                >FAQ <span>answers before you try Ze</span></a
                            >
                            <a href="roadmap/"
                                >Project status
                                <span>what works and what may change</span></a
                            >
                            <a
                                href="https://discord.gg/T8s7CjPDne" target="_blank" rel="noopener"
                                >Ask on Discord
                                <span>talk to the project before a lab build</span></a
                            >
                        </div>
                    </div>
                </div>
            </section>

            <section class="home-section-panel home-section-panel-usecases" aria-labelledby="audience-title">
                <div class="section-head reveal">
                    <h2 id="audience-title">Safe ways to try Ze before the first release.</h2>
                    <p>
                        Ze is early enough that routing feedback can still
                        change the system. These cards give each reader a
                        low-risk starting point.
                    </p>
                </div>
                <div class="audience home-usecase-grid reveal">
{who_cards}
                </div>
            </section>

            <section class="home-section-panel home-section-panel-momentum" aria-labelledby="blog-teaser-title">
                <div class="section-head reveal">
                    <h2 id="blog-teaser-title">Recent engineering notes.</h2>
                    <p>
                        Weekly updates come from git history and Discord's
                        <code>ze-news</code>. They stay specific and technical.
                    </p>
                </div>
                <div class="cards reveal">
{blog_teaser_cards}
                </div>
                <div class="link-list reveal">
                    <a href="changes/">See all updates</a>
                </div>
            </section>

            <section id="try-safely" class="home-section-panel home-section-panel-safe" aria-labelledby="try-safely-title">
                <div class="status-panel try-safely-panel reveal">
                    <div class="status-copy">
                        <span class="tag">Try safely</span>
                        <h2 id="try-safely-title">Try Ze before the first release.</h2>
                        <p>
                            Start where a mistake cannot affect a live network.
                        </p>
                        <div class="actions">
                            <a class="button primary" href="labs/bgp-interop/">Run a BGP lab</a>
                            <a class="button" href="guides/quickstart/">Read the quickstart</a>
                        </div>
                    </div>
                    <div class="status-table" aria-label="Safe starting points">
                        <div class="status-row">
                            <strong>Release</strong>
                            <span
                                >Expected Q4 2026. No stable release has shipped
                                yet, and configuration may change.</span
                            >
                        </div>
                        <div class="status-row">
                            <strong>BGP lab</strong>
                            <span
                                >Exercise Ze against FRR, BIRD, and GoBGP
                                without touching a production router.</span
                            >
                        </div>
                        <div class="status-row">
                            <strong>Migration</strong>
                            <span
                                >Try the ExaBGP migration path against an
                                existing config before changing automation.</span
                            >
                        </div>
                        <div class="status-row">
                            <strong>Appliance</strong>
                            <span
                                >Boot Ze on spare hardware with the same binary
                                and configuration model as daemon mode.</span
                            >
                        </div>
                        <div class="status-row">
                            <strong>Source</strong>
                            <span
                                >Read the code, generated docs, RFC gate, and
                                test evidence before deciding where Ze belongs.</span
                            >
                        </div>
                    </div>
                </div>
            </section>
"""


def render(data):
    root = ""
    title = "Ze - Configuration and Protocol Engine for Internet Infrastructure"
    desc = (
        "Ze is an open-source configuration and protocol engine. The network "
        "operating system built on it speaks BGP, manages Linux interfaces, "
        "programs the FIB, and serves config over SSH, web, API, and MCP."
    )
    og_desc = (
        "Open-source configuration and protocol engine with a "
        "protocol-agnostic core, YANG-modeled subsystems, operator "
        "interfaces, runnable labs, and an ExaBGP migration path."
    )
    head = sitelib.page_head(title, desc, root, og_title=title, og_desc=og_desc)

    counts = sitelib.feature_counts_by_category()
    category_links = "\n".join(
        '                    <a class="cat-%s" href="features/#%s">%s (%d)</a>'
        % (cat, cat, cat.capitalize(), counts[cat])
        for cat in sitelib.CATEGORIES
    )

    run_cards = "\n".join(render_audience_card(c) for c in data["run"])
    who_cards = "\n".join(render_audience_card(c) for c in data["who"])
    change_topics = change_topics_by_slug()
    blog_teaser_cards = "\n".join(
        render_blog_teaser_card(p, i, change_topics.get(p["slug"], []))
        for i, p in enumerate(sitelib.latest_blog_posts(3))
    )
    whats_new = render_whats_new(
        models.validate_whats_new(json.loads(WHATS_NEW_DATA.read_text()))
    )
    body = BODY.format(
        run_cards=run_cards,
        who_cards=who_cards,
        category_links=category_links,
        blog_teaser_cards=blog_teaser_cards,
        whats_new=whats_new,
        **proof_stats(),
    )

    DEST.write_text(head + body + sitelib.page_foot(root))
    print("rendered %s -> %s" % (DATA, DEST))


def main():
    data = models.validate_audience(json.loads(DATA.read_text()))
    render(data)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
