#!/usr/bin/env python3
"""Advisory protocol-skeleton conformance report.

Lists each protocol's subpackages classified against the standard skeleton
(ai/rules/protocol.md): canonical module, RFC-named per-peer state,
wire-version dir, domain module, or documented legacy exception. Prints a
one-line summary by default; --verbose shows the per-protocol table.

ADVISORY: report mode ALWAYS exits 0 -- this is a lens, not a gate (an
enforced skeleton would need a large allowlist; see the tiers Path B lesson,
plan/spec-tiers-0-umbrella.md). Only --selftest may exit non-zero.

Usage:
    scripts/dev/protocol_skeleton_report.py [--verbose]
    scripts/dev/protocol_skeleton_report.py --selftest

Wired as the last, non-enforcing line of `make ze-tier-check`.
"""

import os
import re
import sys

# The protocol manifest: display name -> repo-relative root. Protocols are not
# mechanically discoverable ("is a dir a protocol?" needs judgment), so this
# small list is declared here and mirrored by the probe table in
# ai/rules/protocol.md. Add a row when a protocol lands.
PROTOCOLS = {
    "bgp": "internal/component/bgp",
    "bfd": "internal/component/bfd",
    "ike": "internal/component/ike",
    "isis": "internal/plugins/isis",
    "ospf": "internal/plugins/ospf",
    "ldp": "internal/plugins/ldp",
    "rsvpte": "internal/plugins/rsvpte",
}

# Canonical skeleton modules (ai/rules/protocol.md, speaking the
# ai/rules/go-standards.md glossary).
CANONICAL = {
    "packet",
    "transport",
    "engine",
    "yang",
    "types",
    "cli",
    "cmd",
    "redistribute",
}

# Per-peer conversation state named by the protocol's own RFC term.
RFC_STATE = {"session", "adjacency", "neighbor", "fsm"}

# Documented legacy exceptions ((protocol, module)): kept names that predate
# the glossary. Mirrors the exceptions table in ai/rules/protocol.md.
LEGACY_EXCEPTIONS = {
    ("bgp", "message"),
    ("bgp", "wireu"),
    ("bgp", "reactor"),
    ("ike", "wire"),
}

VERSION_RE = re.compile(r"^v\d+$")


def classify_module(protocol: str, module: str) -> str:
    """One of: canonical | rfc-state | version | legacy-exception | domain."""
    if (protocol, module) in LEGACY_EXCEPTIONS:
        return "legacy-exception"
    if module in CANONICAL:
        return "canonical"
    if module in RFC_STATE:
        return "rfc-state"
    if VERSION_RE.match(module):
        return "version"
    return "domain"


def protocol_modules(root: str, rel: str) -> list:
    """Immediate subpackage dirs of a protocol root (skips testdata/hidden)."""
    base = os.path.join(root, rel)
    if not os.path.isdir(base):
        return []
    out = []
    for name in sorted(os.listdir(base)):
        if name.startswith(".") or name == "testdata":
            continue
        if os.path.isdir(os.path.join(base, name)):
            out.append(name)
    return out


def build_report(root: str, protocols: dict) -> dict:
    """protocol -> {"modules": {module: class}, "single_package": bool,
    "missing": bool}. A manifest row whose root no longer exists is marked
    missing (visible in every output mode) rather than silently rendering as
    single-package -- the manifest is hand-maintained and can go stale."""
    report = {}
    for proto, rel in protocols.items():
        missing = not os.path.isdir(os.path.join(root, rel))
        modules = protocol_modules(root, rel)
        classified = {m: classify_module(proto, m) for m in modules}
        # single-package: no subpackages beyond the uniform yang/ schema dir.
        single = not missing and all(m == "yang" for m in modules)
        report[proto] = {
            "modules": classified,
            "single_package": single,
            "missing": missing,
        }
    return report


def render(report: dict, verbose: bool) -> str:
    counts = {
        "canonical": 0,
        "rfc-state": 0,
        "version": 0,
        "domain": 0,
        "legacy-exception": 0,
    }
    for data in report.values():
        for cls in data["modules"].values():
            counts[cls] += 1
    lines = []
    missing = sorted(p for p, d in report.items() if d["missing"])
    if verbose:
        for proto in sorted(report):
            data = report[proto]
            if data["missing"]:
                lines.append(f"{proto}: MISSING root -- stale PROTOCOLS manifest row?")
                continue
            if data["single_package"]:
                lines.append(f"{proto}: single-package (root + yang/); skeleton N/A")
                continue
            per = ", ".join(f"{m}={cls}" for m, cls in sorted(data["modules"].items()))
            lines.append(f"{proto}: {per}")
    missing_note = f"; MISSING roots: {', '.join(missing)}" if missing else ""
    lines.append(
        "protocol-skeleton advisory: "
        f"{len(report)} protocols; "
        f"canonical {counts['canonical']}, rfc-state {counts['rfc-state']}, "
        f"version {counts['version']}, domain {counts['domain']}, "
        f"legacy {counts['legacy-exception']}"
        f"{missing_note} "
        "(ai/rules/protocol.md; --verbose for detail)"
    )
    return "\n".join(lines)


def selftest() -> int:
    """Fixture classifications; the one mode allowed to exit non-zero."""
    import tempfile

    failed = []

    def check(cond: bool, msg: str) -> None:
        if not cond:
            failed.append(msg)

    # classification table
    check(classify_module("bfd", "packet") == "canonical", "packet not canonical")
    check(classify_module("bfd", "session") == "rfc-state", "session not rfc-state")
    check(
        classify_module("isis", "adjacency") == "rfc-state", "adjacency not rfc-state"
    )
    check(classify_module("ospf", "neighbor") == "rfc-state", "neighbor not rfc-state")
    check(classify_module("bgp", "fsm") == "rfc-state", "fsm not rfc-state")
    check(classify_module("ospf", "v3") == "version", "v3 not version")
    check(classify_module("bgp", "wireu") == "legacy-exception", "bgp/wireu not legacy")
    check(classify_module("ike", "wire") == "legacy-exception", "ike/wire not legacy")
    check(
        classify_module("ospf", "wire") == "domain",
        "ospf/wire must be domain (raw handoff, not the ike exception)",
    )
    check(classify_module("isis", "lsdb") == "domain", "lsdb not domain")

    # tree walking + single-package detection on a fixture
    with tempfile.TemporaryDirectory() as root:
        for d in (
            "internal/plugins/demo/packet",
            "internal/plugins/demo/yang",
            "internal/plugins/demo/testdata",
            "internal/plugins/flat/yang",
        ):
            os.makedirs(os.path.join(root, d))
        rep = build_report(
            root,
            {"demo": "internal/plugins/demo", "flat": "internal/plugins/flat"},
        )
        check(
            rep["demo"]["modules"] == {"packet": "canonical", "yang": "canonical"},
            f"demo modules wrong: {rep['demo']['modules']} (testdata must be skipped)",
        )
        check(not rep["demo"]["single_package"], "demo wrongly single-package")
        check(rep["flat"]["single_package"], "flat (root+yang only) not single-package")
        gone = build_report(root, {"gone": "internal/plugins/gone"})
        check(gone["gone"]["missing"], "missing root not flagged")
        check(
            not gone["gone"]["single_package"],
            "missing root wrongly rendered single-package",
        )
        check(
            "MISSING roots: gone" in render(gone, False),
            "missing root absent from summary line",
        )

    # report mode contract: real-tree render never signals failure
    real = build_report(repo_root(), PROTOCOLS)
    check("protocol-skeleton advisory:" in render(real, False), "summary line missing")

    if failed:
        print("protocol-skeleton selftest FAILED:", file=sys.stderr)
        for m in failed:
            print(f"  {m}", file=sys.stderr)
        return 1
    print("protocol-skeleton selftest OK")
    return 0


def repo_root() -> str:
    return os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


def main() -> int:
    argv = sys.argv[1:]
    if "--selftest" in argv:
        return selftest()
    verbose = "--verbose" in argv
    report = build_report(repo_root(), PROTOCOLS)
    print(render(report, verbose))
    return 0  # ADVISORY: report mode never fails the build.


if __name__ == "__main__":
    sys.exit(main())
