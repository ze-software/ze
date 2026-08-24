#!/usr/bin/env python3
# Design: scripts/dev/lint_flavors.py -- the build-flavor lint driver
#
# Unit tests for the flavor driver. `go test ./scripts/dev/` runs them
# (python_tests_test.go globs *_test.py), so they are inside `make ze-unit-test`
# with no make target of their own.

import os
import re
import shutil
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import lint_flavors  # noqa: E402


class TestBasePasses(unittest.TestCase):
    """The two rows the driver subtracts must be the two passes make runs.

    The driver derives each flavor's package set by subtracting what the base
    passes already load. A row that stops matching the Makefile would silently
    shrink or widen every flavor's scope, and nothing else would notice.
    """

    def setUp(self):
        makefile = os.path.join(lint_flavors.ROOT, "Makefile")
        with open(makefile, encoding="utf-8") as handle:
            self.corpus = handle.read()

    def recipe(self, target):
        lines = self.corpus.split("\n")
        for index, line in enumerate(lines):
            if line.startswith(target + ":"):
                body = []
                for following in lines[index + 1 :]:
                    if following and not following.startswith(("\t", " ")):
                        break
                    body.append(following)
                return body
        self.fail(f"{target} is not in the Makefile: this test must not pass vacuously")
        return []

    def test_base_passes_match_the_lint_recipe(self):
        body = self.recipe("_ze-lint-impl")
        passes = [line for line in body if "ZE_LINT_RUN" in line]
        self.assertEqual(
            len(passes),
            len(lint_flavors.BASE_PASSES),
            f"_ze-lint-impl runs {len(passes)} golangci-lint passes and BASE_PASSES declares "
            f"{len(lint_flavors.BASE_PASSES)}. Every flavor's scope is derived by subtracting "
            "what these load, so the two must agree.",
        )
        for line, base in zip(passes, lint_flavors.BASE_PASSES):
            goos = re.search(r"GOOS=(\w+)", line)
            self.assertEqual(
                goos.group(1) if goos else None,
                base["goos"],
                f"pass {base['name']} runs under a different GOOS than BASE_PASSES declares: {line}",
            )
            tags = re.search(r"--build-tags (\S+)", line)
            self.assertEqual(
                tags.group(1).split(",") if tags else [],
                base["tags"],
                f"pass {base['name']} carries different tags than BASE_PASSES declares: {line}",
            )

    def test_the_driver_runs_from_both_lint_recipes(self):
        for target in ("_ze-lint-impl", "_ze-lint-changed-impl"):
            body = "\n".join(self.recipe(target))
            self.assertIn(
                "ZE_LINT_FLAVOR_RUN",
                body,
                f"{target} does not run the flavor driver, so every personality-tagged file "
                "passes that gate unlinted",
            )


class TestTags(unittest.TestCase):
    def test_effective_tags_adds_to_the_config_list(self):
        tags = lint_flavors.effective_tags({"tags": ["ze_installer"]})
        self.assertIn("ze_core", tags, "a flavor must inherit the config's tags")
        self.assertIn("ze_installer", tags)

    def test_without_removes_a_config_tag(self):
        tags = lint_flavors.effective_tags(
            {"tags": ["ze_setup"], "without": ["ze_core"]}
        )
        self.assertNotIn("ze_core", tags, "`without` is the only way to turn a tag OFF")
        self.assertIn("ze_setup", tags)

    def test_the_compile_out_row_keeps_ze_core_alone(self):
        """The stubs are reached by a build with every feature gate OFF.

        Read from the table rather than from a literal: a gate added to
        feature-gates.txt reaches .golangci.yml through `make generate`, and the
        row derives its `without` from there. A row that stopped dropping one
        gate would leave that feature's stubs selected by no pass.
        """
        rows = [row for row in lint_flavors.FLAVORS if row["name"] == "compile-out"]
        self.assertEqual(len(rows), 1, "the compile-out flavor row is gone")
        self.assertEqual(
            lint_flavors.effective_tags(rows[0]),
            ["ze_core"],
            "the compile-out build must carry ze_core and no feature gate: a gate left ON "
            "leaves its `!ze_<gate>` stubs linted by nothing",
        )
        self.assertGreater(
            len(lint_flavors.feature_gate_tags()),
            1,
            "feature_gate_tags read no gates, so the row above drops nothing and passes "
            "vacuously",
        )

    def test_the_tagless_config_carries_no_build_tags(self):
        path = lint_flavors.tagless_config()
        with open(path, encoding="utf-8") as handle:
            text = handle.read()
        self.assertNotIn("build-tags:", text)
        self.assertIn(
            "relative-path-mode: gitroot",
            text,
            "without it golangci-lint reports paths relative to the config file, which sits "
            "under tmp/",
        )
        # The linter set must be the same one every other pass runs.
        self.assertIn("- errcheck", text)
        os.remove(path)


class TestPopulation(unittest.TestCase):
    def test_a_file_deleted_in_the_working_tree_leaves_the_population(self):
        """`git ls-files` answers from the INDEX, so a deleted file is tracked.

        No pass can lint a file that is not on disk, and calling one blind fails
        every run between the deletion and its commit. Measured on 2026-08-24: a
        full `make ze-lint` went red on `cmd/ze/hub/bgp_decode_nolink_test.go`
        with all 19 passes at 0 issues.

        The fake ROOT is OUTSIDE the repository, for the reason the compile-out
        stub tests gave: a .go file under tmp/ is picked up by tooling that
        walks the tree.
        """
        root = tempfile.mkdtemp(prefix="lint-flavors-population-")
        self.addCleanup(shutil.rmtree, root, True)
        with open(os.path.join(root, "present.go"), "w", encoding="utf-8") as handle:
            handle.write("package sample\n")
        real_root, real_run = lint_flavors.ROOT, lint_flavors.run
        lint_flavors.ROOT = root
        lint_flavors.run = lambda *_a, **_k: (0, "present.go\ndeleted.go\n", "")
        self.addCleanup(setattr, lint_flavors, "ROOT", real_root)
        self.addCleanup(setattr, lint_flavors, "run", real_run)
        self.assertEqual(lint_flavors.population(), {"present.go"})

    def test_the_population_is_ze_source_only(self):
        files = lint_flavors.population()
        self.assertIn("scripts/status/verify_run.go", files)
        self.assertFalse(
            [path for path in files if path.startswith("vendor/")],
            "vendor/ is other people's code",
        )
        self.assertNotIn(
            "scripts/checks/tracked_build.go",
            files,
            "a //go:build ignore file belongs to no build by its own declaration",
        )


if __name__ == "__main__":
    unittest.main()
