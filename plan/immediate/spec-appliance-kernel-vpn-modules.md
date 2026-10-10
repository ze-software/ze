# Spec: appliance-kernel-vpn-modules

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config \| tooling |
| Depends | `plan/pre-release/spec-appliance-ships-ze-kernel.md` (AC-6 and AC-7 only; AC-1..AC-5 land without it) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner decision (2026-10-09, showcase planning): "we need the kernel module in the
appliance". Ze's native IKEv2/IPsec installs XFRM state and needs ESP and xfrm
interfaces in the kernel; Ze also configures WireGuard interfaces. An operator on
the appliance who configures either must find the kernel support present.

Owner instruction (2026-10-09, dictated): the VPN modules are a Linux kernel
CONFIG setting, and the work belongs with the decision that the appliance always
runs Ze's own compiled kernel.

-> Decision (owner, 2026-10-09): this stays a SEPARATE spec from
`plan/pre-release/spec-appliance-ships-ze-kernel.md`. The dependency, as found at
source: the config, manifest and floor work (AC-1..AC-5) stands on its own,
because Ze's runtime kernel is already what the QEMU labs and the deployment
proofs boot; only the claim about the DEFAULT image (AC-6, AC-7) waits on that
spec making Ze's kernel the only image kernel.
-> Decision (owner, 2026-10-09): "we want to control the modules so we need to
compile our own kernel". The arm64 kernel is built natively on the Mac, the amd64
kernel natively on the Linux host; no CPU emulation is ever involved. Recorded and
enforced in `spec-appliance-ships-ze-kernel.md` (its builder owns that rule).
-> Decision (owner, 2026-10-09): N100 (amd64) is a supported build target.
Booting on N100 hardware is owner-deferred until the owner runs real hardware
again; it is not part of this work's evidence and not a gap this spec chose.
-> Open (owner): the GPLv2 source offer for the shipped kernel. The owner asked
what it is; it is explained to him and NOT decided here. It belongs to
`spec-appliance-ships-ze-kernel.md` (R-7), which changes who distributes the kernel.
-> Decision (owner, 2026-10-09, closes the line above): GPLv2 compliance points
users to the published Linux source through a per-image notice (version, tarball
URL, SHA-256, derived from the build's own declaration). Recorded with its AC in
`spec-appliance-ships-ze-kernel.md` (AC-17). This spec ships no image of its own,
so it owes no notice AC; its config changes ride in the image that spec builds.
-> Owner 2026-10-09: no AGPLv3 notice in the image; Ze is the owner's own code, published on GitHub.
-> Decision (owner, 2026-10-09): the kernel pin is the latest stable 7.x by exact version and digest (ships-ze-kernel AC-19:
7.2 pinned today, 7.2.9 latest on 2026-10-09). AC-5's native builds here therefore
run on whatever exact version AC-19 pins; if AC-19 has not landed first, AC-5 is
built twice, so land the bump first.

Goal: every kernel facility Ze's IPsec and WireGuard code asks for is BUILT IN to
Ze's runtime kernel on both arches, the build refuses a config that drops one, the
list of required symbols is derived from what Ze's XFRM code installs rather than
written beside it, and a booted Ze kernel proves an SA and a WireGuard interface
carry traffic.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/appliance/kernel-profiles.md` - profile registry, manifest, universal floor
  → Constraint: requirements are enforced over the EMITTED `build/config` after `olddefconfig` (`enforceKernelRequirements`) and before compile in the worker (`enforceRequiredSymbols`); a symbol added to `runtime.config` but silently dropped by Kconfig is caught only if it is also in `runtime.require` or the compiled floor.
  → Constraint: the page already states the built-in rule for flow export and nftables logging ("a modular logger leaves log-rule installation failing with ENOENT ... where Alpine cannot load Ze's kernel modules"); the VPN symbols follow the same rule and the page gains one paragraph for them.
- [ ] `docs/guide/appliance.md` "Runtime Kernel Requirements" - the operator-facing list
  → Decision: the IPsec/WireGuard row there names the symbols this spec requires; edited in the same change.
- [ ] `docs/guide/ipsec.md` - line 139 already states the XFRM backend needs Linux 7.2 `XFRM_MSG_MIGRATE_STATE` and `CONFIG_XFRM_MIGRATE`
  → Decision: the guide gains the per-cipher kernel requirement only if an operator can meet a refusal; with every transform built in, the guide states the appliance carries them all.
- [ ] `docs/architecture/testing/qemu-integration.md` - QEMU guests boot `tmp/kernel/build/vmlinuz`
  → Constraint: `ipsec-mobike-test` (`internal/le/test/qemu/mobike.go`) already boots Ze's runtime kernel and runs IPsec in the guest; the SA proof extends that lab rather than adding a second one.
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - declared by `xfrm_linux.go`; Child SA install into XFRM, refusals before the kernel
  → Constraint: the page names no per-cipher kernel requirement today; if the NULL spelling changes under AC-6, or the transform tables gain the symbol mapping, the page gains one paragraph naming the kernel symbol each transform needs, in the same change.
- [ ] `ai/rules/platform-linux.md`
  → Constraint: appliance dependency changes go through the bump runbook; a symbol that does not resolve on one arch turns a missing feature into a refused build, so every new required symbol is checked on BOTH arches' emitted configs before it enters the floor.

### Source files
- [ ] `gokrazy/kernel/runtime.config`, `gokrazy/kernel/kernel.config`, `gokrazy/kernel/runtime.require`, `gokrazy/kernel/kernel.require`
- [ ] `internal/appliance/kernelreq.go` - `runtimeKernelRequirements` (compiled floor)
- [ ] `internal/appliance/kernelconfig_pairing_test.go` - `unverifiedRuntimeSymbols`
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - `xfrmEncNames`, `xfrmAEADNames`, `xfrmAuthNames`
- [ ] `internal/component/ike/engine/kernelcap_linux.go` - runtime capability enrolment (`CONFIG_XFRM_USER`)
- [ ] `internal/plugins/flowexport/conntrack_setup_appliance_linux.go` - states modprobe is absent on gokrazy

**Key insights:**
- Ze's runtime kernel ALREADY carries the core VPN options built in. The open work is (1) three crypto transforms Ze's XFRM code names that the kernel does not hold, (2) the enforcement gap (several symbols set but required nowhere), (3) proof on a booted kernel. Adding the options to rtr7 is impossible (it is a pinned upstream binary package) and would be layering under `ai/rules/no-layering.md` in any case; the rtr7 removal is the other spec.
- `=m` is not "present" on the appliance: gokrazy ships no `modprobe`, so the kernel's `request_module` finds no helper and a modular transform is never loaded. Every requirement here is `=y`.

-> Constraint (found at implementation, 2026-10-10): ad0931ae26 had already set AC-1's three lines `=y` in `runtime.config` and declared every VPN and CRYPTO symbol in `runtime.require`, and `xfrmTransformKernel` (`internal/component/ike/dataplane/kernelcap_linux.go`) already maps each table transform to its symbol for the kernelcap enrolment. The derivation test reads that map; no second map was written. The test is `xfrm_kernelsym_linux_test.go`, because the tables are Linux-only.
-> Constraint (read from linux-7.2 `net/xfrm/Kconfig`, 2026-10-10): `config XFRM_ESP` selects `CRYPTO_AES`, `CRYPTO_AUTHENC`, `CRYPTO_CBC`, `CRYPTO_ECHAINIV`, `CRYPTO_GCM`, `CRYPTO_HMAC`, `CRYPTO_SEQIV` and `CRYPTO_SHA256`, so `CONFIG_INET_ESP=y` forces the templates every transform uses; `crypto/sha256.c` implements `hmac(sha256)` itself. The per-transform requirement is therefore the one distinguishing symbol each `xfrmTransformKernel` entry names.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `~/.cache/ze/runtime-kernel/7.2-runtime-arm64-runtime-d1de5ab8-cd0e7a40/config` - the emitted arm64 config of Ze's runtime kernel (built 2026-10-02 by Docker, `modules=yes`, 1627 `.ko` files); no amd64 emitted config exists on this host
- [ ] `gokrazy/kernel/runtime.config` - sets `CONFIG_INET_ESP`, `CONFIG_INET6_ESP`, `CONFIG_INET_AH`, `CONFIG_INET6_AH`, `CONFIG_XFRM_STATISTICS`, `CONFIG_XFRM_MIGRATE`, `CONFIG_XFRM_INTERFACE`, `CONFIG_WIREGUARD`, `CONFIG_NET_UDP_TUNNEL`, `CONFIG_PSTORE` all `=y`
- [ ] `gokrazy/kernel/kernel.config` - sets `CONFIG_XFRM_USER=y` (merged into the runtime build too)
- [ ] `internal/appliance/kernelreq.go` - floor holds `CONFIG_INET_ESP`, `CONFIG_INET6_ESP`, `CONFIG_XFRM_STATISTICS`, `CONFIG_XFRM_MIGRATE`, `CONFIG_PSTORE`, `CONFIG_PSTORE_RAM`, `CONFIG_MODULES`; not `XFRM_USER`, `XFRM_INTERFACE`, AH, `WIREGUARD`
- [ ] `gokrazy/kernel/runtime.require` - holds ESP, AH, `XFRM_STATISTICS`, `XFRM_INTERFACE`, `XFRM_MIGRATE`, `PSTORE`, `MODULES`; not `XFRM_USER`, not `WIREGUARD`; `kernel.require` holds no XFRM symbol
- [ ] `internal/appliance/kernelconfig_pairing_test.go` - `CONFIG_WIREGUARD` sits in `unverifiedRuntimeSymbols`, so it is set but never enforced
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - the kernel transform names Ze installs: `cbc(aes)`, `cbc(des3_ede)`, `ecb(cipher_null)`, `rfc4106(gcm(aes))`, `rfc7539esp(chacha20,poly1305)`, `hmac(sha1|sha256|sha384|sha512)`; the `null` entry carries a "verify against target kernel" note about the `ecb(cipher_null)` spelling
- [ ] `internal/component/ike/ipsec/algorithm_support.go` - `EncryptionImplementedESP` accepts every implemented cipher except AES-CCM
- [ ] `internal/component/ike/engine/kernelcap_linux.go` - IPsec enrols one runtime probe, `CONFIG_XFRM_USER` via `kernelcap.XFRM`; no probe for a cipher

State of each VPN symbol in the arm64 emitted config:

| Symbol | Emitted | Needed by | Enforced by |
|--------|---------|-----------|-------------|
| `CONFIG_XFRM_USER` | `=y` | every SA install (netlink) | nothing at build; runtime kernelcap probe |
| `CONFIG_XFRM_INTERFACE` | `=y` | route-based IPsec | `runtime.require` |
| `CONFIG_XFRM_MIGRATE` | `=y` | MOBIKE | floor + require |
| `CONFIG_XFRM_STATISTICS` | `=y` | SAD counters | floor + require |
| `CONFIG_INET_ESP`, `CONFIG_INET6_ESP` | `=y` | ESP | floor + require |
| `CONFIG_INET_AH`, `CONFIG_INET6_AH` | `=y` | AH (OSPFv3 RFC 4552) | `runtime.require` |
| `CONFIG_WIREGUARD` | `=y` | WireGuard interfaces | nothing (`unverifiedRuntimeSymbols`) |
| `CONFIG_CRYPTO_GCM`, `SEQIV`, `ECHAINIV`, `AUTHENC`, `CBC`, `AES`, `HMAC`, `SHA1`, `SHA256`, `SHA512` | `=y` | the transforms above | nothing (defconfig carries them) |
| `CONFIG_CRYPTO_CHACHA20POLY1305` | not set | `rfc7539esp(chacha20,poly1305)` | nothing: DEFECT |
| `CONFIG_CRYPTO_DES` | `=m` | `cbc(des3_ede)` | nothing: DEFECT (no modprobe) |
| `CONFIG_CRYPTO_NULL` | not set | `ecb(cipher_null)` | nothing: DEFECT |
| `CONFIG_PSTORE`, `CONFIG_PSTORE_RAM` | `=y` | crash capture (not VPN; noted for the parked crash-capture spec) | floor + require |

**Behavior to preserve:**
- requirement enforcement order: worker before compile, Go after compile, over the emitted config
- `kernelconfig_pairing_test.go`'s rule that every `=y` line is required or listed with a reason
- the transform names Ze installs (`xfrm_linux.go` tables), including the refusal of an unknown word

**Behavior to change:**
- the three missing transforms become built in; the VPN symbols become required in `runtime.require` and the compiled floor
- the per-transform requirement is derived from the XFRM name tables, so a cipher added to Ze's tables without its kernel symbol fails a test

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze appliance kernel --target runtime --arch <arch>` (and, after the dependency lands, `ze appliance build`) reading `gokrazy/kernel/*.config` and `*.require`

### Transformation Path
1. Worker merges `kernel.config` + `runtime.config` over `defconfig`, `olddefconfig`, then `enforceRequiredSymbols` (manifest) before compile
2. Go `enforceKernelRequirements` checks the emitted config against `runtime.require` + `runtimeKernelRequirements`
3. Tree cached under `~/.cache/ze/runtime-kernel/<version>-<variant>/`, copied to the output dir QEMU boots
4. At runtime Ze's IKE installs SAs naming the kernel transforms; WireGuard creates `wireguard` links over netlink

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze → kernel XFRM | netlink `XFRM_MSG_NEWSA` with transform name | No (AC-6) |
| Ze → kernel WireGuard | rtnetlink link + genetlink config | No (AC-6) |

### Integration Points
- `runtimeKernelRequirements` - extended, not duplicated
- the XFRM name tables - become the source the symbol requirement derives from

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | same enforcement path as every other runtime symbol |
| No unintended coupling | Yes | the derivation test reads the dataplane tables from a test in the dataplane package; `internal/appliance` gains no import of IKE |
| No duplicated functionality | Yes | the floor is extended; no second checker |
| Zero-copy preserved | N-A | build config, no wire path |
| Registration over hardcoding, outbound | N-A | nothing registered |
| Registration over hardcoding, inbound | Yes | the transform-to-symbol map is a copy of a fact the kernel owns, so it carries a check comparing it to `runtime.require` and to the dataplane tables (`ai/rules/principles.md`: "the copy names its source and a check compares them") |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | the amd64 emitted config matches arm64 for every symbol in the table above | both arches merge the same fragments; only defconfig differs, and defconfig is where the crypto symbols come from | an amd64 build refuses, or ships a transform as `=m` | the amd64 native build on the Linux host reads each symbol (AC-5) | unvalidated |
| A-2 | gokrazy cannot load a `=m` transform (no `modprobe` helper) | `conntrack_setup_appliance_linux.go` comment; gokrazy image holds two ze binaries plus randomd/heartbeat | `=m` would be enough and `=y` costs only kernel size | `ls /sbin/modprobe` in a booted guest | unvalidated |
| A-3 | `ecb(cipher_null)` is the spelling the 7.2 kernel accepts for NULL ESP | `xfrm_linux.go` NOTE says unverified | NULL ESP install fails even with `CONFIG_CRYPTO_NULL=y` | AC-6 installs a NULL-ESP SA; a refusal is a defect in the name table, fixed here | unvalidated |
| A-4 | Kconfig for 7.2 exposes `CONFIG_CRYPTO_CHACHA20POLY1305` and `CONFIG_CRYPTO_NULL` as user-selectable on both arches | emitted config prints them as "is not set" (so they exist on arm64) | the symbol name changed; requirement must name the 7.2 symbol | AC-5 emitted configs | unvalidated |
| A-5 | `ipsec-mobike-test` can install SAs with a chosen ESP transform | the lab runs Ze IKE in a guest on Ze's kernel | a new lab is needed | read `internal/le/test/deployment/mobike.go` at implementation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | a new required symbol resolves on one arch only and refuses that arch's build | `enforceKernelRequirements` error naming the symbol | both native builds read before the floor changes (AC-5) |
| R-2 | the variant hash changes, so every cached kernel is rebuilt (about 30 minutes cold) | builder banner on next run | expected cost of a config change; the cache survives reclamation per the ships-ze-kernel spec |
| R-3 | sibling QEMU work boots `tmp/kernel/build` while a rebuild rewrites it | flapping guest boots | builds for this spec write a separate output dir (`ZE_KERNEL_TEST_OUTPUT_DIR`), never `tmp/kernel/build` while another session uses it |
| R-4 | DES and NULL are weak; building them in widens what an operator can configure | none | they are already accepted by Ze's config and installed by its dataplane; the kernel refusing them is a silent install failure, not a policy. Policy belongs in config validation, not in a missing kernel symbol |
| R-5 | the showcase's Docker host kernel differs from Ze's | showcase IPsec chapter fails | out of this spec; the showcase agent owns the Docker kernel probe (`plan/spec-terminal-demo-showcase.md` R-2) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a runtime kernel build refuses (visible at build), or an SA with one cipher fails to install (visible at runtime) |
| How is it reverted? | single commit revert of the fragment, manifest and floor |
| Who else touches this path? | `spec-appliance-ships-ze-kernel.md` (same fragments and cache), crash-capture (PSTORE), kernel-capability-gate |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze appliance kernel --target runtime` | → | `enforceKernelRequirements` over the emitted config | `TestRuntimeFloorCarriesVPNSymbols` (`internal/appliance/kernelreq_test.go`) |
| Ze IKE SA install | → | `xfrmEncName` / `xfrmAEADName` / `xfrmAuthName` → kernel transform | `TestXfrmTransformsHaveRequiredKernelSymbol` (`internal/component/ike/dataplane/xfrm_kernelsym_test.go`) |
| booted Ze runtime kernel | → | IKE SA per transform + WireGuard link | `./le test qemu ipsec-mobike-test` extended, plus the WireGuard lab named in AC-6 |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `gokrazy/kernel/runtime.config` | sets `CONFIG_CRYPTO_CHACHA20POLY1305=y`, `CONFIG_CRYPTO_DES=y`, `CONFIG_CRYPTO_NULL=y` |
| AC-2 | `runtime.require` and `runtimeKernelRequirements` | both carry `CONFIG_XFRM_USER`, `CONFIG_XFRM_INTERFACE`, `CONFIG_INET_ESP`, `CONFIG_INET6_ESP`, `CONFIG_INET_AH`, `CONFIG_INET6_AH`, `CONFIG_WIREGUARD` and the crypto symbols of AC-3; `CONFIG_WIREGUARD` leaves `unverifiedRuntimeSymbols` |
| AC-3 | every kernel transform name in `xfrmEncNames`, `xfrmAEADNames`, `xfrmAuthNames` | maps to a kernel symbol listed in `runtime.require`; a name with no mapping, or a mapping to a symbol `runtime.require` lacks, fails the test naming the transform |
| AC-4 | an emitted config with any one AC-2 symbol `=m` or unset | `enforceKernelRequirements` refuses, naming the symbol (unit test over a fixture config, one case per symbol) |
| AC-5 | native arm64 build on the Mac and native amd64 build on the Linux host, after AC-1 | each emitted config reads every AC-2/AC-3 symbol `=y`; both configs' symbol lines are pasted in the spec |
| AC-6 | Ze's runtime kernel booted under QEMU (arm64, HVF) | an IKE SA installs and carries traffic for each ESP transform Ze accepts (aes-gcm, aes-cbc+sha256, chacha20poly1305, 3des+sha1, null+sha256), and a WireGuard interface passes a ping; the guest has no `/sbin/modprobe` (A-2) |
| AC-7 | after `spec-appliance-ships-ze-kernel.md` lands: default `ze appliance build` image | its packaged `config` reads every AC-2/AC-3 symbol `=y` |
| AC-8 | `docs/architecture/appliance/kernel-profiles.md` and `docs/guide/appliance.md` | name the VPN symbols, the built-in rule, and their source anchors |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRuntimeFloorCarriesVPNSymbols` | `internal/appliance/kernelreq_test.go` | AC-2 | |
| `TestEnforceRefusesModularVPNSymbol` | `internal/appliance/kernelreq_test.go` | AC-4 | |
| `TestXfrmTransformsHaveRequiredKernelSymbol` | `internal/component/ike/dataplane/xfrm_kernelsym_test.go` | AC-3, reads `gokrazy/kernel/runtime.require` from the repo root | |
| existing `kernelconfig_pairing_test.go` | `internal/appliance/` | AC-1 lines are required, not unverified | |

### Boundary Tests (numeric inputs)
N-A: no numeric input.

### Functional Tests
| Test | Location | Validates |
|------|----------|-----------|
| `./le test qemu ipsec-mobike-test` (transform matrix added) | `internal/le/test/qemu/mobike.go`, `internal/le/test/deployment/mobike.go` | AC-6 IPsec |
| WireGuard guest lab (extend `plan/spec-wireguard-runtime-proof.md`'s lab if it exists at implementation, else a guest step in the same lab) | `internal/le/test/qemu/` | AC-6 WireGuard |

### Interop Tests (Scope: protocol)
N-A: no wire behavior changes; the kernel gains transforms Ze already negotiates. The IPsec interop suites keep their peers.

## Files to Modify
- `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require`
- `internal/appliance/kernelreq.go`, `internal/appliance/kernelconfig_pairing_test.go`
- `internal/component/ike/dataplane/xfrm_linux.go` (the NULL spelling note resolved by AC-6)
- `internal/le/test/qemu/mobike.go`, `internal/le/test/deployment/mobike.go`
- `docs/architecture/appliance/kernel-profiles.md`, `docs/guide/appliance.md`, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`

## Files to Create
- `internal/component/ike/dataplane/xfrm_kernelsym_test.go`
- `internal/appliance/kernelreq_test.go` (if absent; else extend)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config surface |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | N-A | no command |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | N-A | no leaf |
| Functional test for new RPC/API | N-A | no RPC; QEMU lab in AC-6 |
| Pipe completeness | N-A | no output |
| Env var registration | N-A | no env var |
| Doctor check for runtime dependencies | No | the runtime dependency (`CONFIG_XFRM_USER`) already has its kernelcap probe and doctor codes; per-cipher absence is made impossible at build rather than diagnosed at runtime |
| Prometheus counters/metrics | N-A | none |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the IPsec feature file under `features/` gains the QEMU evidence of AC-6 |
| 2 | Config syntax changed? | N-A | none |
| 3 | CLI command added/changed? | N-A | none |
| 4 | API/RPC added/changed? | N-A | none |
| 5 | Plugin added/changed? | N-A | none |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md` "Runtime Kernel Requirements" |
| 7 | Wire format changed? | N-A | none |
| 8 | Plugin SDK/protocol changed? | N-A | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | RFC 7634 (ChaCha20-Poly1305 ESP) and RFC 4303 transforms gain runtime proof; `rfc/short/` rows touched only if a tagged test is added |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/qemu-integration.md` if the mobike lab gains the matrix |
| 11 | Affects daemon comparison? | N-A | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/appliance/kernel-profiles.md` |
| 13 | Route metadata keys added/changed? | N-A | none |
| 14 | Prometheus counters added/changed? | N-A | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/immediate/spec-appliance-kernel-vpn-modules.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | N-A | no example changes |

## Implementation Steps
1. AC-3 test first (red: chacha20poly1305, 3des, null have no required symbol)
2. AC-1 fragment lines, AC-2 manifest and floor, pairing list (green)
3. AC-4 fixture tests
4. Native arm64 build into a separate output dir; native amd64 build on the Linux host; paste symbols (AC-5)
5. AC-6 guest matrix; fix the NULL spelling if the kernel refuses it
6. Docs (AC-8) in the same change as step 2
7. AC-7 once the ships-ze-kernel spec lands

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Built in, not modular | every required symbol `=y` in both emitted configs |
| Derived, not listed | AC-3 test reads the dataplane tables, not a copied cipher list |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| fragment + manifest + floor | `grep` of the three files, AC-2 test |
| both emitted configs | pasted symbol lines (AC-5) |
| guest proof | `./le test qemu ipsec-mobike-test` output |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Weak transforms | DES and NULL become installable in the kernel; confirm Ze's config validation is where an operator is stopped, and that nothing defaults to them |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The skeleton's premise ("the appliance lacks the modules") is true of the image it ships today (rtr7) and false of Ze's own kernel, which carries ESP, AH, xfrm interfaces and WireGuard built in. What Ze's kernel lacks is narrower: three cipher transforms Ze's dataplane names.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| separate spec, depending on ships-ze-kernel for AC-6/AC-7's default-image claim | fold into ships-ze-kernel | owner decision 2026-10-09 |
| every VPN symbol `=y` | `=m` plus a module loader in Ze | gokrazy has no modprobe; the repo's rule for flow export and nftables logging is already "built in" |
| derive the cipher symbols from the XFRM name tables | hand list in `runtime.require` only | a cipher added to Ze's tables without its kernel symbol is the exact defect found here |
| build DES and NULL in | drop them from Ze's tables | Ze accepts them in config today; removing them is a scope change the owner has not asked for |

## Known Limitations
- N100 hardware boot: owner-deferred (decision above), not evidence for this spec.
- GPLv2 source offer: decided by the owner 2026-10-09, carried by `spec-appliance-ships-ze-kernel.md` AC-17.

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only
