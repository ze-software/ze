# Learned: a panic boundary is only as good as the locks above it and the goroutine it sits on

`spec-improve-5-panic-boundaries` put one containment policy at every
network-input task in Ze. `internal/core/taskguard` `Task.Contain`, deferred
directly at a task entry, logs the panic at ERROR, counts it in
`ze_task_panics_total{component,task}`, writes a recovered crash record that
`ze show crashes` lists, and calls the unit's own teardown: a BGP Cease /
Unspecific, an OSPF KillNbr, an IS-IS adjacency teardown, an RSVP-TE soft-state
expiry, a StopCCN, a PADT, an IKE Delete, a plain error to a gNMI or config
client. The inventory, one row per task with its test, and the lock audit are
in the spec's commit history; the operator view is `docs/guide/monitoring.md`,
"Contained Panics", and the BGP lab proof is the interop scenario
`bgp-panic-cease`.

## What outlives the spec

1. **Containment turns a crash into a hang wherever a lock is unlocked by hand.**
   Six components held a lock with no `defer` across a call a panic could come
   from (the BGP reactor and AIGP state, RTR, L2TP, RSVP-TE, OSPF, IS-IS). With
   a recover above them, the next unit to take that lock blocks forever, and
   every peer sharing it stalls. A boundary is not done until every lock its
   task can reach releases by `defer`; each fix carries a test that recovers a
   panic under the lock and then calls `TryLock`.
2. **"This never runs on the receive worker" is a claim to trace, not to
   assume.** OSPF self-origination was audited as off the receive path. A
   received Grace-LSA and an LSDB content change both reach it, and a neighbor
   reaching Full reaches the graceful-restart exit. Trace every caller of the
   function before excluding it.
3. **A request's boundary does not follow its work to another goroutine.** The
   audit listed readers and connection handlers, and missed the work they hand
   off: one goroutine per plugin-to-engine request (`dispatchPluginRPC`), a
   per-session accounting goroutine, and the commit-confirm worker that runs
   every gNMI, SSH, web and API config write. The audit question is "which
   goroutine runs code on input a remote party chose", not "which goroutine
   reads the socket".
4. **An existing recover is not a boundary until it is read.** DNS and the OSPF
   registry wrappers recovered, and dropped the panic with no log line whenever
   the caller passed no callback, which every test did. A recover that can
   discard is the silent-default defect (`ai/rules/principles.md`).
5. **A test seam must sit on every path the input takes.** The BGP lab seam sat
   in `processMessage`; the coalescing reader skips it for a plain IPv4
   announcement, so the first lab run armed a trigger nothing consumed.
6. **A zero that names a role is a guard in disguise.** RSVP-TE's `lspRole`
   zero was ingress, so the panic teardown of a half-built LSP withdrew a push
   it never installed; PPPoE's zero `PppoxFD` closed fd 0. Both now start from
   an explicit Unspecified or -1.
