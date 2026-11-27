# Spec: firewall-dynamic-set-config-fidelity

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Priority | P0 |
| Release reason | Accepted firewall set flags, CIDR interval elements, and term `tcp-flags` matching must match their YANG schema and reach the kernel faithfully or be refused at verify instead of silently truncating or failing validation. |
| RFC gaps | - |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Close the remaining configuration-fidelity gaps on firewall named sets and firewall term matching after the initial set-flag lowering (`flags-dynamic`, `flags-timeout`, `flags-constant`) already landed in HEAD:

1. **CIDR prefix elements in `flags-interval` sets (`plan/journal/validated-value-discarded-by-its-caller.md`, 2026-11-07):** `encodeSetElementKey` (`internal/plugins/firewall/nft/lower_linux.go`) parses a CIDR prefix and discards its prefix length, programming only the bare address (`p.Addr()`, with no `IntervalEnd`), so `element 192.0.2.0/24` matches `192.0.2.0` alone. Lower IPv4 and IPv6 CIDR prefixes (and bare host addresses) in operator-configured `flags-interval` sets to sorted half-open `[start, exclusiveEnd)` intervals with `IntervalEnd`, refuse CIDR prefix elements on sets without `flags-interval`, and remove the one-term-per-prefix workaround from `docs/guide/migrating-from-vyos.md`.
2. **Firewall term `tcp-flags` YANG declaration (`plan/journal/unwired-feature.md`, 2026-11-07):** `internal/component/firewall/config.go` (`parseFromBlock`, `ParseTCPFlags`), `internal/plugins/firewall/nft/lower_linux.go` (`lowerTCPFlagsMatch`), `internal/plugins/firewall/vpp/verify.go` / `translate.go`, and `docs/guide/firewall.md` all support `tcp-flags`, but `grouping from-block` in `internal/component/firewall/yang/ze-firewall-conf.yang` declares no `tcp-flags` leaf, so `tcp-flags syn` is rejected by schema validation. Declare `typedef tcp-flags` and `leaf tcp-flags` in `from-block` (with a YANG-to-Go parity test), and add a `meta l4proto tcp` guard in `lowerTCPFlagsMatch` so a term omitting `protocol tcp` only inspects TCP headers.
3. **Verify-time set-flag and element validation with `Ze always starts` startup isolation (`internal/component/firewall/model.go`, `engine.go`, `register_producer_verify.go`):** `Set.Validate()` currently checks only set name and non-zero type, and `p.OnConfigure` (`engine.go:383`) returns validation errors directly, which exits `ze start` with code 1 (`cmd/ze/hub/main.go:1728-1741`). Enforce `opererr` refusals in `Set.Validate()` for contradictory set flags (`flags-constant` with `flags-dynamic` or `flags-timeout`), per-element `timeout` without `flags-timeout`, `IntervalEnd` without `flags-interval`, CIDR prefix elements without `flags-interval`, non-canonical prefixes with host bits set, family-mismatched IP elements, and overlapping intervals in unexpanded `flags-interval` sets. Keep `ze config validate`, commit, and reload (`VerifyConfig`, `OnConfigVerify`) strictly refusing; at `ze start`, register `"firewall"` via `config.RegisterServiceBlock` and handle validation/parse failures inside `p.OnConfigure` by leaving the firewall unstarted (applying no approximation), logging an `ERROR` record under `diagnostic.CodeConfigPluginVerify` (`config-plugin-verify`) with the `opererr` fix, and returning `nil` so Ze and its other services keep running while `ze doctor` (`config-semantics` -> `config.ValidateSemantics` -> `firewall.VerifyConfig`) reports the same code and fix.

Do not add a packet-triggered insertion action (`expr.Dynset`); that extension stays owned by `plan/post-release/features/spec-firewall-dynamic-address-group.md`.

## Source and Evidence

Split from `plan/post-release/features/spec-firewall-dynamic-address-group.md` under the release-scope and spec-split rules in `ai/META-DECISIONS.md`. The complete original design, decisions, evidence, and implementation history are preserved under Historical Combined Specification in `plan/post-release/features/spec-firewall-dynamic-address-group.md`. Since the split, `flags-dynamic`, `flags-timeout`, and `flags-constant` lowering landed in HEAD (`internal/plugins/firewall/nft/lower_linux.go` `lowerSet`, `backend_linux.go` `applySet`, `readback_linux.go` `fillSets`, `lower_linux_test.go` `TestLowerSetTimeoutFlagAndElementTimeouts` and `TestLowerSetCarriesConstantAndDynamic`, and `test/firewall/firewall-set-element-timeout.ci`). The two 2026-11-07 journal rows owned by this spec and the missing verify-time set-flag/element checks remain open and define the active scope below.

## Work Not Done and Split Ownership

The 2026-10-22 portfolio split changes planning ownership only. Original criteria below the historical boundary in the feature spec are provenance, not a second active owner.

| Work | Implementation owner |
|------|----------------------|
| Original AC-1 through AC-7, packet-triggered insertion (`expr.Dynset`), action validation, and action-inserted entry expiry | `plan/post-release/features/spec-firewall-dynamic-address-group.md` |
| Set flag lowering fidelity (`Dynamic`/`HasTimeout`/`Constant`, already landed in HEAD), set-flag and element verify-time validation, `flags-interval` CIDR prefix lowering, and `from-block` `tcp-flags` YANG wiring | `plan/release/fixes/spec-firewall-dynamic-set-config-fidelity.md` |

Work assigned to `plan/post-release/features/spec-firewall-dynamic-address-group.md` is not performed or closed by this spec.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - Platform boundaries, exact-or-reject at verify, single YANG schema across all interfaces, operator error format.
  → Decision: A backend or schema that cannot apply a config exactly refuses it at verify with the problem, why, and exact fix (`opererr`).
  → Constraint: Never silently drop a prefix length, an element timeout, or an unsupported flag combination at apply time.
- [ ] `ai/doors/forwarding.md` - Firewall component and backend split, ownership, exact-or-reject, Linux integration gates.
  → Decision: `internal/component/firewall` owns the backend-agnostic model and `ValidateTables`; `internal/plugins/firewall/nft` lowers to `google/nftables` and `vpp` verifies/translates to GoVPP ACLs.
  → Constraint: Changes to `firewall.Set` and `lowerSet` must preserve all other table producers (`firewall-irr`, `copp`, `firewall-domain`, `policy-routes`, `ddos-local`, `flowspec`).
- [ ] `ai/doors/config-yang.md` - YANG schema registration, `InProcessConfigVerifier` parity between `ze config validate`, `ze doctor`, and daemon commit/reload.
  → Decision: `VerifyConfig` (`InProcessConfigVerifier`) and `OnConfigVerify` share `parseAndVerifyFirewallSections` -> `ValidateTables`; `config.ValidateSemantics` runs `VerifyPluginConfig` for `ze doctor` under `diagnostic.CodeConfigPluginVerify`, and `config.RegisterServiceBlock` isolates service blocks during `LoadConfigForStart`.
  → Constraint: Native YANG `pattern` constrains `tcp-flags` at Layer 1; semantic set-flag/element checks live in `Set.Validate()` so offline validate, commit, and reload refuse strictly, while `ze start` leaves an invalid firewall unstarted and reports it under `config-plugin-verify` rather than exiting the daemon.
- [ ] `ai/doors/system-plugins.md` - System plugins (`firewall/nft`, `copp`) and backend boundaries.
  → Decision: `firewall/nft` lowers `firewall.Set` and `firewall.MatchTCPFlags`; `copp` builds pre-expanded interval sets via `neighborSet`.
  → Constraint: `lowerSet` must not misinterpret a pre-expanded top-of-space interval from `copp` (such as `0.0.0.0/0` alone) as a single `/32` host address.
- [ ] `ai/doors/le-tooling.md` - Functional `.ci` tests, `option=needs-linux:caps=net-admin`, and `internal/test/fixture/netfilter_fixture.go`.
  → Decision: Netfilter `.ci` tests use registered snapshot fixtures in `internal/test/fixture/netfilter_fixture.go` and assert real kernel `nft list table` output.
  → Constraint: Offline validation `.ci` tests run without `caps=net-admin`; kernel apply `.ci` tests carry `option=needs-linux:caps=net-admin`.
- [ ] `docs/architecture/core-design.md` (sections 14a, 14b, 18, Design decisions) - Firewall abstract model, commit-time capability gate, canonical prefix refusal, backend selectors, and `Ze always starts` (2026-11-26).
  → Decision: Firewall literal prefix inputs with host bits set are refused with the canonical network rather than silently masked; `ze:backend` gates kernel-only features, while features supported on both `kernel` and `vpp` (like `tcp-flags`) carry no `ze:backend` restriction; per `Ze always starts` (`plan/records/ruling-ze-always-starts.md`), an invalid service configuration never makes `ze start` exit — that service stays down and is reported with its fix by an error log line and `ze doctor` under the same doctor code, while reload stays refused.
  → Constraint: Keep `leaf tcp-flags` unannotated by `ze:backend` because both `kernel` and `vpp` implement `MatchTCPFlags`, and handle firewall validation errors in `OnConfigure` at `ze start` without returning a fatal startup error.
- [ ] `docs/architecture/firewall/firewall-irr.md` - How `firewall-irr` populates `SetFlagInterval` sets with `SetElement.IntervalEnd`.
  → Decision: `firewall-irr` encodes prefixes as pre-expanded `{Value: start}` and `{Value: end, IntervalEnd: true}` element pairs.
  → Constraint: `lowerSet` and `Set.Validate()` must preserve pre-expanded `IntervalEnd` pairs untouched.
- [ ] `docs/architecture/firewall/fw-6-firewall-vpp.md` - VPP firewall verifier matrix.
  → Decision: VPP accepts `MatchTCPFlags` in filter chains and refuses all named sets at verify.
  → Constraint: Adding `leaf tcp-flags` in YANG immediately unlocks `tcp-flags` on both `kernel` and `vpp` with no change to VPP verifier rules.
- [ ] `docs/architecture/traffic/cp-survival-2-copp-port179.md` - CoPP neighbor interval sets (`intervalElements`).
  → Decision: CoPP builds `SetFlagInterval` sets with pre-expanded half-open boundaries and omits `IntervalEnd` when a prefix reaches the top of its address space.
  → Constraint: Mark `neighborSet` with `RawIntervals: true` so a lone top-of-space CoPP range (such as `0.0.0.0/0`) is never treated as a bare `/32` host address.

**Key insights:**
- `flags-dynamic`, `flags-timeout`, `flags-constant`, and `flags-interval` lowering ALREADY landed in HEAD (`lower_linux.go:261-290`, `backend_linux.go:319-328`, `readback_linux.go:105-116`, `lower_linux_test.go:102,1981`, `test/firewall/firewall-set-element-timeout.ci`).
- `TestLowerSetTimeoutFlagAndElementTimeouts` (`lower_linux_test.go:130`) and `TestLowerSetCarriesConstantAndDynamic` (`lower_linux_test.go:1991`) call `lowerSet` directly with unvalidated `Set` structs (`Timeout > 0` without `SetFlagTimeout`, and `SetFlagConstant | SetFlagDynamic`), so contradictory-flag and timeout-without-flag refusals belong in `Set.Validate()` (`ValidateTables`) rather than inside `lowerSet`.
- `TestParseSetElements` (`config_test.go:500`) expects `ParseFirewallConfig` to preserve element strings `"10.0.0.0/24"` and `"192.168.1.5"` in `Set.Elements`, and `cmd_show.go` / `web/page_firewall.go` read `LastApplied()`, so CIDR expansion belongs in `lowerSet` (leaving `Set.Elements` human-readable in the model).
- `encodeSetElementKey` (`lower_linux.go:1826`) calls `addr.As4()` without checking `addr.Is4()`, which panics on an IPv6 address passed into an IPv4 set; checking family explicitly fixes that panic as well.
- `docs/guide/firewall.md:600-604` currently shows `element 10.0.0.2 { timeout 3600; }` in a set without `flags-timeout;`, which must be updated alongside the new validation check.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/model.go` - Defines `Set`, `SetFlags` (`SetFlagInterval`, `SetFlagTimeout`, `SetFlagConstant`, `SetFlagDynamic`), `SetElement` (`Value`, `Timeout`, `IntervalEnd`), `MatchTCPFlags`, and `Set.Validate()` (which currently checks only `ValidateName(s.Name)` and `s.Type != 0`).
- [ ] `internal/component/firewall/config.go` - `parseSet` parses all four `flags-*` containers into `Set.Flags`; `parseSetElements` parses `element` keys and `timeout` leaf sorted lexicographically; `parseFromBlock` parses `tcp-flags` via `ParseTCPFlags` (`fin`, `syn`, `rst`, `psh`, `ack`, `urg`); `parseAddressMatch` refuses non-canonical prefixes with `opererr`.
- [ ] `internal/component/firewall/validate.go` - `ValidateTables` calls `tbl.Sets[j].Validate()` and `validateMatch` (which already refuses `MatchTCPFlags` in `FamilyARP` via `ipHeaderMatchLeaf`).
- [ ] `internal/component/firewall/engine.go` - `VerifyConfig` and `OnConfigVerify` call `parseAndVerifyFirewallSections` -> `ValidateTables`; `p.OnConfigure` (`engine.go:359-407`) currently calls `parseConfigSections` and `validateConfiguredFirewallTables` and returns any error directly, which causes `apiServer.WaitForStartupComplete` (`cmd/ze/hub/main.go:1728-1741`) to exit `ze start` with code 1.
- [ ] `internal/component/firewall/register_producer_verify.go` - Registers `firewall-producer-tables` via `config.RegisterTreeValidator`, and does not yet register `"firewall"` via `config.RegisterServiceBlock`.
- [ ] `internal/component/config/service_blocks.go` and `validate_semantic.go` - `RegisterServiceBlock` isolates registered service blocks in `LoadConfigForStart` (pruning invalid blocks and reporting them via `reportServiceNotStarted`); `ValidateSemantics` runs `VerifyPluginConfig` -> `firewall.VerifyConfig` for `ze doctor`'s `config-semantics` check under `diagnostic.CodeConfigPluginVerify`.
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` - Declares `flags-interval`, `flags-timeout`, `flags-constant`, `flags-dynamic`, and `list element` (whose descriptions still document silent prefix-length stripping and silent timeout dropping); `grouping from-block` omits `leaf tcp-flags`.
- [ ] `internal/plugins/firewall/nft/lower_linux.go` - `lowerSet` lowers all four flags (`Interval`, `HasTimeout`, `Constant`, `Dynamic`) and calls `encodeSetElementKey`, which falls back to `netip.ParsePrefix` and returns bare `p.Addr()` without `IntervalEnd` (and calls `As4()`/`As16()` without checking address family); `lowerTCPFlagsMatch` loads transport-header byte 13 without a `meta l4proto tcp` guard.
- [ ] `internal/plugins/firewall/nft/backend_linux.go` - `applySet` calls `lowerSet(t, s)` and `b.conn.AddSet(nftSet, elements)`.
- [ ] `internal/plugins/firewall/nft/readback_linux.go` - `fillSets` reads back all four flags (`Interval`, `HasTimeout`, `Constant`, `Dynamic`), `IntervalEnd`, and `Timeout`.
- [ ] `internal/plugins/firewall/nft/cmd_show.go` - `handleShowFirewallRuleset` and `handleShowFirewallGroup` read `firewall.LastApplied()` and print `Set.Elements` directly.
- [ ] `internal/plugins/firewall/vpp/verify.go` and `translate.go` - VPP already accepts `MatchTCPFlags` in filter chains and lowers it with `Proto = IP_API_PROTO_TCP`, `TCPFlagsValue`, and `TCPFlagsMask`, and rejects all named sets.
- [ ] `internal/plugins/copp/translate.go` - `neighborSet` builds pre-expanded `SetFlagInterval` sets via `intervalElements`, omitting `IntervalEnd` when a prefix reaches the top of its address space.
- [ ] `internal/component/firewall/plugins/irr/sets.go` - `familySet` builds pre-expanded `SetFlagInterval` sets via `prefixesToIntervalElements`.
- [ ] `internal/component/web/page_firewall.go` - Renders `Set.Flags` and `len(s.Elements)` from `LastApplied()`.
- [ ] `docs/guide/firewall.md` - Documents `tcp-flags` in `from-block` and VPP compatibility, and shows a Named Sets example with `element 10.0.0.2 { timeout 3600; }` missing `flags-timeout;`.
- [ ] `docs/guide/migrating-from-vyos.md` - Carries a conditional one-term-per-prefix workaround for `network-group` pointing at this spec and a note saying firewall YANG has no `tcp-flags` leaf.

**Behavior to preserve:**
- Already-landed `SetFlagInterval`, `SetFlagTimeout`, `SetFlagConstant`, and `SetFlagDynamic` parsing, lowering in `lowerSet`, readback in `fillSets`, and `firewall-set-element-timeout.ci`.
- Static named sets without optional flags (`firewall-match-in-set-addr.ci`, `firewall-match-in-set-port.ci`) and `ParseFirewallConfig` preserving operator element strings in `Set.Elements` (`TestParseSetElements`).
- Pre-expanded `SetFlagInterval` sets built by `firewall-irr` and `copp` (including CoPP's top-of-space open-ended prefix `255.255.255.0/24` and `0.0.0.0/0`).
- Existing `lowerSet` direct unit tests (`TestLowerSetTimeoutFlagAndElementTimeouts`, `TestLowerSetCarriesConstantAndDynamic`), `policy-tcp-flags.ci`, and VPP `MatchTCPFlags` behavior.
- No packet-triggered set-insertion action (`expr.Dynset`) is exposed or claimed.

**Behavior to change:**
- `Set.Validate()` rejects contradictory set flags (`flags-constant` with `flags-dynamic` or `flags-timeout`), element `timeout` without `flags-timeout`, `IntervalEnd` without `flags-interval`, CIDR prefixes without `flags-interval`, non-canonical CIDR prefixes (host bits set), IP family mismatches, and overlapping intervals in unexpanded `flags-interval` sets.
- `p.OnConfigure` (`internal/component/firewall/engine.go`) complies with `Ze always starts`: when `parseConfigSections` or `validateConfiguredFirewallTables` fails at `ze start`, it leaves the firewall unstarted (applies no partial/approximated rules and stores an idle `activeCfg` so a later valid reload recovers), logs an `ERROR` record carrying `code=config-plugin-verify` (`diagnostic.CodeConfigPluginVerify`) and the `opererr` fix, and returns `nil` so Ze and its other services stay up while `ze doctor` (`config-semantics` -> `config.ValidateSemantics` -> `firewall.VerifyConfig`) reports the same code and fix. `VerifyConfig` (`ze config validate`), commit, and `OnConfigVerify` (reload) stay strictly refusing. `register_producer_verify.go` also registers `config.RegisterServiceBlock(ConfigRoot, "firewall")`.
- `encodeSetElementKey` rejects CIDR prefixes instead of stripping prefix lengths, and checks IPv4/IPv6 family before `As4()`/`As16()`.
- `lowerSet` expands unexpanded IPv4/IPv6 `flags-interval` sets (CIDR prefixes and bare host addresses) into numerically sorted half-open `[start, exclusiveEnd)` nftables elements with `IntervalEnd: true` (omitting `IntervalEnd` when the prefix reaches the top of the address space, and putting `Timeout` only on the interval start element), while passing pre-expanded (`RawIntervals || hasIntervalEnd`) sets through unchanged.
- `ze-firewall-conf.yang` declares `typedef tcp-flags` and `leaf tcp-flags` in `grouping from-block`, and updates set-flag and `element/timeout` descriptions to match the enforced behavior.
- `lowerTCPFlagsMatch` emits a `meta l4proto tcp` guard before reading transport-header byte 13.
- `docs/guide/firewall.md` and `docs/guide/migrating-from-vyos.md` reflect working `flags-interval` CIDR sets, set-flag validation rules, and the declared `tcp-flags` leaf.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Firewall set flag lowering | `internal/plugins/firewall/nft/lower_linux.go` (`lowerSet`, `encodeSetElementKey`) | Lowers `SetFlagInterval`, `SetFlagTimeout`, `SetFlagConstant`, and `SetFlagDynamic` onto `nftables.Set`, and encodes elements one-for-one (stripping CIDR prefix lengths) | Extends: keeps all four flag lowerings, adds half-open `[start, exclusiveEnd)` expansion in `lowerSet` for unexpanded IPv4/IPv6 `flags-interval` sets, and makes `encodeSetElementKey` reject CIDR strings and family mismatches |
| Firewall verify-time validation | `internal/component/firewall/model.go` (`Set.Validate`), `internal/component/firewall/validate.go` (`ValidateTables`), `internal/component/firewall/engine.go` (`VerifyConfig`, `OnConfigVerify`, `OnConfigure`) | Runs structural and cross-reference checks at offline `ze config validate`, commit, reload, and start (currently returning fatal errors from `OnConfigure`) | Extends: strengthens `Set.Validate()` so contradictory flags, element timeouts without `flags-timeout`, CIDR prefixes without `flags-interval`, host bits, family mismatches, and overlapping intervals are refused at `ze config validate`, commit, and reload, while `OnConfigure` leaves an invalid firewall unstarted at `ze start` under `Ze always starts` |
| Start-time service isolation (`Ze always starts`) | `internal/component/config/service_blocks.go` (`RegisterServiceBlock`), `internal/component/config/validate_semantic.go` (`ValidateSemantics`), `internal/component/ike/engine/icmp_overlap.go` (`warnICMPOverlapAtStart`) | Prunes registered service blocks on `LoadConfigForStart` validation errors, logs start-time config faults under `diagnostic.CodeConfigPluginVerify`, and reports every `VerifyPluginConfig` refusal in `ze doctor` (`config-semantics`) under the same code | Extends: registers `"firewall"` via `config.RegisterServiceBlock` and logs invalid-config start failures in `firewall`'s `OnConfigure` at `ERROR` under `diagnostic.CodeConfigPluginVerify` while returning `nil` so `ze start` stays up and `ze doctor` reports the same code and fix without a duplicate check |
| Canonical prefix refusal | `internal/component/firewall/config.go` (`parseAddressMatch`), `docs/architecture/core-design.md` Section 14b | Refuses literal IP prefixes with host bits set via `opererr.New` naming the masked network | Extends: applies the same canonical-prefix `opererr` rule to CIDR set elements in `Set.Validate()` |
| Half-open prefix interval encoding | `internal/plugins/copp/translate.go` (`intervalElements`, `addressAfter`, `lastAddress`), `internal/component/firewall/plugins/irr/sets.go` (`prefixesToIntervalElements`) | Encodes IPv4/IPv6 prefixes as `{start}` and `{last.Next(), IntervalEnd: true}`, omitting `IntervalEnd` when the prefix reaches the top of the address space | Extends: reuses the exact `lastAddress` + `Next()` half-open boundary rule in `lowerSet` for config-originated `flags-interval` sets, and preserves pre-expanded `copp`/`irr` sets via `Set.RawIntervals` / `IntervalEnd` detection |
| TCP flags parsing, validation, and backend lowering | `internal/component/firewall/config.go` (`parseFromBlock`, `ParseTCPFlags`), `validate.go` (`ipHeaderMatchLeaf`), `nft/lower_linux.go` (`lowerTCPFlagsMatch`), `vpp/verify.go` & `translate.go` | Parses comma-separated TCP flags, refuses them in `arp`, lowers to nftables transport byte 13 and VPP ACL TCP flags; `ze-policyroute-conf.yang` already declares `leaf tcp-flags` | Extends: declares `typedef tcp-flags` and `leaf tcp-flags` in `ze-firewall-conf.yang` `grouping from-block` (with parity test matching `rate_unit_test.go`) and prepends `meta l4proto tcp` in `lowerTCPFlagsMatch` (matching `lowerICMPTypeMatch`) |
| YANG-to-Go pattern parity test | `internal/component/firewall/yang/rate_unit_test.go` (`TestRateSpecPatternNamesTheRateUnits`) | Extracts a YANG typedef pattern and compares its alternation against an exported Go name list so YANG and Go cannot drift | Extends: adds `internal/component/firewall/yang/tcp_flags_test.go` checking `typedef tcp-flags` against `firewall.TCPFlagNames()` |
| Netfilter functional test fixtures | `internal/test/fixture/netfilter_fixture.go` (`registerTableSnapshot`) | Captures `nft list table <family> <name>` for `.ci` assertions under `option=needs-linux:caps=net-admin` | Extends: registers table-snapshot fixtures for `firewall/firewall-set-interval-cidr` and `firewall/firewall-tcp-flags` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A backend that cannot apply a config exactly refuses it at verify. It never approximates" | agrees | `Set.Validate()` refuses contradictory flags, element timeouts without `flags-timeout`, CIDR prefixes without `flags-interval`, host bits, family mismatches, and overlapping intervals at verify, and `lowerSet` lowers exact half-open intervals instead of silently stripping prefix lengths. |
| `docs/architecture/core-design.md` | 14b. Firewall and Traffic Control | `// Design:` and anchors on `model.go`, `engine.go`, `engine_test.go`, `config.go`, `lower_linux.go` | "The firewall configuration parser refuses a literal `source-address` or `destination-address` prefix with host bits set. It reports the effective network and the corrected configuration statement instead of changing the operator's input." | agrees | Applies the same canonical-prefix `opererr` refusal to CIDR set elements in `Set.Validate()` across offline validate, commit, reload, and `ze doctor` (`config-semantics`), while `OnConfigure` leaves an invalid firewall unstarted at `ze start` under `Ze always starts` and keeps the firewall data model backend-agnostic. |
| `docs/architecture/ddos/cp-survival-5-detect-5-characterization.md` | The responder split | anchored to `internal/component/firewall/model.go` | - | silent | Covers DDoS characterization and `ddos-local` mitigation (anchoring `MatchTCPFlags` only); says nothing about firewall named sets, set flags, CIDR interval lowering, or YANG `from-block`. |
| `docs/architecture/firewall/firewall-irr.md` | CIDR is encoded as an interval | anchored to `model.go`, `config.go` | "`SetElement.IntervalEnd` carries the CIDR-to-interval encoding, which is the standard nftables pattern and scales to a large prefix list." | agrees | Uses the same half-open `IntervalEnd` encoding in `lowerSet` for operator-configured `flags-interval` CIDR sets and preserves `firewall-irr`'s pre-expanded `IntervalEnd` pairs unchanged. |
| `ai/doors/forwarding.md` | Boundaries | door covering `internal/component/firewall/` and `internal/plugins/firewall/` | "**Exact or refuse, at verify.** Feature support is declared with `ze:backend` in YANG and judged by one whole-tree walk the config component runs at its two doors (`LoadConfig` and `verifyTree`) and in `ze doctor` (`core-design.md`, section 14a)." | agrees | Enforces cross-leaf set-flag and element constraints at verify in `Set.Validate()` rather than approximating at apply, and leaves `tcp-flags` unannotated by `ze:backend` because both `kernel` and `vpp` support it. |
| `docs/architecture/firewall/fw-6-firewall-vpp.md` | What VPP programs, everything else rejected at commit | anchored to `internal/component/firewall/config.go` | "A filter chain lowers to VPP's ACL plugin, which covers source and destination prefix, port range, protocol, ICMP type and code, TCP flags, and the permit, deny and reflect verdicts." | agrees | Declares `leaf tcp-flags` in `ze-firewall-conf.yang` without a `ze:backend` restriction so operator rules reach VPP's existing `MatchTCPFlags` verification and translation, while VPP continues to refuse named sets. |
| `docs/architecture/traffic/cp-survival-2-copp-port179.md` | The ACL: neighbors in, every other source out | `// Design:` and anchors on `copp/translate.go`, `config.go` | "The neighbor sets are interval sets named `bgp-neighbor-ipv4` and `bgp-neighbor-ipv6`, one per family that has a neighbor. A static peer inside a dynamic group's range is folded into the range, because the kernel refuses overlapping intervals." | agrees | Marks CoPP's `neighborSet` with `RawIntervals: true` so its pre-expanded interval elements (including a lone top-of-space range with no `IntervalEnd`) pass through `lowerSet` unchanged, and enforces the same non-overlapping interval rule on operator-configured sets. |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | Decisions | anchored to `internal/plugins/firewall/nft/lower_linux.go` | - | silent | Covers IKEv2 Child SA dataplane installation and ESP-form firewall checks (anchoring only `lowerAddrMatch` and `lowerProtoMatch` in `lower_linux.go`); says nothing about firewall sets, set flags, CIDR interval lowering, or `tcp-flags`. |
| `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` | Rule 2: a producer's table name carries the `ze_` prefix | `// Design:` on `register_producer_verify.go`; anchored to `engine.go`, `copp/translate.go` | - | silent | Covers table ownership, `ze_` naming, reservations, startup sweeps, and shutdown flush; this spec changes neither table names nor ownership or teardown behavior, and keeps `firewall-producer-tables` unchanged while adding `config.RegisterServiceBlock(ConfigRoot, "firewall")`. |
| `ai/doors/system-plugins.md` | Where it sits | door covering `internal/plugins/copp/translate.go` | "**Firewall lowering is a backend, not an engine.** `firewall/nft` and `firewall/vpp` call `firewall.RegisterBackend` (`internal/component/firewall` `RegisterBackend`). The component owns the model, and the plugin turns it into kernel or VPP state (`ai/digests/firewall.md`)." | agrees | Keeps the abstract `Set` model and `Set.Validate()` in `internal/component/firewall` and puts nftables interval expansion and TCP flags lowering inside `internal/plugins/firewall/nft`. |
| `ai/doors/le-tooling.md` | Boundaries | door covering `netfilter_fixture.go` and `test/firewall/*.ci` | "**A `.ci` that needs a Linux kernel feature carries `option=needs-linux`** and runs in the QEMU VM (`ai/rules/platform-linux.md`)." | agrees | Both kernel-applying `.ci` tests (`firewall-set-interval-cidr.ci`, `firewall-tcp-flags.ci`) carry `option=needs-linux:caps=net-admin` and register snapshot fixtures in `netfilter_fixture.go`, while `firewall-set-flags-validate.ci` runs offline without privileges. |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Named sets stay refused on VPP by the VPP verifier, `tcp-flags` works on both `kernel` and `vpp`, and no kernel-only feature is newly accepted under VPP. |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Firewall set validation (`Set.Validate()`) runs through one `ValidateTables` path at `ze config validate`, commit, reload (`OnConfigVerify`), and `ze doctor` (`config-semantics`), while `ze start` obeys the 2026-11-26 superseding decision below by leaving an invalid firewall unstarted and reported rather than exiting; data-plane mismatch checks are unchanged. |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Leaves `connection-state`, `mark-set`, `flow-offload`, and `flowtable` annotations untouched, and leaves `tcp-flags` unannotated by `ze:backend` because both `kernel` and `vpp` implement `MatchTCPFlags`. |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Every config snippet, `.ci` test, doc example, and `opererr` message in this spec uses `kernel` and `vpp`, never retired implementation names (`nft`, `tc`, `netlink`). |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Firewall config/lowering change; does not touch BGP session config or export filter next-hop rewriting. |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Firewall config/lowering change; does not touch BGP `policy { modify }` or next-hop chain references. |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Firewall config/lowering change; does not touch BGP engine filter steps or `pkg/plugin/rpc.FilterInsertion`. |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Firewall config/lowering change; does not touch BGP UPDATE forwarding or `wireOverride`. |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Firewall config/lowering change; does not touch BGP import/export next-hop validation. |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Firewall config/lowering change; does not touch BGP originated or forwarded next-hop rewriting. |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Firewall config/lowering change; does not touch BGP filter chains or `\| implied` rendering. |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Implements `Ze always starts` for firewall config validation: registers `"firewall"` via `config.RegisterServiceBlock` and updates `p.OnConfigure` (`engine.go`) so parse/validation failures at `ze start` leave the firewall unstarted (applying no approximation, storing an idle `activeCfg`), log an `ERROR` record carrying `diagnostic.CodeConfigPluginVerify` (`config-plugin-verify`) and the `opererr` fix, and return `nil` so Ze and its other services start; `ze doctor` (`config-semantics` -> `config.ValidateSemantics` -> `firewall.VerifyConfig`) reports the same `config-plugin-verify` code and fix; `ze config validate`, commit, and reload (`OnConfigVerify`) remain strictly refused as a whole so running firewall rules stay intact on a bad reload. |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Follows the same start-vs-verify split: `ze config validate`, commit, and reload refuse invalid firewall set configs strictly, while `ze start` leaves the invalid firewall off and reports it via log and `ze doctor` (`config-plugin-verify`) without altering data-plane mismatch handling. |

## Data Flow (MANDATORY)

### Entry Point
- Operator firewall configuration in YANG syntax (`firewall { table <t> { set <s> { ... } chain <c> { term <n> { from { tcp-flags ...; } } } } }`), entering via `ze config validate`, CLI editor commit, SIGHUP reload, or daemon startup.
- Format at entry: YANG-parsed config tree lowered via `Tree.ToPluginMap` into JSON `sdk.ConfigSection` for root `"firewall"`, plus programmatic `[]firewall.Table` callers (`copp`, `firewall-irr`) calling `firewall.RegisterTables`.

### Transformation Path
1. **YANG Layer 1 validation (`internal/component/config/yang`, `ze-firewall-conf.yang`):** validates `leaf tcp-flags` against `typedef tcp-flags` pattern (`fin|syn|rst|psh|ack|urg` comma list) and validates set structure (`type`, `flags-*`, `list element { value; timeout; }`).
2. **Config parsing (`internal/component/firewall/config.go`):** `parseSet` populates `Set.Flags` and `parseSetElements` populates `Set.Elements` preserving operator value strings (`"192.0.2.0/24"`, `"198.51.100.1"`) and `Timeout`; `parseFromBlock` parses `tcp-flags` via `ParseTCPFlags` into `MatchTCPFlags{Flags, Mask}`.
3. **Verify-time validation and `Ze always starts` startup handling (`internal/component/firewall/engine.go` -> `validate.go` -> `model.go`):** `ValidateTables` calls `Set.Validate()` (rejecting contradictory flags, element timeout without `flags-timeout`, `IntervalEnd` without `flags-interval`, CIDR prefix without `flags-interval`, host bits set, IP family mismatch, and overlapping intervals) and `validateMatch` (rejecting `MatchTCPFlags` in `FamilyARP`). `RunVerifier` runs the active backend's verifier (VPP accepts `MatchTCPFlags` and rejects all named sets). On strict verify paths (`ze config validate` via `VerifyConfig`, CLI editor commit, SIGHUP reload via `OnConfigVerify`), any failure refuses the config as a whole and keeps running rules unchanged. At `ze start`, `config.RegisterServiceBlock(ConfigRoot, "firewall")` isolates `ze:validate` failures under `firewall/`, and `p.OnConfigure` catches `parseConfigSections`/`validateConfiguredFirewallTables` errors, leaves the firewall unstarted (idle `activeCfg`, no backend load or partial apply), logs an `ERROR` record carrying `code=config-plugin-verify` (`diagnostic.CodeConfigPluginVerify`) and the `opererr` fix, and returns `nil` so Ze and its other services start normally; `ze doctor`'s `config-semantics` check (`config.ValidateSemantics` -> `firewall.VerifyConfig`) reports the same failure under `code=config-plugin-verify`.
4. **Backend lowering (`internal/plugins/firewall/nft/lower_linux.go`):** `applySet` calls `lowerSet`:
   - For non-interval sets, `encodeSetElementKey` encodes single elements and rejects any CIDR prefix or family mismatch.
   - For pre-expanded interval sets (`s.RawIntervals || hasIntervalEnd(s.Elements)` — `copp` and `firewall-irr`), elements lower verbatim with their existing `IntervalEnd` markers.
   - For unexpanded IPv4/IPv6 `flags-interval` sets (operator config), `lowerSet` parses each element as a canonical prefix (or promotes a bare address to `/32` or `/128`), sorts intervals numerically by start address, rejects overlaps, emits `{Key: start, Timeout: timeout}` and (unless the prefix reaches the top of the address space) `{Key: last.Next(), IntervalEnd: true}`.
   - `lowerTCPFlagsMatch` emits `meta l4proto tcp` (`MetaKeyL4PROTO == IPPROTO_TCP`), then transport-header byte-13 `Payload` + `Bitwise` + `Cmp`.
5. **Kernel apply and readback (`backend_linux.go`, `readback_linux.go`, `cmd_show.go`):** `conn.AddSet` and `conn.AddRule` flush atomically to nftables; `LastApplied()` retains the operator's model (`Set.Elements` with CIDR strings) for `show firewall ruleset`, `show firewall group`, and the web sets view.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config / CLI ↔ Firewall component | YANG schema `ze-firewall-conf.yang` -> `Tree.ToPluginMap` JSON `sdk.ConfigSection` -> `VerifyConfig` / `OnConfigure` / `OnConfigVerify` | Yes |
| Firewall component ↔ `firewall/nft` backend | In-process `firewall.Backend.Apply([]Table)` carrying `firewall.Set` (`Flags`, `RawIntervals`, `Elements`) and `firewall.MatchTCPFlags` | Yes |
| `firewall/nft` backend ↔ Linux kernel nftables | Netlink `NFT_MSG_NEWSET` / `NFT_MSG_NEWSETELEM` (`NFT_SET_INTERVAL`, `NFT_SET_ELEM_INTERVAL_END`, `NFTA_SET_ELEM_TIMEOUT`) and `NFT_MSG_NEWRULE` (`meta l4proto tcp` + payload/bitwise/cmp) via `google/nftables` | Yes |
| `copp` plugin ↔ Firewall component | In-process `firewall.RegisterTables` with `Set{Flags: SetFlagInterval, RawIntervals: true}` | Yes |

### Changed-Wire Census

**Search evidence:** Searched `internal/` and `docs/` with `grep -rn -E "IntervalEnd|SetFlagInterval|flags-interval|tcp-flags|MatchTCPFlags|lowerSet"` and `grep -rn "IntervalEnd" internal/component/firewall/ internal/plugins/firewall/ internal/plugins/copp/`. Positive controls confirmed: `internal/plugins/firewall/nft/lower_linux.go` (`lowerSet`, `encodeSetElementKey`, `lowerTCPFlagsMatch`), `internal/component/firewall/plugins/irr/sets.go` (`prefixesToIntervalElements`), `internal/plugins/copp/translate.go` (`intervalElements`), `internal/plugins/firewall/nft/readback_linux.go` (`fillSets`), `internal/plugins/firewall/vpp/translate.go` (`MatchTCPFlags`), and `internal/component/firewall/config.go` (`parseFromBlock`, `parseSet`). No external Python/shell scripts encode or decode nftables netlink set elements directly; `.ci` fixtures invoke userspace `nft list table`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| nftables netlink set elements (`NFT_MSG_NEWSETELEM` / `NFT_MSG_GETSETELEM`) | writer | `internal/plugins/firewall/nft/lower_linux.go` (`lowerSet`, `encodeSetElementKey`) -> `vendor/github.com/google/nftables/set.go` (`AddSet`, `makeElemList`) | Currently strips CIDR prefix length in `encodeSetElementKey` and writes one bare-address element with `IntervalEnd=false` | Change: expand unexpanded IPv4/IPv6 `flags-interval` elements into sorted `[start, exclusiveEnd)` pairs with `IntervalEnd=true`; preserve pre-expanded `copp`/`irr` elements; prove with unit, kernel integration, and `.ci` tests |
| nftables netlink set elements (`NFT_MSG_NEWSETELEM` / `NFT_MSG_GETSETELEM`) | reader | Linux kernel `nf_tables` (`nft_set_rbtree` / `nft_set_pipapo`, outside repo) | Kernel evaluates half-open `[start, end)` intervals on `NFT_SET_INTERVAL` sets and rejects `NFTA_SET_ELEM_TIMEOUT` on `IntervalEnd` markers | Preserve kernel contract: emit timeout only on start element, omit `IntervalEnd` for top-of-space intervals, reject overlaps at verify |
| nftables netlink set elements (`NFT_MSG_NEWSETELEM` / `NFT_MSG_GETSETELEM`) | reader | `internal/plugins/firewall/nft/readback_linux.go` (`fillSets`, `decodeSetElementKey`) | Already decodes `IntervalEnd`, `Timeout`, and 4/16-byte IP keys from `NFT_MSG_GETSETELEM` | Preserve: unchanged, already reads back interval endpoints and flags |
| nftables netlink set elements (`NFT_MSG_NEWSETELEM` / `NFT_MSG_GETSETELEM`) | reader | Userspace `nft list table` invoked by `internal/test/fixture/netfilter_fixture.go` (`registerTableSnapshot`) | Dumps kernel sets via netlink and renders interval set elements as CIDR prefixes (e.g. `192.0.2.0/24`) and TCP flag rules as `tcp flags` | Preserve: used as external proof in `firewall-set-interval-cidr.ci` and `firewall-tcp-flags.ci` |
| nftables rule expressions (`NFT_MSG_NEWRULE`) | writer | `internal/plugins/firewall/nft/lower_linux.go` (`lowerTCPFlagsMatch`) | Currently emits transport-header byte-13 `Payload`+`Bitwise`+`Cmp` without `meta l4proto tcp` | Change: prepend `meta l4proto tcp` so rules without explicit `protocol tcp` inspect TCP only; preserves `policy-tcp-flags.ci` |
| nftables rule expressions (`NFT_MSG_NEWRULE`) | reader | Linux kernel `nf_tables` (outside repo) and userspace `nft list table` | Kernel evaluates `meta l4proto tcp` then byte-13 TCP flags; `nft` renders `tcp flags` | Preserve kernel contract: identical to `lowerICMPTypeMatch` pattern |
| YANG / plugin config JSON (`firewall/table/chain/term/from/tcp-flags`) | writer | `internal/component/config/tree.go` (`Tree.ToPluginMap`), validated by `internal/component/firewall/yang/ze-firewall-conf.yang` | `grouping from-block` currently omits `leaf tcp-flags`, rejecting config before `ToPluginMap` runs | Change: declare `typedef tcp-flags` and `leaf tcp-flags` in `from-block` so operator configs produce the `"tcp-flags"` JSON string `parseFromBlock` already expects |
| YANG / plugin config JSON (`firewall/table/chain/term/from/tcp-flags`) | reader | `internal/component/firewall/config.go` (`parseFromBlock`, `ParseTCPFlags`) | Already reads `m["tcp-flags"].(string)` into `MatchTCPFlags` | Preserve: existing parser arm now becomes reachable from YANG config |

### Integration Points
- `internal/component/firewall/model.go` (`Set`, `Set.Validate`) - central set validation invoked by `ValidateTables` (`validate.go:39`).
- `internal/component/firewall/engine.go` (`configureAtStart`, `p.OnConfigure`, `VerifyConfig`, `p.OnConfigVerify`) - keeps validate/commit/reload strictly refusing while handling invalid firewall configs at `ze start` without exiting the daemon, logging `ERROR` under `diagnostic.CodeConfigPluginVerify` so `ze doctor` (`config-semantics`) reports the same code and fix.
- `internal/component/firewall/register_producer_verify.go` (`init`) - registers `config.RegisterServiceBlock(ConfigRoot, "firewall")` for `LoadConfigForStart` service-block isolation.
- `internal/component/firewall/config.go` (`TCPFlagNames`) - single Go declaration of valid TCP flag names, consumed by the YANG parity test.
- `internal/component/firewall/yang/ze-firewall-conf.yang` (`typedef tcp-flags`, `grouping from-block`, `list set`) - single YANG schema consumed by CLI, editor, web, REST, gRPC, MCP, and `ze config validate`.
- `internal/plugins/firewall/nft/lower_linux.go` (`lowerSet`, `encodeSetElementKey`, `lowerTCPFlagsMatch`) - nftables lowering consumed by `backend_linux.go` `applySet` and `addChainRules`.
- `internal/plugins/copp/translate.go` (`neighborSet`) - marks CoPP's pre-expanded interval sets with `RawIntervals: true`.
- `internal/test/fixture/netfilter_fixture.go` - registers table-snapshot fixtures for the new kernel `.ci` tests.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | `tcp-flags` enters through `ze-firewall-conf.yang` -> `parseFromBlock` -> `ValidateTables` -> `lowerTCPFlagsMatch` / VPP `translate`; set validation runs in `Set.Validate()` via `ValidateTables` for offline validate, commit, reload, and `ze doctor` (`config-semantics`), while `ze start` (`OnConfigure`) leaves an invalid firewall down and logs `config-plugin-verify` |
| No unintended coupling (components stay isolated) | Yes | `firewall` stays backend-agnostic; interval expansion lives in `firewall/nft/lower_linux.go`; `copp` already imports `internal/component/firewall` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `Set.Validate()`, `opererr.New`, `lowerSet`, `ParseTCPFlags`, `lowerTCPFlagsMatch`, and `registerTableSnapshot`; does not introduce a second set type or parser |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Config-path only (control plane, not packet or BGP hot path); `LastApplied()` deep-copy snapshot semantics unchanged |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No new command, view, family, or core switch case; `leaf tcp-flags` registers through the existing `ze-firewall-conf.yang` embed and reaches CLI/web/API automatically |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `internal/component/firewall/`, `internal/plugins/firewall/`, and `internal/component/config/`: `parseFromBlock` (`config.go:355`), `ipHeaderMatchLeaf` (`validate.go:327`), `lowerMatch` (`lower_linux.go:543`), `vpp/verify.go:302`, and `vpp/translate.go:124` already handle `tcp-flags`/`MatchTCPFlags`; editor completion and validation derive automatically from `ze-firewall-conf.yang` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Linux kernel `nft_set_rbtree` accepts adjacent half-open intervals `[A, B)` and `[B, C)` emitted in ascending numeric order in one `AddSet` batch (e.g. `192.0.2.0/25` and `192.0.2.128/25`, or `10.0.0.1` and `10.0.0.2`), and accepts a trailing top-of-space open-ended interval with no `IntervalEnd` | `internal/plugins/copp/translate.go:247-289`, `internal/plugins/firewall/nft/lower_linux.go:836-848` | Adjacent intervals would fail `Flush` with `EEXIST` and require merging or explicit refusal | Kernel integration test in `internal/plugins/firewall/nft/integration_linux_test.go` | confirmed |
| A-2 | Linux kernel `nft` accepts a per-element `Timeout` on the interval start element (with `Timeout: 0` on the `IntervalEnd` closing element) when `flags-interval` and `flags-timeout` are both set | `vendor/github.com/google/nftables/set.go:420-423`, Linux `nf_tables_newsetelem` | Combining `flags-interval` + `flags-timeout` + element `timeout` would fail at `Flush` | Kernel integration test in `internal/plugins/firewall/nft/integration_linux_test.go` | confirmed |
| A-3 | Adding `meta l4proto tcp` in `lowerTCPFlagsMatch` keeps `test/policy/policy-tcp-flags.ci` green even when `protocol tcp` is also present in the same term | `test/policy/policy-tcp-flags.ci:39` (`expect=stdout:contains=tcp flags`) and `lowerICMPTypeMatch` precedent | `nft` could reject duplicate `meta l4proto tcp` or alter output format | Run `test/policy/policy-tcp-flags.ci` and `test/firewall/firewall-tcp-flags.ci` on Linux | confirmed |
| A-4 | No existing `.ci` or Go test writes a valid config that sets `element ... { timeout N }` without `flags-timeout`, a CIDR prefix without `flags-interval`, or contradictory `flags-constant` + `flags-dynamic`/`flags-timeout` through `ValidateTables` | Grep across `test/` and `internal/component/firewall/` (`firewall-set-element-timeout.ci` already sets `flags-timeout;`; `lower_linux_test.go:130,1991` bypass `ValidateTables`) | An existing test would fail `ValidateTables` and reveal a hidden fixture gap | Scoped unit tests + `test/firewall/` suite run by main thread | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Pre-expanded `copp` or `firewall-irr` interval sets accidentally enter the config CIDR expansion path (especially a lone `copp` `0.0.0.0/0` range with no `IntervalEnd`) | `copp` or `firewall-irr` unit/functional test failure, or `0.0.0.0/0` narrowed to `/32` | Add `Set.RawIntervals bool`, set it in `copp/translate.go` `neighborSet`, and bypass config expansion whenever `s.RawIntervals || hasIntervalEnd(s.Elements)` |
| R-2 | Lexicographic sort from `parseSetElements` (`"10.0.0.10"` before `"10.0.0.2"`) causes nftables interval insertion to fail or misorder an open-ended top-of-space interval | Kernel `Flush` error on multi-prefix interval sets | Sort intervals numerically by `netip.Addr.Compare` inside `lowerSet` (and inside `Set.Validate()` overlap check) before emitting elements |
| R-3 | `ze-firewall-conf.yang` `typedef tcp-flags` pattern drifts from `tcpFlagNames` in `config.go` | Operator writes a valid flag rejected by YANG or vice versa | Add `internal/component/firewall/yang/tcp_flags_test.go` comparing `typedef tcp-flags` against `firewall.TCPFlagNames()` |
| R-4 | `docs/guide/firewall.md` Named Sets example fails `ze config validate` once `timeout` without `flags-timeout` is refused | Operator copying guide example hits validation refusal | Update `docs/guide/firewall.md` Named Sets example to include `flags-timeout;` in the same change |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Valid `firewall` configs using named sets or `tcp-flags` could be wrongly refused at validate/commit, or `flags-interval` sets (including `copp`/`irr`) could mis-program kernel nftables interval boundaries |
| How is it reverted? | Single commit revert; no persistent state format or on-disk migration is involved |
| Who else touches this path? | `firewall-irr` and `copp` share `firewall.Set` and `lowerSet`; `policyroute` shares `firewall.ParseTCPFlags` and `lowerTCPFlagsMatch`; `firewall/vpp` consumes `MatchTCPFlags` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `firewall { table <t> { set <s> { flags-interval; element 192.0.2.0/24 {} } } }` via `ze` boot/apply | → | `parseSet` -> `Set.Validate` -> `applySet` -> `lowerSet` (`IntervalEnd` emission) -> kernel nftables | `test/firewall/firewall-set-interval-cidr.ci` |
| `firewall { table <t> { chain <c> { term <n> { from { tcp-flags syn; } } } } }` via `ze` boot/apply | → | `ze-firewall-conf.yang` `from-block/tcp-flags` -> `parseFromBlock` -> `lowerTCPFlagsMatch` (`meta l4proto tcp` + flags) -> kernel nftables | `test/firewall/firewall-tcp-flags.ci` |
| Invalid set flags/elements (`timeout` without `flags-timeout`, CIDR without `flags-interval`, `flags-constant` + `flags-dynamic`/`flags-timeout`) via `ze config validate` | → | `VerifyConfig` -> `ValidateTables` -> `Set.Validate` | `test/firewall/firewall-set-flags-validate.ci` |
| Invalid firewall set config at `ze start` | → | `p.OnConfigure` (`configureAtStart`) leaves firewall unstarted, logs `ERROR` under `diagnostic.CodeConfigPluginVerify`, returns `nil`; `ze doctor` (`config.ValidateSemantics`) reports `config-plugin-verify` | `TestConfigureAtStartLeavesFirewallDownAndLogsDoctorCode` (`internal/component/firewall/engine_test.go`) and `test/firewall/firewall-set-flags-validate.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Operator configures a `type ipv4` or `type ipv6` set with `flags-interval` containing canonical CIDR prefixes (e.g., `192.0.2.0/24`, `2001:db8::/32`), bare host addresses (e.g., `198.51.100.1`), adjacent prefixes, or a top-of-space prefix (`255.255.255.0/24`) | Config validates, `lowerSet` emits numerically sorted half-open `[start, exclusiveEnd)` pairs with `IntervalEnd: true` (omitting `IntervalEnd` only when the prefix reaches the top of the address space, and putting `Timeout` only on the start element), and `nft list table` displays the programmed prefix ranges (e.g., `192.0.2.0/24`, `2001:db8::/32`) | Pre-expanded `SetFlagInterval` sets (`RawIntervals: true` or already carrying `IntervalEnd: true` from `copp` and `firewall-irr`) bypass config expansion so their existing boundary representation is preserved unchanged |
| AC-2 | Operator configures a CIDR prefix element (e.g., `192.0.2.0/24`) on a set without `flags-interval`, or a non-canonical CIDR prefix with host bits set (`192.0.2.5/24`), or overlapping intervals (`10.0.0.0/24` and `10.0.0.128/25`), or an IP element of the wrong address family | `ze config validate`, commit, reload (`OnConfigVerify`), and `Set.Validate()` refuse the config with an `opererr` naming the set, element, reason, and fix; at `ze start`, `OnConfigure` leaves the firewall unstarted (applying no partial rules), logs an `ERROR` under `code=config-plugin-verify` with the fix, and returns `nil` so Ze keeps running while `ze doctor` (`config-semantics`) reports the same code and fix; `encodeSetElementKey` also refuses CIDR prefixes and family mismatches rather than stripping the prefix length or panicking on `As4()`/`As16()` | Adjacent non-overlapping intervals (e.g., `192.0.2.0/25` and `192.0.2.128/25`) remain accepted because operators legitimately list contiguous prefixes in network groups |
| AC-3 | Operator configures a set combining `flags-constant` with `flags-dynamic` or `flags-timeout`, or configures an element with `timeout > 0` on a set without `flags-timeout` | `ze config validate`, commit, reload (`OnConfigVerify`), and `Set.Validate()` refuse the config with an `opererr` naming the set, conflicting flag or element, reason, and fix; at `ze start`, `OnConfigure` leaves the firewall unstarted, logs an `ERROR` under `code=config-plugin-verify` with the fix, and returns `nil` so Ze keeps running while `ze doctor` (`config-semantics`) reports the same code and fix; valid combinations (`flags-timeout` + element `timeout`, `flags-dynamic` + `flags-timeout`, standalone `flags-constant`, static sets with no flags) remain accepted and apply unchanged | Direct unit calls to `lowerSet` bypassing `ValidateTables` (`TestLowerSetTimeoutFlagAndElementTimeouts`, `TestLowerSetCarriesConstantAndDynamic`) continue to pass because `lowerSet` remains a pure translator for flags while `Set.Validate()` enforces semantic flag compatibility |
| AC-4 | Operator configures a firewall term with `from { tcp-flags syn; }` or `from { tcp-flags syn,ack; }` (with or without `protocol tcp;`) in an `ip`, `ip6`, or `inet` table | YANG schema validation accepts valid comma-separated flags (`fin`, `syn`, `rst`, `psh`, `ack`, `urg`) and rejects unknown/empty values; `parseFromBlock` produces `MatchTCPFlags`; `lowerTCPFlagsMatch` emits `meta l4proto tcp` followed by the transport byte-13 masked compare; `nft list table` displays the `tcp flags` match; `arp` tables continue to refuse `tcp-flags` at verify; VPP filter chains continue to accept `MatchTCPFlags` | Packet-triggered set insertion (`expr.Dynset`) remains absent and owned by `plan/post-release/features/spec-firewall-dynamic-address-group.md` |
| AC-5 | Operator consults `docs/guide/firewall.md`, `docs/guide/migrating-from-vyos.md`, and `ze-firewall-conf.yang` help/descriptions for named sets and `tcp-flags` | `docs/guide/firewall.md` Named Sets example includes `flags-timeout;` and documents `flags-interval` CIDR sets and flag validation; `docs/guide/migrating-from-vyos.md` maps VyOS `network-group` directly to `set ... flags-interval` (`equivalent`) without the one-term-per-prefix workaround or stale `tcp-flags` disclaimer; YANG descriptions accurately describe interval lowering and verify refusals | Other conditional rows in `docs/guide/migrating-from-vyos.md` (such as interface name resolution) remain untouched because they belong to other child specs |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures a named `flags-interval` address-group set with IPv4 and IPv6 CIDR prefixes and matches `@networks` in a filter term | YANG parse -> `parseSet` -> `Set.Validate` -> `applySet` -> `lowerSet` (`[start, exclusiveEnd)` with `IntervalEnd`) -> kernel nftables | `test/firewall/firewall-set-interval-cidr.ci` |
| 2 | Configures a firewall term matching `from { tcp-flags syn; }` (and `tcp-flags syn,ack;`) | YANG `from-block/tcp-flags` -> `parseFromBlock` (`ParseTCPFlags`) -> `ValidateTables` -> `lowerTCPFlagsMatch` (`meta l4proto tcp` + flags) -> kernel nftables | `test/firewall/firewall-tcp-flags.ci` |
| 3 | Runs `ze config validate` on a set with a CIDR element missing `flags-interval`, an element `timeout` missing `flags-timeout`, or contradictory `flags-constant` + `flags-dynamic`/`flags-timeout` | `ze config validate` -> `VerifyConfig` -> `ValidateTables` -> `Set.Validate` (`opererr` refusal) | `test/firewall/firewall-set-flags-validate.ci` |
| 4 | Starts `ze` with a config file containing an invalid firewall set | `LoadConfigForStart` -> `firewall` `OnConfigure` (`configureAtStart`) leaves firewall unstarted, logs `ERROR` under `config-plugin-verify` with fix, returns `nil` so Ze stays up; `ze doctor` (`config.ValidateSemantics`) reports `config-plugin-verify` | `TestConfigureAtStartLeavesFirewallDownAndLogsDoctorCode` and `test/firewall/firewall-set-flags-validate.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSetValidateFlagsAndElements` | `internal/component/firewall/model_test.go` | Contradictory flags (`flags-constant` + `flags-dynamic`, `flags-constant` + `flags-timeout`), element `Timeout > 0` without `SetFlagTimeout`, `IntervalEnd` without `SetFlagInterval`, CIDR prefix without `SetFlagInterval`, non-canonical CIDR host bits, IP family mismatch, overlapping vs adjacent intervals, valid combinations, and `RawIntervals` passthrough | PASS |
| `TestConfigureAtStartLeavesFirewallDownAndLogsDoctorCode` | `internal/component/firewall/engine_test.go` | Invalid firewall set config at `ze start` (`configureAtStart`) returns `nil`, applies no tables, stores idle `activeCfg`, and logs `ERROR` with `code=config-plugin-verify` and fix; `VerifyConfig`, `parseAndVerifyFirewallSections` (reload), and `config.ValidateSemantics` (`ze doctor`) still refuse/report under `config-plugin-verify` | PASS |
| `TestTCPFlagsYANGPatternMatchesGo` | `internal/component/firewall/yang/tcp_flags_test.go` | `typedef tcp-flags` alternation in `ze-firewall-conf.yang` equals `firewall.TCPFlagNames()` | PASS |
| `TestLowerSetIntervalCIDRExpansion` | `internal/plugins/firewall/nft/lower_linux_test.go` | IPv4 and IPv6 CIDR prefixes, mixed CIDR + bare host IP, lexicographically unsorted inputs sorted numerically, top-of-space prefixes (`255.255.255.0/24`, `0.0.0.0/0`, `ffff::/16`) without `IntervalEnd` wrap, timeout placed only on start element, pre-expanded IRR and `RawIntervals` CoPP passthrough | PASS |
| `TestEncodeSetElementKeyRejectsCIDRAndWrongFamily` | `internal/plugins/firewall/nft/lower_linux_test.go` | `encodeSetElementKey` rejects IPv4/IPv6 CIDR strings and family-mismatched addresses (no silent mask strip, no `As4()`/`As16()` panic) | PASS |
| `TestLowerTCPFlagsMatchEmitsL4ProtoGuard` | `internal/plugins/firewall/nft/lower_linux_test.go` | `lowerTCPFlagsMatch` emits `meta l4proto tcp` followed by transport-header offset-13 payload, bitwise mask, and compare | PASS |
| `TestNftIntegration_IntervalCIDRSetAndTCPFlags` | `internal/plugins/firewall/nft/integration_linux_test.go` | Real Linux nftables accepts and reads back a `flags-interval` IPv4+IPv6 CIDR set (with adjacent prefixes, top-of-space prefix, and `flags-timeout` element timeout) and a `MatchTCPFlags` rule | PASS |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| IPv4 CIDR prefix length in `flags-interval` set | `0..32` | `32` (host) and `0` (`0.0.0.0/0`, open-ended) | N/A (`netip.ParsePrefix` rejects negative) | `/33` refused by `netip.ParsePrefix` |
| IPv6 CIDR prefix length in `flags-interval` set | `0..128` | `128` (host) and `0` (`::/0`, open-ended) | N/A (`netip.ParsePrefix` rejects negative) | `/129` refused by `netip.ParsePrefix` |
| IPv4/IPv6 top-of-space prefix end (`255.255.255.255` / `ffff:...:ffff`) | `0.0.0.0..255.255.255.255`, `::..ffff:...:ffff` | `255.255.255.255/32`, `ffff:...:ffff/128` (no `IntervalEnd` wrap) | N/A | N/A (handled without wrap via `!last.Next().IsValid()`) |
| Set element `timeout` (seconds, uint32) | `0..4294967295` | `4294967295` (when `flags-timeout` set); `0` (when `flags-timeout` unset) | `-1` refused by `parseElementTimeout` | `4294967296` refused by `parseElementTimeout`; `>= 1` refused by `Set.Validate` when `flags-timeout` unset |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `firewall-set-interval-cidr` | `test/firewall/firewall-set-interval-cidr.ci` | Operator configures `flags-interval` IPv4 and IPv6 sets with CIDR prefixes and bare host addresses; `nft list table` displays the programmed prefix intervals | PASS (80/80 under load) |
| `firewall-tcp-flags` | `test/firewall/firewall-tcp-flags.ci` | Operator configures firewall terms with `from { tcp-flags syn; }` and `from { tcp-flags syn,ack; }`; `nft list table` displays `tcp flags` matches | PASS (80/80 under load) |
| `firewall-set-flags-validate` | `test/firewall/firewall-set-flags-validate.ci` | Operator runs `ze config validate` against CIDR without `flags-interval`, element `timeout` without `flags-timeout`, and contradictory `flags-constant` + `flags-dynamic`/`flags-timeout` (each refused with exit 1 and a clear fix), and starts `ze` with an invalid set config verifying the daemon stays up and logs `config-plugin-verify` with the fix instead of exiting 1 | PASS (80/80 under load) |

### Interop Tests (Scope: protocol)
N-A: Scope is `config` (local nftables/YANG configuration fidelity, no routing protocol wire exchange).

## Files to Modify
- `internal/component/firewall/model.go` - add `RawIntervals` on `Set` and strengthen `Set.Validate()` for contradictory flags, element `Timeout` without `SetFlagTimeout`, `IntervalEnd` without `SetFlagInterval`, CIDR prefix without `SetFlagInterval`, host bits set, IP family mismatch, and overlapping intervals.
- `internal/component/firewall/engine.go` - extract `configureAtStart` from `p.OnConfigure` so parse/validation failures at `ze start` leave the firewall unstarted (idle `activeCfg`, no partial apply), log an `ERROR` record carrying `diagnostic.CodeConfigPluginVerify` (`config-plugin-verify`) and the `opererr` fix, and return `nil` under `Ze always starts`, while `VerifyConfig` and `p.OnConfigVerify` remain strictly refusing.
- `internal/component/firewall/register_producer_verify.go` - register `config.RegisterServiceBlock(ConfigRoot, "firewall")` in `init()` so `LoadConfigForStart` isolates any `ze:validate` failure scoped under `firewall/`.
- `internal/component/firewall/config.go` - export `TCPFlagNames()` so the YANG pattern parity test reads the single Go declaration of valid TCP flag names.
- `internal/component/firewall/yang/ze-firewall-conf.yang` - add `typedef tcp-flags` and `leaf tcp-flags` in `grouping from-block`; update `flags-interval`, `flags-timeout`, `flags-constant`, `flags-dynamic`, and `element/timeout` descriptions to reflect enforced lowering and verify refusals.
- `internal/plugins/firewall/nft/lower_linux.go` - make `encodeSetElementKey` reject CIDR prefixes and check IPv4/IPv6 family; update `lowerSet` to expand unexpanded IPv4/IPv6 `flags-interval` elements into numerically sorted `[start, exclusiveEnd)` interval pairs with `IntervalEnd` (omitting `IntervalEnd` at top of address space, placing timeout only on start element, preserving pre-expanded `copp`/`irr` sets); prepend `meta l4proto tcp` in `lowerTCPFlagsMatch`.
- `internal/plugins/copp/translate.go` - set `RawIntervals: true` on `neighborSet` so a lone top-of-space CoPP neighbor prefix is never misread as a bare host address.
- `internal/test/fixture/netfilter_fixture.go` - register `firewall/firewall-set-interval-cidr` and `firewall/firewall-tcp-flags` table snapshot fixtures.
- `internal/component/firewall/model_test.go` - add `TestSetValidateFlagsAndElements`.
- `internal/component/firewall/engine_test.go` - add `TestConfigureAtStartLeavesFirewallDownAndLogsDoctorCode`.
- `internal/plugins/firewall/nft/lower_linux_test.go` - add `TestLowerSetIntervalCIDRExpansion`, `TestEncodeSetElementKeyRejectsCIDRAndWrongFamily`, and `TestLowerTCPFlagsMatchEmitsL4ProtoGuard`.
- `internal/plugins/firewall/nft/integration_linux_test.go` - add `TestNftIntegration_IntervalCIDRSetAndTCPFlags`.
- `docs/guide/firewall.md` - add `flags-timeout;` to the Named Sets example and document `flags-interval` CIDR prefix elements and set-flag validation rules.
- `docs/guide/migrating-from-vyos.md` - update the `network-group` row from conditional one-term-per-prefix rewrite to `set ... flags-interval` (`equivalent`), remove the defect paragraph, and update the stale `tcp-flags` note.

## Files to Create
- `internal/component/firewall/yang/tcp_flags_test.go` - YANG-to-Go parity test verifying `typedef tcp-flags` matches `firewall.TCPFlagNames()`.
- `test/firewall/firewall-set-interval-cidr.ci` - functional test (`option=needs-linux:caps=net-admin`) proving `flags-interval` CIDR prefix lowering against real nftables.
- `test/firewall/firewall-tcp-flags.ci` - functional test (`option=needs-linux:caps=net-admin`) proving `from { tcp-flags ...; }` validates through YANG and programs `tcp flags` in nftables.
- `test/firewall/firewall-set-flags-validate.ci` - functional test proving `ze config validate` refuses invalid set-flag/element configs while `ze start` stays up and logs `config-plugin-verify` with the fix.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/firewall/yang/ze-firewall-conf.yang`: declare `typedef tcp-flags` and `leaf tcp-flags` in `grouping from-block`; update `list set` flag/timeout descriptions |
| YANG validation constraints | Yes | `typedef tcp-flags` carries native `pattern '(fin\|syn\|rst\|psh\|ack\|urg)(,(fin\|syn\|rst\|psh\|ack\|urg))*'` |
| YANG custom validators | No | Existing `InProcessConfigVerifier` (`VerifyConfig` -> `ValidateTables` -> `Set.Validate`) validates cross-leaf set flag/element constraints without adding a new `ze:validate` hook |
| CLI commands/flags | No | No new CLI command or flag; existing `ze config validate`, `show firewall ruleset`, and `show firewall group` consume the existing model |
| CLI grammar (keyword before value) | N-A | No CLI command added or modified |
| Editor autocomplete | Yes | Automatic from `ze-firewall-conf.yang` for `leaf tcp-flags` under `firewall table <t> chain <c> term <n> from` |
| Functional test for new RPC/API | N-A | No new RPC or API endpoint; three firewall `.ci` tests cover config validate and kernel apply |
| Pipe completeness | N-A | No new show or command output path |
| Env var registration | N-A | No new environment variable |
| Doctor check for runtime dependencies | N-A | No new file path, socket, kernel module, port, or binary dependency |
| Doctor code per problem class | Yes | `diagnostic.CodeConfigPluginVerify` (`config-plugin-verify`): logged at `ERROR` with `opererr` problem/why/fix by `firewall` `OnConfigure` (`engine.go`) when invalid config leaves firewall unstarted at `ze start`, and reported under the same code by `ze doctor`'s existing `config-semantics` check (`config.ValidateSemantics` -> `VerifyPluginConfig` -> `firewall.VerifyConfig`) so no duplicate doctor check is registered |
| Prometheus counters/metrics | N-A | No new runtime state or counter; existing firewall apply metrics cover set programming |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not a BGP family change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Config fidelity fix for already-documented firewall sets and `tcp-flags`; no new feature maturity entry |
| 2 | Config syntax changed? | No | `docs/guide/configuration.md` and `docs/architecture/config/syntax.md` do not enumerate firewall `from-block` leaves (firewall leaves live in `docs/guide/firewall.md`) |
| 3 | CLI command added/changed? | No | No CLI command added or changed |
| 4 | API/RPC added/changed? | No | No API or RPC added or changed |
| 5 | Plugin added/changed? | No | No plugin registration or lifecycle changed |
| 6 | Has a user guide page? | Yes | `docs/guide/firewall.md` (fix Named Sets example to include `flags-timeout;`, document `flags-interval` CIDR elements and set-flag validation) and `docs/guide/migrating-from-vyos.md` (replace conditional `network-group` workaround and stale `tcp-flags` note) |
| 7 | Wire format changed? | No | No BGP or protocol wire format changed (internal nftables netlink encoding only) |
| 8 | Plugin SDK/protocol changed? | No | No `pkg/plugin/` change |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Scope is `config`, not RFC protocol behavior |
| 10 | Test infrastructure changed? | No | `docs/functional-tests.md` unaffected; only registers two standard `registerTableSnapshot` entries in `netfilter_fixture.go` |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` unaffected |
| 12 | Internal architecture changed? | No | `docs/architecture/core-design.md` Section 14b already describes the firewall abstract model, canonical prefix refusal, and backend split without contradicting this fix |
| 13 | Route metadata keys added/changed? | N-A | No route metadata keys |
| 14 | Prometheus counters added/changed? | N-A | No metrics added or changed |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No plugin, event, send, command, or capability registration changed |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived via `./le spec citation anchors`: **updated:** `docs/guide/firewall.md`, `docs/guide/migrating-from-vyos.md`. **Declared (`// Design:`) and unaffected:** `docs/architecture/core-design.md` (Section 14b already describes the backend-agnostic firewall model, canonical prefix refusal, and nftables lowering; `OnConfigure` startup isolation follows the 2026-11-26 `Ze always starts` decision), `docs/architecture/traffic/cp-survival-2-copp-port179.md` (`RawIntervals: true` preserves CoPP's existing neighbor interval behavior unchanged), `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` (no ownership/flush change; `firewall-producer-tables` stays unchanged). **Anchored (`<!-- source: -->`) and unaffected:** `docs/architecture/ddos/cp-survival-5-detect-5-characterization.md` (only cites `MatchTCPFlags` for `ddos-local`), `docs/architecture/firewall/firewall-irr.md` (IRR pre-expanded `IntervalEnd` sets unchanged), `docs/architecture/firewall/fw-6-firewall-vpp.md` (VPP already accepts `MatchTCPFlags` and refuses sets), `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` (only cites `lowerAddrMatch`/`lowerProtoMatch`), `docs/guide/command-reference.md` (only cites `engine.go` for general firewall command docs), `docs/guide/ddos-mitigation.md` (only cites `engine.go` for shared firewall backend lifecycle), `docs/guide/developer-setup.md` (only mentions `integration_linux_test.go`), `docs/guide/flowspec-protected-router.md` (FlowSpec lowering unchanged), `docs/guide/policy-routing.md` (`policyroute` `tcp-flags`/`tcp-mss` behavior preserved) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/firewall.md` (Named Sets `element ... { timeout 3600; }` example updated with `flags-timeout;`; `tcp-flags syn;` example verified against new YANG leaf) and `docs/guide/migrating-from-vyos.md` (`network-group` and `tcp-flags` notes updated) |

## Design presentation

### What changes

This change finishes three remaining configuration-fidelity gaps in the firewall subsystem (the initial `flags-dynamic`, `flags-timeout`, and `flags-constant` lowering already landed in HEAD). First, IPv4 and IPv6 CIDR prefix elements (and bare host addresses) inside an operator-configured `flags-interval` set are lowered in `lowerSet` to numerically sorted half-open `[start, exclusiveEnd)` nftables intervals with `IntervalEnd` rather than silently losing their prefix length, while pre-expanded sets built by `copp` and `firewall-irr` stay untouched. Second, `leaf tcp-flags` is declared in `ze-firewall-conf.yang` (`grouping from-block`) with a YANG-to-Go parity test, unlocking the existing parser, VPP translation, and nftables lowering (with a `meta l4proto tcp` guard added to `lowerTCPFlagsMatch` so terms omitting `protocol tcp` only match TCP). Third, `Set.Validate()` refuses contradictory set flags (`flags-constant` with `flags-dynamic` or `flags-timeout`), per-element `timeout` without `flags-timeout`, CIDR prefixes without `flags-interval`, host bits set in a prefix, IP family mismatches, and overlapping intervals at `ze config validate`, commit, and reload (`VerifyConfig`, `OnConfigVerify`) with `opererr` messages naming the fix; at `ze start`, `firewall` complies with `Ze always starts` (`config.RegisterServiceBlock` + `OnConfigure` handling) by leaving an invalid firewall unstarted, logging an `ERROR` record under `config-plugin-verify` with the fix, and keeping Ze and its other services running while `ze doctor` (`config-semantics`) reports the same code and fix.

### Before and after

1. **CIDR prefix in a `flags-interval` set:**
   - Before: `set nets { type ipv4; flags-interval; element 192.0.2.0/24 {} }` programmed a single bare address `192.0.2.0` into nftables (prefix length silently discarded by `encodeSetElementKey`), matching only one IP.
   - After: `lowerSet` emits `[192.0.2.0, 192.0.3.0)` with `IntervalEnd: true`, and `nft list table` displays `192.0.2.0/24`.
2. **CIDR prefix without `flags-interval`:**
   - Before: `set hosts { type ipv4; element 192.0.2.0/24 {} }` passed `ze config validate` and silently programmed `192.0.2.0`.
   - After: `ze config validate`, commit, and reload refuse it with an `opererr` naming the set, `192.0.2.0/24`, and the fix (`flags-interval;` on the set or a bare address); if present in a boot file, `ze start` stays up without starting the invalid firewall and reports it under `code=config-plugin-verify` in the log and `ze doctor`.
3. **Element `timeout` without `flags-timeout`, or contradictory `flags-constant` combinations:**
   - Before: `element 10.0.0.1 { timeout 3600; }` without `flags-timeout` committed and silently dropped the timeout; `flags-constant` + `flags-dynamic` or `flags-timeout` passed validate and failed at kernel flush with `-EINVAL`.
   - After: `ze config validate`, commit, and reload refuse each with an `opererr` naming the set, offending flag/element, and fix; at `ze start`, the invalid firewall is left unstarted and reported under `code=config-plugin-verify` while Ze keeps running.
4. **Firewall term `from { tcp-flags syn; }`:**
   - Before: rejected by YANG schema validation as an unknown leaf under `from`, despite `ParseTCPFlags`, `lowerTCPFlagsMatch`, VPP `translate`, and `docs/guide/firewall.md` supporting it.
   - After: accepted by YANG, validated in `ip`/`ip6`/`inet` (refused in `arp`), and programmed in nftables with `meta l4proto tcp` and `tcp flags` (and in VPP ACL rules with `Proto = TCP`).

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Expand CIDR prefixes into `{start, end+IntervalEnd}` pairs inside `parseSetElements` (`config.go`) | Breaks `TestParseSetElements` (which expects original CIDR strings in `Set.Elements`) and leaks raw `IntervalEnd` sentinel rows into `show firewall ruleset`, `show firewall group`, and the web UI element counts |
| Detect pre-expanded `copp`/`irr` interval sets solely by checking whether any element has `IntervalEnd == true` | Fails for a `copp` neighbor set whose only prefix in a family reaches the top of the address space (such as `range 0.0.0.0/0`), where `copp` emits one `{Value: "0.0.0.0"}` element with no `IntervalEnd`; adding `Set.RawIntervals` removes the ambiguity cleanly |
| Enforce contradictory flags and timeout-without-flag inside `lowerSet` instead of `Set.Validate()` | Breaks existing direct `lowerSet` unit tests (`TestLowerSetTimeoutFlagAndElementTimeouts`, `TestLowerSetCarriesConstantAndDynamic`) and would fail only at kernel apply instead of offline `ze config validate` |
| Remove `ParseTCPFlags` and `tcp-flags` from `firewall` instead of declaring the YANG leaf | `MatchTCPFlags` is already wired in both `nft` and `vpp` backends, documented in `docs/guide/firewall.md`, and used by `policyroute` and `ddos-local`; declaring the one missing YANG leaf gives operators full parity across both data planes |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- declare `typedef tcp-flags` and `leaf tcp-flags` in `internal/component/firewall/yang/ze-firewall-conf.yang`, export `TCPFlagNames()` in `internal/component/firewall/config.go`, add `RawIntervals` to `firewall.Set` (`model.go`), register the two snapshot fixtures in `internal/test/fixture/netfilter_fixture.go`, and write the failing parity, unit, and `.ci` wiring tests (`tcp_flags_test.go`, `firewall-set-interval-cidr.ci`, `firewall-tcp-flags.ci`, `firewall-set-flags-validate.ci`).
   - Tests: `TestTCPFlagsYANGPatternMatchesGo`, `firewall-set-interval-cidr.ci`, `firewall-tcp-flags.ci`, `firewall-set-flags-validate.ci`
   - Files: `internal/component/firewall/yang/ze-firewall-conf.yang`, `internal/component/firewall/config.go`, `internal/component/firewall/model.go`, `internal/test/fixture/netfilter_fixture.go`, `internal/component/firewall/yang/tcp_flags_test.go`
   - Verify: YANG parity test passes; validation and lowering wiring tests fail red before implementation.
2. **Phase: Set-Flag and Element Validation + `Ze always starts` Startup Isolation** -- implement full `Set.Validate()` checks in `internal/component/firewall/model.go` (contradictory flags, element `Timeout` without `SetFlagTimeout`, `IntervalEnd` without `SetFlagInterval`, CIDR prefix without `SetFlagInterval`, host bits set, IP family mismatch, overlapping intervals, `RawIntervals` passthrough), extract `configureAtStart` in `internal/component/firewall/engine.go` so invalid firewall configs at `ze start` leave the firewall unstarted and log `ERROR` under `diagnostic.CodeConfigPluginVerify` while returning `nil`, register `config.RegisterServiceBlock(ConfigRoot, "firewall")` in `internal/component/firewall/register_producer_verify.go`, and mark `neighborSet` with `RawIntervals: true` in `internal/plugins/copp/translate.go`.
   - Tests: `TestSetValidateFlagsAndElements`, `TestConfigureAtStartLeavesFirewallDownAndLogsDoctorCode`, `firewall-set-flags-validate.ci`, existing `internal/component/firewall` and `internal/plugins/copp` tests
   - Files: `internal/component/firewall/model.go`, `internal/component/firewall/model_test.go`, `internal/component/firewall/engine.go`, `internal/component/firewall/engine_test.go`, `internal/component/firewall/register_producer_verify.go`, `internal/plugins/copp/translate.go`
   - Verify: new validation and start-isolation unit tests fail before implementation and pass after; `firewall-set-flags-validate.ci` passes.
3. **Phase: Interval CIDR Lowering and TCP Flags Guard** -- update `encodeSetElementKey` (refuse CIDR prefixes, check IPv4/IPv6 family before `As4()`/`As16()`), `lowerSet` (expand unexpanded IPv4/IPv6 `flags-interval` sets into numerically sorted half-open `[start, exclusiveEnd)` intervals, omit `IntervalEnd` at top of address space, place timeout only on start element, preserve pre-expanded sets), and `lowerTCPFlagsMatch` (prepend `meta l4proto tcp`) in `internal/plugins/firewall/nft/lower_linux.go`.
   - Tests: `TestLowerSetIntervalCIDRExpansion`, `TestEncodeSetElementKeyRejectsCIDRAndWrongFamily`, `TestLowerTCPFlagsMatchEmitsL4ProtoGuard`, `TestNftIntegration_IntervalCIDRSetAndTCPFlags`, `firewall-set-interval-cidr.ci`, `firewall-tcp-flags.ci`, `policy-tcp-flags.ci`
   - Files: `internal/plugins/firewall/nft/lower_linux.go`, `internal/plugins/firewall/nft/lower_linux_test.go`, `internal/plugins/firewall/nft/integration_linux_test.go`
   - Verify: unit tests fail before implementation and pass after; kernel integration and `.ci` tests pass on Linux.
4. **Phase: Documentation and YANG Descriptions** -- update `flags-interval`, `flags-timeout`, `flags-constant`, `flags-dynamic`, and `element/timeout` descriptions in `ze-firewall-conf.yang`, fix `docs/guide/firewall.md`, and drop the conditional `network-group` workaround and stale `tcp-flags` note in `docs/guide/migrating-from-vyos.md`.
   - Tests: `./le yang glue check`, `./le doc check all`, `./le config claims`
   - Files: `internal/component/firewall/yang/ze-firewall-conf.yang`, `docs/guide/firewall.md`, `docs/guide/migrating-from-vyos.md`
   - Verify: all doc anchors, YANG glue, and config claims checks pass.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-5 has a concrete implementation at `file:line` and passing tests |
| Feature completeness | All three end-to-end user stories work from YANG config through kernel nftables or `ze config validate` |
| Correctness | Top-of-space prefixes (`255.255.255.0/24`, `0.0.0.0/0`, `ffff::/16`) emit no wrapped `0.0.0.0`/`::` `IntervalEnd`; `Timeout` is never placed on an `IntervalEnd` sentinel; `copp` lone `0.0.0.0/0` is preserved via `RawIntervals: true`; `encodeSetElementKey` never panics on IPv4/IPv6 mismatch |
| Naming | `typedef tcp-flags` and `leaf tcp-flags` match `ze-policyroute-conf.yang` and `ParseTCPFlags` (`fin`, `syn`, `rst`, `psh`, `ack`, `urg`); no `ze:backend` annotation blocks VPP `MatchTCPFlags` |
| Data flow | Model `Set.Elements` keeps operator strings for `show firewall` and web UI; interval boundary expansion happens only in `lowerSet` |
| Operator errors and doctor codes | Every new refusal in `Set.Validate()` uses `opererr.New` naming the set, offending flag/element, reason, and exact fix on the last line; `OnConfigure` at `ze start` logs `ERROR` under `diagnostic.CodeConfigPluginVerify` (`config-plugin-verify`) with the same fix so it matches `ze doctor`'s `config-semantics` check without adding a duplicate doctor check |
| Rule: `ai/rules/protocol.md` (exact or reject) | Zero silent stripping of CIDR prefix length, zero silent dropping of element timeout without `flags-timeout`, zero silent acceptance of contradictory flags |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `Set.Validate()` flags/elements/CIDR checks + `Ze always starts` `OnConfigure` isolation | `./le test unit pkg path ./internal/component/firewall/ run "TestSetValidateFlagsAndElements\|TestConfigureAtStartLeavesFirewallDownAndLogsDoctorCode"` |
| `typedef tcp-flags` YANG parity | `./le test unit pkg path ./internal/component/firewall/yang/ run TestTCPFlagsYANGPatternMatchesGo` |
| `lowerSet` CIDR expansion + `lowerTCPFlagsMatch` L4 proto guard | `./le test unit pkg path ./internal/plugins/firewall/nft/ run "TestLowerSetIntervalCIDRExpansion\|TestEncodeSetElementKeyRejectsCIDRAndWrongFamily\|TestLowerTCPFlagsMatchEmitsL4ProtoGuard"` |
| `copp` `RawIntervals` compatibility | `./le test unit pkg path ./internal/plugins/copp/` |
| Kernel nftables integration | `./le test integration firewall` (on Linux with `CAP_NET_ADMIN` or QEMU) |
| Functional `.ci` proofs | `./le test firewall` (`firewall-set-interval-cidr.ci`, `firewall-tcp-flags.ci`, `firewall-set-flags-validate.ci`) and `./le test policy` (`policy-tcp-flags.ci`) |
| YANG glue & doc anchors | `./le yang glue check` and `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Non-canonical CIDR prefixes, malformed IPs, IPv4-in-IPv6/wrong-family addresses, and overlapping intervals are rejected before netlink marshaling (no panic in `netip.Addr.As4`/`As16`, no kernel `EEXIST` aborting all owners' tables in `ApplyAll`) |
| Fail-closed filtering | A CIDR prefix never silently narrows to a single host address (which would leave 255/256 of a blocked `/24` unfiltered), and `tcp-flags` never matches arbitrary UDP/ICMP payload bytes when `protocol tcp` is omitted |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood -> RESEARCH |
| Lint failure | Fix inline. If architectural -> DESIGN |
| Functional test fails | Check the AC: wrong AC -> DESIGN, correct AC -> IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- Auditing HEAD before designing showed that `flags-dynamic`, `flags-timeout`, and `flags-constant` lowering already landed (`lower_linux.go:261-290`), leaving the two 2026-11-07 journal rows and verify-time set validation as the real remaining work.
- Reading `internal/plugins/copp/translate.go` (`intervalElements`) revealed that CoPP omits `IntervalEnd` for top-of-space prefixes (including `0.0.0.0/0`), which would collide with bare host IP expansion in `lowerSet` unless `Set.RawIntervals` distinguishes pre-expanded boundary slices from config elements.
- Reading `internal/plugins/firewall/nft/lower_linux_test.go` (`TestLowerSetTimeoutFlagAndElementTimeouts:130`, `TestLowerSetCarriesConstantAndDynamic:1991`) showed those tests exercise `lowerSet` directly with unvalidated flag combinations, confirming semantic flag checks belong in `Set.Validate()` rather than `lowerSet`.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `validated-value-discarded-by-its-caller.md` | 2026-11-07 | `encodeSetElementKey` lowers a `flags-interval` prefix element to its bare address; no config path sets `IntervalEnd` |
| `unwired-feature.md` | 2026-11-07 | firewall term `tcp-flags` parsed by `firewall/config.go` (`ParseTCPFlags`), absent from the firewall YANG `from-block` |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Expand config-originated `flags-interval` CIDR/host elements inside `lowerSet` rather than `parseSetElements` | Expand in `parseSetElements` (`config.go`) | Preserves human-readable CIDR strings in `Set.Elements`, `LastApplied()`, `show firewall`, web UI, and `TestParseSetElements` |
| Add `Set.RawIntervals` (set by `copp` `neighborSet`) alongside `hasIntervalEnd` check in `lowerSet` and `Set.Validate()` | Infer pre-expanded state solely from `IntervalEnd == true` | Prevents misreading a lone top-of-space CoPP prefix (`0.0.0.0/0` with no `IntervalEnd`) as a `/32` host address |
| Enforce contradictory flags and element `timeout` without `flags-timeout` in `Set.Validate()` rather than `lowerSet`, and isolate start-time failures in `OnConfigure` | Reject inside `lowerSet`, or let `OnConfigure` return a fatal startup error | Runs at offline `ze config validate`, commit, reload (`OnConfigVerify`), and `ze doctor` (`config-semantics`) via `ValidateTables`, preserves direct `lowerSet` unit tests, and obeys `Ze always starts` at `ze start` by leaving an invalid firewall unstarted and logging `code=config-plugin-verify` |
| Declare `leaf tcp-flags` in `ze-firewall-conf.yang` (unannotated by `ze:backend`) and prepend `meta l4proto tcp` in `lowerTCPFlagsMatch` | Delete `ParseTCPFlags` / `tcp-flags` or leave `lowerTCPFlagsMatch` unguarded | Both `kernel` and `vpp` backends already implement `MatchTCPFlags`; adding `meta l4proto tcp` matches `lowerICMPTypeMatch` and VPP `translate.go` so rules omitting `protocol tcp` never match non-TCP bytes |

## Known Limitations
- Packet-triggered runtime insertion into `flags-dynamic` sets (`expr.Dynset`) is deliberately out of scope and owned by `plan/post-release/features/spec-firewall-dynamic-address-group.md`.
- Named sets remain kernel-only (VPP ACL verifier continues to reject all named sets at verify).

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI report gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: Scope is `config`)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-firewall-dynamic-set-config-fidelity.md` only, in the same `./le commit create` script (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- Added `Set.RawIntervals` (`internal/component/firewall/model.go`) and preserved it in `deepCopyTables` (`internal/component/firewall/accessor.go`), marking CoPP's pre-expanded `neighborSet` with `RawIntervals: true` (`internal/plugins/copp/translate.go`) so a lone top-of-space CoPP range (`0.0.0.0/0`) is never treated as a bare `/32` host address.
- Strengthened `Set.Validate()` (`internal/component/firewall/model.go` `validateFlags`, `validateElements`, `validatePreExpandedAddressElements`, `validateAddressElements`, `validateCIDRElement`, `validateAddrFamily`, `validateNoOverlappingIntervals`) to return `*opererr.Error` refusals for contradictory flags (`flags-constant` with `flags-dynamic` or `flags-timeout`), per-element `timeout` without `flags-timeout`, `IntervalEnd` without `flags-interval`, CIDR prefixes without `flags-interval`, non-canonical CIDR prefixes with host bits set, wrong-family/zoned/IPv4-in-IPv6 elements, and overlapping intervals.
- Implemented `Ze always starts` startup isolation for firewall config failures: registered `config.RegisterServiceBlock(ConfigRoot, "firewall")` (`internal/component/firewall/register_producer_verify.go`) and extracted `configureAtStart` / `idleFirewallConfig` / `logStartConfigFailure` (`internal/component/firewall/engine.go`) so parse or validation failures at `ze start` leave the firewall unstarted (idle `activeCfg`, zero partial/approximated tables applied), log an `ERROR` record carrying `code=config-plugin-verify` (`diagnostic.CodeConfigPluginVerify`) and the `opererr` fix, and return `nil`, while `VerifyConfig`, commit, reload (`OnConfigVerify`), and `ze doctor` (`config.ValidateSemantics`) remain strictly refusing/reporting under `config-plugin-verify`.
- Declared `typedef tcp-flags` and `leaf tcp-flags` (without `ze:backend`, so both `kernel` and `vpp` accept it) in `grouping from-block` of `internal/component/firewall/yang/ze-firewall-conf.yang`, updated set-flag and `element/timeout` YANG descriptions, exported `TCPFlagNames()` (`internal/component/firewall/config.go`), and added `TestTCPFlagsYANGPatternMatchesGo` (`internal/component/firewall/yang/tcp_flags_test.go`).
- Updated `lowerSet` (`shouldExpandAddressIntervals`, `lowerAddressIntervalElements`, `parseIntervalElement`, `validateSetAddrFamily`, `lastPrefixAddr`, `addrKeyBytes`), `encodeSetElementKey`, and `lowerTCPFlagsMatch` (`internal/plugins/firewall/nft/lower_linux.go`) to expand unexpanded IPv4/IPv6 `flags-interval` sets to numerically sorted half-open `[start, exclusiveEnd)` nftables elements with `IntervalEnd: true` (omitting `IntervalEnd` at top of space, attaching `Timeout` only on start elements, passing pre-expanded `copp`/`irr` sets through unchanged), refuse CIDR and family mismatches in `encodeSetElementKey` before `As4()`/`As16()`, and prepend `meta l4proto tcp` in `lowerTCPFlagsMatch`.
- Registered table snapshot fixtures (`firewall/firewall-set-interval-cidr`, `firewall/firewall-tcp-flags`) in `internal/test/fixture/netfilter_fixture.go` and promoted three functional `.ci` tests (`test/firewall/firewall-set-interval-cidr.ci`, `test/firewall/firewall-tcp-flags.ci`, `test/firewall/firewall-set-flags-validate.ci`) after 80/80 stress-repro under load.
- Updated both owned journal rows (`plan/journal/validated-value-discarded-by-its-caller.md` 2026-11-07 and `plan/journal/unwired-feature.md` 2026-11-07) to `fixed:`, cleared citations in `plan/post-release/features/spec-firewall-dynamic-address-group.md`, and wrote `website/changes/fragments/firewall-dynamic-set-config-fidelity.md`.

### Bugs Found/Fixed
- `deepCopyTables` (`internal/component/firewall/accessor.go:90`) copied `Set` field-by-field and would have dropped `Set.RawIntervals` in `LastApplied()` snapshots; fixed to copy `RawIntervals: s.RawIntervals`.
- `encodeSetElementKey` (`internal/plugins/firewall/nft/lower_linux.go`) previously stripped CIDR masks and called `As4()`/`As16()` without checking address family; fixed and covered by `TestEncodeSetElementKeyRejectsCIDRAndWrongFamily`.

### Documentation Updates
- `docs/guide/firewall.md`: added `### TCP Flags` (anchors `ze-firewall-conf.yang -- typedef tcp-flags, leaf tcp-flags`, `config.go -- ParseTCPFlags, parseFromBlock`, `lower_linux.go -- lowerTCPFlagsMatch`, `vpp/translate.go -- applyMatch`) and updated `## Named Sets` to add `flags-timeout;`, add a `flags-interval` CIDR example, document verify-time set-flag/element rules, and document `Ze always starts` (`config-plugin-verify`) (anchors `ze-firewall-conf.yang -- list set, flags-interval, flags-timeout`, `lower_linux.go -- lowerSet, lowerAddressIntervalElements`, `model.go -- Set.Validate, validateFlags, validateElements, validateCIDRElement, validateNoOverlappingIntervals`, `engine.go -- configureAtStart, VerifyConfig`).
- `docs/guide/migrating-from-vyos.md`: replaced the conditional one-term-per-prefix `network-group` workaround with `set ... flags-interval` (`equivalent`), removed the defect paragraph and interim-rewrite entry citing this spec, and updated the stale `tcp-flags` note (anchors `lower_linux.go -- lowerSet, lowerAddressIntervalElements`, `model.go -- Set.Validate`, `ze-firewall-conf.yang -- leaf tcp-flags, tcp-mss-set, tcp-mss-clamp-to-pmtu`).
- `./le doc check all`: 0 errors on all firewall/copp/VyOS-guide pages and source anchors (34 pre-existing anchor errors and 2 unrelated checks belong to other sessions' BGP/fleet/policy files). `./le doc ste check` on `docs/guide/firewall.md` and `docs/guide/migrating-from-vyos.md`: OK (0 errors).

### Deviations from Plan
- Modified `internal/component/firewall/accessor.go` (`deepCopyTables` preserves `RawIntervals`) — not listed in original Files to Modify; caught during Phase 1 wiring audit so `LastApplied()` keeps `RawIntervals`.
- Modified `internal/plugins/copp/translate_test.go` (added `RawIntervals` assertion in `TestTranslatePolicyNeighborSetIntervals` and added `TestTranslatePolicyLoneTopOfSpaceNeighborSet`) — extra regression coverage for R-1 beyond the original Files to Modify list.
- `internal/component/firewall/yang/embed.go` and `register.go` needed no changes after `./le yang glue check` (already embed `ze-firewall-conf.yang` via `//go:embed`).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| none | | | | |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| 1. Lower IPv4/IPv6 CIDR prefixes and bare hosts in `flags-interval` sets to sorted half-open `[start, exclusiveEnd)` with `IntervalEnd`, refuse CIDR without `flags-interval`, update VyOS migration guide | Done | `internal/plugins/firewall/nft/lower_linux.go` (`lowerSet`, `shouldExpandAddressIntervals`, `lowerAddressIntervalElements`, `encodeSetElementKey`), `internal/component/firewall/model.go` (`Set.Validate`), `docs/guide/migrating-from-vyos.md` | Preserves pre-expanded `copp`/`irr` sets via `Set.RawIntervals`/`hasIntervalEnd` |
| 2. Declare `typedef tcp-flags` and `leaf tcp-flags` in `ze-firewall-conf.yang` `grouping from-block` with YANG-to-Go parity test, and add `meta l4proto tcp` guard in `lowerTCPFlagsMatch` | Done | `internal/component/firewall/yang/ze-firewall-conf.yang`, `internal/component/firewall/config.go` (`TCPFlagNames`), `internal/component/firewall/yang/tcp_flags_test.go`, `internal/plugins/firewall/nft/lower_linux.go` (`lowerTCPFlagsMatch`) | No `ze:backend` annotation so both `kernel` and `vpp` accept it |
| 3. Enforce verify-time set-flag and element validation in `Set.Validate()` with `Ze always starts` startup isolation in `OnConfigure` and `config.RegisterServiceBlock` | Done | `internal/component/firewall/model.go` (`Set.Validate`), `internal/component/firewall/engine.go` (`configureAtStart`, `idleFirewallConfig`, `logStartConfigFailure`), `internal/component/firewall/register_producer_verify.go` | Validate/commit/reload strictly refuse; `ze start` leaves firewall down and logs `ERROR` under `code=config-plugin-verify` matching `ze doctor` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestLowerSetIntervalCIDRExpansion`, `TestNftIntegration_IntervalCIDRSetAndTCPFlags`, `test/firewall/firewall-set-interval-cidr.ci` | Verified against real Linux nftables (`nft list table` displays `192.0.2.0/24`, `198.51.100.1`, `2001:db8::/32`); `copp` and `irr` preserved |
| AC-2 | Done | `TestSetValidateFlagsAndElements`, `TestEncodeSetElementKeyRejectsCIDRAndWrongFamily`, `TestConfigureAtStartLeavesFirewallDownAndLogsDoctorCode`, `test/firewall/firewall-set-flags-validate.ci` | Rejects CIDR without `flags-interval`, host bits set, wrong family/zone/IPv4-in-IPv6, overlapping intervals; leaves firewall down at `ze start` under `config-plugin-verify` |
| AC-3 | Done | `TestSetValidateFlagsAndElements`, `TestConfigureAtStartLeavesFirewallDownAndLogsDoctorCode`, `test/firewall/firewall-set-flags-validate.ci`, `test/firewall/firewall-set-element-timeout.ci` | Rejects `flags-constant` + `flags-dynamic`/`flags-timeout` and element `timeout` without `flags-timeout`; preserves valid combinations and direct `lowerSet` unit tests |
| AC-4 | Done | `TestTCPFlagsYANGPatternMatchesGo`, `TestLowerTCPFlagsMatchEmitsL4ProtoGuard`, `TestNftIntegration_IntervalCIDRSetAndTCPFlags`, `test/firewall/firewall-tcp-flags.ci`, `test/policy/policy-tcp-flags.ci` | `nft list table` shows `tcp flags` without `@th`; `arp` refusal preserved by `validate.go` `ipHeaderMatchLeaf` |
| AC-5 | Done | `docs/guide/firewall.md`, `docs/guide/migrating-from-vyos.md`, `internal/component/firewall/yang/ze-firewall-conf.yang`, `./le doc ste check`, `./le doc check all`, `./le yang glue check`, `./le config claims` | All doc claims anchored to producing symbols and STE clean |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestSetValidateFlagsAndElements` | Done | `internal/component/firewall/model_test.go` | PASS (`./le test unit pkg path ./internal/component/firewall/`) |
| `TestConfigureAtStartLeavesFirewallDownAndLogsDoctorCode` | Done | `internal/component/firewall/engine_test.go` | PASS (`./le test unit pkg path ./internal/component/firewall/`) |
| `TestTCPFlagsYANGPatternMatchesGo` | Done | `internal/component/firewall/yang/tcp_flags_test.go` | PASS (`./le test unit pkg path ./internal/component/firewall/yang/ run TestTCPFlagsYANGPatternMatchesGo`) |
| `TestLowerSetIntervalCIDRExpansion` | Done | `internal/plugins/firewall/nft/lower_linux_test.go` | PASS (`./le test unit pkg path ./internal/plugins/firewall/nft/`) |
| `TestEncodeSetElementKeyRejectsCIDRAndWrongFamily` | Done | `internal/plugins/firewall/nft/lower_linux_test.go` | PASS (`./le test unit pkg path ./internal/plugins/firewall/nft/`) |
| `TestLowerTCPFlagsMatchEmitsL4ProtoGuard` | Done | `internal/plugins/firewall/nft/lower_linux_test.go` | PASS (`./le test unit pkg path ./internal/plugins/firewall/nft/`) |
| `TestNftIntegration_IntervalCIDRSetAndTCPFlags` | Done | `internal/plugins/firewall/nft/integration_linux_test.go` | PASS against real Linux kernel nftables (19.5s) |
| `firewall-set-interval-cidr` | Done | `test/firewall/firewall-set-interval-cidr.ci` | PASS (699ms, 80/80 under load) |
| `firewall-tcp-flags` | Done | `test/firewall/firewall-tcp-flags.ci` | PASS (714ms, 80/80 under load) |
| `firewall-set-flags-validate` | Done | `test/firewall/firewall-set-flags-validate.ci` | PASS (2.4s, 80/80 under load) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/firewall/model.go` | Done | Added `RawIntervals` and full `Set.Validate()` checks |
| `internal/component/firewall/engine.go` | Done | Extracted `configureAtStart`, `idleFirewallConfig`, `logStartConfigFailure` |
| `internal/component/firewall/register_producer_verify.go` | Done | Registered `config.RegisterServiceBlock(ConfigRoot, "firewall")` |
| `internal/component/firewall/config.go` | Done | Exported `TCPFlagNames()` + enumeration gate marker |
| `internal/component/firewall/accessor.go` | Done (added) | `deepCopyTables` copies `RawIntervals` |
| `internal/component/firewall/yang/ze-firewall-conf.yang` | Done | Added `typedef tcp-flags`, `leaf tcp-flags`, updated set-flag/element descriptions |
| `internal/component/firewall/yang/tcp_flags_test.go` | Done | Created YANG-to-Go parity test |
| `internal/component/firewall/model_test.go` | Done | Added `TestSetValidateFlagsAndElements` |
| `internal/component/firewall/engine_test.go` | Done | Added `TestConfigureAtStartLeavesFirewallDownAndLogsDoctorCode` |
| `internal/plugins/copp/translate.go` | Done | Set `RawIntervals: true` on `neighborSet` |
| `internal/plugins/copp/translate_test.go` | Done (added) | Added `RawIntervals` assertion + `TestTranslatePolicyLoneTopOfSpaceNeighborSet` |
| `internal/plugins/firewall/nft/lower_linux.go` | Done | Interval CIDR expansion in `lowerSet`, fail-closed `encodeSetElementKey`, `meta l4proto tcp` in `lowerTCPFlagsMatch` |
| `internal/plugins/firewall/nft/lower_linux_test.go` | Done | Added 3 unit test suites |
| `internal/plugins/firewall/nft/integration_linux_test.go` | Done | Added `TestNftIntegration_IntervalCIDRSetAndTCPFlags` |
| `internal/test/fixture/netfilter_fixture.go` | Done | Registered `firewall-set-interval-cidr` and `firewall-tcp-flags` snapshot fixtures |
| `test/firewall/firewall-set-interval-cidr.ci` | Done | Created and promoted after 80/80 stress-repro |
| `test/firewall/firewall-tcp-flags.ci` | Done | Created and promoted after 80/80 stress-repro |
| `test/firewall/firewall-set-flags-validate.ci` | Done | Created and promoted after 80/80 stress-repro |
| `docs/guide/firewall.md` | Done | Added TCP Flags section, updated Named Sets examples and validation/startup docs |
| `docs/guide/migrating-from-vyos.md` | Done | Replaced `network-group` workaround, updated stale `tcp-flags` note |

### Audit Summary
- **Total items:** 28 (3 Task requirements, 5 ACs, 10 TDD tests, 10+ planned/supporting files)
- **Done:** 28
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (added `accessor.go` `deepCopyTables` preservation of `RawIntervals` and `copp/translate_test.go` R-1 regression test; recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| 1. Lower IPv4 and IPv6 CIDR prefixes and bare host addresses in `flags-interval` sets to sorted half-open `[start, exclusiveEnd)` intervals with `IntervalEnd`, refuse CIDR without `flags-interval`, and drop the VyOS workaround | functional + kernel integration | `test/firewall/firewall-set-interval-cidr.ci` PASS (699ms, 80/80 under load; real `nft list table` displays `192.0.2.0/24`, `198.51.100.1`, `2001:db8::/32`), `TestNftIntegration_IntervalCIDRSetAndTCPFlags` PASS (real kernel readback incl. adjacent + top-of-space + timeout), `test/firewall/firewall-set-flags-validate.ci` PASS (refuses CIDR without `flags-interval`), `docs/guide/migrating-from-vyos.md` updated |
| 2. Declare `typedef tcp-flags` and `leaf tcp-flags` in `ze-firewall-conf.yang` `from-block` with YANG-to-Go parity and `meta l4proto tcp` guard in `lowerTCPFlagsMatch` | functional + kernel integration + unit | `TestTCPFlagsYANGPatternMatchesGo` PASS, `TestLowerTCPFlagsMatchEmitsL4ProtoGuard` PASS, `TestNftIntegration_IntervalCIDRSetAndTCPFlags` PASS (`requireTCPFlagsRuleGuarded`), `test/firewall/firewall-tcp-flags.ci` PASS (714ms, 80/80 under load; `tcp flags`, `syn`, `ack`, no `@th`), `test/policy/policy-tcp-flags.ci` PASS (671ms) |
| 3. Enforce verify-time set-flag and element validation in `Set.Validate()` with `Ze always starts` startup isolation under `code=config-plugin-verify` | functional + unit | `TestSetValidateFlagsAndElements` PASS, `TestConfigureAtStartLeavesFirewallDownAndLogsDoctorCode` PASS (`configureAtStart` returns `nil`, applies 0 tables, logs `ERROR` with `code=config-plugin-verify` and fix; `VerifyConfig`, reload, and `config.ValidateSemantics` refuse/report), `test/firewall/firewall-set-flags-validate.ci` PASS (2.4s, 80/80 under load; 4 `ze config validate` exit-1 refusals + `ze -` exit-0 boot logging `code=config-plugin-verify`) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Packet-triggered dynamic set insertion (`expr.Dynset`), action validation, and action-inserted entry expiry | Split into post-release feature scope on 2026-10-22; explicitly out of scope for this release fix spec | `plan/post-release/features/spec-firewall-dynamic-address-group.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/firewall-dynamic-set-config-fidelity-5df1d1e6-ffed-4b7c-9294-8a109bfad051.md` |
| `./le spec review check` | clean (18 code files pinned in artifact, verdict=clean, rounds=2) |
| Rounds | 2 |
| Reviewer lenses used | logic+wiring, security+edge-cases, design-docs+style, spec-completeness (`/ze-review` + `/ze-review-spec`) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | none | 0 BLOCKER, 0 ISSUE (2 NOTEs: NOTE-1 `./le repo check` reports `TCPFlagNames` at `internal/component/firewall/config.go:1237` as having no cross-package non-test caller — intentional per approved spec as the `enumeration: gated by internal/component/firewall/yang:TestTCPFlagsYANGPatternMatchesGo` parity helper; NOTE-2 tiny private `hasIntervalEnd`/`lastPrefixAddr` helpers exist in both `model.go` and `lower_linux.go` to keep internal helpers unexported across the component/backend boundary) | `internal/component/firewall/config.go:1237`, `internal/component/firewall/model.go`, `internal/plugins/firewall/nft/lower_linux.go` | none needed (NOTE-only; style pass run over all 13 changed Go files: zero peer-reachable panics, bounded loops, clean naming) |

### Run 2 (post-gate style fixes)
| Scope | Verdict | Findings | Checks |
|-------|---------|----------|--------|
| `internal/component/firewall/model.go` (`textbuf.Buffer` replacing 28 `fmt.Sprintf` calls; `SetTypeIPv4`/`SetTypeIPv6` guard replacing switch) and `internal/component/firewall/engine.go` (`refuse-by-default: guards runEngine` on `configureAtStart` boot-isolation branches and `guards OnConfigRollback` on nil journal) | CLEAN | 0 BLOCKER, 0 ISSUE (all `textbuf.Buffer` calls properly `Reset()` + `.String()` with no dangling slices or message drift; `SetType` check preserves `unknown(99)` diagnostic in `validateMatch`; annotations name callers) | `./le go style check` OK on both files; `./le arch refuse-by-default check` OK (0 refused); main-thread `./le test unit pkg path ./internal/component/firewall/` PASS and `./le go lint run scope ./internal/component/firewall/...` 0 issues |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/firewall/yang/tcp_flags_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 3688 Nov 27 04:16 internal/component/firewall/yang/tcp_flags_test.go` |
| `test/firewall/firewall-set-interval-cidr.ci` | Yes | `-rw-r--r-- 1 thomas thomas 1870 Nov 27 04:14 test/firewall/firewall-set-interval-cidr.ci` |
| `test/firewall/firewall-tcp-flags.ci` | Yes | `-rw-r--r-- 1 thomas thomas 1642 Nov 27 04:46 test/firewall/firewall-tcp-flags.ci` |
| `test/firewall/firewall-set-flags-validate.ci` | Yes | `-rw-r--r-- 1 thomas thomas 2773 Nov 27 05:42 test/firewall/firewall-set-flags-validate.ci` |
| `website/changes/fragments/firewall-dynamic-set-config-fidelity.md` | Yes | `-rw-r--r-- 1 thomas thomas 2576 Nov 27 07:00 website/changes/fragments/firewall-dynamic-set-config-fidelity.md` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | `flags-interval` IPv4/IPv6 CIDR + bare host lowering with sorted `[start, exclusiveEnd)` and `IntervalEnd`, preserving `copp`/`irr` | `internal/plugins/firewall/nft/lower_linux.go` (`lowerSet`, `shouldExpandAddressIntervals`, `lowerAddressIntervalElements`), `internal/plugins/copp/translate.go` (`neighborSet` `RawIntervals: true`); main-thread verified `TestLowerSetIntervalCIDRExpansion`, `TestNftIntegration_IntervalCIDRSetAndTCPFlags`, `firewall-set-interval-cidr.ci` PASS (699ms, 80/80) |
| AC-2 | Refusal of CIDR without `flags-interval`, host bits set, wrong family/zone/IPv4-in-IPv6, overlapping intervals + `Ze always starts` boot isolation + fail-closed `encodeSetElementKey` | `internal/component/firewall/model.go` (`Set.Validate`, `validateAddressElements`, `validateCIDRElement`, `validateAddrFamily`, `validateNoOverlappingIntervals`), `engine.go` (`configureAtStart`), `lower_linux.go` (`encodeSetElementKey`); main-thread verified `TestSetValidateFlagsAndElements`, `TestEncodeSetElementKeyRejectsCIDRAndWrongFamily`, `TestConfigureAtStartLeavesFirewallDownAndLogsDoctorCode`, `firewall-set-flags-validate.ci` PASS (2.4s, 80/80) |
| AC-3 | Refusal of `flags-constant` with `flags-dynamic`/`flags-timeout` and element `timeout` without `flags-timeout` + `Ze always starts` boot isolation | `internal/component/firewall/model.go` (`validateFlags`, `validateElements`), `engine.go` (`configureAtStart`, `logStartConfigFailure`); main-thread verified `TestSetValidateFlagsAndElements`, `TestConfigureAtStartLeavesFirewallDownAndLogsDoctorCode`, `firewall-set-flags-validate.ci` PASS, `firewall-set-element-timeout.ci` PASS (639ms) |
| AC-4 | `typedef tcp-flags` + `leaf tcp-flags` in `from-block` without `ze:backend` + `meta l4proto tcp` guard in `lowerTCPFlagsMatch` | `internal/component/firewall/yang/ze-firewall-conf.yang` (`leaf tcp-flags`), `lower_linux.go` (`lowerTCPFlagsMatch`), `validate.go` (`ipHeaderMatchLeaf`); main-thread verified `TestTCPFlagsYANGPatternMatchesGo`, `TestLowerTCPFlagsMatchEmitsL4ProtoGuard`, `TestNftIntegration_IntervalCIDRSetAndTCPFlags`, `firewall-tcp-flags.ci` PASS (714ms, 80/80), `policy-tcp-flags.ci` PASS (671ms) |
| AC-5 | `docs/guide/firewall.md`, `docs/guide/migrating-from-vyos.md`, and YANG descriptions match enforced behavior | Read both doc diffs and `ze-firewall-conf.yang`; `./le doc ste check` OK (0 errors); `./le doc check all` 0 errors on firewall/copp/VyOS docs; `./le yang glue check` and `./le config claims` PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `flags-interval` IPv4/IPv6 CIDR + bare host set via `ze -` -> kernel nftables | `test/firewall/firewall-set-interval-cidr.ci` | Yes (read file: runs `ze -`, fixture `firewall/firewall-set-interval-cidr` asserts `table inet ze_fw_cidr`, `flags interval`, `192.0.2.0/24`, `198.51.100.1`, `2001:db8::/32`; PASS 80/80) |
| `from { tcp-flags syn; }` and `from { protocol tcp; tcp-flags syn,ack; }` via `ze -` -> kernel nftables | `test/firewall/firewall-tcp-flags.ci` | Yes (read file: runs `ze -`, fixture `firewall/firewall-tcp-flags` asserts `table inet ze_fw_tcpflags`, `tcp flags`, `syn`, `ack`, rejects `@th`; PASS 80/80) |
| Invalid set flags/elements via `ze config validate` (4 refusal cases) and `ze -` boot (`Ze always starts`) | `test/firewall/firewall-set-flags-validate.ci` | Yes (read file: runs 4 `ze config validate -` exit-1 checks + `ze -` exit-0 awaiting `code=config-plugin-verify` and rejecting `firewall config applied` and `refusing to start`; PASS 80/80) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestNftIntegration_IntervalCIDRSetAndTCPFlags` PASS against real Linux nftables with adjacent `192.0.2.0/25` + `192.0.2.128/25` and trailing top-of-space `255.255.255.0/24` and `ffff::/16` (no `IntervalEnd` wrap) |
| A-2 | confirmed | `TestNftIntegration_IntervalCIDRSetAndTCPFlags` PASS against real Linux nftables with `SetFlagInterval|SetFlagTimeout` and `Timeout: 3600` / `600` on start elements and `Timeout: 0` on every `IntervalEnd` element |
| A-3 | confirmed | `test/policy/policy-tcp-flags.ci` PASS (671ms in Linux userns) and `test/firewall/firewall-tcp-flags.ci` PASS (714ms, 80/80 under load) |
| A-4 | confirmed | Grep across `test/` and `internal/component/firewall/` plus full `internal/component/firewall/`, `internal/plugins/copp/`, `internal/plugins/firewall/nft/`, and `test/firewall/` test runs all PASS with zero fixture failures |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/firewall.md` (`### TCP Flags`, `## Named Sets`) | Anchored to `ze-firewall-conf.yang`, `config.go`, `lower_linux.go`, `vpp/translate.go`, `model.go`, `engine.go`; `./le doc ste check` OK (0 errors), `./le doc check all` 0 firewall errors | Yes |
| `docs/guide/migrating-from-vyos.md` (`### Firewall and NAT`, `## Interim rewrites`, `### Conversion record`) | Anchored to `ze-firewall-conf.yang`, `lower_linux.go`, `model.go`; stale workaround and spec citation removed; `./le doc ste check` OK (0 errors) | Yes |
| `internal/component/firewall/yang/ze-firewall-conf.yang` (`typedef tcp-flags`, `leaf tcp-flags`, `list set`) | Matches `ParseTCPFlags` / `Set.Validate` / `lowerSet`; `./le yang glue check` PASS, `./le config claims` PASS | Yes |
| `plan/post-release/features/spec-firewall-dynamic-address-group.md` (citer cleanup) | `git grep spec-firewall-dynamic-set-config-fidelity plan/ docs/ test/ internal/` shows 0 remaining path citations outside this spec itself | Yes |
| `website/changes/fragments/firewall-dynamic-set-config-fidelity.md` | Front matter (`spec: firewall-dynamic-set-config-fidelity`, `topic: Firewall`, `feature: firewall`, `found-by: review`) and all 6 fixed headings present | Yes |
| All other `docs/` pages anchored to changed files (`core-design.md`, `cp-survival-2-copp-port179.md`, `firewall-irr.md`, `fw-6-firewall-vpp.md`, `table-ownership-and-shutdown-flush.md`, etc.) | Verified via `./le spec design documents` and `## Design documents` table: all `agrees` or `silent`, zero `departs`, zero stale source anchors | Yes |
