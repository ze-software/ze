#!/usr/bin/env python3
"""Render the plugin catalog and local plugin detail pages.

The catalog is generated from data/plugin-registry.json, which is generated
from registry.Registration values in ../main. Plugin grouping, dependencies,
configuration roots, YANG files, source paths, and detail pages come from that
registry data and the repository path layout.
"""

import html
import json
import pathlib
import re
import shutil
import urllib.parse

import sitelib
try:
    import markdown as markdown_lib
except ImportError:  # pragma: no cover - direct runs without uv fallback
    markdown_lib = None


HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
DATA = GH_PAGES / "data" / "plugin-registry.json"
DEST = GH_PAGES / "reference" / "plugins" / "index.html"
DETAIL_DIR = DEST.parent
TEST_DIR_PREFIX = "internal/test/"
CATALOG_ROOT = "../../"
DETAIL_ROOT = "../../../"
COLOR_CLASSES = sitelib.CATEGORIES

# Presentation only: labels keep common network acronyms readable while the
# actual grouping still comes from config roots and source paths.
ACRONYMS = {
    "afi": "AFI",
    "api": "API",
    "as112": "AS112",
    "bfd": "BFD",
    "bgp": "BGP",
    "copp": "CoPP",
    "ddos": "DDoS",
    "dhcp": "DHCP",
    "fib": "FIB",
    "igp": "IGP",
    "ike": "IKE",
    "ip": "IP",
    "ipfix": "IPFIX",
    "irr": "IRR",
    "isis": "IS-IS",
    "l2tp": "L2TP",
    "ldp": "LDP",
    "mrt": "MRT",
    "nat": "NAT",
    "nlri": "NLRI",
    "ntp": "NTP",
    "ospf": "OSPF",
    "p4": "P4",
    "pki": "PKI",
    "qos": "QoS",
    "rib": "RIB",
    "rpki": "RPKI",
    "rsvp": "RSVP",
    "vpn": "VPN",
    "safi": "SAFI",
    "sr": "SR",
    "tc": "TC",
    "te": "TE",
    "vpp": "VPP",
    "yang": "YANG",
}


def esc(value):
    return html.escape(str(value), quote=True)


def slug_base(name):
    slug = re.sub(r"[^a-z0-9]+", "-", name.lower()).strip("-")
    return slug or "plugin"


def assign_slugs(plugins):
    used = {}
    for plugin in sorted(plugins, key=lambda p: p["name"]):
        base = slug_base(plugin["name"])
        count = used.get(base, 0)
        used[base] = count + 1
        plugin["slug"] = base if count == 0 else "%s-%d" % (base, count + 1)


def path_parts(source_dir):
    return [part for part in source_dir.split("/") if part]


def top_config_root(plugin):
    if not plugin["config_roots"]:
        return None
    return plugin["config_roots"][0].split("/", 1)[0]

def plugin_doc(plugin):
    return plugin.get("doc") or {}


def plugin_summary(plugin):
    return plugin_doc(plugin).get("summary") or plugin["description"]


def render_markdown_body(text):
    if not text:
        return ""
    if markdown_lib is not None:
        return markdown_lib.markdown(text, extensions=["tables", "fenced_code", "sane_lists"])
    paragraphs = [p.strip() for p in text.split("\n\n") if p.strip()]
    return "".join("<p>%s</p>" % esc(p) for p in paragraphs)




def source_group(plugin):
    """Derive a group from repository structure, not a hand-built plugin map."""
    area = plugin_doc(plugin).get("area", "").strip()
    if area:
        return slug_base(area)
    source = plugin["source_dir"]
    parts = path_parts(source)
    if source.startswith(TEST_DIR_PREFIX):
        return "test-harness"

    # BGP has many registry entries with no config root. The source layout
    # carries useful generated sub-areas for filters, NLRI families, and
    # redistribution without naming individual plugins here.
    if parts[:3] == ["internal", "component", "bgp"]:
        if len(parts) >= 5 and parts[3] == "plugins":
            sub = parts[4]
            prefix = sub.split("_", 1)[0]
            if sub == "nlri" and len(parts) >= 6:
                return "bgp-nlri"
            if prefix in ("filter", "redistribute"):
                return "bgp-%s" % prefix
        if "filter" in parts:
            return "bgp-filter"
        return "bgp"

    root = top_config_root(plugin)
    if root:
        return root

    if parts[:2] == ["internal", "component"] and len(parts) >= 3:
        return parts[2]
    if parts[:2] == ["internal", "plugins"] and len(parts) >= 3:
        return parts[2]
    return slug_base(plugin["name"]).split("-", 1)[0]


def color_for(group_id):
    idx = sum(ord(ch) for ch in group_id) % len(COLOR_CLASSES)
    return COLOR_CLASSES[idx]


def title_word(token):
    if token in ACRONYMS:
        return ACRONYMS[token]
    if token in ("and", "for", "of", "to"):
        return token
    return token.capitalize()


def label_for(group_id):
    tokens = [t for t in re.split(r"[-_]+", group_id) if t]
    return " ".join(title_word(token) for token in tokens)


def short_for(group_id):
    label = label_for(group_id)
    return label if len(label) <= 18 else label[:16].rstrip() + "..."

# Repository packages cannot always use the public feature noun. For example,
# `interface` is a Go keyword, so the implementation package uses `iface`.
# Keep those code-name aliases local to grouping, while leaving source paths
# and config roots unchanged in the generated details.
GROUP_ALIASES = {
    "iface": "interface",
}


def normalize_group_id(group_id):
    return GROUP_ALIASES.get(group_id, group_id)


FILTER_CATEGORY_ORDER = [
    "routing",
    "security",
    "tunneling",
    "services",
    "dataplane",
    "telemetry",
    "system",
    "test",
]
FILTER_CATEGORY_LABELS = {
    "routing": "Routing",
    "security": "Policy & security",
    "tunneling": "Tunneling",
    "services": "Network services",
    "dataplane": "Interfaces & dataplane",
    "telemetry": "Telemetry & operations",
    "system": "System",
    "test": "Test fixtures",
}


def has_any(text, tokens):
    return any(token in text for token in tokens)


def filter_category_for(group):
    """Coarse UI buckets derived from generated group/root/source names."""
    text = " ".join(
        [
            group["id"],
            group["label"],
            " ".join(group["roots"]),
            " ".join(group["sources"]),
        ]
    ).lower()
    if group["id"] == "test-harness":
        return "test"
    if has_any(text, ("l2tp", "ppp", "pppoe", "ipsec", "ike", "vpn", "ldp", "rsvp", "mpls")):
        return "tunneling"
    if has_any(text, ("firewall", "ddos", "anomaly", "policy", "filter", "rpki", "copp", "aaa", "tacacs", "pki")):
        return "security"
    if has_any(text, ("service", "dhcp", "dns", "ntp", "as112", "image")):
        return "services"
    if has_any(text, ("iface", "interface", "fib", "vpp", "kernel", "sysctl", "bfd")):
        return "dataplane"
    if has_any(text, ("flow", "mrt", "traffic", "telemetry", "monitor", "watchdog")):
        return "telemetry"
    if has_any(text, ("bgp", "ospf", "isis", "rib", "route", "routing", "static", "connected", "redistribute")):
        return "routing"
    return "system"



def load_plugins():
    plugins = json.loads(DATA.read_text())
    assign_slugs(plugins)
    for plugin in plugins:
        group_id = normalize_group_id(source_group(plugin))
        plugin["group_id"] = group_id
    return sorted(plugins, key=lambda p: (label_for(p["group_id"]), p["name"]))


def plugin_by_name(plugins):
    return {plugin["name"]: plugin for plugin in plugins}


def build_groups(plugins):
    groups = {}
    for plugin in plugins:
        group_id = plugin["group_id"]
        group = groups.setdefault(
            group_id,
            {
                "id": group_id,
                "label": plugin_doc(plugin).get("area") or label_for(group_id),
                "short": short_for(plugin_doc(plugin).get("area") or group_id),
                "cat": color_for(group_id),
                "plugins": [],
                "roots": set(),
                "sources": set(),
            },
        )
        group["plugins"].append(plugin)
        for root in plugin["config_roots"]:
            group["roots"].add(root.split("/", 1)[0])
        source = plugin["source_dir"]
        source_parts = path_parts(source)
        if len(source_parts) >= 3:
            group["sources"].add("/".join(source_parts[:3]))
        else:
            group["sources"].add(source)
    for group in groups.values():
        group["plugins"].sort(key=lambda p: p["name"])
        group["roots"] = sorted(group["roots"])
        group["sources"] = sorted(group["sources"])
        group["filter_category"] = filter_category_for(group)
        group["filter_category_label"] = FILTER_CATEGORY_LABELS[group["filter_category"]]
        group["deck"] = group_deck(group)
    return sorted(groups.values(), key=lambda g: (g["id"] == "test-harness", g["label"]))


def group_deck(group):
    bits = [
        "Generated group for registry entries mapped to the %s area." % group["label"],
    ]
    if group["roots"]:
        bits.append("Config roots: %s." % ", ".join("`%s`" % root for root in group["roots"]))
    if group["sources"]:
        bits.append("Source area: %s." % ", ".join("`%s`" % source for source in group["sources"][:3]))
    return " ".join(bits)


def dependency_index(plugins):
    by_name = plugin_by_name(plugins)
    index = {plugin["name"]: {"required": [], "optional": []} for plugin in plugins}
    for plugin in plugins:
        for dep in plugin["dependencies"]:
            if dep in by_name:
                index[dep]["required"].append(plugin)
        for dep in plugin["optional_dependencies"]:
            if dep in by_name:
                index[dep]["optional"].append(plugin)
    for rels in index.values():
        rels["required"].sort(key=lambda p: p["name"])
        rels["optional"].sort(key=lambda p: p["name"])
    return index


def catalog_plugin_href(plugin):
    return "%s/" % plugin["slug"]


def detail_plugin_href(plugin):
    return "../%s/" % plugin["slug"]


def catalog_markdown_href(plugin):
    return "%s/index.md" % plugin["slug"]


def detail_markdown_href(plugin):
    return "../%s/index.md" % plugin["slug"]


def config_href(root, markdown=False):
    suffix = "index.md" if markdown else ""
    return "%sreference/configuration/%s#%s" % (
        DETAIL_ROOT,
        suffix,
        urllib.parse.quote(root, safe="/"),
    )


def chip(label, mode=False):
    cls = "chip mode" if mode else "chip"
    return '<span class="%s">%s</span>' % (cls, esc(label))


def chips_for(plugin):
    values = []
    for root in plugin["config_roots"]:
        values.append(chip("config:" + root, mode=True))
    for dep in plugin["dependencies"][:3]:
        values.append(chip("needs:" + dep))
    if len(plugin["dependencies"]) > 3:
        values.append(chip("+%d deps" % (len(plugin["dependencies"]) - 3)))
    if plugin["optional_dependencies"]:
        values.append(chip("optional:" + plugin["optional_dependencies"][0]))
    if plugin["yang_files"]:
        values.append(chip("YANG:%d" % len(plugin["yang_files"])))
    if not values:
        values.append(chip("no config"))
    return "\n                            ".join(values)


def searchable_text(plugin, group):
    fields = [
        plugin["name"],
        plugin["description"],
        plugin_summary(plugin),
        plugin["source_dir"],
        group["label"],
        " ".join(plugin_doc(plugin).get("tags", [])),
        plugin_doc(plugin).get("body", ""),
        " ".join(plugin["config_roots"]),
        " ".join(plugin["dependencies"]),
        " ".join(plugin["optional_dependencies"]),
        " ".join(plugin["yang_files"]),
    ]
    return " ".join(fields)


def render_meta(plugin):
    rows = []
    if plugin["config_roots"]:
        rows.append(("Config", ", ".join("<code>%s</code>" % esc(r) for r in plugin["config_roots"])))
    if plugin["dependencies"]:
        rows.append(("Needs", ", ".join("<code>%s</code>" % esc(d) for d in plugin["dependencies"])))
    if plugin["optional_dependencies"]:
        rows.append(("Optional", ", ".join("<code>%s</code>" % esc(d) for d in plugin["optional_dependencies"])))
    if plugin["yang_files"]:
        rows.append(("YANG", "%d module%s" % (len(plugin["yang_files"]), "" if len(plugin["yang_files"]) == 1 else "s")))
    if not rows:
        rows.append(("Config", "None"))
    out = ['<dl class="plugin-meta">']
    for key, value in rows:
        out.append("<div><dt>%s</dt><dd>%s</dd></div>" % (esc(key), value))
    out.append("</dl>")
    return "".join(out)


def render_card(plugin, group):
    is_test = plugin["source_dir"].startswith(TEST_DIR_PREFIX)
    test_attr = ' data-test="true"' if is_test else ""
    return """
                    <article class="card plugin-card cat-{cat}" id="plugin-{anchor}" data-plugin-card data-family="{group_id}" data-category="{filter_category}" data-search="{search}"{test_attr}>
                        <span class="cat">{short}</span>
                        <h3><a href="{href}"><code>{name}</code></a></h3>
                        <p class="plugin-desc">{desc}</p>
                        <div class="chips">
                            {chips}
                        </div>
                        {meta}
                    </article>""".format(
        cat=group["cat"],
        anchor=esc(plugin["slug"]),
        group_id=esc(group["id"]),
        filter_category=esc(group["filter_category"]),
        search=esc(searchable_text(plugin, group)),
        test_attr=test_attr,
        short=esc(group["short"]),
        href=esc(catalog_plugin_href(plugin)),
        name=esc(plugin["name"]),
        desc=esc(plugin_summary(plugin)),
        chips=chips_for(plugin),
        meta=render_meta(plugin),
    )


def render_group(group):
    cards = "\n".join(render_card(plugin, group) for plugin in group["plugins"])
    return """
                <section class="plugin-group" data-plugin-group data-family="{group_id}" data-category="{filter_category}" aria-labelledby="plugin-group-{group_id}">
                    <div class="plugin-group-head cat-{cat}">
                        <h2 id="plugin-group-{group_id}">{label}</h2>
                        <span>{count} plugin{plural}</span>
                    </div>
                    <p>{deck}</p>
                    <div class="cards plugin-grid">
{cards}
                    </div>
                </section>""".format(
        group_id=esc(group["id"]),
        filter_category=esc(group["filter_category"]),
        cat=group["cat"],
        label=esc(group["label"]),
        count=len(group["plugins"]),
        plural="" if len(group["plugins"]) == 1 else "s",
        deck=sitelib.bold(group["deck"]),
        cards=cards,
    )


def filter_buckets(groups):
    buckets = []
    for key in FILTER_CATEGORY_ORDER:
        bucket_groups = [group for group in groups if group["filter_category"] == key]
        if bucket_groups:
            buckets.append((key, FILTER_CATEGORY_LABELS[key], bucket_groups))
    return buckets


def render_filters(groups):
    buckets = filter_buckets(groups)
    category_options = ['<option value="">All categories</option>']
    area_options = ['<option value="" data-label="All areas">All areas</option>']
    for key, label, bucket_groups in buckets:
        count = sum(len(group["plugins"]) for group in bucket_groups)
        category_options.append(
            '<option value="{key}" data-label="{label}">{label} ({count})</option>'.format(
                key=esc(key),
                label=esc(label),
                count=count,
            )
        )
        area_options.append('<optgroup label="%s">' % esc(label))
        for group in bucket_groups:
            area_options.append(
                '<option value="{group_id}" data-category="{category}" data-label="{label}">{label} ({count})</option>'.format(
                    group_id=esc(group["id"]),
                    category=esc(key),
                    label=esc(group["label"]),
                    count=len(group["plugins"]),
                )
            )
        area_options.append("</optgroup>")
    return """
                        <div class="plugin-filter-controls">
                            <label for="plugin-category">Category
                                <select id="plugin-category" autocomplete="off">
                                    {category_options}
                                </select>
                            </label>
                            <label for="plugin-family">Area
                                <select id="plugin-family" autocomplete="off">
                                    {area_options}
                                </select>
                            </label>
                        </div>""".format(
        category_options="\n                                    ".join(category_options),
        area_options="\n                                    ".join(area_options),
    )


def render_script():
    return """        <script>
            (function () {
                var root = document.querySelector("[data-plugin-catalog]");
                if (!root) return;
                var input = root.querySelector("#plugin-search");
                var status = root.querySelector("#plugin-status");
                var categorySelect = root.querySelector("#plugin-category");
                var familySelect = root.querySelector("#plugin-family");
                var cards = Array.prototype.slice.call(root.querySelectorAll("[data-plugin-card]"));
                var groups = Array.prototype.slice.call(root.querySelectorAll("[data-plugin-group]"));
                var empty = root.querySelector(".plugin-empty");
                var activeCategory = "";
                var activeFamily = "";
                var totalRuntime = cards.filter(function (card) { return card.dataset.test !== "true"; }).length;
                var totalTest = cards.length - totalRuntime;

                cards.forEach(function (card) {
                    card._pluginSearch = (card.getAttribute("data-search") || "").toLowerCase();
                });

                function tokens(value) {
                    return value.toLowerCase().split(/\\s+/).filter(Boolean);
                }

                function optionForValue(select, value) {
                    for (var i = 0; i < select.options.length; i += 1) {
                        if (select.options[i].value === value) return select.options[i];
                    }
                    return null;
                }

                function selectedLabel(select) {
                    var option = select.options[select.selectedIndex];
                    return option ? (option.dataset.label || option.textContent).replace(/\\s+\\(\\d+\\)$/, "") : "";
                }

                function syncControls() {
                    categorySelect.value = activeCategory;
                    familySelect.value = activeFamily;
                }

                function updateUrl(query) {
                    var url = new URL(location.href);
                    if (query) url.searchParams.set("q", query);
                    else url.searchParams.delete("q");
                    if (activeCategory) url.searchParams.set("category", activeCategory);
                    else url.searchParams.delete("category");
                    if (activeFamily) url.searchParams.set("family", activeFamily);
                    else url.searchParams.delete("family");
                    history.replaceState(null, "", url);
                }

                function suffix() {
                    if (activeFamily) return " in " + selectedLabel(familySelect) + " area";
                    if (activeCategory) return " in " + selectedLabel(categorySelect);
                    return "";
                }

                function apply(pushUrl) {
                    var query = input.value.trim();
                    var parts = tokens(query);
                    var visible = 0;
                    cards.forEach(function (card) {
                        var categoryHit = !activeCategory || card.dataset.category === activeCategory;
                        var familyHit = !activeFamily || card.dataset.family === activeFamily;
                        var textHit = parts.every(function (part) {
                            return card._pluginSearch.indexOf(part) !== -1;
                        });
                        var show = categoryHit && familyHit && textHit;
                        card.classList.toggle("filtered-out", !show);
                        if (show) visible += 1;
                    });
                    groups.forEach(function (group) {
                        var any = !!group.querySelector("[data-plugin-card]:not(.filtered-out)");
                        group.hidden = !any;
                    });
                    empty.hidden = visible !== 0;
                    status.textContent = query || activeCategory || activeFamily
                        ? "Showing " + visible + " of " + cards.length + " plugins" + suffix() + "."
                        : "Showing " + totalRuntime + " runtime plugins" +
                            (totalTest ? " and " + totalTest + " test fixtures." : ".");
                    syncControls();
                    if (pushUrl) updateUrl(query);
                }

                var params = new URLSearchParams(location.search);
                var category = params.get("category") || "";
                if (category && optionForValue(categorySelect, category)) {
                    activeCategory = category;
                }
                var family = params.get("family") || "";
                var familyOption = family ? optionForValue(familySelect, family) : null;
                if (familyOption) {
                    activeFamily = family;
                    activeCategory = familyOption.dataset.category || activeCategory;
                }
                input.value = params.get("q") || "";

                categorySelect.addEventListener("change", function () {
                    activeCategory = categorySelect.value;
                    activeFamily = "";
                    apply(true);
                });
                familySelect.addEventListener("change", function () {
                    var option = familySelect.options[familySelect.selectedIndex];
                    activeFamily = familySelect.value;
                    activeCategory = activeFamily && option ? option.dataset.category : activeCategory;
                    apply(true);
                });
                input.addEventListener("input", function () { apply(true); });
                apply(false);
            })();
        </script>
"""


def html_link_list(items):
    if not items:
        return '<p class="plugin-detail-empty">None declared.</p>'
    return "<ul>" + "".join("<li>%s</li>" % item for item in items) + "</ul>"


def dependency_links(names, by_name):
    out = []
    for name in names:
        plugin = by_name.get(name)
        if plugin:
            out.append('<a href="%s"><code>%s</code></a>' % (esc(detail_plugin_href(plugin)), esc(name)))
        else:
            out.append('<code>%s</code>' % esc(name))
    return out


def plugin_links(plugins):
    return [
        '<a href="%s"><code>%s</code></a>' % (esc(detail_plugin_href(plugin)), esc(plugin["name"]))
        for plugin in plugins
    ]


def group_for_plugin(groups_by_id, plugin):
    return groups_by_id[plugin["group_id"]]


def render_detail_html(plugin, group, by_name, dependents):
    summary = plugin_summary(plugin)
    title = "%s plugin - Ze" % plugin["name"]
    desc = "%s plugin: %s" % (plugin["name"], summary)
    runtime_label = "Test fixture" if plugin["source_dir"].startswith(TEST_DIR_PREFIX) else "Runtime plugin"
    root = DETAIL_ROOT

    config_items = [
        '<a href="%s"><code>%s</code></a>' % (esc(config_href(root_name)), esc(root_name))
        for root_name in plugin["config_roots"]
    ]
    yang_items = ['<code>%s</code>' % esc(path) for path in plugin["yang_files"]]
    required = dependency_links(plugin["dependencies"], by_name)
    optional = dependency_links(plugin["optional_dependencies"], by_name)
    required_by = plugin_links(dependents[plugin["name"]]["required"])
    optional_by = plugin_links(dependents[plugin["name"]]["optional"])
    doc = plugin_doc(plugin)
    doc_body = render_markdown_body(doc.get("body", ""))

    out = [
        sitelib.page_head(
            title,
            desc,
            root,
            og_title=title,
            og_desc=desc,
            page_key="reference/plugins/%s/" % plugin["slug"],
        )
    ]
    out.append('            <section class="md-content reveal cat-%s plugin-detail" aria-labelledby="plugin-detail-title">' % group["cat"])
    out.append(
        sitelib.page_hero(
            "<code>%s</code>" % esc(plugin["name"]),
            esc(summary),
            group["label"],
            h1_id="plugin-detail-title",
            title_html=True,
            lead_html=True,
            classes="journey-hero reveal cat-%s" % group["cat"],
        )
    )
    out.append('                <div class="plugin-detail-grid">')

    out.append('                    <article class="plugin-detail-panel">')
    out.append("                        <h2>At a glance</h2>")
    out.append('                        <dl class="plugin-detail-facts">')
    facts = [
        ("Registry area", group["label"]),
        ("Kind", runtime_label),
        ("Source path", "<code>%s</code>" % esc(plugin["source_dir"])),
        ("YANG modules", str(len(plugin["yang_files"]))),
        ("Metadata source", "<code>%s</code>" % esc(doc["source"]) if doc.get("source") else "Registration"),
    ]
    for key, value in facts:
        out.append("                            <div><dt>%s</dt><dd>%s</dd></div>" % (esc(key), value))
    out.append("                        </dl>")
    out.append("                    </article>")

    out.append('                    <article class="plugin-detail-panel">')
    out.append("                        <h2>Configuration</h2>")
    out.append("                        %s" % html_link_list(config_items))
    out.append("                    </article>")

    out.append('                    <article class="plugin-detail-panel">')
    out.append("                        <h2>Dependencies</h2>")
    out.append("                        <h3>Required</h3>")
    out.append("                        %s" % html_link_list(required))
    out.append("                        <h3>Optional</h3>")
    out.append("                        %s" % html_link_list(optional))
    out.append("                    </article>")

    out.append('                    <article class="plugin-detail-panel">')
    out.append("                        <h2>Used by</h2>")
    out.append("                        <h3>Required dependency for</h3>")
    out.append("                        %s" % html_link_list(required_by))
    out.append("                        <h3>Optional dependency for</h3>")
    out.append("                        %s" % html_link_list(optional_by))
    out.append("                    </article>")

    if doc_body:
        out.append('                    <article class="plugin-detail-panel plugin-detail-panel-wide">')
        out.append("                        <h2>Plugin notes</h2>")
        out.append('                        <div class="plugin-detail-notes">%s</div>' % doc_body)
        out.append("                    </article>")

    out.append('                    <article class="plugin-detail-panel plugin-detail-panel-wide">')
    out.append("                        <h2>Repository artifacts</h2>")
    out.append("                        <p>These paths come from the registry extraction and are shown locally so the detail page stays on the site.</p>")
    out.append('                        <dl class="plugin-detail-facts">')
    out.append("                            <div><dt>Package</dt><dd><code>%s</code></dd></div>" % esc(plugin["source_dir"]))
    out.append("                        </dl>")
    out.append("                        <h3>YANG files</h3>")
    out.append("                        %s" % html_link_list(yang_items))
    out.append("                    </article>")

    out.append("                </div>")
    out.append("            </section>")
    return "\n".join(out) + "\n" + sitelib.page_foot(root)


def md_code_list(values):
    return ", ".join("`%s`" % value for value in values) if values else "None"


def md_dependency_list(names, by_name):
    if not names:
        return "None"
    parts = []
    for name in names:
        plugin = by_name.get(name)
        if plugin:
            parts.append("[`%s`](%s)" % (name, detail_markdown_href(plugin)))
        else:
            parts.append("`%s`" % name)
    return ", ".join(parts)


def md_plugin_list(plugins):
    if not plugins:
        return "None"
    return ", ".join("[`%s`](%s)" % (p["name"], detail_markdown_href(p)) for p in plugins)


def render_detail_markdown(plugin, group, by_name, dependents):
    runtime_label = "Test fixture" if plugin["source_dir"].startswith(TEST_DIR_PREFIX) else "Runtime plugin"
    lines = [
        "# `%s` plugin" % plugin["name"],
        "",
        plugin_summary(plugin),
        "",
        "## At a glance",
        "",
        "| Field | Value |",
        "|-------|-------|",
        "| Registry area | %s |" % group["label"],
        "| Kind | %s |" % runtime_label,
        "| Source path | `%s` |" % plugin["source_dir"],
        "| YANG modules | %d |" % len(plugin["yang_files"]),
        "",
        "## Configuration",
        "",
        md_code_list(plugin["config_roots"]),
        "",
        "## Dependencies",
        "",
        "- Required: %s" % md_dependency_list(plugin["dependencies"], by_name),
        "- Optional: %s" % md_dependency_list(plugin["optional_dependencies"], by_name),
        "",
        "## Used by",
        "",
        "- Required dependency for: %s" % md_plugin_list(dependents[plugin["name"]]["required"]),
        "- Optional dependency for: %s" % md_plugin_list(dependents[plugin["name"]]["optional"]),
        "",
        "## Repository artifacts",
        "",
        "Package: `%s`" % plugin["source_dir"],
        "",
        "YANG files: %s" % md_code_list(plugin["yang_files"]),
        "Metadata source: `%s`" % (plugin_doc(plugin).get("source") or "Registration"),
        "",
        "",
    ]
    doc = plugin_doc(plugin)
    if doc.get("body"):
        lines.extend(["## Plugin notes", "", doc["body"], ""])
    return "\n".join(lines).strip() + "\n"


def render_markdown(plugins, groups):
    runtime = [p for p in plugins if not p["source_dir"].startswith(TEST_DIR_PREFIX)]
    with_config = [p for p in runtime if p["config_roots"]]
    with_yang = [p for p in runtime if p["yang_files"]]
    lines = [
        "# Plugin catalog",
        "",
        "%d runtime plugins generated from `data/plugin-registry.json`, plus %d test fixtures. %d runtime plugins declare configuration roots and %d ship YANG modules." % (
            len(runtime),
            len(plugins) - len(runtime),
            len(with_config),
            len(with_yang),
        ),
        "",
        "The HTML page includes browser-side search across name, purpose, config roots, dependencies, YANG files, and source directories. Clicking a plugin opens its generated local detail page.",
        "",
    ]
    for group in groups:
        items = group["plugins"]
        if not items:
            continue
        lines.extend(["## %s" % group["label"], "", group["deck"], ""])
        lines.append("| Plugin | Used for | Config | Depends on | Source path |")
        lines.append("|--------|----------|--------|------------|-------------|")
        for plugin in items:
            roots = ", ".join("`%s`" % root for root in plugin["config_roots"]) or "None"
            deps = ", ".join("`%s`" % dep for dep in plugin["dependencies"]) or "None"
            lines.append(
                "| [`%s`](%s) | %s | %s | %s | `%s` |"
                % (
                    plugin["name"],
                    catalog_markdown_href(plugin),
                    plugin_summary(plugin).replace("|", "\\|"),
                    roots,
                    deps,
                    plugin["source_dir"],
                )
            )
        lines.append("")
    return "\n".join(lines).strip() + "\n"


def render_catalog(plugins, groups):
    runtime = [p for p in plugins if not p["source_dir"].startswith(TEST_DIR_PREFIX)]
    configured = [p for p in runtime if p["config_roots"]]
    yang = [p for p in runtime if p["yang_files"]]
    dependent = [p for p in runtime if p["dependencies"] or p["optional_dependencies"]]

    title = "Plugin Catalog - Ze"
    desc = "Search every Ze runtime plugin by purpose, config root, dependency, and source."
    out = [
        sitelib.page_head(
            title,
            desc,
            CATALOG_ROOT,
            og_title=title,
            og_desc=desc,
            page_key="reference/plugins/",
        )
    ]
    out.append('            <section class="md-content reveal cat-automate plugin-catalog" data-plugin-catalog aria-labelledby="plugin-catalog-title">')
    out.append(
        sitelib.page_hero(
            "Plugin catalog",
            "Ze features are composed from plugins. This catalog is generated from the live registry and explains what each plugin is for, what it configures, and which other plugins it relies on. Click any plugin to open a local detail page.",
            "Plugins",
            h1_id="plugin-catalog-title",
        )
    )
    out.append(
        '                <p class="plugin-summary">Generated from %d registry entries: %d runtime plugins and %d test fixtures. Among runtime plugins, %d declare configuration roots, %d declare dependencies, and %d ship YANG modules.</p>'
        % (
            len(plugins),
            len(runtime),
            len(plugins) - len(runtime),
            len(configured),
            len(dependent),
            len(yang),
        )
    )
    out.append('                <div class="plugin-console" role="search" aria-label="Search plugins">')
    out.append('                    <label for="plugin-search">Search by feature, protocol, config root, dependency, or source path</label>')
    out.append('                    <input id="plugin-search" type="search" autocomplete="off" placeholder="Try RPKI, FlowSpec, FIB, DHCP, DDoS, l2tp, bgp-filter..." />')
    out.append(render_filters(groups))
    out.append('                    <p id="plugin-status" class="plugin-status search-status" aria-live="polite"></p>')
    out.append("                </div>")
    out.append('                <p class="plugin-empty" hidden>No plugins match this search.</p>')
    out.append('                <div class="plugin-groups">')
    for group in groups:
        out.append(render_group(group))
    out.append("                </div>")
    out.append("            </section>")
    DEST.write_text("\n".join(out) + "\n" + render_script() + "\n" + sitelib.page_foot(CATALOG_ROOT))
    sitelib.write_markdown_sibling(DEST, render_markdown(plugins, groups))


def clean_detail_dirs():
    DETAIL_DIR.mkdir(parents=True, exist_ok=True)
    for child in DETAIL_DIR.iterdir():
        if child.is_dir():
            shutil.rmtree(child)


def render_details(plugins, groups):
    by_name = plugin_by_name(plugins)
    dependents = dependency_index(plugins)
    groups_by_id = {group["id"]: group for group in groups}
    for plugin in plugins:
        group = group_for_plugin(groups_by_id, plugin)
        dest = DETAIL_DIR / plugin["slug"] / "index.html"
        dest.parent.mkdir(parents=True, exist_ok=True)
        dest.write_text(render_detail_html(plugin, group, by_name, dependents))
        sitelib.write_markdown_sibling(dest, render_detail_markdown(plugin, group, by_name, dependents))


def render():
    plugins = load_plugins()
    groups = build_groups(plugins)
    runtime = [p for p in plugins if not p["source_dir"].startswith(TEST_DIR_PREFIX)]
    clean_detail_dirs()
    render_details(plugins, groups)
    render_catalog(plugins, groups)
    print(
        "rendered %s -> %s (%d runtime plugins, %d test fixtures, %d generated areas, %d detail pages, + index.md)"
        % (DATA, DEST, len(runtime), len(plugins) - len(runtime), len(groups), len(plugins))
    )


def main():
    if not DATA.exists():
        print("error: %s not found, run tools/extract-plugin-registry.py first" % DATA)
        return 1
    render()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
