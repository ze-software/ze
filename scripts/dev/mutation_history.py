#!/usr/bin/env python3
"""Append per-package mutation scores to the committed history.

gomu writes tmp/mutation-report.json (gitignored) and keeps an incremental
cache; neither survives as a record, so mutation coverage had no trend and no
baseline. This appends one ndjson line per package per run to
test/mutation/history.ndjson (committed), mirroring the perf-history pattern.

Usage: scripts/dev/mutation_history.py [report.json]
Called by: mk/test-mutation.mk after each gomu run (advisory: never fails).
"""

import json
import os
import subprocess
import sys
from datetime import datetime, timezone

HISTORY = "test/mutation/history.ndjson"


def package_of(file_path: str, root: str) -> str:
    rel = file_path
    if rel.startswith(root):
        rel = rel[len(root) :].lstrip("/")
    return os.path.dirname(rel) or "."


def main() -> int:
    report_path = sys.argv[1] if len(sys.argv) > 1 else "tmp/mutation-report.json"
    root = subprocess.run(
        ["git", "rev-parse", "--show-toplevel"],
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()
    os.chdir(root)
    try:
        with open(report_path, encoding="utf-8") as fh:
            report = json.load(fh)
    except (OSError, ValueError) as exc:
        print(f"mutation history: cannot read {report_path}: {exc}", file=sys.stderr)
        return 0  # advisory: never fail the make target

    stats: dict[str, dict[str, int]] = {}
    for result in report.get("results", []):
        file_path = (result.get("mutant") or {}).get("filePath", "")
        pkg = package_of(file_path, root)
        entry = stats.setdefault(pkg, {"mutants": 0, "killed": 0})
        entry["mutants"] += 1
        if result.get("status") != "SURVIVED":
            entry["killed"] += 1

    if not stats:
        print("mutation history: no results in report, nothing recorded")
        return 0

    sha = (
        subprocess.run(
            ["git", "rev-parse", "--short", "HEAD"],
            capture_output=True,
            text=True,
            check=False,
        ).stdout.strip()
        or "unknown"
    )
    ts = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")

    os.makedirs(os.path.dirname(HISTORY), exist_ok=True)
    with open(HISTORY, "a", encoding="utf-8") as fh:
        for pkg in sorted(stats):
            entry = stats[pkg]
            score = round(100 * entry["killed"] / entry["mutants"], 1)
            fh.write(
                json.dumps(
                    {
                        "ts": ts,
                        "sha": sha,
                        "package": pkg,
                        "mutants": entry["mutants"],
                        "killed": entry["killed"],
                        "score": score,
                    },
                    separators=(",", ":"),
                )
                + "\n"
            )
    print(f"mutation history: recorded {len(stats)} package(s) in {HISTORY}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
