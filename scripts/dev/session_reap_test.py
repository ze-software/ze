#!/usr/bin/env python3
"""A session directory is removed only when its session is provably gone.

scripts/dev/session-reap.py is the one thing in the repository that deletes
another session's directory, and that directory holds the binaries a session is
running tests against plus the state/ digest a compaction reads back. So the
property under test is not "it cleans up". It is that every signal of life KEEPS,
and that a directory goes only when all four are silent.

Each case drives the real script as a subprocess against a temporary tree
($CLAUDE_PROJECT_DIR) and a temporary transcript root ($CLAUDE_CONFIG_DIR), so
the code under test is the code `make clean` runs.

A fake long-lived process named `claude` is spawned for the cases that need a
cutoff to exist. It is a copy of /bin/sleep, so the CLI test in session_id.py
(_is_cli: `claude` as the comm basename) matches it, and nothing else about it
matters. The real Claude processes on the machine running this test are visible
to the script too; they only ever push the cutoff EARLIER, which keeps more, so
no case here depends on the machine being idle.

Run: python3 scripts/dev/session_reap_test.py
(also picked up automatically by TestPythonUnitTests, scripts/dev/python_tests_test.go)
"""

import importlib.util
import os
import pathlib
import shutil
import subprocess
import tempfile
import time
import unittest
import uuid

ROOT = pathlib.Path(__file__).resolve().parents[2]
REAP = ROOT / "scripts" / "dev" / "session-reap.py"
HOOK_LIB = ROOT / ".claude" / "hooks" / "lib" / "session_id.py"

# Older than any plausible Claude uptime, so a transcript stamped with it is
# behind the cutoff whatever else is running on this machine.
LONG_AGO = time.time() - 400 * 24 * 3600


def _session_id_module():
    spec = importlib.util.spec_from_file_location("ze_session_id_probe", HOOK_LIB)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


SID_MOD = _session_id_module()


class ReapCase(unittest.TestCase):
    """A temporary checkout, a temporary transcript root, and one fake CLI."""

    def setUp(self):
        self.tmp = pathlib.Path(tempfile.mkdtemp(prefix="ze-session-reap-"))
        self.addCleanup(shutil.rmtree, self.tmp, ignore_errors=True)

        self.project = self.tmp / "checkout"
        self.sessions = self.project / "tmp" / "session"
        self.sessions.mkdir(parents=True)

        self.config = self.tmp / "claude-home"
        self.transcripts = self.config / "projects" / "a-project"
        self.transcripts.mkdir(parents=True)

        # The id this invocation resolves for ITSELF (source 1). Pinned to a
        # value no case uses as a session, so self-protection never masks a
        # result, and set explicitly so the resolver never falls through to
        # minting one into the temporary tree.
        self.own_sid = str(uuid.uuid4())

        # A `claude` the script will see as running, so a cutoff always exists.
        # Its own directory, because the NAME is the whole point: session_id.py
        # matches the comm basename, so nothing else in the tree may be called
        # `claude` and shadow it.
        (self.tmp / "bin").mkdir()
        self.fake_cli = self.tmp / "bin" / "claude"
        shutil.copy("/bin/sleep", self.fake_cli)
        self.cli = subprocess.Popen(
            [str(self.fake_cli), "600"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        self.addCleanup(self._stop_cli)

    def _stop_cli(self):
        self.cli.terminate()
        try:
            self.cli.wait(timeout=10)
        except subprocess.TimeoutExpired:
            self.cli.kill()

    # -- fixture helpers ----------------------------------------------------

    def session(self, sid, date="2026-01-01"):
        """Create a session directory holding one file, and return its path."""
        d = self.sessions / f"{date}-{sid}"
        (d / "bin").mkdir(parents=True)
        (d / "bin" / "ze").write_text("binary", encoding="utf-8")
        return d

    def transcript(self, sid, mtime):
        f = self.transcripts / f"{sid}.jsonl"
        f.write_text("{}\n", encoding="utf-8")
        os.utime(f, (mtime, mtime))
        return f

    def run_reap(self, *args, config=None):
        env = dict(os.environ)
        env["CLAUDE_PROJECT_DIR"] = str(self.project)
        env["CLAUDE_CONFIG_DIR"] = str(config or self.config)
        env["CLAUDE_CODE_SESSION_ID"] = self.own_sid
        proc = subprocess.run(
            ["python3", str(REAP), *args],
            capture_output=True,
            text=True,
            env=env,
            cwd=str(ROOT),
        )
        self.assertEqual(proc.returncode, 0, proc.stderr)
        return proc.stdout

    # -- the property -------------------------------------------------------

    def test_dead_session_is_removed(self):
        """No signal of life, so the directory goes."""
        sid = str(uuid.uuid4())
        d = self.session(sid)
        self.transcript(sid, LONG_AGO)
        self.run_reap()
        self.assertFalse(d.exists())

    def test_recent_transcript_keeps_the_session(self):
        """Source 4: a transcript at or after the oldest CLI's start keeps it.

        This is the case that covers an idle session -- one with no subprocess
        running right now and no pid pinned to it, which every other source is
        blind to.
        """
        sid = str(uuid.uuid4())
        d = self.session(sid)
        self.transcript(sid, time.time())
        self.run_reap()
        self.assertTrue(d.is_dir())
        self.assertTrue((d / "bin" / "ze").exists())

    def test_live_pid_pin_keeps_the_session(self):
        """Source 2: a .sid-by-pid marker whose pid and start time still match."""
        sid = str(uuid.uuid4())
        d = self.session(sid)
        self.transcript(sid, LONG_AGO)
        pid = self.cli.pid
        pin = self.sessions / f".sid-by-pid-{pid}-{SID_MOD._pstart(pid)}"
        pin.write_text(sid + "\n", encoding="utf-8")

        self.run_reap()

        self.assertTrue(d.is_dir())
        self.assertTrue(pin.exists())

    def test_pin_with_a_reused_pid_keeps_nothing_and_is_swept(self):
        """A pid alive under a DIFFERENT start time is a reused pid, not a session.

        Without the start-time half of the key, a dead session's pin would adopt
        whatever process later took its number, and its directory would be kept
        for as long as that process ran.
        """
        sid = str(uuid.uuid4())
        d = self.session(sid)
        self.transcript(sid, LONG_AGO)
        pin = self.sessions / f".sid-by-pid-{self.cli.pid}-1"
        pin.write_text(sid + "\n", encoding="utf-8")

        self.run_reap()

        self.assertFalse(d.exists())
        self.assertFalse(pin.exists())

    def test_own_session_is_kept(self):
        """Source 1: the session running the command, even with no other signal."""
        d = self.session(self.own_sid)
        self.transcript(self.own_sid, LONG_AGO)
        self.run_reap()
        self.assertTrue(d.is_dir())

    def test_flat_markers_follow_their_session(self):
        """A dead session's markers go; a spec-keyed marker is not sid-keyed."""
        dead, live = str(uuid.uuid4()), str(uuid.uuid4())
        self.session(dead)
        self.session(live)
        self.transcript(dead, LONG_AGO)
        self.transcript(live, time.time())

        dead_marker = self.sessions / f".lsp-loaded-{dead}"
        live_marker = self.sessions / f".source-read-go-{live}"
        spec_marker = self.sessions / ".closure-ack-streaming-answer-protocol"
        for m in (dead_marker, live_marker, spec_marker):
            m.write_text("x", encoding="utf-8")

        self.run_reap()

        self.assertFalse(dead_marker.exists())
        self.assertTrue(live_marker.exists())
        self.assertTrue(spec_marker.exists())

    def test_dry_run_removes_nothing_and_names_everything(self):
        sid = str(uuid.uuid4())
        d = self.session(sid)
        self.transcript(sid, LONG_AGO)

        out = self.run_reap("--dry-run")

        self.assertTrue(d.is_dir())
        self.assertIn(str(d), out)
        self.assertIn("Would remove", out)

    def test_missing_transcript_root_removes_nothing(self):
        """Fail closed: with a CLI running and no transcripts, nothing is judged."""
        sid = str(uuid.uuid4())
        d = self.session(sid)
        blind = self.tmp / "no-claude-config"
        blind.mkdir()

        out = self.run_reap(config=blind)

        self.assertTrue(d.is_dir())
        self.assertIn("Removed nothing", out)

    def test_undated_entries_are_never_candidates(self):
        """The dated shape is the bound, as it is for ze-session-clean."""
        stray = self.sessions / "kernel"
        stray.mkdir()
        loose = self.sessions / "notes.txt"
        loose.write_text("x", encoding="utf-8")

        self.run_reap()

        self.assertTrue(stray.is_dir())
        self.assertTrue(loose.exists())


if __name__ == "__main__":
    unittest.main()
