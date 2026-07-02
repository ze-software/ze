#!/usr/bin/env -S uv run --with markdown python3
"""Render a markdown file into a site-shell-wrapped HTML page.

Usage:
    tools/render-doc.py <source.md> <dest/index.html> [--root ../]

The destination is wrapped in the same header/nav/footer markup as every
other gh-pages page and linked against assets/site.css. Re-run this after
editing the source markdown to refresh the published page -- same workflow
as presentations/tools/bundle-html.py for presentation content.
"""

import argparse
import json
import pathlib
import posixpath
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
            headers={
                "Accept": "application/vnd.github+json",
                "User-Agent": "ze-site-build",
            },
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


HEAD = """<!doctype html>
<html lang="en">
    <head>
        <meta charset="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>{title} - Ze</title>
        <meta name="description" content="{desc}" />
        <meta property="og:title" content="{title} - Ze" />
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
            <section class="md-content reveal">
"""

FOOT = """            </section>
        </main>

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


def first_h1(md_text):
    match = re.search(r"^#\s+(.+)$", md_text, re.MULTILINE)
    return match.group(1).strip() if match else "Ze"


TD_RE = re.compile(r"<td([^>]*)>((?:(?!</td>).)*)</td>", re.S)
TAG_RE = re.compile(r"<[^>]+>")
CELL_CLASSES = (
    (re.compile(r"^yes\b", re.I), "cell-yes"),
    (re.compile(r"^no\b", re.I), "cell-no"),
    (re.compile(r"^partial\b", re.I), "cell-partial"),
    (re.compile(r"^n/a$", re.I), "cell-na"),
)
CELL_SYMBOLS = {
    "cell-yes": "✓",
    "cell-no": "✕",
    "cell-partial": "∿",
}


def colorcode_cells(body_html):
    """Tag Yes/No/Partial/N-A table cells with classes so site.css can
    color-code them for scanning, and collapse Yes/No/Partial to just their
    symbol -- the color and the icon already carry the meaning, the word
    only adds width."""

    def repl(m):
        attrs, inner = m.group(1), m.group(2)
        if "class=" in attrs:
            return m.group(0)
        text = TAG_RE.sub("", inner).strip()
        for pattern, cls in CELL_CLASSES:
            if pattern.match(text):
                symbol = CELL_SYMBOLS.get(cls)
                content = symbol if symbol else inner
                return '<td%s class="%s">%s</td>' % (attrs, cls, content)
        return m.group(0)

    return TD_RE.sub(repl, body_html)


HREF_RE = re.compile(r'href="([^"]*)"')


def rewrite_doc_links(body_html, doc_rel, manifest, dest_rel_dir):
    """Rewrite relative .md links: local site path if the target is also
    published (per manifest), GitHub fallback otherwise. doc_rel is the
    source's path relative to docs/ (e.g. "features/configuration.md").
    dest_rel_dir is this page's own output directory relative to the site
    root (e.g. "docs/features/configuration"), used to compute the local
    relative path back."""
    source_dir = posixpath.dirname(doc_rel)

    def repl(m):
        href = m.group(1)
        if href.startswith(("http://", "https://", "mailto:", "#")):
            return m.group(0)
        path_part, sep, fragment = href.partition("#")
        if not path_part:
            return m.group(0)
        target_doc_rel = posixpath.normpath(posixpath.join(source_dir, path_part))
        if path_part.endswith("/"):
            new_href = (
                "https://github.com/ze-software/ze/tree/main/docs/%s" % target_doc_rel
            )
            return 'href="%s" target="_blank" rel="noopener"' % new_href
        if target_doc_rel in manifest:
            target_dest_dir = manifest[target_doc_rel]
            rel = posixpath.relpath(target_dest_dir, dest_rel_dir) + "/"
            new_href = rel + (("#" + fragment) if sep else "")
            return 'href="%s"' % new_href
        new_href = (
            "https://github.com/ze-software/ze/blob/main/docs/%s" % target_doc_rel
        )
        if sep:
            new_href += "#" + fragment
        return 'href="%s" target="_blank" rel="noopener"' % new_href

    return HREF_RE.sub(repl, body_html)


def render(
    source, dest, root, desc, manifest=None, doc_rel=None, dest_rel_dir=None, cat=None
):
    md_text = source.read_text()
    title = first_h1(md_text)
    body_html = markdown.markdown(
        md_text, extensions=["tables", "fenced_code", "sane_lists"]
    )
    body_html = colorcode_cells(body_html)
    if manifest is not None:
        body_html = rewrite_doc_links(body_html, doc_rel, manifest, dest_rel_dir)
    navblock = build_navblock(root)
    section_class = "md-content reveal cat-%s" % cat if cat else "md-content reveal"
    head = HEAD.format(title=title, desc=desc, root=root, navblock=navblock)
    head = head.replace(
        '<section class="md-content reveal">',
        '<section class="%s">' % section_class,
    )
    dest.parent.mkdir(parents=True, exist_ok=True)
    dest.write_text(head + body_html + "\n" + FOOT.format(root=root))
    print("rendered %s -> %s" % (source, dest))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=pathlib.Path)
    parser.add_argument("dest", type=pathlib.Path)
    parser.add_argument("--root", default="../", help="relative path back to site root")
    parser.add_argument("--desc", default="Ze documentation.", help="meta description")
    parser.add_argument(
        "--manifest",
        type=pathlib.Path,
        help="JSON map of docs-relative .md path -> published dest dir, "
        "for rewriting cross-doc links (local if published, GitHub otherwise)",
    )
    parser.add_argument(
        "--doc-rel",
        help="source path relative to docs/, e.g. features/configuration.md",
    )
    parser.add_argument(
        "--dest-rel-dir",
        help="this page's own output directory relative to the site root, "
        "e.g. docs/features/configuration",
    )
    parser.add_argument(
        "--cat",
        choices=[
            "operate",
            "routing",
            "automate",
            "observe",
            "secure",
            "services",
            "platform",
        ],
        help="topic category, colors the h1 per the site's color convention "
        "(same seven hues as the Features category legend)",
    )
    args = parser.parse_args()

    if not args.source.exists():
        print("error: source not found: %s" % args.source, file=sys.stderr)
        return 1

    manifest = json.loads(args.manifest.read_text()) if args.manifest else None
    render(
        args.source,
        args.dest,
        args.root,
        args.desc,
        manifest=manifest,
        doc_rel=args.doc_rel,
        dest_rel_dir=args.dest_rel_dir,
        cat=args.cat,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
