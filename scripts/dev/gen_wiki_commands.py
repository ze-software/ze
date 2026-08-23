#!/usr/bin/env python3
"""Generate wiki command-catalog.md from `ze help command --json`.

Usage:
    ze help command --json | python3 scripts/dev/gen_wiki_commands.py > ../wiki/command-catalog.md
"""

import json
import sys
from collections import defaultdict


def first_line(desc):
    """Return the first line of a description."""
    if "\n" in desc:
        return desc[: desc.index("\n")]
    return desc


def escape_pipe(s):
    """Escape pipe characters for markdown tables."""
    return s.replace("|", "\\|")


# GLOBAL_OPERATORS is filled from `ze pipe help --json` before rendering. The
# generator holds no operator list of its own: it used to carry a hand-typed ten
# and printed it on all 381 command entries, which is how six real operators went
# unpublished. It is the argument for deriving this page, made by the page.
GLOBAL_OPERATORS = []


def load_operators(path):
    """Read the operator catalog `ze pipe help --json` writes."""
    with open(path, encoding="utf-8") as fh:
        return json.load(fh)


def render_detail(entry):
    """Render detailed info for a command."""
    lines = []

    desc = entry.get("description", "")
    if "\n" in desc:
        lines.append("")
        for line in desc.split("\n"):
            lines.append(line)

    mode = entry.get("mode", "")
    wire = entry.get("wire-method", "")
    meta = f"Mode: {mode}"
    if wire:
        meta += f" | Wire: `{wire}`"
    lines.append("")
    lines.append(meta)

    backend = entry.get("backend", [])
    if backend:
        lines.append("")
        lines.append(f"**Requires backend:** {', '.join(f'`{b}`' for b in backend)}")

    task = entry.get("task-support", "")
    if task:
        lines.append(f"**Task support:** {task}")

    args = entry.get("args", [])
    if args:
        lines.append("")
        lines.append("**Arguments:**")
        lines.append("")
        lines.append("| Name | Type | Required | Values |")
        lines.append("|------|------|----------|--------|")
        for a in args:
            req = "yes" if a.get("mandatory") else ""
            vals = ", ".join(a.get("values", []))
            lines.append(f"| `{a['name']}` | {a['type']} | {req} | {vals} |")

    global_pipes = entry.get("global-pipes", False)
    pipes = entry.get("pipes", [])
    if global_pipes or pipes:
        lines.append("")
        lines.append("**Pipes:**")
        if global_pipes and GLOBAL_OPERATORS:
            names = ", ".join(f"`{name}`" for name in GLOBAL_OPERATORS)
            lines.append(f"Global: {names}")
        if pipes:
            lines.append("")
            lines.append("Command-specific:")
            for p in pipes:
                arg_hint = " `<value>`" if p.get("takes-arg") else ""
                lines.append(f"- `{p['name']}`{arg_hint} -- {p['description']}")
    elif not entry.get("wire-method"):
        lines.append("")
        lines.append("**Pipes:** not available (offline command)")

    subs = entry.get("subcommands", [])
    if subs:
        lines.append("")
        lines.append(f"**Subcommands:** {', '.join(f'`{s}`' for s in subs)}")

    return lines


def main():
    global GLOBAL_OPERATORS
    ops_path = None
    argv = sys.argv[1:]
    if "--operators" in argv:
        ops_path = argv[argv.index("--operators") + 1]
    if ops_path is None:
        sys.exit(
            "gen_wiki_commands.py: --operators <path> is required.\n"
            "  Write it with: ze pipe help --json > <path>\n"
            "  The generator holds no operator list of its own."
        )
    GLOBAL_OPERATORS = [
        op["name"] for op in load_operators(ops_path) if op.get("class") == "global"
    ]

    data = json.load(sys.stdin)

    groups = defaultdict(list)
    for entry in data:
        parts = entry["path"].split(None, 1)
        verb = parts[0] if parts else entry["path"]
        groups[verb].append(entry)

    print("> **Pre-Alpha.** This page is auto-generated from `ze help command --json`.")
    print()
    print("# Command Catalog")
    print()

    print("## Contents")
    print()
    for verb in sorted(groups):
        count = len(groups[verb])
        print(f"- [{verb}](#{verb}) ({count})")
    print()

    for verb in sorted(groups):
        entries = sorted(groups[verb], key=lambda e: e["path"])
        print(f"## {verb}")
        print()
        print("| Command | Mode | Description |")
        print("|---------|------|-------------|")
        for entry in entries:
            desc = entry.get("description", "")
            short = escape_pipe(first_line(desc))
            print(f"| `{entry['path']}` | {entry['mode']} | {short} |")
        print()

        detailed = [
            e
            for e in entries
            if e.get("args")
            or e.get("pipes")
            or e.get("subcommands")
            or e.get("backend")
            or e.get("task-support")
            or e.get("global-pipes")
            or "\n" in e.get("description", "")
        ]
        if detailed:
            for entry in detailed:
                print(f"### `{entry['path']}`")
                for line in render_detail(entry):
                    print(line)
                print()

    print(f"---\n\n*{len(data)} commands total.*")


if __name__ == "__main__":
    main()
