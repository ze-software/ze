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
It owns the pairing rule and the table reader. It owns none of the three
judgements underneath:

  * WHICH STRUCTURAL CHANGES WEAKEN a test is `_test_weakening_errs`, imported
    from `.claude/hooks/pretool-writeedit.py` through `load_detector` in
    `scripts/dev/audit-test-relaxation.py`.
  * WHICH RFC TAGS LEFT a test is `_rfc_tagged_change_err`, imported through
    that same module. The checker uses only tags absent from the new text.
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
import io
import os
import re
import subprocess
import sys
import tempfile
import tokenize
from typing import NamedTuple

RED = "\033[31m"
GREEN = "\033[32m"
BOLD = "\033[1m"
RESET = "\033[0m"

WEAKENED_PATH = "test/weakened.md"

# The second per-commit ledger, and the reason the reader below takes its path as
# an argument. `test/rfc-changed.md` records the OWNER's approval of a change to
# an RFC-tagged test, where `test/weakened.md` records the AUTHOR's own reason for
# a weakening. The two differ in who writes a row and in nothing a parser can see,
# so one table reader serves both and neither gate can invent a second spelling of
# `package.TestName`.
RFC_CHANGED_PATH = "test/rfc-changed.md"

# Prefix of a problem that means "no comparison happened", never "the commit is
# clean". The CLI turns it into exit 2, which is the code every other check here
# uses for "could not run" (`scripts/dev/audit-test-relaxation.py`).
_CANNOT_RUN = "check could not run: "

_HERE = os.path.dirname(os.path.abspath(__file__))
PROJECT_DIR = os.path.abspath(os.path.join(_HERE, "..", ".."))


def _load_by_path(name, filename):
    """Import a sibling in scripts/dev BY PATH, never through sys.path.

    `audit-test-relaxation.py` cannot be imported any other way: the name is not
    an identifier. `rfc_tagged_scope` is loaded the same way because this module
    is itself loaded by path from `.claude/hooks/pretool-writeedit.py`, where
    `scripts/dev` is not on sys.path and a plain import of either would fail.
    `rfc_requirements.py` loads `rfc_tagged_scope` this way for the same reason.

    `scripts/dev/commit_helper.py` needs none of that: it LIVES here, so running
    it puts this directory on sys.path and it imports this module by name.
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


def load_rfc_detector(repo_root):
    """`_rfc_tagged_change_err` from the canonical hook, or None."""
    return _audit().load_rfc_detector(repo_root)


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



class RenamePair(NamedTuple):
    """One Git-detected rename in the prospective commit."""

    old_path: str
    new_path: str
    score: int


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


def parse_weakened_file(text, path=WEAKENED_PATH):
    """(rows, problems) -- the `| Test | Reason |` table, and what is wrong with it.

    Anchored on the header rather than on "every table row", because the file
    carries prose tables too (the carrier table that documents how a name is
    resolved). A parser that read those would invent rows nobody wrote.

    Fails closed. A file with no such header yields no rows AND a problem, so a
    header that drifts refuses the commit instead of silently accepting every
    weakening in it.

    `path` names the file in every problem, so `test/rfc-changed.md` reports its
    own name through this one reader.
    """
    lines = text.split("\n")
    problems = []
    starts = [i for i, line in enumerate(lines) if _cells(line) == list(_HEADER)]
    if not starts:
        return [], [
            f"{path} has no `| Test | Reason |` table header, so no row "
            f"in it can be read"
        ]
    if len(starts) > 1:
        problems.append(
            f"{path} has {len(starts)} `| Test | Reason |` tables; "
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
                f"{path}:{i + 1} has {len(cells)} cells; a row is `| Test | Reason |`"
            )
            continue
        name, reason = cells
        filled = [c for c in cells if c]
        if filled and all(_SEPARATOR_CELL.match(c) for c in filled):
            continue  # the header's own `|------|--------|` separator
        # `filled` must be non-empty above, or `| | |` reads as a separator and
        # a row that names no test disappears instead of being refused.
        if not name:
            problems.append(f"{path}:{i + 1} names no test")
            continue
        if not reason:
            problems.append(
                f"{path}:{i + 1} gives no reason for {name}; a row with "
                f"no reason accepts nothing"
            )
            continue
        if name in seen:
            problems.append(
                f"{path}:{i + 1} names {name} again (already on line "
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


_COUNT_DROP_KINDS = (
    "removing assertions",
    "removing t.Run cases",
    "removing expectations",
    "removing negative expectations",
)
_COUNT_DROP_RE = re.compile(
    r"^(removing (?:assertions|t\.Run cases|expectations|negative expectations)) "
    r"\((\d+) -> (\d+)"
)


def _reported_count_drops(details):
    """Positive detector count deltas keyed by their stable finding kind."""
    drops = {}
    for detail in details:
        match = _COUNT_DROP_RE.match(detail)
        if match:
            drops[match.group(1)] = int(match.group(2)) - int(match.group(3))
    return drops


def _count_deltas(old, new, path, detector):
    """Signed detector count deltas, including counts hidden by an empty side."""
    marker = "\n// retained count scope\n"

    def drops(left, right):
        blocking, advisory = detector(left + marker, right + marker, path)
        return _reported_count_drops(list(blocking) + list(advisory))

    forward = drops(old, new)
    reverse = drops(new, old)
    return {
        kind: forward.get(kind, 0) - reverse.get(kind, 0)
        for kind in _COUNT_DROP_KINDS
    }


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


def executable_test_text(path, text):
    """Text the lexical weakening detector must judge for one test path.

    Python tests routinely carry source snippets as fixture data. Mask every
    STRING token's contents so fixture text such as ``self.skipTest("flaky")``
    is not treated as executable source. Token positions and line endings stay
    intact. If tokenization cannot finish, return the original text so malformed
    source fails closed rather than hiding a weakening.

    This normalization belongs only to weakening detection. RFC-tagged change
    detection must continue to inspect the raw source.
    """
    if not path.endswith(".py"):
        return text

    line_starts = [0]
    for line in text.splitlines(keepends=True):
        line_starts.append(line_starts[-1] + len(line))

    spans = []
    try:
        tokens = tokenize.generate_tokens(io.StringIO(text).readline)
        for token in tokens:
            if token.type != tokenize.STRING:
                continue
            start = line_starts[token.start[0] - 1] + token.start[1]
            end = line_starts[token.end[0] - 1] + token.end[1]
            spans.append((start, end))
    except (IndentationError, tokenize.TokenError):
        return text

    if not spans:
        return text

    masked = []
    cursor = 0
    for start, end in spans:
        masked.append(text[cursor:start])
        masked.append(
            "".join(char if char in "\r\n" else " " for char in text[start:end])
        )
        cursor = end
    masked.append(text[cursor:])
    return "".join(masked)


def weakened_units(path, old, new, detector, rfc_detector=None):
    """[(name, details)] -- the tests `new` weakens, named as `test/weakened.md` names them.

    A non-Go carrier is one unit and its name is the file stem, which is
    `scope_reader`'s answer for it.

    Go is read twice, and both readings are kept. Per function, so a finding
    carries the name of the test that holds it. Then over the whole file, so a
    weakening in the header or package scope is not lost. Count findings use
    signed deltas: top-level-function deltas are subtracted from the file delta,
    and only a positive outside-function remainder uses the file stem.

    A Go weakening outside every function is named by the FILE STEM, the same way
    a `.ci` is. `test/weakened.md` publishes no name for that case, and the stem
    is the only one available: there is no enclosing func to name.
    """
    old = executable_test_text(path, old)
    new = executable_test_text(path, new)
    stem = os.path.splitext(os.path.basename(path))[0]

    def judge(old_text, new_text):
        errs, soft = detector(old_text, new_text, path)
        details = list(errs) + list(soft)
        if rfc_detector is None:
            return details
        tags = rfc_detector(old_text, new_text, path) or ()
        dropped = [tag for tag in tags if tag not in new_text]
        if dropped:
            details.append(
                f"removing RFC requirement tags ({len(dropped)}): "
                + ", ".join(sorted(dropped))
            )
        return details

    file_details = judge(old, new)
    if rfc_tagged_scope.scope_reader(path) != "go":
        return [(stem, file_details)] if file_details else []

    old_units = _units_by_name(old)
    new_units = _units_by_name(new)
    file_count_deltas = _count_deltas(old, new, path, detector)
    unit_count_deltas = {kind: 0 for kind in _COUNT_DROP_KINDS}
    for name in dict.fromkeys((*old_units, *new_units)):
        deltas = _count_deltas(
            old_units.get(name, ""), new_units.get(name, ""), path, detector
        )
        for kind, delta in deltas.items():
            unit_count_deltas[kind] += delta

    found = []
    named_kinds = set()
    for name, old_text in old_units.items():
        if not old_text.strip():
            continue
        details = judge(old_text, new_units.get(name, ""))
        if not details:
            continue
        found.append((name or stem, details))
        named_kinds.update(_kind(d) for d in details)

    residual = [
        detail
        for detail in file_details
        if _kind(detail) not in _COUNT_DROP_KINDS
        and _kind(detail) not in named_kinds
    ]
    for kind in _COUNT_DROP_KINDS:
        outside = file_count_deltas[kind] - unit_count_deltas[kind]
        if outside > 0:
            residual.append(f"{kind} outside top-level functions ({outside})")
    if residual:
        found.append((stem, residual))
    return found


def rfc_changed_units(path, old, new, detector, tag_re):
    """[(name, tags, old_text, new_text)] -- the tagged tests `new` changes, named as the ledger names them.

    The sibling of `weakened_units`, for the other ledger. `detector` is
    `_rfc_tagged_change_err` from the canonical hook, so what counts as a
    behavior change is decided once: a reformat, a comment edit and a Go
    import-only edit are not one, and a rename is. The edit-time hook and the
    commit gate both call this, so neither can disagree with the other about
    which unit a row has to name.

    A non-Go carrier is one unit and its name is the file stem, which is
    `scope_reader`'s answer for it.

    Go resolves to the enclosing top-level func, so changing an untagged test
    beside a tagged one owes nothing. The one exception is a tag no func span
    covers, a hoisted table or a tag separated from its func by a blank line:
    `tag_scope` falls back to the whole file there because a narrower answer
    would be a guess, and this falls back with it. More checking, never less
    (`ai/rules/evidence.md`).

    `old_text` and `new_text` come back so a caller can say what the comparison
    saw. This function judges; it writes no message.
    """
    stem = os.path.splitext(os.path.basename(path))[0]

    def judge(name, old_text, new_text):
        tags = detector(old_text, new_text, path)
        if not tags:
            return None
        return (name, tuple(sorted(set(tags))), old_text, new_text)

    if rfc_tagged_scope.scope_reader(path) != "go":
        found = judge(stem, old, new)
        return [found] if found else []

    spans = rfc_tagged_scope.go_func_scopes(old)
    if any(not any(a <= m.start() < b for a, b in spans) for m in tag_re.finditer(old)):
        found = judge(stem, old, new)
        return [found] if found else []

    old_units = _units_by_name(old)
    new_units = _units_by_name(new)
    out = []
    for name, old_text in old_units.items():
        if not old_text.strip():
            continue
        found = judge(name or stem, old_text, new_units.get(name, ""))
        if found:
            out.append(found)
    return out


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


def _rename_maps(paths, removed, rename_pairs):
    """({old: pair}, {new}, errors) for an unambiguous commit-local pairing."""
    added = set(paths)
    deleted = set(removed)
    by_old = {}
    by_new = {}
    errors = []
    for pair in rename_pairs:
        if pair.old_path not in deleted or pair.new_path not in added:
            errors.append(
                f"{_CANNOT_RUN}rename {pair.old_path} -> {pair.new_path} is outside "
                "the commit's exact add/remove population"
            )
            continue
        if pair.old_path in by_old:
            errors.append(
                f"{_CANNOT_RUN}rename pairing is ambiguous: {pair.old_path} maps "
                f"to both {by_old[pair.old_path].new_path} and {pair.new_path}"
            )
            continue
        if pair.new_path in by_new:
            errors.append(
                f"{_CANNOT_RUN}rename pairing is ambiguous: {pair.new_path} maps "
                f"from both {by_new[pair.new_path].old_path} and {pair.old_path}"
            )
            continue
        if not isinstance(pair.score, int) or not 1 <= pair.score <= 100:
            errors.append(
                f"{_CANNOT_RUN}rename {pair.old_path} -> {pair.new_path} has "
                f"invalid Git similarity score {pair.score!r}"
            )
            continue
        by_old[pair.old_path] = pair
        by_new[pair.new_path] = pair
    return by_old, set(by_new), errors


def _renamed_worktree_text(repo_root, pair):
    """(text, error) for the new side of a rename, with read failure visible."""
    try:
        with open(
            os.path.join(repo_root, pair.new_path),
            encoding="utf-8",
            errors="replace",
        ) as fh:
            return fh.read(), None
    except OSError as exc:
        return "", (
            f"{_CANNOT_RUN}rename {pair.old_path} -> {pair.new_path} was paired "
            f"at {pair.score}% similarity, but the new path could not be read: {exc}"
        )


def weakened_tests(
    repo_root,
    paths,
    removed=(),
    detector=None,
    anchor="HEAD",
    rename_pairs=(),
    rfc_detector=None,
):
    """(weakened, errors) -- every test the named paths weaken, against `anchor`.

    `paths`, `removed`, and `rename_pairs` describe the commit's exact
    population. A rename compares the old path at `anchor` with the new path in
    the worktree. The finding uses the new path as its identity. Nothing else is
    read, so a concurrent session's unrelated edit is invisible here.

    A non-empty `errors` means that the comparison did not happen. The empty
    `weakened` beside it says nothing about the commit. The caller MUST report
    the errors rather than read the list as a clean bill of health.
    """
    paths = tuple(paths)
    removed = tuple(removed)
    renames, renamed_new_paths, errors = _rename_maps(
        paths, removed, rename_pairs
    )
    if errors:
        return [], errors
    if detector is None:
        detector = load_detector(PROJECT_DIR)
    if detector is None:
        return [], [
            f"{_CANNOT_RUN}_test_weakening_errs is not importable from "
            f".claude/hooks/pretool-writeedit.py, so no weakening could be judged"
        ]
    if rfc_detector is None:
        rfc_detector = load_rfc_detector(PROJECT_DIR)
    if rfc_detector is None:
        return [], [
            f"{_CANNOT_RUN}_rfc_tagged_change_err is not importable from "
            f".claude/hooks/pretool-writeedit.py, so no RFC tag loss could be judged"
        ]
    out = []
    errors = []
    seen = set()
    for path in paths + removed:
        if path in seen or path in renamed_new_paths or not is_test_path(path):
            continue
        seen.add(path)
        old, err = _head_text(repo_root, path, anchor)
        if err:
            errors.append(f"{_CANNOT_RUN}{err}")
            continue
        if not old.strip():
            continue  # added by this commit: a new test weakens nothing
        pair = renames.get(path)
        comparison_path = pair.new_path if pair else path
        if pair:
            new, err = _renamed_worktree_text(repo_root, pair)
            if err:
                errors.append(err)
                continue
        else:
            new = "" if path in removed else _worktree_text(repo_root, path)
        package = os.path.basename(os.path.dirname(comparison_path))
        for name, details in weakened_units(
            comparison_path, old, new, detector, rfc_detector
        ):
            out.append(Weakened(comparison_path, package, name, details))
    if errors:
        return [], errors
    return out, []


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


class _Pairing(NamedTuple):
    """The two fields `row_matches` reads, for a caller that holds nothing else."""

    package: str
    name: str


def rows_missing(rows, package, names):
    """[name] -- every name in `names` that no row in `rows` names, in order.

    The pairing rule for a caller that holds names rather than findings, which
    both edit-time gates do: they know which test the edit touches and have no
    finding object to hand. `row_matches` stays the ONE definition of "this row
    names that test", the `package.TestName` qualifier included, so a hook that
    accepted a row the commit gate refuses is not expressible here.
    """
    return [
        name
        for name in names
        if not any(row_matches(row.name, _Pairing(package, name)) for row in rows)
    ]


def _row_to_write(weak, qualify):
    name = f"{weak.package}.{weak.name}" if qualify else weak.name
    return (
        f"| {name} | <what left the suite, and why the commit is correct without it> |"
    )


def unmatched_problems(rows, weakened, ledger_carried=True):
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
        missing = (
            f"{WEAKENED_PATH} has no row for it"
            if ledger_carried
            else "this commit has no carried row for it"
        )
        problems.append(
            f"{weak.path} weakens {weak.name} and {missing}:\n{detail}\n"
            f"    Add the row, then commit the file with the change:\n"
            f"    {_row_to_write(weak, qualify)}"
        )
    return problems


def weakened_problems(
    repo_root,
    paths,
    removed=(),
    detector=None,
    anchor="HEAD",
    rename_pairs=(),
):
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
    weakened, errors = weakened_tests(
        repo_root, paths, removed, detector, anchor, rename_pairs
    )
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

    What `make ze-test-weakened-check` runs, and it deliberately reads no source file.
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

    Runs before the real check in `make ze-test-weakened-check`, so a checker that
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
