#!/usr/bin/env -S uv run python3
"""Render searchable cross-vendor command equivalents.

The index is intentionally compact: one row per live Ze command, with vendor
commands side by side. Each row links to a detail page for that Ze command.
The Ze command list comes from `ze help command --json`; the vendor mapping is
curated in `data/command-equivalents.json` with provenance and confidence.
"""

import html
import importlib.util
import json
import pathlib
import re
import shutil
import sys

import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
DATA = GH_PAGES / "data" / "command-equivalents.json"
DEST = GH_PAGES / "command-equivalents" / "index.html"
DETAIL_ROOT = GH_PAGES / "command-equivalents"
SLUG_RE = re.compile(r"[^a-z0-9]+")
CONFIDENCE_LABELS = {
    "verified": "verified",
    "local-seed": "seed",
    "legacy": "legacy",
    "unknown": "unknown",
}
CONFIDENCE_ORDER = {"verified": 0, "local-seed": 1, "legacy": 2, "unknown": 3}
MODE_LABELS = {
    "daemon": "Daemon",
    "read-only": "Read-only",
    "offline": "Offline",
}


def slugify(prefix, text):
    return prefix + SLUG_RE.sub("-", text.lower()).strip("-")


def one_line(text):
    return " ".join(str(text).split())


def md_cell(text):
    return one_line(text).replace("|", "\\|")


def load_module(name):
    path = HERE / (name + ".py")
    spec = importlib.util.spec_from_file_location(name.replace("-", "_"), path)
    if spec is None or spec.loader is None:
        raise RuntimeError("cannot load %s" % path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def load_inputs():
    if not DATA.exists():
        print("error: %s does not exist" % DATA, file=sys.stderr)
        return None, None, 1
    render_cli_catalog = load_module("render-cli-catalog")
    mapping = json.loads(DATA.read_text())
    commands = render_cli_catalog.load_commands()
    errors = validate_mapping(mapping, commands)
    if errors:
        for err in errors:
            print("error: " + err, file=sys.stderr)
        return None, None, 1
    return mapping, commands, 0


def source_map(mapping):
    sources = dict(mapping.get("sources", {}))
    for vendor in mapping.get("vendors", {}).values():
        for doc in vendor.get("documentation", []):
            sources[doc["id"]] = doc
    return sources


def validate_mapping(mapping, commands):
    errors = []
    if mapping.get("schema-version") != 1:
        errors.append("schema-version must be 1")
    vendors = mapping.get("vendors", {})
    if not vendors:
        errors.append("vendors must not be empty")
    command_paths = {c["path"] for c in commands}
    sources = source_map(mapping)
    vendor_source_ids = {
        vendor_id: {doc["id"] for doc in vendor.get("documentation", [])}
        for vendor_id, vendor in vendors.items()
    }
    seen_entry_ids = set()
    for entry in mapping.get("entries", []):
        entry_id = entry.get("id")
        if not entry_id:
            errors.append("entry without id")
            continue
        if entry_id in seen_entry_ids:
            errors.append("duplicate entry id %s" % entry_id)
        seen_entry_ids.add(entry_id)
        if not entry.get("category"):
            errors.append("%s missing category" % entry_id)
        if not entry.get("intent"):
            errors.append("%s missing intent" % entry_id)
        for path in entry.get("ze", []):
            if path not in command_paths:
                errors.append("%s references stale Ze command %r" % (entry_id, path))
        unknown_vendors = set(entry.get("vendors", {})) - set(vendors)
        for vendor_id in sorted(unknown_vendors):
            errors.append("%s references unknown vendor %s" % (entry_id, vendor_id))
        for vendor_id, rows in entry.get("vendors", {}).items():
            if not isinstance(rows, list):
                errors.append("%s vendor %s must be a list" % (entry_id, vendor_id))
                continue
            for idx, row in enumerate(rows):
                command = row.get("command")
                if not command:
                    errors.append("%s vendor %s row %d missing command" % (entry_id, vendor_id, idx + 1))
                confidence = row.get("confidence", "unknown")
                if confidence not in CONFIDENCE_LABELS:
                    errors.append("%s vendor %s command %r has unknown confidence %r" % (entry_id, vendor_id, command, confidence))
                refs = row.get("source-refs", [])
                if not refs:
                    errors.append("%s vendor %s command %r must cite an external vendor source" % (entry_id, vendor_id, command))
                for ref in refs:
                    if ref not in sources:
                        errors.append("%s vendor %s command %r references unknown source %s" % (entry_id, vendor_id, command, ref))
                    if ref not in vendor_source_ids.get(vendor_id, set()):
                        errors.append("%s vendor %s command %r uses non-vendor source %s" % (entry_id, vendor_id, command, ref))
    return errors


def command_slug(command):
    return slugify("", command["path"])

def command_display_path(command):
    if command.get("syntax"):
        return command["syntax"]
    args = command.get("args", [])
    arg_by_name = {arg.get("name"): arg for arg in args if arg.get("name")}
    used = set()
    parts = []
    for token in command["path"].split():
        if token in arg_by_name:
            used.add(token)
            parts.append("<%s>" % token)
        else:
            parts.append(token)
    for arg in args:
        name = arg.get("name")
        if not name or name in used:
            continue
        marker = "<%s>" % name
        if not arg.get("mandatory"):
            marker = "[%s]" % marker
        parts.append(marker)
    return " ".join(parts)


def command_group(path):
    parts = path.split()
    if not parts:
        return "other"
    return parts[0]


def build_rows(mapping, commands):
    entries_by_path = {c["path"]: [] for c in commands}
    vendor_only = []
    for entry in mapping.get("entries", []):
        ze_paths = entry.get("ze", [])
        if not ze_paths:
            vendor_only.append(entry)
            continue
        for path in ze_paths:
            entries_by_path.setdefault(path, []).append(entry)

    rows = []
    for command in sorted(commands, key=lambda c: c["path"]):
        rows.append(
            {
                "command": command,
                "slug": command_slug(command),
                "group": command_group(command["path"]),
                "entries": entries_by_path.get(command["path"], []),
            }
        )
    return rows, vendor_only


def group_rows(rows):
    grouped = {}
    for row in rows:
        grouped.setdefault(row["group"], []).append(row)
    return [(label, grouped[label]) for label in sorted(grouped)]


def vendor_commands(row, vendor_id):
    seen = set()
    commands = []
    for entry in row["entries"]:
        for item in entry.get("vendors", {}).get(vendor_id, []):
            key = (item.get("command", ""), item.get("confidence", "unknown"), tuple(item.get("source-refs", [])))
            if key in seen:
                continue
            seen.add(key)
            commands.append((entry, item))
    commands.sort(key=lambda pair: (CONFIDENCE_ORDER.get(pair[1].get("confidence", "unknown"), 9), pair[1].get("command", "")))
    return commands


def source_links(refs, sources):
    links = []
    for ref in refs:
        source = sources.get(ref)
        if not source:
            continue
        links.append(
            '<a href="%s">%s</a>'
            % (html.escape(source.get("url", ""), quote=True), html.escape(source.get("label", ref)))
        )
    if not links:
        return ""
    return '<span class="cmd-sources">source: %s</span>' % ", ".join(links)


def confidence_badge(confidence):
    label = CONFIDENCE_LABELS.get(confidence, confidence)
    return '<span class="cmd-confidence cmd-confidence-%s">%s</span>' % (
        html.escape(confidence),
        html.escape(label),
    )


def compact_vendor_cell(row, vendor_id):
    commands = vendor_commands(row, vendor_id)
    if not commands:
        return '<span class="cmd-no-equivalent">-</span>'
    out = []
    for _, item in commands:
        out.append('<code>%s</code>' % html.escape(item["command"]))
    return "".join('<div class="cmd-compact-command">%s</div>' % item for item in out)


def search_text(row, vendor_ids):
    command = row["command"]
    terms = [command["path"], command_display_path(command), command.get("description", ""), command.get("mode", ""), command.get("wire-method", ""), row["group"]]
    for entry in row["entries"]:
        terms.extend([entry.get("intent", ""), entry.get("category", ""), entry.get("notes", "")])
        for vendor_id in vendor_ids:
            for _, item in vendor_commands(row, vendor_id):
                terms.extend([vendor_id, item.get("command", ""), item.get("confidence", "")])
    return one_line(" ".join(terms)).lower()


def render_index_row(row, vendor_ids):
    command = row["command"]
    mode = command.get("mode", "")
    mode_label = MODE_LABELS.get(mode, mode)
    cells = [
        '<td class="cmd-eq-ze"><a href="%s/"><code>%s</code></a><span class="cmd-mode">%s</span></td>'
        % (html.escape(row["slug"], quote=True), html.escape(command_display_path(command)), html.escape(mode_label)),
    ]
    cells.extend('<td>%s</td>' % compact_vendor_cell(row, vendor_id) for vendor_id in vendor_ids)
    cells.append('<td class="cmd-eq-detail-link"><a href="%s/">details</a></td>' % html.escape(row["slug"], quote=True))
    return '<tr id="cmd-eq-%s" data-search="%s">%s</tr>' % (
        html.escape(row["slug"], quote=True),
        html.escape(search_text(row, vendor_ids), quote=True),
        "".join(cells),
    )


def render_index_group(label, rows, vendor_ids, vendor_labels):
    parts = [
        '<details class="cmd-eq-group" id="%s" open>' % slugify("cmd-eq-group-", label),
        '<summary>%s <span class="cmd-eq-count">%d</span></summary>' % (html.escape(label), len(rows)),
        '<div class="cmd-eq-table-wrap">',
        '<table class="cmd-eq-table cmd-eq-compact"><thead><tr><th>Ze</th>',
    ]
    for vendor_id in vendor_ids:
        parts.append("<th>%s</th>" % html.escape(vendor_labels[vendor_id]))
    parts.append("<th>Details</th></tr></thead><tbody>")
    parts.extend(render_index_row(row, vendor_ids) for row in rows)
    parts.append("</tbody></table></div></details>")
    return "\n".join(parts)


def render_sources(mapping):
    parts = ["<details class=\"cmd-eq-sources\"><summary>Data sources and confidence</summary>"]
    parts.append("<p>Source links and confidence labels are shown on each command detail page. The index only shows side-by-side command text.</p>")
    if mapping.get("sources"):
        parts.append("<h2>Local sources</h2><ul>")
        for ref, source in sorted(mapping["sources"].items()):
            parts.append('<li><a href="%s">%s</a> <code>%s</code></li>' % (html.escape(source["url"], quote=True), html.escape(source["label"]), html.escape(ref)))
        parts.append("</ul>")
    parts.append("<h2>Vendor documents</h2>")
    for vendor in mapping.get("vendors", {}).values():
        parts.append("<h3>%s</h3><ul>" % html.escape(vendor["label"]))
        for doc in vendor.get("documentation", []):
            parts.append('<li><a href="%s">%s</a> <code>%s</code></li>' % (html.escape(doc["url"], quote=True), html.escape(doc["label"]), html.escape(doc["id"])))
        parts.append("</ul>")
    parts.append("</details>")
    return "\n".join(parts)


def render_vendor_only(vendor_only, vendor_ids, vendor_labels):
    if not vendor_only:
        return ""
    parts = [
        '<details class="cmd-eq-group cmd-eq-vendor-only" id="cmd-eq-vendor-only" open>',
        "<summary>Vendor-only gaps</summary>",
        '<div class="cmd-eq-table-wrap">',
        '<table class="cmd-eq-table cmd-eq-compact"><thead><tr><th>Intent</th>',
    ]
    for vendor_id in vendor_ids:
        parts.append("<th>%s</th>" % html.escape(vendor_labels[vendor_id]))
    parts.append("<th>Notes</th></tr></thead><tbody>")
    for entry in vendor_only:
        cells = [
            '<td><strong>%s</strong><span class="cmd-mode">%s</span></td>'
            % (html.escape(entry.get("intent", "")), html.escape(entry.get("category", "")))
        ]
        for vendor_id in vendor_ids:
            rows = entry.get("vendors", {}).get(vendor_id, [])
            if not rows:
                cells.append('<td><span class="cmd-no-equivalent">-</span></td>')
                continue
            cells.append("<td>%s</td>" % "".join('<div class="cmd-compact-command"><code>%s</code></div>' % html.escape(row.get("command", "")) for row in rows))
        cells.append("<td>%s</td>" % html.escape(entry.get("notes", "")))
        search = one_line(" ".join([entry.get("intent", ""), entry.get("category", ""), entry.get("notes", "")] + [row.get("command", "") for rows in entry.get("vendors", {}).values() for row in rows])).lower()
        parts.append('<tr data-search="%s">%s</tr>' % (html.escape(search, quote=True), "".join(cells)))
    parts.append("</tbody></table></div></details>")
    return "\n".join(parts)


def render_index(rows, groups, vendor_only, mapping, commands, vendor_ids, vendor_labels):
    mapped_count = len([row for row in rows if row["entries"]])
    title = "Command Equivalents - Ze"
    desc = "One-line Ze command map with side-by-side Junos MX, IOS XR, Nokia SR OS, and VyOS equivalents."
    root = "../"
    out = [
        sitelib.page_head(
            title,
            desc,
            root,
            og_title=title,
            og_desc=desc,
            page_key="command-equivalents/",
        )
    ]
    out.append('<section aria-labelledby="command-equivalents-title" class="md-content command-equivalents reveal cat-operate">')
    out.append(
        sitelib.page_hero(
            "Command Equivalents",
            "One line per live Ze command. Vendor commands sit side by side as curated migration hints, not as exhaustive vendor CLI catalogs; follow <strong>details</strong> for descriptions, notes, confidence, and sources.",
            "Reference",
            h1_id="command-equivalents-title",
            lead_html=True,
        )
    )
    out.append('<div class="cmd-eq-stats">')
    out.append('<div><strong>%d</strong><span>live Ze commands</span></div>' % len(commands))
    out.append('<div><strong>%d</strong><span>commands with vendor mappings</span></div>' % mapped_count)
    out.append('<div><strong>%d</strong><span>curated mapping intents</span></div>' % len(mapping.get("entries", [])))
    out.append('<div><strong>%d</strong><span>vendor-only gap notes</span></div>' % len(vendor_only))
    out.append("</div>")
    out.append('<div class="cmd-eq-search-wrap">')
    out.append('<input id="cmd-eq-search" type="search" autocomplete="off" placeholder="Search Ze, Junos, IOS XR, SR OS, or VyOS commands..." aria-label="Search command equivalents" />')
    out.append('<div id="cmd-eq-search-count" class="cmd-eq-search-count" aria-live="polite"></div>')
    out.append("</div>")
    out.append('<noscript><p>JavaScript is disabled. Browser find works across the side-by-side command table.</p></noscript>')
    for label, grouped_rows in groups:
        out.append(render_index_group(label, grouped_rows, vendor_ids, vendor_labels))
    out.append(render_vendor_only(vendor_only, vendor_ids, vendor_labels))
    out.append(render_sources(mapping))
    out.append("</section>")
    body = "\n".join(out)
    DEST.parent.mkdir(parents=True, exist_ok=True)
    DEST.write_text(body + "\n" + FILTER_SCRIPT + "\n" + sitelib.page_foot(root))
    sitelib.write_markdown_sibling(DEST, render_index_markdown(rows, groups, vendor_only, vendor_ids, vendor_labels, commands, mapped_count))


def render_args(command):
    args = command.get("args", [])
    if not args:
        return "<p>No command-specific arguments listed.</p>"
    out = ["<table class=\"cmd-args\"><thead><tr><th>Name</th><th>Type</th><th>Required</th></tr></thead><tbody>"]
    for arg in args:
        out.append(
            "<tr><td><code>%s</code></td><td>%s</td><td>%s</td></tr>"
            % (
                html.escape(arg.get("name", "")),
                html.escape(arg.get("type", "")),
                "yes" if arg.get("mandatory") else "no",
            )
        )
    out.append("</tbody></table>")
    return "\n".join(out)


def render_ze_detail(command):
    mode = MODE_LABELS.get(command.get("mode", ""), command.get("mode", ""))
    out = ['<article class="cmd-detail-card cmd-detail-ze"><h2>Ze command</h2>']
    out.append('<dl class="cmd-meta">')
    out.append('<div><dt>Syntax</dt><dd><code>%s</code></dd></div>' % html.escape(command_display_path(command)))
    out.append('<div><dt>Registry path</dt><dd><code>%s</code></dd></div>' % html.escape(command["path"]))
    out.append('<div><dt>Mode</dt><dd>%s</dd></div>' % html.escape(mode or "not listed"))
    out.append('<div><dt>Wire method</dt><dd><code>%s</code></dd></div>' % html.escape(command.get("wire-method", "not listed")))
    out.append('<div><dt>Global pipes</dt><dd>%s</dd></div>' % ("yes" if command.get("global-pipes") else "no"))
    out.append("</dl>")
    out.append('<h3>Description</h3><p>%s</p>' % html.escape(command.get("description", "No description listed.")).replace("\n", "<br>"))
    out.append("<h3>Arguments</h3>")
    out.append(render_args(command))
    out.append("</article>")
    return "\n".join(out)


def render_mapping_notes(row):
    if not row["entries"]:
        return '<article class="cmd-detail-card"><h2>Mapping status</h2><p>No vendor equivalent has been curated yet for this Ze command.</p></article>'
    out = ['<article class="cmd-detail-card"><h2>Mapping intents</h2>']
    for entry in row["entries"]:
        out.append('<section class="cmd-intent"><h3>%s</h3>' % html.escape(entry.get("intent", "Mapping")))
        out.append('<p><strong>Category:</strong> %s</p>' % html.escape(entry.get("category", "uncategorized")))
        if entry.get("notes"):
            out.append('<p>%s</p>' % html.escape(entry["notes"]))
        out.append("</section>")
    out.append("</article>")
    return "\n".join(out)


def render_vendor_detail(row, vendor_id, vendor, sources):
    out = ['<article class="cmd-detail-card cmd-vendor-detail"><h2>%s</h2>' % html.escape(vendor["label"])]
    any_rows = False
    for entry in row["entries"]:
        rows = entry.get("vendors", {}).get(vendor_id, [])
        if not rows:
            continue
        any_rows = True
        out.append('<section class="cmd-vendor-intent"><h3>%s</h3>' % html.escape(entry.get("intent", "Equivalent")))
        for item in sorted(rows, key=lambda r: (CONFIDENCE_ORDER.get(r.get("confidence", "unknown"), 9), r.get("command", ""))):
            out.append('<div class="cmd-vendor-line"><code>%s</code> %s %s</div>' % (
                html.escape(item["command"]),
                confidence_badge(item.get("confidence", "unknown")),
                source_links(item.get("source-refs", []), sources),
            ))
            if item.get("notes"):
                out.append('<p class="cmd-note">%s</p>' % html.escape(item["notes"]))
        out.append("</section>")
    if not any_rows:
        out.append('<p class="cmd-no-equivalent">No equivalent is listed for this vendor yet.</p>')
    out.append("</article>")
    return "\n".join(out)


def render_detail_page(row, mapping, vendor_ids, vendor_labels, sources):
    command = row["command"]
    root = "../../"
    title = "%s - Command Equivalents - Ze" % command_display_path(command)
    desc = "Command details and vendor equivalents for %s." % command_display_path(command)
    out = [
        sitelib.page_head(
            title,
            desc,
            root,
            og_title=title,
            og_desc=desc,
            page_key="command-equivalents/%s/" % row["slug"],
        )
    ]
    out.append('<section class="md-content command-equivalents command-equivalent-detail reveal cat-operate" aria-labelledby="command-equivalent-detail-title">')
    out.append(
        sitelib.page_hero(
            "<code>%s</code>" % html.escape(command_display_path(command)),
            "Command details and vendor equivalents for %s."
            % html.escape(command_display_path(command)),
            "Command map",
            h1_id="command-equivalent-detail-title",
            title_html=True,
        )
    )
    out.append('<div class="cmd-detail-grid">')
    out.append(render_ze_detail(command))
    out.append(render_mapping_notes(row))
    for vendor_id in vendor_ids:
        out.append(render_vendor_detail(row, vendor_id, mapping["vendors"][vendor_id], sources))
    out.append("</div></section>")
    detail_dest = DETAIL_ROOT / row["slug"] / "index.html"
    detail_dest.parent.mkdir(parents=True, exist_ok=True)
    detail_dest.write_text("\n".join(out) + "\n" + sitelib.page_foot(root))
    sitelib.write_markdown_sibling(detail_dest, render_detail_markdown(row, mapping, vendor_ids, vendor_labels))


def clean_stale_detail_dirs(rows):
    live_slugs = {row["slug"] for row in rows}
    if not DETAIL_ROOT.exists():
        return
    for child in DETAIL_ROOT.iterdir():
        if child.is_dir() and child.name not in live_slugs:
            shutil.rmtree(child)
            print("removed stale command detail -> %s" % child)


def render_index_markdown(rows, groups, vendor_only, vendor_ids, vendor_labels, commands, mapped_count):
    lines = [
        "# Command Equivalents",
        "",
        "%d live Ze commands. %d have curated vendor mappings. One row per Ze command. Vendor commands are curated migration hints, not exhaustive vendor CLI catalogs." % (len(commands), mapped_count),
        "",
        "| Ze | Mode | " + " | ".join(vendor_labels[vendor_id] for vendor_id in vendor_ids) + " | Details |",
        "| --- | --- | " + " | ".join("---" for _ in vendor_ids) + " | --- |",
    ]
    for _, grouped_rows in groups:
        for row in grouped_rows:
            command = row["command"]
            cells = ["`%s`" % md_cell(command_display_path(command)), md_cell(MODE_LABELS.get(command.get("mode", ""), command.get("mode", "")))]
            for vendor_id in vendor_ids:
                commands_for_vendor = ["`%s`" % md_cell(item["command"]) for _, item in vendor_commands(row, vendor_id)]
                cells.append("<br>".join(commands_for_vendor) if commands_for_vendor else "-")
            cells.append("[%s](%s/)" % ("details", row["slug"]))
            lines.append("| " + " | ".join(cells) + " |")
    if vendor_only:
        lines.extend(["", "## Vendor-only gaps", ""])
        lines.append("| Intent | " + " | ".join(vendor_labels[vendor_id] for vendor_id in vendor_ids) + " | Notes |")
        lines.append("| --- | " + " | ".join("---" for _ in vendor_ids) + " | --- |")
        for entry in vendor_only:
            cells = [md_cell(entry.get("intent", ""))]
            for vendor_id in vendor_ids:
                commands_for_vendor = ["`%s`" % md_cell(item.get("command", "")) for item in entry.get("vendors", {}).get(vendor_id, [])]
                cells.append("<br>".join(commands_for_vendor) if commands_for_vendor else "-")
            cells.append(md_cell(entry.get("notes", "")))
            lines.append("| " + " | ".join(cells) + " |")
    return "\n".join(lines).strip() + "\n"


def render_detail_markdown(row, mapping, vendor_ids, vendor_labels):
    command = row["command"]
    lines = [
        "# `%s`" % command_display_path(command),
        "",
        "## Ze command",
        "",
        "- Syntax: `%s`" % md_cell(command_display_path(command)),
        "- Registry path: `%s`" % md_cell(command["path"]),
        "- Mode: %s" % md_cell(MODE_LABELS.get(command.get("mode", ""), command.get("mode", ""))),
        "- Wire method: `%s`" % md_cell(command.get("wire-method", "not listed")),
        "- Global pipes: %s" % ("yes" if command.get("global-pipes") else "no"),
        "",
        one_line(command.get("description", "No description listed.")),
        "",
        "## Mapping intents",
        "",
    ]
    if not row["entries"]:
        lines.append("No vendor equivalent has been curated yet for this Ze command.")
    for entry in row["entries"]:
        lines.extend(["### %s" % entry.get("intent", "Mapping"), "", "Category: %s" % entry.get("category", "uncategorized"), ""])
        if entry.get("notes"):
            lines.extend([one_line(entry["notes"]), ""])
    lines.append("## Vendor equivalents")
    lines.append("")
    for vendor_id in vendor_ids:
        lines.append("### %s" % vendor_labels[vendor_id])
        rows = vendor_commands(row, vendor_id)
        if not rows:
            lines.extend(["", "No equivalent listed.", ""])
            continue
        for entry, item in rows:
            refs = ", ".join(item.get("source-refs", [])) or "no source ref"
            lines.append("- `%s` (%s, %s)" % (md_cell(item["command"]), item.get("confidence", "unknown"), refs))
            if entry.get("intent"):
                lines.append("  - Intent: %s" % md_cell(entry["intent"]))
            if item.get("notes"):
                lines.append("  - Note: %s" % md_cell(item["notes"]))
        lines.append("")
    return "\n".join(lines).strip() + "\n"


FILTER_SCRIPT = """        <script>
            document.addEventListener("DOMContentLoaded", function () {
                var input = document.getElementById("cmd-eq-search");
                var counter = document.getElementById("cmd-eq-search-count");
                var groups = Array.prototype.slice.call(document.querySelectorAll(".cmd-eq-group"));
                if (!input) return;

                function applyFilter() {
                    var q = input.value.trim().toLowerCase();
                    var visible = 0;
                    groups.forEach(function (group) {
                        var rows = Array.prototype.slice.call(group.querySelectorAll("tbody tr"));
                        var groupVisible = 0;
                        rows.forEach(function (row) {
                            var haystack = row.getAttribute("data-search") || row.textContent.toLowerCase();
                            var match = q === "" || haystack.indexOf(q) !== -1;
                            row.style.display = match ? "" : "none";
                            if (match) {
                                visible += 1;
                                groupVisible += 1;
                            }
                        });
                        group.style.display = groupVisible ? "" : "none";
                        if (q !== "") group.open = groupVisible > 0;
                    });
                    if (counter) {
                        counter.textContent = q === "" ? "" : visible + " matching commands";
                    }
                }

                input.addEventListener("input", applyFilter);
                if (location.hash) {
                    var target = document.getElementById(location.hash.slice(1));
                    if (target && target.tagName === "TR") {
                        var group = target.closest(".cmd-eq-group");
                        if (group) group.open = true;
                        window.setTimeout(function () {
                            target.scrollIntoView({ block: "center" });
                            target.classList.add("cmd-eq-highlight");
                            window.setTimeout(function () { target.classList.remove("cmd-eq-highlight"); }, 2000);
                        }, 50);
                    }
                }
            });
        </script>
"""


def main():
    mapping, commands, rc = load_inputs()
    if rc:
        return rc
    vendor_ids = list(mapping["vendors"].keys())
    vendor_labels = {vendor_id: mapping["vendors"][vendor_id].get("short-label", mapping["vendors"][vendor_id]["label"]) for vendor_id in vendor_ids}
    sources = source_map(mapping)
    rows, vendor_only = build_rows(mapping, commands)
    groups = group_rows(rows)
    render_index(rows, groups, vendor_only, mapping, commands, vendor_ids, vendor_labels)
    for row in rows:
        render_detail_page(row, mapping, vendor_ids, vendor_labels, sources)
    clean_stale_detail_dirs(rows)
    print(
        "rendered %d command rows and detail pages (%d live Ze commands, %d groups) -> %s (+ index.md)"
        % (len(rows), len(commands), len(groups), DEST)
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
