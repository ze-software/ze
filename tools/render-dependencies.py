#!/usr/bin/env -S uv run python3
"""Render a dependency reference from ../main/go.mod + data/dependencies.json.

Usage:
    tools/render-dependencies.py

Parses ../main/go.mod for the direct dependencies (require lines with no
trailing "// indirect") and their pinned versions -- that part can never go
stale, it's read straight from the module file every run. The "why" text
per dependency is curated in data/dependencies.json, grounded in actual
import sites in the codebase (not guessed from a package's own README).

Warns on stderr if go.mod gains a direct dependency with no entry in
data/dependencies.json, or if data/dependencies.json has an entry for a
module go.mod no longer lists as direct -- the same drift class as the
Features/CLI count checks in build.py, just for this page's data source.
"""

import html
import json
import pathlib
import re
import sys

import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
GO_MOD = GH_PAGES.parent / "main" / "go.mod"
DATA = GH_PAGES / "data" / "dependencies.json"
DEST = GH_PAGES / "dependencies" / "index.html"

REQUIRE_BLOCK_RE = re.compile(r"require \(\n(.*?)\n\)", re.DOTALL)
REQUIRE_LINE_RE = re.compile(r"^\s*(\S+)\s+(\S+)(\s*//\s*indirect)?\s*$")


def parse_direct_deps():
    if not GO_MOD.exists():
        print("error: %s not found" % GO_MOD, file=sys.stderr)
        sys.exit(1)
    text = GO_MOD.read_text()
    versions = {}
    for block in REQUIRE_BLOCK_RE.findall(text):
        for line in block.splitlines():
            m = REQUIRE_LINE_RE.match(line)
            if not m:
                continue
            module, version, indirect = m.groups()
            if indirect:
                continue
            versions[module] = version
    return versions


def load_data():
    return json.loads(DATA.read_text())


def check_drift(versions, data):
    """Report drift between go.mod and data/dependencies.json.

    Each mismatch goes through sitelib.warn, which prints it now and makes
    build.py fail the whole build at the very end -- a plain stderr warning
    alone gets scrolled past, so this is a hard gate like the other drift
    checks. The page is still rendered (see main), so the site stays valid;
    only the exit code goes red until go.mod and the curated list agree.
    """
    curated = {
        entry["module"]: cat["name"]
        for cat in data["categories"]
        for entry in cat["modules"]
    }
    missing = sorted(set(versions) - set(curated))
    stale = sorted(set(curated) - set(versions))
    for module in missing:
        sitelib.warn(
            "%s is a direct dependency in go.mod with no entry in "
            "data/dependencies.json -- add one" % module
        )
    for module in stale:
        sitelib.warn(
            "data/dependencies.json has %s but go.mod no longer "
            "lists it as a direct dependency -- remove the entry" % module
        )


def render_row(module, version, why):
    return ("<tr><td><code>%s</code></td><td><code>%s</code></td><td>%s</td></tr>") % (
        html.escape(module),
        html.escape(version),
        sitelib.bold(why),
    )


def render_group(category, versions):
    entries = category["modules"]
    parts = [
        '<details class="dep-group" open>',
        '<summary>%s <span class="dep-group-count">%d</span></summary>'
        % (html.escape(category["name"]), len(entries)),
        "<table><thead><tr><th>Module</th><th>Version</th><th>Why we use it</th></tr></thead><tbody>",
    ]
    for entry in entries:
        version = versions.get(entry["module"], "?")
        parts.append(render_row(entry["module"], version, entry["why"]))
    parts.append("</tbody></table></details>")
    return "\n".join(parts)


def render_markdown(versions, data, total):
    parts = [
        "# Dependencies",
        "",
        "Ze is Go, and Go code leans on packages. %d direct dependencies, "
        "read straight from `go.mod` so the list and versions can't drift "
        "-- each one with a plain-English reason it's there, grounded in "
        "where it's actually imported, not its own pitch." % total,
        "",
    ]
    for category in data["categories"]:
        parts.append("## %s (%d)" % (category["name"], len(category["modules"])))
        parts.append("")
        parts.append("| Module | Version | Why we use it |")
        parts.append("| --- | --- | --- |")
        for entry in category["modules"]:
            version = versions.get(entry["module"], "?")
            why = entry["why"].replace("|", "\\|")
            parts.append("| `%s` | `%s` | %s |" % (entry["module"], version, why))
        parts.append("")
    return "\n".join(parts).strip() + "\n"


def render(versions, data):
    root = "../"
    total = sum(len(cat["modules"]) for cat in data["categories"])
    title = "Dependencies - Ze"
    desc = (
        "Every direct Go dependency Ze ships with and why, generated from "
        "go.mod -- %d packages across %d groups." % (total, len(data["categories"]))
    )
    out = [
        sitelib.page_head(
            title, desc, root, og_title=title, og_desc=desc, page_key="dependencies/"
        )
    ]
    out.append(
        '            <section aria-labelledby="dependencies-title" class="md-content reveal cat-platform">'
    )
    out.append(
        sitelib.page_hero(
            "Dependencies",
            (
                "Ze is Go, and Go code leans on packages. %d direct "
                "dependencies, read straight from <code>go.mod</code> so the list and "
                "versions can't drift -- each one with a plain-English reason it's "
                "there, grounded in where it's actually imported, not its own pitch."
                % total
            ),
            "Platform",
            h1_id="dependencies-title",
            lead_html=True,
        )
    )
    out.append(
        '                <input id="dep-search" type="search" placeholder="Filter dependencies (e.g. netlink, ssh, prometheus)..." aria-label="Filter dependencies" />'
    )
    for category in data["categories"]:
        out.append(render_group(category, versions))
    out.append("            </section>")

    body = "\n".join(out)
    DEST.parent.mkdir(parents=True, exist_ok=True)
    DEST.write_text(body + "\n" + sitelib.page_foot(root))
    sitelib.write_markdown_sibling(DEST, render_markdown(versions, data, total))
    print(
        "rendered %d dependencies (%d groups) -> %s (+ index.md)"
        % (total, len(data["categories"]), DEST)
    )


def main():
    versions = parse_direct_deps()
    data = load_data()
    # Always render a valid page first; check_drift then reports any go.mod /
    # data/dependencies.json mismatch via sitelib.warn, which build.py turns
    # into a build failure at the very end. The site is still generated, the
    # build just goes red until the curator acts on the warning.
    render(versions, data)
    check_drift(versions, data)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
