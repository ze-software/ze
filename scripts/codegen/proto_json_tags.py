#!/usr/bin/env python3
"""Apply explicit proto json_name options to encoding/json tags in generated Go."""

from __future__ import annotations

import re
from collections import Counter
from pathlib import Path


FIELD_WITH_JSON_NAME = re.compile(
    r'^\s*(?:repeated\s+)?[A-Za-z_][A-Za-z0-9_.]*\s+'
    r'(?P<field>[A-Za-z_][A-Za-z0-9_]*)\s*=\s*\d+\s*'
    r'\[\s*json_name\s*=\s*"(?P<json_name>[^"]+)"\s*\]\s*;\s*$',
    re.MULTILINE,
)


def rewrite_tags(proto: str, generated_go: str) -> str:
    """Return generated_go with every explicit proto json_name applied."""
    declarations = Counter(
        (match.group("field"), match.group("json_name"))
        for match in FIELD_WITH_JSON_NAME.finditer(proto)
    )
    if not declarations:
        raise ValueError("proto source has no explicit json_name options")

    names: dict[str, str] = {}
    for field, json_name in declarations:
        prior = names.setdefault(field, json_name)
        if prior != json_name:
            raise ValueError(
                f"proto field {field!r} has conflicting json_name options: "
                f"{prior!r} and {json_name!r}"
            )

    rewritten = generated_go
    for (field, json_name), expected in sorted(declarations.items()):
        source = f'json:"{field},omitempty"'
        replacement = f'json:"{json_name},omitempty"'
        found = rewritten.count(source)
        if found != expected:
            raise ValueError(
                f"generated Go has {found} {source!r} tags, want {expected} "
                "from the proto declarations"
            )
        rewritten = rewritten.replace(source, replacement)

    return rewritten


def main() -> None:
    root = Path(__file__).resolve().parents[2]
    proto_path = root / "api/proto/ze.proto"
    go_path = root / "api/proto/ze.pb.go"
    rewritten = rewrite_tags(
        proto_path.read_text(encoding="utf-8"),
        go_path.read_text(encoding="utf-8"),
    )
    go_path.write_text(rewritten, encoding="utf-8")


if __name__ == "__main__":
    main()
