// Design: website/AI.md -- the size view and the commits curve use the activity widget's theme roles.
package site

// activityGrowthStyle dresses the view tabs, the size view and the commits
// curve. Each section's series color is a site token, so it follows the
// theme; the twelve roles repeat in order when a file lists more sections.
const activityGrowthStyle = `<style>
    .activity-widget .activity-tabs {
        display: flex;
        flex-wrap: wrap;
        gap: 0.3rem;
        width: fit-content;
        max-width: 100%;
        padding: 0.32rem;
        border: 2px solid var(--activity-border);
        border-radius: var(--radius-lg);
        background: var(--activity-card-strong);
        box-shadow: var(--clay);
    }
    .activity-widget .activity-tabs[hidden],
    .activity-widget .activity-view[hidden],
    .activity-widget .size-explorer[hidden] { display: none; }
    .activity-widget {
        --activity-series-1: var(--sky-deep);
        --activity-series-2: var(--teal-deep);
        --activity-series-3: var(--grape-deep);
        --activity-series-4: var(--coral);
        --activity-series-5: var(--amber);
        --activity-series-6: var(--lavender);
        --activity-series-7: var(--green);
        --activity-series-8: var(--gold);
        --activity-series-9: var(--pink);
        --activity-series-10: var(--mint-deep);
        --activity-series-11: var(--danger-deep);
        --activity-series-12: var(--text);
    }
    .activity-widget [data-series="1"] { --series: var(--activity-series-1); }
    .activity-widget [data-series="2"] { --series: var(--activity-series-2); }
    .activity-widget [data-series="3"] { --series: var(--activity-series-3); }
    .activity-widget [data-series="4"] { --series: var(--activity-series-4); }
    .activity-widget [data-series="5"] { --series: var(--activity-series-5); }
    .activity-widget [data-series="6"] { --series: var(--activity-series-6); }
    .activity-widget [data-series="7"] { --series: var(--activity-series-7); }
    .activity-widget [data-series="8"] { --series: var(--activity-series-8); }
    .activity-widget [data-series="9"] { --series: var(--activity-series-9); }
    .activity-widget [data-series="10"] { --series: var(--activity-series-10); }
    .activity-widget [data-series="11"] { --series: var(--activity-series-11); }
    .activity-widget [data-series="12"] { --series: var(--activity-series-12); }
    .activity-tabs button {
        border: 0;
        border-radius: var(--radius-md);
        padding: 0.45rem 0.7rem;
        background: transparent;
        color: var(--muted);
        font: inherit;
        font-size: 0.74rem;
        font-weight: 900;
        letter-spacing: 0.03em;
        text-transform: uppercase;
        cursor: pointer;
    }
    .activity-tabs button:focus-visible,
    .growth-chart:focus-visible,
    .growth-date-control input:focus-visible {
        outline: 2px solid var(--activity-ink);
        outline-offset: 3px;
    }
    .activity-widget .activity-view {
        display: grid;
        gap: 1rem;
        min-width: 0;
    }
    .activity-calendar-footer {
        display: grid;
        grid-template-columns: minmax(0, 1fr) auto;
        align-items: center;
        gap: 0.75rem;
    }
    .activity-widget .activity-calendar-footer .pill { max-width: 100%; white-space: normal; }
    .activity-widget .activity-calendar-footer .legend { margin: 0; }
    .activity-size p,
    .activity-commits p,
    .activity-widget .activity-note {
        margin: 0;
        color: var(--muted);
        line-height: 1.5;
    }
    .activity-widget .activity-size h2,
    .activity-widget .activity-commits h2 { margin: 0; }
    .activity-widget .activity-size,
    .activity-widget .activity-commits { display: grid; gap: 1rem; min-width: 0; }
    .activity-widget .activity-size .size-headline { color: var(--text); font-size: 1.15rem; }
    .size-sections { display: grid; grid-template-columns: repeat(auto-fill, minmax(10rem, 1fr)); gap: 0.8rem; }
    .activity-widget .size-sections .stat span { overflow-wrap: anywhere; }
    .size-legend { display: flex; flex-wrap: wrap; gap: 0.4rem; }
    .size-toggle {
        display: inline-flex;
        align-items: center;
        gap: 0.4rem;
        border: 1px solid var(--activity-border);
        border-radius: 999px;
        padding: 0.3rem 0.65rem;
        background: var(--activity-card);
        color: var(--text);
        font: inherit;
        font-size: 0.8rem;
        font-weight: 700;
        cursor: pointer;
    }
    .size-toggle[aria-pressed="false"] { color: var(--muted); text-decoration: line-through; }
    .size-toggle[aria-pressed="false"] .size-swatch { opacity: 0.3; }
    .size-toggle:focus-visible { outline: 2px solid var(--activity-ink); outline-offset: 2px; }
    .size-swatch { flex: none; width: 0.8rem; height: 0.8rem; border-radius: 0.2rem; background: var(--series); }
    .growth-series.size-line { stroke: var(--series); stroke-width: 2; }
    .growth-point.size-point { fill: var(--series); }
    .change-gained { fill: var(--teal-deep); }
    .change-lost { fill: var(--coral); }
    .size-readout { display: grid; gap: 0.5rem; min-width: 0; }
    .size-table-wrap { max-width: 100%; overflow-x: auto; }
    .size-table { width: 100%; border-collapse: collapse; font-size: 0.85rem; }
    .size-table th, .size-table td { padding: 0.2rem 0.45rem; border-bottom: 1px solid var(--activity-border); text-align: right; }
    .size-table th:first-child { text-align: left; font-weight: 700; overflow-wrap: normal; word-break: normal; }
    .growth-charts { display: grid; gap: 1rem; min-width: 0; }
    .growth-charts > div {
        min-width: 0;
        border: 1px solid var(--activity-border);
        border-radius: 0.8rem;
        padding: 0.75rem;
        background: var(--activity-card);
    }
    .growth-charts h3 { margin: 0 0 0.5rem; color: var(--text); font-size: 0.95rem; }
    .growth-chart { display: block; width: 100%; height: auto; overflow: visible; }
    .growth-grid { fill: none; stroke: var(--activity-border); stroke-width: 1; }
    .growth-axis { fill: var(--muted); font-size: 14px; }
    .growth-series { fill: none; stroke: var(--activity-level-4); stroke-width: 3; }
    .growth-series.commits-series { stroke: var(--sky-deep); }
    .growth-series, .growth-cursor { vector-effect: non-scaling-stroke; }
    .growth-cursor { fill: none; stroke: var(--text); stroke-width: 1; stroke-dasharray: 4 4; }
    .growth-point { fill: var(--text); stroke: var(--activity-card); stroke-width: 2; }
    .growth-date-control { display: grid; gap: 0.5rem; color: var(--text); font-weight: 700; }
    .growth-date-control input { width: 100%; accent-color: var(--teal-deep); }
    .growth-readout {
        display: flex;
        flex-wrap: wrap;
        gap: 0.5rem 1.25rem;
        color: var(--text);
        font-variant-numeric: tabular-nums;
    }
    @media (max-width: 620px) {
        .size-sections { grid-template-columns: repeat(2, minmax(0, 1fr)); }
        .activity-widget .size-sections .stat span { font-size: 0.7rem; }
        .size-table { font-size: 0.78rem; }
        .size-table th, .size-table td { padding: 0.2rem 0.25rem; }
        .growth-charts > div { padding: 0.4rem; }
        .growth-axis { font-size: 21px; }
    }
</style>
`
