#!/usr/bin/env python3
"""Unit tests for docker_exec_checked.

Collected by TestPythonUnitTests (scripts/dev/python_tests_test.go).

Three layers, on purpose:

- the helpers, driven with in-memory fixtures, one property each;
- the ENTRY POINT, driven as a subprocess against a synthetic tree, because a
  gate is only real if its command-line exit code is real (ai/rules/evidence.md);
- the live ratchet, run over the real repository, so `make ze-unit-test`
  enforces the floor as well as `make ze-docker-exec-check` does.
"""

import json
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import docker_exec_checked as chk

TOOL = Path(chk.__file__).resolve()


def members_of(**sources: str) -> set[str]:
    """The fail-open set derived from named in-memory sources."""
    return chk.fail_open_functions(
        {name + ".py": chk.parse(name + ".py", text) for name, text in sources.items()}
    )


def verdicts(source: str, members: set[str]) -> list[tuple[int, str, str]]:
    """(line, member, verdict) for every call site in one source."""
    sites = chk.classify("f.py", source, chk.parse("f.py", source), members)
    return [(s.line, s.member, s.verdict) for s in sites]


class TestFailOpenSet(unittest.TestCase):
    """The set is DERIVED to a fixpoint, never hand-listed."""

    def test_a_wrapper_that_returns_the_seed_joins_the_set(self):
        """The chain is written OUTERMOST FIRST, on purpose.

        A single forward pass over the definitions resolves a chain that happens
        to be written seed-first, so an implementation with no fixpoint loop
        passes such a fixture and fails on the real tree, where a wrapper is
        routinely parsed before the function it wraps. Reversing the order is
        what makes this test discriminate.
        """
        names = members_of(
            interop="""
class FRR:
    def route_table(self):
        return self._vtysh_quiet("show ip route")

    def _vtysh_quiet(self, command):
        return docker_exec_quiet(self.container, ["vtysh", "-c", command])

def parse_lines(text):
    return text.splitlines()

def docker_exec_quiet(container, cmd):
    return ""
"""
        )

        self.assertIn("docker_exec_quiet", names, "the seed is always a member")
        self.assertIn("_vtysh_quiet", names, "one hop out")
        self.assertIn("route_table", names, "two hops out: the fixpoint iterates")
        self.assertNotIn("parse_lines", names, "a pure helper must not be dragged in")

    def test_the_set_spans_files(self):
        """A scenario wrapper in another file is still a member."""
        names = members_of(
            interop="""
def docker_exec_quiet(container, cmd):
    return ""
""",
            lab="""
def _swanctl(container, args):
    return docker_exec_quiet(container, ["swanctl"] + args)
""",
        )
        self.assertIn("_swanctl", names)

    def test_binding_then_returning_is_not_a_member(self):
        """Only a direct `return <call>` propagates: the binder owes the check."""
        names = members_of(
            interop="""
def docker_exec_quiet(container, cmd):
    return ""

def summarize(container):
    out = docker_exec_quiet(container, ["show"])
    return out
"""
        )
        self.assertNotIn("summarize", names)


MEMBERS = {"docker_exec_quiet", "_vtysh_quiet"}


class TestClassify(unittest.TestCase):
    """Each verdict, one property, and the polarity between them."""

    def test_bound_and_tested_for_emptiness_is_checked(self):
        source = """
def route(self, prefix):
    output = self._vtysh_quiet("show bgp %s json" % prefix)
    if not output.strip():
        return None
    return json.loads(output)
"""
        self.assertEqual(verdicts(source, MEMBERS), [(3, "_vtysh_quiet", "checked")])

    def test_an_earlier_guard_does_not_cover_a_later_call(self):
        """A guard protects the call ABOVE it, never the one below.

        This is `FRR.route_count` (test/interop/interop.py): the JSON call is
        guarded, the text fallback is not, and `splitlines()` on "" yields no
        lines so the function answers 0 prefixes for a vtysh that FAILED. That is
        guard 3's `Ze.rib_count` shape surviving inside guard 4. Before the
        positional rule, the second call rode out on the first call's guard and
        three live sites were reported checked.
        """
        source = """
def route_count(self, family):
    output = self._vtysh_quiet("show bgp %s json" % family)
    if not output:
        return 0
    output = self._vtysh_quiet("show bgp %s summary" % family)
    return len(output.splitlines())
"""
        self.assertEqual(
            verdicts(source, MEMBERS),
            [(3, "_vtysh_quiet", "checked"), (6, "_vtysh_quiet", "unchecked")],
        )

    def test_a_test_written_above_the_call_does_not_count(self):
        """Textual position, not mere presence of the name in some test."""
        source = """
def probe(self):
    if not out:
        return None
    out = self._vtysh_quiet("show isis neighbor")
    return "Up" in out
"""
        self.assertEqual(verdicts(source, MEMBERS), [(5, "_vtysh_quiet", "unchecked")])

    def test_bound_and_never_tested_is_unchecked(self):
        """The worked true positive: FRR.is_dis in test/interop/interop.py."""
        source = """
def is_dis(self):
    out = self._vtysh_quiet("show isis interface detail")
    return "DIS" in out or "Designated" in out
"""
        self.assertEqual(verdicts(source, MEMBERS), [(3, "_vtysh_quiet", "unchecked")])

    def test_a_membership_test_is_not_an_emptiness_test(self):
        """`if "x" in out` is False on "", which IS the fail-open shape."""
        source = """
def has_route(self, prefix):
    out = self._vtysh_quiet("show ip route")
    if prefix in out:
        return True
    return False
"""
        self.assertEqual(verdicts(source, MEMBERS), [(3, "_vtysh_quiet", "unchecked")])

    def test_inline_use_is_unchecked(self):
        source = """
def dump(self):
    print(self._vtysh_quiet("show isis neighbor")[:500])
    payload = json.loads(docker_exec_quiet("c", ["show", "-j"]))
    return payload
"""
        self.assertEqual(
            [v for _, _, v in verdicts(source, MEMBERS)], ["unchecked", "unchecked"]
        )

    def test_a_bare_statement_call_is_discarded_not_flagged(self):
        source = """
def warm(self):
    self._vtysh_quiet("show version")
"""
        self.assertEqual(verdicts(source, MEMBERS), [(3, "_vtysh_quiet", "discarded")])

    def test_the_functions_own_return_is_checked(self):
        """The obligation moves to the callers, who are call sites themselves."""
        source = """
def _vtysh_quiet(self, command):
    return docker_exec_quiet(self.container, ["vtysh", "-c", command])
"""
        self.assertEqual(
            verdicts(source, MEMBERS), [(3, "docker_exec_quiet", "checked")]
        )

    def test_emptiness_test_shapes_all_count(self):
        for guard in (
            "if not out:\n        return None",
            "if out:\n        return out",
            'if out == "":\n        return None',
            "if len(out) == 0:\n        return None",
            "assert out, 'empty'",
            "while not out:\n        out = 'x'",
        ):
            with self.subTest(guard=guard.splitlines()[0]):
                source = f"""
def probe(self):
    out = self._vtysh_quiet("show version")
    {guard}
    return out
"""
                self.assertEqual(
                    [v for _, _, v in verdicts(source, MEMBERS)],
                    ["checked"],
                    guard,
                )

    def test_opt_out_with_a_reason_silences_the_site(self):
        above = """
def dump(self):
    # fail-open-ok: diagnostic print on an already-failed run
    print(self._vtysh_quiet("show isis neighbor")[:500])
"""
        trailing = """
def dump(self):
    print(self._vtysh_quiet("show isis neighbor")[:500])  # fail-open-ok: diagnostic
"""
        for name, source in (("above", above), ("trailing", trailing)):
            with self.subTest(placement=name):
                self.assertEqual(
                    [v for _, _, v in verdicts(source, MEMBERS)], ["exempt"]
                )

    def test_a_bare_opt_out_with_no_reason_does_not_silence_it(self):
        source = """
def dump(self):
    # fail-open-ok:
    print(self._vtysh_quiet("show isis neighbor")[:500])
"""
        self.assertEqual([v for _, _, v in verdicts(source, MEMBERS)], ["unchecked"])

    def test_a_non_member_call_is_never_a_site(self):
        source = """
def dump(self):
    out = docker_exec("c", ["true"])
    return "x" in out
"""
        self.assertEqual(verdicts(source, MEMBERS), [])


def write_tree(root: Path, baseline: int, extra: str = "") -> None:
    """A minimal repo the entry point can be pointed at."""
    (root / "test" / "interop").mkdir(parents=True, exist_ok=True)
    (root / "test" / "health").mkdir(parents=True, exist_ok=True)
    (root / "test" / "interop" / "interop.py").write_text(
        """
def docker_exec_quiet(container, cmd):
    return ""


def _vtysh_quiet(container, command):
    return docker_exec_quiet(container, ["vtysh", "-c", command])


def established(container):
    out = _vtysh_quiet(container, "show bgp summary")
    return "Established" in out
"""
        + extra
    )
    (root / "test" / "health" / chk.BASELINE_NAME).write_text(
        json.dumps({chk.BASELINE_KEY: baseline}) + "\n"
    )


def run_tool(root: Path, *args: str) -> subprocess.CompletedProcess:
    return subprocess.run(
        [sys.executable, str(TOOL), "--root", str(root), *args],
        capture_output=True,
        text=True,
        timeout=120,
        check=False,
    )


class TestEntryPoint(unittest.TestCase):
    """Drive the CLI, not the helpers: the exit code is the gate."""

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="docker-exec-check-")).resolve()
        self.addCleanup(shutil.rmtree, self.tmp, ignore_errors=True)

    def test_at_the_baseline_it_exits_zero(self):
        write_tree(self.tmp, baseline=1)
        proc = run_tool(self.tmp)
        self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
        self.assertIn("docker-exec-check: OK", proc.stdout)

    def test_a_rise_over_the_baseline_fails_and_names_the_site(self):
        write_tree(
            self.tmp,
            baseline=1,
            extra="""

def newly_added(container):
    out = _vtysh_quiet(container, "show isis neighbor")
    return "Up" in out
""",
        )
        proc = run_tool(self.tmp)
        self.assertEqual(proc.returncode, 1, proc.stdout + proc.stderr)
        self.assertIn("docker-exec-check: FAIL", proc.stderr)
        self.assertIn("test/interop/interop.py", proc.stderr)
        self.assertIn("newly_added", proc.stderr)

    def test_a_fall_below_the_baseline_passes_and_asks_for_the_tighten(self):
        write_tree(self.tmp, baseline=5)
        proc = run_tool(self.tmp)
        self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
        self.assertIn("lower the baseline", proc.stdout)

    def test_the_opt_out_lowers_the_count_through_the_cli(self):
        write_tree(
            self.tmp,
            baseline=1,
            extra="""

def diagnose(container):
    # fail-open-ok: diagnostic print on an already-failed run
    print(_vtysh_quiet(container, "show isis neighbor")[:500])
""",
        )
        proc = run_tool(self.tmp)
        self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)

    def test_a_missing_baseline_fails_closed(self):
        write_tree(self.tmp, baseline=1)
        (self.tmp / "test" / "health" / chk.BASELINE_NAME).unlink()
        proc = run_tool(self.tmp)
        self.assertEqual(proc.returncode, 1, proc.stdout + proc.stderr)
        self.assertIn("does not exist", proc.stderr)

    def test_an_unparseable_python_file_fails_closed(self):
        write_tree(self.tmp, baseline=1)
        (self.tmp / "test" / "interop" / "broken.py").write_text("def (:\n")
        proc = run_tool(self.tmp)
        self.assertEqual(proc.returncode, 1, proc.stdout + proc.stderr)
        self.assertIn("broken.py", proc.stderr)

    def test_a_non_numeric_baseline_fails_closed(self):
        """A floor that cannot be read must never be read as no floor."""
        write_tree(self.tmp, baseline=1)
        (self.tmp / "test" / "health" / chk.BASELINE_NAME).write_text(
            json.dumps({chk.BASELINE_KEY: "many"})
        )
        with self.assertRaises(chk.CheckError):
            chk.read_baseline(self.tmp)

    def test_json_mode_publishes_the_derivation_and_the_sites(self):
        write_tree(self.tmp, baseline=1)
        proc = run_tool(self.tmp, "--json")
        self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
        report = json.loads(proc.stdout)
        self.assertEqual(report["seed"], chk.SEED)
        self.assertIn("_vtysh_quiet", report["fail-open-functions"])
        self.assertEqual(report["counts"]["unchecked"], 1)
        self.assertEqual(report["baseline"], 1)

    def test_selftest_passes(self):
        proc = subprocess.run(
            [sys.executable, str(TOOL), "--selftest"],
            capture_output=True,
            text=True,
            timeout=120,
            check=False,
        )
        self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
        self.assertIn("selftest: OK", proc.stdout)


class TestRepoRatchet(unittest.TestCase):
    """THE floor, re-measured over the real tree on every unit-test run."""

    def test_the_real_tree_is_at_or_under_the_committed_baseline(self):
        root = chk.repo_root()
        report = chk.scan(root)

        # A vacuous pass here would be indistinguishable from a clean one.
        self.assertGreater(
            len(report.members), 15, f"the fixpoint collapsed: {sorted(report.members)}"
        )
        for expected in ("docker_exec_quiet", "_vtysh_quiet", "_swanctl", "vtysh"):
            self.assertIn(expected, report.members)
        self.assertGreater(len(report.sites), 100, "call sites stopped being found")

        baseline = chk.read_baseline(root)
        self.assertLessEqual(
            report.counts["unchecked"],
            baseline,
            "a new unchecked fail-open call site landed; check the return value or "
            "mark it `# fail-open-ok: <reason>`. New sites:\n  "
            + "\n  ".join(
                f"{s.file}:{s.line} {s.function} -> {s.member}()"
                for s in report.unchecked()[:20]
            ),
        )


if __name__ == "__main__":
    unittest.main()
