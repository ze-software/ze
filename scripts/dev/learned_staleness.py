#!/usr/bin/env python3
"""Detect decay in plan/learned/: dead `## Files` paths and dead NNN citations.

A learned summary is hand-written once and never regenerated, so the paths it
cites rot silently as code moves and old summaries are retired. Nothing detected
either until this gate: 1,856 of 8,506 cited paths (22%) were already gone when
it landed.

What it reports, one finding per bad reference:

  * A path in a `## Files` section that does not resolve.
  * A `plan/learned/NNN` citation, anywhere in the summary, that names no
    surviving summary.
  * A summary with no `## Files` section at all, or one that cannot be read.
    That is a FINDING, never a silent pass (`ai/rules/evidence.md`):
    an empty finding list must mean "every summary was read and every reference
    resolved", never "nothing could be read".

Section parsing reads `## Files` AND every `## Files <qualifier>` section
(`## Files Modified`, `## Files Created`, ...). Three summaries keep a second
qualified section holding 12 paths that a parser reading only the exact heading
would skip. The fix belongs in this parser rather than in those three files, so
corpus uniformity is never a precondition for the gate being correct
(`ai/rules/completion.md`).

Path safety (spec Security Review): a token containing `..` is REPORTED as
invalid rather than resolved, a token whose real path escapes the repository
root is reported the same way, and nothing read is ever executed.

The tree is the source of truth, never `ai/LEARNED-FULL-INDEX.md`: a stale
generated index must not be able to make the gate vacuous.

Baseline (`plan/.learned-staleness-baseline`): a shrink-only ceiling on the
finding count, the `plan/.citation-baseline` idiom. More findings than the
ceiling fails; fewer is reported, and `--write-baseline` tightens the ceiling to
it. No other invocation writes the file: this gate runs inside `ze-doc-test`
inside `ze-verify`, and a check that mutates a tracked file cannot be run freely
in a checkout several sessions share. The corpus cannot start clean,
so the alternative was landing a gate that is red on day one and gets switched
off. A count ceiling cannot name which finding is new when one is fixed and one
is added in the same change; that is the cost of the idiom, shared with the
`test/.ci-sleep-baseline` ratchet.

Usage:
    python3 scripts/dev/learned_staleness.py            # report, exit 1 over baseline
    python3 scripts/dev/learned_staleness.py --check    # alias, for make-target symmetry
    python3 scripts/dev/learned_staleness.py --json     # machine-readable report
    python3 scripts/dev/learned_staleness.py --write-baseline   # tighten the ceiling
    python3 scripts/dev/learned_staleness.py --raise-baseline "<why>"  # re-bless a rise
"""

from __future__ import annotations

import argparse
import glob as globmod
import json
import os
import re
import sys
from pathlib import Path

# The repo's one definition of "which backtick token is a path" lives in the
# doc-links checker; deriving from it keeps the two gates from disagreeing
# about the same corpus (`ai/rules/evidence.md`).
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from check_doc_links import (
    KNOWN_ROOTS,
    LINE_SUFFIX,
    ROOT_FILES,
    SYMBOL_COLON,
    SYMBOL_DOT,
    expand_braces,
)

LEARNED_DIR = ("plan", "learned")
BASELINE_REL = "plan/.learned-staleness-baseline"

BACKTICK = re.compile(r"`([^`]+)`")
# A `{a,b}` alternation or a `{001..011}` range. Stripped before the traversal
# test so a range's `..` is not misread as a parent-directory reference.
BRACE_GROUP = re.compile(r"\{[^{}]*\}")
# `plan/learned/734`, `plan/learned/734-slug.md`: resolved by NUMBER, because a
# renamed summary at the same number still carries the knowledge cited.
CITATION = re.compile(r"plan/learned/(\d+)")
# Build/runtime artifacts: presence is a property of the checkout, not of the
# reference. Same set the doc-links checker skips.
SKIP_PREFIXES = ("tmp/", "bin/", "test/tmp/", "/", "~")
# Templates, not concrete paths. `..` is deliberately NOT here: it is reported.
PLACEHOLDER_MARKERS = ("<", ">", "$", "*", "NNN", "...")

# Step 5 of plan/spec-knowledge-1-corpus.md implemented `check`; the flag stays
# so `--json` consumers can tell a real report from the phase-1 stub's.
IMPLEMENTED = True


def summary_files(root: Path) -> list[Path]:
    """Every numbered learned summary, in number order.

    `README.md`, `METHODOLOGY.md`, `DESIGN-HISTORY.md` and the other named files
    in the directory are not summaries and carry no `## Files` section, so the
    glob is deliberately numeric-only."""
    d = root.joinpath(*LEARNED_DIR)
    if not d.is_dir():
        return []
    return sorted(p for p in d.glob("[0-9]*.md"))


def files_section_lines(text: str) -> list[tuple[int, str]]:
    """(1-based lineno, line) for every line inside a `## Files` section.

    Every `## Files <qualifier>` heading opens a section too, so a summary that
    spells it `## Files Modified` is read rather than skipped. A section runs to
    the next `## ` heading or to end of file."""
    out: list[tuple[int, str]] = []
    inside = False
    for lineno, line in enumerate(text.splitlines(), start=1):
        if line.startswith("## "):
            heading = line[3:].strip()
            inside = heading == "Files" or heading.startswith("Files ")
            continue
        if inside:
            out.append((lineno, line))
    return out


def candidate_paths(root: Path, raw: str) -> list[str]:
    """Reduce one backtick span to concrete repo-relative paths to verify.

    Returns [] for a span that is not a path reference at all (prose, a symbol,
    a slash command, a template). A `..` token survives reduction on purpose so
    that `path_problem` can report it."""
    token = raw.strip().split()[0] if raw.strip() else ""
    token = token.rstrip(".,;:)('\"")
    token = token.split("#")[0]
    token = LINE_SUFFIX.sub("", token)
    token = SYMBOL_COLON.sub("", token)
    if not (root / token).exists():
        token = SYMBOL_DOT.sub("", token)
    if not token:
        return []
    if any(m in token for m in PLACEHOLDER_MARKERS):
        return []
    if token.startswith(SKIP_PREFIXES):
        return []
    if ".." in BRACE_GROUP.sub("", token):
        # Reported, not resolved. Reduction stops here so a traversal token can
        # never reach the filesystem.
        return [token]
    if ".." in token:
        # The only `..` is inside a brace group: `test/firewall/{001..011}.ci`
        # is a range template, not traversal. Skipped like any other template,
        # because calling it traversal would send a reader after the wrong bug.
        return []
    if "/" not in token and token not in ROOT_FILES:
        return []
    if token not in ROOT_FILES and token.split("/")[0] not in KNOWN_ROOTS:
        return []
    return [
        t for t in expand_braces(token) if not any(m in t for m in PLACEHOLDER_MARKERS)
    ]


def path_problem(root: Path, token: str) -> str | None:
    """What is wrong with `token` as a path under `root`, or None when it is fine.

    Never resolves outside the repository root and never follows a symlink out
    of the tree (spec Security Review)."""
    if ".." in token:
        return "path traversal (`..`) is never resolved; cite a repo-relative path"
    target = root / token.rstrip("/")
    try:
        real = os.path.realpath(target)
        real_root = os.path.realpath(root)
    except OSError as err:  # pragma: no cover - realpath is total on every OS we run
        return f"path could not be resolved: {err}"
    if real != real_root and not real.startswith(real_root + os.sep):
        return "path resolves outside the repository root (symlink escape)"
    if not os.path.exists(target):
        return "path does not exist"
    return None


def citation_resolves(root: Path, number: str) -> bool:
    """True when `plan/learned/<number>` names a surviving summary.

    Matched by number, with and without zero padding, because the corpus writes
    both `plan/learned/59` and `plan/learned/059`."""
    d = root.joinpath(*LEARNED_DIR)
    for n in {number, number.lstrip("0") or "0", number.zfill(3)}:
        if globmod.glob(os.path.join(str(d), f"{n}-*.md")):
            return True
    return False


def check(root: Path) -> list[dict]:
    """Dead references in plan/learned/, one finding per bad reference.

    Each finding carries `summary` (repo-relative path of the summary), `line`
    (1-based line the reference sits on), `token` (the reference as written) and
    `problem` (what is wrong, phrased so the reader can act without opening the
    checker).

    A summary that cannot be read, or that has no `## Files` section, is itself
    a finding: this function never returns an empty list because it gave up."""
    findings: list[dict] = []
    for path in summary_files(root):
        rel = path.relative_to(root).as_posix()
        try:
            text = path.read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError) as err:
            findings.append(
                {
                    "summary": rel,
                    "line": 1,
                    "token": path.name,
                    "problem": f"summary could not be read, so its references "
                    f"were never checked: {err}",
                }
            )
            continue

        section = files_section_lines(text)
        if not section:
            findings.append(
                {
                    "summary": rel,
                    "line": 1,
                    "token": "## Files",
                    "problem": "no `## Files` section, so this summary's paths "
                    "cannot be checked; add one, with `None recorded.` when "
                    "there is nothing to list",
                }
            )
        for lineno, line in section:
            for raw in BACKTICK.findall(line):
                for token in candidate_paths(root, raw):
                    problem = path_problem(root, token)
                    if problem:
                        findings.append(
                            {
                                "summary": rel,
                                "line": lineno,
                                "token": token,
                                "problem": problem,
                            }
                        )

        for lineno, line in enumerate(text.splitlines(), start=1):
            for number in CITATION.findall(line):
                if not citation_resolves(root, number):
                    findings.append(
                        {
                            "summary": rel,
                            "line": lineno,
                            "token": f"plan/learned/{number}",
                            "problem": "cites a learned summary that no longer "
                            "exists; recover it with `git log --diff-filter=D`, "
                            "or repoint the citation",
                        }
                    )
    return findings


def load_baseline(root: Path) -> int | None:
    """The recorded shrink-only ceiling, or None when none is recorded.

    Comments (#) and blank lines are ignored; the first bare integer wins. A
    baseline file that exists but holds no integer reads as unrecorded, so a
    corrupted ceiling can never be mistaken for a tight one."""
    try:
        text = (root / BASELINE_REL).read_text(encoding="utf-8")
    except OSError:
        return None
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        try:
            return int(line)
        except ValueError:
            return None
    return None


MIN_RAISE_REASON = 12


class BaselineRaiseRefused(Exception):
    """A write would raise the ceiling, and no reason was recorded for it."""


def write_baseline(root: Path, count: int, *, raise_reason: str | None = None) -> None:
    """Record `count` as the ceiling, refusing to RAISE it without a reason.

    Shrink-only is enforced here, at the producer. The docstring used to say the
    caller enforced it, and no caller did: one `--write-baseline` over a
    regression rewrote the ceiling upward and exited 0, which is the whole
    ratchet defeated by a single command (ai/rules/evidence.md).

    `raise_reason` is the operator overriding that refusal. It is written into
    the file, because a re-blessing nobody can read later is indistinguishable
    from rot that arrived on its own. It is written ONLY when the write actually
    raises the ceiling: passing the flag over a count that TIGHTENS the ceiling
    used to stamp "Raised deliberately" onto a shrink, and a later reader met a
    raise record sitting over a number that had gone down.
    """
    current = load_baseline(root)
    if current is not None and count > current and raise_reason is None:
        raise BaselineRaiseRefused(
            f"refusing to raise the staleness ceiling: {BASELINE_REL} records "
            f"{current} dead reference(s), this run counted {count}.\n"
            f"  Writing {count} re-blesses {count - current} new dead "
            "reference(s), and the ratchet only ever tightens.\n"
            "  next: repoint each reference at the path the code moved to, or "
            "delete the line when the code it named is gone\n"
            "        (plan/learned/METHODOLOGY.md). If the rise is deliberate, "
            "record why:\n"
            '        --raise-baseline "<why the ceiling must rise>"'
        )
    if raise_reason is not None and len(raise_reason.strip()) < MIN_RAISE_REASON:
        raise BaselineRaiseRefused(
            f'--raise-baseline reason is too short: "{raise_reason.strip()}" is '
            f"{len(raise_reason.strip())} characters, {MIN_RAISE_REASON} is the "
            "minimum.\n"
            "  The reason is the record of why the corpus is allowed to carry "
            "more rot,\n"
            "  so it has to say something a later reader can check."
        )
    header = (
        "# Learned-summary staleness baseline -- shrink-only ceiling.\n"
        "#\n"
        "# The number below is how many dead references"
        " (scripts/dev/learned_staleness.py)\n"
        "# plan/learned/ is currently tolerated to carry: dead `## Files` paths"
        " plus dead\n"
        "# `plan/learned/NNN` citations. More than this fails the gate; fewer is"
        " reported,\n"
        "# and `--write-baseline` tightens the ceiling to it. No other run"
        " writes this file:\n"
        "# a plain check inside `make ze-verify` must not modify a tracked file.\n"
        "#\n"
        "# The corpus could not start clean (1,856 dead references when the gate"
        " landed),\n"
        "# so this is the plan/.citation-baseline idiom: grandfather the known"
        " rot, refuse\n"
        "# to let it grow. Raising this number needs"
        ' `--raise-baseline "<reason>"`.\n'
    )
    raised = current is not None and count > current
    note = (
        f"# Raised deliberately: {raise_reason.strip()}\n"
        if raised and raise_reason
        else ""
    )
    target = root / BASELINE_REL
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(f"{header}{note}{count}\n", encoding="utf-8")


def _report(findings: list[dict], limit: int = 40) -> None:
    for f in findings[:limit]:
        print(
            f"  {f['summary']}:{f['line']}: `{f['token']}` -- {f['problem']}",
            file=sys.stderr,
        )
    if len(findings) > limit:
        print(
            f"  ... and {len(findings) - limit} more; run"
            " `python3 scripts/dev/learned_staleness.py --json` for all of them",
            file=sys.stderr,
        )


def enforce(root: Path, findings: list[dict], baseline: int | None) -> int:
    """Compare `findings` against the shrink-only ceiling and print the verdict.

    Returns the process exit code, and writes NOTHING. Over the ceiling fails;
    under it, the available tightening is REPORTED and `--write-baseline` applies
    it. A check that rewrites a tracked file is a check that cannot be run
    freely: this one runs inside ze-doc-test inside ze-verify, and several
    sessions share this checkout, so a silent rewrite lands in whichever
    session commits next. `spec-citation-check.py` is the idiom -- it writes only
    under its own explicit flag.

    An unrecorded ceiling cannot deny, so it says so instead of printing a green
    line it has not earned (`ai/rules/evidence.md`)."""
    count = len(findings)
    summaries = len(summary_files(root))

    if baseline is None:
        print(
            f"learned staleness: NO BASELINE RECORDED. {count} dead reference(s) "
            f"across {summaries} summaries; the gate is measuring but not "
            f"enforcing. Record the ceiling with "
            f"`python3 scripts/dev/learned_staleness.py --write-baseline`.",
            file=sys.stderr,
        )
        return 0

    if count > baseline:
        print(
            f"learned staleness check FAILED: {count} dead reference(s) in "
            f"{len({f['summary'] for f in findings})} summary(ies); the baseline "
            f"({BASELINE_REL}) allows {baseline}",
            file=sys.stderr,
        )
        _report(findings)
        print(
            "Repoint each reference at the path the code moved to, or delete the "
            "line when the code it named is gone (plan/learned/METHODOLOGY.md). "
            "Raising the ceiling re-blesses the regression, so --write-baseline "
            'refuses it; --raise-baseline "<why>" is the deliberate override.',
            file=sys.stderr,
        )
        return 1

    if count < baseline:
        print(
            f"learned staleness OK: {count} dead reference(s), down from "
            f"{baseline}. Tighten the ceiling with `python3 "
            f"scripts/dev/learned_staleness.py --write-baseline`"
        )
        return 0

    print(
        f"checked {summaries} learned summaries, "
        f"{count} dead reference(s) at the baseline ceiling"
    )
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--repo",
        default=str(Path(__file__).resolve().parents[2]),
        help="repository root (default: the checkout this script lives in)",
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="no-op alias, for symmetry with the other ze-doc-test gates",
    )
    parser.add_argument("--json", action="store_true", help="machine-readable report")
    parser.add_argument(
        "--write-baseline",
        action="store_true",
        help="record the current dead-reference count as the ceiling "
        "(refuses to raise it)",
    )
    parser.add_argument(
        "--raise-baseline",
        metavar="REASON",
        default=None,
        help="record the count even when it RAISES the ceiling, and write "
        "REASON into the baseline file as the record of why",
    )
    args = parser.parse_args(sys.argv[1:] if argv is None else argv)
    root = Path(args.repo).resolve()

    summaries = summary_files(root)
    findings = check(root)
    count = len(findings)
    baseline = load_baseline(root)

    if args.write_baseline or args.raise_baseline is not None:
        try:
            write_baseline(root, count, raise_reason=args.raise_baseline)
        except BaselineRaiseRefused as err:
            print(str(err), file=sys.stderr)
            return 1
        print(f"wrote {BASELINE_REL} with a ceiling of {count} dead reference(s)")
        return 0

    if args.json:
        print(
            json.dumps(
                {
                    "summaries": len(summaries),
                    "checked": len(summaries),
                    "implemented": IMPLEMENTED,
                    "baseline": baseline,
                    "dead": count,
                    "findings": findings,
                },
                indent=2,
            )
        )
        return 1 if baseline is not None and count > baseline else 0

    return enforce(root, findings, baseline)


if __name__ == "__main__":
    sys.exit(main())
