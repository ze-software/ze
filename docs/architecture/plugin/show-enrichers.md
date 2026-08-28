# Show Enrichers: external plugins and the web

A show enricher adds data to a show command's output. The registry itself is
described in `docs/architecture/show-enricher.md`. This page covers the three
surfaces added on top of it.

The first registry served in-process plugins only. External plugins had no way
to contribute, and web service-locator pages that bypass CLI dispatch could not
use enrichment either.

<!-- source: internal/component/plugin/server/enricher.go -- proxy enricher registration, cleanup, makeProxyCall -->
<!-- source: internal/test/plugins/fakeenrich/fakeenrich.go -- in-process test enricher -->

## The decisions

**An external plugin declares its enrichers at Stage 1 registration, not after
ready.** This follows the doctor-check pattern and closes the race window where
a show command runs before the external enrichers exist.

**The external callback times out at 2 seconds.** A show enricher is expected to
be fast, and a hung plugin must not block the CLI.

**The proxy enricher lives in the server package, not in `core/show`.**
`core/show` is a leaf package on the standard library alone, and the proxy needs
IPC, which needs server imports.

**An external enricher's response merges at the top level with `maps.Copy`.**
This matches in-process enricher behavior, which mutates the base map in place.
A nested merge would make the two paths differ.

**`RegisterProcessCleanup` is called from a `sync.Once`, not from `init()`.**
The native write hook blocks a registration call inside `init()` in a
non-register file.
<!-- source: internal/le/hookruntime/writeedit.go -- writeGoPatterns -->

**Testing uses both a Go plugin and a compiled `.ci` fixture.** The Go
`fakeenrich` package guards the in-process path. The `ze-test fixture` process
exercises the external SDK path end to end. Either alone leaves one path
unproven.
<!-- source: internal/test/fixture/plugin_fixture_06.go -- fixture06EnricherExternalChecker -->

`Unregister(command, key)` removes one key at plugin exit, replacing the coarse
test-only reset.

## Constraints

**The proxy enricher captures the plugin connection in a closure.** When the
plugin exits, the connection is closed, the send returns an error, and the proxy
logs a warning. The show command continues with partial data, which is safe
because the enrich entry point recovers from panics.

**A test enricher plugin needs a command handler.** `fakeenrich` is a full
registry-registered plugin with `RunEngine` and `OnExecuteCommand`, so a `.ci`
can dispatch `show test enrich` through the standard command pipeline. An
enricher-only plugin is not dispatchable.

**A JSON round trip through the external path turns a `uint16` into a
`float64`.** An enricher that receives external data handles `float64` for every
numeric field.

A web service-locator page enriches by building a side map and calling the
enrich entry point explicitly.
