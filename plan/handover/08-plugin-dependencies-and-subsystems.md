# Handover: plugin dependency edges and declared subsystems

Written 2026-10-20 from an owner review of four design questions. Nothing is
implemented and no spec exists yet. Each item below is an owner ruling that a
fresh session turns into a spec (one spec per session,
`.claude/rules/session-start.md`).

## Owner rulings

| # | Question | Ruling |
|---|----------|--------|
| 1 | May a dependency be soft, so that a missing provider refuses only the config that needs it (flow export `enrichment bgp` in a build without `ze_bgp`), or must every edge be hard? | Soft edges are allowed. A build without BGP still runs flow export, and refuses only `enrichment bgp` |
| 2 | Write the dependency spec now or in a fresh session? | Fresh session |
| 3 | The plugin server reads the interface config to choose a data-plane writer. Fix it with a registry function, or allow the import? | Registry function |
| 4 | Is "subsystem" a declared fact, so the doors' list comes from a command and not from an agent's reading? | Yes. BGP, interfaces, FIB, firewall and traffic are all subsystems |

## What the code holds today (read 2026-10-20 at `5695edfa7a`)

| Ruling | Current state | What the spec must settle |
|--------|---------------|---------------------------|
| 1 | `registry.Registration.OptionalDependencies` already exists: the resolver pulls an optional provider in when it is registered and skips it silently when it is not, and each owner writes its own fallback. Users: `bgp/plugins/rs`, `bgp/plugins/gr`, `plugins/static`, `component/traffic` | Extend this edge, do not add a third edge kind (`ai/rules/no-layering.md`). The missing half is the refusal: a config that needs an absent optional provider is refused with an error naming the build tag, instead of the owner silently producing empty data (`ai/rules/principles.md`, silently wrong values). Every current user of the edge needs a decision on which of its config needs the provider |
| 3 | `plugin/server/startup_autoload.go` imports `internal/component/iface` for one call, `iface.BackendNameFromTree`, to append the data-plane writer plugin. `ai/doors/plugin-infra.md` only forbids importing `bgp`, so no written rule is broken today | Replace the call with a registry function the interface component registers, then drop the import. Decide whether the door's boundary statement widens to "no component imports" once it holds |
| 4 | `pkg/ze.Subsystem` (Name, Start, Stop, Reload under the engine) is the existing declaration. Its implementers today: BGP (through `plugin/server/subsystem.go`), L2TP, PPPoE, SSH. Interfaces, FIB, firewall and traffic do not implement it | Per the ruling, interfaces, FIB, firewall and traffic become subsystems. Settle whether that means each implements `ze.Subsystem` and runs under the engine (a design change), or whether the interface's definition ("a daemon that owns external I/O") is rewritten. Either way there is one declaration, and the list in `ai/doors/` derives from a command over it, never a hand-written copy. L2TP, PPPoE and SSH stay on whatever list results |

## Next step

A fresh session runs `/ze-spec` for ruling 1 (soft dependency edges with
config refusal). Rulings 3 and 4 are each their own spec, written after it.
Ruling 3 is small and independent, and may go first if the owner prefers.
