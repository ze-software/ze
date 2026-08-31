// Design: the deck embed's own stylesheet, dark and sized to one slide
// Related: activity.go renders the widget these rules lay out, and
// activitystyle.go dresses the same widget for the published web page.
package site

// activitySlideStyle is the whole stylesheet of the talk-deck embed. The deck
// shows the embed in an iframe 70vh tall, so every size here is a fraction of
// the viewport: the grid, the type and the padding shrink together, and the
// widget lands inside one slide rather than under a scrollbar.
//
// It replaces the site stylesheet rather than overriding it. The published page
// is light because a site reader arrives from a light page; a slide is dark
// because a projected deck is dark, and the two are separate renderings of one
// measurement rather than one rendering with a theme switch on top.
//
// It is a raw string because it holds quotation marks and no backtick, so a
// reader comparing it against the document it ships to meets the same
// characters in both.
const activitySlideStyle = `        <style>
        :root {
            color-scheme: dark;
            --bg: #071118;
            --panel: rgba(13, 26, 36, 0.82);
            --panel-strong: rgba(17, 34, 47, 0.94);
            --line: rgba(143, 202, 214, 0.18);
            --text: #e8f7fb;
            --muted: #8fb7c3;
            --soft: #476977;
            --cell: clamp(7px, 2.4vh, 18px);
            --gap: clamp(2px, 0.45vh, 4px);
            --level-0: rgba(255, 255, 255, 0.055);
            --level-1: #143d45;
            --level-2: #1d6869;
            --level-3: #299e91;
            --level-4: #48d2b2;
            --level-5: #bef86a;
        }
        * { box-sizing: border-box; }
        html, body { height: 100%; }
        html {
            font-size: clamp(8px, 2.4vh, 20px);
        }
        body {
            margin: 0;
            color: var(--text);
            background:
                radial-gradient(circle at 12% 0%, rgba(72, 210, 178, 0.18), transparent 28rem),
                radial-gradient(circle at 88% 10%, rgba(190, 248, 106, 0.12), transparent 26rem),
                linear-gradient(135deg, #071118 0%, #0d1f2d 52%, #061016 100%);
            font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
            line-height: 1.35;
        }
        .activity-slide {
            display: grid;
            align-content: start;
            gap: 0.55rem;
            width: min(1600px, 100%);
            height: 100%;
            margin: 0 auto;
            padding: 0.6rem 0.8rem;
            overflow: hidden;
        }
        .activity-widget {
            display: grid;
            align-content: start;
            gap: 0.55rem;
            min-height: 0;
        }
        .activity-widget .stats {
            display: grid;
            grid-template-columns: repeat(4, minmax(0, 1fr)) auto;
            gap: 0.55rem;
            align-items: center;
        }
        .activity-widget .stat {
            border: 1px solid var(--line);
            border-radius: 0.7rem;
            padding: 0.45rem 0.6rem;
            background: var(--panel);
        }
        .activity-widget .stat span {
            display: block;
            color: var(--muted);
            font-size: 0.78em;
        }
        .activity-widget .stat strong {
            display: block;
            margin-top: 0.15rem;
            font-size: 1.55em;
            line-height: 1;
            letter-spacing: -0.03em;
        }
        .activity-widget .metric-control {
            display: grid;
            justify-items: center;
            align-content: center;
            gap: 0.35rem;
        }
        .activity-widget .metric-switch {
            display: inline-flex;
            gap: 0.3rem;
            padding: 0.25rem;
            border: 1px solid var(--line);
            border-radius: 999px;
            background: rgba(7, 17, 24, 0.56);
        }
        .activity-widget .metric-switch button {
            border: 0;
            border-radius: 999px;
            padding: 0.35rem 0.7rem;
            color: var(--muted);
            background: transparent;
            font: inherit;
            font-size: 0.86em;
            font-weight: 800;
            cursor: pointer;
        }
        .activity-widget .metric-switch button[aria-pressed="true"] {
            color: #061016;
            background: var(--level-5);
        }
        .activity-widget .metric-switch button:focus-visible {
            outline: 2px solid var(--level-5);
            outline-offset: 2px;
        }
        .activity-widget .pill {
            width: fit-content;
            border: 1px solid var(--line);
            border-radius: 999px;
            padding: 0.3rem 0.65rem;
            color: var(--muted);
            background: rgba(7, 17, 24, 0.56);
            font-size: 0.82em;
            white-space: nowrap;
        }
        .activity-widget .dashboard-grid,
        .activity-widget .left-stack {
            display: grid;
            gap: 0.55rem;
            min-width: 0;
            min-height: 0;
        }
        .activity-widget .panel {
            border: 1px solid var(--line);
            border-radius: 0.8rem;
            padding: 0.6rem 0.7rem;
            background: var(--panel-strong);
        }
        .activity-widget .chart-scroll {
            overflow-x: auto;
            border: 1px solid var(--line);
            border-radius: 0.8rem;
            padding: 0.6rem 0.7rem;
            background: var(--panel-strong);
        }
        .activity-widget .chart {
            min-width: calc(53 * (var(--cell) + var(--gap)) + 3.4rem);
        }
        .activity-widget .months {
            display: grid;
            grid-template-columns: repeat(53, var(--cell));
            gap: var(--gap);
            margin-left: 3.2rem;
            margin-bottom: 0.3rem;
            color: var(--soft);
            font-size: 0.78em;
        }
        .activity-widget .month-label { min-height: 1em; }
        .activity-widget .chart-body {
            display: flex;
            gap: 0.6rem;
        }
        .activity-widget .weekday-labels {
            display: grid;
            grid-template-rows: repeat(7, var(--cell));
            gap: var(--gap);
            width: 2.6rem;
            color: var(--soft);
            font-size: 0.76em;
            line-height: var(--cell);
            text-align: right;
        }
        .activity-widget .activity-grid {
            display: grid;
            grid-auto-flow: column;
            grid-template-rows: repeat(7, var(--cell));
            grid-auto-columns: var(--cell);
            gap: var(--gap);
        }
        .activity-widget .day-cell {
            width: var(--cell);
            height: var(--cell);
            border: 1px solid rgba(255, 255, 255, 0.075);
            border-radius: 2px;
            background: var(--level-0);
            cursor: default;
        }
        .activity-widget .activity-grid .day-cell:hover,
        .activity-widget .activity-grid .day-cell:focus-visible {
            border-color: rgba(232, 247, 251, 0.82);
            outline: none;
            transform: scale(1.28);
        }
        .activity-widget .day-cell[data-level="1"] { background: var(--level-1); }
        .activity-widget .day-cell[data-level="2"] { background: var(--level-2); }
        .activity-widget .day-cell[data-level="3"] { background: var(--level-3); }
        .activity-widget .day-cell[data-level="4"] { background: var(--level-4); }
        .activity-widget .day-cell[data-level="5"] {
            background: var(--level-5);
            box-shadow: 0 0 0.7rem rgba(190, 248, 106, 0.45);
        }
        .activity-widget .day-cell.pre-repo {
            border-color: rgba(255, 255, 255, 0.055);
            background: var(--level-0);
            opacity: 0.22;
        }
        .activity-widget .day-cell.outside { opacity: 0.22; }
        .activity-widget .legend {
            display: flex;
            align-items: center;
            justify-content: flex-end;
            gap: 0.35rem;
            margin-top: 0.5rem;
            color: var(--muted);
            font-size: 0.8em;
        }
        .activity-widget .legend .day-cell {
            display: inline-block;
            vertical-align: middle;
        }
        .activity-widget .panel h2 {
            margin: 0 0 0.45rem;
            color: var(--text);
            font-size: 1em;
            letter-spacing: -0.01em;
        }
        .activity-widget .go-breakdown {
            display: grid;
            grid-template-columns: repeat(4, minmax(0, 1fr));
            gap: 0.55rem;
        }
        .activity-widget .go-bucket {
            display: grid;
            align-content: start;
            gap: 0.3rem;
            border: 1px solid var(--line);
            border-radius: 0.6rem;
            padding: 0.45rem 0.5rem;
            background: var(--panel);
        }
        .activity-widget .go-bucket h3 {
            margin: 0;
            color: var(--level-5);
            font-size: 0.78em;
            font-weight: 800;
            letter-spacing: 0.08em;
            text-transform: uppercase;
        }
        .activity-widget .go-stats {
            display: grid;
            grid-template-columns: 1fr;
            gap: 0.15rem;
        }
        .activity-widget .go-panel .stat {
            display: flex;
            align-items: baseline;
            justify-content: space-between;
            gap: 0.6rem;
            border: 0;
            border-radius: 0;
            padding: 0.1rem 0;
            background: transparent;
        }
        .activity-widget .go-panel .stat span {
            display: inline;
            font-size: 0.8em;
            white-space: nowrap;
        }
        .activity-widget .go-panel .stat strong {
            margin-top: 0;
            font-size: 0.92em;
            letter-spacing: -0.01em;
        }
        .activity-tooltip {
            position: fixed;
            z-index: 50;
            min-width: 12rem;
            padding: 0.6rem 0.75rem;
            border: 1px solid var(--line);
            border-radius: 0.7rem;
            color: var(--text);
            background: rgba(4, 24, 31, 0.96);
            box-shadow: 0 1rem 2.4rem rgba(0, 0, 0, 0.5);
            pointer-events: none;
        }
        .activity-tooltip[hidden] { display: none; }
        .tooltip-date {
            display: block;
            font-size: 1em;
            font-weight: 800;
        }
        .tooltip-gap { height: 0.35rem; }
        .tooltip-value {
            display: block;
            color: var(--muted);
            font-size: 0.9em;
            line-height: 1.35;
        }
        </style>
`
