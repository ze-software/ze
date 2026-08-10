#!/usr/bin/env python3
"""The QEMU DUT binaries must be passed to their driver, never re-derived.

mk/test-integration.mk cross-compiles the in-VM binaries as $(ZE_QEMU_BIN) /
$(ZE_QEMU_STRIPPED_BIN) / $(ZE_QEMU_TEST_BIN). Those paths sit in this session's
own directory under an AI session ($(ZE_BIN_DIR), mk/session.mk), so the literal
`bin/ze-test-linux-<arch>` is NOT the built path in general.

scripts/evidence/netns_qemu.py used to rebuild that literal itself and exec it.
Off-session the two spellings coincide and nothing looked wrong; under a session
the make target wrote into tmp/session/<date>-<id>/bin/ and the driver exec'd a
file that did not exist. Nothing caught it: the ze-qemu-* targets run in no automated CI
(ai/rules/platform-linux.md), so the mismatch only surfaces when a human runs it.

These tests pin both halves of the contract: the make target hands the paths
over, and the driver takes them.

Run: python3 scripts/dev/qemu_binary_paths_test.py
(also picked up automatically by TestPythonUnitTests, scripts/dev/python_tests_test.go)
"""

import os
import pathlib
import re
import subprocess
import sys
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]
MK = ROOT / "mk" / "test-integration.mk"
DRIVER = ROOT / "scripts" / "evidence" / "netns_qemu.py"

QEMU_BIN_VARS = ("ZE_QEMU_BIN", "ZE_QEMU_STRIPPED_BIN", "ZE_QEMU_TEST_BIN")


class TestQemuBinariesAreSessionScoped(unittest.TestCase):
    """The built paths must be session-scoped, or two sessions clobber them."""

    def test_each_qemu_bin_var_is_under_the_session_bin_dir(self):
        text = MK.read_text(encoding="utf-8")
        for var in QEMU_BIN_VARS:
            m = re.search(rf"^{var}\s*:?=\s*(.+)$", text, re.M)
            self.assertIsNotNone(m, f"{var} is not defined in {MK.name}")
            value = m.group(1).strip()
            self.assertTrue(
                value.startswith("$(ZE_BIN_DIR)/"),
                f"{var} = {value!r} is not under $(ZE_BIN_DIR): two concurrent "
                f"sessions cross-compiling for the VM would overwrite each "
                f"other's DUT binary mid-run",
            )


class TestNetnsDriverTakesThePathsItIsGiven(unittest.TestCase):
    """The driver must not re-derive names the make target already computed."""

    def test_make_target_passes_every_binary_path(self):
        text = MK.read_text(encoding="utf-8")
        run_lines = [ln for ln in text.splitlines() if "netns_qemu.py" in ln]
        self.assertTrue(run_lines, "no make target invokes netns_qemu.py")
        invocation = "\n".join(run_lines)
        for var in QEMU_BIN_VARS:
            self.assertIn(
                f'{var}="$({var})"',
                invocation,
                f"the netns target does not pass {var} to netns_qemu.py, so the "
                f"driver cannot know the real (session-scoped) path",
            )

    def test_driver_reads_each_path_from_the_environment(self):
        text = DRIVER.read_text(encoding="utf-8")
        for var in QEMU_BIN_VARS:
            self.assertIn(
                f'_qemu_bin("{var}"',
                text,
                f"{DRIVER.name} does not resolve {var} from the environment",
            )

    def test_driver_execs_no_hardcoded_cross_compiled_name(self):
        """A literal is allowed only as the standalone default inside _qemu_bin."""
        offenders = []
        for num, line in enumerate(DRIVER.read_text(encoding="utf-8").splitlines(), 1):
            if "-linux-{ARCH}" not in line:
                continue
            # The single sanctioned occurrence: the fallback return in _qemu_bin.
            if line.strip().startswith("return os.environ.get(env_key)"):
                continue
            offenders.append(f"{DRIVER.name}:{num}: {line.strip()}")
        self.assertEqual(
            [],
            offenders,
            "hardcoded cross-compiled binary name(s) outside the _qemu_bin "
            "fallback; these ignore the session directory and exec a path the "
            "make target never wrote:\n" + "\n".join(offenders),
        )

    def test_env_override_wins_over_the_default(self):
        """End-to-end: the resolver honours the variable the make target sets."""
        probe = (
            "import netns_qemu as m;"
            "print(m.ZE_QEMU_BIN, m.ZE_QEMU_STRIPPED_BIN, m.ZE_QEMU_TEST_BIN)"
        )
        env = {
            **os.environ,
            "QEMU_GOARCH": "arm64",
            "ZE_QEMU_BIN": "tmp/session/2026-08-10-sid/bin/ze-linux-arm64",
            "ZE_QEMU_STRIPPED_BIN": "tmp/session/2026-08-10-sid/bin/ze-stripped-linux-arm64",
            "ZE_QEMU_TEST_BIN": "tmp/session/2026-08-10-sid/bin/ze-test-linux-arm64",
        }
        out = subprocess.run(
            [sys.executable, "-c", probe],
            cwd=str(DRIVER.parent),
            env=env,
            capture_output=True,
            text=True,
            check=True,
        ).stdout.split()
        self.assertEqual(
            [
                "tmp/session/2026-08-10-sid/bin/ze-linux-arm64",
                "tmp/session/2026-08-10-sid/bin/ze-stripped-linux-arm64",
                "tmp/session/2026-08-10-sid/bin/ze-test-linux-arm64",
            ],
            out,
        )

    def test_default_is_the_plain_name_when_unset(self):
        """Standalone runs (no make target) keep working."""
        probe = "import netns_qemu as m; print(m.ZE_QEMU_TEST_BIN)"
        env = {k: v for k, v in os.environ.items() if k not in QEMU_BIN_VARS}
        env["QEMU_GOARCH"] = "amd64"
        out = subprocess.run(
            [sys.executable, "-c", probe],
            cwd=str(DRIVER.parent),
            env=env,
            capture_output=True,
            text=True,
            check=True,
        ).stdout.strip()
        self.assertEqual("bin/ze-test-linux-amd64", out)


if __name__ == "__main__":
    unittest.main()
