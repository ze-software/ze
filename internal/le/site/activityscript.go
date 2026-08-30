// Design: the activity page's own script, recovered from the published page
// Related: activity.go writes the metric summaries this script reads.
package site

// activityPageScript drives the heatmap: it switches the grid and the summary
// cards between added lines and commits, and it fills the hover tooltip.
//
// It opens on the line the page's own markup already shows, so a reader with
// no JavaScript still reads the added-line grid and its four summary cards.
// The metric summaries are written immediately above it by the producer,
// because they are the page's own numbers rather than shared code.
//
// It is a raw string because it holds quotation marks of both kinds and no
// backtick, and because a reader comparing it against the page it ships to
// should meet the same characters in both. The published page spelled four
// values as template literals; they are written here as concatenation, which
// answers the same strings and keeps this constant one raw string.
const activityPageScript = `let currentMetric = "lines";
const cells = Array.from(document.querySelectorAll(".activity-grid .day-cell"));
const buttons = Array.from(document.querySelectorAll(".metric-switch button"));
const tooltip = document.getElementById("activity-tooltip");
const tooltipDate = tooltip.querySelector(".tooltip-date");
const tooltipPrimary = tooltip.querySelector(".tooltip-primary");
const tooltipSecondary = tooltip.querySelector(".tooltip-secondary");

function plural(value, one, many) {
    return value === 1 ? one : many;
}

function valueLine(metric, cell) {
    if (metric === "commits") {
        const value = Number(cell.dataset.commits || "0");
        return cell.dataset.commitsDisplay + " " + plural(value, "commit", "commits");
    }
    const value = Number(cell.dataset.lines || "0");
    return cell.dataset.linesDisplay + " " + plural(value, "line", "lines") + " added";
}

function setEl(id, text) {
    var el = document.getElementById(id);
    if (el) el.textContent = text;
}

function setMetric(metric) {
    currentMetric = metric;
    document.body.dataset.metric = metric;
    const summary = metricSummaries[metric];
    setEl("total-label", summary.totalLabel);
    setEl("total-value", summary.totalValue);
    setEl("active-label", summary.activeLabel);
    setEl("active-value", summary.activeValue);
    setEl("peak-label", summary.peakLabel);
    setEl("peak-value", summary.peakValue);
    setEl("threshold-label", summary.thresholdLabel);
    setEl("threshold-value", summary.thresholdValue);
    for (const button of buttons) {
        button.setAttribute("aria-pressed", String(button.dataset.metric === metric));
    }
    for (const cell of cells) {
        cell.dataset.level = metric === "commits" ? cell.dataset.commitsLevel : cell.dataset.linesLevel;
    }
}

function fillTooltip(cell) {
    tooltipDate.textContent = cell.dataset.dateLabel;
    if (currentMetric === "commits") {
        tooltipPrimary.textContent = valueLine("commits", cell);
        tooltipSecondary.textContent = valueLine("lines", cell);
    } else {
        tooltipPrimary.textContent = valueLine("lines", cell);
        tooltipSecondary.textContent = valueLine("commits", cell);
    }
}

function placeTooltip(x, y) {
    const margin = 14;
    tooltip.hidden = false;
    let left = x + margin;
    let top = y + margin;
    const width = tooltip.offsetWidth;
    const height = tooltip.offsetHeight;
    if (left + width + margin > window.innerWidth) {
        left = x - width - margin;
    }
    if (top + height + margin > window.innerHeight) {
        top = y - height - margin;
    }
    tooltip.style.left = Math.max(margin, left) + "px";
    tooltip.style.top = Math.max(margin, top) + "px";
}

function showTooltip(cell, event) {
    fillTooltip(cell);
    placeTooltip(event.clientX, event.clientY);
}

function showFocusTooltip(cell) {
    const rect = cell.getBoundingClientRect();
    fillTooltip(cell);
    placeTooltip(rect.left + rect.width / 2, rect.top + rect.height / 2);
}

function hideTooltip() {
    tooltip.hidden = true;
}

for (const button of buttons) {
    button.addEventListener("click", () => setMetric(button.dataset.metric));
}
for (const cell of cells) {
    cell.addEventListener("pointerenter", (event) => showTooltip(cell, event));
    cell.addEventListener("pointermove", (event) => placeTooltip(event.clientX, event.clientY));
    cell.addEventListener("pointerleave", hideTooltip);
    cell.addEventListener("focus", () => showFocusTooltip(cell));
    cell.addEventListener("blur", hideTooltip);
}
setMetric("lines");
</script>
`
