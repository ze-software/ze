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
import re
import subprocess
import sys
import tempfile

import sitelib
import sitepaths

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
LOC_ACTIVITY = GH_PAGES / "presentations" / "tools" / "loc_activity.py"
DEST = GH_PAGES / "activity" / "index.html"
MAIN_REPO = sitepaths.MAIN_REPO


def slice_between(text, start_marker, end_marker, start_from=0):
    start = text.index(start_marker, start_from)
    end = text.index(end_marker, start) + len(end_marker)
    return text[start:end], end


def extract(raw):
    def idx(marker, start=0):
        pos = raw.find(marker, start)
        if pos == -1:
            raise ValueError(
                "activity extract: marker %r not found in loc_activity output; "
                "presentations/tools/loc_activity.py markup may have changed" % marker
            )
        return pos

    stats_start = idx('<section class="stats"')
    stats_end = idx("</section>", stats_start) + len("</section>")
    stats_html = raw[stats_start:stats_end]
    stats_html = stats_html.replace(
        '<div style="display:flex;flex-direction:column;align-items:center;justify-content:center;gap:0.4rem">',
        '<div class="metric-control">',
    )
    stats_html = re.sub(r"Peak (line|commit) day \([^)]+\)", r"Peak \1 day", stats_html)

    chart_start = idx('<div class="chart-scroll">')
    legend_open = idx('<div class="legend">', chart_start)
    legend_end = idx("</div>", legend_open) + len("</div>")
    chart_html = raw[chart_start:legend_end]

    go_panel_start = idx('<section class="panel go-panel"')
    go_panel_end = idx("</section>", go_panel_start) + len("</section>")
    go_panel_html = raw[go_panel_start:go_panel_end]
    # bucket titles repeat the original panel name in every heading.
    # The panel h2 is clearer on its own, so drop the redundant suffix.
    go_panel_html = (
        go_panel_html.replace("<h2>Go Code Stats</h2>", "<h2>Go code composition</h2>")
        .replace("<h3>Total First-Party Go</h3>", "<h3>Total Code</h3>")
        .replace("<h3>Production Go</h3>", "<h3>Production</h3>")
        .replace("<h3>Test Go</h3>", "<h3>Test</h3>")
        .replace("<h3>Vendored Dependencies</h3>", "<h3>Dependencies</h3>")
    )


    tooltip_start = idx('<div id="activity-tooltip"')
    tooltip_last_span = idx("</span>", idx("tooltip-secondary", tooltip_start)) + len("</span>")
    tooltip_end = idx("</div>", tooltip_last_span) + len("</div>")
    tooltip_html = raw[tooltip_start:tooltip_end]

    script_start = idx("<script>")
    script_end = idx("</script>") + len("</script>")
    script_html = re.sub(r'"peakLabel":"Peak (line|commit) day \([^)]+\)"', r'"peakLabel":"Peak \1 day"', raw[script_start:script_end])
    script_html = re.sub(r',"topHeading":"[^"]+","topColumn":"[^"]+"', "", script_html)
    script_html = re.sub(r'^\s*setEl\("top-heading".*\n', "", script_html, flags=re.MULTILINE)
    script_html = re.sub(
        r'^\s*for \(const table of document\.querySelectorAll\("\[data-top-table\]"\)\) \{\n'
        r"\s*table\.hidden = table\.dataset\.topTable !== metric;\n"
        r"\s*\}\n",
        "",
        script_html,
        flags=re.MULTILINE,
    )

    return stats_html, chart_html, go_panel_html, tooltip_html, script_html


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
            href="{font_css}"
            rel="stylesheet"
        />
        <link rel="stylesheet" href="{site_css}" />
        <style>
{style}
        </style>
{json_ld}    </head>
    <body>
        <header class="site-header">
            <nav class="nav" aria-label="Main navigation">
                <a class="brand" href="../#top" aria-label="Ze home">
                    <img src="../assets/ze.svg" alt="" width="32" height="32" />
                    <span>Ze</span>
                </a>
{navblock}
            </nav>
        </header>

        <main id="top">
            <section class="activity-page" aria-labelledby="activity-title">
                <div class="activity-hero journey-hero reveal">
                    <span class="activity-eyebrow journey-eyebrow">Git telemetry</span>
                    <h1 id="activity-title">Development activity</h1>
                    <p>A year of commits, added lines, and Go composition regenerated from the repository.</p>
                </div>
                <div class="activity-widget reveal" aria-label="Activity heatmap">
{stats}
                    <div class="dashboard-grid">
                        <div class="left-stack">
{chart}
{go_panel}
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
        .activity-page {
            position: relative;
            display: grid;
            gap: clamp(0.8rem, 1.8vw, 1.2rem);
        }
        .activity-page::before {
            content: "";
            position: absolute;
            inset: -2rem -1rem auto auto;
            z-index: -1;
            width: min(34rem, 70vw);
            height: min(34rem, 70vw);
            border-radius: 999px;
            background:
                radial-gradient(circle at 45% 40%, rgba(34, 255, 155, 0.24), transparent 48%),
                radial-gradient(circle at 75% 72%, rgba(96, 165, 250, 0.22), transparent 52%);
            filter: blur(8px);
        }
        .activity-hero {
            position: relative;
            overflow: hidden;
            border: 2px solid rgba(255, 255, 255, 0.92);
            border-radius: clamp(1.1rem, 2.5vw, 1.55rem);
            padding: clamp(1.15rem, 2.4vw, 1.65rem);
            background:
                radial-gradient(circle at 86% 38%, rgba(157, 100, 245, 0.18), transparent 14rem),
                radial-gradient(circle at 20% 18%, rgba(26, 188, 156, 0.14), transparent 16rem),
                linear-gradient(135deg, rgba(255, 254, 254, 0.96), rgba(233, 245, 255, 0.84));
            box-shadow: var(--clay), 0 1.7rem 4rem -2.2rem var(--shadow);
            color: var(--text);
            margin-bottom: 0;
        }
        .activity-hero::before {
            content: "";
            position: absolute;
            inset: 0;
            background:
                linear-gradient(rgba(190, 120, 210, 0.08) 1px, transparent 1px),
                linear-gradient(90deg, rgba(190, 120, 210, 0.08) 1px, transparent 1px);
            background-size: 22px 22px;
            mask-image: linear-gradient(115deg, #000, transparent 72%);
            pointer-events: none;
        }
        .activity-hero::after {
            content: "";
            position: absolute;
            right: clamp(0.8rem, 2vw, 1.2rem);
            bottom: clamp(0.8rem, 2vw, 1.2rem);
            width: min(12rem, 28vw);
            height: min(12rem, 28vw);
            border: 1px solid rgba(157, 100, 245, 0.24);
            border-radius: 999px;
            background:
                radial-gradient(circle, transparent 55%, rgba(157, 100, 245, 0.16) 56%, transparent 57%),
                radial-gradient(circle, rgba(0, 159, 227, 0.12), transparent 68%);
            transform: translate(20%, 24%);
            pointer-events: none;
        }
        .activity-eyebrow {
            position: absolute;
            top: clamp(1rem, 2vw, 1.25rem);
            right: clamp(1rem, 2.4vw, 1.5rem);
            z-index: 1;
            display: inline-flex;
            align-items: center;
            gap: 0.45rem;
            margin-bottom: 0;
            color: var(--grape-deep);
            font-size: 0.7rem;
            font-weight: 900;
            letter-spacing: 0.16em;
            text-transform: uppercase;
        }
        .activity-eyebrow::before {
            content: "";
            width: 0.5rem;
            height: 0.5rem;
            border-radius: 999px;
            background: var(--mint-base);
            box-shadow: 0 0 1.1rem var(--mint-glow);
        }
        .activity-hero h1 {
            position: relative;
            max-width: 18ch;
            padding-right: 0;
            margin: 0;
            color: var(--text);
            background: linear-gradient(90deg, var(--grape-deep), var(--sky-deep) 54%, var(--teal-deep));
            -webkit-background-clip: text;
            background-clip: text;
            -webkit-text-fill-color: transparent;
            font-size: clamp(2.25rem, 4.8vw, 4.15rem);
            line-height: 0.92;
            letter-spacing: -0.065em;
        }
        .activity-hero p {
            position: relative;
            max-width: 45rem;
            margin: 0.65rem 0 0;
            color: var(--muted);
            font-size: clamp(0.94rem, 1.4vw, 1.08rem);
            line-height: 1.5;
        }
        .activity-widget {
            --activity-ink: #09111d;
            --activity-card: rgba(255, 255, 255, 0.74);
            --activity-card-strong: rgba(255, 255, 255, 0.9);
            --activity-border: rgba(255, 255, 255, 0.94);
            display: grid;
            gap: 1rem;
        }
        .activity-widget .stats {
            display: grid;
            grid-template-columns: repeat(4, minmax(0, 1fr)) minmax(11rem, auto);
            gap: 0.85rem;
            align-items: stretch;
        }
        .activity-widget .stat {
            position: relative;
            overflow: hidden;
            border: 2px solid var(--activity-border);
            border-radius: 1.05rem;
            padding: 0.85rem;
            background:
                linear-gradient(145deg, rgba(255, 255, 255, 0.78), rgba(255, 255, 255, 0.52)),
                var(--sky-tint);
            box-shadow: var(--clay);
        }
        .activity-widget .stat::after {
            content: "";
            position: absolute;
            inset: auto 0 0;
            height: 0.22rem;
            background: linear-gradient(90deg, var(--mint-base), var(--sky-base), var(--grape-base));
            opacity: 0.75;
        }
        .activity-widget .stat span {
            display: block;
            color: var(--muted);
            font-size: 0.78rem;
            font-weight: 800;
            letter-spacing: 0.02em;
            text-transform: uppercase;
        }
        .activity-widget .stat strong {
            display: block;
            margin-top: 0.22rem;
            color: var(--activity-ink);
            font-size: clamp(1.2rem, 2.1vw, 1.7rem);
            font-weight: 900;
            letter-spacing: -0.04em;
        }
        .activity-widget .metric-control {
            display: grid;
            justify-items: end;
            align-content: center;
            gap: 0.45rem;
            min-width: 10.5rem;
        }
        .activity-widget .metric-switch {
            display: inline-flex;
            gap: 0.3rem;
            padding: 0.32rem;
            border: 2px solid var(--activity-border);
            border-radius: 999px;
            background: var(--activity-card-strong);
            box-shadow: var(--clay);
        }
        .activity-widget .metric-switch button {
            border: 0;
            border-radius: 999px;
            padding: 0.45rem 0.7rem;
            color: var(--muted);
            background: transparent;
            font: inherit;
            font-size: 0.74rem;
            font-weight: 900;
            letter-spacing: 0.03em;
            text-transform: uppercase;
            cursor: pointer;
        }
        .activity-widget .metric-switch button[aria-pressed="true"] {
            color: #07111d;
            background: linear-gradient(135deg, #9fffd0, #7dd3fc);
            box-shadow: inset 0 -1px 0 rgba(0, 0, 0, 0.14);
        }
        .activity-widget .pill {
            width: fit-content;
            border: 1px solid rgba(15, 23, 42, 0.08);
            border-radius: 999px;
            padding: 0.4rem 0.65rem;
            color: var(--muted);
            background: rgba(255, 255, 255, 0.62);
            font-size: 0.74rem;
            font-weight: 800;
            white-space: nowrap;
        }
        .activity-widget .dashboard-grid,
        .activity-widget .left-stack {
            display: grid;
            gap: 1rem;
        }
        .activity-widget .panel {
            border: 2px solid var(--activity-border);
            border-radius: 1.45rem;
            padding: clamp(1rem, 2vw, 1.25rem);
            background:
                linear-gradient(160deg, rgba(255, 255, 255, 0.86), rgba(255, 255, 255, 0.68)),
                var(--panel-strong);
            box-shadow: var(--clay);
        }
        .activity-widget .chart-scroll {
            overflow-x: auto;
            padding: clamp(0.8rem, 1.4vw, 1rem);
            border: 2px solid rgba(255, 255, 255, 0.94);
            border-radius: 1.15rem;
            background:
                radial-gradient(circle at 9% 16%, rgba(26, 188, 156, 0.15), transparent 18rem),
                linear-gradient(145deg, rgba(255, 254, 254, 0.94), rgba(233, 245, 255, 0.82));
            box-shadow: var(--clay);
        }
        .activity-widget .chart-scroll::-webkit-scrollbar {
            height: 0.7rem;
        }
        .activity-widget .chart-scroll::-webkit-scrollbar-track {
            background: rgba(190, 120, 210, 0.16);
            border-radius: 999px;
        }
        .activity-widget .chart-scroll::-webkit-scrollbar-thumb {
            background: rgba(157, 100, 245, 0.5);
            border-radius: 999px;
        }
        .activity-widget .chart {
            min-width: 735px;
        }
        .activity-widget .months {
            display: grid;
            grid-template-columns: repeat(53, 11px);
            gap: 2px;
            margin-left: 3rem;
            margin-bottom: 0.42rem;
            color: var(--muted);
            font-size: 0.72rem;
            font-weight: 800;
        }
        .activity-widget .month-label {
            min-height: 1rem;
        }
        .activity-widget .chart-body {
            display: flex;
            gap: 0.55rem;
        }
        .activity-widget .weekday-labels {
            display: grid;
            grid-template-rows: repeat(7, 11px);
            gap: 2px;
            width: 2.45rem;
            color: var(--muted);
            font-size: 0.66rem;
            font-weight: 800;
            line-height: 11px;
            text-align: right;
            text-transform: uppercase;
        }
        .activity-widget .activity-grid {
            display: grid;
            grid-auto-flow: column;
            grid-template-rows: repeat(7, 11px);
            grid-auto-columns: 11px;
            gap: 2px;
        }
        .activity-widget .day-cell {
            width: 11px;
            height: 11px;
            border: 1px solid rgba(255, 255, 255, 0.95);
            border-radius: 3px;
            background: var(--grape-tint);
            cursor: default;
            transition: transform 150ms ease, border-color 150ms ease, box-shadow 150ms ease;
        }
        .activity-widget .activity-grid .day-cell:hover,
        .activity-widget .activity-grid .day-cell:focus-visible {
            position: relative;
            z-index: 2;
            border-color: rgba(255, 255, 255, 1);
            outline: none;
            box-shadow: 0 0 0 2px var(--grape-glow), var(--clay);
            transform: scale(1.22);
        }
        .activity-widget .day-cell[data-level="1"] { background: var(--mint-chip); }
        .activity-widget .day-cell[data-level="2"] { background: var(--mint-base); }
        .activity-widget .day-cell[data-level="3"] { background: var(--teal-base); }
        .activity-widget .day-cell[data-level="4"] { background: var(--sky-base); }
        .activity-widget .day-cell[data-level="5"] {
            background: var(--grape-base);
            box-shadow: 0 0 0.65rem var(--grape-glow);
        }
        .activity-widget .day-cell.pre-repo {
            border-color: transparent;
            background: transparent;
            opacity: 0.28;
        }
        .activity-widget .day-cell.outside { opacity: 0.22; }
        .activity-widget .legend {
            display: flex;
            align-items: center;
            justify-content: flex-end;
            gap: 0.45rem;
            margin-top: 0.85rem;
            color: var(--muted);
            font-size: 0.78rem;
            font-weight: 800;
        }
        .activity-widget .legend .day-cell {
            display: inline-block;
            vertical-align: middle;
        }
        .activity-widget .panel h2 {
            margin: 0 0 0.75rem;
            color: var(--activity-ink);
            font-size: 1.05rem;
            letter-spacing: -0.02em;
        }
        .activity-widget .go-panel h2::before {
            content: "";
            display: inline-block;
            width: 0.62rem;
            height: 0.62rem;
            margin-right: 0.45rem;
            border-radius: 999px;
            background: var(--grape-base);
            box-shadow: 0 0 0 0.28rem rgba(160, 110, 220, 0.16);
        }
        .activity-widget .go-breakdown {
            display: grid;
            grid-template-columns: repeat(4, minmax(0, 1fr));
            gap: 0.8rem;
        }
        .activity-widget .go-bucket {
            display: grid;
            gap: 0.45rem;
            border: 1px solid rgba(15, 23, 42, 0.07);
            border-radius: 0.95rem;
            padding: 0.65rem;
            background: rgba(255, 255, 255, 0.58);
        }
        .activity-widget .go-bucket h3 {
            margin: 0;
            color: var(--grape-deep);
            font-size: 0.78rem;
            font-weight: 900;
            letter-spacing: 0.08em;
            text-transform: uppercase;
        }
        .activity-widget .go-stats {
            display: grid;
            grid-template-columns: 1fr;
            gap: 0.22rem;
        }
        .activity-widget .go-panel .stat {
            display: flex;
            align-items: baseline;
            justify-content: space-between;
            gap: 0.7rem;
            min-height: 0;
            padding: 0.4rem 0.48rem 0.48rem;
            background: rgba(255, 255, 255, 0.62);
            box-shadow: none;
        }
        .activity-widget .go-panel .stat strong {
            margin-top: 0;
            font-size: 0.86rem;
            letter-spacing: -0.02em;
        }
        .activity-widget .go-panel .stat span {
            font-size: 0.62rem;
            white-space: nowrap;
        }
        .activity-widget table {
            width: 100%;
            border-collapse: collapse;
        }
        .activity-widget table[hidden] { display: none; }
        .activity-widget th,
        .activity-widget td {
            padding: 0.52rem 0;
            border-bottom: 1px solid rgba(15, 23, 42, 0.08);
            text-align: left;
            font-size: 0.83rem;
        }
        .activity-widget th {
            color: var(--muted);
            font-size: 0.68rem;
            font-weight: 900;
            text-transform: uppercase;
            letter-spacing: 0.1em;
        }
        .activity-widget td:last-child,
        .activity-widget th:last-child {
            color: var(--activity-ink);
            font-weight: 900;
            text-align: right;
        }
        .activity-tooltip {
            position: fixed;
            z-index: 50;
            min-width: 15rem;
            padding: 0.9rem 1rem;
            border: 1px solid rgba(255, 255, 255, 0.82);
            border-radius: 1rem;
            color: #fff;
            background: rgba(4, 24, 31, 0.94);
            box-shadow: var(--clay), 0 1.4rem 3rem -0.6rem rgba(35, 245, 156, 0.45);
            pointer-events: none;
            backdrop-filter: blur(12px);
        }
        .activity-tooltip[hidden] { display: none; }
        .tooltip-date {
            display: block;
            font-size: 1.05rem;
            font-weight: 900;
        }
        .tooltip-gap { height: 0.6rem; }
        .tooltip-value {
            display: block;
            color: rgba(255, 255, 255, 0.78);
            font-size: 0.92rem;
            line-height: 1.4;
        }
        @media (max-width: 980px) {
            .activity-widget .stats { grid-template-columns: repeat(2, minmax(0, 1fr)); }
            .activity-widget .metric-control {
                grid-column: 1 / -1;
                justify-items: start;
            }
            .activity-widget .go-breakdown { grid-template-columns: repeat(2, minmax(0, 1fr)); }
        }
        @media (max-width: 620px) {
            .activity-eyebrow {
                position: relative;
                top: auto;
                right: auto;
                margin-bottom: 0.5rem;
            }
            .activity-hero h1 {
                padding-right: 0;
                font-size: clamp(2rem, 12vw, 3.2rem);
            }
            .activity-widget .stats,
            .activity-widget .go-breakdown { grid-template-columns: 1fr; }
            .activity-widget .metric-control { justify-items: stretch; }
            .activity-widget .metric-switch { width: fit-content; }
        }
"""


def render_markdown(stats_html, go_panel_html):
    """Reuses the same stats and Go-panel HTML fragments extract() already
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
                "--repo",
                str(MAIN_REPO),
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

    stats_html, chart_html, go_panel_html, tooltip_html, script_html = extract(raw)

    page = PAGE.format(
        style=STYLE,
        site_css=sitelib.asset_url("../", "assets/site.css"),
        font_css=sitelib.FONT_CSS_URL,
        json_ld=sitelib.structured_data_script(),
        site_js=sitelib.asset_url("../", "assets/site.js"),
        navblock=sitelib.build_navblock("../"),
        stats=stats_html,
        go_panel=go_panel_html,
        chart=chart_html,
        tooltip=tooltip_html,
        script=script_html,
        footer=sitelib.footer_html("../"),
    )
    page = sitelib.patch_page_sidebar(page, "../", "activity/")
    page = sitelib.patch_social_meta(page)
    DEST.parent.mkdir(parents=True, exist_ok=True)
    DEST.write_text(page)
    sitelib.write_markdown_sibling(DEST, render_markdown(stats_html, go_panel_html))
    print("rendered activity heatmap -> %s (+ index.md)" % DEST)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
