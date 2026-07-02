#!/usr/bin/env python3
"""Render config-reference/index.html: the whole Ze configuration as one
searchable, collapsible tree of sections.

Usage:
    tools/render-config-reference.py

This page is about the *configuration*, not the plugins. It shows every
configuration section Ze has -- the complete YANG config tree -- as one
structure a reader can scan, search, and drill into. The tree is produced
by running the live `ze yang tree --json --config` (cached by
tools/extract-yang-config-tree.py to data/yang-config-tree.json), the same
unified walker Ze's own CLI uses, which already merges every config module
in the tree into one shape (no per-plugin fragments).

Every container and YANG list is its own collapsible <details>, so a big
section like `bgp` (hundreds of nodes) opens as a compact set of
sub-sections a reader expands only where they care. Each node's summary
always shows its name, type, and description, so scanning collapsed
sections alone already tells you the structure.

Where a section is provided by a plugin, that ownership is shown as an
annotation on the section, resolved from data/plugin-registry.json (see
tools/extract-plugin-registry.py): a plugin's ConfigRoots declares which
config path it owns (e.g. `bgp`, or the nested `fib/kernel`), so the
section is annotated with the owning plugin(s) and a link to each one's
real YANG source on Codeberg. Sections with no plugin owner are core
config and simply carry no annotation. Plugins that declare no config root
(wire codecs, filters) contribute behaviour, not config structure, so they
do not appear here -- this page is the config, not a plugin inventory.
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
            % html.escape(rel_path)
        )
    return '<a href="%s" target="_blank" rel="noopener"><code>%s</code></a>' % (
        html.escape(yang_source_href(rel_path)),
        html.escape(filename),
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
    """Compact at-a-glance ownership hint shown in a section's summary."""
    if not owners:
        return ""
    if len(owners) == 1:
        text = "provided by %s" % owners[0]["name"]
    else:
        text = "provided by %d plugins" % len(owners)
    return ' <span class="config-owner">%s</span>' % html.escape(text)


def node_label(node, owners):
    label = "<code>%s</code>" % html.escape(node_head(node))
    badge = node_badge(node)
    if badge:
        label += ' <span class="yang-type">%s</span>' % html.escape(badge)
    label += owner_marker(owners)
    desc = node.get("description")
    if desc:
        label += ' <span class="yang-desc">%s</span>' % html.escape(
            " ".join(desc.split())
        )
    return label


def owner_detail_html(owners):
    """The expanded ownership block shown inside a section (below its
    summary): who provides it and a link to each one's real YANG source."""
    if not owners:
        return ""

    def one(plugin):
        links = ", ".join(render_yang_link(y) for y in plugin["yang_files"])
        src = " &mdash; %s" % links if links else ""
        return "<code>%s</code>%s" % (html.escape(plugin["name"]), src)

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
    ) % (label, owner_detail_html(owners), inner)


def render_section(name, node, owner_map):
    owners = owner_map.get(name, [])
    label = node_label(node, owners)
    children = node.get("children") or []
    inner = "".join(
        render_child(c, name + "/" + c["name"], owner_map) for c in children
    )
    body = owner_detail_html(owners)
    if inner:
        body += '<ul class="yang-tree">%s</ul>' % inner
    return '<details class="config-section"><summary>%s</summary>%s</details>' % (
        label,
        body,
    )


FILTER_SCRIPT = """        <script>
            document.addEventListener("DOMContentLoaded", function () {
                var input = document.getElementById("config-search");
                var sections = document.querySelectorAll(".config-section");
                if (!input) return;
                input.addEventListener("input", function () {
                    var q = input.value.trim().toLowerCase();
                    sections.forEach(function (section) {
                        var match =
                            q === "" ||
                            section.textContent.toLowerCase().indexOf(q) !== -1;
                        section.style.display = match ? "" : "none";
                        if (q === "") {
                            // Reset to the clean collapsed overview.
                            section.open = false;
                            section
                                .querySelectorAll("details")
                                .forEach(function (d) {
                                    d.open = false;
                                });
                        } else if (match) {
                            // Open only the branches on the path to a match --
                            // a parent contains its child's text, so ancestors
                            // of a hit open while unrelated siblings stay shut.
                            section.open = true;
                            section
                                .querySelectorAll("details")
                                .forEach(function (d) {
                                    d.open =
                                        d.textContent.toLowerCase().indexOf(q) !==
                                        -1;
                                });
                        }
                    });
                });
            });
        </script>
"""


def render_section_markdown(name, node, owner_map):
    lines = ["## %s" % name, ""]
    owners = owner_map.get(name, [])
    if owners:
        lines.append(owner_line_markdown(owners))
        lines.append("")
    for child in node.get("children") or []:
        lines.extend(
            render_child_markdown(child, name + "/" + child["name"], owner_map, 0)
        )
    lines.append("")
    return lines


def owner_line_markdown(owners):
    def one(plugin):
        links = ", ".join(
            "[%s](%s)" % (y.rsplit("/", 1)[-1], yang_source_href(y))
            for y in plugin["yang_files"]
        )
        return "`%s`%s" % (plugin["name"], (" (%s)" % links) if links else "")

    return "*Provided by %s*" % "; ".join(one(p) for p in owners)


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
    owned_count = sum(1 for name in sections if owner_map.get(name))
    title = "Configuration Reference - Ze"
    desc = (
        "The complete Ze configuration as one searchable tree -- every "
        "section, generated live from the YANG schema."
    )
    out = [sitelib.page_head(title, desc, root, og_title=title, og_desc=desc)]
    out.append(
        '            <section aria-labelledby="config-ref-title" class="md-content reveal cat-platform">'
    )
    out.append('                <h1 id="config-ref-title">Configuration Reference</h1>')
    out.append(
        "                <p>The complete Ze configuration as one tree: "
        "<strong>%d top-level sections</strong> (%d provided by plugins, the "
        "rest core), generated live from the YANG schema with "
        "<code>ze yang tree</code>. This page is about the structure of the "
        "configuration itself -- every section, searchable and inspectable. "
        "Where a section is provided by a plugin, its owner and YANG source "
        "are shown. See the "
        '<a href="%sdocs/features/configuration/">Configuration guide</a> '
        "for a narrative walkthrough of BGP peer config specifically.</p>"
        % (len(sections), owned_count, root)
    )
    out.append(
        '                <input id="config-search" type="search" '
        'placeholder="Search the whole configuration (section, leaf, type, plugin)..." '
        'aria-label="Search the configuration" />'
    )
    for name in sections:
        out.append(render_section(name, tree[name], owner_map))
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
