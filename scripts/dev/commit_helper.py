#!/usr/bin/env python3
"""Generate safe user-run commit scripts for Ze."""

from __future__ import annotations

import argparse
import datetime
import fcntl
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
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path

# The weakened-test gate. `make ze-test-weakened-check` and the edit-time hook run the
# same module, and this file imports it rather than judging a diff itself: two
# spellings of "what does this change weaken" give two gates two answers about
# one edit, which is the drift scripts/dev/rfc_tagged_scope.py records.
import check_weakened_tests

# Discovery-index generators/outputs and the source-trigger predicate are shared
# with the changed-file router (verify_wiring_docs.py) via discovery_sources.py,
# so the commit gate here and the router cannot drift apart.
from discovery_sources import GENERATORS as DISCOVERY_INDEX_GENERATORS
from discovery_sources import OUTPUTS as DISCOVERY_INDEX_OUTPUTS
from discovery_sources import STALE_EXIT as DISCOVERY_STALE_EXIT
from discovery_sources import indexes_fed_by as _indexes_fed_by
from discovery_sources import is_discovery_source as _is_discovery_source

# The journal row parser has ONE implementation, in journal.py, imported the way
# deferral_orphans.py imports _deferral_row_cells from here. Three copies had
# already diverged: spec-closure-check.py's returned None where the others
# returned a malformed marker, so closure evidence skipped malformed rows.
from journal import MALFORMED as JOURNAL_MALFORMED

# The throwaway worktree a debt-clear pass runs its gates in, and the environment
# it runs them under. Both come from verify_worktree.py rather than being spelled
# again here: `make ze-verify-worktree` and this pass ask the same question of the
# same commit, and two spellings of "materialize HEAD and run a gate in it" would
# drift where nobody could see it -- both would still run, and only one would be
# admitted by scripts/dev/ze-run.sh.
from verify_worktree import gate_env as _gate_env
from verify_worktree import worktree_at as _worktree_at
from journal import journal_row_cells, journal_spec_stems

SESSION_RE = re.compile(r"^[0-9a-f]{8}$")
TAG_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,31}$")
LEARNED_RE = re.compile(r"^plan/learned/[0-9]{3,}-.+\.md$")
COMMIT_MESSAGE_WIDTH = 72

# Provenance markers written into every generated script. The first identifies
# the script itself, the second records which files each commit block stages, so
# --replace can refuse a script prepared for an unrelated commit.
SCRIPT_MARKER = "# ze-commit-script:"
BLOCK_MARKER = "# ze-commit-block:"
# Marks the optional push section, which is always the LAST thing in a script.
# `--append` finds it by this marker, lifts it, and re-emits it after the new
# block, so a second commit can never be added BELOW the push that was meant to
# publish it.
PUSH_MARKER = "# ze-commit-push:"

# Anything a shell would read as a line terminator, plus the rest of the C0/DEL
# range. A `#` comment is NOT a safe sink for caller text: a newline inside a
# value ENDS the comment, and everything after it becomes a script line. That is
# enough to fabricate a `# ze-commit-push:` marker -- an authorisation nobody
# gave -- or a bare command, so no caller value may be interpolated into a
# comment raw.
CONTROL_CHARS_RE = re.compile(r"[\x00-\x1f\x7f]")


def comment_safe(text: str) -> str:
    """A caller value reduced to one line, so it cannot leave a `#` comment.

    INVARIANT: the result is unchanged by flattening it again --
    `r.splitlines() == ([r] if r else [])` for every input. Everything that
    reads a generated script back splits it into lines, so "one line" has to
    mean what the SPLITTER means by it, not what a hand-kept character list
    says.

    Two steps, and the second is why the list cannot drift. Every control
    character becomes a space, one for one, and the ends are trimmed. Replacing
    rather than collapsing keeps the value faithful: a path holding two spaces
    still reads as two spaces in the provenance line a `--replace` parses back
    (`script_declared_paths`). Then whatever `str.splitlines` STILL calls a line
    boundary -- U+0085, U+2028, U+2029, none of them in the C0/DEL range -- is
    joined on a single space by the same function that will later split the
    script. A widened character class would have to be kept in sync with
    `splitlines` by hand; deriving the flattening from `splitlines` cannot fall
    behind it.

    The gap this closes was a self-inflicted refusal, not a forgery: bash reads
    U+2028 as one more byte of a comment, so nothing escaped, but a `--push`
    reason holding one rendered a marker line that `split_push_section` then
    read back short and refused as "not its final section", telling the caller
    to discard a perfectly good prepared commit.
    """
    flat = " ".join(CONTROL_CHARS_RE.sub(" ", text).splitlines())
    return flat.strip()


def marker_line(marker: str, payload: str) -> str:
    """A `#` marker line whose payload can never end the comment.

    THE only way a caller-supplied string is written into the generated script as
    a comment. Newline safety used to be a per-flag concern -- `push_authorisation`
    flattened its own value and nothing else did -- so the subject and the staged
    paths each rendered raw, and a newline in either of them forged a push marker
    (see split_push_section).

    Flattening alone still lets a value SPELL the push marker at the start of its
    own line, which `split_push_section` would then have to adjudicate. Refuse it
    here instead: only `render_push` may write that marker.
    """
    line = (marker + " " + comment_safe(payload)).rstrip()
    if marker != PUSH_MARKER and line.startswith(PUSH_MARKER):
        raise UsageError(
            "refusing a value that spells the push marker: "
            + repr(line)
            + "\n  "
            + PUSH_MARKER
            + " records an owner's order to publish, and only --push\n"
            "  writes it. A value that renders as that line would read as an\n"
            "  authorisation nobody gave. Reword it."
        )
    return line


def comment_line(text: str) -> str:
    """One plain `#` comment carrying `text`, whatever `text` contains."""
    return marker_line("#", text)


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
    # Refused, not flattened. A path reaches the script twice: quoted inside
    # `git add`, where shlex.quote makes a newline harmless, and RAW inside the
    # `# ze-commit-block:` provenance comment, where the same newline ends the
    # comment and the remainder becomes a script line (a forged push marker, or a
    # command). Rewriting the path to make the comment safe would leave the
    # recorded provenance disagreeing with what `git add` actually stages, which
    # is what `--replace` reads back. No legitimate path in this repository holds
    # a control character.
    #
    # The second clause asks the SPLITTER, for the reason comment_safe does: a
    # path holding U+0085, U+2028 or U+2029 clears the character class and still
    # breaks a line for every reader of the script. Flattened into the provenance
    # comment, it would leave that comment disagreeing with what `git add`
    # stages, and a later `--replace` would read the disagreement as somebody
    # else's prepared commit and refuse it.
    if CONTROL_CHARS_RE.search(raw) or raw.splitlines() != [raw]:
        raise UsageError(
            "path contains a line break or control character and cannot be "
            "committed: "
            + repr(raw)
            + "\n  A newline, tab, or escape in a path would break out of the "
            "provenance\n  comment the generated script records for it."
        )
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
    tmp/commit-session-id they shared one script path, and a --replace from one
    session silently overwrote the other's prepared script (observed 2026-06-10).
    The session is no longer the whole answer -- one session runs many subagents
    and they all resolve here, so the SCRIPT path adds a per-commit tag and a
    nonce (`script_rel_path`). The session still names the message and script
    namespace. Delegates to the ONE shared session-id resolver
    (.claude/hooks/lib/session_id.py), so this file keys its session script on the
    exact id the hooks use -- no fourth spelling to drift. The resolver already
    guarantees the id is a safe filename component.
    """
    return _ze_session_id.session_id()


def session_id(repo: Path, requested: str | None) -> str:
    tmp_dir = repo / "tmp"
    tmp_dir.mkdir(exist_ok=True)
    # Per-Claude-session file: concurrent sessions must never share one
    # message/script namespace.
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


def message_rel_path(session: str, tag: str) -> str:
    return f"tmp/commit-msg-{session}-{tag}.txt"


def _reserve(path: Path) -> bool:
    """Create path empty, exclusively. False when another process won the race."""
    try:
        fd = os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o644)
    except FileExistsError:
        return False
    os.close(fd)
    return True


def next_tag(repo: Path, session: str) -> str:
    """Allocate the next free message tag and RESERVE it in the same step.

    Allocation used to be a glob of the tags already on disk, while the message
    file that proves a tag taken is written at the END of `create` -- after
    verify-status and the discovery-index materialization, seconds later. Two
    agents of ONE Claude session (they share the session fingerprint, so they
    share this namespace) both read the same free letter inside that window and
    the second message file overwrote the first. The reservation is an O_EXCL
    create of the empty message file, so the winner is decided by the
    filesystem rather than by timing.

    The reservation is released by `create` when the run fails or is a dry run
    (`release_tag_reservation`), so a refused commit does not burn a letter.
    """
    (repo / "tmp").mkdir(exist_ok=True)
    for code in range(ord("a"), ord("z") + 1):
        tag = chr(code)
        if _reserve(repo / message_rel_path(session, tag)):
            return tag
    for _ in range(64):
        tag = "n" + secrets.token_hex(2)
        if _reserve(repo / message_rel_path(session, tag)):
            return tag
    raise UsageError(
        "no free message tag for session " + session + "; clear old "
        "tmp/commit-msg-* files"
    )


def release_tag_reservation(path: Path | None) -> None:
    """Drop an unused tag reservation (the empty file `next_tag` created)."""
    if path is None:
        return
    try:
        if path.stat().st_size == 0:
            path.unlink()
    except OSError:
        pass


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
        # The overage and the subject itself, not just the limit. A create call
        # carries a multi-paragraph --body, so a refusal here costs the caller a
        # full re-invocation of a very long command; two sessions reported that
        # cost independently on 2026-08-22. Naming the limit alone leaves them
        # counting characters by hand to find a subject that fits, which is a
        # second retry. Naming the overage makes the next attempt the last one.
        over = len(cleaned_subject) - COMMIT_MESSAGE_WIDTH
        raise UsageError(
            f"--subject is {len(cleaned_subject)} characters, "
            f"{over} over the {COMMIT_MESSAGE_WIDTH} limit. "
            f"Cut {over} character{'s' if over > 1 else ''} from: {cleaned_subject}"
        )
    parts = [cleaned_subject]
    cleaned_body = wrap_commit_body(body)
    if cleaned_body:
        parts.extend(("", cleaned_body))
    return "\n".join(parts) + "\n"


def learned_paths(paths: tuple[str, ...]) -> tuple[str, ...]:
    return tuple(path for path in paths if LEARNED_RE.match(path))


def closure_reminder(
    add_paths: tuple[str, ...],
    remove_paths: tuple[str, ...],
    repo: Path | None = None,
) -> str | None:
    """Warn when a commit adds a closure artifact but closes no spec.

    The two-commit spec closure is: commit A adds the closure artifact (a
    plan/learned/NNN-*.md or a plan/journal/*.md row naming the spec), commit B
    does `git rm plan/spec-<stem>.md`. Commit B is the step that gets dropped,
    orphaning the spec in plan/ with Status=in-progress. If this commit adds a
    closure artifact and removes no spec, nudge the caller to prepare the
    closure commit next. See ai/rules/planning.md "Spec Closure".

    A journal row counts only when the row this commit ADDS names a spec. A row
    is written when a problem is FOUND, so a mid-work commit carrying a `-` row
    is not closing anything and must not be nudged as though it were.

    A row naming a spec that closed EARLIER does not count either, for the same
    reason `spec_closure_stem` filters it: editing an old row is not closing the
    spec it names. Without the filter this nudge tells the caller to prepare a
    commit B for a spec whose file git removed months ago.

    A row naming an OPEN spec this session neither claims nor carries is another
    session's row (`_spec_belongs_to_another_session`), and this commit closes
    nothing by landing it. Both readers of `_journal_added_spec_stems` apply both
    filters: one that landed in `spec_closure_stem` alone is how these two last
    disagreed about what a closure is.

    The removal is read through `_spec_closing_removals` for the same reason. A
    commit that MOVES a spec to `plan/future/` removes `plan/spec-<stem>.md` and
    adds the file again, so it is no commit B and it silences no nudge.
    """
    has_learned = bool(learned_paths(add_paths))
    has_journal = bool(
        repo is not None
        and [
            stem
            for stem in _journal_added_spec_stems(repo, add_paths, remove_paths)
            if not _spec_closed_earlier(repo, stem)
            and not _spec_belongs_to_another_session(repo, stem, add_paths)
        ]
    )
    if not has_learned and not has_journal:
        return None
    if _spec_closing_removals(add_paths, remove_paths):
        return None
    return (
        "closure-reminder: this commit adds a closure artifact but removes no "
        "spec.\n"
        "  If it completes a spec, prepare the closure commit next:\n"
        "    git rm plan/spec-<stem>.md   (ai/rules/planning.md Spec Closure)\n"
        "  List what is still open: scripts/dev/spec-closure-check.py --list"
    )


def quote_paths(paths: tuple[str, ...]) -> str:
    return " ".join(shlex.quote(path) for path in paths)


def render_git_add(paths: tuple[str, ...]) -> str:
    lines = ["git add -- \\"]
    for index, path in enumerate(paths):
        suffix = " \\" if index + 1 < len(paths) else ""
        lines.append("  " + shlex.quote(path) + suffix)
    return "\n".join(lines)


def render_staging_guard(paths: tuple[str, ...], script_rel: str = "") -> str:
    """Guard for the generated commit script: abort if the shared git index holds
    staged paths this commit did not stage.

    Concurrent Claude sessions share one working tree AND one git index, so a
    sibling's leftover `git add` (e.g. a commit that failed at gpg signing) would
    otherwise be swept into THIS commit. The guard runs after this commit's own
    add/rm, so any remaining staged-but-unexpected path is foreign -> abort.

    The abort names the OWNING script, because the report a reader needs at that
    moment is which prepared commit they just ran, not only which files were
    unexpected.
    """
    if not paths:
        return ""
    expected = " ".join("-e " + shlex.quote(p) for p in sorted(set(paths)))
    # shlex.quote, because this is a live `echo`, not a comment: a script path
    # holding a quote or a `$(...)` would otherwise close the string and run.
    owner = (
        []
        if not script_rel
        else ["  echo " + shlex.quote("  this script: " + script_rel) + " >&2"]
    )
    return "\n".join(
        [
            "# Concurrency guard: refuse to sweep a concurrent session's staged files",
            "# into this commit (sessions share one working tree + git index).",
            # core.quotePath=false: git otherwise C-quotes non-ASCII paths (café ->
            # "caf\303\251"), which the raw grep pattern would miss -> false abort.
            f"_ze_foreign=$(git -c core.quotePath=false diff --cached --name-only | grep -vxF {expected} || true)",
            'if [ -n "$_ze_foreign" ]; then',
            '  echo "ABORT: index has staged files not in this commit (concurrent session?):" >&2',
            *owner,
            '  echo "$_ze_foreign" >&2',
            "  exit 1",
            "fi",
        ]
    )


def render_block(block: CommitBlock, script_rel: str = "") -> str:
    # Every line here carries caller text -- the subject and the staged paths --
    # so every line goes through marker_line/comment_line. Neither may be
    # interpolated raw: a newline in either one ends its comment and turns the
    # remainder into shell (see marker_line).
    lines = [
        comment_line(f"Commit {block.tag}: {block.subject}"),
        # Provenance: which files this block commits, machine-readable. --replace
        # reads it back to refuse a script prepared for an unrelated file set.
        marker_line(
            BLOCK_MARKER,
            f"tag={block.tag} paths="
            + quote_paths(block.add_paths + block.remove_paths),
        ),
    ]
    if block.review_check:
        lines.append("# critical-review gate re-check (ai/rules/planning.md)")
        lines.append(block.review_check)
    if block.add_paths:
        lines.append(render_git_add(block.add_paths))
    if block.remove_paths:
        lines.append("git rm -- " + quote_paths(block.remove_paths))
    guard = render_staging_guard(block.add_paths + block.remove_paths, script_rel)
    if guard:
        lines.append(guard)
    lines.append("git commit -F " + shlex.quote(block.message_path))
    return "\n".join(lines) + "\n"


MIN_PUSH_AUTHORISATION = 12


def push_authorisation(reason: str | None) -> str | None:
    """Normalize a `--push` authorisation to one comment-safe line, or None.

    What is MECHANICAL here is small and worth stating exactly: the script
    records a string of at least MIN_PUSH_AUTHORISATION characters, on one line,
    and `push=AUTHORISED (<that string>)` is echoed so the caller reads it before
    running the script. Nothing verifies that the string names a real person:
    `--push "aaaaaaaaaaaa"` passes. Naming WHO ordered the push and WHEN is a
    convention backed by a recorded string, and the only mechanical gate on
    pushing is the ban on the bare command (`ai/rules/git-safety.md`).

    Comment safety is not convention. The value is written into a `#` comment, a
    newline inside it would end that comment and turn what follows into shell, so
    it goes through `comment_safe` like every other caller value the script
    carries.
    """
    if reason is None:
        return None
    flat = comment_safe(reason)
    if len(flat) < MIN_PUSH_AUTHORISATION:
        raise UsageError(
            "--push authorisation is too short: "
            f'"{flat}" is {len(flat)} characters, '
            f"{MIN_PUSH_AUTHORISATION} is the minimum.\n"
            "  Pushing is an owner instruction, never an agent's choice, and the\n"
            "  reason is the record of who gave it. Name them and when, e.g.\n"
            '  --push "Thomas ordered the push, 2026-08-05".'
        )
    return flat


def render_push(authorisation: str) -> str:
    """The push section: the authorisation, then one `git push`.

    It carries no guard of its own and needs none. The generated header is
    `set -euo pipefail`, so the first failing command ends the script: a failed
    `git add`, a failed `git commit`, or the concurrency guard's own `exit 1`.
    This line is therefore reached only when EVERY commit block above it
    succeeded, which is the whole point of putting the push in the script rather
    than running it beside one.
    """
    return (
        "\n".join(
            [
                marker_line(PUSH_MARKER, authorisation),
                "# Push authorised by the owner (ai/rules/git-safety.md). Reached",
                "# only if every commit above succeeded -- `set -e` aborts before",
                "# here on the first failure.",
                "git push",
            ]
        )
        + "\n"
    )


def split_push_section(text: str) -> tuple[str, str | None]:
    """Split a generated script into (body without its push, its authorisation).

    The second element is None when the script has no push section, and the
    recorded authorisation string when it has one. Used by `--append`: the body
    takes the new block, then the push is re-emitted at the end.

    A marker is an authorisation ONLY as the script's final section: the text
    from it to the end of the script must be exactly what `render_push` writes,
    trailing newlines aside. That admits one marker and no line below the push.
    Anything else is refused rather than read.

    Scanning for the first marker line and truncating there was the second half
    of the forgery: a `# ze-commit-push:` planted anywhere in the script -- inside
    a subject, inside a path -- was read as the owner's authorisation AND
    silently cut every commit block below it out of the script. Position is what
    distinguishes a section this helper wrote from a line some caller's text
    happened to spell.
    """
    lines = text.splitlines(keepends=True)
    found = [index for index, line in enumerate(lines) if line.startswith(PUSH_MARKER)]
    if not found:
        return text, None
    index = found[0]
    authorisation = comment_safe(lines[index][len(PUSH_MARKER) :])
    tail = "".join(lines[index:])
    if len(found) > 1 or tail.rstrip("\n") != render_push(authorisation).rstrip("\n"):
        raise UsageError(
            "refusing a script whose push marker is not its final section: "
            + repr(lines[index].strip())
            + "\n  A "
            + PUSH_MARKER
            + " line is an owner authorisation only when it is the LAST section\n"
            "  of the script and nothing but the push follows it. This one is not,\n"
            "  so it was written by something other than this helper -- a value that\n"
            "  escaped its comment, or a hand edit.\n"
            "  Reading it would fabricate an authorisation nobody gave and drop the\n"
            "  commit blocks below it. Delete the script and prepare a fresh one\n"
            "  (drop --script to get a new path)."
        )
    body = "".join(lines[:index]).rstrip("\n")
    return (body + "\n" if body else ""), authorisation


def read_script_text(script: Path) -> str:
    """An existing script's text, or a UsageError naming the file.

    The append path re-reads a script it is about to rewrite. A file that is not
    UTF-8 is not one this helper wrote, so the read fails either way; what
    changes here is that the caller gets the path and the next step instead of a
    UnicodeDecodeError traceback. `errors="replace"` is deliberately NOT used:
    substituting U+FFFD would let a doctored script through the round-trip
    checks with its bad bytes silently rewritten.
    """
    try:
        return script.read_text(encoding="utf-8")
    except UnicodeDecodeError as exc:
        raise UsageError(
            "cannot read the script to append to, it is not UTF-8: "
            + script.as_posix()
            + f"\n  ({exc.reason} at byte {exc.start})\n"
            "  A script this helper wrote is UTF-8, so this one was corrupted or\n"
            "  hand-edited. Delete it and prepare a fresh one (drop --script to "
            "get a new path)."
        ) from exc


def script_rel_path(session: str, tag: str) -> str:
    """Path of a NEWLY prepared commit script. Unique per PREPARED COMMIT.

    Both halves of the suffix carry weight. <tag> is the per-commit tag the
    message file already uses, now reserved atomically (`next_tag`), so two
    agents preparing at once cannot land on one path. <nonce> makes the name
    unguessable, so no caller can reach another agent's script by rebuilding the
    convention: the helper's own `script=` line is the only way to learn a path.

    Keying on the Claude session alone (2026-06-10) was enough while a session
    was one agent. It stopped being enough once one session ran many subagents:
    they share the fingerprint, so they shared `tmp/commit-<SESSION>.sh`, and a
    --replace from one silently overwrote another's prepared script (measured
    2026-08-05: 53 message files against 18 scripts in ONE session).
    """
    return f"tmp/commit-{session}-{tag}-{secrets.token_hex(3)}.sh"


def looks_like_commit_script(text: str) -> bool:
    """True for a script this helper generated (new form or pre-2026-08-05 form)."""
    return "git commit -F " in text


def script_declared_paths(text: str) -> set[str]:
    """The paths a generated script's commit blocks declare they will commit."""
    declared: set[str] = set()
    for line in text.splitlines():
        if not line.startswith(BLOCK_MARKER):
            continue
        _, _, rest = line.partition("paths=")
        if rest.strip():
            declared.update(shlex.split(rest))
    return declared


def session_scripts(repo: Path, session: str) -> list[Path]:
    """Every prepared script of this session, new-form and legacy, sorted."""
    tmp_dir = repo / "tmp"
    found = set(tmp_dir.glob(f"commit-{session}-*.sh")) | set(
        tmp_dir.glob(f"commit-{session}.sh")
    )
    return sorted(
        p
        for p in found
        if p.is_file()
        and looks_like_commit_script(p.read_text(encoding="utf-8", errors="replace"))
    )


def resolve_target_script(
    repo: Path, session: str, script_arg: str | None, append: bool, replace: bool
) -> Path | None:
    """The existing script this create must write into, or None for a fresh one.

    `--script` is the authoritative form: it names a path the caller read from an
    earlier `script=` line. `--append` without it stays supported for the single
    agent case and resolves ONLY when the session has exactly one script; with
    several it refuses and lists them, because picking one would be the guess
    this change exists to remove.
    """
    if script_arg:
        rel = rel_path(repo, script_arg)
        target = repo / rel
        if not target.is_file():
            raise UsageError(f"--script does not exist: {rel}")
        text = target.read_text(encoding="utf-8", errors="replace")
        if not looks_like_commit_script(text):
            raise UsageError(f"--script is not a generated commit script: {rel}")
        if not append and not replace:
            raise UsageError(f"{rel} exists; pass --append or --replace")
        return target
    if not append:
        return None
    candidates = session_scripts(repo, session)
    if not candidates:
        raise UsageError(
            "--append: this session has no prepared script to append to.\n"
            "  Drop --append to prepare a new one, or pass --script <path> from "
            "the\n  `script=` line of the create that made it."
        )
    if len(candidates) > 1:
        listed = "\n".join("    " + p.relative_to(repo).as_posix() for p in candidates)
        raise UsageError(
            "--append is ambiguous: this session has "
            + str(len(candidates))
            + " prepared scripts.\n"
            "  Pass --script <path> from the `script=` line of the create you are\n"
            "  extending. Never reconstruct the path by convention:\n" + listed
        )
    print(
        "note: --append resolved to "
        + candidates[0].relative_to(repo).as_posix()
        + "; pass --script <path> to name it explicitly",
        file=sys.stderr,
    )
    return candidates[0]


def refuse_foreign_replace(repo: Path, script: Path, block: CommitBlock) -> None:
    """Refuse a --replace over a script prepared for an unrelated file set.

    --replace means "replace MY script". A script whose declared paths do not
    overlap this commit's at all is another prepared commit, so replacing it
    destroys work that was never run. Failing closed costs nothing: dropping
    --script yields a fresh unique path.
    """
    text = script.read_text(encoding="utf-8", errors="replace")
    declared = script_declared_paths(text)
    if not declared:
        return
    mine = set(block.add_paths) | set(block.remove_paths)
    if declared & mine:
        return
    sample = ", ".join(sorted(declared)[:4]) + (", ..." if len(declared) > 4 else "")
    raise UsageError(
        "--replace refused: "
        + script.relative_to(repo).as_posix()
        + " was prepared for a different\n"
        "  commit and has not been run. It commits: "
        + sample
        + "\n  This commit shares none of those files, so replacing it would "
        "destroy\n  another prepared commit. Drop --script to get a fresh script "
        "path."
    )


def replaced_push_authorisation(script: Path) -> str | None:
    """The push authorisation a `--replace` is about to discard, or None.

    `--replace` rewrites the script from its header down, so a push the owner
    authorised for the commit it USED to prepare does not carry over. Dropping it
    is the fail-safe direction and stays. Dropping it in silence is not: the
    caller holds a path they were told would publish, and the next thing they
    read about it must not be a script that quietly no longer does.

    A script this helper cannot parse (a forged or hand-edited push section)
    reports None rather than raising: `--replace` discards that content whole, so
    there is nothing to warn about and nothing to refuse.
    """
    try:
        _, authorisation = split_push_section(
            script.read_text(encoding="utf-8", errors="replace")
        )
    except UsageError:
        return None
    return authorisation


def write_outputs(
    repo: Path,
    session: str,
    block: CommitBlock,
    message: str,
    append: bool,
    replace: bool,
    dry_run: bool,
    script_arg: str | None = None,
    push_reason: str | None = None,
) -> tuple[Path, str | None]:
    if append and replace:
        raise UsageError("--append and --replace are mutually exclusive")
    message_file = repo / block.message_path
    target = resolve_target_script(repo, session, script_arg, append, replace)
    if target is None:
        # Bounded: a fresh path that keeps coming back taken means the name is
        # no longer unique per prepared commit, and looping forever would hide
        # that behind a hang.
        for _ in range(64):
            script = repo / script_rel_path(session, block.tag)
            if not script.exists():
                break
        else:
            raise UsageError(
                "cannot allocate an unused script path for tag "
                + block.tag
                + "; tmp/ already holds every name script_rel_path produced"
            )
    else:
        script = target
        if replace:
            refuse_foreign_replace(repo, script, block)
            dropped = replaced_push_authorisation(script)
            if dropped is not None and push_reason is None:
                print(
                    "note: --replace dropped the push this script carried: "
                    + dropped
                    + "\n  A replaced script is written from its header down, so an "
                    "authorisation\n  given for the commit it used to prepare does not "
                    'carry over. Pass\n  --push "<who ordered it, when>" again if the '
                    "owner ordered this one\n  published.",
                    file=sys.stderr,
                )
    script_rel = script.relative_to(repo).as_posix()
    header = (
        "#!/bin/bash\nset -euo pipefail\n"
        'cd "$(git rev-parse --show-toplevel)"\n\n'
        # script_rel comes from --script, so it is caller text like any other.
        + marker_line(SCRIPT_MARKER, f"{script_rel} session={session}")
        + "\n\n"
    )
    append_here = append and script.exists()
    block_text = render_block(block, script_rel)
    # The push belongs to the SCRIPT, not to a block: one push, always last, no
    # matter how many blocks were appended. An --append lifts the push the script
    # already carries and re-emits it after the new block, so the appended commit
    # ends up INSIDE the push rather than below it. Repeating --push on the append
    # re-authorises it and the newer reason wins; omitting --push keeps the
    # authorisation the script already recorded, because dropping a push the owner
    # ordered would be as surprising as adding one they did not.
    existing_body, existing_push = (
        split_push_section(read_script_text(script)) if append_here else ("", None)
    )
    authorisation = push_reason if push_reason is not None else existing_push
    push_text = render_push(authorisation) if authorisation is not None else ""
    if append_here:
        body = existing_body
        if body and not body.endswith("\n"):
            body += "\n"
        text = body + "\n" + block_text
    else:
        text = header + block_text
    if push_text:
        text += "\n" + push_text
    if dry_run:
        print(f"session={session}")
        print(f"message={block.message_path}")
        print(f"script={script.relative_to(repo).as_posix()}")
        print("--- message ---")
        print(message, end="")
        print("--- script ---")
        print(text, end="")
        return script, authorisation
    message_file.write_text(message, encoding="utf-8")
    script.write_text(text, encoding="utf-8")
    script.chmod(script.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
    return script, authorisation


def manifest_quotes(path: str) -> bool:
    """True when git C-quotes `path`, so no manifest row can spell it raw.

    `dirty_manifest` (scripts/dev/verify-status.sh) and `computeDirtyManifest`
    (scripts/status/verify_run.go) both record a path as `git diff --name-only`
    and `git ls-files -o` PRINT it, and git C-quotes any path carrying a byte
    outside printable ASCII (0x20-0x7E), a double quote, or a backslash. A space
    is NOT quoted, which is why a space is representable in a row and these
    bytes are not.
    """
    return any(ch in '"\\' or not (" " <= ch <= "~") for ch in path)


def scopeable_paths(paths: tuple[str, ...] | list[str]) -> tuple[str, ...]:
    """The commit's paths, or () when the manifest cannot answer about one.

    () is the whole-tree question, which is what `verify-status.sh check` asks
    with no arguments. It is the conservative answer: it is what this gate did
    before scoping existed, and it can only refuse more. Scoping an
    unrepresentable path would instead make the gate answer yes to a question it
    cannot ask.

    Two shapes are unrepresentable, and both widen to (). `manifest_scoped`
    (scripts/dev/verify-status.sh) matches a row when the row's path equals the
    scope argument or starts with it plus a slash, so each of them selects
    nothing from the recorded manifest AND nothing from the live one -- and two
    empty sets compare EQUAL, a FRESH answer covering nothing at all. That is a
    guard failing open, which `ai/rules/evidence.md` refuses.

    An EMPTY path: no real row's path equals "" or starts with "/".

    A path git C-QUOTES (`manifest_quotes`): the row spells it quoted while the
    scope argument is the raw path, so neither side matches. Measured on
    2026-08-19 against the real checker: an edited path holding an accented
    letter and an edited path holding a backslash both answered FRESH, while
    `my file.txt` answered STALE.

    The real repair is to make both producers record the RAW path, and it is not
    a one-line change: the manifest format is agreed between two producers in
    two languages, and a path holding a NEWLINE breaks its one-row-per-line
    shape outright. Until that lands, the checker must never be ASKED about such
    a path, and this is the only caller that passes it any.

    A space used to land here too, because `manifest_scoped` read the path as
    awk's $2. That is fixed at the producer: the path is everything after the
    first space, so a space is asked about rather than widened away.
    `rel_path` already refuses a control character.
    """
    scope = tuple(paths)
    if any(not path or manifest_quotes(path) for path in scope):
        return ()
    return scope


def verify_status(repo: Path, paths: tuple[str, ...] | list[str]) -> tuple[str, str]:
    """Return (state, detail) from scripts/dev/verify-status.sh check <paths>.

    `paths` is the file list this commit carries, and it SCOPES the question:
    did these paths change since the PASS, rather than did the checkout change.
    Several sessions share this checkout and it routinely carries 300+
    uncommitted files, so the whole-tree answer is STALE within seconds of a
    PASS -- almost always for a file the asking session never wrote. An empty
    list keeps the whole-tree meaning, which `hook-parity-check.py` pins.

    Scoping the FRESHNESS question is not a route around a structural red: that
    is `structural_gate_reds`, and it still reads every red the run recorded.

    state is "fresh", "stale", or "unknown". Only "stale" costs the commit a
    verification-debt row: the gate charges a CONFIRMED red, it never invents one
    when the checker is unavailable (missing script, isolated test repo, minimal
    checkout). This never raises.

    Since 2026-08-21 a "stale" answer does not refuse the commit. It records the
    row, the commit proceeds, and `--push` refuses while the row is open
    (ai/rules/git-safety.md, "Verify a Commit, Not the Working Tree"). What still
    refuses at commit time is a structural red charged to the commit's files,
    which `structural_gate_reds` answers.
    """
    script = repo / "scripts" / "dev" / "verify-status.sh"
    if not script.exists():
        return "unknown", "verify-status.sh not found"
    try:
        proc = subprocess.run(
            [str(script), "check", *scopeable_paths(paths)],
            cwd=repo,
            capture_output=True,
            text=True,
        )
    except OSError as exc:
        return "unknown", f"verify-status.sh did not run: {exc}"
    out = (proc.stdout or proc.stderr or "").strip().splitlines()
    return ("fresh" if proc.returncode == 0 else "stale"), (out[0] if out else "")


# Deterministic structural gates in `make ze-precommit-verify` (the non-test stages in
# scripts/status/verify_run.go stagesForMode). Unlike the unit/functional/exabgp
# TEST stages, these NEVER fail for flaky or environmental reasons: a red means
# the tree is structurally broken -- a module-tier misplacement, a lint or vet
# violation, a broken plugin boundary, an unresolved iface, a failed feature-tag
# type-check, a stale generated file, a stale wiring index, or a HEAD that does
# not compile (ze-repository-tracked-build-check). They are therefore NOT eligible to be parked
# in plan/known-failures/, and they are the one fact `create` still refuses to
# commit over: an unverified commit lands and owes a debt row, a structurally
# broken one does not. Every name here MUST be a stage that
# stagesForMode actually emits, or it matches nothing and gates nothing;
# that is enforced by TestStructuralGatesAreLiveStages (Go, scripts/status)
# and test_structural_gates_are_live_stages (Python, scripts/dev).
# See ai/rules/git-safety.md.
# The stage whose red is cleared BY a commit rather than before one, because it
# judges what git already holds. See the escape in create().
TRACKED_BUILD_GATE = "ze-repository-tracked-build-check"

STRUCTURAL_GATES = frozenset(
    {
        "ze-lint",
        "ze-lint-changed",
        "ze-tier-check",
        "ze-iface-resolution-check",
        "ze-plugin-boundary-check",
        "ze-generated-files-check",
        "ze-doc-wiring-check",
        "ze-evidence-vet",
        "ze-staticcheck-feature-matrix-check",
        TRACKED_BUILD_GATE,
    }
)


@dataclass(frozen=True)
class GateReds:
    """The last verify's structural reds, attributed against a commit's file list.

    `charged` refuses the commit. `unattributed` names the groups that put a gate
    into `charged` for want of any path at all, rather than for a path this commit
    carries: AC-4b makes the helper say that out loud, so the blind spot is
    visible instead of silent. `foreign` names the gates attribution dropped --
    every group of theirs named files, and every one of those files lies outside
    the commit.
    """

    charged: tuple[str, ...]
    unattributed: tuple[str, ...]
    foreign: tuple[str, ...]


# The `failureGroup.Kind` values (scripts/status/verify_run.go) whose `Related`
# members can name a repository path, and the producer that writes each one:
# `files` from `declare_failure_group` (scripts/dev/verify_wiring_docs.py), whose
# members are repository paths; `lint` from `classifyLint`, whose members are .go
# files; `package` from `classifyVet`, whose member is a repo-relative package
# pattern. `classifyGoTest` writes the same word `package` for a go test red
# whose members are IMPORT paths and test names, and existence refuses those.
#
# This is an ALLOWLIST over an OPEN namespace, and the direction is the point.
# `kind` arrives as JSON a producer wrote, so the values this gate can meet are
# not the values anybody enumerated here. A kind nobody listed names no path, its
# group is unattributable, and its red is CHARGED to the session committing.
# A denylist made each new kind attributable by DEFAULT: `unparsed`
# (`unparsedGroup`) landed in exactly that hole, and only the absence of a
# checkout entry called `unparsed-group` kept its red from being dropped as
# somebody else's. The empty answer must never be the valid-looking one
# (ai/rules/evidence.md), so the kind a producer adds tomorrow charges its red
# until this set is taught to read it.
#
# One transition rides on that: an artifact written by a runner OLDER than the
# `lint` kind carries the linter name (`revive`, `typecheck`) instead, so its
# lint red is charged rather than attributed until the next verify run rewrites
# the artifact. Charging more than it must is the direction this gate is allowed
# to be wrong in.
PATH_BEARING_GROUP_KINDS = frozenset({"files", "lint", "package"})


def related_repo_path(repo: Path, related: str) -> str | None:
    """The repo path a `related` member names, or None when it names no path.

    `failureGroup.Related` (scripts/status/verify_run.go) is a UNION, and which
    member you hold is not visible in the string: `classifyLint` records a .go
    FILE, `classifyVet` a package PATTERN (`./scripts/evidence/...`),
    `classifyWiringDocs` a check NAME, `classifyFunctional` a suite name,
    `classifyExabgp` test names, and `genericGroup` the stage's own name. `wiring`
    and `Makefile` are both bare words, so shape decides nothing. Which arm of the
    union it is comes from the group's `kind`, and `group_related_paths` reads
    that BEFORE calling this: this function is reached only for a kind that
    ALLOWS a path, never for one that merely fails to forbid it. What is left for existence to answer is whether the
    checkout holds a path-bearing member: a name no file answers to is not
    attribution evidence, and the caller charges the debt for it rather than
    reading a path the checkout no longer holds as somebody else's.

    `.` is refused for the same reason from the other side. A vet pattern of
    `./...` reduces to it, and it names the whole tree rather than a path the
    commit can be compared against.
    """
    text = related.strip().removesuffix("/...").removeprefix("./")
    if not text or text == "." or text.startswith("/"):
        return None
    if any(part in ("", ".", "..") for part in text.split("/")):
        return None
    if not (repo / text).exists():
        return None
    return text


def group_related_paths(repo: Path, group: dict) -> list[str]:
    """Every repo path this failure group names. Empty when it names none.

    The group's `kind` decides whether its members can name a path at all, and
    existence decides nothing on its own. `classifyWiringDocs` builds a member
    out of `([A-Za-z0-9_-]+) failed` over a log carrying the delegated targets'
    stdout, so a subcheck can be called `docs`, `test`, `api`, `build` or
    `Makefile` -- each of them also a repo entry. Read as attribution evidence,
    such a name leaves nothing blind, drops the whole gate into `foreign`, and
    the red goes uncharged.

    A kind outside `PATH_BEARING_GROUP_KINDS` names nothing here, and that covers
    three cases with one answer. A kind the list REFUSES: a subcheck name, a
    stage name, a functional suite, an ExaBGP test name. A kind nobody has SEEN:
    the artifact is producer JSON, so a classifier written after this gate can
    put any word here, and an unknown kind must not become attribution evidence
    by default. NO kind at all: every group `writeFailureIndex` writes carries
    one, so its absence says the artifact is not that index. Charging a red
    nobody attributed is the safe direction in all three.
    """
    if str(group.get("kind") or "") not in PATH_BEARING_GROUP_KINDS:
        return []
    found: list[str] = []
    for related in group.get("related") or []:
        if not isinstance(related, str):
            continue
        path = related_repo_path(repo, related)
        if path:
            found.append(path)
    return found


def related_in_commit(related: str, paths: tuple[str, ...]) -> bool:
    """True when `related` and a committed path name overlapping trees.

    `manifest_scoped` (scripts/dev/verify-status.sh) already fixes one direction:
    a directory argument scopes to everything under it. The other direction
    exists because `Related` itself can be a directory -- `classifyVet` records
    `./scripts/evidence/...` for every red in that package -- and a commit
    carrying one file inside it owns that red.
    """
    for path in paths:
        if related == path:
            return True
        if related.startswith(path + "/") or path.startswith(related + "/"):
            return True
    return False


def structural_gate_reds(
    repo: Path, paths: tuple[str, ...] | list[str] = ()
) -> GateReds:
    """Structural-gate stages recorded red by the last `make ze-precommit-verify` run.

    Reads tmp/ze-verify-failures.json, which verify_run.go rewrites after EVERY
    run (green or red, unconditionally), so a stale red cannot linger past a
    green verify: a fixed-and-reverified tree clears this. Returns nothing
    charged when the artifact is missing or unreadable -- mirroring
    verify_status(), the gate never invents a red it cannot confirm. Preserves
    stage order.

    `paths` is the commit's file list, the same list `verify_status` scopes the
    freshness question to, and here it ATTRIBUTES each red. A gate whose every
    failure group names files, all of them outside that list, cannot have been
    caused by this commit, so it is not charged (AC-4). One named file inside the
    list charges it, exactly as before (AC-5). A group that names NO path is not
    attributable at all, so it is charged as it always was and `unattributed`
    names it (AC-4b): guessing which files a check name or a suite name covers
    would let a real red go uncharged.

    An empty list keeps the whole-tree meaning and charges every red. Attribution
    narrows a question the caller asked about its own files; with no files named
    there is no question to narrow.
    """
    path = repo / "tmp" / "ze-verify-failures.json"
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return GateReds((), (), ())
    scope = tuple(p for p in paths if p)
    charged: list[str] = []
    unattributed: list[str] = []
    foreign: list[str] = []
    for st in data.get("stages", []) if isinstance(data, dict) else []:
        if not isinstance(st, dict):
            continue
        if st.get("exit-code", 0) == 0 or st.get("stage") not in STRUCTURAL_GATES:
            continue
        stage = st["stage"]
        if not scope:
            charged.append(stage)
            continue
        groups = st.get("groups")
        blind: list[str] = []
        owned = False
        if not isinstance(groups, list) or not groups:
            # A red the classifier produced no group for. It names nothing, so
            # it is charged: `genericGroup` is the usual shape here, and a stage
            # recorded with no groups at all must not read as attributable.
            blind.append(stage)
        for group in groups if isinstance(groups, list) else []:
            if not isinstance(group, dict):
                continue
            named = group_related_paths(repo, group)
            if not named:
                blind.append(str(group.get("group-id") or stage))
                continue
            if any(related_in_commit(name, scope) for name in named):
                owned = True
        if not blind and not owned:
            # Every group named files, and every one is another session's.
            foreign.append(stage)
            continue
        charged.append(stage)
        if not owned:
            unattributed.append(stage + " (" + ", ".join(blind) + ")")
    return GateReds(tuple(charged), tuple(unattributed), tuple(foreign))


# The mode `scripts/status/verify_run.go` records for the 25-stage full gate.
# `ze-precommit-verify-changed` writes its own mode and is deliberately NOT
# accepted here: it runs no full lint, no vet evidence, and no cached full unit
# pass, so it is not the run a Go-carrying commit owes.
FULL_VERIFY_MODE = "ze-precommit-verify"


def go_paths_in(paths: list[str] | tuple[str, ...]) -> list[str]:
    """The paths that make a commit a GO-CARRYING one.

    Same population as the tracked-build rule in ai/rules/git-safety.md: source,
    the module files, and vendored code. `go.mod`/`go.sum` are matched by name so
    a nested module counts too.
    """
    hits: list[str] = []
    for path in paths:
        name = path.rsplit("/", 1)[-1]
        if (
            path.endswith(".go")
            or name in ("go.mod", "go.sum")
            or path.startswith("vendor/")
        ):
            hits.append(path)
    return hits


def full_verify_coverage(repo: Path, go_files: list[str]) -> tuple[str, str]:
    """Did a full `make ze-precommit-verify` run SEE this commit's Go code?

    Returns (state, detail) where state is "covered", "uncovered", or "unknown".

    The question is COVERAGE, never the verdict: in a shared checkout a full run
    is red for somebody else's half-finished edits nearly every time, and the
    owner directive of 2026-08-17 says to take that code as working. So the exit
    code is not read here -- only whether a run of the full mode STARTED after
    the newest Go file in the commit was last written. `generated-at` in
    tmp/ze-verify-full.json is stamped when the run starts, so a file written
    later than it was certainly not compiled by that run.

    It reads tmp/ze-verify-full.json rather than tmp/ze-verify-failures.json
    because only the FULL gate writes the former. Several sessions share this
    checkout, and any one of them running ze-precommit-verify-changed rewrites
    the shared artifact -- which would erase your evidence and refuse a commit
    whose full run really did happen.

    "unknown" never blocks, mirroring verify_status(): a checkout with no verify
    runner (an isolated test repo, a minimal clone) cannot answer the question,
    and a gate that invents an answer it cannot confirm is worse than no gate.
    """
    if not (repo / "scripts" / "status" / "verify_run.go").exists():
        return "unknown", "no verify runner in this checkout"
    index = repo / "tmp" / "ze-verify-full.json"
    try:
        data = json.loads(index.read_text(encoding="utf-8"))
    except OSError:
        return (
            "uncovered",
            "no full ze-precommit-verify recorded (tmp/ze-verify-full.json is missing)",
        )
    except ValueError:
        return "uncovered", "tmp/ze-verify-full.json is unreadable"
    if not isinstance(data, dict):
        return "uncovered", "tmp/ze-verify-full.json is unreadable"
    mode = data.get("mode") or "unknown"
    if mode != FULL_VERIFY_MODE:
        return (
            "uncovered",
            f"the last recorded run was `{mode}`, not the full `{FULL_VERIFY_MODE}`",
        )
    stamp = data.get("generated-at") or ""
    try:
        started = datetime.datetime.fromisoformat(str(stamp).replace("Z", "+00:00"))
    except ValueError:
        return "uncovered", f"the recorded run has no readable start time ({stamp!r})"
    if started.tzinfo is None:
        started = started.replace(tzinfo=datetime.timezone.utc)
    newest, newest_at = "", None
    for path in go_files:
        try:
            mtime = (repo / path).stat().st_mtime
        except OSError:
            continue  # a --remove path, or a file already gone: nothing to cover
        edited = datetime.datetime.fromtimestamp(mtime, datetime.timezone.utc)
        if newest_at is None or edited > newest_at:
            newest, newest_at = path, edited
    if newest_at is None:
        return (
            "covered",
            f"full run at {stamp} (no Go file in the commit is still on disk)",
        )
    if newest_at > started:
        return "uncovered", (
            f"{newest} was written at {newest_at.isoformat()}, after the full run started at {stamp}"
        )
    return "covered", f"full run started {stamp}, after every Go file in this commit"


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
    package_map.py and docs_to_code.py), and it is a third of the archive --
    ~138MB of extraction becomes ~98MB. `dest` MUST be empty -- `tar -x`
    overwrites archived paths but never removes extras, so anything already
    there survives into the view and can make an incoherent index look coherent.
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
        # said -- the "run make ze-generated-files-update" advice cannot fix a crash.
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

    Deliberately in the PRODUCER, so whatever next reaches the code that creates
    these directories also collects them, with no harness required. No session
    hook could do it anyway: nothing under tmp/session/ is ever swept, and these
    trees are not under it -- they sit at the tmp/ root, which only the
    operator's own `make ze-scratch-clean` reaps.

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
        files absent from HEAD. A never-committed summary reached HEAD's committed
        index this way.
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
    # existence (scripts/dev/package_map.py build()): a new `internal/x/thing.go` <!-- doc-links: ignore (example path in a comment, deliberately absent) -->
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
            "  Run `make ze-generated-files-update` (or `make ze-discovery-index-update`) and include the\n"
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
            + "\n  Run `make ze-generated-files-update` (or `make ze-discovery-index-update`) and add them: "
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
# ai/rules/planning.md, ai/rules/completion.md.
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
# it: the rule corpus (ai/rules/planning.md, completion.md, planning.md,
# planning.md, ...). A bare "future work" / "out of scope" phrase there is the subject
# under discussion, not an act of parking work, so it is exempt from the
# added-prose scan. Specs (plan/spec-*.md) and code stay in scope -- that is where
# a real deferral gets written and must be homed. See ai/rules/planning.md,
# ai/rules/repo-maintenance.md, plan/learned/HOOK-FRICTION.md.
# vendor/ is third-party source. A TODO in a dependency is its author's note,
# not a Ze deferral, and no plan/deferrals/ shard could sensibly record one.
# Without this, vendoring almost any large dependency blocks the commit.
# github.com/andybalholm/brotli carries a "TODO: Postpone decision" comment,
# which stopped the commit that vendored templ.
# Where verification debt is recorded: one gate a commit owed and did not run.
# The writer and the full rationale are `record_debt` and the block above it.
VERIFICATION_DEBT_DIR = "plan/verification-debt"

# plan/verification-debt/ is exempt for a different reason from the rest: its
# rows ARE owed work, written in the one place that records it, and they carry a
# free-text reason an author might phrase as "deferred to". Demanding a
# plan/deferrals/ shard alongside would home the same obligation twice, in a
# ledger that answers a different question. Debt rows are enforced by their own
# gate -- `create --push` refuses while one is open -- not by this scan.
DEFERRAL_SCAN_EXEMPT_DIRS = (
    "ai/rules/",
    ".claude/rules/",
    "vendor/",
    VERIFICATION_DEBT_DIR + "/",
)


DEFERRAL_NO_DESTINATION_NEEDED = frozenset({"cancelled", "user-approved-drop"})

# A destination names a markdown file: a full `plan/...md` path (which may be
# nested, e.g. plan/journal/registry-contamination.md) or a bare `spec-x.md`
# resolved against plan/. The plan/ alternative comes first so the longest form
# wins; the lookbehind stops a match starting mid-path or mid-word.
DEFERRAL_DEST_PATH_RE = re.compile(r"(?<![\w/-])(plan/[\w./-]+\.md|[\w.-]+\.md)")

# Statuses that mean the row is CLOSED and needs no destination. Everything else
# is live and gets checked -- including a status word nobody has invented yet.
#
# This is a denylist of terminal states, not an allowlist of live ones, and the
# direction is the whole point (ai/rules/evidence.md). The gate used to
# test `status == "open"`, so `deferred` -- the word ai/rules/planning.md
# itself uses for this state -- was never looked at, and four rows written on
# 2026-07-16 named no home and sailed through. An allowlist re-runs that bug the
# next time the vocabulary drifts; a terminal denylist fails closed, because an
# unrecognised status is treated as live and checked.
#
# The deferral shards' contract (ai/rules/planning.md): "A row lives here
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

    ai/rules/planning.md: a live deferral ALWAYS names a destination
    spec that exists on disk. Only a terminal Status may name no file. Prose
    ("later", "future work") is a deletion with a polite name, and a path to a
    spec nobody created loses the work just as completely -- both lose the work,
    so both are rejected.

    Both spellings of a destination are accepted, `plan/spec-x.md` and a bare <!-- doc-links: ignore (example destination spelling, not a real spec) -->
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
    (ai/rules/planning.md, ai/rules/git-safety.md). The aggregate is a
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


# `git show HEAD:<path>` stderr phrases that each mean "nothing committed is at
# that path", so removing it destroys no committed row. Matched under LC_ALL=C.
# Everything ELSE is a read the gate could not make, and unknown is reported
# rather than waved through (ai/rules/evidence.md). Verified against
# git 2.43.0; a wording this list misses costs a false BLOCKER naming the exit
# code and stderr, which is visible and recoverable -- a silent pass is not.
_GIT_SHOW_NOTHING_COMMITTED = (
    "does not exist in",  # tracked at HEAD? no -- absent from the tree
    "invalid object name",  # HEAD is unborn: nothing is committed at all
    "exists on disk, but not in",  # added to the index this commit, not in HEAD
)


def _live_rows_added_elsewhere(repo: Path, add_paths: tuple[str, ...]) -> set[str]:
    """Live-row renderings from every deferral shard this commit ADDS.

    Compared against the rows a removal would destroy, so a shard being RENAMED
    (removed here, added there, same rows) is not read as a deletion. The
    comparison is on the row's rendered identity, not on the file, because a
    rename may also re-shard: two rows can move to two different files.
    """
    kept: set[str] = set()
    for rel in add_paths:
        if not rel.startswith("plan/deferrals/") or not rel.endswith(".md"):
            continue
        path = repo / rel
        if not path.is_file():
            continue
        for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
            cells = _deferral_row_cells(line)
            if cells is None or cells == ["MALFORMED"]:
                continue
            if cells[5].lower() in DEFERRAL_TERMINAL_STATUSES:
                continue
            kept.add(f"      [{cells[5]}] {cells[2][:70]} -> {cells[4][:50]}")
    return kept


def deferral_shard_removal_problems(
    repo: Path, add_paths: tuple[str, ...], remove_paths: tuple[str, ...]
) -> list[str]:
    """A commit that `git rm`s a deferral shard still holding a live row (BLOCK).

    Spec closure removes the closing spec's own shard, and that is correct only
    when every row in it is terminal. A shard whose rows are homed at OTHER specs
    outlives its source spec: the row's home is its Destination cell, and the
    shard is only where the row is written down (ai/rules/planning.md,
    "A shard that still holds a live row SURVIVES its source spec"). Measured on
    2026-08-03: 39 shards were in that state, holding 68 live rows (counted by
    scripts/dev/deferral_orphans.py, which is the citation this number carries so
    the next reader re-runs it instead of re-deriving it by hand).

    This gate BLOCKS rather than warns, and it is the one deferral check that
    must. Every other signal over these rows is a fold across the plan/deferrals/
    DIRECTORY -- deferral_unassigned_problems above, the session-end banner --
    so deleting a live-bearing shard does not raise their count, it lowers it.
    The action the rule forbids is the action that silences every observer of the
    rows it destroys, which is the fail-open shape ai/rules/evidence.md
    exists to refuse. Prose telling a human to read the Status column first
    cannot be the only control over a delete that hides its own evidence.

    Read from HEAD, not the working tree: the commit deletes HEAD's version, and
    the working-tree copy may already be gone by other means. A shard this cannot
    read at HEAD is new in this commit and cannot be destroying committed rows.

    The `plan/deferrals/` prefix test assumes normalized repo-relative paths, and
    that holds because every path reaching a gate has been through normalize_path
    above: `os.path.normpath` strips a `./` prefix, and an absolute path is
    resolved and made relative. Nested shards (`plan/deferrals/sub/x.md`) are in <!-- doc-links: ignore (example shard path, not a real shard) -->
    scope, matching deferral_shard_paths, whose glob is recursive for the same
    reason.
    """
    offenders: list[str] = []
    for rel in remove_paths:
        if not rel.startswith("plan/deferrals/") or not rel.endswith(".md"):
            continue
        shown = subprocess.run(  # noqa: S603
            ["git", "show", f"HEAD:{rel}"],  # noqa: S607
            cwd=repo,
            capture_output=True,
            text=True,
            check=False,
            # LC_ALL=C so the benign stderr phrases below stay in English.
            # git translates its diagnostics when NLS is on, and a translated
            # "does not exist in" would be classified as unknown and reported --
            # a false BLOCKER on a legitimate closure, fired by the operator's
            # locale rather than by anything in the commit.
            env={**os.environ, "LC_ALL": "C"},
        )
        if shown.returncode != 0:
            # _GIT_SHOW_NOTHING_COMMITTED means there is provably nothing
            # committed to destroy. Everything else (a corrupt object, git
            # missing, a permission failure) means the gate cannot SEE the rows,
            # and unknown is not innocent (ai/rules/evidence.md).
            err = shown.stderr
            if any(phrase in err for phrase in _GIT_SHOW_NOTHING_COMMITTED):
                continue
            offenders.append(
                f"  {rel}: cannot be read at HEAD, so its rows cannot be"
                f" checked\n      git show exited {shown.returncode}:"
                f" {shown.stderr.strip()[:160]}"
            )
            continue
        live: list[str] = []
        for line in shown.stdout.splitlines():
            cells = _deferral_row_cells(line)
            if cells is None or cells == ["MALFORMED"]:
                continue
            if cells[5].lower() in DEFERRAL_TERMINAL_STATUSES:
                continue
            live.append(f"      [{cells[5]}] {cells[2][:70]} -> {cells[4][:50]}")
        if live:
            kept = _live_rows_added_elsewhere(repo, add_paths)
            lost = [row for row in live if row not in kept]
            if not lost:
                # Every live row reappears in a shard this same commit adds, so
                # this is a MOVE, not a deletion. Renaming a misnamed shard has
                # to stay possible: plan/deferrals/spec-fixit-rs-community-strip-arity.md
                # carried a doubled `spec-` prefix, which hid it from every
                # gate that pairs a shard with plan/spec-<stem>.md. A gate that
                # forbade the rename would protect the rows by freezing the
                # defect that made them unfindable.
                continue
            offenders.append(f"  {rel} ({len(lost)} live):\n" + "\n".join(lost))
    if not offenders:
        return []
    return [
        "removing a deferral shard that still holds live rows:\n"
        + "\n".join(offenders)
        + "\n  A shard is deleted at closure ONLY when every row in it is terminal"
        "\n  (ai/rules/planning.md). Each row above is live work homed at"
        "\n  another spec; deleting the shard deletes the record, and no other gate"
        "\n  would notice because every one of them folds over this directory."
        "\n  Resolve each row (done / cancelled / resolved), or drop the"
        " --remove for this shard: a shard outliving its source spec is the"
        " correct end state, not mess."
    ]


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
    used to block an otherwise-valid commit (ai/rules/planning.md). The
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
            + "\n  Homing is still required by ai/rules/planning.md -- each"
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


def _prospective_index_diff(
    repo: Path,
    add_paths: tuple[str, ...],
    remove_paths: tuple[str, ...],
    *diff_args: str,
) -> tuple[bytes, str | None]:
    """The prospective commit's raw diff from a throwaway Git index."""
    (repo / "tmp").mkdir(exist_ok=True)
    fd, index = tempfile.mkstemp(prefix="ze-commit-index-", dir=str(repo / "tmp"))
    os.close(fd)
    os.unlink(index)
    env = dict(os.environ, GIT_INDEX_FILE=index)

    def git(*args: str) -> subprocess.CompletedProcess[bytes]:
        return subprocess.run(
            ("git", "-C", str(repo), *args),
            check=False,
            capture_output=True,
            env=env,
        )

    def failed(result: subprocess.CompletedProcess[bytes], operation: str):
        if result.returncode == 0:
            return None
        detail = result.stderr.decode("utf-8", errors="replace").strip()
        return f"{operation} failed for the prospective commit: {detail}"

    try:
        has_head = git("rev-parse", "--verify", "-q", "HEAD").returncode == 0
        if has_head:
            result = git("read-tree", "HEAD")
            error = failed(result, "git read-tree HEAD")
            if error:
                return b"", error
        if add_paths:
            result = git("add", "--", *add_paths)
            error = failed(result, "git add")
            if error:
                return b"", error
        for path in remove_paths:
            result = git("rm", "--cached", "-q", "--", path)
            error = failed(result, f"git rm --cached -- {path}")
            if error:
                return b"", error
        result = git("diff", "--cached", *diff_args)
        error = failed(result, "git diff --cached")
        return result.stdout if error is None else b"", error
    finally:
        for temporary in (index, index + ".lock"):
            try:
                os.unlink(temporary)
            except OSError:
                pass


def _prospective_line_changes(
    repo: Path,
    add_paths: tuple[str, ...],
    remove_paths: tuple[str, ...],
    *extra_args: str,
) -> list[tuple[str, str, str]]:
    """The (path, '+'|'-', text) triples this commit would record.

    Computed in a THROWAWAY index (GIT_INDEX_FILE) so the real staging area is
    never touched: read HEAD's tree, apply this commit's add/remove set, then
    diff --cached -U0. That is exactly what the generated `git add ... ;
    git commit` will record, and it captures brand-new files too.

    Header parsing is state-tracked rather than prefix-matched. `--- ` and `+++ `
    are read as headers only between a `diff --git` line and the first content
    line, so a REMOVED source line whose own text starts with `-- ` (which reaches
    the diff as `--- ...`) is content, not a path header. For a deleted file the
    `+++` side is `/dev/null`, so the path falls back to the `---` side.
    """
    raw, error = _prospective_index_diff(
        repo,
        add_paths,
        remove_paths,
        "--no-color",
        "-U0",
        *extra_args,
    )
    if error:
        return []
    diff = raw.decode("utf-8", errors="replace")
    result: list[tuple[str, str, str]] = []
    old = new = ""
    in_header = False
    for line in diff.splitlines():
        if line.startswith("diff --git "):
            old = new = ""
            in_header = True
            continue
        if in_header and line.startswith("--- "):
            hdr = line[4:]  # "a/<path>", or "/dev/null" for an added file
            old = hdr[2:] if hdr.startswith("a/") else ""
            continue
        if in_header and line.startswith("+++ "):
            hdr = line[4:]  # "b/<path>", or "/dev/null" for a deleted file
            new = hdr[2:] if hdr.startswith("b/") else ""
            in_header = False
            continue
        if line.startswith("+"):
            result.append((new or old, "+", line[1:]))
        elif line.startswith("-"):
            result.append((new or old, "-", line[1:]))
    return result


def _common_suffix_components(old_path: str, new_path: str) -> int:
    """Number of equal trailing path components."""
    count = 0
    for old_part, new_part in zip(
        reversed(Path(old_path).parts), reversed(Path(new_path).parts)
    ):
        if old_part != new_part:
            break
        count += 1
    return count


def _unique_suffix_pairing(
    old_paths: list[str], new_paths: list[str]
) -> tuple[tuple[tuple[str, str], ...], str | None]:
    """Unique maximum pairing by common trailing path components."""

    def component(path: str, depth: int):
        parts = Path(path).parts
        return parts[-depth - 1] if depth < len(parts) else None

    def pair_at_depth(olds: list[str], news: list[str], depth: int):
        old_groups = {}
        new_groups = {}
        for path in olds:
            old_groups.setdefault(component(path, depth), []).append(path)
        for path in news:
            new_groups.setdefault(component(path, depth), []).append(path)

        pairs = []
        old_left = []
        new_left = []
        unique = True
        shared = (set(old_groups) & set(new_groups)) - {None}
        for key in sorted(shared):
            child_pairs, child_old, child_new, child_unique = pair_at_depth(
                old_groups[key], new_groups[key], depth + 1
            )
            pairs.extend(child_pairs)
            old_left.extend(child_old)
            new_left.extend(child_new)
            unique = unique and child_unique
        for key, paths in old_groups.items():
            if key not in shared:
                old_left.extend(paths)
        for key, paths in new_groups.items():
            if key not in shared:
                new_left.extend(paths)

        old_left.sort()
        new_left.sort()
        matched = min(len(old_left), len(new_left)) if depth >= 2 else 0
        if matched and (len(old_left) != 1 or len(new_left) != 1):
            unique = False
        pairs.extend(zip(old_left[:matched], new_left[:matched]))
        return pairs, old_left[matched:], new_left[matched:], unique

    pairs, _old_left, _new_left, unique = pair_at_depth(
        sorted(old_paths), sorted(new_paths), 0
    )
    if not unique:
        return (), "the maximum common-suffix pairing is not unique"
    return tuple(sorted(pairs)), None


def _prospective_rename_pairs(
    repo: Path,
    add_paths: tuple[str, ...],
    remove_paths: tuple[str, ...],
) -> tuple[tuple[check_weakened_tests.RenamePair, ...], list[str]]:
    """Git rename pairs and scores for the prospective commit's exact paths.

    Git's default 50% threshold owns ordinary renames. A lower-score pair must
    share the basename and one parent component. Its basename group is accepted
    only when the maximum-cardinality, maximum-common-suffix subset is unique
    and agrees with Git's scored pairs.
    """
    raw, error = _prospective_index_diff(
        repo,
        add_paths,
        remove_paths,
        "--name-status",
        "-z",
        "--find-renames=1%",
        "-l0",
        "--diff-filter=R",
    )
    if error:
        return (), [f"check could not run: {error}, so no rename was compared"]

    fields = raw.split(b"\0")
    if fields and fields[-1] == b"":
        fields.pop()
    pairs = []
    index = 0
    while index < len(fields):
        status = fields[index].decode("ascii", errors="replace")
        if (
            not status.startswith("R")
            or not status[1:].isdigit()
            or index + 2 >= len(fields)
        ):
            return (), [
                "check could not run: Git returned malformed or ambiguous rename "
                "status, so no rename was compared"
            ]
        pairs.append(
            check_weakened_tests.RenamePair(
                os.fsdecode(fields[index + 1]),
                os.fsdecode(fields[index + 2]),
                int(status[1:]),
            )
        )
        index += 3
    accepted = [pair for pair in pairs if pair.score >= 50]
    used_old = {pair.old_path for pair in accepted}
    used_new = {pair.new_path for pair in accepted}
    low_by_basename = {}
    for pair in pairs:
        if pair.score >= 50:
            continue
        basename = os.path.basename(pair.old_path)
        if basename != os.path.basename(pair.new_path):
            continue
        if _common_suffix_components(pair.old_path, pair.new_path) < 2:
            continue
        low_by_basename.setdefault(basename, []).append(pair)

    for basename, scored_pairs in low_by_basename.items():
        old_candidates = [
            path
            for path in remove_paths
            if path not in used_old and os.path.basename(path) == basename
        ]
        new_candidates = [
            path
            for path in add_paths
            if path not in used_new and os.path.basename(path) == basename
        ]
        optimum, error = _unique_suffix_pairing(old_candidates, new_candidates)
        if error:
            return (), [
                "check could not run: low-similarity rename pairing is ambiguous "
                f"for basename {basename!r}: {error}"
            ]
        scored = {(pair.old_path, pair.new_path): pair for pair in scored_pairs}
        if set(optimum) != set(scored):
            return (), [
                "check could not run: Git's low-similarity rename pairs conflict "
                f"with the unique common-suffix pairing for basename {basename!r}"
            ]
        for paths in optimum:
            accepted.append(scored[paths])
    return tuple(accepted), []


def _prospective_added_lines(
    repo: Path, add_paths: tuple[str, ...], remove_paths: tuple[str, ...]
) -> list[tuple[str, str]]:
    """The (path, '+' line) pairs this commit would introduce (added/modified files).

    Each returned line keeps its leading '+' and is paired with the repo-relative
    path of the file it belongs to, so a caller can path-scope the scan.
    """
    return [
        (path, "+" + text)
        for path, sign, text in _prospective_line_changes(
            repo, add_paths, remove_paths, "--diff-filter=AM"
        )
        if sign == "+" and text != ""
    ]


def _deferral_prose(line: str) -> str:
    """The prose part of a '+' diff line, with quoted and backticked spans blanked.

    A phrase that exists only as a quoted or backticked token -- a
    `DEFERRAL_PATTERNS` entry, a test fixture string, a doc example -- is data,
    not a statement of deferring work, so it is removed before matching. A phrase
    in bare markdown text or a bare code comment survives and is caught.

    A backslash-escaped quote is blanked FIRST. It is what a JSON string holding
    a quotation looks like, and left in place it closes the span that encloses
    it: the rest of that string then reads as bare prose, which is how a
    generated data file quoting a code comment tripped this gate.
    """
    text = line[1:] if line.startswith("+") else line
    text = text.replace('\\"', " ")
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
    # A repository with no plan/ tree cannot record a deferral, so this gate has
    # no satisfying action there and refuses every commit it examines. The
    # published website is such a repository: it is generated, nobody parks work
    # in it, and its pages legitimately quote the phrases this scan looks for --
    # an RFC status page describing what an RFC leaves `out of scope` is prose
    # about a document, not somebody deferring their own work.
    if not (repo / "plan").is_dir():
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
        " committing (ai/rules/planning.md)."
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
        " (ai/rules/completion.md)."
    ]


def feature_gate_tags(repo: Path) -> list[str]:
    """Sorted ze_<feature> build tags from feature-gates.txt, the single source of
    truth (ai/rules/plugins.md). Derived, not hardcoded, so a new
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

    Runs under the SAME build tags as `make ze-doc-drift-check` (Makefile GO_RUN =
    `go run -tags '$(GO_TEST_TAGS)'`, GO_TEST_TAGS = `ze_core $(ZE_FEATURES)`).
    A bare `go run` here compiles out every feature-gated package, so the family
    registry holds only the four always-on families and EVERY address-family
    claim in docs/comparison.md and docs/DESIGN.md is reported as drift -- 11
    fabricated warnings on a tree whose `make ze-doc-verify` is green. That is the
    trap ai/rules/commands.md names: dropping the feature tags fakes reds.
    """
    if not (repo / "scripts" / "docvalid" / "doc_drift.go").exists():
        return []
    tags = " ".join(["ze_core", *feature_gate_tags(repo)])
    env = os.environ.copy()
    env["CGO_ENABLED"] = "0"
    try:
        res = subprocess.run(
            ("go", "run", "-tags", tags, "scripts/docvalid/doc_drift.go"),
            cwd=str(repo),
            capture_output=True,
            text=True,
            timeout=120,
            env=env,
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
    """REPORT, never block, when a commit's own prose grew an ASD-STE100 habit.

    Simplified Technical English (ai/rules/writing.md) is a
    GUIDELINE. It exists to make text clearer for a reader, and it is not a law.
    Owner directive, 2026-07-31: a prose gate that refuses a commit makes an
    author spend edits on wording that changes no meaning, which is the overhead
    the guideline exists to remove. So this prints its findings and returns [].

    Findings are still worth printing. This is the only place where the six
    habits can be attributed to ONE author: several sessions share this checkout,
    so a tree-wide report names a colleague's in-flight sentences. Each file is
    compared against its own HEAD version, so only your new sentences count.
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
        print(
            f"warning: ste gate could not run ({exc}); prose is UNCHECKED",
            file=sys.stderr,
        )
        return []
    if res.returncode == STE_HABIT_GREW:
        # Advisory: print and let the commit through. Read the findings, fix what
        # makes the text clearer, and ignore what does not.
        print((res.stdout + res.stderr).rstrip(), file=sys.stderr)
        print(
            "note: ASD-STE100 is a guideline, not a gate. Apply a finding when it "
            "helps the reader; never rewrite a sentence only to satisfy a count.",
            file=sys.stderr,
        )
        return []
    if res.returncode != 0:
        print(
            f"warning: ste gate could not judge (exit {res.returncode}); "
            f"prose is UNCHECKED: {(res.stdout + res.stderr).strip()[:400]}",
            file=sys.stderr,
        )
    return []


def spec_stem(claimed: str) -> str:
    """The bare stem of a claimed spec basename: `spec-foo.md` -> `foo`.

    One spelling of the transform, because two gates key on it: the closure
    artifact `spec_audit_problems` looks for, and the review artifact
    `spec_closure_stem` names.
    """
    return re.sub(r"\.md$", "", re.sub(r"^spec-", "", claimed))


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
    repo: Path,
    add_paths: tuple[str, ...],
    claimed: str,
    remove_paths: tuple[str, ...] = (),
) -> list[str]:
    """Block a spec-closure commit whose spec has an unfilled verification.

    Ported from the never-wired pre-commit-spec-audit.sh, keyed to the LIVE
    per-session marker (its old tmp/session/selected-spec substrate was removed
    in 276d72c99). Fires ONLY when this commit adds the claimed spec's own
    CLOSURE ARTIFACT -- i.e. the closure commit of the claiming session -- so it
    never blocks unrelated commits or other sessions (the historic umbrella-spec
    false-positive mode). No spec claimed -> no gate.

    Two artifacts spell a closure, and both are read here. The learned-summary
    path (`plan/learned/NNN-<stem>.md`) is the historic one. The live one since
    `ai/skills/ze-close.md` step 6a is a NEW `plan/journal/<class>.md` row whose
    Spec cell names the stem, and reading only the first left this gate unable
    to fire on ANY closure: `plan/learned/` is gone, so the pattern below can no
    longer match. `_journal_added_spec_stems` is what answers "which row did
    this commit add", and it is the reader `spec_closure_stem` uses too.

    The two call sites read it differently, and that is deliberate rather than a
    drift. `spec_closure_stem` filters the stems through `_spec_closed_earlier`,
    because it must pick ONE stem and an already-closed one is the wrong pick.
    This gate needs no filter: it asks whether the CLAIMED spec is among the
    added stems, and the `if not claimed` guard above plus the claimed spec's own
    file being on disk already exclude a spec that closed earlier. Adding the
    filter here would cost a subprocess per stem and change no answer.
    """
    if not claimed:
        return []
    stem = spec_stem(claimed)
    pattern = re.compile(rf"^plan/learned/[0-9]+-{re.escape(stem)}\.md$")
    closes = any(pattern.match(p) for p in add_paths) or stem in (
        _journal_added_spec_stems(repo, add_paths, remove_paths)
    )
    if not closes:
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
            " commit adds its closure artifact.\n"
            "  Add and fill it from plan/TEMPLATE-CLOSURE.md before closing"
            " (ai/rules/planning.md)."
        ]
    if gaps:
        return [
            f"spec {claimed} '## Pre-Commit Verification' has no evidence rows in:"
            f" {', '.join(gaps)}.\n"
            "  This commit adds its closure artifact. Each table is a"
            " separate obligation: re-verify independently and paste the evidence"
            " for EVERY one (plan/TEMPLATE-CLOSURE.md, ai/rules/planning.md)."
        ]
    return []


def weakened_problems(
    repo: Path, add_paths: tuple[str, ...], remove_paths: tuple[str, ...]
) -> list[str]:
    """BLOCK a commit whose test weakenings test/weakened.md does not cover.

    The judgement is delegated whole to scripts/dev/check_weakened_tests.py: what
    a diff weakens, what name the enclosing test carries, and how a row pairs
    with it. This function contributes the commit's own paths and Git's rename
    pairs from the same prospective add/remove population.

    Those paths are the reason the gate can BLOCK. Several sessions share this
    checkout, so a check that read the working tree at large would refuse a
    commit for a colleague's in-flight weakening. That false positive is what
    demoted deferral_unassigned_problems to warn (ai/rules/git-safety.md).
    Keying on the --file and --remove lists makes the BLOCK tier safe by
    construction (owner decision 5, plan/spec-weakened-per-commit.md).

    The test paths are also what keeps the cost off every other commit: the
    delegate imports the hook to borrow its detector, and a commit that names no
    test file needs no detector and gets no import.

    One question the delegate cannot answer is added here, because it is about
    the COMMIT and not about the change set: test/weakened.md must be one of the
    committed paths. A row that stays in the working tree records nothing, and
    the mechanism exists so the reason sits in history beside the weakening it
    accepts (owner, 2026-08-16; AC-10). The delegate reads the ledger only when
    this commit carries it. Otherwise, this function prints suggestions from the
    commit's findings without reading mutable worktree rows.
    """
    tests = tuple(p for p in add_paths if check_weakened_tests.is_test_path(p))
    removed = tuple(p for p in remove_paths if check_weakened_tests.is_test_path(p))
    if not tests and not removed:
        return []
    rename_pairs = ()
    if tests and removed:
        prospective_pairs, errors = _prospective_rename_pairs(
            repo, add_paths, remove_paths
        )
        if errors:
            return errors
        rename_pairs = tuple(
            pair
            for pair in prospective_pairs
            if check_weakened_tests.is_test_path(pair.old_path)
            and check_weakened_tests.is_test_path(pair.new_path)
        )
    weakened, errors = check_weakened_tests.weakened_tests(
        str(repo), tests, removed=removed, rename_pairs=rename_pairs
    )
    if errors:
        return errors  # the comparison did not happen, so nothing is accepted
    if not weakened:
        return []  # AC-5: no weakening, so no row is owed and none is read
    carries_ledger = check_weakened_tests.WEAKENED_PATH in add_paths
    if carries_ledger:
        problems = check_weakened_tests.weakened_problems(
            str(repo), tests, removed=removed, rename_pairs=rename_pairs
        )
    else:
        problems = check_weakened_tests.unmatched_problems(
            [], weakened, ledger_carried=False
        )
    if not carries_ledger:
        problems.append(
            f"this commit weakens {len(weakened)} test(s) and does not carry "
            f"{check_weakened_tests.WEAKENED_PATH}. The row is in the working "
            f"tree only, so git history would hold the weakening with no reason "
            f"beside it. Name the file too:\n"
            f"    --file {check_weakened_tests.WEAKENED_PATH}"
        )
    return problems


# One spelling of the ledger's path, shared with the edit-time hook through the
# module both gates read it with.
RFC_CHANGED_PATH = check_weakened_tests.RFC_CHANGED_PATH

_RFC_CANNOT_RUN = (
    "the RFC-tagged-change gate could not run: _rfc_tagged_change_err is not "
    "importable from .claude/hooks/pretool-writeedit.py, so no tagged change "
    "was judged. Repair the hook rather than commit past this."
)


@dataclass(frozen=True)
class RfcChanged:
    """One RFC-tagged test a commit changes.

    `name` and `package` are the two fields `check_weakened_tests.row_matches`
    reads, so one matcher pairs the rows of both ledgers and neither can invent
    a second spelling of `package.TestName`.
    """

    path: str  # the file that holds it
    package: str  # the directory that holds the file, the row's qualifier
    name: str  # the enclosing Go func, or the file stem when there is none
    tags: tuple[str, ...]  # the requirement ids the change puts at risk
    saw: str  # what the comparison against HEAD found, in the author's words


def _rfc_hook_parts():
    """(detector, tag pattern) from the canonical hook, or None.

    The judgement is borrowed whole, exactly as `weakened_problems` borrows
    `_test_weakening_errs`. `_rfc_tagged_change_err` decides what a behavior
    change to a tagged test is, and it already exempts a reformat, a comment
    edit and a Go import-only edit. A second copy here would let the edit-time
    hook and this gate disagree about one diff, which is the drift
    `scripts/dev/rfc_tagged_scope.py` exists to record.
    """
    module = _writeedit_module()
    if module is None:
        return None
    parts = (
        getattr(module, "_rfc_tagged_change_err", None),
        getattr(module, "_RFC_TAG", None),
    )
    return None if any(part is None for part in parts) else parts


def _rfc_saw(tag_re, old_text, new_text):
    """What the comparison found, said plainly enough for the author to judge it.

    A gate that prints only its demand leaves the author no way to tell a real
    finding from a blind spot. This one has blind spots, and they are listed
    under "What this gate cannot see" in `test/rfc-changed.md`: the comparison
    is textual and cannot follow a call, so an assertion moved into a helper
    reads exactly like one removed.
    """
    if not new_text.strip():
        return (
            "the test is gone from this file under this name. A deletion, a "
            "rename and a move to a sibling file all look like this"
        )
    if set(tag_re.findall(old_text)) - set(tag_re.findall(new_text)):
        return (
            "the `RFC requirement:` tag is no longer on this test, so the "
            "proof behind the compliance claim would leave with this commit"
        )
    return (
        "its text differs from HEAD once comments and whitespace are removed. "
        "The gate compares text and cannot follow a call, so an assertion "
        "moved into a helper reads the same as one removed"
    )


def _rfc_changed_units(path, old, new, parts):
    """[(name, tags, saw)] -- the tagged tests `new` changes, named as the ledger names them.

    `rfc_changed_units` (`scripts/dev/check_weakened_tests.py`) is the shared
    namer, and the edit-time hook calls it with the same detector, so the two
    gates cannot ask for two different rows for one change. What this adds is
    the sentence a person reads: `_rfc_saw` turns the comparison into words the
    author can judge the finding by.
    """
    detector, tag_re = parts
    return [
        (name, tags, _rfc_saw(tag_re, old_text, new_text))
        for name, tags, old_text, new_text in check_weakened_tests.rfc_changed_units(
            path, old, new, detector, tag_re
        )
    ]


def rfc_changed_tests(
    repo: Path, add_paths: tuple[str, ...], remove_paths: tuple[str, ...]
):
    """([RfcChanged], [error]) -- every RFC-tagged test the named paths change.

    The population is the commit, never the tree. Several sessions share this
    checkout, so a gate that read the working tree at large would refuse a
    commit for a colleague's in-flight edit. Keying on the `--file` and
    `--remove` lists is what makes the BLOCK tier safe by construction, and it
    is the same reason `weakened_problems` gives.

    The carrier set is `is_tag_carrier`, not `is_test_path`: an interop
    `check.py` carries RFC evidence and the weakening gate does not judge one.

    A non-empty error list means no comparison happened, and the empty list
    beside it says nothing about the commit.
    """
    scope = check_weakened_tests.rfc_tagged_scope
    paths = [p for p in add_paths if scope.is_tag_carrier(p)]
    removed = [p for p in remove_paths if scope.is_tag_carrier(p)]
    if not paths and not removed:
        return [], []
    parts = _rfc_hook_parts()
    if parts is None:
        return [], [_RFC_CANNOT_RUN]
    tag_re = parts[1]
    out: list[RfcChanged] = []
    errors: list[str] = []
    seen: list[str] = []
    for path in paths + removed:
        if path in seen:
            continue
        seen.append(path)
        old, err = check_weakened_tests._head_text(str(repo), path, "HEAD")
        if err:
            errors.append(f"the RFC-tagged-change gate could not run: {err}")
            continue
        if not tag_re.search(old):
            continue  # nothing was proven here at HEAD, so nothing is at risk
        new = (
            ""
            if path in removed
            else check_weakened_tests._worktree_text(str(repo), path)
        )
        package = os.path.basename(os.path.dirname(path))
        for name, tags, saw in _rfc_changed_units(path, old, new, parts):
            out.append(RfcChanged(path, package, name, tags, saw))
    return out, errors


def _rfc_ids(changed: RfcChanged) -> str:
    """The requirement ids alone, for a message a person reads.

    `_RFC_TAG` has no capture group, so the detector hands back the whole match,
    `RFC requirement: RFC1035-3.1-3`. That is the right thing to compare and the
    wrong thing to print inside a sentence.
    """
    return ", ".join(tag.split(":", 1)[-1].strip() for tag in changed.tags)


def _rfc_row_to_write(changed: RfcChanged, qualify: bool) -> str:
    name = f"{changed.package}.{changed.name}" if qualify else changed.name
    return (
        f"| {name} | <what the owner approved, and why "
        f"{_rfc_ids(changed)} is still proven> |"
    )


def rfc_changed_problems(
    repo: Path, add_paths: tuple[str, ...], remove_paths: tuple[str, ...]
) -> list[str]:
    """[problem] -- why `test/rfc-changed.md` does not cover this commit.

    Every problem BLOCKS. Kept out of `commit_gate_problems` because it carries
    an owner override, and `review_gate_problems` is the precedent for that
    shape: `create` calls it behind the flag so the flag can clear it.

    An in-file `rfc-test-change-approved:` marker records nothing here. It was
    accepted while `.claude/hooks/pretool-writeedit.py` demanded one at edit
    time, because a gate refusing it would have refused every author for obeying
    the other gate. That hook reads this ledger now, so the acceptance is gone
    with it and a marker in a commit is neither an approval nor a notice.
    """
    changed, errors = rfc_changed_tests(repo, add_paths, remove_paths)
    if errors:
        return errors
    if not changed:
        return []  # nothing at risk, so the ledger's content is not read

    problems: list[str] = []
    ledger = repo / RFC_CHANGED_PATH
    rows = []
    if ledger.is_file():
        text = ledger.read_text(encoding="utf-8", errors="replace")
        # ONE table reader serves both ledgers, so a row shape that parses in
        # test/weakened.md parses here. It takes the path it is reading, so its
        # problems name this file.
        rows, row_problems = check_weakened_tests.parse_weakened_file(
            text, RFC_CHANGED_PATH
        )
        problems += row_problems

    claimed: set[int] = set()
    for row in rows:
        hits = [
            i
            for i, c in enumerate(changed)
            if check_weakened_tests.row_matches(row.name, c)
        ]
        if not hits:
            problems.append(
                f"{RFC_CHANGED_PATH}:{row.line} names {row.name}, which this "
                f"commit does not change. A row left over from the last commit "
                f"approves nothing here; delete it."
            )
            continue
        if len(hits) > 1:
            hit = [changed[i] for i in hits]
            where = ", ".join(f"{c.package} ({c.path})" for c in hit)
            problems.append(
                f"{RFC_CHANGED_PATH}:{row.line} names {row.name}, which this "
                f"commit changes in {len(hit)} packages: {where}.\n"
                f"    Write package.TestName, one row each:\n"
                + "\n".join(f"    {_rfc_row_to_write(c, True)}" for c in hit)
            )
        claimed.update(hits)

    for index, changed_test in enumerate(changed):
        if index in claimed:
            continue
        qualify = sum(1 for c in changed if c.name == changed_test.name) > 1
        problems.append(
            f"{changed_test.path} changes the RFC-tagged test "
            f"{changed_test.name} and {RFC_CHANGED_PATH} has no row for it:\n"
            f"    - requirement(s) it proves: {_rfc_ids(changed_test)}\n"
            f"    - what this gate saw: {changed_test.saw}\n"
            f"    The OWNER approves this, never the author: a self-written "
            f"justification is not approval. Ask, then add the row and commit "
            f"the file with the change:\n"
            f"    {_rfc_row_to_write(changed_test, qualify)}\n"
            f"    If the gate is wrong about what it saw, the row is still how "
            f"you say so: name the blind spot, and see "
            f'"What this gate cannot see" in {RFC_CHANGED_PATH}.'
        )

    if claimed and RFC_CHANGED_PATH not in add_paths:
        problems.append(
            f"this commit changes {len(claimed)} RFC-tagged test(s) and does "
            f"not carry {RFC_CHANGED_PATH}. The row is in the working tree "
            f"only, so git history would hold the change with no approval "
            f"beside it. Name the file too:\n"
            f"    --file {RFC_CHANGED_PATH}"
        )
    return problems


# The exemptions c_require_design_ref applies, restated here because this gate
# judges the same population from the other end. Keep the two in step: a file the
# hook waves through and this refuses would make the sanctioned route impossible.
_DESIGN_REF_EXEMPT_BASENAMES = ("register.go", "embed.go", "doc.go")
_GENERATED_HEAD_RE = re.compile(r"Code generated|DO NOT EDIT")


def _design_ref_required(path: str) -> bool:
    """Whether `path` owes a `// Design:` header at all."""
    if not path.endswith(".go"):
        return False
    if path.endswith("_test.go") or path.endswith("_gen.go"):
        return False
    if os.path.basename(path) in _DESIGN_REF_EXEMPT_BASENAMES:
        return False
    # Vendored third-party code has no ze design document to point at.
    return not re.search(r"(^|/)vendor/", path)


def go_design_ref_problems(repo: Path, add_paths: tuple[str, ...]) -> list[str]:
    """Every non-exempt .go file in the commit carries a `// Design:` header.

    This is the COMMIT-TIME half of c_require_design_ref
    (.claude/hooks/pretool-writeedit.py). That check is wired to the matcher
    `Write|Edit|MultiEdit|NotebookEdit` in .claude/settings.json, so a .go file
    written from Bash -- a heredoc, `sed -i`, a python payload -- reaches it
    never. Auto mode tells agents to prefer Bash for file changes, so the bypass
    is the default route rather than an unusual one.

    A pre-tool hook and a commit gate answer different questions, and the second
    is the one that cannot be routed around. The hook asks "is this edit
    allowed", which depends on the tool used. This asks "does the tree this
    commit produces hold the header", which depends on nothing but the file. A
    changed-file set at commit time is a FACT; how each file came to be written
    is not recoverable and does not need to be.

    Scoped to what the tree can prove. c_require_related_refs gates on
    session-state markers, which say what the AUTHOR did and are not properties
    of the commit, so it stays hook-only by construction.

    c_require_test_first was named here too, and that was wrong: it reads
    isfile() and no session state at all, so the tree could always have answered
    its question. The claim went unchecked and the gap stayed open until
    2026-08-19. Its second half is test_coverage_problems, below.
    """
    missing: list[str] = []
    for path in add_paths:
        if not _design_ref_required(path):
            continue
        full = repo / path
        if not full.is_file():
            continue
        try:
            text = full.read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue
        if "// Design:" in text:
            continue
        if _GENERATED_HEAD_RE.search(text[:500]):
            continue
        missing.append(path)
    if not missing:
        return []
    return [
        "these .go files carry no `// Design:` header:\n"
        + "\n".join(f"  {p}" for p in missing)
        + "\n  Format: // Design: <path-to-design-doc> -- <brief description>\n"
        "  Exempt: _test.go, _gen.go, register.go, embed.go, doc.go, vendor/,\n"
        "  and a file whose first 500 bytes say `Code generated` or `DO NOT EDIT`.\n"
        "  This is the commit-time half of c_require_design_ref, which a Bash\n"
        "  write never reaches (ai/rules/go-standards.md)."
    ]


def _test_coverage_required(path: str) -> bool:
    """Whether `path` raises a test obligation for the commit that carries it."""
    if not _design_ref_required(path):
        return False
    # cmd/ is thin wiring with no package of its own to test. The hook exempts
    # it; restated here so a file one gate waves through the other cannot refuse.
    return not re.search(r"(^|/)cmd/", path)


def _is_test_path(path: str) -> bool:
    """A path that proves something: a Go unit test, or a functional test.

    Both kinds count, and either alone clears the gate. ai/rules/testing.md
    requires both and says neither substitutes for the other, but that is a
    property of a FEATURE, not of a commit -- a commit landing the unit tests
    and a later one landing the .ci are both doing the right thing.
    """
    if path.endswith("_test.go"):
        return True
    return path.endswith((".ci", ".et")) and re.search(r"(^|/)test/", path) is not None


def test_coverage_problems(repo: Path, add_paths: tuple[str, ...]) -> list[str]:
    """A commit carrying non-exempt Go carries a test too.

    The commit-time half of c_require_test_first
    (.claude/hooks/pretool-writeedit.py). That check fires on Write of a NEW
    .go file only, so a source file added by Edit -- or written from a Bash
    heredoc, which auto mode tells agents to prefer -- meets no test
    obligation anywhere.

    go_design_ref_problems' docstring is why this gap stayed open: it records
    that c_require_test_first "gates on session-state markers, which say what
    the AUTHOR did", and concludes the check must stay hook-only. That is not
    what the check does. It reads isfile() and nothing else, so the tree a
    commit produces answers the same question, and the reasoning that kept
    design-ref's second half out did not apply to this one.

    BLOCK, armed by the owner on 2026-08-19. It shipped WARN for one session,
    because this checkout is shared and a gate refusing every refactor commit
    holds other sessions' work back.

    `--no-test "<reason>"` is what answers it, and it is deliberately NOT a
    DEBT_FLAGS entry. Every flag there names a gate a later run can re-judge,
    which is what `debt-clear` does. "This commit carried no test" is not
    re-judgeable once the commit exists, so a row for it could never be
    cleared. The reason is echoed instead, and lands in the transcript --
    the same shape as ZE_ADMIT_GOVERNED_WRITE, and auditable the same way. An
    empty reason admits nothing.
    """
    owing = [p for p in add_paths if _test_coverage_required(p)]
    if not owing:
        return []
    kept: list[str] = []
    for path in owing:
        full = repo / path
        if not full.is_file():
            continue
        try:
            text = full.read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue
        if _GENERATED_HEAD_RE.search(text[:500]):
            continue
        kept.append(path)
    if not kept:
        return []
    if any(_is_test_path(p) for p in add_paths):
        return []
    return [
        "this commit carries Go and no test:\n"
        + "\n".join(f"  {p}" for p in kept)
        + "\n  Add the _test.go that proves the change, or the .ci/.et under\n"
        "  test/ that exercises it end to end. Either clears this gate.\n"
        "  Exempt: _test.go, _gen.go, register.go, embed.go, doc.go, vendor/,\n"
        "  cmd/, and a file whose first 500 bytes say `Code generated`.\n"
        "  This is the commit-time half of c_require_test_first, which sees a\n"
        "  Write of a new file and nothing else (ai/rules/testing.md)."
    ]


# The content checks pretool-writeedit.py applies to Go, re-run here over the
# lines a commit ADDS. Named rather than discovered: a check that reads session
# state answers nothing at commit time, and sweeping in every c_* would gate on
# checks nobody measured.
#
# Measured before choosing them. Over the last 40 commits, 159 Go file-diffs,
# this set fires twice, and both were Go an agent wrote through a Bash heredoc.
# Over WHOLE files it fires on 1646 of 10212, which is why the ADDED lines are
# the subject: std_content in the hook returns "the text added by Write, Edit,
# or MultiEdit", so added lines are what the hook has always judged. Anything
# wider invents a standard the repository has never held itself to.
GO_CONTENT_CHECKS = (
    "c_panic",
    "c_os_exit",
    "c_ignored_errors",
    "c_legacy_log",
    "c_sprintf_new",
    "c_string_concat",
    "c_temp_debug",
    "c_json_kebab",
    "c_goroutine",
    "c_raw_ansi",
)


def _writeedit_module():
    """The hook module, so both gates run ONE implementation.

    check_weakened_tests.py is the precedent: the pre-tool hook and this file
    share it, so neither can disagree with the other about what a diff does.
    Returns None when the hook cannot be loaded, because a broken guard must
    never brick every commit in a shared checkout.

    A FRESH module comes back on every call, so the obvious way to patch one of
    these checks in a test is a no-op: patch a check on the module you hold, and
    `go_style_problems` loads its own and never sees it. That matters more than
    it looks. A discrimination check -- revert the fix, prove the test goes red
    (`ai/rules/interop-and-goal-validation.md`) -- then reports "no
    discrimination" when the truth is the opposite, so the proof lies in the one
    direction the proof exists to catch. Patch THIS function instead, and have
    it return a module whose checks are already wrapped.
    """
    import importlib.util

    path = Path(__file__).resolve().parents[2] / ".claude/hooks/pretool-writeedit.py"
    if not path.is_file():
        return None
    spec = importlib.util.spec_from_file_location("ze_pretool_writeedit", path)
    module = importlib.util.module_from_spec(spec)
    try:
        spec.loader.exec_module(module)
    except Exception:
        return None
    return module


def go_style_problems(repo: Path, add_paths: tuple[str, ...]) -> list[str]:
    """Go style findings the pre-tool hook never saw, because Bash bypassed it.

    .claude/settings.json wires pretool-writeedit.py to
    `Write|Edit|MultiEdit|NotebookEdit`. Go written through a Bash heredoc
    reaches none of its checks, and auto mode tells agents to prefer Bash for
    file changes, so the bypass is the DEFAULT route rather than an unusual one.
    c_panic is the one that matters most: docs/contributing/ze-go-style.md calls "a
    peer MUST NOT be able to panic the daemon" the single most important line on
    the page, and that check was exactly as bypassable as the rest.

    At commit time the changed-file set is a FACT rather than a guess, which no
    pattern over a shell command can have. This gate therefore reaches what a
    pre-tool heuristic must miss.
    """
    # vendor/ is third-party source, and these checks judge how ZE writes Go.
    # A dependency author's `fmt.Sprintf`, string concatenation, or unchecked
    # Close is not a Ze style violation, and no reachable edit clears it: the
    # next `go mod vendor` writes the upstream text back. Without this, bumping
    # almost any dependency blocks the commit. Measured on the 2026-08-24
    # testify, x/tools, logrus and x/mod bump, where logrus alone raised
    # c_ignored_errors, c_sprintf_new and c_string_concat across four files.
    # Same exemption and same reason as `_design_ref_required` above, and as
    # `DEFERRAL_SCAN_EXEMPT_DIRS` took on 2026-08-14.
    go_paths = [
        p
        for p in add_paths
        if p.endswith(".go")
        and not p.endswith("_test.go")
        and not re.search(r"(^|/)vendor/", p)
    ]
    if not go_paths:
        return []
    module = _writeedit_module()
    if module is None:
        return []
    # A name that no longer resolves is an ERROR, never a skip. Renaming
    # c_panic in the hook would otherwise stop this gate checking panics with
    # the same output as a clean tree, which is the failure `ai/rules/evidence.md`
    # names: a guard must fail closed or say something. A check that RAISES is a
    # different case and stays tolerated below, for the reason given there.
    missing = [name for name in GO_CONTENT_CHECKS if not hasattr(module, name)]
    if missing:
        raise UsageError(
            "GO_CONTENT_CHECKS names checks that pretool-writeedit.py no longer "
            "defines: " + ", ".join(missing) + "\n"
            "  Rename them here too, or drop them from the tuple. Skipping one "
            "silently leaves this gate reporting a clean tree."
        )
    checks = [(name, getattr(module, name)) for name in GO_CONTENT_CHECKS]
    problems: list[str] = []
    for path in go_paths:
        # A file HEAD does not carry is entirely new, and `git diff HEAD` prints
        # nothing for it. Reading the diff alone would skip exactly the case
        # where a violation arrives: a whole new file written from a heredoc.
        tracked = run_git(repo, "ls-tree", "HEAD", "--", path, check=False).stdout
        if tracked.strip():
            diff = run_git(repo, "diff", "HEAD", "--", path, check=False).stdout
            added = "\n".join(
                line[1:]
                for line in diff.split("\n")
                if line.startswith("+") and not line.startswith("+++")
            )
        else:
            try:
                added = (repo / path).read_text(encoding="utf-8")
            except OSError:
                continue
        if not added.strip():
            continue
        # The leading slash is load-bearing. Four of these checks exempt a file
        # by a path form that HAS one -- c_panic, c_os_exit, c_legacy_log and
        # c_temp_debug test `"/scripts/" in fp`, and two also anchor on
        # `/main.go$` or `/register.go$`. At write time fp is the absolute path
        # the tool passes, so those exemptions hold; a repo-relative
        # `scripts/status/verify_run.go` matches none of them, and this gate
        # then refused a file the Edit hook had waved through, with no override
        # flag to get past it (`ai/rules/repo-maintenance.md`: the two paths'
        # exemptions must stay in step).
        #
        # Rooted at the repository rather than at the filesystem on purpose. A
        # true absolute path drags the checkout's own directory names into every
        # test, so a clone under a directory named `scripts` would exempt the
        # whole tree -- a wider failure than the one being fixed.
        ctx = {"fp": "/" + path, "content": added, "tool": "Edit", "ti": {}}
        for name, check in checks:
            try:
                result = check(ctx)
            except Exception:
                continue  # one broken check must not block every commit
            if result is None:
                continue
            detail = result[1] if isinstance(result, tuple) else str(result)
            problems.append(
                f"{path}: {name} fires on the lines this commit adds.\n{detail}\n"
                "    The pre-tool hook would have refused this edit. A shell "
                "write reached none of its checks.\n"
                "    Fix it at the source, or carry the //nolint the check "
                "documents, with its reason."
            )
    return problems


def commit_gate_problems(
    repo: Path, add_paths: tuple[str, ...], remove_paths: tuple[str, ...]
) -> list[str]:
    """All BLOCK-severity commit-time gates, in one call for create()."""
    problems: list[str] = []
    problems += go_style_problems(repo, add_paths)
    problems += go_design_ref_problems(repo, add_paths)
    problems += deferral_in_diff_problems(repo, add_paths, remove_paths)
    problems += deferral_shard_removal_problems(repo, add_paths, remove_paths)
    problems += journal_row_problems(repo, add_paths, remove_paths)
    problems += spec_audit_problems(repo, add_paths, claimed_spec(repo), remove_paths)
    problems += ste_problems(repo, add_paths)
    problems += weakened_problems(repo, add_paths, remove_paths)
    return problems


def commit_gate_warnings(
    repo: Path, add_paths: tuple[str, ...], remove_paths: tuple[str, ...] = ()
) -> list[str]:
    """All WARN-severity commit-time gates, in one call for create().

    deferral_unassigned is advisory, not blocking. An unhomed live deferral row
    is a bookkeeping state that is harmless to software behaviour: the worst case
    is a row committed too early or in the wrong commit. Blocking EVERY commit on
    it -- including commits that never touched deferrals, and rows another session
    wrote into the shared working tree -- held real work back for no software
    reason. It is still surfaced loudly (printed to stderr) so a genuine unhomed
    deferral stays visible; only the hard block is removed. Homing is still
    required by ai/rules/planning.md; this changes the gate's severity,
    not the rule.
    """
    return (
        deferral_unassigned_problems(repo)
        + wiring_warnings(add_paths)
        + doc_drift_warnings(repo)
    )


_LEARNED_STEM_RE = re.compile(r"^plan/learned/[0-9]{3,}-(?P<stem>.+)\.md$")
_SPEC_STEM_RE = re.compile(r"^plan/spec-(?P<stem>.+)\.md$")


def is_journal_class_file(path: str) -> bool:
    """A plan/journal/ class file, which README.md is not."""
    return (
        path.startswith("plan/journal/")
        and path.endswith(".md")
        and not path.endswith("/README.md")
    )


def journal_paths(paths: tuple[str, ...]) -> tuple[str, ...]:
    return tuple(path for path in paths if is_journal_class_file(path))


def journal_row_problems(
    repo: Path, add_paths: tuple[str, ...], remove_paths: tuple[str, ...]
) -> list[str]:
    """BLOCK a commit that adds a journal row this repo's parser cannot read.

    A malformed row is not a cosmetic defect here: `_journal_added_spec_stems`
    reads the SAME rows to derive the closure stem, and a row it cannot parse
    yields no stem, so `spec_closure_stem` returns None and
    `review_gate_problems` returns [] -- a closure commit carrying code lands
    with no independent review. Skipping the row was the fail-open answer
    (`ai/rules/evidence.md`): the miss path returned the permissive verdict.

    Blocking is the right severity because the author is holding the fix. The
    row is in this commit, the message names the file and the text, and one
    edit clears it.

    A row whose Spec cell names no stem is refused for the same reason. The cell
    is the review gate's key, and prose in it was read as a stem: a row saying
    `none (walked into during <spec> closure)` sent the gate looking for
    `tmp/review/none (walked into ...)-<session>.md`, so a commit that owed no
    review was blocked with a path nobody could write. `journal_spec_stems`
    reads the cell now, and what it cannot read stops here rather than reaching
    a gate as a key.

    The other two readers of the same parser already refuse to be silent:
    `journal.py` `report()` exits 1 and `spec-closure-check.py`
    `_journal_evidence()` warns on stderr. Neither can cover this one: both read
    HEAD, and the bad row is only visible BEFORE the commit that lands it.
    """
    journal = journal_paths(add_paths)
    if not journal:
        return []
    scope = set(journal)
    bad: list[str] = []
    unreadable_spec: list[str] = []
    for path, line in _prospective_added_lines(repo, add_paths, remove_paths):
        if path not in scope:
            continue
        cells = journal_row_cells(line[1:])
        if cells == [JOURNAL_MALFORMED]:
            bad.append(f"  {path}: {line[1:].strip()!r}")
        elif cells is not None and journal_spec_stems(cells[1]) is None:
            unreadable_spec.append(f"  {path}: Spec cell {cells[1]!r}")
    problems: list[str] = []
    if bad:
        problems.append(
            "this commit adds journal row(s) that do not hold the five cells\n"
            "  | Date | Spec | Surface | Symptom | Fix |, starting with `|`:\n"
            + "\n".join(bad)
            + "\n  A row this parser cannot read carries no Spec cell, so the closure\n"
            "  stem is unknown and the review gate stops firing on the commit that\n"
            "  holds the code. Fix the row (plan/journal/README.md)."
        )
    if unreadable_spec:
        problems.append(
            "this commit adds journal row(s) whose Spec cell names no spec stem:\n"
            + "\n".join(unreadable_spec)
            + "\n  The Spec cell is a KEY: the review gate looks up\n"
            "  tmp/review/<stem>-<session>.md under it. Write `-` when the defect\n"
            "  was found outside a spec, the spec stem when it was not, and put any\n"
            "  explanation in a trailing `(note)` or in the Symptom cell\n"
            "  (plan/journal/README.md)."
        )
    return problems


def _journal_added_spec_stems(
    repo: Path, add_paths: tuple[str, ...], remove_paths: tuple[str, ...]
) -> list[str]:
    """EVERY Spec cell of the journal rows this commit ADDS, in path order.

    EVERY, because one commit can add more than one row. Returning the first was
    the fail-open shape one case narrower: a commit that EDITS an older row (a
    typo in a Symptom cell) and appends the closure row emits two surviving `+`
    lines, and diff order is file order, so the edited older row answered first
    and the closure stem was months old. `spec_audit_problems` tests membership,
    so the Pre-Commit Verification gate then did not fire on the closure at all.
    The caller picks which stem it means; this function states what it found.

    ADDS is the load-bearing word. A class file is multi-row by design, so its
    first row names whatever spec closed through that class FIRST -- often
    months ago and already closed. Reading the file from the working tree and
    taking the first non-`-` Spec cell therefore hands `review_gate_problems()`
    a stale stem from the second closure through a class onward, and the review
    gate then blocks or passes on a spec nobody is closing. This is the journal
    form of the `_tracked_at_head()` newness test the learned branch already
    has: the row must be new to this commit.

    A MALFORMED added row is skipped here and REFUSED by `journal_row_problems`,
    which `commit_gate_problems` runs before the review gate. That ordering is
    what makes the skip safe: no commit reaches this function carrying a row the
    parser could not read, so the skip can no longer silence the review gate.
    Drop that gate and this becomes a fail-open branch again. An unreadable Spec
    cell (`journal_spec_stems` returns None) is the same shape and the same gate
    refuses it, which is why `or ()` below is safe to write.

    A row is compared by its CELLS, not by its diff line. The shards are
    unpadded today, so appending one row emits one `+` line; column-pad a class
    file and every row re-emits as added, the first of them naming a spec that
    closed months ago. `journal_row_cells` strips each cell, so a re-pad produces
    a `+` whose cells match a `-` exactly, and matching the two sides removes it.
    What survives is the row whose CONTENT is new, which is the question this
    function is asked. An EDIT is not cancelled by that comparison, and must not
    be: its cells differ from the `-` side, so its content IS new to this commit.
    That is why the answer is a list rather than the first survivor.

    A stem appears once. Two rows naming the same spec are one closure, and the
    duplicate would make `spec_closure_stem`'s "more than one" test read as an
    ambiguity that is not there.
    """
    journal = journal_paths(add_paths)
    if not journal:
        return []
    changes = _prospective_line_changes(
        repo, add_paths, remove_paths, "--diff-filter=AM"
    )
    scope = set(journal)
    added: dict[str, list[list[str]]] = {}
    removed: dict[str, set[tuple[str, ...]]] = {}
    for path, sign, text in changes:
        if path not in scope or text == "":
            continue
        cells = journal_row_cells(text)
        if cells is None or cells == [JOURNAL_MALFORMED]:
            continue
        if sign == "+":
            added.setdefault(path, []).append(cells)
        elif sign == "-":
            removed.setdefault(path, set()).add(tuple(cells))
    stems: list[str] = []
    for path in journal:
        gone = removed.get(path, set())
        for cells in added.get(path, []):
            if tuple(cells) in gone:
                continue  # the same row, reformatted: not new content
            for spec in journal_spec_stems(cells[1]) or ():
                if spec not in stems:
                    stems.append(spec)
    return stems


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


def _tracked_at_head(repo: Path, path: str) -> bool:
    """Whether HEAD already carries this path."""
    return (
        subprocess.run(
            ["git", "cat-file", "-e", f"HEAD:{path}"],
            cwd=repo,
            capture_output=True,
        ).returncode
        == 0
    )


def _spec_closed_earlier(repo: Path, stem: str) -> bool:
    """Whether plan/spec-<stem>.md is GONE from the tree, having once existed.

    Spec closure is two commits and commit B is `git rm plan/spec-<stem>.md`
    (ai/rules/planning.md "Spec Closure"). So at commit A the spec file is still
    on disk, and for any spec that closed earlier it is not. That is the fact
    that separates "this row names the spec closing now" from "this row names a
    spec that closed months ago", and it needs no new state: the two-commit rule
    already guarantees it.

    BOTH halves are load-bearing. Gone-from-disk alone would also drop a Spec
    cell naming a path git has never held -- a typo, or a spec not yet written --
    and that stem must be KEPT, because a misspelled cell on a real closure
    commit would otherwise disarm the review gate silently. Only a path git once
    held and no longer holds is a closed spec.
    """
    path = f"plan/spec-{stem}.md"
    if (repo / path).exists():
        return False
    res = subprocess.run(
        ["git", "log", "-1", "--format=%H", "--", path],
        cwd=repo,
        capture_output=True,
        text=True,
    )
    return res.returncode == 0 and res.stdout.strip() != ""


def _spec_closing_removals(
    add_paths: tuple[str, ...], remove_paths: tuple[str, ...]
) -> list[str]:
    """The stems of the specs this commit removes and does NOT put back.

    `git rm plan/spec-<stem>.md` is commit B of the two-commit spec closure
    (`ai/rules/planning.md`, "Spec Closure"), so a removal states which spec
    closed. A MOVE removes the same path and adds the file again elsewhere:
    `plan/future/spec-<stem>.md` is where real work that does not block the first
    release goes (`plan/future/README.md`). Nothing closed there, so no review
    was run and none is owed, and `review_gate.py` keys its artifact on the stem
    of a spec that is still open -- a path nobody can write. The only escapes
    were an owner-only override or two commits that each leave the tree
    inconsistent (`plan/journal/gate-fires-outside-its-population.md`).

    So a removal is evidence of intent only when nothing else in the same commit
    contradicts it. The re-added path is matched on its BASENAME, because the
    destination is not fixed: what makes it the same spec is that
    `spec-<stem>.md` still exists somewhere for the next reader to find.

    The removed path itself is excluded from that search. A commit naming one
    path in both lists is contradictory rather than a move, and reading it as a
    move would take the review gate off a real closure.

    Both readers of a removal come through here: `spec_closure_stem` arms the
    review gate, and `closure_reminder` nudges for the commit B a closure
    artifact is waiting for. A filter that lands in one reader alone is how those
    two last disagreed about what a closure is.
    """
    stems: list[str] = []
    for path in remove_paths:
        match = _SPEC_STEM_RE.match(path)
        if match is None:
            continue
        moved_to = f"spec-{match.group('stem')}.md"
        if any(p != path and p.rsplit("/", 1)[-1] == moved_to for p in add_paths):
            continue
        stems.append(match.group("stem"))
    return stems


def _spec_belongs_to_another_session(
    repo: Path, stem: str, add_paths: tuple[str, ...]
) -> bool:
    """Whether plan/spec-<stem>.md is an OPEN spec this session is not working on.

    A journal class file is shared. Several sessions append to it, and the row
    one of them adds names the spec THAT session closes. `review_gate.py` keys
    its artifact on the committing session, so reading a foreign row as this
    commit's closure asks for `tmp/review/<their-stem>-<my-session>.md`, a path
    only the closing session can write. The file then cannot be landed by anyone
    else, and the session it blocks can be one that added no row at all.

    Two facts say the spec is this session's, and BOTH are needed. The claim
    (`claimed_spec`) covers every commit made while the work is in flight. The
    `--file` list covers closure itself: `ze-close` step 6b releases the claim
    BEFORE step 6d prepares the commit, so at closure the claim is already gone
    and what remains is commit A carrying `plan/spec-<stem>.md` to preserve its
    edits (ai/rules/planning.md, "Spec Closure").

    A stem with no spec file at all is NOT another session's: it is a typo, or a
    spec not yet written, and it stays a closure signal so a misspelled Spec cell
    on a real closure commit still fails closed. A stem whose spec closed
    earlier is dropped before this by `_spec_closed_earlier`.
    """
    spec = f"plan/spec-{stem}.md"
    if not (repo / spec).is_file():
        return False
    if spec in add_paths:
        return False
    return stem != spec_stem(claimed_spec(repo))


# --- Verification debt -------------------------------------------------------
#
# A row says "this gate did not run green over this commit, and the commit was
# made anyway". Before this ledger the same commits were made and nothing was
# written down, and the alternative -- refusing the commit -- is worse: several
# sessions share this checkout, the record is red for somebody else's in-flight
# work nearly always, and a commit that never lands is the most expensive failure
# this repo has (`ai/rules/rule-precedence.md`).
#
# Since 2026-08-21 the row is written from the FACT rather than from a flag: a
# stale verify record and Go no full run has seen each record one, whether or not
# the caller typed an override (`create`, and ai/rules/git-safety.md "Verify a
# Commit, Not the Working Tree"). An override flag survives to give the row its
# REASON, which is the attribution a measurement cannot make.
#
# So the commit is allowed and the OBLIGATION is written down. What is recorded
# is VERIFICATION debt: a gate that has not yet run over this code. That is a
# different thing from a DEFECT, which `ai/rules/completion.md` requires you to
# fix rather than record, and this ledger is never a home for one.
#
# Sharded per commit-session for the reason `plan/deferrals/` is: a single shared
# file cross-commits between concurrent sessions whatever the --file list says
# (`ai/rules/git-safety.md`).
#
# VERIFICATION_DEBT_DIR itself is declared beside DEFERRAL_SCAN_EXEMPT_DIRS,
# which names it and is read at import time well before this point.

# Each gate a commit can owe, keyed by the flag that declares it. `debt_owed`
# reads the flag and the fact `create` measured under the same key, so a gate
# whose absence the run detects needs no flag to earn its row.
DEBT_FLAGS = (
    ("unverified", "ze-precommit-verify (not FRESH-green)"),
    ("structural_red_ok", "ze-precommit-verify structural gates (red)"),
    ("missing_full_verify_ok", "full ze-precommit-verify over this commit's Go"),
    ("stale_index_ok", "discovery-index freshness"),
    ("review_override", "independent critical review"),
    ("broken_head_fix", "ze-repository-tracked-build-check (HEAD does not compile)"),
    ("rfc_change_ok", "owner approval for an RFC-tagged test change"),
)

DEBT_HEADER = (
    "| Date | Session | Subject | Gate owed | Reason | Status |",
    "|------|---------|---------|-----------|--------|--------|",
)


def debt_shard_path(session: str) -> str:
    return f"{VERIFICATION_DEBT_DIR}/{session}.md"


def debt_owed(
    args: argparse.Namespace,
    observed: list[tuple[str, str]] | tuple[tuple[str, str], ...] = (),
) -> list[tuple[str, str]]:
    """(gate, reason) for every gate this commit owes. Order is DEBT_FLAGS.

    Two sources name the same gates. `args` carries what the CALLER declared: an
    override flag, whose reason is the attribution the ledger keeps. `observed`
    carries what `create` MEASURED, keyed by the same flag name: a verify record
    that is not FRESH-green, or Go that no full run has seen. Since 2026-08-21
    those two facts record a row instead of refusing the commit, so a row is
    written whether or not anybody typed a flag (ai/rules/git-safety.md).

    A declared reason WINS over a measured one for the same gate. The caller can
    say whose red it is and which run will cover the commit; the measurement can
    only say what state the record is in. One gate never yields two rows: this
    walks DEBT_FLAGS once, which is what `record_debt` and `open_debt_rows` both
    rely on.
    """
    measured = dict(observed)
    owed: list[tuple[str, str]] = []
    for attr, gate in DEBT_FLAGS:
        reason = (getattr(args, attr, None) or "").strip() or measured.get(
            attr, ""
        ).strip()
        if reason:
            owed.append((gate, reason))
    return owed


def _debt_cell(text: str) -> str:
    """One table cell: no newline, no pipe, so a reason cannot forge a row."""
    return " ".join(text.replace("|", "/").split())


def record_debt(
    repo: Path, session: str, subject: str, owed: list[tuple[str, str]]
) -> str:
    """Append one row per owed gate to this session's shard. Returns its path.

    A row the shard already holds OPEN is not appended again. Two routes
    re-record what a previous run wrote: `--replace`, which supersedes a script
    without knowing which rows its predecessor left, and a `create` killed after
    it appended and then run again. Both inflate the count the push gate reads
    and make `debt-clear` re-run one gate per duplicate row.

    Byte-identical open rows are duplicates and nothing else, so the text is a
    safe key HERE. `DEBT_FLAGS` holds one entry per gate and `debt_owed` walks
    it once, so a single call cannot emit the same gate twice. `clear_debt_rows`
    still keys on the line POSITION, and for its own reason: it judges rows that
    already exist, where an append landing mid-pass could match a verdict passed
    on a twin. This function runs before any position exists.

    Deduplication is over OPEN rows alone. A cleared row is a spent record, so a
    later commit owing the same gate earns a fresh one.
    """
    rel = debt_shard_path(session)
    path = repo / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    stamp = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%d")
    header: list[str] = [
        f"# Verification debt -- commit session {session}",
        "",
        "Gates that had not run green over these commits when they were made.",
        "Each row is work owed, not a defect: a defect is fixed, never recorded",
        "(`ai/rules/completion.md`). Clear a row with `make ze-verify-debt-clear`:",
        "it re-runs the gate the row names and writes `cleared` only when that",
        "run exits 0. Every gate runs inside one throwaway worktree at HEAD, so",
        "a cleared row says the gate was green over the COMMIT rather than",
        "beside the uncommitted files several sessions keep in this checkout.",
        "When no worktree can be made, nothing clears and the pass exits 1: the",
        "fallback it refuses, judging the working tree, is the whole reason the",
        "worktree is there. A human MAY delete the",
        "shard once every row is cleared.",
        "`scripts/dev/commit_helper.py create --push` refuses while any row here",
        "is open.",
        "",
        *DEBT_HEADER,
    ]
    candidates = [
        f"| {stamp} | {session} | {_debt_cell(subject)} | {_debt_cell(gate)} "
        f"| {_debt_cell(reason)} | open |"
        for gate, reason in owed
    ]
    # The same exclusive lock `clear_debt_rows` takes. Without it an append
    # landing inside a clearing pass's read-modify-write is lost, and this
    # checkout runs several sessions at once. The read of what the shard
    # already holds sits inside it too, so a concurrent append cannot slip
    # between the test and the write.
    with path.open("a+", encoding="utf-8") as fh:
        fcntl.flock(fh, fcntl.LOCK_EX)
        fh.seek(0)
        held = {line.strip() for line in fh.read().splitlines()}
        lines = [] if held else list(header)
        lines += [row for row in candidates if row not in held]
        if lines:
            fh.write("\n".join(lines) + "\n")
    return rel


def open_debt_rows(repo: Path) -> list[tuple[str, str, int]]:
    """(shard, row, position) for every row whose Status cell is `open`.

    Reads the tracked shards, so it answers the same question in any session on
    this checkout -- which is the point: the session that pushes is rarely the
    session that owed the gate.

    `position` is the row's 0-based line index in its shard, and it is the row's
    IDENTITY. The text is not, because a shard CAN hold byte-identical open
    rows: one written by hand, and one recorded before `record_debt` learned to
    refuse a duplicate. `clear_debt_rows` keys on the position for that reason.

    This docstring also claimed that one `create` renders two identical rows
    when two overrides share a gate and reason. It cannot. `DEBT_FLAGS` holds
    one entry per gate and `debt_owed` walks it once, so a call emits each gate
    at most once. The claim went unchecked and made the duplicate rows a second
    `create` produced look like a shape the ledger expected.
    """
    rows: list[tuple[str, str, int]] = []
    root = repo / VERIFICATION_DEBT_DIR
    if not root.is_dir():
        return rows
    for shard in sorted(root.glob("*.md")):
        try:
            text = shard.read_text(encoding="utf-8")
        except OSError:
            continue
        for index, line in enumerate(text.splitlines()):
            cells = [c.strip() for c in line.split("|")]
            # A rendered row is `| a | b | c | d | e | f |` -> 8 split cells.
            if len(cells) != 8 or cells[-2].lower() != "open":
                continue
            rows.append((shard.name, line.strip(), index))
    return rows


# --------------------------------------------------------------------------- #
# Clearing a debt row.
#
# A row is cleared by RUNNING the gate it names and reading that run's exit
# code. The alternative is what the ledger shipped with -- a human editing the
# Status cell -- and by 2026-08-19 it had produced 270 open rows against 22
# cleared, because the session that owes a gate is rarely the session that can
# spare an hour to run it. A `cleared` written any other way would be a CLAIM about verification
# sitting in the artifact that exists to hold evidence of it
# (`ai/rules/evidence.md`), so nothing below writes that word without an exit 0
# in hand.
# --------------------------------------------------------------------------- #

# (exit code, what the gate printed). Exit 0, and only exit 0, clears a row.
GateVerdict = tuple[int, str]
GateRunner = Callable[[Path], GateVerdict]


def gate_command(*argv: str) -> GateRunner:
    """A debt gate re-run by executing `argv` from the repo root.

    The command line is echoed into the returned output, so a reader of a red
    gate sees which command produced it without the runner carrying a second
    field for it.
    """

    def run(repo: Path) -> GateVerdict:
        shown = "$ " + " ".join(argv) + "\n"
        try:
            proc = subprocess.run(
                list(argv),
                cwd=repo,
                capture_output=True,
                text=True,
                # `repo` is a throwaway worktree during a debt-clear pass, so the
                # gate runs as its own admitted job rather than inheriting the
                # caller's `ZE_RUN_JOB` marker and skipping admission.
                env=_gate_env(),
            )
        except OSError as exc:
            return 1, shown + f"{argv[0]} did not run: {exc}"
        return proc.returncode, shown + (proc.stdout or "") + (proc.stderr or "")

    return run


def index_head_gate(repo: Path) -> GateVerdict:
    """The discovery-index gate, re-judged over HEAD rather than the tree.

    `discovery_index_head_status` materializes HEAD and re-runs the generators
    there, so it answers about the COMMITTED code, which is what a debt row is
    about. The working-tree spelling (`make ze-discovery-index-check`) answers
    about every other session's uncommitted sources too, and the row would then
    wait on work nobody in this pass owns. "unknown" does not clear: a gate that
    could not answer has not passed.
    """
    state, stale = discovery_index_head_status(repo)
    detail = f"discovery-index at HEAD: {state}"
    if stale:
        detail += "\n" + "\n".join("  stale: " + name for name in stale)
    return (0 if state == "fresh" else 1), detail


_DEBT_GATE_NAME = dict(DEBT_FLAGS)

# How each owed gate is re-judged: a runner taking the tree to run in, or the
# sentence saying why no command can produce that judgement. Keyed by the flag
# rather than by the literal cell text, so rewording a gate name in DEBT_FLAGS
# cannot orphan an entry here. A row written under an OLDER wording finds no
# runner and stays open, which is the safe direction.
#
# A gate added here MUST NOT pass make variables on the command line. Job
# admission keys a running job on its command text plus the variables it parsed
# (`_job_key`, `scripts/dev/ze-run.sh`), and a variable the key loses makes two
# different runs hash the same, so a job can take its verdict from an unrelated
# one. That was live on this host until 2026-08-22 -- GNU Make 3.81 omits the
# ` -- ` separator when the command line carries no flag, and the parser read
# only what followed it -- and a run for one package was measured exiting 0 on
# another's result. Everything here is a bare target for that reason. A
# parameterized gate would write somebody else's verdict into the ledger as
# `cleared`, which is the artifact a later reader trusts (`ai/rules/evidence.md`).
DEBT_GATE_RUNNERS: dict[str, GateRunner | str] = {
    _DEBT_GATE_NAME["unverified"]: gate_command("make", "ze-precommit-verify"),
    _DEBT_GATE_NAME["structural_red_ok"]: gate_command(
        "make", *sorted(STRUCTURAL_GATES)
    ),
    _DEBT_GATE_NAME["missing_full_verify_ok"]: gate_command(
        "make", "ze-precommit-verify"
    ),
    _DEBT_GATE_NAME["stale_index_ok"]: index_head_gate,
    _DEBT_GATE_NAME["review_override"]: (
        "a review is a judgement made in another context, and no command "
        "produces one. `scripts/dev/review_gate.py` records that a review "
        "happened; it cannot perform it. Run /ze-review over the commit, then "
        "set this row to `cleared` by hand"
    ),
    _DEBT_GATE_NAME["broken_head_fix"]: gate_command("make", TRACKED_BUILD_GATE),
    _DEBT_GATE_NAME["rfc_change_ok"]: (
        "an approval is the owner's answer, and no command produces one. Ask "
        "him, write the row in test/rfc-changed.md beside the change it "
        "approves, then set this row to `cleared` by hand"
    ),
}


def gate_last_word(output: str) -> str:
    """The last thing a passing gate said, which is what a PASS prints.

    A green `make ze-precommit-verify` writes an hour of log and none of it
    changes what the operator does next, so a pass does not print the whole
    thing. Printing NOTHING is the other failure: a `PASS` line this file wrote
    would then look the same whether the gate ran or not, and this pass exists
    because a `cleared` nobody could check is worthless. One line of the gate's
    own output is the cheapest thing that is still evidence.
    """
    spoken = [line for line in output.splitlines() if line.strip()]
    return spoken[-1] if spoken else "(the gate printed nothing)"


def debt_row_gate(row: str) -> str:
    """The `Gate owed` cell of a rendered debt row, or "" when it has none."""
    cells = [c.strip() for c in row.split("|")]
    return cells[4] if len(cells) == 8 else ""


def clear_debt_rows(path: Path, rows: dict[int, str]) -> int:
    """Set Status to `cleared` at each POSITION of `rows`, in place. Returns the count.

    The key is the line index this pass judged the row at; the value is the text
    it carried then. Both must still hold, so a shard a human rewrote in the
    meantime loses nothing: a position that no longer reads as the judged row is
    copied out unchanged.

    The position is the identity because the TEXT is not. Two rows can be
    byte-identical (`open_debt_rows`), so a row APPENDED while the gates ran
    could match a judgement passed on its twin and be cleared with it -- a
    `cleared` over a commit no gate covered, which is the one thing this ledger
    exists not to hold.

    The shard is re-read under the exclusive lock `record_debt` takes, which is
    what orders an append against this rewrite rather than racing it. An append
    lands after every position this pass holds and shifts none of them, and the
    rewrite preserves the line count.
    """
    written = 0
    with path.open("r+", encoding="utf-8") as handle:
        fcntl.flock(handle, fcntl.LOCK_EX)
        out: list[str] = []
        for index, line in enumerate(handle.read().splitlines()):
            if rows.get(index) == line.strip():
                cells = line.split("|")
                cells[-2] = " cleared "
                line = "|".join(cells)
                written += 1
            out.append(line)
        handle.seek(0)
        handle.write("\n".join(out) + "\n")
        handle.truncate()
    return written


def clear_debt(args: argparse.Namespace) -> int:
    """`commit_helper.py debt-clear`: re-run every open row's gate, clear on 0.

    Every gate runs in ONE throwaway worktree at HEAD (`_worktree_at`), so a
    `cleared` says the gate was green over the COMMIT. It used to say something
    much weaker. The runnable gates are mostly plain `make` targets, they used to
    run against the WORKING TREE, and in this checkout that tree carries several
    other sessions' uncommitted files: a pass could therefore go red on work
    nobody in it owned, or green on work nobody in it had written. This function's
    own docstring named the repair for months -- materialize HEAD once and run the
    make targets inside it -- and that is now what happens.

    A pass with no runnable gate materializes nothing, so an all-unrunnable pass
    pays no checkout.

    When the worktree cannot be made, NOTHING clears and the pass exits 1. The
    fallback that suggests itself -- run them here instead -- is the exact defect
    this removes, and it would write `cleared` into the artifact that exists to
    hold verification evidence (`ai/rules/evidence.md`).

    The worktree is also what keeps a cached verdict out of the ledger. `go test`
    keys a cached result on the absolute paths in its testlog, so a gate at a
    fresh path starts cold and cannot answer `ok (cached)` for a run that never
    happened (`worktree_path`, `scripts/dev/verify_worktree.py`). Against the
    shared tree the path is stable and that staleness is reachable, which is a
    second reason the old spelling could write a `cleared` nobody could check.

    Each distinct gate runs ONCE, however many rows name it, and every row
    naming a gate that exited 0 is cleared (AC-6). A gate that exited non-zero
    leaves its rows open and its output is printed (AC-7), and so does a gate no
    command can produce -- with the reason, so an unrunnable row never reads as
    a clearable one.

    The shard is never deleted, even when its last row clears: the header says a
    human MAY delete it, and a file another session still holds rows in is not
    this pass's to remove (`ai/rules/never-destroy-work.md`).
    """
    repo = repo_root(args.repo)
    rows = open_debt_rows(repo)
    if not rows:
        print("No open verification-debt rows.")
        return 0
    by_shard: dict[str, list[tuple[int, str, str]]] = {}
    for shard, row, line in rows:
        by_shard.setdefault(shard, []).append((line, row, debt_row_gate(row)))
    owed = {gate for shard_rows in by_shard.values() for _, _, gate in shard_rows}
    passed: set[str] = set()
    runnable: list[str] = []
    for gate in sorted(owed):
        runner = DEBT_GATE_RUNNERS.get(gate)
        if callable(runner):
            runnable.append(gate)
            continue
        reason = runner or f"no gate is registered for {gate!r}"
        print(f"UNRUNNABLE  {gate}\n  {reason}")
    # Every runnable gate runs in ONE worktree at HEAD, and the unrunnable ones
    # were reported above so a pass with nothing to run pays no checkout.
    if runnable:
        head = subprocess.run(
            ["git", "-C", str(repo), "rev-parse", "HEAD"],
            capture_output=True,
            text=True,
            check=False,
        )
        if head.returncode != 0:
            print("UNWORKTREED no HEAD to materialize; every row stays open")
            return 1
        sha = head.stdout.strip()
        print(f"WORKTREE    {sha[:12]} (gates run over the commit, not the tree)")
        try:
            with _worktree_at(repo, sha) as tree:
                for gate in runnable:
                    runner = DEBT_GATE_RUNNERS[gate]
                    assert callable(runner)
                    print(f"RUN         {gate}", flush=True)
                    code, output = runner(tree)
                    if code == 0:
                        print(f"PASS        {gate}")
                        print(textwrap.indent(gate_last_word(output), "  "))
                        passed.add(gate)
                        continue
                    print(f"RED         {gate} (exit {code})")
                    print(textwrap.indent(output.rstrip("\n"), "  "))
        except RuntimeError as exc:
            # No worktree means no honest answer. Falling back to the working
            # tree is the defect this pass exists to remove, so nothing clears.
            print(f"UNWORKTREED {exc}\n  every row stays open")
            return 1
    cleared = 0
    for shard in sorted(by_shard):
        judged = {line: row for line, row, gate in by_shard[shard] if gate in passed}
        if not judged:
            continue
        try:
            cleared += clear_debt_rows(repo / VERIFICATION_DEBT_DIR / shard, judged)
        except OSError as exc:
            # A human MAY delete a shard whose rows all cleared, and one may have
            # done so between the read above and here. Say which shard kept its
            # rows open; do not lose the rest of the pass over it.
            print(f"UNWRITTEN   {shard}: {exc}")
    print(f"cleared {cleared} row(s), {len(rows) - cleared} still open")
    return 0


def spec_closure_stem(
    add_paths: tuple[str, ...],
    remove_paths: tuple[str, ...],
    repo: Path | None = None,
) -> str | None:
    """The spec-stem this commit closes, or None if it is not a closure commit.

    Closure = commit A adds a NEW plan/learned/NNN-<stem>.md or a NEW journal row
    naming the spec, or commit B removes plan/spec-<stem>.md (ai/rules/planning.md
    "Spec Closure"). The <stem> is the key the review artifact
    (tmp/review/<stem>-<session-id>.md) is written under.

    The removed spec is read BEFORE the journal, because `git rm
    plan/spec-<stem>.md` states which spec closes and a Spec cell only names one.
    Commit B carries no code and often carries a journal edit; reading the
    journal first made it derive the wrong spec.

    A removal whose file the same commit puts back elsewhere is a MOVE and says
    nothing about a closure (`_spec_closing_removals`).

    NEW is the load-bearing word, and `repo` is what makes it checkable. A commit
    may carry a learned summary for a reason that is not a closure: repointing a
    dead `## Files` path, correcting a citation, a sweep over many summaries at
    once. Reading any such path as a closure picks whichever sorts first and then
    demands a review artifact for a spec nobody is closing -- measured on a commit
    that repointed 23 summaries and was accused of closing the first of them.
    A summary already in HEAD is therefore not a closure signal.

    With `repo` omitted the old behaviour stands, so a caller that cannot reach git
    still fails CLOSED (every learned path counts) rather than silently letting a
    real closure through unreviewed.

    A journal row counts only when it is THIS session's row
    (`_spec_belongs_to_another_session`). A class file collects rows from many
    sessions, so an added row can name a spec another session is closing, and a
    rewritten row reads as an added row in a diff against HEAD. The detection was
    right in both measured cases and the attribution was wrong: the gate demanded
    `tmp/review/<their-stem>-<my-session>.md`, which only the closing session can
    write, so a shared class file became unlandable by anyone else
    (`plan/journal/gate-fires-outside-its-population.md`, 2026-08-22).

    One commit CAN still add two rows this session owns, and only then is the
    claim consulted as a tie-break: the row says a closure happened, the claim
    says which one is this session's.

    A stem whose spec CLOSED EARLIER is dropped before any of that
    (`_spec_closed_earlier`). The claim tie-break only runs on two stems or more,
    so it never reached the shape that matters most: a commit that only EDITS one
    older row -- the typo fix `_journal_added_spec_stems` cites -- yields exactly
    ONE stem, months old, and the fallback returned it. An ordinary code-free
    journal typo fix was then refused in the name of a spec nobody was closing.

    The filter can only REMOVE stems, so it arms the gate nowhere it was not
    armed already. That is what settles the question the next reader will ask:
    a MID-WORK commit that carries code and adds a row naming the still-OPEN
    claimed spec is read as a closure and demands a review artifact, and that
    behaviour is untouched here -- it predates this filter, which cannot reach
    it. `spec-closure-check.py` `completed_not_closed` met the same shape and
    reconciled it by counting a row only alongside the spec's own finished
    Review Gate. That reconciliation does NOT port into this gate: it is an
    ADVISORY detector, and a conjunct that costs it a nag costs this gate its
    block. Deleting the `## Review Gate` section from the spec would become the
    cheapest route to landing a closure with no review on record, and a BLOCK
    gate may not depend on a section the committed spec controls. Whether the
    mid-work demand is friction worth its own fix is a live question; weakening
    this gate is not the answer to it.
    """
    for p in add_paths:
        m = _LEARNED_STEM_RE.match(p)
        if m and (repo is None or not _tracked_at_head(repo, p)):
            return m.group("stem")
    removed = _spec_closing_removals(add_paths, remove_paths)
    if removed:
        return removed[0]
    if repo is not None:
        stems = [
            stem
            for stem in _journal_added_spec_stems(repo, add_paths, remove_paths)
            if not _spec_closed_earlier(repo, stem)
            and not _spec_belongs_to_another_session(repo, stem, add_paths)
        ]
        if len(stems) > 1:
            claimed = spec_stem(claimed_spec(repo))
            if claimed in stems:
                return claimed
        if stems:
            return stems[0]
    return None


def review_gate_problems(
    repo: Path, add_paths: tuple[str, ...], remove_paths: tuple[str, ...]
) -> list[str]:
    """BLOCK a spec-closure commit whose code is not covered by a fresh, CLEAN,
    INDEPENDENT review (ai/rules/planning.md).

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
    stem = spec_closure_stem(add_paths, remove_paths, repo)
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
        "  (ai/rules/planning.md): spawn reviewer subagents over the diff,\n"
        "  fix findings, loop to zero, and record with:\n"
        "    python3 scripts/dev/review_gate.py record --spec "
        + stem
        + " --verdict clean --rounds <N> --files <code files>\n"
        "  --rounds is the pass count. Past 5 it needs --rounds-reason naming the\n"
        "  PRODUCT defect a later round found; a false statement in the spec's own\n"
        "  closure prose is not one. Past 5 it ALSO needs --owner-authorised:\n"
        "  more than five passes is Thomas's decision, so you stop and ask him\n"
        "  rather than setting that flag yourself.\n"
        '  Owner override: --review-override "<reason>".'
    ]


def create(args: argparse.Namespace) -> int:
    """Prepare one commit: a message file, and a block in a script.

    Owns the tag RESERVATION `next_tag` takes. A run that is refused by a gate,
    or that only dry-runs, must give the tag back: otherwise every refusal burns
    a letter and the tags stop tracking prepared commits.
    """
    repo = repo_root(args.repo)
    session = session_id(repo, args.session)
    tag = normalize_tag(args.tag, repo, session)
    reservation = None if args.tag else repo / message_rel_path(session, tag)
    try:
        code = _create(args, repo, session, tag)
    except BaseException:
        release_tag_reservation(reservation)
        raise
    if args.dry_run:
        release_tag_reservation(reservation)
    return code


def _create(args: argparse.Namespace, repo: Path, session: str, tag: str) -> int:
    add_paths = unique_paths([rel_path(repo, raw) for raw in args.file])
    remove_paths = unique_paths([rel_path(repo, raw) for raw in args.remove])
    if not add_paths and not remove_paths:
        raise UsageError("at least one --file or --remove path is required")
    for path in add_paths:
        validate_add_path(repo, path)
    for path in remove_paths:
        validate_remove_path(repo, path)
    # Refuse an unusable authorisation before any expensive gate runs: the reason
    # is the only record of who ordered the push.
    push_reason = push_authorisation(args.push)
    # Verify gate. It reads the verify record, and since 2026-08-21 that reading
    # gates the PUSH rather than the commit (ai/rules/git-safety.md, "Verify a
    # Commit, Not the Working Tree"). A commit that stays local costs nobody
    # anything and a commit that never happens costs the work, so a record that
    # is not FRESH-green records a verification-debt row here and the commit
    # proceeds. `--push` refuses while that row is open, which is where the debt
    # is really owed: a push is what reaches users.
    #
    # `observed_debt` is what this run MEASURED, keyed by the flag whose gate it
    # names, and `debt_owed` merges it with what the caller declared.
    observed_debt: list[tuple[str, str]] = []
    vstate, detail = verify_status(repo, add_paths + remove_paths)
    if vstate == "stale":
        # A DETERMINISTIC STRUCTURAL GATE red (tier/lint/vet/plugin-boundary/
        # iface-resolution/regen-check-readonly/wiring-docs/tracked-build) is
        # never flaky or environmental: the tree is structurally broken. That is a
        # DIFFERENT fact from unverified, and it is the one thing still refused at
        # commit time. Unverified says a gate has not run over this code, which a
        # debt row carries to the push. Structurally broken says the code does not
        # hold together now, and a debt row cannot carry that anywhere: the next
        # session inherits a tree that does not build. It is NOT eligible for a
        # plan/known-failures/ known-red either (those cover flaky TEST stages
        # only). This closes the hole that let a misplaced-tier gate
        # (routeinstall) be parked as "pre-existing" and shipped red on main.
        # See ai/rules/git-safety.md.
        reds = structural_gate_reds(repo, add_paths + remove_paths)
        gate_reds = list(reds.charged)
        # Attribution is LOUD in both directions. A dropped red is still a red
        # somebody owns, and a session that never hears which one was dropped
        # cannot tell a scoped gate from an ignored one.
        if reds.foreign:
            print(
                "NOTE: structural gate(s) red for another session only: "
                + ", ".join(reds.foreign)
                + "\n  Every file their failure groups name lies outside this"
                " commit, so no\n  verification debt is charged for them. They"
                " are still red for the tree.",
                file=sys.stderr,
            )
        # ze-repository-tracked-build-check is the one structural gate whose red lives in
        # HEAD rather than in the working tree, so it is NOT cleared before the
        # next commit -- it is cleared BY it, by landing the producer that was
        # left behind. Refusing every commit until it goes green is a deadlock:
        # the gate blocks the only commit that can fix it, and the sole escape
        # would be the owner-only --structural-red-ok. This flag is that escape
        # made reachable, and it is deliberately NARROW: it applies only when
        # tracked-build is the ONLY structural red, so it can never wave through
        # a lint, tier, vet or wiring failure riding alongside it.
        broken_head_declared = bool((args.broken_head_fix or "").strip())
        if set(gate_reds) == {TRACKED_BUILD_GATE} and broken_head_declared:
            print(
                "WARNING: HEAD does not compile ("
                + TRACKED_BUILD_GATE
                + " is red).\n  This commit is declared to be the fix: "
                + args.broken_head_fix.strip()
                + "\n  Run `make ze-repository-tracked-build-check` after the script and confirm it is"
                " green.\n  If it is not, HEAD is still broken for everybody who builds it.",
                file=sys.stderr,
            )
            gate_reds = []
        # ONE escape, owner-only, and it carries its own flag so that nothing
        # which merely says "this code is unverified" can reach this branch. The
        # reason has to be written down. Without any escape a red structural gate
        # was the only route to any commit at all -- including one that touches no
        # compiled code and cannot affect the red -- so the refusal was pushing
        # sessions toward the real hole: editing STRUCTURAL_GATES. The override is
        # LOUD on purpose: a silent bypass would make a red tree
        # indistinguishable from a green one in the transcript.
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
                "ze-precommit-verify has a DETERMINISTIC STRUCTURAL GATE red charged "
                "to this commit: " + ", ".join(gate_reds) + ".\n"
                "  Structural gates (tier/lint/vet/plugin-boundary/iface-resolution/\n"
                "  regen-check-readonly/wiring-docs/tracked-build) never fail for flaky\n"
                "  or environmental reasons -- a red means the tree is structurally\n"
                "  broken. An unverified commit lands and owes a debt row. A BROKEN one\n"
                "  does not, because the next session inherits the tree. This red is NOT\n"
                "  eligible for a plan/known-failures/ known-red.\n"
                + (
                    "  Charged for want of attribution: "
                    + "; ".join(reds.unattributed)
                    + ".\n  Those failure groups name a check, a suite or the"
                    " stage itself, never a\n  file, so this commit's file list"
                    " cannot rule the red out. A gate whose\n  groups name only"
                    " other sessions' files is dropped instead.\n"
                    if reds.unattributed
                    else ""
                )
                + (
                    "  "
                    + TRACKED_BUILD_GATE
                    + " is red: HEAD ITSELF does not compile,\n"
                    "  usually because a commit took a consumer and left its producer\n"
                    "  uncommitted. That red is cleared BY a commit, not before one. If THIS\n"
                    '  commit lands the missing producer, pass --broken-head-fix "<reason>"\n'
                    "  and prepare it again. The stale verify record needs no flag: it\n"
                    "  records a debt row and --push refuses while that row is open.\n"
                    if TRACKED_BUILD_GATE in gate_reds
                    else ""
                )
                + "  Fix it at the source (run `make "
                + gate_reds[0]
                + "` to see the\n"
                "  failure). "
                + (
                    "For every gate EXCEPT tracked-build, to CLEAR this\n"
                    "  refusal you must refresh the verify record:\n"
                    if TRACKED_BUILD_GATE in gate_reds
                    else "To CLEAR this refusal you must refresh the verify record:\n"
                )
                + "  only `make ze-precommit-verify` (or `make ze-precommit-verify-changed`) rewrites\n"
                "  tmp/ze-verify-failures.json -- `make "
                + gate_reds[0]
                + "` alone does\n"
                "  NOT. Re-run a full verify until green and this clears."
                + (
                    "\n  tracked-build is the exception: only the commit that lands the\n"
                    "  missing producer clears it, which is what --broken-head-fix is for.\n"
                    "  That flag is honored ONLY when tracked-build is the sole structural\n"
                    "  red, so fix any gate listed beside it first."
                    if TRACKED_BUILD_GATE in gate_reds
                    else ""
                )
            )
        # The record is not FRESH-green, and past this point nothing about that
        # refuses the commit. The gate is OWED, not skipped: one row per gate, and
        # `--push` refuses while any row is open. No flag is required, and that is
        # the point of the 2026-08-21 directive -- a reason typed on every commit
        # stops being read, and the reason nobody reads was the only thing the
        # refusal bought.
        #
        # `--unverified` still writes this row's reason when the caller gives one,
        # because a caller can say whose red it is and which run will cover the
        # commit. `detail` is the checker's verdict, and it can only say what
        # state the record is in.
        observed_debt.append(
            ("unverified", "verify-status is not FRESH-green: " + (detail or "unknown"))
        )
        print(
            "verification debt: ze-precommit-verify is not FRESH-green ("
            + (detail or "unknown")
            + ").\n"
            "  The commit proceeds and owes one debt row. --push refuses while that\n"
            "  row is open. Clear it with `make ze-verify-debt-clear`, which re-runs\n"
            "  the gate and writes `cleared` only on exit 0.",
            file=sys.stderr,
        )
    # Full-verify coverage: did a FULL `make ze-precommit-verify` run after the
    # last Go edit this commit carries? This is a different question from the one
    # the verify record answers above, and it earns a row of its own for that
    # reason: one gate says a run went red, the other says no run happened at
    # all. Both are verification debt, and since 2026-08-21 both record a row and
    # let the commit through (ai/rules/git-safety.md, "Verify a Commit, Not the
    # Working Tree"). The gate costs 25 to 53 minutes, so refusing here made a
    # session batch its finished work until one run was worth the wait, which is
    # the accumulation the directive exists to stop.
    #
    # Removals count: deleting a .go file changes what the tree builds, and at
    # `create` time the file is still on disk (the script runs the git rm), so
    # its mtime reads like any other -- old, hence covered by an existing run.
    #
    # The coverage question is asked whether or not --missing-full-verify-ok was
    # given: the flag writes the row's reason, and it does not decide whether the
    # run happened.
    go_files = go_paths_in(add_paths + remove_paths)
    if go_files:
        cstate, cdetail = full_verify_coverage(repo, go_files)
        if cstate == "covered":
            print(f"full-verify coverage: {cdetail}")
        elif cstate == "uncovered":
            observed_debt.append(("missing_full_verify_ok", cdetail))
            print(
                "verification debt: this commit carries Go and no full "
                "ze-precommit-verify covers it (" + cdetail + ").\n"
                "  The commit proceeds and owes one debt row. --push refuses while that\n"
                "  row is open. Clear it with `make ze-verify-debt-clear`, which runs the\n"
                "  full gate (25-53 min, foreground, and it takes a repo-wide lock).",
                file=sys.stderr,
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
    # Commit-time repo-state BLOCK gates (deferral log, journal rows, spec
    # closure audit, test weakenings).
    gate_problems = commit_gate_problems(repo, add_paths, remove_paths)
    if gate_problems:
        raise UsageError("\n\n".join(gate_problems))
    # Test-coverage gate: a commit carrying non-exempt Go carries a test too.
    # It sits behind its own flag rather than inside commit_gate_problems, the
    # way the RFC gate below does, because the escape is a per-commit judgement
    # rather than a gate anything can re-run afterwards.
    no_test_reason = (getattr(args, "no_test", None) or "").strip()
    if not no_test_reason:
        coverage_problems = test_coverage_problems(repo, add_paths)
        if coverage_problems:
            raise UsageError(
                "\n\n".join(coverage_problems)
                + '\n  ... or pass --no-test "<why this commit needs none>" to'
                " commit anyway."
            )
    else:
        print(f"no-test: {no_test_reason}", file=sys.stderr)
    # RFC-tagged-change gate: a test that proves an RFC obligation is the
    # evidence behind a public compliance claim, so the owner approves every
    # behavior change to one and test/rfc-changed.md is where that approval is
    # recorded. It sits behind its own flag rather than inside
    # commit_gate_problems, the way the review gate does.
    if not (args.rfc_change_ok or "").strip():
        rfc_problems = rfc_changed_problems(repo, add_paths, remove_paths)
        if rfc_problems:
            raise UsageError(
                "\n\n".join(rfc_problems)
                + '\n  ... or pass --rfc-change-ok "<who approved it, and when>"'
                " to commit anyway."
            )
    # Critical-review gate: a spec cannot close without an INDEPENDENT review of
    # its code that is fresh (hash-pinned) and clean. This makes review the
    # central, unskippable step -- it cannot be satisfied by narrating "0 issues"
    # into the spec. See ai/rules/planning.md.
    if not args.review_override:
        review_problems = review_gate_problems(repo, add_paths, remove_paths)
        if review_problems:
            raise UsageError("\n\n".join(review_problems))
    # For a (non-overridden) spec-closure commit, re-run the review gate inside the
    # generated script so an edit between `create` and `bash tmp/commit-*.sh` is
    # caught at commit-RUN time (TOCTOU), not only here at generation time.
    review_check = ""
    closure_stem = spec_closure_stem(add_paths, remove_paths, repo)
    if closure_stem is not None and not args.review_override:
        rc_code = tuple(p for p in add_paths if _is_review_code(p))
        review_check = (
            "python3 scripts/dev/review_gate.py check --spec "
            + shlex.quote(closure_stem)
            + " --files"
            + ("" if not rc_code else " " + quote_paths(rc_code))
        )
    # Verification debt. Every gate that has not run green over this commit
    # becomes one row -- the ones the run MEASURED (`observed_debt`) and the ones
    # an override flag declared. The shard rides along in the commit so the
    # obligation reaches the session that eventually pushes.
    # Recorded AFTER the gates, so the shard itself can never trip one.
    owed = debt_owed(args, observed_debt)
    debt_rel: str | None = None
    if owed and not args.dry_run:
        debt_rel = record_debt(repo, session, args.subject, owed)
        if debt_rel not in add_paths:
            add_paths = add_paths + (debt_rel,)
    # A push publishes to users, which is the one place the debt has to be paid.
    # Refused here rather than at commit time, because a commit that stays local
    # costs nobody anything and a commit that never happens costs the work.
    if push_reason is not None:
        blocking = [(shard, row) for shard, row, _ in open_debt_rows(repo)] + [
            (debt_rel or debt_shard_path(session), f"| (this commit) | {gate} |")
            for gate, _ in owed
        ]
        if blocking:
            raise UsageError(
                "refusing --push: "
                + str(len(blocking))
                + " open verification-debt row(s). A commit that stays local is\n"
                "  free; a push is what reaches users, so the gates are owed here.\n"
                + "\n".join(f"    {shard}: {row}" for shard, row in blocking[:10])
                + ("\n    ..." if len(blocking) > 10 else "")
                + "\n  Run `make ze-verify-debt-clear`: it re-runs each owed gate and\n"
                "  writes `cleared` on exit 0. A row is never turned by hand. Then push\n"
                "  again. Prepare the commit WITHOUT --push to land the work now and\n"
                "  clear the debt in a later session."
            )
    msg = message_text(args.subject, args.body)
    msg_path = message_rel_path(session, tag)
    block = CommitBlock(
        tag,
        args.subject.strip(),
        add_paths,
        remove_paths,
        msg_path,
        review_check,
    )
    script, authorisation = write_outputs(
        repo,
        session,
        block,
        msg,
        args.append,
        args.replace,
        args.dry_run,
        getattr(args, "script", None),
        push_reason,
    )
    if not args.dry_run:
        print(f"session={session}")
        print(f"message={msg_path}")
        print(f"script={script.relative_to(repo).as_posix()}")
        # The STATE is always what is printed. `--unverified` annotates the debt
        # row, and a flag must never rename the record's state in the output the
        # session reads back.
        if args.unverified:
            print(f"verify={vstate.upper()} (unverified: {args.unverified})")
        else:
            print(f"verify={vstate.upper()} ({detail})")
        if args.review_override and spec_closure_stem(add_paths, remove_paths, repo):
            print(f"review=OVERRIDDEN ({args.review_override})")
        if debt_rel:
            print(f"debt={len(owed)} row(s) -> {debt_rel}")
            for gate, _ in owed:
                print(f"  owed: {gate}")
        # Printed whenever the SCRIPT will push, including when this create only
        # appended a block to a script an earlier --push authorised: what the
        # caller needs to know is what the script they are about to run does.
        if authorisation is not None:
            print(f"push=AUTHORISED ({authorisation})")
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
            "committed that references not-yet-committed work. Run `make ze-generated-files-update`\n"
            "  once the tree is coherent and commit the fix.",
            file=sys.stderr,
        )
    reminder = closure_reminder(add_paths, remove_paths, repo)
    if reminder:
        print(reminder, file=sys.stderr)
    # Commit-time WARN gates (plugin .go without .ci, doc drift). Advisory:
    # printed to stderr, the commit still proceeds.
    for warning in commit_gate_warnings(repo, add_paths, remove_paths):
        print("warning: " + warning, file=sys.stderr)
    return 0


def print_session(args: argparse.Namespace) -> int:
    repo = repo_root(args.repo)
    print(session_id(repo, args.session))
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

    debt = sub.add_parser(
        "debt-clear",
        help="re-run the gate every open verification-debt row names, and set "
        "that row to `cleared` only when the gate exits 0",
    )
    debt.set_defaults(func=clear_debt)

    create_cmd = sub.add_parser(
        "create",
        help="write one commit message file and one commit script; the printed "
        "`script=` line is the only authoritative path",
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
        "--script",
        help="existing script to write into, taken from the `script=` line an "
        "earlier create printed. Required with --append when this session has "
        "more than one prepared script, and the only way to target a script for "
        "--replace. Never reconstruct this path by convention: it carries a "
        "random suffix so a guess cannot hit another agent's prepared commit",
    )
    create_cmd.add_argument(
        "--append",
        action="store_true",
        help="append this commit block to an existing script (see --script)",
    )
    create_cmd.add_argument(
        "--replace",
        action="store_true",
        help="replace the script named by --script. Refused when that script was "
        "prepared for an unrelated file set. Without --script every create "
        "already gets its own new script, so nothing needs replacing",
    )
    create_cmd.add_argument(
        "--broken-head-fix",
        help="reason to allow a commit while ze-repository-tracked-build-check is red, i.e. "
        "HEAD itself does not compile. Use when THIS commit lands the producer a "
        "previous commit left behind. Accepted only when tracked-build is the ONLY "
        "structural red, so it can never wave through a lint, tier or wiring failure. "
        "It clears the STRUCTURAL refusal and nothing else. The stale verify record "
        "needs no flag: it records a debt row on its own",
    )
    create_cmd.add_argument(
        "--unverified",
        help="OPTIONAL. The reason cell for the row a not-FRESH-green verify record "
        "already writes: whose red it is, and which run will cover this commit. It "
        "unlocks nothing, because nothing is locked -- the commit lands either way "
        "and --push refuses while the row is open. Without it the row carries the "
        "checker's own verdict",
    )
    create_cmd.add_argument(
        "--structural-red-ok",
        help="reason this commit lands while a deterministic structural gate is red in "
        "the verify record. This one DOES unlock a refusal, and it is the only flag "
        "that reaches it: a broken tree is a different fact from an unverified one. "
        "Use when the red belongs to another session's in-flight work and this commit "
        "cannot affect it. SELF-SERVICE and RECORDED: the reason is echoed with the "
        "red gate names and becomes an open row in "
        "plan/verification-debt/<session>.md that --push refuses to publish over",
    )
    create_cmd.add_argument(
        "--missing-full-verify-ok",
        help="OPTIONAL. The reason cell for the row a commit carrying Go already "
        "writes when no full ze-precommit-verify ran after its last Go edit. It is "
        "a separate row from --unverified because it answers a separate question: "
        "that one explains a RED run, this one covers a run that never happened. "
        "--push refuses while either row is open",
    )
    create_cmd.add_argument(
        "--stale-index-ok",
        help="reason this commit lands while a generated discovery index "
        "(ai/PACKAGE-MAP.md, ai/LEARNED-FULL-INDEX.md) is stale or omitted. "
        "SELF-SERVICE and RECORDED: it becomes an open row in "
        "plan/verification-debt/<session>.md that --push refuses to publish over",
    )
    create_cmd.add_argument(
        "--no-test",
        help="reason this commit carries non-exempt Go and no test path. "
        "SELF-SERVICE: give a truthful reason and proceed -- a pure rename, a "
        "comment fix, a refactor whose behaviour is unchanged. Deliberately NOT "
        "a verification-debt row: every gate a row names can be re-run, and "
        "'this commit carried no test' cannot be re-judged once the commit "
        "exists. The reason is echoed to stderr, so it lands in the transcript",
    )
    create_cmd.add_argument(
        "--rfc-change-ok",
        help="OWNER APPROVAL ONLY: reason this commit changes an RFC-tagged test that "
        "test/rfc-changed.md does not name. The normal route is the row, because a row "
        "reaches the reader of git history and a flag does not. Give WHO approved the "
        "change and WHEN. RECORDED: it becomes an open row in "
        "plan/verification-debt/<session>.md that --push refuses to publish over",
    )
    create_cmd.add_argument(
        "--review-override",
        help="reason a spec-closure commit lands while the independent critical-review "
        "gate (ai/rules/planning.md) is missing or stale. RECORDED: it becomes an "
        "open row in plan/verification-debt/<session>.md that --push refuses to "
        "publish over. A review not performed is never a clean tree, so this one is "
        "owed loudly rather than waved through",
    )
    create_cmd.add_argument(
        "--push",
        metavar="AUTHORISATION",
        help="OWNER INSTRUCTION ONLY: append a push to the end of the script, so "
        "the commits it prepares are published only when every one of them "
        "succeeded. Never a default and never inferred: pass it when the "
        "owner has ordered the push, and give WHO ordered it and WHEN as the "
        "value, which is recorded in the script and echoed as `push=AUTHORISED`. "
        "One push per script: an --append moves it after the new block",
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
