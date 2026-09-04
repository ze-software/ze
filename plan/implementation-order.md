# Implementation Order

Authored 2026-07-10 (followup-wave impact review session; user-requested queue).
This is the execution queue for `plan/`. To work from it, ask: "implement the next
item in plan/implementation-order.md" (optionally naming a track). Sessions still
claim specs individually via `internal/le/spec/session/session.go claim`.

Maintenance: when a spec closes, strike its row (append-only style, `~~row~~ closed
NNN`) and re-check the Blocked ledger. Re-derive from `./le spec status` if this
file and reality diverge -- reality wins.

Followup specs (`spec-followup-*`) are excluded: they are in flight in their own
sessions (l2tp-call phase 1/6, test-infra 6/7 awaiting AC-5, bgp-feature awaiting
two-commit closure).

## 0. Hygiene wins (closure-only, minutes each, any session)

These are done or nearly done; they only need the two-commit closure discipline.

| # | Spec | Why it is a win |
|---|------|-----------------|
| 0.1 | spec-ipsec-13-rekey-wire | flagged high-confidence by spec-closure-check: learned/1069 committed, spec still open; finish docs row if truly missing, then close |
| 0.2 | spec-ownership-1-rs-invariant | Status `done`, learned/1063 committed; needs closure commits only |
| 0.3 | spec-ownership-2-coordinator-types | Status `done`, learned/1064 committed; closure only |
| 0.4 | spec-ownership-3-reactor-modes | Status `done`, learned/1065 committed; closure only |
| 0.5 | spec-ownership-0-umbrella | Status `done`, no learned summary yet; write umbrella closure summary (model: learned/1099) + two commits |
| 0.6 | spec-release-audit-1 disposition | pending user decision (see report of 2026-07-10); RA-INV-001 resolved, inventory refresh is release-time work |
| 0.7 | spec-dhcp-log-level decision | pending user decision: premise overlaps the existing per-subsystem log surface (`slogutil.Logger` env lookup + runtime `log-set` command); either drop the YANG leaf and document/test the existing path, or define precedence -- options recorded in the spec 2026-07-10 |
| 0.8 | ping source-after-resolve | new finding 2026-07-10 (same bug class as traceroute-source-af, in ping's resolve path); decide: fold into the traceroute fix session or spawn a sibling spec |

## 1. Now queue (easy wins, small designed specs, one session each)

Ordered smallest-first within similar value; each is independent of the others and
of the parallel tracks below, so any subset can run concurrently.

| # | Spec | Status | Scope hint |
|---|------|--------|-----------|
| 1.1 | spec-tunnel-ttl-default | ready (2026-07-10) | netlink-only per user decision 2026-07-10; default TTL 64 on 4 tunnel kinds |
| 1.2 | spec-vpp-isolated-cpus | ready (2026-07-10) | cpu validation + isolated-CPU sourcing in startup.conf |
| 1.3 | spec-dhcp-log-level | design, ON HOLD (0.7 decision) | dhcpserver log-level leaf vs existing slogutil surface |
| 1.4 | spec-mgmt-version-header-suppress | ready (2026-07-10) | suppress X-Ze-Version on web/lg |
| 1.5 | spec-password-weakness-warning | ready (2026-07-10) | warn on weak admin passwords at commit |
| 1.6 | spec-traceroute-source-af | ready (2026-07-10) | source-AF resolution fix (resolve-path only; see 0.8 for ping sibling) |
| 1.7 | spec-bgp-as-notation | design | asdot/asplain display leaf |
| 1.8 | spec-pki-show-private-key | design | pki show with authz gate |
| 1.9 | spec-bgp-remote-as-auto | design | remote-as auto on listen |
| 1.10 | spec-bgp-evpn-route-type-match | design | filter match on EVPN route type |
| 1.11 | spec-firewall-prefix-normalize | design | canonicalize prefixes at parse |
| 1.12 | spec-isis-spf-lsp-timers | design | SPF/LSP timer leaves |
| 1.13 | spec-ipsec-lifetime-volume | design | volume-based SA lifetime |
| 1.14 | spec-ike-reauth | design | IKE SA reauth timer |

## 2. Parallel tracks (independent subsystems; one session per track)

Within a track: top to bottom. Across tracks: fully parallel (disjoint file sets).

### Track A -- VPP correctness (user goal 2026-07-10: "implement VPP correctly and fully")
| Order | Spec | Status |
|-------|------|--------|
| A1 | spec-finish-vpp-stub | ready (2026-07-10): stub parity + missing runtime .ci |
| A2 | spec-vpp-interface-in-use | design (fixed 2026-07-10: covers mirror/LCP/tunnel refs) |
| A3 | spec-bridge-disable-learning | design |
| A4 | spec-vrf | skeleton -> design next |
| A5 | spec-vrf-0-umbrella | design (big; VRF as organizing principle) |
| A6 | spec-ipsec-0-umbrella VPP-backend phase | skeleton (vendored-binapi pattern now exists) |

### Track B -- BGP engine and policy
| Order | Spec | Status |
|-------|------|--------|
| B1 | spec-bgp-bfd-strict | design |
| B2 | spec-bgp-update-delay | design |
| B3 | spec-gr-advanced | moved to `plan/future/` 2026-08-29 (hard-reset RFC 8538 + selection-deferral; ze advertises no N-bit today, so this is a feature) |
| B4 | spec-pol-0-umbrella | ready (big: structured policy language) |
| B5 | spec-rib-arch-0..8 set | skeleton set -> design pipeline (architectural; gate for reactor-split, rename-2/3) |

### Track C -- IKE/IPsec
| Order | Spec | Status |
|-------|------|--------|
| C1 | spec-ipsec-13-rekey-wire closure | see 0.1 |
| C2 | spec-ipsec-12-esn | ready (use vendored binapi/ipsec per 2026-07-10 note) |
| C3 | spec-ike-ppk | design |
| C4 | spec-ipsec-11-mobike | design |
| C5 | spec-ike-post-quantum | moved to `plan/future/` 2026-08-29 |

### Track D -- Routing protocols (non-BGP)
| Order | Spec | Status |
|-------|------|--------|
| D1 | spec-mpls-10-rsvp-te-reload-completeness | ready |
| D2 | spec-mpls-9-rsvp-te-one-to-one-backup | ready |
| D3 | spec-ospf-ext-13-l3vpn-dn-bit | ready (ospf-ext-0 umbrella coordinates) |
| D4 | spec-srv6-ebgp-egress-filter | ready |
| D5 | spec-srv6-* remainder (bestpath-resolvability, evpn-label-width, labeled-unicast, static-segments) | skeletons -> design |

### Track E -- Platform, mgmt, appliance
| Order | Spec | Status |
|-------|------|--------|
| E1 | spec-hardware-watchdog | design (note ze-platform-vet split obligation) |
| E2 | spec-ssh-fido2-keys | design |
| E3 | spec-firewall-dynamic-address-group | design |
| E4 | spec-pki-full-chain | closed 2026-09-03: web + DoT/DoH chain serving. Design is now `docs/architecture/pki/tls-listeners.md`; the looking-glass follow-up, spec-lg-pki-certificate, closed 2026-09-04 into that same page |
| E5 | spec-router-advertisement | ready (2026-07-10): send-side RA |
| E6 | spec-install-9-cloud-init | design (RESEARCH phase noted) |
| E7 | spec-kernel-lockdown-hardening | design (explicitly not scheduled; park until asked) |
| E8 | spec-ntp-server, spec-dhcpv6-server | both moved to `plan/future/` 2026-08-29 |
| E9 | spec-managed-server-hardening + fleet set (1 -> 2,3,4 -> 5; 6 -> 7) | fleet-1..7 moved to `plan/future/` 2026-08-29; spec-managed-server-hardening stays in `plan/` |

### Track F -- CLI/web surface
| Order | Spec | Status |
|-------|------|--------|
| F1 | spec-cmd-deprecation | design |
| F2 | spec-command-completion | design |
| F3 | spec-mgmt-version-header-suppress | (also listed 1.4; whichever session gets there first) |

### Track G -- Data plane analytics
| Order | Spec | Status |
|-------|------|--------|
| G1 | spec-anomaly-3-observe | ready |
| G2 | spec-anomaly-5-entity-matrix + spec-anomaly-6-as-enrichment | ready (parallel to each other) |
| G3 | spec-anomaly-7-as-entities-cohorts | ready once 5+6 land |
| G4 | spec-flow-export-3-sampled-scale | moved to `plan/future/` 2026-08-29 |
| G5 | spec-anomaly-0-umbrella / spec-cp-survival-0-umbrella | umbrella coordination + closure hygiene |

### Track H -- In-flight completions (resume before anything new in their areas)
| Order | Spec | Status |
|-------|------|--------|
| H1 | spec-route-config-plugin-migration | in-progress 5/8 |
| H2 | spec-tiers-0-umbrella | in-progress 4/6 |
| H3 | spec-perf-next-1/2/3 (+umbrella) | in-progress 1/5 |
| H4 | spec-fib-depth | in-progress 7/12 |
| H5 | spec-release-evidence-gate | in-progress (unblocked 2026-07-10; needs capable host for the matrix run) |
| H6 | spec-improve-0..6 set | skeletons -> design pipeline |
| H7 | spec-finish-report-bus / spec-finish-ci-coverage | skeletons (report-bus starts with the L103 repro re-run; watchdog may have changed the symptom) |
| H8 | spec-finish-l2tp | skeleton; WAIT for spec-followup-l2tp-call to close (same component) |
| H9 | spec-release-audit-0/1/2/8 set | design; release-time work, sequence behind release-evidence-gate |
| H10 | spec-rename-0/1 | ready (rename-1 mechanical); rename-2/3 stay blocked on rib-arch |

## 3. Blocked ledger

| Spec | Blocked on | Re-check when |
|------|-----------|---------------|
| spec-fib-depth-4-srv6 | spec-fib-depth completion + bgp-nlri-srv6 | fib-depth closes |
| spec-rename-2-bgp-packet | rib-arch set | rib-arch-0 designed |
| spec-rename-3-wireu-fold | rename-2 | rename-2 closes |
| spec-reactor-split | rib-arch set | moved to `plan/future/` 2026-08-29; still gated on rib-arch-0 when it returns |
| spec-review-bus-async-fanout | spec-unify-buffer-lifetime ordering note | before scheduling |
| spec-anomaly-7 | anomaly-5 + anomaly-6 | both close |

## 4. Design pipeline (skeletons to bring to ready, in likely-need order)

gr-advanced, vrf, improve-5-panic-boundaries, improve-6-yang-coverage, finish-report-bus,
finish-ci-coverage, rib-arch-1 (opens the rib-arch set), fleet-1-device-registry,
ntp-server, dhcpv6-server, srv6 set, ike-post-quantum, radius-admin set,
flow-export-3, managed-server-hardening, improve-1/2/3/4, remaining rib-arch children.

Added 2026-07-10 (osvbng comparison refresh; user-selected items): dhcp-pool-options,
radius-subscriber-attributes, radius-acct-timewheel (investigation first),
ppp-ra-refinements, vpp-host-tuning (Depends: vpp-isolated-cpus, so behind 1.2),
startup-resilience (audit first). The radius/ppp items share the l2tp component
with spec-followup-l2tp-call and spec-finish-l2tp (H8): coordinate before starting.

## 5. Cross-cutting obligations (bind all new work; from the 2026-07-10 review)

- New verify gates apply to every spec: ze-platform-vet (host/iface trees compile
  under darwin+freebsd), ./le port-defaults check (YANG port defaults vs Go table),
  ze-iface-resolution-check (no direct kernel name resolution).
- A NEW gate must be added to `stagesForMode` in `internal/le/verify/engine/run.go`
  (both branches); the the native action tables under `internal/le/` `_ze-verify-impl` list is documented dead code
  (`internal/le/` native action tables comment).
- Anything writing over pkg/plugin/rpc inherits the 30s write-deadline/watchdog.
- New VPP work uses vendored govpp binapi (pattern: vendor/go.fd.io/govpp/binapi).
- New TLS listeners should plan for pki-full-chain integration (E4).
