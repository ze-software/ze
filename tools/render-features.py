#!/usr/bin/env -S uv run python3
"""Render features/index.html from data/features.json.

Usage:
    tools/render-features.py

Every feature card is data (data/features.json), not hand-authored HTML --
add, remove, or re-categorize a card there and re-run this to publish it.
The intro paragraph's feature count is computed from the data, so it can
never drift out of sync the way a hand-typed number did before (the "41
features" copy silently went stale when AS112 moved from roadmap to
experimental and nobody remembered to bump it).
"""

import json
import pathlib

import html

import models
import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
DATA = GH_PAGES / "data" / "features.json"
DEST = GH_PAGES / "features" / "index.html"

STATUS_LABELS = {
    "experimental": "Experimental",
    "aspiration": "Spec'd",
}


def render_markdown(data, feature_count):
    """Straight from the same data dict as render() (HTML) -- card bullets
    already carry Zeledon-style **bold**/`code` markers, which are valid
    Markdown as-is, so no HTML round-trip is needed here."""
    parts = [
        "# Every feature Ze ships.",
        "",
        "%d features plus a spec'd roadmap, color-coded by category. A "
        "card's category is how the feature fits into the system: operate, "
        "routing, services, automate, observe, secure, or platform. "
        "Everything shipped runs in both daemon and appliance modes unless "
        "a card says otherwise." % feature_count,
        "",
    ]
    for section in data["sections"]:
        parts.append("## %s" % section["heading"])
        parts.append("")
        parts.append(section["lead"])
        parts.append("")
        if section["note"]:
            parts.append("> %s" % section["note"])
            parts.append("")
        for card in section["cards"]:
            parts.append("### %s" % card["title"])
            parts.append("")
            meta_bits = [card["category"]]
            if card["status"]:
                meta_bits.append(STATUS_LABELS[card["status"]])
            line = "*%s*" % " / ".join(meta_bits)
            if card["chips"]:
                line += " -- " + " ".join("`%s`" % c["text"] for c in card["chips"])
            parts.append(line)
            parts.append("")
            for bullet in card["bullets"]:
                parts.append("- %s" % bullet)
            href = card["href"] if card["external"] else sitelib.SITE_BASE + card["href"]
            parts.append("")
            parts.append("[Learn more](%s)" % href)
            parts.append("")
    return "\n".join(parts).strip() + "\n"


def esc(value):
    return html.escape(str(value), quote=True)


def render_chip(chip):
    cls = "chip mode" if chip["mode"] else "chip"
    return '<span class="%s">%s</span>' % (cls, esc(chip["text"]))


def render_card(card):
    extra = ""
    if card["status"]:
        extra += " " + card["status"]
    parts = [
        '<article class="card feature-card%s cat-%s" data-cat="%s">'
        % (esc(extra), esc(card["category"]), esc(card["category"]))
    ]
    parts.append('<span class="cat">%s</span>' % esc(card["category"].capitalize()))
    if card["status"]:
        parts.append('<span class="status">%s</span>' % esc(STATUS_LABELS[card["status"]]))
    href = card["href"] if card["external"] else "../" + card["href"]
    link_attrs = ' target="_blank" rel="noopener"' if card["external"] else ""
    parts.append('<h3><a href="%s"%s>%s</a></h3>' % (esc(href), link_attrs, esc(card["title"])))
    parts.append('<div class="chips">')
    for chip in card["chips"]:
        parts.append(render_chip(chip))
    parts.append("</div>")
    parts.append("<ul>")
    for bullet in card["bullets"]:
        parts.append("<li>%s</li>" % sitelib.bold(bullet))
    parts.append("</ul>")
    parts.append("</article>")
    return "\n                    ".join(parts)


def render_section(section):
    parts = []
    parts.append(
        '            <section id="%s" aria-labelledby="%s-title" data-cards>'
        % (esc(section["id"]), esc(section["id"]))
    )
    parts.append('                <div class="section-head reveal">')
    parts.append(
        '                    <h2 id="%s-title">%s</h2>'
        % (esc(section["id"]), esc(section["heading"]))
    )
    parts.append("                    <p>%s</p>" % esc(section["lead"]))
    parts.append("                </div>")
    if section["note"]:
        parts.append('                <div class="section-note reveal">')
        parts.append("                    <p>%s</p>" % sitelib.bold(section["note"]))
        parts.append("                </div>")
    parts.append('                <div class="cards feature-grid reveal">')
    for card in section["cards"]:
        parts.append("                    " + render_card(card))
    parts.append("                </div>")
    parts.append("            </section>")
    return "\n".join(parts)


def render(data):
    root = "../"
    core = next(s for s in data["sections"] if s["id"] == "core")
    experimental = next(s for s in data["sections"] if s["id"] == "experimental")
    feature_count = len(core["cards"]) + len(experimental["cards"])
    category_counts = sitelib.feature_counts_by_category()

    title = "Features - Ze"
    desc = (
        "Every feature Ze ships and the spec'd roadmap ahead, "
        "grouped by maturity and color-coded by category."
    )
    out = [
        sitelib.page_head(
            title, desc, root, og_title=title, og_desc=desc, page_key="features/"
        )
    ]

    out.append('            <section aria-labelledby="features-title">')
    out.append(
        sitelib.page_hero(
            "Every feature Ze ships.",
            "%d features plus a spec'd roadmap, color-coded by category."
            % feature_count,
            "Project",
            h1_id="features-title",
        )
    )
    out.append('                <div class="section-note reveal">')
    out.append(
        "                    <p>Each card's color is its category: how the feature "
        "fits into the system. Solid cards are shipped; dashed cards are "
        "experimental; blueprint cards at the bottom are specs, not code. "
        "Everything shipped runs in both daemon and appliance modes unless "
        "a card says otherwise. Click a category to filter, click again to "
        "show everything.</p>"
    )
    out.append("                </div>")
    out.append(
        '                <div class="legend reveal" role="group" aria-label="Filter features by category">'
    )
    for cat in sitelib.CATEGORIES:
        out.append(
            '                    <button class="cat-%s" data-cat="%s" aria-pressed="false" aria-label="Filter features by %s, %d features">%s <span class="legend-count" aria-hidden="true">%d</span></button>'
            % (cat, cat, cat.capitalize(), category_counts.get(cat, 0), cat.capitalize(), category_counts.get(cat, 0))
        )
    out.append("                </div>")
    out.append(
        '                <p id="feature-filter-status" class="feature-filter-status search-status" aria-live="polite"></p>'
    )
    out.append("            </section>")
    out.append("")

    for section in data["sections"]:
        out.append(render_section(section))
        out.append("")

    body = "\n".join(out)
    dest_text = body + "\n" + sitelib.page_foot(root)
    DEST.write_text(dest_text)
    sitelib.write_markdown_sibling(DEST, render_markdown(data, feature_count))
    print(
        "rendered %s -> %s (%d cards, + index.md)"
        % (
            DATA,
            DEST,
            feature_count
            + len(next(s for s in data["sections"] if s["id"] == "roadmap")["cards"]),
        )
    )


def main():
    data = models.validate_features(json.loads(DATA.read_text()))
    render(data)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
