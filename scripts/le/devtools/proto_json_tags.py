"""Apply explicit proto `json_name` options to the json tags in generated Go.

`protoc-gen-go` derives a json tag from the FIELD name and ignores the
`json_name` option beside it, so a `.proto` that says `json_name = "as-path"`
still produces `json:"asPath,omitempty"`. Every wire consumer then sees a name
the schema does not declare. This runs after protoc and applies the option the
generator dropped.

It is a rewrite rather than a check, and it refuses rather than guesses: an
unexpected tag count means the generated file and the proto have diverged in a
way this cannot safely patch, so it raises instead of writing a half-converted
file. `ze-proto-generate` runs protoc and then this, in that order, and the
pair is the generation step.
"""

from __future__ import annotations

import re
from collections import Counter

from le.paths import REPO_ROOT

__all__ = ['FIELD_WITH_JSON_NAME', 'main', 'rewrite_tags']

FIELD_WITH_JSON_NAME = re.compile(
    r'^\s*(?:repeated\s+)?[A-Za-z_][A-Za-z0-9_.]*\s+'
    r'(?P<field>[A-Za-z_][A-Za-z0-9_]*)\s*=\s*\d+\s*'
    r'\[\s*json_name\s*=\s*"(?P<json_name>[^"]+)"\s*\]\s*;\s*$',
    re.MULTILINE,
)


def rewrite_tags(proto: str, generated_go: str) -> str:
    """Return `generated_go` with every explicit proto `json_name` applied."""
    declarations = Counter(
        (match.group('field'), match.group('json_name'))
        for match in FIELD_WITH_JSON_NAME.finditer(proto)
    )
    if not declarations:
        raise ValueError('proto source has no explicit json_name options')

    names: dict[str, str] = {}
    for field, json_name in declarations:
        prior = names.setdefault(field, json_name)
        if prior != json_name:
            raise ValueError(
                f'proto field {field!r} has conflicting json_name options: '
                f'{prior!r} and {json_name!r}'
            )

    rewritten = generated_go
    for (field, json_name), expected in sorted(declarations.items()):
        source = f'json:"{field},omitempty"'
        replacement = f'json:"{json_name},omitempty"'
        found = rewritten.count(source)
        if found != expected:
            raise ValueError(
                f'generated Go has {found} {source!r} tags, want {expected} '
                'from the proto declarations'
            )
        rewritten = rewritten.replace(source, replacement)

    return rewritten


def main() -> int:
    """Rewrite api/proto/ze.pb.go in place. Returns the exit code."""
    # REPO_ROOT rather than counting directories up from __file__. The original
    # used `parents[2]`, which was correct in scripts/codegen and silently
    # wrong anywhere else, so it could not be moved without being read first.
    proto_path = REPO_ROOT / 'api/proto/ze.proto'
    go_path = REPO_ROOT / 'api/proto/ze.pb.go'
    rewritten = rewrite_tags(
        proto_path.read_text(encoding='utf-8'),
        go_path.read_text(encoding='utf-8'),
    )
    go_path.write_text(rewritten, encoding='utf-8')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
