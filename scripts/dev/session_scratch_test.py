#!/usr/bin/env python3
"""Unit tests for per-session tmp/ scratch isolation.

Covers scripts/dev/session-scratch.sh (creates tmp/s/<session-id>/, and --reap
removes dead sessions' dirs) and .claude/hooks/session-end-scratch.sh (removes
only this session's dir at SessionEnd, keyed on the JSON `reason`/`session_id`).
Everything runs inside a throwaway git repo so the tests never touch the real
tmp/. The scripts source .claude/hooks/lib/session-id.sh by a path relative to
the script (the REAL repo), while resolving the working root from
$CLAUDE_PROJECT_DIR or the caller's cwd (the throwaway repo) -- which is exactly
why this isolation works.
"""

from __future__ import annotations

import json
import os
import subprocess
import tempfile
import time
import unittest
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
HELPER = REPO / "scripts" / "dev" / "session-scratch.sh"
END_HOOK = REPO / ".claude" / "hooks" / "session-end-scratch.sh"


def _init_repo(root: Path) -> None:
    subprocess.run(["git", "-C", str(root), "init", "-q"], check=True)


def _helper_env(sid: str | None, project_dir: Path | None) -> dict:
    env = dict(os.environ)
    # This process runs inside a real session; drop the inherited project dir so
    # the helper resolves the root from the throwaway repo unless a test sets it.
    env.pop("CLAUDE_PROJECT_DIR", None)
    if sid is not None:
        env["CLAUDE_CODE_SESSION_ID"] = sid  # strategy 1 wins: no /proc walk
    if project_dir is not None:
        env["CLAUDE_PROJECT_DIR"] = str(project_dir)
    return env


def _run_helper(
    root: Path,
    sid: str,
    *args: str,
    project_dir: Path | None = None,
    check: bool = True,
) -> subprocess.CompletedProcess:
    return subprocess.run(
        ["bash", str(HELPER), *args],
        cwd=str(root),
        env=_helper_env(sid, project_dir),
        capture_output=True,
        text=True,
        check=check,
    )


def _run_end_hook(root: Path, payload: dict) -> None:
    env = dict(os.environ)
    env["CLAUDE_PROJECT_DIR"] = str(root)
    subprocess.run(
        ["bash", str(END_HOOK)],
        cwd=str(root),
        env=env,
        input=json.dumps(payload),
        capture_output=True,
        text=True,
        check=True,
    )


class TestSessionScratchHelper(unittest.TestCase):
    def test_creates_per_session_dir(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            out = _run_helper(root, "sid-AAA").stdout.strip()
            self.assertEqual(out, "tmp/s/sid-AAA")
            self.assertTrue((root / "tmp" / "s" / "sid-AAA").is_dir())

    def test_distinct_sessions_get_distinct_dirs(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            a = _run_helper(root, "sid-AAA").stdout.strip()
            b = _run_helper(root, "sid-BBB").stdout.strip()
            self.assertNotEqual(a, b)
            self.assertTrue((root / a).is_dir())
            self.assertTrue((root / b).is_dir())

    def test_path_flag_prints_without_creating(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            out = _run_helper(root, "sid-CCC", "--path").stdout.strip()
            self.assertEqual(out, "tmp/s/sid-CCC")
            self.assertFalse((root / "tmp" / "s" / "sid-CCC").exists())

    def test_project_dir_takes_precedence_over_cwd(self):
        # The helper must resolve the same root the SessionEnd hook uses
        # ($CLAUDE_PROJECT_DIR), even when invoked from a different cwd.
        with (
            tempfile.TemporaryDirectory() as proj,
            tempfile.TemporaryDirectory() as elsewhere,
        ):
            proot = Path(proj)
            _init_repo(proot)
            _init_repo(Path(elsewhere))
            out = _run_helper(
                Path(elsewhere), "sid-DDD", project_dir=proot
            ).stdout.strip()
            self.assertEqual(out, "tmp/s/sid-DDD")
            self.assertTrue((proot / "tmp" / "s" / "sid-DDD").is_dir())
            self.assertFalse((Path(elsewhere) / "tmp" / "s" / "sid-DDD").exists())

    def test_dot_ids_are_refused(self):
        # A hostile CLAUDE_CODE_SESSION_ID must not let the helper escape tmp/s/.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            for bad in ("..", "."):
                proc = _run_helper(root, bad, check=False)
                self.assertNotEqual(proc.returncode, 0, f"id {bad!r} should be refused")
            # Nothing created above tmp/s (e.g. no tmp/ materialized via '..').
            self.assertFalse((root / "tmp" / "s").exists())

    def test_clean_removes_only_own_dir(self):
        # `make clean` runs --clean: it removes THIS session's dir and nothing
        # else (no shared caches, no sibling sessions), and prints nothing.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            mine = root / "tmp" / "s" / "sid-CLEAN"
            other = root / "tmp" / "s" / "other"
            shared_cache = root / "tmp" / "go-cache"  # outside tmp/s -> must survive
            mine.mkdir(parents=True)
            other.mkdir(parents=True)
            shared_cache.mkdir(parents=True)
            (mine / "x.log").write_text("scratch")
            (shared_cache / "c").write_text("cached")
            proc = _run_helper(root, "sid-CLEAN", "--clean")
            self.assertEqual(proc.stdout.strip(), "")
            self.assertFalse(mine.exists())
            self.assertTrue(other.exists())
            self.assertTrue(shared_cache.exists())


class TestSessionScratchReap(unittest.TestCase):
    def test_reap_removes_inactive_and_keeps_active(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            dead = root / "tmp" / "s" / "dead"
            live = root / "tmp" / "s" / "live"
            dead.mkdir(parents=True)
            live.mkdir(parents=True)
            (dead / "old.log").write_text("x")
            (live / "recent.log").write_text("y")
            old = time.time() - 25 * 3600
            # Backdate every entry of the dead dir (file then dir) beyond 24h.
            os.utime(dead / "old.log", (old, old))
            os.utime(dead, (old, old))
            _run_helper(root, "ignored", "--reap", project_dir=root)
            self.assertFalse(dead.exists(), "inactive dir should be reaped")
            self.assertTrue(live.exists(), "active dir must survive")

    def test_reap_keeps_dir_with_recent_file_but_old_dir_mtime(self):
        # Locks in the fix: reap keys on file activity, not the dir's own mtime.
        # An append/overwrite does not bump the parent dir's mtime, so a
        # dir-mtime reaper would wrongly delete this live dir; the recursive
        # find keeps it because a file inside is recent.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            live = root / "tmp" / "s" / "live-old-dir"
            live.mkdir(parents=True)
            (live / "active.log").write_text("recent")
            old = time.time() - 25 * 3600
            os.utime(live, (old, old))  # backdate ONLY the dir; file stays recent
            _run_helper(root, "ignored", "--reap", project_dir=root)
            self.assertTrue(
                live.exists(),
                "a dir with a recent file must survive even if its own mtime is old",
            )

    def test_reap_noop_without_scratch_root(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            # No tmp/s at all: --reap must succeed and create nothing.
            _run_helper(root, "ignored", "--reap", project_dir=root)
            self.assertFalse((root / "tmp" / "s").exists())


class TestSessionEndScratchHook(unittest.TestCase):
    def test_removes_only_own_dir(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            (root / "tmp" / "s" / "mine").mkdir(parents=True)
            (root / "tmp" / "s" / "other").mkdir(parents=True)
            _run_end_hook(root, {"session_id": "mine", "reason": "clear"})
            self.assertFalse((root / "tmp" / "s" / "mine").exists())
            self.assertTrue((root / "tmp" / "s" / "other").exists())

    def test_logout_also_deletes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            (root / "tmp" / "s" / "mine").mkdir(parents=True)
            _run_end_hook(root, {"session_id": "mine", "reason": "logout"})
            self.assertFalse((root / "tmp" / "s" / "mine").exists())

    def test_resume_keeps_scratch(self):
        # Uses the REAL SessionEnd field name ("reason"); a resumed session keeps
        # its id, so its scratch must survive.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            (root / "tmp" / "s" / "mine").mkdir(parents=True)
            _run_end_hook(root, {"session_id": "mine", "reason": "resume"})
            self.assertTrue((root / "tmp" / "s" / "mine").exists())

    def test_empty_sid_is_noop(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            (root / "tmp" / "s" / "keep").mkdir(parents=True)
            _run_end_hook(root, {"session_id": "", "reason": "clear"})
            self.assertTrue((root / "tmp" / "s" / "keep").exists())

    def test_traversal_sid_cannot_escape(self):
        # A path-bearing id must never delete anything outside tmp/s/.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            (root / "tmp" / "s").mkdir(parents=True)
            (root / "tmp" / "victim").mkdir()
            _run_end_hook(root, {"session_id": "../victim", "reason": "clear"})
            self.assertTrue((root / "tmp" / "victim").exists())


class TestSessionBinaryReaping(unittest.TestCase):
    """Session-suffixed binaries (mk/session.mk) are swept with their session.

    They live in bin/, not under tmp/s/<sid>/, because a binary's location
    decides where ze resolves its config and database
    (internal/core/paths/paths.go ConfigDirFromBinary) and the repository's
    etc/ze holds live state. Being outside the reaped directory, they need
    sweeping by name -- otherwise bin/ accumulates one full binary set per dead
    session, which is the mess the per-session root exists to prevent.
    """

    @staticmethod
    def _seed(root: Path, sid: str) -> None:
        (root / "bin").mkdir(exist_ok=True)
        (root / "bin" / f"ze-{sid}").write_text("x")
        (root / "bin" / f"ze-test-{sid}").write_text("x")
        # Shared binaries a human or CI built: must never be candidates.
        (root / "bin" / "ze").write_text("x")
        (root / "bin" / "ze-test").write_text("x")

    def test_clean_removes_this_sessions_binaries_only(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            sid = "sid-clean"
            _run_helper(root, sid)
            self._seed(root, sid)
            (root / "bin" / "ze-other-session").write_text("x")

            _run_helper(root, sid, "--clean")

            self.assertFalse((root / "bin" / f"ze-{sid}").exists())
            self.assertFalse((root / "bin" / f"ze-test-{sid}").exists())
            self.assertTrue((root / "bin" / "ze").exists(), "shared ze deleted")
            self.assertTrue((root / "bin" / "ze-test").exists())
            self.assertTrue((root / "bin" / "ze-other-session").exists())

    def test_reap_spares_a_recently_built_binary(self):
        """A binary is not recreated on demand the way scratch is.

        --reap treats 24h of scratch inactivity as "session died". Applying that
        to binaries alone would delete the binary of a live session that simply
        had not written scratch lately, and nothing would rebuild it.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            sid = "sid-idle-scratch"
            scratch = root / "tmp" / "s" / sid
            scratch.mkdir(parents=True)
            (scratch / "old.log").write_text("x")
            stale = time.time() - (48 * 3600)
            os.utime(scratch / "old.log", (stale, stale))
            os.utime(scratch, (stale, stale))
            self._seed(root, sid)  # binaries are fresh

            _run_helper(root, sid, "--reap")

            self.assertFalse(scratch.exists(), "stale scratch should be reaped")
            self.assertTrue(
                (root / "bin" / f"ze-{sid}").exists(),
                "freshly built binary reaped despite idle scratch",
            )

    def test_reap_removes_a_stale_binary(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            sid = "sid-dead"
            scratch = root / "tmp" / "s" / sid
            scratch.mkdir(parents=True)
            (scratch / "old.log").write_text("x")
            self._seed(root, sid)
            stale = time.time() - (48 * 3600)
            for p in (
                scratch / "old.log",
                scratch,
                root / "bin" / f"ze-{sid}",
                root / "bin" / f"ze-test-{sid}",
            ):
                os.utime(p, (stale, stale))

            _run_helper(root, sid, "--reap")

            self.assertFalse(scratch.exists())
            self.assertFalse((root / "bin" / f"ze-{sid}").exists())
            self.assertFalse((root / "bin" / f"ze-test-{sid}").exists())
            self.assertTrue((root / "bin" / "ze").exists(), "shared ze deleted")

    def test_session_end_hook_removes_this_sessions_binaries(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            sid = "sid-end"
            (root / "tmp" / "s" / sid).mkdir(parents=True)
            self._seed(root, sid)

            _run_end_hook(root, {"session_id": sid, "reason": "clear"})

            self.assertFalse((root / "tmp" / "s" / sid).exists())
            self.assertFalse((root / "bin" / f"ze-{sid}").exists())
            self.assertTrue((root / "bin" / "ze").exists(), "shared ze deleted")

if __name__ == "__main__":
    unittest.main()
