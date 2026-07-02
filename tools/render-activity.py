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

import pathlib
import subprocess
import sys
import tempfile

import sitelib

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
        <link rel="stylesheet" href="{site_css}" />
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

        <script src="{site_js}" defer></script>

{footer}
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
            background: var(--sky-tint);
            box-shadow: var(--clay);
        }
        .activity-widget .stat span {
            color: var(--muted);
            font-size: 0.82rem;
        }
        .activity-widget .stat strong {
            display: block;
            margin-top: 0.2rem;
            color: var(--sky-deep);
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
            background: var(--sky-base);
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
            font-size: 1.1rem;
        }
        .activity-widget .go-panel h2 {
            color: var(--grape-deep);
        }
        .activity-widget .right-stack .panel h2 {
            color: var(--tangerine-deep);
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
            background: var(--grape-tint);
        }
        .activity-widget .go-bucket h3 {
            margin: 0 0 0.55rem;
            color: var(--grape-deep);
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


def render_markdown(stats_html, go_panel_html, tables_html):
    """Reuses the same stats/go-panel/tables HTML fragments extract() already
    sliced out of loc_activity.py's raw output -- run through
    sitelib.html_to_markdown -- rather than the SVG heatmap or the tab-switch
    script, neither of which mean anything as text."""
    base = sitelib.SITE_BASE + "activity/"
    parts = [
        "# Development activity",
        "",
        "A year of commits, at a glance.",
        "",
        sitelib.html_to_markdown(stats_html, base_url=base).strip(),
        "",
        sitelib.html_to_markdown(go_panel_html, base_url=base).strip(),
        "",
        sitelib.html_to_markdown(tables_html, base_url=base).strip(),
        "",
    ]
    return "\n".join(parts).strip() + "\n"


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
        site_css=sitelib.asset_url("../", "assets/site.css"),
        site_js=sitelib.asset_url("../", "assets/site.js"),
        navblock=sitelib.build_navblock("../"),
        stats=stats_html,
        chart=chart_html,
        go_panel=go_panel_html,
        tables=tables_html,
        tooltip=tooltip_html,
        script=script_html,
        footer=sitelib.footer_html("../"),
    )
    DEST.parent.mkdir(parents=True, exist_ok=True)
    DEST.write_text(page)
    sitelib.write_markdown_sibling(
        DEST, render_markdown(stats_html, go_panel_html, tables_html)
    )
    print("rendered activity heatmap -> %s (+ index.md)" % DEST)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
