#!/usr/bin/env -S uv run python3
"""Render data/site-facts.json from the generated site data and ../main."""

import sitefacts


def main():
    facts = sitefacts.write_facts()
    print(
        "rendered %s: %s CLI commands, %s Go tests, %s fuzz targets"
        % (
            sitefacts.FACTS_PATH,
            facts["cli_commands"],
            facts["tests"]["unit_display"],
            facts["tests"]["fuzz_display"],
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
