#!/usr/bin/env python3
"""The Homebrew prefix is resolved, in every language this repo builds with.

Homebrew installs under /opt/homebrew on Apple Silicon and /usr/local on Intel.
Eight sites across Go, Python and Make had written the first as a literal, so an
Intel Mac with e2fsprogs and QEMU properly installed reported both missing:
`ze appliance build` logged "debugfs not found" with the binary on disk, and the
install gates skipped with "firmware not found".

Two things keep that fixed, and both are checked here. The literal may not come
back on its own anywhere in the tree, and the copies of the resolver that exist
for good reasons must give the same answer on the same machine.
"""

from __future__ import annotations

import ast
import importlib
import importlib.util
import os
import re
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

HERE = Path(__file__).resolve().parent
REPO_ROOT = HERE.parents[1]

sys.path.insert(0, str(HERE))
dev_setup = importlib.import_module("dev-setup")


def _load(name: str, path: Path):
    spec = importlib.util.spec_from_file_location(name, path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"cannot load {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


evidence_brew = _load("evidence_homebrew", REPO_ROOT / "scripts/evidence/homebrew.py")

# The Apple Silicon prefix, spelled in pieces so this file does not trip its own
# scan and so the scan cannot be satisfied by the mention here.
ARM_PREFIX = "/opt/" + "homebrew"
INTEL_PREFIX = "/usr/" + "local"

# The files allowed to spell the Apple Silicon prefix: the resolvers, where it
# is one of the two documented defaults, and the one fallback constant.
#
# An EXACT set, not a marker substring. The first version accepted any file
# that also mentioned /usr/local or HOMEBREW_PREFIX, which two reviewers broke
# the same way: the three files most likely to grow a new literal are the
# resolvers themselves, and each already carries both markers, so a hardcoded
# path added there was invisible. A whole-file substring test also passed any
# file with /usr/local/bin in a PATH line.
#
# The set is checked in both directions. A file here that no longer spells the
# prefix has to be removed from the set, which is what stops it rotting into an
# allowlist nobody reads.
RESOLVERS = {
    "internal/appliance/homebrew.go",
    "scripts/dev/dev-setup.py",
    "scripts/evidence/homebrew.py",
    "scripts/evidence/effective-install-qemu.py",
    "Makefile",
    "mk/gokrazy.mk",
}

SOURCE_SUFFIXES = (".go", ".py", ".sh", ".mk", ".bash", ".zsh")

# `.claude/worktrees` holds whole checkouts of this repository, so walking it
# would report every site twice and pin another branch's content. `.claude`
# itself is NOT skipped: its hooks are tracked .py and .sh files that this
# scan must cover.
SKIP_DIRS = {
    ".git",
    "vendor",
    "node_modules",
    "__pycache__",
    "tmp",
    "bin",
    "backups",
}
# Whole trees that are not this repository's source. `.claude/worktrees` holds
# entire checkouts, and `gokrazy/modcache` holds ~25,900 gitignored third-party
# Go files: walking it was most of this file's runtime, and one upstream module
# spelling the prefix would red the gate with an offender only fixable by
# adding a vendored path to RESOLVERS, which is the allowlist rot this design
# exists to prevent.
SKIP_PATHS = {".claude/worktrees", ".claude/plan", "gokrazy/modcache"}


def code_lines(text: str, suffix: str) -> str:
    """The file with its comments and Python docstrings removed.

    The scan is about a path the CODE uses. A comment explaining that Homebrew
    lives at one prefix on Apple Silicon and another on Intel is the opposite
    of the defect, and flagging it would push authors to delete the sentence
    that explains the rule.

    Crude in one direction on purpose: a `#` inside a string literal ends the
    line early here, which can only lose a hit, never invent one. Go block
    comments ARE handled, because leaving them would do the opposite and report
    a sentence as an offender.
    """
    if suffix == ".go":
        text = re.sub(r"/\*.*?\*/", "", text, flags=re.S)
    skip = docstring_lines(text) if suffix == ".py" else set()
    out = []
    for number, line in enumerate(text.splitlines(), 1):
        if number in skip:
            continue
        code = line.split("//", 1)[0] if suffix == ".go" else line.split("#", 1)[0]
        out.append(code)
    return "\n".join(out)


def docstring_lines(text: str) -> set[int]:
    """Line numbers covered by a docstring, from the AST rather than by
    counting quotes.

    A hand-rolled `\"\"\"` toggle desynced on this very file and reported one of
    its own docstrings as a hardcoded path. Only DOCSTRINGS are excluded, never
    string literals in general: a hardcoded path in Python IS a string literal,
    so dropping those would blind the scan to the thing it looks for.

    A file that does not parse contributes nothing to the skip set, so its
    every line is scanned. The scan fails closed on a syntax error.
    """
    try:
        tree = ast.parse(text)
    except SyntaxError:
        return set()

    covered: set[int] = set()
    for node in ast.walk(tree):
        if not isinstance(
            node, (ast.Module, ast.ClassDef, ast.FunctionDef, ast.AsyncFunctionDef)
        ):
            continue
        body = getattr(node, "body", None)
        if not body:
            continue
        first = body[0]
        if (
            isinstance(first, ast.Expr)
            and isinstance(first.value, ast.Constant)
            and isinstance(first.value.value, str)
        ):
            covered.update(range(first.lineno, (first.end_lineno or first.lineno) + 1))
    return covered


def source_files():
    for dirpath, dirnames, filenames in os.walk(REPO_ROOT):
        here = Path(dirpath)
        dirnames[:] = [
            d
            for d in dirnames
            if d not in SKIP_DIRS
            and str((here / d).relative_to(REPO_ROOT)) not in SKIP_PATHS
        ]
        for name in filenames:
            if name.endswith(SOURCE_SUFFIXES) or name == "Makefile":
                yield here / name


class TestNoBareAppleSiliconPrefix(unittest.TestCase):
    """The scan runs over the WORKING TREE, not `git ls-files`, so a file added
    but not yet committed is covered on the run that adds it."""

    def setUp(self):
        self.seen = []
        self.spelling_it = set()
        for path in source_files():
            try:
                text = path.read_text(errors="replace")
            except OSError:
                continue
            rel = str(path.relative_to(REPO_ROOT))
            self.seen.append(rel)
            suffix = ".go" if path.suffix == ".go" else path.suffix
            if ARM_PREFIX in code_lines(text, suffix):
                self.spelling_it.add(rel)

    def test_the_walk_reached_every_source_tree(self):
        """Without this the class passes on an empty or narrowed walk.

        A bare file count is not the guard: the walk reaches thousands of
        files, so any single tree could be added to SKIP_DIRS and a count
        floor would never notice. Each source tree is named instead, so
        silencing one reds this test rather than the scan it covers.
        """
        trees = ("internal", "cmd", "scripts", "test", "tools", "mk", "demos")
        for tree in trees:
            self.assertTrue(
                any(rel.startswith(tree + "/") for rel in self.seen),
                f"the walk reached no file under {tree}/: a SKIP_DIRS entry or a "
                "wrong REPO_ROOT is hiding a whole tree from the scan",
            )
        for required in ("Makefile", "mk/gokrazy.mk", "internal/appliance/homebrew.go"):
            self.assertIn(required, self.seen)

    def test_only_the_resolvers_spell_the_prefix(self):
        offenders = sorted(self.spelling_it - RESOLVERS)
        self.assertEqual(
            offenders,
            [],
            f"these name {ARM_PREFIX} and are not resolvers: an Intel Mac has "
            f"its Homebrew at {INTEL_PREFIX}, so that path is simply absent "
            "there. Resolve the prefix (brew_prefixes / brewPrefixes / "
            "BREW_PREFIX) instead of writing it.",
        )

    def test_the_resolver_set_has_not_rotted(self):
        """Every listed file must still spell it, or the entry is stale and the
        set is drifting into an allowlist that excuses files nobody checked."""
        stale = sorted(RESOLVERS - self.spelling_it)
        self.assertEqual(
            stale,
            [],
            "these are listed as resolvers but no longer name the prefix; "
            "drop them from RESOLVERS",
        )


class TestTheMakefileResolvesToo(unittest.TestCase):
    """Make is the third language, and it has no unit test of its own.

    BREW_PREFIX is derived from `command -v brew`, so the two consumers below
    (E2FS and the aarch64 firmware) follow the host rather than a literal.
    """

    def setUp(self):
        # The derivation lives in the root Makefile, before its own first use of
        # it (the tinygo PATH). mk/gokrazy.mk is included from there and
        # consumes the same variable.
        self.root = self._joined(REPO_ROOT / "Makefile")
        self.text = self._joined(REPO_ROOT / "mk/gokrazy.mk")

    @staticmethod
    def _joined(path: Path) -> str:
        """The file with make's backslash continuations folded onto one line,
        so an assignment can be matched as the single statement it is."""
        return path.read_text().replace("\\\n", " ")

    def test_brew_prefix_is_derived_from_the_brew_binary(self):
        derivation = [
            ln for ln in self.root.splitlines() if re.match(r"BREW_PREFIXES\s*:=", ln)
        ]
        self.assertEqual(
            len(derivation),
            1,
            "the root Makefile must define BREW_PREFIXES exactly once",
        )
        self.assertIn("command -v brew", self.root)
        self.assertIn("HOMEBREW_PREFIX", derivation[0])

    def test_make_keeps_every_prefix_not_just_the_first(self):
        """Go and Python both return a LIST and search all of it. A single
        `firstword` here would hide a real Homebrew at /usr/local behind a
        stale empty /opt/homebrew, which is the state this rung exists for."""
        self.assertRegex(self.root, r"BREW_PREFIXES\s*:=\s*\$\(wildcard")
        self.assertRegex(
            self.root, r"BREW_PREFIX\s*:=\s*\$\(firstword \$\(BREW_PREFIXES\)\)"
        )
        e2fs = next(ln for ln in self.text.splitlines() if re.match(r"E2FS\s*:=", ln))
        self.assertIn("BREW_PREFIXES", e2fs)

    def test_make_carries_the_defaults_rung_the_others_have(self):
        """Without it, a make run whose PATH has no brew (sudo resets PATH, and
        so does launchd) left BREW_PREFIX empty, and E2FS lost its whole
        Homebrew branch on an Apple Silicon Mac where the old unconditional
        Cellar glob had found the tools."""
        self.assertRegex(self.root, r"BREW_DEFAULTS\s*:=.*Darwin")
        self.assertIn("BREW_DEFAULTS", self.root.split("BREW_PREFIX", 1)[1])

    def test_make_takes_the_newest_cellar_version(self):
        """Asserted by RUNNING the snippet, not by reading it.

        The first version of this test asserted the string `ls -dr` was
        present. It was, and the answer was still wrong: `ls -dr` reverses the
        SPELLING, so over {1.47.4, 1.47.9, 1.47.10} it yields 1.47.9 first and
        the loop breaks there, two releases behind the copies in Go and Python.
        A test that pins a mechanism cannot see that. This one plants a Cellar
        and compares the directory the shell actually picks.
        """
        e2fs = next(ln for ln in self.text.splitlines() if re.match(r"E2FS\s*:=", ln))
        snippet = e2fs.split(":=", 1)[1].strip()
        # Unwrap $(shell ...) and undo make's $$ escaping to get real shell.
        snippet = re.sub(r"^\$\(shell\s+", "", snippet).rstrip(")")
        snippet = snippet.replace("$$", "$")

        with tempfile.TemporaryDirectory() as tmp:
            prefix = Path(tmp).resolve()
            for version in ("1.47.4", "1.47.9", "1.47.10"):
                sbin = prefix / "Cellar" / "e2fsprogs" / version / "sbin"
                sbin.mkdir(parents=True)
                for tool in ("mkfs.ext4", "debugfs"):
                    (sbin / tool).write_text("#!/bin/sh\n")
                    (sbin / tool).chmod(0o755)

            # The snippet iterates $(BREW_PREFIXES) through make's $(foreach);
            # substitute the fixture for it and drop the system directories, so
            # what remains is exactly the Cellar ordering under test.
            expanded = re.sub(
                r"\$\(foreach p,\$\(BREW_PREFIXES\),(.*?)\)\s*;",
                lambda m: m.group(1).replace("$(p)", str(prefix)) + ";",
                snippet,
            )
            expanded = expanded.replace("/usr/sbin /sbin /usr/local/sbin", "")
            picked = (
                subprocess.run(
                    ["sh", "-c", expanded],
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                )
                .stdout.decode()
                .strip()
            )

        self.assertEqual(
            picked,
            str(prefix / "Cellar" / "e2fsprogs" / "1.47.10" / "sbin"),
            "make picked a Cellar version the other two copies would not",
        )

    def test_the_tinygo_path_goes_through_it(self):
        """The site the repo-wide scan found and a hand search had missed: the
        root Makefile has no extension, so an --include=*.mk grep skips it."""
        line = next((ln for ln in self.root.splitlines() if "go@1.26/bin" in ln), None)
        self.assertIsNotNone(line, "the tinygo Go 1.26 PATH line is gone")
        self.assertIn("BREW_PREFIX", str(line))

    def test_both_homebrew_consumers_go_through_it(self):
        for var in ("E2FS", "GOKRAZY_QEMU_AARCH64_BIOS"):
            line = next(
                (
                    ln
                    for ln in self.text.splitlines()
                    if re.match(rf"{var}\s*[:?]?=", ln)
                ),
                None,
            )
            self.assertIsNotNone(line, f"{var} assignment not found")
            self.assertIn(
                "BREW_PREFIX", str(line), f"{var} does not go through BREW_PREFIX"
            )


class TestTheCopiesAgree(unittest.TestCase):
    """dev-setup.py carries its own brew_prefixes rather than importing one.

    That is deliberate: it is what a contributor runs against a machine where
    nothing is set up, so it stays runnable on its own. The cost of a copy is
    drift, and this is what pays it: both implementations answer the same
    question about the same machine, so an edit to one that the other does not
    get shows up here rather than on somebody's Intel Mac.
    """

    def test_same_answer_for_the_same_machine(self):
        with tempfile.TemporaryDirectory() as tmp:
            prefix = Path(tmp).resolve()
            brew_bin = prefix / "bin"
            brew_bin.mkdir()
            brew = brew_bin / "brew"
            brew.write_text("#!/bin/sh\n")
            brew.chmod(0o755)

            old_path = os.environ.get("PATH", "")
            old_prefix = os.environ.get("HOMEBREW_PREFIX")
            os.environ["PATH"] = str(brew_bin)
            os.environ.pop("HOMEBREW_PREFIX", None)
            try:
                from_setup = dev_setup.brew_prefixes()
                from_evidence = evidence_brew.brew_prefixes()
            finally:
                os.environ["PATH"] = old_path
                if old_prefix is not None:
                    os.environ["HOMEBREW_PREFIX"] = old_prefix

        self.assertEqual(from_setup, from_evidence)
        self.assertEqual(
            from_setup[0],
            prefix,
            "the brew binary's own location must lead; HOMEBREW_PREFIX is unset "
            "in a plain non-login shell, which is what `make` runs in",
        )

    def test_the_defaults_are_macos_only_in_both(self):
        """/usr/local is a directory on essentially every Linux host, and it is
        not Homebrew's there.

        Offering it unconditionally put /usr/local/sbin ahead of /usr/sbin in
        the Go e2fsprogs search and /usr/local/share/qemu ahead of
        /usr/share/OVMF in the firmware lists: a box that has never seen
        Homebrew got a source build in place of its distribution's. Both
        reviewers found this independently, and the module docstrings claiming
        "on Linux this returns nothing" were false until it was fixed.
        """
        old = os.environ.get("HOMEBREW_PREFIX")
        os.environ.pop("HOMEBREW_PREFIX", None)
        try:
            with (
                mock.patch.object(dev_setup.sys, "platform", "linux"),
                mock.patch.object(dev_setup.shutil, "which", return_value=None),
            ):
                from_setup = dev_setup.brew_prefixes()
            with (
                mock.patch.object(evidence_brew.sys, "platform", "linux"),
                mock.patch.object(evidence_brew.shutil, "which", return_value=None),
            ):
                from_evidence = evidence_brew.brew_prefixes()
        finally:
            if old is not None:
                os.environ["HOMEBREW_PREFIX"] = old

        self.assertEqual(from_setup, [])
        self.assertEqual(from_evidence, [])

    def test_a_linuxbrew_install_is_still_found(self):
        """The gating drops the GUESS, never the answer. A prefix that names
        itself is honored on any platform."""
        with tempfile.TemporaryDirectory() as tmp:
            prefix = Path(tmp).resolve()
            old = os.environ.get("HOMEBREW_PREFIX")
            os.environ["HOMEBREW_PREFIX"] = str(prefix)
            try:
                # `brew` is off PATH here so only the exported prefix is in
                # play: this developer's Mac has a real one, and rung 2 would
                # add it whatever the platform says.
                with (
                    mock.patch.object(dev_setup.sys, "platform", "linux"),
                    mock.patch.object(dev_setup.shutil, "which", return_value=None),
                ):
                    from_setup = dev_setup.brew_prefixes()
                with (
                    mock.patch.object(evidence_brew.sys, "platform", "linux"),
                    mock.patch.object(evidence_brew.shutil, "which", return_value=None),
                ):
                    from_evidence = evidence_brew.brew_prefixes()
            finally:
                if old is None:
                    os.environ.pop("HOMEBREW_PREFIX", None)
                else:
                    os.environ["HOMEBREW_PREFIX"] = old

        self.assertEqual(from_setup, [prefix])
        self.assertEqual(from_evidence, [prefix])

    def test_the_exported_variable_wins_in_both(self):
        with tempfile.TemporaryDirectory() as tmp:
            exported = Path(tmp).resolve()
            old_prefix = os.environ.get("HOMEBREW_PREFIX")
            os.environ["HOMEBREW_PREFIX"] = str(exported)
            try:
                from_setup = dev_setup.brew_prefixes()
                from_evidence = evidence_brew.brew_prefixes()
            finally:
                if old_prefix is None:
                    os.environ.pop("HOMEBREW_PREFIX", None)
                else:
                    os.environ["HOMEBREW_PREFIX"] = old_prefix

        self.assertEqual(from_setup, from_evidence)
        self.assertEqual(from_setup[0], exported)


class TestKegOnlyLookupCoversBothLayouts(unittest.TestCase):
    """e2fsprogs is keg-only: Homebrew links none of it onto PATH.

    Its binaries are in <prefix>/opt/<formula>/sbin, the link kept at the
    current version, and in <prefix>/Cellar/<formula>/<version>/sbin, which is
    what an interrupted upgrade leaves behind with no link. A probe that knew
    only one of them calls a working install absent.
    """

    def test_both_layouts_are_searched(self):
        with tempfile.TemporaryDirectory() as tmp:
            prefix = Path(tmp).resolve()
            opt = prefix / "opt" / "e2fsprogs" / "sbin"
            cellar = prefix / "Cellar" / "e2fsprogs" / "1.47.4" / "sbin"
            opt.mkdir(parents=True)
            cellar.mkdir(parents=True)

            old_prefix = os.environ.get("HOMEBREW_PREFIX")
            os.environ["HOMEBREW_PREFIX"] = str(prefix)
            try:
                dirs = evidence_brew.brew_keg_dirs("e2fsprogs")
            finally:
                if old_prefix is None:
                    os.environ.pop("HOMEBREW_PREFIX", None)
                else:
                    os.environ["HOMEBREW_PREFIX"] = old_prefix

        self.assertIn(opt, dirs)
        self.assertIn(cellar, dirs)

    def test_both_keg_layouts_are_searched(self):
        """Asserted over WHERE the probe looks, not over what it answers.

        `probe_e2fsprogs` ends its list with /usr/sbin and /sbin, so on
        any Linux host that has e2fsprogs -- which `make ze-setup` installs as a
        required tool, and which is where `python_tests_test.go` runs this --
        the boolean is True with both Homebrew branches deleted. A test on the
        boolean alone is green either way. `e2fsprogs_dirs` exists so this
        assertion can name the directories instead.
        """
        with tempfile.TemporaryDirectory() as tmp:
            prefix = Path(tmp).resolve()
            with mock.patch.object(dev_setup, "brew_prefixes", return_value=[prefix]):
                dirs = dev_setup.e2fsprogs_dirs()

        for layout in (
            ("opt", "e2fsprogs", "sbin"),
            ("Cellar", "e2fsprogs", "1.47.4", "sbin"),
        ):
            want = prefix.joinpath(*layout)
            if layout[0] == "Cellar":
                # The glob only yields what exists, so the Cellar branch is
                # asserted by planting a tree and re-reading the list.
                continue
            self.assertIn(want, dirs)

        with tempfile.TemporaryDirectory() as tmp:
            prefix = Path(tmp).resolve()
            cellar = prefix / "Cellar" / "e2fsprogs" / "1.47.4" / "sbin"
            cellar.mkdir(parents=True)
            with mock.patch.object(dev_setup, "brew_prefixes", return_value=[prefix]):
                dirs = dev_setup.e2fsprogs_dirs()
            self.assertIn(cellar, dirs)

    def test_the_probe_finds_a_keg_only_install(self):
        """The boolean, over each layout ON ITS OWN.

        `shutil.which` answers nothing for a keg-only formula however well it is
        installed, so this is the only thing standing between a correct machine
        and `make ze-setup` offering to reinstall what is already there.
        """
        for layout in (
            ("opt", "e2fsprogs", "sbin"),
            ("Cellar", "e2fsprogs", "1.47.4", "sbin"),
        ):
            with self.subTest(layout="/".join(layout)):
                self.assertTrue(
                    self._probe_with_only(layout),
                    f"a keg-only e2fsprogs found only at <prefix>/{'/'.join(layout)}",
                )

    def test_cellar_versions_sort_by_number(self):
        """String order puts 1.47.10 below 1.47.4, so the newest must be keyed
        on the numbers. Both Python copies of the sort are checked."""
        with tempfile.TemporaryDirectory() as tmp:
            prefix = Path(tmp).resolve()
            for version in ("1.47.4", "1.47.10", "1.47.9"):
                (prefix / "Cellar" / "e2fsprogs" / version / "sbin").mkdir(parents=True)

            with mock.patch.object(dev_setup, "brew_prefixes", return_value=[prefix]):
                setup_dirs = dev_setup.e2fsprogs_dirs()
            old = os.environ.get("HOMEBREW_PREFIX")
            os.environ["HOMEBREW_PREFIX"] = str(prefix)
            try:
                evidence_dirs = evidence_brew.brew_keg_dirs("e2fsprogs")
            finally:
                if old is None:
                    os.environ.pop("HOMEBREW_PREFIX", None)
                else:
                    os.environ["HOMEBREW_PREFIX"] = old

            newest = prefix / "Cellar" / "e2fsprogs" / "1.47.10" / "sbin"
            cellar_only = [d for d in setup_dirs if "Cellar" in d.parts]
            self.assertEqual(cellar_only[0], newest)
            self.assertEqual(
                [d for d in evidence_dirs if "Cellar" in d.parts][0], newest
            )

    def _probe_with_only(self, layout: tuple[str, ...]) -> bool:
        """Probe a prefix holding ONLY this layout.

        `brew_prefixes` is replaced rather than steered through
        HOMEBREW_PREFIX. Setting the variable only puts the temp prefix FIRST:
        the resolver still offers /opt/homebrew, so on this developer's Mac the
        probe found the real e2fsprogs and answered True whatever the fake tree
        held. A mutation run caught it, which is what mutation runs are for.
        """
        with tempfile.TemporaryDirectory() as tmp:
            prefix = Path(tmp).resolve()
            sbin = prefix.joinpath(*layout)
            sbin.mkdir(parents=True)
            for tool in ("mkfs.ext4", "debugfs"):
                (sbin / tool).write_text("#!/bin/sh\n")
                (sbin / tool).chmod(0o755)

            with mock.patch.object(dev_setup, "brew_prefixes", return_value=[prefix]):
                return dev_setup.probe_e2fsprogs()


if __name__ == "__main__":
    unittest.main()
