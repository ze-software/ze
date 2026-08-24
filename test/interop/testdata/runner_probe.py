#!/usr/bin/env python3
"""Drive run.py's per-scenario failure handler with no Docker and no containers.

Guards in that handler exist because the interop runner LOST its summary, and a
lost summary is worse than a red one: the tallies for every scenario the suite
already ran go with it, and the operator is left with an exit code and a
traceback. A separate guard exists because the same removal runs as a PRE-CLEAN,
where continuing past a failure leaves a scenario racing another one's daemon.

None of it could go red on its own, which is what this driver fixes. It is
invoked by `run_test.go`, because there is no Python test root under
`test/interop/` (`pythonTestRoots`, `scripts/dev/python_tests_test.go`) and this
package already drives `run.py` as a subprocess from Go.

    runner_probe.py interrupt-note <scenario>
    runner_probe.py <phase>-<target>-<failure> <scenario>

`interrupt-note` raises `KeyboardInterrupt` inside the up-to-15s `docker logs`
read the handler performs after a scenario fails. Before the guard, that
interrupt escaped `main` and no summary printed at all.

The three-part modes break exactly ONE Docker removal, which is the point.

  phase    `setup` runs the REAL pre-clean, which must DENY. `teardown` runs the
           cleanup path from run.py's `finally`, which must report and continue.
  target   `container` breaks the ten `docker rm -f` calls, `network` breaks the
           `docker network rm`. Breaking both at once made a test green with
           either half of the guard deleted, because each half printed the same
           words the assertion matched.
  failure  `timeout` never answers, `error` answers non-zero, `oserror` cannot
           run at all (a missing docker binary). All three leave the object
           standing, so the pre-clean denies on all three and the cleanup path
           reports all three without raising. `teardown-*-error` is REFUSED:
           the cleanup contract does not read the exit code, by design, so
           asserting on it would assert nothing.

           Two more exist for the network only, and they are the two sides of
           its one exemption. `absent` is the ordinary missing network, which
           the pre-clean must let THROUGH. `notfound` is a different failure
           whose text merely contains the words "not found", which it must
           still DENY: a misconfigured `DOCKER_CONTEXT` answers that way having
           removed nothing.

Two more modes drive the IMAGE the containers are started from, which is the one
piece of lab state the per-run suffix never covered.

    runner_probe.py image-pinned <scenario>
    runner_probe.py image-nobuild <scenario>

`image-pinned` runs the real `build_images`, answering each `docker build -q`
with a distinguishable ID, and prints every `docker run` argv. Every container
must be started from an ID, because a tag is shared and a concurrent build
rebinds it. `image-nobuild` is the `NO_BUILD=1` half: nothing is built, so the
shared tag is the only reference there is and it must still be used.

No mode starts a container, and all of them stub the runner's Docker probe, so
the result is identical on a host with Docker and on one without.
"""

import os
import subprocess
import sys

PROBE_DIR = os.path.dirname(os.path.abspath(__file__))
INTEROP_DIR = os.path.dirname(PROBE_DIR)
sys.path.insert(0, INTEROP_DIR)

import interop  # noqa: E402
import run  # noqa: E402

FAILURE = "BIRD route 10.55.1.0/24 not found after 60s"

_REAL_RUN = subprocess.run

# What docker answers when the object survives. A missing container exits 0 and
# a missing network exits 1 "not found", so neither of these is the ordinary
# nothing-to-remove case.
_ERRORS = {
    "container": "Error response from daemon: removal already in progress",
    "network": (
        "Error response from daemon: error while removing network: has active endpoints"
    ),
}


class _DockerOK:
    """A docker call that succeeded and printed nothing."""

    returncode = 0
    stdout = ""
    stderr = ""


class _DockerFailed:
    """What a refused `docker run` returns, so a broken case ends quickly."""

    returncode = 1
    stdout = ""
    stderr = "probe: docker run refused"


class _Result:
    """A docker call that ANSWERED, with the code and text given."""

    def __init__(self, returncode, stderr="", stdout=""):
        self.returncode = returncode
        self.stdout = stdout
        self.stderr = stderr


def _stub_docker_probe(*args, **kwargs):
    return _DockerOK()


def _breaking_stub(target, failure):
    """Break one removal, succeed at everything else.

    ONE stub, dispatching on the command, because `run.subprocess` and
    `interop.subprocess` are the same module object: patching the attribute
    through either name reaches both, so two stubs cannot coexist.

    `docker run` is refused so a REGRESSION of the pre-clean guard ends the
    scenario in a second or two instead of running on into the health waiters.
    A test whose broken case hangs reports a timeout rather than the defect.
    """

    def stub(*args, **kwargs):
        cmd = args[0] if args else []
        hit = (target == "container" and cmd[:3] == ["docker", "rm", "-f"]) or (
            target == "network" and cmd[:3] == ["docker", "network", "rm"]
        )
        if hit:
            if failure == "timeout":
                raise subprocess.TimeoutExpired(cmd=cmd, timeout=30)
            if failure == "oserror":
                raise FileNotFoundError(2, "No such file or directory: 'docker'")
            if failure == "absent":
                return _Result(
                    1,
                    "Error response from daemon: network %s not found"
                    % interop.NETWORK,
                )
            if failure == "notfound":
                return _Result(
                    1,
                    "Failed to initialize: unable to resolve docker endpoint: "
                    'context "zz-probe": context not found',
                )
            return _Result(1, _ERRORS[target])
        if cmd[:2] == ["docker", "run"]:
            return _DockerFailed()
        return _DockerOK()

    return stub


def _image_stub():
    """Answer every build with a distinguishable ID and record every container start.

    The recorded argv is printed with `%r`, so an assertion can name an argv
    ELEMENT rather than a substring. That distinction is the whole value here: a
    volume mount carries the repository path, and a substring match for a tag
    would be satisfied by any path that happened to contain it.

    `docker run` succeeds until the Ze container, then refuses, so setup always
    reaches the Ze start whatever sidecars the chosen scenario carries and the
    scenario still ends in a second instead of waiting out a health poll.
    """

    def stub(*args, **kwargs):
        cmd = args[0] if args else []
        if cmd[:2] == ["docker", "build"]:
            tag = cmd[cmd.index("-t") + 1]
            return _Result(0, stdout="sha256:probe-%s\n" % tag)
        if cmd[:2] == ["docker", "run"]:
            print("DOCKER_RUN=%r" % (list(cmd),))
            if cmd[4] == interop.ZE_CONTAINER:
                return _DockerFailed()
        return _DockerOK()

    return stub


class FakeScenario:
    """Fails in `run_check`, touches nothing outside this process."""

    def __init__(self, scenario_dir, frr_image):
        pass

    def setup(self):
        pass

    def run_check(self):
        raise AssertionError(FAILURE)

    def teardown(self):
        pass


_INSTANCES = []


class RecordingScenario(interop.Scenario):
    """The real Scenario, kept reachable so the probe can report its state.

    `probe_rendered` remembers the path that was rendered, and `probe_existed`
    remembers that it was really there. `teardown` clears `rendered_dir` whether
    or not the copy actually went away, so reporting the attribute alone would
    answer "cleared" for a removal that silently failed, and reporting the path
    alone would answer "gone" for a copy that was never created.
    """

    probe_rendered = None
    probe_existed = None

    def __init__(self, scenario_dir, frr_image):
        super().__init__(scenario_dir, frr_image)
        _INSTANCES.append(self)

    def _capture(self):
        """Record the rendered path AND that it was really there.

        Both halves are needed. "Gone after teardown" is also true of a copy
        that was never created, so without the precondition a render that
        quietly produced nothing would keep every teardown assertion green.
        """
        self.probe_rendered = self.rendered_dir
        self.probe_existed = os.path.isdir(self.rendered_dir)

    def setup(self):
        try:
            super().setup()
        finally:
            if self.rendered_dir:
                self._capture()


class RealTeardownScenario(RecordingScenario):
    """Renders as the real setup does, fails, then runs the REAL teardown.

    The render is load-bearing and not decoration. `teardown` clears the
    rendered copy in a `finally`, and a removal that takes its early return
    used to skip that. With `rendered_dir` left at None there is nothing to
    leak, so the guard could be deleted with every test still green.
    """

    def setup(self):
        self.rendered_dir = interop._render_scenario_dir(  # noqa: SLF001
            self.source_dir, self.name
        )
        self.scenario_dir = self.rendered_dir
        self._capture()

    def run_check(self):
        raise AssertionError(FAILURE)


def _configure(mode):
    """Install the stub and the Scenario this mode needs, or return False."""
    if mode == "interrupt-note":
        subprocess.run = _stub_docker_probe
        run.Scenario = FakeScenario

        def interrupting_note(container=None):
            raise KeyboardInterrupt()

        run.observer_failure_note = interrupting_note
        return True

    if mode in ("image-pinned", "image-nobuild"):
        # The only modes that let `build_images` run, so they are the only ones
        # that must not force NO_BUILD. `image-nobuild` sets it back on to drive
        # the other half.
        os.environ["NO_BUILD"] = "1" if mode == "image-nobuild" else "0"
        subprocess.run = _image_stub()
        run.observer_failure_note = lambda container=None: None
        run.Scenario = RecordingScenario
        return True

    parts = mode.split("-")
    if len(parts) != 3:
        return False
    phase, target, failure = parts
    if phase not in ("setup", "teardown"):
        return False
    if target not in ("container", "network"):
        return False
    if failure not in ("timeout", "error", "oserror", "absent", "notfound"):
        return False
    if phase == "teardown" and failure in ("error", "absent", "notfound"):
        # The cleanup contract does not read the exit code, so a dedicated
        # teardown mode for an exit-code shape would assert nothing. Refuse
        # rather than pass vacuously.
        #
        # That contract is not untested, though: `run.py`'s `finally` tears down
        # AGAIN after a failed setup, against the same broken stub, so every
        # `setup-*-error` mode drives the cleanup path for free. Stop the setup
        # modes tearing down twice and that coverage goes with them.
        return False
    if target == "container" and failure in ("absent", "notfound"):
        # The exemption belongs to the network removal alone: `docker rm -f` on
        # a missing container exits 0, so it has no message to exempt.
        return False

    subprocess.run = _breaking_stub(target, failure)
    run.observer_failure_note = lambda container=None: None
    # The REAL Scenario for `setup`, so its own pre-clean runs.
    run.Scenario = RecordingScenario if phase == "setup" else RealTeardownScenario
    return True


def main():
    if len(sys.argv) != 3:
        print("usage: runner_probe.py <mode> <scenario>", file=sys.stderr)
        return 2

    mode, scenario = sys.argv[1], sys.argv[2]
    os.environ["NO_BUILD"] = "1"

    if not _configure(mode):
        print("unknown mode %s" % mode, file=sys.stderr)
        return 2

    sys.argv = ["run.py", scenario]
    try:
        run.main()
    except SystemExit as exc:
        print("EXIT=%s" % exc.code)
        return 0
    except BaseException as exc:  # noqa: BLE001  # the defect under test
        print("ESCAPED=%s: %s" % (type(exc).__name__, exc))
        return 0
    finally:
        # `global_cleanup` runs at interpreter exit and calls docker for real.
        # Leaving a stub installed would make that atexit hook raise.
        subprocess.run = _REAL_RUN
        # Whether teardown cleared the rendered copy, which is host-side state
        # no docker stub can fake.
        for inst in _INSTANCES:
            print("RENDERED_LEFT=%s" % (inst.rendered_dir or "none"))
            # Three prints, and each answers a different question, because no
            # two of them are enough on their own.
            #
            #   RENDERED_LEFT     what teardown BELIEVES. Cleared unconditionally,
            #                     so it says "none" for a copy still on disk.
            #   RENDERED_EXISTED  whether a copy was really made, observed at
            #                     capture time. Without it, "gone afterwards" is
            #                     also true of a render that produced nothing.
            #   RENDERED_ON_DISK  what the filesystem holds NOW, read from the
            #                     remembered path. The only one that catches a
            #                     removal that failed quietly.
            path = inst.probe_rendered
            print("RENDERED_EXISTED=%s" % (inst.probe_existed if path else "no-render"))
            print(
                "RENDERED_ON_DISK=%s" % (os.path.isdir(path) if path else "no-render")
            )
    print("ESCAPED=none-but-no-exit")
    return 0


if __name__ == "__main__":
    sys.exit(main())
