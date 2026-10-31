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
 * counts rfcs, walked and not-walked, or register, the set's mandatory
 * register in words. The text inside a marked element is the last build's
 * value, shown until the files are read.
 *
 * A set sums only walked RFCs and counts the rest apart, and a set with no
 * member shows "no rows". A file the index names that cannot be read is named
 * on the page with the command that publishes it, never dropped from a sum.
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

    var publish = "Run ./le site build to publish data/rfc/.";

    // unreadable is the error for files that could not be read: which files,
    // what the page cannot show without them, and the command that writes them.
    function unreadable(paths, consequence) {
        return new Error(paths.join(", ") + " could not be read, so " + consequence + ". " + publish);
    }

    // loadFile answers the parsed file at a site-relative path, fetched once
    // per page load, or rejects with the path when it cannot be read: the
    // caller words the error, because only it knows what the file was for.
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
        if (!indexPromise) {
            indexPromise = loadFile("data/rfc/index.json").catch(function (error) {
                throw unreadable([error.message], "no RFC figure on this page can be shown");
            });
        }
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
            return Promise.reject(new Error(subject + " is not a subject the RFC data answers, so nothing can be " +
                "shown here. Fix the marker: write profile/<name>/required or profile/<name>/optional."));
        }
        var entry = (index.profiles || []).filter(function (profile) {
            return profile.name === parts[1];
        })[0];
        if (!entry) {
            return Promise.reject(new Error("the index lists no profile named " + parts[1] + ", so it has no " +
                "figure. Fix the marker, or add rfc/profiles/" + parts[1] + ".md. " + publish));
        }
        return loadFile(entry.file).then(function (profile) {
            return profile[parts[2]] || [];
        }, function (error) {
            throw unreadable([error.message], "the members of profile " + parts[1] + " are unknown");
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
            if (unknown.length) {
                throw new Error("the index lists no RFC named " + unknown.join(", ") + ", so it has no figure. " +
                    "Fix the marker, or add its rfc/short/ summary. " + publish);
            }
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
            if (missing.length) {
                throw unreadable(missing, "this figure would leave " + (missing.length === 1 ? "that RFC" : "those RFCs") + " out");
            }
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
            sum.register = registerOf(sum);
            return sum;
        });
    }

    // registerOf answers a set's mandatory register in words: complete only
    // when the set holds at least one RFC, none of them is unwalked, and
    // neither the Ze correctness gap nor the OS testing gap holds a row. The
    // site build writes the same words as fallback (rfcRegisterOf,
    // internal/le/site/rfcfallback.go).
    function registerOf(sum) {
        if (sum.rfcs === 0 || sum["not-walked"] !== 0 || !sum.must) return "not complete";
        return sum.must["ze-gap"] === 0 && sum.must["os-gap"] === 0 ? "complete" : "not complete";
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

    // byKind matches a per-kind count of the rows not owed: a kind no row was
    // excluded by is absent from every file, and its count is 0.
    var byKind = /^(must|should)\.not-owed-by-kind\.[^.]+$/;

    function fieldsSum(sum, fields) {
        var total = 0;
        for (var i = 0; i < fields.length; i++) {
            var value = lookup(sum, fields[i]);
            if (typeof value !== "number") return NaN;
            total += value;
        }
        return total;
    }

    function badMarker(node, attribute, shape) {
        return attribute + "=\"" + node.getAttribute(attribute) + "\" is not a marker the RFC data answers, so nothing can be shown here. " +
            "Write it as " + shape + ".";
    }

    function showError(node, message) {
        node.textContent = message;
        node.classList.add("rfc-figure-error");
    }

    function fillValue(node) {
        var marker = splitMarker(node.getAttribute("data-ze-rfc"));
        if (!marker) return showError(node, badMarker(node, "data-ze-rfc", "<subject>:<field>"));
        sumSubject(marker[0]).then(function (sum) {
            var value = lookup(sum, marker[1]);
            if (typeof value === "number") {
                node.textContent = value.toLocaleString("en-GB");
                return;
            }
            if (typeof value === "string") {
                node.textContent = value;
                return;
            }
            if (sum.rfcs === 0) {
                node.textContent = "no rows";
                return;
            }
            if (sum.walked === 0) {
                node.textContent = "not walked";
                return;
            }
            if (value === undefined && byKind.test(marker[1])) {
                node.textContent = "0";
                return;
            }
            showError(node, marker[1] + " is not a field of the RFC data, so nothing can be shown here. " +
                "Fix the data-ze-rfc marker: docs/architecture/site-facts.md lists the fields.");
        }, function (error) {
            showError(node, error.message);
        });
    }

    // drawBar replaces the element's content with the bar and its legend. The
    // bar's segments run left to right in the order the index declares, each
    // carrying its words and count as text and a width that is its count over
    // the bar's total. A segment too narrow for its words cuts them, so the
    // legend beside the bar names every non-zero segment again, from the same
    // layout and counts, with its colour as a swatch.
    function drawBar(node, layout, sum, bar) {
        var segments = layout[bar];
        var totalFields = layout[bar + "-total"];
        if (!segments || !totalFields) {
            return showError(node, bar + " is not a bar the RFC data declares, so nothing can be drawn here. " +
                "Write the marker as <subject>:must or <subject>:overview.");
        }
        if (sum.rfcs === 0) {
            node.textContent = "no rows";
            return;
        }
        if (sum.walked === 0) {
            node.textContent = "not walked";
            return;
        }
        var total = fieldsSum(sum, totalFields);
        var counts = segments.map(function (segment) { return fieldsSum(sum, segment.fields); });
        var drawn = counts.reduce(function (a, b) { return a + b; }, 0);
        if (!isFinite(total) || drawn !== total) {
            return showError(node, "the " + bar + " segments sum to " + drawn + ", not the " + total +
                " rows the files state, so the bar would misdraw them. " + publish);
        }
        while (node.firstChild) node.removeChild(node.firstChild);
        node.classList.add("rfc-seg-figure");
        if (total === 0) {
            node.textContent = "no rows";
            return;
        }
        var track = document.createElement("div");
        track.className = "rfc-seg-bar";
        var legend = document.createElement("ul");
        legend.className = "rfc-seg-legend";
        segments.forEach(function (segment, index) {
            if (!counts[index]) return;
            var words = segment.label + " " + counts[index].toLocaleString("en-GB");
            var span = document.createElement("span");
            span.className = "rfc-seg rfc-seg-" + segment.segment;
            span.setAttribute("data-segment", segment.segment);
            span.style.width = (100 * counts[index] / total) + "%";
            span.textContent = words;
            span.title = words;
            track.appendChild(span);
            var item = document.createElement("li");
            item.setAttribute("data-segment", segment.segment);
            var swatch = document.createElement("span");
            swatch.className = "rfc-seg-swatch rfc-seg-" + segment.segment;
            swatch.setAttribute("aria-hidden", "true");
            item.appendChild(swatch);
            item.appendChild(document.createTextNode(words));
            legend.appendChild(item);
        });
        node.appendChild(track);
        node.appendChild(legend);
    }

    function fillBar(node) {
        var marker = splitMarker(node.getAttribute("data-ze-rfc-bar"));
        if (!marker) return showError(node, badMarker(node, "data-ze-rfc-bar", "<subject>:must or <subject>:overview"));
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
