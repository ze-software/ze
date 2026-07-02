#!/usr/bin/env -S uv run python3
"""Render the project activity heatmap into its own site page.

Usage:
    tools/render-activity.py

Runs presentations/tools/loc_activity.py fresh against the main repo (never
touches any presentation's own frozen activity.html -- those are historic
snapshots), extracts the heatmap widget, restyles it to the site's candy
palette, and writes activity/index.html. Re-run this to refresh the data;
same one-command-regenerates-everything workflow as tools/render-docs.py.
"""

import json
import pathlib
import subprocess
import sys
import tempfile
import urllib.request

NAV_CHEVRON = (
    '<svg viewBox="0 0 12 8" fill="none" aria-hidden="true">'
    '<path d="M1 1l5 5 5-5" stroke="currentColor" stroke-width="1.6" '
    'stroke-linecap="round" stroke-linejoin="round"/></svg>'
)

_GITHUB_STARS_FALLBACK = 39  # last known count, used if the API call fails
_github_stars_cache = None


def get_github_stars():
    """Live star count for ze-software/ze, fetched once per script run and
    cached (unauthenticated GitHub API allows 60 req/hour/IP -- a batch run
    over ~40 docs must not spend that fetching the same number 40 times).
    Falls back to the last known count on any network/API failure so a
    regeneration never hard-fails for lack of connectivity."""
    global _github_stars_cache
    if _github_stars_cache is not None:
        return _github_stars_cache
    try:
        req = urllib.request.Request(
            "https://api.github.com/repos/ze-software/ze",
            headers={
                "Accept": "application/vnd.github+json",
                "User-Agent": "ze-site-build",
            },
        )
        with urllib.request.urlopen(req, timeout=5) as resp:
            data = json.loads(resp.read())
        _github_stars_cache = int(data["stargazers_count"])
    except Exception as exc:
        print(
            "warning: could not fetch live GitHub star count (%s), using last known value %d"
            % (exc, _GITHUB_STARS_FALLBACK),
            file=sys.stderr,
        )
        _github_stars_cache = _GITHUB_STARS_FALLBACK
    return _github_stars_cache


def build_nav_badges():
    stars = get_github_stars()
    return (
        "                    <a\n"
        '                        class="nav-badge"\n'
        '                        href="https://discord.gg/3Sx4S2dYQ"\n'
        '                        target="_blank"\n'
        '                        rel="noopener"\n'
        '                        aria-label="Ze Discord"\n'
        '                        ><svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">'
        '<path d="M4 4h16a1 1 0 0 1 1 1v11a1 1 0 0 1-1 1H9l-5 4v-4H4a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1z"/>'
        "</svg>Discord</a\n"
        "                    >\n"
        "                    <a\n"
        '                        class="nav-badge"\n'
        '                        href="https://github.com/ze-software/ze"\n'
        '                        target="_blank"\n'
        '                        rel="noopener"\n'
        '                        aria-label="Ze on GitHub, %d stars"\n'
        '                        ><svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">'
        '<path d="M12 2l2.9 6.9 7.1.6-5.4 4.6 1.7 7-6.3-3.9-6.3 3.9 1.7-7L1 9.5l7.1-.6L12 2z"/>'
        "</svg>%d</a\n"
        "                    >\n"
    ) % (stars, stars)


def nav_item(root, href, icon, title, desc):
    return (
        '                        <a class="nav-dropdown-item" href="%s%s">\n'
        '                            <span class="nav-dropdown-icon">%s</span>\n'
        "                            <span><strong>%s</strong><small>%s</small></span>\n"
        "                        </a>\n"
    ) % (root, href, icon, title, desc)


def nav_dropdown(label, columns):
    out = ['                    <div class="nav-dropdown">\n']
    out.append(
        '                    <button class="nav-dropdown-trigger" type="button">%s\n'
        "                        %s\n"
        "                    </button>\n" % (label, NAV_CHEVRON)
    )
    out.append('                    <div class="nav-dropdown-panel">\n')
    for col in columns:
        out.append('                    <div class="nav-dropdown-col">\n')
        for entry in col:
            if isinstance(entry, str):
                out.append(
                    '                        <span class="nav-dropdown-label">%s</span>\n'
                    % entry
                )
            else:
                out.append(nav_item(*entry))
        out.append("                    </div>\n")
    out.append("                    </div>\n")
    out.append("                    </div>\n")
    return "".join(out)


def nav_dropdown_rooted(root, label, columns):
    rooted_columns = [
        [entry if isinstance(entry, str) else (root,) + entry for entry in col]
        for col in columns
    ]
    return nav_dropdown(label, rooted_columns)


def build_navblock(root):
    out = ['                <div class="nav-links">\n']
    out.append('                    <a href="%sindex.html#status">Status</a>\n' % root)
    out.append('                    <a href="%sindex.html#try">Try</a>\n' % root)
    out.append(
        nav_dropdown_rooted(
            root,
            "Project",
            [
                [
                    (
                        "features/",
                        "\U0001f9e9",
                        "Features",
                        "41 features, color-coded by category",
                    ),
                    (
                        "performance/",
                        "⚡",
                        "Performance",
                        "Measured BGP benchmarks, not claims",
                    ),
                    (
                        "compare/",
                        "⚖️",
                        "Compare",
                        "How Ze stacks up against BIRD, FRR, and more",
                    ),
                    (
                        "activity/",
                        "\U0001f4c8",
                        "Activity",
                        "A year of commits, at a glance",
                    ),
                ]
            ],
        )
    )
    out.append(
        nav_dropdown_rooted(
            root,
            "Labs",
            [
                [
                    (
                        "labs/bgp-interop/",
                        "\U0001f310",
                        "BGP Protocol Interop",
                        "Real FRR, BIRD, and GoBGP sessions",
                    ),
                    (
                        "labs/l2tp-interop/",
                        "\U0001f50c",
                        "L2TP PPP/NCP Interop",
                        "Ze as LNS vs real xl2tpd",
                    ),
                    (
                        "labs/pppoe-interop/",
                        "\U0001f4e1",
                        "PPPoE Interop",
                        "vs real accel-ppp access concentrator",
                    ),
                    (
                        "labs/ipsec-interop/",
                        "\U0001f512",
                        "IPsec / IKEv2 Interop",
                        "Ze as IKE initiator vs strongSwan",
                    ),
                ],
                [
                    (
                        "labs/vlan-qos/",
                        "\U0001f3f7️",
                        "VLAN QoS Wire-Level Proof",
                        "802.1p PCP tagging, actually on the wire",
                    ),
                    (
                        "labs/looking-glass-graph/",
                        "\U0001f52d",
                        "Looking Glass Graph Demo",
                        "Realistic UK topology, real external ASNs",
                    ),
                    (
                        "labs/appliance-install/",
                        "\U0001f4bf",
                        "Appliance Installer Evidence",
                        "HTTP/PXE, ISO, Ventoy, real boots",
                    ),
                    (
                        "labs/vpp-dataplane/",
                        "\U0001f680",
                        "VPP Dataplane Evidence",
                        "FIB, traffic, and firewall in a real VPP daemon",
                    ),
                ],
            ],
        )
    )
    out.append(
        nav_dropdown_rooted(
            root,
            "Docs",
            [
                [
                    "Getting started",
                    (
                        "docs/guide/quickstart/",
                        "\U0001f680",
                        "Quickstart",
                        "Two BGP peers in under 5 minutes",
                    ),
                    (
                        "docs/features/configuration/",
                        "⚙️",
                        "Configuration",
                        "YANG-modeled, one model for everything",
                    ),
                    (
                        "docs/features/cli-commands/",
                        "⌨️",
                        "CLI Commands",
                        "SSH CLI with diff, commit, and history",
                    ),
                    (
                        "docs/architecture/",
                        "\U0001f3d7️",
                        "Architecture",
                        "How the pieces fit together",
                    ),
                ],
                [
                    "Deep dives",
                    (
                        "docs/features/mcp-integration/",
                        "\U0001f916",
                        "MCP Integration",
                        "AI-assisted operations",
                    ),
                    (
                        "docs/features/web-interface/",
                        "\U0001f5a5️",
                        "Web Interface",
                        "HTMX config editor and admin panel",
                    ),
                    (
                        "docs/guide/vpp/",
                        "\U0001f4e6",
                        "VPP Dataplane",
                        "FIB, traffic, and firewall via VPP",
                    ),
                    (
                        "docs/guide/benchmarking/",
                        "\U0001f4ca",
                        "Benchmarking",
                        "ze-perf architecture, flags, JSON output",
                    ),
                ],
            ],
        )
    )
    out.append('                    <a href="%sblog/">Blog</a>\n' % root)
    out.append('                    <a href="%stalks/">Talks</a>\n' % root)
    out.append(build_nav_badges())
    out.append("                </div>")
    return "".join(out)


HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
LOC_ACTIVITY = GH_PAGES / "presentations" / "tools" / "loc_activity.py"
DEST = GH_PAGES / "activity" / "index.html"


def slice_between(text, start_marker, end_marker, start_from=0):
    start = text.index(start_marker, start_from)
    end = text.index(end_marker, start) + len(end_marker)
    return text[start:end], end


def extract(raw):
    stats_start = raw.index('<section class="stats"')
    stats_end = raw.index("</section>", stats_start) + len("</section>")
    stats_html = raw[stats_start:stats_end]

    chart_start = raw.index('<div class="chart-scroll">')
    legend_open = raw.index('<div class="legend">', chart_start)
    legend_end = raw.index("</div>", legend_open) + len("</div>")
    chart_html = raw[chart_start:legend_end]

    go_panel_start = raw.index('<section class="panel go-panel"')
    go_panel_end = raw.index("</section>", go_panel_start) + len("</section>")
    go_panel_html = raw[go_panel_start:go_panel_end]
    # bucket titles repeat "Go Code Stats" ("Total First-Party Go" etc.) --
    # the panel h2 already says "Go Code Stats", so drop the redundant suffix.
    go_panel_html = (
        go_panel_html.replace("<h3>Total First-Party Go</h3>", "<h3>Total Code</h3>")
        .replace("<h3>Production Go</h3>", "<h3>Production</h3>")
        .replace("<h3>Test Go</h3>", "<h3>Test</h3>")
        .replace("<h3>Vendored Dependencies</h3>", "<h3>Dependencies</h3>")
    )

    tables_h2 = raw.index('<h2 id="top-heading">')
    panel_start = raw.rindex('<div class="panel">', 0, tables_h2)
    first_table_end = raw.index("</table>", tables_h2) + len("</table>")
    second_table_end = raw.index("</table>", first_table_end) + len("</table>")
    panel_end = raw.index("</div>", second_table_end) + len("</div>")
    tables_html = raw[panel_start:panel_end]

    tooltip_start = raw.index('<div id="activity-tooltip"')
    tooltip_end = raw.index("</div>", tooltip_start) + len("</div>")
    tooltip_html = raw[tooltip_start:tooltip_end]

    script_start = raw.index("<script>")
    script_end = raw.index("</script>") + len("</script>")
    script_html = raw[script_start:script_end]

    return stats_html, chart_html, go_panel_html, tables_html, tooltip_html, script_html


PAGE = """<!doctype html>
<html lang="en">
    <head>
        <meta charset="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>Development Activity - Ze</title>
        <meta
            name="description"
            content="A year of Ze's commit and added-line history, visualized as a calendar heatmap. Live data, regenerated from git history."
        />
        <meta property="og:title" content="Development Activity - Ze" />
        <meta
            property="og:description"
            content="A year of Ze's commit and added-line history, visualized as a calendar heatmap. Live data, regenerated from git history."
        />
        <meta property="og:type" content="website" />
        <link rel="icon" href="../assets/ze.svg" type="image/svg+xml" />
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
        <link
            href="https://fonts.googleapis.com/css2?family=Poppins:wght@300;400;600;700;800&family=Lato:wght@300;400;700&display=swap"
            rel="stylesheet"
        />
        <link rel="stylesheet" href="../assets/site.css" />
        <style>
{style}
        </style>
    </head>
    <body>
        <header class="site-header">
            <nav class="nav" aria-label="Main navigation">
                <a class="brand" href="../index.html#top" aria-label="Ze home">
                    <img src="../assets/ze.svg" alt="" width="32" height="32" />
                    <span>Ze</span>
                </a>
{navblock}
            </nav>
        </header>

        <main id="top">
            <section aria-labelledby="activity-title">
                <div class="section-head reveal cat-observe">
                    <h2 id="activity-title">Development activity.</h2>
                    <p>A year of commits, at a glance.</p>
                </div>
                <div class="activity-widget reveal" aria-label="Activity heatmap">
{stats}
                    <div class="dashboard-grid">
                        <div class="left-stack">
{chart}
{go_panel}
                        </div>
                        <div class="right-stack">
{tables}
                        </div>
                    </div>
{tooltip}
                </div>
            </section>
        </main>

{script}

        <script src="../assets/site.js" defer></script>

        <footer>
            <div class="footer-inner">
                <span>Ze is AGPLv3 open source.</span>
                <div class="footer-links">
                    <a
                        href="https://github.com/ze-software/ze"
                        target="_blank"
                        rel="noopener"
                        >GitHub</a
                    >
                    <a
                        href="https://codeberg.org/thomas-mangin/ze"
                        target="_blank"
                        rel="noopener"
                        >Codeberg</a
                    >
                    <a
                        href="https://github.com/ze-software/ze/issues"
                        target="_blank"
                        rel="noopener"
                        >Issues</a
                    >
                    <a
                        href="https://discord.gg/3Sx4S2dYQ"
                        target="_blank"
                        rel="noopener"
                        >Discord</a
                    >
                    <a href="../style-guide/">Style Guide</a>
                </div>
            </div>
        </footer>
    </body>
</html>
"""

STYLE = """
        .activity-widget {
            display: grid;
            gap: 1rem;
        }
        .activity-widget .stats {
            display: grid;
            grid-template-columns: repeat(4, minmax(0, 1fr)) auto;
            gap: 0.9rem;
            align-items: center;
        }
        .activity-widget .stat {
            border: 2px solid rgba(255, 255, 255, 0.9);
            border-radius: 1rem;
            padding: 0.8rem;
            background: var(--mint-tint);
            box-shadow: var(--clay);
        }
        .activity-widget .stat span {
            color: var(--muted);
            font-size: 0.82rem;
        }
        .activity-widget .stat strong {
            display: block;
            margin-top: 0.2rem;
            color: var(--mint-deep);
            font-size: clamp(1.15rem, 2vw, 1.6rem);
            font-weight: 800;
            letter-spacing: -0.02em;
        }
        .activity-widget .metric-switch {
            display: inline-flex;
            gap: 0.3rem;
            padding: 0.3rem;
            border: 2px solid rgba(255, 255, 255, 0.9);
            border-radius: 999px;
            background: var(--panel);
            box-shadow: var(--clay);
        }
        .activity-widget .metric-switch button {
            border: 0;
            border-radius: 999px;
            padding: 0.4rem 0.65rem;
            color: var(--muted);
            background: transparent;
            font: inherit;
            font-size: 0.76rem;
            font-weight: 800;
            cursor: pointer;
        }
        .activity-widget .metric-switch button[aria-pressed="true"] {
            color: #fff;
            background: var(--mint-base);
        }
        .activity-widget .pill {
            width: fit-content;
            border: 2px solid rgba(255, 255, 255, 0.9);
            border-radius: 999px;
            padding: 0.35rem 0.6rem;
            color: var(--muted);
            background: var(--panel);
            font-size: 0.74rem;
            white-space: nowrap;
        }
        .activity-widget .dashboard-grid {
            display: grid;
            grid-template-columns: minmax(0, 1fr) minmax(14rem, 16rem);
            gap: 1rem;
            align-items: start;
        }
        .activity-widget .left-stack,
        .activity-widget .right-stack {
            display: grid;
            gap: 1rem;
        }
        .activity-widget .panel {
            border: 2px solid rgba(255, 255, 255, 0.9);
            border-radius: 1.4rem;
            padding: 1.1rem;
            background: var(--panel-strong);
            box-shadow: var(--clay);
        }
        .activity-widget .chart-scroll {
            overflow-x: auto;
            padding-bottom: 0.4rem;
        }
        .activity-widget .chart {
            min-width: calc(53 * (12px + 3px) + 3.4rem);
        }
        .activity-widget .months {
            display: grid;
            grid-template-columns: repeat(53, 12px);
            gap: 3px;
            margin-left: 3.4rem;
            margin-bottom: 0.45rem;
            color: var(--dim);
            font-size: 0.75rem;
        }
        .activity-widget .month-label {
            min-height: 1rem;
        }
        .activity-widget .chart-body {
            display: flex;
            gap: 0.7rem;
        }
        .activity-widget .weekday-labels {
            display: grid;
            grid-template-rows: repeat(7, 12px);
            gap: 3px;
            width: 2.7rem;
            color: var(--dim);
            font-size: 0.72rem;
            line-height: 12px;
            text-align: right;
        }
        .activity-widget .activity-grid {
            display: grid;
            grid-auto-flow: column;
            grid-template-rows: repeat(7, 12px);
            grid-auto-columns: 12px;
            gap: 3px;
        }
        .activity-widget .day-cell {
            width: 12px;
            height: 12px;
            border: 1px solid rgba(255, 255, 255, 0.9);
            border-radius: 3px;
            background: #0d0d10;
            cursor: default;
        }
        .activity-widget .activity-grid .day-cell:hover,
        .activity-widget .activity-grid .day-cell:focus-visible {
            border-color: var(--mint-deep);
            outline: none;
            transform: scale(1.28);
        }
        .activity-widget .day-cell[data-level="1"] { background: #cdf2de; }
        .activity-widget .day-cell[data-level="2"] { background: #8fe0b8; }
        .activity-widget .day-cell[data-level="3"] { background: #4fce8f; }
        .activity-widget .day-cell[data-level="4"] { background: #22b06a; }
        .activity-widget .day-cell[data-level="5"] {
            background: #14d97a;
            box-shadow: 0 0 10px var(--mint-glow);
        }
        .activity-widget .day-cell.pre-repo {
            border-color: transparent;
            background: transparent;
            opacity: 0.3;
        }
        .activity-widget .day-cell.outside { opacity: 0.3; }
        .activity-widget .legend {
            display: flex;
            align-items: center;
            justify-content: flex-end;
            gap: 0.45rem;
            margin-top: 0.85rem;
            color: var(--muted);
            font-size: 0.8rem;
        }
        .activity-widget .legend .day-cell { display: inline-block; }
        .activity-widget .panel h2 {
            margin: 0 0 0.6rem;
            color: var(--mint-deep);
            font-size: 1.1rem;
        }
        .activity-widget .go-breakdown {
            display: grid;
            grid-template-columns: repeat(4, minmax(0, 1fr));
            gap: 0.9rem;
        }
        .activity-widget .go-bucket {
            border: 2px solid rgba(255, 255, 255, 0.9);
            border-radius: 1rem;
            padding: 0.8rem;
            background: var(--mint-tint);
        }
        .activity-widget .go-bucket h3 {
            margin: 0 0 0.55rem;
            color: var(--mint-deep);
            font-size: 0.92rem;
        }
        .activity-widget .go-stats {
            display: grid;
            grid-template-columns: 1fr;
            gap: 0.35rem;
        }
        .activity-widget .go-panel .stat {
            padding: 0.5rem;
            background: var(--panel);
        }
        .activity-widget .go-panel .stat strong {
            font-size: 1rem;
        }
        .activity-widget .go-panel .stat span {
            font-size: 0.72rem;
        }
        .activity-widget table {
            width: 100%;
            border-collapse: collapse;
        }
        .activity-widget table[hidden] { display: none; }
        .activity-widget th,
        .activity-widget td {
            padding: 0.4rem 0;
            border-bottom: 1px solid var(--line);
            text-align: left;
            font-size: 0.85rem;
        }
        .activity-widget th {
            color: var(--muted);
            font-size: 0.72rem;
            font-weight: 700;
            text-transform: uppercase;
            letter-spacing: 0.08em;
        }
        .activity-widget td:last-child,
        .activity-widget th:last-child { text-align: right; }
        .activity-tooltip {
            position: fixed;
            z-index: 50;
            min-width: 15rem;
            padding: 0.9rem 1rem;
            border: 2px solid rgba(255, 255, 255, 0.9);
            border-radius: 1rem;
            color: #fff;
            background: var(--mint-deep);
            box-shadow: var(--clay), 0 1.4rem 3rem -0.6rem var(--mint-glow);
            pointer-events: none;
        }
        .activity-tooltip[hidden] { display: none; }
        .tooltip-date {
            display: block;
            font-size: 1.05rem;
            font-weight: 800;
        }
        .tooltip-gap { height: 0.6rem; }
        .tooltip-value {
            display: block;
            color: rgba(255, 255, 255, 0.85);
            font-size: 0.92rem;
            line-height: 1.4;
        }
        @media (max-width: 860px) {
            .activity-widget .dashboard-grid { grid-template-columns: 1fr; }
            .activity-widget .stats { grid-template-columns: repeat(2, minmax(0, 1fr)); }
            .activity-widget .go-breakdown { grid-template-columns: repeat(2, minmax(0, 1fr)); }
        }
"""


def main():
    with tempfile.TemporaryDirectory() as tmp:
        raw_path = pathlib.Path(tmp) / "activity-raw.html"
        result = subprocess.run(
            [
                sys.executable,
                str(LOC_ACTIVITY),
                "--compact",
                "--output",
                str(raw_path),
                "--days",
                "365",
            ],
            capture_output=True,
            text=True,
        )
        if result.returncode != 0:
            print(result.stderr, file=sys.stderr)
            return 1
        raw = raw_path.read_text()

    stats_html, chart_html, go_panel_html, tables_html, tooltip_html, script_html = (
        extract(raw)
    )

    page = PAGE.format(
        style=STYLE,
        navblock=build_navblock("../"),
        stats=stats_html,
        chart=chart_html,
        go_panel=go_panel_html,
        tables=tables_html,
        tooltip=tooltip_html,
        script=script_html,
    )
    DEST.parent.mkdir(parents=True, exist_ok=True)
    DEST.write_text(page)
    print("rendered activity heatmap -> %s" % DEST)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
