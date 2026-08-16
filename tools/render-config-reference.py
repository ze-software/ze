#!/usr/bin/env python3
"""Render reference/configuration/index.html: the whole Ze configuration as a
breadcrumb-navigated browser -- the same table-of-children presentation at
every level.

Usage:
    tools/render-config-reference.py

This page is about the *configuration*, not the plugins. Every level of the
config -- the top-level sections and every container/list inside them -- is
shown the same way: a table of that node's immediate children (setting name,
owning plugin, description). A child that has children of its own is a link;
clicking it makes that child the current level and shows its children. A
breadcrumb walks back up. There is no separate index and no inline
expand-arrow tree: the landing view is just level 0 of the same browser.

The tree is produced by running the live `ze yang tree --json --config`
(cached by tools/extract-yang-config-tree.py to data/yang-config-tree.json),
the same unified walker Ze's own CLI uses, which already merges every config
module in the tree into one shape (no per-plugin fragments). The whole tree
is embedded as JSON and each level is rendered on demand in the browser, so
the page ships the data once instead of pre-rendering hundreds of panels;
without JavaScript it falls back to the plain-text index.md sibling.

Where a section is provided by a plugin, that ownership is shown as an
annotation, resolved from data/plugin-registry.json (see
tools/extract-plugin-registry.py): a plugin's ConfigRoots declares which
config path it owns (e.g. `bgp`, or the nested `fib/kernel` / `anomaly/detect`),
so the owning plugin(s) and a link to each one's real YANG source on GitHub
are shown against that exact node. Ownership resolves at the right depth, so
`anomaly` is a core container whose `detect` and `shape` children each carry
their plugin. A ConfigRoot that resolves to no node in the config tree (an
../main mid-refactor mismatch) is reported by warn_orphan_roots and shows as
core rather than silently swallowing the ownership. Plugins that declare no
config root (wire codecs, filters) contribute behaviour, not config
structure, so they do not appear here -- this page is the config, not a
plugin inventory.
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
DEST = GH_PAGES / "reference" / "configuration" / "index.html"

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
        sitelib.warn(
            "config root %r (declared by %s) resolves to no node in "
            "the YANG config tree -- its ownership is not shown; check "
            "../main for a ConfigRoots vs YANG container mismatch" % (root, names)
        )
    return orphans


def yang_source_href(rel_path):
    return "%s/%s" % (sitelib.REPO_BLOB, rel_path)


def node_head(node):
    """A node's own line, e.g. "peer <name>" for a YANG list keyed by
    "name", or plain "hold-time" for a leaf -- the shape an operator would
    type, not the raw YANG "list[name]" kind string. Kept in sync with the
    nodeHead() twin in the browser script."""
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


def owner_short(owners):
    if not owners:
        return "core"
    if len(owners) == 1:
        return owners[0]["name"]
    return "%d plugins" % len(owners)


def build_owners_data(owner_map):
    """path-string -> {label, plugins:[{name, yang:[{file, href}]}]} embedded
    as JSON for the browser to annotate each node with its owner."""
    data = {}
    for path, plugins in owner_map.items():
        data[path] = {
            "label": owner_short(plugins),
            "plugins": [
                {
                    "name": p["name"],
                    "yang": [
                        {"file": y.rsplit("/", 1)[-1], "href": yang_source_href(y)}
                        for y in p["yang_files"]
                    ],
                }
                for p in plugins
            ],
        }
    return data


def embed_json(obj):
    # Minified, with "<" escaped so the JSON can't close its <script> host.
    return json.dumps(obj, separators=(",", ":")).replace("<", "\\u003c")


BROWSER_SCRIPT = """        <script>
            (function () {
                var root = document.querySelector("[data-config-explorer]");
                if (!root) return;
                var treeEl = document.getElementById("config-tree");
                var ownersEl = document.getElementById("config-owners");
                if (!treeEl || !ownersEl) return;
                var tree = JSON.parse(treeEl.textContent);
                var owners = JSON.parse(ownersEl.textContent);
                var rootChildren = Object.keys(tree)
                    .sort()
                    .map(function (k) { return tree[k]; });
                var search = root.querySelector("#config-search");
                var crumbs = root.querySelector(".config-crumbs");
                var level = root.querySelector(".config-level");

                var ENT = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" };
                function esc(s) {
                    return String(s).replace(/[&<>"]/g, function (c) { return ENT[c]; });
                }
                function ws(s) { return String(s).replace(/\\s+/g, " ").trim(); }
                function nodeHead(n) {
                    var k = n.kind || "";
                    if (k.slice(0, 5) === "list[" && k.slice(-1) === "]")
                        return n.name + " <" + k.slice(5, -1) + ">";
                    return n.name;
                }
                function nodeBadge(n) {
                    var k = n.kind || "", t = n.type || "";
                    if ((k === "leaf" || k === "leaf-list") && t)
                        return k === "leaf-list" ? t + "[]" : t;
                    if (k.slice(0, 5) === "list[") return "list";
                    return k;
                }
                function nameCell(n, pathStr, drill) {
                    var badge = nodeBadge(n);
                    var inner = "<code>" + esc(nodeHead(n)) + "</code>" +
                        (badge ? ' <span class="yang-type">' + esc(badge) + "</span>" : "");
                    return drill ? '<a href="#' + esc(pathStr) + '">' + inner + "</a>" : inner;
                }
                // A node's own ConfigRoot owner, else the nearest ancestor
                // ConfigRoot's owner but only when that root has a *single*
                // owner -- inheriting a multi-owner root (bgp, environment,
                // ...) down would wrongly attribute its core/shared children
                // (e.g. environment/api-server) to plugins that only augment
                // part of it. So single-owner subtrees fill; shared ones stay
                // blank, and the header separately marks them shared vs core.
                function effectiveOwner(pathStr) {
                    if (owners[pathStr]) return owners[pathStr];
                    var parts = pathStr ? pathStr.split("/") : [];
                    parts.pop();
                    while (parts.length) {
                        var o = owners[parts.join("/")];
                        if (o) return o.plugins.length === 1 ? o : null;
                        parts.pop();
                    }
                    return null;
                }
                function ownerContext(pathStr) {
                    if (owners[pathStr]) return { owner: owners[pathStr] };
                    var parts = pathStr ? pathStr.split("/") : [];
                    parts.pop();
                    while (parts.length) {
                        var o = owners[parts.join("/")];
                        if (o) return o.plugins.length === 1 ? { owner: o } : { shared: o };
                        parts.pop();
                    }
                    return { core: true };
                }
                function ownerTag(pathStr) {
                    var o = effectiveOwner(pathStr);
                    return o
                        ? '<span class="config-owner-tag is-plugin">' + esc(o.label) + "</span>"
                        : "";
                }
                function ownerLine(o) {
                    function one(p) {
                        var links = p.yang.map(function (y) {
                            return '<a href="' + esc(y.href) +
                                '" target="_blank" rel="noopener"><code>' +
                                esc(y.file) + "</code></a>";
                        }).join(", ");
                        return "<code>" + esc(p.name) + "</code>" +
                            (links ? " &mdash; " + links : "");
                    }
                    if (o.plugins.length === 1)
                        return '<p class="config-owner-detail">Provided by ' +
                            one(o.plugins[0]) + "</p>";
                    return '<details class="config-owners"><summary>Provided by ' +
                        o.plugins.length + " plugins</summary><ul>" +
                        o.plugins.map(function (p) { return "<li>" + one(p) + "</li>"; }).join("") +
                        "</ul></details>";
                }
                function ownerDetail(pathStr) {
                    var ctx = ownerContext(pathStr);
                    if (ctx.owner) return ownerLine(ctx.owner);
                    if (ctx.core)
                        return '<p class="config-owner-detail">Core configuration ' +
                            "&mdash; no plugin owner.</p>";
                    return '<p class="config-owner-detail">Part of a plugin-shared ' +
                        "container (" + esc(ctx.shared.label) + ").</p>";
                }

                function nodeAt(P) {
                    var kids = rootChildren, node = null;
                    for (var i = 0; i < P.length; i++) {
                        node = null;
                        for (var j = 0; j < kids.length; j++)
                            if (kids[j].name === P[i]) { node = kids[j]; break; }
                        if (!node) return null;
                        kids = node.children || [];
                    }
                    return { node: node, children: kids };
                }

                function tableFor(P, kids) {
                    if (!kids.length)
                        return '<p class="config-empty">No settings under this node.</p>';
                    var rows = "";
                    for (var i = 0; i < kids.length; i++) {
                        var c = kids[i];
                        var ps = P.concat([c.name]).join("/");
                        var drill = (c.children || []).length > 0;
                        var desc = c.description
                            ? esc(ws(c.description))
                            : '<span class="config-index-nodesc">' +
                              (drill ? "" : "&mdash;") + "</span>";
                        rows += "<tr" +
                            (drill ? ' class="is-drillable" data-path="' + esc(ps) + '"' : "") +
                            '><th scope="row">' + nameCell(c, ps, drill) + "</th>" +
                            "<td>" + ownerTag(ps) + "</td>" +
                            "<td>" + desc + "</td></tr>";
                    }
                    return '<div class="config-index-wrap"><table class="config-index"><thead><tr>' +
                        '<th scope="col">Setting</th><th scope="col">Provided by</th>' +
                        '<th scope="col">Description</th></tr></thead><tbody>' +
                        rows + "</tbody></table></div>";
                }

                function renderCrumbs(P) {
                    var parts = ['<a href="#">Configuration</a>'], acc = [];
                    for (var i = 0; i < P.length; i++) {
                        acc = acc.concat([P[i]]);
                        if (i < P.length - 1)
                            parts.push('<a href="#' + esc(acc.join("/")) + '"><code>' +
                                esc(P[i]) + "</code></a>");
                        else
                            parts.push('<span class="config-crumb-current"><code>' +
                                esc(P[i]) + "</code></span>");
                    }
                    crumbs.innerHTML = parts.join('<span class="config-crumb-sep">/</span>');
                    crumbs.style.display = "";
                }

                function renderLevel(P) {
                    var at = nodeAt(P);
                    if (!at) { location.hash = ""; return; }
                    renderCrumbs(P);
                    var html = "";
                    if (P.length > 0) {
                        var n = at.node, badge = nodeBadge(n);
                        html += '<div class="config-detail-head"><h2><code>' +
                            esc(nodeHead(n)) + "</code>" +
                            (badge ? ' <span class="yang-type">' + esc(badge) + "</span>" : "") +
                            "</h2>" + ownerDetail(P.join("/")) +
                            (n.description
                                ? '<p class="config-detail-desc">' + esc(ws(n.description)) + "</p>"
                                : "") + "</div>";
                    } else {
                        html += '<p class="config-index-hint">' + rootChildren.length +
                            " configuration sections. Pick one to inspect its structure, " +
                            "or search across the whole configuration.</p>";
                    }
                    html += tableFor(P, at.children);
                    level.innerHTML = html;
                }

                function renderSearch(q) {
                    crumbs.style.display = "none";
                    var ql = q.toLowerCase(), results = [];
                    function walk(n, P) {
                        var o = owners[P.join("/")];
                        var hay = (nodeHead(n) + " " + (n.type || "") + " " +
                            (n.description || "")).toLowerCase();
                        if (o) hay += " " + o.label.toLowerCase() + " " +
                            o.plugins.map(function (p) { return p.name; }).join(" ").toLowerCase();
                        if (hay.indexOf(ql) !== -1) results.push({ n: n, P: P });
                        var kids = n.children || [];
                        for (var i = 0; i < kids.length; i++)
                            walk(kids[i], P.concat([kids[i].name]));
                    }
                    for (var i = 0; i < rootChildren.length; i++)
                        walk(rootChildren[i], [rootChildren[i].name]);
                    var rows = "";
                    for (var k = 0; k < results.length; k++) {
                        var r = results[k], drill = (r.n.children || []).length > 0;
                        var target = drill ? r.P.join("/") : r.P.slice(0, -1).join("/");
                        var label = r.P.map(function (x) { return esc(x); })
                            .join(' <span class="config-crumb-sep">/</span> ');
                        rows += '<tr class="is-drillable" data-path="' + esc(target) + '">' +
                            '<th scope="row"><a href="#' + esc(target) + '">' + label + "</a></th>" +
                            "<td>" + ownerTag(r.P.join("/")) + "</td>" +
                            "<td>" + (r.n.description ? esc(ws(r.n.description)) : "") +
                            "</td></tr>";
                    }
                    var head = '<p class="config-index-count">' + results.length +
                        ' match "' + esc(q) + '"</p>';
                    level.innerHTML = head + (results.length
                        ? '<div class="config-index-wrap"><table class="config-index"><thead><tr><th scope="col">Path</th>' +
                          '<th scope="col">Provided by</th><th scope="col">Description</th>' +
                          "</tr></thead><tbody>" + rows + "</tbody></table></div>"
                        : "");
                }

                function pathFromHash() {
                    var h = (location.hash || "").replace(/^#/, "");
                    return h === "" ? [] : h.split("/");
                }
                function refresh() {
                    var q = search.value.trim();
                    if (q !== "") renderSearch(q);
                    else renderLevel(pathFromHash());
                }

                window.addEventListener("hashchange", function () {
                    if (search.value.trim() !== "") search.value = "";
                    renderLevel(pathFromHash());
                    var top = root.getBoundingClientRect().top;
                    if (top < 60 || top > 160)
                        window.scrollTo({ top: window.scrollY + top - 76, behavior: "smooth" });
                });
                root.addEventListener("click", function (e) {
                    var tr = e.target.closest && e.target.closest("tr.is-drillable");
                    if (tr && !(e.target.closest && e.target.closest("a")))
                        location.hash = tr.getAttribute("data-path");
                });
                search.addEventListener("input", refresh);

                refresh();
            })();
        </script>
"""


# -- Markdown sibling: the whole tree by section (plain text, no browser) --


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
    root = "../../"
    sections = sorted(tree)
    warn_orphan_roots(tree, owner_map)
    owned_count = sum(1 for name in sections if owner_map.get(name))
    owners_data = build_owners_data(owner_map)
    title = "Configuration Reference - Ze"
    desc = (
        "Browse the whole Ze configuration -- every level shown the same way, "
        "generated live from the YANG schema."
    )
    out = [
        sitelib.page_head(
            title,
            desc,
            root,
            og_title=title,
            og_desc=desc,
            page_key="reference/configuration/",
        )
    ]
    out.append(
        '            <section aria-labelledby="config-ref-title" class="md-content reveal cat-operate">'
    )
    out.append(
        sitelib.page_hero(
            "Configuration Reference",
            (
                "The complete Ze configuration in one place: "
                "<strong>%d sections</strong> (%d provided by plugins, the rest "
                "core), generated live from the YANG schema with "
                "<code>ze yang tree</code>. Every level -- sections and the "
                "containers inside them -- is browsed the same way: pick a setting to "
                "step into it, or search across the whole configuration. Where a "
                "setting is provided by a plugin, its owner and YANG source are "
                "shown. See the "
                '<a href="%sdocs/features/configuration/">Configuration guide</a> '
                "for a narrative walkthrough of BGP peer config specifically."
                % (len(sections), owned_count, root)
            ),
            "Reference",
            h1_id="config-ref-title",
            lead_html=True,
        )
    )
    out.append('                <div class="config-explorer" data-config-explorer>')
    out.append('<script>document.documentElement.classList.add("config-js")</script>')
    out.append(
        '                <input id="config-search" type="search" '
        'placeholder="Search the whole configuration (setting, type, plugin)..." '
        'aria-label="Search the configuration" />'
    )
    out.append(
        '                <nav class="config-crumbs" aria-label="Breadcrumb"></nav>'
    )
    out.append('                <div class="config-level"></div>')
    out.append(
        '                <noscript><p class="config-noscript">This config browser '
        "needs JavaScript. The whole configuration is also available as "
        '<a href="index.md">plain text</a>.</p></noscript>'
    )
    out.append("                </div>")  # config-explorer
    out.append(
        '                <script id="config-tree" type="application/json">%s</script>'
        % embed_json(tree)
    )
    out.append(
        '                <script id="config-owners" type="application/json">%s</script>'
        % embed_json(owners_data)
    )
    out.append("            </section>")

    body = "\n".join(out)
    DEST.parent.mkdir(parents=True, exist_ok=True)
    DEST.write_text(body + "\n" + BROWSER_SCRIPT + "\n" + sitelib.page_foot(root))
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
