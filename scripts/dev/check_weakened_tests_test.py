#!/usr/bin/env python3
"""Tests for check_weakened_tests.py.

The checker decides whether `test/weakened.md` accepts the weakenings a commit
carries. Two failures cost the same thing and are opposite: accepting a
weakening nobody wrote a row for, and refusing a commit whose rows are correct.
So every case here drives the pair, never one side alone.

The fixtures build a real git repository, because the checker reads HEAD through
git and judges the paths the commit NAMES. A fixture that handed it two strings
would not exercise that. The durable contract says to judge the commit's named
paths against HEAD, never the working tree
(`docs/architecture/testing/test-health.md`, "The per-commit weakening record").
"""

from __future__ import annotations

import os
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(__file__))
import check_weakened_tests as cw  # noqa: E402

GIT_ENV = [
    "-c",
    "user.email=t@t",
    "-c",
    "user.name=t",
    "-c",
    "commit.gpgsign=false",
]

# A healthy Go test file. gofmt shape matters: `rfc_tagged_scope.go_func_scopes`
# finds a top-level func by `^func` and its end by `^}` at column 0.
BASELINE_GO = """package a

import "testing"

func TestA(t *testing.T) {
	require.Equal(t, 1, f())
	require.NoError(t, err)
}

func TestB(t *testing.T) {
	require.True(t, g())
}
"""

# TestA stops running. `adding t.Skip` is a blocking arm of _test_weakening_errs.
SKIPPED_GO = """package a

import "testing"

func TestA(t *testing.T) {
	t.Skip("flaky")
	require.Equal(t, 1, f())
	require.NoError(t, err)
}

func TestB(t *testing.T) {
	require.True(t, g())
}
"""

# TestA loses one assertion and nothing else. `removing assertions` is an
# ADVISORY arm: the hook reports it and lets the edit through. AC-6 requires the
# commit gate to record it anyway.
FEWER_ASSERTIONS_GO = """package a

import "testing"

func TestA(t *testing.T) {
	require.Equal(t, 1, f())
}

func TestB(t *testing.T) {
	require.True(t, g())
}
"""

# TestA gains an assertion: strictly more coverage, so nothing to accept.
STRONGER_GO = """package a

import "testing"

func TestA(t *testing.T) {
	require.Equal(t, 1, f())
	require.NoError(t, err)
	require.Positive(t, n)
}

func TestB(t *testing.T) {
	require.True(t, g())
}
"""

# An assertion-shaped URL remains fixture data. The `//` is inside a balanced
# string token, so it cannot become the blocking "commenting out assertions" arm.
ASSERTION_STRING_GO = """package a

import "testing"

func TestA(t *testing.T) {
	url := "https://example.test//require.NoError"
	require.Equal(t, 1, f())
	require.NoError(t, err)
	_ = url
}

func TestB(t *testing.T) {
	require.True(t, g())
}
"""

# TestA is gone from the file.
DELETED_GO = """package a

import "testing"

func TestB(t *testing.T) {
	require.True(t, g())
}
"""


DOCWIRING_OLD_GO = """package a

func TestKept(t *testing.T) {
	require.NoError(t, err)
}

func TestRemoved(t *testing.T) {
	require.Equal(t, 1, got)
	require.NoError(t, err)
}
"""

DOCWIRING_NEW_GO = """package a

func TestKept(t *testing.T) {
	require.NoError(t, err)
}
"""

PACKAGE_ASSERTION_OLD_GO = """package a

var packageCheck = assert.Equal(t, 1, got)

func TestA(t *testing.T) {
	require.NoError(t, err)
}
"""

PACKAGE_ASSERTION_NEW_GO = """package a

func TestA(t *testing.T) {
	require.NoError(t, err)
	require.Equal(t, 1, got)
}
"""

MARKUP_GO = """package %s

import "testing"

func TestNoGoFileBuildsMarkup(t *testing.T) {
	require.Empty(t, offenders)
	require.NoError(t, err)
}
"""

MARKUP_WEAK_GO = """package %s

import "testing"

func TestNoGoFileBuildsMarkup(t *testing.T) {
	t.Skip("later")
	require.Empty(t, offenders)
	require.NoError(t, err)
}
"""

BASELINE_CI = """name=session comes up
exec=ze bgp run
expect=stdout:contains=Established
reject=stdout:contains=NOTIFICATION
"""

WEAK_CI = """name=session comes up
exec=ze bgp run
expect=stdout:contains=Established
"""

PYTHON_BASELINE = (
    "import unittest\n"
    "class T(unittest.TestCase):\n"
    "    def test_skip_detection(self):\n"
    '        fixture = "pass\\n"\n'
    "        self.assertEqual([], detect(fixture))\n"
)
PYTHON_FIXTURE_SKIP = (
    "import unittest\n"
    "class T(unittest.TestCase):\n"
    "    def test_skip_detection(self):\n"
    '        fixture = \'self.skipTest("flaky")\\n\'\n'
    "        self.assertEqual([], detect(fixture))\n"
)
PYTHON_EXECUTABLE_SKIP = (
    "import unittest\n"
    "class T(unittest.TestCase):\n"
    "    def test_skip_detection(self):\n"
    '        self.skipTest("flaky")\n'
    '        fixture = "pass\\n"\n'
    "        self.assertEqual([], detect(fixture))\n"
)


def contract(*rows: tuple[str, str]) -> str:
    """`test/weakened.md` carrying `rows`, in the shape phase 2 published."""
    body = "".join(f"| {name} | {reason} |\n" for name, reason in rows)
    return (
        "# Tests this commit weakens\n"
        "\n"
        "Prose the parser must not read as rows.\n"
        "\n"
        "| Carrier | The name |\n"
        "|---------|----------|\n"
        "| Go | the enclosing top-level `func TestXxx` |\n"
        "| `.ci`, `.et` | the file stem |\n"
        "\n"
        "| Test | Reason |\n"
        "|------|--------|\n" + body
    )


class Repo:
    """A throwaway git repository the checker can read HEAD from."""

    def __init__(self, root: str) -> None:
        self.root = root
        self._git(["init", "-q"])

    def _git(self, args: list[str]):
        return subprocess.run(
            ["git", *args], cwd=self.root, capture_output=True, text=True
        )

    def write(self, rel: str, text: str) -> None:
        full = os.path.join(self.root, rel)
        os.makedirs(os.path.dirname(full), exist_ok=True)
        with open(full, "w", encoding="utf-8") as fh:
            fh.write(text)

    def delete(self, rel: str) -> None:
        os.remove(os.path.join(self.root, rel))

    def commit(self, msg: str = "baseline") -> None:
        self._git(["add", "-A"])
        self._git([*GIT_ENV, "commit", "-q", "-m", msg])


class RepoCase(unittest.TestCase):
    """Builds a repository per test and tears it down afterwards."""

    def repo(self) -> Repo:
        tmp = tempfile.TemporaryDirectory(prefix="ze-weakened-test-")
        self.addCleanup(tmp.cleanup)
        return Repo(tmp.name)

    def baseline(self, go_path: str = "pkg/a_test.go") -> Repo:
        repo = self.repo()
        repo.write(go_path, BASELINE_GO)
        repo.write(cw.WEAKENED_PATH, contract())
        repo.commit()
        return repo


class TestCommitWithoutARow(RepoCase):
    """AC-3: a weakening the file does not name refuses the commit."""

    def test_commit_without_a_row_is_refused(self) -> None:
        repo = self.baseline()
        repo.write("pkg/a_test.go", SKIPPED_GO)
        problems = cw.weakened_problems(repo.root, ["pkg/a_test.go"])
        self.assertTrue(problems, "a skipped test with no row must refuse the commit")
        joined = "\n".join(problems)
        self.assertIn("TestA", joined)
        self.assertIn(cw.WEAKENED_PATH, joined)
        self.assertIn("adding t.Skip", joined)

    def test_the_row_is_what_makes_it_pass(self) -> None:
        # The discriminating half. Without it, a checker that always refused
        # would pass the test above (ai/rules/interop-and-goal-validation.md).
        repo = self.baseline()
        repo.write("pkg/a_test.go", SKIPPED_GO)
        repo.write(
            cw.WEAKENED_PATH,
            contract(("TestA", "the fixture it drove was deleted in this commit")),
        )
        self.assertEqual(cw.weakened_problems(repo.root, ["pkg/a_test.go"]), [])

    def test_a_deleted_test_function_needs_a_row(self) -> None:
        repo = self.baseline()
        repo.write("pkg/a_test.go", DELETED_GO)
        problems = cw.weakened_problems(repo.root, ["pkg/a_test.go"])
        self.assertTrue(problems)
        self.assertIn("TestA", "\n".join(problems))

    def test_a_removed_file_is_a_weakening(self) -> None:
        repo = self.baseline()
        repo.delete("pkg/a_test.go")
        problems = cw.weakened_problems(repo.root, [], removed=["pkg/a_test.go"])
        self.assertTrue(problems, "deleting the whole file must need a row")
        self.assertIn("TestA", "\n".join(problems))

    def test_a_path_the_commit_does_not_name_is_not_judged(self) -> None:
        # The property the spec's Required Reading pins: judge the paths the
        # commit NAMES, never the working tree. A shared checkout moves under
        # whoever reads it.
        repo = self.baseline()
        repo.write("pkg/a_test.go", SKIPPED_GO)
        repo.write("pkg/b_test.go", BASELINE_GO)
        self.assertEqual(cw.weakened_problems(repo.root, ["pkg/b_test.go"]), [])


class TestMovedTests(RepoCase):
    """A rename compares the old test with its new path instead of empty text."""

    def move(
        self,
        repo: Repo,
        new_text: str = BASELINE_GO,
        old_path: str = "pkg/a_test.go",
        new_path: str = "internal/le/a_test.go",
    ) -> tuple[str, str, tuple[cw.RenamePair, ...]]:
        repo.write(new_path, new_text)
        repo.delete(old_path)
        return old_path, new_path, (cw.RenamePair(old_path, new_path, 100),)

    def test_an_unchanged_move_is_clean(self) -> None:
        repo = self.baseline()
        old_path, new_path, renames = self.move(repo)
        self.assertEqual(
            cw.weakened_problems(
                repo.root, [new_path], removed=[old_path], rename_pairs=renames
            ),
            [],
        )

    def test_a_move_with_import_path_and_comment_edits_is_clean(self) -> None:
        repo = self.baseline()
        moved = BASELINE_GO.replace(
            'import "testing"',
            'import (\n\t"testing"\n\t"internal/pkg/testsupport"\n)',
        ).replace(
            "func TestA(t *testing.T) {",
            "// TestA uses testsupport after the package move.\n"
            "func TestA(t *testing.T) {\n\t_ = testsupport.Path",
        )
        old_path, new_path, renames = self.move(repo, moved)
        self.assertEqual(
            cw.weakened_problems(
                repo.root, [new_path], removed=[old_path], rename_pairs=renames
            ),
            [],
        )

    def test_a_moved_test_that_loses_an_assertion_uses_the_new_identity(self) -> None:
        repo = self.baseline()
        old_path, new_path, renames = self.move(repo, FEWER_ASSERTIONS_GO)
        weakened, errors = cw.weakened_tests(
            repo.root, [new_path], removed=[old_path], rename_pairs=renames
        )
        self.assertEqual(errors, [])
        self.assertEqual(
            [(item.path, item.package, item.name) for item in weakened],
            [(new_path, "le", "TestA")],
        )
        self.assertIn("removing assertions", "\n".join(weakened[0].details))
        repo.write(
            cw.WEAKENED_PATH,
            contract(("le.TestA", "the moved test no longer checks the error")),
        )
        self.assertEqual(
            cw.weakened_problems(
                repo.root, [new_path], removed=[old_path], rename_pairs=renames
            ),
            [],
        )

    def test_a_moved_rfc_tagged_test_remains_protected(self) -> None:
        old = BASELINE_GO.replace(
            "func TestA",
            "// RFC requirement: RFC4271-6.3-1 positive: malformed input is rejected.\n"
            "func TestA",
        )
        repo = self.baseline()
        repo.write("pkg/a_test.go", old)
        repo.commit("tagged baseline")
        moved = old.replace("\trequire.NoError(t, err)\n", "")
        old_path, new_path, renames = self.move(repo, moved)
        weakened, errors = cw.weakened_tests(
            repo.root, [new_path], removed=[old_path], rename_pairs=renames
        )
        self.assertEqual(errors, [])
        self.assertEqual(
            [(item.path, item.package, item.name) for item in weakened],
            [(new_path, "le", "TestA")],
        )
        self.assertIn("removing assertions", "\n".join(weakened[0].details))

    def test_a_moved_test_that_loses_an_rfc_tag_is_blocked(self) -> None:
        tag = (
            "\t// RFC requirement: RFC4271-6.3-1 positive: "
            "malformed input is rejected.\n"
        )
        old = BASELINE_GO.replace(
            "\trequire.Equal(t, 1, f())\n",
            tag + "\trequire.Equal(t, 1, f())\n",
        )
        repo = self.baseline()
        repo.write("pkg/a_test.go", old)
        repo.commit("tagged baseline")
        old_path, new_path, renames = self.move(repo, old.replace(tag, ""))
        weakened, errors = cw.weakened_tests(
            repo.root, [new_path], removed=[old_path], rename_pairs=renames
        )
        self.assertEqual(errors, [])
        self.assertEqual(
            [(item.path, item.package, item.name) for item in weakened],
            [(new_path, "le", "TestA")],
        )
        self.assertIn(
            "removing RFC requirement tag", "\n".join(weakened[0].details)
        )

    def test_ambiguous_rename_pairs_fail_closed(self) -> None:
        repo = self.baseline()
        old_path, new_path, _ = self.move(repo)
        second_path = "other/pkg/a_test.go"
        repo.write(second_path, BASELINE_GO)
        renames = (
            cw.RenamePair(old_path, new_path, 100),
            cw.RenamePair(old_path, second_path, 100),
        )
        weakened, errors = cw.weakened_tests(
            repo.root,
            [new_path, second_path],
            removed=[old_path],
            rename_pairs=renames,
        )
        self.assertEqual(weakened, [])
        self.assertIn("ambiguous", "\n".join(errors).lower())

    def test_an_unreadable_rename_target_fails_closed(self) -> None:
        repo = self.baseline()
        old_path = "pkg/a_test.go"
        new_path = "internal/le/a_test.go"
        repo.delete(old_path)
        weakened, errors = cw.weakened_tests(
            repo.root,
            [new_path],
            removed=[old_path],
            rename_pairs=(cw.RenamePair(old_path, new_path, 100),),
        )
        self.assertEqual(weakened, [])
        self.assertIn("could not run", "\n".join(errors))

    def test_an_unrelated_worktree_weakening_is_excluded_from_a_move(self) -> None:
        repo = self.baseline()
        repo.write("other/unrelated_test.go", BASELINE_GO)
        repo.commit("add unrelated test")
        repo.write("other/unrelated_test.go", SKIPPED_GO)
        old_path, new_path, renames = self.move(repo)
        self.assertEqual(
            cw.weakened_problems(
                repo.root, [new_path], removed=[old_path], rename_pairs=renames
            ),
            [],
        )


class TestStaleRow(RepoCase):
    """AC-4 / R-2: a row for a test this commit does not weaken is refused."""

    def test_a_row_for_an_untouched_test_is_refused(self) -> None:
        repo = self.baseline()
        repo.write("pkg/a_test.go", SKIPPED_GO)
        repo.write(
            cw.WEAKENED_PATH,
            contract(
                ("TestA", "the fixture it drove was deleted"),
                ("TestGone", "left over from the last commit"),
            ),
        )
        problems = cw.weakened_problems(repo.root, ["pkg/a_test.go"])
        self.assertTrue(problems, "a stale row must refuse the commit")
        joined = "\n".join(problems)
        self.assertIn("TestGone", joined)
        self.assertNotIn("TestA |", joined)

    def test_a_row_with_no_reason_is_refused(self) -> None:
        repo = self.baseline()
        repo.write("pkg/a_test.go", SKIPPED_GO)
        repo.write(cw.WEAKENED_PATH, contract(("TestA", "")))
        problems = cw.weakened_problems(repo.root, ["pkg/a_test.go"])
        self.assertTrue(problems, "a row with no reason accepts nothing")
        self.assertIn("reason", "\n".join(problems).lower())

    def test_the_same_test_named_twice_is_refused(self) -> None:
        repo = self.baseline()
        repo.write("pkg/a_test.go", SKIPPED_GO)
        repo.write(
            cw.WEAKENED_PATH, contract(("TestA", "one reason"), ("TestA", "another"))
        )
        problems = cw.weakened_problems(repo.root, ["pkg/a_test.go"])
        self.assertTrue(problems, "two rows for one test are two answers")


class TestWeakensNothing(RepoCase):
    """AC-5: with nothing to accept, the file's content is not read."""

    def test_a_commit_that_weakens_nothing_ignores_the_file(self) -> None:
        repo = self.baseline()
        repo.write("pkg/a_test.go", STRONGER_GO)
        repo.write(cw.WEAKENED_PATH, contract(("TestGone", "stale from last commit")))
        self.assertEqual(cw.weakened_problems(repo.root, ["pkg/a_test.go"]), [])

    def test_a_missing_file_costs_nothing_when_nothing_is_weakened(self) -> None:
        repo = self.baseline()
        repo.delete(cw.WEAKENED_PATH)
        repo.write("pkg/a_test.go", STRONGER_GO)
        self.assertEqual(cw.weakened_problems(repo.root, ["pkg/a_test.go"]), [])

    def test_an_assertion_shaped_url_is_not_a_weakening(self) -> None:
        repo = self.baseline()
        repo.write("pkg/a_test.go", ASSERTION_STRING_GO)
        self.assertEqual(cw.weakened_problems(repo.root, ["pkg/a_test.go"]), [])

    def test_a_missing_file_refuses_a_weakening(self) -> None:
        repo = self.baseline()
        repo.delete(cw.WEAKENED_PATH)
        repo.write("pkg/a_test.go", SKIPPED_GO)
        problems = cw.weakened_problems(repo.root, ["pkg/a_test.go"])
        self.assertTrue(problems)
        self.assertIn(cw.WEAKENED_PATH, "\n".join(problems))

    def test_a_non_test_path_is_ignored(self) -> None:
        repo = self.baseline()
        repo.write("pkg/a.go", "package a\n")
        self.assertEqual(cw.weakened_problems(repo.root, ["pkg/a.go"]), [])

    def test_the_same_path_in_both_lists_is_judged_once(self) -> None:
        # Deleting the file loses both tests it held, so two problems is the
        # right answer and four would mean the path was read twice.
        repo = self.baseline()
        repo.delete("pkg/a_test.go")
        problems = cw.weakened_problems(
            repo.root, ["pkg/a_test.go"], removed=["pkg/a_test.go"]
        )
        self.assertEqual(len(problems), 2, problems)
        joined = "\n".join(problems)
        self.assertIn("TestA", joined)
        self.assertIn("TestB", joined)


class TestUnitResidualAttribution(unittest.TestCase):
    """Whole-file count drops name only coverage outside top-level functions."""

    @classmethod
    def setUpClass(cls) -> None:
        cls.detector = staticmethod(cw.load_detector(cw.PROJECT_DIR))
        cls.rfc_detector = staticmethod(cw.load_rfc_detector(cw.PROJECT_DIR))

    def units(self, old: str, new: str):
        return cw.weakened_units(
            "pkg/a_test.go",
            old,
            new,
            self.detector,
            self.rfc_detector,
        )

    def test_deleted_function_assertions_are_not_attributed_to_file_stem(
        self,
    ) -> None:
        units = self.units(DOCWIRING_OLD_GO, DOCWIRING_NEW_GO)
        self.assertEqual([name for name, _ in units], ["TestRemoved"])
        self.assertNotIn(
            "removing assertions",
            "\n".join(
                detail
                for name, details in units
                if name == "a_test"
                for detail in details
            ),
        )

    def test_package_scope_assertion_drop_is_preserved(self) -> None:
        units = self.units(PACKAGE_ASSERTION_OLD_GO, PACKAGE_ASSERTION_NEW_GO)
        self.assertEqual([name for name, _ in units], ["a_test"])
        self.assertIn(
            "removing assertions outside top-level functions",
            "\n".join(units[0][1]),
        )


class TestPythonExecutableText(RepoCase):
    """The shared producer judges Python code, not source stored as fixture data."""

    def test_fixture_skip_is_ignored_and_executable_skip_is_detected(self) -> None:
        repo = self.repo()
        path = "scripts/dev/tool_test.py"
        repo.write(path, PYTHON_BASELINE)
        repo.commit()

        repo.write(path, PYTHON_FIXTURE_SKIP)
        weakened, errors = cw.weakened_tests(repo.root, [path])
        self.assertEqual(errors, [])
        self.assertEqual(weakened, [], "fixture text is not an executable skip")

        repo.write(path, PYTHON_EXECUTABLE_SKIP)
        weakened, errors = cw.weakened_tests(repo.root, [path])
        self.assertEqual(errors, [])
        self.assertEqual(len(weakened), 1, weakened)
        self.assertEqual(weakened[0].name, "tool_test")
        self.assertIn("adding a Python skip", "\n".join(weakened[0].details))

    def test_malformed_python_source_is_not_masked(self) -> None:
        malformed = 'fixture = """self.skipTest("flaky")\n'
        self.assertEqual(
            cw.executable_test_text("scripts/dev/tool_test.py", malformed),
            malformed,
        )


class TestNoComparison(RepoCase):
    """An empty result must never come from a comparison that did not happen."""

    def test_an_anchor_that_does_not_resolve_is_reported(self) -> None:
        repo = self.baseline()
        repo.write("pkg/a_test.go", SKIPPED_GO)
        problems = cw.weakened_problems(
            repo.root, ["pkg/a_test.go"], anchor="no-such-ref"
        )
        self.assertTrue(
            problems,
            "an unresolvable anchor compares nothing, which is not a clean tree",
        )
        self.assertIn("could not run", "\n".join(problems))

    def test_a_repository_with_no_commit_is_reported(self) -> None:
        repo = self.repo()
        repo.write("pkg/a_test.go", BASELINE_GO)
        problems = cw.weakened_problems(repo.root, ["pkg/a_test.go"])
        self.assertTrue(problems, "no HEAD means nothing was compared")
        self.assertIn("could not run", "\n".join(problems))


class TestEveryWeakeningKind(RepoCase):
    """AC-6: the advisory arms are recorded too, not only the blocking ones."""

    def test_every_weakening_kind_reaches_the_check(self) -> None:
        repo = self.baseline()
        repo.write("pkg/a_test.go", FEWER_ASSERTIONS_GO)
        problems = cw.weakened_problems(repo.root, ["pkg/a_test.go"])
        self.assertTrue(
            problems,
            "a falling assertion count is advisory at the hook and recorded here",
        )
        self.assertIn("removing assertions", "\n".join(problems))

    def test_no_detector_finding_is_filtered_out(self) -> None:
        # The structural half of AC-6. Naming two arms proves those two reach the
        # check; driving the detector proves NOTHING it returns is dropped, which
        # is the claim, and it holds for arms added after this test was written.
        repo = self.baseline()
        repo.write("pkg/a_test.go", SKIPPED_GO)
        problems = cw.weakened_problems(
            repo.root,
            ["pkg/a_test.go"],
            detector=lambda old, new, fp: (["BLOCKING-ARM"], ["ADVISORY-ARM"]),
        )
        joined = "\n".join(problems)
        self.assertIn("BLOCKING-ARM", joined)
        self.assertIn("ADVISORY-ARM", joined)

    def test_a_ci_expectation_that_left_is_recorded(self) -> None:
        repo = self.repo()
        repo.write("test/plugin/session.ci", BASELINE_CI)
        repo.write(cw.WEAKENED_PATH, contract())
        repo.commit()
        repo.write("test/plugin/session.ci", WEAK_CI)
        problems = cw.weakened_problems(repo.root, ["test/plugin/session.ci"])
        self.assertTrue(problems)
        joined = "\n".join(problems)
        self.assertIn("session", joined, "a .ci test is named by its file stem")
        self.assertIn("expectations", joined)

    def test_the_ci_file_stem_is_the_row_name(self) -> None:
        repo = self.repo()
        repo.write("test/plugin/session.ci", BASELINE_CI)
        repo.write(cw.WEAKENED_PATH, contract())
        repo.commit()
        repo.write("test/plugin/session.ci", WEAK_CI)
        repo.write(cw.WEAKENED_PATH, contract(("session", "the reject= moved to x.ci")))
        self.assertEqual(
            cw.weakened_problems(repo.root, ["test/plugin/session.ci"]), []
        )


class TestAmbiguousName(RepoCase):
    """AC-7 / A-2: one name, two packages, both weakened."""

    def two_packages(self) -> Repo:
        repo = self.repo()
        repo.write("internal/component/lg/markup_check_test.go", MARKUP_GO % "lg")
        repo.write("internal/component/web/markup_check_test.go", MARKUP_GO % "web")
        repo.write(cw.WEAKENED_PATH, contract())
        repo.commit()
        repo.write("internal/component/lg/markup_check_test.go", MARKUP_WEAK_GO % "lg")
        repo.write(
            "internal/component/web/markup_check_test.go", MARKUP_WEAK_GO % "web"
        )
        return repo

    def paths(self) -> list[str]:
        return [
            "internal/component/lg/markup_check_test.go",
            "internal/component/web/markup_check_test.go",
        ]

    def test_an_ambiguous_test_name_is_refused(self) -> None:
        repo = self.two_packages()
        repo.write(
            cw.WEAKENED_PATH,
            contract(("TestNoGoFileBuildsMarkup", "the scan moved to markupcheck")),
        )
        problems = cw.weakened_problems(repo.root, self.paths())
        self.assertTrue(problems, "one bare name cannot accept two weakenings")
        joined = "\n".join(problems)
        self.assertIn("package.TestName", joined)
        self.assertIn("lg", joined)
        self.assertIn("web", joined)
        self.assertEqual(
            len(problems),
            1,
            "the ambiguity is the one problem to fix; reporting each weakening "
            "again as unrecorded would bury it",
        )

    def test_a_qualified_name_resolves_each_one(self) -> None:
        repo = self.two_packages()
        repo.write(
            cw.WEAKENED_PATH,
            contract(
                ("lg.TestNoGoFileBuildsMarkup", "the scan moved to markupcheck"),
                ("web.TestNoGoFileBuildsMarkup", "the scan moved to markupcheck"),
            ),
        )
        self.assertEqual(cw.weakened_problems(repo.root, self.paths()), [])

    def test_a_bare_name_is_accepted_when_only_one_is_weakened(self) -> None:
        repo = self.two_packages()
        repo.write("internal/component/web/markup_check_test.go", MARKUP_GO % "web")
        repo.write(
            cw.WEAKENED_PATH,
            contract(("TestNoGoFileBuildsMarkup", "the scan moved to markupcheck")),
        )
        self.assertEqual(cw.weakened_problems(repo.root, self.paths()), [])

    def test_a_qualifier_naming_another_package_does_not_match(self) -> None:
        repo = self.two_packages()
        repo.write("internal/component/web/markup_check_test.go", MARKUP_GO % "web")
        repo.write(
            cw.WEAKENED_PATH,
            contract(("web.TestNoGoFileBuildsMarkup", "wrong package")),
        )
        problems = cw.weakened_problems(repo.root, self.paths())
        self.assertTrue(problems, "a row must name the package it actually weakens")


class TestSharedNameResolution(RepoCase):
    """The enclosing name has ONE definition and this module does not own it."""

    def test_the_enclosing_test_name_comes_from_rfc_tagged_scope(self) -> None:
        repo = self.baseline()
        repo.write("pkg/a_test.go", SKIPPED_GO)
        with mock.patch.object(
            cw.rfc_tagged_scope,
            "go_func_units",
            side_effect=lambda content: [("SentinelUnit", content)],
        ) as units:
            problems = cw.weakened_problems(repo.root, ["pkg/a_test.go"])
        self.assertTrue(units.called, "the checker must ask rfc_tagged_scope")
        self.assertIn(
            "SentinelUnit",
            "\n".join(problems),
            "the reported name must be the one rfc_tagged_scope returned",
        )

    def test_the_module_is_loaded_from_scripts_dev(self) -> None:
        # A second copy that drifted would make two gates disagree about which
        # text a rule covers (scripts/dev/rfc_tagged_scope.py, module docstring).
        self.assertEqual(
            os.path.realpath(cw.rfc_tagged_scope.__file__),
            os.path.realpath(
                os.path.join(os.path.dirname(__file__), "rfc_tagged_scope.py")
            ),
        )

    def test_the_detector_is_the_hook_own_function(self) -> None:
        detector = cw.load_detector(cw.PROJECT_DIR)
        self.assertIsNotNone(detector, "the hook's detector must be importable")
        self.assertEqual(detector.__name__, "_test_weakening_errs")


class TestParse(unittest.TestCase):
    """The table reader, driven directly."""

    def test_only_the_test_reason_table_is_read(self) -> None:
        rows, problems = cw.parse_weakened_file(
            contract(("TestA", "a reason")),
        )
        self.assertEqual(problems, [])
        self.assertEqual([(r.name, r.reason) for r in rows], [("TestA", "a reason")])

    def test_a_missing_header_is_a_problem(self) -> None:
        rows, problems = cw.parse_weakened_file("# Tests\n\nno table here\n")
        self.assertEqual(rows, [])
        self.assertTrue(problems)
        self.assertIn("| Test | Reason |", "\n".join(problems))

    def test_two_tables_are_a_problem(self) -> None:
        text = contract(("TestA", "a reason")) + "\n" + contract(("TestB", "another"))
        _, problems = cw.parse_weakened_file(text)
        self.assertTrue(problems, "two tables give the gate two answers")

    def test_an_empty_table_parses_to_no_rows(self) -> None:
        rows, problems = cw.parse_weakened_file(contract())
        self.assertEqual(rows, [])
        self.assertEqual(problems, [])

    def test_a_row_with_no_cells_filled_is_reported(self) -> None:
        # `| | |` has the shape of the separator once empty cells are dropped.
        # Reading it as one would make a row that names no test disappear.
        rows, problems = cw.parse_weakened_file(contract() + "| | |\n")
        self.assertEqual(rows, [])
        self.assertTrue(problems, "a row naming no test must be reported")


class TestFileShape(unittest.TestCase):
    """What `make ze-test-weakened-check` runs with no paths."""

    def test_the_committed_contract_file_parses(self) -> None:
        # Pins the parser against the file phase 2 published. If either moves,
        # this fails rather than the commit gate silently reading no rows.
        self.assertEqual(cw.file_shape_problems(cw.PROJECT_DIR), [])

    def test_a_broken_table_is_reported(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            os.makedirs(os.path.join(tmp, "test"))
            with open(os.path.join(tmp, cw.WEAKENED_PATH), "w") as fh:
                fh.write("# Tests\n\nno table\n")
            self.assertTrue(cw.file_shape_problems(tmp))

    def test_a_missing_file_is_reported(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            problems = cw.file_shape_problems(tmp)
            self.assertTrue(problems)
            self.assertIn(cw.WEAKENED_PATH, "\n".join(problems))


class TestSelftest(unittest.TestCase):
    def test_selftest_passes(self) -> None:
        self.assertEqual(cw.selftest(quiet=True), 0)


if __name__ == "__main__":
    unittest.main()
