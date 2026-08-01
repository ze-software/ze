# 799: Response Typed Data

## Context

`plugin.Response.Data` was `any`, accepting strings, maps, structs, and slices
without distinction. 12 handlers pre-serialized structured data to `string(json.Marshal(x))`
before assigning to Data, bypassing the pipe framework (| count, | resolve, | json).
Error messages and success payloads shared the same field, requiring runtime type
switching at marshaling boundaries.

## Decision

Replace `Data any` with `Data ResponseData` (marker interface) and add `Error string`.

Types satisfying ResponseData:
- `plugin.Map` (named `map[string]any`) for map payloads
- `plugin.DataMarker` struct embed for external package structs
- `plugin.Slice[T any]` generic named slice
- `plugin.RawJSON` for pre-serialized JSON crossing RPC boundaries

## Consequences

- Compiler rejects `Data: "bare string"` on success responses
- 12 pre-serialization bugs fixed: handlers return typed data directly
- Marshaling boundaries simplified: no string pass-through branch
- External structs used as Response.Data need DataMarker embed (one-time cost)
- `map[string]any` variables need `plugin.Map()` conversion at assignment
- `[]T` slices need `plugin.Slice[T]()` conversion

## Gotchas

- RPC bridge points (subsystem, plugin dispatch) receive data as strings from the wire.
  Use `plugin.RawJSON` with `MarshalJSON` to avoid double-encoding.
- `plugin.Slice[plugin.Map]` cannot be directly converted from `[]map[string]any`
  because Go doesn't allow slice conversion between named and unnamed element types.
  Build element-by-element or wrap in `plugin.Map{"key": slice}`.
- Warning-status responses (`Status: "warning"`) use Data for the message, not Error.
  Error field is only for StatusError responses.
- `ExecResult` in the API package is a separate type with `Data any` (not ResponseData).
  It lives at the API boundary and is populated from JSON unmarshaling.

## Files

None recorded.
