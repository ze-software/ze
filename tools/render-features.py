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

import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
DATA = GH_PAGES / "data" / "features.json"
DEST = GH_PAGES / "features" / "index.html"

CATEGORY_ORDER = [
    "operate",
    "routing",
    "services",
    "automate",
    "observe",
    "secure",
    "platform",
]

STATUS_LABELS = {
    "experimental": "Experimental",
    "aspiration": "Spec'd",
}


def render_chip(chip):
    cls = "chip mode" if chip["mode"] else "chip"
    return '<span class="%s">%s</span>' % (cls, chip["text"])


def render_card(card):
    extra = ""
    if card["status"]:
        extra += " " + card["status"]
    parts = [
        '<article class="card%s cat-%s" data-cat="%s">'
        % (extra, card["category"], card["category"])
    ]
    parts.append('<span class="cat">%s</span>' % card["category"].capitalize())
    if card["status"]:
        parts.append('<span class="status">%s</span>' % STATUS_LABELS[card["status"]])
    href = card["href"] if card["external"] else "../" + card["href"]
    link_attrs = ' target="_blank" rel="noopener"' if card["external"] else ""
    parts.append('<h3><a href="%s"%s>%s</a></h3>' % (href, link_attrs, card["title"]))
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
        % (section["id"], section["id"])
    )
    parts.append('                <div class="section-head reveal">')
    parts.append(
        '                    <h2 id="%s-title">%s</h2>'
        % (section["id"], section["heading"])
    )
    parts.append("                    <p>%s</p>" % section["lead"])
    parts.append("                </div>")
    if section["note"]:
        parts.append('                <div class="section-note reveal">')
        parts.append("                    <p>%s</p>" % section["note"])
        parts.append("                </div>")
    parts.append('                <div class="cards reveal">')
    for card in section["cards"]:
        parts.append("                    " + render_card(card))
    parts.append("                </div>")
    parts.append("            </section>")
    return "\n".join(parts)


FILTER_SCRIPT = """        <script>
            document.addEventListener("DOMContentLoaded", function () {
                var buttons = document.querySelectorAll(".legend button");
                var cards = document.querySelectorAll(".card[data-cat]");
                var sections = document.querySelectorAll("section[data-cards]");

                function applyFilter(cat) {
                    cards.forEach(function (card) {
                        card.classList.toggle(
                            "filtered-out",
                            cat !== null && card.dataset.cat !== cat,
                        );
                    });
                    sections.forEach(function (section) {
                        var visible = section.querySelector(
                            ".card[data-cat]:not(.filtered-out)",
                        );
                        section.style.display = visible ? "" : "none";
                    });
                }

                buttons.forEach(function (btn) {
                    btn.addEventListener("click", function () {
                        var wasPressed =
                            btn.getAttribute("aria-pressed") === "true";
                        buttons.forEach(function (other) {
                            other.setAttribute("aria-pressed", "false");
                        });
                        if (wasPressed) {
                            applyFilter(null);
                        } else {
                            btn.setAttribute("aria-pressed", "true");
                            applyFilter(btn.dataset.cat);
                        }
                    });
                });
            });
        </script>
"""


def render(data):
    root = "../"
    core = next(s for s in data["sections"] if s["id"] == "core")
    experimental = next(s for s in data["sections"] if s["id"] == "experimental")
    feature_count = len(core["cards"]) + len(experimental["cards"])

    title = "Features - Ze"
    desc = (
        "Every feature Ze ships and the spec'd roadmap ahead, "
        "grouped by maturity and color-coded by category."
    )
    out = [sitelib.page_head(title, desc, root, og_title=title, og_desc=desc)]

    out.append('            <section aria-labelledby="features-title">')
    out.append('                <div class="section-head reveal">')
    out.append(
        '                    <h2 id="features-title">Every feature Ze ships.</h2>'
    )
    out.append(
        "                    <p>%d features plus a spec'd roadmap, color-coded by category.</p>"
        % feature_count
    )
    out.append("                </div>")
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
    for cat in CATEGORY_ORDER:
        out.append(
            '                    <button class="cat-%s" data-cat="%s" aria-pressed="false">%s</button>'
            % (cat, cat, cat.capitalize())
        )
    out.append("                </div>")
    out.append("            </section>")
    out.append("")

    for section in data["sections"]:
        out.append(render_section(section))
        out.append("")

    body = "\n".join(out)
    dest_text = body + "\n" + FILTER_SCRIPT + "\n" + sitelib.page_foot(root)
    DEST.write_text(dest_text)
    print(
        "rendered %s -> %s (%d cards)"
        % (
            DATA,
            DEST,
            feature_count
            + len(next(s for s in data["sections"] if s["id"] == "roadmap")["cards"]),
        )
    )


def main():
    data = json.loads(DATA.read_text())
    render(data)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
