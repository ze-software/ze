#!/usr/bin/env python3
"""Spec citation freshness gate (2026-07-16 audit, failure mode #1).

Specs reference sibling ``plan/spec-*.md`` files and ``path:line`` source
locations that drift or vanish as specs close and code moves. The two-commit
closure model git-rm's a spec while other specs may still cite it, so the
citation dangles silently -- there was no checker at all.

Two passes:

  1. DANGLING-SPEC FAIL (AC-1). A ``plan/spec-*.md`` that references another
     ``plan/spec-*.md`` absent on disk fails the gate (exit 1), naming the citing
     spec, the citing line, and the dangling ref -- UNLESS the absent target is
     in the baseline allow-list. Scope is spec -> spec on purpose: a learned
     summary referencing its own now-closed spec is the EXPECTED result of the
     two-commit closure model (spec removed, learned kept), so ``plan/learned/``
     is excluded from the FAIL pass.

  2. TOKEN-DRIFT WARN (AC-2). A ``path:line`` citation (in a spec OR a learned
     summary) whose backtick-quoted neighbour token no longer appears on that
     source line prints a WARN -- non-fatal. WARN-forever per the spec's
     autonomous default: the line-token heuristic can false-positive on
     inconsistent quote conventions, so the fatal signal stays reserved for the
     unambiguous dangling-file case.

Baseline (``plan/.citation-baseline``): an allow-list of the currently-known
dangling spec targets, exactly like ``test/.ci-sleep-baseline`` grandfathers
legacy sleeps. The current (rotted) tree passes; a target that becomes newly
absent -- e.g. a freshly-closed spec still cited by a sibling -- is NOT in the
baseline and fails, naming who cites it. Regenerate with ``--write-baseline``.

Usage:
  scripts/dev/spec-citation-check.py [--repo PATH]
  scripts/dev/spec-citation-check.py --write-baseline   # refresh the allow-list
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

PLAN_DIR = "plan"
BASELINE_REL = "plan/.citation-baseline"

# A reference to a spec file, e.g. ``plan/spec-foo-bar.md`` (optionally :line). <!-- doc-links: ignore (example spec name in a comment, not a real spec) -->
SPEC_REF_RE = re.compile(r"plan/spec-[a-z0-9][a-z0-9-]*\.md")
# A backtick-quoted ``path:line`` citation, e.g. `internal/x/foo.go:104`.
CITATION_RE = re.compile(r"`([^`\s]+?):(\d+)`")
# Any backtick-quoted token on a line.
BACKTICK_RE = re.compile(r"`([^`]+)`")


def load_baseline(repo: Path) -> set[str]:
    """Allow-list of known-dangling spec targets (posix paths). Comments (#) and
    blank lines are ignored; a missing file yields an empty allow-list."""
    path = repo / BASELINE_REL
    out: set[str] = set()
    try:
        text = path.read_text(encoding="utf-8")
    except OSError:
        return out
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        out.add(line)
    return out


def spec_files(repo: Path) -> list[Path]:
    plan = repo / PLAN_DIR
    if not plan.is_dir():
        return []
    return sorted(p for p in plan.glob("spec-*.md") if p.name != "spec-template.md")


def learned_files(repo: Path) -> list[Path]:
    learned = repo / PLAN_DIR / "learned"
    if not learned.is_dir():
        return []
    return sorted(learned.glob("*.md"))


def find_dangling(repo: Path, baseline: set[str]) -> list[tuple[str, int, str]]:
    """(citing_rel, lineno, dangling_ref) for every spec -> absent-spec reference
    whose target is not baselined. Scoped to plan/spec-*.md citers."""
    dangling: list[tuple[str, int, str]] = []
    for spec in spec_files(repo):
        rel = spec.relative_to(repo).as_posix()
        try:
            lines = spec.read_text(encoding="utf-8").splitlines()
        except OSError:
            continue
        for i, line in enumerate(lines, start=1):
            for ref in SPEC_REF_RE.findall(line):
                if ref == rel:
                    continue  # a spec citing itself is not dangling
                if (repo / ref).is_file():
                    continue
                if ref in baseline:
                    continue
                dangling.append((rel, i, ref))
    return dangling


def find_token_drift(repo: Path) -> list[tuple[str, int, str, str]]:
    """(citing_rel, lineno, 'path:line', missing_token) for every path:line
    citation whose backtick-quoted neighbour token is no longer on that source
    line. Runs over specs AND learned summaries. Non-fatal (WARN)."""
    warns: list[tuple[str, int, str, str]] = []
    for doc in spec_files(repo) + learned_files(repo):
        rel = doc.relative_to(repo).as_posix()
        try:
            lines = doc.read_text(encoding="utf-8").splitlines()
        except OSError:
            continue
        for i, line in enumerate(lines, start=1):
            # Ordered backtick tokens with positions, split into path:line
            # citations and plain neighbour tokens. Pairing each citation with
            # only its NEAREST plain neighbour (the token "quoted next to" it, per
            # the spec convention) keeps the WARN low-noise: a line with several
            # unrelated backtick tokens does not cross-check every one against
            # every citation.
            spans = list(BACKTICK_RE.finditer(line))
            cites = [m for m in spans if CITATION_RE.fullmatch(m.group(0))]
            if not cites:
                continue
            plain = [
                m
                for m in spans
                if not re.search(r":\d+$", m.group(1))
                and any(c.isalpha() for c in m.group(1))
            ]
            if not plain:
                continue
            for cm in cites:
                token = _nearest_token(cm, plain)
                if token is None:
                    continue
                mref = CITATION_RE.search(cm.group(0))
                path, lineno = mref.group(1), int(mref.group(2))
                src = repo / path
                if not src.is_file():
                    continue  # missing source file is not this pass's concern
                try:
                    src_lines = src.read_text(encoding="utf-8").splitlines()
                except OSError:
                    continue
                src_line = (
                    src_lines[lineno - 1] if 1 <= lineno <= len(src_lines) else ""
                )
                if token not in src_line:
                    warns.append((rel, i, f"{path}:{lineno}", token))
    return warns


def _nearest_token(cite, plain: list) -> str | None:
    """The plain backtick token nearest to citation match ``cite``: prefer the
    one immediately preceding it on the line, else the one immediately following.
    ``plain`` and ``cite`` are re.Match objects from the same line."""
    before = [m for m in plain if m.end() <= cite.start()]
    after = [m for m in plain if m.start() >= cite.end()]
    if before:
        return before[-1].group(1)
    if after:
        return after[0].group(1)
    return None


def compute_baseline(repo: Path) -> list[str]:
    """The set of spec -> absent-spec dangling targets in the current tree,
    sorted. Used to (re)generate plan/.citation-baseline."""
    targets = {ref for _, _, ref in find_dangling(repo, set())}
    return sorted(targets)


def write_baseline(repo: Path) -> int:
    targets = compute_baseline(repo)
    header = (
        "# Spec citation baseline -- known-dangling plan/spec-*.md targets.\n"
        "#\n"
        "# Allow-list, like test/.ci-sleep-baseline: these spec files are cited\n"
        "# by a sibling spec but no longer exist on disk (closed/folded). The\n"
        "# gate (scripts/dev/spec-citation-check.py) grandfathers them so the\n"
        "# current tree passes; a target that becomes NEWLY absent (e.g. a spec\n"
        "# closed while still cited) is not listed here and fails the gate.\n"
        "#\n"
        "# Shrink this list by cleaning up the citing reference, then removing\n"
        "# the entry. Regenerate with: scripts/dev/spec-citation-check.py"
        " --write-baseline\n"
    )
    body = "".join(f"{t}\n" for t in targets)
    (repo / BASELINE_REL).write_text(header + body, encoding="utf-8")
    print(f"wrote {BASELINE_REL} with {len(targets)} known-dangling spec targets")
    return 0


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", default=".", help="repository root (default: cwd)")
    parser.add_argument(
        "--write-baseline",
        action="store_true",
        help="regenerate plan/.citation-baseline from the current tree",
    )
    args = parser.parse_args(argv)
    repo = Path(args.repo).resolve()

    if args.write_baseline:
        return write_baseline(repo)

    baseline = load_baseline(repo)
    dangling = find_dangling(repo, baseline)
    warns = find_token_drift(repo)

    for citer, lineno, ref, token in warns:
        print(
            f"WARN {citer}:{lineno}: citation `{ref}` no longer shows token"
            f" `{token}` on that line (line-token drift)"
        )

    if dangling:
        print("spec-citation-check FAILED: dangling plan/spec-*.md references")
        for citer, lineno, ref in dangling:
            print(
                f"  {citer}:{lineno}: references {ref} which is absent on disk"
                " (not in baseline)"
            )
        print(
            f"\n{len(dangling)} dangling reference(s). Either fix the citing"
            " reference, or -- if the target is legitimately gone -- add it to"
            f" {BASELINE_REL} (or run spec-citation-check.py --write-baseline)."
        )
        return 1

    total = len(spec_files(repo))
    msg = f"spec-citation-check OK ({total} specs, {len(baseline)} baselined dangling"
    if warns:
        msg += f", {len(warns)} line-token WARN"
    msg += ")"
    print(msg)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
