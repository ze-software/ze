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

import json
import pathlib
import re
import sys
import urllib.request

import markdown

NAV_CHEVRON = (
    '<svg viewBox="0 0 12 8" fill="none" aria-hidden="true">'
    '<path d="M1 1l5 5 5-5" stroke="currentColor" stroke-width="1.6" '
    'stroke-linecap="round" stroke-linejoin="round"/></svg>'
)

_GITHUB_STARS_FALLBACK = 39  # last known count, used if the API call fails
_github_stars_cache = None


def get_github_stars():
    """Live star count for ze-software/ze, fetched once per script run and
    cached (unauthenticated GitHub API allows 60 req/hour/IP -- a batch run
    over ~40 docs must not spend that fetching the same number 40 times).
    Falls back to the last known count on any network/API failure so a
    regeneration never hard-fails for lack of connectivity."""
    global _github_stars_cache
    if _github_stars_cache is not None:
        return _github_stars_cache
    try:
        req = urllib.request.Request(
            "https://api.github.com/repos/ze-software/ze",
            headers={"Accept": "application/vnd.github+json", "User-Agent": "ze-site-build"},
        )
        with urllib.request.urlopen(req, timeout=5) as resp:
            data = json.loads(resp.read())
        _github_stars_cache = int(data["stargazers_count"])
    except Exception as exc:
        print(
            "warning: could not fetch live GitHub star count (%s), using last known value %d"
            % (exc, _GITHUB_STARS_FALLBACK),
            file=sys.stderr,
        )
        _github_stars_cache = _GITHUB_STARS_FALLBACK
    return _github_stars_cache


def build_nav_badges():
    stars = get_github_stars()
    return (
        "                    <a\n"
        '                        class="nav-badge"\n'
        '                        href="https://discord.gg/3Sx4S2dYQ"\n'
        '                        target="_blank"\n'
        '                        rel="noopener"\n'
        '                        aria-label="Ze Discord"\n'
        '                        ><svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">'
        '<path d="M4 4h16a1 1 0 0 1 1 1v11a1 1 0 0 1-1 1H9l-5 4v-4H4a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1z"/>'
        "</svg>Discord</a\n"
        "                    >\n"
        "                    <a\n"
        '                        class="nav-badge"\n'
        '                        href="https://github.com/ze-software/ze"\n'
        '                        target="_blank"\n'
        '                        rel="noopener"\n'
        '                        aria-label="Ze on GitHub, %d stars"\n'
        '                        ><svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">'
        '<path d="M12 2l2.9 6.9 7.1.6-5.4 4.6 1.7 7-6.3-3.9-6.3 3.9 1.7-7L1 9.5l7.1-.6L12 2z"/>'
        "</svg>%d</a\n"
        "                    >\n"
    ) % (stars, stars)

def nav_item(root, href, icon, title, desc):
    return (
        '                        <a class="nav-dropdown-item" href="%s%s">\n'
        '                            <span class="nav-dropdown-icon">%s</span>\n'
        "                            <span><strong>%s</strong><small>%s</small></span>\n"
        "                        </a>\n"
    ) % (root, href, icon, title, desc)


def nav_dropdown(label, columns):
    out = ['                    <div class="nav-dropdown">\n']
    out.append(
        '                    <button class="nav-dropdown-trigger" type="button">%s\n'
        "                        %s\n"
        "                    </button>\n" % (label, NAV_CHEVRON)
    )
    out.append('                    <div class="nav-dropdown-panel">\n')
    for col in columns:
        out.append('                    <div class="nav-dropdown-col">\n')
        for entry in col:
            if isinstance(entry, str):
                out.append(
                    '                        <span class="nav-dropdown-label">%s</span>\n'
                    % entry
                )
            else:
                out.append(nav_item(*entry))
        out.append("                    </div>\n")
    out.append("                    </div>\n")
    out.append("                    </div>\n")
    return "".join(out)


def nav_dropdown_rooted(root, label, columns):
    rooted_columns = [
        [entry if isinstance(entry, str) else (root,) + entry for entry in col]
        for col in columns
    ]
    return nav_dropdown(label, rooted_columns)


def build_navblock(root):
    out = ['                <div class="nav-links">\n']
    out.append('                    <a href="%sindex.html#status">Status</a>\n' % root)
    out.append('                    <a href="%sindex.html#try">Try</a>\n' % root)
    out.append(
        nav_dropdown_rooted(
            root,
            "Project",
            [
                [
                    (
                        "features/",
                        "\U0001f9e9",
                        "Features",
                        "41 features, color-coded by category",
                    ),
                    (
                        "performance/",
                        "⚡",
                        "Performance",
                        "Measured BGP benchmarks, not claims",
                    ),
                    (
                        "compare/",
                        "⚖️",
                        "Compare",
                        "How Ze stacks up against BIRD, FRR, and more",
                    ),
                    (
                        "activity/",
                        "\U0001f4c8",
                        "Activity",
                        "A year of commits, at a glance",
                    ),
                ]
            ],
        )
    )
    out.append(
        nav_dropdown_rooted(
            root,
            "Labs",
            [
                [
                    (
                        "labs/bgp-interop/",
                        "\U0001f310",
                        "BGP Protocol Interop",
                        "Real FRR, BIRD, and GoBGP sessions",
                    ),
                    (
                        "labs/l2tp-interop/",
                        "\U0001f50c",
                        "L2TP PPP/NCP Interop",
                        "Ze as LNS vs real xl2tpd",
                    ),
                    (
                        "labs/pppoe-interop/",
                        "\U0001f4e1",
                        "PPPoE Interop",
                        "vs real accel-ppp access concentrator",
                    ),
                    (
                        "labs/ipsec-interop/",
                        "\U0001f512",
                        "IPsec / IKEv2 Interop",
                        "Ze as IKE initiator vs strongSwan",
                    ),
                ],
                [
                    (
                        "labs/vlan-qos/",
                        "\U0001f3f7️",
                        "VLAN QoS Wire-Level Proof",
                        "802.1p PCP tagging, actually on the wire",
                    ),
                    (
                        "labs/looking-glass-graph/",
                        "\U0001f52d",
                        "Looking Glass Graph Demo",
                        "Realistic UK topology, real external ASNs",
                    ),
                    (
                        "labs/appliance-install/",
                        "\U0001f4bf",
                        "Appliance Installer Evidence",
                        "HTTP/PXE, ISO, Ventoy, real boots",
                    ),
                    (
                        "labs/vpp-dataplane/",
                        "\U0001f680",
                        "VPP Dataplane Evidence",
                        "FIB, traffic, and firewall in a real VPP daemon",
                    ),
                ],
            ],
        )
    )
    out.append(
        nav_dropdown_rooted(
            root,
            "Docs",
            [
                [
                    "Getting started",
                    (
                        "docs/guide/quickstart/",
                        "\U0001f680",
                        "Quickstart",
                        "Two BGP peers in under 5 minutes",
                    ),
                    (
                        "docs/features/configuration/",
                        "⚙️",
                        "Configuration",
                        "YANG-modeled, one model for everything",
                    ),
                    (
                        "docs/features/cli-commands/",
                        "⌨️",
                        "CLI Commands",
                        "SSH CLI with diff, commit, and history",
                    ),
                    (
                        "docs/architecture/",
                        "\U0001f3d7️",
                        "Architecture",
                        "How the pieces fit together",
                    ),
                ],
                [
                    "Deep dives",
                    (
                        "docs/features/mcp-integration/",
                        "\U0001f916",
                        "MCP Integration",
                        "AI-assisted operations",
                    ),
                    (
                        "docs/features/web-interface/",
                        "\U0001f5a5️",
                        "Web Interface",
                        "HTMX config editor and admin panel",
                    ),
                    (
                        "docs/guide/vpp/",
                        "\U0001f4e6",
                        "VPP Dataplane",
                        "FIB, traffic, and firewall via VPP",
                    ),
                    (
                        "docs/guide/benchmarking/",
                        "\U0001f4ca",
                        "Benchmarking",
                        "ze-perf architecture, flags, JSON output",
                    ),
                ],
            ],
        )
    )
    out.append('                    <a href="%sblog/">Blog</a>\n' % root)
    out.append('                    <a href="%stalks/">Talks</a>\n' % root)
    out.append(build_nav_badges())
    out.append("                </div>")
    return "".join(out)


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


HEAD = """<!doctype html>
<html lang="en">
    <head>
        <meta charset="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>{title} - Ze Blog</title>
        <meta name="description" content="{desc}" />
        <meta property="og:title" content="{title} - Ze Blog" />
        <meta property="og:description" content="{desc}" />
        <meta property="og:type" content="website" />
        <link rel="icon" href="{root}assets/ze.svg" type="image/svg+xml" />
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
        <link
            href="https://fonts.googleapis.com/css2?family=Poppins:wght@300;400;600;700;800&family=Lato:wght@300;400;700&display=swap"
            rel="stylesheet"
        />
        <link rel="stylesheet" href="{root}assets/site.css" />
    </head>
    <body>
        <header class="site-header">
            <nav class="nav" aria-label="Main navigation">
                <a class="brand" href="{root}index.html#top" aria-label="Ze home">
                    <img src="{root}assets/ze.svg" alt="" width="32" height="32" />
                    <span>Ze</span>
                </a>
{navblock}
            </nav>
        </header>

        <main id="top">
"""

FOOT = """        </main>

        <script src="{root}assets/site.js" defer></script>

        <footer>
            <div class="footer-inner">
                <span>Ze is AGPLv3 open source.</span>
                <div class="footer-links">
                    <a
                        href="https://github.com/ze-software/ze"
                        target="_blank"
                        rel="noopener"
                        >GitHub</a
                    >
                    <a
                        href="https://codeberg.org/thomas-mangin/ze"
                        target="_blank"
                        rel="noopener"
                        >Codeberg</a
                    >
                    <a
                        href="https://github.com/ze-software/ze/issues"
                        target="_blank"
                        rel="noopener"
                        >Issues</a
                    >
                    <a
                        href="https://discord.gg/3Sx4S2dYQ"
                        target="_blank"
                        rel="noopener"
                        >Discord</a
                    >
                    <a href="{root}style-guide/">Style Guide</a>
                </div>
            </div>
        </footer>
    </body>
</html>
"""


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

        dest.write_text(
            HEAD.format(
                title=title, desc=desc, root="../../", navblock=build_navblock("../../")
            )
            + content
            + "\n"
            + FOOT.format(root="../../")
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
    index_dest.write_text(
        HEAD.format(
            title="Blog",
            desc="Ze weekly updates, mined from git history and posted to Discord.",
            root="../",
            navblock=build_navblock("../"),
        )
        + index_content
        + "\n"
        + FOOT.format(root="../")
    )
    print("rendered index -> %s (%d posts)" % (index_dest, len(index_entries)))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
