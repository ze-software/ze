# Spec: LDP and RSVP-TE one-off defects

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Fixes remaining shipped defects in LDP default transport-address advertisement in Hellos, completes the owed `TestLDPInteropFRR` QEMU guest proof, and fixes two RSVP-TE native MPLS egress lab fixture defects |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Split out of `plan/release/fixes/spec-network-services-one-off-defects.md` on 2026-11-26 so the routing-protocols door (`ai/doors/routing-protocols.md`) LDP and RSVP-TE defects live in one focused release-fix spec with zero duplicate AC or journal ownership.

This spec owns three open items (four journal rows):
1. **LDP `TestLDPInteropFRR` rerun (`internal/plugins/ldp/frr_interop_integration_linux_test.go`):** After `a42919934f` added `iface.EnsureBackend` so an LDP-only config loads the interface backend, the QEMU guest proof against FRR `ldpd` was never re-run and verified green.
2. **LDP default `transport-address` (`internal/plugins/ldp/register.go` `parseLDPConfig`, `sendHello`, `ldpSessionDialer`):** When `ldp { transport-address }` is omitted, `cfg.TransportAddr` stays the zero `netip.Addr{}`, so Ze's Hellos omit the Transport Address TLV. Per RFC 5036 Section 2.5.2 and FRR's `ldpd` (which defaults the IPv4 transport address to the LSR-ID / router-id), when `transport-address` is omitted and `lsr-id` is a valid IPv4 address, Ze's Hellos carry the LSR-ID as the Transport Address TLV (matching FRR) while `ldpSessionDialer` binds the TCP source address only when `transport-address` was explicitly configured (or when the defaulted LSR-ID is held locally on the host) so an LSR-ID that is not assigned to a local interface does not break outbound TCP dials.
3. **RSVP-TE native MPLS egress lab fixtures (`internal/plugins/rsvpte/producer_integration_linux_test.go` `newRSVPProducerLab`, `requirePush`):** (a) `newRSVPProducerLab` disables reverse-path filtering on links but leaves inherited strict `rp_filter` on `lo`, causing loose egress packets to be dropped on loopback (`IPReversePathFilter` rose from 0 to 3); (b) `requirePush` compares the stored frame budget against the label-adjusted inner IP limit instead of asserting the exact 1500-byte frame budget and reporting the actual owner and MTU (RFC 3209 Section 2.6 separates Path MTU from Path MTU minus label overhead).

## Work Not Done

- RSVP-TE refresh-period slew ratio (`adoptedRefreshPeriod`, `plan/journal/setting-changed-faster-than-its-consumer-allows.md` 2026-09-05): already owned by `plan/release/fixes/spec-rsvp-refresh-timing.md`; duplicate ownership removed here.
- Network services plugins (flow export, DHCP/TFTP server, authoritative DNS, `resolve ping`): remain in `plan/release/fixes/spec-network-services-one-off-defects.md`.
- Interface, netlink monitor, DHCP client, VPP tunnels, traffic control, and kernel MPLS egress pop: split into `plan/release/fixes/spec-network-services-1-iface-dataplane-defects.md`.
- Firewall and policy-route defects: split into `plan/release/fixes/spec-network-services-2-firewall-policyroute-defects.md`.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/routing-protocols.md`, `ai/doors/forwarding.md`
- [ ] `docs/architecture/ldp/mpls-ldp.md`
- [ ] `docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md`
- [ ] `docs/architecture/mpls/mpls-kernel.md`
- [ ] `docs/architecture/testing/interop.md`
- [ ] `ai/rules/rfc-compliance.md`, `ai/rules/interop-and-goal-validation.md`, `ai/rules/platform-linux.md`

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc5036.txt` Section 2.5.2 (Transport Address TLV in Basic Discovery Hellos) and `rfc/short/rfc5036.md` row `RFC5036-2.5.2-1`
- [ ] `rfc/full/rfc3209.txt` Section 2.6 (Path MTU vs label stack overhead)

## Current Behavior (MANDATORY)

All producers read at HEAD on 2026-11-26:
- [ ] `internal/plugins/ldp/register.go` - `parseLDPConfig` (lines 169-175) leaves `cfg.TransportAddr` zero when `transport-address` is absent, even when `cfg.LSRID` is configured; `sendHello` therefore sends Basic Discovery Hellos without a Transport Address TLV; `ldpSessionDialer` (lines 515-521) binds `LocalAddr` whenever `localTransport.IsValid()`.
- [ ] `internal/plugins/ldp/frr_interop_integration_linux_test.go` - `TestLDPInteropFRR` was fixed in `a42919934f` via `iface.EnsureBackend` but its QEMU guest rerun under `./le test qemu run packages frr` remains unrecorded.
- [ ] `internal/plugins/rsvpte/producer_integration_linux_test.go` - `newRSVPProducerLab` (line 428) omits explicit `rp_filter=0` on `lo`; `requirePush` (line 903) checks the label-adjusted inner IP limit instead of the fixture's exact 1500-byte frame budget and omits actual owner/MTU reporting.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| LDP config parse & Hello Transport Address TLV | `internal/plugins/ldp/register.go` `parseLDPConfig`, `sendHello`, `ldpSessionDialer`; `wire.go` `encodeHello` | Encodes IPv4 Transport Address TLV when `cfg.TransportAddr.IsValid()` and binds TCP source in `ldpSessionDialer` when `localTransport.IsValid()` | Extends: AC-2 splits advertised Hello transport address (defaulting to `cfg.LSRID` per RFC 5036 §2.5.2 / FRR `ldpd`) from explicit TCP bind address (binding `LocalAddr` when configured or when the defaulted `lsr-id` is assigned locally) |
| LDP FRR interop harness | `internal/plugins/ldp/frr_interop_integration_linux_test.go` `TestLDPInteropFRR`; `test/interop/scenarios/ldp-backoff-clear-frr/` | Runs Ze LDP against real FRR `ldpd` in Ze's QEMU guest and Docker interop suite | Extends: AC-1 verifies `TestLDPInteropFRR` in Ze's QEMU guest, and AC-2 adds `test/interop/scenarios/ldp-default-transport-address-frr/` proving FRR accepts Ze's defaulted LSR-ID Transport Address TLV |
| RSVP-TE native producer integration lab | `internal/plugins/rsvpte/producer_integration_linux_test.go` `newRSVPProducerLab`, `requirePush` | Exercises native RSVP-TE FRR/egress forwarding in isolated Linux netns | Extends: AC-3 sets `net.ipv4.conf.lo.rp_filter=0` explicitly in `newRSVPProducerLab` and asserts the exact 1500-byte frame budget with owner/MTU diagnostics in `requirePush` |

## Data Flow (MANDATORY)

### Entry Point
1. LDP config parse (`parseLDPConfig`) -> Basic Discovery Hello transmission (`discoverOnInterface` / `sendHello`) -> outbound TCP session establishment (`startSessionForAdj` / `ldpSessionDialer`).
2. QEMU Linux integration tests `TestLDPInteropFRR` and `TestRSVPNativeProducer` (`//go:build linux`, `userns.Enter`).

### Transformation Path
1. **LDP `TestLDPInteropFRR` rerun (AC-1):** Run `./le test qemu run packages frr command "go test -tags integration -run TestLDPInteropFRR ./internal/plugins/ldp"` in Ze's QEMU guest and fix any remaining guest failure if exposed.
2. **LDP default `transport-address` (AC-2):** Keep `cfg.TransportAddr` as the operator's explicit `transport-address` (zero when omitted) and add a helper `effectiveHelloTransportAddr(cfg ldpConfig) netip.Addr` returning `cfg.TransportAddr` when valid, else `cfg.LSRID` when valid IPv4. `sendHello` encodes `effectiveHelloTransportAddr(cfg)` in the IPv4 Transport Address TLV (RFC 5036 §2.5.2: "An LSR MUST advertise the same transport address in all Hellos that advertise the same label space"). For TCP session dialing, `effectiveDialSourceAddr(cfg)` binds `cfg.TransportAddr` when explicitly configured, and when omitted binds `cfg.LSRID` only if that IP is currently assigned on a local interface (checked via `net.InterfaceAddrs`), otherwise letting the OS choose the source address so a non-local LSR-ID never breaks outbound TCP dials. Quote RFC 5036 Section 2.5.2 above the helper and prove with unit tests + `test/interop/scenarios/ldp-default-transport-address-frr/` against FRR `ldpd`.
3. **RSVP-TE native egress lab fixtures (AC-3):** In `newRSVPProducerLab`, write `net.ipv4.conf.lo.rp_filter = 0` alongside the link `rp_filter` sysctls so loose egress packets re-injected on loopback are not dropped by inherited strict host defaults. In `requirePush`, assert the fixture's exact 1500-byte frame budget (matching `buildRichRoute` and RFC 3209 §2.6) and include the route's actual protocol owner and MTU in any assertion failure message; prove green in Ze's QEMU guest.

### Boundaries Crossed
- Wire boundary: LDP Basic Discovery UDP Hello packets (`224.0.0.2:646`) now carry the IPv4 Transport Address TLV set to `lsr-id` when `transport-address` is unconfigured.

### Changed-Wire Census

**Search evidence:** Searched `internal/plugins/ldp/` and `test/` with `git grep` for `sendHello`, `EncodeHello`, `DecodeHello`, `TransportAddr`, `0x0401`, and `TLVTransport`. Positive controls confirmed: writer `internal/plugins/ldp/register.go:995` (`sendHello`) -> `internal/plugins/ldp/wire.go:393,417` (`EncodeHello`, encoding `TLVTypeIPv4Transport = 0x0401` defined at `wire.go:60`) and reader `internal/plugins/ldp/wire.go:442,460` (`DecodeHello`) -> `internal/plugins/ldp/discovery.go:120`; no other LDP Hello encoder or decoder exists in the repository.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| LDP Basic Discovery Hello PDU (UDP 646) | writer | `internal/plugins/ldp/register.go:995` (`sendHello`) -> `internal/plugins/ldp/wire.go:393,417` (`EncodeHello`, `TLVTypeIPv4Transport = 0x0401`) | `parseLDPConfig` leaves `cfg.TransportAddr` zero when `transport-address` is omitted, so `sendHello` passes an invalid `netip.Addr` to `EncodeHello` and omits TLV `0x0401` | Change: populate `HelloMessage.TransportAddr` via `effectiveHelloTransportAddr(cfg)` (explicit `cfg.TransportAddr` if set, else `cfg.LSRID` when valid IPv4 per RFC 5036 §2.5.2) so Basic Discovery Hellos always carry TLV `0x0401` |
| LDP Basic Discovery Hello PDU (UDP 646) | reader | External peer FRR `ldpd` (`test/interop/scenarios/ldp-default-transport-address-frr/`), `internal/plugins/ldp/wire.go:442,460` (`DecodeHello`), `internal/plugins/ldp/discovery.go:120`, `internal/plugins/ldp/register_test.go`, `internal/plugins/ldp/rfc5036_sessionparams_test.go:453` | `DecodeHello` already parses TLV `0x0401` into `HelloMessage.TransportAddr`; FRR `ldpd` reads TLV `0x0401` to select the TCP session endpoint | Preserve decoder unchanged; add unit tests and FRR interop scenario asserting TLV `0x0401` equals `lsr-id` on the wire and establishes an operational LDP session with FRR |

### Integration Points
- `test/interop/scenarios/ldp-default-transport-address-frr/` (FRR `ldpd` interop scenario for AC-2)
- `docs/architecture/ldp/mpls-ldp.md` (document defaulted Hello transport address and dial source selection)

### Architectural Verification
- [ ] Wire-visible LDP Hello change (AC-2) has a named FRR interop scenario under `test/interop/scenarios/ldp-default-transport-address-frr/` proven red with the fix reverted
- [ ] RFC 5036 Section 2.5.2 is quoted verbatim above `effectiveHelloTransportAddr` and named at its caller
- [ ] All kernel integration tests in `internal/plugins/ldp/` and `internal/plugins/rsvpte/` carry `//go:build linux` and `userns.Enter`

## Risks & Assumptions

### Assumptions
- Per `ai/META-DECISIONS.md`, FRR's `ldpd` default (using LSR-ID as the default IPv4 transport address when `transport-address` is omitted) settles the default behavior without requiring a new owner question.

### Risks
- If a test sets `lsr-id` to an RFC 5737 address not assigned to any local interface and expects outbound TCP dial to succeed, binding `LocalAddr` unconditionally to `lsr-id` would fail with `EADDRNOTAVAIL`. Checking local assignment before binding `LocalAddr` when `transport-address` is omitted avoids breaking such setups while still matching explicit `transport-address` binding semantics when configured.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| LDP config with `lsr-id` and no `transport-address` | → | `parseLDPConfig` -> `effectiveHelloTransportAddr` -> `sendHello` -> `ldpSessionDialer` | `TestLDPHelloDefaultsTransportAddressToLSRID`, `test/interop/scenarios/ldp-default-transport-address-frr/` |
| LDP-only config in Ze's QEMU guest against FRR `ldpd` | → | `iface.EnsureBackend` + `discoverOnInterface` + `runSession` | `TestLDPInteropFRR` |
| Native RSVP-TE MPLS egress lab in Ze's QEMU guest | → | `newRSVPProducerLab` (`lo` `rp_filter=0`) + `requirePush` (1500-byte frame budget) | `TestRSVPNativeProducer` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `TestLDPInteropFRR` rerun (old AC-23 part 1): `TestLDPInteropFRR` runs in Ze's QEMU guest (`./le test qemu run packages frr command "go test -tags integration -run TestLDPInteropFRR ./internal/plugins/ldp"`) after `a42919934f` | `TestLDPInteropFRR` loads the default interface backend via `iface.EnsureBackend`, establishes an operational LDP session with FRR `ldpd`, exchanges label bindings, and passes in Ze's QEMU guest | None; any remaining guest failure exposed by the rerun must be fixed before closing AC-1 |
| AC-2 | LDP `transport-address` default (old AC-23 part 2): LDP is configured with a valid IPv4 `lsr-id` and `transport-address` omitted (local or non-local `lsr-id`), or with an explicit `transport-address` | Basic Discovery Hellos encode IPv4 Transport Address TLV (`0x0401`) equal to `lsr-id` when `transport-address` is omitted and equal to the explicit `transport-address` when configured (RFC 5036 §2.5.2); outbound TCP dials bind `LocalAddr` when `transport-address` is explicit or the defaulted `lsr-id` is held locally, and delegate source-address selection to the OS when an unconfigured `lsr-id` is non-local; proven by unit tests and `test/interop/scenarios/ldp-default-transport-address-frr/` | Non-local defaulted `lsr-id` values remain permitted to dial using OS-selected source addresses so RFC 5737 / loopback-unassigned test topologies stay dialable; explicit `transport-address` retains strict `LocalAddr` binding |
| AC-3 | RSVP-TE egress lab fixtures (old AC-24 parts 2 & 3): `TestRSVPNativeProducer` runs in Ze's QEMU guest under strict host `rp_filter` defaults | `newRSVPProducerLab` sets `net.ipv4.conf.lo.rp_filter=0` explicitly so loopback egress packets forward cleanly, `requirePush` asserts the exact 1500-byte frame budget (RFC 3209 §2.6) and reports actual route owner and MTU on mismatch, and `TestRSVPNativeProducer` passes in Ze's QEMU guest | Per-link `rp_filter=0` setup on lab veth links and existing MPLS push/swap/pop assertions in `TestRSVPNativeProducer` remain intact |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| `TestLDPHelloDefaultsTransportAddressToLSRID`, `TestLDPDialerSkipsBindForNonLocalDefaultedLSRID`, `TestLDPExplicitTransportAddressOverridesDefault` | `internal/plugins/ldp/register_test.go` | AC-2 |

### Functional Tests

| Test | AC |
|------|----|
| QEMU guest: `./le test qemu run packages frr command "go test -tags integration -run TestLDPInteropFRR ./internal/plugins/ldp"` | AC-1 |
| Interop scenario `test/interop/scenarios/ldp-default-transport-address-frr/` (`./le test integration interop` with `INTEROP_SCENARIO=ldp-default-transport-address-frr`) | AC-2 |
| QEMU guest: `./le test qemu run packages rsvpte command "go test -tags integration -run TestRSVPNativeProducer ./internal/plugins/rsvpte"` | AC-3 |

## Files to Modify
- `internal/plugins/ldp/register.go`
- `internal/plugins/ldp/register_test.go`
- `internal/plugins/ldp/frr_interop_integration_linux_test.go` (only if guest rerun exposes a defect)
- `internal/plugins/rsvpte/producer_integration_linux_test.go`
- `docs/architecture/ldp/mpls-ldp.md`

## Files to Create
- `test/interop/scenarios/ldp-default-transport-address-frr/` (FRR interop scenario for AC-2)

### Integration Checklist
- [ ] `TestLDPInteropFRR` green in Ze's QEMU guest
- [ ] `ldp-default-transport-address-frr` green against FRR `ldpd` and proven red with the default reverted
- [ ] `TestRSVPNativeProducer` green in Ze's QEMU guest

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/architecture/ldp/mpls-ldp.md` (document default Hello Transport Address TLV from `lsr-id` and dialer source-binding behavior)

## Implementation Steps
1. **AC-1 (`TestLDPInteropFRR`):** Run `TestLDPInteropFRR` in Ze's QEMU guest and record the green verdict (fixing any defect if it fails).
2. **AC-2 (LDP `transport-address` default):** Implement `effectiveHelloTransportAddr` and `effectiveDialSourceAddr` in `internal/plugins/ldp/register.go` with RFC 5036 §2.5.2 quote, write red-first unit tests in `register_test.go`, create and prove `test/interop/scenarios/ldp-default-transport-address-frr/`, and update `docs/architecture/ldp/mpls-ldp.md`.
3. **AC-3 (RSVP-TE lab fixtures):** Set `net.ipv4.conf.lo.rp_filter=0` in `newRSVPProducerLab` and fix `requirePush` to assert the 1500-byte frame budget with owner/MTU reporting in `internal/plugins/rsvpte/producer_integration_linux_test.go`; prove green in Ze's QEMU guest (obtain `ze-test-judge` approval for the test-predicate correction in `requirePush`).
4. Close all 4 owned journal rows in `plan/journal/`.

## Review Gate
- [ ] `/ze-review` clean on all modified Go, interop, and doc files
- [ ] `/ze-review-spec` confirms AC-1 through AC-3, Changed-Wire Census, FRR interop, and doc updates are met
- [ ] Independent `ze-test-judge` approval recorded for the `requirePush` test-assertion correction

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | The RFC, then ExaBGP API compatibility, then ExaBGP's behavior, in that order | agrees | AC-2 follows RFC 5036 §2.5.2 and FRR `ldpd` default transport-address semantics, verified by a real FRR interop scenario |
| `ai/doors/routing-protocols.md` | RFC and interop obligations | the door covering `internal/plugins/ldp/register.go` and `internal/plugins/rsvpte/producer_integration_linux_test.go` | OSPF, OSPFv3, IS-IS, LDP and BFD scenarios against FRR and BIRD live in `test/interop/scenarios/`, named for the protocol, the case and the peer. | agrees | AC-2 adds `test/interop/scenarios/ldp-default-transport-address-frr/` and proves it red with the fix reverted |
| `docs/architecture/ldp/mpls-ldp.md` | Trap: an LDP-only config loads no iface backend | the `// Design:` page of `internal/plugins/ldp/register.go`, `register_test.go`, `frr_interop_integration_linux_test.go`; a page this spec modifies | `EnsureBackend` loads the build-time default backend and does nothing when one is already loaded, so an explicit `interface { backend ... }` still wins. | agrees | AC-1 completes the QEMU guest verification of that `EnsureBackend` fix (`TestLDPInteropFRR`); AC-2 extends this page with the Hello transport-address default |
| `docs/architecture/iface/management.md` | The resolver fan-out discards the oldest, never the newest | anchored to `internal/plugins/ldp/register.go` | This section describes resolver fan-out queue overflow behavior and `ldpInterfaceRetry`, which this spec does not touch. | silent | AC-1 reruns `TestLDPInteropFRR` and AC-2 only changes Hello transport-address defaulting and dial source selection; discovery retry and resolver fan-out are unchanged |
| `docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md` | Decision: local repair moves data and control traffic | the `// Design:` page of `internal/plugins/rsvpte/producer_integration_linux_test.go` | Kernel MTU handling must be checked separately from small-packet push, swap and pop behavior. | agrees | AC-3 fixes `requirePush` to assert the exact 1500-byte frame budget and `newRSVPProducerLab` to configure `lo` `rp_filter=0` explicitly |

## Design presentation

### What changes
- Runs and verifies `TestLDPInteropFRR` in Ze's QEMU guest after `a42919934f`.
- Defaults LDP Basic Discovery Hello IPv4 Transport Address TLV to `lsr-id` when `transport-address` is omitted (RFC 5036 §2.5.2, matching FRR `ldpd`), while binding the TCP dial source only when `transport-address` is explicitly configured or the defaulted `lsr-id` is assigned locally, proven by units and `test/interop/scenarios/ldp-default-transport-address-frr/`.
- Fixes `newRSVPProducerLab` (`lo` `rp_filter=0`) and `requirePush` (exact 1500-byte frame budget with owner/MTU diagnostics) in `internal/plugins/rsvpte/producer_integration_linux_test.go`.

### Before and after
- **Before:** Omitting `ldp transport-address` sends Hellos with no Transport Address TLV; `TestLDPInteropFRR` has not been re-verified in Ze's QEMU guest since `a42919934f`; `newRSVPProducerLab` drops loose egress packets on `lo` under inherited strict `rp_filter`, and `requirePush` checks the label-adjusted inner IP limit instead of the stored 1500-byte frame budget.
- **After:** LDP Hellos advertise `lsr-id` as the default Transport Address TLV with safe dial source handling, proven against FRR `ldpd`; `TestLDPInteropFRR` and `TestRSVPNativeProducer` both pass in Ze's QEMU guest with explicit prerequisites.

### Alternatives set aside
- **Unconditionally binding TCP `LocalAddr` to the defaulted `lsr-id`:** set aside because an operator or test whose `lsr-id` is not assigned on a local interface would fail every outbound dial with `EADDRNOTAVAIL`; checking local interface addresses preserves explicit `transport-address` binding while keeping unconfigured non-local `lsr-id` setups dialable.
- **Keeping `adoptedRefreshPeriod` here:** set aside because `plan/release/fixes/spec-rsvp-refresh-timing.md` already owns that RFC 2205 §3.7 item 5 work and its journal row.

### Documents departed from
None. `docs/architecture/ldp/mpls-ldp.md` is extended in the same commit to document the defaulted Hello Transport Address TLV.

### Choices for Thomas
none

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `test-against-broken-path` | 2026-08-24 | `TestLDPInteropFRR` rerun owed |
| `zero-value-as-valid-answer` | 2026-09-21 | LDP `parseLDPConfig` zero `transport-address` |
| `test-against-broken-path` | 2026-09-22 | `newRSVPProducerLab` loopback rp_filter |
| `test-against-broken-path` | 2026-09-22 | `requirePush` frame budget |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (red recorded per AC)
- [ ] Tests PASS
- [ ] Scoped runs: `./le test unit pkg path ./internal/plugins/ldp/`, `./le test qemu run packages frr command "go test -tags integration -run TestLDPInteropFRR ./internal/plugins/ldp"`, `./le test integration interop` (`ldp-default-transport-address-frr`), `./le test qemu run packages rsvpte command "go test -tags integration -run TestRSVPNativeProducer ./internal/plugins/rsvpte"`
