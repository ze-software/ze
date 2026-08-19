#!/usr/bin/env python3
"""Refuse the next test-harness call site that reads a fail-open return value.

`docker_exec_quiet` (test/interop/interop.py) returns `""` on ANY non-zero exit.
A caller that does not test the value for emptiness turns a command that FAILED
into a passing assertion over nothing: `"DIS" in ""` is False, and False is also
the honest answer when no Designated IS is elected. The two are indistinguishable
at the call site, which is the zero-is-a-valid-looking-answer shape
ai/rules/evidence.md names.

Nineteen instances were repaired by hand on 2026-08-07. Nothing refused the
twentieth. This does.

WHAT COUNTS AS FAIL-OPEN
    Derived, not listed: the transitive closure from the seed, to a fixpoint. A
    function whose body `return`s a call to a member is itself a member, because
    its own callers inherit the empty string unchanged. Scenarios call the
    WRAPPERS, so a check that knew only the seed name would report a handful of
    sites and miss the class it exists to close. The closure is computed over
    every walked file at once: a wrapper in a scenario's own check.py joins the
    set the day it is written.

THE FOUR VERDICTS
    checked     the value is bound and the bound name is tested for emptiness in
                the same function, OR the call is that function's own `return`
                (the obligation moves to its callers, who are call sites too).
    discarded   a bare-statement call. Fire-and-forget asserts nothing, so it
                cannot assert nothing wrongly. Not flagged.
    exempt      an opt-out marker with a reason.
    unchecked   everything else: bound and never tested, or used inline
                (`f(x)[:5]`, `"s" in f(x)`, `json.loads(f(x))`).

    A membership test is NOT an emptiness test. `if prefix in out` is False on
    "", so it is the defect rather than the guard.

ESCAPE HATCH
    `# fail-open-ok: <reason>` on the call line or the line above it. The reason
    is required: a bare marker does not count, so an exemption is a decision on
    the record rather than a quiet skip. Auditable with one grep, the same shape
    as `test-asserts-nothing:`.

TURNING IT ON
    A committed floor in test/health/docker-exec-baseline.json that may only go
    DOWN, following test/health/sensitivity-baseline.json. That refuses the
    twentieth site on the day it lands, which is what this gate is FOR, without
    putting a large mechanical diff between the defect and its guard. The count
    is the backlog, in the open, and it can only fall.

Usage:     python3 scripts/dev/docker_exec_checked.py [--root DIR] [--json|--selftest]
Called by: make ze-functional-docker-exec-check (routed onto the verify path by
           scripts/dev/verify_wiring_docs.py when a test/**/*.py scenario or lab
           file changes) and scripts/dev/docker_exec_checked_test.py, whose
           TestRepoRatchet runs the real scan so `make ze-unit-test` enforces the
           floor too.
"""

import argparse
import ast
import json
import re
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path

# The one hand-written name in this file. Everything else is derived from it.
SEED = "docker_exec_quiet"

BASELINE_DIR = "test/health"
BASELINE_NAME = "docker-exec-baseline.json"
BASELINE_KEY = "unchecked"

# The reason is what makes an exemption auditable, so the pattern requires one.
EXEMPT_RE = re.compile(r"#\s*fail-open-ok:\s*\S")

# Functional tests under development live here. Gitignored and invisible to every
# repo-wide gate, so a draft cannot move this ratchet for a session that had
# nothing to do with it (test/draft/README.md, internal/test/runner/draft_dir.go).
DRAFT_DIR = "draft"

# Directory names that hold a whole extra copy of the repository. Counting them
# turned 81 real sites into 1986 during the survey that produced this gate.
SKIP_PARTS = {"tmp", ".claude", ".git", "__pycache__", "node_modules", "vendor"}

# Constants that mean "this call produced nothing".
EMPTY_CONSTANTS = ("", None)


class CheckError(Exception):
    """A tree this checker cannot read. Always fatal: silence would be the same
    blind spot in a new place."""


@dataclass(frozen=True)
class Site:
    """One call of a fail-open function."""

    file: str
    line: int
    function: str
    member: str
    verdict: str


@dataclass
class Report:
    members: set[str] = field(default_factory=set)
    sites: list[Site] = field(default_factory=list)
    files: int = 0

    @property
    def counts(self) -> dict[str, int]:
        out = {"checked": 0, "discarded": 0, "exempt": 0, "unchecked": 0}
        for site in self.sites:
            out[site.verdict] += 1
        return out

    def unchecked(self) -> list[Site]:
        return [s for s in self.sites if s.verdict == "unchecked"]


def repo_root() -> Path:
    """Locate the module root by walking up for go.mod."""
    here = Path(__file__).resolve()
    for candidate in here.parents:
        if (candidate / "go.mod").is_file():
            return candidate
    raise CheckError("go.mod not found above " + str(here))


def python_files(root: Path):
    """Every *.py under test/, minus the draft incubator and any nested copy of
    the repository."""
    base = root / "test"
    for path in sorted(base.rglob("*.py")):
        parts = path.relative_to(base).parts
        if parts[:1] == (DRAFT_DIR,):
            continue
        if SKIP_PARTS.intersection(parts):
            continue
        yield path


def parse(rel: str, text: str) -> ast.Module:
    """Parse one source, naming the file when it cannot be read."""
    try:
        return ast.parse(text)
    except SyntaxError as err:
        raise CheckError(f"{rel}: cannot be parsed: {err}") from None


def call_name(node: ast.Call) -> str:
    """The trailing name of a call: `api.f()` and `f()` are the same function
    here, because a member is matched on its name across every file."""
    func = node.func
    if isinstance(func, ast.Attribute):
        return func.attr
    if isinstance(func, ast.Name):
        return func.id
    return ""


def fail_open_functions(trees: dict[str, ast.Module]) -> set[str]:
    """The transitive fail-open set, to a fixpoint, seeded with SEED.

    A function joins when its body `return`s a CALL to a member. Binding the
    value and then returning the name does not propagate: that function had the
    chance to test it, so the obligation stays there rather than moving out.
    """
    returned: dict[str, set[str]] = {}
    for tree in trees.values():
        for node in ast.walk(tree):
            if not isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
                continue
            names = {
                call_name(sub.value)
                for sub in ast.walk(node)
                if isinstance(sub, ast.Return) and isinstance(sub.value, ast.Call)
            }
            returned.setdefault(node.name, set()).update(names)

    members = {SEED}
    changed = True
    while changed:
        changed = False
        for name, callees in returned.items():
            if name not in members and callees & members:
                members.add(name)
                changed = True
    return members


def _truth_root(expr: ast.AST) -> str:
    """The name a truth-position expression evaluates, through attribute and call
    chains: `out`, `out.strip()` and `out.splitlines()` all root at `out`."""
    while True:
        if isinstance(expr, ast.Name):
            return expr.id
        if isinstance(expr, ast.Attribute):
            expr = expr.value
        elif isinstance(expr, ast.Call):
            expr = expr.func
        else:
            return ""


def _truth_operands(test: ast.AST) -> list[ast.AST]:
    """The sub-expressions a test evaluates for truthiness, unwrapping `not` and
    `and`/`or` so `if not out` and `if out and x` both reach `out`."""
    out: list[ast.AST] = []
    stack = [test]
    while stack:
        node = stack.pop()
        if isinstance(node, ast.UnaryOp) and isinstance(node.op, ast.Not):
            stack.append(node.operand)
        elif isinstance(node, ast.BoolOp):
            stack.extend(node.values)
        else:
            out.append(node)
    return out


def _is_empty_constant(node: ast.AST) -> bool:
    return isinstance(node, ast.Constant) and any(
        node.value is empty or node.value == empty for empty in EMPTY_CONSTANTS
    )


def _is_len_of(node: ast.AST, name: str) -> bool:
    return (
        isinstance(node, ast.Call)
        and isinstance(node.func, ast.Name)
        and node.func.id == "len"
        and bool(node.args)
        and isinstance(node.args[0], ast.Name)
        and node.args[0].id == name
    )


def _tests_emptiness(test: ast.AST, name: str) -> bool:
    """True when this test expression asks whether `name` is empty.

    A membership test is deliberately NOT one: `if prefix in out` is False on the
    empty string, so it is the defect rather than the guard.
    """
    for operand in _truth_operands(test):
        if _truth_root(operand) == name:
            return True
        if not isinstance(operand, ast.Compare) or len(operand.ops) != 1:
            continue
        left, right = operand.left, operand.comparators[0]
        if isinstance(operand.ops[0], (ast.Eq, ast.NotEq, ast.Is, ast.IsNot)):
            for near, far in ((left, right), (right, left)):
                if isinstance(near, ast.Name) and near.id == name:
                    if _is_empty_constant(far):
                        return True
        if _is_len_of(left, name) or _is_len_of(right, name):
            return True
    return False


def _emptiness_tested(scope: ast.AST, name: str, after: int) -> bool:
    """True when the scope tests `name` for emptiness AFTER line `after`.

    The position is load-bearing, not a refinement. Without it any test of a
    same-named variable anywhere in the function marked EVERY assignment of that
    name checked, and three live sites rode out on a guard belonging to an
    earlier call. `FRR.route_count` (test/interop/interop.py) is the worst: its
    JSON call is guarded, its text-fallback `output = self._vtysh_quiet(...)` is
    not, `splitlines()` on "" is empty, and the function answers 0 prefixes for a
    vtysh that FAILED. That is guard 3's `Ze.rib_count` shape surviving inside
    guard 4, which is the one outcome this checker may not have.

    A test on the line of the call itself counts: `if not (out := f(...)):` and
    `if not f(...):` are both written that way.
    """
    for node in ast.walk(scope):
        if isinstance(node, ast.comprehension):
            if any(_tests_emptiness(cond, name) for cond in node.ifs):
                return True
            continue
        test = getattr(node, "test", None)
        if not isinstance(node, (ast.If, ast.While, ast.IfExp, ast.Assert)):
            continue
        if test is None or getattr(test, "lineno", 0) < after:
            continue
        if _tests_emptiness(test, name):
            return True
    return False


def _is_exempt(lines: list[str], lineno: int) -> bool:
    """Marker with a reason, on the call line or the line directly above it."""
    for probe in (lineno - 1, lineno - 2):
        if 0 <= probe < len(lines) and EXEMPT_RE.search(lines[probe]):
            return True
    return False


def _parents_and_scopes(tree: ast.Module):
    """Every node's parent, and the function that encloses it."""
    parent: dict[ast.AST, ast.AST] = {}
    scope: dict[ast.AST, ast.AST] = {}
    stack: list[tuple[ast.AST, ast.AST]] = [(tree, tree)]
    while stack:
        node, enclosing = stack.pop()
        for child in ast.iter_child_nodes(node):
            parent[child] = node
            child_scope = (
                child
                if isinstance(child, (ast.FunctionDef, ast.AsyncFunctionDef))
                else enclosing
            )
            scope[child] = child_scope
            stack.append((child, child_scope))
    return parent, scope


def classify(rel: str, source: str, tree: ast.Module, members: set[str]) -> list[Site]:
    """One Site per call of a member, with its verdict."""
    parent, scope_of = _parents_and_scopes(tree)
    lines = source.splitlines()
    sites: list[Site] = []
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        member = call_name(node)
        if member not in members:
            continue
        scope = scope_of.get(node, tree)
        function = getattr(scope, "name", "<module>")
        sites.append(
            Site(
                file=rel,
                line=node.lineno,
                function=function,
                member=member,
                verdict=_verdict(node, parent.get(node), scope, lines),
            )
        )
    return sorted(sites, key=lambda s: (s.file, s.line, s.member))


def _verdict(node: ast.Call, par: ast.AST | None, scope: ast.AST, lines) -> str:
    if isinstance(par, ast.Expr):
        return "discarded"
    if isinstance(par, ast.Return):
        return "checked"
    if (
        isinstance(par, ast.Assign)
        and len(par.targets) == 1
        and isinstance(par.targets[0], ast.Name)
        and _emptiness_tested(scope, par.targets[0].id, node.lineno)
    ):
        return "checked"
    if _is_exempt(lines, node.lineno):
        return "exempt"
    return "unchecked"


def scan(root: Path) -> Report:
    """Parse the whole population once, derive the set, classify every site."""
    sources: dict[str, str] = {}
    trees: dict[str, ast.Module] = {}
    for path in python_files(root):
        rel = path.relative_to(root).as_posix()
        try:
            text = path.read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError) as err:
            raise CheckError(f"{rel}: cannot be read: {err}") from None
        sources[rel] = text
        trees[rel] = parse(rel, text)

    members = fail_open_functions(trees)
    report = Report(members=members, files=len(trees))
    for rel, tree in trees.items():
        report.sites.extend(classify(rel, sources[rel], tree, members))
    report.sites.sort(key=lambda s: (s.file, s.line, s.member))
    return report


def baseline_path(root: Path) -> Path:
    return root / BASELINE_DIR / BASELINE_NAME


def read_baseline(root: Path) -> int:
    """The committed floor. A missing file is an error, never a default.

    Defaulting it to today's count is exactly how a regression is laundered:
    `rm` the baseline and the gate would mint the new number as the floor.
    """
    path = baseline_path(root)
    if not path.is_file():
        raise CheckError(
            f"{BASELINE_DIR}/{BASELINE_NAME} does not exist. Restore it from git "
            "rather than letting this run mint today's count as the new floor."
        )
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as err:
        raise CheckError(f"{BASELINE_DIR}/{BASELINE_NAME}: {err}") from None
    if not isinstance(data, dict) or BASELINE_KEY not in data:
        raise CheckError(
            f"{BASELINE_DIR}/{BASELINE_NAME}: no `{BASELINE_KEY}` key; a missing "
            "key would silently disable the ratchet."
        )
    try:
        return int(data[BASELINE_KEY])
    except (TypeError, ValueError) as err:
        raise CheckError(
            f"{BASELINE_DIR}/{BASELINE_NAME}: `{BASELINE_KEY}` is not a number: {err}"
        ) from None


def touched_files(root: Path) -> set[str] | None:
    """Test files this working tree changed against HEAD, or None when git
    cannot answer. Used only to narrow a failure report to the likely author."""
    try:
        proc = subprocess.run(
            ["git", "status", "--porcelain", "--", "test"],
            cwd=root,
            capture_output=True,
            text=True,
            timeout=30,
            check=False,
        )
    except (OSError, subprocess.SubprocessError):
        return None
    if proc.returncode != 0:
        return None
    paths: set[str] = set()
    for line in proc.stdout.splitlines():
        entry = line[3:].strip()
        if " -> " in entry:
            entry = entry.split(" -> ", 1)[1]
        paths.add(entry.strip('"'))
    return paths


def report_failure(root: Path, report: Report, baseline: int) -> None:
    """Name the sites that most likely caused the rise, then say how to fix it."""
    unchecked = report.unchecked()
    touched = touched_files(root)
    suspects = [s for s in unchecked if touched and s.file in touched]
    if suspects:
        heading = "New site(s), in the test files this tree changed:"
    else:
        suspects = unchecked
        heading = "No changed test file explains the rise; every unchecked site:"

    print("docker-exec-check: FAIL", file=sys.stderr)
    print("", file=sys.stderr)
    print(
        f"  {len(unchecked)} unchecked fail-open call site(s); the committed floor "
        f"({BASELINE_DIR}/{BASELINE_NAME}) allows {baseline}.",
        file=sys.stderr,
    )
    print("", file=sys.stderr)
    print("  " + heading, file=sys.stderr)
    for site in suspects:
        print(
            f"    {site.file}:{site.line}  {site.function}() reads {site.member}()",
            file=sys.stderr,
        )
    print("", file=sys.stderr)
    print(
        "  Fix: test the returned value for emptiness before you read it, and "
        "raise or fail the scenario when it is empty. A command that FAILED "
        "otherwise reads as an assertion over nothing. If the read is genuinely "
        "diagnostic on an already-failed run, mark the line: "
        "# fail-open-ok: <reason>",
        file=sys.stderr,
    )


SELFTEST_SOURCE = """
def docker_exec_quiet(container, cmd):
    return ""


def _vtysh_quiet(container, command):
    return docker_exec_quiet(container, ["vtysh", "-c", command])


def route(container, prefix):
    output = _vtysh_quiet(container, "show bgp %s json" % prefix)
    if not output.strip():
        return None
    return output


def is_dis(container):
    out = _vtysh_quiet(container, "show isis interface detail")
    return "DIS" in out or "Designated" in out


def warm(container):
    _vtysh_quiet(container, "show version")


def dump(container):
    print(_vtysh_quiet(container, "show isis neighbor")[:500])  # fail-open-ok: diag


def unmarked(container):
    print(_vtysh_quiet(container, "show isis neighbor")[:500])


def bare_marker(container):
    # fail-open-ok:
    print(_vtysh_quiet(container, "show isis neighbor")[:500])
"""

# Expected verdict per enclosing function, for SELFTEST_SOURCE. `_vtysh_quiet`
# itself is the return-site case: the obligation moves to its callers.
SELFTEST_EXPECTED = {
    "_vtysh_quiet": "checked",
    "route": "checked",
    "is_dis": "unchecked",
    "warm": "discarded",
    "dump": "exempt",
    "unmarked": "unchecked",
    "bare_marker": "unchecked",
}


def run_selftest() -> None:
    """Prove both polarities of every verdict on a fixture, before trusting a
    real scan: a detector that fires on nothing passes any tree."""
    tree = parse("selftest.py", SELFTEST_SOURCE)
    members = fail_open_functions({"selftest.py": tree})
    expected_members = {SEED, "_vtysh_quiet"}
    if members != expected_members:
        raise CheckError(
            f"fail-open set is {sorted(members)}, want {sorted(expected_members)}"
        )

    got = {
        site.function: site.verdict
        for site in classify("selftest.py", SELFTEST_SOURCE, tree, members)
    }
    if got != SELFTEST_EXPECTED:
        raise CheckError(f"verdicts are {got}, want {SELFTEST_EXPECTED}")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", default=None, help="repository root to scan")
    parser.add_argument("--json", action="store_true", help="emit the report as JSON")
    parser.add_argument(
        "--selftest", action="store_true", help="run built-in fixture tests"
    )
    args = parser.parse_args()

    try:
        if args.selftest:
            run_selftest()
            print("docker-exec-check selftest: OK")
            return 0

        root = Path(args.root).resolve() if args.root else repo_root()
        report = scan(root)
        baseline = read_baseline(root)
    except CheckError as err:
        print(f"docker-exec-check: {err}", file=sys.stderr)
        return 1

    counts = report.counts
    over = counts["unchecked"] > baseline

    if args.json:
        json.dump(
            {
                "schema-version": 1,
                "seed": SEED,
                "fail-open-functions": sorted(report.members),
                "files-scanned": report.files,
                "counts": counts,
                "baseline": baseline,
                "unchecked-sites": [
                    {
                        "file": s.file,
                        "line": s.line,
                        "function": s.function,
                        "member": s.member,
                    }
                    for s in report.unchecked()
                ],
            },
            sys.stdout,
            indent=2,
        )
        sys.stdout.write("\n")
        return 1 if over else 0

    if over:
        report_failure(root, report, baseline)
        return 1

    print(
        f"docker-exec-check: OK ({counts['unchecked']} unchecked <= floor {baseline}; "
        f"{len(report.members)} fail-open functions, {len(report.sites)} call sites in "
        f"{report.files} files)"
    )
    if counts["unchecked"] < baseline:
        print(
            f"  The count fell to {counts['unchecked']}: lower the baseline in "
            f"{BASELINE_DIR}/{BASELINE_NAME} in this change to keep the floor tight."
        )
    return 0


if __name__ == "__main__":
    sys.exit(main())
