#!/usr/bin/env python3
"""Build and load shared public facts for the Ze website."""

import json
import pathlib
import re
import sys
import urllib.request
from datetime import date

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
MAIN_REPO = (GH_PAGES.parent / "main").resolve()
DATA_DIR = GH_PAGES / "data"
FACTS_PATH = DATA_DIR / "site-facts.json"

GITHUB_REPO = "ze-software/ze"
GITHUB_STARS_FALLBACK = 46
_github_stars_cache = None

GO_REQUIRE_BLOCK_RE = re.compile(r"require\s*\(([^)]*)\)", re.DOTALL)
GO_REQUIRE_LINE_RE = re.compile(r"^\s*(\S+)\s+\S+(\s*//\s*indirect)?\s*$")
TEST_FUNC_RE = re.compile(r"^func Test[A-Za-z0-9_]+\(", re.MULTILINE)
FUZZ_FUNC_RE = re.compile(r"^func Fuzz[A-Za-z0-9_]+\(", re.MULTILINE)
SKIP_DIRS = {"vendor", ".claude", ".git"}


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
    return bool(SKIP_DIRS & set(parts))


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


def count_go_tests():
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
