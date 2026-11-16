# Spec: iface-mirror-target-reference

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Priority | P1 |
| Release reason | A mistyped or deleted port-mirror target commits, and a VLAN-unit target on a selected parent mirrors toward the wrong device: accepted config that the apply cannot honour is incorrect shipped behavior |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-16 |

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
| 1 | `mirror ingress`, `mirror egress` (AC-1 to AC-5) | Implemented: AC-1 and AC-2 in b2bb71058e, AC-3 and the mirrors installed after the per-entry loop in 38906b1d12. AC-4 and AC-5 unchanged behavior, with AC-4 extended to a VLAN-unit target. The two privileged `.ci` tests passed as root in Ze's QEMU guest on 2026-11-15 |
| 2 | Every other leaf that names an interface (AC-6 to AC-9) | Design below, decided 2026-11-15 (Option B): a leaf that configures an interface Ze owns becomes a reference, and a protocol or match leaf keeps naming kernel devices. Not implemented |

## Required Reading

### Architecture Docs
- [x] `docs/architecture/iface/logical-name-resolution.md` - "A mirror destination is resolved when the mirror spec is built" and "left undone with a warning and the commit still succeeds"
  → Constraint: verify stays pure (it reads no kernel), so it judges whether the CONFIG declares the target, never whether the device is present. A declared but unbound target still defers, as today
  → Constraint: "a name with NO selector is its own kernel device", and for a VRRP `track interface` "a name the interface tree does not carry must still resolve to the kernel device wearing it"
  → Constraint: "A consumer that can run without one (an OSPF-only, IS-IS-only or LDP-only config) calls `iface.EnsureBackend`": a config naming interfaces under a protocol with no `interface` tree is a supported mode today
  → Decision: the page's mirror paragraph changed in the same work (done)
  → Decision: the page states which interface names must be declared and which may name a kernel device (Option B, done)
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
- A unit target resolves through its parent's binding (implemented, AC-3: `mirrorDestination` looks the name up in `newMirrorTargets`' index of declared VLAN units, binds the parent through `deviceFor` and builds the device with `unitOSName`; an unbound parent defers the mirror with the existing warning)
- The mirrors are installed after the Phase 2 loop has created every VLAN (implemented with AC-3). Inside the loop, a mirror toward a VLAN of an entry the loop reached later was set up before that VLAN existed, the netlink backend's `SetupMirror` failed to find the destination, and the apply rolled back. `test/parse/iface-mirror-target-undeclared.ci`'s "later" config, which validate accepts, had this shape

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
| VRRP track completion | `internal/plugins/vrrp/register.go` `trackInterfaceValidator` (`RegisterSuggestion`) | Completion-only: offers kernel devices and refuses nothing, by design | does not apply: the leaf that most clearly names an interface Ze does not configure, and Option B keeps it naming kernel devices |

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
| SIGHUP reload deleting a live mirror target, then removing mirror and target together | → | the same check, then `removeStaleMirrors` on the netlink backend | `test/reload/tx-iface-mirror-target-delete-refused.ci` (PASS as root in Ze's QEMU guest, 2026-11-15) |
| Boot apply of a mirror toward a VLAN unit of a `mac match` entry | → | `mirrorDestination` through `newMirrorTargets`, after the Phase 2 VLAN loop | `test/plugin/iface-mirror-vlan-target-selector.ci` (PASS as root in Ze's QEMU guest, 2026-11-15) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConfiguredInterfaceReferenceNames` | `internal/component/iface/interface_reference_test.go` | the declared-name set: entries, veth peer, VLAN unit; nothing for a plain unit or the loopback | written |
| `TestConfiguredInterfaceReferenceCheck` | `internal/component/iface/interface_reference_test.go` | AC-1: offered names accepted, undeclared name and VLAN unit refused with the fix | written |
| `TestConfiguredInterfaceKindsAreTheSchemaLists` | `internal/component/iface/interface_reference_test.go` | the kind list equals the lists under the schema's `interface` container | written |
| `TestMirrorToAVLANUnitFollowsItsParentsSelector` | `internal/component/iface/config_mirror_test.go` | AC-3: the filter points at `<bound device>.<vlan-id>`, not at the decoy carrying the unit's logical name | red against the previous code, green with 38906b1d12 |
| `TestMirrorToAVLANUnitDefersWhenItsParentIsUnbound` | `internal/component/iface/config_mirror_test.go` | AC-4 for a VLAN-unit target: nothing installed, the apply succeeds | red against the previous code, green with 38906b1d12 |
| `TestMirrorTowardALaterEntrysVLANWaitsForTheVLAN` | `internal/component/iface/config_mirror_test.go` | the mirror installs after the VLAN it names exists, whatever the entry order (fake `mirrorNeedsDestination`) | red against the previous code, green with 38906b1d12 |
| `TestMirrorDestinationFollowsItsSelector`, `TestMirrorDefersWhenItsDestinationIsAbsent` | `internal/component/iface/config_mirror_test.go` | AC-5 and AC-4 for an entry target (existing) | existing |

## Part 2: every leaf that names an interface (design, Option B decided)

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

### The question, and the decision

What does "exists" mean for a name the `interface` tree does not declare? The
code gives two answers that conflict, and the check must read the config alone
(it runs offline in `ze config validate`, and a NIC may not have enumerated at
boot):

| Option | What a user sees | Cost |
|--------|------------------|------|
| A: "exists" means declared in the `interface` tree, for every leaf | `ospf { area 0 { interface eth1 } }` with no `interface ethernet eth1` is refused, naming `set interface ethernet eth1` | Departs from the documented OSPF-only, IS-IS-only and LDP-only mode and from VRRP `track`; converts about 190 `.ci` configs and the guides; needs the exemptions below |
| B: declared for the leaves that configure an interface Ze owns (mirror, bridge member, tunnel local, pppoe-client source, unnumbered, VRRP group parent, PPPoE server), while a protocol or match leaf keeps naming kernel devices | Part 1 behavior extends to the `ze-iface-conf` leaves; `ospf interface eth1` keeps working undeclared | Leaves a mistyped OSPF or firewall name committing, which the ruling forbids unless the owner reads "exists" as "declared where Ze configures it" |

**Decided 2026-11-15: Option B.** Thomas handed the question to the main
session ("Can you please resolve them?"). The design
(`docs/architecture/iface/logical-name-resolution.md`) deliberately lets a
protocol config name kernel devices with no `interface` tree: the OSPF-only,
IS-IS-only and LDP-only mode, and VRRP `track`. Option A would contradict that
page and convert about 190 `.ci` configs. Under Option B the leaves that
configure an interface Ze owns become `configured-interface` references (AC-6),
and every protocol and match leaf keeps naming kernel devices, so
`ospf { area 0 { interface eth1 } }` with no `interface` tree still commits.
The page states the rule in the same work.

Exemptions Option B keeps, each with its own test: the loopback (`lo`, whose
config name is `loopback`), a firewall wildcard (`prefix*`, matching runtime
L2TP and PPP sessions), devices Ze creates at runtime under no declared name
(L2TP and PPPoE server sessions, XFRM), and a declared dynamic name
(`pppoe-client`).

### Part 2 as built, from the leaf census (2026-11-16)

Thomas approved Part 2 on 2026-11-16 ("Start by the interface name work"). A
census of the six AC-6 leaves against their YANG, their consumers and every
repository user refined the design:

| Leaf | Decision | Why |
|------|----------|-----|
| bridge `member` (leaf-list) | `ze:reference "configured-interface"`. A VLAN-unit member binds through its parent's selector and is enslaved after the Phase 2 loop has created the VLANs (AC-10) | Phase 2a enslaved `<entry>.<vlan>` by its literal name before the VLAN existed, the mirror's AC-3 fault. The description's "the config or the OS already carries" is narrowed. No repository user named an undeclared port |
| tunnel `local interface` (v4 and v6) | `ze:reference "configured-interface"`. The name binds through the selector map before the tunnel is created (AC-11) | Phase 1 handed the logical name to `LinkByName`, so an uplink bound by `mac match` or `os-name` gave the wrong device or none. No repository user |
| pppoe-client `source-interface` | `ze:reference "configured-interface"` | 26 repository users named a fixture or container device with no `interface` entry; each is converted (AC-9) |
| PPPoE server `interface` (list key) | `ze:reference "configured-interface"` | 28 users, none declared; each is converted (AC-9). A VLAN the test fixture creates (`veth-bng.100`) is declared as an `ethernet` entry of that name, so Ze adopts the device rather than creating a second one |
| unit `ipv4 unnumbered` | Unchanged: the donor check already refuses an undeclared donor at verify (`resolveUnnumbered`, `pickDonorAddress`, `TestUnnumberedCommitRefusals`) | Its main donor is `loopback`, which the source does not offer, because `deviceFor` would hand a mirror the nonexistent device `loopback`. A reference would refuse it, or report a missing donor twice |
| VRRP group `interface` | Nothing to convert | No such leaf: a group is augmented under an entry's unit, so its interface is the enclosing, declared entry. `track interface` keeps naming kernel devices |

Declaring a device as a bare `ethernet <name> {}` claims only its admin state:
no unit means no address key, so the reconcile removes none of its addresses,
and an omitted `mtu` or offload leaf makes no kernel call
(`desiredState`, the `mtu` leaf description, `docs/architecture/iface/offload.md`).
The conversions therefore change nothing on the lab devices.

The second source `configured-interface-match` for wildcard leaves is dropped:
under Option B no reference leaf takes a wildcard, so it would have no user.
AC-8's wildcard and runtime-device exemptions are proven on the protocol and
match leaves Option B leaves free, which keep committing.

Two faults the census found are too big for this work and go to journal rows
(Owned journal rows): the shared resolver resolves a VLAN unit of a
selector-bound entry by its literal name (pppoe-client, PPPoE server, every
protocol consumer), owned here as AC-12; and a tunnel whose underlay is a VLAN
unit of the same config is created before the VLAN exists, owned by
`spec-iface-apply-on-device-appear.md`, whose worker applies an entry when a
device it needs appears.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `mirror ingress` names an interface the config does not declare | `ze config validate` and the commit refuse it, naming the YANG path, the unknown name and the fix | none |
| AC-2 | A commit deletes an interface that another unit's mirror names | Verify refuses the deletion, naming the mirroring interface and unit | none: the reference must go first |
| AC-3 | `mirror egress` names a VLAN unit of an ethernet entry bound by `mac match` to a veth | The tc mirred filter points at `<bound device>.<vlan-id>` | none |
| AC-4 | A declared target whose device is absent | The commit succeeds, the mirror is not installed, and a warning names it | the deferred mirror, by the page's rule for unbound settings |
| AC-5 | A target declared and present | The mirror is installed as today | the mirror |
| AC-6 | (Part 2, Option B) a leaf that configures an interface Ze owns (bridge `member`, tunnel `local interface`, pppoe-client `source-interface`, PPPoE server `interface`, and unit `ipv4 unnumbered` through its existing donor check) names an undeclared interface | Refused at validate, reload and commit, naming the path and the fix | none |
| AC-7 | (Part 2) a referenced interface is declared later in the file than its user | Accepted | none |
| AC-8 | (Part 2) each kept exemption: `ipv4 unnumbered loopback`, a firewall wildcard, a runtime-created device and an undeclared kernel device on a protocol leaf, a declared `pppoe-client` as a tunnel's local interface | Accepted, one test each | the exemption |
| AC-9 | (Part 2) every repository config, `.ci`, fixture and guide naming an interface Option B refuses | Converted in the same work, and each converted config passes `ze config validate` | none |
| AC-10 | (Part 2) a bridge `member` names a VLAN unit of an entry bound by `mac match`, on a fresh apply | The member enslaved is `<bound device>.<vlan-id>`, after the VLAN exists, and the apply succeeds | none |
| AC-11 | (Part 2) a tunnel `local interface` names an ethernet entry bound by `mac match` or `os-name` | The tunnel's parent is the bound device, not a device carrying the logical name; an unbound entry fails the apply with a message naming the tunnel and the entry, as an absent local device already did (no later phase can configure a tunnel that was not created) | none |
| AC-12 | (Part 3) a pppoe-client `source-interface`, PPPoE server `interface` or protocol leaf names a VLAN unit of a selector-bound entry | The shared resolver answers `<bound device>.<vlan-id>` | none |

### Tests that prove each AC

| AC | Test | Entry point |
|----|------|-------------|
| AC-1 | `test/parse/iface-mirror-target-undeclared.ci` (undeclared name, undeclared VLAN unit, declared-later positive with a veth peer and a VLAN unit) | `ze config validate` |
| AC-1, AC-2 | `test/reload/iface-mirror-target-undeclared-refused.ci` (a reload adding a mirror toward an undeclared name is refused) | config reload through the daemon |
| AC-1, AC-2 | `TestConfiguredInterfaceReferenceNames`, `TestConfiguredInterfaceReferenceCheck`, `TestConfiguredInterfaceKindsAreTheSchemaLists` in `internal/component/iface/interface_reference_test.go` | the reference source |
| AC-2 | `test/reload/tx-iface-mirror-target-delete-refused.ci`, `option=needs-linux:caps=net-admin` (PASS as root in Ze's QEMU guest on 2026-11-15, `./le test qemu ci test <path>`): a veth pair, a refused reload deleting the live target with the tc mirred filter still copying to it, then the reload removing mirror and target with no tc filter left on the source. Driver `reload/tx-iface-mirror-target-delete-refused` in `internal/test/fixture/iface_mirror_fixture_linux.go` | config reload through the daemon |
| AC-3 | `test/plugin/iface-mirror-vlan-target-selector.ci`, `option=needs-linux:caps=net-admin` (PASS as root in Ze's QEMU guest on 2026-11-15, `./le test qemu ci test <path>`): reads the egress mirred action of the source over netlink, against a decoy device carrying the unit's logical name; the source entry precedes the target, so it also proves the post-loop install. Unit: `TestMirrorToAVLANUnitFollowsItsParentsSelector` | config commit through the daemon |
| AC-4, AC-5 | `TestMirrorDefersWhenItsDestinationIsAbsent`, `TestMirrorToAVLANUnitDefersWhenItsParentIsUnbound`, `TestMirrorDestinationFollowsItsSelector` in `internal/component/iface/config_mirror_test.go`, plus AC-3's `.ci` | `mirrorSpecFor` |

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - the reference declarations and leaf descriptions (done)
- `internal/component/iface/config_mirror.go` - bind a unit target through its parent (`mirrorTargets`, `newMirrorTargets`, AC-3, done); comment on `mirrorDestination` (done)
- `internal/component/iface/config_apply.go` - the mirrors installed after the Phase 2 VLAN loop (done)
- `internal/component/iface/config_mirror_test.go`, `config_test.go` - the AC-3 and ordering tests, the fake's `mirrorNeedsDestination` (done)
- `docs/architecture/iface/logical-name-resolution.md` - the mirror destination paragraph, and the VLAN-unit and ordering paragraph (done)
- `docs/features/interfaces.md` - the second-interface paragraph (done)
- `ai/digests/iface.md` - Phase 2 mirror placement (done)
- `internal/test/fixture/misc_fixture_shellports.go` - the reload trigger (done)

## Files to Create
- `internal/component/iface/interface_reference.go`, `register_interface_reference.go`, `interface_reference_test.go` (done)
- `test/parse/iface-mirror-target-undeclared.ci`, `test/reload/iface-mirror-target-undeclared-refused.ci` (done)
- `test/reload/tx-iface-mirror-target-delete-refused.ci`, `test/plugin/iface-mirror-vlan-target-selector.ci` (done; both PASS as root in Ze's QEMU guest on 2026-11-15; no stress run recorded)
- `internal/test/fixture/register_iface_mirror_linux.go`, `iface_mirror_fixture_linux.go` (done)

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config syntax changed? | No | the grammar is unchanged; the accepted values narrow, as the leaf descriptions say |
| 12 | Internal architecture changed? | Yes | `docs/architecture/iface/logical-name-resolution.md` (done) |

## Implementation Steps
1. **Phase: Wiring** -- the failing `.ci` tests (done for AC-1, AC-2: b2bb71058e)
2. **Phase: Verify check** -- AC-1, AC-2 (done: b2bb71058e)
3. **Phase: Unit target binding** -- AC-3 (done: 38906b1d12; both privileged `.ci` tests pass in Ze's QEMU guest)
4. **Phase: Part 2** -- Option B, decided 2026-11-15: the references AC-6 names, the exemptions (AC-8) and the conversions (AC-9)

## Implementation Summary

### What Was Implemented (2026-11-15, committed as b2bb71058e)
- The `configured-interface` reference source in iface: it offers every declared entry, veth peer and VLAN unit, and its `Check` refuses any other name with the set command that declares it and at most eight declared choices
- `ze:reference "configured-interface"` on `mirror ingress` and `mirror egress`
- No dead path removed: the "mirror deferred" warning serves a declared ethernet whose selector has no device, which the refusal cannot see
- Repository users converted: `test/parse/iface-vpp-accepts-mirror.ci` and `TestBackendGateVppMirror` mirrored toward an undeclared `eth1` and `xe1`, now declared. `internal/le/test/deployment/vppifacescenarios.go` already declares its target

### What Was Implemented (2026-11-15, second pass, committed as 38906b1d12)
- AC-3: a VLAN-unit target binds through its parent entry. `newMirrorTargets` indexes every declared VLAN unit by its `<entry>.<vlan-id>` name (`unitOSName` of the logical name, the text the reference source offers); `mirrorDestination` binds the parent with `deviceFor` and returns `unitOSName` of the parent's device, or empty (deferred, with the existing warning) when the parent is unbound. An ethernet entry named like a unit stays an entry
- Found while proving AC-3, fixed in this work: the mirrors were installed inside the Phase 2 loop, so a mirror toward a VLAN of a later entry failed at `SetupMirror` and rolled the apply back. `applyConfigPass` now installs them after the loop
- The AC-2 privileged reload test, and the AC-3 privileged boot test, with their netlink drivers. Both ran as root in Ze's QEMU guest on 2026-11-15 (`./le test qemu ci test <path>`) and passed. No ze-test stress run is recorded
- Evidence: the parse and reload `.ci` tests of b2bb71058e are red with the two `ze:reference` lines removed and green with them; the three unit tests of 38906b1d12 are red against the previous code and green with it

### What Was Implemented (2026-11-16, Part 2)
- AC-6: `ze:reference "configured-interface"` on bridge `member`, both tunnel `local interface` leaves, pppoe-client `source-interface` (ze-iface-conf) and the PPPoE server `interface` key (ze-pppoe-conf), with their descriptions. The unnumbered donor keeps its own verify check (`TestUnnumberedCommitRefusals`); VRRP has no group interface leaf
- One binder for every reference leaf: `interfaceTargets` (`interface_targets.go`) replaces the mirror-only `mirrorTargets`, so a mirror target, a bridge member and a tunnel local interface bind the same way
- AC-10: `enslaveBridgeMembers` (`config_bridge_members.go`) enslaves entry members in Phase 2a and VLAN-unit members after the Phase 2 loop, each through `interfaceTargets`
- AC-11: `tunnelSpecOnDevices` binds a tunnel's local interface before every create (Phase 1, the delete-then-create undo, the prune undo); an unbound one fails the apply naming the tunnel and the entry
- AC-9: 54 repository configs converted by declaring the device as a bare `ethernet` entry (pppoe `.ci` suites, static and plugin `.ci`, interop-pppoe and interop-topology configs, two iface PPPoE integration tests, the PPPoE backend gate test, the QEMU accel-ppp lab config whose test now also asserts the declaration), plus `docs/guide/pppoe.md` and `docs/guide/configuration.md`
- Found and fixed in passing: the PPPoE server `interface` description said an absent device is logged and skipped, but start and reload both fail on it (`prepareAccessConfiguration`)
- Evidence: `test/parse/iface-reference-leaves-undeclared.ci` passes, and goes red at its own case with each leaf's reference removed alone (member seq 1, tunnel seq 2, client seq 3, server seq 4). The four AC-10 and AC-11 unit tests pass and are red against the previous `config_apply.go` and `config_mirror.go` (decoy `wan.100` enslaved; missing VLAN port; tunnel on decoy `wan`; unbound tunnel built on the decoy). `TestRepositoryConfigsValidate` validated 3337 configs and refused only `test/web/fixtures/cli-load.conf`, for two missing BGP mandatory leaves this work does not touch

## Checklist

### Goal Gates (MUST pass)
- [ ] `./le test bgp parse` and `./le test bgp reload` for the new `.ci` files; the iface unit package through `./le test unit pkg path` (Part 1: the parse and reload `.ci` files red without the `ze:reference` lines and green with them, b2bb71058e; the iface package passes, 38906b1d12)
- [ ] The two privileged `.ci` files (`tx-iface-mirror-target-delete-refused`, `iface-mirror-vlan-target-selector`) run on Linux with CAP_NET_ADMIN, and each passes the ze-test stress run (Part 1: both PASS as root in Ze's QEMU guest on 2026-11-15; no stress run recorded)

### TDD
- [x] Tests written
- [ ] Tests FAIL (paste output) (Part 1: red before the fix, recorded in b2bb71058e and 38906b1d12)
- [ ] Tests PASS (paste output) (Part 1: green in the same commits, and the two QEMU runs above)

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `validate-accepts-what-start-refuses.md` | 2026-11-08 | mirror target is an unchecked free string (fixed: b2bb71058e, 38906b1d12) |
| `validate-accepts-what-start-refuses.md` | 2026-11-16 | the shared resolver resolves a VLAN unit of a selector-bound entry by its literal name (AC-12) |
