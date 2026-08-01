#!/usr/bin/env python3
"""Report blocking rules whose trigger matched this session's files, unread.

`plan/spec-knowledge-3-rule-digest.md` wants to stop loading all 97 condensed
rule sections into every session and route rules by their `**When:**` trigger
instead. That plan rests on one assumption (A-4) which no amount of code reading
can settle: that a model which SEES a matching trigger will actually open the
rule. This detector measures it instead of betting on it.

It runs in the CURRENT world, where `ai/rules/CONDENSED.md` is still imported
eagerly and every rule body is already in context. A miss reported here is
therefore a session that never consulted a rule its own file types matched, even
with the whole digest in front of it. That is exactly the population that would
break once the body stops being eager.

Only `blocking` rules are reported. An advisory miss is not worth an operator's
attention and would bury the ones that are.

## The blind spot, stated rather than hidden

A trigger that names an ACTION rather than a file type cannot be matched from
touched files at all. `ai/rules/never-destroy-work.md` ("before deleting,
reverting, or overwriting any file holding uncommitted or user-visible work") is
the standing example: no file extension implies it. 22 of the 69 blocking rules
are unmatchable this way on the current corpus.

The detector therefore UNDER-reports, which is the safe direction: it never
claims coverage it has not observed. Silence from this tool is NOT evidence that
a session read what it needed. Every report line carries `unmatchable` so a
reader can never mistake one for the other (`ai/rules/fail-closed-guards.md`,
"or say something").

## What counts as reading a rule

Only a direct read of `ai/rules/<name>.md`. Reading `ai/rules/CONDENSED.md` does
NOT count, and that is deliberate: the digest is loaded in every session today,
so counting it would mark every rule read and the detector would measure
nothing. The question being asked is whether the session opened the rule.

Main-thread and subagent (`isSidechain`) turns are both counted. A subagent's
edits are the session's edits, and a rule a subagent read HAS been consulted;
excluding either half would manufacture misses that did not happen.

Usage:
    python3 scripts/dev/rule_coverage.py                    # this session
    python3 scripts/dev/rule_coverage.py --transcript PATH  # a named transcript
    python3 scripts/dev/rule_coverage.py --json             # machine-readable
    python3 scripts/dev/rule_coverage.py --no-append        # do not record

Exit: 1 when a blocking rule was matched but unread, 0 otherwise. NEVER 2. This
is a report, and a Stop hook that refuses a stop over a report is a gate nobody
keeps (`ai/rules/rule-precedence.md`, rung 5).
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
import time
from pathlib import Path

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import rules_lint
import running_model

# One line per session, appended, so a week of evidence accumulates instead of
# scrolling past in a terminal (AC-12). Deliberately NOT under a per-session
# scratch dir: the whole point is that every session writes to the same file.
# Lines are kept short so concurrent O_APPEND writes stay atomic.
REPORT_PATH = Path("tmp") / "rule-coverage" / "report.ndjson"

TOOLS_WRITE = ("Write", "Edit", "MultiEdit", "NotebookEdit")
TOOLS_READ = ("Read",)

RULES_DIR = "ai/rules/"

# The file-kind vocabulary. Each entry is (kind, path predicate, trigger words
# that NAME work on that kind of file). Matching is keyword -> file kind, never
# rule name -> file kind: adding a rule needs no edit here, which is what
# `ai/rules/derive-not-hardcode.md` asks for.
#
# The word lists are a FLOOR on recall, not a ceiling. They were tuned against
# the live corpus until a Go-editing session matched 19 of 69 blocking rules:
# few enough to act on, many enough to carry signal. Widening a list makes the
# detector report more, which is always the recoverable direction.
KIND_RULES = (
    (
        "go",
        lambda p: p.endswith(".go"),
        (
            "go",
            ".go",
            "goroutine",
            "wire-encoding",
            "wire encoding",
            "encoding path",
            "buffer",
            "pool",
            "allocation",
            "string-building",
            "fmt.sprintf",
            "hot path",
            "exported",
            "protocol-implementing",
            "protocol behavior",
            "wire format",
            "import",
            "function",
            "functions",
            "error",
            "errors",
        ),
    ),
    (
        "go-test",
        lambda p: p.endswith("_test.go"),
        ("test", "tests", "tdd", "test-first"),
    ),
    (
        "ci-test",
        lambda p: p.endswith((".ci", ".et")),
        (
            "functional test",
            "functional tests",
            ".ci",
            "user-facing behavior",
            "user-visible behavior",
        ),
    ),
    (
        "yang",
        lambda p: p.endswith(".yang"),
        (
            "yang",
            "config leaf",
            "config option",
            "config surface",
            "config content",
            "schema",
        ),
    ),
    (
        "docs",
        lambda p: p.startswith("docs/") and p.endswith(".md"),
        (
            "documentation",
            "docs",
            "doc",
            "prose",
            "user-visible behavior",
            "comment",
            "comments",
        ),
    ),
    (
        "spec",
        lambda p: p.startswith(("plan/spec-", "plan/design-")),
        ("spec", "specs", "acceptance criterion", "acceptance criteria"),
    ),
    (
        "learned",
        lambda p: p.startswith("plan/learned/"),
        ("learned", "learned summary"),
    ),
    (
        "rule",
        lambda p: p.startswith(RULES_DIR) and p.endswith(".md"),
        ("rule", "rules", "ai/rules/*.md"),
    ),
    ("python", lambda p: p.endswith(".py"), ("script", "scripts", "python")),
    (
        "shell",
        lambda p: p.endswith(".sh") or p.startswith(".claude/hooks/"),
        ("shell", "bash", "hook", "hooks"),
    ),
    (
        "make",
        lambda p: p.endswith(".mk") or os.path.basename(p) == "Makefile",
        ("make target", "makefile", "build", "gate"),
    ),
)


def _keyword_re(words: tuple[str, ...]) -> re.Pattern[str]:
    """Word-boundary alternation, so 'doc' never matches 'doctor'."""
    return re.compile(
        "|".join(r"(?<![A-Za-z0-9])" + re.escape(w) + r"(?![A-Za-z0-9])" for w in words)
    )


KIND_MATCHERS = tuple(
    (kind, pred, _keyword_re(words)) for kind, pred, words in KIND_RULES
)


def repo_root() -> Path:
    return Path(
        os.environ.get("CLAUDE_PROJECT_DIR") or Path(__file__).resolve().parents[2]
    )


def load_rules(rules_dir: Path) -> list[dict]:
    """Every rule with its trigger and severity, using the linted metadata block.

    The parse is `scripts/dev/rules_lint.py`'s: same `META_LINE`, same skip set.
    A second parser here would drift from the one the gate enforces.

    Generated artifacts are skipped by SHAPE (an all-caps stem, the repo's
    convention for `CONDENSED.md` / `INDEX.md` / the forthcoming `TRIGGERS.md`)
    rather than by a list that would need editing when the next one lands.
    """
    rules = []
    for md in sorted(rules_dir.glob("*.md")):
        if md.name in rules_lint.SKIP or md.stem.isupper():
            continue
        meta = {}
        for line in md.read_text(encoding="utf-8", errors="replace").splitlines()[:12]:
            m = rules_lint.META_LINE.match(line.strip())
            if not m:
                if meta:
                    break
                continue
            meta[m.group("key")] = m.group("val").strip()
        rules.append(
            {
                "name": md.name,
                "path": f"{RULES_DIR}{md.name}",
                "trigger": meta.get("When", ""),
                "severity": meta.get("Severity", ""),
            }
        )
    return rules


def rel_path(path: str, root: Path) -> str:
    """Repo-relative POSIX path, or the absolute path when it sits outside."""
    try:
        return Path(path).resolve().relative_to(root.resolve()).as_posix()
    except (ValueError, OSError):
        return path


def kinds_for(paths: set[str]) -> set[str]:
    """Every file kind the touched paths carry.

    Multi-label on purpose: a `_test.go` is both `go-test` and `go`, because a
    Go test file is Go code and the allocation and naming rules apply to it.
    """
    kinds = set()
    for p in paths:
        for kind, pred, _ in KIND_MATCHERS:
            if pred(p):
                kinds.add(kind)
    return kinds


def read_transcript(path: str) -> tuple[set[str], set[str]]:
    """(files written, rule files read) from one transcript.

    A transcript that cannot be read yields two empty sets AND says so on
    stderr. `touched=0` alone does not say it: a session that wrote nothing
    produces exactly the same number, so silence here is indistinguishable from
    a genuinely read-only session (ai/rules/fail-closed-guards.md -- a guard that
    cannot evaluate must speak). The resolved-but-missing case in `main` already
    speaks; this is the unreadable case, and it stays advisory, never fatal.
    """
    written: set[str] = set()
    rules_read: set[str] = set()
    try:
        with open(path, encoding="utf-8", errors="replace") as fh:
            lines = fh.readlines()
    except OSError as err:
        print(
            f"rule-coverage: cannot read the session transcript {path}: {err}; "
            "reporting nothing rather than guessing which rules were consulted",
            file=sys.stderr,
        )
        return written, rules_read

    root = repo_root()
    for line in lines:
        if '"tool_use"' not in line:
            continue
        try:
            entry = json.loads(line)
        except (ValueError, TypeError):
            continue
        content = (entry.get("message") or {}).get("content")
        if not isinstance(content, list):
            continue
        for block in content:
            if not isinstance(block, dict) or block.get("type") != "tool_use":
                continue
            name = block.get("name")
            fp = (block.get("input") or {}).get("file_path")
            if not isinstance(fp, str) or not fp:
                continue
            rel = rel_path(fp, root)
            if name in TOOLS_WRITE:
                written.add(rel)
            elif name in TOOLS_READ and rel.startswith(RULES_DIR):
                rules_read.add(os.path.basename(rel))
    return written, rules_read


def analyse(rules: list[dict], written: set[str], rules_read: set[str]) -> dict:
    """Match touched file kinds against triggers; report unread blocking rules."""
    blocking = [r for r in rules if r["severity"] == "blocking"]
    kinds = kinds_for(written)

    matched, unmatchable = [], []
    for rule in blocking:
        trigger = rule["trigger"].lower()
        hit_any = False
        for kind, _, rx in KIND_MATCHERS:
            if not rx.search(trigger):
                continue
            hit_any = True
            if kind in kinds:
                matched.append(rule)
                break
        if not hit_any:
            unmatchable.append(rule["name"])

    missed = sorted(r["name"] for r in matched if r["name"] not in rules_read)
    return {
        "blocking-total": len(blocking),
        "touched": len(written),
        "kinds": sorted(kinds),
        "rules-read": sorted(rules_read),
        "matched": sorted(r["name"] for r in matched),
        "missed": missed,
        "unmatchable": len(unmatchable),
        "unmatchable-rules": sorted(unmatchable),
    }


def append_report(result: dict, session: str, root: Path) -> Path:
    """Append one line. A failure to record must never fail the caller."""
    path = root / REPORT_PATH
    line = json.dumps(
        {
            "ts": time.strftime("%Y-%m-%dT%H:%M:%S"),
            "session": session,
            "touched": result["touched"],
            "kinds": result["kinds"],
            "matched": len(result["matched"]),
            "read": len(result["rules-read"]),
            "missed": result["missed"],
            "unmatchable": result["unmatchable"],
        },
        separators=(",", ":"),
    )
    try:
        path.parent.mkdir(parents=True, exist_ok=True)
        with open(path, "a", encoding="utf-8") as fh:
            fh.write(line + "\n")
    except OSError as err:
        print(
            f"rule-coverage: cannot record report at {path}: {err}; "
            "the analysis below still stands, only the accumulated evidence is lost",
            file=sys.stderr,
        )
    return path


def format_text(result: dict, report_path: Path) -> str:
    """Human-readable summary. Always states the unmatchable count."""
    out = []
    if result["missed"]:
        out.append(
            f"rule-coverage: {len(result['missed'])} blocking rule(s) matched this "
            f"session's files but were never read:"
        )
        for name in result["missed"]:
            out.append(f"  - {RULES_DIR}{name}")
    else:
        out.append(
            f"rule-coverage: 0 missed of {len(result['matched'])} blocking rule(s) "
            f"matched by {result['touched']} touched file(s)"
        )
    out.append(
        f"rule-coverage: {result['unmatchable']} of {result['blocking-total']} "
        "blocking rules have action-shaped triggers that no file type can match, "
        "so this count UNDER-reports; silence is not proof of coverage"
    )
    out.append(f"rule-coverage: report {report_path}")
    return "\n".join(out)


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--transcript", default=None, help="transcript path to analyse")
    ap.add_argument("--session", default="", help="session id recorded in the report")
    ap.add_argument("--rules-dir", default=None, help="rule directory to parse")
    ap.add_argument("--json", action="store_true", help="emit JSON instead of text")
    ap.add_argument("--no-append", action="store_true", help="do not record a line")
    args = ap.parse_args(argv)

    root = repo_root()
    rules_dir = Path(args.rules_dir) if args.rules_dir else root / "ai" / "rules"
    if not rules_dir.is_dir():
        print(
            f"rule-coverage: rule directory {rules_dir} does not exist; "
            "nothing to match against",
            file=sys.stderr,
        )
        return 0

    transcript = args.transcript or running_model.transcript_path()
    if not transcript or not os.path.isfile(transcript):
        print(
            "rule-coverage: no readable session transcript "
            f"({transcript or 'none resolved'}); reporting nothing rather than "
            "guessing which rules were consulted",
            file=sys.stderr,
        )
        return 0

    rules = load_rules(rules_dir)
    written, rules_read = read_transcript(transcript)
    result = analyse(rules, written, rules_read)

    report_path = root / REPORT_PATH
    if not args.no_append:
        report_path = append_report(result, args.session or "unknown", root)

    if args.json:
        print(json.dumps(result, indent=2))
    else:
        print(format_text(result, report_path))

    # 1 when something was missed, 0 otherwise. Never 2: see the module docstring.
    return 1 if result["missed"] else 0


if __name__ == "__main__":
    sys.exit(main())
