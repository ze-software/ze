#!/usr/bin/env -S uv run --with markdown python3
"""Render the weekly changelog from changes/posts/*.md.

Usage:
    tools/render-changes.py

The weekly Zeledon updates (what shipped each week, mined from git history and
posted to Discord's ze-news) live in changes/posts/*.md. This renderer owns
them end to end:

  data/changes.json           the structured index: each week parsed into its
                              intro line and its list of {topic, category},
                              extracted from the post's themed section headers
                              (the "**emoji Header**" lines). This is what lets
                              the index say more than the one-sentence intro.
  changes/<slug>/index.html   the full write-up for one week (themed sections,
                              the readable version)
  changes/index.html          a scannable index: one row per week (date, intro,
                              and category-colored topic chips) linking to the
                              week's page
  changes/feed.xml            the "Ze weekly updates" RSS feed

Each week is authored as 3-7 themed sections with a consistent emoji vocabulary
(STYLE.md standardises it: routing, security, appliance, observability, ...).
We turn those headers into typed topic chips, colored by the same seven site
categories as Features and Milestones, so the index summarises what a week
touched at a glance instead of leaning on the deliberately short intro line.

This used to live under blog/; the weekly changelog is not really a blog, so
it moved here and the blog is now for editorial articles (see render-blog.py).
"""

import html
import json
import pathlib
import re
import sys
from datetime import date
from xml.sax.saxutils import escape

import markdown

import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
POSTS_DIR = GH_PAGES / "changes" / "posts"
OUT_DIR = GH_PAGES / "changes"
INDEX_JSON = GH_PAGES / "data" / "changes.json"
CHANGES_URL = sitelib.SITE_BASE + "changes/"
RSS_HEAD = (
    '        <link rel="alternate" type="application/rss+xml" '
    'title="Ze weekly updates" href="../feed.xml" />\n'
)
INDEX_RSS_HEAD = (
    '        <link rel="alternate" type="application/rss+xml" '
    'title="Ze weekly updates" href="feed.xml" />\n'
)

DESC = "What shipped in Ze, week by week: the weekly updates, newest first."
LIST_ITEM_RE = re.compile(r"^[-*]\s")
LEADING_NONWORD_RE = re.compile(r"^[^0-9A-Za-z(]+")

parse_front_matter = sitelib.parse_blog_front_matter
split_sections = sitelib.split_blog_sections
start_date = sitelib.blog_start_date

TOPICS_JSON = GH_PAGES / "data" / "topics.json"


def load_topic_vocab():
    """The controlled tag vocabulary: {tag -> category}. See data/topics.json."""
    return json.loads(TOPICS_JSON.read_text())["tags"]


TAG_CATEGORY = load_topic_vocab()


# --- Topic classification --------------------------------------------------
#
# Each section header starts with an emoji whose meaning STYLE.md fixes; we map
# it to one of the seven site categories (assets/site.css cat-* colors). The
# emoji is author-controlled and reliable, so it wins; a keyword fallback on
# the header text covers the rare unmapped emoji, and one ambiguous emoji (the
# plug, used for both interfaces and APIs) is refined by keyword.

EMOJI_CAT = {
    "🛰": "routing", "🏗": "routing", "📜": "routing", "📄": "routing",
    "🔒": "secure", "🔐": "secure", "🧱": "secure",
    "🖥": "operate", "🧰": "operate", "📦": "operate",
    "📊": "observe", "🌪": "observe",
    "💿": "platform", "🧩": "platform", "🛠": "platform", "⚡": "platform",
    "🐛": "platform", "📡": "platform", "🧪": "platform", "🌐": "platform", "🔧": "platform",
    "🔌": "platform",
    "🔀": "automate", "🔄": "automate", "🔁": "automate",
    "📶": "services", "🏠": "services",
}

KEYWORD_CAT = [
    ("routing", ("bgp", "is-is", "isis", "ospf", "mpls", "rsvp", "ldp", "rpki", "bfd",
                 "route", "rib", "flowspec", "evpn", "srv6", "sr-policy", "best-path", "igp")),
    ("services", ("l2tp", "pppoe", "ppp ", "dhcp", "bng", "cpe", "class of service",
                  "cos", "qos", "subscriber", "radius", "access")),
    ("secure", ("security", "firewall", "ipsec", "ike", "auth", "aaa", "tacacs", "rbac",
                "ddos", "harden", "control-plane", "md5", "gtsm")),
    ("automate", ("api", "mcp", "rest", "grpc", "gnmi", "exabgp", "migration", "redistribut", "plugin")),
    ("observe", ("observab", "telemetr", "metric", "diagnostic", "flow export", "bmp",
                 "mrt", "chaos", "looking glass", "monitor")),
    ("operate", ("cli", "config", "editor", "command", "help", "yang")),
    ("platform", ("appliance", "installer", "vpp", "interface", "kernel", "provision",
                  "pxe", "under the hood", "performance", "storage", "build", "feature gate", "sysctl", "fleet")),
]

SKIP_TOPIC_RE = re.compile(r"coming up|drawing board|on the horizon|road ?map ahead", re.I)


def clean_header(header):
    """Strip the leading emoji (and any variation selector) so the header
    text becomes a plain chip label: '🛰️ IS-IS & MPLS' -> 'IS-IS & MPLS'."""
    return LEADING_NONWORD_RE.sub("", header).strip()


def lead_emoji(header):
    return header.split(" ")[0].replace("️", "")


def classify(header):
    text = header.lower()
    e = lead_emoji(header)
    if e == "🔌" and any(k in text for k in ("api", "rest", "grpc", "gnmi", "mcp")):
        return "automate"
    cat = EMOJI_CAT.get(e)
    if cat:
        return cat
    for c, kws in KEYWORD_CAT:
        if any(k in text for k in kws):
            return c
    return "platform"


def topics_of(sections):
    """The chip list for a week: one {label, category} per themed section,
    minus the forward-looking 'Coming up' section (planned, not shipped)."""
    topics = []
    for header, _body in sections:
        label = clean_header(header)
        if not label or SKIP_TOPIC_RE.search(label):
            continue
        topics.append({"label": label, "category": classify(header), "key": label})
    return topics


def topics_from_tags(meta, source):
    """The chip list for a week, from its front-matter `tags:` line and the
    controlled vocabulary in data/topics.json. Each tag maps to one of the
    eight site categories (color); a namespaced tag ('Presentation: LINX 126')
    is classified and filtered by its family (the part before the colon) but
    shown in full. Returns None if the post has no `tags:` line so the caller
    can fall back to the section-header heuristic. Unknown tags still render
    (neutral) but warn, so the vocabulary can't drift silently."""
    raw = meta.get("tags", "").strip()
    if not raw:
        return None
    topics = []
    for tag in (t.strip() for t in raw.split(",")):
        if not tag:
            continue
        family = tag.split(":", 1)[0].strip()
        category = TAG_CATEGORY.get(family)
        if category is None:
            sitelib.warn("%s: tag %r not in data/topics.json vocabulary" % (source, tag))
            category = "meta"
        topics.append({"label": tag, "category": category, "key": family})
    return topics


# --- Detail page (the full weekly write-up) --------------------------------

def rfc822(slug):
    y, m, d = (int(x) for x in slug.split("-"))
    return date(y, m, d).strftime("%a, %d %b %Y 00:00:00 +0000")


def ensure_blank_line_before_lists(text):
    lines = text.split("\n")
    out = []
    for line in lines:
        if LIST_ITEM_RE.match(line) and out and out[-1].strip() and not LIST_ITEM_RE.match(out[-1]):
            out.append("")
        out.append(line)
    return "\n".join(out)


def render_post(meta, intro, sections, covers):
    is_draft = meta.get("status", "").upper().startswith("DRAFT")
    parts = []
    parts.append('            <section class="blog-post" aria-labelledby="post-title">')
    if is_draft:
        parts.append('                <span class="tag">Draft -- pending review</span>')
    parts.append(
        sitelib.page_hero(
            "Week of %s" % start_date(covers),
            markdown.markdown(intro)[3:-4] if intro else None,
            "Weekly update",
            h1_id="post-title",
            lead_html=True,
        )
    )
    parts.append('                <p class="post-back"><a href="../">&larr; All weekly updates</a></p>')
    parts.append("            </section>")

    parts.append('            <section class="blog-post reveal">')
    parts.append('                <div class="blog-grid">')
    for header, section_body in sections:
        html_body = markdown.markdown(
            ensure_blank_line_before_lists(section_body),
            extensions=["fenced_code", "sane_lists"],
        )
        parts.append('                    <div class="blog-block" aria-label="%s">' % html.escape(header, quote=True))
        parts.append('                        <div class="md-content">')
        parts.append("                            <h2>%s</h2>" % html.escape(header))
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


# --- Index (the scannable list of weeks) -----------------------------------

INDEX_CSS = """        <style>
            .ch-list { display: grid; gap: 0.45rem; margin-top: 1.5rem; }
            .ch-week {
                display: block;
                padding: 0.95rem 1rem;
                border: 1px solid transparent;
                border-radius: 0.85rem;
                color: inherit;
                text-decoration: none;
                transition: background 160ms ease, border-color 160ms ease, box-shadow 160ms ease, transform 160ms ease;
            }
            .ch-week:hover,
            .ch-week:focus-visible {
                border-color: var(--line);
                background: rgba(255, 254, 254, 0.72);
                box-shadow: var(--clay);
                transform: translateY(-1px);
            }
            .ch-week:focus-visible {
                outline: 3px solid rgba(0, 159, 227, 0.22);
                outline-offset: 2px;
            }
            .ch-week.filtered-out { display: none; }
            .ch-filters { margin: 1.1rem 0 0.2rem; }
            .ch-empty { margin: 1.5rem 0; color: var(--muted); }
            .ch-empty.filtered-out { display: none; }
            .ch-head { display: flex; align-items: baseline; gap: 1rem; justify-content: flex-start; }
            .ch-head h2 { margin: 0; font-size: 1.12rem; letter-spacing: -0.01em; }
            .ch-week-title {
                color: var(--text);
                text-decoration: underline;
                text-decoration-color: var(--sky-chip);
                text-decoration-thickness: 0.16em;
                text-underline-offset: 0.18em;
            }
            .ch-week:hover .ch-week-title,
            .ch-week:focus-visible .ch-week-title { color: var(--sky-deep); text-decoration-color: var(--sky-base); }
            .ch-intro { margin: 0.3rem 0 0.65rem; color: var(--text); }
            .ch-chips { display: flex; flex-wrap: wrap; gap: 0.4rem; }
            .ch-chip {
                font-size: 0.72rem;
                font-weight: 600;
                line-height: 1.4;
                padding: 0.12rem 0.6rem;
                border-radius: 999px;
                background: var(--acc-tint, var(--bg-soft));
                color: var(--acc-deep, var(--muted));
                border: 1px solid var(--acc, var(--line));
                white-space: nowrap;
            }
            .ch-chip.cat-meta {
                background: var(--bg-soft);
                color: var(--muted);
                border-color: var(--line);
                border-style: dashed;
            }
            /* The meta bucket has no accent hue, so the shared .legend button
               style (white text over var(--acc)) renders invisible. Give the
               filter button a readable neutral treatment instead. */
            .ch-filters .cat-meta {
                background: var(--bg-soft);
                color: var(--muted);
                border-color: var(--line);
            }
            .ch-filters .cat-meta:hover { color: var(--text); }
            .ch-filters .cat-meta[aria-pressed="true"] {
                border-color: var(--muted);
                box-shadow: 0 0 0 2px var(--line);
            }
            .ch-draft { font-size: 0.7rem; font-weight: 700; text-transform: uppercase; letter-spacing: 0.05em; color: var(--acc-deep, var(--muted)); margin-left: 0.5rem; }
        </style>
"""

# Legend order matches the Features page (sitelib.CATEGORIES), with the
# neutral meta bucket last.
FILTER_ORDER = sitelib.CATEGORIES + ["meta"]


def week_categories(w):
    """The distinct categories a week touched, in canonical legend order."""
    present = {t["category"] for t in w["topics"]}
    return [c for c in FILTER_ORDER if c in present]


def render_index_html(weeks):
    present = {c for w in weeks for c in week_categories(w)}
    legend_cats = [c for c in FILTER_ORDER if c in present]

    out = ['            <section aria-labelledby="changes-title">']
    out.append('                <div class="section-head journey-hero reveal">')
    out.append('                    <span class="journey-eyebrow">Weekly updates</span>')
    out.append('                    <h1 id="changes-title">Changes.</h1>')
    out.append(
        '                    <p>What shipped in Ze, newest first: the weekly '
        "updates, mined from git history and posted to Discord's "
        "<code>ze-news</code>. Each week's chips are the areas it touched; "
        "click a category to show just the weeks that touched it, click again "
        "to show everything, or click a week for the full write-up. Ze is "
        'pre-release, so the configuration syntax can still change, and the '
        '<a href="../roadmap/">roadmap</a> tracks the path to a stable '
        "release. For the landmark features on a timeline, see "
        '<a href="../milestones/">Milestones</a>.</p>'
    )
    out.append("                </div>")
    out.append(
        '                <div class="legend ch-filters reveal" role="group" '
        'aria-label="Filter weeks by category">'
    )
    for cat in legend_cats:
        out.append(
            '                    <button class="cat-%s" data-cat="%s" aria-pressed="false">%s</button>'
            % (cat, cat, cat.capitalize())
        )
    out.append("                </div>")
    out.append('                <div class="ch-list reveal">')
    for w in weeks:
        slug = w["slug"]
        cats = " ".join(week_categories(w))
        out.append(
            '                    <a class="ch-week" data-cats="%s" href="%s/" aria-label="Read Week of %s">'
            % (html.escape(cats, quote=True), slug, slug)
        )
        out.append('                        <div class="ch-head">')
        draft = '<span class="ch-draft">pending review</span>' if w["is_draft"] else ""
        out.append(
            '                            <h2><span class="ch-week-title">Week of %s</span>%s</h2>'
            % (slug, draft)
        )
        out.append("                        </div>")
        if w["intro"]:
            out.append('                        <p class="ch-intro">%s</p>' % html.escape(" ".join(w["intro"].split())))
        if w["topics"]:
            out.append('                        <div class="ch-chips">')
            for t in w["topics"]:
                out.append(
                    '                            <span class="ch-chip cat-%s">%s</span>'
                    % (t["category"], html.escape(t["label"]))
                )
            out.append("                        </div>")
        out.append("                    </a>")
    out.append(
        '                    <p class="ch-empty filtered-out">No weeks in that '
        "category yet.</p>"
    )
    out.append("                </div>")
    out.append("            </section>")
    return "\n".join(out)


def render_index_markdown(weeks):
    parts = [
        "# Changes",
        "",
        "What shipped in Ze, newest first: the weekly updates, mined from git "
        "history and posted to Discord's `ze-news`. Each week lists the areas "
        "it touched; click a week for the full write-up. Ze is pre-release, so "
        "the configuration syntax can still change, and the "
        "[roadmap](../roadmap/) tracks the path to a stable release. For the "
        "landmark features on a timeline, see [Milestones](../milestones/).",
        "",
    ]
    for w in weeks:
        slug = w["slug"]
        title = "Week of %s" % slug
        if w["is_draft"]:
            title += " (pending review)"
        parts.append("## [%s](%s/index.md)" % (title, slug))
        parts.append("")
        if w["intro"]:
            parts.append(" ".join(w["intro"].split()))
            parts.append("")
        if w["topics"]:
            parts.append("Areas: " + ", ".join(t["label"] for t in w["topics"]))
            parts.append("")
    return "\n".join(parts).strip() + "\n"


def render_feed(weeks):
    live = [w for w in weeks if not w["is_draft"]]
    items = []
    for w in live:
        slug = w["slug"]
        link = "%s%s/" % (CHANGES_URL, slug)
        desc = " ".join(w["intro"].split()) if w["intro"] else "Ze weekly update."
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
    built = rfc822(live[0]["slug"]) if live else rfc822("2026-01-01")
    feed = "\n".join(
        [
            '<?xml version="1.0" encoding="UTF-8"?>',
            '<rss version="2.0">',
            "    <channel>",
            "        <title>Ze weekly updates</title>",
            "        <link>%s</link>" % CHANGES_URL,
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
    print("rendered feed -> %s (%d items)" % (OUT_DIR / "feed.xml", len(live)))


def clean_stale_week_dirs(keep_slugs):
    date_re = re.compile(r"^\d{4}-\d{2}-\d{2}$")
    for child in OUT_DIR.iterdir():
        if child.is_dir() and date_re.match(child.name) and child.name not in keep_slugs:
            for f in child.rglob("*"):
                if f.is_file():
                    f.unlink()
            child.rmdir()
            print("removed stale week dir -> %s" % child)


def main():
    if not POSTS_DIR.exists():
        print("error: %s not found" % POSTS_DIR, file=sys.stderr)
        return 1
    post_files = sorted(POSTS_DIR.glob("*.md"))
    if not post_files:
        print("error: no posts found in %s" % POSTS_DIR, file=sys.stderr)
        return 1

    weeks = []
    for f in post_files:
        meta, body = parse_front_matter(f.read_text())
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
        full_title = "Week of %s - Ze" % slug
        dest.write_text(
            sitelib.page_head(
                full_title,
                desc,
                "../../",
                og_title=full_title,
                og_desc=desc,
                extra_head=RSS_HEAD,
                page_key="changes/%s/" % slug,
            )
            + render_post(meta, intro, sections, covers)
            + "\n"
            + sitelib.page_foot("../../")
        )
        sitelib.write_markdown_sibling(dest, render_post_markdown(meta, intro, sections, covers))
        topics = topics_from_tags(meta, f.name)
        if topics is None:
            sitelib.warn(
                "%s: no `tags:` front matter; deriving chips from section headers. "
                "Add a curated `tags:` line (vocabulary: data/topics.json)." % f.name
            )
            topics = topics_of(sections)
        weeks.append(
            {
                "slug": slug,
                "intro": intro,
                "is_draft": meta.get("status", "").upper().startswith("DRAFT"),
                "topics": topics,
            }
        )

    weeks.sort(key=lambda w: w["slug"], reverse=True)
    clean_stale_week_dirs({w["slug"] for w in weeks})

    # The structured index -- each week's intro and typed topic chips, the
    # data that lets the index page and any other consumer summarise a week.
    INDEX_JSON.write_text(json.dumps(weeks, indent=2, ensure_ascii=False) + "\n")
    print("wrote %d weeks -> %s" % (len(weeks), INDEX_JSON))

    full_title = "Changes - Ze"
    head = sitelib.page_head(
        full_title,
        DESC,
        "../",
        og_title=full_title,
        og_desc=DESC,
        extra_head=INDEX_RSS_HEAD + INDEX_CSS,
        page_key="changes/",
    )
    out = OUT_DIR / "index.html"
    out.write_text(head + render_index_html(weeks) + "\n" + sitelib.page_foot("../"))
    sitelib.write_markdown_sibling(out, render_index_markdown(weeks))
    print("rendered changes -> %s (%d weeks, + detail pages, + index.md)" % (out, len(weeks)))
    render_feed(weeks)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
