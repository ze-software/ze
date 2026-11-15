# Spec: iface-mirror-target-reference

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | A mistyped or deleted port-mirror target commits, and a VLAN-unit target on a selected parent mirrors toward the wrong device: accepted config that the apply cannot honour is incorrect shipped behavior |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`interface ... unit ... mirror { ingress <name>; egress <name>; }` took any
string. Verify never checked that the name is an interface the config declares,
so three faults committed:

| Fault | What happened |
|-------|---------------|
| The target is mistyped | The commit passed verify. `deviceFor` returns a name absent from the selector map unchanged, so tc was pointed at the literal name and the apply failed or mirrored toward a stranger |
| The target interface is deleted while a mirror still names it | The commit passed, and the mirror points at nothing |
| The target is a VLAN unit of an entry bound by `mac match` or `os-name` | `deviceFor` does not find the unit name in the selector map and returns it unchanged, so tc mirrors toward whatever device carries the logical name, or toward no device |

The goal: a mirror target is a reference. Verify refuses a target the
`interface` tree does not declare, refuses the deletion of a referenced target
(naming the interface that mirrors to it), and the apply binds a unit target
through its parent's selector.

Found by comparing recent VyOS fixes against Ze. VyOS fixed the same class in
T9491 (280cb4837, "refuse deleting an interface used as mirror or redirect
target") and T6393 (99271db4a "accept mirror and redirect targets defined on the
CLI", 9854913e2 "install mirror and redirect filters for VLAN targets"), in
`~/Code/github.com/vyos/vyos-1x`.

### Owner ruling, 2026-11-15, and the widened scope

Thomas: a config that references an interface that does not exist must be
refused at commit ("Yes, correct behaviour, refuse commit"). VyOS shipped the
same refusal (T7886, deleting an interface other config references is blocked;
vyos-1x #5597 for a mirror target), and fixed the opposite trap, a reference to
an object created later in commit order failing at boot (#5517).

The scope is widened to two parts:

| Part | Leaves | State |
|------|--------|-------|
| 1 | `mirror ingress`, `mirror egress` (AC-1 to AC-5) | AC-1 and AC-2 implemented, awaiting the owed runs. AC-3 open. AC-4 and AC-5 unchanged behavior |
| 2 | Every other leaf that names an interface (AC-6 to AC-9) | Design below. Blocked on one owner question: what "exists" means for an interface the `interface` tree does not declare |

## Required Reading

### Architecture Docs
- [x] `docs/architecture/iface/logical-name-resolution.md` - "A mirror destination is resolved when the mirror spec is built" and "left undone with a warning and the commit still succeeds"
  → Constraint: verify stays pure (it reads no kernel), so it judges whether the CONFIG declares the target, never whether the device is present. A declared but unbound target still defers, as today
  → Constraint: "a name with NO selector is its own kernel device", and for a VRRP `track interface` "a name the interface tree does not carry must still resolve to the kernel device wearing it"
  → Constraint: "A consumer that can run without one (an OSPF-only, IS-IS-only or LDP-only config) calls `iface.EnsureBackend`": a config naming interfaces under a protocol with no `interface` tree is a supported mode today
  → Decision: the page's mirror paragraph changed in the same work (done)
- [x] `docs/architecture/config/yang-config-design.md` - the `ze:reference` row, "Layer 1" (the whole-tree instance check), "Where a validator actually runs"
  → Constraint: `Validator.ValidateReferences` runs over the whole tree inside `ValidateCustomSections`, which `ze config validate`, the daemon's load and reload, and the editor's commit share, so one declaration gives one verdict on every path, order-independent

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/component/iface/yang/ze-iface-conf.yang` `container mirror` - `ingress` and `egress` were `type string` with no validator; they now carry `ze:reference "configured-interface"`
- [x] `internal/component/iface/config.go` unit parsing - copies `mirror/ingress` and `mirror/egress` into `MirrorIngress` and `MirrorEgress` unchecked
- [x] `internal/component/iface/register.go` `parseAndVerifyIfaceSections` - runs selector, delegated-prefix and VPP QoS checks, and no mirror check
- [x] `internal/component/iface/config_mirror.go` `applyMirror`, `mirrorDestination` - the "mirror deferred" warning fires only when `deviceFor` answers unbound, which is a DECLARED ethernet whose selector finds no device. An undeclared name was never deferred: `deviceFor` returns it unchanged as its own kernel device
- [x] `internal/component/iface/config_apply.go` `deviceFor`, `unitOSName` - a name absent from the selector map is its own device. A VLAN unit's kernel name is `<parent device>.<vlan-id>`

**Behavior to preserve:**
- A declared target whose device is absent defers with a warning, and the commit succeeds (the page's rule for every unbound setting). The refusal does not make this path unreachable, because verify cannot see the kernel, so it stays
- A target bound by `mac match` or `os-name` resolves through the selector map

**Behavior to change:**
- An undeclared target is refused at verify, with the YANG path and the fix (done)
- Deleting a referenced target is refused at verify, naming the referencing interface and unit (done: the same whole-tree check, and the refused path names both)
- A unit target resolves through its parent's binding (open, AC-3)

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `ze:reference` with a registered source | `internal/component/config/yang/reference_source.go` `RegisterReferenceSource`, `ReferenceAnswer` | A component answers the names a leaf may hold, and an optional `Check` | extends: iface registers `configured-interface` (`internal/component/iface/interface_reference.go`, `register_interface_reference.go`). Part 2 adds the same declaration to each leaf in its own module's YANG, so no central list names a feature |
| Whole-tree reference check | `internal/component/config/yang/validator_instance.go` `ValidateReferences`, `checkMapKey` | Judges every reference leaf, list keys included, over the whole tree at validate, boot, reload and commit | extends, unchanged: a list keyed by an interface name (OSPF, IS-IS) is reached through `checkMapKey` |
| iface verify | `internal/component/iface/register.go` `parseAndVerifyIfaceSections` | Pure checks over the parsed config | does not apply: the reference walk also gives editor completion and reaches leaves of other modules, which an iface verifier cannot see |
| Unnumbered donor check | `internal/component/iface/unnumbered.go` `resolveUnnumbered`, `pickDonorAddress` | Refuses a donor interface the config does not declare, inside the iface parse | Part 2: a candidate to move onto the `configured-interface` source, so it gains completion; its loopback spelling (`loopback`) differs from the device name (`lo`) |
| Selector map | `internal/component/iface/config_apply.go` `deviceFor`, `bindDevices` | Translates an ethernet logical name to its kernel device | extends (AC-3): a unit target is split into parent and VLAN id, the parent is translated, and `unitOSName` builds the device |
| Dynamic kinds | `internal/component/iface/declared_dynamic.go` `DeclaredDynamicNames` | Names the declared kinds whose device exists only part of the time (`pppoe-client`) | Part 2: a declared dynamic name stays a declared name, and the source accepts it |
| Interface usage map (VPP) | `plan/release/fixes/spec-vpp-interface-in-use.md`, AC-6 and AC-9 | Designed, not built: VPP mirror destinations as references that block deletion | extends: the reference check is backend-neutral, so that spec's mirror deletion rows are met by this one, and its role-conflict checks stay its own |
| Absent mirror destination retry | `plan/release/fixes/spec-network-services-one-off-defects.md`, AC-22 | Designed: an absent destination is reported and retried, not a whole-commit abort | does not apply: that spec owns a declared target whose device is absent. This spec owns an undeclared one |
| VRRP track completion | `internal/plugins/vrrp/register.go` `trackInterfaceValidator` (`RegisterSuggestion`) | Completion-only: offers kernel devices and refuses nothing, by design | Part 2: the leaf that most clearly names an interface Ze does not configure; the owner question decides it |

## Data Flow (MANDATORY)

### Entry Point
- A config commit, a reload, a boot or `ze config validate` carrying a leaf declared `ze:reference "configured-interface"`

### Transformation Path
1. `ValidateCustomSections` calls `Validator.ValidateReferences` over `Tree.ToPluginMap()` of the whole candidate
2. The walk meets a reference leaf and asks the `configured-interface` source once per parent node
3. The source reads the candidate's `interface` section: every entry of every interface kind, every veth peer, every VLAN unit as `<entry>.<vlan-id>`
4. The `Check` accepts a declared name, and otherwise refuses with the name, why, the `set` command that declares it and the declared choices
5. A refusal fails validate, boot, reload and commit with the leaf's YANG path, and the apply never sees the name
6. At apply, `mirrorDestination` binds the accepted name through the selector map (AC-3 adds the unit case)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config to iface reference source | `RegisterReferenceSource` from iface `init()`; iface is always linked | `TestConfiguredInterfaceReferenceNames`, `test/parse/iface-mirror-target-undeclared.ci` |
| daemon reload to validation | a SIGHUP reload runs the same `ValidateCustomSections` before any plugin is asked | `test/reload/iface-mirror-target-undeclared-refused.ci` |

### Integration Points
- `configuredInterfaceReference` (`interface_reference.go`), registered in `register_interface_reference.go`
- `Validator.ValidateReferences`, unchanged
- `mirrorDestination`, `deviceFor`, `unitOSName` (AC-3)

### Changed-Wire Census

**Search evidence:** searched `mirror {`, `mirror/ingress`, `mirror/egress` and `"mirror":` across the repository outside `vendor/`, `tmp/` and `.claude/worktrees/`; known users: `test/parse/iface-vpp-accepts-mirror.ci`, `internal/component/iface/backend_gate_vpp_test.go`, `internal/le/test/deployment/vppifacescenarios.go`.

No wire changes. The config grammar is unchanged: the same leaves take the same strings, and the change narrows which values a commit accepts. No protocol message, plugin protocol field or event changes. The repository users that named an undeclared target were converted (Implementation Summary).

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` with an undeclared mirror target | → | `configuredInterfaceReference` through `ValidateReferences` | `test/parse/iface-mirror-target-undeclared.ci` |
| SIGHUP reload adding a mirror toward an undeclared target | → | the same check on the daemon's reload | `test/reload/iface-mirror-target-undeclared-refused.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConfiguredInterfaceReferenceNames` | `internal/component/iface/interface_reference_test.go` | the declared-name set: entries, veth peer, VLAN unit; nothing for a plain unit or the loopback | written |
| `TestConfiguredInterfaceReferenceCheck` | `internal/component/iface/interface_reference_test.go` | AC-1: offered names accepted, undeclared name and VLAN unit refused with the fix | written |
| `TestConfiguredInterfaceKindsAreTheSchemaLists` | `internal/component/iface/interface_reference_test.go` | the kind list equals the lists under the schema's `interface` container | written |
| `TestMirrorDestination` | `internal/component/iface/config_mirror_test.go` | AC-3 to AC-5 | |

## Part 2: every leaf that names an interface (design, awaiting the owner)

### The leaves (from the YANG, 2026-11-15)

| Module | Leaves | What the name is today |
|--------|--------|------------------------|
| `ze-iface-conf` | bridge `member`, tunnel `local interface`, pppoe-client `source-interface`, unit `ipv4 unnumbered` | An interface Ze configures; unnumbered is already refused when undeclared |
| `ze-ospf-conf`, `ze-isis-conf`, `ze-ldp-conf`, `ze-rsvp-te-conf` | `interface` lists (keys), LDP `interfaces` | A kernel device; 54 of 55 OSPF, 20 of 21 IS-IS and 13 of 13 LDP `.ci` configs carry no `interface` tree |
| `ze-bfd-conf`, `ze-bgp-conf`, `ze-healthcheck-conf` | single-hop `interface`, BGP connection `interface`, healthcheck `interface` | A kernel device, resolved through `ResolveDevice` |
| `ze-dhcp-server-conf`, `ze-tftp-server-conf`, `ze-image-server-conf` | `listen-interface` | A kernel device; 24 of 30 DHCP server `.ci` configs carry no `interface` tree |
| `ze-firewall-conf`, `ze-firewall-irr`, `ze-policyroute-conf`, `ze-policy-conf` | `input-interface`, `output-interface` (a trailing `*` is a prefix match), IRR `interface`, policy-route and policy `interface` | A match on a kernel name, including devices created at runtime (`l2tp*`, `ppp*`); 78 of 81 firewall `.ci` configs carry no `interface` tree |
| `ze-vrrp-conf` | group `interface`, `track interface` | The group parent is configured; `track` deliberately accepts kernel-only devices (logical-name-resolution.md) |
| `ze-static-conf`, `ze-pppoe-conf`, `ze-ipsec-conf`, `ze-traffic-control-conf`, `ze-traffic-usage-conf`, `ze-flowexport-conf`, `ze-vpp-conf`, `ze-system-conf` | next-hop `interface`, PPPoE server `interface`, IPsec `interface`, traffic-control and usage `interface` lists, sampling `interface`, VPP `interface`, system `interface` and `device` leaves | Mixed: configured interfaces, kernel devices and runtime devices |

### The owner question

What does "exists" mean for a name the `interface` tree does not declare? The
code gives two answers that conflict, and the check must read the config alone
(it runs offline in `ze config validate`, and a NIC may not have enumerated at
boot):

| Option | What a user sees | Cost |
|--------|------------------|------|
| A: "exists" means declared in the `interface` tree, for every leaf | `ospf { area 0 { interface eth1 } }` with no `interface ethernet eth1` is refused, naming `set interface ethernet eth1` | Departs from the documented OSPF-only, IS-IS-only and LDP-only mode and from VRRP `track`; converts about 190 `.ci` configs and the guides; needs the exemptions below |
| B: declared for the leaves that configure an interface Ze owns (mirror, bridge member, tunnel local, pppoe-client source, unnumbered, VRRP group parent, PPPoE server), while a protocol or match leaf keeps naming kernel devices | Part 1 behavior extends to the `ze-iface-conf` leaves; `ospf interface eth1` keeps working undeclared | Leaves a mistyped OSPF or firewall name committing, which the ruling forbids unless the owner reads "exists" as "declared where Ze configures it" |

Exemptions either option needs, each with its own test: the loopback (`lo`, whose
config name is `loopback`), a firewall wildcard (`prefix*`, matching runtime
L2TP and PPP sessions), devices Ze creates at runtime under no declared name
(L2TP and PPPoE server sessions, XFRM), and a declared dynamic name
(`pppoe-client`). A wildcard must not be accepted by a leaf that installs toward
one device (mirror), so a wildcard-accepting leaf takes a second source
(`configured-interface-match`, also registered by iface), never a flag on the
first.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `mirror ingress` names an interface the config does not declare | `ze config validate` and the commit refuse it, naming the YANG path, the unknown name and the fix | none |
| AC-2 | A commit deletes an interface that another unit's mirror names | Verify refuses the deletion, naming the mirroring interface and unit | none: the reference must go first |
| AC-3 | `mirror egress` names a VLAN unit of an ethernet entry bound by `mac match` to a veth | The tc mirred filter points at `<bound device>.<vlan-id>` | none |
| AC-4 | A declared target whose device is absent | The commit succeeds, the mirror is not installed, and a warning names it | the deferred mirror, by the page's rule for unbound settings |
| AC-5 | A target declared and present | The mirror is installed as today | the mirror |
| AC-6 | (Part 2, after the owner answers) each leaf the chosen option covers names an undeclared interface | Refused at validate, reload and commit, naming the path and the fix | none |
| AC-7 | (Part 2) a referenced interface is declared later in the file than its user | Accepted | none |
| AC-8 | (Part 2) each kept exemption: `lo`, a firewall wildcard, a runtime-created device, a declared `pppoe-client` | Accepted, one test each | the exemption |
| AC-9 | (Part 2) every repository config, `.ci`, fixture and guide naming an interface the chosen option refuses | Converted in the same work | none |

### Tests that prove each AC

| AC | Test | Entry point |
|----|------|-------------|
| AC-1 | `test/parse/iface-mirror-target-undeclared.ci` (undeclared name, undeclared VLAN unit, declared-later positive with a veth peer and a VLAN unit) | `ze config validate` |
| AC-1, AC-2 | `test/reload/iface-mirror-target-undeclared-refused.ci` (a reload adding a mirror toward an undeclared name is refused) | config reload through the daemon |
| AC-1, AC-2 | `TestConfiguredInterfaceReferenceNames`, `TestConfiguredInterfaceReferenceCheck`, `TestConfiguredInterfaceKindsAreTheSchemaLists` in `internal/component/iface/interface_reference_test.go` | the reference source |
| AC-2 | the deletion of a live target, owed as `test/reload/tx-iface-mirror-target-delete-refused.ci` with `option=needs-linux:caps=net-admin`, because the boot config must create the target first | config reload through the daemon |
| AC-3 | new `test/plugin/iface-mirror-vlan-target-selector.ci`, `option=needs-linux`, reads the tc filter on the source device | config commit through the daemon |
| AC-4, AC-5 | `TestMirrorDestination` table rows in `internal/component/iface/config_mirror_test.go`, plus AC-3's `.ci` | `mirrorSpecFor` |

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - the reference declarations and leaf descriptions (done)
- `internal/component/iface/config_mirror.go` - bind a unit target through its parent (AC-3); comment on `mirrorDestination` (done)
- `docs/architecture/iface/logical-name-resolution.md` - the mirror destination paragraph (done)
- `docs/features/interfaces.md` - the second-interface paragraph (done)
- `internal/test/fixture/misc_fixture_shellports.go` - the reload trigger (done)

## Files to Create
- `internal/component/iface/interface_reference.go`, `register_interface_reference.go`, `interface_reference_test.go` (done)
- `test/parse/iface-mirror-target-undeclared.ci`, `test/reload/iface-mirror-target-undeclared-refused.ci` (done)

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config syntax changed? | No | the grammar is unchanged; the accepted values narrow, as the leaf descriptions say |
| 12 | Internal architecture changed? | Yes | `docs/architecture/iface/logical-name-resolution.md` (done) |

## Implementation Steps
1. **Phase: Wiring** -- the failing `.ci` tests (done for AC-1, AC-2)
2. **Phase: Verify check** -- AC-1, AC-2 (done)
3. **Phase: Unit target binding** -- AC-3
4. **Phase: Part 2** -- after the owner answers the question above

## Implementation Summary

### What Was Implemented (2026-11-15, uncommitted, awaiting the owed runs)
- The `configured-interface` reference source in iface: it offers every declared entry, veth peer and VLAN unit, and its `Check` refuses any other name with the set command that declares it and at most eight declared choices
- `ze:reference "configured-interface"` on `mirror ingress` and `mirror egress`
- No dead path removed: the "mirror deferred" warning serves a declared ethernet whose selector has no device, which the refusal cannot see
- Repository users converted: `test/parse/iface-vpp-accepts-mirror.ci` and `TestBackendGateVppMirror` mirrored toward an undeclared `eth1` and `xe1`, now declared. `internal/le/test/deployment/vppifacescenarios.go` already declares its target

## Checklist

### Goal Gates (MUST pass)
- [ ] `./le test bgp parse` and `./le test bgp reload` for the new `.ci` files; the iface unit package through `./le test unit pkg path`

### TDD
- [x] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `validate-accepts-what-start-refuses.md` | 2026-11-08 | mirror target is an unchecked free string |
