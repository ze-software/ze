#!/usr/bin/env python3
"""Bulk find-and-replace with diff preview before applying.

Usage:
    replace.py <file> <old> <new> [--regex] [--all]
    replace.py <file> <old> <new> [--regex] [--all] --apply

Without --apply: prints a unified diff of what would change. Exit 0 if changes found, 1 if none.
With --apply: writes the changes to the file.

Flags:
    --regex   Treat <old> as a Python regex pattern
    --all     Replace all occurrences (default: first only)
    --apply   Write changes to disk (without this, only shows diff)

Examples:
    # Preview a literal replacement
    replace.py internal/core/events/events.go 'OldName' 'NewName' --all

    # Preview a regex replacement
    replace.py config.go 'timeout:\\s*\\d+' 'timeout: 30' --regex

    # Apply after reviewing
    replace.py internal/core/events/events.go 'OldName' 'NewName' --all --apply
"""

import argparse
import difflib
import re
import sys


def main():
    parser = argparse.ArgumentParser(
        description="Bulk find-and-replace with diff preview"
    )
    parser.add_argument("file", help="File to modify")
    parser.add_argument("old", help="String or pattern to find")
    parser.add_argument("new", help="Replacement string")
    parser.add_argument("--regex", action="store_true", help="Treat <old> as regex")
    parser.add_argument(
        "--all",
        action="store_true",
        help="Replace all occurrences (default: first only)",
    )
    parser.add_argument("--apply", action="store_true", help="Write changes to disk")
    args = parser.parse_args()

    try:
        with open(args.file) as f:
            original = f.read()
    except FileNotFoundError:
        print(f"error: file not found: {args.file}", file=sys.stderr)
        sys.exit(2)

    if args.regex:
        try:
            pattern = re.compile(args.old)
        except re.error as e:
            print(f"error: invalid regex: {e}", file=sys.stderr)
            sys.exit(2)
        if args.all:
            modified = pattern.sub(args.new, original)
        else:
            modified = pattern.sub(args.new, original, count=1)
    else:
        if args.all:
            modified = original.replace(args.old, args.new)
        else:
            modified = original.replace(args.old, args.new, 1)

    if modified == original:
        print("no changes", file=sys.stderr)
        sys.exit(1)

    # Count replacements for the summary
    if args.regex:
        count = len(re.findall(args.old, original))
        if not args.all:
            count = min(count, 1)
    else:
        count = original.count(args.old)
        if not args.all:
            count = min(count, 1)

    # Show diff
    diff = difflib.unified_diff(
        original.splitlines(keepends=True),
        modified.splitlines(keepends=True),
        fromfile=f"a/{args.file}",
        tofile=f"b/{args.file}",
    )
    diff_text = "".join(diff)
    print(diff_text, end="")
    print(f"\n--- {count} replacement(s) ---", file=sys.stderr)

    if args.apply:
        with open(args.file, "w") as f:
            f.write(modified)
        print(f"applied to {args.file}", file=sys.stderr)


if __name__ == "__main__":
    main()
