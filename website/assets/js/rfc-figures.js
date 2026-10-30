/*
 * rfc-figures.js fills the RFC completion figures of a page from the files
 * the site build publishes under data/rfc/ (internal/le/site/rfcdata.go):
 * index.json, one <stem>.json per RFC and one profiles/<name>.json per
 * profile. The page computes nothing else: every count is a file's, and a set
 * of RFCs is summed here, count by count, before anything is divided.
 *
 * Markup it reads (docs/architecture/site-facts.md, "RFC completion data"):
 *
 *   data-ze-rfc="<subject>:<field>"     the text becomes the subject's count
 *   data-ze-rfc-bar="<subject>:<bar>"   the element becomes the bar, <bar>
 *                                       being "must" or "overview"
 *
 * A subject is an RFC stem (rfc4271), or a profile tier
 * (profile/<name>/required, profile/<name>/optional). A field is a dotted path
 * of a per-RFC file (must.ze-proven, should.not-decided), or one of the set
 * counts rfcs, walked and not-walked. The text inside a marked element is the
 * last build's value, shown until the files are read.
 *
 * A set sums only walked RFCs and counts the rest apart. A file the index
 * names that cannot be read is named on the page, never dropped from a sum.
 */
(function () {
    "use strict";

    var scriptMarker = "assets/rfc-figures.js";
    var root = null;
    var indexPromise = null;
    var filePromises = {};

    function siteRoot() {
        if (root !== null) return root;
        var script = document.querySelector('script[src*="' + scriptMarker + '"]');
        var src = script ? (script.getAttribute("src") || "") : "";
        var clean = src.split("#")[0].split("?")[0];
        var at = clean.indexOf(scriptMarker);
        root = at === -1 ? "" : clean.slice(0, at);
        return root;
    }

    // loadFile answers the parsed file at a site-relative path, fetched once
    // per page load, or rejects with the path when it cannot be read.
    function loadFile(path) {
        if (!filePromises[path]) {
            filePromises[path] = fetch(siteRoot() + path).then(function (response) {
                if (!response.ok) throw new Error(path);
                return response.json();
            }).catch(function () {
                throw new Error(path);
            });
        }
        return filePromises[path];
    }

    function loadIndex() {
        if (!indexPromise) indexPromise = loadFile("data/rfc/index.json");
        return indexPromise;
    }

    // splitMarker answers [subject, rest] at the last colon: a stem never
    // holds one, and the field or bar name after it never does either.
    function splitMarker(value) {
        var at = String(value || "").lastIndexOf(":");
        if (at <= 0) return null;
        return [value.slice(0, at), value.slice(at + 1)];
    }

    // membersOf answers the stems a subject names, read from the index and,
    // for a profile tier, from the profile's own file.
    function membersOf(index, subject) {
        var parts = subject.split("/");
        if (parts[0] !== "profile") {
            return Promise.resolve([subject]);
        }
        if (parts.length !== 3 || (parts[2] !== "required" && parts[2] !== "optional")) {
            return Promise.reject(new Error("unknown subject " + subject));
        }
        var entry = (index.profiles || []).filter(function (profile) {
            return profile.name === parts[1];
        })[0];
        if (!entry) return Promise.reject(new Error("unknown profile " + parts[1]));
        return loadFile(entry.file).then(function (profile) {
            return profile[parts[2]] || [];
        }, function (error) {
            throw new Error("missing " + error.message);
        });
    }

    // addCounts adds every count of source into target: numbers, and the
    // per-kind maps one level down.
    function addCounts(target, source) {
        Object.keys(source).forEach(function (key) {
            var value = source[key];
            if (typeof value === "number") {
                target[key] = (target[key] || 0) + value;
                return;
            }
            if (value && typeof value === "object" && !Array.isArray(value)) {
                target[key] = target[key] || {};
                addCounts(target[key], value);
            }
        });
    }

    // sumStems answers the summed counts of the stems: rfcs, walked and
    // not-walked, then must and should over the walked ones (null when none
    // is walked). It rejects, naming every file, when any cannot be read.
    function sumStems(stems) {
        return loadIndex().then(function (index) {
            var files = {};
            (index.rfcs || []).forEach(function (entry) {
                files[entry.stem] = entry.file;
            });
            var unknown = stems.filter(function (stem) { return !files[stem]; });
            if (unknown.length) throw new Error("no RFC data for " + unknown.join(", "));
            return Promise.all(stems.map(function (stem) {
                return loadFile(files[stem]).then(function (data) {
                    return { data: data };
                }, function () {
                    return { missing: files[stem] };
                });
            }));
        }).then(function (loaded) {
            var missing = loaded.filter(function (item) { return item.missing; })
                .map(function (item) { return item.missing; });
            if (missing.length) throw new Error("missing " + missing.join(", "));
            var sum = { rfcs: loaded.length, walked: 0, "not-walked": 0, must: null, should: null };
            loaded.forEach(function (item) {
                if (!item.data.walked) {
                    sum["not-walked"] += 1;
                    return;
                }
                sum.walked += 1;
                sum.must = sum.must || {};
                sum.should = sum.should || {};
                addCounts(sum.must, item.data.must || {});
                addCounts(sum.should, item.data.should || {});
            });
            return sum;
        });
    }

    function sumSubject(subject) {
        return loadIndex().then(function (index) {
            return membersOf(index, subject);
        }).then(sumStems);
    }

    function lookup(sum, field) {
        var value = sum;
        var parts = field.split(".");
        for (var i = 0; i < parts.length; i++) {
            if (value === null || typeof value !== "object") return undefined;
            value = value[parts[i]];
        }
        return value;
    }

    function fieldsSum(sum, fields) {
        var total = 0;
        for (var i = 0; i < fields.length; i++) {
            var value = lookup(sum, fields[i]);
            if (typeof value !== "number") return NaN;
            total += value;
        }
        return total;
    }

    function showError(node, message) {
        node.textContent = message;
        node.classList.add("rfc-figure-error");
    }

    function fillValue(node) {
        var marker = splitMarker(node.getAttribute("data-ze-rfc"));
        if (!marker) return showError(node, "bad marker " + node.getAttribute("data-ze-rfc"));
        sumSubject(marker[0]).then(function (sum) {
            var value = lookup(sum, marker[1]);
            if (typeof value === "number") {
                node.textContent = value.toLocaleString("en-GB");
                return;
            }
            if (sum.walked === 0) {
                node.textContent = "not walked";
                return;
            }
            showError(node, "no field " + marker[1]);
        }, function (error) {
            showError(node, error.message);
        });
    }

    // drawBar replaces the element's content with the bar's segments, left to
    // right in the order the index declares, each carrying its words and count
    // as text and a width that is its count over the bar's total.
    function drawBar(node, layout, sum, bar) {
        var segments = layout[bar];
        var totalFields = layout[bar + "-total"];
        if (!segments || !totalFields) return showError(node, "unknown bar " + bar);
        if (sum.walked === 0) {
            node.textContent = "not walked";
            return;
        }
        var total = fieldsSum(sum, totalFields);
        var counts = segments.map(function (segment) { return fieldsSum(sum, segment.fields); });
        var drawn = counts.reduce(function (a, b) { return a + b; }, 0);
        if (!isFinite(total) || drawn !== total) {
            return showError(node, "the " + bar + " segments sum to " + drawn + ", not the " + total + " rows the files state");
        }
        while (node.firstChild) node.removeChild(node.firstChild);
        node.classList.add("rfc-seg-bar");
        if (total === 0) {
            node.textContent = "no rows";
            return;
        }
        segments.forEach(function (segment, index) {
            if (!counts[index]) return;
            var span = document.createElement("span");
            span.className = "rfc-seg rfc-seg-" + segment.segment;
            span.setAttribute("data-segment", segment.segment);
            span.style.width = (100 * counts[index] / total) + "%";
            span.textContent = segment.label + " " + counts[index].toLocaleString("en-GB");
            span.title = span.textContent;
            node.appendChild(span);
        });
    }

    function fillBar(node) {
        var marker = splitMarker(node.getAttribute("data-ze-rfc-bar"));
        if (!marker) return showError(node, "bad marker " + node.getAttribute("data-ze-rfc-bar"));
        Promise.all([loadIndex(), sumSubject(marker[0])]).then(function (loaded) {
            drawBar(node, loaded[0].bars || {}, loaded[1], marker[1]);
        }, function (error) {
            showError(node, error.message);
        });
    }

    function fill(scope) {
        if (!scope || !window.fetch) return;
        Array.prototype.forEach.call(scope.querySelectorAll("[data-ze-rfc]"), fillValue);
        Array.prototype.forEach.call(scope.querySelectorAll("[data-ze-rfc-bar]"), fillBar);
    }

    window.zeRFCFigures = { fill: fill, sumStems: sumStems };

    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", function () { fill(document); });
    } else {
        fill(document);
    }
})();
