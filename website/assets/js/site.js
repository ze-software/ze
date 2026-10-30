document.documentElement.classList.add("js");

document.addEventListener("DOMContentLoaded", function () {
    var ENT = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" };
    var siteRootCache = null;
    var frontendVocab = null;
    var frontendVocabPromise = null;

    function slice(nodes) {
        return Array.prototype.slice.call(nodes || []);
    }

    function esc(s) {
        return String(s == null ? "" : s).replace(/[&<>"]/g, function (c) {
            return ENT[c];
        });
    }

    function textLower(text) {
        return String(text || "").toLowerCase();
    }

    function cleanLabel(text) {
        return String(text || "").replace(/\s+/g, " ").replace(/`/g, "").trim();
    }

    var THEME_STORAGE_KEY = "ze-theme";
    var themeMedia = window.matchMedia
        ? window.matchMedia("(prefers-color-scheme: dark)")
        : null;

    function storedTheme() {
        try {
            var value = window.localStorage.getItem(THEME_STORAGE_KEY);
            return value === "dark" || value === "light" ? value : null;
        } catch (err) {
            return null;
        }
    }

    function preferredTheme() {
        return storedTheme() || (themeMedia && themeMedia.matches ? "dark" : "light");
    }

    function syncThemeControls(theme) {
        slice(document.querySelectorAll("[data-theme-toggle]")).forEach(function (button) {
            var dark = theme === "dark";
            var label = dark ? "Use light theme" : "Use dark theme";
            button.setAttribute("aria-pressed", dark ? "true" : "false");
            button.setAttribute("aria-label", label);
            button.setAttribute("title", label);
        });
    }

    function applyTheme(theme, persist) {
        document.documentElement.setAttribute("data-theme", theme);
        syncThemeControls(theme);
        var themeColor = document.querySelector('meta[name="theme-color"]');
        if (!themeColor) {
            themeColor = document.createElement("meta");
            themeColor.setAttribute("name", "theme-color");
            document.head.appendChild(themeColor);
        }
        themeColor.setAttribute("content", theme === "dark" ? "#091b2e" : "#f7f9fc");
        if (persist) {
            try {
                window.localStorage.setItem(THEME_STORAGE_KEY, theme);
            } catch (err) {
                /* The visual change still applies when storage is unavailable. */
            }
        }
    }

    function initThemeToggle() {
        var buttons = slice(document.querySelectorAll("[data-theme-toggle]"));
        if (!buttons.length) return;
        applyTheme(
            document.documentElement.getAttribute("data-theme") || preferredTheme(),
            false,
        );
        buttons.forEach(function (button) {
            button.addEventListener("click", function () {
                var current = document.documentElement.getAttribute("data-theme");
                applyTheme(current === "dark" ? "light" : "dark", true);
            });
            button.setAttribute("data-theme-toggle-ready", "true");
        });
        if (themeMedia && themeMedia.addEventListener) {
            themeMedia.addEventListener("change", function () {
                if (!storedTheme()) applyTheme(preferredTheme(), false);
            });
        }
    }

    function initTaglineCarousel() {
        var carousel = document.querySelector("[data-tagline-carousel]");
        if (!carousel) return;

        var text = carousel.querySelector("[data-tagline-text]");
        var template = carousel.querySelector("[data-tagline-options]");
        var dotsHost = carousel.querySelector("[data-tagline-dots]");
        var previous = carousel.querySelector("[data-tagline-prev]");
        var next = carousel.querySelector("[data-tagline-next]");
        var status = carousel.querySelector("[data-tagline-status]");
        if (!text || !template || !template.content || !dotsHost || !previous || !next) return;

        var taglines = slice(template.content.querySelectorAll("span")).map(function (option) {
            return cleanLabel(option.textContent);
        }).filter(Boolean);
        if (taglines.length < 2) return;

        var current = 0;
        var transitionTimer = null;
        var autoplayTimer = null;
        var hovered = false;
        var focused = false;
        var reduceMotion = window.matchMedia &&
            window.matchMedia("(prefers-reduced-motion: reduce)").matches;
        var dots = taglines.map(function (tagline, index) {
            var button = document.createElement("button");
            button.type = "button";
            button.className = "tagline-carousel-dot";
            button.setAttribute("aria-label", "Show tagline " + (index + 1) + ": " + tagline);
            button.setAttribute("aria-controls", "hero-title");
            button.addEventListener("click", function () {
                show(index, true);
            });
            dotsHost.appendChild(button);
            return button;
        });

        function updateDots() {
            dots.forEach(function (dot, index) {
                if (index === current) {
                    dot.setAttribute("aria-current", "true");
                } else {
                    dot.removeAttribute("aria-current");
                }
            });
        }

        function announce() {
            if (!status) return;
            status.textContent =
                "Showing tagline " + (current + 1) + " of " + taglines.length + ": " + taglines[current];
        }

        function commit(index, manual) {
            current = (index + taglines.length) % taglines.length;
            text.textContent = taglines[current];
            updateDots();
            if (manual) announce();
            window.requestAnimationFrame(function () {
                carousel.classList.remove("is-changing");
            });
        }

        function show(index, manual) {
            window.clearTimeout(transitionTimer);
            carousel.classList.remove("is-changing");
            if (reduceMotion) {
                commit(index, manual);
                return;
            }
            carousel.classList.add("is-changing");
            transitionTimer = window.setTimeout(function () {
                commit(index, manual);
            }, 180);
        }

        function stopAutoplay() {
            window.clearTimeout(autoplayTimer);
            autoplayTimer = null;
        }

        function scheduleAutoplay() {
            stopAutoplay();
            if (reduceMotion || hovered || focused || document.hidden) return;
            autoplayTimer = window.setTimeout(function () {
                show(current + 1, false);
                scheduleAutoplay();
            }, 6000);
        }

        previous.setAttribute("aria-controls", "hero-title");
        next.setAttribute("aria-controls", "hero-title");
        previous.addEventListener("click", function () {
            show(current - 1, true);
            scheduleAutoplay();
        });
        next.addEventListener("click", function () {
            show(current + 1, true);
            scheduleAutoplay();
        });
        carousel.addEventListener("mouseenter", function () {
            hovered = true;
            stopAutoplay();
        });
        carousel.addEventListener("mouseleave", function () {
            hovered = false;
            scheduleAutoplay();
        });
        carousel.addEventListener("focusin", function () {
            focused = true;
            stopAutoplay();
        });
        carousel.addEventListener("focusout", function () {
            window.setTimeout(function () {
                focused = carousel.contains(document.activeElement);
                scheduleAutoplay();
            }, 0);
        });
        document.addEventListener("visibilitychange", scheduleAutoplay);

        updateDots();
        scheduleAutoplay();
    }

    function escapeRegExp(text) {
        return String(text || "").replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    }

    function siteRootFromScript() {
        if (siteRootCache !== null) return siteRootCache;
        var script = document.querySelector('script[src*="assets/site.js"]');
        if (!script) {
            siteRootCache = "";
            return siteRootCache;
        }
        var src = script.getAttribute("src") || "";
        var clean = src.split("#")[0].split("?")[0];
        var marker = "assets/site.js";
        var at = clean.indexOf(marker);
        siteRootCache = at === -1 ? "" : clean.slice(0, at);
        return siteRootCache;
    }

    function loadFrontendVocab() {
        if (frontendVocab) return Promise.resolve(frontendVocab);
        if (frontendVocabPromise) return frontendVocabPromise;
        if (!window.fetch) {
            frontendVocab = {};
            return Promise.resolve(frontendVocab);
        }
        frontendVocabPromise = fetch(siteRootFromScript() + "data/frontend-vocab.json")
            .then(function (response) {
                if (!response.ok) throw new Error("vocab");
                return response.json();
            })
            .then(function (data) {
                frontendVocab = data || {};
                return frontendVocab;
            })
            .catch(function () {
                frontendVocab = {};
                return frontendVocab;
            });
        return frontendVocabPromise;
    }

    function lookupFact(facts, key) {
        var value = facts;
        String(key || "").split(".").forEach(function (part) {
            if (value && Object.prototype.hasOwnProperty.call(value, part)) {
                value = value[part];
            } else {
                value = null;
            }
        });
        return value;
    }

    // The snapshot is fetched at most once per page load, however many
    // subtrees ask for it: the body fills at load and the shared header fills
    // after it is inserted, and both read this one promise. It answers null
    // when the snapshot cannot be read, and every element keeps the value the
    // last build wrote into it.
    var siteFactsPromise = null;

    function loadSiteFacts() {
        if (siteFactsPromise) return siteFactsPromise;
        siteFactsPromise = fetch(siteRootFromScript() + "data/site-facts.json")
            .then(function (response) {
                if (!response.ok) throw new Error("site facts");
                return response.json();
            })
            .catch(function () {
                return null;
            });
        return siteFactsPromise;
    }

    // fillRepoStats replaces the text of every [data-ze-stat] element under
    // scope with the snapshot's value for its dotted key. It writes
    // textContent only, so a snapshot value can never become markup.
    //
    // A [data-ze-stat-width] element is a bar drawn from a percentage: its
    // width is set from the snapshot's value for its key, so the bar and the
    // figure beside it move together. A value that is not a number from 0 to
    // 100 leaves the width the last build wrote.
    function fillRepoStats(scope) {
        if (!scope || !window.fetch) return;
        var nodes = slice(scope.querySelectorAll("[data-ze-stat]"));
        var bars = slice(scope.querySelectorAll("[data-ze-stat-width]"));
        if (!nodes.length && !bars.length) return;
        loadSiteFacts().then(function (facts) {
            if (!facts) return;
            nodes.forEach(function (node) {
                var value = lookupFact(facts, node.getAttribute("data-ze-stat"));
                if (value !== null && value !== undefined) {
                    node.textContent = String(value);
                }
            });
            bars.forEach(function (bar) {
                var value = lookupFact(facts, bar.getAttribute("data-ze-stat-width"));
                if (value === null || value === undefined || String(value).trim() === "") return;
                var percent = Number(value);
                if (!isFinite(percent) || percent < 0 || percent > 100) return;
                bar.style.width = percent + "%";
            });
        });
    }


    function debounce(fn, ms) {
        var timer;
        return function () {
            var args = arguments;
            clearTimeout(timer);
            timer = setTimeout(function () {
                fn.apply(null, args);
            }, ms);
        };
    }

    function loadSharedHeader() {
        var mount = document.getElementById("site-header-mount");
        if (!mount) return Promise.resolve();

        // The generator only ever writes a run of "../" for the root and the
        // header fragment below it. Anything else is not trusted: the root is
        // spliced into links and into the fetched header's markup.
        var root = mount.getAttribute("data-site-root") || "";
        if (!/^(\.\.\/)*$/.test(root)) root = "";
        var source = mount.getAttribute("data-header-src") || "";
        if (!/^(\.\.\/)*assets\/header\.html$/.test(source)) source = root + "assets/header.html";

        function renderFallback() {
            var header = document.createElement("header");
            header.className = "site-header";

            var nav = document.createElement("nav");
            nav.className = "nav";
            nav.setAttribute("aria-label", "Main navigation");

            var brand = document.createElement("a");
            brand.className = "brand";
            brand.setAttribute("href", root + "#top");
            brand.setAttribute("aria-label", "Ze home");

            var logo = document.createElement("img");
            logo.setAttribute("src", root + "assets/ze.svg");
            logo.setAttribute("alt", "");
            logo.setAttribute("width", "32");
            logo.setAttribute("height", "32");

            var name = document.createElement("span");
            name.textContent = "Ze";

            brand.appendChild(logo);
            brand.appendChild(name);
            nav.appendChild(brand);
            header.appendChild(nav);
            mount.parentNode.replaceChild(header, mount);
        }

        if (!window.fetch) {
            renderFallback();
            return Promise.resolve();
        }

        return fetch(source)
            .then(function (response) {
                if (!response.ok) throw new Error("shared header");
                return response.text();
            })
            .then(function (markup) {
                mount.outerHTML = markup
                    .split("__ZE_SITE_ROOT__")
                    .join(root);
            })
            .catch(renderFallback);
    }

    var reveals = slice(document.querySelectorAll(".reveal"));
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
                // A long reference section can exceed one hundred viewports.
                // Reveal on any intersection, not a fraction it cannot expose.
                { threshold: 0 },
            );
            reveals.forEach(function (el) {
                el.classList.add("reveal-pending");
                observer.observe(el);
            });
        }
    }

    function initNavigation() {
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
    }

    function searchTokens(query) {
        return String(query || "").toLowerCase().split(/\s+/).filter(Boolean);
    }

    function normalizeSearchRecord(record) {
        var title = record.displayTitle || record.title || "";
        var section = record.displaySection || record.section || "";
        var text = record.text || "";
        return {
            title: record.title || title,
            displayTitle: title,
            section: record.section || section,
            displaySection: section,
            url: record.url || "",
            text: text,
            titleLower: textLower((record.title || "") + " " + title),
            sectionLower: textLower((record.section || "") + " " + section),
            textLower: textLower(text),
        };
    }

    function normalizeSearchRecords(data) {
        return (data || []).map(normalizeSearchRecord);
    }

    function searchSnippet(text, tokens) {
        var source = String(text || "");
        var low = source.toLowerCase();
        var at = -1;
        for (var i = 0; i < tokens.length; i++) {
            var p = low.indexOf(tokens[i]);
            if (p !== -1 && (at === -1 || p < at)) at = p;
        }
        if (at === -1) at = 0;
        var start = Math.max(0, at - 60);
        var frag = source.slice(start, start + 200);
        if (start > 0) frag = "..." + frag;
        if (start + 200 < source.length) frag = frag + "...";
        frag = esc(frag);
        tokens.forEach(function (token) {
            if (!token) return;
            var re = new RegExp("(" + escapeRegExp(token) + ")", "ig");
            frag = frag.replace(re, "<mark>$1</mark>");
        });
        return frag;
    }

    function scoreSearchRecord(record, tokens) {
        var total = 0;
        for (var i = 0; i < tokens.length; i++) {
            var token = tokens[i];
            var hit = 0;
            if (record.titleLower.indexOf(token) !== -1) hit += 12;
            if (record.sectionLower.indexOf(token) !== -1) hit += 4;
            var bodyHits = record.textLower.split(token).length - 1;
            if (bodyHits > 0) hit += Math.min(bodyHits, 5);
            if (hit === 0) return 0;
            total += hit;
        }
        return total;
    }

    function renderSearchResults(config) {
        var query = config.query;
        var records = config.records || [];
        var list = config.list;
        var status = config.status;
        var root = config.root || "";
        var limit = config.limit || 30;
        var emptyPrefix = config.emptyPrefix || '"';
        var emptySuffix = config.emptySuffix || emptyPrefix;
        var tokens = searchTokens(query);
        list.innerHTML = "";
        if (!tokens.length) {
            status.textContent = "";
            return [];
        }
        var scored = [];
        for (var i = 0; i < records.length; i++) {
            var score = scoreSearchRecord(records[i], tokens);
            if (score > 0) scored.push([score, records[i]]);
        }
        scored.sort(function (a, b) { return b[0] - a[0]; });
        status.textContent = scored.length
            ? scored.length + " result" + (scored.length === 1 ? "" : "s")
            : "No results for " + emptyPrefix + query + emptySuffix + ".";
        scored.slice(0, limit).forEach(function (pair) {
            var record = pair[1];
            var li = document.createElement("li");
            li.className = "search-result";
            li.innerHTML =
                '<a href="' + esc(root + record.url) + '"><span class="search-result-title">' +
                esc(record.displayTitle || record.title) + '</span> <span class="chip">' +
                esc(record.displaySection || record.section) + '</span></a>' +
                '<p class="search-result-snippet">' + searchSnippet(record.text, tokens) + "</p>";
            list.appendChild(li);
        });
        return scored;
    }

    function initSearchOverlay() {
        var searchTriggers = slice(document.querySelectorAll(".nav-badge-search, .search-trigger"));
        var searchBadge = searchTriggers[0];
        if (!searchBadge) return;

        var badgeHref = searchBadge.getAttribute("href") || siteRootFromScript() + "search/";
        var root = badgeHref.replace(/search\/?$/, "");
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
            'aria-label="Search the site" aria-describedby="search-overlay-help" />' +
            '<p id="search-overlay-help" class="search-overlay-help search-overlay-shortcuts">Press Ctrl+K, ⌘K, or / to search. Press Escape to close.</p>' +
            '<p class="search-overlay-status" role="status" aria-live="polite"></p>' +
            '<ul class="search-overlay-results" aria-label="Search results"></ul>' +
            "</div>";
        document.body.appendChild(overlay);

        var input = overlay.querySelector(".search-overlay-input");
        var status = overlay.querySelector(".search-overlay-status");
        var list = overlay.querySelector(".search-overlay-results");
        var closeButton = overlay.querySelector(".search-overlay-close");

        searchTriggers.forEach(function (trigger) {
            trigger.setAttribute("aria-haspopup", "dialog");
            trigger.setAttribute("aria-expanded", "false");
            trigger.setAttribute("aria-keyshortcuts", "Control+K Meta+K /");
        });

        function run(query) {
            if (!records) {
                list.innerHTML = "";
                status.textContent = loading ? "Loading index..." : "";
                return;
            }
            renderSearchResults({
                query: query,
                records: records,
                list: list,
                status: status,
                root: root,
                limit: 30,
                emptyQuote: '"',
            });
        }

        function load() {
            if (records || loading) return;
            loading = true;
            status.textContent = "Loading index...";
            fetch(root + "data/search-index.json")
                .then(function (response) { return response.json(); })
                .then(function (data) {
                    records = normalizeSearchRecords(data);
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
                slice(document.body.children).forEach(function (el) {
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
            return slice(overlay.querySelectorAll('a[href], button:not([disabled]), input:not([disabled])'))
                .filter(function (el) {
                    return el.offsetWidth || el.offsetHeight || el === document.activeElement;
                });
        }

        function setExpanded(value) {
            searchTriggers.forEach(function (trigger) {
                trigger.setAttribute("aria-expanded", value);
            });
        }

        function open() {
            if (!overlay.hidden) return;
            previousFocus = document.activeElement;
            overlay.hidden = false;
            setExpanded("true");
            document.body.classList.add("search-open");
            setBackgroundInert(true);
            load();
            input.focus();
            input.select();
        }

        function close() {
            if (overlay.hidden) return;
            overlay.hidden = true;
            setExpanded("false");
            document.body.classList.remove("search-open");
            setBackgroundInert(false);
            if (previousFocus && typeof previousFocus.focus === "function") {
                previousFocus.focus();
            }
        }

        input.addEventListener("input", debounce(function () { run(input.value); }, 120));
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
        searchTriggers.forEach(function (trigger) {
            trigger.addEventListener("click", function (event) {
                event.preventDefault();
                open();
            });
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

    function initSearchPage() {
        var input = document.getElementById("site-search");
        var status = document.getElementById("search-status");
        var list = document.getElementById("search-results");
        if (!input || !status || !list) return;

        var records = null;
        function run(query) {
            if (!records) return;
            renderSearchResults({
                query: query,
                records: records,
                list: list,
                status: status,
                root: siteRootFromScript(),
                limit: 40,
                emptyPrefix: "\u201c",
                emptySuffix: "\u201d",
            });
        }

        var params = new URLSearchParams(location.search);
        var initial = params.get("q") || "";
        if (initial) input.value = initial;

        status.textContent = "Loading index...";
        fetch(siteRootFromScript() + "data/search-index.json")
            .then(function (response) { return response.json(); })
            .then(function (data) {
                records = normalizeSearchRecords(data);
                status.textContent = "";
                if (input.value) run(input.value);
            })
            .catch(function () {
                status.textContent = "Could not load the search index.";
            });

        input.addEventListener("input", debounce(function () {
            run(input.value);
            var url = new URL(location);
            if (input.value) url.searchParams.set("q", input.value);
            else url.searchParams.delete("q");
            history.replaceState(null, "", url);
        }, 120));
    }

    function pressedCategory(buttons, activeButton) {
        buttons.forEach(function (button) {
            button.setAttribute("aria-pressed", button === activeButton ? "true" : "false");
        });
    }

    function findButtonForCategory(buttons, category) {
        for (var i = 0; i < buttons.length; i++) {
            if (buttons[i].getAttribute("data-cat") === category) return buttons[i];
        }
        return null;
    }

    function initCategoryFilter(config) {
        var buttons = slice(document.querySelectorAll(config.buttonSelector));
        var items = slice(document.querySelectorAll(config.itemSelector));
        if (!buttons.length || !items.length) return;

        var containers = config.containerSelector ? slice(document.querySelectorAll(config.containerSelector)) : [];
        var status = config.statusSelector ? document.querySelector(config.statusSelector) : null;
        var empty = config.emptySelector ? document.querySelector(config.emptySelector) : null;
        var categoryAttr = config.categoryAttr || "data-cat";
        var listAttr = config.listAttr || null;

        function itemMatches(item, category) {
            if (!category) return true;
            if (listAttr) {
                return (item.getAttribute(listAttr) || "").split(/\s+/).indexOf(category) !== -1;
            }
            return item.getAttribute(categoryAttr) === category;
        }

        function updateStatus(category, shown) {
            if (!status) return;
            var total = items.length;
            if (!category) {
                status.textContent = "Showing all " + total + " " + (config.statusName || "items") + ".";
            } else {
                status.textContent = "Showing " + shown + " " + (config.statusName || "items") + " for " + category + ".";
            }
        }

        function applyFilter(category) {
            var shown = 0;
            items.forEach(function (item) {
                var hit = itemMatches(item, category);
                item.classList.toggle("filtered-out", !hit);
                if (hit) shown += 1;
            });
            containers.forEach(function (container) {
                var visible = container.querySelector(config.visibleSelector);
                container.style.display = visible ? "" : "none";
            });
            if (empty) empty.classList.toggle("filtered-out", shown !== 0);
            updateStatus(category, shown);
        }

        buttons.forEach(function (button) {
            button.addEventListener("click", function () {
                var wasPressed = button.getAttribute("aria-pressed") === "true";
                pressedCategory(buttons, null);
                if (wasPressed) {
                    applyFilter(null);
                    return;
                }
                button.setAttribute("aria-pressed", "true");
                applyFilter(button.getAttribute("data-cat"));
            });
        });

        var hashCat = location.hash.replace("#", "");
        var hashButton = hashCat ? findButtonForCategory(buttons, hashCat) : null;
        if (hashButton) {
            pressedCategory(buttons, hashButton);
            applyFilter(hashCat);
        } else {
            updateStatus(null, items.length);
        }
    }

    function initFeatureFilters() {
        var input = document.getElementById("feature-search");
        if (!input) {
            initCategoryFilter({
                buttonSelector: ".legend button[data-cat]",
                itemSelector: ".card[data-cat]",
                containerSelector: "section[data-cards]",
                visibleSelector: ".card[data-cat]:not(.filtered-out)",
                statusSelector: "#feature-filter-status",
                statusName: "features",
            });
            return;
        }
        var cards = slice(document.querySelectorAll(".feature-card[data-cat]"));
        var buttons = slice(document.querySelectorAll(".legend button[data-cat]"));
        var sections = slice(document.querySelectorAll("section[data-cards]"));
        var maturity = document.getElementById("feature-maturity");
        var status = document.getElementById("feature-filter-status");
        var empty = document.getElementById("feature-empty");
        var category = "";
        var records = cards.map(function (card) {
            return { card: card, text: textLower(card.textContent).replace(/\s+/g, " ") };
        });
        function apply() {
            var terms = textLower(input.value).trim().split(/\s+/).filter(Boolean);
            var selected = maturity ? maturity.value : "";
            var count = 0;
            records.forEach(function (record) {
                var card = record.card;
                var match = (!category || card.getAttribute("data-cat") === category)
                    && (!selected || selected === "all" || card.getAttribute("data-maturity") === selected)
                    && terms.every(function (term) { return record.text.indexOf(term) !== -1; });
                card.classList.toggle("filtered-out", !match);
                if (match) count += 1;
            });
            sections.forEach(function (section) {
                section.hidden = !section.querySelector(".feature-card:not(.filtered-out)");
            });
            buttons.forEach(function (button) {
                button.setAttribute("aria-pressed", button.getAttribute("data-cat") === category ? "true" : "false");
            });
            if (status) status.textContent = "Showing " + count + " of " + cards.length + " features.";
            if (empty) empty.hidden = count !== 0;
        }
        function categoryFromHash() {
            var value;
            try { value = decodeURIComponent(location.hash.slice(1)); } catch (err) { return; }
            category = buttons.some(function (button) { return button.getAttribute("data-cat") === value; }) ? value : "";
            apply();
        }
        function writeHash() {
            history.replaceState(null, "", location.pathname + location.search + (category ? "#" + encodeURIComponent(category) : ""));
        }
        buttons.forEach(function (button) {
            button.addEventListener("click", function () {
                var value = button.getAttribute("data-cat");
                category = category === value ? "" : value;
                writeHash();
                apply();
            });
        });
        input.addEventListener("input", apply);
        if (maturity) maturity.addEventListener("change", apply);
        slice(document.querySelectorAll("[data-reset-features]")).forEach(function (button) {
            button.addEventListener("click", function () {
                input.value = "";
                if (maturity) maturity.value = "all";
                category = "";
                writeHash();
                apply();
                input.focus();
            });
        });
        window.addEventListener("hashchange", categoryFromHash);
        categoryFromHash();
        slice(document.querySelectorAll(".feature-filter-controls")).forEach(function (control) { control.hidden = false; });
        var catalog = document.querySelector(".feature-catalog");
        if (catalog) catalog.setAttribute("data-enhanced", "true");
    }

    function initTimelineFilters() {
        initCategoryFilter({
            buttonSelector: ".legend button[data-cat]",
            itemSelector: ".tl-item[data-cat]",
            containerSelector: ".tl-quarter[data-quarter]",
            visibleSelector: ".tl-item[data-cat]:not(.filtered-out)",
            statusName: "milestones",
        });
    }

    function initChangesFilters() {
        initCategoryFilter({
            buttonSelector: ".ch-filters button[data-cat]",
            itemSelector: ".ch-week[data-cats]",
            listAttr: "data-cats",
            emptySelector: ".ch-empty",
            statusName: "weeks",
        });
    }

    function initDependencyFilter() {
        var input = document.getElementById("dep-search");
        var groups = slice(document.querySelectorAll(".dep-group"));
        if (!input || !groups.length) return;
        input.addEventListener("input", function () {
            var query = input.value.trim().toLowerCase();
            groups.forEach(function (group) {
                var rows = slice(group.querySelectorAll("tbody tr"));
                var anyVisible = false;
                rows.forEach(function (row) {
                    var match = query === "" || row.textContent.toLowerCase().indexOf(query) !== -1;
                    row.style.display = match ? "" : "none";
                    if (match) anyVisible = true;
                });
                group.style.display = anyVisible ? "" : "none";
                if (query !== "") group.open = anyVisible;
            });
        });
    }

    function initCliCatalogFilter() {
        var input = document.getElementById("cli-search");
        var suggestions = document.getElementById("cli-suggestions");
        var groups = slice(document.querySelectorAll(".cli-group"));
        if (!input || !groups.length) return;
        var groupFilter = document.getElementById("cli-group-filter");
        var status = document.getElementById("cli-filter-status");
        var empty = document.getElementById("cli-empty");
        var toolbar = document.querySelector(".cli-toolbar");
        var disclosures = slice(document.querySelectorAll(".cli-group, .cli-command-facts, .cli-pipes"));
        var savedDisclosures = null;
        var commands = [];
        function normalize(text) { return textLower(text).replace(/\s+/g, " ").trim(); }
        groups.forEach(function (group) {
            var summary = group.querySelector("summary");
            var label = summary && summary.firstChild ? summary.firstChild.textContent.trim() : "";
            if (groupFilter) group.open = false;
            slice(group.querySelectorAll("tbody tr")).forEach(function (row) {
                commands.push({ id: row.id, path: row.cells[0].textContent.trim(),
                    desc: row.cells[2].textContent.trim(), text: normalize(row.textContent),
                    group: label, row: row, details: group });
            });
        });
        function highlight(row) {
            row.classList.add("cli-row-highlight");
            window.setTimeout(function () { row.classList.remove("cli-row-highlight"); }, 2000);
        }
        function applyRowFilter() {
            var terms = normalize(input.value).split(" ").filter(Boolean);
            var selected = groupFilter && groupFilter.value !== "all" ? groupFilter.value : "";
            var filtering = terms.length > 0 || !!selected;
            if (filtering && !savedDisclosures) {
                savedDisclosures = disclosures.map(function (detail) { return { detail: detail, open: detail.open }; });
            }
            var count = 0;
            commands.forEach(function (command) {
                var match = (!selected || command.details.id === selected)
                    && terms.every(function (term) { return command.text.indexOf(term) !== -1; });
                command.row.hidden = !match;
                if (match) count += 1;
                if (filtering) {
                    slice(command.row.querySelectorAll("details")).forEach(function (detail) {
                        if (match && terms.some(function (term) { return normalize(detail.textContent).indexOf(term) !== -1; })) detail.open = true;
                    });
                }
            });
            var visibleGroups = 0;
            groups.forEach(function (group) {
                var visible = commands.filter(function (command) { return command.details === group && !command.row.hidden; }).length;
                group.hidden = visible === 0;
                if (visible) visibleGroups += 1;
                if (filtering) group.open = visible > 0;
                var badge = group.querySelector(".cli-group-count");
                if (badge) {
                    if (!badge.hasAttribute("data-total")) badge.setAttribute("data-total", badge.textContent.trim());
                    badge.textContent = filtering ? visible + " / " + badge.getAttribute("data-total") : badge.getAttribute("data-total");
                }
            });
            if (!filtering && savedDisclosures) {
                savedDisclosures.forEach(function (saved) { saved.detail.open = saved.open; });
                savedDisclosures = null;
            }
            if (status) status.textContent = count + " of " + commands.length + " commands across " + visibleGroups + " groups.";
            if (empty) empty.hidden = count !== 0;
        }
        function reset() {
            input.value = "";
            if (groupFilter) groupFilter.value = "all";
            applyRowFilter();
            if (suggestions) { suggestions.hidden = true; suggestions.innerHTML = ""; }
        }
        function reveal(target) {
            reset();
            var ancestor = target;
            while (ancestor && ancestor !== document.body) {
                if (ancestor.tagName === "DETAILS") ancestor.open = true;
                ancestor = ancestor.parentElement;
            }
            slice(target.querySelectorAll("details")).forEach(function (detail) { detail.open = true; });
            var columnHost = toolbar && toolbar.querySelector("[data-column-selector]");
            if (columnHost) columnHost.dispatchEvent(new Event("ze:show-all-columns"));
            target.scrollIntoView({ block: "center" });
            if (target.tagName === "TR") highlight(target);
        }
        function jumpTo(command) {
            history.replaceState(null, "", "#" + command.id);
            reveal(command.row);
        }
        function renderSuggestions() {
            if (!suggestions) return;
            var terms = normalize(input.value).split(" ").filter(Boolean);
            suggestions.innerHTML = "";
            if (!terms.length) { suggestions.hidden = true; return; }
            var matches = commands.filter(function (command) {
                return !command.row.hidden && terms.every(function (term) {
                    return normalize(command.path + " " + command.desc).indexOf(term) !== -1;
                });
            }).slice(0, 20);
            matches.forEach(function (command) {
                var button = document.createElement("button");
                button.type = "button";
                var code = document.createElement("code"); code.textContent = command.path;
                var group = document.createElement("span"); group.className = "cli-suggestion-group"; group.textContent = command.group;
                button.appendChild(code); button.appendChild(group);
                button.addEventListener("click", function () { jumpTo(command); });
                suggestions.appendChild(button);
            });
            suggestions.hidden = !matches.length;
        }
        function filter() { applyRowFilter(); renderSuggestions(); }
        input.addEventListener("input", filter);
        if (groupFilter) groupFilter.addEventListener("change", filter);
        input.addEventListener("keydown", function (event) {
            if (event.key === "Escape" && suggestions) suggestions.hidden = true;
        });
        document.addEventListener("click", function (event) {
            if (suggestions && !suggestions.hidden && event.target !== input && !suggestions.contains(event.target)) suggestions.hidden = true;
        });
        slice(document.querySelectorAll("[data-reset-cli]")).forEach(function (button) {
            button.addEventListener("click", function () { reset(); input.focus(); });
        });
        slice(document.querySelectorAll("[data-cli-expand]")).forEach(function (button) {
            button.addEventListener("click", function () {
                var open = button.getAttribute("data-cli-expand") === "all";
                groups.forEach(function (group) { if (!group.hidden) group.open = open; });
            });
        });
        function revealHash() {
            var id;
            try { id = decodeURIComponent(location.hash.slice(1)); } catch (err) { return; }
            var target = id && document.getElementById(id);
            if (target && (target.tagName === "TR" || target.classList.contains("cli-group"))) {
                window.setTimeout(function () { reveal(target); }, 0);
            }
        }
        window.addEventListener("hashchange", revealHash);
        slice(document.querySelectorAll('.cli-group-nav a[href^="#"]')).forEach(function (link) {
            link.addEventListener("click", function (event) {
                if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
                if (link.getAttribute("href") === location.hash) revealHash();
            });
        });
        if (toolbar) toolbar.hidden = false;
        applyRowFilter();
        revealHash();
    }

    function initCommandEquivalentFilter() {
        var input = document.getElementById("cmd-eq-search");
        var counter = document.getElementById("cmd-eq-search-count");
        var groups = slice(document.querySelectorAll(".cmd-eq-group"));
        var mappedFirst = document.querySelector(".cmd-eq-mapped-first");
        if (!input || !groups.length) return;

        function filterRows(container, query, countMatches) {
            var rows = slice(container.querySelectorAll("tbody tr"));
            var visible = 0;
            rows.forEach(function (row) {
                var haystack = row.getAttribute("data-search") || row.textContent.toLowerCase();
                var match = query === "" || haystack.indexOf(query) !== -1;
                row.style.display = match ? "" : "none";
                if (match) visible += 1;
            });
            container.style.display = visible ? "" : "none";
            if (query !== "" && "open" in container) container.open = visible > 0;
            return countMatches ? visible : 0;
        }

        function applyFilter() {
            var query = input.value.trim().toLowerCase();
            var visible = 0;
            if (mappedFirst) filterRows(mappedFirst, query, false);
            groups.forEach(function (group) {
                visible += filterRows(group, query, true);
            });
            if (counter) counter.textContent = query === "" ? "" : visible + " matching commands";
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
    }

    function initSourceLinks() {
        var codes = slice(document.querySelectorAll(".md-content code"));
        if (!codes.length) return;
        loadFrontendVocab().then(function (vocab) {
            // A rule with a "scope" list only applies on pages whose path holds
            // one of its fragments. Foreign-project paths (VyOS "data/", freeRtr
            // "cfg/", a bare "Makefile") are ordinary words elsewhere on the
            // site, so an unscoped rule turns prose and our own data/*.json into
            // links to somebody else's repository.
            var page = location.pathname;
            var sources = (vocab.sourceLinks || []).filter(function (source) {
                var scope = source.scope || [];
                if (!scope.length) return true;
                return scope.some(function (fragment) {
                    return page.indexOf(fragment) !== -1;
                });
            }).map(function (source) {
                return {
                    match: new RegExp(source.match),
                    base: source.base,
                    forge: source.forge,
                };
            });
            var javaDirs = vocab.sourceJavaDirs || {};
            if (!sources.length) return;

            function lineAnchor(forge, start, end) {
                if (!start) return "";
                if (forge === "github") return "#L" + start + (end ? "-L" + end : "");
                return "#L" + start + (end ? "-L" + end : "");
            }

            function sourceFor(text) {
                var match = text.trim().match(/^([^\s:]+)(?::(\d+)(?:-(\d+))?(?:,\d+(?:-\d+)?)*)?$/);
                if (!match) return "";
                var path = match[1];
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

            codes.forEach(function (code) {
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
        });
    }

    function initFeatureTooltips() {
        var tables = slice(document.querySelectorAll(".md-content table"));
        if (!tables.length) return;
        loadFrontendVocab().then(function (vocab) {
            var productNames = vocab.productNames || [];
            var acronymGlossary = vocab.acronymGlossary || [];
            var featureGlossary = vocab.featureGlossary || {};
            if (!productNames.length) return;

            function productHeaderCount(table) {
                if (!table.rows.length) return 0;
                var header = table.tHead && table.tHead.rows.length ? table.tHead.rows[0] : table.rows[0];
                return slice(header.cells).filter(function (cell) {
                    var label = cleanLabel(cell.textContent);
                    return productNames.indexOf(label) !== -1;
                }).length;
            }

            function expandedLabel(label) {
                var expansions = [];
                acronymGlossary.slice().sort(function (a, b) { return b[0].length - a[0].length; }).forEach(function (entry) {
                    var pattern = new RegExp("(^|[^A-Za-z0-9+/-])" + escapeRegExp(entry[0]) + "(?=$|[^A-Za-z0-9+/-])");
                    if (pattern.test(label)) expansions.push(entry[1]);
                });
                if (!expansions.length) return label;
                return label + " (" + expansions.join("; ") + ")";
            }

            function sectionFor(node) {
                var current = node;
                while (current) {
                    current = current.previousElementSibling;
                    if (current && current.tagName === "H2") return cleanLabel(current.textContent);
                }
                return "";
            }

            function descriptionFor(label, section) {
                var key = cleanLabel(label).toLowerCase();
                if (featureGlossary[key]) return featureGlossary[key];
                var description = "Compares product support for " + expandedLabel(label) + ".";
                description += " Read across the product columns. Yes or Present means supporting evidence was found, Partial means incomplete or delegated support, Unclear means the evidence was not strong enough, and No or Not found means the inspected sources did not show support.";
                if (section) description += " Section: " + section + ".";
                return description;
            }

            var targets = [];
            tables.forEach(function (table) {
                if (table.classList.contains("cmd-eq-table") || table.closest(".command-equivalents")) return;
                if (productHeaderCount(table) < 2) return;
                var section = sectionFor(table);
                slice(table.querySelectorAll("tbody tr")).forEach(function (row) {
                    var cell = row.cells[0];
                    if (!cell || cell.querySelector(".feature-help")) return;
                    var label = cleanLabel(cell.textContent);
                    if (!label || label === "---") return;
                    var help = document.createElement("span");
                    var description = descriptionFor(label, section);
                    help.className = "feature-help";
                    help.tabIndex = 0;
                    help.textContent = label;
                    help.title = description;
                    help.setAttribute("aria-label", label + ": " + description);
                    help.setAttribute("data-feature-help", description);
                    cell.textContent = "";
                    cell.appendChild(help);
                    targets.push(help);
                });
            });

            if (!targets.length) return;

            var popover = document.createElement("div");
            popover.className = "feature-tooltip-popover";
            popover.id = "feature-tooltip-popover";
            popover.setAttribute("role", "tooltip");
            popover.hidden = true;
            document.body.appendChild(popover);

            function position(target) {
                var rect = target.getBoundingClientRect();
                var tip = popover.getBoundingClientRect();
                var margin = 12;
                var left = Math.max(margin, Math.min(rect.left, window.innerWidth - tip.width - margin));
                var top = rect.bottom + 10;
                if (top + tip.height > window.innerHeight - margin) {
                    top = Math.max(margin, rect.top - tip.height - 10);
                }
                popover.style.left = left + "px";
                popover.style.top = top + "px";
            }

            function show(target) {
                popover.textContent = target.getAttribute("data-feature-help");
                popover.hidden = false;
                popover.setAttribute("data-visible", "true");
                target.setAttribute("aria-describedby", popover.id);
                position(target);
            }

            function hide(target) {
                popover.removeAttribute("data-visible");
                popover.hidden = true;
                if (target) target.removeAttribute("aria-describedby");
            }

            targets.forEach(function (target) {
                target.addEventListener("mouseenter", function () { show(target); });
                target.addEventListener("mouseleave", function () { hide(target); });
                target.addEventListener("focus", function () { show(target); });
                target.addEventListener("blur", function () { hide(target); });
            });
            window.addEventListener("scroll", function () {
                var active = document.activeElement;
                if (active && active.classList && active.classList.contains("feature-help") && !popover.hidden) {
                    position(active);
                }
            }, true);
            window.addEventListener("resize", function () {
                var active = document.activeElement;
                if (active && active.classList && active.classList.contains("feature-help") && !popover.hidden) {
                    position(active);
                }
            });
        });
    }

    function columnSelectorClean(text) {
        return (text || "").replace(/\s+/g, " ").trim();
    }

    function columnSelectorList(text) {
        return (text || "").split(",").map(function (item) {
            return columnSelectorClean(item);
        }).filter(Boolean);
    }

    function initColumnSelector(config) {
        var host = config.host;
        if (!host || host.getAttribute("data-column-selector-ready") === "true") return null;
        var tables = config.tables || [];
        if (!tables.length && config.targetSelector) {
            tables = slice(document.querySelectorAll(config.targetSelector));
        }
        if (!tables.length && config.content) {
            tables = slice(config.content.querySelectorAll(config.tableSelector || "table"));
        }
        tables = tables.filter(function (table) { return table && table.rows && table.rows.length; });
        if (!tables.length) return null;

        var explicitLabels = columnSelectorList(config.columns || "");
        var explicitByKey = {};
        explicitLabels.forEach(function (label) {
            explicitByKey[label.toLowerCase()] = label;
        });
        var columns = {};
        var order = [];

        tables.forEach(function (table) {
            var header = table.tHead && table.tHead.rows.length ? table.tHead.rows[0] : table.rows[0];
            slice(header.cells).forEach(function (cell, index) {
                var label = config.labelResolver
                    ? config.labelResolver(cell.textContent, cell, table, index)
                    : columnSelectorClean(cell.textContent);
                if (!label) return;
                if (explicitLabels.length) {
                    label = explicitByKey[columnSelectorClean(label).toLowerCase()] || "";
                    if (!label) return;
                }
                if (!columns[label]) {
                    columns[label] = [];
                    order.push(label);
                }
                columns[label].push({ table: table, index: index });
            });
        });

        if (order.length < (config.minColumns || 1)) return null;

        var defaultLabels = columnSelectorList(config.defaultVisible || "");
        var defaultKeys = {};
        defaultLabels.forEach(function (label) {
            defaultKeys[label.toLowerCase()] = true;
        });
        var state = {};
        order.forEach(function (label) {
            state[label] = !defaultLabels.length || !!defaultKeys[label.toLowerCase()];
        });
        if (!order.some(function (label) { return state[label]; })) state[order[0]] = true;

        var fieldset = document.createElement("fieldset");
        fieldset.className = config.fieldsetClass || "column-selector-fieldset";
        fieldset.setAttribute("data-column-selector-fieldset", "");
        fieldset.innerHTML = "<legend>" + (config.legend || "Show columns") + "</legend>";
        var controls = {};
        var mode = config.mode === "buttons" ? "buttons" : "checks";

        function visibleLabels() {
            return order.filter(function (label) { return state[label]; });
        }

        function setColumn(entry, visible) {
            slice(entry.table.rows).forEach(function (row) {
                var cell = row.cells[entry.index];
                if (!cell) return;
                cell.hidden = !visible;
                cell.classList.toggle("column-selector-hidden", !visible);
                cell.classList.toggle("compare-column-hidden", !visible);
            });
        }

        function syncControls() {
            order.forEach(function (label) {
                var control = controls[label];
                if (!control) return;
                if (control.tagName === "INPUT") {
                    control.checked = state[label];
                } else {
                    control.setAttribute("aria-pressed", state[label] ? "true" : "false");
                }
            });
        }

        function applyColumns() {
            var visible = visibleLabels();
            order.forEach(function (label) {
                columns[label].forEach(function (entry) {
                    setColumn(entry, state[label]);
                });
            });
            tables.forEach(function (table) {
                for (var rowIndex = 0; rowIndex < table.rows.length; rowIndex += 1) {
                    var row = table.rows[rowIndex];
                    var firstVisible = null;
                    var lastVisible = null;
                    for (var cellIndex = 0; cellIndex < row.cells.length; cellIndex += 1) {
                        var cell = row.cells[cellIndex];
                        cell.classList.remove("column-selector-first-visible", "column-selector-last-visible");
                        if (cell.hidden) continue;
                        if (!firstVisible) firstVisible = cell;
                        lastVisible = cell;
                    }
                    if (firstVisible) firstVisible.classList.add("column-selector-first-visible");
                    if (lastVisible) lastVisible.classList.add("column-selector-last-visible");
                }
                table.classList.add("column-selector-active");
                table.style.setProperty("--visible-column-count", String(visible.length));
                table.setAttribute("data-visible-column-count", String(visible.length));
            });
            if (config.summaryStatus) {
                config.summaryStatus.textContent = visible.length + "/" + order.length + " visible";
            }
            syncControls();
            if (config.updateStatus && config.status) {
                var kind = config.kind || "columns";
                config.status.textContent = "Showing " + visible.length + " of " + order.length + " " + kind + ": " + visible.join(", ") + ".";
            }
        }

        function setOne(label, visible) {
            state[label] = visible;
            if (!order.some(function (item) { return state[item]; })) state[label] = true;
            applyColumns();
        }

        order.forEach(function (label) {
            if (mode === "buttons") {
                var button = document.createElement("button");
                button.type = "button";
                button.className = "column-selector-button";
                button.textContent = label;
                button.setAttribute("aria-pressed", state[label] ? "true" : "false");
                button.addEventListener("click", function () {
                    setOne(label, !state[label]);
                });
                controls[label] = button;
                fieldset.appendChild(button);
                return;
            }
            var wrapper = document.createElement("label");
            wrapper.className = config.toggleClass || "column-selector-toggle";
            var input = document.createElement("input");
            input.type = "checkbox";
            input.value = label;
            input.checked = state[label];
            input.addEventListener("change", function () {
                setOne(label, input.checked);
            });
            controls[label] = input;
            wrapper.appendChild(input);
            wrapper.appendChild(document.createTextNode(label));
            fieldset.appendChild(wrapper);
        });

        if (config.actions) {
            var actions = document.createElement("div");
            actions.className = "column-selector-actions";
            var allButton = document.createElement("button");
            allButton.type = "button";
            allButton.textContent = "All";
            allButton.addEventListener("click", function () {
                order.forEach(function (label) { state[label] = true; });
                applyColumns();
            });
            var defaultButton = document.createElement("button");
            defaultButton.type = "button";
            defaultButton.textContent = "Default";
            defaultButton.addEventListener("click", function () {
                order.forEach(function (label) {
                    state[label] = !defaultLabels.length || !!defaultKeys[label.toLowerCase()];
                });
                if (!order.some(function (label) { return state[label]; })) state[order[0]] = true;
                applyColumns();
            });
            actions.appendChild(allButton);
            actions.appendChild(defaultButton);
            fieldset.appendChild(actions);
        }

        host.addEventListener("ze:show-all-columns", function () {
            order.forEach(function (label) { state[label] = true; });
            applyColumns();
        });
        host.insertBefore(fieldset, config.insertBefore || null);
        host.setAttribute("data-column-selector-ready", "true");
        applyColumns();
        return { fieldset: fieldset, columns: columns, order: order, apply: applyColumns };
    }

    function initGenericColumnSelectors() {
        slice(document.querySelectorAll("[data-column-selector]")).forEach(function (root) {
            var status = root.querySelector("[data-column-selector-status]");
            if (!status) {
                status = document.createElement("p");
                status.className = "column-selector-status";
                status.setAttribute("data-column-selector-status", "");
                status.setAttribute("aria-live", "polite");
                root.appendChild(status);
            }
            initColumnSelector({
                host: root,
                targetSelector: root.getAttribute("data-column-selector-target"),
                columns: root.getAttribute("data-column-selector-columns"),
                defaultVisible: root.getAttribute("data-column-selector-default"),
                mode: root.getAttribute("data-column-selector-mode"),
                legend: root.getAttribute("data-column-selector-label"),
                kind: root.getAttribute("data-column-selector-kind") || "columns",
                actions: root.getAttribute("data-column-selector-actions") === "true",
                status: status,
                insertBefore: status,
                updateStatus: true,
            });
        });
    }

    function tableColumnControlsDisabled(table) {
        if (
            table.getAttribute("data-column-controls") === "off"
            || table.classList.contains("no-column-controls")
            || table.closest('[data-table-columns="off"]')
        ) return true;

        var marker = table.previousSibling;
        while (marker && marker.nodeType === 3 && !columnSelectorClean(marker.textContent)) {
            marker = marker.previousSibling;
        }
        if (!marker || marker.nodeType !== 8) return false;
        return /^(?:table-columns|column-controls):\s*(?:off|false)$/i.test(
            columnSelectorClean(marker.nodeValue)
        );
    }

    function initAutomaticTableColumnSelectors(root, includeComparison) {
        var tables = [];
        if (root && root.matches && root.matches("table")) tables.push(root);
        if (root && root.querySelectorAll) {
            tables = tables.concat(slice(root.querySelectorAll("table")));
        }
        tables.forEach(function (table) {
            if (
                table.classList.contains("column-selector-active")
                || table.getAttribute("data-auto-column-selector") === "true"
                || tableColumnControlsDisabled(table)
            ) return;

            var content = table.closest(".md-content");
            if (!includeComparison && content && content.querySelector("[data-compare-filter]")) return;

            var header = table.tHead && table.tHead.rows.length
                ? table.tHead.rows[0]
                : table.rows[0];
            if (!header || header.cells.length < 2) return;

            var host = document.createElement("details");
            host.className = "column-selector table-column-selector";
            host.setAttribute("data-auto-column-selector", "true");
            var summary = document.createElement("summary");
            summary.className = "table-column-selector-summary";
            var summaryLabel = document.createElement("span");
            summaryLabel.textContent = "Columns";
            var summaryStatus = document.createElement("span");
            summaryStatus.className = "table-column-selector-summary-status";
            summary.appendChild(summaryLabel);
            summary.appendChild(summaryStatus);
            host.appendChild(summary);
            var status = document.createElement("p");
            status.className = "column-selector-status sr-only";
            status.setAttribute("aria-live", "polite");
            host.appendChild(status);
            table.parentNode.insertBefore(host, table);

            var selector = initColumnSelector({
                host: host,
                tables: [table],
                legend: "Show columns",
                mode: "checks",
                kind: "columns",
                actions: true,
                status: status,
                summaryStatus: summaryStatus,
                insertBefore: status,
                minColumns: 2,
                updateStatus: true,
            });
            if (!selector) {
                host.parentNode.removeChild(host);
                return;
            }
            var legend = selector.fieldset.querySelector("legend");
            if (legend) legend.classList.add("sr-only");
            table.setAttribute("data-auto-column-selector", "true");
        });
    }

    function observeAutomaticTableColumnSelectors() {
        if (!window.MutationObserver || !document.body) return;
        var observer = new MutationObserver(function (mutations) {
            mutations.forEach(function (mutation) {
                slice(mutation.addedNodes).forEach(function (node) {
                    if (node.nodeType === 1) initAutomaticTableColumnSelectors(node);
                });
            });
        });
        observer.observe(document.body, { childList: true, subtree: true });
    }

    function initComparisonFilters() {
        var tools = slice(document.querySelectorAll("[data-compare-filter]"));
        if (!tools.length) return;
        loadFrontendVocab().then(function (vocab) {
            var productMatchers = (vocab.productMatchers || []).map(function (entry) {
                return { label: entry.label, pattern: new RegExp(entry.pattern) };
            });

            function productLabel(text) {
                var normalized = text.replace(/\s+/g, " ").trim().toLowerCase();
                for (var i = 0; i < productMatchers.length; i++) {
                    if (productMatchers[i].pattern.test(normalized)) return productMatchers[i].label;
                }
                return "";
            }

            function initColumnControls(tool, content, status) {
                return initColumnSelector({
                    host: tool,
                    content: content,
                    tableSelector: "table",
                    labelResolver: productLabel,
                    legend: "Show products",
                    mode: "checks",
                    fieldsetClass: "compare-columns column-selector-fieldset column-selector-checks",
                    toggleClass: "compare-column-toggle column-selector-toggle",
                    insertBefore: status,
                    minColumns: 3,
                    updateStatus: false,
                });
            }

            tools.forEach(function (tool) {
                var input = tool.querySelector("[data-compare-search]");
                var select = tool.querySelector("[data-compare-section]");
                var status = tool.querySelector("[data-compare-status]");
                var content = tool.closest(".doc-body") || tool.closest(".md-content");
                if (!input || !select || !status || !content) return;
                if (!initColumnControls(tool, content, status)) {
                    initAutomaticTableColumnSelectors(content, true);
                }

                var headings = slice(content.querySelectorAll("h2"));
                var groups = headings.map(function (heading, index) {
                    var nodes = [];
                    var node = heading.nextElementSibling;
                    while (node && node.tagName !== "H2") {
                        if (node !== tool) nodes.push(node);
                        node = node.nextElementSibling;
                    }
                    var rows = [];
                    nodes.forEach(function (groupNode) {
                        slice(groupNode.querySelectorAll("tbody tr")).forEach(function (row) {
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
        });
    }

    function initFAQDeepLinks() {
        if (!document.querySelector(".faq-groups")) return;

        function revealQuestion() {
            if (!location.hash) return;
            var id;
            try {
                id = decodeURIComponent(location.hash.slice(1));
            } catch (err) {
                return;
            }
            var target = document.getElementById(id);
            if (!target || !target.matches("details.faq-card")) return;
            target.open = true;
            window.requestAnimationFrame(function () {
                target.scrollIntoView({ block: "start" });
            });
        }

        revealQuestion();
        window.addEventListener("hashchange", revealQuestion);
    }

    function copyText(text) {
        if (navigator.clipboard && navigator.clipboard.writeText) {
            return navigator.clipboard.writeText(text);
        }
        return new Promise(function (resolve, reject) {
            var textarea = document.createElement("textarea");
            textarea.value = text;
            textarea.setAttribute("readonly", "");
            textarea.style.position = "fixed";
            textarea.style.left = "-9999px";
            document.body.appendChild(textarea);
            textarea.select();
            try {
                document.execCommand("copy");
                document.body.removeChild(textarea);
                resolve();
            } catch (err) {
                document.body.removeChild(textarea);
                reject(err);
            }
        });
    }

    function initCodeCopyButtons() {
        slice(document.querySelectorAll("pre > code")).forEach(function (code) {
            var pre = code.parentNode;
            if (!pre || pre.querySelector(".code-copy-button")) return;
            if (pre.closest && pre.closest('[data-code-copy="off"]')) return;
            var button = document.createElement("button");
            button.type = "button";
            button.className = "code-copy-button";
            button.textContent = "Copy";
            button.setAttribute("aria-label", "Copy code block");
            button.addEventListener("click", function () {
                copyText(code.textContent).then(function () {
                    button.textContent = "Copied";
                    window.setTimeout(function () { button.textContent = "Copy"; }, 1600);
                }).catch(function () {
                    button.textContent = "Copy failed";
                    window.setTimeout(function () { button.textContent = "Copy"; }, 1600);
                });
            });
            pre.classList.add("code-copy-wrap");
            pre.appendChild(button);
        });
    }

    // The header arrives after the body's figures were filled, so its own
    // marked figures are filled here, from the same snapshot fetch.
    loadSharedHeader().then(function () {
        fillRepoStats(document.querySelector("header.site-header"));
        initThemeToggle();
        initNavigation();
        initSearchOverlay();
    });
    // Terminal demos are asciicasts replayed by the self-hosted player
    // (assets/vendor/asciinema-player.min.js), which the page loads only when
    // it carries a demo. The mount already has the box the recording needs, so
    // nothing here changes the page's height.
    //
    // terminalLineHeight matches the ratio the mount reserved its box with and
    // the grid derived by internal/le/terminaldemo/pty.go for the recording.
    // terminalFontFamily is passed here rather than set in CSS because the
    // player MEASURES the font to size a character cell, and a stack it renders
    // with but did not measure gives the wrong cell.
    // castVersion appends the recording's own digest to its URL, so a browser
    // that cached a previous render fetches the new bytes. The digest is read
    // at RUNTIME from the demo manifest rather than written into data-cast-src,
    // the same reason data-ze-stat reads site-facts.json: a hash baked into the
    // markup rewrites every page carrying a demo at every release, and the
    // recordings change far more often than the pages around them do.
    function castVersion(manifest, src) {
        if (!manifest || !manifest.demos) {
            return src;
        }
        var name = src.split("/").pop();
        var id = name.replace(/\.cast$/, "");
        var entry = manifest.demos[id];
        var cast = entry && entry.assets && entry.assets.cast;
        if (!cast || !cast.sha256) {
            return src;
        }
        return src + (src.indexOf("?") === -1 ? "?" : "&") + "v=" + cast.sha256.slice(0, 12);
    }

    function initTerminalDemoPlayers() {
        var mounts = slice(document.querySelectorAll("[data-terminal-demo-player]"));
        if (!mounts.length || typeof AsciinemaPlayer === "undefined") {
            return;
        }

        function mountAll(manifest) {
            mounts.forEach(function (mount) {
                var src = mount.getAttribute("data-cast-src");
                if (!src) {
                    return;
                }
                var chapter = mount.closest("[data-cast-start]");
                var preview = mount.closest("[data-cast-poster]");
                var poster = chapter ? chapter.getAttribute("data-cast-start") :
                    preview ? preview.getAttribute("data-cast-poster") : undefined;
                var player = AsciinemaPlayer.create(castVersion(manifest, src), mount, {
                    startAt: chapter ? chapter.getAttribute("data-cast-start") : undefined,
                    poster: poster !== undefined ? "npt:" + poster : undefined,
                    autoPlay: false,
                    fit: "both",
                    controls: true,
                    theme: "ze",
                    terminalLineHeight: 1.32,
                    terminalFontFamily:
                        '"JetBrains Mono", "SFMono-Regular", Consolas, monospace',
                });
                if (preview && !chapter) {
                    mount.setAttribute("data-cast-preview-active", "");
                    var clearPreview = function () {
                        mount.removeAttribute("data-cast-preview-active");
                    };
                    player.addEventListener("play", clearPreview);
                    player.addEventListener("seeked", clearPreview);
                    player.addEventListener("ready", function () {
                        // Initial poster output precedes ready; later output can
                        // also come from frame stepping without a seeked event.
                        player.addEventListener("output", clearPreview);
                    });
                }
            });
        }

        // The manifest is the one file that must never come from the cache:
        // it is what says which bytes are current. It is a few KB, and a
        // failed or unsupported fetch falls back to the unversioned URL, so a
        // demo still plays when this lookup does not answer.
        if (!window.fetch) {
            mountAll(null);
            return;
        }
        fetch(siteRootFromScript() + "assets/demos/manifest.json", { cache: "no-store" })
            .then(function (response) {
                return response.ok ? response.json() : null;
            })
            .catch(function () {
                return null;
            })
            .then(mountAll);
    }

    function initBlogCards() {
        slice(document.querySelectorAll("article.blog-card")).forEach(function (card) {
            var titleLink = card.querySelector("h3 a[href]");
            if (!titleLink) return;

            var title = cleanLabel(titleLink.textContent);
            var overlay = document.createElement("a");
            overlay.className = "blog-card-overlay";
            overlay.setAttribute("href", titleLink.getAttribute("href"));
            overlay.setAttribute("aria-label", title);
            titleLink.replaceWith(document.createTextNode(title));
            card.appendChild(overlay);
        });
    }

    function initProductShowcases() {
        slice(document.querySelectorAll("[data-product-showcase]")).forEach(function (showcase) {
            var buttons = slice(showcase.querySelectorAll("[data-product-target]"));
            var panels = slice(showcase.querySelectorAll("[data-product-panel]"));
            if (!buttons.length || !panels.length) return;
            function select(button) {
                var target = button.getAttribute("data-product-target");
                if (!panels.some(function (panel) { return panel.id === target; })) return;
                panels.forEach(function (panel) {
                    var selected = panel.id === target;
                    panel.setAttribute("aria-hidden", selected ? "false" : "true");
                    panel.inert = !selected;
                });
                buttons.forEach(function (item) { item.setAttribute("aria-pressed", item === button ? "true" : "false"); });
            }
            buttons.forEach(function (button) {
                button.addEventListener("click", function () { select(button); });
            });
            showcase.setAttribute("data-enhanced", "true");
            select(buttons.filter(function (button) { return button.getAttribute("aria-pressed") === "true"; })[0] || buttons[0]);
            var controls = showcase.querySelector(".screen-tabs");
            if (controls) controls.hidden = false;
        });
    }

    function initReadingOutlines() {
        if (!window.matchMedia || !window.matchMedia("(max-width: 850px)").matches) return;
        slice(document.querySelectorAll("details.reading-outline, .cli-group-nav > details")).forEach(function (outline) {
            if (!outline.contains(document.activeElement)) outline.open = false;
        });
    }

    function initPageOutlines() {
        var links = slice(document.querySelectorAll('.doc-toc a[href^="#"], .blog-article-toc a[href^="#"], .cli-group-nav a[href^="#"]'));
        if (!links.length || !("IntersectionObserver" in window)) return;
        var targets = links.map(function (link) {
            var id;
            try { id = decodeURIComponent(link.getAttribute("href").slice(1)); } catch (err) { return null; }
            return { link: link, target: document.getElementById(id) };
        }).filter(function (entry) { return entry && entry.target; });
        var observer = new IntersectionObserver(function (entries) {
            entries.forEach(function (entry) {
                if (!entry.isIntersecting) return;
                targets.forEach(function (item) {
                    if (item.target === entry.target) item.link.setAttribute("aria-current", "location");
                    else item.link.removeAttribute("aria-current");
                });
            });
        }, { rootMargin: "-12% 0px -65% 0px", threshold: 0 });
        targets.forEach(function (entry) { observer.observe(entry.target); });
    }

    initProductShowcases();
    initReadingOutlines();
    initPageOutlines();
    initBlogCards();
    initTaglineCarousel();
    initSearchPage();
    initFeatureFilters();
    initTimelineFilters();
    initChangesFilters();
    initDependencyFilter();
    initCliCatalogFilter();
    initCommandEquivalentFilter();
    initFeatureTooltips();
    initGenericColumnSelectors();
    initComparisonFilters();
    initAutomaticTableColumnSelectors(document);
    observeAutomaticTableColumnSelectors();
    initTerminalDemoPlayers();
    initSourceLinks();
    initCodeCopyButtons();
    initFAQDeepLinks();
    fillRepoStats(document);
});
