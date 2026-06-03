#!/usr/bin/env python3
"""Run changed-file-aware wiring, documentation, command, and inventory gates.

The script is intentionally a router. It keeps the expensive checks in their
existing direct targets, and only decides which ones apply to the current diff.
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Callable, Iterable


@dataclass(frozen=True)
class Symbol:
    path: str
    line: int
    kind: str
    name: str


TARGET_ORDER = (
    "wiring",
    "ze-validate-commands",
    "ze-command-ownership-check",
    "ze-doc-test",
    "ze-doc-check-stale",
    "ze-inventory-json",
    "ze-command-list-json",
    "ze-plugin-imports-check",
)

MAKE_TARGETS = {
    "ze-validate-commands",
    "ze-command-ownership-check",
    "ze-doc-test",
    "ze-doc-check-stale",
    "ze-inventory-json",
    "ze-command-list-json",
    "ze-plugin-imports-check",
}

# Reviewed exceptions for exported symbols that are deliberately API surface.
# Keep this small. A new entry here should name the path and symbol exactly.
WIRING_ALLOWLIST: set[tuple[str, str]] = set()

FUNC_RE = re.compile(r"^func\s+(?:\([^)]*\)\s*)?([A-Z][A-Za-z0-9_]*)\s*\(")
TYPE_RE = re.compile(r"^type\s+([A-Z][A-Za-z0-9_]*)\b")
CONST_RE = re.compile(r"^const\s+(.+)$")
VAR_RE = re.compile(r"^var\s+(.+)$")
BLOCK_RE = re.compile(r"^(type|const|var)\s*\(")
IDENT_LIST_RE = re.compile(
    r"^([A-Za-z_][A-Za-z0-9_]*(?:\s*,\s*[A-Za-z_][A-Za-z0-9_]*)*)\b"
)
TOKEN_TEMPLATE = r"\b{}\b"


class GateFailure(Exception):
    pass


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", default=".", help="repository root, defaults to cwd")
    parser.add_argument(
        "--changed-file",
        action="append",
        default=[],
        help="changed file to evaluate; repeatable",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="print selected gates instead of running them",
    )
    parser.add_argument(
        "--make", default="make", help="make executable used for delegated targets"
    )
    parser.add_argument(
        "--check-plugin-imports",
        action="store_true",
        help="verify internal/component/plugin/all/all.go without rewriting it",
    )
    args = parser.parse_args()

    root = find_repo_root(Path(args.root).resolve())
    if args.check_plugin_imports:
        check_plugin_imports(root)
        return 0

    changed = [normalize_changed_path(root, p) for p in args.changed_file]

    if not changed:
        changed = changed_files(root)

    targets = selected_targets(root, changed)
    if args.dry_run:
        if targets:
            print("\n".join(targets))
        else:
            print("No wiring/doc/inventory checks needed")
        return 0

    if not targets:
        print("No wiring/doc/inventory checks needed")
        return 0

    if "wiring" in targets:
        issues = check_wiring(root, changed)
        if issues:
            print("Wiring check FAILED:")
            for issue in issues:
                print("  - " + issue)
            return 1
        print("Wiring check PASSED")

    for target in targets:
        if target == "wiring":
            continue
        run_make_target(args.make, target, root)

    print("Wiring/doc/inventory gates passed")
    return 0


def find_repo_root(start: Path) -> Path:
    cur = start
    while True:
        if (cur / "go.mod").exists():
            return cur
        if cur.parent == cur:
            raise SystemExit(f"could not find go.mod above {start}")
        cur = cur.parent


def normalize_changed_path(root: Path, path: str) -> str:
    p = Path(path)
    if p.is_absolute():
        try:
            return p.resolve().relative_to(root).as_posix()
        except ValueError:
            return p.as_posix()
    return p.as_posix()


def changed_files(root: Path) -> list[str]:
    files: set[str] = set()
    commands = (
        ("git", "diff", "--name-only"),
        ("git", "diff", "--cached", "--name-only"),
        ("git", "ls-files", "--others", "--exclude-standard"),
    )
    for cmd in commands:
        proc = subprocess.run(
            cmd, cwd=root, text=True, capture_output=True, check=False
        )
        if proc.returncode != 0:
            raise GateFailure(proc.stderr.strip() or f"{' '.join(cmd)} failed")
        for line in proc.stdout.splitlines():
            line = line.strip()
            if line:
                files.add(line)
    return sorted(files)


def selected_targets(root: Path, changed: Iterable[str]) -> list[str]:
    selected: set[str] = set()
    changed_list = list(changed)
    for path in changed_list:
        if is_wiring_source(path):
            selected.add("wiring")
        if is_command_source(root, path):
            selected.add("ze-validate-commands")
        if is_command_ownership_source(path):
            selected.add("ze-command-ownership-check")
        if is_doc_source(root, path):
            selected.add("ze-doc-test")
            selected.add("ze-doc-check-stale")
        if is_inventory_source(root, path):
            selected.add("ze-inventory-json")
            selected.add("ze-command-list-json")
            selected.add("ze-plugin-imports-check")
    return [target for target in TARGET_ORDER if target in selected]


def is_wiring_source(path: str) -> bool:
    return (
        path.endswith(".go")
        and not path.endswith("_test.go")
        and (path.startswith("internal/") or path.startswith("cmd/"))
    )


def is_command_ownership_source(path: str) -> bool:
    """Changed files that must re-run the command-surface-ownership gate:
    the checker, the command registry + shim, any owner command package
    register.go, and the cmd/ze dispatch + central-registration files."""
    if path in {
        "scripts/checks/command_ownership.go",
        "cmd/ze/main.go",
    }:
        return True
    if path.startswith("internal/component/command/registry/"):
        return True
    if path.startswith("cmd/ze/internal/cmdregistry/"):
        return True
    if path.endswith("register.go") and (
        "/cli/" in path or "/client/" in path or path.startswith("cmd/ze/")
    ):
        return True
    return False


def is_command_source(root: Path, path: str) -> bool:
    if path in {
        "scripts/docvalid/commands.go",
        "scripts/inventory/commands.go",
        "internal/component/config/yang/command.go",
        "internal/component/plugin/server/command.go",
    }:
        return True
    if path.endswith("-cmd.yang"):
        return True
    if path.endswith(".yang") and file_or_head_contains(root, path, "ze:command"):
        return True
    if path.endswith(".go") and file_or_head_contains_any(
        root,
        path,
        (
            "cmdregistry.MustRegisterLocal",
            "cmdregistry.MustRegisterLocalMeta",
            "pluginserver.RegisterRPCs",
            "ze:command",
        ),
    ):
        return True
    return False


def is_doc_source(root: Path, path: str) -> bool:
    if path in {
        "scripts/docvalid/doc_drift.go",
        "scripts/docvalid/commands.go",
        "scripts/dev/code_to_docs.py",
        "ai/CODE-TO-DOCS.md",
    }:
        return True
    if path == "Makefile" or path.startswith("mk/"):
        return True
    if (path.startswith("docs/") or path == "README.md") and path.endswith(".md"):
        return file_or_head_contains(root, path, "<!-- source:")
    return False


def is_inventory_source(root: Path, path: str) -> bool:
    if path in {
        "Makefile",
        "mk/inventory.mk",
        "scripts/codegen/plugin_imports.go",
        "internal/component/plugin/all/all.go",
    }:
        return True
    if path.startswith("scripts/inventory/"):
        return True
    if path.endswith(".yang") and path.startswith("internal/"):
        return True
    if path.endswith("register.go") and path.startswith("internal/"):
        return file_or_head_contains_any(
            root,
            path,
            (
                "registry.Register",
                "MustRegister",
                "RegisterNamespace",
                "RegisterBackend",
                "yang.MustRegister",
            ),
        )
    return False


def file_or_head_contains(root: Path, path: str, needle: str) -> bool:
    return needle in read_current_or_empty(root, path) or needle in read_head_or_empty(
        root, path
    )


def file_or_head_contains_any(root: Path, path: str, needles: Iterable[str]) -> bool:
    text = read_current_or_empty(root, path)
    head = read_head_or_empty(root, path)
    return any(needle in text or needle in head for needle in needles)


def read_current_or_empty(root: Path, path: str) -> str:
    try:
        return (root / path).read_text(encoding="utf-8")
    except (FileNotFoundError, UnicodeDecodeError):
        return ""


def read_head_or_empty(root: Path, path: str) -> str:
    proc = subprocess.run(
        ("git", "show", f"HEAD:{path}"),
        cwd=root,
        text=True,
        capture_output=True,
        check=False,
    )
    if proc.returncode != 0:
        return ""
    return proc.stdout


def check_wiring(
    root: Path,
    changed: Iterable[str],
    baseline_reader: Callable[[str], str] | None = None,
) -> list[str]:
    if baseline_reader is None:
        baseline_reader = lambda path: read_head_or_empty(root, path)

    added: list[Symbol] = []
    for path in changed:
        if not is_wiring_source(path):
            continue
        current = read_current_or_empty(root, path)
        if not current:
            continue
        old_names = {sym.name for sym in exported_symbols(path, baseline_reader(path))}
        for sym in exported_symbols(path, current):
            if sym.name in old_names:
                continue
            if (sym.path, sym.name) in WIRING_ALLOWLIST:
                continue
            added.append(sym)

    if not added:
        return []

    issues: list[str] = []
    for sym in added:
        if not has_production_reference(root, sym):
            issues.append(
                f"{sym.path}:{sym.line}: exported {sym.kind} {sym.name} has no non-test reference in internal/ or cmd/"
            )
    return issues


def exported_symbols(path: str, content: str) -> list[Symbol]:
    symbols: list[Symbol] = []
    block_kind = ""
    for line_no, line in enumerate(content.splitlines(), start=1):
        code = line.split("//", 1)[0].strip()
        if not code:
            continue

        if block_kind:
            if code.startswith(")"):
                block_kind = ""
                continue
            for name in leading_exported_idents(code):
                symbols.append(
                    Symbol(path=path, line=line_no, kind=block_kind, name=name)
                )
            continue

        block_match = BLOCK_RE.match(code)
        if block_match:
            block_kind = block_match.group(1)
            continue

        func_match = FUNC_RE.match(code)
        if func_match:
            symbols.append(
                Symbol(path=path, line=line_no, kind="func", name=func_match.group(1))
            )
            continue

        type_match = TYPE_RE.match(code)
        if type_match:
            symbols.append(
                Symbol(path=path, line=line_no, kind="type", name=type_match.group(1))
            )
            continue

        const_match = CONST_RE.match(code)
        if const_match:
            for name in leading_exported_idents(const_match.group(1)):
                symbols.append(Symbol(path=path, line=line_no, kind="const", name=name))
            continue

        var_match = VAR_RE.match(code)
        if var_match:
            for name in leading_exported_idents(var_match.group(1)):
                symbols.append(Symbol(path=path, line=line_no, kind="var", name=name))
            continue

    return symbols


def leading_exported_idents(code: str) -> list[str]:
    match = IDENT_LIST_RE.match(code)
    if not match:
        return []
    names = [part.strip() for part in match.group(1).split(",")]
    return [name for name in names if name and name[0].isupper()]


def has_production_reference(root: Path, sym: Symbol) -> bool:
    token_re = re.compile(TOKEN_TEMPLATE.format(re.escape(sym.name)))
    for base in ("internal", "cmd"):
        base_path = root / base
        if not base_path.exists():
            continue
        for path in base_path.rglob("*.go"):
            rel = path.relative_to(root).as_posix()
            if rel.endswith("_test.go"):
                continue
            try:
                lines = path.read_text(encoding="utf-8").splitlines()
            except UnicodeDecodeError:
                continue
            for line_no, line in enumerate(lines, start=1):
                if rel == sym.path and line_no == sym.line:
                    continue
                code = line.split("//", 1)[0]
                if token_re.search(code):
                    return True
    return False


def check_plugin_imports(root: Path) -> None:
    proc = subprocess.run(
        ("go", "run", "scripts/codegen/plugin_imports.go", "--check"),
        cwd=root,
        text=True,
        capture_output=True,
        check=False,
    )
    if proc.stdout:
        print(proc.stdout, end="")
    if proc.stderr:
        print(proc.stderr, end="", file=sys.stderr)
    if proc.returncode != 0:
        raise GateFailure("plugin import check failed")


def run_make_target(make: str, target: str, root: Path) -> None:
    if target not in MAKE_TARGETS:
        raise GateFailure(f"unknown make target {target}")
    print(f"Running {target}...")
    proc = subprocess.run(
        (make, "--no-print-directory", target),
        cwd=root,
        text=True,
        capture_output=True,
        check=False,
    )
    if proc.returncode != 0:
        if proc.stdout:
            print(proc.stdout, end="")
        if proc.stderr:
            print(proc.stderr, end="", file=sys.stderr)
        raise GateFailure(f"{target} failed")
    print(f"{target} PASSED")


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except GateFailure as exc:
        print(str(exc), file=sys.stderr)
        raise SystemExit(1)
