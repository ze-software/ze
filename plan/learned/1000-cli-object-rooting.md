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

If Ze gains VRF / routing-instance support, add a `vrf <name>` keyword filter
(`show route vrf <name>`), not a Nokia-style `router` root. Instance and family
stay filters; the tree stays flat. See the namespacing doc.
