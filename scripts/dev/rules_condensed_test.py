#!/usr/bin/env python3
"""Tests for the rule-digest generator's two artifacts.

Run: python3 scripts/dev/rules_condensed_test.py

Picked up automatically by `TestPythonUnitTests` (scripts/dev/python_tests_test.go),
which globs `*_test.py` under every root in `pythonTestRoots`.

`rules_condensed.py` emits two files from ONE parse of `ai/rules/*.md`:

  TRIGGERS.md   one routing line per rule, all of them, always loaded
  CORE.md       the directives of the rules that must never sit behind a trigger

The structural tests below build their own tiny corpus, so a rule edited
elsewhere never turns them red. The two that MUST read the live tree say so in
their names and docstrings: the payload budget (AC-5) and the precedence-ladder
membership (AC-3) are claims about this repository, not about the algorithm.
"""

import contextlib
import io
import os
import re
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import rules_condensed

ROOT = Path(__file__).resolve().parents[2]
RULES_DIR = ROOT / "ai" / "rules"


def write_rule(rules_dir, name, trigger, severity, body="- do the thing\n"):
    (rules_dir / name).write_text(
        f"# {name}\n**When:** {trigger}\n**Severity:** {severity}\n\n## Directives\n{body}",
        encoding="utf-8",
    )


def write_precedence(rules_dir, rung1, rung2):
    """A minimal rule-precedence.md carrying only the ladder table."""
    r1 = ", ".join(f"`{s}`" for s in rung1)
    r2 = ", ".join(f"`{s}`" for s in rung2)
    (rules_dir / "rule-precedence.md").write_text(
        "# Rule Precedence\n"
        "**When:** when two rules point in different directions\n"
        "**Severity:** blocking\n\n"
        "## Directives\n\n"
        "| Rung | Governs | Rules | What it does |\n"
        "|------|---------|-------|--------------|\n"
        f"| 1 | Irreversible action | {r1} | STOP and ask |\n"
        f"| 2 | Outside-facing correctness | {r2} | Implement it |\n"
        "| 3 | Scope integrity | `no-parking` | Never reduce scope |\n",
        encoding="utf-8",
    )


class TriggerIndexTest(unittest.TestCase):
    """AC-1, AC-2, AC-4: the index names every rule, routably, on every generate."""

    def test_triggers_cover_every_rule(self):
        """AC-1/AC-4: one line per rule, and a NEW rule needs no second edit."""
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            write_precedence(rules_dir, ["never-destroy-work"], ["rfc-compliance"])
            write_rule(
                rules_dir, "never-destroy-work.md", "before deleting", "blocking"
            )
            write_rule(
                rules_dir, "rfc-compliance.md", "when writing protocol", "blocking"
            )
            write_rule(rules_dir, "alpha.md", "when editing alpha files", "advisory")

            entries = rules_condensed.trigger_lines(
                rules_condensed.load_rules(rules_dir)
            )
            self.assertEqual(len(entries), 4)

            # AC-4: a rule added under ai/rules/ appears on the next generate
            # with no other edit -- no list anywhere names it.
            write_rule(
                rules_dir, "brand-new.md", "when doing something new", "blocking"
            )
            entries = rules_condensed.trigger_lines(
                rules_condensed.load_rules(rules_dir)
            )
            self.assertEqual(len(entries), 5)
            self.assertTrue(any("brand-new.md" in line for line in entries))

    def test_trigger_line_has_path_when_severity(self):
        """AC-2: every line carries the path, the trigger and the severity."""
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            write_precedence(rules_dir, ["never-destroy-work"], ["rfc-compliance"])
            write_rule(
                rules_dir, "never-destroy-work.md", "before deleting", "blocking"
            )
            write_rule(
                rules_dir, "rfc-compliance.md", "when writing protocol", "blocking"
            )
            write_rule(rules_dir, "alpha.md", "when editing alpha files", "advisory")

            for line in rules_condensed.trigger_lines(
                rules_condensed.load_rules(rules_dir)
            ):
                cells = [c.strip() for c in line.strip().strip("|").split("|")]
                self.assertEqual(len(cells), 3, line)
                path, severity, trigger = cells
                self.assertRegex(path, r"^`ai/rules/[a-z0-9-]+\.md`$")
                self.assertIn(severity.split(",")[0].strip(), ("blocking", "advisory"))
                self.assertGreater(len(trigger), 0, line)

    def test_core_rows_are_marked_always_on(self):
        """The index's header promises the reader can see which bodies are loaded."""
        rules = rules_condensed.load_rules(RULES_DIR)
        core = {r["name"] for r in rules_condensed.core_members(rules)}
        self.assertTrue(core)
        marked = set()
        for line in rules_condensed.trigger_lines(rules, core):
            if "always-on" in line:
                marked.add(line.split("`")[1].removeprefix("ai/rules/"))
        self.assertEqual(marked, core)

    def test_trigger_line_length_boundary(self):
        """Boundary: 200 characters is valid, 201 is not, so a long trigger is cut."""
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            write_precedence(rules_dir, ["never-destroy-work"], ["rfc-compliance"])
            write_rule(
                rules_dir, "never-destroy-work.md", "before deleting", "blocking"
            )
            write_rule(
                rules_dir, "rfc-compliance.md", "when writing protocol", "blocking"
            )
            write_rule(
                rules_dir,
                "verbose.md",
                "when " + ("a very long clause about editing files " * 12),
                "blocking",
            )
            for line in rules_condensed.trigger_lines(
                rules_condensed.load_rules(rules_dir)
            ):
                self.assertLessEqual(len(line), rules_condensed.MAX_TRIGGER_LINE)

    def test_live_trigger_index_covers_every_live_rule(self):
        """The live tree: every rule present, every line inside the budget."""
        rules = rules_condensed.load_rules(RULES_DIR)
        lines = rules_condensed.trigger_lines(rules)
        self.assertEqual(len(lines), len(rules))
        self.assertGreaterEqual(len(lines), 20)
        for line in lines:
            self.assertLessEqual(len(line), rules_condensed.MAX_TRIGGER_LINE, line)


class CoreMembershipTest(unittest.TestCase):
    """AC-3: the always-on set is derived from the precedence ladder."""

    def test_core_contains_every_precedence_rung_1_and_2_rule(self):
        """Every rung-1 and rung-2 rule named in the ladder is in the core."""
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            write_precedence(
                rules_dir,
                ["never-destroy-work", "git-safety"],
                ["rfc-compliance", "interop-and-goal-validation"],
            )
            for stem in (
                "never-destroy-work",
                "git-safety",
                "rfc-compliance",
                "interop-and-goal-validation",
            ):
                write_rule(rules_dir, f"{stem}.md", "when doing the thing", "blocking")
            write_rule(rules_dir, "completion.md", "when blocked", "blocking")
            write_rule(rules_dir, "alpha.md", "when editing alpha files", "advisory")

            core = rules_condensed.core_members(rules_condensed.load_rules(rules_dir))
            names = {r["name"] for r in core}
            for stem in (
                "never-destroy-work",
                "git-safety",
                "rfc-compliance",
                "interop-and-goal-validation",
            ):
                self.assertIn(f"{stem}.md", names)
            # Rung 3 is NOT automatically eager: it is reachable through its
            # trigger like every other routed rule.
            self.assertNotIn("completion.md", names)

    def test_core_contains_live_rung_1_and_2_rules(self):
        """The live ladder: the four named rule files are eager in this repo."""
        core = rules_condensed.core_members(rules_condensed.load_rules(RULES_DIR))
        names = {r["name"] for r in core}
        for stem in (
            "never-destroy-work",
            "git-safety",
            "rfc-compliance",
            "interop-and-goal-validation",
        ):
            self.assertIn(f"{stem}.md", names)

    def test_core_is_derived_not_hardcoded(self):
        """AC-3: rename a ladder entry and the core follows it.

        The failure this guards is a filename list in the generator, which reads
        identically to a derivation until the ladder changes underneath it
        (`ai/rules/evidence.md`). The ladder here names rules that
        exist NOWHERE in the real repo, so a hardcoded list cannot fake it.
        """
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            # A ladder naming rules that exist NOWHERE in the real repo.
            write_precedence(rules_dir, ["zebra-guard"], ["yak-shaving"])
            write_rule(rules_dir, "zebra-guard.md", "when guarding zebras", "blocking")
            write_rule(rules_dir, "yak-shaving.md", "when shaving yaks", "blocking")
            write_rule(
                rules_dir, "never-destroy-work.md", "before deleting", "blocking"
            )

            names = {
                r["name"]
                for r in rules_condensed.core_members(
                    rules_condensed.load_rules(rules_dir)
                )
            }
            self.assertIn("zebra-guard.md", names)
            self.assertIn("yak-shaving.md", names)
            # The real repo's rung-1 rule is NOT on this ladder, so it must not
            # be in this core. If it is, the membership came from a hardcoded
            # list rather than from the ladder that was parsed.
            self.assertNotIn("never-destroy-work.md", names)


class LadderRefusalTest(unittest.TestCase):
    """The ladder guard fails closed: every empty parse names its cause.

    Columns are located by HEADER text, which is what makes re-ordering the
    ladder safe and REWORDING it dangerous. Before this, a renamed column left
    `rung_col` unset, every row was skipped, and `git-safety`,
    `never-destroy-work`, `rfc-compliance` and `interop-and-goal-validation`
    left the always-on core with no error at all
    (`ai/rules/evidence.md`).
    """

    def _rules(self, rules_dir):
        write_rule(rules_dir, "never-destroy-work.md", "before deleting", "blocking")
        write_rule(rules_dir, "rfc-compliance.md", "when writing protocol", "blocking")
        return rules_condensed.load_rules(rules_dir)

    def test_reworded_rung_column_refuses(self):
        """`| Rung |` renamed to `| Level |`: refuse, do not return an empty set."""
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            write_precedence(rules_dir, ["never-destroy-work"], ["rfc-compliance"])
            reworded = (rules_dir / "rule-precedence.md").read_text(encoding="utf-8")
            reworded = reworded.replace("| Rung |", "| Level |")
            (rules_dir / "rule-precedence.md").write_text(reworded, encoding="utf-8")

            rules = self._rules(rules_dir)
            with self.assertRaises(rules_condensed.LadderError) as ctx:
                rules_condensed.precedence_rung_slugs(rules)
            self.assertIn("rule-precedence.md", str(ctx.exception))
            self.assertIn("Rung", str(ctx.exception))
            # The refusal must reach the caller that derives the core, not stop
            # at the parser it was raised in.
            with self.assertRaises(rules_condensed.LadderError):
                rules_condensed.core_members(rules)

    def test_reworded_rules_column_refuses(self):
        """The other column is equally load-bearing."""
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            write_precedence(rules_dir, ["never-destroy-work"], ["rfc-compliance"])
            path = rules_dir / "rule-precedence.md"
            path.write_text(
                path.read_text(encoding="utf-8").replace("| Rules |", "| Guards |"),
                encoding="utf-8",
            )
            with self.assertRaises(rules_condensed.LadderError):
                rules_condensed.core_members(self._rules(rules_dir))

    def test_ladder_rewritten_as_a_list_refuses(self):
        """A ladder that is no longer a table has no header row to find."""
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            (rules_dir / "rule-precedence.md").write_text(
                "# Rule Precedence\n"
                "**When:** when two rules point in different directions\n"
                "**Severity:** blocking\n\n"
                "## Directives\n\n"
                "- Rung 1, irreversible action: `never-destroy-work`\n"
                "- Rung 2, outside-facing correctness: `rfc-compliance`\n",
                encoding="utf-8",
            )
            with self.assertRaises(rules_condensed.LadderError):
                rules_condensed.core_members(self._rules(rules_dir))

    def test_absent_ladder_refuses(self):
        """No rule-precedence.md at all: there is nothing to derive the core from."""
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            with self.assertRaises(rules_condensed.LadderError) as ctx:
                rules_condensed.core_members(self._rules(rules_dir))
            self.assertIn("rule-precedence.md", str(ctx.exception))

    def test_ladder_naming_no_known_rule_refuses(self):
        """Rungs 1 and 2 naming only non-rule tokens yield nothing: refuse.

        `CLAUDE.md` is skipped by design (already loaded, not a rule under
        `ai/rules/`). A ladder carrying ONLY such tokens parses cleanly to the
        empty set, which is the same value the reworded-header bug produced.
        """
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            write_precedence(rules_dir, ["CLAUDE"], ["AGENTS"])
            with self.assertRaises(rules_condensed.LadderError) as ctx:
                rules_condensed.core_members(self._rules(rules_dir))
            self.assertIn("names no", str(ctx.exception))

    def test_missing_rung_rows_refuse(self):
        """A table with the right headers but no rung 1/2 rows still refuses."""
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            (rules_dir / "rule-precedence.md").write_text(
                "# Rule Precedence\n"
                "**When:** when two rules point in different directions\n"
                "**Severity:** blocking\n\n"
                "## Directives\n\n"
                "| Rung | Governs | Rules | What it does |\n"
                "|------|---------|-------|--------------|\n"
                "| 3 | Scope integrity | `no-parking` | Never reduce scope |\n",
                encoding="utf-8",
            )
            write_rule(rules_dir, "completion.md", "when blocked", "blocking")
            with self.assertRaises(rules_condensed.LadderError) as ctx:
                rules_condensed.core_members(self._rules(rules_dir))
            self.assertIn("rung 1/2", str(ctx.exception))

    def test_reordered_ladder_columns_still_parse(self):
        """The property the header lookup exists for must survive the refusal.

        Re-ordering is SAFE precisely because columns are found by name. A fix
        that switched to fixed indexes would trade one silent failure for
        another, so this pins that it did not.
        """
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            (rules_dir / "rule-precedence.md").write_text(
                "# Rule Precedence\n"
                "**When:** when two rules point in different directions\n"
                "**Severity:** blocking\n\n"
                "## Directives\n\n"
                "| Governs | Rules | Rung | What it does |\n"
                "|---------|-------|------|--------------|\n"
                "| Irreversible | `never-destroy-work` | 1 | STOP and ask |\n"
                "| Outside-facing | `rfc-compliance` | 2 | Implement it |\n",
                encoding="utf-8",
            )
            slugs, _ = rules_condensed.precedence_rung_slugs(self._rules(rules_dir))
            self.assertEqual(slugs, {"never-destroy-work.md", "rfc-compliance.md"})

    def _fixture_tree(self, td, break_ladder):
        """A miniature repo the generator can be run against.

        It resolves its root as `parents[2]` of its own path, so the script is
        COPIED next to a fixture `ai/rules/` rather than pointed at one.
        """
        root = Path(td)
        script_dir = root / "scripts" / "dev"
        script_dir.mkdir(parents=True)
        here = Path(__file__).parent
        for name in ("rules_condensed.py", "rules_router.py"):
            shutil.copy2(here / name, script_dir / name)

        rules_dir = root / "ai" / "rules"
        rules_dir.mkdir(parents=True)
        write_precedence(rules_dir, ["never-destroy-work"], ["rfc-compliance"])
        if break_ladder:
            path = rules_dir / "rule-precedence.md"
            path.write_text(
                path.read_text(encoding="utf-8").replace("| Rung |", "| Level |"),
                encoding="utf-8",
            )
        self._rules(rules_dir)
        return root, script_dir / "rules_condensed.py"

    def test_generator_exits_non_zero_on_an_unreadable_ladder(self):
        """The refusal reaches the process exit code, and writes nothing."""
        with tempfile.TemporaryDirectory() as td:
            root, script = self._fixture_tree(td, break_ladder=True)
            proc = subprocess.run(
                [sys.executable, str(script)],
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(proc.returncode, 1, proc.stdout + proc.stderr)
            self.assertIn("rule-precedence.md", proc.stderr)
            self.assertIn("Rung", proc.stderr)
            # Nothing is emitted: an artifact whose core silently lost the
            # ladder is worse than no artifact at all.
            self.assertFalse((root / "ai" / "rules" / "CORE.md").exists())

    def test_generator_exits_zero_on_a_readable_ladder(self):
        """The same fixture with the ladder intact succeeds.

        Without this, the test above would pass for any reason the generator
        failed, including a broken fixture.
        """
        with tempfile.TemporaryDirectory() as td:
            root, script = self._fixture_tree(td, break_ladder=False)
            proc = subprocess.run(
                [sys.executable, str(script)],
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
            core = (root / "ai" / "rules" / "CORE.md").read_text(encoding="utf-8")
            self.assertIn("never-destroy-work.md", core)
            self.assertIn("rfc-compliance.md", core)


class CorpusCouplingTest(unittest.TestCase):
    """CORE.md must not move when only the SIZE of the `plan/` corpus moves.

    The header embedded `len(corpus)`, so every spec closure rewrote a generated
    file that no rule edit had touched and left `--check` red. That check sits
    in `ze-regen-check-readonly`, and a structural gate is never a known-red
    (`ai/rules/git-safety.md`).
    """

    def _fixture(self, rules_dir):
        write_precedence(rules_dir, ["never-destroy-work"], ["rfc-compliance"])
        write_rule(rules_dir, "never-destroy-work.md", "before deleting", "blocking")
        write_rule(rules_dir, "rfc-compliance.md", "when writing protocol", "blocking")
        write_rule(rules_dir, "alpha.md", "when editing alpha files", "advisory")

    def test_core_output_ignores_corpus_size(self):
        """Two corpora, different sizes, same membership: byte-identical output."""
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            self._fixture(rules_dir)
            # Neither corpus surfaces any rule, so membership is identical and
            # only the SIZE differs -- which is exactly the churn being removed.
            small = [{"source": "a.md", "text": "unrelated narrative prose"}]
            large = [
                {"source": f"{i}.md", "text": "unrelated narrative prose"}
                for i in range(40)
            ]
            first, n_first = rules_condensed.build_core(rules_dir, corpus=small)
            second, n_second = rules_condensed.build_core(rules_dir, corpus=large)
            self.assertEqual(n_first, n_second)
            self.assertEqual(first, second)

    def test_live_core_embeds_no_corpus_count(self):
        """The emitted file states no corpus size, so `plan/` churn cannot stale it."""
        text = (RULES_DIR / "CORE.md").read_text(encoding="utf-8")
        header = text.split("\n---\n", 1)[0]
        self.assertIn("past task description", header)
        self.assertNotRegex(
            header,
            r"\d+\s+past task description",
            "CORE.md embeds a plan/-derived count again; every spec closure "
            "will re-stale this generated file",
        )

    def test_empty_corpus_is_distinguishable_from_no_corpus(self):
        """An empty read says so on stderr; `corpus=None` is silent and legal."""
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            self._fixture(rules_dir)
            rules = rules_condensed.load_rules(rules_dir)

            err = io.StringIO()
            with contextlib.redirect_stderr(err):
                self.assertEqual(
                    rules_condensed.unreachable_blocking(rules, None), set()
                )
            self.assertEqual(err.getvalue(), "")

            err = io.StringIO()
            with contextlib.redirect_stderr(err):
                self.assertEqual(rules_condensed.unreachable_blocking(rules, []), set())
            self.assertIn("corpus is empty", err.getvalue())

    def test_unparseable_trigger_lands_in_core(self):
        """Fail closed: a rule the router cannot route is eager, never dropped."""
        with tempfile.TemporaryDirectory() as td:
            rules_dir = Path(td)
            write_precedence(rules_dir, ["never-destroy-work"], ["rfc-compliance"])
            write_rule(
                rules_dir, "never-destroy-work.md", "before deleting", "blocking"
            )
            write_rule(
                rules_dir, "rfc-compliance.md", "when writing protocol", "blocking"
            )
            (rules_dir / "broken.md").write_text(
                "# Broken\n**Severity:** blocking\n\n## Directives\n- do it\n",
                encoding="utf-8",
            )
            names = {
                r["name"]
                for r in rules_condensed.core_members(
                    rules_condensed.load_rules(rules_dir)
                )
            }
            self.assertIn("broken.md", names)

    def test_core_body_matches_the_rule_it_came_from(self):
        """One parse: a core section reads identically to its own rule file."""
        rules = rules_condensed.load_rules(RULES_DIR)
        core = rules_condensed.core_members(rules)
        self.assertTrue(core)
        core_text, _ = rules_condensed.build_core(RULES_DIR)
        for rule in core:
            body = "\n".join(rules_condensed.condense_body(rule["body"])).strip()
            self.assertTrue(body)
            self.assertIn(body.splitlines()[0], core_text)


def instruction_imports():
    """The `@` imports in ai/INSTRUCTIONS.md, in file order.

    These ARE the always-loaded payload: Claude Code resolves each `@` line at
    session start. Anything not named here is not loaded, whatever else the
    generator emits.
    """
    text = (ROOT / "ai" / "INSTRUCTIONS.md").read_text(encoding="utf-8")
    return re.findall(r"^@(\S+)$", text, flags=re.MULTILINE)


class ImportSwitchTest(unittest.TestCase):
    """AC-8/AC-9: the routed pair is imported, and nothing became invisible."""

    def test_instructions_import_triggers_and_core(self):
        """AC-8: the canonical source imports the index and the core, not the digest.

        Asserted on `ai/INSTRUCTIONS.md` because that is the tracked, canonical
        file (`ai/rules/repo-maintenance.md`). `CLAUDE.md` and `AGENTS.md` are
        gitignored generator output, so a fresh checkout has neither until
        `make ze-ai-instructions` runs.
        """
        imports = instruction_imports()
        self.assertIn("ai/rules/TRIGGERS.md", imports)
        self.assertIn("ai/rules/CORE.md", imports)
        self.assertNotIn(
            "ai/rules/CONDENSED.md",
            imports,
            "the whole digest is imported again; the routing saving is gone",
        )

    def test_generated_instructions_match_the_canonical_imports(self):
        """CLAUDE.md and AGENTS.md carry exactly what INSTRUCTIONS.md declares."""
        expected = instruction_imports()
        for name in ("CLAUDE.md", "AGENTS.md"):
            path = ROOT / name
            if not path.is_file():
                continue  # gitignored; only present after `make ze-ai-instructions`
            found = re.findall(
                r"^@(\S+)$", path.read_text(encoding="utf-8"), flags=re.MULTILINE
            )
            self.assertEqual(
                found, expected, f"{name} is stale; run make ze-ai-instructions"
            )

    def test_condensed_is_gone_and_stays_gone(self):
        """`CONDENSED.md` was deleted on 2026-08-03 and must not come back.

        It held every rule's directives in one file. Nothing loaded it, and it
        regenerated 5,182 lines on every rule edit, so the edit cost was paid
        for a file no session read. `TRIGGERS.md` keeps every rule named.
        """
        self.assertFalse(
            (RULES_DIR / "CONDENSED.md").is_file(),
            "CONDENSED.md is back; a rule edit now regenerates it again",
        )
        self.assertNotIn(
            "CONDENSED.md", [name for name, _ in rules_condensed.ARTIFACTS]
        )
        self.assertNotIn("ai/rules/CONDENSED.md", instruction_imports())

    def test_every_live_rule_is_named_in_the_index_file(self):
        """AC-9: routing a rule's body away never makes the rule undiscoverable.

        Reads the EMITTED index rather than `trigger_lines()`, because what a
        session holds is the file, not the function that built it.
        """
        index = (RULES_DIR / "TRIGGERS.md").read_text(encoding="utf-8")
        rules = rules_condensed.load_rules(RULES_DIR)
        self.assertGreaterEqual(len(rules), 20)
        missing = [r["name"] for r in rules if f"ai/rules/{r['name']}" not in index]
        self.assertEqual(missing, [], f"unreachable in every session: {missing}")


class PayloadBudgetTest(unittest.TestCase):
    """AC-5: what a session actually loads, measured against the budget."""

    def test_payload_under_budget(self):
        instructions = ROOT / "ai" / "INSTRUCTIONS.md"
        triggers = RULES_DIR / "TRIGGERS.md"
        core = RULES_DIR / "CORE.md"
        for path in (instructions, triggers, core):
            self.assertTrue(path.is_file(), f"{path} must be generated first")
        total = sum(
            len(p.read_text(encoding="utf-8")) for p in (instructions, triggers, core)
        )
        tokens = rules_condensed.estimate_tokens(total)
        self.assertLess(
            tokens,
            rules_condensed.TOKEN_BUDGET,
            f"always-loaded payload is {tokens} tokens, budget {rules_condensed.TOKEN_BUDGET}",
        )


class GeneratedShapeTest(unittest.TestCase):
    """The artifacts carry the generated banner the sync rules depend on."""

    def test_artifacts_declare_themselves_generated(self):
        for name in ("TRIGGERS.md", "CORE.md"):
            text = (RULES_DIR / name).read_text(encoding="utf-8")
            self.assertIn("GENERATED by scripts/dev/rules_condensed.py", text)
            self.assertIn("make ze-rules-condensed", text)

    def test_generated_artifacts_are_not_parsed_as_rules(self):
        """An all-caps stem is an artifact, never a rule, so it cannot recurse."""
        names = {r["name"] for r in rules_condensed.load_rules(RULES_DIR)}
        for artifact in ("CONDENSED.md", "TRIGGERS.md", "CORE.md", "INDEX.md"):
            self.assertNotIn(artifact, names)


if __name__ == "__main__":
    unittest.main()
