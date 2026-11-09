# Controlled startup abort: source diagnosis and handoff

No execution occurred. This is a repair of the test envelope, not the original
cleanup defect or an observed runtime RED.

1. Symptom: an unprotected safety-accept failure could return while its helper
   was still preparing, and that helper could subsequently start a child.
2. Producing cause: the helper hashed four inputs before observing safety EOF;
   its startup deadline was relative to its own later schedule. The supervisor
   discarded `controller.native` after twenty seconds while the helper could
   remain active for forty. `TestNativeCatalogPartialParent` also stopped
   observing its direct child on an independent thirty-second timer.
3. Owner: the private controlled probe's startup protocol and sole Wait owners.
   The source rule was incomplete; no kernel/PID-reuse fault was needed.
4. [workaround] Increase a product limit, force an untransferred process group,
   or forget the join on a timer. [source] Retain the original absolute startup
   deadline, observe abort before preparation, serialize abort with child
   ownership registration, and retain each actual Wait through settlement.
5. The workaround either changes the oracle, risks another process, or returns
   without proving cleanup. This revision makes no such force or success claim.

## Actual selected delta

- `TestNativeCatalogCompilerTool`: only the explicitly selected partial seam
  receives an absolute startup deadline, bounded cancellable hashing and a
  child Start guarded by its abort owner. The ordinary native path is unchanged.
- `nativeCatalogPartialHandoff.readControl`: one lifecycle reader consumes
  exactly the existing safety grant. Before that grant, EOF/expiry closes
  startup and joins an already started child. The same mutex covers the fork
  and registration interval, so an observed abort cannot fall between them.
- `nativeCatalogPartialHandoff.close`: closes the connection, joins that reader
  and retains any actual child Wait before the helper may return.
- `proveNativeCatalogPartialHandoff` and `TestNativeCatalogPartialParent`:
  retain their original sole Wait bridges, without the discarded-handle branch
  or independent twenty/thirty-second give-up timers. Existing 40/45/50-second
  process test bounds remain in place. No product bound changes.

The protected grant ends this startup-abort reader. Actual old `finish` still
kills the helper before joining its untransferred descendant, and final safety
signals/reaping remain after every intended oracle. The original missing-join
failure is not repaired or satisfied by the new guard.

## MAIN work still required

Obtain independent actual-delta review before formatting/execution. MAIN formats
only `catalog_cancel_test.go.txt`, records source identities, and runs only
`TestNativeCatalogPartialHandoffCleanup` through the existing overlay and
admitted one-minute command in `HANDOFF.md`. Keep the old native roots held.
Read the exact named failure and independently completed final safety reaping;
compile, preparation, hashing, transfer or pidfs failures are not the RED.

The fresh continuation read the complete changed delta against `refused-v2`,
checked unchanged canonical/native snapshots and the exact old `finish` body,
and ran no formatter, compiler, runtime operation or hash computation.
No approval row is owed for unchanged original assertions; the independent
reviewer decides execution safety from this exact source, not this handoff.
