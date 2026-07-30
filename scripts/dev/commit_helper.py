#!/usr/bin/env python3
"""Generate safe user-run commit scripts for Ze."""

from __future__ import annotations

import argparse
import importlib.util
import json
import os
import re
import secrets
import shlex
import shutil
import stat
import subprocess
import sys
import tempfile
import textwrap
import time
from dataclasses import dataclass
from pathlib import Path

# Discovery-index generators/outputs and the source-trigger predicate are shared
# with the changed-file router (verify_wiring_docs.py) via discovery_sources.py,
# so the commit gate here and the router cannot drift apart.
from discovery_sources import GENERATORS as DISCOVERY_INDEX_GENERATORS
from discovery_sources import OUTPUTS as DISCOVERY_INDEX_OUTPUTS
from discovery_sources import STALE_EXIT as DISCOVERY_STALE_EXIT
from discovery_sources import indexes_fed_by as _indexes_fed_by
from discovery_sources import is_discovery_source as _is_discovery_source

SESSION_RE = re.compile(r"^[0-9a-f]{8}$")
TAG_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,31}$")
LEARNED_RE = re.compile(r"^plan/learned/[0-9]{3,}-.+\.md$")
COMMIT_MESSAGE_WIDTH = 72


LESSON_WORTHY_PREFIXES = (
    "ai/rules/",
    "ai/skills/",
    "internal/plugins/skills/",
    "scripts/dev/",
    "scripts/docvalid/",
    "scripts/inventory/",
    "scripts/lint/",
    "scripts/status/",
    "mk/",
    "docs/contributing/",
)

LESSON_WORTHY_FILES = {
    "Makefile",
    "ai/INDEX.md",
    "ai/LEARNED-INDEX.md",
    "ai/INSTRUCTIONS.md",
}

FORBIDDEN_COMMIT_SCRIPT_PATHS = {
    "AGENTS.md",
    "CLAUDE.md",
}


@dataclass(frozen=True)
class CommitBlock:
    tag: str
    subject: str
    add_paths: tuple[str, ...]
    remove_paths: tuple[str, ...]
    message_path: str
    lesson_comment: str
    # A shell line re-run inside the generated script (under `set -e`) before the
    # git commands, so a spec-closure commit re-verifies the review gate at
    # commit-RUN time, not only at script-generation time. Closes the edit-after-
    # generate hole. Empty when not a closure or when the owner overrode the gate.
    review_check: str = ""


class UsageError(Exception):
    pass


def run_git(
    repo: Path, *args: str, check: bool = True
) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(
        ("git", "-C", str(repo), *args),
        check=False,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if check and result.returncode != 0:
        detail = result.stderr.strip() or result.stdout.strip()
        raise UsageError(f"git {' '.join(args)} failed: {detail}")
    return result


def repo_root(start: str | None) -> Path:
    cwd = Path(start or ".")
    result = run_git(cwd, "rev-parse", "--show-toplevel")
    return Path(result.stdout.strip()).resolve()


def rel_path(repo: Path, raw: str) -> str:
    if not raw:
        raise UsageError("empty path is not allowed")
    path = Path(raw)
    if path.is_absolute():
        try:
            rel = path.resolve().relative_to(repo)
        except ValueError as exc:
            raise UsageError(f"path is outside repository: {raw}") from exc
    else:
        rel = Path(os.path.normpath(raw))
        if str(rel) == ".":
            raise UsageError("repository root is not an explicit file path")
        if str(rel).startswith(".."):
            raise UsageError(f"path is outside repository: {raw}")
    rel_s = rel.as_posix()
    if rel_s.startswith(".git/") or rel_s == ".git":
        raise UsageError(f"git internals cannot be committed: {rel_s}")
    return rel_s


def unique_paths(paths: list[str]) -> tuple[str, ...]:
    seen: set[str] = set()
    out: list[str] = []
    for path in paths:
        if path in seen:
            continue
        seen.add(path)
        out.append(path)
    return tuple(out)


def ignored(repo: Path, path: str) -> bool:
    result = run_git(repo, "check-ignore", "--no-index", "-q", "--", path, check=False)
    return result.returncode == 0


def tracked(repo: Path, path: str) -> bool:
    result = run_git(repo, "ls-files", "--error-unmatch", "--", path, check=False)
    return result.returncode == 0


def validate_add_path(repo: Path, path: str) -> None:
    if path in FORBIDDEN_COMMIT_SCRIPT_PATHS:
        raise UsageError(f"generated agent file must not be committed: {path}")
    if ignored(repo, path):
        raise UsageError(f"ignored path must not be committed: {path}")
    full = repo / path
    if not full.exists():
        raise UsageError(
            f"path does not exist, use --remove for tracked deletions: {path}"
        )
    if full.is_dir():
        raise UsageError(
            f"commit scripts require explicit files, not directories: {path}"
        )


def validate_remove_path(repo: Path, path: str) -> None:
    if not tracked(repo, path):
        raise UsageError(f"--remove path is not tracked: {path}")


# The commit-script session file (tmp/commit-session-id-<id>) MUST key on the SAME
# session id the hooks use. This once carried a THIRD, independent derivation
# (JWT-first / comm-walk / getppid) that drifted from both hook resolvers for weeks
# (spec-fixit-session-id-collision). It now delegates to the ONE shared resolver.
_SID_MODULE_PATH = (
    Path(__file__).resolve().parents[2] / ".claude" / "hooks" / "lib" / "session_id.py"
)
_sid_spec = importlib.util.spec_from_file_location(
    "ze_session_id", str(_SID_MODULE_PATH)
)
_ze_session_id = importlib.util.module_from_spec(_sid_spec)
_sid_spec.loader.exec_module(_ze_session_id)


def claude_session_fingerprint() -> str:
    """Identify the Claude session that owns this process.

    Concurrent Claude sessions share tmp/; when they also shared one
    tmp/commit-session-id they shared one tmp/commit-<SESSION>.sh, and a --replace
    from one session silently overwrote the other's prepared script (observed
    2026-06-10). Delegates to the ONE shared session-id resolver
    (.claude/hooks/lib/session_id.py), so this file keys its session script on the
    exact id the hooks use -- no fourth spelling to drift. The resolver already
    guarantees the id is a safe filename component.
    """
    return _ze_session_id.session_id()


def session_id(repo: Path, requested: str | None) -> str:
    tmp_dir = repo / "tmp"
    tmp_dir.mkdir(exist_ok=True)
    # Per-Claude-session file: concurrent sessions must never resolve to
    # the same tmp/commit-<SESSION>.sh script path.
    session_file = tmp_dir / f"commit-session-id-{claude_session_fingerprint()}"
    if requested:
        session = requested.lower()
        if not SESSION_RE.match(session):
            raise UsageError("--session must be 8 lowercase hexadecimal characters")
        session_file.write_text(session + "\n", encoding="utf-8")
        return session
    if session_file.exists():
        existing = session_file.read_text(encoding="utf-8").strip().lower()
        if SESSION_RE.match(existing):
            return existing
    session = secrets.token_hex(4)
    session_file.write_text(session + "\n", encoding="utf-8")
    return session


def next_tag(repo: Path, session: str) -> str:
    used = {
        path.name[len(f"commit-msg-{session}-") : -len(".txt")]
        for path in (repo / "tmp").glob(f"commit-msg-{session}-*.txt")
        if path.name.endswith(".txt")
    }
    for code in range(ord("a"), ord("z") + 1):
        tag = chr(code)
        if tag not in used:
            return tag
    return f"n{len(used) + 1}"


def normalize_tag(tag: str | None, repo: Path, session: str) -> str:
    resolved = tag or next_tag(repo, session)
    if not TAG_RE.match(resolved):
        raise UsageError(
            "--tag must start with an alphanumeric character and contain only alnum, dot, underscore, or dash"
        )
    return resolved


def wrap_commit_body_line(line: str) -> list[str]:
    indent_len = len(line) - len(line.lstrip())
    indent = line[:indent_len]
    content = line[indent_len:]
    bullet = re.match(r"^((?:[-*+]|\d+[.)])\s+)(.*)$", content)
    if bullet:
        initial_indent = indent + bullet.group(1)
        subsequent_indent = " " * len(initial_indent)
        content = bullet.group(2)
    else:
        initial_indent = indent
        subsequent_indent = indent
    wrapped = textwrap.wrap(
        content,
        width=COMMIT_MESSAGE_WIDTH,
        initial_indent=initial_indent,
        subsequent_indent=subsequent_indent,
        break_long_words=False,
        break_on_hyphens=False,
    )
    if not wrapped:
        return [line]
    for wrapped_line in wrapped:
        if len(wrapped_line) > COMMIT_MESSAGE_WIDTH:
            raise UsageError(
                "--body contains an unwrappable line longer than "
                f"{COMMIT_MESSAGE_WIDTH} characters"
            )
    return wrapped


def wrap_commit_body(body: list[str]) -> str:
    wrapped: list[str] = []
    for chunk in body:
        if not chunk:
            wrapped.append("")
            continue
        for raw_line in chunk.splitlines():
            line = raw_line.rstrip()
            if not line.strip():
                wrapped.append("")
                continue
            wrapped.extend(wrap_commit_body_line(line))
    while wrapped and wrapped[0] == "":
        wrapped.pop(0)
    while wrapped and wrapped[-1] == "":
        wrapped.pop()
    return "\n".join(wrapped)


def message_text(subject: str, body: list[str]) -> str:
    cleaned_subject = subject.strip()
    if not cleaned_subject:
        raise UsageError("--subject is required")
    if "\n" in cleaned_subject:
        raise UsageError("--subject must be a single line")
    if len(cleaned_subject) > COMMIT_MESSAGE_WIDTH:
        raise UsageError(f"--subject must be at most {COMMIT_MESSAGE_WIDTH} characters")
    parts = [cleaned_subject]
    cleaned_body = wrap_commit_body(body)
    if cleaned_body:
        parts.extend(("", cleaned_body))
    return "\n".join(parts) + "\n"


def lesson_worthy(paths: tuple[str, ...]) -> bool:
    for path in paths:
        if path in LESSON_WORTHY_FILES:
            return True
        if path.startswith(LESSON_WORTHY_PREFIXES):
            return True
    return False


def learned_paths(paths: tuple[str, ...]) -> tuple[str, ...]:
    return tuple(path for path in paths if LEARNED_RE.match(path))


SPEC_PATH_RE = re.compile(r"^plan/spec-.+\.md$")


def closure_reminder(
    add_paths: tuple[str, ...], remove_paths: tuple[str, ...]
) -> str | None:
    """Warn when a commit adds a learned summary but closes no spec.

    The two-commit spec closure is: commit A adds plan/learned/NNN-*.md (this
    commit), commit B does `git rm plan/spec-<stem>.md`. Commit B is the step
    that gets dropped, orphaning the spec in plan/ with Status=in-progress. If
    this commit adds a learned summary and removes no spec, nudge the caller to
    prepare the closure commit next. See ai/rules/planning.md "Spec Closure".
    """
    if not learned_paths(add_paths):
        return None
    if any(SPEC_PATH_RE.match(path) for path in remove_paths):
        return None
    return (
        "closure-reminder: this commit adds a learned summary but removes no "
        "spec.\n"
        "  If it completes a spec, prepare the closure commit next:\n"
        "    git rm plan/spec-<stem>.md   (ai/rules/planning.md Spec Closure)\n"
        "  List what is still open: scripts/dev/spec-closure-check.py --list"
    )


def lesson_comment(
    add_paths: tuple[str, ...],
    remove_paths: tuple[str, ...],
    required: bool,
    lesson_not_needed: str | None,
) -> str:
    paths = add_paths + remove_paths
    learned = learned_paths(add_paths)
    reason = ""
    if lesson_not_needed is not None:
        reason = lesson_not_needed.strip()
        if len(reason) < 12:
            raise UsageError("--lesson-not-needed reason is too short")
    if required and lesson_not_needed:
        raise UsageError(
            "--lesson-required cannot be combined with --lesson-not-needed"
        )
    if learned:
        return "Lesson: " + ", ".join(learned)
    if required:
        raise UsageError("lesson is required; include plan/learned/NNN-name.md")
    if lesson_worthy(paths):
        if not reason:
            raise UsageError(
                "lesson-worthy paths changed; include plan/learned/NNN-name.md or pass --lesson-not-needed with the reason"
            )
        return "Lesson: not needed - " + reason
    if reason:
        return "Lesson: not needed - " + reason
    return "Lesson: not required by helper heuristic"


def quote_paths(paths: tuple[str, ...]) -> str:
    return " ".join(shlex.quote(path) for path in paths)


def render_git_add(paths: tuple[str, ...]) -> str:
    lines = ["git add -- \\"]
    for index, path in enumerate(paths):
        suffix = " \\" if index + 1 < len(paths) else ""
        lines.append("  " + shlex.quote(path) + suffix)
    return "\n".join(lines)


def render_staging_guard(paths: tuple[str, ...]) -> str:
    """Guard for the generated commit script: abort if the shared git index holds
    staged paths this commit did not stage.

    Concurrent Claude sessions share one working tree AND one git index, so a
    sibling's leftover `git add` (e.g. a commit that failed at gpg signing) would
    otherwise be swept into THIS commit. The guard runs after this commit's own
    add/rm, so any remaining staged-but-unexpected path is foreign -> abort.
    """
    if not paths:
        return ""
    expected = " ".join("-e " + shlex.quote(p) for p in sorted(set(paths)))
    return "\n".join(
        [
            "# Concurrency guard: refuse to sweep a concurrent session's staged files",
            "# into this commit (sessions share one working tree + git index).",
            # core.quotePath=false: git otherwise C-quotes non-ASCII paths (café ->
            # "caf\303\251"), which the raw grep pattern would miss -> false abort.
            f"_ze_foreign=$(git -c core.quotePath=false diff --cached --name-only | grep -vxF {expected} || true)",
            'if [ -n "$_ze_foreign" ]; then',
            '  echo "ABORT: index has staged files not in this commit (concurrent session?):" >&2',
            '  echo "$_ze_foreign" >&2',
            "  exit 1",
            "fi",
        ]
    )


def render_block(block: CommitBlock) -> str:
    lines = [
        f"# Commit {block.tag}: {block.subject}",
        f"# {block.lesson_comment}",
    ]
    if block.review_check:
        lines.append("# critical-review gate re-check (ai/rules/critical-review.md)")
        lines.append(block.review_check)
    if block.add_paths:
        lines.append(render_git_add(block.add_paths))
    if block.remove_paths:
        lines.append("git rm -- " + quote_paths(block.remove_paths))
    guard = render_staging_guard(block.add_paths + block.remove_paths)
    if guard:
        lines.append(guard)
    lines.append("git commit -F " + shlex.quote(block.message_path))
    return "\n".join(lines) + "\n"


def write_outputs(
    repo: Path,
    session: str,
    block: CommitBlock,
    message: str,
    append: bool,
    replace: bool,
    dry_run: bool,
) -> Path:
    script = repo / "tmp" / f"commit-{session}.sh"
    message_file = repo / block.message_path
    if script.exists() and not append and not replace:
        raise UsageError(
            f"{script.relative_to(repo)} exists; pass --append or --replace"
        )
    if append and replace:
        raise UsageError("--append and --replace are mutually exclusive")
    header = '#!/bin/bash\nset -euo pipefail\ncd "$(git rev-parse --show-toplevel)"\n\n'
    block_text = render_block(block)
    if dry_run:
        print(f"session={session}")
        print(f"message={block.message_path}")
        print(f"script={script.relative_to(repo).as_posix()}")
        print("--- message ---")
        print(message, end="")
        print("--- script ---")
        if append and script.exists():
            print(script.read_text(encoding="utf-8"), end="")
        else:
            print(header, end="")
        print(block_text, end="")
        return script
    message_file.write_text(message, encoding="utf-8")
    if append and script.exists():
        existing = script.read_text(encoding="utf-8")
        with script.open("a", encoding="utf-8") as fh:
            if existing and not existing.endswith("\n"):
                fh.write("\n")
            fh.write("\n")
            fh.write(block_text)
    else:
        script.write_text(header + block_text, encoding="utf-8")
    script.chmod(script.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
    return script


def verify_status(repo: Path) -> tuple[str, str]:
    """Return (state, detail) from scripts/dev/verify-status.sh check.

    state is "fresh", "stale", or "unknown". Only "stale" blocks a commit:
    the gate enforces a CONFIRMED red, it never invents one when the checker is
    unavailable (missing script, isolated test repo, minimal checkout). This
    never raises.
    """
    script = repo / "scripts" / "dev" / "verify-status.sh"
    if not script.exists():
        return "unknown", "verify-status.sh not found"
    try:
        proc = subprocess.run(
            [str(script), "check"], cwd=repo, capture_output=True, text=True
        )
    except OSError as exc:
        return "unknown", f"verify-status.sh did not run: {exc}"
    out = (proc.stdout or proc.stderr or "").strip().splitlines()
    return ("fresh" if proc.returncode == 0 else "stale"), (out[0] if out else "")


# Deterministic structural gates in `make ze-verify` (the non-test stages in
# scripts/status/verify_run.go stagesForMode). Unlike the unit/functional/exabgp
# TEST stages, these NEVER fail for flaky or environmental reasons: a red means
# the tree is structurally broken -- a module-tier misplacement, a lint or vet
# violation, a broken plugin boundary, an unresolved iface, a stale generated
# file, or a stale wiring index. They are therefore NOT eligible to be parked
# in plan/known-failures/
# or waved through with --unverified. Every name here MUST be a stage that
# stagesForMode actually emits, or it matches nothing and gates nothing;
# that is enforced by TestStructuralGatesAreLiveStages (Go, scripts/status)
# and test_structural_gates_are_live_stages (Python, scripts/dev).
# See ai/rules/git-safety.md.
STRUCTURAL_GATES = frozenset(
    {
        "ze-lint",
        "ze-lint-changed",
        "ze-tier-check",
        "ze-iface-resolution-check",
        "ze-plugin-boundary-check",
        "ze-regen-check-readonly",
        "ze-verify-wiring-docs",
        "ze-vet-evidence",
    }
)


def structural_gate_reds(repo: Path) -> list[str]:
    """Structural-gate stages recorded red by the last `make ze-verify` run.

    Reads tmp/ze-verify-failures.json, which verify_run.go rewrites after EVERY
    run (green or red, unconditionally), so a stale red cannot linger past a
    green verify: a fixed-and-reverified tree clears this. Returns [] when the
    artifact is missing or unreadable -- mirroring verify_status(), the gate
    never invents a red it cannot confirm. Preserves stage order.
    """
    path = repo / "tmp" / "ze-verify-failures.json"
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return []
    reds: list[str] = []
    for st in data.get("stages", []) if isinstance(data, dict) else []:
        if not isinstance(st, dict):
            continue
        if st.get("exit_code", 0) != 0 and st.get("stage") in STRUCTURAL_GATES:
            reds.append(st["stage"])
    return reds


def _read_head(path: Path, n: int) -> str:
    try:
        with open(path, encoding="utf-8", errors="replace") as fh:
            return "".join(line for _, line in zip(range(n), fh))
    except OSError:
        return ""


def feeds_discovery_index(repo: Path, path: str) -> bool:
    """True if committing `path` can change a generated discovery index.

    Path rules are shared with verify_wiring_docs via discovery_sources.py; the
    `// Package` / `// Design:` markers are matched against the working-tree
    file's first 40 lines (the header the indexes derive from).
    """
    header = ""
    if path.endswith(".go") and not path.endswith("_test.go"):
        header = _read_head(repo / path, 40)
    return _is_discovery_source(path, header)


def discovery_index_freshness(repo: Path) -> tuple[str, list[str]]:
    """Return (state, stale) where state is "fresh", "stale", or "unknown".

    Only a CONFIRMED stale index (a generator reports its committed output no
    longer matches the tree) blocks a commit. A generator that errors for an
    unrelated reason (missing dirs, minimal checkout) yields "unknown" and never
    blocks, mirroring verify_status().
    """
    stale: list[str] = []
    confirmed = False
    for gen, out in zip(DISCOVERY_INDEX_GENERATORS, DISCOVERY_INDEX_OUTPUTS):
        script = repo / gen
        if not script.exists():
            continue
        try:
            proc = subprocess.run(
                [sys.executable, str(script), "--check"],
                cwd=repo,
                capture_output=True,
                text=True,
            )
        except OSError:
            continue
        if proc.returncode == 0:
            confirmed = True
        elif proc.returncode == DISCOVERY_STALE_EXIT:
            # The generators' documented "output no longer matches its sources"
            # code. Matching on the warning TEXT here (the old form) meant any
            # rewording downgraded this BLOCKING gate to warn-only: the nonzero
            # exit would read as "generator failed", the index would be treated as
            # unjudgeable, and the commit would pass.
            confirmed = True
            stale.append(out)
        # any other nonzero exit: generator error, treat as unknown (skip)
    if not confirmed:
        return "unknown", []
    return ("stale" if stale else "fresh"), stale


def index_pending(repo: Path, path: str) -> bool:
    """True if a discovery index has uncommitted changes (new or modified)."""
    if not tracked(repo, path):
        return (repo / path).exists()
    result = run_git(repo, "diff", "--quiet", "HEAD", "--", path, check=False)
    return result.returncode != 0


def extract_head_into(repo: Path, dest: Path) -> None:
    """Materialize HEAD (minus `vendor/`) into `dest`.

    `vendor/` is excluded: every discovery generator skips it (`SKIP_DIRS` in
    package_map.py and docs_to_code.py; learned_index.py only reads
    plan/learned/), and it is a third of the archive -- ~138MB of extraction
    becomes ~98MB. `dest` MUST be empty -- `tar -x` overwrites archived paths but
    never removes extras, so anything already there survives into the view and can
    make an incoherent index look coherent.
    """
    if not rev_parse_head_exists(repo):
        raise FileNotFoundError("HEAD does not exist (repository has no commits)")
    dest.mkdir(parents=True, exist_ok=True)
    tar_path = dest.parent / (dest.name + ".tar")
    try:
        with open(tar_path, "wb") as fh:
            proc = subprocess.run(
                ["git", "archive", "--format=tar", "HEAD", ":(exclude)vendor"],
                cwd=repo,
                stdout=fh,
                stderr=subprocess.PIPE,
                text=True,
            )
        if proc.returncode != 0:
            raise OSError(f"git archive failed: {(proc.stderr or '').strip()}")
        subprocess.run(
            ["tar", "-xf", str(tar_path), "-C", str(dest)],
            check=True,
            capture_output=True,
        )
    finally:
        tar_path.unlink(missing_ok=True)


def apply_commit_overlay(
    repo: Path, add_paths: tuple[str, ...], remove_paths: tuple[str, ...], dest: Path
) -> None:
    """Turn an extracted HEAD tree into the tree this commit will PRODUCE.

    Split from extract_head_into so the SAME tree can be judged twice: once
    pristine (what HEAD already committed) and once with this overlay applied
    (what this commit produces). See discovery_index_verdicts.
    """
    for rel in add_paths:
        src = repo / rel
        if not src.is_file():
            continue
        target = dest / rel
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(src.read_bytes())
    for rel in remove_paths:
        target = dest / rel
        if target.is_dir():
            shutil.rmtree(target, ignore_errors=True)
        else:
            target.unlink(missing_ok=True)


def rev_parse_head_exists(repo: Path) -> bool:
    """True when HEAD names a commit. A fresh `git init` has none."""
    return (
        run_git(repo, "rev-parse", "--verify", "-q", "HEAD", check=False).returncode
        == 0
    )


def _judge_indexes(
    root: Path, candidates: list[str], verbose: bool
) -> tuple[list[str], list[str]]:
    """Run each candidate index's generator against the materialized tree `root`.

    Returns (stale, unjudged). A generator missing from `root`, one that wedges,
    and one that exits nonzero for any reason OTHER than DISCOVERY_STALE_EXIT are
    all UNJUDGED: the index is neither confirmed stale nor confirmed coherent, and
    "the generator broke" must never be read as "the index drifted".

    The generator is taken from `root`, not the working tree: for the commit view
    `add_paths` already overwrote it when the commit changes a generator, so the
    tree judges this commit with the code the commit ships, not with a concurrent
    session's uncommitted generator edits.
    """
    gen_for = dict(zip(DISCOVERY_INDEX_OUTPUTS, DISCOVERY_INDEX_GENERATORS))
    stale: list[str] = []
    unjudged: list[str] = []
    for out in candidates:
        gen = gen_for.get(out)
        if gen is None or not (root / gen).exists():
            unjudged.append(out)  # nothing in this tree can judge it
            continue
        try:
            proc = subprocess.run(
                [sys.executable, str(root / gen), "--check", "--root", str(root)],
                cwd=root,
                capture_output=True,
                text=True,
                timeout=120,
            )
        except subprocess.TimeoutExpired:
            # Caught here, not by the view-build handler in the caller: the tree
            # was built fine, this ONE generator wedged. Reporting it as "could not
            # build the commit view" would send the reader to the wrong place.
            if verbose:
                print(
                    f"warning: {gen} timed out judging {out} in the commit view; "
                    "treating it as unjudgeable.",
                    file=sys.stderr,
                )
            unjudged.append(out)
            continue
        if proc.returncode == 0:
            continue
        if proc.returncode == DISCOVERY_STALE_EXIT:
            stale.append(out)
            continue
        # Any other nonzero exit is the generator failing, not the index drifting.
        # Mirror discovery_index_freshness: unjudgeable, not stale. Show what it
        # said -- the "run make ze-regen" advice cannot fix a crash.
        if verbose:
            output = ((proc.stdout or "") + (proc.stderr or "")).strip()[:400]
            print(
                f"warning: {gen} could not judge {out} in the commit view "
                f"(exit {proc.returncode}): {output}",
                file=sys.stderr,
            )
        unjudged.append(out)
    return stale, unjudged


@dataclass(frozen=True)
class DiscoveryVerdicts:
    """Both discovery-index questions, answered from ONE materialization of HEAD.

    `head_*` is HEAD's committed indexes vs HEAD's committed sources (did a prior
    commit bypass the gate?); `view_*` is the tree THIS commit produces. The two
    trees differ only by the commit's own overlay, so extracting the repo twice --
    which is what two independent functions used to do, once WITHOUT the `vendor/`
    exclusion and into the system temp dir -- bought nothing but ~7s and ~236MB of
    I/O on every `create`.
    """

    candidates: list[str]
    head_state: str  # "fresh" | "stale" | "unknown"
    head_stale: list[str]
    view_stale: list[str]
    view_unjudged: list[str]
    view_judged: bool
    view_error: str | None


COMMIT_VIEW_TTL_SECONDS = 24 * 60 * 60


def _sweep_stale_commit_views(repo: Path) -> None:
    """Reap `tmp/commit-view-*` trees and tars a killed run left behind.

    Each view is a ~98MB extracted tree plus the ~90MB tar it came from. The
    `finally` in discovery_index_verdicts removes them on every normal AND
    exceptional exit, but SIGKILL, an OOM kill, and power loss run no `finally`,
    and nothing else in the repo reaps them: they accumulate in the project tmp/
    at ~190MB apiece.

    Deliberately HERE rather than beside the other stale-marker sweeps in
    `_cleanup_stale_markers` (.claude/hooks/lib/state-file.sh): that function runs
    only from Claude Code session hooks and only knows tmp/session/ markers, while
    commit_helper.py is a standalone tool that humans, CI, and the fixture tests
    also run. Sweeping in the producer means whatever next reaches the code that
    creates these directories also collects them, with no harness required.

    One day old, so a concurrent session's LIVE view (which lasts seconds) can
    never be in scope.
    """
    scratch = repo / "tmp"
    if not scratch.is_dir():
        return
    cutoff = time.time() - COMMIT_VIEW_TTL_SECONDS
    for leftover in scratch.glob("commit-view-*"):
        try:
            if leftover.stat().st_mtime >= cutoff:
                continue
            if leftover.is_dir():
                shutil.rmtree(leftover, ignore_errors=True)
            else:
                leftover.unlink(missing_ok=True)
        except OSError:
            continue  # a permission wall or a racing sweep: not ours to fix


def discovery_index_verdicts(
    repo: Path,
    add_paths: tuple[str, ...] = (),
    remove_paths: tuple[str, ...] = (),
    candidates: list[str] | None = None,
) -> DiscoveryVerdicts:
    """Judge HEAD and the tree this commit produces, from ONE extracted tree.

    Order is load-bearing: the HEAD pass MUST run BEFORE the overlay is applied,
    or the commit's own not-yet-committed files would be counted as committed and
    the HEAD verdict would be a verdict on nothing.

    A working-tree check cannot tell an index THIS commit leaves stale from one a
    CONCURRENT session dirtied with uncommitted sources -- and the remediation it
    prints for the second case ("regenerate and include it") is actively wrong: it
    would commit an index row pointing at a file absent from HEAD, which is red for
    everyone else. The view pass answers the question that actually matters, by
    running the real generators against HEAD plus this commit's own files.
    """
    if candidates is None:
        gen_for = dict(zip(DISCOVERY_INDEX_OUTPUTS, DISCOVERY_INDEX_GENERATORS))
        candidates = sorted(
            out for out in DISCOVERY_INDEX_OUTPUTS if (repo / gen_for[out]).exists()
        )
    # Inside the try: creating the scratch dir is itself a step that can fail (a
    # root-owned tmp/ left by `sudo make ze-netns-test`, ENOSPC, read-only FS), and
    # outside it that failure escaped as an uncaught traceback instead of degrading.
    dest = None
    try:
        _sweep_stale_commit_views(repo)
        dest = Path(tempfile.mkdtemp(prefix="commit-view-", dir=_scratch_dir(repo)))
        extract_head_into(repo, dest)
        # 1. HEAD exactly as committed. Quiet: discovery_index_head_status only
        #    WARNS on its result and has never reported a broken generator.
        head_stale, head_unjudged = _judge_indexes(
            dest, list(DISCOVERY_INDEX_OUTPUTS), verbose=False
        )
        confirmed = len(DISCOVERY_INDEX_OUTPUTS) > len(head_unjudged)
        head_state = (
            "unknown" if not confirmed else ("stale" if head_stale else "fresh")
        )
        # 2. The same tree, now carrying this commit's own files.
        apply_commit_overlay(repo, add_paths, remove_paths, dest)
        view_stale, view_unjudged = _judge_indexes(dest, candidates, verbose=True)
        return DiscoveryVerdicts(
            candidates=candidates,
            head_state=head_state,
            head_stale=head_stale if confirmed else [],
            view_stale=view_stale,
            view_unjudged=view_unjudged,
            view_judged=True,
            view_error=None,
        )
    except (OSError, subprocess.CalledProcessError, subprocess.TimeoutExpired) as exc:
        # No HEAD, or git/tar unavailable: nothing about drift was established in
        # either direction. Both halves degrade, neither blocks.
        return DiscoveryVerdicts(
            candidates=candidates,
            head_state="unknown",
            head_stale=[],
            view_stale=[],
            view_unjudged=list(candidates),
            view_judged=False,
            view_error=str(exc),
        )
    finally:
        if dest is not None:
            shutil.rmtree(dest, ignore_errors=True)


def _consume_view(verdicts: DiscoveryVerdicts) -> tuple[list[str], list[str], bool]:
    """The commit-view half of a verdict, reporting a build failure exactly once.

    The print lives here, not in discovery_index_verdicts, so that a caller that
    only wants the HEAD half (discovery_index_head_status, which has always been
    silent about a tree it could not build) does not start emitting a commit-view
    warning it never asked for.
    """
    if verdicts.view_error is not None:
        print(
            f"warning: could not build the commit view ({verdicts.view_error}); "
            "falling back to the working-tree verdict.",
            file=sys.stderr,
        )
    return verdicts.view_stale, verdicts.view_unjudged, verdicts.view_judged


def stale_in_commit_view(
    repo: Path,
    candidates: list[str],
    add_paths: tuple[str, ...],
    remove_paths: tuple[str, ...],
) -> tuple[list[str], list[str], bool]:
    """Check `candidates` against the tree this commit produces.

    Returns (still_stale, unjudged, judged). `judged` is False when the view could
    not be built at all, so the caller can say that rather than assert a direction
    of drift it has not established.
    """
    return _consume_view(
        discovery_index_verdicts(repo, add_paths, remove_paths, candidates)
    )


def _scratch_dir(repo: Path) -> Path:
    """Project `tmp/` (ai/rules/testing.md), created if missing."""
    scratch = repo / "tmp"
    scratch.mkdir(parents=True, exist_ok=True)
    return scratch


def discovery_index_problems(
    repo: Path,
    add_paths: tuple[str, ...],
    remove_paths: tuple[str, ...] = (),
    verdicts: DiscoveryVerdicts | None = None,
) -> list[str]:
    """Reasons this commit would leave a generated discovery index incoherent.

    `verdicts` lets a caller that ALSO needs the HEAD verdict (create) pay for one
    materialization of HEAD instead of two; omitted, this computes its own.

    The question is always the same: after this commit, does regenerating from the
    COMMITTED tree reproduce the committed index? Both directions matter, and the
    working tree answers neither reliably in a shared checkout:

      - working tree stale, commit coherent -- a concurrent session's uncommitted
        sources. Blocking here is a false positive, and the old remediation
        ("regenerate and include it") would cross-commit their rows.
      - working tree fresh, commit INCOHERENT -- the index on disk was regenerated
        WITH those uncommitted sources, so committing it publishes rows pointing at
        files absent from HEAD. This is how `plan/learned/1282-*.md` reached HEAD's
        committed index without ever being committed itself.
    """
    state, stale = discovery_index_freshness(repo)
    if state == "unknown":
        return []

    # Indexes this commit touches, either as a source or as the index itself.
    fed: set[str] = set()
    for p in (*add_paths, *remove_paths):
        header = (
            _read_head(repo / p, 40)
            if p.endswith(".go") and not p.endswith("_test.go")
            else ""
        )
        fed |= _indexes_fed_by(p, header)

    # Generator-free rule, kept because it is the only one a minimal checkout can
    # apply (T-6): a dirty index this commit FEEDS but omits must ride along, and
    # one it does not feed must never be demanded.
    missing = [
        out
        for out in DISCOVERY_INDEX_OUTPUTS
        if out in fed and out not in add_paths and index_pending(repo, out)
    ]
    if missing:
        return [
            "this commit changes sources that feed the discovery indexes but omits\n"
            "  the regenerated index(es): " + ", ".join(missing) + ".\n"
            "  Add them: " + " ".join("--file " + m for m in missing) + "."
        ]

    # Verify EVERY index the repo can judge, not just the ones `fed` names.
    # `indexes_fed_by` recognizes a PACKAGE-MAP source by a `// Package` header or
    # a register.go filename, but package_map.build keys its rows on DIRECTORY
    # existence (scripts/dev/package_map.py build()): a new `internal/x/thing.go`
    # carrying only `// Design:` adds a PACKAGE-MAP row while feeding only
    # DOCS-TO-CODE. Narrowing the check to `fed` therefore let that commit land
    # and leave HEAD incoherent. The view is already built; checking all three
    # costs ~3.6s against ~1.9s for one. `fed` still decides the `missing` message.
    if verdicts is None:
        verdicts = discovery_index_verdicts(repo, add_paths, remove_paths)
    if not verdicts.candidates:
        return []
    still, unjudged, judged = _consume_view(verdicts)
    if not judged:
        # The view could not be built, so nothing about drift was established.
        # Fall back to the working tree, which is the only verdict in evidence.
        if not stale:
            return []
        return [
            "discovery indexes are stale in the working tree and the commit view\n"
            "  could not be built to check them: " + ", ".join(stale) + ".\n"
            "  Run `make ze-regen` (or `make ze-discovery-index`) and include the\n"
            "  regenerated files in this commit."
        ]
    if not still:
        # Only an index that actually got a clean verdict may be called coherent.
        # One the generator could not judge (crash, timeout) is neither stale nor
        # coherent, and claiming "a concurrent session has uncommitted sources"
        # about it invents a cause the run never established.
        cleared = [out for out in stale if out not in unjudged]
        if cleared:
            print(
                "note: "
                + ", ".join(cleared)
                + " is stale in the WORKING TREE but coherent in the tree this\n"
                "  commit produces; a concurrent session has uncommitted sources."
                " Not blocking.",
                file=sys.stderr,
            )
        return []
    omitted = [out for out in still if out not in add_paths]
    included = [out for out in still if out in add_paths]
    lines = ["discovery indexes will not match the tree this commit produces:"]
    if omitted:
        lines.append(
            "  omitted: "
            + ", ".join(omitted)
            + "\n  Run `make ze-regen` (or `make ze-discovery-index`) and add them: "
            + " ".join("--file " + m for m in omitted)
        )
    if included:
        lines.append(
            "  included but wrong: "
            + ", ".join(included)
            + "\n  Regenerated from a working tree holding sources this commit does"
            "\n  NOT contain, so it carries rows for files that will be absent from"
            "\n  HEAD. Regenerate from HEAD plus this commit's own files"
            "\n  (see ai/rules/rule-format.md, the concurrent-session recipe)."
        )
    return ["\n".join(lines)]


def discovery_index_head_status(repo: Path) -> tuple[str, list[str]]:
    """Return (state, stale) for HEAD's COMMITTED indexes vs HEAD's committed
    sources.

    Unlike discovery_index_freshness (which checks the working tree), this
    materializes HEAD and re-runs the generators there, so it catches a commit
    that landed a feeding-source change without its index update even when the
    working tree carries unrelated uncommitted changes. "unknown" (no HEAD, or
    git/tar unavailable) surfaces nothing.

    This is the HEAD half of discovery_index_verdicts: the tree is extracted and
    judged BEFORE any commit overlay is applied. `create` gets both halves from a
    single call, so the repo is materialized once per commit script, not twice.
    """
    verdicts = discovery_index_verdicts(repo)
    return verdicts.head_state, verdicts.head_stale


# --------------------------------------------------------------------------- #
# Commit-time repo-state gates.
#
# Re-homed from pretool-bash.py, where four of them gated on the literal string
# "git commit" -- which the sanctioned `bash tmp/commit-<SID>.sh` path never
# contains, and which check_destructive_git blocks outright when it does. They
# were therefore doubly dead. Creation time is where the agent can still fix the
# commit, and the helper already knows exactly which files the commit will add
# and remove. Severities: spec-audit and deferral-in-diff BLOCK (raise
# UsageError); deferral-unassigned, wiring, and doc-drift WARN (print to stderr,
# commit proceeds -- an unhomed deferral row is harmless to software behaviour,
# so it is surfaced, not used to hard-block an otherwise-valid commit). See
# ai/rules/deferral-tracking.md, ai/rules/integration-completeness.md.
# --------------------------------------------------------------------------- #

DEFERRAL_PLACEHOLDERS = frozenset({"", "-", "unassigned", "tbd", "none"})

DEFERRAL_PATTERNS = (
    "deferred to",
    "deferred for",
    "defer to",
    "out of scope",
    "future work",
    "future spec",
    "handle later",
    "address later",
    "will be handled later",
    "will be done later",
    "will be addressed later",
    "skip for now",
    "skipping for now",
    "postpone",
    "not yet implemented",
    "not yet wired",
    "follow.up work",
)


# Directories whose markdown DISCUSSES deferral as policy rather than performing
# it: the rule corpus (ai/rules/deferral-tracking.md, no-parking.md, planning.md,
# handoff.md, ...) and the generated ai/rules/CONDENSED.md digest that flattens
# their prose. A bare "future work" / "out of scope" phrase there is the subject
# under discussion, not an act of parking work, so it is exempt from the
# added-prose scan. Specs (plan/spec-*.md) and code stay in scope -- that is where
# a real deferral gets written and must be homed. See ai/rules/deferral-tracking.md,
# ai/rules/friction-reporting.md, plan/learned/HOOK-FRICTION.md.
DEFERRAL_SCAN_EXEMPT_DIRS = ("ai/rules/", ".claude/rules/")


DEFERRAL_NO_DESTINATION_NEEDED = frozenset({"cancelled", "user-approved-drop"})

# A destination names a markdown file: a full `plan/...md` path (which may be
# nested, e.g. plan/learned/1127-x.md) or a bare `spec-x.md` resolved against
# plan/. The plan/ alternative comes first so the longest form wins; the
# lookbehind stops a match starting mid-path or mid-word.
DEFERRAL_DEST_PATH_RE = re.compile(r"(?<![\w/-])(plan/[\w./-]+\.md|[\w.-]+\.md)")

# Statuses that mean the row is CLOSED and needs no destination. Everything else
# is live and gets checked -- including a status word nobody has invented yet.
#
# This is a denylist of terminal states, not an allowlist of live ones, and the
# direction is the whole point (ai/rules/fail-closed-guards.md). The gate used to
# test `status == "open"`, so `deferred` -- the word ai/rules/deferral-tracking.md
# itself uses for this state -- was never looked at, and four rows written on
# 2026-07-16 named no home and sailed through. An allowlist re-runs that bug the
# next time the vocabulary drifts; a terminal denylist fails closed, because an
# unrecognised status is treated as live and checked.
#
# The deferral shards' contract (ai/rules/deferral-tracking.md): "A row lives here
# only while the work has no home. Once it is moved into a spec, it is resolved ...
# and the spec becomes the tracker."
DEFERRAL_TERMINAL_STATUSES = frozenset({"done", "cancelled", "resolved"})

# The table's own header and its |---|---| separator are not rows.
DEFERRAL_HEADER_CELLS = ("date", "source", "what", "reason", "destination", "status")
DEFERRAL_SEPARATOR_RE = re.compile(r"^:?-{2,}:?$")


def _deferral_row_cells(line: str) -> list[str] | None:
    """The six cells of a deferrals table row, or None when the line is not a row.

    Returns None for prose and for the table's header/separator. A line that IS a
    row but does not split into exactly six cells is malformed and NOT silently
    dropped: the caller reports it. Reading `dest`/`status` from fixed indices of
    a short row is how a Status-less row got status "" and passed as absent-and-fine.
    """
    if not line.lstrip().startswith("|"):
        return None
    fields = line.split("|")
    # A well-formed `| a | b | c | d | e | f |` splits into 8: leading and
    # trailing empties around the six cells.
    if len(fields) != 8 or fields[0].strip() or fields[-1].strip():
        return ["MALFORMED"]
    cells = [f.strip() for f in fields[1:7]]
    if tuple(c.lower() for c in cells) == DEFERRAL_HEADER_CELLS:
        return None
    if all(DEFERRAL_SEPARATOR_RE.match(c) for c in cells):
        return None
    return cells


def deferral_destination_paths(dest: str) -> list[str]:
    """The repo-relative plan/ files a Destination cell names, in order.

    The single source of truth for reading a Destination: the gate asks which
    files must exist, and the test harness asks which files to create. Two
    implementations of that question drift, and the drift is invisible (the
    harness creates one path while the gate checks another, so a test proves
    nothing about the gate).
    """
    return [
        p if p.startswith("plan/") else f"plan/{p}"
        for p in DEFERRAL_DEST_PATH_RE.findall(dest)
    ]


def deferral_destination_problem(repo: Path, dest: str) -> str | None:
    """Why this Destination cell fails the rule, or None when it is a valid home.

    ai/rules/deferral-tracking.md: a live deferral ALWAYS names a destination
    spec that exists on disk. Only a terminal Status may name no file. Prose
    ("later", "future work") is a deletion with a polite name, and a path to a
    spec nobody created loses the work just as completely -- both lose the work,
    so both are rejected.

    Both spellings of a destination are accepted, `plan/spec-x.md` and a bare
    `spec-x.md` resolved against plan/, because both name the same file and the
    rule is about the work having a home, not about path punctuation.

    ONE named file needs to exist, not all of them. A Destination cell is a
    filename plus prose, and good rows cite a retired spec in that prose to
    explain a re-homing ("the original destination `spec-old.md` was deleted at
    closure, orphaning this row"). Requiring every filename to resolve flagged
    two such rows that were correctly homed, which would have taught people to
    stop recording provenance to keep the gate quiet.
    """
    plain = dest.strip().strip("`").lower()
    if plain in DEFERRAL_NO_DESTINATION_NEEDED:
        return None
    if plain in DEFERRAL_PLACEHOLDERS:
        return "no destination"
    paths = deferral_destination_paths(dest)
    if not paths:
        return "destination names no file"
    if not any((repo / p).is_file() for p in paths):
        return "destination does not exist: " + ", ".join(paths)
    return None


def deferral_shard_paths(repo: Path) -> list[Path]:
    """Every deferral shard under plan/deferrals/, sorted for deterministic output.

    Deferrals are sharded one file per source (plan/deferrals/<spec-stem>.md, plus
    plan/deferrals/ad-hoc-<date>-<sid>.md), so that a session stages only files it
    owns and git merges disjoint creations without conflict
    (ai/rules/deferral-tracking.md, ai/rules/git-safety.md). The aggregate is a
    fold over this directory, computed on read and never stored.

    The glob is RECURSIVE (rglob) to stay aligned with deferral_in_diff_problems,
    whose clearing check accepts any path under plan/deferrals/ at any depth: a
    shard the clearing gate accepts must also be folded by this advisory check, or
    a nested shard could clear the block gate while escaping the unassigned fold.
    """
    deferrals_dir = repo / "plan" / "deferrals"
    if not deferrals_dir.is_dir():
        return []
    return sorted(deferrals_dir.rglob("*.md"))


def deferral_unassigned_problems(repo: Path) -> list[str]:
    """Live rows in plan/deferrals/*.md with no usable destination (WARN, advisory).

    The six-column table is Date | Source | What | Reason | Destination | Status.
    A row is checked unless its Status is terminal (DEFERRAL_TERMINAL_STATUSES);
    an unrecognised status is live, so the check still fails closed on a status
    vocabulary it does not know. Independent of what this commit touches -- it
    folds over every shard in plan/deferrals/ and surfaces every live row whose
    home is missing or is prose.

    Routed through commit_gate_warnings, NOT commit_gate_problems: an unhomed
    deferral row is harmless to software behaviour, so it is surfaced rather than
    used to block an otherwise-valid commit (ai/rules/deferral-tracking.md). The
    returned strings are printed as warnings; homing is still required by the
    rule, but a missing home no longer blocks the commit.
    """
    shards = deferral_shard_paths(repo)
    if not shards:
        return []
    unassigned: list[str] = []
    malformed: list[str] = []
    for path in shards:
        rel = path.relative_to(repo).as_posix()
        for lineno, line in enumerate(
            path.read_text(encoding="utf-8", errors="replace").splitlines(), 1
        ):
            cells = _deferral_row_cells(line)
            if cells is None:
                continue
            if cells == ["MALFORMED"]:
                malformed.append(f"  - {rel}:{lineno}: {line.strip()[:90]}")
                continue
            _date, _source, what, _reason, dest, status = cells
            if status.lower() in DEFERRAL_TERMINAL_STATUSES:
                continue
            problem = deferral_destination_problem(repo, dest)
            if problem is not None:
                unassigned.append(f'  - {what} ({problem}: "{dest}")')
    problems: list[str] = []
    if unassigned:
        problems.append(
            "live deferrals without a destination spec (advisory, does not block):\n"
            + "\n".join(unassigned)
            + "\n  Homing is still required by ai/rules/deferral-tracking.md -- each"
            "\n  should name an existing plan/spec-*.md (add the work to a spec that"
            "\n  covers the topic, or create plan/spec-<source>-deferred-<subtask>.md"
            "\n  from plan/TEMPLATE.md), or be cancelled -- but an unhomed row no"
            "\n  longer holds the commit back."
        )
    if malformed:
        problems.append(
            "malformed rows in plan/deferrals/ (cannot be checked, so not\n"
            "  assumed innocent -- a row this gate cannot read is a row it cannot"
            " enforce):\n" + "\n".join(malformed) + "\n  Each row needs exactly six"
            " cells: | Date | Source | What | Reason | Destination | Status |."
        )
    return problems


def _prospective_added_lines(
    repo: Path, add_paths: tuple[str, ...], remove_paths: tuple[str, ...]
) -> list[tuple[str, str]]:
    """The (path, '+' line) pairs this commit would introduce (added/modified files).

    Computed in a THROWAWAY index (GIT_INDEX_FILE) so the real staging area is
    never touched: read HEAD's tree, apply this commit's add/remove set, then
    diff --cached -U0 --diff-filter=AM. That is exactly what the generated
    `git add ... ; git commit` will record, and it captures brand-new files too.

    Each returned line is paired with the repo-relative path of the file it belongs
    to (from the diff's `+++ b/<path>` header) so a caller can path-scope the scan.
    """
    (repo / "tmp").mkdir(exist_ok=True)
    fd, index = tempfile.mkstemp(prefix="ze-commit-index-", dir=str(repo / "tmp"))
    os.close(fd)
    os.unlink(index)  # git wants to create it; a pre-existing empty file is fine too
    env = dict(os.environ, GIT_INDEX_FILE=index)

    def git(*args: str) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            ("git", "-C", str(repo), *args),
            check=False,
            text=True,
            capture_output=True,
            env=env,
        )

    try:
        has_head = git("rev-parse", "--verify", "-q", "HEAD").returncode == 0
        if has_head:
            git("read-tree", "HEAD")
        if add_paths:
            git("add", "--", *add_paths)
        for path in remove_paths:
            git("rm", "--cached", "-q", "--", path)
        diff = git("diff", "--cached", "--no-color", "-U0", "--diff-filter=AM").stdout
    finally:
        try:
            os.unlink(index)
        except OSError:
            pass
    result: list[tuple[str, str]] = []
    current = ""
    for line in diff.splitlines():
        if line.startswith("+++ "):
            hdr = line[4:]  # "b/<path>" for A/M files, "/dev/null" never here
            current = hdr[2:] if hdr.startswith("b/") else hdr
            continue
        if line.startswith("+") and not line.startswith("+++") and line != "+":
            result.append((current, line))
    return result


def _deferral_prose(line: str) -> str:
    """The prose part of a '+' diff line, with quoted and backticked spans blanked.

    A phrase that exists only as a quoted or backticked token -- a
    `DEFERRAL_PATTERNS` entry, a test fixture string, a doc example -- is data,
    not a statement of deferring work, so it is removed before matching. A phrase
    in bare markdown text or a bare code comment survives and is caught.
    """
    text = line[1:] if line.startswith("+") else line
    text = re.sub(r'"[^"]*"', " ", text)
    text = re.sub(r"'[^']*'", " ", text)
    text = re.sub(r"`[^`]*`", " ", text)
    return text


def deferral_in_diff_problems(
    repo: Path, add_paths: tuple[str, ...], remove_paths: tuple[str, ...]
) -> list[str]:
    """Deferral language in the commit's added PROSE with no plan/deferrals/ entry.

    Matching runs on `_deferral_prose(line)`, not the raw line, so the canonical
    `DEFERRAL_PATTERNS` list, the rule docs that quote it, and the fixtures that
    test it -- all of which now ride the gated commit path -- do not self-trip:
    their trigger phrases live inside quotes or backticks. Go's `defer f()` is
    also skipped. Lines from `DEFERRAL_SCAN_EXEMPT_DIRS` (the rule corpus and its
    generated digest, which discuss deferral policy in bare prose) are skipped by
    path. This prose-only, path-scoped match is the intentional divergence from
    the dead `check-deferral-in-diff.sh`, which never met its own list.

    The gate is satisfied when the commit stages any deferral shard under
    plan/deferrals/ (deferrals are sharded per source, so recording a deferral
    creates or edits a shard, not the retired single plan/deferrals.md file).
    """
    if any(p.startswith("plan/deferrals/") for p in add_paths):
        return []
    prose = [
        (line, _deferral_prose(line))
        for path, line in _prospective_added_lines(repo, add_paths, remove_paths)
        if not path.startswith(DEFERRAL_SCAN_EXEMPT_DIRS)
        and not re.match(r"^\+\s*defer [a-zA-Z]", line)
    ]
    if not prose:
        return []
    hits: list[str] = []
    for pattern in DEFERRAL_PATTERNS:
        matches = [
            orig for orig, text in prose if re.search(pattern, text, re.IGNORECASE)
        ]
        if matches:
            hits.append(f"  pattern '{pattern}':")
            hits.extend("    " + m[1:] for m in matches[:3])
    if not hits:
        return []
    return [
        "deferral language in staged changes without a plan/deferrals/ entry:\n"
        + "\n".join(hits)
        + "\n  Record each deferral in its plan/deferrals/<source>.md shard before"
        " committing (ai/rules/deferral-tracking.md)."
    ]


def wiring_warnings(add_paths: tuple[str, ...]) -> list[str]:
    """Plugin .go committed without any .ci functional test (advisory warn)."""
    plugin_go = [
        f
        for f in add_paths
        if re.search(r"^internal/plugins/.*\.go$", f)
        and not f.endswith("_test.go")
        and not f.endswith("register.go")
        and "/schema/" not in f
        and not f.endswith("doc.go")
    ]
    if not plugin_go or any(f.endswith(".ci") for f in add_paths):
        return []
    return [
        "plugin code committed without a .ci functional test:\n"
        + "\n".join(f"    {f}" for f in plugin_go)
        + "\n  Is this reachable by a user via config/CLI/API?"
        " (ai/rules/integration-completeness.md)."
    ]


def feature_gate_tags(repo: Path) -> list[str]:
    """Sorted ze_<feature> build tags from feature-gates.txt, the single source of
    truth (ai/rules/feature-gate-registration.md). Derived, not hardcoded, so a new
    gate is picked up automatically -- mirrors ZE_FEATURES in the Makefile,
    featureGateTags() in internal/test/runner, and _feature_gate_tags() in
    stress-repro.py."""
    manifest = repo / "feature-gates.txt"
    if not manifest.exists():
        # Degrading to a bare `ze_core` build silently reproduces the exact bug
        # this function exists to fix (11 fabricated drift warnings on a green
        # tree), so say so rather than guess.
        print(
            "warning: feature-gates.txt not found; doc-drift ran without feature tags "
            "and its output is not trustworthy",
            file=sys.stderr,
        )
        return []
    tags = set()
    for line in manifest.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        tags.add(line.split()[0])
    return sorted(tags)


def doc_drift_warnings(repo: Path) -> list[str]:
    """Advisory warning when docs drift from the live registry.

    Runs under the SAME build tags as `make ze-doc-drift` (Makefile GO_RUN =
    `go run -tags '$(GO_TEST_TAGS)'`, GO_TEST_TAGS = `ze_core $(ZE_FEATURES)`).
    A bare `go run` here compiles out every feature-gated package, so the family
    registry holds only the four always-on families and EVERY address-family
    claim in docs/comparison.md and docs/DESIGN.md is reported as drift -- 11
    fabricated warnings on a tree whose `make ze-doc-test` is green. That is the
    trap ai/rules/bash-output.md names: dropping the feature tags fakes reds.
    """
    if not (repo / "scripts" / "docvalid" / "doc_drift.go").exists():
        return []
    tags = " ".join(["ze_core", *feature_gate_tags(repo)])
    try:
        res = subprocess.run(
            ("go", "run", "-tags", tags, "scripts/docvalid/doc_drift.go"),
            cwd=str(repo),
            capture_output=True,
            text=True,
            timeout=120,
        )
    except Exception:
        return []  # compile error / timeout / no toolchain -- never block
    if res.returncode == 1:
        return [(res.stdout + res.stderr).rstrip()]
    return []


# Kept in sync with EXIT_HABIT_GREW in scripts/dev/ste_check.py. Distinct from
# argparse's usage exit (2), so a malformed invocation cannot read as a finding.
STE_HABIT_GREW = 3


def ste_problems(repo: Path, add_paths: tuple[str, ...]) -> list[str]:
    """BLOCK a commit whose own prose grew an ASD-STE100 habit.

    Rule one of the repository is Simplified Technical English
    (ai/rules/simplified-technical-english.md). This is the only place where the
    six banned habits can be attributed to ONE author: several sessions share
    this checkout, so a tree-wide prose gate reports a colleague's in-flight
    sentences, and a gate that reddens for someone else's typing gets switched
    off. The files of one commit are the right unit.

    Each file is compared against its own HEAD version, so legacy prose in a file
    you touched costs nothing. Only the sentences you added count.
    """
    checker = repo / "scripts" / "dev" / "ste_check.py"
    if not checker.exists():
        return []
    reviewable = [p for p in add_paths if p.endswith((".md", ".go", ".yang"))]
    if not reviewable:
        return []
    try:
        res = subprocess.run(
            (sys.executable, "scripts/dev/ste_check.py", "--check", *reviewable),
            cwd=str(repo),
            capture_output=True,
            text=True,
            timeout=120,
        )
    except Exception as exc:  # noqa: BLE001 - a broken checker must not wedge commits
        # Fail OPEN, but never in silence. A crash, a timeout, or a missing
        # interpreter removes this guard from every commit, and a gate that can
        # evaporate without a word is worse than no gate. Same shape as
        # discovery_index_verdicts.
        print(f"warning: ste gate could not run ({exc}); prose is UNCHECKED", file=sys.stderr)
        return []
    if res.returncode == STE_HABIT_GREW:
        return [(res.stdout + res.stderr).rstrip()]
    if res.returncode != 0:
        print(
            f"warning: ste gate could not judge (exit {res.returncode}); "
            f"prose is UNCHECKED: {(res.stdout + res.stderr).strip()[:400]}",
            file=sys.stderr,
        )
    return []


def claimed_spec(repo: Path) -> str:
    """This session's claimed spec basename via spec-session.sh, or '' if none."""
    script = repo / "scripts" / "dev" / "spec-session.sh"
    if not script.exists():
        return ""
    try:
        res = subprocess.run(
            (str(script), "current"),
            cwd=str(repo),
            capture_output=True,
            text=True,
            timeout=10,
        )
    except Exception:
        return ""
    return res.stdout.strip() if res.returncode == 0 else ""


def _table_data_rows(lines: list[str]) -> int:
    """Table DATA rows in `lines`.

    Each table ships as header + separator only, so data rows =
    pipe-rows - 2*(separator rows): every table contributes exactly one header
    and one separator.
    """
    pipe_rows = sum(1 for line in lines if line.strip().startswith("|"))
    sep_rows = sum(1 for line in lines if re.match(r"^\|[\s:|-]+\|?\s*$", line.strip()))
    return pipe_rows - 2 * sep_rows


def pre_commit_verification_gaps(spec_text: str) -> list[str] | None:
    """Evidence sub-tables of '## Pre-Commit Verification' left without a row.

    Returns None when the section is absent, [] when every evidence table
    carries at least one data row, else the names of the tables that do not.

    Checked PER SUB-TABLE, not per section. Each `###` table is a separate
    obligation (files exist / AC re-verified / wiring re-read / assumptions
    resolved / docs verified), so a row in one is not evidence for another.
    The old section-level rule accepted a single row anywhere, which is why
    ~73% of `AC Verified` and ~75% of `Wiring Verified` tables reached closure
    byte-identical to the template.

    A section with no `###` sub-headings (the pre-2026-07 spec shape) keeps the
    old floor -- one data row anywhere -- so widening the gate cannot
    retroactively block a spec whose section never had sub-tables.
    """
    section: list[str] = []
    in_section = False
    for line in spec_text.splitlines():
        if line.startswith("## Pre-Commit Verification"):
            in_section = True
            continue
        if in_section and line.startswith("## "):
            break
        if in_section:
            section.append(line)
    if not in_section:
        return None

    subs: list[tuple[str, list[str]]] = []
    for line in section:
        if line.startswith("### "):
            # Strip the parenthetical hint: "### Files Exist (ls)" -> "Files Exist".
            subs.append((line[4:].split("(")[0].strip(), []))
        elif subs:
            subs[-1][1].append(line)

    if not subs:
        return [] if _table_data_rows(section) > 0 else ["Pre-Commit Verification"]
    return [name for name, body in subs if _table_data_rows(body) <= 0]


def spec_audit_problems(
    repo: Path, add_paths: tuple[str, ...], claimed: str
) -> list[str]:
    """Block a spec-closure commit whose spec has an unfilled verification.

    Ported from the never-wired pre-commit-spec-audit.sh, keyed to the LIVE
    per-session marker (its old tmp/session/selected-spec substrate was removed
    in 276d72c99). Fires ONLY when this commit adds the claimed spec's own
    learned summary -- i.e. the closure commit of the claiming session -- so it
    never blocks unrelated commits or other sessions (the historic umbrella-spec
    false-positive mode). No spec claimed -> no gate.
    """
    if not claimed:
        return []
    stem = re.sub(r"\.md$", "", re.sub(r"^spec-", "", claimed))
    pattern = re.compile(rf"^plan/learned/[0-9]+-{re.escape(stem)}\.md$")
    if not any(pattern.match(p) for p in add_paths):
        return []  # not this spec's closure commit
    spec_path = repo / "plan" / claimed
    if not spec_path.is_file():
        return []
    gaps = pre_commit_verification_gaps(
        spec_path.read_text(encoding="utf-8", errors="replace")
    )
    if gaps is None:
        return [
            f"spec {claimed} has no '## Pre-Commit Verification' section, but this"
            " commit adds its learned summary (closure).\n"
            "  Add and fill it from plan/TEMPLATE-CLOSURE.md before closing"
            " (ai/rules/planning.md)."
        ]
    if gaps:
        return [
            f"spec {claimed} '## Pre-Commit Verification' has no evidence rows in:"
            f" {', '.join(gaps)}.\n"
            "  This commit adds its learned summary (closure). Each table is a"
            " separate obligation: re-verify independently and paste the evidence"
            " for EVERY one (plan/TEMPLATE-CLOSURE.md, ai/rules/planning.md)."
        ]
    return []


def commit_gate_problems(
    repo: Path, add_paths: tuple[str, ...], remove_paths: tuple[str, ...]
) -> list[str]:
    """All BLOCK-severity commit-time gates, in one call for create()."""
    problems: list[str] = []
    problems += deferral_in_diff_problems(repo, add_paths, remove_paths)
    problems += spec_audit_problems(repo, add_paths, claimed_spec(repo))
    problems += ste_problems(repo, add_paths)
    return problems


def commit_gate_warnings(repo: Path, add_paths: tuple[str, ...]) -> list[str]:
    """All WARN-severity commit-time gates, in one call for create().

    deferral_unassigned is advisory, not blocking. An unhomed live deferral row
    is a bookkeeping state that is harmless to software behaviour: the worst case
    is a row committed too early or in the wrong commit. Blocking EVERY commit on
    it -- including commits that never touched deferrals, and rows another session
    wrote into the shared working tree -- held real work back for no software
    reason. It is still surfaced loudly (printed to stderr) so a genuine unhomed
    deferral stays visible; only the hard block is removed. Homing is still
    required by ai/rules/deferral-tracking.md; this changes the gate's severity,
    not the rule.
    """
    return (
        deferral_unassigned_problems(repo)
        + wiring_warnings(add_paths)
        + doc_drift_warnings(repo)
    )


_LEARNED_STEM_RE = re.compile(r"^plan/learned/[0-9]{3,}-(?P<stem>.+)\.md$")
_SPEC_STEM_RE = re.compile(r"^plan/spec-(?P<stem>.+)\.md$")
# Kept in sync with review_gate.py CODE_SUFFIXES / CODE_BASENAMES: prose (.md) and
# the spec/learned records go to other gates; everything logic-bearing (including
# hand-written build/template files) is reviewable.
_REVIEW_CODE_SUFFIXES = (
    ".go",
    ".ci",
    ".et",
    ".py",
    ".sh",
    ".yang",
    ".wb",
    ".mk",
    ".tmpl",
    ".html",
    ".c",
    ".rs",
    ".s",
    ".rego",
    ".tac",
)
_REVIEW_CODE_BASENAMES = ("Makefile",)


def _is_review_code(path: str) -> bool:
    return (
        path.endswith(_REVIEW_CODE_SUFFIXES)
        or Path(path).name in _REVIEW_CODE_BASENAMES
    )


def spec_closure_stem(
    add_paths: tuple[str, ...], remove_paths: tuple[str, ...]
) -> str | None:
    """The spec-stem this commit closes, or None if it is not a closure commit.

    Closure = commit A adds plan/learned/NNN-<stem>.md, or commit B removes
    plan/spec-<stem>.md (ai/rules/planning.md "Spec Closure"). The <stem> is the
    key the review artifact (tmp/review/<stem>-<session-id>.md) is written under.
    """
    for p in add_paths:
        m = _LEARNED_STEM_RE.match(p)
        if m:
            return m.group("stem")
    for p in remove_paths:
        m = _SPEC_STEM_RE.match(p)
        if m:
            return m.group("stem")
    return None


def review_gate_problems(
    repo: Path, add_paths: tuple[str, ...], remove_paths: tuple[str, ...]
) -> list[str]:
    """BLOCK a spec-closure commit whose code is not covered by a fresh, CLEAN,
    INDEPENDENT review (ai/rules/critical-review.md).

    Runs scripts/dev/review_gate.py check over the code/test files in this commit.
    The artifact is written by INDEPENDENT reviewers (subagents / a fresh
    session), never the author's own inline reasoning, and pins each reviewed
    file by hash so a post-review edit re-opens the gate.

    Coverage note: this checks the code in THIS commit. The ze-implement closure
    commits all of a spec's code in one commit A, so that is full coverage. A
    workflow that commits code in earlier (feature) commits and then closes with a
    code-free learned-summary commit is NOT fully covered here -- but a code-free
    closure still requires a CLEAN artifact to EXIST (below), so a spec cannot
    close with no review on record at all. Returns [] only when this is not a
    closure commit or when review_gate.py is absent.
    """
    stem = spec_closure_stem(add_paths, remove_paths)
    if stem is None:
        return []
    gate = repo / "scripts" / "dev" / "review_gate.py"
    if not gate.exists():
        return []
    # code_files may be empty (a code-free closure); review_gate.py check then
    # only requires a CLEAN artifact to exist, closing the "commit code first,
    # then a bare closure" hole into "at least record a review".
    code_files = [p for p in add_paths if _is_review_code(p)]
    proc = subprocess.run(
        [sys.executable, str(gate), "check", "--spec", stem, "--files", *code_files],
        cwd=repo,
        capture_output=True,
        text=True,
    )
    if proc.returncode == 0:
        return []
    detail = (proc.stderr or proc.stdout or "review gate failed").strip()
    return [
        detail + "\n"
        "  A spec closes only after an INDEPENDENT critical review of its code\n"
        "  (ai/rules/critical-review.md): spawn reviewer subagents over the diff,\n"
        "  fix findings, loop to zero, and record with:\n"
        "    python3 scripts/dev/review_gate.py record --spec "
        + stem
        + " --verdict clean --files <code files>\n"
        '  Owner override: --review-override "<reason>".'
    ]


def create(args: argparse.Namespace) -> int:
    repo = repo_root(args.repo)
    session = session_id(repo, args.session)
    tag = normalize_tag(args.tag, repo, session)
    add_paths = unique_paths([rel_path(repo, raw) for raw in args.file])
    remove_paths = unique_paths([rel_path(repo, raw) for raw in args.remove])
    if not add_paths and not remove_paths:
        raise UsageError("at least one --file or --remove path is required")
    for path in add_paths:
        validate_add_path(repo, path)
    for path in remove_paths:
        validate_remove_path(repo, path)
    # Verify gate: a commit script must not be prepared over a non-green verify
    # unless the caller explicitly acknowledges why (owner override, or a
    # known-red logged in plan/known-failures/). This turns "verify before
    # commit" from honor-system into an enforced, overridable gate. See
    # ai/rules/git-safety.md.
    vstate, detail = verify_status(repo)
    if vstate == "stale":
        # A DETERMINISTIC STRUCTURAL GATE red (tier/lint/vet/plugin-boundary/
        # iface-resolution/regen-check-readonly/wiring-docs) is never flaky or
        # environmental: it means the tree is structurally broken. Such a red is
        # NOT bypassable by --unverified or a plan/known-failures/ known-red
        # (those cover flaky TEST stages only). This closes the hole that let a
        # misplaced-tier gate (routeinstall) be parked as "pre-existing" and
        # shipped red on main. See ai/rules/git-safety.md.
        gate_reds = structural_gate_reds(repo)
        # ONE escape, owner-only, and deliberately NOT --unverified: keeping it a
        # separate flag means the flaky-test path can never reach this branch by
        # accident, and the reason has to be written down. Without any escape a
        # green tree was the only route to any commit at all -- including one that
        # touches no compiled code and cannot affect the red -- so the refusal was
        # pushing sessions toward the real hole: widening --unverified or editing
        # STRUCTURAL_GATES. The override is LOUD on purpose: a silent bypass would
        # make a red tree indistinguishable from a green one in the transcript.
        if gate_reds and (args.structural_red_ok or "").strip():
            print(
                "WARNING: committing over a RED structural gate: "
                + ", ".join(gate_reds)
                + "\n  Owner override: "
                + args.structural_red_ok.strip()
                + "\n  The tree is structurally red. Fix it, or confirm the red is"
                " another session's and cannot be affected by this commit.",
                file=sys.stderr,
            )
            gate_reds = []
        if gate_reds:
            raise UsageError(
                "ze-verify has a DETERMINISTIC STRUCTURAL GATE red that "
                "--unverified cannot bypass: " + ", ".join(gate_reds) + ".\n"
                "  Structural gates (tier/lint/vet/plugin-boundary/iface-resolution/\n"
                "  regen-check-readonly/wiring-docs) never fail for flaky or environmental\n"
                "  reasons -- a red means the tree is structurally broken. They are\n"
                "  NOT eligible for --unverified or a plan/known-failures/ known-red.\n"
                "  Fix it at the source (run `make " + gate_reds[0] + "` to see the\n"
                "  failure). To CLEAR this refusal you must refresh the verify record:\n"
                "  only `make ze-verify` (or `make ze-verify-changed`) rewrites\n"
                "  tmp/ze-verify-failures.json -- `make "
                + gate_reds[0]
                + "` alone does\n"
                "  NOT. Re-run a full verify until green and this clears."
            )
        # --structural-red-ok acknowledges a strictly WORSE condition than
        # --unverified (a red structural gate, not a flaky test), so it satisfies
        # this check too rather than demanding both flags for one decision.
        if not args.unverified and not (args.structural_red_ok or "").strip():
            raise UsageError(
                "ze-verify is not FRESH-green (" + (detail or "unknown") + ").\n"
                "  Run `make ze-verify` (or `make ze-verify-changed`) until green, then\n"
                '  commit, OR pass --unverified "<reason>" to commit anyway (owner\n'
                "  override, or a flaky/environmental known-red logged in\n"
                "  plan/known-failures/; structural gates are never eligible)."
            )
    # Discovery-index gate: the generated maps (ai/PACKAGE-MAP.md,
    # ai/DOCS-TO-CODE.md, ai/LEARNED-FULL-INDEX.md) must match the committed
    # sources. With no CI, this is the only place the freshness is enforced.
    #
    # ONE materialization of HEAD answers both index questions asked in this
    # function: this gate (does the tree this commit PRODUCES still regenerate to
    # the committed indexes?) and the HEAD warning printed further down (did a
    # prior commit already break them?). They used to extract the repo twice.
    verdicts: DiscoveryVerdicts | None = None
    if not args.stale_index_ok:
        verdicts = discovery_index_verdicts(repo, add_paths, remove_paths)
        problems = discovery_index_problems(repo, add_paths, remove_paths, verdicts)
        if problems:
            raise UsageError(
                "\n".join(problems)
                + '\n  ... or pass --stale-index-ok "<reason>" to commit anyway.'
            )
    # Commit-time repo-state BLOCK gates (deferral log + spec closure audit).
    gate_problems = commit_gate_problems(repo, add_paths, remove_paths)
    if gate_problems:
        raise UsageError("\n\n".join(gate_problems))
    # Critical-review gate: a spec cannot close without an INDEPENDENT review of
    # its code that is fresh (hash-pinned) and clean. This makes review the
    # central, unskippable step -- it cannot be satisfied by narrating "0 issues"
    # into the spec. See ai/rules/critical-review.md.
    if not args.review_override:
        review_problems = review_gate_problems(repo, add_paths, remove_paths)
        if review_problems:
            raise UsageError("\n\n".join(review_problems))
    # For a (non-overridden) spec-closure commit, re-run the review gate inside the
    # generated script so an edit between `create` and `bash tmp/commit-*.sh` is
    # caught at commit-RUN time (TOCTOU), not only here at generation time.
    review_check = ""
    closure_stem = spec_closure_stem(add_paths, remove_paths)
    if closure_stem is not None and not args.review_override:
        rc_code = tuple(p for p in add_paths if _is_review_code(p))
        review_check = (
            "python3 scripts/dev/review_gate.py check --spec "
            + shlex.quote(closure_stem)
            + " --files"
            + ("" if not rc_code else " " + quote_paths(rc_code))
        )
    msg = message_text(args.subject, args.body)
    msg_path = f"tmp/commit-msg-{session}-{tag}.txt"
    comment = lesson_comment(
        add_paths,
        remove_paths,
        args.lesson_required,
        args.lesson_not_needed,
    )
    block = CommitBlock(
        tag,
        args.subject.strip(),
        add_paths,
        remove_paths,
        msg_path,
        comment,
        review_check,
    )
    script = write_outputs(
        repo, session, block, msg, args.append, args.replace, args.dry_run
    )
    if not args.dry_run:
        print(f"session={session}")
        print(f"message={msg_path}")
        print(f"script={script.relative_to(repo).as_posix()}")
        print(f"lesson={comment}")
        if args.unverified:
            print(f"verify=UNVERIFIED ({args.unverified})")
        else:
            print(f"verify={vstate.upper()} ({detail})")
        if args.review_override and spec_closure_stem(add_paths, remove_paths):
            print(f"review=OVERRIDDEN ({args.review_override})")
    if verdicts is None:
        # --stale-index-ok skipped the gate above, so nothing has been materialized
        # yet. The HEAD warning is independent of that override and still applies.
        verdicts = discovery_index_verdicts(repo)
    head_state, head_stale = verdicts.head_state, verdicts.head_stale
    if head_state == "stale":
        print(
            "warning: HEAD's committed discovery index does not match HEAD's "
            "committed sources: " + ", ".join(head_stale) + ".\n"
            "  A prior commit bypassed the freshness gate, or an index was "
            "committed that references not-yet-committed work. Run `make ze-regen`\n"
            "  once the tree is coherent and commit the fix.",
            file=sys.stderr,
        )
    reminder = closure_reminder(add_paths, remove_paths)
    if reminder:
        print(reminder, file=sys.stderr)
    # Commit-time WARN gates (plugin .go without .ci, doc drift). Advisory:
    # printed to stderr, the commit still proceeds.
    for warning in commit_gate_warnings(repo, add_paths):
        print("warning: " + warning, file=sys.stderr)
    return 0


def print_session(args: argparse.Namespace) -> int:
    repo = repo_root(args.repo)
    print(session_id(repo, args.session))
    return 0


SLUG_RE = re.compile(r"^[a-z0-9][a-z0-9-]*$")


LEARNED_NEXT_RETRIES = 8


def learned_next(args: argparse.Namespace) -> int:
    """Allocate the next learned-summary number collision-free.

    The plan/learned/NNNN-slug.md filenames ARE the record, so the next number
    is max(existing prefixes) + 1 -- no separate .counter cache. The file is
    created with an exclusive O_EXCL open, so any later session in the same tree
    sees it and allocates past it. If a concurrent session wins the same number
    in the window between the glob and the create, the O_EXCL fails; we re-glob
    (the winner's file now raises the floor) and retry a bounded number of
    times, always landing on the next free number rather than crashing.

    Duplicate numbers across BRANCHES cannot be prevented here (two trees, no
    shared filesystem); scripts/dev/learned_numbers.py --check is the backstop.
    """
    repo = repo_root(args.repo)
    slug = args.slug
    if not SLUG_RE.match(slug):
        raise UsageError(f"slug {slug!r} must be lower-kebab-case")
    learned_dir = repo / "plan" / "learned"
    for _ in range(LEARNED_NEXT_RETRIES):
        highest = 0
        for entry in learned_dir.glob("[0-9]*-*.md"):
            prefix = entry.name.split("-", 1)[0]
            if prefix.isdigit():
                highest = max(highest, int(prefix))
        number = highest + 1
        path = learned_dir / f"{number:03d}-{slug}.md"
        try:
            fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o644)
        except FileExistsError:
            continue  # a concurrent session won this number; re-glob and retry
        with os.fdopen(fd, "w") as fh:
            fh.write(f"# {slug}\n\n<!-- write the learned summary here -->\n")
        print(path.relative_to(repo).as_posix())
        return 0
    raise UsageError(
        f"could not allocate a learned number after {LEARNED_NEXT_RETRIES} "
        f"attempts; concurrent allocation contention for slug {slug!r}"
    )


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Generate Ze commit message files and user-run commit scripts."
    )
    parser.add_argument(
        "--repo",
        help="repository root or subdirectory, defaults to the current directory",
    )
    sub = parser.add_subparsers(dest="cmd", required=True)

    session = sub.add_parser(
        "session", help="print and persist the reusable 8-char commit session id"
    )
    session.add_argument(
        "--session", help="set the reusable session id, mainly for tests"
    )
    session.set_defaults(func=print_session)

    learned_cmd = sub.add_parser(
        "learned-next",
        help="allocate the next plan/learned number and create the file",
    )
    learned_cmd.add_argument(
        "slug", help="lower-kebab-case summary name (no NNN- prefix)"
    )
    learned_cmd.set_defaults(func=learned_next)

    create_cmd = sub.add_parser(
        "create",
        help="write one commit message file and one block in tmp/commit-SESSION.sh",
    )
    create_cmd.add_argument(
        "--session", help="set the reusable session id, mainly for tests"
    )
    create_cmd.add_argument(
        "--tag", help="message tag, defaults to the next available letter"
    )
    create_cmd.add_argument(
        "--subject", required=True, help="single-line commit subject, max 72 chars"
    )
    create_cmd.add_argument(
        "--body",
        action="append",
        default=[],
        help="commit body paragraph or lines, wrapped to 72 chars",
    )
    create_cmd.add_argument(
        "--file",
        action="append",
        default=[],
        help="explicit file to add, repeat for each file",
    )
    create_cmd.add_argument(
        "--remove",
        action="append",
        default=[],
        help="tracked file to remove with git rm, repeat for each file",
    )
    create_cmd.add_argument(
        "--append",
        action="store_true",
        help="append this commit block to an existing script",
    )
    create_cmd.add_argument(
        "--replace",
        action="store_true",
        help="replace any existing script for this session",
    )
    create_cmd.add_argument(
        "--lesson-required",
        action="store_true",
        help="require a new plan/learned/NNN-name.md in --file",
    )
    create_cmd.add_argument(
        "--lesson-not-needed",
        help="explicit reason no learned summary is useful for this commit",
    )
    create_cmd.add_argument(
        "--unverified",
        help="reason to allow a commit when ze-verify is not FRESH-green "
        "(owner override, or a flaky/environmental known-red logged in "
        "plan/known-failures/; deterministic structural gates are never eligible)",
    )
    create_cmd.add_argument(
        "--structural-red-ok",
        help="OWNER OVERRIDE ONLY: reason to allow a commit while a deterministic "
        "structural gate is red in the verify record. Deliberately separate from "
        "--unverified so the flaky-test path can never reach it. Use when the red "
        "belongs to another session's in-flight work and this commit cannot affect "
        "it; the reason is echoed with the red gate names",
    )
    create_cmd.add_argument(
        "--stale-index-ok",
        help="reason to allow a commit when a generated discovery index "
        "(ai/PACKAGE-MAP.md, ai/DOCS-TO-CODE.md, ai/LEARNED-FULL-INDEX.md) is "
        "stale or omitted (owner override)",
    )
    create_cmd.add_argument(
        "--review-override",
        help="reason to allow a spec-closure commit when the independent "
        "critical-review gate (ai/rules/critical-review.md) is missing/stale "
        "(owner override; a review not performed is never a clean tree)",
    )
    create_cmd.add_argument(
        "--dry-run",
        action="store_true",
        help="print generated files without writing them",
    )
    create_cmd.set_defaults(func=create)
    return parser


def main(argv: list[str]) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    try:
        return args.func(args)
    except UsageError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
