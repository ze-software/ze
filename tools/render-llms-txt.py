#!/usr/bin/env -S uv run python3
"""Render llms.txt at the site root from data/nav.json + live counts.

Usage:
    tools/render-llms-txt.py

llms.txt is the emerging convention for giving LLMs/crawlers a concise,
structured map of a site instead of making them parse full HTML. Every
entry's URL, title, and description come from data/nav.json -- the exact
same source that drives the mega-menu on every page -- so llms.txt can't
list a page the nav doesn't, or vice versa. The three descriptions that
carry a live count (Features, CLI Reference, Dependencies) are recomputed
here from data/features.json, data/cli-commands.json, and
data/dependencies.json rather than trusted from nav.json's own (hand-edited,
drift-checked-but-not-auto-fixed) desc text, so a stale nav.json copy can't
make llms.txt wrong too. Wired into tools/build.py's default step list, so
a normal site build always regenerates this file.
"""

import json
import pathlib

import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
DATA_DIR = GH_PAGES / "data"
DEST = GH_PAGES / "llms.txt"

SITE_BASE = "https://ze-software.github.io/ze/"


def live_feature_count():
    features = json.loads((DATA_DIR / "features.json").read_text())
    core = next(s for s in features["sections"] if s["id"] == "core")
    experimental = next(s for s in features["sections"] if s["id"] == "experimental")
    return len(core["cards"]) + len(experimental["cards"])


def live_cli_count():
    path = DATA_DIR / "cli-commands.json"
    if not path.exists():
        return None
    return len(json.loads(path.read_text()))


def live_deps_count():
    path = DATA_DIR / "dependencies.json"
    if not path.exists():
        return None
    data = json.loads(path.read_text())
    return sum(len(cat["modules"]) for cat in data["categories"])


def live_blog_count():
    return len(list((GH_PAGES / "blog" / "posts").glob("*.md")))


LIVE_DESC_OVERRIDES = {
    "features/": lambda: "%d features, color-coded by category" % live_feature_count(),
    "cli/": lambda: (
        "%d commands, generated from the live binary" % live_cli_count()
        if live_cli_count() is not None
        else None
    ),
    "dependencies/": lambda: (
        "%d direct packages, generated from go.mod" % live_deps_count()
        if live_deps_count() is not None
        else None
    ),
}


def nav_entry_line(entry):
    href = entry["href"]
    desc = entry["desc"]
    override = LIVE_DESC_OVERRIDES.get(href)
    if override:
        fresh = override()
        if fresh:
            desc = fresh
    return "- [%s](%s%s): %s" % (entry["title"], SITE_BASE, href, desc)


def render_dropdown_section(dropdown):
    lines = ["## %s" % dropdown["label"], ""]
    for column in dropdown["columns"]:
        for entry in column:
            if "label_only" in entry:
                lines.append("### %s" % entry["label_only"])
                continue
            lines.append(nav_entry_line(entry))
    lines.append("")
    return "\n".join(lines)


def render(nav):
    parts = []
    parts.append("# Ze")
    parts.append("")
    parts.append(
        "> Open, programmable network OS for Linux. Ze speaks BGP, IS-IS, "
        "and OSPF, manages interfaces, programs the FIB, and gives "
        "operators a CLI, web UI, telemetry, looking glass, API, and "
        "plugin system around one coherent configuration model."
    )
    parts.append("")
    parts.append(
        "Built for people who want a network stack they can inspect, "
        "automate, and extend. ExaBGP users get a migration path to a "
        "more performant codebase. Pre-release: no tagged versions yet, "
        "built continuously from the main branch. AGPLv3 open source."
    )
    parts.append("")

    for dropdown in nav["dropdowns"]:
        parts.append(render_dropdown_section(dropdown))

    more_lines = ["## More", ""]
    for link in nav["trailing_links"]:
        if link["href"] == "blog/":
            more_lines.append(
                "- [Blog](%s%s): %d weekly updates, mined from git history"
                % (SITE_BASE, link["href"], live_blog_count())
            )
        else:
            more_lines.append("- [%s](%s%s)" % (link["label"], SITE_BASE, link["href"]))
    more_lines.append("- [Discord](%s): community and support" % sitelib.DISCORD_INVITE)
    more_lines.append(
        "- [GitHub](https://github.com/%s): mirror, issues" % sitelib.GITHUB_REPO
    )
    more_lines.append("- [Codeberg](%s): canonical repository" % sitelib.CODEBERG_REPO)
    parts.append("\n".join(more_lines))
    parts.append("")

    text = "\n".join(parts)
    DEST.write_text(text)
    print("rendered llms.txt -> %s" % DEST)


def main():
    nav = json.loads((DATA_DIR / "nav.json").read_text())
    render(nav)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
