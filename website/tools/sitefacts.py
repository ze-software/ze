#!/usr/bin/env python3
"""Build and load shared public facts for the Ze website."""

import json
import os
import pathlib
import re
import sys
import urllib.request
from datetime import date, datetime, timezone

import sitelib
import sitepaths

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
MAIN_REPO = sitepaths.MAIN_REPO
DATA_DIR = GH_PAGES / "data"
FACTS_PATH = DATA_DIR / "site-facts.json"

GITHUB_REPO = "ze-software/ze"
GITHUB_STARS_FALLBACK = 46
_github_stars_cache = None

GO_REQUIRE_BLOCK_RE = re.compile(r"require\s*\(([^)]*)\)", re.DOTALL)
GO_REQUIRE_LINE_RE = re.compile(r"^\s*(\S+)\s+\S+(\s*//\s*indirect)?\s*$")
TEST_FUNC_RE = re.compile(r"^func Test[A-Za-z0-9_]+\(", re.MULTILINE)
FUZZ_FUNC_RE = re.compile(r"^func Fuzz[A-Za-z0-9_]+\(", re.MULTILINE)
# Directories whose Go tests are NOT Ze's own and must never reach a published
# count. `gokrazy/modcache` is the appliance build's Go module cache: a full copy
# of every third-party dependency, tests included. Omitting it here published
# "121,500+ unit tests" and "570+ fuzz targets" when the real in-repo figures
# were 19,856 and 72 -- 76% of the headline was other people's test suites.
#
# This set is the FALLBACK path only. The authoritative count comes from
# ../main/test/health/latest.json (see count_go_tests), because two independent
# counters over one tree drift by construction: they disagreed by 30 the moment
# both existed, differing on accepted directories and on which function-name
# shapes count.
#
#
# Two kinds, because the matching rule differs:
#   TOP_LEVEL - only meaningful as a root. "gokrazy" as an any-component match
#               also swallowed internal/component/gokrazy/gokrazy_test.go, a
#               first-party test.
#   ANY_LEVEL - genuinely nested third-party or fixture trees.
SKIP_TOP_LEVEL = {".claude", ".git", "gokrazy", "third_party"}
SKIP_ANY_LEVEL = {"vendor", "node_modules"}


def display_step(n):
    """Floor display counts to one tenth of their visible unit.

    Examples:
    - 11,123 is in the thousands group, so it displays at 100 precision.
    - 364 is in the hundreds group, so it displays at 10 precision.
    A plus suffix marks values that were rounded down.
    """
    if n < 100:
        return 1
    if n < 1000:
        return 10
    group_power = 3 * ((len(str(n)) - 1) // 3)
    return 10 ** (group_power - 1)


def fmt_int(n):
    step = display_step(n)
    rounded = (n // step) * step
    suffix = "+" if rounded != n else ""
    return f"{rounded:,}{suffix}"


def load_json(path, default):
    if not path.exists():
        return default
    return json.loads(path.read_text())


def under_skip_dir(path, root):
    try:
        parts = path.relative_to(root).parts
    except ValueError:
        return False
    if parts and parts[0] in SKIP_TOP_LEVEL:
        return True
    # scripts/vendor contains Ze's first-party vendoring commands and tests.
    # Keep that package while still excluding dependency vendor trees below it.
    scanned_parts = parts[2:] if parts[:2] == ("scripts", "vendor") else parts
    return bool(SKIP_ANY_LEVEL & set(scanned_parts))


def count_features():
    data = load_json(DATA_DIR / "features.json", {"sections": []})
    core = 0
    planned = 0
    for section in data.get("sections", []):
        cards = len(section.get("cards", []))
        if section.get("id") in ("core", "experimental"):
            core += cards
        else:
            planned += cards
    return {"core_experimental": core, "planned": planned}


def count_cli_commands():
    return len(load_json(DATA_DIR / "cli-commands.json", []))


def count_config_sections():
    return len(load_json(DATA_DIR / "yang-config-tree.json", {}))


def count_direct_dependencies():
    go_mod = MAIN_REPO / "go.mod"
    if not go_mod.exists():
        return 0
    total = 0
    for block in GO_REQUIRE_BLOCK_RE.findall(go_mod.read_text()):
        for line in block.splitlines():
            match = GO_REQUIRE_LINE_RE.match(line)
            if match and not match.group(2):
                total += 1
    return total


def inventory_counts():
    """The main repository's own test inventory, from test/health/latest.json.

    ONE counter, so the site cannot publish a total the repository disagrees
    with -- the exact failure this page set exists to correct. Returns the
    `inventory` metric's `counts` dict, or None when the generated file is
    absent/shapeless (warned, so a diverging figure is never published silently).
    """
    facts = MAIN_REPO / "test" / "health" / "latest.json"
    if facts.exists():
        try:
            metrics = json.loads(facts.read_text())["metrics"]
            for metric in metrics:
                if metric.get("key") == "inventory":
                    return metric["counts"]
            sitelib.warn(
                "sitefacts: %s has no 'inventory' metric; counting locally. The "
                "published figures will not match the repository's own." % facts
            )
        except (ValueError, KeyError, TypeError) as exc:
            sitelib.warn(
                "sitefacts: %s unreadable (%s); counting locally. The published "
                "figures will not match the repository's own." % (facts, exc)
            )
    elif MAIN_REPO.exists():
        sitelib.warn(
            "sitefacts: %s missing; counting locally. Run `make ze-test-health-update` in "
            "../main so the site and the repository publish the same figures." % facts
        )
    return None


def count_go_tests():
    """Unit and fuzz counts for the published proof strip.

    Reads inventory_counts(); the local walk is only a fallback for a checkout
    without the generated file. A site-only checkout with no sibling ../main is
    NOT drift -- write_facts preserves the last published values rather than
    zeros -- so that path does not warn.
    """
    counts = inventory_counts()
    if counts is not None:
        return {"unit": counts["test_funcs"], "fuzz": counts["fuzz_funcs"]}
    unit = 0
    fuzz = 0
    if not MAIN_REPO.exists():
        return {"unit": unit, "fuzz": fuzz}
    for path in MAIN_REPO.rglob("*_test.go"):
        if under_skip_dir(path, MAIN_REPO):
            continue
        text = path.read_text(errors="ignore")
        unit += len(TEST_FUNC_RE.findall(text))
        fuzz += len(FUZZ_FUNC_RE.findall(text))
    return {"unit": unit, "fuzz": fuzz}


def count_e2e_files():
    """.ci scenario count, from the SAME source as the unit count.

    Previously this walked the tree independently and published 1445 while the
    repository's own inventory said 1443 -- the two-counter drift this page set
    removes, surviving for .ci after it was fixed for unit tests. inventory_counts()
    already carries `ci_files`, so read it.
    """
    counts = inventory_counts()
    if counts is not None and "ci_files" in counts:
        return counts["ci_files"]
    test_dir = MAIN_REPO / "test"
    if not test_dir.exists():
        return 0
    return sum(
        1 for path in test_dir.rglob("*.ci") if not under_skip_dir(path, MAIN_REPO)
    )


def count_interop_targets():
    interop_dir = MAIN_REPO / "test" / "interop"
    if not interop_dir.exists():
        return 0
    dockerfiles = sum(
        1 for path in interop_dir.glob("Dockerfile.*") if path.name != "Dockerfile.ze"
    )
    return dockerfiles + 1  # FRR is selected via FRR_IMAGE.


def count_interop_scenarios():
    scenarios = MAIN_REPO / "test" / "interop" / "scenarios"
    if not scenarios.exists():
        return {"visible": 0, "raw": 0}
    dirs = [path for path in scenarios.iterdir() if path.is_dir()]
    visible = [path for path in dirs if not path.name.startswith(".")]
    return {"visible": len(visible), "raw": len(dirs)}


def count_changes():
    return len(list((GH_PAGES / "changes" / "posts").glob("*.md")))


def count_blog_articles():
    return len(list((GH_PAGES / "blog" / "posts").glob("*.md")))


def github_stars():
    """Fetch the star count once, preserving the last published value offline."""
    global _github_stars_cache
    if _github_stars_cache is not None:
        return _github_stars_cache

    previous = load_json(FACTS_PATH, {}).get("github_stars", GITHUB_STARS_FALLBACK)
    try:
        req = urllib.request.Request(
            "https://api.github.com/repos/%s" % GITHUB_REPO,
            headers={
                "Accept": "application/vnd.github+json",
                "User-Agent": "ze-site-build",
            },
        )
        with urllib.request.urlopen(req, timeout=5) as resp:
            data = json.loads(resp.read())
        _github_stars_cache = int(data["stargazers_count"])
    except Exception as exc:
        _github_stars_cache = int(previous)
        print(
            "warning: could not fetch live GitHub star count (%s), "
            "keeping last published value %d" % (exc, _github_stars_cache),
            file=sys.stderr,
        )
    return _github_stars_cache


def build_facts():
    tests = count_go_tests()
    scenarios = count_interop_scenarios()
    e2e = count_e2e_files()
    targets = count_interop_targets()
    facts = {
        "generated_at": date.today().isoformat(),
        "published_at": published_at(),
        "github_stars": github_stars(),
        "features": count_features(),
        "cli_commands": count_cli_commands(),
        "config_sections": count_config_sections(),
        "dependencies": count_direct_dependencies(),
        "changes": count_changes(),
        "blog_articles": count_blog_articles(),
        "tests": {
            "unit": tests["unit"],
            "unit_display": fmt_int(tests["unit"]),
            "e2e": e2e,
            "e2e_display": fmt_int(e2e),
            "fuzz": tests["fuzz"],
            "fuzz_display": fmt_int(tests["fuzz"]),
        },
        "interop": {
            "targets": targets,
            "target_display": fmt_int(targets),
            "scenarios": scenarios["visible"],
            "scenario_dirs_raw": scenarios["raw"],
        },
    }
    return facts


def write_facts(path=FACTS_PATH):
    facts = build_facts()
    if not MAIN_REPO.exists() and path.exists():
        # ../main is the only source for these; on a checkout without the
        # sibling repo present, keep the last known good values rather than
        # publishing zeros. The site-data-derived counts (features, cli,
        # config, changes, blog) come from local data and stay accurate.
        previous = json.loads(path.read_text())
        for key in ("dependencies", "tests", "interop"):
            if key in previous:
                facts[key] = previous[key]
    path.write_text(json.dumps(facts, indent=2, sort_keys=True) + "\n")
    return facts


def load_facts():
    if FACTS_PATH.exists():
        return json.loads(FACTS_PATH.read_text())
    return build_facts()


PUBLISHED_AT_ENV = "ZE_SITE_PUBLISHED_AT"

_published_at_cache = None


def published_at():
    """The one publication timestamp for this build, ISO-8601 in UTC.

    `website/tools/build.py` stamps ZE_SITE_PUBLISHED_AT once, before the first step,
    and the page renderers it starts as subprocesses inherit it. That is what
    makes one build publish ONE time.

    Reading the stamp out of data/site-facts.json instead does not work, and
    the failure is silent: the `facts` step runs after most page renderers, so
    the pages built before it would carry the PREVIOUS build's timestamp and
    the pages built after it the current one. A build with the env unset (a
    renderer run by hand) falls back to this process's clock, read once."""
    global _published_at_cache
    if _published_at_cache is None:
        _published_at_cache = os.environ.get(PUBLISHED_AT_ENV) or (
            datetime.now(timezone.utc).replace(microsecond=0).isoformat()
        )
    return _published_at_cache


def published_display(raw=None):
    """The publication stamp the site footer carries, formatted for a reader.

    The site is generated and pushed in one run, so the build timestamp IS the
    publication time. The footer names a time rather than a revision because
    the publishing commit's own hash cannot appear in the pages that commit
    contains."""
    stamp = datetime.fromisoformat(raw or published_at())
    # The footer prints "UTC", so convert rather than trust the offset the
    # stamp carries: a page that says UTC over a local-time clock reading is a
    # wrong published fact.
    return stamp.astimezone(timezone.utc).strftime("%-d %B %Y %H:%M UTC")
