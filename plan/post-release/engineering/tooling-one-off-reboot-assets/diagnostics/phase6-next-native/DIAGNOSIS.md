# Why this observer is additive

Symptom: the existing cancellation helper enters `Answer` directly. Its PASS
does not prove native launcher/composition-root cancellation, and its ordinary
sleep descendant does not establish TERM-ignoring teardown.

Root cause at the proof producer: `TestCatalogCancellationConsumerHelper` calls
`Answer`; `newCatalogCancellationProbe` starts that test binary as its consumer.
The actual `runDrift` -> `Drift` -> `BuildDaemon` route already exists. This
package changes the proof carrier, without claiming a product correction.

Owning layer: the tooling functional observer, in the existing yangcontract
test package. Native process context remains owned by `runDrift` and
`BuildDaemon`; the observer never replaces either implementation.

[workaround] Count the direct-helper result as native proof, use a PATH go
shim that native activation bypasses, or force the entire cache cold with -a.
Each changes the claimed boundary or cost instead of observing the owed route.

[source] Add a native CLI child and a shipped-compiler-only gate, hold exact
kernel process identities and sockets before cancellation, and preserve the
original product deadlines. A normal released-compiler control supplies the
positive build behavior. Existing direct-helper tests remain unchanged.

No runtime result exists. Independent source review and MAIN execution remain
required; the earlier proposal's hypothetical producer cut is not authored.
