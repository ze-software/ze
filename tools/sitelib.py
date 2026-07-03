"""Shared chrome (nav, head, foot) for every gh-pages page generator.

Single source of truth for the ClickUp-style mega-menu (data/nav.json), the
live GitHub star badge, and the standard page head/foot. Every render-*.py
script imports this instead of carrying its own copy -- previously the nav
markup was duplicated verbatim in render-doc.py, render-blog.py, and
render-activity.py, which is how a bulk-patch bug once duplicated dropdown
content across 82 pages: three copies to keep in sync, one code path now.
"""

import hashlib
import html
import json
import pathlib
import re
import sys
import urllib.request
from html.parser import HTMLParser
from urllib.parse import urljoin

import sitefacts

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
DATA_DIR = GH_PAGES / "data"

# Published site root -- every render-*.py that needs to turn a page-relative
# href into an absolute URL (llms.txt entries, Markdown mirrors whose links
# must resolve outside the site's own relative-path structure) imports this
# instead of hardcoding it a second time. This MUST be the live canonical
# domain (the CNAME apex), not the github.io project URL, which 301-redirects
# here: sitemap <loc> entries, the robots.txt Sitemap line, JSON-LD url, and
# og:image must all point at the final URL, never a cross-host redirect.
SITE_BASE = "https://ze-software.net/"
FONT_CSS_URL = (
    "https://fonts.googleapis.com/css2?"
    "family=Poppins:wght@400;700;800&family=Lato:wght@400;700&display=swap"
)


# Build-time drift warnings that MUST be resolved before a build can pass.
# A generator calls sitelib.warn(msg) -- instead of print("warning: ...") --
# when the warning marks real drift: a page gone stale against its source, an
# undocumented dependency, an orphaned config root. The message still prints
# immediately (so it shows next to the step that emitted it), and it is also
# recorded so build.py can list every one and exit non-zero at the very end.
# The site is still fully generated either way; only the exit code goes red,
# and it stays red until every drift is fixed -- a warning nobody has to act
# on gets scrolled past, which is exactly how these went stale before.
#
# A plain print("warning: ...", file=sys.stderr) is still correct for
# tolerable fallbacks that must NOT fail an otherwise-valid build: no network
# (cached GitHub star count) or no built ze binary (cached CLI catalog). Those
# are graceful degradation, not drift.
_BUILD_WARNINGS = []


def warn(message):
    """Record a drift warning that fails the build at the end (see build.py).

    Prints immediately so it appears next to the step that emitted it, then
    appends it so build.py can report the full list and exit non-zero.
    """
    print("warning: " + message, file=sys.stderr)
    _BUILD_WARNINGS.append(message)


def build_warnings():
    """Every drift warning recorded via warn() so far this process."""
    return list(_BUILD_WARNINGS)

NAV_CHEVRON = (
    '<svg viewBox="0 0 12 8" fill="none" aria-hidden="true">'
    '<path d="M1 1l5 5 5-5" stroke="currentColor" stroke-width="1.6" '
    'stroke-linecap="round" stroke-linejoin="round"/></svg>'
)

DISCORD_INVITE = "https://discord.gg/T8s7CjPDne"
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


# Magnifying glass (Font Awesome "magnifying-glass"), viewBox 0 0 512 512.
SEARCH_ICON_PATH = (
    "M416 208c0 45.9-14.9 88.3-40 122.7L502.6 457.4c12.5 12.5 12.5 32.8 0 45.3"
    "s-32.8 12.5-45.3 0L330.7 376c-34.4 25.2-76.8 40-122.7 40C93.1 416 0 322.9 "
    "0 208S93.1 0 208 0S416 93.1 416 208zM208 352a144 144 0 1 0 0-288 144 144 "
    "0 1 0 0 288z"
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
    ) % (
        html.escape(href, quote=True),
        html.escape(aria_label, quote=True),
        html.escape(icon_viewbox, quote=True),
        html.escape(icon_path, quote=True),
        html.escape(str(count_text)),
    )


def _nav_search_badge(root):
    return (
        "                    <a\n"
        '                        class="nav-badge nav-badge-search"\n'
        '                        href="%ssearch/"\n'
        '                        aria-label="Search the site"\n'
        '                        aria-expanded="false"\n'
        "                    >\n"
        '                        <span class="nav-badge-icon">'
        '<svg viewBox="0 0 512 512" fill="currentColor" aria-hidden="true">'
        '<path d="%s"/></svg>'
        "</span>\n"
        '                        <span class="nav-badge-count nav-badge-search-label">Search</span>\n'
        "                    </a>\n"
    ) % (html.escape(root, quote=True), html.escape(SEARCH_ICON_PATH, quote=True))


def build_nav_badges(root=""):
    stars = get_github_stars()
    out = [_nav_search_badge(root)]
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


def nav_slug(label):
    slug = re.sub(r"[^a-z0-9]+", "-", label.lower()).strip("-")
    return slug or "menu"


def nav_item(root, href, icon, title, desc, feature=False):
    cls = "nav-dropdown-item nav-dropdown-feature" if feature else "nav-dropdown-item"
    return (
        '                        <a class="%s" href="%s">\n'
        '                            <span class="nav-dropdown-icon">%s</span>\n'
        "                            <span><strong>%s</strong><small>%s</small></span>\n"
        "                        </a>\n"
    ) % (
        cls,
        html.escape(root + href, quote=True),
        html.escape(icon),
        html.escape(title),
        html.escape(desc),
    )


def nav_dropdown(label, columns):
    panel_id = "nav-panel-" + nav_slug(label)
    out = ['                    <div class="nav-dropdown">\n']
    out.append(
        '                    <button class="nav-dropdown-trigger" type="button" '
        'aria-haspopup="true" aria-expanded="false" aria-controls="%s">%s\n'
        "                        %s\n"
        "                    </button>\n"
        % (panel_id, html.escape(label), NAV_CHEVRON)
    )
    out.append('                    <div class="nav-dropdown-panel" id="%s">\n' % panel_id)
    for col in columns:
        out.append('                    <div class="nav-dropdown-col">\n')
        for entry in col:
            if isinstance(entry, str):
                out.append(
                    '                        <span class="nav-dropdown-label">%s</span>\n'
                    % html.escape(entry)
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




def live_counts():
    """The live numbers substituted into data/nav.json desc placeholders."""
    try:
        facts = sitefacts.load_facts()
    except (OSError, KeyError, ValueError):
        facts = {}
    features = facts.get("features", {})
    return {
        "features": features.get("core_experimental", "?"),
        "cli_commands": facts.get("cli_commands", "?"),
        "config_sections": facts.get("config_sections", "?"),
        "dependencies": facts.get("dependencies", "?"),
        "changes": facts.get("changes", "?"),
        "articles": facts.get("blog_articles", "?"),
    }


def load_nav_data():
    global _nav_data_cache
    if _nav_data_cache is None:
        data = json.loads((DATA_DIR / "nav.json").read_text())
        counts = live_counts()
        for dropdown in data.get("dropdowns", []):
            for column in dropdown.get("columns", []):
                for entry in column:
                    desc = entry.get("desc")
                    if desc and "%(" in desc:
                        entry["desc"] = desc % counts
        _nav_data_cache = data
    return _nav_data_cache


def _columns_from_data(columns):
    out = []
    for col in columns:
        c = []
        for entry in col:
            if "label_only" in entry:
                c.append(entry["label_only"])
            else:
                c.append(
                    (
                        entry["href"],
                        entry["icon"],
                        entry["title"],
                        entry["desc"],
                        entry.get("feature", False),
                    )
                )
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


def blog_dropdown_columns(n=5):
    """The Blog dropdown is populated live from the newest posts (via
    latest_blog_posts) rather than a hand-maintained nav.json list, so it can
    never fall behind blog/posts/. An all-posts link first, then a column of
    the most recent weeks; entries are pre-root tuples (href, icon, title,
    desc, feature) that nav_dropdown_rooted then prefixes with the page's
    root."""
    col = [
        ("changes/", "\U0001f4da", "All updates", "Every weekly update, newest first", False)
    ]
    for post in latest_blog_posts(n):
        title = "Week of %s" % post["slug"]
        if post.get("is_draft"):
            title += " (draft)"
        intro = " ".join((post.get("intro") or "").split())
        desc = (intro[:70] + "…") if len(intro) > 70 else (intro or "Weekly update")
        col.append(("changes/%s/" % post["slug"], "\U0001f5d3️", title, desc, False))
    return [col]


def build_navblock(root):
    data = load_nav_data()
    out = ['                <div id="site-nav-links" class="nav-links">\n']
    for link in data["top_links"]:
        out.append(
            '                    <a href="%s">%s</a>\n'
            % (
                html.escape(rooted_href(root, link["href"]), quote=True),
                html.escape(link["label"]),
            )
        )
    for dropdown in data["dropdowns"]:
        if dropdown.get("dynamic") == "blog":
            columns = blog_dropdown_columns()
        else:
            columns = _columns_from_data(dropdown["columns"])
        out.append(nav_dropdown_rooted(root, dropdown["label"], columns))
    for link in data["trailing_links"]:
        out.append(
            '                    <a href="%s">%s</a>\n'
            % (
                html.escape(rooted_href(root, link["href"]), quote=True),
                html.escape(link["label"]),
            )
        )
    out.append(build_nav_badges(root))
    out.append("                </div>")
    return "".join(out)


NAV_LINKS_START_RE = re.compile(r'[ \t]*<div\b[^>]*class="nav-links"[^>]*>')
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
    div_start = m.start()
    end = _find_balanced_div_end(html_text, div_start)
    return html_text[: m.start()] + build_navblock(root) + html_text[end:]


# (heading, [(path relative to site root, label), ...]) -- local links only,
# rooted through `root` at render time. External links (Discord, GitHub,
# Codeberg, Issues) are appended separately since they never take a root
# prefix.
FOOTER_LOCAL_COLUMNS = [
    (
        "Project",
        [
            ("features/", "Features"),
            ("cli/", "CLI Reference"),
            ("dependencies/", "Dependencies"),
            ("performance/", "Performance"),
            ("compare/", "Compare"),
            ("roadmap/", "Roadmap"),
            ("milestones/", "Milestones"),
            ("changes/", "Changes"),
            ("activity/", "Activity"),
        ],
    ),
    (
        "Learn",
        [
            ("docs/", "Documentation"),
            ("docs/guide/quickstart/", "Quickstart"),
            ("docs/architecture/", "Architecture"),
            ("labs/", "Labs"),
            ("blog/", "Blog"),
            ("talks/", "Talks"),
            ("faq/", "FAQ"),
        ],
    ),
]


def footer_html(root):
    """Full <footer> markup: a three-column sitemap (Project / Learn /
    Community) plus a bottom bar (license, Style Guide) -- replaces the
    single flat row of five links every page used to carry. Used both by
    page_foot() for generated pages and by patch_footer() to refresh the
    same markup inside hand-authored pages, so the two can never drift
    apart the way nav content once did before data/nav.json existed."""
    out = ["        <footer>\n", '            <div class="footer-inner">\n']
    out.append('                <div class="footer-columns">\n')
    for heading, links in FOOTER_LOCAL_COLUMNS:
        out.append('                    <div class="footer-col">\n')
        out.append("                        <h3>%s</h3>\n" % heading)
        for path, label in links:
            out.append(
                '                        <a href="%s">%s</a>\n'
                % (rooted_href(root, path), label)
            )
        out.append("                    </div>\n")
    out.append('                    <div class="footer-col">\n')
    out.append("                        <h3>Community</h3>\n")
    out.append(
        '                        <a href="%s" target="_blank" rel="noopener">Discord</a>\n'
        % DISCORD_INVITE
    )
    out.append(
        '                        <a href="%scontribute/">Contribute</a>\n' % root
    )
    out.append('                        <a href="%ssecurity/">Security</a>\n' % root)
    out.append(
        '                        <a href="%scode-of-conduct/">Code of Conduct</a>\n'
        % root
    )
    out.append(
        '                        <a href="https://github.com/%s" target="_blank" rel="noopener">GitHub</a>\n'
        % GITHUB_REPO
    )
    out.append(
        '                        <a href="%s" target="_blank" rel="noopener">Codeberg</a>\n'
        % CODEBERG_REPO
    )
    out.append(
        '                        <a href="https://github.com/%s/issues" target="_blank" rel="noopener">Issues</a>\n'
        % GITHUB_REPO
    )
    out.append("                    </div>\n")
    out.append("                </div>\n")
    out.append('                <div class="footer-bottom">\n')
    out.append(
        '                    <a href="%slicense/">Ze is AGPLv3 open source.</a>\n'
        % root
    )
    out.append('                    <a href="%sstyle-guide/">Style Guide</a>\n' % root)
    out.append("                </div>\n")
    out.append("            </div>\n")
    out.append("        </footer>")
    return "".join(out)


FOOTER_START_RE = re.compile(r"[ \t]*<footer>")


def patch_footer(html_text, root):
    """Replace the <footer>...</footer> block in an already hand-authored
    page with a freshly built one, the same way patch_navblock() refreshes
    <div class="nav-links">. Footer markup never nests another <footer>, so
    a plain first-match pair is enough (no balanced-tag counting needed)."""
    m = FOOTER_START_RE.search(html_text)
    if not m:
        raise ValueError("no <footer> found")
    start = html_text.index("<footer>", m.start())
    end = html_text.index("</footer>", start) + len("</footer>")
    return html_text[: m.start()] + footer_html(root) + html_text[end:]


_ASSET_VERSIONS = {}


def asset_query(relpath):
    """The ?v=<short content hash> cache-busting suffix for a site asset
    (site.css, site.js), or "" if the file can't be read. Computed once per
    asset per build and memoised."""
    if relpath not in _ASSET_VERSIONS:
        try:
            digest = hashlib.sha1((GH_PAGES / relpath).read_bytes()).hexdigest()[:10]
        except OSError:
            digest = None
        _ASSET_VERSIONS[relpath] = digest
    digest = _ASSET_VERSIONS[relpath]
    return ("?v=%s" % digest) if digest else ""


def asset_url(root, relpath):
    """Cache-busting URL for a site asset (site.css, site.js).

    Appends ?v=<short content hash> so a browser never serves a stale copy
    after a rebuild: change the file, the hash changes, the URL changes, the
    browser refetches. Without this, plain http.server / GitHub Pages caching
    can pin an old site.js and CSS/JS edits silently don't show up."""
    return "%s%s%s" % (root, relpath, asset_query(relpath))


_ASSET_REF_RE = re.compile(r"assets/site\.(css|js)(?:\?v=[0-9a-f]+)?")


def patch_asset_versions(html_text):
    """Refresh shared generated head bits in already-authored pages."""
    html_text = _ASSET_REF_RE.sub(
        lambda m: "assets/site.%s%s"
        % (m.group(1), asset_query("assets/site." + m.group(1))),
        html_text,
    )
    html_text = _FONT_REF_RE.sub(FONT_CSS_URL, html_text)
    html_text = patch_social_meta(html_text)
    return patch_structured_data(html_text)


_FONT_REF_RE = re.compile(r"https://fonts\.googleapis\.com/css2\?family=Poppins[^\"']+display=swap")
_STRUCTURED_DATA_RE = re.compile(
    r'[ \t]*<script type="application/ld\+json">.*?</script>\n?', re.DOTALL
)


def structured_data_script():
    data = {
        "@context": "https://schema.org",
        "@graph": [
            {
                "@type": "WebSite",
                "name": "Ze",
                "url": SITE_BASE,
                "description": (
                    "Open, programmable network OS for Linux with BGP, IS-IS, "
                    "OSPF, telemetry, operator interfaces, and plugins."
                ),
                "inLanguage": "en",
            },
            {
                "@type": "SoftwareSourceCode",
                "name": "Ze",
                "description": (
                    "Open-source network OS for Linux, built around native "
                    "routing engines and operator automation."
                ),
                "codeRepository": CODEBERG_REPO,
                "license": "https://www.gnu.org/licenses/agpl-3.0.en.html",
                "programmingLanguage": "Go",
                "runtimePlatform": "Linux",
                "applicationCategory": "Network operating system",
                "isAccessibleForFree": True,
            },
        ],
    }
    payload = json.dumps(data, ensure_ascii=False, separators=(",", ":")).replace(
        "</", "<\\/"
    )
    return '        <script type="application/ld+json">%s</script>\n' % payload


def patch_structured_data(html_text):
    script = structured_data_script()
    cleaned = _STRUCTURED_DATA_RE.sub("", html_text)
    return cleaned.replace("    </head>\n", script + "    </head>\n", 1)


# A raster 1200x630 card -- SVG is not rendered by social scrapers.
OG_IMAGE = SITE_BASE + "assets/social-card.png"

_SOCIAL_META = (
    '        <meta property="og:image" content="%s" />\n'
    '        <meta property="og:image:width" content="1200" />\n'
    '        <meta property="og:image:height" content="630" />\n'
    '        <meta property="og:image:alt" '
    'content="Ze, an open, programmable network OS for Linux" />\n'
    '        <meta name="twitter:card" content="summary_large_image" />\n'
    '        <meta name="twitter:image" content="%s" />\n'
) % (html.escape(OG_IMAGE, quote=True), html.escape(OG_IMAGE, quote=True))

_OG_TYPE_RE = re.compile(r'([ \t]*<meta property="og:type"[^>]*>\n)')


def patch_social_meta(html_text):
    """Give hand-authored pages the same share-card meta as generated pages.

    Idempotent: any page rendered through page_head already declares og:image
    (and its explicit twitter:title/description), so those are left untouched.
    Hand-authored heads carry only og:title/og:description/og:type, so append
    the shared image + large-image twitter card; twitter falls back to the
    existing og:title/og:description for the text."""
    if 'property="og:image"' in html_text:
        return html_text
    m = _OG_TYPE_RE.search(html_text)
    if m:
        return html_text[: m.end()] + _SOCIAL_META + html_text[m.end() :]
    return html_text.replace("    </head>\n", _SOCIAL_META + "    </head>\n", 1)

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
        <meta property="og:image" content="{og_image}" />
        <meta property="og:image:width" content="1200" />
        <meta property="og:image:height" content="630" />
        <meta property="og:image:alt" content="Ze, an open, programmable network OS for Linux" />
        <meta name="twitter:card" content="summary_large_image" />
        <meta name="twitter:title" content="{og_title}" />
        <meta name="twitter:description" content="{og_desc}" />
        <meta name="twitter:image" content="{og_image}" />
        <link rel="icon" href="{root}assets/ze.svg" type="image/svg+xml" />
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
        <link
            href="{font_css}"
            rel="stylesheet"
        />
        <link rel="stylesheet" href="{site_css}" />
{json_ld}{extra_head}    </head>
    <body>
        <a class="skip-link" href="#top">Skip to main content</a>
        <header class="site-header">
            <nav class="nav" aria-label="Main navigation">
                <a class="brand" href="{brand_href}" aria-label="Ze home">
                    <img src="{root}assets/ze.svg" alt="" width="32" height="32" />
                    <span>Ze</span>
                </a>
                <button class="nav-menu-toggle" type="button" aria-controls="site-nav-links" aria-expanded="false">
                    <span class="nav-menu-toggle-bars" aria-hidden="true"></span>
                    <span>Menu</span>
                </button>
{navblock}
            </nav>
        </header>

        <main id="top" tabindex="-1">
"""

PAGE_FOOT = """        </main>

        <script src="{site_js}" defer></script>

{footer}
    </body>
</html>
"""


def page_head(title, desc, root, og_title=None, og_desc=None, extra_head=""):
    page_title = str(title)
    page_desc = str(desc)
    social_title = str(og_title if og_title is not None else title)
    social_desc = str(og_desc if og_desc is not None else desc)
    og_image = OG_IMAGE
    return PAGE_HEAD.format(
        title=html.escape(page_title),
        desc=html.escape(page_desc, quote=True),
        og_title=html.escape(social_title, quote=True),
        og_desc=html.escape(social_desc, quote=True),
        og_image=html.escape(og_image, quote=True),
        root=root,
        site_css=asset_url(root, "assets/site.css"),
        font_css=html.escape(FONT_CSS_URL, quote=True),
        brand_href=html.escape(rooted_href(root, "index.html#top"), quote=True),
        navblock=build_navblock(root),
        extra_head=extra_head,
        json_ld=structured_data_script(),
    )


def page_foot(root):
    return PAGE_FOOT.format(
        root=root,
        site_js=asset_url(root, "assets/site.js"),
        footer=footer_html(root),
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


# Blog post parsing, shared by render-blog.py (full post + index rendering)
# and render-index.py (homepage "Latest from the blog" teaser) -- one parser
# for Zeledon's Discord-style weekly-update format instead of two copies
# drifting apart.
BLOG_HEADER_RE = re.compile(r"^\*\*(.+?)\*\*\s*$", re.MULTILINE)
BLOG_FRONT_MATTER_RE = re.compile(r"^---\n(.*?)\n---\n(.*)$", re.DOTALL)
# The weekly-update sources (the changelog) live under changes/posts/; the
# blog is now for editorial articles (see render-blog.py). latest_blog_posts()
# below feeds the homepage "latest updates" teaser from these weekly sources.
POSTS_DIR = GH_PAGES / "changes" / "posts"


def parse_blog_front_matter(text):
    m = BLOG_FRONT_MATTER_RE.match(text)
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


def split_blog_sections(body):
    """Return (title_marker, intro, [(header, section_body), ...])."""
    parts = BLOG_HEADER_RE.split(body)
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


def blog_start_date(covers):
    # "2026-06-08 .. 2026-06-14" or "2026-06-25 21:10 .. 2026-07-01"
    return covers.split("..")[0].strip().split(" ")[0]


def latest_blog_posts(n):
    """The N most recent blog posts (by covers' start date), each as
    {slug, covers, intro, is_draft} -- same shape render-blog.py's index
    entries use. Used by render-index.py's homepage teaser so the two never
    disagree about what "latest" means."""
    posts = []
    for f in sorted(POSTS_DIR.glob("*.md")):
        meta, body = parse_blog_front_matter(f.read_text())
        covers = meta.get("covers", f.stem.replace("..", " .. "))
        _title_marker, intro, sections = split_blog_sections(body)
        if not sections:
            continue
        slug = blog_start_date(covers)
        posts.append(
            {
                "slug": slug,
                "covers": covers,
                "intro": intro,
                "is_draft": meta.get("status", "").upper().startswith("DRAFT"),
            }
        )
    posts.sort(key=lambda p: p["slug"], reverse=True)
    return posts[:n]


# ---------------------------------------------------------------------------
# Markdown mirrors
#
# llms.txt (tools/render-llms-txt.py) links every page to a sibling index.md
# instead of its index.html, so an LLM fetching the site never has to parse
# HTML tags/CSS/JS to get at the content. Pages with a real Markdown source
# (docs, compare, contribute, blog posts) publish that source directly --
# see render-doc.py / render-blog.py. Pages built from JSON data (features,
# CLI, dependencies) render Markdown straight from the same data dict as the
# HTML -- see render-features.py etc. Pages with neither (labs/*, talks/,
# style-guide/, performance/, zeledon/ -- hand-authored HTML using this
# site's own component classes rather than plain tags) fall back to the
# functions below: extract_main() pulls the <main> content out of the
# rendered HTML, html_to_markdown() converts it. Wired into tools/build.py's
# "nav" step, so it can never drift from whatever HTML the page currently
# has -- same zero-drift philosophy as the rest of the build.

MAIN_START_RE = re.compile(r'<main id="top">')


def extract_main(html_text):
    """The page content between <main id="top"> and </main> -- excludes the
    mega-menu header and footer sitemap, which every page carries and which
    add nothing for an LLM already holding llms.txt itself."""
    m = MAIN_START_RE.search(html_text)
    if not m:
        raise ValueError('no <main id="top"> found')
    start = m.end()
    end = html_text.index("</main>", start)
    return html_text[start:end]


def write_markdown_sibling(dest_html_path, md_text):
    """Write index.md next to dest_html_path's index.html, same directory --
    the site's directory-per-page structure (features/index.html) means the
    sibling is reachable at the exact same URL with .md swapped in for the
    trailing slash (features/index.md)."""
    dest_md = dest_html_path.with_name("index.md")
    dest_md.write_text(md_text)
    return dest_md


_MD_SKIP_TAGS = {
    "script",
    "style",
    "svg",
    "button",
    "input",
    "select",
    "defs",
    "path",
    "textarea",
    "form",
    "noscript",
}
_MD_WS_RE = re.compile(r"\s+")
_MD_TRAILING_WS_RE = re.compile(r"[ \t]+\n")
_MD_BLANK_RUN_RE = re.compile(r"\n{3,}")
_MD_MULTI_SPACE_RE = re.compile(r"[ \t]{2,}")
_MD_LABEL_VALUE_RE = re.compile(r"([^\s:])(\*\*)(\s*)")


def _label_value_repl(m):
    before, stars, ws = m.groups()
    return "%s:%s%s" % (before, stars, ws if ws else " ")


def _collapse_spaces_outside_code(text):
    """Adjacent whitespace-only text nodes between sibling tags (pretty-
    printed HTML indentation) each collapse to a single space independently,
    so runs of 2+ spaces can appear at tag boundaries -- collapse those, but
    never inside a fenced code block, where meaningful indentation (e.g. a
    second shell command on its own line) must survive verbatim."""
    parts = text.split("```")
    for i in range(0, len(parts), 2):
        parts[i] = _MD_MULTI_SPACE_RE.sub(" ", parts[i])
    return "```".join(parts)


class _MDNode:
    __slots__ = ("tag", "attrs", "text")

    def __init__(self, tag, attrs):
        self.tag = tag
        self.attrs = dict(attrs)
        self.text = []

    def raw(self):
        return "".join(self.text)


def _render_table(rows):
    if not rows:
        return ""
    header = [text for _is_th, text in rows[0]]
    body = rows[1:]
    cols = len(header)
    lines = ["| " + " | ".join(header) + " |", "| " + " | ".join(["---"] * cols) + " |"]
    for row in body:
        cells = [text for _is_th, text in row]
        cells = (cells + [""] * cols)[:cols]
        lines.append("| " + " | ".join(cells) + " |")
    return "\n\n" + "\n".join(lines) + "\n\n"


class _HTMLToMarkdown(HTMLParser):
    """Converts an HTML fragment into Markdown. Not a general-purpose
    converter -- built against exactly the tags and component classes this
    site's own render-*.py scripts and hand-authored pages emit (plain
    headings/paragraphs/lists/tables/links/code, plus status-row, stat,
    chip/tag/card-label, and details/summary)."""

    def __init__(self, base_url):
        super().__init__(convert_charrefs=True)
        self.base_url = base_url
        self.root = _MDNode("root", {})
        self.stack = [self.root]
        self.skip_depth = 0
        self.list_stack = []
        self.pre_depth = 0
        self.table_stack = []
        self.row_stack = []

    def handle_starttag(self, tag, attrs):
        self._start(tag, attrs)

    def handle_startendtag(self, tag, attrs):
        # Self-closing tags (<path d="..." />, <input />, <br/>) never get a
        # matching handle_endtag, so they must never touch skip_depth --
        # only handle_starttag/handle_endtag pairs are allowed to.
        if self.skip_depth:
            return
        if tag in ("br", "hr", "img"):
            self._start(tag, attrs)
            return
        # Other bare self-closed tags (meta, link, path, defs children,
        # standalone <input/>) have no content of their own and contribute
        # nothing to the Markdown output.

    def _start(self, tag, attrs):
        if self.skip_depth:
            # Any nested open tag while skipping must be matched by exactly
            # one handle_endtag before skip mode ends, regardless of its
            # name -- balanced-depth counting, not a name check, so an
            # unrelated tag nested inside e.g. <svg> can't close it early.
            self.skip_depth += 1
            return
        attrs_d = dict(attrs)
        classes = (attrs_d.get("class") or "").split()
        if tag in _MD_SKIP_TAGS or "terminal-dots" in classes:
            self.skip_depth = 1
            return
        if tag == "br":
            self.stack[-1].text.append("  \n")
            return
        if tag == "hr":
            self.stack[-1].text.append("\n\n---\n\n")
            return
        if tag == "img":
            src = attrs_d.get("src", "")
            if src and not src.startswith(("http://", "https://", "data:")):
                src = urljoin(self.base_url, src)
            self.stack[-1].text.append("![%s](%s)" % (attrs_d.get("alt", ""), src))
            return
        if tag in ("ul", "ol"):
            self.list_stack.append({"type": tag, "n": 0})
        elif tag == "pre":
            self.pre_depth += 1
        elif tag == "table":
            self.table_stack.append([])
        elif tag == "tr":
            self.row_stack.append([])
        self.stack.append(_MDNode(tag, attrs))

    def handle_endtag(self, tag):
        if self.skip_depth:
            self.skip_depth -= 1
            return
        if tag in ("br", "hr", "img"):
            return
        if len(self.stack) <= 1:
            return
        node = self.stack.pop()
        if tag in ("ul", "ol") and self.list_stack:
            self.list_stack.pop()
        if tag == "pre":
            self.pre_depth -= 1
        if tag in ("td", "th"):
            cell = node.raw().strip().replace("\n", " ").replace("|", "\\|")
            if self.row_stack:
                self.row_stack[-1].append((tag == "th", cell))
            return
        if tag == "tr":
            row = self.row_stack.pop() if self.row_stack else []
            if row and self.table_stack:
                self.table_stack[-1].append(row)
            return
        if tag == "table":
            rows = self.table_stack.pop() if self.table_stack else []
            self.stack[-1].text.append(_render_table(rows))
            return
        if tag in ("thead", "tbody", "tfoot"):
            return
        self.stack[-1].text.append(self._render_node(tag, node))

    def handle_data(self, data):
        if self.skip_depth:
            return
        if self.pre_depth:
            self.stack[-1].text.append(data)
            return
        self.stack[-1].text.append(_MD_WS_RE.sub(" ", data))

    def _render_node(self, tag, node):
        inner = node.raw()
        if tag in ("h1", "h2", "h3", "h4", "h5", "h6"):
            text = inner.strip()
            return "\n\n%s %s\n\n" % ("#" * int(tag[1]), text) if text else ""
        if tag == "p":
            text = inner.strip()
            return "\n\n%s\n\n" % text if text else ""
        if tag == "a":
            href = node.attrs.get("href", "")
            label = inner.strip() or href
            if not href or href.startswith("#"):
                return label
            url = href
            if not href.startswith(("http://", "https://", "mailto:")):
                url = urljoin(self.base_url, href)
            return "[%s](%s)" % (label, url)
        if tag in ("strong", "b"):
            text = inner.strip()
            return "**%s**" % text if text else ""
        if tag in ("em", "i"):
            text = inner.strip()
            return "*%s*" % text if text else ""
        if tag == "code":
            return "`%s`" % inner.strip()
        if tag == "pre":
            code = inner.strip("\n")
            return "\n\n```\n%s\n```\n\n" % code if code else ""
        if tag in ("ul", "ol"):
            text = inner.strip("\n")
            return "\n\n%s\n\n" % text if text else ""
        if tag == "li":
            marker = "-"
            if self.list_stack and self.list_stack[-1]["type"] == "ol":
                self.list_stack[-1]["n"] += 1
                marker = "%d." % self.list_stack[-1]["n"]
            text = inner.strip().replace("\n", "\n  ")
            return "%s %s\n" % (marker, text) if text else ""
        if tag == "summary":
            text = inner.strip()
            return "\n\n**%s**\n\n" % text if text else ""
        if tag == "blockquote":
            text = inner.strip()
            if not text:
                return ""
            quoted = "\n".join("> " + line for line in text.splitlines())
            return "\n\n%s\n\n" % quoted
        classes = (node.attrs.get("class") or "").split()
        if tag == "div" and ("status-row" in classes or "stat" in classes):
            # <div class="status-row"><strong>Label</strong><span>Value</span></div>
            # (or the reverse order for .stat) concatenates with no
            # separator in the source HTML -- insert "label: value" so the
            # two aren't run together as "Label**Value**".
            text = inner.strip()
            text = _MD_LABEL_VALUE_RE.sub(_label_value_repl, text, count=1)
            return "- %s\n" % text if text else ""
        if "chip" in classes or "tag" in classes or "card-label" in classes:
            text = inner.strip()
            return "`%s` " % text if text else ""
        # Transparent containers (div, span, section, article, details, and
        # any other unrecognized tag/class): pass children through as-is
        # rather than dropping content the converter has no special case for.
        return inner


def html_to_markdown(fragment_html, base_url=SITE_BASE):
    parser = _HTMLToMarkdown(base_url)
    parser.feed(fragment_html)
    parser.close()
    text = parser.root.raw()
    text = _collapse_spaces_outside_code(text)
    text = _MD_TRAILING_WS_RE.sub("\n", text)
    text = _MD_BLANK_RUN_RE.sub("\n\n", text)
    return text.strip() + "\n"
