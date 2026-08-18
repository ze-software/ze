#!/usr/bin/env python3
"""Generate safe user-run commit scripts for Ze."""

from __future__ import annotations

import argparse
import datetime
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
from journal import journal_row_cells

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
        raise UsageError(f"--subject must be at most {COMMIT_MESSAGE_WIDTH} characters")
    parts = [cleaned_subject]
    cleaned_body = wrap_commit_body(body)
    if cleaned_body:
        parts.extend(("", cleaned_body))
    return "\n".join(parts) + "\n"


def learned_paths(paths: tuple[str, ...]) -> tuple[str, ...]:
    return tuple(path for path in paths if LEARNED_RE.match(path))


SPEC_PATH_RE = re.compile(r"^plan/spec-.+\.md$")


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
    """
    has_learned = bool(learned_paths(add_paths))
    has_journal = bool(
        repo is not None
        and [
            stem
            for stem in _journal_added_spec_stems(repo, add_paths, remove_paths)
            if not _spec_closed_earlier(repo, stem)
        ]
    )
    if not has_learned and not has_journal:
        return None
    if any(SPEC_PATH_RE.match(path) for path in remove_paths):
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


# Deterministic structural gates in `make ze-precommit-verify` (the non-test stages in
# scripts/status/verify_run.go stagesForMode). Unlike the unit/functional/exabgp
# TEST stages, these NEVER fail for flaky or environmental reasons: a red means
# the tree is structurally broken -- a module-tier misplacement, a lint or vet
# violation, a broken plugin boundary, an unresolved iface, a failed feature-tag
# type-check, a stale generated file, a stale wiring index, or a HEAD that does
# not compile (ze-repository-tracked-build-check). They are therefore NOT eligible to be parked
# in plan/known-failures/
# or waved through with --unverified. Every name here MUST be a stage that
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


def structural_gate_reds(repo: Path) -> list[str]:
    """Structural-gate stages recorded red by the last `make ze-precommit-verify` run.

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
        if st.get("exit-code", 0) != 0 and st.get("stage") in STRUCTURAL_GATES:
            reds.append(st["stage"])
    return reds


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
        diff = git("diff", "--cached", "--no-color", "-U0", *extra_args).stdout
    finally:
        try:
            os.unlink(index)
        except OSError:
            pass
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
    with it. This function contributes the commit's own paths and nothing else.

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
    accepts (owner, 2026-08-16; AC-10). The delegate is asked for the weakenings
    first and for the row judgement second, so a commit that weakens nothing
    still runs one comparison and reads no file.
    """
    tests = tuple(p for p in add_paths if check_weakened_tests.is_test_path(p))
    removed = tuple(p for p in remove_paths if check_weakened_tests.is_test_path(p))
    if not tests and not removed:
        return []
    weakened, errors = check_weakened_tests.weakened_tests(
        str(repo), tests, removed=removed
    )
    if errors:
        return errors  # the comparison did not happen, so nothing is accepted
    if not weakened:
        return []  # AC-5: no weakening, so no row is owed and none is read
    problems = check_weakened_tests.weakened_problems(str(repo), tests, removed=removed)
    if check_weakened_tests.WEAKENED_PATH not in add_paths:
        problems.append(
            f"this commit weakens {len(weakened)} test(s) and does not carry "
            f"{check_weakened_tests.WEAKENED_PATH}. The row is in the working "
            f"tree only, so git history would hold the weakening with no reason "
            f"beside it. Name the file too:\n"
            f"    --file {check_weakened_tests.WEAKENED_PATH}"
        )
    return problems


def commit_gate_problems(
    repo: Path, add_paths: tuple[str, ...], remove_paths: tuple[str, ...]
) -> list[str]:
    """All BLOCK-severity commit-time gates, in one call for create()."""
    problems: list[str] = []
    problems += deferral_in_diff_problems(repo, add_paths, remove_paths)
    problems += deferral_shard_removal_problems(repo, add_paths, remove_paths)
    problems += journal_row_problems(repo, add_paths, remove_paths)
    problems += spec_audit_problems(repo, add_paths, claimed_spec(repo), remove_paths)
    problems += ste_problems(repo, add_paths)
    problems += weakened_problems(repo, add_paths, remove_paths)
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
    for path, line in _prospective_added_lines(repo, add_paths, remove_paths):
        if path not in scope:
            continue
        if journal_row_cells(line[1:]) == [JOURNAL_MALFORMED]:
            bad.append(f"  {path}: {line[1:].strip()!r}")
    if not bad:
        return []
    return [
        "this commit adds journal row(s) that do not hold the five cells\n"
        "  | Date | Spec | Surface | Symptom | Fix |, starting with `|`:\n"
        + "\n".join(bad)
        + "\n  A row this parser cannot read carries no Spec cell, so the closure\n"
        "  stem is unknown and the review gate stops firing on the commit that\n"
        "  holds the code. Fix the row (plan/journal/README.md)."
    ]


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
    Drop that gate and this becomes a fail-open branch again.

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
            spec = cells[1]
            if spec and spec != "-" and spec not in stems:
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


# --- Verification debt -------------------------------------------------------
#
# A commit override says "this gate did not run green over this commit, and I am
# committing anyway". Before this ledger those overrides were waved through with
# a reason nobody could find again, and the alternative -- refusing the commit --
# is worse: several sessions share this checkout, the record is red for somebody
# else's in-flight work nearly always, and a commit that never lands is the most
# expensive failure this repo has (`ai/rules/rule-precedence.md`).
#
# So the override is allowed and the OBLIGATION is written down. What is recorded
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

# Each override flag, and the gate whose absence it is admitting.
DEBT_FLAGS = (
    ("unverified", "ze-precommit-verify (not FRESH-green)"),
    ("structural_red_ok", "ze-precommit-verify structural gates (red)"),
    ("missing_full_verify_ok", "full ze-precommit-verify over this commit's Go"),
    ("stale_index_ok", "discovery-index freshness"),
    ("review_override", "independent critical review"),
    ("broken_head_fix", "ze-repository-tracked-build-check (HEAD does not compile)"),
)

DEBT_HEADER = (
    "| Date | Session | Subject | Gate owed | Reason | Status |",
    "|------|---------|---------|-----------|--------|--------|",
)


def debt_shard_path(session: str) -> str:
    return f"{VERIFICATION_DEBT_DIR}/{session}.md"


def debt_owed(args: argparse.Namespace) -> list[tuple[str, str]]:
    """(gate, reason) for every override this create used. Order is DEBT_FLAGS."""
    owed: list[tuple[str, str]] = []
    for attr, gate in DEBT_FLAGS:
        reason = (getattr(args, attr, None) or "").strip()
        if reason:
            owed.append((gate, reason))
    return owed


def _debt_cell(text: str) -> str:
    """One table cell: no newline, no pipe, so a reason cannot forge a row."""
    return " ".join(text.replace("|", "/").split())


def record_debt(
    repo: Path, session: str, subject: str, owed: list[tuple[str, str]]
) -> str:
    """Append one row per owed gate to this session's shard. Returns its path."""
    rel = debt_shard_path(session)
    path = repo / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    stamp = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%d")
    lines: list[str] = []
    if not path.exists():
        lines += [
            f"# Verification debt -- commit session {session}",
            "",
            "Gates that had not run green over these commits when they were made.",
            "Each row is work owed, not a defect: a defect is fixed, never recorded",
            "(`ai/rules/completion.md`). Clear a row by running the gate over the",
            "committed code and setting Status to `cleared`, or delete the shard once",
            "every row is cleared. `scripts/dev/commit_helper.py create --push` refuses",
            "while any row here is open.",
            "",
            *DEBT_HEADER,
        ]
    for gate, reason in owed:
        lines.append(
            f"| {stamp} | {session} | {_debt_cell(subject)} | {_debt_cell(gate)} "
            f"| {_debt_cell(reason)} | open |"
        )
    with path.open("a", encoding="utf-8") as fh:
        fh.write("\n".join(lines) + "\n")
    return rel


def open_debt_rows(repo: Path) -> list[tuple[str, str]]:
    """(shard, row) for every row whose Status cell is `open`.

    Reads the tracked shards, so it answers the same question in any session on
    this checkout -- which is the point: the session that pushes is rarely the
    session that owed the gate.
    """
    rows: list[tuple[str, str]] = []
    root = repo / VERIFICATION_DEBT_DIR
    if not root.is_dir():
        return rows
    for shard in sorted(root.glob("*.md")):
        try:
            text = shard.read_text(encoding="utf-8")
        except OSError:
            continue
        for line in text.splitlines():
            cells = [c.strip() for c in line.split("|")]
            # A rendered row is `| a | b | c | d | e | f |` -> 8 split cells.
            if len(cells) != 8 or cells[-2].lower() != "open":
                continue
            rows.append((shard.name, line.strip()))
    return rows


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

    One commit CAN add rows naming two specs (an edit to an older row beside the
    closure row, or two classes touched at once), and only then is this session's
    claim consulted: the artifact says a closure happened, the claim says which
    one is this session's. It is a tie-break and never an override, because a
    commit prepared with no claim, or by a session claiming another spec, must
    still be recognised as the closure it is. With no claim among the stems the
    first still answers, so the gate fires on SOMETHING rather than nothing.

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
    for p in remove_paths:
        m = _SPEC_STEM_RE.match(p)
        if m:
            return m.group("stem")
    if repo is not None:
        stems = [
            stem
            for stem in _journal_added_spec_stems(repo, add_paths, remove_paths)
            if not _spec_closed_earlier(repo, stem)
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
    # Verify gate: a commit script must not be prepared over a non-green verify
    # unless the caller explicitly acknowledges why (owner override, or a
    # known-red logged in plan/known-failures/). This turns "verify before
    # commit" from honor-system into an enforced, overridable gate. See
    # ai/rules/git-safety.md.
    vstate, detail = verify_status(repo)
    if vstate == "stale":
        # A DETERMINISTIC STRUCTURAL GATE red (tier/lint/vet/plugin-boundary/
        # iface-resolution/regen-check-readonly/wiring-docs/tracked-build) is
        # never flaky or environmental: the tree is structurally broken. Such a red is
        # NOT bypassable by --unverified or a plan/known-failures/ known-red
        # (those cover flaky TEST stages only). This closes the hole that let a
        # misplaced-tier gate (routeinstall) be parked as "pre-existing" and
        # shipped red on main. See ai/rules/git-safety.md.
        gate_reds = structural_gate_reds(repo)
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
                "ze-precommit-verify has a DETERMINISTIC STRUCTURAL GATE red that "
                "--unverified cannot bypass: " + ", ".join(gate_reds) + ".\n"
                "  Structural gates (tier/lint/vet/plugin-boundary/iface-resolution/\n"
                "  regen-check-readonly/wiring-docs/tracked-build) never fail for flaky\n"
                "  or environmental reasons -- a red means the tree is structurally\n"
                "  broken. They are NOT eligible for --unverified or a\n"
                "  plan/known-failures/ known-red.\n"
                + (
                    "  "
                    + TRACKED_BUILD_GATE
                    + " is red: HEAD ITSELF does not compile,\n"
                    "  usually because a commit took a consumer and left its producer\n"
                    "  uncommitted. That red is cleared BY a commit, not before one. If THIS\n"
                    '  commit lands the missing producer, pass --broken-head-fix "<reason>"\n'
                    '  (and --unverified "<reason>" as well: the verify record is not green,\n'
                    "  and a reason written about HEAD does not speak for anything else).\n"
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
        # --structural-red-ok acknowledges a strictly WORSE condition than
        # --unverified (a red structural gate, not a flaky test), so it satisfies
        # this check too rather than demanding both flags for one decision.
        # --broken-head-fix is deliberately NOT in this list. It answers one
        # question -- "may the commit that repairs a broken HEAD be prepared" --
        # and `verify_status` goes stale for flaky test reds and for age as well.
        # Letting a reason written about HEAD wave those through would make it a
        # self-service --unverified. The fixing commit passes both flags.
        if not args.unverified and not (args.structural_red_ok or "").strip():
            raise UsageError(
                "ze-precommit-verify is not FRESH-green ("
                + (detail or "unknown")
                + ").\n"
                "  Run `make ze-precommit-verify` (or `make ze-precommit-verify-changed`) until green, then\n"
                '  commit, OR pass --unverified "<reason>" to commit anyway (owner\n'
                "  override, or a flaky/environmental known-red logged in\n"
                "  plan/known-failures/; structural gates are never eligible)."
            )
    # Full-verify coverage gate (owner directive, 2026-08-17): a commit carrying
    # Go must be preceded by a FULL `make ze-precommit-verify` that ran after the
    # last Go edit. This is a different question from the verify-status gate
    # above, which asks whether the record is GREEN. In a shared checkout the
    # record is red for another session's in-flight work nearly every time, and
    # the directive says to take that code as working -- so --unverified is
    # passed on almost every commit and stopped being evidence that the gate ran
    # at all. Hence a separate flag: --unverified explains a red, and this one
    # answers "did the full run happen over YOUR code". See ai/rules/git-safety.md.
    # Removals count: deleting a .go file changes what the tree builds, and at
    # `create` time the file is still on disk (the script runs the git rm), so
    # its mtime reads like any other -- old, hence covered by an existing run.
    go_files = go_paths_in(add_paths + remove_paths)
    if go_files and (args.missing_full_verify_ok or "").strip():
        # LOUD on purpose: a silent bypass makes an unverified Go commit
        # indistinguishable from a verified one in the transcript.
        print(
            "WARNING: committing Go with no full ze-precommit-verify over it.\n"
            "  Owner override: " + args.missing_full_verify_ok.strip(),
            file=sys.stderr,
        )
    elif go_files:
        cstate, cdetail = full_verify_coverage(repo, go_files)
        if cstate == "uncovered":
            raise UsageError(
                "this commit carries Go and no full ze-precommit-verify covers it: "
                + cdetail
                + ".\n"
                "  Owner directive (2026-08-17): a commit carrying .go, go.mod, go.sum or\n"
                "  vendor/ is preceded by a full `make ze-precommit-verify`. Its RED stages\n"
                "  do not block this commit -- a failure in code another session has\n"
                "  uncommitted is taken as working, named in --unverified, and committed.\n"
                "  What blocks it is never having run the gate over your own code.\n"
                "  Run `make ze-precommit-verify` (25-30 min, foreground, and it takes a\n"
                "  repo-wide lock), then prepare the commit, OR pass\n"
                '  --missing-full-verify-ok "<reason>" (owner override only).'
            )
        if cstate == "covered":
            print(f"full-verify coverage: {cdetail}")
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
    # Verification debt. Every override used above admits a gate that has not run
    # green over this commit; each becomes one row, and the shard rides along in
    # the commit so the obligation reaches the session that eventually pushes.
    # Recorded AFTER the gates, so the shard itself can never trip one.
    owed = debt_owed(args)
    debt_rel: str | None = None
    if owed and not args.dry_run:
        debt_rel = record_debt(repo, session, args.subject, owed)
        if debt_rel not in add_paths:
            add_paths = add_paths + (debt_rel,)
    # A push publishes to users, which is the one place the debt has to be paid.
    # Refused here rather than at commit time, because a commit that stays local
    # costs nobody anything and a commit that never happens costs the work.
    if push_reason is not None:
        blocking = open_debt_rows(repo) + [
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
                + "\n  Run the named gate over the committed code, set that row's Status\n"
                "  to `cleared`, and push again. Prepare the commit WITHOUT --push to\n"
                "  land the work now and clear the debt in a later session."
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
        if args.unverified:
            print(f"verify=UNVERIFIED ({args.unverified})")
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
    for warning in commit_gate_warnings(repo, add_paths):
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
        "It clears the STRUCTURAL refusal and nothing else: pass --unverified as "
        "well, since the verify record is not green either",
    )
    create_cmd.add_argument(
        "--unverified",
        help="reason this commit lands while ze-precommit-verify is not FRESH-green. "
        "SELF-SERVICE: give a truthful reason and proceed -- do not stop to ask. "
        "The gate is not skipped, it is OWED: one row lands in "
        "plan/verification-debt/<session>.md and --push refuses while it is open",
    )
    create_cmd.add_argument(
        "--structural-red-ok",
        help="reason this commit lands while a deterministic structural gate is red in "
        "the verify record. Deliberately separate from --unverified so the flaky-test "
        "path can never reach it. Use when the red belongs to another session's "
        "in-flight work and this commit cannot affect it. SELF-SERVICE and RECORDED: "
        "the reason is echoed with the red gate names and becomes an open row in "
        "plan/verification-debt/<session>.md that --push refuses to publish over",
    )
    create_cmd.add_argument(
        "--missing-full-verify-ok",
        help="reason to prepare a commit carrying Go when no full ze-precommit-verify ran "
        "after the last Go edit. Deliberately separate from --unverified, which "
        "explains a RED run: in a shared checkout that flag is passed on nearly every "
        "commit, so it cannot also certify that the run happened. SELF-SERVICE and "
        "RECORDED: the missing coverage becomes an open row in "
        "plan/verification-debt/<session>.md that --push refuses to publish over",
    )
    create_cmd.add_argument(
        "--stale-index-ok",
        help="reason this commit lands while a generated discovery index "
        "(ai/PACKAGE-MAP.md, ai/LEARNED-FULL-INDEX.md) is stale or omitted. "
        "SELF-SERVICE and RECORDED: it becomes an open row in "
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
