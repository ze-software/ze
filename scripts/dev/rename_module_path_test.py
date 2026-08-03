#!/usr/bin/env python3
"""Unit tests for scripts/dev/rename_module_path.py.

Run directly, or via `go test ./scripts/dev` (TestPythonUnitTests globs
*_test.py). What these pin down is the part a rename tool gets wrong quietly:
which occurrences are module paths and which merely look like one.
"""

from __future__ import annotations

import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import rename_module_path as rmp  # noqa: E402

# Deliberately NOT this repository's real module paths: the tool rewrites every
# tracked file, so real paths here would rewrite the fixtures out from under the
# test the first time someone renames the module again.
OLD = "oldforge.example/owner/proj"
NEW = "newforge.example/other-owner/proj"


def write(root: Path, rel: str, text: str) -> None:
    path = root / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


class PlanEdits(unittest.TestCase):
    """VALIDATES: every occurrence lands in exactly one of rewrite/regenerate/skip.
    PREVENTS: a silent third outcome -- an occurrence that is neither rewritten
    nor reported, which is how a rename ships half-done."""

    def plan(self, files: dict[str, str]):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for rel, text in files.items():
                write(root, rel, text)
            return rmp.plan_edits(root, sorted(files), OLD, NEW)

    def test_source_and_docs_are_rewritten(self):
        plan = self.plan(
            {
                "go.mod": f"module {OLD}\n",
                "internal/a/a.go": f'import "{OLD}/internal/core/env"\n',
                "docs/guide/x.md": f"`{OLD}/pkg/plugin`\n",
                "Makefile": f"go list ./... | grep -v '^{OLD}$'\n",
            }
        )
        self.assertEqual(
            [rel for rel, _ in plan["edits"]],
            ["Makefile", "docs/guide/x.md", "go.mod", "internal/a/a.go"],
        )
        self.assertEqual(plan["regenerate"], [])
        self.assertEqual(plan["skips"], [])

    def test_occurrences_are_counted_not_just_files(self):
        plan = self.plan({"a.go": f'"{OLD}/x"\n"{OLD}/y"\n"{OLD}/z"\n'})
        self.assertEqual(plan["edits"], [("a.go", 3)])

    def test_pb_go_is_refused_for_regeneration(self):
        # The rawDesc encodes go_package with a varint length prefix; a plain
        # substitution of a shorter path compiles and decodes to garbage.
        plan = self.plan({"api/proto/ze.pb.go": f'"\\x0eZ,{OLD}/api/proto;zepb"\n'})
        self.assertEqual(plan["regenerate"], [("api/proto/ze.pb.go", 1)])
        self.assertEqual(plan["edits"], [])

    def test_proto_source_is_rewritten_normally(self):
        # The .proto IS the canonical input, so it must change for the
        # regenerated .pb.go to come out right.
        plan = self.plan(
            {"api/proto/ze.proto": f'option go_package = "{OLD}/api/proto;zepb";\n'}
        )
        self.assertEqual(plan["edits"], [("api/proto/ze.proto", 1)])

    def test_vendored_third_party_is_skipped(self):
        plan = self.plan({"vendor/example.com/x/x.go": f'"{OLD}/nope"\n'})
        self.assertEqual(plan["edits"], [])
        self.assertEqual(plan["skips"], [("vendor/example.com/x/x.go", 1, "vendor")])

    def test_absolute_checkout_path_is_skipped_and_reported(self):
        # Not a module path: the directory this repo happens to be cloned into.
        plan = self.plan(
            {".claude/settings.local.json": f'"Read(//Users/t/Code/{OLD}/**)"\n'}
        )
        self.assertEqual(plan["edits"], [])
        self.assertEqual(
            plan["skips"], [(".claude/settings.local.json", 1, "not-a-module-path")]
        )

    def test_the_module_cache_shim_go_mod_is_rewritten(self):
        # gokrazy/modcache holds third-party trees, but its own go.mod declares a
        # module inside our namespace, so the cache is in scope, not skipped.
        plan = self.plan(
            {
                "gokrazy/modcache/go.mod": f"module {OLD}/gokrazy/modcache\n",
                "gokrazy/modcache/example.com/x@v1/x.go": "package x\n",
            }
        )
        self.assertEqual(plan["edits"], [("gokrazy/modcache/go.mod", 1)])
        self.assertEqual(plan["skips"], [])

    def test_rfc_audit_history_is_never_text_rewritten(self):
        # A re-stamp note names the module paths of the rename it recorded. A
        # later rename rewriting those would turn the record into a false account
        # of what happened; the fingerprints inside are updated by the reseal
        # step, which proves what it changes.
        plan = self.plan(
            {"rfc/audit/rfc9999.json": f'{{"reaudit_note": "moved from {OLD}"}}\n'}
        )
        self.assertEqual(plan["edits"], [])
        self.assertEqual(plan["skips"], [("rfc/audit/rfc9999.json", 1, "rfc/audit")])

    def test_other_worktrees_are_skipped(self):
        plan = self.plan({".claude/worktrees/w/internal/a.go": f'"{OLD}/x"\n'})
        self.assertEqual(plan["edits"], [])
        self.assertEqual(plan["skips"][0][2], ".claude/worktrees")

    def test_binary_files_are_left_alone(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "blob.bin").write_bytes(b"\x00\x01" + OLD.encode() + b"\x00")
            self.assertEqual(rmp.plan_edits(root, ["blob.bin"], OLD, NEW)["edits"], [])

    def test_file_without_the_path_is_not_listed(self):
        plan = self.plan({"unrelated.go": "package main\n"})
        self.assertEqual(plan["edits"], [])


class SegmentedForm(unittest.TestCase):
    """VALIDATES: the module path spelled as quoted path segments is renamed too.
    PREVENTS: the literal-only rename that left six appliance tests calling
    filepath.Join(.., "codeberg.org", "thomas-mangin", "ze") -- code that still
    compiles and still passes review, and fails against a directory that moved."""

    def test_filepath_join_segments_are_rewritten(self):
        text = 'filepath.Join(root, "builddir", "oldforge.example", "owner", "proj")'
        out, n = rmp.rewrite(text, OLD, NEW)
        self.assertEqual(n, 1)
        self.assertEqual(
            out,
            'filepath.Join(root, "builddir", "newforge.example", "other-owner", "proj")',
        )

    def test_separators_including_line_breaks_are_preserved(self):
        text = 'Join(\n\t"oldforge.example",\n\t"owner",\n\t"proj",\n)'
        out, n = rmp.rewrite(text, OLD, NEW)
        self.assertEqual(n, 1)
        self.assertEqual(
            out, 'Join(\n\t"newforge.example",\n\t"other-owner",\n\t"proj",\n)'
        )

    def test_a_partial_segment_run_is_not_touched(self):
        text = '"oldforge.example", "someone-else"'
        out, n = rmp.rewrite(text, OLD, NEW)
        self.assertEqual((out, n), (text, 0))

    def test_both_spellings_are_counted_together(self):
        text = f'"{OLD}/x"\nfilepath.Join("oldforge.example", "owner", "proj")\n'
        _, n = rmp.rewrite(text, OLD, NEW)
        self.assertEqual(n, 2)

    def test_a_depth_change_skips_the_segment_form_rather_than_guessing(self):
        # Segments cannot map one to one across different depths, and inventing a
        # mapping would silently produce a wrong path.
        text = 'filepath.Join("oldforge.example", "owner", "proj")'
        out, n = rmp.rewrite(text, OLD, "newforge.example/proj")
        self.assertEqual((out, n), (text, 0))


class PlanMoves(unittest.TestCase):
    """VALIDATES: directories whose NAME spells the module path are moved.
    PREVENTS: leaving gokrazy's builddir at the old path, where the module it
    declares no longer matches the directory it sits in."""

    def moves(self, files: list[str]):
        with tempfile.TemporaryDirectory() as tmp:
            return rmp.plan_moves(Path(tmp), files, OLD, NEW)

    def test_builddir_is_moved(self):
        self.assertEqual(
            self.moves([f"gokrazy/ze/builddir/{OLD}/go.mod"]),
            [
                (
                    f"gokrazy/ze/builddir/{OLD}",
                    f"gokrazy/ze/builddir/{NEW}",
                )
            ],
        )

    def test_one_move_per_directory_not_per_file(self):
        self.assertEqual(
            len(
                self.moves(
                    [
                        f"gokrazy/ze/builddir/{OLD}/go.mod",
                        f"gokrazy/ze/builddir/{OLD}/go.sum",
                        f"gokrazy/ze/builddir/{OLD}/sub/x.go",
                    ]
                )
            ),
            1,
        )

    def test_scratch_copies_are_not_moved(self):
        self.assertEqual(self.moves([f"tmp/scratch/builddir/{OLD}/go.mod"]), [])

    def test_a_file_merely_importing_the_path_moves_nothing(self):
        self.assertEqual(self.moves(["internal/core/env/env.go"]), [])


class GoTargets(unittest.TestCase):
    """VALIDATES: goimports is pointed at post-move paths, and never at the
    generated composition roots.
    PREVENTS: goimports reformatting all.go, whose exact bytes the generator
    owns and re-checks."""

    def test_paths_are_remapped_through_the_move(self):
        with tempfile.TemporaryDirectory() as tmp:
            targets = rmp.go_targets(
                Path(tmp),
                [(f"gokrazy/ze/builddir/{OLD}/x.go", 1)],
                [(f"gokrazy/ze/builddir/{OLD}", f"gokrazy/ze/builddir/{NEW}")],
            )
            self.assertEqual(targets, [f"gokrazy/ze/builddir/{NEW}/x.go"])

    def test_generated_composition_roots_are_excluded(self):
        with tempfile.TemporaryDirectory() as tmp:
            targets = rmp.go_targets(
                Path(tmp),
                [
                    ("internal/component/plugin/all/all.go", 1),
                    ("internal/component/plugin/all/all_ze_bgp.go", 1),
                    ("internal/component/plugin/all/helper.go", 1),
                ],
                [],
            )
            self.assertEqual(targets, ["internal/component/plugin/all/helper.go"])

    def test_non_go_files_are_not_handed_to_goimports(self):
        with tempfile.TemporaryDirectory() as tmp:
            self.assertEqual(rmp.go_targets(Path(tmp), [("docs/x.md", 1)], []), [])


class Apply(unittest.TestCase):
    """VALIDATES: --apply rewrites contents and relocates directories.
    PREVENTS: a half-applied rename that still builds locally because the stale
    directory is still on disk."""

    def test_contents_are_rewritten(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, "a.go", f'import "{OLD}/internal/x"\n')
            rmp.apply_edits(root, [("a.go", 1)], OLD, NEW)
            self.assertEqual(
                (root / "a.go").read_text(), f'import "{NEW}/internal/x"\n'
            )

    def test_directory_is_moved_and_the_old_husk_removed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, f"gokrazy/ze/builddir/{OLD}/go.mod", "module x\n")
            rmp.apply_moves(
                root,
                [(f"gokrazy/ze/builddir/{OLD}", f"gokrazy/ze/builddir/{NEW}")],
            )
            self.assertTrue((root / f"gokrazy/ze/builddir/{NEW}/go.mod").is_file())
            # No empty codeberg.org/thomas-mangin/ shell left behind.
            self.assertFalse((root / "gokrazy/ze/builddir/codeberg.org").exists())
            self.assertTrue((root / "gokrazy/ze/builddir").is_dir())

    def test_move_onto_an_existing_destination_is_refused(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, f"b/{OLD}/go.mod", "module x\n")
            write(root, f"b/{NEW}/go.mod", "module y\n")
            with self.assertRaises(SystemExit):
                rmp.apply_moves(root, [(f"b/{OLD}", f"b/{NEW}")])


class Cli(unittest.TestCase):
    """VALIDATES: the default is a dry run and bad module paths are rejected.
    PREVENTS: an accidental repo-wide rewrite from a mistyped invocation."""

    def repo(self, tmp: str) -> Path:
        root = Path(tmp)
        write(root, "go.mod", f"module {OLD}\n\ngo 1.26\n")
        write(root, "internal/a.go", f'import "{OLD}/internal/x"\n')
        write(root, f"gokrazy/ze/builddir/{OLD}/go.mod", f"require {OLD} v0.0.0\n")
        write(root, "README.md", f"clone https://{OLD}.git\n")
        env = {**os.environ, "GIT_CONFIG_GLOBAL": str(root / ".gitconfig")}
        subprocess.run(["git", "init", "-q"], cwd=root, check=True, env=env)
        subprocess.run(
            ["git", "add", "-A"], cwd=root, check=True, env=env
        )  # fixture repo, not the working tree
        return root

    def test_dry_run_changes_nothing(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self.repo(tmp)
            before = (root / "internal/a.go").read_text()
            rc = rmp.main(["--repo", str(root), "--to", NEW])
            self.assertEqual(rc, 0)
            self.assertEqual((root / "internal/a.go").read_text(), before)
            self.assertTrue((root / f"gokrazy/ze/builddir/{OLD}").is_dir())

    def test_apply_renames_everything(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self.repo(tmp)
            rc = rmp.main(
                ["--repo", str(root), "--to", NEW, "--apply", "--no-goimports"]
            )
            self.assertEqual(rc, 0)
            self.assertEqual(
                (root / "go.mod").read_text().splitlines()[0], f"module {NEW}"
            )
            self.assertIn(NEW, (root / "internal/a.go").read_text())
            self.assertNotIn(OLD, (root / "internal/a.go").read_text())
            self.assertTrue((root / f"gokrazy/ze/builddir/{NEW}/go.mod").is_file())

    def test_old_module_is_read_from_go_mod(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self.repo(tmp)
            self.assertEqual(rmp.current_module(root), OLD)

    def test_a_non_module_argument_is_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self.repo(tmp)
            self.assertEqual(rmp.main(["--repo", str(root), "--to", "github"]), 2)
            self.assertEqual(
                rmp.main(["--repo", str(root), "--to", "https://github.com/a/b"]), 2
            )
            self.assertEqual(
                (root / "go.mod").read_text().splitlines()[0], f"module {OLD}"
            )

    def test_renaming_to_itself_is_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self.repo(tmp)
            self.assertEqual(rmp.main(["--repo", str(root), "--to", OLD]), 2)


class RenameOnlyProof(unittest.TestCase):
    """VALIDATES: the proof that gates the rfc/audit re-stamp accepts a pure
    rename and rejects anything else.
    PREVENTS: re-sealing an audit verdict over a real edit, which would leave a
    stale assurance wearing the badge of a fresh one -- the exact failure the
    fingerprint exists to catch."""

    def repo(self, tmp: str, committed: str, current: str) -> Path:
        root = Path(tmp)
        env = {**os.environ, "GIT_CONFIG_GLOBAL": str(root / ".gitconfig")}
        write(root, "x_test.go", committed)
        subprocess.run(["git", "init", "-q"], cwd=root, check=True, env=env)
        subprocess.run(["git", "add", "-A"], cwd=root, check=True, env=env)
        subprocess.run(
            ["git", "-c", "user.email=t@e", "-c", "user.name=t", "commit", "-qm", "x"],
            cwd=root,
            check=True,
            env=env,
        )
        write(root, "x_test.go", current)
        return root

    def test_a_pure_rename_is_proven(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self.repo(
                tmp, f'import "{OLD}/internal/x"\n', f'import "{NEW}/internal/x"\n'
            )
            self.assertTrue(rmp.rename_only_since_head(root, "x_test.go", OLD, NEW))

    def test_an_edited_assertion_is_not_proven(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self.repo(
                tmp,
                f'import "{OLD}/x"\nrequire.Equal(t, 3, got)\n',
                f'import "{NEW}/x"\nrequire.Equal(t, 4, got)\n',
            )
            self.assertFalse(rmp.rename_only_since_head(root, "x_test.go", OLD, NEW))

    def test_a_removed_assertion_is_not_proven(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self.repo(
                tmp,
                f'import "{OLD}/x"\nrequire.NoError(t, err)\nrequire.Equal(t, 3, got)\n',
                f'import "{NEW}/x"\nrequire.NoError(t, err)\n',
            )
            self.assertFalse(rmp.rename_only_since_head(root, "x_test.go", OLD, NEW))

    def test_reformatting_that_moves_no_code_is_proven(self):
        # The fingerprint's own normalisation drops blank lines and indentation,
        # so a goimports re-sort that regroups without moving a line still counts
        # as unchanged. Matching that exactly is why the tool imports it.
        with tempfile.TemporaryDirectory() as tmp:
            root = self.repo(
                tmp,
                f'import (\n\t"{OLD}/a"\n\n\t"{OLD}/b"\n)\n',
                f'import (\n    "{NEW}/a"\n    "{NEW}/b"\n)\n',
            )
            self.assertTrue(rmp.rename_only_since_head(root, "x_test.go", OLD, NEW))

    def test_a_file_absent_from_head_is_not_proven(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self.repo(tmp, "package x\n", "package x\n")
            write(root, "new_test.go", f'import "{NEW}/x"\n')
            self.assertFalse(rmp.rename_only_since_head(root, "new_test.go", OLD, NEW))


class Residuals(unittest.TestCase):
    """VALIDATES: leftover references to the old HOST are surfaced.
    PREVENTS: a rename that reports success while README still tells people to
    clone from the host we left."""

    def test_hosting_urls_are_reported(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, "README.md", f"clone https://{OLD}.git\n")
            env = {**os.environ, "GIT_CONFIG_GLOBAL": str(root / ".gitconfig")}
            subprocess.run(["git", "init", "-q"], cwd=root, check=True, env=env)
            subprocess.run(["git", "add", "-A"], cwd=root, check=True, env=env)
            self.assertEqual(rmp.residual_report(root, OLD), [("README.md", 1)])


# The re-stamp boundary the rename tool must NOT re-implement: forbidden token -> the live
# spelling in the gate that proves the token still names something reachable.
#
# ONE list, consumed by BOTH assertions below. They used to hold independent hardcoded copies of
# these tokens, which made the vacuity guard blind to the exact recurrence it was added to catch:
# a dead token in the absence guard's own list. Reproduced before this was derived -- adding
# `verdict_is_fresh(` back as a fourth forbidden token left the module at `Ran 42 tests ... OK`,
# because the liveness check iterated its own separate copy and never saw the new entry
# (`ai/rules/evidence.md`, applied to a test's own data).
#
# Every probe must also be LOAD-BEARING -- a spelling the gate cannot satisfy in prose. See
# RESEAL_RULE_WRITES below for the one that was not.

# Forbidden tokens that name a gate FUNCTION. Their liveness probe is DERIVED as `def <token>`,
# so a token and its probe cannot be mis-paired and a token whose function is deleted reds by
# itself rather than needing someone to remember to re-point a second list.
RESEAL_RULE_FUNCTIONS = (
    # DECIDING freshness -- both spellings of the boundary. `verdict_freshness(` is the per-verdict
    # rule; `audit_freshness(` is the per-RFC wrapper that the gate's own `reseal_audits` calls
    # (`scripts/dev/rfc_requirements.py:2687`), and it is the likelier route for a copy-paste
    # re-implementation, which is why forbidding only the inner one left the boundary crossable.
    "verdict_freshness(",
    "audit_freshness(",
    # COMPUTING a fingerprint. `unit_shas(` is the unit-level producer `audit_freshness` calls;
    # `tagged_unit_shas(` is the file-level one. A re-implementation reaching for either is a
    # second copy of the hash rule.
    "unit_shas(",
    "tagged_unit_shas(",
)

# The one forbidden token that is not a call but a WRITE to the recorded fingerprint map, so its
# probe cannot be derived and is given explicitly -- which is exactly why it has to be chosen
# carefully. The probe was once the bare word `"tests"`, which cannot falsify: it occurs NINE times
# in the gate (the schema key set, docstrings, error messages), so renaming the schema field at its
# load-bearing sites -- the tuple, the `recorded_map` lookup, the write -- left SIX prose hits
# keeping this green while `verdict["tests"] = ` had become unmatchable. `_FINGERPRINT_MAPS =
# ("tests"` is the tuple the gate iterates to walk the fingerprint maps: it cannot survive that
# rename, and unlike a bare word it cannot be written in a sentence.
RESEAL_RULE_WRITES = {
    'verdict["tests"] = ': '_FINGERPRINT_MAPS = ("tests"',
}

RESEAL_RULE_TOKENS = {
    **{token: "def " + token for token in RESEAL_RULE_FUNCTIONS},
    **RESEAL_RULE_WRITES,
}


class ResealDelegates(unittest.TestCase):
    """VALIDATES: the rename tool CALLS the gate's shared re-stamp and adds its own per-file
    proof, rather than owning a second copy of the fingerprint rule.
    PREVENTS: the hazard reseal_rfc_audits' own docstring named -- a second copy of the rule that
    drifted would re-seal verdicts against a hash the gate does not compute
    (plan/spec-rfcgate-3-audit-teeth.md AC-22). Also pins A-1: the tool never touches a
    JUDGEMENT field."""

    def test_it_delegates_to_the_gate_and_passes_its_own_proof(self):
        calls = {}

        class FakeGate:
            @staticmethod
            def reseal_audits(prove=None, note=None):
                calls["prove"] = prove
                calls["note"] = note
                return ["rfc7606 RFC7606-2-1"], []

        original = rmp._rfc_requirements
        rmp._rfc_requirements = lambda root: FakeGate
        try:
            resealed, refused = rmp.reseal_rfc_audits(Path("/nowhere"), OLD, NEW)
        finally:
            rmp._rfc_requirements = original

        self.assertEqual((resealed, refused), (["rfc7606 RFC7606-2-1"], []))
        self.assertTrue(
            callable(calls["prove"]), "the rename-specific proof must be passed in"
        )
        self.assertIn(OLD, calls["note"])
        self.assertIn(NEW, calls["note"])
        self.assertIn("rename_only_since_head", calls["note"])

    def test_the_proof_it_passes_is_rename_only_since_head(self):
        """The predicate must be the real proof, not a lambda that says yes."""
        seen = []
        # rename_only_since_head reads the gate's own `_normalize` through the SAME accessor this
        # test replaces, so the stub has to keep supplying it -- otherwise the predicate raises,
        # reseal_rfc_audits catches, and the test would pass by never running the predicate at all.
        real_normalize = rmp._rfc_requirements(Path(rmp.__file__).parents[2])._normalize

        class FakeGate:
            _normalize = staticmethod(real_normalize)

            @staticmethod
            def reseal_audits(prove=None, note=None):
                seen.append(prove("x_test.go"))
                return [], []

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            env = {**os.environ, "GIT_CONFIG_GLOBAL": str(root / ".gitconfig")}
            write(root, "x_test.go", f'import "{OLD}/x"\nrequire.Equal(t, 3, got)\n')
            subprocess.run(["git", "init", "-q"], cwd=root, check=True, env=env)
            subprocess.run(["git", "add", "-A"], cwd=root, check=True, env=env)
            subprocess.run(
                [
                    "git",
                    "-c",
                    "user.email=t@e",
                    "-c",
                    "user.name=t",
                    "commit",
                    "-qm",
                    "x",
                ],
                cwd=root,
                check=True,
                env=env,
            )
            # A real edit beside the rename: the proof must say NO.
            write(root, "x_test.go", f'import "{NEW}/x"\nrequire.Equal(t, 4, got)\n')
            original = rmp._rfc_requirements
            rmp._rfc_requirements = lambda r: FakeGate
            try:
                rmp.reseal_rfc_audits(root, OLD, NEW)
            finally:
                rmp._rfc_requirements = original
        self.assertEqual(seen, [False], "the predicate is not rename_only_since_head")

    def test_it_owns_no_second_copy_of_the_rule(self):
        """AC-22 mechanically: exactly one definition of the re-stamp in the tree.

        `RESEAL_RULE_TOKENS` names the three ingredients of a re-implementation: DECIDING
        freshness, COMPUTING a fingerprint, and WRITING one. The tool may do none of them; it may
        only pass its extra `prove` predicate to `reseal_audits`.

        `verdict_freshness(` replaced `verdict_is_fresh(` there when that second, dead spelling of
        the freshness rule was deleted from the gate. Re-pointed rather than dropped: a token that
        can no longer appear anywhere in the tree asserts nothing, and the BOUNDARY is still
        crossable -- the successor function is importable, so a future edit can still decide its
        own re-stamp set and bypass the shared loop. The symbol going away is not the coupling
        becoming impossible.
        """
        src = Path(rmp.__file__).read_text(encoding="utf-8")
        self.assertIn("reseal_audits(", src)
        for gone in RESEAL_RULE_TOKENS:
            self.assertNotIn(
                gone, src, f"the rename tool still re-implements the rule ({gone})"
            )

    def test_each_forbidden_token_names_something_that_still_exists(self):
        """The vacuity guard on the guard above. An absence assertion is only protection while the
        thing it forbids is reachable: `verdict_is_fresh(` sat in that list after the function was
        deleted, so a third of the check was satisfied by nothing at all. Each token must still
        name a live symbol or a live schema field in the gate.

        It reads the SAME `RESEAL_RULE_TOKENS` the guard above iterates, which is the whole point:
        while the two held separate copies, a dead token added to the guard's list was invisible
        here, and this check could not fail for the one case it exists to catch.
        """
        gate = Path(rmp.__file__).parent / "rfc_requirements.py"
        gate_src = gate.read_text(encoding="utf-8")
        for token, live in RESEAL_RULE_TOKENS.items():
            # assertTrue, not assertIn: the haystack is the whole 5,600-line gate, and assertIn
            # prints it on failure (257KB), burying the one line that says what to do.
            self.assertTrue(
                live in gate_src,
                f"{token!r} forbids something the gate no longer has ({live!r} is absent): "
                f"re-point it at the live spelling of the boundary, or the assertion cannot fail",
            )

    def test_a_gate_that_cannot_be_imported_is_reported_not_raised(self):
        original = rmp._rfc_requirements
        rmp._rfc_requirements = lambda root: None
        try:
            resealed, refused = rmp.reseal_rfc_audits(Path("/nowhere"), OLD, NEW)
        finally:
            rmp._rfc_requirements = original
        self.assertEqual(resealed, [])
        self.assertTrue(refused)


if __name__ == "__main__":
    unittest.main()
