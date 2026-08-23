# A reconciler repairs against a snapshot older than its own input

A periodic reconciler reads state on a timer and compares it against state the
system updates from an event stream. Where the two reach the reconciler by
different paths, the snapshot can be older than the events applied just before
it. The comparison then reports a contradiction that no longer exists, the
reconciler repairs something that was already correct, counts a repair, and the
next pass undoes it.

Nothing is lost and nothing stays wrong, which is why this survives review: the
end state is right. What it costs is the meaning of the signal. A counter that
says "an event was lost" now has two producers, so anything reading it as a
discriminator is reading a number that fires for a second reason.

The tell is a snapshot and an event stream reaching one consumer with no
ordering between them.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-23 | fixit-link-event-drop-loses-carrier-state | iface carrier resync, and the counter its functional test discriminates on | `resyncCarrierState` (`internal/component/iface/link_queue.go`) repairs an interface whose acted-on route metric contradicts live carrier, and its idea of live carrier is a snapshot the rate tracker took on its 1 Hz tick (`SubscribeCollectNotify` in `register.go`, a one-second `time.NewTicker` in `rate.go`). That snapshot is pushed into the SAME queue the carrier events use, so the worker applies it after any carrier entry sitting ahead of it in `order`. A transition landing between the last tick and the drain therefore leaves the worker comparing fresh metric state against a snapshot up to a second older, `carrierContradicted` answers true for a contradiction that no longer exists, and the resync moves the route and increments `ze_iface_carrier_resyncs_total`. The next tick moves it back. The operator sees a transient metric move and a counter that over-reports. This explains an observation an earlier phase of the same spec left unexplained: one 401-transition QEMU run reported one carrier resync on FIXED code with no kernel netlink drops, while a later 401 sweep reported none. Derived by reading the producers rather than by re-running, because the race is structural and a stochastic re-run cannot prove its absence | not fixed. It blocks no goal of the spec that found it: no event is lost, the route ends correct, and the counter over-reports rather than under-reports, so the shipped assertion in `test/plugin/iface-link-flap-during-commit.ci` that it stays at zero is conservative rather than wrong. What it costs is precision, because the counter is documented as saying an event was never delivered or a route install failed, and it can also mean a snapshot and the event stream disagreed for under a second. Two repairs fit. Skip an interface whose carrier the worker itself set after the snapshot was taken, or require the same contradiction on two consecutive passes, which a transient cannot survive and a real loss always does. Either changes what that `.ci` measured for its recorded red-and-green proof, so it wants a QEMU run of its own |
