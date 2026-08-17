#!/usr/bin/env python3
"""Tests for dev-setup.py."""

from __future__ import annotations

import importlib
import json
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
    # A prefix-free path on purpose: detect_os only asks whether `brew` was
    # found, never where, and naming a prefix here would be a second place that
    # believes Homebrew lives at one (scripts/dev/homebrew_prefix_test.py).
    @mock.patch.object(dev_setup.shutil, "which", return_value="/somewhere/bin/brew")
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


class TestPrivilegeMode(unittest.TestCase):
    """No route to root may block on a password prompt.

    `make ze-dev-setup` runs from a Makefile, from a container build, and from an
    agent session with no terminal at all. sudo reads its prompt from the stdin
    it inherits, so "can sudo act without a password" has to be answered BEFORE
    a command runs. Answered after, the answer arrives on a run that is already
    hung with no output.
    """

    def _mode(
        self,
        euid: int = 1000,
        sudo: str | None = "/usr/bin/sudo",
        rc: int = 0,
        tty: bool = True,
        boom: Exception | None = None,
    ) -> str:
        stdin = mock.Mock()
        stdin.isatty.return_value = tty
        with (
            mock.patch.object(dev_setup.os, "geteuid", return_value=euid),
            mock.patch.object(dev_setup.shutil, "which", return_value=sudo),
            mock.patch.object(dev_setup.sys, "stdin", stdin),
            mock.patch.object(dev_setup.subprocess, "run") as run,
        ):
            run.side_effect = boom
            if boom is None:
                run.return_value = _completed(rc)
            return dev_setup.privilege_mode()

    def test_root_needs_no_sudo(self):
        """A container build runs as root, where sudo is usually not installed."""
        self.assertEqual(self._mode(euid=0, sudo=None), "root")

    def test_no_sudo_binary_is_no_route(self):
        self.assertEqual(self._mode(sudo=None), "none")

    def test_passwordless_sudo_is_a_route(self):
        self.assertEqual(self._mode(rc=0), "sudo")

    def test_a_password_with_a_terminal_to_type_it_on_is_a_route(self):
        self.assertEqual(self._mode(rc=1, tty=True), "sudo-prompt")

    def test_a_password_with_no_terminal_is_no_route(self):
        """The discriminating case, and the whole reason this function exists.

        sudo would print its prompt and wait forever. Drop the isatty test and
        only this fails: every other case here is already decided by sudo -n.
        """
        self.assertEqual(self._mode(rc=1, tty=False), "none")

    def test_a_wedged_sudo_is_no_route(self):
        """An unreachable sudoers source (LDAP, typically) hangs `sudo -n`."""
        timeout = subprocess.TimeoutExpired(cmd="sudo", timeout=1)
        self.assertEqual(self._mode(boom=timeout), "none")


class TestRunPrivileged(unittest.TestCase):
    def _run(self, argv, mode="sudo", rc=0, **kw):
        with (
            mock.patch.object(dev_setup, "privilege_mode", return_value=mode),
            mock.patch.object(dev_setup.subprocess, "run") as run,
            mock.patch("builtins.print"),
        ):
            run.return_value = _completed(rc, stderr=b"E: nope\nmore\n")
            ok, detail = dev_setup.run_privileged(argv, **kw)
        return ok, detail, run

    def test_sudo_is_always_given_n(self):
        """The regression that motivated one helper for all three callers.

        `sudo tee` used to be handed the file's content on stdin while sudo was
        free to prompt on that same stdin: the prompt eats the content and the
        drop-in gets written with a password in it, or nothing. `-n` means no
        code path can reach a prompt, so a piped stdin only ever reaches the
        command.
        """
        _, _, run = self._run(["tee", "/etc/x.conf"], stdin=b"a = 0\n")
        self.assertEqual(run.call_args[0][0], ["sudo", "-n", "tee", "/etc/x.conf"])
        self.assertEqual(run.call_args[1]["input"], b"a = 0\n")

    def test_root_runs_the_command_bare(self):
        _, _, run = self._run(["apt-get", "update"], mode="root")
        self.assertEqual(run.call_args[0][0], ["apt-get", "update"])

    def test_the_echoed_line_says_what_actually_ran(self):
        """A root run must not print a `sudo` it did not use.

        The echoed line is what a reader copies when the step fails, and on a
        container build there is no sudo on the box to copy it to.
        """
        with (
            mock.patch.object(dev_setup, "privilege_mode", return_value="root"),
            mock.patch.object(dev_setup.subprocess, "run") as run,
            mock.patch("builtins.print") as printed,
        ):
            run.return_value = _completed(0)
            dev_setup.run_privileged(
                ["tee", "/etc/x.conf"],
                stdin=b"a = 0\n",
                shown='echo "a = 0" | {sudo}tee /etc/x.conf',
            )
        self.assertIn('echo "a = 0" | tee /etc/x.conf', _said(printed))
        self.assertNotIn("sudo", _said(printed))

    def test_a_password_is_asked_for_once_then_never_prompted_again(self):
        """Row 3 of the documented table: sudo wants a password and a terminal
        is attached to type it on.

        `sudo -v` is the only interactive call, and every command after it is
        still `-n`, so a piped stdin cannot reach a prompt even here.
        """
        with (
            mock.patch.object(dev_setup, "privilege_mode", return_value="sudo-prompt"),
            mock.patch.object(dev_setup.subprocess, "run") as run,
            mock.patch("builtins.print"),
        ):
            run.return_value = _completed(0)
            ok, _ = dev_setup.run_privileged(["apt-get", "update"])

        self.assertTrue(ok)
        calls = [c[0][0] for c in run.call_args_list]
        self.assertEqual(calls[0], ["sudo", "-v"])
        self.assertEqual(calls[1], ["sudo", "-n", "apt-get", "update"])

    def test_a_refused_password_runs_nothing(self):
        with (
            mock.patch.object(dev_setup, "privilege_mode", return_value="sudo-prompt"),
            mock.patch.object(dev_setup.subprocess, "run") as run,
            mock.patch("builtins.print"),
        ):
            run.return_value = _completed(1)
            ok, detail = dev_setup.run_privileged(["apt-get", "update"])

        self.assertFalse(ok)
        self.assertIn("could not authenticate", detail)
        self.assertEqual([c[0][0] for c in run.call_args_list], [["sudo", "-v"]])

    def test_no_route_runs_nothing(self):
        ok, detail, run = self._run(["apt-get", "update"], mode="none")
        self.assertFalse(ok)
        run.assert_not_called()
        self.assertIn("no password-free route to root", detail)

    def test_failure_carries_the_command_s_own_first_line(self):
        ok, detail, _ = self._run(["apt-get", "install", "-y", "x"], rc=100)
        self.assertFalse(ok)
        self.assertIn("E: nope", detail)
        self.assertNotIn("more", detail)


class TestAptInstalls(unittest.TestCase):
    """Linux installs, rather than printing a list for a human to retype.

    macOS ran `brew install` and Linux printed `sudo apt-get install ...` and
    returned False, so one `make ze-dev-setup` set the machine up and the other
    produced homework. Every tool row already carries its apt package.
    """

    def setUp(self):
        # The once-per-run guard is module state, so each case starts from the
        # state a fresh `make ze-dev-setup` starts from.
        setattr(dev_setup, "_apt_updated", False)

    def _install(self, pkg="xorriso", mode="sudo", rc=0):
        with (
            mock.patch.object(dev_setup, "privilege_mode", return_value=mode),
            mock.patch.object(dev_setup.subprocess, "run") as run,
            mock.patch("builtins.print") as printed,
        ):
            run.return_value = _completed(rc)
            ok = dev_setup.apt_install(pkg, pkg)
        return ok, [c[0][0] for c in run.call_args_list], _said(printed)

    def test_install_tool_reaches_apt_get(self):
        tool = dev_setup.Tool(name="xorriso", probe=["xorriso"], apt="xorriso")
        with (
            mock.patch.object(dev_setup, "privilege_mode", return_value="root"),
            mock.patch.object(dev_setup.subprocess, "run") as run,
            mock.patch("builtins.print"),
        ):
            run.return_value = _completed(0)
            self.assertTrue(dev_setup.install_tool(tool, "apt"))
        self.assertIn("apt-get", run.call_args_list[-1][0][0])

    def test_the_install_carries_a_noninteractive_frontend(self):
        """sudo resets the environment, so an exported value never arrives.

        A package with a debconf prompt stops the run dead without this, and
        the tool rows already include ones that have had them (ppp, docker.io).
        """
        _, calls, _ = self._install()
        self.assertEqual(
            calls[-1],
            [
                "sudo",
                "-n",
                "env",
                "DEBIAN_FRONTEND=noninteractive",
                "apt-get",
                "install",
                "-y",
                "xorriso",
            ],
        )

    def test_update_runs_once_before_the_first_install(self):
        """A container image ships no package lists at all.

        Without the update, apt-get answers "Unable to locate package", which
        reads as a wrong package name rather than a missing index.
        """
        _, first, _ = self._install("xorriso")
        _, second, _ = self._install("e2fsprogs")
        self.assertEqual(first[0], ["sudo", "-n", "apt-get", "update"])
        self.assertNotIn(["sudo", "-n", "apt-get", "update"], second)

    def test_a_stale_index_still_installs(self):
        """`apt-get update` failing is a warning, not the end of the install.

        A machine behind a broken mirror still has the index it fetched last
        week, and the package is usually in it.
        """
        with (
            mock.patch.object(dev_setup, "privilege_mode", return_value="sudo"),
            mock.patch.object(dev_setup.subprocess, "run") as run,
            mock.patch("builtins.print") as printed,
        ):
            run.side_effect = [_completed(1, stderr=b"E: mirror\n"), _completed(0)]
            self.assertTrue(dev_setup.apt_install("xorriso", "xorriso"))
        self.assertIn("WARN apt-get update", _said(printed))

    def test_no_root_prints_the_command_and_runs_nothing(self):
        """The old behaviour, kept as the fallback rather than the rule."""
        ok, calls, printed = self._install(mode="none")
        self.assertFalse(ok)
        self.assertEqual(calls, [])
        self.assertIn("sudo apt-get install -y xorriso", printed)

    def test_a_failed_install_says_what_to_run(self):
        ok, _, printed = self._install(rc=100)
        self.assertFalse(ok)
        self.assertIn("sudo apt-get install -y xorriso", printed)

    def test_the_echoed_command_is_copyable_on_the_box_that_printed_it(self):
        """A root container usually has no sudo binary at all, so naming one
        gives the reader a command they cannot run. `run_privileged` already
        got this right; the manual line beside it did not."""
        _, _, printed = self._install(mode="root", rc=100)
        self.assertIn("Run: apt-get install -y xorriso", printed)
        self.assertNotIn("sudo", printed)

    def test_install_tool_reports_the_failure_up(self):
        """The other half of the chain the old test covered end to end: a
        refused install must come back as False through `install_tool`, which
        is what makes `main` file it as pending and exit nonzero."""
        tool = dev_setup.Tool(name="xorriso", probe=["xorriso"], apt="xorriso")
        with (
            mock.patch.object(dev_setup, "privilege_mode", return_value="none"),
            mock.patch.object(dev_setup.subprocess, "run") as run,
            mock.patch("builtins.print") as printed,
        ):
            self.assertFalse(dev_setup.install_tool(tool, "apt"))
        run.assert_not_called()
        self.assertIn("sudo apt-get install -y xorriso", _said(printed))


class TestInstalledButNotUsable(unittest.TestCase):
    """An install whose tool the probe still cannot find is not a success.

    This used to print `[installed] <tool> (not yet on PATH)`, count it as
    installed, and end the run "Setup complete" with exit 0 -- while `--check`
    on the same box exited 1. The two modes disagreed permanently, and the
    install one was the mode reading green on an unusable tool. pipx reaches
    this every time on a fresh Debian, where ~/.local/bin is on PATH only if it
    existed at login.
    """

    def _run_main(self, tool):
        with (
            mock.patch.object(dev_setup, "REQUIRED_TOOLS", [tool]),
            mock.patch.object(dev_setup, "OPTIONAL_TOOLS", []),
            mock.patch.object(dev_setup, "detect_os", return_value="brew"),
            mock.patch.object(dev_setup, "install_tool", return_value=True),
            mock.patch.object(dev_setup, "probe_tool", return_value=False),
            mock.patch.object(dev_setup, "gopls_status", return_value=("ok", "x")),
            mock.patch.object(dev_setup, "pyright_status", return_value=("ok", "x")),
            mock.patch.object(dev_setup, "vendor_go_deps", return_value=True),
            mock.patch.object(dev_setup.sys, "argv", ["dev-setup.py"]),
            mock.patch("builtins.print") as printed,
        ):
            code = dev_setup.main()
        return code, _said(printed)

    def test_exit_is_nonzero_and_says_where_to_look(self):
        tool = dev_setup.Tool(
            name="ze-fake-uv", probe=["ze-fake-uv"], pipx_install="ze-fake-uv"
        )
        code, printed = self._run_main(tool)
        self.assertEqual(code, 1, "a tool that cannot be run is not a complete setup")
        self.assertIn("not on PATH", printed)
        self.assertIn(".local/bin", printed)
        self.assertNotIn("Setup complete", printed)

    def test_a_go_tool_names_its_own_bin_directory(self):
        tool = dev_setup.Tool(
            name="ze-fake-gopls", probe=["ze-fake-gopls"], go_install="example.com/x@v1"
        )
        _, printed = self._run_main(tool)
        self.assertIn("go/bin", printed)


class TestGrubPackageFollowsTheHostArch(unittest.TestCase):
    """One GRUB module set per EFI target, one Debian package per host arch.

    Measured in debian:stable-slim on both arches: an arm64 host answers "E:
    Package 'grub-efi-amd64-bin' has no installation candidate" and installs
    NOTHING, `grub-mkstandalone` included, so the whole setup run ends short.
    The arm64 package installs there and carries grub-common, which is where
    `grub-mkstandalone` lives. An amd64 host takes the amd64 package.
    """

    def test_an_arm64_host_gets_the_arm64_module_set(self):
        """The discriminator: this was a constant amd64 name until it measured."""
        self.assertEqual(dev_setup.grub_apt_package("aarch64"), "grub-efi-arm64-bin")
        self.assertEqual(dev_setup.grub_apt_package("arm64"), "grub-efi-arm64-bin")

    def test_an_x86_host_gets_the_amd64_module_set(self):
        self.assertEqual(dev_setup.grub_apt_package("x86_64"), "grub-efi-amd64-bin")

    def test_both_grub_surfaces_name_the_same_package(self):
        """The tool row installs it; APPLIANCE_CHECKS documents what installs
        the doctor check. Two spellings drift, and only one of them is tested."""
        grub = [t for t in dev_setup.REQUIRED_TOOLS if t.name == "grub"][0]
        self.assertEqual(grub.apt, dev_setup.GRUB_APT_PACKAGE)
        self.assertEqual(
            dev_setup.APPLIANCE_CHECKS["appliance-grub"]["apt"],
            dev_setup.GRUB_APT_PACKAGE,
        )


class TestUvIsReachableOnLinux(unittest.TestCase):
    """uv had `apt=None` and no other installer, so on Linux it fell out.

    `_has_package` answers False for that state, and the loop in `main` prints
    `[skipped]` for it and never appends to `missing_required` -- so check mode
    reported "All required tools present" on a box with no uv, and the evidence
    SSH probe (`uv run --with paramiko`) failed later with no clue why. A guard
    that reads green on absence is worse than no guard.
    """

    def _tool(self, name):
        for tool in dev_setup.REQUIRED_TOOLS:
            if tool.name == name:
                return tool
        return None

    def test_uv_is_still_required(self):
        self.assertIsNotNone(self._tool("uv"), "uv missing from REQUIRED_TOOLS")

    def test_uv_has_an_installer_on_both_platforms(self):
        """The discriminator: restore `apt=None` with no pipx route and the
        apt half of this fails, which is exactly the state that read green."""
        uv = self._tool("uv")
        self.assertTrue(dev_setup._has_package(uv, "apt"))
        self.assertTrue(dev_setup._has_package(uv, "brew"))

    def test_uv_installs_through_pipx(self):
        uv = self._tool("uv")
        for pkg_mgr in ("apt", "brew"):
            with (
                mock.patch.object(
                    dev_setup.shutil, "which", return_value="/usr/bin/pipx"
                ),
                mock.patch.object(dev_setup.subprocess, "run") as run,
                mock.patch("builtins.print"),
            ):
                run.return_value = _completed(0)
                self.assertTrue(dev_setup.install_tool(uv, pkg_mgr))
            self.assertEqual(run.call_args[0][0], ["pipx", "install", "--force", "uv"])

    def test_pipx_is_installed_before_uv_needs_it(self):
        """Order in the list is the install order, and pipx is uv's installer.

        Move uv above the pipx row and a fresh box prints "SKIP uv: pipx not
        available yet" instead of installing it.
        """
        names = [tool.name for tool in dev_setup.REQUIRED_TOOLS]
        self.assertLess(names.index("pipx"), names.index("uv"))


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


class TestDevSetupRequiresPinnedStaticcheck(unittest.TestCase):
    """The feature-tag matrix needs standalone Staticcheck, not the lint wrapper."""

    def test_required_staticcheck_pin(self):
        tools = [
            tool
            for tool in dev_setup.REQUIRED_TOOLS
            if tool.name == "staticcheck"
        ]
        self.assertEqual(len(tools), 1, "staticcheck must occur once in REQUIRED_TOOLS")
        self.assertEqual(tools[0].probe, ["staticcheck"])
        self.assertEqual(
            tools[0].go_install,
            "honnef.co/go/tools/cmd/staticcheck@2026.1",
        )


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
                mock.patch.dict(
                    dev_setup.os.environ,
                    {"ZE_SETUP_TEST_ENV": "preserved", "CGO_ENABLED": "1"},
                ),
                mock.patch.object(dev_setup.subprocess, "run") as run,
            ):
                run.return_value = subprocess.CompletedProcess([], 0, b"", b"")
                self.assertTrue(dev_setup.install_tool(tool, pkg_mgr))
                self.assertEqual(dev_setup.os.environ["CGO_ENABLED"], "1")
            self.assertEqual(
                run.call_args[0][0],
                ["go", "install", "golang.org/x/tools/gopls@latest"],
            )
            child_env = run.call_args.kwargs["env"]
            self.assertEqual(child_env["ZE_SETUP_TEST_ENV"], "preserved")
            self.assertEqual(child_env["CGO_ENABLED"], "0")


def _completed(returncode: int, stdout: bytes = b"", stderr: bytes = b""):
    return subprocess.CompletedProcess([], returncode, stdout, stderr)


def _said(printed) -> str:
    """Everything a mocked `print` was given, as one string to search."""
    return " ".join(str(call) for call in printed.call_args_list)


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


class TestPyrightIsRequired(unittest.TestCase):
    """pyright is the Go story's Python half.

    405 Read calls in the measured transcript store named a `.py` file while no
    Python language server existed on the machine, so every symbol question
    about `scripts/dev/` or `.claude/hooks/` was answered by reading the file
    (ai/rules/context-economy.md). This setup entry is what installs the server.
    """

    def _pyright(self):
        for tool in dev_setup.REQUIRED_TOOLS:
            if tool.name == "pyright":
                return tool
        return None

    def test_pyright_is_a_required_tool(self):
        self.assertIsNotNone(self._pyright(), "pyright missing from REQUIRED_TOOLS")

    def test_pyright_probes_the_language_server_binary(self):
        """`pyright` alone is the CLI. The LSP tool spawns `pyright-langserver`.

        Both ship from the one pipx package, so probing only the CLI would call
        a half-install complete on any machine where the console script for the
        server failed to link.
        """
        self.assertIn("pyright-langserver", self._pyright().probe)

    def test_pyright_installs_via_pipx(self):
        """The install actually taken is `pipx install`, on every package manager.

        Same reasoning as the gopls case above: `_has_package` returns True for
        anything carrying `pipx_install` before it consults brew or apt, so only
        the command `install_tool` runs can fail on its own.
        """
        tool = self._pyright()
        self.assertEqual(tool.pipx_install, "pyright")
        for pkg_mgr in ("apt", "brew"):
            with (
                mock.patch.object(
                    dev_setup.shutil, "which", return_value="/usr/bin/pipx"
                ),
                mock.patch.object(dev_setup.subprocess, "run") as run,
            ):
                run.return_value = subprocess.CompletedProcess([], 0, b"", b"")
                self.assertTrue(dev_setup.install_tool(tool, pkg_mgr))
            self.assertEqual(
                run.call_args[0][0],
                ["pipx", "install", "--force", "pyright"],
            )


class TestPyrightAnswers(unittest.TestCase):
    """pyright on PATH is not a working language server either.

    It bootstraps a node runtime the first time it runs, so a binary that has
    never run is the same silent failure gopls was in. This check RUNS it.
    """

    def _reply(self, analyzed: int = 1, errors: int = 0) -> bytes:
        return json.dumps(
            {
                "version": "1.1.411",
                "generalDiagnostics": [],
                "summary": {"filesAnalyzed": analyzed, "errorCount": errors},
            }
        ).encode()

    def _status(self, probe, present: bool = True):
        which = "/home/x/.local/bin/pyright" if present else None
        with mock.patch.object(dev_setup.shutil, "which", return_value=which):
            return dev_setup.pyright_status(probe=probe)

    def test_the_probe_file_is_this_script(self):
        """Why `pyright_status` has no "n/a" state, asserted rather than assumed.

        `gopls_status` carries one for the day its probe file is renamed. This
        probe names the running script, so that state is unreachable and the
        branch is absent. Repoint PYRIGHT_PROBE_FILE at any other file and this
        fails, which is the signal to put the branch back.
        """
        self.assertEqual(
            (dev_setup.REPO_ROOT / dev_setup.PYRIGHT_PROBE_FILE).resolve(),
            Path(dev_setup.__file__).resolve(),
        )

    def test_a_node_bootstrap_preamble_is_not_a_broken_server(self):
        """The first run on a machine with no global node, and only the first.

        The pipx wrapper installs a node runtime through nodeenv, which writes
        its progress to the stdout this probe captures: `_install_node_env`
        (`pyright/node.py`) runs it with no redirection, and only the npm path
        is silenced. `json.loads` on the whole capture refuses the reply, so the
        run that just succeeded at installing node is the run that reds, on the
        fresh Linux box `ze-dev-setup` exists to prepare. The second run is green,
        which is what makes this read as flakiness rather than a bug.

        The preamble is a Python dict repr, single-quoted, so it is not itself
        valid JSON: `pyright_summary` scans past it rather than guessing.
        """
        preamble = (
            b"{'x86': False, 'risc': False, 'lts': False}\n"
            b" * Install prebuilt node (26.7.0) ..... done.\n"
        )
        state, detail = self._status(lambda: _completed(0, preamble + self._reply()))
        self.assertEqual(state, "ok")
        self.assertIn("1 file analysed", detail)

    def test_answering_server_is_ok(self):
        state, detail = self._status(lambda: _completed(0, self._reply()))
        self.assertEqual(state, "ok")
        self.assertIn("1 file analysed", detail)
        self.assertIn(dev_setup.PYRIGHT_PROBE_FILE, detail)

    def test_a_type_error_in_the_probe_file_is_still_a_working_server(self):
        """The discriminating case: pyright exits 1 when it finds a diagnostic.

        Key this check on the exit code and it goes red the day somebody's
        script gains a type error, which says nothing about the server. Flip
        `pyright_status` to `returncode != 0` and only this test fails.
        """
        state, detail = self._status(lambda: _completed(1, self._reply(errors=3)))
        self.assertEqual(state, "ok")
        self.assertIn("1 file analysed", detail)

    def test_absent_binary_is_not_the_same_finding_as_a_mute_server(self):
        """The two need different fixes, so they must not share a message."""

        def unreachable():
            raise AssertionError("probe must not run when pyright is absent")

        absent_state, absent_detail = self._status(unreachable, present=False)
        mute_state, mute_detail = self._status(lambda: _completed(0, b"\n"))
        self.assertEqual(absent_state, "absent")
        self.assertEqual(mute_state, "broken")
        self.assertEqual(absent_detail, dev_setup.PYRIGHT_NOT_INSTALLED)
        self.assertIn(dev_setup.PYRIGHT_NOT_ANSWERING, mute_detail)
        self.assertNotIn(dev_setup.PYRIGHT_NOT_ANSWERING, absent_detail)

    def test_analysing_nothing_is_not_an_answer(self):
        """Valid JSON and exit 0 is not proof: a file must have been analysed."""
        state, detail = self._status(lambda: _completed(0, self._reply(analyzed=0)))
        self.assertEqual(state, "broken")
        self.assertIn("analysed no file", detail)

    def test_failing_server_reports_its_own_message(self):
        state, detail = self._status(
            lambda: _completed(1, b"", b"err: node download failed\nmore\n")
        )
        self.assertEqual(state, "broken")
        self.assertIn("node download failed", detail)
        # Only the first line of the server's complaint, not its whole output.
        self.assertNotIn("more", detail)

    def test_timeout_is_a_mute_server_not_a_missing_one(self):
        def slow():
            raise subprocess.TimeoutExpired(cmd="pyright", timeout=1)

        state, detail = self._status(slow)
        self.assertEqual(state, "broken")
        self.assertIn(str(dev_setup.PYRIGHT_PROBE_TIMEOUT), detail)

    def test_check_mode_reports_the_probe(self):
        r = subprocess.run(
            [sys.executable, SCRIPT, "--check"],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
        )
        self.assertIn("pyright-answers", r.stdout.decode())


class TestLoopbackAddresses(unittest.TestCase):
    """The addresses a fixture binds, which the test runner cannot add itself.

    A BGP session needs a different address at each end (RFC 4271 Section 5.1.3
    forbids a peer its own address as NEXT_HOP), and IPv6 has one loopback
    address per host. Adding a second one needs root, so it happens here.
    """

    def test_ipv6_is_unique_local_on_every_platform(self):
        """fd00::/8 is RFC 4193 unique-local: never globally routable, so a
        fixture that leaks a packet toward it cannot reach a real destination.
        """
        self.assertTrue(dev_setup.LOOPBACK_IPV6.startswith("fd"))
        for system in ("Darwin", "Linux"):
            with mock.patch.object(dev_setup.platform, "system", return_value=system):
                self.assertIn(dev_setup.LOOPBACK_IPV6, dev_setup.loopback_addresses())

    def test_ipv4_aliases_are_darwin_only(self):
        """Linux routes all of 127.0.0.0/8 to lo, so an alias there is work with
        no effect. macOS binds only 127.0.0.1 until each alias is added.
        """
        with mock.patch.object(dev_setup.platform, "system", return_value="Darwin"):
            self.assertIn("127.0.0.2", dev_setup.loopback_addresses())
            self.assertIn("127.0.0.5", dev_setup.loopback_addresses())
        with mock.patch.object(dev_setup.platform, "system", return_value="Linux"):
            self.assertEqual(dev_setup.loopback_addresses(), [dev_setup.LOOPBACK_IPV6])

    def test_the_add_command_matches_the_platform(self):
        with mock.patch.object(dev_setup.platform, "system", return_value="Darwin"):
            self.assertEqual(
                dev_setup.loopback_add_argv("fd00::2"),
                ["ifconfig", "lo0", "inet6", "fd00::2/128", "alias"],
            )
            self.assertEqual(
                dev_setup.loopback_add_argv("127.0.0.2"),
                ["ifconfig", "lo0", "alias", "127.0.0.2"],
            )
        with mock.patch.object(dev_setup.platform, "system", return_value="Linux"):
            self.assertEqual(
                dev_setup.loopback_add_argv("fd00::2"),
                ["ip", "-6", "addr", "add", "fd00::2/128", "dev", "lo"],
            )

    def test_the_probe_answers_on_a_bind(self):
        """::1 is on every host; the documentation prefix (RFC 3849) is on none."""
        self.assertTrue(dev_setup.loopback_bindable("::1"))
        self.assertFalse(dev_setup.loopback_bindable("2001:db8::1"))

    def test_only_missing_addresses_are_added(self):
        """Idempotence is structural: a configured host passes an empty list, so
        the re-run of `make ze-dev-setup` runs no command at all.
        """
        with mock.patch.object(dev_setup, "loopback_bindable", return_value=True):
            self.assertEqual(dev_setup.missing_loopback_addresses(), [])

        with (
            mock.patch.object(dev_setup, "run_privileged") as privileged,
            mock.patch.object(dev_setup, "missing_loopback_addresses", return_value=[]),
            mock.patch("builtins.print"),
        ):
            self.assertTrue(dev_setup.apply_loopback_fix([]))
            privileged.assert_not_called()

    def test_each_missing_address_reaches_run_privileged(self):
        with (
            mock.patch.object(dev_setup.platform, "system", return_value="Linux"),
            mock.patch.object(
                dev_setup, "run_privileged", return_value=(True, "")
            ) as privileged,
            mock.patch.object(dev_setup, "missing_loopback_addresses", return_value=[]),
            mock.patch("builtins.print"),
        ):
            self.assertTrue(dev_setup.apply_loopback_fix(["fd00::2"]))
        self.assertEqual(
            privileged.call_args[0][0],
            ["ip", "-6", "addr", "add", "fd00::2/128", "dev", "lo"],
        )

    def test_an_address_that_did_not_appear_is_a_failure(self):
        """The command can exit 0 and leave the address unusable. The bind probe
        after the fact is what decides, not the exit code.
        """
        with (
            mock.patch.object(dev_setup, "run_privileged", return_value=(True, "")),
            mock.patch.object(
                dev_setup, "missing_loopback_addresses", return_value=["fd00::2"]
            ),
            mock.patch("builtins.print"),
        ):
            self.assertFalse(dev_setup.apply_loopback_fix(["fd00::2"]))

    def test_no_route_to_root_says_what_to_run(self):
        with (
            mock.patch.object(dev_setup.platform, "system", return_value="Darwin"),
            mock.patch.object(
                dev_setup, "run_privileged", return_value=(False, "no route")
            ),
            mock.patch("builtins.print") as printed,
        ):
            self.assertFalse(dev_setup.apply_loopback_fix(["fd00::2"]))
            dev_setup.print_loopback_fix(["fd00::2"])
        self.assertIn("sudo ifconfig lo0 inet6 fd00::2/128 alias", _said(printed))

    def test_check_mode_reports_and_changes_nothing(self):
        """`make ze-dev-setup CHECK=1` must never touch the interface."""
        with (
            mock.patch.object(dev_setup, "run_privileged") as privileged,
            mock.patch.object(
                dev_setup, "missing_loopback_addresses", return_value=["fd00::2"]
            ),
            mock.patch.object(dev_setup.sys, "argv", ["dev-setup.py", "--check"]),
            mock.patch("builtins.print") as printed,
        ):
            dev_setup.main()
        privileged.assert_not_called()
        self.assertIn("loopback-addresses", _said(printed))


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
