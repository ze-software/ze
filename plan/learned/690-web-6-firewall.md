# 690 -- web-6-firewall

## Context

Ze manages nftables firewall rules through a YANG-modeled config with a `firewall` component (`internal/component/firewall/`) that owns Table, Chain, Term (rule), Set, and Flowtable types. The existing web UI showed firewall config only as a generic YANG tree browser. Operators needed purpose-built pages to view tables, drill into chains, inspect ordered rules with match/action summaries and live counters, manage sets, and see conntrack entries, all following the workbench table pattern from spec-web-3-foundation.

## Decisions

- **Five sub-pages behind a path-based dispatcher** (`renderFirewallPageContent`) over a single page with tabs. Tables, Chains, Rules, Sets, and Connections are separate URL paths under `/show/firewall/`, each building its own `WorkbenchTableData`. This matches the left nav structure and allows cross-page linking (Tables row "View Chains" links to chains filtered by table).
- **Read-only from `firewall.LastApplied()`** over re-parsing config tree. The applied state is an atomic immutable snapshot, safe for concurrent reads without locking. Display names strip the kernel `ze_` prefix via `StripZeTablePrefix()`.
- **Counter enrichment from `firewall.GetBackend().GetCounters()`** over a separate RPC. Counters are joined into rule entries at collect time, keyed by term name. If no backend is loaded, counters show "-".
- **Full match/action type-switch rendering** (`matchSummary`, `actionSummary`) over generic `fmt.Sprintf`. Each of the 15+ match types and 19+ action types has a human-readable rendering (e.g., `MatchSourceAddress` becomes `saddr 10.0.0.0/8`, `SNAT` becomes `snat 192.168.1.1`). This exhaustive switch ensures new match/action types produce a type name fallback (`%T`) rather than silent omission.
- **Query parameter filtering** (`?table=X&chain=Y`) over path-based scoping for drill-down. Chains page accepts `?table=`, `?hook=`, `?type=` filters. Rules page accepts `?table=` and `?chain=`. This allows cross-page links from table rows to pre-filtered chain/rule views.
- **Connections page as placeholder** for v1. Conntrack data requires runtime command dispatch, which is not yet wired for firewall. The page renders correct columns and an empty state that differentiates "no backend" from "no connections".
- **ConnState and SetFlags as bitmask-to-string helpers** (`connStateStr`, `setFlagsStr`) over storing pre-formatted strings in the model. Keeps the model types clean and rendering in the web layer.

## Consequences

- Firewall pages render immediately from applied state with no RPC round-trips (except counters, which are optional).
- The drill-down flow (Tables -> Chains -> Rules) uses query parameters, so the browser back button and bookmarking work naturally.
- Empty states are context-sensitive: "No rules in chain X" vs. "No rules configured" depending on whether a chain filter is active.
- Rule actions include Move Up/Down, Clone, Toggle (enable/disable), and Delete, all via `HxPost` with confirmation dialogs. These POST to config-editing URLs (not yet wired to handler implementations in v1).
- Adding new firewall match or action types to the Go model automatically produces a `%T` fallback in the web display, making missing renderers visible rather than silent.

## Gotchas

- `firewall.LastApplied()` returns the applied (committed) state, not the user's draft. Pending edits are not reflected in the tables page until committed. This is intentional for operational display but means the "Add Table" workflow goes through the config editor, not the display tables.
- Counter map is keyed by term name, not by chain+term. If two chains have terms with the same name, counters could cross-contaminate in display. In practice, `GetCounters` returns per-chain groupings, so the flattened map keys correctly in the current implementation since term names are unique within a table.
- The `valueOrDash` helper (used for optional chain fields like Type, Hook on non-base chains) is defined elsewhere; the firewall code depends on it without importing explicitly.
- Connections page has the column structure defined but no data source wired. It will need a show command or direct nftables conntrack query in a follow-up.

## Files

- `internal/component/web/page_firewall.go` -- all five firewall sub-pages: tables, chains, rules, sets, connections
- `internal/component/web/page_firewall_test.go` -- 45 tests covering table/chain/rule/set builders, match/action summaries, filtering, empty states
- `internal/component/web/workbench_sections.go` -- firewall section in left nav (pre-existing)
- `internal/component/web/workbench_pages.go` -- firewall page registration in dispatcher
