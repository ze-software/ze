# Deferrals

Tracked deferrals from implementation sessions. Every decision to not perform in-scope work
must be recorded here with a destination (receiving spec or explicit cancellation).

A row lives here only while the work has **no home**. Once it is moved into a spec, it is
resolved (`ai/rules/deferral-tracking.md`: "Moved to another spec → `done`, Destination =
receiving spec") and the spec becomes the tracker. Run `/ze-status` for the live backlog.

> 2026-07-06: backlog triage. The accumulated log (220 rows) was cleared. 113 rows were already
> resolved (done/cancelled). Every remaining row was verified against the codebase with a
> producing `file:line`: 24 were already implemented (closed) and 83 were migrated into 13
> consolidated umbrella skeleton specs, named `spec-finish-<subsystem>` (subsystem shipped,
> residual/test bits left) or `spec-followup-<subsystem>` (additional/future work).
> Finish: `spec-finish-l2tp`, `spec-finish-ci-coverage`, `spec-finish-report-bus`,
> `spec-finish-vpp-stub`. Followup: `spec-followup-test-infra`, `spec-followup-hooks`,
> `spec-followup-vpp-traffic`, `spec-followup-vpp-iface`, `spec-followup-l2tp-call`,
> `spec-followup-bgp-rib-arch`, `spec-followup-bgp-feature`, `spec-followup-web-cli-ux`,
> `spec-followup-subsystem`. Plus one re-point to `spec-firewall-dynamic-address-group`.
> The pre-triage revision of this file (in git history)
> preserves every closed row with its evidence. The log below is intentionally empty.

> 2026-07-08: split. `spec-followup-bgp-rib-arch` (named above) was split into one child
> spec per work item under the `spec-rib-arch-*` prefix (`spec-rib-arch-0-umbrella` indexes
> `-1`..`-8`), per `ai/rules/planning.md` "Spec Sets". The umbrella file was renamed via
> `git mv` (history preserved); see `git log --follow plan/spec-rib-arch-0-umbrella.md`.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-09 | spec-followup-web-cli-ux AC-8/9 | Nushell shell-generator glue for the new flag inventory + `ze config show` config-section completion | AC-8 scopes completion to bash/zsh/fish (all wired + tested); nushell's single-completer model needs separate, un-runnable-here wiring | `plan/spec-followup-web-cli-ux.md` (AC-8/9 follow-up) | deferred |
| 2026-07-09 | spec-followup-web-cli-ux AC-5 | Subprocess-plugin web-route extensions (out-of-process plugins registering Go `http.Handler`s) | Architectural: ze plugins are subprocesses (JSON/text IPC), cannot register in-process handlers; AC-5 scoped to in-tree component pages by user decision | none (permanent exclusion; AC-5 covers in-tree pages) | cancelled |
| 2026-07-09 | spec-followup-web-cli-ux AC-1 tail | Control-hiding on purpose-built workbench pages (bgp peers/groups/policy, system, firewall, interfaces Add buttons via `workbench_table.html`/`WorkbenchTableData`) | Those page builders construct table data without the `*http.Request`, so `ReadOnly` cannot be threaded without a wider refactor; enforcement is already complete (route gate 403 + per-mutation authz), so this is UI polish only | `plan/spec-followup-web-cli-ux.md` (AC-1 follow-up) | deferred |
| 2026-07-10 | spec-followup-vpp-iface A-4 | BGP netns-aware listener: teach the BGP TCP listener to bind inside a named network namespace, so LCP TAPs in a non-root netns (`vpp.lcp.netns`) are reachable by BGP without forcing the operator to a root-reachable netns | Out of scope for the iface spec (BGP has zero netns awareness today, `reactor/listener.go`); `doctor-vpp-lcp-netns` makes the constraint visible meanwhile | none yet (future `spec-bgp-netns` when picked up) | deferred |
| 2026-07-10 | spec-followup-vpp-iface | Doctor check for VPP linux-cp plugin presence (parallel to `doctor-vpp-wireguard`): warn pre-apply when `lcp.enabled` but the VPP build lacks `linux_cp_plugin.so`, instead of failing the whole apply at the binapi layer | Needs a runtime GoVPP probe; the netns doctor check (delivered) does not cover plugin absence | none yet (future doctor follow-up) | deferred |
| 2026-07-10 | spec-pki-full-chain (design) | Looking-glass TLS serves a PKI-stored chain: `cmd/ze/hub/service_lg.go:78` keeps the self-signed-only `LoadOrGenerateCert` path; extend by consuming `pki.ServerTLSMaterial` like web/DoT/DoH | Design bounded to web + dnsserver consumers to keep the spec reviewable; same pattern applies cleanly later | none yet (small follow-up once pki-full-chain lands) | deferred |
| 2026-07-10 | spec-pki-full-chain (design) | Multi-intermediate chains: `intermediate` holds a single certificate (`pki/config.go:147-158`), so a 4-tier CA (leaf + 2 intermediates) cannot be expressed; extend `intermediate` to a list | Single-intermediate covers the common case; the spec's doctor chain check reports AKI/SKI mismatch so the gap stays visible | none yet (extend when an operator needs deeper chains) | deferred |
| 2026-07-10 | spec-followup-test-infra AC-5 | Wire the LLGR egress filter into the RIB readvertise / `AnnounceNLRIBatch` direct-send rail so RFC 9494 per-peer egress divergence (NO_EXPORT+LOCAL_PREF=0 for non-LLGR IBGP, withdraw for non-LLGR EBGP) actually fires on the wire; then the multi-peer + RR-client `.ci`s. Root cause (file:line): `LLGREgressFilter` (`gr_egress.go:57`) runs only on ForwardUpdate (`forward_rs.go:324`, `reactor_api_forward.go:490`); the only `meta["stale"]` producer (`rib_replay.go:299`) flows through `AnnounceNLRIBatch` (`reactor_api_batch.go:28`) which drops `ctx.Meta` and calls no filter (documented `sdk_engine.go:42`) | LARGE FEATURE: hot-path wiring (buffer-first) with broad blast radius (all plugin route injection), not a bounded fix; out of scope for a closure session | `plan/spec-rib-arch-7-llgr-multipeer-ci.md` (updated with root cause) | deferred |
| 2026-07-10 | spec-followup-bgp-feature item 3 | AS-Confederation OTC (RFC 9234 §5 confederation rules: OTC value = confed identifier, member-AS semantics) | User decision 2026-07-08: re-defer unchanged — ze is a single-AS speaker (`role.getLocalASN`, role.go:66) so §5 is vacuously satisfied; true support needs confederation-member config + AS_CONFED origination first (large feature) | none yet (future confederation spec) | deferred |
| 2026-07-10 | spec-followup-bgp-feature item 4 | community-name web decorator leaf wiring (decorator registered in service_web.go and functional; no community leaf exists in the BGP YANG to attach it to) | Blocked on a BGP YANG community leaf existing; recorded 2026-07-08 at item 4 completion | none yet (wire when a community leaf lands) | deferred |
| 2026-07-10 | spec-improve-7-yang-handler-gate (Known Limitations) | Strict unknown-key rejection at config verify (reject config keys absent from the schema); `validator.go:527` is permissive today | Opposite direction from improve-7's handler-completeness gate (config-not-in-schema vs schema-not-in-handler); recorded as follow-up candidate in the spec's Design Insights | none yet (future strict-unknown-key-verify spec) | deferred |
| 2026-07-10 | spec-improve-3-event-replay (R-3) | Exact goroutine-interleaving reproduction in event replay (deterministic scheduler / event-queue layer) | Replay asserts outcomes (FSM transitions, RIB effect), not interleavings; exact reproduction needs the analysis doc's event-queue layer, out of scope for this spec | none yet (future event-queue scheduler layer, per the reactor analysis doc) | deferred |
| 2026-07-10 | spec-radius-acct-timewheel (Known Limitations) | RADIUS accounting packet content (which attributes are emitted) | The timewheel spec covers interim-update scheduling only; packet content is the sibling spec's concern | `plan/spec-radius-subscriber-attributes.md` | deferred |
| 2026-07-10 | spec-radius-subscriber-attributes (Known Limitations) | Interim-update accounting scheduling (timer wheel) | The subscriber-attributes spec covers packet content only; scheduling is the sibling spec's concern | `plan/spec-radius-acct-timewheel.md` | deferred |
| 2026-07-10 | spec-radius-subscriber-attributes | Adjacent verified RADIUS gaps: Calling-Station-Id (31), Event-Timestamp (55), Acct-Delay-Time (41), Acct-Terminate-Cause (49), present in the dictionary but not emitted | DESIGN decides whether these join the subscriber-attributes scope or stay out; flagged so the decision is not lost | `plan/spec-radius-subscriber-attributes.md` (DESIGN-phase decision) | deferred |
| 2026-07-10 | spec-startup-resilience (Known Limitations) | Re-apply idempotency (osvbng's sibling theme) | Explicitly scoped out of the startup-resilience spec; findings route to a new spec when picked up | none yet (future re-apply-idempotency spec) | deferred |
