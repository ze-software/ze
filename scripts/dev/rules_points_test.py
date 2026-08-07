#!/usr/bin/env python3
"""Tests for scripts/dev/rules_points.py.

The load-bearing test is `test_roundtrip_every_committed_rule`: it runs over the
real corpus, not a fixture, and it is the go/no-go for the whole design. The
partition tests beside it exist because a round trip alone is not evidence. A
splitter that drops a line and a renderer that puts it back would pass a diff.
"""

from __future__ import annotations

import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import rules_points  # noqa: E402

ROOT = Path(__file__).resolve().parents[2]
RULES_DIR = ROOT / "ai" / "rules"

# A fixture carrying every awkward shape the corpus contains: a table inside a
# fence, a bullet with indented continuation prose and a nested bullet, a
# blockquote with a bare marker, a `####` heading, a tab inside a fence, and a
# table row whose cell carries an escaped pipe and a trailing HTML comment.
#
# Two `##` sections, the second one capitalised and punctuated, so the manifest
# has to carry the heading VERBATIM rather than reconstruct it from a slug.
FIXTURE = """# Fixture Rule

**When:** exercising the splitter
**Severity:** blocking

## Directives

Connective prose that sits between directives, not above them.

- A bullet whose sentence wraps onto a second physical line and
  carries indented continuation prose.
  - A nested bullet under the same parent.
- A second bullet.

| Column | Meaning |
|--------|---------|
| `a\\|b` | an escaped pipe in a code span | <!-- trailing comment -->

> A blockquote.
>
> With a bare marker line above this one.

#### A deeper heading

```markdown
| This table | is inside a fence |
|------------|-------------------|
| and must   | never be split    |

\tA tab, and a blank line above, both inside the fence.
```

## A Sequel: With Punctuation, and Capitals

Final paragraph.
"""


def committed_rules():
    return rules_points.rule_files(RULES_DIR)


class SplitPartitionTest(unittest.TestCase):
    def test_split_partitions_every_line(self):
        """AC-1: every source line is in one point, in the header, or a blank gap.

        Asserted by SUMMATION over line indices, never by diffing the render.
        The ranges must be disjoint, must cover every non-blank line exactly
        once, and every index they leave uncovered must be a blank separator.
        """
        for path in committed_rules():
            with self.subTest(rule=path.name):
                source = path.read_text(encoding="utf-8")
                split = rules_points.split_rule(source, path.stem)
                lines = source.split("\n")
                self.assertEqual(lines[-1], "", "corpus files end with one newline")
                lines = lines[:-1]
                self.assertEqual(split.line_count, len(lines))

                owner = [None] * len(lines)
                ranges = [("header", split.header_start, split.header_end)]
                for section in split.sections:
                    # A `##` heading line is owned by its SECTION, which holds it
                    # in the manifest. Leave it out and this test would say the
                    # line belongs to nothing.
                    ranges.append(
                        (f"{section.slug}/", section.start, section.start + 1)
                    )
                    ranges += [(p.slug, p.start, p.end) for p in section.points]
                for name, start, end in ranges:
                    self.assertLess(start, end, f"{name} owns no line")
                    for i in range(start, end):
                        self.assertIsNone(
                            owner[i],
                            f"line {i + 1} claimed by {owner[i]} and {name}",
                        )
                        owner[i] = name

                for i, who in enumerate(owner):
                    if who is None:
                        self.assertEqual(
                            lines[i].strip(),
                            "",
                            f"{path.name}:{i + 1} is non-blank and owned by nothing",
                        )

                # Every point body is exactly the source lines it claims.
                for point in split.points:
                    self.assertEqual(point.body, lines[point.start : point.end])

    def test_roundtrip_every_committed_rule(self):
        """AC-2: all 27 rules round-trip byte-identical through files on disk."""
        with tempfile.TemporaryDirectory(prefix="ze-rules-points-test-") as tmp:
            failures = rules_points.roundtrip(RULES_DIR, Path(tmp))
        self.assertEqual(failures, [], "\n".join(failures))

    def test_roundtrip_covers_the_whole_corpus(self):
        """The go/no-go is over 27 rules. A narrowed corpus is not a pass."""
        self.assertEqual(len(committed_rules()), 27)

    def test_fenced_table_is_one_point(self):
        """A table inside a fence is one fence point, never split as a table."""
        split = rules_points.split_rule(FIXTURE, "fixture")
        fenced = [p for p in split.points if p.kind == "fence"]
        self.assertEqual(len(fenced), 1)
        body = "\n".join(fenced[0].body)
        self.assertIn("| This table | is inside a fence |", body)
        self.assertIn("\tA tab", body)
        self.assertTrue(body.startswith("```markdown"))
        self.assertTrue(body.endswith("```"))
        # No table point may carry a line that lives inside the fence.
        for point in split.points:
            if point.kind == "table":
                self.assertNotIn("is inside a fence", "\n".join(point.body))

    def test_nested_bullet_stays_with_parent(self):
        """Continuation prose and a nested bullet stay in the parent's point."""
        split = rules_points.split_rule(FIXTURE, "fixture")
        holders = [p for p in split.points if "A nested bullet" in "\n".join(p.body)]
        self.assertEqual(len(holders), 1)
        body = "\n".join(holders[0].body)
        self.assertIn("carries indented continuation prose.", body)
        self.assertIn("- A second bullet.", body)
        self.assertTrue(body.startswith("- A bullet whose sentence wraps"))

    def test_a_hash_hash_heading_becomes_a_section_and_deeper_ones_do_not(self):
        """The depth is FIXED at two: only `##` opens a section directory."""
        split = rules_points.split_rule(FIXTURE, "fixture")
        self.assertEqual(
            [s.slug for s in split.sections],
            ["directives", "a-sequel-with-punctuation-and-capitals"],
        )
        self.assertEqual(split.sections[0].heading, "## Directives")
        # The `####` heading stays a point INSIDE the section it sits in.
        deeper = [p for p in split.points if p.body[0].startswith("#### ")]
        self.assertEqual(len(deeper), 1)
        self.assertIn(deeper[0], split.sections[0].points)
        for ref in split.ids:
            self.assertEqual(len(ref.split("/")), 3, ref)

    def test_every_committed_point_id_has_three_components(self):
        """AC: `<rule>/<section>/<slug>`, over the real corpus and never a fixture."""
        for path in committed_rules():
            with self.subTest(rule=path.name):
                split = rules_points.split_rule(
                    path.read_text(encoding="utf-8"), path.stem
                )
                self.assertTrue(split.sections)
                for ref in split.ids:
                    self.assertEqual(len(ref.split("/")), 3, ref)

    def test_a_point_before_the_first_section_is_refused(self):
        """A point outside every section directory has no three-part id."""
        headless = FIXTURE.replace("## Directives\n\n", "")
        with self.assertRaises(rules_points.RulePointsError) as caught:
            rules_points.split_rule(headless, "fixture")
        self.assertIn("before the first `##` section", str(caught.exception))

    def test_a_section_heading_sharing_its_block_is_refused(self):
        """A heading names a directory, so it may not carry body lines with it."""
        glued = FIXTURE.replace(
            "## Directives\n\nConnective", "## Directives\nConnective"
        )
        with self.assertRaises(rules_points.RulePointsError) as caught:
            rules_points.split_rule(glued, "fixture")
        self.assertIn("must stand alone", str(caught.exception))


class RenderFailureTest(unittest.TestCase):
    """R-4: the manifest's silent-drop weakness is closed by a hard error."""

    def _write_fixture(self, tmp):
        split = rules_points.split_rule(FIXTURE, "fixture")
        rules_points.write_split(split, Path(tmp))
        return Path(tmp) / "fixture"

    def test_render_fails_on_unlisted_point(self):
        """AC-3: a point on disk that no manifest lists is an error, not a skip."""
        with tempfile.TemporaryDirectory() as tmp:
            rule_dir = self._write_fixture(tmp)
            rules_points.render_dir(rule_dir)  # green before the tampering
            (rule_dir / "directives" / "orphan-point.md").write_text(
                "---\nkind: note\nlevel:\nstage:\n---\nOrphan.\n", encoding="utf-8"
            )
            with self.assertRaises(rules_points.RulePointsError) as caught:
                rules_points.render_dir(rule_dir)
            self.assertIn("orphan-point", str(caught.exception))

    def test_render_fails_on_a_point_outside_every_section(self):
        """A point at the rule level has no three-part id, so it cannot be one.

        Without this the file would sit in the tree unlisted and unrendered, and
        the old flat layout is exactly the shape that produces it. Refusing here
        is what makes the depth-two invariant enforced rather than a convention.
        """
        with tempfile.TemporaryDirectory() as tmp:
            rule_dir = self._write_fixture(tmp)
            (rule_dir / "loose-point.md").write_text(
                "---\nkind: note\nlevel:\nstage:\n---\nLoose.\n", encoding="utf-8"
            )
            with self.assertRaises(rules_points.RulePointsError) as caught:
                rules_points.render_dir(rule_dir)
            self.assertIn("loose-point.md", str(caught.exception))
            self.assertIn("section directory", str(caught.exception))

    def test_render_fails_on_an_unlisted_section_directory(self):
        """A whole section nobody listed is the same silent drop, one level up."""
        with tempfile.TemporaryDirectory() as tmp:
            rule_dir = self._write_fixture(tmp)
            orphan = rule_dir / "orphan-section"
            orphan.mkdir()
            (orphan / "a-point.md").write_text(
                "---\nkind: note\nlevel:\nstage:\n---\nStranded.\n", encoding="utf-8"
            )
            with self.assertRaises(rules_points.RulePointsError) as caught:
                rules_points.render_dir(rule_dir)
            self.assertIn("orphan-section", str(caught.exception))

    def test_render_fails_on_a_listed_section_with_no_directory(self):
        with tempfile.TemporaryDirectory() as tmp:
            rule_dir = self._write_fixture(tmp)
            manifest = rule_dir / "manifest.md"
            manifest.write_text(
                manifest.read_text(encoding="utf-8") + "ghost ## Ghost\n  a-point\n",
                encoding="utf-8",
            )
            with self.assertRaises(rules_points.RulePointsError) as caught:
                rules_points.render_dir(rule_dir)
            self.assertIn("ghost", str(caught.exception))

    def test_render_fails_on_missing_and_duplicate_slug(self):
        """AC-4: a slug with no file, and a slug listed twice, both fail."""
        with tempfile.TemporaryDirectory() as tmp:
            rule_dir = self._write_fixture(tmp)
            manifest = rule_dir / "manifest.md"
            original = manifest.read_text(encoding="utf-8")

            manifest.write_text(original + "  no-such-point\n", encoding="utf-8")
            with self.assertRaises(rules_points.RulePointsError) as caught:
                rules_points.render_dir(rule_dir)
            self.assertIn("no-such-point", str(caught.exception))

            last = original.rstrip("\n").split("\n")[-1]
            manifest.write_text(original + last + "\n", encoding="utf-8")
            with self.assertRaises(rules_points.RulePointsError) as caught:
                rules_points.render_dir(rule_dir)
            self.assertIn("duplicate", str(caught.exception).lower())
            self.assertIn(last.strip(), str(caught.exception))

    def test_render_rejects_unsafe_slug(self):
        """Security: a manifest may not steer the renderer out of its directory."""
        with tempfile.TemporaryDirectory() as tmp:
            rule_dir = self._write_fixture(tmp)
            manifest = rule_dir / "manifest.md"
            original = manifest.read_text(encoding="utf-8")
            for bad in ("../escape", "sub/point", ".hidden", "..", "a/../../b"):
                manifest.write_text(f"{original}  {bad}\n", encoding="utf-8")
                with self.assertRaises(rules_points.RulePointsError) as caught:
                    rules_points.render_dir(rule_dir)
                self.assertIn("slug", str(caught.exception).lower())
                # A section directory names a path component too.
                manifest.write_text(f"{original}{bad} ## Bad\n  x\n", encoding="utf-8")
                with self.assertRaises(rules_points.RulePointsError) as caught:
                    rules_points.render_dir(rule_dir)
                self.assertIn("slug", str(caught.exception).lower())

    def test_render_rejects_a_manifest_line_of_neither_shape(self):
        """A body line the parser cannot classify is an error, never a skip."""
        with tempfile.TemporaryDirectory() as tmp:
            rule_dir = self._write_fixture(tmp)
            manifest = rule_dir / "manifest.md"
            original = manifest.read_text(encoding="utf-8")
            for bad in ("### Not a section", "no-indent-slug", "  ## indented heading"):
                manifest.write_text(original + bad + "\n", encoding="utf-8")
                with self.assertRaises(rules_points.RulePointsError):
                    rules_points.render_dir(rule_dir)


class SectionSpineTest(unittest.TestCase):
    """The manifest is the rule's structural spine, not only its reading order."""

    def test_the_manifest_records_the_heading_verbatim(self):
        """A slug drops capitals, punctuation and the marker. The heading keeps them."""
        split = rules_points.split_rule(FIXTURE, "fixture")
        manifest = rules_points.format_manifest(split)
        self.assertIn(
            "a-sequel-with-punctuation-and-capitals "
            "## A Sequel: With Punctuation, and Capitals",
            manifest,
        )
        fields, sections = rules_points.parse_manifest(manifest, "fixture")
        self.assertEqual(fields["title"], "Fixture Rule")
        self.assertEqual(
            [(slug, heading) for slug, heading, _ in sections],
            [
                ("directives", "## Directives"),
                (
                    "a-sequel-with-punctuation-and-capitals",
                    "## A Sequel: With Punctuation, and Capitals",
                ),
            ],
        )
        self.assertEqual(
            [len(points) for _, _, points in sections],
            [len(s.points) for s in split.sections],
        )

    def test_section_order_round_trips_and_is_the_manifest_s_to_change(self):
        """Reordering the manifest reorders the RULE, and nothing is renamed.

        The mutation proof for section order: the two trees on disk are
        byte-identical file for file, and only the manifest's section lines move.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            split = rules_points.split_rule(FIXTURE, "fixture")
            rules_points.write_split(split, root)
            rule_dir = root / "fixture"
            before = rules_points.render_dir(rule_dir)
            self.assertEqual(before, FIXTURE)

            manifest = rule_dir / "manifest.md"
            fields, sections = rules_points.parse_manifest(
                manifest.read_text(encoding="utf-8"), "fixture"
            )
            head = (
                f"---\ntitle: {fields['title']}\nwhen: {fields['when']}\n"
                f"severity: {fields['severity']}\n---\n"
            )
            body = ""
            for slug, heading, points in reversed(sections):
                body += f"{slug} {heading}\n"
                body += "".join(f"{rules_points.POINT_INDENT}{p}\n" for p in points)
            manifest.write_text(head + body, encoding="utf-8")

            after = rules_points.render_dir(rule_dir)
            self.assertNotEqual(after, before, "the order must be observable")
            self.assertLess(
                after.index("## A Sequel"),
                after.index("## Directives"),
                "the reordered manifest must reorder the rendered rule",
            )
            # Nothing on disk was renamed: reading order is the manifest's job.
            self.assertTrue((rule_dir / "directives").is_dir())
            self.assertTrue(
                (rule_dir / "a-sequel-with-punctuation-and-capitals").is_dir()
            )


class DelimiterTest(unittest.TestCase):
    def test_body_opening_with_delimiter(self):
        """AC-5: a body whose first line is a triple dash round-trips unchanged."""
        body = ["---", "A body that opens with the delimiter.", "--- still body"]
        point = rules_points.Point(
            slug="delimiter-body",
            kind="note",
            level="",
            stage="",
            body=body,
            start=0,
            end=len(body),
        )
        text = rules_points.format_point(point)
        parsed = rules_points.parse_point(text, "delimiter-body")
        self.assertEqual(parsed.body, body)
        self.assertEqual(parsed.kind, "note")

    def test_point_file_on_disk_opening_with_delimiter(self):
        """AC-5 through the filesystem, not only in memory."""
        with tempfile.TemporaryDirectory() as tmp:
            split = rules_points.split_rule(FIXTURE, "fixture")
            split.points[-1].body = ["---", "Body after a delimiter line."]
            rules_points.write_split(split, Path(tmp))
            rendered = rules_points.render_dir(Path(tmp) / "fixture")
            self.assertTrue(rendered.endswith("---\nBody after a delimiter line.\n"))


class HeaderTest(unittest.TestCase):
    def test_manifest_carries_title_and_metadata(self):
        """The manifest holds the rule's spine: rules_lint validates it rendered."""
        for path in committed_rules():
            with self.subTest(rule=path.name):
                split = rules_points.split_rule(
                    path.read_text(encoding="utf-8"), path.stem
                )
                self.assertTrue(split.header["title"])
                self.assertTrue(split.header["when"])
                self.assertIn(split.header["severity"], ("blocking", "advisory"))

    def test_every_point_declares_a_known_kind(self):
        for path in committed_rules():
            with self.subTest(rule=path.name):
                split = rules_points.split_rule(
                    path.read_text(encoding="utf-8"), path.stem
                )
                self.assertTrue(split.points)
                for point in split.points:
                    self.assertIn(point.kind, rules_points.KINDS)
                    self.assertIn(point.level, ("", *rules_points.LEVELS))
                    self.assertEqual(point.stage, "")
                    self.assertTrue(
                        rules_points.SLUG_SAFE.match(point.slug), point.slug
                    )
                for section in split.sections:
                    self.assertTrue(
                        rules_points.SLUG_SAFE.match(section.slug), section.slug
                    )
                    slugs = [p.slug for p in section.points]
                    self.assertEqual(
                        len(slugs), len(set(slugs)), "slugs are unique in a section"
                    )
                sections = [s.slug for s in split.sections]
                self.assertEqual(
                    len(sections), len(set(sections)), "section slugs must be unique"
                )
                # The id is the whole path, so it is what has to be unique.
                self.assertEqual(len(split.ids), len(set(split.ids)))


# A miniature corpus for the gate map: two instruction points and the two
# structural kinds, so the ungated denominator has something to exclude. Every id
# is `<rule>/<section>/<slug>`, which is what a binding comment has to name.
GATE_FIXTURE_POINTS = {
    "alpha/directives/first-directive": ("directive", "- Do the thing."),
    "alpha/directives/second-directive": ("directive", "- Do the other thing."),
    "alpha/directives/a-sub-heading": ("heading", "### A Sub Heading"),
    "alpha/directives/a-fence": ("fence", "```go\nfunc main() {}\n```"),
}


def write_points(tmp, points=None):
    """A miniature point tree: two directives, one heading, one fence."""
    points = GATE_FIXTURE_POINTS if points is None else points
    root = Path(tmp) / "points"
    body: dict[str, list[str]] = {}
    for ref, (kind, text) in points.items():
        _rule, section, slug = ref.split("/")
        section_dir = root / "alpha" / section
        section_dir.mkdir(parents=True, exist_ok=True)
        body.setdefault(section, []).append(slug)
        (section_dir / f"{slug}.md").write_text(
            f"---\nkind: {kind}\nlevel:\nstage:\n---\n{text}\n", encoding="utf-8"
        )
    lines = ""
    for section, slugs in body.items():
        lines += f"{section} ## {section.replace('-', ' ').title()}\n"
        lines += "".join(f"{rules_points.POINT_INDENT}{s}\n" for s in slugs)
    (root / "alpha" / "manifest.md").write_text(
        "---\ntitle: Alpha\nwhen: always\nseverity: blocking\n---\n" + lines,
        encoding="utf-8",
    )
    return root


class GateMapTest(unittest.TestCase):
    """AC-10, AC-11, AC-12: the three sets and the exit code each one earns."""

    POINTS = GATE_FIXTURE_POINTS

    def _points_dir(self, tmp):
        return write_points(tmp, self.POINTS)

    def _dispatcher(self, tmp, body, name="fake-hook.py"):
        path = Path(tmp) / name
        path.write_text(body, encoding="utf-8")
        return path

    def test_gate_map_sets_and_exits(self):
        """gated exits 0, ungated exits 0, dangling exits NON-ZERO."""
        with tempfile.TemporaryDirectory() as tmp:
            points = self._points_dir(tmp)

            # AC-10 + AC-12: one check names a point that exists. That point is
            # gated, the other directive is ungated, and the exit code is 0.
            good = self._dispatcher(
                tmp,
                "# ze point: alpha/directives/first-directive\ndef c_alpha(ctx):\n    return None\n",
            )
            gm = rules_points.gate_map([good], points)
            self.assertEqual(sorted(gm.gated), ["alpha/directives/first-directive"])
            self.assertEqual(gm.ungated, ["alpha/directives/second-directive"])
            self.assertEqual(gm.dangling, [])
            _, code = rules_points.report_gate_map(gm)
            self.assertEqual(code, 0, "gated and ungated are measurements, never a red")

            # AC-12: the ungated DENOMINATOR excludes the structural kinds. The
            # heading and the fence are in `points` and in neither set.
            self.assertEqual(len(gm.points), 4)
            self.assertEqual(sorted(gm.candidates), sorted(self.POINTS)[2:])
            self.assertNotIn("alpha/directives/a-sub-heading", gm.ungated)
            self.assertNotIn("alpha/directives/a-fence", gm.ungated)

            # AC-11: a check naming a point that does not exist FAILS. This is
            # the discrimination proof: the only difference from the run above is
            # the slug, and the exit code must move with it.
            bad = self._dispatcher(
                tmp,
                "# ze point: alpha/directives/no-such-point\ndef c_alpha(ctx):\n    return None\n",
                name="bad-hook.py",
            )
            gm_bad = rules_points.gate_map([bad], points)
            self.assertEqual(
                [b.ref for b in gm_bad.dangling], ["alpha/directives/no-such-point"]
            )
            self.assertEqual(gm_bad.gated, {})
            lines, code = rules_points.report_gate_map(gm_bad)
            self.assertNotEqual(code, 0, "a dangling binding must fail")
            self.assertIn("alpha/directives/no-such-point", "\n".join(lines))

    def test_gate_map_reports_a_binding_that_gates_nothing(self):
        """A binding separated from its check by code names no running check."""
        with tempfile.TemporaryDirectory() as tmp:
            points = self._points_dir(tmp)
            orphan = self._dispatcher(
                tmp,
                "# ze point: alpha/directives/first-directive\nX = 1\n\ndef c_alpha(ctx):\n"
                "    return None\n",
            )
            gm = rules_points.gate_map([orphan], points)
            self.assertEqual(gm.gated, {})
            self.assertEqual(len(gm.dangling), 1)
            self.assertEqual(gm.dangling[0].check, rules_points.NO_CHECK)
            _, code = rules_points.report_gate_map(gm)
            self.assertNotEqual(code, 0)

    def test_gate_map_declared_none_needs_a_reason(self):
        """`none -- <why>` is unbound; a bare `none` is dangling, not a pass."""
        with tempfile.TemporaryDirectory() as tmp:
            points = self._points_dir(tmp)
            declared = self._dispatcher(
                tmp,
                "# ze point: none -- nothing states this\ndef c_a(ctx):\n    pass\n",
            )
            gm = rules_points.gate_map([declared], points)
            self.assertEqual([b.check for b in gm.unbound], ["c_a"])
            self.assertEqual(gm.unbound[0].reason, "nothing states this")
            self.assertEqual(gm.dangling, [])
            self.assertEqual(rules_points.report_gate_map(gm)[1], 0)

            bare = self._dispatcher(
                tmp, "# ze point: none\ndef c_a(ctx):\n    pass\n", name="bare.py"
            )
            gm_bare = rules_points.gate_map([bare], points)
            self.assertEqual([b.ref for b in gm_bare.dangling], ["none"])
            self.assertNotEqual(rules_points.report_gate_map(gm_bare)[1], 0)

            empty = self._dispatcher(
                tmp, "# ze point:\ndef c_a(ctx):\n    pass\n", name="empty.py"
            )
            gm_empty = rules_points.gate_map([empty], points)
            self.assertEqual(
                [b.ref for b in gm_empty.dangling], [rules_points.EMPTY_REF]
            )
            self.assertNotEqual(rules_points.report_gate_map(gm_empty)[1], 0)

    def test_gate_map_empty_result_is_never_success(self):
        """No points, or no bindings, is a failure -- not a vacuous green."""
        with tempfile.TemporaryDirectory() as tmp:
            points = self._points_dir(tmp)
            silent = self._dispatcher(tmp, "def c_alpha(ctx):\n    return None\n")
            gm = rules_points.gate_map([silent], points)
            self.assertEqual(gm.bindings, [])
            lines, code = rules_points.report_gate_map(gm)
            self.assertNotEqual(code, 0)
            self.assertIn("must not report success", "\n".join(lines))

            bound = self._dispatcher(
                tmp,
                "# ze point: alpha/directives/first-directive\ndef c_alpha(ctx):\n    return None\n",
                name="bound.py",
            )
            empty_points = Path(tmp) / "no-points"
            empty_points.mkdir()
            gm_none = rules_points.gate_map([bound], empty_points)
            self.assertEqual(gm_none.points, {})
            lines, code = rules_points.report_gate_map(gm_none)
            self.assertNotEqual(code, 0)
            self.assertIn("must not report success", "\n".join(lines))

    # The published Hook-to-Rule Mapping. One dispatcher, one check, one row.
    HOOK_DOC = """## Hook-to-Rule Mapping

#### Fake (`fake-hook.py`)

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `c_alpha` | `alpha.md` | Write | Blocks a thing. BLOCKING. |

Prose after the table, which ends it.
"""
    HOOK_SRC = "# ze point: alpha/directives/first-directive\ndef c_alpha(ctx):\n    return None\n"

    def _hook_problems(self, tmp, doc=None, src=None):
        points = Path(tmp) / "points"
        if not points.is_dir():
            self._points_dir(tmp)
        source = self.HOOK_SRC if src is None else src
        gm = rules_points.gate_map([self._dispatcher(tmp, source)], points)
        return rules_points.hook_table_problems(
            gm, self.HOOK_DOC if doc is None else doc, {"fake-hook.py": source}
        )

    def test_hook_table_agrees_with_the_bindings(self):
        """A row per check, and an Enforces cell naming what the binding names."""
        with tempfile.TemporaryDirectory() as tmp:
            self.assertEqual(self._hook_problems(tmp), [])

    def test_hook_table_flags_a_row_naming_no_check(self):
        """A row for a function deleted from the tree is the drift this catches."""
        with tempfile.TemporaryDirectory() as tmp:
            doc = self.HOOK_DOC.replace("| `c_alpha` |", "| `c_removed` |")
            problems = self._hook_problems(tmp, doc=doc)
            self.assertEqual(len(problems), 2, problems)
            self.assertIn("row `c_removed` names no check", problems[0])
            self.assertIn("`c_alpha` has no row", problems[1])

    def test_hook_table_flags_a_check_with_no_row(self):
        """A check nobody documented must not pass by being absent."""
        with tempfile.TemporaryDirectory() as tmp:
            src = (
                self.HOOK_SRC
                + "\n\n# ze point: none -- nothing\ndef c_beta(ctx):\n    pass\n"
            )
            problems = self._hook_problems(tmp, src=src)
            self.assertEqual(
                problems,
                ["fake-hook.py: `c_beta` has no row in the Hook-to-Rule Mapping table"],
            )

    def test_hook_table_flags_a_drifted_enforces_cell(self):
        """The Enforces cell states the rule; the binding states the point."""
        with tempfile.TemporaryDirectory() as tmp:
            doc = self.HOOK_DOC.replace("`alpha.md`", "`testing.md`")
            problems = self._hook_problems(tmp, doc=doc)
            self.assertEqual(len(problems), 1, problems)
            self.assertIn("its bindings say `alpha.md`", problems[0])

            # A check that binds NO point may not claim a rule either.
            src = "# ze point: none -- nothing\ndef c_alpha(ctx):\n    pass\n"
            problems = self._hook_problems(tmp, src=src)
            self.assertEqual(len(problems), 1, problems)
            self.assertIn("its bindings say no rule", problems[0])

            # A rule OUTSIDE this corpus is free text, not a claim about a point.
            doc = self.HOOK_DOC.replace("`alpha.md`", "`.claude/rules/alpha.md`")
            self.assertEqual(len(self._hook_problems(tmp, doc=doc, src=src)), 0)

    def test_hook_table_missing_or_empty_is_never_success(self):
        """No table, a duplicate row, and a table under no heading all fail."""
        with tempfile.TemporaryDirectory() as tmp:
            problems = self._hook_problems(tmp, doc="## Hook-to-Rule Mapping\n")
            self.assertEqual(len(problems), 1, problems)
            self.assertIn("check(s) are published nowhere", problems[0])

            # The table must sit under a heading that NAMES its dispatcher.
            orphan = self.HOOK_DOC.replace("#### Fake (`fake-hook.py`)", "#### Fake")
            self.assertIn("published nowhere", self._hook_problems(tmp, doc=orphan)[0])

            row = "| `c_alpha` | `alpha.md` | Write | Blocks a thing. BLOCKING. |"
            twice = self.HOOK_DOC.replace(row, row + "\n" + row)
            problems = self._hook_problems(tmp, doc=twice)
            self.assertEqual(len(problems), 1, problems)
            self.assertIn("has a second row", problems[0])

    def test_hook_table_over_the_real_tree(self):
        """The committed mapping table agrees with every committed binding."""
        gate_files, problems = rules_points.dispatchers(ROOT)
        self.assertEqual(problems, [])
        gm = rules_points.gate_map(gate_files, RULES_DIR / "points")
        doc = (RULES_DIR / f"{rules_points.DOC_RULE}.md").read_text(encoding="utf-8")
        sources = {p.name: p.read_text(encoding="utf-8") for p in gate_files}
        self.assertEqual(rules_points.hook_table_problems(gm, doc, sources), [])

    def test_gate_map_over_the_real_dispatchers(self):
        """The committed tree: every binding resolves, and the map is not empty."""
        gate_files, problems = rules_points.dispatchers(ROOT)
        self.assertEqual(problems, [])
        for path in gate_files:
            self.assertTrue(path.is_file(), path)
        gm = rules_points.gate_map(gate_files, RULES_DIR / "points")
        self.assertEqual(
            [f"{b.file}: {b.check} -> {b.ref}" for b in gm.dangling],
            [],
            "a check names a point that does not exist",
        )
        self.assertTrue(gm.gated, "the real tree must carry bindings")
        self.assertEqual(rules_points.report_gate_map(gm)[1], 0)
        # Every check in the three dispatchers answers the question: it names a
        # point, or it declares `none` with a reason. A check in neither set is
        # one nobody decided about, which is what this binding pass removes.
        answered = {b.check for b in gm.gated_bindings} | {b.check for b in gm.unbound}
        self.assertEqual(sorted(_dispatcher_checks() - answered), [])


class RationaleTest(unittest.TestCase):
    """AC-15, AC-16, AC-17: the `rationale` header field and what it gates.

    A point states an instruction. Nothing in the split says WHY the instruction
    exists, and the record that does say so lives in `plan/learned/` or
    `ai/rationale/`. `rationale` is the link, and it is a HEADER field so the
    rendered rule cannot change when one is added.
    """

    HEAD = "---\nkind: directive\nlevel:\nstage:\n"
    BODY = "- Do the thing.\n"

    def _point(self, rationale=None):
        line = f"rationale: {rationale}\n" if rationale is not None else ""
        return f"{self.HEAD}{line}---\n{self.BODY}"

    def test_a_point_parses_with_and_without_a_rationale(self):
        """AC-15: the field is optional, and it round-trips through the file."""
        without = rules_points.parse_point(self._point(), "p")
        self.assertEqual(without.rationale, "")
        self.assertEqual(rules_points.format_point(without), self._point())

        with_link = rules_points.parse_point(
            self._point("plan/learned/METHODOLOGY.md"), "p"
        )
        self.assertEqual(with_link.rationale, "plan/learned/METHODOLOGY.md")
        self.assertEqual(
            rules_points.format_point(with_link),
            self._point("plan/learned/METHODOLOGY.md"),
        )
        # An empty `rationale:` is not written back. The split cannot derive the
        # field, so an empty line would claim the point was examined.
        blank = rules_points.parse_point(self._point(""), "p")
        self.assertEqual(blank.rationale, "")
        self.assertEqual(rules_points.format_point(blank), self._point())

    def test_a_rationale_never_reaches_the_rendered_rule(self):
        """AC-15: two trees differing only in the field render the same bytes."""
        with tempfile.TemporaryDirectory() as tmp:
            manifest = (
                "---\ntitle: Alpha\nwhen: always\nseverity: blocking\n---\n"
                "directives ## Directives\n  only\n"
            )
            rendered = []
            for name, text in (("bare", self._point()), ("linked", self._point("ai"))):
                rule_dir = Path(tmp) / name / "alpha"
                (rule_dir / "directives").mkdir(parents=True)
                (rule_dir / "manifest.md").write_text(manifest, encoding="utf-8")
                (rule_dir / "directives" / "only.md").write_text(text, encoding="utf-8")
                rendered.append(rules_points.render_dir(rule_dir))
            self.assertEqual(rendered[0], rendered[1])
            self.assertNotIn("rationale", rendered[0])

    def test_an_unknown_header_field_is_still_refused(self):
        """The key set stays closed: `rationale` is a fourth key, not free text."""
        with self.assertRaises(rules_points.RulePointsError) as err:
            rules_points.parse_point(
                f"{self.HEAD}reason: because\n---\n{self.BODY}", "p"
            )
        self.assertIn("unknown header field", str(err.exception))

    def _tree(self, tmp, rationales):
        """A points tree of len(rationales) directives, one rationale each."""
        root = Path(tmp) / "points"
        rule_dir = root / "alpha"
        section_dir = rule_dir / "directives"
        section_dir.mkdir(parents=True)
        slugs = []
        for i, rationale in enumerate(rationales):
            slug = f"point-{i}"
            slugs.append(slug)
            (section_dir / f"{slug}.md").write_text(
                self._point(rationale), encoding="utf-8"
            )
        (rule_dir / "manifest.md").write_text(
            "---\ntitle: Alpha\nwhen: always\nseverity: blocking\n---\n"
            "directives ## Directives\n"
            + "".join(f"{rules_points.POINT_INDENT}{s}\n" for s in slugs),
            encoding="utf-8",
        )
        (Path(tmp) / "hook.py").write_text(
            "# ze point: alpha/directives/point-0\ndef c_alpha(ctx):\n    return None\n",
            encoding="utf-8",
        )
        (Path(tmp) / "real.md").write_text("a record\n", encoding="utf-8")
        return root, [Path(tmp) / "hook.py"]

    def test_a_rationale_naming_no_file_fails_the_gate_map(self):
        """AC-16, and the mutation proof: only the path differs between the two.

        The tree, the binding and the point count are identical in both runs.
        The exit code moves with the path, so the check cannot be satisfied by
        anything else in the fixture.
        """
        with tempfile.TemporaryDirectory() as tmp:
            points, gates = self._tree(tmp, ["real.md", None])
            good = rules_points.gate_map(gates, points, Path(tmp))
            self.assertEqual(good.missing_rationale, [])
            self.assertEqual(good.rationales, {"alpha/directives/point-0": "real.md"})
            self.assertEqual(rules_points.report_gate_map(good)[1], 0)

            (points / "alpha" / "directives" / "point-0.md").write_text(
                self._point("gone.md"), encoding="utf-8"
            )
            bad = rules_points.gate_map(gates, points, Path(tmp))
            self.assertEqual(
                bad.missing_rationale, [("alpha/directives/point-0", "gone.md")]
            )
            lines, code = rules_points.report_gate_map(bad)
            self.assertNotEqual(code, 0, "a rationale naming no file must fail")
            self.assertIn("alpha/directives/point-0 -> gone.md", "\n".join(lines))
            # It fails on its own, not by borrowing another set's red.
            self.assertEqual(bad.dangling, [])

    def test_a_rationale_escaping_the_repository_is_reported(self):
        """A path that resolves outside the tree names no record this gate can
        re-check, so it is reported rather than resolved (Security Review)."""
        with tempfile.TemporaryDirectory() as tmp:
            outside = Path(tmp) / "outside.md"
            outside.write_text("not in the tree\n", encoding="utf-8")
            inner = Path(tmp) / "repo"
            inner.mkdir()
            points, gates = self._tree(str(inner), ["../outside.md", None])
            gm = rules_points.gate_map(gates, points, inner)
            self.assertEqual(
                gm.missing_rationale, [("alpha/directives/point-0", "../outside.md")]
            )
            self.assertNotEqual(rules_points.report_gate_map(gm)[1], 0)

    def test_rationale_coverage_is_a_measurement(self):
        """AC-17: the count is reported and never reds, whatever it is."""
        with tempfile.TemporaryDirectory() as tmp:
            points, gates = self._tree(tmp, ["real.md", None, None])
            gm = rules_points.gate_map(gates, points, Path(tmp))
            lines, code = rules_points.report_gate_map(gm)
            self.assertEqual(code, 0, "coverage is a measurement, never a red")
            self.assertIn("RATIONALE: 1 of 3 instruction points", "\n".join(lines))

            bare, gates = self._tree(str(Path(tmp) / "b"), [None, None])
            gm_bare = rules_points.gate_map(gates, bare, Path(tmp) / "b")
            lines, code = rules_points.report_gate_map(gm_bare)
            self.assertEqual(code, 0, "zero rationales is a measurement too")
            self.assertIn("RATIONALE: 0 of 2 instruction points", "\n".join(lines))

    def test_committed_rationales_resolve(self):
        """The real corpus: the field is in use, and every link names a file."""
        gate_files, problems = rules_points.dispatchers(ROOT)
        self.assertEqual(problems, [])
        gm = rules_points.gate_map(gate_files, RULES_DIR / "points", ROOT)
        self.assertEqual(gm.missing_rationale, [])
        self.assertTrue(
            gm.rationales, "no committed point carries a rationale; the field is dead"
        )


class ExceptedByTest(unittest.TestCase):
    """AC-20, AC-21: the `excepted-by` header field and what it gates.

    A general instruction must carry its own exception, or a reader who stops
    after the general statement is misled. `ai/rules/writing.md` does that by
    hand-repeating the UK English exception at three levels, and that repetition
    is load-bearing while being invisible to every gate: a dedup pass can delete
    the exception point and nothing goes red. `excepted-by` is declared on the
    GENERAL point and names the exception, so the deletion leaves a dangling ref
    and the gate map fails.
    """

    HEAD = "---\nkind: directive\nlevel:\nstage:\n"
    BODY = "- Do the thing.\n"

    def _point(self, excepted_by=None, rationale=None):
        rat = f"rationale: {rationale}\n" if rationale is not None else ""
        exc = f"excepted-by: {excepted_by}\n" if excepted_by is not None else ""
        return f"{self.HEAD}{rat}{exc}---\n{self.BODY}"

    def test_a_point_parses_with_and_without_an_excepted_by(self):
        """AC-20: the field is optional, and it round-trips through the file."""
        without = rules_points.parse_point(self._point(), "p")
        self.assertEqual(without.excepted_by, "")
        self.assertEqual(rules_points.format_point(without), self._point())

        one = "alpha/directives/carve-out"
        linked = rules_points.parse_point(self._point(one), "p")
        self.assertEqual(linked.excepted_by, one)
        self.assertEqual(rules_points.format_point(linked), self._point(one))

        # An empty `excepted-by:` is not written back, for the reason an empty
        # `rationale:` is not: the split cannot derive the field, so the line
        # would claim the point was examined and states no exception.
        blank = rules_points.parse_point(self._point(""), "p")
        self.assertEqual(blank.excepted_by, "")
        self.assertEqual(rules_points.format_point(blank), self._point())

    def test_both_optional_keys_coexist(self):
        """`rationale` and `excepted-by` are independent, and both round-trip."""
        text = self._point("a/b/c", "plan/learned/METHODOLOGY.md")
        point = rules_points.parse_point(text, "p")
        self.assertEqual(point.rationale, "plan/learned/METHODOLOGY.md")
        self.assertEqual(point.excepted_by, "a/b/c")
        self.assertEqual(rules_points.format_point(point), text)

    def test_several_exceptions_are_comma_separated(self):
        """One general instruction can have several exceptions."""
        self.assertEqual(
            rules_points.exception_refs("a/b/c, d/e/f"), ["a/b/c", "d/e/f"]
        )
        # Whitespace around a separator is authoring noise, not part of the id.
        self.assertEqual(
            rules_points.exception_refs(" a/b/c ,d/e/f"), ["a/b/c", "d/e/f"]
        )
        # A trailing comma is a typo in the separator, not a claim about a point.
        self.assertEqual(rules_points.exception_refs("a/b/c,"), ["a/b/c"])
        self.assertEqual(rules_points.exception_refs(" , "), [])

    def test_an_excepted_by_never_reaches_the_rendered_rule(self):
        """AC-20: two trees differing only in the field render the same bytes."""
        with tempfile.TemporaryDirectory() as tmp:
            manifest = (
                "---\ntitle: Alpha\nwhen: always\nseverity: blocking\n---\n"
                "directives ## Directives\n  only\n"
            )
            rendered = []
            for name, text in (
                ("bare", self._point()),
                ("linked", self._point("x/y/z")),
            ):
                rule_dir = Path(tmp) / name / "alpha"
                (rule_dir / "directives").mkdir(parents=True)
                (rule_dir / "manifest.md").write_text(manifest, encoding="utf-8")
                (rule_dir / "directives" / "only.md").write_text(text, encoding="utf-8")
                rendered.append(rules_points.render_dir(rule_dir))
            self.assertEqual(rendered[0], rendered[1])
            self.assertNotIn("excepted-by", rendered[0])

    def _tree(self, tmp, general_links_to):
        """A rule of two points: `general`, and `exception` it may name."""
        root = Path(tmp) / "points"
        rule_dir = root / "alpha"
        section_dir = rule_dir / "directives"
        section_dir.mkdir(parents=True)
        (section_dir / "general.md").write_text(
            self._point(general_links_to), encoding="utf-8"
        )
        (section_dir / "exception.md").write_text(self._point(), encoding="utf-8")
        (rule_dir / "manifest.md").write_text(
            "---\ntitle: Alpha\nwhen: always\nseverity: blocking\n---\n"
            "directives ## Directives\n"
            f"{rules_points.POINT_INDENT}general\n"
            f"{rules_points.POINT_INDENT}exception\n",
            encoding="utf-8",
        )
        (Path(tmp) / "hook.py").write_text(
            "# ze point: alpha/directives/general\ndef c_alpha(ctx):\n    return None\n",
            encoding="utf-8",
        )
        return root, [Path(tmp) / "hook.py"]

    def test_deleting_the_exception_point_reds_the_gate(self):
        """The protection this key buys, proven by the deletion it must catch.

        This is the mutation: the general point and the binding are byte for
        byte the same in both runs, and only the exception point's existence
        differs. It is the move `ai/rules/writing.md` was one warning away from
        during a dedup pass, and every gate stayed green then.
        """
        with tempfile.TemporaryDirectory() as tmp:
            points, gates = self._tree(tmp, "alpha/directives/exception")
            good = rules_points.gate_map(gates, points, Path(tmp))
            self.assertEqual(good.missing_excepted_by, [])
            self.assertEqual(
                good.excepted,
                {"alpha/directives/general": ["alpha/directives/exception"]},
            )
            self.assertEqual(rules_points.report_gate_map(good)[1], 0)

            # The dedup pass: the exception leaves the corpus, manifest and all.
            (points / "alpha" / "directives" / "exception.md").unlink()
            (points / "alpha" / "manifest.md").write_text(
                "---\ntitle: Alpha\nwhen: always\nseverity: blocking\n---\n"
                "directives ## Directives\n"
                f"{rules_points.POINT_INDENT}general\n",
                encoding="utf-8",
            )
            bad = rules_points.gate_map(gates, points, Path(tmp))
            self.assertEqual(
                bad.missing_excepted_by,
                [("alpha/directives/general", "alpha/directives/exception")],
            )
            lines, code = rules_points.report_gate_map(bad)
            self.assertNotEqual(
                code, 0, "deleting an exception point must red the gate"
            )
            self.assertIn(
                "alpha/directives/general -> alpha/directives/exception (no such point)",
                "\n".join(lines),
            )
            # It fails on its own, not by borrowing another set's red.
            self.assertEqual(bad.dangling, [])
            self.assertEqual(bad.missing_rationale, [])

    def test_a_point_cannot_except_itself(self):
        """A self-reference resolves as a point, so it needs its own refusal."""
        with tempfile.TemporaryDirectory() as tmp:
            points, gates = self._tree(tmp, "alpha/directives/general")
            gm = rules_points.gate_map(gates, points, Path(tmp))
            self.assertEqual(
                gm.missing_excepted_by,
                [("alpha/directives/general", "alpha/directives/general")],
            )
            lines, code = rules_points.report_gate_map(gm)
            self.assertNotEqual(code, 0)
            self.assertIn("a point cannot except itself", "\n".join(lines))

    def test_a_separator_only_value_is_reported(self):
        """`excepted-by: ,` names nothing while looking declared."""
        with tempfile.TemporaryDirectory() as tmp:
            points, gates = self._tree(tmp, ",")
            gm = rules_points.gate_map(gates, points, Path(tmp))
            self.assertEqual(gm.excepted, {})
            self.assertEqual(
                gm.missing_excepted_by, [("alpha/directives/general", ",")]
            )
            self.assertNotEqual(rules_points.report_gate_map(gm)[1], 0)

    def test_excepted_by_coverage_is_a_measurement(self):
        """AC-21: the count is reported and never reds, whatever it is."""
        with tempfile.TemporaryDirectory() as tmp:
            points, gates = self._tree(tmp, "alpha/directives/exception")
            lines, code = rules_points.report_gate_map(
                rules_points.gate_map(gates, points, Path(tmp))
            )
            self.assertEqual(code, 0, "coverage is a measurement, never a red")
            self.assertIn(
                "EXCEPTED: 1 of 2 instruction points name an exception, "
                "naming 1 point(s)",
                "\n".join(lines),
            )

            bare, gates = self._tree(str(Path(tmp) / "b"), None)
            lines, code = rules_points.report_gate_map(
                rules_points.gate_map(gates, bare, Path(tmp) / "b")
            )
            self.assertEqual(code, 0, "zero exceptions is a measurement too")
            self.assertIn("EXCEPTED: 0 of 2 instruction points", "\n".join(lines))

    def test_committed_exceptions_resolve(self):
        """The real corpus: the field is in use, and every link names a point."""
        gate_files, problems = rules_points.dispatchers(ROOT)
        self.assertEqual(problems, [])
        gm = rules_points.gate_map(gate_files, RULES_DIR / "points", ROOT)
        self.assertEqual(gm.missing_excepted_by, [])
        self.assertTrue(
            gm.excepted, "no committed point names an exception; the field is dead"
        )
        # The verified case: the US English rule and the UK English exception.
        self.assertIn(
            "writing/language-and-spelling/prose-written-in-thomas-s-voice-"
            "keeps-uk-british-english",
            gm.excepted[
                "writing/directives/write-us-english-in-ste-and-document-every-change"
            ],
        )

    def test_the_committed_corpus_still_renders_with_the_links(self):
        """AC-20 over the real tree: a header field cannot move a rendered byte."""
        self.assertEqual(
            rules_points.render_all(RULES_DIR, RULES_DIR / "points", check=True), []
        )


class VerifyPartitionTest(unittest.TestCase):
    """`_verify_partition` asserts the property; the round trip sees its effect.

    Driven directly, because no corpus file reaches either branch: a splitter
    that produced an overlap or a hole would have to be broken first. Without
    these two fixtures the whole function can be replaced by `pass` and every
    other test stays green.
    """

    LINES = [
        "# T",
        "",
        "**When:** x",
        "**Severity:** blocking",
        "",
        "## S",
        "",
        "- A.",
        "",
        "- B.",
    ]

    def _split(self, ranges, section_at=5):
        """A split whose one section claims `section_at` and holds `ranges`."""
        sections = []
        if section_at is not None:
            sections.append(
                rules_points.Section(slug="s", heading="## S", start=section_at)
            )
            sections[0].points = [
                rules_points.Point(
                    slug=slug,
                    kind="directive",
                    level="",
                    stage="",
                    body=self.LINES[start:end],
                    start=start,
                    end=end,
                )
                for slug, start, end in ranges
            ]
        return rules_points.Split(
            stem="fixture",
            header={"title": "T", "when": "x", "severity": "blocking"},
            header_start=0,
            header_end=4,
            sections=sections,
            line_count=len(self.LINES),
        )

    def test_two_points_claiming_one_line_raise(self):
        split = self._split([("first", 7, 8), ("second", 7, 8)])
        with self.assertRaises(rules_points.RulePointsError) as caught:
            rules_points._verify_partition(self.LINES, split)
        self.assertIn("claimed by first and second", str(caught.exception))

    def test_a_non_blank_line_owned_by_nothing_raises(self):
        split = self._split([("first", 7, 8)])
        with self.assertRaises(rules_points.RulePointsError) as caught:
            rules_points._verify_partition(self.LINES, split)
        self.assertIn(
            "line 10 is non-blank and belongs to no point", str(caught.exception)
        )

    def test_a_section_heading_owned_by_no_section_raises(self):
        """The heading line lives in the manifest, so a section must claim it."""
        split = rules_points.Split(
            stem="fixture",
            header={"title": "T", "when": "x", "severity": "blocking"},
            header_start=0,
            header_end=4,
            sections=[],
            line_count=len(self.LINES),
        )
        with self.assertRaises(rules_points.RulePointsError) as caught:
            rules_points._verify_partition(self.LINES, split)
        self.assertIn("line 6 is non-blank", str(caught.exception))

    def test_a_total_partition_is_accepted(self):
        """The discrimination control: the same shape, with nothing missing."""
        split = self._split([("first", 7, 8), ("second", 9, 10)])
        rules_points._verify_partition(self.LINES, split)


class RenderAllTest(unittest.TestCase):
    """`render_all` is the producer behind four gates and named by no test.

    Mutation this fixes: changing its `if check:` to `if False:` made `--check`
    REWRITE the tree and report no failures, so `ze-rules-render-check`,
    `ze-doc-test` and `ze-regen-check-readonly` all exited 0 while silently
    overwriting rules. Every assertion below moves under that one edit.
    """

    def _rule(self, stem):
        return (
            f"# {stem.title()}\n\n**When:** always\n**Severity:** blocking\n\n"
            f"## Directives\n\n- Do the {stem} thing.\n"
        )

    def _tree(self, tmp, stems=("alpha", "beta")):
        rules = Path(tmp) / "rules"
        points = rules / "points"
        points.mkdir(parents=True)
        for stem in stems:
            text = self._rule(stem)
            (rules / f"{stem}.md").write_text(text, encoding="utf-8")
            rules_points.write_split(rules_points.split_rule(text, stem), points)
        return rules, points

    def test_check_is_clean_when_the_rendered_rules_match(self):
        with tempfile.TemporaryDirectory() as tmp:
            rules, points = self._tree(tmp)
            self.assertEqual(rules_points.render_all(rules, points, check=True), [])

    def test_check_never_writes_and_names_every_drifted_rule(self):
        """`--check` compares. It must report BOTH drifts and change no byte."""
        with tempfile.TemporaryDirectory() as tmp:
            rules, points = self._tree(tmp)
            for stem in ("alpha", "beta"):
                path = rules / f"{stem}.md"
                path.write_text(
                    path.read_text(encoding="utf-8") + "\n- Hand-edited.\n",
                    encoding="utf-8",
                )
            before = {p.name: p.read_text(encoding="utf-8") for p in rules.glob("*.md")}

            failures = rules_points.render_all(rules, points, check=True)
            self.assertEqual(len(failures), 2, failures)
            self.assertTrue(any("alpha.md is stale" in f for f in failures), failures)
            self.assertTrue(any("beta.md is stale" in f for f in failures), failures)

            after = {p.name: p.read_text(encoding="utf-8") for p in rules.glob("*.md")}
            self.assertEqual(after, before, "--check must not write")

    def test_render_writes_and_then_check_is_clean(self):
        with tempfile.TemporaryDirectory() as tmp:
            rules, points = self._tree(tmp)
            (rules / "alpha.md").write_text("# Stale\n", encoding="utf-8")
            self.assertEqual(rules_points.render_all(rules, points, check=False), [])
            self.assertEqual(
                (rules / "alpha.md").read_text(encoding="utf-8"), self._rule("alpha")
            )
            self.assertEqual(rules_points.render_all(rules, points, check=True), [])

    def test_an_empty_points_tree_is_never_a_vacuous_pass(self):
        with tempfile.TemporaryDirectory() as tmp:
            rules = Path(tmp) / "rules"
            rules.mkdir()
            failures = rules_points.render_all(rules, rules / "points", check=True)
            self.assertEqual(len(failures), 1, failures)
            self.assertIn("must not report success", failures[0])

    def test_a_rule_with_no_point_directory_is_a_failure(self):
        with tempfile.TemporaryDirectory() as tmp:
            rules, points = self._tree(tmp, stems=("alpha",))
            (rules / "orphan.md").write_text(self._rule("orphan"), encoding="utf-8")
            failures = rules_points.render_all(rules, points, check=True)
            self.assertEqual(len(failures), 1, failures)
            self.assertIn("orphan.md: no point directory", failures[0])

    def test_a_point_directory_named_for_a_generated_artifact_is_a_failure(self):
        """`ai/rules/points/CORE/` would render OVER a file rules_condensed owns."""
        with tempfile.TemporaryDirectory() as tmp:
            rules, points = self._tree(tmp, stems=("alpha",))
            rules_points.write_split(
                rules_points.split_rule(self._rule("alpha"), "CORE"), points
            )
            failures = rules_points.render_all(rules, points, check=False)
            self.assertTrue(
                any("may not be named for a generated artifact" in f for f in failures),
                failures,
            )
            self.assertFalse(
                (rules / "CORE.md").exists(), "CORE.md must not be written"
            )


class DispatcherRosterTest(unittest.TestCase):
    """Which files the gate map reads is derived, and pinned against the tree.

    Mutation this fixes: dropping `.claude/hooks/pretool-bash.py` from a
    hand-typed 3-tuple retired seven checks from both the binding join and the
    published-table check, and every gate stayed green. `main()`'s absent-file
    guard saw a MOVED dispatcher and never a SHRUNK roster.
    """

    THREE = ("pretool-writeedit.py", "pretool-bash.py", "pretool-agent-skill.py")

    def _root(self, tmp, registered, on_disk):
        root = Path(tmp)
        hooks = root / ".claude" / "hooks"
        hooks.mkdir(parents=True)
        for name in on_disk:
            (hooks / name).write_text(
                "def c_a(ctx):\n    return None\n", encoding="utf-8"
            )
        commands = [f"$CLAUDE_PROJECT_DIR/.claude/hooks/{n}" for n in registered]
        # A `.sh` PreToolUse hook is registered too, and must not enter the
        # roster: it carries no `def`, so no check and no binding can live in it.
        commands.append("$CLAUDE_PROJECT_DIR/.claude/hooks/block-until-lsp.sh")
        settings = {
            "hooks": {
                "PreToolUse": [
                    {"matcher": "Bash", "hooks": [{"type": "command", "command": c}]}
                    for c in commands
                ]
            }
        }
        (root / ".claude" / "settings.json").write_text(
            json.dumps(settings), encoding="utf-8"
        )
        return root

    def test_roster_is_the_three_committed_dispatchers(self):
        """The pin. The corpus has one at 27 rules; the roster had none."""
        paths, problems = rules_points.dispatchers(ROOT)
        self.assertEqual(problems, [])
        self.assertEqual(
            [p.name for p in paths],
            ["pretool-agent-skill.py", "pretool-bash.py", "pretool-writeedit.py"],
        )

    def test_a_complete_roster_reports_nothing(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root(tmp, self.THREE, self.THREE)
            paths, problems = rules_points.dispatchers(root)
            self.assertEqual(problems, [])
            self.assertEqual(len(paths), 3)

    def test_a_dispatcher_no_settings_entry_runs_is_a_failure(self):
        """The shrink: the file still exists, and nothing dispatches to it."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root(tmp, self.THREE[:2], self.THREE)
            paths, problems = rules_points.dispatchers(root)
            self.assertEqual(len(paths), 2)
            self.assertEqual(len(problems), 1, problems)
            self.assertIn("pretool-agent-skill.py", problems[0])
            self.assertIn("no PreToolUse entry", problems[0])

    def test_a_registered_dispatcher_with_no_file_is_a_failure(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root(tmp, self.THREE, self.THREE[:2])
            paths, problems = rules_points.dispatchers(root)
            self.assertEqual(len(paths), 2)
            self.assertEqual(len(problems), 1, problems)
            self.assertIn("which does not exist", problems[0])

    def test_a_fourth_dispatcher_joins_the_roster(self):
        with tempfile.TemporaryDirectory() as tmp:
            four = self.THREE + ("pretool-newthing.py",)
            root = self._root(tmp, four, four)
            paths, problems = rules_points.dispatchers(root)
            self.assertEqual(problems, [])
            self.assertIn("pretool-newthing.py", [p.name for p in paths])

    def test_an_unreadable_settings_file_is_never_an_empty_roster(self):
        with tempfile.TemporaryDirectory() as tmp:
            paths, problems = rules_points.dispatchers(Path(tmp))
            self.assertEqual(paths, [])
            self.assertEqual(len(problems), 1, problems)
            self.assertIn("the dispatcher roster is unknown", problems[0])

    def test_a_check_that_is_neither_prefixed_nor_bound_is_still_a_check(self):
        """The prefix hole: two of the three dispatchers' gates carry no prefix.

        `verdict` and `review_model_refusal` are bound today, which hid this.
        A third gate beside them, unprefixed and unbound, escaped the roster
        entirely and owed no row in the published table.

        `_helper` is the control: a dispatcher's helpers declare themselves with
        the private prefix, so a gate escapes only by being named private.
        """
        source = (
            "def _helper(x):\n    return x\n\n\n"
            "def gatekeeper(prompt):\n    return None\n\n\n"
            "def main():\n"
            "    payload = _helper(1)\n"
            "    if gatekeeper(payload):\n"
            "        return 2\n"
            "    return 0\n"
        )
        self.assertEqual(rules_points.dispatcher_checks(source, []), {"gatekeeper"})

    def test_a_checks_tuple_is_the_roster_when_one_exists(self):
        """With a dispatch table, main()'s own helpers are not checks."""
        source = (
            "def std_content(ti):\n    return ti\n\n\n"
            "def c_one(ctx):\n    return None\n\n\n"
            "CHECKS = (\n    c_one,\n)\n\n\n"
            "def main():\n"
            "    ctx = std_content({})\n"
            "    for check in CHECKS:\n"
            "        check(ctx)\n"
            "    return 0\n"
        )
        self.assertEqual(rules_points.dispatcher_checks(source, []), {"c_one"})

    def test_an_unparsable_dispatcher_is_a_problem_not_an_empty_roster(self):
        with self.assertRaises(rules_points.RulePointsError):
            rules_points.dispatcher_checks("def broken(:\n", [])


class GatedRatchetTest(unittest.TestCase):
    """The gated set is monotonic against HEAD (`check_coverage_ratchet`'s shape).

    Mutation this fixes: deleting a `# ze point:` comment AND the backticked
    stem from that row's `Enforces` cell left both sides agreeing on nothing.
    The point moved from gated to ungated and every gate exited 0, which is the
    cheapest route from red to green this design had.
    """

    BOTH = (
        "# ze point: alpha/directives/first-directive\n"
        "def c_alpha(ctx):\n    return None\n\n\n"
        "# ze point: alpha/directives/second-directive\n"
        "def c_beta(ctx):\n    return None\n"
    )
    ONE = "# ze point: alpha/directives/second-directive\ndef c_beta(ctx):\n    return None\n"

    def _gm(self, tmp, source, points):
        path = Path(tmp) / "fake-hook.py"
        path.write_text(source, encoding="utf-8")
        return rules_points.gate_map([path], points)

    def test_a_point_that_lost_every_binding_is_a_regression(self):
        with tempfile.TemporaryDirectory() as tmp:
            points = write_points(tmp)
            baseline = rules_points.gated_at_head({"fake-hook.py": self.BOTH})
            self.assertEqual(
                baseline,
                {
                    "alpha/directives/first-directive",
                    "alpha/directives/second-directive",
                },
            )

            # Control: the same baseline against the unchanged source.
            gm = self._gm(tmp, self.BOTH, points)
            self.assertEqual(rules_points.gated_regressions(gm, baseline), [])
            self.assertEqual(rules_points.report_gate_map(gm, regressed=[])[1], 0)

            # The mutation: one binding deleted, everything else identical.
            gm = self._gm(tmp, self.ONE, points)
            regressed = rules_points.gated_regressions(gm, baseline)
            self.assertEqual(regressed, ["alpha/directives/first-directive"])
            lines, code = rules_points.report_gate_map(gm, regressed=regressed)
            self.assertNotEqual(code, 0, "a point that lost its gate must fail")
            self.assertIn("alpha/directives/first-directive", "\n".join(lines))
            self.assertIn("REGRESSED: 1", "\n".join(lines))

    def test_a_point_deleted_from_the_corpus_is_not_a_regression(self):
        """Its instruction left the corpus. That is a rule diff, not a lost gate."""
        with tempfile.TemporaryDirectory() as tmp:
            points = write_points(tmp)
            gm = self._gm(tmp, self.ONE, points)
            baseline = {
                "alpha/directives/first-directive",
                "alpha/directives/deleted-point",
            }
            self.assertEqual(
                rules_points.gated_regressions(gm, baseline),
                ["alpha/directives/first-directive"],
            )

    def test_no_baseline_says_so_rather_than_reading_as_clean(self):
        with tempfile.TemporaryDirectory() as tmp:
            points = write_points(tmp)
            gm = self._gm(tmp, self.BOTH, points)
            lines, code = rules_points.report_gate_map(gm, baseline=False)
            self.assertEqual(code, 0)
            self.assertIn("no HEAD baseline", "\n".join(lines))

    def test_head_sources_over_the_real_repository(self):
        """The baseline reader answers for a committed file, or says it cannot."""
        names = [p.name for p in rules_points.dispatchers(ROOT)[0]]
        sources = rules_points.head_sources(ROOT, names)
        if sources is None:
            self.skipTest("git could not answer; no HEAD baseline on this machine")
        for name, text in sources.items():
            self.assertIn("def ", text, name)


def _dispatcher_checks() -> set[str]:
    """Every check function in the three dispatchers, by its `def` name.

    Enumerated here independently of `rules_points.dispatcher_checks`, so the
    assertion above is a second opinion rather than the production derivation
    agreeing with itself.
    """
    import re

    names = set()
    for path in rules_points.dispatchers(ROOT)[0]:
        text = path.read_text(encoding="utf-8")
        names |= set(re.findall(r"^def ((?:c|check)_[a-z0-9_]+)\(", text, re.MULTILINE))
    # The agent-skill dispatcher's two gates are not `c_`/`check_` prefixed.
    names |= {"verdict", "review_model_refusal"}
    return names


if __name__ == "__main__":
    unittest.main()
