#!/usr/bin/env python3
"""Rename schema/ directories to yang/ across the ze codebase.

Phase 1 of spec-yang-rename-ownership: mechanical rename of all schema/
directories containing .yang files to yang/, with Go import path and
package declaration updates.

Usage:
    python3 scripts/dev/rename_schema_to_yang.py              # dry run
    python3 scripts/dev/rename_schema_to_yang.py --apply      # apply changes

Handles:
    - Simple renames (schema/ -> yang/ when no sibling yang/ exists)
    - Merges (.yang files from schema/ into existing yang/ sibling)
    - Go import path updates (/schema" -> /yang")
    - Go import alias updates (xxxschema -> xxxyang)
    - Go package declaration updates (package schema -> package yang)
    - Go identifier usage updates (xxxschema.Foo -> xxxyang.Foo)
    - String literal path updates in tests and docs
    - Skips config/schema/cli/ (separate CLI command, no .yang files)
"""

import os
import re
import shutil
import sys
from pathlib import Path


def find_project_root():
    d = Path(__file__).resolve()
    while d != d.parent:
        if (d / "go.mod").exists():
            return d
        d = d.parent
    raise RuntimeError("go.mod not found")


ROOT = find_project_root()
INTERNAL = ROOT / "internal"
MODULE = "github.com/ze-software/ze"

# config/schema/cli is a separate CLI command ("ze schema"), not a YANG dir.
SKIP_DIRS = {INTERNAL / "component" / "config" / "schema"}


def discover_schema_dirs():
    """Find all schema/ directories under internal/ containing .yang files."""
    schema_dirs = []
    for dirpath, dirnames, filenames in os.walk(INTERNAL):
        p = Path(dirpath)
        if p.name != "schema":
            continue
        if p in SKIP_DIRS:
            continue
        has_yang = any(f.endswith(".yang") for f in filenames)
        if has_yang:
            schema_dirs.append(p)
    return sorted(schema_dirs)


def classify_dirs(schema_dirs):
    """Split into simple renames vs merges (sibling yang/ exists)."""
    simple = []
    merge = []
    for d in schema_dirs:
        sibling_yang = d.parent / "yang"
        if sibling_yang.exists():
            merge.append(d)
        else:
            simple.append(d)
    return simple, merge


def rename_dirs(simple, merge, apply):
    """Rename schema/ -> yang/ or merge .yang files into existing yang/."""
    actions = []

    for d in simple:
        target = d.parent / "yang"
        actions.append(("rename", str(d), str(target)))
        if apply:
            d.rename(target)

    for d in merge:
        target = d.parent / "yang"
        # Move .yang files from schema/ to yang/
        for f in d.iterdir():
            if f.suffix == ".yang":
                dest = target / f.name
                actions.append(("move", str(f), str(dest)))
                if apply:
                    shutil.move(str(f), str(dest))
            elif f.name not in ("embed.go", "register.go") and f.is_file():
                # Move non-glue files (tests, doc.go, etc.)
                dest = target / f.name
                if not dest.exists():
                    actions.append(("move", str(f), str(dest)))
                    if apply:
                        shutil.move(str(f), str(dest))
        # Remove now-empty schema/ dir (may have embed.go + register.go left)
        if apply:
            shutil.rmtree(str(d))
        actions.append(("rmdir", str(d), ""))

    return actions


def find_go_files(root):
    """Find all .go files under internal/, cmd/, pkg/, test/ (excluding vendor/)."""
    go_files = []
    for subdir in ["internal", "cmd", "pkg", "test"]:
        base = root / subdir
        if not base.exists():
            continue
        for dirpath, _, filenames in os.walk(base):
            if "vendor" in Path(dirpath).parts:
                continue
            for f in filenames:
                if f.endswith(".go"):
                    go_files.append(Path(dirpath) / f)
    return sorted(go_files)


def find_doc_files(root):
    """Find markdown files under docs/."""
    doc_files = []
    docs = root / "docs"
    if docs.exists():
        for dirpath, _, filenames in os.walk(docs):
            for f in filenames:
                if f.endswith(".md"):
                    doc_files.append(Path(dirpath) / f)
    return sorted(doc_files)


# Regex patterns for Go import lines.
# Matches: alias "path/to/schema"  or  _ "path/to/schema"  or  "path/to/schema"
RE_IMPORT_LINE = re.compile(
    r"^(\s*)"  # leading whitespace
    r"(?:(\w+)\s+)?"  # optional alias
    r'"([^"]*)/schema"'  # import path ending in /schema
    r"(\s*(?://.*)?)?$",  # optional trailing comment
    re.MULTILINE,
)

# Matches: _ "path/to/schema"
RE_BLANK_IMPORT = re.compile(r'^(\s*)_\s+"([^"]*)/schema"', re.MULTILINE)


def compute_alias_map(content):
    """Parse Go file to build old_alias -> new_alias mapping for schema imports."""
    alias_map = {}

    for m in RE_IMPORT_LINE.finditer(content):
        alias = m.group(2)
        path = m.group(3) + "/schema"

        if alias == "_":
            continue  # blank imports don't need alias tracking

        if alias is None:
            # Bare import: package name is "schema", becomes "yang"
            alias_map["schema"] = "yang"
        elif alias == "schema":
            # Explicit "schema" alias: after rename, package IS "yang", so remove alias
            # Usage changes: schema.Foo -> yang.Foo
            alias_map["schema"] = "yang"
        elif "schema" in alias:
            # Alias like bfdschema, ntpschema -> bfdyang, ntpyang
            new_alias = alias.replace("schema", "yang")
            alias_map[alias] = new_alias
        else:
            # Alias doesn't contain "schema" (unusual). Keep it, just update path.
            pass

    return alias_map


def update_go_imports(content):
    """Update import paths and aliases in a Go file. Returns (new_content, alias_map)."""
    alias_map = compute_alias_map(content)

    def replace_import(m):
        indent = m.group(1)
        alias = m.group(2)
        path_prefix = m.group(3)
        trailing = m.group(4) or ""
        new_path = f"{path_prefix}/yang"

        if alias is None:
            # Bare import: schema -> yang. No alias needed (package IS yang).
            return f'{indent}"{new_path}"{trailing}'
        elif alias == "_":
            # Blank import
            return f'{indent}_ "{new_path}"{trailing}'
        elif alias == "schema":
            # Was explicitly aliased as "schema". Package is now "yang", so drop alias.
            return f'{indent}"{new_path}"{trailing}'
        elif "schema" in alias:
            new_alias = alias.replace("schema", "yang")
            return f'{indent}{new_alias} "{new_path}"{trailing}'
        else:
            return f'{indent}{alias} "{new_path}"{trailing}'

    new_content = RE_IMPORT_LINE.sub(replace_import, content)

    # Also handle blank imports that the main regex might not catch
    def replace_blank(m):
        indent = m.group(1)
        path_prefix = m.group(2)
        return f'{indent}_ "{path_prefix}/yang"'

    new_content = RE_BLANK_IMPORT.sub(replace_blank, new_content)

    return new_content, alias_map


def update_go_identifiers(content, alias_map):
    """Replace identifier usage based on alias mapping."""
    for old, new in alias_map.items():
        # Replace word-boundary matches: old.Something -> new.Something
        # Use word boundary to avoid partial matches
        pattern = re.compile(r"\b" + re.escape(old) + r"\b")
        content = pattern.sub(new, content)
    return content


def update_go_package_decl(content, filepath):
    """Update 'package schema' to 'package yang' for files now in yang/ directories."""
    if filepath.parent.name == "yang":
        content = re.sub(
            r"^package schema_test\b",
            "package yang_test",
            content,
            count=1,
            flags=re.MULTILINE,
        )
        content = re.sub(
            r"^package schema\b", "package yang", content, count=1, flags=re.MULTILINE
        )
    return content


def update_schema_string_refs(content):
    """Update string references to schema/ paths in Go files.

    Handles patterns like:
        "internal/component/bgp/plugins/cmd/rib/schema"
        "internal/component/ike/schema"
    """
    # Match internal/*/schema in string literals
    content = re.sub(r'(internal/[^"]*?)/schema"', r'\1/yang"', content)
    content = re.sub(r'(internal/[^"]*?)/schema([,\s])', r"\1/yang\2", content)
    # Also handle paths at end of string without trailing quote
    content = re.sub(
        r'(internal/[^"`]*?)/schema$', r"\1/yang", content, flags=re.MULTILINE
    )
    return content


def update_doc_refs(content):
    """Update schema/ references in documentation files."""
    # source anchors: internal/component/xxx/schema/
    content = re.sub(r"(internal/[^>\s]*?)/schema/", r"\1/yang/", content)
    # Directory tree references: schema/
    content = re.sub(r"^(.*├── )schema/$", r"\1yang/", content, flags=re.MULTILINE)
    return content


def process_go_file(filepath, apply):
    """Process a single Go file. Returns list of change descriptions."""
    try:
        content = filepath.read_text(encoding="utf-8")
    except (UnicodeDecodeError, OSError):
        return []

    original = content

    # Step 1: Update package declaration if file is in a yang/ dir
    content = update_go_package_decl(content, filepath)

    # Step 2: Update imports and collect alias mapping
    content, alias_map = update_go_imports(content)

    # Step 3: Update identifier usage
    if alias_map:
        content = update_go_identifiers(content, alias_map)

    # Step 4: Update string literal references to schema/ paths
    content = update_schema_string_refs(content)

    if content != original:
        if apply:
            filepath.write_text(content, encoding="utf-8")
        return [str(filepath)]
    return []


def process_doc_file(filepath, apply):
    """Process a documentation file."""
    try:
        content = filepath.read_text(encoding="utf-8")
    except (UnicodeDecodeError, OSError):
        return []

    original = content
    content = update_doc_refs(content)

    if content != original:
        if apply:
            filepath.write_text(content, encoding="utf-8")
        return [str(filepath)]
    return []


def verify_no_stale_refs():
    """Check for remaining /schema references that should have been updated."""
    issues = []

    # Check Go imports
    for f in find_go_files(ROOT):
        try:
            content = f.read_text(encoding="utf-8")
        except (UnicodeDecodeError, OSError):
            continue
        for i, line in enumerate(content.split("\n"), 1):
            stripped = line.strip()
            if '/schema"' in stripped and not stripped.startswith("//"):
                # Allow config/schema/cli (skipped deliberately)
                if "config/schema/cli" in stripped:
                    continue
                issues.append(f"  {f}:{i}: {stripped}")

    # Check for remaining schema/ dirs with .yang files
    for dirpath, _, filenames in os.walk(INTERNAL):
        p = Path(dirpath)
        if p.name == "schema" and p not in SKIP_DIRS:
            has_yang = any(f.endswith(".yang") for f in filenames)
            if has_yang:
                issues.append(f"  schema/ dir with .yang files: {p}")

    return issues


def main():
    apply = "--apply" in sys.argv

    print(f"{'APPLY' if apply else 'DRY RUN'}: rename schema/ -> yang/")
    print(f"Project root: {ROOT}")
    print()

    # Phase 1: Discover
    schema_dirs = discover_schema_dirs()
    simple, merge = classify_dirs(schema_dirs)
    print(f"schema/ dirs with .yang files: {len(schema_dirs)}")
    print(f"  simple rename: {len(simple)}")
    print(f"  merge into yang/: {len(merge)}")
    for d in merge:
        print(f"    {d.relative_to(ROOT)}")
    print()

    # Phase 2: Rename directories
    dir_actions = rename_dirs(simple, merge, apply)
    print(f"Directory operations: {len(dir_actions)}")
    for action, src, dst in dir_actions:
        src_rel = os.path.relpath(src, ROOT)
        dst_rel = os.path.relpath(dst, ROOT) if dst else ""
        if action == "rename":
            print(f"  {src_rel} -> {dst_rel}")
        elif action == "move":
            print(f"  mv {src_rel} -> {dst_rel}")
        elif action == "rmdir":
            print(f"  rm -r {src_rel}")
    print()

    # Phase 3: Update Go files
    go_files = find_go_files(ROOT)
    modified_go = []
    for f in go_files:
        modified_go.extend(process_go_file(f, apply))
    print(f"Go files modified: {len(modified_go)} / {len(go_files)} scanned")
    print()

    # Phase 4: Update doc files
    doc_files = find_doc_files(ROOT)
    modified_docs = []
    for f in doc_files:
        modified_docs.extend(process_doc_file(f, apply))
    print(f"Doc files modified: {len(modified_docs)} / {len(doc_files)} scanned")
    print()

    # Phase 5: Verify
    if apply:
        issues = verify_no_stale_refs()
        if issues:
            print("WARNING: remaining schema references:")
            for issue in issues:
                print(issue)
        else:
            print("No stale schema references found.")
    print()

    # Summary
    print("Skipped (separate CLI, no .yang files):")
    for d in SKIP_DIRS:
        print(f"  {d.relative_to(ROOT)}")

    if not apply:
        print()
        print(
            "This was a dry run. To apply: python3 scripts/dev/rename_schema_to_yang.py --apply"
        )

    return 0


if __name__ == "__main__":
    sys.exit(main())
