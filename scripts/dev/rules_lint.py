#!/usr/bin/env python3
"""Validate that every ai/rules/*.md rule follows the canonical format.

The format (see ai/rules/rule-format.md) is:

    # Title
    **When:** <trigger>
    **Severity:** blocking|advisory
    **Related:** slug, slug          (optional)
    ...body...

Tooling (rules_index.py, rules_condensed.py, and the eager @-import of
CONDENSED.md) relies on this block being present and machine-readable. This
linter is the durable gate that keeps it true; it runs inside `make ze-doc-test`.

Usage:
    python3 scripts/dev/rules_lint.py           # report violations, exit 1 if any
    python3 scripts/dev/rules_lint.py --quiet    # exit code only
"""

import re
import sys
from pathlib import Path

SKIP = {"INDEX.md", "CONDENSED.md"}
SEVERITIES = {"blocking", "advisory"}
# Exact spelling the consumers require: rules_condensed.py:META_LINE and
# rules_index.py match `**When:**` / `**Severity:**` / `**Related:**`
# case-sensitively, so the lint must too -- a lowercase key that "passes" here
# would leak into CONDENSED/INDEX bodies unparsed. Keep this in sync with them.
CANON_KEYS = ("When", "Severity", "Related")

META_LINE = re.compile(r"^\*\*(?P<key>[A-Za-z]+):\*\*\s*(?P<val>.*)$")
H1 = re.compile(r"^#\s+(\S.*)$")
RELATED_SLUG = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")


def check_rule(path):
    """Return a list of human-readable violation strings for one rule file."""
    problems = []
    lines = path.read_text(encoding="utf-8", errors="replace").splitlines()

    # Find the title: first non-blank line, must be an H1.
    idx = 0
    while idx < len(lines) and not lines[idx].strip():
        idx += 1
    if idx >= len(lines) or not H1.match(lines[idx].strip()):
        problems.append("first non-blank line must be a single '# Title'")
        return problems
    idx += 1

    # Allow one blank line between title and the metadata block.
    while idx < len(lines) and not lines[idx].strip():
        idx += 1

    # Collect the contiguous metadata block (**Key:** value lines). Keys are
    # case-sensitive: a mis-cased key (e.g. `**when:**`) is a violation, not a
    # silently-accepted alias, because the consumers only parse the exact case.
    meta = {}
    order = []
    while idx < len(lines) and lines[idx].strip():
        m = META_LINE.match(lines[idx].strip())
        if not m:
            break
        key = m.group("key")
        if key not in CANON_KEYS:
            problems.append(
                f"metadata key '**{key}:**' must be one of "
                f"{'/'.join(CANON_KEYS)} (exact case)"
            )
            break
        meta[key] = m.group("val").strip()
        order.append(key)
        idx += 1

    if "When" not in meta:
        problems.append("missing required '**When:** <trigger>' line")
    elif not meta["When"]:
        problems.append("'**When:**' line is empty")

    if "Severity" not in meta:
        problems.append("missing required '**Severity:** blocking|advisory' line")
    elif meta["Severity"] not in SEVERITIES:
        problems.append(
            f"'**Severity:**' must be one of {sorted(SEVERITIES)}, got "
            f"'{meta['Severity']}'"
        )

    # Order: When before Severity before Related, when present.
    canon = [k for k in CANON_KEYS if k in order]
    present_in_order = [k for k in order if k in CANON_KEYS]
    if present_in_order != canon:
        problems.append(
            "metadata keys must be ordered When, Severity, Related "
            f"(found {', '.join(present_in_order)})"
        )

    if "Related" in meta and meta["Related"]:
        for slug in [s.strip() for s in meta["Related"].split(",") if s.strip()]:
            if not RELATED_SLUG.match(slug):
                problems.append(
                    f"'**Related:**' entry '{slug}' must be a bare rule slug "
                    "(filename without .md, no path)"
                )

    # Nothing but the metadata block may sit before the first body line: the
    # loop above already stopped at the first non-metadata line, so if the block
    # was empty we would have flagged missing When/Severity. Guard the case
    # where prose precedes the block entirely.
    if "When" not in meta and "Severity" not in meta and not problems:
        problems.append("no metadata block found directly after the title")

    return problems


def main():
    root = Path(__file__).resolve().parents[2]
    rules_dir = root / "ai" / "rules"
    quiet = "--quiet" in sys.argv

    if not rules_dir.is_dir():
        print(f"error: {rules_dir} not found", file=sys.stderr)
        sys.exit(1)

    failures = {}
    n = 0
    for md in sorted(rules_dir.glob("*.md")):
        if md.name in SKIP:
            continue
        n += 1
        problems = check_rule(md)
        if problems:
            failures[md.name] = problems

    if failures:
        if not quiet:
            print(f"rules_lint: {len(failures)}/{n} rule file(s) violate the format\n")
            for name, problems in sorted(failures.items()):
                print(f"  ai/rules/{name}")
                for p in problems:
                    print(f"      - {p}")
            print("\nFormat spec: ai/rules/rule-format.md")
        sys.exit(1)

    if not quiet:
        print(f"rules_lint: {n} rule file(s) conform to ai/rules/rule-format.md")


if __name__ == "__main__":
    main()
