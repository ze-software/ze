#!/usr/bin/env python3
"""Normalise section headings across plan/learned/NNN-*.md.

Why this exists: `plan/learned/METHODOLOGY.md` prescribes five sections, and a
later gate reads `## Files` to resolve the paths a summary cites. A summary with
no `## Files` heading is silently skipped by such a gate, so an absent section
reads as a clean result rather than as the format defect it is. 425 summaries
also still carry `## Objective`, a heading retired in favour of `## Context`.

Two mechanical changes, and nothing else:
  1. a `## Objective` heading becomes `## Context`
  2. a summary with no `## Files` heading gains one

For change 2 the content is the literal `None recorded.`, so the later gate can
tell "this summary records no files" from "this summary has no Files section".
When a summary instead spells the heading a different way (`## Files Changed`,
`## Files Touched`, `## Files Created`), the FIRST such heading is canonicalised
to `## Files` rather than a `None recorded.` section being appended: those
summaries do record files, so appending the sentence would state something
false and would hide real paths from the gate.

Headings this tool never touches: the H1 (`# NNN -- Name`), which
`scripts/dev/learned_numbers.py` checks against the filename, and every invented
heading (`## Patterns`, `## Mistakes`, ...). Rewriting those is a judgement call
about prose, not a mechanical fix.

Running twice changes nothing the second time.

Usage:
    python3 scripts/dev/learned_normalise.py --check   # exit 1 if work remains
    python3 scripts/dev/learned_normalise.py --fix     # rewrite in place
"""

from __future__ import annotations

import argparse
import os
import re
import stat
import sys
import tempfile
from pathlib import Path

# The retired heading, alone on its line. A line that merely contains the word
# ("the ## Objective heading", a bullet about objectives) is not a heading.
OBJECTIVE_RE = re.compile(r"^## Objective[ \t]*$")

# Any spelling of the Files heading: "## Files", "## Files Changed",
# "## Files touched (summary)". The word boundary keeps "## Filesystem" out.
FILES_ANY_RE = re.compile(r"^## Files\b")

CONTEXT = "## Context"
FILES = "## Files"
NO_FILES = "None recorded."


def _lines(text: str) -> list[str]:
    return text.split("\n")


def objective_lines(text: str) -> list[int]:
    """Indices of every retired `## Objective` heading."""
    return [i for i, line in enumerate(_lines(text)) if OBJECTIVE_RE.match(line)]


def has_context(text: str) -> bool:
    return any(line.rstrip() == CONTEXT for line in _lines(text))


def has_files(text: str) -> bool:
    """True when the canonical `## Files` heading is present."""
    return any(line.rstrip() == FILES for line in _lines(text))


def files_variant_line(text: str) -> int | None:
    """Index of the first non-canonical Files heading, or None."""
    for i, line in enumerate(_lines(text)):
        if FILES_ANY_RE.match(line) and line.rstrip() != FILES:
            return i
    return None


def problems(text: str) -> list[str]:
    """What normalising this summary would change; empty means it conforms."""
    out: list[str] = []
    objectives = objective_lines(text)
    if objectives:
        if has_context(text):
            # Fail closed: renaming would leave two `## Context` headings, and a
            # duplicated section is worse than the retired name. Report instead.
            out.append(
                f"line {objectives[0] + 1}: `## Objective` cannot be renamed, "
                "`## Context` already present"
            )
        elif len(objectives) > 1:
            # The same collision, reached from the other side: renaming BOTH
            # headings manufactures the duplicate the branch above exists to
            # prevent. Which one is the real context is a judgement about prose.
            out.append(
                f"line {objectives[0] + 1}: {len(objectives)} `## Objective` "
                f"headings cannot be renamed, `{CONTEXT}` would be duplicated"
            )
        else:
            out.append(f"line {objectives[0] + 1}: `## Objective` -> `{CONTEXT}`")
    if not has_files(text):
        variant = files_variant_line(text)
        if variant is None:
            out.append(f"no `{FILES}` section; would append one with `{NO_FILES}`")
        else:
            name = _lines(text)[variant].rstrip()
            out.append(f"line {variant + 1}: `{name}` -> `{FILES}`")
    return out


def normalise(text: str) -> str:
    """Apply both mechanical changes. Idempotent."""
    lines = _lines(text)

    # Exactly one, and no `## Context` to collide with. Two headings would both
    # be rewritten and leave the duplicate this guard exists to prevent, so the
    # count is part of the condition rather than something `problems()` alone
    # reports (`ai/rules/evidence.md`).
    objectives = objective_lines(text)
    if len(objectives) == 1 and not has_context(text):
        lines[objectives[0]] = CONTEXT

    text = "\n".join(lines)

    if not has_files(text):
        variant = files_variant_line(text)
        if variant is not None:
            lines = _lines(text)
            lines[variant] = FILES
            text = "\n".join(lines)
        else:
            body = text.rstrip("\n")
            head = f"{body}\n\n" if body else ""
            text = f"{head}{FILES}\n\n{NO_FILES}\n"

    return text


def summaries(learned_dir: Path) -> list[Path]:
    return sorted(learned_dir.glob("[0-9]*.md"))


def write_atomic(path: Path, text: str) -> None:
    """Replace `path` in one step, so a concurrent reader never sees a partial file.

    The original mode is carried onto the replacement. `mkstemp` creates at 0600,
    and `os.replace` keeps the SOURCE file's mode, so without this every rewritten
    summary is narrowed from the corpus baseline of 0644 to owner-only. Git tracks
    the exec bit alone, so such a narrowing is invisible in `git diff` and would
    ship unnoticed."""
    mode = stat.S_IMODE(os.stat(path).st_mode)
    fd, tmp = tempfile.mkstemp(dir=str(path.parent), prefix=path.name, suffix=".tmp")
    try:
        with os.fdopen(fd, "w", encoding="utf-8", errors="surrogateescape") as handle:
            handle.write(text)
        os.chmod(tmp, mode)
        os.replace(tmp, path)
    except BaseException:
        if os.path.exists(tmp):
            os.unlink(tmp)
        raise


def run(learned_dir: Path, fix: bool) -> int:
    found = summaries(learned_dir)
    if not found:
        print(f"error: no numbered summaries under {learned_dir}", file=sys.stderr)
        return 1

    changed = 0
    blocked = 0
    for path in found:
        text = path.read_text(encoding="utf-8", errors="surrogateescape")
        issues = problems(text)
        if not issues:
            continue
        new = normalise(text)

        # What survives normalising is exactly what normalise() declined to do.
        # Derived rather than re-classified here, so the two can never disagree
        # about which issue is actionable. It is reported whether or not the
        # file also changed: a summary carrying a `## Context` collision AND a
        # missing `## Files` used to have the collision swallowed by the
        # successful half, write, and exit 0.
        remaining = problems(new)
        if remaining:
            blocked += 1
            for issue in remaining:
                print(f"{path.name}: {issue}", file=sys.stderr)

        if new == text:
            continue
        changed += 1
        if fix:
            write_atomic(path, new)
        else:
            for issue in issues:
                if issue not in remaining:
                    print(f"{path.name}: {issue}")

    verb = "normalised" if fix else "would normalise"
    print(f"{verb} {changed} of {len(found)} summaries")
    if blocked:
        print(
            f"{blocked} summaries need a human decision (listed above)", file=sys.stderr
        )
    if fix:
        return 1 if blocked else 0
    return 1 if (changed or blocked) else 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument(
        "--check", action="store_true", help="exit 1 if any summary needs normalising"
    )
    group.add_argument("--fix", action="store_true", help="rewrite summaries in place")
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

    return run(learned_dir, fix=args.fix)


if __name__ == "__main__":
    sys.exit(main())
