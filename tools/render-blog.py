#!/usr/bin/env -S uv run --with markdown python3
"""Render editorial blog articles from blog/posts/*.md.

Usage:
    tools/render-blog.py

The blog is for occasional editorial articles (deep dives, talk write-ups,
design notes), NOT the weekly changelog -- that moved to the Changes section
(see render-changes.py). Each article is a Markdown file in blog/posts/ with
front matter:

    ---
    title: From ExaBGP to a Network OS
    date: 2026-07-03
    description: One-line summary shown on the index and in the feed.
    ---

    Normal Markdown body, with ## headings, code fences, links.

It renders to blog/<slug>/index.html (slug = the file's front-matter `slug`
or its filename stem) plus blog/index.html (newest first). When there are no
articles yet, the index shows an intro and points readers at the weekly
changelog. A feed (blog/feed.xml) is written only once at least one article
exists.
"""

import html
import pathlib
import re
from datetime import date
from xml.sax.saxutils import escape

import markdown

import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
POSTS_DIR = GH_PAGES / "blog" / "posts"
OUT_DIR = GH_PAGES / "blog"
BLOG_URL = sitelib.SITE_BASE + "blog/"
DATE_DIR_RE = re.compile(r"^\d{4}-\d{2}-\d{2}$")


def slug_of(meta, f):
    return meta.get("slug", f.stem).strip()


def rfc822(iso):
    y, m, d = (int(x) for x in iso.split("-"))
    return date(y, m, d).strftime("%a, %d %b %Y 00:00:00 +0000")


def parse_articles():
    """Every blog/posts/*.md as {slug, title, date, description, body},
    newest first. Skips files without a title (treat as not ready)."""
    articles = []
    for f in sorted(POSTS_DIR.glob("*.md")):
        meta, body = sitelib.parse_blog_front_matter(f.read_text())
        title = meta.get("title")
        if not title:
            sitelib.warn("blog article %s has no title, skipping" % f.name)
            continue
        articles.append(
            {
                "slug": slug_of(meta, f),
                "title": title.strip(),
                "date": meta.get("date", "").strip(),
                "description": meta.get("description", "").strip(),
                "body": body,
            }
        )
    articles.sort(key=lambda a: a["date"], reverse=True)
    return articles


def render_article(a):
    body_html = markdown.markdown(a["body"], extensions=["tables", "fenced_code", "sane_lists"])
    lead = (
        '<time datetime="%s">%s</time>' % (a["date"], a["date"])
        if a["date"]
        else None
    )
    parts = [
        '            <section aria-labelledby="post-title">',
        sitelib.page_hero(
            a["title"],
            lead,
            "Article",
            h1_id="post-title",
            lead_html=True,
        ),
        '                <p class="post-back"><a href="../">&larr; All articles</a></p>',
        "            </section>",
    ]
    parts.append('            <section class="md-content reveal">')
    parts.append(body_html)
    parts.append("            </section>")
    return "\n".join(parts)


def render_article_markdown(a):
    parts = ["# %s" % a["title"], ""]
    if a["date"]:
        parts.append("*%s*" % a["date"])
        parts.append("")
    parts.append(a["body"].strip())
    return "\n".join(parts).strip() + "\n"


def render_index(articles):
    parts = ['            <section aria-labelledby="blog-title">']
    if articles:
        lead = (
            "Occasional articles on Ze: design notes, deep dives, and talk "
            "write-ups. For what shipped week by week, see the "
            '<a href="../changes/">changelog</a>.'
        )
    else:
        lead = (
            "Occasional articles on Ze: design notes, deep dives, and talk "
            "write-ups. None published yet. In the meantime, what shipped "
            'week by week is in the <a href="../changes/">changelog</a>, and '
            'the landmark features are on the <a href="../milestones/">'
            "Milestones</a> timeline."
        )
    parts.append(
        sitelib.page_hero(
            "The Ze blog.",
            lead,
            "Blog",
            h1_id="blog-title",
            lead_html=True,
        )
    )
    if articles:
        parts.append('                <div class="cards reveal">')
        cats = ["cat-operate", "cat-routing", "cat-automate", "cat-observe", "cat-secure", "cat-services", "cat-platform"]
        for i, a in enumerate(articles):
            parts.append('                    <article class="card card-post %s">' % cats[i % 7])
            parts.append('                        <h3><a href="%s/">%s</a></h3>' % (a["slug"], html.escape(a["title"])))
            if a["date"]:
                parts.append('                        <span class="chip">%s</span>' % html.escape(a["date"]))
            if a["description"]:
                parts.append("                        <p>%s</p>" % html.escape(a["description"]))
            parts.append('                        <span class="post-more">Read the article</span>')
            parts.append("                    </article>")
        parts.append("                </div>")
    parts.append("            </section>")
    return "\n".join(parts)


def render_index_markdown(articles):
    parts = ["# The Ze blog", ""]
    if articles:
        parts.append(
            "Occasional articles on Ze: design notes, deep dives, and talk "
            "write-ups. For what shipped week by week, see the "
            "[changelog](../changes/)."
        )
        parts.append("")
        for a in articles:
            line = "- [%s](%s/index.md)" % (a["title"], a["slug"])
            if a["date"]:
                line += " (%s)" % a["date"]
            if a["description"]:
                line += ": %s" % a["description"]
            parts.append(line)
    else:
        parts.append(
            "Occasional articles on Ze. None published yet. See the "
            "[changelog](../changes/) for what shipped week by week, and the "
            "[Milestones](../milestones/) timeline for the landmark features."
        )
    return "\n".join(parts).strip() + "\n"


def render_feed(articles):
    items = []
    for a in articles:
        if not a["date"]:
            continue
        link = "%s%s/" % (BLOG_URL, a["slug"])
        items.append(
            "\n".join(
                [
                    "        <item>",
                    "            <title>%s</title>" % escape(a["title"]),
                    "            <link>%s</link>" % link,
                    '            <guid isPermaLink="true">%s</guid>' % link,
                    "            <pubDate>%s</pubDate>" % rfc822(a["date"]),
                    "            <description>%s</description>" % escape(a["description"] or a["title"]),
                    "        </item>",
                ]
            )
        )
    built = rfc822(articles[0]["date"]) if articles and articles[0]["date"] else rfc822("2026-01-01")
    feed = "\n".join(
        [
            '<?xml version="1.0" encoding="UTF-8"?>',
            '<rss version="2.0">',
            "    <channel>",
            "        <title>Ze blog</title>",
            "        <link>%s</link>" % BLOG_URL,
            "        <description>Editorial articles on Ze.</description>",
            "        <language>en</language>",
            "        <lastBuildDate>%s</lastBuildDate>" % built,
            "".join(items),
            "    </channel>",
            "</rss>",
            "",
        ]
    )
    (OUT_DIR / "feed.xml").write_text(feed)
    print("rendered feed -> %s (%d items)" % (OUT_DIR / "feed.xml", len(items)))


def clean_stale_dirs(keep_slugs):
    """Remove leftover blog/<dir>/ pages that are not current articles --
    notably the old YYYY-MM-DD weekly dirs, now served from changes/."""
    for child in OUT_DIR.iterdir():
        if not child.is_dir() or child.name == "posts":
            continue
        if child.name in keep_slugs:
            continue
        for f in child.rglob("*"):
            if f.is_file():
                f.unlink()
        child.rmdir()
        print("removed stale blog dir -> %s" % child)


def main():
    POSTS_DIR.mkdir(parents=True, exist_ok=True)
    articles = parse_articles()

    for a in articles:
        dest_dir = OUT_DIR / a["slug"]
        dest_dir.mkdir(parents=True, exist_ok=True)
        dest = dest_dir / "index.html"
        desc = a["description"] or a["title"]
        full_title = "%s - Ze Blog" % a["title"]
        dest.write_text(
            sitelib.page_head(full_title, desc, "../../", og_title=full_title, og_desc=desc)
            + render_article(a)
            + "\n"
            + sitelib.page_foot("../../")
        )
        sitelib.write_markdown_sibling(dest, render_article_markdown(a))
        print("rendered article %s -> %s (+ index.md)" % (a["slug"], dest))

    clean_stale_dirs({a["slug"] for a in articles})

    index_desc = "Editorial articles on Ze: design notes, deep dives, and talk write-ups."
    extra = ""
    if articles:
        extra = (
            '        <link rel="alternate" type="application/rss+xml" '
            'title="Ze blog" href="feed.xml" />\n'
        )
    index_dest = OUT_DIR / "index.html"
    index_dest.write_text(
        sitelib.page_head("Blog - Ze", index_desc, "../", og_title="Blog - Ze", og_desc=index_desc, extra_head=extra)
        + render_index(articles)
        + "\n"
        + sitelib.page_foot("../")
    )
    sitelib.write_markdown_sibling(index_dest, render_index_markdown(articles))
    print("rendered blog index -> %s (%d articles, + index.md)" % (index_dest, len(articles)))

    if articles:
        render_feed(articles)
    else:
        stale_feed = OUT_DIR / "feed.xml"
        if stale_feed.exists():
            stale_feed.unlink()
            print("removed stale blog feed (no articles) -> %s" % stale_feed)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
