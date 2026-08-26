#!/usr/bin/env python3
"""The four Docker labs remove a container under the SAME two contracts.

Every lab runs the same removal at two moments that owe opposite answers.

  cleanup    from the runner's `finally`, after every scenario. It must raise
             nothing: an exception there escapes the `finally` and the run ends
             with no summary, discarding the tally of every scenario the suite
             already finished.
  pre-clean  from `Scenario.setup`, before this scenario starts anything. It
             must raise: a removal that failed silently leaves this scenario
             running beside an earlier run's daemon on the same address, and
             nothing downstream catches that. A container THIS scenario starts
             collides by name and is reported; a stale peer it never starts is
             invisible, and `docker network create` accepts a network that
             already exists.

`test/interop/interop.py` carried both contracts. The other three defined their
own `docker_rm` with a bare `subprocess.run`, no returncode read and no
exception handling, so all three failure shapes were silent on both paths.

Three shapes, because each leaves the container standing and the first version
of this guard read only the timeout:

  * the call never answers                    TimeoutExpired
  * docker answers with an error              a non-zero exit. `docker rm -f` on
                                              a container that is not there
                                              exits 0, so a non-zero exit is
                                              always a real failure
  * the call cannot run at all                OSError, a missing or unusable
                                              docker binary

The pre-clean half is asserted through `Scenario.setup`, its real entry point,
not through the helper alone: a helper that denies proves the helper denies, not
that the caller passes `strict` (ai/rules/evidence.md).

Run: python3 scripts/dev/lab_docker_rm_test.py
(also picked up automatically by TestPythonUnitTests, scripts/dev/python_tests_test.go)
"""

from __future__ import annotations

import atexit
import contextlib
import importlib.util
import io
import os
import pathlib
import subprocess
import unittest
from unittest import mock

ROOT = pathlib.Path(__file__).resolve().parents[2]

# Each lab, by the module name to import it under and the file that holds it.
# `scenario` builds a Scenario for the pre-clean test from a real scenario
# directory, because two of the three constructors read files out of it.
LABS = {
    "bgp": ROOT / "test" / "interop" / "interop.py",
    "ipsec": ROOT / "test" / "interop-ipsec" / "lab.py",
    "l2tp": ROOT / "test" / "interop-l2tp" / "lab.py",
    "pppoe": ROOT / "test" / "interop-pppoe" / "lab.py",
}

# The BGP lab's own pre-clean is driven end to end from Go
# (test/interop/run_test.go, TestInteropRunnerDeniesWhenThePreCleanFails), so
# only the three siblings need their entry point covered here.
PRECLEAN_LABS = ("ipsec", "l2tp", "pppoe")

SCENARIO_DIRS = {
    "ipsec": ROOT / "test" / "interop-ipsec" / "scenarios" / "psk-site-to-site",
    "l2tp": ROOT / "test" / "interop-l2tp" / "scenarios" / "01-ppp-ipv4",
    "pppoe": ROOT / "test" / "interop-pppoe" / "scenarios" / "01-pppoe-chap-ipv4",
}

CONTAINER = "ze-lab-docker-rm-probe"


def _load(name: str, path: pathlib.Path):
    """Import a lab module by path, with its atexit cleanup disarmed.

    Importing a lab registers `global_cleanup`, which force-removes that lab's
    containers with the REAL docker at interpreter exit. Those names carry no
    per-run suffix, so leaving it armed would let this test delete the lab of a
    run happening on the same host.
    """
    spec = importlib.util.spec_from_file_location(f"ze_lab_{name}", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    cleanup = getattr(module, "global_cleanup", None)
    if cleanup is not None:
        atexit.unregister(cleanup)
    return module


MODULES = {name: _load(name, path) for name, path in LABS.items()}


class _Answered:
    """A docker call that answered, with the code and text given."""

    def __init__(self, returncode: int, stderr: str = "", stdout: str = ""):
        self.returncode = returncode
        self.stdout = stdout
        self.stderr = stderr


def _timeout(*args, **kwargs):
    raise subprocess.TimeoutExpired(cmd=args[0] if args else [], timeout=30)


def _oserror(*args, **kwargs):
    raise FileNotFoundError(2, "No such file or directory: 'docker'")


def _error(*args, **kwargs):
    return _Answered(1, "Error response from daemon: removal already in progress")


# The message each shape must carry, so a test that matched only the shared
# wording could not stay green with one branch deleted.
SHAPES = {
    "timeout": (_timeout, "timed out after 30s"),
    "error": (_error, "exit 1"),
    "oserror": (_oserror, "could not run"),
}


class TestCleanupSwallowsAndReports(unittest.TestCase):
    """The default contract raises nothing, and still says what happened."""

    def test_every_lab_returns_on_every_failure_shape(self):
        """No shape raises here. Returning IS the contract."""
        for lab, module in MODULES.items():
            for shape, (stub, _) in SHAPES.items():
                with self.subTest(lab=lab, shape=shape):
                    out = io.StringIO()
                    with mock.patch.object(subprocess, "run", stub):
                        with contextlib.redirect_stdout(out):
                            module.docker_rm(CONTAINER)
                    if shape == "error":
                        # Silent by design; pinned by the test below.
                        continue
                    self.assertIn(
                        CONTAINER,
                        out.getvalue(),
                        f"{lab}/{shape}: a swallowed cleanup failure must still "
                        "name the container it could not remove",
                    )

    def test_the_cleanup_report_does_not_read_the_exit_code(self):
        """A non-zero exit is silent on this path, by design.

        The exit code is read only to DECIDE, and the cleanup contract decides
        nothing. Pinning the silence keeps the two contracts distinguishable: it
        is what makes `exit 1` a one-producer assertion in the strict test below.
        """
        for lab, module in MODULES.items():
            with self.subTest(lab=lab):
                out = io.StringIO()
                with mock.patch.object(subprocess, "run", _error):
                    with contextlib.redirect_stdout(out):
                        module.docker_rm(CONTAINER)
                self.assertEqual(out.getvalue(), "", f"{lab}: cleanup must stay quiet")


class TestPreCleanDenies(unittest.TestCase):
    """`strict=True` refuses, and names the container and the consequence."""

    def test_every_lab_raises_on_every_failure_shape(self):
        for lab, module in MODULES.items():
            for shape, (stub, wording) in SHAPES.items():
                with self.subTest(lab=lab, shape=shape):
                    with mock.patch.object(subprocess, "run", stub):
                        with self.assertRaises(RuntimeError) as caught:
                            module.docker_rm(CONTAINER, strict=True)
                    message = str(caught.exception)
                    self.assertIn(CONTAINER, message, f"{lab}/{shape}")
                    self.assertIn(
                        "would race this scenario",
                        message,
                        f"{lab}/{shape}: the denial must say what a leftover costs",
                    )
                    self.assertIn(
                        wording,
                        message,
                        f"{lab}/{shape}: the denial must name HOW the removal failed",
                    )


class TestSetupPreCleansStrictly(unittest.TestCase):
    """The entry point passes `strict`, and stops before it creates anything."""

    def _scenario(self, lab, module):
        path = str(SCENARIO_DIRS[lab])
        self.assertTrue(os.path.isdir(path), f"{lab}: {path} is missing")
        if lab == "pppoe":
            return module.Scenario(path)
        return module.Scenario(path, "quay.io/frrouting/frr:10.3.1")

    def test_a_failed_removal_stops_setup_before_the_network(self):
        for lab in PRECLEAN_LABS:
            module = MODULES[lab]
            for shape, (stub, _) in SHAPES.items():
                with self.subTest(lab=lab, shape=shape):
                    issued = []

                    def recording(*args, **kwargs):
                        cmd = args[0] if args else []
                        issued.append(list(cmd))
                        if list(cmd[:3]) == ["docker", "rm", "-f"]:
                            return stub(*args, **kwargs)
                        return _Answered(0)

                    scenario = self._scenario(lab, module)
                    with mock.patch.object(subprocess, "run", recording):
                        with self.assertRaises(RuntimeError) as caught:
                            scenario.setup()
                    self.assertIn("would race this scenario", str(caught.exception))
                    created = [
                        c for c in issued if c[:3] == ["docker", "network", "create"]
                    ]
                    self.assertEqual(
                        created,
                        [],
                        f"{lab}/{shape}: setup reached the network create after a "
                        "failed pre-clean, so the scenario would have run beside "
                        "whatever it could not remove",
                    )


if __name__ == "__main__":
    unittest.main()
