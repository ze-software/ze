#!/usr/bin/env python3
"""Combine multiple gomu mutation-report JSON files into one."""

import glob
import json
import os
import sys


def main():
    reports = sorted(glob.glob("tmp/mutation-report-*.json"))
    if not reports:
        print("No reports generated")
        return

    combined = {"results": [], "summary": {}}
    for path in reports:
        with open(path) as f:
            data = json.load(f)
        combined["results"].extend(data.get("results", []))

    total = len(combined["results"])
    killed = sum(1 for r in combined["results"] if r["status"] != "SURVIVED")
    survived = total - killed
    score = round(100 * killed / total, 1) if total else 0

    combined["summary"] = {
        "total": total,
        "killed": killed,
        "survived": survived,
        "score": score,
    }

    with open("tmp/mutation-report.json", "w") as f:
        json.dump(combined, f, indent=2)

    print(f"Combined: {killed}/{total} killed ({score}%), {survived} survived")

    for path in reports:
        os.unlink(path)


if __name__ == "__main__":
    main()
