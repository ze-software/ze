// Design: the activity page's own stylesheet, recovered from the published page
// Related: activity.go renders the page these rules lay out.
package site

// activityStyle is the heatmap page's own stylesheet. It is inline because no
// shared rule in site.css lays out a calendar grid, a metric switch or a hover
// tooltip, and this is the one page that draws them.
//
// It is a raw string because it holds quotation marks and no backtick, so a
// reader comparing it against the page it ships to meets the same characters
// in both.
const activityStyle = `        <style>

        .activity-page {
            position: relative;
            display: grid;
            gap: clamp(0.8rem, 1.8vw, 1.2rem);
        }
        .activity-page::before {
            content: "";
            position: absolute;
            inset: -2rem -1rem auto auto;
            z-index: -1;
            width: min(34rem, 70vw);
            height: min(34rem, 70vw);
            border-radius: 999px;
            background:
                radial-gradient(circle at 45% 40%, rgba(34, 255, 155, 0.24), transparent 48%),
                radial-gradient(circle at 75% 72%, rgba(96, 165, 250, 0.22), transparent 52%);
            filter: blur(8px);
        }
        .activity-hero {
            position: relative;
            overflow: hidden;
            border: 2px solid rgba(255, 255, 255, 0.92);
            border-radius: clamp(1.1rem, 2.5vw, 1.55rem);
            padding: clamp(1.15rem, 2.4vw, 1.65rem);
            background:
                radial-gradient(circle at 86% 38%, rgba(157, 100, 245, 0.18), transparent 14rem),
                radial-gradient(circle at 20% 18%, rgba(26, 188, 156, 0.14), transparent 16rem),
                linear-gradient(135deg, rgba(255, 254, 254, 0.96), rgba(233, 245, 255, 0.84));
            box-shadow: var(--clay), 0 1.7rem 4rem -2.2rem var(--shadow);
            color: var(--text);
            margin-bottom: 0;
        }
        .activity-hero::before {
            content: "";
            position: absolute;
            inset: 0;
            background:
                linear-gradient(rgba(190, 120, 210, 0.08) 1px, transparent 1px),
                linear-gradient(90deg, rgba(190, 120, 210, 0.08) 1px, transparent 1px);
            background-size: 22px 22px;
            mask-image: linear-gradient(115deg, #000, transparent 72%);
            pointer-events: none;
        }
        .activity-hero::after {
            content: "";
            position: absolute;
            right: clamp(0.8rem, 2vw, 1.2rem);
            bottom: clamp(0.8rem, 2vw, 1.2rem);
            width: min(12rem, 28vw);
            height: min(12rem, 28vw);
            border: 1px solid rgba(157, 100, 245, 0.24);
            border-radius: 999px;
            background:
                radial-gradient(circle, transparent 55%, rgba(157, 100, 245, 0.16) 56%, transparent 57%),
                radial-gradient(circle, rgba(0, 159, 227, 0.12), transparent 68%);
            transform: translate(20%, 24%);
            pointer-events: none;
        }
        .activity-eyebrow {
            position: absolute;
            top: clamp(1rem, 2vw, 1.25rem);
            right: clamp(1rem, 2.4vw, 1.5rem);
            z-index: 1;
            display: inline-flex;
            align-items: center;
            gap: 0.45rem;
            margin-bottom: 0;
            color: var(--grape-deep);
            font-size: 0.7rem;
            font-weight: 900;
            letter-spacing: 0.16em;
            text-transform: uppercase;
        }
        .activity-eyebrow::before {
            content: "";
            width: 0.5rem;
            height: 0.5rem;
            border-radius: 999px;
            background: var(--mint-base);
            box-shadow: 0 0 1.1rem var(--mint-glow);
        }
        .activity-hero h1 {
            position: relative;
            max-width: 18ch;
            padding-right: 0;
            margin: 0;
            color: var(--text);
            background: linear-gradient(90deg, var(--grape-deep), var(--sky-deep) 54%, var(--teal-deep));
            -webkit-background-clip: text;
            background-clip: text;
            -webkit-text-fill-color: transparent;
            font-size: clamp(2.25rem, 4.8vw, 4.15rem);
            line-height: 0.92;
            letter-spacing: -0.065em;
        }
        .activity-hero p {
            position: relative;
            max-width: 45rem;
            margin: 0.65rem 0 0;
            color: var(--muted);
            font-size: clamp(0.94rem, 1.4vw, 1.08rem);
            line-height: 1.5;
        }
        .activity-widget {
            --activity-ink: #09111d;
            --activity-card: rgba(255, 255, 255, 0.74);
            --activity-card-strong: rgba(255, 255, 255, 0.9);
            --activity-border: rgba(255, 255, 255, 0.94);
            display: grid;
            gap: 1rem;
        }
        .activity-widget .stats {
            display: grid;
            grid-template-columns: repeat(4, minmax(0, 1fr)) minmax(11rem, auto);
            gap: 0.85rem;
            align-items: stretch;
        }
        .activity-widget .stat {
            position: relative;
            overflow: hidden;
            border: 2px solid var(--activity-border);
            border-radius: 1.05rem;
            padding: 0.85rem;
            background:
                linear-gradient(145deg, rgba(255, 255, 255, 0.78), rgba(255, 255, 255, 0.52)),
                var(--sky-tint);
            box-shadow: var(--clay);
        }
        .activity-widget .stat::after {
            content: "";
            position: absolute;
            inset: auto 0 0;
            height: 0.22rem;
            background: linear-gradient(90deg, var(--mint-base), var(--sky-base), var(--grape-base));
            opacity: 0.75;
        }
        .activity-widget .stat span {
            display: block;
            color: var(--muted);
            font-size: 0.78rem;
            font-weight: 800;
            letter-spacing: 0.02em;
            text-transform: uppercase;
        }
        .activity-widget .stat strong {
            display: block;
            margin-top: 0.22rem;
            color: var(--activity-ink);
            font-size: clamp(1.2rem, 2.1vw, 1.7rem);
            font-weight: 900;
            letter-spacing: -0.04em;
        }
        .activity-widget .metric-control {
            display: grid;
            justify-items: end;
            align-content: center;
            gap: 0.45rem;
            min-width: 10.5rem;
        }
        .activity-widget .metric-switch {
            display: inline-flex;
            gap: 0.3rem;
            padding: 0.32rem;
            border: 2px solid var(--activity-border);
            border-radius: 999px;
            background: var(--activity-card-strong);
            box-shadow: var(--clay);
        }
        .activity-widget .metric-switch button {
            border: 0;
            border-radius: 999px;
            padding: 0.45rem 0.7rem;
            color: var(--muted);
            background: transparent;
            font: inherit;
            font-size: 0.74rem;
            font-weight: 900;
            letter-spacing: 0.03em;
            text-transform: uppercase;
            cursor: pointer;
        }
        .activity-widget .metric-switch button[aria-pressed="true"] {
            color: #07111d;
            background: linear-gradient(135deg, #9fffd0, #7dd3fc);
            box-shadow: inset 0 -1px 0 rgba(0, 0, 0, 0.14);
        }
        .activity-widget .pill {
            width: fit-content;
            border: 1px solid rgba(15, 23, 42, 0.08);
            border-radius: 999px;
            padding: 0.4rem 0.65rem;
            color: var(--muted);
            background: rgba(255, 255, 255, 0.62);
            font-size: 0.74rem;
            font-weight: 800;
            white-space: nowrap;
        }
        .activity-widget .dashboard-grid,
        .activity-widget .left-stack {
            display: grid;
            gap: 1rem;
        }
        .activity-widget .panel {
            border: 2px solid var(--activity-border);
            border-radius: 1.45rem;
            padding: clamp(1rem, 2vw, 1.25rem);
            background:
                linear-gradient(160deg, rgba(255, 255, 255, 0.86), rgba(255, 255, 255, 0.68)),
                var(--panel-strong);
            box-shadow: var(--clay);
        }
        .activity-widget .chart-scroll {
            overflow-x: auto;
            padding: clamp(0.8rem, 1.4vw, 1rem);
            border: 2px solid rgba(255, 255, 255, 0.94);
            border-radius: 1.15rem;
            background:
                radial-gradient(circle at 9% 16%, rgba(26, 188, 156, 0.15), transparent 18rem),
                linear-gradient(145deg, rgba(255, 254, 254, 0.94), rgba(233, 245, 255, 0.82));
            box-shadow: var(--clay);
        }
        .activity-widget .chart-scroll::-webkit-scrollbar {
            height: 0.7rem;
        }
        .activity-widget .chart-scroll::-webkit-scrollbar-track {
            background: rgba(190, 120, 210, 0.16);
            border-radius: 999px;
        }
        .activity-widget .chart-scroll::-webkit-scrollbar-thumb {
            background: rgba(157, 100, 245, 0.5);
            border-radius: 999px;
        }
        .activity-widget .chart {
            min-width: 735px;
        }
        .activity-widget .months {
            display: grid;
            grid-template-columns: repeat(53, 11px);
            gap: 2px;
            margin-left: 3rem;
            margin-bottom: 0.42rem;
            color: var(--muted);
            font-size: 0.72rem;
            font-weight: 800;
        }
        .activity-widget .month-label {
            min-height: 1rem;
        }
        .activity-widget .chart-body {
            display: flex;
            gap: 0.55rem;
        }
        .activity-widget .weekday-labels {
            display: grid;
            grid-template-rows: repeat(7, 11px);
            gap: 2px;
            width: 2.45rem;
            color: var(--muted);
            font-size: 0.66rem;
            font-weight: 800;
            line-height: 11px;
            text-align: right;
            text-transform: uppercase;
        }
        .activity-widget .activity-grid {
            display: grid;
            grid-auto-flow: column;
            grid-template-rows: repeat(7, 11px);
            grid-auto-columns: 11px;
            gap: 2px;
        }
        .activity-widget .day-cell {
            width: 11px;
            height: 11px;
            border: 1px solid rgba(255, 255, 255, 0.95);
            border-radius: 3px;
            background: var(--grape-tint);
            cursor: default;
            transition: transform 150ms ease, border-color 150ms ease, box-shadow 150ms ease;
        }
        .activity-widget .activity-grid .day-cell:hover,
        .activity-widget .activity-grid .day-cell:focus-visible {
            position: relative;
            z-index: 2;
            border-color: rgba(255, 255, 255, 1);
            outline: none;
            box-shadow: 0 0 0 2px var(--grape-glow), var(--clay);
            transform: scale(1.22);
        }
        .activity-widget .day-cell[data-level="1"] { background: var(--mint-chip); }
        .activity-widget .day-cell[data-level="2"] { background: var(--mint-base); }
        .activity-widget .day-cell[data-level="3"] { background: var(--teal-base); }
        .activity-widget .day-cell[data-level="4"] { background: var(--sky-base); }
        .activity-widget .day-cell[data-level="5"] {
            background: var(--grape-base);
            box-shadow: 0 0 0.65rem var(--grape-glow);
        }
        .activity-widget .day-cell.pre-repo {
            border-color: transparent;
            background: transparent;
            opacity: 0.28;
        }
        .activity-widget .day-cell.outside { opacity: 0.22; }
        .activity-widget .legend {
            display: flex;
            align-items: center;
            justify-content: flex-end;
            gap: 0.45rem;
            margin-top: 0.85rem;
            color: var(--muted);
            font-size: 0.78rem;
            font-weight: 800;
        }
        .activity-widget .legend .day-cell {
            display: inline-block;
            vertical-align: middle;
        }
        .activity-widget .panel h2 {
            margin: 0 0 0.75rem;
            color: var(--activity-ink);
            font-size: 1.05rem;
            letter-spacing: -0.02em;
        }
        .activity-widget .go-panel h2::before {
            content: "";
            display: inline-block;
            width: 0.62rem;
            height: 0.62rem;
            margin-right: 0.45rem;
            border-radius: 999px;
            background: var(--grape-base);
            box-shadow: 0 0 0 0.28rem rgba(160, 110, 220, 0.16);
        }
        .activity-widget .go-breakdown {
            display: grid;
            grid-template-columns: repeat(4, minmax(0, 1fr));
            gap: 0.8rem;
        }
        .activity-widget .go-bucket {
            display: grid;
            gap: 0.45rem;
            border: 1px solid rgba(15, 23, 42, 0.07);
            border-radius: 0.95rem;
            padding: 0.65rem;
            background: rgba(255, 255, 255, 0.58);
        }
        .activity-widget .go-bucket h3 {
            margin: 0;
            color: var(--grape-deep);
            font-size: 0.78rem;
            font-weight: 900;
            letter-spacing: 0.08em;
            text-transform: uppercase;
        }
        .activity-widget .go-stats {
            display: grid;
            grid-template-columns: 1fr;
            gap: 0.22rem;
        }
        .activity-widget .go-panel .stat {
            display: flex;
            align-items: baseline;
            justify-content: space-between;
            gap: 0.7rem;
            min-height: 0;
            padding: 0.4rem 0.48rem 0.48rem;
            background: rgba(255, 255, 255, 0.62);
            box-shadow: none;
        }
        .activity-widget .go-panel .stat strong {
            margin-top: 0;
            font-size: 0.86rem;
            letter-spacing: -0.02em;
        }
        .activity-widget .go-panel .stat span {
            font-size: 0.62rem;
            white-space: nowrap;
        }
        .activity-widget table {
            width: 100%;
            border-collapse: collapse;
        }
        .activity-widget table[hidden] { display: none; }
        .activity-widget th,
        .activity-widget td {
            padding: 0.52rem 0;
            border-bottom: 1px solid rgba(15, 23, 42, 0.08);
            text-align: left;
            font-size: 0.83rem;
        }
        .activity-widget th {
            color: var(--muted);
            font-size: 0.68rem;
            font-weight: 900;
            text-transform: uppercase;
            letter-spacing: 0.1em;
        }
        .activity-widget td:last-child,
        .activity-widget th:last-child {
            color: var(--activity-ink);
            font-weight: 900;
            text-align: right;
        }
        .activity-tooltip {
            position: fixed;
            z-index: 50;
            min-width: 15rem;
            padding: 0.9rem 1rem;
            border: 1px solid rgba(255, 255, 255, 0.82);
            border-radius: 1rem;
            color: #fff;
            background: rgba(4, 24, 31, 0.94);
            box-shadow: var(--clay), 0 1.4rem 3rem -0.6rem rgba(35, 245, 156, 0.45);
            pointer-events: none;
            backdrop-filter: blur(12px);
        }
        .activity-tooltip[hidden] { display: none; }
        .tooltip-date {
            display: block;
            font-size: 1.05rem;
            font-weight: 900;
        }
        .tooltip-gap { height: 0.6rem; }
        .tooltip-value {
            display: block;
            color: rgba(255, 255, 255, 0.78);
            font-size: 0.92rem;
            line-height: 1.4;
        }
        @media (max-width: 980px) {
            .activity-widget .stats { grid-template-columns: repeat(2, minmax(0, 1fr)); }
            .activity-widget .metric-control {
                grid-column: 1 / -1;
                justify-items: start;
            }
            .activity-widget .go-breakdown { grid-template-columns: repeat(2, minmax(0, 1fr)); }
        }
        @media (max-width: 620px) {
            .activity-eyebrow {
                position: relative;
                top: auto;
                right: auto;
                margin-bottom: 0.5rem;
            }
            .activity-hero h1 {
                padding-right: 0;
                font-size: clamp(2rem, 12vw, 3.2rem);
            }
            .activity-widget .stats,
            .activity-widget .go-breakdown { grid-template-columns: 1fr; }
            .activity-widget .metric-control { justify-items: stretch; }
            .activity-widget .metric-switch { width: fit-content; }
        }

        </style>
`
