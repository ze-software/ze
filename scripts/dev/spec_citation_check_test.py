#!/usr/bin/env python3
"""Unit tests for spec-citation-check.py (the spec citation freshness gate).

Driven end-to-end through the real entry point (subprocess) per the guard test
corollary in ai/rules/fail-closed-guards.md: the exit code IS the gate, so the
gate -- not a helper -- is what gets asserted. Each fixture builds a throwaway
repo under a tempdir with its own plan/ tree and (optionally) a baseline, so a
test never depends on the real repo's rot.

Coverage:
  AC-1  test_citation_dangling_spec_fails       spec -> absent spec, no baseline
        test_citation_baselined_passes          same, but the target is baselined
        test_removing_baselined_ref_is_fine     baseline entry no longer cited
        test_learned_reference_not_fatal        learned -> closed spec is expected
  AC-2  test_citation_line_drift_warns          quoted token no longer on the line
        test_citation_line_token_present_no_warn token still present -> silent
"""

from __future__ import annotations

import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
SCRIPT = HERE / "spec-citation-check.py"

SPEC_HEAD = (
    "# Spec: {name}\n\n| Field | Value |\n|-------|-------|\n| Status | design |\n\n"
)


def run(repo: Path) -> subprocess.CompletedProcess:
    return subprocess.run(
        [sys.executable, str(SCRIPT), "--repo", str(repo)],
        capture_output=True,
        text=True,
    )


def write(repo: Path, rel: str, body: str) -> None:
    p = repo / rel
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(body, encoding="utf-8")


class CitationGateTest(unittest.TestCase):
    def _repo(self) -> Path:
        d = tempfile.mkdtemp(prefix="citation-")
        self.addCleanup(lambda: shutil.rmtree(d, ignore_errors=True))
        repo = Path(d)
        (repo / "plan").mkdir()
        return repo

    # --- AC-1: dangling spec reference is fatal unless baselined -------------

    def test_citation_dangling_spec_fails(self):
        repo = self._repo()
        write(
            repo,
            "plan/spec-a.md",
            SPEC_HEAD.format(name="a")
            + "This depends on `plan/spec-gone.md` for the wire format.\n",
        )
        r = run(repo)
        out = r.stdout + r.stderr
        self.assertNotEqual(r.returncode, 0, out)
        self.assertIn("plan/spec-a.md", out)  # citing spec named
        self.assertIn("plan/spec-gone.md", out)  # dangling ref named
        self.assertRegex(out, r"plan/spec-a\.md:\d+")  # citing line named

    def test_citation_baselined_passes(self):
        repo = self._repo()
        write(
            repo,
            "plan/spec-a.md",
            SPEC_HEAD.format(name="a") + "Superseded `plan/spec-gone.md`.\n",
        )
        write(
            repo,
            "plan/.citation-baseline",
            "# known-dangling spec targets (allow-list)\nplan/spec-gone.md\n",
        )
        r = run(repo)
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)

    def test_removing_baselined_ref_is_fine(self):
        # A baseline entry that nothing references anymore must not fail the gate:
        # cleaning up a dangling reference is always allowed.
        repo = self._repo()
        write(repo, "plan/spec-a.md", SPEC_HEAD.format(name="a") + "No refs here.\n")
        write(repo, "plan/.citation-baseline", "plan/spec-gone.md\n")
        r = run(repo)
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)

    def test_learned_reference_not_fatal(self):
        # A learned summary referencing its now-closed spec is the expected result
        # of the two-commit closure model (spec git-rm'd, learned kept), so the
        # FAIL pass must be scoped to spec -> spec and must NOT fire here.
        repo = self._repo()
        (repo / "plan" / "learned").mkdir()
        write(
            repo,
            "plan/learned/1000-thing.md",
            "# 1000 thing\n\nImplements `plan/spec-thing.md` (now closed).\n",
        )
        r = run(repo)
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)

    # --- AC-2: line-token drift is a non-fatal WARN -------------------------

    def test_citation_line_drift_warns(self):
        repo = self._repo()
        write(repo, "src/foo.go", "package foo\nnewToken := 1\n")
        write(
            repo,
            "plan/spec-a.md",
            SPEC_HEAD.format(name="a")
            + "The guard `oldToken` lives at `src/foo.go:2`.\n",
        )
        r = run(repo)
        out = r.stdout + r.stderr
        self.assertEqual(r.returncode, 0, out)  # WARN is non-fatal
        self.assertIn("WARN", out)
        self.assertIn("src/foo.go:2", out)  # citation named
        self.assertIn("oldToken", out)  # missing token named

    def test_citation_line_token_present_no_warn(self):
        repo = self._repo()
        write(repo, "src/foo.go", "package foo\nnewToken := 1\n")
        write(
            repo,
            "plan/spec-a.md",
            SPEC_HEAD.format(name="a")
            + "The guard `newToken` lives at `src/foo.go:2`.\n",
        )
        r = run(repo)
        out = r.stdout + r.stderr
        self.assertEqual(r.returncode, 0, out)
        self.assertNotIn("WARN", out)


if __name__ == "__main__":
    unittest.main()
