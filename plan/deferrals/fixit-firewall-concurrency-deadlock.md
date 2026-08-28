# Deferrals: fixit-firewall-concurrency-deadlock

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-19 | spec-fixit-firewall-concurrency-deadlock functional-proof | sibling slices D-2/D-3/D-4 + core-design note remain (spec stays open) | live-server/QEMU constraint, deferred to CI | plan/spec-fixit-firewall-concurrency-deadlock.md | done |
| 2026-08-07 | spec-fixit-firewall-concurrency-deadlock D-2 review | The traffic VPP backend has the unbounded binary-API call D-2 removed from the two firewall backends. `Apply` (`internal/plugins/traffic/vpp/backend_linux.go`) builds `&govppOps{ch: ch}` and never calls `SetReplyTimeout`, so the channel keeps `core.DefaultReplyTimeout`, which govpp sets to 0 and documents as "default timeout for replies from VPP is disabled". The producer is `(*Channel).receiveReplyInternal` (`vendor/go.fd.io/govpp/core/channel.go`): `timeout := ch.replyTimeout; if timeout <= 0 { timeout = maxInt64 }`, about 292 years, and `Channel.ReceiveReply` has no context arm, so the caller's `ctx` cannot unblock it. A wedged VPP therefore holds `b.mu` in that backend forever. Fix shape is the firewall one: bind the deadline inside the ops constructor, since govpp pools channels and `(*Channel).Reset` does not clear `replyTimeout` | Out of scope here, and narrower: this spec owns the firewall reconcile, whose `reconcileMu` is process-wide, so an unbounded call there stalls every firewall owner. The traffic backend's lock is its own, so the blast radius stops at traffic. `ai/rules/planning.md` "Bounding the loop" homes it rather than widening this round | plan/spec-traffic-vpp-deferred-reply-timeout.md | resolved 2026-08-23: `newGovppOps` (`internal/plugins/traffic/vpp/timeout_linux.go`) installs the deadline before it returns the facade, and `(*backend).Apply` builds the facade through it. `ze.traffic.vpp.reply-timeout` defaults to 10s and clamps to 1s..60s, refusing zero because zero is govpp's spelling of no deadline. Three tests, because the first two alone would leave the fix resting on convention. `TestVppReplyTimeoutBounds` covers the clamp; `TestNewGovppOpsBindsReplyTimeout` proves the constructor installs the deadline, on its own fake channel; `TestGovppOpsIsBuiltOnlyByItsConstructor` (`internal/plugins/traffic/vpp/ops_construction_test.go`) parses the package's own sources and fails if anything builds a facade outside the constructor, or if it finds none at all. That guard is a RATCHET, not a proof: it reads the three forms that name the type directly (composite literal, `new()`, bare `var`), which covers the inline `&govppOps{ch: ch}` this replaced, and it does not see one built as part of another value. All three run inside the QEMU VM, and the guard also runs on darwin because its scan is build-tag blind. Each was shown RED against the defect it exists to catch. **The deadline bounds ONE round trip, not one apply**: an apply issues many, so a silent VPP holds `b.mu` for roughly the request count times the deadline. What this buys is termination, not a 10s ceiling |

| 2026-08-07 | spec-fixit-firewall-concurrency-deadlock review round 3 | the retired `scripts/dev/audit-test-relaxation.py` (current producer: `internal/le/testweakened/audit.go`) cannot see a `test-relax:` token in an UNTRACKED test file, so the audit is blind on exactly the files where a weakened assertion is easiest to introduce: brand-new ones. The producer is the script's own diff source, `git diff` against HEAD, which lists tracked modifications only; an untracked file has no HEAD side and never appears. Measured 2026-08-07: `test/plugin/firewall-metrics-registered.ci` carries two `test-relax:` tokens and the audit reported `1 finding(s)`, that one being another session's `gr_egress_test.go`, with this file named nowhere. `/ze-review` step 0 runs this script, so a review of a new test file silently audits nothing. Fix shape: include untracked test files (`git ls-files --others --exclude-standard`) and treat their whole content as added | Not this spec's subject and not reachable from its ACs: this spec owns the firewall reconcile, and the finding is a defect in the review tooling that happens to be invisible to it. Recording it where it was found would leave it in a shard nobody reads for tooling work. Homed with the other harness guards that cannot fire, which is that spec's stated subject | `plan/future/spec-harness-fail-open-guard-backlog.md` | deferred |

## Resolution of the 2026-07-19 row (2026-08-07)

Every named slice is implemented, and each one has a test that fails when the fix
is reverted. Slice, where it lives, and the red observed when it is reverted:

- **D-2, bounded kernel call.** `internal/plugins/firewall/nft/deadline_linux.go`
  (`netlinkTimeout`, `netlinkDeadlineOption`, `asKernelTimeout`), installed by
  `newBackend` in `backend_linux.go`. `TestNetlinkTimeoutBounds` refuses a zero
  deadline; `TestNftApplyDeadlineSurfacesError` runs to the harness timeout without
  the SockOption. Both run under `./le qemu run command "./le qemu all-tests"`.
- **D-2 observability (AC-10 "counted").** New: `internal/component/firewall/metrics.go`
  (`observeApply`), wired by `ApplyAll` and `Registration.ConfigureMetrics`.
  `TestApplyAllCountsKernelTimeout` reports `timeout counter = 0, want 1` with the
  increment removed.
- **D-2 for the vpp firewall backend.** New:
  `internal/plugins/firewall/vpp/timeout_linux.go` (`vppReplyTimeout`, `newGovppOps`,
  `asDataplaneTimeout`). `TestNewGovppOpsBindsReplyTimeout` reports "SetReplyTimeout
  was never called"; `TestApplyTagsDataplaneTimeout` reports
  `errors.Is(err, ErrKernelTimeout) = false, want true`. Both observed in QEMU.
- **Metrics wiring proof.** New: `test/plugin/firewall-metrics-registered.ci`
  (needs-linux, caps=net-admin). Green in under a second; with `ConfigureMetrics`
  emptied it polls its whole budget and fails.
- **D-3, ddos-local status off the reconcile lock.**
  `internal/plugins/ddos/local/responder.go` (`setStatus`, `status`).
  `TestResponderStatusDuringSlowApply` reports "status() blocked behind the
  in-flight reconcile" when `status` takes `r.mu` again.
- **D-3 rollback (R-8).** Same file, `applyMitigation`.
  `TestKernelTimeoutSkipsRollbackReconcile` reports "applyAll called 2 times,
  want 1" when the timeout branch is removed.
- **D-4, anomaly-shape.** `internal/plugins/anomaly/shape/responder.go`
  (`publishStatus`, `statusSnapshot`). `TestShapeStatusDuringSlowApply` reports
  "statusSnapshot() blocked behind the in-flight reconcile" when the lock is restored.
- **core-design note.** `docs/architecture/core-design.md`, "Firewall reconcile
  concurrency": the lock order, the bounded-`Apply` obligation per backend,
  `ErrKernelTimeout`, and the two metrics.

The recorded reason ("live-server/QEMU constraint") did not hold: QEMU runs on
this machine and the linux-only half was proven there, not deferred to CI.

The spec stays OPEN for one item this row never covered: phase 1, the 2026-07-12
reproduction (AC-1, AC-2, AC-3, AC-7 and `test/plugin/ddos-firewall-concurrency.ci`).
Assumption A-6 is still unvalidated. R-6 recommended waiting for
spec-fixit-qemu-runtime-kernel rather than debugging on a kernel recorded as
crashing on nft operations. **That wait is over, and on both counts** (2026-08-24):
that spec closed, both functional QEMU targets boot ze's own runtime kernel, and
the crash claim itself was measured FALSE. Stock 6.12.13-0-virt runs the whole
firewall suite 24/24. The reproduction therefore runs against a healthy kernel,
and a stall it finds is a Go-level defect.
