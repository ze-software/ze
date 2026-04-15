#!/usr/bin/env python3
"""Format-aware YANG path refactoring across the ze codebase.

When YANG nodes move (remove a container level, rename a node, move a subtree),
references in ~400 locations across 6 file formats need updating. This tool
handles four formats automatically and reports two more for manual editing.

Usage:
    yang_move.py remove <segment> --under <path> [--list-nodes x,y] [--apply]
    yang_move.py rename <old> <new> --under <path> [--list-nodes x,y] [--apply]
    yang_move.py move <src> <dst> [--list-nodes x,y] [--apply]

    yang_move.py --test   # run embedded self-tests

Without --apply: prints a unified diff of what would change. Exit 0 if changes, 1 if none.
With --apply: writes changes to disk.

Formats handled (auto-fix):
    1. Slash-separated paths in strings: "connection/remote/ip" -> "remote/ip"
    2. Space-separated set commands: "set bgp peer p1 connection ..." -> "set ... p1 ..."
    3. Brace-nested config blocks: removes "connection {" + "}", dedents children
    4. Inline Go GetContainer chains: .GetContainer("x").GetContainer("y")

Formats reported (manual fix):
    - YANG node definitions (container/leaf restructuring)
    - Multi-line GetContainer chains with nil checks

Examples:
    # Remove the "connection" container under bgp/peer
    yang_move.py remove connection --under bgp/peer

    # Rename "session" to "protocol" under bgp/peer
    yang_move.py rename session protocol --under bgp/peer

    # Move capability from peer/session/capability to peer/capability
    yang_move.py move bgp/peer/session/capability bgp/peer/capability
"""

import argparse
import difflib
import os
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path


# -- Data types ---------------------------------------------------------------

DEFAULT_LIST_NODES = frozenset({
    "peer", "group", "family", "update", "route", "external",
    "profile", "user", "server", "tunnel", "bridge", "vlan",
    "macvlan", "veth", "wireguard", "vxlan",
})

FILE_GLOBS = {
    "yang":  ["**/*.yang"],
    "go":    ["internal/**/*.go", "cmd/**/*.go", "pkg/**/*.go"],
    "ci":    ["test/**/*.ci"],
    "et":    ["test/**/*.et"],
}


@dataclass
class Operation:
    kind: str          # "remove", "rename", "move"
    target: str        # segment to remove/rename (remove/rename) or source path (move)
    replacement: str   # new name (rename) or destination path (move); empty for remove
    under: str         # path prefix (remove/rename); empty for move
    list_nodes: frozenset


@dataclass
class FileChange:
    path: str
    original: str
    modified: str


@dataclass
class ManualEdit:
    path: str
    line: int
    text: str
    reason: str


# -- Path matching ------------------------------------------------------------

def split_path(p: str) -> list[str]:
    """Split a slash-separated path into segments, filtering empties."""
    return [s for s in p.split("/") if s]


def match_under_prefix(path_segs: list[str], under_segs: list[str],
                       list_nodes: frozenset) -> int | None:
    """Check if path_segs starts with the structural prefix under_segs.

    List keys (segments following a list node name) are skipped in the
    concrete path. Returns the index in path_segs right after the prefix
    match, or None if no match.
    """
    pi = 0  # index into path_segs
    ui = 0  # index into under_segs
    while ui < len(under_segs) and pi < len(path_segs):
        if path_segs[pi] != under_segs[ui]:
            return None
        pi += 1
        # If this segment is a list node, skip the key value in the concrete path
        if under_segs[ui] in list_nodes and pi < len(path_segs):
            pi += 1
        ui += 1
    if ui == len(under_segs):
        return pi
    return None


def transform_slash_path(path_str: str, op: Operation) -> str | None:
    """Transform a slash-separated path string. Returns new path or None if unchanged."""
    segs = split_path(path_str)
    if not segs:
        return None

    if op.kind == "remove":
        under_segs = split_path(op.under)
        # Try matching with under prefix (absolute paths)
        if under_segs:
            idx = match_under_prefix(segs, under_segs, op.list_nodes)
            if idx is not None and idx < len(segs) and segs[idx] == op.target:
                new_segs = segs[:idx] + segs[idx + 1:]
                return "/".join(new_segs) if new_segs else None
        else:
            # No --under: remove first occurrence of target
            if op.target in segs:
                idx = segs.index(op.target)
                new_segs = segs[:idx] + segs[idx + 1:]
                return "/".join(new_segs) if new_segs else None
        # Also handle relative paths (no under prefix in the path itself)
        # e.g., ze:required "connection/remote/ip" where context is implicit
        if segs[0] == op.target:
            new_segs = segs[1:]
            return "/".join(new_segs) if new_segs else None
        return None

    elif op.kind == "rename":
        under_segs = split_path(op.under)
        changed = False
        if under_segs:
            idx = match_under_prefix(segs, under_segs, op.list_nodes)
            if idx is not None and idx < len(segs) and segs[idx] == op.target:
                segs[idx] = op.replacement
                changed = True
        else:
            if op.target in segs:
                idx = segs.index(op.target)
                segs[idx] = op.replacement
                changed = True
        # Also handle relative paths
        if not changed and segs[0] == op.target:
            segs[0] = op.replacement
            changed = True
        return "/".join(segs) if changed else None

    elif op.kind == "move":
        src_segs = split_path(op.target)
        dst_segs = split_path(op.replacement)
        # Check if this path starts with the source prefix (accounting for list keys)
        idx = match_under_prefix(segs, src_segs, op.list_nodes)
        if idx is not None:
            # Replace source prefix with destination, preserving list key values
            # We need to rebuild with actual key values from the original path
            new_segs = _rebuild_move_path(segs, src_segs, dst_segs, op.list_nodes)
            if new_segs is not None:
                remaining = segs[idx:]
                return "/".join(new_segs + remaining)
        return None

    return None


def _rebuild_move_path(path_segs: list[str], src_segs: list[str],
                       dst_segs: list[str], list_nodes: frozenset) -> list[str] | None:
    """Rebuild the prefix portion of a path after a move operation.

    Extracts list key values from the original path and inserts them
    at the right positions in the destination path.
    """
    # Extract key values from original path
    keys = {}  # list_node_name -> key_value
    pi = 0
    for seg in src_segs:
        if pi >= len(path_segs) or path_segs[pi] != seg:
            return None
        pi += 1
        if seg in list_nodes and pi < len(path_segs):
            keys[seg] = path_segs[pi]
            pi += 1

    # Build destination path, inserting key values where needed
    result = []
    for seg in dst_segs:
        result.append(seg)
        if seg in list_nodes and seg in keys:
            result.append(keys[seg])
    return result


# -- Format 1: Slash paths in quoted strings ----------------------------------

# Matches YANG-style slash paths in quotes (lowercase segments with hyphens).
# Excludes Go import paths (contain dots like codeberg.org), file paths (contain
# dots like .md/.go), and URLs (contain ://).
RE_QUOTED_SLASH_PATH = re.compile(r'(?<=["\'])([a-z][\w-]*/[a-z][\w/-]*[a-z\d])(?=["\'])')
# Matches --context <path> in .ci/.et files
RE_CONTEXT_FLAG = re.compile(r'(--context\s+)([\w/.-]+)')


def transform_slash_paths(content: str, op: Operation, ext: str) -> str:
    """Transform slash-separated paths in quoted strings and --context flags."""

    def replace_quoted(m: re.Match) -> str:
        original = m.group(1)
        result = transform_slash_path(original, op)
        return result if result is not None else original

    def replace_context(m: re.Match) -> str:
        prefix = m.group(1)
        original = m.group(2)
        result = transform_slash_path(original, op)
        return prefix + (result if result is not None else original)

    new_content = RE_QUOTED_SLASH_PATH.sub(replace_quoted, content)
    if ext in (".ci", ".et"):
        new_content = RE_CONTEXT_FLAG.sub(replace_context, new_content)
    return new_content


# -- Format 2: Set commands (space-separated paths) ---------------------------

RE_SET_COMMAND = re.compile(r'^(\s*)(set\s+.+)$', re.MULTILINE)
# In .et files: input=type:text=set ...
RE_ET_SET = re.compile(r'(input=type:text=)(set\s+.+)$', re.MULTILINE)


def transform_set_command_line(line: str, op: Operation) -> str | None:
    """Transform a single set command line. Returns new line or None if unchanged."""
    parts = line.split()
    if len(parts) < 2 or parts[0] != "set":
        return None

    # Walk the segments, tracking structural position
    structural = []  # structural path built so far
    new_parts = ["set"]
    i = 1
    changed = False

    while i < len(parts):
        seg = parts[i]
        structural.append(seg)

        if op.kind == "remove":
            under_segs = split_path(op.under)
            struct_path = "/".join(structural)
            # Check if current structural position matches under/target
            if seg == op.target:
                check_path = "/".join(structural[:-1])
                idx = match_under_prefix(split_path(check_path), under_segs, op.list_nodes)
                if idx is not None or not under_segs:
                    # Skip this segment (remove it)
                    changed = True
                    structural.pop()
                    i += 1
                    continue

            new_parts.append(seg)
            # If this is a list node, the next segment is a key value
            if seg in op.list_nodes and i + 1 < len(parts):
                i += 1
                new_parts.append(parts[i])
                # Don't add key to structural path for matching purposes
            i += 1

        elif op.kind == "rename":
            under_segs = split_path(op.under)
            if seg == op.target:
                check_path = "/".join(structural[:-1])
                idx = match_under_prefix(split_path(check_path), under_segs, op.list_nodes)
                if idx is not None or not under_segs:
                    new_parts.append(op.replacement)
                    changed = True
                    if seg in op.list_nodes and i + 1 < len(parts):
                        i += 1
                        new_parts.append(parts[i])
                    i += 1
                    continue

            new_parts.append(seg)
            if seg in op.list_nodes and i + 1 < len(parts):
                i += 1
                new_parts.append(parts[i])
            i += 1
        else:
            new_parts.append(seg)
            if seg in op.list_nodes and i + 1 < len(parts):
                i += 1
                new_parts.append(parts[i])
            i += 1

    if changed:
        return " ".join(new_parts)
    return None


def transform_set_commands(content: str, op: Operation, ext: str) -> str:
    """Transform set commands in file content."""

    def replace_set(m: re.Match) -> str:
        indent = m.group(1)
        line = m.group(2)
        result = transform_set_command_line(line, op)
        if result is not None:
            return indent + result
        return m.group(0)

    def replace_et_set(m: re.Match) -> str:
        prefix = m.group(1)
        line = m.group(2)
        result = transform_set_command_line(line, op)
        if result is not None:
            return prefix + result
        return m.group(0)

    new_content = RE_SET_COMMAND.sub(replace_set, content)
    if ext == ".et":
        new_content = RE_ET_SET.sub(replace_et_set, new_content)
    return new_content


# -- Format 3: Brace-nested config blocks ------------------------------------

def transform_brace_blocks(content: str, op: Operation) -> str:
    """Remove or rename brace-nested config blocks.

    For remove: finds "target {", removes it and matching "}", dedents children.
    For rename: finds "old {" and replaces with "new {".
    """
    if op.kind == "move":
        # Move is decomposed into remove + insert elsewhere; skip here
        return content

    lines = content.split("\n")
    result = []
    i = 0

    while i < len(lines):
        line = lines[i]
        stripped = line.strip()

        if op.kind == "remove":
            # Match "target {" or "target{" at the right nesting context
            if _is_target_brace_open(stripped, op.target):
                if _is_in_context(lines, i, op):
                    # Find matching close brace
                    close_idx = _find_matching_brace(lines, i)
                    if close_idx is not None:
                        # Get the indentation of the target line
                        target_indent = _get_indent(line)
                        child_indent = _detect_child_indent(lines, i + 1, close_idx)
                        # Skip opening line, dedent children, skip closing line
                        for j in range(i + 1, close_idx):
                            dedented = _dedent_line(lines[j], child_indent, target_indent)
                            result.append(dedented)
                        i = close_idx + 1
                        continue

        elif op.kind == "rename":
            if _is_target_brace_open(stripped, op.target):
                if _is_in_context(lines, i, op):
                    # Replace the target name with the new name
                    new_line = line.replace(op.target, op.replacement, 1)
                    result.append(new_line)
                    i += 1
                    continue

        result.append(line)
        i += 1

    return "\n".join(result)


def _is_target_brace_open(stripped: str, target: str) -> bool:
    """Check if a stripped line opens a brace block for the target."""
    # "target {" or "target{"
    return stripped == f"{target} {{" or stripped == f"{target}{{"


def _is_in_context(lines: list[str], target_line: int, op: Operation) -> bool:
    """Check if the target line is within the right nesting context.

    Walks backward from target_line, tracking brace nesting to determine
    the container path at that point.
    """
    under_segs = split_path(op.under)
    if not under_segs:
        return True

    # Build the nesting stack by scanning from the beginning
    stack = []
    depth = 0
    for i in range(target_line):
        line = lines[i].strip()
        if not line or line.startswith("#") or line.startswith("//"):
            continue
        # Count braces
        opens = line.count("{")
        closes = line.count("}")

        if opens > closes:
            # Entering a block - extract the container name
            # Patterns: "name {", "name key {", "name {"
            name = _extract_block_name(line)
            if name:
                stack.append(name)
            depth += opens - closes
        elif closes > opens:
            depth -= closes - opens
            # Pop from stack for each net close
            for _ in range(closes - opens):
                if stack:
                    stack.pop()

    # Check if the structural stack matches --under
    return _stack_matches_under(stack, under_segs, op.list_nodes)


def _stack_matches_under(stack: list[str], under_segs: list[str],
                         list_nodes: frozenset) -> bool:
    """Check if the nesting stack contains the --under path as a subsequence.

    Outer containers (e.g., environment, stdin config blocks) are skipped.
    This means "bgp/peer" matches inside both "bgp { peer ... }" and
    "environment { bgp { peer ... } }".
    """
    si = 0  # stack index
    ui = 0  # under index
    while ui < len(under_segs) and si < len(stack):
        if stack[si] == under_segs[ui]:
            si += 1
            # Skip key value in stack if this is a list node
            if under_segs[ui] in list_nodes and si < len(stack):
                si += 1  # skip the key value
            ui += 1
        else:
            si += 1  # skip non-matching stack entries (could be outer containers)
    return ui == len(under_segs)


def _extract_block_name(line: str) -> str:
    """Extract the container/list name from a line like 'name {' or 'name key {'."""
    # Remove trailing { and whitespace
    stripped = line.rstrip().rstrip("{").strip()
    if not stripped:
        return ""
    # The first word is the name
    parts = stripped.split()
    return parts[0] if parts else ""


def _find_matching_brace(lines: list[str], open_line: int) -> int | None:
    """Find the line containing the matching close brace."""
    depth = 0
    for i in range(open_line, len(lines)):
        depth += lines[i].count("{")
        depth -= lines[i].count("}")
        if depth == 0:
            return i
    return None


def _get_indent(line: str) -> str:
    """Extract the leading whitespace from a line."""
    return line[: len(line) - len(line.lstrip())]


def _detect_child_indent(lines: list[str], start: int, end: int) -> str:
    """Detect the indentation of the first non-empty child line."""
    for i in range(start, end):
        stripped = lines[i].strip()
        if stripped:
            return _get_indent(lines[i])
    return ""


def _dedent_line(line: str, child_indent: str, target_indent: str) -> str:
    """Dedent a line by replacing child_indent prefix with target_indent."""
    if not line.strip():
        return line  # preserve blank lines
    if child_indent and line.startswith(child_indent):
        return target_indent + line[len(child_indent):]
    return line


# -- Format 4: Go GetContainer chains ----------------------------------------

def transform_getcontainer_inline(content: str, op: Operation) -> str:
    """Transform inline GetContainer chains where the call is mid-chain.

    Only transforms when .GetContainer("target") is followed by another
    .Get* call on the same expression (i.e., it is part of a dot-chain,
    not the terminal call). Standalone calls are left for manual editing.

    Handles: .GetContainer("target").GetContainer("child") -> .GetContainer("child")
    """
    if op.kind == "remove":
        # Only remove .GetContainer("target") when followed by another .Get call
        pattern = re.compile(
            r'\.GetContainer\("' + re.escape(op.target) + r'"\)(?=\.Get)'
        )
        return pattern.sub("", content)

    elif op.kind == "rename":
        # Rename is safe even for terminal calls (no structural change)
        pattern = re.compile(
            r'(\.GetContainer\(")' + re.escape(op.target) + r'("\))'
        )
        return pattern.sub(r'\g<1>' + op.replacement + r'\g<2>', content)

    return content


def find_getcontainer_manual(content: str, op: Operation) -> list[ManualEdit]:
    """Find GetContainer calls that need manual editing.

    Reports:
    - Standalone assignment: varName := parent.GetContainer("target")
    - Any .GetContainer("target") NOT followed by another .Get (terminal call)
    """
    edits = []
    target = op.target if op.kind in ("remove", "rename") else None
    if target is None:
        return edits

    # Match lines with .GetContainer("target") that are NOT mid-chain
    # (i.e., the call is the terminal expression, or is assigned to a variable)
    chain_pattern = re.compile(
        r'\.GetContainer\("' + re.escape(target) + r'"\)(?=\.Get)'
    )
    any_pattern = re.compile(
        r'\.GetContainer\("' + re.escape(target) + r'"\)'
    )

    for i, line in enumerate(content.split("\n"), 1):
        stripped = line.strip()
        # Skip if the line has a mid-chain match (those are auto-fixed)
        if chain_pattern.search(stripped):
            continue
        # Report if the line has a terminal/standalone GetContainer call
        if any_pattern.search(stripped):
            edits.append(ManualEdit(
                path="",  # filled in by caller
                line=i,
                text=stripped,
                reason=f"GetContainer(\"{target}\") - not in chain, manual rewrite needed",
            ))
    return edits


# -- Format 6: YANG node reporter --------------------------------------------

def find_yang_definitions(content: str, op: Operation) -> list[ManualEdit]:
    """Find YANG container/leaf definitions that need manual restructuring."""
    edits = []
    target = op.target if op.kind in ("remove", "rename") else split_path(op.target)[-1]

    pattern = re.compile(
        r'^\s*(container|leaf|leaf-list|list|grouping)\s+' + re.escape(target) + r'\s*\{',
    )

    for i, line in enumerate(content.split("\n"), 1):
        m = pattern.match(line)
        if m:
            edits.append(ManualEdit(
                path="",
                line=i,
                text=line.strip(),
                reason=f"YANG {m.group(1)} definition - manual restructure needed",
            ))
    return edits


# -- Orchestration ------------------------------------------------------------

def find_project_root() -> Path:
    """Find the project root via git."""
    try:
        result = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            capture_output=True, text=True, check=True,
        )
        return Path(result.stdout.strip())
    except (subprocess.CalledProcessError, FileNotFoundError):
        return Path.cwd()


def discover_files(root: Path) -> dict[str, list[Path]]:
    """Discover files by category using glob patterns."""
    files = {}
    for category, globs in FILE_GLOBS.items():
        paths = []
        for pattern in globs:
            paths.extend(root.glob(pattern))
        # Filter out vendor/ and tmp/ (check as path components, not substrings)
        paths = [p for p in paths
                 if "vendor" not in p.relative_to(root).parts
                 and "tmp" not in p.relative_to(root).parts]
        files[category] = sorted(paths)
    return files


def process_file(filepath: Path, op: Operation, category: str) -> tuple[FileChange | None, list[ManualEdit]]:
    """Process a single file through applicable transformers."""
    try:
        content = filepath.read_text(encoding="utf-8")
    except (UnicodeDecodeError, OSError):
        return None, []

    ext = filepath.suffix
    original = content
    manual_edits = []

    if category == "yang":
        # Slash paths in YANG constraints
        content = transform_slash_paths(content, op, ext)
        # Report node definitions
        edits = find_yang_definitions(original, op)
        for e in edits:
            e.path = str(filepath)
        manual_edits.extend(edits)

    elif category == "go":
        # Slash paths in string literals
        content = transform_slash_paths(content, op, ext)
        # Inline GetContainer chains
        content = transform_getcontainer_inline(content, op)
        # Set commands in Go string literals (help text)
        content = transform_set_commands(content, op, ext)
        # Report standalone/terminal GetContainer calls
        edits = find_getcontainer_manual(original, op)
        for e in edits:
            e.path = str(filepath)
        manual_edits.extend(edits)

    elif category in ("ci", "et"):
        # Slash paths (--context flags)
        content = transform_slash_paths(content, op, ext)
        # Set commands
        content = transform_set_commands(content, op, ext)
        # Brace blocks
        content = transform_brace_blocks(content, op)

    change = None
    if content != original:
        change = FileChange(
            path=str(filepath),
            original=original,
            modified=content,
        )

    return change, manual_edits


def format_diff(change: FileChange, root: Path) -> str:
    """Format a unified diff for a single file change."""
    rel_path = os.path.relpath(change.path, root)
    diff = difflib.unified_diff(
        change.original.splitlines(keepends=True),
        change.modified.splitlines(keepends=True),
        fromfile=f"a/{rel_path}",
        tofile=f"b/{rel_path}",
    )
    return "".join(diff)


# -- Self-tests ---------------------------------------------------------------

def run_tests():
    """Run embedded self-tests."""
    passed = 0
    failed = 0

    def check(name, got, expected):
        nonlocal passed, failed
        if got == expected:
            passed += 1
        else:
            failed += 1
            print(f"FAIL: {name}", file=sys.stderr)
            print(f"  expected: {expected!r}", file=sys.stderr)
            print(f"  got:      {got!r}", file=sys.stderr)

    # -- match_under_prefix tests --
    ln = DEFAULT_LIST_NODES

    check("match basic",
          match_under_prefix(["bgp", "peer", "p1", "connection"], ["bgp", "peer"], ln),
          3)
    check("match no list key",
          match_under_prefix(["bgp", "router-id"], ["bgp"], ln),
          1)
    check("match mismatch",
          match_under_prefix(["bgp", "router-id"], ["iface"], ln),
          None)

    # -- transform_slash_path: remove --
    op_rm = Operation("remove", "connection", "", "bgp/peer", ln)
    check("slash remove full",
          transform_slash_path("bgp/peer/p1/connection/remote/ip", op_rm),
          "bgp/peer/p1/remote/ip")
    check("slash remove relative",
          transform_slash_path("connection/remote/ip", op_rm),
          "remote/ip")
    check("slash remove no match",
          transform_slash_path("bgp/peer/p1/session/asn", op_rm),
          None)

    # -- transform_slash_path: rename --
    op_rn = Operation("rename", "session", "protocol", "bgp/peer", ln)
    check("slash rename full",
          transform_slash_path("bgp/peer/p1/session/asn/local", op_rn),
          "bgp/peer/p1/protocol/asn/local")
    check("slash rename relative",
          transform_slash_path("session/asn/local", op_rn),
          "protocol/asn/local")

    # -- transform_set_command_line: remove --
    check("set remove",
          transform_set_command_line("set bgp peer p1 connection remote ip 1.2.3.4", op_rm),
          "set bgp peer p1 remote ip 1.2.3.4")
    check("set remove no match",
          transform_set_command_line("set bgp peer p1 session asn local 65000", op_rm),
          None)

    # -- transform_brace_blocks: remove --
    input_brace = (
        "bgp {\n"
        "    peer p1 {\n"
        "        connection {\n"
        "            remote {\n"
        "                ip 1.2.3.4\n"
        "            }\n"
        "        }\n"
        "    }\n"
        "}"
    )
    expected_brace = (
        "bgp {\n"
        "    peer p1 {\n"
        "        remote {\n"
        "            ip 1.2.3.4\n"
        "        }\n"
        "    }\n"
        "}"
    )
    check("brace remove", transform_brace_blocks(input_brace, op_rm), expected_brace)

    # -- transform_brace_blocks: rename --
    op_rn_brace = Operation("rename", "session", "protocol", "bgp/peer", ln)
    input_rename = (
        "bgp {\n"
        "    peer p1 {\n"
        "        session {\n"
        "            asn {\n"
        "                local 65000\n"
        "            }\n"
        "        }\n"
        "    }\n"
        "}"
    )
    expected_rename = (
        "bgp {\n"
        "    peer p1 {\n"
        "        protocol {\n"
        "            asn {\n"
        "                local 65000\n"
        "            }\n"
        "        }\n"
        "    }\n"
        "}"
    )
    check("brace rename", transform_brace_blocks(input_rename, op_rn_brace), expected_rename)

    # -- inline GetContainer: remove (chain) --
    go_input = 'conn := peer.GetContainer("connection").GetContainer("remote")'
    go_expected = 'conn := peer.GetContainer("remote")'
    check("getcontainer chain remove",
          transform_getcontainer_inline(go_input, op_rm),
          go_expected)

    # -- inline GetContainer: remove (standalone, should NOT transform) --
    go_standalone = 'connContainer := peerTree.GetContainer("connection")'
    check("getcontainer standalone no-op",
          transform_getcontainer_inline(go_standalone, op_rm),
          go_standalone)

    # -- inline GetContainer: rename --
    go_input2 = 'sess := peer.GetContainer("session").GetContainer("asn")'
    go_expected2 = 'sess := peer.GetContainer("protocol").GetContainer("asn")'
    check("getcontainer chain rename",
          transform_getcontainer_inline(go_input2, op_rn),
          go_expected2)

    # -- move: slash path --
    op_mv = Operation("move", "bgp/peer/session/capability",
                      "bgp/peer/capability", "", ln)
    check("slash move full",
          transform_slash_path("bgp/peer/p1/session/capability/graceful-restart", op_mv),
          "bgp/peer/p1/capability/graceful-restart")
    check("slash move no match",
          transform_slash_path("bgp/peer/p1/session/asn/local", op_mv),
          None)
    check("slash move exact",
          transform_slash_path("bgp/peer/p1/session/capability", op_mv),
          "bgp/peer/p1/capability")

    # -- Summary --
    total = passed + failed
    if failed:
        print(f"\n{failed}/{total} tests FAILED", file=sys.stderr)
        sys.exit(1)
    else:
        print(f"\n{total}/{total} tests passed", file=sys.stderr)
        sys.exit(0)


# -- CLI entry point ----------------------------------------------------------

def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Format-aware YANG path refactoring for ze",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )

    parser.add_argument("--test", action="store_true",
                        help="Run embedded self-tests")

    # Shared flags for subcommands
    shared = argparse.ArgumentParser(add_help=False)
    shared.add_argument("--apply", action="store_true",
                        help="Write changes to disk (default: preview only)")
    shared.add_argument("--list-nodes", type=str, default=None,
                        help="Comma-separated list node names (default: peer,group,...)")
    shared.add_argument("--verbose", "-v", action="store_true",
                        help="Show per-file processing info")

    sub = parser.add_subparsers(dest="command")

    rm = sub.add_parser("remove", parents=[shared], help="Remove a container level")
    rm.add_argument("segment", help="Container name to remove")
    rm.add_argument("--under", required=True, help="Path prefix (e.g., bgp/peer)")

    rn = sub.add_parser("rename", parents=[shared], help="Rename a node")
    rn.add_argument("old", help="Current node name")
    rn.add_argument("new", help="New node name")
    rn.add_argument("--under", required=True, help="Path prefix")

    mv = sub.add_parser("move", parents=[shared], help="Move a subtree")
    mv.add_argument("src", help="Source path (e.g., bgp/peer/session/capability)")
    mv.add_argument("dst", help="Destination path (e.g., bgp/peer/capability)")

    return parser.parse_args()


def main():
    args = parse_args()

    if args.test:
        run_tests()
        return

    if not args.command:
        print("error: specify a command (remove, rename, move) or --test", file=sys.stderr)
        sys.exit(2)

    # Parse list nodes
    if args.list_nodes:
        list_nodes = frozenset(args.list_nodes.split(","))
    else:
        list_nodes = DEFAULT_LIST_NODES

    # Build operation
    if args.command == "remove":
        op = Operation("remove", args.segment, "", args.under, list_nodes)
    elif args.command == "rename":
        op = Operation("rename", args.old, args.new, args.under, list_nodes)
    elif args.command == "move":
        op = Operation("move", args.src, args.dst, "", list_nodes)
    else:
        print(f"error: unknown command {args.command}", file=sys.stderr)
        sys.exit(2)

    root = find_project_root()
    files_by_cat = discover_files(root)

    all_changes: list[FileChange] = []
    all_manual: list[ManualEdit] = []
    files_scanned = 0

    for category, paths in files_by_cat.items():
        for filepath in paths:
            files_scanned += 1
            change, manual = process_file(filepath, op, category)
            if change:
                all_changes.append(change)
            all_manual.extend(manual)
            if args.verbose and (change or manual):
                rel = os.path.relpath(filepath, root)
                status = []
                if change:
                    status.append("modified")
                if manual:
                    status.append(f"{len(manual)} manual")
                print(f"  {rel}: {', '.join(status)}", file=sys.stderr)

    # Output diff
    diff_text = ""
    for change in all_changes:
        diff_text += format_diff(change, root)

    if diff_text:
        print(diff_text, end="")

    # Summary to stderr
    print(f"\n--- {len(all_changes)} file(s) modified, {files_scanned} scanned ---",
          file=sys.stderr)

    if all_manual:
        print("\nManual edits needed:", file=sys.stderr)
        for edit in all_manual:
            rel = os.path.relpath(edit.path, root)
            print(f"  {rel}:{edit.line}: {edit.reason}", file=sys.stderr)
            print(f"    {edit.text}", file=sys.stderr)

    # Apply
    if args.apply and all_changes:
        for change in all_changes:
            with open(change.path, "w", encoding="utf-8") as f:
                f.write(change.modified)
        print(f"Applied to {len(all_changes)} file(s)", file=sys.stderr)

    sys.exit(0 if all_changes or all_manual else 1)


if __name__ == "__main__":
    main()
