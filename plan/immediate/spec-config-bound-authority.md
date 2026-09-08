# Spec: config-bound-authority

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Bucket: `plan/immediate/`, because an operator on the first release meets two of
these as bugs. A ring size the driver refuses passes `ze config validate` and
fails at apply with no number in the error, and a DPDK queue count is accepted
with no bound at all.

## Task

A numeric config leaf carries a bound. Somebody decided that bound, and the
decision has an owner. When the owner is a published standard the bound is safe
to write down, because a twelve-bit restart time is 0 to 4095 forever. When the
owner is a driver, a kernel or VPP, the written bound is a COPY of a fact that
machine answers for itself, and the copy is wrong on the next machine.

VyOS shipped `vyos/vyos-1x#5448` on this defect. Its CLI refused a VPP ring
descriptor count of 32768 because two validators held `256-16384` for RX and
`256-8192` for TX, while VPP itself reported `desc 16384 (min 0 max 32768 align
1)` for the Mellanox ConnectX card in the machine. Configuring hardware the
daemon could already drive needed a code change and a release.

Ze holds the same shape today, in the other polarity. `system tuning ethtool
<if> ring rx` and `ring tx` declare `range "1..65535"`, and `setEthtoolRing`
reads the driver's real maxima into `rp.rxMaxPending` and `rp.txMaxPending` and
then ignores both, writing the operator's number straight into
`ETHTOOL_SRINGPARAM`. So `ze config validate` accepts a ring the driver refuses,
and the operator meets `ETHTOOL_SRINGPARAM failed for eth0`, an error carrying
no number and no way to learn what the driver would have taken. The VPP DPDK
`rx-queues` and `tx-queues` leaves declare no bound at all, and their own
`ze:help` states that Ze does not compare the count against the queues the NIC
holds.

The goal is that Ze cannot hold this class. Every numeric leaf DECLARES who owns
its bound. A leaf whose owner is the host is refused at `ze config validate`
against the number the host gives, never against a number a developer typed. A
gate refuses a leaf that declares nothing, and refuses a host-owned leaf whose
static ceiling sits below what its type can carry, because that ceiling is the
copy.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/doctor-and-health-checks.md` - the tier that already turns a host fact into a refusal
  → Decision: a capability enrolment has three states, and the third exists so an unreadable probe warns rather than refuses. A host bound Ze cannot read MUST NOT refuse a commit.
  → Constraint: `kernelcap.Refuse` is called from exactly three places, `runValidate` in `internal/component/config/cli/cmd_validate.go`, `run` in `cmd/ze/hub/main.go`, and the reload path in `cmd/ze/hub/main_reload.go`. A new host verdict reaches the operator through rails that already exist, not through a fourth call site.
- [ ] `docs/architecture/config/yang-config-design.md` - the YANG system, extensions, and the custom validator seam
  → Decision: `ze:validate` already exists for "runtime-determined valid sets", with `CustomValidator.ValidateFn(path string, value any) error`. The path carries the list key, so a per-interface leaf recovers its subject from the path. This spec adds NO runtime registry.
  → Constraint: `CheckAllValidatorsRegistered` runs at schema build, called by `BuildSchema` in `internal/component/config/yang_schema.go`, so a `ze:validate` name with no registration is already refused at startup. The new gate leans on that rather than resolving Go symbols itself.
- [ ] `docs/architecture/host/tuning.md` - the ring write this spec changes
  → Constraint: the page states "Tuning errors are non-fatal. They are reported, and they do not block a config commit", and "A ring-size write can fail on a virtual NIC or on a driver with no ring parameter support. Those failures are expected, reported, and not retried." Both sentences describe behavior this spec keeps at APPLY time and supplements at VALIDATE time. The page is edited in the same work.
- [ ] `docs/guide/vpp.md` - the DPDK interface settings an operator writes
  → Constraint: the queue-count leaves are documented as pass-through. The page states the new refusal and the unknown case.

**Key insights:**
- The bound and its OWNER are two different facts. Ze writes the first and has never written the second, so no reader and no gate can tell a safe copy from a stale one.
- The runtime half needs no new machinery. `ze:validate` plus `ValidatorRegistry` is the seam, `diagnostic.RegisterDoctorCheck` is the read-time report, and `CPUSettings.validateAgainst` is the working in-tree example of both.
- The prevention half is one YANG extension and one `le` gate. Nothing runs in the daemon for it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/host/tuning_linux.go` - `applyEthtoolRing` and `setEthtoolRing`. `setEthtoolRing` calls `getEthtoolRingParam`, which fills the whole `ringparam` struct including `rxMaxPending` and `txMaxPending`, then overwrites `rxPending` and `txPending` with the configured values and fires the set ioctl. Neither maximum is read. The two assignments carry a `nolint:gosec` comment reading "bounded by YANG range", which names the static YANG range as the authority for a number the struct beside it already answers correctly.
- [ ] `internal/component/host/ethtool_linux.go` - `ringparam` declares `rxMaxPending`, `rxMiniMaxPending`, `rxJumboMaxPending` and `txMaxPending`. `ethtoolRingparam` returns only `rxPending` and `txPending`, so the read path discards the four maxima as well. `ethtoolIoctl` is the one ioctl wrapper, and `ethtoolGRingparam` and `ethtoolSRingparam` are the only two commands declared.
- [ ] `internal/component/config/system/yang/ze-system-conf.yang` - the `ethtool` list keyed by interface, holding `ring` with `rx` and `tx`, both `uint16` with `range "1..65535"`. Each `ze:help` says "a driver refuses a size it cannot serve", which states the authority in prose and enforces nothing.
- [ ] `internal/component/vpp/yang/ze-vpp-conf.yang` - `rx-queues` and `tx-queues` are `type uint8` with NO range. Both `ze:help` strings end "Ze does not compare the count against the queues the NIC holds." `main-core`, `workers` and `buffers` in the same module also carry no range.
- [ ] `internal/component/vpp/startupconf.go` - `devEntry` writes `num-rx-queues` and `num-tx-queues` into the dpdk dev entry verbatim. No descriptor count is written anywhere in the tree: `num-rx-desc` and `num-tx-desc` appear in no Go file and no YANG file, so the exact VyOS leaf is absent from Ze.
- [ ] `internal/component/vpp/cpuset.go` - `hostCPUInventory` reads the host's online and isolated CPU lists, `hasCore` answers membership, `CPUSettings.validateAgainst` refuses a configured core the host does not run, and `IsolationKnown` keeps "no isolated CPU" apart from "could not read". Fixed at `39bae5125`, where `workerPool` drew from the isolated set without holding it to the online set. This is the pattern the spec generalizes.
- [ ] `internal/component/vpp/register_linux.go` - registers `vppHugepagesDoctorCheck` and `vppCPUIsolationDoctorCheck` through `diagnostic.RegisterDoctorCheck`.
- [ ] `internal/component/kernelcap/kernelcap.go` - `Capability` carries `InUse` over a config tree and `Probe` over the host, and `Evaluate` and `Refuse` turn them into diagnostics. The verdict is present, absent or unknown. It answers whether a kernel feature EXISTS, never what a number's limit is.
- [ ] `internal/component/config/yang/validator_registry.go` - `CustomValidator` carries `ValidateFn`, `CompleteFn` and `DescribeFn`. A nil `ValidateFn` refuses nothing and is documented as a suggestion.
- [ ] `internal/component/config/validators_register.go` - `RegisterValidators` holds 25 registrations today, `crash-memory-image` and `os-device-name` among them.
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` - 30 `ze:` extensions, `validate` and `help` among them. A new extension is uniform with what the tree already carries.
- [ ] `internal/le/config/claims/configclaims.go` - a gate that loads the schema with `configyang.NewLoader` and walks it, with a recorded exception set in the report. The structural model for the new gate.
- [ ] `internal/le/portdefaults/portdefaults.go` - a gate that compares a hand-written Go table against the YANG that restates it. The precedent for "a copy names its source and a check compares them".
- [ ] `internal/component/bgp/reactor/session_connection.go` - `socketRecvBufSize` is a Go constant of 256KB written with `SO_RCVBUF`. It is NOT the YANG `read-buffer-size` leaf, whose `ze:help` says the kernel receive buffer is separate. Read to test the hypothesis that the two BGP buffer leaves are host-bounded. They are not: they size a userspace buffered reader and writer, so their authority is `policy`.

**The population, counted on 2026-09-08:**

| Measure | Count | How it was derived |
|---------|-------|--------------------|
| `range` statements under `internal/` | 343 | grep for the statement across every `.yang` file |
| Files carrying at least one | 60 | the same grep, listing files |
| On a `leaf` / on a `leaf-list` / on a `typedef` | 337 / 2 / 4 | a brace-counted block walk over each declaration |
| Numeric leaves and leaf-lists in the tree | 564 | leaves whose type is an integer, `decimal64`, or `zt:decimal-2` |
| Of those, carrying a `range` | 339 | |
| Of those, carrying NO bound at all | 225 | the larger under-declaration, and where the VPP queue leaves sit |
| Numeric leaves in a `*-conf.yang` module | 429 | the operator-writable half |
| Numeric leaves in a command or API module | 135 | command arguments an operator also types |
| Of the 343, whose own text names a host tell | 66 | leaf block matching sysctl, procfs, sysfs, netlink, ethtool, driver, kernel, NIC, VPP, hardware, hugepage, or device |
| Of those 66, a true host authority after reading each one | 3 | `ring rx`, `ring tx`, `interface mtu` |
| Host-authority leaves with NO range, found outside the 343 | 2 | VPP `rx-queues`, `tx-queues` |

The 63 of 66 that fall out are host-ADJACENT and not host-BOUNDED. The conntrack
timeouts (`1..604800`, 30 leaves) name a sysctl Ze writes, and the kernel imposes
no smaller ceiling, so the seven-day cap is Ze's own policy. `table-size` and
`hash-size` write `nf_conntrack_max` and `nf_conntrack_buckets`, which the kernel
accepts at any `uint32`, so those bounds are memory policy. `accept-ra`,
`arp-announce` and `arp-ignore` copy a kernel value SET rather than a device
limit, and a kernel ABI value set does not vary by machine.

**Behavior to preserve:**
- Ring apply stays non-fatal at apply time. A driver that supports no ring parameters keeps producing a reported, non-blocking error (`docs/architecture/host/tuning.md`).
- A host fact Ze cannot read MUST NOT refuse a commit. `kernelcap` states the rule and `CPUInventory.IsolationKnown` implements it.
- Every existing `range` that this spec classifies `standard` or `policy` keeps its exact numbers. Classification is not re-tuning.

**Behavior to change:**
- A ring size above the driver's maximum is refused at `ze config validate` with the configured value, the driver's maximum and the interface name, instead of failing at apply with none of them.
- A DPDK queue count above the NIC's maximum is refused the same way.
- The `interface mtu` ceiling stops being 16000 and becomes the device's own maximum, read from netlink.
- Every numeric leaf declares its bound's owner.

## Data Flow (MANDATORY)

### Entry Point
- An operator sets a numeric leaf, in the CLI editor or in a config file, and runs `commit` or `ze config validate`.
- A developer adds or edits a numeric leaf in a `.yang` file and runs the verification gate.

### Transformation Path
1. Config text or editor input becomes a `config.Tree`.
2. The YANG validator checks native constraints (`type`, `range`, `pattern`), then runs the leaf's `ze:validate` custom validator if it declares one (`internal/component/config/yang/validator.go`).
3. A host-owned leaf's validator reads the host: an ethtool ioctl for a ring or a channel count, a netlink `RTM_GETLINK` attribute for an MTU. It refuses with both numbers, or reports that it could not read and refuses nothing.
4. `ze doctor` reads the same host facts through a registered doctor check and reports the same comparison without a commit.
5. Separately and offline, `./le config authority check` loads the schema and judges every numeric leaf's declaration.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config validator ↔ host driver | `SIOCETHTOOL` ioctl through the existing `ethtoolIoctl` wrapper | No |
| Config validator ↔ kernel | `RTM_GETLINK`, reading `IFLA_MAX_MTU` and `IFLA_MIN_MTU` | No |
| `le` gate ↔ schema | `configyang.NewLoader`, the loader the claims gate already uses | No |
| Doctor registry ↔ host component | `diagnostic.RegisterDoctorCheck` from the owning package's `init` | No |

### Integration Points
- `internal/component/config/validators_register.go` - three new validator registrations, alongside the 25 that exist.
- `internal/component/config/yang/modules/ze-extensions.yang` - the `authority` extension.
- `internal/le/register.go` - the blank import that registers the new gate, as the claims and coercion gates are registered.
- `internal/core/diagnostic/codes.go` - the codes the new doctor checks report.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `ETHTOOL_GRINGPARAM` fills `rxMaxPending` and `txMaxPending` on the drivers Ze runs on, and a zero there means the driver reports no ring support rather than a limit of zero | `internal/component/host/ethtool_linux.go` declares the fields; the kernel ethtool ABI defines them | A zero maximum read as a limit refuses every ring config on that driver | Unit test over a fake `ringparam` plus a QEMU run against virtio-net, which is the driver the appliance boots with | unvalidated |
| A-2 | `ETHTOOL_GCHANNELS` reports the maximum receive and transmit queue counts for a NIC still bound to its kernel driver, and fails cleanly once `DPDKBinder.bindPCI` has moved it to vfio-pci | `internal/component/vpp/dpdk.go` saves the previous driver, so the pre-bind state is the normal validate-time state | The VPP queue check is unreadable in the common case and only ever warns | Integration test binding and unbinding a device in QEMU, asserting refuse before the bind and warn after | unvalidated |
| A-3 | `IFLA_MAX_MTU` and `IFLA_MIN_MTU` are present in `RTM_GETLINK` replies on the kernels Ze targets | `vendor/golang.org/x/sys/unix/ztypes_linux.go` declares both constants; `nl.ParseRouteAttr` is vendored | The MTU probe reads nothing and every MTU check degrades to unknown | Unit test over a captured reply plus a QEMU assertion on a real link | unvalidated |
| A-4 | `arp-ignore` `range "0..2"` is narrower than the value set the kernel accepts, which the kernel documents as 0 to 8 | `internal/component/iface/yang/ze-iface-conf.yang` declares 0..2; the kernel `ip-sysctl` documentation is the counter-claim and has NOT been read in this repository | Ze refuses a legal ARP policy an operator asks for, which is the VyOS defect in Ze's own tree | Read the kernel documentation for `arp_ignore`, then classify the leaf and, if it is narrow, widen it in this spec | unvalidated |
| A-5 | Every numeric leaf can be classified `standard`, `policy` or `host` with no fourth case | The 66-leaf hand read produced no residue | A fourth token is needed, and the closed vocabulary the gate enforces has to grow | The classification pass itself: any leaf that resists the three names is reported before the annotation lands | unvalidated |
| A-6 | The schema loader answers `config false` for a node, so the gate can scope its population without reading file names | `internal/le/config/claims/configclaims.go` walks the loaded tree today | The population has to be derived from module file names, which is a second declaration of what the schema already says | Read the loader's node type during Phase 1 wiring | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A validator that reads the host makes `ze config validate` machine-dependent, so the same file validates on one host and not another | A `.ci` test that passes locally and fails in CI, or the reverse | That dependence is the POINT, and it is what the class needs. It is bounded by the unknown state: where the host cannot be read, nothing is refused. The functional tests drive a fake host reader, and only the QEMU tests touch a real device |
| R-2 | The classification pass touches roughly 90 YANG files, several owned by components another session is editing, so the annotation commit collides | A merge conflict, or a session reporting hunks it did not write | The annotation is one line per leaf and is applied per module. Land it module by module rather than as one commit, and skip a module another session holds, naming it |
| R-3 | A `host` leaf whose ceiling is widened to the type maximum loses the completion hint the old ceiling gave the operator | Completion offers an absurd value | Completion is a separate job from refusal, and `CustomValidator` already documents that: the validator's `CompleteFn` offers the host's real maximum, which is a better hint than the constant it replaces |
| R-4 | The gate refuses the whole tree until all 564 leaves are annotated, blocking every other session | The verification gate goes red repository-wide | The gate lands GREEN: the annotation pass completes before the gate is registered in `internal/le/register.go`. Phase order in this spec puts the check last for that reason |
| R-5 | A host probe runs on every validate, so a config with 200 interfaces issues 200 ioctls per commit | Commit latency on a large config | Probe once per subject per validate run and reuse it, as `hostCPUInventory` is read once and passed to `validateAgainst` |
| R-6 | The ring maximum a driver reports can change after a firmware or driver change, so a config that validated once is refused later | A commit refused on a config that has not changed | Correct behavior, and the message names the driver's current maximum. The doctor check reports it before the operator meets it at commit |
| R-7 | The `authority` declaration is itself a claim a developer types, so a leaf can declare `policy` over a bound the host really owns | Nothing mechanical sees it | The gate cannot decide this, and the spec says so rather than implying coverage it does not have. What the gate removes is the SILENT case: no leaf can be added without an author answering the question |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A config that used to commit is refused, on the five enrolled leaves only. A wrong probe refuses a legal ring, queue count or MTU and blocks a commit. Nothing on the wire changes and no session is dropped |
| How is it reverted? | Single commit revert. The annotations are inert without the gate, and the gate is registered by one blank import |
| Who else touches this path? | Any session editing a `*-conf.yang` module during the annotation pass. One session is live on `internal/core/eap/`, `internal/component/ike/`, `rfc/`, `test/` and `sdk`, and `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` carries 14 ranges, so that module is annotated last or by that session |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le config authority check` | → | the gate's `Answer` | `TestAuthorityGateIsRegisteredUnderConfig` |
| A `.yang` numeric leaf with no `ze:authority` | → | the gate's schema walk | `TestAuthorityGateRefusesAnUndeclaredNumericLeaf` |
| `ze config validate` over a config setting `ring rx` | → | `ringSizeValidator` through `ValidatorRegistry` | `TestRingValidatorRefusesAboveTheDriverMaximum` |
| `ze config validate` over a config setting `vpp dpdk rx-queues` | → | `dpdkQueueValidator` | `TestQueueValidatorRefusesAboveTheNICMaximum` |
| `ze config validate` over a config setting `interface mtu` | → | `deviceMTUValidator` | `TestMTUValidatorRefusesAboveTheDeviceMaximum` |
| `ze doctor` | → | `hostRingDoctorCheck` | `TestDoctorReportsARingAboveTheDriverMaximum` |
| An operator commits an over-large ring in the CLI | → | the whole chain | `test/config/config-validate-ring-above-driver-max.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A numeric leaf in a loaded YANG module carries no `ze:authority` statement | `./le config authority check` fails and names the module, the leaf path and the missing statement |
| AC-2 | A `ze:authority` whose first token is not `standard`, `policy` or `host` | The check fails and prints the three tokens it accepts |
| AC-3 | `ze:authority "standard"` with no citation text after the token | The check fails, because a standard authority owes the reader which standard |
| AC-4 | A leaf declares `ze:authority "host"` and carries no `ze:validate` on the same leaf | The check fails and names the leaf. A host-owned bound with no runtime validator declares an owner and enforces nothing |
| AC-5 | A leaf declares `ze:authority "host"` and its static `range` upper bound is below the maximum its YANG type can carry | The check fails and prints both numbers. This is the VyOS defect stated mechanically: a ceiling narrower than the type is a copy of a host fact |
| AC-6 | `./le config authority check` runs over the committed tree | It passes, so every numeric leaf carries a declaration |
| AC-7 | `system tuning ethtool eth0 ring rx N` where N is above the driver's reported receive maximum | `ze config validate` fails, and the message carries N, the driver's maximum and `eth0` |
| AC-8 | The same for `ring tx` against the driver's transmit maximum | Same refusal, same three facts in the message |
| AC-9 | A ring value at or below the driver's maximum, and above the old static ceiling if the driver allows it | Validate passes and the value is applied. The check never narrows what the driver accepts |
| AC-10 | The ring parameters cannot be read: no such interface, or a driver with no ring support | Validate does NOT refuse. `ze doctor` reports a warning naming the interface and the reason |
| AC-11 | `ze doctor` with a ring configured above the driver's maximum | An error row with a registered diagnostic code, carrying the same three facts as the validate message |
| AC-12 | `ze doctor` with every configured ring inside its driver's maxima | No row for rings |
| AC-13 | `vpp dpdk interface <pci> rx-queues N` above the NIC's maximum receive queue count, read while the device is on its kernel driver | `ze config validate` fails with N, the maximum and the PCI address |
| AC-14 | The same for `tx-queues` against the NIC's maximum transmit queue count | Same refusal |
| AC-15 | The DPDK device is already bound to vfio-pci, so the channel read fails | Validate does NOT refuse. `ze doctor` reports a warning naming the binding as the reason it could not read |
| AC-16 | `interface <name> mtu N` above the device's `IFLA_MAX_MTU`, or below its `IFLA_MIN_MTU` | `ze config validate` fails with N, the device bound and the interface name |
| AC-17 | An MTU above the old static ceiling of 16000 that the device's `IFLA_MAX_MTU` permits | Validate passes. A loopback or a virtual link that accepts 65536 is configurable |
| AC-18 | The link cannot be read, or the kernel reply carries no `IFLA_MAX_MTU` | Validate does NOT refuse, and doctor warns |
| AC-19 | The `mtu` config leaf and the `bytes` command argument that also names an MTU | Both declare `ze:authority "host"` and both reach the same validator, so the config path and the command path cannot disagree about the ceiling as they do today at 16000 against 65535 |
| AC-20 | `ze doctor --json` output for each new check | Every code it emits is declared in `internal/core/diagnostic/codes.go` and is explained by `ze explain <code>` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets an ethtool ring larger than the NIC supports and commits | CLI editor → config tree → YANG validator → `ringSizeValidator` → ethtool ioctl → refusal naming the driver maximum | `test/config/config-validate-ring-above-driver-max.ci` |
| 2 | Sets a ring the NIC supports but the old static ceiling would once have refused | same path, no refusal, `applyEthtoolRing` writes it | `test/config/config-validate-ring-at-driver-max.ci` |
| 3 | Sets a DPDK queue count above the NIC's maximum before VPP has bound the device | CLI → validator → ethtool channels read → refusal naming the PCI address | `test/vpp/vpp-validate-queues-above-nic-max.ci` |
| 4 | Runs `ze doctor` on a host whose configured ring exceeds the driver | doctor registry → `hostRingDoctorCheck` → error row with code | `test/health/doctor-ring-above-driver-max.ci` |
| 5 | Sets an MTU of 65536 on a loopback | CLI → validator → `RTM_GETLINK` → accepted | `test/ui/config-validate-mtu-device-maximum.ci` |
| 6 | Adds a numeric leaf to a YANG module and forgets the authority statement | `./le config authority check` in the verification gate → refusal naming the leaf | `TestAuthorityGateRefusesAnUndeclaredNumericLeaf` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAuthorityGateRefusesAnUndeclaredNumericLeaf` | `internal/le/config/authority/authority_test.go` | AC-1 over a synthetic module | |
| `TestAuthorityGateRefusesAnUnknownToken` | `internal/le/config/authority/authority_test.go` | AC-2 | |
| `TestAuthorityGateRefusesAStandardWithNoCitation` | `internal/le/config/authority/authority_test.go` | AC-3 | |
| `TestAuthorityGateRefusesAHostLeafWithNoValidator` | `internal/le/config/authority/authority_test.go` | AC-4 | |
| `TestAuthorityGateRefusesAHostCeilingBelowTheTypeMaximum` | `internal/le/config/authority/authority_test.go` | AC-5, the VyOS shape | |
| `TestAuthorityGatePassesOverTheCommittedTree` | `internal/le/config/authority/authority_test.go` | AC-6 | |
| `TestAuthorityGateIgnoresANonNumericLeaf` | `internal/le/config/authority/authority_test.go` | the population boundary | |
| `TestRingValidatorRefusesAboveTheDriverMaximum` | `internal/component/host/ring_validate_test.go` | AC-7, AC-8 against a fake reader | |
| `TestRingValidatorAcceptsAtTheDriverMaximum` | `internal/component/host/ring_validate_test.go` | AC-9 | |
| `TestRingValidatorRefusesNothingWhenTheDriverCannotBeRead` | `internal/component/host/ring_validate_test.go` | AC-10, the unknown state | |
| `TestRingValidatorMessageNamesAllThreeFacts` | `internal/component/host/ring_validate_test.go` | the message contract in AC-7 | |
| `TestRingParamReadKeepsTheMaxima` | `internal/component/host/ethtool_test.go` | the read path stops discarding the reported maxima | |
| `TestSetRingRefusesAboveTheReportedMaximum` | `internal/component/host/tuning_test.go` | `setEthtoolRing` no longer writes a value the struct beside it says is too large | |
| `TestQueueValidatorRefusesAboveTheNICMaximum` | `internal/component/vpp/queue_validate_test.go` | AC-13, AC-14 | |
| `TestQueueValidatorWarnsWhenTheDeviceIsDPDKBound` | `internal/component/vpp/queue_validate_test.go` | AC-15 | |
| `TestMTUValidatorRefusesAboveTheDeviceMaximum` | `internal/component/iface/mtu_validate_test.go` | AC-16 | |
| `TestMTUValidatorAcceptsAboveTheOldStaticCeiling` | `internal/component/iface/mtu_validate_test.go` | AC-17 | |
| `TestMTUValidatorRefusesNothingWithoutTheAttribute` | `internal/component/iface/mtu_validate_test.go` | AC-18 | |
| `TestMTUConfigLeafAndCommandArgumentShareOneValidator` | `internal/component/iface/mtu_validate_test.go` | AC-19 | |
| `TestDoctorReportsARingAboveTheDriverMaximum` | `internal/component/host/doctor_ring_test.go` | AC-11 | |
| `TestDoctorSaysNothingWhenRingsAreInsideTheirMaxima` | `internal/component/host/doctor_ring_test.go` | AC-12 | |
| `TestEveryNewDiagnosticCodeIsDeclared` | `internal/component/host/doctor_ring_test.go` | AC-20 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `system tuning ethtool <if> ring rx` | 1 to the driver's receive maximum | the driver's maximum | 0 | the driver's maximum plus 1 |
| `system tuning ethtool <if> ring tx` | 1 to the driver's transmit maximum | the driver's maximum | 0 | the driver's maximum plus 1 |
| `vpp dpdk interface <pci> rx-queues` | 1 to the NIC's receive queue maximum | the NIC's maximum | 0 | the NIC's maximum plus 1 |
| `vpp dpdk interface <pci> tx-queues` | 1 to the NIC's transmit queue maximum | the NIC's maximum | 0 | the NIC's maximum plus 1 |
| `interface <name> mtu` | the device's `IFLA_MIN_MTU` to its `IFLA_MAX_MTU` | the device maximum | the device minimum minus 1 | the device maximum plus 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `config-validate-ring-above-driver-max` | `test/config/*.ci` | Story 1: the commit is refused and the message names the driver's number | |
| `config-validate-ring-at-driver-max` | `test/config/*.ci` | Story 2: a ring the driver accepts is not refused | |
| `config-validate-ring-unknown-driver` | `test/config/*.ci` | AC-10: an unreadable driver refuses nothing | |
| `vpp-validate-queues-above-nic-max` | `test/vpp/*.ci` | Story 3 | |
| `doctor-ring-above-driver-max` | `test/health/*.ci` | Story 4 | |
| `config-validate-mtu-device-maximum` | `test/ui/*.ci` | Story 5 | |

### Interop Tests (Scope: protocol)
Not applicable. Nothing in this spec reaches the wire: the change is config
validation, a schema declaration, and an offline gate.
`ai/rules/interop-and-goal-validation.md` names a config-only feature with no
protocol impact as the case where an interop scenario is not owed.

## Files to Modify
- `internal/component/config/yang/modules/ze-extensions.yang` - the `authority` extension
- `internal/component/config/validators_register.go` - register `host-ring-size`, `host-dpdk-queues`, `host-device-mtu`
- `internal/component/config/system/yang/ze-system-conf.yang` - `ring rx` and `ring tx` gain `ze:authority "host"` and `ze:validate "host-ring-size"`; the other numeric leaves gain their declaration
- `internal/component/vpp/yang/ze-vpp-conf.yang` - `rx-queues` and `tx-queues` gain the declaration, the validator, and a `ze:help` that no longer says Ze does not compare the count; `main-core`, `workers` and `buffers` gain declarations
- `internal/component/iface/yang/ze-iface-conf.yang` - `mtu` gains the declaration and the validator, and its ceiling widens to the type maximum
- `internal/component/iface/yang/ze-iface-cmd.yang` - the `bytes` MTU argument reaches the same validator
- every other `.yang` module holding a numeric leaf - one `ze:authority` line per leaf
- `internal/component/host/ethtool_linux.go` - the ring read stops discarding the maxima, and gains the channels read
- `internal/component/host/tuning_linux.go` - `setEthtoolRing` refuses a value above the reported maximum instead of writing it, and the two `nolint` comments stop naming the YANG range as the bound
- `internal/component/host/tuning.go` - the ring config carries its host verdict
- `internal/le/register.go` - the blank import for the new gate
- `internal/core/diagnostic/codes.go` - the codes for the three new doctor checks
- `docs/architecture/host/tuning.md` - validate-time refusal beside the apply-time report
- `docs/architecture/config/yang-config-design.md` - the `authority` extension, its three tokens, and what the gate refuses
- `docs/architecture/doctor-and-health-checks.md` - the three new checks and where they register
- `docs/guide/vpp.md` - the queue-count refusal and the unknown case
- `docs/guide/configuration.md` - the MTU ceiling is now the device's
- `docs/features.md` - the operator-visible change
- `ai/CODE-TO-DOCS.md`, `ai/DOCS-TO-CODE.md` - regenerated, never hand-edited

**Design documents declared by the files above, and what each owes.** A `// Design:`
header names a page as the authority for a file, so each is named here whether or
not it changes:
- `docs/architecture/host/inventory.md` - declared by `ethtool_linux.go`. The read side gains the ring maxima and a channel read, and the page describes what the inventory reports, so it is edited.
- `docs/architecture/core-design.md` - declared by the `le` gate files this spec follows. A new gate is an area under `le`, which the page enumerates, so it gains the row.
- `docs/features/ai-first.md` - declared by the same `le` area headers. Unaffected: it describes how an agent reaches the repository's own tooling, and a new gate changes nothing it asserts.

## Files to Create
- `internal/le/config/authority/authority.go` - the schema walk and the four refusals
- `internal/le/config/authority/register.go` - gate registration
- `internal/le/config/authority/report.go` - the report shape, following the claims gate
- `internal/component/host/ring_validate.go` - the ring validator and its host reader
- `internal/component/host/doctor_ring_linux.go` - the ring doctor check
- `internal/component/vpp/queue_validate.go` - the DPDK queue validator
- `internal/component/iface/mtu_validate.go` - the device MTU validator and the `RTM_GETLINK` attribute read
- `test/config/config-validate-ring-above-driver-max.ci` and the five other `.ci` files named above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/config/yang/modules/ze-extensions.yang` for the extension; no new config leaf is added, and every existing numeric leaf gains a statement |
| YANG validation constraints | Yes | The point of the spec. A `host` leaf's static `range` widens to its type maximum and the refusal moves to the validator |
| YANG custom validators | Yes | `host-ring-size`, `host-dpdk-queues`, `host-device-mtu` in `internal/component/config/validators_register.go` |
| CLI commands/flags | Yes | `./le config authority check` and `./le config authority selftest`, following `internal/le/portdefaults/actions.go` |
| CLI grammar (keyword before value) | Yes | The gate is a `le` verb pair with no flags, so the grammar rule is met by shape |
| Editor autocomplete | Yes | Each new validator carries a `CompleteFn` offering the host's real maximum, which replaces the constant ceiling the editor used to suggest |
| Functional test for new RPC/API | Yes | The six `.ci` files above |
| Pipe completeness | Yes | The gate answers through `leaction`, which routes every answer through the pipe layer |
| Env var registration | N-A | No leaf under `environment/` is added or changed |
| Doctor check for runtime dependencies | Yes | Three checks, registered from their owning packages through `diagnostic.RegisterDoctorCheck`, each with a code in `internal/core/diagnostic/codes.go` and a unit and functional test |
| Prometheus counters/metrics | N-A | No new observable runtime state. A refusal is a diagnostic, not a counter |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP wire surface is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`: a numeric bound the host owns is checked against the host |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` for the MTU ceiling; no syntax changes, the accepted VALUES do |
| 3 | CLI command added/changed? | N-A | `./le` is a development entry point, not a shipped CLI command |
| 4 | API/RPC added/changed? | N-A | No RPC is added |
| 5 | Plugin added/changed? | N-A | No plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/vpp.md` for the queue counts |
| 7 | Wire format changed? | N-A | Nothing reaches the wire |
| 8 | Plugin SDK/protocol changed? | N-A | The SDK is untouched |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC requirement is implemented or changed. The `standard` token cites standards, and citing one is not implementing it |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` for the new `.ci` files and the fake host reader they drive |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: this is the behavior VyOS shipped a release to fix, and it is worth one row |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` and `docs/architecture/doctor-and-health-checks.md` |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | N-A | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | Three doctor checks and three validators are registered, so `docs/guide/status.md` names them |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation time by `./le spec citation anchors spec plan/immediate/spec-config-bound-authority.md`. `docs/architecture/host/tuning.md` already carries source anchors on `tuning_linux.go` and `ethtool_linux.go`, so it blocks |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/host/tuning.md` states the ring failure is expected and not retried; that sentence changes |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - the gate exists, is registered, and refuses a synthetic module
   - Tests: `TestAuthorityGateIsRegisteredUnderConfig`, `TestAuthorityGateRefusesAnUndeclaredNumericLeaf`
   - Files: `internal/le/config/authority/authority.go`, `register.go`, `report.go`, `internal/le/register.go`, `internal/component/config/yang/modules/ze-extensions.yang`
   - Verify: the gate runs against a synthetic module and fails on it. It is NOT yet run over the tree
2. **Phase: The four refusals** - the token vocabulary, the citation rule, the validator rule, the ceiling rule
   - Tests: `TestAuthorityGateRefusesAnUnknownToken`, `TestAuthorityGateRefusesAStandardWithNoCitation`, `TestAuthorityGateRefusesAHostLeafWithNoValidator`, `TestAuthorityGateRefusesAHostCeilingBelowTheTypeMaximum`
   - Files: `internal/le/config/authority/authority.go`
   - Verify: each refusal has a synthetic case, and the selftest holds them
3. **Phase: The ring** - the read stops discarding the maxima, the validator refuses, the doctor check reports
   - Tests: `TestRingParamReadKeepsTheMaxima`, the four `TestRingValidator` cases, `TestSetRingRefusesAboveTheReportedMaximum`, `TestDoctorReportsARingAboveTheDriverMaximum`, and `config-validate-ring-above-driver-max`
   - Files: `internal/component/host/ethtool_linux.go`, `ring_validate.go`, `doctor_ring_linux.go`, `tuning_linux.go`, `internal/component/config/validators_register.go`, `internal/core/diagnostic/codes.go`
   - Verify: the functional test refuses the commit and prints the driver's number
4. **Phase: The DPDK queues and the device MTU** - the second and third validators
   - Tests: the `TestQueueValidator` and `TestMTUValidator` cases, `vpp-validate-queues-above-nic-max`, `config-validate-mtu-device-maximum`
   - Files: `internal/component/vpp/queue_validate.go`, `internal/component/iface/mtu_validate.go`, both YANG modules, `ze-iface-cmd.yang`
   - Verify: the MTU config leaf and the MTU command argument produce the same refusal, which they cannot today
5. **Phase: The classification pass** - every numeric leaf declares its owner
   - Tests: `TestAuthorityGatePassesOverTheCommittedTree`
   - Files: every `.yang` module with a numeric leaf, module by module. Any leaf that resists the three tokens is reported before its module is annotated, because A-5 says the vocabulary is closed and a residue disproves it
   - Verify: the gate passes over the tree, then and only then is it added to the verification population
6. **Phase: Documentation** - the pages in the checklist above, each in the phase whose code made it wrong, and not here as a batch. This row states the rule and holds no work of its own

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and the classification pass covers 564 leaves rather than the 339 that carry a range |
| Feature completeness | The five enrolled leaves each refuse, each stay silent when the host cannot be read, and each report through doctor |
| Correctness | A refusal message carries three facts: the configured value, the host's number, and the subject. A message with two of them fails the review |
| Correctness | No probe returns a zero maximum read as a limit. Unreadable and zero are distinct answers, which is the defect `39bae5125` fixed in `cpuset.go` and the rule `ai/rules/principles.md` states |
| Naming | The `ze:authority` token set is exactly `standard`, `policy`, `host`. A fourth token in the tree is a spec change, not an implementation choice |
| Data flow | The host is read once per subject per validate run, not once per leaf |
| Rule: `ai/rules/no-layering.md` | `kernelcap` is not extended and not wrapped. It keeps its boolean verdict, and the numeric verdict rides the `ze:validate` seam that already exists |
| Rule: `ai/rules/stale-comments.md` | The two `nolint` comments reading "bounded by YANG range" and the two `ze:help` sentences saying Ze does not compare are each false after this change |
| Rule: `ai/rules/documentation.md` | `docs/architecture/host/tuning.md` states apply-time behavior this spec supplements. It is edited in phase 3, not at closure |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The `authority` extension exists | grep for `extension authority` in `internal/component/config/yang/modules/ze-extensions.yang` |
| Every numeric leaf declares an owner | `./le config authority check` |
| The gate is in the verification population | `./le verify current mode full` reaches it |
| The ring maxima are read rather than discarded | grep for `rxMaxPending` in `internal/component/host/` shows a reader outside the struct declaration |
| Three validators are registered | grep for the three validator names in `internal/component/config/validators_register.go` |
| Three doctor codes are declared | grep for each code in `internal/core/diagnostic/codes.go` |
| Six functional tests exist and run | list the four `.ci` directories named in the Functional Tests table |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A host probe reads a subject name out of a config path. The name reaches an ioctl request-name field of fixed length, so it is bounded before the copy, as `ethtoolIoctl` already bounds it |
| Fail closed or say something | A probe that cannot read MUST return an explicit unknown. Returning a zero maximum, or returning the type maximum as a stand-in, makes a guard that refuses everything or nothing with no line to delete later |
| Resource exhaustion | A config with many interfaces issues one probe per subject. The read is cached per validate run so a large config cannot multiply ioctls per leaf |
| Error leakage | A refusal names an interface, a PCI address and two numbers. None of them is a secret |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A leaf resists all three authority tokens | STOP. A-5 is broken. Report the leaf and the reading before annotating its module |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The two polarities of the same defect look like opposites and are one bug. VyOS refused a value the hardware accepted; Ze accepts a value the driver refuses. Both are a bound written in a place that does not own it.
- The discovery that shrank this spec: `ze:validate` already exists for exactly this, described in `ze-extensions.yang` as "runtime-determined valid sets". The first design drafted a numeric sibling of `kernelcap` with its own registry, evaluator and refusal path. It was deleted once the seam was read.
- The gate cannot find a leaf that declares `policy` over a host-owned bound. It removes the SILENT case, not the wrong-answer case, and the spec says so rather than implying coverage it does not have.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Declare the authority on the LEAF | Derive the population from the APPLIERS, the files that write to procfs, sysfs, device nodes and ethtool | The applier population is not enumerable. `internal/le/fspersistence` holds 13 kernel and device control files, and it covers neither the netlink MTU write nor any future one. The leaf population IS enumerable, from the schema loader, which is what makes the gate complete rather than best-effort |
| Mandatory declaration on every numeric leaf | Opt-in: declare only `standard` and `host`, and read an absent statement as `policy` | An absent statement reading as a safe answer is the zero-value-as-valid-answer defect `ai/rules/principles.md` names. A forgotten host leaf would be invisible, which is precisely the VyOS failure. The cost is 564 mechanical one-line additions |
| Enforce through `ze:validate` | A new `internal/component/hostlimit` package modelled on `kernelcap` | The seam exists, is registered, is checked at schema build by `CheckAllValidatorsRegistered`, and already runs inside `ze config validate`. A new registry would duplicate the evaluate and refuse plumbing for a second verdict shape |
| Do not extend `kernelcap` | Add numeric fields to `Capability` | Its verdict is present, absent or unknown and carries no subject. A merged struct would carry two disjoint field sets and a branch on which one is filled, which is the `plan/journal/field-carries-two-meanings.md` shape |
| A `host` leaf's static ceiling must equal its type maximum | Delete the `range` entirely on a host leaf | The lower bound is usually a real standard fact, 68 for an IPv4 MTU among them, and the type still needs its parse. Only the CEILING is the copy, so only the ceiling is constrained |
| The gate refuses a `host` leaf with no `ze:validate` | Let the gate resolve the Go function by name with `go/ast` | The schema-local rule is simpler and the Go side is already closed by `CheckAllValidatorsRegistered`. Two checks, each doing one job, with no Go parser in the gate |
| Widen `interface mtu` rather than defer it | Leave `mtu` at 68..16000 and write the device-maximum work as its own spec | The probe is reachable: `unix.IFLA_MAX_MTU` and `IFLA_MIN_MTU` are declared in the vendored `x/sys`, and `nl.ParseRouteAttr` is vendored. The vendored `vishvananda/netlink` link attributes do not expose the field, so this is a raw attribute read rather than a library call, and that is the whole of the extra work |
| One doctor check per subsystem, not one shared check | A single check reading every host-owned leaf | A check is registered by the package that owns the fact, which is how `vppCPUIsolationDoctorCheck` and `vppHugepagesDoctorCheck` are registered. A shared check would have to enumerate the leaves, which is the central list this spec exists to remove |

## Known Limitations

- The `authority` statement is a claim a developer writes. A leaf that declares `policy` over a bound the host really owns passes the gate. What the gate removes is the leaf added with no answer at all.
- The conntrack timeouts, `table-size` and `hash-size` are classified `policy`, so their seven-day and two-million ceilings stay. They are Ze's caps on sysctls the kernel accepts more widely, and widening them is a separate question about what an operator should be able to ask for.
- `accept-ra`, `arp-announce` and `arp-ignore` copy a kernel value SET rather than a device limit. They are classified `standard`, citing the kernel ABI. A-4 records that `arp-ignore` `0..2` may already be narrower than the kernel's own set, and it is settled inside this spec rather than left open.
- VPP itself is never asked. The vendored `go.fd.io/govpp v0.13.0` binapi set holds no `dpdk` package and no `cli_inband` message, so no binary-API call can return a device limit. The queue check asks the DEVICE instead, through ethtool, while the NIC is still on its kernel driver, and reports unknown once VPP has taken it.

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
- [ ] AC-1..AC-20 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** remove the spec file only, since commit A preserves it in history

## Review Gate

<!-- Filled by /ze-close via /ze-review before closure. Loop until 0 BLOCKER, 0 ISSUE. -->

### Run 1
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|

### Run 2
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|
