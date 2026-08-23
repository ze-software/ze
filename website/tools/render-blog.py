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
    author: Thomas Mangin
    description: One-line summary shown on the index and in the feed.
    ---

Every article carries a byline: `author` is required, and a missing one is a
build warning rather than a silently anonymous page.

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
# The article sources and their parser live in sitelib, so the homepage
# "what's new" band and these pages can never disagree about what the newest
# article is.
POSTS_DIR = sitelib.ARTICLES_DIR
OUT_DIR = GH_PAGES / "blog"
BLOG_URL = sitelib.SITE_BASE + "blog/"
DATE_DIR_RE = re.compile(r"^\d{4}-\d{2}-\d{2}$")

BLOG_INDEX_INTRO = (
    "Design notes, deep dives, and engineering essays on how Ze is built."
)


def blog_index_lead(changelog_link, milestones_link=None):
    if milestones_link is not None:
        return "%s None published yet. In the meantime, see the %s and the %s." % (
            BLOG_INDEX_INTRO,
            changelog_link,
            milestones_link,
        )
    return "%s For week-by-week shipping notes, read the %s." % (
        BLOG_INDEX_INTRO,
        changelog_link,
    )


def rfc822(iso):
    y, m, d = (int(x) for x in iso.split("-"))
    return date(y, m, d).strftime("%a, %d %b %Y 00:00:00 +0000")

def article_asset(root, path):
    if not path:
        return ""
    if path.startswith(("http://", "https://", "/")):
        return path
    return root + path.lstrip("/")


def render_article_meta(a):
    bits = []
    if a["date"]:
        bits.append(
            '<time datetime="%s">%s</time>'
            % (html.escape(a["date"], quote=True), html.escape(a["date"]))
        )
    if a["author"]:
        bits.append("<span>by %s</span>" % html.escape(a["author"]))
    if not bits:
        return ""
    return '                <div class="blog-article-meta">%s</div>' % "".join(bits)


def render_key_points(points):
    if not points:
        return ""
    out = [
        '            <aside class="blog-key-points reveal" aria-label="Key points">',
        '                <p class="blog-key-points-label">Key points</p>',
        "                <ul>",
    ]
    for point in points:
        out.append("                    <li>%s</li>" % html.escape(point))
    out.extend(["                </ul>", "            </aside>"])
    return "\n".join(out)

def article_sections(tokens):
    sections = []

    def collect(items):
        for token in items:
            if token["level"] == 2:
                sections.append({"id": token["id"], "title": token["name"]})
            collect(token.get("children", []))

    collect(tokens)
    return sections


def render_article_body(body):
    renderer = markdown.Markdown(
        extensions=["tables", "fenced_code", "sane_lists", "toc"]
    )
    body_html = renderer.convert(body)
    return body_html, article_sections(renderer.toc_tokens)


def render_article_toc(sections):
    if len(sections) < 3:
        return ""
    out = [
        '            <nav class="blog-article-toc reveal" aria-label="Article sections">',
        '                <p class="blog-article-toc-label">In this article</p>',
        "                <ol>",
    ]
    for section in sections:
        out.append(
            '                    <li><a href="#%s">%s</a></li>'
            % (
                html.escape(section["id"], quote=True),
                html.escape(section["title"]),
            )
        )
    out.extend(["                </ol>", "            </nav>"])
    return "\n".join(out)


def render_blog_image(a, root, loading, tag, classes):
    image = a.get("image")
    if not image:
        return ""
    alt = a.get("image_alt") or a["title"]
    dark_image = a.get("image_dark")
    if not dark_image:
        return (
            '<%s class="%s"><img src="%s" alt="%s" loading="%s" '
            'decoding="async" /></%s>'
            % (
                tag,
                classes,
                html.escape(article_asset(root, image), quote=True),
                html.escape(alt, quote=True),
                loading,
                tag,
            )
        )
    return (
        '<%s class="blog-theme-image has-dark %s" role="img" aria-label="%s">'
        '<img class="blog-theme-image-light" src="%s" alt="" loading="%s" '
        'decoding="async" />'
        '<img class="blog-theme-image-dark" src="%s" alt="" loading="%s" '
        'decoding="async" /></%s>'
        % (
            tag,
            classes,
            html.escape(alt, quote=True),
            html.escape(article_asset(root, image), quote=True),
            loading,
            html.escape(article_asset(root, dark_image), quote=True),
            loading,
            tag,
        )
    )


def render_article_visual(a, root):
    return render_blog_image(
        a, root, "eager", "figure", "blog-article-visual reveal"
    )


def article_lead(a):
    return a.get("deck") or a.get("description") or ""

def render_article(a):
    body = sitelib.substitute_number_tokens(a["body"], html_spans=True)
    body_html, sections = render_article_body(body)
    lead = article_lead(a) or None
    shell_class = "blog-article-shell"
    if a.get("image"):
        shell_class += " has-visual"
    parts = [
        '            <section class="%s" aria-labelledby="post-title">'
        % shell_class,
        sitelib.page_hero(
            a["title"],
            lead,
            "Article",
            h1_id="post-title",
            classes="journey-hero blog-article-hero reveal",
        ),
        render_article_meta(a),
        render_article_visual(a, "../../"),
        '                <p class="post-back"><a href="../">&larr; All articles</a></p>',
        "            </section>",
    ]
    key_points = render_key_points(a.get("key_points", []))
    if key_points:
        parts.append(key_points)
    toc = render_article_toc(sections)
    if toc:
        parts.append(toc)
    # Articles are read top to bottom: no column selector on their tables and no
    # copy button on their code blocks.
    parts.append(
        '            <section class="md-content blog-article-content reveal" data-table-columns="off" data-code-copy="off">'
    )
    parts.append(body_html)
    parts.append("            </section>")
    return "\n".join(parts)


def render_article_markdown(a):
    parts = ["# %s" % a["title"], ""]
    byline = " ".join(
        x for x in (a["date"], "by %s" % a["author"] if a["author"] else "") if x
    )
    if byline:
        parts.append("*%s*" % byline)
        parts.append("")
    if a.get("deck"):
        parts.append(a["deck"])
        parts.append("")
    if a.get("image"):
        alt = a.get("image_alt") or a["title"]
        parts.append("![%s](%s)" % (alt, article_asset("../../", a["image"])))
        parts.append("")
    points = a.get("key_points", [])
    if points:
        parts.append("## Key points")
        parts.append("")
        for point in points:
            parts.append("- %s" % point)
        parts.append("")
    parts.append(sitelib.substitute_number_tokens(a["body"]).strip())
    return "\n".join(parts).strip() + "\n"


def render_index(articles):
    parts = ['            <section class="blog-index" aria-labelledby="blog-title">']
    if articles:
        lead = blog_index_lead(
            '<a href="../project/changes/">changelog</a>'
        )
    else:
        lead = blog_index_lead(
            '<a href="../project/changes/">changelog</a>',
            '<a href="../project/milestones/">Milestones timeline</a>',
        )
    parts.append(
        sitelib.page_hero(
            "The Ze blog.",
            lead,
            "Blog",
            h1_id="blog-title",
            lead_html=True,
            classes="journey-hero blog-index-hero reveal",
        )
    )
    if articles:
        parts.append('                <div class="blog-list reveal">')
        tones = sitelib.PRESENTATION_TONES
        for i, a in enumerate(articles):
            image = a.get("image")
            media_class = " has-media" if image else ""
            parts.append(
                '                    <article class="card card-post blog-card%s tone-%s">'
                % (media_class, tones[i % len(tones)])
            )
            if a["date"]:
                parts.append(
                    '                        <div class="blog-card-meta"><time datetime="%s">%s</time><span>Article</span></div>'
                    % (html.escape(a["date"], quote=True), html.escape(a["date"]))
                )
            if image:
                parts.append(
                    render_blog_image(
                        a, "../", "lazy", "div", "blog-card-media"
                    )
                )
            parts.append('                        <h3><a href="%s/">%s</a></h3>' % (a["slug"], html.escape(a["title"])))
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
        parts.append(blog_index_lead("[changelog](../project/changes/)"))
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
            blog_index_lead(
                "[changelog](../project/changes/)",
                "[Milestones timeline](../project/milestones/)",
            )
        )
    return "\n".join(parts).strip() + "\n"


def render_feed(articles):
    items = []
    for a in articles:
        if not a["date"]:
            continue
        link = "%s%s/" % (BLOG_URL, a["slug"])
        item = [
            "        <item>",
            "            <title>%s</title>" % escape(a["title"]),
            "            <link>%s</link>" % link,
            '            <guid isPermaLink="true">%s</guid>' % link,
            "            <pubDate>%s</pubDate>" % rfc822(a["date"]),
        ]
        # RSS <author> wants an email address, so the byline goes in
        # dc:creator, which every reader understands and needs no address.
        if a["author"]:
            item.append("            <dc:creator>%s</dc:creator>" % escape(a["author"]))
        item.append(
            "            <description>%s</description>" % escape(a["description"] or a["title"])
        )
        item.append("        </item>")
        items.append("\n".join(item))
    built = rfc822(articles[0]["date"]) if articles and articles[0]["date"] else rfc822("2026-01-01")
    feed = "\n".join(
        [
            '<?xml version="1.0" encoding="UTF-8"?>',
            '<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/">',
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
    articles = sitelib.blog_articles()

    for a in articles:
        dest_dir = OUT_DIR / a["slug"]
        dest_dir.mkdir(parents=True, exist_ok=True)
        dest = dest_dir / "index.html"
        desc = a["description"] or a["title"]
        full_title = "%s - Ze Blog" % a["title"]
        head_extra = ""
        if a["author"]:
            head_extra = '        <meta name="author" content="%s" />\n' % html.escape(
                a["author"], quote=True
            )
        dest.write_text(
            sitelib.page_head(
                full_title,
                desc,
                "../../",
                og_title=full_title,
                og_desc=desc,
                extra_head=head_extra,
                wide=True,
            )
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
        sitelib.page_head(
            "Blog - Ze",
            index_desc,
            "../",
            og_title="Blog - Ze",
            og_desc=index_desc,
            extra_head=extra,
            wide=True,
        )
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
