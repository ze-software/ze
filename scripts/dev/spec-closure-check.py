#!/usr/bin/env python3
"""Detect specs that were implemented but never formally closed.

Ze's two-commit spec closure (ai/rules/planning.md "Spec Closure") is:
  commit A  = code + final spec update + plan/learned/NNN-<slug>.md
  commit B  = git rm plan/spec-<stem>.md

"Completed but not closed" is the state where commit A ran but commit B never
did: the learned summary is committed, yet the spec file still sits in plan/
with `| Status | in-progress |`. Until now that condition was only detectable
by the /ze-status skill (model judgement) or advisory prose, so it was routinely
forgotten. This script makes it mechanical so hooks can enforce it.

The decisive signal is a *committed* learned summary whose slug carries the
spec's stem, or a committed journal row whose Spec cell names the stem AND a
finished Review Gate in the spec itself. Committed (not merely on disk)
matters: an on-disk-only file is not evidence of completion. The second signal
needs the Review Gate because a journal row is written when a problem is FOUND,
not at closure, so the row alone says the spec is being worked on.

Modes:
  (default) / --list   Print every completed-but-not-closed spec (triage view).
  --json               Machine-readable report of all specs + signals.
  --spec <path|name>   Check one spec. Exit 3 if it is completed-but-not-closed,
                       0 otherwise.

--spec was written for the block-premature-stop Stop hook, and that hook calls it
(.claude/hooks/block-premature-stop.sh:96). The hook was registered on NO event
from 41e5fa44f (2026-06-29) until 2026-07-31, so the closure gate did not run for
a month. Check the Stop array in .claude/settings.json before you describe this as
enforced.

The Stop-hook use is the one that must never false-positive, so --spec stays
strict (a committed closure artifact required) and honours an ack escape hatch, read
by cmd_spec below:
  tmp/session/.closure-ack-<stem>   (spec genuinely still open; do not block)

Usage:
  scripts/dev/spec-closure-check.py [--list|--json]
  scripts/dev/spec-closure-check.py --spec plan/spec-<name>.md
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

# One journal row parser, in journal.py. The copy that lived here returned None
# for a malformed row, so closure evidence skipped rows the journal gate names.
from journal import MALFORMED as JOURNAL_MALFORMED  # noqa: E402
from journal import journal_row_cells, journal_spec_stems  # noqa: E402

PLAN_DIR = Path("plan")
LEARNED_DIR = PLAN_DIR / "learned"

# `| Status | in-progress |` in the metadata table at the top of a spec.
STATUS_RE = re.compile(r"^\|\s*Status\s*\|\s*([^|]*?)\s*\|", re.MULTILINE)
# plan/learned/NNN-<slug>.md references embedded in a spec body.
LEARNED_REF_RE = re.compile(r"plan/learned/(\d{3,}-[a-z0-9][a-z0-9-]*)\.md")
# Learned-summary filenames on disk: NNN-<slug>.md
LEARNED_FILE_RE = re.compile(r"^(\d{3,})-([a-z0-9][a-z0-9-]*)\.md$")
# Unfinished closure checkbox left behind in a Review Gate.
UNCHECKED_CLOSE_RE = re.compile(
    r"^\s*-\s*\[ \].*(before closing|still to run|re-run shows 0 BLOCKER)",
    re.IGNORECASE | re.MULTILINE,
)


def _is_sublist(needle: list[str], haystack: list[str]) -> bool:
    """True if needle appears as a contiguous run inside haystack."""
    if not needle or len(needle) > len(haystack):
        return False
    for start in range(len(haystack) - len(needle) + 1):
        if haystack[start : start + len(needle)] == needle:
            return True
    return False


def _committed_learned(repo: Path) -> set[str]:
    """Relpaths of learned summaries tracked in git (one call, memoizable).

    Tracked ~ committed here: a new summary may exist on disk before commit A
    stages+commits it, so a tracked learned file is proof commit A ran.
    git unavailable -> empty set (fail-open: the Stop hook must never wedge
    the session on a tooling error).
    """
    try:
        out = subprocess.run(
            ["git", "ls-files", "--", LEARNED_DIR.as_posix()],
            cwd=repo,
            capture_output=True,
            text=True,
            timeout=10,
        )
    except (OSError, subprocess.SubprocessError):
        return set()
    if out.returncode != 0:
        return set()
    return {line.strip() for line in out.stdout.splitlines() if line.strip()}


def _learned_files(repo: Path) -> list[tuple[str, str, list[str]]]:
    """Return (relpath, slug, slug-token-list) for every learned summary on disk."""
    learned_dir = repo / LEARNED_DIR
    result: list[tuple[str, str, list[str]]] = []
    if not learned_dir.is_dir():
        return result
    for entry in sorted(learned_dir.iterdir()):
        m = LEARNED_FILE_RE.match(entry.name)
        if not m:
            continue
        slug = m.group(2)
        rel = f"{LEARNED_DIR.as_posix()}/{entry.name}"
        result.append((rel, slug, slug.split("-")))
    return result


JOURNAL_DIR = PLAN_DIR / "journal"


def _journal_evidence(repo: Path) -> dict[str, str]:
    """Map {spec-stem: journal-relpath} from committed journal files.

    Reads committed files via `git ls-files` and `git show`, so a journal row
    on disk but not yet committed does not count (same rule as learned summaries).

    README.md is skipped: its fenced example is a row, and reading it as
    evidence names whatever spec the example happens to use.

    A malformed row is NAMED on stderr rather than skipped in silence, because
    a row this reader cannot parse is a row it cannot honour. It does not fail
    the run: the Stop hook consumes this and must not wedge a session.
    """
    result: dict[str, str] = {}
    try:
        out = subprocess.run(
            ["git", "ls-files", "--", JOURNAL_DIR.as_posix()],
            cwd=repo,
            capture_output=True,
            text=True,
            timeout=10,
        )
    except (OSError, subprocess.SubprocessError):
        return result
    if out.returncode != 0:
        return result
    for relpath in out.stdout.splitlines():
        relpath = relpath.strip()
        if not relpath or not relpath.endswith(".md"):
            continue
        if relpath.endswith("/README.md"):
            continue
        try:
            content = subprocess.run(
                ["git", "show", f"HEAD:{relpath}"],
                cwd=repo,
                capture_output=True,
                text=True,
                timeout=10,
            )
        except (OSError, subprocess.SubprocessError):
            continue
        if content.returncode != 0:
            continue
        named_malformed = False
        for line in content.stdout.splitlines():
            cells = journal_row_cells(line)
            if cells is None:
                continue
            if cells == [JOURNAL_MALFORMED]:
                if not named_malformed:
                    print(
                        f"warning: malformed journal row in {relpath} "
                        "(run `make ze-journal-report`)",
                        file=sys.stderr,
                    )
                    named_malformed = True
                continue
            for spec in journal_spec_stems(cells[1]) or ():
                result.setdefault(spec, relpath)
    return result


def _status(content: str) -> str:
    # Only the metadata table (first lines) is authoritative; take the first hit.
    head = "\n".join(content.splitlines()[:12])
    m = STATUS_RE.search(head)
    return m.group(1).strip().lower() if m else "unknown"


def _stem(path: Path) -> str:
    return path.stem[len("spec-") :] if path.stem.startswith("spec-") else path.stem


def _all_stems(repo: Path) -> set[str]:
    return {
        _stem(p)
        for p in (repo / PLAN_DIR).glob("spec-*.md")
        if p.name != "spec-template.md"
    }


class SpecReport:
    def __init__(
        self,
        repo: Path,
        path: Path,
        learned: list[tuple[str, str, list[str]]],
        all_stems: set[str],
        committed: set[str],
        journal_evidence: dict[str, str] | None = None,
    ):
        self.path = path
        try:
            self.rel = path.relative_to(repo).as_posix()
        except ValueError:
            self.rel = path.as_posix()
        self.stem = (
            path.stem[len("spec-") :] if path.stem.startswith("spec-") else path.stem
        )
        content = path.read_text(encoding="utf-8", errors="replace")
        self.status = _status(content)
        stem_tokens = self.stem.split("-")

        # Umbrella specs close via their children, never directly. Two markers:
        # the name says "umbrella", or child specs `spec-<stem>-<N>-*` exist.
        # Verified empirically: umbrella flags were false positives every time
        # (the flagging learned summary belonged to a closed child).
        self.is_umbrella = "umbrella" in stem_tokens or any(
            re.match(rf"^{re.escape(self.stem)}-\d", other)
            for other in all_stems
            if other != self.stem
        )

        # Committed learned summaries tied to this spec, tiered by link strength:
        #   exact  slug == stem      -> the spec's OWN summary (high confidence)
        #   stem   stem is a token-run inside the slug (weak: may be a child's,
        #          e.g. fib-depth <- fib-depth-2-ecmp)
        #   ref    spec body cites a learned path (weak: often a predecessor's)
        # Only committed summaries count: on-disk alone proves nothing.
        self.learned_exact: str | None = None
        self.learned_stem: str | None = None
        for rel, slug, slug_tokens in learned:
            if rel not in committed:
                continue
            if slug == self.stem and self.learned_exact is None:
                self.learned_exact = rel
            if _is_sublist(stem_tokens, slug_tokens) and self.learned_stem is None:
                self.learned_stem = rel

        self.learned_ref: str | None = None
        for m in LEARNED_REF_RE.finditer(content):
            ref = f"plan/learned/{m.group(1)}.md"
            if ref in committed:
                self.learned_ref = ref
                break

        # A committed journal row whose Spec cell names this stem is a closure
        # signal equivalent to a learned summary (plan/spec-problem-journal.md).
        je = journal_evidence or {}
        self.journal_match: str | None = je.get(self.stem)

        self.gate_present = "## Review Gate" in content
        self.unchecked_close = bool(UNCHECKED_CLOSE_RE.search(content))

    @property
    def gate_finished(self) -> bool:
        """The spec's own Review Gate says the work is finished.

        The section is appended at closure (plan/TEMPLATE-CLOSURE.md), and an
        unticked "before closing" box inside it says it is not finished yet.
        """
        return self.gate_present and not self.unchecked_close

    @property
    def completed_not_closed(self) -> bool:
        # Decisive, low-false-positive: this is the only signal the Stop-hook
        # block acts on, so it must almost never misfire.
        #
        # A learned summary whose slug IS the stem was written AT closure, so it
        # is evidence on its own. A journal row is not: a row is written when a
        # problem is FOUND, mid-work, and the Spec cell names the spec that found
        # it. Acting on the row alone blocked every stop for the rest of the
        # session from the moment the first row was written. The row therefore
        # counts only alongside the spec's own finished Review Gate.
        if self.status != "in-progress" or self.is_umbrella:
            return False
        if self.learned_exact is not None:
            return True
        return self.journal_match is not None and self.gate_finished

    @property
    def needs_verification(self) -> bool:
        # In-progress with a weaker completion signal (child/sibling/predecessor
        # learned summary, or an umbrella): possibly closable, but a human or
        # audit agent must confirm. Empirically most of these are NOT closable.
        if self.completed_not_closed or self.status != "in-progress":
            return False
        return bool(self.learned_stem or self.learned_ref)

    @property
    def evidence(self) -> str | None:
        return (
            self.learned_exact
            or self.journal_match
            or self.learned_stem
            or self.learned_ref
        )

    def to_dict(self) -> dict:
        return {
            "spec": self.rel,
            "stem": self.stem,
            "status": self.status,
            "completed-not-closed": self.completed_not_closed,
            "needs-verification": self.needs_verification,
            "is-umbrella": self.is_umbrella,
            "evidence-learned": self.evidence,
            "learned-exact": self.learned_exact,
            "journal-match": self.journal_match,
            "learned-stem": self.learned_stem,
            "learned-ref": self.learned_ref,
            "review-gate-present": self.gate_present,
            "unchecked-close-box": self.unchecked_close,
        }


def _load_all(repo: Path) -> list[SpecReport]:
    learned = _learned_files(repo)
    all_stems = _all_stems(repo)
    committed = _committed_learned(repo)
    je = _journal_evidence(repo)
    reports: list[SpecReport] = []
    for path in sorted((repo / PLAN_DIR).glob("spec-*.md")):
        if path.name == "spec-template.md":
            continue
        reports.append(
            SpecReport(repo, path, learned, all_stems, committed, journal_evidence=je)
        )
    return reports


def _resolve_spec_path(repo: Path, spec: str) -> Path:
    p = Path(spec)
    if p.is_absolute():
        return p
    if p.exists():
        return p.resolve()
    name = spec if spec.endswith(".md") else f"{spec}.md"
    if not name.startswith("spec-"):
        name = f"spec-{name}"
    return repo / PLAN_DIR / name


def cmd_spec(repo: Path, spec: str) -> int:
    path = _resolve_spec_path(repo, spec)
    if not path.is_file():
        # Nothing to close if the spec is gone (e.g. already git-rm'd).
        return 0
    learned = _learned_files(repo)
    report = SpecReport(
        repo,
        path,
        learned,
        _all_stems(repo),
        _committed_learned(repo),
        journal_evidence=_journal_evidence(repo),
    )
    if not report.completed_not_closed:
        return 0
    ack = repo / "tmp" / "session" / f".closure-ack-{report.stem}"
    if ack.exists():
        print(f"closure-ack present for {report.stem}; not blocking", file=sys.stderr)
        return 0
    print(
        f"Spec '{report.rel}' is COMPLETED BUT NOT CLOSED.\n"
        f"  Evidence: committed closure artifact {report.evidence} exists,\n"
        f"  but the spec is still in plan/ with Status=in-progress.\n"
        f"  Close it (ai/rules/planning.md Spec Closure):\n"
        f"    1. Finalize the Review Gate (0 BLOCKER, 0 ISSUE) via /ze-review.\n"
        f"    2. Prepare closure commit: git rm {report.rel}\n"
        f"  If it is genuinely still open, record why and proceed:\n"
        f"    echo '<reason>' > tmp/session/.closure-ack-{report.stem}",
        file=sys.stderr,
    )
    return 3


def cmd_report(repo: Path, as_json: bool) -> int:
    reports = _load_all(repo)
    if as_json:
        json.dump([r.to_dict() for r in reports], sys.stdout, indent=2)
        sys.stdout.write("\n")
        return 0
    flagged = [r for r in reports if r.completed_not_closed]
    possible = [r for r in reports if r.needs_verification]
    if not flagged and not possible:
        print("No completed-but-not-closed specs.")
        return 0
    if flagged:
        print(f"Completed but not closed ({len(flagged)}) -- high confidence:\n")
        for r in flagged:
            print(f"  {r.rel}")
            print(f"      evidence: own closure artifact {r.evidence}")
        print(
            "\nClose each via ai/rules/planning.md Spec Closure (finalize gate, git rm spec)."
        )
    if possible:
        print(
            f"\nPossibly closable -- NEEDS VERIFICATION ({len(possible)}):\n"
            "  Weak signal (umbrella, or a child/sibling/predecessor learned\n"
            "  summary). Audit before closing -- most of these are false positives.\n"
        )
        for r in possible:
            why = "umbrella" if r.is_umbrella else "weak-match"
            print(f"  {r.rel}  [{why}]")
            print(f"      signal: {r.evidence}")
    return 0


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", default=".", help="repository root (default: cwd)")
    group = parser.add_mutually_exclusive_group()
    group.add_argument(
        "--list",
        action="store_true",
        help="list completed-but-not-closed specs (default)",
    )
    group.add_argument("--json", action="store_true", help="emit full JSON report")
    group.add_argument(
        "--spec", help="check one spec; exit 3 if completed-but-not-closed"
    )
    args = parser.parse_args(argv)

    repo = Path(args.repo).resolve()
    if args.spec:
        return cmd_spec(repo, args.spec)
    return cmd_report(repo, args.json)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
