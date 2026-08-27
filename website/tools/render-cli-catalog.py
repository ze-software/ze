#!/usr/bin/env -S uv run python3
"""Render a live CLI command reference from `ze help command --json`.

Usage:
    tools/render-cli-catalog.py

Runs the current build session's `ze help command --json`. This is the exact JSON
that the project's command catalog uses. The command registry combines the YANG
dispatch tree and offline local commands, so the generated page cannot silently
drift from the binary.

Run `make ze-build` in ../main before this tool to update changed commands.
"""

import html
import json
import os
import pathlib
import re
import subprocess
import sys

import sitelib
import sitepaths
import zebinary

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
ZE_BINARY = zebinary.resolve(sitepaths.MAIN_REPO)
DATA = GH_PAGES / "data" / "cli-commands.json"
DEST = GH_PAGES / "reference" / "cli" / "index.html"

MODE_LABELS = {
    "daemon": "Daemon",
    "read-only": "Read-only",
    "offline": "Offline",
}

AVAILABILITY_LABELS = {
    "always": "Always",
    "with-rows": "With rows",
    "when-streaming": "While streaming",
    "local-only": "Local process only",
}

AVAILABILITY_ORDER = {
    "always": 0,
    "with-rows": 1,
    "when-streaming": 2,
    "local-only": 3,
}

PIPE_CLASS_LABELS = {
    "global": "Output and control",
    "stream": "Streaming",
    "data": "Row data",
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
    usage = description[idx + len(marker) :].strip().splitlines()[0].strip()
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
                    "error: %s was built with zetest; run `make ze-build` in ../main "
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
            "error: %s not found -- run `make ze-build` in ../main first" % ZE_BINARY,
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
        "error: neither %s nor a cached %s exist -- run `make ze-build` in ../main first"
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
            groups.append(("%s (other)" % verb, sorted(other, key=lambda c: c["path"])))
    return groups

def pipe_items(command, key):
    return command.get(key) or []


def pipe_name(item):
    name = item.get("name", "")
    if item.get("takes-arg"):
        name += " <value>"
    return name


def pipe_summary(command):
    parts = []
    pipes = pipe_items(command, "pipes")
    aliases = pipe_items(command, "pipe-aliases")
    operators = pipe_items(command, "operators")
    answer_shape = command.get("answer-shape")
    address_fields = pipe_items(command, "address-fields")
    if pipes:
        parts.append("%d command pipe%s" % (len(pipes), "" if len(pipes) == 1 else "s"))
    if aliases:
        parts.append("%d alias%s" % (len(aliases), "" if len(aliases) == 1 else "es"))
    if operators:
        parts.append(
            "%d operator%s" % (len(operators), "" if len(operators) == 1 else "s")
        )
    if answer_shape:
        parts.append("answer: %s" % answer_shape)
    if address_fields:
        parts.append(
            "%d address field%s"
            % (len(address_fields), "" if len(address_fields) == 1 else "s")
        )
    return " · ".join(parts) or "None"


def operators_by_availability(command):
    grouped = {}
    for operator in pipe_items(command, "operators"):
        availability = operator.get("available")
        if availability not in AVAILABILITY_LABELS:
            raise ValueError(
                "unknown operator availability for "
                + operator.get("name", "<unnamed>")
            )
        grouped.setdefault(availability, []).append(operator.get("name", ""))
        if operator.get("local-only"):
            grouped.setdefault("local-only", []).append(operator.get("name", ""))
    return {
        availability: grouped[availability]
        for availability in sorted(
            grouped, key=lambda value: AVAILABILITY_ORDER.get(value, 99)
        )
    }


def render_pipe_details(command):
    pipes = pipe_items(command, "pipes")
    aliases = pipe_items(command, "pipe-aliases")
    operators = pipe_items(command, "operators")
    answer_shape = command.get("answer-shape")
    address_fields = pipe_items(command, "address-fields")
    if not pipes and not aliases and not operators and not answer_shape and not address_fields:
        return '<span class="cli-pipe-none">None</span>'

    parts = [
        '<details class="cli-pipes">',
        "<summary>%s</summary>" % html.escape(pipe_summary(command)),
        '<div class="cli-pipe-detail">',
    ]
    if answer_shape:
        parts.append(
            '<p><span>Answer shape</span><code>%s</code></p>'
            % html.escape(answer_shape)
        )
    if address_fields:
        parts.append(
            '<p><span>Address fields</span><code>%s</code></p>'
            % html.escape(" · ".join(address_fields))
        )
    if pipes:
        parts.append('<strong>Command pipes</strong><div class="cli-pipe-chips">')
        for pipe in pipes:
            parts.append(
                '<code title="%s">%s</code>'
                % (
                    html.escape(pipe.get("description", ""), quote=True),
                    html.escape(pipe_name(pipe)),
                )
            )
        parts.append("</div>")
        parts.append(
            '<details class="cli-pipe-descriptions"><summary>Command pipe descriptions</summary><dl>'
        )
        for pipe in pipes:
            parts.append(
                "<dt><code>%s</code></dt><dd>%s</dd>"
                % (
                    html.escape(pipe_name(pipe)),
                    html.escape(pipe.get("description", "")),
                )
            )
        parts.append("</dl></details>")
    if aliases:
        parts.append("<strong>Aliases</strong><dl>")
        for alias in aliases:
            parts.append(
                "<dt><code>%s</code></dt><dd>%s <code>%s</code></dd>"
                % (
                    html.escape(alias.get("name", "")),
                    html.escape(alias.get("description", "")),
                    html.escape(alias.get("expansion", "")),
                )
            )
        parts.append("</dl>")
    for availability, names in operators_by_availability(command).items():
        parts.append(
            '<p><span>%s</span><code>%s</code></p>'
            % (
                html.escape(AVAILABILITY_LABELS.get(availability, availability)),
                html.escape(" · ".join(names)),
            )
        )
    parts.extend(["</div>", "</details>"])
    return "".join(parts)


def operator_catalog(commands):
    catalog = {}
    for command in commands:
        for operator in pipe_items(command, "operators"):
            name = operator.get("name", "")
            entry = catalog.setdefault(
                name,
                {
                    "name": name,
                    "class": operator.get("class", ""),
                    "description": operator.get("description", ""),
                    "available": [],
                },
            )
            available = operator.get("available")
            if available not in AVAILABILITY_LABELS:
                raise ValueError(
                    "unknown operator availability for "
                    + operator.get("name", "<unnamed>")
                )
            for value in (available, "local-only" if operator.get("local-only") else None):
                if value and value not in entry["available"]:
                    entry["available"].append(value)
    return list(catalog.values())


def render_pipe_guide(commands):
    operators = operator_catalog(commands)
    if not operators:
        return ""
    parts = [
        '<section class="cli-pipe-guide" aria-labelledby="cli-pipe-guide-title">',
        '<div class="cli-pipe-guide-head">',
        '<span class="tag">Pipes</span>',
        '<div><h2 id="cli-pipe-guide-title">Pipe operators</h2>',
        "<p>Each command row names the operators it accepts after <code>|</code>. "
        "Availability comes from the live command registry: operators may require row "
        "data, a streaming answer, or expansion by the operator's local process.</p></div>",
        "</div>",
        "<details>",
        "<summary>Operator reference <span>%d</span></summary>" % len(operators),
        "<table><thead><tr><th>Operator</th><th>Class</th><th>Available</th>"
        "<th>Description</th></tr></thead><tbody>",
    ]
    for operator in operators:
        available = sorted(
            operator["available"], key=lambda value: AVAILABILITY_ORDER.get(value, 99)
        )
        availability = ", ".join(
            AVAILABILITY_LABELS.get(value, value) for value in available
        )
        parts.append(
            "<tr><td><code>%s</code></td><td>%s</td><td>%s</td><td>%s</td></tr>"
            % (
                html.escape(operator["name"]),
                html.escape(
                    PIPE_CLASS_LABELS.get(operator["class"], operator["class"])
                ),
                html.escape(availability),
                html.escape(operator["description"]),
            )
        )
    parts.extend(["</tbody></table>", "</details>", "</section>"])
    return "\n".join(parts)


def render_row(c):
    desc_html = html.escape(c["description"]).replace("\n", "<br>")
    mode = c.get("mode", "")
    mode_label = MODE_LABELS.get(mode, mode)
    return (
        '<tr id="%s"><td><code>%s</code></td>'
        '<td><span class="cli-mode cli-mode-%s">%s</span></td>'
        "<td>%s</td><td>%s</td></tr>"
    ) % (
        slugify("cmd-", c["path"]),
        html.escape(c.get("syntax") or c["path"]),
        mode,
        mode_label,
        desc_html,
        render_pipe_details(c),
    )


def render_group(label, entries):
    parts = [
        '<details class="cli-group" id="%s" open>' % slugify("cli-group-", label),
        '<summary>%s <span class="cli-group-count">%d</span></summary>'
        % (html.escape(label), len(entries)),
        "<table><thead><tr><th>Command</th><th>Mode</th><th>Description</th><th>Pipes</th></tr></thead><tbody>",
    ]
    parts.extend(render_row(c) for c in entries)
    parts.append("</tbody></table></details>")
    return "\n".join(parts)

def markdown_code_list(values):
    return ", ".join("`%s`" % value.replace("|", "\\|") for value in values)


def markdown_pipe_details(command):
    parts = []
    answer_shape = command.get("answer-shape")
    address_fields = pipe_items(command, "address-fields")
    if answer_shape:
        parts.append("Answer shape: %s" % markdown_code_list([answer_shape]))
    if address_fields:
        parts.append("Address fields: %s" % markdown_code_list(address_fields))
    pipes = pipe_items(command, "pipes")
    aliases = pipe_items(command, "pipe-aliases")
    if pipes:
        parts.append("Command: %s" % markdown_code_list([pipe_name(p) for p in pipes]))
    if aliases:
        parts.append(
            "Aliases: %s"
            % markdown_code_list(
                [
                    "%s -> %s"
                    % (alias.get("name", ""), alias.get("expansion", ""))
                    for alias in aliases
                ]
            )
        )
    for availability, names in operators_by_availability(command).items():
        parts.append(
            "%s: %s"
            % (
                AVAILABILITY_LABELS.get(availability, availability),
                markdown_code_list(names),
            )
        )
    return "<br>".join(parts) or "None"


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
        "actually supports. Full machine-readable list (path, mode, description, "
        "pipe operators, command pipes, and aliases for every command): "
        "[data/cli-commands.json](%sdata/cli-commands.json)."
        % (len(commands), len(groups), sitelib.SITE_BASE),
        "",
    ]
    for label, entries in groups:
        parts.append("## %s (%d)" % (label, len(entries)))
        parts.append("")
        parts.append("| Command | Mode | Description | Pipes |")
        parts.append("| --- | --- | --- | --- |")
        for c in entries:
            mode_label = MODE_LABELS.get(c.get("mode", ""), c.get("mode", ""))
            path = c["path"].replace("|", "\\|")
            desc = " ".join(c["description"].split()).replace("|", "\\|")
            parts.append(
                "| `%s` | %s | %s | %s |"
                % (path, mode_label, desc, markdown_pipe_details(c))
            )
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
                "binary itself uses, including the pipe operators available to each command. "
                "Full machine-readable list: "
                '<a href="../../data/cli-commands.json">data/cli-commands.json</a>.'
                % (len(commands), len(groups))
            ),
            "Reference",
            h1_id="cli-title",
            lead_html=True,
        )
    )
    out.append(render_pipe_guide(commands))
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
