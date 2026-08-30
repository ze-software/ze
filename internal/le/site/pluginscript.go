// Design: website/AI.md -- the catalog's browser-side search
// Detail: plugins.go renders the markup this script acts on.
package site

// pluginCatalogScript narrows the catalog in the browser: it filters the cards
// by the search box and by the two selects, hides an area whose cards all
// filtered out, keeps the query in the URL so a filtered view can be shared,
// and states the count in a live region a screen reader announces.
//
// It is a raw string because it holds quotation marks of both kinds and no
// backtick, and because a reader comparing it against the page it ships to
// should meet the same characters in both.
const pluginCatalogScript = `        <script>
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
                    return value.toLowerCase().split(/\s+/).filter(Boolean);
                }

                function optionForValue(select, value) {
                    for (var i = 0; i < select.options.length; i += 1) {
                        if (select.options[i].value === value) return select.options[i];
                    }
                    return null;
                }

                function selectedLabel(select) {
                    var option = select.options[select.selectedIndex];
                    return option ? (option.dataset.label || option.textContent).replace(/\s+\\(\d+\\)$/, "") : "";
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
`
