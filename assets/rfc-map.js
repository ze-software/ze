/* RFC planning map. Topology and associations come from rfc-map.json;
 * every measurement comes only from the canonical data/rfc/<stem>.json.
 * A view is a reading aid. Selecting a node changes no release obligation.
 */
(function () {
    "use strict";

    const root = document.querySelector("[data-rfc-map]");
    if (!root) return;
    const svgNS = "http://www.w3.org/2000/svg";
    const htmlNS = "http://www.w3.org/1999/xhtml";
    const base = new URL(root.dataset.rfcBase || "../../data/rfc/", document.baseURI);
    const site = new URL("../../", document.baseURI);
    const figures = new Map();
    const figureValues = new Map();
    const relationLabels = {
        "declared-requirement": "Declared requirement relation",
        "declared-rfc": "Declared RFC relation · requirement level unassigned",
        "summary-gap-citation": "Named in a requirement gap",
        "summary-partial-citation": "Named in a partial requirement"
    };
    const edgeLabels = { "builds-on": "Cites", updates: "Updates", obsoletes: "Obsoletes" };
    let model;
    let roadmap;
    let roadmapError = "";
    let nodes;
    let specs;
    let state;
    let ui;

    function el(tag, className, text) {
        const node = document.createElement(tag);
        if (className) node.className = className;
        if (text !== undefined) node.textContent = text;
        return node;
    }
    function svg(tag, attrs) {
        const node = document.createElementNS(svgNS, tag);
        Object.entries(attrs || {}).forEach(([name, value]) => node.setAttribute(name, String(value)));
        return node;
    }
    function link(text, href, className) {
        const node = el("a", className, text);
        node.href = href;
        return node;
    }
    function button(text, action, className) {
        const node = el("button", className || "rfc-map-button", text);
        node.type = "button";
        node.addEventListener("click", action);
        return node;
    }
    function display(stem) {
        return /^rfc\d+$/.test(stem) ? "RFC " + stem.slice(3) : stem;
    }
    function detail(stem, rid) {
        const url = new URL("quality/rfc-compliance/" + encodeURIComponent(stem) + "/", site);
        if (rid) url.hash = rid.toLowerCase();
        return url.href;
    }
    function note(text, className) { return el("p", className || "rfc-map-note", text); }
    function count(value) { return Number.isInteger(value) && value >= 0; }
    function human(value) { return String(value || "Unclassified").replace(/-/g, " "); }
    function unique(values) { return Array.from(new Set(values)); }
    function sorted(values) {
        return values.sort((a, b) => a.localeCompare(b, undefined, { numeric: true }));
    }
    async function json(url) {
        const response = await fetch(url, { credentials: "same-origin" });
        if (!response.ok) throw new Error(new URL(url, document.baseURI).pathname + " returned HTTP " + response.status);
        return response.json();
    }
    function validateMap(data) {
        if (!data || data["schema-version"] !== 1 || !Array.isArray(data.nodes) ||
            !Array.isArray(data.edges) || !Array.isArray(data.layers) || !Array.isArray(data.profiles) ||
            !Array.isArray(data.relations) || !data.requirements || typeof data.requirements !== "object") {
            throw new Error("The RFC map has an unsupported or incomplete data format.");
        }
        const names = new Set();
        data.nodes.forEach(node => {
            if (!node || typeof node.stem !== "string" || names.has(node.stem)) {
                throw new Error("The RFC map contains a missing or repeated document identity.");
            }
            names.add(node.stem);
        });
        return data;
    }
    function ensureFigure(stem) {
        if (!figures.has(stem)) {
            const node = nodes.get(stem);
            const task = !node || !node.summary ? Promise.resolve({ unknown: "No summary" }) :
                json(new URL(encodeURIComponent(stem) + ".json", base)).then(data => {
                    if (data.stem !== stem || typeof data.walked !== "boolean") throw new Error("RFC figure identity is invalid");
                    if (!data.walked) return { unknown: "Not walked", data };
                    const must = data.must;
                    const fields = ["kept", "total", "ze-proven", "os-verified", "proof-pending", "not-built", "not-assessed", "os-gap", "exclusion-unchecked", "not-owed"];
                    if (!must || !fields.every(field => count(must[field]))) throw new Error("Mandatory figure fields are missing");
                    const proven = must["ze-proven"] + must["os-verified"];
                    if (proven > must.kept || must.kept + must["not-owed"] !== must.total) throw new Error("Mandatory figure counts do not add back");
                    return { data, must, proven, percent: must.kept ? (100 * proven / must.kept) : null };
                }).catch(error => ({ error: "Could not read " + stem + ".json: " + error.message }));
            figures.set(stem, task.then(figure => {
                figureValues.set(stem, figure);
                root.querySelectorAll("[data-rfc-proof]").forEach(node => {
                    if (node.dataset.rfcProof === stem) node.textContent = shortProof(figure);
                });
                return figure;
            }));
        }
        return figures.get(stem);
    }
    function shortProof(figure) {
        if (!figure) return "Loading proof…";
        if (figure.error) return "Proof unavailable";
        if (figure.unknown) return "MUST unknown · " + figure.unknown.toLowerCase();
        if (figure.percent === null) return figure.must.total ? "No MUST obligations owed" : "No MUST rows";
        return "MUST proof " + figure.percent.toFixed(1).replace(/\.0$/, "") + "%";
    }
    function setURL() {
        const url = new URL(window.location.href);
        ["layer", "profile", "technology", "view", "rfc", "q"].forEach(key => url.searchParams.delete(key));
        if (state.mode === "all") url.searchParams.set("view", "all");
        if (state.mode !== "all" && state.mode !== "single") url.searchParams.set(state.mode, state.filter);
        url.searchParams.set("rfc", state.selected);
        if (state.query) url.searchParams.set("q", state.query);
        history.replaceState(null, "", url);
    }
    function select(stem) {
        if (!nodes.has(stem)) return;
        const focusGraph = ui.graph.contains(document.activeElement);
        const focusList = ui.listContent.contains(document.activeElement);
        const focusInspector = ui.inspector.contains(document.activeElement);
        state.selected = stem;
        renderGraph();
        renderList();
        renderInspector();
        setURL();
        // Rendering replaces the activated control; keep keyboard navigation
        // in the same area using the newly selected control or inspector.
        if (focusGraph) ui.graph.querySelector('.rfc-map-node[aria-pressed="true"]')?.focus({ preventScroll: true });
        if (focusList) ui.listContent.querySelector('.rfc-map-document[aria-pressed="true"]')?.focus({ preventScroll: true });
        if (focusInspector) ui.inspector.focus({ preventScroll: true });
        ui.live.textContent = "Selected " + display(stem) + ". Its details are beside the graph.";
    }
    function scope() {
        let members;
        if (state.mode === "layer") members = model.layers.find(layer => layer.id === state.filter)?.stems || [];
        else if (state.mode === "profile") members = model.profiles.find(profile => profile.id === state.filter)?.required || [];
        else if (state.mode === "technology") members = model.nodes.filter(node => node.aggregate === state.filter).map(node => node.stem);
        else if (state.mode === "single") members = [state.selected];
        else members = model.nodes.map(node => node.stem);
        return new Set(unique(members.concat(state.added)).filter(stem => nodes.has(stem)));
    }
    function scopeTitle() {
        if (state.mode === "layer") return model.layers.find(layer => layer.id === state.filter)?.title || "Proposed layer";
        if (state.mode === "profile") return model.profiles.find(profile => profile.id === state.filter)?.title || "Existing profile";
        if (state.mode === "technology") return human(state.filter);
        if (state.mode === "single") return display(state.selected);
        return "All RFCs and standards";
    }
    function matches(node) {
        const q = state.query.trim().toLowerCase();
        return !q || [node.stem, display(node.stem), node.title || "", node.aggregate || ""].join(" ").toLowerCase().includes(q);
    }
    function filteredScope() { return sorted(Array.from(scope()).filter(stem => matches(nodes.get(stem)))); }
    function changeView(mode, value) {
        state.mode = mode;
        state.filter = value;
        state.added = [];
        state.graphMode = "view";
        const members = filteredScope();
        if (!members.includes(state.selected) && members.length) state.selected = members[0];
        render();
    }
    function labelledControl(label, control) {
        const field = el("div", "rfc-map-field");
        const text = el("label", "", label);
        text.htmlFor = control.id;
        field.append(text, control);
        return field;
    }
    function dropdown(id, placeholder, options, action) {
        const node = el("select");
        node.id = id;
        const empty = el("option", "", placeholder);
        empty.value = "";
        node.append(empty);
        options.forEach(([value, title]) => {
            const option = el("option", "", title);
            option.value = value;
            node.append(option);
        });
        node.addEventListener("change", () => { if (node.value) action(node.value); });
        return node;
    }
    function disclosure(title, className) {
        const outer = el("details", className || "rfc-map-disclosure");
        outer.append(el("summary", "", title));
        return outer;
    }
    function buildShell() {
        const shell = el("div", "rfc-map-app");
        const controls = el("div", "rfc-map-controls");
        ui = {};
        ui.layer = dropdown("rfc-map-layer", "Choose a proposed layer", model.layers.map(layer => [layer.id, layer.title]), value => changeView("layer", value));
        ui.profile = dropdown("rfc-map-profile", "Choose an existing profile", model.profiles.filter(p => p.kind === "use-case").map(p => [p.id, p.title]), value => changeView("profile", value));
        const aggregates = unique(model.nodes.map(node => node.aggregate).filter(Boolean));
        ui.technology = dropdown("rfc-map-technology", "Browse another technology", sorted(aggregates).map(name => [name, model.profiles.find(p => p.id === name)?.title || human(name)]), value => changeView("technology", value));
        ui.search = el("input");
        ui.search.type = "search";
        ui.search.id = "rfc-map-search";
        ui.search.placeholder = "RFC number, title or technology";
        ui.search.value = state.query;
        ui.search.addEventListener("input", () => {
            state.query = ui.search.value;
            renderGraph();
            renderList();
            setURL();
        });
        controls.append(labelledControl("Proposed release layer", ui.layer), labelledControl("Existing release profile", ui.profile), labelledControl("Technology", ui.technology), labelledControl("Search this view", ui.search));
        shell.append(controls);
        const caption = el("div", "rfc-map-scope-row");
        ui.scope = el("div", "rfc-map-scope");
        caption.append(ui.scope, button("Browse all RFCs", () => changeView("all", ""), "rfc-map-button rfc-map-button-quiet"));
        shell.append(caption);
        const workspace = el("div", "rfc-map-workspace");
        const graphPanel = el("section", "rfc-map-graph-panel");
        graphPanel.setAttribute("aria-label", "RFC relationship graph");
        const toolbar = el("div", "rfc-map-toolbar");
        const modes = el("div", "rfc-map-view-buttons");
        ui.neighborhood = button("Neighborhood", () => { state.graphMode = "neighborhood"; renderGraph(); });
        ui.wholeView = button("This view", () => { state.graphMode = "view"; renderGraph(); });
        modes.append(ui.neighborhood, ui.wholeView);
        const actions = el("div", "rfc-map-graph-actions");
        ui.context = el("input");
        ui.context.type = "checkbox";
        ui.context.id = "rfc-map-context";
        ui.context.checked = true;
        ui.context.addEventListener("change", () => { state.context = ui.context.checked; renderGraph(); });
        const contextLabel = el("label", "rfc-map-checkbox", "");
        contextLabel.append(ui.context, document.createTextNode("Related context"));
        actions.append(contextLabel, button("−", () => zoom(-1), "rfc-map-icon-button"), button("+", () => zoom(1), "rfc-map-icon-button"), button("Reset", () => { state.zoom = 100; renderGraph(); }));
        actions.querySelectorAll("button")[0].setAttribute("aria-label", "Zoom out");
        actions.querySelectorAll("button")[1].setAttribute("aria-label", "Zoom in");
        toolbar.append(modes, actions);
        ui.graph = el("div", "rfc-map-graph");
        ui.graph.id = "rfc-map-graph";
        ui.graph.tabIndex = 0;
        ui.graph.setAttribute("aria-label", "Scrollable graph. Tab to RFC buttons, or use the document list below.");
        const legend = el("div", "rfc-map-legend");
        Object.entries(edgeLabels).forEach(([kind, label]) => legend.append(el("span", "rfc-map-edge-key rfc-map-edge-key-" + kind, label)));
        legend.append(el("span", "rfc-map-context-key", "Dashed card: context only"));
        ui.graphNote = note("", "rfc-map-graph-caption");
        graphPanel.append(toolbar, ui.graph, legend, ui.graphNote);
        ui.inspector = el("aside", "rfc-map-inspector");
        ui.inspector.id = "rfc-map-inspector";
        ui.inspector.tabIndex = -1;
        ui.inspector.setAttribute("aria-label", "Selected RFC details");
        workspace.append(graphPanel, ui.inspector);
        shell.append(workspace);
        ui.list = disclosure("Documents in this view", "rfc-map-disclosure rfc-map-document-list");
        ui.list.open = true;
        ui.listContent = el("div", "rfc-map-list-content");
        ui.list.append(ui.listContent);
        shell.append(ui.list);
        ui.inventory = disclosure("Specifications needing RFC mapping");
        shell.append(ui.inventory);
        shell.append(renderProvenance());
        ui.live = el("p", "sr-only");
        ui.live.setAttribute("role", "status");
        ui.live.setAttribute("aria-live", "polite");
        shell.append(ui.live);
        root.replaceChildren(shell);
    }
    function zoom(direction) {
        const levels = [65, 80, 100, 125, 150];
        state.zoom = levels[Math.max(0, Math.min(levels.length - 1, levels.indexOf(state.zoom) + direction))];
        renderGraph();
    }
    function render() {
        ui.layer.value = state.mode === "layer" ? state.filter : "";
        ui.profile.value = state.mode === "profile" ? state.filter : "";
        ui.technology.value = state.mode === "technology" ? state.filter : "";
        ui.scope.replaceChildren(el("strong", "", scopeTitle()), el("span", "", state.mode === "layer" ? "Proposed capability boundary · changes no support claim" : state.mode === "profile" ? "Required members only · optional members remain context" : "Browsing scope · relationships do not add obligations"));
        renderGraph();
        renderList();
        renderInspector();
        setURL();
    }
    function graphMembers() {
        const inView = scope();
        const matchesView = filteredScope();
        const adjacent = new Set();
        model.edges.forEach(edge => {
            if (edge.from === state.selected) adjacent.add(edge.to);
            if (edge.to === state.selected) adjacent.add(edge.from);
        });
        let shown = state.graphMode === "neighborhood" ? matchesView.filter(stem => adjacent.has(stem) || stem === state.selected) : matchesView.slice(0, 28);
        if (!shown.includes(state.selected)) shown.unshift(state.selected);
        if (state.context) {
            const context = sorted(Array.from(adjacent).filter(stem => nodes.has(stem) && !shown.includes(stem)));
            // Updates are the useful first context for the selected base RFC;
            // the complete relationship list remains in its inspector.
            const updaters = new Set(model.edges.filter(edge => edge.to === state.selected && edge.kind === "updates").map(edge => edge.from));
            context.sort((a, b) => Number(inView.has(b)) - Number(inView.has(a)) ||
                Number(updaters.has(b)) - Number(updaters.has(a)) ||
                Number(/^rfc\d+$/.test(b)) - Number(/^rfc\d+$/.test(a)) ||
                Number(nodes.get(b).summary) - Number(nodes.get(a).summary));
            const limit = state.graphMode === "neighborhood" ? 12 : 36;
            shown = shown.concat(context.slice(0, Math.max(0, limit - shown.length)));
        }
        return { shown: unique(shown).filter(stem => nodes.has(stem)), inView, matched: matchesView.length, adjacent: adjacent.size };
    }
    function renderGraph() {
        const graph = graphMembers();
        ui.neighborhood.setAttribute("aria-pressed", String(state.graphMode === "neighborhood"));
        ui.wholeView.setAttribute("aria-pressed", String(state.graphMode === "view"));
        ui.graph.dataset.zoom = String(state.zoom);
        const columns = [[], [state.selected], []];
        const outward = new Set(model.edges.filter(edge => edge.from === state.selected).map(edge => edge.to));
        graph.shown.filter(stem => stem !== state.selected).forEach(stem => {
            let column = outward.has(stem) ? 0 : 2;
            if (!model.edges.some(edge => (edge.from === stem && edge.to === state.selected) || (edge.to === stem && edge.from === state.selected))) {
                column = columns[0].length <= columns[2].length ? 0 : 2;
            }
            columns[column].push(stem);
        });
        const rowHeight = 110;
        const rows = Math.max(4, ...columns.map(column => column.length));
        const height = rows * rowHeight + 36;
        const positions = new Map();
        columns.forEach((column, col) => column.forEach((stem, row) => {
            positions.set(stem, { x: 22 + col * 254, y: 18 + row * rowHeight + (rows - column.length) * rowHeight / 2 });
        }));
        const canvas = svg("svg", { viewBox: "0 0 752 " + height, width: 752, height, "aria-label": "Relationships around " + display(state.selected) });
        canvas.classList.add("rfc-map-canvas");
        const defs = svg("defs");
        ["builds-on", "updates", "obsoletes"].forEach(kind => {
            const marker = svg("marker", { id: "rfc-map-arrow-" + kind, viewBox: "0 0 10 10", refX: 9, refY: 5, markerWidth: 5, markerHeight: 5, orient: "auto-start-reverse" });
            marker.append(svg("path", { d: "M 0 0 L 10 5 L 0 10 z", class: "rfc-map-arrow" }));
            defs.append(marker);
        });
        canvas.append(defs);
        model.edges.forEach(edge => {
            const from = positions.get(edge.from);
            const to = positions.get(edge.to);
            if (!from || !to || edge.from === edge.to) return;
            const right = to.x > from.x;
            const x1 = from.x + (right ? 200 : 0);
            const x2 = to.x + (right ? 0 : 200);
            const y1 = from.y + 46;
            const y2 = to.y + 46;
            const bend = right ? 45 : -45;
            const path = svg("path", { d: "M " + x1 + " " + y1 + " C " + (x1 + bend) + " " + y1 + ", " + (x2 - bend) + " " + y2 + ", " + x2 + " " + y2,
                class: "rfc-map-edge rfc-map-edge-" + edge.kind, "marker-end": "url(#rfc-map-arrow-" + edge.kind + ")" });
            const title = svg("title");
            title.textContent = display(edge.from) + " " + (edgeLabels[edge.kind] || edge.kind).toLowerCase() + " " + display(edge.to);
            path.append(title);
            canvas.append(path);
        });
        graph.shown.forEach(stem => {
            const data = nodes.get(stem);
            const position = positions.get(stem);
            const holder = svg("foreignObject", { x: position.x, y: position.y, width: 200, height: 94 });
            const card = document.createElementNS(htmlNS, "button");
            card.type = "button";
            card.className = "rfc-map-node" + (graph.inView.has(stem) ? "" : " rfc-map-node-context") + (stem === state.selected ? " rfc-map-node-selected" : "");
            card.setAttribute("aria-pressed", String(stem === state.selected));
            card.setAttribute("aria-label", display(stem) + ": " + (data.title || "No summary") + (graph.inView.has(stem) ? ". In this view." : ". Related context only."));
            card.addEventListener("click", () => select(stem));
            const top = el("span", "rfc-map-node-top");
            top.append(el("strong", "", display(stem)), el("small", "", graph.inView.has(stem) ? "View" : "Context"));
            const proof = el("span", "rfc-map-node-proof", shortProof(figureValues.get(stem)));
            proof.dataset.rfcProof = stem;
            const title = el("span", "rfc-map-node-title", data.title || "No summary available");
            title.title = data.title || "";
            card.append(top, title, proof);
            if (Array.isArray(data.component) && data.component.length > 1) card.append(el("span", "rfc-map-cycle", "Cycle · " + data.component.length));
            holder.append(card);
            canvas.append(holder);
            ensureFigure(stem);
        });
        ui.graph.replaceChildren(canvas);
        const selected = positions.get(state.selected);
        const ratio = state.zoom / 100;
        ui.graph.scrollTop = Math.max(0, (selected.y + 46) * ratio - ui.graph.clientHeight / 2);
        const contextCount = graph.shown.filter(stem => !graph.inView.has(stem)).length;
        ui.graphNote.textContent = graph.shown.length + " documents shown · " + contextCount + " context only. " + graph.matched + " documents match this view; all remain available in the list. Arrows preserve the source relationship, not a release commitment.";
    }
    function renderList() {
        const members = filteredScope();
        ui.list.querySelector("summary").textContent = "Documents in this view · " + members.length;
        const list = el("div", "rfc-map-document-grid");
        members.forEach(stem => {
            const data = nodes.get(stem);
            const item = button("", () => select(stem), "rfc-map-document");
            item.setAttribute("aria-pressed", String(stem === state.selected));
            item.append(el("strong", "", display(stem)), el("span", "", data.title || "No summary available"));
            // Only visible graph nodes are fetched automatically. The list
            // remains small and useful without fetching every RFC figure.
            list.append(item);
        });
        ui.listContent.replaceChildren(members.length ? list : note("No documents match this search. Clear the search or browse all RFCs."));
    }
    function metric(label, value) {
        const node = el("div", "rfc-map-metric");
        node.append(el("strong", "", String(value)), el("span", "", label));
        return node;
    }
    function paintProof(host, figure) {
        host.replaceChildren();
        host.append(el("h3", "rfc-map-eyebrow", "Recorded mandatory proof"));
        if (figure.error) {
            host.append(note(figure.error, "rfc-map-error"), note("The recorded figure could not be loaded. Open the canonical RFC record for its source; no percentage has been inferred."));
            return;
        }
        if (figure.unknown) {
            host.append(el("p", "rfc-map-proof-value", "Unknown"), note(figure.unknown === "No summary" ? "This document has no summary. Its requirements and proof have not been assessed here." : "The requirement extraction is not signed off. No bounded proof percentage is available."));
            return;
        }
        const must = figure.must;
        host.append(el("p", "rfc-map-proof-value", figure.percent === null ? (must.total ? "None owed" : "No MUST rows") : figure.percent.toFixed(1).replace(/\.0$/, "") + "%"));
        if (figure.percent !== null) {
            host.append(note(figure.proven + " of " + must.kept + " owed mandatory requirements proven. This card always covers the full mandatory register."));
            const bar = svg("svg", { viewBox: "0 0 100 4", width: "100%", height: 10, role: "img", "aria-label": figure.proven + " proven, " + must["proof-pending"] + " awaiting proof, " + must["not-built"] + " recorded unmet" });
            const segments = [
                ["ze-proven", "Ze proven"], ["os-verified", "OS verified"], ["proof-pending", "Proof pending"],
                ["os-gap", "OS testing gap"], ["exclusion-unchecked", "Exclusion unchecked"], ["not-assessed", "Not assessed"], ["not-built", "Recorded unmet"]
            ];
            let offset = 0;
            segments.forEach(([key, title]) => {
                const width = 100 * must[key] / must.kept;
                if (width) {
                    const rectangle = svg("rect", { x: offset, y: 0, width, height: 4, class: "rfc-map-proof-" + key });
                    const tooltip = svg("title");
                    tooltip.textContent = title + ": " + must[key];
                    rectangle.append(tooltip);
                    bar.append(rectangle);
                }
                offset += width;
            });
            host.append(bar);
            const metrics = el("div", "rfc-map-metrics");
            metrics.append(metric("Proven", figure.proven), metric("Proof pending", must["proof-pending"]), metric("Recorded unmet", must["not-built"]), metric("Not assessed", must["not-assessed"]));
            if (must["os-gap"] || must["exclusion-unchecked"]) metrics.append(metric("OS testing gap", must["os-gap"]), metric("Exclusions unchecked", must["exclusion-unchecked"]));
            host.append(metrics);
        } else host.append(note(must.total ? "Every listed mandatory row is recorded as not owed. This is not a 100% proof result." : "The recorded summary lists no mandatory requirements. Advisory requirements can still apply."));
        if (must["not-owed"]) host.append(note(must["not-owed"] + " mandatory rows are separately recorded as not owed."));
        host.append(note("Proof is recorded evidence, not a fresh source audit or a release verdict. Recorded gaps can need source reconciliation.", "rfc-map-caption"));
    }
    function renderInspector() {
        const data = nodes.get(state.selected);
        if (!data) return;
        const target = ui.inspector;
        target.replaceChildren();
        target.append(el("div", "rfc-map-eyebrow", scope().has(data.stem) ? "Selected in this view" : "Related context · outside this view"), el("h2", "", display(data.stem)), note(data.title || "No summary available", "rfc-map-inspector-title"));
        if (data.aggregate) target.append(el("span", "rfc-map-chip", human(data.aggregate)));
        if (Array.isArray(data.component) && data.component.length > 1) target.append(note("Citation cycle: " + data.component.map(display).join(", ")));
        const links = el("div", "rfc-map-inspector-links");
        if (data.summary) links.append(link("Open RFC evidence ↗", detail(data.stem)));
        else if (/^rfc\d+$/.test(data.stem)) links.append(link("Open RFC text ↗", "https://www.rfc-editor.org/rfc/" + encodeURIComponent(data.stem), "rfc-map-external"));
        else links.append(link("Browse standards records ↗", new URL("quality/rfc-compliance/", site).href));
        if (!scope().has(data.stem)) links.append(button("Add to this view", () => { state.added.push(data.stem); render(); }, "rfc-map-button rfc-map-button-quiet"));
        target.append(links);
        const proof = el("section", "rfc-map-proof-card");
        proof.append(note("Loading the canonical proof record…"));
        target.append(proof);
        ensureFigure(data.stem).then(figure => { if (proof.isConnected) paintProof(proof, figure); });
        renderRelationships(target, data.stem);
        const requirements = model.requirements[data.stem] || [];
        const section = el("section", "rfc-map-requirements");
        section.append(el("h3", "", "Requirements and linked work"));
        const levels = sorted(unique(requirements.map(row => row.level).filter(Boolean)));
        const level = dropdown("rfc-map-level", "All requirement keywords", [["mandatory", "MUST family · mandatory"], ...levels.map(value => [value, value + " only"])], value => {
            state.level = value;
            renderRequirementList(body, data.stem);
        });
        // The empty option is a real all-keywords filter here.
        level.addEventListener("change", () => {
            if (!level.value) { state.level = "all"; renderRequirementList(body, data.stem); }
        });
        level.value = state.level === "all" ? "" : levels.includes(state.level) || state.level === "mandatory" ? state.level : "mandatory";
        if (level.value === "mandatory") state.level = "mandatory";
        section.append(labelledControl("Show statements", level));
        const body = el("div", "rfc-map-requirement-body");
        section.append(body);
        renderRequirementList(body, data.stem);
        target.append(section);
    }
    function renderRelationships(target, stem) {
        const edges = model.edges.filter(edge => edge.from === stem || edge.to === stem);
        const block = disclosure("Relationships · " + edges.length);
        const list = el("ul", "rfc-map-relationships");
        edges.forEach(edge => {
            const other = edge.from === stem ? edge.to : edge.from;
            if (!nodes.has(other)) return;
            const item = el("li");
            const words = edge.from === stem ? (edgeLabels[edge.kind] || edge.kind) : edge.kind === "builds-on" ? "Cited by" : edge.kind === "updates" ? "Updated by" : "Obsoleted by";
            item.append(el("span", "", words), button(display(other), () => select(other), "rfc-map-text-button"));
            list.append(item);
        });
        block.append(note("Citations suggest context. Header updates do not identify which requirement was rewritten. Exact row applicability remains subject to review."), list);
        target.append(block);
    }
    function relationsFor(stem) {
        const rows = model.requirements[stem] || [];
        const eligible = new Set(rows.filter(row => state.level === "all" || state.level === "mandatory" && row.gated || row.level === state.level).map(row => row.rid));
        return model.relations.filter(relation => relation.stem === stem && (!relation.rid || eligible.has(relation.rid)));
    }
    function renderRequirementList(target, stem) {
        const all = model.requirements[stem] || [];
        const rows = all.filter(row => state.level === "all" || state.level === "mandatory" && row.gated || row.level === state.level);
        target.replaceChildren(note(rows.length + " of " + all.length + " listed statements. Keyword filtering changes this list and exact-ID relations, never the mandatory proof card."));
        const relations = relationsFor(stem);
        const identities = unique(relations.map(row => row.spec));
        const specBlock = disclosure("Associated specifications · " + identities.length);
        identities.forEach(name => {
            const item = el("div", "rfc-map-spec");
            const spec = specs.get(name);
            if (spec?.["source-url"] && safeURL(spec["source-url"])) item.append(link(spec.title || human(name), spec["source-url"]));
            else item.append(el("strong", "", human(name)));
            item.append(note(spec ? [spec.priority, spec.status, human(spec.bucket)].filter(Boolean).join(" · ") : "Specification absent from the recorded inventory"));
            unique(relations.filter(row => row.spec === name).map(row => (relationLabels[row.kind] || row.kind) + (row.rid ? " · " + row.rid : ""))).forEach(text => item.append(note(text, "rfc-map-caption")));
            const evidence = relations.filter(row => row.spec === name && (row.source || row.note));
            if (evidence.length) {
                const sources = disclosure("Association sources");
                evidence.forEach(row => sources.append(note([row.source, row.note].filter(Boolean).join(" · "), "rfc-map-source")));
                item.append(sources);
            }
            specBlock.append(item);
        });
        if (!identities.length) specBlock.append(note("No recorded association for this selection. This does not establish that no work is owed."));
        else specBlock.append(note("Associations are not confirmed implementation ownership. A whole-RFC relation has no assigned requirement level."));
        target.append(specBlock);
        const list = el("div", "rfc-map-statements");
        rows.forEach(row => {
            const item = el("article", "rfc-map-statement");
            const heading = el("div", "rfc-map-statement-heading");
            heading.append(el("span", "rfc-map-keyword", row.level), link(row.rid, detail(stem, row.rid)));
            item.append(heading, note(row.text));
            if (row.annotation) {
                item.append(el("span", "rfc-map-chip", "Recorded " + human(row.annotation.kind)));
                if (row.annotation.reason) {
                    const why = disclosure("Recorded annotation");
                    why.append(note(row.annotation.reason));
                    item.append(why);
                }
            }
            if (row.section) item.append(note("Section " + row.section, "rfc-map-caption"));
            list.append(item);
        });
        if (!rows.length) list.append(note(all.length ? "No listed statements match this keyword." : "No requirement statements are recorded in this map for this document."));
        target.append(list);
    }
    function safeURL(value) {
        if (typeof value !== "string" || !value.trim()) return false;
        try { const parsed = new URL(value, document.baseURI); return parsed.protocol === "https:" || parsed.protocol === "http:"; }
        catch (_) { return false; }
    }
    function renderInventory() {
        ui.inventory.replaceChildren(el("summary", "", "Specifications needing RFC mapping"));
        if (!roadmap) {
            ui.inventory.append(note("The specification inventory is unavailable: " + roadmapError + ". No unmapped count has been inferred.", "rfc-map-error"));
            return;
        }
        const mapped = new Set(model.relations.map(row => row.spec));
        const unmapped = roadmap.items.filter(spec => !mapped.has(spec.name));
        ui.inventory.querySelector("summary").textContent = "Specifications needing RFC mapping · " + unmapped.length + " of " + roadmap.items.length;
        ui.inventory.append(note("Every recorded specification remains accounted for. These have no association in this map; their absence from an RFC is not evidence that they are outside release scope."));
        if (roadmap.revision !== model["roadmap-revision"]) ui.inventory.append(note("The inventory and association projection name different revisions. Associations remain navigable, but their joined counts need reconciliation.", "rfc-map-error"));
        const search = el("input");
        search.type = "search";
        search.id = "rfc-map-spec-search";
        search.placeholder = "Find an unmapped specification";
        const list = el("div", "rfc-map-unmapped-list");
        let limit = 50;
        const draw = () => {
            const query = search.value.toLowerCase();
            const matches = unmapped.filter(spec => [spec.name, spec.title, spec.bucket, spec.status].join(" ").toLowerCase().includes(query));
            list.replaceChildren(note(matches.length + " matching specifications"));
            matches.slice(0, limit).forEach(spec => {
                const row = el("div", "rfc-map-unmapped");
                row.append(safeURL(spec["source-url"]) ? link(spec.title || human(spec.name), spec["source-url"]) : el("strong", "", spec.title || human(spec.name)), el("span", "", [spec.priority, spec.status, human(spec.bucket)].filter(Boolean).join(" · ")));
                list.append(row);
            });
            if (matches.length > limit) list.append(button("Show 50 more", () => { limit += 50; draw(); }));
        };
        search.addEventListener("input", () => { limit = 50; draw(); });
        ui.inventory.append(labelledControl("Search unmapped work", search), list);
        draw();
    }
    function renderProvenance() {
        const block = disclosure("Sources, scope and mapping diagnostics");
        block.append(note("The graph, specification inventory and proof records can describe different snapshots. File or render time is not evidence freshness. This page is a planning aid, not release approval."));
        const inputs = el("div", "rfc-map-inputs");
        (model.inputs || []).forEach(input => {
            const item = el("div", "rfc-map-input");
            item.append(el("strong", "", input.label || "Recorded input"), el("code", "", input.path || "Path unknown"), note("Revision: " + (input.revision || "unknown") + " · Evidence age: unknown", "rfc-map-caption"), el("code", "rfc-map-hash", "SHA-256 " + (input.sha256 || "unknown")));
            inputs.append(item);
        });
        block.append(inputs, note("Per-RFC figures load separately from the canonical data/rfc/ records. No timestamp in these records has been substituted for the time evidence was collected."));
        const diagnostics = model.diagnostics || [];
        block.append(el("h3", "", "Mapping diagnostics · " + diagnostics.length));
        if (!diagnostics.length) block.append(note("No input mapping diagnostic was recorded. Unmapped specifications remain listed separately."));
        diagnostics.forEach(item => block.append(note([item.message, item.spec, item.rid, item.reference, item.source].filter(Boolean).join(" · "), "rfc-map-source")));
        return block;
    }
    async function start() {
        const results = await Promise.allSettled([
            json(new URL(root.dataset.source || "../../data/rfc-map.json", document.baseURI)),
            json(new URL(root.dataset.roadmap || "../../data/release-roadmap.json", document.baseURI))
        ]);
        if (results[0].status !== "fulfilled") throw results[0].reason;
        model = validateMap(results[0].value);
        if (results[1].status === "fulfilled" && Array.isArray(results[1].value.items)) roadmap = results[1].value;
        else roadmapError = results[1].status === "rejected" ? results[1].reason.message : "unsupported inventory format";
        nodes = new Map(model.nodes.map(node => [node.stem, node]));
        specs = new Map((roadmap?.items || []).map(spec => [spec.name, spec]));
        const query = new URL(window.location.href).searchParams;
        const initialLayer = model.layers.find(layer => layer.id === query.get("layer")) || model.layers.find(layer => layer.id === model["default-layer"]) || model.layers[0];
        state = { mode: "layer", filter: initialLayer?.id || "", selected: nodes.has(query.get("rfc")) ? query.get("rfc") : initialLayer?.stems[0] || model.nodes[0]?.stem,
            query: query.get("q") || "", level: "mandatory", context: true, graphMode: "neighborhood", zoom: 100, added: [] };
        if (query.get("view") === "all") { state.mode = "all"; state.filter = ""; }
        if (!state.selected) throw new Error("The recorded graph contains no documents.");
        const profile = model.profiles.find(item => item.id === query.get("profile"));
        if (profile) { state.mode = "profile"; state.filter = profile.id; }
        if (model.nodes.some(node => node.aggregate === query.get("technology"))) { state.mode = "technology"; state.filter = query.get("technology"); }
        buildShell();
        render();
        renderInventory();
    }
    start().catch(error => {
        const message = note("The interactive RFC map could not load: " + error.message + ". Reload the page after the rfc-map producer has published its recorded inputs. The reference links below remain available.", "rfc-map-error");
        message.setAttribute("role", "alert");
        root.prepend(message);
    });
}());
