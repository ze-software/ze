#!/usr/bin/env -S uv run python3
"""Render index.html from a template plus data/audience.json.

Usage:
    tools/render-index.py

The hero, proof strip, and status panel are bespoke marketing copy and stay
as a literal template here (there's nothing repeated to model as data). The
"Two ways to run Ze" and "Who should look now" card grids are data
(data/audience.json) -- add or edit an audience card there instead of
hand-editing HTML.
"""

import html
import json
import pathlib

import sitelib
import sitefacts

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
DATA = GH_PAGES / "data" / "audience.json"
CHANGES_DATA = GH_PAGES / "data" / "changes.json"
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

    return """                    <article class="card audience-card cat-{category}">
                        <span class="cat">{label}</span>
                        <h3>{title}</h3>
                        <p>
                            {body}
                        </p>
{chips}{cta}                    </article>""".format(
        category=esc(category),
        label=esc(label),
        title=title,
        body=esc(card["body"]),
        chips=render_chip_row(card),
        cta=cta,
    )


BLOG_TEASER_CATEGORIES = [
    "cat-operate",
    "cat-routing",
    "cat-automate",
    "cat-secure",
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
    cat = BLOG_TEASER_CATEGORIES[i % len(BLOG_TEASER_CATEGORIES)]
    parts = [
        '                    <article class="card card-post home-update-card %s">'
        % cat
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


BODY = """            <section class="hero" aria-labelledby="hero-title">
                <div>
                    <div class="hero-brand">
                        <a
                            href="zeledon/"
                            aria-label="Meet Zeledon, the Ze mascot"
                        >
                            <img
                                class="hero-mascot"
                                src="assets/zeledon.svg"
                                alt="Zeledon, the Ze bird mascot"
                                width="68"
                                height="68"
                            />
                        </a>
                        <h1 id="hero-title">ze</h1>
                    </div>
                    <p class="eyebrow">
                        Open, programmable network OS for Linux
                    </p>
                    <p class="lead">
                        Ze creates appliances or makes Linux speak
                        <strong class="hl blue">BGP</strong>, IS-IS, and OSPF,
                        manages interfaces, programs the FIB, and gives
                        operators a CLI, web UI, telemetry, looking glass,
                        API, and plugin system around one coherent
                        configuration model.
                    </p>
                    <p class="sublead">
                        Built for people who want a network stack they can
                        inspect, automate, and extend.
                        <strong class="hl blue">ExaBGP</strong> users get a
                        migration path to a more performant codebase.
                    </p>
                    <div class="actions">
                        <a
                            class="button primary"
                            href="docs/guide/quickstart/"
                            >Quick Start</a
                        >
                        <a
                            class="button"
                            href="docs/"
                            >Read Docs</a
                        >
                        <a
                            class="button"
                            href="https://discord.gg/T8s7CjPDne"
                            target="_blank"
                            rel="noopener"
                            >Get Help</a
                        >
                    </div>
                </div>
                <div class="terminal hero-terminal" aria-label="Ze CLI session">
                    <div class="terminal-dots">
                        <span></span><span></span><span></span>
                            <span class="terminal-title">bin/ze cli</span>
                    </div>
                    <div class="terminal-body">
                        <pre><span class="term-comment"># start the daemon</span>
<span class="term-prompt">$</span> bin/ze example.conf
level=INFO  msg="hub ready" subsystem=hub plugins=1 peers=1 listen=":179"
level=INFO  msg="peer connecting" subsystem=bgp.reactor peer=test-peer address=10.0.0.2

<span class="term-comment"># from another terminal</span>
<span class="term-prompt">$</span> bin/ze <span class="term-cmd">cli</span> -c "show bgp peer list"
<span class="term-prompt">$</span> bin/ze <span class="term-cmd">cli</span> -c "monitor event"</pre>
                    </div>
                </div>
            </section>

            <div class="proof-strip reveal" aria-label="Project evidence">
                <div class="proof">
                    <strong
                        >{unit_tests} <span class="label">unit tests</span></strong
                    >
                    <ul>
                        <li>Wire encoding, parsing</li>
                        <li>Config, FSM, plugins</li>
                        <li>gomu mutates code to check assertions</li>
                    </ul>
                </div>
                <div class="proof">
                    <strong
                        >{e2e_tests}
                        <span class="label">end to end tests</span></strong
                    >
                    <ul>
                        <li>Peering, sessions, updates</li>
                        <li>Editor, commits, reloads</li>
                    </ul>
                </div>
                <div class="proof">
                    <strong>{fuzz_targets} <span class="label">fuzz targets</span></strong>
                    <ul>
                        <li>Parsers, external inputs</li>
                        <li>Wire formats, config files</li>
                    </ul>
                </div>
                <div class="proof">
                    <strong
                        >{interop_targets} <span class="label">interop targets</span></strong
                    >
                    <ul>
                        <li>FRR, BIRD, GoBGP</li>
                        <li>OpenBGPd, FreeRtr</li>
                        <li>RustyBGP, rustbgpd</li>
                    </ul>
                </div>
            </div>

            <section id="status" aria-labelledby="status-title">
                <div class="status-panel reveal">
                    <div class="status-copy">
                        <span class="tag">Current status</span>
                        <h2 id="status-title">Jump in early.</h2>
                        <p>
                            Ze has a modern routing core, BGP, OSPF, IS-IS,
                            and MPLS, wrapped in a friendly network OS. The
                            code is heavily tested, and the project is
                            moving fast.
                        </p>
                        <p>
                            It is still young. Operational mileage is limited
                            and configuration may change. Upgrade paths will be
                            provided after the first release. Use it in labs,
                            break it, read the code, and tell us what is wrong.
                        </p>
                        <p class="status-links">
                            Ze is free software under the
                            <a href="license/">AGPLv3</a>. See the
                            <a href="roadmap/">roadmap</a> for the path to a
                            release, and the
                            <a href="security/">security policy</a> to report an
                            issue.
                        </p>
                    </div>
                    <div class="status-table" aria-label="Feature status">
                        <div class="status-row">
                            <strong>Full-featured BGP</strong>
                            <span
                                >Sessions, capabilities, UPDATE handling,
                                IPv4/IPv6 unicast, and growing FlowSpec family
                                coverage.</span
                            >
                        </div>
                        <div class="status-row">
                            <strong>Powerful tooling</strong>
                            <span
                                >SSH CLI with diff and commit, web workbench,
                                looking glass, telemetry, all from one config
                                model.</span
                            >
                        </div>
                        <div class="status-row">
                            <strong>Extensible by design</strong>
                            <span
                                >RIB, route-server, graceful restart, RPKI,
                                policy, persistence, and external process
                                plugins.</span
                            >
                        </div>
                        <div class="status-row">
                            <strong>Easy to explore</strong>
                            <span
                                >Built-in doctor checks, readable state, MCP
                                server for AI-assisted debugging, clear errors
                                throughout.</span
                            >
                        </div>
                    </div>
                </div>
            </section>

            <section aria-labelledby="run-title">
                <div class="section-head reveal cat-platform">
                    <h2 id="run-title">Two ways to run Ze.</h2>
                    <p>
                        Same binary, same config, either way. Pick whichever
                        fits how you already operate.
                    </p>
                </div>
                <div class="audience audience-2col reveal">
{run_cards}
                </div>
            </section>

            <section id="features-summary" aria-labelledby="core-title">
                <div class="section-head reveal">
                    <h2 id="core-title">Built for demanding operators.</h2>
                    <p>Shipped and tested, or experimental and growing.</p>
                </div>
                <div class="section-note reveal">
                    <p>
                        Ze owns its BGP engine, configuration model, plugin
                        system, and operator surfaces, all designed together --
                        from the native BGP, OSPF, and IS-IS engines and SSH
                        CLI to RPKI, looking glass, telemetry, firewall, VPN,
                        MPLS, and appliance packaging.
                    </p>
                </div>
                <div class="legend reveal" role="group" aria-label="Browse features by category">
{category_links}
                </div>
            </section>


            <section id="try" aria-labelledby="try-title">
                <div class="section-head reveal cat-operate">
                    <h2 id="try-title">Discover what makes Ze unique.</h2>
                    <p>
                        The best users today are people building labs,
                        route-server experiments, BGP tooling, or network
                        appliances, and anyone curious about what comes after
                        ExaBGP.
                    </p>
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

<span class="term-comment"># connect to the CLI</span>
<span class="term-prompt">$</span> bin/ze <span class="term-cmd">cli</span></pre>
                        </div>
                    </div>
                    <div class="terminal-note">
                        <span class="tag">Good first paths</span>
                        <p>
                            Start with a lab peer, a migrated ExaBGP config, or
                            a looking-glass instance. The project needs feedback
                            from people who know what real routing operations
                            look like.
                        </p>
                        <div class="link-list">
                            <a
                                href="docs/architecture/"
                                >YANG Configuration
                                <span>one model for everything</span></a
                            >
                            <a
                                href="docs/features/"
                                >Programmable
                                <span>API, plugins, automation</span></a
                            >
                            <a
                                href="docs/features/mcp-integration/"
                                >MCP Control
                                <span>AI-assisted operations</span></a
                            >
                            <a href="compare/"
                                >Compare
                                <span>how Ze stacks up -- and where it's still behind</span></a
                            >
                            <a href="labs/"
                                >Labs
                                <span>real interop proof you can run yourself</span></a
                            >
                        </div>
                    </div>
                </div>
            </section>

            <section aria-labelledby="audience-title">
                <div class="section-head reveal">
                    <h2 id="audience-title">Who should look now?</h2>
                    <p>
                        Ze is early enough that strong feedback can still
                        shape the system. If you care about open routing
                        software, now is the time to look.
                    </p>
                </div>
                <div class="audience reveal">
{who_cards}
                </div>
            </section>

            <section aria-label="Closing statement">
                <div class="closing reveal">
                    <h2>Open routing needs boring miles.</h2>
                    <p>
                        Ze has the shape of the system we want: open, modern,
                        and programmable. It still needs users, hardware,
                        failures, odd networks, and the slow confidence that
                        comes from deployments.
                    </p>
                </div>
            </section>

            <section aria-labelledby="blog-teaser-title">
                <div class="section-head reveal">
                    <h2 id="blog-teaser-title">Latest updates.</h2>
                    <p>
                        What shipped recently, week by week, mined from git
                        history and posted to Discord's <code>ze-news</code>.
                    </p>
                </div>
                <div class="cards reveal">
{blog_teaser_cards}
                </div>
                <div class="link-list reveal">
                    <a href="changes/">See all updates</a>
                </div>
            </section>
"""


def render(data):
    root = ""
    title = "Ze - Open, Programmable Network OS For Linux"
    desc = (
        "Ze is a pre-release open-source network OS for Linux, built around "
        "a native BGP engine, operator interfaces, telemetry, and plugins."
    )
    og_desc = (
        "A serious BGP core and a growing network OS around it. Early, "
        "tested, open, and ready for labs."
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
        for i, p in enumerate(sitelib.latest_blog_posts(4))
    )
    body = BODY.format(
        run_cards=run_cards,
        who_cards=who_cards,
        category_links=category_links,
        blog_teaser_cards=blog_teaser_cards,
        **proof_stats(),
    )

    DEST.write_text(head + body + sitelib.page_foot(root))
    print("rendered %s -> %s" % (DATA, DEST))


def main():
    data = json.loads(DATA.read_text())
    render(data)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
