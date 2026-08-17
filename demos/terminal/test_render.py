#!/usr/bin/env python3

import importlib.util
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock


HERE = pathlib.Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location("terminal_render", HERE / "render.py")
terminal_render = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(terminal_render)

# A process that holds an exclusive lock on argv[1], announces it by creating
# argv[2], and holds it for argv[3] seconds. Scaffolding for the contention
# tests below: they need a rival that genuinely owns the lock before the code
# under test asks for it.
#
# This was `flock -F <path> sleep <n>`. `-F` is util-linux's --no-fork, and it
# is what made kill() reach the sleep rather than orphan it. The flock(1) on a
# macOS dev machine is the BSD reimplementation (discoteq flock 0.4.0), which
# has no -F: the holder died on `flock: invalid option -- F`, the lock file was
# never created, and both tests failed against code that was correct. flock(2)
# from Python has no flavour to get wrong, and one process needs no --no-fork.
#
# The ready marker closes a race the flock(1) holder had: the lock FILE exists
# from the moment it is opened, which is before the lock is held.
LOCK_HOLDER_SOURCE = """
import fcntl, pathlib, sys, time
handle = pathlib.Path(sys.argv[1]).open("w")
fcntl.flock(handle, fcntl.LOCK_EX)
pathlib.Path(sys.argv[2]).touch()
time.sleep(float(sys.argv[3]))
"""


class SourceDigestTest(unittest.TestCase):
    def test_ignores_page_metadata_but_tracks_render_inputs(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            demo_root = root / "demos" / "terminal"
            source_dir = demo_root / "example"
            source_dir.mkdir(parents=True)
            shared = demo_root / "common.tape"
            shared.write_text("Set Width 1200\n")
            source = source_dir / "demo.tape"
            source.write_text("Type 'ze show version'\n")
            demo = {
                "id": "example",
                "source": "example/demo.tape",
                "page": "guide/example.md",
                "anchor": "first",
                "title": "First title",
            }

            with mock.patch.multiple(
                terminal_render,
                ROOT=root,
                DEMO_ROOT=demo_root,
                SHARED_SOURCE_PATHS=(shared,),
            ):
                baseline = terminal_render.source_digest(demo)

                page_only = dict(
                    demo,
                    page="guide/elsewhere.md",
                    anchor="second",
                    title="Second title",
                )
                self.assertEqual(baseline, terminal_render.source_digest(page_only))

                source.write_text("Type 'ze show health'\n")
                self.assertNotEqual(baseline, terminal_render.source_digest(demo))

                source.write_text("Type 'ze show version'\n")
                privileged = dict(demo, privileged=True)
                self.assertNotEqual(baseline, terminal_render.source_digest(privileged))

                realtime = dict(demo, realtime=True)
                self.assertNotEqual(baseline, terminal_render.source_digest(realtime))


class RenderAccelerationTest(unittest.TestCase):
    def test_shortens_capture_delays_without_changing_presentation_timing(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            demo_root = root / "demos" / "terminal"
            source = demo_root / "example" / "demo.tape"
            source.parent.mkdir(parents=True)
            source.write_text(
                "Output artifacts/example.webm\n"
                "Source common.tape\n"
                "Sleep 20s\n"
                'Type "ze show version"\n'
            )
            with mock.patch.multiple(
                terminal_render,
                ROOT=root,
                DEMO_ROOT=demo_root,
            ):
                generated = terminal_render.accelerated_terminal_tape(
                    {"id": "example", "source": "example/demo.tape"}
                )
                rendered = generated.read_text()
            self.assertIn("Set TypingSpeed 25ms\n", rendered)
            self.assertIn("Sleep 4000ms\n", rendered)
            self.assertNotIn("PlaybackSpeed", rendered)
            self.assertIn("Sleep 20s\n", source.read_text())

    def test_expands_capture_timestamps_and_dimensions(self):
        with tempfile.TemporaryDirectory() as tmp:
            capture = pathlib.Path(tmp) / "example.webm"
            capture.write_bytes(b"compressed")

            def write_expanded(command, check):
                self.assertTrue(check)
                self.assertEqual(command[command.index("-itsscale") + 1], "5")
                self.assertEqual(command[command.index("-c:v") + 1], "libvpx-vp9")
                self.assertIn("scale=1680:1008:flags=lanczos", command)
                pathlib.Path(command[-1]).write_bytes(b"expanded")

            with mock.patch.object(
                terminal_render.subprocess, "run", side_effect=write_expanded
            ):
                terminal_render.expand_timeline(capture, terminal_render.RENDER_SPEEDUP)
            self.assertEqual(capture.read_bytes(), b"expanded")
            self.assertFalse((pathlib.Path(tmp) / "example.fast.webm").exists())

    def test_realtime_capture_keeps_original_timestamps(self):
        with tempfile.TemporaryDirectory() as tmp:
            capture = pathlib.Path(tmp) / "example.webm"
            capture.write_bytes(b"compressed")

            def write_expanded(command, check):
                self.assertTrue(check)
                self.assertEqual(command[command.index("-itsscale") + 1], "1")
                self.assertEqual(command[command.index("-c:v") + 1], "libvpx-vp9")
                self.assertIn("scale=1680:1008:flags=lanczos", command)
                pathlib.Path(command[-1]).write_bytes(b"expanded")

            with mock.patch.object(
                terminal_render.subprocess, "run", side_effect=write_expanded
            ):
                terminal_render.expand_timeline(capture, 1)
            self.assertEqual(capture.read_bytes(), b"expanded")


class RenderTapeSelectionTest(unittest.TestCase):
    def _write_source(self, tmp):
        root = pathlib.Path(tmp)
        demo_root = root / "demos" / "terminal"
        source = demo_root / "example" / "demo.tape"
        source.parent.mkdir(parents=True)
        source.write_text(
            "Output artifacts/example.webm\n"
            "Source common.tape\n"
            "Sleep 20s\n"
            'Type "ze show version"\n'
        )
        return root, demo_root, source

    def test_normal_demo_uses_accelerated_tape(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, demo_root, source = self._write_source(tmp)
            demo = {
                "id": "example",
                "source": "example/demo.tape",
                "kind": "terminal",
            }
            with mock.patch.multiple(terminal_render, ROOT=root, DEMO_ROOT=demo_root):
                self.assertEqual(terminal_render.capture_speedup(demo), 5)
                tape = terminal_render.render_tape(demo)
                self.assertNotEqual(tape, source)
                rendered = tape.read_text()
            self.assertIn("Set TypingSpeed 25ms\n", rendered)
            self.assertIn("Sleep 4000ms\n", rendered)

    def test_realtime_demo_uses_original_tape_unchanged(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, demo_root, source = self._write_source(tmp)
            demo = {
                "id": "example",
                "source": "example/demo.tape",
                "kind": "terminal",
                "realtime": True,
            }
            with mock.patch.multiple(terminal_render, ROOT=root, DEMO_ROOT=demo_root):
                self.assertEqual(terminal_render.capture_speedup(demo), 1)
                tape = terminal_render.render_tape(demo)
                self.assertEqual(tape, source)
                rendered = tape.read_text()
            self.assertNotIn("Set TypingSpeed 25ms", rendered)
            self.assertIn("Sleep 20s\n", rendered)
            self.assertEqual(rendered, source.read_text())


class DemoLockTest(unittest.TestCase):
    """The demo state tree has one owner at a time.

    A demo run deletes tmp/terminal-demos/state/<demo-id> and runs `ze init`
    over it. Two runs at once delete and re-initialise each other's database,
    which reaches the operator as `database already exists`, as a missing
    `file/active/ze.conf`, or as an SSH authentication failure against a daemon
    that loaded the database its rival replaced.
    """

    WORKER_SECONDS = 0.5

    def lock_env(self, lock_dir, **extra):
        env = dict(
            os.environ,
            ZE_DEMO_LOCK_DIR=str(lock_dir),
            ZE_DEMO_LOCK_WAIT="60",
        )
        env.pop("ZE_DEMO_LOCK_HELD", None)
        env.update(extra)
        return env

    def lock_is_taken(self):
        """True when another process cannot take terminal_render.LOCK_PATH.

        Exit 1 is the conflict flock reports. Any other non-zero status is a
        usage or permission failure, and reading it as "locked" would let an
        unlocked write pass.
        """
        probe = subprocess.run(
            ["flock", "-n", str(terminal_render.LOCK_PATH), "true"],
            capture_output=True,
            timeout=30,
        )
        return probe.returncode == 1

    def wait_for(self, path):
        deadline = time.monotonic() + 30
        while not path.exists() and time.monotonic() < deadline:
            time.sleep(0.01)
        self.assertTrue(path.exists(), f"{path} never appeared")

    def hold_lock(self, lock_path, seconds):
        """Start a rival that owns `lock_path`, and return once it does.

        Returns the holder for the caller to kill. Waiting on the ready marker
        rather than on the lock file is what makes the wait mean "the lock is
        held": the file is created before flock(2) is called on it.

        A stale marker would defeat that, so it goes before the holder starts.
        The caller's `finally` cannot reach a holder this method never returned,
        so a failed wait kills it here: without that, a timeout leaves a live
        process sleeping on a lock it still owns.
        """
        ready = pathlib.Path(str(lock_path) + ".held")
        ready.unlink(missing_ok=True)
        holder = subprocess.Popen(
            [
                sys.executable,
                "-c",
                LOCK_HOLDER_SOURCE,
                str(lock_path),
                str(ready),
                str(seconds),
            ]
        )
        try:
            self.wait_for(ready)
        except BaseException:
            holder.kill()
            holder.wait(30)
            raise
        return holder

    def test_a_holder_that_never_readies_is_still_killed(self):
        """`hold_lock` owns the holder until it returns one.

        The caller's `finally` can only kill a holder it was given, so a wait
        that fails inside `hold_lock` would otherwise leave a live process
        sleeping on a lock it still owns. `flock -F` had this covered by
        accident: the old code bound the holder in the caller, so the caller's
        `finally` ran either way.
        """
        with tempfile.TemporaryDirectory() as tmp:
            lock_path = pathlib.Path(tmp) / "demo-run.lock"
            started = []
            # Bound before the patch: calling subprocess.Popen from inside the
            # side effect would re-enter the mock and recurse.
            real_popen = subprocess.Popen

            def capture(*args, **kwargs):
                started.append(real_popen(*args, **kwargs))
                return started[-1]

            with (
                mock.patch.object(subprocess, "Popen", side_effect=capture),
                mock.patch.object(
                    type(self), "wait_for", side_effect=AssertionError("no marker")
                ),
            ):
                with self.assertRaises(AssertionError):
                    self.hold_lock(lock_path, 120)

        self.assertEqual(len(started), 1)
        # poll() is None only while the process is alive.
        self.assertIsNotNone(started[0].poll(), "the holder was left running")

    def test_a_stale_marker_does_not_stand_in_for_a_held_lock(self):
        """The marker means "the lock is held", so a leftover one is a lie.

        A holder killed before its `finally` leaves the marker behind. The next
        run would then see it immediately, believe a rival owns the lock, and
        test nothing: exactly the vacuity the marker was added to remove, with
        the old wait-on-the-lock-file race back in a new costume.

        Popen and `wait_for` are both faked, so what the holder does cannot race
        the assertion: nothing but `hold_lock` itself can create the marker here.
        """
        with tempfile.TemporaryDirectory() as tmp:
            lock_path = pathlib.Path(tmp) / "demo-run.lock"
            ready = pathlib.Path(str(lock_path) + ".held")
            ready.touch()
            seen = []

            def record(path):
                seen.append(path.exists())
                raise AssertionError("stop before the holder can ready itself")

            with (
                # spec=, so a future hold_lock that consulted poll() or a
                # return value cannot pass here on an auto-created attribute.
                mock.patch.object(
                    subprocess, "Popen", return_value=mock.Mock(spec=subprocess.Popen)
                ),
                mock.patch.object(type(self), "wait_for", side_effect=record),
            ):
                with self.assertRaises(AssertionError):
                    self.hold_lock(lock_path, 1)

        # False: the stale marker was gone before anything could wait on it.
        self.assertEqual(seen, [False])

    @unittest.skipIf(shutil.which("flock") is None, "flock(1) is not installed")
    def test_a_second_run_waits_for_the_first(self):
        with tempfile.TemporaryDirectory() as tmp:
            lock_dir = pathlib.Path(tmp)
            trace = lock_dir / "trace"
            marker = lock_dir / "first-is-running"
            worker = lock_dir / "worker.sh"
            # The marker is a start barrier. The second run starts only once
            # the first is inside the lock, so the test cannot pass because the
            # two runs never overlapped.
            worker.write_text(
                "#!/usr/bin/env bash\n"
                'printf "%s start\\n" "$1" >>"$2"\n'
                'touch "$3"\n'
                f"sleep {self.WORKER_SECONDS}\n"
                'printf "%s end\\n" "$1" >>"$2"\n'
            )
            worker.chmod(0o755)
            env = self.lock_env(lock_dir)
            lock = str(HERE / "demo-lock.sh")

            started = time.monotonic()
            first = subprocess.Popen(
                [lock, str(worker), "a", str(trace), str(marker)], env=env
            )
            self.wait_for(marker)
            second = subprocess.Popen(
                [lock, str(worker), "b", str(trace), str(marker)], env=env
            )
            self.assertEqual(first.wait(120), 0)
            self.assertEqual(second.wait(120), 0)
            elapsed = time.monotonic() - started

            order = trace.read_text().splitlines()

        self.assertEqual(len(order), 4, order)
        for start, end in (order[0:2], order[2:4]):
            # One run holds the lock from its start line to its end line, so
            # the other run's lines cannot fall between them.
            self.assertEqual(start.split()[0], end.split()[0], order)
            self.assertEqual(start.split()[1], "start", order)
            self.assertEqual(end.split()[1], "end", order)
        # Serialised, the two runs cost two workers. Overlapped, they cost one.
        self.assertGreater(elapsed, 2 * self.WORKER_SECONDS)

    @unittest.skipIf(shutil.which("flock") is None, "flock(1) is not installed")
    def test_a_run_told_the_lock_is_held_does_not_take_it_again(self):
        """render.py holds the lock while it starts the container.

        The container would otherwise wait for a lock its own harness holds,
        for the whole of ZE_DEMO_LOCK_WAIT, and every render would deadlock.
        """
        with tempfile.TemporaryDirectory() as tmp:
            lock_dir = pathlib.Path(tmp)
            # The holder outlives the timeout below, so a run that takes the
            # lock again waits past it and the timeout fails the test. A run
            # that honours the flag returns at once.
            holder = self.hold_lock(lock_dir / "demo-run.lock", 120)
            try:
                result = subprocess.run(
                    [str(HERE / "demo-lock.sh"), "true"],
                    capture_output=True,
                    text=True,
                    timeout=10,
                    env=self.lock_env(lock_dir, ZE_DEMO_LOCK_HELD="1"),
                )
            finally:
                holder.kill()
                holder.wait(30)

        self.assertEqual(result.returncode, 0, result.stderr)

    def test_the_container_entrypoint_takes_the_lock_before_it_writes(self):
        """The lock precedes the setup, not only the demo command.

        HOME is on the mounted repository and every demo container shares it.
        The setup truncates `.bashrc`, which carries the prompt and the PATH of
        the shell vhs drives, so a lock taken after it leaves that window open.
        """
        lines = (HERE / "container-entrypoint.sh").read_text().splitlines()
        source = [
            i
            for i, line in enumerate(lines)
            if line.strip() == "source /src/demos/terminal/demo-lock.sh"
        ]
        acquire = [i for i, line in enumerate(lines) if "demo_lock_acquire" in line]
        writes = [
            i
            for i, line in enumerate(lines)
            if not line.strip().startswith("#")
            and ('>"${HOME}' in line or line.strip().startswith(("cp ", "mkdir ")))
        ]

        self.assertTrue(source, "the entrypoint never sources the lock")
        self.assertTrue(acquire, "the entrypoint never takes the lock")
        self.assertTrue(writes, "the entrypoint writes no shared state")
        self.assertLess(max(source), min(acquire))
        self.assertLess(max(acquire), min(writes))

    def test_the_container_and_the_harness_lock_one_file(self):
        """Two holders that name different files exclude nothing.

        The container reaches the repository at /src, so the shell default and
        render.py's path are one inode seen through the bind mount.
        """
        shell = (HERE / "demo-lock.sh").read_text().splitlines()
        default = [line for line in shell if line.strip().startswith("lock_dir=")]
        named = [line for line in shell if line.strip().startswith("lock_file=")]

        self.assertEqual(len(default), 1, default)
        self.assertEqual(len(named), 1, named)
        self.assertIn("/src/tmp/terminal-demos", default[0])
        self.assertIn(terminal_render.LOCK_PATH.name, named[0])
        self.assertEqual(
            terminal_render.LOCK_PATH,
            terminal_render.ROOT / "tmp" / "terminal-demos" / "demo-run.lock",
        )

    def test_the_harness_tells_the_container_the_lock_is_held(self):
        command = terminal_render.container_command(
            {"image": "ze-terminal-demo-render-all:test", "platform": "linux/native"},
            pathlib.PurePosixPath("/src/demos/terminal/example/validate.sh"),
            False,
        )

        self.assertIn("ZE_DEMO_LOCK_HELD=1", command)

    def test_the_harness_lock_refuses_a_tree_another_run_holds(self):
        with tempfile.TemporaryDirectory() as tmp:
            lock_path = pathlib.Path(tmp) / "demo-run.lock"
            holder = self.hold_lock(lock_path, 30)
            try:
                with mock.patch.multiple(
                    terminal_render, LOCK_PATH=lock_path, LOCK_WAIT_SECONDS=1
                ):
                    with self.assertRaises(RuntimeError):
                        with terminal_render.demo_lock():
                            self.fail("the lock was taken while another run held it")
            finally:
                holder.kill()
                holder.wait(30)

    @unittest.skipIf(
        hasattr(os, "geteuid") and os.geteuid() == 0,
        "root ignores the permission bits this test relies on",
    )
    def test_the_harness_locks_a_file_it_cannot_write(self):
        """The container runs as root and owns the lock file it creates.

        The harness runs as the user who owns the repository, so a lock opened
        for writing fails with EACCES on the second run of the day.
        """
        with tempfile.TemporaryDirectory() as tmp:
            lock_path = pathlib.Path(tmp) / "demo-run.lock"
            lock_path.touch()
            lock_path.chmod(0o444)

            with mock.patch.multiple(
                terminal_render, LOCK_PATH=lock_path, LOCK_WAIT_SECONDS=1
            ):
                with terminal_render.demo_lock():
                    pass

    @unittest.skipIf(shutil.which("flock") is None, "flock(1) is not installed")
    def test_the_artifact_manifest_is_published_under_the_lock(self):
        """The manifest is a read-modify-write, not only a write.

        Two runs that each load it before either writes both publish their own
        view, and the second write drops the first run's entries.
        """
        seen = {}

        def render_one(manifest, demo, release):  # noqa: ARG001
            # run_demo takes the lock again, under the one render_selected
            # holds. A lock that is not re-entrant waits for itself here.
            with terminal_render.demo_lock():
                return {"assets": {}}

        def record(generated):
            seen["locked"] = self.lock_is_taken()

        with tempfile.TemporaryDirectory() as tmp:
            with mock.patch.multiple(
                terminal_render,
                LOCK_PATH=pathlib.Path(tmp) / "demo-run.lock",
                LOCK_WAIT_SECONDS=5,
                run_demo=render_one,
                load_artifact_manifest=mock.Mock(
                    return_value={"schema": 2, "renderer": {}, "demos": {}}
                ),
                write_artifact_manifest=record,
                verify_assets=mock.Mock(),
            ):
                terminal_render.render_selected(
                    {"renderer": {}}, {"example": {"id": "example"}}, ["example"], "1"
                )

        self.assertTrue(seen.get("locked"), "the manifest was written unlocked")

    @unittest.skipIf(shutil.which("flock") is None, "flock(1) is not installed")
    def test_a_validation_runs_its_container_under_the_lock(self):
        seen = {}

        def record(command, **kwargs):  # noqa: ARG001
            seen["locked"] = self.lock_is_taken()
            return subprocess.CompletedProcess(command, 0)

        with tempfile.TemporaryDirectory() as tmp:
            with mock.patch.multiple(
                terminal_render,
                LOCK_PATH=pathlib.Path(tmp) / "demo-run.lock",
                LOCK_WAIT_SECONDS=5,
                subprocess=mock.Mock(run=record),
            ):
                terminal_render.run_validation(
                    {"renderer": {"image": "i", "platform": "linux/native"}},
                    {"id": "example", "validate": "example/validate.sh"},
                )

        self.assertTrue(seen.get("locked"), "the validator ran unlocked")

    @unittest.skipIf(shutil.which("flock") is None, "flock(1) is not installed")
    def test_the_check_path_reads_the_manifest_under_the_lock(self):
        seen = {}

        def record(*args, **kwargs):  # noqa: ARG001
            seen["locked"] = self.lock_is_taken()

        with tempfile.TemporaryDirectory() as tmp:
            argv = ["render.py", "--demo", "example", "--check"]
            with mock.patch.object(sys, "argv", argv):
                with mock.patch.multiple(
                    terminal_render,
                    LOCK_PATH=pathlib.Path(tmp) / "demo-run.lock",
                    LOCK_WAIT_SECONDS=5,
                    load_json=mock.Mock(return_value={"renderer": {}}),
                    validate_contract=mock.Mock(return_value={"example": {}}),
                    verify_assets=record,
                ):
                    self.assertEqual(terminal_render.main(), 0)

        self.assertTrue(seen.get("locked"), "the check read the manifest unlocked")

    def test_the_lock_hands_the_demo_the_environment_it_was_given(self):
        """The demo shell's prompt is an exported PS1.

        A non-interactive bash drops PS1 from the environment it passes on, so
        a lock that runs the demo in a child shell leaves vhs to paint its own
        `> ` prompt, and every `Wait+Screen /\\$ /` in a tape times out on it.
        """
        with tempfile.TemporaryDirectory() as tmp:
            caller = pathlib.Path(tmp) / "caller.sh"
            caller.write_text(
                "#!/usr/bin/env bash\n"
                "set -euo pipefail\n"
                "export PS1='ZE_PROMPT_MARKER '\n"
                f"source {HERE / 'demo-lock.sh'}\n"
                "demo_lock_run env\n"
            )
            caller.chmod(0o755)

            result = subprocess.run(
                [str(caller)],
                capture_output=True,
                text=True,
                timeout=60,
                env=self.lock_env(tmp),
            )

        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("PS1=ZE_PROMPT_MARKER", result.stdout)


class ValidatorDiagnosticTest(unittest.TestCase):
    def test_a_failing_validator_prints_the_logs_the_run_captured(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            state = root / "state" / "example"
            state.mkdir(parents=True)
            (state / "daemon.log").write_text("bridge stp: permission denied\n")
            demo = root / "example"
            demo.mkdir()
            validator = demo / "validate.sh"
            validator.write_text(
                "#!/usr/bin/env bash\n"
                "set -euo pipefail\n"
                f"source {HERE / 'validate-common.sh'}\n"
                # Inside a function, which an ERR trap does not reach without
                # errtrace. A validator's helpers are where a failure lands.
                "fail_in_a_function() { false; }\n"
                "fail_in_a_function\n"
            )
            validator.chmod(0o755)

            result = subprocess.run(
                [str(validator)],
                capture_output=True,
                text=True,
                timeout=60,
                env=dict(os.environ, ZE_DEMO_STATE_ROOT=str(root / "state")),
            )

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("bridge stp: permission denied", result.stderr)


if __name__ == "__main__":
    unittest.main()
