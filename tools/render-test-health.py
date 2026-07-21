#!/usr/bin/env -S uv run python3
"""Render the testing-health page from ../main/test/health/*.

Design: ../main/plan/spec-test-health-dashboard.md

This renderer computes NOTHING itself. Every number comes from
../main/scripts/dev/testing_health.py: at build time it runs that generator
read-only (`--json` for the metrics, `--emit-page` for the page Markdown) against
the main tree being built, so the published numbers reflect the tree, not
whatever was committed at the last `make ze-test-health`. When the generator
cannot run but the main tree is present, it falls back to the committed
../main/test/health/latest.json (+ history.ndjson) and
../main/docs/features/test-health.md. (Those live under ../main too, so a
checkout with no main tree has no fallback either and the build fails loudly --
which is correct: the site cannot be built without the repository.) Keeping ALL
the arithmetic on the main side of the boundary is what stops the site and the
repository ever publishing two different answers to the same question -- the
failure this page was built to correct, where data/site-facts.json advertised a
test count six times the real one.

Warns on stderr (via sitelib.warn, which build.py turns into a build failure)
whenever the generator runs and fails, when the main tree is present but the
generator file is gone, and when neither the generator nor the committed snapshot
can be read.

Usage:
    tools/render-test-health.py
"""

import html
import json
import pathlib
import subprocess
import sys

import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
MAIN = GH_PAGES.parent / "main"
LATEST = MAIN / "test" / "health" / "latest.json"
HISTORY = MAIN / "test" / "health" / "history.ndjson"
# The canonical Markdown deliverable, generated and committed in the main
# repository. The site mirrors it; it is never authored here.
PAGE_MD = MAIN / "docs" / "features" / "test-health.md"
DEST = GH_PAGES / "quality" / "health" / "index.html"

# The single generator in the main repository. The website REGENERATES the
# numbers from it at build time (read-only: --json and --emit-page write no file
# and touch no ratchet baseline) so the published page reflects the tree being
# built, not whatever was committed at the last `make ze-test-health`. The
# committed test/health/latest.json + docs/features/test-health.md remain the
# fallback for when the generator cannot run against a present main tree.
GENERATOR = MAIN / "scripts" / "dev" / "testing_health.py"


def _generate(mode):
    """Run the main-repo generator read-only and return its stdout, or None.

    `mode` is "--json" (the metric record) or "--emit-page" (the page Markdown).
    Returns None (with a warning) when the generator cannot run, so callers fall
    back to the committed artifact rather than publishing nothing.
    """
    if not GENERATOR.exists():
        # No main tree at all -> silent: load()/page_markdown() will find the
        # fallback absent too and fail the build loudly. But a PRESENT main tree
        # with the generator gone is an anomaly that would silently serve
        # possibly-stale committed numbers, so warn on that.
        if MAIN.exists():
            sitelib.warn(
                "test-health: generator %s is missing though the main tree is "
                "present; serving the committed snapshot, which may be stale"
                % GENERATOR
            )
        return None
    try:
        proc = subprocess.run(
            [sys.executable, str(GENERATOR), mode, "--root", str(MAIN)],
            capture_output=True,
            text=True,
            timeout=600,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        sitelib.warn("test-health: could not run the main generator (%s)" % exc)
        return None
    if proc.returncode != 0:
        sitelib.warn(
            "test-health: `%s %s` failed (%s); falling back to the committed copy"
            % (GENERATOR.name, mode, proc.stderr.strip())
        )
        return None
    return proc.stdout


# Minimum samples before a series is drawn. Mirrors MIN_SAMPLES in
# testing_health.py: a line through three points is noise with a direction.
MIN_SAMPLES = 4

QUESTIONS = [
    ("Q1", "Sensitivity", "If the code were wrong, would something go red?"),
    (
        "Q2",
        "Intent coverage",
        "Are the things that matter checked, or only the happy path?",
    ),
    ("Q3", "Integrity", "When something goes red, does it stop the line?"),
]

# unknown first: a number nobody is computing is worse than a bad number.
#
# An UNRECOGNISED status is treated as unknown for both ordering and labelling.
# Defaulting it to a rank below `ok` put a typo'd or newly-introduced status at
# the bottom of the attention table with an empty chip and no coloured border --
# sensor rot presenting as calm, the exact failure this ordering exists to stop.
STATUS_ORDER = {"unknown": 0, "warn": 1, "ok": 2}
UNKNOWN_RANK = 0


def rank(status):
    return STATUS_ORDER.get(status, UNKNOWN_RANK)


def status_class(status):
    return status if status in STATUS_ORDER else "unknown"


STATUS_LABEL = {
    "ok": "Within threshold",
    "warn": "Needs attention",
    "unknown": "Not measured",
}


def _parse_record(text, source):
    """Validate a {metrics:[...], history:[...]} record. None on malformed."""
    try:
        record = json.loads(text)
        metrics = record["metrics"]
        if not isinstance(metrics, list) or not all(
            isinstance(m, dict) for m in metrics
        ):
            raise TypeError("'metrics' must be a list of objects")
    except (ValueError, KeyError, TypeError) as exc:
        sitelib.warn("test-health: %s is unreadable (%s)" % (source, exc))
        return None
    history = record.get("history") if isinstance(record, dict) else None
    return metrics, (history if isinstance(history, list) else None)


def load():
    # Regenerate from the main tree being built, so the published numbers are
    # current rather than as-of-last-commit. --json is read-only.
    fresh = _generate("--json")
    if fresh is not None:
        parsed = _parse_record(fresh, "the main generator (--json)")
        if parsed is not None:
            metrics, history = parsed
            if history is not None:
                return metrics, history
            # Record carried no history; fall through to read the file below.
            return metrics, _read_history()

    # Fallback: the committed snapshot (a checkout without the main tree, or a
    # generator that could not run -- both already warned).
    if not LATEST.exists():
        sitelib.warn(
            "test-health: %s missing -- run `make ze-test-health` in ../main" % LATEST
        )
        return None, []
    parsed = _parse_record(LATEST.read_text(), str(LATEST))
    if parsed is None:
        return None, []
    metrics, history = parsed
    return metrics, (history if history is not None else _read_history())


def _read_history():
    history = []
    if HISTORY.exists():
        for line in HISTORY.read_text().splitlines():
            line = line.strip()
            if not line:
                continue
            try:
                history.append(json.loads(line))
            except ValueError as exc:
                sitelib.warn(
                    "test-health: %s has an unparseable line (%s)" % (HISTORY, exc)
                )
                return []
    return history


def sparkline(values, width=260, height=44):
    """Inline SVG. No chart library: the site requires content that is
    meaningful without JavaScript, and a strict CSP blocks external scripts."""
    if len(values) < 2:
        return ""
    lo, hi = min(values), max(values)
    span = (hi - lo) or 1
    step = width / (len(values) - 1)
    points = " ".join(
        "%.1f,%.1f" % (i * step, height - ((v - lo) / span) * (height - 6) - 3)
        for i, v in enumerate(values)
    )
    return (
        '<svg class="th-spark" viewBox="0 0 %d %d" width="%d" height="%d" role="img" '
        'aria-label="trend over %d samples, low %s, high %s">'
        '<polyline points="%s" fill="none" stroke="currentColor" stroke-width="2" '
        'stroke-linejoin="round" stroke-linecap="round"/></svg>'
        % (width, height, width, height, len(values), lo, hi, points)
    )


def meter(metric):
    """A proportion bar for metrics that carry numerator and denominator.

    Both parts are always printed beside it. A bar alone would hide the case
    where a ratio improves only because its denominator shrank.
    """
    for key in ("proof_density", "inert", "kill_rate", "overall", "unproven"):
        part = metric.get(key)
        if isinstance(part, dict) and part.get("percent") is not None:
            pct = part["percent"]
            # Escape both parts. They land in an attribute AND in body text, and
            # they arrive as JSON from another repository: the only two
            # unescaped interpolations in this file were here, on a line where
            # seven neighbours were escaped.
            num = html.escape(str(part.get("numerator", "?")))
            den = html.escape(str(part.get("denominator", "?")))
            try:
                width = float(pct)
            except (TypeError, ValueError):
                continue
            return (
                '<div class="th-meter" role="img" aria-label="%s of %s">'
                '<span class="th-meter-fill" style="width:%.1f%%"></span></div>'
                '<p class="th-meter-note">%s of %s (%.1f%%)</p>'
                % (num, den, width, num, den, width)
            )
    return ""


def detail_table(metric):
    rows = [r for r in (metric.get("worst") or []) if isinstance(r, dict)]
    if rows:
        # Union of keys, in first-seen order: taking them from rows[0] alone
        # raised KeyError the moment a later row carried a different shape, and
        # a malformed collector must not take the whole site build down.
        keys = []
        for row in rows:
            for k in row:
                if k not in keys:
                    keys.append(k)
        head = "".join("<th>%s</th>" % html.escape(k.replace("_", " ")) for k in keys)
        body = "".join(
            "<tr>%s</tr>"
            % "".join(
                "<td><code>%s</code></td>" % html.escape(str(r.get(k, "")))
                for k in keys
            )
            for r in rows
        )
        return "<table><thead><tr>%s</tr></thead><tbody>%s</tbody></table>" % (
            head,
            body,
        )
    orphans = metric.get("orphans") or []
    if orphans:
        body = "".join(
            "<tr><td><code>%s</code></td><td><code>%s</code></td></tr>"
            % (
                html.escape(str(o.get("file", "?"))),
                html.escape(str(o.get("requires", "?"))),
            )
            for o in orphans
            if isinstance(o, dict)
        )
        return (
            "<table><thead><tr><th>File</th><th>Requires</th></tr></thead>"
            "<tbody>%s</tbody></table>" % body
        )
    buckets = metric.get("buckets") or {}
    if buckets:
        body = "".join(
            "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>"
            % (
                html.escape(str(year)),
                html.escape(str(b.get("packages", "?"))),
                html.escape(str(b.get("with_fuzz", "?"))),
                html.escape(str(b.get("with_rfc_tag", "?"))),
                html.escape(str(b.get("with_ci", "?"))),
            )
            for year, b in sorted(buckets.items())
            if isinstance(b, dict)
        )
        return (
            "<table><thead><tr><th>Package first commit</th><th>Packages with tests</th>"
            "<th>With a fuzz target</th><th>With an RFC-tagged test</th>"
            "<th>With a .ci scenario</th></tr></thead>"
            "<tbody>%s</tbody></table>" % body
        )
    return ""


def render_metric(metric):
    return "\n".join(
        [
            '<article class="th-card th-%s">'
            % html.escape(status_class(metric.get("status"))),
            "  <h3>%s</h3>" % html.escape(str(metric.get("label", "(unnamed metric)"))),
            '  <p class="th-value">%s <span class="th-status">%s</span></p>'
            % (
                html.escape(str(metric.get("value", "unknown"))),
                STATUS_LABEL.get(status_class(metric.get("status")), "Not measured"),
            ),
            "  " + meter(metric),
            "  <p>%s</p>" % html.escape(str(metric.get("detail") or "")),
            '  <p class="th-action"><strong>If this degrades:</strong> %s</p>'
            % html.escape(str(metric.get("action") or "")),
            "  " + detail_table(metric),
            "</article>",
        ]
    )


def render_attention(metrics):
    problems = [m for m in metrics if m.get("status") != "ok"]
    if not problems:
        return (
            '<section class="th-attention"><h2>Needs attention</h2>'
            "<p>Nothing outstanding. Every metric is within its threshold.</p></section>"
        )
    rows = "".join(
        "<tr><td>%s</td><td>%s</td><td><strong>%s</strong></td><td>%s</td></tr>"
        % (
            html.escape(str(m.get("label", "(unnamed metric)"))),
            html.escape(str(m.get("question", ""))),
            html.escape(str(m.get("value", "unknown"))),
            html.escape(str(m.get("action") or "")),
        )
        for m in sorted(problems, key=lambda m: rank(m.get("status")))
    )
    return (
        '<section class="th-attention"><h2>Needs attention</h2>'
        "<table><thead><tr><th>Metric</th><th>Question</th><th>Value</th>"
        "<th>What to do</th></tr></thead><tbody>%s</tbody></table></section>" % rows
    )


def render_trends(history):
    series_defs = [
        ("rfc_proof_percent", "RFC proof density %"),
        ("assert_nothing", "Tests that cannot fail"),
        ("tag_orphan", "Test files nothing runs"),
        ("mutation_percent", "Mutation kill %"),
    ]
    rows = []
    for key, label in series_defs:
        values = [h[key] for h in history if h.get(key) is not None]
        if len(values) < MIN_SAMPLES:
            rows.append(
                "<tr><td>%s</td><td><em>insufficient data</em></td><td>-</td><td>%d</td></tr>"
                % (html.escape(label), len(values))
            )
            continue
        rows.append(
            "<tr><td>%s</td><td>%s</td><td><code>%s</code></td><td>%d</td></tr>"
            % (html.escape(label), sparkline(values), values[-1], len(values))
        )
    return (
        '<section class="th-trends"><h2>Evolution over time</h2>'
        "<p>Each row shows its sample count. A statistic without its <em>n</em> is "
        "an assertion, not a measurement, and a trend drawn through three points is "
        "noise with a direction.</p>"
        "<table><thead><tr><th>Series</th><th>Trend</th><th>Latest</th>"
        "<th>Samples</th></tr></thead><tbody>%s</tbody></table></section>"
        % "".join(rows)
    )


STYLE = """
<style>
.th-card { border-radius: 14px; padding: 1.1rem 1.25rem; margin: 0 0 1rem; background: var(--surface, #fff); box-shadow: 0 2px 0 rgba(0,0,0,.06); border: 2px solid rgba(255,255,255,.8); }
.th-card h3 { margin: 0 0 .35rem; font-size: 1.05rem; }
.th-value { font-size: 1.5rem; font-weight: 700; margin: .2rem 0 .5rem; }
.th-status { font-size: .8rem; font-weight: 600; opacity: .7; margin-left: .5rem; }
.th-warn { border-left: 6px solid #e2a33c; }
.th-unknown { border-left: 6px solid #8b8b8b; }
.th-ok { border-left: 6px solid #57a773; }
.th-meter { display: block; height: 10px; border-radius: 6px; background: rgba(0,0,0,.08); overflow: hidden; }
.th-meter-fill { display: block; height: 100%; background: currentColor; opacity: .55; }
.th-meter-note { font-size: .85rem; opacity: .75; margin: .3rem 0 .6rem; }
.th-action { font-size: .9rem; }
.th-spark { vertical-align: middle; }
.th-attention table, .th-trends table, .th-card table { width: 100%; border-collapse: collapse; }
.th-card table { font-size: .85rem; margin-top: .6rem; }
.th-card td, .th-card th, .th-attention td, .th-attention th, .th-trends td, .th-trends th { text-align: left; padding: .35rem .5rem; border-bottom: 1px solid rgba(0,0,0,.08); }
</style>
"""


def page_markdown():
    """The Markdown sibling, from the ONE generator in the main repository.

    It is never authored here. It used to COMPOSE its own summary table, which
    made quality/health/index.md a second, independently authored document about
    the same subject -- the two-sources-of-truth defect this whole feature
    exists to remove. So this asks the main generator for the page bytes.

    Regenerated at build time (`--emit-page` is read-only) so the published
    mirror carries current numbers, with the committed docs/features/test-health.md
    as the fallback for a checkout without the main tree.
    """
    fresh = _generate("--emit-page")
    if fresh is not None:
        return fresh
    if not PAGE_MD.exists():
        sitelib.warn(
            "test-health: %s missing -- run `make ze-test-health` in ../main" % PAGE_MD
        )
        return None
    return PAGE_MD.read_text()


def render(metrics, history):
    root = "../../"
    title = "Testing Health - Ze"
    desc = (
        "Whether a regression would be caught: proof density, tests that cannot "
        "fail, tests nothing runs, and how they move over time."
    )
    out = [
        sitelib.page_head(
            title, desc, root, og_title=title, og_desc=desc, page_key="quality/health/"
        )
    ]
    out.append(
        '            <section aria-labelledby="test-health-title" class="md-content reveal cat-observe">'
    )
    out.append(
        sitelib.page_hero(
            "Testing Health",
            (
                "Not how many tests exist, but whether a regression would be caught. "
                "A suite can grow forever while the share of behaviour it actually "
                "pins falls, and no count of tests can show that. Every metric here "
                "belongs to one of three questions; anything belonging to none is "
                "volume, and is deliberately absent."
            ),
            "Observe",
            h1_id="test-health-title",
            lead_html=True,
        )
    )
    out.append(STYLE)
    out.append(render_attention(metrics))

    for qkey, qtitle, qtext in QUESTIONS:
        group = [m for m in metrics if m.get("question", "") == qkey]
        if not group:
            continue
        out.append(
            "<section><h2>%s</h2><p><em>%s</em></p>" % (qtitle, html.escape(qtext))
        )
        for metric in sorted(group, key=lambda m: rank(m.get("status"))):
            out.append(render_metric(metric))
        out.append("</section>")

    out.append(render_trends(history))
    out.append("            </section>")

    body = "\n".join(out)
    DEST.parent.mkdir(parents=True, exist_ok=True)
    DEST.write_text(body + "\n" + sitelib.page_foot(root))
    mirror = page_markdown()
    if mirror is not None:
        sitelib.write_markdown_sibling(DEST, mirror)
    print(
        "rendered %d testing-health metrics (%d KPI samples) -> %s (+ index.md)"
        % (len(metrics), len(history), DEST)
    )


def main():
    metrics, history = load()
    if metrics is None:
        return 1
    render(metrics, history)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
