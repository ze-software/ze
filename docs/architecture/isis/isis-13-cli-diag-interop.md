# IS-IS CLI, Diagnostics and Web

The presentation and verification layer over the IS-IS engine. It originates no
protocol state: it renders the engine's read-only snapshots as `show isis ...`
commands and two web pages, adds two `clear isis ...` actions, asserts the
canonical metric set, and registers two config-sanity diagnostic codes.

| Concern | File |
|---------|------|
| Show and clear RPC proxies | `cmd_show.go` |
| Engine-side snapshot render and clear | `show.go` |
| Engine command dispatch | `register.go` |
| Config-sanity doctor check | `doctor.go` |
| Owned diagnostic code metadata | `codes.go` |
| SPF run log render | `spf/spflog.go` |
| Web handlers and page shell | `internal/component/web/handler_isis.go`, `page_isis.go` |

## Decision: show and clear are dispatcher proxies

Each command is registered as a central-namespace RPC (`ze-show:isis-*`,
`ze-clear:isis-*`) carrying a plugin-command declaration, so the engine can claim
the same command name without a builtin conflict. Each handler forwards through
`ForwardToPlugin`.

The engine's execute-command switch is the single authority that turns each fixed
command string into a snapshot. No protocol logic lives in the proxy.

<!-- source: internal/plugins/isis/cmd_show.go -- the show and clear RPC registrations and forwardToISIS -->
<!-- source: internal/plugins/isis/show.go -- the hostname, interface and SPF-log snapshots, clearAdjacencies, clearCounters -->

**Never re-dispatch.** The builtin RPC and the engine command share the command
string, so `Dispatch` re-matches the builtin and recurses to a stack overflow.
The file header comment says so; keep it.

## Decision: no per-component API YANG module

Both verbs bind into the **central** `ze-show:` and `ze-clear:` namespaces, so
the component ships only a command module with two separate augment-style roots,
a `show` container and a **separate** `clear` container, never nested. A
per-component API module is needed only when a component coins its own RPC
namespace.

## Decision: diagnostic codes live in the component

The two config-sanity codes are registered from the component rather than the
central code slice, with a comment in the central file recording that they are
deliberately absent there. Deleting the IS-IS component removes its codes.

The raw-socket code stays owned by the transport and is only surfaced here. One
code, one owner.

<!-- source: internal/plugins/isis/codes.go -- the two config-sanity code definitions -->
<!-- source: internal/plugins/isis/doctor.go -- the config-sanity check, a no-op when IS-IS is absent -->

## Decision: assert the metrics, do not register them

This layer owns no `ze_isis_*` series. Every series is registered by the
subsystem that produces it. A test wires a recording registry through every owner
and asserts the exact canonical name and label set, that none is a bare `isis_*`,
and that no unexpected `ze_isis_*` series leaks in. That is a two-way guard
against drift.

## Consequence: the show layer is removable with the component

Command tokens (the owner YANG), handlers, engine dispatch, diagnostic codes and
web views all live under the component. Two self-containment guards assert the
tokens never drift back into the central schema.

## Gap: the web routes are not mounted

The handlers and their SSE ticker exist and are unit-tested, including emit and
close on client disconnect, but nothing mounts `/isis` and `/isis/database` on
the running web server. The L2TP web surface has the identical shape and the same
gap. The handlers are implemented and tested; the route is not yet reachable.

<!-- source: internal/component/web/handler_isis.go -- ISISHandlers and the SSE ticker -->

## Trap: a functional test needs a passive interface

A passive interface advertises reachability and opens no raw circuit, so the
engine starts cleanly with no raw-socket capability and still originates its own
LSP. That is what makes `show isis database` non-empty in a functional test on a
host with no `AF_PACKET`. Live adjacency and SPF over the wire are the QEMU and
interop job.

## Trap: a one-shot command still waits for a readiness file

`ze doctor` and `ze explain` exit immediately, but the functional-test runner
treats every foreground `ze` invocation as a daemon and waits for a readiness
file the one-shot never writes. A file running several sequential explain and
doctor calls needs a generous timeout on the first command so they fit the
budget.
