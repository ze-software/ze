---
name: No pointers across plugin/component boundaries
description: Event payloads and cross-component data MUST be self-contained value types. Never pass a pointer to data owned by another plugin or component, even via a shared package.
type: feedback
originSessionId: d98c3dc1-a63b-444a-9194-5e817e886103
---
BLOCKING rule: payloads that cross plugin or component boundaries MUST be
self-contained value types. No pointer fields pointing to data owned by
another plugin or component, even when the target lives in a shared core
package.

**Why:** user said explicitly "NO POINTER. YOU ARE NOT ALLOWED TO POKE
INTO OTHER PLUGIN OR COMPONENT CODE. IF I WANTED GOTO CODE I WOULD WRITE
GOTO." Pointers across boundaries act like a goto: the receiver gains
implicit access to every field of a struct it doesn't own, creating
non-local coupling that isn't visible at the call site. The user designs
the software; plugin isolation is a hard boundary, not a soft preference.

**How to apply:**
- Event payloads (`ze.EventBus` Emit): use value types only (numeric IDs,
  `family.Family`, `netip.Prefix`, `netip.Addr`, enum uint8).
- Cross-plugin identifiers: use registered numeric IDs, not pointers
  into a shared registry. If the consumer needs a human-readable name,
  it calls a registry lookup locally -- it does not dereference a
  pointer to someone else's struct.
- Cross-component types in IPC/events: same rule. `*foopkg.Something`
  as a payload field is forbidden even when `foopkg` is "shared core".
- **Registration patterns are ALSO subject to this rule.** When a plugin
  registers "I produce X" in a shared registry, the registry MUST store
  value types only (IDs, immutable string copies, bits), NOT pointers
  to handles or structs the producer allocated. Consumers enumerate the
  registry and build their OWN typed handles locally (e.g. via
  `events.Register[T]`) based on the value-typed metadata. Handle
  identity is by `(namespace, eventType, T)` tuple, not by pointer.
- Shared core packages (`internal/core/*`) are still fine for code
  reuse (functions, constants, value-typed records). What is forbidden
  is carrying a pointer to their state OR to producer-allocated data
  through an event, RPC, or registry surface that another plugin reads.
- **Shared type definitions are the contract, not cross-plugin memory
  access.** Two plugins both importing a type (`family.Family`,
  `RouteChangeBatch`) share a compile-time type identity; that is
  fine. What is forbidden is one plugin HOLDING A POINTER TO DATA
  ANOTHER PLUGIN ALLOCATED.

**Applies to:** every `Emit`/`Subscribe` payload, every plugin RPC
payload, every shared struct that crosses a component seam, every
registry surface that another plugin or component reads.

**Rejected alternative I tried (first pass):** proposed
`Source *redistribute.RouteSource` as an event payload field with the
justification "stable pointer from a shared registry." User rejected
this as a cross-boundary pointer. Switched to numeric `ProtocolID`.

**Rejected alternative I tried (second pass):** proposed a registry
returning `map[ProtocolID]*Event[*RouteChangeBatch]` so consumers
could iterate and subscribe directly on producer-allocated handles.
User rejected this too -- the registration pattern made producer
handle pointers cross-plugin accessible. Switched to a registry that
stores only `(id, name, has-producer-bit)` value tuples; consumers
build their own local handles via `events.Register[T]` which is
idempotent on the `(namespace, eventType, T)` contract.
