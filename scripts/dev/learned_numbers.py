#!/usr/bin/env python3
"""Enforce unique, self-consistent numbering across plan/learned/NNN-*.md.

Why this exists: `scripts/dev/commit_helper.py learned-next` allocates via
`number = max(existing prefixes) + 1` against the LOCAL filesystem. It cannot
see a number allocated on another branch, so two branches developed in parallel
both allocate the same number and the duplicate only appears when they merge or
rebase. Nothing detected that: the number silently stops identifying a summary,
and `ai/LEARNED-FULL-INDEX.md` renders two rows with the same `#`.

Invariants enforced:
  1. no two summaries share a number
  2. a summary's H1 number, when it carries one, matches its filename number

`--fix` resolves duplicates by keeping, in each colliding group, the summary
with the most references elsewhere in the tree (renumbering it would churn the
most `// Design:` headers and links); ties break by earliest add-commit, then
filename. Everything else is reassigned a fresh number above the current
highest. References, H1 numbers, and numbered markdown links are rewritten.

Usage:
    python3 scripts/dev/learned_numbers.py --check   # exit 1 if numbering is broken
    python3 scripts/dev/learned_numbers.py --fix     # resolve duplicates in place

After --fix, run `make ze-discovery-index` to refresh ai/LEARNED-FULL-INDEX.md.
"""

from __future__ import annotations

import argparse
import collections
import os
import re
import subprocess
import sys
from pathlib import Path

NAME_RE = re.compile(r"^(\d+)-(.+)\.md$")
# "# 477 -- Title", "# 821 — Title", "# 610: bng-2 -- Title"
H1_RE = re.compile(r"^#\s+(\d+)(\s*(?::|--|—|-)\s*)(.*)$")
H1_SCAN_LINES = 6


def summaries(learned_dir: Path) -> dict[int, list[str]]:
    """Map number -> sorted filenames of every numbered summary."""
    out: dict[int, list[str]] = collections.defaultdict(list)
    for entry in sorted(learned_dir.glob("[0-9]*.md")):
        m = NAME_RE.match(entry.name)
        if m:
            out[int(m.group(1))].append(entry.name)
    return dict(out)


def duplicates(items: dict[int, list[str]]) -> dict[int, list[str]]:
    """Numbers claimed by more than one summary."""
    return {n: f for n, f in sorted(items.items()) if len(f) > 1}


def h1_number(text: str) -> int | None:
    """The number in the file's first heading, or None if it carries none."""
    for line in text.split("\n")[:H1_SCAN_LINES]:
        m = H1_RE.match(line)
        if m:
            return int(m.group(1))
        if line.startswith("# "):
            return None
    return None


def retitle(text: str, number: int) -> str:
    """Rewrite the H1's number to `number`, preserving the separator."""
    lines = text.split("\n")
    for i, line in enumerate(lines[:H1_SCAN_LINES]):
        m = H1_RE.match(line)
        if m:
            lines[i] = f"# {number}{m.group(2)}{m.group(3)}"
            break
        if line.startswith("# "):
            break
    return "\n".join(lines)


def check(learned_dir: Path) -> list[str]:
    """Return a list of problems; empty means the numbering is sound."""
    problems: list[str] = []
    items = summaries(learned_dir)
    if not items:
        return [f"no numbered summaries found under {learned_dir}"]

    for num, files in duplicates(items).items():
        problems.append(
            f"number {num} claimed by {len(files)} summaries: {', '.join(files)}"
        )

    for num, files in sorted(items.items()):
        for name in files:
            try:
                text = (learned_dir / name).read_text(
                    encoding="utf-8", errors="replace"
                )
            except OSError:
                continue
            got = h1_number(text)
            if got is not None and got != num:
                problems.append(f"{name}: H1 says {got}, filename says {num}")

    return problems


def _tracked(root: Path) -> list[str]:
    out = subprocess.run(
        ["git", "-C", str(root), "ls-files"], capture_output=True, text=True
    ).stdout
    return out.splitlines()


def _added_at(root: Path, rel: str) -> int:
    out = subprocess.run(
        ["git", "-C", str(root), "log", "--diff-filter=A", "--format=%at", "--", rel],
        capture_output=True,
        text=True,
    ).stdout.split()
    return int(out[-1]) if out else 1 << 62


def rename_plan(
    items: dict[int, list[str]],
    refcount: dict[str, int],
    added_at: dict[str, int],
) -> list[tuple[str, str, int]]:
    """Return [(old_name, new_name, new_number)] resolving every duplicate.

    Pure: callers supply reference counts and add-times so this is testable
    without a git repo.
    """
    dups = duplicates(items)
    if not dups:
        return []
    next_num = max(items) + 1
    plan: list[tuple[str, str, int]] = []
    for num in sorted(dups):
        ranked = sorted(
            dups[num],
            key=lambda n: (-refcount.get(n, 0), added_at.get(n, 1 << 62), n),
        )
        for name in ranked[1:]:  # ranked[0] keeps the number
            slug = NAME_RE.match(name).group(2)
            plan.append((name, f"{next_num}-{slug}.md", next_num))
            next_num += 1
    return plan


def fix(root: Path, learned_dir: Path) -> int:
    items = summaries(learned_dir)
    dups = duplicates(items)
    if not dups:
        print("no duplicate numbers; nothing to do")
        return 0

    dup_names = [n for names in dups.values() for n in names]
    stems = {n: n[:-3] for n in dup_names}

    refcount: dict[str, int] = collections.Counter()
    for rel in _tracked(root):
        # LEARNED-FULL-INDEX is regenerated wholesale and lists every summary,
        # so it says nothing about how established a number is.
        if rel == "ai/LEARNED-FULL-INDEX.md":
            continue
        path = root / rel
        try:
            text = path.read_text(encoding="utf-8", errors="ignore")
        except OSError:
            continue
        for name, stem in stems.items():
            if rel != f"plan/learned/{name}" and stem in text:
                refcount[name] += 1

    added = {n: _added_at(root, f"plan/learned/{n}") for n in dup_names}
    plan = rename_plan(items, refcount, added)

    for old, new, _ in plan:
        os.rename(learned_dir / old, learned_dir / new)

    renames = {stems[old]: new[:-3] for old, new, _ in plan}
    edited = 0
    for rel in _tracked(root) + [f"plan/learned/{new}" for _, new, _ in plan]:
        path = root / rel
        if not path.exists():
            continue
        try:
            text = path.read_text(encoding="utf-8", errors="surrogateescape")
        except (OSError, UnicodeDecodeError):
            continue
        orig = text
        for old_stem, new_stem in renames.items():
            text = text.replace(old_stem, new_stem)
        # a markdown link whose label is the old number: [821](.../1145-slug.md)
        for old, new, num in plan:
            old_num = NAME_RE.match(old).group(1)
            text = re.sub(
                r"\[" + old_num + r"\]\((\S*" + re.escape(new[:-3]) + r"\.md)\)",
                lambda m: f"[{num}]({m.group(1)})",
                text,
            )
        if text != orig:
            path.write_text(text, encoding="utf-8", errors="surrogateescape")
            edited += 1

    # H1s: every summary, not just renamed ones -- the same defect predates the
    # rename in a few files.
    retitled = 0
    for num, names in summaries(learned_dir).items():
        for name in names:
            path = learned_dir / name
            text = path.read_text(encoding="utf-8", errors="surrogateescape")
            got = h1_number(text)
            if got is not None and got != num:
                path.write_text(
                    retitle(text, num), encoding="utf-8", errors="surrogateescape"
                )
                retitled += 1

    for old, new, _ in plan:
        print(f"{old} -> {new}")
    print(
        f"renumbered {len(plan)} summaries, rewrote {edited} files, "
        f"retitled {retitled} H1s"
    )
    print("now run: make ze-discovery-index")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument(
        "--check", action="store_true", help="exit 1 if numbering is broken"
    )
    group.add_argument("--fix", action="store_true", help="resolve duplicates in place")
    parser.add_argument(
        "--repo", help="repository root (defaults to this script's repo)"
    )
    args = parser.parse_args()

    root = (
        Path(args.repo).resolve() if args.repo else Path(__file__).resolve().parents[2]
    )
    learned_dir = root / "plan" / "learned"
    if not learned_dir.is_dir():
        print(f"error: {learned_dir} not found", file=sys.stderr)
        return 1

    if args.fix:
        return fix(root, learned_dir)

    problems = check(learned_dir)
    if problems:
        print(
            f"WARNING: plan/learned numbering is broken ({len(problems)} problem(s)) -- "
            "run: python3 scripts/dev/learned_numbers.py --fix",
            file=sys.stderr,
        )
        for p in problems:
            print(f"  {p}", file=sys.stderr)
        return 1
    print(
        f"checked {len(summaries(learned_dir))} summaries, numbering is unique and consistent"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
