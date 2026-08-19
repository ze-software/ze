#!/usr/bin/env python3
"""Unit tests for scripts/dev/ze-run.sh, the admission point for heavy jobs.

Several Claude sessions share one checkout and one machine, and every heavy
target is sized for the whole box. Two agents starting `make ze-lint` at the
same moment oversubscribe the machine until it stops responding (owner report,
2026-08-17). ze-run.sh is the one place that decides whether a heavy job runs
now or waits, and this file drives it from its command line -- the entry point
an agent reaches through a make recipe -- rather than from any helper inside it.

Every test runs inside a throwaway directory, because the registry lives at the
repo-relative path tmp/.ze-jobs/ and the tests must never touch the real one.
The script resolves its siblings from its own location, so it works from any
cwd.

Two properties are load-bearing and each has its own test:

  - a slot is a slot. A second heavy job WAITS; it is never let through
    unadmitted, and it never kills the holder to get in (AC-2).
  - a slot is reclaimed, not lost. A registry entry whose process is dead is
    reaped by the next waiter with no operator action (AC-7). Without this the
    first crashed job wedges every session, which is worse than the freeze the
    wrapper exists to prevent.

Fail-closed is the third: an entry the wrapper cannot read counts as OCCUPIED.
Reading "cannot parse" as "nothing is running" would let every session in at
once, which is the failure, so the ambiguous case queues.

The fourth is liveness. A slot is broken when the holder is GONE, or when the
holder is alive and its log stopped growing for the stall window -- never
because the holder has been running a long time (AC-6). The age rule this
replaced killed a 20 minute verify at 1800 seconds, which is the run this
wrapper exists to protect, so the test that a slow live holder survives is the
one that would have failed before.

The fifth is attach and share, and it is the point of the wrapper. Serializing
alone puts eight agents in a line for eight runs of the same thing; a second
asker for the SAME label on the SAME tree follows the running job and exits
with its code instead (AC-3). The safety half is the tree hash: the same label
on a DIFFERENT tree queues, because a shared run that never saw the asker's
code would certify it anyway (AC-4). The attach tests therefore make their
throwaway root a git repository, so the hash they compare is the one
verify-status.sh really computes over a tree, and changing the tree between
two asks is what tells the two tests apart.
"""

from __future__ import annotations

import os
import re
import signal
import subprocess
import tempfile
import time
import unittest
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
ZE_RUN = REPO / "scripts" / "dev" / "ze-run.sh"

# ze-run.sh polls the registry every POLL_SECONDS while it waits. The waits
# below are expressed in multiples of it so a change to the script's constant
# shows up as a failure here rather than as flakiness.
POLL_SECONDS = 2
# How long to let a queued job prove it stays queued: two poll intervals plus
# process startup.
QUEUED_FOR = POLL_SECONDS * 2 + 1
# Ceiling on any admission that should happen promptly.
ADMITTED_WITHIN = 30


class ZeRunCase(unittest.TestCase):
    """A throwaway working root per test, with tmp/.ze-jobs/ inside it."""

    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.root = Path(self._tmp.name)
        self.jobs = self.root / "tmp" / ".ze-jobs"
        self._running: list[subprocess.Popen] = []

    def tearDown(self) -> None:
        for proc in self._running:
            if proc.poll() is None:
                proc.kill()
                proc.wait(timeout=10)
            for stream in (proc.stdin, proc.stdout, proc.stderr):
                if stream is not None and not stream.closed:
                    stream.close()
        self._tmp.cleanup()

    # -- helpers ---------------------------------------------------------

    def env(self, **overrides: str) -> dict:
        env = dict(os.environ)
        # The wrapper must not inherit a parent job from the session running
        # these tests: that is the nested path, and only one test wants it.
        env.pop("ZE_RUN_JOB", None)
        # Nor the slot count. The Makefile exports it, so a test run through
        # make would otherwise measure the box's number instead of one slot,
        # and every serialization test here asserts that the second job waits.
        env.pop("ZE_RUN_SLOTS", None)
        env.update(overrides)
        return env

    def spawn(
        self, *argv: str, stdin=None, name: str = "", **kwargs
    ) -> subprocess.Popen:
        """Start ze-run.sh in the throwaway root, capturing output to files.

        `name` separates the capture files when two jobs share one label, which
        is the shape every attach test needs: attaching requires the labels to
        match.
        """
        label = name or argv[0]
        out = open(self.root / f"{label}.out", "wb")
        err = open(self.root / f"{label}.err", "wb")
        proc = subprocess.Popen(
            [str(ZE_RUN), *argv],
            cwd=self.root,
            stdin=stdin,
            stdout=out,
            stderr=err,
            env=kwargs.pop("env", self.env()),
            **kwargs,
        )
        proc.label = label  # type: ignore[attr-defined]
        self._running.append(proc)
        return proc

    def stderr_of(self, proc: subprocess.Popen) -> str:
        return (self.root / f"{proc.label}.err").read_text(errors="replace")

    def entries(self) -> list[Path]:
        return sorted(self.jobs.glob("*.job")) if self.jobs.is_dir() else []

    def wait_for_entry(self, label: str, timeout: int = ADMITTED_WITHIN) -> Path:
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            found = [p for p in self.entries() if p.name.startswith(f"{label}.")]
            if found:
                return found[0]
            time.sleep(0.1)
        self.fail(f"no registry entry for {label} within {timeout}s")

    def wait_for_file(self, path: Path, timeout: int = ADMITTED_WITHIN) -> None:
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if path.exists():
                return
            time.sleep(0.1)
        self.fail(f"{path} did not appear within {timeout}s")

    def fields(self, entry: Path) -> dict:
        out = {}
        for line in entry.read_text().splitlines():
            key, _, value = line.partition("=")
            out[key] = value
        return out

    def write_entry(self, name: str, body: str) -> Path:
        self.jobs.mkdir(parents=True, exist_ok=True)
        entry = self.jobs / name
        entry.write_text(body)
        return entry

    def backdate_started(self, entry: Path, seconds: int) -> None:
        """Make a live job look as though it started `seconds` ago.

        AC-6 is about a holder that has outlived the 1800s age limit this
        wrapper used to enforce. Rewriting STARTED reaches that state in a few
        seconds instead of half an hour, and it leaves the log alone -- the log
        is the fact the waiter must judge.
        """
        lines = [
            f"STARTED={int(time.time()) - seconds}"
            if line.startswith("STARTED=")
            else line
            for line in entry.read_text().splitlines()
        ]
        staging = entry.parent / (entry.name + ".tmp")
        staging.write_text("\n".join(lines) + "\n")
        os.replace(staging, entry)

    def dead_pid(self) -> int:
        """A PID that is certainly not running: a child we started and reaped."""
        proc = subprocess.Popen(["true"])
        proc.wait()
        return proc.pid

    def git_tree(self) -> None:
        """Make the throwaway root a tree `verify-status.sh tree_hash` can read.

        The hash is over HEAD, the diff, and the untracked files, so a plain
        directory hashes to one constant and no test could tell two trees
        apart. The ignore file keeps the wrapper's own logs, the capture files
        and the markers out of it: they change while a job runs, and a tree
        hash that moves under a running job would make attaching impossible for
        a reason no production tree has. Only `tracked-*` is tree content here.
        """
        subprocess.run(["git", "init", "-q", "."], cwd=self.root, check=True)
        (self.root / ".gitignore").write_text("*\n!tracked-*\n")

    def tree_hash(self) -> str:
        """The fingerprint ze-run.sh records, computed by its own producer."""
        proc = subprocess.run(
            [str(REPO / "scripts" / "dev" / "verify-status.sh"), "tree_hash"],
            cwd=self.root,
            capture_output=True,
            text=True,
            check=True,
        )
        return proc.stdout.strip()

    def change_the_tree(self) -> None:
        """What another session doing its own work looks like to the hash."""
        (self.root / "tracked-change").write_text("another session edited this\n")

    def wait_for_text(
        self, path: Path, text: str, timeout: int = ADMITTED_WITHIN
    ) -> None:
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if path.exists() and text in path.read_text(errors="replace"):
                return
            time.sleep(0.1)
        self.fail(f"{path} never carried {text!r} within {timeout}s")

    # -- tests -----------------------------------------------------------

    def test_a_second_heavy_job_waits_for_the_slot(self):
        """AC-2: one runs, the second waits, and neither is killed."""
        # `cat` holds the slot until the test closes its stdin, so the holder's
        # lifetime is controlled without signals.
        holder = self.spawn("holder", "cat", stdin=subprocess.PIPE)
        self.wait_for_entry("holder")

        marker = self.root / "waiter-ran"
        waiter = self.spawn("waiter", "touch", str(marker))
        time.sleep(QUEUED_FOR)

        self.assertIsNone(waiter.poll(), "the second job must wait, not exit")
        self.assertFalse(marker.exists(), "the second job ran unadmitted")
        self.assertIsNone(holder.poll(), "the waiter must not kill the holder")
        banner = self.stderr_of(waiter)
        self.assertIn("holder", banner, "the banner must name the holder's label")
        self.assertIn("elapsed", banner, "the banner must report elapsed time")

        holder.stdin.close()
        self.assertEqual(holder.wait(timeout=ADMITTED_WITHIN), 0)
        self.assertEqual(waiter.wait(timeout=ADMITTED_WITHIN), 0)
        self.assertTrue(marker.exists(), "the waiter never got the freed slot")

    def test_an_equivalent_job_is_attached_not_queued(self):
        """AC-3: same label, same tree -> one run, two answers, one exit code.

        This is what the wrapper exists for. Serializing alone would run the
        second copy after the first, at twice the cost, for the same verdict.
        The run counter is the assertion that matters: it must hold ONE line
        however the two processes are scheduled.
        """
        self.git_tree()
        runs = self.root / "runs"
        job = f"echo start >> {runs}; echo holder-output; sleep 6; echo holder-finished; exit 7"

        holder = self.spawn("ze-lint", "sh", "-c", job, name="holder")
        self.wait_for_entry("ze-lint")
        sharer = self.spawn("ze-lint", "sh", "-c", job, name="sharer")

        self.assertEqual(
            sharer.wait(timeout=ADMITTED_WITHIN),
            7,
            f"the sharer must exit with the shared job's code: {self.stderr_of(sharer)}",
        )
        self.assertEqual(holder.wait(timeout=ADMITTED_WITHIN), 7)
        self.assertEqual(
            runs.read_text().count("start"), 1, "the equivalent job ran a second time"
        )

        shared = (self.root / "sharer.out").read_text()
        self.assertIn("holder-output", shared, "the sharer saw none of the run")
        self.assertIn(
            "holder-finished", shared, "the sharer stopped following before the end"
        )
        self.assertEqual(
            [p.name for p in self.entries()],
            [],
            "a sharer must leave no registry entry",
        )
        durations = (self.root / "tmp" / ".ze-verify-duration.txt").read_text()
        self.assertEqual(
            durations.count("ze-lint\t"), 1, "a sharer recorded a run it never made"
        )

    def test_a_different_tree_hash_does_not_attach(self):
        """AC-4: same label, different tree -> it queues and runs its own job.

        The safety half of attach-and-share. A job that attached on the label
        alone would hand this asker a verdict about code the running job never
        read, and `full_verify_coverage` in scripts/dev/commit_helper.py takes
        that verdict as the asker's commit evidence.
        """
        self.git_tree()
        runs = self.root / "runs"
        holder = self.spawn(
            "ze-lint",
            "sh",
            "-c",
            f"echo start >> {runs}; echo holder-output; cat",
            name="holder",
            stdin=subprocess.PIPE,
        )
        self.wait_for_entry("ze-lint")
        self.change_the_tree()

        marker = self.root / "second-ran"
        waiter = self.spawn(
            "ze-lint",
            "sh",
            "-c",
            f"echo start >> {runs}; touch {marker}",
            name="waiter",
        )
        time.sleep(QUEUED_FOR)

        self.assertIsNone(waiter.poll(), "a job for another tree must queue")
        self.assertFalse(marker.exists(), "it ran unadmitted")
        self.assertNotIn(
            "holder-output",
            (self.root / "waiter.out").read_text(),
            "it attached to a run that never saw its code",
        )
        self.assertIn("waiting", self.stderr_of(waiter))

        holder.stdin.close()
        self.assertEqual(holder.wait(timeout=ADMITTED_WITHIN), 0)
        self.assertEqual(waiter.wait(timeout=ADMITTED_WITHIN), 0)
        self.assertTrue(marker.exists(), "the queued job never got the freed slot")
        self.assertEqual(
            runs.read_text().count("start"), 2, "the second asker must be its own run"
        )

    def _two_askers(
        self,
        holder_flags: str,
        waiter_flags: str,
        holder_job: str = "",
        waiter_job: str = "",
        holder_env: dict | None = None,
        waiter_env: dict | None = None,
    ) -> int:
        """Start two askers for one label on one tree, return how many RAN.

        One is the answer when the second attached to the first; two is the
        answer when it decided the work was different and ran its own. The two
        commands are the same string unless a caller wants them apart, so the
        only difference between the askers is the one the case is about.

        The holder sleeps long enough for the waiter to reach its decision, and
        every assertion is over the run counter rather than over timing.
        """
        self.git_tree()
        runs = self.root / "runs"
        default = f"echo start >> {runs}; echo shared-output; sleep 6; exit 7"
        holder = self.spawn(
            "ze-unit-pkg-test",
            "sh",
            "-c",
            holder_job or default,
            name="holder",
            env=self.env(MAKEFLAGS=holder_flags, **(holder_env or {})),
        )
        self.wait_for_entry("ze-unit-pkg-test")
        waiter = self.spawn(
            "ze-unit-pkg-test",
            "sh",
            "-c",
            waiter_job or default,
            name="waiter",
            env=self.env(MAKEFLAGS=waiter_flags, **(waiter_env or {})),
        )

        self.assertEqual(
            holder.wait(timeout=ADMITTED_WITHIN * 2), 7, self.stderr_of(holder)
        )
        waiter.wait(timeout=ADMITTED_WITHIN * 2)
        return runs.read_text().count("start")

    def test_a_different_parameter_set_does_not_attach(self):
        """The work key carries the make variables, so two packages are two jobs.

        `make ze-unit-pkg-test PKG=./a` and the same target on `./b` hand this
        wrapper the SAME label and the SAME command: PKG reaches the impl half
        alone. Keyed on the label the second asker attached and reported the
        first run's exit code as its own. On 2026-08-19 that read green for a
        package that was never compiled (plan/journal/stale-artifact-reused.md).
        """
        ran = self._two_askers(
            " --no-print-directory -- PKG=./package-a",
            " --no-print-directory -- PKG=./package-b",
        )
        self.assertEqual(ran, 2, "one run answered for two different packages")

    def test_the_same_parameter_set_still_attaches(self):
        """The sharing that is correct survives: same parameters, one run.

        The fix must split the jobs the parameters make different, and only
        those. Two askers for the same package are the same work, and paying
        for it twice is what the attach path exists to prevent.
        """
        flags = " --no-print-directory -- PKG=./package-a RUN=^TestOne$"
        self.assertEqual(self._two_askers(flags, flags), 1, "an equivalent job re-ran")

    def test_the_parameter_order_does_not_split_a_shared_job(self):
        """make lists the definitions in the caller's own order; the key sorts.

        Two sessions typing the same pair the other way round are asking for
        one thing, so the key is over the SET of definitions.
        """
        ran = self._two_askers(
            " --no-print-directory -- PKG=./package-a RUN=^TestOne$",
            " --no-print-directory -- RUN=^TestOne$ PKG=./package-a",
        )
        self.assertEqual(ran, 1, "the same parameters in another order re-ran")

    def test_an_admission_knob_does_not_split_a_shared_job(self):
        """This wrapper's own variables choose HOW a job is admitted, not WHAT it does.

        `make <target> ZE_RUN_SLOTS=1` is documented in the Makefile, and
        ai/rules/git-safety.md tells readers to raise ZE_VERIFY_MAX_LOCK_AGE.
        Neither can change a verdict, so neither may cost a duplicate run.
        """
        ran = self._two_askers(
            " --no-print-directory -- PKG=./package-a",
            " --no-print-directory -- ZE_RUN_SLOTS=1 PKG=./package-a"
            " ZE_JOB_STALL_SECONDS=900 ZE_VERIFY_MAX_LOCK_AGE=900",
        )
        self.assertEqual(ran, 1, "an admission knob was read as different work")

    def test_the_environment_can_decline_sharing(self):
        """MAY_ATTACH=0 makes a job queue for its own run, as the rules say.

        This is the opt-out ai/rules and plan/journal name for a caller that
        wants its OWN verdict, and it was inert: an unconditional MAY_ATTACH=1
        before the admission loop overwrote whatever the caller exported, so a
        session using the documented escape still took the running job's exit
        code. Measured on 2026-08-19 while another session was relying on it.
        """
        flags = " --no-print-directory -- PKG=./package-a"
        ran = self._two_askers(flags, flags, waiter_env={"MAY_ATTACH": "0"})
        self.assertEqual(ran, 2, "MAY_ATTACH=0 did not decline the shared run")

    def test_declining_to_attach_does_not_stop_others_sharing_this_job(self):
        """The flag governs this job's ASKING, and nothing else.

        A job that declined to attach still runs its own work honestly, so its
        verdict is as good as any other job's. Keying on the flag would have
        turned "I will not take your answer" into "you may not take mine", and
        the caller asked for the first.
        """
        flags = " --no-print-directory -- PKG=./package-a"
        ran = self._two_askers(flags, flags, holder_env={"MAY_ATTACH": "0"})
        self.assertEqual(ran, 1, "a job that declined to attach became unshareable")

    def test_two_commands_under_one_label_do_not_attach(self):
        """The hand-queued route is keyed too: `ze-run.sh <label> <command>`.

        ai/rules/commands.md tells an agent to queue work no make target
        expresses by naming a label itself. Two agents choosing one label for
        two commands is the same defect with no make in it.
        """
        runs = self.root / "runs"
        ran = self._two_askers(
            "",
            "",
            holder_job=f"echo start >> {runs}; echo first; sleep 6; exit 7",
            waiter_job=f"echo start >> {runs}; echo second; sleep 6; exit 7",
        )
        self.assertEqual(ran, 2, "one run answered for two different commands")

    def test_a_holder_that_dies_mid_attach_leaves_no_verdict_behind(self):
        """A killed job records nothing, so its sharer reports nothing.

        The sharer must not hang on a corpse and must not invent a pass. It
        goes back to admission and runs the job itself, which is the answer it
        would have had with no attach path at all.

        The two askers run one command string, because the work key reads the
        command: a sharer typing something else is a different job and never
        reaches the attach path this case is about. The run counter is what
        says the sharer ran its own copy, which a marker only its own command
        wrote used to say.
        """
        self.git_tree()
        runs = self.root / "runs"
        job = f"echo start >> {runs}; echo holder-output; cat"
        holder = self.spawn(
            "ze-lint",
            "sh",
            "-c",
            job,
            name="holder",
            stdin=subprocess.PIPE,
            start_new_session=True,
        )
        self.wait_for_entry("ze-lint")
        sharer = self.spawn(
            "ze-lint", "sh", "-c", job, name="sharer", stdin=subprocess.PIPE
        )
        self.wait_for_text(self.root / "sharer.out", "holder-output")

        os.killpg(holder.pid, signal.SIGKILL)

        # The sharer's own run holds on `cat`, so closing its stdin is what
        # ends it. Reaching this point at all means it was admitted.
        self.wait_for_text(runs, "start\nstart")
        sharer.stdin.close()
        self.assertEqual(
            sharer.wait(timeout=ADMITTED_WITHIN * 2), 0, self.stderr_of(sharer)
        )
        self.assertEqual(
            runs.read_text().count("start"),
            2,
            "the sharer must run the job it could not observe",
        )
        self.assertIn(
            "without recording a result",
            self.stderr_of(sharer),
            "the sharer must say why it stopped sharing",
        )

    def test_the_tree_hash_is_taken_when_the_job_is_admitted(self):
        """A queued job records the tree it judges, not the one it asked about.

        TREE is the field a later asker attaches on. A job that waited behind a
        20 minute holder asked about a tree that has moved since, and an entry
        advertising that older hash would invite exactly the share AC-4 exists
        to refuse.
        """
        self.git_tree()
        blocker = self.spawn("blocker", "cat", stdin=subprocess.PIPE)
        self.wait_for_entry("blocker")

        waiter = self.spawn("ze-lint", "cat", stdin=subprocess.PIPE, name="waiter")
        time.sleep(QUEUED_FOR)
        self.assertIsNone(waiter.poll(), "the second job was not queued")
        self.change_the_tree()

        blocker.stdin.close()
        blocker.wait(timeout=ADMITTED_WITHIN)
        entry = self.wait_for_entry("ze-lint")
        self.assertEqual(
            self.fields(entry).get("TREE"),
            self.tree_hash(),
            "a queued job recorded a tree hash it is not judging",
        )

        waiter.stdin.close()
        waiter.wait(timeout=ADMITTED_WITHIN)

    def test_a_dead_holder_s_slot_is_reclaimed(self):
        """AC-7: an entry whose PID is dead is reaped by the next waiter."""
        pid = self.dead_pid()
        stale = self.write_entry(
            f"crashed.{pid}.job",
            f"LABEL=crashed\nPID={pid}\nPGID={pid}\nSTARTED={int(time.time())}\n"
            "TREE=deadbeef\nLOG=tmp/.ze-jobs/crashed.log\nSTATE=running\nCMD=make ze-lint\n",
        )

        marker = self.root / "reclaimed"
        proc = self.spawn("newjob", "touch", str(marker))
        self.assertEqual(proc.wait(timeout=ADMITTED_WITHIN), 0)
        self.assertTrue(marker.exists(), "the free slot was never granted")
        self.assertFalse(stale.exists(), "the dead holder's entry was not reaped")

    def test_a_slow_live_holder_is_not_killed(self):
        """AC-6: alive, past the old 1800s limit, log still growing -> untouched.

        This is the run the age rule destroyed. `make ze-precommit-verify` took
        over 20 minutes under load on 2026-08-17, so the first waiter SIGKILLed
        it at 1800 seconds and was killed in turn by the next waiter. Elapsed
        time is now evidence of nothing; the log is the evidence.
        """
        holder = self.spawn(
            "holder", "sh", "-c", "while :; do echo tick; sleep 0.5; done"
        )
        entry = self.wait_for_entry("holder")
        self.backdate_started(entry, 3600)

        marker = self.root / "waiter-ran"
        waiter = self.spawn(
            "waiter", "touch", str(marker), env=self.env(ZE_JOB_STALL_SECONDS="60")
        )
        time.sleep(QUEUED_FOR)

        self.assertIsNone(holder.poll(), "a live holder with a growing log was killed")
        self.assertTrue(entry.exists(), "the live holder's registry entry was reaped")
        self.assertIsNone(waiter.poll(), "the waiter took a live holder's slot")
        self.assertFalse(marker.exists(), "the waiter ran unadmitted")
        self.assertNotIn("breaking", self.stderr_of(waiter))

    def test_a_stalled_holder_is_broken_and_the_kill_carries_its_evidence(self):
        """A live holder that has produced nothing for the window loses its slot.

        `start_new_session` gives the holder its own process group, which is
        what the waiter signals: a job's children must die with it, and the
        wrapper's own group must never be the target.
        """
        holder = self.spawn(
            "holder", "cat", stdin=subprocess.PIPE, start_new_session=True
        )
        entry = self.wait_for_entry("holder")
        log_path = self.fields(entry)["LOG"]
        log = self.root / log_path
        self.wait_for_file(log)
        stale = time.time() - 7200
        os.utime(log, (stale, stale))

        marker = self.root / "waiter-ran"
        waiter = self.spawn(
            "waiter", "touch", str(marker), env=self.env(ZE_JOB_STALL_SECONDS="60")
        )
        self.assertEqual(
            waiter.wait(timeout=ADMITTED_WITHIN), 0, self.stderr_of(waiter)
        )
        self.assertTrue(marker.exists(), "the stalled holder's slot was never freed")
        self.assertNotEqual(
            holder.wait(timeout=ADMITTED_WITHIN), 0, "the stalled holder survived"
        )
        self.assertFalse(entry.exists(), "the broken holder's entry was left behind")

        err = self.stderr_of(waiter)
        self.assertIn("STALLED", err, "a kill must announce itself")
        self.assertIn(log_path, err, "the kill must name the file that stopped growing")
        self.assertIn(
            "stall window 60s", err, "the kill must state the window it applied"
        )
        static = re.search(r"has not grown for (\d+)s", err)
        self.assertIsNotNone(static, f"no stall evidence in: {err}")
        self.assertGreater(
            int(static.group(1)), 7000, "the evidence must be the measured silence"
        )

    def test_the_stall_window_boundary_is_enforced(self):
        """60..3600 seconds. Outside it the job is refused, never run on a guess.

        The window is enforced by killing a process group, so a value the script
        was not designed for is a configuration error rather than something to
        clamp silently. Both spellings feed the same check: `ZE_VERIFY_MAX_LOCK_AGE`
        is the older one, and `ai/rules/git-safety.md` still names it.
        """
        cases = [
            ("ZE_JOB_STALL_SECONDS", "59", 2),
            ("ZE_JOB_STALL_SECONDS", "60", 0),
            ("ZE_JOB_STALL_SECONDS", "3600", 0),
            ("ZE_JOB_STALL_SECONDS", "3601", 2),
            ("ZE_JOB_STALL_SECONDS", "not-a-number", 2),
            ("ZE_VERIFY_MAX_LOCK_AGE", "3601", 2),
        ]
        for name, value, want in cases:
            with self.subTest(env=name, window=value):
                marker = self.root / f"ran-{name}-{value}"
                proc = subprocess.run(
                    [str(ZE_RUN), "boundary", "touch", str(marker)],
                    cwd=self.root,
                    capture_output=True,
                    text=True,
                    env=self.env(**{name: value}),
                    timeout=ADMITTED_WITHIN,
                )
                self.assertEqual(proc.returncode, want, proc.stderr)
                self.assertEqual(marker.exists(), want == 0)
                if want != 0:
                    self.assertIn("60..3600", proc.stderr)

    def test_the_slot_count_is_read_from_the_environment(self):
        """ZE_RUN_SLOTS admits that many jobs at once, not one.

        The Makefile derives and exports the number. A wrapper that ignored it
        would leave every wrapped target queued behind a single slot, which is
        strictly worse than before the rollout: 105 targets serialized where
        three were before.
        """
        holder = self.spawn("holder", "cat", stdin=subprocess.PIPE)
        self.wait_for_entry("holder")

        queued = self.root / "one-slot"
        waiter = self.spawn("other", "touch", str(queued), name="default")
        time.sleep(QUEUED_FOR)
        self.assertIsNone(waiter.poll(), "the default admitted a second job")
        self.assertFalse(queued.exists())

        admitted = self.root / "two-slots"
        second = self.spawn(
            "extra",
            "touch",
            str(admitted),
            name="raised",
            env=self.env(ZE_RUN_SLOTS="2"),
        )
        self.assertEqual(second.wait(timeout=ADMITTED_WITHIN), 0)
        self.assertTrue(admitted.exists(), "the second slot was never offered")
        self.assertIsNone(holder.poll(), "the holder was displaced, not joined")

    def test_the_slot_count_boundary_is_enforced(self):
        """1..cores. Outside it the job is refused, never run on a guess."""
        cores = int(
            subprocess.run(
                ["nproc"], capture_output=True, text=True, check=True
            ).stdout.strip()
        )
        cases = [
            ("0", 2),
            ("1", 0),
            (str(cores), 0),
            (str(cores + 1), 2),
            ("all-of-them", 2),
        ]
        for value, want in cases:
            with self.subTest(slots=value):
                marker = self.root / f"ran-{value}"
                proc = subprocess.run(
                    [str(ZE_RUN), "boundary", "touch", str(marker)],
                    cwd=self.root,
                    capture_output=True,
                    text=True,
                    env=self.env(ZE_RUN_SLOTS=value),
                    timeout=ADMITTED_WITHIN,
                )
                self.assertEqual(proc.returncode, want, proc.stderr)
                self.assertEqual(marker.exists(), want == 0)
                if want != 0:
                    self.assertIn("ZE_RUN_SLOTS", proc.stderr)

    def test_a_corrupt_registry_entry_makes_the_job_queue(self):
        """Fail closed: an unreadable entry counts as occupied, never as free."""
        self.write_entry("mystery.1.job", "this is not a registry entry\n")

        marker = self.root / "unadmitted"
        proc = self.spawn("newjob", "touch", str(marker))
        time.sleep(QUEUED_FOR)
        self.assertIsNone(proc.poll(), "a corrupt entry must make the job queue")
        self.assertFalse(marker.exists(), "the job ran on an unreadable registry")

    def test_a_corrupt_entry_past_the_age_limit_is_discarded(self):
        """The registry stays bounded: an ancient unreadable entry is dropped."""
        entry = self.write_entry("mystery.1.job", "this is not a registry entry\n")
        old = time.time() - 7200
        os.utime(entry, (old, old))

        marker = self.root / "ran-after-discard"
        proc = self.spawn(
            "newjob", "touch", str(marker), env=self.env(ZE_VERIFY_MAX_LOCK_AGE="60")
        )
        self.assertEqual(proc.wait(timeout=ADMITTED_WITHIN), 0)
        self.assertTrue(marker.exists())
        self.assertFalse(entry.exists(), "the ancient entry was not discarded")

    def test_the_registry_entry_carries_the_job_s_identity(self):
        """Every field a waiter, an attacher, or an operator needs is recorded."""
        holder = self.spawn("holder", "cat", stdin=subprocess.PIPE)
        entry = self.wait_for_entry("holder")
        fields = self.fields(entry)

        self.assertEqual(fields.get("LABEL"), "holder")
        self.assertEqual(fields.get("PID"), str(holder.pid))
        self.assertEqual(fields.get("STATE"), "running")
        for key in ("PGID", "TREE", "STARTED", "LOG", "CMD", "KEY"):
            self.assertTrue(fields.get(key), f"{key} missing from the entry")
        # PARAMS is what an operator reads to see WHY two jobs did not share.
        # It is empty for a job nobody parameterised, so its presence is the
        # assertion, not its content.
        self.assertIn("PARAMS", fields, "PARAMS missing from the entry")
        self.assertTrue((self.root / fields["LOG"]).exists(), "the log path is a claim")

        holder.stdin.close()
        holder.wait(timeout=ADMITTED_WITHIN)
        self.assertFalse(entry.exists(), "the entry outlived the job")

    def test_a_label_that_is_not_a_path_component_is_refused(self):
        """The label names a file under tmp/.ze-jobs/: no escape, no surprises."""
        for label in ("../evil", "a/b", ".", "..", "", "two words", "dot.ted"):
            with self.subTest(label=label):
                proc = subprocess.run(
                    [str(ZE_RUN), label, "touch", str(self.root / "escaped")],
                    cwd=self.root,
                    capture_output=True,
                    text=True,
                    env=self.env(),
                    timeout=ADMITTED_WITHIN,
                )
                self.assertEqual(proc.returncode, 2, proc.stderr)
                self.assertFalse((self.root / "escaped").exists())
        self.assertEqual(self.entries(), [], "a refused label left a registry entry")

    def test_a_nested_job_inherits_the_parent_slot(self):
        """`make ze-lint` inside `make ze-precommit-verify` must not deadlock.

        The verify runner holds the slot and then runs the lint stage, which is
        itself wrapped. Queueing there would make the job wait for a slot it is
        already holding, and no timeout would ever release it.
        """
        holder = self.spawn("holder", "cat", stdin=subprocess.PIPE)
        parent_entry = self.wait_for_entry("holder")

        marker = self.root / "nested-ran"
        nested = self.spawn(
            "nested",
            "touch",
            str(marker),
            env=self.env(ZE_RUN_JOB=str(parent_entry)),
        )
        self.assertEqual(nested.wait(timeout=ADMITTED_WITHIN), 0)
        self.assertTrue(marker.exists(), "a nested job must run inside the slot")
        self.assertEqual(
            [p.name for p in self.entries()],
            [parent_entry.name],
            "a nested job must not take a second slot",
        )

        holder.stdin.close()
        holder.wait(timeout=ADMITTED_WITHIN)

    def test_a_wrapped_job_wraps_its_own_stages(self):
        """The real path of the test above: the job exports what a stage reads.

        `make ze-precommit-verify` runs `make ze-lint` as a stage, and both are
        wrapped. The outer job must hand the inner one its slot; setting the
        environment by hand proves the rule, not the plumbing.
        """
        marker = self.root / "stage-ran"
        outer = self.spawn("outer", "sh", "-c", f"{ZE_RUN} stage touch {marker}")
        self.assertEqual(outer.wait(timeout=ADMITTED_WITHIN), 0, self.stderr_of(outer))
        self.assertTrue(marker.exists(), "the stage never ran inside its parent's slot")

    def test_the_job_s_exit_code_is_the_wrapper_s_exit_code(self):
        """A failing stage must fail the caller: `make` reads this code."""
        proc = self.spawn("failing", "sh", "-c", "exit 7")
        self.assertEqual(proc.wait(timeout=ADMITTED_WITHIN), 7)

    def test_usage_is_refused_without_a_command(self):
        proc = subprocess.run(
            [str(ZE_RUN), "ze-lint"],
            cwd=self.root,
            capture_output=True,
            text=True,
            env=self.env(),
            timeout=ADMITTED_WITHIN,
        )
        self.assertEqual(proc.returncode, 2)
        self.assertIn("usage", proc.stderr.lower())


class VerifyLockAliasCase(unittest.TestCase):
    """verify-lock.sh keeps its name, its interface, and its callers.

    Makefile:682 and :701 and mk/test-chaos.mk:47 invoke it as
    `verify-lock.sh LABEL CMD [ARGS...]`, and ai/rules/git-safety.md tells
    readers the owner file names the holder. Generalising the mechanism must
    not move either.
    """

    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.root = Path(self._tmp.name)

    def tearDown(self) -> None:
        self._tmp.cleanup()

    def test_the_alias_runs_the_command_and_records_the_holder(self):
        alias = REPO / "scripts" / "dev" / "verify-lock.sh"
        env = dict(os.environ)
        env.pop("ZE_RUN_JOB", None)
        # The job itself reads the owner file, so the assertion sees it while
        # the holder is running rather than after it is cleared.
        proc = subprocess.run(
            [
                str(alias),
                "ze-precommit-verify",
                "sh",
                "-c",
                "cat tmp/.ze-verify.lock.owner > owner-copy",
            ],
            cwd=self.root,
            capture_output=True,
            text=True,
            env=env,
            timeout=ADMITTED_WITHIN,
        )
        self.assertEqual(proc.returncode, 0, proc.stderr)
        owner = (self.root / "owner-copy").read_text()
        for key in ("LABEL=ze-precommit-verify", "PID=", "PGID=", "STARTED=", "CMD="):
            self.assertIn(key, owner, "the owner file lost a documented field")
        # The duration history the rules tell agents to read for a timeout.
        duration = self.root / "tmp" / ".ze-verify-duration.txt"
        self.assertTrue(duration.exists(), "the duration history was not appended")
        self.assertTrue(duration.read_text().startswith("ze-precommit-verify\t"))
        # The owner file is cleared when the job ends, as it always was.
        self.assertFalse((self.root / "tmp" / ".ze-verify.lock.owner").exists())


if __name__ == "__main__":
    unittest.main()
