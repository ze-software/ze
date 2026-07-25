#!/usr/bin/env python3
"""make and Go must agree on which session ids are usable.

mk/session.mk decides the binary suffix (bin/ze-<id>); internal/test/sessionpath
decides where the test runner looks for binaries and scratch. If the two
validators disagree, make builds bin/ze-a+b while Go's ID() rejects the same id
and looks in the shared bin/ -- the build and the runner then disagree about
which artifacts belong to the session. That is the exact drift the
single-resolver design (.claude/hooks/lib/session_id.py) exists to prevent, so
it is pinned here rather than left to review.

Also pinned: an id may not reproduce another binary's name. ZE_SESSION_ID=test
turns bin/ze-<id> into bin/ze-test, the real test-runner binary, and `make ze`
would silently write a `ze` build over it.

Run: python3 scripts/dev/session_bin_suffix_test.py
(also picked up automatically by TestPythonUnitTests, scripts/dev/python_tests_test.go)
"""

import os
import pathlib
import re
import subprocess
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]

# Mirrors sidSafe in internal/test/sessionpath/sessionpath.go, itself a mirror of
# _SID_SAFE_RE in .claude/hooks/lib/session_id.py.
GO_SID_SAFE = re.compile(r"\A[A-Za-z0-9._-]+\Z")

# Mirrors ZE_BIN_NAMES in mk/session.mk.
BINARY_NAMES = {
    "ze",
    "ze-appliance",
    "ze-setup",
    "ze-stripped",
    "ze-test",
    "ze-chaos",
    "ze-analyze",
    "ze-perf",
}


def ze_path(session_id=None, drop_session=False):
    """`make ze-path` with ZE_SESSION_ID set (or the session dropped entirely)."""
    env = dict(os.environ)
    if drop_session:
        env.pop("CLAUDE_CODE_SESSION_ID", None)
        env.pop("ZE_SESSION_ID", None)
    cmd = ["make", "ze-path"]
    if session_id is not None:
        cmd.append(f"ZE_SESSION_ID={session_id}")
    return subprocess.run(
        cmd, cwd=str(ROOT), env=env, capture_output=True, text=True, check=True
    ).stdout.strip()


def go_accepts(sid):
    """What internal/test/sessionpath.ID() would do with sid."""
    return bool(GO_SID_SAFE.match(sid)) and sid not in (".", "..")


class TestSuffixMatchesGoValidation(unittest.TestCase):
    """make accepts an id only when Go would, and vice versa."""

    # Ids Go accepts and that collide with nothing.
    ACCEPTED = [
        "af90b60f-f7ca-4a73-bbe2-925b93376a72",  # the real shape: a UUID
        "ok-id",
        "ok.id_1",
        "AF90-b60f.1",
        "a",
    ]
    # Ids Go rejects; make must reject them too.
    REJECTED = ["a+b", "a@b", "a!b", "a:b", "a b", "..", ".", "../../etc", "a/b", ""]

    def test_accepted_ids_get_the_suffix(self):
        for sid in self.ACCEPTED:
            with self.subTest(sid=sid):
                self.assertTrue(go_accepts(sid), "fixture must be Go-acceptable")
                self.assertEqual(f"bin/ze-{sid}", ze_path(sid))

    def test_rejected_ids_fall_back_to_the_shared_path(self):
        for sid in self.REJECTED:
            with self.subTest(sid=sid):
                self.assertFalse(go_accepts(sid), "fixture must be Go-rejectable")
                self.assertEqual(
                    "bin/ze",
                    ze_path(sid),
                    f"make accepted {sid!r} that Go rejects: the build and the "
                    f"test runner would disagree about this session's artifacts",
                )

    def test_off_session_is_the_plain_name(self):
        self.assertEqual("bin/ze", ze_path(drop_session=True))


class TestSuffixCannotCollideWithARealBinary(unittest.TestCase):
    def test_ids_reproducing_a_binary_name_are_refused(self):
        for name in sorted(BINARY_NAMES):
            if not name.startswith("ze-"):
                continue
            sid = name[len("ze-") :]  # noqa: E203 - "test" -> bin/ze-test
            with self.subTest(sid=sid):
                self.assertTrue(go_accepts(sid), "the id itself is well-formed")
                self.assertEqual(
                    "bin/ze",
                    ze_path(sid),
                    f"ZE_SESSION_ID={sid} yields bin/{name}, overwriting a real binary",
                )

    def test_binary_name_list_matches_the_makefile(self):
        """The guard is only as complete as the list it is derived from."""
        text = (ROOT / "mk" / "session.mk").read_text(encoding="utf-8")
        m = re.search(r"^ZE_BIN_NAMES\s*:=\s*(.+)$", text, re.M)
        self.assertIsNotNone(m, "ZE_BIN_NAMES not found in mk/session.mk")
        self.assertEqual(BINARY_NAMES, set(m.group(1).split()))


class TestValidationDoesNotRunShell(unittest.TestCase):
    """The charset check interpolates the id into a shell command.

    A single quote is the only character that can terminate that literal, so it
    is refused in pure make before the shell sees it. If that guard regresses,
    `make` executes attacker-chosen shell at parse time -- on every invocation.
    """

    def test_quote_bearing_id_is_refused_and_not_executed(self):
        marker = "ZE-INJECTION-MARKER"
        hostile = f"a'; echo {marker}; '"
        env = dict(os.environ)
        proc = subprocess.run(
            ["make", "ze-path", f"ZE_SESSION_ID={hostile}"],
            cwd=str(ROOT),
            env=env,
            capture_output=True,
            text=True,
            check=True,
        )
        self.assertEqual("bin/ze", proc.stdout.strip())
        self.assertNotIn(marker, proc.stdout + proc.stderr, "shell injection executed")


if __name__ == "__main__":
    unittest.main()
