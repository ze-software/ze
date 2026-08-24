#!/usr/bin/env python3
"""Unit tests for the composable ci-sleep DELTA ratchet in verify_wiring_docs.py.

The ratchet used to store one absolute integer in test/.ci-sleep-baseline, so
two specs that each lowered it collided on the second merge (spec-fixit-sleeps-
cli-harness 132->129 vs spec-fixit-reject-fence-observability 132->130). The
delta form stores a column of signed integers that SUM to the ceiling: parallel
removals append distinct `-N` lines and never touch a shared integer, yet a net
rise still fails. This test pins that arithmetic and the monotonic guarantee.
"""

from __future__ import annotations

import argparse
import io
import json
import os
import subprocess
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest import mock

sys.path.insert(0, os.path.dirname(__file__))
import verify_wiring_docs
from verify_wiring_docs import (
    DECLARED_GROUPS,
    FAILURE_GROUP_PREFIX,
    FAILURE_GROUPS_COMPLETE_PREFIX,
    MAKE_TARGETS,
    PATH_BEARING_KIND,
    TARGET_ORDER,
    UNATTRIBUTABLE_KIND,
    GateFailure,
    check_ci_log_subsystem_keys,
    check_ci_sleep_justification,
    check_ci_sleep_ratchet,
    check_design_refs,
    check_known_failure_load_excuses,
    check_wiring,
    declare_failure_group,
    declare_groups_complete,
    declare_wiring_group,
    parse_sleep_baseline,
    run_check,
    run_gate,
    run_make_target,
    selected_targets,
)


def write(root: Path, rel: str, body: str) -> None:
    p = root / rel
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(body, encoding="utf-8")


def ci_with_sleeps(n: int) -> str:
    return "".join(f"time.sleep(0.1)  # deliberate\n" for _ in range(n))


class WiringForTestConventionTest(unittest.TestCase):
    def test_fortest_export_does_not_require_fake_production_reference(self):
        root = Path(tempfile.mkdtemp(prefix="wiring-fortest-"))
        self.addCleanup(lambda: __import__("shutil").rmtree(root, ignore_errors=True))
        write(
            root,
            "internal/component/example/helper.go",
            "package example\n\nfunc ProbeForTest() {}\n",
        )

        issues = check_wiring(
            root,
            ["internal/component/example/helper.go"],
            baseline_reader=lambda _path: "",
        )
        self.assertEqual(issues, [])


class ParseSleepBaselineTest(unittest.TestCase):
    def test_plain_int_backward_compatible(self):
        # A pre-existing single-integer baseline still parses to that ceiling.
        self.assertEqual(parse_sleep_baseline("125\n"), 125)

    def test_signed_int_lines_sum(self):
        # Origin plus two independent removal deltas.
        self.assertEqual(parse_sleep_baseline("125\n-1\n-1\n"), 123)

    def test_comments_and_blanks_ignored(self):
        text = "# header\n\n125\n# a removal\n-3\n"
        self.assertEqual(parse_sleep_baseline(text), 122)

    def test_positive_delta_raises_ceiling(self):
        # The explicit-approval knob: a `+N` line is how the ceiling is raised
        # (equivalent to editing the old absolute integer upward).
        self.assertEqual(parse_sleep_baseline("125\n+2\n"), 127)

    def test_no_integer_lines_is_inactive(self):
        self.assertIsNone(parse_sleep_baseline("# only comments\n"))


class SleepRatchetDeltaTest(unittest.TestCase):
    def _root(self) -> Path:
        d = tempfile.mkdtemp(prefix="sleep-ratchet-")
        self.addCleanup(lambda: __import__("shutil").rmtree(d, ignore_errors=True))
        return Path(d)

    def _run(self, root: Path) -> tuple[int, str]:
        buf = io.StringIO()
        with redirect_stdout(buf):
            rc = check_ci_sleep_ratchet(root, ["test/a.ci"])
        return rc, buf.getvalue()

    def test_sleep_ratchet_delta_composes(self):
        # Two independent removals recorded as separate `-1` lines both take
        # effect (the parser sums them): ceiling 4 - 1 - 1 = 2. Tree now holds 2
        # sleeps -> at the ceiling, passes.
        root = self._root()
        write(root, "test/a.ci", ci_with_sleeps(1))
        write(root, "test/b.ci", ci_with_sleeps(1))
        write(root, "test/.ci-sleep-baseline", "4\n-1\n-1\n")
        rc, out = self._run(root)
        self.assertEqual(rc, 0, out)

    def test_net_rise_still_fails(self):
        # Same delta ceiling (2), but the tree now holds 3 sleeps: a net rise
        # must fail even though the removals were recorded as deltas.
        root = self._root()
        write(root, "test/a.ci", ci_with_sleeps(2))
        write(root, "test/b.ci", ci_with_sleeps(1))
        write(root, "test/.ci-sleep-baseline", "4\n-1\n-1\n")
        rc, out = self._run(root)
        self.assertEqual(rc, 1, out)
        self.assertIn("ratchet FAILED", out)

    def test_net_zero_boundary_passes(self):
        # count == ceiling is the last valid value (boundary).
        root = self._root()
        write(root, "test/a.ci", ci_with_sleeps(2))
        write(root, "test/.ci-sleep-baseline", "2\n")
        rc, out = self._run(root)
        self.assertEqual(rc, 0, out)

    def test_under_ceiling_is_advisory_not_failure(self):
        # Fewer sleeps than the ceiling is fine (ratchet only fails on a rise);
        # it prints an advisory to tighten the baseline.
        root = self._root()
        write(root, "test/a.ci", ci_with_sleeps(1))
        write(root, "test/.ci-sleep-baseline", "4\n")
        rc, out = self._run(root)
        self.assertEqual(rc, 0, out)


class KnownFailureLoadExcuseTest(unittest.TestCase):
    """The gate behind ai/rules/completion.md.

    A shard blaming host load is stating a diagnosis (the test asserts on elapsed
    time) and calling it a mystery. The gate rejects the excuse, NOT the shard: a
    red whose mechanism is genuinely unknown still belongs in the directory.
    """

    def _root(self) -> Path:
        d = tempfile.mkdtemp(prefix="load-excuse-")
        self.addCleanup(lambda: __import__("shutil").rmtree(d, ignore_errors=True))
        return Path(d)

    def _run(self, root: Path, changed: list[str]) -> tuple[int, str]:
        buf = io.StringIO()
        with redirect_stdout(buf):
            rc = check_known_failure_load_excuses(root, changed)
        return rc, buf.getvalue()

    def test_load_excuse_fails_and_names_the_line(self):
        root = self._root()
        write(
            root,
            "plan/known-failures/flaky.md",
            "### suite N -- flaky\n\nIt only fails on a loaded host.\n",
        )
        rc, out = self._run(root, ["plan/known-failures/flaky.md"])
        self.assertEqual(rc, 1, out)
        self.assertIn("plan/known-failures/flaky.md:3", out)

    def test_every_banned_phrase_is_caught(self):
        # One shard per phrase, so a regex edit that drops an alternative fails
        # here rather than silently reopening that wording.
        phrases = [
            "it fails under load",
            "only on a loaded host",
            "load average was 11",
            "this test is load-sensitive",
            "all three pass in isolation",
            "it passed in isolation",
            "attributed to resource contention",
            "seen on a contended host",
        ]
        for i, phrase in enumerate(phrases):
            with self.subTest(phrase=phrase):
                root = self._root()
                rel = f"plan/known-failures/s{i}.md"
                write(root, rel, f"### red\n\n{phrase}.\n")
                rc, out = self._run(root, [rel])
                self.assertEqual(rc, 1, out)

    def test_unknown_mechanism_shard_still_allowed(self):
        # The directory is not closed: a shard that does not blame load passes.
        root = self._root()
        write(
            root,
            "plan/known-failures/mystery.md",
            "### suite N\n\nFails once in 200 runs; mechanism unknown.\n"
            "Repro: scripts/dev/stress-repro.py ...\nNext: read the producer.\n",
        )
        rc, out = self._run(root, ["plan/known-failures/mystery.md"])
        self.assertEqual(rc, 0, out)

    def test_readme_and_resolved_are_exempt(self):
        # README states the policy (so it must quote the phrases) and RESOLVED is
        # a verbatim archive that is never edited to satisfy a present-day gate.
        root = self._root()
        for name in ("README.md", "RESOLVED.md"):
            write(root, f"plan/known-failures/{name}", "fails under load\n")
        rc, out = self._run(
            root,
            ["plan/known-failures/README.md", "plan/known-failures/RESOLVED.md"],
        )
        self.assertEqual(rc, 0, out)

    def test_deleted_shard_is_not_a_violation(self):
        # Deleting a shard is the intended outcome of fixing the test, and a
        # deleted path still appears in the changed set.
        root = self._root()
        rc, out = self._run(root, ["plan/known-failures/gone.md"])
        self.assertEqual(rc, 0, out)

    def test_unrelated_changed_files_do_not_trigger(self):
        root = self._root()
        write(root, "docs/perf.md", "throughput degrades under load\n")
        rc, out = self._run(root, ["docs/perf.md"])
        self.assertEqual(rc, 0, out)
        self.assertEqual(out, "")


class RoutedTargetsExistTest(unittest.TestCase):
    """Every routed target must have a rule in Makefile or mk/.

    The router runs `make <target>` for each selected name, so a name whose rule
    was deleted turns a routed change into "No rule to make target". That state
    survived the removal of the learned-summary staleness gate: MAKE_TARGETS and
    TARGET_ORDER still named ze-learned-staleness after its recipe was gone, and
    nothing read the two lists against the build files.
    """

    def test_every_make_target_has_a_rule(self):
        repo = Path(__file__).resolve().parents[2]
        rules: set[str] = set()
        for path in [repo / "Makefile"] + sorted((repo / "mk").glob("*.mk")):
            for line in path.read_text(encoding="utf-8").splitlines():
                if not line or line[0].isspace() or line.startswith("#"):
                    continue
                name, sep, _ = line.partition(":")
                # A rule head is one bare token; skip assignments and recipes.
                if sep and name and "=" not in name and " " not in name.strip():
                    rules.add(name.strip())
        self.assertEqual(sorted(MAKE_TARGETS - rules), [])

    def test_every_make_target_is_ordered(self):
        # A target in MAKE_TARGETS but absent from TARGET_ORDER is selected and
        # then dropped by the ordering filter, which fails open silently.
        self.assertEqual(MAKE_TARGETS - set(TARGET_ORDER), set())


class DockerExecRoutingTest(unittest.TestCase):
    """Changed-file routing for the fail-open call-site ratchet.

    Wiring row: a changed test/**/*.py must select ze-functional-docker-exec-check, so
    `make ze-precommit-verify-changed` runs the gate exactly when a scenario or a lab
    could have added an unchecked read of a fail-open return value.
    """

    def _root(self) -> Path:
        d = tempfile.mkdtemp(prefix="docker-exec-routing-")
        self.addCleanup(lambda: __import__("shutil").rmtree(d, ignore_errors=True))
        return Path(d)

    def test_a_scenario_or_lab_change_selects_the_target(self):
        root = self._root()
        for path in (
            "test/interop/interop.py",
            "test/interop/scenarios/bgp-routes-from-frr/check.py",
            "test/interop-ipsec/lab.py",
        ):
            with self.subTest(path=path):
                self.assertIn(
                    "ze-functional-docker-exec-check", selected_targets(root, [path])
                )

    def test_checker_and_baseline_select_the_target(self):
        root = self._root()
        for path in (
            "scripts/dev/docker_exec_checked.py",
            "test/health/docker-exec-baseline.json",
        ):
            with self.subTest(path=path):
                self.assertIn(
                    "ze-functional-docker-exec-check", selected_targets(root, [path])
                )

    def test_a_draft_and_an_unrelated_change_do_not_select_it(self):
        root = self._root()
        for path in ("test/draft/plugin/wip.py", "test/plugin/api-peer.ci"):
            with self.subTest(path=path):
                self.assertNotIn(
                    "ze-functional-docker-exec-check", selected_targets(root, [path])
                )

    def test_target_is_runnable_and_ordered(self):
        self.assertIn("ze-functional-docker-exec-check", MAKE_TARGETS)
        self.assertIn("ze-functional-docker-exec-check", TARGET_ORDER)


class TemplRoutingTest(unittest.TestCase):
    """Changed-file routing for the templ generated-output freshness gate.

    Wiring row: a changed .templ or *_templ.go must select
    ze-templ-output-check, so `make ze-precommit-verify-changed` runs the gate exactly
    when the pair could have gone stale.
    """

    def _root(self) -> Path:
        d = tempfile.mkdtemp(prefix="templ-routing-")
        self.addCleanup(lambda: __import__("shutil").rmtree(d, ignore_errors=True))
        return Path(d)

    def test_either_side_of_the_pair_selects_the_target(self):
        root = self._root()
        for path in (
            "internal/component/web/templates/page.templ",
            "internal/component/web/templates/page_templ.go",
            "Makefile",
            # The guard that stops templ deleting a tracked *_templ.go. An edit
            # to it must run the gate it protects.
            "scripts/dev/templ_orphan_check.py",
        ):
            with self.subTest(path=path):
                self.assertIn("ze-templ-output-check", selected_targets(root, [path]))

    def test_an_unrelated_change_does_not_select_it(self):
        root = self._root()
        for path in ("internal/component/web/render.go", "docs/guide/docker.md"):
            with self.subTest(path=path):
                self.assertNotIn(
                    "ze-templ-output-check", selected_targets(root, [path])
                )

    def test_target_is_runnable_and_ordered(self):
        self.assertIn("ze-templ-output-check", MAKE_TARGETS)
        self.assertIn("ze-templ-output-check", TARGET_ORDER)


def run_capturing(call):
    """Run `call`, returning what it returned and what it printed to stdout."""
    buf = io.StringIO()
    with redirect_stdout(buf):
        value = call()
    return value, buf.getvalue()


def groups_in(text: str) -> list[dict]:
    """Every failure group declared in `text`, parsed back from its JSON."""
    prefix = FAILURE_GROUP_PREFIX + " "
    return [
        json.loads(line[len(prefix) :])
        for line in text.splitlines()
        if line.startswith(prefix)
    ]


class TestEveryWiringSubcheckDeclaresItsFiles(unittest.TestCase):
    """Every failure path of this gate declares a group, and names its files.

    The gate is a router: five checks run in-process, one reads the tree, and the
    rest are delegated make targets. Sub-spec 1 attributes a structural red by
    the files its groups name, and it charges the gate when a group names none.
    So a path that CAN name files must, or the red lands on whoever happened to
    be committing (71 of the 102 open structural debt rows are this gate), and a
    path that cannot must still declare, or its failure disappears from the
    index entirely.

    One method per non-zero exit path enumerated in
    plan/spec-verify-scope-6-wiring-docs-attribution.md, minus
    check_plugin_imports: `main` returns 0 on that branch and the
    ze-doc-wiring-check recipe (mk/check-docs.mk) never passes its flag.
    """

    def _root(self) -> Path:
        d = tempfile.mkdtemp(prefix="wiring-groups-")
        self.addCleanup(lambda: __import__("shutil").rmtree(d, ignore_errors=True))
        return Path(d)

    def test_ci_sleep_ratchet_declares_a_group_that_names_no_file(self):
        # The ratchet compares a tree-wide count against one ceiling, so no file
        # is the offender. It must still declare, and with a kind the commit
        # helper refuses to read paths from, so the red is charged.
        root = self._root()
        write(root, "test/.ci-sleep-baseline", "0\n")
        write(root, "test/a.ci", ci_with_sleeps(1))
        rc, out = run_capturing(lambda: check_ci_sleep_ratchet(root, ["test/a.ci"]))
        self.assertEqual(rc, 1)
        groups = groups_in(out)
        self.assertEqual(len(groups), 1)
        self.assertEqual(groups[0]["group-id"], "subcheck:ci-sleep-ratchet")
        self.assertEqual(groups[0]["kind"], UNATTRIBUTABLE_KIND)
        self.assertEqual(groups[0]["related"], [])

    def test_ci_sleep_justification_names_the_changed_file(self):
        root = self._root()
        write(root, "test/a.ci", "time.sleep(0.1)\n")
        rc, out = run_capturing(
            lambda: check_ci_sleep_justification(root, ["test/a.ci"])
        )
        self.assertEqual(rc, 1)
        groups = groups_in(out)
        self.assertEqual(len(groups), 1)
        self.assertEqual(groups[0]["kind"], PATH_BEARING_KIND)
        self.assertEqual(groups[0]["related"], ["test/a.ci"])

    def test_known_failure_load_excuse_names_the_shard(self):
        root = self._root()
        write(root, "plan/known-failures/flaky.md", "It fails under load.\n")
        rc, out = run_capturing(
            lambda: check_known_failure_load_excuses(
                root, ["plan/known-failures/flaky.md"]
            )
        )
        self.assertEqual(rc, 1)
        groups = groups_in(out)
        self.assertEqual(len(groups), 1)
        self.assertEqual(groups[0]["kind"], PATH_BEARING_KIND)
        self.assertEqual(groups[0]["related"], ["plan/known-failures/flaky.md"])

    def test_ci_log_subsystem_key_names_the_ci_file(self):
        root = self._root()
        write(root, "test/a.ci", "env ze.log.bgp.adj-rib-in=debug\n")
        rc, out = run_capturing(
            lambda: check_ci_log_subsystem_keys(root, ["test/a.ci"])
        )
        self.assertEqual(rc, 1)
        groups = groups_in(out)
        self.assertEqual(len(groups), 1)
        self.assertEqual(groups[0]["kind"], PATH_BEARING_KIND)
        self.assertEqual(groups[0]["related"], ["test/a.ci"])

    def _stub_checker(self, root: Path, body: str) -> None:
        """A stand-in for check_doc_links.py, run as the real child is run."""
        write(root, "scripts/dev/check_doc_links.py", body)

    def test_design_refs_names_the_paths_its_child_reported(self):
        # The finding lives one process down: this check runs check_doc_links.py.
        # Before this spec it passed no capture_output and held only the return
        # code, so the gate's likeliest tree-wide red named no file at all.
        root = self._root()
        write(root, "internal/component/ssh/server.go", "package ssh\n")
        self._stub_checker(
            root,
            "import sys\n"
            "print('internal/component/ssh/server.go:12: broken Design reference: docs/gone.md')\n"
            "sys.exit(1)\n",
        )
        rc, out = run_capturing(lambda: check_design_refs(root))
        self.assertEqual(rc, 1)
        self.assertIn("broken Design reference", out)  # the child is re-printed
        groups = groups_in(out)
        self.assertEqual(len(groups), 1)
        self.assertEqual(groups[0]["kind"], PATH_BEARING_KIND)
        self.assertEqual(groups[0]["related"], ["internal/component/ssh/server.go"])

    def test_design_refs_naming_nothing_the_checkout_holds_declares_no_path(self):
        # The referencing file is what earns the red, and a token no checkout
        # holds is not one. Naming it anyway would make the group look
        # attributable, and the commit helper would drop a red it should charge.
        root = self._root()
        self._stub_checker(
            root,
            "import sys\n"
            "print('internal/gone/away.go:12: broken Design reference: docs/gone.md')\n"
            "print('something went wrong')\n"
            "sys.exit(1)\n",
        )
        rc, out = run_capturing(lambda: check_design_refs(root))
        self.assertEqual(rc, 1)
        groups = groups_in(out)
        self.assertEqual(len(groups), 1)
        self.assertEqual(groups[0]["kind"], UNATTRIBUTABLE_KIND)
        self.assertEqual(groups[0]["related"], [])

    def test_a_child_naming_a_path_outside_the_checkout_names_nothing(self):
        # An absolute or escaping token is not a path this commit can be
        # compared against, and related_repo_path refuses both downstream. The
        # producer refuses them too, so the group says "no file" rather than
        # naming one nobody can attribute.
        root = self._root()
        self._stub_checker(
            root,
            "import sys\n"
            "print('/etc/passwd:1: broken Design reference: docs/gone.md')\n"
            "print('../outside/x.go:2: broken Design reference: docs/gone.md')\n"
            "sys.exit(1)\n",
        )
        _, out = run_capturing(lambda: check_design_refs(root))
        groups = groups_in(out)
        self.assertEqual(len(groups), 1)
        self.assertEqual(groups[0]["related"], [])
        self.assertEqual(groups[0]["kind"], UNATTRIBUTABLE_KIND)

    def test_the_wiring_group_names_the_file_check_wiring_reported(self):
        # Holds two formats together: the `{sym.path}:{sym.line}:` prefix
        # check_wiring writes, and the prefix child_finding_paths reads back.
        root = self._root()
        write(
            root,
            "internal/component/example/helper.go",
            "package example\n\nfunc Probe() {}\n",
        )
        issues = check_wiring(
            root,
            ["internal/component/example/helper.go"],
            baseline_reader=lambda _path: "",
        )
        self.assertEqual(len(issues), 1)
        _, out = run_capturing(lambda: declare_wiring_group(root, issues))
        groups = groups_in(out)
        self.assertEqual(len(groups), 1)
        self.assertEqual(groups[0]["kind"], PATH_BEARING_KIND)
        self.assertEqual(groups[0]["related"], ["internal/component/example/helper.go"])

    def test_a_delegated_target_declares_a_group_naming_no_file(self):
        # Every path in a delegated target's output was produced by another
        # program. The group names the target so the red is charged, and names
        # no file so it is never attributed to somebody on a guess.
        root = self._root()

        def failing_target():
            with self.assertRaises(GateFailure):
                run_make_target("false", "ze-doc-verify", root)

        _, out = run_capturing(failing_target)
        groups = groups_in(out)
        self.assertEqual(len(groups), 1)
        self.assertEqual(groups[0]["group-id"], "subcheck:ze-doc-verify")
        self.assertEqual(groups[0]["kind"], UNATTRIBUTABLE_KIND)
        self.assertEqual(groups[0]["related"], [])
        self.assertEqual(groups[0]["rerun"], "make ze-doc-verify")


class DeclaredGroupProtocolTest(unittest.TestCase):
    """The line this gate writes, as the verify runner has to read it back."""

    def test_a_pathological_path_cannot_forge_a_second_group(self):
        # A file name may hold any byte but the separator, the prefix included.
        # json.dumps escapes the quote and the newline into the string they
        # belong to, so the group stays one line and carries the path back whole.
        evil = 'test/we"ird\n' + FAILURE_GROUP_PREFIX + ' {"kind":"forged"}\n.ci'
        _, out = run_capturing(
            lambda: declare_failure_group("ci-sleep-justification", [evil], "s", "r")
        )
        self.assertEqual(len(out.splitlines()), 1)
        groups = groups_in(out)
        self.assertEqual(len(groups), 1)
        self.assertEqual(groups[0]["related"], [evil])
        self.assertEqual(groups[0]["kind"], PATH_BEARING_KIND)

    def test_a_long_file_list_is_split_into_sibling_groups(self):
        # A line above maxLogLineBytes (4 MiB, scripts/status/verify_run.go) ENDS
        # the read: splitLines marks the log truncated and every line after the
        # long one goes unclassified. So a tree-wide check naming hundreds of
        # files must not print one enormous line. Nothing is dropped: the
        # siblings carry the rest, and chunking keeps each line far under the
        # limit rather than near it.
        paths = [f"internal/component/example/file{n:04d}.go" for n in range(120)]
        _, out = run_capturing(lambda: declare_failure_group("wiring", paths, "s", "r"))
        groups = groups_in(out)
        self.assertEqual(len(groups), 3)
        self.assertEqual([g["group-id"] for g in groups][0], "files:wiring")
        self.assertEqual(sorted(p for g in groups for p in g["related"]), sorted(paths))
        self.assertEqual(len({g["group-id"] for g in groups}), 3)
        for line in out.splitlines():
            self.assertLess(len(line), 4 * 1024 * 1024)

    def test_the_completeness_line_counts_every_group_declared(self):
        before = len(DECLARED_GROUPS)
        _, out = run_capturing(
            lambda: (
                declare_failure_group("ci-sleep-ratchet", [], "s", "r"),
                declare_failure_group("wiring", ["Makefile"], "s", "r"),
                declare_groups_complete(),
            )
        )
        self.assertEqual(len(DECLARED_GROUPS) - before, 2)
        self.assertIn(
            f"{FAILURE_GROUPS_COMPLETE_PREFIX} {len(DECLARED_GROUPS)}",
            out.splitlines(),
        )

    def test_a_failing_run_declares_its_groups_and_states_the_count(self):
        # The end of the wire this gate owns: the script itself, run the way the
        # recipe runs it, on a tree whose only fault is an unjustified sleep.
        root = Path(tempfile.mkdtemp(prefix="wiring-run-"))
        self.addCleanup(lambda: __import__("shutil").rmtree(root, ignore_errors=True))
        write(root, "go.mod", "module example\n")
        write(root, "test/a.ci", "time.sleep(0.1)\n")
        proc = subprocess.run(
            [
                sys.executable,
                os.path.join(os.path.dirname(__file__), "verify_wiring_docs.py"),
                "--root",
                str(root),
                "--changed-file",
                "test/a.ci",
            ],
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(proc.returncode, 1, proc.stdout + proc.stderr)
        groups = groups_in(proc.stdout)
        self.assertEqual([g["related"] for g in groups], [["test/a.ci"]])
        self.assertIn(
            f"{FAILURE_GROUPS_COMPLETE_PREFIX} {len(groups)}",
            proc.stdout.splitlines(),
        )

    def test_the_early_return_still_states_its_count(self):
        # A wiring failure returns from `main` before the delegated targets run.
        # Its group must be declared on that path too, and the count must still
        # be stated, or the runner falls back to the prose classifier and the
        # gate stays unattributable exactly where it names files best.
        root = Path(tempfile.mkdtemp(prefix="wiring-early-"))
        self.addCleanup(lambda: __import__("shutil").rmtree(root, ignore_errors=True))
        write(root, "go.mod", "module example\n")
        write(
            root,
            "internal/component/example/helper.go",
            "package example\n\nfunc Probe() {}\n",
        )
        proc = subprocess.run(
            [
                sys.executable,
                os.path.join(os.path.dirname(__file__), "verify_wiring_docs.py"),
                "--root",
                str(root),
                "--changed-file",
                "internal/component/example/helper.go",
            ],
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(proc.returncode, 1, proc.stdout + proc.stderr)
        groups = groups_in(proc.stdout)
        self.assertEqual(
            [g["related"] for g in groups],
            [["internal/component/example/helper.go"]],
        )
        self.assertIn(
            f"{FAILURE_GROUPS_COMPLETE_PREFIX} {len(groups)}",
            proc.stdout.splitlines(),
        )

    def test_a_green_run_declares_nothing(self):
        # Nothing failed, so there is no group to declare and no count to state.
        # A count on a green run would be noise in every stage log the runner
        # never classifies.
        root = Path(tempfile.mkdtemp(prefix="wiring-green-"))
        self.addCleanup(lambda: __import__("shutil").rmtree(root, ignore_errors=True))
        write(root, "go.mod", "module example\n")
        write(root, "test/a.ci", "# nothing to see\n")
        proc = subprocess.run(
            [
                sys.executable,
                os.path.join(os.path.dirname(__file__), "verify_wiring_docs.py"),
                "--root",
                str(root),
                "--changed-file",
                "test/a.ci",
            ],
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
        self.assertNotIn(FAILURE_GROUPS_COMPLETE_PREFIX, proc.stdout)
        self.assertEqual(groups_in(proc.stdout), [])


class ASilentFailureIsChargedTest(unittest.TestCase):
    """A check that fails without declaring is charged, not dropped.

    The completeness count cannot see this failure. `declare_groups_complete`
    prints `len(DECLARED_GROUPS)`, the number of declarations that were MADE, so
    a check that fails and declares nothing publishes a count agreeing with
    itself: `parseDeclaredGroups` (scripts/status/verify_run.go) reports
    complete, the runner prefers a group set that says nothing about that
    failure, and the red leaves the failure index (R-1).

    `run_check` closes it at the producer, so "declared nothing" becomes
    "declared one no-file group" and `structural_gate_reds`
    (scripts/dev/commit_helper.py) charges it. The count is then honest by
    construction rather than by review of every check.
    """

    def test_a_failing_check_that_declares_nothing_is_charged(self):
        _, out = run_capturing(
            lambda: run_check("silent-check", "make ze-doc-wiring-check", lambda: 1)
        )
        groups = groups_in(out)
        self.assertEqual(len(groups), 1)
        self.assertEqual(groups[0]["group-id"], f"{UNATTRIBUTABLE_KIND}:silent-check")
        self.assertEqual(groups[0]["kind"], UNATTRIBUTABLE_KIND)
        self.assertEqual(groups[0]["related"], [])
        self.assertEqual(groups[0]["rerun"], "make ze-doc-wiring-check")

    def test_a_check_that_raises_past_the_rest_is_charged(self):
        # run_make_target raises GateFailure for an unknown target before it has
        # declared anything, and the raise skips every target after it.
        def raiser():
            raise GateFailure("silent-check exploded")

        def call():
            with self.assertRaises(GateFailure):
                run_check("silent-check", "make ze-doc-wiring-check", raiser)

        _, out = run_capturing(call)
        groups = groups_in(out)
        self.assertEqual(len(groups), 1)
        self.assertEqual(groups[0]["kind"], UNATTRIBUTABLE_KIND)

    def test_a_check_that_declared_its_own_group_gets_no_second_one(self):
        def declaring():
            declare_failure_group("wiring", ["Makefile"], "s", "r")
            return 1

        _, out = run_capturing(
            lambda: run_check("wiring", "make ze-doc-wiring-check", declaring)
        )
        groups = groups_in(out)
        self.assertEqual(len(groups), 1)
        self.assertEqual(groups[0]["kind"], PATH_BEARING_KIND)
        self.assertEqual(groups[0]["related"], ["Makefile"])

    def test_a_passing_check_declares_nothing(self):
        _, out = run_capturing(
            lambda: run_check("quiet-check", "make ze-doc-wiring-check", lambda: 0)
        )
        self.assertEqual(groups_in(out), [])

    def test_the_dispatch_charges_a_check_that_fails_in_silence(self):
        # The regression guard for the dispatch itself, not for one check. The
        # eight methods of TestEveryWiringSubcheckDeclaresItsFiles are hand
        # written per check; this drives `run_gate`, which is what routes them,
        # with a check that fails and prints nothing at all.
        root = Path(tempfile.mkdtemp(prefix="wiring-silent-"))
        self.addCleanup(lambda: __import__("shutil").rmtree(root, ignore_errors=True))
        write(root, "go.mod", "module example\n")
        args = argparse.Namespace(
            changed_file=["test/a.ci"], dry_run=False, make="false"
        )
        with mock.patch.object(
            verify_wiring_docs,
            "check_ci_log_subsystem_keys",
            lambda _root, _changed: 1,
        ):
            rc, out = run_capturing(lambda: run_gate(root, args))
        self.assertEqual(rc, 1, out)
        groups = groups_in(out)
        self.assertEqual(len(groups), 1, out)
        self.assertEqual(
            groups[0]["group-id"], f"{UNATTRIBUTABLE_KIND}:ci-log-subsystem-key"
        )
        self.assertEqual(groups[0]["related"], [])


if __name__ == "__main__":
    unittest.main()
