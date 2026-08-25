#!/usr/bin/env python3
"""Tests for the proto json_name rewriter.

Ported from `scripts/codegen/proto_json_tags_test.py` when the module moved
into `le`. The three cases are the originals, and the import is the visible
difference: the old file loaded its subject through `importlib.util` and a
`Path(__file__).with_name(...)`, because a script beside it was not importable
as a module. Inside the package it is one `from le.devtools...` line, and a
missing module is an ImportError at collection rather than a None the loader
assertion has to catch.
"""

from __future__ import annotations

import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from le.devtools.proto_json_tags import rewrite_tags


class TestRewriteTags(unittest.TestCase):
    def test_rewrites_each_declared_field_occurrence(self) -> None:
        proto = """
message One {
  string session_id = 1 [json_name = "session-id"];
}
message Two {
  string session_id = 1 [json_name = "session-id"];
  bool read_only = 2 [json_name = "read-only"];
}
"""
        generated = """
SessionID string `protobuf:"bytes,1,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
SessionID string `protobuf:"bytes,1,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
ReadOnly bool `protobuf:"varint,2,opt,name=read_only,json=readOnly,proto3" json:"read_only,omitempty"`
"""
        got = rewrite_tags(proto, generated)
        assert got.count('json:"session-id,omitempty"') == 2
        assert got.count('json:"read-only,omitempty"') == 1
        assert 'json:"session_id,omitempty"' not in got
        assert 'json:"read_only,omitempty"' not in got

    def test_refuses_generated_tag_count_mismatch(self) -> None:
        proto = 'message One {\n  string session_id = 1 [json_name = "session-id"];\n}\n'
        with self.assertRaisesRegex(ValueError, 'generated Go has 0'):
            rewrite_tags(proto, 'package proto\n')

    def test_refuses_proto_without_explicit_names(self) -> None:
        with self.assertRaisesRegex(ValueError, 'no explicit json_name options'):
            rewrite_tags('message One { string session_id = 1; }\n', 'package proto\n')


class TestItFindsTheRealProto(unittest.TestCase):
    """The move changed how the module locates the repository.

    `main()` read `Path(__file__).resolve().parents[2]`, which counted
    directories and was true only in `scripts/codegen`. It reads `REPO_ROOT`
    now. Nothing in the three cases above would have caught the difference,
    because none of them calls `main()`.
    """

    def test_the_declared_paths_exist(self) -> None:
        from le.paths import REPO_ROOT

        assert (REPO_ROOT / 'api/proto/ze.proto').is_file()
        assert (REPO_ROOT / 'api/proto/ze.pb.go').is_file()

    def test_the_real_proto_declares_names_to_apply(self) -> None:
        """Non-vacuity: `rewrite_tags` raises on a proto declaring none."""
        from le.devtools.proto_json_tags import FIELD_WITH_JSON_NAME
        from le.paths import REPO_ROOT

        proto = (REPO_ROOT / 'api/proto/ze.proto').read_text(encoding='utf-8')
        assert FIELD_WITH_JSON_NAME.search(proto), 'the regex matches nothing in the real proto'


if __name__ == '__main__':
    unittest.main()
