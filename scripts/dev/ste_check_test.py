#!/usr/bin/env python3
"""Unit tests for ste_check.py, the ASD-STE100 review tool.

Every case here is either a habit the tool must find, or a construction it must
NOT flag. The second group is the load-bearing one: a checker that cries wolf on
`setup`, on an RFC 2119 MUST, or on a code span gets switched off, and then it
protects nothing.

Run: python3 scripts/dev/ste_check_test.py
Also runs inside `make ze-unit-test` (see scripts/dev/python_tests_test.go).
"""

import subprocess
import tempfile
import unittest
from pathlib import Path

import ste_check as ste


def findings(text, surface=ste.SURFACE_MARKDOWN, path="t.md"):
    found, skipped = ste.review(path, text, surface)
    assert skipped is None, f"unexpectedly skipped: {skipped}"
    return found


def habits(text, **kw):
    return sorted(f.habit for f in findings(text, **kw))


def details(text, **kw):
    return [f.detail for f in findings(text, **kw)]


class TestHedging(unittest.TestCase):
    def test_lowercase_hedges_are_found(self):
        for word in ("may", "might", "could", "should", "probably", "typically"):
            with self.subTest(word=word):
                self.assertIn(
                    "hedging",
                    habits(f"The peer {word} reject the route."),
                    f"{word} must be a hedging finding",
                )

    def test_rfc2119_keywords_are_never_findings(self):
        # rfc-compliance.md and every `RFC requirement:` tag read the exact word.
        text = "A speaker MUST send the NOTIFICATION. It MAY log it. It SHOULD retry."
        self.assertNotIn("hedging", habits(text))

    def test_capitalised_hedges_are_still_hedges(self):
        # Only the ALL-CAPS form is an RFC 2119 keyword. A case-sensitive match
        # let every sentence-initial hedge through, which is where writers hedge.
        for text in (
            "Typically the peer retries.",
            "Should the peer fail, stop it.",
            "Simply run the daemon.",
        ):
            with self.subTest(text=text):
                self.assertIn("hedging", habits(text))

    def test_usually_is_approved(self):
        # The dictionary resolves `generally` and `normally` to USUALLY.
        self.assertNotIn("hedging", habits("The switch is usually closed."))
        self.assertIn("hedging", habits("The switch is generally closed."))
        self.assertIn("hedging", habits("The switch is normally closed."))

    def test_hedge_phrases_are_found(self):
        self.assertIn("hedging", habits("In most cases the route is installed."))
        self.assertIn("hedging", habits("We believe the pool is exhausted."))

    def test_code_span_is_not_prose(self):
        # The rule file itself must be able to name the banned words.
        self.assertNotIn("hedging", habits("The dictionary maps `may` to CAN."))

    def test_fix_names_the_replacement(self):
        found = [f for f in findings("The peer should retry.") if f.habit == "hedging"]
        self.assertEqual(found[0].fix, "MUST")


class TestFrozenVerbs(unittest.TestCase):
    def test_light_verb_plus_nominalization(self):
        self.assertIn("frozen-verbs", habits("Do the installation of the plugin."))
        self.assertIn("frozen-verbs", habits("It performs validation of the config."))

    def test_preposition_form(self):
        self.assertIn(
            "frozen-verbs", habits("Before the removal of the unit, stop it.")
        )

    def test_ste_legal_light_verb_is_not_flagged(self):
        # STE Rule 3.7 gives this exact sentence as CORRECT: "check" is not an
        # approved verb, so "do a check of" is the STE construction.
        self.assertNotIn("frozen-verbs", habits("Do a check of the laptop battery."))

    def test_plain_technical_noun_is_not_flagged(self):
        self.assertNotIn("frozen-verbs", habits("The configuration file is on disk."))
        self.assertNotIn("frozen-verbs", habits("Read the installation guide."))

    def test_fix_names_the_verb(self):
        found = [
            f for f in findings("Do the installation now.") if f.habit == "frozen-verbs"
        ]
        self.assertIn("install", found[0].fix)

    def test_gerund_clause_is_still_found(self):
        # The detector must keep its catch. These are real gerund clauses.
        self.assertIn("frozen-verbs", habits("Before installing the plugin, stop it."))
        self.assertIn("frozen-verbs", habits("The server replies without checking it."))

    def test_indefinite_pronoun_is_not_a_gerund(self):
        # GERUND_CLAUSE matches any lowercase word that ends in `-ing`, so an
        # indefinite pronoun after one of its five prepositions used to read as
        # a banned gerund clause. Filed as F-ste-1 in plan/learned/HOOK-FRICTION.md.
        for text in (
            "The sweep runs when nothing is removed.",
            "The handler returns without anything in the body.",
            "The gate stays green while something is still queued.",
            "The list is empty after everything is deleted.",
        ):
            self.assertNotIn("frozen-verbs", habits(text), text)

    def test_numbered_abbreviation_still_holds(self):
        # "No. 5" and "Fig. 3" label a number, so the dot is not a full stop.
        from ste_check import sentences

        self.assertEqual(len(sentences("Read No. 5 before you start.")), 1)
        self.assertEqual(len(sentences("Fig. 3 shows the frame.")), 1)

    def test_no_before_a_word_ends_the_sentence(self):
        # `Required=No.` and "answered Yes/No." end sentences. Holding the dot
        # glued the next sentence on and reported a run-on nobody could fix,
        # because the text was already two sentences. Filed as F-ste-2.
        from ste_check import sentences

        self.assertEqual(
            len(sentences("Each row is answered Yes/No. Every Yes names a file.")), 2
        )
        self.assertEqual(
            len(sentences("The field is Required=No. The server omits it.")), 2
        )

    def test_string_is_not_a_gerund(self):
        # This repository writes about strings. "when string parsing fails" is
        # ordinary prose here, and it is the entry most likely to recur.
        self.assertNotIn(
            "frozen-verbs", habits("The parser reports when string parsing fails.")
        )


class TestMarketing(unittest.TestCase):
    def test_marketing_adjectives_are_found(self):
        for word in ("powerful", "seamless", "blazingly", "world-class"):
            with self.subTest(word=word):
                self.assertIn("marketing-adjectives", habits(f"Ze is {word}."))

    def test_measurable_prose_is_not_flagged(self):
        text = "Ze forwards 1.2 million updates each second on one core."
        self.assertEqual(habits(text), [])


class TestRunOns(unittest.TestCase):
    def test_long_descriptive_sentence(self):
        text = " ".join(["word"] * 30) + "."
        self.assertIn("run-ons", habits(text))

    def test_short_sentence_passes(self):
        self.assertEqual(habits("The peer sends an UPDATE."), [])

    def test_procedural_limit_is_tighter(self):
        # A numbered step is a procedure: 20 words, not 25 (STE Rules 5.1, 6.3).
        sentence = " ".join(["word"] * 22) + "."
        self.assertNotIn("run-ons", habits(sentence))
        self.assertIn("run-ons", habits("1. " + sentence))

    def test_semicolon_is_a_finding(self):
        found = findings("The route is valid; the peer accepts it.")
        self.assertTrue(any("semicolon" in f.detail for f in found))

    def test_paragraph_sentence_cap(self):
        text = " ".join(f"Sentence number {n} is here." for n in range(8))
        self.assertTrue(
            any("sentences in one paragraph" in f.detail for f in findings(text))
        )

    def test_table_row_is_not_a_run_on(self):
        row = "| " + " | ".join(["a short cell of prose"] * 6) + " |"
        self.assertEqual(habits(row), [])

    def test_table_cell_is_not_a_paragraph(self):
        # STE Rule 6.6 caps the sentences in a PARAGRAPH. A reference-table cell
        # of eight short facts is a table, and capping it would ask the author
        # for fewer, longer sentences.
        cell = " ".join(f"Fact {n} is short." for n in range(9))
        self.assertEqual(habits(f"| a | {cell} |"), [])
        self.assertTrue(any("sentences in one paragraph" in d for d in details(cell)))

    def test_fenced_block_is_data(self):
        text = "```\n" + " ".join(["word"] * 40) + "\n```\n"
        self.assertEqual(habits(text), [])

    def test_blockquote_is_external_text(self):
        text = "> " + " ".join(["word"] * 40) + "\n"
        self.assertEqual(habits(text), [])


class TestPhrasalVerbs(unittest.TestCase):
    def test_phrasal_verbs_are_found(self):
        for phrase in ("spin up", "kick off", "figure out", "get rid of", "set up"):
            with self.subTest(phrase=phrase):
                self.assertIn("phrasal-verbs", habits(f"Now {phrase} the daemon."))

    def test_technical_nouns_are_not_phrasal_verbs(self):
        # STE Rules 1.5 and 1.6 permit these as technical nouns.
        for noun in ("setup", "teardown", "shutdown", "backoff"):
            with self.subTest(noun=noun):
                self.assertNotIn("phrasal-verbs", habits(f"The {noun} is complete."))

    def test_peer_teardown_command_is_correct(self):
        self.assertNotIn("phrasal-verbs", habits("Run `request peer teardown` now."))


class TestSynonymRotation(unittest.TestCase):
    def test_rotation_beside_canonical(self):
        text = "The peer is up. The neighbour is up."
        self.assertIn("synonym-rotation", habits(text))

    def test_canonical_alone_is_correct(self):
        text = "The peer is up. The peer sends an UPDATE."
        self.assertNotIn("synonym-rotation", habits(text))

    def test_different_operations_are_not_synonyms(self):
        # delete (config), clear (counters), remove (route) are three operations.
        text = "Delete the peer. Clear the counters. Remove the route."
        self.assertNotIn("synonym-rotation", habits(text))

    def test_formal_word_is_flagged_with_the_plain_one(self):
        # The standard's core discipline: one action, one plain word.
        for formal, plain in (
            ("initiate", "start"),
            ("terminate", "stop"),
            ("obtain", "get"),
            ("utilize", "use"),
            ("ascertain", "make sure"),
        ):
            with self.subTest(formal=formal):
                found = [
                    f
                    for f in findings(f"Now {formal} the session.")
                    if f.habit == "synonym-rotation"
                ]
                self.assertTrue(found, f"{formal} must be flagged")
                self.assertIn(plain, found[0].fix)

    def test_domain_verbs_are_not_formal_words(self):
        # No plainer word means these. Flagging them would be wrong, and would
        # teach reviewers to ignore the habit.
        text = "Ze implements RFC 4271. The peer negotiates the capability."
        self.assertNotIn("synonym-rotation", habits(text))
        text2 = "The peer withdraws the route and advertises the prefix."
        self.assertNotIn("synonym-rotation", habits(text2))

    def test_plain_words_pass(self):
        self.assertEqual(habits("Start the daemon. Stop the peer. Get the route."), [])

    def test_above_and_below_for_a_limit(self):
        # An approved word keeps its approved meaning: these are positions.
        self.assertIn("synonym-rotation", habits("The value must be above 800 kPa."))
        self.assertIn("synonym-rotation", habits("Keep the count below 500."))

    def test_above_for_a_position_passes(self):
        self.assertNotIn("synonym-rotation", habits("Lift the cover above its seat."))

    def test_definite_article_before_an_identifier(self):
        found = [
            f
            for f in findings("Read the RFC 4271 before you edit the peer 10.0.0.1.")
            if f.habit == "synonym-rotation"
        ]
        self.assertEqual(len(found), 2)
        self.assertIn("drop", found[0].fix)

    def test_bare_identifier_passes(self):
        self.assertNotIn(
            "synonym-rotation", habits("Read RFC 4271, then configure peer 10.0.0.1.")
        )

    def test_counting_phrase_is_not_an_article_error(self):
        # "the first 3 bytes" is correct, and a general `the \\w+ \\d` pattern
        # would have flagged it.
        self.assertNotIn("synonym-rotation", habits("Read the first 3 bytes."))


class TestLatinAbbreviations(unittest.TestCase):
    def test_latin_abbreviations_are_flagged(self):
        for abbreviation, english in (
            ("e.g.", "for example"),
            ("i.e.", "that is"),
            ("etc.", "and so on"),
        ):
            with self.subTest(abbreviation=abbreviation):
                found = [
                    f
                    for f in findings(f"Use a pool, {abbreviation} the read pool.")
                    if f.habit == "hedging"
                ]
                self.assertTrue(found, f"{abbreviation} must be flagged")
                self.assertIn(english, found[0].fix)

    def test_english_words_pass(self):
        self.assertEqual(habits("Use a pool, for example the read pool."), [])


class TestGerundClause(unittest.TestCase):
    def test_gerund_after_a_preposition_is_flagged(self):
        found = [
            f
            for f in findings("Before installing the plugin, stop the daemon.")
            if f.habit == "frozen-verbs"
        ]
        self.assertTrue(found)
        self.assertIn("before installing", found[0].detail)
        # The fix names the actor and does NOT guess the conjugation. Stripping
        # "ing" produced "after you writ" for "after writing".
        self.assertIn("before you <verb>", found[0].fix)
        self.assertNotIn("you writ", found[0].fix)

    def test_ing_technical_noun_passes(self):
        # The -ing noun and the -ing modifier are the permitted uses.
        self.assertEqual(habits("The routing table holds the prefix."), [])
        self.assertEqual(habits("Read the section on Troubleshooting."), [])
        self.assertEqual(habits("The switching relay closes."), [])


class TestWordCount(unittest.TestCase):
    def test_parentheses_count_as_one_word(self):
        # STE Rule 8.5.
        self.assertEqual(ste.word_count("Stop the peer (see the note above)."), 4)

    def test_quoted_text_counts_as_one_word(self):
        # STE Rule 8.6.
        self.assertEqual(ste.word_count('The error is "no such peer here".'), 4)

    def test_number_with_unit_counts_as_one_word(self):
        # "800 kPa" is one word, so it must count the same as a bare number.
        # Asserted as a difference so the test cannot depend on my arithmetic.
        with_unit = ste.word_count("The value must be more than 800 kPa.")
        bare = ste.word_count("The value must be more than 800.")
        self.assertEqual(with_unit, bare)
        self.assertEqual(with_unit, 7)

    def test_hyphenated_word_counts_as_one_word(self):
        # STE Rule 8.7.
        self.assertEqual(ste.word_count("Use a route-server."), 3)


class TestSentenceSplitting(unittest.TestCase):
    def test_rule_numbers_do_not_split(self):
        self.assertEqual(len(ste.sentences("Refer to STE Rule 1.1 for words.")), 1)

    def test_filenames_do_not_split(self):
        self.assertEqual(len(ste.sentences("Read peer.go before you edit it.")), 1)

    def test_abbreviations_do_not_split(self):
        self.assertEqual(len(ste.sentences("Use a pool, e.g. the read pool.")), 1)

    def test_real_boundaries_split(self):
        self.assertEqual(len(ste.sentences("Stop the peer. Start the peer.")), 2)

    def test_bold_close_ends_a_sentence(self):
        # "**Lead-in.** Body" is two sentences. Treating it as one made every
        # bolded directive bullet look like a run-on.
        self.assertEqual(len(ste.sentences("**Stop the peer.** Then start it.")), 2)


class TestSurfacesAreScrubbed(unittest.TestCase):
    """Every surface must lose code spans and URLs before the detectors run.

    The `//` path scrubbed and the other two did not, so a backticked word or a
    URL query string inside a block comment or a YANG description became prose.
    """

    def test_go_block_comment_is_scrubbed(self):
        source = "/*\nThe dictionary maps `may` to CAN.\n*/\npackage p\n"
        self.assertEqual(habits(source, surface=ste.SURFACE_GO, path="t.go"), [])

    def test_yang_description_is_scrubbed(self):
        module = 'leaf x {\n  description "Set `may` to CAN in the table.";\n}\n'
        self.assertEqual(habits(module, surface=ste.SURFACE_YANG, path="t.yang"), [])

    def test_yang_single_quoted_description_is_read(self):
        module = "leaf x {\n  description 'The timer may expire early.';\n}\n"
        self.assertIn(
            "hedging", habits(module, surface=ste.SURFACE_YANG, path="t.yang")
        )

    def test_html_entity_is_not_a_semicolon(self):
        text = "The peer&nbsp;sends the route to the RIB now."
        self.assertEqual(habits(text), [])

    def test_multiline_html_comment_is_not_prose(self):
        text = "Text.\n\n<!--\nThe peer should retry.\n-->\n"
        self.assertEqual(habits(text), [])

    def test_a_nested_fence_does_not_close_the_block(self):
        # A ~~~ line inside a ``` block used to close it and expose the code.
        text = "```\n~~~\nThe peer should retry seamlessly.\n```\n"
        self.assertEqual(habits(text), [])


class TestGoExtractor(unittest.TestCase):
    def test_structured_markers_are_not_prose(self):
        # Each marker line carries a banned habit on purpose. Without it the
        # fixture passed even with the whole GO_MARKERS filter deleted.
        source = "\n".join(
            [
                "// Design: docs/x.md -- the peer should retry seamlessly",
                "// Related: peer.go -- it may spin up",
                "//go:build linux",
                "//nolint:errcheck // it should probably be checked",
                "package reactor",
            ]
        )
        self.assertEqual(habits(source, surface=ste.SURFACE_GO, path="t.go"), [])

    def test_a_marker_word_inside_prose_does_not_discard_the_comment(self):
        # The filter was a substring test over the first 28 characters, so any
        # prose merely CONTAINING "TODO" or "go:" was thrown away whole.
        source = (
            "// The TODO list should be emptied before the peer starts.\npackage p\n"
        )
        self.assertIn("hedging", habits(source, surface=ste.SURFACE_GO, path="t.go"))

    def test_wrapped_comment_is_measured_once(self):
        # A 30-word sentence wrapped over three lines is one run-on, not none.
        body = " ".join(["word"] * 30) + "."
        chunks = [body[i : i + 40] for i in range(0, len(body), 40)]
        source = "\n".join("// " + c for c in chunks) + "\npackage p\n"
        found = findings(source, surface=ste.SURFACE_GO, path="t.go")
        self.assertTrue(any(f.habit == "run-ons" for f in found))

    def test_commented_out_code_is_not_prose(self):
        source = "// if err != nil {\n// return err\n// }\npackage p\n"
        self.assertEqual(habits(source, surface=ste.SURFACE_GO, path="t.go"), [])

    def test_prose_comment_is_reviewed(self):
        source = "// The peer should retry the connection.\npackage p\n"
        self.assertIn("hedging", habits(source, surface=ste.SURFACE_GO, path="t.go"))


class TestYangExtractor(unittest.TestCase):
    def test_description_is_reviewed(self):
        module = 'leaf hold-time {\n  description "The timer may expire early.";\n}\n'
        self.assertIn(
            "hedging", habits(module, surface=ste.SURFACE_YANG, path="t.yang")
        )

    def test_schema_keywords_are_not_prose(self):
        module = "leaf x {\n  type uint32;\n  units seconds;\n}\n"
        self.assertEqual(habits(module, surface=ste.SURFACE_YANG, path="t.yang"), [])


class TestSkips(unittest.TestCase):
    def test_ignore_file_marker(self):
        text = (
            "<!-- ste: ignore-file quotes the BIRD manual verbatim -->\nIt may work.\n"
        )
        found, reason = ste.review("t.md", text, ste.SURFACE_MARKDOWN)
        self.assertEqual(found, [])
        self.assertIn("BIRD", reason)

    def test_generated_marker(self):
        text = "# Index\n\n<!-- GENERATED by scripts/dev/x.py -->\nIt may work.\n"
        found, reason = ste.review("t.md", text, ste.SURFACE_MARKDOWN)
        self.assertEqual(found, [])
        self.assertEqual(reason, "generated file")

    def test_ignore_line_marker_skips_the_next_line(self):
        text = "<!-- ste: ignore -->\nThe peer should retry.\n"
        self.assertEqual(habits(text), [])

    def test_a_document_deleted_at_closure_is_out_of_scope(self):
        # A spec `git rm`s itself in commit B, and a deferral or known-failure
        # shard goes when its rows resolve. Editing prose there is work nobody
        # reads (owner directive, 2026-08-10).
        self.assertTrue(ste.excluded("plan/spec-fixit-something.md"))
        self.assertTrue(ste.excluded("plan/deferrals/fixit-something.md"))
        self.assertTrue(ste.excluded("plan/known-failures/flaky-thing.md"))

    def test_the_durable_half_of_plan_stays_in_scope(self):
        # A journal row, a learned summary and the template outlive every spec
        # and are read by sessions that were not there.
        self.assertFalse(ste.excluded("plan/journal/unwired-feature.md"))
        self.assertFalse(ste.excluded("plan/learned/RECURRING-PATTERNS.md"))
        self.assertFalse(ste.excluded("plan/TEMPLATE.md"))
        self.assertFalse(ste.excluded("docs/guide/quickstart.md"))


class TestBaselineArithmetic(unittest.TestCase):
    def test_tally_is_per_surface(self):
        found = findings("The peer should retry.")
        counts = ste.tally(found)
        self.assertEqual(counts[ste.SURFACE_MARKDOWN]["hedging"], 1)
        self.assertEqual(counts[ste.SURFACE_GO]["hedging"], 0)
        self.assertEqual(ste.total(counts, "hedging"), 1)

    def test_every_habit_has_a_number_and_a_slug(self):
        self.assertEqual(len(ste.HABITS), 6)
        self.assertEqual(sorted(ste.HABITS), [1, 2, 3, 4, 5, 6])
        for number, slug in ste.HABITS.items():
            self.assertEqual(ste.SLUGS[slug], number)


class TestRatchet(unittest.TestCase):
    """The git-facing half: candidates(), head_text(), ratchet(), exit codes.

    This had NO tests. Both shipped blockers lived here (a character-set strip
    that ungated every dot-directory path, and git's quoting of a non-ASCII
    path), and the exit-code contract changed with nothing holding it.
    """

    def _repo(self, tmp):
        root = Path(tmp)
        (root / "docs").mkdir()
        subprocess.run(["git", "init", "-q"], cwd=root, check=True)
        subprocess.run(["git", "config", "user.email", "t@e.com"], cwd=root, check=True)
        subprocess.run(["git", "config", "user.name", "T"], cwd=root, check=True)
        return root

    def _commit(self, root):
        subprocess.run(["git", "add", "-A"], cwd=root, check=True)
        subprocess.run(
            ["git", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "b"],
            cwd=root,
            check=True,
        )

    BAD = "# X\n\nIt should spin up seamlessly.\n"
    GOOD = "# X\n\nZe starts the daemon.\n"

    def test_growth_in_a_changed_file_is_reported(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "docs" / "x.md").write_text(self.GOOD)
            self._commit(root)
            (root / "docs" / "x.md").write_text(self.GOOD + "\nIt should spin up.\n")
            growth, examined = ste.ratchet(root, 5)
            self.assertEqual(examined, 1)
            self.assertTrue(growth)
            self.assertEqual(growth[0][0], "docs/x.md")

    def test_inherited_prose_is_not_growth(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "docs" / "x.md").write_text(self.BAD)
            self._commit(root)
            (root / "docs" / "x.md").write_text(self.BAD + "\nZe stops the peer.\n")
            growth, _ = ste.ratchet(root, 5)
            self.assertEqual(growth, [])

    def test_untracked_file_is_gated_whole(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "docs" / "keep.md").write_text(self.GOOD)
            self._commit(root)
            (root / "docs" / "new.md").write_text(self.BAD)
            growth, _ = ste.ratchet(root, 5)
            self.assertTrue(any(g[0] == "docs/new.md" for g in growth))

    def test_a_rename_carries_its_baseline(self):
        # Moving a legacy document must not report its inherited prose as new.
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "docs" / "old.md").write_text(self.BAD)
            self._commit(root)
            subprocess.run(
                ["git", "mv", "docs/old.md", "docs/new.md"], cwd=root, check=True
            )
            growth, _ = ste.ratchet(root, 5)
            self.assertEqual(growth, [], "a rename must keep the file's own baseline")

    def test_a_deleted_file_is_not_reviewed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "docs" / "x.md").write_text(self.BAD)
            self._commit(root)
            (root / "docs" / "x.md").unlink()
            growth, examined = ste.ratchet(root, 5)
            self.assertEqual((growth, examined), ([], 0))

    def test_a_non_ascii_path_is_gated(self):
        # git quotes such a path by default, and an unquoted comparison dropped
        # the file silently.
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "docs" / "keep.md").write_text(self.GOOD)
            self._commit(root)
            (root / "docs" / "naive-\u00ef.md").write_text(self.BAD)
            growth, _ = ste.ratchet(root, 5)
            self.assertTrue(growth, "a quoted path must still be gated")

    def test_only_restricts_the_set(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / "docs" / "a.md").write_text(self.GOOD)
            (root / "docs" / "b.md").write_text(self.GOOD)
            self._commit(root)
            (root / "docs" / "a.md").write_text(self.BAD)
            (root / "docs" / "b.md").write_text(self.BAD)
            growth, _ = ste.ratchet(root, 5, only=["docs/a.md"])
            self.assertEqual({g[0] for g in growth}, {"docs/a.md"})

    def test_dot_directory_path_survives_the_prefix_strip(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._repo(tmp)
            (root / ".claude").mkdir()
            (root / "docs" / "keep.md").write_text(self.GOOD)
            self._commit(root)
            (root / ".claude" / "r.md").write_text(self.BAD)
            growth, _ = ste.ratchet(root, 5, only=[".claude/r.md"])
            self.assertTrue(growth, "str.lstrip('./') ate the leading dot")

    def test_added_reports_only_the_new_finding(self):
        old = [ste.Finding("f", 1, "markdown", "hedging", '"may"', "fix", "old line")]
        now = [
            ste.Finding("f", 1, "markdown", "hedging", '"may"', "fix", "old line"),
            ste.Finding("f", 9, "markdown", "hedging", '"may"', "fix", "new line"),
        ]
        fresh = ste.added(now, old, "hedging")
        self.assertEqual([f.excerpt for f in fresh], ["new line"])

    def test_exit_code_contract(self):
        # 3 means a habit grew, and it is deliberately NOT argparse's 2.
        self.assertEqual(ste.EXIT_HABIT_GREW, 3)
        self.assertNotEqual(ste.EXIT_HABIT_GREW, 2)


if __name__ == "__main__":
    unittest.main()
