// Design: website/AI.md -- growth charts use the activity widget's theme roles.
package site

const activityGrowthStyle = `<style>
    .activity-widget .activity-tabs {
        display: flex;
        flex-direction: column;
        align-self: center;
        gap: 0.3rem;
        width: fit-content;
        padding: 0.32rem;
        border: 2px solid var(--activity-border);
        border-radius: var(--radius-md);
        background: var(--activity-card-strong);
        box-shadow: var(--clay);
    }
    .activity-widget .activity-tabs[hidden],
    .activity-widget .activity-view[hidden] { display: none; }
    .activity-tabs button {
        border: 0;
        border-radius: var(--radius-sm);
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
    .activity-tabs button[aria-selected="true"] {
        background: var(--activity-card);
        color: var(--text);
        box-shadow: var(--clay);
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
    .activity-growth p {
        margin: 0;
        color: var(--muted);
        line-height: 1.5;
    }
    .activity-widget .activity-growth h2 { margin: 0; }
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
    #activity-growth-commits-chart .growth-series { stroke: var(--sky-deep); }
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
    .growth-snapshot { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 0.8rem; }
    @media (max-width: 620px) {
        .growth-snapshot { grid-template-columns: repeat(2, minmax(0, 1fr)); }
        .growth-charts > div { padding: 0.4rem; }
        .growth-axis { font-size: 21px; }
    }
</style>
`
