#!/usr/bin/env python3
"""Judge whether `test/weakened.md` accepts every test weakening a commit carries.

Spec: plan/spec-weakened-per-commit.md

WHAT THIS DECIDES
-----------------
A commit names a set of paths. For each test path among them this compares HEAD
against what the commit will carry, asks the hook's own detector which
weakenings that change makes, resolves each one to the name of the test that
holds it, and pairs those names against the rows of `test/weakened.md`.

Two failures cost the same and point opposite ways, so both are refused:

  * a weakening no row names   -- the record the reviewer reads is incomplete
  * a row no weakening matches -- a stale row from the previous commit, which
                                  would otherwise authorise the next one (R-2)

A commit that weakens nothing needs no rows, and its content is not read at all
(AC-5). That is what keeps a leftover file from the last commit out of the way
of unrelated work, and it is why this gate can BLOCK rather than warn.

WHAT THIS OWNS, AND WHAT IT BORROWS
-----------------------------------
It owns the pairing rule and the table reader. It owns neither of the two
judgements underneath:

  * WHICH CHANGES WEAKEN a test is `_test_weakening_errs`, imported from
    `.claude/hooks/pretool-writeedit.py` through `load_detector` in
    `scripts/dev/audit-test-relaxation.py`. The edit-time hook and this gate
    must never disagree about what a weakening is, and one function is the only
    way to guarantee that.
  * WHICH TEXT IS ONE TEST is `scripts/dev/rfc_tagged_scope.py`. Its own
    docstring records the failure a second copy would cause: two gates that
    drifted about which text a rule covers. Go resolves to the enclosing
    top-level func; every other carrier is one whole unit, so a `.ci` or `.et`
    file IS one test and its stem is its name.

BOTH HALVES OF THE DETECTOR ARE RECORDED
----------------------------------------
`_test_weakening_errs` returns `(blocking, advisory)`. The hook refuses on the
first and only reports the second, because a falling count reads the same as
consolidating three checks into one and refusing on it built a corpus of 601
excuses. This gate records both (owner decision, 2026-08-16): a per-commit file
cannot accumulate, so recording a count drop costs one line that is deleted with
the next commit, and dropping it silently is the thing AC-6 forbids.

THE POPULATION IS THE COMMIT, NEVER THE TREE
--------------------------------------------
Several agents share this checkout, so the working tree moves under whoever
reads it. Every path judged here comes from the caller, which is the commit's
own `--file` and `--remove` lists. Run with no paths, this checks only that
`test/weakened.md` still parses, which is a fact about a tracked file and true
for everyone.

Usage:
    check_weakened_tests.py --selftest              # internal tests
    check_weakened_tests.py                         # test/weakened.md parses
    check_weakened_tests.py PATH [PATH...]          # judge these paths
    check_weakened_tests.py --removed=PATH PATH     # a path the commit deletes

Exit 0 = nothing to fix. Exit 1 = problems, listed. Exit 2 = no comparison was
made, so no verdict is available: the shared detector would not import, or the
anchor does not resolve to a commit.
"""

import importlib.util
import os
import re
import subprocess
import sys
import tempfile
from typing import NamedTuple

RED = "\033[31m"
GREEN = "\033[32m"
BOLD = "\033[1m"
RESET = "\033[0m"

WEAKENED_PATH = "test/weakened.md"

# Prefix of a problem that means "no comparison happened", never "the commit is
# clean". The CLI turns it into exit 2, which is the code every other check here
# uses for "could not run" (`scripts/dev/audit-test-relaxation.py`).
_CANNOT_RUN = "check could not run: "

_HERE = os.path.dirname(os.path.abspath(__file__))
PROJECT_DIR = os.path.abspath(os.path.join(_HERE, "..", ".."))


def _load_by_path(name, filename):
    """Import a sibling in scripts/dev BY PATH, never through sys.path.

    Two of the three siblings this module needs cannot be imported any other
    way: `audit-test-relaxation.py` is not an identifier, and this module is
    itself imported by path from `.claude/hooks/` and from `commit_helper.py`,
    where `scripts/dev` is not on sys.path. `rfc_requirements.py` loads
    `rfc_tagged_scope` the same way and for the same reason.
    """
    spec = importlib.util.spec_from_file_location(name, os.path.join(_HERE, filename))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


rfc_tagged_scope = _load_by_path("rfc_tagged_scope", "rfc_tagged_scope.py")

_audit_module = None


def _audit():
    """`scripts/dev/audit-test-relaxation.py`, loaded on first use and kept.

    Lazy because `.claude/hooks/pretool-writeedit.py` imports this module on
    every Edit and runs under `python3 -S` to stay import-light. Executing a
    second module there buys that hook nothing: it already holds
    `_test_weakening_errs`, passes it as `detector=`, and never asks for the
    audit at all.
    """
    global _audit_module
    if _audit_module is None:
        _audit_module = _load_by_path(
            "ze_audit_test_relaxation", "audit-test-relaxation.py"
        )
    return _audit_module


# The two borrowed judgements, reached through the module that owns them so a
# caller of this one never writes a second spelling of either.
def is_test_path(path):
    """True when `path` is a test file this gate judges."""
    return _audit().is_test_path(path)


def load_detector(repo_root):
    """`_test_weakening_errs` from the canonical hook, or None."""
    return _audit().load_detector(repo_root)


class Row(NamedTuple):
    """One row of `test/weakened.md`."""

    name: str  # `TestXxx`, or `package.TestXxx` when a bare name is ambiguous
    reason: str  # what left the suite, and why the commit is correct without it
    line: int  # 1-based, for a message that tells the author where to look


class Weakened(NamedTuple):
    """One test a change set weakens."""

    path: str  # the file that holds it
    package: str  # the directory that holds the file, the row's qualifier
    name: str  # the enclosing Go func, or the file stem when there is none
    details: list  # what the detector said, blocking and advisory together


# --------------------------------------------------------------------------- #
# Reading test/weakened.md
# --------------------------------------------------------------------------- #

_HEADER = ("Test", "Reason")
_SEPARATOR_CELL = re.compile(r"^:?-{2,}:?$")


def _cells(line):
    """The cells of a markdown table row, or None when the line is not one."""
    body = line.strip()
    if not body.startswith("|"):
        return None
    parts = body.split("|")
    if parts and not parts[0].strip():
        parts = parts[1:]
    if parts and not parts[-1].strip():
        parts = parts[:-1]
    return [p.strip() for p in parts]


def parse_weakened_file(text):
    """(rows, problems) -- the `| Test | Reason |` table, and what is wrong with it.

    Anchored on the header rather than on "every table row", because the file
    carries prose tables too (the carrier table that documents how a name is
    resolved). A parser that read those would invent rows nobody wrote.

    Fails closed. A file with no such header yields no rows AND a problem, so a
    header that drifts refuses the commit instead of silently accepting every
    weakening in it.
    """
    lines = text.split("\n")
    problems = []
    starts = [i for i, line in enumerate(lines) if _cells(line) == list(_HEADER)]
    if not starts:
        return [], [
            f"{WEAKENED_PATH} has no `| Test | Reason |` table header, so no row "
            f"in it can be read"
        ]
    if len(starts) > 1:
        problems.append(
            f"{WEAKENED_PATH} has {len(starts)} `| Test | Reason |` tables; "
            f"keep one, or the gate has two answers"
        )
    rows = []
    seen = {}
    for i in range(starts[0] + 1, len(lines)):
        cells = _cells(lines[i])
        if cells is None:
            break
        if len(cells) != 2:
            problems.append(
                f"{WEAKENED_PATH}:{i + 1} has {len(cells)} cells; a row is "
                f"`| Test | Reason |`"
            )
            continue
        name, reason = cells
        filled = [c for c in cells if c]
        if filled and all(_SEPARATOR_CELL.match(c) for c in filled):
            continue  # the header's own `|------|--------|` separator
        # `filled` must be non-empty above, or `| | |` reads as a separator and
        # a row that names no test disappears instead of being refused.
        if not name:
            problems.append(f"{WEAKENED_PATH}:{i + 1} names no test")
            continue
        if not reason:
            problems.append(
                f"{WEAKENED_PATH}:{i + 1} gives no reason for {name}; a row with "
                f"no reason accepts nothing"
            )
            continue
        if name in seen:
            problems.append(
                f"{WEAKENED_PATH}:{i + 1} names {name} again (already on line "
                f"{seen[name]}); one test, one reason"
            )
            continue
        seen[name] = i + 1
        rows.append(Row(name, reason, i + 1))
    return rows, problems


# --------------------------------------------------------------------------- #
# Finding what a change set weakens
# --------------------------------------------------------------------------- #


def _kind(detail):
    """A detail line without its counts: what the detector found, not by how much.

    `removing assertions (5 -> 3)` and `removing assertions (12 -> 9)` are the
    same finding seen at two scopes, and the file-level one must not be reported
    a second time when a function already carries it.
    """
    return re.split(r"\s*[(;]", detail, maxsplit=1)[0].strip()


def _units_by_name(content):
    """{name: text} for the top-level Go funcs of `content`, in file order.

    Two funcs can share a name in one file (methods on different receivers), and
    their texts are joined rather than one being picked. Picking would judge text
    nobody chose; joining judges both, which can only over-report.
    """
    out = {}
    for name, text in rfc_tagged_scope.go_func_units(content):
        out.setdefault(name, []).append(text)
    return {name: "\n".join(texts) for name, texts in out.items()}


def weakened_units(path, old, new, detector):
    """[(name, details)] -- the tests `new` weakens, named as `test/weakened.md` names them.

    A non-Go carrier is one unit and its name is the file stem, which is
    `scope_reader`'s answer for it.

    Go is read twice, and both readings are kept. Per function, so a finding
    carries the name of the test that holds it. Then over the whole file, so a
    weakening that sits outside every function is not lost: an `ignore` build tag
    in the header drops the file from the build, and no function span covers it.
    Only the file-level KINDS no function already reported are added, or deleting
    one test would demand two rows.

    A Go weakening outside every function is named by the FILE STEM, the same way
    a `.ci` is. `test/weakened.md` publishes no name for that case, and the stem
    is the only one available: there is no enclosing func to name.
    """
    stem = os.path.splitext(os.path.basename(path))[0]
    file_errs, file_soft = detector(old, new, path)
    file_details = list(file_errs) + list(file_soft)
    if rfc_tagged_scope.scope_reader(path) != "go":
        return [(stem, file_details)] if file_details else []

    old_units = _units_by_name(old)
    new_units = _units_by_name(new)
    found = []
    named_kinds = set()
    for name, old_text in old_units.items():
        if not old_text.strip():
            continue
        errs, soft = detector(old_text, new_units.get(name, ""), path)
        details = list(errs) + list(soft)
        if not details:
            continue
        found.append((name or stem, details))
        named_kinds.update(_kind(d) for d in details)
    residual = [d for d in file_details if _kind(d) not in named_kinds]
    if residual:
        found.append((stem, residual))
    return found


def _git(args, cwd):
    return subprocess.run(["git", *args], cwd=cwd, capture_output=True, text=True)


def _head_text(repo_root, path, anchor):
    """(text, err) -- what `anchor` holds at `path`.

    A path absent at the anchor is "" and no error: the commit adds it, and a new
    test weakens nothing. EVERY other failure is an error rather than "", because
    "" reads as a new file and would accept every weakening in it in silence.
    That is the zero-value trap `ai/rules/evidence.md` names, and a gate is the
    one place it must not be sprung.
    """
    if _git(["rev-parse", "--verify", f"{anchor}^{{commit}}"], repo_root).returncode:
        return "", f"{anchor} does not resolve to a commit, so nothing was compared"
    if _git(["cat-file", "-e", f"{anchor}:{path}"], repo_root).returncode:
        return "", None  # not at the anchor: this commit adds it
    out = _git(["show", f"{anchor}:{path}"], repo_root)
    if out.returncode:
        return "", f"git show {anchor}:{path} failed: {out.stderr.strip()}"
    return out.stdout, None


def _worktree_text(repo_root, path):
    """The file's content, or "" when it is not there.

    "" is honest here: a path the commit names that is absent from the tree is a
    deletion, and a deletion of a test is exactly what this gate must report.
    """
    try:
        with open(
            os.path.join(repo_root, path), encoding="utf-8", errors="replace"
        ) as fh:
            return fh.read()
    except OSError:
        return ""


def weakened_tests(repo_root, paths, removed=(), detector=None, anchor="HEAD"):
    """(weakened, errors) -- every test the named paths weaken, against `anchor`.

    `paths` and `removed` are the commit's own lists. Nothing else is read, so a
    concurrent session's edit to a file this commit does not name is invisible
    here, which is what lets the caller BLOCK on the result.

    A non-empty `errors` means the comparison did not happen and the empty
    `weakened` beside it says nothing about the commit. The caller MUST report
    the errors rather than read the list as a clean bill of health.
    """
    if detector is None:
        detector = load_detector(PROJECT_DIR)
    if detector is None:
        return [], [
            f"{_CANNOT_RUN}_test_weakening_errs is not importable from "
            f".claude/hooks/pretool-writeedit.py, so no weakening could be judged"
        ]
    out = []
    errors = []
    seen = []
    for path in list(paths) + list(removed):
        if path in seen or not is_test_path(path):
            continue
        seen.append(path)
        old, err = _head_text(repo_root, path, anchor)
        if err:
            errors.append(f"{_CANNOT_RUN}{err}")
            continue
        if not old.strip():
            continue  # added by this commit: a new test weakens nothing
        new = "" if path in removed else _worktree_text(repo_root, path)
        package = os.path.basename(os.path.dirname(path))
        for name, details in weakened_units(path, old, new, detector):
            out.append(Weakened(path, package, name, details))
    return out, errors


# --------------------------------------------------------------------------- #
# Pairing rows against weakenings
# --------------------------------------------------------------------------- #


def row_matches(row_name, weak):
    """True when `row_name` names `weak`.

    A bare name matches on the test name alone. A qualified `package.TestName`
    matches only when the directory holding the file is that package, which is
    the spelling `test/weakened.md` publishes for the ambiguous case.
    """
    qualifier, _, bare = row_name.rpartition(".")
    if qualifier:
        return weak.name == bare and weak.package == qualifier
    return weak.name == row_name


def _row_to_write(weak, qualify):
    name = f"{weak.package}.{weak.name}" if qualify else weak.name
    return (
        f"| {name} | <what left the suite, and why the commit is correct without it> |"
    )


def unmatched_problems(rows, weakened):
    """[problem] -- every row and every weakening that does not pair one to one.

    Ambiguity is refused rather than resolved (AC-7). One bare name over two
    weakened tests is not an acceptance of both: the reader cannot tell which
    reason belongs to which, so the row is asked to qualify itself.
    """
    problems = []
    claimed = set()
    for row in rows:
        hits = [i for i, w in enumerate(weakened) if row_matches(row.name, w)]
        if not hits:
            problems.append(
                f"{WEAKENED_PATH}:{row.line} names {row.name}, which this commit "
                f"does not weaken. A row left over from the last commit accepts "
                f"nothing here; delete it."
            )
            continue
        if len(hits) > 1:
            hit = [weakened[i] for i in hits]
            where = ", ".join(f"{w.package} ({w.path})" for w in hit)
            problems.append(
                f"{WEAKENED_PATH}:{row.line} names {row.name}, which this commit "
                f"weakens in {len(hit)} packages: {where}.\n"
                f"    Write package.TestName, one row each:\n"
                + "\n".join(f"    {_row_to_write(w, True)}" for w in hit)
            )
        # Claimed even when ambiguous: the row above already names every
        # weakening it hit, and reporting each one again as unrecorded would bury
        # the one problem the author has to fix.
        claimed.update(hits)
    for index, weak in enumerate(weakened):
        if index in claimed:
            continue
        detail = "\n".join(f"    - {d}" for d in weak.details)
        qualify = sum(1 for w in weakened if w.name == weak.name) > 1
        problems.append(
            f"{weak.path} weakens {weak.name} and {WEAKENED_PATH} has no row for "
            f"it:\n{detail}\n"
            f"    Add the row, then commit the file with the change:\n"
            f"    {_row_to_write(weak, qualify)}"
        )
    return problems


def weakened_problems(repo_root, paths, removed=(), detector=None, anchor="HEAD"):
    """[problem] -- why `test/weakened.md` does not accept this change set.

    The whole gate, and what `scripts/dev/commit_helper.py` calls with the
    commit's own `--file` and `--remove` lists. An empty list means every
    weakening in the named paths has exactly one row, and every row names a
    weakening.

    A caller that already holds `_test_weakening_errs` passes it as `detector`.
    `.claude/hooks/pretool-writeedit.py` is that caller: without it this module
    would import the hook back through `load_detector`, and a hook importing a
    module that re-executes the hook does not terminate.
    """
    weakened, errors = weakened_tests(repo_root, paths, removed, detector, anchor)
    if errors:
        return errors
    if not weakened:
        return []  # AC-5: nothing to accept, so the file's content is not read
    text = _read_contract(repo_root)
    if text is None:
        listed = "\n".join(f"    {_row_to_write(w, False)}" for w in weakened)
        return [
            f"{WEAKENED_PATH} does not exist, and this commit weakens "
            f"{len(weakened)} test(s). Write it:\n{listed}"
        ]
    rows, problems = parse_weakened_file(text)
    return problems + unmatched_problems(rows, weakened)


def _read_contract(repo_root):
    full = os.path.join(repo_root, WEAKENED_PATH)
    if not os.path.isfile(full):
        return None
    with open(full, encoding="utf-8", errors="replace") as fh:
        return fh.read()


def file_shape_problems(repo_root=PROJECT_DIR):
    """[problem] -- ways `test/weakened.md` would not be readable by the two gates.

    What `make ze-weakened-check` runs, and it deliberately reads no source file.
    Whether a commit's weakenings are covered is a question about that commit's
    paths, and a verify stage has none. What a verify stage CAN answer is whether
    the file the commit gate depends on still parses, which is a fact about a
    tracked file and the same for every session in this checkout.
    """
    text = _read_contract(repo_root)
    if text is None:
        return [
            f"{WEAKENED_PATH} is missing. The commit gate reads it, so a commit "
            f"that weakens a test has nowhere to record the reason."
        ]
    _, problems = parse_weakened_file(text)
    return problems


# --------------------------------------------------------------------------- #
# Selftest and CLI
# --------------------------------------------------------------------------- #

_SELFTEST_BASE = """package a

import "testing"

func TestA(t *testing.T) {
\trequire.Equal(t, 1, f())
\trequire.NoError(t, err)
}
"""

_SELFTEST_WEAK = """package a

import "testing"

func TestA(t *testing.T) {
\tt.Skip("later")
\trequire.Equal(t, 1, f())
\trequire.NoError(t, err)
}
"""

_SELFTEST_TABLE = "| Test | Reason |\n|------|--------|\n"


def selftest(quiet=False):
    """Prove the checker fires on fixtures whose answer is known.

    Runs before the real check in `make ze-weakened-check`, so a checker that
    reported nothing because it was broken cannot report a clean file.
    """
    failures = []

    def check(condition, what):
        if not condition:
            failures.append(what)

    with tempfile.TemporaryDirectory(prefix="ze-weakened-selftest-") as work:
        env = [
            "-c",
            "user.email=t@t",
            "-c",
            "user.name=t",
            "-c",
            "commit.gpgsign=false",
        ]
        _git(["init", "-q"], work)
        os.makedirs(os.path.join(work, "pkg"))
        os.makedirs(os.path.join(work, "test"))

        def write(rel, text):
            with open(os.path.join(work, rel), "w", encoding="utf-8") as fh:
                fh.write(text)

        write("pkg/a_test.go", _SELFTEST_BASE)
        write(WEAKENED_PATH, _SELFTEST_TABLE)
        _git(["add", "-A"], work)
        _git([*env, "commit", "-q", "-m", "baseline"], work)

        write("pkg/a_test.go", _SELFTEST_WEAK)
        missing = weakened_problems(work, ["pkg/a_test.go"])
        check(missing, "a skipped test with no row must be refused")
        check(
            any("TestA" in p for p in missing),
            "the refusal must name the enclosing test",
        )

        write(
            WEAKENED_PATH,
            _SELFTEST_TABLE + "| TestA | the feature it drove is gone |\n",
        )
        check(
            weakened_problems(work, ["pkg/a_test.go"]) == [],
            "a row naming the weakened test must be accepted",
        )

        write(WEAKENED_PATH, _SELFTEST_TABLE + "| TestGone | stale |\n")
        check(
            weakened_problems(work, ["pkg/a_test.go"]),
            "a row naming an untouched test must be refused",
        )

        write(WEAKENED_PATH, "# Tests\n\nno table\n")
        check(file_shape_problems(work), "a file with no table header must be refused")

    if not quiet:
        if failures:
            print(f"{RED}SELFTEST FAIL{RESET}")
            for f in failures:
                print(f"  - {f}")
        else:
            print("SELFTEST PASS")
    return 1 if failures else 0


def _repo_root():
    top = _git(["rev-parse", "--show-toplevel"], os.getcwd()).stdout.strip()
    return top or os.getcwd()


def main(argv):
    if "--selftest" in argv:
        return selftest()
    removed = [a.split("=", 1)[1] for a in argv[1:] if a.startswith("--removed=")]
    paths = [a for a in argv[1:] if not a.startswith("-")]
    root = _repo_root()

    if not paths and not removed:
        problems = file_shape_problems(root)
        if not problems:
            rows, _ = parse_weakened_file(_read_contract(root) or "")
            print(
                f"{GREEN}Weakened-test check: {WEAKENED_PATH} parses "
                f"({len(rows)} row(s)).{RESET}"
            )
            return 0
    else:
        problems = weakened_problems(root, paths, removed)
        blocked = [p for p in problems if p.startswith(_CANNOT_RUN)]
        if blocked:
            print(
                f"{RED}{BOLD}Weakened-test check: CANNOT RUN.{RESET}", file=sys.stderr
            )
            for problem in blocked:
                print(f"  {problem}", file=sys.stderr)
            return 2
        if not problems:
            # The verdict names what was actually READ. A count of every path
            # given would read as a comparison over paths this check skips,
            # which is how a pass stops being evidence (ai/rules/evidence.md).
            given = list(dict.fromkeys(list(paths) + list(removed)))
            judged = [p for p in given if is_test_path(p)]
            print(
                f"{GREEN}Weakened-test check: clean ({len(judged)} of "
                f"{len(given)} path(s) are tests, judged against HEAD).{RESET}"
            )
            return 0

    print(
        f"{RED}{BOLD}Weakened-test check: {len(problems)} problem(s).{RESET}\n",
        file=sys.stderr,
    )
    for problem in problems:
        print(f"  {problem}", file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv))
