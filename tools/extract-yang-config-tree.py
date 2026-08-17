#!/usr/bin/env python3
"""Extract the live YANG config tree from `ze yang tree --json --config`.

Usage:
    tools/extract-yang-config-tree.py
Runs the current build session's `ze yang tree --json --config` and caches the
unified configuration tree to data/yang-config-tree.json. The tree is keyed by
top-level container name, such as bgp and interface.

Run `make ze` in ../main before this tool if the binary is missing or stale.
"""

import json
import pathlib
import subprocess
import sys

import zebinary

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
ZE_BINARY = zebinary.resolve(GH_PAGES.parent / "main")
DEST = GH_PAGES / "data" / "yang-config-tree.json"


def fetch_tree():
    if not ZE_BINARY.exists():
        print(
            "error: %s not found -- run `make ze` in ../main first" % ZE_BINARY,
            file=sys.stderr,
        )
        sys.exit(1)
    result = subprocess.run(
        [str(ZE_BINARY), "yang", "tree", "--json", "--config"],
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        print(
            "error: %s yang tree --json --config failed: %s"
            % (ZE_BINARY, result.stderr),
            file=sys.stderr,
        )
        sys.exit(1)
    return json.loads(result.stdout)


def main():
    roots = fetch_tree()
    indexed = {node["name"]: node for node in roots}
    DEST.write_text(json.dumps(indexed, indent=2, sort_keys=True) + "\n")
    print("wrote %d config roots -> %s" % (len(indexed), DEST))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
