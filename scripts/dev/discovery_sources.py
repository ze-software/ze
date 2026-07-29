"""Shared definition of which changed files can drift a generated discovery index.

Two gates must agree on this set: the commit gate
(`commit_helper.feeds_discovery_index`) and the changed-file router
(`verify_wiring_docs.is_discovery_source`). They previously carried the same
path rules twice with a "keep in sync" comment; the rules now live here once.

The two callers differ only in which file text they scan for the `// Package` /
`// Design:` header markers -- the commit gate reads HEAD, the router reads the
working tree plus HEAD -- so each passes its own `header_text`; the path
patterns are shared.

Generated indexes: ai/PACKAGE-MAP.md (from `// Package` docs + register.go
Description), ai/DOCS-TO-CODE.md (from `// Design:` headers), ai/LEARNED-FULL-INDEX.md
(from plan/learned/NNN-*.md).
"""

from __future__ import annotations

import sys
from pathlib import Path

# Named index outputs. Kept as constants (not bare tuple positions) so the
# per-index source map below reads as data, not as a coincidence of ordering.
PACKAGE_MAP = "ai/PACKAGE-MAP.md"  # from `// Package` docs + register.go Description
DOCS_TO_CODE = "ai/DOCS-TO-CODE.md"  # from `// Design:` headers
LEARNED_INDEX = "ai/LEARNED-FULL-INDEX.md"  # from plan/learned/NNN-*.md

# GENERATORS[i] produces OUTPUTS[i]; the two tuples MUST stay parallel (the commit
# gate and the freshness check both `zip()` them).
GENERATORS = (
    "scripts/dev/package_map.py",
    "scripts/dev/docs_to_code.py",
    "scripts/dev/learned_index.py",
)

OUTPUTS = (
    PACKAGE_MAP,
    DOCS_TO_CODE,
    LEARNED_INDEX,
)

HEADER_MARKERS = ("// Package", "// Design:")

# `--check` exit code meaning "the committed output no longer matches its sources".
# Distinct from 1 (the generator itself failed: missing dir, minimal checkout,
# crash) because the commit gate must tell those apart -- it BLOCKS on drift and
# stays warn-only on a broken generator. It used to tell them apart by matching the
# human-facing warning text ("is stale"), which made a wording change silently
# degrade a BLOCKING gate to warn-only: the nonzero exit reads as "generator
# failed", the index becomes unjudgeable, and the commit passes. The exit code is
# the contract; the warning text is for humans and may be reworded freely.
# Declared once here so generators and callers cannot drift (derive-not-hardcode).
STALE_EXIT = 3


def root_from_argv(script_file: str, argv: list[str] | None = None) -> Path:
    """Repo root for a generator: `--root <dir>` when given, else its own repo.

    Every generator derives its root from `__file__` so it can be run from
    anywhere. That makes the WORKING TREE the only tree it can describe, which is
    wrong at commit time: the question a commit gate must answer is whether the
    index will match the tree the commit PRODUCES, and a concurrent session's
    uncommitted sources are not part of that tree. `--root` lets the gate point
    the real generator at a materialized commit view instead of reimplementing
    (and drifting from) its input-gathering.
    """
    args = sys.argv if argv is None else argv
    if "--root" in args:
        i = args.index("--root")
        if i + 1 >= len(args):
            raise SystemExit("error: --root needs a directory argument")
        return Path(args[i + 1]).resolve()
    return Path(script_file).resolve().parents[2]


def indexes_fed_by(path: str, header_text: str = "") -> frozenset[str]:
    """The subset of OUTPUTS that committing `path` can drift.

    This is the DATA the module docstring only stated as prose: each source maps
    to the specific index(es) it feeds, not to "some index". A commit that touches
    a source must refresh ONLY the indexes in this set, so an unrelated index left
    dirty by another session is not falsely demanded (see T-6 in
    plan/spec-fixit-agent-tooling-misleads.md). Returns an empty set when `path`
    feeds no index.

    `header_text` is only consulted for non-test `.go` files (to look for the
    `// Package` / `// Design:` markers the indexes derive from); the caller
    supplies HEAD and/or working-tree content as appropriate.
    """
    # A committed index "feeds" only itself: committing it is how its own
    # freshness is satisfied, and it never obliges any OTHER index to ride along.
    if path in OUTPUTS:
        return frozenset({path})
    # Makefile / mk/ carry the ze-discovery-index wiring that runs every
    # generator, so a change there can drift any index. Conservative: demand all.
    if path == "Makefile" or path.startswith("mk/"):
        return frozenset(OUTPUTS)
    # A generator feeds exactly its paired output.
    for gen, out in zip(GENERATORS, OUTPUTS):
        if path == gen:
            return frozenset({out})
    fed: set[str] = set()
    if path.startswith("plan/learned/") and path.endswith(".md"):
        fed.add(LEARNED_INDEX)
    if path.endswith("register.go"):
        fed.add(PACKAGE_MAP)  # register.go Description strings feed PACKAGE-MAP
    if path.endswith(".go") and not path.endswith("_test.go"):
        if "// Package" in header_text:
            fed.add(PACKAGE_MAP)
        if "// Design:" in header_text:
            fed.add(DOCS_TO_CODE)
    return frozenset(fed)


def is_discovery_source(path: str, header_text: str = "") -> bool:
    """True if committing `path` can change a generated discovery index.

    `header_text` is only consulted for non-test `.go` files (to look for the
    `// Package` / `// Design:` markers the indexes derive from); the caller
    supplies HEAD and/or working-tree content as appropriate. Defined in terms of
    `indexes_fed_by` so the "is it a source" and "which index" answers can never
    disagree.
    """
    return bool(indexes_fed_by(path, header_text))
