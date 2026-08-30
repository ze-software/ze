// Design: website/AI.md -- the configuration browser's own script
// Detail: config.go renders the markup and the two JSON payloads it reads.
package site

// configBrowserScript walks the configuration tree in the browser: it renders
// one level at a time from the embedded JSON, keeps the current path in the URL
// fragment so a level can be linked, walks back up through a breadcrumb, and
// searches every node when the box is not empty.
//
// It is a raw string because it holds quotation marks of both kinds and no
// backtick, and because a reader comparing it against the page it ships to
// should meet the same characters in both.
const configBrowserScript = `        <script>
            (function () {
                var root = document.querySelector("[data-config-explorer]");
                if (!root) return;
                var treeEl = document.getElementById("config-tree");
                var ownersEl = document.getElementById("config-owners");
                if (!treeEl || !ownersEl) return;
                var tree = JSON.parse(treeEl.textContent);
                var owners = JSON.parse(ownersEl.textContent);
                var rootChildren = Object.keys(tree)
                    .sort()
                    .map(function (k) { return tree[k]; });
                var search = root.querySelector("#config-search");
                var crumbs = root.querySelector(".config-crumbs");
                var level = root.querySelector(".config-level");

                var ENT = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" };
                function esc(s) {
                    return String(s).replace(/[&<>"]/g, function (c) { return ENT[c]; });
                }
                function ws(s) { return String(s).replace(/\s+/g, " ").trim(); }
                function nodeHead(n) {
                    var k = n.kind || "";
                    if (k.slice(0, 5) === "list[" && k.slice(-1) === "]")
                        return n.name + " <" + k.slice(5, -1) + ">";
                    return n.name;
                }
                function nodeBadge(n) {
                    var k = n.kind || "", t = n.type || "";
                    if ((k === "leaf" || k === "leaf-list") && t)
                        return k === "leaf-list" ? t + "[]" : t;
                    if (k.slice(0, 5) === "list[") return "list";
                    return k;
                }
                function nameCell(n, pathStr, drill) {
                    var badge = nodeBadge(n);
                    var inner = "<code>" + esc(nodeHead(n)) + "</code>" +
                        (badge ? ' <span class="yang-type">' + esc(badge) + "</span>" : "");
                    return drill ? '<a href="#' + esc(pathStr) + '">' + inner + "</a>" : inner;
                }
                // A node's own ConfigRoot owner, else the nearest ancestor
                // ConfigRoot's owner but only when that root has a *single*
                // owner -- inheriting a multi-owner root (bgp, environment,
                // ...) down would wrongly attribute its core/shared children
                // (e.g. environment/api-server) to plugins that only augment
                // part of it. So single-owner subtrees fill; shared ones stay
                // blank, and the header separately marks them shared vs core.
                function effectiveOwner(pathStr) {
                    if (owners[pathStr]) return owners[pathStr];
                    var parts = pathStr ? pathStr.split("/") : [];
                    parts.pop();
                    while (parts.length) {
                        var o = owners[parts.join("/")];
                        if (o) return o.plugins.length === 1 ? o : null;
                        parts.pop();
                    }
                    return null;
                }
                function ownerContext(pathStr) {
                    if (owners[pathStr]) return { owner: owners[pathStr] };
                    var parts = pathStr ? pathStr.split("/") : [];
                    parts.pop();
                    while (parts.length) {
                        var o = owners[parts.join("/")];
                        if (o) return o.plugins.length === 1 ? { owner: o } : { shared: o };
                        parts.pop();
                    }
                    return { core: true };
                }
                function ownerTag(pathStr) {
                    var o = effectiveOwner(pathStr);
                    return o
                        ? '<span class="config-owner-tag is-plugin">' + esc(o.label) + "</span>"
                        : "";
                }
                function ownerLine(o) {
                    function one(p) {
                        var links = p.yang.map(function (y) {
                            return '<a href="' + esc(y.href) +
                                '" target="_blank" rel="noopener"><code>' +
                                esc(y.file) + "</code></a>";
                        }).join(", ");
                        return "<code>" + esc(p.name) + "</code>" +
                            (links ? " &mdash; " + links : "");
                    }
                    if (o.plugins.length === 1)
                        return '<p class="config-owner-detail">Provided by ' +
                            one(o.plugins[0]) + "</p>";
                    return '<details class="config-owners"><summary>Provided by ' +
                        o.plugins.length + " plugins</summary><ul>" +
                        o.plugins.map(function (p) { return "<li>" + one(p) + "</li>"; }).join("") +
                        "</ul></details>";
                }
                function ownerDetail(pathStr) {
                    var ctx = ownerContext(pathStr);
                    if (ctx.owner) return ownerLine(ctx.owner);
                    if (ctx.core)
                        return '<p class="config-owner-detail">Core configuration ' +
                            "&mdash; no plugin owner.</p>";
                    return '<p class="config-owner-detail">Part of a plugin-shared ' +
                        "container (" + esc(ctx.shared.label) + ").</p>";
                }

                function nodeAt(P) {
                    var kids = rootChildren, node = null;
                    for (var i = 0; i < P.length; i++) {
                        node = null;
                        for (var j = 0; j < kids.length; j++)
                            if (kids[j].name === P[i]) { node = kids[j]; break; }
                        if (!node) return null;
                        kids = node.children || [];
                    }
                    return { node: node, children: kids };
                }

                function tableFor(P, kids) {
                    if (!kids.length)
                        return '<p class="config-empty">No settings under this node.</p>';
                    var rows = "";
                    for (var i = 0; i < kids.length; i++) {
                        var c = kids[i];
                        var ps = P.concat([c.name]).join("/");
                        var drill = (c.children || []).length > 0;
                        var desc = c.description
                            ? esc(ws(c.description))
                            : '<span class="config-index-nodesc">' +
                              (drill ? "" : "&mdash;") + "</span>";
                        rows += "<tr" +
                            (drill ? ' class="is-drillable" data-path="' + esc(ps) + '"' : "") +
                            '><th scope="row">' + nameCell(c, ps, drill) + "</th>" +
                            "<td>" + ownerTag(ps) + "</td>" +
                            "<td>" + desc + "</td></tr>";
                    }
                    return '<div class="config-index-wrap"><table class="config-index"><thead><tr>' +
                        '<th scope="col">Setting</th><th scope="col">Provided by</th>' +
                        '<th scope="col">Description</th></tr></thead><tbody>' +
                        rows + "</tbody></table></div>";
                }

                function renderCrumbs(P) {
                    var parts = ['<a href="#">Configuration</a>'], acc = [];
                    for (var i = 0; i < P.length; i++) {
                        acc = acc.concat([P[i]]);
                        if (i < P.length - 1)
                            parts.push('<a href="#' + esc(acc.join("/")) + '"><code>' +
                                esc(P[i]) + "</code></a>");
                        else
                            parts.push('<span class="config-crumb-current"><code>' +
                                esc(P[i]) + "</code></span>");
                    }
                    crumbs.innerHTML = parts.join('<span class="config-crumb-sep">/</span>');
                    crumbs.style.display = "";
                }

                function renderLevel(P) {
                    var at = nodeAt(P);
                    if (!at) { location.hash = ""; return; }
                    renderCrumbs(P);
                    var html = "";
                    if (P.length > 0) {
                        var n = at.node, badge = nodeBadge(n);
                        html += '<div class="config-detail-head"><h2><code>' +
                            esc(nodeHead(n)) + "</code>" +
                            (badge ? ' <span class="yang-type">' + esc(badge) + "</span>" : "") +
                            "</h2>" + ownerDetail(P.join("/")) +
                            (n.description
                                ? '<p class="config-detail-desc">' + esc(ws(n.description)) + "</p>"
                                : "") + "</div>";
                    } else {
                        html += '<p class="config-index-hint">' + rootChildren.length +
                            " configuration sections. Pick one to inspect its structure, " +
                            "or search across the whole configuration.</p>";
                    }
                    html += tableFor(P, at.children);
                    level.innerHTML = html;
                }

                function renderSearch(q) {
                    crumbs.style.display = "none";
                    var ql = q.toLowerCase(), results = [];
                    function walk(n, P) {
                        var o = owners[P.join("/")];
                        var hay = (nodeHead(n) + " " + (n.type || "") + " " +
                            (n.description || "")).toLowerCase();
                        if (o) hay += " " + o.label.toLowerCase() + " " +
                            o.plugins.map(function (p) { return p.name; }).join(" ").toLowerCase();
                        if (hay.indexOf(ql) !== -1) results.push({ n: n, P: P });
                        var kids = n.children || [];
                        for (var i = 0; i < kids.length; i++)
                            walk(kids[i], P.concat([kids[i].name]));
                    }
                    for (var i = 0; i < rootChildren.length; i++)
                        walk(rootChildren[i], [rootChildren[i].name]);
                    var rows = "";
                    for (var k = 0; k < results.length; k++) {
                        var r = results[k], drill = (r.n.children || []).length > 0;
                        var target = drill ? r.P.join("/") : r.P.slice(0, -1).join("/");
                        var label = r.P.map(function (x) { return esc(x); })
                            .join(' <span class="config-crumb-sep">/</span> ');
                        rows += '<tr class="is-drillable" data-path="' + esc(target) + '">' +
                            '<th scope="row"><a href="#' + esc(target) + '">' + label + "</a></th>" +
                            "<td>" + ownerTag(r.P.join("/")) + "</td>" +
                            "<td>" + (r.n.description ? esc(ws(r.n.description)) : "") +
                            "</td></tr>";
                    }
                    var head = '<p class="config-index-count">' + results.length +
                        ' match "' + esc(q) + '"</p>';
                    level.innerHTML = head + (results.length
                        ? '<div class="config-index-wrap"><table class="config-index"><thead><tr><th scope="col">Path</th>' +
                          '<th scope="col">Provided by</th><th scope="col">Description</th>' +
                          "</tr></thead><tbody>" + rows + "</tbody></table></div>"
                        : "");
                }

                function pathFromHash() {
                    var h = (location.hash || "").replace(/^#/, "");
                    return h === "" ? [] : h.split("/");
                }
                function refresh() {
                    var q = search.value.trim();
                    if (q !== "") renderSearch(q);
                    else renderLevel(pathFromHash());
                }

                window.addEventListener("hashchange", function () {
                    if (search.value.trim() !== "") search.value = "";
                    renderLevel(pathFromHash());
                    var top = root.getBoundingClientRect().top;
                    if (top < 60 || top > 160)
                        window.scrollTo({ top: window.scrollY + top - 76, behavior: "smooth" });
                });
                root.addEventListener("click", function (e) {
                    var tr = e.target.closest && e.target.closest("tr.is-drillable");
                    if (tr && !(e.target.closest && e.target.closest("a")))
                        location.hash = tr.getAttribute("data-path");
                });
                search.addEventListener("input", refresh);

                refresh();
            })();
        </script>
`
