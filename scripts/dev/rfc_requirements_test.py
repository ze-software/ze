#!/usr/bin/env python3
"""Tests for rfc_requirements.py — the RFC requirement coverage gate.

Auto-discovered and run under `go test` by scripts/dev/python_tests_test.go
(glob over scripts/dev/*_test.py). No make target needed.

Spec: plan/spec-rfc-requirement-coverage.md
"""

import contextlib
import datetime
import importlib.util
import io
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
import unittest

_HERE = os.path.dirname(os.path.abspath(__file__))


def _load():
    path = os.path.join(_HERE, "rfc_requirements.py")
    spec = importlib.util.spec_from_file_location("rfc_requirements", path)
    mod = importlib.util.module_from_spec(spec)
    sys.modules["rfc_requirements"] = mod
    spec.loader.exec_module(mod)
    return mod


R = _load()

# Scratch lives in the PROJECT tmp/, never the system one: `go test ./...` walks /tmp, and
# ai/rules/testing.md ("Temporary Files") makes the project directory the only allowed home.
_TMP_ROOT = os.path.join(R.PROJECT_DIR, "tmp")


def _mkdtemp(prefix):
    os.makedirs(_TMP_ROOT, exist_ok=True)
    return tempfile.mkdtemp(prefix=prefix, dir=_TMP_ROOT)


def _mkstemp(suffix):
    os.makedirs(_TMP_ROOT, exist_ok=True)
    fd, path = tempfile.mkstemp(suffix=suffix, dir=_TMP_ROOT)
    os.close(fd)
    return path


@contextlib.contextmanager
def _patched(**overrides):
    """Temporarily replace module-level attributes on R, restoring them afterward.

    Used by the GATE-LEVEL tests: they drive run_check / run_check_fresh end-to-end with
    controlled data sources so a wiring regression (a check that stops being called) fails
    the test, not just the helper in isolation.
    """
    saved = {name: getattr(R, name) for name in overrides}
    # The HEAD baseline is memoized for the process (two ratchets read it). A test that
    # swaps the git layer underneath it must not inherit, or leave behind, an answer read
    # through a different one -- that would be cross-test coupling to whatever ran first.
    R.reset_baseline_cache()
    try:
        for name, val in overrides.items():
            setattr(R, name, val)
        yield
    finally:
        for name, val in saved.items():
            setattr(R, name, val)
        R.reset_baseline_cache()


def _run_capturing(fn):
    """Run a driver function, returning (exit_code, captured_stdout). Exceptions propagate
    so an uncaught traceback (e.g. a handler that no longer catches OSError) surfaces as a
    test error rather than being hidden."""
    buf = io.StringIO()
    with contextlib.redirect_stdout(buf):
        code = fn()
    return code, buf.getvalue()


# --------------------------------------------------------------------------
# Requirement line parsing (AC-1, AC-2)
# --------------------------------------------------------------------------
class TestParseChecklistLine(unittest.TestCase):
    def test_parse_checklist_line_with_id(self):
        """AC-1: ID, level, text, section all extracted."""
        line = "- [ ] [RFC7606-2-1] [MUST] Treat the UPDATE as withdrawn (§2)"
        req = R.parse_checklist_line(line, "rfc7606")
        self.assertEqual(req.rid, "RFC7606-2-1")
        self.assertEqual(req.level, "MUST")
        self.assertEqual(req.section, "2")
        self.assertIn("withdrawn", req.text)
        self.assertTrue(req.gated)

    def test_parse_all_keyword_levels(self):
        cases = [
            ("[MUST]", True),
            ("[MUST NOT]", True),
            ("[SHALL]", True),
            ("[SHALL NOT]", True),
            ("[SHOULD]", False),
            ("[SHOULD NOT]", False),
            ("[MAY]", False),
            ("[RECOMMENDED]", False),
            ("[NOT RECOMMENDED]", False),
            ("[OPTIONAL]", False),
            ("[REQUIRED]", True),
        ]
        for kw, gated in cases:
            line = f"- [ ] [RFC7606-2-1] {kw} something (§2)"
            req = R.parse_checklist_line(line, "rfc7606")
            self.assertIsNotNone(req, kw)
            self.assertEqual(req.gated, gated, f"{kw} gated should be {gated}")

    def test_should_and_may_never_gate(self):
        """Scope decision: SHOULD/MAY listed and taggable, never gating."""
        for kw in ("SHOULD", "MAY", "SHOULD NOT"):
            req = R.parse_checklist_line(
                f"- [ ] [RFC7606-5-1] [{kw}] x (§5)", "rfc7606"
            )
            self.assertFalse(req.gated)

    def test_malformed_line_fails_closed(self):
        """AC-1: a MUST-level line without an ID errors; it is never skipped.

        A silently skipped MUST is a false green — the exact failure this gate exists
        to prevent (ai/rules/evidence.md).
        """
        line = "- [ ] [MUST] legacy line with no ID (§2)"
        with self.assertRaises(R.ParseError):
            R.parse_checklist_line(line, "rfc7606")

    def test_retired_counter_form_fails_loudly(self):
        """The retired per-RFC counter form must ERROR, never be silently skipped.

        Regression: a line like `- [ ] [RFC9234-R012] [MUST] ... (§5)` has an unrecognised
        first bracket, so it was dismissed as an ad-hoc category line and dropped — taking
        a live MUST out of the ledger with it. A silently skipped obligation is exactly the
        false green this gate exists to prevent (ai/rules/evidence.md).
        """
        line = (
            "- [ ] [RFC9234-R012] [MUST] OTC on egress MUST equal the identifier (§5)"
        )
        with self.assertRaises(R.ParseError) as cm:
            R.parse_checklist_line(line, "rfc9234")
        self.assertIn("RFC 2119 keyword", str(cm.exception))

    def test_any_unparseable_line_carrying_a_keyword_fails(self):
        """The rule is general: a 2119 keyword bracket means 'this is a requirement',
        so failing to parse it is an error rather than a skip."""
        for line in (
            "- [ ] [WHATEVER-9] [MUST] x (§2)",
            "- [ ] [RFC7606-nope] [SHALL NOT] x (§2)",
        ):
            with self.assertRaises(R.ParseError, msg=line):
                R.parse_checklist_line(line, "rfc7606")

    def test_adhoc_category_line_without_keyword_is_prose(self):
        """[FORMAT]/[IPSEC]/[LSA] lines are implementation TASKS, not compliance lines.
        They carry no 2119 keyword, so they are ignored rather than errored on."""
        self.assertIsNone(
            R.parse_checklist_line(
                "- [ ] [FORMAT] TLV 22 carries a 24-bit metric (Section 5)", "rfc3787"
            )
        )

    def test_section_cite_styles(self):
        """Three styles are in the wild and all are legitimate."""
        self.assertEqual(R.extract_section("x (§5.3)"), "5.3")
        self.assertEqual(R.extract_section("x (Section 6)"), "6")
        self.assertEqual(R.extract_section("x (S4.1)"), "4.1")
        self.assertEqual(R.extract_section("x (Pitfalls)"), R.NO_SECTION)

    def test_bare_s_style_does_not_match_keywords_or_asn(self):
        r"""`\bS(?=\d)` must not fire on SHOULD/SHALL, nor on the S of AS4."""
        self.assertEqual(R.extract_section("SHOULD do the thing"), R.NO_SECTION)
        self.assertEqual(R.extract_section("AS4 handling applies"), R.NO_SECTION)

    def test_cross_rfc_cite_is_not_an_anchor(self):
        """(RFC 2328 §A.3.1) on an RFC 1071 requirement names RFC 2328's section;
        anchoring an RFC1071 id to it would point at the wrong document."""
        self.assertEqual(
            R.extract_section("Compute over the packet (RFC 2328 §A.3.1)"), R.NO_SECTION
        )

    def test_trailing_paren_wins_over_an_earlier_mention(self):
        """A requirement whose prose refers to another section must anchor to the section
        it is FROM, not the one it refers TO."""
        self.assertEqual(
            R.extract_section(
                "flags inconsistent; this routes via §3.j to reset (§5.3)"
            ),
            "5.3",
        )

    def test_non_checklist_line_returns_none(self):
        self.assertIsNone(R.parse_checklist_line("## Some heading", "rfc7606"))
        self.assertIsNone(R.parse_checklist_line("", "rfc7606"))
        self.assertIsNone(R.parse_checklist_line("regular prose", "rfc7606"))

    def test_ticked_checkbox_is_recorded_not_obeyed(self):
        """A tick is a CLAIM, not evidence. It parses (losing the requirement would be
        worse) but never counts as coverage — that is derived from test tags."""
        line = "- [x] [RFC7606-2-1] [MUST] x (§2)"
        req = R.parse_checklist_line(line, "rfc7606")
        self.assertTrue(req.ticked)

    def test_ticked_checkbox_fails_the_gate_once_enrolled(self):
        """Recorded, then reported: the declare-instead-of-prove habit this replaces."""
        req = R.parse_checklist_line(
            "- [x] [RFC7606-2-1] [MUST] x (§2)", "rfc7606", source="s.md", lineno=1
        )
        errs = R.evaluate(
            [req],
            [_tag("RFC7606-2-1", "positive"), _tag("RFC7606-2-1", "negative")],
            {"rfc7606"},
        )
        self.assertTrue(any("ticked" in e for e in errs), errs)

    def test_id_must_match_owning_rfc(self):
        line = "- [ ] [RFC4271-2-1] [MUST] x (§2)"
        with self.assertRaises(R.ParseError):
            R.parse_checklist_line(line, "rfc7606")

    def test_duplicate_id_fails(self):
        """AC-2."""
        text = (
            "## Compliance Checklist\n"
            "- [ ] [RFC7606-2-1] [MUST] a (§2)\n"
            "- [ ] [RFC7606-2-1] [MUST] b (§3)\n"
        )
        with self.assertRaises(R.ParseError) as cm:
            R.parse_summary_text(text, "rfc7606")
        self.assertIn("RFC7606-2-1", str(cm.exception))

    def test_id_reuse_after_removal_fails(self):
        """AC-2: a retired ID must not come back.

        §2 had allocated 2-1..2-5. 2-3 was deleted. Re-adding a DIFFERENT 2-3 would
        silently re-point every test tagged RFC7606-2-3 at a new obligation.
        """
        # 2-3 is in the baseline, so keeping it (even with corrected text) is fine.
        baseline = {"RFC7606-2-%d" % i for i in range(1, 6)}
        reqs = R.parse_summary_text(
            "- [ ] [RFC7606-2-3] [MUST] a reworded obligation (§2)\n", "rfc7606"
        )
        self.assertEqual(R.check_id_allocation(reqs, baseline), [])

        # 2-3 was retired: below §2's high-water (2-5) but never allocated -> reuse.
        retired = {"RFC7606-2-1", "RFC7606-2-2", "RFC7606-2-5"}
        errs = R.check_id_allocation(reqs, retired)
        self.assertTrue(errs, "re-adding retired 2-3 below high-water 2-5 must fail")
        self.assertIn("RFC7606-2-3", errs[0])

    def test_high_water_is_per_section_not_per_rfc(self):
        """The section anchor's payoff: adding to §5.3 only clears §5.3's mark, so a new
        requirement lands beside its siblings instead of at the end of the RFC."""
        baseline = {"RFC7606-2-9", "RFC7606-5.3-1"}
        # 5.3-2 is new for §5.3 even though 2 <= §2's high-water of 9.
        reqs = R.parse_summary_text(
            "- [ ] [RFC7606-5.3-2] [MUST] a new 5.3 criterion (§5.3)\n", "rfc7606"
        )
        self.assertEqual(R.check_id_allocation(reqs, baseline), [])

    def test_new_id_above_high_water_passes(self):
        """AC-2 happy path: growth is normal, only backfill is suspect."""
        baseline = {"RFC7606-2-1", "RFC7606-2-2"}
        reqs = R.parse_summary_text(
            "- [ ] [RFC7606-9-3] [MUST] a new obligation (§9)\n", "rfc7606"
        )
        self.assertEqual(R.check_id_allocation(reqs, baseline), [])

    def test_text_edit_keeps_id_and_passes(self):
        """Fixing a misquote must NOT be punished — only the ID is a contract."""
        baseline = {"RFC7606-2-1"}
        reqs = R.parse_summary_text(
            "- [ ] [RFC7606-2-1] [MUST] corrected wording (§2)\n", "rfc7606"
        )
        self.assertEqual(R.check_id_allocation(reqs, baseline), [])

    def test_unknown_prefix_baseline_allows_first_allocation(self):
        """A brand-new summary has no baseline; every id is legitimate."""
        reqs = R.parse_summary_text("- [ ] [RFC9999-1-1] [MUST] x (§1)\n", "rfc9999")
        self.assertEqual(R.check_id_allocation(reqs, set()), [])

    def test_high_water_computed_per_section(self):
        marks = R.high_water({"RFC7606-2-10", "RFC7606-2-2", "RFC7606-5.3-3"})
        self.assertEqual(marks, {"RFC7606-2": 10, "RFC7606-5.3": 3})

    def test_boundary_ordinals(self):
        self.assertIsNotNone(
            R.parse_checklist_line("- [ ] [RFC7606-2-999] [MUST] x (§2)", "rfc7606")
        )
        self.assertIsNotNone(
            R.parse_checklist_line("- [ ] [RFC7606-2-1] [MUST] x (§2)", "rfc7606")
        )
        # Ordinal 0 does not exist; ids start at 1.
        with self.assertRaises(R.ParseError):
            R.parse_checklist_line("- [ ] [RFC7606-2-0] [MUST] x (§2)", "rfc7606")

    def test_id_section_must_match_cited_section(self):
        """The cross-check a bare counter could never do: an id claiming §5.3 on a line
        citing §7.1 is a contradiction, so the two can never silently drift apart."""
        with self.assertRaises(R.ParseError) as cm:
            R.parse_checklist_line("- [ ] [RFC7606-5.3-1] [MUST] x (§7.1)", "rfc7606")
        self.assertIn("disagrees with its section", str(cm.exception))

    def test_deep_and_lettered_sections(self):
        for stem, rid, sec in [
            ("rfc4271", "RFC4271-9.1.2.2-3", "9.1.2.2"),
            ("rfc7606", "RFC7606-3.b-1", "3.b"),
            ("rfc7606", "RFC7606-7.11-2", "7.11"),
        ]:
            req = R.parse_checklist_line(f"- [ ] [{rid}] [MUST] x (§{sec})", stem)
            self.assertEqual(req.rid, rid)
            self.assertEqual(req.section, sec)


class TestAnchorMismatchIsRefused(unittest.TestCase):
    """spec-fixit-rfc-row-level-and-anchor-drift AC-4, over the real rows that drove it.

    A row can name a section its obligation is NOT in. `RFC7296-3.3.6-1` cites §3.3.6
    while the D-H mandate is the mandatory Transform Type table of §3.3.3. The obvious
    repair -- leave the id, move the citation -- is the one `_validate_id` refuses, and
    `check_retired_requirements` refuses the other one (renumber the id). Both doors
    being shut is WHY the repair had to land in the row's TEXT, so a test that stops
    proving it would let a later session reach for a move the gate rejects.

    `test_id_section_must_match_cited_section` above states the general rule on a
    synthetic id. This states it on the id that actually carries the mismatch, and pairs
    it with the two forms that MUST still parse -- otherwise "always refuses" would pass.
    """

    def test_recited_without_renumber_is_refused(self):
        """The move D-2 rejected: re-anchor §3.3.6 -> §3.3.3 keeping the id."""
        with self.assertRaises(R.ParseError) as cm:
            R.parse_checklist_line(
                "- [ ] [RFC7296-3.3.6-1] [MUST] x (§3.3.3)", "rfc7296"
            )
        msg = str(cm.exception)
        self.assertIn("disagrees with its section", msg)
        self.assertIn("RFC7296-3.3.3-<n>", msg)

    def test_row_as_it_stands_parses(self):
        """Discrimination: the citation the row actually carries is accepted, so the
        case above is refusing the re-anchor rather than refusing everything."""
        req = R.parse_checklist_line(
            "- [ ] [RFC7296-3.3.6-1] [MUST] x (§3.3.6)", "rfc7296"
        )
        self.assertEqual(req.section, "3.3.6")
        self.assertTrue(req.gated)

    def test_multi_section_citation_anchors_on_the_first(self):
        """`RFC7296-2.8-1` cites (§2.8, §2.8.1): the id anchors to the FIRST section, so
        naming the subsection that carries the sentence costs no renumber. This is the
        room the level correction used, and it is why that row needed no id change."""
        req = R.parse_checklist_line(
            "- [ ] [RFC7296-2.8-1] [SHOULD] x (§2.8, §2.8.1)", "rfc7296"
        )
        self.assertEqual(req.rid, "RFC7296-2.8-1")
        self.assertEqual(req.section, "2.8")


# --------------------------------------------------------------------------
# Annotations (AC-8, AC-9, AC-6b)
# --------------------------------------------------------------------------
class TestAnnotations(unittest.TestCase):
    def test_disposition_with_reason_passes(self):
        """AC-8."""
        req = R.parse_checklist_line(
            "- [ ] [RFC5303-3-1] [MUST] x (§3) "
            "{not-applicable: Ze does not implement IS-IS mesh groups}",
            "rfc5303",
        )
        self.assertEqual(req.annotation.kind, "not-applicable")
        self.assertIn("mesh groups", req.annotation.reason)

    def test_gap_annotation_parsed(self):
        req = R.parse_checklist_line(
            "- [ ] [RFC7606-5.1-1] [MUST] x (§5.1) "
            "{gap: Ze emits MP_UNREACH first; docs/architecture/wire/mp-nlri-ordering.md}",
            "rfc7606",
        )
        self.assertEqual(req.annotation.kind, "gap")

    def test_disposition_without_reason_fails(self):
        """AC-9: a bare escape hatch is not allowed (R-1)."""
        for bad in ("{not-applicable}", "{not-applicable:}", "{gap: }"):
            with self.assertRaises(R.ParseError, msg=bad):
                R.parse_checklist_line(
                    f"- [ ] [RFC7606-2-1] [MUST] x (§2) {bad}", "rfc7606"
                )

    def test_single_polarity_annotation_parsed(self):
        """AC-6b."""
        req = R.parse_checklist_line(
            "- [ ] [RFC7606-4-1] [MUST] x (§4) "
            "{single-polarity: negative; no conforming input exists}",
            "rfc7606",
        )
        self.assertEqual(req.annotation.kind, "single-polarity")
        self.assertEqual(req.annotation.polarity, "negative")
        self.assertIn("conforming", req.annotation.reason)

    def test_single_polarity_without_reason_fails(self):
        with self.assertRaises(R.ParseError):
            R.parse_checklist_line(
                "- [ ] [RFC7606-4-1] [MUST] x (§4) {single-polarity: negative}",
                "rfc7606",
            )

    def test_single_polarity_bad_polarity_fails(self):
        with self.assertRaises(R.ParseError):
            R.parse_checklist_line(
                "- [ ] [RFC7606-4-1] [MUST] x (§4) {single-polarity: sideways; why}",
                "rfc7606",
            )

    def test_unknown_annotation_kind_fails(self):
        with self.assertRaises(R.ParseError):
            R.parse_checklist_line(
                "- [ ] [RFC7606-4-1] [MUST] x (§4) {whatever: hi}", "rfc7606"
            )


# --------------------------------------------------------------------------
# Tag scanning (AC-3, AC-4, AC-7)
# --------------------------------------------------------------------------
class TestGoTagScan(unittest.TestCase):
    def test_go_tag_covers_requirement(self):
        """AC-3: doc-comment tag on a test function."""
        src = (
            "package message\n\n"
            "// RFC requirement: RFC7606-7.1-1 negative — ORIGIN len 2 withdraws\n"
            "func TestRFC7606MalformedOriginLength(t *testing.T) {}\n"
        )
        tags = R.scan_go_tags(src, "x_test.go")
        self.assertEqual(len(tags), 1)
        self.assertEqual(tags[0].rid, "RFC7606-7.1-1")
        self.assertEqual(tags[0].polarity, "negative")
        self.assertEqual(tags[0].line, 3)

    def test_go_inline_case_tag_covers_requirement(self):
        """AC-3 / R-2: tag inline at a table case, not just the function.

        TestRFC7606SystematicLengthCorruption covers ~12 requirements across ~100
        cases; a function-level tag would stay green after the enforcing case is
        deleted.
        """
        src = (
            "func TestBig(t *testing.T) {\n"
            "  cases := []struct{ name string }{\n"
            "    // RFC requirement: RFC7606-3.g-1 negative — duplicate MP_REACH resets\n"
            '    {name: "dup/MP_REACH_duplicate_session_reset"},\n'
            "  }\n"
            "}\n"
        )
        tags = R.scan_go_tags(src, "x_test.go")
        self.assertEqual(len(tags), 1)
        self.assertEqual(tags[0].rid, "RFC7606-3.g-1")

    def test_multiple_tags_multiple_lines(self):
        src = (
            "// RFC requirement: RFC7606-7.1-1 negative — bad\n"
            "// RFC requirement: RFC7606-4-1 positive — good\n"
            "func TestX(t *testing.T) {}\n"
        )
        tags = R.scan_go_tags(src, "x_test.go")
        self.assertEqual({t.rid for t in tags}, {"RFC7606-7.1-1", "RFC7606-4-1"})

    def test_missing_polarity_in_tag_fails(self):
        """AC-7: polarity mandatory, never inferred."""
        src = "// RFC requirement: RFC7606-7.1-1\nfunc TestX(t *testing.T) {}\n"
        with self.assertRaises(R.ParseError):
            R.scan_go_tags(src, "x_test.go")

    def test_invalid_polarity_value_fails(self):
        """AC-7."""
        src = "// RFC requirement: RFC7606-7.1-1 maybe\nfunc TestX(t *testing.T) {}\n"
        with self.assertRaises(R.ParseError):
            R.scan_go_tags(src, "x_test.go")

    def test_tag_tolerates_trailing_punctuation(self):
        """`godot` requires a Go doc comment's last line to end in a period, so a tag
        placed last reads "... negative." — the lint rule and the tag convention must not
        contradict each other, leaving an author no way to satisfy both."""
        src = (
            "// RFC requirement: RFC7606-7.1-1 negative.\nfunc TestX(t *testing.T) {}\n"
        )
        tags = R.scan_go_tags(src, "x_test.go")
        self.assertEqual(len(tags), 1)
        self.assertEqual(tags[0].rid, "RFC7606-7.1-1")
        self.assertEqual(tags[0].polarity, "negative")

    def test_note_after_polarity_is_optional(self):
        src = (
            "// RFC requirement: RFC7606-7.1-1 positive\nfunc TestX(t *testing.T) {}\n"
        )
        tags = R.scan_go_tags(src, "x_test.go")
        self.assertEqual(tags[0].polarity, "positive")


class TestCiTagScan(unittest.TestCase):
    def test_ci_tag_covers_requirement(self):
        """AC-3."""
        src = (
            "# RFC 7606 treat-as-withdraw test\n"
            "# RFC requirement: RFC7606-7.1-1 negative — malformed ORIGIN withdraws\n"
            "option=timeout:value=15s\n"
        )
        tags = R.scan_ci_tags(src, "x.ci")
        self.assertEqual(len(tags), 1)
        self.assertEqual(tags[0].rid, "RFC7606-7.1-1")
        self.assertEqual(tags[0].polarity, "negative")

    def test_ci_terminator_block_not_scanned(self):
        """A-4: inside a terminator= block, '#' is raw file content, not a comment.

        Verified producer: internal/test/runner/parsing.go:248 treats '#' as a comment
        only at line start in OtherLines; terminator blocks are captured as raw content
        (test/plugin/rfc7606-withdraw.ci:35 embeds a Python shebang).
        """
        src = (
            "# RFC requirement: RFC7606-2-1 positive — real tag\n"
            "stdin=peer:terminator=EOF_PEER\n"
            "#!/usr/bin/env python3\n"
            "# RFC requirement: RFC7606-9.9-999 negative — PHANTOM, inside raw block\n"
            "EOF_PEER\n"
            "# RFC requirement: RFC7606-2-2 negative — real tag again\n"
        )
        tags = R.scan_ci_tags(src, "x.ci")
        self.assertEqual({t.rid for t in tags}, {"RFC7606-2-1", "RFC7606-2-2"})

    def test_ci_tag_must_be_at_line_start(self):
        """parsing.go:248 strips leading whitespace then checks '#', so an indented
        comment IS a comment; but a trailing '#' on a directive line is not."""
        src = "option=timeout:value=15s # RFC requirement: RFC7606-2-1 positive\n"
        self.assertEqual(R.scan_ci_tags(src, "x.ci"), [])


# --------------------------------------------------------------------------
# Carrier table and the non-Go carriers (spec-rfcgate-2-evidence AC-4..AC-9)
# --------------------------------------------------------------------------
_PY_TAG = "# RFC requirement: RFC7606-2-1 positive — real tag\n"


class TestPythonTagScan(unittest.TestCase):
    """An interop check.py is tokenized, not regex-scanned: scenario checks are full of
    quoted protocol text, and a '#' inside a string is not a comment."""

    def test_scan_python_tags_found(self):
        """AC-4: a line-start comment tag resolves with id, polarity and line."""
        src = "import os\n\n\ndef check():\n    " + _PY_TAG.lstrip() + "    pass\n"
        tags = R.scan_python_tags(src, "test/interop/scenarios/47-x/check.py")
        self.assertEqual(len(tags), 1, tags)
        self.assertEqual(tags[0].rid, "RFC7606-2-1")
        self.assertEqual(tags[0].polarity, "positive")
        self.assertEqual(tags[0].file, "test/interop/scenarios/47-x/check.py")
        self.assertEqual(tags[0].line, 5)

    def test_scan_python_tags_indented_comment_is_a_tag(self):
        """Mirrors _GO_TAG_RE's `^\\s*//`: indentation does not stop a comment being one."""
        src = "def check():\n        # RFC requirement: RFC7606-2-2 negative — indented\n        pass\n"
        self.assertEqual(
            [t.rid for t in R.scan_python_tags(src, "x/check.py")], ["RFC7606-2-2"]
        )

    def test_scan_python_tags_ignores_string_literals(self):
        """AC-5: tag-shaped text inside a docstring or a string literal is NOT a tag.

        This is the whole reason for tokenizing. A regex line scan reports three tags
        here; only the last line is a genuine comment token.
        """
        src = (
            '"""Scenario docstring.\n'
            "\n"
            "# RFC requirement: RFC7606-9.9-901 positive — PHANTOM, inside a docstring\n"
            '"""\n'
            'BANNER = "# RFC requirement: RFC7606-9.9-902 negative — PHANTOM, in a str"\n'
            "# RFC requirement: RFC7606-2-1 positive — the only real one\n"
        )
        self.assertEqual(
            [t.rid for t in R.scan_python_tags(src, "x/check.py")], ["RFC7606-2-1"]
        )

    def test_scan_python_tags_ignores_trailing_comment(self):
        """A trailing comment is not a tag in ANY carrier (scan_ci_tags agrees, :537)."""
        src = "x = 1  # RFC requirement: RFC7606-2-1 positive\n"
        self.assertEqual(R.scan_python_tags(src, "x/check.py"), [])

    def test_scan_python_tags_rejects_invalid_syntax(self):
        """AC-9: a file whose comments cannot be read must not be reported as tag-free.

        An unterminated triple-quoted string is the realistic shape: everything after it
        is swallowed into a string token, so a tokenizer that failed open would silently
        lose every tag below the break.
        """
        src = 'x = """unterminated\n' + _PY_TAG
        with self.assertRaises(R.ParseError) as cm:
            R.scan_python_tags(src, "test/interop/scenarios/47-x/check.py")
        self.assertIn("test/interop/scenarios/47-x/check.py", str(cm.exception))
        self.assertIn("cannot tokenize", str(cm.exception))

    def test_scan_python_does_not_inherit_terminator(self):
        """`terminator=` models the .ci runner's raw tmpfs blocks. Python has no such
        construct (interop.py hands the whole file to importlib), so a line that happens
        to mention it must not blind the scanner to the comments below it."""
        src = (
            "CMD = 'stdin=peer:terminator=EOF_PEER'\n"
            "# RFC requirement: RFC7606-2-1 positive — still a real tag\n"
        )
        self.assertEqual(
            [t.rid for t in R.scan_python_tags(src, "x/check.py")], ["RFC7606-2-1"]
        )


class TestEtCarrier(unittest.TestCase):
    def test_scan_et_reuses_ci_semantics(self):
        """AC-6: .et routes to scan_ci_tags -- one implementation of the terminator trap,
        not a third. 163 of 164 .et files use terminator= blocks."""
        c = R.carrier_for("test/editor/commands/x.et")
        self.assertIsNotNone(c)
        self.assertEqual(c.reader, "ci")
        self.assertIs(R._READERS[c.reader], R.scan_ci_tags)
        self.assertEqual(c.tier, R.TIER_VERIFY)

    def test_et_terminator_block_is_not_scanned(self):
        """The tag outside the block resolves; the one inside it does not."""
        src = (
            "# RFC requirement: RFC7606-2-1 positive — real tag\n"
            "tmpfs=test.conf:terminator=EOF\n"
            "# RFC requirement: RFC7606-9.9-903 negative — PHANTOM, raw config content\n"
            "EOF\n"
        )
        c = R.carrier_for("test/editor/x.et")
        self.assertEqual(
            [t.rid for t in R._READERS[c.reader](src, "test/editor/x.et")],
            ["RFC7606-2-1"],
        )


class TestCarrierTable(unittest.TestCase):
    def test_carrier_table_is_single_source(self):
        """Every consumer reads CARRIERS; no literal suffix check survives outside it.

        This is A-3 mechanized. The module used to spell the carrier list THREE times
        (scan_tree, the HEAD baseline filter, _scan_tags_tolerant), and widening one alone
        desynchronizes the ratchet baseline in the green direction -- the failure mode a
        conflict marker would have made loud and this one does not.
        """
        path = os.path.join(_HERE, "rfc_requirements.py")
        with open(path, encoding="utf-8") as fh:
            lines = fh.readlines()
        start = next(i for i, ln in enumerate(lines) if ln.startswith("CARRIERS"))
        end = next(i for i in range(start, len(lines)) if lines[i].rstrip() == ")")
        table = set(range(start, end + 1))
        offenders = []
        for i, ln in enumerate(lines):
            if i in table or ln.lstrip().startswith("#"):
                continue
            for literal in (
                '"_test.go"',
                '".ci"',
                '".et"',
                '"/check.py"',
                '"check.py"',
            ):
                if literal in ln:
                    offenders.append(f"{i + 1}: {ln.strip()}")
        self.assertEqual(offenders, [], "carrier suffixes spelled outside CARRIERS")

    def test_every_reader_exists(self):
        for c in R.CARRIERS:
            self.assertIn(c.reader, R._READERS, c.name)
            self.assertIn(c.tier, (R.TIER_VERIFY, R.TIER_NIGHTLY, R.TIER_UNRUN), c.name)

    def test_carrier_for_picks_the_specific_tree_before_the_catch_all(self):
        self.assertEqual(
            R.carrier_for("test/interop/scenarios/47-x/check.py").name, "interop-bgp"
        )
        self.assertEqual(
            R.carrier_for("test/interop-ipsec/scenarios/01-x/check.py").name,
            "interop-ipsec",
        )
        self.assertEqual(
            R.carrier_for("test/stress/scenarios/01-x/check.py").name, "scenario-check"
        )
        self.assertEqual(R.carrier_for("internal/x_test.go").name, "unit")
        # One row per suite ze-functional-test runs, so the specific suite beats the
        # catch-all -- the check the flat `functional` row could not make.
        self.assertEqual(R.carrier_for("test/plugin/x.ci").name, "functional-plugin")
        self.assertEqual(R.carrier_for("test/traffic/x.ci").name, "functional-unrun")

    def test_carrier_for_declines_an_unrelated_shape(self):
        """A file that is not a carrier yields None, so scan_tree skips it as before."""
        for rel in ("test/interop/interop.py", "internal/x.go", "test/plugin/x.conf"):
            self.assertIsNone(R.carrier_for(rel), rel)

    def test_a_non_check_py_is_not_a_carrier(self):
        """The suffix is `/check.py`, an exact basename match: `precheck.py` is not one."""
        self.assertIsNone(R.carrier_for("test/interop/scenarios/47-x/precheck.py"))


def _write(root, rel, body):
    path = os.path.join(root, rel)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(body)
    return path


class _FakeGit:
    """Stands in for the `subprocess` module with a faithful `git grep -l -z` +
    `git cat-file --batch` pair over an in-memory {path: content} tree.

    Faithful matters. _FakeSubprocess returns one canned payload, which is enough to pin
    the returncode guards it was written for but cannot answer a VARYING request. The
    baseline reader chooses which paths to ask for -- that choice IS the filter under test
    -- so the fake has to answer exactly what it was asked, in order, the way cat-file
    does. A canned payload would frame correctly for one path list and silently mis-frame
    for any other, which is how a filter regression could still look green.
    """

    def __init__(self, tree):
        self._tree = dict(tree)

    class _Result:
        def __init__(self, stdout="", stdout_bytes=b""):
            self.returncode = 0
            self.stdout = stdout_bytes if stdout_bytes else stdout

    def run(self, argv, **kwargs):
        if kwargs.get("input") is None:
            # `git grep -l -z -F "RFC requirement:"`: every fixture file holds the marker.
            return self._Result(
                stdout="".join(f"HEAD:{p}\0" for p in sorted(self._tree))
            )
        # `git cat-file --batch`: one framed record per requested path, in request order.
        out = bytearray()
        for line in kwargs["input"].decode("utf-8").split("\n"):
            if not line:
                continue
            rel = line[len("HEAD:") :]
            body = self._tree.get(rel, "").encode("utf-8")
            out += b"aaaaaaa blob " + str(len(body)).encode() + b"\n" + body + b"\n"
        return self._Result(stdout_bytes=bytes(out))


class TestUnrunCarrierRefused(unittest.TestCase):
    """AC-7: a tag in a suite nothing executes is refused, not marked.

    A marker is a note; a refusal is a guard (ai/rules/evidence.md). Raised from
    scan_tree so `make ze-rfc-index-update` refuses it too -- a check that only run_check enforced
    would let `--write` publish a ledger crediting evidence no pipeline runs.
    """

    def setUp(self):
        self.root = _mkdtemp("rfcgate2-unrun-")
        self.addCleanup(shutil.rmtree, self.root, True)

    def test_tag_in_unrun_carrier_is_refused(self):
        # test/interop-l2tp/, not test/interop-ipsec/: the ipsec tree gained a scheduled
        # caller (evidence-nightly.yml `ipsec-interop`) and its tier is now DERIVED as
        # nightly, so it is no longer an example of a carrier nothing executes. The l2tp
        # lab still has no scheduled runner. Same assertion, current example.
        _write(self.root, "test/interop-l2tp/scenarios/01-lac/check.py", _PY_TAG)
        with self.assertRaises(R.ParseError) as cm:
            R.scan_tree(self.root)
        msg = str(cm.exception)
        self.assertIn("test/interop-l2tp/scenarios/01-lac/check.py", msg)
        self.assertIn("interop-l2tp", msg)
        self.assertIn("make ze-deployment-docker-l2tp-ppp-test", msg)
        self.assertIn("SCHEDULED workflow", msg)

    def test_unrun_carrier_without_a_tag_is_silent(self):
        """Discriminates from 'always raises': the refusal is about the TAG, not the tree."""
        _write(self.root, "test/interop-l2tp/scenarios/01-lac/check.py", "x = 1\n")
        self.assertEqual(R.scan_tree(self.root), [])

    def test_unclassified_scenario_check_is_refused_too(self):
        """The catch-all fails closed: a tree nobody declared is exactly where silence
        would be indistinguishable from proof."""
        _write(self.root, "test/stress/scenarios/01-burst/check.py", _PY_TAG)
        with self.assertRaises(R.ParseError) as cm:
            R.scan_tree(self.root)
        self.assertIn("scenario-check", str(cm.exception))


class TestFilterAgreement(unittest.TestCase):
    """AC-8: the tree filter and the HEAD baseline filter accept the same carrier set.

    They are physically the same predicate now (carrier_for), and this test is what keeps
    them that way: it drives BOTH code paths over one fixture tree holding every carrier
    kind and asserts an identical tag set. Extending one alone makes it red.
    """

    FIXTURE = {
        "internal/component/bgp/a_test.go": "// RFC requirement: RFC7606-2-1 positive\n",
        "test/plugin/b.ci": "# RFC requirement: RFC7606-2-2 negative\n",
        "test/editor/commands/c.et": "# RFC requirement: RFC7606-2-3 positive\n",
        "test/interop/scenarios/47-x/check.py": "# RFC requirement: RFC7606-2-4 negative\n",
        # Not carriers: neither path may contribute a tag on either side.
        "test/interop/interop.py": "# RFC requirement: RFC7606-9.9-904 positive\n",
        "test/plugin/d.conf": "# RFC requirement: RFC7606-9.9-905 positive\n",
    }

    def setUp(self):
        self.root = _mkdtemp("rfcgate2-agree-")
        self.addCleanup(shutil.rmtree, self.root, True)
        for rel, body in self.FIXTURE.items():
            _write(self.root, rel, body)

    def _baseline_side(self):
        """The REAL _git_baseline_tag_polarities, over a faithful in-memory git.

        Deliberately not a re-implementation of the filter. An earlier draft of this test
        inlined `carrier_for` here and was mutation-verified VACUOUS: reverting the
        baseline filter to its old literal `_test.go`/`.ci` pair left it green, because it
        never executed the function it claimed to pin. A test that re-derives the thing it
        is checking checks nothing (ai/rules/interop-and-goal-validation.md).
        """
        with _patched(subprocess=_FakeGit(self.FIXTURE)):
            return R._git_baseline_tag_polarities()

    def _tree_side(self):
        out = {}
        for t in R.scan_tree(self.root):
            out.setdefault(t.rid, set()).add(t.polarity)
        return out

    def test_tree_and_baseline_filters_agree(self):
        tree, base = self._tree_side(), self._baseline_side()
        self.assertEqual(tree, base)
        self.assertEqual(
            set(tree),
            {"RFC7606-2-1", "RFC7606-2-2", "RFC7606-2-3", "RFC7606-2-4"},
            "every carrier kind must resolve, and no non-carrier may",
        )

    def test_baseline_declines_the_unrun_carrier(self):
        """The tree REFUSES an unrun tag, so the baseline must not credit one either --
        otherwise the ratchet demands evidence the tree is forbidden to supply."""
        # l2tp, not ipsec: the ipsec tree now has a scheduled caller and derives a nightly
        # tier, so it no longer demonstrates an unrun carrier.
        fixture = {"test/interop-l2tp/scenarios/01-lac/check.py": _PY_TAG}
        with _patched(subprocess=_FakeGit(fixture)):
            self.assertEqual(R._git_baseline_tag_polarities(), {})

    def test_baseline_declines_a_non_carrier(self):
        """Discriminates the other way: a shape that is not a carrier contributes nothing
        even though `git grep` listed it as containing the marker."""
        fixture = {"test/interop/interop.py": _PY_TAG}
        with _patched(subprocess=_FakeGit(fixture)):
            self.assertEqual(R._git_baseline_tag_polarities(), {})


# --------------------------------------------------------------------------
# Coverage evaluation (AC-4, AC-5, AC-6, AC-9)
# --------------------------------------------------------------------------
def _req(rid, level="MUST", annotation=None, rfc="rfc7606"):
    return R.Requirement(
        rfc=rfc,
        rid=rid,
        level=level,
        text="x",
        section="1",
        annotation=annotation,
        source="rfc/short/%s.md" % rfc,
        line=1,
    )


def _tag(rid, polarity, file="a_test.go", line=1):
    return R.Tag(rid=rid, polarity=polarity, file=file, line=line)


@contextlib.contextmanager
def _scratch():
    """A throwaway repo root under the PROJECT tmp/, never the system one.

    `go test ./...` walks /tmp and ai/rules/testing.md makes the project directory the
    only allowed home for scratch.
    """
    os.makedirs(_TMP_ROOT, exist_ok=True)
    root = tempfile.mkdtemp(prefix="rfcreq-", dir=_TMP_ROOT)
    try:
        yield root
    finally:
        shutil.rmtree(root, ignore_errors=True)


def _read_repo(rel):
    """Read a repo file as text, for the source-text assertions."""
    with open(os.path.join(R.PROJECT_DIR, rel), encoding="utf-8") as fh:
        return fh.read()


def _make_target(runner):
    """The bare make target named in a Carrier.runner ("make ze-foo" -> "ze-foo")."""
    parts = runner.split()
    if len(parts) >= 2 and parts[0] == "make":
        return parts[1]
    return ""


def _verify_stages(verify_src):
    """Every stage stagesForMode builds, read out of its own `mk("...")` calls."""
    import re as _re

    return set(_re.findall(r'mk\("([A-Za-z0-9._-]+)"\)', verify_src))


class TestCoverage(unittest.TestCase):
    def test_both_polarities_pass(self):
        """AC-6 happy path."""
        errs = R.evaluate(
            requirements=[_req("RFC7606-2-1")],
            tags=[_tag("RFC7606-2-1", "positive"), _tag("RFC7606-2-1", "negative")],
            enrolled={"rfc7606"},
        )
        self.assertEqual(errs, [])

    def test_uncovered_must_fails(self):
        """AC-5."""
        errs = R.evaluate([_req("RFC7606-2-1")], [], {"rfc7606"})
        self.assertTrue(any("RFC7606-2-1" in e for e in errs))

    def test_single_polarity_fails(self):
        """AC-6: positive-only and negative-only both fail.

        A negative-only test passes if the code rejects everything; a positive-only
        test passes if it accepts everything. Only the pair pins the behavior.
        """
        for polarity, missing in (("positive", "negative"), ("negative", "positive")):
            errs = R.evaluate(
                [_req("RFC7606-2-1")],
                [_tag("RFC7606-2-1", polarity)],
                {"rfc7606"},
            )
            self.assertTrue(errs, f"{polarity}-only must fail")
            self.assertTrue(
                any(missing in e for e in errs),
                f"error should name the missing '{missing}' polarity, got {errs}",
            )

    def test_single_polarity_annotation_allows_one(self):
        """AC-6b."""
        ann = R.Annotation(kind="single-polarity", polarity="negative", reason="why")
        errs = R.evaluate(
            [_req("RFC7606-2-1", annotation=ann)],
            [_tag("RFC7606-2-1", "negative")],
            {"rfc7606"},
        )
        self.assertEqual(errs, [])

    def test_stale_single_polarity_annotation_fails(self):
        """AC-6c: the other polarity showed up, so the annotation is a lie."""
        ann = R.Annotation(kind="single-polarity", polarity="negative", reason="why")
        errs = R.evaluate(
            [_req("RFC7606-2-1", annotation=ann)],
            [_tag("RFC7606-2-1", "negative"), _tag("RFC7606-2-1", "positive")],
            {"rfc7606"},
        )
        self.assertTrue(any("stale" in e.lower() for e in errs), errs)

    def test_stale_disposition_fails(self):
        """AC-9: disposition + tags is a contradiction (dep_audit.py:861-868 shape)."""
        ann = R.Annotation(kind="not-applicable", polarity=None, reason="why")
        errs = R.evaluate(
            [_req("RFC7606-2-1", annotation=ann)],
            [_tag("RFC7606-2-1", "positive")],
            {"rfc7606"},
        )
        self.assertTrue(any("stale" in e.lower() for e in errs), errs)

    def test_disposition_passes_without_tags(self):
        ann = R.Annotation(kind="not-applicable", polarity=None, reason="why")
        errs = R.evaluate([_req("RFC7606-2-1", annotation=ann)], [], {"rfc7606"})
        self.assertEqual(errs, [])

    def test_unknown_id_in_tag_fails(self):
        """AC-4: mirrors check_doc_links.py:208 dangling-reference treatment."""
        errs = R.evaluate(
            [_req("RFC7606-2-1")],
            [
                _tag("RFC7606-2-1", "positive"),
                _tag("RFC7606-2-1", "negative"),
                _tag("RFC7606-9.9-404", "positive", file="b_test.go", line=42),
            ],
            {"rfc7606"},
        )
        joined = " ".join(errs)
        self.assertIn("RFC7606-9.9-404", joined)
        self.assertIn("b_test.go:42", joined)

    def test_unenrolled_rfc_not_gated(self):
        """R-3: un-enrolled requirements are listed, not gated."""
        errs = R.evaluate([_req("RFC7606-2-1")], [], enrolled=set())
        self.assertEqual(errs, [])

    def test_unenrolled_rfc_still_rejects_dangling_tags(self):
        """A dangling tag is a bug regardless of enrolment."""
        errs = R.evaluate(
            [_req("RFC7606-2-1")], [_tag("RFC7606-9.9-404", "positive")], set()
        )
        self.assertTrue(any("RFC7606-9.9-404" in e for e in errs))

    def test_should_level_needs_no_polarity_pair(self):
        errs = R.evaluate(
            [_req("RFC7606-5-1", level="SHOULD")],
            [_tag("RFC7606-5-1", "positive")],
            {"rfc7606"},
        )
        self.assertEqual(errs, [])


# --------------------------------------------------------------------------
# Enrolment ratchet (AC-14, AC-15)
# --------------------------------------------------------------------------
class TestEnrolment(unittest.TestCase):
    def test_empty_enrolled_list_fails(self):
        """AC-14: clean must mean 'compared and found nothing', never 'compared nothing'."""
        errs = R.check_enrolment(current=set(), baseline=set(), summaries={"rfc7606"})
        self.assertTrue(errs)

    def test_unenrolling_fails(self):
        """AC-15: enrolment is monotonic."""
        errs = R.check_enrolment(
            current={"rfc7606"},
            baseline={"rfc7606", "rfc4271"},
            summaries={"rfc7606", "rfc4271"},
        )
        self.assertTrue(any("rfc4271" in e for e in errs))

    def test_enrolling_more_passes(self):
        errs = R.check_enrolment(
            current={"rfc7606", "rfc4271"},
            baseline={"rfc7606"},
            summaries={"rfc7606", "rfc4271"},
        )
        self.assertEqual(errs, [])

    def test_enrolled_rfc_without_summary_fails(self):
        """AC-14."""
        errs = R.check_enrolment(
            current={"rfc9999"}, baseline={"rfc9999"}, summaries={"rfc7606"}
        )
        self.assertTrue(any("rfc9999" in e for e in errs))

    def test_enrolled_rfc_without_source_text_fails(self):
        """Enrolment REQUIRES the RFC's own text (owner ruling 2026-07-27).

        Without it the summary is validated only against itself: the gate reports
        'no source text under rfc/full/ -- cannot judge' and every obligation the
        summary states is taken on faith. 55 of the enrolled RFCs were in that
        state, including the two the BMP work was done under.
        """
        errs = R.check_enrolment(
            current={"rfc9999"}, baseline={"rfc9999"}, summaries={"rfc9999"}
        )
        self.assertTrue(
            any("no source text" in e and "rfc9999" in e for e in errs),
            f"expected a missing-source-text violation, got {errs}",
        )

    def test_enrolled_rfc_with_source_text_passes(self):
        """The other polarity: a stem whose text IS present raises nothing.

        rfc7606 is enrolled and has rfc/full/rfc7606.txt in the tree, so this
        fails if the new check ever rejects a stem it should accept.
        """
        errs = R.check_enrolment(
            current={"rfc7606"}, baseline={"rfc7606"}, summaries={"rfc7606"}
        )
        self.assertEqual(errs, [])


# --------------------------------------------------------------------------
# Fingerprints (AC-13)
# --------------------------------------------------------------------------
class TestFingerprint(unittest.TestCase):
    def test_fingerprint_detects_requirement_edit(self):
        a = R.requirement_sha("MUST do the thing (§2)")
        b = R.requirement_sha("MUST do the other thing (§2)")
        self.assertNotEqual(a, b)

    def test_fingerprint_detects_test_edit(self):
        """AC-13 / user story 5: weakening a tagged test re-stales its verdict."""
        old = "func TestX(t *testing.T) {\n  want := 1\n  assert(got, want)\n}\n"
        new = "func TestX(t *testing.T) {\n  want := 2\n  assert(got, want)\n}\n"
        self.assertNotEqual(R.test_sha(old), R.test_sha(new))

    def test_fingerprint_ignores_pure_whitespace(self):
        """R-5: over-triggering is safe but pointless churn helps nobody."""
        a = R.test_sha("func TestX(t *testing.T) {\n  a := 1\n}\n")
        b = R.test_sha("func TestX(t *testing.T) {\n\n  a := 1\n\n}\n")
        self.assertEqual(a, b)


# --------------------------------------------------------------------------
# The transitional (pre-`units`) file-level rule -- AC-20
# --------------------------------------------------------------------------
class TestTransitionalFileLevelRule(unittest.TestCase):
    """The rule a verdict recorded before unit fingerprints existed still takes.

    These four cases used to drive `verdict_is_fresh`, a second spelling of this rule that
    `verdict_freshness` was documented as delegating to and never called. The two had already
    diverged (the helper consulted no `code` map), so the assurance these cases gave was about a
    function the gate never executed. They now drive the live transitional branch --
    `verdict_freshness` with no `units` recorded -- so the coverage sits on the code that runs.

    Re-pointed rather than deleted: the FUNCTIONALITY was not removed, only relocated
    (`ai/rules/testing.md`). The move also makes each case stronger, because the live
    path names WHICH stale state it is where the boolean could only say "not fresh".
    """

    def test_verdict_fresh_when_nothing_changed(self):
        verdict = {"requirement_sha": "aaa", "tests": {"a_test.go::TestOne": "bbb"}}
        self.assertEqual(
            R.verdict_freshness(verdict, "aaa", {"a_test.go::TestOne": "bbb"}),
            (R.FRESH, []),
        )

    def test_verdict_stale_when_requirement_sha_changes(self):
        verdict = {"requirement_sha": "aaa", "tests": {"a_test.go::TestOne": "bbb"}}
        state, _moved = R.verdict_freshness(
            verdict, "CHANGED", {"a_test.go::TestOne": "bbb"}
        )
        self.assertEqual(state, R.STALE_REQUIREMENT)

    def test_verdict_stale_when_test_sha_changes(self):
        verdict = {"requirement_sha": "aaa", "tests": {"a_test.go::TestOne": "bbb"}}
        state, _moved = R.verdict_freshness(
            verdict, "aaa", {"a_test.go::TestOne": "CHANGED"}
        )
        self.assertEqual(state, R.STALE_UNIT)

    def test_verdict_stale_when_test_disappears_or_appears(self):
        """A tagged test deleted, or a new one added, both invalidate the verdict."""
        verdict = {"requirement_sha": "aaa", "tests": {"a_test.go::TestOne": "bbb"}}
        gone, _ = R.verdict_freshness(verdict, "aaa", {})
        added, _ = R.verdict_freshness(
            verdict, "aaa", {"a_test.go::TestOne": "bbb", "b_test.go::TestTwo": "ccc"}
        )
        self.assertEqual(gone, R.STALE_UNIT)
        self.assertEqual(added, R.STALE_UNIT)

    def test_a_recorded_units_map_leaves_the_transitional_branch(self):
        """The discriminating twin, and the reason the branch is reachable at all: the same input
        with `units` recorded takes the unit-level path instead, where an unchanged unit and a
        moved file are SHIFTED rather than STALE_UNIT. If this ever equals the case above, the
        transitional branch has stopped being transitional.

        The moved file is now spelled as a moved FILE sha under an unchanged key. It used to be
        spelled as a moved key (`a_test.go:10` to `a_test.go:99`), which is the state a location
        key had and a symbol key does not: an edit above the function leaves the key alone."""
        verdict = {
            "requirement_sha": "aaa",
            "tests": {"a_test.go::TestOne": "bbb"},
            "units": {"a_test.go::TestOne": "u" * 16},
        }
        state, _moved = R.verdict_freshness(
            verdict,
            "aaa",
            {"a_test.go::TestOne": "MOVED"},
            {"a_test.go::TestOne": "u" * 16},
        )
        self.assertEqual(state, R.SHIFTED)


# --------------------------------------------------------------------------
# rfc-status.md cross-check (AC-10)
# --------------------------------------------------------------------------
class TestAuditFreshness(unittest.TestCase):
    """The hinge between the mechanical half and the semantic half.

    The gate proves a LINK exists; only a reader can say the test still enforces the
    requirement's letter and spirit. The fingerprint decides WHEN that reader must look
    again.
    """

    def test_missing_verdict_is_not_an_error(self):
        """The audit is sampled; the gate is total. An un-audited requirement is normal --
        making it fail would force 2162 verdicts before anything could go green.

        Uses an RFC with no rfc/audit/<stem>.json so the "no verdict" path is exercised
        regardless of which real audit files exist in the tree (rfc7606 now has one)."""
        errs = R.check_audit_freshness(
            [_req("RFC9999-1-1", rfc="rfc9999")], [], {"rfc9999"}
        )
        self.assertEqual(errs, [])

    def test_unenrolled_rfc_is_not_audited(self):
        errs = R.check_audit_freshness([_req("RFC7606-2-1")], [], set())
        self.assertEqual(errs, [])

    def test_verdict_fresh_and_stale(self):
        """A verdict that no longer matches what it judged is worse than none: it is a
        stale assurance. That is why it FAILS while a missing one does not.

        Drives `verdict_freshness` (the live rule) rather than the deleted `verdict_is_fresh`
        duplicate it used to call."""
        v = {"requirement_sha": R.requirement_sha("x"), "tests": {}}
        self.assertEqual(
            R.verdict_freshness(v, R.requirement_sha("x"), {}), (R.FRESH, [])
        )
        state, _moved = R.verdict_freshness(v, R.requirement_sha("CHANGED"), {})
        self.assertEqual(state, R.STALE_REQUIREMENT)

    def test_requirement_text_edit_stales_the_verdict(self):
        a = R.requirement_sha("MUST discard the attribute")
        b = R.requirement_sha("MUST treat the UPDATE as withdrawn")
        self.assertNotEqual(
            a, b, "re-reading the RFC must invalidate the old judgement"
        )


class TestEnrolledDescriptorLevelsMatchTheSummaries(unittest.TestCase):
    """rfc/enrolled.txt quotes requirement LEVELS in prose; the summary owns them.

    An enrolment note is the human-readable record of why an RFC is gated, and it routinely
    cites a requirement as `RFCNNNN-x-1 (MUST NOT ...)`. That parenthetical is a copy of a
    fact that lives in rfc/short/<stem>.md, and a copy drifts: RFC7947-x-1 was corrected to
    SHOULD NOT in the summary on 2026-07-23 (RFC 7947 Section 2.2.2.1 calls it "a
    recommendation rather than a requirement") while enrolled.txt went on calling it a MUST
    NOT. Nothing noticed, because GATED_LEVELS excludes SHOULD NOT, so the coverage checker
    never reads either spelling.

    That is the failure mode this pins: a level overstated in the note is invisible to the
    gate but is exactly what a reader trusts when deciding what Ze owes
    (ai/rules/evidence.md, ai/rules/evidence.md).
    """

    # Longest-first so "MUST NOT" is preferred over "MUST" and "SHOULD NOT" over "SHOULD".
    _LEVEL_ALT = "|".join(
        re.escape(x) for x in sorted(R.ALL_LEVELS, key=len, reverse=True)
    )
    _CITE = re.compile(
        r"\b(RFC\d+[A-Za-z0-9-]*-[A-Za-z0-9.]+-\d+)\s*\((" + _LEVEL_ALT + r")\b"
    )

    def _summary_levels(self):
        levels = {}
        for stem in R.summary_stems():
            path = os.path.join(R.SUMMARY_DIR, stem + ".md")
            for req in R.parse_summary_file(path):
                levels[req.rid] = req.level
        return levels

    def test_every_level_cited_in_an_enrolment_note_matches_its_summary(self):
        levels = self._summary_levels()
        raw = _read_repo("rfc/enrolled.txt")
        checked = 0
        for n, line in enumerate(raw.splitlines(), 1):
            for rid, cited in self._CITE.findall(line):
                checked += 1
                self.assertIn(
                    rid,
                    levels,
                    f"rfc/enrolled.txt:{n} cites {rid}, which no summary declares",
                )
                self.assertEqual(
                    cited,
                    levels[rid],
                    f"rfc/enrolled.txt:{n} calls {rid} a {cited}; "
                    f"rfc/short/ declares it {levels[rid]}. The summary owns the level",
                )
        # Guard against the check quietly covering nothing if the prose style changes.
        self.assertGreater(checked, 5, "no level citations parsed out of enrolled.txt")


class TestStatusLedgerCrossCheck(unittest.TestCase):
    STATUS = (
        "| RFC | Area | Status | Implemented coverage | Remaining if not complete |\n"
        "|-----|------|--------|----------------------|---------------------------|\n"
        "| RFC 7606 | Revised UPDATE error handling | Partial | stuff | "
        "Ze intentionally emits MP_UNREACH first, not compliant with 5.1 ordering. |\n"
        "| RFC 4271 | BGP-4 base protocol | Supported | stuff | "
        "No tracked gap in current source anchors. |\n"
        "| draft-ietf-example-thing | Example draft | Partial | stuff | "
        "Section 6 validation is unimplemented. |\n"
        "| sflow-v5 | Example non-RFC stem | Experimental | stuff | "
        "Three MUST gaps tracked. |\n"
    )

    def test_gap_disclosed_passes(self):
        rows = R.parse_status_ledger(self.STATUS)
        ann = R.Annotation(kind="gap", polarity=None, reason="ordering")
        errs = R.check_status_agreement(
            [_req("RFC7606-5.1-1", annotation=ann)], rows, {"rfc7606"}
        )
        self.assertEqual(errs, [])

    def test_gap_must_be_disclosed_in_status_ledger(self):
        """AC-10: a known unmet MUST must not hide behind a clean 'Supported' row."""
        rows = R.parse_status_ledger(self.STATUS)
        ann = R.Annotation(kind="gap", polarity=None, reason="something")
        errs = R.check_status_agreement(
            [_req("RFC4271-2-1", annotation=ann, rfc="rfc4271")], rows, {"rfc4271"}
        )
        self.assertTrue(errs, "Supported row with no disclosed gap must fail")

    def test_status_ledger_parses_real_file(self):
        """A-5: the real ledger has 17 non-uniform status values."""
        path = os.path.join(_HERE, "..", "..", "docs", "features", "rfc-status.md")
        with open(path, encoding="utf-8") as fh:
            rows = R.parse_status_ledger(fh.read())
        self.assertGreater(len(rows), 40)
        self.assertIn("rfc7606", rows)

    def test_draft_stem_row_is_keyed(self):
        """A draft summary enrolls under its stem, so its status row is keyed by
        that stem (there is no RFC number). Without this a {gap} on an enrolled
        draft could never be disclosed."""
        rows = R.parse_status_ledger(self.STATUS)
        self.assertIn("draft-ietf-example-thing", rows)
        self.assertEqual(rows["draft-ietf-example-thing"]["status"], "Partial")

    def test_nonrfc_stem_row_is_keyed(self):
        """A non-RFC, non-draft summary (e.g. sflow-v5) enrolls under its file
        stem, a lowercase hyphenated token; its status row is keyed by that stem
        so a {gap} on it can find its disclosure row."""
        rows = R.parse_status_ledger(self.STATUS)
        self.assertIn("sflow-v5", rows)
        ann = R.Annotation(kind="gap", polarity=None, reason="split seq")
        errs = R.check_status_agreement(
            [_req("SFLOW-V5-x-9", annotation=ann, rfc="sflow-v5")],
            rows,
            {"sflow-v5"},
        )
        self.assertEqual(errs, [])

    def test_draft_gap_disclosed_by_partial_row(self):
        """A {gap} on an enrolled draft passes when its draft-stem row is
        non-'Supported' (Partial discloses the shortfall)."""
        rows = R.parse_status_ledger(self.STATUS)
        ann = R.Annotation(kind="gap", polarity=None, reason="section 6")
        errs = R.check_status_agreement(
            [
                _req(
                    "DRAFT-IETF-EXAMPLE-THING-6-1",
                    annotation=ann,
                    rfc="draft-ietf-example-thing",
                )
            ],
            rows,
            {"draft-ietf-example-thing"},
        )
        self.assertEqual(errs, [])


# --------------------------------------------------------------------------
# Ledger render (AC-20)
# --------------------------------------------------------------------------
class TestCoverageRollup(unittest.TestCase):
    """The rollup is the worklist. If it miscounts, the backlog lies about itself."""

    def test_counts_each_state_once(self):
        ann = R.Annotation(kind="not-applicable", polarity=None, reason="why")
        reqs = [
            _req("RFC7606-2-1"),  # both
            _req("RFC7606-2-2"),  # one polarity
            _req("RFC7606-2-3"),  # no test
            _req("RFC7606-2-4", annotation=ann),  # annotated
            _req("RFC7606-2-5", level="SHOULD"),  # not gated -> excluded entirely
        ]
        tags = [
            _tag("RFC7606-2-1", "positive"),
            _tag("RFC7606-2-1", "negative"),
            _tag("RFC7606-2-2", "negative"),
        ]
        cov = R.rfc_coverage(reqs, tags)
        self.assertEqual(len(cov), 1)
        c = cov[0]
        self.assertEqual(c.gated, 4, "SHOULD must not be counted as gated")
        self.assertEqual((c.both, c.one, c.annotated, c.missing), (1, 1, 1, 1))

    def test_outstanding_is_the_work_that_does_not_exist_yet(self):
        """Outstanding = one-polarity + no-test. An annotated requirement owes nothing:
        its reason has already been argued and is itself gated."""
        ann = R.Annotation(kind="gap", polarity=None, reason="why")
        reqs = [_req("RFC7606-2-1"), _req("RFC7606-2-2", annotation=ann)]
        cov = R.rfc_coverage(reqs, [_tag("RFC7606-2-1", "positive")])
        self.assertEqual(cov[0].outstanding, 1)

    def test_fully_covered_rfc_owes_nothing(self):
        reqs = [_req("RFC7606-2-1")]
        tags = [_tag("RFC7606-2-1", "positive"), _tag("RFC7606-2-1", "negative")]
        self.assertEqual(R.rfc_coverage(reqs, tags)[0].outstanding, 0)

    def test_rfc_with_no_gated_requirements_is_omitted(self):
        """A summary of pure SHOULD/MAY has no backlog and must not pad the rollup."""
        self.assertEqual(R.rfc_coverage([_req("RFC7606-5-1", level="MAY")], []), [])


class TestLedgerRender(unittest.TestCase):
    """The requirement rows, which live in one shard per RFC stem."""

    def test_ledger_render_is_independent_of_input_order(self):
        """Varies the ORDER of both ordered inputs rather than calling the renderer twice
        with the same arguments: `f(x) == f(x)` in one process holds for every
        implementation and discriminated nothing.

        Order-independence is the property that matters. os.walk and os.listdir yield in
        filesystem order, and check_ledger_fresh re-renders and compares byte for byte, so
        an order-sensitive renderer reports a perfectly fresh ledger as stale on the next
        machine."""
        reqs = [_req("RFC7606-2-1"), _req("RFC7606-2-2"), _req("RFC7606-3-1")]
        tags = [
            _tag("RFC7606-2-1", "positive", file="a_test.go", line=3),
            _tag("RFC7606-2-1", "negative", file="b_test.go", line=9),
            _tag("RFC7606-2-2", "positive", file="c_test.go", line=1),
        ]
        forward = R.render_shards(reqs, tags, {"rfc7606"})["rfc7606"]
        backward = R.render_shards(
            list(reversed(reqs)), list(reversed(tags)), {"rfc7606"}
        )["rfc7606"]
        self.assertEqual(forward, backward)
        self.assertIn("RFC7606-2-1", forward)
        # Rows follow the requirement ID order, never the input order.
        self.assertLess(
            forward.index("RFC7606-2-1"), forward.index("RFC7606-3-1"), forward
        )

    def test_ledger_shows_both_directions(self):
        """The generated half of the two-way reference."""
        reqs = [_req("RFC7606-2-1")]
        tags = [
            _tag("RFC7606-2-1", "positive", file="p_test.go", line=7),
            _tag("RFC7606-2-1", "negative", file="n_test.go", line=9),
        ]
        out = R.render_shards(reqs, tags, {"rfc7606"})["rfc7606"]
        self.assertIn("`p_test.go` (unit/verify)", out)
        self.assertIn("`n_test.go` (unit/verify)", out)

    def test_citation_order_independent_of_scan_order(self):
        """os.walk yields files in filesystem order, so the render must sort citations
        by (file, line) — otherwise the ledger churns across machines and the freshness
        gate (AC-20) flags a stale ledger that is not actually wrong.

        The sort still runs on (file, line) even though the line is no longer RENDERED.
        It is what makes the collapse below deterministic: the surviving citation is the
        one whose tag comes first in the file, on every machine.
        """
        reqs = [_req("RFC7606-2-1")]
        tags = [
            _tag("RFC7606-2-1", "negative", file="a_test.go", line=5),
            _tag("RFC7606-2-1", "negative", file="a_test.go", line=90),
            _tag("RFC7606-2-1", "negative", file="z_test.go", line=1),
        ]
        forward = R.render_shards(reqs, list(tags), {"rfc7606"})["rfc7606"]
        backward = R.render_shards(reqs, list(reversed(tags)), {"rfc7606"})["rfc7606"]
        self.assertEqual(forward, backward)
        self.assertLess(forward.index("a_test.go"), forward.index("z_test.go"))

    def test_two_tags_in_one_unit_render_one_citation(self):
        """Several inline tags in one file collapse to a single citation.

        The line is gone from the render, so two tags in `a_test.go` would otherwise
        emit the identical string twice and tell the reader nothing by repeating it.
        The count of inline cases inside a unit was never a fact a generated page could
        keep current, and the unit that enforces the requirement is what the row is for.
        """
        reqs = [_req("RFC7606-2-1")]
        tags = [
            _tag("RFC7606-2-1", "negative", file="a_test.go", line=5),
            _tag("RFC7606-2-1", "negative", file="a_test.go", line=90),
        ]
        out = R.render_shards(reqs, tags, {"rfc7606"})["rfc7606"]
        self.assertEqual(1, out.count("`a_test.go`"))

    def test_no_line_number_reaches_a_rendered_citation(self):
        """The whole point: an edit ABOVE a tag must not rewrite this page.

        Asserted over the citation columns rather than the row, because the Note column
        carries authored prose that legitimately cites producer code.
        """
        reqs = [_req("RFC7606-2-1")]
        tags = [
            _tag("RFC7606-2-1", "positive", file="p_test.go", line=7),
            _tag("RFC7606-2-1", "negative", file="n_test.go", line=9),
        ]
        out = R.render_shards(reqs, tags, {"rfc7606"})["rfc7606"]
        row = [ln for ln in out.splitlines() if ln.startswith("| `RFC7606-2-1`")][0]
        cited = row.split("|")[4] + row.split("|")[5]
        self.assertNotRegex(cited, r"\.go:\d+")


_CI_FILE = "test/plugin/rfc7606-reset.ci"
_INTEROP_FILE = "test/interop/scenarios/bgp-rfc7606-relay-shape-frr/check.py"


class TestLedgerEvidenceTier(unittest.TestCase):
    """AC-10/AC-11: the ledger publishes evidence STRENGTH, not just its existence.

    Before this, a nightly-advisory interop scenario and a merge-gate unit test rendered
    into byte-identical cells, so "proven" meant two different things in one column and a
    reader had no way to tell which (R-1).
    """

    def _render(self, tags, reqs=None):
        """The rfc7606 SHARD: the requirement rows are what carries an evidence label."""
        return R.render_shards(reqs or [_req("RFC7606-2-1")], tags, {"rfc7606"})[
            "rfc7606"
        ]

    def _index(self, tags, reqs=None):
        """The INDEX: the legend and the rollup, which summarise those rows."""
        return R.render_index(reqs or [_req("RFC7606-2-1")], tags, {"rfc7606"})

    def test_ledger_row_carries_evidence_tier(self):
        """AC-10: every link carries its kind AND its tier, derived from CARRIERS."""
        out = self._render(
            [
                _tag("RFC7606-2-1", "positive", file="internal/x_test.go", line=3),
                _tag("RFC7606-2-1", "negative", file=_CI_FILE, line=7),
            ]
        )
        self.assertIn(
            "`internal/x_test.go` (unit/verify)",  # <!-- doc-links: ignore (fixture path, deliberately absent) -->
            out,
        )
        self.assertIn(f"`{_CI_FILE}` (functional/verify)", out)

    def test_interop_link_is_labelled_nightly(self):
        out = self._render(
            [_tag("RFC7606-2-1", "positive", file=_INTEROP_FILE, line=51)]
        )
        self.assertIn(f"`{_INTEROP_FILE}` (interop/nightly)", out)

    def test_nightly_only_marker_rendered(self):
        """AC-11: a requirement whose ONLY evidence is nightly says so on its row."""
        out = self._render(
            [
                _tag("RFC7606-2-1", "positive", file=_INTEROP_FILE, line=51),
                _tag("RFC7606-2-1", "negative", file=_INTEROP_FILE, line=60),
            ]
        )
        self.assertIn("**nightly-only**", out)

    def test_no_nightly_marker_when_verify_evidence_exists(self):
        """Discriminates from 'always marks': one verify-tier link removes the marker,
        because the requirement IS proven on the merge path."""
        out = self._render(
            [
                _tag("RFC7606-2-1", "positive", file=_INTEROP_FILE, line=51),
                _tag("RFC7606-2-1", "negative", file=_CI_FILE, line=7),
            ]
        )
        self.assertNotIn("**nightly-only**", out)

    def test_untested_requirement_is_not_nightly_only(self):
        """No evidence is a `missing`, not a weak proof. Marking it would hide the
        difference between 'proven somewhere unrun' and 'not proven at all'."""
        self.assertFalse(R.is_nightly_only([]))
        self.assertNotIn("**nightly-only**", self._render([]))

    def test_nightly_only_has_its_own_rollup_column(self):
        """AC-11: counted separately, never folded into the merge-gate columns."""
        cov = R.rfc_coverage(
            [_req("RFC7606-2-1"), _req("RFC7606-2-2")],
            [
                _tag("RFC7606-2-1", "positive", file=_INTEROP_FILE),
                _tag("RFC7606-2-1", "negative", file=_INTEROP_FILE, line=2),
                _tag("RFC7606-2-2", "positive", file="internal/x_test.go"),
                _tag("RFC7606-2-2", "negative", file="internal/x_test.go", line=2),
            ],
        )
        self.assertEqual(len(cov), 1)
        self.assertEqual(cov[0].nightly_only, 1)
        self.assertEqual(cov[0].both, 2, "both stays the polarity view, unweakened")
        out = self._index([], reqs=[_req("RFC7606-2-1")])
        self.assertIn("| Outstanding | Nightly-only | State |", out)

    def test_legend_is_derived_from_the_carrier_table(self):
        """A hand-written legend rots the moment a carrier is added
        (ai/rules/evidence.md)."""
        out = self._index([])
        for c in R.CARRIERS:
            if c.tier == R.TIER_UNRUN:
                self.assertNotIn(f"| `{c.label}` | `*{c.suffix}`", out, c.name)
            else:
                self.assertIn(f"| `{c.label}` | `*{c.suffix}`", out, c.name)

    def test_ledger_render_is_stable(self):
        """AC-12: two renders of one tree are byte-identical, so check_ledger_fresh stays
        a real gate rather than a coin flip. Both outputs: a shard is a committed generated
        file too, so an unstable one reads as a hand edit on the next machine exactly as an
        unstable index would."""
        tags = [
            _tag("RFC7606-2-1", "positive", file=_INTEROP_FILE, line=51),
            _tag("RFC7606-2-1", "negative", file=_CI_FILE, line=7),
        ]
        self.assertEqual(self._render(list(tags)), self._render(list(tags)))
        self.assertEqual(self._index(list(tags)), self._index(list(tags)))

    def test_evidence_phrase_names_every_executable_label_including_zeros(self):
        """A label omitted when zero reads as 'not applicable', not as 'we have none'."""
        phrase = R._evidence_phrase(R.tag_kind_counts([_tag("X-1-1", "positive")]))
        self.assertIn("unit/verify 1", phrase)
        self.assertIn("interop/nightly 0", phrase)
        self.assertIn("editor/verify 0", phrase)


def _FRESH_INDEX(case):
    """The index the sources render to, evaluated where the outputs are patched."""
    return R.render_index(case._reqs, case._tags, {"rfc7606"}) + "\n"


class TestLedgerFreshness(unittest.TestCase):
    """AC-20: a stale ai/RFC-REQUIREMENTS.md must fail the build, not rot silently.
    This is what let the ledger drift once already — two commits re-tagged tests without
    regenerating it and nothing caught it."""

    _reqs = [_req("RFC7606-2-1")]
    _tags = [_tag("RFC7606-2-1", "positive"), _tag("RFC7606-2-1", "negative")]

    def _check(self, contents):
        """Run the freshness check over a scratch pair of outputs.

        `contents` is what the index holds on disk: a string, None for an absent file, or a
        callable rendering it. A callable runs INSIDE the patch, which is what
        `_FRESH_INDEX` needs -- the index cites the shard directory, so a body rendered
        against the real directory and checked against a scratch one reads stale for the
        path alone, and the test would pass for a reason it was not written for.

        The shards written here are the CORRECT render, so every case varies the index.
        """
        path = _mkstemp(".md")
        shard_dir = _mkdtemp("fresh-shards-")
        try:
            with _patched(LEDGER_FILE=path, SHARD_DIR=shard_dir):
                for stem, body in R.render_shards(
                    self._reqs, self._tags, {"rfc7606"}
                ).items():
                    with open(
                        os.path.join(shard_dir, stem + ".md"), "w", encoding="utf-8"
                    ) as fh:
                        fh.write(body + "\n")
                text = contents(self) if callable(contents) else contents
                if text is None:
                    os.unlink(path)
                else:
                    with open(path, "w", encoding="utf-8") as fh:
                        fh.write(text)
                return R.check_ledger_fresh(self._reqs, self._tags, {"rfc7606"})
        finally:
            shutil.rmtree(shard_dir, ignore_errors=True)
            if os.path.exists(path):
                os.unlink(path)

    def test_fresh_when_file_matches_render(self):
        self.assertEqual(self._check(_FRESH_INDEX), [])

    def test_stale_when_file_differs(self):
        errs = self._check("not the rendered ledger\n")
        self.assertEqual(len(errs), 1)
        self.assertIn("ze-rfc-index-update", errs[0])

    def test_missing_ledger_reads_as_stale(self):
        """A missing ledger is '' != body, so it fails closed rather than passing by
        vacuum (ai/rules/evidence.md)."""
        errs = self._check(None)
        self.assertEqual(len(errs), 1)


# --------------------------------------------------------------------------
# Per-RFC shards (plan/spec-rfc-ledger-per-rfc-shards.md)
# --------------------------------------------------------------------------
_SHARD_REQS = [_req("RFC7606-2-1"), _req("RFC9999-1-1", rfc="rfc9999")]
_SHARD_TAGS = [_tag("RFC7606-2-1", "positive"), _tag("RFC7606-2-1", "negative")]

# The per-RFC table header. It appears in a shard and nowhere in the index, so it is what
# tells the two files apart: a test that searched for a requirement id instead would also
# match the audit worklist, which the index still carries.
_ROW_HEADER = "| Requirement | Level | § | Positive test | Negative test | Note |"


@contextlib.contextmanager
def _shard_tree():
    """Point BOTH outputs at a scratch tree: R.LEDGER_FILE at a file, R.SHARD_DIR at a
    directory.

    Patching only the ledger is the trap this helper exists to close -- run_write would
    then write one shard per summary into the real rfc/requirements/.
    """
    root = _mkdtemp("shards-")
    try:
        with _patched(
            LEDGER_FILE=os.path.join(root, "RFC-REQUIREMENTS.md"),
            SHARD_DIR=os.path.join(root, "requirements"),
        ):
            yield root
    finally:
        shutil.rmtree(root, ignore_errors=True)


def _read_dir(path):
    """name -> contents for every file directly in `path` (absent directory = {})."""
    if not os.path.isdir(path):
        return {}
    out = {}
    for name in sorted(os.listdir(path)):
        full = os.path.join(path, name)
        if os.path.isfile(full):
            with open(full, encoding="utf-8") as fh:
                out[name] = fh.read()
    return out


class TestIndexRender(unittest.TestCase):
    """AC-1, AC-3: ai/RFC-REQUIREMENTS.md becomes the index and keeps the head sections."""

    def test_index_has_no_per_rfc_section(self):
        """AC-1: the 97 percent of the old ledger that was per-RFC tables is gone from the
        index, and the same rows are reachable through a named shard."""
        body = R.render_index(_SHARD_REQS, _SHARD_TAGS, {"rfc7606"})
        self.assertNotIn(_ROW_HEADER, body)
        self.assertNotIn("## RFC7606 --", body)
        self.assertNotIn("## RFC9999 --", body)
        # The head sections the index keeps.
        for section in (
            "# RFC Requirement Ledger",
            "## Coverage by RFC",
            "## Evidence kinds",
            "## Extraction sign-off",
        ):
            self.assertIn(section, body)
        shards = R.render_shards(_SHARD_REQS, _SHARD_TAGS, {"rfc7606"})
        self.assertIn(_ROW_HEADER, shards["rfc7606"])
        self.assertIn("RFC7606-2-1", shards["rfc7606"])

    def test_index_names_a_real_shard_as_its_example(self):
        """R-5: check_doc_links.py sweeps every tracked file and requires each cited path to
        exist, so a placeholder with an angle-bracket stem is a dead citation in a generated
        file. The example is derived from the rendered set, so it cannot go dead."""
        body = R.render_index(_SHARD_REQS, _SHARD_TAGS, {"rfc7606"})
        self.assertIn("rfc/requirements/rfc7606.md", body)
        self.assertNotIn("rfc/requirements/<", body)

    def test_rollup_header_unchanged(self):
        """AC-3: scripts/dev/testing_health.py collect_rfc pins the rollup's header and its
        row shape and fails closed when either moves. The pins are read from that file, so
        this test breaks if the consumer and the producer ever disagree."""
        th_path = os.path.join(_HERE, "testing_health.py")
        spec = importlib.util.spec_from_file_location("testing_health", th_path)
        th = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(th)
        body = R.render_index(_SHARD_REQS, _SHARD_TAGS, {"rfc7606"})
        self.assertIn(th.RFC_TABLE_HEADER, body)
        rows = [ln for ln in body.split("\n") if th.RFC_ROW.match(ln.strip())]
        self.assertTrue(rows, "collect_rfc parses zero coverage rows from the index")
        self.assertIn("enrolled", " ".join(rows))


class TestShardBanner(unittest.TestCase):
    def test_shard_declares_generated(self):
        """AC-4: ai/rules/evidence.md permits a derived `file.go:line` in a document only
        where a generator maintains it, and a file earns that by saying so in its first ten
        lines. Every shard is a page of derived `file:line` citations."""
        shards = R.render_shards(_SHARD_REQS, _SHARD_TAGS, {"rfc7606"})
        for stem, body in shards.items():
            head = "\n".join(body.split("\n")[:10])
            self.assertIn("GENERATED", head, stem)
            self.assertIn("do not edit", head, stem)
            self.assertIn("make ze-rfc-index-update", head, stem)


class TestShardWrite(unittest.TestCase):
    """AC-1, AC-7, AC-12: the write owns the shard directory and nothing else in it."""

    def _write(self):
        with _patched(
            _collect_for_check=lambda: ({"rfc7606"}, _SHARD_REQS, [], _SHARD_TAGS, {}),
        ):
            return _run_capturing(R.run_write)

    def _write_with(self, collect):
        with _patched(_collect_for_check=collect):
            return _run_capturing(R.run_write)

    def test_a_parse_error_refuses_the_write_and_deletes_nothing(self):
        """The write DELETES, so an incomplete collection is a destructive input.

        `_collect_for_check` catches a ParseError per summary and carries on, which is
        right for the gate and wrong here: the stem that failed to parse renders nothing,
        so the prune would remove that RFC's tracked file and the run would still exit 0.
        Driven from `run_write`, the entry point, not from the guard's helper.

        The collection carries a parse error AND renders normally, which is the shape of
        the real defect: one summary of 178 fails while the rest are fine. A fixture that
        rendered nothing would trip the empty-render guard instead, and then `if parse_errs
        and not shards` would keep this test green while the defect was live.
        """
        with _shard_tree():
            os.makedirs(R.SHARD_DIR, exist_ok=True)
            # A stem the render does NOT produce: the file the prune would delete, and the
            # one whose RFC lost its rows because its summary is the one that failed.
            victim = os.path.join(R.SHARD_DIR, "rfc0000.md")
            with open(victim, "w", encoding="utf-8") as fh:
                fh.write("# RFC0000 -- rows an earlier write produced\n")

            code, out = self._write_with(
                lambda: (
                    {"rfc7606"},
                    _SHARD_REQS,
                    ["rfc/short/rfc0000.md: unparseable table"],
                    _SHARD_TAGS,
                    {},
                )
            )
            self.assertEqual(code, 2, out)
            self.assertIn("unparseable table", out, "the swallowed error is printed")
            self.assertIn("refusing to write", out)
            self.assertTrue(
                os.path.exists(victim),
                "the file whose summary failed to parse must survive; the prune runs on "
                "a trusted collection only",
            )
            self.assertEqual(
                _read_dir(R.SHARD_DIR),
                {"rfc0000.md": "# RFC0000 -- rows an earlier write produced\n"},
                "a refused write writes nothing either, so the two stems that DID render "
                "must not reach the directory",
            )

    def test_an_empty_render_refuses_the_write_and_deletes_nothing(self):
        """An absent `rfc/short/` makes `summary_stems` return an empty set, so every file
        in the directory becomes an orphan and one green run empties it. The refusal is
        what stops a sparse or half-checked-out tree from deleting all 177."""
        with _shard_tree():
            os.makedirs(R.SHARD_DIR, exist_ok=True)
            path = os.path.join(R.SHARD_DIR, "rfc7606.md")
            with open(path, "w", encoding="utf-8") as fh:
                fh.write("# a file an earlier write produced\n")

            code, out = self._write_with(lambda: (set(), [], [], [], {}))
            self.assertEqual(code, 2, out)
            self.assertIn("refusing to write", out)
            self.assertTrue(
                os.path.exists(path), "nothing is deleted on an empty render"
            )

    def test_write_emits_one_file_per_stem(self):
        """AC-1, the wiring test: `make ze-rfc-index-update` reaches the shard directory."""
        with _shard_tree():
            code, out = self._write()
            self.assertEqual(code, 0, out)
            self.assertEqual(
                sorted(_read_dir(R.SHARD_DIR)), ["rfc7606.md", "rfc9999.md"], out
            )
            self.assertTrue(os.path.exists(R.LEDGER_FILE), "the index is still written")

    def test_a_stem_with_no_requirement_renders_no_shard(self):
        """The zero boundary. A summary that declares nothing rendered no section before
        the split, so it must render no shard after it -- and the prune must not then be
        asked to delete a file the render never wrote."""
        self.assertEqual(R.render_shards([], [], set()), {})

    def test_write_prunes_orphan_shard(self):
        """AC-7: a shard whose stem no longer renders is deleted, so a retired RFC cannot
        leave a stale page that reads as current (R-1)."""
        with _shard_tree():
            os.makedirs(R.SHARD_DIR, exist_ok=True)
            orphan = os.path.join(R.SHARD_DIR, "rfc0000.md")
            with open(orphan, "w", encoding="utf-8") as fh:
                fh.write("# RFC0000 -- an RFC that no longer renders\n")
            other = os.path.join(R.SHARD_DIR, "notes.txt")
            with open(other, "w", encoding="utf-8") as fh:
                fh.write("not markdown, not the generator's\n")
            readme = os.path.join(R.SHARD_DIR, "README.md")
            with open(readme, "w", encoding="utf-8") as fh:
                fh.write("# what this directory holds\n")
            bare = os.path.join(R.SHARD_DIR, ".md")
            with open(bare, "w", encoding="utf-8") as fh:
                fh.write("a name whose stem is empty\n")
            # Named `sub.md`, not `sub`: a bare `sub` is already excluded by the `*.md`
            # test, so it pinned nothing and the isdir guard could be deleted with every
            # test still green.
            sub = os.path.join(R.SHARD_DIR, "sub.md")
            os.makedirs(sub)
            nested = os.path.join(sub, "rfc0000.md")
            with open(nested, "w", encoding="utf-8") as fh:
                fh.write("# in a subdirectory\n")

            code, out = self._write()
            self.assertEqual(code, 0, out)
            self.assertFalse(os.path.exists(orphan), out)
            self.assertIn("rfc0000", out, "the prune names what it deleted")
            self.assertTrue(
                os.path.exists(other), "the prune deletes markdown files only"
            )
            self.assertTrue(
                os.path.exists(readme),
                "an authored README beside the generated files must survive: its stem is "
                "not a summary stem, so the generator does not own it",
            )
            self.assertTrue(
                os.path.exists(bare),
                "a bare `.md` has an empty stem and is not a shard",
            )
            self.assertTrue(os.path.isdir(sub), "the prune never removes a directory")
            self.assertTrue(
                os.path.exists(nested), "the prune never descends into a subdirectory"
            )

    def test_second_write_is_a_no_op(self):
        """AC-12: the shard directory is an output, never an input. A generator that read
        its own output would double every count on the second run, and the prune would
        churn files it had just written."""
        with _shard_tree():
            self.assertEqual(self._write()[0], 0)
            first_shards = _read_dir(R.SHARD_DIR)
            with open(R.LEDGER_FILE, encoding="utf-8") as fh:
                first_index = fh.read()

            code, out = self._write()
            self.assertEqual(code, 0, out)
            self.assertEqual(_read_dir(R.SHARD_DIR), first_shards, out)
            with open(R.LEDGER_FILE, encoding="utf-8") as fh:
                self.assertEqual(fh.read(), first_index, out)
            self.assertNotIn("deleted", out, "the second run deletes nothing")


class TestShardFreshness(unittest.TestCase):
    """AC-5, AC-6: freshness compares the index, every shard, AND the set of files present.

    A whole-file comparison carried deletion detection for free: bytes that vanished were
    bytes that differed. Many files do not carry it, so each of the three staleness states
    is driven here (R-1).
    """

    _reqs = _SHARD_REQS
    _tags = _SHARD_TAGS
    _enrolled = {"rfc7606"}

    @contextlib.contextmanager
    def _tree(self):
        """A scratch tree holding the CORRECT index and the CORRECT shards.

        Each test below breaks exactly ONE file, so the message it reads names that file and
        the case cannot pass for an unrelated difference. The bodies are rendered inside the
        patch because the index cites a shard path.
        """
        with _shard_tree():
            os.makedirs(R.SHARD_DIR, exist_ok=True)
            with open(R.LEDGER_FILE, "w", encoding="utf-8") as fh:
                fh.write(R.render_index(self._reqs, self._tags, self._enrolled) + "\n")
            for stem, body in R.render_shards(
                self._reqs, self._tags, self._enrolled
            ).items():
                with open(R.shard_path(stem), "w", encoding="utf-8") as fh:
                    fh.write(body + "\n")
            yield

    def _check(self):
        return R.check_ledger_fresh(self._reqs, self._tags, self._enrolled)

    def test_index_and_shards_together_read_fresh(self):
        """The discriminating twin: without it the three cases below could pass against a
        check that calls everything stale."""
        with self._tree():
            self.assertEqual(self._check(), [])

    def test_edited_shard_is_stale(self):
        """AC-5: a hand edit to one shard fails the gate, and the message names that shard
        rather than the index a reader would otherwise regenerate and find unchanged."""
        with self._tree():
            with open(R.shard_path("rfc7606"), "a", encoding="utf-8") as fh:
                fh.write("hand-edited\n")
            errs = self._check()
            self.assertEqual(len(errs), 1, errs)
            self.assertIn(R.shard_rel("rfc7606"), errs[0])
            self.assertIn("ze-rfc-index-update", errs[0])

    def test_missing_shard_is_stale(self):
        """AC-5: a deleted shard is the state a byte comparison over one file used to catch
        for nothing. The index alone still matches, so only a per-file check can see it."""
        with self._tree():
            os.unlink(R.shard_path("rfc9999"))
            errs = self._check()
            self.assertEqual(len(errs), 1, errs)
            self.assertIn(R.shard_rel("rfc9999"), errs[0])
            self.assertIn("ze-rfc-index-update", errs[0])

    def test_orphan_shard_is_stale(self):
        """AC-6: a markdown file the render did not produce is an RFC page that reads as
        current and is not (R-1).

        The three limits are the prune's own, so the file the gate names is exactly the file
        the write would delete: the non-markdown file and the subdirectory beside it are
        untouched by both.
        """
        with self._tree():
            with open(
                os.path.join(R.SHARD_DIR, "rfc0000.md"), "w", encoding="utf-8"
            ) as fh:
                fh.write("# RFC0000 -- an RFC that no longer renders\n")
            with open(
                os.path.join(R.SHARD_DIR, "notes.txt"), "w", encoding="utf-8"
            ) as fh:
                fh.write("not markdown, not the generator's\n")
            os.makedirs(os.path.join(R.SHARD_DIR, "sub"))
            with open(
                os.path.join(R.SHARD_DIR, "sub", "rfc0001.md"), "w", encoding="utf-8"
            ) as fh:
                fh.write("# in a subdirectory\n")

            errs = self._check()
            self.assertEqual(len(errs), 1, errs)
            self.assertIn(R.shard_rel("rfc0000"), errs[0])
            self.assertIn("ze-rfc-index-update", errs[0])

    def test_the_gate_names_what_the_write_deletes(self):
        """The gate and the prune must never disagree about what the generator owns. The
        orphan the gate reported is the orphan the write removes, and the write then reads
        fresh."""
        with self._tree():
            orphan = os.path.join(R.SHARD_DIR, "rfc0000.md")
            with open(orphan, "w", encoding="utf-8") as fh:
                fh.write("# RFC0000 -- an RFC that no longer renders\n")
            self.assertEqual(len(self._check()), 1)
            removed = R.prune_shards(
                set(R.render_shards(self._reqs, self._tags, self._enrolled))
            )
            self.assertEqual(removed, ["rfc0000"])
            self.assertFalse(os.path.exists(orphan))
            self.assertEqual(self._check(), [])


class TestShardShow(unittest.TestCase):
    """AC-8, AC-9 and the Security Review row: one stem in, one shard out.

    The mode reads the shard FROM DISK, which is the reason it exists -- reading is
    instant, and freshness stays the gate's job (Key Design Decisions).
    """

    _reqs = _SHARD_REQS
    _tags = _SHARD_TAGS
    _enrolled = {"rfc7606"}
    _SENTINEL = "SENTINEL: this line exists only on disk"

    @contextlib.contextmanager
    def _tree(self):
        """Shards on disk, each carrying a line no render produces. A mode that re-rendered
        the stem instead of reading the file would print the rows without the sentinel."""
        with _shard_tree():
            os.makedirs(R.SHARD_DIR, exist_ok=True)
            for stem, body in R.render_shards(
                self._reqs, self._tags, self._enrolled
            ).items():
                with open(R.shard_path(stem), "w", encoding="utf-8") as fh:
                    fh.write(body + "\n" + self._SENTINEL + "\n")
            yield

    def _show(self, *argv):
        return _run_capturing(lambda: R.main(["prog", "--show", *argv]))

    def test_show_prints_shard(self):
        """AC-8: the stem resolves to its file, and the bytes come off the disk."""
        with self._tree():
            code, out = self._show("rfc7606")
            self.assertEqual(code, 0, out)
            self.assertIn(_ROW_HEADER, out)
            self.assertIn("RFC7606-2-1", out)
            self.assertIn(self._SENTINEL, out)
            self.assertNotIn("RFC9999-1-1", out, "one stem prints one shard")

    def test_show_accepts_uppercase_stem(self):
        """AC-8: a reader who types the stem the way the RFC spells it gets the same page.
        The stems on disk are lower case, so this is case folding, not a second file."""
        with self._tree():
            lower = self._show("rfc7606")
            upper = self._show("RFC7606")
            self.assertEqual(upper[0], 0, upper[1])
            self.assertEqual(upper[1], lower[1])

    def test_show_unknown_stem_exits_two(self):
        """AC-9: both halves. A stem with no shard, and no stem at all, exit 2 and name the
        command that writes the shards."""
        with self._tree():
            code, out = self._show("rfc0000")
            self.assertEqual(code, 2, out)
            self.assertIn("ze-rfc-index-update", out)

            code, out = _run_capturing(lambda: R.main(["prog", "--show"]))
            self.assertEqual(code, 2, out)
            self.assertIn("ze-rfc-index-update", out)

    def test_show_refuses_a_separator_in_the_stem(self):
        """Security Review row: the stem becomes a path, so it may never carry one.

        Discriminating: the planted file really exists one directory above the shards and is
        reachable by the traversal, so an unvalidated mode prints it. The upper-case spelling
        is driven too, because the case folding runs BEFORE the validator -- lowering maps
        letters and never a separator or a dot, so it cannot launder an escape.
        """
        with self._tree():
            outside = os.path.join(os.path.dirname(R.SHARD_DIR), "secret.md")
            with open(outside, "w", encoding="utf-8") as fh:
                fh.write("SECRET: outside the shard directory\n")
            for stem in ("../secret", "../SECRET", "sub/rfc7606", "..", "/etc/passwd"):
                code, out = self._show(stem)
                self.assertEqual(code, 2, f"{stem}: {out}")
                self.assertNotIn("SECRET: outside", out, stem)
                # The validator's own words. The usage text a missing mode prints also
                # carries the word "stem", so a looser assertion would pass against no
                # mode at all.
                self.assertIn("not an RFC or draft stem", out, stem)


# The rows below are VERBATIM from the ledger as it stood before the split: one fully
# populated row, one with neither polarity proven, one carrying an annotation. Between them
# every cell of the row is filled, and the two that can be empty (the test cells and the
# note) are seen both ways. Pinned as literals on purpose -- a test that
# rebuilt the expected string from the same f-string the renderer uses would agree with any
# format the renderer drifted to, which is the vacuity trap in
# ai/rules/interop-and-goal-validation.md ("Prove the test discriminates").
_PRE_SPLIT_ROWS = {
    "RFC4271-10-1": (
        "| `RFC4271-10-1` | MUST | 10 | "
        "`internal/component/bgp/reactor/rfc4271_test.go` "
        "`TestRFC4271HoldTimeConfigurablePerPeer` (unit/verify) | "
        "`internal/component/bgp/reactor/rfc4271_test.go` "
        "`TestRFC4271PerPeerHoldTimeSurvivesNegotiation` (unit/verify) |  |"
    ),
    "RFC7606-2-4": "| `RFC7606-2-4` | MAY | 2 | -- | -- |  |",
    "RFC4659-4-3": (
        "| `RFC4659-4-3` | MUST | 4 | -- | -- | {not-applicable} a data-plane "
        "transport-selection decision for a forwarding PE, a role ze does not perform |"
    ),
}


def _rows_by_stem(shards):
    """stem -> {rid: row}, reading each shard's requirement table the way a human does.

    A row counts only after the table header, so a shard's title and banner cannot be
    mistaken for content.
    """
    out = {}
    for stem, body in shards.items():
        rows, in_table = {}, False
        for line in body.split("\n"):
            if line == _ROW_HEADER:
                in_table = True
                continue
            if not in_table:
                continue
            if not line.startswith("| `"):
                if line.startswith("|"):
                    continue  # the |---|---| separator
                in_table = False
                continue
            rid = line.split("`")[1]
            rows[rid] = line
        out[stem] = rows
    return out


class TestShardMigration(unittest.TestCase):
    """AC-10: the split moved the requirement rows, it did not rewrite them.

    Two halves, and the second is the one only this test owns:

    - the row TEXT keeps the shape the pre-split ledger used, pinned as a literal string;
    - a requirement id renders in exactly ONE shard. Before the split every row lived in one
      file, so "exactly one" was free. Now it is a property of `shard_stems` and the
      per-stem grouping in `render_shards`, and nothing else asserts it.

    The one-time comparison of the CAPTURED pre-split ledger against the real tree cannot
    live here: that capture is session scratch, so a test reading it would pass today and
    error or silently skip forever after. Its numbers are recorded in
    plan/spec-rfc-ledger-per-rfc-shards.md under AC-10.
    """

    _REQS = [
        R.Requirement(
            rfc="rfc4271",
            rid="RFC4271-10-1",
            level="MUST",
            text="x",
            section="10",
            annotation=None,
            source="rfc/short/rfc4271.md",
            line=1,
        ),
        R.Requirement(
            rfc="rfc7606",
            rid="RFC7606-2-4",
            level="MAY",
            text="x",
            section="2",
            annotation=None,
            source="rfc/short/rfc7606.md",
            line=1,
        ),
        R.Requirement(
            rfc="rfc4659",
            rid="RFC4659-4-3",
            level="MUST",
            text="x",
            section="4",
            annotation=R.Annotation(
                kind="not-applicable",
                polarity=None,
                reason=(
                    "a data-plane transport-selection decision for a forwarding PE, "
                    "a role ze does not perform"
                ),
            ),
            source="rfc/short/rfc4659.md",
            line=1,
        ),
    ]
    _TAGS = [
        _tag(
            "RFC4271-10-1",
            "positive",
            file="internal/component/bgp/reactor/rfc4271_test.go",
            line=269,
        ),
        _tag(
            "RFC4271-10-1",
            "negative",
            file="internal/component/bgp/reactor/rfc4271_test.go",
            line=293,
        ),
    ]
    _ENROLLED = {"rfc4271", "rfc7606", "rfc4659"}

    def _rows(self):
        # load_audits is neutralised so the fixture answers from the fixture alone. The real
        # rfc/audit/rfc7606.json holds no verdict for RFC7606-2-4 today, which is why the
        # pre-split row's note cell is empty -- but an audit landing later would append a
        # marker and redden this test for a reason that has nothing to do with the migration.
        with _patched(load_audits=lambda *a, **k: {}):
            return _rows_by_stem(
                R.render_shards(self._REQS, self._TAGS, self._ENROLLED)
            )

    def test_every_requirement_row_survives(self):
        """AC-10: each pre-split row renders, byte for byte, in the one shard that owns it."""
        by_stem = self._rows()
        for req in self._REQS:
            expected = _PRE_SPLIT_ROWS[req.rid]
            carriers = sorted(s for s, rows in by_stem.items() if req.rid in rows)
            self.assertEqual(
                carriers,
                [req.rfc],
                f"{req.rid} must render in exactly one shard, its own",
            )
            self.assertEqual(by_stem[req.rfc][req.rid], expected, req.rid)

        # Nothing beyond the input renders, so a row cannot survive by being duplicated into
        # a second shard, and no shard is written for a stem that declares nothing.
        rendered = sorted(rid for rows in by_stem.values() for rid in rows)
        self.assertEqual(rendered, sorted(r.rid for r in self._REQS))
        self.assertEqual(sorted(by_stem), sorted({r.rfc for r in self._REQS}))


# --------------------------------------------------------------------------
# ID-reuse ratchet wiring (AC-2) — GATE LEVEL, not just the helper
# --------------------------------------------------------------------------
class TestIDAllocationWiring(unittest.TestCase):
    """check_id_allocation is dead code unless run_check actually calls it with the git
    baseline. These drive run_check end-to-end: if the wiring regresses (the call is
    removed, or fed an empty baseline), the reuse goes undetected and the test fails."""

    def _drive(self, baseline_ids):
        reused = _req("RFC7606-2-3")  # source rfc/short/rfc7606.md:1
        tags = [
            _tag("RFC7606-2-3", "positive"),
            _tag("RFC7606-2-3", "negative"),
        ]
        with _patched(
            load_enrolled=lambda: {"rfc7606"},
            summary_stems=lambda: {"rfc7606"},
            parse_summary_file=lambda path: [reused],
            _git_baseline_enrolment=lambda: {"rfc7606"},
            _git_baseline_ids=lambda: baseline_ids,
            scan_tree=lambda *a, **k: tags,
            # Scoped to its subject: this fixture's baseline deliberately holds ids absent
            # from the tree, which is a genuine finding for the retired-id check and noise
            # for this one.
            check_retired_requirements=lambda *a, **k: [],
            check_status_agreement=lambda *a, **k: [],
            check_audit_freshness=lambda *a, **k: [],
            # The rest of the audit half, neutralised for the same reason the line above
            # is: this driver's subject is not the audit record, and the REAL
            # rfc/audit/rfc7606.json legitimately describes 52 requirements this fixture
            # does not declare (spec-rfcgate-3-audit-teeth.md).
            check_audit_files=lambda *a, **k: [],
            check_audit_schema=lambda *a, **k: [],
            check_audit_disclosure=lambda *a, **k: [],
            check_audit_note=lambda *a, **k: [],
            check_audit_findings=lambda *a, **k: [],
            check_audit_verdict_ratchet=lambda *a, **k: [],
            # The four ledger-edge checks (plan/spec-rfcgate-4-ledger.md), neutralised for
            # the same reason check_status_agreement above is: they read the REAL
            # rfc/not-enrolled.txt and docs/features/rfc-status.md, and this driver
            # declares a SYNTHETIC summary universe. Judging 157 real status rows and 7
            # real dispositions against it produces a wall of violations that has nothing
            # to do with this driver's subject. Each of the four has its own wiring class,
            # which is where a lost call site is caught.
            # The extraction half, neutralised for the same reason: evaluate_extractions
            # reads the REAL rfc/extraction/*.json, whose sites name requirement ids from
            # the REAL summaries, not from this driver's synthetic set.
            signed_extractions=lambda reqs_: {},
            check_extraction_signoff=lambda *a, **k: [],
            check_extraction_ratchet=lambda *a, **k: [],
            check_drain_floor=lambda *a, **k: [],
            check_summary_disposition=lambda *a, **k: [],
            check_status_completeness=lambda *a, **k: [],
            check_unproven_support=lambda *a, **k: [],
            check_gap_count_agreement=lambda *a, **k: [],
            check_ledger_fresh=lambda *a, **k: [],
        ):
            return _run_capturing(R.run_check)

    def test_run_check_fails_on_reused_id(self):
        """AC-2: §2 allocated up to 2-5, then 2-3 was retired. Re-adding a DIFFERENT 2-3
        (below the high-water, absent from the committed baseline) must fail the GATE,
        proving check_id_allocation is wired into run_check with the real baseline."""
        code, out = self._drive({"RFC7606-2-1", "RFC7606-2-2", "RFC7606-2-5"})
        self.assertEqual(code, 2, out)
        self.assertIn("RFC7606-2-3", out)
        self.assertIn("reuses a retired id", out)

    def test_run_check_allows_text_edit_of_existing_id(self):
        """Discriminates from 'always fails': when 2-3 IS in the committed baseline the id
        is a text edit, not a reuse, so the same run_check reports clean (exit 0)."""
        code, out = self._drive({"RFC7606-2-1", "RFC7606-2-3", "RFC7606-2-5"})
        self.assertEqual(code, 0, out)
        self.assertNotIn("reuses a retired id", out)


# --------------------------------------------------------------------------
# Coverage ratchet (spec-rfc-gate-regression-ratchets AC-5..AC-8)
# --------------------------------------------------------------------------
class TestCoverageRatchet(unittest.TestCase):
    """Enrolment was already monotonic; PROOF was not. A requirement carrying a positive
    and a negative test could be demoted to {gap} and the gate stayed green, because
    evaluate() only ever reads the working tree. Reading the tree alone cannot tell
    'never proven' from 'stopped being proven'."""

    def _errs(self, reqs, tags, baseline, enrolled=("rfc7606",), base_enrolled=None):
        return R.check_coverage_ratchet(
            requirements=reqs,
            tags=tags,
            enrolled=set(enrolled),
            baseline_polarities=baseline,
            baseline_enrolled=set(enrolled if base_enrolled is None else base_enrolled),
        )

    def test_lost_polarity_fails(self):
        """AC-5: the negative test is gone. The positive one alone cannot tell correct
        behavior from blanket accept."""
        errs = self._errs(
            [_req("RFC7606-2-1")],
            [_tag("RFC7606-2-1", "positive")],
            {"RFC7606-2-1": {"positive", "negative"}},
        )
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("RFC7606-2-1", errs[0])
        self.assertIn("negative", errs[0])

    def test_gap_annotation_is_not_an_escape(self):
        """AC-6: annotating {gap} is EXACTLY the downgrade being blocked. Letting the
        annotation satisfy the ratchet would make the ratchet self-service."""
        ann = R.Annotation(kind="gap", polarity=None, reason="hard")
        errs = self._errs(
            [_req("RFC7606-2-1", annotation=ann)],
            [],
            {"RFC7606-2-1": {"positive", "negative"}},
        )
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("positive", errs[0])
        self.assertIn("negative", errs[0])

    def test_moved_tags_are_clean(self):
        """AC-7: discriminates from 'always fails'. A refactor that moves both tags to
        another file changes no polarity, so the ratchet must stay silent -- otherwise it
        fires on ordinary work and gets routed around."""
        errs = self._errs(
            [_req("RFC7606-2-1")],
            [
                _tag("RFC7606-2-1", "positive", file="moved_test.go"),
                _tag("RFC7606-2-1", "negative", file="moved_test.go", line=9),
            ],
            {"RFC7606-2-1": {"positive", "negative"}},
        )
        self.assertEqual(errs, [])

    def test_no_baseline_no_violation(self):
        """AC-8/AC-14: a requirement with no committed baseline (new id, or git could not
        answer) has nothing to lose. Fail-closed applies to judging evidence, not to
        inventing evidence that was never there."""
        errs = self._errs([_req("RFC7606-2-1")], [], {})
        self.assertEqual(errs, [])

    def test_skips_rfc_not_enrolled_at_baseline(self):
        """Scope limit: an RFC enrolled in THIS change is judged by evaluate()'s normal
        rules, where a {gap} is a legitimate starting position. Ratcheting it against a
        pre-enrolment state would forbid enrolling anything with a known gap."""
        errs = self._errs(
            [_req("RFC7606-2-1")],
            [],
            {"RFC7606-2-1": {"positive", "negative"}},
            enrolled=("rfc7606",),
            base_enrolled=(),
        )
        self.assertEqual(errs, [])

    def test_duplicate_id_reports_one_loss_not_two(self):
        """Two summary lines sharing an id (a wrapped or duplicated entry) describe ONE
        requirement, so losing its tests is one violation. Emitting one per line pads the
        report and makes the count meaningless."""
        errs = self._errs(
            [_req("RFC7606-2-1"), _req("RFC7606-2-1")],
            [],
            {"RFC7606-2-1": {"positive", "negative"}},
        )
        self.assertEqual(len(errs), 1, errs)

    def test_advisory_requirement_is_ratcheted_too(self):
        """A SHOULD is not gated by evaluate(), but a test that PROVED it and then vanished
        is still lost evidence. The ratchet is about what existed, not about level."""
        errs = self._errs(
            [_req("RFC7606-2-1", level="SHOULD")],
            [],
            {"RFC7606-2-1": {"positive"}},
        )
        self.assertEqual(len(errs), 1, errs)


class TestLevelChangeUnderStableIdIsAccepted(unittest.TestCase):
    """spec-fixit-rfc-row-level-and-anchor-drift AC-2, over the repair it actually made.

    Correcting a misquoted level means a row leaves the gated MUST-level population
    while keeping its id and every test that proves it. Two ratchets sit on that path
    and NEITHER may fire, because the alternative route -- delete the row -- is the one
    move they exist to refuse. `test_advisory_requirement_is_ratcheted_too` above shows
    a SHOULD losing a polarity still fires; it says nothing about a level CHANGE, which
    is the move here.

    Since 2026-08-17 a THIRD ratchet sits on the same path and it does judge the level:
    `check_level_ratchet` (TestLevelRatchet) refuses the demotion unless the summary
    records the correction. These two stay as they are -- the demotion must not read as a
    retirement or as lost proof, whatever the level ratchet says about it -- but "costless"
    is now wrong: the cost is a `Correction <date>:` paragraph quoting the RFC.

    The demotion must stay costless only while the proof survives. The second case is
    what keeps the first from being vacuous: demote AND drop the negative, and the
    ratchet fires. So this pair says "a level may fall, its evidence may not".
    """

    def _ratchet(self, reqs, tags, baseline):
        return R.check_coverage_ratchet(
            requirements=reqs,
            tags=tags,
            enrolled={"rfc7296"},
            baseline_polarities=baseline,
            baseline_enrolled={"rfc7296"},
        )

    def test_demotion_keeping_both_polarities_is_silent(self):
        """RFC7296-2.8-1 went [MUST] -> [SHOULD] on 2026-08-15 with all six tags in
        place. The ratchet compares polarity sets, never levels, so it must say nothing."""
        errs = self._ratchet(
            [_req("RFC7296-2.8-1", level="SHOULD", rfc="rfc7296")],
            [
                _tag("RFC7296-2.8-1", "positive"),
                _tag("RFC7296-2.8-1", "negative", line=9),
            ],
            {"RFC7296-2.8-1": {"positive", "negative"}},
        )
        self.assertEqual(errs, [])

    def test_demotion_that_drops_a_polarity_still_fires(self):
        """Discrimination: the level is not a licence to shed proof. Lowering the row
        AND losing its negative is evidence loss, and the ratchet must still catch it."""
        errs = self._ratchet(
            [_req("RFC7296-2.8-1", level="SHOULD", rfc="rfc7296")],
            [_tag("RFC7296-2.8-1", "positive")],
            {"RFC7296-2.8-1": {"positive", "negative"}},
        )
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("negative", errs[0])

    def _retired(self, requirements):
        return R.check_retired_requirements(
            requirements=requirements,
            enrolled={"rfc7296"},
            baseline_ids={"RFC7296-2.8-1"},
            baseline_enrolled={"rfc7296"},
            stems={"rfc7296"},
        )

    def test_demoted_id_is_not_read_as_retired(self):
        """The other ratchet on the path. `check_retired_requirements` keys on ids, so a
        row that changed level but is still listed must not read as a deleted one --
        that verdict would push the next session toward deleting it for real."""
        errs = self._retired([_req("RFC7296-2.8-1", level="SHOULD", rfc="rfc7296")])
        self.assertEqual(errs, [])

    def test_the_same_id_actually_deleted_is_still_caught(self):
        """Discrimination for the case above, which asserts an ABSENCE and would pass
        against a `check_retired_requirements` that returned nothing at all. Delete the
        row instead of correcting its level and the ratchet must fire, so the silence
        above is a verdict about THIS input rather than the function's only answer."""
        errs = self._retired([_req("RFC7296-9.9-1", level="MUST", rfc="rfc7296")])
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("RFC7296-2.8-1", errs[0])


class TestBaselineReaders(unittest.TestCase):
    """The two new HEAD readers. Both must degrade to 'no baseline' rather than crash or
    invent one: the gate runs in worktrees, in CI checkouts and on the first commit."""

    def test_tag_polarities_survives_git_failure(self):
        """AC-14. Only `git grep` fails; the following cat-file SUCCEEDS with a tag-bearing
        blob. An implementation that ignores the grep returncode therefore returns a real
        baseline here, instead of being masked by the second command failing too."""
        with _patched(subprocess=_FakeSubprocess(returncode=128, fail_only="grep")):
            self.assertEqual(R._git_baseline_tag_polarities(), {})

    def test_extraction_baseline_reads_head_artifacts(self):
        """`_git_baseline_extractions` was never executed: every ratchet test patches it
        out, so its JSON parse, its exclusion count and its signed-off/resign-reason
        extraction were all uncovered -- `excluded = 0` survived. It is the ONLY input to
        both exclusion ratchets, so a reader that always returns zero exclusions makes
        every rise look like a first sign-off and the ratchet never fires.

        Driven through _FakeSubprocess, which serves the `git ls-tree` listing as text and
        the `git cat-file --batch` payload as bytes (it switches on `input=`)."""
        blob = json.dumps(
            {
                "sites": [
                    {"disposition": "excluded"},
                    {"disposition": "mapped"},
                    {"disposition": "excluded"},
                    {"disposition": None},
                ],
                "signed-off": "2026-07-29",
                "resign-reason": "a previous walk",
            }
        ).encode()
        payload = b"aaa blob %d\n%s\n" % (len(blob), blob)
        with _patched(
            subprocess=_FakeSubprocess(
                returncode=0,
                stdout="rfc/extraction/rfc9999.json\0",
                stdout_bytes=payload,
            )
        ):
            got = R._git_baseline_extractions()
        self.assertEqual(
            got,
            {"rfc9999": R.BaselineExtraction(2, "2026-07-29", "a previous walk")},
        )

    def test_extraction_baseline_skips_an_unparseable_head_artifact(self):
        """A committed artifact that no longer parses contributes NO baseline for its stem
        rather than a zeroed one. A zeroed baseline would read as 'zero exclusions at
        HEAD', so every exclusion in the working tree would look like a rise."""
        payload = b"aaa blob 5\n{ bad\n"
        with _patched(
            subprocess=_FakeSubprocess(
                returncode=0,
                stdout="rfc/extraction/rfc9999.json\0",
                stdout_bytes=payload,
            )
        ):
            self.assertEqual(R._git_baseline_extractions(), {})

    def test_extraction_baseline_with_no_artifacts_at_head(self):
        """An empty listing is a real answer ({}), not a failure (None): it is the ordinary
        first-commit state."""
        with _patched(subprocess=_FakeSubprocess(returncode=0, stdout="")):
            self.assertEqual(R._git_baseline_extractions(), {})

    def test_enrolment_baseline_returns_none_on_git_failure(self):
        """None, not set(). The enrolment baseline feeds TWO consumers with OPPOSITE
        polarities: `baseline - current` (the un-enrolment ratchet, where an empty set
        accuses nobody) and `current - baseline` (the new-enrolment sign-off precondition,
        where an empty set accuses EVERY enrolled RFC). Only the reader knows which case it
        is in, so it must not hand both consumers the same answer.

        The fake returns a plausible enrolled.txt beside the failure code, so a reader that
        ignores the returncode yields {'rfc7606'} and fails here rather than being masked
        by an empty stdout."""
        with _patched(
            subprocess=_FakeSubprocess(returncode=128, stdout="rfc7606\nrfc4271\n")
        ):
            self.assertIsNone(R._git_baseline_enrolment())

    def test_enrolment_baseline_empty_is_distinct_from_failure(self):
        """A successful read of an empty rfc/enrolled.txt is set(), never None: the
        distinction is the whole point of the reader."""
        with _patched(subprocess=_FakeSubprocess(returncode=0, stdout="")):
            self.assertEqual(R._git_baseline_enrolment(), set())

    def test_summary_stems_returns_none_on_git_failure(self):
        """AC-14, and the distinction that matters: None, not set().

        check_new_summaries consumes this as `stems - baseline_stems`, where an empty set
        accuses EVERY summary of being new. The other two baselines can conflate 'could not
        look' with 'nothing there' because they are consumed as `baseline - current`; this
        one cannot. The fake returns a real-looking listing with the failure code, so a
        missing returncode guard yields {'x'} and fails this."""
        with _patched(
            subprocess=_FakeSubprocess(returncode=128, stdout="rfc/short/x.md\0")
        ):
            self.assertIsNone(R._git_baseline_summary_stems())

    def test_summary_stems_empty_is_distinct_from_failure(self):
        """A successful call over an empty rfc/short returns set(), not None: the caller
        treats both as 'judge nothing', but conflating them in the READER would hide a
        genuine git failure from any future caller that wants to tell them apart."""
        with _patched(subprocess=_FakeSubprocess(returncode=0, stdout="")):
            self.assertEqual(R._git_baseline_summary_stems(), set())

    def test_tag_polarities_treats_no_match_as_a_real_answer(self):
        """git grep exits 1 when nothing matched: 'HEAD has no tags', not a failure. The
        fake supplies output that WOULD parse into a baseline, so this discriminates
        between an implementation that accepts exit 1 and one that rejects it -- both
        return {} here, but only because exit 1 legitimately found nothing to read."""
        with _patched(subprocess=_FakeSubprocess(returncode=1, stdout="")):
            self.assertEqual(R._git_baseline_tag_polarities(), {})

    def test_id_baseline_reads_summaries_in_one_batch(self):
        """The ID ratchet must not start one git process per committed summary."""
        paths = ["rfc/short/rfc1.md", "rfc/short/rfc2.md"]
        blobs = {
            paths[0]: "- [ ] [RFC1-1-1] [MUST] first requirement (§1)\n",
            paths[1]: "- [ ] [RFC2-2-1] [MUST NOT] second requirement (§2)\n",
        }
        requested = []

        def cat_blobs(got):
            requested.extend(got)
            return blobs

        with _patched(
            subprocess=_FakeSubprocess(returncode=0, stdout="\0".join(paths) + "\0"),
            _git_cat_blobs=cat_blobs,
        ):
            got = R._git_baseline_ids()

        self.assertEqual(requested, paths)
        self.assertEqual(got, {"RFC1-1-1", "RFC2-2-1"})

    def test_cat_blobs_parses_a_batch(self):
        """The batch reader replaces one `git show` per file (350 forks). Its framing is
        hand-parsed, so pin it: two blobs and one missing path."""
        payload = (
            b"aaa blob 5\nhello\n"  # a.go
            b"bbb missing\n"  # b.go, absent from HEAD
            b"ccc blob 3\nxyz\n"  # c.ci
        )
        with _patched(subprocess=_FakeSubprocess(returncode=0, stdout_bytes=payload)):
            got = R._git_cat_blobs(["a.go", "b.go", "c.ci"])
        self.assertEqual(got, {"a.go": "hello", "c.ci": "xyz"})

    def test_cat_blobs_frames_a_non_blob_body(self):
        """A non-blob object still HAS a body. Skipping only its header would consume that
        body as the next header and silently drop EVERY following path -- a partial
        baseline that reads exactly like a complete one."""
        payload = b"aaa blob 2\nhi\nbbb tree 4\nXXXX\nccc blob 3\nend\n"
        with _patched(subprocess=_FakeSubprocess(returncode=0, stdout_bytes=payload)):
            got = R._git_cat_blobs(["a.go", "d", "b.go"])
        self.assertEqual(got, {"a.go": "hi", "b.go": "end"})

    def test_cat_blobs_zero_length_blob(self):
        payload = b"aaa blob 0\n\nbbb blob 2\nhi\n"
        with _patched(subprocess=_FakeSubprocess(returncode=0, stdout_bytes=payload)):
            got = R._git_cat_blobs(["a.go", "b.go"])
        self.assertEqual(got, {"a.go": "", "b.go": "hi"})

    def test_cat_blobs_desync_yields_nothing_not_a_partial(self):
        """A truncated or unparseable stream must return {}, never the prefix it managed to
        read. A partial baseline is worse than none: the ratchet silently stops covering
        every requirement in the dropped files while still reporting success."""
        with _patched(
            subprocess=_FakeSubprocess(returncode=0, stdout_bytes=b"aaa blob 99\nshort")
        ):
            self.assertEqual(R._git_cat_blobs(["a.go", "b.go"]), {})
        with _patched(
            subprocess=_FakeSubprocess(
                returncode=0, stdout_bytes=b"aaa blob 2\nhi\nbbb blob NaN\nxx\n"
            )
        ):
            self.assertEqual(R._git_cat_blobs(["a.go", "b.go"]), {})

    def test_cat_blobs_survives_git_failure(self):
        """The fake returns a well-formed blob body with the failure code: an
        implementation missing the returncode guard would return {'a.go': 'hello'}."""
        with _patched(subprocess=_FakeSubprocess(returncode=128)):
            self.assertEqual(R._git_cat_blobs(["a.go"]), {})

    def test_tolerant_scan_keeps_good_go_tags_past_a_bad_one(self):
        """The commit that FIXES a malformed tag is exactly when HEAD fails to parse and
        the tree succeeds. Dropping the whole file's baseline there would blind the ratchet
        in the one change most likely to be touching those tests."""
        blob = (
            "// RFC requirement: RFC7606-2-1 positive - ok\n"
            "// RFC requirement: RFC7606-2-2\n"  # no polarity: malformed
            "// RFC requirement: RFC7606-2-3 negative - ok\n"
        )
        got = R._scan_tags_tolerant(blob, "internal/x_test.go")
        self.assertEqual(
            {(t.rid, t.polarity) for t in got},
            {("RFC7606-2-1", "positive"), ("RFC7606-2-3", "negative")},
        )

    def test_tolerant_scan_drops_a_malformed_ci_whole(self):
        """A .ci cannot use the line-wise fallback: terminator= blocks make position
        meaningful (scan_ci_tags:510), and re-implementing that is the phantom-tag hazard
        the shared scanner exists to avoid.

        The blob carries a `//`-style line that the Go fallback WOULD match. Without the
        .ci guard the fallback invents RFC7606-9-9 as baseline proof -- a tag that the real
        .ci scanner never counted, whose later 'loss' the ratchet would then report. A
        fixture using only `#` tags could not tell the guard from its absence."""
        blob = (
            "# RFC requirement: RFC7606-2-1 positive - ok\n"
            "# RFC requirement: RFC7606-2-2\n"  # malformed: forces the fallback path
            "run=cat > x.go <<EOF\n"
            "// RFC requirement: RFC7606-9-9 positive - Go source INSIDE a .ci block\n"
            "EOF\n"
        )
        self.assertEqual(R._scan_tags_tolerant(blob, "test/x.ci"), [])

    def test_tolerant_scan_is_the_path_the_baseline_reader_uses(self):
        """Wiring: the tolerant scan is dead code unless _git_baseline_tag_polarities calls
        it. Pointing that reader back at the strict scanners must fail something."""
        blob = (
            "// RFC requirement: RFC7606-2-1 positive - ok\n"
            "// RFC requirement: RFC7606-2-2\n"  # malformed
        )
        payload = f"aaa blob {len(blob.encode())}\n{blob}\n".encode()
        with _patched(
            subprocess=_FakeSubprocess(
                returncode=0,
                stdout="HEAD:internal/x_test.go\0",
                stdout_bytes=payload,
            )
        ):
            got = R._git_baseline_tag_polarities()
        self.assertEqual(got, {"RFC7606-2-1": {"positive"}})

    def test_baseline_prunes_the_same_dirs_as_the_tree_scanner(self):
        """scan_tree never visits testdata/vendor/.git (:551). A tag there would enter the
        baseline and never the tree, so the ratchet would report 'no longer proven' for a
        test nobody removed -- a false red on the gate."""
        blob = "// RFC requirement: RFC7606-2-1 positive - ok\n"
        payload = f"aaa blob {len(blob.encode())}\n{blob}\n".encode()
        with _patched(
            subprocess=_FakeSubprocess(
                returncode=0,
                stdout="HEAD:internal/testdata/x_test.go\0",
                stdout_bytes=payload,
            )
        ):
            self.assertEqual(R._git_baseline_tag_polarities(), {})

    def test_baseline_skips_a_path_containing_a_newline(self):
        """cat-file --batch is newline-delimited: such a path would split one request into
        two and desync every blob after it. Losing one entry beats corrupting all of them."""
        blob = "// RFC requirement: RFC7606-2-1 positive - ok\n"
        payload = f"aaa blob {len(blob.encode())}\n{blob}\n".encode()
        with _patched(
            subprocess=_FakeSubprocess(
                returncode=0,
                stdout="HEAD:internal/we\nird_test.go\0",
                stdout_bytes=payload,
            )
        ):
            self.assertEqual(R._git_baseline_tag_polarities(), {})

    def test_baseline_reads_a_path_with_spaces_verbatim(self):
        """`git grep -z` emits the path verbatim, so it is used verbatim.

        Honest scope note: dropping the `.strip()` is not observable through this function
        for a LEADING or TRAILING space, because such a path no longer ends in `_test.go`
        and is filtered out either way -- the same empty result. What this pins is the
        interior-space case, which must survive."""
        blob = "// RFC requirement: RFC7606-2-1 positive - ok\n"
        payload = f"aaa blob {len(blob.encode())}\n{blob}\n".encode()
        with _patched(
            subprocess=_FakeSubprocess(
                returncode=0,
                stdout="HEAD:internal/two words_test.go\0",
                stdout_bytes=payload,
            )
        ):
            self.assertEqual(
                R._git_baseline_tag_polarities(), {"RFC7606-2-1": {"positive"}}
            )


class _FakeSubprocess:
    """Stands in for the `subprocess` module inside rfc_requirements only.

    A failing run returns PLAUSIBLE OUTPUT alongside the non-zero code, never empty output.
    An empty-stdout fake cannot tell a reader that checks `returncode` from one that does
    not: both return {}, and the test passes on an implementation with no guard at all.
    """

    _FAIL_STDOUT = "HEAD:internal/x_test.go\0"
    _FAIL_TAG = "// RFC requirement: RFC7606-2-1 positive - x\n"
    _FAIL_BYTES = f"deadbeef blob {len(_FAIL_TAG)}\n{_FAIL_TAG}\n".encode()

    def __init__(self, returncode=0, stdout=None, stdout_bytes=None, fail_only=None):
        """`fail_only` fails just the git subcommand containing that token.

        Needed because _git_baseline_tag_polarities runs TWO git commands and the second
        (_git_cat_blobs) has its own returncode guard. Failing both hides whether the first
        guard exists at all: the second returns {} regardless. Failing only `grep` lets an
        unguarded reader run on to a SUCCESSFUL cat-file and produce a real baseline, which
        is the difference the test needs to see.
        """
        self._rc = returncode
        self._fail_only = fail_only
        failing = returncode != 0
        if stdout is None:
            stdout = self._FAIL_STDOUT if failing else ""
        if stdout_bytes is None:
            stdout_bytes = self._FAIL_BYTES if failing else b""
        self._out = stdout
        self._bytes = stdout_bytes

    def run(self, argv, **kwargs):
        rc = self._rc
        if self._fail_only is not None and self._fail_only not in argv:
            rc = 0
        out, raw = self._out, self._bytes

        class _R:
            returncode = rc
            stdout = raw if kwargs.get("input") is not None else out

        return _R()


class TestCoverageRatchetWiring(unittest.TestCase):
    """check_coverage_ratchet is dead code unless run_check calls it with the real git
    baseline. These drive run_check end-to-end."""

    def _drive(self, tags, baseline):
        with _patched(
            load_enrolled=lambda: {"rfc7606"},
            summary_stems=lambda: {"rfc7606"},
            parse_summary_file=lambda path: [_req("RFC7606-2-1")],
            _git_baseline_enrolment=lambda: {"rfc7606"},
            _git_baseline_ids=lambda: {"RFC7606-2-1"},
            _git_baseline_tag_polarities=lambda: baseline,
            _git_baseline_evidence=lambda: {},
            _git_baseline_summary_stems=lambda: {"rfc7606"},
            scan_tree=lambda *a, **k: tags,
            check_status_agreement=lambda *a, **k: [],
            check_audit_freshness=lambda *a, **k: [],
            # The rest of the audit half, neutralised for the same reason the line above
            # is: this driver's subject is not the audit record, and the REAL
            # rfc/audit/rfc7606.json legitimately describes 52 requirements this fixture
            # does not declare (spec-rfcgate-3-audit-teeth.md).
            check_audit_files=lambda *a, **k: [],
            check_audit_schema=lambda *a, **k: [],
            check_audit_disclosure=lambda *a, **k: [],
            check_audit_note=lambda *a, **k: [],
            check_audit_findings=lambda *a, **k: [],
            check_audit_verdict_ratchet=lambda *a, **k: [],
            # The four ledger-edge checks (plan/spec-rfcgate-4-ledger.md), neutralised for
            # the same reason check_status_agreement above is: they read the REAL
            # rfc/not-enrolled.txt and docs/features/rfc-status.md, and this driver
            # declares a SYNTHETIC summary universe. Judging 157 real status rows and 7
            # real dispositions against it produces a wall of violations that has nothing
            # to do with this driver's subject. Each of the four has its own wiring class,
            # which is where a lost call site is caught.
            # The extraction half, neutralised for the same reason: evaluate_extractions
            # reads the REAL rfc/extraction/*.json, whose sites name requirement ids from
            # the REAL summaries, not from this driver's synthetic set.
            signed_extractions=lambda reqs_: {},
            check_extraction_signoff=lambda *a, **k: [],
            check_extraction_ratchet=lambda *a, **k: [],
            check_drain_floor=lambda *a, **k: [],
            check_summary_disposition=lambda *a, **k: [],
            check_status_completeness=lambda *a, **k: [],
            check_unproven_support=lambda *a, **k: [],
            check_gap_count_agreement=lambda *a, **k: [],
            check_ledger_fresh=lambda *a, **k: [],
        ):
            return _run_capturing(R.run_check)

    def test_run_check_fails_on_lost_coverage(self):
        code, out = self._drive(
            [_tag("RFC7606-2-1", "positive")],
            {"RFC7606-2-1": {"positive", "negative"}},
        )
        self.assertEqual(code, 2, out)
        self.assertIn("no longer proven", out)

    def test_run_check_clean_when_coverage_held(self):
        """Discriminates from 'always fails'."""
        code, out = self._drive(
            [_tag("RFC7606-2-1", "positive"), _tag("RFC7606-2-1", "negative", line=2)],
            {"RFC7606-2-1": {"positive", "negative"}},
        )
        self.assertEqual(code, 0, out)


# --------------------------------------------------------------------------
# Non-unit evidence ratchet (spec-rfcgate-2-evidence AC-13..AC-15)
# --------------------------------------------------------------------------
class TestEvidenceRatchet(unittest.TestCase):
    """Wire-level evidence can only rise, and a tier is never substitutable for another.

    check_coverage_ratchet compares POLARITY sets, so swapping a running `.ci` for a Go
    table test of the same polarity is invisible to it. That swap is a downgrade from
    "the daemon does this" to "the function does this", and this ratchet is what sees it.
    """

    def _errs(self, tags, baseline, enrolled=("rfc7606",)):
        return R.check_evidence_ratchet(
            requirements=[_req("RFC7606-2-1")],
            tags=tags,
            enrolled=set(enrolled),
            baseline_evidence=baseline,
            baseline_enrolled=set(enrolled),
        )

    def test_non_unit_ratchet_fires_on_loss(self):
        """AC-13: the last non-unit tag cannot quietly become a unit tag."""
        errs = self._errs(
            [_tag("RFC7606-2-1", "negative", file="internal/x_test.go")],
            {"RFC7606-2-1": {"functional/verify"}},
        )
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("RFC7606-2-1", errs[0])
        self.assertIn("functional/verify", errs[0])
        self.assertIn("nothing but unit tests", errs[0])

    def test_no_annotation_satisfies_the_ratchet(self):
        """AC-13: {gap} is the move being blocked, not a way through it."""
        ann = R.Annotation(kind="gap", polarity=None, reason="dropped the .ci")
        errs = R.check_evidence_ratchet(
            requirements=[_req("RFC7606-2-1", annotation=ann)],
            tags=[],
            enrolled={"rfc7606"},
            baseline_evidence={"RFC7606-2-1": {"functional/verify"}},
            baseline_enrolled={"rfc7606"},
        )
        self.assertEqual(len(errs), 1, errs)

    def test_verify_tier_ratchet_rejects_nightly_substitution(self):
        """AC-14: a `.ci` binding replaced by an interop one is a LOSS, not a wash.

        The total count of non-unit evidence is unchanged here -- one tag before, one
        after. A single "non-unit evidence" counter would call this clean, which is why
        the ratchet is keyed by `kind/tier` and not by a number (R-1).
        """
        errs = self._errs(
            [_tag("RFC7606-2-1", "negative", file=_INTEROP_FILE)],
            {"RFC7606-2-1": {"functional/verify"}},
        )
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("functional/verify", errs[0])
        self.assertIn("interop/nightly", errs[0], "the message must say what is left")

    def test_nightly_evidence_is_additive_not_a_replacement(self):
        """Keeping the .ci AND adding interop passes: nightly evidence is welcome, it just
        cannot stand in for the verify-tier binding."""
        self.assertEqual(
            self._errs(
                [
                    _tag("RFC7606-2-1", "negative", file=_CI_FILE),
                    _tag("RFC7606-2-1", "positive", file=_INTEROP_FILE),
                ],
                {"RFC7606-2-1": {"functional/verify"}},
            ),
            [],
        )

    def test_losing_nightly_while_keeping_verify_still_fires(self):
        """The counters are INDEPENDENT, so the check runs in both directions."""
        errs = self._errs(
            [_tag("RFC7606-2-1", "negative", file=_CI_FILE)],
            {"RFC7606-2-1": {"functional/verify", "interop/nightly"}},
        )
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("interop/nightly", errs[0])

    def test_non_unit_ratchet_accepts_growth(self):
        """AC-15: baseline+1 passes. Boundary above."""
        self.assertEqual(
            self._errs(
                [_tag("RFC7606-2-1", "negative", file=_CI_FILE)],
                {},
            ),
            [],
        )

    def test_holding_the_baseline_exactly_passes(self):
        """AC-15 boundary: the baseline value itself is valid, only below it fails."""
        self.assertEqual(
            self._errs(
                [_tag("RFC7606-2-1", "negative", file=_CI_FILE)],
                {"RFC7606-2-1": {"functional/verify"}},
            ),
            [],
        )

    def test_moving_a_ci_tag_within_the_carrier_is_invisible(self):
        """Keyed by kind/tier, not by file:line, so renaming or moving a .ci is not a
        loss -- the same reason check_coverage_ratchet compares sets."""
        self.assertEqual(
            self._errs(
                [
                    _tag(
                        "RFC7606-2-1",
                        "negative",
                        file="test/plugin/renamed.ci",
                        line=99,
                    )
                ],
                {"RFC7606-2-1": {"functional/verify"}},
            ),
            [],
        )

    def test_rfc_enrolled_in_this_change_is_not_accused(self):
        """Scoped like its sibling: an RFC not enrolled at HEAD is judged by evaluate()'s
        ordinary rules, not accused of losing evidence it never had."""
        self.assertEqual(
            R.check_evidence_ratchet(
                requirements=[_req("RFC7606-2-1")],
                tags=[],
                enrolled={"rfc7606"},
                baseline_evidence={"RFC7606-2-1": {"functional/verify"}},
                baseline_enrolled=set(),
            ),
            [],
        )

    def test_unit_evidence_is_not_ratcheted_here(self):
        """Unit tags never enter the comparison: check_coverage_ratchet already guards
        proof-of-polarity, and duplicating it here would fire twice for one loss."""
        self.assertEqual(
            R.nonunit_evidence([_tag("RFC7606-2-1", "positive", file="a_test.go")]), {}
        )


class TestEvidenceRatchetWiring(unittest.TestCase):
    """check_evidence_ratchet is dead code unless run_check calls it with the real
    baseline. Drives run_check end-to-end (ai/rules/completion.md)."""

    def _drive(self, tags, baseline_evidence):
        with _patched(
            load_enrolled=lambda: {"rfc7606"},
            summary_stems=lambda: {"rfc7606"},
            parse_summary_file=lambda path: [_req("RFC7606-2-1")],
            _git_baseline_enrolment=lambda: {"rfc7606"},
            _git_baseline_ids=lambda: {"RFC7606-2-1"},
            _git_baseline_tag_polarities=lambda: {},
            _git_baseline_evidence=lambda: baseline_evidence,
            _git_baseline_summary_stems=lambda: {"rfc7606"},
            scan_tree=lambda *a, **k: tags,
            check_status_agreement=lambda *a, **k: [],
            check_audit_freshness=lambda *a, **k: [],
            # The rest of the audit half, neutralised for the same reason the line above
            # is: this driver's subject is not the audit record, and the REAL
            # rfc/audit/rfc7606.json legitimately describes 52 requirements this fixture
            # does not declare (spec-rfcgate-3-audit-teeth.md).
            check_audit_files=lambda *a, **k: [],
            check_audit_schema=lambda *a, **k: [],
            check_audit_disclosure=lambda *a, **k: [],
            check_audit_note=lambda *a, **k: [],
            check_audit_findings=lambda *a, **k: [],
            check_audit_verdict_ratchet=lambda *a, **k: [],
            # The four ledger-edge checks (plan/spec-rfcgate-4-ledger.md), neutralised for
            # the same reason check_status_agreement above is: they read the REAL
            # rfc/not-enrolled.txt and docs/features/rfc-status.md, and this driver
            # declares a SYNTHETIC summary universe. Judging 157 real status rows and 7
            # real dispositions against it produces a wall of violations that has nothing
            # to do with this driver's subject. Each of the four has its own wiring class,
            # which is where a lost call site is caught.
            # The extraction half, neutralised for the same reason: evaluate_extractions
            # reads the REAL rfc/extraction/*.json, whose sites name requirement ids from
            # the REAL summaries, not from this driver's synthetic set.
            signed_extractions=lambda reqs_: {},
            check_extraction_signoff=lambda *a, **k: [],
            check_extraction_ratchet=lambda *a, **k: [],
            check_drain_floor=lambda *a, **k: [],
            check_summary_disposition=lambda *a, **k: [],
            check_status_completeness=lambda *a, **k: [],
            check_unproven_support=lambda *a, **k: [],
            check_gap_count_agreement=lambda *a, **k: [],
            check_ledger_fresh=lambda *a, **k: [],
        ):
            return _run_capturing(R.run_check)

    def test_run_check_fails_when_non_unit_evidence_is_lost(self):
        code, out = self._drive(
            [
                _tag("RFC7606-2-1", "positive", file="internal/x_test.go"),
                _tag("RFC7606-2-1", "negative", file="internal/x_test.go", line=2),
            ],
            {"RFC7606-2-1": {"functional/verify"}},
        )
        self.assertEqual(code, 2, out)
        self.assertIn("lost its functional/verify evidence", out)

    def test_run_check_clean_when_non_unit_evidence_held(self):
        """Discriminates from 'always fails'."""
        code, out = self._drive(
            [
                _tag("RFC7606-2-1", "positive", file="internal/x_test.go"),
                _tag("RFC7606-2-1", "negative", file=_CI_FILE, line=7),
            ],
            {"RFC7606-2-1": {"functional/verify"}},
        )
        self.assertEqual(code, 0, out)

    def test_run_check_publishes_the_tier_split(self):
        """The summary line never flattens the tiers into one number (R-1)."""
        _, out = self._drive(
            [
                _tag("RFC7606-2-1", "positive", file="internal/x_test.go"),
                _tag("RFC7606-2-1", "negative", file=_CI_FILE, line=7),
            ],
            {},
        )
        self.assertIn("unit/verify 1", out)
        self.assertIn("functional/verify 1", out)
        self.assertIn("interop/nightly 0", out)


# The key-words paragraph and a real obligation in ONE paragraph, with the obligation
# opening on a digit -- the shape the sentence splitter cannot cut, because it demands
# `[A-Z"(\[]` after the full stop. Merged, the whole thing matches _BOILERPLATE_RE and the
# obligation leaves with it. "6PE" is the real spelling of RFC 4798's subject, and an RFC
# that opens a sentence with it is ordinary.
_SRC_MERGED_DIGIT = """\
1.  Conventions

   The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT",
   "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this
   document are to be interpreted as described in RFC 2119.
   6PE routers MUST support the label stack of Section 3.
"""

# The same trap sprung by a lowercase opener instead, and through the OTHER arm of
# _BOILERPLATE_RE: "keywords" as one word is invisible to the `key\\s+words` alternative, so
# the match starts at "interpreted" and ends inside the sentence rather than at its head.
_SRC_MERGED_LOWER = """\
1.  Conventions

   The keywords MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, MAY in
   this document are to be interpreted as described in RFC 2119.
   iSCSI targets MUST reject the request.
"""


# --------------------------------------------------------------------------
# New-summary enrolment (spec-rfc-gate-regression-ratchets AC-9..AC-13)
# --------------------------------------------------------------------------
class TestNewSummaryEnrolment(unittest.TestCase):
    """A new rfc/short/*.md is un-enrolled by definition, so adding an RFC used to add
    exactly no checking. Only summaries that are NEW since HEAD are judged: the ones that
    predate it are the existing backlog, and re-litigating it here would block every
    unrelated commit."""

    def _errs(self, stems, baseline, enrolled, reqs, parse_errors=None, src=0):
        with _patched(source_obligation_keyword_count=lambda stem: src):
            return R.check_new_summaries(
                stems=set(stems),
                baseline_stems=set(baseline),
                enrolled=set(enrolled),
                requirements=reqs,
                parse_errors=parse_errors or {},
            )

    def test_new_summary_with_gated_musts_must_enrol(self):
        """AC-9."""
        errs = self._errs(
            ["rfc9999", "rfc0000"],
            ["rfc0000"],
            [],
            [_req("RFC9999-1-1", rfc="rfc9999")],
        )
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("rfc9999", errs[0])
        self.assertIn("enrolled.txt", errs[0])

    def test_new_summary_enrolled_is_clean(self):
        """AC-10: discriminates from 'always fails'."""
        errs = self._errs(
            ["rfc9999", "rfc0000"],
            ["rfc0000"],
            ["rfc9999"],
            [_req("RFC9999-1-1", rfc="rfc9999")],
        )
        self.assertEqual(errs, [])

    def test_new_summary_capturing_nothing_fails_against_source(self):
        """AC-11: zero captured requirements is either a non-normative RFC or a capture
        failure, and only the source text tells them apart (the signal
        unconverted_summaries reports, made blocking for summaries that are new)."""
        errs = self._errs(["rfc9999", "rfc0000"], ["rfc0000"], [], [], src=23)
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("23", errs[0])

    def test_new_non_normative_summary_is_clean(self):
        """The other side of AC-11: no MUSTs in the source either, so nothing was lost."""
        errs = self._errs(["rfc9999", "rfc0000"], ["rfc0000"], [], [], src=0)
        self.assertEqual(errs, [])

    def test_new_summary_unknown_source_is_clean(self):
        """rfc/full/<stem>.txt absent: source_obligation_keyword_count returns None.
        Guessing a violation from a missing file would punish RFCs we simply have not
        downloaded."""
        errs = self._errs(["rfc9999", "rfc0000"], ["rfc0000"], [], [], src=None)
        self.assertEqual(errs, [])

    def test_new_summary_boilerplate_only_passes(self):
        """The real counter, over a real source: a document whose ONLY capitalised
        MUST-level keywords are its own RFC 2119 key-words sentence states no obligation,
        so a summary that gates nothing has hidden nothing.

        Unpatched on purpose. The four keywords of rfc7454 Section 1.1 were read as four
        hidden obligations and refused a BCP whose body contains no MUST at all.
        """
        self.assertEqual(R.source_keyword_count("rfc7454"), 4)
        self.assertEqual(R.source_obligation_keyword_count("rfc7454"), 0)
        errs = R.check_new_summaries(
            stems={"rfc7454", "rfc0000"},
            baseline_stems={"rfc0000"},
            enrolled=set(),
            requirements=[],
            parse_errors={},
        )
        self.assertEqual(errs, [])

    def test_a_real_must_outside_the_boilerplate_still_fails(self):
        """The discriminating twin: strip the key-words paragraph out of a genuinely
        normative source and the obligations are still there. Without this the change
        above is indistinguishable from switching the gate off."""
        self.assertGreater(R.source_obligation_keyword_count("rfc4271"), 100)
        errs = R.check_new_summaries(
            stems={"rfc4271", "rfc0000"},
            baseline_stems={"rfc0000"},
            enrolled=set(),
            requirements=[],
            parse_errors={},
        )
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("rfc4271", errs[0])
        self.assertIn("key-words paragraph", errs[0])

    def _count(self, src):
        with _patched(source_text=lambda stem: src):
            return R.source_obligation_keyword_count("rfc9999")

    def test_an_obligation_merged_into_the_key_words_paragraph_survives(self):
        """The exclusion drops a SENTENCE, and the splitter decides what one is. Where it
        cannot cut, the key-words paragraph and the obligation after it are one sentence,
        and excluding the first takes the second with it.

        This is the one failure direction a gate must not have: the counter feeds
        check_new_summaries, so a swallowed MUST reads as an RFC that asks for nothing and
        the summary capturing nothing passes. Zero occurrences in today's corpus, which is
        a fact about the corpus rather than about the rule (ai/rules/rfc-compliance.md).
        Both openers the splitter refuses are covered: a digit and a lowercase letter."""
        self.assertEqual(self._count(_SRC_MERGED_DIGIT), 1)
        self.assertEqual(self._count(_SRC_MERGED_LOWER), 1)

    def test_the_key_words_paragraph_alone_still_counts_nothing(self):
        """The discriminating control: the fix above must save the obligation without
        readmitting the four keywords of the paragraph that carried it. Same fixtures, cut
        at the paragraph's own full stop."""
        for src in (_SRC_MERGED_DIGIT, _SRC_MERGED_LOWER):
            head = "\n".join(src.rstrip("\n").split("\n")[:-1]) + "\n"
            self.assertIn("interpreted as described", head)
            self.assertNotIn("MUST support", head)
            self.assertNotIn("MUST reject", head)
            self.assertEqual(self._count(head), 0)

    def test_new_summary_parse_error_is_reported(self):
        """AC-12: parse errors are suppressed for un-enrolled summaries (they predate the
        id migration). A summary that is NEW has no such excuse -- and suppressing it would
        make AC-9 trivially evadable by shipping a summary that does not parse."""
        errs = self._errs(
            ["rfc9999", "rfc0000"],
            ["rfc0000"],
            [],
            [],
            parse_errors={"rfc9999": "bad line 3"},
        )
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("bad line 3", errs[0])

    def test_preexisting_unenrolled_summary_grandfathered(self):
        """AC-13: the 9 summaries already un-enrolled at HEAD must not turn the gate red."""
        errs = self._errs(
            ["rfc9999"],
            ["rfc9999"],
            [],
            [_req("RFC9999-1-1", rfc="rfc9999")],
            parse_errors={"rfc9999": "bad"},
            src=23,
        )
        self.assertEqual(errs, [])


class TestRetiredRequirements(unittest.TestCase):
    """Without this, deleting the checklist line is the CHEAPEST route from red to green:
    cheaper than {gap}, which costs a public disclosure row. The coverage ratchet iterates
    the CURRENT requirements, so a deleted one is never visited and its lost tests are
    never noticed -- the ratchet would pressure people toward hiding an obligation rather
    than declaring it."""

    def _errs(
        self,
        reqs,
        baseline_ids,
        enrolled=("rfc7606",),
        base_enrolled=None,
        stems=("rfc7606",),
    ):
        return R.check_retired_requirements(
            requirements=reqs,
            enrolled=set(enrolled),
            baseline_ids=set(baseline_ids),
            baseline_enrolled=set(enrolled if base_enrolled is None else base_enrolled),
            stems=set(stems),
            baseline_stems=set(stems),
        )

    def test_deleted_requirement_line_fails(self):
        errs = self._errs([], {"RFC7606-2-1"})
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("RFC7606-2-1", errs[0])

    def test_present_requirement_is_clean(self):
        """Discriminates from 'always fails'."""
        self.assertEqual(self._errs([_req("RFC7606-2-1")], {"RFC7606-2-1"}), [])

    def test_text_edit_under_the_same_id_is_clean(self):
        """Correcting a misquoted requirement is the legitimate move, and it keeps the id.
        Blocking that would push people back toward deleting the line."""
        r = _req("RFC7606-2-1")._replace(text="corrected wording")
        self.assertEqual(self._errs([r], {"RFC7606-2-1"}), [])

    def test_unenrolled_rfc_is_not_judged(self):
        self.assertEqual(self._errs([], {"RFC7606-2-1"}, enrolled=()), [])

    def test_unparseable_summary_does_not_look_retired(self):
        """A summary that failed to parse contributed NO requirements, so every id it holds
        looks deleted. One unparseable rfc7606.md produced 39 confident wrong messages,
        burying the single parse error that is the real problem."""
        errs = R.check_retired_requirements(
            requirements=[],
            enrolled={"rfc7606"},
            baseline_ids={f"RFC7606-2-{i}" for i in range(1, 40)},
            baseline_enrolled={"rfc7606"},
            stems={"rfc7606"},
            baseline_stems={"rfc7606"},
            parse_errors={"rfc7606": "bad line 12"},
        )
        self.assertEqual(errs, [])

    def test_deleted_summary_file_does_not_look_retired(self):
        """The twin of the parse-error case. A DELETED summary is never parsed at all, so
        `reqs` holds none of its ids and every one looks retired: 39 confident wrong
        messages on top of check_enrolment's single accurate 'enrolled but rfc/short/
        rfc7606.md does not exist'. Same burying effect, adjacent input."""
        errs = R.check_retired_requirements(
            requirements=[],
            enrolled={"rfc7606"},
            baseline_ids={f"RFC7606-2-{i}" for i in range(1, 40)},
            baseline_enrolled={"rfc7606"},
            stems=set(),  # the file is gone from the working tree
            baseline_stems={"rfc7606"},
        )
        self.assertEqual(errs, [])

    def test_id_is_not_misattributed_to_a_prefix_sharing_stem(self):
        """An id whose real owner is UN-enrolled must not be blamed on whichever enrolled
        stem shares its prefix. Matching only against judged stems reported
        DRAFT-FOO-BAR-3.1-1 against rfc/short/draft-foo.md, a summary that never held it."""
        errs = R.check_retired_requirements(
            requirements=[],
            enrolled={"draft-foo"},
            baseline_ids={"DRAFT-FOO-BAR-3.1-1"},
            baseline_enrolled={"draft-foo"},
            stems={"draft-foo", "draft-foo-bar"},
            baseline_stems={"draft-foo", "draft-foo-bar"},
        )
        self.assertEqual(errs, [])

    def test_draft_stem_is_matched_by_longest_prefix(self):
        """A draft stem is itself hyphenated (draft-foo-bar -> DRAFT-FOO-BAR-3.1-1), so
        splitting the id on '-' would name the wrong RFC and skip the check."""
        errs = R.check_retired_requirements(
            requirements=[],
            enrolled={"draft-foo-bar"},
            baseline_ids={"DRAFT-FOO-BAR-3.1-1"},
            baseline_enrolled={"draft-foo-bar"},
            stems={"draft-foo-bar"},
            baseline_stems={"draft-foo-bar"},
        )
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("draft-foo-bar", errs[0])


_RFC_TEXT = (
    "2.8.1.  Simultaneous Child SA Rekeying\n\n"
    "   If redundant SAs are created, the SA created with the lowest of the\n"
    "   four nonces SHOULD be closed by the endpoint that created it.\n"
)


def _correction(body):
    """One correction paragraph as a summary would carry it, parsed by the real reader."""
    return R.parse_corrections(body)


class TestParseCorrections(unittest.TestCase):
    """The record the corpus already writes. rfc/short/rfc7947.md blockquotes it and
    rfc/short/rfc7296.md writes it as a plain paragraph, so the reader accepts both or it
    reads only half the corrections in the tree."""

    def test_blockquoted_paragraph_is_read(self):
        got = _correction(
            "> Correction 2026-08-15: `RFC7947-x-3` was extracted at MUST strength.\n"
            '> RFC 7947 says it "SHOULD be propagated to other clients".\n'
        )
        self.assertEqual(len(got), 1, got)
        self.assertEqual(got[0].date, "2026-08-15")
        self.assertEqual(got[0].rids, ("RFC7947-x-3",))
        self.assertEqual(got[0].quotes, ("SHOULD be propagated to other clients",))

    def test_plain_paragraph_is_read(self):
        got = _correction(
            "Correction 2026-08-15: `RFC7296-2.8-1` was extracted at MUST strength. The\n"
            'SA "SHOULD be closed by the endpoint that created it".\n'
        )
        self.assertEqual(len(got), 1, got)
        self.assertEqual(got[0].rids, ("RFC7296-2.8-1",))

    def test_quote_spanning_a_line_break_is_one_quote(self):
        """RFC prose wraps, and so does a summary. A quotation cut in two by the wrap
        would never match the RFC text it came from."""
        got = _correction(
            'Correction 2026-08-15: `RFC7296-2.8-1`. The SA "SHOULD be closed by the\n'
            'endpoint that created it".\n'
        )
        self.assertEqual(
            got[0].quotes, ("SHOULD be closed by the endpoint that created it",)
        )

    def test_ordinary_prose_is_not_a_correction(self):
        """Discriminates from 'every paragraph counts'. A summary is mostly prose, and a
        paragraph that merely mentions a correction authorises nothing."""
        self.assertEqual(
            _correction("This row was corrected on 2026-08-15: see the journal.\n"), []
        )

    def test_a_correction_without_a_quotation_still_parses(self):
        """Tolerant on purpose: rfc/short/rfc7947.md's 2026-08-14 note records a POLARITY
        repair and quotes nothing. Raising here would red the gate over a legitimate note.
        It authorises no level change -- that verdict belongs to the ratchet, which says so
        at the row."""
        got = _correction(
            "Correction 2026-08-14: `RFC7947-x-3` read single-polarity.\n"
        )
        self.assertEqual(len(got), 1, got)
        self.assertEqual(got[0].quotes, ())

    def test_bare_id_in_prose_does_not_name_the_row(self):
        """The id must be backticked, as every correction in the corpus writes it. A
        paragraph that mentions a neighbouring row in passing must not authorise it."""
        got = _correction(
            'Correction 2026-08-15: `RFC7296-2.8-1`, unlike RFC7296-2.8-2, "SHOULD be '
            'closed by the endpoint that created it".\n'
        )
        self.assertEqual(got[0].rids, ("RFC7296-2.8-1",))


class TestLevelRatchet(unittest.TestCase):
    """Gating is monotonic. A MUST demoted to a SHOULD keeps its id (so
    check_retired_requirements is silent), keeps its tests (so check_coverage_ratchet and
    check_evidence_ratchet are silent), and stops being asked for either polarity. It was
    the cheapest route from red to green in the whole gate: cheaper than {gap}, which costs
    a public disclosure row, and cheaper than deleting the row, which costs the id."""

    def _errs(
        self,
        reqs,
        baseline_levels,
        corrections=(),
        rfc_text=_RFC_TEXT,
        enrolled=("rfc7296",),
        base_enrolled=None,
    ):
        with _patched(
            summary_corrections=lambda stem: list(corrections),
            source_text=lambda stem: rfc_text,
        ):
            return R.check_level_ratchet(
                requirements=reqs,
                enrolled=set(enrolled),
                baseline_levels=dict(baseline_levels),
                baseline_enrolled=set(
                    enrolled if base_enrolled is None else base_enrolled
                ),
            )

    def _row(self, level="SHOULD"):
        return _req("RFC7296-2.8-1", level=level, rfc="rfc7296")._replace(
            section="2.8.1", source="rfc/short/rfc7296.md", line=520
        )

    def _authorising(self):
        return _correction(
            "Correction 2026-08-15: `RFC7296-2.8-1` was extracted at MUST strength. "
            '§2.8.1 states it as a recommendation: the SA "SHOULD be closed by the '
            'endpoint that created it".\n'
        )

    def test_unrecorded_demotion_fails(self):
        """AC-1. The message must carry the id, BOTH levels and the section, because the
        reader's next action is to open that section and decide which one is right."""
        errs = self._errs([self._row()], {"RFC7296-2.8-1": "MUST"})
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("RFC7296-2.8-1", errs[0])
        self.assertIn("[MUST]", errs[0])
        self.assertIn("[SHOULD]", errs[0])
        self.assertIn("2.8.1", errs[0])

    def test_recorded_correction_passes(self):
        """AC-2. The escape, and the shape the corpus already writes."""
        errs = self._errs(
            [self._row()], {"RFC7296-2.8-1": "MUST"}, corrections=self._authorising()
        )
        self.assertEqual(errs, [])

    def test_promotion_needs_no_record(self):
        """AC-3. One-directional by construction: a row GAINING a gated level is a
        conformance improvement, and gating it would make the gate argue against its own
        purpose."""
        errs = self._errs([self._row(level="MUST")], {"RFC7296-2.8-1": "SHOULD"})
        self.assertEqual(errs, [])

    def test_unchanged_gated_row_is_clean(self):
        """AC-5 in miniature: the corpus is overwhelmingly rows that did not move, and a
        ratchet that fires on them would be routed around within a day."""
        errs = self._errs([self._row(level="MUST")], {"RFC7296-2.8-1": "MUST"})
        self.assertEqual(errs, [])

    def test_advisory_to_advisory_is_not_this_ratchet(self):
        """A SHOULD lowered to a MAY loses no gating, because a SHOULD never gated. Stated
        as a test so the boundary is a decision rather than an oversight: the SHOULD tier
        is the backlog's second phase (the spec's Known Limitations)."""
        errs = self._errs([self._row(level="MAY")], {"RFC7296-2.8-1": "SHOULD"})
        self.assertEqual(errs, [])

    def test_correction_for_another_id_does_not_authorise(self):
        """A paragraph naming a sibling row is not this row's authorisation. Without the id
        match, one correction anywhere in a summary would license every demotion in it."""
        other = _correction(
            'Correction 2026-08-15: `RFC7296-2.8-2` "SHOULD be closed by the endpoint '
            'that created it".\n'
        )
        errs = self._errs([self._row()], {"RFC7296-2.8-1": "MUST"}, corrections=other)
        self.assertEqual(len(errs), 1, errs)

    def test_quotation_absent_from_the_rfc_does_not_authorise(self):
        """The condition that makes the record evidence rather than assertion. A reason
        nobody can check is what `GATED_FLOOR` already was: a note the demoting commit
        writes about itself."""
        invented = _correction(
            'Correction 2026-08-15: `RFC7296-2.8-1` because the RFC says "this obligation '
            'is only a recommendation for implementers".\n'
        )
        errs = self._errs(
            [self._row()], {"RFC7296-2.8-1": "MUST"}, corrections=invented
        )
        self.assertEqual(len(errs), 1, errs)

    def test_keyword_sized_quotation_does_not_authorise(self):
        """ "SHOULD" appears in every RFC, so quoting it proves nothing about THIS row. The
        quotation has to carry the obligation, which is what MIN_CORRECTION_QUOTE buys."""
        thin = _correction('Correction 2026-08-15: `RFC7296-2.8-1` says "SHOULD".\n')
        errs = self._errs([self._row()], {"RFC7296-2.8-1": "MUST"}, corrections=thin)
        self.assertEqual(len(errs), 1, errs)

    def test_missing_rfc_text_fails_closed(self):
        """No source text, no way to check a quotation. Failing OPEN here would make
        deleting rfc/full/<stem>.txt the new cheapest route from red to green."""
        errs = self._errs(
            [self._row()],
            {"RFC7296-2.8-1": "MUST"},
            corrections=self._authorising(),
            rfc_text=None,
        )
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("rfc/full/rfc7296.txt", errs[0])

    def test_retired_row_is_left_to_its_own_ratchet(self):
        """AC-4. A vanished id is check_retired_requirements' subject. Reporting it here
        too would double-count one loss and split its fix across two messages."""
        errs = self._errs([], {"RFC7296-2.8-1": "MUST"})
        self.assertEqual(errs, [])
        retired = R.check_retired_requirements(
            requirements=[],
            enrolled={"rfc7296"},
            baseline_ids={"RFC7296-2.8-1"},
            baseline_enrolled={"rfc7296"},
            stems={"rfc7296"},
            baseline_stems={"rfc7296"},
        )
        self.assertEqual(len(retired), 1, retired)

    def test_duplicate_lines_report_one_demotion(self):
        errs = self._errs([self._row(), self._row()], {"RFC7296-2.8-1": "MUST"})
        self.assertEqual(len(errs), 1, errs)

    def test_row_with_no_baseline_is_not_judged(self):
        """A requirement added in this very change has no level to have fallen from."""
        self.assertEqual(self._errs([self._row()], {}), [])

    def test_newly_enrolled_rfc_is_not_judged(self):
        """Scoped like its siblings. An RFC enrolled in this change is judged by
        evaluate()'s ordinary rules, not accused of a regression it predates."""
        errs = self._errs([self._row()], {"RFC7296-2.8-1": "MUST"}, base_enrolled=())
        self.assertEqual(errs, [])

    def test_unenrolled_rfc_is_not_judged(self):
        errs = self._errs([self._row()], {"RFC7296-2.8-1": "MUST"}, enrolled=())
        self.assertEqual(errs, [])


class TestLevelRatchetWiring(unittest.TestCase):
    """Drive run_check. The check is dead code unless the gate calls it with the real HEAD
    levels, and every sibling ratchet has a wiring class for exactly that reason: correct in
    isolation, deletable from run_check with every other test still green."""

    def _drive(self, level, corrections=()):
        row = _req("RFC7606-2-1", level=level)._replace(section="2")
        tags = [
            _tag("RFC7606-2-1", "positive"),
            _tag("RFC7606-2-1", "negative", line=2),
        ]
        with _patched(
            load_enrolled=lambda: {"rfc7606"},
            summary_stems=lambda: {"rfc7606"},
            parse_summary_file=lambda path: [row],
            summary_corrections=lambda stem: list(corrections),
            source_text=lambda stem: _RFC_TEXT,
            _git_baseline_enrolment=lambda: {"rfc7606"},
            _git_baseline_ids=lambda: {"RFC7606-2-1"},
            _git_baseline_levels=lambda: {"RFC7606-2-1": "MUST"},
            _git_baseline_tag_polarities=lambda: {},
            _git_baseline_evidence=lambda: {},
            _git_baseline_summary_stems=lambda: {"rfc7606"},
            scan_tree=lambda *a, **k: tags,
            check_status_agreement=lambda *a, **k: [],
            # The audit half and the ledger edges, neutralised for the reason every driver
            # in this file neutralises them: they read the REAL rfc/audit/*.json,
            # rfc/not-enrolled.txt and docs/features/rfc-status.md, and this driver declares
            # a synthetic summary universe. Each has its own wiring class.
            check_audit_files=lambda *a, **k: [],
            check_audit_schema=lambda *a, **k: [],
            check_audit_freshness=lambda *a, **k: [],
            check_audit_disclosure=lambda *a, **k: [],
            check_audit_note=lambda *a, **k: [],
            check_audit_findings=lambda *a, **k: [],
            check_audit_verdict_ratchet=lambda *a, **k: [],
            signed_extractions=lambda reqs_: {},
            check_extraction_signoff=lambda *a, **k: [],
            check_extraction_ratchet=lambda *a, **k: [],
            check_drain_floor=lambda *a, **k: [],
            check_summary_disposition=lambda *a, **k: [],
            check_status_completeness=lambda *a, **k: [],
            check_unproven_support=lambda *a, **k: [],
            check_gap_count_agreement=lambda *a, **k: [],
            check_ledger_fresh=lambda *a, **k: [],
        ):
            return _run_capturing(R.run_check)

    def test_run_check_fails_on_an_unrecorded_demotion(self):
        """AC-1 at the gate. The row keeps its id and both its tags, so every other check
        on the path is content: only this one can see the MUST leave."""
        code, out = self._drive("SHOULD")
        self.assertEqual(code, 2, out)
        self.assertIn("RFC7606-2-1", out)
        self.assertIn("Gating is monotonic", out)

    def test_run_check_passes_with_the_correction_recorded(self):
        """AC-2 at the gate, and the discrimination for the case above: the same demoted
        tree with the authorising paragraph present reports clean."""
        code, out = self._drive(
            "SHOULD",
            corrections=_correction(
                'Correction 2026-08-15: `RFC7606-2-1` -- the RFC says it "SHOULD be '
                'closed by the endpoint that created it".\n'
            ),
        )
        self.assertEqual(code, 0, out)

    def test_run_check_passes_when_the_level_holds(self):
        code, out = self._drive("MUST")
        self.assertEqual(code, 0, out)


class TestCorrectionsInTheRealCorpus(unittest.TestCase):
    """The three level corrections the tree already carries, judged by the rule that now
    reads them. Written against the REAL summaries and the REAL RFC texts: a synthetic
    fixture would pass with the convention spelled any way at all, and the point of reusing
    the corpus's own paragraph is that it is the one people write."""

    CASES = (
        ("rfc7296", "RFC7296-2.8-1"),
        ("rfc7947", "RFC7947-x-1"),
        ("rfc7947", "RFC7947-x-3"),
    )

    def test_every_recorded_demotion_is_authorised(self):
        for stem, rid in self.CASES:
            with self.subTest(rid=rid):
                got = R.authorising_correction(
                    rid, R.summary_corrections(stem), R.source_text(stem)
                )
                self.assertIsNotNone(
                    got,
                    f"{rid}'s correction in rfc/short/{stem}.md no longer authorises it",
                )

    def test_an_uncorrected_row_is_not_authorised(self):
        """Discriminates from 'authorises everything'. RFC7947-x-2 is a MUST NOT that was
        never corrected, so no paragraph in rfc/short/rfc7947.md may cover it."""
        self.assertIsNone(
            R.authorising_correction(
                "RFC7947-x-2",
                R.summary_corrections("rfc7947"),
                R.source_text("rfc7947"),
            )
        )


class TestRetiredRequirementsWiring(unittest.TestCase):
    """Drive run_check. Without this the check is exactly what its own siblings had a
    wiring class to prevent: correct in isolation, and deletable from run_check with all
    other tests still green. That is the same defect class as the fail-loud blocker this
    whole check was written to close."""

    def _drive(self, reqs):
        # Tags follow the requirements EXACTLY. Tagging an id that no longer exists makes
        # evaluate() emit "unknown RFC requirement: <id>", which mentions the same id and
        # returns the same exit code as the check under test -- the first version of this
        # test passed on that message while check_retired_requirements was not called at
        # all. Same id, same exit code, different producer: the assertion below names the
        # message text for that reason.
        tags = []
        for i, r in enumerate(reqs):
            tags.append(_tag(r.rid, "positive", line=2 * i + 1))
            tags.append(_tag(r.rid, "negative", line=2 * i + 2))
        with _patched(
            load_enrolled=lambda: {"rfc7606"},
            summary_stems=lambda: {"rfc7606"},
            parse_summary_file=lambda path: reqs,
            _git_baseline_enrolment=lambda: {"rfc7606"},
            _git_baseline_ids=lambda: {"RFC7606-2-1", "RFC7606-2-2"},
            _git_baseline_tag_polarities=lambda: {},
            _git_baseline_evidence=lambda: {},
            _git_baseline_summary_stems=lambda: {"rfc7606"},
            scan_tree=lambda *a, **k: tags,
            check_status_agreement=lambda *a, **k: [],
            check_audit_freshness=lambda *a, **k: [],
            # The rest of the audit half, neutralised for the same reason the line above
            # is: this driver's subject is not the audit record, and the REAL
            # rfc/audit/rfc7606.json legitimately describes 52 requirements this fixture
            # does not declare (spec-rfcgate-3-audit-teeth.md).
            check_audit_files=lambda *a, **k: [],
            check_audit_schema=lambda *a, **k: [],
            check_audit_disclosure=lambda *a, **k: [],
            check_audit_note=lambda *a, **k: [],
            check_audit_findings=lambda *a, **k: [],
            check_audit_verdict_ratchet=lambda *a, **k: [],
            # The four ledger-edge checks (plan/spec-rfcgate-4-ledger.md), neutralised for
            # the same reason check_status_agreement above is: they read the REAL
            # rfc/not-enrolled.txt and docs/features/rfc-status.md, and this driver
            # declares a SYNTHETIC summary universe. Judging 157 real status rows and 7
            # real dispositions against it produces a wall of violations that has nothing
            # to do with this driver's subject. Each of the four has its own wiring class,
            # which is where a lost call site is caught.
            # The extraction half, neutralised for the same reason: evaluate_extractions
            # reads the REAL rfc/extraction/*.json, whose sites name requirement ids from
            # the REAL summaries, not from this driver's synthetic set.
            signed_extractions=lambda reqs_: {},
            check_extraction_signoff=lambda *a, **k: [],
            check_extraction_ratchet=lambda *a, **k: [],
            check_drain_floor=lambda *a, **k: [],
            check_summary_disposition=lambda *a, **k: [],
            check_status_completeness=lambda *a, **k: [],
            check_unproven_support=lambda *a, **k: [],
            check_gap_count_agreement=lambda *a, **k: [],
            check_ledger_fresh=lambda *a, **k: [],
            check_id_allocation=lambda *a, **k: [],
        ):
            return _run_capturing(R.run_check)

    def test_run_check_fails_when_a_requirement_line_is_deleted(self):
        code, out = self._drive([_req("RFC7606-2-1")])  # 2-2 was at HEAD, now gone
        self.assertEqual(code, 2, out)
        self.assertIn("RFC7606-2-2", out)
        self.assertIn("Requirement ids are permanent", out)

    def test_run_check_clean_when_both_survive(self):
        """Discriminates from 'always fails'."""
        code, out = self._drive([_req("RFC7606-2-1"), _req("RFC7606-2-2")])
        self.assertEqual(code, 0, out)


class TestDegradedBaselineIsQuiet(unittest.TestCase):
    """The composed gate, not the reader in isolation.

    A reader returning 'no baseline' is only half the guarantee; the other half is that
    run_check stays quiet on it. `stems - baseline_stems` against an empty baseline is
    `stems` -- every summary in the repo accused of being new -- so a reader test asserting
    only `is None` would have passed while the gate reported a wall of false violations."""

    def _drive(self, stems_baseline):
        with _patched(
            load_enrolled=lambda: {"rfc7606"},
            summary_stems=lambda: {"rfc7606", "rfc9999"},
            parse_summary_file=lambda path: (
                [] if path.endswith("rfc9999.md") else [_req("RFC7606-2-1")]
            ),
            _git_baseline_enrolment=lambda: {"rfc7606"},
            _git_baseline_ids=lambda: {"RFC7606-2-1"},
            _git_baseline_tag_polarities=lambda: {},
            _git_baseline_evidence=lambda: {},
            _git_baseline_summary_stems=lambda: stems_baseline,
            source_obligation_keyword_count=lambda stem: 23,
            scan_tree=lambda *a, **k: [
                _tag("RFC7606-2-1", "positive"),
                _tag("RFC7606-2-1", "negative", line=2),
            ],
            check_status_agreement=lambda *a, **k: [],
            check_audit_freshness=lambda *a, **k: [],
            # The rest of the audit half, neutralised for the same reason the line above
            # is: this driver's subject is not the audit record, and the REAL
            # rfc/audit/rfc7606.json legitimately describes 52 requirements this fixture
            # does not declare (spec-rfcgate-3-audit-teeth.md).
            check_audit_files=lambda *a, **k: [],
            check_audit_schema=lambda *a, **k: [],
            check_audit_disclosure=lambda *a, **k: [],
            check_audit_note=lambda *a, **k: [],
            check_audit_findings=lambda *a, **k: [],
            check_audit_verdict_ratchet=lambda *a, **k: [],
            # The four ledger-edge checks (plan/spec-rfcgate-4-ledger.md), neutralised for
            # the same reason check_status_agreement above is: they read the REAL
            # rfc/not-enrolled.txt and docs/features/rfc-status.md, and this driver
            # declares a SYNTHETIC summary universe. Judging 157 real status rows and 7
            # real dispositions against it produces a wall of violations that has nothing
            # to do with this driver's subject. Each of the four has its own wiring class,
            # which is where a lost call site is caught.
            # The extraction half, neutralised for the same reason: evaluate_extractions
            # reads the REAL rfc/extraction/*.json, whose sites name requirement ids from
            # the REAL summaries, not from this driver's synthetic set.
            signed_extractions=lambda reqs_: {},
            check_extraction_signoff=lambda *a, **k: [],
            check_extraction_ratchet=lambda *a, **k: [],
            check_drain_floor=lambda *a, **k: [],
            check_summary_disposition=lambda *a, **k: [],
            check_status_completeness=lambda *a, **k: [],
            check_unproven_support=lambda *a, **k: [],
            check_gap_count_agreement=lambda *a, **k: [],
            check_ledger_fresh=lambda *a, **k: [],
        ):
            return _run_capturing(R.run_check)

    def test_git_failure_reports_nothing(self):
        """AC-14 at the gate."""
        code, out = self._drive(None)
        self.assertEqual(code, 0, out)

    def test_empty_baseline_reports_nothing(self):
        """First commit / no rfc/short at HEAD. Judging all of them 'new' would name files
        committed years ago."""
        code, out = self._drive(set())
        self.assertEqual(code, 0, out)

    def test_real_baseline_still_reports(self):
        """Discriminates from 'always quiet': with a genuine baseline the same tree fails."""
        code, out = self._drive({"rfc7606"})
        self.assertEqual(code, 2, out)
        self.assertIn("rfc9999", out)


class TestNewSummaryEnrolmentWiring(unittest.TestCase):
    """Drive run_check: the check is worthless if run_check never calls it, or calls it
    with a baseline that already contains every summary."""

    def _drive(self, baseline_stems):
        with _patched(
            load_enrolled=lambda: {"rfc7606"},
            summary_stems=lambda: {"rfc7606", "rfc9999"},
            parse_summary_file=lambda path: (
                [_req("RFC9999-1-1", rfc="rfc9999")]
                if path.endswith("rfc9999.md")
                else [_req("RFC7606-2-1")]
            ),
            _git_baseline_enrolment=lambda: {"rfc7606"},
            _git_baseline_ids=lambda: {"RFC7606-2-1", "RFC9999-1-1"},
            _git_baseline_tag_polarities=lambda: {},
            _git_baseline_evidence=lambda: {},
            _git_baseline_summary_stems=lambda: baseline_stems,
            scan_tree=lambda *a, **k: [
                _tag("RFC7606-2-1", "positive"),
                _tag("RFC7606-2-1", "negative", line=2),
            ],
            check_status_agreement=lambda *a, **k: [],
            check_audit_freshness=lambda *a, **k: [],
            # The rest of the audit half, neutralised for the same reason the line above
            # is: this driver's subject is not the audit record, and the REAL
            # rfc/audit/rfc7606.json legitimately describes 52 requirements this fixture
            # does not declare (spec-rfcgate-3-audit-teeth.md).
            check_audit_files=lambda *a, **k: [],
            check_audit_schema=lambda *a, **k: [],
            check_audit_disclosure=lambda *a, **k: [],
            check_audit_note=lambda *a, **k: [],
            check_audit_findings=lambda *a, **k: [],
            check_audit_verdict_ratchet=lambda *a, **k: [],
            # The four ledger-edge checks (plan/spec-rfcgate-4-ledger.md), neutralised for
            # the same reason check_status_agreement above is: they read the REAL
            # rfc/not-enrolled.txt and docs/features/rfc-status.md, and this driver
            # declares a SYNTHETIC summary universe. Judging 157 real status rows and 7
            # real dispositions against it produces a wall of violations that has nothing
            # to do with this driver's subject. Each of the four has its own wiring class,
            # which is where a lost call site is caught.
            # The extraction half, neutralised for the same reason: evaluate_extractions
            # reads the REAL rfc/extraction/*.json, whose sites name requirement ids from
            # the REAL summaries, not from this driver's synthetic set.
            signed_extractions=lambda reqs_: {},
            check_extraction_signoff=lambda *a, **k: [],
            check_extraction_ratchet=lambda *a, **k: [],
            check_drain_floor=lambda *a, **k: [],
            check_summary_disposition=lambda *a, **k: [],
            check_status_completeness=lambda *a, **k: [],
            check_unproven_support=lambda *a, **k: [],
            check_gap_count_agreement=lambda *a, **k: [],
            check_ledger_fresh=lambda *a, **k: [],
        ):
            return _run_capturing(R.run_check)

    def test_run_check_fails_on_new_unenrolled_summary(self):
        code, out = self._drive({"rfc7606"})
        self.assertEqual(code, 2, out)
        self.assertIn("rfc9999", out)

    def test_run_check_clean_when_summary_predates_head(self):
        """Discriminates from 'always fails': the same tree, with rfc9999 already at HEAD."""
        code, out = self._drive({"rfc7606", "rfc9999"})
        self.assertEqual(code, 0, out)


class TestSummaryParseErrorWiring(unittest.TestCase):
    """`errs.extend(parse_errs)` in run_check is the ONLY thing that turns an unparseable
    ENROLLED summary into a violation, and nothing drove it: the parser tests call
    `parse_checklist_line` directly, and the gate-level fixtures hand `_collect_for_check`
    a hardcoded empty `parse_errs`. Delete the line and all 295 tests stayed green while
    an enrolled summary that does not parse exited 0 -- with `evaluate` seeing zero
    requirements, so the RFC's every MUST silently stopped being checked.

    Driven through the REAL parser over a REAL file (a temporary SUMMARY_DIR), so this
    also proves the two halves agree: that `parse_summary_file` raises on the legacy
    no-id line, and that run_check surfaces what it raised."""

    _BAD = (
        "# RFC 7606\n\n## Compliance Checklist\n\n"
        "- [ ] [MUST] legacy line with no ID (§2)\n"
    )
    _GOOD = (
        "# RFC 7606\n\n## Compliance Checklist\n\n"
        "- [ ] [RFC7606-2-1] [MUST] Treat the UPDATE as withdrawn (§2)\n"
    )

    def _drive(self, body, tags=()):
        tmp = _mkdtemp("ze-parseerr-")
        try:
            with open(os.path.join(tmp, "rfc7606.md"), "w", encoding="utf-8") as fh:
                fh.write(body)
            with _patched(
                SUMMARY_DIR=tmp,
                load_enrolled=lambda: {"rfc7606"},
                _git_baseline_enrolment=lambda: {"rfc7606"},
                _git_baseline_ids=lambda: set(),
                _git_baseline_summary_stems=lambda: {"rfc7606"},
                _git_baseline_tag_polarities=lambda: {},
                _git_baseline_evidence=lambda: {},
                scan_tree=lambda *a, **k: list(tags),
                check_status_agreement=lambda *a, **k: [],
                check_audit_freshness=lambda *a, **k: [],
                # The rest of the audit half, neutralised for the same reason the line above
                # is: this driver's subject is not the audit record, and the REAL
                # rfc/audit/rfc7606.json legitimately describes 52 requirements this fixture
                # does not declare (spec-rfcgate-3-audit-teeth.md).
                check_audit_files=lambda *a, **k: [],
                check_audit_schema=lambda *a, **k: [],
                check_audit_disclosure=lambda *a, **k: [],
                check_audit_note=lambda *a, **k: [],
                check_audit_findings=lambda *a, **k: [],
                check_audit_verdict_ratchet=lambda *a, **k: [],
                # The four ledger-edge checks (plan/spec-rfcgate-4-ledger.md), neutralised for
                # the same reason check_status_agreement above is: they read the REAL
                # rfc/not-enrolled.txt and docs/features/rfc-status.md, and this driver
                # declares a SYNTHETIC summary universe. Judging 157 real status rows and 7
                # real dispositions against it produces a wall of violations that has nothing
                # to do with this driver's subject. Each of the four has its own wiring class,
                # which is where a lost call site is caught.
                # The extraction half, neutralised for the same reason: evaluate_extractions
                # reads the REAL rfc/extraction/*.json, whose sites name requirement ids from
                # the REAL summaries, not from this driver's synthetic set.
                signed_extractions=lambda reqs_: {},
                check_extraction_signoff=lambda *a, **k: [],
                check_extraction_ratchet=lambda *a, **k: [],
                check_drain_floor=lambda *a, **k: [],
                check_summary_disposition=lambda *a, **k: [],
                check_status_completeness=lambda *a, **k: [],
                check_unproven_support=lambda *a, **k: [],
                check_gap_count_agreement=lambda *a, **k: [],
                check_ledger_fresh=lambda *a, **k: [],
            ):
                return _run_capturing(R.run_check)
        finally:
            shutil.rmtree(tmp, ignore_errors=True)

    def test_unparseable_enrolled_summary_fails_the_gate(self):
        code, out = self._drive(self._BAD)
        self.assertEqual(code, 2, out)
        self.assertIn("checklist line has no requirement id", out)
        self.assertIn("rfc7606.md", out)

    def test_a_parsing_enrolled_summary_still_passes(self):
        """The discriminating twin. Without it the test above would pass under a run_check
        that failed on every input, which proves nothing about the parse-error wiring."""
        code, out = self._drive(
            self._GOOD,
            tags=[
                _tag("RFC7606-2-1", "positive"),
                _tag("RFC7606-2-1", "negative", line=2),
            ],
        )
        self.assertEqual(code, 0, out)


# --------------------------------------------------------------------------
# Ledger staleness (AC-20) — run_check_fresh / --check-fresh
# --------------------------------------------------------------------------
class TestLedgerStaleness(unittest.TestCase):
    """AC-20 at the CLI level: a committed ai/RFC-REQUIREMENTS.md that drifts from a fresh
    render must fail `--check-fresh` (what ze-doc-verify runs) and name the regeneration
    target. TestLedgerFreshness drives the check_ledger_fresh helper; this drives the
    run_check_fresh entry point, so an unwired helper still fails here. Driven from
    fixtures so it does not depend on the live tree (whose tags may be mid-flight)."""

    REQS = [_req("RFC7606-2-1")]
    TAGS = [_tag("RFC7606-2-1", "positive"), _tag("RFC7606-2-1", "negative")]

    def _drive(self, committed):
        """`committed` is the index on disk: a string, None for absent, or a callable that
        renders it inside the patch. The render cites the shard directory, so a body built
        against the real one would read stale for the path alone."""
        path = _mkstemp(".md")
        # The shard directory moves with the ledger, holding the CORRECT render, so every
        # case below varies the index alone.
        shard_dir = _mkdtemp("stale-shards-")
        try:
            with _patched(
                LEDGER_FILE=path,
                SHARD_DIR=shard_dir,
                _collect_for_check=lambda: ({"rfc7606"}, self.REQS, [], self.TAGS, {}),
            ):
                for stem, body in R.render_shards(
                    self.REQS, self.TAGS, {"rfc7606"}
                ).items():
                    with open(
                        os.path.join(shard_dir, stem + ".md"), "w", encoding="utf-8"
                    ) as fh:
                        fh.write(body + "\n")
                text = committed(self) if callable(committed) else committed
                if text is None:
                    os.unlink(path)
                else:
                    with open(path, "w", encoding="utf-8") as fh:
                        fh.write(text)
                return _run_capturing(R.run_check_fresh)
        finally:
            shutil.rmtree(shard_dir, ignore_errors=True)
            if os.path.exists(path):
                os.unlink(path)

    def test_fresh_ledger_passes(self):
        code, out = self._drive(
            lambda case: R.render_index(case.REQS, case.TAGS, {"rfc7606"}) + "\n"
        )
        self.assertEqual(code, 0, out)

    def test_stale_ledger_fails_and_names_regen_target(self):
        """Would pass under a version that never compared; fails because the committed copy
        differs from the fresh render, and the message points at the regeneration target."""
        code, out = self._drive("# a stale, hand-drifted ledger\n")
        self.assertNotEqual(code, 0, out)
        self.assertIn("ze-rfc-index-update", out)

    def test_missing_ledger_fails(self):
        """A missing ledger must fail closed at the CLI too, not pass by vacuum."""
        code, out = self._drive(None)
        self.assertNotEqual(code, 0, out)

    def test_a_parse_error_refuses_the_verdict_and_names_the_summary(self):
        """The sibling of `run_write`'s destructive discard, one function away.

        `_collect_for_check` catches a ParseError per summary and carries on. `run_check`
        reports every one; `run_write` refuses on them. This driver swallowed them, and a
        swallowed parse error does not produce a WRONG-looking answer here -- it produces a
        confident one about the wrong thing. The stem that failed to parse renders no rows,
        so the freshness comparison calls the index stale and calls that RFC's file an
        orphan the generator no longer owns. Both readings are false: the RFC is fine and
        its file must be kept. A reader who believes the orphan line deletes the evidence
        file by hand, which is the deletion `run_write` now refuses to make itself.

        The fixture is the real shape: ONE summary of several fails while the rest render.
        A fixture where nothing rendered would fail for the emptiness instead, and would
        stay green over the defect this test exists for.
        """
        with _shard_tree():
            os.makedirs(R.SHARD_DIR, exist_ok=True)
            # Disk holds the COMPLETE render, which is what a correct tree looks like.
            with open(R.LEDGER_FILE, "w", encoding="utf-8") as fh:
                fh.write(R.render_index(_SHARD_REQS, _SHARD_TAGS, {"rfc7606"}) + "\n")
            for stem, body in R.render_shards(
                _SHARD_REQS, _SHARD_TAGS, {"rfc7606"}
            ).items():
                with open(R.shard_path(stem), "w", encoding="utf-8") as fh:
                    fh.write(body + "\n")

            # rfc9999's summary stopped parsing, so the collection carries the error and
            # loses that stem's rows. Its file is untouched on disk.
            partial = [r for r in _SHARD_REQS if r.rfc != "rfc9999"]
            with _patched(
                _collect_for_check=lambda: (
                    {"rfc7606"},
                    partial,
                    ["rfc/short/rfc9999.md:1: unparseable checklist line"],
                    _SHARD_TAGS,
                    {},
                ),
            ):
                code, out = _run_capturing(R.run_check_fresh)

            self.assertEqual(code, 2, out)
            self.assertIn(
                "unparseable checklist line", out, "the swallowed error is named"
            )
            self.assertNotIn(
                "renders no requirement section",
                out,
                "an unparsed summary must never be reported as a retired RFC whose file "
                "the generator no longer owns",
            )

    def test_render_is_independent_of_tag_order(self):
        """--check-fresh only works if the render does not depend on the order the scanner
        happened to find things in: scan_tree walks the filesystem, so the same tree yields
        the same tags in a different order on a different machine, and an order-sensitive
        render would report a fresh ledger as stale there. Reversing the inputs is the test
        that can see that; re-rendering identical arguments is not.

        Both outputs, because both are committed generated files: an order-sensitive shard
        churns across machines exactly as an order-sensitive index does."""
        tags = list(reversed(self.TAGS))
        self.assertEqual(
            R.render_index(self.REQS, self.TAGS, {"rfc7606"}),
            R.render_index(list(reversed(self.REQS)), tags, {"rfc7606"}),
        )
        self.assertEqual(
            R.render_shards(self.REQS, self.TAGS, {"rfc7606"}),
            R.render_shards(list(reversed(self.REQS)), tags, {"rfc7606"}),
        )


# --------------------------------------------------------------------------
# Status disclosure fails closed on empty Remaining (AC-10)
# --------------------------------------------------------------------------
class TestStatusDisclosureFailsClosed(unittest.TestCase):
    """AC-10: a 'Supported' row with a blank Remaining does NOT disclose a {gap} MUST.
    The old code let the empty string slip through _NO_GAP_RE and the gap hid behind a
    clean 'Supported' claim -- the exact lie the cross-check exists to catch."""

    def _rows(self, remaining):
        status = (
            "| RFC | Area | Status | Coverage | Remaining |\n"
            "|-----|------|--------|----------|-----------|\n"
            "| RFC 7606 | x | Supported | cov | %s |\n" % remaining
        )
        return R.parse_status_ledger(status)

    def test_supported_with_empty_remaining_fails(self):
        for remaining in ("", "   ", "No tracked gap in current source anchors."):
            rows = self._rows(remaining)
            ann = R.Annotation(kind="gap", polarity=None, reason="ordering")
            errs = R.check_status_agreement(
                [_req("RFC7606-5.1-1", annotation=ann)], rows, {"rfc7606"}
            )
            self.assertTrue(
                errs, "Supported + non-disclosing Remaining %r must fail" % remaining
            )
            joined = " ".join(errs)
            self.assertIn("RFC7606-5.1-1", joined)
            self.assertIn("rfc7606", joined)

    def test_supported_with_real_gap_text_passes(self):
        """A 'Supported' row that spells the gap out in Remaining discloses it -> passes,
        so the fix does not over-fire on genuinely disclosed gaps."""
        rows = self._rows("Ze emits MP_UNREACH first, not compliant with 5.1 ordering.")
        ann = R.Annotation(kind="gap", polarity=None, reason="ordering")
        errs = R.check_status_agreement(
            [_req("RFC7606-5.1-1", annotation=ann)], rows, {"rfc7606"}
        )
        self.assertEqual(errs, [])


# --------------------------------------------------------------------------
# Fail-closed on unreadable inputs (Fix 4)
# --------------------------------------------------------------------------
class TestRunCheckReadErrors(unittest.TestCase):
    """An OSError reading rfc/enrolled.txt or docs/features/rfc-status.md must exit 2 with
    a clean message, not an uncaught traceback. Drives run_check so a handler that stops
    catching OSError surfaces here (the exception would escape _run_capturing)."""

    def test_unreadable_enrolled_exits_two_cleanly(self):
        def boom():
            raise OSError("[Errno 13] Permission denied: rfc/enrolled.txt")

        with _patched(load_enrolled=boom):
            code, out = _run_capturing(R.run_check)
        self.assertEqual(code, 2, out)
        self.assertIn("cannot run", out)


# --------------------------------------------------------------------------
# Extraction sign-off (plan/spec-rfcgate-1-extraction.md)
# --------------------------------------------------------------------------
# A miniature RFC in the real wire format: numbered headings at column 0, the RFC 2119
# boilerplate in §1 (which must NOT become a site), and two normative sentences in §2.
_SRC_TWO_SITES = """\
Network Working Group                                          A. Tester
Request for Comments: 9999                                  October 2026


1.  Introduction

   The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT",
   "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this
   document are to be interpreted as described in RFC 2119.

2.  Requirements

   A speaker MUST do the first thing.  A speaker MUST NOT do the second
   thing.

3.  References

   Nothing normative lives here.
"""

# No capitalised MUST-level keyword ANYWHERE: the pre-2119 shape (A-3), the 22 enrolled
# RFCs whose source has none at all. The obligations are stated in lowercase indicative
# prose, which only the `prose` register can see. Deliberately keyword-free rather than
# keyword-poor: the undercount arm has its own fixture (_SRC_TWO_SITES with gated=23), and
# a fixture covering both arms at once would let either one break unnoticed.
_SRC_PROSE_ONLY = """\
1.  Introduction

   This memo states its obligations in ordinary prose, as documents
   predating the 2119 conventions do.

2.  Rules

   A speaker must do the first thing.  A speaker shall not do the second
   thing.
"""

# The rfc2865/rfc2869/rfc1195/sflow-v5 shape: a column-0 attribute-table row that the
# heading pattern reads as a section heading. Its number COLLIDES with a real heading, and
# its text is a live obligation. Before the merge fix, all three consequences fired at once:
# the sentence was dropped from the inventory, the section id was emitted twice (so the
# generated skeleton could not be re-read), and two bodies sharing an id both numbered
# their sites from 1.
_SRC_TABLE_ROW_HEADING = """\
1.  Introduction

   This document defines attributes.

2.  Attribute Table

   The table below gives, for each attribute, how often it may appear:

1     Exactly one instance of this attribute MUST be present in packet.
2     A speaker MUST NOT send more than one instance of it.
"""

# Neither register finds anything: the rfc1877 shape (A-5). Only a manual walk remains.
_SRC_NO_INVENTORY = """\
1.  Introduction

   This document describes a format.  It states no obligations at all.

2.  Format

   The field is four octets wide.
"""


def _sha(src):
    return R.requirement_sha(src)


def _site(sid, quote, disposition=None, **over):
    entry = {"id": sid, "quote": quote, "disposition": disposition}
    entry.update(over)
    return entry


def _section(sid, sites, disposition=None, **over):
    entry = {"id": sid, "sites": sites, "disposition": disposition}
    entry.update(over)
    return entry


_Q1 = "A speaker MUST do the first thing."
_Q2 = "A speaker MUST NOT do the second thing."
_Q3 = "A speaker MUST also do a third thing."

# Three sites, so a fixture can hold mapped != excluded and both non-zero. Two sites cannot:
# every split of two is either degenerate (2/0, 0/2) or symmetric (1/1), and a symmetric one
# cannot see mapped and excluded swapped.
_SRC_THREE_SITES = _SRC_TWO_SITES.replace(
    "   A speaker MUST do the first thing.  A speaker MUST NOT do the second\n   thing.\n",
    "   A speaker MUST do the first thing.  A speaker MUST NOT do the second\n"
    "   thing.  A speaker MUST also do a third thing.\n",
)


def _artifact(stem="rfc9999", src=_SRC_TWO_SITES, register="rfc2119", **over):
    """A fully classified, valid sign-off for _SRC_TWO_SITES. Tests break ONE field."""
    art = {
        "schema-version": 1,
        "stem": stem,
        "register": register,
        "source-path": f"rfc/full/{stem}.txt",
        "source-sha": _sha(src),
        "signed-off": "2026-07-29",
        "reviewer": "tester",
        "sections": [
            _section(
                "front",
                0,
                "skipped",
                **{"skip-kind": "front-matter", "reason": "boilerplate"},
            ),
            _section("1", 0, "walked"),
            _section("2", 2, "walked"),
            _section(
                "3",
                0,
                "skipped",
                **{"skip-kind": "references", "reason": "no obligations"},
            ),
        ],
        "sites": [
            _site("2:1", _Q1, "mapped", **{"mapped-to": "RFC9999-2-1"}),
            _site("2:2", _Q2, "mapped", **{"mapped-to": "RFC9999-2-2"}),
        ],
    }
    art.update(over)
    return art


# Sentinel for `_extraction_tree(baseline=...)`: leave the PRODUCTION HEAD reader in
# place. Only a test whose subject IS that reader wants this; see
# TestExtractionRatchet.test_git_failure_judges_nothing, which drives it through a fake
# git and must observe the real None-on-failure path.
_LIVE_BASELINE = object()


# The destination of a relocation, as the artifact spells it and as the file is named. A
# FIXTURE spec, never a real one: plan/spec-*.md is deleted the day its work closes
# (ai/rules/planning.md), so a test pinned to a live spec would red on the day the
# obligation it points at was finally met.
_SPEC_NAME = "spec-relocation-fixture.md"
_SPEC_REL = "plan/" + _SPEC_NAME
_SPEC_BODY = (
    "# Fixture spec\n\n"
    "| Requirement | Level | Text |\n"
    "|---|---|---|\n"
    "| `RFC9999-2-3` | MUST | the obligation this spec owes |\n"
)


@contextlib.contextmanager
def _spec_tree(specs=None):
    """A temp plan/, holding {filename: text}. SPEC_DIR is patched the way EXTRACTION_DIR
    is, so the tripwire reads this tree and never the repository's own plan/.

    Default: the one fixture spec, carrying the one reserved id. Pass `{}` for "the
    destination spec does not exist".
    """
    tmp = _mkdtemp("ze-spec-")
    body = {_SPEC_NAME: _SPEC_BODY} if specs is None else specs
    for name, text in body.items():
        with open(os.path.join(tmp, name), "w", encoding="utf-8") as fh:
            fh.write(text)
    try:
        with _patched(SPEC_DIR=tmp):
            yield tmp
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


def _relocated_site(sid, quote, rel=_SPEC_REL, rid="RFC9999-2-3", **over):
    entry = {
        "excluded-kind": "relocated-to-spec",
        "relocated-to": rel,
        "reserved-id": rid,
        "reason": "owner ruling moved this obligation to the named spec, still gated there",
    }
    entry.update(over)
    return _site(sid, quote, "excluded", **entry)


def _relocated_artifact(rel=_SPEC_REL, rid="RFC9999-2-3", **over):
    """`_artifact()` with its second site RELOCATED rather than mapped.

    Its summary therefore declares ONE requirement (`_reqs_9999(1)`), which is the whole
    point of the kind: the row for the relocated obligation is not in rfc/short/ at all,
    it is reserved in the destination spec.
    """
    art = _artifact()
    art["sites"][1] = _relocated_site("2:2", _Q2, rel, rid, **over)
    return art


@contextlib.contextmanager
def _extraction_tree(
    artifacts=None, budget="start 2026-07-01\nrate 0\n", src=None, baseline=None
):
    """A temp rfc/extraction/ plus rfc/drain-budget.txt, with the source text patched.

    `artifacts` maps stem -> dict (written as JSON) or str (written verbatim, for the
    malformed-input cases). `budget=None` deletes the budget file.

    THE HEAD BASELINE IS PART OF THE FIXTURE. `baseline` (default `{}`, i.e. "HEAD holds
    no artifact for this tree") replaces `_git_baseline_extractions`, because the ratchets
    consume `baseline - current` and `current` is this temp directory. Leaving the
    production reader live pointed one side of that subtraction at the real repository
    HEAD and the other at a temp dir, so the comparison was between two unrelated trees
    and every artifact committed under rfc/extraction/ read as a sign-off this fixture had
    "deleted". It stayed invisible only while HEAD carried no artifacts; the four
    sign-offs of plan/spec-rfcgate-4-ledger.md turned eight fixture tests red at once,
    the THIRD instance of one trap in this spec set (the first two were
    `extraction_stems() == set()` and `budget.rate == 0.0` asserted against the live
    corpus). Both sides now come from the same place, so no future commit can separate
    them. Pass `baseline=` to state a different HEAD, or `_LIVE_BASELINE` to opt out.
    """
    tmp = _mkdtemp("ze-extract-")
    ext = os.path.join(tmp, "extraction")
    os.makedirs(ext)
    for stem, body in (artifacts or {}).items():
        with open(os.path.join(ext, stem + ".json"), "w", encoding="utf-8") as fh:
            fh.write(body if isinstance(body, str) else json.dumps(body, indent=2))
    budget_path = os.path.join(tmp, "drain-budget.txt")
    if budget is not None:
        with open(budget_path, "w", encoding="utf-8") as fh:
            fh.write(budget)
    texts = src if src is not None else {}
    over = {"EXTRACTION_DIR": ext, "DRAIN_BUDGET_FILE": budget_path}
    if baseline is not _LIVE_BASELINE:
        head = {} if baseline is None else baseline
        over["_git_baseline_extractions"] = lambda: head
    if src is not None:
        over["source_text"] = lambda stem: texts.get(stem)
        over["source_path"] = lambda stem: (
            f"rfc/full/{stem}.txt" if stem in texts else None
        )
    try:
        with _patched(**over):
            yield ext
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


def _reqs_9999(n=2):
    return [_req(f"RFC9999-2-{i}", rfc="rfc9999") for i in range(1, n + 1)]


class TestSiteInventory(unittest.TestCase):
    """The inventory is DERIVED from the source text. A hand-typed 'sites seen' is a
    claim, and claims are what this programme removes (ai/rules/evidence.md)."""

    def _inv(self, src, gated=0, stem="rfc9999"):
        with _patched(
            source_text=lambda s: src, source_path=lambda s: f"rfc/full/{s}.txt"
        ):
            return R.derive_inventory(stem, gated)

    def test_sites_are_attributed_to_enclosing_section(self):
        """Locator is <section>:<n>, the nth normative site of that section in document
        order."""
        inv = self._inv(_SRC_TWO_SITES)
        self.assertEqual([s.id for s in inv.sites], ["2:1", "2:2"])
        self.assertIn(_Q1, inv.sites[0].quote)
        self.assertIn("second", inv.sites[1].quote)

    def test_every_section_is_enumerated_with_its_site_count(self):
        inv = self._inv(_SRC_TWO_SITES)
        counts = {s.id: s.sites for s in inv.sections}
        self.assertEqual(counts.get("2"), 2)
        self.assertEqual(counts.get("1"), 0)
        self.assertEqual(counts.get("3"), 0)

    def test_content_before_the_first_heading_is_still_attributed(self):
        """A site must never be dropped for preceding the first numbered heading: that
        would be a silent hole in the very bound this artifact exists to provide."""
        inv = self._inv(
            "Preamble text.  A speaker MUST do it.\n\n1.  Body\n\n   None.\n"
        )
        self.assertEqual([s.id for s in inv.sites], ["front:1"])

    def test_rfc2119_boilerplate_is_not_a_site(self):
        """The 'key words ... interpreted as described' sentence is not an obligation."""
        inv = self._inv(_SRC_TWO_SITES)
        joined = " ".join(s.quote for s in inv.sites)
        self.assertNotIn("interpreted as described", joined)
        self.assertEqual(len(inv.sites), 2)

    def test_rfc8174_boilerplate_variant_is_not_a_site(self):
        src = (
            "1.  Terms\n\n"
            '   The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT",\n'
            '   "SHOULD", "MAY" in this document are to be interpreted as described\n'
            "   in BCP 14 [RFC2119] [RFC8174] when, and only when, they appear in\n"
            "   all capitals, as shown here.\n"
        )
        self.assertEqual(self._inv(src).sites, [])

    def test_obligation_merged_into_the_key_words_paragraph_is_still_a_site(self):
        """`_sentences` feeds the inventory as well as the gate counter, so the merge that
        hides a MUST from the counter also hides it from the reviewer's walk -- and a site
        nobody sees is a site nobody classifies. The quote must be the obligation alone,
        because the sign-off records what was classified."""
        for src in (_SRC_MERGED_DIGIT, _SRC_MERGED_LOWER):
            sites = self._inv(src).sites
            self.assertEqual(len(sites), 1, sites)
            self.assertNotIn("interpreted as described", sites[0].quote)
            self.assertRegex(sites[0].quote, r"^(6PE routers|iSCSI targets) MUST ")

    def test_register_is_derived_not_authored(self):
        """AC-9: a keyword-rich source derives rfc2119; a keyword-free one derives
        prose."""
        self.assertEqual(self._inv(_SRC_TWO_SITES, gated=2).register, "rfc2119")
        self.assertEqual(self._inv(_SRC_PROSE_ONLY, gated=2).register, "prose")

    def test_register_falls_to_prose_when_sites_undercount_declared(self):
        """A-2, the rfc2181 shape: 2 keyword sites, 23 gated rows declared. The keyword
        inventory cannot bound a summary that legitimately out-declares it."""
        self.assertEqual(self._inv(_SRC_TWO_SITES, gated=23).register, "prose")

    def test_prose_register_inventory_is_not_empty(self):
        """A-4: the case-insensitive modal scan finds sites a keyword scan cannot."""
        inv = self._inv(_SRC_PROSE_ONLY, gated=2)
        self.assertEqual(inv.register, "prose")
        self.assertTrue(inv.sites, "prose register must yield a non-empty inventory")
        self.assertEqual(inv.keyword_sites, 0)

    def test_empty_under_both_registers_derives_manual_walk(self):
        """A-5, the rfc1877 shape: no mechanical inventory exists at all."""
        inv = self._inv(_SRC_NO_INVENTORY, gated=4)
        self.assertEqual(inv.register, "manual-walk")
        self.assertEqual(inv.sites, [])

    def test_missing_source_text_derives_nothing(self):
        """Fail closed: no source means no inventory to judge, never an empty one that
        reads as 'nothing to extract' (ai/rules/evidence.md, zero-value trap)."""
        with _patched(source_text=lambda s: None, source_path=lambda s: None):
            self.assertIsNone(R.derive_inventory("rfc9999", 3))

    def test_page_furniture_is_not_part_of_a_quote(self):
        """A sentence spanning a page break must not carry the footer, the form feed and
        the running header into its derived quote."""
        src = (
            "2.  Rules\n\n"
            "   A speaker MUST do the first\n\n"
            "Tester                       Standards Track              [Page 3]\n"
            "\f\n"
            "RFC 9999                       Testing                  October 2026\n\n"
            "   thing before anything else.\n"
        )
        inv = self._inv(src)
        self.assertEqual(len(inv.sites), 1)
        self.assertNotIn("[Page", inv.sites[0].quote)
        self.assertNotIn("Standards Track", inv.sites[0].quote)
        self.assertIn("first thing before anything else", inv.sites[0].quote)

    def test_source_sha_tracks_the_source_text(self):
        """AC-4: the artifact's freshness key is the source text itself."""
        a = self._inv(_SRC_TWO_SITES)
        b = self._inv(_SRC_TWO_SITES.replace("first", "primary"))
        self.assertNotEqual(a.source_sha, b.source_sha)

    def test_a_line_read_as_a_heading_keeps_its_own_sentence(self):
        """A false heading match must never ERASE the sentence it matched.

        `_section_bodies` drops the matched line, so a column-0 attribute-table row
        carrying a live MUST vanished from the inventory the artifact exists to bound --
        silently, in four enrolled RFCs. Verified against rfc2865:2893, whose 'Exactly one
        instance of this attribute MUST be present in packet.' was absent from all 72
        derived sites."""
        inv = self._inv(_SRC_TABLE_ROW_HEADING)
        quotes = [s.quote for s in inv.sites]
        self.assertIn(
            "Exactly one instance of this attribute MUST be present in packet.", quotes
        )
        self.assertIn("A speaker MUST NOT send more than one instance of it.", quotes)

    def test_a_repeated_section_id_is_merged_not_emitted_twice(self):
        """`parse_extraction_artifact` refuses a duplicate section, so a derivation that
        emits one produces a skeleton that cannot be re-read: `make ze-rfc-extraction-create` exits 0
        having bricked the stem, and every later --check reports 'cannot run', hiding every
        other RFC violation in the repository."""
        inv = self._inv(_SRC_TABLE_ROW_HEADING)
        ids = [s.id for s in inv.sections]
        self.assertEqual(sorted(ids), sorted(set(ids)), f"duplicate section ids: {ids}")

    def test_site_locators_are_unique_across_the_whole_inventory(self):
        """`_evaluate_extraction` keys sites into a dict, so two sites sharing a locator
        collapse into one and the second obligation is judged by nobody."""
        inv = self._inv(_SRC_TABLE_ROW_HEADING)
        ids = [s.id for s in inv.sites]
        self.assertEqual(sorted(ids), sorted(set(ids)), f"duplicate site ids: {ids}")

    def test_a_duplicate_locator_from_the_derivation_fails_closed(self):
        """The invariant is asserted at its PRODUCER, not left for a downstream dict to
        swallow (ai/rules/evidence.md: a guard that cannot deny must say so).

        The memo is cleared first, and that is load-bearing rather than tidiness. This is the one
        test whose subject is reached only by RUNNING the derivation, and derive_inventory answers
        from _INVENTORY_MEMO before it looks at _section_bodies -- so any earlier test in the
        process that derived this stem, this gated count and this body serves the cached
        inventory and the patched duplicate is never produced. That is how it broke: a new class
        deriving _SRC_TWO_SITES under "rfc9999" at gated=0 ran first (classes load alphabetically)
        and this assertion started reporting "ParseError not raised". The clear makes the test
        independent of what ran before it."""
        R._INVENTORY_MEMO.clear()
        with _patched(
            _section_bodies=lambda text: [("2", "A speaker MUST do it.")] * 2
        ):
            with self.assertRaises(R.ParseError) as cm:
                self._inv(_SRC_TWO_SITES)
        self.assertIn("2:1", str(cm.exception))

    def test_the_memo_is_keyed_on_the_source_not_the_stem(self):
        """derive_inventory is memoised (run_check derives every signed stem three times,
        at ~8.5ms each). The key includes the source sha, so the SAME stem with a DIFFERENT
        body derives afresh.

        A stem-keyed memo would hand the second call the first call's inventory: sites,
        quotes and register all belonging to text nobody has now -- and it would do it
        silently, in the direction of a sign-off that still validates after its source
        moved, which is precisely what source-sha freshness exists to catch."""
        first = self._inv(_SRC_TWO_SITES, gated=2)
        moved = _SRC_TWO_SITES.replace("the first thing", "something else entirely")
        second = self._inv(moved, gated=2)
        self.assertNotEqual(first.source_sha, second.source_sha)
        self.assertIn("something else entirely", second.sites[0].quote)
        # ...and the original is still served for the original body.
        self.assertEqual(self._inv(_SRC_TWO_SITES, gated=2), first)

    def test_the_memo_is_keyed_on_the_declared_gated_count(self):
        """The gated count decides the REGISTER (the undercount clause), so it is part of
        the key. Dropping it would freeze the first caller's register for the stem."""
        self.assertEqual(self._inv(_SRC_TWO_SITES, gated=2).register, "rfc2119")
        self.assertEqual(self._inv(_SRC_TWO_SITES, gated=23).register, "prose")
        self.assertEqual(self._inv(_SRC_TWO_SITES, gated=2).register, "rfc2119")

    def test_the_memo_key_survives_an_indentation_only_variant(self):
        """The key is the RAW bytes, never the NORMALIZED requirement sha.

        `requirement_sha` runs `_normalize`, which strips every line and drops blank ones
        -- and the derivation depends on exactly those two things. `_SECTION_HEADING_RE`
        anchors at `^`, so leading whitespace decides whether a line is a heading at all,
        and `_sentences` splits paragraphs on blank lines. Under a normalized key these
        two bodies collide (both sha 9daecc1f191899eb) and the indented one is served the
        flush one's inventory: a heading that does not exist, and a site located at 1:1
        instead of front:1.
        """
        flush = "1. Rules\n\nThe speaker MUST send it.\n"
        indented = "   " + flush
        self.assertEqual(
            R.requirement_sha(flush),
            R.requirement_sha(indented),
            "fixture no longer exercises the collision the key must survive",
        )
        first = self._inv(flush)
        self.assertEqual([s.id for s in first.sections], ["front", "1"])
        self.assertEqual([s.id for s in first.sites], ["1:1"])

        second = self._inv(indented)
        self.assertEqual(
            [s.id for s in second.sections],
            ["front"],
            "an indented line is not a heading; the memo served a stale derivation",
        )
        self.assertEqual([s.id for s in second.sites], ["front:1"])

    def test_the_memo_key_survives_a_relocated_source(self):
        """`source_path` is part of the key. The same stem carrying the same bytes at a
        DIFFERENT path (rfc/full -> rfc/drafts, the two locations source_path searches)
        must not be served the old path: the artifact records `source-path`, and
        `_evaluate_extraction` compares the sign-off's copy against the re-derived one, so
        a stale path is a sign-off validated against a file nobody is reading."""
        src = _SRC_TWO_SITES
        with _patched(
            source_text=lambda s: src, source_path=lambda s: "rfc/full/rfc9999.txt"
        ):
            full = R.derive_inventory("rfc9999", 0)
        with _patched(
            source_text=lambda s: src, source_path=lambda s: "rfc/drafts/rfc9999.txt"
        ):
            draft = R.derive_inventory("rfc9999", 0)
        self.assertEqual(full.source_path, "rfc/full/rfc9999.txt")
        self.assertEqual(
            draft.source_path,
            "rfc/drafts/rfc9999.txt",
            "the memo served the stale path for a relocated source",
        )

    def test_derivation_is_deterministic(self):
        """Pinned against a hand-typed expectation, not against a second call of itself:
        f(x) == f(x) in one process holds for every implementation, including a broken
        one."""
        inv = self._inv(_SRC_TWO_SITES)
        self.assertEqual(
            [(s.id, s.quote, s.section) for s in inv.sites],
            [
                ("2:1", "A speaker MUST do the first thing.", "2"),
                ("2:2", "A speaker MUST NOT do the second thing.", "2"),
            ],
        )
        self.assertEqual(
            [(s.id, s.sites) for s in inv.sections],
            [("front", 0), ("1", 0), ("2", 2), ("3", 0)],
        )
        self.assertEqual(inv.register, "rfc2119")
        self.assertEqual(inv.keyword_sites, 2)


class TestExtractionArtifact(unittest.TestCase):
    """Parse-time validation. The enums are CLOSED and anything outside them is a
    ParseError, exactly as an unknown annotation kind is today (_parse_annotation:213)."""

    def _parse(self, body, stem="rfc9999"):
        tmp = _mkdtemp("ze-artifact-")
        try:
            path = os.path.join(tmp, stem + ".json")
            with open(path, "w", encoding="utf-8") as fh:
                fh.write(body if isinstance(body, str) else json.dumps(body))
            return R.parse_extraction_artifact(path)
        finally:
            shutil.rmtree(tmp, ignore_errors=True)

    def test_a_valid_artifact_parses(self):
        art = self._parse(_artifact())
        self.assertEqual(art.stem, "rfc9999")
        self.assertEqual(art.register, "rfc2119")
        self.assertEqual(len(art.sites), 2)

    def test_unreadable_artifact_raises_parse_error(self):
        """AC-18: malformed JSON is a clean exit-2, never a traceback."""
        with self.assertRaises(R.ParseError):
            self._parse("{ this is not json")

    def test_missing_file_raises_parse_error(self):
        with self.assertRaises(R.ParseError):
            R.parse_extraction_artifact("/nonexistent/rfc9999.json")

    def test_missing_reason_on_exclusion_fails(self):
        """AC-7."""
        art = _artifact()
        art["sites"][0] = _site(
            "2:1", _Q1, "excluded", **{"excluded-kind": "not-a-requirement"}
        )
        with self.assertRaises(R.ParseError):
            self._parse(art)

    def test_empty_reason_on_exclusion_fails(self):
        art = _artifact()
        art["sites"][0] = _site(
            "2:1",
            _Q1,
            "excluded",
            **{"excluded-kind": "cross-document", "reason": "  "},
        )
        with self.assertRaises(R.ParseError):
            self._parse(art)

    def test_unknown_exclusion_kind_fails(self):
        """AC-7: the kind set is closed."""
        art = _artifact()
        art["sites"][0] = _site(
            "2:1", _Q1, "excluded", **{"excluded-kind": "because-i-said", "reason": "x"}
        )
        with self.assertRaises(R.ParseError):
            self._parse(art)

    def test_mapped_site_without_a_target_fails(self):
        art = _artifact()
        art["sites"][0] = _site("2:1", _Q1, "mapped")
        with self.assertRaises(R.ParseError):
            self._parse(art)

    def test_unknown_site_disposition_fails(self):
        art = _artifact()
        art["sites"][0] = _site("2:1", _Q1, "sort-of-mapped")
        with self.assertRaises(R.ParseError):
            self._parse(art)

    def test_unknown_register_is_hard_error(self):
        """AC-31: a missing, empty or unknown register is a ParseError, never a silent
        default to the strong grade -- that would publish the weakest sign-off as the
        strongest, inverting the decision the register exists to record."""
        for bad in ("rfc-2119", "", None, "strong"):
            with self.assertRaises(R.ParseError, msg=repr(bad)):
                self._parse(_artifact(register=bad))
        art = _artifact()
        del art["register"]
        with self.assertRaises(R.ParseError):
            self._parse(art)

    def test_stem_must_match_the_filename(self):
        """Security review, path handling: the artifact cannot claim to be another RFC."""
        with self.assertRaises(R.ParseError):
            self._parse(_artifact(stem="rfc7606"), stem="rfc9999")

    def test_wrong_schema_version_fails(self):
        for bad in (0, 2, "1", None):
            with self.assertRaises(R.ParseError, msg=repr(bad)):
                self._parse(_artifact(**{"schema-version": bad}))

    def test_unknown_top_level_key_fails(self):
        """A typo'd key ('sigend-off') would otherwise read as an absent field, and an
        absent authored field is exactly what must never pass silently."""
        with self.assertRaises(R.ParseError):
            self._parse(_artifact(**{"signed-of": "2026-07-29"}))

    def test_an_unsigned_skeleton_still_parses(self):
        """`signed-off`, `reviewer` and `register-reason` are required to SIGN OFF, not to
        parse: an unsigned skeleton is a legal intermediate state, so a reviewer can run
        the check mid-walk and see which sites are left. TestExtractionSignoff proves the
        CHECK still refuses it."""
        art = self._parse(_artifact(reviewer="", **{"signed-off": ""}))
        self.assertEqual(art.reviewer, "")

    def test_skipped_section_needs_a_kind_and_a_reason(self):
        art = _artifact()
        art["sections"][3] = _section("3", 0, "skipped", **{"reason": "no obligations"})
        with self.assertRaises(R.ParseError):
            self._parse(art)
        art["sections"][3] = _section(
            "3", 0, "skipped", **{"skip-kind": "made-up", "reason": "x"}
        )
        with self.assertRaises(R.ParseError):
            self._parse(art)

    def test_duplicate_site_ids_fail(self):
        art = _artifact()
        art["sites"][1] = _site("2:1", _Q2, "mapped", **{"mapped-to": "RFC9999-2-2"})
        with self.assertRaises(R.ParseError):
            self._parse(art)

    def test_a_register_outside_the_closed_set_is_still_a_parse_error(self):
        """AC-31 stays at parse time even though the sign-off fields moved to check time:
        an unreadable register has no safe reading at all, whereas an empty date is
        simply 'not signed yet'."""
        with self.assertRaises(R.ParseError):
            self._parse(_artifact(register="rfc2119 "))

    # ------------------------------------------------------------------
    # relocated-to-spec, parse time. The tripwire itself lives in
    # TestRelocatedToSpec; these are the SHAPE refusals, which belong here beside every
    # other closed-enum and required-field case.
    # ------------------------------------------------------------------

    def test_a_relocated_site_parses(self):
        """The discriminating twin for every refusal below: the kind is legal, and its two
        authored fields survive the parse."""
        art = self._parse(_relocated_artifact())
        site = {s.id: s for s in art.sites}["2:2"]
        self.assertEqual(site.excluded_kind, "relocated-to-spec")
        self.assertEqual(site.relocated_to, _SPEC_REL)
        self.assertEqual(site.reserved_id, "RFC9999-2-3")

    def test_a_relocation_with_no_destination_spec_is_refused(self):
        """A pointer with no target is the shrug this kind exists NOT to be."""
        art = _relocated_artifact()
        del art["sites"][1]["relocated-to"]
        with self.assertRaisesRegex(R.ParseError, "needs a 'relocated-to'"):
            self._parse(art)

    def test_a_relocation_naming_something_other_than_a_spec_is_refused(self):
        """`relocated-to-spec` means a SPEC. A deferral shard, a known-failure file and a
        learned summary are the three homes ai/rules/rfc-compliance.md names as NOT a
        compliance decision procedure, and a path leaving the repository is not a document
        this gate may read at all."""
        for bad in (
            "plan/deferrals/rfcgate-1b-rfc7296-pilot.md",
            "plan/known-failures/whatever.md",
            "plan/learned/1300-something.md",
            "docs/architecture/ike.md",
            "plan/spec-x.txt",
            "/etc/passwd",
            "../plan/spec-x.md",
            "plan/../../etc/spec-x.md",
            "plan/sub/spec-x.md",
        ):
            art = _relocated_artifact(rel=bad)
            with self.assertRaisesRegex(
                R.ParseError, "needs a 'relocated-to'", msg=repr(bad)
            ):
                self._parse(art)

    def test_a_relocation_with_no_reserved_id_is_refused(self):
        """Without the id the relocation points at a document rather than at a row, and
        the tripwire has nothing to re-check."""
        art = _relocated_artifact()
        del art["sites"][1]["reserved-id"]
        with self.assertRaisesRegex(R.ParseError, "needs a 'reserved-id'"):
            self._parse(art)

    def test_a_reserved_id_of_another_rfc_is_refused(self):
        """The obligation is THIS RFC's. A reserved id belonging to another one would let
        any spec that happens to quote any requirement satisfy the tripwire."""
        for bad in ("RFC7606-2-1", "TBD", "RFC9999", "RFC99990-2-1", "rfc9999-2-3"):
            art = _relocated_artifact(rid=bad)
            with self.assertRaisesRegex(
                R.ParseError, "needs a 'reserved-id'", msg=repr(bad)
            ):
                self._parse(art)

    def test_relocation_fields_on_any_other_site_are_refused(self):
        """An authored field that means nothing where it sits must never pass silently:
        that is the same failure `_reject_unknown_keys` exists to stop, one level down."""
        for over in (
            {"relocated-to": _SPEC_REL},
            {"reserved-id": "RFC9999-2-3"},
        ):
            art = _artifact()
            art["sites"][0] = _site(
                "2:1", _Q1, "mapped", **{"mapped-to": "RFC9999-2-1"}, **over
            )
            with self.assertRaisesRegex(
                R.ParseError, "mean something only on a", msg=repr(over)
            ):
                self._parse(art)

            art = _artifact()
            art["sites"][0] = _site(
                "2:1",
                _Q1,
                "excluded",
                **{"excluded-kind": "not-a-requirement", "reason": "boilerplate"},
                **over,
            )
            with self.assertRaisesRegex(
                R.ParseError, "mean something only on a", msg=repr(over)
            ):
                self._parse(art)

    def test_the_kind_is_refused_as_a_section_skip_kind(self):
        """`relocated-to-spec` is a SITE disposition. The two closed sets stay apart: a
        section is a run of text, so it can be walked or skipped, and it can never be the
        thing a spec reserves an id for."""
        art = _artifact()
        art["sections"][3] = _section(
            "3", 0, "skipped", **{"skip-kind": "relocated-to-spec", "reason": "moved"}
        )
        with self.assertRaises(R.ParseError):
            self._parse(art)
        self.assertNotIn("relocated-to-spec", R.SECTION_SKIP_KINDS)


class TestSkeletonWriter(unittest.TestCase):
    """R-2, structurally: the writer can ONLY emit UNCLASSIFIED dispositions, so
    generating skeletons en masse makes the gate REDDER, never greener. There is no
    --sign-off mode, no default disposition and no bulk classifier."""

    def _write_raw(self, existing=None, src=_SRC_TWO_SITES, gated=2, stem="rfc9999"):
        """(exit code, stdout, artifact path, parsed file or None if absent).

        Tolerates a missing output file so the fail-closed cases can assert its ABSENCE.
        """
        tmp = _mkdtemp("ze-skel-")
        ext = os.path.join(tmp, "extraction")
        os.makedirs(ext)
        path = os.path.join(ext, stem + ".json")
        if existing is not None:
            with open(path, "w", encoding="utf-8") as fh:
                fh.write(json.dumps(existing, indent=2))
        try:
            with _patched(
                EXTRACTION_DIR=ext,
                source_text=lambda s: src,
                source_path=lambda s: f"rfc/full/{s}.txt",
                summary_stems=lambda: {stem},
                parse_summary_file=lambda p: _reqs_9999(gated),
            ):
                code, out = _run_capturing(lambda: R.run_extract_skeleton(stem))
            on_disk = None
            if os.path.exists(path):
                with open(path, encoding="utf-8") as fh:
                    on_disk = json.load(fh)
            # Fail-closed must not leave scratch behind either: a stray temp directory
            # under rfc/extraction/ would be committed by the next `git add`.
            leftovers = [n for n in sorted(os.listdir(ext)) if n != stem + ".json"]
            self.assertEqual(
                leftovers, [], "the writer left scratch in rfc/extraction/"
            )
            return code, out, path, on_disk
        finally:
            shutil.rmtree(tmp, ignore_errors=True)

    def _write(self, existing=None, src=_SRC_TWO_SITES, gated=2):
        code, out, _, on_disk = self._write_raw(existing=existing, src=src, gated=gated)
        self.assertIsNotNone(on_disk, out)
        return code, out, on_disk

    def test_skeleton_writes_every_site_unclassified(self):
        """AC-2: all N sites, every section, the derived register, source-path and
        source-sha -- and NOT ONE disposition invented."""
        code, out, art = self._write()
        self.assertEqual(code, 0, out)
        self.assertEqual([s["id"] for s in art["sites"]], ["2:1", "2:2"])
        self.assertTrue(all(s["disposition"] is None for s in art["sites"]))
        self.assertTrue(all(s["disposition"] is None for s in art["sections"]))
        self.assertEqual(art["register"], "rfc2119")
        self.assertEqual(art["source-path"], "rfc/full/rfc9999.txt")
        self.assertEqual(art["source-sha"], _sha(_SRC_TWO_SITES))
        self.assertEqual(art["schema-version"], 1)

    def test_a_generated_skeleton_fails_the_check(self):
        """R-2 is structural, not policy: generation cannot produce a pass."""
        _, _, art = self._write()
        with _extraction_tree(
            artifacts={"rfc9999": art}, src={"rfc9999": _SRC_TWO_SITES}
        ):
            errs = R.check_extraction_signoff(_reqs_9999())
        self.assertTrue(errs, "a freshly generated skeleton must FAIL the check")

    def test_skeleton_refresh_preserves_existing_classifications(self):
        """ai/rules/never-destroy-work.md: a re-run must not silently discard a
        reviewer's work."""
        _, _, art = self._write(existing=_artifact())
        by_id = {s["id"]: s for s in art["sites"]}
        self.assertEqual(by_id["2:1"]["disposition"], "mapped")
        self.assertEqual(by_id["2:1"]["mapped-to"], "RFC9999-2-1")
        self.assertEqual(art["reviewer"], "tester")
        self.assertEqual(art["signed-off"], "2026-07-29")

    def test_refresh_preserves_every_field_the_kind_requires(self):
        """A refresh copies the disposition and the kind. It did NOT copy the fields those
        kinds make MANDATORY, and the consequence is not a silent one: the writer
        re-parses its own output before it lands, so the whole refresh was refused.

        Found in the live tree, not in a fixture. rfc/extraction/rfc1035.json holds one
        `duplicate-of` site, so `make ze-rfc-extraction-create STEM=rfc1035` could not write at all
        -- it printed 'a defect in the derivation' and exited 2, leaving the reviewer no
        way to re-run the walk after the source moved. The carry-forward is now driven by
        what the PARSER stored rather than by a per-kind ladder, so a seventh kind with an
        authored field inherits it.
        """
        art = _artifact()
        art["sites"][1] = _site(
            "2:2",
            _Q2,
            "excluded",
            **{
                "excluded-kind": "duplicate-of",
                "mapped-to": "RFC9999-2-1",
                "reason": "restates RFC9999-2-1",
            },
        )
        code, out, _path, on_disk = self._write_raw(existing=art)
        self.assertEqual(code, 0, out)
        by_id = {s["id"]: s for s in on_disk["sites"]}
        self.assertEqual(by_id["2:2"]["excluded-kind"], "duplicate-of")
        self.assertEqual(by_id["2:2"]["mapped-to"], "RFC9999-2-1")

        code, out, _path, on_disk = self._write_raw(existing=_relocated_artifact())
        self.assertEqual(code, 0, out)
        by_id = {s["id"]: s for s in on_disk["sites"]}
        self.assertEqual(by_id["2:2"]["excluded-kind"], "relocated-to-spec")
        self.assertEqual(by_id["2:2"]["relocated-to"], _SPEC_REL)
        self.assertEqual(by_id["2:2"]["reserved-id"], "RFC9999-2-3")

    def test_refresh_drops_a_classification_whose_sentence_changed(self):
        """A locator is a POSITION. If the source text moved under it, keeping the old
        disposition would silently re-point a reviewer's decision at a sentence they
        never read -- the same hazard check_id_allocation:427 exists to stop."""
        moved = _SRC_TWO_SITES.replace(
            "A speaker MUST do the first thing.", "A speaker MUST do something else."
        )
        _, _, art = self._write(existing=_artifact(), src=moved)
        by_id = {s["id"]: s for s in art["sites"]}
        self.assertIsNone(by_id["2:1"]["disposition"])
        self.assertEqual(by_id["2:2"]["disposition"], "mapped")

    def test_a_stem_carrying_a_path_is_refused(self):
        """`stem` reaches os.path.join for the source lookup AND the artifact path, and the
        Makefile passes $(STEM) unquoted. '../full/rfc4271' resolves a real source and
        would write outside rfc/extraction/."""
        for bad in (
            "../full/rfc4271",
            "rfc/short/rfc4271",
            "..",
            "rfc4271/../../etc/passwd",
            "/etc/passwd",
            "RFC4271",
            # Python's `$` matches BEFORE a trailing newline, so `^...$` under re.match
            # accepted "rfc4271\n". Not a traversal -- it is the ONE shape `$` lets
            # through, and it lands as a file literally named "rfc4271\n.json" -- but the
            # stem reaches os.path.join for both the source lookup and the artifact path,
            # and a validator for a filename that admits a line terminator is not one.
            "rfc4271\n",
        ):
            with self.assertRaises(R.ParseError, msg=repr(bad)):
                R._validated_stem(bad)

        # Wired, and discriminating: '../full/rfc4271' names a source that really EXISTS,
        # so it reaches the writer rather than stopping at 'no source text'. The staging
        # dir is a temp one, so an unwired validator writes its escape there and is seen
        # by the assertion below rather than landing in rfc/full/.
        tmp = _mkdtemp("ze-stem-")
        try:
            with _patched(EXTRACTION_DIR=tmp):
                code, out = _run_capturing(
                    lambda: R.main(["prog", "--extract-skeleton", "../full/rfc4271"])
                )
            self.assertEqual(code, 2, out)
            self.assertIn("not an RFC or draft stem", out)
            self.assertEqual(
                os.listdir(tmp), [], "nothing may be written for a bad stem"
            )
        finally:
            shutil.rmtree(tmp, ignore_errors=True)

    def test_a_real_stem_shape_is_accepted(self):
        """The discriminating twin: the validator must not refuse the corpus it serves --
        RFC stems, draft stems with hyphens and a version suffix, and 'sflow-v5'."""
        for good in ("rfc4271", "sflow-v5", "draft-ietf-bess-mup-safi-00"):
            self.assertEqual(R._validated_stem(good), good)

    def test_skeleton_for_an_unknown_stem_fails_closed(self):
        tmp = _mkdtemp("ze-skel-")
        try:
            with _patched(
                EXTRACTION_DIR=tmp,
                source_text=lambda s: None,
                source_path=lambda s: None,
            ):
                code, out = _run_capturing(lambda: R.run_extract_skeleton("rfc0000"))
            self.assertEqual(code, 2, out)
            self.assertIn("no source text", out)
        finally:
            shutil.rmtree(tmp, ignore_errors=True)

    def _staged_run(self, ages):
        """Plant a `.staging-<age>` dir per entry in `ages` (seconds old), run the writer,
        and return the leftover staging dir names."""
        tmp = _mkdtemp("ze-stale-")
        ext = os.path.join(tmp, "extraction")
        os.makedirs(ext)
        try:
            now = time.time()
            for age in ages:
                planted = os.path.join(ext, R._STAGING_PREFIX + "age%d" % age)
                os.makedirs(planted)
                # A marker file, so an implementation that empties the directory without
                # removing it cannot pass by looking tidy.
                with open(os.path.join(planted, "half-written.json"), "w") as fh:
                    fh.write("{")
                os.utime(planted, (now - age, now - age))
            with _patched(
                EXTRACTION_DIR=ext,
                source_text=lambda s: _SRC_TWO_SITES,
                source_path=lambda s: f"rfc/full/{s}.txt",
                summary_stems=lambda: {"rfc9999"},
                parse_summary_file=lambda p: _reqs_9999(2),
            ):
                code, out = _run_capturing(lambda: R.run_extract_skeleton("rfc9999"))
            self.assertEqual(code, 0, out)
            self.assertTrue(os.path.exists(os.path.join(ext, "rfc9999.json")), out)
            return sorted(n for n in os.listdir(ext) if n.startswith(R._STAGING_PREFIX))
        finally:
            shutil.rmtree(tmp, ignore_errors=True)

    def test_an_abandoned_staging_dir_is_swept_on_entry(self):
        """The writer stages beside its target so os.replace is an atomic same-filesystem
        rename -- which means a kill between mkdtemp and the finally leaves a `.staging-*`
        directory inside TRACKED rfc/extraction/, where the next `git add` commits it.
        The next run must clear it."""
        stale = 10 * R._STAGING_STALE_SECONDS
        self.assertEqual(self._staged_run([stale]), [])

    def test_a_fresh_staging_dir_is_left_alone(self):
        """The age gate is load-bearing, not decoration. An unguarded sweep would delete a
        CONCURRENT run's in-flight staging dir, and that run's parse_extraction_artifact
        would then raise FileNotFoundError -- an OSError outside run_extract_skeleton's
        ParseError handler, i.e. a traceback where the shipped code exits cleanly. A live
        staging dir is milliseconds old; an abandoned one is minutes."""
        self.assertEqual(self._staged_run([0]), [R._STAGING_PREFIX + "age0"])

    def test_skeleton_output_is_deterministic(self):
        """Pinned to a hand-typed document, not to a second call of itself. `a == b` in one
        process is true of every implementation, including one that emits nothing."""
        _, _, art = self._write()
        self.assertEqual(
            art,
            {
                "schema-version": 1,
                "stem": "rfc9999",
                "register": "rfc2119",
                "source-path": "rfc/full/rfc9999.txt",
                "source-sha": _sha(_SRC_TWO_SITES),
                "signed-off": "",
                "reviewer": "",
                "sections": [
                    {"id": "front", "sites": 0, "disposition": None},
                    {"id": "1", "sites": 0, "disposition": None},
                    {"id": "2", "sites": 2, "disposition": None},
                    {"id": "3", "sites": 0, "disposition": None},
                ],
                "sites": [
                    {"id": "2:1", "quote": _Q1, "disposition": None},
                    {"id": "2:2", "quote": _Q2, "disposition": None},
                ],
            },
        )

    def test_a_skeleton_the_parser_would_refuse_is_never_written(self):
        """The writer round-trips its own output through parse_extraction_artifact before
        the file lands.

        Without it `make ze-rfc-extraction-create STEM=rfc2865` exited 0 announcing success while
        writing a file that could not be re-read -- and one such committed file makes every
        later `--check` print 'cannot run', hiding EVERY other RFC violation in the repo.
        A guard that neither denies nor speaks does not exist
        (ai/rules/evidence.md)."""
        broken = _artifact()
        broken["sections"] = broken["sections"] + [broken["sections"][1]]
        with _patched(_artifact_document=lambda inv, previous: broken):
            code, out, path, on_disk = self._write_raw()
        self.assertEqual(code, 2, out)
        self.assertIn("cannot run", out)
        self.assertIn("duplicate section", out)
        self.assertFalse(os.path.exists(path), "a refused skeleton must not be written")
        self.assertIsNone(on_disk)

    def test_a_refused_skeleton_leaves_the_previous_artifact_intact(self):
        """ai/rules/never-destroy-work.md: failing closed must not take the reviewer's
        existing sign-off down with it."""
        broken = _artifact()
        broken["sections"] = broken["sections"] + [broken["sections"][1]]
        with _patched(_artifact_document=lambda inv, previous: broken):
            code, out, _, on_disk = self._write_raw(existing=_artifact())
        self.assertEqual(code, 2, out)
        self.assertEqual(on_disk, _artifact())


class TestExtractionSignoff(unittest.TestCase):
    """The two arithmetics. FORWARD catches a MISSED obligation (every derived site is
    classified); REVERSE catches an INVENTED one (every gated requirement is backed by a
    site or declared unsourced)."""

    def _errs(self, art, reqs=None, src=_SRC_TWO_SITES, stem="rfc9999"):
        reqs = _reqs_9999() if reqs is None else reqs
        with _extraction_tree(artifacts={stem: art}, src={stem: src}):
            return R.check_extraction_signoff(reqs)

    def test_fully_classified_signoff_passes(self):
        """The discriminating twin for everything below: proves the check is not
        'always fails'."""
        self.assertEqual(self._errs(_artifact()), [])

    def test_unclassified_site_fails(self):
        """AC-3: naming the locator AND its quote, so the reviewer knows what to read."""
        art = _artifact()
        art["sites"][0] = _site("2:1", _Q1)
        errs = self._errs(art)
        joined = " ".join(errs)
        self.assertIn("2:1", joined)
        self.assertIn("first thing", joined)

    def test_unclassified_section_fails(self):
        art = _artifact()
        art["sections"][2] = _section("2", 2)
        self.assertTrue(any("section 2" in e for e in self._errs(art)))

    def test_source_sha_mismatch_fails(self):
        """AC-4: over-trigger bias, exactly as `verdict_freshness` records -- a false
        stale costs a re-read, a false fresh ships an unbounded summary."""
        errs = self._errs(_artifact(**{"source-sha": "0" * 16}))
        self.assertTrue(any("source-sha" in e or "source text" in e for e in errs))

    def test_source_sha_mismatch_reports_once_not_per_site(self):
        """One accurate error, not a wall: with the source moved, every site would
        mismatch too, and the useful message is the first one."""
        errs = self._errs(_artifact(**{"source-sha": "0" * 16}))
        self.assertEqual(len(errs), 1, errs)

    def test_mapped_to_unknown_requirement_fails(self):
        """AC-5."""
        art = _artifact()
        art["sites"][0]["mapped-to"] = "RFC9999-2-99"
        self.assertTrue(any("RFC9999-2-99" in e for e in self._errs(art)))

    def test_gated_requirement_with_no_site_fails(self):
        """AC-6, the reverse arithmetic: the summary asserts an obligation no source site
        backs. rfc/enrolled.txt's own header records that one was found once; nothing
        looks for the next."""
        errs = self._errs(_artifact(), reqs=_reqs_9999(3))
        self.assertTrue(any("RFC9999-2-3" in e for e in errs), errs)

    def test_unsourced_id_recorded_on_a_section_passes(self):
        """AC-6's escape for indicative prose (the RFC 4271 §8.2.2 shape, R-7): an
        obligation read from prose with no keyword site behind it, attributed to the
        section it was read from.

        Signed under `prose`, and that is not incidental: declaring a third gated
        requirement over a two-site source trips the undercount clause, so the derivation
        demotes the register. An RFC that needs unsourced-ids is by construction an RFC
        whose keyword inventory cannot bound its summary."""
        art = _artifact(register="prose")
        art["sections"][2]["unsourced-ids"] = ["RFC9999-2-3"]
        self.assertEqual(self._errs(art, reqs=_reqs_9999(3)), [])

    def test_unsourced_ids_naming_an_unknown_requirement_fails(self):
        """The negative twin the unsourced-ids arm never had: only the PASSING direction
        was covered, so `for u in sorted(())` -- the arm never firing -- survived.

        unsourced-ids is the one escape from the reverse arithmetic: it lets a reviewer say
        'this row was read from indicative prose, no keyword site backs it'. An unchecked
        escape is a free pass, since any string typed there would silence AC-6 for a
        requirement id that does not exist."""
        art = _artifact(register="prose")
        art["sections"][2]["unsourced-ids"] = ["RFC9999-2-3", "RFC9999-9-9"]
        errs = self._errs(art, reqs=_reqs_9999(3))
        self.assertTrue(any("RFC9999-9-9" in e for e in errs), errs)
        self.assertTrue(
            all("RFC9999-2-3" not in e for e in errs),
            "the id that DOES exist must not be accused",
        )

    def test_an_unsigned_skeleton_is_refused_by_the_check(self):
        """The other half of the parse/check split: parsing tolerates an unsigned
        skeleton, the CHECK does not."""
        errs = self._errs(_artifact(reviewer="", **{"signed-off": ""}))
        joined = " ".join(errs)
        self.assertIn("signed-off", joined)
        self.assertIn("reviewer", joined)

    def test_manual_walk_without_a_register_reason_is_refused(self):
        """AC-10: a manual walk is an assertion the gate cannot verify, so it must at
        least state what it rests on."""
        art = _artifact(src=_SRC_NO_INVENTORY, register="manual-walk")
        art["sites"] = []
        art["sections"] = [
            _section("front", 0, "walked"),
            _section("1", 0, "walked", **{"unsourced-ids": ["RFC9999-2-1"]}),
            _section("2", 0, "walked", **{"unsourced-ids": ["RFC9999-2-2"]}),
        ]
        errs = self._errs(art, src=_SRC_NO_INVENTORY)
        self.assertTrue(any("register-reason" in e for e in errs), errs)

    def test_a_must_bearing_site_mapped_to_an_advisory_row_fails(self):
        """The gate held both facts and compared neither.

        `_evaluate_extraction` checked only that `mapped-to` names a row that EXISTS, and
        `known_ids` holds every level. So a site quoting a capitalised MUST could be mapped
        to a SHOULD row and reported as captured, while `evaluate()` never gates a SHOULD --
        the obligation is recorded as bound and is proven by nothing. That is the RFC's
        own MUST silently downgraded to advice, which is a compliance defect, not
        bookkeeping (ai/rules/rfc-compliance.md)."""
        reqs = [
            _req("RFC9999-2-1", level="SHOULD", rfc="rfc9999"),
            _req("RFC9999-2-2", rfc="rfc9999"),
        ]
        errs = self._errs(_artifact(), reqs=reqs)
        joined = " ".join(errs)
        self.assertIn("2:1", joined)
        self.assertIn("RFC9999-2-1", joined)
        self.assertIn("SHOULD", joined)

    def test_a_must_bearing_site_mapped_to_a_gated_row_passes(self):
        """The discriminating twin: mapping a MUST site to a MUST row is the normal case
        and must stay silent."""
        self.assertEqual(self._errs(_artifact()), [])

    def test_an_advisory_site_may_map_to_an_advisory_row(self):
        """Only a CAPITALISED MUST-level keyword triggers the level comparison. A prose
        site says nothing about its level, so demanding a gated target there would refuse
        the `prose` register's ordinary output."""
        src = _SRC_PROSE_ONLY
        art = _artifact(src=src, register="prose")
        art["sections"] = [
            _section("front", 0, "walked"),
            _section("1", 0, "walked"),
            _section("2", 2, "walked"),
        ]
        art["sites"] = [
            _site(
                "2:1",
                "A speaker must do the first thing.",
                "mapped",
                **{"mapped-to": "RFC9999-2-1"},
            ),
            _site(
                "2:2",
                "A speaker shall not do the second thing.",
                "mapped",
                **{"mapped-to": "RFC9999-2-2"},
            ),
        ]
        reqs = [
            _req("RFC9999-2-1", level="SHOULD", rfc="rfc9999"),
            _req("RFC9999-2-2", level="MAY", rfc="rfc9999"),
        ]
        self.assertEqual(self._errs(art, reqs=reqs, src=src), [])

    def test_advisory_requirement_needs_no_site(self):
        """SHOULD/MAY are outside the inventory (GATED_LEVELS:69), so the reverse
        arithmetic must not demand a site for one."""
        reqs = _reqs_9999() + [_req("RFC9999-2-3", level="SHOULD", rfc="rfc9999")]
        self.assertEqual(self._errs(_artifact(), reqs=reqs), [])

    def test_hand_edited_quote_fails(self):
        """AC-20, R-9: every derived field is re-derived and compared. A mismatch names
        the locator; it is never silently overwritten."""
        art = _artifact()
        art["sites"][0]["quote"] = "A speaker MUST do whatever it likes."
        self.assertTrue(any("2:1" in e for e in self._errs(art)))

    def test_hand_edited_section_count_fails(self):
        """AC-20 on the section axis."""
        art = _artifact()
        art["sections"][2]["sites"] = 1
        self.assertTrue(any("section 2" in e for e in self._errs(art)))

    def test_a_site_missing_from_the_artifact_fails(self):
        """A site the reviewer never saw -- or one that appeared because the source text
        changed -- must red the gate, not vanish."""
        art = _artifact()
        art["sites"] = art["sites"][:1]
        self.assertTrue(any("2:2" in e for e in self._errs(art)))

    def test_an_invented_site_fails(self):
        art = _artifact()
        art["sites"].append(
            _site(
                "9:1",
                "invented",
                "excluded",
                **{"excluded-kind": "not-a-requirement", "reason": "x"},
            )
        )
        self.assertTrue(any("9:1" in e for e in self._errs(art)))

    def test_duplicate_of_must_name_a_mapped_id(self):
        """AC-8, R-1: a chain of duplicates cannot cover an RFC in which nothing is
        actually mapped."""
        art = _artifact()
        art["sites"] = [
            _site(
                "2:1",
                _Q1,
                "excluded",
                **{
                    "excluded-kind": "duplicate-of",
                    "mapped-to": "RFC9999-2-2",
                    "reason": "restates the other",
                },
            ),
            _site(
                "2:2",
                _Q2,
                "excluded",
                **{
                    "excluded-kind": "duplicate-of",
                    "mapped-to": "RFC9999-2-1",
                    "reason": "restates the other",
                },
            ),
        ]
        errs = self._errs(art)
        self.assertTrue(any("duplicate-of" in e for e in errs), errs)

    def test_duplicate_of_needs_the_id_it_duplicates(self):
        """A duplicate that names nothing cannot be checked against anything."""
        art = _artifact()
        art["sites"][1] = _site(
            "2:2",
            _Q2,
            "excluded",
            **{"excluded-kind": "duplicate-of", "reason": "restates the other"},
        )
        with self.assertRaises(R.ParseError):
            self._errs(art)

    def test_duplicate_of_passes_when_another_site_maps_the_id(self):
        """The twin: a genuine duplicate beside a genuine mapping is legal."""
        art = _artifact()
        art["sites"][1] = _site(
            "2:2",
            _Q2,
            "excluded",
            **{
                "excluded-kind": "duplicate-of",
                "mapped-to": "RFC9999-2-1",
                "reason": "restates RFC9999-2-1",
            },
        )
        art["sections"][2]["unsourced-ids"] = ["RFC9999-2-2"]
        self.assertEqual(self._errs(art), [])

    def test_a_contradicted_artifact_is_not_in_the_signed_set(self):
        """Only a ZERO-violation artifact counts as signed.

        `evaluate_extractions` returns (signed, violations) and the filter that keeps them
        apart had no test at all: deleting `if not found:` left 252/252 green. Without it
        an all-unclassified skeleton is 'signed', which satisfies AC-1's new-enrolment
        precondition and earns drain credit -- R-2's failure exactly, and generation
        producing a pass is the one thing this design forbids structurally."""
        art = _artifact()
        art["sites"][0] = _site("2:1", _Q1)  # UNCLASSIFIED
        with _extraction_tree(
            artifacts={"rfc9999": art}, src={"rfc9999": _SRC_TWO_SITES}
        ):
            signed, errs = R.evaluate_extractions(_reqs_9999())
        self.assertTrue(errs, "an unclassified site must be a violation")
        self.assertNotIn("rfc9999", signed)

        # And the consequence that matters: the enrolment precondition still fires, so a
        # contradicted artifact cannot buy an enrolment.
        enrol = R.check_enrolment(
            current={"rfc9999"},
            baseline=set(),
            summaries={"rfc9999"},
            newly_enrolled={"rfc9999"},
            signed=set(signed),
        )
        self.assertTrue(
            any("rfc9999" in e and "sign-off" in e for e in enrol),
            f"a contradicted sign-off must not satisfy the enrolment precondition: {enrol}",
        )

    def test_a_clean_artifact_is_in_the_signed_set(self):
        """The discriminating twin: the filter must not reject everything."""
        with _extraction_tree(
            artifacts={"rfc9999": _artifact()}, src={"rfc9999": _SRC_TWO_SITES}
        ):
            signed, errs = R.evaluate_extractions(_reqs_9999())
        self.assertEqual(errs, [])
        self.assertIn("rfc9999", signed)

    def test_missing_source_text_for_a_signed_stem_fails(self):
        """Fail closed: an artifact whose source vanished can no longer be re-derived, so
        the bound it claims cannot be re-checked."""
        with _extraction_tree(artifacts={"rfc9999": _artifact()}, src={}):
            errs = R.check_extraction_signoff(_reqs_9999())
        self.assertTrue(any("no source text" in e for e in errs), errs)


# The phrase only the live-prose arm of `_relocation_errors` emits. The other
# denying arms say "does not exist or cannot be read" and "still declares", so
# asserting this one distinguishes them.
_LIVE_PROSE_ARM = "in live prose"


class TestRelocatedToSpec(unittest.TestCase):
    """The sixth exclusion kind, and the one that does not say "this binds nobody".

    The other five dismiss a sentence. `relocated-to-spec` says the opposite: the
    obligation IS owed, by a named spec, under a reserved id, and it is not in this
    summary because an owner ruling moved it there. Two independent agents refused to
    force `binds-another-role` onto the twelve RFC 7296 sites of owner ruling D-1
    (2026-07-31), and they were right to: that kind asserts Ze plays no such role, while
    two specs exist to implement exactly those roles.

    So the kind is a POINTER, and a pointer with no tripwire is a shrug with a longer
    name. The parse-time shape refusals live in TestExtractionArtifact; this class is the
    tripwire, the ratchet and the published counts.
    """

    def _errs(self, art, reqs=None, specs=None, src=_SRC_TWO_SITES, stem="rfc9999"):
        reqs = _reqs_9999(1) if reqs is None else reqs
        with (
            _spec_tree(specs),
            _extraction_tree(artifacts={stem: art}, src={stem: src}),
        ):
            return R.check_extraction_signoff(reqs)

    def test_a_relocation_whose_spec_carries_the_id_is_accepted(self):
        """The discriminating twin. Without it every refusal below is satisfied by a
        check that refuses everything."""
        self.assertEqual(self._errs(_relocated_artifact()), [])

    def test_a_relocation_whose_spec_does_not_exist_is_refused(self):
        """Delete the destination and the gate goes red naming the site. This is the
        whole reason the kind is not a shrug: an obligation cannot be parked by pointing
        at a document that nobody has to keep."""
        errs = self._errs(_relocated_artifact(), specs={})
        joined = " ".join(errs)
        self.assertIn("2:2", joined)
        self.assertIn(_SPEC_REL, joined)
        self.assertIn("RFC9999-2-3", joined)
        # The ARM, not merely the redness. Both arms name the site, the path and the id,
        # so an assertion over those three survives the existence arm being deleted: the
        # id search then fails against an empty read and reports the other message.
        self.assertIn("does not exist", joined)

    def test_a_relocation_whose_spec_dropped_the_id_is_refused(self):
        """The likelier failure, and the one a file-existence check alone would miss: the
        spec survives, the row is edited out of it, and the obligation is owed by nobody
        while both documents look healthy."""
        errs = self._errs(
            _relocated_artifact(),
            specs={_SPEC_NAME: "# Fixture spec\n\nNothing reserved here.\n"},
        )
        joined = " ".join(errs)
        self.assertIn("2:2", joined)
        self.assertIn("RFC9999-2-3", joined)
        self.assertIn("no longer names", joined)

    def test_an_empty_destination_spec_is_refused(self):
        """A file that exists and says nothing reserves nothing. Present-but-empty passes
        every existence test there is, which is the shape ai/rules/evidence.md
        records as passing `ok` while being unusable."""
        errs = self._errs(_relocated_artifact(), specs={_SPEC_NAME: ""})
        self.assertTrue(any("2:2" in e and "no longer names" in e for e in errs), errs)

    def test_a_longer_id_does_not_satisfy_the_reservation(self):
        """`RFC9999-2-30` is not `RFC9999-2-3`. A substring test would let the wrong row
        keep the tripwire green, and it is the neighbouring ordinal that a renumbering
        actually produces."""
        errs = self._errs(
            _relocated_artifact(),
            specs={_SPEC_NAME: "| `RFC9999-2-30` | MUST | a different obligation |\n"},
        )
        self.assertTrue(any("2:2" in e for e in errs), errs)

    # ------------------------------------------------------------------
    # The id must appear in LIVE prose. Every arm below is a file that still contains
    # the characters and has stopped claiming the work, which is the one direction the
    # raw-text search failed OPEN in: finding the id is what PASSES, so text that no
    # longer binds anybody kept an obligation owed by nobody looking homed.
    #
    # Each case asserts _LIVE_PROSE_ARM, the phrase only THIS arm emits. Asserting
    # `"2:2" in e and "RFC9999-2-3" in e` instead was vacuous: every denying arm
    # embeds the site id and the requirement id, so a stub returning the
    # spec-is-missing message satisfied all four while the feature was dead.
    # ------------------------------------------------------------------

    def test_a_commented_out_reservation_is_refused(self):
        """An HTML comment renders as nothing. A row parked inside one is a row the spec
        has stopped owing, and it is the cheapest way to make this tripwire green without
        keeping the obligation."""
        errs = self._errs(
            _relocated_artifact(),
            specs={
                _SPEC_NAME: (
                    "# Fixture spec\n\n"
                    "<!-- | `RFC9999-2-3` | MUST | parked while we think -->\n"
                )
            },
        )
        self.assertTrue(any(_LIVE_PROSE_ARM in e for e in errs), errs)

    def test_a_struck_through_reservation_is_refused(self):
        """Strikethrough is this repository's own notation for superseded spec content
        (ai/rules/planning.md, append-only editing). A struck row is a row that was
        withdrawn, so reading it as a live reservation inverts the notation's meaning."""
        errs = self._errs(
            _relocated_artifact(),
            specs={
                _SPEC_NAME: "# Fixture spec\n\n~~| `RFC9999-2-3` | MUST | dropped~~\n"
            },
        )
        self.assertTrue(any(_LIVE_PROSE_ARM in e for e in errs), errs)

    def test_an_id_only_inside_a_code_fence_is_refused(self):
        """A fenced block is an example, not a claim. Specs in this tree carry shell
        snippets that name ids (`grep -o 'RFC7296-2\\.22-[0-9]*'`), so a search that
        counts them would let a spec reserve an obligation by quoting a command."""
        errs = self._errs(
            _relocated_artifact(),
            specs={
                _SPEC_NAME: "# Fixture spec\n\n```\ngrep RFC9999-2-3 rfc/short/\n```\n"
            },
        )
        self.assertTrue(any(_LIVE_PROSE_ARM in e for e in errs), errs)

    def test_an_unterminated_comment_hides_the_reservation(self):
        """The ambiguous input, resolved toward RED. A Markdown renderer swallows
        everything after an unclosed `<!--`, and a reader of the rendered spec sees no
        reservation. Stripping to end of file agrees with them, and it puts the one
        undecidable case on the side that costs a look rather than an obligation."""
        errs = self._errs(
            _relocated_artifact(),
            specs={
                _SPEC_NAME: "# Fixture spec\n\n<!-- notes\n\n| `RFC9999-2-3` | MUST | owed |\n"
            },
        )
        self.assertTrue(any(_LIVE_PROSE_ARM in e for e in errs), errs)

    def test_an_id_only_inside_a_tilde_fence_is_refused(self):
        """CommonMark accepts `~~~` as a fence delimiter as readily as ```` ``` ````, and
        covering backticks alone left this OPEN in a way that also broke the strike rule:
        `~~~` starts with `~~`, so the strike pass ate the two delimiter LINES and
        published the block body as live prose."""
        errs = self._errs(
            _relocated_artifact(),
            specs={
                _SPEC_NAME: "# Fixture spec\n\n~~~\ngrep RFC9999-2-3 rfc/short/\n~~~\n"
            },
        )
        self.assertTrue(any(_LIVE_PROSE_ARM in e for e in errs), errs)

    def test_an_id_only_inside_an_indented_code_block_is_refused(self):
        """The form both live destination specs actually use. `plan/spec-ipsec-ipcomp.md`
        carries eleven four-space-indented shell lines and `plan/spec-ipsec-remote-access.md`
        one, so this is the reachable shape of "reserving an obligation by example"."""
        errs = self._errs(
            _relocated_artifact(),
            specs={
                _SPEC_NAME: "# Fixture spec\n\nRun:\n\n    grep RFC9999-2-3 rfc/short/\n"
            },
        )
        self.assertTrue(any(_LIVE_PROSE_ARM in e for e in errs), errs)

    def test_an_unterminated_fence_hides_the_reservation(self):
        """The fence fallback, pinned. An opener with no closer swallows the rest of the
        file for a renderer, so it does here too. Without this case, deleting the
        `|^ {0,3}(?:...)` branch left every other case green."""
        errs = self._errs(
            _relocated_artifact(),
            specs={
                _SPEC_NAME: "# Fixture spec\n\n```\nnotes\n\n| `RFC9999-2-3` | MUST | owed |\n"
            },
        )
        self.assertTrue(any(_LIVE_PROSE_ARM in e for e in errs), errs)

    def test_an_unterminated_strike_hides_the_rest_of_its_line(self):
        """The strike fallback, pinned, and scoped to ONE line: strikethrough does not
        span lines, so an unclosed `~~` must not swallow the file. Without this case,
        deleting the `|~~.*$` branch left every other case green.

        The id sits on the `~~` line and ANOTHER line follows it. With the id on the
        last line, `$` matches whether or not the pattern carries `re.M`, so dropping
        that flag -- which would make the fallback swallow the rest of the FILE --
        killed nothing. The trailing line is what makes the scope observable.
        """
        errs = self._errs(
            _relocated_artifact(),
            specs={
                _SPEC_NAME: "# Fixture spec\n\n~~withdrawn: `RFC9999-2-3`\nand the spec continues here.\n"
            },
        )
        self.assertTrue(any(_LIVE_PROSE_ARM in e for e in errs), errs)

    def test_an_id_after_a_closed_fence_still_reserves(self):
        """The accept-twin for the fence's PAIRED branch. Without it, replacing the
        pattern with its fallback-only form killed nothing, while a live row after a
        closed fence silently stopped reserving: the opener would have swallowed the
        rest of the file."""
        self.assertEqual(
            self._errs(
                _relocated_artifact(),
                specs={
                    _SPEC_NAME: (
                        "# Fixture spec\n\n```\ngrep something\n```\n\n"
                        "| `RFC9999-2-3` | MUST | owed |\n"
                    )
                },
            ),
            [],
        )

    def test_an_id_after_a_closed_comment_still_reserves(self):
        """The same twin for the comment's paired branch."""
        self.assertEqual(
            self._errs(
                _relocated_artifact(),
                specs={
                    _SPEC_NAME: (
                        "# Fixture spec\n\n<!-- an aside -->\n\n"
                        "| `RFC9999-2-3` | MUST | owed |\n"
                    )
                },
            ),
            [],
        )

    def test_an_id_only_inside_a_tab_indented_block_is_refused(self):
        """The `\\t` branch of the indented-code rule. A tab is a code block to a
        renderer exactly as four spaces are, and pinning only the spaces branch left
        the tab one able to reserve an obligation by example."""
        errs = self._errs(
            _relocated_artifact(),
            specs={
                _SPEC_NAME: "# Fixture spec\n\nRun:\n\n\tgrep RFC9999-2-3 rfc/short/\n"
            },
        )
        self.assertTrue(any(_LIVE_PROSE_ARM in e for e in errs), errs)

    def test_repeated_constructs_are_all_stripped(self):
        """`sub` replaces every occurrence, never the first. `plan/spec-ipsec-remote-access.md`
        carries six strikethrough lines, so a `count=1` on any of the three passes would
        leave the later ones live while the earlier ones read as handled."""
        errs = self._errs(
            _relocated_artifact(),
            specs={
                _SPEC_NAME: (
                    "# Fixture spec\n\n"
                    "~~first withdrawal~~\n"
                    "~~second withdrawal, `RFC9999-2-3`~~\n"
                )
            },
        )
        self.assertTrue(any(_LIVE_PROSE_ARM in e for e in errs), errs)

    def test_a_live_row_beside_a_struck_one_is_accepted(self):
        """The discriminating twin for all four refusals above. Without it, a strip that
        deleted the whole file would pass every one of them while breaking the twelve live
        relocations this kind exists to carry."""
        self.assertEqual(
            self._errs(
                _relocated_artifact(),
                specs={
                    _SPEC_NAME: (
                        "# Fixture spec\n\n"
                        "~~| `RFC9999-2-3` | MUST | first attempt, withdrawn~~\n"
                        "| `RFC9999-2-3` | MUST | re-stated, and owed |\n"
                    )
                },
            ),
            [],
        )

    def test_a_reserved_id_the_summary_still_declares_is_refused(self):
        """A relocation ASSERTS the row left rfc/short/. If the summary still declares it,
        the site is a mapping wearing the wrong kind, and the obligation would be counted
        as homed elsewhere while it is also claimed here."""
        errs = self._errs(_relocated_artifact(), reqs=_reqs_9999(3))
        self.assertTrue(
            any("2:2" in e and "RFC9999-2-3" in e for e in errs),
            errs,
        )

    def test_a_relocation_is_not_in_the_signed_set_until_it_resolves(self):
        """The consequence that matters: a broken pointer costs the stem its sign-off, so
        it stops earning drain credit and cannot satisfy the enrolment precondition."""
        with (
            _spec_tree({}),
            _extraction_tree(
                artifacts={"rfc9999": _relocated_artifact()},
                src={"rfc9999": _SRC_TWO_SITES},
            ),
        ):
            signed, errs = R.evaluate_extractions(_reqs_9999(1))
        self.assertTrue(errs)
        self.assertNotIn("rfc9999", signed)

    # ------------------------------------------------------------------
    # Decision 1: the exclusion count. A relocation IS an exclusion, so it counts in the
    # ratchet's number and in the published ratio -- and it is ALSO published apart, so a
    # reviewer can tell a homed obligation from a dismissed sentence.
    # ------------------------------------------------------------------

    def test_a_relocation_counts_toward_the_ratchet(self):
        """One definition of "exclusion" on both sides. `_git_baseline_extractions` counts
        `disposition == "excluded"` in a HEAD blob and never reads the kind, so netting
        relocations out of the working-tree number would make relocation the cheap route
        past the resign-reason gate, and would split the comparison across two parsers."""
        baseline = {
            "rfc9999": R.BaselineExtraction(
                excluded=0, signed_off="2026-07-29", resign_reason=""
            )
        }
        with (
            _spec_tree(),
            _extraction_tree(
                artifacts={"rfc9999": _relocated_artifact()},
                src={"rfc9999": _SRC_TWO_SITES},
                baseline=baseline,
            ),
        ):
            errs = R.check_extraction_ratchet()
        self.assertTrue(any("exclusions rose from 0 to 1" in e for e in errs), errs)

    def test_a_resigned_relocation_passes_the_ratchet(self):
        """The twin: the rise is legal once the walk is redone and says so."""
        baseline = {
            "rfc9999": R.BaselineExtraction(
                excluded=0, signed_off="2026-07-29", resign_reason=""
            )
        }
        art = _relocated_artifact()
        art["signed-off"] = "2026-08-02"
        art["resign-reason"] = "owner ruling D-1 moved the obligation to a named spec"
        with (
            _spec_tree(),
            _extraction_tree(
                artifacts={"rfc9999": art},
                src={"rfc9999": _SRC_TWO_SITES},
                baseline=baseline,
            ),
        ):
            self.assertEqual(R.check_extraction_ratchet(), [])

    def test_the_published_ratio_counts_a_relocation_and_names_it_apart(self):
        """The ratio keeps its reach: 1 of 2 sites unmapped is 0.50 whether the walk
        dismissed the sentence or homed it. Netting relocations out would let a walk that
        relocated everything publish a pristine 0.00, which is the exact shape the ratio
        exists to make visible. The separate count is what stops the two being
        confused."""
        with (
            _spec_tree(),
            _extraction_tree(
                artifacts={"rfc9999": _relocated_artifact()},
                src={"rfc9999": _SRC_TWO_SITES},
            ),
        ):
            body = "\n".join(R.render_extraction_table(_reqs_9999(1), {"rfc9999"}))
        self.assertIn("| 2 | 1 | 1 | 0.50 |", body)
        self.assertIn("relocated-to-spec", body)
        self.assertIn("`rfc9999` 1", body)

    def test_a_corpus_with_no_relocation_publishes_no_relocation_line(self):
        """Derived, so it appears exactly when the fact does. Its absence is a true
        statement about the corpus, and it keeps ai/RFC-REQUIREMENTS.md byte-identical
        until the first relocation actually lands."""
        with _extraction_tree(
            artifacts={"rfc9999": _artifact()}, src={"rfc9999": _SRC_TWO_SITES}
        ):
            body = "\n".join(R.render_extraction_table(_reqs_9999(), {"rfc9999"}))
        self.assertNotIn("relocated", body.lower())

    def test_the_envelope_publishes_the_relocated_count(self):
        """`make ze-rfc-extraction-status` is the machine-readable half. A count that only
        a human can read is not one the drain policy can ever consume."""
        with (
            _spec_tree(),
            _extraction_tree(
                artifacts={"rfc9999": _relocated_artifact()},
                src={"rfc9999": _SRC_TWO_SITES},
            ),
        ):
            env = R.extraction_status(_reqs_9999(1), {"rfc9999"})
        self.assertEqual(env["relocated"], 1)
        self.assertEqual(env["signed"], 1)
        for key in env:
            self.assertEqual(key, key.lower(), key)
            self.assertNotIn("_", key)

    def test_the_envelope_reports_zero_rather_than_nothing(self):
        """A missing key reads as "not a thing", not as "zero" -- the same reason every
        register is present in the split even at zero."""
        with _extraction_tree(
            artifacts={"rfc9999": _artifact()}, src={"rfc9999": _SRC_TWO_SITES}
        ):
            env = R.extraction_status(_reqs_9999(), {"rfc9999"})
        self.assertEqual(env["relocated"], 0)


class TestPre2119FailsClosed(unittest.TestCase):
    """R-4, the fail-open this spec exists to close: a keyword-driven check reports
    '0 sites, all classified' for 23 enrolled RFCs holding 172 gated MUSTs. A guard that
    cannot deny must SAY so (ai/rules/evidence.md).

    Both figures are at the SITE denominator -- `derive_inventory(stem, gated).keyword_sites
    == 0`, which is the oracle this check actually uses. There is a second, narrower
    denominator that is easy to mix with it: `source_keyword_count(stem) == 0` counts
    keyword OCCURRENCES, and gives 22 stems / 164 gated MUSTs. The whole 8-MUST difference
    is rfc5443, whose four uppercase occurrences all sit inside its own RFC 2119 boilerplate
    paragraph -- _BOILERPLATE_RE excludes that sentence from the site scan, so it has
    occurrences but no sites. Quoting 23 stems beside 164 MUSTs mixes the two. Re-measured
    2026-07-29, stable over three runs (scripts/dev/rfc_requirements.py unchanged since)."""

    def _errs(self, art, reqs, src):
        with _extraction_tree(artifacts={"rfc9999": art}, src={"rfc9999": src}):
            return R.check_extraction_signoff(reqs)

    def test_keyword_free_source_cannot_claim_rfc2119(self):
        """AC-9: the register is a property of the SOURCE, not a claim the signer may
        assert. The 23 keyword-free RFCs are exactly the population that would benefit
        from claiming the strong grade."""
        art = _artifact(src=_SRC_PROSE_ONLY, register="rfc2119")
        errs = self._errs(art, _reqs_9999(), _SRC_PROSE_ONLY)
        self.assertTrue(any("register" in e for e in errs), errs)

    def test_rfc2119_register_below_source_count_is_rejected(self):
        """AC-32, the undercount arm (the rfc2181 shape): the source HAS capitalised
        keywords, but fewer sites than the summary declares gated rows, so the
        derivation grades it `prose` and the rfc2119 claim is stronger than that."""
        art = _artifact(register="rfc2119")
        errs = self._errs(art, _reqs_9999(23), _SRC_TWO_SITES)
        self.assertTrue(any("register" in e for e in errs), errs)

    def test_a_weaker_claim_than_the_derivation_is_allowed(self):
        """The other direction is legal and recorded: an artifact may sign under the
        derived register or a WEAKER one, never a stronger one."""
        art = _artifact(register="prose")
        self.assertEqual(self._errs(art, _reqs_9999(), _SRC_TWO_SITES), [])

    def test_empty_inventory_with_gated_musts_is_refused(self):
        """AC-10: the gate says it cannot evaluate rather than passing. With no site to
        map, every gated requirement must be declared unsourced on a walked section --
        which is a reviewer's assertion, made explicitly, not a silent zero."""
        art = _artifact(
            src=_SRC_NO_INVENTORY,
            register="manual-walk",
            **{"register-reason": "no capitalised keyword and no lowercase modal"},
        )
        art["sites"] = []
        art["sections"] = [
            _section(
                "front",
                0,
                "skipped",
                **{"skip-kind": "front-matter", "reason": "title"},
            ),
            _section("1", 0, "walked"),
            _section("2", 0, "walked"),
        ]
        errs = self._errs(art, _reqs_9999(), _SRC_NO_INVENTORY)
        self.assertTrue(
            errs, "an empty inventory with gated MUSTs must not pass silently"
        )
        self.assertTrue(any("RFC9999-2-1" in e for e in errs), errs)

    def test_manual_walk_passes_when_every_gated_must_is_declared_unsourced(self):
        """The twin, and owner ruling 1's premise: an RFC whose own authors wrote no
        RFC 2119 keywords must have SOME route out of the backlog."""
        art = _artifact(
            src=_SRC_NO_INVENTORY,
            register="manual-walk",
            **{"register-reason": "no capitalised keyword and no lowercase modal"},
        )
        art["sites"] = []
        art["sections"] = [
            _section(
                "front",
                0,
                "skipped",
                **{"skip-kind": "front-matter", "reason": "title"},
            ),
            _section("1", 0, "walked", **{"unsourced-ids": ["RFC9999-2-1"]}),
            _section("2", 0, "walked", **{"unsourced-ids": ["RFC9999-2-2"]}),
        ]
        self.assertEqual(self._errs(art, _reqs_9999(), _SRC_NO_INVENTORY), [])


class TestExtractionRatchet(unittest.TestCase):
    """Two ratchets keep the bound moving in one direction only: the signed set may not
    shrink (AC-12), and a signed stem's exclusion count may not rise without a recorded
    re-sign (AC-13, R-1). Both consume the baseline as `baseline - current`, so an empty
    baseline accuses nobody and a git failure judges nothing (AC-17) -- the OPPOSITE
    polarity from _git_baseline_summary_stems:763, restated rather than copied."""

    def _errs(self, artifacts, baseline):
        with _extraction_tree(artifacts=artifacts, src={"rfc9999": _SRC_TWO_SITES}):
            with _patched(_git_baseline_extractions=lambda: baseline):
                return R.check_extraction_ratchet()

    def test_signoff_count_is_monotonic(self):
        """AC-12: a stem signed at HEAD and unsigned now."""
        errs = self._errs({}, {"rfc9999": R.BaselineExtraction(0, "2026-07-29", "")})
        self.assertTrue(any("rfc9999" in e for e in errs), errs)

    def test_a_retained_signoff_passes(self):
        """The discriminating twin."""
        self.assertEqual(
            self._errs(
                {"rfc9999": _artifact()},
                {"rfc9999": R.BaselineExtraction(0, "2026-07-29", "")},
            ),
            [],
        )

    def test_a_new_signoff_is_not_a_violation(self):
        """A rise is the whole point."""
        self.assertEqual(self._errs({"rfc9999": _artifact()}, {}), [])

    def test_exclusions_are_shrink_only(self):
        """AC-13, R-1: without this the exclusion list becomes a 1600-slot escape hatch."""
        art = _artifact()
        art["sites"][1] = _site(
            "2:2",
            _Q2,
            "excluded",
            **{"excluded-kind": "not-a-requirement", "reason": "a table caption"},
        )
        errs = self._errs(
            {"rfc9999": art}, {"rfc9999": R.BaselineExtraction(0, "2026-07-29", "")}
        )
        self.assertTrue(any("exclusion" in e for e in errs), errs)

    def test_shrinking_exclusions_passes(self):
        """Any decrease is legal: the pressure is directional."""
        self.assertEqual(
            self._errs(
                {"rfc9999": _artifact()},
                {"rfc9999": R.BaselineExtraction(2, "2026-07-29", "")},
            ),
            [],
        )

    def test_resign_reason_permits_a_rise(self):
        """AC-13's twin: a recorded re-sign with a bumped date is the only way up."""
        art = _artifact(**{"resign-reason": "the source added a new obligation"})
        art["signed-off"] = "2026-08-15"
        art["sites"][1] = _site(
            "2:2",
            _Q2,
            "excluded",
            **{"excluded-kind": "not-a-requirement", "reason": "a table caption"},
        )
        self.assertEqual(
            self._errs(
                {"rfc9999": art}, {"rfc9999": R.BaselineExtraction(0, "2026-07-29", "")}
            ),
            [],
        )

    def test_a_carried_over_resign_reason_does_not_license_a_second_rise(self):
        """AC-13: the reason must be written for THIS walk.

        `_artifact_document` copies `resign-reason` forward on every refresh, so once set
        it is permanently non-empty and the only remaining guard on a rise is the date --
        and a date is one edit. Verified: exclusions 0 -> 2 with the reason unchanged from
        HEAD and the date bumped passed silently, which is the exclusion ratchet defeated
        by two keystrokes."""
        art = _artifact(**{"resign-reason": "the source added a new obligation"})
        art["signed-off"] = "2026-08-15"
        art["sites"][1] = _site(
            "2:2",
            _Q2,
            "excluded",
            **{"excluded-kind": "not-a-requirement", "reason": "a table caption"},
        )
        errs = self._errs(
            {"rfc9999": art},
            {
                "rfc9999": R.BaselineExtraction(
                    0, "2026-07-29", "the source added a new obligation"
                )
            },
        )
        self.assertTrue(errs, "a reason carried over from HEAD is not a new reason")
        self.assertIn("resign-reason", " ".join(errs))

    def test_a_freshly_written_resign_reason_permits_the_rise(self):
        """The discriminating twin: a reason that differs from the one at HEAD, with a
        bumped date, is the sanctioned way up and must stay silent."""
        art = _artifact(**{"resign-reason": "RFC 9999bis added two attribute rows"})
        art["signed-off"] = "2026-08-15"
        art["sites"][1] = _site(
            "2:2",
            _Q2,
            "excluded",
            **{"excluded-kind": "not-a-requirement", "reason": "a table caption"},
        )
        self.assertEqual(
            self._errs(
                {"rfc9999": art},
                {
                    "rfc9999": R.BaselineExtraction(
                        0, "2026-07-29", "the previous walk's reason"
                    )
                },
            ),
            [],
        )

    def test_resign_reason_without_a_bumped_date_still_fails(self):
        """A re-sign is a new walk. Reusing the old date says the walk did not happen."""
        art = _artifact(**{"resign-reason": "the source added a new obligation"})
        art["sites"][1] = _site(
            "2:2",
            _Q2,
            "excluded",
            **{"excluded-kind": "not-a-requirement", "reason": "a table caption"},
        )
        errs = self._errs(
            {"rfc9999": art}, {"rfc9999": R.BaselineExtraction(0, "2026-07-29", "")}
        )
        self.assertTrue(any("signed-off" in e for e in errs), errs)

    def test_git_failure_judges_nothing(self):
        """AC-17, driven through _FakeSubprocess with PLAUSIBLE non-empty output: an
        empty-stdout fake cannot tell a reader that checks returncode from one that does
        not, and both would pass on an implementation with no guard at all.

        `baseline=_LIVE_BASELINE` because this row's SUBJECT is the production HEAD reader:
        it must run and return None. Every other fixture test wants it replaced."""
        with _extraction_tree(src={"rfc9999": _SRC_TWO_SITES}, baseline=_LIVE_BASELINE):
            with _patched(
                subprocess=_FakeSubprocess(
                    returncode=128, stdout="rfc/extraction/rfc9999.json\0"
                )
            ):
                self.assertIsNone(R._git_baseline_extractions())
                self.assertEqual(R.check_extraction_ratchet(), [])

    def test_no_extraction_dir_at_head_accuses_nobody(self):
        """AC-17's other arm: an EMPTY baseline is the ordinary first-commit state and is
        safe at this polarity, unlike its sibling at _git_baseline_summary_stems:763."""
        self.assertEqual(self._errs({}, {}), [])


class TestEnrolmentSignoffPrecondition(unittest.TestCase):
    """AC-1 / AC-19: extraction sign-off is a precondition of a NEW enrolment only.
    Grandfathering is SCOPE, not an allowlist file (ai/rules/evidence.md)."""

    def test_new_enrolment_without_signoff_fails(self):
        errs = R.check_enrolment(
            current={"rfc7606", "rfc9999"},
            baseline={"rfc7606"},
            summaries={"rfc7606", "rfc9999"},
            newly_enrolled={"rfc9999"},
            signed=set(),
        )
        self.assertTrue(any("rfc9999" in e and "sign-off" in e for e in errs), errs)

    def test_new_enrolment_with_a_signoff_passes(self):
        errs = R.check_enrolment(
            current={"rfc7606", "rfc9999"},
            baseline={"rfc7606"},
            summaries={"rfc7606", "rfc9999"},
            newly_enrolled={"rfc9999"},
            signed={"rfc9999"},
        )
        self.assertEqual([e for e in errs if "sign-off" in e], [])

    def test_preexisting_enrolment_without_signoff_passes(self):
        """AC-19: the 166 stay green when the machinery lands. A rule that reds the gate
        on unrelated work gets removed rather than obeyed
        (ai/rules/rfc-compliance.md:114-116)."""
        errs = R.check_enrolment(
            current={"rfc7606"},
            baseline={"rfc7606"},
            summaries={"rfc7606"},
            newly_enrolled=set(),
            signed=set(),
        )
        self.assertEqual([e for e in errs if "sign-off" in e], [])

    def test_unknown_baseline_judges_nothing(self):
        """`current - baseline` against an empty baseline would accuse EVERY enrolled RFC
        of being new -- 166 false violations nobody can act on. 'I could not look' must
        not render as 'nothing was there'."""
        errs = R.check_enrolment(
            current={"rfc7606", "rfc9999"},
            baseline=set(),
            summaries={"rfc7606", "rfc9999"},
            newly_enrolled=None,
            signed=set(),
        )
        self.assertEqual([e for e in errs if "sign-off" in e], [])


def _signed_for(stem, src, register, reason=""):
    """A valid sign-off for `src`, built by DERIVING the inventory rather than typing it.

    Every site is excluded (the summary declares nothing gated), so the reverse arithmetic
    is trivially satisfied and the fixture isolates the register/counting behaviour.
    """
    with _patched(source_text=lambda s: src, source_path=lambda s: f"rfc/full/{s}.txt"):
        inv = R.derive_inventory(stem, 0)
    art = {
        "schema-version": 1,
        "stem": stem,
        "register": register,
        "source-path": inv.source_path,
        "source-sha": inv.source_sha,
        "signed-off": "2026-07-29",
        "reviewer": "tester",
        "sections": [
            {"id": s.id, "sites": s.sites, "disposition": "walked"}
            for s in inv.sections
        ],
        "sites": [
            {
                "id": s.id,
                "quote": s.quote,
                "disposition": "excluded",
                "excluded-kind": "not-a-requirement",
                "reason": "fixture",
            }
            for s in inv.sites
        ],
    }
    if reason:
        art["register-reason"] = reason
    return art


_THREE_REGISTERS = {
    "rfc9001": (_SRC_TWO_SITES, "rfc2119", ""),
    "rfc9002": (_SRC_PROSE_ONLY, "prose", ""),
    "rfc9003": (
        _SRC_NO_INVENTORY,
        "manual-walk",
        "no keyword and no modal in the source",
    ),
}


@contextlib.contextmanager
def _one_per_register():
    """One valid sign-off in EACH register, so the counting half of owner ruling 1 can be
    proved: each raises the signed total by one apiece."""
    arts = {
        s: _signed_for(s, src, reg, why)
        for s, (src, reg, why) in _THREE_REGISTERS.items()
    }
    srcs = {s: src for s, (src, _, _) in _THREE_REGISTERS.items()}
    with _extraction_tree(artifacts=arts, src=srcs):
        yield


class TestExtractionStatus(unittest.TestCase):
    """The machine-readable envelope the umbrella's drain quota consumes. Owner ruling 1
    has TWO halves and both are gated here: every register CREDITS the total, and every
    published count carries its register split."""

    def test_json_envelope_carries_counts_and_registers(self):
        """AC-16: schema-version, signed and enrolled counts, per-register counts, and
        the unsigned backlog list. Lower kebab-case keys (ai/rules/cli.md)."""
        with _one_per_register():
            with _patched(
                load_enrolled=lambda: {"rfc9001", "rfc9002", "rfc9003", "rfc7606"},
                summary_stems=lambda: set(),
                parse_summary_file=lambda p: [],
            ):
                code, out = _run_capturing(R.run_extraction_status)
        self.assertEqual(code, 0, out)
        env = json.loads(out)
        self.assertEqual(env["schema-version"], 1)
        self.assertEqual(env["enrolled"], 4)
        self.assertEqual(env["signed"], 3)
        self.assertEqual(env["backlog"], 1)
        self.assertEqual(env["unsigned"], ["rfc7606"])
        self.assertEqual(
            sorted(env["signed-by-register"]), ["manual-walk", "prose", "rfc2119"]
        )
        for key in env:
            self.assertEqual(key, key.lower(), key)
            self.assertNotIn("_", key)

    def test_every_register_counts_toward_the_signed_total(self):
        """AC-11, the CREDIT half of owner ruling 1: a prose or manual-walk sign-off
        counts exactly as an rfc2119 one does. Excluding the weaker registers would leave
        65 of 166 enrolled RFCs permanently undrainable -- a backlog with no exit."""
        with _one_per_register():
            with _patched(
                load_enrolled=lambda: set(_THREE_REGISTERS),
                summary_stems=lambda: set(),
                parse_summary_file=lambda p: [],
            ):
                env = R.extraction_status([], set(_THREE_REGISTERS))
        self.assertEqual(env["signed"], 3)
        self.assertEqual(
            env["signed-by-register"], {"rfc2119": 1, "prose": 1, "manual-walk": 1}
        )

    def test_signed_by_register_sums_to_total(self):
        """AC-22, and its discriminating twin: a register silently dropped from the total
        makes the sum disagree with the published figure."""
        with _one_per_register():
            with _patched(
                load_enrolled=lambda: set(_THREE_REGISTERS),
                summary_stems=lambda: set(),
                parse_summary_file=lambda p: [],
            ):
                env = R.extraction_status([], set(_THREE_REGISTERS))
        self.assertEqual(sum(env["signed-by-register"].values()), env["signed"])
        self.assertEqual(sorted(env["signed-by-register"]), sorted(R.REGISTERS))

    def test_every_register_key_is_present_even_at_zero(self):
        """A register missing from the split reads as 'not a thing', not as 'zero'."""
        with _extraction_tree(src={}):
            env = R.extraction_status([], {"rfc7606"})
        self.assertEqual(
            env["signed-by-register"], {"rfc2119": 0, "prose": 0, "manual-walk": 0}
        )

    def test_the_envelope_arithmetic_is_self_consistent(self):
        """signed + backlog == enrolled, always.

        With `signed` counting every valid artifact and `backlog` counting only enrolled
        ones, three sign-offs for un-enrolled stems published `enrolled 1, signed 3,
        backlog 1` -- a figure that cannot be true of any set, and the same mismatch that
        let un-enrolled credit satisfy the drain floor."""
        with _one_per_register():
            env = R.extraction_status([], {"rfc7606"})
        self.assertEqual(env["enrolled"], 1)
        self.assertEqual(env["signed"], 0, "an un-enrolled sign-off is not yet credit")
        self.assertEqual(env["backlog"], 1)
        self.assertEqual(env["signed"] + env["backlog"], env["enrolled"])
        self.assertEqual(sum(env["signed-by-register"].values()), env["signed"])


def _mixed_signoff():
    """(artifact, requirements, source) for one signed stem whose derived columns are all
    DISTINCT and non-zero: 3 sites, 2 mapped, 1 excluded, ratio 0.33, 2 gated rows.

    Every number differs from every other on purpose. `_signed_for` excludes every site and
    declares nothing gated, so mapped 0, gated 0 and ratio 1.00 make the row unable to tell
    a correct renderer from one printing a constant -- which is how `| 0 | 0 | 0 | -- | 0 |`
    survived. A 1-mapped/1-excluded fixture is only half a fix: mutation-verified, swapping
    `Extraction.mapped` to count EXCLUDED sites is invisible when the two are equal
    (ai/rules/interop-and-goal-validation.md, the fixture-at-an-extreme trap).
    """
    art = _artifact(src=_SRC_THREE_SITES)
    art["sections"][2] = _section("2", 3, "walked")
    art["sites"] = [
        _site("2:1", _Q1, "mapped", **{"mapped-to": "RFC9999-2-1"}),
        _site("2:2", _Q2, "mapped", **{"mapped-to": "RFC9999-2-2"}),
        _site(
            "2:3",
            _Q3,
            "excluded",
            **{"excluded-kind": "not-a-requirement", "reason": "restates the header"},
        ),
    ]
    return art, _reqs_9999(2), _SRC_THREE_SITES


class TestExtractionLedger(unittest.TestCase):
    def _render(self, enrolled, reqs=()):
        return "\n".join(R.render_extraction_table(list(reqs), set(enrolled)))

    def test_table_reports_unsigned_backlog(self):
        """AC-15 / user story 2: how much of the standards claim is bounded."""
        with _extraction_tree(src={}):
            body = self._render({"rfc7606", "rfc4271"})
        self.assertIn("UNSIGNED (grandfathered)", body)
        self.assertIn("rfc7606", body)
        self.assertIn("rfc4271", body)

    def test_registers_are_published_in_separate_columns(self):
        """AC-21, the COUNTERWEIGHT half of owner ruling 1: a signed total rendered
        without its three component counts beside it is a failure of this AC, not a
        formatting preference. Reading 'N signed off' as 'N keyword-verified' is the same
        category error this whole spec set exists to correct.

        Asserts the rendered COUNTS, not merely the register names. Mutation-verified: the
        table's own explanatory prose names all three registers, so a name-presence
        assertion stayed green with every count zeroed and gated nothing."""
        with _one_per_register():
            body = self._render(set(_THREE_REGISTERS))
        # One artifact in each register -> each column reads 1.
        for register in R.REGISTERS:
            self.assertIn(
                f"{register} 1",
                body,
                f"{register} must publish its OWN signed count, not just its name",
            )

    def test_no_bare_signed_total_is_rendered(self):
        """Umbrella D6: 'There is no bare total anywhere, in the ledger or in a gate
        message.' Every line naming a signed count names the registers on the same line.

        The summary line is asserted to EXIST first. Guarding only on a phrase the renderer
        itself chooses ('signed off') means rendering the bare total under any other
        wording -- `Total extractions complete: 3` -- runs the loop body zero times and
        passes, which is the precise output D6 forbids."""
        with _one_per_register():
            body = self._render(set(_THREE_REGISTERS))
        summary = [
            ln
            for ln in body.split("\n")
            if "signed off by register" in ln.lower() and "|" not in ln
        ]
        self.assertEqual(len(summary), 1, f"no register-split summary line in:\n{body}")
        self.assertIn("rfc2119 1, prose 1, manual-walk 1", summary[0])
        for line in body.split("\n"):
            if "signed off" in line.lower() and "|" not in line:
                self.assertTrue(
                    all(r in line for r in R.REGISTERS),
                    f"signed count rendered without its register split: {line!r}",
                )

    def test_signed_rows_carry_their_derived_counts(self):
        """AC-15 / AC-21's published evidence, pinned to a hand-typed row.

        Every derived number here is computed from the artifact, and every one of them
        could be replaced by 0 (or the ratio by '--', or `Extraction.mapped` made to count
        excluded sites) with all 252 tests green: the row's only assertions were a
        hardcoded date and a stem name, neither of which is derived."""
        art, reqs, src = _mixed_signoff()
        with _extraction_tree(artifacts={"rfc9999": art}, src={"rfc9999": src}):
            body = self._render({"rfc9999", "rfc4271"}, reqs)
        self.assertIn(
            "| `rfc9999` | rfc2119 | 3 | 2 | 1 | 0.33 | 2 | 2026-07-29 (tester) |",
            body.split("\n"),
        )
        # An unsigned stem publishes NO derived column: nobody walked it, so there is
        # nothing this repository has established to print.
        self.assertIn(
            "| `rfc4271` | -- | -- | -- | -- | -- | 0 | UNSIGNED (grandfathered) |",
            body.split("\n"),
        )

    def test_extraction_table_is_in_the_rendered_ledger(self):
        """The table must reach ai/RFC-REQUIREMENTS.md, not just exist as a helper. It is a
        whole-corpus backlog, so it belongs to the index and to no single shard."""
        with _extraction_tree(src={}):
            body = R.render_index([_req("RFC7606-2-1")], [], {"rfc7606"})
        self.assertIn("Extraction sign-off", body)

    def test_stale_extraction_table_fails_check_fresh(self):
        """AC-15 through the EXISTING freshness gate (check_ledger_fresh:1578), so the
        published backlog cannot rot."""
        reqs = [_req("RFC7606-2-1")]
        tags = [_tag("RFC7606-2-1", "positive"), _tag("RFC7606-2-1", "negative")]
        path = _mkstemp(".md")
        # SHARD_DIR moves with the ledger and holds the CORRECT render, so the index is the
        # only thing that varies. Patching LEDGER_FILE alone left check_ledger_fresh walking
        # the real rfc/requirements/, where 177 files read as orphans: `errs` was then
        # non-empty whatever the extraction table did, and the test passed with
        # render_extraction_table stubbed out to return nothing.
        shard_dir = _mkdtemp("extraction-shards-")
        try:
            with _extraction_tree(src={}):
                with _patched(LEDGER_FILE=path, SHARD_DIR=shard_dir):
                    for stem, body in R.render_shards(reqs, tags, {"rfc7606"}).items():
                        with open(
                            os.path.join(shard_dir, stem + ".md"), "w", encoding="utf-8"
                        ) as fh:
                            fh.write(body + "\n")
                    fresh = R.render_index(reqs, tags, {"rfc7606"}) + "\n"
                    without = "\n".join(
                        ln for ln in fresh.split("\n") if "UNSIGNED" not in ln
                    )
                    self.assertNotEqual(
                        without,
                        fresh,
                        "the fixture must actually drop an extraction row, or the case "
                        "asserts nothing",
                    )
                    # Control: the untouched index and the shards read fresh, so the one
                    # error below is the extraction table and nothing else.
                    with open(path, "w", encoding="utf-8") as fh:
                        fh.write(fresh)
                    self.assertEqual(R.check_ledger_fresh(reqs, tags, {"rfc7606"}), [])
                    with open(path, "w", encoding="utf-8") as fh:
                        fh.write(without)
                    errs = R.check_ledger_fresh(reqs, tags, {"rfc7606"})
            self.assertTrue(
                errs, "a ledger missing the extraction table must read stale"
            )
            self.assertEqual(
                len(errs), 1, f"the index alone must be named stale, got: {errs}"
            )
            self.assertIn("ze-rfc-index-update", errs[0])
            self.assertNotIn(
                "requirements/",
                errs[0],
                "the index is what drifted, so no shard may be named",
            )
        finally:
            shutil.rmtree(shard_dir, ignore_errors=True)
            if os.path.exists(path):
                os.unlink(path)

    def test_extraction_table_render_is_independent_of_input_order(self):
        """Varies the ORDER of the one ordered input, rather than calling the renderer
        twice with the same arguments: `f(x) == f(x)` in a single process is true of every
        implementation, so it discriminated nothing. Order-independence is the property
        that matters -- the freshness gate re-renders and compares byte for byte, so a
        renderer that follows requirement order would report a fresh ledger as stale."""
        reqs = [
            _req("RFC9999-2-1", rfc="rfc9999"),
            _req("RFC4271-5-1", rfc="rfc4271"),
            _req("RFC4271-6-1", rfc="rfc4271"),
        ]
        art, _, src = _mixed_signoff()
        with _extraction_tree(artifacts={"rfc9999": art}, src={"rfc9999": src}):
            forward = self._render({"rfc9999", "rfc4271"}, reqs)
            backward = self._render({"rfc9999", "rfc4271"}, list(reversed(reqs)))
        self.assertEqual(forward, backward)
        self.assertLess(forward.index("rfc4271"), len(forward))


class TestDrainFloor(unittest.TestCase):
    """The floor COMPARISON lives here; the POLICY it reads is authored in
    rfc/drain-budget.txt and owned by the umbrella and ultimately by Thomas. A rate, a
    date or a cadence hardcoded in this module is a violation, not a default."""

    # 3 signed against a 20-entry backlog. The size is load-bearing: the floor is CAPPED
    # at the backlog (AC-28), so a fixture with a backlog of 1 could never fail whatever
    # the rate, and the discriminating test below would be vacuous.
    _BACKLOG = tuple(f"rfc80{i:02d}" for i in range(20))

    def _errs(self, budget, enrolled=tuple(_THREE_REGISTERS) + _BACKLOG):
        arts = {
            s: _signed_for(s, src, reg, why)
            for s, (src, reg, why) in _THREE_REGISTERS.items()
        }
        srcs = {s: src for s, (src, _, _) in _THREE_REGISTERS.items()}
        with _extraction_tree(artifacts=arts, src=srcs, budget=budget):
            signed = R.signed_extractions([])
            return R.check_drain_floor(set(enrolled), signed)

    def test_rate_zero_computes_a_zero_floor(self):
        """AC-27 (umbrella AC-13): the comparison ships INERT, and D5 reserves the
        non-zero rate to Thomas."""
        self.assertEqual(self._errs("start 2020-01-01\nrate 0\n"), [])

    def test_signed_below_armed_floor_fails(self):
        """AC-30, and the discriminating twin of every row above: without this the whole
        floor could ship unable to fail, which is the vacuity this spec exists to remove."""
        errs = self._errs("start 2020-01-01\nrate 1\n")
        self.assertTrue(errs, "an armed rate over 5+ years must exceed 3 sign-offs")
        joined = " ".join(errs)
        self.assertIn("2020-01-01", joined)
        self.assertIn("drain-budget", joined)

    def test_the_failure_message_names_the_registers_it_summed(self):
        """Umbrella D6: a child that needs one number for a threshold states which
        registers it summed, in the message it prints."""
        errs = self._errs("start 2020-01-01\nrate 1\n")
        joined = " ".join(errs)
        for register in R.REGISTERS:
            self.assertIn(register, joined)

    def test_a_register_at_zero_is_still_named_with_its_count(self):
        """`_register_phrase` must print EVERY register, including the ones at zero.

        Every fixture that reached it had all three registers non-zero, so
        `", ".join(... if counts[name] ...)` -- silently dropping a zero register -- was
        invisible (ai/rules/interop-and-goal-validation.md, the fixture-at-an-extreme
        trap). A register missing from the split reads as 'not a thing' rather than as
        'zero', which is the exact misreading owner ruling 1's counterweight exists to
        prevent."""
        self.assertEqual(
            R._register_phrase({"rfc2119": 2, "prose": 0, "manual-walk": 0}),
            "rfc2119 2, prose 0, manual-walk 0",
        )
        self.assertEqual(
            R._register_phrase({"rfc2119": 0, "prose": 0, "manual-walk": 0}),
            "rfc2119 0, prose 0, manual-walk 0",
        )

    def test_the_failure_message_names_a_zero_register_too(self):
        """The same property through the gate message, where an operator reads it: one
        rfc2119 sign-off must still publish 'prose 0, manual-walk 0'."""
        arts = {"rfc9001": _signed_for("rfc9001", _SRC_TWO_SITES, "rfc2119")}
        with _extraction_tree(
            artifacts=arts,
            src={"rfc9001": _SRC_TWO_SITES},
            budget="start 2020-01-01\nrate 1\n",
        ):
            signed = R.signed_extractions([])
            errs = R.check_drain_floor({"rfc9001"} | set(self._BACKLOG), signed)
        joined = " ".join(errs)
        self.assertIn("rfc2119 1, prose 0, manual-walk 0", joined)

    def test_a_fully_drained_corpus_is_permanently_green(self):
        """AC-28 self-retirement: once every enrolled stem is signed the comparison is
        satisfied for good, so the check needs no removal commit.

        The route to that is the cap, not a zero floor: `signed == enrolled` and the floor
        can never exceed `enrolled`. rate 3 == the enrolled count, the last valid value at
        that boundary."""
        self.assertEqual(
            self._errs("start 2020-01-01\nrate 3\n", enrolled=tuple(_THREE_REGISTERS)),
            [],
        )

    def test_the_floor_can_demand_the_whole_enrolled_set(self):
        """The cap is the DRAINABLE set, not the remaining backlog.

        Capping at the remainder counts every sign-off twice -- once raising the cumulative
        total, once lowering the bar -- so the comparison collapsed to
        `signed >= enrolled / 2`: at 166 enrolled, rate 100/month and 12 months elapsed it
        flipped red-to-green at exactly 83, and NO rate Thomas could arm would ever have
        demanded more than half the corpus. Neither the cap test above nor the armed-floor
        test below can see it, because both sit at 0% or 100% signed."""
        start, today = datetime.date(2026, 1, 1), datetime.date(2027, 1, 1)
        self.assertEqual(R.required_floor(start, 100.0, 166, today), 166)
        self.assertEqual(
            R.required_floor(start, 5.0, 166, today), 60
        )  # 5 x 12, uncapped

    def test_half_the_corpus_does_not_satisfy_an_armed_schedule(self):
        """The same defect at the gate, where it decides red or green. Three of four
        enrolled stems signed: under the remainder cap the floor was min(1, huge) = 1 and 3
        sign-offs sailed past it; the schedule asked for four."""
        errs = self._errs(
            "start 2020-01-01\nrate 3\n",
            enrolled=tuple(_THREE_REGISTERS) + ("rfc7606",),
        )
        self.assertTrue(errs, "3 of 4 signed must not satisfy an armed schedule")
        joined = " ".join(errs)
        self.assertIn("requires 4 extraction sign-off(s)", joined)
        self.assertIn("there are 3", joined)

    def test_required_floor_never_exceeds_the_drainable_set(self):
        start = datetime.date(2000, 1, 1)
        self.assertEqual(R.required_floor(start, 100.0, drainable=7), 7)
        self.assertEqual(R.required_floor(start, 0.0, drainable=7), 0)

    def test_whole_months_are_counted_at_the_day_boundary(self):
        """ai/rules/testing.md boundary testing: last invalid, first valid, first beyond.

        Every part of this arithmetic was unpinned -- a `+1` on the month count, dropping
        the partial-month adjustment, and ceil->floor all survived the whole suite. The
        floor decides whether the gate is red, so an off-by-one month is an off-by-one
        sign-off owed."""
        start = datetime.date(2026, 1, 15)
        cases = [
            (datetime.date(2026, 1, 15), 0),  # the start day itself: no month yet
            (datetime.date(2026, 2, 14), 0),  # start.day - 1: still short
            (datetime.date(2026, 2, 15), 1),  # start.day: exactly one month
            (datetime.date(2026, 2, 16), 1),  # start.day + 1: still one, not two
            (datetime.date(2026, 3, 14), 1),
            (datetime.date(2026, 3, 15), 2),
            (datetime.date(2027, 1, 15), 12),
        ]
        for today, want in cases:
            self.assertEqual(
                R.required_floor(start, 1.0, drainable=99, today=today),
                want,
                f"{today} since {start}",
            )

    def test_a_month_shorter_than_the_start_day_still_elapses(self):
        """A start on the 29th, 30th or 31st has no anniversary in every month. Comparing
        the raw day numbers drops one month at each such boundary -- and the SHIPPED policy
        starts 2026-07-29, so it would lose a month every February for as long as the drain
        runs."""
        self.assertEqual(
            R.required_floor(
                datetime.date(2026, 3, 31), 1.0, 99, datetime.date(2026, 4, 30)
            ),
            1,
            "31 Mar -> 30 Apr is a whole calendar month; April has no 31st",
        )
        self.assertEqual(
            R.required_floor(
                datetime.date(2026, 3, 31), 1.0, 99, datetime.date(2026, 4, 29)
            ),
            0,
            "...but 29 Apr is not yet",
        )
        self.assertEqual(
            R.required_floor(
                datetime.date(2026, 7, 29), 1.0, 99, datetime.date(2027, 2, 28)
            ),
            7,
            "the shipped start date across a non-leap February",
        )

    def test_a_fractional_rate_rounds_up(self):
        """`ceil`, not `floor` or `round`: at 0.5/month the first month already owes one
        sign-off, and a schedule that owes 0 for its first month is not a schedule. Both
        weaker roundings survived the entire suite."""
        start = datetime.date(2026, 1, 1)
        self.assertEqual(
            R.required_floor(start, 0.5, 99, datetime.date(2026, 2, 1)), 1
        )  # 0.5 -> 1
        self.assertEqual(
            R.required_floor(start, 0.5, 99, datetime.date(2026, 3, 1)), 1
        )  # 1.0 -> 1
        self.assertEqual(
            R.required_floor(start, 0.5, 99, datetime.date(2026, 4, 1)), 2
        )  # 1.5 -> 2
        self.assertEqual(
            R.required_floor(start, 0.25, 99, datetime.date(2026, 2, 1)), 1
        )  # 0.25 -> 1

    def test_required_floor_is_zero_before_the_start_date(self):
        import datetime

        future = datetime.date.today() + datetime.timedelta(days=400)
        self.assertEqual(R.required_floor(future, 10.0, drainable=99), 0)

    def test_a_rate_of_zero_reads_off_disk_as_an_inert_floor(self):
        """AC-27 / owner decision D5: the shipped comparison is INERT and only Thomas arms
        it. Pinned through a FIXTURE file, never through rfc/drain-budget.txt: asserting
        `rate == 0.0` against the committed policy makes his one-line arming commit red the
        suite, and a rule that fails on the work it exists to trigger gets deleted rather
        than obeyed. The live file is still parsed on every run by check_drain_floor, so a
        malformed one fails `make ze-rfc-check` regardless."""
        path = _mkstemp(".txt")
        try:
            with open(path, "w", encoding="utf-8") as fh:
                fh.write("# shipped shape\nstart 2020-01-01\nrate 0\n")
            budget = R.parse_drain_budget(path)
            self.assertEqual(budget.rate, 0.0)
            self.assertEqual(budget.start, datetime.date(2020, 1, 1))
            self.assertEqual(R.required_floor(budget.start, budget.rate, 166), 0)
        finally:
            os.unlink(path)

    def test_missing_drain_budget_is_error_not_empty(self):
        """AC-29 (umbrella AC-4): the zero-value trap closed on the POLICY input. An
        absent budget must NOT compute a floor of 0 and read as 'nothing owed'; the guard
        is the same shape as check_enrolment:660, which refuses to report clean while
        enforcing nothing."""
        errs = self._errs(None)
        self.assertTrue(errs)
        self.assertIn("drain-budget.txt", errs[0])

    def test_unparseable_drain_budget_is_an_error(self):
        for bad in (
            "",
            "rate 0\n",
            "start 2020-01-01\n",
            "start nonsense\nrate 0\n",
            "start 2020-01-01\nrate -1\n",
            "start 2020-01-01\nrate x\n",
            # float() accepts every one of these, and NaN is false against <, > and >=, so
            # it slipped past the negative guard AND the above-enrolled guard and reached
            # math.ceil(nan * months) -- an uncaught ValueError outside run_check's handler,
            # i.e. a traceback instead of the clean exit 2 AC-18 requires.
            "start 2020-01-01\nrate nan\n",
            "start 2020-01-01\nrate NaN\n",
            "start 2020-01-01\nrate inf\n",
            "start 2020-01-01\nrate -inf\n",
        ):
            errs = self._errs(bad)
            self.assertTrue(errs, f"budget {bad!r} must be refused")

    def test_a_non_finite_rate_is_refused_at_parse_time(self):
        """Rejected where every other malformed value is rejected, so the caller never has
        to defend against it. Asserts the exit CODE too: a traceback out of run_check is a
        different failure from a violation, and only one of them is acceptable."""
        for bad in ("nan", "inf", "-inf", "Infinity"):
            path = _mkstemp(".txt")
            try:
                with open(path, "w", encoding="utf-8") as fh:
                    fh.write(f"start 2020-01-01\nrate {bad}\n")
                with self.assertRaises(R.ParseError, msg=bad) as cm:
                    R.parse_drain_budget(path)
                self.assertIn("finite", str(cm.exception))
            finally:
                os.unlink(path)

    def test_a_duplicate_budget_key_is_refused(self):
        """The key set is closed AND single-valued: with two 'rate' lines the last would
        silently win, so a reviewer reading the top of the file sees a policy that is not
        in force."""
        for bad in (
            "start 2020-01-01\nrate 0\nrate 5\n",
            "start 2020-01-01\nstart 2021-01-01\nrate 0\n",
        ):
            path = _mkstemp(".txt")
            try:
                with open(path, "w", encoding="utf-8") as fh:
                    fh.write(bad)
                with self.assertRaises(R.ParseError, msg=bad) as cm:
                    R.parse_drain_budget(path)
                self.assertIn("set twice", str(cm.exception))
            finally:
                os.unlink(path)

    def test_a_budget_naming_an_rfc_is_refused(self):
        """The file carries POLICY ONLY. The moment it names an RFC it has become the
        hand-kept registry the 2026-07-29 resolution rejected."""
        errs = self._errs("start 2020-01-01\nrate 0\nrfc7296 signed\n")
        self.assertTrue(errs)

    def test_rate_above_the_enrolled_set_is_refused(self):
        """Asserts the MESSAGE, not merely that something failed: with this guard disabled
        the armed-floor arm fires on the same fixture and `assertTrue(errs)` still holds,
        so the row proved nothing about the guard it is named for."""
        errs = self._errs("start 2020-01-01\nrate 999\n")
        joined = " ".join(errs)
        self.assertIn("exceeds the whole enrolled set", joined)
        self.assertIn("999", joined)

    def test_an_unenrolled_signoff_earns_no_drain_credit(self):
        """The credit and the backlog must describe the SAME set.

        Counting every valid artifact as credit while counting only enrolled stems as
        backlog let a sign-off for a stem nobody enrolled satisfy the floor without
        draining anything: 8 enrolled, 6 sign-offs for un-enrolled stems, floor met,
        backlog still 8 of 8. Eleven un-enrolled source texts sit in rfc/full and
        rfc/drafts today, so the credit was already there for the taking."""
        # The three signed stems are NOT in the enrolled set: pure un-enrolled credit.
        errs = self._errs("start 2020-01-01\nrate 1\n", enrolled=self._BACKLOG)
        self.assertTrue(errs, "an un-enrolled sign-off must not satisfy the floor")
        self.assertIn("and there are 0", " ".join(errs))

    def test_a_signoff_counts_the_moment_its_stem_enrols(self):
        """The discriminating twin, and the workflow AC-1 requires: sign-off is a
        PRECONDITION of enrolment, so a stem is routinely signed before it is enrolled. It
        simply does not count yet, and starts counting on the enrolling commit."""
        self.assertEqual(
            self._errs("start 2020-01-01\nrate 3\n", enrolled=tuple(_THREE_REGISTERS)),
            [],
        )


class _ExtractionDrive(unittest.TestCase):
    """Shared run_check driver: everything unrelated to extraction is patched out, so a
    failure here is the extraction machinery and nothing else."""

    def _drive(self, enrolled=("rfc9999",), baseline_enrolled=None, reqs=None, **kw):
        reqs = _reqs_9999() if reqs is None else reqs
        base = set(enrolled) if baseline_enrolled is None else set(baseline_enrolled)
        overrides = dict(
            load_enrolled=lambda: set(enrolled),
            summary_stems=lambda: set(enrolled),
            parse_summary_file=lambda path: reqs,
            _git_baseline_enrolment=lambda: base,
            _git_baseline_ids=lambda: {r.rid for r in reqs},
            _git_baseline_tag_polarities=lambda: {},
            _git_baseline_evidence=lambda: {},
            _git_baseline_summary_stems=lambda: set(enrolled),
            scan_tree=lambda *a, **k: [
                _tag(r.rid, p) for r in reqs for p in ("positive", "negative")
            ],
            check_status_agreement=lambda *a, **k: [],
            check_audit_freshness=lambda *a, **k: [],
            # The rest of the audit half, neutralised for the same reason the line above
            # is: this driver's subject is not the audit record, and the REAL
            # rfc/audit/rfc7606.json legitimately describes 52 requirements this fixture
            # does not declare (spec-rfcgate-3-audit-teeth.md).
            check_audit_files=lambda *a, **k: [],
            check_audit_schema=lambda *a, **k: [],
            check_audit_disclosure=lambda *a, **k: [],
            check_audit_note=lambda *a, **k: [],
            check_audit_findings=lambda *a, **k: [],
            check_audit_verdict_ratchet=lambda *a, **k: [],
            # The four ledger-edge checks (plan/spec-rfcgate-4-ledger.md), neutralised for
            # the same reason check_status_agreement above is: they read the REAL
            # rfc/not-enrolled.txt and docs/features/rfc-status.md, and this driver
            # declares a SYNTHETIC summary universe. Judging 157 real status rows and 7
            # real dispositions against it produces a wall of violations that has nothing
            # to do with this driver's subject. Each of the four has its own wiring class,
            # which is where a lost call site is caught.
            check_summary_disposition=lambda *a, **k: [],
            check_status_completeness=lambda *a, **k: [],
            check_unproven_support=lambda *a, **k: [],
            check_gap_count_agreement=lambda *a, **k: [],
            check_ledger_fresh=lambda *a, **k: [],
        )
        overrides.update(kw)  # an explicit override wins over the default wiring
        with _patched(**overrides):
            return _run_capturing(R.run_check)


class TestExtractionSignoffWiring(_ExtractionDrive):
    """check_extraction_signoff and the check_enrolment precondition are dead code unless
    run_check calls them. These drive run_check end-to-end."""

    def test_run_check_fails_on_new_enrolment_without_signoff(self):
        """AC-1: a stem enrolled since HEAD with no sign-off reds the gate."""
        with _extraction_tree(src={"rfc9999": _SRC_TWO_SITES}):
            code, out = self._drive(enrolled=("rfc9999",), baseline_enrolled=())
        self.assertEqual(code, 2, out)
        self.assertIn("rfc9999", out)
        self.assertIn("extraction sign-off", out)

    def test_run_check_passes_on_preexisting_enrolment_without_signoff(self):
        """AC-19, the discriminating twin: grandfathering is scope, so a stem enrolled at
        HEAD with no sign-off is NOT accused."""
        with _extraction_tree(src={"rfc9999": _SRC_TWO_SITES}):
            code, out = self._drive(
                enrolled=("rfc9999",), baseline_enrolled=("rfc9999",)
            )
        self.assertEqual(code, 0, out)

    def test_an_unavailable_enrolment_baseline_accuses_nobody(self):
        """The precondition is gated on the availability of the baseline it is computed
        FROM.

        It was gated on `_git_baseline_summary_stems` instead -- a different git call,
        against a different path. Drive the state where that one succeeds and
        `_git_baseline_enrolment` fails (a shallow clone, a grafted worktree, an
        rfc/enrolled.txt that is new in this commit) and every enrolled RFC is reported as
        newly enrolled without a sign-off: 166 violations no developer can act on, which is
        precisely the wall check_enrolment's own docstring says the design prevents, and
        the fastest way to teach people to bypass the gate."""
        with _extraction_tree(src={"rfc9999": _SRC_TWO_SITES}):
            code, out = self._drive(
                enrolled=("rfc9999",), _git_baseline_enrolment=lambda: None
            )
        self.assertEqual(code, 0, out)
        self.assertNotIn("newly enrolled", out)

    def test_an_available_baseline_still_accuses_a_new_enrolment(self):
        """The discriminating twin: 'could not look' must widen nothing else. With the
        baseline READABLE and empty, rfc9999 really is new and really has no sign-off."""
        with _extraction_tree(src={"rfc9999": _SRC_TWO_SITES}):
            code, out = self._drive(
                enrolled=("rfc9999",), _git_baseline_enrolment=lambda: set()
            )
        self.assertEqual(code, 2, out)
        self.assertIn("newly enrolled", out)

    def test_run_check_fails_on_unclassified_site(self):
        """AC-3: a generated skeleton can never pass."""
        art = _artifact()
        art["sites"] = [_site("2:1", _Q1), _site("2:2", _Q2)]
        with _extraction_tree(
            artifacts={"rfc9999": art}, src={"rfc9999": _SRC_TWO_SITES}
        ):
            code, out = self._drive()
        self.assertEqual(code, 2, out)
        self.assertIn("2:1", out)

    def test_run_check_clean_on_a_fully_classified_signoff(self):
        """Discriminating twin: proves the check is not 'always fails'."""
        with _extraction_tree(
            artifacts={"rfc9999": _artifact()}, src={"rfc9999": _SRC_TWO_SITES}
        ):
            code, out = self._drive()
        self.assertEqual(code, 0, out)


class TestSuccessLine(_ExtractionDrive):
    """AC-14 / R-6: the gate states the extraction bound OUT LOUD on every clean run,
    including when it is zero. Nothing read run_check's SUCCESS output, so deleting the
    whole `extraction:` print broke no test -- the D6 publishing counterweight was ungated
    on the one surface an operator actually sees."""

    def test_a_clean_run_publishes_the_extraction_bound(self):
        with _extraction_tree(
            artifacts={"rfc9999": _artifact()}, src={"rfc9999": _SRC_TWO_SITES}
        ):
            code, out = self._drive()
        self.assertEqual(code, 0, out)
        self.assertIn("extraction", out)
        self.assertIn("rfc2119 1, prose 0, manual-walk 0", out)
        self.assertIn("of 1 enrolled", out)
        self.assertIn("0 unsigned", out)

    def test_a_clean_run_with_nothing_signed_still_says_so(self):
        """The zero case is the one that matters: a check quietly satisfied by nothing
        reproduces the very failure it exists to fix, so the bound is printed even when it
        bounds nothing."""
        with _extraction_tree(src={"rfc9999": _SRC_TWO_SITES}):
            code, out = self._drive(baseline_enrolled=("rfc9999",))
        self.assertEqual(code, 0, out)
        self.assertIn("rfc2119 0, prose 0, manual-walk 0", out)
        self.assertIn("1 unsigned", out)


class TestExtractionRatchetWiring(_ExtractionDrive):
    def test_run_check_fails_when_a_signoff_disappears(self):
        """AC-12: sign-off is monotonic."""
        with _extraction_tree(src={"rfc9999": _SRC_TWO_SITES}):
            with _patched(
                _git_baseline_extractions=lambda: {
                    "rfc9999": R.BaselineExtraction(0, "2026-07-29", "")
                }
            ):
                code, out = self._drive(baseline_enrolled=("rfc9999",))
        self.assertEqual(code, 2, out)
        self.assertIn("rfc9999", out)

    def test_run_check_clean_when_the_signoff_is_still_there(self):
        with _extraction_tree(
            artifacts={"rfc9999": _artifact()}, src={"rfc9999": _SRC_TWO_SITES}
        ):
            with _patched(
                _git_baseline_extractions=lambda: {
                    "rfc9999": R.BaselineExtraction(0, "2026-07-29", "")
                }
            ):
                code, out = self._drive()
        self.assertEqual(code, 0, out)


class TestDrainFloorWiring(_ExtractionDrive):
    def test_run_check_fails_when_signed_count_is_below_the_floor(self):
        """AC-30: with the rate armed, a signed count under the floor reds."""
        with _extraction_tree(
            budget="start 2020-01-01\nrate 1\n", src={"rfc9999": _SRC_TWO_SITES}
        ):
            code, out = self._drive(baseline_enrolled=("rfc9999",))
        self.assertEqual(code, 2, out)
        self.assertIn("drain", out.lower())

    def test_run_check_clean_at_the_shipped_rate_of_zero(self):
        """AC-27: the comparison ships INERT."""
        with _extraction_tree(
            budget="start 2020-01-01\nrate 0\n", src={"rfc9999": _SRC_TWO_SITES}
        ):
            code, out = self._drive(baseline_enrolled=("rfc9999",))
        self.assertEqual(code, 0, out)


class TestFixtureIsolationFromTheRealExtractionTree(_ExtractionDrive):
    """The recurrence guard: a fixture-driven run_check may not be influenced by the real
    rfc/extraction/ contents, in the tree OR at HEAD.

    Eight tests went red the moment plan/spec-rfcgate-4-ledger.md committed four
    sign-offs, because `_extraction_tree` replaced the artifact DIRECTORY while leaving
    the HEAD baseline reader pointed at the real repository. Nothing failed until a real
    artifact existed, so the asymmetry shipped invisibly and its cost landed on whichever
    session next committed a sign-off -- the exact work the gate exists to encourage.
    These rows fail immediately if that isolation is removed again.
    """

    REAL_STEMS = ("rfc1035", "rfc3765", "rfc4486", "rfc5301")

    def test_the_real_tree_carries_signoffs_so_this_guard_discriminates(self):
        """The precondition, asserted rather than assumed: if rfc/extraction/ were empty
        the rows below would pass with the isolation deleted, and the guard would be
        theatre. Safe to rely on because check_extraction_ratchet makes sign-off
        monotonic -- a committed artifact cannot go away."""
        real = R.extraction_stems()
        for stem in self.REAL_STEMS:
            self.assertIn(stem, real, "the four ledger sign-offs are committed")

    def test_a_fixture_baseline_reports_no_real_stem(self):
        """The mechanism, isolated: inside a fixture tree the HEAD baseline describes the
        FIXTURE's history (nothing committed), never the repository's."""
        with _extraction_tree(src={}):
            baseline = R._git_baseline_extractions()
        self.assertEqual(baseline, {})

    def test_an_empty_fixture_tree_is_not_accused_of_deleting_the_real_signoffs(self):
        """The regression itself. An empty fixture tree plus the real HEAD produced
        'rfcNNNN had an extraction sign-off at HEAD and has none now' four times over,
        for four artifacts the fixture never claimed to hold."""
        with _extraction_tree(src={}):
            self.assertEqual(R.check_extraction_ratchet(), [])

    def test_a_fixture_driven_run_check_names_no_real_stem(self):
        """And end to end, which is what the gate's operators see: a clean fixture run
        exits 0 and its output mentions only the fixture's own universe. Asserting the
        absence of the stem NAMES catches leakage that a 0 exit code alone would not --
        a real stem surfacing in a warning line, a published count, or a success line."""
        with _extraction_tree(src={"rfc9999": _SRC_TWO_SITES}):
            code, out = self._drive(baseline_enrolled=("rfc9999",))
        self.assertEqual(code, 0, out)
        for stem in self.REAL_STEMS:
            self.assertNotIn(stem, out, f"{stem} leaked into a fixture-driven run")


class TestSkeletonWriterWiring(unittest.TestCase):
    def test_extract_skeleton_dispatches_from_main(self):
        """AC-2: `make ze-rfc-extraction-create STEM=x` reaches run_extract_skeleton."""
        seen = {}

        def fake(stem):
            seen["stem"] = stem
            return 0

        with _patched(run_extract_skeleton=fake):
            code, _ = _run_capturing(
                lambda: R.main(["prog", "--extract-skeleton", "rfc9999"])
            )
        self.assertEqual(code, 0)
        self.assertEqual(seen.get("stem"), "rfc9999")

    def test_extract_skeleton_without_a_stem_fails_closed(self):
        code, out = _run_capturing(lambda: R.main(["prog", "--extract-skeleton"]))
        self.assertEqual(code, 2)
        self.assertIn("needs a stem", out)


class TestExtractionStatusWiring(unittest.TestCase):
    def test_extraction_status_dispatches_from_main(self):
        with _patched(run_extraction_status=lambda: 0):
            code, _ = _run_capturing(
                lambda: R.main(["prog", "--extraction-status", "--json"])
            )
        self.assertEqual(code, 0)


# --------------------------------------------------------------------------
# Real-tree sanity
# --------------------------------------------------------------------------
class TestRealTreeExtraction(unittest.TestCase):
    """Fixtures prove the rules; these prove the rules meet the real corpus. A derivation
    that silently returns nothing for real RFC formatting would pass every fixture above
    and bound nothing at all."""

    @classmethod
    def setUpClass(cls):
        cls.enrolled = sorted(R.load_enrolled())
        # Parse failures are COLLECTED, never swallowed. Swallowing one contributes zero
        # gated rows for that stem, which makes `keyword_sites >= gated` easier to satisfy
        # and hands the source the rfc2119 grade it may not deserve -- a fixture that
        # degrades toward false-green whenever a summary breaks.
        reqs, cls.unparsed = [], []
        for stem in cls.enrolled:
            path = os.path.join(R.SUMMARY_DIR, stem + ".md")
            try:
                reqs.extend(R.parse_summary_file(path))
            except R.ParseError as exc:
                cls.unparsed.append(f"{stem}: {exc}")
        cls.gated = R.gated_counts(reqs)
        cls.inventories = {
            stem: R.derive_inventory(stem, cls.gated.get(stem, 0))
            for stem in cls.enrolled
        }

    def test_every_enrolled_summary_parses(self):
        """The fixture every other test in this class rests on. An enrolled summary that
        stops parsing silently lowers this class's gated counts, and a lower gated count
        makes the STRONG register easier to derive."""
        self.assertEqual(self.unparsed, [], "enrolled summaries that no longer parse")

    def test_every_enrolled_stem_round_trips_through_the_parser(self):
        """Generate, then re-read: what `make ze-rfc-extraction-create STEM=<stem>` would write for
        every enrolled RFC must satisfy parse_extraction_artifact.

        Fixtures cannot see this. rfc2865, rfc2869, rfc1195 and sflow-v5 each carry a
        column-0 line the heading pattern reads as a heading, so each derived a duplicate
        section id and produced a skeleton that could not be re-read -- `ze-rfc-extraction-create`
        exited 0 over a bricked file, and one such artifact committed would have made every
        later --check print 'cannot run' and hide every other RFC violation in the repo.
        252 fixture tests were green throughout."""
        tmp = _mkdtemp("ze-roundtrip-")
        try:
            broken = []
            for stem, inv in sorted(self.inventories.items()):
                if inv is None:
                    continue
                path = os.path.join(tmp, stem + ".json")
                with open(path, "w", encoding="utf-8") as fh:
                    json.dump(R._artifact_document(inv, None), fh, indent=2)
                try:
                    R.parse_extraction_artifact(path)
                except R.ParseError as exc:
                    broken.append(f"{stem}: {exc}")
                os.unlink(path)
            self.assertEqual(broken, [], "skeletons their own parser would refuse")
        finally:
            shutil.rmtree(tmp, ignore_errors=True)

    def test_every_existing_signoff_survives_a_refresh(self):
        """The case above passes `previous=None`, so the whole carry-forward path was
        untested against the corpus. A refresh over an artifact that ALREADY exists must
        also produce a document its own parser accepts, or the reviewer cannot re-run the
        walk after the source text moves.

        rfc1035's single `duplicate-of` site failed exactly here: `mapped-to` was dropped,
        the re-parse guard refused the write, and `make ze-rfc-extraction-create STEM=rfc1035`
        exited 2 over a file it declined to touch."""
        tmp = _mkdtemp("ze-refresh-")
        try:
            broken = []
            for stem in sorted(R.extraction_stems()):
                previous = R.parse_extraction_artifact(
                    os.path.join(R.EXTRACTION_DIR, stem + ".json")
                )
                inv = R.derive_inventory(
                    stem, R.gated_counts(R._summary_requirements(stem)).get(stem, 0)
                )
                if inv is None:
                    continue
                path = os.path.join(tmp, stem + ".json")
                with open(path, "w", encoding="utf-8") as fh:
                    json.dump(R._artifact_document(inv, previous), fh, indent=2)
                try:
                    R.parse_extraction_artifact(path)
                except R.ParseError as exc:
                    broken.append(f"{stem}: {exc}")
                os.unlink(path)
            self.assertEqual(broken, [], "refreshes their own parser would refuse")
        finally:
            shutil.rmtree(tmp, ignore_errors=True)

    def test_no_enrolled_stem_derives_a_duplicate_locator(self):
        """A locator is the only handle a reviewer's decision has on a sentence.
        _evaluate_extraction keys sites into a dict, so a collision silently drops one
        obligation from the judgement -- rfc1195 lost 6 of its normative sentences that
        way, and no fixture could see it."""
        collisions = {}
        for stem, inv in sorted(self.inventories.items()):
            if inv is None:
                continue
            ids = [s.id for s in inv.sites]
            dupes = sorted({i for i in ids if ids.count(i) > 1})
            if dupes:
                collisions[stem] = dupes
        self.assertEqual(collisions, {})

    def test_every_enrolled_rfc_derives_a_register(self):
        """Enrolment already requires a source text (check_enrolment:676), so every
        enrolled stem must yield an inventory. None would mean 'I could not look'."""
        self.assertGreater(len(self.enrolled), 150)
        missing = [s for s, inv in self.inventories.items() if inv is None]
        self.assertEqual(missing, [], "enrolled stems with no derivable inventory")
        for stem, inv in self.inventories.items():
            self.assertIn(inv.register, R.REGISTERS, stem)

    def test_a_large_minority_cannot_take_the_rfc2119_grade(self):
        """A-2: a capitalised-keyword inventory is the WRONG oracle for a large minority
        of the enrolled corpus, so a single-register design would have been vacuously
        green for a third of the tree."""
        weak = [s for s, i in self.inventories.items() if i.register != "rfc2119"]
        self.assertGreater(
            len(weak),
            30,
            "measured 65 of 166 on 2026-07-29; a collapse here means the undercount "
            "clause or the prose scan stopped working",
        )

    def test_keyword_free_sources_never_derive_rfc2119(self):
        """A-3 / R-4, the fail-open this spec exists to close: the RFCs with no
        capitalised MUST-level keyword site are exactly the population that would be
        reported as '0 sites, all classified' by a keyword-only check."""
        zero = [s for s, i in self.inventories.items() if i.keyword_sites == 0]
        self.assertGreater(
            len(zero),
            15,
            "measured 23 stems / 172 gated MUSTs on 2026-07-29, at the SITE denominator "
            "(keyword_sites == 0). Not to be confused with the OCCURRENCE denominator "
            "(source_keyword_count == 0), which gives 22 / 164",
        )
        # Bounded ABOVE as well, and that half is load-bearing: with the site scan broken
        # every stem would report zero keyword sites, derive `manual-walk`, and satisfy
        # the per-stem assertion below trivially. Mutation-verified -- without this line,
        # disabling _sites_for left this test green.
        self.assertLess(
            len(zero),
            len(self.enrolled) // 2,
            "most enrolled RFCs DO use capitalised keywords; a majority reporting zero "
            "means the site scan stopped seeing them",
        )
        for stem in zero:
            self.assertNotEqual(self.inventories[stem].register, "rfc2119", stem)
            self.assertGreater(
                self.gated.get(stem, 0) + len(self.inventories[stem].sites),
                0,
                f"{stem} would sign off on an empty inventory with nothing declared",
            )

    def test_the_prose_register_rescues_almost_all_of_them(self):
        """A-4: the case-insensitive modal scan gives the keyword-free stems a non-empty
        inventory. A-5 predicts a small remainder that even prose cannot see."""
        zero = [s for s, i in self.inventories.items() if i.keyword_sites == 0]
        rescued = [s for s in zero if self.inventories[s].register == "prose"]
        manual = [s for s in zero if self.inventories[s].register == "manual-walk"]
        self.assertGreater(len(rescued), 15)
        self.assertLessEqual(
            len(manual),
            5,
            "manual-walk is the terminal escape; a large population here means the prose "
            "scan regressed",
        )


class TestGrandfatheredBacklog(unittest.TestCase):
    """AC-19: grandfathering is SCOPE, not an allowlist file. With no artifact present the
    new checks judge only stems enrolled since HEAD, so a large pre-existing backlog stays
    green when the machinery lands.

    Driven from a FIXTURE extraction directory, never the live rfc/extraction/. Asserting
    `extraction_stems() == set()` against the real tree would red on the very first
    sign-off this spec set exists to produce -- the pilot, the fleet drain, or child 4's
    four enrolments -- and a rule that reds the gate on the work it demands gets deleted
    rather than obeyed (ai/rules/rfc-compliance.md:114-116). It is also non-hermetic
    against any session writing an artifact while the suite runs."""

    _BACKLOG = tuple(f"rfc90{i:02d}" for i in range(20))

    def test_an_unsigned_backlog_is_accused_by_nothing(self):
        reqs = [_req("RFC9000-2-1", rfc="rfc9000")]
        with _extraction_tree(src={}):
            self.assertEqual(R.extraction_stems(), set())
            self.assertEqual(R.check_extraction_signoff(reqs), [])
            self.assertEqual(R.check_extraction_ratchet(), [])
            self.assertEqual(R.check_drain_floor(set(self._BACKLOG), {}), [])

    def test_a_stem_with_no_artifact_is_never_reported_as_signed(self):
        """The other half: 'unaccused' must not mean 'silently counted'. A backlog of 20
        with nothing walked publishes a backlog of 20, not a quiet zero."""
        with _extraction_tree(src={}):
            env = R.extraction_status([], set(self._BACKLOG))
        self.assertEqual(env["signed"], 0)
        self.assertEqual(env["backlog"], len(self._BACKLOG))
        self.assertEqual(env["unsigned"], sorted(self._BACKLOG))


class TestCITierIsEarnedNotAssumed(unittest.TestCase):
    """A `.ci`/`.et` claims merge-gate tier only where a ze-precommit-verify stage runs it.

    The `functional` and `editor` carriers used to declare `prefix=""`, so ANY `.ci`
    anywhere under internal/, pkg/ or test/ was credited `functional/verify` by extension
    alone. Three evasions followed, each of them silent: move a tagged `.ci` out of a run
    suite (test/traffic/), into the gitignored incubator (test/draft/), or into a tree
    whose sibling check.py the SAME table refuses as unrun (test/interop-ipsec/). The tier
    is now derived from mk/test-functional.mk's own suite list, so it tracks reality
    instead of restating it (ai/rules/evidence.md).
    """

    def test_a_run_suite_is_verify_tier(self):
        # test/ipsec/ joined all_suites (mk/test-functional.mk) on 2026-07-30. Its 8 .ci
        # files had a registered runner root and needed no privilege, and nothing ran them.
        # It is asserted here so the tier it now earns cannot be lost silently.
        for rel in (
            "test/plugin/x.ci",
            "test/parse/x.ci",
            "test/ospf/x.ci",
            "test/ipsec/x.ci",
        ):
            c = R.carrier_for(rel)
            self.assertIsNotNone(c, rel)
            self.assertEqual(c.tier, R.TIER_VERIFY, rel)
            self.assertEqual(c.label, "functional/verify", rel)

    def test_evasion_moving_a_ci_out_of_a_run_suite_is_not_verify_tier(self):
        """test/traffic/, test/vpp/, test/chaos-web/ and friends have runners but sit
        outside ze-functional-test's suite list, so nothing runs them on the merge path."""
        for rel in (
            "test/traffic/x.ci",
            "test/vpp/x.ci",
            "test/chaos-web/x.ci",
            "test/static/x.ci",
            "test/vrrp/x.ci",
            "test/flow-export/x.ci",
            "test/chaos/x.ci",
        ):
            c = R.carrier_for(rel)
            self.assertIsNotNone(c, rel)
            self.assertEqual(c.tier, R.TIER_UNRUN, rel)

    def test_evasion_moving_a_ci_into_the_draft_incubator_is_not_a_carrier(self):
        """test/draft/ is the mandated incubator (ai/rules/testing.md) and is gitignored.
        A repo-wide scanner must SKIP it, not refuse it: a draft is invisible to gates, so
        it can neither claim evidence nor redden anyone else's run."""
        for rel in (
            "test/draft/plugin/wip.ci",
            "test/draft/editor/wip.et",
            "test/draft/interop/scenarios/1/check.py",
        ):
            self.assertIsNone(R.carrier_for(rel), rel)

    def test_evasion_moving_a_ci_beside_a_refused_check_py_is_not_verify_tier(self):
        """The sharpest of the three: test/interop-ipsec/**/check.py is refused as unrun
        while test/interop-ipsec/**.ci was credited verify-tier, in ONE table."""
        for tree in ("ipsec-interop", "l2tp-interop", "pppoe-interop", "interop"):
            ci = R.carrier_for(f"test/{tree}/regress.ci")
            self.assertIsNotNone(ci, tree)
            self.assertEqual(ci.tier, R.TIER_UNRUN, tree)

    def test_a_ci_outside_test_is_not_verify_tier(self):
        """TEST_ROOTS includes internal/ and pkg/. No suite walks either for .ci."""
        for rel in (
            "internal/component/bgp/x.ci",
            "pkg/plugin/x.ci",
            "pkg/plugin/x.et",
        ):
            c = R.carrier_for(rel)
            self.assertIsNotNone(c, rel)
            self.assertEqual(c.tier, R.TIER_UNRUN, rel)

    def test_editor_et_is_verify_tier_only_under_the_editor_suite(self):
        self.assertEqual(R.carrier_for("test/editor/mode/x.et").tier, R.TIER_VERIFY)
        self.assertEqual(R.carrier_for("test/traffic/x.et").tier, R.TIER_UNRUN)

    def test_exabgp_compat_is_verify_tier_via_its_own_stage(self):
        """test/exabgp-compat is not in ze-functional-test's list; it runs in the SEPARATE
        ze-functional-exabgp-test stage, which is in both stagesForMode branches."""
        c = R.carrier_for("test/exabgp-compat/encoding/x.ci")
        self.assertIsNotNone(c)
        self.assertEqual(c.tier, R.TIER_VERIFY)
        self.assertIn("ze-functional-exabgp-test", c.runner)

    def test_scan_tree_refuses_a_tag_in_an_unrun_ci(self):
        """End-to-end: the refusal fires through scan_tree, not only through carrier_for."""
        with _scratch() as root:
            d = os.path.join(root, "test", "traffic")
            os.makedirs(d)
            with open(os.path.join(d, "x.ci"), "w", encoding="utf-8") as fh:
                fh.write("# RFC requirement: RFC7606-1-1 positive -- note\n")
            with self.assertRaises(R.ParseError) as cm:
                R.scan_tree(root)
        self.assertIn("nothing executes automatically", str(cm.exception))

    def test_scan_tree_silently_skips_a_draft(self):
        with _scratch() as root:
            d = os.path.join(root, "test", "draft", "plugin")
            os.makedirs(d)
            with open(os.path.join(d, "wip.ci"), "w", encoding="utf-8") as fh:
                fh.write("# RFC requirement: RFC7606-1-1 positive -- note\n")
            self.assertEqual(R.scan_tree(root), [])

    def test_functional_suites_are_read_from_the_makefile(self):
        suites = R.functional_suites()
        self.assertIn("plugin", suites)
        self.assertIn("editor", suites)
        self.assertNotIn("traffic", suites)
        raw = _read_repo("mk/test-functional.mk")
        for s in suites:
            self.assertIn(s, raw, s)

    def test_an_unparseable_suite_list_fails_closed(self):
        with _scratch() as root:
            os.makedirs(os.path.join(root, "mk"))
            path = os.path.join(root, "mk", "test-functional.mk")
            with open(path, "w", encoding="utf-8") as fh:
                fh.write("ze-functional-test:\n\t@echo nope\n")
            with self.assertRaises(R.ParseError):
                R.functional_suites(path)

    def test_a_second_suite_assignment_fails_closed(self):
        """The derivation took the FIRST `all_suites=` match, so a second assignment --
        a plausible future `ze-functional-list` or `-quick` recipe -- silently decided the
        tier of every `.ci` in the repo. That is the same fail-open the earned tier exists
        to close, arriving through the derivation instead of the extension: whichever list
        the regex reached first became the definition of "runs on the merge path".

        Deliberately name-agnostic. The suite names in both decoys below are nonsense, so
        this test cannot be satisfied by enumerating today's un-run directories the way
        test_evasion_moving_a_ci_out_of_a_run_suite_is_not_verify_tier does -- a decoy
        naming test/hub/ or test/stress/ walked straight past that enumeration. What is
        asserted is that AMBIGUITY itself is refused: two answers is not an answer
        (ai/rules/evidence.md).
        """
        with _scratch() as root:
            os.makedirs(os.path.join(root, "mk"))
            path = os.path.join(root, "mk", "test-functional.mk")
            with open(path, "w", encoding="utf-8") as fh:
                fh.write(
                    "ze-functional-list:\n"
                    '\tall_suites="qqalpha qqbeta"; \\\n'
                    "\tfor suite in $$all_suites; do echo $$suite; done\n"
                    "\n"
                    "ze-functional-test:\n"
                    '\tall_suites="qqgamma qqdelta"; \\\n'
                    "\tfor suite in $$all_suites; do echo $$suite; done\n"
                )
            with self.assertRaises(R.ParseError) as cm:
                R.functional_suites(path)
        msg = str(cm.exception)
        self.assertIn(path, msg)
        self.assertIn("2", msg)
        # Neither list may be adopted: the refusal must not read as "we picked one".
        self.assertNotIn("qqalpha", msg)
        self.assertNotIn("qqgamma", msg)

    def test_the_repo_declares_its_suite_list_exactly_once(self):
        """The other half of the pin: the refusal above is only reachable if the real
        recipe is unambiguous today, so the count is asserted against the real file."""
        raw = _read_repo("mk/test-functional.mk")
        self.assertEqual(len(R._ALL_SUITES_RE.findall(raw)), 1)

    def test_a_declared_suite_that_is_never_dispatched_fails_closed(self):
        """The .ci half of the same defect the Go compile check closes.

        `ipsec` sat in all_suites with no `run_suite` line: the recipe counted it toward
        the progress denominator, ran nothing, and every test/ipsec/*.ci still earned a
        verify tier here. Only a comment tied the declaration to the dispatch, so the
        tier came from a claim nobody measured.
        """
        with _scratch() as root:
            os.makedirs(os.path.join(root, "mk"))
            path = os.path.join(root, "mk", "test-functional.mk")
            with open(path, "w", encoding="utf-8") as fh:
                fh.write(
                    "ze-functional-test:\n"
                    '\tall_suites="qqalpha qqbeta"; \\\n'
                    "\trun_suite() { \\\n"
                    '\t\t"$$@"; \\\n'
                    "\t}; \\\n"
                    "\trun_suite qqalpha ze-test qqalpha --all; \\\n"
                    "\techo done\n"
                )
            with self.assertRaises(R.ParseError) as cm:
                R.functional_suites(path)
        msg = str(cm.exception)
        self.assertIn("qqbeta", msg)
        self.assertNotIn("qqalpha", msg)
        self.assertIn("run_suite", msg)

    def test_a_fully_dispatched_suite_list_is_accepted(self):
        """Discriminates from 'always raises', and pins that the shell function definition
        `run_suite() {` is not read as a dispatch of a suite called `()`."""
        with _scratch() as root:
            os.makedirs(os.path.join(root, "mk"))
            path = os.path.join(root, "mk", "test-functional.mk")
            with open(path, "w", encoding="utf-8") as fh:
                fh.write(
                    "ze-functional-test:\n"
                    '\tall_suites="qqalpha qqbeta"; \\\n'
                    "\trun_suite() { \\\n"
                    '\t\t"$$@"; \\\n'
                    "\t}; \\\n"
                    "\trun_suite qqalpha ze-test qqalpha --all; \\\n"
                    "\trun_suite qqbeta ze-test qqbeta --all\n"
                )
            self.assertEqual(R.functional_suites(path), ("qqalpha", "qqbeta"))

    def test_the_repo_dispatches_every_suite_it_declares(self):
        """The other half of the pin: the refusal above is only reachable if the real
        recipe dispatches everything today, so that is asserted against the real file."""
        raw = _read_repo("mk/test-functional.mk")
        declared = R._ALL_SUITES_RE.findall(raw)[0].split()
        dispatched = set(R._RUN_SUITE_RE.findall(raw))
        self.assertEqual([s for s in declared if s not in dispatched], [])

    def test_refusal_message_is_grammatical(self):
        """The catch-all's message used to read '... -- no declared runner has no
        automated caller'. It is the message most likely to be hit, so it is the one that
        must parse as English (ai/rules/cli.md)."""
        for c in R.CARRIERS:
            if c.tier != R.TIER_UNRUN:
                continue
            msg = str(R._refuse_unrun(c, R.Tag("RFC7606-1-1", "positive", "f.ci", 1)))
            self.assertNotIn(" has no automated caller", msg, c.name)
            self.assertIn(f"runner: {c.runner}", msg, c.name)
            self.assertIn(f"pipeline: {c.pipeline}", msg, c.name)


_SCHEDULED_WF = """\
name: evidence-nightly

on:
  schedule:
    - cron: "17 3 * * *"
  workflow_dispatch:

jobs:
  interop:
    runs-on: ubuntu-latest
    continue-on-error: true
    steps:
      - name: make ze-interop-test
        run: make ze-interop-test

  ipsec-interop:
    runs-on: ubuntu-latest
    continue-on-error: true
    steps:
      - name: make ze-interop-ipsec-test
        run: make ze-interop-ipsec-test
"""

_PUSH_ONLY_WF = """\
name: verify

on:
  push:
  pull_request:

jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - run: make ze-interop-ipsec-test
"""


@contextlib.contextmanager
def _workflows(**files):
    """A scratch .github/workflows/ holding the named files, each given a .yml suffix."""
    with _scratch() as root:
        d = os.path.join(root, ".github", "workflows")
        os.makedirs(d)
        for name, body in files.items():
            with open(os.path.join(d, f"{name}.yml"), "w", encoding="utf-8") as fh:
                fh.write(body)
        yield d


class TestInteropTierIsDerivedFromWorkflows(unittest.TestCase):
    """The interop tier used to be four literals: `interop-bgp` asserted `nightly` and the
    other three asserted `unrun`, and NOTHING tied either claim to a pipeline. Deleting the
    nightly interop job changed no byte of CARRIERS, so the gate would have kept crediting
    `interop/nightly` to a job that no longer existed.
    """

    def test_scheduled_workflow_targets_reads_make_targets(self):
        with _workflows(nightly=_SCHEDULED_WF) as d:
            found = R.scheduled_workflow_targets(d)
        self.assertIn("ze-interop-test", found)
        self.assertIn("ze-interop-ipsec-test", found)
        self.assertEqual(found["ze-interop-ipsec-test"], "nightly.yml")

    def test_scheduled_workflow_targets_ignores_push_only_workflow(self):
        """A target named ONLY by a push/pull_request workflow grants no nightly tier.
        Discriminates from 'any mention of the target counts'."""
        with _workflows(verify=_PUSH_ONLY_WF) as d:
            self.assertEqual(R.scheduled_workflow_targets(d), {})

    def test_scheduled_workflow_targets_ignores_comments(self):
        """Matches stripComments on the Go side: a commented-out command is not a caller."""
        body = _SCHEDULED_WF.replace(
            "        run: make ze-interop-ipsec-test",
            "        # run: make ze-interop-ipsec-test",
        )
        with _workflows(nightly=body) as d:
            found = R.scheduled_workflow_targets(d)
        self.assertIn("ze-interop-test", found)
        self.assertNotIn("ze-interop-ipsec-test", found)

    def test_scheduled_workflow_targets_fails_closed_when_unreadable(self):
        """The unsafe direction is answering 'everything runs': that would upgrade every
        carrier on an error. An unreadable directory raises instead
        (ai/rules/evidence.md)."""
        with _scratch() as root:
            with self.assertRaises(R.ParseError):
                R.scheduled_workflow_targets(os.path.join(root, "nope"))

    def test_scheduled_workflow_targets_fails_closed_when_empty(self):
        with _workflows() as d:
            with self.assertRaises(R.ParseError):
                R.scheduled_workflow_targets(d)

    def test_ipsec_interop_carrier_earns_nightly_when_wired(self):
        c = R.carrier_for("test/interop-ipsec/scenarios/04-eap-tls/check.py")
        self.assertIsNotNone(c)
        self.assertEqual(c.name, "interop-ipsec")
        self.assertEqual(c.tier, R.TIER_NIGHTLY)
        self.assertEqual(c.label, "interop/nightly")

    def test_interop_carrier_falls_to_unrun_without_a_scheduled_caller(self):
        """The same tree resolves `unrun` when no scheduled workflow names its runner, and
        the refusal names the runner so the fix is actionable."""
        rows = R._interop_carriers("/check.py", {})
        by_name = {c.name: c for c in rows}
        for name in ("interop-bgp", "interop-ipsec", "interop-l2tp", "interop-pppoe"):
            self.assertEqual(by_name[name].tier, R.TIER_UNRUN, name)
        tag = R.Tag(
            rid="RFC7296-1-1",
            polarity="positive",
            file="test/interop-ipsec/scenarios/x/check.py",
            line=3,
        )
        msg = str(R._refuse_unrun(by_name["interop-ipsec"], tag))
        self.assertIn("make ze-interop-ipsec-test", msg)
        self.assertIn("nothing executes automatically", msg)

    def test_l2tp_and_pppoe_stay_unrun_today(self):
        """Their labs need host kernel modules that no scheduled runner is confirmed to
        provide, so no workflow names their runners and the tier derivation refuses them.
        The pin is on the DERIVATION's answer, not on a literal."""
        for rel in (
            "test/interop-l2tp/scenarios/x/check.py",
            "test/interop-pppoe/scenarios/x/check.py",
        ):
            self.assertEqual(R.carrier_for(rel).tier, R.TIER_UNRUN, rel)

    def test_deleting_the_interop_job_downgrades_the_bgp_carrier(self):
        """AC-6. Today this deletion changes nothing; after the derivation it takes the
        tier away, which is what makes the ratchet see a LOSS."""
        body = _SCHEDULED_WF.split("  ipsec-interop:")[0].replace(
            "        run: make ze-interop-test", "        run: echo nothing"
        )
        with _workflows(nightly=body) as d:
            found = R.scheduled_workflow_targets(d)
        self.assertNotIn("ze-interop-test", found)
        self.assertEqual(
            {c.name: c.tier for c in R._interop_carriers("/check.py", found)}[
                "interop-bgp"
            ],
            R.TIER_UNRUN,
        )

    def test_head_carriers_read_head_workflows(self):
        """_build_head_carriers must label from HEAD's workflow set. Labelling both sides
        with today's table would make a job deletion symmetric, so evidence that stopped
        running would read as a wash rather than a loss."""
        head = {c.name: c for c in R._head_carriers() if c.kind == "interop"}
        self.assertTrue(head, "no interop rows in the HEAD table")
        # Every HEAD interop row must be justified by HEAD's own workflows, not by today's.
        sources = R._head_workflow_sources()
        if sources is None:
            self.skipTest("HEAD workflows unreadable (shallow checkout or no git)")
        scheduled = R._scheduled_targets_from(sources)
        for name, prefix, target in R.INTEROP_TREES:
            want = R.TIER_NIGHTLY if scheduled.get(target) else R.TIER_UNRUN
            self.assertEqual(head[name].tier, want, name)

    def test_no_interop_tier_literal_survives(self):
        """`ai/rules/evidence.md`: the workflow reader is the ONLY place an
        interop tier is decided. INTEROP_TREES carries the path and the runner; a tier
        constant beside them would be the literal coming back."""
        for row in R.INTEROP_TREES:
            self.assertEqual(len(row), 3, row)
            self.assertNotIn(R.TIER_NIGHTLY, row)
            self.assertNotIn(R.TIER_UNRUN, row)


class TestTierMatchesPipelineReality(unittest.TestCase):
    """F4: the tier table is a claim about OTHER files, so it is pinned to their text.

    Source-text assertions in the style of internal/test/runner/draft_dir_test.go's
    TestDraftDirIsInvisibleToRepoGates: cheaper and more honest than materializing a fake
    repo, and they red the moment the claim stops being true.
    """

    def test_every_verify_tier_carrier_names_a_real_verify_stage(self):
        """A declared verify row names its OWN stage; a derived row rides the functional
        stage, so what must be pinned there is that the functional stage exists at all."""
        verify_src = _read_repo("scripts/status/verify_run.go")
        for c in R.CARRIERS:
            if c.tier != R.TIER_VERIFY:
                continue
            if c.derived:
                self.assertIn(
                    'mk("ze-functional-test")',
                    verify_src,
                    f"carrier {c.name} rides the functional stage, which stagesForMode "
                    f"no longer runs",
                )
                continue
            target = _make_target(c.runner)
            stages = _verify_stages(verify_src)
            # Prefix, not equality: ze-precommit-verify splits the unit run into ze-unit-test-cached
            # and ze-unit-test-race-changed (full) or ze-unit-test-changed (changed mode),
            # while `make ze-unit-test` is the whole-suite target a human runs. What must
            # hold is that SOME verify stage runs this target's work.
            self.assertTrue(
                any(s == target or s.startswith(target + "-") for s in stages),
                f"carrier {c.name} claims tier '{R.TIER_VERIFY}' but no stagesForMode "
                f"stage runs {target}; stages are {sorted(stages)}",
            )

    def test_every_derived_suite_is_a_token_in_the_makefile_suite_list(self):
        """The text pin. Token-exact, not substring: `ospf` is a prefix of `ospfv3` and
        `l2tp` of `l2tp-wire`, so a substring test would pass a suite that is not run."""
        raw = _read_repo("mk/test-functional.mk")
        line = next(ln for ln in raw.splitlines() if "all_suites=" in ln)
        tokens = set(line.split('"')[1].split())
        self.assertGreater(len(tokens), 5)
        derived = [c for c in R.CARRIERS if c.derived]
        self.assertGreater(
            len(derived), 5, "no derived suite rows: the parse found nothing"
        )
        for c in derived:
            suite = c.prefix.split("/")[1]
            self.assertIn(
                suite,
                tokens,
                f"carrier {c.name} claims a ze-precommit-verify functional suite that "
                f"mk/test-functional.mk does not run",
            )

    def test_the_nightly_carrier_names_a_job_the_nightly_workflow_runs(self):
        wf = _read_repo(".github/workflows/evidence-nightly.yml")
        for c in R.CARRIERS:
            if c.tier != R.TIER_NIGHTLY:
                continue
            target = _make_target(c.runner)
            self.assertIn(
                target,
                wf,
                f"carrier {c.name} claims tier '{R.TIER_NIGHTLY}' but {target} is not in "
                f"the nightly workflow",
            )

    def test_no_unrun_carrier_is_secretly_wired(self):
        """The other side of the ratchet: an unrun row that HAS become automated is a
        stale refusal, and a stale refusal blocks evidence that is now real."""
        verify_src = _read_repo("scripts/status/verify_run.go")
        wf = _read_repo(".github/workflows/evidence-nightly.yml")
        for c in R.CARRIERS:
            if c.tier != R.TIER_UNRUN:
                continue
            target = _make_target(c.runner)
            if not target:
                continue
            self.assertNotIn(f'mk("{target}")', verify_src, c.name)
            self.assertNotIn(target, wf, c.name)


class TestTagMarkerPrefilter(unittest.TestCase):
    """F5: TAG_MARKER's comment claimed it was the cheap pre-filter; nothing used it."""

    def test_tag_marker_gates_the_expensive_scan(self):
        """A check.py that cannot be tokenized is a hard ParseError -- unless it holds no
        marker at all, in which case it is skipped before the tokenizer ever sees it. That
        asymmetry is the pre-filter, observable."""
        with _scratch() as root:
            d = os.path.join(root, "test", "interop", "scenarios", "01-x")
            os.makedirs(d)
            with open(os.path.join(d, "check.py"), "w", encoding="utf-8") as fh:
                fh.write("def broken(:\n")  # unparseable
            self.assertEqual(R.scan_tree(root), [])

            with open(os.path.join(d, "check.py"), "w", encoding="utf-8") as fh:
                fh.write(
                    "# RFC requirement: RFC7606-1-1 positive -- note\ndef broken(:\n"
                )
            with self.assertRaises(R.ParseError):
                R.scan_tree(root)

    def test_the_git_baseline_reuses_the_marker_constant(self):
        src = _read_repo("scripts/dev/rfc_requirements.py")
        body = src.split("def _read_git_baseline_tags", 1)[1].split("\ndef ", 1)[0]
        self.assertNotIn(
            '"RFC requirement:"',
            body,
            "_read_git_baseline_tags re-spells TAG_MARKER instead of reading it",
        )
        self.assertIn("TAG_MARKER", body)


class TestNightlyRollupProse(unittest.TestCase):
    """F8: the rollup counted a nightly-only requirement in Both AND in Nightly-only while
    the prose said adding them to Both would misrepresent nightly evidence."""

    def test_prose_does_not_contradict_the_counting(self):
        prose = "\n".join(
            R._render_rollup(
                {"rfc7606": [_req("RFC7606-2-1")]},
                {},
                {"rfc7606"},
            )
        )
        self.assertNotIn("adding them to **Both** would present", prose)
        self.assertIn("polarity view", prose)


class TestRealTree(unittest.TestCase):
    def test_all_summaries_parse_or_report(self):
        """Every rfc/short/*.md must parse. Pre-migration, lines lack IDs — those are
        reported as errors, never silently skipped."""
        root = os.path.join(_HERE, "..", "..")
        d = os.path.join(root, "rfc", "short")
        self.assertTrue(os.path.isdir(d))
        n = 0
        for name in sorted(os.listdir(d)):
            if not name.endswith(".md"):
                continue
            n += 1
        self.assertGreater(n, 150)


# --------------------------------------------------------------------------
# Audit teeth (plan/spec-rfcgate-3-audit-teeth.md)
# --------------------------------------------------------------------------
# The verdict field was written by a skill and read by NOTHING: the freshness rule compared the
# requirement sha and the tests map and never looked at the value, so a `weak` verdict -- which
# ai/skills/ze-rfc-audit.md calls one of its two valuable outputs -- was treated exactly like
# `enforced`. These classes are the reader.


@contextlib.contextmanager
def _audit_tree(files=None, status=None):
    """A temp rfc/audit/ plus a temp docs/features/rfc-status.md.

    `files` maps stem -> dict (dumped as JSON) or str (written verbatim, for the malformed-input
    cases). `status` is the raw status-ledger text; the default discloses a gap for rfc9999 so a
    disclosure test has to opt INTO the clean-support row rather than inherit it.
    """
    tmp = _mkdtemp("ze-audit-")
    adir = os.path.join(tmp, "audit")
    os.makedirs(adir)
    for stem, body in (files or {}).items():
        with open(os.path.join(adir, stem + ".json"), "w", encoding="utf-8") as fh:
            fh.write(body if isinstance(body, str) else json.dumps(body, indent=2))
    spath = os.path.join(tmp, "rfc-status.md")
    with open(spath, "w", encoding="utf-8") as fh:
        fh.write(status if status is not None else _STATUS_DISCLOSED)
    try:
        with _patched(AUDIT_DIR=adir, STATUS_FILE=spath):
            yield adir
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


_STATUS_HEAD = (
    "| RFC | Area | Status | Coverage | Remaining |\n"
    "|-----|------|--------|----------|-----------|\n"
)
_STATUS_DISCLOSED = (
    _STATUS_HEAD + "| RFC 9999 | x | Partial | cov | Section 2 is unmet. |\n"
)
_STATUS_CLEAN = _STATUS_HEAD + "| RFC 9999 | x | Supported | cov | No tracked gap. |\n"


def _go_fixture(
    root, name, tag_rid="RFC9999-2-1", body="\trequire.Equal(t, 1, one())\n"
):
    """A tagged Go test file on disk, returned as a repo-relative path.

    Repo-relative and under the project tmp/ because the fingerprint helpers resolve against
    PROJECT_DIR: a fixture outside the tree could not be read, and ai/rules/testing.md makes the
    project tmp/ the only allowed scratch home.
    """
    path = os.path.join(root, name)
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(
            "package x\n"
            "\n"
            "func helperUntagged(t *testing.T) {\n\trequire.Equal(t, 2, two())\n}\n"
            "\n"
            f"// RFC requirement: {tag_rid} positive -- one() returns one.\n"
            "func TestTagged(t *testing.T) {\n" + body + "}\n"
        )
    return os.path.relpath(path, R.PROJECT_DIR).replace(os.sep, "/")


def _fixture_tags(rid, rel, line):
    """The two tags `_AuditFixture` puts on one requirement: one per polarity, one site.

    One definition, shared with `_verdict`, so the recorded verdict and the computed reading are
    never taken over different tag lists.
    """
    return [
        _tag(rid, "positive", file=rel, line=line),
        _tag(rid, "negative", file=rel, line=line),
    ]


def _verdict(
    req,
    rel,
    line,
    value="enforced",
    note="require.Equal pins one()",
    tags=None,
    **extra,
):
    """A well-formed verdict over the fixture's tagged test, with both fingerprint maps recorded.

    The keys are MINTED by the gate rather than spelled here. A fixture that spelled its own key
    would be the second definition of the key form, and the migration off `<path>:<line>` is
    exactly what a second definition would have hidden.

    It records BOTH of `_fixture_tags` by default, because that is what the gate computes for
    this requirement and a verdict citing fewer tags than exist is STALE_UNIT by design. The old
    `<path>:<line>` key hid the difference: two tags on one line collapsed into one key, so a
    one-tag verdict compared equal to a two-tag reading. A caller whose requirement carries ONE
    tag (a `{single-polarity}` annotation) passes that tag instead.
    """
    tags = _fixture_tags(req.rid, rel, line) if tags is None else tags
    out = {
        "verdict": value,
        "note": note,
        "requirement_sha": R.requirement_sha(req.text),
        "tests": R.tagged_unit_shas(tags),
        "units": R.unit_shas(R.tag_keys(tags)),
    }
    out.update(extra)
    return out


def _audit_file(stem, verdicts):
    return {
        "rfc": stem,
        "audited": "2026-07-29",
        "requirements": verdicts,
    }


class _AuditFixture(unittest.TestCase):
    """One requirement, one tagged Go test on disk, one recorded verdict.

    Shared because every audit check needs the same three things to exist and agree before it
    can be shown to fire on one of them being wrong.
    """

    def setUp(self):
        self.tmp = _mkdtemp("ze-auditfx-")
        self.addCleanup(shutil.rmtree, self.tmp, ignore_errors=True)
        self.req = _req("RFC9999-2-1", rfc="rfc9999")
        self.rel = _go_fixture(self.tmp, "a_test.go")
        # Line 7 is the `// RFC requirement:` comment, which is INSIDE TestTagged's span (a doc
        # comment belongs to the func below it). Line 6 is the blank line before it and sits
        # outside every span, so a fixture pointing there silently resolves to FILE scope and
        # every unit-versus-file assertion below reads as passing while proving nothing.
        self.line = 7
        self.tags = _fixture_tags(self.req.rid, self.rel, self.line)
        # The keys the gate mints for those tags. Spelled nowhere in this module: the key form is
        # the thing under test in several classes below, and a fixture that spelled it would
        # assert the two spellings agree with each other rather than with the gate.
        self.keys = R.tag_keys(self.tags)
        self.key = self.keys[0]
        # `helperUntagged`, the fixture's OTHER top-level func: a producer to cite in a `code`
        # map, chosen because it is not the tagged unit.
        self.code_key = f"{self.rel}::helperUntagged"

    def verdict(self, value="enforced", **extra):
        return _verdict(self.req, self.rel, self.line, value=value, **extra)

    def audits(self, verdict=None, value="enforced", **extra):
        v = verdict if verdict is not None else self.verdict(value=value, **extra)
        return {"rfc9999": {self.req.rid: v}}

    def schema(self, verdict=None, reqs=None, tags=None, **kw):
        return R.check_audit_schema(
            reqs if reqs is not None else [self.req],
            tags if tags is not None else self.tags,
            {"rfc9999"},
            self.audits(verdict, **kw),
        )


class TestAuditSchema(_AuditFixture):
    """`load_audit` was a bare json.load returning `data.get("requirements", {})`: no field
    check, no enum check, no type check, and every other top-level key silently discarded. The
    vocabulary had already drifted to a fifth value and nothing noticed, because nothing looked.
    """

    def _load(self, verdicts, stem="rfc9999", raw=None):
        body = raw if raw is not None else _audit_file(stem, verdicts)
        with _audit_tree(files={stem: body}):
            return R.load_audit(stem)

    def test_unknown_verdict_value_fails(self):
        """AC-1, with the live `implemented` value as the fixture."""
        v = self.verdict()
        v["verdict"] = "implemented"
        with self.assertRaises(R.ParseError) as cm:
            self._load({self.req.rid: v})
        msg = str(cm.exception)
        self.assertIn(self.req.rid, msg)
        self.assertIn("implemented", msg)
        for legal in R.AUDIT_VERDICTS:
            self.assertIn(legal, msg, "the message must name the legal set")

    def test_missing_required_field_fails(self):
        """AC-2: one case per required field. An absent authored field is never a default."""
        for field in ("verdict", "note", "requirement_sha"):
            v = self.verdict()
            del v[field]
            with self.assertRaises(R.ParseError, msg=field) as cm:
                self._load({self.req.rid: v})
            self.assertIn(self.req.rid, str(cm.exception))

    def test_unknown_key_in_verdict_fails(self):
        """AC-2: a typo'd field would otherwise read as an ABSENT field and pass silently."""
        v = self.verdict()
        v["notez"] = "typo"
        with self.assertRaises(R.ParseError) as cm:
            self._load({self.req.rid: v})
        self.assertIn("notez", str(cm.exception))

    def test_wrong_type_fails_closed_not_as_a_traceback(self):
        """Security Review, Input validation: a string where a map belongs must be a clean
        error. It used to flow into an equality comparison and report as STALE -- a real defect
        wearing the costume of a routine re-read."""
        v = self.verdict()
        v["tests"] = "internal/a_test.go:1"
        with self.assertRaises(R.ParseError) as cm:
            self._load({self.req.rid: v})
        self.assertIn("must be an object", str(cm.exception))

    def test_fingerprint_key_may_not_escape_the_tree(self):
        """Security Review, Path handling: `tests` keys become open() calls, and a verdict is
        agent-authored input, not a trusted path source.

        Every shape the key form allows is covered, because the refusal must not depend on which
        half of the key is present: symbol-scoped, bare path, and the retired `:<line>` form that
        a stale record could still carry."""
        for bad in (
            "/etc/passwd::Read",
            "../../etc/passwd::Read",
            "~/x.go::Read",
            "../escape.go",
            "/etc/passwd",
            "~/x.go",
            "/etc/passwd:1",
            "../../etc/passwd:1",
        ):
            v = self.verdict()
            v["tests"] = {bad: "a" * 16}
            with self.assertRaises(R.ParseError, msg=bad) as cm:
                self._load({self.req.rid: v})
            self.assertIn(
                "outside the repository",
                str(cm.exception),
                f"{bad} must be refused for its PATH, whatever else is wrong with it",
            )

    def test_a_retired_location_key_is_refused_by_name(self):
        """The migration off `<path>:<line>` must not read as a mystery. A record still carrying
        a location key names the fault and the two legal shapes."""
        v = self.verdict()
        v["tests"] = {f"{self.rel}:7": "a" * 16}
        with self.assertRaises(R.ParseError) as cm:
            self._load({self.req.rid: v})
        msg = str(cm.exception)
        self.assertIn("retired", msg)
        self.assertIn(f"{self.rel}::<FuncName>", msg)

    def test_a_bare_path_key_is_legal(self):
        """A `.ci`, an `.et` and an interop `check.py` have no function boundary, and a Go tag
        outside every span resolves to file scope. The bare path IS that declaration, so it must
        parse rather than look like a truncated key."""
        self.assertEqual(R._fingerprint_key("test/x.ci", "w"), ("test/x.ci", None))
        self.assertEqual(
            R._fingerprint_key("a/b_test.go::TestFoo", "w"), ("a/b_test.go", "TestFoo")
        )
        self.assertEqual(
            R._fingerprint_key("a/b_test.go::TestFoo#2", "w"),
            ("a/b_test.go", "TestFoo"),
            "the ordinal resolves to the same unit",
        )

    def test_top_level_typo_is_not_discarded(self):
        """Everything except `requirements` used to be dropped without inspection, so a
        misspelled `reaudit_note` silently deleted the record of why a verdict was trusted."""
        body = _audit_file("rfc9999", {self.req.rid: self.verdict()})
        body["reaudit_notes"] = "typo"
        with self.assertRaises(R.ParseError) as cm:
            self._load(None, raw=body)
        self.assertIn("reaudit_notes", str(cm.exception))

    def test_rfc_field_must_match_the_filename(self):
        body = _audit_file("rfc7606", {self.req.rid: self.verdict()})
        with self.assertRaises(R.ParseError) as cm:
            self._load(None, stem="rfc9999", raw=body)
        self.assertIn("filename", str(cm.exception))

    def test_malformed_json_still_fails_closed(self):
        """Preserved behavior: a syntax error is a clean exit-2 message, not a traceback."""
        with self.assertRaises(R.ParseError) as cm:
            self._load(None, raw="{not json")
        self.assertIn("cannot read", str(cm.exception))

    def test_missing_file_is_legal_and_empty(self):
        """Preserved behavior: the audit is sampled, so an absent file is normal."""
        with _audit_tree():
            self.assertEqual(R.load_audit("rfc9999"), {})

    def test_verdict_for_unknown_rid_fails(self):
        """AC-3: the direction check_audit_freshness never walked. It iterates REQUIREMENTS and
        asks each for its verdict, so a verdict for an id that does not exist was read by
        nothing and reported by nothing."""
        errs = R.check_audit_schema(
            [self.req],
            self.tags,
            {"rfc9999"},
            {"rfc9999": {"RFC9999-9-9": self.verdict()}},
        )
        self.assertTrue(errs)
        self.assertIn("RFC9999-9-9", " ".join(errs))

    def test_audit_file_for_unenrolled_stem_fails(self):
        """AC-4: an audit file nothing loads is evidence that does not exist."""
        with _audit_tree(files={"rfc9999": _audit_file("rfc9999", {})}):
            errs = R.check_audit_files(set(), {"rfc9999"})
            self.assertTrue(errs)
            self.assertIn("rfc/audit/rfc9999.json", errs[0])
            self.assertIn("enrolled", errs[0])

    def test_audit_file_with_no_summary_fails(self):
        """AC-4, the other half: judgements about requirements that do not exist."""
        with _audit_tree(files={"rfc9999": _audit_file("rfc9999", {})}):
            errs = R.check_audit_files({"rfc9999"}, set())
            self.assertTrue(errs)
            self.assertIn("rfc/short/rfc9999.md", errs[0])

    def test_audit_file_that_is_enrolled_and_summarised_passes(self):
        """The discriminating twin: AC-4 is not "always fails"."""
        with _audit_tree(files={"rfc9999": _audit_file("rfc9999", {})}):
            self.assertEqual(R.check_audit_files({"rfc9999"}, {"rfc9999"}), [])

    def test_enforced_with_empty_tests_fails(self):
        """AC-5: "proven" with no cited test is a contradiction, not a weaker claim."""
        v = self.verdict()
        v["tests"] = {}
        v["units"] = {}
        errs = self.schema(v)
        self.assertTrue(errs)
        joined = " ".join(errs)
        self.assertIn("empty 'tests'", joined)
        # The error must point at the honest alternative, or the only way out looks like a lie.
        self.assertIn(R.VERDICT_NOT_APPLICABLE, joined)

    def test_enforced_needs_both_polarities(self):
        """AC-6: a negative-only test passes if the code rejects everything, and a positive-only
        one passes if it accepts everything. Only the pair pins behaviour."""
        errs = self.schema(tags=[self.tags[0]])
        self.assertTrue(errs)
        self.assertIn("negative", " ".join(errs))

    def test_single_polarity_annotation_exempts_it(self):
        """AC-6's negative half: the exemption is the annotation, and it works."""
        ann = R.Annotation(kind="single-polarity", polarity="positive", reason="why")
        req = _req("RFC9999-2-1", rfc="rfc9999", annotation=ann)
        errs = R.check_audit_schema([req], [self.tags[0]], {"rfc9999"}, self.audits())
        self.assertEqual(errs, [])

    def test_unimplemented_needs_code_map(self):
        """AC-7: with neither tests nor code fingerprinted the freshness test reduces to
        `{} == {}`, so the verdict can never go stale and nobody is ever asked to look again."""
        ann = R.Annotation(kind="gap", polarity=None, reason="deliberate")
        req = _req("RFC9999-2-1", rfc="rfc9999", annotation=ann)
        v = self.verdict(value="unimplemented")
        v["tests"], v["units"] = {}, {}
        errs = R.check_audit_schema([req], [], {"rfc9999"}, {"rfc9999": {req.rid: v}})
        self.assertTrue(errs)
        self.assertIn("'code' map", " ".join(errs))

    def test_unimplemented_needs_gap_annotation(self):
        """AC-7's other half: a divergence Ze knows about must be declared where a reader of the
        summary will meet it, not only in an audit file."""
        v = self.verdict(value="unimplemented")
        v["tests"], v["units"] = {}, {}
        v["code"] = R.unit_shas([self.code_key])
        errs = R.check_audit_schema(
            [self.req], [], {"rfc9999"}, {"rfc9999": {self.req.rid: v}}
        )
        self.assertTrue(errs)
        self.assertIn("{gap}", " ".join(errs))

    def test_unimplemented_with_code_and_gap_passes(self):
        """AC-7's discriminating twin."""
        ann = R.Annotation(kind="gap", polarity=None, reason="deliberate")
        req = _req("RFC9999-2-1", rfc="rfc9999", annotation=ann)
        v = self.verdict(value="unimplemented")
        v["tests"], v["units"] = {}, {}
        v["code"] = R.unit_shas([self.code_key])
        self.assertEqual(
            R.check_audit_schema([req], [], {"rfc9999"}, {"rfc9999": {req.rid: v}}), []
        )


class TestNotApplicableVerdict(_AuditFixture):
    """Owner ruling OR-1 (Thomas, 2026-07-29).

    RFC7606-8-1 was `enforced` with an empty `tests` map on a `{not-applicable}` requirement, and
    the schema this spec writes left it NO legal state: AC-5 refuses `enforced` with no cited
    test, and AC-7's `code`-map remedy is open only to `unimplemented`. Thomas added a state
    rather than have the record re-judged. It must not become the cheap escape from AC-5, so it
    costs two committed facts where `enforced` costs one word.
    """

    def _na(self, req, **extra):
        v = self.verdict(value=R.VERDICT_NOT_APPLICABLE, **extra)
        v["tests"], v["units"] = {}, {}
        return {"rfc9999": {req.rid: v}}

    def _req_na(self):
        ann = R.Annotation(kind="not-applicable", polarity=None, reason="binds authors")
        return _req("RFC9999-2-1", rfc="rfc9999", annotation=ann)

    def test_legal_with_a_reason_and_an_agreeing_annotation(self):
        req = self._req_na()
        errs = R.check_audit_schema(
            [req],
            [],
            {"rfc9999"},
            self._na(req, no_code_path="section 8 binds authors"),
        )
        self.assertEqual(errs, [])

    def test_a_bare_state_is_refused(self):
        """OR-1: a verdict whose only content is its own name is the unfalsifiable entry the
        schema exists to reject."""
        req = self._req_na()
        errs = R.check_audit_schema([req], [], {"rfc9999"}, self._na(req))
        self.assertTrue(errs)
        self.assertIn("no_code_path", " ".join(errs))

    def test_an_empty_reason_is_refused_too(self):
        """A whitespace reason is the same escape one space wider."""
        req = self._req_na()
        errs = R.check_audit_schema(
            [req], [], {"rfc9999"}, self._na(req, no_code_path="   ")
        )
        self.assertTrue(errs)
        self.assertIn("no_code_path", " ".join(errs))

    def test_the_annotation_must_agree(self):
        """OR-1: two committed facts. The audit record cannot reclassify a requirement alone."""
        errs = R.check_audit_schema(
            [self.req],
            [],
            {"rfc9999"},
            self._na(self.req, no_code_path="because"),
        )
        self.assertTrue(errs)
        self.assertIn("not-applicable", " ".join(errs))

    def test_a_gap_annotation_does_not_satisfy_it(self):
        """{gap} says Ze could comply and does not; {not-applicable} says nothing could. Reading
        one as the other is how a gap would launder itself into unreachability."""
        ann = R.Annotation(kind="gap", polarity=None, reason="deliberate")
        req = _req("RFC9999-2-1", rfc="rfc9999", annotation=ann)
        errs = R.check_audit_schema(
            [req], [], {"rfc9999"}, self._na(req, no_code_path="because")
        )
        self.assertTrue(errs)

    def test_citing_a_test_is_refused(self):
        """If a test can exercise it, a reachable code path exists."""
        req = self._req_na()
        audits = self._na(req, no_code_path="because")
        audits["rfc9999"][req.rid]["tests"] = R.tagged_unit_shas(
            [_tag(req.rid, "positive", file=self.rel, line=self.line)]
        )
        errs = R.check_audit_schema([req], self.tags, {"rfc9999"}, audits)
        self.assertTrue(errs)
        self.assertIn("cites tests", " ".join(errs))

    def test_ac5_stays_strict(self):
        """OR-1's own constraint: the new state is the honest ALTERNATIVE to abusing `enforced`,
        never a relaxation of AC-5. Enforced with no tests is still a hard failure."""
        req = self._req_na()
        v = self.verdict()
        v["tests"], v["units"] = {}, {}
        errs = R.check_audit_schema([req], [], {"rfc9999"}, {"rfc9999": {req.rid: v}})
        self.assertTrue(errs)
        self.assertIn("empty 'tests'", " ".join(errs))

    def test_no_code_path_may_not_sit_on_another_verdict(self):
        """A field that means one thing may only appear where it means it: unread anywhere else,
        an author can believe they filled it in."""
        v = self.verdict()
        v["no_code_path"] = "why"
        with _audit_tree(files={"rfc9999": _audit_file("rfc9999", {self.req.rid: v})}):
            with self.assertRaises(R.ParseError) as cm:
                R.load_audit("rfc9999")
        self.assertIn("no_code_path", str(cm.exception))


class TestFingerprintShapeBoundary(_AuditFixture):
    """Boundary row 1: a recorded fingerprint is exactly `SHA_HEX_LEN` lowercase hex characters.

    The schema used to accept ANY non-empty string here. That was fail-closed rather than unsound
    -- a malformed sha compares unequal to the computed value and resolves to STALE_UNIT -- but
    STALE is the wrong diagnosis for a typo: it sends a reader to re-audit a judgement that never
    moved, and prints a remediation that does not name the fault.

    Driven through `load_audit`, the entry point an authored record actually arrives by, not
    through `_sha_value` alone (`ai/rules/evidence.md`: drive the guard from its entry
    point). The four cases per field are the boundary trio plus the one a pure length check would
    wave through.
    """

    def test_the_pattern_itself_rejects_a_trailing_newline_under_search(self):
        """`_SHA_RE` ends `\\Z`, not `$`, and nothing else in this class would notice a revert.

        Every other case here drives `load_audit` -> `_sha_value`, which uses `fullmatch` and is
        therefore correct under EITHER anchor. So the anchor is load-bearing only for a
        `search`/`match` caller, and an anchored `search` is equivalent to `match`, not to
        `fullmatch`: with `$`, `re.search(pattern, "a"*16 + "\\n")` MATCHES, which is the exact
        17-character hole `fullmatch` was introduced to close. A comment claimed the pattern was
        "safe for a `search`-based caller" while that was untrue (third review pass, NOTE 5).

        Pinned at the pattern rather than through an entry point on purpose: the property belongs
        to the regex, and the guard exists so that swapping `\\Z` back to `$` cannot pass silently.
        """
        self.assertIsNone(R._SHA_RE.search("a" * R.SHA_HEX_LEN + "\n"))
        # The discriminating twin: the anchor must still accept the shape it is meant to accept,
        # or the assertion above would also pass on a pattern that matches nothing at all.
        self.assertIsNotNone(R._SHA_RE.search("a" * R.SHA_HEX_LEN))

    # 16 lowercase hex is the only accepted shape. `"g"` is not hex; `"A"` is hex but uppercase,
    # which is what a case-insensitive check would let through; both are the RIGHT LENGTH, so
    # neither is caught by a length test alone.
    BAD = {
        "one short (15)": "a" * (R.SHA_HEX_LEN - 1),
        "one long (17)": "a" * (R.SHA_HEX_LEN + 1),
        "non-hex at the right length": "g" * R.SHA_HEX_LEN,
        "uppercase hex at the right length": "A" * R.SHA_HEX_LEN,
        # The subtlest "one long": Python's `$` matches immediately BEFORE a final newline, so a
        # `match`-based check accepts this 17-character value while reporting the shape as
        # enforced. Found by adversarial self-review of the guard, and the reason it uses
        # `fullmatch`.
        "one long via a trailing newline": "a" * R.SHA_HEX_LEN + "\n",
    }

    def _load(self, verdict):
        with _audit_tree(
            files={"rfc9999": _audit_file("rfc9999", {self.req.rid: verdict})}
        ):
            return R.load_audit("rfc9999")

    def test_the_producers_emit_the_shape_the_schema_demands(self):
        """The two must agree by construction, or the gate rejects its own output."""
        for produced in (R.requirement_sha("some text"), R.test_sha("func F() {}\n")):
            self.assertEqual(len(produced), R.SHA_HEX_LEN, produced)
            self.assertRegex(produced, r"^[0-9a-f]+$", produced)

    def test_last_valid_is_accepted(self):
        """The discriminating twin: the check must not reject the legal shape. The fixture's own
        fingerprints come from the producers, so this also pins producer-schema agreement."""
        loaded = self._load(self.verdict())
        self.assertIn(self.req.rid, loaded)

    def test_a_malformed_requirement_sha_is_refused(self):
        for name, bad in self.BAD.items():
            v = self.verdict()
            v["requirement_sha"] = bad
            with self.assertRaises(R.ParseError, msg=name) as cm:
                self._load(v)
            self.assertIn("requirement_sha", str(cm.exception), name)

    def test_a_malformed_sha_in_any_fingerprint_map_is_refused(self):
        for field in R._FINGERPRINT_MAPS:
            for name, bad in self.BAD.items():
                v = self.verdict()
                v[field] = {self.key: bad}
                with self.assertRaises(R.ParseError, msg=f"{field}/{name}") as cm:
                    self._load(v)
                self.assertIn(field, str(cm.exception), f"{field}/{name}")

    def test_the_refusal_names_the_value_and_the_expected_shape(self):
        """`ai/rules/cli.md`: the offending value AND the expected one. 'invalid sha'
        with neither is unactionable, and the reader cannot tell a typo from a real re-audit."""
        v = self.verdict()
        v["requirement_sha"] = "a" * (R.SHA_HEX_LEN - 1)
        with self.assertRaises(R.ParseError) as cm:
            self._load(v)
        msg = str(cm.exception)
        self.assertIn("a" * (R.SHA_HEX_LEN - 1), msg)
        self.assertIn(str(R.SHA_HEX_LEN), msg)
        self.assertIn("hex", msg)
        self.assertIn("15-character", msg)

    def test_a_wrong_type_keeps_its_own_message(self):
        """A non-string is a TYPE fault, not a shape one, and the two must not collapse: the
        author of `tests: {k: 42}` needs to be told it is a number."""
        v = self.verdict()
        v["tests"] = {self.key: 42}
        with self.assertRaises(R.ParseError) as cm:
            self._load(v)
        self.assertIn("int", str(cm.exception))

    def test_every_live_recorded_sha_passes_the_shape(self):
        """The gate is only sound if the real tree satisfies it. A shape the committed records
        fail would mean the shape is wrong, not the records (and `load_audits` would abort every
        consumer of rfc/audit/)."""
        checked = 0
        for stem in sorted(R.audit_stems()):
            for rid, v in R.load_audit(stem).items():
                self.assertRegex(v["requirement_sha"], R._SHA_RE, f"{stem} {rid}")
                checked += 1
                for field in R._FINGERPRINT_MAPS:
                    for key, sha in (v.get(field) or {}).items():
                        self.assertRegex(sha, R._SHA_RE, f"{stem} {rid} {field} {key}")
                        checked += 1
        self.assertGreater(checked, 300, "the live audit tree must be non-trivial")


class TestNotApplicableTestsMapSpelling(_AuditFixture):
    """The documented way to author OR-1's state produced a permanently red gate.

    OR-1 says a `not-applicable` verdict's `tests` map is absent OR empty, and
    `ai/skills/ze-rfc-audit.md` tells the author it is for "every verdict except
    `not-applicable`" -- i.e. omit it. `_verdict_claims` accepts both spellings (it tests the map
    for falsiness). `verdict_freshness` did not: it compared the raw `verdict.get("tests")`,
    which is `None` when the key is absent, against a computed `{}`, and `None == {}` is False.

    So the DOCUMENTED authoring path read STALE_UNIT forever, and the message it emitted was
    false in all three of its clauses -- no tagged test was ever gone (OR-1 forbids citing one),
    it is not a line shift, and re-running `/ze-rfc-audit` reproduces the identical record.
    `--reseal` refused it too ("a human must re-read it"), so there was no exit at all short of
    guessing that `"tests": {}` had to be written literally.
    """

    def _na(self, spelling, **extra):
        """A legal not-applicable record, with `tests` written empty or omitted."""
        ann = R.Annotation(
            kind="not-applicable", polarity=None, reason="binds future authors"
        )
        req = _req(self.req.rid, rfc="rfc9999", annotation=ann)
        v = self.verdict(value=R.VERDICT_NOT_APPLICABLE, **extra)
        v["no_code_path"] = "section 8 binds the authors of future specifications"
        v["units"] = {}
        if spelling == "empty":
            v["tests"] = {}
        else:
            del v["tests"]
        return req, v

    def test_absent_and_empty_are_the_same_state(self):
        """The zero-value trap (ai/rules/evidence.md): a present-but-empty value must
        never diverge from an absent one. One record, two spellings, two different states."""
        seen = {}
        for spelling in ("empty", "omitted"):
            req, v = self._na(spelling)
            seen[spelling] = R.verdict_freshness(
                v, R.requirement_sha(req.text), {}, {}, {}
            )
        self.assertEqual(seen["omitted"], (R.FRESH, []))
        self.assertEqual(seen["empty"], seen["omitted"])

    def test_neither_spelling_raises_a_freshness_violation(self):
        for spelling in ("empty", "omitted"):
            req, v = self._na(spelling)
            errs = R.check_audit_freshness(
                [req], [], {"rfc9999"}, {"rfc9999": {req.rid: v}}
            )
            self.assertEqual(errs, [], spelling)

    def test_the_transitional_rule_normalises_it_too(self):
        """The same absent-versus-empty normalisation on a record with NO `units` key at all.

        `test_absent_and_empty_are_the_same_state` above reaches the transitional branch with
        `units` recorded EMPTY; this one omits the key entirely, which is how a verdict written
        before unit fingerprints existed actually looks. These three assertions used to drive
        `verdict_is_fresh`, the deleted duplicate of this branch, and its docstring claimed the
        two spellings were pinned together -- they were not, which is why the coverage moved to
        the branch that runs."""
        self.assertEqual(
            R.verdict_freshness({"requirement_sha": "aaa"}, "aaa", {}), (R.FRESH, [])
        )
        self.assertEqual(
            R.verdict_freshness({"requirement_sha": "aaa", "tests": {}}, "aaa", {}),
            (R.FRESH, []),
        )
        state, _moved = R.verdict_freshness(
            {"requirement_sha": "aaa"}, "aaa", {"a_test.go:1": "b"}
        )
        self.assertEqual(state, R.STALE_UNIT)

    def test_a_real_stale_not_applicable_record_still_fails(self):
        """The discriminating twin. Normalising absent-to-empty must not make the state
        unfalsifiable: editing the requirement TEXT under it still voids the judgement."""
        req, v = self._na("omitted")
        state, _moved = R.verdict_freshness(v, R.requirement_sha("CHANGED"), {}, {}, {})
        self.assertEqual(state, R.STALE_REQUIREMENT)

    def test_no_code_path_must_be_prose_not_any_json_value(self):
        """`no_code_path` was the only string field never type-checked: read as
        `str(verdict.get("no_code_path") or "").strip()`, so `123`, `["a"]`, `{"k": "v"}` and
        `true` all satisfied OR-1's MANDATORY prose reason. Its siblings `note`,
        `requirement_sha` and `upgrade_reason` all go through `_str_field`, which is why
        `upgrade_reason: 123` was correctly refused and this was not."""
        for bad in (123, ["a", "b"], {"k": "v"}, True, 0.5):
            req, v = self._na("empty")
            v["no_code_path"] = bad
            with _audit_tree(files={"rfc9999": _audit_file("rfc9999", {req.rid: v})}):
                with self.assertRaises(R.ParseError, msg=repr(bad)) as cm:
                    R.load_audit("rfc9999")
            self.assertIn("no_code_path", str(cm.exception))

    def test_a_prose_reason_is_still_loaded(self):
        """The discriminating twin: the type check must not reject the legal state."""
        req, v = self._na("empty")
        with _audit_tree(files={"rfc9999": _audit_file("rfc9999", {req.rid: v})}):
            loaded = R.load_audit("rfc9999")
        self.assertIn(req.rid, loaded)

    def test_an_empty_reason_is_still_the_friendly_violation(self):
        """A whitespace reason must stay a REPORTED violation with OR-1's message, not become a
        ParseError that aborts the run: `_verdict_claims` owns that case and names what to do."""
        req, v = self._na("empty")
        v["no_code_path"] = "   "
        with _audit_tree(files={"rfc9999": _audit_file("rfc9999", {req.rid: v})}):
            loaded = R.load_audit("rfc9999")
        errs = R.check_audit_schema([req], [], {"rfc9999"}, {"rfc9999": loaded})
        self.assertTrue(errs)
        self.assertIn("no_code_path", " ".join(errs))


class TestSymbolKeyResolution(_AuditFixture):
    """A key names a symbol, so the states a LOCATION key could not have are the ones to pin.

    A stored line had one failure mode, drifting silently onto whatever code moved under it. A
    stored symbol has two, and both must be refusals rather than answers: the name is gone, or
    the name is declared twice and picking either would fingerprint text nobody chose.
    """

    def _rename_tagged_func(self):
        path = os.path.join(R.PROJECT_DIR, self.rel)
        with open(path, encoding="utf-8") as fh:
            src = fh.read()
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(src.replace("func TestTagged(", "func TestRenamed("))

    def test_a_key_whose_symbol_is_gone_is_refused_by_name(self):
        """The third refusal, beside the missing file and the empty extraction. The message must
        say WHICH symbol and WHICH file, because the reader's next move is to decide whether the
        rename was innocent, and nothing else in the run tells them what moved."""
        self._rename_tagged_func()
        with self.assertRaises(R.ParseError) as cm:
            R.unit_shas([self.key])
        msg = str(cm.exception)
        self.assertIn("TestTagged", msg)
        self.assertIn(self.rel, msg)
        self.assertIn("ai/skills/ze-rfc-audit.md", msg)

    def test_a_gone_symbol_reaches_the_gate_as_stale_not_as_a_crash(self):
        """The state, not the exception. Raising through `audit_freshness` would take the LEDGER
        RENDER down with it; STALE_UNIT sends the verdict for a re-read AND reds the check."""
        v = self.verdict()
        self._rename_tagged_func()
        state, moved = R.audit_freshness(
            [self.req], self.tags, {"rfc9999"}, self.audits(v)
        )[self.req.rid]
        self.assertEqual(state, R.STALE_UNIT)
        self.assertTrue(moved)

    def test_two_functions_with_one_name_are_refused(self):
        """Two methods of different receivers may share a name in one Go file. Neither is `the`
        unit, so `func_text` returns nothing and the key is refused rather than resolved to
        whichever came first."""
        path = os.path.join(self.tmp, "twice_test.go")
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(
                "package x\n"
                "\n"
                "func (a A) Run(t *testing.T) {\n\trequire.Equal(t, 1, one())\n}\n"
                "\n"
                "func (b B) Run(t *testing.T) {\n\trequire.Equal(t, 2, two())\n}\n"
            )
        rel = os.path.relpath(path, R.PROJECT_DIR).replace(os.sep, "/")
        with open(path, encoding="utf-8") as fh:
            src = fh.read()
        self.assertIsNone(R.rfc_tagged_scope.func_text(src, "Run"))
        with self.assertRaises(R.ParseError) as cm:
            R.unit_shas([f"{rel}::Run"])
        self.assertIn("2 top-level functions declare it", str(cm.exception))

    def test_unit_identity_reads_the_file_from_every_key_shape(self):
        """`_unit_identity` recovers the FILE from a key, and the key now has three shapes. A
        `rsplit(':')` returned `path:` for a symbol key, which made every pair unequal and every
        verdict STALE for a reason that was never in the tree."""
        ident = R._unit_identity(
            {
                "a/b_test.go::TestFoo": "u" * 16,
                "a/b_test.go::TestFoo#2": "u" * 16,
                "test/x.ci": "v" * 16,
                "test/y.ci#2": "v" * 16,
            }
        )
        self.assertEqual(
            ident,
            {
                ("a/b_test.go", "u" * 16): 2,
                ("test/x.ci", "v" * 16): 1,
                ("test/y.ci", "v" * 16): 1,
            },
        )


class TestUnitIdentityIsAMultiset(_AuditFixture):
    """`_unit_identity` counts (file, unit-sha) pairs instead of collecting them into a set, and
    its docstring names exactly what the count guards: two tags inside ONE function collapse to
    one pair, so a set would call deleting one of them "unchanged" -- a false FRESH, which the
    spec's Security Review calls the one catastrophic outcome.

    Nothing tested it. Turning the count into `out[(rel, sha)] = 1` passed all 488 tests, and
    the shape is not hypothetical: 334 of 1351 requirement ids in the tree have a (file,
    unit-sha) pair counted above one, four of them carrying a verdict today.

    The count survives the move from a location key to a SYMBOL key only because of the `#2`
    ordinal. Without it both tags below mint the string `<path>::TestTagged`, the map holds one
    entry instead of two, and deleting one tag leaves it byte-identical. The first migration
    written for this change did exactly that and lost 8 of 337 recorded keys.
    """

    def _two_tags(self):
        """Two same-polarity tags inside one function: the doc comment and a body line."""
        return [
            _tag(self.req.rid, "positive", file=self.rel, line=self.line),
            _tag(self.req.rid, "positive", file=self.rel, line=self.line + 2),
        ]

    def test_two_tags_in_one_unit_take_an_ordinal(self):
        """The key form itself: one key per tag, the second one carrying `#2`."""
        keys = R.tag_keys(self._two_tags())
        self.assertEqual(keys, [f"{self.rel}::TestTagged", f"{self.rel}::TestTagged#2"])

    def test_two_tags_in_one_unit_count_twice(self):
        """The mechanism, directly: a set reports one pair where the record holds two."""
        recorded = R.unit_shas(R.tag_keys(self._two_tags()))
        self.assertEqual(len(recorded), 2, "the fixture needs two keys...")
        self.assertEqual(
            len(set(recorded.values())), 1, "...resolving to the SAME unit"
        )
        self.assertEqual(list(R._unit_identity(recorded).values()), [2])

    _SECOND_TAG = (
        "\t// RFC requirement: RFC9999-2-1 positive -- and again after a reset.\n"
    )

    def _two_tag_file(self):
        """A file whose ONE function carries two real tag comments, at lines 3 and 6."""
        path = os.path.join(self.tmp, "twotags_test.go")
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(
                "package x\n"
                "\n"
                "// RFC requirement: RFC9999-2-1 positive -- one() returns one.\n"
                "func TestTwice(t *testing.T) {\n"
                "\trequire.Equal(t, 1, one())\n"
                + self._SECOND_TAG
                + "\trequire.Equal(t, 1, one())\n"
                "}\n"
            )
        rel = os.path.relpath(path, R.PROJECT_DIR).replace(os.sep, "/")
        return rel, [
            _tag(self.req.rid, "positive", file=rel, line=3),
            _tag(self.req.rid, "positive", file=rel, line=6),
        ]

    def test_deleting_one_tag_from_the_file_is_not_fresh(self):
        """The same loss driven through a real FILE rather than through a tag list.

        The tag-list version below proves the comparison; this one proves what a real edit meets.
        Both tag comments sit in one function, so both keys name `TestTwice` and only the ordinal
        tells them apart. Delete one comment and the surviving reading holds ONE key. A verdict
        recorded over two tagged sites must not read FRESH over one."""
        rel, two = self._two_tag_file()
        v = {
            "verdict": "enforced",
            "note": "two tagged assertions inside one function",
            "requirement_sha": R.requirement_sha(self.req.text),
            "tests": R.tagged_unit_shas(two),
            "units": R.unit_shas(R.tag_keys(two)),
        }
        self.assertEqual(
            sorted(v["units"]),
            [f"{rel}::TestTwice", f"{rel}::TestTwice#2"],
            "the recorded map must hold one key per tagged site",
        )
        path = os.path.join(R.PROJECT_DIR, rel)
        with open(path, encoding="utf-8") as fh:
            src = fh.read()
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(src.replace(self._SECOND_TAG, ""))
        survivor = [two[0]]
        state, _moved = R.verdict_freshness(
            v,
            R.requirement_sha(self.req.text),
            R.tagged_unit_shas(survivor),
            R.unit_shas(R.tag_keys(survivor)),
            {},
        )
        self.assertNotEqual(state, R.FRESH)
        self.assertEqual(state, R.STALE_UNIT)

    def test_deleting_one_of_two_tags_in_a_unit_is_stale_not_shifted(self):
        """The consequence, through the real consumer. Deleting one of two same-polarity tags in
        one function is a real change to what is proven: STALE_UNIT, which `--reseal` refuses.
        Under a set it compares equal and degrades to the mechanically re-sealable SHIFTED --
        which is the false FRESH one `make ze-rfc-reseal` away."""
        two = self._two_tags()
        survivor = [two[0]]
        v = {
            "verdict": "enforced",
            "note": "two tagged assertions inside one function",
            "requirement_sha": R.requirement_sha(self.req.text),
            "tests": R.tagged_unit_shas(two),
            "units": R.unit_shas(R.tag_keys(two)),
        }
        state, moved = R.verdict_freshness(
            v,
            R.requirement_sha(self.req.text),
            R.tagged_unit_shas(survivor),
            R.unit_shas(R.tag_keys(survivor)),
            {},
        )
        self.assertEqual(state, R.STALE_UNIT)
        self.assertIn(f"{self.rel}::TestTagged#2", moved)

    def test_both_tags_intact_is_still_fresh(self):
        """The discriminating twin: the count must not manufacture a new false-stale class."""
        two = self._two_tags()
        v = {
            "verdict": "enforced",
            "note": "two tagged assertions inside one function",
            "requirement_sha": R.requirement_sha(self.req.text),
            "tests": R.tagged_unit_shas(two),
            "units": R.unit_shas(R.tag_keys(two)),
        }
        state, _moved = R.verdict_freshness(
            v,
            R.requirement_sha(self.req.text),
            R.tagged_unit_shas(two),
            R.unit_shas(R.tag_keys(two)),
            {},
        )
        self.assertEqual(state, R.FRESH)


class TestAuditUnitFreshness(_AuditFixture):
    """The false-stale fix. `tagged_unit_shas` hashes the whole enclosing FILE, so a verdict went
    stale on any edit anywhere in a tagged file and on any line shift: six of a pending sixteen
    commits to the one existing audit file were mechanical re-stamps in which no verdict changed.
    """

    def _state(self, verdict=None, reqs=None, tags=None):
        return R.audit_freshness(
            reqs if reqs is not None else [self.req],
            tags if tags is not None else self.tags,
            {"rfc9999"},
            self.audits(verdict),
        ).get(self.req.rid)

    def test_untouched_is_fresh(self):
        self.assertEqual(self._state()[0], R.FRESH)

    def test_sibling_edit_is_shifted_not_stale(self):
        """AC-14, the F18 case: editing an UNRELATED test in the same file. This is the class
        that cost a human a mechanical proof and a 200-word note, six times."""
        v = self.verdict()
        path = os.path.join(R.PROJECT_DIR, self.rel)
        with open(path, encoding="utf-8") as fh:
            src = fh.read()
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(
                src.replace("require.Equal(t, 2, two())", "require.NotNil(t, two())")
            )
        self.assertEqual(self._state(v)[0], R.SHIFTED)

    def test_line_shift_is_shifted_not_stale(self):
        """A nine-line header prepended to a tagged file shifted every key by +9 and staled every
        verdict in it, having changed nothing about any assertion."""
        v = self.verdict()
        path = os.path.join(R.PROJECT_DIR, self.rel)
        with open(path, encoding="utf-8") as fh:
            src = fh.read()
        with open(path, "w", encoding="utf-8") as fh:
            fh.write("// header\n" + src)
        tags = [t._replace(line=t.line + 1) for t in self.tags]
        state, moved = R.audit_freshness([self.req], tags, {"rfc9999"}, self.audits(v))[
            self.req.rid
        ]
        self.assertEqual(state, R.SHIFTED)
        self.assertTrue(moved, "the message must be able to name what moved")

    def test_edit_inside_unit_is_stale(self):
        """AC-15: the discriminating twin. A change to what the tagged test ASSERTS is a real
        judgement change and must never be reported as the mechanical case."""
        v = self.verdict()
        path = os.path.join(R.PROJECT_DIR, self.rel)
        with open(path, encoding="utf-8") as fh:
            src = fh.read()
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(
                src.replace(
                    "require.Equal(t, 1, one())", "require.NotEqual(t, 1, one())"
                )
            )
        self.assertEqual(self._state(v)[0], R.STALE_UNIT)

    def test_requirement_edit_is_distinguished(self):
        """AC-15: re-reading the RFC voids every judgement under it, and that is a different
        remedy from a test edit, so it is a different state and a different message."""
        v = self.verdict()
        v["requirement_sha"] = R.requirement_sha("something else entirely")
        self.assertEqual(self._state(v)[0], R.STALE_REQUIREMENT)

    def test_requirement_edit_wins_over_a_shift(self):
        """Order is the bias: a real judgement change must never be reported as re-sealable."""
        v = self.verdict()
        v["requirement_sha"] = R.requirement_sha("other")
        v["tests"] = {self.key: "0" * 16}
        self.assertEqual(self._state(v)[0], R.STALE_REQUIREMENT)

    def test_missing_units_falls_back_to_file_rule(self):
        """AC-20: a verdict recorded before unit fingerprints existed keeps EXACTLY today's
        behaviour, so the migration is a backfill and not a re-judgement."""
        v = self.verdict()
        del v["units"]
        self.assertEqual(self._state(v)[0], R.FRESH)
        v2 = self.verdict()
        del v2["units"]
        v2["tests"] = {self.key: "0" * 16}
        self.assertEqual(self._state(v2)[0], R.STALE_UNIT)

    def test_a_deleted_tagged_test_is_stale(self):
        """A verdict whose test is gone must never read as unchanged."""
        v = self.verdict()
        state, _ = R.audit_freshness([self.req], [], {"rfc9999"}, self.audits(v))[
            self.req.rid
        ]
        self.assertEqual(state, R.STALE_UNIT)

    def test_unresolvable_span_falls_back_to_file(self):
        """R-2: a tag outside every function span resolves to the WHOLE FILE, which is more
        checking, never less. A narrower guess there would be a false FRESH."""
        path = os.path.join(self.tmp, "hoist_test.go")
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(
                "package x\n\n// RFC requirement: RFC9999-2-1 positive -- x.\nvar y = 1\n"
            )
        rel = os.path.relpath(path, R.PROJECT_DIR).replace(os.sep, "/")
        with open(path, encoding="utf-8") as fh:
            src = fh.read()
        text, kind = R.rfc_tagged_scope.unit_at(rel, src, 3)
        self.assertEqual(kind, R.rfc_tagged_scope.SCOPE_FILE)
        self.assertIn("package x", text)

    def test_empty_extraction_is_an_error_not_a_hash(self):
        """R-2, the zero-value trap. Hashing "" would give every unreadable file the same
        fingerprint, so a deleted test would read as unchanged -- a false FRESH, the one
        catastrophic outcome (ai/rules/evidence.md)."""
        path = os.path.join(self.tmp, "empty_test.go")
        open(path, "w").close()
        rel = os.path.relpath(path, R.PROJECT_DIR).replace(os.sep, "/")
        with self.assertRaises(R.ParseError) as cm:
            R.unit_shas([rel])
        self.assertIn("fingerprint", str(cm.exception))

    def test_unit_fingerprint_differs_from_the_file_fingerprint(self):
        """The whole premise: if the unit sha equalled the file sha, nothing would have changed
        and the SHIFTED state could never exist."""
        self.assertNotEqual(
            R.unit_shas([self.key])[self.key],
            R.tagged_unit_shas(self.tags)[self.key],
            "a unit sha equal to the file sha means the span was not resolved",
        )

    def test_shifted_message_names_reseal_and_not_index(self):
        """AC-14 is explicit about the WORDS: the remedy is `make ze-rfc-reseal`, and it must not
        name `make ze-rfc-index-update`, which does not clear this state (A-7)."""
        v = self.verdict()
        v["tests"] = {self.key: "0" * 16}
        errs = R.check_audit_freshness(
            [self.req], self.tags, {"rfc9999"}, self.audits(v)
        )
        self.assertTrue(errs)
        self.assertIn("make ze-rfc-reseal", errs[0])
        self.assertIn("SHIFTED", errs[0])
        self.assertNotIn("ze-rfc-index-update", errs[0])

    def test_stale_message_does_not_offer_the_mechanical_remedy(self):
        """The inverse, and the one that matters: a real judgement change must not be handed a
        one-command fix."""
        v = self.verdict()
        v["units"] = {self.key: "0" * 16}
        errs = R.check_audit_freshness(
            [self.req], self.tags, {"rfc9999"}, self.audits(v)
        )
        self.assertTrue(errs)
        self.assertIn("ai/skills/ze-rfc-audit.md", errs[0])
        self.assertIn("refuse", errs[0])


class TestAuditCodeFingerprint(_AuditFixture):
    """AC-8: the empty-`tests` class is no longer permanently fresh.

    Three of the 52 verdicts in the one existing audit file carried an empty `tests` map, so
    their freshness test was `{} == {}` and they could never go stale -- claims about CODE
    fingerprinted against TESTS that do not exist.
    """

    def _gap(self):
        ann = R.Annotation(kind="gap", polarity=None, reason="deliberate")
        return _req("RFC9999-2-1", rfc="rfc9999", annotation=ann)

    def _unimpl(self, code=None):
        v = self.verdict(value="unimplemented")
        v["tests"], v["units"] = {}, {}
        v["code"] = R.unit_shas([code or self.code_key])
        return v

    def test_editing_cited_producer_stales_verdict(self):
        req, v = self._gap(), self._unimpl()
        path = os.path.join(R.PROJECT_DIR, self.rel)
        with open(path, encoding="utf-8") as fh:
            src = fh.read()
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(
                src.replace("require.Equal(t, 2, two())", "require.Equal(t, 3, two())")
            )
        state, moved = R.audit_freshness(
            [req], [], {"rfc9999"}, {"rfc9999": {req.rid: v}}
        )[req.rid]
        self.assertEqual(state, R.STALE_UNIT)
        self.assertTrue(moved)

    def test_an_untouched_producer_stays_fresh(self):
        """The discriminating twin: the code map is not "always stale"."""
        req, v = self._gap(), self._unimpl()
        self.assertEqual(
            R.audit_freshness([req], [], {"rfc9999"}, {"rfc9999": {req.rid: v}})[
                req.rid
            ][0],
            R.FRESH,
        )

    def test_symbol_span_preferred_over_whole_file(self):
        """R-5: producer files churn far more than test files, so a file-level hash here would
        just manufacture a new false-stale class. Editing a DIFFERENT function in the same
        producer file must not stale the verdict."""
        req, v = self._gap(), self._unimpl()
        path = os.path.join(R.PROJECT_DIR, self.rel)
        with open(path, encoding="utf-8") as fh:
            src = fh.read()
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(
                src.replace("require.Equal(t, 1, one())", "require.Equal(t, 9, one())")
            )
        self.assertEqual(
            R.audit_freshness([req], [], {"rfc9999"}, {"rfc9999": {req.rid: v}})[
                req.rid
            ][0],
            R.FRESH,
        )

    def test_a_deleted_producer_fails_closed(self):
        """A cited producer that is gone is not "unchanged": it degrades toward MORE checking.

        Deliberately a STATE and not an exception. Raising would take the LEDGER RENDER down
        with it -- a report is not a gate -- while STALE_UNIT sends the verdict for a re-read AND
        reds `make ze-rfc-check`, which is the outcome that matters."""
        req, v = self._gap(), self._unimpl()
        os.remove(os.path.join(R.PROJECT_DIR, self.rel))
        state, moved = R.audit_freshness(
            [req], [], {"rfc9999"}, {"rfc9999": {req.rid: v}}
        )[req.rid]
        self.assertEqual(state, R.STALE_UNIT)
        self.assertTrue(moved, "the message must name what could not be resolved")
        self.assertTrue(
            R.check_audit_freshness([req], [], {"rfc9999"}, {"rfc9999": {req.rid: v}})
        )


class TestAuditDisclosure(_AuditFixture):
    """AC-9: `check_status_agreement` already refuses to let a {gap} ANNOTATION hide behind a
    clean 'Supported' row. A verdict saying the same thing must not be weaker than an annotation
    saying it."""

    def _errs(self, status, value="wrong"):
        rows = R.parse_status_ledger(status)
        return R.check_audit_disclosure(
            [self.req], rows, {"rfc9999"}, self.audits(value=value)
        )

    def test_wrong_under_clean_supported_fails(self):
        errs = self._errs(_STATUS_CLEAN)
        self.assertTrue(errs)
        joined = " ".join(errs)
        self.assertIn(self.req.rid, joined)
        self.assertIn("Supported", joined)

    def test_wrong_with_disclosed_row_passes(self):
        self.assertEqual(self._errs(_STATUS_DISCLOSED), [])

    def test_unimplemented_under_clean_supported_fails(self):
        self.assertTrue(self._errs(_STATUS_CLEAN, value="unimplemented"))

    def test_missing_row_fails(self):
        self.assertTrue(self._errs(_STATUS_HEAD))

    def test_weak_is_not_gated_on_disclosure(self):
        """AC-10's incentive shape, made executable. `weak` says the TEST cannot fail on
        non-compliance, not that the code is wrong, so demanding a public gap note for it would
        publish a claim the audit does not support -- and would make honesty the expensive
        path, which is the inversion this whole spec removes."""
        self.assertEqual(self._errs(_STATUS_CLEAN, value="weak"), [])

    def test_one_definition_of_disclosure(self):
        """The annotation check and the verdict check must not drift: both read
        row_discloses_a_gap, so a row that discloses for one discloses for the other."""
        clean = R.parse_status_ledger(_STATUS_CLEAN)["rfc9999"]
        disclosed = R.parse_status_ledger(_STATUS_DISCLOSED)["rfc9999"]
        self.assertFalse(R.row_discloses_a_gap(clean))
        self.assertTrue(R.row_discloses_a_gap(disclosed))


class TestAuditNote(_AuditFixture):
    """AC-17. No gate can prove a human read the RFC; this is the cheapest honest proxy -- an
    account of a test one has actually read almost always names something in it."""

    def _errs(self, note):
        return R.check_audit_note(
            [self.req], self.tags, {"rfc9999"}, self.audits(note=note)
        )

    def test_note_must_cite_a_symbol_in_a_tagged_unit(self):
        errs = self._errs("looks fine to me, seems correct enough overall")
        self.assertTrue(errs)
        joined = " ".join(errs)
        self.assertIn(self.rel, joined, "the message must name the files searched")
        self.assertIn("tokens checked", joined)

    def test_one_matching_token_is_enough(self):
        """R-3: a prose note must not be punished for being prose."""
        self.assertEqual(
            self._errs(
                "the negative case would pass on any error, but require.Equal pins the exact "
                "value, so a stub implementation cannot satisfy it"
            ),
            [],
        )

    def test_a_symbol_only_in_a_sibling_function_does_not_count(self):
        """The unit, not the file: naming something from an unrelated test in the same file is
        exactly the citation that proves nothing was read."""
        self.assertTrue(self._errs("helperUntagged covers this"))

    def test_short_words_do_not_satisfy_it(self):
        """`the` occurs in every file. A token has to be long enough to be a symbol."""
        self.assertTrue(self._errs("it is the one"))

    def test_only_enforced_notes_are_checked(self):
        """A `weak` or `unimplemented` note describes something that is NOT in the test, so
        demanding a symbol from the test would punish the honest verdict."""
        for value in ("weak", "wrong"):
            errs = R.check_audit_note(
                [self.req],
                self.tags,
                {"rfc9999"},
                self.audits(value=value, note="nothing here matches"),
            )
            self.assertEqual(errs, [], value)


class TestAuditFindings(_AuditFixture):
    """A finding that can be deleted is a finding that will be. This is
    check_coverage_ratchet's shape applied to judgement rather than to tags."""

    def _errs(self, now_value, was_value="weak", baseline=None, **extra):
        was = self.verdict(value=was_value)
        base = {"rfc9999": {self.req.rid: was}} if baseline is None else baseline
        audits = (
            {"rfc9999": {}}
            if now_value is None
            else self.audits(value=now_value, **extra)
        )
        return R.check_audit_findings([self.req], {"rfc9999"}, audits, base)

    def test_weak_verdict_does_not_fail_the_gate(self):
        """AC-10, the incentive decision made executable: recording a finding is FREE. Failing
        immediately is safe only because zero such verdicts exist today, and that is exactly the
        problem -- the first person it would bite is the honest auditor."""
        self.assertEqual(self._errs("weak"), [])

    def test_deleted_finding_fails(self):
        """AC-11: once the verdict value has consequences, deleting it becomes the cheapest
        route from red to green."""
        errs = self._errs(None)
        self.assertTrue(errs)
        self.assertIn("DELETED", " ".join(errs))

    def test_upgrade_without_unit_change_fails(self):
        """AC-12: a finding cannot become proof with nothing changed."""
        errs = self._errs("enforced")
        self.assertTrue(errs)
        self.assertIn("byte-identical", " ".join(errs))

    def test_upgrade_with_changed_unit_passes(self):
        """AC-12's negative half: fixing the test moves its unit fingerprint, which IS the
        evidence that something changed."""
        was = self.verdict(value="weak")
        was["units"] = {self.key: "0" * 16}
        errs = R.check_audit_findings(
            [self.req], {"rfc9999"}, self.audits(), {"rfc9999": {self.req.rid: was}}
        )
        self.assertEqual(errs, [])

    def test_upgrade_with_recorded_reason_passes(self):
        """AC-12's auditable escape: the unit hash cannot follow a call or see a re-read of the
        RFC, so there has to be a way through -- one that is written down rather than silent."""
        self.assertEqual(
            self._errs(
                "enforced", upgrade_reason="re-read S5.1; the earlier read was wrong"
            ),
            [],
        )

    def test_a_blank_reason_is_not_an_escape(self):
        self.assertTrue(self._errs("enforced", upgrade_reason="   "))

    def test_downgrade_to_a_finding_is_free(self):
        """Reporting is never the expensive path: `enforced` -> `weak` passes."""
        was = self.verdict(value="enforced")
        errs = R.check_audit_findings(
            [self.req],
            {"rfc9999"},
            self.audits(value="weak"),
            {"rfc9999": {self.req.rid: was}},
        )
        self.assertEqual(errs, [])

    def test_degraded_git_baseline_accuses_nobody(self):
        """R-7: "I could not look" must never render as "nothing was there". A wall of
        violations on a fresh clone is the fastest way to teach people to bypass a gate."""
        with _patched(_git_baseline_audits=lambda: None):
            self.assertEqual(
                R.check_audit_findings([self.req], {"rfc9999"}, {"rfc9999": {}}), []
            )


class TestAuditVerdictRatchet(_AuditFixture):
    """AC-13/AC-19. The ratchet is on the SET of audited ids, never on the ratio: a ratio
    ratchet would fail the build for adding a tagged test, which punishes coverage work."""

    def test_removed_verdict_fails(self):
        errs = R.check_audit_verdict_ratchet(
            [self.req],
            {"rfc9999"},
            {"rfc9999": {}},
            {"rfc9999": {self.req.rid: self.verdict()}},
            {"rfc9999"},
        )
        self.assertTrue(errs)
        self.assertIn("monotonic", " ".join(errs))

    def test_a_kept_verdict_passes(self):
        self.assertEqual(
            R.check_audit_verdict_ratchet(
                [self.req],
                {"rfc9999"},
                self.audits(),
                {"rfc9999": {self.req.rid: self.verdict()}},
                {"rfc9999"},
            ),
            [],
        )

    def test_a_stale_verdict_may_not_be_deleted_either(self):
        """Deliberately stricter than the literal "fresh at HEAD": a stale verdict must be
        RE-JUDGED, not removed. Staleness is the state in which deletion is most tempting and
        least honest, so exempting it would aim the ratchet away from its own case."""
        was = self.verdict()
        was["units"] = {self.key: "0" * 16}
        self.assertTrue(
            R.check_audit_verdict_ratchet(
                [self.req],
                {"rfc9999"},
                {"rfc9999": {}},
                {"rfc9999": {self.req.rid: was}},
                {"rfc9999"},
            )
        )

    def test_percentage_drop_from_new_tag_passes(self):
        """AC-19/R-6, the ratchet-on-a-ratio trap stated as a test. Adding a second audited-able
        requirement halves the percentage and removes no verdict, so it must be green."""
        second = _req("RFC9999-2-2", rfc="rfc9999")
        errs = R.check_audit_verdict_ratchet(
            [self.req, second],
            {"rfc9999"},
            self.audits(),
            {"rfc9999": {self.req.rid: self.verdict()}},
            {"rfc9999"},
        )
        self.assertEqual(errs, [])

    def test_degraded_git_baseline_accuses_nobody(self):
        with _patched(_git_baseline_audits=lambda: None):
            self.assertEqual(
                R.check_audit_verdict_ratchet([self.req], {"rfc9999"}, {"rfc9999": {}}),
                [],
            )

    def test_an_rfc_enrolled_in_this_change_is_not_accused(self):
        """Scoped like its two siblings: an RFC enrolled in THIS change is judged by the
        ordinary rules rather than accused of losing evidence it never had."""
        self.assertEqual(
            R.check_audit_verdict_ratchet(
                [self.req],
                {"rfc9999"},
                {"rfc9999": {}},
                {"rfc9999": {self.req.rid: self.verdict()}},
                set(),
            ),
            [],
        )


class TestReseal(_AuditFixture):
    """AC-16. Every no-judgement re-stamp is a training example teaching that re-stamping is
    what one does when the gate goes red. Removing the human from that class is worth more than
    any message improvement, because at fleet scale the reflex is what fails."""

    def _drive(self, verdict, prove=None):
        stem = "rfc9999"
        with _audit_tree(files={stem: _audit_file(stem, {self.req.rid: verdict})}):
            with _patched(
                load_enrolled=lambda: {stem},
                summary_stems=lambda: {stem},
                parse_summary_file=lambda path: [self.req],
                scan_tree=lambda *a, **k: self.tags,
            ):
                resealed, refused = R.reseal_audits(prove=prove)
                with open(
                    os.path.join(R.AUDIT_DIR, stem + ".json"), encoding="utf-8"
                ) as fh:
                    after = json.load(fh)
        return resealed, refused, after

    def _shift(self):
        """A verdict in the SHIFTED state: unit identical, file fingerprint moved."""
        v = self.verdict()
        v["tests"] = {self.key: "0" * 16}
        return v

    def test_only_shifted_are_restamped(self):
        resealed, refused, after = self._drive(self._shift())
        self.assertEqual(len(resealed), 1, refused)
        self.assertEqual(
            after["requirements"][self.req.rid]["tests"],
            R.tagged_unit_shas(self.tags),
        )

    def test_a_stale_unit_is_refused(self):
        v = self.verdict()
        v["units"] = {self.key: "0" * 16}
        resealed, refused, after = self._drive(v)
        self.assertEqual(resealed, [])
        self.assertTrue(refused)
        self.assertEqual(after["requirements"][self.req.rid], v)

    def test_a_stale_requirement_is_refused(self):
        v = self._shift()
        v["requirement_sha"] = R.requirement_sha("other")
        resealed, refused, _ = self._drive(v)
        self.assertEqual(resealed, [])
        self.assertTrue(refused)

    def test_a_fresh_verdict_is_left_alone(self):
        resealed, refused, after = self._drive(self.verdict())
        self.assertEqual(resealed, [])
        self.assertEqual(refused, [])
        self.assertNotIn(
            "reaudit_note", after, "an untouched file must not be rewritten"
        )

    def test_reseal_never_mutates_judgement_fields(self):
        """AC-16: every key except `tests` is byte-identical afterward. The re-seal is a
        fingerprint refresh; a re-seal that could touch a verdict would be an audit."""
        before = self._shift()
        before["note"] = "require.Equal pins one(); unchanged by any re-stamp"
        _, _, after = self._drive(dict(before))
        got = after["requirements"][self.req.rid]
        for key in ("verdict", "note", "units", "requirement_sha"):
            self.assertEqual(got[key], before[key], key)

    def test_reseal_preserves_previous_note_into_history(self):
        """AC-16: an earlier re-stamp's reasoning is evidence about the same verdicts.
        Overwriting it would delete the record of why they were trusted then."""
        stem = "rfc9999"
        body = _audit_file(stem, {self.req.rid: self._shift()})
        body["reaudit_note"] = "the 2026-07-22 module rename"
        with _audit_tree(files={stem: body}):
            with _patched(
                load_enrolled=lambda: {stem},
                summary_stems=lambda: {stem},
                parse_summary_file=lambda path: [self.req],
                scan_tree=lambda *a, **k: self.tags,
            ):
                R.reseal_audits()
                with open(
                    os.path.join(R.AUDIT_DIR, stem + ".json"), encoding="utf-8"
                ) as fh:
                    after = json.load(fh)
        self.assertIn("the 2026-07-22 module rename", after["reaudit_history"])
        self.assertIn("ze-rfc-reseal", after["reaudit_note"])

    def test_a_transitional_verdict_needs_the_callers_proof(self):
        """A verdict with no recorded `units` cannot be SHOWN to have an unchanged unit, so it is
        re-sealable only when the caller supplies an independent proof. That is exactly the
        capability rename_module_path.py had before the loop became shared -- kept, not lost."""
        v = self.verdict()
        del v["units"]
        v["tests"] = {self.key: "0" * 16}
        resealed, refused, _ = self._drive(dict(v))
        self.assertEqual(resealed, [], "without a proof it must be refused")
        self.assertTrue(refused)
        resealed, refused, _ = self._drive(dict(v), prove=lambda rel: True)
        self.assertEqual(len(resealed), 1, refused)

    def test_the_callers_proof_can_only_refuse_more(self):
        """A `prove` that says no must veto even a SHIFTED verdict: two independent proofs, and
        the caller's can only ever make the re-seal stricter."""
        resealed, refused, _ = self._drive(self._shift(), prove=lambda rel: False)
        self.assertEqual(resealed, [])
        self.assertTrue(refused)

    def test_a_written_file_still_validates(self):
        """The writer round-trips through the REAL parser before the file lands: a derivation
        defect that wrote an unreadable record would make every later --check exit "cannot run",
        hiding every other violation in the repository."""
        stem = "rfc9999"
        with _audit_tree(
            files={stem: _audit_file(stem, {self.req.rid: self._shift()})}
        ):
            with _patched(
                load_enrolled=lambda: {stem},
                summary_stems=lambda: {stem},
                parse_summary_file=lambda path: [self.req],
                scan_tree=lambda *a, **k: self.tags,
            ):
                R.reseal_audits()
                self.assertIn(self.req.rid, R.load_audit(stem))

    def test_no_staging_directory_is_left_behind(self):
        stem = "rfc9999"
        with _audit_tree(
            files={stem: _audit_file(stem, {self.req.rid: self._shift()})}
        ) as adir:
            with _patched(
                load_enrolled=lambda: {stem},
                summary_stems=lambda: {stem},
                parse_summary_file=lambda path: [self.req],
                scan_tree=lambda *a, **k: self.tags,
            ):
                R.reseal_audits()
            self.assertEqual(
                [n for n in os.listdir(adir) if n.startswith(R._STAGING_PREFIX)], []
            )


class _AuditDrive(_AuditFixture):
    """Shared run_check driver: everything unrelated to the audit is patched out, so a failure
    here is the audit machinery and nothing else. A check that stops being CALLED must fail a
    test rather than passing silently (ai/rules/evidence.md: drive the guard from its
    entry point, not only the helper)."""

    def _drive(
        self, verdicts, status=None, baseline_audits=None, reqs=None, tags=None, **kw
    ):
        stem = "rfc9999"
        reqs = [self.req] if reqs is None else reqs
        tags = self.tags if tags is None else tags
        overrides = dict(
            load_enrolled=lambda: {stem},
            summary_stems=lambda: {stem},
            parse_summary_file=lambda path: reqs,
            _git_baseline_enrolment=lambda: {stem},
            _git_baseline_ids=lambda: {r.rid for r in reqs},
            _git_baseline_tag_polarities=lambda: {},
            _git_baseline_evidence=lambda: {},
            _git_baseline_summary_stems=lambda: {stem},
            _git_baseline_audits=lambda: baseline_audits or {},
            scan_tree=lambda *a, **k: tags,
            signed_extractions=lambda reqs_: {},
            check_extraction_signoff=lambda *a, **k: [],
            check_extraction_ratchet=lambda *a, **k: [],
            check_drain_floor=lambda *a, **k: [],
            # The four ledger-edge checks (plan/spec-rfcgate-4-ledger.md), neutralised for
            # the same reason check_status_agreement above is: they read the REAL
            # rfc/not-enrolled.txt and docs/features/rfc-status.md, and this driver
            # declares a SYNTHETIC summary universe. Judging 157 real status rows and 7
            # real dispositions against it produces a wall of violations that has nothing
            # to do with this driver's subject. Each of the four has its own wiring class,
            # which is where a lost call site is caught.
            check_summary_disposition=lambda *a, **k: [],
            check_status_completeness=lambda *a, **k: [],
            check_unproven_support=lambda *a, **k: [],
            check_gap_count_agreement=lambda *a, **k: [],
            check_ledger_fresh=lambda *a, **k: [],
            # check_enrolment refuses an enrolled stem with no source text under rfc/full/, which
            # is child 1's rule and nothing to do with the audit record.
            source_text=lambda s: "1.  Intro\n\nA speaker MUST do the thing.\n",
            source_path=lambda s: f"rfc/full/{s}.txt",
        )
        overrides.update(kw)
        with _audit_tree(files={stem: _audit_file(stem, verdicts)}, status=status):
            with _patched(**overrides):
                return _run_capturing(R.run_check)


class TestAuditSchemaWiring(_AuditDrive):
    def test_run_check_fails_on_an_illegal_verdict_value(self):
        v = self.verdict()
        v["verdict"] = "implemented"
        code, out = self._drive({self.req.rid: v})
        self.assertEqual(code, 2, out)
        self.assertIn("implemented", out)

    def test_run_check_fails_on_a_dangling_rid(self):
        code, out = self._drive({"RFC9999-9-9": self.verdict()})
        self.assertEqual(code, 2, out)
        self.assertIn("RFC9999-9-9", out)

    def test_run_check_fails_on_enforced_with_no_tests(self):
        v = self.verdict()
        v["tests"], v["units"] = {}, {}
        code, out = self._drive({self.req.rid: v})
        self.assertEqual(code, 2, out)
        self.assertIn("empty 'tests'", out)

    def test_run_check_clean_on_a_well_formed_verdict(self):
        """The discriminating twin for every wiring test above."""
        code, out = self._drive({self.req.rid: self.verdict()})
        self.assertEqual(code, 0, out)

    def test_run_check_reports_the_audit_bound_on_a_clean_run(self):
        """The semantic half's coverage is stated out loud, including when it is small: a gate
        that reports OK while its judgement half covers one RFC in 166 tells a reader something
        it has not measured."""
        code, out = self._drive({self.req.rid: self.verdict()})
        self.assertEqual(code, 0, out)
        self.assertIn("audit", out)
        self.assertIn("proven", out)
        self.assertIn("sampled", out)

    def test_a_malformed_audit_file_exits_two_cleanly(self):
        """A traceback would hide every other violation in the repository."""
        stem = "rfc9999"
        with _audit_tree(files={stem: "{not json"}):
            with _patched(
                load_enrolled=lambda: {stem},
                summary_stems=lambda: {stem},
                parse_summary_file=lambda path: [self.req],
                scan_tree=lambda *a, **k: self.tags,
            ):
                code, out = _run_capturing(R.run_check)
        self.assertEqual(code, 2, out)
        self.assertIn("cannot run", out)


class TestAuditUnitFreshnessWiring(_AuditDrive):
    def test_run_check_reports_shifted_through_the_entry_point(self):
        v = self.verdict()
        v["tests"] = {self.key: "0" * 16}
        code, out = self._drive({self.req.rid: v})
        self.assertEqual(code, 2, out)
        self.assertIn("SHIFTED", out)
        self.assertIn("make ze-rfc-reseal", out)


class TestAuditDisclosureWiring(_AuditDrive):
    def test_run_check_fails_on_an_undisclosed_finding(self):
        code, out = self._drive(
            {self.req.rid: self.verdict(value="wrong")}, status=_STATUS_CLEAN
        )
        self.assertEqual(code, 2, out)
        self.assertIn("clean support", out)

    def test_run_check_passes_when_the_row_discloses(self):
        code, out = self._drive(
            {self.req.rid: self.verdict(value="wrong")}, status=_STATUS_DISCLOSED
        )
        self.assertEqual(code, 0, out)


class TestAuditNoteWiring(_AuditDrive):
    def test_run_check_fails_on_a_note_citing_nothing(self):
        code, out = self._drive(
            {self.req.rid: self.verdict(note="looks fine, seems correct enough")}
        )
        self.assertEqual(code, 2, out)
        self.assertIn("tokens checked", out)


class TestAuditRatchetWiring(_AuditDrive):
    def test_run_check_fails_when_a_verdict_is_deleted(self):
        code, out = self._drive(
            {}, baseline_audits={"rfc9999": {self.req.rid: self.verdict()}}
        )
        self.assertEqual(code, 2, out)
        self.assertIn("monotonic", out)

    def test_run_check_fails_when_a_finding_is_deleted(self):
        code, out = self._drive(
            {}, baseline_audits={"rfc9999": {self.req.rid: self.verdict(value="weak")}}
        )
        self.assertEqual(code, 2, out)
        self.assertIn("DELETED", out)

    def test_run_check_passes_on_a_freshly_recorded_weak_verdict(self):
        """AC-10 at the entry point: recording a finding is green, end to end."""
        code, out = self._drive({self.req.rid: self.verdict(value="weak")})
        self.assertEqual(code, 0, out)


class TestAuditUpgradeGuardWiring(_AuditDrive):
    def test_run_check_fails_on_a_silent_upgrade(self):
        code, out = self._drive(
            {self.req.rid: self.verdict()},
            baseline_audits={"rfc9999": {self.req.rid: self.verdict(value="weak")}},
        )
        self.assertEqual(code, 2, out)
        self.assertIn("upgrade_reason", out)

    def test_run_check_passes_with_a_recorded_reason(self):
        code, out = self._drive(
            {
                self.req.rid: self.verdict(
                    upgrade_reason="re-read S2; the earlier judgement was wrong"
                )
            },
            baseline_audits={"rfc9999": {self.req.rid: self.verdict(value="weak")}},
        )
        self.assertEqual(code, 0, out)


class TestAuditFilesWiring(_AuditDrive):
    def test_run_check_fails_on_an_audit_file_for_an_unenrolled_stem(self):
        stem = "rfc9999"
        with _audit_tree(
            files={
                stem: _audit_file(stem, {self.req.rid: self.verdict()}),
                "rfc4242": _audit_file("rfc4242", {}),
            }
        ):
            with _patched(
                load_enrolled=lambda: {stem},
                summary_stems=lambda: {stem},
                parse_summary_file=lambda path: [self.req],
                _git_baseline_enrolment=lambda: {stem},
                _git_baseline_ids=lambda: {self.req.rid},
                _git_baseline_tag_polarities=lambda: {},
                _git_baseline_evidence=lambda: {},
                _git_baseline_summary_stems=lambda: {stem},
                _git_baseline_audits=lambda: {},
                scan_tree=lambda *a, **k: self.tags,
                signed_extractions=lambda reqs_: {},
                check_extraction_signoff=lambda *a, **k: [],
                check_extraction_ratchet=lambda *a, **k: [],
                check_drain_floor=lambda *a, **k: [],
                # The four ledger-edge checks (plan/spec-rfcgate-4-ledger.md), neutralised for
                # the same reason check_status_agreement above is: they read the REAL
                # rfc/not-enrolled.txt and docs/features/rfc-status.md, and this driver
                # declares a SYNTHETIC summary universe. Judging 157 real status rows and 7
                # real dispositions against it produces a wall of violations that has nothing
                # to do with this driver's subject. Each of the four has its own wiring class,
                # which is where a lost call site is caught.
                check_summary_disposition=lambda *a, **k: [],
                check_status_completeness=lambda *a, **k: [],
                check_unproven_support=lambda *a, **k: [],
                check_gap_count_agreement=lambda *a, **k: [],
                check_ledger_fresh=lambda *a, **k: [],
            ):
                code, out = _run_capturing(R.run_check)
        self.assertEqual(code, 2, out)
        self.assertIn("rfc4242", out)


class TestAuditLedger(_AuditFixture):
    """AC-18/AC-24. The verdict field being inert inverted the skill's own incentive: the skill
    says `weak` and `wrong` are the valuable outputs, and the gate treated them as identical to
    `enforced`, so recording a finding cost the auditor effort and bought the project nothing.
    The first thing a finding needs is to be VISIBLE."""

    def _audited(self, value="enforced", **extra):
        """The audit fixture on disk, for the duration of the block."""
        return _audit_tree(
            files={
                "rfc9999": _audit_file(
                    "rfc9999", {self.req.rid: self.verdict(value=value, **extra)}
                )
            }
        )

    def _render(self, value="enforced", **extra):
        """The INDEX: the audit coverage section and its worklist, which are corpus-wide."""
        with self._audited(value, **extra):
            return R.render_index([self.req], self.tags, {"rfc9999"})

    def _shard(self, value="enforced", **extra):
        """The rfc9999 SHARD: the per-row audit marker, which sits on the requirement."""
        with self._audited(value, **extra):
            return R.render_shards([self.req], self.tags, {"rfc9999"})["rfc9999"]

    def test_coverage_section_is_derived(self):
        body = self._render()
        self.assertIn("## Audit coverage", body)
        self.assertIn("| `rfc9999` | 1 | 1 | 1 | 0 | 0 |", body)

    def test_weak_verdict_removes_proven_status(self):
        """AC-24: the requirement has BOTH polarities and is NOT proven, and the ledger says both
        without contradicting itself. The discriminating twin is
        TestAuditRatchetWiring.test_run_check_passes_on_a_freshly_recorded_weak_verdict, which
        proves the same fixture exits 0 (AC-10).

        The two halves land in two files now: the count is a corpus fact and stays in the index,
        the marker contradicts a row and must be visible where that row is."""
        self.assertIn("| `rfc9999` | 1 | 1 | 0 | 1 | 0 |", self._render(value="weak"))
        self.assertIn("**audit: weak**", self._shard(value="weak"))

    def test_findings_worklist_names_each_rid(self):
        """AC-18: a blur is not a worklist."""
        body = self._render(value="weak")
        self.assertIn("### Audited but not proven", body)
        self.assertIn(f"| `{self.req.rid}` | `weak` |", body)
        self.assertIn("cannot fail on non-compliance", body)

    def test_the_polarity_rollup_is_untouched(self):
        """C-5. Child 2's doctrine: **Both** and **One polarity** are the POLARITY view, never a
        total to adjust. Subtracting an audit verdict from Both would also break the partition
        scripts/dev/testing_health.py asserts (`gated - both` must equal the annotation split, or
        it raises rather than publishing a non-partition as one)."""
        weak = self._render(value="weak")
        enforced = self._render()
        for body in (weak, enforced):
            self.assertIn(
                "| `rfc9999` | 1 | 1 | 0 | 0 | 0 | 0 | 0 | **enrolled** |", body
            )

    def test_a_stale_verdict_is_not_published_as_proven(self):
        """A stale verdict describes a test that has since changed, so publishing it as proof is
        the stale assurance the whole machinery exists to stop."""
        v = self.verdict()
        v["units"] = {self.key: "0" * 16}
        with _audit_tree(files={"rfc9999": _audit_file("rfc9999", {self.req.rid: v})}):
            index = R.render_index([self.req], self.tags, {"rfc9999"})
            shard = R.render_shards([self.req], self.tags, {"rfc9999"})["rfc9999"]
        self.assertIn("| `rfc9999` | 1 | 1 | 0 | 1 | 0 |", index)
        self.assertIn("stale-unit", index)
        self.assertIn("stale-unit", shard, "the row that carries the claim says so too")

    def test_an_unaudited_requirement_is_counted_as_such(self):
        with _audit_tree():
            body = R.render_index([self.req], self.tags, {"rfc9999"})
        self.assertIn("| `rfc9999` | 1 | 0 | 0 | 0 | 1 |", body)
        self.assertIn("never fails", body)

    def test_the_render_is_stable(self):
        """check_ledger_fresh compares bytes, so an unstable render would report a fresh ledger
        as stale on another machine."""
        self.assertEqual(self._render(), self._render())
        self.assertEqual(self._shard(), self._shard())


class TestAuditCoverageCountsEveryVerdict(_AuditFixture):
    """The gate's green output said every verdict it holds is proven while the ledger named three
    that are not.

    `audit_coverage` derived its `both` flag from TAGS alone and never consulted
    `req.annotation`, whereas `_verdict_claims` exempts a `{single-polarity}` requirement from the
    both-polarity demand -- the annotation IS the second polarity's justification. So a
    requirement whose coverage is ANNOTATED fell outside `Auditable`, was counted in no column,
    and when its verdict was a fresh `enforced` it was not in the worklist either.

    Measured on the tree: 52 verdicts recorded, `audited=44 proven=44 worklist=3`. Five verdicts
    appeared NOWHERE (all `enforced` + `{single-polarity}` + fresh), so the ledger's own claim
    that the worklist partitions the VERDICTS was arithmetically false: 44 + 3 of 52.
    """

    def _single_polarity(self, rid="RFC9999-3-1"):
        ann = R.Annotation(
            kind="single-polarity",
            polarity="positive",
            reason="no conforming input exists",
        )
        req = _req(rid, rfc="rfc9999", annotation=ann)
        tag = _tag(rid, "positive", file=self.rel, line=self.line)
        return req, tag, _verdict(req, self.rel, self.line, tags=[tag])

    def _annotation_only(self, rid="RFC9999-4-1"):
        """An `unimplemented` verdict over a `{gap}` line: a verdict with no tagged test at all."""
        ann = R.Annotation(kind="gap", polarity=None, reason="deliberate divergence")
        req = _req(rid, rfc="rfc9999", annotation=ann)
        v = _verdict(req, self.rel, self.line, value=R.VERDICT_UNIMPLEMENTED)
        v["tests"], v["units"] = {}, {}
        v["code"] = R.unit_shas([self.key])
        return req, v

    def test_an_annotated_requirement_is_auditable_and_proven(self):
        req, tag, v = self._single_polarity()
        rows, worklist = R.audit_coverage(
            [req], [tag], {"rfc9999"}, {"rfc9999": {req.rid: v}}
        )
        row = next(r for r in rows if r.rfc == "rfc9999")
        self.assertEqual(
            (row.auditable, row.audited, row.proven, row.findings, row.verdicts),
            (1, 1, 1, 0, 1),
        )
        self.assertEqual(worklist, [])

    def test_the_schema_agrees_that_it_is_auditable(self):
        """The two must not disagree about what complete polarity coverage IS: the exemption in
        `_verdict_claims` is what makes this verdict legal in the first place, so a coverage walk
        that calls the same requirement un-auditable is reading a different rule."""
        req, tag, v = self._single_polarity()
        self.assertEqual(
            R.check_audit_schema([req], [tag], {"rfc9999"}, {"rfc9999": {req.rid: v}}),
            [],
        )

    def test_one_polarity_without_an_annotation_is_not_auditable(self):
        """The discriminating twin: the exemption is the ANNOTATION, never the bare fact that one
        polarity exists. Without it, AC-6 refuses the verdict and the pair is still owed."""
        req = _req("RFC9999-3-1", rfc="rfc9999")
        tag = _tag(req.rid, "positive", file=self.rel, line=self.line)
        v = _verdict(req, self.rel, self.line, tags=[tag])
        rows, _worklist = R.audit_coverage(
            [req], [tag], {"rfc9999"}, {"rfc9999": {req.rid: v}}
        )
        row = next(r for r in rows if r.rfc == "rfc9999")
        self.assertEqual(row.auditable, 0)
        self.assertTrue(
            R.check_audit_schema([req], [tag], {"rfc9999"}, {"rfc9999": {req.rid: v}})
        )

    def test_every_recorded_verdict_is_proven_or_named(self):
        """The invariant the ledger asserts, over the three shapes that used to fall between the
        columns and the worklist: both-polarity proven, annotated single-polarity proven, and an
        annotation-only `unimplemented` with no tagged test at all."""
        req_one, tag_one, v_one = self._single_polarity()
        req_gap, v_gap = self._annotation_only()
        audits = {
            "rfc9999": {
                self.req.rid: self.verdict(),
                req_one.rid: v_one,
                req_gap.rid: v_gap,
            }
        }
        rows, worklist = R.audit_coverage(
            [self.req, req_one, req_gap],
            list(self.tags) + [tag_one],
            {"rfc9999"},
            audits,
        )
        recorded = sum(len(v) for v in audits.values())
        self.assertEqual(recorded, 3)
        self.assertEqual(sum(r.verdicts for r in rows), recorded)
        self.assertEqual(sum(r.proven for r in rows) + len(worklist), recorded)
        self.assertEqual(sum(r.findings for r in rows), len(worklist))
        self.assertEqual([rid for _rfc, rid, _reason in worklist], [req_gap.rid])

    def test_the_ledger_publishes_the_annotated_requirement(self):
        """AC-24's reporting surface. A proven verdict that appears in no row and no worklist line
        is a judgement the ledger holds and does not publish."""
        req, tag, v = self._single_polarity()
        with _audit_tree(files={"rfc9999": _audit_file("rfc9999", {req.rid: v})}):
            body = R.render_index([req], [tag], {"rfc9999"})
        self.assertIn("| `rfc9999` | 1 | 1 | 1 | 0 | 0 |", body)


class TestAuditSummaryLineAgreesWithTheLedger(_AuditDrive):
    """`run_check` summed `r.findings` and DISCARDED the worklist it had just computed, so
    `make ze-rfc-check` printed `44 proven, 0 audited-but-not-proven, of 44 verdict(s)` on a tree
    holding two `unimplemented` gaps and one `not-applicable`. The ledger reconciled it in prose;
    the one line an operator actually reads did not. It is the exact reporting surface AC-24
    exists to make honest.
    """

    def _annotation_only(self, rid="RFC9999-4-1"):
        ann = R.Annotation(kind="gap", polarity=None, reason="deliberate divergence")
        req = _req(rid, rfc="rfc9999", annotation=ann)
        v = _verdict(req, self.rel, self.line, value=R.VERDICT_UNIMPLEMENTED)
        v["tests"], v["units"] = {}, {}
        v["code"] = R.unit_shas([self.key])
        return req, v

    def test_the_audit_line_counts_the_unproven_verdicts(self):
        req_gap, v_gap = self._annotation_only()
        code, out = self._drive(
            {self.req.rid: self.verdict(), req_gap.rid: v_gap},
            reqs=[self.req, req_gap],
        )
        self.assertEqual(code, 0, out)
        self.assertIn("1 proven", out)
        self.assertIn("1 audited-but-not-proven", out)
        self.assertIn("of 2 verdict(s)", out)

    def test_a_clean_run_still_reports_zero_unproven(self):
        """The discriminating twin: the count must come from the worklist, not be a constant."""
        code, out = self._drive({self.req.rid: self.verdict()})
        self.assertEqual(code, 0, out)
        self.assertIn("1 proven", out)
        self.assertIn("0 audited-but-not-proven", out)
        self.assertIn("of 1 verdict(s)", out)


class TestAuditCountersCoverTheWholeTree(unittest.TestCase):
    """The fixture classes prove the rule; this proves it against the real audit records. The
    defect it pins was invisible precisely because it only appears once a verdict sits on an
    annotated requirement, which no fixture had and the tree has five of."""

    @classmethod
    def setUpClass(cls):
        cls.enrolled = R.load_enrolled()
        reqs = []
        for stem in sorted(R.summary_stems()):
            try:
                reqs.extend(
                    R.parse_summary_file(os.path.join(R.SUMMARY_DIR, stem + ".md"))
                )
            except R.ParseError:
                continue
        cls.reqs = reqs
        cls.tags = R.scan_tree()
        cls.audits = R.load_audits(cls.enrolled)

    def test_every_recorded_verdict_is_counted_somewhere(self):
        rows, worklist = R.audit_coverage(
            self.reqs, self.tags, self.enrolled, self.audits
        )
        recorded = sum(len(v) for v in self.audits.values())
        self.assertGreater(recorded, 0, "the corpus test must not cover nothing")
        self.assertEqual(
            sum(r.verdicts for r in rows),
            recorded,
            "a recorded verdict is counted in no RFC's row",
        )
        self.assertEqual(
            sum(r.proven for r in rows) + len(worklist),
            recorded,
            "the worklist must partition the verdicts with `proven`",
        )
        self.assertEqual(sum(r.findings for r in rows), len(worklist))

    def test_the_ledger_and_the_worklist_agree_on_the_tree(self):
        """`Not proven` in the published table and the worklist beneath it are one number."""
        rows, worklist = R.audit_coverage(
            self.reqs, self.tags, self.enrolled, self.audits
        )
        body = "\n".join(R._render_audit_coverage(self.reqs, self.tags, self.enrolled))
        recorded = sum(len(v) for v in self.audits.values())
        self.assertEqual(sum(r.findings for r in rows), len(worklist))
        self.assertIn(f"names every one of those {len(worklist)}", body)
        self.assertIn(f"all {recorded} recorded verdict(s)", body)


class TestAuditTableCannotBeMistakenForTheRollup(unittest.TestCase):
    """C-6. scripts/dev/testing_health.py pins the polarity rollup with a nine-cell regex and
    matches it against EVERY line of the ledger, raising rather than yielding zero when the shape
    moves. A new table whose rows had the same shape would be silently folded into that tool's
    proof-density figure -- a wrong number that nothing would report."""

    def _health_row_re(self):
        src = _read_repo("scripts/dev/testing_health.py")
        m = re.search(r"RFC_ROW = re\.compile\(\s*r\"(?P<pat>.+?)\"\s*\)", src, re.S)
        self.assertIsNotNone(m, "testing_health.py no longer spells RFC_ROW this way")
        return re.compile(m.group("pat"))

    def test_no_audit_row_matches_the_health_tools_rollup_pattern(self):
        row_re = self._health_row_re()
        reqs = [_req("RFC9999-2-1", rfc="rfc9999")]
        tags = [_tag("RFC9999-2-1", p) for p in ("positive", "negative")]
        rows = R._render_audit_coverage(reqs, tags, {"rfc9999"})
        matched = [line for line in rows if row_re.match(line.strip())]
        self.assertEqual(
            matched,
            [],
            "an audit row parses as a polarity-rollup row; testing_health.py would fold it "
            "into its proof-density figure",
        )

    def test_the_rollup_header_it_pins_is_unchanged(self):
        src = _read_repo("scripts/dev/testing_health.py")
        m = re.search(r"RFC_TABLE_HEADER = \(\s*(?P<body>.+?)\)\n", src, re.S)
        pinned = "".join(re.findall(r'"([^"]*)"', m.group("body")))
        rollup = R._render_rollup(
            {"rfc9999": [_req("RFC9999-2-1", rfc="rfc9999")]}, {}, {"rfc9999"}
        )
        self.assertIn(pinned, rollup)


class TestTaggedScopeCorpus(unittest.TestCase):
    """AC-23/A-3, run over the REAL tree like TestRealTree does.

    The standing guard behind the unit fingerprint: every tag in the tree must resolve to a unit
    span or an explicit file-scope marker, and never to an empty result that would hash to a
    constant. An empty extraction is the false-FRESH failure -- the one catastrophic outcome.
    """

    def test_every_tag_in_the_tree_resolves(self):
        tags = R.scan_tree()
        self.assertGreater(len(tags), 2000, "the corpus test must not cover nothing")
        cache = {}
        kinds = {}
        for t in tags:
            content = R._read_source(t.file, R.PROJECT_DIR, cache)
            self.assertTrue(content, f"{t.file}: a tagged file must be readable")
            text, kind = R.rfc_tagged_scope.unit_at(t.file, content, t.line)
            self.assertTrue(text, f"{t.file}:{t.line} resolved to an empty unit")
            self.assertIn(
                kind, (R.rfc_tagged_scope.SCOPE_FUNC, R.rfc_tagged_scope.SCOPE_FILE)
            )
            kinds[kind] = kinds.get(kind, 0) + 1
        self.assertGreater(
            kinds.get(R.rfc_tagged_scope.SCOPE_FUNC, 0),
            2000,
            "if every Go tag fell back to file scope the unit fingerprint would be a no-op",
        )

    def test_every_go_tag_sits_in_exactly_one_span(self):
        """A-3, measured rather than assumed. The trap: go_func_scopes returns CHARACTER
        OFFSETS, so comparing a tag's LINE against a span reads as a clean corpus while checking
        nothing."""
        cache = {}
        multi, outside = [], []
        for t in R.scan_tree():
            if not t.file.endswith(".go"):
                continue
            content = R._read_source(t.file, R.PROJECT_DIR, cache)
            spans = R.rfc_tagged_scope.go_func_scopes(content)
            off = R.rfc_tagged_scope.line_offset(content, t.line)
            hit = [s for s in spans if s[0] <= off < s[1]]
            if len(hit) > 1:
                multi.append(f"{t.file}:{t.line}")
            elif not hit:
                outside.append(f"{t.file}:{t.line}")
        self.assertEqual(multi, [], "a tag inside two spans has no single honest unit")
        self.assertEqual(
            outside, [], "a tag outside every span falls back to file scope"
        )

    def test_every_go_tag_names_a_symbol_that_resolves_back(self):
        """The key form, over the real corpus. A key is only as good as its round trip: the name
        `func_name_at` takes out of a span must find that same span again through `func_text`, or
        the gate fingerprints one text and checks another.

        Measured, not assumed, because the two readers are different code. `func_name_at` walks
        spans and reads a declaration; `func_text` searches every declaration for a name. A file
        holding two methods of the same name breaks the second without touching the first, and
        the honest answer there is a refusal rather than either one of the two."""
        cache = {}
        unresolved = []
        for t in R.scan_tree():
            if not t.file.endswith(".go"):
                continue
            content = R._read_source(t.file, R.PROJECT_DIR, cache)
            name = R.rfc_tagged_scope.func_name_at(t.file, content, t.line)
            if name is None:
                continue  # file scope: the bare path is the key, nothing to resolve
            span_text = R.rfc_tagged_scope.unit_at(t.file, content, t.line)[0]
            if R.rfc_tagged_scope.func_text(content, name) != span_text:
                unresolved.append(f"{t.file} {name}")
        self.assertEqual(
            unresolved,
            [],
            "a tagged unit whose key cannot find it again is not fingerprintable",
        )

    def test_the_live_audit_records_are_all_fresh(self):
        """The real tree's own verdicts must satisfy the schema and be current. Fixtures prove
        the checks fire; this proves the shipped evidence passes them."""
        enrolled, reqs, _, tags, _ = R._collect_for_check()
        audits = R.load_audits(enrolled)
        self.assertTrue(any(audits.values()), "no audit file was loaded at all")
        self.assertEqual(R.check_audit_schema(reqs, tags, enrolled, audits), [])
        self.assertEqual(R.check_audit_freshness(reqs, tags, enrolled, audits), [])
        self.assertEqual(R.check_audit_note(reqs, tags, enrolled, audits), [])


class TestTaggedScopeCoversEveryCarrier(unittest.TestCase):
    """C-4a. The edit-time guard's file predicate reads TAG_CARRIER_SUFFIXES, which mirrors the
    scanner's carrier table. When plan/spec-rfcgate-2-evidence.md admitted interop `check.py`
    evidence, two files began carrying RFC obligations the guard could not see AT ALL -- because
    that predicate was a hand-written literal in the hook. This holds the two together."""

    def test_every_carrier_suffix_is_a_known_tag_carrier(self):
        missing = sorted(
            {c.suffix for c in R.CARRIERS}
            - {
                s
                for s in {c.suffix for c in R.CARRIERS}
                if R.rfc_tagged_scope.is_tag_carrier("test/x/y" + s)
            }
        )
        self.assertEqual(
            missing,
            [],
            "a carrier the scanner counts as evidence that the edit-time guard cannot see",
        )

    def test_a_non_carrier_is_not_claimed(self):
        for path in ("internal/a/foo.go", "docs/x.md", "scripts/dev/tool.py"):
            self.assertFalse(R.rfc_tagged_scope.is_tag_carrier(path), path)

    def test_the_hook_uses_the_shared_definition(self):
        """AC-22: exactly ONE definition. A second copy that drifted would let the gate re-seal
        a verdict against a hash the guard does not compute."""
        hook = _read_repo(".claude/hooks/pretool-writeedit.py")
        self.assertIn("rfc_tagged_scope.py", hook)
        self.assertIn("_rfc_scope.tag_scope(", hook)
        self.assertNotIn(
            "def _go_func_scopes",
            hook,
            "the hook kept a private copy of the span logic",
        )


class TestVerdictVocabularyAgreesWithTheSkill(unittest.TestCase):
    """The gate's enum and `ai/skills/ze-rfc-audit.md`'s table are the same vocabulary read by a
    machine and by an agent. A drift makes the skill teach the fleet to write records the schema
    refuses -- which is how `implemented` got into the one existing audit file
    (`ai/rules/evidence.md`)."""

    def test_the_skill_documents_exactly_the_gates_enum(self):
        skill = _read_repo("ai/skills/ze-rfc-audit.md")
        # Scoped to the VERDICT table by its own header, not to every backticked table cell: the
        # skill also tables the freshness states and the record's fields, and a pattern loose
        # enough to catch those would pass whatever it happened to find.
        head = skill.index("| Verdict | Meaning |")
        table = skill[head:].split("\n\n", 1)[0]
        documented = set(re.findall(r"^\| `([a-z-]+)` \| ", table, re.M))
        self.assertEqual(
            documented,
            set(R.AUDIT_VERDICTS),
            "the skill's verdict table and AUDIT_VERDICTS have drifted",
        )

    def test_enforced_is_the_only_proven_verdict(self):
        self.assertEqual(
            R.UNPROVEN_VERDICTS, set(R.AUDIT_VERDICTS) - {R.VERDICT_ENFORCED}
        )
        self.assertTrue(R.FINDING_VERDICTS < R.UNPROVEN_VERDICTS)

    def test_every_unproven_verdict_has_a_published_meaning(self):
        """The worklist explains each row. A verdict added to the enum without a meaning would
        render a row a reader silently skips."""
        for value in R.UNPROVEN_VERDICTS:
            self.assertNotIn("add one to", R._verdict_meaning(value), value)

    def test_a_value_outside_the_vocabulary_says_so(self):
        """Its discriminating twin, and the reason it exists: the test above passes whether or
        not the vocabulary CHECK is present, because it only ever asks about members. Mutation
        found that -- disabling the membership guard left it green."""
        self.assertEqual(
            R._verdict_meaning("implemented"), "outside the recorded vocabulary"
        )
        self.assertEqual(
            R._verdict_meaning(R.VERDICT_ENFORCED), "outside the recorded vocabulary"
        )


class TestFreshnessStatesAreTotal(unittest.TestCase):
    """Critical Review, Correctness: the four states must be mutually exclusive AND total. A
    fifth, implicitly-fresh outcome is the false-fresh failure wearing a shrug."""

    STATES = None

    def setUp(self):
        self.STATES = {R.FRESH, R.SHIFTED, R.STALE_UNIT, R.STALE_REQUIREMENT}
        self.assertEqual(
            len(self.STATES), 4, "the four states must be distinct strings"
        )

    def test_every_input_shape_yields_one_of_the_four(self):
        sha = R.requirement_sha("x")
        shas = {"a_test.go:1": "f" * 16}
        units = {"a_test.go:1": "u" * 16}
        other = {"a_test.go:1": "0" * 16}
        code = {"p.go:1": "c" * 16}
        shapes = [
            {},
            {"requirement_sha": sha},
            {"requirement_sha": sha, "tests": shas},
            {"requirement_sha": sha, "tests": shas, "units": units},
            {"requirement_sha": sha, "tests": other, "units": units},
            {"requirement_sha": sha, "tests": shas, "units": other},
            {"requirement_sha": sha, "tests": {}, "units": {}, "code": code},
            {"requirement_sha": "nope", "tests": shas, "units": units},
        ]
        for shape in shapes:
            state, moved = R.verdict_freshness(shape, sha, shas, units, code)
            self.assertIn(state, self.STATES, shape)
            self.assertIsInstance(moved, list)

    def test_a_requirement_change_never_reads_as_reseal_able(self):
        """Order is the bias: SHIFTED is the only re-sealable state, so no input that changed a
        judgement may reach it."""
        shas = {"a_test.go:1": "f" * 16}
        units = {"a_test.go:1": "u" * 16}
        for shape in (
            {"requirement_sha": "old", "tests": {}, "units": units},
            {
                "requirement_sha": "old",
                "tests": shas,
                "units": {"a_test.go:1": "z" * 16},
            },
        ):
            state, _ = R.verdict_freshness(
                shape, R.requirement_sha("x"), shas, units, {}
            )
            self.assertNotEqual(state, R.SHIFTED, shape)


class TestScopeReaderIsDeclared(unittest.TestCase):
    """F-2: `.py` fell to whole-file scope only because the Go span finder finds no `func` in
    Python -- the right answer reached for the wrong reason, and one that would have changed
    silently the day anyone taught the span finder about `def`."""

    def test_non_go_carriers_are_file_scoped_by_declaration(self):
        for path in ("test/plugin/a.ci", "test/editor/a.et", "test/interop/s/check.py"):
            self.assertEqual(
                R.rfc_tagged_scope.scope_reader(path),
                R.rfc_tagged_scope.SCOPE_FILE,
                path,
            )

    def test_go_is_span_scoped(self):
        self.assertEqual(R.rfc_tagged_scope.scope_reader("internal/a/a_test.go"), "go")

    def test_a_python_unit_is_the_whole_file(self):
        src = "# RFC requirement: RFC9999-1-1 positive -- x.\ndef check():\n    assert 1\n"
        text, kind = R.rfc_tagged_scope.unit_at("test/interop/s/check.py", src, 2)
        self.assertEqual(kind, R.rfc_tagged_scope.SCOPE_FILE)
        self.assertEqual(text, src)


class TestIndexNeverWritesAudit(_AuditFixture):
    """A-7/AC-16: `--check` is read-only and `--write` touches the ledger alone, so
    `--reseal` is the only path by which an evidence file changes without a human editing it.
    ze-rfc-index-update runs ROUTINELY, for reasons that have nothing to do with an audit."""

    def _snapshot(self, adir):
        out = {}
        for name in sorted(os.listdir(adir)):
            with open(os.path.join(adir, name), "rb") as fh:
                out[name] = fh.read()
        return out

    def test_check_and_write_modes_never_touch_audit(self):
        stem = "rfc9999"
        shifted = self.verdict()
        shifted["tests"] = {self.key: "0" * 16}
        ledger = _mkstemp(".md")
        self.addCleanup(os.remove, ledger)
        # run_write writes BOTH outputs. Patching only the ledger would put one shard per
        # summary into the real rfc/requirements/ while proving the audit tree is untouched.
        shards = _mkdtemp("audit-shards-")
        self.addCleanup(shutil.rmtree, shards, True)
        with _audit_tree(
            files={stem: _audit_file(stem, {self.req.rid: shifted})}
        ) as adir:
            before = self._snapshot(adir)
            with _patched(
                load_enrolled=lambda: {stem},
                summary_stems=lambda: {stem},
                parse_summary_file=lambda path: [self.req],
                _git_baseline_enrolment=lambda: {stem},
                _git_baseline_ids=lambda: {self.req.rid},
                _git_baseline_tag_polarities=lambda: {},
                _git_baseline_evidence=lambda: {},
                _git_baseline_summary_stems=lambda: {stem},
                _git_baseline_audits=lambda: {},
                scan_tree=lambda *a, **k: self.tags,
                signed_extractions=lambda reqs_: {},
                check_extraction_signoff=lambda *a, **k: [],
                check_extraction_ratchet=lambda *a, **k: [],
                check_drain_floor=lambda *a, **k: [],
                LEDGER_FILE=ledger,
                SHARD_DIR=shards,
            ):
                _run_capturing(R.run_check)
                self.assertEqual(
                    self._snapshot(adir), before, "--check wrote to rfc/audit/"
                )
                code, _ = _run_capturing(R.run_write)
                self.assertEqual(code, 0)
                self.assertEqual(
                    self._snapshot(adir), before, "--write wrote to rfc/audit/"
                )

    def test_only_one_make_target_reseals(self):
        """Deliverable: every write to an evidence file is intentional AND greppable."""
        hits = [
            line
            for line in _read_repo("Makefile").splitlines()
            if line.startswith("ze-rfc-reseal:")
        ]
        self.assertEqual(len(hits), 1, "ze-rfc-reseal must be exactly one make target")
        self.assertIn("--reseal", _read_repo("Makefile"))


class TestResealOnlyTouchesShifted(_AuditFixture):
    """The `--reseal` mode driven through main(), so the flag being unwired fails a test."""

    def test_main_dispatches_reseal(self):
        called = []
        with _patched(run_reseal=lambda: called.append(True) or 0):
            code = R.main(["rfc_requirements.py", "--reseal"])
        self.assertEqual(code, 0)
        self.assertEqual(called, [True], "--reseal did not reach run_reseal")

    def test_run_reseal_reports_and_exits_zero(self):
        stem = "rfc9999"
        shifted = self.verdict()
        shifted["tests"] = {self.key: "0" * 16}
        with _audit_tree(files={stem: _audit_file(stem, {self.req.rid: shifted})}):
            with _patched(
                load_enrolled=lambda: {stem},
                summary_stems=lambda: {stem},
                parse_summary_file=lambda path: [self.req],
                scan_tree=lambda *a, **k: self.tags,
            ):
                code, out = _run_capturing(R.run_reseal)
        self.assertEqual(code, 0, out)
        self.assertIn("re-stamped", out)
        self.assertIn("ze-rfc-index-update", out, "the follow-up command must be named")

    def test_run_reseal_fails_closed_on_a_malformed_record(self):
        stem = "rfc9999"
        with _audit_tree(files={stem: "{not json"}):
            with _patched(
                load_enrolled=lambda: {stem},
                summary_stems=lambda: {stem},
                parse_summary_file=lambda path: [self.req],
                scan_tree=lambda *a, **k: self.tags,
            ):
                code, out = _run_capturing(R.run_reseal)
        self.assertEqual(code, 2, out)
        self.assertIn("cannot run", out)


class TestNotApplicableWiring(_AuditDrive):
    """The documented authoring path, driven end-to-end.

    Implementation Step 9's criterion is that a reader following `ai/skills/ze-rfc-audit.md`
    produces a record that passes the schema on the FIRST try. The skill says `tests` is for
    "every verdict except `not-applicable`", so a reader omits it -- and that spelling exited 2
    with a message whose remedy was false, while the spelling nothing documented exited 0.
    """

    def _na(self, spelling):
        ann = R.Annotation(
            kind="not-applicable", polarity=None, reason="binds future authors"
        )
        req = _req(self.req.rid, rfc="rfc9999", annotation=ann)
        v = self.verdict(value=R.VERDICT_NOT_APPLICABLE)
        v["no_code_path"] = "section 8 binds the authors of future specifications"
        v["units"] = {}
        if spelling == "empty":
            v["tests"] = {}
        else:
            del v["tests"]
        return req, v

    def test_run_check_accepts_the_documented_omitted_tests_map(self):
        for spelling in ("empty", "omitted"):
            req, v = self._na(spelling)
            code, out = self._drive({req.rid: v}, reqs=[req], tags=[])
            self.assertEqual(code, 0, f"{spelling}: {out}")
            self.assertNotIn("STALE", out)

    def test_reseal_neither_re_stamps_nor_refuses_it(self):
        """The remedy the STALE message named was false in every clause. `--reseal` refused the
        record ("a human must re-read it") and re-running `/ze-rfc-audit` reproduces it
        byte-for-byte, so there was no exit at all short of guessing the literal spelling."""
        for spelling in ("empty", "omitted"):
            req, v = self._na(spelling)
            with _audit_tree(files={"rfc9999": _audit_file("rfc9999", {req.rid: v})}):
                with _patched(
                    load_enrolled=lambda: {"rfc9999"},
                    summary_stems=lambda: {"rfc9999"},
                    parse_summary_file=lambda path: [req],
                    scan_tree=lambda *a, **k: [],
                ):
                    resealed, refused = R.reseal_audits()
            self.assertEqual((resealed, refused), ([], []), spelling)


class TestWorklistMeaningNamesTheState(unittest.TestCase):
    """A non-fresh row's meaning is its STATE, not its verdict word.

    Every non-fresh reason rendered "the verdict no longer describes what it judged", which is the
    OPPOSITE of what `shifted` means -- there the tagged unit IS byte-identical and only the file
    around it moved -- so the ledger sent that reader to re-read an RFC when one mechanical
    command clears it. Reachable transiently between `ze-rfc-index-update` and `ze-rfc-reseal`.
    """

    def test_shifted_says_byte_identical_and_names_reseal(self):
        meaning = R._verdict_meaning(f"{R.VERDICT_ENFORCED} ({R.SHIFTED})")
        self.assertIn("byte-identical", meaning)
        self.assertIn("ze-rfc-reseal", meaning)
        self.assertNotIn("no longer describes", meaning)

    def test_stale_unit_says_what_it_judged_changed_and_wants_a_human(self):
        meaning = R._verdict_meaning(f"{R.VERDICT_WEAK} ({R.STALE_UNIT})")
        self.assertIn("changed", meaning)
        self.assertIn("ze-rfc-audit", meaning)
        self.assertNotIn("ze-rfc-reseal", meaning)

    def test_stale_requirement_says_the_obligation_text_moved(self):
        meaning = R._verdict_meaning(f"{R.VERDICT_ENFORCED} ({R.STALE_REQUIREMENT})")
        self.assertIn("text changed", meaning)
        self.assertNotIn("ze-rfc-reseal", meaning)

    def test_every_non_fresh_state_has_a_published_meaning(self):
        for state in (R.SHIFTED, R.STALE_UNIT, R.STALE_REQUIREMENT):
            self.assertIn(state, R._STATE_MEANING)

    def test_an_unpublished_state_says_so_rather_than_borrowing_one(self):
        """Fail closed: a fifth state must not inherit a sentence written for another one."""
        meaning = R._verdict_meaning("enforced (invented-state)")
        self.assertIn("invented-state", meaning)
        self.assertIn("_STATE_MEANING", meaning)

    def test_a_fresh_verdict_still_renders_its_verdict_meaning(self):
        """The discriminating twin: the state branch must not swallow the verdict branch."""
        self.assertIn("cannot fail", R._verdict_meaning(R.VERDICT_WEAK))


class _JSONShim:
    """`json` for one `_write_audit` call: `dump` writes a record the caller never held, `load`
    is real. The only way to make the staged BYTES differ from the in-memory dict."""

    def __init__(self, bad):
        self.bad = bad

    def dump(self, data, fh, **kw):
        json.dump(self.bad, fh, **kw)

    def load(self, fh):
        return json.load(fh)


class TestWriteAuditValidatesTheStagedBytes(_AuditFixture):
    """`_write_audit`'s docstring promised the record was "re-read through the validating parser
    BEFORE it lands"; the code validated the in-memory dict instead. `--check` reads the FILE, so
    only the file can prove what `--check` will see, and a JSON round trip is exactly where a
    writer defect would hide."""

    def _write(self, shim=None):
        stem = "rfc9999"
        good = {stem: _audit_file(stem, {self.req.rid: self.verdict()})}
        with _audit_tree(files=good) as adir:
            path = os.path.join(adir, stem + ".json")
            with open(path, encoding="utf-8") as fh:
                before = fh.read()
            with _patched(**({"json": shim} if shim else {})):
                raised = None
                try:
                    R._write_audit(stem, {self.req.rid: self.verdict()}, "a note")
                except R.ParseError as exc:
                    raised = str(exc)
            with open(path, encoding="utf-8") as fh:
                after = fh.read()
        return raised, before, after

    def test_a_record_only_broken_on_disk_is_refused(self):
        bad = _audit_file("rfc9999", {self.req.rid: {"verdict": "implemented"}})
        raised, before, after = self._write(_JSONShim(bad))
        self.assertIsNotNone(raised, "the staged bytes were never re-read")
        self.assertIn("implemented", raised)
        self.assertEqual(
            before, after, "a refusal must leave the existing evidence untouched"
        )

    def test_a_good_record_still_lands(self):
        """The discriminating twin: the extra read must not refuse the legal write."""
        raised, before, after = self._write()
        self.assertIsNone(raised, raised)
        self.assertNotEqual(before, after, "the re-stamp note must have landed")
        self.assertIn("a note", after)


class TestSkillDocumentsWhatTheSchemaAccepts(unittest.TestCase):
    """The skill is the authoring path; a schema it contradicts produces a red gate on the first
    try. Its `tests` row said "every verdict except `not-applicable`" (omit it) while the only
    spelling the freshness rule accepted was a literal empty map, and its `units` row said
    "always" while the one live `not-applicable` record carries no `units` key at all.
    """

    def _row(self, field):
        for line in _read_repo("ai/skills/ze-rfc-audit.md").splitlines():
            if line.startswith(f"| `{field}` |"):
                return line
        self.fail(f"ai/skills/ze-rfc-audit.md has no field table row for {field!r}")

    def test_the_tests_row_does_not_forbid_the_field(self):
        """ "except `not-applicable`" reads as "omit it", which is legal -- but so is an empty map,
        and a reader who cannot tell has to guess which one the gate wants."""
        row = self._row("tests")
        self.assertNotIn("except", row)
        self.assertIn("empty", row)

    def test_the_units_row_does_not_claim_to_be_unconditional(self):
        """`units` is the unit-level twin of each `tests` entry, so a verdict citing no test has
        none. "always" contradicts the record the ruling that created this state produced."""
        row = self._row("units")
        self.assertNotIn("always", row)

    def test_the_no_code_path_row_still_states_prose(self):
        """The type check added for it enforces exactly what this row promises."""
        self.assertIn("prose", self._row("no_code_path"))


# --------------------------------------------------------------------------
# The public ledger's edges (plan/spec-rfcgate-4-ledger.md)
# --------------------------------------------------------------------------
def _status_rows():
    """The committed docs/features/rfc-status.md rows.

    A helper rather than an inline `open(...).read()` so the file handle is closed: the
    unittest runner turns an unclosed handle into a ResourceWarning on stderr, and a gate
    that prints warnings on a clean run teaches a reader to skim its output.
    """
    with open(R.STATUS_FILE, encoding="utf-8") as fh:
        return R.parse_status_ledger(fh.read())


def _all_requirements():
    """Every requirement in the REAL rfc/short/ tree, for the real-file assertions."""
    out = []
    for stem in sorted(R.summary_stems()):
        out.extend(R.parse_summary_file(os.path.join(R.SUMMARY_DIR, stem + ".md")))
    return out


def _row(status="Supported", coverage="c", remaining=""):
    return {"status": status, "coverage": coverage, "remaining": remaining}


def _disp(kind="backlog", reason="the extraction is owed"):
    return R.Disposition(kind=kind, reason=reason)


def _gap(reason="not implemented"):
    return R.Annotation(kind="gap", reason=reason, polarity="")


def _extraction(stem="rfc1", register="prose", register_reason="because"):
    """An Extraction value, for the two fields check_unproven_support reads.

    The NamedTuple directly rather than `_artifact`'s dict: what this check consumes is a
    member of run_check's SIGNED set, which is already the parsed and validated form. Going
    through the JSON writer would test the parser a second time and say nothing more about
    the escape.
    """
    return R.Extraction(
        stem=stem,
        register=register,
        source_path=f"rfc/full/{stem}.txt",
        source_sha="0" * 16,
        signed_off="2026-07-30",
        reviewer="tester",
        resign_reason="",
        register_reason=register_reason,
        sections=[],
        sites=[],
        path=f"rfc/extraction/{stem}.json",
    )


def _manual_walk_extraction(stem="rfc8888", src=None):
    """The reproduced OR-A evasion, as a member of run_check's SIGNED set.

    A VALID sign-off over _SRC_TWO_SITES: both derived sites classified (excluded
    `not-a-requirement`), the four sections dispositioned, and the register declared
    `manual-walk` with a reason. _evaluate_extraction accepts every part of it -- `manual-walk`
    is the WEAKEST grade and only a STRONGER claim than derived is refused -- so this is what a
    stem that "simply captured nothing" can put on disk today.
    """
    return R.Extraction(
        stem=stem,
        register="manual-walk",
        source_path=f"rfc/full/{stem}.txt",
        source_sha=_sha(_SRC_TWO_SITES if src is None else src),
        signed_off="2026-07-30",
        reviewer="tester",
        resign_reason="",
        register_reason="walked by hand; no obligation falls on any speaker",
        sections=[],
        sites=[],
        path=f"rfc/extraction/{stem}.json",
    )


class _LedgerEdgeDrive(unittest.TestCase):
    """Shared run_check driver for the four ledger-edge checks.

    Everything unrelated is patched out, so a failure here is one of the four checks and
    nothing else -- the same contract the audit and extraction drivers above use. What is
    NOT patched out is the check under test: a helper-only test would prove the helper and
    say nothing about whether run_check calls it (ai/rules/evidence.md, the test
    corollary).
    """

    STEM = "rfc9999"

    def _drive(
        self,
        reqs=None,
        enrolled=("rfc9999",),
        baseline_enrolled=None,
        stems=None,
        rows=None,
        dispositions=None,
        baseline_rows=None,
        baseline_dispositions=(),
        signed=None,
        tags=None,
        **kw,
    ):
        reqs = [_req("RFC9999-1-1", rfc=self.STEM)] if reqs is None else reqs
        base = set(enrolled) if baseline_enrolled is None else set(baseline_enrolled)
        stems = set(enrolled) if stems is None else set(stems)
        rows = {} if rows is None else rows
        dispositions = {} if dispositions is None else dispositions
        signed = {} if signed is None else signed
        if tags is None:
            tags = [_tag(r.rid, p) for r in reqs for p in ("positive", "negative")]
        overrides = dict(
            load_enrolled=lambda: set(enrolled),
            summary_stems=lambda: stems,
            parse_summary_file=lambda path: reqs,
            _git_baseline_enrolment=lambda: base,
            _git_baseline_ids=lambda: {r.rid for r in reqs},
            _git_baseline_tag_polarities=lambda: {},
            _git_baseline_evidence=lambda: {},
            _git_baseline_summary_stems=lambda: stems,
            scan_tree=lambda *a, **k: tags,
            parse_status_ledger=lambda text: rows,
            load_dispositions=lambda: dispositions,
            _git_baseline_status_rows=lambda: baseline_rows,
            _git_baseline_dispositions=lambda: set(baseline_dispositions),
            signed_extractions=lambda reqs_: signed,
            # Neutralised because a synthetic stem has no rfc/full/<stem>.txt and no
            # rfc/extraction/<stem>.json, so check_enrolment accuses it of both the moment a
            # test sets baseline_enrolled=() to make it newly enrolled -- which is precisely
            # what the status-completeness half needs. Its own drivers own that check.
            check_enrolment=lambda *a, **k: [],
            check_status_agreement=lambda *a, **k: [],
            check_extraction_signoff=lambda *a, **k: [],
            check_extraction_ratchet=lambda *a, **k: [],
            check_drain_floor=lambda *a, **k: [],
            check_audit_files=lambda *a, **k: [],
            check_audit_schema=lambda *a, **k: [],
            check_audit_freshness=lambda *a, **k: [],
            check_audit_disclosure=lambda *a, **k: [],
            check_audit_note=lambda *a, **k: [],
            check_audit_findings=lambda *a, **k: [],
            check_audit_verdict_ratchet=lambda *a, **k: [],
            check_ledger_fresh=lambda *a, **k: [],
        )
        overrides.update(kw)
        with _patched(**overrides):
            return _run_capturing(R.run_check)


class TestDispositionParsing(unittest.TestCase):
    """A malformed rfc/not-enrolled.txt line must be REJECTED, never skipped: skipping it
    silently un-declares a summary, which is the absence the file exists to abolish."""

    def test_disposition_file_parses_like_enrolled(self):
        """A-6: the same comment and blank-line tolerance, and the same first-token stem."""
        text = "# a comment\n\n  rfc1234\tbacklog\tthe extraction is owed  \n"
        out = R.parse_dispositions(text)
        self.assertEqual(set(out), {"rfc1234"})
        self.assertEqual(out["rfc1234"].kind, "backlog")
        self.assertEqual(out["rfc1234"].reason, "the extraction is owed")

    def test_all_three_kinds_parse(self):
        text = "\n".join(
            f"rfc{i}\t{kind}\tbecause"
            for i, kind in enumerate(sorted(R.DISPOSITION_KINDS), start=1)
        )
        self.assertEqual(len(R.parse_dispositions(text)), len(R.DISPOSITION_KINDS))

    def test_unknown_kind_is_rejected(self):
        """AC-9: the accepted kinds are named, and a fourth is not tolerated."""
        with self.assertRaises(R.ParseError) as ctx:
            R.parse_dispositions("rfc1234\tsomeday\tbecause\n")
        self.assertIn("someday", str(ctx.exception))
        self.assertIn("non-normative", str(ctx.exception))

    def test_missing_kind_is_rejected(self):
        with self.assertRaises(R.ParseError) as ctx:
            R.parse_dispositions("rfc1234\n")
        self.assertIn("no kind", str(ctx.exception))

    def test_missing_reason_is_rejected(self):
        """AC-9: a bare kind is an absence with a label on it."""
        with self.assertRaises(R.ParseError) as ctx:
            R.parse_dispositions("rfc1234\tbacklog\n")
        self.assertIn("no reason", str(ctx.exception))

    def test_whitespace_only_reason_is_rejected(self):
        """The present-but-empty case: `len(fields) > 2` is true, so an `ok`-style test
        passes and the reason is still nothing (ai/rules/evidence.md)."""
        with self.assertRaises(R.ParseError) as ctx:
            R.parse_dispositions("rfc1234\tbacklog\t   \n")
        self.assertIn("no reason", str(ctx.exception))

    def test_duplicate_stem_is_rejected(self):
        with self.assertRaises(R.ParseError) as ctx:
            R.parse_dispositions("rfc1\tbacklog\ta\nrfc1\tblocked\tb\n")
        self.assertIn("duplicate", str(ctx.exception))

    def test_the_line_number_is_reported(self):
        """An error naming no line makes a 200-row file unsearchable."""
        with self.assertRaises(R.ParseError) as ctx:
            R.parse_dispositions("# c\n\nrfc1\tbacklog\ta\nrfc2\tnope\tb\n")
        self.assertIn(":4", str(ctx.exception))


class TestSummaryDisposition(unittest.TestCase):
    """check_summary_disposition: every summary is enrolled or declared (AC-5..AC-9, AC-14)."""

    def _errs(self, stems, enrolled, dispositions, baseline=()):
        return R.check_summary_disposition(
            set(stems), set(enrolled), dispositions, set(baseline)
        )

    def test_undeclared_summary_fails(self):
        """AC-5: neither enrolled nor declared."""
        errs = self._errs({"rfc1", "rfc2"}, {"rfc1"}, {})
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("rfc2", errs[0])
        self.assertIn("rfc/not-enrolled.txt", errs[0])
        self.assertIn("rfc/enrolled.txt", errs[0])

    def test_declared_summary_passes(self):
        """The discriminating twin of the row above."""
        self.assertEqual(self._errs({"rfc1", "rfc2"}, {"rfc1"}, {"rfc2": _disp()}), [])

    def test_stem_in_both_files_fails(self):
        """AC-6: the contradiction is rejected, not resolved by precedence."""
        errs = self._errs({"rfc1"}, {"rfc1"}, {"rfc1": _disp()})
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("BOTH", errs[0])

    def test_disposition_naming_no_summary_fails(self):
        """AC-7: a decision about a summary nobody wrote."""
        errs = self._errs({"rfc1"}, {"rfc1"}, {"rfc404": _disp()})
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("rfc404", errs[0])
        self.assertIn("does not exist", errs[0])

    def test_leaving_disposition_without_enrolling_fails(self):
        """AC-8: a disposition is discharged by enrolment and by nothing else."""
        errs = self._errs({"rfc1"}, set(), {}, baseline={"rfc1"})
        self.assertTrue(any("left rfc/not-enrolled.txt" in e for e in errs), errs)

    def test_leaving_disposition_by_enrolling_passes(self):
        """AC-8's happy path, and the transition phase 6 performs four times."""
        self.assertEqual(self._errs({"rfc1"}, {"rfc1"}, {}, baseline={"rfc1"}), [])

    def test_deleting_the_summary_and_its_row_together_is_legal(self):
        """J5. A declared stem had NO legal way out of the tree. Keep the row and delete
        rfc/short/<stem>.md and the stale-disposition branch fires (AC-7); delete both and the
        left-without-enrolling branch fires (AC-8). Enrolling is the only other move, and it
        needs a summary. So a summary could be added but never removed, which is a state no
        rule asks for -- AC-8 exists to stop an EXISTING summary returning to the undeclared
        state, and a summary that is gone cannot be in it."""
        self.assertEqual(self._errs(set(), set(), {}, baseline={"rfc1"}), [])

    def test_deleting_the_row_while_the_summary_remains_still_fails(self):
        """The discriminating twin, and the case AC-8 was written for: the summary is still in
        the tree, so deleting its row does return it to the undeclared state."""
        errs = self._errs({"rfc1"}, set(), {}, baseline={"rfc1"})
        self.assertTrue(any("left rfc/not-enrolled.txt" in e for e in errs), errs)

    def test_deleting_only_the_summary_reports_the_stale_row_alone(self):
        """The third state, unchanged: the row now names a summary nobody wrote, so AC-7 fires
        and says so. One actionable error, and it names the fix (delete the row too)."""
        errs = self._errs(set(), set(), {"rfc1": _disp()}, baseline={"rfc1"})
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("does not exist", errs[0])

    def test_an_unreadable_baseline_accuses_nobody(self):
        """`baseline - current` over an empty baseline: the safe polarity, and the reason
        _git_baseline_dispositions returns a set rather than Optional."""
        self.assertEqual(self._errs({"rfc1"}, {"rfc1"}, {}, baseline=set()), [])

    def test_non_applicability_reason_is_rejected(self):
        """AC-14: `non-normative` says what the DOCUMENT states. The moment it says Ze does
        not need the obligation it has laundered an unextracted MUST into a decision."""
        for reason in (
            "not applicable to ze",
            "does not apply to ze",
            "ze does not implement inverse queries",
            "ze has no resolver",
            "we do not implement this",
            "out of scope for ze",
        ):
            errs = self._errs({"rfc1"}, set(), {"rfc1": _disp("non-normative", reason)})
            self.assertEqual(len(errs), 1, f"{reason!r} -> {errs}")
            self.assertIn("judges what ZE owes", errs[0])

    def test_a_document_property_reason_is_accepted(self):
        """The discriminating twin: the same kind with a reason about the TEXT passes."""
        reason = (
            "Informational, no RFC 2119 key-words section, and zero occurrences of any "
            "of the ten keywords anywhere in the source"
        )
        self.assertEqual(
            self._errs({"rfc1"}, set(), {"rfc1": _disp("non-normative", reason)}), []
        )

    def test_a_reason_that_cites_no_document_property_is_rejected(self):
        """J4. The phrase list is six spellings, so it was never the rule AC-14 states. Seven
        rephrasings of the same laundering walked through it. The rule is now a POSITIVE
        requirement that fails closed: a `non-normative` reason must cite something about the
        DOCUMENT, and a reason that cites nothing is refused whatever its phrasing
        (ai/rules/evidence.md)."""
        for reason in (
            "Ze is not required to do any of this",
            "Ze plays no role addressed by this document",
            "No obligation falls on us here",
            "This RFC is irrelevant for our implementation",
            "Nothing here binds an implementation like ours",
            "We are outside the scope this document addresses",
            "Our daemon owes nothing under this specification",
        ):
            errs = self._errs({"rfc1"}, set(), {"rfc1": _disp("non-normative", reason)})
            self.assertEqual(len(errs), 1, f"{reason!r} -> {errs}")
            self.assertIn("cites nothing about the DOCUMENT", errs[0])

    def test_each_accepted_form_of_document_evidence_passes(self):
        """The whitelist the message names, one reason per arm. A guard whose accepted set the
        message describes wrongly sends the author round in circles
        (ai/rules/cli.md leg 3)."""
        for reason in (
            "Informational; it carries no RFC 2119 key-words section",
            "the BCP 14 boilerplate is absent from this text",
            "Experimental, and its obligations are all stated as MAY",
            "Historic; superseded and stating no requirement",
            "a Best Current Practice about process, not about a speaker",
            "zero capitalised MUST, SHALL or REQUIRED anywhere in the source",
            "the keyword scan over rfc/full/rfc1.txt returns nothing",
        ):
            self.assertEqual(
                self._errs({"rfc1"}, set(), {"rfc1": _disp("non-normative", reason)}),
                [],
                reason,
            )

    def test_a_reason_that_does_both_reports_the_laundering_once(self):
        """Both halves can fire on one row. The Ze-judgement message is the specific one and
        wins, so the reader gets one actionable error rather than two."""
        errs = self._errs(
            {"rfc1"},
            set(),
            {"rfc1": _disp("non-normative", "Informational, and ze has no resolver")},
        )
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("judges what ZE owes", errs[0])

    def test_the_positive_requirement_only_reads_non_normative(self):
        """`backlog` and `blocked` are DEBT and assert nothing about conformance, so neither owes
        a document-property citation. Demanding one there would break every row in the file."""
        for kind in ("backlog", "blocked"):
            self.assertEqual(
                self._errs(
                    {"rfc1"},
                    set(),
                    {
                        "rfc1": _disp(
                            kind, "the extraction is owed and nobody has run it"
                        )
                    },
                ),
                [],
                kind,
            )

    def test_the_keyword_arm_is_case_sensitive(self):
        """What the arm MEANS is "the reason talks about CAPITALISED keywords", which is a claim
        about the register the document is written in. A lowercase "must" in ordinary prose is
        not that claim, and reading it as one would accept "this must be out of scope"."""
        self.assertTrue(
            R.non_normative_reason_cites_the_document("no MUST in the text")
        )
        self.assertFalse(
            R.non_normative_reason_cites_the_document("this must be out of scope")
        )

    def test_the_non_applicability_check_only_reads_non_normative(self):
        """A `backlog` reason legitimately says what Ze does not implement -- that is what
        makes it debt. Rejecting it there would forbid the honest word."""
        self.assertEqual(
            self._errs(
                {"rfc1"},
                set(),
                {"rfc1": _disp("backlog", "ze does not implement it yet")},
            ),
            [],
        )

    def test_the_real_tree_partitions(self):
        """Over the REAL rfc/ tree, not a fixture: every summary in this repository is
        enrolled or declared. A synthetic fixture would pass with the tree undeclared,
        which is the state this check exists to end."""
        self.assertEqual(
            R.check_summary_disposition(
                R.summary_stems(), R.load_enrolled(), R.load_dispositions(), set()
            ),
            [],
        )


class TestSummaryDispositionWiring(_LedgerEdgeDrive):
    """check_summary_disposition is dead code unless run_check calls it."""

    def test_run_check_fails_on_a_summary_that_is_neither_enrolled_nor_declared(self):
        code, out = self._drive(stems={"rfc9999", "rfc7777"})
        self.assertEqual(code, 2, out)
        self.assertIn("rfc7777", out)
        self.assertIn("rfc/not-enrolled.txt", out)

    def test_run_check_clean_when_every_summary_is_declared(self):
        code, out = self._drive(
            stems={"rfc9999", "rfc7777"}, dispositions={"rfc7777": _disp()}
        )
        self.assertEqual(code, 0, out)

    def test_a_malformed_disposition_file_exits_two_without_a_traceback(self):
        """A ParseError from the loader must reach run_check's handler as a clean exit 2."""

        def boom():
            raise R.ParseError(
                "rfc/not-enrolled.txt:4: kind 'nope' is not one of [...]"
            )

        code, out = self._drive(load_dispositions=boom)
        self.assertEqual(code, 2, out)
        self.assertIn("cannot run", out)
        self.assertIn("rfc/not-enrolled.txt:4", out)


# The page's own glossary sentence, quoted verbatim from docs/features/rfc-status.md so the
# premise of TestDebtStatusHonesty is PINNED to the page rather than remembered from it. If
# the vocabulary is ever rewritten, the assertion that reads this fails first and tells the
# reader to re-derive the invariant instead of silently judging by a stale definition.
_GLOSSARY_IMPLEMENTED = (
    "`Supported` means the behavior is implemented and tied to current source anchors"
)
_GLOSSARY_EXPERIMENTAL = (
    "`Experimental` means implemented but still needs deployment evidence or hardening"
)
_GLOSSARY_PARTIAL = (
    "`Partial` means a named subset is missing, intentionally skipped, or not proven"
)


def _status_claims_implemented(status: str) -> bool:
    """Does this Status cell assert the behavior IS implemented?

    Two words do, by the glossary quoted above: anything beginning `Supported` (including the
    platform-scoped forms, "Supported on Linux" and the rest -- they narrow WHERE it is
    implemented, never WHETHER) and `Experimental`, which says implemented-but-unhardened.
    `Partial`, `Unsupported`, `Not supported` and `Future` all assert the opposite.
    """
    status = status.strip()
    return status.startswith("Supported") or status == "Experimental"


class TestDebtStatusHonesty(unittest.TestCase):
    """AC-24: a Status word may not contradict its own row.

    A stem declared DEBT in rfc/not-enrolled.txt (`backlog` or `blocked`) has obligations that
    are unfinished, unproven, or unruled -- that is what the kind MEANS, and
    rfc/not-enrolled.txt says so outright: "Neither asserts anything about conformance". A
    public row claiming the behavior is IMPLEMENTED therefore contradicts the declaration
    beside it, and the page's own glossary already supplies the honest word: `Partial` is "a
    named subset is missing, intentionally skipped, or not proven".

    Two rows violated this on the committed page: RFC 1035 read `Supported` while its Remaining
    cell named six obligations with no code path (the 512-octet bound, the TC bit, the
    TTL/SOA-MINIMUM rule, the inverse-query NOTIMP reply, zone transfer), and RFC 5301 read
    `Experimental` over three unmet MUSTs. Nothing in the gate could see it: every machine
    check on that page reads presence, counts and disclosure, and Status is editorial.

    A TEST rather than a check in run_check, deliberately. `Status` is a product judgement the
    page states no gate reads, and promoting it to a gate would make the four documented
    machine-checked properties five without the page saying so. This pins the two rows that
    were wrong and the rule that makes them wrong, and it reds if either is reverted.
    """

    def setUp(self):
        self.rows = _status_rows()
        self.dispositions = R.load_dispositions()

    def test_the_glossary_still_defines_the_words_this_invariant_reads(self):
        """The premise. Without this the class could keep passing against a page that had
        redefined `Partial`, judging by a definition no longer published."""
        page = _read_repo("docs/features/rfc-status.md")
        for quote in (
            _GLOSSARY_IMPLEMENTED,
            _GLOSSARY_EXPERIMENTAL,
            _GLOSSARY_PARTIAL,
        ):
            self.assertIn(quote, page, quote)

    def test_no_debt_declared_stem_claims_its_behavior_is_implemented(self):
        """The invariant, over the REAL page and the REAL disposition file."""
        offenders = sorted(
            (stem, self.rows[stem]["status"])
            for stem, disp in self.dispositions.items()
            if disp.kind != R.DISPOSITION_NON_NORMATIVE
            and stem in self.rows
            and _status_claims_implemented(self.rows[stem]["status"])
        )
        self.assertEqual(offenders, [], offenders)

    def test_the_two_rows_that_were_wrong_are_the_ones_this_governs(self):
        """The discriminating half: the invariant above would also pass if NO declared stem had
        a row at all, which is the state that made the two wrong rows survive. Only a DEBT stem
        the page also rows can be judged here, and such a stem must read the word the glossary
        supplies.

        The set was ["rfc1035", "rfc5301"] when spec-rfcgate-4-ledger closed. rfc5301 left it
        on 2026-08-10 by ENROLLING (spec-fixit-isis-hostname-ascii), which is the one discharge
        rfc/not-enrolled.txt allows, so the set is asserted non-empty rather than frozen: a
        stem leaving by the front door must not redden this, and a stem leaving by having its
        row deleted still must."""
        rowed = sorted(
            stem
            for stem, disp in self.dispositions.items()
            if disp.kind != R.DISPOSITION_NON_NORMATIVE and stem in self.rows
        )
        self.assertIn("rfc1035", rowed, rowed)
        for stem in rowed:
            self.assertEqual(self.rows[stem]["status"].strip(), "Partial", stem)

    def test_the_remaining_cells_still_name_the_unmet_obligations(self):
        """What makes `Partial` the right word and not a downgrade for its own sake: the
        Remaining cell names the missing subset, so the row carries its own evidence. The task
        was to correct the Status word and nothing else.

        The named subset SHRINKS as obligations are met, and it must, or this test pins a debt
        the tree has paid. It read "512-octet" and "TC bit" until 2026-08-19, when the ledger
        was corrected against its producers: `send` (`internal/core/dnsserver/handler.go`)
        calls `Msg.Truncate(udpReplyLimit(r))` on a datagram transport, and `Msg.Truncate` sets
        TC. Asserting those two words today would require the page to keep publishing a claim
        that is false, which is the opposite of what this test exists to enforce. What survives
        as debt is zone transfer, so that is what the cell must name."""
        self.assertIn("zone transfer", self.rows["rfc1035"]["remaining"])
        # rfc5301 carried the third and fourth of these until 2026-08-10, when
        # spec-fixit-isis-hostname-ascii met the three unmet obligations and
        # enrolled the stem. A row that stops being debt stops owing a named
        # missing subset; what it owes instead is asserted above, in
        # test_the_declared_remainder_is_debt_not_a_decision.
        self.assertNotIn("rfc5301", R.load_dispositions())

    def test_a_status_that_narrows_the_platform_still_claims_implementation(self):
        """The predicate's boundary, in both directions. "Supported on Linux" narrows WHERE,
        not WHETHER, so it must read as a claim; `Partial` and `Future` must not."""
        for status in (
            "Supported",
            "Supported on Linux",
            "Supported within BFD",
            "Experimental",
        ):
            self.assertTrue(_status_claims_implemented(status), status)
        for status in ("Partial", "Unsupported", "Not supported", "Future", ""):
            self.assertFalse(_status_claims_implemented(status), status)


class TestStatusCompleteness(unittest.TestCase):
    """check_status_completeness: a new enrolment brings a row, a row does not vanish
    (AC-1..AC-4)."""

    def _errs(self, enrolled, rows, baseline_rows, newly, baseline_enrolled=None):
        base = set(enrolled) if baseline_enrolled is None else set(baseline_enrolled)
        return R.check_status_completeness(
            set(enrolled), rows, baseline_rows, newly, base
        )

    def test_new_enrolment_without_a_row_fails(self):
        """AC-1."""
        errs = self._errs({"rfc1"}, {}, {}, {"rfc1"}, baseline_enrolled=set())
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("newly enrolled", errs[0])
        self.assertIn("docs/features/rfc-status.md", errs[0])

    def test_new_enrolment_with_a_row_passes(self):
        errs = self._errs(
            {"rfc1"}, {"rfc1": _row()}, {}, {"rfc1"}, baseline_enrolled=set()
        )
        self.assertEqual(errs, [])

    def test_deleted_row_under_enrolment_fails(self):
        """AC-2: the row went away while the RFC stayed enrolled."""
        errs = self._errs({"rfc1"}, {}, {"rfc1": _row()}, set())
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("at HEAD and does not now", errs[0])

    def test_preexisting_rowless_enrolment_is_grandfathered(self):
        """AC-3: the 32 that predate the ratchet are not accused."""
        self.assertEqual(self._errs({"rfc1"}, {}, {}, set()), [])

    def test_absent_baseline_judges_nothing(self):
        """AC-4: `baseline_rows is None` means git could not answer, so the deletion half
        reports nothing rather than reading 157 missing rows as 157 deletions."""
        self.assertEqual(self._errs({"rfc1"}, {}, None, None), [])

    def test_an_unreadable_enrolment_baseline_judges_nothing(self):
        """`newly_enrolled is None` is the same statement about the other input. An empty
        set here would accuse every enrolled RFC of being new."""
        self.assertEqual(self._errs({"rfc1", "rfc2"}, {}, {}, None), [])

    def test_a_row_deleted_for_a_newly_enrolled_stem_reports_once(self):
        """Both halves could fire on one stem. The deletion half is scoped to
        `enrolled & baseline_enrolled`, so a stem that was not enrolled at HEAD is judged by
        the new-enrolment half alone and the reader gets one actionable message."""
        errs = self._errs(
            {"rfc1"}, {}, {"rfc1": _row()}, {"rfc1"}, baseline_enrolled=set()
        )
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("newly enrolled", errs[0])

    def test_the_real_tree_has_thirty_two_grandfathered(self):
        """A-1 over the REAL tree: the grandfather set is the 32 the census measured. If it
        grew, a rowless enrolment slipped in; if it shrank, someone wrote rows."""
        missing = R.load_enrolled() - set(_status_rows())
        self.assertEqual(len(missing), 32, sorted(missing))


class TestStatusBaselineReader(unittest.TestCase):
    """A-7, AC-4 at the reader: git failing must not read as "no rows at HEAD"."""

    def test_status_baseline_survives_git_failure(self):
        with _patched(subprocess=_FakeSubprocess(returncode=1)):
            self.assertIsNone(R._git_baseline_status_rows())

    def test_status_baseline_is_none_when_the_blob_is_absent(self):
        with _patched(_git_cat_blobs=lambda paths: {}):
            self.assertIsNone(R._git_baseline_status_rows())

    def test_status_baseline_parses_the_blob(self):
        blob = "| RFC 1234 | Area | Supported | cov | rem |\n"
        with _patched(_git_cat_blobs=lambda paths: {R.STATUS_REL: blob}):
            rows = R._git_baseline_status_rows()
        self.assertEqual(set(rows), {"rfc1234"})

    def test_disposition_baseline_survives_a_malformed_blob(self):
        """A HEAD blob nobody can parse is history nobody can use. The empty set accuses
        nobody, which is the safe polarity for `baseline - current`."""
        with _patched(
            _git_cat_blobs=lambda paths: {R.NOT_ENROLLED_REL: "rfc1\tnope\tx\n"}
        ):
            self.assertEqual(R._git_baseline_dispositions(), set())

    def test_disposition_baseline_parses_the_blob(self):
        blob = "rfc1\tbacklog\tthe extraction is owed\n"
        with _patched(_git_cat_blobs=lambda paths: {R.NOT_ENROLLED_REL: blob}):
            self.assertEqual(R._git_baseline_dispositions(), {"rfc1"})


class TestStatusCompletenessWiring(_LedgerEdgeDrive):
    def test_run_check_fails_when_a_new_enrolment_has_no_row(self):
        code, out = self._drive(baseline_enrolled=(), rows={})
        self.assertEqual(code, 2, out)
        self.assertIn("newly enrolled but has no row", out)

    def test_run_check_fails_when_a_row_is_deleted_under_enrolment(self):
        code, out = self._drive(rows={}, baseline_rows={self.STEM: _row()})
        self.assertEqual(code, 2, out)
        self.assertIn("at HEAD and does not now", out)

    def test_run_check_clean_when_the_row_is_there(self):
        code, out = self._drive(
            baseline_enrolled=(), rows={self.STEM: _row()}, baseline_rows={}
        )
        self.assertEqual(code, 0, out)


class TestUnprovenSupport(unittest.TestCase):
    """check_unproven_support: a public claim may not rest on an empty checklist
    (AC-10, AC-11)."""

    def _errs(
        self, reqs, rows, stems=("rfc1",), dispositions=None, signed=None, derived=None
    ):
        return R.check_unproven_support(
            reqs, rows, set(stems), dispositions or {}, signed or {}, derived or {}
        )

    def test_support_claim_over_zero_gated_fails(self):
        """AC-10."""
        errs = self._errs([], {"rfc1": _row("Supported")})
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("empty checklist", errs[0])

    def test_experimental_is_a_support_claim(self):
        """AC-10 boundary: `Experimental` says the code exists, which is exactly what an
        empty checklist cannot support. It must not escape."""
        errs = self._errs([], {"rfc1": _row("Experimental")})
        self.assertEqual(len(errs), 1, errs)

    def test_unsupported_and_future_rows_are_not_claims(self):
        """AC-10 boundary, the other side: the two values that make no claim."""
        for status in sorted(R.NON_CLAIM_STATUSES):
            self.assertEqual(self._errs([], {"rfc1": _row(status)}), [], status)

    def test_a_blank_status_is_treated_as_a_claim(self):
        """The zero-value trap: `row["status"]` is PRESENT and empty, so an `ok`-style test
        passes. A blank cell on a page of support claims must fail closed
        (ai/rules/evidence.md)."""
        errs = self._errs([], {"rfc1": _row("   ")})
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("(blank)", errs[0])

    def test_a_stem_with_gated_requirements_passes(self):
        """AC-10 negative: the checklist is not empty, so there is something to contradict."""
        self.assertEqual(
            self._errs([_req("RFC1-1-1", rfc="rfc1")], {"rfc1": _row("Supported")}), []
        )

    def test_advisory_requirements_do_not_satisfy_the_claim(self):
        """One SHOULD row is the immunity D5 exposed elsewhere. It must not buy a pass here:
        an advisory row never gates, so nothing can contradict the claim."""
        errs = self._errs(
            [_req("RFC1-1-1", level="SHOULD", rfc="rfc1")], {"rfc1": _row("Supported")}
        )
        self.assertEqual(len(errs), 1, errs)

    def test_non_normative_disposition_permits_the_claim(self):
        """AC-11: the first evidenced escape."""
        self.assertEqual(
            self._errs(
                [],
                {"rfc1": _row("Supported")},
                dispositions={
                    "rfc1": _disp("non-normative", "the document imposes none")
                },
            ),
            [],
        )

    def test_a_backlog_disposition_does_not_permit_the_claim(self):
        """`backlog` is DEBT. It says the extraction is owed, which is not evidence that the
        document imposes nothing."""
        errs = self._errs(
            [], {"rfc1": _row("Supported")}, dispositions={"rfc1": _disp("backlog")}
        )
        self.assertEqual(len(errs), 1, errs)

    def test_an_evidenced_manual_walk_signoff_permits_the_claim(self):
        """Owner Ruling OR-A: rfc3765 enrols on an evidenced zero. FOUR committed facts -- a
        VALID sign-off (so every site and section was classified), register `manual-walk`, a
        recorded register-reason, and a DERIVED register that is not `rfc2119` (the source
        quotes no capitalised keyword, so the zero is a property of the document)."""
        art = _extraction(
            register="manual-walk", register_reason="Informational, no 2119"
        )
        self.assertEqual(
            self._errs(
                [],
                {"rfc1": _row("Supported")},
                signed={"rfc1": art},
                derived={"rfc1": "prose"},
            ),
            [],
        )

    def test_a_manual_walk_signoff_without_a_reason_does_not_permit_it(self):
        """The reason is the evidence. Without it the escape is an assertion."""
        art = _extraction(register="manual-walk", register_reason="")
        errs = self._errs(
            [],
            {"rfc1": _row("Supported")},
            signed={"rfc1": art},
            derived={"rfc1": "prose"},
        )
        self.assertEqual(len(errs), 1, errs)

    def test_a_manual_walk_signoff_over_an_rfc2119_source_does_not_permit_it(self):
        """J2. `manual-walk` is the WEAKEST grade and _evaluate_extraction refuses only a
        STRONGER claim than derived, so ANY stem may declare it. The escape therefore has to
        read the DERIVED register, not the declared one: a source whose own sentences quote
        capitalised MUST/SHALL plainly imposes obligations, and no register-reason can make
        zero a property of that document."""
        art = _extraction(
            register="manual-walk", register_reason="walked by hand, found nothing"
        )
        errs = self._errs(
            [],
            {"rfc1": _row("Supported")},
            signed={"rfc1": art},
            derived={"rfc1": "rfc2119"},
        )
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("rfc2119", errs[0])
        self.assertIn("capitalised", errs[0])

    def test_the_refused_escape_gets_its_own_message(self):
        """ai/rules/cli.md leg 3: a remediation must be TRUE. Telling an author who
        already wrote a manual-walk sign-off to "record a manual-walk sign-off" is a dead end,
        so the refused-escape state says what is wrong with the one on disk."""
        art = _extraction(
            register="manual-walk", register_reason="walked by hand, found nothing"
        )
        refused = self._errs(
            [],
            {"rfc1": _row("Supported")},
            signed={"rfc1": art},
            derived={"rfc1": "rfc2119"},
        )[0]
        general = self._errs([], {"rfc1": _row("Supported")})[0]
        self.assertNotEqual(refused, general)
        self.assertIn("rfc/extraction/rfc1.json", refused)
        self.assertNotIn("or a manual-walk extraction sign-off", refused)

    def test_a_manual_walk_signoff_with_no_derived_grade_does_not_permit_it(self):
        """Fail closed on the unknown (ai/rules/evidence.md). An absent grade means
        the source could not be re-derived, so nothing established that zero is a property of
        the text -- and a missing dict entry must not read as a passing one."""
        art = _extraction(
            register="manual-walk", register_reason="Informational, no 2119"
        )
        errs = self._errs([], {"rfc1": _row("Supported")}, signed={"rfc1": art})
        self.assertEqual(len(errs), 1, errs)

    def test_a_prose_signoff_does_not_permit_the_claim(self):
        """A stem that simply captured nothing cannot reach the escape: the register has to
        be the weakest grade, which is what says the mechanical inventory found nothing
        normative."""
        art = _extraction(register="prose", register_reason="pre-2119 prose")
        errs = self._errs([], {"rfc1": _row("Supported")}, signed={"rfc1": art})
        self.assertEqual(len(errs), 1, errs)

    def test_a_row_with_no_summary_is_outside_the_check(self):
        """19 rows on the real page name an RFC with no rfc/short/*.md. That is a different
        defect, and firing here would bury this one under it."""
        self.assertEqual(self._errs([], {"rfc404": _row("Supported")}, stems=()), [])

    def test_the_real_tree_is_clean(self):
        """The check ARMED, over the REAL tree (AC-10 on the committed page). Exactly one
        stem reaches it -- rfc3765, whose zero is honest -- and OR-A's evidenced form is what
        lets it through. A synthetic fixture would pass with the page still lying."""
        reqs = _all_requirements()
        rows = _status_rows()
        signed = R.signed_extractions(reqs)
        self.assertEqual(
            R.check_unproven_support(
                reqs,
                rows,
                R.summary_stems(),
                R.load_dispositions(),
                signed,
                R.derived_registers(signed, reqs),
            ),
            [],
        )

    def test_rfc3765_is_the_stem_that_needs_the_evidenced_escape(self):
        """The discriminating half of the test above: with the escape removed, rfc3765 is
        named. Without this, a bug that made the check judge NOTHING would pass as clean."""
        reqs = _all_requirements()
        rows = _status_rows()
        errs = R.check_unproven_support(
            reqs, rows, R.summary_stems(), R.load_dispositions(), {}, {}
        )
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("rfc3765", errs[0])

    def test_rfc3765_derives_prose_so_the_tightened_escape_still_reaches_it(self):
        """J2's calibration, over the REAL source. The tightening had to refuse a source that
        quotes capitalised keywords WITHOUT refusing the one shipped case OR-A settled, so the
        line is drawn at the derived register rather than at the site count: rfc3765 derives
        `prose` from one lowercase modal, and rfc/full/rfc3765.txt contains no capitalised RFC
         2119 keyword at all. Refusing `prose` too would have reddened honest work."""
        inv = R.derive_inventory("rfc3765", 0)
        self.assertIsNotNone(inv, "rfc3765 must have source text to derive from")
        self.assertEqual(inv.register, "prose")
        self.assertEqual(inv.keyword_sites, 0)
        self.assertEqual(len(inv.sites), 1)
        self.assertIsNone(R._SITE_KEYWORD_RE.search(_read_repo("rfc/full/rfc3765.txt")))


class TestUnprovenSupportWiring(_LedgerEdgeDrive):
    def test_run_check_fails_on_support_claim_over_zero_gated_requirements(self):
        code, out = self._drive(
            reqs=[_req("RFC9999-1-1", level="MAY", rfc=self.STEM)],
            rows={self.STEM: _row("Supported")},
            tags=[],
        )
        self.assertEqual(code, 2, out)
        self.assertIn("empty checklist", out)

    def test_run_check_clean_when_the_claim_has_a_gated_requirement(self):
        code, out = self._drive(rows={self.STEM: _row("Supported")})
        self.assertEqual(code, 0, out)

    # A stem of this class's own, NOT _LedgerEdgeDrive.STEM. derive_inventory is memoised on
    # (stem, gated, source sha, path) and the memo outlives every test in the process, so two
    # classes deriving _SRC_TWO_SITES under one stem at gated=0 share an answer -- and
    # TestSiteInventory patches _section_bodies to force a duplicate locator under exactly that
    # key, which a pre-filled memo would serve around.
    WALK_STEM = "rfc8888"

    def _drive_walk(self, src, **kw):
        with _extraction_tree(src={self.WALK_STEM: src}):
            return self._drive(
                reqs=[],
                enrolled=(self.WALK_STEM,),
                rows={self.WALK_STEM: _row("Supported")},
                signed={self.WALK_STEM: _manual_walk_extraction(self.WALK_STEM, src)},
                tags=[],
                **kw,
            )

    def test_run_check_refuses_the_manual_walk_escape_over_an_rfc2119_source(self):
        """J2 driven through run_check with the REAL derivation, not a passed-in grade: the
        source is _SRC_TWO_SITES (two capitalised MUST sentences), the summary declares zero
        gated requirements, the page claims Supported, and the sign-off is a VALID manual-walk
        artifact that excludes both sites as not-a-requirement. That is the reproduced evasion,
        end to end, and run_check must refuse it."""
        code, out = self._drive_walk(_SRC_TWO_SITES)
        self.assertEqual(code, 2, out)
        self.assertIn("rfc2119", out)

    def test_run_check_permits_the_escape_over_a_keyword_free_source(self):
        """The discriminating twin: the same artifact over a source with no capitalised keyword
        derives `prose`, so OR-A's escape still works through run_check. Without this pair, a
        tightening that refused EVERY manual-walk escape would look correct."""
        code, out = self._drive_walk(_SRC_PROSE_ONLY)
        self.assertEqual(code, 0, out)


class TestDerivedRegisters(unittest.TestCase):
    """derived_registers: the fourth committed fact behind OR-A's escape.

    The grade is DERIVED from the source at check time and keyed by stem, so the escape reads
    what the document is written in rather than what the artifact declares about itself
    (ai/rules/evidence.md).

    STEM is this class's own, for the reason TestUnprovenSupportWiring.WALK_STEM records: the
    derive_inventory memo is process-wide, so a class that derives a shared fixture under a
    shared stem hands its answer to whoever asks next."""

    STEM = "rfc8888"

    def test_the_grade_comes_from_the_source_not_the_artifact(self):
        """The whole point: the artifact says `manual-walk`, the source says `rfc2119`."""
        signed = {self.STEM: _extraction(stem=self.STEM, register="manual-walk")}
        with _extraction_tree(src={self.STEM: _SRC_TWO_SITES}):
            got = R.derived_registers(signed, [])
        self.assertEqual(got, {self.STEM: "rfc2119"})

    def test_a_prose_source_grades_prose(self):
        signed = {self.STEM: _extraction(stem=self.STEM, register="manual-walk")}
        with _extraction_tree(src={self.STEM: _SRC_PROSE_ONLY}):
            self.assertEqual(R.derived_registers(signed, []), {self.STEM: "prose"})

    def test_a_source_neither_register_can_see_grades_manual_walk(self):
        signed = {"rfc9999": _extraction(stem="rfc9999", register="manual-walk")}
        with _extraction_tree(src={"rfc9999": _SRC_NO_INVENTORY}):
            self.assertEqual(
                R.derived_registers(signed, []), {"rfc9999": "manual-walk"}
            )

    def test_a_stem_with_no_source_text_is_ABSENT_rather_than_defaulted(self):
        """None is not a register. A stem whose source cannot be read must leave no entry, so
        the consumer sees "I could not look" instead of a grade nobody derived
        (ai/rules/evidence.md, the zero-value trap)."""
        signed = {self.STEM: _extraction(stem=self.STEM, register="manual-walk")}
        with _extraction_tree(src={}):
            self.assertEqual(R.derived_registers(signed, []), {})

    def test_the_gated_count_reaches_the_derivation(self):
        """derive_register reads the declared gated count: two capitalised sites against 23
        declared rows is an UNDERCOUNT, which grades `prose`, not `rfc2119`. Dropping the
        requirements argument would silently grade every stem at gated=0."""
        signed = {self.STEM: _extraction(stem=self.STEM, register="manual-walk")}
        declared = [_req(f"RFC8888-2-{i}", rfc=self.STEM) for i in range(1, 24)]
        with _extraction_tree(src={self.STEM: _SRC_TWO_SITES}):
            self.assertEqual(
                R.derived_registers(signed, declared), {self.STEM: "prose"}
            )

    def test_unsigned_stems_are_not_derived(self):
        """Scoped to the signed set: deriving the whole corpus here would pay for 166 walks no
        consumer reads."""
        with _extraction_tree(src={self.STEM: _SRC_TWO_SITES}):
            self.assertEqual(R.derived_registers({}, []), {})

    def test_the_real_tree_grades_every_signed_stem(self):
        """Over the REAL tree: every VALID sign-off has derivable source (evaluate_extractions
        refuses one that does not), so no signed stem may be missing a grade."""
        reqs = _all_requirements()
        signed = R.signed_extractions(reqs)
        self.assertTrue(signed)
        self.assertEqual(set(R.derived_registers(signed, reqs)), set(signed))


class TestGapCountAgreement(unittest.TestCase):
    """check_gap_count_agreement: a spelled MUST-gap count must equal the real count
    (AC-12)."""

    def _errs(self, reqs, remaining):
        return R.check_gap_count_agreement(reqs, {"rfc1": _row(remaining=remaining)})

    def _one_gap(self, n):
        return [
            _req(f"RFC1-1-{i}", annotation=_gap(), rfc="rfc1") for i in range(1, n + 1)
        ]

    def test_matching_spelled_count_passes(self):
        self.assertEqual(
            self._errs(self._one_gap(3), "Three MUSTs remain unproven."), []
        )

    def test_mismatched_spelled_count_fails(self):
        errs = self._errs(self._one_gap(2), "Three MUSTs remain unproven.")
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("3 MUST-level gap(s)", errs[0])
        self.assertIn("2 {gap} annotation(s)", errs[0])

    def test_compound_number_is_not_read_as_its_tail(self):
        """`Twenty-five` is 25, never 5. A units-first alternation matches the tail of every
        compound and would red four honest rows on the committed page."""
        self.assertEqual(R.spelled_gap_count("Twenty-five MUSTs remain"), 25)
        self.assertEqual(self._errs(self._one_gap(25), "Twenty-five MUSTs remain"), [])
        errs = self._errs(self._one_gap(5), "Twenty-five MUSTs remain")
        self.assertEqual(len(errs), 1, errs)

    def test_row_without_a_spelled_count_is_not_judged(self):
        """AC-12 boundary: no numeric claim is not a claim of zero."""
        self.assertIsNone(
            R.spelled_gap_count("No tracked gap in current source anchors.")
        )
        self.assertEqual(self._errs(self._one_gap(4), "Some gaps remain."), [])

    def test_zero_is_not_a_gap_claim(self):
        """`Zero` is absent from SPELLED_NUMBERS on purpose: it makes no claim about a gap,
        and reading it as 0 would invent a disagreement with any real count."""
        self.assertIsNone(R.spelled_gap_count("Zero MUSTs remain"))

    def test_forty_is_not_read_as_four(self):
        """Boundary: an unrecognised word must not silently match a shorter one."""
        self.assertEqual(R.spelled_gap_count("Forty MUSTs remain"), 40)
        self.assertIsNone(R.spelled_gap_count("Hundreds of MUSTs remain"))

    def test_the_range_boundaries_parse(self):
        for word, want in (
            ("One", 1),
            ("Nineteen", 19),
            ("Twenty", 20),
            ("Thirty-nine", 39),
            ("Sixty-four", 64),
        ):
            self.assertEqual(R.spelled_gap_count(f"{word} MUSTs remain"), want, word)

    def test_only_immediate_adjacency_counts(self):
        """The load-bearing decision: the page uses a SECOND convention where a spelled
        number NEAR the word MUST is the {not-applicable} count. A tolerance window would
        read those as gap counts and red four honest rows."""
        self.assertIsNone(
            R.spelled_gap_count(
                "Sixty-four further MUSTs bind PE roles ze does not fill"
            )
        )
        self.assertIsNone(R.spelled_gap_count("Nine further MUSTs are not-applicable"))

    def test_shall_is_read_like_must(self):
        self.assertEqual(R.spelled_gap_count("Two SHALLs remain"), 2)

    def test_a_stem_with_no_gaps_and_a_spelled_count_fails(self):
        """The commonest real drift: the gaps were closed and the row was not updated."""
        errs = self._errs([], "Two MUSTs remain unproven.")
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("0 {gap} annotation(s)", errs[0])


class TestGapCountAgreementRealFile(unittest.TestCase):
    """A-4 over the committed docs/features/rfc-status.md, not a fixture. What this proves
    is a fact about THIS repository, which a synthetic row cannot."""

    def test_committed_page_agrees(self):
        reqs = []
        for stem in sorted(R.summary_stems()):
            reqs.extend(R.parse_summary_file(os.path.join(R.SUMMARY_DIR, stem + ".md")))
        rows = _status_rows()
        self.assertEqual(R.check_gap_count_agreement(reqs, rows), [])

    def test_the_page_really_does_spell_the_judged_counts(self):
        """The discriminating half: a parser that matched NOTHING would also report zero
        violations above, so the count of rows it judges is asserted too.

        57, not the 60 measured on 2026-07-30. rfc7296, rfc3101 and rfc7911 left
        the judged population when their last MUST gap closed. Their rows now
        spell no number, because the check reads One..Ninety-nine and has no word
        for zero. A stem that drops out this way is the intended direction of
        travel."""
        rows = _status_rows()
        judged = [
            stem
            for stem, row in rows.items()
            if R.spelled_gap_count(row["remaining"]) is not None
        ]
        self.assertEqual(len(judged), 57, sorted(judged))
        self.assertNotIn("rfc7296", judged)
        self.assertNotIn("rfc3101", judged)
        self.assertNotIn("rfc7911", judged)

    def test_the_unjudged_rows_are_the_seven_the_docstring_names(self):
        """The coverage limit check_gap_count_agreement states, MEASURED rather than remembered.
        Its note used to say "two rows spell their count in DIGITS and one spells `Sixty-four`,
        and none of the three is judged" -- wrong on every clause. There are four digit rows and
        three separated ones, `Sixty-four` parses to 64, and rfc7432 is judged on its first
        match. A prose count nothing measures is the shape that went stale
        (ai/rules/stale-comments.md), so this pins it."""
        rows = _status_rows()
        digit_re = re.compile(r"\b(\d+)\s+(?:MUST|SHALL)s?\b")
        spelled_re = re.compile(r"\b(?:" + R._SPELLED_ALT + r")\b", re.IGNORECASE)
        digit, separated = [], []
        for stem, row in rows.items():
            remaining = row["remaining"]
            if R.spelled_gap_count(remaining) is not None:
                continue
            near = any(
                re.search(r"\b(?:MUST|SHALL)s?\b", remaining[m.end() : m.end() + 50])
                for m in spelled_re.finditer(remaining)
            )
            if digit_re.search(remaining):
                digit.append(stem)
            elif near:
                separated.append(stem)
        self.assertEqual(
            sorted(digit), ["rfc7166", "rfc7311", "rfc9012", "rfc9830"], sorted(digit)
        )
        self.assertEqual(
            sorted(separated), ["rfc5575", "rfc9085", "rfc9514"], sorted(separated)
        )

    def test_sixty_four_parses_and_its_row_is_judged_on_another_match(self):
        """The clause the old note got exactly backwards. `Sixty-four` is a number this parser
        reads; adjacency is what skips it, and rfc7432's count is checked all the same."""
        self.assertEqual(R.SPELLED_NUMBERS["sixty-four"], 64)
        self.assertEqual(R.spelled_gap_count("Sixty-four MUSTs"), 64)
        self.assertIsNone(R.spelled_gap_count("Sixty-four further MUSTs"))
        remaining = _status_rows()["rfc7432"]["remaining"]
        self.assertIn("Sixty-four further MUSTs", remaining)
        self.assertEqual(R.spelled_gap_count(remaining), 15)


class TestGapCountWiring(_LedgerEdgeDrive):
    def test_run_check_fails_when_a_spelled_count_disagrees(self):
        code, out = self._drive(
            reqs=[_req("RFC9999-1-1", annotation=_gap(), rfc=self.STEM)],
            rows={self.STEM: _row(remaining="Three MUSTs remain unproven.")},
            tags=[],
        )
        self.assertEqual(code, 2, out)
        self.assertIn("3 MUST-level gap(s)", out)

    def test_run_check_clean_when_the_count_agrees(self):
        code, out = self._drive(
            reqs=[_req("RFC9999-1-1", annotation=_gap(), rfc=self.STEM)],
            rows={self.STEM: _row(remaining="One MUST remains unproven.")},
            tags=[],
        )
        self.assertEqual(code, 0, out)


class TestGapDisclosureScope(unittest.TestCase):
    """check_status_agreement's exemption narrowed from "not enrolled" to "no row"
    (AC-15, AC-16)."""

    def _errs(self, enrolled, rows):
        reqs = [_req("RFC1-1-1", annotation=_gap("unimplemented"), rfc="rfc1")]
        return R.check_status_agreement(reqs, rows, set(enrolled))

    def test_unenrolled_gap_with_a_row_must_disclose(self):
        """AC-15: the hole. A private {gap} contradicting a public clean 'Supported' row was
        exempt purely because the stem was not enrolled."""
        errs = self._errs(set(), {"rfc1": _row("Supported", remaining="")})
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("cannot be advertised as clean support", errs[0])

    def test_unenrolled_gap_with_a_disclosing_row_passes(self):
        self.assertEqual(
            self._errs(set(), {"rfc1": _row("Partial", remaining="One MUST unproven")}),
            [],
        )

    def test_unenrolled_gap_without_a_row_is_clean(self):
        """AC-16: an un-rowed, un-enrolled RFC makes no public claim to contradict, and
        demanding a row would force rows for reference-only summaries."""
        self.assertEqual(self._errs(set(), {}), [])

    def test_enrolled_gap_without_a_row_still_fails(self):
        """Regression: the missing-row branch is unchanged for an enrolled stem."""
        errs = self._errs({"rfc1"}, {})
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("has no row", errs[0])

    def test_enrolled_behaviour_unchanged(self):
        """Regression on the disclosure branch itself."""
        errs = self._errs({"rfc1"}, {"rfc1": _row("Supported", remaining="")})
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("cannot be advertised as clean support", errs[0])

    def test_the_real_tree_is_clean_under_the_narrowed_scope(self):
        """A-2 over the REAL tree: all 539 {gap} annotations belong to rowed, enrolled RFCs,
        so narrowing the exemption is a no-op today. If it were not, this reds on landing."""
        reqs = []
        for stem in sorted(R.summary_stems()):
            reqs.extend(R.parse_summary_file(os.path.join(R.SUMMARY_DIR, stem + ".md")))
        rows = _status_rows()
        self.assertEqual(R.check_status_agreement(reqs, rows, R.load_enrolled()), [])


class TestGapDisclosureWiring(_LedgerEdgeDrive):
    """The narrowed exemption, driven through run_check. This driver does NOT patch out
    check_status_agreement -- it is the subject."""

    def _drive_disclosure(self, enrolled, rows):
        return self._drive(
            reqs=[_req("RFC9999-1-1", annotation=_gap("unimplemented"), rfc=self.STEM)],
            enrolled=enrolled,
            stems={self.STEM},
            rows=rows,
            dispositions={} if enrolled else {self.STEM: _disp()},
            tags=[],
            check_status_agreement=R.check_status_agreement,
            check_unproven_support=lambda *a, **k: [],
            check_gap_count_agreement=lambda *a, **k: [],
        )

    def test_run_check_fails_on_unenrolled_gap_with_a_rowed_claim(self):
        code, out = self._drive_disclosure((), {self.STEM: _row("Supported")})
        self.assertEqual(code, 2, out)
        self.assertIn("cannot be advertised as clean support", out)

    def test_run_check_clean_on_unenrolled_gap_with_no_row(self):
        code, out = self._drive_disclosure((), {})
        self.assertEqual(code, 0, out)


class TestParseErrorReporting(unittest.TestCase):
    """AC-17: every parse error is reported, enrolled or not."""

    def test_unenrolled_parse_error_is_reported(self):
        stems = {"rfc1", "rfc2"}

        def parse(path):
            if path.endswith("rfc2.md"):
                raise R.ParseError("rfc/short/rfc2.md:9: bad requirement id")
            return [_req("RFC1-1-1", rfc="rfc1")]

        with _patched(
            load_enrolled=lambda: {"rfc1"},
            summary_stems=lambda: stems,
            parse_summary_file=parse,
            scan_tree=lambda *a, **k: [],
        ):
            _, reqs, errs, _, by_stem = R._collect_for_check()
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("rfc/short/rfc2.md:9", errs[0])
        self.assertEqual(set(by_stem), {"rfc2"})

    def test_the_real_tree_parses(self):
        """A-5: zero of the summaries fail to parse, which is what makes dropping the
        enrolment filter a no-op today rather than a wall of new violations."""
        for stem in sorted(R.summary_stems()):
            R.parse_summary_file(os.path.join(R.SUMMARY_DIR, stem + ".md"))


class TestParseErrorReportingWiring(_LedgerEdgeDrive):
    def test_run_check_reports_an_unenrolled_parse_error(self):
        def parse(path):
            if path.endswith("rfc7777.md"):
                raise R.ParseError("rfc/short/rfc7777.md:9: bad requirement id")
            return [_req("RFC9999-1-1", rfc=self.STEM)]

        code, out = self._drive(
            stems={self.STEM, "rfc7777"},
            dispositions={"rfc7777": _disp()},
            parse_summary_file=parse,
        )
        self.assertEqual(code, 2, out)
        self.assertIn("rfc/short/rfc7777.md:9", out)


class TestLedgerBacklogTables(unittest.TestCase):
    """AC-18: the two grandfathered backlogs are RENDERED, not listed."""

    def _body(self, enrolled=("rfc7606",), rows=None, dispositions=None):
        return R.render_index(
            [_req("RFC7606-2-1")],
            [_tag("RFC7606-2-1", "positive"), _tag("RFC7606-2-1", "negative")],
            set(enrolled),
            {} if rows is None else rows,
            {} if dispositions is None else dispositions,
        )

    def test_enrolled_without_row_table_rendered(self):
        body = self._body(enrolled=("rfc7606", "rfc1234"))
        self.assertIn("## Enrolled without a public status row", body)
        self.assertIn("`rfc1234`", body)
        self.assertIn("2 enrolled RFC(s) have no row", body)

    def test_the_table_is_empty_when_every_row_exists(self):
        body = self._body(rows={"rfc7606": _row()})
        self.assertIn("None: every enrolled RFC has a row.", body)

    def test_disposition_table_rendered(self):
        body = self._body(dispositions={"rfc1234": _disp("backlog", "extraction owed")})
        self.assertIn("## Declared not enrolled", body)
        self.assertIn("`rfc1234`", body)
        self.assertIn("extraction owed", body)

    def test_debt_kinds_are_marked_as_debt(self):
        """R-4: `backlog` and `blocked` must never read as a settled decision."""
        body = self._body(
            dispositions={
                "rfc1": _disp("backlog", "a"),
                "rfc2": _disp("blocked", "b"),
                "rfc3": _disp("non-normative", "c"),
            }
        )
        section = body.split("## Declared not enrolled", 1)[1].split("\n## ", 1)[0]
        rows = [ln for ln in section.split("\n") if ln.startswith("| `rfc")]
        debt = {ln.split("|")[1].strip(): "**DEBT**" in ln for ln in rows}
        self.assertEqual(debt, {"`rfc1`": True, "`rfc2`": True, "`rfc3`": False})

    def test_render_is_deterministic(self):
        """A-8: check_ledger_fresh compares bytes, so an unstable render turns the freshness
        gate into noise."""
        args = dict(
            enrolled=("rfc7606", "rfc1234", "rfc5678"),
            dispositions={"rfc9": _disp(), "rfc8": _disp("blocked", "no source")},
        )
        self.assertEqual(self._body(**args), self._body(**args))

    def test_the_render_reads_the_real_files_when_not_given_them(self):
        """run_write and run_check_fresh call render_index with three arguments. The bytes
        must be the same as run_check's five-argument call, or the freshness gate compares a
        ledger to a different render of the same tree."""
        reqs = [_req("RFC7606-2-1")]
        tags = [_tag("RFC7606-2-1", "positive")]
        rows = _status_rows()
        self.assertEqual(
            R.render_index(reqs, tags, {"rfc7606"}),
            R.render_index(reqs, tags, {"rfc7606"}, rows, R.load_dispositions()),
        )


class TestUnconvertedSummaries(unittest.TestCase):
    """AC-13, D5: one advisory row used to buy immunity from the RE-AUTHOR table."""

    def test_advisory_only_summary_is_listed(self):
        """D5: `captured` meant "captured anything", the check meant "captured the
        obligations", and seven advisory-only summaries hid in the gap."""
        body = R.render_index(
            [_req("RFC7606-2-1", level="SHOULD")],
            [],
            {"rfc7606"},
            {},
            {},
        )
        self.assertIn("## Summaries declaring no MUST-level requirement", body)
        self.assertIn("`rfc7606`", body)

    def test_a_gated_summary_is_not_listed(self):
        body = R.render_index([_req("RFC7606-2-1")], [], {"rfc7606"}, {}, {})
        rows = [ln for ln in body.split("\n") if ln.startswith("| `rfc7606` |")]
        self.assertFalse(
            any("RE-AUTHOR" in ln or "UNDECIDED" in ln for ln in rows), rows
        )

    def test_zero_source_count_verdict_names_pre_2119_doubt(self):
        """AC-13, R-7: 0 uppercase with a non-zero lowercase count is the pre-RFC-2119
        signature. "consistent: source declares none" read RFC 1035, a normative wire
        specification, as non-normative."""
        with _patched(
            summary_stems=lambda: {"rfcX"},
            source_keyword_count=lambda stem: 0,
            source_prose_keyword_count=lambda stem: 23,
        ):
            rows = R.unconverted_summaries(set())
            body = R.render_index([], [], set(), {}, {})
        self.assertEqual(rows[0]["prose"], 23)
        self.assertIn("UNDECIDED", body)
        self.assertIn("pre-RFC-2119", body)
        self.assertNotIn("consistent: source declares none", body)

    def test_zero_in_both_registers_is_consistent(self):
        """The discriminating twin: a genuinely non-normative source shows 0 and 0, and it
        must still read as consistent rather than as undecided."""
        with _patched(
            summary_stems=lambda: {"rfcX"},
            source_keyword_count=lambda stem: 0,
            source_prose_keyword_count=lambda stem: 0,
        ):
            body = R.render_index([], [], set(), {}, {})
        self.assertIn("consistent: source declares none", body)
        self.assertNotIn("UNDECIDED", body)

    def test_a_normative_source_still_says_re_author(self):
        with _patched(
            summary_stems=lambda: {"rfcX"},
            source_keyword_count=lambda stem: 7,
            source_prose_keyword_count=lambda stem: 9,
        ):
            body = R.render_index([], [], set(), {}, {})
        self.assertIn("RE-AUTHOR", body)

    def test_the_lowercase_counter_reads_rfc1035_as_normative(self):
        """The worked example, over the REAL source text: 0 uppercase, many lowercase."""
        self.assertEqual(R.source_keyword_count("rfc1035"), 0)
        self.assertGreater(R.source_prose_keyword_count("rfc1035"), 20)

    def test_the_real_table_lists_every_advisory_only_summary(self):
        """Over the REAL tree: the table's row count is the number of summaries that gate
        nothing. With `captured` still meaning "captured anything", the advisory-only stems
        were absent from it."""
        reqs = []
        for stem in sorted(R.summary_stems()):
            reqs.extend(R.parse_summary_file(os.path.join(R.SUMMARY_DIR, stem + ".md")))
        listed = {
            row["stem"]
            for row in R.unconverted_summaries({r.rfc for r in reqs if r.gated})
        }
        gated_stems = {r.rfc for r in reqs if r.gated}
        self.assertEqual(listed, R.summary_stems() - gated_stems)
        self.assertIn("rfc3765", listed)


class TestFourStemEnrolmentRealTree(unittest.TestCase):
    """Owner Ruling OR-1 and OR-A, asserted over the REAL rfc/ tree.

    Deliberately not patched. What these prove is a fact about THIS repository -- the D1
    stems are extracted, sourced and signed off -- and a synthetic fixture would pass with
    all four still broken, which is the whole failure OR-1 exists to end.
    """

    FOUR = ("rfc1035", "rfc3765", "rfc4486", "rfc5301")

    @classmethod
    def setUpClass(cls):
        cls.reqs = {}
        for stem in cls.FOUR:
            cls.reqs[stem] = R.parse_summary_file(
                os.path.join(R.SUMMARY_DIR, stem + ".md")
            )

    def test_all_four_are_sourced(self):
        """AC-20: check_enrolment refuses an enrolment with no source text, so the fetch is a
        precondition of enrolling rfc3765 and rfc4486 at all."""
        for stem in self.FOUR:
            self.assertIsNotNone(R.source_path(stem), stem)

    def test_all_four_carry_an_extraction_signoff(self):
        """AC-27: four walks, four artifacts, each re-derived against the source text and
        found to have zero violations."""
        signed = R.signed_extractions([r for reqs in self.reqs.values() for r in reqs])
        for stem in self.FOUR:
            self.assertIn(stem, signed, f"{stem} has no VALID sign-off")

    def test_no_x_anchor_and_no_bare_bracket_tag_remains(self):
        """AC-21: the `-x-` anchor is the skill's deliberate defect marker, and a bare
        category tag is an obligation the parser reads as prose."""
        for stem in self.FOUR:
            for req in self.reqs[stem]:
                self.assertNotEqual(req.section, R.NO_SECTION, f"{stem} {req.rid}")

    def test_no_checkbox_is_ticked(self):
        """AC-21: the box is a template marker, never coverage state."""
        for stem in self.FOUR:
            for req in self.reqs[stem]:
                self.assertFalse(req.ticked, f"{stem} {req.rid}")

    def test_three_declare_a_gated_requirement_and_rfc3765_declares_none(self):
        """AC-21 as corrected by OR-A. rfc1035, rfc5301 and rfc4486 carry real MUST-level
        obligations; rfc3765 declares zero, and that zero is a property of an Informational
        document with no RFC 2119 section and no keyword occurrence anywhere."""
        gated = {stem: sum(1 for r in self.reqs[stem] if r.gated) for stem in self.FOUR}
        self.assertEqual(gated["rfc3765"], 0, "OR-A: zero is the honest answer here")
        for stem in ("rfc1035", "rfc4486", "rfc5301"):
            self.assertGreater(gated[stem], 0, stem)

    def test_rfc3765_signs_under_manual_walk_with_a_reason(self):
        """OR-A's evidenced form, which check_unproven_support accepts and nothing else can
        reach: register `manual-walk` (weaker than the derived `prose`, which child 1
        permits) plus a recorded register-reason."""
        art = R.parse_extraction_artifact(
            os.path.join(R.EXTRACTION_DIR, "rfc3765.json")
        )
        self.assertEqual(art.register, "manual-walk")
        self.assertTrue(art.register_reason)
        self.assertIn("Informational", art.register_reason)

    def test_rfc4486_is_enrolled_and_its_must_is_proven_both_ways(self):
        """The one gated requirement of the four that is fully proven today."""
        self.assertIn("rfc4486", R.load_enrolled())
        tags = [t for t in R.scan_tree() if t.rid == "RFC4486-4-1"]
        self.assertEqual(
            {t.polarity for t in tags}, {"positive", "negative"}, [t.file for t in tags]
        )

    def test_the_prefix_maximum_ci_is_credited_as_functional_evidence(self):
        """J3. docs/features/rfc-status.md cites test/plugin/prefix-maximum-enforce.ci as proof
        that RFC4486-4-1 puts subcode 1 on the wire, and that .ci does pin the exact bytes --
        but it carried no `RFC requirement:` tag, so ai/RFC-REQUIREMENTS.md credited the
        requirement with unit evidence only and the run line reported functional/verify 6.

        Child 2's whole thesis is that unit evidence proves the ALGORITHM while only a running
        non-unit test proves the DAEMON, so a prose claim of functional proof the gate does not
        credit is the claim and the evidence disagreeing (ai/rules/evidence.md)."""
        rel = "test/plugin/prefix-maximum-enforce.ci"
        tags = [t for t in R.scan_tree() if t.rid == "RFC4486-4-1"]
        found = [t for t in tags if t.file == rel]
        self.assertEqual(len(found), 1, [t.file for t in tags])
        self.assertEqual(found[0].polarity, "positive")
        self.assertEqual(R.evidence_label(rel), "functional/verify")
        self.assertFalse(R.is_nightly_only(tags))

    def test_the_page_still_cites_that_ci_as_the_wire_level_proof(self):
        """The other half of the pair: the tag exists to make a PROSE claim machine-checked, so
        the prose it answers to is asserted here. If the citation is ever dropped, the tag is
        answering nothing and this says so."""
        row = _status_rows()["rfc4486"]
        self.assertIn("test/plugin/prefix-maximum-enforce.ci", row["coverage"])
        self.assertIn("asserts the bytes on the wire", row["coverage"])

    def test_the_ci_still_pins_the_cease_subcode_bytes(self):
        """What the tag now claims, read off the .ci itself: error code 06, subcode 01, and the
        Figure 1 Data field (AFI 0001, SAFI 01, upper bound 00000003). A tag over a test that
        stopped asserting the subcode would be evidence of nothing."""
        body = _read_repo("test/plugin/prefix-maximum-enforce.ci")
        self.assertIn(
            "hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001C03060100010100000003", body
        )

    def test_rfc3765_is_enrolled(self):
        """OR-A: enrolled on the evidenced zero, not on a fabricated MUST."""
        self.assertIn("rfc3765", R.load_enrolled())

    def test_the_declared_remainder_is_debt_not_a_decision(self):
        """OC-4: a stem whose obligations are escalated is declared `backlog`, which renders as
        DEBT and excuses nothing. A stem is in exactly one of the two files, never neither and
        never both.

        rfc1035 is still declared. rfc5301 was declared beside it until 2026-08-10, when
        spec-fixit-isis-hostname-ascii met its three unmet obligations and ENROLLED it, so it
        is asserted on the other side of the partition now. Both halves are checked, because
        the failure this guards against is a stem falling out of both files."""
        dispositions = R.load_dispositions()
        enrolled = R.load_enrolled()
        self.assertIn("rfc1035", dispositions)
        self.assertEqual(dispositions["rfc1035"].kind, "backlog")
        self.assertNotIn("rfc1035", enrolled)
        self.assertIn("rfc5301", enrolled)
        self.assertNotIn("rfc5301", dispositions)

    def test_no_id_a_test_references_was_lost(self):
        """A-11, R-9. Ids WERE replaced here, deliberately: all six `-x-` anchors in rfc3765
        and rfc4486 are the skill's defect marker, and OR-1 re-anchored them to real
        sections. That is legal only while no test tags them, so the invariant to assert is
        not "no id changed" -- it is that no id a test REFERENCES went away, which is the
        failure a re-anchor could actually cause (a tag silently re-pointed at a different
        obligation).

        After enrolment check_retired_requirements freezes rfc3765's and rfc4486's ids for
        good, so this window is now shut for those two."""
        current = {r.rid for reqs in self.reqs.values() for r in reqs}
        tagged = {t.rid for t in R.scan_tree()}
        for stem in self.FOUR:
            prefix = stem.upper() + "-"
            lost = {
                rid
                for rid in R._git_baseline_ids()
                if rid.startswith(prefix) and rid not in current
            }
            self.assertEqual(
                lost & tagged,
                set(),
                f"{stem}: a tagged id was dropped: {sorted(lost & tagged)}",
            )

    # The six `-x-` no-section anchors rfc3765 and rfc4486 carried BEFORE OR-1 re-anchored
    # them (`git show e558c55b2^:rfc/short/rfc3765.md`, same for rfc4486). Pinned rather
    # than read from _git_baseline_ids(), because what OR-1 replaced is a fact about a
    # specific past commit and cannot change: once the re-anchored summaries were
    # committed, HEAD and the tree agreed and a HEAD-vs-tree diff could no longer see the
    # replacement at all.
    _PRE_OR1_X_ANCHORS = (
        "RFC3765-x-1",
        "RFC3765-x-2",
        "RFC3765-x-3",
        "RFC3765-x-4",
        "RFC4486-x-1",
        "RFC4486-x-2",
    )

    def test_the_x_anchored_ids_were_the_ones_replaced(self):
        """The discriminating half: the test above would also pass if NOTHING had changed.

        Asserted against the PINNED pre-OR-1 anchors, not against `_git_baseline_ids()`.
        The original form compared HEAD to the working tree, which held only while the
        re-anchored summaries were uncommitted: commit e558c55b2 committed them, `gone`
        became empty, and `assertTrue(gone)` failed on work that was correct and complete.
        Re-tuning it would buy one commit. The durable statement is the one below -- none
        of the six defect markers survives in the tree, whoever committed what -- and it
        still reds if OR-1 is reverted or an `-x-` anchor is reintroduced.

        The loop over `gone` is kept verbatim as a live guard on the live baseline: a
        re-anchor may only ever retire a defect marker, so any id of these two stems that
        disappears relative to HEAD must be an `-x-` anchor. It is satisfied by an empty
        `gone` today, which is the honest reading -- nothing was dropped, so nothing needs
        justifying -- and it fires the moment a real id is dropped."""
        current = {r.rid for reqs in self.reqs.values() for r in reqs}
        for rid in self._PRE_OR1_X_ANCHORS:
            self.assertIn("-x-", rid, f"{rid} is not an -x- anchor; the pin is wrong")
            self.assertNotIn(rid, current, f"{rid} survived the OR-1 re-anchor")
        gone = {
            rid
            for rid in R._git_baseline_ids()
            if rid.startswith("RFC3765-") or rid.startswith("RFC4486-")
        } - current
        for rid in gone:
            self.assertIn("-x-", rid, f"{rid} was dropped but was not an -x- anchor")

    def test_no_requirement_id_is_reused(self):
        """A-12: check_id_allocation is what makes an id a permanent contract. Reuse -- drop
        an id and add a different requirement under the same id -- is textually invisible and
        is the one thing re-authoring must never do."""
        reqs = []
        for stem in sorted(R.summary_stems()):
            reqs.extend(R.parse_summary_file(os.path.join(R.SUMMARY_DIR, stem + ".md")))
        self.assertEqual(R.check_id_allocation(reqs, R._git_baseline_ids()), [])


# --------------------------------------------------------------------------
# Evidence admissibility: the package a tag lives in has to compile
# --------------------------------------------------------------------------
class _FakeVet:
    """Stands in for `subprocess` inside rfc_requirements, answering the vet call only.

    A failing run returns the REAL text `go vet` produced for the defect that prompted this
    check, never empty output. An empty-stderr fake cannot tell a reader that parses the
    output from one that only reads `returncode`: both report something, and the test then
    passes on an implementation that names no package and quotes no compiler message.

    Every other call is a git baseline reader. Those get a clean empty result, which is
    exactly what an unavailable baseline looks like and is already handled everywhere.
    """

    TimeoutExpired = subprocess.TimeoutExpired

    REAL_FAILURE = (
        "# github.com/ze-software/ze/internal/component/ike/engine\n"
        "# [github.com/ze-software/ze/internal/component/ike/engine]\n"
        "vet: internal/component/ike/engine/rfc7296_msgid_test.go:420:39: "
        "undefined: log2tr\n"
    )

    def __init__(self, returncode=0, stderr="", raises=None):
        self._rc = returncode
        self._err = stderr
        self._raises = raises
        self.argv = None

    def run(self, argv, **kwargs):
        outer = self
        if "vet" not in argv:

            class _Git:
                returncode = 0
                stdout = b"" if kwargs.get("input") is not None else ""
                stderr = ""

            return _Git()
        self.argv = list(argv)
        if self._raises is not None:
            raise self._raises

        class _Vet:
            returncode = outer._rc
            stdout = ""
            stderr = outer._err

        return _Vet()


# A real tagged-carrier path under TEST_ROOTS, used where the subject is the CHECK and not
# the tree walk. It has to exist and it has to sit under a walked root, because both are
# what go_tag_packages filters on. Its package is never really vetted: the fake intercepts.
_REAL_GO_TEST = "internal/component/bgp/message/header_test.go"
_REAL_GO_PKG = "./internal/component/bgp/message"
_REAL_GO_IMPORT = "github.com/ze-software/ze/internal/component/bgp/message"


class TestBuildTagsDerivation(unittest.TestCase):
    """The compile check has to use the tag set `make ze-unit-test` compiles with.

    Dropping the feature tags is the failure ai/rules/commands.md names: a bare `go`
    invocation excludes every gated file, so a tagged test behind `ze_ospf` is never
    type-checked and the check reports clean over code it did not read.
    """

    def test_tags_carry_ze_core_and_every_declared_feature_gate(self):
        tags = R.build_tags().split()
        self.assertIn("ze_core", tags)
        for gate in R.feature_tags():
            self.assertIn(gate, tags)
        self.assertTrue(R.feature_tags(), "feature-gates.txt declared no gate")

    def test_no_unexpanded_make_variable_survives(self):
        """A literal `$(ZE_FEATURES)` handed to `go vet` is a silent tag set of one."""
        self.assertNotIn("$", R.build_tags())

    def test_feature_tags_match_the_makefile_awk(self):
        """Derived from feature-gates.txt exactly as Makefile ZE_FEATURES derives them."""
        with open(R.FEATURE_GATES, encoding="utf-8") as fh:
            want = sorted(
                {
                    line.split()[0]
                    for line in fh
                    if line.split() and line.split()[0].startswith("ze_")
                }
            )
        self.assertEqual(list(R.feature_tags()), want)

    def test_missing_assignment_fails_closed(self):
        root = _mkdtemp("rfcgate-tags-")
        self.addCleanup(shutil.rmtree, root, True)
        mk = _write(root, "Makefile", "ZE_VERSION := 1\n")
        with self.assertRaises(R.ParseError) as cm:
            R.build_tags(makefile=mk)
        self.assertIn("GO_TEST_TAGS", str(cm.exception))

    def test_two_assignments_fail_closed(self):
        """Two answers is not an answer, the reading functional_suites already takes."""
        root = _mkdtemp("rfcgate-tags2-")
        self.addCleanup(shutil.rmtree, root, True)
        mk = _write(root, "Makefile", "GO_TEST_TAGS = ze_core\nGO_TEST_TAGS = ze_bgp\n")
        with self.assertRaises(R.ParseError) as cm:
            R.build_tags(makefile=mk)
        self.assertIn("GO_TEST_TAGS", str(cm.exception))

    def test_unknown_make_variable_fails_closed(self):
        root = _mkdtemp("rfcgate-tags3-")
        self.addCleanup(shutil.rmtree, root, True)
        mk = _write(root, "Makefile", "GO_TEST_TAGS = ze_core $(ZE_MYSTERY)\n")
        with self.assertRaises(R.ParseError) as cm:
            R.build_tags(makefile=mk)
        self.assertIn("ZE_MYSTERY", str(cm.exception))


class TestGoTagPackages(unittest.TestCase):
    """Which packages the check vets, derived from CARRIERS rather than an extension."""

    def test_the_fixture_path_is_real(self):
        """Guards every assertion below. A rename would otherwise turn each of them into a
        silent pass over an empty package list."""
        self.assertTrue(
            os.path.isfile(os.path.join(R.PROJECT_DIR, _REAL_GO_TEST)),
            f"{_REAL_GO_TEST} is gone; point _REAL_GO_TEST at another real _test.go "
            f"under one of {R.TEST_ROOTS}",
        )

    def test_go_carrier_tags_become_their_directory(self):
        pkgs = R.go_tag_packages([_tag("RFC7606-2-1", "positive", file=_REAL_GO_TEST)])
        self.assertEqual(pkgs, [_REAL_GO_PKG])

    def test_a_tag_outside_the_walked_roots_yields_no_package(self):
        """The `unit` carrier matches `_test.go` at ANY prefix, which is right for reading
        a tag and too wide for naming a package. scan_tree walks TEST_ROOTS only, so a
        scratch fixture under tmp/ is not a package to vet."""
        scratch = _mkdtemp("rfcgate-outside-")
        self.addCleanup(shutil.rmtree, scratch, True)
        rel = os.path.relpath(
            _write(scratch, "x_test.go", "package x\n"), R.PROJECT_DIR
        ).replace(os.sep, "/")
        self.assertTrue(rel.startswith("tmp/"), rel)
        self.assertEqual(
            R.go_tag_packages([_tag("RFC7606-2-1", "positive", file=rel)]), []
        )

    def test_non_go_carriers_are_not_vetted(self):
        """A .ci is data the runner interprets. There is no compile step to check."""
        pkgs = R.go_tag_packages(
            [_tag("RFC7606-2-1", "positive", file="test/plugin/rfc7606.ci")]
        )
        self.assertEqual(pkgs, [])

    def test_a_tag_whose_file_is_absent_yields_no_package(self):
        """scan_tree only reports files it READ, so an absent path is a synthetic tag and
        not a missing package. Inventing `./nonexistent` for one would fail `go vet` with
        an error that has nothing to do with RFC coverage."""
        self.assertEqual(R.go_tag_packages([_tag("RFC7606-2-1", "positive")]), [])

    def test_each_package_is_vetted_once(self):
        pkgs = R.go_tag_packages(
            [
                _tag("RFC7606-2-1", "positive", file=_REAL_GO_TEST),
                _tag("RFC7606-2-2", "negative", file=_REAL_GO_TEST, line=9),
            ]
        )
        self.assertEqual(pkgs, [_REAL_GO_PKG])


class TestTagPackagesCompile(unittest.TestCase):
    """A tag in a package the compiler rejects is not evidence.

    The sibling of _refuse_unrun: that one refuses a tag nothing RUNS, this one refuses a
    tag nothing can COMPILE. Both are admissibility, not coverage
    (ai/rules/evidence.md).
    """

    def _tags(self):
        return [_tag("RFC7606-2-1", "positive", file=_REAL_GO_TEST)]

    def test_a_broken_package_is_reported_with_its_tags_and_the_compiler_message(self):
        fake = _FakeVet(returncode=1, stderr=_FakeVet.REAL_FAILURE)
        with _patched(subprocess=fake):
            errs = R.check_tag_packages_compile(self._tags())
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("internal/component/ike/engine", errs[0])
        self.assertIn("undefined: log2tr", errs[0])
        self.assertIn("rfc7296_msgid_test.go:420", errs[0])

    def test_the_violation_names_the_requirement_at_stake(self):
        """A package path alone does not tell a reader which claim just lost its proof."""
        fake = _FakeVet(
            returncode=1,
            stderr=f"# {_REAL_GO_IMPORT}\nvet: {_REAL_GO_TEST}:3:1: undefined: nope\n",
        )
        with _patched(subprocess=fake):
            errs = R.check_tag_packages_compile(self._tags())
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("RFC7606-2-1", errs[0])

    def test_a_clean_vet_reports_nothing(self):
        """Discriminates from 'always fails'."""
        fake = _FakeVet(returncode=0)
        with _patched(subprocess=fake):
            self.assertEqual(R.check_tag_packages_compile(self._tags()), [])

    def test_no_go_tags_runs_no_toolchain(self):
        fake = _FakeVet(returncode=1, stderr=_FakeVet.REAL_FAILURE)
        with _patched(subprocess=fake):
            self.assertEqual(R.check_tag_packages_compile([]), [])
        self.assertIsNone(fake.argv)

    def test_the_command_selects_one_analyzer_and_the_unit_tags(self):
        fake = _FakeVet(returncode=0)
        with _patched(subprocess=fake):
            R.check_tag_packages_compile(self._tags())
        argv = fake.argv
        self.assertIsNotNone(argv)
        self.assertEqual(argv[:3], ["go", "vet", "-" + R.TYPECHECK_ANALYZER])
        self.assertIn("-tags", argv)
        self.assertIn("ze_core", argv[argv.index("-tags") + 1])
        self.assertIn(_REAL_GO_PKG, argv)

    def test_a_toolchain_that_fails_as_a_whole_fails_closed(self):
        """No `# package` header means the toolchain never got as far as a package. Naming
        one would be an accusation, and staying silent would credit every tag in the tree."""
        fake = _FakeVet(
            returncode=1, stderr="go: module lookup disabled by GOFLAGS=-mod=vendor\n"
        )
        with _patched(subprocess=fake):
            with self.assertRaises(R.ParseError) as cm:
                R.check_tag_packages_compile(self._tags())
        self.assertIn("module lookup disabled", str(cm.exception))

    def test_a_missing_toolchain_fails_closed(self):
        fake = _FakeVet(raises=FileNotFoundError("go"))
        with _patched(subprocess=fake):
            with self.assertRaises(R.ParseError) as cm:
                R.check_tag_packages_compile(self._tags())
        self.assertIn("go vet", str(cm.exception))

    def test_a_hung_toolchain_fails_closed(self):
        fake = _FakeVet(raises=subprocess.TimeoutExpired(cmd="go vet", timeout=1))
        with _patched(subprocess=fake):
            with self.assertRaises(R.ParseError) as cm:
                R.check_tag_packages_compile(self._tags())
        self.assertIn("go vet", str(cm.exception))

    def test_a_test_build_header_folds_onto_the_same_package(self):
        """`go vet` prints the header twice for a test build, once bare and once as
        `# [pkg]`, and a test package can arrive as `# pkg [pkg.test]`. All three name ONE
        package, so all three have to fold onto one violation rather than three."""
        fake = _FakeVet(
            returncode=1,
            stderr=(
                f"# {_REAL_GO_IMPORT} [{_REAL_GO_IMPORT}.test]\n"
                f"# [{_REAL_GO_IMPORT}]\n"
                f"vet: {_REAL_GO_TEST}:3:1: undefined: nope\n"
            ),
        )
        with _patched(subprocess=fake):
            errs = R.check_tag_packages_compile(self._tags())
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("RFC7606-2-1", errs[0])

    def test_a_broken_dependency_is_named_without_inventing_a_requirement(self):
        """go vet reports the DEPENDENCY under its own header when a tagged package fails
        to build because of it. That package holds no tag, so the violation must say what
        it blocks instead of attributing a requirement that does not live there."""
        fake = _FakeVet(
            returncode=1,
            stderr=(
                "# github.com/ze-software/ze/internal/core/thing\n"
                "vet: internal/core/thing/thing.go:9:2: undefined: helper\n"
            ),
        )
        with _patched(subprocess=fake):
            errs = R.check_tag_packages_compile(self._tags())
        self.assertEqual(len(errs), 1, errs)
        self.assertIn("internal/core/thing", errs[0])
        self.assertIn("depends on it", errs[0])
        self.assertNotIn("RFC7606-2-1", errs[0])

    def test_a_wall_of_compiler_errors_is_truncated(self):
        """One broken import produces a type error per use site. The whole list in one
        message helps nobody (ai/rules/cli.md, "Truncate large blobs")."""
        lines = "".join(
            f"vet: {_REAL_GO_TEST}:{n}:1: undefined: nope\n" for n in range(1, 41)
        )
        fake = _FakeVet(returncode=1, stderr=f"# {_REAL_GO_IMPORT}\n{lines}")
        with _patched(subprocess=fake):
            errs = R.check_tag_packages_compile(self._tags())
        self.assertEqual(len(errs), 1, errs)
        self.assertIn(f"and {40 - R._QUOTED_MESSAGES} more", errs[0])
        self.assertLess(len(errs[0]), 900, errs[0])
        # Still says what and where: a truncated message that names nothing is no message.
        self.assertIn("undefined: nope", errs[0])

    def test_every_broken_package_is_reported_not_just_the_first(self):
        """`go vet` reports each failing package, and so does this check (measured)."""
        fake = _FakeVet(
            returncode=1,
            stderr=(
                "# github.com/ze-software/ze/internal/core/probe\n"
                "vet: internal/core/probe/a_test.go:5:39: undefined: alpha\n"
                "# github.com/ze-software/ze/internal/plugins/tftpserver\n"
                "vet: internal/plugins/tftpserver/b_test.go:5:39: undefined: beta\n"
            ),
        )
        with _patched(subprocess=fake):
            errs = R.check_tag_packages_compile(self._tags())
        self.assertEqual(len(errs), 2, errs)
        self.assertTrue(any("internal/core/probe" in e for e in errs), errs)
        self.assertTrue(any("tftpserver" in e for e in errs), errs)


class TestTagPackagesCompileWiring(unittest.TestCase):
    """check_tag_packages_compile is dead code unless run_check calls it."""

    def _drive(self, fake):
        with _patched(
            subprocess=fake,
            load_enrolled=lambda: {"rfc7606"},
            summary_stems=lambda: {"rfc7606"},
            parse_summary_file=lambda path: [_req("RFC7606-2-1")],
            _git_baseline_enrolment=lambda: {"rfc7606"},
            _git_baseline_ids=lambda: {"RFC7606-2-1"},
            _git_baseline_tag_polarities=lambda: {},
            _git_baseline_evidence=lambda: {},
            _git_baseline_summary_stems=lambda: {"rfc7606"},
            scan_tree=lambda *a, **k: [
                _tag("RFC7606-2-1", "positive", file=_REAL_GO_TEST),
                _tag("RFC7606-2-1", "negative", file=_REAL_GO_TEST, line=9),
            ],
            check_status_agreement=lambda *a, **k: [],
            check_audit_files=lambda *a, **k: [],
            check_audit_schema=lambda *a, **k: [],
            check_audit_freshness=lambda *a, **k: [],
            check_audit_disclosure=lambda *a, **k: [],
            check_audit_note=lambda *a, **k: [],
            check_audit_findings=lambda *a, **k: [],
            check_audit_verdict_ratchet=lambda *a, **k: [],
            signed_extractions=lambda reqs_: {},
            check_extraction_signoff=lambda *a, **k: [],
            check_extraction_ratchet=lambda *a, **k: [],
            check_drain_floor=lambda *a, **k: [],
            check_summary_disposition=lambda *a, **k: [],
            check_status_completeness=lambda *a, **k: [],
            check_unproven_support=lambda *a, **k: [],
            check_gap_count_agreement=lambda *a, **k: [],
            check_ledger_fresh=lambda *a, **k: [],
        ):
            return _run_capturing(R.run_check)

    def test_run_check_fails_when_a_tagged_package_does_not_compile(self):
        code, out = self._drive(
            _FakeVet(
                returncode=1,
                stderr=f"# {_REAL_GO_IMPORT}\nvet: {_REAL_GO_TEST}:3:1: undefined: nope\n",
            )
        )
        self.assertEqual(code, 2, out)
        self.assertIn("undefined: nope", out)
        self.assertIn("RFC7606-2-1", out)

    def test_run_check_is_clean_when_the_package_compiles(self):
        """Discriminates from 'always fails'."""
        code, out = self._drive(_FakeVet(returncode=0))
        self.assertEqual(code, 0, out)

    def test_run_check_fails_closed_when_the_toolchain_cannot_run(self):
        code, out = self._drive(_FakeVet(raises=FileNotFoundError("go")))
        self.assertEqual(code, 2, out)
        self.assertIn("cannot run", out)


class TestRealTree(unittest.TestCase):
    """The RFC 7296 pilot's Wiring Test, asserted over the REAL rfc/ tree.

    Design: docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md. Deliberately not
    patched. What these prove is a fact about THIS repository -- the pilot grew
    rfc/short/rfc7296.md from 23 rows to 227 and proved every gated one of them in both
    polarities -- and a synthetic fixture would pass with all 204 landed rows absent, which
    is the whole failure the pilot exists to end.

    R-12 is why they arrive green rather than red-first. `make ze-rfc-check` runs
    `--selftest` over this file (rfc_requirements.py, run_selftest) and
    scripts/dev/python_tests_test.go globs the same path into ze-unit-test, so a red case
    here reds BOTH verify branches and commit_helper.py then refuses every session's
    commits. The spec's Resolution 2 settles it: TDD requires the test to fail before the
    implementation exists, not to be committed failing. Each case's red was observed and
    recorded in the spec before its subject landed.
    """

    STEM = "rfc7296"
    PREFIX = "RFC7296-"

    # The 23 rows rfc/short/rfc7296.md carried before the pilot's first landing, with the
    # LEVEL each one carried, read from `git show 9551e66f4^:rfc/short/rfc7296.md`.
    # Recorded here rather than re-derived, because all three ratchets below compare against
    # HEAD and HEAD has already moved past the re-authoring they were written to police.
    #
    # The levels are recorded, not just the ids, because the level ratchet's whole subject
    # is a row leaving the gated population. 18 of these 23 were MUST-level; a baseline
    # derived from today's rows would carry whatever level each row has NOW, which is the
    # tautology `test_rfc7296_ids_are_neither_retired_nor_lose_a_polarity` used to be
    # named for.
    PRE_PILOT_LEVELS = {
        "RFC7296-1.2-1": "MUST",
        "RFC7296-1.3.3-1": "MUST",
        "RFC7296-1.4-1": "MUST",
        "RFC7296-2.1-1": "SHOULD",
        "RFC7296-2.1-2": "SHOULD",
        "RFC7296-2.4-1": "MUST",
        "RFC7296-2.4-2": "SHOULD",
        "RFC7296-2.4-3": "MUST NOT",
        "RFC7296-2.4-4": "MUST",
        "RFC7296-2.6-1": "MUST",
        "RFC7296-2.7-1": "MUST",
        "RFC7296-2.8-1": "MUST",
        "RFC7296-2.8-2": "MUST NOT",
        "RFC7296-2.8-3": "SHOULD",
        "RFC7296-2.9-1": "MUST",
        "RFC7296-2.23-1": "MUST",
        "RFC7296-2.23-2": "MUST",
        "RFC7296-2.23-3": "MUST",
        "RFC7296-3.3-1": "MUST",
        "RFC7296-3.3-2": "MUST",
        "RFC7296-3.3.2-1": "MUST",
        "RFC7296-3.3.6-1": "MUST",
        "RFC7296-3.8-1": "SHOULD",
    }
    PRE_PILOT_IDS = frozenset(PRE_PILOT_LEVELS)

    # The three ids that carried an annotation before the pilot: {single-polarity} on
    # 3.3-1 and {gap} on the other two. AC-18: each was CLEARED by the work that
    # implemented its obligation. None was reclassified downward, which only the owner may
    # do (ai/rules/rfc-compliance.md).
    FORMERLY_ANNOTATED = ("RFC7296-3.3-1", "RFC7296-2.9-1", "RFC7296-1.4-1")

    # The pilot's landed figure. A floor, never an equality: a later row is welcome, and
    # deleting rows must not be the cheap way to keep this case green.
    #
    # It must be the figure the tree ACTUALLY carries. A floor set below it is slack: at
    # 218 against a tree of 222 the four MUSTs the extraction walk had just found could
    # each be deleted without reding this case, which is the one move the floor exists to
    # refuse. Raise it in the commit that raises the count.
    #
    # LOWERED 222 -> 221 on 2026-08-15, on the owner's authorisation, and this is the
    # one shape of decrease the floor may take. No row was deleted and no id moved:
    # RFC7296-2.8-1 recorded [MUST] over a sentence RFC 7296 Section 2.8.1 writes as
    # "SHOULD be closed by the endpoint that created it", so correcting the quotation
    # moved that row out of the gated MUST-level population. Ze's behaviour is
    # unchanged and still meets it. A decrease that cannot name the row it corrected,
    # and the RFC sentence that corrected it, is the deletion this floor refuses.
    #
    # THE FLOOR IS NOT THE CONTROL, and never was: the commit that lowered the level
    # edited this integer in the same breath, so it recorded the demotion instead of
    # refusing it, and 165 other enrolled RFCs have no floor at all. Since 2026-08-17 the
    # control is check_level_ratchet, which compares every row's level against HEAD and
    # demands the correction paragraph in the summary
    # (`test_rfc7296_pre_pilot_musts_are_still_gated_or_corrected`). What survives here is
    # a count no ratchet can supply: 221 rows landed in the pilot, and a mass deletion
    # spread over several commits would clear each HEAD-to-HEAD comparison one row at a
    # time while this case still fails.
    GATED_FLOOR = 221

    @classmethod
    def setUpClass(cls):
        cls.reqs = R.parse_summary_file(os.path.join(R.SUMMARY_DIR, cls.STEM + ".md"))
        cls.tags = [t for t in R.scan_tree() if t.rid.startswith(cls.PREFIX)]
        cls.polarities = {}
        for tag in cls.tags:
            cls.polarities.setdefault(tag.rid, set()).add(tag.polarity)
        cls.enrolled = R.load_enrolled()

    def test_rfc7296_signoff_is_valid_and_the_rest_stay_grandfathered(self):
        """AC-5. The sign-off is VALID, and grandfathering stays the majority position.

        rfc/extraction/rfc7296.json landed signed on 2026-08-02, so the first half now
        takes its live branch: the artifact exists, therefore it MUST be a valid sign-off.
        That is the strong form. R-13 measured the tempting alternative, and an UNSIGNED
        skeleton committed here yields 385 gate errors and exit 2; this names that move in
        one line instead of 385, whichever branch it is on.

        "Grandfathered" is asserted only while a stem has no artifact, because the two
        states are exclusive: a signed stem is signed. What survives the sign-off is the
        claim that carries the meaning, that unsigned stems remain the MAJORITY. That is
        what "grandfathering is scope, not an allowlist" means
        (rfc/extraction/README.md), and it breaks if evaluate_extractions starts demanding
        an artifact from a stem that predates the bar.
        """
        self.assertIn(self.STEM, self.enrolled, "rfc7296 must stay enrolled")

        _enrolled, all_reqs, _errs, _tags, _by_stem = R._collect_for_check()
        signed, violations = R.evaluate_extractions(all_reqs)

        artifact = os.path.join(R.EXTRACTION_DIR, self.STEM + ".json")
        if os.path.exists(artifact):
            self.assertIn(
                self.STEM,
                signed,
                f"{artifact} exists but is not a VALID sign-off: {violations}",
            )
        else:
            self.assertNotIn(self.STEM, signed)

        self.assertEqual(
            violations,
            [],
            "an artifact in rfc/extraction/ is unsigned or contradicted by its source",
        )
        unsigned = self.enrolled - set(signed)
        if not os.path.exists(artifact):
            self.assertIn(
                self.STEM,
                unsigned,
                "rfc7296 has no artifact, so it must be grandfathered rather than accused",
            )
        self.assertGreater(
            len(unsigned),
            len(signed),
            "grandfathering must stay the majority position, or the bound moved silently",
        )

    def test_rfc7296_summary_carries_the_section_2_2_requirements(self):
        """AC-1. The two Message ID obligations, each citing the section its id names.

        2.2-2 is the one the walk found unimplemented: NextMsgID was a bare uint32 whose
        every mutation was an unchecked `++`, so the SA wrapped to 0 and kept running.
        engine/msgid.go now freezes both counters at the ceiling.
        """
        by_rid = {r.rid: r for r in self.reqs}
        for rid in ("RFC7296-2.2-1", "RFC7296-2.2-2"):
            req = by_rid.get(rid)
            self.assertIsNotNone(req, f"{rid} missing from rfc/short/rfc7296.md")
            self.assertEqual(req.section, "2.2", f"{rid} cites the wrong section")
            self.assertTrue(req.gated, f"{rid} must stay MUST-level")
            self.assertIsNone(req.annotation, f"{rid} carries an annotation")
            self.assertEqual(
                self.polarities.get(rid), R.POLARITIES, f"{rid} is not proven both ways"
            )

    def test_rfc7296_every_gated_row_is_proven_in_both_polarities(self):
        """AC-2 and AC-4: 221 gated rows, every one proven positive AND negative, and not
        one annotation in the file.

        Owner ruling OR-1 gave this spec no annotation budget. Nothing mechanical enforces
        that, because {gap} is a LEGAL annotation the gate accepts (umbrella R-9), so
        absorbing a newly extracted MUST into a fresh annotation is the cheapest way to
        keep the gate green over a row nobody implemented. This case is the catch.

        221 rather than the pilot's 222 since 2026-08-15: see GATED_FLOOR above for the row
        whose level was corrected and the RFC sentence that corrected it. The file still
        carries 227 rows, so the count moved without a row moving.
        """
        gated = [r for r in self.reqs if r.gated]
        self.assertGreaterEqual(
            len(gated),
            self.GATED_FLOOR,
            f"the pilot landed {self.GATED_FLOOR} gated rows; rows do not disappear",
        )
        unproven = sorted(
            r.rid for r in gated if self.polarities.get(r.rid) != R.POLARITIES
        )
        self.assertEqual(unproven, [], "gated rows lacking a polarity")
        annotated = sorted(f"{r.rid} {r.annotation}" for r in self.reqs if r.annotation)
        self.assertEqual(annotated, [], "OR-1 permits no annotation on any rfc7296 row")

    def test_rfc7296_ids_are_neither_retired_nor_lose_a_polarity(self):
        """AC-3, driving check_retired_requirements and check_coverage_ratchet.

        Both ratchets compare the working tree against HEAD, and HEAD sits well past the
        pilot's start, so neither can still see the 23 -> 227 re-authoring they protect.
        Feeding them the RECORDED pre-pilot baseline restores that reach. The polarity
        baseline claims BOTH for every gated pre-pilot id, which is the strongest claim a
        real loss could fail against.

        WHAT THIS CASE COVERS, and what it does not. It catches a pre-pilot id that is
        RETIRED, and one that LOSES a polarity. It does not judge a LEVEL change: `baseline`
        is built from `r.gated` over the CURRENT rows, so an id that stops being MUST-level
        leaves the baseline rather than failing against it. The case was named
        `..._nor_demoted` until 2026-08-17 and RFC7296-2.8-1 went [MUST] -> [SHOULD] on
        2026-08-15 straight through it. The assertions were all true; the NAME reached
        further than any of them, and a reader who grepped for a demotion ratchet found it
        and stopped looking. The level is now
        `test_rfc7296_pre_pilot_musts_are_still_gated_or_corrected`'s subject, over the
        recorded PRE_PILOT_LEVELS and check_level_ratchet.
        """
        live = {r.rid for r in self.reqs}
        self.assertEqual(
            sorted(self.PRE_PILOT_IDS - live), [], "a pre-pilot id was retired"
        )

        stems = R.summary_stems()
        retired = R.check_retired_requirements(
            self.reqs,
            self.enrolled,
            set(self.PRE_PILOT_IDS),
            {self.STEM},
            stems,
            stems,
        )
        self.assertEqual(retired, [])

        baseline = {
            r.rid: set(R.POLARITIES)
            for r in self.reqs
            if r.gated and r.rid in self.PRE_PILOT_IDS
        }
        self.assertTrue(baseline, "the pre-pilot baseline must not be empty")
        demoted = R.check_coverage_ratchet(
            self.reqs, self.tags, self.enrolled, baseline, {self.STEM}
        )
        self.assertEqual(demoted, [])

        for rid in self.FORMERLY_ANNOTATED:
            self.assertIn(rid, live, f"{rid} was retired rather than cleared")
            req = next(r for r in self.reqs if r.rid == rid)
            self.assertIsNone(req.annotation, f"{rid} carries an annotation again")
            self.assertEqual(
                self.polarities.get(rid),
                R.POLARITIES,
                f"{rid} lost the polarity that let its annotation be cleared",
            )

    def test_rfc7296_pre_pilot_musts_are_still_gated_or_corrected(self):
        """AC-3's missing half, over the REAL summary and the REAL RFC text.

        18 of the 23 pre-pilot rows were MUST-level. Any one of them may leave that
        population only behind a recorded correction, and exactly one has: RFC7296-2.8-1,
        whose `Correction 2026-08-15:` paragraph in rfc/short/rfc7296.md quotes §2.8.1's
        "SHOULD be closed by the endpoint that created it" -- a sentence this case's second
        half proves is really in rfc/full/rfc7296.txt.

        This is the case GATED_FLOOR was standing in for. A single integer maintained by
        hand, in the file that also holds the assertion, was edited by the very commit that
        lowered the level: a record of the demotion, never a control over it.
        """
        errs = R.check_level_ratchet(
            self.reqs, self.enrolled, dict(self.PRE_PILOT_LEVELS), {self.STEM}
        )
        self.assertEqual(
            errs, [], "a pre-pilot MUST left the gated population unrecorded"
        )

        # Discrimination: the silence above is a verdict about THIS tree, not the only
        # answer the check can give. Take the corrections away and RFC7296-2.8-1's [MUST]
        # -> [SHOULD] is exactly what the ratchet exists to refuse.
        with _patched(summary_corrections=lambda stem: []):
            unrecorded = R.check_level_ratchet(
                self.reqs, self.enrolled, dict(self.PRE_PILOT_LEVELS), {self.STEM}
            )
        self.assertEqual(len(unrecorded), 1, unrecorded)
        self.assertIn("RFC7296-2.8-1", unrecorded[0])

    def test_rfc7296_ledger_is_fresh_after_the_pilot(self):
        """Story 1: an operator sizes ze's IKEv2 from docs/features/rfc-status.md, and that
        page's evidence is ai/RFC-REQUIREMENTS.md, which is DERIVED from the summaries and
        the `RFC requirement:` tags.

        566 tag lines across 73 files moved during the pilot. A moved tag line stales the
        ledger, and the ledger then cites a test at a line that holds something else. The
        fix is `make ze-rfc-index-update` in the same commit, never a hand edit.
        """
        enrolled, reqs, _errs, tags, _by_stem = R._collect_for_check()
        self.assertEqual(
            R.check_ledger_fresh(reqs, tags, enrolled),
            [],
            "regenerate with: make ze-rfc-index-update",
        )

        # The rows live in RFC 7296's own shard, which is where the operator's question
        # ("which test enforces this requirement") is answered after the split.
        path = R.shard_path("rfc7296")
        self.assertTrue(
            os.path.exists(path), f"{path} is absent -- run: make ze-rfc-index-update"
        )
        with open(path, encoding="utf-8") as fh:
            shard = fh.read()
        # One id from each half of the pilot: a row that was already implemented and only
        # needed proof, one that was a LIVE defect (a peer Delete of the live Child SA was
        # ignored), and one the walk found had no producer at all (keys survived SA close).
        for rid in ("RFC7296-2.2-1", "RFC7296-1.4.1-6", "RFC7296-2.12-1"):
            self.assertIn(rid, shard, f"{rid} is absent from the generated shard")


class TestRealTreeIsGreen(unittest.TestCase):
    """AC-19, AC-26: the whole gate, over the committed tree.

    The four new checks are ARMED here. Every other test in this section drives one check
    over a fixture; this one is the only assertion that the REPOSITORY passes, which is what
    AC-26 requires at every commit boundary.
    """

    def test_run_check_exits_zero_on_the_real_tree(self):
        code, out = _run_capturing(R.run_check)
        self.assertEqual(code, 0, out)
        self.assertIn("rfc-requirements OK", out)


if __name__ == "__main__":
    unittest.main()
