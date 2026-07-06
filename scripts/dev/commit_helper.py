#!/usr/bin/env python3
"""Generate safe user-run commit scripts for Ze."""

from __future__ import annotations

import argparse
import base64
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


def render_block(block: CommitBlock) -> str:
    lines = [
        f"# Commit {block.tag}: {block.subject}",
        f"# {block.lesson_comment}",
    ]
    if block.add_paths:
        lines.append(render_git_add(block.add_paths))
    if block.remove_paths:
        lines.append("git rm -- " + quote_paths(block.remove_paths))
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
    if vstate == "stale" and not args.unverified:
        raise UsageError(
            "ze-verify is not FRESH-green (" + (detail or "unknown") + ").\n"
            "  Run `make ze-verify` (or `make ze-verify-changed`) until green, then\n"
            '  commit, OR pass --unverified "<reason>" to commit anyway (owner\n'
            "  override, or a known-red logged in plan/known-failures.md)."
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
        tag, args.subject.strip(), add_paths, remove_paths, msg_path, comment
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
        "(owner override, or a known-red logged in plan/known-failures.md)",
    )
    create_cmd.add_argument(
        "--stale-index-ok",
        help="reason to allow a commit when a generated discovery index "
        "(ai/PACKAGE-MAP.md, ai/DOCS-TO-CODE.md, ai/LEARNED-FULL-INDEX.md) is "
        "stale or omitted (owner override)",
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
