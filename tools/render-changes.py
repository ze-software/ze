#!/usr/bin/env -S uv run --with markdown python3
"""Render a condensed "Changes" log from the weekly blog posts.

Usage:
    tools/render-changes.py

Reads the same blog/posts/*.md files render-blog.py does, and produces
changes/index.html (+ index.md): a terse, reverse-chronological digest of
what shipped each week, one line per topic, linking back to the full post.
It is deliberately denser than the blog index -- a changelog you scan, not a
narrative you read -- but shares the same parser (sitelib.parse_blog_front_matter
/ split_blog_sections) so the two can never disagree about what a week
contained. Re-run whenever a post is added or edited.
"""

import pathlib
import re
import sys

import markdown

import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
POSTS_DIR = GH_PAGES / "blog" / "posts"
OUT = GH_PAGES / "changes" / "index.html"

DESC = "What shipped in Ze, week by week: a condensed changelog of the weekly updates."
LEADING_NONWORD_RE = re.compile(r"^[^0-9A-Za-z]+")
LIST_MARKER_RE = re.compile(r"^[-*]\s+")


def clean_header(header):
    return LEADING_NONWORD_RE.sub("", header).strip()


def flatten_body(body):
    """Collapse a section body (prose or a bullet list) into one readable
    line: bullet markers dropped, lines joined with '; ', whitespace
    collapsed. Markdown emphasis/code markers are left intact -- the whole
    digest is rendered through python-markdown afterwards."""
    pieces = []
    for line in body.splitlines():
        line = line.strip()
        if not line:
            continue
        line = LIST_MARKER_RE.sub("", line)
        pieces.append(line)
    return "; ".join(pieces)


def build_markdown(posts):
    parts = [
        "# Changes",
        "",
        "What shipped in Ze, newest first. This is a condensed changelog "
        "mined from the same weekly updates as the [blog](../blog/); each "
        "entry links to the full write-up. Ze is pre-release, so the "
        "configuration syntax can still change: anything that affects an "
        "existing config is called out in the week it lands, and the "
        "[roadmap](../roadmap/) tracks the path to a stable release.",
        "",
    ]
    for p in posts:
        slug = p["slug"]
        title = "Week of %s" % slug
        if p["is_draft"]:
            title += " (pending review)"
        parts.append("## [%s](../blog/%s/)" % (title, slug))
        parts.append("")
        if p["intro"]:
            parts.append("*%s*" % " ".join(p["intro"].split()))
            parts.append("")
        for header, body in p["sections"]:
            summary = flatten_body(body)
            if summary:
                parts.append("- **%s:** %s" % (clean_header(header), summary))
            else:
                parts.append("- **%s**" % clean_header(header))
        parts.append("")
    return "\n".join(parts).strip() + "\n"


def main():
    if not POSTS_DIR.exists():
        print("error: %s not found" % POSTS_DIR, file=sys.stderr)
        return 1

    posts = []
    for f in sorted(POSTS_DIR.glob("*.md")):
        meta, body = sitelib.parse_blog_front_matter(f.read_text())
        covers = meta.get("covers", f.stem.replace("..", " .. "))
        _title_marker, intro, sections = sitelib.split_blog_sections(body)
        if not sections:
            continue
        posts.append(
            {
                "slug": sitelib.blog_start_date(covers),
                "intro": intro,
                "sections": sections,
                "is_draft": meta.get("status", "").upper().startswith("DRAFT"),
            }
        )
    posts.sort(key=lambda p: p["slug"], reverse=True)

    md_text = build_markdown(posts)
    body_html = markdown.markdown(
        md_text, extensions=["tables", "fenced_code", "sane_lists"]
    )
    full_title = "Changes - Ze"
    head = sitelib.page_head(full_title, DESC, "../", og_title=full_title, og_desc=DESC)
    head += '            <section class="md-content reveal">\n'
    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text(
        head + body_html + "\n            </section>\n" + sitelib.page_foot("../")
    )
    sitelib.write_markdown_sibling(OUT, md_text)
    print("rendered changes -> %s (%d weeks, + index.md)" % (OUT, len(posts)))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
