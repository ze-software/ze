#!/usr/bin/env python3
"""Render a GitHub-style activity page from git history.

The default metric is additions from `git log --numstat`, grouped by day.
Deleted lines and binary file changes are ignored. The generated page can also
toggle to commit counts for the same date range.

Usage:
    python3 scripts/dev/loc_activity.py
    python3 scripts/dev/loc_activity.py --serve
    python3 scripts/dev/loc_activity.py --output pages/activity.html --days 730
    python3 scripts/dev/loc_activity.py --all-files --author 'Alice'
"""

from __future__ import annotations

import argparse
import datetime as dt
import fnmatch
import html
import json
import subprocess
import webbrowser
from collections import defaultdict
from dataclasses import dataclass
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlparse


DEFAULT_OUTPUT = "tmp/code-activity.html"
DEFAULT_REF = "HEAD"
DEFAULT_SERVE = "127.0.0.1:8000"

CODE_EXTENSIONS = {
    ".awk",
    ".bash",
    ".c",
    ".cc",
    ".cfg",
    ".ci",
    ".conf",
    ".cpp",
    ".css",
    ".et",
    ".go",
    ".h",
    ".hpp",
    ".html",
    ".js",
    ".json",
    ".jsx",
    ".lua",
    ".mk",
    ".pl",
    ".proto",
    ".py",
    ".rb",
    ".rs",
    ".scss",
    ".sh",
    ".sql",
    ".tmpl",
    ".toml",
    ".tpl",
    ".ts",
    ".tsx",
    ".yang",
    ".yaml",
    ".yml",
    ".zsh",
}

CODE_NAMES = {
    "Dockerfile",
    "GNUmakefile",
    "Makefile",
    "go.mod",
    "go.sum",
}

DEFAULT_EXCLUDES = (
    "tmp/*",
    "vendor/*",
    "*/vendor/*",
    "pages/activity.html",
    "pages/code-activity.html",
    "tmp/code-activity.html",
)


@dataclass(frozen=True)
class Options:
    repo: Path
    days: int
    output: Path
    ref: str
    all_refs: bool
    all_files: bool
    extensions: frozenset[str]
    excludes: tuple[str, ...]
    author: str | None
    compact: bool = False
    today: dt.date | None = None


@dataclass(frozen=True)
class Activity:
    additions: dict[dt.date, int]
    commits: dict[dt.date, int]


@dataclass(frozen=True)
class GoBucketStats:
    files: int
    total_lines: int
    code_lines: int
    blank_lines: int
    comment_lines: int


@dataclass(frozen=True)
class GoStats:
    total: GoBucketStats
    code: GoBucketStats
    tests: GoBucketStats
    vendor: GoBucketStats
    vendor_modules: int


def run_git(
    repo: Path, args: list[str], check: bool = True
) -> subprocess.CompletedProcess[str]:
    cmd = ["git", "-c", "core.quotePath=false", "-C", str(repo), *args]
    result = subprocess.run(cmd, text=True, capture_output=True, check=False)
    if check and result.returncode != 0:
        detail = result.stderr.strip() or result.stdout.strip()
        raise SystemExit(f"git command failed: {' '.join(cmd)}\n{detail}")
    return result


def find_repo_root(start: Path) -> Path:
    result = run_git(start, ["rev-parse", "--show-toplevel"], check=False)
    if result.returncode != 0:
        raise SystemExit(f"not inside a git repository: {start}")
    repo = Path(result.stdout.strip()).resolve()
    common = run_git(start, ["rev-parse", "--git-common-dir"], check=False)
    if common.returncode == 0:
        main = Path(common.stdout.strip()).resolve().parent
        if main != repo and (main / "internal").is_dir():
            return main
    return repo


def default_repo() -> Path:
    return find_repo_root(Path(__file__).resolve().parents[2])


def parse_extensions(raw: str) -> frozenset[str]:
    extensions: set[str] = set()
    for item in raw.split(","):
        item = item.strip().lower()
        if not item:
            continue
        if not item.startswith("."):
            item = "." + item
        extensions.add(item)
    if not extensions:
        raise SystemExit("at least one extension is required")
    return frozenset(extensions)


def display_number(value: int) -> str:
    return f"{value:,}"


def numstat_target_path(path: str) -> str:
    """Return a best-effort destination path for renamed numstat entries."""
    path = path.strip()
    if "=>" not in path:
        return path
    if "}" in path and "{" in path:
        prefix, suffix = path.split("}", 1)
        target = prefix.split("=>", 1)[1].strip()
        return target + suffix
    return path.split("=>", 1)[1].strip()


def is_excluded(path: str, excludes: tuple[str, ...]) -> bool:
    return any(fnmatch.fnmatch(path, pattern) for pattern in excludes)


def is_source_path(path: str, options: Options) -> bool:
    path = numstat_target_path(path)
    if is_excluded(path, options.excludes):
        return False
    if options.all_files:
        return True
    name = Path(path).name
    return name in CODE_NAMES or Path(path).suffix.lower() in options.extensions


def collect_activity(options: Options, today: dt.date) -> Activity:
    start = today - dt.timedelta(days=options.days - 1)
    args = [
        "log",
        "--date=short",
        "--pretty=format:@@@%ad",
        "--numstat",
        f"--since={start.isoformat()}",
    ]
    if options.author:
        args.append(f"--author={options.author}")
    if options.all_refs:
        args.append("--all")
    else:
        args.append(options.ref)
    args.append("--")

    output = run_git(options.repo, args).stdout
    additions: dict[dt.date, int] = defaultdict(int)
    commits: dict[dt.date, int] = defaultdict(int)
    current_date: dt.date | None = None

    for line in output.splitlines():
        if line.startswith("@@@"):
            current_date = dt.date.fromisoformat(line[3:])
            commits[current_date] += 1
            continue
        if not current_date or not line:
            continue

        parts = line.split("\t")
        if len(parts) < 3:
            continue
        added, _deleted, path = parts[0], parts[1], parts[2]
        if not added.isdigit():
            continue
        if not is_source_path(path, options):
            continue
        additions[current_date] += int(added)

    return Activity(dict(additions), dict(commits))


def first_commit_date(options: Options) -> dt.date | None:
    args = ["log", "--date=short", "--pretty=format:%ad"]
    if options.all_refs:
        args.append("--all")
    else:
        args.append(options.ref)
    args.append("--")
    dates = [
        dt.date.fromisoformat(line)
        for line in run_git(options.repo, args).stdout.splitlines()
    ]
    if not dates:
        return None
    return min(dates)


def classify_go_line(line: str, in_block: bool) -> tuple[str, bool]:
    stripped = line.strip()
    if not stripped:
        return "blank", in_block

    cursor = stripped
    while True:
        if in_block:
            end = cursor.find("*/")
            if end == -1:
                return "comment", True
            cursor = cursor[end + 2 :].strip()
            in_block = False
            if not cursor:
                return "comment", False
            continue
        if cursor.startswith("//"):
            return "comment", False
        if cursor.startswith("/*"):
            end = cursor.find("*/", 2)
            if end == -1:
                return "comment", True
            cursor = cursor[end + 2 :].strip()
            if not cursor:
                return "comment", False
            continue
        return "code", False


def empty_go_bucket() -> GoBucketStats:
    return GoBucketStats(
        files=0, total_lines=0, code_lines=0, blank_lines=0, comment_lines=0
    )


def add_go_file(bucket: GoBucketStats, path: Path) -> GoBucketStats:
    total_lines = 0
    code_lines = 0
    blank_lines = 0
    comment_lines = 0

    in_block = False
    with open(path, encoding="utf-8", errors="replace") as f:
        for line in f:
            total_lines += 1
            kind, in_block = classify_go_line(line, in_block)
            if kind == "blank":
                blank_lines += 1
            elif kind == "comment":
                comment_lines += 1
            else:
                code_lines += 1

    return GoBucketStats(
        files=bucket.files + 1,
        total_lines=bucket.total_lines + total_lines,
        code_lines=bucket.code_lines + code_lines,
        blank_lines=bucket.blank_lines + blank_lines,
        comment_lines=bucket.comment_lines + comment_lines,
    )


def count_vendor_modules(repo: Path) -> int:
    modules = repo / "vendor" / "modules.txt"
    if not modules.is_file():
        return 0
    total = 0
    with open(modules, encoding="utf-8", errors="replace") as f:
        for line in f:
            if line.startswith("# ") and not line.startswith("##"):
                total += 1
    return total


def collect_go_stats(options: Options) -> GoStats:
    result = run_git(options.repo, ["ls-files", "--", "*.go"])
    total = empty_go_bucket()
    code = empty_go_bucket()
    tests = empty_go_bucket()
    vendor = empty_go_bucket()

    for rel_path in result.stdout.splitlines():
        path = options.repo / rel_path
        if not path.is_file():
            continue
        if rel_path.startswith("vendor/"):
            vendor = add_go_file(vendor, path)
            continue
        if is_excluded(rel_path, options.excludes):
            continue
        total = add_go_file(total, path)
        if rel_path.endswith("_test.go"):
            tests = add_go_file(tests, path)
        else:
            code = add_go_file(code, path)

    return GoStats(
        total=total,
        code=code,
        tests=tests,
        vendor=vendor,
        vendor_modules=count_vendor_modules(options.repo),
    )


def percentile(sorted_values: list[int], fraction: float) -> int:
    if not sorted_values:
        return 0
    index = int(round((len(sorted_values) - 1) * fraction))
    return sorted_values[index]


def activity_thresholds(totals: dict[dt.date, int]) -> list[int]:
    positives = [value for value in totals.values() if value > 0]
    if not positives:
        return [0, 0, 0, 0]
    max_value = max(positives)
    if max_value == 1:
        return [0, 0, 0, 0]
    max_threshold = max_value - 1
    return [
        min(max_threshold, max(1, (max_value * step + 4) // 5)) for step in range(1, 5)
    ]


def activity_level(value: int, thresholds: list[int]) -> int:
    if value <= 0:
        return 0
    for index, threshold in enumerate(thresholds, 1):
        if value <= threshold:
            return index
    return 5


def sunday_before(day: dt.date) -> dt.date:
    days_since_sunday = (day.weekday() + 1) % 7
    return day - dt.timedelta(days=days_since_sunday)


def saturday_after(day: dt.date) -> dt.date:
    days_until_saturday = (5 - day.weekday()) % 7
    return day + dt.timedelta(days=days_until_saturday)


def weeks_between(start: dt.date, end: dt.date) -> list[list[dt.date]]:
    weeks: list[list[dt.date]] = []
    day = start
    while day <= end:
        weeks.append([day + dt.timedelta(days=offset) for offset in range(7)])
        day += dt.timedelta(days=7)
    return weeks


def month_labels(weeks: list[list[dt.date]]) -> str:
    labels: list[str] = []
    seen: set[tuple[int, int]] = set()
    last_label_column = -10
    for column, week in enumerate(weeks, 1):
        label = ""
        for day in week:
            key = (day.year, day.month)
            if day.day == 1 and key not in seen:
                if column - last_label_column >= 4:
                    label = day.strftime("%b")
                    last_label_column = column
                seen.add(key)
                break
        if column == 1 and not label:
            first = week[0]
            label = first.strftime("%b")
            last_label_column = column
            seen.add((first.year, first.month))
        labels.append(
            f'<span class="month-label" style="grid-column:{column}">{html.escape(label)}</span>'
        )
    return "\n".join(labels)


def render_cells(
    weeks: list[list[dt.date]],
    additions: dict[dt.date, int],
    commits: dict[dt.date, int],
    addition_thresholds: list[int],
    commit_thresholds: list[int],
    start: dt.date,
    today: dt.date,
    repo_start: dt.date | None,
) -> str:
    cells: list[str] = []
    for week in weeks:
        for day in week:
            added = additions.get(day, 0)
            commit_count = commits.get(day, 0)
            line_level = activity_level(added, addition_thresholds)
            commit_level = activity_level(commit_count, commit_thresholds)
            outside = day < start or day > today
            classes = ["day-cell"]
            if outside:
                classes.append("outside")
            if repo_start and day < repo_start:
                classes.append("pre-repo")
            date_label = day.strftime("%a %d %b %Y")
            aria_label = (
                f"{date_label}: {display_number(added)} lines added, "
                f"{display_number(commit_count)} commits"
            )
            cells.append(
                "<div "
                f'class="{" ".join(classes)}" '
                f'data-date="{day.isoformat()}" '
                f'data-date-label="{html.escape(date_label)}" '
                f'data-lines="{added}" '
                f'data-lines-display="{display_number(added)}" '
                f'data-lines-level="{line_level}" '
                f'data-commits="{commit_count}" '
                f'data-commits-display="{display_number(commit_count)}" '
                f'data-commits-level="{commit_level}" '
                f'data-level="{line_level}" '
                f'aria-label="{html.escape(aria_label)}" '
                'tabindex="0"'
                "></div>"
            )
    return "\n".join(cells)


TOP_DAY_LIMIT = 14


def render_top_days(totals: dict[dt.date, int], empty_text: str) -> str:
    rows = sorted(totals.items(), key=lambda item: (item[1], item[0]), reverse=True)[
        :TOP_DAY_LIMIT
    ]
    if not rows:
        return f'<tr><td colspan="2">{html.escape(empty_text)}</td></tr>'
    html_rows: list[str] = []
    for day, value in rows:
        html_rows.append(
            "<tr>"
            f"<td>{html.escape(day.strftime('%a %d %b %Y'))}</td>"
            f"<td>{display_number(value)}</td>"
            "</tr>"
        )
    return "\n".join(html_rows)


def render_go_cards(stats: GoBucketStats) -> str:
    return "\n".join(
        [
            f'<div class="stat"><span>Files</span><strong>{display_number(stats.files)}</strong></div>',
            f'<div class="stat"><span>Total lines</span><strong>{display_number(stats.total_lines)}</strong></div>',
            f'<div class="stat"><span>Code</span><strong>{display_number(stats.code_lines)}</strong></div>',
            f'<div class="stat"><span>Blank</span><strong>{display_number(stats.blank_lines)}</strong></div>',
            f'<div class="stat"><span>Comments</span><strong>{display_number(stats.comment_lines)}</strong></div>',
        ]
    )


def render_go_bucket(title: str, stats: GoBucketStats, note: str) -> str:
    note_html = f'<p class="note">{html.escape(note)}</p>' if note else ""
    return f"""<div class="go-bucket">
            <h3>{html.escape(title)}</h3>
            <div class="go-stats">
{render_go_cards(stats)}
            </div>
            {note_html}
        </div>"""


def render_vendor_bucket(stats: GoStats, compact: bool = False) -> str:
    note_html = (
        ""
        if compact
        else '<p class="note">Tracked vendored .go files under vendor/. Module count comes from vendor/modules.txt.</p>'
    )
    return f"""<div class="go-bucket">
            <h3>Vendored Dependencies</h3>
            <div class="go-stats">
{render_go_cards(stats.vendor)}
                <div class="stat"><span>Modules</span><strong>{display_number(stats.vendor_modules)}</strong></div>
            </div>
            {note_html}
        </div>"""


def git_label(options: Options) -> str:
    if options.all_refs:
        return "all refs"
    branch = run_git(
        options.repo, ["rev-parse", "--abbrev-ref", options.ref], check=False
    )
    commit = run_git(options.repo, ["rev-parse", "--short", options.ref], check=False)
    if branch.returncode == 0 and commit.returncode == 0:
        return f"{branch.stdout.strip()} @ {commit.stdout.strip()}"
    return options.ref


def render_page(options: Options) -> str:
    today = options.today or dt.date.today()
    start = today - dt.timedelta(days=options.days - 1)
    grid_start = sunday_before(start)
    grid_end = saturday_after(today)
    weeks = weeks_between(grid_start, grid_end)
    activity = collect_activity(options, today)
    repo_start = first_commit_date(options)
    additions = activity.additions
    commits = activity.commits
    addition_thresholds = activity_thresholds(additions)
    commit_thresholds = activity_thresholds(commits)

    line_days = {
        day: value for day, value in additions.items() if start <= day <= today
    }
    commit_days = {
        day: value for day, value in commits.items() if start <= day <= today
    }
    total_lines = sum(line_days.values())
    total_commits = sum(commit_days.values())
    line_active_days = sum(1 for value in line_days.values() if value > 0)
    commit_active_days = sum(1 for value in commit_days.values() if value > 0)
    line_peak_day, line_peak_value = max(
        line_days.items(), key=lambda item: item[1], default=(today, 0)
    )
    commit_peak_day, commit_peak_value = max(
        commit_days.items(), key=lambda item: item[1], default=(today, 0)
    )
    generated_at = (
        dt.datetime.combine(today, dt.time()).strftime("%Y-%m-%d %H:%M:%S")
        if options.today
        else dt.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    )
    filter_label = "all files" if options.all_files else "source files"
    author_label = options.author or "all authors"
    repo_label = html.escape(str(options.repo))
    ref_label = html.escape(git_label(options))
    title_range = f"{start.isoformat()} to {today.isoformat()}"
    line_threshold_text = ", ".join(
        display_number(value) for value in addition_thresholds
    )
    commit_threshold_text = ", ".join(
        display_number(value) for value in commit_thresholds
    )
    week_count = len(weeks)
    go_stats = collect_go_stats(options)
    metric_summary = {
        "lines": {
            "totalLabel": "Total added lines",
            "totalValue": display_number(total_lines),
            "activeLabel": "Days with added lines",
            "activeValue": display_number(line_active_days),
            "peakLabel": f"Peak line day ({line_peak_day.isoformat()})",
            "peakValue": display_number(line_peak_value),
            "topHeading": "Top Added-Line Days",
            "topColumn": "Added lines",
            "thresholdLabel": "Line thresholds",
            "thresholdValue": line_threshold_text,
        },
        "commits": {
            "totalLabel": "Total commits",
            "totalValue": display_number(total_commits),
            "activeLabel": "Days with commits",
            "activeValue": display_number(commit_active_days),
            "peakLabel": f"Peak commit day ({commit_peak_day.isoformat()})",
            "peakValue": display_number(commit_peak_value),
            "topHeading": "Top Commit Days",
            "topColumn": "Commits",
            "thresholdLabel": "Commit thresholds",
            "thresholdValue": commit_threshold_text,
        },
    }
    metric_summary_json = json.dumps(metric_summary, separators=(",", ":"))

    return f"""<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>Source Line Activity</title>
<style>
:root {{
    color-scheme: dark;
    --bg: #071118;
    --panel: rgba(13, 26, 36, 0.82);
    --panel-strong: rgba(17, 34, 47, 0.94);
    --line: rgba(143, 202, 214, 0.18);
    --text: #e8f7fb;
    --muted: #8fb7c3;
    --soft: #476977;
    --cell: 12px;
    --gap: 3px;
    --level-0: rgba(255, 255, 255, 0.055);
    --level-1: #143d45;
    --level-2: #1d6869;
    --level-3: #299e91;
    --level-4: #48d2b2;
    --level-5: #bef86a;
}}
* {{ box-sizing: border-box; }}
body {{
    margin: 0;
    min-height: 100vh;
    font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    color: var(--text);
    background:
        radial-gradient(circle at 12% 0%, rgba(72, 210, 178, 0.18), transparent 28rem),
        radial-gradient(circle at 88% 10%, rgba(190, 248, 106, 0.12), transparent 26rem),
        linear-gradient(135deg, #071118 0%, #0d1f2d 52%, #061016 100%);
}}
main {{
    display: grid;
    grid-template-rows: auto auto auto auto;
    gap: 0.65rem;
    width: min(1440px, calc(100% - 1.4rem));
    min-height: 100vh;
    margin: 0 auto;
    padding: 0.7rem 0;
    align-content: start;
}}
.hero {{
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 1.5rem;
    align-items: end;
}}
.eyebrow {{
    color: var(--level-5);
    font-size: 0.78rem;
    font-weight: 800;
    letter-spacing: 0.14em;
    text-transform: uppercase;
}}
h1 {{
    margin: 0.2rem 0 0.25rem;
    font-size: clamp(2rem, 3.6vw, 3.4rem);
    line-height: 0.95;
    letter-spacing: -0.06em;
}}
.sub {{
    max-width: 64rem;
    margin: 0;
    color: var(--muted);
    font-size: 0.92rem;
    line-height: 1.35;
}}
.pill {{
    width: fit-content;
    border: 1px solid var(--line);
    border-radius: 999px;
    padding: 0.55rem 0.8rem;
    color: var(--muted);
    background: rgba(7, 17, 24, 0.56);
    white-space: nowrap;
}}
.hero-side {{
    display: flex;
    gap: 0.75rem;
    align-items: center;
    justify-content: flex-end;
}}
.metric-switch {{
    display: inline-flex;
    gap: 0.35rem;
    padding: 0.3rem;
    border: 1px solid var(--line);
    border-radius: 999px;
    background: rgba(7, 17, 24, 0.56);
}}
.metric-switch button {{
    border: 0;
    border-radius: 999px;
    padding: 0.55rem 0.85rem;
    color: var(--muted);
    background: transparent;
    font: inherit;
    font-size: 0.86rem;
    font-weight: 800;
    cursor: pointer;
}}
.metric-switch button[aria-pressed="true"] {{
    color: #061016;
    background: var(--level-5);
}}
.metric-switch button:focus-visible {{
    outline: 2px solid var(--level-5);
    outline-offset: 2px;
}}
.stats {{
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr)){
        " auto" if options.compact else ""
    };
    gap: 0.9rem;
    align-items: center;
}}
.stat {{
    border: 1px solid var(--line);
    border-radius: 0.9rem;
    padding: 0.7rem;
    background: var(--panel);
    box-shadow: 0 20px 70px rgba(0, 0, 0, 0.24);
}}
.stat strong {{
    display: block;
    margin-top: 0.2rem;
    font-size: clamp(1.25rem, 2vw, 1.8rem);
    line-height: 1;
}}
.stat span {{ color: var(--muted); font-size: 0.82rem; }}
.panel {{
    border: 1px solid var(--line);
    border-radius: 1rem;
    padding: 0.8rem;
    background: var(--panel-strong);
    box-shadow: 0 24px 90px rgba(0, 0, 0, 0.28);
}}
.dashboard-grid {{
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(16rem, 20rem);
    gap: 0.65rem;
    min-height: 0;
    align-items: start;
}}
.left-stack,
.right-stack {{
    display: grid;
    gap: 0.65rem;
    min-width: 0;
}}
.activity-panel {{ min-width: 0; }}
.chart-scroll {{ overflow-x: auto; padding-bottom: 0.4rem; }}
.chart {{ min-width: calc({week_count} * (var(--cell) + var(--gap)) + 3.6rem); }}
.months {{
    display: grid;
    grid-template-columns: repeat({week_count}, var(--cell));
    gap: var(--gap);
    margin-left: 3.4rem;
    margin-bottom: 0.45rem;
    color: var(--soft);
    font-size: 0.75rem;
}}
.month-label {{ min-height: 1rem; }}
.chart-body {{ display: flex; gap: 0.7rem; }}
.weekday-labels {{
    display: grid;
    grid-template-rows: repeat(7, var(--cell));
    gap: var(--gap);
    width: 2.7rem;
    color: var(--soft);
    font-size: 0.72rem;
    line-height: var(--cell);
    text-align: right;
}}
.activity-grid {{
    display: grid;
    grid-auto-flow: column;
    grid-template-rows: repeat(7, var(--cell));
    grid-auto-columns: var(--cell);
    gap: var(--gap);
}}
.day-cell {{
    width: var(--cell);
    height: var(--cell);
    border: 1px solid rgba(255, 255, 255, 0.075);
    border-radius: 3px;
    background: var(--level-0);
    cursor: default;
}}
.activity-grid .day-cell:hover,
.activity-grid .day-cell:focus-visible {{
    border-color: rgba(232, 247, 251, 0.82);
    outline: none;
    transform: scale(1.28);
}}
.day-cell[data-level="1"] {{ background: var(--level-1); }}
.day-cell[data-level="2"] {{ background: var(--level-2); }}
.day-cell[data-level="3"] {{ background: var(--level-3); }}
.day-cell[data-level="4"] {{ background: var(--level-4); }}
.day-cell[data-level="5"] {{ background: var(--level-5); box-shadow: 0 0 14px rgba(190, 248, 106, 0.45); }}
.activity-grid .day-cell[data-level="0"]:not(.pre-repo):not(.outside) {{
    border-color: rgba(255, 255, 255, 0.16);
    background: #000;
}}
.day-cell.pre-repo {{
    border-color: rgba(255, 255, 255, 0.055);
    background: var(--level-0);
    opacity: 0.22;
}}
.day-cell.outside {{ opacity: 0.22; }}
.legend {{
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 0.45rem;
    margin-top: 0.85rem;
    color: var(--muted);
    font-size: 0.8rem;
}}
.legend .day-cell {{ display: inline-block; }}
.below {{
    display: grid;
    grid-template-columns: minmax(0, 0.95fr) minmax(20rem, 0.65fr);
    gap: 1rem;
    margin-top: 1rem;
}}
.dashboard-grid .below {{
    grid-template-columns: 1fr;
    gap: 0.65rem;
    margin-top: 0;
    min-width: 0;
}}
.dashboard-grid .below .panel {{ min-width: 0; }}
.dashboard-grid h2,
.go-panel h2 {{
    margin: 0 0 0.45rem;
    font-size: 0.96rem;
}}
.go-panel {{ margin-top: 0; }}
.go-breakdown {{
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 0.65rem;
}}
.go-bucket {{
    border: 1px solid var(--line);
    border-radius: 1rem;
    padding: 0.65rem;
    background: rgba(7, 17, 24, 0.34);
}}
.go-bucket h3 {{
    margin: 0 0 0.45rem;
    color: var(--level-5);
    font-size: 0.88rem;
}}
.go-stats {{
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.45rem;
}}
.go-panel .stat {{ padding: 0.5rem; }}
.go-panel .stat strong {{ font-size: 1.05rem; }}
.go-panel .stat span {{ font-size: 0.72rem; }}
.right-stack .panel {{ padding: 0.65rem; }}
.right-stack table th,
.right-stack table td {{ padding: 0.22rem 0; }}
.right-stack td {{ font-size: 0.78rem; }}
.right-stack .meta {{
    gap: 0.32rem;
    font-size: 0.78rem;
    line-height: 1.25;
}}
.right-stack .note {{ font-size: 0.66rem; }}
table {{ width: 100%; border-collapse: collapse; }}
table[hidden] {{ display: none; }}
th, td {{ padding: 0.34rem 0; border-bottom: 1px solid var(--line); text-align: left; }}
th {{ color: var(--muted); font-size: 0.72rem; font-weight: 700; text-transform: uppercase; letter-spacing: 0.08em; }}
td {{ font-size: 0.84rem; }}
td:last-child, th:last-child {{ text-align: right; }}
.meta {{
    display: grid;
    gap: 0.55rem;
    color: var(--muted);
    font-size: 0.92rem;
    line-height: 1.5;
}}
.meta code {{
    color: var(--text);
    overflow-wrap: anywhere;
    word-break: break-word;
}}
.note {{ margin: 0.55rem 0 0; color: var(--soft); font-size: 0.73rem; line-height: 1.32; }}
.activity-tooltip {{
    position: fixed;
    z-index: 50;
    min-width: 15rem;
    padding: 0.95rem 1rem;
    border: 1px solid rgba(190, 248, 106, 0.34);
    border-radius: 0.9rem;
    color: var(--text);
    background: rgba(4, 10, 14, 0.96);
    box-shadow: 0 18px 60px rgba(0, 0, 0, 0.45), 0 0 30px rgba(72, 210, 178, 0.12);
    pointer-events: none;
}}
.activity-tooltip[hidden] {{ display: none; }}
.tooltip-date {{
    display: block;
    font-size: 1.05rem;
    font-weight: 900;
}}
.tooltip-gap {{ height: 0.75rem; }}
.tooltip-value {{
    display: block;
    color: var(--muted);
    font-size: 0.98rem;
    line-height: 1.45;
}}
@media (max-width: 860px) {{
    body {{ overflow: auto; }}
    main {{ min-height: auto; }}
    .hero, .below, .dashboard-grid, .go-breakdown {{ grid-template-columns: 1fr; }}
    .hero-side {{ justify-items: start; }}
    .stats, .go-stats {{ grid-template-columns: repeat(2, minmax(0, 1fr)); }}
    .pill {{ white-space: normal; }}
}}
@media (min-width: 1100px) and (min-height: 760px) {{
    body {{ overflow-x: hidden; }}
}}
@media (max-width: 520px) {{
    main {{ width: min(100% - 1rem, 1180px); }}
    .stats, .go-stats {{ grid-template-columns: 1fr; }}
}}
</style>
</head>
<body>
<main>
{
        ""
        if options.compact
        else f'''    <section class="hero">
        <div>
            <div class="eyebrow">Git additions and commits</div>
            <h1>Source Line Activity</h1>
            <p class="sub">A GitHub-style daily grid where brighter squares mean more activity. Toggle between source lines added and commit counts. Deletions and binary file changes are ignored for the added-line metric.</p>
        </div>
        <div class="hero-side">
            <div class="metric-switch" aria-label="Activity metric">
                <button type="button" data-metric="lines" aria-pressed="true">Added Lines</button>
                <button type="button" data-metric="commits" aria-pressed="false">Commits</button>
            </div>
            <div class="pill">{html.escape(title_range)}</div>
        </div>
    </section>'''
    }

    <section class="stats" aria-label="Summary">
        <div class="stat"><span id="total-label">Total added lines</span><strong id="total-value">{
        display_number(total_lines)
    }</strong></div>
        <div class="stat"><span id="active-label">Days with added lines</span><strong id="active-value">{
        display_number(line_active_days)
    }</strong></div>
        <div class="stat"><span id="peak-label">Peak line day ({
        html.escape(line_peak_day.isoformat())
    })</span><strong id="peak-value">{display_number(line_peak_value)}</strong></div>
        <div class="stat"><span>Days shown</span><strong>{
        display_number(options.days)
    }</strong></div>
{
        f'''        <div style="display:flex;flex-direction:column;align-items:center;justify-content:center;gap:0.4rem">
            <div class="metric-switch" aria-label="Activity metric">
                <button type="button" data-metric="lines" aria-pressed="true">Added Lines</button>
                <button type="button" data-metric="commits" aria-pressed="false">Commits</button>
            </div>
            <div class="pill">{html.escape(title_range)}</div>
        </div>'''
        if options.compact
        else ""
    }
    </section>

    <div class="dashboard-grid">
    <div class="left-stack">
    <section class="panel activity-panel" aria-label="Daily source line activity">
        <div class="chart-scroll">
            <div class="chart">
                <div class="months" aria-hidden="true">
{month_labels(weeks)}
                </div>
                <div class="chart-body">
                    <div class="weekday-labels" aria-hidden="true"><span></span><span>Mon</span><span></span><span>Wed</span><span></span><span>Fri</span><span></span></div>
                    <div class="activity-grid" role="img" aria-label="Daily activity from {
        html.escape(title_range)
    }">
{
        render_cells(
            weeks,
            additions,
            commits,
            addition_thresholds,
            commit_thresholds,
            start,
            today,
            repo_start,
        )
    }
                    </div>
                </div>
            </div>
        </div>
        <div class="legend"><span>Less</span><span class="day-cell" data-level="0"></span><span class="day-cell" data-level="1"></span><span class="day-cell" data-level="2"></span><span class="day-cell" data-level="3"></span><span class="day-cell" data-level="4"></span><span class="day-cell" data-level="5"></span><span>More</span></div>
    </section>
    <section class="panel go-panel" aria-label="Go code stats">
        <h2>Go Code Stats</h2>
        <div class="go-breakdown">
{
        render_go_bucket(
            "Total First-Party Go",
            go_stats.total,
            ""
            if options.compact
            else "Tracked .go files outside vendor/, including tests.",
        )
    }
{
        render_go_bucket(
            "Production Go",
            go_stats.code,
            ""
            if options.compact
            else "Tracked .go files outside vendor/ and excluding _test.go files.",
        )
    }
{
        render_go_bucket(
            "Test Go",
            go_stats.tests,
            "" if options.compact else "Tracked _test.go files outside vendor/.",
        )
    }
{render_vendor_bucket(go_stats, compact=options.compact)}
        </div>
{
        ""
        if options.compact
        else '        <p class="note">Comment lines are full-line <code>//</code> comments or block-comment lines; inline comments remain code lines.</p>'
    }
    </section>
    </div>

    <div class="right-stack">
    <section class="below">
        <div class="panel">
            <h2 id="top-heading">Top Added-Line Days</h2>
            <table data-top-table="lines">
                <thead><tr><th>Day</th><th>Added lines</th></tr></thead>
                <tbody>
{render_top_days(line_days, "No added source lines in this range.")}
                </tbody>
            </table>
            <table data-top-table="commits" hidden>
                <thead><tr><th>Day</th><th>Commits</th></tr></thead>
                <tbody>
{render_top_days(commit_days, "No commits in this range.")}
                </tbody>
            </table>
        </div>
{
        ""
        if options.compact
        else f'''        <div class="panel meta">
            <div><strong>Repository:</strong> <code>{repo_label}</code></div>
            <div><strong>Ref:</strong> <code>{ref_label}</code></div>
            <div><strong>Author filter:</strong> <code>{html.escape(author_label)}</code></div>
            <div><strong>File filter:</strong> <code>{html.escape(filter_label)}</code></div>
            <div><strong id="threshold-label">Line thresholds</strong>: <code id="threshold-value">{html.escape(line_threshold_text)}</code></div>
            <div><strong>Generated:</strong> <code>{html.escape(generated_at)}</code></div>
            <p class="note">Source mode counts common source, test, template, config, and schema extensions, excluding vendor and tmp paths. Use <code>--all-files</code> to count every text file reported by git.</p>
        </div>'''
    }
    </section>
    </div>
    </div>
</main>
<div id="activity-tooltip" class="activity-tooltip" role="tooltip" hidden>
    <span class="tooltip-date"></span>
    <div class="tooltip-gap"></div>
    <span class="tooltip-value tooltip-primary"></span>
    <span class="tooltip-value tooltip-secondary"></span>
</div>
<script>
const metricSummaries = {metric_summary_json};
let currentMetric = "lines";
const cells = Array.from(document.querySelectorAll(".activity-grid .day-cell"));
const buttons = Array.from(document.querySelectorAll(".metric-switch button"));
const tooltip = document.getElementById("activity-tooltip");
const tooltipDate = tooltip.querySelector(".tooltip-date");
const tooltipPrimary = tooltip.querySelector(".tooltip-primary");
const tooltipSecondary = tooltip.querySelector(".tooltip-secondary");

function plural(value, one, many) {{
    return value === 1 ? one : many;
}}

function valueLine(metric, cell) {{
    if (metric === "commits") {{
        const value = Number(cell.dataset.commits || "0");
        return `${{cell.dataset.commitsDisplay}} ${{plural(value, "commit", "commits")}}`;
    }}
    const value = Number(cell.dataset.lines || "0");
    return `${{cell.dataset.linesDisplay}} ${{plural(value, "line", "lines")}} added`;
}}

function setEl(id, text) {{
    var el = document.getElementById(id);
    if (el) el.textContent = text;
}}

function setMetric(metric) {{
    currentMetric = metric;
    document.body.dataset.metric = metric;
    const summary = metricSummaries[metric];
    setEl("total-label", summary.totalLabel);
    setEl("total-value", summary.totalValue);
    setEl("active-label", summary.activeLabel);
    setEl("active-value", summary.activeValue);
    setEl("peak-label", summary.peakLabel);
    setEl("peak-value", summary.peakValue);
    setEl("top-heading", summary.topHeading);
    setEl("threshold-label", summary.thresholdLabel);
    setEl("threshold-value", summary.thresholdValue);
    for (const button of buttons) {{
        button.setAttribute("aria-pressed", String(button.dataset.metric === metric));
    }}
    for (const cell of cells) {{
        cell.dataset.level = metric === "commits" ? cell.dataset.commitsLevel : cell.dataset.linesLevel;
    }}
    for (const table of document.querySelectorAll("[data-top-table]")) {{
        table.hidden = table.dataset.topTable !== metric;
    }}
}}

function fillTooltip(cell) {{
    tooltipDate.textContent = cell.dataset.dateLabel;
    if (currentMetric === "commits") {{
        tooltipPrimary.textContent = valueLine("commits", cell);
        tooltipSecondary.textContent = valueLine("lines", cell);
    }} else {{
        tooltipPrimary.textContent = valueLine("lines", cell);
        tooltipSecondary.textContent = valueLine("commits", cell);
    }}
}}

function placeTooltip(x, y) {{
    const margin = 14;
    tooltip.hidden = false;
    let left = x + margin;
    let top = y + margin;
    const width = tooltip.offsetWidth;
    const height = tooltip.offsetHeight;
    if (left + width + margin > window.innerWidth) {{
        left = x - width - margin;
    }}
    if (top + height + margin > window.innerHeight) {{
        top = y - height - margin;
    }}
    tooltip.style.left = `${{Math.max(margin, left)}}px`;
    tooltip.style.top = `${{Math.max(margin, top)}}px`;
}}

function showTooltip(cell, event) {{
    fillTooltip(cell);
    placeTooltip(event.clientX, event.clientY);
}}

function showFocusTooltip(cell) {{
    const rect = cell.getBoundingClientRect();
    fillTooltip(cell);
    placeTooltip(rect.left + rect.width / 2, rect.top + rect.height / 2);
}}

function hideTooltip() {{
    tooltip.hidden = true;
}}

for (const button of buttons) {{
    button.addEventListener("click", () => setMetric(button.dataset.metric));
}}
for (const cell of cells) {{
    cell.addEventListener("pointerenter", (event) => showTooltip(cell, event));
    cell.addEventListener("pointermove", (event) => placeTooltip(event.clientX, event.clientY));
    cell.addEventListener("pointerleave", hideTooltip);
    cell.addEventListener("focus", () => showFocusTooltip(cell));
    cell.addEventListener("blur", hideTooltip);
}}
setMetric("lines");
</script>
</body>
</html>
"""


def parse_host_port(value: str) -> tuple[str, int]:
    if ":" not in value:
        return value, 8000
    host, raw_port = value.rsplit(":", 1)
    if not host:
        host = "127.0.0.1"
    try:
        port = int(raw_port)
    except ValueError as exc:
        raise SystemExit(f"invalid port: {raw_port}") from exc
    return host, port


def serve(options: Options, address: str, open_browser: bool) -> None:
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self) -> None:
            parsed = urlparse(self.path)
            if parsed.path not in ("/", "/activity.html"):
                self.send_error(404)
                return
            try:
                body = render_page(options).encode("utf-8")
                status = 200
            except Exception as exc:  # noqa: BLE001 - show the browser a useful failure.
                body = f"<pre>{html.escape(str(exc))}</pre>".encode("utf-8")
                status = 500
            self.send_response(status)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, format: str, *args: object) -> None:
            print(f"{self.address_string()} - {format % args}")

    host, port = parse_host_port(address)
    server = ThreadingHTTPServer((host, port), Handler)
    url = f"http://{host}:{port}/"
    print(f"serving dynamic activity page at {url}")
    if open_browser:
        webbrowser.open(url)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nstopped")
    finally:
        server.server_close()


class HelpFormatter(argparse.RawDescriptionHelpFormatter):
    pass


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description=(
            "Generate a self-contained GitHub-style activity dashboard from git history. "
            "The page can toggle between added source lines and commit counts, "
            "and includes current Go code and vendored dependency stats."
        ),
        formatter_class=HelpFormatter,
        epilog=f"""examples:
  Generate the default static page:
    python3 scripts/dev/loc_activity.py

  Open a dynamic page that recomputes on each refresh:
    python3 scripts/dev/loc_activity.py --serve --open

  Write the page into the static website directory:
    python3 scripts/dev/loc_activity.py --output pages/activity.html

  Show two years, scanning every ref:
    python3 scripts/dev/loc_activity.py --days 730 --all

  Restrict history to one author:
    python3 scripts/dev/loc_activity.py --author 'Alice'

notes:
  Added-line mode uses git log --numstat additions only. Deletions and binary changes are ignored.
  Commit mode counts commits per day for the same ref, author, and day range.
  Source mode counts common code, test, template, config, and schema extensions.
  Go stats are based on tracked .go files in the current working tree.
  Default output is {DEFAULT_OUTPUT}; tmp/ is ignored by git in this repository.
""",
    )
    parser.add_argument(
        "--repo",
        type=Path,
        help="git repository to scan, default: repo containing this script",
    )
    parser.add_argument(
        "--days",
        type=int,
        default=365,
        help="number of calendar days to show, default: 365",
    )
    parser.add_argument(
        "--output",
        type=Path,
        default=Path(DEFAULT_OUTPUT),
        help=f"static HTML file to write, default: {DEFAULT_OUTPUT}",
    )
    parser.add_argument(
        "--ref",
        default=DEFAULT_REF,
        help=f"git ref to scan when --all is not used, default: {DEFAULT_REF}",
    )
    parser.add_argument(
        "--all", action="store_true", help="scan all git refs instead of --ref"
    )
    parser.add_argument(
        "--all-files",
        action="store_true",
        help="count additions from every file reported by git numstat",
    )
    parser.add_argument(
        "--extensions",
        default=",".join(sorted(CODE_EXTENSIONS)),
        help="comma-separated file extensions counted in source mode, default: built-in common source set",
    )
    parser.add_argument(
        "--exclude",
        action="append",
        default=[],
        help="fnmatch path pattern to ignore, repeatable",
    )
    parser.add_argument(
        "--author", help="git --author regex filter for history metrics"
    )
    parser.add_argument(
        "--today",
        type=dt.date.fromisoformat,
        help="final date in YYYY-MM-DD form, for reproducible historical snapshots",
    )
    parser.add_argument(
        "--serve",
        nargs="?",
        const=DEFAULT_SERVE,
        metavar="HOST:PORT",
        help=f"serve dynamically and recompute on each request, default address: {DEFAULT_SERVE}",
    )
    parser.add_argument(
        "--open",
        action="store_true",
        help="open the generated or served page in a browser",
    )
    parser.add_argument(
        "--compact",
        action="store_true",
        help="compact output: heatmap and stats only, no Go stats or metadata",
    )
    return parser


def main() -> None:
    parser = build_parser()
    args = parser.parse_args()
    if args.days <= 0:
        raise SystemExit("--days must be positive")

    repo = find_repo_root((args.repo or default_repo()).resolve())
    output = args.output
    if not output.is_absolute():
        output = repo / output

    excludes = list(DEFAULT_EXCLUDES) + list(args.exclude)
    try:
        excludes.append(output.relative_to(repo).as_posix())
    except ValueError:
        pass

    options = Options(
        repo=repo,
        days=args.days,
        output=output,
        ref=args.ref,
        all_refs=args.all,
        all_files=args.all_files,
        extensions=parse_extensions(args.extensions),
        excludes=tuple(excludes),
        author=args.author,
        compact=args.compact,
        today=args.today,
    )

    if args.serve:
        serve(options, args.serve, args.open)
        return

    content = render_page(options)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(content, encoding="utf-8")
    print(f"wrote {output}")
    if args.open:
        webbrowser.open(output.resolve().as_uri())


if __name__ == "__main__":
    main()
