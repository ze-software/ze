# Learned: Domain Request Types at the API Transport Boundary

**Spec:** spec-arch-1-grpc-types
**Date:** 2026-05-17
**Outcome:** Completed. All engine and config session methods now take typed request structs.

## What Was Done

Introduced a typed domain request layer between gRPC/REST transports and the API engine.
Both transports construct the same `*api.<Method>Request` struct before calling the engine.
Proto types (`zepb.*`) remain confined to `grpc/`. The shared `api.BuildCommand` helper
replaced duplicated param-flattening logic in both transports.

## Key Decisions

| Decision | Rationale |
|----------|-----------|
| All engine methods take `*Request` structs | Uniformity, three-line handlers, positional safety |
| `<Method>Request` naming | Package qualifier disambiguates from proto (`zepb.CommandRequest` vs `api.ExecuteRequest`) |
| Pre-flattened `Command string` in ExecuteRequest | Dispatcher is string-based; no structured dispatch planned |
| Full convert helpers in both transports | Structural symmetry; conversion logic independently testable |
| CallerIdentity embedded in request | One arg per method; keeps handlers thin |

## What Worked

- The migration was purely mechanical once request types were defined. Every caller
  got a clear compilation error pointing to the exact line needing update.
- The `ConfigSetRequest` struct immediately makes parameter-order bugs impossible
  (previously `Set(username, sessionID, path, value)` could be silently swapped).
- Moving `buildCommand`/`execResultToProto`/`commandMetaToProto` to `convert.go`
  made the server.go handler code shorter and more readable.

## What to Watch

- Convenience handlers in REST (handleConvenience, handlePeerByName, etc.) still
  construct `&api.ExecuteRequest{...}` inline because they have no params to
  process. This is intentional (the "no identity wrappers" rule): a fromREST helper
  that just copies a string into a struct field adds no value.
- The `fmt.Sprint(val)` in `rest/convert.go` for `map[string]any` param conversion
  uses a type assertion fast path for string values (common case from JSON) to avoid
  the allocation.

## Patterns for Future Use

When adding a new RPC:
1. Add request struct to `requests.go`
2. Add engine method taking the struct
3. Add `fromProto*` in `grpc/convert.go`
4. Add `fromREST*` in `rest/convert.go`
5. Handler becomes: validate input, convert, call engine, convert response

## Files

None recorded.
