# Show Enricher Registry

A show handler builds only the data its own component owns. The enricher
registry lets any plugin add fields to any show command, keyed by command name,
with no coupling between the handler and the plugin.

<!-- source: internal/core/show/show.go -- Register, MustRegister, Enrich, EnrichBrief -->
<!-- source: internal/plugins/cos/enricher.go -- CoS enricher for subscriber show commands -->

## Why a core leaf package

The registry is in `internal/core/show/`, not in `command/registry/`.
Enrichment serves the CLI, the web UI and the API, and any plugin must be able
to register from `init()` with no import cycle. The package imports the
standard library only (`errors`, `log/slog`, `sort`, `sync`), which is what
keeps that guarantee.

Note the name overlap with `internal/component/cmd/show`, the central show
verb. They serve different consumers and never meet.

## Contract

| Decision | Reason |
|----------|--------|
| The enricher mutates a `map[string]any` in place | no copy, and the enricher can read any base field with no separate argument contract |
| The handler calls `show.Enrich()` explicitly | dispatcher middleware would have to parse and merge JSON at the wire layer, and the dispatcher is single-response |
| Brief and Detail are separate functions | they map to the summary and the detail command; a detail-only enricher leaves Brief nil instead of writing a no-op |
| Keys are emitted in alphabetical order | deterministic output for `\| json` and for debugging, matching the `registry.All()` convention |
| `Enrich()` recovers from a panic | a network OS must not lose a show command because one enricher has a defect |
| `Register` returns an error, `MustRegister` panics | the native write hook forbids a direct panic carrying dynamic content, so the panic lives in one named wrapper |
<!-- source: internal/le/hookruntime/writeedit.go -- writeGoPatterns -->

A web handler that dispatches a command gets the enriched output already. A web
handler that uses the service locator calls `show.Enrich()` itself.
