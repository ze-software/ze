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


if __name__ == "__main__":
    unittest.main()
