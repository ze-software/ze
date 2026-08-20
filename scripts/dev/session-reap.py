#!/usr/bin/env python3
"""Remove the session directories of Claude sessions that are no longer running.

`tmp/session/<YYYY-MM-DD>-<sid>/` holds a session's binaries, its seeded etc/ze
store, its scratch and its per-spec state digest (scripts/dev/session-scratch.sh).
Nothing ages those directories out, by owner decision (2026-08-03): an age timer
deletes the operator's data unasked, because a directory dated last week can
belong to a session that is still open in a terminal right now.

This reaper asks a different question, and it is the question that decision left
open: is the session that owns this directory STILL RUNNING? A directory whose
owner exited holds nothing anybody can return to. The date it carries is not
evidence either way, so no date is read here.

`make clean` runs it, and `make ze-session-reap` is the operator's direct route.

WHAT COUNTS AS ALIVE (four sources, and every one of them KEEPS)

  1. This process's own session, read from the environment or the process tree
     (.claude/hooks/lib/session_id.py). The command runs from inside a session,
     so that session is alive by construction.
     `make clean` removes this session's own directory through
     session-scratch.sh --clean, which is a separate and deliberate step.
  2. A `.sid-by-pid-<pid>-<start>` marker whose pid is running AND whose start
     time still matches. The pair is what makes the pin safe: a reused pid
     carries a different start time, so a dead session's pin cannot adopt a live
     process (the same reasoning as _cache_key in session_id.py).
  3. A sid that appears anywhere in a running process's argv. The CLI's own
     `--session-id <uuid>` lands here, and so does the `export
     CLAUDE_CODE_SESSION_ID=<uuid>` prefix the PreToolUse hook puts on a
     restricted Bash command.
  4. A transcript under $CLAUDE_CONFIG_DIR/projects/*/<sid>.jsonl modified at or
     after the START of the OLDEST running Claude CLI process.

Source 4 is what covers an idle session, and its bound is a proof rather than a
guess. A running CLI process P writes its transcript when the session starts, so
mtime(P) >= start(P), and start(P) >= min(start) over every running CLI, which
is the cutoff. So every running session clears the cutoff whatever it is doing
now. The cost of the bound is in the other direction: a session that exited
minutes ago also clears it and survives one more sweep. That is the direction
this file errs in on purpose.

FAIL CLOSED

When a Claude CLI is running and the transcript root does not exist, source 4
cannot answer and an idle session would look dead. Nothing is removed in that
case: the script says why and exits 0. With no CLI running at all there is
nothing for source 4 to keep, so its absence is not a blind spot.

Usage (run from anywhere; paths below are relative to the checkout root):

    scripts/dev/session-reap.py             # reap, print one summary line
    scripts/dev/session-reap.py --dry-run   # name what would go, remove nothing

Run: python3 scripts/dev/session_reap_test.py
(also picked up automatically by TestPythonUnitTests, scripts/dev/python_tests_test.go)
"""

from __future__ import annotations

import argparse
import importlib.util
import os
import pathlib
import re
import shutil
import subprocess
import sys
import time

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
CHECKOUT = SCRIPT_DIR.parent.parent
HOOK_LIB = CHECKOUT / ".claude" / "hooks" / "lib" / "session_id.py"

# The ONE root for per-session state, spelled the same way in mk/session.mk and
# .claude/hooks/lib/session-dir.sh.
SESSION_ROOT = pathlib.Path("tmp/session")

# A session directory: <YYYY-MM-DD>-<sid>. The DATED SHAPE is the bound on what
# can be a candidate, exactly as in the ze-session-clean recipe, so a flat marker
# file beside them is never one.
CANDIDATE_RE = re.compile(r"\A\d{4}-\d{2}-\d{2}-(?P<sid>[A-Za-z0-9._-]+)\Z")

# `ps -o etime=` prints [[dd-]hh:]mm:ss. Only used when `etimes` is unsupported.
ETIME_RE = re.compile(r"\A(?:(?:(\d+)-)?(\d+):)?(\d+):(\d+)\Z")

MARKER_PREFIX = ".sid-by-pid-"


def _session_id_module():
    """Import .claude/hooks/lib/session_id.py, the ONE session-id resolver.

    Imported rather than copied: this script needs the same _is_cli, _pargv and
    _pstart the hooks use, and a fourth independent derivation of session
    identity is the drift that spec-fixit-session-id-collision removed.
    """
    spec = importlib.util.spec_from_file_location("ze_session_id", HOOK_LIB)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"cannot load {HOOK_LIB}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def _project_dir() -> pathlib.Path:
    """The checkout root: $CLAUDE_PROJECT_DIR, else this script's own checkout."""
    return pathlib.Path(os.environ.get("CLAUDE_PROJECT_DIR") or CHECKOUT)


def _config_dir() -> pathlib.Path:
    """The Claude configuration root that holds projects/<slug>/<sid>.jsonl."""
    return pathlib.Path(
        os.environ.get("CLAUDE_CONFIG_DIR") or os.path.expanduser("~/.claude")
    )


def _running_pids() -> list[int]:
    """Every pid this user can see, via /proc (Linux) or `ps` (macOS/BSD)."""
    proc = pathlib.Path("/proc")
    if proc.is_dir():
        return sorted(int(p.name) for p in proc.iterdir() if p.name.isdigit())
    out = subprocess.run(["ps", "-eo", "pid="], capture_output=True, text=True).stdout
    return [int(t) for t in out.split() if t.isdigit()]


def _age_seconds(pid: int) -> int | None:
    """Seconds since pid started, or None when neither ps format answers.

    `etimes` is the direct answer and procps has it; BSD `ps` does not, so
    `etime` and its [[dd-]hh:]mm:ss are parsed as the fallback.
    """
    out = subprocess.run(
        ["ps", "-o", "etimes=", "-p", str(pid)], capture_output=True, text=True
    ).stdout.strip()
    if out.isdigit():
        return int(out)
    out = subprocess.run(
        ["ps", "-o", "etime=", "-p", str(pid)], capture_output=True, text=True
    ).stdout.strip()
    m = ETIME_RE.match(out)
    if not m:
        return None
    days, hours, minutes, seconds = (int(g or 0) for g in m.groups())
    return ((days * 24 + hours) * 60 + minutes) * 60 + seconds


def _oldest_cli_start(pids: list[int], sid_mod) -> float | None:
    """Epoch seconds at which the OLDEST running Claude CLI started, or None.

    None means no CLI process is running, which is the answer source 4 needs to
    distinguish "nothing to keep" from "cannot tell".
    """
    ages = [_age_seconds(p) for p in pids if sid_mod._is_cli(p)]
    ages = [a for a in ages if a is not None]
    if not ages:
        return None
    return time.time() - max(ages)


def _argv_text(pids: list[int], sid_mod) -> str:
    """Every running process's argv, joined, for the source-3 membership test."""
    return "\n".join("\0".join(sid_mod._pargv(p)) for p in pids)


def _pinned_sids(root: pathlib.Path, sid_mod) -> tuple[set[str], list[pathlib.Path]]:
    """Source 2: sids pinned to a live (pid, start) pair, and the stale pins.

    A pin whose pid is gone, or whose pid now carries a different start time,
    keeps nothing and is itself litter, so it is returned for removal.
    """
    live: set[str] = set()
    stale: list[pathlib.Path] = []
    for marker in sorted(root.glob(f"{MARKER_PREFIX}*")):
        if not marker.is_file() or marker.name.endswith(".tmp"):
            continue
        pid, _, start = marker.name[len(MARKER_PREFIX) :].partition("-")
        if pid.isdigit() and start and sid_mod._pstart(int(pid)) == start:
            try:
                sid = sid_mod._sid_safe(marker.read_text().splitlines()[0].strip())
            except (OSError, IndexError):
                sid = ""
            if sid:
                live.add(sid)
            continue
        stale.append(marker)
    return live, stale


def _transcript_sids(config_dir: pathlib.Path, cutoff: float) -> set[str]:
    """Source 4: sids whose transcript was written at or after cutoff."""
    live: set[str] = set()
    for jsonl in (config_dir / "projects").glob("*/*.jsonl"):
        try:
            if jsonl.stat().st_mtime >= cutoff:
                live.add(jsonl.stem)
        except OSError:
            continue
    return live


def _flat_markers(root: pathlib.Path, sids: set[str]) -> list[pathlib.Path]:
    """Flat marker files keyed by one of sids (.lsp-loaded-<sid>, and friends).

    Matched on a `-<sid>` SUFFIX, so `.closure-ack-<spec-stem>` -- which is keyed
    by a spec, not a session -- can never match one.
    """
    if not sids:
        return []
    hits = []
    for entry in sorted(root.iterdir()):
        if not entry.is_file() or entry.name.startswith(MARKER_PREFIX):
            continue
        if any(entry.name.endswith(f"-{sid}") for sid in sids):
            hits.append(entry)
    return hits


def reap(dry_run: bool = False, out=sys.stdout) -> int:
    root = _project_dir() / SESSION_ROOT
    if not root.is_dir():
        print(f"session-reap: no {SESSION_ROOT}, nothing to do", file=out)
        return 0

    sid_mod = _session_id_module()
    candidates: dict[str, pathlib.Path] = {}
    for entry in sorted(root.iterdir()):
        m = CANDIDATE_RE.match(entry.name)
        if m and entry.is_dir():
            candidates[m.group("sid")] = entry

    pids = _running_pids()
    live: set[str] = set()

    # Source 1. The two sources that only READ are used, never session_id(),
    # whose last source MINTS an id and caches it at .sid-by-pid-<pid>-<start>:
    # a cleanup that leaves a new marker behind every time a human runs it is
    # producing the litter it came to remove. An empty answer means off-session,
    # and off-session there is no own directory to protect.
    own = sid_mod._sid_from_env() or sid_mod._sid_from_process_tree()
    if own:
        live.add(own)

    pinned, stale_pins = _pinned_sids(root, sid_mod)
    live |= pinned  # source 2

    argv_text = _argv_text(pids, sid_mod)
    live |= {sid for sid in candidates if sid in argv_text}  # source 3

    cutoff = _oldest_cli_start(pids, sid_mod)
    if cutoff is not None:
        config_dir = _config_dir()
        if not (config_dir / "projects").is_dir():
            print(
                f"session-reap: a Claude CLI is running but {config_dir}/projects "
                "does not exist, so an idle session cannot be told from a dead "
                "one. Removed nothing.",
                file=out,
            )
            return 0
        live |= _transcript_sids(config_dir, cutoff)  # source 4

    dead = {sid: d for sid, d in candidates.items() if sid not in live}
    markers = _flat_markers(root, set(dead)) + stale_pins

    if dry_run:
        for path in sorted(list(dead.values()) + markers):
            print(path, file=out)

    removed_dirs = 0
    for path in dead.values():
        if not dry_run:
            shutil.rmtree(path, ignore_errors=True)
        removed_dirs += 1
    removed_markers = 0
    for path in markers:
        if not dry_run:
            try:
                path.unlink()
            except OSError:
                continue
        removed_markers += 1

    verb = "Would remove" if dry_run else "Removed"
    kept = len(candidates) - removed_dirs
    print(
        f"session-reap: {verb} {removed_dirs} dead session "
        f"{'directory' if removed_dirs == 1 else 'directories'} and "
        f"{removed_markers} marker{'' if removed_markers == 1 else 's'}; "
        f"kept {kept} running.",
        file=out,
    )
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="name what would be removed, remove nothing",
    )
    return reap(dry_run=parser.parse_args().dry_run)


if __name__ == "__main__":
    sys.exit(main())
