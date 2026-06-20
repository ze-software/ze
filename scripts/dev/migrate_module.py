#!/usr/bin/env python3
"""Deterministic module-tier migration tool (see ai/rules/module-tiers.md).

Moves a top-level subsystem directory between internal/component/ and
internal/plugins/ -- the component<->plugin tiers -- performing every mechanical
edit a behaviour-preserving relocation needs, and REFUSING the moves it cannot
make safe mechanically.

What it does (in --apply):
  1. filesystem move of the directory (shutil.move; NEVER `git mv` -- git add/rm/mv
     are forbidden from tooling, the user runs the commit script afterwards).
  2. repo-wide rewrite of quoted, module-qualified Go import paths, boundary-safe
     (only the exact package and its subpackages; `<name>2` is never touched).
  3. edit of the generator's `pluginDirs` literal in scripts/codegen/plugin_imports.go
     (drop the component entry when moving out; add it when moving in).
  4. re-run the generator to regenerate internal/component/plugin/all/all.go.
  5. re-sort import groups with goimports (the move changes alphabetical order,
     which `go build` ignores but golangci-lint's goimports check enforces).
  6. verify the generated all.go blank-import set is preserved (0 registrations
     dropped) by diffing it before/after, normalised for the moved prefix.

Dry-run by DEFAULT: prints the full plan and changes nothing. Pass --apply to execute.

What it REFUSES / reports (cannot be fixed by a pure mover):
  - RPC packages (pluginserver.RegisterRPCs) in a tree moving OUT of component:
    the generator's `rpcRoot` scans only internal/component, so those packages would
    silently drop from all.go. --apply aborts unless --allow-rpc-drop is given
    (pass it only after the generator's discoverRPCPackages has been widened to scan
    internal/plugins). This is the load-bearing precondition for the tiers-2 moves.
  - residual references the import rewrite cannot fix: non-Go surfaces
    (.mk/.sh/.ci/.yang/docs/rfc) and .go comments. plan/ specs are skipped
    (owned by other work). These are reported for manual follow-up.

This tool performs NO git operations. After --apply, verify with `go build ./...`,
diff `bin/ze --plugins`, run `scripts/dev/dep_audit.py --check`, then commit via a
user-run script.

Usage:
    scripts/dev/migrate_module.py NAME [--to {plugins,component}]   # dry-run plan
    scripts/dev/migrate_module.py NAME --to plugins --apply         # execute
    scripts/dev/migrate_module.py --selftest                        # isolated fixtures

Exit codes:
    0  dry-run plan printed, or --apply succeeded
    2  bad arguments / directory not found / ambiguous
    3  --apply refused: unsafe RPC-package drop (widen the generator first)
    4  --apply completed but all.go dropped a registration (investigate)
"""

import argparse
import os
import re
import shutil
import subprocess
import sys

GENERATOR = "scripts/codegen/plugin_imports.go"
ALL_GO = "internal/component/plugin/all/all.go"
AREAS = ("internal/component", "internal/plugins")
SKIP_DIRS = {".git", "vendor", "tmp", "node_modules"}
# Text surfaces scanned for residual references the import rewrite does not touch
# (.go comments, and non-Go build/test/schema/doc files). "Makefile" is matched by
# basename. plan/ is skipped (its specs are owned by other work).
RESIDUAL_EXTS = (
    ".go",
    ".mk",
    ".sh",
    ".ci",
    ".yang",
    ".md",
    ".txt",
    ".py",
    ".yaml",
    ".yml",
    ".json",
    ".toml",
)
RESIDUAL_SKIP_TOP = ("plan",)


# --------------------------------------------------------------------------- #
# repo helpers
# --------------------------------------------------------------------------- #
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


def find_source(root: str, name: str) -> str:
    """Return the area ('internal/component' or 'internal/plugins') that holds NAME.

    Errors if it is in both or neither -- the mover must be unambiguous.
    """
    here = [a for a in AREAS if os.path.isdir(os.path.join(root, a, name))]
    if len(here) == 1:
        return here[0]
    if not here:
        raise SystemExit(
            f"migrate_module: '{name}' not found under {' or '.join(AREAS)}/"
        )
    raise SystemExit(
        f"migrate_module: '{name}' exists in BOTH areas ({', '.join(here)}); refusing"
    )


# --------------------------------------------------------------------------- #
# import rewrite (boundary-safe, module-qualified)
# --------------------------------------------------------------------------- #
def import_regex(prefix: str) -> "re.Pattern":
    """Match a quoted import equal to PREFIX or any subpackage of it.

    The closing `/` or `"` boundary is what stops `<name>` from matching
    `<name>2`: after the prefix the only accepted characters are a path separator
    or the closing quote.
    """
    return re.compile(r'"' + re.escape(prefix) + r'(/[^"]*)?"')


def rewrite_go_imports(root: str, old: str, new: str, apply: bool) -> list:
    """Rewrite quoted imports old->new across all .go files.

    Returns a list of (repo-relative-path, count). Writes only when apply=True.
    """
    rx = import_regex(old)
    hits = []
    for dirpath, dirs, files in os.walk(root):
        dirs[:] = [d for d in dirs if d not in SKIP_DIRS]
        for f in files:
            if not f.endswith(".go"):
                continue
            full = os.path.join(dirpath, f)
            try:
                with open(full, encoding="utf-8") as fh:
                    txt = fh.read()
            except OSError:
                continue
            new_txt, n = rx.subn(lambda m: '"' + new + (m.group(1) or "") + '"', txt)
            if n == 0:
                continue
            hits.append((os.path.relpath(full, root), n))
            if apply and new_txt != txt:
                with open(full, "w", encoding="utf-8") as fh:
                    fh.write(new_txt)
    return sorted(hits)


# --------------------------------------------------------------------------- #
# generator pluginDirs edit
# --------------------------------------------------------------------------- #
PLUGINDIRS_RE = re.compile(r"(var pluginDirs = \[\]string\{\n)(.*?)(\n\})", re.DOTALL)


def parse_plugin_dirs(text: str) -> list:
    m = PLUGINDIRS_RE.search(text)
    if not m:
        raise SystemExit(f"{GENERATOR}: could not locate pluginDirs literal")
    return re.findall(r'"([^"]+)"', m.group(2))


def plan_plugin_dirs(entries: list, name: str, to: str) -> list:
    """New pluginDirs set after moving NAME into area `to`.

    Moving to plugins: drop the component entry (the package is then discovered via
    the whole-tree `internal/plugins` entry). Moving to component: add the explicit
    `internal/component/<name>` entry (the whole-tree plugins entry no longer covers
    it). Result is sorted to match the existing alphabetical layout.
    """
    comp_entry = f"internal/component/{name}"
    s = set(entries)
    if to == "internal/plugins":
        s.discard(comp_entry)
    else:
        s.add(comp_entry)
    return sorted(s)


def render_plugin_dirs(text: str, new_entries: list) -> str:
    inner = "\n".join(f'\t"{e}",' for e in new_entries)
    return PLUGINDIRS_RE.sub(lambda m: m.group(1) + inner + m.group(3), text, count=1)


def edit_plugin_dirs(root: str, name: str, to: str, apply: bool) -> tuple:
    """Return (before, after, changed). Writes plugin_imports.go when apply=True."""
    path = os.path.join(root, GENERATOR)
    with open(path, encoding="utf-8") as fh:
        text = fh.read()
    before = parse_plugin_dirs(text)
    after = plan_plugin_dirs(before, name, to)
    changed = after != before
    if apply and changed:
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(render_plugin_dirs(text, after))
    return before, after, changed


# --------------------------------------------------------------------------- #
# hazard detection (cannot be fixed mechanically)
# --------------------------------------------------------------------------- #
def detect_rpc_packages(root: str, src_rel: str) -> list:
    """Repo-relative dirs under src_rel whose non-test code calls
    pluginserver.RegisterRPCs( -- these are discovered only when the tree is under
    the generator's rpcRoot (internal/component)."""
    found = []
    base = os.path.join(root, src_rel)
    for dirpath, dirs, files in os.walk(base):
        dirs[:] = [d for d in dirs if d not in SKIP_DIRS]
        for f in files:
            if not f.endswith(".go") or f.endswith("_test.go"):
                continue
            try:
                with open(os.path.join(dirpath, f), encoding="utf-8") as fh:
                    txt = fh.read()
            except OSError:
                continue
            if "RegisterRPCs(" in txt:
                found.append(os.path.relpath(dirpath, root))
                break
    return sorted(set(found))


def detect_residual_refs(root: str, module: str, src_rel: str) -> list:
    """(file, count) for references to the old path the import rewrite will NOT fix.

    Scans the whole repo (text surfaces in RESIDUAL_EXTS) for the repo-relative
    path. For .go files, quoted module imports are subtracted (the mover rewrites
    those), so only comment/string references remain. The generator's pluginDirs
    line and plan/ specs are skipped. This is broader than a fixed dir list so
    .ci/.yang/docs references are not missed (review finding ISSUE-2)."""
    quoted = f'"{module}/{src_rel}'  # start of a quoted module import (auto-fixed)
    hits = []
    for dirpath, dirs, files in os.walk(root):
        dirs[:] = [d for d in dirs if d not in SKIP_DIRS]
        for f in files:
            full = os.path.join(dirpath, f)
            rel = os.path.relpath(full, root)
            if rel.split(os.sep, 1)[0] in RESIDUAL_SKIP_TOP:
                continue
            if rel == GENERATOR:  # pluginDirs handled by edit_plugin_dirs
                continue
            if f != "Makefile" and not f.endswith(RESIDUAL_EXTS):
                continue
            try:
                with open(full, encoding="utf-8", errors="ignore") as fh:
                    txt = fh.read()
            except OSError:
                continue
            total = txt.count(src_rel)
            if not total:
                continue
            residual = total - txt.count(quoted) if f.endswith(".go") else total
            if residual > 0:
                hits.append((rel, residual))
    return sorted(hits)


def read_blank_imports(path: str) -> set:
    """The set of blank-imported packages in a generated all.go (the registration
    inventory). Used to prove a move drops nothing (review finding NOTE-5)."""
    try:
        with open(path, encoding="utf-8") as fh:
            txt = fh.read()
    except OSError:
        return set()
    return set(re.findall(r'_\s+"([^"]+)"', txt))


def norm_back(path: str, dst_rel: str, src_rel: str) -> str:
    """Map a moved import path back to its pre-move form, boundary-safe so a
    sibling like `<name>-cmd` is never rewritten."""
    return re.sub(re.escape(dst_rel) + r"(?![A-Za-z0-9-])", src_rel, path)


def run_goimports(root: str, module: str, plan: dict) -> None:
    """Re-sort import groups after the path rewrite. The move changes a package's
    alphabetical position (component < plugins), which `go build` tolerates but
    golangci-lint's goimports formatter rejects (review finding ISSUE-3)."""
    gi = shutil.which("goimports")
    if not gi:
        print(
            "  goimports NOT found -- imports are now mis-sorted; run manually:\n"
            f"    goimports -w -local {module} ./internal ./cmd ./pkg ./test"
        )
        return
    # the moved tree (new location) plus every external importer that was rewritten.
    # ALL_GO is excluded: it is generated code whose exact byte format the generator
    # owns (goimports would strip its trailing blank line and break --check).
    targets = [plan["dst_rel"]]
    targets += [
        f
        for f, _ in plan["import_hits"]
        if not f.startswith(plan["src_rel"] + "/") and f != ALL_GO
    ]
    targets = sorted(set(targets))
    print(f"  running: goimports -w -local {module} ({len(targets)} target(s))")
    subprocess.run([gi, "-w", "-local", module, *targets], cwd=root, check=False)


def run_generator(root: str) -> int:
    print(f"  running: go run {GENERATOR}")
    return subprocess.run(["go", "run", GENERATOR], cwd=root, check=False).returncode


# --------------------------------------------------------------------------- #
# plan + drive
# --------------------------------------------------------------------------- #
def build_plan(root: str, module: str, name: str, to: str | None) -> dict:
    src_area = find_source(root, name)
    dst_area = (
        to
        if to
        else (
            "internal/plugins"
            if src_area == "internal/component"
            else "internal/component"
        )
    )
    if dst_area not in AREAS:
        raise SystemExit(f"migrate_module: --to must be one of {AREAS}")
    if dst_area == src_area:
        raise SystemExit(
            f"migrate_module: '{name}' already in {src_area}/ (nothing to do)"
        )

    src_rel = f"{src_area}/{name}"
    dst_rel = f"{dst_area}/{name}"
    old_prefix = f"{module}/{src_rel}"
    new_prefix = f"{module}/{dst_rel}"

    import_hits = rewrite_go_imports(root, old_prefix, new_prefix, apply=False)
    before, after, pd_changed = edit_plugin_dirs(root, name, dst_area, apply=False)
    rpc_pkgs = detect_rpc_packages(root, src_rel)
    rpc_hazard = bool(rpc_pkgs) and dst_area == "internal/plugins"
    residual = detect_residual_refs(root, module, src_rel)

    return {
        "name": name,
        "src_rel": src_rel,
        "dst_rel": dst_rel,
        "old_prefix": old_prefix,
        "new_prefix": new_prefix,
        "import_hits": import_hits,
        "pd_before": before,
        "pd_after": after,
        "pd_changed": pd_changed,
        "rpc_pkgs": rpc_pkgs,
        "rpc_hazard": rpc_hazard,
        "residual": residual,
    }


def print_plan(plan: dict, apply: bool) -> None:
    head = "APPLYING" if apply else "DRY RUN (no changes) --"
    print(f"{head} move {plan['src_rel']}  ->  {plan['dst_rel']}\n")

    print(f"1. filesystem move: {plan['src_rel']}/  ->  {plan['dst_rel']}/")

    total = sum(c for _, c in plan["import_hits"])
    print(
        f"\n2. rewrite quoted import {plan['old_prefix']}\n"
        f"                     -> {plan['new_prefix']}\n"
        f"   {total} occurrence(s) in {len(plan['import_hits'])} file(s):"
    )
    for rel, c in plan["import_hits"]:
        print(f"     {c:3d}  {rel}")

    print(f"\n3. {GENERATOR} pluginDirs:")
    if plan["pd_changed"]:
        added = sorted(set(plan["pd_after"]) - set(plan["pd_before"]))
        removed = sorted(set(plan["pd_before"]) - set(plan["pd_after"]))
        for e in removed:
            print(f"     - {e}")
        for e in added:
            print(f"     + {e}")
    else:
        print("     (no change -- discovered via whole-tree scan or scanners)")

    print("\n4. regenerate internal/component/plugin/all/all.go via the generator")

    if plan["rpc_pkgs"]:
        flag = "BLOCKING" if plan["rpc_hazard"] else "ok (moving INTO component)"
        print(f"\nRPC packages in the moved tree [{flag}]:")
        for d in plan["rpc_pkgs"]:
            print(f"     {d}")
        if plan["rpc_hazard"]:
            print(
                "   The generator's rpcRoot scans only internal/component; these would\n"
                "   drop from all.go. Widen discoverRPCPackages to scan internal/plugins\n"
                "   FIRST, then re-run with --allow-rpc-drop to confirm."
            )

    if plan["residual"]:
        print("\nResidual references to the old path (review/handle manually):")
        for rel, c in plan["residual"]:
            print(f"     {c:3d}  {rel}")

    if not apply:
        print("\nNo files were changed. Re-run with --apply to execute.")


def apply_plan(root: str, module: str, plan: dict) -> int:
    src = os.path.join(root, plan["src_rel"])
    dst = os.path.join(root, plan["dst_rel"])
    if os.path.exists(dst):
        raise SystemExit(
            f"migrate_module: destination already exists: {plan['dst_rel']}"
        )
    all_go = os.path.join(root, ALL_GO)
    before = read_blank_imports(all_go)

    shutil.move(src, dst)
    rewrite_go_imports(root, plan["old_prefix"], plan["new_prefix"], apply=True)
    edit_plugin_dirs(root, plan["name"], os.path.dirname(plan["dst_rel"]), apply=True)
    rc = run_generator(root)
    run_goimports(root, module, plan)

    # Prove the registration inventory is preserved (NOTE-5): normalise the new
    # all.go back to old paths and diff against the pre-move set.
    after = {
        norm_back(p, plan["dst_rel"], plan["src_rel"])
        for p in read_blank_imports(all_go)
    }
    dropped = sorted(before - after)
    added = sorted(after - before)
    if dropped:
        print(
            "\nWARNING: all.go DROPPED registrations (investigate before commit!):",
            file=sys.stderr,
        )
        for p in dropped:
            print(f"  - {p}", file=sys.stderr)
    if added:
        print("\nall.go gained registrations (new discovery, usually benign):")
        for p in added:
            print(f"  + {p}")
    if not dropped and not added:
        print("\nall.go registration set preserved (0 changed).")

    print(
        "\nApplied. NEXT (manual): go build ./...  |  diff bin/ze --plugins  |  "
        "scripts/dev/dep_audit.py --check  |  then a user-run commit script."
    )
    if rc != 0:
        return rc
    return 4 if dropped else 0


# --------------------------------------------------------------------------- #
# selftest (isolated fixtures, no repo mutation, no go toolchain)
# --------------------------------------------------------------------------- #
def _w(root: str, rel: str, content: str) -> None:
    p = os.path.join(root, rel)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, "w", encoding="utf-8") as fh:
        fh.write(content)


def selftest() -> int:
    import tempfile

    mod = "example.com/m"
    with tempfile.TemporaryDirectory() as root:
        _w(root, "go.mod", f"module {mod}\n")
        _w(
            root,
            GENERATOR,
            "var pluginDirs = []string{\n"
            '\t"internal/component/edgeproto",\n'
            '\t"internal/component/iface",\n'
            '\t"internal/plugins",\n}\n'
            'const rpcRoot = "internal/component"\n',
        )
        # edge engine to move OUT of component; imports its own subpackage.
        _w(
            root,
            "internal/component/edgeproto/register.go",
            f'package edgeproto\nimport _ "{mod}/internal/component/edgeproto/sub"\n'
            "func R(){ sdk.NewWithConn() }\n",
        )
        _w(root, "internal/component/edgeproto/sub/s.go", "package sub\n")
        # an RPC package inside the moved tree -> rpc hazard on move-out.
        _w(
            root,
            "internal/component/edgeproto/cmd/c.go",
            "package cmd\nfunc f(){ pluginserver.RegisterRPCs() }\n",
        )
        # external importer (must be rewritten).
        _w(
            root,
            "internal/plugins/consumer/c.go",
            f'package consumer\nimport _ "{mod}/internal/component/edgeproto"\n',
        )
        # boundary trap: edgeproto2 must NOT be rewritten.
        _w(
            root,
            "internal/component/edgeproto2/x.go",
            f'package edgeproto2\nimport _ "{mod}/internal/component/edgeproto2/y"\n',
        )
        # residual refs the import rewrite does NOT fix (ISSUE-2): script, .ci,
        # doc, and a .go *comment* (quoted imports are auto-fixed, comments are not).
        _w(root, "scripts/foo.py", "# uses internal/component/edgeproto here\n")
        _w(root, "test/edge/edge.ci", "# exercises internal/component/edgeproto\n")
        _w(root, "docs/edge.md", "see internal/component/edgeproto/sub\n")
        _w(
            root,
            "internal/component/note.go",
            "package note\n// ref: internal/component/edgeproto in a comment\n",
        )
        # a plan/ spec referencing the old path -- MUST be skipped (other-owned).
        _w(root, "plan/spec-other.md", "mirror internal/component/edgeproto\n")
        # a generated all.go to exercise read_blank_imports (NOTE-5).
        _w(
            root,
            ALL_GO,
            'package all\nimport (\n\t_ "a/b"\n\t_ "c/d"\n)\n',
        )
        # a platform plugin to move INTO component.
        _w(
            root,
            "internal/plugins/platform/register.go",
            "package platform\nfunc R(){ sdk.NewWithConn() }\n",
        )

        # --- dry-run plan for the move-out, assert no disk mutation ---
        plan = build_plan(root, mod, "edgeproto", "internal/plugins")
        assert os.path.isdir(os.path.join(root, "internal/component/edgeproto")), (
            "dry-run must not move"
        )
        names = {f for f, _ in plan["import_hits"]}
        assert "internal/plugins/consumer/c.go" in names, "external importer not seen"
        assert "internal/component/edgeproto/register.go" in names, (
            "self-import not seen"
        )
        assert all("edgeproto2" not in f for f in names), "boundary: edgeproto2 leaked"
        assert plan["rpc_hazard"], "rpc hazard not detected on move-out"
        assert plan["rpc_pkgs"] == ["internal/component/edgeproto/cmd"], plan[
            "rpc_pkgs"
        ]
        assert "internal/component/edgeproto" not in plan["pd_after"], "pd not dropped"
        res = {r for r, _ in plan["residual"]}
        assert "scripts/foo.py" in res, "residual ref missed"
        assert "test/edge/edge.ci" in res, "ISSUE-2: .ci residual missed"
        assert "docs/edge.md" in res, "ISSUE-2: doc residual missed"
        assert "internal/component/note.go" in res, (
            "ISSUE-2: .go comment residual missed"
        )
        assert "internal/plugins/consumer/c.go" not in res, (
            "quoted import wrongly reported as residual"
        )
        assert not any(r.startswith("plan/") for r in res), "plan/ must be skipped"
        # NOTE-5 helpers: read_blank_imports parses all.go; norm_back is boundary-safe.
        assert read_blank_imports(os.path.join(root, ALL_GO)) == {"a/b", "c/d"}, (
            "read_blank_imports failed"
        )
        assert (
            norm_back(
                f"{mod}/internal/plugins/edgeproto/x",
                "internal/plugins/edgeproto",
                "internal/component/edgeproto",
            )
            == f"{mod}/internal/component/edgeproto/x"
        ), "norm_back failed"
        assert (
            norm_back(
                f"{mod}/internal/plugins/edgeproto-cmd",
                "internal/plugins/edgeproto",
                "internal/component/edgeproto",
            )
            == f"{mod}/internal/plugins/edgeproto-cmd"
        ), "norm_back boundary: -cmd sibling corrupted"

        # --- apply the move-out (skip the generator subprocess) ---
        shutil.move(
            os.path.join(root, "internal/component/edgeproto"),
            os.path.join(root, "internal/plugins/edgeproto"),
        )
        rewrite_go_imports(root, plan["old_prefix"], plan["new_prefix"], apply=True)
        edit_plugin_dirs(root, "edgeproto", "internal/plugins", apply=True)

        assert not os.path.exists(os.path.join(root, "internal/component/edgeproto"))
        assert os.path.isdir(os.path.join(root, "internal/plugins/edgeproto"))
        with open(os.path.join(root, "internal/plugins/consumer/c.go")) as fh:
            assert f"{mod}/internal/plugins/edgeproto" in fh.read(), (
                "consumer not rewritten"
            )
        with open(os.path.join(root, "internal/plugins/edgeproto/register.go")) as fh:
            assert f"{mod}/internal/plugins/edgeproto/sub" in fh.read(), (
                "self not rewritten"
            )
        with open(os.path.join(root, "internal/component/edgeproto2/x.go")) as fh:
            assert "internal/component/edgeproto2/y" in fh.read(), "edgeproto2 mangled"
        with open(os.path.join(root, GENERATOR)) as fh:
            pd = parse_plugin_dirs(fh.read())
        assert "internal/component/edgeproto" not in pd, "pd entry not removed"

        # --- move INTO component: pluginDirs gains the explicit entry ---
        plan2 = build_plan(root, mod, "platform", "internal/component")
        assert not plan2["rpc_hazard"], "move-in must not flag rpc hazard"
        assert "internal/component/platform" in plan2["pd_after"], "pd entry not added"

    print("migrate_module selftest OK")
    return 0


def main() -> int:
    ap = argparse.ArgumentParser(
        add_help=True, description="module-tier migration tool"
    )
    ap.add_argument(
        "name", nargs="?", help="subsystem directory name (e.g. flowexport)"
    )
    ap.add_argument(
        "--to",
        choices=("plugins", "component"),
        help="target tier (default: the other area from where NAME currently lives)",
    )
    ap.add_argument("--apply", action="store_true", help="execute (default: dry-run)")
    ap.add_argument(
        "--allow-rpc-drop",
        action="store_true",
        help="proceed past the RPC-drop guard (only after widening the generator)",
    )
    ap.add_argument(
        "--selftest", action="store_true", help="run isolated fixture tests"
    )
    args = ap.parse_args()

    if args.selftest:
        return selftest()
    if not args.name:
        ap.error("NAME is required (or use --selftest)")

    root = repo_root()
    module = module_path(root)
    to = None if args.to is None else f"internal/{args.to}"
    plan = build_plan(root, module, args.name, to)

    print_plan(plan, args.apply)

    if not args.apply:
        return 0
    if plan["rpc_hazard"] and not args.allow_rpc_drop:
        print(
            "\nREFUSED: moving RPC packages out of internal/component would drop them "
            "from all.go.\nWiden the generator's discoverRPCPackages to scan "
            "internal/plugins, then re-run with --allow-rpc-drop.",
            file=sys.stderr,
        )
        return 3
    print()
    return apply_plan(root, module, plan)


if __name__ == "__main__":
    sys.exit(main())
