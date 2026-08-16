#!/usr/bin/env -S uv run python3
"""Render talks/index.html from data/talks.json.

Usage:
    tools/render-talks.py

Every talk card is data (data/talks.json), not hand-authored HTML -- add a
talk there (plus its talks/<slug>/ directory, built separately via
presentations/tools/bundle-html.py) and re-run this to publish it.
"""

import datetime
import json
import pathlib

import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
DATA = GH_PAGES / "data" / "talks.json"
DEST = GH_PAGES / "talks" / "index.html"


def display_date(iso_date):
    return datetime.date.fromisoformat(iso_date).strftime("%-d %B %Y")


def render_talk(talk):
    return """                    <article class="audience-card">
                        <a href="{slug}/" class="talk-link">
                            <h3>{venue}</h3>
                            <p>{title}</p>
                            <p class="talk-date">{date}</p>
                        </a>
                        <p class="talk-alt"><a href="{slug}/index-inlined.html" download>Download standalone HTML deck</a></p>
                    </article>""".format(
        slug=talk["slug"],
        venue=talk["venue"],
        title=talk["title"],
        date=display_date(talk["date"]),
    )


def render(talks):
    root = "../"
    title = "Talks - Ze"
    desc = "Talks and presentations about Ze."
    out = [sitelib.page_head(title, desc, root, og_title=title, og_desc=desc)]
    out.append('            <section id="talks" aria-labelledby="talks-title">')
    out.append(
        sitelib.page_hero(
            "Talks and presentations.",
            "Sharing Ze with the community.",
            "Community",
            h1_id="talks-title",
        )
    )
    out.append('                <div class="audience reveal">')
    talks_sorted = sorted(talks, key=lambda t: t["date"], reverse=True)
    for talk in talks_sorted:
        out.append(render_talk(talk))
    out.append("                </div>")
    out.append("            </section>")
    body = "\n".join(out)
    DEST.write_text(body + "\n" + sitelib.page_foot(root))
    print("rendered %s -> %s" % (DATA, DEST))

    md_parts = ["# Talks and presentations.", "", "Sharing Ze with the community.", ""]
    for talk in talks_sorted:
        md_parts.append("## %s" % talk["venue"])
        md_parts.append("")
        md_parts.append("%s -- %s" % (talk["title"], display_date(talk["date"])))
        md_parts.append("")
        md_parts.append("[Watch](%stalks/%s/)" % (sitelib.SITE_BASE, talk["slug"]))
        md_parts.append(
            "[Download standalone HTML deck](%stalks/%s/index-inlined.html)"
            % (sitelib.SITE_BASE, talk["slug"])
        )
        md_parts.append("")
    sitelib.write_markdown_sibling(DEST, "\n".join(md_parts).strip() + "\n")
    print("rendered %s -> %s/index.md" % (DATA, DEST.parent))


def main():
    data = json.loads(DATA.read_text())
    render(data["talks"])
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
