#!/usr/bin/env -S uv run --with markdown python3
"""Render blog/posts/*.md (Zeledon weekly updates) into the site.

Usage:
    tools/render-blog.py

Reads every file in blog/posts/, parses its front matter and Zeledon-style
body (bold "**text**"-only lines act as section headers, Discord style --
not markdown # headings), and renders each to blog/<start-date>/index.html.
Then generates blog/index.html itself: a reverse-chronological list of every
post found, so the main page is always in sync with whatever .md files
exist on disk -- add, edit, or remove a post in blog/posts/ and re-run this
to update both the post page and the index. Same one-command-regenerates-
everything workflow as tools/render-docs.py.
"""

import pathlib
import re
import sys

import markdown

import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
POSTS_DIR = GH_PAGES / "blog" / "posts"
OUT_DIR = GH_PAGES / "blog"

HEADER_RE = re.compile(r"^\*\*(.+?)\*\*\s*$", re.MULTILINE)
FRONT_MATTER_RE = re.compile(r"^---\n(.*?)\n---\n(.*)$", re.DOTALL)
LIST_ITEM_RE = re.compile(r"^[-*]\s")


def ensure_blank_line_before_lists(text):
    """python-markdown (unlike CommonMark) won't start a list immediately
    after a paragraph with no blank line between -- Zeledon's Discord-style
    writing does exactly that ("intro:\\n- item"), so insert the blank line
    it needs."""
    lines = text.split("\n")
    out = []
    for i, line in enumerate(lines):
        is_item = LIST_ITEM_RE.match(line)
        if is_item and out and out[-1].strip() and not LIST_ITEM_RE.match(out[-1]):
            out.append("")
        out.append(line)
    return "\n".join(out)


def parse_front_matter(text):
    m = FRONT_MATTER_RE.match(text)
    if not m:
        return {}, text
    raw, body = m.group(1), m.group(2)
    meta = {}
    for line in raw.splitlines():
        if ":" not in line:
            continue
        key, _, value = line.partition(":")
        meta[key.strip()] = value.strip()
    return meta, body.strip()


def split_sections(body):
    """Return (title_marker, intro, [(header, section_body), ...])."""
    parts = HEADER_RE.split(body)
    # parts[0] is stray text before the first header (should be blank)
    if len(parts) < 2:
        return None, body, []
    title_marker = parts[1]
    intro = parts[2].strip() if len(parts) > 2 else ""
    sections = []
    i = 3
    while i < len(parts) - 1:
        sections.append((parts[i], parts[i + 1].strip()))
        i += 2
    return title_marker, intro, sections


def start_date(covers):
    # "2026-06-08 .. 2026-06-14" or "2026-06-25 21:10 .. 2026-07-01"
    return covers.split("..")[0].strip().split(" ")[0]


def render_post(meta, intro, sections, covers):
    is_draft = meta.get("status", "").upper().startswith("DRAFT")
    parts = []
    parts.append('            <section class="blog-post" aria-labelledby="post-title">')
    parts.append('                <div class="section-head reveal">')
    if is_draft:
        parts.append(
            '                    <span class="tag">Draft -- pending review</span>'
        )
    parts.append(
        '                    <h2 id="post-title">Week of %s</h2>' % start_date(covers)
    )
    if intro:
        parts.append("                    <p>%s</p>" % markdown.markdown(intro)[3:-4])
    parts.append("                </div>")
    parts.append("            </section>")

    parts.append('            <section class="blog-post reveal">')
    parts.append('                <div class="blog-grid">')
    for header, section_body in sections:
        html_body = markdown.markdown(
            ensure_blank_line_before_lists(section_body),
            extensions=["fenced_code", "sane_lists"],
        )
        parts.append(
            '                    <div class="blog-block" aria-label="%s">'
            % header.replace('"', "")
        )
        parts.append('                        <div class="md-content">')
        parts.append("                            <h3>%s</h3>" % header)
        parts.append("                            %s" % html_body)
        parts.append("                        </div>")
        parts.append("                    </div>")
    parts.append("                </div>")
    parts.append("            </section>")

    return "\n".join(parts)


def render_index(posts):
    # posts: list of dict(slug, covers, intro, is_draft)
    posts_sorted = sorted(posts, key=lambda p: p["start"], reverse=True)
    parts = []
    parts.append('            <section aria-labelledby="blog-title">')
    parts.append('                <div class="section-head reveal">')
    parts.append('                    <h2 id="blog-title">Ze weekly updates.</h2>')
    parts.append(
        "                    <p>%d weeks of shipped work, in Zeledon's voice, "
        "mined from git history. New weeks are also posted to Discord's "
        "<code>ze-news</code>.</p>" % len(posts_sorted)
    )
    parts.append("                </div>")
    parts.append('                <div class="cards reveal">')
    for i, p in enumerate(posts_sorted):
        cat = [
            "cat-operate",
            "cat-routing",
            "cat-automate",
            "cat-observe",
            "cat-secure",
            "cat-services",
            "cat-platform",
        ][i % 7]
        parts.append('                    <article class="card %s">' % cat)
        if p["is_draft"]:
            parts.append('                        <span class="chip mode">Draft</span>')
        parts.append(
            '                        <h3><a href="%s/">Week of %s</a></h3>'
            % (p["slug"], start_date(p["covers"]))
        )
        if p["intro"]:
            excerpt = markdown.markdown(p["intro"])[3:-4]
            parts.append("                        <p>%s</p>" % excerpt)
        parts.append("                    </article>")
    parts.append("                </div>")
    parts.append("            </section>")
    return "\n".join(parts)


def main():
    if not POSTS_DIR.exists():
        print("error: %s not found" % POSTS_DIR, file=sys.stderr)
        return 1

    post_files = sorted(POSTS_DIR.glob("*.md"))
    if not post_files:
        print("error: no posts found in %s" % POSTS_DIR, file=sys.stderr)
        return 1

    index_entries = []

    for f in post_files:
        text = f.read_text()
        meta, body = parse_front_matter(text)
        covers = meta.get("covers", f.stem.replace("..", " .. "))
        title_marker, intro, sections = split_sections(body)
        if title_marker is None:
            print("warning: no sections found in %s, skipping" % f, file=sys.stderr)
            continue

        slug = start_date(covers)
        dest_dir = OUT_DIR / slug
        dest_dir.mkdir(parents=True, exist_ok=True)
        dest = dest_dir / "index.html"

        desc = intro.replace("\n", " ")[:200] if intro else "Ze weekly update."
        title = "Week of %s" % start_date(covers)
        content = render_post(meta, intro, sections, covers)

        full_title = "%s - Ze Blog" % title
        dest.write_text(
            sitelib.page_head(
                full_title, desc, "../../", og_title=full_title, og_desc=desc
            )
            + content
            + "\n"
            + sitelib.page_foot("../../")
        )
        print("rendered %s -> %s" % (f.name, dest))

        index_entries.append(
            {
                "slug": slug,
                "start": slug,
                "covers": covers,
                "intro": intro,
                "is_draft": meta.get("status", "").upper().startswith("DRAFT"),
            }
        )

    index_dest = OUT_DIR / "index.html"
    index_content = render_index(index_entries)
    index_desc = "Ze weekly updates, mined from git history and posted to Discord."
    index_dest.write_text(
        sitelib.page_head(
            "Blog - Ze Blog",
            index_desc,
            "../",
            og_title="Blog - Ze Blog",
            og_desc=index_desc,
        )
        + index_content
        + "\n"
        + sitelib.page_foot("../")
    )
    print("rendered index -> %s (%d posts)" % (index_dest, len(index_entries)))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
