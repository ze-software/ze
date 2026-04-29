# 627 -- fw-7-traffic-vpp

## Context

Ze had a single traffic-control backend (netlink, spec-fw-3) and a component
reactor wired to call `Apply` on reload (spec-fw-9). The VPP lifecycle
component (spec-vpp-1) provided a shared GoVPP connector. The gap was a VPP
traffic backend registering as `traffic.RegisterBackend("vpp", ...)`. The goal
was to program VPP policers from the same `traffic-control { }` config the
netlink backend consumes, rejecting any qdisc or filter type that VPP cannot
represent faithfully at commit time so operators never learn about
incompatibilities only after apply. The spec also added a 5-second VPP
connection wait to handle cold-boot races where ze and VPP start together.

**Final scope after five review passes:** one HTB or TBF class per
interface, translated to a single VPP policer bound to interface egress
via `PolicerOutput`. Multi-class configurations, every filter type
(dscp / protocol / mark), and every other qdisc type (hfsc / fq / sfq /
fq_codel / netem / prio / clsact / ingress) are rejected at commit. The first two
implementation passes shipped DSCP and protocol filter translations
that pass 3 review proved were silent no-ops in VPP (classify table
never attached; QoS mark without ingress record); dropping them is the
per-`rules/exact-or-reject.md` response. Rate limiting by class is
functional; traffic classification by DSCP / protocol / mark is deferred
to follow-up specs that design the missing VPP pipelines.

## Decisions

- **Hard-fail over silent retry.** `Apply` calls `Connector.WaitConnected(ctx, 5s)`
  and returns `vpp not connected after 5s` on timeout instead of stashing state
  for later replay. Rejected soft-accept-with-warning and stash-and-retry: both
  create the "feature not wired" failure mode from the mistake log where the
  operator thinks QoS is active but nothing actually happened.
- **Per-backend `Verifier` function over YANG `ze:backend` gate.** The schema
  gate annotates leaves, not enum values, so "reject qdisc hfsc but accept
  qdisc htb" needs per-value logic. Added `traffic.RegisterVerifier` /
  `RunVerifier` called from `OnConfigVerify` after the schema gate passes.
  Original Decision 2 (YANG annotation) was revised mid-implementation.
- **Per-Apply GoVPP channel over held channel.** Backend struct holds only a
  `*vpp.Connector` accessor. Each Apply opens and closes its own channel.
  Simpler lifecycle and no pool-draining risk; matches fibvpp's per-call
  channel pattern.
- **Reject every filter type AND `qdisc prio` under the vpp backend.**
  Three separate review findings converged on the same conclusion:
  (a) `filter mark` has no VPP equivalent (packet-header matching, not
  SKB metadata), (b) `filter protocol` needed classify-table
  attachment and a correct packet offset the first draft did not have,
  (c) `filter dscp` needed ingress `QosRecordEnableDisable` the first
  draft did not emit, (d) `qdisc prio` had no operator-facing
  class-index-to-DSCP mapping. Each would have shipped as a silent
  no-op. Per `rules/exact-or-reject.md` we reject at commit and defer
  the pipelines to named follow-up specs. HTB/TBF policers are the
  only shipping translation, and they work end to end.
- **Exact or reject (new project-wide rule).** Codified from review
  findings on fw-7 into `rules/exact-or-reject.md`: if a backend cannot
  apply EXACTLY what the operator's config asks for, the verifier MUST
  fail at commit with a clear error. No silent approximation, truncation,
  or best-effort mapping. Surfaced from an `egressMapFromPrioClasses`
  instance that silently discarded classes beyond 256, and a DSCP-filter
  path that overwrote prior entries because each filter issued its own
  `QosEgressMapUpdate`.
- **Component tracks previous state; backend tracks in-VPP state.** The
  traffic component's reactor already holds `previousCfg` and invokes
  `Apply(desired)` with the full new state. Backend tracks which policer
  names it has bound to which interface so it can diff and remove what is
  no longer referenced. No duplicate state tracking between layers.
- **Per-Apply undo list for partial-failure rollback.** Every successful
  `PolicerAddDel`, `PolicerOutput`, `ClassifyAddDelSession`, and
  `QosMarkEnableDisable` appends an undo closure to a local list. On any
  error before commit, the undos run in reverse, so VPP returns to its
  pre-Apply state before the component's journal rollback re-applies
  the previous config. Avoids orphaned policers accumulating on a flaky
  apply path.
- **Tolerant reconcile for VPP-restart.** Deletion of stale policer
  indexes / classify sessions (cached in `interfaceOutputPolicers` etc.)
  logs a warning and continues instead of failing the whole Apply. After
  a VPP restart the first Apply programs the new state successfully and
  the stale cache is replaced; no separate reconnect subscription
  required.
- **Per-interface DSCP map aggregation.** `applyInterface` now builds a
  single `QosEgressMap` across all classes and pushes one
  `QosEgressMapUpdate` per interface. The earlier per-filter push had
  a bug where N DSCP filters on one interface would produce N map
  replacements, with only the last filter's entry surviving.

## Consequences

- VPP users can now use `traffic-control { backend vpp }` with HTB, TBF, and
  prio qdiscs. DSCP and protocol filters are accepted; HFSC, fair-queue
  variants, netem, clsact, ingress, and mark filters are rejected with a
  specific error naming the unsupported type.
- The `RegisterVerifier` pattern is available to any future backend that
  accepts only a subset of what the YANG schema permits. Firewall's fw-8
  (future) can use the same hook for nftables-only vs VPP-ACL-only features.
- `Connector.WaitConnected(ctx, timeout)` is now a public method; any future
  VPP-dependent synchronous operation can use it instead of reinventing the
  polling loop.
- A future `spec-vpp-ci-infrastructure` is needed before `010-vpp-boot-apply.ci`
  (exercising real VPP from a `.ci` test) can land. Deferred with destination
  named in `plan/deferrals.md`.
- An AC-level test confirming backend-Apply programs a real policer requires
  VPP in CI; until then coverage is unit (translation) + .ci (reject path,
  timeout path) + code review of the binapi send helpers.

## Gotchas

- **`PolicerAddDel` returns a new `PolicerIndex`; `PolicerDel` takes that
  index, not the policer name.** Spec-driven implementation had to be
  revised to track `(name, index)` pairs in the backend state.
- **`fmt.Sscanf` in Go's stdlib does not support `%[...]` character
  classes.** A first-pass implementation used
  `fmt.Sscanf(key, "%[^|]|%d", ...)` to parse a composite
  `"ifaceName|proto"` session key; it fails at runtime with
  `bad verb '%['`. Fixed by replacing the string key with a typed
  `sessionKey{iface string; proto uint8}` struct -- safer and avoids
  reinventing a string parser.
- **`QosEgressMapUpdate` replaces the whole map.** Two DSCP filters on
  the same interface, each building its own update with one entry, end
  up with only the last one applied because each update blanks the
  previous. The fix is to aggregate entries at the interface level and
  push once per interface. Easy to miss in isolated unit tests.
- **`make ze-verify*` runs under a single repo-wide `flock`.** While a
  stuck `ze-test bgp plugin --all` from another session held the lock for
  80+ minutes, ze-verify was unavailable. Work proceeded using
  targeted `go test ./path/...` and `golangci-lint run ./path/...` on
  touched packages; the blocking pre-commit `ze-verify` ran after
  the lock cleared.
- **Vendored GoVPP v0.13.0 did not include `policer`, `policer_types`,
  `qos`, or `classify` binapi packages.** Same trap as vishvananda/netlink
  version drift in spec-iface-tunnel. Resolved by adding a blank-imports
  anchor file (`binapi_imports.go`) then running `go mod vendor`; the
  anchor file is permanent because non-Linux builds do not reference the
  same packages through `backend_linux.go`.
- **The auto-linter hook `block-silent-ignore.sh` flags `default:` in
  switch statements even when the default branch returns an error.** Avoid
  `default:` by pre-validating the discriminator with an `if != A && != B`
  guard before the switch; reads the same and passes the hook cleanly.
- **`ze config validate` (offline) does not invoke plugin OnConfigVerify
  callbacks** (memory.md "YANG Choice/Case Validation Gaps"). `.ci` tests
  for Verifier-driven rejection must run the daemon, not the offline CLI.
- **Review caught silent-approximation in multiple places on this spec**
  (prio-map class-index-as-DSCP, multi-DSCP overwrite, 256-class silent
  truncation, classify table never attached to an interface, QoS mark
  without ingress record, multi-policer stacking on output feature arc).
  Strong enough signal to codify a project-wide rule
  (`rules/exact-or-reject.md`) rather than treat as one-off fixes.
- **The multi-policer-stacking bug** is the most insidious kind. Tests
  pass, verifier accepts, backend programs VPP successfully, operator
  sees "commit applied." Runtime behavior is silently wrong: N
  policers on VPP's output feature arc run IN SERIES on every packet,
  so a config asking for "fast class 10 Mbps, slow class 1 Mbps"
  becomes "all traffic limited to 1 Mbps". Only pass-5 reasoning about
  VPP's feature-arc semantics exposed it. Mitigation: verifier now
  requires exactly 1 class under HTB/TBF until filter-based
  classification lands.
- **"Tests compile and pass" is not proof the feature works.** The unit
  tests for `egressMapFromPrioClasses`, `protocolMatchBytes`, etc. all
  passed on a translation that was structurally wrong at the VPP API
  layer. Unit tests validate the translator's internal consistency, not
  that the translator's output is what VPP will act on. For a backend
  talking to an external system, tests MUST either exercise the external
  system or the reviewer MUST read the external system's semantics.
  Reading VPP's actual classify-pipeline docs exposed the gap that
  three passes of unit-test-level review missed.

## Files

- `internal/plugins/traffic/vpp/` -- backend + translation + registration + verifier + tests (9 files)
- `internal/component/traffic/backend.go` -- added `Verifier` type, `RegisterVerifier`, `RunVerifier`
- `internal/component/traffic/register.go` -- `RunVerifier` wired into OnConfigVerify
- `internal/component/vpp/conn.go` -- added `Connector.WaitConnected`
- `internal/component/vpp/conn_test.go` -- new tests for WaitConnected
- `internal/component/plugin/all/all.go` -- blank import added
- `vendor/go.fd.io/govpp/binapi/{policer,policer_types,qos,classify}/` -- vendored at v0.13.0
- `vendor/modules.txt` -- 4 new package lines
- `test/traffic/011-vpp-reject-hfsc.ci`, `012-vpp-not-connected.ci` -- functional tests
- `docs/features.md`, `docs/guide/traffic-control.md` -- user docs
- `ai/rules/exact-or-reject.md`, `ai/rationale/exact-or-reject.md` -- new project-wide rule surfaced by review
- `ai/rules/design-principles.md`, `CLAUDE.md` -- cross-reference the new rule
- `plan/deferrals.md` -- closed 2 fw-9 deferrals (ze:backend annotations superseded), opened 4 new deferrals (010-vpp-boot-apply.ci VPP infra, filter mark VPP-native metadata match, qdisc prio mapping design, IPv6 protocol classify)
