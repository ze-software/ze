#!/usr/bin/env python3
"""Post-verify validation: catches recurring implementation mistakes.

Each check derives from a documented defect pattern in
plan/learned/RECURRING-PATTERNS.md.

Exit codes:
  0 - all checks passed
  1 - findings (ISSUE severity)
  2 - script error
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

YELLOW = "\033[33m"
RED = "\033[31m"
GREEN = "\033[32m"
RESET = "\033[0m"


@dataclass(frozen=True)
class Finding:
    severity: str
    file: str
    line: int
    message: str

    def __str__(self) -> str:
        loc = f"{self.file}:{self.line}" if self.line else self.file
        return f"[{self.severity}] {loc}: {self.message}"


SOURCE_ANCHOR_RE = re.compile(r"<!--\s*source:\s*(\S+)\s+--")
SOURCE_ANCHOR_LINE_RE = re.compile(r"<!--\s*source:\s*\S+\.go:\d+\s")
AC_ROW_RE = re.compile(r"^\|\s*(AC-\d+)\s*\|([^|]*)\|([^|]*)\|([^|]*)\|")
EXPORTED_FUNC_RE = re.compile(r"^func\s+(?:\([^)]*\)\s*)?([A-Z][A-Za-z0-9_]*)\s*\(")
EXPORTED_TYPE_RE = re.compile(r"^type\s+([A-Z][A-Za-z0-9_]*)\b")
SPEC_STATUS_RE = re.compile(r"^\|\s*Status\s*\|\s*(\S+)\s*\|", re.MULTILINE)

CLI_PATHS = (
    "internal/component/cli/",
    "internal/component/cmd/",
    "internal/plugins/",
)
REGISTER_RE = re.compile(r'MustRegister\w+\(\s*"([^"]+)"')


def find_repo_root(start: Path | None = None) -> Path:
    cur = (start or Path.cwd()).resolve()
    while True:
        if (cur / "go.mod").exists():
            return cur
        if cur.parent == cur:
            raise SystemExit("could not find go.mod above " + str(start or Path.cwd()))
        cur = cur.parent


def changed_files(root: Path) -> list[str]:
    files: set[str] = set()
    commands = (
        ("git", "diff", "--name-only", "HEAD"),
        ("git", "ls-files", "--others", "--exclude-standard"),
    )
    for cmd in commands:
        proc = subprocess.run(
            cmd, cwd=root, text=True, capture_output=True, check=False
        )
        if proc.returncode == 0:
            for line in proc.stdout.splitlines():
                line = line.strip()
                if line:
                    files.add(line)
    return sorted(files)


def check_source_anchor_line_numbers(root: Path) -> list[Finding]:
    findings: list[Finding] = []
    docs_dir = root / "docs"
    if not docs_dir.is_dir():
        return findings
    for md_file in sorted(docs_dir.rglob("*.md")):
        try:
            lines = md_file.read_text(encoding="utf-8").splitlines()
        except OSError:
            continue
        for i, line in enumerate(lines, 1):
            m = SOURCE_ANCHOR_LINE_RE.search(line)
            if m:
                anchor = SOURCE_ANCHOR_RE.search(line)
                path = anchor.group(1) if anchor else "unknown"
                rel = md_file.relative_to(root)
                findings.append(
                    Finding(
                        severity="ISSUE",
                        file=str(rel),
                        line=i,
                        message=f"source anchor {path} contains line number; use path only (line numbers rot)",
                    )
                )
    return findings


def check_source_anchor_stale_paths(root: Path) -> list[Finding]:
    findings: list[Finding] = []
    docs_dir = root / "docs"
    if not docs_dir.is_dir():
        return findings
    for md_file in sorted(docs_dir.rglob("*.md")):
        try:
            lines = md_file.read_text(encoding="utf-8").splitlines()
        except OSError:
            continue
        for i, line in enumerate(lines, 1):
            m = SOURCE_ANCHOR_RE.search(line)
            if not m:
                continue
            path = m.group(1)
            if path.startswith(("http://", "https://")) or "/" not in path:
                continue
            path_clean = re.sub(r":\d+$", "", path)
            if not (root / path_clean).exists():
                rel = md_file.relative_to(root)
                findings.append(
                    Finding(
                        severity="ISSUE",
                        file=str(rel),
                        line=i,
                        message=f"source anchor points to non-existent file: {path_clean}",
                    )
                )
    return findings


def _has_cross_pkg_ref(
    root: Path, sym: str, pkg_dir: str, search_dirs: list[str]
) -> bool:
    """True if sym is referenced (whole word) by a non-test file in another package."""
    proc = subprocess.run(
        ["grep", "-rlw", "--include=*.go", sym] + search_dirs,
        cwd=root,
        capture_output=True,
        text=True,
        check=False,
    )
    for mf in (f.strip() for f in proc.stdout.splitlines()):
        if not mf or mf.endswith("_test.go"):
            continue
        if str(Path(mf).parent) != pkg_dir:
            return True
    return False


# One const spec's leading form: "Name1, Name2  Type = expr", "Name = expr",
# "Name Type", or a bare "Name". Group 1 = comma-separated names, group 2 = the
# explicit type (absent for value-only or bare specs), group 3 = the "=" when an
# expression is present.
CONST_SPEC_RE = re.compile(
    r"^\s*([A-Za-z_][A-Za-z0-9_]*(?:\s*,\s*[A-Za-z_][A-Za-z0-9_]*)*)"
    r"(?:\s+([A-Za-z_][A-Za-z0-9_.]*))?"
    r"(\s*=)?"
)


def _const_spec(text: str) -> tuple[list[str], str | None, bool]:
    """Parse one const spec into (names, explicit_type, has_value_expression)."""
    m = CONST_SPEC_RE.match(text)
    if not m:
        return [], None, False
    names = [n.strip() for n in m.group(1).split(",")]
    return names, m.group(2), m.group(3) is not None


def _exported(names: list[str]) -> list[str]:
    return [n for n in names if n[:1].isupper()]


def _exported_consts_of_type(root: Path, pkg_dir: str, type_name: str) -> set[str]:
    """Exported const identifiers declared with type_name in pkg_dir.

    A typed enum is reached through its constant values, so the bare type name
    may never appear in another package (callers switch on RouteVerbInstall,
    never spelling RouteVerb) and the bare-name grep undercounts. This recovers
    the constants -- single-line consts, block consts, multi-name specs, and the
    Go iota idiom where a bare spec inherits the type of the last preceding spec
    that carried one (an explicit "= expr" with no type resets that inheritance,
    matching the language spec) -- so the caller can prove the type is reachable.
    """
    names: set[str] = set()
    pkg_path = root / pkg_dir
    if not pkg_path.is_dir():
        return names
    for go_file in sorted(pkg_path.glob("*.go")):
        if go_file.name.endswith("_test.go"):
            continue
        try:
            content = go_file.read_text(encoding="utf-8")
        except OSError:
            continue
        in_block = False
        inherits = False  # do bare specs in this block currently inherit type_name?
        for line in content.splitlines():
            stripped = line.strip()
            if not in_block:
                if re.match(r"^const\s*\(", stripped):
                    in_block = True
                    inherits = False
                else:
                    m = re.match(r"^const\s+(.+)$", stripped)
                    if m:
                        spec, etype, _ = _const_spec(m.group(1))
                        if etype == type_name:
                            names.update(_exported(spec))
                continue
            if stripped.startswith(")"):
                in_block = False
                inherits = False
                continue
            if not stripped or stripped.startswith("//"):
                continue
            spec, etype, has_value = _const_spec(line)
            if not spec:
                continue
            if etype is not None:
                inherits = etype == type_name
                if inherits:
                    names.update(_exported(spec))
            elif has_value:
                inherits = False  # own type from the value; resets iota inheritance
            elif inherits:
                names.update(_exported(spec))
    return names


def check_cross_package_wiring(root: Path, changed: list[str]) -> list[Finding]:
    go_files = [
        f
        for f in changed
        if f.endswith(".go")
        and not f.endswith("_test.go")
        and (f.startswith("internal/") or f.startswith("cmd/"))
    ]
    if not go_files:
        return []

    symbols: list[tuple[str, int, str, str, str]] = []
    for go_file in go_files:
        full_path = root / go_file
        if not full_path.exists():
            continue
        pkg_dir = str(Path(go_file).parent)
        try:
            content = full_path.read_text(encoding="utf-8")
        except OSError:
            continue
        for line_num, line in enumerate(content.splitlines(), 1):
            m = EXPORTED_FUNC_RE.match(line)
            if m:
                symbols.append((go_file, line_num, m.group(1), pkg_dir, "func"))
                continue
            m = EXPORTED_TYPE_RE.match(line)
            if m:
                symbols.append((go_file, line_num, m.group(1), pkg_dir, "type"))

    if not symbols:
        return []

    findings: list[Finding] = []
    search_dirs = [d for d in ("internal", "cmd", "pkg") if (root / d).is_dir()]
    if not search_dirs:
        return findings

    for go_file, line_num, sym, pkg_dir, kind in symbols:
        if _has_cross_pkg_ref(root, sym, pkg_dir, search_dirs):
            continue

        # A typed enum is reached through its constant values, not the bare type
        # name: callers switch on RouteVerbInstall and never spell RouteVerb. If
        # any of its exported constants is referenced cross-package, the type is
        # wired even though grep on the type name found nothing.
        if kind == "type" and any(
            _has_cross_pkg_ref(root, const, pkg_dir, search_dirs)
            for const in _exported_consts_of_type(root, pkg_dir, sym)
        ):
            continue

        findings.append(
            Finding(
                severity="ISSUE",
                file=go_file,
                line=line_num,
                message=f"exported symbol {sym} has no cross-package non-test caller",
            )
        )

    return findings


def check_spec_ac_completeness(root: Path) -> list[Finding]:
    findings: list[Finding] = []
    plan_dir = root / "plan"
    if not plan_dir.is_dir():
        return findings

    for spec_file in sorted(plan_dir.glob("spec-*.md")):
        try:
            content = spec_file.read_text(encoding="utf-8")
        except OSError:
            continue

        status_m = SPEC_STATUS_RE.search(content)
        if not status_m or status_m.group(1) != "in-progress":
            continue

        in_audit = False
        lines = content.splitlines()
        for i, line in enumerate(lines, 1):
            if "### Acceptance Criteria" in line:
                in_audit = True
                continue
            if in_audit and line.startswith("### "):
                in_audit = False
                continue
            if not in_audit:
                continue

            m = AC_ROW_RE.match(line)
            if not m:
                continue
            ac_id = m.group(1).strip()
            demonstrated_by = m.group(3).strip()
            if not demonstrated_by:
                rel = spec_file.relative_to(root)
                findings.append(
                    Finding(
                        severity="ISSUE",
                        file=str(rel),
                        line=i,
                        message=f"{ac_id} has empty 'Demonstrated By' column",
                    )
                )

    return findings


def check_cli_handler_coverage(root: Path, changed: list[str]) -> list[Finding]:
    cli_files = [
        f
        for f in changed
        if f.endswith(".go")
        and not f.endswith("_test.go")
        and any(f.startswith(p) for p in CLI_PATHS)
    ]
    if not cli_files:
        return []

    findings: list[Finding] = []
    test_dir = root / "test"
    if not test_dir.is_dir():
        return findings

    ci_content: str | None = None

    for cli_file in cli_files:
        full_path = root / cli_file
        if not full_path.exists():
            continue
        try:
            content = full_path.read_text(encoding="utf-8")
        except OSError:
            continue

        commands = REGISTER_RE.findall(content)
        if not commands:
            continue

        if ci_content is None:
            parts = []
            for ci_file in test_dir.rglob("*.ci"):
                try:
                    parts.append(ci_file.read_text(encoding="utf-8"))
                except OSError:
                    continue
            ci_content = "\n".join(parts)

        for cmd in commands:
            if cmd not in ci_content:
                findings.append(
                    Finding(
                        severity="ISSUE",
                        file=cli_file,
                        line=0,
                        message=f"CLI command '{cmd}' has no .ci test mentioning it",
                    )
                )

    return findings


def run_checks(root: Path, changed: list[str]) -> list[Finding]:
    findings: list[Finding] = []
    findings.extend(check_source_anchor_line_numbers(root))
    findings.extend(check_source_anchor_stale_paths(root))
    findings.extend(check_cross_package_wiring(root, changed))
    findings.extend(check_spec_ac_completeness(root))
    findings.extend(check_cli_handler_coverage(root, changed))
    return findings


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Post-verify validation: catches recurring implementation mistakes."
    )
    parser.add_argument("--root", default=".", help="repository root (default: cwd)")
    parser.add_argument(
        "--changed-file",
        action="append",
        default=[],
        dest="changed_files",
        help="changed file to check (repeatable; default: git diff)",
    )
    args = parser.parse_args()

    try:
        root = find_repo_root(Path(args.root))
    except SystemExit:
        print(
            f"{RED}error: not in a git repository with go.mod{RESET}", file=sys.stderr
        )
        return 2

    changed = args.changed_files if args.changed_files else changed_files(root)
    findings = run_checks(root, changed)

    if not findings:
        print(f"{GREEN}ze-validate: all checks passed{RESET}")
        return 0

    issues = [f for f in findings if f.severity == "ISSUE"]
    warns = [f for f in findings if f.severity == "WARN"]

    if issues:
        print(f"{RED}ze-validate: {len(issues)} issue(s) found{RESET}")
        for f in issues:
            print(f"  {RED}{f}{RESET}")

    if warns:
        print(f"{YELLOW}ze-validate: {len(warns)} warning(s){RESET}")
        for f in warns:
            print(f"  {YELLOW}{f}{RESET}")

    return 1 if issues else 0


if __name__ == "__main__":
    sys.exit(main())
