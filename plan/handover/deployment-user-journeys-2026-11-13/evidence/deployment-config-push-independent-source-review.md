# Configuration upload: bounded independent source review

Reviewer: deployment_complete_source_review. Parent SID
c43b4620-2510-44dd-8936-84100b444bb5; native namespace 79a29bf3.
This is source preparation, not a formal CLEAN or closure artifact.

## Scope actually reviewed

All changed hunks and reached producers of the 23 Go paths named in
deployment-config-push-source-freeze.sha256 were reviewed. New files were read
in full, including the real SSH receiver, real agent/transport fixtures and
independent framed-input fuzz oracle. Historical adaptations were independently
judged before application in deployment-config-push-historical-independent-test-judge.md.
The freeze manifest identifies the author's pre-format source; this report does
not claim those hashes equal later ROOT-formatted or repaired source.

Also reviewed the four-Go parser follow-up: setparser.go, setparser_meta.go,
parser_validation.go and new setparser_line_length_test.go, plus the syntax.md
paragraph. The live-upload sections in remote-operations.md and appliance.md
were reviewed. Other broad appliance-guide maintenance is outside this narrow
upload review and remains in the final documentation reconciliation.

The retained immutable 431-path historical review population is unchanged here;
its foreign IKE paths remain included. Later commits still need final immutable
population reconciliation rather than a new whole-population read.

Three lenses were applied inline: production/wiring and security, lifecycle
and rejecting edges with meaningful positive tests, and style/document claims.
Host entry -> real agent/pin -> registered SSH subsystem -> accepted identity's
commit authorization -> bounded complete frame -> daemon mutation owner ->
canonical candidate/reload/rollback/promotion -> exact receipt was traced.
The SSH owner's transport-first cancellation and joined workers were checked
against the actual charm.land/ssh v0.4.3 subsystem and Exit producers. New input
allocation is bounded before allocation; private remote/parser diagnostics are
not echoed; both SSH startup paths register the receiver before serving.

## Findings sent to ROOT

1. **PLAUSIBLE correctness blocker: confirmed-window exclusion can be bypassed
   during window withdrawal.** configMutationRuntime.commit in config_upload.go
   loads daemonConfirmWindow for every operation. stopConfirmWindow withdraws
   that pointer before Window.Stop joins its worker; main stops that window
   before closing the mutation owner's admission. An upload admitted in this
   interval passes nil to confirm.WriteOutside, which directly applies and
   skips pending-window refusal. A pending window remains recorded for boot
   revert, so an upload can be accepted despite that pending revert. Startup
   also publishes the owner before its window, although no pending window is
   established there. The storage candidate guard was read and prevents the
   initially suspected concurrent-candidate overwrite; no overwrite claim is
   made. ROOT must reproduce the actual owner/window interleaving with controlled
   barriers and a useful no-pending positive. A stable runtime-owned window or
   separate write-admission sealing can repair it, preserving revert reload
   admission. No runtime reproduction was executed by this reviewer.

2. **ISSUE: new repair commands name the wrong binary.** sshExecReal's new
   unknown/empty/oversize/subsystem-refusal fixes and registerSSHConfigUpload's
   frame refusal print `ze appliance ...`. The appliance door states shipped
   ze lacks ze_setup and the corresponding appliance commands; the guide now
   correctly names ./ze-host. Use the actual host-driver command for these
   new fixes, consistent with the existing buildhostdriver convention.

3. **Fixture isolation defect, corroborated by ROOT's actual warning:**
   sshUploadBefore/After omit system dns resolv-conf-path, so the real reload
   reaches applyResolvConf -> ExtractSystemConfig's /tmp/resolv.conf default
   -> Resolvers.Configure/flush and attempts a host file write. Add an explicit
   empty resolv-conf-path to both complete configurations, preserving payload,
   parser, persistent/running state, boundary and truncation assertions. The
   four-MiB fixture computes padding from the constant length and needs no
   changed bound. The declared-96 disconnect remains truncated after this
   short addition; update only its stale 32-byte comments. ROOT accepted this
   bounded correction; no source edit was made by this reviewer.

The parser follow-up has no source finding: SplitSeq scans the already-owned
input without a Scanner token ceiling or a materialized line slice, preserving
trim/line counters/schema refusals. New tests independently require long-value
preservation, metadata, LF/CRLF and the later refusal at line four. ROOT owns
the semantic RED and all subsequent GREEN/coverage/compile/native proofs.

## Remaining before a clean source disposition

Read only the fixes and their reached producers/new tests for the three findings,
then reconcile immutable committed deltas and actual proof receipts. Native
installed-appliance upload and remaining FRR/site/artifact proof obligations are
ROOT-owned; no runtime success or final gate is asserted by this report.

No tests, builds, formatters, product execution, tracked edits, commits or child
agents were performed in this review. Native historical-test approvals preceded
their edits and are recorded separately in the independent test-judge report.
