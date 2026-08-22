#!/usr/bin/env -S uv run python3
"""Render project/milestones/index.html from data/milestones.json.

Usage:
    tools/render-timeline.py

Every milestone is data (data/milestones.json), not hand-authored HTML.
The page is a vertical timeline of the landmark features Ze has shipped,
newest first, grouped by quarter and color-coded by the same seven
categories as the Features page (so a routing node reads tangerine, a
secure node pink, and so on, straight from assets/site.css's cat-* vars).

This is deliberately coarser than project/changes/ (which lists every week) and
features/ (which is current state, not chronology): one node per capability
class the first time it arrived. Add a row to data/milestones.json and
re-run to publish it. A render_markdown() sibling reads the same data, so
the .md mirror can never disagree with the HTML.
"""

import datetime
import html
import json
import pathlib

import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
DATA = GH_PAGES / "data" / "milestones.json"
DEST = GH_PAGES / "project" / "milestones" / "index.html"

TITLE = "Milestones - Ze"


def quarter_of(date):
    """'2026-04-13' -> ('2026-Q2', 'Q2 2026') for grouping and display."""
    d = datetime.date.fromisoformat(date)
    q = (d.month - 1) // 3 + 1
    return "%d-Q%d" % (d.year, q), "Q%d %d" % (q, d.year)


def month_label(date):
    """'2026-04-13' -> 'Apr 2026', the per-node date shown on the spine."""
    d = datetime.date.fromisoformat(date)
    return d.strftime("%b %Y")


def grouped(milestones):
    """Milestones sorted newest first, split into
    (quarter_label, [milestone, ...]) groups."""
    groups = []
    for m in sorted(milestones, key=lambda item: item["date"], reverse=True):
        key, label = quarter_of(m["date"])
        if not groups or groups[-1][0] != key:
            groups.append((key, label, []))
        groups[-1][2].append(m)
    return [(label, items) for _key, label, items in groups]


EXTRA_HEAD = """        <style>
            .tl-quarter { margin: 0 0 0.5rem; }
            .tl-quarter-head {
                font-size: 0.82rem;
                font-weight: 700;
                letter-spacing: 0.12em;
                text-transform: uppercase;
                color: var(--muted);
                margin: 2.2rem 0 0.9rem;
                padding-left: 1.75rem;
            }
            .tl-list {
                list-style: none;
                margin: 0;
                padding: 0 0 0 1.75rem;
                position: relative;
            }
            /* The spine: one continuous line down the left of each quarter. */
            .tl-list::before {
                content: "";
                position: absolute;
                left: 6px;
                top: 6px;
                bottom: 6px;
                width: 2px;
                background: var(--line-strong);
                border-radius: 2px;
            }
            .tl-item {
                position: relative;
                padding: 0 0 1.4rem 0;
            }
            .tl-item.filtered-out { display: none; }
            .tl-node {
                position: absolute;
                left: -1.75rem;
                top: 0.35rem;
                width: 14px;
                height: 14px;
                border-radius: 50%;
                background: var(--acc, var(--muted));
                border: 3px solid var(--bg);
                box-shadow: 0 0 0 2px var(--acc, var(--muted));
            }
            .tl-date {
                font-size: 0.78rem;
                font-weight: 700;
                letter-spacing: 0.04em;
                color: var(--acc-deep, var(--muted));
                text-transform: uppercase;
            }
            .tl-card {
                margin-top: 0.25rem;
                padding: 0.9rem 1.1rem;
                background: var(--acc-tint, var(--bg-soft));
                border: 1px solid var(--line);
                border-left: 3px solid var(--acc, var(--line-strong));
                border-radius: 10px;
            }
            .tl-head {
                display: flex;
                flex-wrap: wrap;
                align-items: baseline;
                gap: 0.6rem;
                margin-bottom: 0.35rem;
            }
            .tl-title { margin: 0; font-size: 1.06rem; }
            .tl-cat {
                font-size: 0.68rem;
                font-weight: 700;
                letter-spacing: 0.06em;
                text-transform: uppercase;
                color: var(--acc-deep, var(--muted));
            }
            .tl-card p { margin: 0 0 0.5rem; color: var(--text); }
            .tl-link {
                font-size: 0.85rem;
                font-weight: 600;
                text-decoration: none;
                color: var(--acc-deep, var(--muted));
            }
            .tl-link:hover { text-decoration: underline; }
            @media (max-width: 640px) {
                .tl-quarter-head, .tl-list { padding-left: 1.4rem; }
                .tl-node { left: -1.4rem; }
            }
        </style>
"""


def render_item(m):
    href = "../changes/%s/" % m["blog"] if m.get("blog") else None
    parts = [
        '                    <li class="tl-item cat-%s" data-cat="%s">'
        % (m["category"], m["category"]),
        '                        <span class="tl-node" aria-hidden="true"></span>',
        '                        <div class="tl-date"><time datetime="%s">%s</time></div>'
        % (m["date"], month_label(m["date"])),
        '                        <div class="tl-card">',
        '                            <div class="tl-head">',
        '                                <h3 class="tl-title">%s</h3>'
        % html.escape(m["title"], quote=False),
        '                                <span class="tl-cat">%s</span>'
        % html.escape(m["category"], quote=False),
        "                            </div>",
        "                            <p>%s</p>" % sitelib.bold(m["blurb"]),
    ]
    if href:
        parts.append(
            '                            <a class="tl-link" href="%s">Read the week &rarr;</a>'
            % href
        )
    parts.append("                        </div>")
    parts.append("                    </li>")
    return "\n".join(parts)


def render(data):
    milestones = data["milestones"]
    count = len(milestones)
    desc = (
        "The landmark features Ze has shipped, newest first: one node per "
        "capability the first time it arrived, on a timeline."
    )
    out = [sitelib.page_head(TITLE, desc, "../../", extra_head=EXTRA_HEAD, page_key="project/milestones/")]

    out.append('            <section aria-labelledby="milestones-title">')
    out.append('                <div class="section-head journey-hero reveal">')
    out.append('                    <span class="journey-eyebrow">Timeline</span>')
    out.append(
        '                    <h1 id="milestones-title">The road so far.</h1>'
    )
    out.append(
        "                    <p>%d milestones, newest first. %s</p>"
        % (count, html.escape(data["intro"], quote=False))
    )
    out.append("                </div>")
    out.append('                <div class="section-note reveal">')
    out.append(
        "                    <p>Each node's color is its category. This is the "
        "coarse view: the <a href=\"../changes/\">Changes</a> log has every "
        "week, and <a href=\"../../features/\">Features</a> lists what ships "
        "today. Click a category to filter, click again to show everything.</p>"
    )
    out.append("                </div>")
    out.append(
        '                <div class="legend reveal" role="group" aria-label="Filter milestones by category">'
    )
    for cat in sitelib.CATEGORIES:
        out.append(
            '                    <button class="cat-%s" data-cat="%s" aria-pressed="false">%s</button>'
            % (cat, cat, cat.capitalize())
        )
    out.append("                </div>")
    out.append("            </section>")
    out.append("")

    out.append('            <section class="reveal" aria-label="Milestone timeline">')
    for label, items in grouped(milestones):
        out.append('                <div class="tl-quarter" data-quarter>')
        out.append('                    <h2 class="tl-quarter-head">%s</h2>' % label)
        out.append('                    <ol class="tl-list">')
        for m in items:
            out.append(render_item(m))
        out.append("                    </ol>")
        out.append("                </div>")
    out.append("            </section>")

    body = "\n".join(out)
    dest_text = body + "\n" + sitelib.page_foot("../../")
    DEST.parent.mkdir(parents=True, exist_ok=True)
    DEST.write_text(dest_text)
    sitelib.write_markdown_sibling(DEST, render_markdown(data))
    print("rendered %s -> %s (%d milestones, + index.md)" % (DATA, DEST, count))


def render_markdown(data):
    parts = [
        "# Milestones",
        "",
        data["intro"],
        "",
    ]
    for label, items in grouped(data["milestones"]):
        parts.append("## %s" % label)
        parts.append("")
        for m in items:
            parts.append("### %s (%s)" % (m["title"], month_label(m["date"])))
            parts.append("")
            parts.append("*%s*" % m["category"])
            parts.append("")
            parts.append(m["blurb"])
            if m.get("blog"):
                parts.append("")
                parts.append("[Read the week](../changes/%s/)" % m["blog"])
            parts.append("")
    return "\n".join(parts).strip() + "\n"


def main():
    data = json.loads(DATA.read_text())
    render(data)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
