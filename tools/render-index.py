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

import json
import pathlib

import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
DATA = GH_PAGES / "data" / "audience.json"
DEST = GH_PAGES / "index.html"


def render_run_card(card):
    link = card["link"]
    return """                    <article class="audience-card">
                        <h3>{title}</h3>
                        <p>
                            {body}
                        </p>
                        <div class="link-list">
                            <a
                                href="{href}"
                                >{label} <span>{sublabel}</span></a
                            >
                        </div>
                    </article>""".format(
        title=card["title"],
        body=card["body"],
        href=link["href"],
        label=link["label"],
        sublabel=link["sublabel"],
    )


def render_who_card(card):
    return """                    <article class="audience-card">
                        <h3>{title}</h3>
                        <p>
                            {body}
                        </p>
                    </article>""".format(title=card["title"], body=card["body"])


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
                        Successor to <strong class="hl blue">ExaBGP</strong>.
                        Built for people who want a network stack they can
                        inspect, automate, and extend.
                    </p>
                    <div class="actions">
                        <a
                            class="button primary"
                            href="https://github.com/ze-software/ze#quick-start"
                            target="_blank"
                            rel="noopener"
                            >Quick Start</a
                        >
                        <a
                            class="button"
                            href="https://github.com/ze-software/ze/tree/main/docs"
                            target="_blank"
                            rel="noopener"
                            >Read Docs</a
                        >
                        <a
                            class="button"
                            href="https://discord.gg/3Sx4S2dYQ"
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
<span class="term-prompt">$</span> bin/ze <span class="term-cmd">cli</span> -c "bgp monitor"</pre>
                    </div>
                </div>
            </section>

            <div class="proof-strip reveal" aria-label="Project evidence">
                <div class="proof">
                    <strong
                        >13,700+ <span class="label">unit tests</span></strong
                    >
                    <ul>
                        <li>Wire encoding, parsing</li>
                        <li>Config, FSM, plugins</li>
                        <li>Mutation testing via gomu</li>
                    </ul>
                </div>
                <div class="proof">
                    <strong
                        >1,200+
                        <span class="label">end to end tests</span></strong
                    >
                    <ul>
                        <li>Peering, sessions, updates</li>
                        <li>Editor, commits, reloads</li>
                    </ul>
                </div>
                <div class="proof">
                    <strong>55+ <span class="label">fuzz targets</span></strong>
                    <ul>
                        <li>Parsers, external inputs</li>
                        <li>Wire formats, config files</li>
                    </ul>
                </div>
                <div class="proof">
                    <strong
                        >7 <span class="label">interop targets</span></strong
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
<span class="term-prompt">$</span> <span class="term-cmd">git clone</span> https://codeberg.org/thomas-mangin/ze.git
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

    run_cards = "\n".join(render_run_card(c) for c in data["run"])
    who_cards = "\n".join(render_who_card(c) for c in data["who"])
    body = BODY.format(run_cards=run_cards, who_cards=who_cards)

    DEST.write_text(head + body + sitelib.page_foot(root))
    print("rendered %s -> %s" % (DATA, DEST))


def main():
    data = json.loads(DATA.read_text())
    render(data)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
