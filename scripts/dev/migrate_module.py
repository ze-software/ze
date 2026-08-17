#!/usr/bin/env python3
"""Deterministic module-tier migration tool (see ai/rules/architecture.md).

Moves a package directory from one repo-relative internal/... path to another.
Bare names keep the original top-level component<->plugins behavior; explicit
source or destination paths also support core and nested domain moves. The tool
performs every mechanical edit a behaviour-preserving relocation needs, and
REFUSES the moves it cannot make safe mechanically.

What it does (in --apply):
  1. filesystem move of the directory (shutil.move; NEVER `git mv` -- git add/rm/mv
     are forbidden from tooling, the user runs the commit script afterwards). When
     the destination directory already exists and no file path collides (a MERGE,
     e.g. an engine joining its `cmd/` subpackage that already occupies the target
     name), the source's files are merged into it; a real file collision is refused.
  2. repo-wide rewrite of quoted, module-qualified Go import paths, boundary-safe
     (only the exact package and its subpackages; `<name>2` is never touched).
  3. edit of the generator's `pluginDirs` literal in scripts/codegen/plugin_imports.go
     when a moved tree is, or was, generator-discovered.
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
    the destination tier). This is the load-bearing precondition for out-of-component moves.
  - residual references the import rewrite cannot fix: non-Go surfaces
    (.mk/.sh/.ci/.yang/docs/rfc) and .go comments. plan/ specs are skipped
    (owned by other work). These are reported for manual follow-up.

This tool performs NO git operations. After --apply, verify with `CGO_ENABLED=0 go build ./...`,
diff `bin/ze --plugins`, run `scripts/dev/dep_audit.py --check`, then commit via a
user-run script.

Usage:
    scripts/dev/migrate_module.py NAME [--to {plugins,component,core}]  # dry-run
    scripts/dev/migrate_module.py audit --to internal/core/audit         # core move
    scripts/dev/migrate_module.py ppp --to internal/component/l2tp/ppp   # nested move
    scripts/dev/migrate_module.py internal/component/bgp/wire --to internal/core/bgp/wire
    scripts/dev/migrate_module.py NAME --to plugins --apply              # execute
    scripts/dev/migrate_module.py --selftest                             # fixtures

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
SOURCE_AREAS = ("internal/component", "internal/plugins", "internal/core")
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


def _clean_rel_path(path: str, label: str) -> str:
    """Return a normalised repo-relative path, refusing absolute or parent escapes."""
    p = os.path.normpath(path.strip().rstrip("/"))
    if p == "." or os.path.isabs(p) or p == ".." or p.startswith("../"):
        raise SystemExit(f"migrate_module: {label} must be repo-relative: {path!r}")
    return p


def _source_is_path(name: str) -> bool:
    return name.startswith("internal/") or "/" in name


def _legacy_target_area(to: str | None) -> str | None:
    if to in ("component", "internal/component"):
        return "internal/component"
    if to in ("plugins", "internal/plugins"):
        return "internal/plugins"
    if to in ("core", "internal/core"):
        return "internal/core"
    return None


def find_source(root: str, name: str, to: str | None = None) -> str:
    """Return the repo-relative source directory for NAME.

    NAME may be a legacy top-level subsystem name or an explicit repo-relative
    source path. Bare-name lookup searches the top-level migration areas. If NAME
    exists in both component and plugins, only the legacy --to component/plugins
    form disambiguates it by selecting the other area as the source.
    """
    if _source_is_path(name):
        src_rel = _clean_rel_path(name, "source")
        if not src_rel.startswith("internal/"):
            raise SystemExit(
                "migrate_module: source paths must start with internal/: " + src_rel
            )
        if not os.path.isdir(os.path.join(root, src_rel)):
            raise SystemExit(f"migrate_module: source not found: {src_rel}/")
        return src_rel

    here = [
        f"{a}/{name}"
        for a in SOURCE_AREAS
        if os.path.isdir(os.path.join(root, a, name))
    ]
    if len(here) == 1:
        return here[0]
    if not here:
        raise SystemExit(
            f"migrate_module: '{name}' not found under "
            + " or ".join(f"{a}/" for a in SOURCE_AREAS)
        )
    if to in SOURCE_AREAS:  # source is whichever top-level area is not the destination
        dst = f"{to}/{name}"
        candidates = [h for h in here if h != dst]
        if len(candidates) == 1:
            return candidates[0]
    raise SystemExit(
        f"migrate_module: '{name}' exists in multiple top-level areas "
        f"({', '.join(here)}); pass an explicit repo-relative source path"
    )

def parse_destination(src_rel: str, to: str | None) -> str:
    """Return the repo-relative destination directory for a source path."""
    leaf = os.path.basename(src_rel)
    if to is None:
        if src_rel == f"internal/component/{leaf}":
            return f"internal/plugins/{leaf}"
        if src_rel == f"internal/plugins/{leaf}":
            return f"internal/component/{leaf}"
        raise SystemExit(
            "migrate_module: --to is required for core or nested source paths"
        )

    raw = to.strip().rstrip("/")
    tier_aliases = {
        "component": "internal/component",
        "plugins": "internal/plugins",
        "core": "internal/core",
    }
    if raw in tier_aliases:
        return f"{tier_aliases[raw]}/{leaf}"
    if raw in SOURCE_AREAS:
        return f"{raw}/{leaf}"
    for short, full in tier_aliases.items():
        if raw.startswith(short + "/"):
            raw = full + raw[len(short) :]
            break

    dst_rel = _clean_rel_path(raw, "--to")
    if not any(dst_rel == a or dst_rel.startswith(a + "/") for a in SOURCE_AREAS):
        raise SystemExit(
            "migrate_module: --to must be plugins, component, core, or a "
            "repo-relative internal/{component,plugins,core}/... path"
        )
    if dst_rel in SOURCE_AREAS:
        return f"{dst_rel}/{leaf}"
    return dst_rel


def merge_conflicts(src_abs: str, dst_abs: str) -> list:
    """Relative file paths present in BOTH trees -- a real merge conflict the mover
    cannot resolve. Empty means every source file slots into the destination tree."""
    conflicts = []
    for dirpath, dirs, files in os.walk(src_abs):
        dirs[:] = [d for d in dirs if d not in SKIP_DIRS]
        rel_dir = os.path.relpath(dirpath, src_abs)
        for f in files:
            rel = f if rel_dir == "." else os.path.join(rel_dir, f)
            if os.path.exists(os.path.join(dst_abs, rel)):
                conflicts.append(rel)
    return sorted(conflicts)


def do_merge(src_abs: str, dst_abs: str) -> None:
    """Move every file from src into dst (creating directories), then remove the
    now-empty source tree. Callers MUST have verified merge_conflicts() is empty."""
    for dirpath, dirs, files in os.walk(src_abs):
        dirs[:] = [d for d in dirs if d not in SKIP_DIRS]
        rel_dir = os.path.relpath(dirpath, src_abs)
        for f in files:
            s = os.path.join(dirpath, f)
            rel = f if rel_dir == "." else os.path.join(rel_dir, f)
            d = os.path.join(dst_abs, rel)
            os.makedirs(os.path.dirname(d), exist_ok=True)
            shutil.move(s, d)
    for dirpath, _dirs, files in os.walk(src_abs):  # only empty dirs may remain
        if files:
            raise SystemExit(f"do_merge: file left behind under {dirpath}: {files}")
    shutil.rmtree(src_abs)


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
NESTED_DOMAINS_RE = re.compile(
    r"(var nestedPluginDomains = \[\]string\{\n)(.*?)(\n\})", re.DOTALL
)


def parse_plugin_dirs(text: str) -> list:
    m = PLUGINDIRS_RE.search(text)
    if not m:
        raise SystemExit(f"{GENERATOR}: could not locate pluginDirs literal")
    return re.findall(r'"([^"]+)"', m.group(2))


def parse_nested_plugin_domains(text: str) -> list:
    m = NESTED_DOMAINS_RE.search(text)
    if not m:
        return []
    return re.findall(r'"([^"]+)"', m.group(2))


def nested_plugin_dirs(domains: list) -> list:
    return [f"internal/component/{domain}/plugins" for domain in domains]


def effective_plugin_dirs(entries: list, nested_domains: list) -> list:
    seen = set()
    roots = []
    for rel in [*entries, *nested_plugin_dirs(nested_domains)]:
        if rel in seen:
            continue
        seen.add(rel)
        roots.append(rel)
    return roots


def _covers(entry: str, path: str) -> bool:
    return path == entry or path.startswith(entry + "/")


def _is_generator_discoverable(path: str) -> bool:
    return path.startswith("internal/component/") or path.startswith("internal/plugins/")


def plan_plugin_dirs(
    entries: list, src_rel: str, dst_rel: str, nested_domains: list | None = None
) -> list:
    """New literal pluginDirs set after moving src_rel to dst_rel.

    The generator discovers all of internal/plugins, literal pluginDirs entries,
    and nested roots derived from nestedPluginDomains. Use the effective roots for
    coverage decisions, but render edits only into the literal pluginDirs list.
    Exact source entries move or disappear; ancestor entries remain because they
    may cover sibling packages.
    """
    if nested_domains is None:
        nested_domains = []
    literal = set(entries)
    before_effective = effective_plugin_dirs(entries, nested_domains)
    source_was_discovered = any(_covers(e, src_rel) for e in before_effective)
    literal.discard(src_rel)
    after_effective = effective_plugin_dirs(sorted(literal), nested_domains)
    if (
        source_was_discovered
        and _is_generator_discoverable(dst_rel)
        and not any(_covers(e, dst_rel) for e in after_effective)
    ):
        literal.add(dst_rel)
    return sorted(literal)


def render_plugin_dirs(text: str, new_entries: list) -> str:
    inner = "\n".join(f'\t"{e}",' for e in new_entries)
    return PLUGINDIRS_RE.sub(lambda m: m.group(1) + inner + m.group(3), text, count=1)


def edit_plugin_dirs(root: str, src_rel: str, dst_rel: str, apply: bool) -> tuple:
    """Return (before, after, changed). Writes plugin_imports.go when apply=True."""
    path = os.path.join(root, GENERATOR)
    with open(path, encoding="utf-8") as fh:
        text = fh.read()
    before = parse_plugin_dirs(text)
    nested_domains = parse_nested_plugin_domains(text)
    after = plan_plugin_dirs(before, src_rel, dst_rel, nested_domains)
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


def _remap(path: str, frm: str, to: str) -> str:
    """Boundary-safe path-prefix remap: rewrite FRM->TO only at a package boundary,
    so a sibling like `<name>-cmd` or `<name>2` is never touched."""
    return re.sub(re.escape(frm) + r"(?![A-Za-z0-9-])", to, path)


def norm_back(path: str, dst_rel: str, src_rel: str) -> str:
    """Map a moved import path back to its pre-move form (dst->src)."""
    return _remap(path, dst_rel, src_rel)


def norm_fwd(path: str, src_rel: str, dst_rel: str) -> str:
    """Map a pre-move import path to its post-move form (src->dst). Used by the apply
    set-diff: normalising the BEFORE set forward (rather than the AFTER set backward)
    leaves pre-existing destination paths -- e.g. a `cmd/` subpackage already at the
    merge target -- untouched, so a merge reports no false drop."""
    return _remap(path, src_rel, dst_rel)


def run_goimports(root: str, module: str, plan: dict) -> None:
    """Re-sort import groups after the path rewrite. The move changes a package's
    position in goimports' module-local ordering, which `go build` tolerates but
    golangci-lint's goimports check enforces."""
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
    print(f"  running: CGO_ENABLED=0 go run {GENERATOR}")
    env = os.environ.copy()
    env["CGO_ENABLED"] = "0"
    return subprocess.run(
        ["go", "run", GENERATOR], cwd=root, check=False, env=env
    ).returncode


# --------------------------------------------------------------------------- #
# plan + drive
# --------------------------------------------------------------------------- #
def build_plan(root: str, module: str, name: str, to: str | None) -> dict:
    src_rel = find_source(root, name, _legacy_target_area(to))
    dst_rel = parse_destination(src_rel, to)
    if dst_rel == src_rel:
        raise SystemExit(
            f"migrate_module: source and destination are both {src_rel}/ (nothing to do)"
        )
    if dst_rel.startswith(src_rel + "/"):
        raise SystemExit(
            f"migrate_module: destination is inside the source tree: {dst_rel}/"
        )

    old_prefix = f"{module}/{src_rel}"
    new_prefix = f"{module}/{dst_rel}"

    import_hits = rewrite_go_imports(root, old_prefix, new_prefix, apply=False)
    before, after, pd_changed = edit_plugin_dirs(root, src_rel, dst_rel, apply=False)
    rpc_pkgs = detect_rpc_packages(root, src_rel)
    rpc_hazard = bool(rpc_pkgs) and src_rel.startswith(
        "internal/component/"
    ) and not dst_rel.startswith("internal/component/")
    residual = detect_residual_refs(root, module, src_rel)
    dst_abs = os.path.join(root, dst_rel)
    is_merge = os.path.isdir(dst_abs)
    conflicts = (
        merge_conflicts(os.path.join(root, src_rel), dst_abs) if is_merge else []
    )

    return {
        "name": os.path.basename(src_rel),
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
        "is_merge": is_merge,
        "conflicts": conflicts,
    }


def print_plan(plan: dict, apply: bool) -> None:
    head = "APPLYING" if apply else "DRY RUN (no changes) --"
    print(f"{head} move {plan['src_rel']}  ->  {plan['dst_rel']}\n")

    if plan["is_merge"]:
        print(
            f"1. filesystem MERGE: {plan['src_rel']}/  ->  {plan['dst_rel']}/ "
            "(destination already exists)"
        )
        if plan["conflicts"]:
            print("   REFUSED: these paths exist in BOTH trees (real conflict):")
            for c in plan["conflicts"]:
                print(f"     ! {c}")
        else:
            print("   no file-level conflict; source files merge into the tree")
    else:
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
        print(
            "     (no change -- destination already covered or not generator-discovered)"
        )

    print("\n4. regenerate internal/component/plugin/all/all.go via the generator")
    print("5. verify all.go registration set is preserved by a forward-normalized diff")

    if plan["rpc_pkgs"]:
        flag = "BLOCKING" if plan["rpc_hazard"] else "ok (stays under component)"
        print(f"\nRPC packages in the moved tree [{flag}]:")
        for d in plan["rpc_pkgs"]:
            print(f"     {d}")
        if plan["rpc_hazard"]:
            print(
                "   The generator's rpcRoot scans only internal/component; these would\n"
                "   drop from all.go. Widen discoverRPCPackages to scan the destination\n"
                "   tier FIRST, then re-run with --allow-rpc-drop to confirm."
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
    if plan["is_merge"]:
        if plan["conflicts"]:
            raise SystemExit(
                "migrate_module: cannot merge -- paths exist in both trees: "
                + ", ".join(plan["conflicts"])
            )
    elif os.path.exists(dst):
        raise SystemExit(
            f"migrate_module: destination already exists: {plan['dst_rel']}"
        )
    all_go = os.path.join(root, ALL_GO)
    before = read_blank_imports(all_go)

    os.makedirs(os.path.dirname(dst), exist_ok=True)
    if plan["is_merge"]:
        do_merge(src, dst)
    else:
        shutil.move(src, dst)
    rewrite_go_imports(root, plan["old_prefix"], plan["new_prefix"], apply=True)
    edit_plugin_dirs(root, plan["src_rel"], plan["dst_rel"], apply=True)
    rc = run_generator(root)
    run_goimports(root, module, plan)

    # Prove the registration inventory is preserved (NOTE-5): normalise the BEFORE
    # set FORWARD (src->dst, boundary-safe) and diff against the raw after set.
    # Forward beats backward for a merge: pre-existing destination paths (an already
    # present cmd/ subpackage) stay put under forward normalisation, so they are not
    # reported as a spurious drop.
    after = read_blank_imports(all_go)
    before_n = {norm_fwd(p, plan["src_rel"], plan["dst_rel"]) for p in before}
    dropped = sorted(before_n - after)
    added = sorted(after - before_n)
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
        "\nApplied. NEXT (manual): CGO_ENABLED=0 go build ./...  |  diff bin/ze --plugins  |  "
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
            '\t"internal/component/ipsec",\n'
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
        edit_plugin_dirs(
            root,
            "internal/component/edgeproto",
            "internal/plugins/edgeproto",
            apply=True,
        )

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


        # --- core destination: a component library moves to internal/core/... ---
        _w(root, "internal/component/audit/a.go", "package audit\n")
        _w(root, "internal/component/audit/sub/s.go", "package sub\n")
        _w(
            root,
            "internal/component/coreconsumer/c.go",
            f'package coreconsumer\nimport _ "{mod}/internal/component/audit/sub"\n',
        )
        _w(root, "docs/audit.md", "see internal/component/audit/sub\n")
        cplan = build_plan(root, mod, "internal/component/audit", "internal/core/audit")
        assert cplan["src_rel"] == "internal/component/audit", cplan["src_rel"]
        assert cplan["dst_rel"] == "internal/core/audit", cplan["dst_rel"]
        assert cplan["new_prefix"] == f"{mod}/internal/core/audit", cplan[
            "new_prefix"
        ]
        assert "internal/core/audit" not in cplan["pd_after"], (
            "core move must not add pluginDirs"
        )
        assert ("docs/audit.md", 1) in cplan["residual"], "core residual missed"
        os.makedirs(os.path.join(root, "internal/core"), exist_ok=True)
        shutil.move(
            os.path.join(root, "internal/component/audit"),
            os.path.join(root, "internal/core/audit"),
        )
        rewrite_go_imports(root, cplan["old_prefix"], cplan["new_prefix"], apply=True)
        with open(os.path.join(root, "internal/component/coreconsumer/c.go")) as fh:
            assert f"{mod}/internal/core/audit/sub" in fh.read(), (
                "core importer not rewritten"
            )

        # --- nested component-domain destination with generator edit planning ---
        _w(
            root,
            "internal/component/ipsec/register.go",
            f'package ipsec\nimport _ "{mod}/internal/component/ipsec/child"\n',
        )
        _w(root, "internal/component/ipsec/child/c.go", "package child\n")
        _w(
            root,
            "internal/plugins/vpnconsumer/c.go",
            f'package vpnconsumer\nimport _ "{mod}/internal/component/ipsec"\n',
        )
        _w(root, "docs/vpn.md", "see internal/component/ipsec/child\n")
        nplan = build_plan(root, mod, "internal/component/ipsec", "internal/component/ike/ipsec")
        assert nplan["src_rel"] == "internal/component/ipsec", nplan["src_rel"]
        assert nplan["dst_rel"] == "internal/component/ike/ipsec", nplan["dst_rel"]
        assert not nplan["rpc_hazard"], "nested component move must keep RPC coverage"
        assert "internal/component/ipsec" not in nplan["pd_after"], (
            "old pluginDirs entry not removed"
        )
        assert "internal/component/ike/ipsec" in nplan["pd_after"], (
            "nested pluginDirs entry not added"
        )
        assert ("docs/vpn.md", 1) in nplan["residual"], "nested residual missed"
        import io
        import contextlib

        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            print_plan(nplan, apply=False)
        dry = out.getvalue()
        assert "rewrite quoted import" in dry, "nested dry-run lacks import rewrite"
        assert "+ internal/component/ike/ipsec" in dry, (
            "nested dry-run lacks generator edit"
        )
        assert "Residual references to the old path" in dry, (
            "nested dry-run lacks residual references"
        )
        assert "verify all.go registration set is preserved" in dry, (
            "nested dry-run lacks registration preservation"
        )
        os.makedirs(os.path.join(root, "internal/component/ike"), exist_ok=True)
        shutil.move(
            os.path.join(root, "internal/component/ipsec"),
            os.path.join(root, "internal/component/ike/ipsec"),
        )
        rewrite_go_imports(root, nplan["old_prefix"], nplan["new_prefix"], apply=True)
        edit_plugin_dirs(
            root,
            "internal/component/ipsec",
            "internal/component/ike/ipsec",
            apply=True,
        )
        with open(os.path.join(root, "internal/plugins/vpnconsumer/c.go")) as fh:
            assert f"{mod}/internal/component/ike/ipsec" in fh.read(), (
                "nested importer not rewritten"
            )
        with open(
            os.path.join(root, "internal/component/ike/ipsec/register.go")
        ) as fh:
            assert f"{mod}/internal/component/ike/ipsec/child" in fh.read(), (
                "nested self-import not rewritten"
            )
        with open(os.path.join(root, GENERATOR)) as fh:
            pd = parse_plugin_dirs(fh.read())
        assert "internal/component/ipsec" not in pd, "old nested pd entry remains"
        assert "internal/component/ike/ipsec" in pd, "new nested pd entry missing"
        # --- move INTO component: pluginDirs gains the explicit entry ---
        plan2 = build_plan(root, mod, "platform", "internal/component")
        assert not plan2["rpc_hazard"], "move-in must not flag rpc hazard"
        assert "internal/component/platform" in plan2["pd_after"], "pd entry not added"

        # --- MERGE: an engine in plugins joins a cmd/ already at the component name ---
        _w(root, "internal/plugins/widget/w.go", "package widget\n")
        _w(root, "internal/plugins/widget/yang/y.go", "package yang\n")
        _w(root, "internal/component/widget/cmd/c.go", "package cmd\n")
        wplan = build_plan(root, mod, "widget", "internal/component")
        assert wplan["src_rel"] == "internal/plugins/widget", wplan["src_rel"]
        assert wplan["is_merge"], "merge not detected (dst dir exists)"
        assert wplan["conflicts"] == [], wplan["conflicts"]
        assert "internal/component/widget" in wplan["pd_after"], "pd entry not added"
        # forward-normalise: pre-existing cmd/ path untouched, moved subtree mapped.
        assert (
            norm_fwd(
                f"{mod}/internal/component/widget/cmd",
                "internal/plugins/widget",
                "internal/component/widget",
            )
            == f"{mod}/internal/component/widget/cmd"
        ), "norm_fwd corrupted a pre-existing destination path"
        assert (
            norm_fwd(
                f"{mod}/internal/plugins/widget/yang",
                "internal/plugins/widget",
                "internal/component/widget",
            )
            == f"{mod}/internal/component/widget/yang"
        ), "norm_fwd failed to map the moved subtree"
        do_merge(
            os.path.join(root, "internal/plugins/widget"),
            os.path.join(root, "internal/component/widget"),
        )
        assert os.path.isfile(os.path.join(root, "internal/component/widget/w.go"))
        assert os.path.isfile(os.path.join(root, "internal/component/widget/yang/y.go"))
        assert os.path.isfile(
            os.path.join(root, "internal/component/widget/cmd/c.go")
        ), "pre-existing cmd/ lost in merge"
        assert not os.path.exists(os.path.join(root, "internal/plugins/widget")), (
            "source tree not removed after merge"
        )

        # --- MERGE conflict: the same relative file in both trees is refused ---
        _w(root, "internal/plugins/gadget/dup.go", "package gadget\n")
        _w(root, "internal/component/gadget/dup.go", "package gadget\n")
        gplan = build_plan(root, mod, "gadget", "internal/component")
        assert gplan["is_merge"], "gadget merge not detected"
        assert gplan["conflicts"] == ["dup.go"], gplan["conflicts"]

    print("migrate_module selftest OK")
    return 0


def main() -> int:
    ap = argparse.ArgumentParser(
        add_help=True, description="module-tier migration tool"
    )
    ap.add_argument(
        "name",
        nargs="?",
        help="top-level subsystem name or repo-relative source directory",
    )
    ap.add_argument(
        "--to",
        metavar="DEST",
        help=(
            "target tier (plugins, component, core) or repo-relative destination "
            "under internal/{component,plugins,core}"
        ),
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
    to = args.to
    plan = build_plan(root, module, args.name, to)

    print_plan(plan, args.apply)

    if not args.apply:
        return 0
    if plan["rpc_hazard"] and not args.allow_rpc_drop:
        print(
            "\nREFUSED: moving RPC packages out of internal/component would drop them "
            "from all.go.\nWiden the generator's discoverRPCPackages to scan the "
            "destination tier, then re-run with --allow-rpc-drop.",
            file=sys.stderr,
        )
        return 3
    print()
    return apply_plan(root, module, plan)


if __name__ == "__main__":
    sys.exit(main())
