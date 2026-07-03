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
});
