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
import shutil
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
        forward = R.render_ledger(reqs, tags, {"rfc7606"})
        backward = R.render_ledger(
            list(reversed(reqs)), list(reversed(tags)), {"rfc7606"}
        )
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
        path = _mkstemp(".md")
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
                scan_tree=lambda *a, **k: list(tags),
                check_status_agreement=lambda *a, **k: [],
                check_audit_freshness=lambda *a, **k: [],
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
    render must fail `--check-fresh` (what ze-doc-test runs) and name the regeneration
    target. TestLedgerFreshness drives the check_ledger_fresh helper; this drives the
    run_check_fresh entry point, so an unwired helper still fails here. Driven from
    fixtures so it does not depend on the live tree (whose tags may be mid-flight)."""

    REQS = [_req("RFC7606-2-1")]
    TAGS = [_tag("RFC7606-2-1", "positive"), _tag("RFC7606-2-1", "negative")]

    def _drive(self, committed):
        path = _mkstemp(".md")
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

    def test_render_is_independent_of_tag_order(self):
        """--check-fresh only works if the render does not depend on the order the scanner
        happened to find things in: scan_tree walks the filesystem, so the same tree yields
        the same tags in a different order on a different machine, and an order-sensitive
        render would report a fresh ledger as stale there. Reversing the inputs is the test
        that can see that; re-rendering identical arguments is not."""
        tags = list(reversed(self.TAGS))
        self.assertEqual(
            R.render_ledger(self.REQS, self.TAGS, {"rfc7606"}),
            R.render_ledger(list(reversed(self.REQS)), tags, {"rfc7606"}),
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


@contextlib.contextmanager
def _extraction_tree(artifacts=None, budget="start 2026-07-01\nrate 0\n", src=None):
    """A temp rfc/extraction/ plus rfc/drain-budget.txt, with the source text patched.

    `artifacts` maps stem -> dict (written as JSON) or str (written verbatim, for the
    malformed-input cases). `budget=None` deletes the budget file.
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
    claim, and claims are what this programme removes (ai/rules/derive-not-hardcode.md)."""

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
        reads as 'nothing to extract' (ai/rules/fail-closed-guards.md, zero-value trap)."""
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
        emits one produces a skeleton that cannot be re-read: `make ze-rfc-extract` exits 0
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
        swallow (ai/rules/fail-closed-guards.md: a guard that cannot deny must say so)."""
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

        Without it `make ze-rfc-extract STEM=rfc2865` exited 0 announcing success while
        writing a file that could not be re-read -- and one such committed file makes every
        later `--check` print 'cannot run', hiding EVERY other RFC violation in the repo.
        A guard that neither denies nor speaks does not exist
        (ai/rules/fail-closed-guards.md)."""
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
        """AC-4: over-trigger bias, exactly as verdict_is_fresh:1231 records -- a false
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


class TestPre2119FailsClosed(unittest.TestCase):
    """R-4, the fail-open this spec exists to close: a keyword-driven check reports
    '0 sites, all classified' for 23 enrolled RFCs holding 172 gated MUSTs. A guard that
    cannot deny must SAY so (ai/rules/fail-closed-guards.md).

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
        not, and both would pass on an implementation with no guard at all."""
        with _extraction_tree(src={"rfc9999": _SRC_TWO_SITES}):
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
    Grandfathering is SCOPE, not an allowlist file (ai/rules/derive-not-hardcode.md)."""

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
        the unsigned backlog list. Lower kebab-case keys (ai/rules/json-format.md)."""
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
        """The table must reach ai/RFC-REQUIREMENTS.md, not just exist as a helper."""
        with _extraction_tree(src={}):
            body = R.render_ledger([_req("RFC7606-2-1")], [], {"rfc7606"})
        self.assertIn("Extraction sign-off", body)

    def test_stale_extraction_table_fails_check_fresh(self):
        """AC-15 through the EXISTING freshness gate (check_ledger_fresh:1578), so the
        published backlog cannot rot."""
        reqs = [_req("RFC7606-2-1")]
        tags = [_tag("RFC7606-2-1", "positive"), _tag("RFC7606-2-1", "negative")]
        path = _mkstemp(".md")
        orig = R.LEDGER_FILE
        try:
            with _extraction_tree(src={}):
                fresh = R.render_ledger(reqs, tags, {"rfc7606"}) + "\n"
                without = "\n".join(
                    ln for ln in fresh.split("\n") if "UNSIGNED" not in ln
                )
                with open(path, "w", encoding="utf-8") as fh:
                    fh.write(without)
                R.LEDGER_FILE = path
                errs = R.check_ledger_fresh(reqs, tags, {"rfc7606"})
            self.assertTrue(
                errs, "a ledger missing the extraction table must read stale"
            )
            self.assertIn("ze-rfc-index", errs[0])
        finally:
            R.LEDGER_FILE = orig
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
        """ai/rules/tdd.md boundary testing: last invalid, first valid, first beyond.

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
            _git_baseline_summary_stems=lambda: set(enrolled),
            scan_tree=lambda *a, **k: [
                _tag(r.rid, p) for r in reqs for p in ("positive", "negative")
            ],
            check_status_agreement=lambda *a, **k: [],
            check_audit_freshness=lambda *a, **k: [],
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


class TestSkeletonWriterWiring(unittest.TestCase):
    def test_extract_skeleton_dispatches_from_main(self):
        """AC-2: `make ze-rfc-extract STEM=x` reaches run_extract_skeleton."""
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
        """Generate, then re-read: what `make ze-rfc-extract STEM=<stem>` would write for
        every enrolled RFC must satisfy parse_extraction_artifact.

        Fixtures cannot see this. rfc2865, rfc2869, rfc1195 and sflow-v5 each carry a
        column-0 line the heading pattern reads as a heading, so each derived a duplicate
        section id and produced a skeleton that could not be re-read -- `ze-rfc-extract`
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
