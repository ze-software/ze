#!/usr/bin/env python3
"""Count deferral shards whose source spec is gone, split by whether rows are live.

Why this is a script and not a number somebody typed. `ai/rules/planning.md`
and the `deferral_shard_removal_problems` docstring both quote this measurement to
justify a BLOCKING rule, and a quoted count is a claim. The first hand-count of it
was wrong twice: 62 (summed from a printed list by eye) and then 40/71 (a shard whose
name carried a doubled `spec-` prefix read as orphaned when its spec was alive). Both
survived a reading. Neither survived being re-derived. Run this instead of re-deriving
it, and paste its output.

    python3 scripts/dev/deferral_orphans.py
    python3 scripts/dev/deferral_orphans.py --list

A shard is ORPHANED when no spec file on disk corresponds to its stem. `ad-hoc-*`
shards have no source spec by design and are never orphaned. An orphaned shard
holding a live row is the CORRECT end state, not mess: the rows are homed at other
specs and the shard is only where they are written down. An orphaned shard whose
rows are ALL terminal is residue, and closure of the last spec that homed one of its
rows removes it.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path
from typing import NamedTuple

sys.path.insert(0, str(Path(__file__).resolve().parent))

from commit_helper import (  # noqa: E402
    DEFERRAL_TERMINAL_STATUSES,
    _deferral_row_cells,
    deferral_shard_paths,
)


def spec_for_shard(repo: Path, stem: str) -> Path | None:
    """The spec a shard belongs to, or None when the shard is orphaned.

    `plan/deferrals/<stem>.md` pairs with `plan/spec-<stem>.md` by convention.
    Accepting ONLY that is what produced the 40/71 miscount: one shard's name
    carried a doubled `spec-` prefix, so the lookup asked for
    `plan/spec-spec-...md`, found nothing, and read the shard as orphaned while
    its spec was alive and in progress.

    The second spelling is therefore gated on the stem ALREADY starting with
    `spec-`, rather than being tried for every stem. An ungated `plan/<stem>.md`
    would pair a shard with any same-named file directly under `plan/` --
    `implementation-order.md`, `README.md`, a review document -- and silently
    drop it from the orphan count. No such collision exists today, which is
    exactly when a loose lookup is cheapest to tighten.
    """
    if (repo / f"plan/spec-{stem}.md").is_file():
        return repo / f"plan/spec-{stem}.md"
    if stem.startswith("spec-") and (repo / f"plan/{stem}.md").is_file():
        return repo / f"plan/{stem}.md"
    return None


def live_rows(path: Path) -> list[str]:
    """The non-terminal rows of a shard, rendered short."""
    out: list[str] = []
    for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
        cells = _deferral_row_cells(line)
        if cells is None or cells == ["MALFORMED"]:
            continue
        if cells[5].lower() in DEFERRAL_TERMINAL_STATUSES:
            continue
        out.append(f"[{cells[5]}] {cells[2][:70]}")
    return out


class Orphans(NamedTuple):
    """What this script reports about plan/deferrals/. NOT a partition.

    `live_bearing` and `residue` are exclusive, and together they cover the
    ORPHANED shards only: a shard whose spec is alive is in neither. `misnamed`
    is a cross-cutting axis over all shards, so a doubled-prefix shard with no
    spec at either spelling appears in `misnamed` AND in one orphan bucket.
    """

    live_bearing: list[tuple[str, list[str]]]
    residue: list[str]
    misnamed: list[str]

    @property
    def live_row_count(self) -> int:
        return sum(len(rows) for _, rows in self.live_bearing)


def classify(repo: Path) -> Orphans:
    """Sort the orphaned shards into live-bearing and residue, and flag misnamed ones.

    A shard whose spec is alive is reported in neither orphan bucket. `misnamed`
    is independent of both and can overlap either: see the Orphans docstring.

    Separate from main() so the counting is testable without scraping stdout.
    main() is then only a printer, and a test that walks the corpus the same way
    the script does would prove nothing about the script.
    """
    live_bearing: list[tuple[str, list[str]]] = []
    residue: list[str] = []
    misnamed: list[str] = []

    for path in deferral_shard_paths(repo):
        rel = path.relative_to(repo).as_posix()
        if path.name in {"README.md", "RESOLVED.md"} or path.stem.startswith("ad-hoc-"):
            continue
        if path.stem.startswith("spec-"):
            misnamed.append(rel)
        if spec_for_shard(repo, path.stem) is not None:
            continue
        rows = live_rows(path)
        if rows:
            live_bearing.append((rel, rows))
        else:
            residue.append(rel)
    return Orphans(live_bearing, residue, misnamed)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--list", action="store_true", help="name every shard")
    parser.add_argument("--repo", default=".", help="repository root")
    args = parser.parse_args()

    found = classify(Path(args.repo).resolve())
    live_bearing, residue, misnamed = found
    total = found.live_row_count
    print(f"orphaned, live-bearing: {len(live_bearing)} shards, {total} live rows")
    print(f"orphaned, all-terminal (residue): {len(residue)} shards")
    if misnamed:
        print(
            f"misnamed (stem carries a redundant `spec-` prefix, so every gate"
            f" pairing a shard with plan/spec-<stem>.md is blind to it):"
            f" {len(misnamed)}"
        )
        for rel in misnamed:
            print(f"  {rel}")
    if args.list:
        print("\nlive-bearing:")
        for rel, rows in live_bearing:
            print(f"  {rel} ({len(rows)})")
            for row in rows:
                print(f"      {row}")
        print("\nresidue:")
        for rel in residue:
            print(f"  {rel}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
