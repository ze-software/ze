#!/usr/bin/env python3
"""Generate safe user-run commit scripts for Ze."""

from __future__ import annotations

import argparse
import base64
import json
import os
import re
import secrets
import shlex
import stat
import subprocess
import sys
import tempfile
import textwrap
from dataclasses import dataclass
from pathlib import Path

# Discovery-index generators/outputs and the source-trigger predicate are shared
# with the changed-file router (verify_wiring_docs.py) via discovery_sources.py,
# so the commit gate here and the router cannot drift apart.
from discovery_sources import GENERATORS as DISCOVERY_INDEX_GENERATORS
from discovery_sources import OUTPUTS as DISCOVERY_INDEX_OUTPUTS
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


def _ps_field(field: str, pid: int) -> str:
    try:
        return subprocess.run(
            ["ps", "-o", field, "-p", str(pid)],
            capture_output=True,
            text=True,
            check=False,
        ).stdout.strip()
    except Exception:
        return ""


def claude_session_fingerprint() -> str:
    """Identify the Claude session that owns this process.

    Concurrent Claude sessions share tmp/; when they also shared one
    tmp/commit-session-id they shared one tmp/commit-<SESSION>.sh, and a
    --replace from one session silently overwrote the other session's
    prepared script (observed 2026-06-10). Keying the session file by the
    owning Claude session gives every session its own script path. Uses
    the session_id claim of the access token when present, else the PID
    of the `claude` ancestor process (stable across Bash tool calls of
    one session), else the parent PID.
    """
    tok = os.environ.get("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "")
    if tok.count(".") >= 2:
        try:
            payload = tok.split(".")[1].replace("_", "/").replace("-", "+")
            payload += "=" * (-len(payload) % 4)
            decoded = base64.b64decode(payload).decode("utf-8", "replace")
            m = re.search(r'"session_id":\s*"([^"]+)"', decoded)
            if m:
                return re.sub(r"[^A-Za-z0-9._-]", "-", m.group(1))
        except Exception:
            pass
    pid = os.getpid()
    for _ in range(64):
        if pid <= 1:
            break
        argv0 = _ps_field("comm=", pid)
        if argv0.rsplit("/", 1)[-1] == "claude":
            return str(pid)
        ppid = _ps_field("ppid=", pid)
        if not ppid.isdigit():
            break
        pid = int(ppid)
    return str(os.getppid())


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
    existing_lesson: bool,
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
        if not existing_lesson and "plan/learned/.counter" not in add_paths:
            raise UsageError("new learned summaries must add plan/learned/.counter")
        return "Lesson: " + ", ".join(learned)
    if required:
        raise UsageError(
            "lesson is required; include plan/learned/NNN-name.md and plan/learned/.counter"
        )
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
# violation, a broken plugin boundary, an unresolved iface, or a stale wiring
# index. They are therefore NOT eligible to be parked in plan/known-failures.md
# or waved through with --unverified. See ai/rules/git-safety.md.
STRUCTURAL_GATES = frozenset(
    {
        "ze-lint",
        "ze-lint-changed",
        "ze-tier-check",
        "ze-iface-resolution-check",
        "ze-plugin-boundary-check",
        "ze-cli-grammar-check",
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
        elif "is stale" in (proc.stdout or "") + (proc.stderr or ""):
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


def discovery_index_problems(repo: Path, add_paths: tuple[str, ...]) -> list[str]:
    """Reasons this commit would leave a generated discovery index stale."""
    state, stale = discovery_index_freshness(repo)
    if state == "unknown":
        return []
    if state == "stale":
        return [
            "discovery indexes are stale: " + ", ".join(stale) + ".\n"
            "  Run `make ze-regen` (or `make ze-discovery-index`) and include the\n"
            "  regenerated files in this commit."
        ]
    # Fresh on disk: a regenerated index must ride along when a source that feeds
    # it is part of this commit, or the committed tree drifts out of sync.
    if not any(feeds_discovery_index(repo, p) for p in add_paths):
        return []
    missing = [
        out
        for out in DISCOVERY_INDEX_OUTPUTS
        if out not in add_paths and index_pending(repo, out)
    ]
    if not missing:
        return []
    return [
        "this commit changes sources that feed the discovery indexes but omits\n"
        "  the regenerated index(es): " + ", ".join(missing) + ".\n"
        "  Add them: " + " ".join("--file " + m for m in missing) + "."
    ]


def discovery_index_head_status(repo: Path) -> tuple[str, list[str]]:
    """Return (state, stale) for HEAD's COMMITTED indexes vs HEAD's committed
    sources.

    Unlike discovery_index_freshness (which checks the working tree), this
    materializes HEAD and re-runs the generators there, so it catches a commit
    that landed a feeding-source change without its index update even when the
    working tree carries unrelated uncommitted changes. "unknown" (no HEAD, or
    git/tar unavailable) surfaces nothing.
    """
    if run_git(repo, "rev-parse", "--verify", "-q", "HEAD", check=False).returncode:
        return "unknown", []
    stale: list[str] = []
    confirmed = False
    try:
        with tempfile.TemporaryDirectory() as tmp:
            archive = subprocess.Popen(
                ["git", "archive", "HEAD"], cwd=repo, stdout=subprocess.PIPE
            )
            extract = subprocess.run(["tar", "-x", "-C", tmp], stdin=archive.stdout)
            if archive.stdout:
                archive.stdout.close()
            archive.wait()
            if archive.returncode or extract.returncode:
                return "unknown", []
            tmp_path = Path(tmp)
            for gen, out in zip(DISCOVERY_INDEX_GENERATORS, DISCOVERY_INDEX_OUTPUTS):
                script = tmp_path / gen
                if not script.exists():
                    continue
                proc = subprocess.run(
                    [sys.executable, str(script), "--check"],
                    cwd=tmp_path,
                    capture_output=True,
                    text=True,
                )
                if proc.returncode == 0:
                    confirmed = True
                elif "is stale" in (proc.stdout or "") + (proc.stderr or ""):
                    confirmed = True
                    stale.append(out)
    except OSError:
        return "unknown", []
    if not confirmed:
        return "unknown", []
    return ("stale" if stale else "fresh"), stale


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
# plan/deferrals.md's own header is the contract: "A row lives here only while the
# work has no home. Once it is moved into a spec, it is resolved ... and the spec
# becomes the tracker."
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


def deferral_unassigned_problems(repo: Path) -> list[str]:
    """Live rows in plan/deferrals.md with no usable destination (WARN, advisory).

    The six-column table is Date | Source | What | Reason | Destination | Status.
    A row is checked unless its Status is terminal (DEFERRAL_TERMINAL_STATUSES);
    an unrecognised status is live, so the check still fails closed on a status
    vocabulary it does not know. Independent of what this commit touches -- it
    surfaces every live row whose home is missing or is prose, across the whole
    file.

    Routed through commit_gate_warnings, NOT commit_gate_problems: an unhomed
    deferral row is harmless to software behaviour, so it is surfaced rather than
    used to block an otherwise-valid commit (ai/rules/deferral-tracking.md). The
    returned strings are printed as warnings; homing is still required by the
    rule, but a missing home no longer blocks the commit.
    """
    path = repo / "plan" / "deferrals.md"
    if not path.is_file():
        return []
    unassigned: list[str] = []
    malformed: list[str] = []
    for lineno, line in enumerate(
        path.read_text(encoding="utf-8", errors="replace").splitlines(), 1
    ):
        cells = _deferral_row_cells(line)
        if cells is None:
            continue
        if cells == ["MALFORMED"]:
            malformed.append(f"  - line {lineno}: {line.strip()[:90]}")
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
            "malformed rows in plan/deferrals.md (cannot be checked, so not\n"
            "  assumed innocent -- a row this gate cannot read is a row it cannot"
            " enforce):\n" + "\n".join(malformed) + "\n  Each row needs exactly six"
            " cells: | Date | Source | What | Reason | Destination | Status |."
        )
    return problems


def _prospective_added_lines(
    repo: Path, add_paths: tuple[str, ...], remove_paths: tuple[str, ...]
) -> list[str]:
    """The '+' lines this commit would introduce (added/modified files).

    Computed in a THROWAWAY index (GIT_INDEX_FILE) so the real staging area is
    never touched: read HEAD's tree, apply this commit's add/remove set, then
    diff --cached -U0 --diff-filter=AM. That is exactly what the generated
    `git add ... ; git commit` will record, and it captures brand-new files too.
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
    return [
        line
        for line in diff.splitlines()
        if line.startswith("+") and not line.startswith("+++") and line != "+"
    ]


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
    """Deferral language in the commit's added PROSE with no deferrals.md entry.

    Matching runs on `_deferral_prose(line)`, not the raw line, so the canonical
    `DEFERRAL_PATTERNS` list, the rule docs that quote it, and the fixtures that
    test it -- all of which now ride the gated commit path -- do not self-trip:
    their trigger phrases live inside quotes or backticks. Go's `defer f()` is
    also skipped. This prose-only match is the one intentional divergence from
    the dead `check-deferral-in-diff.sh`, which never met its own list.
    """
    if "plan/deferrals.md" in add_paths:
        return []
    prose = [
        (line, _deferral_prose(line))
        for line in _prospective_added_lines(repo, add_paths, remove_paths)
        if not re.match(r"^\+\s*defer [a-zA-Z]", line)
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
        "deferral language in staged changes without a plan/deferrals.md entry:\n"
        + "\n".join(hits)
        + "\n  Record each deferral in plan/deferrals.md before committing"
        " (ai/rules/deferral-tracking.md)."
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


def doc_drift_warnings(repo: Path) -> list[str]:
    """Advisory warning when docs drift from the live registry."""
    if not (repo / "scripts" / "docvalid" / "doc_drift.go").exists():
        return []
    try:
        res = subprocess.run(
            ("go", "run", "scripts/docvalid/doc_drift.go"),
            cwd=str(repo),
            capture_output=True,
            text=True,
            timeout=30,
        )
    except Exception:
        return []  # compile error / timeout / no toolchain -- never block
    if res.returncode == 1:
        return [(res.stdout + res.stderr).rstrip()]
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


def _pre_commit_verification_filled(spec_text: str) -> bool | None:
    """Tri-state for the spec's '## Pre-Commit Verification' section.

    Returns True (filled), False (present but empty), or None (section absent).
    "Filled" = the section has at least one table DATA row. Each table ships as
    header + separator only; data rows = pipe-rows - 2*(separator rows), since
    every table contributes exactly one header and one separator.
    """
    lines = spec_text.splitlines()
    section: list[str] = []
    in_section = False
    for line in lines:
        if line.startswith("## Pre-Commit Verification"):
            in_section = True
            continue
        if in_section and line.startswith("## "):
            break
        if in_section:
            section.append(line)
    if not in_section:
        return None
    pipe_rows = sum(1 for line in section if line.strip().startswith("|"))
    sep_rows = sum(
        1 for line in section if re.match(r"^\|[\s:|-]+\|?\s*$", line.strip())
    )
    return (pipe_rows - 2 * sep_rows) > 0


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
    filled = _pre_commit_verification_filled(
        spec_path.read_text(encoding="utf-8", errors="replace")
    )
    if filled is None:
        return [
            f"spec {claimed} has no '## Pre-Commit Verification' section, but this"
            " commit adds its learned summary (closure).\n"
            "  Add and fill it from plan/TEMPLATE.md before closing"
            " (ai/rules/planning.md)."
        ]
    if not filled:
        return [
            f"spec {claimed} '## Pre-Commit Verification' has no evidence rows, but"
            " this commit adds its learned summary (closure).\n"
            "  Re-verify each file / AC / wiring independently and paste the"
            " evidence before closing (TEMPLATE.md, ai/rules/planning.md)."
        ]
    return []


def commit_gate_problems(
    repo: Path, add_paths: tuple[str, ...], remove_paths: tuple[str, ...]
) -> list[str]:
    """All BLOCK-severity commit-time gates, in one call for create()."""
    problems: list[str] = []
    problems += deferral_in_diff_problems(repo, add_paths, remove_paths)
    problems += spec_audit_problems(repo, add_paths, claimed_spec(repo))
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
    # known-red logged in plan/known-failures.md). This turns "verify before
    # commit" from honor-system into an enforced, overridable gate. See
    # ai/rules/git-safety.md.
    vstate, detail = verify_status(repo)
    if vstate == "stale":
        # A DETERMINISTIC STRUCTURAL GATE red (tier/lint/vet/plugin-boundary/
        # iface-resolution/cli-grammar/wiring-docs) is never flaky or
        # environmental: it means the tree is structurally broken. Such a red is
        # NOT bypassable by --unverified or a plan/known-failures.md known-red
        # (those cover flaky TEST stages only). This closes the hole that let a
        # misplaced-tier gate (routeinstall) be parked as "pre-existing" and
        # shipped red on main. See ai/rules/git-safety.md.
        gate_reds = structural_gate_reds(repo)
        if gate_reds:
            raise UsageError(
                "ze-verify has a DETERMINISTIC STRUCTURAL GATE red that "
                "--unverified cannot bypass: " + ", ".join(gate_reds) + ".\n"
                "  Structural gates (tier/lint/vet/plugin-boundary/iface-resolution/\n"
                "  cli-grammar/wiring-docs) never fail for flaky or environmental\n"
                "  reasons -- a red means the tree is structurally broken. They are\n"
                "  NOT eligible for --unverified or a plan/known-failures.md known-red.\n"
                "  Fix it at the source, then re-run `make " + gate_reds[0] + "` (or\n"
                "  `make ze-verify`) until green. If you already fixed it, that re-run\n"
                "  refreshes tmp/ze-verify-failures.json and clears this."
            )
        if not args.unverified:
            raise UsageError(
                "ze-verify is not FRESH-green (" + (detail or "unknown") + ").\n"
                "  Run `make ze-verify` (or `make ze-verify-changed`) until green, then\n"
                '  commit, OR pass --unverified "<reason>" to commit anyway (owner\n'
                "  override, or a flaky/environmental known-red logged in\n"
                "  plan/known-failures.md; structural gates are never eligible)."
            )
    # Discovery-index gate: the generated maps (ai/PACKAGE-MAP.md,
    # ai/DOCS-TO-CODE.md, ai/LEARNED-FULL-INDEX.md) must match the committed
    # sources. With no CI, this is the only place the freshness is enforced.
    if not args.stale_index_ok:
        problems = discovery_index_problems(repo, add_paths)
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
        args.lesson_existing,
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
    head_state, head_stale = discovery_index_head_status(repo)
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


def learned_next(args: argparse.Namespace) -> int:
    """Allocate the next learned-summary number collision-free.

    Concurrent sessions used to read .counter hours apart and both write the
    same number (13 duplicate prefixes exist). Allocation here is
    max(existing file prefixes, .counter) + 1 and the file is created
    immediately, so any later session sees it and allocates past it.
    """
    repo = repo_root(args.repo)
    slug = args.slug
    if not SLUG_RE.match(slug):
        raise UsageError(f"slug {slug!r} must be lower-kebab-case")
    learned_dir = repo / "plan" / "learned"
    counter_path = learned_dir / ".counter"
    highest = 0
    for entry in learned_dir.glob("[0-9]*-*.md"):
        prefix = entry.name.split("-", 1)[0]
        if prefix.isdigit():
            highest = max(highest, int(prefix))
    try:
        counter = int(counter_path.read_text().strip())
    except (OSError, ValueError):
        counter = 0
    number = max(highest + 1, counter)
    path = learned_dir / f"{number:03d}-{slug}.md"
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o644)
    with os.fdopen(fd, "w") as fh:
        fh.write(f"# {slug}\n\n<!-- write the learned summary here -->\n")
    counter_path.write_text(f"{number + 1}\n")
    print(path.relative_to(repo).as_posix())
    return 0


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
        help="allocate the next plan/learned number, create the file, bump .counter",
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
        "--lesson-existing",
        action="store_true",
        help="the learned file is an edit to an existing lesson, so .counter is not required",
    )
    create_cmd.add_argument(
        "--lesson-not-needed",
        help="explicit reason no learned summary is useful for this commit",
    )
    create_cmd.add_argument(
        "--unverified",
        help="reason to allow a commit when ze-verify is not FRESH-green "
        "(owner override, or a flaky/environmental known-red logged in "
        "plan/known-failures.md; deterministic structural gates are never eligible)",
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
