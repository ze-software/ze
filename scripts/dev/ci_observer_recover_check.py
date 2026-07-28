#!/usr/bin/env python3
"""Keep the observer fail-closed guard's no-false-positive claim TRUE.

`_emit_sentinel_if_unwinding` (test/scripts/ze_api.py) writes the
ZE-OBSERVER-FAIL sentinel whenever an exception is in flight as the observer
calls into the engine. That is what un-swallowed a whole class of silent .ci
false-passes.

It is safe only because of a claim its own docstring makes:

    An AST scan of all 707 observer blocks found no engine call inside an
    `except` handler that recovers, so there is no false-positive surface.

That scan was run once, by hand, and nothing keeps it true. `sys.exc_info()` is
set for the WHOLE body of an `except` handler, including one that goes on to
recover -- so the first observer to write

    try:
        api.dispatch('show bgp summary')
    except RuntimeError:
        api.dispatch('show bgp peer list')   # <-- sentinel fires here
        continue                             # ... and the test fails anyway

turns a deliberately-handled error into a spurious FAILURE, and the author has
no way to know why. A one-off audit is not a guarantee; this is the guarantee.

WHAT COUNTS AS RECOVERING
    A handler that neither re-raises nor terminates. A handler ending in
    `raise`, `sys.exit(...)`, `os._exit(...)` or `runtime_fail(...)` is a
    failure path, where firing the sentinel is exactly right -- that is why
    test/plugin/cursor-replay.ci's `except RuntimeError: ... sys.exit(1)` is not
    a finding.

WHAT COUNTS AS AN ENGINE CALL
    Derived, not listed: this parses ze_api.py and takes the transitive closure
    of everything that reaches `_call_engine` or `wait_for_shutdown`, the guard's
    two call sites (ai/rules/derive-not-hardcode.md). A new ze_api helper that
    talks to the engine is covered the day it is written.

ESCAPE HATCH
    `# ze-observer-check: <reason>` on the call line or the line above it, so an
    exemption is a decision on the record rather than a quiet skip.

Usage:     python3 scripts/dev/ci_observer_recover_check.py [--json]
Called by: scripts/dev/ci_observer_recover_check_test.py, which runs the real
           repo scan and asserts zero findings -- so `make ze-unit-test` enforces
           the ratchet via TestPythonUnitTests, with no separate make target to
           forget. Promote it to its own verify stage if it ever gets slow;
           today it is well under a second.
"""

import argparse
import ast
import json
import sys
from pathlib import Path

EXEMPT_MARKER = "ze-observer-check:"

# The guard's two call sites in ze_api.py. Anything that transitively reaches
# either one can fire the sentinel.
GUARD_ENTRY_POINTS = ("_call_engine", "wait_for_shutdown")

# Names whose presence in a handler means it terminates rather than recovers.
TERMINATING_CALLS = {"exit", "_exit", "runtime_fail", "fail"}


def repo_root() -> Path:
    """Locate the module root by walking up for go.mod."""
    here = Path(__file__).resolve()
    for candidate in here.parents:
        if (candidate / "go.mod").is_file():
            return candidate
    raise SystemExit("go.mod not found above " + str(here))


# --------------------------------------------------------------------------
# 1. Derive the engine-reaching name set from ze_api.py
# --------------------------------------------------------------------------


def _called_names(node: ast.AST) -> set[str]:
    """Every function name called anywhere under node, attribute or bare."""
    names: set[str] = set()
    for sub in ast.walk(node):
        if not isinstance(sub, ast.Call):
            continue
        func = sub.func
        if isinstance(func, ast.Name):
            names.add(func.id)
        elif isinstance(func, ast.Attribute):
            names.add(func.attr)
    return names


def engine_reaching_names(ze_api_source: str) -> set[str]:
    """Names of ze_api functions/methods that transitively reach the guard.

    Both module-level functions and API methods are collected into one flat name
    set, because a .ci observer call site is matched on its trailing name: a bare
    `dispatch` and an `api.dispatch` attribute access are the same function to us.
    """
    tree = ast.parse(ze_api_source)

    # name -> names it calls
    calls: dict[str, set[str]] = {}
    for node in ast.walk(tree):
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            calls.setdefault(node.name, set()).update(_called_names(node))

    reaching = set(GUARD_ENTRY_POINTS)
    changed = True
    while changed:
        changed = False
        for name, callees in calls.items():
            if name in reaching:
                continue
            if callees & reaching:
                reaching.add(name)
                changed = True
    return reaching


# --------------------------------------------------------------------------
# 2. Extract embedded observer blocks from .ci files
# --------------------------------------------------------------------------


def extract_python_blocks(ci_text: str) -> list[tuple[str, int, str]]:
    """Return (block-name, first-content-line-number, source) per Python block.

    Mirrors internal/test/tmpfs/tmpfs.go: a `tmpfs=<path>[:opt...]:terminator=T`
    header, then content until a line that is exactly T. base64 blocks are
    skipped -- they are fixtures, not observers.
    """
    blocks: list[tuple[str, int, str]] = []
    lines = ci_text.splitlines()
    i = 0
    while i < len(lines):
        line = lines[i]
        if not line.startswith("tmpfs="):
            i += 1
            continue

        parts = line.split(":")
        path = parts[0][len("tmpfs=") :]
        terminator = ""
        encoding = ""
        for part in parts[1:]:
            if part.startswith("terminator="):
                terminator = part[len("terminator=") :]
            elif part.startswith("encoding="):
                encoding = part[len("encoding=") :]
        if not terminator:
            i += 1
            continue

        start = i + 1
        end = start
        while end < len(lines) and lines[end] != terminator:
            end += 1

        suffix = path.rsplit(".", 1)[-1] if "." in path else ""
        if suffix in ("py", "run") and encoding != "base64":
            blocks.append((path, start + 1, "\n".join(lines[start:end])))

        i = end + 1
    return blocks


# --------------------------------------------------------------------------
# 3. Find engine calls inside recovering except handlers
# --------------------------------------------------------------------------


def _handler_recovers(handler: ast.ExceptHandler) -> bool:
    """True when the handler neither re-raises nor terminates the process."""
    for node in ast.walk(handler):
        if isinstance(node, ast.Raise):
            return False
        if isinstance(node, ast.Call):
            func = node.func
            name = (
                func.attr
                if isinstance(func, ast.Attribute)
                else getattr(func, "id", "")
            )
            if name in TERMINATING_CALLS:
                return False
    return True


def _call_name(node: ast.Call) -> str:
    func = node.func
    if isinstance(func, ast.Attribute):
        return func.attr
    if isinstance(func, ast.Name):
        return func.id
    return ""


def _is_exempt(block_lines: list[str], lineno: int) -> bool:
    """Marker on the call line or the line directly above it."""
    for probe in (lineno - 1, lineno - 2):
        if 0 <= probe < len(block_lines) and EXEMPT_MARKER in block_lines[probe]:
            return True
    return False


def scan_block(source: str, engine_names: set[str]) -> list[tuple[int, str]]:
    """Return (line-within-block, called-name) for each finding."""
    try:
        tree = ast.parse(source)
    except SyntaxError:
        # Not Python (a .run block can be shell). Silence is correct here: this
        # gate is about Python observers, and ci_dispatch_commands.go already
        # owns "a command string I could not read".
        return []

    block_lines = source.splitlines()
    findings: list[tuple[int, str]] = []
    for node in ast.walk(tree):
        if not isinstance(node, ast.ExceptHandler):
            continue
        if not _handler_recovers(node):
            continue
        for sub in ast.walk(node):
            if not isinstance(sub, ast.Call):
                continue
            name = _call_name(sub)
            if name in engine_names and not _is_exempt(block_lines, sub.lineno):
                findings.append((sub.lineno, name))
    return findings


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--json", action="store_true", help="emit findings as JSON")
    args = parser.parse_args()

    root = repo_root()
    ze_api_path = root / "test/scripts/ze_api.py"
    if not ze_api_path.is_file():
        print(f"error: {ze_api_path} not found", file=sys.stderr)
        return 1

    engine_names = engine_reaching_names(ze_api_path.read_text())

    ci_files = sorted((root / "test").rglob("*.ci"))
    findings = []
    blocks_scanned = 0
    for ci in ci_files:
        try:
            text = ci.read_text()
        except (OSError, UnicodeDecodeError):
            continue
        for name, offset, source in extract_python_blocks(text):
            blocks_scanned += 1
            for lineno, called in scan_block(source, engine_names):
                findings.append(
                    {
                        "file": str(ci.relative_to(root)),
                        "line": offset + lineno - 1,
                        "block": name,
                        "call": called,
                    }
                )

    if args.json:
        json.dump(
            {
                "schema-version": 1,
                "engine-reaching-names": sorted(engine_names),
                "ci-files-scanned": len(ci_files),
                "observer-blocks-scanned": blocks_scanned,
                "findings": findings,
            },
            sys.stdout,
            indent=2,
        )
        sys.stdout.write("\n")
        return 1 if findings else 0

    if not findings:
        print(
            f"ci-observer-recover-check: OK "
            f"({blocks_scanned} observer blocks in {len(ci_files)} .ci files, "
            f"{len(engine_names)} engine-reaching names)"
        )
        return 0

    print("ci-observer-recover-check: FAIL", file=sys.stderr)
    print("", file=sys.stderr)
    print(
        "An engine call sits inside an `except` handler that RECOVERS. "
        "ze_api's fail-closed guard fires the ZE-OBSERVER-FAIL sentinel for any "
        "engine call made while an exception is in flight -- including a handled "
        "one -- so this would fail the test spuriously, with no clue why.",
        file=sys.stderr,
    )
    print("", file=sys.stderr)
    for f in findings:
        print(
            f"  {f['file']}:{f['line']}  {f['block']}  calls {f['call']}()",
            file=sys.stderr,
        )
    print("", file=sys.stderr)
    print(
        "Fix: move the engine call out of the handler, re-raise, or terminate "
        f"(sys.exit / runtime_fail). If firing IS intended, mark the line: "
        f"# {EXEMPT_MARKER} <reason>",
        file=sys.stderr,
    )
    return 1


if __name__ == "__main__":
    sys.exit(main())
