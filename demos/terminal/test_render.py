#!/usr/bin/env python3

import contextlib
import errno
import importlib.util
import json
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

# The site tool that puts a demo on a page. It is the other end of the artifact
# manifest render.py writes, so the asset set the renderer produces and the
# markup the page carries are one contract tested from both sides. It reads
# `sitepaths` as a sibling, so its own directory goes on the path first.
sys.path.insert(0, str(HERE.parent.parent / "website" / "tools"))
import terminal_demos  # noqa: E402  needs the path above

# The recorder. Its file name carries a hyphen, so it is loaded the way
# render.py is above rather than imported: the driver is a program the
# container runs, and the tests below drive it as one. The parser is reached
# directly because a tape that fails to parse must fail before any terminal
# exists, and that is a property of the function, not of a session.
PTY_SPEC = importlib.util.spec_from_file_location(
    "pty_session", HERE / "pty-session.py"
)
pty_session = importlib.util.module_from_spec(PTY_SPEC)
PTY_SPEC.loader.exec_module(pty_session)

# The screen model both of them read. It is an ordinary module beside them, and
# the two loads above put its directory on the import path.
import screen as terminal_screen  # noqa: E402  needs the loads above

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
        the shell the recorder drives, so a lock taken after it leaves that
        window open.
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

    def test_container_mounts_the_resolved_absolute_scratch_target(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = pathlib.Path(tmp).resolve()
            root = base / "checkout"
            scratch_root = root / "tmp" / "terminal-demos"
            cache_parent = base / "cache"
            cache_target = cache_parent / "terminal-demos"
            artifact_root = base / "artifacts"
            scratch_root.parent.mkdir(parents=True)
            cache_target.mkdir(parents=True)
            scratch_root.symlink_to(cache_target, target_is_directory=True)

            with mock.patch.multiple(
                terminal_render, ROOT=root, ARTIFACT_ROOT=artifact_root
            ):
                command = terminal_render.container_command(
                    {
                        "image": "ze-terminal-demo-render-all:test",
                        "platform": "linux/native",
                    },
                    pathlib.PurePosixPath(
                        "/src/demos/terminal/example/validate.sh"
                    ),
                    False,
                )

            volumes = [
                command[index + 1]
                for index, argument in enumerate(command)
                if argument == "--volume"
            ]
            resolved_target = cache_target.resolve()
            self.assertEqual(
                volumes,
                [
                    f"{root}:/src",
                    f"{resolved_target}:{resolved_target}",
                    f"{artifact_root}:/src/demos/terminal/artifacts",
                ],
            )
            self.assertNotIn(f"{cache_parent}:{cache_parent}", volumes)

    def test_container_adds_no_scratch_mount_for_a_real_directory(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = pathlib.Path(tmp).resolve()
            root = base / "checkout"
            scratch_root = root / "tmp" / "terminal-demos"
            cache_parent = base / "cache"
            artifact_root = base / "artifacts"
            scratch_root.mkdir(parents=True)
            cache_parent.mkdir()

            with mock.patch.multiple(
                terminal_render, ROOT=root, ARTIFACT_ROOT=artifact_root
            ):
                command = terminal_render.container_command(
                    {
                        "image": "ze-terminal-demo-render-all:test",
                        "platform": "linux/native",
                    },
                    pathlib.PurePosixPath(
                        "/src/demos/terminal/example/validate.sh"
                    ),
                    False,
                )

            volumes = [
                command[index + 1]
                for index, argument in enumerate(command)
                if argument == "--volume"
            ]
            self.assertEqual(
                volumes,
                [
                    f"{root}:/src",
                    f"{artifact_root}:/src/demos/terminal/artifacts",
                ],
            )
            self.assertNotIn(f"{cache_parent}:{cache_parent}", volumes)

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
                # The publish pass removes what the kind no longer produces, so
                # it reads the artifact root. Pointed inside the temporary tree
                # here: nothing about locking is worth touching a real one.
                ARTIFACT_ROOT=pathlib.Path(tmp) / "demos",
                run_demo=render_one,
                load_artifact_manifest=mock.Mock(
                    return_value={"schema": 2, "renderer": {}, "demos": {}}
                ),
                write_artifact_manifest=record,
                verify_assets=mock.Mock(),
            ):
                terminal_render.render_selected(
                    {"renderer": {}},
                    # `kind` is required of every demo (`validate_contract`), so
                    # a fixture without one describes a demo that cannot exist.
                    {"example": {"id": "example", "kind": "terminal"}},
                    ["example"],
                    "1",
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
        a lock that runs the demo in a child shell leaves the demo shell to paint
        its own default prompt, and every `Wait+Screen /\\$ /` in a tape times
        out on it.
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


class HealthReportsDemoSourceContractTest(unittest.TestCase):
    """Synchronization contracts for the checked-in health reports tape."""

    # VALIDATES: `show health` completes on its final aggregate status row.
    # PREVENTS: waiting for the editor title after the health table scrolls it away.
    def test_show_health_waits_for_aggregate_status_not_the_editor_title(self):
        source = (HERE / "health-reports" / "demo.tape").read_text(encoding="utf-8")
        health_flow = source[
            source.index("sshpass -e ssh ze-demo 'show health'")
            : source.index("sshpass -e ssh ze-demo 'show warnings source bgp'")
        ]

        self.assertIn("Wait+Screen /ipsec/", health_flow)
        self.assertIn(r"Wait+Screen /status\s+down/", health_flow)
        self.assertLess(
            health_flow.index("Wait+Screen /ipsec/"),
            health_flow.index(r"Wait+Screen /status\s+down/"),
        )
        self.assertNotIn("Wait+Screen /Ze Editor/", health_flow)

    # VALIDATES: the teardown completes on its final structured result row.
    # PREVENTS: waiting for the editor title after the result scrolls it away.
    def test_peer_teardown_waits_for_final_subcode_not_the_editor_title(self):
        source = (HERE / "health-reports" / "demo.tape").read_text(encoding="utf-8")
        teardown_flow = source[
            source.index("sshpass -e ssh ze-demo 'request peer 127.0.0.2 teardown 4'")
            : source.index("sshpass -e ssh ze-demo 'show errors source bgp'")
        ]

        peer_wait = r"Wait+Screen /peer\s+127\.0\.0\.2/"
        subcode_wait = r"Wait+Screen /subcode\s+4/"
        self.assertIn(peer_wait, teardown_flow)
        self.assertIn(subcode_wait, teardown_flow)
        self.assertLess(
            teardown_flow.index(peer_wait),
            teardown_flow.index(subcode_wait),
        )
        self.assertNotIn("Wait+Screen /Ze Editor/", teardown_flow)

    # VALIDATES: an interactive SSH login exposes the banner, then every
    # transcript operation is a visible shell-level SSH exec.
    # PREVENTS: asking SSH exec for a login banner or claiming hidden TUI input.
    def test_banner_login_precedes_visible_ssh_exec_commands(self):
        source = (HERE / "health-reports" / "demo.tape").read_text(encoding="utf-8")
        transcript = (HERE / "health-reports" / "transcript.txt").read_text(
            encoding="utf-8"
        )
        interactive = "sshpass -e ssh ze-demo"
        invocations = (
            "sshpass -e ssh ze-demo 'show health'",
            "sshpass -e ssh ze-demo 'show warnings source bgp'",
            "sshpass -e ssh ze-demo 'request peer 127.0.0.2 teardown 4'",
            "sshpass -e ssh ze-demo 'show errors source bgp'",
        )
        login_flow = source[
            source.index(f'Type "{interactive}"')
            : source.index(f'Type "{invocations[0]}"')
        ]

        banner_wait = "Wait+Screen /stale prefix data/"
        editor_exit = 'Type "exit"\nEnter\n'
        launcher_wait = "Wait+Screen /ze>/"
        shell_wait = r"Wait+Screen /\$ /"
        first_exit = login_flow.index(editor_exit)
        second_exit = login_flow.index(editor_exit, first_exit + len(editor_exit))
        positions = (
            login_flow.index(banner_wait),
            first_exit,
            login_flow.index(launcher_wait),
            second_exit,
            login_flow.index(shell_wait),
        )
        self.assertEqual(positions, tuple(sorted(positions)))

        for invocation in invocations:
            with self.subTest(invocation=invocation):
                self.assertIn(f'Type "{invocation}"\nEnter\n', source)

        transcript_commands = [
            line.removeprefix("$ ")
            for line in transcript.splitlines()
            if line.startswith("$ ")
        ]
        self.assertEqual(transcript_commands, [interactive, *invocations])
        banner = "Warning: stale-prefix-data has stale prefix data (updated 2024-01-01)"
        self.assertLess(transcript.index(f"$ {interactive}"), transcript.index(banner))
        self.assertLess(transcript.index(banner), transcript.index(f"$ {invocations[0]}"))
        self.assertNotIn('Type "run ', source)
        self.assertNotIn("Escape\n", source)
        self.assertNotIn("ze# run ", transcript)


class BrowserDemoSourceContractTest(unittest.TestCase):
    """Synchronization contracts for the checked-in browser demo driver."""

    # VALIDATES: diff review waits for the exact HTMX response before the modal.
    # PREVENTS: aggregate rendering turning server latency into a selector timeout.
    def test_diff_review_waits_on_the_response_and_then_the_modal(self):
        source = (HERE / "web-config" / "demo.cjs").read_text(encoding="utf-8")
        diff_flow = source[
            source.index("const [diffResponse]")
            : source.index("await pause(6000)")
        ]

        promise_flow = diff_flow[: diff_flow.index("    ]);")]
        self.assertIn("const [diffResponse] = await Promise.all([", promise_flow)
        self.assertIn("page.waitForResponse((response) =>", promise_flow)
        self.assertIn('url.pathname === "/config/diff"', promise_flow)
        self.assertIn('url.search === ""', promise_flow)
        self.assertIn('page.click("#commit-review-btn")', promise_flow)
        self.assertLess(
            promise_flow.index("page.waitForResponse"),
            promise_flow.index('page.click("#commit-review-btn")'),
        )
        self.assertIn('response.request().method() === "GET"', promise_flow)
        self.assertIn("if (!diffResponse.ok())", diff_flow)
        self.assertIn("diffResponse.status()", diff_flow)
        self.assertIn("await diffResponse.text()", diff_flow)
        self.assertIn("diffResponse.url()", diff_flow)
        self.assertIn(
            'await page.waitForSelector("#diff-modal.open .diff-content");',
            diff_flow,
        )
        self.assertNotIn("timeout: 10000", diff_flow)


class TerminalDemoAssetTest(unittest.TestCase):
    """The four Wiring Test rows of spec-website-asciinema-terminal-demos.

    Each row runs an entry point through to the code that must answer it. The
    per-kind asset map is the seam they meet at: `_render_demo` asks it what a
    demo owes, `verify_assets` asks it what to check, and the site asks it what
    to put on the page. A demo cannot be rendered as one shape and published as
    another while all three read one map.
    """

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        root = pathlib.Path(self.tmp.name)
        self.demo_root = root / "demos" / "terminal"
        (self.demo_root / "example").mkdir(parents=True)
        (self.demo_root / "common.tape").write_text(
            'Set Shell "bash"\n', encoding="utf-8"
        )
        (self.demo_root / "example" / "demo.tape").write_text(
            "Output artifacts/example.webm\nSource common.tape\n", encoding="utf-8"
        )
        self.artifact_root = root / "gh-pages" / "assets" / "demos"
        self.artifact_root.mkdir(parents=True)
        patch = mock.patch.multiple(
            terminal_render,
            ROOT=root,
            DEMO_ROOT=self.demo_root,
            ARTIFACT_ROOT=self.artifact_root,
            ARTIFACT_MANIFEST_PATH=self.artifact_root / "manifest.json",
        )
        patch.start()
        self.addCleanup(patch.stop)
        self.renderer = {
            "name": "ze-demo",
            "version": "2",
            "image": "ze-terminal-demo-render-all:test",
            "platform": "linux/native",
        }

    def demo(self, kind):
        return {"id": "example", "kind": kind, "source": "example/demo.tape"}

    def suffix(self, name):
        """The file extension for an asset name, whichever kind owns it.

        A fixture must be able to name an asset the demo's kind does NOT own,
        because that half-converted state is what R-5 exists to refuse. So this
        reads the union of the two kinds rather than one kind's map, and it
        stays derived from the map instead of repeating it.
        """
        for extensions in terminal_render.ASSET_EXTENSIONS.values():
            if name in extensions:
                return extensions[name]
        raise KeyError(name)

    def publish(self, names, demo_id="example"):
        """Write each named asset to the artifact root and describe it."""
        assets = {}
        for name in names:
            path = self.artifact_root / (demo_id + self.suffix(name))
            path.write_text(f"{name} bytes\n", encoding="utf-8")
            assets[name] = {
                "path": path.name,
                "bytes": path.stat().st_size,
                "sha256": terminal_render.sha256(path),
            }
        return assets

    def check(self, demo, assets):
        """Run the demo's assets through `--check-definition`.

        Every digest here matches, so nothing the check reports is about a
        stale or missing file. What is left for it to judge is the asset SET.
        """
        terminal_render.ARTIFACT_MANIFEST_PATH.write_text(
            json.dumps(
                {
                    "schema": 2,
                    "renderer": self.renderer,
                    "demos": {
                        "example": {
                            "release": "test",
                            "definition-sha256": terminal_render.definition_digest(
                                demo
                            ),
                            "assets": assets,
                        }
                    },
                }
            )
            + "\n",
            encoding="utf-8",
        )
        terminal_render.verify_assets(
            {"renderer": self.renderer},
            {"example": demo},
            ["example"],
            None,
            definition_only=True,
        )

    def test_render_terminal_demo_produces_cast(self):
        """AC-1: a terminal demo's artifact is one cast, and the driver writes it.

        Two halves, because the row runs from `render.py` through to the
        recorder. The map says what the render owes. The tape front end is what
        must produce it, and `container-entrypoint.sh` is what hands it the
        tape. The tape below is self-contained so the assertion is about the
        recorder and not about the demo daemon.
        """
        expected = terminal_render.asset_paths(self.demo("terminal"))
        self.assertEqual(sorted(expected), ["cast", "transcript"])
        self.assertTrue(expected["cast"].name.endswith(".cast"), expected["cast"])

        work = pathlib.Path(self.tmp.name) / "record"
        (work / "artifacts").mkdir(parents=True)
        tape = work / "demo.tape"
        # The dimensions are PIXELS, as they are in every checked-in tape:
        # `common.tape` asks for 1680x1008 and `render.py` renders that box.
        # This fixture read 80x24 when it was written, which is a character
        # grid, and 80 pixels of width hold no terminal at all.
        tape.write_text(
            "Output artifacts/example.cast\n"
            'Set Shell "bash"\n'
            "Set Width 1680\n"
            "Set Height 1008\n"
            "Set TypingSpeed 5ms\n"
            'Type "echo hello"\n'
            "Enter\n"
            "Sleep 200ms\n",
            encoding="utf-8",
        )

        result = subprocess.run(
            [sys.executable, str(HERE / "pty-session.py"), "--tape", str(tape)],
            cwd=work,
            capture_output=True,
            text=True,
            timeout=120,
        )

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertTrue(
            (work / "artifacts" / "example.cast").is_file(),
            "the driver wrote no cast: " + result.stdout + result.stderr,
        )

    def test_verify_assets_demands_cast_for_terminal_kind(self):
        """AC-6: a terminal demo whose manifest names a video fails the check.

        Every asset below is on disk with a matching size and digest, so
        nothing is missing and nothing is stale. What is wrong is the SET: a
        terminal demo owes a cast and a transcript, and a video beside them
        means a half-converted tree is about to publish the demo twice (R-5).
        """
        demo = self.demo("terminal")
        with self.assertRaises(ValueError) as raised:
            self.check(demo, self.publish(["cast", "transcript", "video", "poster"]))

        message = str(raised.exception)
        self.assertIn("example", message)
        self.assertIn("video", message)
        self.assertIn("terminal", message)

    def test_browser_demo_still_produces_video_and_poster(self):
        """AC-7: the browser demo is untouched by the conversion.

        It keeps its three assets, and the check still accepts exactly those.
        It also refuses a cast beside them: a browser recording has no terminal
        byte stream, so a cast on that demo is the same half-converted tree the
        terminal row refuses from the other side (R-5).
        """
        demo = self.demo("browser")
        expected = terminal_render.asset_paths(demo)
        self.assertEqual(sorted(expected), ["poster", "transcript", "video"])
        self.assertTrue(expected["video"].name.endswith(".webm"), expected["video"])
        self.assertTrue(expected["poster"].name.endswith(".png"), expected["poster"])

        self.check(demo, self.publish(["video", "poster", "transcript"]))

        with self.assertRaises(ValueError) as raised:
            self.check(demo, self.publish(["video", "poster", "transcript", "cast"]))
        self.assertIn("cast", str(raised.exception))

    def test_a_video_render_asks_for_ffmpeg_before_the_container_runs(self):
        """The host binary the VHS installer used to bring is named, not tripped over.

        `expand_timeline` and `resize_poster` run ffmpeg on the HOST, and it
        arrived with `demos/terminal/install-vhs.sh` until that script was
        deleted with VHS. Reached at the ffmpeg call it is a bare
        FileNotFoundError, after the container has already run the whole demo.
        """
        (self.demo_root / "example" / "transcript.txt").write_text(
            "$ ze show version\n", encoding="utf-8"
        )

        def refuse(*args, **kwargs):
            raise AssertionError("the container must not start")

        with mock.patch.object(terminal_render.shutil, "which", return_value=None):
            with mock.patch.object(terminal_render.subprocess, "run", refuse):
                with self.assertRaises(RuntimeError) as raised:
                    terminal_render._render_demo(
                        {"renderer": self.renderer}, self.demo("browser"), "test"
                    )

        message = str(raised.exception)
        self.assertIn("ffmpeg", message)
        self.assertIn("example", message)

    def test_a_render_removes_the_artifacts_its_kind_no_longer_produces(self):
        """AC-12: the published tree keeps only what the kinds produce.

        The tree this conversion lands on holds a `.webm` and a `.png` for every
        terminal demo, and the manifest entry a re-render writes stops naming
        them. Nothing else walks the artifact root: `verify_assets` reads the
        set the manifest NAMES, and the site stages the directory as it stands.
        So a file left behind here is published for as long as the tree lives.

        The browser demo is rendered in the same run, because a removal that
        reached ITS video would be the same defect from the other side.
        """
        (self.demo_root / "browser").mkdir()
        (self.demo_root / "browser" / "demo.tape").write_text(
            "Output artifacts/browser.webm\nSource common.tape\n", encoding="utf-8"
        )
        terminal = self.demo("terminal")
        browser = dict(self.demo("browser"), id="browser", source="browser/demo.tape")

        # The pre-conversion tree: the terminal demo still carries the video and
        # the poster the previous renderer recorded for it.
        self.publish(["video", "poster"])
        stale_video = self.artifact_root / "example.webm"
        stale_poster = self.artifact_root / "example.png"
        self.assertTrue(stale_video.is_file() and stale_poster.is_file())

        def fake_run_demo(manifest, demo, release):
            assets = self.publish(
                terminal_render.ASSET_EXTENSIONS[demo["kind"]], demo["id"]
            )
            return {
                "release": release,
                "source-sha256": terminal_render.source_digest(demo),
                "definition-sha256": terminal_render.definition_digest(demo),
                "assets": assets,
            }

        # The shared sources are real repository files, and this fixture's ROOT
        # is a temporary tree, so `source_digest` is pointed at a file inside it
        # instead. Both the fake render and `verify_assets` read the same one.
        with mock.patch.multiple(
            terminal_render,
            run_demo=fake_run_demo,
            demo_lock=contextlib.nullcontext,
            SHARED_SOURCE_PATHS=(self.demo_root / "common.tape",),
        ):
            terminal_render.render_selected(
                {"renderer": self.renderer},
                {"example": terminal, "browser": browser},
                ["example", "browser"],
                "test",
            )

        self.assertFalse(stale_video.exists(), "the superseded video is published")
        self.assertFalse(stale_poster.exists(), "the superseded poster is published")
        self.assertTrue((self.artifact_root / "example.cast").is_file())
        self.assertTrue((self.artifact_root / "example.txt").is_file())
        # A browser demo records no cast, so its own video is not superseded.
        self.assertTrue((self.artifact_root / "browser.webm").is_file())
        self.assertTrue((self.artifact_root / "browser.png").is_file())

    def test_render_demo_page_embeds_player(self):
        """AC-8: a published page plays a terminal demo through the player.

        `_render_html` is the only producer of a demo's markup on a page. The
        artifact below names every asset both kinds use, so the video path is
        available to it and the markup it emits is a choice rather than a
        fallback.
        """
        artifact = {
            "release": "test",
            "assets": {
                name: {
                    "path": "example" + self.suffix(name),
                    "bytes": 1,
                    "sha256": "0" * 64,
                }
                for name in ("cast", "video", "poster", "transcript")
            },
        }
        demo = {
            "title": "Example demo",
            "description": "Example description.",
            "platform": "portable",
            "kind": "terminal",
            "engine": "Ze recorder",
        }

        # The duration and the grid are the cast's, which is why the manifest
        # entry above states neither: the caller reads them off the artifact
        # (`terminal_demos._cast_facts`) and hands them in.
        markup = terminal_demos._render_html(
            "example",
            demo,
            artifact,
            self.renderer,
            "../",
            "40 seconds",
            terminal_demos.CastFacts(137, 36, 40.0),
            "transcript text\n",
        )

        self.assertIn(".cast", markup)
        self.assertNotIn("<video", markup)
        self.assertNotIn("video/webm", markup)


class TapeRecorderTest(unittest.TestCase):
    """The tape front end of spec-website-asciinema-terminal-demos.

    Every case here drives `pty-session.py` the way the container does, over a
    tape written for the case. A real shell answers a real pseudo-terminal, so
    what the cast holds is what a reader would replay, not what the recorder
    believed it wrote.
    """

    # Every setting `common.tape` carries that reaches the recording, at the
    # values it carries them. A case that needs a different one overrides it
    # after this block, because a later `Set` wins.
    PROLOGUE = (
        "Output artifacts/example.cast\n"
        'Set Shell "bash"\n'
        "Set Width 1680\n"
        "Set Height 1008\n"
        "Set FontSize 20\n"
        "Set Padding 17\n"
        "Set TypingSpeed 5ms\n"
        "Set WaitTimeout 20s\n"
    )

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.work = pathlib.Path(self.tmp.name)
        (self.work / "artifacts").mkdir()
        # HOME is the temporary tree, so the shell the recorder drives reads no
        # startup file of the person running the tests and prints nothing this
        # test did not ask for.
        self.env = dict(
            os.environ,
            HOME=str(self.work),
            PS1="$ ",
            TERM="xterm-256color",
            LANG="C.UTF-8",
            LC_ALL="C.UTF-8",
        )

    def write_tape(self, body, name="demo.tape"):
        path = self.work / name
        path.write_text(body, encoding="utf-8")
        return path

    def parse(self, body):
        return pty_session.parse_tape(self.write_tape(body), root=self.work)

    def record(self, body, timeout=120):
        """Drive a tape and return its cast as (header, events)."""
        tape = self.write_tape(body)
        result = subprocess.run(
            [sys.executable, str(HERE / "pty-session.py"), "--tape", str(tape)],
            cwd=self.work,
            capture_output=True,
            text=True,
            timeout=timeout,
            env=self.env,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        cast = self.work / "artifacts" / "example.cast"
        self.assertTrue(cast.is_file(), result.stdout + result.stderr)
        lines = cast.read_text(encoding="utf-8").splitlines()
        return json.loads(lines[0]), [json.loads(line) for line in lines[1:]]

    def test_tape_vocabulary_is_covered(self):
        """AC-13: the closed vocabulary parses, and anything else fails by name.

        The census is taken from the checked-in tapes rather than from a list
        written here, so a directive that enters the tree without entering the
        recorder fails this test instead of being skipped at render time. The
        counts are C-14's, and they are asserted so the walk cannot pass by
        finding nothing.
        """
        directives = set()
        keys = set()
        tapes = sorted(HERE.glob("*/demo.tape"))
        self.assertEqual(len(tapes), 17, tapes)
        for tape in tapes + [HERE / "common.tape"]:
            pty_session.parse_tape(tape, root=HERE)
            for line in tape.read_text(encoding="utf-8").splitlines():
                word = line.split(" ")[0].strip()
                if not word or word.startswith("#"):
                    continue
                directives.add(word.split("+")[0])
                if word == "Set":
                    keys.add(line.split()[1])
        self.assertEqual(directives - set(pty_session.TAPE_DIRECTIVES), set())
        self.assertEqual(keys - set(pty_session.TAPE_SET_KEYS), set())
        self.assertEqual(len(directives), 13, sorted(directives))
        self.assertEqual(len(keys), 10, sorted(keys))

        with self.assertRaises(pty_session.TapeError) as raised:
            self.parse(self.PROLOGUE + "Frobnicate now\n")
        self.assertIn("Frobnicate", str(raised.exception))

        with self.assertRaises(pty_session.TapeError) as raised:
            self.parse(self.PROLOGUE + "Set Blink 3s\n")
        self.assertIn("Blink", str(raised.exception))

        with self.assertRaises(pty_session.TapeError) as raised:
            self.parse(self.PROLOGUE + "Wait+Line /done/\n")
        self.assertIn("Line", str(raised.exception))

    def test_hide_suspends_recording(self):
        """AC-2: no byte emitted between `Hide` and `Show` reaches the cast."""
        _, events = self.record(
            self.PROLOGUE + 'Type "echo VISIBLE-ONE"\n'
            "Enter\n"
            "Wait+Screen /VISIBLE-ONE/\n"
            "Hide\n"
            'Type "echo SECRET-TEXT"\n'
            "Enter\n"
            "Sleep 500ms\n"
            "Show\n"
            'Type "echo VISIBLE-TWO"\n'
            "Enter\n"
            "Wait+Screen /VISIBLE-TWO/\n"
        )

        text = "".join(event[2] for event in events)
        self.assertIn("VISIBLE-ONE", text)
        self.assertIn("VISIBLE-TWO", text)
        self.assertNotIn("SECRET", text)
        # Nothing was cleared while hidden, so the reader's screen is still
        # the terminal's screen and the recorder says nothing about it.
        self.assertNotIn(pty_session.ERASE_SCREEN, text)

    def test_a_hidden_clear_hands_back_the_screen_it_left(self):
        """A hidden `clear` reaches the cast as a reset plus what survived it.

        Sixty of the tapes' sixty-one hidden regions end in a `clear`, or in a
        card whose script starts with one, so a reader whose screen kept the
        section before it would read the whole demo through the setup it was
        never meant to see. The reset says the screen was thrown away, and what
        the terminal painted AFTER it is the screen the reader resumes on: the
        shell prompt, in every tape that ends its hidden region with `clear`.
        Output erased BEFORE the reset is the setup, and it stays out (AC-2).

        Phase 3 of the spec measured the alternative. With the reset alone,
        `cli-dashboard` showed `sshpass -e ssh ze-demo` typed onto a bare line,
        where the transcript and the video it replaces both show it behind the
        prompt the clear painted, and the transcript gate failed on it.
        """
        _, events = self.record(
            self.PROLOGUE + 'Type "echo VISIBLE-ONE"\n'
            "Enter\n"
            "Wait+Screen /VISIBLE-ONE/\n"
            "Hide\n"
            'Type "echo SECRET-TEXT"\n'
            "Enter\n"
            "Type \"clear; printf 'SCREEN-KEPT'\"\n"
            "Enter\n"
            "Sleep 500ms\n"
            "Show\n"
            'Type "echo VISIBLE-TWO"\n'
            "Enter\n"
            "Wait+Screen /VISIBLE-TWO/\n"
        )

        text = "".join(event[2] for event in events)
        # Everything the clear erased is gone, the command that produced it
        # included: the typed line was echoed before the screen was thrown away.
        self.assertNotIn("SECRET", text)
        self.assertNotIn("clear;", text)
        reset = text.index(pty_session.ERASE_SCREEN)
        self.assertLess(text.index("VISIBLE-ONE"), reset)
        self.assertLess(reset, text.index("VISIBLE-TWO"))
        # What the terminal painted after the clear is on the screen the reader
        # is handed back, and it arrives with the reset rather than later.
        self.assertIn("SCREEN-KEPT", text)
        self.assertLess(reset, text.index("SCREEN-KEPT"))
        self.assertLess(text.index("SCREEN-KEPT"), text.index("VISIBLE-TWO"))

    def test_hidden_region_rebases_the_clock(self):
        """AC-3: a hidden region leaves neither a gap nor a rewind behind it.

        The tape hides for four seconds and does almost nothing else, so a
        recorder that merely stopped writing would leave a four-second gap in
        the middle and a four-second tail on the end. Both are asserted away.
        """
        _, events = self.record(
            self.PROLOGUE + 'Type "echo BEFORE-HIDE"\n'
            "Enter\n"
            "Wait+Screen /BEFORE-HIDE/\n"
            "Hide\n"
            "Sleep 4s\n"
            "Show\n"
            'Type "echo AFTER-HIDE"\n'
            "Enter\n"
            "Wait+Screen /AFTER-HIDE/\n"
        )

        times = [event[0] for event in events]
        self.assertGreater(len(times), 2, events)
        self.assertEqual(times, sorted(times))
        self.assertGreaterEqual(times[0], 0.0)
        gaps = [later - earlier for earlier, later in zip(times, times[1:])]
        self.assertLess(max(gaps), 1.0, times)
        self.assertLess(times[-1], 2.5, times)

    def test_cast_header_carries_tape_dimensions(self):
        """AC-4: the cast's grid is the tape's box, and the terminal is that grid.

        `stty size` is answered by the terminal the recorder opened, so the two
        halves are checked against each other: a header that agreed with the
        tape while the shell painted into a different grid would still wrap
        every line at the wrong column.
        """
        header, events = self.record(
            self.PROLOGUE + 'Type "stty size"\nEnter\nWait+Screen /36 137/\n'
        )
        self.assertEqual((header["width"], header["height"]), (137, 36))
        self.assertEqual(header["version"], 2)
        self.assertIn("36 137", "".join(event[2] for event in events))

        # A second box, so the grid is derived from the tape rather than fixed.
        header, events = self.record(
            self.PROLOGUE + "Set Width 800\n"
            "Set Height 480\n"
            'Type "stty size"\n'
            "Enter\n"
            "Wait+Screen /16 63/\n"
        )
        self.assertEqual((header["width"], header["height"]), (63, 16))
        self.assertIn("16 63", "".join(event[2] for event in events))

        # A box of no pixels has no grid, and is refused rather than rounded
        # up to one column (the spec's boundary table).
        with self.assertRaises(pty_session.TapeError) as raised:
            self.parse(self.PROLOGUE + "Set Width 0\n")
        self.assertIn("Width", str(raised.exception))


class TranscriptGateTest(unittest.TestCase):
    """AC-5, R-10: the gate on the engine, and the clock the artifact stores.

    The transcript is the one description of a demo that the recorder did not
    produce, so it is the only thing in the tree that can tell a faithful
    re-drive from a drifted one (A-4). These tests drive `_render_demo` itself
    rather than the gate alone: a gate that is never called is the failure this
    spec is most exposed to, because a render that publishes a wrong cast looks
    exactly like a render that publishes a right one.
    """

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = pathlib.Path(self.tmp.name)
        self.demo_root = self.root / "demos" / "terminal"
        (self.demo_root / "example").mkdir(parents=True)
        (self.demo_root / "common.tape").write_text(
            'Set Shell "bash"\nSet TypingSpeed 125ms\n', encoding="utf-8"
        )
        (self.demo_root / "example" / "demo.tape").write_text(
            "Output artifacts/example.webm\nSource common.tape\nSleep 10s\n",
            encoding="utf-8",
        )
        for name in (
            "cards.sh",
            "Dockerfile",
            "container-entrypoint.sh",
            "demo-lock.sh",
            "validate-common.sh",
            "pty-session.py",
            "render.py",
        ):
            (self.demo_root / name).write_text(name, encoding="utf-8")
        self.binary = self.root / "tmp" / "terminal-demos" / "bin" / "ze"
        self.binary.parent.mkdir(parents=True)
        self.binary.write_text("ze", encoding="utf-8")
        self.artifact_root = self.root / "gh-pages" / "assets" / "demos"
        self.artifact_root.mkdir(parents=True)
        patch = mock.patch.multiple(
            terminal_render,
            ROOT=self.root,
            DEMO_ROOT=self.demo_root,
            ARTIFACT_ROOT=self.artifact_root,
            ARTIFACT_MANIFEST_PATH=self.artifact_root / "manifest.json",
            BINARY_PATH=self.binary,
            SHARED_SOURCE_PATHS=(self.demo_root / "common.tape",),
        )
        patch.start()
        self.addCleanup(patch.stop)
        self.demo = {"id": "example", "kind": "terminal", "source": "example/demo.tape"}
        self.renderer = {
            "name": "ze-demo",
            "version": "2",
            "image": "ze-terminal-demo-render-all:test",
            "platform": "linux/native",
        }

    def write_cast(self, painted, *, columns=137, rows=36, step=0.5):
        """Write the cast the container would have produced.

        `painted` is what the terminal shows, one entry per painted run, and
        each entry becomes one event ending in the CRLF a PTY writes.
        """
        lines = [json.dumps({"version": 2, "width": columns, "height": rows})]
        for index, text in enumerate(painted, start=1):
            lines.append(json.dumps([round(index * step, 6), "o", text + "\r\n"]))
        path = self.artifact_root / "example.cast"
        path.write_text("\n".join(lines) + "\n", encoding="utf-8")
        return path

    def transcript(self, text):
        path = self.demo_root / "example" / "transcript.txt"
        path.write_text(text, encoding="utf-8")
        return path

    def render(self, painted, **kwargs):
        """Run `_render_demo` with the container replaced by a written cast."""

        def fake_run(command, cwd=None, check=None):
            self.write_cast(painted, **kwargs)

        with mock.patch.object(terminal_render.subprocess, "run", fake_run):
            return terminal_render._render_demo(
                {"renderer": self.renderer}, self.demo, "test"
            )

    def test_transcript_mismatch_fails_the_render(self):
        """AC-5: a cast that does not show the claimed session publishes nothing."""
        self.transcript(
            "An operator checks the build.\n"
            "\n"
            "$ ze show version\n"
            "The version banner names the release.\n"
        )
        with self.assertRaises(RuntimeError) as raised:
            self.render(["$ ze show config", "running-config is empty", "$ "])

        message = str(raised.exception)
        self.assertIn("example", message)
        self.assertIn("transcript.txt:3", message)
        self.assertIn("ze show version", message)
        # Nothing published: no cast and no transcript in the artifact tree,
        # and the recording is where the next reader can still see it.
        self.assertFalse((self.artifact_root / "example.cast").exists())
        self.assertFalse((self.artifact_root / "example.txt").exists())
        self.assertTrue(
            (
                self.root / "tmp" / "terminal-demos" / "rejected" / "example.cast"
            ).is_file()
        )

    def test_transcript_mismatch_quarantines_across_filesystems(self):
        painted = ["$ ze show config", "running-config is empty", "$ "]
        self.transcript("$ ze show version\n")
        cast_path = self.write_cast(painted)

        with tempfile.TemporaryDirectory() as cache_tmp:
            cache_root = pathlib.Path(cache_tmp).resolve()
            expected_cast = cache_root / "expected.cast"
            shutil.copy2(cast_path, expected_cast)
            terminal_render.expand_cast_timeline(
                expected_cast, terminal_render.capture_speedup(self.demo)
            )
            expected_bytes = expected_cast.read_bytes()
            cast_path.unlink()

            cache_target = cache_root / "terminal-demos"
            cache_target.mkdir()
            scratch_root = self.root / "tmp" / "terminal-demos"
            shutil.rmtree(scratch_root)
            scratch_root.symlink_to(cache_target, target_is_directory=True)
            cross_device = OSError(
                errno.EXDEV, os.strerror(errno.EXDEV), str(cast_path)
            )

            with (
                mock.patch.object(
                    terminal_render.os, "rename", side_effect=cross_device
                ),
                mock.patch.object(
                    terminal_render.os, "replace", side_effect=cross_device
                ),
            ):
                with self.assertRaises(RuntimeError):
                    self.render(painted)

            rejected = cache_target / "rejected" / "example.cast"
            self.assertFalse(cast_path.exists())
            self.assertEqual(rejected.read_bytes(), expected_bytes)

    def test_a_faithful_cast_passes_the_render(self):
        """The other half of AC-5: the gate accepts the session it describes.

        Without this the gate could be a function that always raises, and the
        test above would still be green.
        """
        self.transcript("$ ze show version\nThe version banner names the release.\n")
        entry = self.render(["$ ze show version", "ze 1.0.0", "$ "])
        self.assertEqual(sorted(entry["assets"]), ["cast", "transcript"])
        self.assertTrue((self.artifact_root / "example.cast").is_file())

    def test_the_gate_reads_through_the_escape_sequences(self):
        """The visible-text rule: colour and cursor moves are not text.

        A recorded terminal wraps almost every line in SGR sequences, so a gate
        that compared raw payloads would fail every real demo.
        """
        self.transcript("$ ze show version\n")
        self.render(
            [
                "\x1b[?25l\x1b[1;1H$ \x1b[32mze show version\x1b[0m\x1b[?25h",
                "\x1b[38;5;213mze 1.0.0\x1b[m",
            ]
        )
        visible = terminal_render.cast_visible_text(self.artifact_root / "example.cast")
        self.assertIn("$ ze show version", visible)
        self.assertIn("ze 1.0.0", visible)

    def test_commands_shown_out_of_order_fail_the_gate(self):
        """Order is gated, not only presence.

        Both commands are in the recording, so a containment check would pass.
        A demo whose steps happen in the wrong order shows the reader a
        different session from the one the transcript describes.
        """
        self.transcript("$ ze config validate\n$ ze config commit\n")
        with self.assertRaises(RuntimeError) as raised:
            self.render(["$ ze config commit", "$ ze config validate"])
        self.assertIn("ze config commit", str(raised.exception))

    def test_a_transcript_quoting_no_command_fails_the_render(self):
        """A gate with nothing to check must say so rather than pass.

        A transcript that is all narration would make the gate vacuous for that
        demo, silently, and vacuity is exactly what AC-5 exists to prevent.
        """
        self.transcript("The dashboard polls three local BGP sessions.\n")
        with self.assertRaises(RuntimeError) as raised:
            self.render(["$ ze show version"])
        self.assertIn("gates nothing", str(raised.exception))

    def test_the_stored_cast_is_paced_in_real_time(self):
        """R-10: the capture is accelerated and the artifact is not.

        The tape is driven with its sleeps divided by RENDER_SPEEDUP and its
        typing set to the same fraction (125ms / 5 = 25ms), so ONE factor
        describes the whole capture and multiplying the timeline by it restores
        the timing the tape states. A cast has no ffmpeg pass to do that, which
        is the defect this asserts against: an unscaled cast replays every
        pause five times too fast.
        """
        self.transcript("$ ze show version\n")
        self.render(["$ ze show version", "ze 1.0.0"], step=1.0)
        events = [
            json.loads(line)
            for line in (self.artifact_root / "example.cast")
            .read_text(encoding="utf-8")
            .splitlines()[1:]
        ]
        self.assertEqual([event[0] for event in events], [5.0, 10.0])

    def test_a_real_time_demo_keeps_the_clock_it_recorded(self):
        """A demo that captured at wall-clock speed is not stretched.

        `capture_speedup` returns 1 for it, and the tape it drove was the
        checked-in one, so its timestamps are already the tape's own.
        """
        self.demo["realtime"] = True
        self.transcript("$ ze show version\n")
        self.render(["$ ze show version", "ze 1.0.0"], step=1.0)
        events = [
            json.loads(line)
            for line in (self.artifact_root / "example.cast")
            .read_text(encoding="utf-8")
            .splitlines()[1:]
        ]
        self.assertEqual([event[0] for event in events], [1.0, 2.0])

    def test_expanding_a_cast_leaves_its_header_alone(self):
        """The grid and the version are not a clock.

        The header is what the player sizes the terminal from, and the artifact
        is committed (D-1), so a pass that rewrote it would move bytes nobody
        asked to move.
        """
        cast = self.write_cast(["$ ze show version"])
        header = cast.read_text(encoding="utf-8").splitlines()[0]
        terminal_render.expand_cast_timeline(cast, 5)
        self.assertEqual(cast.read_text(encoding="utf-8").splitlines()[0], header)


class ScreenTest(unittest.TestCase):
    """`screen.py`: the difference between a byte stream and a screen.

    Every case here is a sequence a Ze demo actually emits, and every one of
    them was found by a render that failed against it. The recorder searches
    `text` for a tape's `Wait+Screen`, and the transcript gate searches
    `painted` for what a reader was shown, so a mechanism missing here is a
    demo that cannot be recorded or a gate that cannot see it.
    """

    def paint(self, *bursts, height=36, width=137):
        painted = terminal_screen.Screen(height, width)
        for burst in bursts:
            painted.settle(burst)
        return painted

    def test_an_inline_completion_leaves_the_typed_text(self):
        """The Ze CLI editor's ghost completion, one keystroke at a time.

        Typing "m" emits "m", the dim completion "onitor", an erase to the end
        of the line, and a cursor move back over the completion. The stream
        holds "monitoronit..."; the screen holds what was typed.
        """
        bursts = []
        typed = "monitor"
        for index, character in enumerate(typed):
            ghost = typed[index + 1 :]
            bursts.append(f"\x1b[36;{5 + index}H{character}{ghost}\x1b[K")
        screen = self.paint("\x1b[36;1Hze> ", *bursts)
        self.assertIn("ze> monitor", screen.text())

    def test_a_scrolling_region_leaves_the_rows_outside_it(self):
        """`CSI 6;32r` then `CSI 7S`: the editor scrolls its answer, not itself.

        `zefs-config` sets a region over the answer area and scrolls inside it.
        A scroll of the whole screen carries the status line and the prompt off
        with it, and every absolute write afterwards lands on the wrong row.
        """
        screen = self.paint(
            "\x1b[10;1Hanswer\x1b[34;1Hstatus\x1b[36;1Hze# commit",
            "\x1b[6;32r\x1b[32;1H\x1b[7S\x1b[1;36r",
        )
        rows = screen.text().splitlines()
        self.assertIn("status", rows)
        self.assertIn("ze# commit", rows)
        self.assertNotIn("answer", rows)

    def test_a_deleted_line_pulls_the_rest_up(self):
        """`CSI M`: how the launcher removes a row from its filtered list."""
        screen = self.paint("\x1b[1;1Hone\r\ntwo\r\nthree", "\x1b[2;1H\x1b[M")
        self.assertEqual(screen.text().splitlines(), ["one", "three"])

    def test_reverse_index_moves_up_a_row(self):
        """`ESC M`: how the launcher rewrites the line above the cursor."""
        screen = self.paint("\x1b[1;1Hone\r\ntwo", "\x1bM\rONE")
        self.assertEqual(screen.text().splitlines(), ["ONE", "two"])

    def test_the_screen_keeps_the_blanks_to_its_right_edge(self):
        """Six tapes wait for `/\\$ /`, and the container prompt is `$ `.

        A row stripped of its trailing blanks holds `$`, and those six waits
        time out against a screen that shows the prompt.
        """
        self.assertIn("$ ", self.paint("\x1b[1;1H$ ").text())

    def test_a_row_repainted_where_it_stands_reaches_the_history(self):
        """The gate's half: what the cursor never leaves is still shown.

        The Ze editor replaces `ze# commit` with the answer to it without the
        cursor going anywhere, so nothing but the settle between two bursts
        records that the reader saw the command.
        """
        # No erase and no cursor move off the row: the second burst simply
        # writes over the first. Only the settle between them can see it.
        screen = self.paint("\x1b[36;1Hze# commit", "\x1b[36;1Hze# ......")
        self.assertIn("ze# commit", screen.painted())

    def test_a_line_wider_than_the_screen_is_read_as_two(self):
        """A terminal wraps at its width, and so does the player replaying it.

        `zefs-config` paints a 265-column table row into a 137-column terminal.
        A model that let the line run holds one line where the reader read two.
        """
        screen = self.paint("\x1b[1;1H" + "x" * 200)
        self.assertEqual([len(row) for row in screen.text().splitlines()], [137, 63])

    def test_a_row_that_scrolls_off_the_top_reaches_the_history(self):
        """A transcript quotes commands from the whole session, not one screen."""
        screen = self.paint(
            "\x1b[1;1Hfirst", *[f"\x1b[{row};1Hrow{row}" for row in range(2, 40)]
        )
        self.assertIn("first", screen.painted())
        self.assertNotIn("first", screen.text())


class ManifestDurationTest(unittest.TestCase):
    """The manifest states a duration only for a demo that records no cast.

    VALIDATES: a cast carries its own running time, so the page reads the
    number off the artifact instead of off the manifest
    (`website/tools/terminal_demos._load_catalog`).
    PREVENTS: the second source of truth coming back. Four published durations
    had already drifted from their recordings while the field was required of
    every demo: `cli-dashboard` said 40 seconds against a 55.0s cast, and
    `zefs-config` said 2 minutes 21 seconds against 200.3s.
    """

    def manifest(self):
        return json.loads(terminal_render.MANIFEST_PATH.read_text())

    def demo(self, manifest, kind):
        for demo in manifest["demos"]:
            if demo["kind"] == kind:
                return demo
        raise AssertionError(f"the manifest carries no {kind} demo")

    def test_the_checked_in_manifest_is_accepted(self):
        terminal_render.validate_contract(self.manifest())

    def test_a_recorded_demo_may_not_state_a_duration(self):
        manifest = self.manifest()
        demo = self.demo(manifest, "terminal")
        demo["duration"] = "40 seconds"

        with self.assertRaises(ValueError) as raised:
            terminal_render.validate_contract(manifest)

        self.assertIn(f"{demo['id']}.duration", str(raised.exception))

    def test_a_demo_with_no_cast_still_states_its_duration(self):
        manifest = self.manifest()
        demo = self.demo(manifest, "browser")
        del demo["duration"]

        with self.assertRaises(ValueError) as raised:
            terminal_render.validate_contract(manifest)

        self.assertIn(f"{demo['id']}.duration is required", str(raised.exception))


if __name__ == "__main__":
    unittest.main()
