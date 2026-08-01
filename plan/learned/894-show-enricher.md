# 894 -- Show Enricher Registry

## Context

Show commands returned only the data their owning handler could build. Plugin-contributed data (e.g., CoS profile on a subscriber session) required either direct coupling between the handler and the plugin, or an out-of-band mechanism. The show enricher registry lets plugins register enrichment functions keyed by command name, which handlers call at show time to merge plugin-contributed data into the base output.

## Decisions

- Chose a core leaf registry in `internal/core/show/` over extending `command/registry/` because enrichment is broader than CLI (serves web, API too) and must be importable by any plugin from init() without cycles.
- Chose in-place `map[string]any` mutation over return-and-merge because it avoids copy overhead and lets enrichers read any base field without a separate args contract.
- Chose explicit `show.Enrich()` call in handlers over dispatcher middleware because the dispatcher is single-response and middleware would require JSON parse/merge at the wire layer.
- Chose separate Brief/Detail functions over a single function with a detail bool for clarity and direct mapping to summary vs detail show commands.
- Chose `Register` returning an error with `MustRegister` panicking variant over panic-only `Register` to satisfy hook enforcement (no direct panic with dynamic content).
- Chose alphabetical key ordering over registration order for deterministic output in `| json` and debugging, consistent with `registry.All()` sort convention.
- Chose panic recovery in `Enrich()` over propagation because a network OS must not crash a show command due to one enricher bug.

## Consequences

- Any plugin can now contribute data to any show command by registering an enricher in init(). No handler modification needed after the initial wiring.
- Web handlers using command dispatch get enriched output for free. Service-locator web handlers need explicit `show.Enrich()` calls.
- The registry imports only stdlib (`errors`, `log/slog`, `sort`, `sync`), preserving the leaf guarantee.
- Brief function is optional (nil means no brief enrichment), so detail-only enrichers (e.g., full RADIUS attribute dump) avoid no-op stubs.

## Gotchas

- Hook enforcement blocks `fmt.Sprintf` and string concatenation even in panic messages. Had to restructure from panic-only `Register` to error-returning `Register` + `MustRegister` pattern.
- The `internal/core/show/` package name overlaps with `internal/component/cmd/show/` (central show verb). No practical collision since they serve different consumers.
- CoS enricher accesses package-level `sessionStore` sync.Map directly (same package). If the enricher were in a different package, an exported accessor would be needed.

## Files

- `internal/core/show/show.go` -- enricher registry (Register, MustRegister, Enrich, EnrichBrief, ResetForTest)
- `internal/core/show/show_test.go` -- 10 registry unit tests
- `internal/component/l2tp/subscriber/cmd/subscriber.go` -- added show.Enrich/EnrichBrief calls in handlers
- `internal/component/l2tp/subscriber/cmd/subscriber_test.go` -- 2 wiring tests
- `internal/plugins/cos/enricher.go` -- CoS enricher functions
- `internal/plugins/cos/enricher_test.go` -- 4 enricher unit tests
- `internal/plugins/cos/register.go` -- added show.MustRegister calls in init()
- `test/plugin/show-subscriber-enricher-wiring.ci` -- functional test
- `ai/patterns/registration.md` -- added Show Enricher Registry section
- `ai/INDEX.md` -- added enricher keyword
- `docs/features.md` -- mentioned show enrichment in Subscriber Session Model
