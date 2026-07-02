#!/usr/bin/env python3
"""Render config-reference/index.html: the whole Ze configuration as a
searchable index of sections, each drilling into its own config tree.

Usage:
    tools/render-config-reference.py

This page is about the *configuration*, not the plugins. The landing view is
an index table of every configuration section Ze has -- the complete YANG
config tree, one row per top-level section (name, owning plugin, what it
configures). That table is the navigation: pick a section and its config
tree replaces the index; a back link and prev/next return you. There is no
separate section list -- the index already is one.

The tree is produced by running the live `ze yang tree --json --config`
(cached by tools/extract-yang-config-tree.py to data/yang-config-tree.json),
the same unified walker Ze's own CLI uses, which already merges every config
module in the tree into one shape (no per-plugin fragments). Every container
and YANG list in a section is a collapsible <details>, so a big section like
`bgp` opens compact and a reader expands only what they care about.

Where a section is provided by a plugin, that ownership is shown as an
annotation, resolved from data/plugin-registry.json (see
tools/extract-plugin-registry.py): a plugin's ConfigRoots declares which
config path it owns (e.g. `bgp`, or the nested `fib/kernel`), so the section
is annotated with the owning plugin(s) and a link to each one's real YANG
source on Codeberg. Ownership resolves at the right depth, so a nested root
like `fib/kernel` annotates the `kernel` child under `fib`. Sections with no
plugin owner are core config. A ConfigRoot that resolves to no node in the
config tree (an ../main mid-refactor mismatch) is reported by
warn_orphan_roots and that section shows as core rather than silently
swallowing the ownership. Plugins that declare no config root (wire codecs,
filters) contribute behaviour, not config structure, so they do not appear
here -- this page is the config, not a plugin inventory.
"""

import html
import json
import pathlib
import sys

import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
MAIN_REPO = (GH_PAGES.parent / "main").resolve()
DATA = GH_PAGES / "data" / "plugin-registry.json"
YANG_TREE_DATA = GH_PAGES / "data" / "yang-config-tree.json"
DEST = GH_PAGES / "config-reference" / "index.html"

TEST_DIR_PREFIX = "internal/test/"


def esc(text):
    return html.escape(str(text))


def load_plugins():
    if not DATA.exists():
        print(
            "error: %s not found -- run tools/extract-plugin-registry.py first" % DATA,
            file=sys.stderr,
        )
        sys.exit(1)
    plugins = json.loads(DATA.read_text())
    return [p for p in plugins if not p["source_dir"].startswith(TEST_DIR_PREFIX)]


def load_yang_tree():
    if not YANG_TREE_DATA.exists():
        print(
            "error: %s not found -- run tools/extract-yang-config-tree.py "
            "first (needs ../main/bin/ze)" % YANG_TREE_DATA,
            file=sys.stderr,
        )
        sys.exit(1)
    return json.loads(YANG_TREE_DATA.read_text())


def build_owner_map(plugins):
    """config-path -> [plugins that declare it as a ConfigRoot]. The path is
    the raw ConfigRoots value: a top-level section name (`bgp`) or a nested
    path (`fib/kernel`), matched against a node's own path during render."""
    owners = {}
    for plugin in plugins:
        for root in plugin["config_roots"]:
            owners.setdefault(root, []).append(plugin)
    return owners


def path_exists(tree, path):
    """Whether a config-root path (e.g. "bgp" or "fib/kernel") resolves to a
    real node in the YANG config tree, walking child names."""
    parts = path.split("/")
    node = tree.get(parts[0])
    if node is None:
        return False
    for part in parts[1:]:
        node = next((c for c in node.get("children") or [] if c["name"] == part), None)
        if node is None:
            return False
    return True


def warn_orphan_roots(tree, owner_map):
    """A ConfigRoot that resolves to no config-tree node means its owning
    plugin's config section is silently unattributed on the page -- usually
    a mid-refactor mismatch in ../main between a plugin's declared ConfigRoot
    and the YANG container it actually augments. Surface it instead of hiding
    it (no silent truncation)."""
    orphans = sorted(root for root in owner_map if not path_exists(tree, root))
    for root in orphans:
        names = ", ".join(p["name"] for p in owner_map[root])
        print(
            "warning: config root %r (declared by %s) resolves to no node in "
            "the YANG config tree -- its ownership is not shown; check "
            "../main for a ConfigRoots vs YANG container mismatch" % (root, names),
            file=sys.stderr,
        )
    return orphans


def read_yang(rel_path):
    path = MAIN_REPO / rel_path
    if not path.exists():
        return None
    return path.read_text(errors="replace")


def yang_source_href(rel_path):
    return "%s/src/branch/main/%s" % (sitelib.CODEBERG_REPO, rel_path)


def render_yang_link(rel_path):
    filename = rel_path.rsplit("/", 1)[-1]
    if read_yang(rel_path) is None:
        return (
            '<span class="config-yang-missing">%s (not found on disk at render time)</span>'
            % esc(rel_path)
        )
    return '<a href="%s" target="_blank" rel="noopener"><code>%s</code></a>' % (
        esc(yang_source_href(rel_path)),
        esc(filename),
    )


def node_head(node):
    """A node's own line, e.g. "peer <name>" for a YANG list keyed by
    "name", or plain "hold-time" for a leaf -- the shape an operator would
    type, not the raw YANG "list[name]" kind string."""
    kind = node.get("kind") or ""
    if kind.startswith("list[") and kind.endswith("]"):
        return "%s <%s>" % (node["name"], kind[5:-1])
    return node["name"]


def node_badge(node):
    kind = node.get("kind") or ""
    typ = node.get("type") or ""
    if kind in ("leaf", "leaf-list") and typ:
        return typ + "[]" if kind == "leaf-list" else typ
    if kind.startswith("list["):
        return "list"
    return kind


def owner_marker(owners):
    """Compact at-a-glance ownership hint shown inline on a tree node."""
    if not owners:
        return ""
    if len(owners) == 1:
        text = "provided by %s" % owners[0]["name"]
    else:
        text = "provided by %d plugins" % len(owners)
    return ' <span class="config-owner">%s</span>' % esc(text)


def owner_short(owners):
    if not owners:
        return "core"
    if len(owners) == 1:
        return owners[0]["name"]
    return "%d plugins" % len(owners)


def node_label(node, owners):
    label = "<code>%s</code>" % esc(node_head(node))
    badge = node_badge(node)
    if badge:
        label += ' <span class="yang-type">%s</span>' % esc(badge)
    label += owner_marker(owners)
    desc = node.get("description")
    if desc:
        label += ' <span class="yang-desc">%s</span>' % esc(" ".join(desc.split()))
    return label


def owner_detail_html(owners):
    """The ownership block shown at the top of a section: who provides it and
    a link to each one's real YANG source."""
    if not owners:
        return '<p class="config-owner-detail">Core configuration &mdash; no plugin owner.</p>'

    def one(plugin):
        links = ", ".join(render_yang_link(y) for y in plugin["yang_files"])
        src = " &mdash; %s" % links if links else ""
        return "<code>%s</code>%s" % (esc(plugin["name"]), src)

    if len(owners) == 1:
        return '<p class="config-owner-detail">Provided by %s</p>' % one(owners[0])
    items = "".join("<li>%s</li>" % one(p) for p in owners)
    return (
        '<details class="config-owners"><summary>Provided by %d plugins</summary>'
        "<ul>%s</ul></details>"
    ) % (len(owners), items)


def render_child(node, path, owner_map):
    owners = owner_map.get(path, [])
    label = node_label(node, owners)
    children = node.get("children") or []
    if not children:
        return "<li>%s</li>" % label
    inner = "".join(
        render_child(c, path + "/" + c["name"], owner_map) for c in children
    )
    return (
        '<li><details class="yang-branch"><summary>%s</summary>%s<ul>%s</ul></details></li>'
    ) % (label, owner_detail_html(owners) if owners else "", inner)


# -- Views: the index (landing) and one detail view per section --


def render_index(tree, owner_map, sections):
    rows = []
    for name in sections:
        node = tree[name]
        owners = owner_map.get(name, [])
        tag_cls = "is-plugin" if owners else "is-core"
        desc = node.get("description")
        desc_html = (
            esc(" ".join(desc.split()))
            if desc
            else '<span class="config-index-nodesc">no description</span>'
        )
        rows.append(
            '<tr data-view-link="%s">'
            '<th scope="row"><a href="#%s"><code>%s</code></a></th>'
            '<td><span class="config-owner-tag %s">%s</span></td>'
            "<td>%s</td></tr>"
            % (
                esc(name),
                esc(name),
                esc(name),
                tag_cls,
                esc(owner_short(owners)),
                desc_html,
            )
        )
    return (
        '<section class="config-view is-active" data-view="overview">'
        '<p class="config-index-hint">%d configuration sections. Pick one to '
        "inspect its tree, or search across all of them.</p>"
        '<p class="config-index-count" role="status" aria-live="polite"></p>'
        '<table class="config-index"><thead><tr>'
        '<th scope="col">Section</th><th scope="col">Provided by</th>'
        '<th scope="col">What it configures</th></tr></thead>'
        "<tbody>%s</tbody></table></section>" % (len(sections), "".join(rows))
    )


def render_pager(prev_name, next_name):
    parts = []
    if prev_name:
        parts.append(
            '<a class="config-pager-prev" href="#%s"><code>%s</code></a>'
            % (esc(prev_name), esc(prev_name))
        )
    if next_name:
        parts.append(
            '<a class="config-pager-next" href="#%s"><code>%s</code></a>'
            % (esc(next_name), esc(next_name))
        )
    return '<span class="config-pager">%s</span>' % "".join(parts)


def render_detail(name, node, owner_map, prev_name, next_name):
    owners = owner_map.get(name, [])
    head = "<code>%s</code>" % esc(name)
    badge = node_badge(node)
    if badge:
        head += ' <span class="yang-type">%s</span>' % esc(badge)
    parts = [
        '<section class="config-view" data-view="%s" aria-label="%s configuration">'
        % (esc(name), esc(name)),
        '<div class="config-detail-bar">',
        '<a class="config-back" href="#overview">All sections</a>',
        render_pager(prev_name, next_name),
        "</div>",
        '<div class="config-detail-head">',
        "<h2>%s</h2>" % head,
        owner_detail_html(owners),
    ]
    desc = node.get("description")
    if desc:
        parts.append(
            '<p class="config-detail-desc">%s</p>' % esc(" ".join(desc.split()))
        )
    parts.append("</div>")
    children = node.get("children") or []
    if children:
        inner = "".join(
            render_child(c, name + "/" + c["name"], owner_map) for c in children
        )
        parts.append('<ul class="yang-tree">%s</ul>' % inner)
    parts.append("</section>")
    return "\n".join(parts)


FILTER_SCRIPT = """        <script>
            (function () {
                var root = document.querySelector("[data-config-explorer]");
                if (!root) return;
                var search = root.querySelector("#config-search");
                var views = {};
                root.querySelectorAll(".config-view").forEach(function (v) {
                    views[v.getAttribute("data-view")] = v;
                });
                var rows = Array.prototype.slice.call(
                    root.querySelectorAll(".config-index tbody tr")
                );
                var note = root.querySelector(".config-index-count");

                function setBranches(view, q) {
                    view.querySelectorAll("details").forEach(function (d) {
                        d.open =
                            q !== "" &&
                            d.textContent.toLowerCase().indexOf(q) !== -1;
                    });
                }

                function activate(name) {
                    if (!views[name]) name = "overview";
                    Object.keys(views).forEach(function (k) {
                        views[k].classList.toggle("is-active", k === name);
                    });
                    var q = search.value.trim().toLowerCase();
                    if (name !== "overview") setBranches(views[name], q);
                }

                function applySearch() {
                    var q = search.value.trim().toLowerCase();
                    var count = 0;
                    rows.forEach(function (r) {
                        var v = views[r.getAttribute("data-view-link")];
                        var match =
                            q === "" ||
                            (v && v.textContent.toLowerCase().indexOf(q) !== -1);
                        r.style.display = match ? "" : "none";
                        if (match) count++;
                    });
                    if (note) {
                        note.textContent =
                            q === ""
                                ? ""
                                : count +
                                  " section" +
                                  (count === 1 ? "" : "s") +
                                  " match \\u201c" +
                                  search.value.trim() +
                                  "\\u201d";
                    }
                    // Searching always returns to the index (the results
                    // list); clicking a result drills in with matching
                    // branches already expanded.
                    activate("overview");
                    if (q === "") {
                        Object.keys(views).forEach(function (k) {
                            if (k !== "overview") setBranches(views[k], "");
                        });
                    }
                }

                function targetFromHash() {
                    return (location.hash || "#overview").replace(/^#/, "");
                }

                window.addEventListener("hashchange", function () {
                    activate(targetFromHash());
                    // If the explorer has scrolled out of view, bring its top
                    // back under the header so you land at the section's top.
                    var top = root.getBoundingClientRect().top;
                    if (top < 60 || top > 160) {
                        window.scrollTo({
                            top: window.scrollY + top - 76,
                            behavior: "smooth",
                        });
                    }
                });
                root.addEventListener("click", function (e) {
                    var row =
                        e.target.closest &&
                        e.target.closest(".config-index tbody tr[data-view-link]");
                    if (row && !(e.target.closest && e.target.closest("a"))) {
                        location.hash = row.getAttribute("data-view-link");
                    }
                });
                search.addEventListener("input", applySearch);

                activate(targetFromHash());
            })();
        </script>
"""


# -- Markdown sibling: the whole tree by section (no interactive views) --


def render_child_markdown(node, path, owner_map, depth):
    indent = "  " * depth
    badge = node_badge(node)
    line = "%s- **%s**%s" % (
        indent,
        node_head(node),
        (" `%s`" % badge) if badge else "",
    )
    lines = [line]
    owners = owner_map.get(path, [])
    if owners:
        lines.append("%s  %s" % (indent, owner_line_markdown(owners)))
    desc = node.get("description")
    if desc:
        lines.append("%s  %s" % (indent, " ".join(desc.split())))
    for child in node.get("children") or []:
        lines.extend(
            render_child_markdown(
                child, path + "/" + child["name"], owner_map, depth + 1
            )
        )
    return lines


def owner_line_markdown(owners):
    def one(plugin):
        links = ", ".join(
            "[%s](%s)" % (y.rsplit("/", 1)[-1], yang_source_href(y))
            for y in plugin["yang_files"]
        )
        return "`%s`%s" % (plugin["name"], (" (%s)" % links) if links else "")

    return "*Provided by %s*" % "; ".join(one(p) for p in owners)


def render_section_markdown(name, node, owner_map):
    lines = ["## %s" % name, ""]
    owners = owner_map.get(name, [])
    if owners:
        lines.append(owner_line_markdown(owners))
        lines.append("")
    desc = node.get("description")
    if desc:
        lines.append(" ".join(desc.split()))
        lines.append("")
    for child in node.get("children") or []:
        lines.extend(
            render_child_markdown(child, name + "/" + child["name"], owner_map, 0)
        )
    lines.append("")
    return lines


def render_markdown(tree, owner_map, owned_count):
    sections = sorted(tree)
    parts = [
        "# Configuration Reference",
        "",
        "The complete Ze configuration as one tree: %d top-level sections "
        "(%d provided by plugins, the rest core), generated live from the "
        "YANG schema with `ze yang tree`. This is about the structure of the "
        "configuration -- every section, searchable and inspectable. See "
        "[the Configuration guide](%sdocs/features/configuration/) for a "
        "narrative walkthrough of BGP peer config specifically."
        % (len(sections), owned_count, sitelib.SITE_BASE),
        "",
    ]
    for name in sections:
        parts.extend(render_section_markdown(name, tree[name], owner_map))
    return "\n".join(parts).strip() + "\n"


def render(tree, owner_map):
    root = "../"
    sections = sorted(tree)
    warn_orphan_roots(tree, owner_map)
    owned_count = sum(1 for name in sections if owner_map.get(name))
    title = "Configuration Reference - Ze"
    desc = (
        "Browse the whole Ze configuration -- every section as a searchable "
        "index, generated live from the YANG schema."
    )
    out = [sitelib.page_head(title, desc, root, og_title=title, og_desc=desc)]
    out.append(
        '            <section aria-labelledby="config-ref-title" class="md-content reveal cat-platform">'
    )
    out.append('                <h1 id="config-ref-title">Configuration Reference</h1>')
    out.append(
        "                <p>The complete Ze configuration in one place: "
        "<strong>%d sections</strong> (%d provided by plugins, the rest "
        "core), generated live from the YANG schema with "
        "<code>ze yang tree</code>. The table below is the index -- pick a "
        "section to inspect its structure, or search across the whole "
        "configuration. Where a section is provided by a plugin, its owner "
        "and YANG source are shown. See the "
        '<a href="%sdocs/features/configuration/">Configuration guide</a> '
        "for a narrative walkthrough of BGP peer config specifically.</p>"
        % (len(sections), owned_count, root)
    )
    out.append('                <div class="config-explorer" data-config-explorer>')
    out.append('<script>document.documentElement.classList.add("config-js")</script>')
    out.append(
        '                <input id="config-search" type="search" '
        'placeholder="Search the whole configuration (section, leaf, type, plugin)..." '
        'aria-label="Search the configuration" />'
    )
    out.append(render_index(tree, owner_map, sections))
    for i, name in enumerate(sections):
        prev_name = sections[i - 1] if i > 0 else None
        next_name = sections[i + 1] if i < len(sections) - 1 else None
        out.append(render_detail(name, tree[name], owner_map, prev_name, next_name))
    out.append("                </div>")  # config-explorer
    out.append("            </section>")

    body = "\n".join(out)
    DEST.parent.mkdir(parents=True, exist_ok=True)
    DEST.write_text(body + "\n" + FILTER_SCRIPT + "\n" + sitelib.page_foot(root))
    sitelib.write_markdown_sibling(DEST, render_markdown(tree, owner_map, owned_count))
    print(
        "rendered %d config sections (%d plugin-provided) -> %s (+ index.md)"
        % (len(sections), owned_count, DEST)
    )


def main():
    plugins = load_plugins()
    tree = load_yang_tree()
    owner_map = build_owner_map(plugins)
    render(tree, owner_map)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
