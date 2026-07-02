document.addEventListener("DOMContentLoaded", function () {
    var observer = new IntersectionObserver(
        function (entries) {
            entries.forEach(function (entry) {
                if (entry.isIntersecting) {
                    entry.target.classList.add("visible");
                    observer.unobserve(entry.target);
                }
            });
        },
        { threshold: 0.01 },
    );
    document.querySelectorAll(".reveal").forEach(function (el) {
        observer.observe(el);
    });

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
        document.addEventListener("mousemove", function (event) {
            if (event.clientY <= 12) {
                header.classList.remove("hidden");
            }
        });

        // An in-page anchor jump (e.g. clicking "Status" in the nav) triggers
        // a smooth scroll that the listener above would otherwise read as
        // "scrolling down" and hide the header mid-jump -- but
        // scroll-padding-top still reserves space for it, so the target
        // lands short. Keep the header visible for the duration of the jump.
        document.querySelectorAll('a[href*="#"]').forEach(function (link) {
            link.addEventListener("click", function () {
                suppressHide = true;
                header.classList.remove("hidden");
                if (suppressTimer) clearTimeout(suppressTimer);
                suppressTimer = setTimeout(function () {
                    suppressHide = false;
                }, 1000);
            });
        });
    }

    // Mega-menu dropdowns (Project, Labs, Docs) open on CSS :hover/
    // :focus-within alone -- no JS state to fall out of sync. Escape still
    // needs JS: it can't un-hover, so blur whatever's focused inside one.
    document.addEventListener("keydown", function (event) {
        if (event.key !== "Escape") return;
        var active = document.activeElement;
        if (active && active.closest(".nav-dropdown")) active.blur();
    });

    // Site search overlay. The search icon is a link to the /search/ page as
    // a no-JS fallback; here it instead opens an in-page modal that searches
    // the same shared index, so search works from every page without a full
    // navigation. Root is derived from the icon's own href (".../search/").
    var searchBadge = document.querySelector(".nav-badge-search");
    if (searchBadge) {
        var root = searchBadge.getAttribute("href").replace(/search\/$/, "");
        var records = null;
        var loading = false;
        var overlay = document.createElement("div");
        overlay.className = "search-overlay";
        overlay.hidden = true;
        overlay.innerHTML =
            '<div class="search-overlay-backdrop" data-close></div>' +
            '<div class="search-overlay-panel" role="dialog" aria-modal="true" ' +
            'aria-label="Search">' +
            '<input type="search" class="search-overlay-input" ' +
            'placeholder="Search docs, config, CLI, blog…" ' +
            'aria-label="Search the site" />' +
            '<p class="search-overlay-status" role="status" aria-live="polite"></p>' +
            '<ul class="search-overlay-results"></ul>' +
            "</div>";
        document.body.appendChild(overlay);
        var input = overlay.querySelector(".search-overlay-input");
        var status = overlay.querySelector(".search-overlay-status");
        var list = overlay.querySelector(".search-overlay-results");
        var ENT = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" };
        function esc(s) {
            return String(s).replace(/[&<>"]/g, function (c) { return ENT[c]; });
        }
        function snippet(text, tokens) {
            var low = text.toLowerCase(), at = -1;
            for (var i = 0; i < tokens.length; i++) {
                var p = low.indexOf(tokens[i]);
                if (p !== -1 && (at === -1 || p < at)) at = p;
            }
            if (at === -1) at = 0;
            var start = Math.max(0, at - 60);
            var frag = text.slice(start, start + 200);
            if (start > 0) frag = "…" + frag;
            if (start + 200 < text.length) frag = frag + "…";
            frag = esc(frag);
            tokens.forEach(function (t) {
                if (!t) return;
                var re = new RegExp(
                    "(" + t.replace(/[.*+?^${}()|[\]\\]/g, "\\$&") + ")", "ig"
                );
                frag = frag.replace(re, "<mark>$1</mark>");
            });
            return frag;
        }
        function score(r, tokens) {
            var title = r.title.toLowerCase();
            var section = (r.section || "").toLowerCase();
            var text = r.text.toLowerCase();
            var total = 0;
            for (var i = 0; i < tokens.length; i++) {
                var t = tokens[i], hit = 0;
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
                status.textContent = loading ? "Loading index…" : "";
                return;
            }
            var tokens = q.toLowerCase().split(/\s+/).filter(Boolean);
            if (!tokens.length) { status.textContent = ""; return; }
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
                var r = pair[1], li = document.createElement("li");
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
            status.textContent = "Loading index…";
            fetch(root + "data/search-index.json")
                .then(function (r) { return r.json(); })
                .then(function (data) { records = data; loading = false; run(input.value); })
                .catch(function () {
                    loading = false;
                    status.textContent = "Could not load the search index.";
                });
        }
        function open() {
            overlay.hidden = false;
            document.body.classList.add("search-open");
            load();
            input.focus();
            input.select();
        }
        function close() {
            overlay.hidden = true;
            document.body.classList.remove("search-open");
        }
        var debounceTimer;
        input.addEventListener("input", function () {
            clearTimeout(debounceTimer);
            debounceTimer = setTimeout(function () { run(input.value); }, 120);
        });
        overlay.addEventListener("click", function (event) {
            if (event.target.hasAttribute("data-close")) close();
        });
        searchBadge.addEventListener("click", function (event) {
            event.preventDefault();
            open();
        });
        document.addEventListener("keydown", function (event) {
            if (event.key === "Escape" && !overlay.hidden) { close(); return; }
            if ((event.key === "k" || event.key === "K") && (event.metaKey || event.ctrlKey)) {
                event.preventDefault();
                open();
                return;
            }
            if (event.key === "/" && overlay.hidden) {
                var el = document.activeElement, tag = el && el.tagName;
                if (tag !== "INPUT" && tag !== "TEXTAREA" && !(el && el.isContentEditable)) {
                    event.preventDefault();
                    open();
                }
            }
        });
    }
});
