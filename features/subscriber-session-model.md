# Subscriber Session Model

## Meta

| Field | Value |
|-------|-------|
| Name | Subscriber Session Model |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/core/show, internal/component/plugin/server |
| Real-path tests | test/plugin/subscriber-summary-show.ci, test/plugin/subscriber-enricher-wiring-show.ci, test/l2tp/subscriber-reader-failing-socket.ci |
| Docs | docs/guide/l2tp.md, docs/guide/pppoe.md |
| Doc review | 2026-10-07: every source anchor in the Description resolves to its file and symbol; the 2s enrich-show timeout is enrichShowTimeout in internal/component/plugin/server/enricher.go |
| Defect review | 2026-10-07: plan/immediate/spec-pppoe-subscribers-produce-no-accounting-or-telemetry.md and plan/immediate/spec-subscriber-utilisation-has-no-operator-view.md open |
| Extra criteria | supported: PPPoE and L2TP subscribers report the same session fields = test/plugin/subscriber-pppoe-l2tp-parity.ci |

## Description

Unified subscriber session model for L2TP/PPPoE subscribers with shared session lifecycle, auth, pool, and shaper infrastructure. Show enricher registry (`internal/core/show/`) lets plugins contribute data to show commands: in-process plugins register via `show.MustRegister()` in init(); external plugins declare enrichers at registration (Stage 1) and handle `ze-plugin-callback:enrich-show` callbacks at runtime with a 2s timeout. Web service-locator pages call `show.Enrich()` explicitly. `show subscriber detail` and `show subscriber` gain CoS profile data when the cos plugin is loaded. <!-- source: internal/core/show/show.go -- Register, Enrich, EnrichBrief, Unregister -->
