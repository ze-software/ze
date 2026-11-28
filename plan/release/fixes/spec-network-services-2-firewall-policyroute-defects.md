# Spec: Firewall and policy-route one-off defects

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P1 |
| Release reason | Fixes remaining shipped defects in VPP firewall ACL binding 255-count wrap, policy-route `from/protocol` YANG model test coverage, policy-route empty wildcard `interface "*"` verification, firewall self-jump/cycle/base-chain jump validation, and 12 `test/firewall/firewall-*.ci` expectation placements |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Split out of `plan/release/fixes/spec-network-services-one-off-defects.md` on 2026-11-26 so the firewall, VPP firewall backend, policy-route, and `test/firewall/*.ci` defects live in one focused release-fix spec with zero duplicate AC or journal ownership.

This spec owns five open defects:
1. VPP firewall `bindAllACLs` (`internal/plugins/firewall/vpp/backend_linux.go`) and its GoVPP adapter narrow `len(acls)` and `nInput` to `uint8` without refusing a merged total > 255, so 255 foreign input ACLs plus one desired ACL wraps both counts to 0 and silently detaches foreign ACLs.
2. Policy-route `from/protocol` YANG leaf (`internal/plugins/policyroute/yang/ze-policyroute-conf.yang`) is not gated by `TestParsedNamesMatchTheModel` (`internal/component/firewall/protocol_test.go`), unlike the firewall `protocol` leaf that shares `firewall.ianaProtocolNumbers`.
3. Policy-route bare `interface "*"` (`internal/plugins/policyroute/config.go` `parseIfaceSpec`) strips `*` to leave an empty `Name`, passing config verify and then failing `firewall.ApplyAll` (`lowerIfaceMatch` returns `errInterfaceNameMustNotBeEmpty`) for every firewall table owner (`copp`, `ddos-local`, `flowspec-firewall`, operator `firewall {}`).
4. Firewall `validateAction` (`internal/component/firewall/validate.go`) checks only that `Jump` and `Goto` targets exist in the table, accepting self-jumps, jump cycles (kernel `EMLINK`/`ELOOP`), and jumps/gotos to a base chain (kernel `EINVAL`) at `ze config validate` only to fail at kernel apply.
5. Twelve `test/firewall/firewall-*.ci` functional tests place `expect=stderr:contains=firewall config applied` under the fixture `cmd=` instead of under the `ze` `cmd=`.

Note on "Ze always starts" (`docs/architecture/core-design.md`, 2026-11-26): the new verify-time refusals in AC-3 (`policyroute` `interface "*"`) and AC-4 (`firewall` invalid `Jump`/`Goto` targets) MUST refuse in `ze config validate` and config reload (`OnConfigVerify`), while at `ze start` (`LoadConfigForStart`) `policy/route[*]` and `firewall` register with `config.RegisterServiceBlock` so a bad policy route or firewall table is excluded with an ERROR log and `ze doctor` finding under its doctor code rather than preventing `ze start` from bringing up networking.

## Work Not Done

- Network services plugins (flow export, DHCP/TFTP server, authoritative DNS, `resolve ping`): remain in `plan/release/fixes/spec-network-services-one-off-defects.md`.
- Interface, netlink monitor, DHCP client, VPP tunnels, traffic control, and kernel MPLS egress pop defects: split into `plan/release/fixes/spec-network-services-1-iface-dataplane-defects.md`.
- LDP and RSVP-TE one-off defects: split into `plan/release/fixes/spec-network-services-3-ldp-rsvpte-defects.md`.
- Per-owner isolation inside `firewall.ApplyAll` across unrelated registry owners: already designed in `plan/release/fixes/spec-vrrp-owner-arp-rfc-defects.md` (Q4 (b)); AC-3 here fixes the policy-route producer defect (`interface "*"` refused at verify and isolated per `policy/route[*]` entry at start).
- Already fixed at HEAD (retired in `plan/release/fixes/spec-network-services-one-off-defects.md`): firewall ARP literal addresses (`6da2c99b2c`), nftables NAT placement (`72d4616b2a`), and kernel `CONFIG_NF_FLOW_TABLE` (2026-10-27).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/forwarding.md`, `ai/doors/config-yang.md`, `ai/doors/le-tooling.md`
- [ ] `docs/architecture/core-design.md` (sections 14a, 14b; `## Design decisions` "Ze always starts")
- [ ] `docs/architecture/firewall/fw-6-firewall-vpp.md` ("Read-merge-write ACL bindings")
- [ ] `docs/architecture/policyroute/policy-routing.md`
- [ ] `ai/rules/user-facing-errors.md`, `ai/rules/testing.md`

## Current Behavior (MANDATORY)

All producers read at HEAD on 2026-11-26:
- [ ] `internal/plugins/firewall/vpp/backend_linux.go` - `bindAllACLs` lines 326-335 builds `acls` by concatenating `foreignInput`, `zeInput`, `foreignOutput`, `zeOutput` and casts `nInput := uint8(len(acls))` with no bounds check against 255 (`math.MaxUint8`) on either `len(foreignInput)+len(zeInput)` or total `len(acls)`, and `aclInterfaceSetACLList` does not reject `len(acls) > 255` before GoVPP narrows `Count` to `uint8`.
- [ ] `internal/component/firewall/protocol_test.go` - checks `firewall.ianaProtocolNumbers` against firewall YANG protocol leaf only; `internal/plugins/policyroute/yang/ze-policyroute-conf.yang` `policy/route/rule/from/protocol` is unchecked.
- [ ] `internal/plugins/policyroute/config.go` - `parseIfaceSpec("*")` returns `InterfaceSpec{Name: "", Wildcard: true}` with no error; `parsePolicyRoute` accepts it; later `lowerIfaceMatch` (`internal/plugins/firewall/nft/lower_linux.go`) rejects empty interface name during `firewall.ApplyAll`.
- [ ] `internal/component/firewall/validate.go` - `validateAction` lines 457-466 checks only `!chains[v.Target]` for `Jump` and `Goto`; it accepts self-jumps (`v.Target == ch.Name`), cycles across regular chains, and `Jump`/`Goto` targeting a base chain (`target.IsBase`).
- [ ] `test/firewall/firewall-*.ci` (12 files: `boot-apply`, `byte-rate-limit`, `coexistence`, `icmp-type`, `iface-wildcard`, `masquerade-flags`, `masquerade-ports`, `match-in-set-addr`, `match-in-set-port`, `nat-exclude`, `set-element-timeout`, `setdscp-inet`, `snat-addr-range`, `tcp-mss-clamp-to-pmtu` — check exact 12 from the journal row) place `expect=stderr:contains=firewall config applied` under the fixture `cmd=` instead of under the `ze` `cmd=`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| VPP ACL read-merge-write binding | `internal/plugins/firewall/vpp/backend_linux.go` `bindAllACLs`, `splitForeign`, `vppOps.aclInterfaceSetACLList` | Reads per-interface bindings, preserves foreign ACLs, and writes merged vector in one `ACLInterfaceSetACLList` call | Extends: AC-1 checks merged total `<= 255` before narrowing `nInput` or calling `aclInterfaceSetACLList`, and guards the direct adapter against `len(acls) > 255` |
| Firewall protocol YANG-to-Go agreement test | `internal/component/firewall/protocol_test.go` `TestParsedNamesMatchTheModel` | Verifies firewall YANG `protocol` enum matches `firewall.ianaProtocolNumbers` | Extends: AC-2 adds `policy/route/rule/from/protocol` from `ze-policyroute-conf.yang` to the same model agreement check |
| Firewall interface-name validation | `internal/component/firewall/validate.go` `validateInterfaceName` | Refuses empty interface name (including bare `"*"`) and names > 15 bytes with clear error | Extends: AC-3 applies the same non-empty and length check to `parseIfaceSpec` / `parsePolicyRoute` in `internal/plugins/policyroute/config.go` |
| Firewall chain-graph reachability walk | `internal/component/firewall/validate.go` `chainReaches`, `jumpTarget`, `validateAction` | Walks `Jump`/`Goto` edges with a `visited` set for `validateClampTCPMSSToPMTU` and `validateNftNATAction` | Extends: AC-4 reuses `jumpTarget` and `byName` chain map in `validateAction` / `validateTableJumps` to refuse self-jump, jump cycles (DFS/path tracking), and jump/goto to any base chain (`target.IsBase`) |
| Start-time service-block isolation ("Ze always starts") | `internal/component/config/service_blocks.go` `RegisterServiceBlock`, `excludeInvalidServiceBlocks`; `tree_validators.go` `RegisterScopedTreeValidator` | Drops an invalid registered service block at `ze start`, logging an ERROR under a doctor code and reporting it in `ze doctor`, while keeping `ze config validate` and reload strict | Extends: AC-3 and AC-4 register `config.RegisterServiceBlock("policy/route[*]", "policy route")` and `config.RegisterServiceBlock("firewall", "firewall")` with matching `ze doctor` codes so invalid policy routes or firewall jump cycles never abort `ze start` |

## Data Flow (MANDATORY)

### Entry Point
1. `ze config validate`, reload (`OnConfigVerify`), and `ze start` (`LoadConfigForStart`) for `firewall {}` and `policy { route ... }`.
2. VPP firewall reconcile (`firewall.ApplyAll` -> `vpp` `backend.Apply` -> `bindAllACLs`).
3. Functional test runner executing `test/firewall/firewall-*.ci`.

### Transformation Path
1. **VPP ACL 255-count guard (AC-1):** In `bindAllACLs`, before narrowing `nInput` to `uint8` and before calling `ops.aclInterfaceSetACLList`, check `len(foreignInput)+len(zeInput)+len(foreignOutput)+len(zeOutput) > maxVPPInterfaceACLs` (`255`). Return an operator-actionable error naming the interface index, foreign ACL count, desired Ze ACL count, the 255-ACL VPP API limit, and the fix, without modifying the interface's existing foreign bindings. Also enforce `len(acls) <= 255` and `int(nInput) <= len(acls)` inside the GoVPP adapter `aclInterfaceSetACLList` so direct adapter callers cannot wrap `Count` either.
2. **Policy-route protocol enum gate (AC-2):** Extend `TestParsedNamesMatchTheModel` (or add a companion test in `internal/plugins/policyroute/` importing `firewall.ProtocolNames()`—remembering `internal/component/firewall` must not import `internal/plugins/policyroute` per tier rules!) so `policy/route/rule/from/protocol` in `ze-policyroute-conf.yang` is verified equal to `firewall.ProtocolNames()`.
3. **Policy-route `interface "*"` refusal + boot isolation (AC-3):** Change `parseIfaceSpec` to return `(InterfaceSpec, error)`, refusing empty name (`""` or bare `"*"`) and names exceeding 15 bytes with an error naming the policy route, the invalid interface value, why (`"*"` strips to an empty interface name that matches no kernel interface), and the fix (name a prefix such as `eth*` or omit `interface` to match all interfaces). Register `config.RegisterServiceBlock("policy/route[*]", "policy route")` + scoped validator / `ze doctor` check (`doctor-policyroute-invalid-interface`) so `ze start` drops only the invalid policy-route entry while keeping all other policy routes and services running.
4. **Firewall jump/goto validation + boot isolation (AC-4):** In `internal/component/firewall/validate.go`, build `chainsByName map[string]*Chain` per table and validate every `Jump` and `Goto`: (a) refuse self-jump (`v.Target == ch.Name`), (b) refuse jump/goto to a base chain (`target.IsBase`), and (c) run a 3-color DFS over the table's jump/goto graph to refuse any cycle, printing the exact cycle chain path (`a -> b -> a`), why (the Linux kernel refuses self-jumps, base-chain jumps, and cycles with `EINVAL`/`EMLINK`), and the fix. Register `config.RegisterServiceBlock("firewall", "firewall")` + scoped validator / `ze doctor` check (`doctor-firewall-invalid-jump`) so `ze start` isolates a broken firewall block rather than exiting.
5. **12 `firewall-*.ci` expectation placement (AC-5):** Inspect each of the 12 `test/firewall/firewall-*.ci` files named in `plan/journal/test-against-broken-path.md` (2026-10-27) and move `expect=stderr:contains=firewall config applied` from the fixture `cmd=` block to the `ze` daemon `cmd=` block where the daemon actually logs that line.

### Boundaries Crossed
- Module tier boundary (`ai/doors/system.md`, `./le arch tier check`): `internal/component/firewall` MUST NOT import `internal/plugins/policyroute`. AC-2 places the policy-route YANG-to-`firewall.ProtocolNames()` agreement test inside `internal/plugins/policyroute/` (or via YANG schema lookup without importing the plugin package into the component).
- Process/VPP boundary: `bindAllACLs` protects foreign VPP ACL bindings before calling `ACLInterfaceSetACLList`.

### Changed-Wire Census

**Search evidence:** Inspected `internal/plugins/firewall/vpp/backend_linux.go`, `internal/component/firewall/validate.go`, `internal/component/firewall/protocol_test.go`, `internal/plugins/policyroute/config.go`, and `test/firewall/firewall-*.ci` at HEAD on 2026-11-26. No wire format changes: this spec adds pre-write bounds checking before existing GoVPP `ACLInterfaceSetACLList` marshaling, adds config-verify and YANG-agreement checks on existing schemas, and fixes `.ci` test expectation placement; zero network protocol, GoVPP binapi, nftables netlink, or plugin RPC wire formats are altered.

### Integration Points
- `internal/component/config/service_blocks.go` (`RegisterServiceBlock` for `policy/route[*]` and `firewall`)
- `internal/core/diagnostic` (`diagnostic.RegisterEventCode` for `doctor-policyroute-invalid-interface` and `doctor-firewall-invalid-jump`)

### Architectural Verification
- [ ] `./le arch tier check` passes (`internal/component/firewall` never imports `internal/plugins/policyroute`)
- [ ] Both new verify-time refusals (AC-3, AC-4) keep `ze start` starting cleanly via `config.RegisterServiceBlock` + `ze doctor` checks
- [ ] All 12 edited `test/firewall/firewall-*.ci` files pass in QEMU under the `ze` `cmd=` expectation (and fail if the `firewall` apply log is removed)

## Risks & Assumptions

### Assumptions
- `internal/plugins/firewall/vpp` is behind the `ze_vpp` build tag; unit tests for AC-1 run with the `ze_vpp` tag enabled via `./le test unit pkg path ./internal/plugins/firewall/vpp/`.

### Risks
- Moving `expect=stderr:contains=firewall config applied` under the `ze` `cmd=` in the 12 `test/firewall/firewall-*.ci` files may expose pre-existing timing or log-level issues in any of those 12 tests; each file must be verified individually in QEMU (and edited with `ze-test-judge` review if any expectation semantics shift, though moving a misplaced expect to the actual producer strengthens the test).

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `firewall.ApplyAll` on VPP backend with 255 foreign input ACLs + 1 desired Ze ACL | → | `bindAllACLs` and `aclInterfaceSetACLList` 255-ACL bound | `TestBindAllACLsRefusesOverflowAbove255` |
| `ze config validate` and `ze start` with `policy route p1 { interface "*"; }` | → | `parseIfaceSpec` refusal + `RegisterServiceBlock("policy/route[*]", ...)` | `TestPolicyRouteWildcardOnlyInterfaceRefusedAtVerifyAndIsolatedAtStart` |
| `ze config validate` and `ze start` with firewall self-jump, 2-chain cycle, or jump to base chain | → | `validateAction` / `validateTableJumps` + `RegisterServiceBlock("firewall", ...)` | `TestFirewallValidateRefusesInvalidJumpsAndIsolatesAtStart` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | VPP `bindAllACLs` 255-ACL wrap (old AC-1): 255 foreign ACLs plus 1 desired Ze ACL on a VPP interface, or a direct `aclInterfaceSetACLList` call with `len(acls) > 255` | `bindAllACLs` and `aclInterfaceSetACLList` refuse the oversized vector before `uint8` narrowing, returning an operator-actionable error naming the interface index, foreign ACL count, desired Ze ACL count, the 255-ACL VPP API limit, and the fix while leaving foreign ACL bindings untouched on the interface | Merged ACL vectors with total length `<= 255` (and `int(nInput) <= len(acls)`) remain accepted and applied in one `ACLInterfaceSetACLList` call |
| AC-2 | policy-route `from/protocol` enum (old AC-8): `TestPolicyRouteProtocolLeafMatchesFirewallVocabulary` runs in `internal/plugins/policyroute` | The unit test compares every enum word on `policy/route/rule/from/protocol` in `ze-policyroute-conf.yang` against `firewall.ProtocolNames()`, passes when both vocabularies match identically, and fails on any missing, extra, or misspelled protocol while keeping `internal/component/firewall` free of any import of `internal/plugins/policyroute` | None; every protocol enum entry in `ze-policyroute-conf.yang` must match `firewall.ProtocolNames()` 1-to-1 |
| AC-3 | policy-route `interface "*"` (old AC-14): a `policy route` configures bare `interface "*"` or an empty interface name, tested via `ze config validate`, reload, and `ze start` | `parseIfaceSpec` returns an error naming the policy route, offending value, why it matches no kernel interface, and the fix; `ze config validate` and reload return that refusal, while `ze start` isolates only the offending `policy/route[*]` entry via `config.RegisterServiceBlock`, brings up all other policy routes and firewall owners, logs an ERROR under `doctor-policyroute-invalid-interface`, and reports the same code in `ze doctor` | Prefix wildcards with a non-empty prefix up to 15 bytes (such as `eth*`) and rules omitting `interface` to match all interfaces remain permitted |
| AC-4 | firewall `validateAction` jumps (journal 2026-10-27): a firewall table configures a self-jump, a multi-chain `Jump`/`Goto` cycle, or a `Jump`/`Goto` targeting a base chain, tested via `ze config validate`, reload, and `ze start` | `validateAction` / `validateTableJumps` refuses each invalid target with an error naming the table, chain or cycle path (`a -> b -> a`), why the kernel rejects it, and the fix; `ze config validate` and reload return that refusal, while `ze start` isolates the invalid `firewall` block via `config.RegisterServiceBlock`, logs an ERROR under `doctor-firewall-invalid-jump`, and reports the same code in `ze doctor` | Acyclic `Jump` and `Goto` edges targeting non-base regular chains in the same table remain accepted |
| AC-5 | 12 `test/firewall/firewall-*.ci` expectations (journal 2026-10-27): each of the 12 `.ci` files listed in `plan/journal/test-against-broken-path.md` (2026-10-27) runs in QEMU | All 12 `.ci` files assert `expect=stderr:contains=firewall config applied` under the `ze` daemon `cmd=` block, pass in QEMU when the firewall applies, and fail when the daemon's firewall apply log is absent | Fixture `cmd=` blocks retain their own fixture-specific expectations unrelated to the `ze` daemon's `firewall config applied` log line |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| `TestBindAllACLsRefusesOverflowAbove255`, `TestACLInterfaceSetACLListRejectsOversizedVector` | `internal/plugins/firewall/vpp/backend_linux_test.go` | AC-1 |
| `TestPolicyRouteProtocolLeafMatchesFirewallVocabulary` | `internal/plugins/policyroute/config_test.go` | AC-2 |
| `TestPolicyRouteWildcardOnlyInterfaceRefusedAtVerifyAndIsolatedAtStart` | `internal/plugins/policyroute/config_test.go` | AC-3 |
| `TestFirewallValidateRefusesInvalidJumpsAndIsolatesAtStart` (self-jump, 2-chain and 3-chain cycle, jump to base chain, goto to base chain, valid DAG neighbour) | `internal/component/firewall/validate_test.go` | AC-4 |

### Functional Tests

| Test | AC |
|------|----|
| QEMU functional run of the 12 updated `test/firewall/firewall-*.ci` files | AC-5 |
| `test/firewall/` `.ci` verifying `ze config validate -` refuses self-jump, cycle, and jump to base chain | AC-4 |

## Files to Modify
- `internal/plugins/firewall/vpp/backend_linux.go`
- `internal/plugins/firewall/vpp/ops_linux.go` (or the adapter file implementing `aclInterfaceSetACLList`)
- `internal/plugins/policyroute/config.go`
- `internal/plugins/policyroute/register.go`
- `internal/plugins/policyroute/config_test.go`
- `internal/component/firewall/validate.go`
- `internal/component/firewall/register.go`
- `internal/component/firewall/validate_test.go`
- `test/firewall/firewall-boot-apply.ci`
- `test/firewall/firewall-byte-rate-limit.ci`
- `test/firewall/firewall-coexistence.ci`
- `test/firewall/firewall-icmp-type.ci`
- `test/firewall/firewall-iface-wildcard.ci`
- `test/firewall/firewall-masquerade-flags.ci`
- `test/firewall/firewall-masquerade-ports.ci`
- `test/firewall/firewall-match-in-set-addr.ci`
- `test/firewall/firewall-match-in-set-port.ci`
- `test/firewall/firewall-nat-exclude.ci`
- `test/firewall/firewall-set-element-timeout.ci`
- `test/firewall/firewall-setdscp-inet.ci`
- `docs/architecture/firewall/fw-6-firewall-vpp.md`
- `docs/architecture/policyroute/policy-routing.md`

## Files to Create
- None (all tests extend existing `*_test.go` files in the owning packages)

### Integration Checklist
- [ ] `./le arch tier check` passes
- [ ] `./le test unit pkg path` passes for `./internal/plugins/firewall/vpp/`, `./internal/component/firewall/`, and `./internal/plugins/policyroute/`
- [ ] All 12 `test/firewall/firewall-*.ci` files pass in QEMU

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/architecture/firewall/fw-6-firewall-vpp.md` (update "Read-merge-write ACL bindings" to document the 255-ACL refusal and remove the open-defect sentence)
- [ ] `docs/architecture/policyroute/policy-routing.md` (document non-empty interface validation and per-route start isolation)

## Implementation Steps
1. **AC-1 (VPP ACL 255 bound):** Add `maxVPPInterfaceACLs = 255` check in `bindAllACLs` and in the GoVPP `aclInterfaceSetACLList` adapter before narrowing to `uint8`; prove red-to-green with unit tests.
2. **AC-2 (policy-route protocol model test):** Add `TestPolicyRouteProtocolLeafMatchesFirewallVocabulary` in `internal/plugins/policyroute/config_test.go` comparing `ze-policyroute-conf.yang` `policy/route/rule/from/protocol` against `firewall.ProtocolNames()`.
3. **AC-3 (policy-route `interface "*"`):** Make `parseIfaceSpec` return an error on empty interface name (`"*"` or `""`) and >15 bytes; register `config.RegisterServiceBlock("policy/route[*]", "policy route")` and `doctor-policyroute-invalid-interface` check; prove validate refusal and start isolation.
4. **AC-4 (firewall jump validation):** Add self-jump, base-chain target, and cycle detection in `internal/component/firewall/validate.go`; register `config.RegisterServiceBlock("firewall", "firewall")` and `doctor-firewall-invalid-jump` check; prove validate refusal and start isolation.
5. **AC-5 (12 `firewall-*.ci`):** Move `expect=stderr:contains=firewall config applied` under the `ze` `cmd=` in all 12 files and verify in QEMU.
6. Update docs and close all 5 owned journal rows in `plan/journal/`.

## Review Gate
- [ ] `/ze-review` clean on all modified Go, `.ci`, and doc files
- [ ] `/ze-review-spec` confirms AC-1 through AC-5 and doc updates are met

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | the door every change reads | a config-driven engine others depend on is a component (`internal/component/`) | agrees | AC-2 places the policy-route YANG protocol agreement test in `internal/plugins/policyroute` so `internal/component/firewall` never imports an edge plugin |
| `ai/doors/forwarding.md` | Boundaries | the door covering `internal/plugins/firewall/vpp/*`, `internal/plugins/policyroute/*`, and `internal/component/firewall/*` | Exact or refuse, at verify. | agrees | AC-3 and AC-4 refuse invalid policy-route interfaces and invalid firewall jump targets at verify before any kernel netlink call |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `test/firewall/firewall-*.ci` | runs in the QEMU VM (`ai/rules/platform-linux.md`). | agrees | AC-5 fixes the 12 QEMU firewall `.ci` files so `firewall config applied` is asserted on the `ze` daemon command |
| `docs/architecture/firewall/fw-6-firewall-vpp.md` | Read-merge-write ACL bindings | the `// Design:` page of `internal/plugins/firewall/vpp/backend_linux.go`; anchored to `backend_linux.go` and `firewall/register.go`; a page this spec modifies | The API's one-octet count represents at most 255 ACL indexes. The merge currently lacks an oversized-vector refusal, so preservation above that limit is not established; the pre-existing narrowing defect is recorded in [`bound-wraps-before-it-refuses`](../../../plan/journal/bound-wraps-before-it-refuses.md). | agrees | AC-1 closes the exact oversized-vector refusal this section records as owed and updates this paragraph in the same commit |
| `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` | Rule 4: an attack-response table is cleared at the next start | anchored to `internal/plugins/firewall/vpp/backend_linux.go` | `bindAllACLs` removes stale and duplicate owned indexes from every live interface while preserving foreign input and output ACLs; `reconcileRemovals` then deletes only the detached objects. | agrees | AC-1 refuses an oversized merged ACL vector before calling `ACLInterfaceSetACLList`, preserving foreign ACL bindings and keeping `reconcileRemovals` behind a successful bind |
| `docs/architecture/policyroute/policy-routing.md` | One term for each named interface | the `// Design:` page of `internal/plugins/policyroute/config.go`; anchored to `policyroute/register.go`; a page this spec modifies | A rule installs one term for each interface its policy names. A policy that names no interface gets one term with no interface match. | agrees | AC-3 refuses bare `interface "*"` (directing operators to omit `interface` to match all interfaces) and updates this page in the same commit |
| `docs/architecture/policyroute/netlink-int-field-truncation.md` | The bound is `math.MaxInt`, not `math.MaxInt32` | anchored to `internal/plugins/policyroute/config.go` | This page covers uint32-to-int netlink table ID and metric bounds in `validateActionTable`, which this spec does not modify. | silent | AC-2 and AC-3 touch only the `from/protocol` model test and `parseIfaceSpec` interface validation; table-ID bounds are unchanged |
| `docs/architecture/firewall/firewall-irr.md` | A term states the type of the set another owner provides | anchored to `internal/component/firewall/validate.go` | This page covers IRR set match validation (`MatchInSet.ProvidedType`) in `validateMatch`, which this spec does not modify. | silent | AC-4 changes only `validateAction` and `ValidateTables` for jump/goto targets; `validateMatch` and IRR sets are unchanged |
| `docs/architecture/core-design.md` | Design decisions | the `// Design:` page of `internal/component/firewall/validate.go` and `internal/component/firewall/register.go`; anchored to `firewall/register.go` | Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. | agrees | AC-3 and AC-4 register `policy/route[*]` and `firewall` via `config.RegisterServiceBlock` + `ze doctor` checks so invalid blocks fail validate/reload but stay isolated at `ze start` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now. | agrees | Unchanged by this spec; none of AC-1 through AC-5 alter kernel-vs-VPP backend mismatch checks or `fib kernel` / `sysctl` rules |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each. | agrees | Superseded for daemon start by the 2026-11-26 row below; commit, reload, and `ze config validate` keep one shared verdict, which AC-3 and AC-4 follow for their new checks |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation. | agrees | Unchanged by this spec; AC-1 operates at VPP ACL binding merge time (`bindAllACLs`) and touches no `ze:backend` annotation or verifier scope |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it". | agrees | Unchanged by this spec; all firewall and policyroute changes keep `kernel` and `vpp` plane names untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it." | agrees | Orthogonal to firewall and policyroute; this spec touches no BGP export filter or session next-hop code |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs"). | agrees | Orthogonal to firewall and policyroute; this spec touches no BGP `policy { modify }` or `policychain.Sets` code |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)"). | agrees | Orthogonal to firewall and policyroute; this spec touches no BGP filter chain compilation |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self"). | agrees | Orthogonal to firewall and policyroute; this spec touches no BGP forwarding or `wireOverride` logic |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load"). | agrees | Orthogonal to firewall and policyroute; this spec touches no BGP next-hop attributes |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes"). | agrees | Orthogonal to firewall and policyroute; this spec touches no BGP route origination or forwarding |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?". | agrees | Orthogonal to firewall and policyroute; this spec touches no BGP implied-filter seam |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 on `docs/architecture/core-design.md` | Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it. | agrees | AC-3 and AC-4 register `policy/route[*]` and `firewall` via `config.RegisterServiceBlock` and register `doctor-policyroute-invalid-interface` / `doctor-firewall-invalid-jump` so bad blocks are isolated at `ze start` and refused at validate/reload |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 on `docs/architecture/core-design.md` | The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch. | agrees | AC-3 and AC-4 follow this exact split: bad `policy/route[*]` or `firewall` blocks stay off at `ze start` with a doctor finding, while commit, reload, and `ze config validate` refuse them |

## Design presentation

### What changes
- VPP firewall `bindAllACLs` and its GoVPP adapter refuse merged ACL vectors exceeding 255 before narrowing to `uint8`.
- Policy-route `from/protocol` YANG leaf gets a model-agreement test against `firewall.ProtocolNames()`.
- Policy-route `parseIfaceSpec` refuses bare `interface "*"` and empty names at verify, and registers `policy/route[*]` with `config.RegisterServiceBlock` + `doctor-policyroute-invalid-interface`.
- Firewall `validateAction` / `ValidateTables` refuses self-jumps, jump/goto cycles, and jumps/gotos to base chains at verify, and registers `firewall` with `config.RegisterServiceBlock` + `doctor-firewall-invalid-jump`.
- Twelve `test/firewall/firewall-*.ci` files move `expect=stderr:contains=firewall config applied` to the `ze` daemon `cmd=`.

### Before and after
- **Before:** 255 foreign VPP ACLs + 1 Ze ACL wraps `nInput` and `Count` to 0; `policy route` protocol leaf can drift from firewall's table undetected; `interface "*"` passes verify and breaks `firewall.ApplyAll` for every owner; firewall jump cycles/base-chain jumps pass `ze config validate` and fail at nftables netlink apply; 12 firewall `.ci` tests assert the daemon's apply log line against the fixture process.
- **After:** VPP refuses >255 ACLs cleanly without clobbering foreign ACLs; policy-route protocol leaf is model-tested; `interface "*"` and invalid firewall jumps are refused at validate/reload and isolated at `ze start` under registered doctor codes; all 12 firewall `.ci` files assert against the `ze` daemon.

### Alternatives set aside
- **Importing `internal/plugins/policyroute` from `internal/component/firewall/protocol_test.go`:** set aside because `ai/doors/system.md` and `./le arch tier check` forbid a component from importing a plugin; placing the test in `internal/plugins/policyroute/config_test.go` respects dependency direction.
- **Accepting `interface "*"` as equivalent to omitting `interface`:** set aside because silent translation hides operator ambiguity and conflicts with `firewall`'s own `validateInterfaceName` refusal of `"*"`.

### Documents departed from
None. `docs/architecture/firewall/fw-6-firewall-vpp.md` and `docs/architecture/policyroute/policy-routing.md` are updated in the same commit to reflect the completed bounds checks.

### Choices for Thomas
none

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `bound-wraps-before-it-refuses` | 2026-10-06 | `bindAllACLs` |
| `guard-added-to-one-half-of-a-pair` | 2026-09-14 | policy-route `from/protocol` |
| `one-members-bad-input-fails-every-member` | 2026-09-09 | firewall `ApplyAll` |
| `validate-accepts-what-start-refuses` | 2026-10-27 | firewall `validateAction` self-jump, jump cycle, jump to base chain |
| `test-against-broken-path` | 2026-10-27 | 12 `firewall-*.ci` assert `firewall config applied` under the fixture cmd |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (red recorded per AC)
- [ ] Tests PASS
- [ ] Scoped runs: `./le test unit pkg path ./internal/plugins/firewall/vpp/`, `./le test unit pkg path ./internal/component/firewall/`, `./le test unit pkg path ./internal/plugins/policyroute/`, `./le arch tier check`, `./le test qemu` on the 12 `test/firewall/firewall-*.ci` files
