#!/usr/bin/env python3
"""Render config-reference/index.html from data/plugin-registry.json.

Usage:
    tools/render-config-reference.py

Every plugin (all subsystems, not just BGP) grouped by the YANG config
root(s) its own Go registration declares -- see
tools/extract-plugin-registry.py, which must be re-run first (or via
tools/build.py) to refresh data/plugin-registry.json from the live Go
source. Grouping here is a tiered, purely mechanical fallback, run fresh
every render, never a hand-authored plugin->group mapping:

  1. The plugin's own ConfigRoots[0] (which YANG subtree its config lives
     under -- e.g. every bgp-* plugin declares ConfigRoots ["bgp"]).
  2. Else its first Dependencies entry, resolved one level if that
     dependency is itself a known plugin with its own ConfigRoots, else
     used as a literal group key.
  3. Else the prefix before the first "-" in its own name (e.g.
     "bgp-aigp" -> "bgp") -- covers BGP wire-codec plugins that declare
     neither ConfigRoots nor Dependencies but still carry the bgp- name
     convention.
  4. Else an explicit "ungrouped" bucket -- shown, not hidden, so a
     plugin with no declared grouping signal is still visible instead of
     silently dropped.

Each plugin's own YANG module(s) (resolved by extract-plugin-registry.py
via its real Go import statement, not a directory-name guess) are read
fresh from ../main/ and shown as real source, not a re-parsed summary --
nothing beats showing the actual schema.
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


def group_key(plugin, by_name, _seen=None):
    if plugin["config_roots"]:
        return plugin["config_roots"][0], "config-roots"
    if plugin["dependencies"]:
        seen = _seen or {plugin["name"]}
        dep = plugin["dependencies"][0]
        dep_plugin = by_name.get(dep)
        if dep_plugin and dep not in seen:
            # Resolve the dependency through the same tiered rule (it may
            # itself have no ConfigRoots but a usable name-prefix, e.g.
            # bgp-rr depends on bgp-adj-rib-in, which has neither
            # ConfigRoots nor Dependencies but resolves to "bgp" via its
            # own name prefix) -- a single-level dep_plugin["config_roots"]
            # check alone would leave bgp-rr in a stray "bgp-adj-rib-in"
            # group of one instead of joining the real "bgp" group.
            key, _reason = group_key(dep_plugin, by_name, seen | {dep})
            return key, "dependencies"
        return dep, "dependencies"
    if "-" in plugin["name"]:
        return plugin["name"].split("-", 1)[0], "name-prefix"
    return "ungrouped", "ungrouped"


def prettify(key):
    return key.replace("-", " ").replace("/", " / ").title()


def read_yang(rel_path):
    path = MAIN_REPO / rel_path
    if not path.exists():
        return None
    return path.read_text(errors="replace")


def render_yang_block(rel_path):
    content = read_yang(rel_path)
    filename = rel_path.rsplit("/", 1)[-1]
    if content is None:
        return (
            '<p class="config-yang-missing">%s (not found on disk at render time)</p>'
            % html.escape(rel_path)
        )
    return (
        '<details class="config-yang">'
        "<summary>%s</summary>"
        "<pre><code>%s</code></pre>"
        "</details>"
    ) % (html.escape(filename), html.escape(content))


def render_plugin(plugin):
    parts = ['<article class="config-plugin">']
    meta_bits = []
    if plugin["config_roots"]:
        meta_bits.append(
            "config root: <code>%s</code>"
            % ", ".join(html.escape(r) for r in plugin["config_roots"])
        )
    if plugin["dependencies"]:
        meta_bits.append(
            "depends on: <code>%s</code>"
            % ", ".join(html.escape(d) for d in plugin["dependencies"])
        )
    if plugin["optional_dependencies"]:
        meta_bits.append(
            "optional: <code>%s</code>"
            % ", ".join(html.escape(d) for d in plugin["optional_dependencies"])
        )
    parts.append(
        '<h3>%s <span class="config-plugin-src">%s</span></h3>'
        % (html.escape(plugin["name"]), html.escape(plugin["source_dir"]))
    )
    if plugin["description"]:
        parts.append("<p>%s</p>" % html.escape(plugin["description"]))
    if meta_bits:
        parts.append(
            '<p class="config-plugin-meta">%s</p>' % " &middot; ".join(meta_bits)
        )
    if plugin["yang_files"]:
        for rel_path in plugin["yang_files"]:
            parts.append(render_yang_block(rel_path))
    else:
        parts.append(
            '<p class="config-plugin-meta">No YANG module of its own '
            "(reads config defined by another plugin, or has none).</p>"
        )
    parts.append("</article>")
    return "\n".join(parts)


def render_group(key, members):
    total_yang = sum(len(p["yang_files"]) for p in members)
    parts = [
        '<details class="config-group">',
        "<summary>%s <code>%s</code> "
        '<span class="config-group-count">%d plugin%s, %d module%s</span></summary>'
        % (
            html.escape(prettify(key)),
            html.escape(key),
            len(members),
            "" if len(members) == 1 else "s",
            total_yang,
            "" if total_yang == 1 else "s",
        ),
        '<div class="config-group-body">',
    ]
    for plugin in members:
        parts.append(render_plugin(plugin))
    parts.append("</div>")
    parts.append("</details>")
    return "\n".join(parts)


FILTER_SCRIPT = """        <script>
            document.addEventListener("DOMContentLoaded", function () {
                var input = document.getElementById("config-search");
                var groups = document.querySelectorAll(".config-group");
                if (!input) return;
                input.addEventListener("input", function () {
                    var q = input.value.trim().toLowerCase();
                    groups.forEach(function (group) {
                        var plugins = group.querySelectorAll(".config-plugin");
                        var anyVisible = false;
                        plugins.forEach(function (plugin) {
                            var match = q === "" || plugin.textContent.toLowerCase().indexOf(q) !== -1;
                            plugin.style.display = match ? "" : "none";
                            if (match) anyVisible = true;
                        });
                        group.style.display = anyVisible ? "" : "none";
                        if (q !== "") {
                            group.open = anyVisible;
                        }
                    });
                });
            });
        </script>
"""


def build_groups(plugins):
    by_name = {p["name"]: p for p in plugins}
    groups = {}
    for plugin in plugins:
        key, _reason = group_key(plugin, by_name)
        groups.setdefault(key, []).append(plugin)
    for key, members in groups.items():
        members.sort(key=lambda p, key=key: (p["name"] != key, p["name"]))
    return groups


def render_plugin_markdown(plugin):
    parts = ["### %s" % plugin["name"], ""]
    meta_bits = ["source: `%s`" % plugin["source_dir"]]
    if plugin["config_roots"]:
        meta_bits.append("config root: `%s`" % ", ".join(plugin["config_roots"]))
    if plugin["dependencies"]:
        meta_bits.append("depends on: `%s`" % ", ".join(plugin["dependencies"]))
    if plugin["optional_dependencies"]:
        meta_bits.append(
            "optional: `%s`" % ", ".join(plugin["optional_dependencies"])
        )
    parts.append(" -- ".join(meta_bits))
    parts.append("")
    if plugin["description"]:
        parts.append(plugin["description"])
        parts.append("")
    if plugin["yang_files"]:
        for rel_path in plugin["yang_files"]:
            content = read_yang(rel_path)
            filename = rel_path.rsplit("/", 1)[-1]
            parts.append("`%s`" % filename)
            parts.append("")
            if content is None:
                parts.append("*(not found on disk at render time)*")
            else:
                parts.append("```yang\n%s\n```" % content.strip("\n"))
            parts.append("")
    else:
        parts.append(
            "No YANG module of its own (reads config defined by another "
            "plugin, or has none)."
        )
        parts.append("")
    return "\n".join(parts)


def render_markdown(plugins, groups, total_yang):
    parts = [
        "# Configuration Reference",
        "",
        "%d plugins across %d groups, %d YANG modules total. Generated "
        "from every real `registry.Registration{}` in `../main/internal/` "
        "and the YANG modules each one actually imports -- grouped by the "
        "config root each plugin's own registration declares, not a "
        "hand-picked category list. Every subsystem here, not only BGP: "
        "see [the Configuration guide](%sdocs/features/configuration/) "
        "for a narrative walkthrough of BGP peer config specifically."
        % (len(plugins), len(groups), total_yang, sitelib.SITE_BASE),
        "",
    ]
    for key in sorted(groups):
        members = groups[key]
        parts.append("## %s (`%s`, %d plugins)" % (prettify(key), key, len(members)))
        parts.append("")
        for plugin in members:
            parts.append(render_plugin_markdown(plugin))
    return "\n".join(parts).strip() + "\n"


def render(plugins, groups):
    root = "../"
    total_yang = sum(len(p["yang_files"]) for p in plugins)
    title = "Configuration Reference - Ze"
    desc = (
        "Every plugin's config syntax, generated from the live Go registration "
        "and YANG modules -- %d plugins across %d groups, not just BGP."
        % (len(plugins), len(groups))
    )
    out = [sitelib.page_head(title, desc, root, og_title=title, og_desc=desc)]
    out.append(
        '            <section aria-labelledby="config-ref-title" class="md-content reveal cat-platform">'
    )
    out.append('                <h1 id="config-ref-title">Configuration Reference</h1>')
    out.append(
        "                <p>%d plugins across %d groups, %d YANG modules total. "
        "Generated from every real <code>registry.Registration{}</code> in "
        "<code>../main/internal/</code> and the YANG modules each one "
        "actually imports -- grouped by the config root each plugin's own "
        "registration declares, not a hand-picked category list. "
        "Every subsystem here, not only BGP: see the "
        '<a href="%sdocs/features/configuration/">Configuration guide</a> '
        "for a narrative walkthrough of BGP peer config specifically.</p>"
        % (len(plugins), len(groups), total_yang, root)
    )
    out.append(
        '                <input id="config-search" type="search" '
        'placeholder="Filter by plugin name, description, or config root..." '
        'aria-label="Filter configuration reference" />'
    )
    for key in sorted(groups):
        out.append(render_group(key, groups[key]))
    out.append("            </section>")

    body = "\n".join(out)
    DEST.parent.mkdir(parents=True, exist_ok=True)
    DEST.write_text(body + "\n" + FILTER_SCRIPT + "\n" + sitelib.page_foot(root))
    sitelib.write_markdown_sibling(DEST, render_markdown(plugins, groups, total_yang))
    print(
        "rendered %d plugins across %d groups (%d yang modules) -> %s (+ index.md)"
        % (len(plugins), len(groups), total_yang, DEST)
    )


def main():
    plugins = load_plugins()
    groups = build_groups(plugins)
    render(plugins, groups)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
