#!/usr/bin/env python3
"""Repoint dead `## Files` paths in plan/learned/ at the file the code moved to.

Most dead paths in the learned corpus are not deleted code. They are code that
MOVED: the `spec-tiers` and `spec-layout` reorganisations relocated whole
subtrees, so a summary written before the move cites a path that no longer
resolves while the file itself is alive under a new name.

This tool finds a successor for such a path and rewrites the citation. It never
guesses. A path with several plausible successors, and a path with none, are
both LEFT ALONE and counted, because a citation silently rewritten to the wrong
file is worse than a dead one: the dead one is still detectable by
`scripts/dev/learned_staleness.py`, and the wrong one reads as true.

Three resolvers run, strongest evidence first:

  1. **Recorded rename.** `git log --diff-filter=R` over HEAD gives old -> new
     pairs that git itself recorded. The chain is followed transitively (A moved
     to B, B moved to C) until it reaches a path that exists. This is evidence
     about the exact file, so it wins whenever it answers.
  2. **Learned directory rule.** Those same recorded renames are collapsed into
     directory-level rules: many files moving from `internal/plugins/fibkernel/`
     to `internal/plugins/fib/kernel/` IS the evidence that the directory moved.
     A rule fires only when it is unambiguous (one destination) and only when
     the rewritten path actually exists. A file deleted during a subtree move
     therefore stays dead rather than being invented.
  3. **Unique three-segment suffix.** A path ending in `cmd/bfd/yang/embed.go`
     matching exactly one tracked file is that file. Two segments is not enough:
     `config/register.go` matched an unrelated BGP package and
     `updater/updater.go` matched a VENDORED third-party file, both wrong.
     `vendor/` and `third_party/` are excluded outright, because ze source never
     moves into them.

The dead-path set comes from `learned_staleness.check`, and the reduction from
one backtick span to a concrete path comes from its `candidate_paths`, so the
repair tool and the gate can never disagree about which citation is dead
(`ai/rules/evidence.md`).

Not rewritten, on purpose:

  * A `{a,b}` brace span whose members disagree. Such a span is repaired only
    when EVERY expansion is dead and every one resolves to the same directory
    swap, which is the case where the members moved together and one head
    substitution says so exactly. Any other brace span is left alone.
  * A `..` traversal token. `learned_staleness` reports it rather than resolving
    it, and this tool inherits that refusal.
  * Anything outside a `## Files` section. The gate only checks those, and prose
    elsewhere often names a path historically ("we deleted X"), where a rewrite
    would falsify the sentence.

Usage:
    python3 scripts/dev/learned_repath.py            # report, write nothing
    python3 scripts/dev/learned_repath.py --check    # alias of the above
    python3 scripts/dev/learned_repath.py --apply    # rewrite the summaries
    python3 scripts/dev/learned_repath.py --json     # machine-readable report
"""

from __future__ import annotations

import argparse
import json
import os
import re
import stat
import subprocess
import sys
import tempfile
from collections import defaultdict
from pathlib import Path

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from learned_staleness import (
    BACKTICK,
    candidate_paths,
    check,
    files_section_lines,
    path_problem,
    summary_files,
)

# A suffix match must cover at least this many trailing path segments. Two was
# tried and rewrote `cmd/ze/config/register.go` into an unrelated BGP package:
# both `config` and `register.go` are generic, so the pair carried no identity.
MIN_SUFFIX_SEGMENTS = 3
# Never a successor: ze source does not move into vendored third-party trees.
# Without this, `.../updater/updater.go` resolved into `vendor/`.
NEVER_SUCCESSOR = ("vendor/", "third_party/")
# Bound on how far a rename chain is followed before it is called a cycle.
MAX_RENAME_HOPS = 20
# A span holding one of these needs the whole-span repair, not a token swap.
UNREWRITABLE = ("{", "}")
# Verdict for a dead path whose successor is known but whose SPAN cannot carry
# the repair. Distinct from `gone`, which is about the file, and from
# `ambiguous`, which is about the evidence: this one is about the citation.
SPAN_DECLINED = "unrepairable-span"
# Verdict for a citation the summary itself says was DELETED. The path resolves
# nowhere by design, and repointing it at the live successor turns a true
# sentence into a false one.
OBITUARY = "cited-as-deleted"

# The line labels its citations as deletions: `Deleted: a.go, b.go`.
DELETION_LABEL = re.compile(r"^\s*[-*|]?\s*(?:Deleted|Removed)\b\s*:", re.IGNORECASE)
# A deletion word standing alone against the citation: `` `a.go` -- deleted ``,
# `(deleted)`. It must be BARE. "removed temp cleanup" and "admin-distance
# removed" describe code taken OUT of a file that still exists, and those
# citations are repaired normally.
DELETION_BARE = re.compile(
    r"(?:--|—|\(|\|)\s*(?:deleted|removed|gone|retired)\b\s*(?:\)|\||$|\()",
    re.IGNORECASE,
)


def missing_sentinel(root: Path) -> str:
    """The exact `path_problem` verdict for a path that does not exist.

    Derived rather than spelled, so a reworded verdict in `learned_staleness`
    cannot silently make this tool select nothing and report a clean tree."""
    probe = "__ze_learned_repath_probe_that_does_not_exist__"
    verdict = path_problem(root, probe)
    if verdict is None:  # pragma: no cover - the probe name is not a real path
        raise RuntimeError(f"sentinel probe {probe!r} unexpectedly exists")
    return verdict


def tracked_paths(root: Path) -> tuple[set[str], set[str]]:
    """(files, directories) tracked by git, as repo-relative posix paths.

    Directories are derived from the file list: git tracks no directory of its
    own, and a summary may legitimately cite one (`internal/plugins/sysrib/`)."""
    out = subprocess.run(
        ["git", "-C", str(root), "ls-files", "-z"],
        capture_output=True,
        text=True,
        check=True,
    ).stdout
    files = {p for p in out.split("\0") if p}
    dirs: set[str] = set()
    for path in files:
        parts = path.split("/")
        for i in range(1, len(parts)):
            dirs.add("/".join(parts[:i]))
    return files, dirs


def rename_map(root: Path) -> dict[str, set[str]]:
    """old path -> every new path git recorded a rename to.

    HEAD history only, never `--all`: a repair that depends on which branches a
    checkout happens to have fetched is not reproducible. Measured over this
    corpus the two agree exactly, so the reproducible one is free."""
    proc = subprocess.run(
        [
            "git",
            "-C",
            str(root),
            "log",
            "--diff-filter=R",
            "--name-status",
            "--format=",
            "-M",
            "HEAD",
        ],
        capture_output=True,
        text=True,
        check=True,
    )
    renames: dict[str, set[str]] = defaultdict(set)
    for line in proc.stdout.splitlines():
        parts = line.rstrip("\n").split("\t")
        if len(parts) == 3 and parts[0].startswith("R"):
            renames[parts[1]].add(parts[2])
    return renames


def suffix_index(paths: set[str]) -> dict[str, set[str]]:
    """Every long-enough trailing suffix of every path, mapped to its owners.

    Shorter suffixes are deliberately absent: see MIN_SUFFIX_SEGMENTS."""
    index: dict[str, set[str]] = defaultdict(set)
    for path in paths:
        if path.startswith(NEVER_SUCCESSOR):
            continue
        parts = path.split("/")
        for i in range(len(parts) - (MIN_SUFFIX_SEGMENTS - 1)):
            index["/".join(parts[i:])].add(path)
    return index


def directory_rules(renames: dict[str, set[str]]) -> dict[str, str]:
    """Directory-level moves, collapsed out of git's per-file rename records.

    A rename `a/b/x.go -> c/d/x.go` has the trailing segments in common, so what
    it really records is `a/b -> c/d`. Many files agreeing on that pair is the
    evidence that a whole subtree moved, which then carries the files git did
    not individually record.

    A head that git saw moving to two different places is DROPPED, not
    majority-voted: a subtree that split has no single successor, and picking
    the more popular half would be the guess this tool exists to avoid."""
    votes: dict[str, set[str]] = defaultdict(set)
    for old, targets in renames.items():
        if len(targets) != 1:
            continue
        new = next(iter(targets))
        old_parts, new_parts = old.split("/"), new.split("/")
        shared = 0
        limit = min(len(old_parts), len(new_parts))
        while shared < limit and old_parts[-1 - shared] == new_parts[-1 - shared]:
            shared += 1
        if shared == 0:
            continue
        old_head = "/".join(old_parts[: len(old_parts) - shared])
        new_head = "/".join(new_parts[: len(new_parts) - shared])
        if old_head and new_head and old_head != new_head:
            votes[old_head].add(new_head)
    return {head: next(iter(t)) for head, t in votes.items() if len(t) == 1}


class Resolver:
    """Finds the one successor of a dead path, or refuses to answer.

    Every method returns a `(verdict, successor)` pair where verdict is
    `renamed`, `relocated`, `moved`, `ambiguous` or `gone`. Only the first three
    carry a successor; the other two are outcomes to report, never to act on."""

    def __init__(self, root: Path) -> None:
        self.files, self.dirs = tracked_paths(root)
        self.renames = rename_map(root)
        self.rules = directory_rules(self.renames)
        self.file_suffix = suffix_index(self.files)
        self.dir_suffix = suffix_index(self.dirs)

    def exists(self, path: str) -> bool:
        return path in self.files or path in self.dirs

    def by_rename(self, token: str) -> tuple[str, str | None]:
        """Follow git's recorded renames until the chain reaches a live path.

        A hop with several recorded targets ends the walk as ambiguous rather
        than picking one: git saw the file split, and so should the reader."""
        seen = {token}
        current = token
        for _ in range(MAX_RENAME_HOPS):
            targets = {t for t in self.renames.get(current, ()) if t not in seen}
            if not targets:
                return ("gone", None)
            if len(targets) > 1:
                return ("ambiguous", None)
            current = next(iter(targets))
            seen.add(current)
            if self.exists(current):
                return ("renamed", current)
        return ("ambiguous", None)

    def by_directory_rule(self, token: str) -> tuple[str, str | None]:
        """Apply every learned directory move whose head this token sits under.

        The rewritten path must EXIST. A rule proves where a directory went, not
        that any particular file survived the trip, so a file deleted during the
        move stays dead instead of being invented at a plausible address."""
        hits = set()
        for old_head, new_head in self.rules.items():
            if token != old_head and not token.startswith(old_head + "/"):
                continue
            candidate = new_head + token[len(old_head) :]
            if self.exists(candidate):
                hits.add(candidate)
        if len(hits) == 1:
            return ("relocated", next(iter(hits)))
        return ("ambiguous" if hits else "gone", None)

    def by_suffix(self, token: str, want_dir: bool) -> tuple[str, str | None]:
        """The single tracked path ending in the longest suffix of `token`.

        Longest first, so `isis/spf/ipv6.go` is tried before `spf/ipv6.go`. The
        first suffix length that matches anything decides the verdict: falling
        through to a shorter, weaker suffix after a multi-candidate hit would
        trade a refusal for a guess."""
        index = self.dir_suffix if want_dir else self.file_suffix
        parts = token.split("/")
        for i in range(len(parts) - (MIN_SUFFIX_SEGMENTS - 1)):
            owners = index.get("/".join(parts[i:]))
            if not owners:
                continue
            if len(owners) == 1:
                return ("moved", next(iter(owners)))
            return ("ambiguous", None)
        return ("gone", None)

    def resolve(self, raw_token: str) -> tuple[str, str | None]:
        """Resolve one dead token, strongest evidence first.

        A directory citation stays a directory citation: the trailing slash is
        stripped for matching and put back on the answer."""
        want_dir = raw_token.endswith("/")
        token = raw_token.rstrip("/")
        if ".." in token:
            return ("gone", None)

        for step in (
            lambda: self.by_rename(token),
            lambda: self.by_directory_rule(token),
            lambda: self.by_suffix(token, want_dir),
        ):
            verdict, successor = step()
            if successor is not None:
                return (verdict, successor + "/" if want_dir else successor)
            if verdict == "ambiguous":
                # A refusal by the stronger resolver is FINAL. Falling through
                # to a weaker one after it saw several candidates would turn
                # "the evidence is split" into a confident wrong answer.
                return ("ambiguous", None)
        return ("gone", None)


def cites_a_deletion(line: str) -> bool:
    """True when the line says the path it cites was DELETED.

    Such a citation is dead on purpose. The gate still reports it, and that is
    correct: the reader wants to know the file is gone. What must not happen is
    the tool repointing it at a live successor, which leaves the sentence
    reading `internal/core/hostload/x.go -- deleted` about a file that exists.

    Backtick spans are stripped before matching so a path or symbol containing
    the word cannot trigger it."""
    prose = BACKTICK.sub("", line)
    return bool(DELETION_LABEL.search(line) or DELETION_BARE.search(prose))


def common_prefix(values: list[str]) -> str:
    """The longest string every value starts with."""
    if not values:
        return ""
    head = values[0]
    for value in values[1:]:
        while not value.startswith(head):
            head = head[:-1]
    return head


def brace_rewrite(
    root: Path, raw: str, dead: set[str], resolver: Resolver
) -> tuple[str | None, list[str]]:
    """Repair a `{a,b}` span as ONE head substitution, or decline.

    `internal/test/runner/hostload_{linux,darwin}.go` expands to two dead paths
    that both moved to `internal/core/hostload/`. The span cannot be repaired
    token by token, because neither expansion appears literally in it, but it
    CAN be repaired by swapping the head every expansion shares.

    Three conditions, all required. Every expansion must be dead: a span with a
    live member would be broken by moving its head. Every expansion must
    resolve. And every successor must be that same head swap applied to that
    same expansion, which is what proves the members moved TOGETHER rather than
    coincidentally each having a successor somewhere."""
    tokens = candidate_paths(root, raw)
    stuck = [SPAN_DECLINED] * sum(1 for t in tokens if t in dead)
    if len(tokens) < 2 or any(t not in dead for t in tokens):
        return None, stuck

    verdicts: list[str] = []
    successors: list[str] = []
    for token in tokens:
        verdict, successor = resolver.resolve(token)
        if successor is None:
            return None, stuck
        verdicts.append(verdict)
        successors.append(successor)

    old_head = common_prefix(tokens)
    new_head = common_prefix(successors)
    if not old_head or not new_head or old_head == new_head:
        return None, stuck
    for token, successor in zip(tokens, successors):
        if successor != new_head + token[len(old_head) :]:
            return None, stuck
    if old_head not in raw:
        return None, stuck
    return raw.replace(old_head, new_head, 1), verdicts


def dead_tokens(root: Path) -> set[str]:
    """Every token `learned_staleness` reports as a path that does not exist.

    Sourced from the gate itself so the repair can never target a citation the
    gate considers fine, nor miss one it considers dead."""
    missing = missing_sentinel(root)
    return {f["token"] for f in check(root) if f["problem"] == missing}


def plan_file(
    root: Path, path: Path, dead: set[str], resolver: Resolver
) -> tuple[list[str], dict[str, int]]:
    """Rewritten lines for one summary, plus a verdict tally.

    Returns ([], tally) when nothing in this summary changes, so the caller can
    skip the write entirely rather than rewriting a file byte-identically."""
    text = path.read_text(encoding="utf-8")
    lines = text.splitlines(keepends=True)
    tally: dict[str, int] = defaultdict(int)
    changed = False

    for lineno, _ in files_section_lines(text):
        line = lines[lineno - 1]
        if cites_a_deletion(line):
            for raw in BACKTICK.findall(line):
                tally[OBITUARY] += sum(
                    1 for t in candidate_paths(root, raw) if t in dead
                )
            continue
        for raw in BACKTICK.findall(line):
            if any(marker in raw for marker in UNREWRITABLE):
                repaired, verdicts = brace_rewrite(root, raw, dead, resolver)
                for verdict in verdicts:
                    tally[verdict] += 1
                span = "`" + raw + "`"
                if repaired is not None and span in line:
                    line = line.replace(span, "`" + repaired + "`", 1)
                    changed = True
                continue
            for token in candidate_paths(root, raw):
                if token not in dead:
                    continue
                verdict, successor = resolver.resolve(token)
                tally[verdict] += 1
                if successor is None or successor == token:
                    continue
                # Replace the token inside its own backtick span only, so a
                # `:42` line suffix or trailing prose on the same line survives.
                span = "`" + raw + "`"
                fixed = "`" + raw.replace(token, successor, 1) + "`"
                if span in line:
                    line = line.replace(span, fixed, 1)
                    changed = True
        lines[lineno - 1] = line

    return (lines if changed else []), dict(tally)


def write_atomic(path: Path, lines: list[str]) -> None:
    """Replace `path` with `lines` without ever exposing a half-written file.

    Other sessions read these summaries concurrently, so the new content lands
    by rename onto the same filesystem, never by truncate-then-write.

    The temp name is unique per call, not `<name>.repath.tmp`: two concurrent
    `--apply` runs sharing one fixed name interleave their writes into it and
    then each rename the other's half of the result into place. It is also
    removed on failure rather than orphaned beside the summary.

    The original mode is carried across. `mkstemp` creates at 0600 and
    `os.replace` keeps the source's mode, so without this every repaired summary
    is narrowed from the corpus baseline of 0644 to owner-only, invisibly: git
    tracks the exec bit alone."""
    mode = stat.S_IMODE(os.stat(path).st_mode)
    fd, tmp = tempfile.mkstemp(
        dir=str(path.parent), prefix=path.name, suffix=".repath.tmp"
    )
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            handle.write("".join(lines))
        os.chmod(tmp, mode)
        os.replace(tmp, path)
    except BaseException:
        if os.path.exists(tmp):
            os.unlink(tmp)
        raise


def run(root: Path, apply: bool) -> dict:
    dead = dead_tokens(root)
    resolver = Resolver(root)
    totals: dict[str, int] = defaultdict(int)
    rewritten: list[str] = []
    unresolved: dict[str, str] = {}

    for path in summary_files(root):
        lines, tally = plan_file(root, path, dead, resolver)
        for verdict, count in tally.items():
            totals[verdict] += count
        if lines:
            rel = path.relative_to(root).as_posix()
            rewritten.append(rel)
            if apply:
                write_atomic(path, lines)

    for token in sorted(dead):
        verdict, successor = resolver.resolve(token)
        if successor is None:
            unresolved[token] = verdict

    return {
        "applied": apply,
        "dead_tokens": len(dead),
        "verdicts": dict(totals),
        "summaries_rewritten": len(rewritten),
        "rewritten": sorted(rewritten),
        "unresolved": unresolved,
    }


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
        help="report what would be repaired and write nothing (the default)",
    )
    parser.add_argument(
        "--apply", action="store_true", help="rewrite the summaries in place"
    )
    parser.add_argument("--json", action="store_true", help="machine-readable report")
    args = parser.parse_args(sys.argv[1:] if argv is None else argv)
    root = Path(args.repo).resolve()

    report = run(root, apply=args.apply)

    if args.json:
        print(json.dumps(report, indent=2, sort_keys=True))
        return 0

    verdicts = report["verdicts"]
    verb = "repointed" if report["applied"] else "would repoint"
    print(
        f"learned repath: {verdicts.get('renamed', 0)} citation(s) {verb} from a "
        f"recorded git rename, {verdicts.get('relocated', 0)} from a learned "
        f"directory move, {verdicts.get('moved', 0)} from a unique "
        f"{MIN_SUFFIX_SEGMENTS}-segment suffix, across "
        f"{report['summaries_rewritten']} summary(ies)"
    )
    print(
        f"  left alone: {verdicts.get('ambiguous', 0)} ambiguous (several "
        f"plausible successors), {verdicts.get(SPAN_DECLINED, 0)} in a brace "
        f"span whose members disagree, {verdicts.get(OBITUARY, 0)} the summary "
        f"itself calls deleted, {verdicts.get('gone', 0)} with no successor "
        f"(the file is genuinely deleted)"
    )
    if not report["applied"]:
        print("  nothing was written; re-run with --apply to repair")
    return 0


if __name__ == "__main__":
    sys.exit(main())
