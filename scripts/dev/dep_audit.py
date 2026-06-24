#!/usr/bin/env python3
"""Reverse-dependency audit: find which components/plugins are import-independent.

Complements scripts/dev/arch_map.py (which lists the directories) by answering a
different question: for each top-level dir under an area, WHO imports it from
OUTSIDE its own subtree?

A directory is a "plugin candidate" when its only importers are registration
blank-imports (the generated composition root internal/component/plugin/all/all.go
and the cmd/ze dispatch files) plus tests. Nothing in the core or any sibling
calls into it directly -- it is reachable only through the registry, so it already
passes the "delete the folder" test (ai/rules/plugin-self-containment.md) and could
live under internal/plugins/ unchanged. Directories WITH external importers are
genuine core/shared code (the daemon wires them, siblings call them).

This is a static text scan of import strings, so it is build-tag agnostic on
purpose: it sees linux-only importers that `go list` on darwin would miss. The
trade-off is it cannot tell a used import from a dead one -- it answers "who could
reference this", which is the right question for a "can this move" audit.

Usage:
    scripts/dev/dep_audit.py [AREA ...] [--json] [--candidates-only]   # report
    scripts/dev/dep_audit.py --check            # Path C engine-placement GATE
    scripts/dev/dep_audit.py --write-baseline   # (re)generate the migration baseline

    AREA defaults to the three registries:
        internal/component  internal/plugins  internal/component/bgp/plugins

Report exit code: 0 always.
Gate (--check) exit codes: 0 = compliant (or only baselined), 2 = a new misplaced
engine OR a stale baseline entry OR the pluginDirs parse failed.

Module-tier rule (Path C, see ai/rules/module-tiers.md): a config-driven engine
(`sdk.NewWithConn`) at a top-level subsystem MUST live in internal/component/ if a
feature depends on it, else in internal/plugins/. Nested sub-plugin namespaces
(from the generator's pluginDirs) are excluded. core/composition tiers are reported
but NOT enforced (advisory). The only exception list is the transitional migration
baseline (scripts/dev/tier_migration_baseline.txt); it shrinks to zero as the
tiers-2/3 child specs land. It is NOT a permanent allowlist.
"""

import json
import os
import re
import subprocess
import sys
from collections import defaultdict

DEFAULT_AREAS = (
    "internal/component",
    "internal/plugins",
    "internal/component/bgp/plugins",
)


def repo_root() -> str:
    return subprocess.run(
        ["git", "rev-parse", "--show-toplevel"],
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()


def module_path(root: str) -> str:
    with open(os.path.join(root, "go.mod"), encoding="utf-8") as fh:
        for line in fh:
            if line.startswith("module "):
                return line.split(None, 1)[1].strip()
    raise SystemExit("go.mod: no module line found")


def is_registration_importer(rel: str) -> bool:
    """True for files that only blank-import for side-effect registration."""
    if rel.endswith("/all/all.go"):
        return True
    if rel.startswith("cmd/ze/") and ("dispatch" in rel or rel.endswith("_imports.go")):
        return True
    return False


def collect_edges(root: str, module: str) -> dict:
    """imported package import-path -> set of importer file paths (repo-relative)."""
    edges = defaultdict(set)
    imp_re = re.compile(r'"(' + re.escape(module) + r'/[^"]+)"')
    for dirpath, dirs, files in os.walk(root):
        dirs[:] = [
            d for d in dirs if d not in (".git", "vendor", "tmp", "node_modules")
        ]
        for f in files:
            if not f.endswith(".go"):
                continue
            full = os.path.join(dirpath, f)
            rel = os.path.relpath(full, root)
            try:
                txt = open(full, encoding="utf-8", errors="ignore").read()
            except OSError:
                continue
            for m in imp_re.finditer(txt):
                edges[m.group(1)].add(rel)
    return edges


def classify(area: str, root: str, module: str, edges: dict, engines: set) -> list:
    base = module + "/" + area
    area_dir = os.path.join(root, area)
    out = []
    for top in sorted(os.listdir(area_dir)):
        if not os.path.isdir(os.path.join(area_dir, top)):
            continue
        subsystem = area + "/" + top
        pkg_prefix = base + "/" + top
        own_rel_prefix = os.path.join(area, top) + os.sep
        external, registration, tests = set(), set(), set()
        for imported, importers in edges.items():
            if imported != pkg_prefix and not imported.startswith(pkg_prefix + "/"):
                continue
            for imp in importers:
                if imp.startswith(own_rel_prefix):
                    continue  # inside its own subtree
                if imp.endswith("_test.go"):
                    tests.add(imp)
                elif is_registration_importer(imp):
                    registration.add(imp)
                else:
                    external.add(imp)
        # "wired" = blank-imported by a composition root (the generated all.go OR
        # cmd/ze dispatch), per is_registration_importer. This is the authoritative
        # "the build treats this subsystem as a plugin/command" signal -- it reuses
        # the generator's output (all.go) plus the dispatch root, instead of a
        # per-mechanism registration grep that mis-sends *-cmd dirs to core
        # (umbrella blocker B-1). It catches every shape: registry.Register,
        # RegisterRPCs, RegisterBackend, doctor checks, and *-cmd verb providers.
        is_registered = len(registration) > 0
        is_engine = subsystem in engines
        out.append(
            {
                "name": top,
                "external": sorted(external),
                "registration": sorted(registration),
                "tests": sorted(tests),
                "is_candidate": len(external) == 0,
                "is_registered": is_registered,
                "is_engine": is_engine,
                # a genuine internal/core candidate: a leaf nothing depends on that
                # no composition root wires and that is not a config-driven engine.
                "core_candidate": len(external) == 0
                and not is_registered
                and not is_engine,
            }
        )
    return out


# ---------------------------------------------------------------------------
# Disableable-feature direct-import audit (feature-gate compile-out)
# ---------------------------------------------------------------------------
#
# A compile-out-able feature (guarded by //go:build ze_<feature>) must be reached
# ONLY through build-tag-gated registration. A direct functional import from
# always-on (untagged) code pins the package into every binary, defeating the
# compile-out. This audit flags any non-test, out-of-tree file that imports a
# disableable package without the gating build tag.
# See plan/spec-feature-gate-0-umbrella.md and ai/rules/module-tiers.md.
DISABLEABLE = {
    # gated package (repo-relative) -> required //go:build tag
    "internal/component/lg": "ze_lg",
}


def _tag_required(constraint: str, tag: str) -> bool:
    """True if a //go:build constraint string positively requires tag."""
    if re.search(r"!\s*" + re.escape(tag) + r"\b", constraint):
        return False
    return re.search(r"\b" + re.escape(tag) + r"\b", constraint) is not None


def file_requires_tag(root: str, rel: str, tag: str) -> bool:
    """True if file rel carries a //go:build constraint requiring tag."""
    try:
        txt = open(os.path.join(root, rel), encoding="utf-8", errors="ignore").read()
    except OSError:
        return False
    for line in txt.splitlines():
        s = line.strip()
        if s.startswith("//go:build"):
            return _tag_required(s[len("//go:build") :], tag)
        if s and not s.startswith("//"):
            return False  # reached code before any build constraint
    return False


def disableable_violations(
    root: str, module: str, edges: dict, disableable=None
) -> list:
    """Always-on files that directly import a disableable feature package."""
    if disableable is None:
        disableable = DISABLEABLE
    out = []
    for pkg, tag in sorted(disableable.items()):
        import_path = module + "/" + pkg
        own_prefix = pkg + "/"
        for imp in sorted(edges.get(import_path, ())):
            if imp.startswith(own_prefix):
                continue  # the feature's own tree
            if imp.endswith("_test.go"):
                continue  # tests may import the package directly
            if not file_requires_tag(root, imp, tag):
                out.append((imp, pkg, tag))
    return out


def disableable_gate(root: str, module: str, edges: dict) -> int:
    viol = disableable_violations(root, module, edges)
    if not viol:
        return 0
    print(
        "FAIL: disableable feature(s) imported by always-on code -- breaks compile-out:",
        file=sys.stderr,
    )
    for imp, pkg, tag in viol:
        print(f"  {imp} imports {pkg} without //go:build {tag}", file=sys.stderr)
    print(
        "  Rule: a compile-out-able feature is reached ONLY via build-tag-gated "
        "registration (ai/rules/module-tiers.md). Gate the importer with "
        "//go:build <tag> or route it through the service construction registry.",
        file=sys.stderr,
    )
    return 2


# ---------------------------------------------------------------------------
# Path C engine-placement gate
# ---------------------------------------------------------------------------

GENERATOR = "scripts/codegen/plugin_imports.go"
BASELINE = "scripts/dev/tier_migration_baseline.txt"
NON_FEATURE_PREFIXES = (
    "cmd/ze/",
    "internal/core/",
    "internal/chaos/",
    "internal/test/",
)


def parse_plugin_dirs(root: str) -> list:
    """Read the `var pluginDirs = []string{...}` literal from the generator.

    The generator is the authority on where plugins (incl. nested sub-plugin
    namespaces) legitimately live. Parsing it keeps the gate consistent with the
    composition root instead of duplicating a guess (umbrella blocker B-1).
    """
    path = os.path.join(root, GENERATOR)
    with open(path, encoding="utf-8") as fh:
        text = fh.read()
    m = re.search(r"var pluginDirs = \[\]string\{(.*?)\}", text, re.DOTALL)
    if not m:
        return []
    return re.findall(r'"([^"]+)"', m.group(1))


def nested_namespaces(plugin_dirs: list) -> list:
    """Sub-plugin registries nested under a component (e.g. bgp/plugins).

    These are pluginDirs entries under internal/component/ that are deeper than a
    top-level subsystem (>= 4 path segments). Engine packages inside them are
    sub-plugins of their host and are correctly placed, so the tier check skips
    them.
    """
    return [
        pd
        for pd in plugin_dirs
        if pd.startswith("internal/component/") and pd.count("/") >= 3
    ]


def find_engine_dirs(root: str, nested_ns: list) -> list:
    """Repo-relative dirs (under the two areas) whose non-test code calls
    sdk.NewWithConn, excluding anything inside a nested sub-plugin namespace."""
    engines = []
    for area in ("internal/component", "internal/plugins"):
        area_abs = os.path.join(root, area)
        for dirpath, dirs, files in os.walk(area_abs):
            dirs[:] = [d for d in dirs if d not in (".git", "vendor", "tmp")]
            rel = os.path.relpath(dirpath, root)
            if any(rel == ns or rel.startswith(ns + os.sep) for ns in nested_ns):
                continue
            for f in files:
                if not f.endswith(".go") or f.endswith("_test.go"):
                    continue
                try:
                    txt = open(os.path.join(dirpath, f), errors="ignore").read()
                except OSError:
                    continue
                if "sdk.NewWithConn(" in txt:
                    engines.append(rel)
                    break
    return sorted(engines)


def top_subsystem(rel: str) -> str:
    """Map any package path under the two areas to its top-level subsystem dir,
    e.g. internal/component/ike/engine -> internal/component/ike, and
    internal/plugins/log/cmd -> internal/plugins/log."""
    return "/".join(rel.split("/")[:3])


def engine_subsystems(root: str) -> set:
    """Top-level subsystems containing an sdk.NewWithConn engine (nested sub-plugin
    namespaces from the generator's pluginDirs excluded), used only to LABEL the
    advisory tiers -- the enforced gate uses find_engine_dirs directly."""
    plugin_dirs = parse_plugin_dirs(root)
    nested_ns = nested_namespaces(plugin_dirs)
    return {top_subsystem(e) for e in find_engine_dirs(root, nested_ns)}


def engine_depended(engine_rel: str, module: str, edges: dict) -> bool:
    """True if a feature (another component/plugins package) imports the engine
    package's own subtree. Excludes the engine's own subtree, tests, the generated
    composition root, cmd/ze dispatch, and non-feature trees (core/chaos/test)."""
    pkg = module + "/" + engine_rel
    own = engine_rel + os.sep
    for imported, importers in edges.items():
        if imported != pkg and not imported.startswith(pkg + "/"):
            continue
        for imp in importers:
            if imp.startswith(own) or imp.endswith("_test.go"):
                continue
            if is_registration_importer(imp):
                continue
            if any(imp.startswith(p) for p in NON_FEATURE_PREFIXES):
                continue
            if imp.startswith("internal/component/") or imp.startswith(
                "internal/plugins/"
            ):
                return True
    return False


def engine_misplacements(root: str, module: str, edges: dict) -> dict:
    """dir -> expected_area for every misplaced engine (Path C). Raises on a failed
    pluginDirs parse so the gate fails loud rather than flagging everything."""
    plugin_dirs = parse_plugin_dirs(root)
    if not plugin_dirs:
        raise SystemExit(
            f"{GENERATOR}: could not parse pluginDirs -- gate cannot run safely"
        )
    nested_ns = nested_namespaces(plugin_dirs)
    out = {}
    for eng in find_engine_dirs(root, nested_ns):
        area = (
            "internal/component"
            if eng.startswith("internal/component/")
            else "internal/plugins"
        )
        expected = (
            "internal/component"
            if engine_depended(eng, module, edges)
            else "internal/plugins"
        )
        if expected != area:
            out[eng] = expected
    return out


def read_baseline(root: str) -> set:
    path = os.path.join(root, BASELINE)
    if not os.path.exists(path):
        return set()
    keys = set()
    for line in open(path, encoding="utf-8"):
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        keys.add(line.split()[0])
    return keys


def write_baseline(root: str, mis: dict) -> None:
    lines = [
        "# Tier migration baseline -- TRANSITIONAL, not a permanent allowlist.",
        "# Each row is a misplaced engine scheduled to move; the gate FAILS on a NEW",
        "# misplacement and on a STALE entry (one no longer misplaced). An empty file",
        "# means full engine-placement enforcement with zero exceptions.",
        "# See ai/rules/module-tiers.md and plan/spec-tiers-0-umbrella.md.",
        "# columns: <current-dir>\\t<expected-area>\\t<resolving-child-spec>",
    ]
    for eng in sorted(mis):
        spec = "spec-tiers-2" if mis[eng].endswith("plugins") else "spec-tiers-3"
        lines.append(f"{eng}\t{mis[eng]}\t{spec}")
    out = os.path.join(root, BASELINE)
    os.makedirs(os.path.dirname(out), exist_ok=True)
    with open(out, "w", encoding="utf-8") as fh:
        fh.write("\n".join(lines) + "\n")


def run_gate(root: str, module: str, edges: dict) -> int:
    """Combined --check gate: engine placement (Path C) + disableable imports."""
    engine_rc = engine_placement_gate(root, module, edges)
    dis_rc = disableable_gate(root, module, edges)
    return engine_rc or dis_rc


def engine_placement_gate(root: str, module: str, edges: dict) -> int:
    mis = engine_misplacements(root, module, edges)
    baseline = read_baseline(root)
    current = set(mis)
    new = current - baseline
    stale = baseline - current

    if new:
        print("FAIL: new misplaced engine(s) -- wrong module tier:", file=sys.stderr)
        for eng in sorted(new):
            print(f"  {eng}  must move to {mis[eng]}/", file=sys.stderr)
        print(
            "  Rule: ai/rules/module-tiers.md (engine -> component if a feature "
            "depends on it, else plugins).",
            file=sys.stderr,
        )
        print(
            f"  If this is an intentional, scheduled move, add it to {BASELINE}.",
            file=sys.stderr,
        )
    if stale:
        print(
            "FAIL: stale baseline entry(ies) -- no longer misplaced, remove from "
            f"{BASELINE}:",
            file=sys.stderr,
        )
        for eng in sorted(stale):
            print(f"  {eng}", file=sys.stderr)
    if new or stale:
        return 2

    if current:
        print(
            f"OK: engine placement clean; {len(current)} engine(s) baselined "
            "(pending migration):"
        )
        for eng in sorted(current):
            print(f"  {eng} -> {mis[eng]}/")
    else:
        print("OK: engine placement clean; no exceptions (baseline empty).")
    print(
        "advisory: Path C enforces only engine placement; core/composition tiers "
        "are reported by `dep_audit.py` (no --check) but NOT gated."
    )
    return 0


def _quiet_gate(root: str, module: str, edges: dict) -> int:
    import contextlib
    import io

    with (
        contextlib.redirect_stdout(io.StringIO()),
        contextlib.redirect_stderr(io.StringIO()),
    ):
        return run_gate(root, module, edges)


def selftest() -> int:
    """Isolated fixture tests for the Path C gate -- no repo mutation.

    Mirrors the `--selftest` convention of scripts/dev/audit-test-relaxation.py.
    """
    import tempfile

    def w(root: str, rel: str, content: str) -> None:
        p = os.path.join(root, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "w", encoding="utf-8") as fh:
            fh.write(content)

    with tempfile.TemporaryDirectory() as root:
        mod = "example.com/m"
        w(root, "go.mod", f"module {mod}\n")
        w(
            root,
            "scripts/codegen/plugin_imports.go",
            "var pluginDirs = []string{\n"
            '\t"internal/component/host/plugins",\n'
            '\t"internal/plugins",\n}\n',
        )
        # edge engine in component, nothing depends on it -> belongs in plugins
        w(
            root,
            "internal/component/edgeproto/r.go",
            "package edgeproto\nfunc R(){ sdk.NewWithConn() }\n",
        )
        # platform engine in component, a feature depends on it -> correct
        w(
            root,
            "internal/component/platform/r.go",
            f'package platform\nimport _ "{mod}/internal/plugins/platformx"\nfunc R(){{ sdk.NewWithConn() }}\n',
        )
        w(
            root,
            "internal/plugins/consumer/r.go",
            f'package consumer\nimport _ "{mod}/internal/component/platform"\n',
        )
        # nested sub-plugin engine -> excluded by pluginDirs
        w(
            root,
            "internal/component/host/plugins/sub/r.go",
            "package sub\nfunc R(){ sdk.NewWithConn() }\n",
        )
        # edge engine already in plugins -> correct
        w(
            root,
            "internal/plugins/edge1/r.go",
            "package edge1\nfunc R(){ sdk.NewWithConn() }\n",
        )
        # platform engine stuck in plugins (a feature depends on it) -> belongs in component
        w(
            root,
            "internal/plugins/platformx/r.go",
            "package platformx\nfunc R(){ sdk.NewWithConn() }\n",
        )
        # --- classify / B-1 fixtures: "wired" via a composition root, not an engine ---
        # command plugin wired ONLY via cmd/ze dispatch (the *-cmd shape the all.go-only
        # signal used to mis-send to core) -> must classify as a registered plugin.
        w(root, "internal/plugins/cmdverb/r.go", "package cmdverb\n")
        w(
            root,
            "cmd/ze/ze_core_dispatch.go",
            f'package main\nimport _ "{mod}/internal/plugins/cmdverb"\n',
        )
        # plugin wired via the generated composition root (all.go) -> registered plugin.
        w(root, "internal/plugins/genverb/r.go", "package genverb\n")
        w(
            root,
            "internal/component/plugin/all/all.go",
            f'package all\nimport _ "{mod}/internal/plugins/genverb"\n',
        )
        # genuine leaf: no composition root wires it, nothing imports it, not an engine.
        w(root, "internal/plugins/leaflib/lib.go", "package leaflib\n")
        # shared library: a feature imports it -> has an external importer (not a candidate).
        w(root, "internal/plugins/sharedlib/lib.go", "package sharedlib\n")
        w(
            root,
            "internal/plugins/edge1/use.go",
            f'package edge1\nimport _ "{mod}/internal/plugins/sharedlib"\n',
        )

        edges = collect_edges(root, mod)
        mis = engine_misplacements(root, mod, edges)
        expect = {
            "internal/component/edgeproto": "internal/plugins",
            "internal/plugins/platformx": "internal/component",
        }
        assert mis == expect, f"misplacements {mis} != {expect}"
        assert "internal/component/host/plugins/sub" not in mis, (
            "nested sub-plugin not excluded"
        )
        assert "internal/component/platform" not in mis, (
            "depended component engine wrongly flagged"
        )
        assert "internal/plugins/edge1" not in mis, "edge plugin wrongly flagged"

        # --- classify / B-1: wired-vs-core determination from composition roots ---
        engines = engine_subsystems(root)
        rows = {
            r["name"]: r
            for r in classify("internal/plugins", root, mod, edges, engines)
        }
        assert rows["cmdverb"]["is_registered"], (
            "dispatch-wired *-cmd not seen as wired"
        )
        assert not rows["cmdverb"]["core_candidate"], (
            "wired *-cmd mis-sent to core (B-1)"
        )
        assert rows["genverb"]["is_registered"], "all.go-wired plugin not seen as wired"
        assert not rows["genverb"]["core_candidate"], (
            "all.go-wired plugin mis-sent to core"
        )
        assert rows["leaflib"]["core_candidate"], "genuine leaf not a core candidate"
        assert not rows["leaflib"]["is_registered"], (
            "unwired leaf wrongly seen as wired"
        )
        assert not rows["sharedlib"]["core_candidate"], (
            "depended lib wrongly a core candidate"
        )
        assert rows["sharedlib"]["external"], (
            "shared lib should have an external importer"
        )

        # --- disableable-feature direct-import audit (feature-gate compile-out) ---
        w(root, "internal/component/widget/w.go", "package widget\n")
        # gated importer (correct): reaches widget only behind its build tag
        w(
            root,
            "cmd/app/svc_widget.go",
            f'//go:build ze_widget\n\npackage app\nimport _ "{mod}/internal/component/widget"\n',
        )
        # always-on importer (violation): imports widget with no build tag
        w(
            root,
            "internal/app/use.go",
            f'package app\nimport _ "{mod}/internal/component/widget"\n',
        )
        # test importer (allowed: tests don't affect the production binary)
        w(
            root,
            "internal/app/use_test.go",
            f'package app\nimport _ "{mod}/internal/component/widget"\n',
        )
        dis_edges = collect_edges(root, mod)
        dmap = {"internal/component/widget": "ze_widget"}
        dfiles = {v[0] for v in disableable_violations(root, mod, dis_edges, dmap)}
        assert "internal/app/use.go" in dfiles, (
            "always-on direct import of a disableable feature not flagged"
        )
        assert "cmd/app/svc_widget.go" not in dfiles, (
            "build-tag-gated importer wrongly flagged"
        )
        assert "internal/app/use_test.go" not in dfiles, "test importer wrongly flagged"

        assert _quiet_gate(root, mod, edges) == 2, "empty baseline should fail (new)"
        write_baseline(root, mis)
        assert _quiet_gate(root, mod, edges) == 0, "matching baseline should pass"
        with open(os.path.join(root, BASELINE), "a", encoding="utf-8") as fh:
            fh.write("internal/component/gone\tinternal/plugins\tspec-tiers-2\n")
        assert _quiet_gate(root, mod, edges) == 2, "stale baseline entry should fail"

    print("dep_audit selftest OK")
    return 0


def main() -> int:
    argv = sys.argv[1:]
    as_json = "--json" in argv
    cands_only = "--candidates-only" in argv

    if "--selftest" in argv:
        return selftest()

    if "--check" in argv or "--write-baseline" in argv:
        root = repo_root()
        module = module_path(root)
        edges = collect_edges(root, module)
        if "--write-baseline" in argv:
            mis = engine_misplacements(root, module, edges)
            write_baseline(root, mis)
            print(f"wrote {BASELINE} with {len(mis)} engine(s)")
            return 0
        return run_gate(root, module, edges)
    areas = [a for a in argv if not a.startswith("--")] or list(DEFAULT_AREAS)

    root = repo_root()
    module = module_path(root)
    edges = collect_edges(root, module)

    engines = engine_subsystems(root)
    report = {area: classify(area, root, module, edges, engines) for area in areas}

    if as_json:
        if cands_only:
            report = {
                a: [r for r in rs if r["core_candidate"]] for a, rs in report.items()
            }
        print(json.dumps({"module": module, "areas": report}, indent=2))
        return 0

    for area, rows in report.items():
        plugins = [r for r in rows if r["is_registered"]]
        core_cands = [r for r in rows if r["core_candidate"]]
        shared = sorted(
            (r for r in rows if not r["is_registered"] and not r["core_candidate"]),
            key=lambda r: -len(r["external"]),
        )
        print("\n" + "=" * 78)
        print("AREA:", area)
        print("=" * 78)
        print(
            f"\n-- REGISTERED PLUGINS (wired by the generator / all.go): "
            f"{len(plugins)} --"
        )
        for r in plugins:
            eng = "engine" if r["is_engine"] else "  -   "
            print(f"  {r['name']:24s} {eng}  external={len(r['external'])}")
        print(
            f"\n-- CORE CANDIDATES (0 external, not registered, not an engine): "
            f"{len(core_cands)} --"
        )
        for r in core_cands:
            print(
                f"  {r['name']:24s} registration={len(r['registration'])} "
                f"tests={len(r['tests'])}"
            )
        if cands_only:
            continue
        print(
            f"\n-- SHARED LIBRARIES (external importers, not a registered plugin): "
            f"{len(shared)} --"
        )
        for r in shared:
            print(f"  {r['name']:24s} external={len(r['external'])}")
            for s in r["external"][:6]:
                print(f"        <- {s}")
            if len(r["external"]) > 6:
                print(f"        ... and {len(r['external']) - 6} more")
    return 0


if __name__ == "__main__":
    sys.exit(main())
