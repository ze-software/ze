#!/usr/bin/env python3
"""Extract the live YANG config tree from `ze yang tree --json --config`.

Usage:
    tools/extract-yang-config-tree.py

Runs ../main/bin/ze yang tree --json --config -- Ze's own unified
config-tree walker (internal/component/config/yang/cli/tree.go,
BuildUnifiedTree; format.go, FormatTreeJSON) -- and caches it to
data/yang-config-tree.json, keyed by top-level container name (bgp,
interface, firewall, ...). tools/render-config-reference.py uses this to
show a readable, command-line-shaped config tree per group instead of
asking a non-developer to read raw YANG module source. Run `make ze` in
../main first if bin/ze is missing or stale, same requirement as
tools/render-cli-catalog.py.
"""

import json
import pathlib
import subprocess
import sys

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
ZE_BINARY = GH_PAGES.parent / "main" / "bin" / "ze"
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
