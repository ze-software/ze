# Learned: a plugin's declaration belongs on its registration

`spec-daemon-backed-command-catalog` set out to make the published command
catalog report what a plugin declares. The name says "daemon-backed" and the
answer needs no daemon: a plugin's `commandDecls()` slice now rides on the
`registry.Registration` its `init()` already builds, and every reader that links
the composition root reads it from `registry.All()`.

Four things cost real effort to establish. Each one outlives the spec.

## 1. Running a plugin engine to read its declaration is not viable

A collector that speaks the server half of the Stage 1 handshake was designed
first. It is the obvious answer: a plugin's declaration exists only after its
runner sends it, and an in-process plugin sends it over a `net.Pipe`, so no
socket, no config and no privileges are needed. It was rejected on evidence.

**All 97 registered runners were read, entry to `p.Run`, on 2026-09-07.**
Reading a declaration that way means first running everything the engine does
before it declares.

| Finding | Instances |
|---------|-----------|
| MUTATES THE HOST before Stage 1 | `flowspec-firewall.runEngine` (`internal/plugins/flowspec-firewall/engine.go`) calls `firewall.ApplyAll()` under `firewall.LegacySweepPending()`, which `init()` stores TRUE while `legacyTables` is non-empty (`internal/component/firewall/legacy_tables.go`) and which is the documented exemption that makes `ApplyAll` autoload the OS backend for an EMPTY desired set: on Linux that is nftables syscalls. `ike.runEngine` (`internal/component/ike/engine/register.go`) calls `installIKEBypass` unconditionally, writing four node-wide XFRM policies; its `defer` removes them, which beside a LIVE ike engine removes that daemon's bypass too |
| Mutates the collector PROCESS before Stage 1 | `trafficusage.runEngine` calls `rlimit.RemoveMemlock()` (a `setrlimit`) before its own gate. `fib/kernel.runFIBKernelPlugin` opens a netlink handle whose `close()` sits after the abort's `return 1`, so one socket leaks per collection. `connected`, `rib` and `static` each fire `locrib.Default()`, allocating the process-wide default Loc-RIB |
| Never reaches Stage 1 | Three return without calling `p.Run`: `capa`, `loop` (`internal/component/bgp/reactor/filter/register.go`), `srpolicy`. Four return 1 at `if !p.IsInternal()`, which a bare `net.Pipe` always fails: `as112`, `flowexport`, `vrrp`, `trafficusage` |
| Leaves a process global pointing at a dead plugin | Eight: `adj_rib_in` (`rpc.RegisterBatchValidator`), `rib` (`rpc.RegisterRouteInjector`, no defer), `redistribute_egress` (replay coordinator, no defer), `sysrib` (`SetLocRIB`, `publishDistances`, no defer), `isis` and `ospf` (`routeInstallPtr`, no defer), `iface` (three triggers plus a `SubscribeCollectNotify` that APPENDS), and `ospf` again for the opaque-type registry, whose second registration returns `ErrOpaqueTypeRegistered` and is only logged |
| Cleanup defeated by the abort | `iface` stops four goroutines in straight-line code AFTER `p.Run`, so a Stage 1 abort leaks all four. `sysrib` leaves `distance.Of` and `igpcost.Lookup` answering from a dead plugin for the process life |
| Installs a SIGINT/SIGTERM handler | 80-plus, via `sdk.SignalContext`. While any is live the collector process stops dying on Ctrl-C |
| Declares no commands at all | The MAJORITY. Roughly 60 of 97 reach Stage 1 with an empty `Commands` list, so "declared nothing" is the normal answer and never a failure signal |

`./le command list` must not program nftables. **This table is the answer to any
future proposal to run engines for introspection.** `plan/spec-plugin-query-mode.md`
transcribes the part of it that bears on a query mode, and its A-3 says the
findings are re-read at design time rather than trusted forever.

The last row is the one that generalizes past this repository. A collector whose
normal answer for two thirds of its inputs is "nothing" has no failure signal:
an engine that aborted before declaring and an engine that declared nothing look
identical downstream.

## 2. A latent inherited value becomes a behavior change the moment it is published

`DeclaredForCommand` (`internal/component/command/declared.go`) resolves by
longest declared prefix, which is what a running daemon answers. That
inheritance predates this work. No reader printed a column order, so a wrongly
inherited value reached nobody and nothing was red.

Publishing turned three dormant wrong values into public claims in one commit.
`show bgp rib help`, `commands` and `events` each inherited `show bgp rib`'s
answer: eleven route columns, two address fields, a `tab` shape and thirteen
route pipe filters, on three commands whose answers hold subcommand names. Each
now declares its own.

**The class: making a latent value visible is a behavior change, even when no
code path changed.** The three were found by forcing a red, not by reading the
diff, and the fourth (the pipe-filter barrier) was found while forcing the red
for the first. A reader that starts publishing a registry is a reader that
starts asserting what the registry holds.

The same shape has a second face here. Deleting a command's own declaration does
not empty its answer, because the parent's answer arrives instead. A test
asserting only that `column-orders` is PRESENT passes over that, which is why
the discrimination break for this spec asserts the exact order
`[prefix max-length asn]` rather than its presence.

## 3. A gate that compares two sources can be vacuous about what they carry

`./le plugin declarations check`
(`internal/le/plugin/declarations/plugindeclarations.go`) holds each plugin's
`registry.Registration.Commands` against the `sdk.Registration` its runner
builds. The first version compared COMMAND NAMES, in one direction. It could not
fail on a shape, a column order or an address field, which is every field the
spec exists to carry, and it could not fail on a command only the registration
held.

Review found both. The gate now compares every field an entry states, in both
directions, and refuses a package it cannot pair rather than letting the last
literal read overwrite the first.

**The class: a gate over two declarations of one fact must be tested against a
disagreement in the FIELD it exists to protect, not only in the key.** A gate
whose red phase was only ever forced on a missing key has no evidence about the
values.

That gate has a second, sharper limit worth writing down. It reads the two
literals as SOURCE TEXT, so a shape written as `table` where the wire vocabulary
spells it `tab` passes: both literals say `table`, and they agree. Stage 1 would
refuse the WHOLE registration for it (`validateShapeDecls`,
`internal/component/plugin/server/startup.go`), so the catalog would publish
commands no daemon serves, which is exactly what the gate exists to prevent.
`TestEveryDeclaredShapeIsOneStage1Accepts`
(`internal/component/plugin/all/all_test.go`) closes it by walking
`registry.All()` and holding every in-tree `Shape` to what Stage 1 accepts. A
syntactic comparison of two literals can prove they agree and can never prove
either one is valid.

## 4. A guard needs a path that can deliver the value it rejects

AC-4 is a claim about a READER: it answers a plugin's declarations with no
engine started. The first version of `TestRegistrationCarriesTheDeclaredCommands`
sat in `internal/component/plugin/all`, stubbed every `RunEngine`, and then read
`registry.All().Commands` itself. That package holds the composition root and no
reader, so nothing under the stub could ever have reached a `RunEngine`. Its
"no engine started" assertion had nothing that could have started one.

It moved to `internal/le/command/list` and drives `Collect`, the reader
`./le command list` runs. Its red was then forceable: adding
`_ = registration.RunEngine(nil)` to `Collect`'s plugin loop makes it say
`reading the declarations started 92 engine(s)`.

**The class: a negative assertion is only as strong as the path under it.** Ask
what call would trip this guard, then check that the test's own stimulus can
reach that call. If it cannot, the guard is decoration and the test is green for
the wrong reason.

## The design, in one line

`registry.Registration` was called a dead end because it had no command field.
That was a fact about the tree on the day someone looked, not a constraint. One
`commandDecls()` function, two readers, nothing copied, so nothing can disagree
(`ai/rules/principles.md`), and one gate to catch the plugin that declares to
one and forgets the other.
