#!/usr/bin/env python3
"""Read plan/journal/*.md at git HEAD, report problem classes with 2+ rows.

The journal is one file per problem class, each holding a markdown table
whose rows are occurrences.  Recurrence is the row count.  A second row
in a file is the alarm.

    python3 scripts/dev/journal.py
    python3 scripts/dev/journal.py --repo /path/to/checkout

Exit 0 when nothing to report.  Exit 1 on a malformed table or an
unparseable Date.  Exit 2 when the journal cannot be read at all.

Reading HEAD rather than the working tree is deliberate: a working-tree read
counts another session's unlanded rows.  It also means a journal that is not
committed yet is INVISIBLE to the count, so the three outcomes are kept apart --
an empty result is only legitimate when the working tree has no journal
either, and a git failure is an error rather than a silent zero.  A class file
on disk that HEAD does not carry is named on stderr, so the count is never read
as the whole journal.
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from datetime import date
from pathlib import Path

JOURNAL_DIR = "plan/journal"
JOURNAL_HEADER_CELLS = ("date", "spec", "surface", "symptom", "fix")
JOURNAL_SEPARATOR_RE = re.compile(r"^:?-{2,}:?$")

# The single cell a malformed row carries. `journal_row_cells()` returns
# [MALFORMED] rather than None so a caller cannot mistake a broken row for
# prose and skip it (AC-4: a malformed row is named, never skipped).
MALFORMED = "MALFORMED"


class JournalError(RuntimeError):
    """The journal could not be read, so no verdict about it is possible.

    Raised instead of returning an empty result, because "no rows" and "I
    could not look" are different answers and only one of them is green.
    """


def journal_row_cells(line: str) -> list[str] | None:
    """The five cells of a journal table row, or None for non-row lines.

    Returns None for prose and for the table header/separator.  A line that
    IS a row but does not split into exactly five cells returns [MALFORMED],
    and so does a row that lost its leading pipe: inside a table such a line
    is a row, and skipping it silently is what AC-4 forbids.  A journal file
    is repo-authored, so prose in one must not contain a `|`.

    This is the ONE implementation.  `commit_helper.py` and
    `spec-closure-check.py` import it, the way `deferral_orphans.py` imports
    `_deferral_row_cells` from `commit_helper`.
    """
    if "|" not in line:
        return None
    if not line.lstrip().startswith("|"):
        return [MALFORMED]
    fields = line.split("|")
    # A well-formed `| a | b | c | d | e |` splits into 7: leading and
    # trailing empties around the five cells.
    if len(fields) != 7 or fields[0].strip() or fields[-1].strip():
        return [MALFORMED]
    cells = [f.strip() for f in fields[1:6]]
    if tuple(c.lower() for c in cells) == JOURNAL_HEADER_CELLS:
        return None
    if all(JOURNAL_SEPARATOR_RE.match(c) for c in cells):
        return None
    return cells


# A Spec cell names the spec stems a row belongs to, or says the row belongs to
# no spec. `-` is the documented "no spec" (plan/journal/README.md); rows also
# write `none`, add a parenthesised note, and list two stems separated by a
# comma, so all four shapes are read here rather than at each call site.
JOURNAL_SPEC_STEM_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]*$")
JOURNAL_SPEC_NONE = ("", "-", "none", "n/a")
_JOURNAL_SPEC_NOTE_RE = re.compile(r"\s*\([^()]*\)\s*$")


def journal_spec_stems(cell: str) -> list[str] | None:
    """The spec stems a Spec cell names: [] for none, None when unreadable.

    A stem is the `plan/spec-<stem>.md` name, and it is a KEY: the gates look
    up `tmp/review/<stem>-<session>.md` under it. So the cell's prose must not
    reach them. `commit_helper.py` took the cell verbatim, and a row saying
    `none (walked into during <spec> closure)` asked the review gate for an
    artifact filed under that sentence. The gate then blocked a commit that
    owed no review, and its error named a path nobody could write.

    Four shapes are read, because the journal already holds all four:

    * `-`, `none`, an empty cell -> `[]`. The row belongs to no spec.
    * `some-spec` -> `["some-spec"]`.
    * `some-spec (measurement only)` -> `["some-spec"]`. A trailing
      parenthesised note is the author writing to the next reader, and it is
      never part of the key.
    * `spec-a, spec-b` -> both. One row can name two specs.

    Anything else returns None, which means UNREADABLE and does not mean "no
    spec". That difference is load-bearing: `commit_helper.journal_row_problems`
    blocks the commit on None, and only that block makes the skip in
    `_journal_added_spec_stems` safe. Read None as `[]` there and a cell nobody
    can parse takes the review gate off the commit that carries the code.
    """
    text = _JOURNAL_SPEC_NOTE_RE.sub("", cell.strip())
    stems: list[str] = []
    for token in text.split(","):
        token = token.strip()
        if token.lower() in JOURNAL_SPEC_NONE:
            continue
        if not JOURNAL_SPEC_STEM_RE.match(token):
            return None
        if token not in stems:
            stems.append(token)
    return stems


def journal_rows(text: str) -> list[list[str]]:
    """Every table row in one journal file's text."""
    rows: list[list[str]] = []
    for line in text.splitlines():
        cells = journal_row_cells(line)
        if cells is not None:
            rows.append(cells)
    return rows


def _git(repo: Path, *args: str) -> subprocess.CompletedProcess[str]:
    try:
        return subprocess.run(
            ["git", *args],
            capture_output=True,
            text=True,
            cwd=repo,
        )
    except OSError as exc:  # git absent, repo path gone
        raise JournalError(f"cannot run git in {repo}: {exc}") from exc


def _class_files_on_disk(repo: Path) -> list[str]:
    """Journal class files in the WORKING TREE, as repo-relative paths."""
    return sorted(
        p.relative_to(repo).as_posix()
        for p in (repo / JOURNAL_DIR).glob("*.md")
        if p.name != "README.md"
    )


def read_journal_at_head(repo: Path) -> tuple[dict[str, list[list[str]]], list[str]]:
    """Read plan/journal/*.md from git HEAD: the rows, and what was NOT read.

    Returns {class_name: rows} from HEAD, plus the class files the working tree
    holds that HEAD does not.  The second half is what keeps the first honest.
    This detector reads HEAD by design -- a working-tree read counts another
    session's unlanded rows -- so an uncommitted class file contributes nothing
    to the count, and the reader is told which files those were.

    Three outcomes, kept distinct:

    * HEAD carries journal CLASS files -> their rows, and the names of any class
      file on disk that HEAD does not carry.  `report()` prints those names and
      still exits 0.  A red here would fire on every session that opens a NEW
      problem class, at the moment `/ze-close` runs the pre-commit gate over the
      row it has just written and not yet committed, and the only way out would
      be to reuse a class that does not fit.  A guard that pressures a writer to
      misfile is worse than the miss it prevents; naming the file removes the
      SILENCE, which is the defect.
    * HEAD carries none AND the working tree holds no class file either -> {}.
      There is no journal, so zero rows is the truth.
    * HEAD carries none while the working tree holds class files -> JournalError.
      Nothing at all was read, so the verdict is vacuous and there is no output
      to carry the caveat.  A git invocation that fails raises for that reason
      too: "I could not look" is not "nothing to report".

    The on-disk comparison keys on `paths`, never on `names`.  `names` counts
    `README.md`, which carries no occurrence and is skipped everywhere else, so
    a HEAD holding the README and no class file made `names` non-empty, took the
    guard's early exit, and reported zero rows over a working tree full of
    classes.  That is the exact vacuous green the guard exists to refuse, and
    the README is committed one ordering step before the first class file.
    """
    listing = _git(repo, "ls-tree", "--name-only", "-r", "HEAD", "--", JOURNAL_DIR)
    if listing.returncode != 0:
        stderr = listing.stderr.strip() or f"exit {listing.returncode}"
        raise JournalError(f"git ls-tree HEAD -- {JOURNAL_DIR} failed: {stderr}")

    names = [n.strip() for n in listing.stdout.splitlines() if n.strip()]
    paths = [n for n in names if n.endswith(".md") and not n.endswith("/README.md")]
    on_disk = _class_files_on_disk(repo)
    if not paths:
        if on_disk:
            raise JournalError(
                f"{JOURNAL_DIR}/ exists in the working tree but HEAD carries no "
                "journal class file.\n"
                "  This detector reads HEAD, so it would count zero rows over an "
                "uncommitted journal\n"
                f"  and report no recurrence at all.  Commit {JOURNAL_DIR}/."
            )
        return {}, []
    unread = [path for path in on_disk if path not in set(paths)]

    result: dict[str, list[list[str]]] = {}
    for path in paths:
        content = _git(repo, "show", f"HEAD:{path}")
        if content.returncode != 0:
            stderr = content.stderr.strip() or f"exit {content.returncode}"
            raise JournalError(f"git show HEAD:{path} failed: {stderr}")
        rows = journal_rows(content.stdout)
        if rows:
            result[path[len(JOURNAL_DIR) + 1 :].removesuffix(".md")] = rows
    return result, unread


def _parse_date(cell: str) -> date | None:
    """The row's Date as a date, or None when the cell is not YYYY-MM-DD."""
    try:
        return date.fromisoformat(cell.strip())
    except (ValueError, AttributeError):
        return None


def report(repo: Path) -> int:
    """Print classes with 2+ rows.  Return non-zero on an unreadable row.

    An unparseable Date is an ERROR, not a footnote: the span is the only
    thing the report says beyond the count, and a class whose dates do not
    parse silently loses it.  A seed of 52 rows carrying `-` in every Date
    cell passed every gate before this returned non-zero.

    A class file on disk that HEAD does not carry is NAMED on stderr and does
    not fail the run.  The count below is over HEAD, so those rows are not in
    it; saying nothing would make the number read as the whole journal
    (`ai/rules/evidence.md`).
    """
    classes, unread = read_journal_at_head(repo)

    problems: list[str] = []
    lines: list[str] = []

    for class_name in sorted(classes):
        rows = classes[class_name]
        rel = f"{JOURNAL_DIR}/{class_name}.md"

        broken = sum(1 for row in rows if row == [MALFORMED])
        if broken:
            problems.append(
                f"MALFORMED: {rel}: {broken} row(s) do not hold the five cells "
                "| Date | Spec | Surface | Symptom | Fix |"
            )
            continue

        dates = [_parse_date(row[0]) for row in rows]
        unparseable = [row[0] for row, d in zip(rows, dates) if d is None]
        if unparseable:
            shown = ", ".join(repr(cell) for cell in unparseable[:3])
            problems.append(
                f"UNPARSEABLE DATE: {rel}: {shown} -- every row needs a "
                "YYYY-MM-DD Date, which is what the span is computed from"
            )
            continue

        if len(rows) < 2:
            continue

        first = min(d for d in dates if d is not None)
        last = max(d for d in dates if d is not None)
        lines.append(
            f"{class_name}: {len(rows)} rows, "
            f"{(last - first).days}d span ({first} .. {last})"
        )

    for line in lines:
        print(line)
    for path in unread:
        print(
            f"NOT AT HEAD: {path} is on disk and not committed, so its rows are "
            "not in the counts above",
            file=sys.stderr,
        )
    if problems:
        for problem in problems:
            print(problem, file=sys.stderr)
        return 1
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description="Report problem journal classes with 2+ rows.",
    )
    parser.add_argument(
        "--repo",
        default=None,
        help="repository root to read (default: this checkout)",
    )
    args = parser.parse_args(argv)

    repo = (
        Path(args.repo).resolve()
        if args.repo
        else Path(__file__).resolve().parent.parent.parent
    )
    try:
        return report(repo)
    except JournalError as exc:
        print(f"journal: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
