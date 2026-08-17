#!/usr/bin/env -S uv run python3
"""Render llms.txt at the site root.

Usage:
    tools/render-llms-txt.py

llms.txt is the single fast path for AI crawlers. It is intentionally
more than a sitemap: it denormalizes the product facts, generated
inventories, command surface, plugin registry, dependency rationale,
quality model, curated navigation, and the complete published page map into one plain-text file.

The source of truth remains the structured site inputs:

* data/nav.json for curated navigation and generated page order.
* tools/page_registry.py and page Markdown for the complete documentation map.
* data/site-facts.json for live counts emitted by the build.
* data/features.json for the product inventory.
* data/cli-commands.json for the live command catalog generated from ze.
* data/command-equivalents.json for vendor command mappings.
* data/plugin-registry.json for runtime plugin metadata.
* data/yang-config-tree.json for config roots extracted from YANG.
* data/dependencies.json for direct Go dependency rationale.

Every page link still points at that page's index.md sibling first, but
most common product questions should be answerable from this file alone.
"""

from __future__ import annotations

import html
import json
import pathlib
import re
from collections import Counter, defaultdict

import sitelib
import page_registry
import sitepaths

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
DATA_DIR = GH_PAGES / "data"
DEST = GH_PAGES / "llms.txt"

SITE_BASE = sitelib.SITE_BASE


def read_json(name: str):
    return json.loads((DATA_DIR / name).read_text())


def live_counts():
    return sitelib.live_counts()


def live_article_count():
    return live_counts()["articles"]


LIVE_DESC_OVERRIDES = {
    "features/": lambda: (
        "%s features, color-coded by category" % live_counts()["features"]
    ),
    "reference/cli/": lambda: (
        "%s commands, generated from the live binary" % live_counts()["cli_commands"]
    ),
    "reference/dependencies/": lambda: (
        "%s direct packages, generated from go.mod" % live_counts()["dependencies"]
    ),
    "changes/": lambda: "%s weekly updates, newest first" % live_counts()["changes"],
}


SPACE_RE = re.compile(r"\s+")
TAG_RE = re.compile(r"<[^>]+>")
LINK_RE = re.compile(r"\[([^\]]+)\]\([^)]+\)")
MARK_RE = re.compile(r"\*\*|__")


def clean(value) -> str:
    """Plain, single-line text safe for llms.txt inventories."""
    if value is None:
        return ""
    text = html.unescape(str(value))
    text = TAG_RE.sub(" ", text)
    text = LINK_RE.sub(r"\1", text)
    text = MARK_RE.sub("", text)
    text = text.replace("—", "-").replace("–", "-").replace("…", "...")
    text = SPACE_RE.sub(" ", text).strip()
    return text


def trim(value, limit: int = 220) -> str:
    text = clean(value)
    if len(text) <= limit:
        return text
    cut = text[:limit].rsplit(" ", 1)[0].rstrip(".,;:")
    return cut + "..."


def join_items(items) -> str:
    values = [clean(item) for item in items if clean(item)]
    return ", ".join(values) if values else "none"


def local_md_url(href: str) -> str:
    if href.startswith("http://") or href.startswith("https://"):
        return href
    href = href.lstrip("/")
    path, sep, fragment = href.partition("#")
    suffix = ("#" + fragment) if sep else ""
    if path in ("", "index.html"):
        path = "index.md"
    elif path.endswith("/"):
        path = path + "index.md"
    elif not path.endswith(".md") and not path.endswith(".html"):
        path = path + "/index.md"
    return SITE_BASE + path + suffix


def local_web_url(href: str) -> str:
    if href.startswith("http://") or href.startswith("https://"):
        return href
    return SITE_BASE + href.lstrip("/")


def nav_entry_line(entry):
    href = entry["href"]
    desc = entry["desc"]
    override = LIVE_DESC_OVERRIDES.get(href)
    if override:
        fresh = override()
        if fresh:
            desc = fresh
    return "- [%s](%s): %s (web: %s)" % (
        entry["title"],
        local_md_url(href),
        clean(desc),
        local_web_url(href),
    )


def render_dropdown_section(dropdown):
    lines = ["## %s" % dropdown["label"], ""]
    if dropdown.get("dynamic") == "blog":
        for column in sitelib.blog_dropdown_columns():
            for href, _icon, title, desc, _feature in column:
                lines.append(
                    nav_entry_line({"href": href, "title": title, "desc": desc})
                )
        lines.append("")
        return "\n".join(lines)
    for column in dropdown["columns"]:
        for entry in column:
            if "label_only" in entry:
                lines.append("### %s" % entry["label_only"])
                continue
            lines.append(nav_entry_line(entry))
    lines.append("")
    return "\n".join(lines)


def render_intro():
    return "\n".join(
        [
            "# Ze",
            "",
            "> Ze is an open-source configuration and protocol engine. The network operating system built on it speaks BGP, manages Linux interfaces, programs the FIB, and serves the same YANG-modeled configuration through CLI, SSH, web, API, and MCP. Its core holds the supervisor, message bus, config provider, and plugin manager; protocols and services arrive as subsystems or plugins.",
            "",
            "Pre-release: no tagged versions yet, built continuously from the main branch. AGPLv3 open source. See the ExaBGP [migration path](use-cases/exabgp-migration/index.md).",
            "",
            "This file is intentionally denormalized for AI use. It includes the high-signal product inventory and then the normal page map, so common questions should not require fetching many separate pages. Page links still point at Markdown `index.md` files first, with rendered web URLs beside them for humans.",
            "",
        ]
    )


def render_product_snapshot():
    facts = read_json("site-facts.json")
    features = facts["features"]
    tests = facts["tests"]
    interop = facts["interop"]
    plugin_count = len(read_json("plugin-registry.json"))
    lines = ["## Product snapshot", ""]
    lines.extend(
        [
            "- Purpose: configuration and protocol engine for Linux routing, plus a network operating system built on that core.",
            "- Protocols and subsystems in the shipped daemon: BGP, IS-IS, OSPF, BFD, static routes, policy routing, FIB programming, interfaces, firewall, traffic control, DNS, DHCP, NTP, IPsec, L2TP, PPPoE, telemetry, web UI, SSH CLI, MCP, and plugins.",
            "- Operator surfaces: SSH CLI with commit and rollback, generated command reference, server-rendered web workbench, looking glass, telemetry, gNMI, gRPC, MCP, JSON/YAML/NDJSON/table output, and shell-like output pipes derived from the schema where possible.",
            "- Dataplane: Linux netlink, nftables, eBPF, AF_PACKET, psample, optional VPP integrations, and namespace-aware testing.",
            "- Release state: pre-release, main-branch builds, no tagged stable release yet.",
            "- License and repos: AGPLv3. Canonical repository: %s. Discord: %s."
            % (sitelib.REPO_URL, sitelib.DISCORD_INVITE),
            "- Current generated counts: %s shipped or experimental feature cards, %s roadmap cards, %s CLI commands, %s config sections, %s plugin registrations, %s direct Go dependencies, %s weekly change entries."
            % (
                features["core_experimental"],
                features["planned"],
                facts["cli_commands"],
                facts["config_sections"],
                plugin_count,
                facts["dependencies"],
                facts["changes"],
            ),
            "- Test evidence counts: %s unit tests, %s fuzz targets, %s end-to-end transcript steps, %s interop scenarios across %s target implementations."
            % (
                tests["unit_display"],
                tests["fuzz_display"],
                tests["e2e_display"],
                interop["scenarios"],
                interop["target_display"],
            ),
            "- Generated date: %s." % facts["generated_at"],
            "",
        ]
    )
    return "\n".join(lines)


def render_quality_snapshot():
    facts = read_json("site-facts.json")
    tests = facts["tests"]
    interop = facts["interop"]
    lines = ["## Quality and verification model", ""]
    lines.extend(
        [
            "Ze uses layered proof because bugs appear at different boundaries.",
            "",
            "- Local Go tests: package behavior, parser rules, encoders, state transitions, validation paths, and error shapes. Current scale: %s unit tests."
            % tests["unit_display"],
            "- Race, coverage, and fuzz: fuzz targets are normal Go tests with generated input. Current scale: %s fuzz targets."
            % tests["fuzz_display"],
            "- gomu mutation checks: mutate production Go code and rerun tests to find weak assertions. gomu is advisory, not the default CI gate.",
            "- Functional `.ci` transcripts: drive processes, CLI commands, files, HTTP, syslog, peers, daemons, exits, and BGP wire expectations. BGP failures are decoded structurally, not shown as raw hex only.",
            "- Browser `.wb` transcripts: drive the rendered web UI through real browser flows.",
            "- Editor `.et` transcripts: drive the headless interactive editor.",
            "- QEMU: runs Linux-only behavior from macOS or CI where netlink, nftables, eBPF, PPP, network namespaces, and kernel modules exist.",
            "- Interop: %s scenarios against %s target implementations, including FRR, BIRD, GoBGP, RustyBGP, OpenBGPD, ExaBGP, and other real daemons where applicable."
            % (interop["scenarios"], interop["target_display"]),
            "- Verify workflow: `make ze-precommit-verify` takes a shared lock, writes stage logs under `tmp/`, groups related failures, and prints narrow rerun commands. `make ze-precommit-verify-changed` and `make ze-repository-check` are narrower handoff gates.",
            "- Rule for regressions: do not hide a failure with a skip or loose assertion. Move the proof to the layer that can see the real behavior, add the narrow test, rerun it, then rerun the gate that should have caught it.",
            "",
            "Useful commands: `go test -race -run TestName ./internal/...`, `make ze-fuzz-test-one FUZZ=FuzzName PKG=./path TIME=30s`, `make ze-mutation-test-changed`, `bin/ze-test bgp plugin 42 -v`, `make ze-qemu-needs-linux-test`, `make ze-interop-test`, `make ze-evidence-release-verify`.",
            "",
        ]
    )
    return "\n".join(lines)


def render_comparison_snapshot():
    lines = ["## Comparison positioning", ""]
    lines.extend(
        [
            "- BGP comparison lens: Ze is compared with BIRD, FRR, OpenBGPD, GoBGP, bio-rd, ExaBGP, RustyBGP, rustbgpd, and freeRtr across AFI/SAFI, core protocol, policy, security, observability, APIs, operations, and best-path behavior.",
            "- Network OS lens: Ze is compared with VyOS and freeRtr across routing, interfaces, firewall, NAT, VPN, AAA, services, management APIs, automation, packaging, observability, tests, and implementation model.",
            "- Evidence policy: capability claims should cite upstream code, official feature documentation, or the integration layer that owns the behavior. `Unclear`, `Partial`, and `Not found` are valid outcomes when evidence does not support a stronger claim.",
            "- Comparison pages are advice for product decisions, not marketing copy.",
            "",
        ]
    )
    return "\n".join(lines)


def feature_card_line(card):
    title = clean(card["title"])
    category = clean(card.get("category") or "uncategorized")
    status = clean(card.get("status") or "current")
    chips = join_items(chip.get("text") for chip in card.get("chips", []))
    bullets = "; ".join(
        clean(bullet) for bullet in card.get("bullets", []) if clean(bullet)
    )
    href = card.get("href")
    parts = ["- %s [%s, %s]" % (title, category, status)]
    if chips != "none":
        parts.append("chips: %s" % chips)
    if bullets:
        parts.append(bullets)
    if href:
        parts.append("link: %s" % local_md_url(href))
    return ": ".join([parts[0], "; ".join(parts[1:])])


def render_feature_inventory():
    features = read_json("features.json")
    lines = ["## Feature inventory", ""]
    for section in features["sections"]:
        cards = section.get("cards", [])
        heading = clean(section.get("heading") or section.get("id") or "Features")
        lead = clean(section.get("lead") or "")
        status_counts = Counter(
            clean(card.get("status") or "current") for card in cards
        )
        status_text = ", ".join(
            "%s %s" % (count, status) for status, count in sorted(status_counts.items())
        )
        lines.append("### %s (%s cards: %s)" % (heading, len(cards), status_text))
        if lead:
            lines.append(lead)
        for card in cards:
            lines.append(feature_card_line(card))
        lines.append("")
    return "\n".join(lines)


def render_config_inventory():
    tree = read_json("yang-config-tree.json")
    lines = ["## Configuration model roots", ""]
    lines.append(
        "Top-level YANG-derived config roots. Child names are direct children only, enough to orient without fetching the full reference."
    )
    lines.append("")
    for name in sorted(tree):
        node = tree[name]
        desc = trim(node.get("description"), 180)
        children = [clean(child.get("name")) for child in node.get("children", [])]
        child_text = ", ".join(children[:14]) if children else "none"
        if len(children) > 14:
            child_text += ", ..."
        line = "- `%s`: %s" % (name, desc or clean(node.get("kind") or "config root"))
        line += " Children: %s." % child_text
        lines.append(line)
    lines.append("")
    return "\n".join(lines)


def render_plugin_inventory():
    plugins = read_json("plugin-registry.json")
    lines = ["## Plugin registry", ""]
    lines.append(
        "Each registration comes from the Go runtime registry. Config roots come from plugin metadata and YANG files."
    )
    lines.append("")
    for plugin in sorted(plugins, key=lambda item: item["name"]):
        config = join_items(plugin.get("config_roots", []))
        deps = join_items(plugin.get("dependencies", []))
        optional = join_items(plugin.get("optional_dependencies", []))
        yang_count = len(plugin.get("yang_files", []))
        line = (
            "- `%s`: %s Config roots: %s. Dependencies: %s. Optional: %s. YANG files: %s. Source: `%s`."
            % (
                clean(plugin["name"]),
                trim(plugin.get("description"), 170),
                config,
                deps,
                optional,
                yang_count,
                clean(plugin.get("source_dir")),
            )
        )
        lines.append(line)
    lines.append("")
    return "\n".join(lines)


def command_description(command):
    desc = clean(command.get("description"))
    if " Usage: " in desc:
        desc = desc.split(" Usage: ", 1)[0]
    return trim(desc, 170)


def command_meta(command):
    meta = [clean(command.get("mode") or "unknown")]
    method = clean(command.get("wire-method"))
    if method:
        meta.append("wire %s" % method)
    if command.get("global-pipes"):
        meta.append("pipes")
    args = command.get("args") or []
    if args:
        meta.append(
            "args %s"
            % ", ".join(clean(arg.get("name")) for arg in args if arg.get("name"))
        )
    return "; ".join(meta)


def render_cli_inventory():
    commands = sorted(read_json("cli-commands.json"), key=lambda item: item["path"])
    modes = Counter(command.get("mode") or "unknown" for command in commands)
    roots = defaultdict(list)
    for command in commands:
        roots[command["path"].split()[0]].append(command)
    lines = ["## CLI command surface", ""]
    lines.append(
        "The command catalog is generated from `ze help command --json`, not hand-written. Modes: %s."
        % ", ".join("%s %s" % (count, mode) for mode, count in sorted(modes.items()))
    )
    lines.append(
        "`daemon` commands require a running Ze daemon. `read-only` commands query state. `offline` commands can run without daemon state. `pipes` means the command supports the shared output pipeline."
    )
    lines.append("")
    for root in sorted(roots):
        group = roots[root]
        lines.append("### `%s` commands (%s)" % (root, len(group)))
        for command in group:
            path = clean(command["path"])
            syntax = clean(command.get("syntax"))
            desc = command_description(command)
            line = "- `%s` (%s): %s" % (path, command_meta(command), desc)
            if syntax and syntax != path:
                line += " Syntax: `%s`." % syntax
            lines.append(line)
        lines.append("")
    return "\n".join(lines)


def vendor_command_text(item):
    command = clean(item.get("command"))
    meta = []
    if item.get("mode"):
        meta.append(clean(item["mode"]))
    if item.get("confidence"):
        meta.append(clean(item["confidence"]))
    return "`%s`%s" % (command, " (%s)" % ", ".join(meta) if meta else "")


def render_command_equivalents_inventory():
    data = read_json("command-equivalents.json")
    vendors = data["vendors"]
    lines = ["## Vendor command equivalents", ""]
    lines.append(clean(data.get("summary")))
    lines.append("Updated: %s." % clean(data.get("updated")))
    lines.append(
        "Vendors: %s."
        % ", ".join(
            "%s (%s)" % (clean(vendor["short-label"]), clean(vendor["rooting-model"]))
            for vendor in vendors.values()
        )
    )
    lines.append("")
    for entry in data["entries"]:
        parts = [
            "- %s: %s" % (clean(entry.get("category")), clean(entry.get("intent"))),
            "Ze: %s" % ", ".join("`%s`" % clean(cmd) for cmd in entry.get("ze", [])),
        ]
        for vendor_id, vendor in vendors.items():
            mapped = entry.get("vendors", {}).get(vendor_id, [])
            if not mapped:
                continue
            parts.append(
                "%s: %s"
                % (
                    clean(vendor["short-label"]),
                    "; ".join(vendor_command_text(item) for item in mapped),
                )
            )
        lines.append(". ".join(parts) + ".")
    lines.append("")
    return "\n".join(lines)


def render_dependency_inventory():
    data = read_json("dependencies.json")
    lines = ["## Dependency rationale", ""]
    lines.append(
        "Direct Go modules are grouped by why Ze needs them. This is generated from go.mod plus curated rationale, not copied from package names alone."
    )
    lines.append("")
    for category in data["categories"]:
        modules = category["modules"]
        lines.append("### %s (%s)" % (clean(category["name"]), len(modules)))
        for module in modules:
            lines.append(
                "- `%s`: %s" % (clean(module["module"]), trim(module.get("why"), 240))
            )
        lines.append("")
    return "\n".join(lines)


def markdown_title_and_summary(path: pathlib.Path) -> tuple[str, str]:
    """Return the H1 and first prose paragraph from a published source."""
    lines = path.read_text().splitlines()
    if lines and lines[0].strip() == "---":
        closing = next(
            (index for index, line in enumerate(lines[1:], 1) if line.strip() == "---"),
            None,
        )
        if closing is not None:
            lines = lines[closing + 1 :]

    text = "\n".join(lines)
    title_match = re.search(r"^#\s+(.+)$", text, flags=re.MULTILINE)
    title = (
        clean(title_match.group(1))
        if title_match
        else path.stem.replace("-", " ").title()
    )

    text = re.sub(r"<!--.*?-->", "", text, flags=re.DOTALL)
    list_fallback = ""
    list_item = re.compile(r"^(?:[-+*]|\d+[.)])\s+")
    for block in re.split(r"\n\s*\n", text):
        block_lines = [line.strip() for line in block.splitlines() if line.strip()]
        if not block_lines:
            continue
        first = block_lines[0]
        if first.startswith(("```", "~~~", "#", ">", "|", "<")):
            continue
        if list_item.match(first):
            if not list_fallback:
                list_fallback = trim(list_item.sub("", first), 220)
            continue
        return title, trim(" ".join(block_lines), 220)

    return title, list_fallback


def render_published_documentation():
    """List every hand-authored docs and use-case page registered for publication."""
    lines = ["## Complete documentation index", ""]
    main_docs = sitepaths.MAIN_REPO / "docs"
    for source in page_registry.DOCS_MANIFEST:
        title, desc = markdown_title_and_summary(main_docs / source)
        href = page_registry.docs_dest_rel_dir_for(source) + "/"
        lines.append(
            "- [%s](%s): %s (web: %s)"
            % (title, local_md_url(href), desc, local_web_url(href))
        )
    for page in page_registry.USE_CASE_PAGES:
        title, desc = markdown_title_and_summary(GH_PAGES / "use-cases" / page.source)
        href = page.dest.removesuffix("index.html")
        lines.append(
            "- [%s](%s): %s (web: %s)"
            % (title, local_md_url(href), desc, local_web_url(href))
        )
    lines.append("")
    return "\n".join(lines)


def render_page_map(nav):
    parts = ["## Page map", ""]
    parts.append(
        "Every link points to the page Markdown mirror first. The web URL is the human-rendered version of the same page."
    )
    parts.append("")
    for dropdown in nav["dropdowns"]:
        parts.append(render_dropdown_section(dropdown))

    more_lines = ["## More", ""]
    for link in nav["trailing_links"]:
        href = link["href"]
        md_url = local_md_url(href)
        web_url = local_web_url(href)
        if href == "blog/":
            n = live_article_count()
            desc = "%d editorial articles" % n if n else "editorial articles"
            more_lines.append("- [Blog](%s): %s (web: %s)" % (md_url, desc, web_url))
        else:
            more_lines.append("- [%s](%s) (web: %s)" % (link["label"], md_url, web_url))
    more_lines.append("- [Discord](%s): community and support" % sitelib.DISCORD_INVITE)
    more_lines.append(
        "- [GitHub](%s): canonical repository, issues, wiki" % sitelib.REPO_URL
    )
    parts.append("\n".join(more_lines))
    parts.append("")
    return "\n".join(parts)


def render(nav):
    parts = [
        render_intro(),
        render_product_snapshot(),
        render_quality_snapshot(),
        render_comparison_snapshot(),
        render_feature_inventory(),
        render_config_inventory(),
        render_plugin_inventory(),
        render_cli_inventory(),
        render_command_equivalents_inventory(),
        render_dependency_inventory(),
        render_published_documentation(),
        render_page_map(nav),
    ]
    text = "\n".join(parts).rstrip() + "\n"
    DEST.write_text(text)
    print("rendered llms.txt -> %s" % DEST)


def main():
    nav = sitelib.load_nav_data()
    render(nav)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
