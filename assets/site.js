document.documentElement.classList.add("js");

document.addEventListener("DOMContentLoaded", function () {
    var reveals = Array.prototype.slice.call(document.querySelectorAll(".reveal"));
    if (reveals.length) {
        if (!("IntersectionObserver" in window)) {
            reveals.forEach(function (el) {
                el.classList.add("visible");
            });
        } else {
            var observer = new IntersectionObserver(
                function (entries) {
                    entries.forEach(function (entry) {
                        if (entry.isIntersecting) {
                            entry.target.classList.add("visible");
                            entry.target.classList.remove("reveal-pending");
                            observer.unobserve(entry.target);
                        }
                    });
                },
                { threshold: 0.01 },
            );
            reveals.forEach(function (el) {
                el.classList.add("reveal-pending");
                observer.observe(el);
            });
        }
    }

    var header = document.querySelector(".site-header");
    if (header) {
        var lastY = window.scrollY;
        var hideThreshold = header.offsetHeight;
        var suppressHide = false;
        var suppressTimer = null;

        window.addEventListener(
            "scroll",
            function () {
                var y = window.scrollY;
                if (!suppressHide && y > lastY && y > hideThreshold) {
                    header.classList.add("hidden");
                } else {
                    header.classList.remove("hidden");
                }
                lastY = y;
            },
            { passive: true },
        );
        header.addEventListener("focusin", function () {
            header.classList.remove("hidden");
        });
        document.addEventListener("mousemove", function (event) {
            if (event.clientY <= 12) {
                header.classList.remove("hidden");
            }
        });

        document.querySelectorAll('a[href*="#"]').forEach(function (link) {
            link.addEventListener("click", function () {
                suppressHide = true;
                header.classList.remove("hidden");
                clearTimeout(suppressTimer);
                suppressTimer = setTimeout(function () {
                    suppressHide = false;
                }, 1000);
            });
        });
    }

    var menuToggle = document.querySelector(".nav-menu-toggle");
    var navLinks = document.getElementById("site-nav-links");
    if (menuToggle && navLinks) {
        function setMenu(open) {
            navLinks.classList.toggle("is-open", open);
            menuToggle.setAttribute("aria-expanded", open ? "true" : "false");
        }
        menuToggle.addEventListener("click", function () {
            setMenu(!navLinks.classList.contains("is-open"));
        });
        document.addEventListener("click", function (event) {
            if (!navLinks.classList.contains("is-open")) return;
            if (event.target.closest(".nav")) return;
            setMenu(false);
        });
        document.addEventListener("keydown", function (event) {
            if (event.key === "Escape") setMenu(false);
        });
    }

    function setDropdown(dropdown, open) {
        var trigger = dropdown.querySelector(".nav-dropdown-trigger");
        dropdown.classList.toggle("is-open", open);
        if (trigger) trigger.setAttribute("aria-expanded", open ? "true" : "false");
    }

    document.querySelectorAll(".nav-dropdown").forEach(function (dropdown) {
        var trigger = dropdown.querySelector(".nav-dropdown-trigger");
        if (!trigger) return;
        trigger.addEventListener("click", function (event) {
            event.preventDefault();
            var open = !dropdown.classList.contains("is-open");
            document.querySelectorAll(".nav-dropdown.is-open").forEach(function (other) {
                if (other !== dropdown) setDropdown(other, false);
            });
            setDropdown(dropdown, open);
        });
        dropdown.addEventListener("mouseenter", function () {
            trigger.setAttribute("aria-expanded", "true");
        });
        dropdown.addEventListener("mouseleave", function () {
            if (!dropdown.classList.contains("is-open")) {
                trigger.setAttribute("aria-expanded", "false");
            }
        });
        dropdown.addEventListener("focusin", function () {
            trigger.setAttribute("aria-expanded", "true");
        });
        dropdown.addEventListener("focusout", function (event) {
            if (!dropdown.contains(event.relatedTarget) && !dropdown.classList.contains("is-open")) {
                trigger.setAttribute("aria-expanded", "false");
            }
        });
    });

    document.addEventListener("click", function (event) {
        if (event.target.closest(".nav-dropdown")) return;
        document.querySelectorAll(".nav-dropdown.is-open").forEach(function (dropdown) {
            setDropdown(dropdown, false);
        });
    });

    document.addEventListener("keydown", function (event) {
        if (event.key !== "Escape") return;
        document.querySelectorAll(".nav-dropdown.is-open").forEach(function (dropdown) {
            setDropdown(dropdown, false);
        });
        var active = document.activeElement;
        if (active && active.closest(".nav-dropdown")) active.blur();
    });

    var searchBadge = document.querySelector(".nav-badge-search");
    if (searchBadge) {
        var root = searchBadge.getAttribute("href").replace(/search\/$/, "");
        var records = null;
        var loading = false;
        var previousFocus = null;
        var inerted = [];
        var overlay = document.createElement("div");
        overlay.className = "search-overlay";
        overlay.hidden = true;
        overlay.innerHTML =
            '<div class="search-overlay-backdrop" data-close></div>' +
            '<div class="search-overlay-panel" role="dialog" aria-modal="true" ' +
            'aria-labelledby="search-overlay-title">' +
            '<div class="search-overlay-head">' +
            '<h2 id="search-overlay-title" class="search-overlay-title">Search Ze</h2>' +
            '<button type="button" class="search-overlay-close" data-close ' +
            'aria-label="Close search">×</button>' +
            "</div>" +
            '<input type="search" class="search-overlay-input" ' +
            'placeholder="Search docs, config, CLI, blog..." ' +
            'aria-label="Search the site" />' +
            '<p class="search-overlay-status" role="status" aria-live="polite"></p>' +
            '<ul class="search-overlay-results" aria-label="Search results"></ul>' +
            "</div>";
        document.body.appendChild(overlay);
        var input = overlay.querySelector(".search-overlay-input");
        var status = overlay.querySelector(".search-overlay-status");
        var list = overlay.querySelector(".search-overlay-results");
        var closeButton = overlay.querySelector(".search-overlay-close");
        var ENT = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" };

        function esc(s) {
            return String(s).replace(/[&<>"]/g, function (c) {
                return ENT[c];
            });
        }

        function snippet(text, tokens) {
            var low = text.toLowerCase();
            var at = -1;
            for (var i = 0; i < tokens.length; i++) {
                var p = low.indexOf(tokens[i]);
                if (p !== -1 && (at === -1 || p < at)) at = p;
            }
            if (at === -1) at = 0;
            var start = Math.max(0, at - 60);
            var frag = text.slice(start, start + 200);
            if (start > 0) frag = "..." + frag;
            if (start + 200 < text.length) frag = frag + "...";
            frag = esc(frag);
            tokens.forEach(function (t) {
                if (!t) return;
                var re = new RegExp(
                    "(" + t.replace(/[.*+?^${}()|[\]\\]/g, "\\$&") + ")",
                    "ig",
                );
                frag = frag.replace(re, "<mark>$1</mark>");
            });
            return frag;
        }

        function score(r, tokens) {
            var title = (r.titleLower || r.title.toLowerCase());
            var section = (r.sectionLower || (r.section || "").toLowerCase());
            var text = (r.textLower || r.text.toLowerCase());
            var total = 0;
            for (var i = 0; i < tokens.length; i++) {
                var t = tokens[i];
                var hit = 0;
                if (title.indexOf(t) !== -1) hit += 12;
                if (section.indexOf(t) !== -1) hit += 4;
                var n = text.split(t).length - 1;
                if (n > 0) hit += Math.min(n, 5);
                if (hit === 0) return 0;
                total += hit;
            }
            return total;
        }

        function run(q) {
            list.innerHTML = "";
            if (!records) {
                status.textContent = loading ? "Loading index..." : "";
                return;
            }
            var tokens = q.toLowerCase().split(/\s+/).filter(Boolean);
            if (!tokens.length) {
                status.textContent = "";
                return;
            }
            var scored = [];
            for (var i = 0; i < records.length; i++) {
                var s = score(records[i], tokens);
                if (s > 0) scored.push([s, records[i]]);
            }
            scored.sort(function (a, b) { return b[0] - a[0]; });
            status.textContent = scored.length
                ? scored.length + " result" + (scored.length === 1 ? "" : "s")
                : 'No results for "' + q + '".';
            scored.slice(0, 30).forEach(function (pair) {
                var r = pair[1];
                var li = document.createElement("li");
                li.className = "search-result";
                li.innerHTML =
                    '<a href="' + esc(root + (r.url || "")) +
                    '"><span class="search-result-title">' + esc(r.title) +
                    '</span> <span class="chip">' + esc(r.section) + "</span></a>" +
                    '<p class="search-result-snippet">' + snippet(r.text, tokens) +
                    "</p>";
                list.appendChild(li);
            });
        }

        function load() {
            if (records || loading) return;
            loading = true;
            status.textContent = "Loading index...";
            fetch(root + "data/search-index.json")
                .then(function (r) { return r.json(); })
                .then(function (data) {
                    records = data.map(function (record) {
                        record.titleLower = record.title.toLowerCase();
                        record.sectionLower = (record.section || "").toLowerCase();
                        record.textLower = record.text.toLowerCase();
                        return record;
                    });
                    loading = false;
                    run(input.value);
                })
                .catch(function () {
                    loading = false;
                    status.textContent = "Could not load the search index.";
                });
        }

        function setBackgroundInert(on) {
            if (on) {
                inerted = [];
                Array.prototype.slice.call(document.body.children).forEach(function (el) {
                    if (el === overlay || el.tagName === "SCRIPT") return;
                    inerted.push({ el: el, aria: el.getAttribute("aria-hidden"), inert: el.inert });
                    el.setAttribute("aria-hidden", "true");
                    if ("inert" in el) el.inert = true;
                });
                return;
            }
            inerted.forEach(function (item) {
                if (item.aria === null) item.el.removeAttribute("aria-hidden");
                else item.el.setAttribute("aria-hidden", item.aria);
                if ("inert" in item.el) item.el.inert = item.inert;
            });
            inerted = [];
        }

        function focusable() {
            return Array.prototype.slice.call(
                overlay.querySelectorAll('a[href], button:not([disabled]), input:not([disabled])'),
            ).filter(function (el) {
                return el.offsetWidth || el.offsetHeight || el === document.activeElement;
            });
        }

        function open() {
            if (!overlay.hidden) return;
            previousFocus = document.activeElement;
            overlay.hidden = false;
            searchBadge.setAttribute("aria-expanded", "true");
            document.body.classList.add("search-open");
            setBackgroundInert(true);
            load();
            input.focus();
            input.select();
        }

        function close() {
            if (overlay.hidden) return;
            overlay.hidden = true;
            searchBadge.setAttribute("aria-expanded", "false");
            document.body.classList.remove("search-open");
            setBackgroundInert(false);
            if (previousFocus && typeof previousFocus.focus === "function") {
                previousFocus.focus();
            }
        }

        var debounceTimer;
        input.addEventListener("input", function () {
            clearTimeout(debounceTimer);
            debounceTimer = setTimeout(function () { run(input.value); }, 120);
        });
        overlay.addEventListener("click", function (event) {
            if (event.target.hasAttribute("data-close")) close();
        });
        overlay.addEventListener("keydown", function (event) {
            if (event.key === "Escape") {
                event.preventDefault();
                close();
                return;
            }
            if (event.key !== "Tab") return;
            var nodes = focusable();
            if (!nodes.length) return;
            var first = nodes[0];
            var last = nodes[nodes.length - 1];
            if (event.shiftKey && document.activeElement === first) {
                event.preventDefault();
                last.focus();
            } else if (!event.shiftKey && document.activeElement === last) {
                event.preventDefault();
                first.focus();
            }
        });
        searchBadge.addEventListener("click", function (event) {
            event.preventDefault();
            open();
        });
        closeButton.addEventListener("click", close);
        document.addEventListener("keydown", function (event) {
            if (event.key === "Escape" && !overlay.hidden) {
                event.preventDefault();
                close();
                return;
            }
            if ((event.key === "k" || event.key === "K") && (event.metaKey || event.ctrlKey)) {
                event.preventDefault();
                open();
                return;
            }
            if (event.key === "/" && overlay.hidden) {
                var el = document.activeElement;
                var tag = el && el.tagName;
                if (tag !== "INPUT" && tag !== "TEXTAREA" && !(el && el.isContentEditable)) {
                    event.preventDefault();
                    open();
                }
            }
        });
    }

    function initSourceLinks() {
        var sources = [
            {
                match: /^(internal|pkg|cmd|docs|schema|test|plan|mk|scripts)\//,
                base: "https://codeberg.org/thomas-mangin/ze/src/branch/main/",
                forge: "forgejo",
            },
            {
                match: /^(interface-definitions|include|data|python|smoketest|op-mode-definitions)\//,
                base: "https://github.com/vyos/vyos-1x/blob/current/",
                forge: "github",
            },
            {
                match: /^src\/conf_mode\//,
                base: "https://github.com/vyos/vyos-1x/blob/current/",
                forge: "github",
            },
            {
                match: /^Makefile$/,
                base: "https://github.com/vyos/vyos-1x/blob/current/",
                forge: "github",
            },
            {
                match: /^(src\/org\/freertr|cfg|misc)\//,
                base: "https://codeberg.org/mc36/freeRtr/src/branch/master/",
                forge: "forgejo",
            },
        ];

        function lineAnchor(forge, start, end) {
            if (!start) return "";
            if (forge === "github") return "#L" + start + (end ? "-L" + end : "");
            return "#L" + start + (end ? "-L" + end : "");
        }

        function sourceFor(text) {
            var match = text.trim().match(/^([^\s:]+)(?::(\d+)(?:-(\d+))?(?:,\d+(?:-\d+)?)*)?$/);
            if (!match) return "";
            var path = match[1];
            var javaDirs = {
                cfg: "src/org/freertr/cfg/",
                rtr: "src/org/freertr/rtr/",
                tab: "src/org/freertr/tab/",
                ip: "src/org/freertr/ip/",
                serv: "src/org/freertr/serv/",
                clnt: "src/org/freertr/clnt/",
                user: "src/org/freertr/user/",
                ifc: "src/org/freertr/ifc/",
                prt: "src/org/freertr/prt/",
            };
            Object.keys(javaDirs).some(function (prefix) {
                if (path.indexOf("/") !== -1 || !new RegExp("^" + prefix + "[A-Za-z0-9_]*\\.java$").test(path)) {
                    return false;
                }
                path = javaDirs[prefix] + path;
                return true;
            });
            for (var i = 0; i < sources.length; i++) {
                if (!sources[i].match.test(path)) continue;
                return sources[i].base + path + lineAnchor(sources[i].forge, match[2], match[3]);
            }
            return "";
        }

        Array.prototype.slice.call(document.querySelectorAll(".md-content code")).forEach(function (code) {
            if (code.closest("a")) return;
            var href = sourceFor(code.textContent);
            if (!href) return;
            var link = document.createElement("a");
            link.className = "source-link";
            link.href = href;
            link.target = "_blank";
            link.rel = "noopener";
            link.setAttribute("aria-label", "Open source evidence " + code.textContent.trim());
            code.parentNode.insertBefore(link, code);
            link.appendChild(code);
        });
    }

    function initComparisonFilters() {
        var productMatchers = [
            ["Ze", /^ze(?: evidence)?$/],
            ["VyOS", /^vyos(?: evidence)?$/],
            ["freeRtr", /^freertr(?: evidence)?$/],
            ["BIRD 3", /^bird 3$/],
            ["BIRD 2", /^bird 2$/],
            ["FRR", /^frr$/],
            ["OpenBGPd", /^openbgpd$/],
            ["GoBGP", /^gobgp$/],
            ["bio-rd", /^bio-rd$/],
            ["ExaBGP", /^exabgp$/],
            ["RustyBGP", /^rustybgp$/],
            ["rustbgpd", /^rustbgpd$/],
        ];

        function productLabel(text) {
            var normalized = text.replace(/\s+/g, " ").trim().toLowerCase();
            for (var i = 0; i < productMatchers.length; i++) {
                if (productMatchers[i][1].test(normalized)) return productMatchers[i][0];
            }
            return "";
        }

        function initColumnControls(tool, content, status) {
            var tables = Array.prototype.slice.call(content.querySelectorAll("table"));
            var columns = {};
            var order = [];

            tables.forEach(function (table) {
                if (!table.rows.length) return;
                var header = table.tHead && table.tHead.rows.length ? table.tHead.rows[0] : table.rows[0];
                Array.prototype.slice.call(header.cells).forEach(function (cell, index) {
                    var label = productLabel(cell.textContent);
                    if (!label) return;
                    if (!columns[label]) {
                        columns[label] = [];
                        order.push(label);
                    }
                    columns[label].push({ table: table, index: index });
                });
            });

            if (order.length < 3) return;

            var fieldset = document.createElement("fieldset");
            fieldset.className = "compare-columns";
            fieldset.setAttribute("data-compare-columns", "");
            fieldset.innerHTML = '<legend>Show products</legend>';
            var inputs = [];

            function setColumn(entry, visible) {
                Array.prototype.slice.call(entry.table.rows).forEach(function (row) {
                    var cell = row.cells[entry.index];
                    if (!cell) return;
                    cell.hidden = !visible;
                    cell.classList.toggle("compare-column-hidden", !visible);
                });
            }

            function applyColumns() {
                order.forEach(function (label) {
                    var input = fieldset.querySelector('input[value="' + label + '"]');
                    var visible = !input || input.checked;
                    columns[label].forEach(function (entry) {
                        setColumn(entry, visible);
                    });
                });
            }

            order.forEach(function (label) {
                var wrapper = document.createElement("label");
                wrapper.className = "compare-column-toggle";
                var input = document.createElement("input");
                input.type = "checkbox";
                input.value = label;
                input.checked = true;
                input.addEventListener("change", function () {
                    if (!inputs.some(function (node) { return node.checked; })) {
                        input.checked = true;
                    }
                    applyColumns();
                });
                inputs.push(input);
                wrapper.appendChild(input);
                wrapper.appendChild(document.createTextNode(label));
                fieldset.appendChild(wrapper);
            });

            tool.insertBefore(fieldset, status);
            applyColumns();
        }

        var tools = Array.prototype.slice.call(document.querySelectorAll("[data-compare-filter]"));
        tools.forEach(function (tool) {
            var input = tool.querySelector("[data-compare-search]");
            var select = tool.querySelector("[data-compare-section]");
            var status = tool.querySelector("[data-compare-status]");
            var content = tool.closest(".md-content");
            if (!input || !select || !status || !content) return;
            initColumnControls(tool, content, status);

            var headings = Array.prototype.slice.call(content.querySelectorAll("h2"));
            var groups = headings.map(function (heading, index) {
                var nodes = [];
                var node = heading.nextElementSibling;
                while (node && node.tagName !== "H2") {
                    if (node !== tool) nodes.push(node);
                    node = node.nextElementSibling;
                }
                var rows = [];
                nodes.forEach(function (groupNode) {
                    Array.prototype.slice.call(groupNode.querySelectorAll("tbody tr")).forEach(function (row) {
                        row.compareText = row.textContent.toLowerCase();
                        rows.push(row);
                    });
                });
                var option = document.createElement("option");
                option.value = String(index);
                option.textContent = heading.textContent.trim();
                select.appendChild(option);
                return {
                    heading: heading,
                    nodes: nodes,
                    rows: rows,
                    value: option.value,
                    text: (heading.textContent + " " + nodes.map(function (n) {
                        return n.textContent;
                    }).join(" ")).toLowerCase(),
                };
            });

            function setHidden(node, hidden) {
                node.classList.toggle("compare-hidden", hidden);
            }

            function applyFilter() {
                var query = input.value.trim().toLowerCase();
                var wanted = select.value;
                var visibleGroups = 0;
                var visibleRows = 0;
                var totalRows = 0;

                groups.forEach(function (group) {
                    var sectionAllowed = !wanted || wanted === group.value;
                    var groupTextMatch = !query || group.text.indexOf(query) !== -1;
                    var groupRowsVisible = 0;

                    group.rows.forEach(function (row) {
                        totalRows += 1;
                        var rowMatch = sectionAllowed && (!query || row.compareText.indexOf(query) !== -1);
                        setHidden(row, !rowMatch);
                        if (rowMatch) {
                            visibleRows += 1;
                            groupRowsVisible += 1;
                        }
                    });

                    var showGroup = sectionAllowed && (!query || groupTextMatch || groupRowsVisible > 0);
                    setHidden(group.heading, !showGroup);
                    group.nodes.forEach(function (node) {
                        if (node.tagName === "TABLE") {
                            setHidden(node, !sectionAllowed || (query && group.rows.length && groupRowsVisible === 0));
                        } else {
                            setHidden(node, !showGroup);
                        }
                    });
                    if (showGroup) visibleGroups += 1;
                });

                if (!groups.length) {
                    status.textContent = "No sections found.";
                } else if (!query && !wanted) {
                    status.textContent = totalRows + " table rows across " + groups.length + " sections.";
                } else {
                    status.textContent = visibleRows + " matching table rows in " + visibleGroups + " sections.";
                }
            }

            input.addEventListener("input", applyFilter);
            select.addEventListener("change", applyFilter);
            applyFilter();
        });
    }

    initComparisonFilters();
    initSourceLinks();
});
