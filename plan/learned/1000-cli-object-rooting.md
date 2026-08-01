# cli-object-rooting

Operational CLI commands are **object-rooted**: every command roots at the object
it inspects, and the object is the owning component/plugin. There is no shared
`ip` namespace. Address family is a positional keyword filter, never a namespace
and never a `--flag`.

## What changed

| Before | After |
|---|---|
| `show ip route` / `show ip route lookup` | `show route` / `show route lookup` |
| `show ip arp` | `show neighbor` (+ `show neighbor ipv4`/`ipv6`); `show arp` = IPv4 alias |
| `show neighbors`, `show kernel-routes` (aliases) | removed (use `show neighbor` / `show route`) |
| `show ip ospf …` / `clear ip ospf …` | `show ospf …` / `clear ospf …` |

Rationale and the vendor analysis behind the choice (Cisco `ip`-rooted, Nokia
`router`-rooted, Juniper/Linux object-rooted): `docs/architecture/cli/command-namespacing.md`.
The "no `--flag` syntax in YANG; filters are keyword grammar" rule:
`ai/rules/cli-grammar.md` ("No Flag Syntax in YANG").

## How the family filter and the row cap are typed

- **Address family is validated by the YANG enum `ArgDef`, case-sensitively,
  before the handler runs.** The handler's own `strings.ToLower` normalisation was
  therefore dead code and was removed. A lowercase-only enum plus a pre-handler
  check is the whole contract; a handler must not re-normalise.
- **The row cap is a `limit N` keyword leaf**, matching the YANG `limit` `ArgDef`.
  It is not `--limit`, for the same reason the family filter is not `--family`
  (`ai/rules/cli-grammar.md`, "No Flag Syntax in YANG").

## Traps for the next agent

- **OSPF dispatches on the literal command-path string.** `PluginCommand`,
  `sdk.CommandDecl.Name`, the `OnExecuteCommand` switch cases, and the
  `dbSubviewType` map keys in `internal/plugins/ospf/` must all change in lockstep
  with the YANG path. The YANG `ze:command` wire method (`ze-show:ospf*`) does
  **not** change; only the path the dispatcher forwards does.
- **The `plugin/all` wire-method snapshot is golden-file based** (`testdata/*.snapshot`).
  After adding/renaming a command, regenerate with
  `go test -tags '<ze_core + ZE_FEATURES>' ./internal/component/plugin/all/ -update`
  (custom flag goes AFTER the package path).
- **Feature plugins (ospf/isis/ldp/rsvp-te/gnmi) need build tags.** Bare `go test`
  excludes them; use the `ZE_FEATURES` tag set or the registry-completeness tests
  report false "missing" wire methods.
- **Don't blanket-replace command strings in docs.** `command-catalogue.md` and
  `command-namespacing.md` contain other-vendor columns and Cisco/FRR examples
  that legitimately use `show ip route` / `show ip ospf`; edit only the Ze cells.

## Forward-looking

VRF / routing-instance support is designed in `plan/spec-vrf-0-umbrella.md`
(agreed with user) as an **instance-first prefix**: `show vrf <name> <object>`
(e.g. `show vrf surfprotect route`); the default VRF keeps the bare, unwrapped
form (`show route`). This is the Nokia-style instance-first ordering, NOT a
trailing `show route vrf <name>` filter. Architectural reason: each VRF is a full
replicated stack (own reactor/RIB/hub/listeners) behind a hub-of-hubs that
intercepts the leading `vrf <name>` token and forwards the remainder verbatim, so
the instance must be a prefix; the YANG of each child module is wrapped in `vrf
<name> { ... }`, so the config tree is instance-rooted too. **Family stays a
trailing filter** (`show vrf red ospf ipv6`); only the instance moves to the
front, and the keyword is `vrf`, not Nokia's `router`. See the namespacing doc
and spec-vrf-0.

**OSPFv3 / IPv6 (decided, not yet shipped):** Ze runs ONE unified OSPF engine
(v2+v3 via address-family strategy), so OSPFv3 show is a family selector on the
`ospf` object, not a separate `ospf3` object and not a `show ipv6 ospf`
namespace prefix: bare `show ospf <noun>` = IPv4 (OSPFv2), `show ospf ipv6
<noun>` = IPv6 (OSPFv3). The future spec-ospf-ext-* specs were updated to this
form (FRR-aware: FRR `ospfd` `show ip ospf ...` and FRR `ospf6d` `show ipv6
ospf6 ...` interop references are KEPT). Shipped OSPFv2 needs no change (it is
the bare/IPv4 default).

## Files

None recorded.
