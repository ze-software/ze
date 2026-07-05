#!/usr/bin/env -S uv run python3
"""Build data/search-index.json for the site's client-side search.

Usage:
    tools/render-search-index.py

Walks every published index.md mirror on the site (the same plain-Markdown
twins llms.txt links to) and emits one JSON record per page: title, url,
section, and a stripped-to-text body capped for size. search/index.html
fetches this file and does the matching in the browser -- no server, no
build-time index format, just data the site already generates.

The Configuration Reference is the exception. Its index.md is one 250KB+
dump of the whole YANG config tree, so a single capped page record would
bury every config term (searching "ddos" found nothing before this). Instead
we read data/yang-config-tree.json directly and emit one record per
top-level config section (bgp, ddos, ospf, ...), each deep-linked to
config-reference/#<section> -- the same hash the in-page config explorer
routes on -- with the whole subtree's names, types and descriptions as its
body. So "ddos", "flowtriq", "flowspec" and friends now land straight on the
ddos section. The raw config-reference/index.md page is skipped in the
generic walk to avoid a giant duplicate record.

Run this after the pages and their index.md siblings exist (i.e. after the
docs/blog/nav steps) and after data/yang-config-tree.json is generated, so
the mirrors and the config tree it reads are current.
"""

import json
import pathlib
import re
import sys

import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
OUT = GH_PAGES / "data" / "search-index.json"
YANG_TREE = GH_PAGES / "data" / "yang-config-tree.json"
PAGE = GH_PAGES / "search" / "index.html"
PAGE_DESC = "Search across all of Ze's documentation and pages, in your browser."

# Directories under the site root that are not published pages.
SKIP_TOP = {"data", "assets", "tmp", ".git", ".ruff_cache", "tools"}

# The Configuration Reference is indexed per config section from the YANG
# tree (see build_config_records), not as one capped page, so its raw
# index.md mirror is skipped in the generic walk.
CONFIG_REF_DIR = "config-reference"

# Per-page body cap. Big enough that multi-section pages (the Configuration
# guide, command reference, the per-protocol guides) are searchable past
# their first screen, not just their intro.
BODY_CAP = 8000

# Per-section cap for the config-reference records. The body is built
# names-first (every setting name and type across the whole subtree, then the
# descriptions), so this only ever truncates trailing prose on the three giant
# sections (bgp, interface, ospf) whose names alone are ~10KB -- every setting
# name in every section stays findable.
CONFIG_BODY_CAP = 12000

HTML_COMMENT_RE = re.compile(r"<!--.*?-->", re.S)
FENCE_RE = re.compile(r"^```.*$", re.M)
LINK_RE = re.compile(r"!?\[([^\]]*)\]\([^)]*\)")
HEADING_RE = re.compile(r"^\s{0,3}#{1,6}\s*", re.M)
EMPHASIS_RE = re.compile(r"[*`_>]+")
WS_RE = re.compile(r"\s+")
FIRST_H1_RE = re.compile(r"^#\s+(.+)$", re.M)


def strip_markdown(text):
    text = HTML_COMMENT_RE.sub(" ", text)
    text = FENCE_RE.sub(" ", text)
    text = LINK_RE.sub(r"\1", text)
    text = HEADING_RE.sub("", text)
    text = text.replace("|", " ")
    text = EMPHASIS_RE.sub(" ", text)
    text = WS_RE.sub(" ", text)
    return text.strip()


def url_for(rel_dir):
    # rel_dir is the page's directory relative to the site root ("" for root).
    return "" if rel_dir in ("", ".") else rel_dir.replace("\\", "/") + "/"


def title_for(md_text, rel_dir):
    m = FIRST_H1_RE.search(md_text)
    if m:
        return m.group(1).strip()
    return rel_dir.rsplit("/", 1)[-1] if rel_dir else "Ze"


def section_for(rel_dir):
    if not rel_dir:
        return "Home"
    return rel_dir.split("/", 1)[0]


def node_head(node):
    """A config node's own line as an operator would type it: "peer <name>"
    for a YANG list keyed by "name", plain "hold-time" for a leaf. Kept in
    sync with the node_head twins in tools/render-config-reference.py."""
    kind = node.get("kind") or ""
    if kind.startswith("list[") and kind.endswith("]"):
        return "%s <%s>" % (node["name"], kind[5:-1])
    return node["name"]


def flatten_section(node, heads, descs):
    """Collect a config subtree's searchable words into two buckets: `heads`
    gets every node's head and type (the compact, high-value terms an operator
    searches -- `hold-time`, `flowtriq`), `descs` gets the prose descriptions.
    The caller joins heads-then-descs so a per-section cap only ever trims
    trailing prose, never a setting name."""
    heads.append(node_head(node))
    typ = node.get("type")
    if typ:
        heads.append(typ)
    desc = node.get("description")
    if desc:
        descs.append(" ".join(desc.split()))
    for child in node.get("children") or []:
        flatten_section(child, heads, descs)


def build_config_records():
    """One search record per top-level config section, read straight from the
    YANG config tree, each deep-linked to config-reference/#<section>. Returns
    [] (with a warning) if the tree has not been generated yet."""
    if not YANG_TREE.exists():
        sitelib.warn(
            "%s not found -- config sections will be missing from "
            "search; run tools/extract-yang-config-tree.py first" % YANG_TREE
        )
        return []
    tree = json.loads(YANG_TREE.read_text())
    names = sorted(tree)
    records = []
    # A landing record so "configuration reference" and bare section-name
    # queries also reach the top of the explorer, not only a deep section.
    records.append(
        {
            "title": "Configuration Reference",
            "url": CONFIG_REF_DIR + "/",
            "section": CONFIG_REF_DIR,
            "text": (
                "The complete Ze configuration as one searchable tree of "
                "sections, generated from the YANG schema. Sections: "
                + ", ".join(names)
                + "."
            )[:BODY_CAP],
        }
    )
    for name in names:
        heads, descs = [], []
        flatten_section(tree[name], heads, descs)
        body = " ".join(heads + descs)
        records.append(
            {
                "title": name,
                "url": "%s/#%s" % (CONFIG_REF_DIR, name),
                "section": CONFIG_REF_DIR,
                "text": body[:CONFIG_BODY_CAP],
            }
        )
    return records


def main():
    records = []
    seen = set()
    for md_path in sorted(GH_PAGES.rglob("index.md")):
        rel = md_path.relative_to(GH_PAGES)
        top = rel.parts[0] if len(rel.parts) > 1 else ""
        if top in SKIP_TOP:
            continue
        rel_dir = str(rel.parent) if rel.parent != pathlib.Path(".") else ""
        if rel_dir == CONFIG_REF_DIR:
            continue  # emitted as per-section records by build_config_records
        text = md_path.read_text()
        body = strip_markdown(text)
        records.append(
            {
                "title": title_for(text, rel_dir),
                "url": url_for(rel_dir),
                "section": section_for(rel_dir),
                "text": body[:BODY_CAP],
            }
        )
        seen.add(url_for(rel_dir))

    records.extend(build_config_records())

    # The homepage is generated HTML with no index.md mirror; add it by hand
    # so the front page is searchable too.
    if "" not in seen:
        records.append(
            {
                "title": "Ze - Open, Programmable Network OS For Linux",
                "url": "",
                "section": "Home",
                "text": (
                    "Ze is an open, programmable network OS for Linux, built "
                    "around a native BGP, OSPF, and IS-IS engine, operator "
                    "interfaces, telemetry, and a plugin system around one "
                    "configuration model."
                ),
            }
        )

    records.sort(key=lambda r: r["url"])
    OUT.write_text(json.dumps(records, ensure_ascii=False, separators=(",", ":")) + "\n")
    print("wrote %s (%d pages)" % (OUT, len(records)))

    render_search_page()
    return 0


SEARCH_BODY = """            <section class="md-content reveal" aria-labelledby="search-title">
                <h1 id="search-title">Search</h1>
                <p>Search across all of Ze's documentation and pages. Everything
                runs in your browser: nothing you type is sent anywhere.</p>
                <div class="cli-search-wrap">
                    <input id="site-search" type="search" autocomplete="off"
                        autofocus aria-label="Search the site"
                        placeholder="Search the site (e.g. flowspec, quickstart, RPKI, exabgp)..." />
                </div>
                <p id="search-status" class="search-status" aria-live="polite"></p>
                <ol id="search-results" class="search-results"></ol>
            </section>
            <script>
            (function () {
                var idxUrl = "../data/search-index.json";
                var input = document.getElementById("site-search");
                var status = document.getElementById("search-status");
                var list = document.getElementById("search-results");
                var records = null;

                function esc(s) {
                    return s.replace(/[&<>"]/g, function (c) {
                        return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c];
                    });
                }
                function href(url) { return "../" + (url || ""); }

                function snippet(text, tokens) {
                    var low = text.toLowerCase(), at = -1;
                    for (var i = 0; i < tokens.length; i++) {
                        var p = low.indexOf(tokens[i]);
                        if (p !== -1 && (at === -1 || p < at)) at = p;
                    }
                    if (at === -1) at = 0;
                    var start = Math.max(0, at - 60);
                    var frag = text.slice(start, start + 200);
                    if (start > 0) frag = "..." + frag;
                    if (start + 200 < text.length) frag = frag + "...";
                    frag = esc(frag);
                    tokens.forEach(function (t) {
                        if (!t) return;
                        var re = new RegExp("(" + t.replace(/[.*+?^${}()|[\\]\\\\]/g, "\\\\$&") + ")", "ig");
                        frag = frag.replace(re, "<mark>$1</mark>");
                    });
                    return frag;
                }

                function score(r, tokens) {
                    var title = r.title.toLowerCase();
                    var section = (r.section || "").toLowerCase();
                    var text = r.text.toLowerCase();
                    var total = 0;
                    for (var i = 0; i < tokens.length; i++) {
                        var t = tokens[i], hit = 0;
                        if (title.indexOf(t) !== -1) { hit += 12; }
                        if (section.indexOf(t) !== -1) { hit += 4; }
                        var n = text.split(t).length - 1;
                        if (n > 0) { hit += Math.min(n, 5); }
                        if (hit === 0) return 0; // every token must appear somewhere
                        total += hit;
                    }
                    return total;
                }

                function run(q) {
                    if (!records) return;
                    var tokens = q.toLowerCase().split(/\\s+/).filter(Boolean);
                    list.innerHTML = "";
                    if (!tokens.length) { status.textContent = ""; return; }
                    var scored = [];
                    for (var i = 0; i < records.length; i++) {
                        var s = score(records[i], tokens);
                        if (s > 0) scored.push([s, records[i]]);
                    }
                    scored.sort(function (a, b) { return b[0] - a[0]; });
                    var top = scored.slice(0, 40);
                    status.textContent = scored.length
                        ? scored.length + " result" + (scored.length === 1 ? "" : "s")
                        : "No results for \\u201c" + q + "\\u201d.";
                    top.forEach(function (pair) {
                        var r = pair[1];
                        var li = document.createElement("li");
                        li.className = "search-result";
                        li.innerHTML =
                            '<a href="' + href(r.url) + '"><span class="search-result-title">' +
                            esc(r.title) + '</span> <span class="chip">' + esc(r.section) +
                            '</span></a><p class="search-result-snippet">' +
                            snippet(r.text, tokens) + "</p>";
                        list.appendChild(li);
                    });
                }

                function debounce(fn, ms) {
                    var t; return function () {
                        clearTimeout(t);
                        t = setTimeout(fn, ms);
                    };
                }

                var params = new URLSearchParams(location.search);
                var initial = params.get("q") || "";
                if (initial) input.value = initial;

                status.textContent = "Loading index...";
                fetch(idxUrl).then(function (r) { return r.json(); }).then(function (data) {
                    records = data;
                    status.textContent = "";
                    if (input.value) run(input.value);
                }).catch(function () {
                    status.textContent = "Could not load the search index.";
                });

                input.addEventListener("input", debounce(function () {
                    run(input.value);
                    var u = new URL(location);
                    if (input.value) u.searchParams.set("q", input.value);
                    else u.searchParams.delete("q");
                    history.replaceState(null, "", u);
                }, 120));
            })();
            </script>
"""

SEARCH_MD = (
    "# Search\n\n"
    "Search across all of Ze's documentation and pages. This page runs the "
    "search in your browser against [data/search-index.json](../data/search-index.json), "
    "a JSON index of every page's Markdown mirror. It needs JavaScript; with it "
    "off, browse from the [documentation hub](../docs/) instead.\n"
)


def render_search_page():
    full_title = "Search - Ze"
    head = sitelib.page_head(
        full_title,
        PAGE_DESC,
        "../",
        og_title=full_title,
        og_desc=PAGE_DESC,
        page_key="search/",
    )
    PAGE.parent.mkdir(parents=True, exist_ok=True)
    PAGE.write_text(head + SEARCH_BODY + sitelib.page_foot("../"))
    sitelib.write_markdown_sibling(PAGE, SEARCH_MD)
    print("rendered search page -> %s (+ index.md)" % PAGE)


if __name__ == "__main__":
    raise SystemExit(main())
