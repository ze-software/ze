# Refused fixture: diagnosis before repair

No runtime result exists. These are source findings from the independent review,
not product defects or observed test failures.

## P1: child owner lost during handoff

1. Symptom: a failed second receipt can enter fixture cleanup while the helper
   has a successfully started TERM-ignoring child outside `p.handles`.
2. Producing cause: `nativeCatalogProbe.finish` force-kills the recorded gate
   before closing its control channel. SIGKILL bypasses the actual helper's
   deferred `child.finish`; the untransferred child's owner disappears.
3. Owning layer: this new diagnostic fixture and its startup/handoff protocol,
   not `runDrift`, `BuildDaemon` or the product runner.
4. [workaround] Kill a numeric process/group, add a timer that silently removes
   the child before the oracle, or let the final safety owner satisfy cleanup.
   [source] Keep an abort owner throughout startup, explicitly settle each
   child handoff, and require the actual child's join acknowledgement before
   a controller may force the helper. Monitor controller EOF at every startup
   step; preserve every product oracle before fixture fallback.
5. The workaround either risks foreign work or hides the exact lost-owner
   failure. The source repair is not implemented pending MAIN's baseline RED.

The controlled baseline invokes the actual helper and unchanged `finish`.
An optional, test-only seam transfers the actual child's Start pidfd and the
helper's pidfd to a separate safety endpoint BEFORE the helper can publish
gate readiness. An ownership acknowledgement makes later force safe for this
test only. The actual controller never receives the child's normal packet.
The independent child observer receives that packet without granting it to
the actual cleanup controller. The old `finish` must still demonstrate the
lost owner; final safety cleanup happens only after those assertions.

The seam also samples a child-joined acknowledgement only after the actual
`child.finish` returns and its sole Wait worker is complete. It does not fix
the actual controller's cleanup order. Normal native roots never select the
seam and remain refused. Before ownership acknowledgement, a failed test
closes its safety control and waits for the helper's own abort/join. It never
force-kills an unprotected helper or guesses a PID/group.

## P2: foreign sentinel expires before the case

1. Symptom: permitted five-minute compiler readiness plus a normal minute can
   outlast the foreign helper's earlier independent five-minute deadline.
2. Producing cause: `TestNativeCatalogPeer` independently calls
   `SetDeadline(time.Now().Add(5*time.Minute))` for all roles.
3. Owning layer: the diagnostic sentinel's case/controller lifecycle.
4. [workaround] Enlarge the product's cancellation/build limits or silently
   restart the sentinel. [source] Root the helper in the same original case
   deadline and controller abort/EOF lifecycle, retaining its ready connection.
5. The workaround changes the behavior under test or loses continuous foreign
   identity. The source repair and a deadline discriminator are still owed.

The quick partial-start baseline does not wait for five minutes and claims
no P2 result. The full native roots remain held until both fixture defects
are repaired, independently reviewed and proven.
