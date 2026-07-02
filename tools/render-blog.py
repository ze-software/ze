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
from datetime import date
from xml.sax.saxutils import escape

import markdown

import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
POSTS_DIR = GH_PAGES / "blog" / "posts"
OUT_DIR = GH_PAGES / "blog"
BLOG_URL = sitelib.SITE_BASE + "blog/"
FEED_URL = BLOG_URL + "feed.xml"
RSS_HEAD = (
    '        <link rel="alternate" type="application/rss+xml" '
    'title="Ze weekly updates" href="feed.xml" />\n'
)

LIST_ITEM_RE = re.compile(r"^[-*]\s")


def rfc822(slug):
    """slug is a YYYY-MM-DD start date; format it as an RSS pubDate. Derived
    from the date itself, never the wall clock, so rebuilds are stable."""
    y, m, d = (int(x) for x in slug.split("-"))
    return date(y, m, d).strftime("%a, %d %b %Y 00:00:00 +0000")


def render_feed(entries):
    posts = sorted(
        (e for e in entries if not e["is_draft"]),
        key=lambda p: p["start"],
        reverse=True,
    )
    items = []
    for p in posts:
        slug = p["slug"]
        link = "%s%s/" % (BLOG_URL, slug)
        desc = " ".join(p["intro"].split()) if p["intro"] else "Ze weekly update."
        items.append(
            "\n".join(
                [
                    "        <item>",
                    "            <title>Week of %s</title>" % slug,
                    "            <link>%s</link>" % link,
                    '            <guid isPermaLink="true">%s</guid>' % link,
                    "            <pubDate>%s</pubDate>" % rfc822(slug),
                    "            <description>%s</description>" % escape(desc),
                    "        </item>",
                ]
            )
        )
    built = rfc822(posts[0]["slug"]) if posts else rfc822("2026-01-01")
    feed = "\n".join(
        [
            '<?xml version="1.0" encoding="UTF-8"?>',
            '<rss version="2.0">',
            "    <channel>",
            "        <title>Ze weekly updates</title>",
            "        <link>%s</link>" % BLOG_URL,
            "        <description>What shipped in Ze each week, in Zeledon's "
            "voice, mined from git history.</description>",
            "        <language>en</language>",
            "        <lastBuildDate>%s</lastBuildDate>" % built,
            "".join(items),
            "    </channel>",
            "</rss>",
            "",
        ]
    )
    (OUT_DIR / "feed.xml").write_text(feed)
    print("rendered feed -> %s (%d items)" % (OUT_DIR / "feed.xml", len(posts)))


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


parse_front_matter = sitelib.parse_blog_front_matter
split_sections = sitelib.split_blog_sections
start_date = sitelib.blog_start_date


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


def render_post_markdown(meta, intro, sections, covers):
    title = "Week of %s" % start_date(covers)
    if meta.get("status", "").upper().startswith("DRAFT"):
        title += " (Draft -- pending review)"
    parts = ["# %s" % title, ""]
    if intro:
        parts.append(intro.strip())
        parts.append("")
    for header, section_body in sections:
        parts.append("## %s" % header)
        parts.append("")
        parts.append(ensure_blank_line_before_lists(section_body).strip())
        parts.append("")
    return "\n".join(parts).strip() + "\n"


def render_index_markdown(posts):
    posts_sorted = sorted(posts, key=lambda p: p["start"], reverse=True)
    parts = [
        "# Ze weekly updates",
        "",
        "%d weeks of shipped work, in Zeledon's voice, mined from git "
        "history. New weeks are also posted to Discord's `ze-news`."
        % len(posts_sorted),
        "",
    ]
    for p in posts_sorted:
        title = "Week of %s" % start_date(p["covers"])
        if p["is_draft"]:
            title += " (Draft)"
        line = "- [%s](%s/index.md)" % (title, p["slug"])
        if p["intro"]:
            line += ": %s" % " ".join(p["intro"].split())
        parts.append(line)
    return "\n".join(parts).strip() + "\n"


def render_index(posts):
    # posts: list of dict(slug, covers, intro, is_draft)
    posts_sorted = sorted(posts, key=lambda p: p["start"], reverse=True)
    parts = []
    parts.append('            <section aria-labelledby="blog-title">')
    parts.append('                <div class="section-head reveal">')
    parts.append('                    <h2 id="blog-title">Ze weekly updates.</h2>')
    parts.append(
        '                    <p>%d weeks of shipped work, in <a href="../zeledon/">Zeledon</a>\'s voice, '
        "mined from git history. New weeks are also posted to Discord's "
        "<code>ze-news</code>. Subscribe by "
        '<a href="feed.xml">RSS</a>, or scan the terser '
        '<a href="../changes/">changelog</a>.</p>' % len(posts_sorted)
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
        parts.append('                    <article class="card card-post %s">' % cat)
        if p["is_draft"]:
            parts.append('                        <span class="chip mode">Draft</span>')
        parts.append(
            '                        <h3><a href="%s/">Week of %s</a></h3>'
            % (p["slug"], start_date(p["covers"]))
        )
        if p["intro"]:
            excerpt = markdown.markdown(p["intro"])[3:-4]
            parts.append("                        <p>%s</p>" % excerpt)
        parts.append(
            '                        <span class="post-more">Read the update</span>'
        )
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
            sitelib.warn("no sections found in %s, skipping" % f)
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
        sitelib.write_markdown_sibling(
            dest, render_post_markdown(meta, intro, sections, covers)
        )
        print("rendered %s -> %s (+ index.md)" % (f.name, dest))

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
            extra_head=RSS_HEAD,
        )
        + index_content
        + "\n"
        + sitelib.page_foot("../")
    )
    sitelib.write_markdown_sibling(index_dest, render_index_markdown(index_entries))
    print(
        "rendered index -> %s (%d posts, + index.md)" % (index_dest, len(index_entries))
    )
    render_feed(index_entries)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
