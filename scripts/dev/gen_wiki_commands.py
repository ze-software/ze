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


# The generator holds NO operator list. It used to carry a hand-typed ten and
# printed it on all 381 command entries, which is how six real operators went
# unpublished and how every `clear` command was told it supported them. Each
# entry now carries what that command supports, derived from the operator
# catalog and the shape the command declared.


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
    shape = entry.get("answer-shape", "")
    if shape:
        lines.append(f"Answer shape: `{shape}`")
    address_fields = entry.get("address-fields", [])
    if address_fields:
        fields = ", ".join(f"`{field}`" for field in address_fields)
        lines.append(f"Address fields: {fields}")

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

    operators = entry.get("operators", [])
    pipes = entry.get("pipes", [])
    aliases = entry.get("pipe-aliases", [])
    if operators or pipes or aliases:
        lines.append("")
        lines.append("**Pipes:**")
        known_availability = {"always", "with-rows", "when-streaming"}
        unknown = [
            o
            for o in operators
            if o.get("available") not in known_availability
        ]
        if unknown:
            raise ValueError(
                "unknown operator availability for "
                + ", ".join(o.get("name", "<unnamed>") for o in unknown)
            )
        always = [
            o["name"]
            for o in operators
            if o.get("available") == "always" and not o.get("local-only")
        ]
        with_rows = [
            o["name"]
            for o in operators
            if o.get("available") == "with-rows" and not o.get("local-only")
        ]
        streaming = [
            o["name"]
            for o in operators
            if o.get("available") == "when-streaming" and not o.get("local-only")
        ]
        local_only = [o["name"] for o in operators if o.get("local-only")]
        if always:
            names = ", ".join(f"`{name}`" for name in always)
            lines.append(f"Always: {names}")
        if with_rows:
            names = ", ".join(f"`{name}`" for name in with_rows)
            lines.append("")
            lines.append(
                f"When the answer has rows: {names} "
                "-- this command has not declared its answer shape, so each of "
                "these applies to an answer that carries rows and is refused by "
                "name over one that does not."
            )
        if streaming:
            names = ", ".join(f"`{name}`" for name in streaming)
            lines.append("")
            lines.append(f"While the command keeps answering: {names}")
        if local_only:
            names = ", ".join(f"`{name}`" for name in local_only)
            lines.append("")
            lines.append(
                f"Local process only: {names} -- daemon-expanded SSH and web "
                "chains refuse these operators."
            )
        if aliases:
            lines.append("")
            lines.append("Named chains:")
            for a in aliases:
                lines.append(f"- `{a['name']}` -- {a['description']} (`{a['expansion']}`)")
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
            or e.get("operators")
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
