#!/usr/bin/env python3
"""Generate the architecture lists in ai/INSTRUCTIONS.md from the tree.

The component and plugin inventories drift whenever directories are added,
moved, or removed; hand-maintained lists in the auto-loaded instructions went
stale (CLAUDE.md named components that no longer existed). This script owns
the volatile lists; the surrounding prose stays hand-written and its path
references are validated by scripts/dev/check_doc_links.py.

Replaces the text between marker pairs:
    <!-- BEGIN GENERATED: arch-<name> ... -->
    <!-- END GENERATED: arch-<name> -->
for <name> in: components, system-plugins, bgp-plugins.

Usage: scripts/dev/arch_map.py [--check]
Called by: make ze-arch-map-update, make ze-generated-files-update / ze-generated-files-update-check
Exit codes: 0 = ok, 1 = --check found drift or a marker is missing.
"""

import os
import subprocess
import sys
import textwrap

INSTRUCTIONS = "ai/INSTRUCTIONS.md"

SOURCES = {
    "components": "internal/component",
    "system-plugins": "internal/plugins",
    "bgp-plugins": "internal/component/bgp/plugins",
}


def dirs(path: str) -> list[str]:
    return sorted(d for d in os.listdir(path) if os.path.isdir(os.path.join(path, d)))


def block(name: str, path: str) -> str:
    names = dirs(path)
    body = textwrap.fill(
        ", ".join(names),
        width=78,
        break_long_words=False,
        break_on_hyphens=False,
    )
    return f"{len(names)} directories under `{path}/`:\n\n{body}\n"


def render(content: str) -> str:
    for name, path in SOURCES.items():
        begin = f"<!-- BEGIN GENERATED: arch-{name}"
        end = f"<!-- END GENERATED: arch-{name} -->"
        start = content.find(begin)
        stop = content.find(end)
        if start == -1 or stop == -1:
            print(
                f"{INSTRUCTIONS}: marker pair for arch-{name} not found",
                file=sys.stderr,
            )
            sys.exit(1)
        start = content.index("-->", start) + len("-->")
        content = content[:start] + "\n" + block(name, path) + content[stop:]
    return content


def main() -> int:
    check = "--check" in sys.argv[1:]
    os.chdir(
        subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            capture_output=True,
            text=True,
            check=True,
        ).stdout.strip()
    )
    with open(INSTRUCTIONS, encoding="utf-8") as fh:
        current = fh.read()
    rendered = render(current)
    if rendered == current:
        print(f"{INSTRUCTIONS}: architecture lists up to date")
        return 0
    if check:
        print(
            f"{INSTRUCTIONS}: architecture lists are stale -- run: make ze-generated-files-update",
            file=sys.stderr,
        )
        return 1
    with open(INSTRUCTIONS, "w", encoding="utf-8") as fh:
        fh.write(rendered)
    print(f"{INSTRUCTIONS}: architecture lists regenerated")
    return 0


if __name__ == "__main__":
    sys.exit(main())
