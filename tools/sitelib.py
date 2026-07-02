"""Shared chrome (nav, head, foot) for every gh-pages page generator.

Single source of truth for the ClickUp-style mega-menu (data/nav.json), the
live GitHub star badge, and the standard page head/foot. Every render-*.py
script imports this instead of carrying its own copy -- previously the nav
markup was duplicated verbatim in render-doc.py, render-blog.py, and
render-activity.py, which is how a bulk-patch bug once duplicated dropdown
content across 82 pages: three copies to keep in sync, one code path now.
"""

import html
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

# The site's seven feature categories, in display order. Shared by
# render-features.py (the legend/filter buttons) and render-index.py (the
# homepage's per-category links into that filter) so the two never disagree
# on which categories exist or what order they're shown in.
CATEGORIES = [
    "operate",
    "routing",
    "services",
    "automate",
    "observe",
    "secure",
    "platform",
]


def feature_counts_by_category():
    """Card count per category, core + experimental sections only (matches
    the "N features" count used elsewhere -- roadmap/aspiration cards
    aren't shipped features yet)."""
    features = json.loads((DATA_DIR / "features.json").read_text())
    core = next(s for s in features["sections"] if s["id"] == "core")
    experimental = next(s for s in features["sections"] if s["id"] == "experimental")
    counts = {cat: 0 for cat in CATEGORIES}
    for card in core["cards"] + experimental["cards"]:
        counts[card["category"]] += 1
    return counts


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


DISCORD_ICON_PATH = (
    "M524.531,69.836a1.5,1.5,0,0,0-.764-.7A485.065,485.065,0,0,0,404.081,32.03a1.816,"
    "1.816,0,0,0-1.923.91,337.461,337.461,0,0,0-14.9,30.6,447.848,447.848,0,0,0-134.426,"
    "0,309.541,309.541,0,0,0-15.135-30.6,1.89,1.89,0,0,0-1.924-.91A483.689,483.689,0,0,0,"
    "116.085,69.137a1.712,1.712,0,0,0-.788.676C39.068,183.651,18.186,294.69,28.43,404.354a"
    "2.016,2.016,0,0,0,.765,1.375A487.666,487.666,0,0,0,176.02,479.918a1.9,1.9,0,0,0,"
    "2.063-.676A348.2,348.2,0,0,0,208.12,430.4a1.86,1.86,0,0,0-1.019-2.588,321.173,321.173,"
    "0,0,1-45.868-21.853,1.885,1.885,0,0,1-.185-3.126c3.082-2.309,6.166-4.711,9.109-7.137a"
    "1.819,1.819,0,0,1,1.9-.256c96.229,43.917,200.41,43.917,295.5,0a1.812,1.812,0,0,1,"
    "1.924.233c2.944,2.426,6.027,4.851,9.132,7.16a1.884,1.884,0,0,1-.162,3.126,301.407,"
    "301.407,0,0,1-45.89,21.83,1.875,1.875,0,0,0-1,2.611,391.055,391.055,0,0,0,30.014,"
    "48.815,1.864,1.864,0,0,0,2.063.7A486.048,486.048,0,0,0,610.7,405.729a1.882,1.882,0,0,0,"
    ".765-1.352C623.729,277.594,590.933,167.465,524.531,69.836ZM222.491,337.58c-28.972,"
    "0-52.844-26.587-52.844-59.239S193.056,219.1,222.491,219.1c29.665,0,53.306,26.82,"
    "52.843,59.239C275.334,310.993,251.924,337.58,222.491,337.58Zm195.38,0c-28.971,"
    "0-52.843-26.587-52.843-59.239S388.437,219.1,417.871,219.1c29.667,0,53.307,26.82,"
    "52.844,59.239C470.715,310.993,447.538,337.58,417.871,337.58Z"
)

GITHUB_ICON_PATH = (
    "M165.9 397.4c0 2-2.3 3.6-5.2 3.6-3.3.3-5.6-1.3-5.6-3.6 0-2 2.3-3.6 5.2-3.6 3-.3 5.6 1.3 "
    "5.6 3.6zm-31.1-4.5c-.7 2 1.3 4.3 4.3 4.9 2.6 1 5.6 0 6.2-2s-1.3-4.3-4.3-5.2c-2.6-.7-5.5.3-"
    "6.2 2.3zm44.2-1.7c-2.9.7-4.9 2.6-4.6 4.9.3 2 2.9 3.3 5.9 2.6 2.9-.7 4.9-2.6 4.6-4.6-.3-1.9-"
    "3-3.2-5.9-2.9zM244.8 8C106.1 8 0 113.3 0 252c0 110.9 69.8 205.8 169.5 239.2 12.8 2.3 "
    "17.3-5.6 17.3-12.1 0-6.2-.3-40.4-.3-61.4 0 0-70 15-84.7-29.8 0 0-11.4-29.1-27.8-36.6 0 "
    "0-22.9-15.7 1.6-15.4 0 0 24.9 2 38.6 25.8 21.9 38.6 58.6 27.5 72.9 20.9 2.3-16 8.8-27.1 "
    "16-33.7-55.9-6.2-112.3-14.3-112.3-110.5 0-27.5 7.6-41.3 23.6-58.9-2.6-6.5-11.1-33.3 "
    "2.6-67.9 20.9-6.5 69 27 69 27 20-5.6 41.5-8.5 62.8-8.5s42.8 2.9 62.8 8.5c0 0 48.1-33.6 "
    "69-27 13.7 34.7 5.2 61.4 2.6 67.9 16 17.7 25.8 31.5 25.8 58.9 0 96.5-58.9 104.2-114.8 "
    "110.5 9.2 7.9 17 22.9 17 46.4 0 33.7-.3 75.4-.3 83.6 0 6.5 4.6 14.4 17.3 12.1C428.2 "
    "457.8 496 362.9 496 252 496 113.3 383.5 8 244.8 8zM97.2 352.9c-1.3 1-1 3.3.7 5.2 1.6 1.6 "
    "3.9 2.3 5.2 1 1.3-1 1-3.3-.7-5.2-1.6-1.6-3.9-2.3-5.2-1zm-10.8-8.1c-.7 1.3.3 2.9 2.3 3.9 "
    "1.6 1 3.6.7 4.3-.7.7-1.3-.3-2.9-2.3-3.9-2-.6-3.6-.3-4.3.7zm32.4 35.6c-1.6 1.3-1 4.3 1.3 "
    "6.2 2.3 2.3 5.2 2.6 6.5 1 1.3-1.3.7-4.3-1.3-6.2-2.2-2.3-5.2-2.6-6.5-1zm-11.4-14.7c-1.6 "
    "1-1.6 3.6 0 5.9 1.6 2.3 4.3 3.3 5.6 2.3 1.6-1.3 1.6-3.9 0-6.2-1.4-2.3-4-3.3-5.6-2z"
)


def _nav_badge(href, aria_label, icon_path, icon_viewbox, count_text):
    return (
        "                    <a\n"
        '                        class="nav-badge"\n'
        '                        href="%s"\n'
        '                        target="_blank"\n'
        '                        rel="noopener"\n'
        '                        aria-label="%s"\n'
        "                    >\n"
        '                        <span class="nav-badge-icon">'
        '<svg viewBox="%s" fill="currentColor" aria-hidden="true"><path d="%s"/></svg>'
        "</span>\n"
        '                        <span class="nav-badge-count">%s</span>\n'
        "                    </a>\n"
    ) % (href, aria_label, icon_viewbox, icon_path, count_text)


def build_nav_badges():
    stars = get_github_stars()
    out = []
    out.append(
        _nav_badge(
            DISCORD_INVITE, "Ze Discord", DISCORD_ICON_PATH, "0 0 640 512", "Discord"
        )
    )
    out.append(
        _nav_badge(
            "https://github.com/%s" % GITHUB_REPO,
            "Ze on GitHub, %d stars" % stars,
            GITHUB_ICON_PATH,
            "0 0 496 512",
            str(stars),
        )
    )
    return "".join(out)


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
CODE_RE = re.compile(r"`([^`]+?)`")


def bold(text):
    """Convert Zeledon-style **bold** and `code` markers to <strong>/<code>
    for card bullet text pulled out of data/*.json -- avoids storing raw
    HTML in data. Code-span content is HTML-escaped first: unescaped, a
    literal "<code>" placeholder inside backticks (meant as display text,
    e.g. `` `ze explain <code>` ``) gets parsed as a real opening tag by the
    browser instead of shown as text, swallowing everything after it."""
    text = CODE_RE.sub(lambda m: "<code>%s</code>" % html.escape(m.group(1)), text)
    return BOLD_RE.sub(r"<strong>\1</strong>", text)
