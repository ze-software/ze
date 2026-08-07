---
kind: table
level:
stage:
---
| Element | Canonical | Anti-pattern |
|---------|-----------|--------------|
| Module name | `ze-<component>[-<kind>]`, matches the filename | `exabgp` (unprefixed; external-compat only) |
| Namespace | `urn:ze:<component>:<kind>`, where `<kind>` (`conf`/`cmd`/`api`) is ALWAYS a final colon segment | `urn:ze:ddos-detect-conf` (kind baked with `-`), `urn:ze:role` (no kind segment) |
| Prefix | short, lowercase, **unquoted**, no hyphens, derived from the module | `prefix "bgp-mon-api";` (quoted, hyphens, abbreviated), `prefix updateshowcmd;` |
| `revision` | at least one `revision YYYY-MM-DD { description ...; }` | no revision statement |
| `description` | module-level `description` required | omitted |
| `organization` / `contact` | omit (not a project convention; present in only one legacy batch) | adding `organization` to new modules |
