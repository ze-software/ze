# Connected replay diagnostics: incremental independent review

November 13, source CLEAR for the seven named incremental Go deltas and own
redistribution guide paragraph. Retained earlier connected startup/removal
review is reused, not repeated. No concrete new blocker, execution, tracked
authoring, approval metadata, formal CLEAN or closure.

Read whole new183-line replay_diagnostic_test.go, changed replay/reporting
producers, SDK report Raise/Clear, registration and current metadata, bootstrap
fixture report RPC hook and propagated event errors, and all six strengthened
return-error checks. Three lenses: runtime/security, tests/wiring/edges,
style/docs. CodeDoctorPluginPanic explicitly describes plugin event application
failure; registration permits multiple owners and verifies code metadata.

The replay loop preserves all useful publications, logs each failed prefix
with registered code/fix and retains the first contextual error for one active
report. emitMu serializes reporting and publication. warningRaised changes
only after acknowledgement; partial success cannot clear the incident, while
a complete successful replay or an actually empty current inventory can.
Report Raise/Clear refusals return through reemitAll and OnEvent. Successfully
reported publication failures remain a visible active warning, as documented.

New tests use real SDK direct/socket callbacks, useful IPv4/IPv6 inventory,
one failed family beside a successful subset, deduplicated active reports,
stable subject/code/fix/error text, complete and empty recovery, and report
service refusal. Clear-refusal setup first confirms an acknowledged raise,
then requires both useful recovered prefixes before observing refused clear.
Existing seven SDK/lifecycle assertions are preserved; five old connected-test
calls plus the deterministic retired-replay call now require no error.

Read actual92ab4507 receipt: both transports fail on zero report requests and
false callback acknowledgement under old code. ROOT still owes new current
GREEN including the added clear-refusal case; this review claims none.

Frozen SHA-256:
```text
b420dab76823c6b5d98035046f6be72f306f286ad4cdac76509638a264ca3933 connected.go
784204d89dbe71c78ee502e8c06660d0f07795328dc505d5ff6eed4bd9d63235 eventbus.go
5aac52abd128d2165f56e196aaee57a768f5afdf822f777c3fde7e4375916c6f register.go
a71be60f075c7ef8aeab5d77b9d8e813607cd7b45fd23df08dfc9570093e7a72 bootstrap_test.go
942bb89d846ae33fba21a9959013a583b17ef6ee94fb362ed3a17179cad62a07 connected_test.go
2522fcb0fdcd2d17ba21ac1279e4ed042429c35c4345992fb15eb727981e0284 lifecycle_test.go
fd5bdbf3e9c6bf5e5870329d056b0b1e38b4bbbb0edfb33f587595b90a263755 replay_diagnostic_test.go
```
Paths are under internal/plugins/connected/. Match eventual committed delta
against these read units; original historical431 context remains retained.
