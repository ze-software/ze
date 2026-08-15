#!/usr/bin/env -S uv run python3
"""Render a live CLI command reference from `ze help command --json`.

Usage:
    tools/render-cli-catalog.py

Runs ../main/bin/ze help command --json -- the exact JSON the project's own
wiki command-catalog is generated from (see cmd/ze/help_command.go and
docs/guide/command-reference.md) -- caches it to data/cli-commands.json, and
renders reference/cli/index.html grouped by command verb with a client-side filter.

The catalog is generated from the live binary's own command registry
(YANG dispatch tree + offline local commands), so it cannot go stale the
way a hand-maintained command table can. Run `make ze` in ../main, then
re-run this, to pick up new or changed commands.
"""

import html
import json
import os
import pathlib
import re
import subprocess
import sys

import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
ZE_BINARY = GH_PAGES.parent / "main" / "bin" / "ze"
DATA = GH_PAGES / "data" / "cli-commands.json"
DEST = GH_PAGES / "reference" / "cli" / "index.html"

MODE_LABELS = {
    "daemon": "Daemon",
    "read-only": "Read-only",
    "offline": "Offline",
}

MAX_GROUP_SIZE = 20

SLUG_RE = re.compile(r"[^a-z0-9]+")


def slugify(prefix, text):
    return prefix + SLUG_RE.sub("-", text.lower()).strip("-")

def usage_syntax(description):
    marker = "Usage:"
    idx = description.find(marker)
    if idx == -1:
        return ""
    usage = description[idx + len(marker):].strip().splitlines()[0].strip()
    if "." in usage:
        usage = usage.split(".", 1)[0].strip()
    return usage.rstrip(".").strip()


def normalize_commands(commands):
    for command in commands:
        syntax = usage_syntax(command.get("description", ""))
        if syntax:
            command["syntax"] = syntax
    return commands


def ensure_production_binary():
    result = subprocess.run(
        ["go", "version", "-m", str(ZE_BINARY)], capture_output=True, text=True
    )
    if result.returncode != 0:
        print(
            "error: unable to inspect %s build tags: %s" % (ZE_BINARY, result.stderr),
            file=sys.stderr,
        )
        sys.exit(1)
    for line in result.stdout.splitlines():
        line = line.strip()
        if not line.startswith("build\t-tags="):
            continue
        tags = line.removeprefix("build\t-tags=").split(",")
        if "zetest" in tags:
            print(
                (
                    "error: %s was built with zetest; run `make bin/ze` in ../main "
                    "before generating public command docs"
                )
                % ZE_BINARY,
                file=sys.stderr,
            )
            sys.exit(1)
        return


def fetch_commands():
    if not ZE_BINARY.exists():
        print(
            "error: %s not found -- run `make bin/ze` in ../main first" % ZE_BINARY,
            file=sys.stderr,
        )
        sys.exit(1)
    ensure_production_binary()
    result = subprocess.run(
        [str(ZE_BINARY), "help", "command", "--json"], capture_output=True, text=True
    )
    if result.returncode != 0:
        print(
            "error: %s help command --json failed: %s" % (ZE_BINARY, result.stderr),
            file=sys.stderr,
        )
        sys.exit(1)
    commands = normalize_commands(json.loads(result.stdout))
    DATA.write_text(json.dumps(commands, indent=2, ensure_ascii=False) + "\n")
    return commands


def load_commands():
    """Load live commands, with an explicit cache escape hatch for review builds."""
    if os.environ.get("ZE_CLI_CATALOG_USE_CACHE") == "1" and DATA.exists():
        commands = normalize_commands(json.loads(DATA.read_text()))
        DATA.write_text(json.dumps(commands, indent=2, ensure_ascii=False) + "\n")
        return commands
    if ZE_BINARY.exists():
        return fetch_commands()
    if DATA.exists():
        print(
            "warning: %s not found, using cached %s" % (ZE_BINARY, DATA),
            file=sys.stderr,
        )
        commands = normalize_commands(json.loads(DATA.read_text()))
        DATA.write_text(json.dumps(commands, indent=2, ensure_ascii=False) + "\n")
        return commands
    print(
        "error: neither %s nor a cached %s exist -- run `make ze` in ../main first"
        % (ZE_BINARY, DATA),
        file=sys.stderr,
    )
    sys.exit(1)


MIN_SUBGROUP_SIZE = 4


def group_commands(commands):
    """Group by top-level verb; split verbs with more than MAX_GROUP_SIZE
    entries (e.g. "show" alone has 200+) into verb+subject subgroups so no
    single table is unwieldy. A verb+subject pair below MIN_SUBGROUP_SIZE
    doesn't get its own group -- "show" has 67 distinct subjects and most
    of them are a single command, so splitting naively produces dozens of
    one-row groups (e.g. "show arp", "show cache", ...). Those fold into a
    single "<verb> (other)" catch-all instead, so the group list stays
    scannable: frequent subjects (show bgp, show ospf, ...) keep their own
    group, the long tail shares one."""
    by_verb = {}
    for c in commands:
        verb = c["path"].split(" ")[0]
        by_verb.setdefault(verb, []).append(c)

    groups = []
    for verb in sorted(by_verb):
        entries = by_verb[verb]
        if len(entries) <= MAX_GROUP_SIZE:
            groups.append((verb, sorted(entries, key=lambda c: c["path"])))
            continue
        by_subject = {}
        for c in entries:
            parts = c["path"].split(" ")
            subject = parts[1] if len(parts) > 1 else ""
            by_subject.setdefault(subject, []).append(c)
        other = []
        for subject in sorted(by_subject):
            bucket = by_subject[subject]
            if len(bucket) < MIN_SUBGROUP_SIZE:
                other.extend(bucket)
                continue
            label = "%s %s" % (verb, subject) if subject else verb
            groups.append((label, sorted(bucket, key=lambda c: c["path"])))
        if other:
            groups.append(
                ("%s (other)" % verb, sorted(other, key=lambda c: c["path"]))
            )
    return groups


def render_row(c):
    desc_html = html.escape(c["description"]).replace("\n", "<br>")
    mode = c.get("mode", "")
    mode_label = MODE_LABELS.get(mode, mode)
    return (
        '<tr id="%s"><td><code>%s</code></td>'
        '<td><span class="cli-mode cli-mode-%s">%s</span></td>'
        "<td>%s</td></tr>"
    ) % (slugify("cmd-", c["path"]), html.escape(c.get("syntax") or c["path"]), mode, mode_label, desc_html)


def render_group(label, entries):
    parts = [
        '<details class="cli-group" id="%s" open>' % slugify("cli-group-", label),
        '<summary>%s <span class="cli-group-count">%d</span></summary>'
        % (html.escape(label), len(entries)),
        "<table><thead><tr><th>Command</th><th>Mode</th><th>Description</th></tr></thead><tbody>",
    ]
    parts.extend(render_row(c) for c in entries)
    parts.append("</tbody></table></details>")
    return "\n".join(parts)


def render_markdown(commands, groups):
    """Full grouped listing, same data and same grouping as render() --
    group_commands() runs once in main() and both renderers consume its
    result, so the human page and this Markdown mirror can never disagree
    about how commands are organized."""
    parts = [
        "# CLI Reference",
        "",
        "%d commands across %d groups, generated straight from `ze help "
        "command --json` -- the same live command registry the binary "
        "itself uses, so this list cannot drift from what the binary "
        "actually supports. Full machine-readable list (path, mode, "
        "description for every command, one JSON array): "
        "[data/cli-commands.json](%sdata/cli-commands.json)."
        % (len(commands), len(groups), sitelib.SITE_BASE),
        "",
    ]
    for label, entries in groups:
        parts.append("## %s (%d)" % (label, len(entries)))
        parts.append("")
        parts.append("| Command | Mode | Description |")
        parts.append("| --- | --- | --- |")
        for c in entries:
            mode_label = MODE_LABELS.get(c.get("mode", ""), c.get("mode", ""))
            path = c["path"].replace("|", "\\|")
            desc = " ".join(c["description"].split()).replace("|", "\\|")
            parts.append("| `%s` | %s | %s |" % (path, mode_label, desc))
        parts.append("")
    return "\n".join(parts).strip() + "\n"


def render(commands, groups):
    root = "../../"
    title = "CLI Reference - Ze"
    desc = (
        "Every ze command, generated live from the binary's own command "
        "registry -- %d commands across %d groups." % (len(commands), len(groups))
    )
    out = [
        sitelib.page_head(
            title, desc, root, og_title=title, og_desc=desc, page_key="reference/cli/"
        )
    ]
    out.append(
        '            <section aria-labelledby="cli-title" class="md-content reveal cat-operate">'
    )
    out.append(
        sitelib.page_hero(
            "CLI Reference",
            (
                "%d commands across %d groups, generated straight from "
                "<code>ze help command --json</code> -- the same live command registry the "
                "binary itself uses, so this page cannot drift from what the binary actually "
                "supports the way a hand-maintained list can. Full machine-readable list: "
                '<a href="../data/cli-commands.json">data/cli-commands.json</a>.'
                % (len(commands), len(groups))
            ),
            "Reference",
            h1_id="cli-title",
            lead_html=True,
        )
    )
    out.append('                <div class="cli-search-wrap">')
    out.append(
        '                    <input id="cli-search" type="search" autocomplete="off" '
        'placeholder="Filter commands (e.g. bgp, traceroute, monitor)..." '
        'aria-label="Filter commands" />'
    )
    out.append(
        '                    <div id="cli-suggestions" class="cli-suggestions" hidden></div>'
    )
    out.append("                </div>")
    for label, entries in groups:
        out.append(render_group(label, entries))
    out.append("            </section>")

    body = "\n".join(out)
    DEST.parent.mkdir(parents=True, exist_ok=True)
    DEST.write_text(body + "\n" + sitelib.page_foot(root))
    sitelib.write_markdown_sibling(DEST, render_markdown(commands, groups))
    print(
        "rendered %d commands (%d groups) -> %s (+ index.md)"
        % (len(commands), len(groups), DEST)
    )


def main():
    commands = load_commands()
    groups = group_commands(commands)
    render(commands, groups)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
