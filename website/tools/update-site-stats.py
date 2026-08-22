#!/usr/bin/env -S uv run python3
"""Refresh rendered website statistic values without rebuilding every page."""

import pathlib
import sys

import check_site_stats
import sitefacts
import sitepaths


def main(argv=None):
    args = list(sys.argv[1:] if argv is None else argv)
    if len(args) > 1:
        print("usage: tools/update-site-stats.py [published-site-root]", file=sys.stderr)
        return 2

    root = pathlib.Path(args[0]).resolve() if args else sitepaths.OUTPUT_ROOT
    if not root.exists():
        print("error: published site root does not exist: %s" % root, file=sys.stderr)
        return 1

    facts_path = root / "data" / "site-facts.json"
    facts_path.parent.mkdir(parents=True, exist_ok=True)
    facts = sitefacts.write_facts(facts_path)
    errors, pages, markers = check_site_stats.update_html_stat_markers(root, facts)
    errors.extend(check_site_stats.check_html_stat_markers(root, facts))
    if errors:
        for error in errors:
            print("error: " + error, file=sys.stderr)
        return 1

    print("wrote %s" % facts_path.relative_to(root))
    print("updated %d stat marker(s) in %d html file(s)" % (markers, len(pages)))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
