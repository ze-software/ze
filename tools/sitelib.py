"""Shared chrome (nav, head, foot) for every gh-pages page generator.

Single source of truth for the ClickUp-style mega-menu (data/nav.json), the
live GitHub star badge, and the standard page head/foot. Every render-*.py
script imports this instead of carrying its own copy -- previously the nav
markup was duplicated verbatim in render-doc.py, render-blog.py, and
render-activity.py, which is how a bulk-patch bug once duplicated dropdown
content across 82 pages: three copies to keep in sync, one code path now.
"""

import json
import pathlib
import re
import sys
import urllib.request

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
DATA_DIR = GH_PAGES / "data"

NAV_CHEVRON = (
    '<svg viewBox="0 0 12 8" fill="none" aria-hidden="true">'
    '<path d="M1 1l5 5 5-5" stroke="currentColor" stroke-width="1.6" '
    'stroke-linecap="round" stroke-linejoin="round"/></svg>'
)

DISCORD_INVITE = "https://discord.gg/3Sx4S2dYQ"
GITHUB_REPO = "ze-software/ze"
CODEBERG_REPO = "https://codeberg.org/thomas-mangin/ze"

_GITHUB_STARS_FALLBACK = 39  # last known count, used if the API call fails
_github_stars_cache = None
_nav_data_cache = None


def get_github_stars():
    """Live star count for ze-software/ze, fetched once per process and
    cached (unauthenticated GitHub API allows 60 req/hour/IP -- a full site
    build touching ~40 docs must not spend that fetching the same number 40
    times). Falls back to the last known count on any network/API failure so
    a regeneration never hard-fails for lack of connectivity."""
    global _github_stars_cache
    if _github_stars_cache is not None:
        return _github_stars_cache
    try:
        req = urllib.request.Request(
            "https://api.github.com/repos/%s" % GITHUB_REPO,
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
        '                        href="%s"\n'
        '                        target="_blank"\n'
        '                        rel="noopener"\n'
        '                        aria-label="Ze Discord"\n'
        '                        ><svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">'
        '<path d="M4 4h16a1 1 0 0 1 1 1v11a1 1 0 0 1-1 1H9l-5 4v-4H4a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1z"/>'
        "</svg>Discord</a\n"
        "                    >\n"
        "                    <a\n"
        '                        class="nav-badge"\n'
        '                        href="https://github.com/%s"\n'
        '                        target="_blank"\n'
        '                        rel="noopener"\n'
        '                        aria-label="Ze on GitHub, %d stars"\n'
        '                        ><svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">'
        '<path d="M12 2l2.9 6.9 7.1.6-5.4 4.6 1.7 7-6.3-3.9-6.3 3.9 1.7-7L1 9.5l7.1-.6L12 2z"/>'
        "</svg>%d</a\n"
        "                    >\n"
    ) % (DISCORD_INVITE, GITHUB_REPO, stars, stars)


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


def load_nav_data():
    global _nav_data_cache
    if _nav_data_cache is None:
        _nav_data_cache = json.loads((DATA_DIR / "nav.json").read_text())
    return _nav_data_cache


def _columns_from_data(columns):
    out = []
    for col in columns:
        c = []
        for entry in col:
            if "label_only" in entry:
                c.append(entry["label_only"])
            else:
                c.append((entry["href"], entry["icon"], entry["title"], entry["desc"]))
        out.append(c)
    return out


def rooted_href(root, path):
    """root + path, except on index.html itself (root == "") a same-page
    "index.html#frag" link collapses to a bare "#frag" -- a hard navigation
    to your own URL still works, but the bare fragment is what lets
    site.js's smooth-scroll handle it instead of a full reload."""
    if root == "" and path.startswith("index.html#"):
        return path[len("index.html") :]
    return root + path


def build_navblock(root):
    data = load_nav_data()
    out = ['                <div class="nav-links">\n']
    for link in data["top_links"]:
        out.append(
            '                    <a href="%s">%s</a>\n'
            % (rooted_href(root, link["href"]), link["label"])
        )
    for dropdown in data["dropdowns"]:
        out.append(
            nav_dropdown_rooted(
                root, dropdown["label"], _columns_from_data(dropdown["columns"])
            )
        )
    for link in data["trailing_links"]:
        out.append(
            '                    <a href="%s">%s</a>\n'
            % (rooted_href(root, link["href"]), link["label"])
        )
    out.append(build_nav_badges())
    out.append("                </div>")
    return "".join(out)


NAV_LINKS_START_RE = re.compile(r'[ \t]*<div class="nav-links">')
DIV_OPEN_RE = re.compile(r"<div\b")
DIV_CLOSE_RE = re.compile(r"</div>")


def _find_balanced_div_end(text, div_start):
    """div_start is the index of the opening '<div' tag's '<'. Returns the
    index just past the matching '</div>', counting nested <div> depth so a
    naive first-</div>-wins regex can't truncate early (that was the root
    cause of the duplicated-dropdown-content bug from a prior bulk patch)."""
    open_end = text.index(">", div_start) + 1
    depth = 1
    pos = open_end
    while depth > 0:
        next_open = DIV_OPEN_RE.search(text, pos)
        next_close = DIV_CLOSE_RE.search(text, pos)
        if next_close is None:
            raise ValueError("unbalanced <div> after position %d" % div_start)
        if next_open and next_open.start() < next_close.start():
            depth += 1
            pos = next_open.end()
        else:
            depth -= 1
            pos = next_close.end()
    return pos


def patch_navblock(html_text, root):
    """Replace the <div class="nav-links">...</div> block in an already
    hand-authored page with a freshly built one, so pages with no dedicated
    generator (labs evidence pages, talks, style guide) still stay in sync
    with data/nav.json and the live star count."""
    m = NAV_LINKS_START_RE.search(html_text)
    if not m:
        raise ValueError('no <div class="nav-links"> found')
    div_start = html_text.index('<div class="nav-links">', m.start())
    end = _find_balanced_div_end(html_text, div_start)
    return html_text[: m.start()] + build_navblock(root) + html_text[end:]


PAGE_HEAD = """<!doctype html>
<html lang="en">
    <head>
        <meta charset="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>{title}</title>
        <meta name="description" content="{desc}" />
        <meta property="og:title" content="{og_title}" />
        <meta property="og:description" content="{og_desc}" />
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
                <a class="brand" href="{brand_href}" aria-label="Ze home">
                    <img src="{root}assets/ze.svg" alt="" width="32" height="32" />
                    <span>Ze</span>
                </a>
{navblock}
            </nav>
        </header>

        <main id="top">
"""

PAGE_FOOT = """        </main>

        <script src="{root}assets/site.js" defer></script>

        <footer>
            <div class="footer-inner">
                <span>Ze is AGPLv3 open source.</span>
                <div class="footer-links">
                    <a
                        href="https://github.com/{github_repo}"
                        target="_blank"
                        rel="noopener"
                        >GitHub</a
                    >
                    <a
                        href="{codeberg_repo}"
                        target="_blank"
                        rel="noopener"
                        >Codeberg</a
                    >
                    <a
                        href="https://github.com/{github_repo}/issues"
                        target="_blank"
                        rel="noopener"
                        >Issues</a
                    >
                    <a
                        href="{discord_invite}"
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


def page_head(title, desc, root, og_title=None, og_desc=None):
    return PAGE_HEAD.format(
        title=title,
        desc=desc,
        og_title=og_title if og_title is not None else title,
        og_desc=og_desc if og_desc is not None else desc,
        root=root,
        brand_href=rooted_href(root, "index.html#top"),
        navblock=build_navblock(root),
    )


def page_foot(root):
    return PAGE_FOOT.format(
        root=root,
        github_repo=GITHUB_REPO,
        codeberg_repo=CODEBERG_REPO,
        discord_invite=DISCORD_INVITE,
    )


BOLD_RE = re.compile(r"\*\*(.+?)\*\*")


def bold(text):
    """Convert Zeledon-style **bold** markers to <strong> for card bullet
    text pulled out of data/*.json -- avoids storing raw HTML in data."""
    return BOLD_RE.sub(r"<strong>\1</strong>", text)
