#!/usr/bin/env python3
"""The ONE session-id resolver for the .claude/hooks harness.

Every hook that names a per-session marker under tmp/session/ (.lsp-loaded-<sid>,
.lsp-invoked-<sid>, .source-read-<sid>, .session-<sid>, session-state-<sid>.md)
resolves the <sid> HERE, and only here. Two faces:

  * an importable ``session_id()`` for the in-process Python callers
    (.claude/hooks/pretool-writeedit.py, scripts/dev/commit_helper.py); and
  * a ``__main__`` that prints the id, for the Bash callers, which reach it through
    the one-line shim .claude/hooks/lib/session-id.sh (``_session_id``).

There is deliberately no second copy. Three independent derivations used to exist
(Bash, Python-hook, commit_helper) and drifted for weeks despite a prose "MUST stay
identical" invariant; a disagreement fails CLOSED -- the reader looks for a marker
nothing wrote and blocks work already done (incident 2026-07-16). One source of
truth is the fix (spec-fixit-session-id-collision).

Precedence:

  1. $CLAUDE_CODE_SESSION_ID -- the session UUID the CLI exports into every child
     process, so it reaches each short-lived hook subprocess for free: no walk, no
     decode. Subagents and forks inherit the PARENT session's value deliberately: a
     fork must see the fail-closed markers its parent wrote.
  2. --session-id from the process tree -- the CLI's own flag, present only when the
     CLI was launched with it (an interactive `claude` has none). /proc on Linux,
     `ps` on macOS/BSD.
  3. CLAUDE_CODE_SESSION_ACCESS_TOKEN JWT session_id claim, when present (empty for
     subscription auth).
  4. A UUID minted once and cached at tmp/session/.sid-by-pid-<clipid>, keyed by the
     long-lived CLI-ancestor PID. Per-session unique AND stable across the many
     short-lived hook subprocesses -- it replaces the old shared constant
     "claude-session-fallback", which every concurrent session collided on.

An id from any source is used only when it is safe as a filename component
([A-Za-z0-9._-]); anything else falls through rather than being rewritten, so the
Bash and Python entry points cannot disagree on the marker path.
"""

from __future__ import annotations

import base64
import os
import re
import subprocess
import uuid

# An id is used only when it is usable as a filename component. Reject rather than
# rewrite, so two callers cannot silently diverge on the resulting marker path.
_SID_SAFE_RE = re.compile(r"\A[A-Za-z0-9._-]+\Z")


def _sid_safe(sid: str) -> str:
    """Return sid when it is usable as a filename component, else ''."""
    return sid if sid and _SID_SAFE_RE.match(sid) else ""


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


def _session_id_from_argv(argv: list[str]) -> str:
    """Return the value following --session-id in an argv list, or ''."""
    for i, a in enumerate(argv):
        if a == "--session-id" and i + 1 < len(argv):
            return argv[i + 1]
        if a.startswith("--session-id="):
            return a.split("=", 1)[1]
    return ""


def _sid_from_env() -> str:
    """This session's UUID as exported by the CLI, or '' (source 1)."""
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
    (_is_cli: `claude` as comm basename or an argv path component). It is stable
    across a session's short-lived hook subprocesses and distinct between concurrent
    top-level sessions. The PID is never itself the id, only the cache key, so PID
    reuse cannot alias a stale marker set once the cache file ages out.

    Fallback: if NO ancestor looks like the CLI, key on the topmost walkable
    ancestor. Known limitation (source-4 only): two sessions launched from ONE shell,
    neither carrying a `claude` marker anywhere in the ancestry, would share that top
    ancestor and mint ONE id. This is the irreducible floor of a process-tree-only
    key. It cannot bite while the CLI exports CLAUDE_CODE_SESSION_ID (source 1, always
    set on current CLIs), which resolves before source 4 is ever reached.
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


def _mint_cached(cache_dir: str, clipid: int) -> str:
    """Return a per-clipid UUID from cache_dir, minting on first miss (source 4).

    Stable: the same (cache_dir, clipid) always yields the same id, so every
    short-lived hook subprocess of one session resolves identically. Unique:
    distinct clipid yields a distinct minted UUID, so concurrent sessions never
    share a marker set. A cache hit refreshes the file mtime so a live session's
    cache does not age out from under it.

    The published cache file is NEVER empty or partial: the id is written to a
    per-pid temp file and atomically os.replace()d into place, so a reader always
    sees either the previous file or the fully-written new one. A plain O_EXCL create
    leaves an EMPTY file visible between create and the separate write; a reader (or
    the writer after a crash there) would then read "", fall through, and mint a fresh
    id on every call -- the session never matches its own markers until the 24h
    cleanup. Treating an empty/garbage cache as a miss and overwriting it atomically
    heals that poison. Residual (acceptable for a last-resort fallback): two source-4
    subprocesses of the SAME session that both miss at the same instant may
    os.replace() different ids; last writer wins and the cache is stable from the next
    call on.
    """
    cache = os.path.join(cache_dir, f".sid-by-pid-{clipid}")
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
    tmp = os.path.join(cache_dir, f".sid-by-pid-{clipid}.{os.getpid()}.tmp")
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
    return _mint_cached(_session_dir(), _cli_ancestor_pid())


def session_id() -> str:
    """Resolve this session's id (see module docstring for the four sources)."""
    return (
        _sid_from_env() or _sid_from_process_tree() or _sid_from_jwt() or _sid_minted()
    )


if __name__ == "__main__":
    print(session_id())
