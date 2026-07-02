#!/usr/bin/env -S uv run python3
"""Render a live CLI command reference from `ze help command --json`.

Usage:
    tools/render-cli-catalog.py

Runs ../main/bin/ze help command --json -- the exact JSON the project's own
wiki command-catalog is generated from (see cmd/ze/help_command.go and
docs/guide/command-reference.md) -- caches it to data/cli-commands.json, and
renders cli/index.html grouped by command verb with a client-side filter.

The catalog is generated from the live binary's own command registry
(YANG dispatch tree + offline local commands), so it cannot go stale the
way a hand-maintained command table can. Run `make ze` in ../main, then
re-run this, to pick up new or changed commands.
"""

import html
import json
import pathlib
import subprocess
import sys

import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
ZE_BINARY = GH_PAGES.parent / "main" / "bin" / "ze"
DATA = GH_PAGES / "data" / "cli-commands.json"
DEST = GH_PAGES / "cli" / "index.html"

MODE_LABELS = {
    "daemon": "Daemon",
    "read-only": "Read-only",
    "offline": "Offline",
}

MAX_GROUP_SIZE = 20


def fetch_commands():
    if not ZE_BINARY.exists():
        print(
            "error: %s not found -- run `make ze` in ../main first" % ZE_BINARY,
            file=sys.stderr,
        )
        sys.exit(1)
    result = subprocess.run(
        [str(ZE_BINARY), "help", "command", "--json"], capture_output=True, text=True
    )
    if result.returncode != 0:
        print(
            "error: %s help command --json failed: %s" % (ZE_BINARY, result.stderr),
            file=sys.stderr,
        )
        sys.exit(1)
    commands = json.loads(result.stdout)
    DATA.write_text(json.dumps(commands, indent=2, ensure_ascii=False) + "\n")
    return commands


def load_commands():
    """Use the cached data/cli-commands.json if the binary isn't available
    (e.g. a checkout without ../main built) instead of hard-failing."""
    if ZE_BINARY.exists():
        return fetch_commands()
    if DATA.exists():
        print(
            "warning: %s not found, using cached %s" % (ZE_BINARY, DATA),
            file=sys.stderr,
        )
        return json.loads(DATA.read_text())
    print(
        "error: neither %s nor a cached %s exist -- run `make ze` in ../main first"
        % (ZE_BINARY, DATA),
        file=sys.stderr,
    )
    sys.exit(1)


def group_commands(commands):
    """Group by top-level verb; split verbs with more than MAX_GROUP_SIZE
    entries (e.g. "show" alone has 180+) into verb+subject subgroups so no
    single table is unwieldy."""
    by_verb = {}
    for c in commands:
        verb = c["path"].split(" ")[0]
        by_verb.setdefault(verb, []).append(c)

    groups = []
    for verb in sorted(by_verb):
        entries = by_verb[verb]
        if len(entries) <= MAX_GROUP_SIZE:
            groups.append((verb, sorted(entries, key=lambda c: c["path"])))
            continue
        by_subject = {}
        for c in entries:
            parts = c["path"].split(" ")
            subject = parts[1] if len(parts) > 1 else ""
            by_subject.setdefault(subject, []).append(c)
        for subject in sorted(by_subject):
            label = "%s %s" % (verb, subject) if subject else verb
            groups.append((label, sorted(by_subject[subject], key=lambda c: c["path"])))
    return groups


def render_row(c):
    desc_html = html.escape(c["description"]).replace("\n", "<br>")
    mode = c.get("mode", "")
    mode_label = MODE_LABELS.get(mode, mode)
    return (
        "<tr><td><code>%s</code></td>"
        '<td><span class="cli-mode cli-mode-%s">%s</span></td>'
        "<td>%s</td></tr>"
    ) % (html.escape(c["path"]), mode, mode_label, desc_html)


def render_group(label, entries):
    parts = [
        '<details class="cli-group" open>',
        '<summary>%s <span class="cli-group-count">%d</span></summary>'
        % (html.escape(label), len(entries)),
        "<table><thead><tr><th>Command</th><th>Mode</th><th>Description</th></tr></thead><tbody>",
    ]
    parts.extend(render_row(c) for c in entries)
    parts.append("</tbody></table></details>")
    return "\n".join(parts)


FILTER_SCRIPT = """        <script>
            document.addEventListener("DOMContentLoaded", function () {
                var input = document.getElementById("cli-search");
                var groups = document.querySelectorAll(".cli-group");
                if (!input) return;
                input.addEventListener("input", function () {
                    var q = input.value.trim().toLowerCase();
                    groups.forEach(function (group) {
                        var rows = group.querySelectorAll("tbody tr");
                        var anyVisible = false;
                        rows.forEach(function (row) {
                            var match = q === "" || row.textContent.toLowerCase().indexOf(q) !== -1;
                            row.style.display = match ? "" : "none";
                            if (match) anyVisible = true;
                        });
                        group.style.display = anyVisible ? "" : "none";
                        if (q !== "") {
                            group.open = anyVisible;
                        }
                    });
                });
            });
        </script>
"""


def render(commands):
    root = "../"
    groups = group_commands(commands)
    title = "CLI Reference - Ze"
    desc = (
        "Every ze command, generated live from the binary's own command "
        "registry -- %d commands across %d groups." % (len(commands), len(groups))
    )
    out = [sitelib.page_head(title, desc, root, og_title=title, og_desc=desc)]
    out.append(
        '            <section aria-labelledby="cli-title" class="md-content reveal cat-operate">'
    )
    out.append('                <h1 id="cli-title">CLI Reference</h1>')
    out.append(
        "                <p>%d commands across %d groups, generated straight from "
        "<code>ze help command --json</code> -- the same live command registry the "
        "binary itself uses, so this page cannot drift from what the binary actually "
        "supports the way a hand-maintained list can.</p>"
        % (len(commands), len(groups))
    )
    out.append(
        '                <input id="cli-search" type="search" placeholder="Filter commands (e.g. bgp, traceroute, monitor)..." aria-label="Filter commands" />'
    )
    for label, entries in groups:
        out.append(render_group(label, entries))
    out.append("            </section>")

    body = "\n".join(out)
    DEST.parent.mkdir(parents=True, exist_ok=True)
    DEST.write_text(body + "\n" + FILTER_SCRIPT + "\n" + sitelib.page_foot(root))
    print("rendered %d commands (%d groups) -> %s" % (len(commands), len(groups), DEST))


def main():
    commands = load_commands()
    render(commands)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
