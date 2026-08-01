# bug-review-backlog-closure

Closure of the 8 bugfix specs produced by the bug-review program
(`plan/review-bug-review-final.md`): BENG-001/003 (validate OPEN
capabilities and ROUTE-REFRESH before delivery), BENG-002 (convert to the
destination EncodingContext before splitting oversized forwards), BENG-004
(reactor startup aborts through one cleanup path), BENG-005 (IPv6
link-local next-hop-self avoids a 32-byte heap alloc), BPLUG-001 (strict
NLRI parsers for labeled/MUP/MVPN/VPLS), BPLUG-002 (SR-Policy owner-package
encoders), SYS-001/002 (plugin-startup rollback + correct reload cleanup
identifiers), SYS-003 (DirectBridge callback panic returns a prompt error).

## Reusable lesson: review Task promises against the AC table, not just the ACs

A spec can pass its own "done" gate while silently dropping a capability
the **Task** or **Security Review** section promised, because ACs drift
toward unit-level checks and no AC encodes the promise. Found here in
`bugfix-bgp-reactor-startup-cleanup`: the Task and Security Review required
that *no event subscription remain after a failed startup*, but the AC
table only covered listeners, cache, context, and Stop-safety. `abortStartup`
correspondingly released everything **except** the EventBus subscriptions
(`SubscribeInterfaceEvents` handlers leaked on a failed startup), and the
two failure tests built the reactor with a nil eventBus so the gap was
invisible. Fix: shared `releaseEventSubscriptions` helper called from both
`cleanup` and `abortStartup`, plus a fake-bus test asserting
`activeSubscriptions()==0` after a failed start. This is the
`ze-review-spec` step-9 Task-vs-AC cross-check earning its keep: when a
Task promises an operation and no AC covers it end-to-end, treat it as a
BLOCKER even though every listed AC passes.

## Lock-safety note for resource-release helpers

`abortStartup` runs while holding `r.mu`; `cleanup` runs before taking it.
Sharing one unsubscribe helper across both is safe only because the engine
EventBus (`internal/component/plugin/server/engine_event.go`) copies
handlers under its own lock and invokes them outside it, and the unsubscribe
func is a non-blocking, idempotent map delete. So releasing subscriptions
under `r.mu` cannot deadlock with an in-flight handler that is waiting on
`r.mu.RLock()`. Verify bus blocking/locking semantics before reusing a
"release before taking the lock" pattern under a held lock.

## Round 2: deeper-review BLOCKERs (the initial fixes were incomplete)

A second review pass found five BLOCKERs where the first-cut fixes did not
hold. All are now fixed with regression tests that fail under the old code:

- BENG-002 (transcode): forwarding decided ASN4 transcoding from the
  original `SourceCtxID`, but the EBGP-wire and RS-client paths had already
  rewritten the payload to the destination ASN width while keeping that ID,
  so an already-2-octet AS_PATH was re-parsed as 4-octet and corrupted/
  dropped. Fix: new `internal/component/bgp/reactor/forward_context.go`
  `fwdContextIDWithASN4` makes every transcode site report the
  post-transcode width via `WireUpdate.SourceCtxID()`. **Lesson: when you
  transform a payload, update the context/ID that downstream code reads to
  decide the same transform, or it runs twice.**
- SYS-001 (startup): a failed plugin tier rolled back only that tier, but
  all tiers were spawned up front, so later tiers leaked as unmanaged
  processes. Fix: `rollbackNonRunningStartupProcesses` sweeps the entire
  batch, not just the failed tier.
- SYS-002 (reload): failed-reload cleanup could stop a pre-existing
  dependency. Fix: stop only the exact set this reload auto-loaded
  (`autoStopPluginNames`), never the `stopOrphanedDependencies` path.
- BENG-001/003 (NOTIFICATIONs): OPEN Unsupported-Capability now carries the
  offending capability TLV (RFC 5492 Section 5); invalid ROUTE-REFRESH now
  carries the complete message including the 19-byte header (RFC 7313
  Section 5). **Lesson: error NOTIFICATIONs have RFC-mandated Data fields;
  a test that asserts only code/subcode hides empty or truncated Data.**

## Verification context at closure

All bug-review packages pass `go test -race`; `make ze-lint-changed` is
clean (0 issues); the new `test/encode/bgp-srpolicy-*.ci` fixtures pass.
Note the working tree also held a concurrent IS-IS session's uncommitted
work (`internal/component/isis/*`, `docs/guide/isis.md`, `test/isis/*`);
those files were deliberately excluded from this commit.
`make ze-verify-changed` is red only on failures pre-existing
on HEAD and unrelated to this work: `internal/component/plugin/all`
(`TestRegisteredPluginNames` etc.) and `ze-doc-test` drift from the IS-IS /
firewall-irr / bgp-capa / rsvp-te additions, and BGP functional fixtures
(e.g. `bgp parse 45`, plugin 56, bmp 73-75) tripping the mandatory
prefix-maximum validation in `internal/component/bgp/reactor/config.go`
(unchanged vs HEAD) and firewall-irr YANG description-mismatch warnings.

## Files

None recorded.
