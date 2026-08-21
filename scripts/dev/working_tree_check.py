#!/usr/bin/env python3
"""Report how wide the uncommitted working tree is, grouped by area.

Ze commits one logical change at a time (ai/rules/git-safety.md, "Commit
Granularity"). The failure this reports is not a big diff: it is SEVERAL
FINISHED chunks held in one tree, where a checkout destroys them and every
later chunk has to be diffed around them.

Advisory by default: it prints and exits 0, because only a person can say
whether two areas are one logical change. `--max-areas N` makes it exit 1 past
a ceiling, for a caller that wants a gate.
"""

import argparse
import collections
import subprocess
import sys

# Each entry maps a path prefix to the area a reader thinks in. First match
# wins, so the specific prefixes come before the general ones.
AREAS = [
    ("ai/rules/", "rules"),
    ("ai/", "ai-docs"),
    ("plan/journal/", "journal"),
    ("plan/audits/", "audits"),
    ("plan/", "specs"),
    ("docs/", "docs"),
    ("test/", "tests"),
    ("scripts/evidence/", "evidence-tools"),
    ("scripts/", "tooling"),
    ("mk/", "build"),
    ("Makefile", "build"),
    (".golangci.yml", "build"),
    ("pkg/plugin/", "plugin-sdk"),
    ("internal/component/bgp/", "bgp"),
    ("internal/component/plugin/", "plugin-engine"),
    ("internal/component/command/", "cli-command"),
    ("internal/", "internal"),
    ("cmd/", "cmd"),
]


def area_of(path: str) -> str:
    for prefix, name in AREAS:
        if path.startswith(prefix):
            return name
    return "other"


def changed_paths() -> list[str]:
    """Tracked modifications plus untracked files, ignoring what git ignores."""
    out = subprocess.run(
        ["git", "status", "--porcelain", "--untracked-files=all"],
        capture_output=True,
        text=True,
        check=True,
    ).stdout
    paths = []
    for line in out.splitlines():
        if not line.strip():
            continue
        # Porcelain v1: two status characters, a space, then the path. A rename
        # carries "old -> new"; the new name is what a commit would name.
        path = line[3:]
        if " -> " in path:
            path = path.split(" -> ", 1)[1]
        paths.append(path.strip().strip('"'))
    return paths


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--max-areas",
        type=int,
        default=0,
        help="exit 1 when more than N areas are in flight (0 = advisory)",
    )
    args = parser.parse_args()

    paths = changed_paths()
    if not paths:
        print("working tree: clean")
        return 0

    by_area = collections.defaultdict(list)
    for path in paths:
        by_area[area_of(path)].append(path)

    print(f"working tree: {len(paths)} path(s) across {len(by_area)} area(s)")
    for area in sorted(by_area, key=lambda a: (-len(by_area[a]), a)):
        files = sorted(by_area[area])
        shown = ", ".join(files[:4])
        more = f", +{len(files) - 4} more" if len(files) > 4 else ""
        print(f"  {area:<16} {len(files):>3}  {shown}{more}")

    if len(by_area) > 1:
        print()
        print("More than one area is in flight. Land the chunks that are already")
        print("finished before starting the next piece (ai/rules/git-safety.md,")
        print('"Commit Granularity"). Areas are a hint, not a verdict: a feature')
        print("and its tests are one change even when they sit in two of them.")

    if args.max_areas and len(by_area) > args.max_areas:
        print(f"\nworking-tree-check: {len(by_area)} areas exceeds --max-areas {args.max_areas}")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
