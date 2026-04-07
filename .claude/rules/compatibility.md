# No Backwards Compatibility

Rationale: `.claude/rationale/compatibility.md`

## Pre-release (current state)

Ze has never been released. No users. No compat code, comments, shims, or fallbacks anywhere — including the plugin API. If something needs to change, just change it.

## Post-release (future state)

Code under `internal/` is not user-exposed. It follows the no-backwards-compatibility rule forever: change it freely, no shims, no deprecation layers, no "keep the old name working".

**The only exception is the plugin API** — the surface that external plugin authors compile against (`internal/plugin/` SDK types, the JSON event / text command protocol between core and plugins, and anything re-exported for plugin consumption). Once released, that surface MUST NOT break. Everything else under `internal/` remains free to change.

To be clear: the plugin API's *implementation* can change freely. Only its *contract* (signatures, protocol shape, documented semantics) is frozen post-release.

## ExaBGP compat

External tools only (`ze exabgp plugin`, `ze config migrate`). Engine code: zero ExaBGP format awareness.
