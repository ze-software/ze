#!/usr/bin/env python3
"""Unit tests for spec_doc_anchors.py (spec design-document owner check)."""

import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))

from spec_doc_anchors import (  # noqa: E402
    audit,
    design_doc,
    load_index,
    spec_source_files,
)

INDEX = """# CODE-TO-DOCS

## `pkg/plugin/rpc/`

| File | Docs |
|------|------|
| `` | `docs/architecture/dir-only.md` |
| `message.go` | `docs/architecture/api/ipc_protocol.md`, `docs/plugin-development/protocol.md` |
| `mux.go` | `docs/architecture/api/ipc_protocol.md`, `docs/why-ze.md` |
"""


class SpecDocAnchorsTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        (self.root / "ai").mkdir()
        (self.root / "ai" / "CODE-TO-DOCS.md").write_text(INDEX, encoding="utf-8")
        self.src = self.root / "pkg" / "plugin" / "rpc"
        self.src.mkdir(parents=True)

    def tearDown(self):
        self.tmp.cleanup()

    def write_go(self, name: str, design: str | None):
        header = f"// Design: {design} -- topic\n" if design else ""
        (self.src / name).write_text(header + "\npackage rpc\n", encoding="utf-8")

    # --- design_doc: the strong edge, read from the file itself ---

    def test_reads_the_declared_design_doc(self):
        self.write_go("message.go", "docs/architecture/api/ipc_protocol.md")
        self.assertEqual(
            design_doc(self.root, "pkg/plugin/rpc/message.go"),
            "docs/architecture/api/ipc_protocol.md",
        )

    def test_file_with_no_design_header_declares_nothing(self):
        self.write_go("message.go", None)
        self.assertEqual(design_doc(self.root, "pkg/plugin/rpc/message.go"), "")

    def test_a_file_the_spec_plans_to_create_declares_nothing(self):
        # It does not exist yet, so the spec cannot have omitted its doc.
        self.assertEqual(design_doc(self.root, "pkg/plugin/rpc/absent.go"), "")

    def test_design_line_below_the_header_block_is_not_read(self):
        body = "\n" * 40 + "// Design: docs/late.md -- too late\n"
        (self.src / "late.go").write_text(body, encoding="utf-8")
        self.assertEqual(design_doc(self.root, "pkg/plugin/rpc/late.go"), "")

    # --- index parsing ---

    def test_index_maps_files_to_docs(self):
        index = load_index(self.root)
        self.assertEqual(
            index["pkg/plugin/rpc/mux.go"],
            ["docs/architecture/api/ipc_protocol.md", "docs/why-ze.md"],
        )

    def test_directory_only_row_maps_to_no_path(self):
        index = load_index(self.root)
        self.assertNotIn("pkg/plugin/rpc/", index)

    def test_absent_index_is_empty_so_the_caller_can_refuse(self):
        (self.root / "ai" / "CODE-TO-DOCS.md").unlink()
        self.assertEqual(load_index(self.root), {})

    # --- spec parsing ---

    def test_collects_source_paths_from_both_files_sections(self):
        spec = (
            "## Files to Modify\n"
            "- `pkg/plugin/rpc/message.go` - the tail\n"
            "- `docs/architecture/api/ipc_protocol.md` - the grammar\n"
            "\n## Files to Create\n"
            "- `test/plugin/answer.ci` - functional\n"  # <!-- doc-links: ignore (fixture data: the path must NOT exist, that is the property under test) -->
            "- `scripts/dev/tool.py` - helper\n"  # <!-- doc-links: ignore (fixture data: the path must NOT exist, that is the property under test) -->
        )
        self.assertEqual(
            spec_source_files(spec),
            ["pkg/plugin/rpc/message.go", "scripts/dev/tool.py"],
        )

    def test_a_doc_path_is_not_a_source_file(self):
        spec = "## Files to Modify\n- `docs/architecture/api/commands.md` - contract\n"
        self.assertEqual(spec_source_files(spec), [])

    def test_integration_checklist_rows_are_not_files(self):
        spec = (
            "## Files to Create\n"
            "- `internal/a.go` - real\n"  # <!-- doc-links: ignore (fixture data: the path must NOT exist, that is the property under test) -->
            "### Integration Checklist\n"
            "- `internal/should-not-count.go` - a row, not a file list\n"  # <!-- doc-links: ignore (fixture data: the path must NOT exist, that is the property under test) -->
        )
        self.assertEqual(spec_source_files(spec), ["internal/a.go"])

    # --- audit: the behavior the gate depends on ---

    def test_omitted_owner_is_reported(self):
        self.write_go("message.go", "docs/architecture/api/ipc_protocol.md")
        spec = "## Files to Modify\n- `pkg/plugin/rpc/message.go` - the tail\n"
        owners, _ = audit(
            spec, spec_source_files(spec), self.root, load_index(self.root)
        )
        self.assertEqual(
            owners,
            {"docs/architecture/api/ipc_protocol.md": ["pkg/plugin/rpc/message.go"]},
        )

    def test_named_owner_is_not_reported(self):
        self.write_go("message.go", "docs/architecture/api/ipc_protocol.md")
        spec = (
            "## Files to Modify\n"
            "- `pkg/plugin/rpc/message.go` - the tail\n"
            "- `docs/architecture/api/ipc_protocol.md` - the grammar\n"
        )
        owners, _ = audit(
            spec, spec_source_files(spec), self.root, load_index(self.root)
        )
        self.assertEqual(owners, {})

    def test_naming_the_doc_anywhere_satisfies_the_check(self):
        # A Documentation Update Checklist row saying WHY it is unaffected counts:
        # the requirement is that the author looked, not that they edited it.
        self.write_go("message.go", "docs/architecture/api/ipc_protocol.md")
        spec = (
            "## Files to Modify\n"
            "- `pkg/plugin/rpc/message.go` - the tail\n"
            "\n| 7 | Wire format changed? | No | `docs/architecture/api/ipc_protocol.md`"
            " states framing only, which is unchanged |\n"
        )
        owners, _ = audit(
            spec, spec_source_files(spec), self.root, load_index(self.root)
        )
        self.assertEqual(owners, {})

    def test_mentions_are_separated_from_owners(self):
        self.write_go("mux.go", "docs/architecture/api/ipc_protocol.md")
        spec = "## Files to Modify\n- `pkg/plugin/rpc/mux.go` - lifetime\n"
        owners, mentions = audit(
            spec, spec_source_files(spec), self.root, load_index(self.root)
        )
        # The declared design doc is an owner and must never also appear as a
        # mention: one document owed twice reads as two obligations.
        self.assertIn("docs/architecture/api/ipc_protocol.md", owners)
        self.assertNotIn("docs/architecture/api/ipc_protocol.md", mentions)
        self.assertIn("docs/why-ze.md", mentions)

    def test_a_file_declaring_nothing_produces_no_owner(self):
        self.write_go("message.go", None)
        spec = "## Files to Modify\n- `pkg/plugin/rpc/message.go` - the tail\n"
        owners, _ = audit(
            spec, spec_source_files(spec), self.root, load_index(self.root)
        )
        self.assertEqual(owners, {})

    def test_the_real_omission_this_check_exists_for(self):
        # message.go and mux.go both declare ipc_protocol.md. A spec naming
        # process-protocol.md and commands.md but not ipc_protocol.md is the exact
        # shape that shipped, and it must fail.
        self.write_go("message.go", "docs/architecture/api/ipc_protocol.md")
        self.write_go("mux.go", "docs/architecture/api/ipc_protocol.md")
        spec = (
            "## Files to Modify\n"
            "- `pkg/plugin/rpc/message.go` - tail tokenizer\n"
            "- `pkg/plugin/rpc/mux.go` - pending lifetime\n"
            "- `docs/architecture/api/process-protocol.md` - the wire grammar\n"
            "- `docs/architecture/api/commands.md` - the answer contract\n"
        )
        owners, _ = audit(
            spec, spec_source_files(spec), self.root, load_index(self.root)
        )
        self.assertEqual(
            owners,
            {
                "docs/architecture/api/ipc_protocol.md": [
                    "pkg/plugin/rpc/message.go",
                    "pkg/plugin/rpc/mux.go",
                ]
            },
        )


if __name__ == "__main__":
    unittest.main()
