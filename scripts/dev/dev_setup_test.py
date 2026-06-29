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
