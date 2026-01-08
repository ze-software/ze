#!/usr/bin/env python3
"""Migrate .run files from old API syntax to family-first syntax.

Old: announce route <prefix> next-hop <nh> [attrs...]
New: update text [attrs...] nhop set <nh> nlri <family> add <prefix>

Old: announce attributes <attrs>... next-hop <nh> nlri <prefixes>
New: update text <attrs>... nhop set <nh> nlri <family> add <prefixes>
"""

import re
import sys
from pathlib import Path


def detect_family(prefix: str) -> str:
    """Detect address family from prefix."""
    return "ipv6/unicast" if ":" in prefix else "ipv4/unicast"


def convert_attrs(attrs: str) -> str:
    """Convert attribute syntax from old to new per-attribute format.

    Old: origin igp med 100 local-preference 200
    New: origin set igp med set 100 local-preference set 200

    Transforms each attribute keyword to per-attribute syntax.
    """
    import re

    if not attrs.strip():
        return attrs

    # Convert each attribute to per-attribute syntax
    # Scalar attributes: origin, med, local-preference
    result = attrs.strip()
    result = re.sub(r'\borigin\s+(igp|egp|incomplete)\b', r'origin set \1', result, flags=re.IGNORECASE)
    result = re.sub(r'\bmed\s+(\d+)\b', r'med set \1', result)
    result = re.sub(r'\blocal-preference\s+(\d+)\b', r'local-preference set \1', result)

    # AS-PATH: as-path [ASN ASN ...] -> as-path set [ASN ASN ...]
    result = re.sub(r'\bas-path\s+(\[)', r'as-path set \1', result)

    # Community: community [...] -> community set [...]
    result = re.sub(r'\bcommunity\s+(\[)', r'community set \1', result)

    # Large community: large-community [...] -> large-community set [...]
    result = re.sub(r'\blarge-community\s+(\[)', r'large-community set \1', result)

    # Extended community: extended-community [...] -> extended-community set [...]
    result = re.sub(r'\bextended-community\s+(\[)', r'extended-community set \1', result)

    return result


def migrate_announce_route(match: re.Match) -> str:
    """Migrate 'announce route' command."""
    prefix_part = match.group("prefix")
    nh_part = match.group("nh")
    attrs_part = match.group("attrs") or ""

    # Detect family
    family = detect_family(prefix_part)

    # Convert attributes
    attrs = convert_attrs(attrs_part.strip())

    # Build new command
    parts = ["update text"]
    if attrs:
        parts.append(attrs)
    parts.append(f"nhop set {nh_part}")
    parts.append(f"nlri {family} add {prefix_part}")

    return " ".join(parts)


def migrate_announce_attributes(match: re.Match) -> str:
    """Migrate 'announce attributes' command."""
    pre_attrs = match.group("pre_attrs") or ""
    nh_part = match.group("nh")
    post_attrs = match.group("post_attrs") or ""
    nlri_part = match.group("nlri")

    # Parse NLRIs to detect family
    prefixes = nlri_part.strip().split()
    family = detect_family(prefixes[0]) if prefixes else "ipv4/unicast"

    # Convert attributes (combine pre and post)
    all_attrs = f"{pre_attrs.strip()} {post_attrs.strip()}".strip()
    attrs = convert_attrs(all_attrs)

    # Build new command
    parts = ["update text"]
    if attrs:
        parts.append(attrs)
    parts.append(f"nhop set {nh_part}")
    parts.append(f"nlri {family} add {nlri_part.strip()}")

    return " ".join(parts)


def migrate_line(line: str) -> str:
    """Migrate a single line."""
    # Pattern for: [peer/neighbor X] announce route <prefix> next-hop <nh> [attrs...]
    # Use [^\s'\"\\]+ to avoid capturing trailing quotes/commas/escapes
    pattern_route = re.compile(
        r"(?P<peer>(?:peer|neighbor)\s+[^\s'\"\\]+\s+)?"
        r"announce\s+route\s+"
        r"(?P<prefix>[^\s'\"\\]+)\s+"
        r"next-hop\s+(?P<nh>[^\s'\"\\]+)"
        r"(?P<attrs>[^'\"\\]*)?",
        re.IGNORECASE,
    )

    # Pattern for: [peer/neighbor X] announce attributes <pre_attrs>... next-hop <nh> <post_attrs>... nlri <prefixes>
    pattern_attrs = re.compile(
        r"(?P<peer>(?:peer|neighbor)\s+\S+\s+)?"
        r"announce\s+attributes\s+"
        r"(?P<pre_attrs>.*?)\s*"
        r"next-hop\s+(?P<nh>\S+)\s*"
        r"(?P<post_attrs>.*?)\s*"
        r"nlri\s+(?P<nlri>.+)",
        re.IGNORECASE,
    )

    # Pattern for: [peer/neighbor X] withdraw route <prefix> [next-hop X]
    # next-hop is optional and will be discarded (not needed for withdrawals)
    pattern_withdraw = re.compile(
        r"(?P<peer>(?:peer|neighbor)\s+[^\s'\"\\]+\s+)?"
        r"withdraw\s+route\s+"
        r"(?P<prefix>[^\s'\"\\]+)"
        r"(?:\s+next-hop\s+[^\s'\"\\]+)?",  # Optional next-hop, discarded
        re.IGNORECASE,
    )

    # Try announce attributes first (more specific)
    m = pattern_attrs.search(line)
    if m:
        peer = m.group("peer") or ""
        new_cmd = migrate_announce_attributes(m)
        return line[: m.start()] + peer + new_cmd + line[m.end() :]

    # Try announce route
    m = pattern_route.search(line)
    if m:
        peer = m.group("peer") or ""
        new_cmd = migrate_announce_route(m)
        return line[: m.start()] + peer + new_cmd + line[m.end() :]

    # Try withdraw route
    m = pattern_withdraw.search(line)
    if m:
        peer = m.group("peer") or ""
        prefix = m.group("prefix")
        family = detect_family(prefix)
        new_cmd = f"update text nlri {family} del {prefix}"
        return line[: m.start()] + peer + new_cmd + line[m.end() :]

    return line


def migrate_file(path: Path, dry_run: bool = False) -> bool:
    """Migrate a single file. Returns True if changes were made."""
    content = path.read_text()
    new_content = "\n".join(migrate_line(line) for line in content.split("\n"))

    if content == new_content:
        return False

    if dry_run:
        print(f"Would modify: {path}")
    else:
        path.write_text(new_content)
        print(f"Modified: {path}")

    return True


def main():
    import argparse

    parser = argparse.ArgumentParser(description="Migrate API syntax in .run files")
    parser.add_argument("files", nargs="*", help="Files to migrate (default: test/data/**/*.run)")
    parser.add_argument("--dry-run", "-n", action="store_true", help="Show what would be changed")
    args = parser.parse_args()

    if args.files:
        files = [Path(f) for f in args.files]
    else:
        # Default: find all .run files in test/data
        base = Path(__file__).parent.parent / "test" / "data"
        files = list(base.rglob("*.run"))

    modified = 0
    for f in files:
        if migrate_file(f, dry_run=args.dry_run):
            modified += 1

    print(f"\n{modified} file(s) {'would be ' if args.dry_run else ''}modified")


if __name__ == "__main__":
    main()
