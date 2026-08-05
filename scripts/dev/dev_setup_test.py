#!/usr/bin/env python3
"""Tests for dev-setup.py."""

from __future__ import annotations

import importlib
import subprocess
import sys
import unittest
from pathlib import Path
from unittest import mock

SCRIPT = str(Path(__file__).parent / "dev-setup.py")

sys.path.insert(0, str(Path(__file__).parent))
dev_setup = importlib.import_module("dev-setup")


class TestOSDetect(unittest.TestCase):
    @mock.patch.object(dev_setup.platform, "system", return_value="Darwin")
    @mock.patch.object(dev_setup.shutil, "which", return_value="/opt/homebrew/bin/brew")
    def test_macos_with_brew(self, _which, _sys):
        self.assertEqual(dev_setup.detect_os(), "brew")

    @mock.patch.object(dev_setup.platform, "system", return_value="Darwin")
    @mock.patch.object(dev_setup.shutil, "which", return_value=None)
    def test_macos_no_brew(self, _which, _sys):
        self.assertIsNone(dev_setup.detect_os())

    @mock.patch.object(dev_setup.platform, "system", return_value="Linux")
    @mock.patch.object(dev_setup.shutil, "which", return_value="/usr/bin/apt-get")
    def test_linux_with_apt(self, _which, _sys):
        self.assertEqual(dev_setup.detect_os(), "apt")

    @mock.patch.object(dev_setup.platform, "system", return_value="Linux")
    @mock.patch.object(dev_setup.shutil, "which", return_value=None)
    def test_linux_no_apt(self, _which, _sys):
        self.assertIsNone(dev_setup.detect_os())

    @mock.patch.object(dev_setup.platform, "system", return_value="Windows")
    def test_unsupported(self, _sys):
        self.assertIsNone(dev_setup.detect_os())


class TestCheckModeExit(unittest.TestCase):
    def test_check_mode_missing_required_exits_nonzero(self):
        r = subprocess.run(
            [sys.executable, SCRIPT, "--check"],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            env={"PATH": "/usr/bin:/bin", "HOME": str(Path.home())},
        )
        self.assertNotEqual(r.returncode, 0)

    def test_check_mode_does_not_install(self):
        r = subprocess.run(
            [sys.executable, SCRIPT, "--check"],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
        )
        output = r.stdout.decode()
        self.assertNotIn("[installed]", output)
        self.assertNotIn("brew install", output)


class TestIdempotent(unittest.TestCase):
    def test_all_present_probe(self):
        tool = dev_setup.Tool(name="python3", probe=["python3"])
        self.assertTrue(dev_setup.probe_tool(tool))


class TestUnsupportedOS(unittest.TestCase):
    def test_unsupported_prints_list_exits_nonzero(self):
        r = subprocess.run(
            [sys.executable, SCRIPT, "--check"],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            env={"PATH": "", "HOME": str(Path.home())},
        )
        self.assertNotEqual(r.returncode, 0)
        output = r.stdout.decode()
        self.assertIn("Unsupported platform", output)
        self.assertIn("Manual installation", output)


class TestNoAutoSudo(unittest.TestCase):
    def test_install_tool_apt_prints_command(self):
        tool = dev_setup.Tool(name="test-pkg", probe=["test-pkg"], apt="test-pkg")
        with mock.patch("builtins.print") as mock_print:
            result = dev_setup.install_tool(tool, "apt")
        self.assertFalse(result)
        printed = " ".join(str(c) for c in mock_print.call_args_list)
        self.assertIn("sudo apt-get install", printed)
        self.assertNotIn("subprocess", printed)


class TestHasPackage(unittest.TestCase):
    def test_brew_with_package(self):
        tool = dev_setup.Tool(name="t", probe=["t"], brew="t")
        self.assertTrue(dev_setup._has_package(tool, "brew"))

    def test_brew_without_package(self):
        tool = dev_setup.Tool(name="t", probe=["t"], brew=None)
        self.assertFalse(dev_setup._has_package(tool, "brew"))

    def test_go_install_always_available(self):
        tool = dev_setup.Tool(name="t", probe=["t"], go_install="example.com/t@latest")
        self.assertTrue(dev_setup._has_package(tool, "brew"))
        self.assertTrue(dev_setup._has_package(tool, "apt"))


class TestSmokeCheck(unittest.TestCase):
    def test_check_mode_runs_and_reports(self):
        r = subprocess.run(
            [sys.executable, SCRIPT, "--check"],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
        )
        output = r.stdout.decode()
        self.assertIn("Ze dev setup", output)
        self.assertIn("[present]", output)


class TestGoplsIsRequired(unittest.TestCase):
    """gopls backs the agent LSP tool, which .claude/rules/session-start.md
    makes a BLOCKING first action. The gate there lifts on the query text, so
    an absent server is invisible to it; this setup entry is what installs it.
    """

    def _gopls(self):
        for tool in dev_setup.REQUIRED_TOOLS:
            if tool.name == "gopls":
                return tool
        return None

    def test_gopls_is_a_required_tool(self):
        self.assertIsNotNone(self._gopls(), "gopls missing from REQUIRED_TOOLS")

    def test_gopls_installs_via_go_install(self):
        """The install actually taken is `go install`, on every package manager.

        Asserting `_has_package` here would prove nothing: it returns True for
        any tool carrying `go_install`, before it consults brew or apt, so the
        assertion is implied by the line above it and cannot fail on its own.
        What CAN fail is the command install_tool runs, so that is what is
        checked -- give gopls an `apt` package and drop `go_install`, and the
        run below becomes a printed sudo line and a False.
        """
        tool = self._gopls()
        self.assertEqual(tool.go_install, "golang.org/x/tools/gopls@latest")
        self.assertEqual(tool.probe, ["gopls"])
        for pkg_mgr in ("apt", "brew"):
            with (
                mock.patch.object(
                    dev_setup.shutil, "which", return_value="/usr/bin/go"
                ),
                mock.patch.object(dev_setup.subprocess, "run") as run,
            ):
                run.return_value = subprocess.CompletedProcess([], 0, b"", b"")
                self.assertTrue(dev_setup.install_tool(tool, pkg_mgr))
            self.assertEqual(
                run.call_args[0][0],
                ["go", "install", "golang.org/x/tools/gopls@latest"],
            )


def _completed(returncode: int, stdout: bytes = b"", stderr: bytes = b""):
    return subprocess.CompletedProcess([], returncode, stdout, stderr)


class TestGoplsAnswers(unittest.TestCase):
    """gopls on PATH is not a working language server.

    It was absent from this machine for weeks while the BLOCKING "load LSP
    first" rule was satisfied every session: that gate lifts on the query text,
    and no call was ever made, so every one would have returned ENOENT unseen.
    A presence check would not have caught it, which is why this check RUNS the
    server. The probe is faked here -- a unit test cannot own a real one.
    """

    ANSWER = b"Clock Interface 18:6-18:11\n\tNow Method 20:2-20:5\n"

    def _status(self, probe, present: bool = True):
        which = "/home/x/go/bin/gopls" if present else None
        with mock.patch.object(dev_setup.shutil, "which", return_value=which):
            return dev_setup.gopls_status(probe=probe)

    def test_probe_file_is_in_the_checkout(self):
        """A probe file that moves turns the whole check into a silent 'n/a'."""
        self.assertTrue((dev_setup.REPO_ROOT / dev_setup.GOPLS_PROBE_FILE).is_file())

    def test_answering_server_is_ok(self):
        state, detail = self._status(lambda: _completed(0, self.ANSWER))
        self.assertEqual(state, "ok")
        self.assertIn("2 symbols", detail)
        self.assertIn(dev_setup.GOPLS_PROBE_FILE, detail)

    def test_absent_binary_is_not_the_same_finding_as_a_mute_server(self):
        """The two need different fixes, so they must not share a message."""

        def unreachable():
            raise AssertionError("probe must not run when gopls is absent")

        absent_state, absent_detail = self._status(unreachable, present=False)
        mute_state, mute_detail = self._status(lambda: _completed(0, b"\n"))
        self.assertEqual(absent_state, "absent")
        self.assertEqual(mute_state, "broken")
        self.assertEqual(absent_detail, dev_setup.GOPLS_NOT_INSTALLED)
        self.assertIn(dev_setup.GOPLS_NOT_ANSWERING, mute_detail)
        self.assertNotIn(dev_setup.GOPLS_NOT_ANSWERING, absent_detail)

    def test_output_without_a_symbol_is_not_an_answer(self):
        """Exit 0 and some text is not proof: the reply must carry a symbol."""
        state, detail = self._status(lambda: _completed(0, b"loading packages...\n"))
        self.assertEqual(state, "broken")
        self.assertIn("no symbol in its reply", detail)

    def test_failing_server_reports_its_own_message(self):
        state, detail = self._status(
            lambda: _completed(1, b"", b"err: no module cache\nmore\n")
        )
        self.assertEqual(state, "broken")
        self.assertIn("no module cache", detail)
        # Only the first line of the server's complaint, not its whole output.
        self.assertNotIn("more", detail)

    def test_timeout_is_a_mute_server_not_a_missing_one(self):
        def slow():
            raise subprocess.TimeoutExpired(cmd="gopls", timeout=1)

        state, detail = self._status(slow)
        self.assertEqual(state, "broken")
        self.assertIn(str(dev_setup.GOPLS_PROBE_TIMEOUT), detail)

    def test_check_mode_reports_the_probe(self):
        r = subprocess.run(
            [sys.executable, SCRIPT, "--check"],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
        )
        self.assertIn("gopls-answers", r.stdout.decode())


class TestApplianceChecksMarker(unittest.TestCase):
    def test_appliance_checks_has_required_keys(self):
        self.assertIn("appliance-grub", dev_setup.APPLIANCE_CHECKS)
        self.assertIn("appliance-xorriso", dev_setup.APPLIANCE_CHECKS)
        self.assertIn("appliance-e2fsprogs", dev_setup.APPLIANCE_CHECKS)

    def test_appliance_checks_have_probe(self):
        for name, info in dev_setup.APPLIANCE_CHECKS.items():
            self.assertIn("probe", info, f"{name} missing probe key")
            self.assertIsInstance(info["probe"], list, f"{name} probe not a list")
            self.assertTrue(len(info["probe"]) > 0, f"{name} probe is empty")


if __name__ == "__main__":
    unittest.main()
