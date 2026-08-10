#!/usr/bin/env python3
"""Unit tests for per-session tmp/ scratch isolation, and for the ONE rule that
governs its lifetime: nothing under tmp/session/ is ever deleted automatically.

Covers scripts/dev/session-scratch.sh (prints the scratch/ subdirectory of
tmp/session/<YYYY-MM-DD>-<session-id>/, and --clean removes THIS session's own
directory when a person or `make clean` asks). Everything runs inside a
throwaway git repo so the tests never touch the real tmp/. The scripts source
.claude/hooks/lib/session-id.sh by a path relative to the script (the REAL
repo), while resolving the working root from $CLAUDE_PROJECT_DIR or the caller's
cwd (the throwaway repo) -- which is exactly why this isolation works.

The dated directory is LOOKED UP, never recomputed: an existing
tmp/session/????-??-??-<sid> wins, and today's date names a new one only on a
miss. TestSessionDirIsLookedUp is what pins that here; make and Go implement the
same rule, and scripts/dev/session_bin_dir_test.py pins their agreement.

Owner decision, 2026-08-03 (plan/spec-session-bin-directory.md, AC-7 and AC-16):
no session end, no age timer and no hook may remove anything under tmp/session/.
Growth is the accepted price of never deleting the operator's data unasked, and
cleanup is made easy instead -- the YYYY-MM-DD- prefix, and
`make ze-clean-sessions BEFORE=<date>` (driven in session_bin_dir_test.py).
TestSessionEndDeletesNothing and TestNoAutomaticDeletionRemains are what stop
the sweeps coming back under a new name.
"""

from __future__ import annotations

import datetime
import json
import os
import re
import subprocess
import tempfile
import unittest
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
HELPER = REPO / "scripts" / "dev" / "session-scratch.sh"
SETTINGS = REPO / ".claude" / "settings.json"

TODAY = datetime.date.today().isoformat()


def _session_dir(root: Path, sid: str, date: str = TODAY) -> Path:
    """The directory a session owns: <root>/tmp/session/<YYYY-MM-DD>-<sid>/."""
    return root / "tmp" / "session" / f"{date}-{sid}"


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


def _registered_hooks(event: str) -> list[str]:
    """The hook COMMANDS the harness runs for one event, from settings.json.

    Read the registry rather than a file listing: a hook script nobody
    registers never runs, and a registered one runs on every such event in this
    checkout. The registry is the entry point.
    """
    settings = json.loads(SETTINGS.read_text())
    return [
        hook["command"]
        for group in settings.get("hooks", {}).get(event, [])
        for hook in group.get("hooks", [])
    ]


def _tree(root: Path) -> set[str]:
    """Every path under root, relative and sorted, for a before/after compare."""
    return {str(p.relative_to(root)) for p in root.rglob("*")}


class TestSessionScratchHelper(unittest.TestCase):
    def test_creates_per_session_dir(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            out = _run_helper(root, "sid-AAA").stdout.strip()
            self.assertEqual(out, f"tmp/session/{TODAY}-sid-AAA/scratch")
            self.assertTrue((_session_dir(root, "sid-AAA") / "scratch").is_dir())

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
            self.assertEqual(out, f"tmp/session/{TODAY}-sid-CCC/scratch")
            self.assertFalse(_session_dir(root, "sid-CCC").exists())

    def test_project_dir_takes_precedence_over_cwd(self):
        # The helper must resolve the same root the hooks use
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
            self.assertEqual(out, f"tmp/session/{TODAY}-sid-DDD/scratch")
            self.assertTrue((_session_dir(proot, "sid-DDD") / "scratch").is_dir())
            self.assertFalse(_session_dir(Path(elsewhere), "sid-DDD").exists())

    def test_dot_ids_are_refused(self):
        # A hostile CLAUDE_CODE_SESSION_ID must not let the helper escape
        # tmp/session/.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            for bad in ("..", "."):
                proc = _run_helper(root, bad, check=False)
                self.assertNotEqual(proc.returncode, 0, f"id {bad!r} should be refused")
            # Nothing created above the session root (e.g. no tmp/ via '..').
            self.assertFalse((root / "tmp" / "session").exists())

    def test_clean_removes_only_own_dir(self):
        # `make clean` runs --clean: it removes THIS session's whole directory,
        # binaries included, and nothing else (no shared caches, no sibling
        # sessions), and prints nothing.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            mine = _session_dir(root, "sid-CLEAN")
            other = _session_dir(root, "other")
            shared_cache = root / "tmp" / "go-cache"  # outside the root -> survives
            (mine / "scratch").mkdir(parents=True)
            (mine / "bin").mkdir()
            other.mkdir(parents=True)
            shared_cache.mkdir(parents=True)
            (mine / "scratch" / "x.log").write_text("scratch")
            (mine / "bin" / "ze").write_text("binary")
            (shared_cache / "c").write_text("cached")
            proc = _run_helper(root, "sid-CLEAN", "--clean")
            self.assertEqual(proc.stdout.strip(), "")
            self.assertFalse(mine.exists())
            self.assertTrue(other.exists())
            self.assertTrue(shared_cache.exists())

    def test_flat_marker_files_are_never_touched(self):
        # .sid-by-pid-<clipid> mints the id the directory is named for, and
        # .closure-ack-<stem> is keyed by spec stem, so both stay flat under
        # tmp/session/. --clean is the only removal this helper performs, and it
        # must match neither.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            flat = root / "tmp" / "session"
            flat.mkdir(parents=True)
            markers = [flat / ".sid-by-pid-42-987", flat / ".closure-ack-some-spec"]
            for m in markers:
                m.write_text("x")
            _session_dir(root, "sid-FLAT").mkdir()
            _run_helper(root, "sid-FLAT", "--clean")
            for m in markers:
                self.assertTrue(m.exists(), f"{m.name} was swept")

    def test_reap_is_gone(self):
        # --reap removed the dated directory of any session idle for 24h, which
        # is exactly the automatic deletion AC-7 bans. The flag is gone, not
        # neutered: an unknown flag is a usage error, so a caller that still
        # passes it fails loudly instead of silently doing nothing.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            dead = _session_dir(root, "dead", "2026-01-02")
            dead.mkdir(parents=True)
            proc = _run_helper(root, "ignored", "--reap", project_dir=root, check=False)
            self.assertEqual(proc.returncode, 2, proc.stderr)
            self.assertIn("usage:", proc.stderr)
            self.assertNotIn("--reap", proc.stderr)
            self.assertTrue(dead.exists(), "a dead session's directory was removed")


class TestSessionDirIsLookedUp(unittest.TestCase):
    """The dated name is resolved by glob, never rebuilt from today's clock.

    A helper that recomputed <today>-<sid> would hand a session a different
    directory at 00:01 than it used at 23:59, orphaning the binaries that
    session is running out of the old one.
    """

    def test_an_existing_dated_directory_wins_over_today(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            yesterday = (datetime.date.today() - datetime.timedelta(days=1)).isoformat()
            planted = _session_dir(root, "sid-OLD", yesterday)
            planted.mkdir(parents=True)

            out = _run_helper(root, "sid-OLD").stdout.strip()

            self.assertEqual(out, f"tmp/session/{yesterday}-sid-OLD/scratch")
            self.assertEqual(
                [planted.name],
                sorted(p.name for p in (root / "tmp" / "session").iterdir()),
                "resolving the directory minted a second one",
            )


class TestSessionEndDeletesNothing(unittest.TestCase):
    """AC-7: a session ending removes NOTHING under tmp/session/.

    A SessionEnd hook once removed the whole dated directory, its bin/, and the
    session's spec claim. It is deleted, and the event is unregistered. Reading
    the REGISTRY rather than a directory listing is what makes this a guard: a
    hook file nobody registers never runs, and any command registered on that
    event runs on every session end in this checkout. The loop below drives
    whatever is registered, so re-adding a deleting hook goes red here.
    """

    def test_no_session_end_hook_is_registered(self):
        self.assertEqual(
            _registered_hooks("SessionEnd"),
            [],
            "a SessionEnd hook is registered; the only work that event ever did "
            "in this repo was deleting tmp/session/",
        )

    def test_a_session_end_payload_leaves_the_session_intact(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _init_repo(root)
            mine = _session_dir(root, "mine")
            (mine / "bin").mkdir(parents=True)
            (mine / "bin" / "ze").write_text("binary")
            (mine / "scratch").mkdir()
            (mine / "scratch" / "run.log").write_text("output")
            flat = root / "tmp" / "session"
            (flat / ".session-mine").write_text("spec-fixture.md\n")
            (flat / ".sid-by-pid-42-987").write_text("mine\n")
            (flat / "session-state-fixture-mine.md").write_text("# state\n")
            before = _tree(root)

            env = dict(os.environ)
            env["CLAUDE_PROJECT_DIR"] = str(root)
            payload = json.dumps({"session_id": "mine", "reason": "clear"})
            for command in _registered_hooks("SessionEnd"):
                script = command.replace("$CLAUDE_PROJECT_DIR", str(REPO))
                subprocess.run(
                    ["bash", script],
                    cwd=str(root),
                    env=env,
                    input=payload,
                    capture_output=True,
                    text=True,
                )

            self.assertEqual(before, _tree(root), "SessionEnd removed or added a path")


class TestNoAutomaticDeletionRemains(unittest.TestCase):
    """AC-16: no time-based or event-based deletion under tmp/session/ survives.

    The sweeps were spread over five files and three idioms, so the guard is on
    the IDIOMS rather than on the file that held them: `find -mmin +N`, the two
    reaper functions, and an `rm -rf` naming the session root. A sweep written
    back under a new name still spells one of the three.

    `*_test.py` is skipped, and so is a `#` comment line. Both NAME the idiom
    rather than run it, and this file names all four in BANNED below. Anything
    a shell or a Python interpreter would execute is still scanned, a heredoc
    body and a string literal included.
    """

    TREES = (".claude/hooks", "scripts/dev")
    BANNED = (
        (re.compile(r"-mmin\s+[+-]\d"), "an age-based find sweep"),
        (re.compile(r"\breap_dead\b"), "the dead-session directory reaper"),
        (re.compile(r"\breap_binaries\b"), "the session binary reaper"),
        (re.compile(r"rm\s+-rf?\b[^\n]*tmp/session"), "an rm -rf of the session root"),
    )

    def test_no_producer_sweeps_the_session_root(self):
        offenders = []
        for tree in self.TREES:
            for path in sorted((REPO / tree).rglob("*")):
                if not path.is_file() or path.name.endswith("_test.py"):
                    continue
                try:
                    text = path.read_text()
                except (UnicodeDecodeError, OSError):
                    continue
                for line_no, line in enumerate(text.splitlines(), 1):
                    if line.lstrip().startswith("#"):
                        continue
                    for pattern, what in self.BANNED:
                        if pattern.search(line):
                            rel = path.relative_to(REPO)
                            offenders.append(f"{rel}:{line_no}: {what}")
        self.assertEqual(
            offenders,
            [],
            "automatic deletion is back: " + "; ".join(offenders),
        )


class TestCleanTmpPreservesSessionRoot(unittest.TestCase):
    """AC-10: `make ze-clean-tmp` never reaches a live session's directory.

    The target sweeps the tmp/ ROOT: files older than 24h, and directories older
    than 24h other than session/ and kernel/. It is operator-invoked, so it is
    allowed to delete; it must simply never descend into tmp/session/.

    The recipe is taken from make itself (`make -n`) and run against a fixture
    tree, so this drives the target's own commands rather than a copy of them,
    and no test ever sweeps the real tmp/.
    """

    def _recipe(self) -> str:
        proc = subprocess.run(
            ["make", "-n", "ze-clean-tmp"],
            cwd=str(REPO),
            capture_output=True,
            text=True,
            check=True,
        )
        return proc.stdout

    def test_a_live_session_survives_the_sweep(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            session = _session_dir(root, "sid-LIVE")
            (session / "bin").mkdir(parents=True)
            (session / "bin" / "ze").write_text("binary")
            flat = root / "tmp" / "session"
            (flat / ".session-sid-LIVE").write_text("spec-fixture.md\n")
            kernel = root / "tmp" / "kernel"
            kernel.mkdir()
            (kernel / "vmlinuz").write_text("kernel")
            stale_dir = root / "tmp" / "verify"
            stale_dir.mkdir()
            (stale_dir / "old.log").write_text("old")
            stale_file = root / "tmp" / "ze-verify.log"
            stale_file.write_text("old")
            old = (datetime.datetime.now() - datetime.timedelta(hours=26)).timestamp()
            for path in (
                session,
                session / "bin",
                session / "bin" / "ze",
                flat,
                flat / ".session-sid-LIVE",
                kernel,
                kernel / "vmlinuz",
                stale_dir,
                stale_dir / "old.log",
                stale_file,
            ):
                os.utime(path, (old, old))

            subprocess.run(
                ["sh", "-c", self._recipe()],
                cwd=str(root),
                capture_output=True,
                text=True,
                check=True,
            )

            self.assertTrue(
                (session / "bin" / "ze").is_file(),
                "ze-clean-tmp reached a session's binaries",
            )
            self.assertTrue(
                (flat / ".session-sid-LIVE").is_file(),
                "ze-clean-tmp reached a session's claim marker",
            )
            self.assertTrue((kernel / "vmlinuz").is_file(), "the kernel cache went")
            # The control: the sweep really ran, so the two survivals above are
            # exclusions rather than a target that did nothing.
            self.assertFalse(stale_dir.exists(), "the sweep did not run")
            self.assertFalse(stale_file.exists(), "the sweep did not run")

    def test_a_stale_tmp_root_is_not_swept_as_its_own_child(self):
        """`find tmp/` yields tmp/ ITSELF at depth 0, and `tmp` is not `session`.

        Without -mindepth 1 the directory sweep matches the root, and `rm -rf`
        takes the whole scratch tree -- every session's binaries, seeded store
        and state/ digest -- while both -not -name exclusions read as if they
        prevented exactly that. The case is invisible unless tmp/'s OWN mtime is
        stale, which is why the sibling test above never saw it: creating a
        fixture child touches the parent, so the root always looked fresh.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            session = _session_dir(root, "sid-LIVE")
            (session / "bin").mkdir(parents=True)
            (session / "bin" / "ze").write_text("binary")
            (session / "state").mkdir()
            (session / "state" / "session-state-spec-x-sid-LIVE.md").write_text("d\n")
            tmp_root = root / "tmp"

            # The root LAST, so no later mkdir refreshes it.
            old = (datetime.datetime.now() - datetime.timedelta(hours=26)).timestamp()
            os.utime(tmp_root, (old, old))

            subprocess.run(
                ["sh", "-c", self._recipe()],
                cwd=str(root),
                capture_output=True,
                text=True,
                check=True,
            )

            self.assertTrue(tmp_root.is_dir(), "ze-clean-tmp removed the tmp/ root")
            self.assertTrue(
                (session / "bin" / "ze").is_file(),
                "ze-clean-tmp took a session's binaries with the tmp/ root",
            )
            self.assertTrue(
                (session / "state" / "session-state-spec-x-sid-LIVE.md").is_file(),
                "ze-clean-tmp took a session's spec digest with the tmp/ root",
            )


if __name__ == "__main__":
    unittest.main()
