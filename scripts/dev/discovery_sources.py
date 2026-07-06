"""Shared definition of which changed files can drift a generated discovery index.

Two gates must agree on this set: the commit gate
(`commit_helper.feeds_discovery_index`) and the changed-file router
(`verify_wiring_docs.is_discovery_source`). They previously carried the same
path rules twice with a "keep in sync" comment; the rules now live here once.

The two callers differ only in which file text they scan for the `// Package` /
`// Design:` header markers -- the commit gate reads HEAD, the router reads the
working tree plus HEAD -- so each passes its own `header_text`; the path
patterns are shared.

Generated indexes: ai/PACKAGE-MAP.md (from `// Package` docs + register.go
Description), ai/DOCS-TO-CODE.md (from `// Design:` headers), ai/LEARNED-FULL-INDEX.md
(from plan/learned/NNN-*.md).
"""

from __future__ import annotations

GENERATORS = (
    "scripts/dev/package_map.py",
    "scripts/dev/docs_to_code.py",
    "scripts/dev/learned_index.py",
)

OUTPUTS = (
    "ai/PACKAGE-MAP.md",
    "ai/DOCS-TO-CODE.md",
    "ai/LEARNED-FULL-INDEX.md",
)

HEADER_MARKERS = ("// Package", "// Design:")


def is_discovery_source(path: str, header_text: str = "") -> bool:
    """True if committing `path` can change a generated discovery index.

    `header_text` is only consulted for non-test `.go` files (to look for the
    `// Package` / `// Design:` markers the indexes derive from); the caller
    supplies HEAD and/or working-tree content as appropriate.
    """
    if path in GENERATORS or path in OUTPUTS:
        return True
    if path == "Makefile" or path.startswith("mk/"):
        return True
    if path.startswith("plan/learned/") and path.endswith(".md"):
        return True
    if path.endswith("register.go"):
        return True
    if path.endswith(".go") and not path.endswith("_test.go"):
        return any(marker in header_text for marker in HEADER_MARKERS)
    return False
