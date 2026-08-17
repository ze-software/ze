#!/usr/bin/env python3
"""Focused tests for proto_json_tags.py."""

from __future__ import annotations

import importlib.util
from pathlib import Path
import unittest


MODULE_PATH = Path(__file__).with_name("proto_json_tags.py")
SPEC = importlib.util.spec_from_file_location("proto_json_tags", MODULE_PATH)
assert SPEC is not None and SPEC.loader is not None
PROTO_JSON_TAGS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PROTO_JSON_TAGS)


class RewriteTagsTest(unittest.TestCase):
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
        got = PROTO_JSON_TAGS.rewrite_tags(proto, generated)
        self.assertEqual(got.count('json:"session-id,omitempty"'), 2)
        self.assertEqual(got.count('json:"read-only,omitempty"'), 1)
        self.assertNotIn('json:"session_id,omitempty"', got)
        self.assertNotIn('json:"read_only,omitempty"', got)

    def test_refuses_generated_tag_count_mismatch(self) -> None:
        proto = 'message One {\n  string session_id = 1 [json_name = "session-id"];\n}\n'
        with self.assertRaisesRegex(ValueError, "generated Go has 0"):
            PROTO_JSON_TAGS.rewrite_tags(proto, "package proto\n")

    def test_refuses_proto_without_explicit_names(self) -> None:
        with self.assertRaisesRegex(ValueError, "no explicit json_name options"):
            PROTO_JSON_TAGS.rewrite_tags(
                "message One { string session_id = 1; }\n", "package proto\n"
            )


if __name__ == "__main__":
    unittest.main()
