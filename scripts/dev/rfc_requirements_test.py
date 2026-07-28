#!/usr/bin/env python3
"""Tests for rfc_requirements.py — the RFC requirement coverage gate.

Auto-discovered and run under `go test` by scripts/dev/python_tests_test.go
(glob over scripts/dev/*_test.py). No make target needed.

Spec: plan/spec-rfc-requirement-coverage.md
"""

import contextlib
import importlib.util
import io
import os
import sys
import tempfile
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


@contextlib.contextmanager
def _patched(**overrides):
    """Temporarily replace module-level attributes on R, restoring them afterward.

    Used by the GATE-LEVEL tests: they drive run_check / run_check_fresh end-to-end with
    controlled data sources so a wiring regression (a check that stops being called) fails
    the test, not just the helper in isolation.
    """
    saved = {name: getattr(R, name) for name in overrides}
    try:
        for name, val in overrides.items():
            setattr(R, name, val)
        yield
    finally:
        for name, val in saved.items():
            setattr(R, name, val)


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
        to prevent (ai/rules/fail-closed-guards.md).
        """
        line = "- [ ] [MUST] legacy line with no ID (§2)"
        with self.assertRaises(R.ParseError):
            R.parse_checklist_line(line, "rfc7606")

    def test_retired_counter_form_fails_loudly(self):
        """The retired per-RFC counter form must ERROR, never be silently skipped.

        Regression: a line like `- [ ] [RFC9234-R012] [MUST] ... (§5)` has an unrecognised
        first bracket, so it was dismissed as an ad-hoc category line and dropped — taking
        a live MUST out of the ledger with it. A silently skipped obligation is exactly the
        false green this gate exists to prevent (ai/rules/fail-closed-guards.md).
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

    def test_verdict_fresh_when_nothing_changed(self):
        verdict = {"requirement_sha": "aaa", "tests": {"a_test.go:10": "bbb"}}
        self.assertTrue(R.verdict_is_fresh(verdict, "aaa", {"a_test.go:10": "bbb"}))

    def test_verdict_stale_when_requirement_sha_changes(self):
        verdict = {"requirement_sha": "aaa", "tests": {"a_test.go:10": "bbb"}}
        self.assertFalse(
            R.verdict_is_fresh(verdict, "CHANGED", {"a_test.go:10": "bbb"})
        )

    def test_verdict_stale_when_test_sha_changes(self):
        verdict = {"requirement_sha": "aaa", "tests": {"a_test.go:10": "bbb"}}
        self.assertFalse(
            R.verdict_is_fresh(verdict, "aaa", {"a_test.go:10": "CHANGED"})
        )

    def test_verdict_stale_when_test_disappears_or_appears(self):
        """A tagged test deleted, or a new one added, both invalidate the verdict."""
        verdict = {"requirement_sha": "aaa", "tests": {"a_test.go:10": "bbb"}}
        self.assertFalse(R.verdict_is_fresh(verdict, "aaa", {}))
        self.assertFalse(
            R.verdict_is_fresh(
                verdict, "aaa", {"a_test.go:10": "bbb", "b_test.go:1": "ccc"}
            )
        )


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
        stale assurance. That is why it FAILS while a missing one does not."""
        v = {"requirement_sha": R.requirement_sha("x"), "tests": {}}
        self.assertTrue(R.verdict_is_fresh(v, R.requirement_sha("x"), {}))
        self.assertFalse(R.verdict_is_fresh(v, R.requirement_sha("CHANGED"), {}))

    def test_requirement_text_edit_stales_the_verdict(self):
        a = R.requirement_sha("MUST discard the attribute")
        b = R.requirement_sha("MUST treat the UPDATE as withdrawn")
        self.assertNotEqual(
            a, b, "re-reading the RFC must invalidate the old judgement"
        )


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
    def test_ledger_render_stable(self):
        reqs = [_req("RFC7606-2-1"), _req("RFC7606-2-2")]
        tags = [_tag("RFC7606-2-1", "positive"), _tag("RFC7606-2-1", "negative")]
        a = R.render_ledger(reqs, tags, {"rfc7606"})
        b = R.render_ledger(reqs, tags, {"rfc7606"})
        self.assertEqual(a, b)
        self.assertIn("RFC7606-2-1", a)

    def test_ledger_shows_both_directions(self):
        """The generated half of the two-way reference."""
        reqs = [_req("RFC7606-2-1")]
        tags = [
            _tag("RFC7606-2-1", "positive", file="p_test.go", line=7),
            _tag("RFC7606-2-1", "negative", file="n_test.go", line=9),
        ]
        out = R.render_ledger(reqs, tags, {"rfc7606"})
        self.assertIn("p_test.go:7", out)
        self.assertIn("n_test.go:9", out)

    def test_citation_order_independent_of_scan_order(self):
        """os.walk yields files in filesystem order, so the render must sort citations
        by (file, line) — otherwise the ledger churns across machines and the freshness
        gate (AC-20) flags a stale ledger that is not actually wrong."""
        reqs = [_req("RFC7606-2-1")]
        tags = [
            _tag("RFC7606-2-1", "negative", file="a_test.go", line=5),
            _tag("RFC7606-2-1", "negative", file="a_test.go", line=90),
            _tag("RFC7606-2-1", "negative", file="z_test.go", line=1),
        ]
        forward = R.render_ledger(reqs, list(tags), {"rfc7606"})
        backward = R.render_ledger(reqs, list(reversed(tags)), {"rfc7606"})
        self.assertEqual(forward, backward)
        # sorted by (file, line): a:5 < a:90 (numeric, not lexical) < z:1
        self.assertLess(forward.index("a_test.go:5"), forward.index("a_test.go:90"))
        self.assertLess(forward.index("a_test.go:90"), forward.index("z_test.go:1"))


class TestLedgerFreshness(unittest.TestCase):
    """AC-20: a stale ai/RFC-REQUIREMENTS.md must fail the build, not rot silently.
    This is what let the ledger drift once already — two commits re-tagged tests without
    regenerating it and nothing caught it."""

    _reqs = [_req("RFC7606-2-1")]
    _tags = [_tag("RFC7606-2-1", "positive"), _tag("RFC7606-2-1", "negative")]

    def _with_ledger(self, contents):
        """Point R.LEDGER_FILE at a temp file holding `contents` (None = absent)."""
        fd, path = tempfile.mkstemp(suffix=".md")
        os.close(fd)
        if contents is None:
            os.unlink(path)
        else:
            with open(path, "w", encoding="utf-8") as fh:
                fh.write(contents)
        return path

    def _check(self, contents):
        path = self._with_ledger(contents)
        orig = R.LEDGER_FILE
        try:
            R.LEDGER_FILE = path
            return R.check_ledger_fresh(self._reqs, self._tags, {"rfc7606"})
        finally:
            R.LEDGER_FILE = orig
            if os.path.exists(path):
                os.unlink(path)

    def test_fresh_when_file_matches_render(self):
        body = R.render_ledger(self._reqs, self._tags, {"rfc7606"}) + "\n"
        self.assertEqual(self._check(body), [])

    def test_stale_when_file_differs(self):
        errs = self._check("not the rendered ledger\n")
        self.assertEqual(len(errs), 1)
        self.assertIn("ze-rfc-index", errs[0])

    def test_missing_ledger_reads_as_stale(self):
        """A missing ledger is '' != body, so it fails closed rather than passing by
        vacuum (ai/rules/fail-closed-guards.md)."""
        errs = self._check(None)
        self.assertEqual(len(errs), 1)


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


class TestBaselineReaders(unittest.TestCase):
    """The two new HEAD readers. Both must degrade to 'no baseline' rather than crash or
    invent one: the gate runs in worktrees, in CI checkouts and on the first commit."""

    def test_tag_polarities_survives_git_failure(self):
        """AC-14. Only `git grep` fails; the following cat-file SUCCEEDS with a tag-bearing
        blob. An implementation that ignores the grep returncode therefore returns a real
        baseline here, instead of being masked by the second command failing too."""
        with _patched(subprocess=_FakeSubprocess(returncode=128, fail_only="grep")):
            self.assertEqual(R._git_baseline_tag_polarities(), {})

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
            _git_baseline_summary_stems=lambda: {"rfc7606"},
            scan_tree=lambda *a, **k: tags,
            check_status_agreement=lambda *a, **k: [],
            check_audit_freshness=lambda *a, **k: [],
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
# New-summary enrolment (spec-rfc-gate-regression-ratchets AC-9..AC-13)
# --------------------------------------------------------------------------
class TestNewSummaryEnrolment(unittest.TestCase):
    """A new rfc/short/*.md is un-enrolled by definition, so adding an RFC used to add
    exactly no checking. Only summaries that are NEW since HEAD are judged: the ones that
    predate it are the existing backlog, and re-litigating it here would block every
    unrelated commit."""

    def _errs(self, stems, baseline, enrolled, reqs, parse_errors=None, src=0):
        with _patched(source_keyword_count=lambda stem: src):
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
        """rfc/full/<stem>.txt absent: source_keyword_count returns None. Guessing a
        violation from a missing file would punish RFCs we simply have not downloaded."""
        errs = self._errs(["rfc9999", "rfc0000"], ["rfc0000"], [], [], src=None)
        self.assertEqual(errs, [])

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
            _git_baseline_summary_stems=lambda: {"rfc7606"},
            scan_tree=lambda *a, **k: tags,
            check_status_agreement=lambda *a, **k: [],
            check_audit_freshness=lambda *a, **k: [],
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
            _git_baseline_summary_stems=lambda: stems_baseline,
            source_keyword_count=lambda stem: 23,
            scan_tree=lambda *a, **k: [
                _tag("RFC7606-2-1", "positive"),
                _tag("RFC7606-2-1", "negative", line=2),
            ],
            check_status_agreement=lambda *a, **k: [],
            check_audit_freshness=lambda *a, **k: [],
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
            _git_baseline_summary_stems=lambda: baseline_stems,
            scan_tree=lambda *a, **k: [
                _tag("RFC7606-2-1", "positive"),
                _tag("RFC7606-2-1", "negative", line=2),
            ],
            check_status_agreement=lambda *a, **k: [],
            check_audit_freshness=lambda *a, **k: [],
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


# --------------------------------------------------------------------------
# Ledger staleness (AC-20) — run_check_fresh / --check-fresh
# --------------------------------------------------------------------------
class TestLedgerStaleness(unittest.TestCase):
    """AC-20 at the CLI level: a committed ai/RFC-REQUIREMENTS.md that drifts from a fresh
    render must fail `--check-fresh` (what ze-doc-test runs) and name the regeneration
    target. TestLedgerFreshness drives the check_ledger_fresh helper; this drives the
    run_check_fresh entry point, so an unwired helper still fails here. Driven from
    fixtures so it does not depend on the live tree (whose tags may be mid-flight)."""

    REQS = [_req("RFC7606-2-1")]
    TAGS = [_tag("RFC7606-2-1", "positive"), _tag("RFC7606-2-1", "negative")]

    def _drive(self, committed):
        fd, path = tempfile.mkstemp(suffix=".md")
        os.close(fd)
        if committed is None:
            os.unlink(path)
        else:
            with open(path, "w", encoding="utf-8") as fh:
                fh.write(committed)
        orig = R.LEDGER_FILE
        try:
            R.LEDGER_FILE = path
            with _patched(
                _collect_for_check=lambda: ({"rfc7606"}, self.REQS, [], self.TAGS, {}),
            ):
                return _run_capturing(R.run_check_fresh)
        finally:
            R.LEDGER_FILE = orig
            if os.path.exists(path):
                os.unlink(path)

    def test_fresh_ledger_passes(self):
        fresh = R.render_ledger(self.REQS, self.TAGS, {"rfc7606"}) + "\n"
        code, out = self._drive(fresh)
        self.assertEqual(code, 0, out)

    def test_stale_ledger_fails_and_names_regen_target(self):
        """Would pass under a version that never compared; fails because the committed copy
        differs from the fresh render, and the message points at the regeneration target."""
        code, out = self._drive("# a stale, hand-drifted ledger\n")
        self.assertNotEqual(code, 0, out)
        self.assertIn("ze-rfc-index", out)

    def test_missing_ledger_fails(self):
        """A missing ledger must fail closed at the CLI too, not pass by vacuum."""
        code, out = self._drive(None)
        self.assertNotEqual(code, 0, out)

    def test_render_is_deterministic(self):
        """--check-fresh only works if render is stable: two renders of the same inputs
        must be byte-identical, else a fresh ledger would spuriously read as stale."""
        a = R.render_ledger(self.REQS, self.TAGS, {"rfc7606"})
        b = R.render_ledger(self.REQS, self.TAGS, {"rfc7606"})
        self.assertEqual(a, b)


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
# Real-tree sanity
# --------------------------------------------------------------------------
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


if __name__ == "__main__":
    unittest.main()
