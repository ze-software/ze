#!/usr/bin/env python3
"""The ONE session-id resolver for the .claude/hooks harness.

Every hook that names a per-session marker under tmp/session/ (.lsp-loaded-<sid>,
.lsp-invoked-<sid>, .source-read-<sid>, .session-<sid>) or the session directory
holding the digest (tmp/session/<YYYY-MM-DD>-<sid>/state/session-state-*.md)
resolves the <sid> HERE, and only here. Three faces:

  * an importable ``session_id()`` for in-process Python callers;
  * ``--hook-session-id``, which validates the raw ``session_id`` string in hook
    JSON from stdin before shell command substitution can normalize it; and
  * a default ``__main__`` that prints the resolved id for Bash callers through
    .claude/hooks/lib/session-id.sh (``_session_id``).

There is deliberately no second copy. Three independent derivations used to exist
(Bash, Python-hook, commit_helper) and drifted for weeks despite a prose "MUST stay
identical" invariant; a disagreement fails CLOSED -- the reader looks for a marker
nothing wrote and blocks work already done (incident 2026-07-16). One source of
truth is the fix (spec-fixit-session-id-collision).

Precedence:

  1. $CLAUDE_CODE_SESSION_ID when it is already present in this process.
     SessionStart accepts it from the validated hook payload. PreToolUse Bash
     prefixes a validated parent payload id for restricted fork commands that do
     not receive the SessionStart environment file.
  2. --session-id from the process tree -- the CLI's own flag, present only when the
     CLI was launched with it (an interactive `claude` has none). /proc on Linux,
     `ps` on macOS/BSD.
  3. CLAUDE_CODE_SESSION_ACCESS_TOKEN JWT session_id claim, when present (empty for
     subscription auth).
  4. A UUID minted once and cached at tmp/session/.sid-by-pid-<clipid>-<starttime>,
     keyed by the long-lived CLI-ancestor PID AND that process's start time.
     Per-session unique. It is stable across short-lived subprocesses when the CLI
     ancestry can be inspected. Restricted forks that cannot inspect ancestry must
     use the hook payload before they reach this fallback.

An id from any source is used only when it is a non-dot filename component
([A-Za-z0-9._-]); anything else falls through rather than being rewritten, so the
Bash and Python entry points cannot disagree on the marker path.
"""

from __future__ import annotations

import base64
import json
import os
import re
import subprocess
import uuid

# An id is used only when it is usable as a filename component. Reject rather than
# rewrite, so two callers cannot silently diverge on the resulting marker path.
_SID_SAFE_RE = re.compile(r"\A[A-Za-z0-9._-]+\Z")

# A cache-key part is a filename component too, so anything outside that charset is
# squeezed rather than carried through: `ps -o lstart=` prints "Mon Aug  3 00:59:30
# 2026", spaces included. This applies to the cache FILE name only. The id itself is
# still rejected rather than rewritten (_sid_safe).
_TOKEN_UNSAFE_RE = re.compile(r"[^A-Za-z0-9._-]+")


def _sid_safe(sid: object) -> str:
    """Return sid when it is a non-dot filename component, else ''."""
    if not isinstance(sid, str) or sid in ("", ".", ".."):
        return ""
    return sid if _SID_SAFE_RE.match(sid) else ""


def _path_token(value: str) -> str:
    """Squeeze value into a filename component, or '' when nothing usable is left."""
    return _TOKEN_UNSAFE_RE.sub("_", value.strip()).strip("_")


def _project_dir() -> str:
    """The repo root: $CLAUDE_PROJECT_DIR, else three levels up from this file."""
    return os.environ.get("CLAUDE_PROJECT_DIR") or os.path.abspath(
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "..")
    )


def _session_dir() -> str:
    return os.path.join(_project_dir(), "tmp", "session")


def _ps(field: str, pid: int) -> str:
    try:
        return subprocess.run(
            ["ps", "-o", field, "-p", str(pid)], capture_output=True, text=True
        ).stdout.strip()
    except Exception:
        return ""


def _ppid(pid: int) -> str:
    """Parent pid of pid, via /proc (Linux) or ps (macOS/BSD)."""
    status = f"/proc/{pid}/status"
    if os.path.isfile(status):
        try:
            with open(status) as fh:
                for ln in fh:
                    if ln.startswith("PPid:"):
                        return ln.split()[1]
        except Exception:
            return ""
        return ""
    return _ps("ppid=", pid).strip()


def _pargv(pid: int) -> list[str]:
    """argv of pid, via /proc (Linux) or ps (macOS/BSD)."""
    cmdline = f"/proc/{pid}/cmdline"
    if os.path.isfile(cmdline):
        try:
            with open(cmdline, "rb") as fh:
                raw = fh.read()
            return [a.decode("utf-8", "replace") for a in raw.split(b"\0") if a]
        except Exception:
            return []
    return _ps("command=", pid).split()


def _pcomm(pid: int) -> str:
    """Command name of pid, via /proc (Linux) or ps (macOS/BSD)."""
    comm = f"/proc/{pid}/comm"
    if os.path.isfile(comm):
        try:
            with open(comm) as fh:
                return fh.read().strip()
        except Exception:
            return ""
    return _ps("comm=", pid)


def _pstart(pid: int) -> str:
    """Start time of pid as a filename token, via /proc (Linux) or ps (macOS/BSD).

    Linux: field 22 of /proc/<pid>/stat, the start time in clock ticks since boot.
    The scan begins after the LAST ')' because field 2 is the parenthesised command
    name and can itself hold spaces and parentheses, which a plain split() mis-counts.
    macOS/BSD: `ps -o lstart=`, whose "Mon Aug  3 00:59:30 2026" becomes a token.

    Returns '' when neither source answers.
    """
    stat = f"/proc/{pid}/stat"
    if os.path.isfile(stat):
        try:
            with open(stat) as fh:
                raw = fh.read()
            # Field 3 (state) is the first one after the command name, so field 22
            # sits at index 19 of the remainder.
            return _path_token(raw[raw.rindex(")") + 1 :].split()[19])
        except Exception:
            return ""
    return _path_token(_ps("lstart=", pid))


def _session_id_from_argv(argv: list[str]) -> str:
    """Return the value following --session-id in an argv list, or ''."""
    for i, a in enumerate(argv):
        if a == "--session-id" and i + 1 < len(argv):
            return argv[i + 1]
        if a.startswith("--session-id="):
            return a.split("=", 1)[1]
    return ""


def _sid_from_env() -> str:
    """A safe UUID already present in this process, or '' (source 1)."""
    return _sid_safe(os.environ.get("CLAUDE_CODE_SESSION_ID", ""))


def _sid_from_process_tree() -> str:
    """The CLI's own --session-id, read up the process tree, or '' (source 2)."""
    pid = os.getpid()
    while pid > 1:
        try:
            sid = _sid_safe(_session_id_from_argv(_pargv(pid)))
            if sid:
                return sid
            ppid = _ppid(pid)
            if not ppid or not ppid.isdigit():
                break
            pid = int(ppid)
        except Exception:
            break
    return ""


def _sid_from_jwt() -> str:
    """session_id from CLAUDE_CODE_SESSION_ACCESS_TOKEN, or '' (source 3)."""
    tok = os.environ.get("CLAUDE_CODE_SESSION_ACCESS_TOKEN")
    if not tok:
        return ""
    try:
        payload = tok.split(".")[1].replace("_", "/").replace("-", "+")
        mod = len(payload) % 4
        if mod == 2:
            payload += "=="
        elif mod == 3:
            payload += "="
        decoded = base64.b64decode(payload).decode("utf-8", "replace")
        m = re.search(r'"session_id":\s*"([^"]*)"', decoded)
        if m and m.group(1):
            return _sid_safe(m.group(1))
    except Exception:
        pass
    return ""


def _is_cli(pid: int) -> bool:
    """True when pid looks like the Claude CLI process.

    The CLI's comm/argv0 is not reliably the literal `claude`: it can be a
    version-path basename (e.g. .../versions/2.1.206) or a node launcher. Match
    `claude` as the basename of comm, OR as an exact path COMPONENT of any argv
    element -- so /Users/.../bin/claude and .../claude/versions/2.1.206 both match,
    while `--model claude-opus-...` does NOT (an exact component, not a substring,
    avoids that false positive).
    """
    if _pcomm(pid).rsplit("/", 1)[-1] == "claude":
        return True
    for a in _pargv(pid):
        if "claude" in a.split("/"):
            return True
    return False


def _cli_ancestor_pid() -> int:
    """PID of the long-lived Claude CLI ancestor, used only as a CACHE KEY.

    Walk the process tree and return the nearest ancestor that looks like the CLI
    (_is_cli: `claude` as comm basename or an argv path component). When ancestry
    is visible, it is stable across short-lived subprocesses and distinct between
    concurrent top-level sessions. The PID is never itself the id. _cache_key
    pairs it with the process start time, so a reused PID cannot alias the dead
    session's id or markers.

    If no ancestor looks like the CLI, use the topmost process that the walk can
    inspect. A restricted fork that cannot inspect parent processes can get a
    different key in each subprocess. SubagentStart and PreToolUse Bash carry the
    validated parent payload id so restricted Bash commands do not depend on this
    fallback.
    """
    pid = os.getpid()
    top = pid
    for _ in range(256):
        if pid <= 1:
            break
        if _is_cli(pid):
            return pid
        top = pid
        ppid = _ppid(pid)
        if not ppid or not ppid.isdigit():
            break
        pid = int(ppid)
    return top


def _read_cached(path: str) -> str:
    """First line of a cache file when usable as a filename component, else ''."""
    try:
        with open(path) as fh:
            return _sid_safe(fh.readline().strip())
    except OSError:
        return ""


def _cache_key(clipid: int) -> str:
    """Cache key for the minted id: the CLI-ancestor PID AND its start time.

    A PID alone is reusable. The kernel hands the same number to a new process once
    the old one is gone, and a session keyed on it would read the DEAD session's
    cached id -- adopting its spec claim and its gate markers (incidents 1162, 1246).
    Pairing the PID with the start time makes the entry self-invalidating: a reused
    PID carries a different start time, so the stale entry is never looked up again
    and needs no expiry to retire it.

    'unknown' when neither /proc nor `ps` answers. That is the pre-2026-08-10 PID-only
    key, and the alias window with it. Both supported platforms answer, and spelling
    the degraded case out keeps it visible in `ls tmp/session/`.
    """
    return f"{clipid}-{_pstart(clipid) or 'unknown'}"


def _mint_cached(cache_dir: str, key: str) -> str:
    """Return a per-key UUID from cache_dir, minting on first miss (source 4).

    Stable: the same (cache_dir, key) always yields the same id. Unique: a
    distinct key yields a distinct minted UUID. A cache hit refreshes the file
    mtime. Nothing ages the file out, so the touch dates the entry for an
    operator.

    The published cache file is NEVER empty or partial: the id is written to a
    per-pid temp file and atomically os.replace()d into place, so a reader always
    sees either the previous file or the fully-written new one. A plain O_EXCL create
    leaves an EMPTY file visible between create and the separate write; a reader (or
    the writer after a crash there) would then read "", fall through, and mint a fresh
    id on every call -- the session never matches its own markers, and nothing sweeps
    the poison away. Treating an empty/garbage cache as a miss and overwriting it atomically
    heals that poison. Residual (acceptable for a last-resort fallback): two source-4
    subprocesses of the SAME session that both miss at the same instant may
    os.replace() different ids; last writer wins and the cache is stable from the next
    call on.
    """
    cache = os.path.join(cache_dir, f".sid-by-pid-{key}")
    existing = _read_cached(cache)
    if existing:
        try:
            os.utime(cache, None)
        except OSError:
            pass
        return existing
    # Miss, OR a poisoned (empty/garbage) cache: mint and atomically replace.
    try:
        os.makedirs(cache_dir, exist_ok=True)
    except OSError:
        pass
    minted = str(uuid.uuid4())
    tmp = os.path.join(cache_dir, f".sid-by-pid-{key}.{os.getpid()}.tmp")
    try:
        with open(tmp, "w") as fh:
            fh.write(minted + "\n")
        os.replace(tmp, cache)
    except OSError:
        try:
            os.unlink(tmp)
        except OSError:
            pass
        return minted
    return _read_cached(cache) or minted


def _sid_minted() -> str:
    return _mint_cached(_session_dir(), _cache_key(_cli_ancestor_pid()))


def session_id() -> str:
    """Resolve this session's id (see module docstring for the four sources)."""
    return (
        _sid_from_env() or _sid_from_process_tree() or _sid_from_jwt() or _sid_minted()
    )


def _hook_session_id_status(payload: object) -> tuple[int, str]:
    """Return the hook payload status and its safe raw session_id, if present."""
    if not isinstance(payload, dict):
        return 2, ""
    if "session_id" not in payload:
        return 1, ""
    hook_sid = _sid_safe(payload["session_id"])
    return (0, hook_sid) if hook_sid else (2, "")


def _hook_session_id(payload: object) -> str:
    """Return a hook payload's raw session_id only when the validator accepts it."""
    return _hook_session_id_status(payload)[1]


if __name__ == "__main__":
    import sys

    # Hook payload mode validates the decoded JSON string before a shell reads it
    # through command substitution. That order matters because a shell removes
    # trailing newlines from command output.
    if len(sys.argv) == 2 and sys.argv[1] == "--hook-session-id":
        try:
            hook_status, hook_sid = _hook_session_id_status(json.load(sys.stdin))
        except Exception:
            hook_status, hook_sid = 2, ""
        if hook_status == 0:
            sys.stdout.write(hook_sid + "\n")
        sys.exit(hook_status)
    # `--safe <value>` validates a caller-supplied id instead of resolving one.
    # Payload consumers use --hook-session-id so JSON type and raw-value checks
    # happen before the value crosses a shell boundary.
    elif len(sys.argv) == 3 and sys.argv[1] == "--safe":
        print(_sid_safe(sys.argv[2]))
    else:
        print(session_id())
