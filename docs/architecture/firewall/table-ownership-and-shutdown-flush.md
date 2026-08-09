# Firewall Table Ownership and Shutdown Flush

Four producers register nft tables with one shared registry: the firewall
engine, copp, policy-routes and ddos-local. All four run in process. This
document holds the three rules that keep their tables visible, reconciled and
removed, and the traps a test plugin met while proving the last one.

<!-- source: internal/component/firewall/registry.go -- ApplyAll, FlushAllTables -->
<!-- source: internal/component/firewall/backend.go -- LoadBackend, loadBackendLocked -->

## Rule 1: the backend loads on demand

A config with `control-plane-protection` and no `firewall {}` block never loaded
the nft backend, so `ApplyAll` returned "backend not loaded" and copp's apply
failed. `ApplyAll` now loads the OS-default backend when a producer registers
tables and no firewall section loaded one. `LoadBackend` was split so `ApplyAll`
can load atomically while it holds the backend mutex. The fix sits at the shared
altitude, so every registry consumer gets it.

## Rule 2: a producer's table name carries the `ze_` prefix

The nft backend uses `firewall.Table.Name` verbatim and adds nothing. It
reconciles and deletes only kernel tables whose name starts with `ze_`. A
producer that registers a bare name gets a table that `nft list table inet
ze_<name>` cannot see and that nothing ever withdraws.

| Producer | Name | Where it is set |
|----------|------|-----------------|
| firewall engine | `tableNamePrefix + name` | `ParseFirewallConfig` |
| policy-routes | `ze_pr` | hardcoded |
| copp | `ze_copp` | `coppTableName` |
| ddos-local | `ze_ddos-local` | `tableName` |

<!-- source: internal/plugins/copp/translate.go -- coppTableName -->
<!-- source: internal/plugins/ddos/local/responder.go -- tableName -->

copp and ddos-local both shipped bare names and both leaked their tables.

## Rule 3: teardown belongs to the firewall engine, gated by config

`ProcessManager.Stop` cancels every plugin at once with no dependency order, so
copp's own post-`Run` withdraw raced the firewall engine's `CloseBackend`. Once
the backend closed, the table could not be removed: a fresh backend refuses to
delete a `ze_` table that is not in its own applied set.

The `flush-on-shutdown` boolean leaf, default true, drives `FlushAllTables` from
the engine's post-`Run` path. One ordered actor holds a live backend and runs
before `CloseBackend`, so there is no race. copp's own withdraw was removed.
Removing the config while the daemon runs still withdraws through
`OnConfigApply`.

**The default behavior changed.** An orderly stop now removes every ze-owned nft
table. Set `firewall { flush-on-shutdown false; }` to use ze as a one-shot
provisioner that programs rules and exits. This is process lifecycle only and is
unrelated to BGP graceful restart, which never restarts the daemon.

**Keep-on-crash is automatic.** The teardown runs on the orderly-stop path only.
SIGKILL, a panic and a power loss all bypass it, so the tables persist.

The firewall engine parses `flush-on-shutdown` only when a `firewall` section is
present. A copp-only config keeps the package default of true. `atomic.Bool`
zeroes to false, so the fail-safe default needs a `Store(true)` at init.

## The injector plugin, and why it exists

`fakeddos` emits a synthetic `AttackDetected`, waits, then emits
`AttackCleared`. It is the first and only test of the while-running withdraw
path: `removeMitigation` to `RegisterTables(nil)` to `ApplyAll` delete. The copp
withdraw test exercises the shutdown flush path only, so the responder's
clear-while-running withdraw had never run against a real backend.
<!-- source: internal/test/plugins/fakeddos/fakeddos.go -- synthetic attack injector -->

**`events.Event.Emit` counts out-of-process subscribers only.** An in-process
subscriber is delivered to synchronously and is not counted, so `Emit` returns 0
after it has just run the handler. `fakeddos` first looped "emit until the count
is above zero", which never terminates. Never gate on the return count for
in-process delivery. `fakeddos` re-emits idempotently until the driver's trigger
file appears, and the driver creates that file only after it has seen the table.

**A test plugin cannot take an "unused" signal as a control channel.** Unhandled
means Go's default disposition applies, and for SIGUSR2 that terminates the
process. SIGUSR1 is already the reactor status dump, and the hub catches INT,
TERM and HUP only. Use a filesystem handshake: the daemon's working directory is
the per-test tmpfs directory, which is the same directory the driver reads
`daemon.pid` from, so a trigger file needs no path coordination.

**nft 1.1.1 renders a MASKED source-address match raw.** `ip saddr
192.0.2.0/24` becomes `@nh,96,32 & 0xffffff00 == 0xc0000200`. Only an unmasked
literal renders cooked. A version-robust `.ci` assertion matches either form.
