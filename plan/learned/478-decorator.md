# 478 -- YANG Decorator Framework

## Context

The web UI displays YANG leaf values as plain text with no contextual information. When viewing AS numbers in peer configuration, operators have to look up the organization name elsewhere. The goal was to create a general-purpose mechanism where YANG schema declarations drive display-time enrichment, starting with ASN-to-organization-name resolution via Team Cymru DNS.

## Decisions

- Chose YANG extension (`ze:decorate`) over hardcoded renderer logic, because it keeps enrichment declarations in the schema rather than scattered in Go code. Adding a new decorator to any leaf requires only a YANG change, no renderer modifications.
- Added `Decorate` field to `LeafNode` (config schema) over creating a separate metadata lookup, because the YANG-to-schema conversion already extracts extensions (sensitive, display-key, etc.) and this follows the same pattern.
- Used `DecoratorRegistry` on `Renderer` over a package-level global, because the renderer is already threaded through all handler functions and this avoids mutable global state.
- Designed decorator with `txtResolver` function parameter over direct `*dns.Resolver` dependency, to keep the web package decoupled from the DNS package and enable testing with fake resolvers.
- Chose graceful degradation (return empty string) over error propagation for DNS failures, because display enrichment is supplementary -- a failed lookup should never prevent rendering.

## Consequences

- Any YANG leaf can be decorated by adding `ze:decorate "name"` and registering a matching `Decorator` implementation. Future decorators (reverse DNS, community names, RPKI status) follow the same pattern.
- The `FieldMeta` struct now carries `DecoratorName` and `Decoration` fields. Templates can use `.Decoration` without any template function changes.
- Two resolution paths exist: `Renderer.RenderField()` (HTMX field swap) and `Renderer.ResolveDecorations()` (fragment rendering). Both must be called for decoration to appear.
- The `ze-extensions.yang` module now has 11 extensions. Any new YANG module can import it and use `ze:decorate`.

## Gotchas

- The `ze-extensions.yang` file already existed at `internal/component/config/yang/modules/` -- the spec originally suggested creating it at `internal/component/web/schema/`. Always check for existing files before creating new ones.
- Functional test config needs `local.ip` on peers -- `ze config validate` enforces this even for minimal test configs.
- The `fieldFor` template function (closure in `NewRenderer`) operates on `any`, not `FieldMeta`, so decoration must be resolved before template rendering, not inside the template function.
- `nilerr` linter catches functions that have `if err != nil { return ..., nil }` -- needs `//nolint:nilerr` with a comment explaining the intentional swallowing.

## Files

- `internal/component/config/yang/modules/ze-extensions.yang` -- added `ze:decorate` extension
- `internal/component/config/schema.go` -- added `Decorate` field to `LeafNode`
- `internal/component/config/yang_schema.go` -- added `getDecorateExtension()`, wired into `yangToLeaf()`
- `internal/component/config/yang_schema_test.go` -- `TestYANGSchemaDecorateExtension`
- `internal/component/bgp/schema/ze-bgp-conf.yang` -- added `ze:decorate "asn-name"` to ASN leaves
- `internal/component/web/decorator.go` -- `Decorator` interface, `DecoratorRegistry`, `DecoratorFunc`
- `internal/component/web/decorator_asn.go` -- ASN name decorator (Team Cymru DNS TXT)
- `internal/component/web/decorator_test.go` -- registry tests
- `internal/component/web/decorator_asn_test.go` -- ASN decorator tests (parsing, failure, boundary)
- `internal/component/web/fragment.go` -- `DecoratorName`/`Decoration` on `FieldMeta`, resolution call
- `internal/component/web/render.go` -- `SetDecorators()`, `ResolveDecorations()`, resolution in `RenderField()`
- `internal/component/web/render_test.go` -- `TestRenderDecoratedLeaf`, `TestRenderUnDecoratedLeaf`
- `internal/component/web/templates/input/wrapper.html` -- decoration span in field wrapper
- `test/parse/decorator-yang.ci` -- functional test for config parsing with ze:decorate
- `docs/features.md` -- YANG decorators feature entry
- `docs/comparison.md` -- ASN name enrichment mention
- `docs/architecture/web-components.md` -- Decorators section
