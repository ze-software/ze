# Spec: appliance-kernel-vpn-modules

| Field | Value |
|-------|-------|
| Status | blocked |
| Priority | P2 |
| Release reason | The delivered appliance must contain the kernel facilities its VPN configuration requires. |
| Scope | config \| tooling |
| Depends | `plan/release/delivery/spec-appliance-ships-ze-kernel.md` (AC-6 and AC-7 only; AC-1..AC-5 land without it) |
| Phase | 5/7 — native proof accepted; closure handed back for the promised Mac cache receipt |
| Handoff | - |
| Updated | 2026-10-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Blocker

Independent closure decision, 2026-10-28: hand back for the missing read-only
Mac cache receipt requested by
`plan/handover/27-macos-vpn-kernel-proof.md`, “Return to the Linux session”.
This is an evidence-delivery gap, not a failed VPN test or a demonstrated
kernel defect.

### Receipt sufficiency

- AC-5's literal post-AC-1 native-build and pasted-symbol requirements have
  historical arm64 and amd64 receipts below. They remain evidence for those
  builds; they are not copies from the repaired Mac tree.
- The Mac run at `ceb83b7140c088445a1f7edcdfa682186201e3f2`, image
  `4c4d23d6ca4234aa`, supplies native arm64/HVF traffic and no-module evidence.
  `internal/le/test/qemu/vpnproof.go` `runVPNKernelHere` calls
  `vpnKernelConfig` before boot; `requireVPNBuiltins` rejects a manifest symbol
  unless the selected emitted config contains `=y`. That prerequisite is
  stronger than checking requested fragments, but it returns only a verdict:
  it does not return the config, provenance or installed module manifest.
- The transferred build log shows non-modular compilation of `pwm-fan.o`
  and installation of `modules.builtin`; the console shows missing
  `pwm-fan.ko` and an empty loaded-module list. Neither contains the requested
  installed manifest entry or the selected cache's complete config and
  `kernel.version`. The five transferred files therefore do not discharge
  the explicit post-repair handover. The older AC-5 tables cannot replace it.

### Diagnosis and clearing condition

| Item | Finding |
|------|---------|
| Symptom | The successful Mac transfer has host, revision, image-build, VPN and console logs only; the requested selected-cache readback is absent. |
| Producer | `internal/le/test/qemu/vpnproof.go` `vpnKernelConfig` reads the config but emits no success receipt. `internal/appliance/runtimekernel.go` `ImageRuntimeKernelTree` selects the runtime tree; `internal/appliance/cache.go` `kernelCacheVariantFor` keys it from resolver inputs. `internal/appliance/kernelbuilder/worker.go` `copyRuntimeOutputs` installs the module tree. |
| Owning layer | Evidence transfer under handover 27, owned by the Mac agent through the parent session; no change to kernel enforcement or traffic assertions is required. |
| [workaround] | Treat the historical symbol paste and current PASS as the requested direct cache receipt. Rejected: this silently drops a promised artifact and does not identify the repaired installed tree. |
| [source] | Return the retained run's selected kernel key, actual `config` and `kernel.version` paths and contents, the three fan-related `=y` lines, installed `modules.builtin` fan entry, and an absence check for loadable `pwm-fan.ko` in that same tree. Reconcile selection with the run's resolver inputs, never the cache date. |

The parent must obtain that read-only transfer from the Mac agent. No identified
Mac SSH destination is available to this closure phase. If the retained tree
cannot be identified, return that fact; do not rebuild, boot, purge caches,
choose a newer tree or repeat passing traffic. Once reconciled, the parent
sets this spec back to `in-progress` and resumes the independent closure gate
and its two-commit removal. This spec continues to own the missing receipt.

This phase read the five transferred files and the producing source; it ran no
build, test, formatter, lint or broad verification. It did not run the final
Review Gate or claim a CLEAN review artifact. The unsafe `verify changed`
scope recorded in `bfc1ec9876` remains separately owned by
`plan/post-release/engineering/spec-verify-scope-0-umbrella.md`; it is not the
pre-closing blocker and must not be rerun unchanged. L2TP privileged proof and
the WireGuard configuration-commit workflow are not blockers for this spec.

## Task

Owner decision (2026-10-09, showcase planning): "we need the kernel module in the
appliance". Ze's native IKEv2/IPsec installs XFRM state and needs ESP and xfrm
interfaces in the kernel; Ze also configures WireGuard interfaces. An operator on
the appliance who configures either must find the kernel support present.

Owner instruction (2026-10-09, dictated): the VPN modules are a Linux kernel
CONFIG setting, and the work belongs with the decision that the appliance always
runs Ze's own compiled kernel.

-> Decision (owner, 2026-10-09): this stays a SEPARATE spec from
`plan/release/delivery/spec-appliance-ships-ze-kernel.md`. The dependency, as found at
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
- [ ] `docs/architecture/testing/qemu-integration.md` - native Docker-host guest and VPN proof
  → Constraint: `vpn-kernel-test` uses the shared Docker-host transport, not the MOBIKE movement scenarios. `runVPNKernelAt` selects the ESP matrix and WireGuard kernel check; A-5 records why the original MOBIKE-extension plan was replaced.
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
- [ ] `internal/le/test/qemu/vpnproof.go`, `internal/le/test/deployment/vpnproof.go` - native kernel/config preparation and matrix entry point
- [ ] `internal/le/test/deployment/wireguardproof.go`, `internal/le/interoplab/ipsec/vpnproof.go` - live guest-root, traffic and transform checks

**Key insights:**
- Ze's runtime kernel ALREADY carries the core VPN options built in. The open work is (1) three crypto transforms Ze's XFRM code names that the kernel does not hold, (2) the enforcement gap (several symbols set but required nowhere), (3) proof on a booted kernel. Adding the options to rtr7 is impossible (it is a pinned upstream binary package) and would be layering under `ai/rules/no-layering.md` in any case; the rtr7 removal is the other spec.
- Every VPN requirement here is `=y`. The proof disables module loading and refuses loaded modules before traffic; absence of a `modprobe` helper alone does not exclude gokrazy's direct boot-module loader.

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
| A-1 | the amd64 emitted config matches arm64 for every symbol in the table above | both arches merge the same fragments; only defconfig differs, and defconfig is where the crypto symbols come from | an amd64 build refuses, or ships a transform as `=m` | the amd64 native build on the Linux host reads each symbol (AC-5) | validated 2026-10-12: the arm64 and amd64 emitted configs read the same 22 symbol lines (AC-5 Evidence) |
| A-2 | gokrazy cannot load a `=m` transform (no `modprobe` helper) | `conntrack_setup_appliance_linux.go` comment; gokrazy image holds two ze binaries plus randomd/heartbeat | `=m` would be enough and `=y` costs only kernel size | `vpnGuestPreparation`, `checkWireGuardProof`, native guest receipts | insufficient premise: gokrazy's `loadModule` calls `unix.FinitModule` directly for its boot module list. The proof instead disables module loading, refuses any loaded module and removes the helper; the accepted native arm64 run satisfies those guards. |
| A-3 | `ecb(cipher_null)` is the spelling the 7.2 kernel accepts for NULL ESP | `xfrm_linux.go` NOTE originally said unverified | NULL ESP install fails even with `CONFIG_CRYPTO_NULL=y` | AC-6 installs a NULL-ESP SA; a refusal is a defect in the name table, fixed here | validated 2026-10-27: `xfrmEncNames` supplies `ecb(cipher_null)` and `esp-integrity-only-null` passed installed-state and traffic checks against strongSwan on native amd64/KVM and arm64/HVF. |
| A-4 | Kconfig for 7.2 exposes `CONFIG_CRYPTO_CHACHA20POLY1305` and `CONFIG_CRYPTO_NULL` as user-selectable on both arches | emitted config prints them as "is not set" (so they exist on arm64) | the symbol name changed; requirement must name the 7.2 symbol | AC-5 emitted configs | validated 2026-10-12: both 7.2.9 emitted configs read `CONFIG_CRYPTO_CHACHA20POLY1305=y` and `CONFIG_CRYPTO_NULL=y` (AC-5 Evidence) |
| A-5 | `ipsec-mobike-test` can install SAs with a chosen ESP transform | the lab runs Ze IKE in a guest on Ze's kernel | a new lab is needed | read `internal/le/test/deployment/mobike.go` at implementation | broken 2026-10-25: `RunMOBIKENetns` runs only `mobike-initiator` and `mobike-responder`; the QEMU wrapper supplies an Alpine package environment, not the default appliance image |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | a new required symbol resolves on one arch only and refuses that arch's build | `enforceKernelRequirements` error naming the symbol | both native builds read before the floor changes (AC-5) |
| R-2 | the variant hash changes, so every cached kernel is rebuilt (about 30 minutes cold) | builder banner on next run | expected cost of a config change; the cache survives reclamation per the ships-ze-kernel spec |
| R-3 | sibling QEMU work boots `tmp/kernel/build` while a rebuild rewrites it | flapping guest boots | builds for this spec write a separate output dir (`ZE_KERNEL_TEST_OUTPUT_DIR`), never `tmp/kernel/build` while another session uses it |
| R-4 | DES and NULL are weak; building them in widens what an operator can configure | none | they are already accepted by Ze's config and installed by its dataplane; the kernel refusing them is a silent install failure, not a policy. Policy belongs in config validation, not in a missing kernel symbol |
| R-5 | the showcase's Docker host kernel differs from Ze's | showcase IPsec chapter fails | out of this spec; the showcase agent owns the Docker kernel probe (`plan/post-release/engineering/spec-terminal-demo-showcase.md` R-2) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a runtime kernel build refuses (visible at build), or an SA with one cipher fails to install (visible at runtime) |
| How is it reverted? | single commit revert of the fragment, manifest and floor |
| Who else touches this path? | `spec-appliance-ships-ze-kernel.md` (same fragments and cache), crash-capture (PSTORE), kernel-capability-gate |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Runtime floor and manifest declarations | → | required-symbol membership | `TestRuntimeFloorCarriesVPNSymbols` (`internal/appliance/kernelreq_test.go`); declaration consistency, not command execution |
| `ze appliance kernel --target runtime` | → | resolver enforcement over the emitted config | Native build/readback receipts below; `TestRuntimeKernelFloorRejectsMissing` (`internal/appliance/cmd_kernel_test.go`) separately exercises resolver refusal with a fake builder |
| XFRM algorithm tables | → | transform-to-symbol mapping, fragment and manifest correspondence | `TestXfrmTransformsHaveRequiredKernelSymbol` (`internal/component/ike/dataplane/xfrm_kernelsym_linux_test.go`); declaration consistency, not SA installation |
| booted Ze runtime kernel | → | IKE SA per transform + WireGuard link | `./le test qemu vpn-kernel-test`, using the existing docker-host guest and interop infrastructure |

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
| `TestXfrmTransformsHaveRequiredKernelSymbol` | `internal/component/ike/dataplane/xfrm_kernelsym_linux_test.go` | AC-3, reads `gokrazy/kernel/runtime.require` from the repo root | |
| existing `kernelconfig_pairing_test.go` | `internal/appliance/` | AC-1 lines are required, not unverified | |

### Boundary Tests (numeric inputs)
N-A: no numeric input.

### Functional Tests
| Test | Location | Validates |
|------|----------|-----------|
| `./le test qemu vpn-kernel-test` (five-transform matrix) | `internal/le/test/qemu/vpnproof.go`, `internal/le/test/deployment/vpnproof.go`, `internal/le/interoplab/ipsec/vpnproof.go` | AC-6 IPsec |
| WireGuard guest step in the same action | `internal/le/test/deployment/wireguardproof.go` | AC-6 WireGuard, handshake and received ping traffic, no module loading |

### Interop Tests (Scope: protocol)
N-A: no wire behavior changes; the kernel gains transforms Ze already negotiates. The IPsec interop suites keep their peers.

## Files to Modify
- `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require`
- `internal/appliance/kernelreq.go`, `internal/appliance/kernelconfig_pairing_test.go`
- `internal/component/ike/dataplane/xfrm_linux.go` (the NULL spelling note resolved by AC-6)
- `internal/le/test/qemu/vpnproof.go`, `internal/le/test/deployment/vpnproof.go`
- `internal/le/test/deployment/wireguardproof.go`, `internal/le/interoplab/ipsec/vpnproof.go`
- `docs/architecture/appliance/kernel-profiles.md`, `docs/guide/appliance.md`, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`

## Files to Create
- `internal/component/ike/dataplane/xfrm_kernelsym_linux_test.go`
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
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/release/delivery/spec-appliance-kernel-vpn-modules.md` at implementation |
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
| guest proof | `./le test qemu vpn-kernel-test` output on native arm64/HVF; native amd64/KVM is additional evidence |

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
| every VPN symbol `=y` | `=m` plus a module loader in Ze | required facilities must not depend on runtime module loading; the build enforces `=y` and the native proof disables loading |
| derive the cipher symbols from the XFRM name tables | hand list in `runtime.require` only | a cipher added to Ze's tables without its kernel symbol is the exact defect found here |
| build DES and NULL in | drop them from Ze's tables | Ze accepts them in config today; removing them is a scope change the owner has not asked for |

## AC-5 Evidence

The two tables below are historical build receipts from October 10–12.
Current amd64 build and packaged-image evidence is recorded later in this spec.
The current Mac run passed its emitted-config prerequisite, but its selected
cache/config/module-tree copies still need the read-only handover receipt.

### arm64 (built 2026-10-10, native Docker builder in colima's aarch64 VM, Mac)
- Cache entry: `~/.cache/ze/runtime-kernel/7.2.9-runtime-arm64-runtime-7c0d8f79-bf3ac1c9` (`vmlinuz`, `config`, `lib/modules/7.2.9-ze`, 1612 `.ko`; `kernel.version`: version=7.2.9, target=runtime, profile=runtime, arch=arm64, modules=yes, builder=docker). Log: `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/kernel-build-arm64.log`.
- `enforceKernelRequirements` (`runKernel`, `internal/appliance/cmd_kernel.go`) accepted the emitted config with the runtime floor; a shell walk of all 114 `runtime.require` symbols against the same `config` found none missing. `CONFIG_LOCALVERSION="-ze"`.
- First attempt refused before compile: `FATAL: kernel profile runtime: CONFIG_IP_NF_FILTER did not resolve to =y` (`kernel-build-arm64-try1-iptables-legacy.log`). Since Linux 6.17 the iptables filter, mangle, raw and nat tables depend on `IP_NF_IPTABLES_LEGACY` / `IP6_NF_IPTABLES_LEGACY`, which depend on `NETFILTER_XTABLES_LEGACY` (7.2.9 `net/ipv4/netfilter/Kconfig`, `net/ipv6/netfilter/Kconfig`, `net/netfilter/Kconfig`). The 7.2 entry `d1de5ab8` shows the old silence: `# CONFIG_NETFILTER_XTABLES_LEGACY is not set` and no `CONFIG_IP_NF_NAT`. Fixed by requesting and requiring the three parents in `runtime.config` / `runtime.require`.
- Emitted VPN symbol lines:

```
CONFIG_XFRM_USER=y
CONFIG_XFRM_INTERFACE=y
CONFIG_XFRM_MIGRATE=y
CONFIG_XFRM_STATISTICS=y
CONFIG_INET_AH=y
CONFIG_INET_ESP=y
CONFIG_INET6_AH=y
CONFIG_INET6_ESP=y
CONFIG_WIREGUARD=y
CONFIG_CRYPTO_NULL=y
CONFIG_CRYPTO_AUTHENC=y
CONFIG_CRYPTO_AES=y
CONFIG_CRYPTO_DES=y
CONFIG_CRYPTO_CBC=y
CONFIG_CRYPTO_CHACHA20POLY1305=y
CONFIG_CRYPTO_GCM=y
CONFIG_CRYPTO_SEQIV=y
CONFIG_CRYPTO_ECHAINIV=y
CONFIG_CRYPTO_HMAC=y
CONFIG_CRYPTO_SHA1=y
CONFIG_CRYPTO_SHA256=y
CONFIG_CRYPTO_SHA512=y
```

### amd64 (built 2026-10-11 00:24 +0100, native Docker builder, Linux host; read 2026-10-12 at HEAD c79501292b)
- Cache entry: `~/.cache/ze/runtime-kernel/7.2.9-runtime-amd64-runtime-docker-7c0d8f79-34cba457` (`vmlinuz`, `config`, `lib/modules/7.2.9-ze`, 2 `.ko`: `xt_LOG.ko`, `x86_pkg_temp_thermal.ko`; `kernel.version`: version=7.2.9, target=runtime, profile=runtime, arch=amd64, modules=yes, builder=docker). `CONFIG_LOCALVERSION="-ze"`.
- The key is HEAD's, read from its producer and not from the entry name. `kernelCacheVariantFor` (`internal/appliance/cache.go`) hashes the fragments and manifests `resolveKernelProfile` resolves plus every patch (`7c0d8f79`), and the builder sources with their Ze import closure (`34cba457`). With `gokrazy/` and `internal/appliance/` clean against HEAD, `go run -tags ze_setup ./cmd/ze appliance kernel --target runtime --arch amd64 --version 7.2.9 --builder docker --print-cache-dir` (through `./le job run`) printed exactly this entry's path.
- A shell walk of all 114 `runtime.require` symbols and all 18 `kernel.require` symbols against this `config` found every one `=y`.
- Emitted VPN symbol lines (identical to arm64):

```
CONFIG_XFRM_USER=y
CONFIG_XFRM_INTERFACE=y
CONFIG_XFRM_MIGRATE=y
CONFIG_XFRM_STATISTICS=y
CONFIG_INET_AH=y
CONFIG_INET_ESP=y
CONFIG_INET6_AH=y
CONFIG_INET6_ESP=y
CONFIG_WIREGUARD=y
CONFIG_CRYPTO_NULL=y
CONFIG_CRYPTO_AUTHENC=y
CONFIG_CRYPTO_AES=y
CONFIG_CRYPTO_DES=y
CONFIG_CRYPTO_CBC=y
CONFIG_CRYPTO_CHACHA20POLY1305=y
CONFIG_CRYPTO_GCM=y
CONFIG_CRYPTO_SEQIV=y
CONFIG_CRYPTO_ECHAINIV=y
CONFIG_CRYPTO_HMAC=y
CONFIG_CRYPTO_SHA1=y
CONFIG_CRYPTO_SHA256=y
CONFIG_CRYPTO_SHA512=y
```

## AC-6 Implementation and Native Proof

Native amd64/KVM traffic passed on 2026-10-27. The repaired arm64/HVF run at `ceb83b7140` also passed, as recorded below. All four prescribed deliberate breaks and an unmutated Linux restoration are now proved. The remaining Mac request is read-only cache/config/module-tree evidence, not another traffic run.

- `internal/le/test/qemu/mobike.go` `runIPsecMOBIKEHere` boots through
  `NewRun` with `iproute2 iputils util-linux strongswan`. This is the Alpine
  guest path. A supplied Ze kernel does not establish default appliance image
  provenance.
- `internal/le/interoplab/ipsec/mobike_netns_linux.go` `RunMOBIKENetns`
  enumerates only the two movement scenarios. Its namespace/process adapter
  can be reused, but it does not expose the five-transform matrix.
- NULL already has a source-backed name in `xfrmEncNames`:
  `ecb(cipher_null)`. The existing `checkESPIntegrityOnlyNull` selects NULL,
  reads the negotiated Child SA, waits for kernel SAs, and tests traffic.
  This audit neither changes that spelling nor treats the existing checker
  as a passed default-image guest proof.
- `plan/release/validation/spec-wireguard-runtime-proof.md` is a skeleton,
  not an implemented guest ping lab. The VPP interface deployment scenario
  reads an interface, not Linux WireGuard tunnel traffic.
- The phase brief initially required the default application image for AC-6.
  The main thread corrected that interpretation: AC-6 may use Ze's docker-host
  image (Ze kernel and gokrazy base); AC-7 separately owns the default
  `ze appliance build` image's packaged config. No product shell RPC or
  default-image guest execution adapter is needed.
- `./le test qemu vpn-kernel-test` uses the existing `docker-lab` path,
  with no kernel or image override. The deployment action runs the existing
  `psk-site-to-site` and `esp-integrity-only-null` scenarios plus
  `esp-aes-cbc-sha256`, `esp-chacha20poly1305`, and `esp-3des-sha1`.
  Their `checkESPKernel` checker reads negotiated encryption and installed
  transforms and uses the existing four-direction ESP traffic oracle.
- Its WireGuard step creates two isolated Linux kernel peers, requires a
  successful tunnel ping, checks each peer's handshake and received bytes,
  and checks `/sbin/modprobe` inside the Docker host's root. The QEMU caller
  makes that host Ze's guest. This does not claim WireGuard config-commit
  coverage from the separate runtime-proof spec.
- Initial audit found that `buildEncryptionRegistry` admitted AES variants,
  `espOnlyEncryptionRegistry` admitted only NULL, and the integrity/PRF
  registries admitted SHA2 only. `parseESPProposal` therefore refused two
  combinations the AC had assumed were already accepted. This was a product
  prerequisite, not permission to substitute manually installed SAs.
- Owner decision, 2026-10-26: **"Implement both ESP combinations"**.
  This one-off approval adds ChaCha20-Poly1305 and 3DES/SHA1 for ESP; it does
  not expand IKE control-channel algorithms. The main thread assigned the
  product changes separately. The harness includes all five negotiated
  scenarios and does not skip unsupported combinations.
- The main thread approved preparation only inside the disposable proof
  guest: disable future module loading, refuse preloaded modules, remove
  `/sbin/modprobe`, and require emitted config symbols to be built in.
  `vpnGuestPreparation` runs before services through the existing cidata
  boot script. Normal `docker-lab` behavior and the shipped image are
  unchanged. `vpnKernelConfig` reads the resolver's emitted config and both
  manifests; the WireGuard checker rechecks the guest's loader state.
- The native amd64/KVM run below cannot replace the arm64/HVF condition.
  The initial Linux phase had no native arm64 runner; the later Mac result is below.

### Wrong Assumptions

| Date | Assumption | Producer | Consequence |
|------|------------|----------|-------------|
| 2026-10-25 | A-5: the existing MOBIKE action already selects arbitrary ESP transforms in the required image | `runIPsecMOBIKEHere`, `RunMOBIKENetns` | Use Ze's docker-host guest and the existing interop scenarios. The remaining two transforms need product support, not another guest base. |

### Phase 5 Verification

`./le --name vpn-proof test qemu vpn-kernel-test timeout 30m` passed on native
amd64/KVM. The first two arm64/HVF attempts stopped during preparation; the
second confirmed a loaded `pwm_fan` module. After the built-in repair, the
native arm64/HVF run at `ceb83b7140` passed preparation and all traffic cases.

The focused CBC transform, WireGuard receipt, built-in floor, guest preparation
and scenario-input tests passed with race detection. The separate
`TestVPNCleanupFailureRefusesPassingTraffic` also passed. These results are
recorded with the harness in `22f782e1c2`; they were not rerun for this update.
- Discriminating breaks: replace the CBC cipher with GCM in one peer;
  replace SHA256 with SHA1 in installed-state observations; remove the
  WireGuard receiving peer's allowed IP or give it the wrong public key;
  make `/sbin/modprobe` exist in a disposable guest image. None may pass
  the corresponding live assertion. Completed cases are recorded below.

### Live discrimination, native amd64/KVM, 2026-10-28

The one-peer CBC-to-GCM break passed its discrimination check. A private Go
overlay changed only the CBC scenario's strongSwan configuration mount to a
fixture whose ESP proposal was `aes256gcm16`; Ze still proposed AES-CBC/SHA256.
IKE established, the peer returned `NO_PROPOSAL_CHOSEN`, and the unchanged
checker failed waiting for a Child SA. This exercises the Child-SA prerequisite,
not the later negotiated-cipher, installed-transform or traffic assertions.
Every other ESP case and WireGuard passed.

The outer receipt records KVM, config check 0, expected lab exit 1, no leftovers
and a stopped VM. Evidence:
`tmp/session/2026-10-22-644a4e78-0122-4eeb-8642-eb50638a1641/scratch/job-vpn-break-cbc-gcm-30adb7c9.log`;
the private fixture, overlay and preserved console are under
`scratch/vpn-esp-breaks/cbc-gcm/` in that session. Independent review confirmed
this is the intended proposal-mismatch rejection, not a setup failure.

The SHA256-to-SHA1 observation break also passed its targeted discrimination
check. The CBC Child SA established and reached the installed-transform
assertion. A private overlay preserved the real directed XFRM observation
(SHA256 integrity), then substituted the SHA1 token in the returned observation.
The unchanged oracle rejected it: `ze: installed ESP state lacks hmac(sha256)`.
This proves rejection of the altered observation, not installation or
negotiation of SHA1. The first rejected direction ends that scenario.

Evidence is `scratch/job-vpn-break-sha256-ssh-trace-c6296b20.log` in the same
session. The private original observation's SHA-256 is
`977f62b6441f200188cd4915faec1ff3213f23fe70c34dc3fadc4c12d86748ad`;
raw XFRM keys remain private. PSK and ChaCha passed, but later 3DES, NULL and
WireGuard did not run because a concurrent DNS edit temporarily referenced
undefined `RejectFunc`. Thus the aggregate lab exit 1 is not solely the
mutation's verdict. The targeted rejection is established independently of
those later setup failures; KVM, the guest kernel probe, no leftovers and a
stopped VM are recorded. Independent review accepted this narrow claim.

Earlier SHA attempts stopped at an incomplete shared import edit and then at
SSH forwarding, before any lab. The instrumented run recorded client
authentication and creation of the local Unix listener without changing the
15-second deadline; it did not establish the earlier failure's cause or a fix.

The WireGuard allowed-IP break passed its targeted discrimination check.
After normal endpoint setup, a private overlay removed the receiving peer's
allowed IP and read back the same peer identity with no allowed IPs.
`VPN-WG-SETUP-APPLIED` precedes the unchanged tunnel-ping failure: three
packets transmitted, zero received. This proves the ping assertion rejects
that configuration; the later handshake/transfer receipt checks were not
reached, and this run does not establish whether a handshake occurred.

Evidence is `scratch/job-vpn-break-wg-allowedip-667acf82.log` in the same session.
PSK, ChaCha, 3DES and NULL passed. CBC did not run because a concurrent DHCP
edit temporarily lacked `neighborEntry`, so the aggregate exit 1 is not solely
the mutation's verdict. KVM, the guest kernel probe, no leftovers and a stopped
VM are recorded. Independent review accepted the targeted allowed-IP case;
the spec's alternative wrong-key mutation is not additionally required.

The modprobe-present break passed its discrimination check. The original
preparation first disabled loading, verified an empty module list and verified
helper absence. Only then did the private intervention create `/sbin/modprobe`;
the console records `VPN-MODPROBE-SETUP-APPLIED`. All five ESP cases passed,
then the unchanged WireGuard guest-root guard rejected the helper path before
interface setup or traffic. This proves helper-path detection, not an attempted
module load or a WireGuard traffic failure.

Evidence is `scratch/job-vpn-break-modprobe-present-1a52e208.log`, with
`scratch/vpn-wg-breaks/modprobe-present/console.log` lines 515–531, in the same
session. KVM, guest kernel probe pass, expected lab exit 1, no leftovers and a
stopped VM are recorded. Independent review accepted this fourth negative.

### Unmutated native restoration, 2026-10-28

After all four interventions, the normal native command ran with `GOFLAGS`
unset and without the SSH tracing wrapper:
`./le --name vpn-discrimination test qemu vpn-kernel-test timeout 30m`.
All five ESP cases and WireGuard passed on image `6fff09d78be893ac`.
The outer report records KVM, check-exit 0, lab-exit 0, no interruption, no
leftovers and a stopped VM.

Evidence:
`tmp/session/2026-10-22-644a4e78-0122-4eeb-8642-eb50638a1641/scratch/job-vpn-restored-native-positive-fe3e7ddf.log`;
the console is preserved as `scratch/vpn-restored-native-positive-console.log`
in that session. Independent review confirmed that the four targeted negatives
and this restored positive complete the prescribed discrimination requirement.
The negative assertions retain their narrow meanings above; this is not proof
that every later assertion independently rejects every possible bad receipt.

### Native amd64 result, 2026-10-27

The actual `test qemu vpn-kernel-test` entry point passed all five negotiated
ESP scenarios and the Linux WireGuard traffic check. The outer report named
KVM, check and lab exit zero, no leftover resources, and a stopped VM.

| Evidence | Observed result |
|----------|-----------------|
| `psk-site-to-site`, `esp-aes-cbc-sha256`, `esp-chacha20poly1305`, `esp-3des-sha1`, `esp-integrity-only-null` | Each passed installed ESP-state and bidirectional traffic checks; CBC, ChaCha and 3DES additionally assert the installed transform tokens |
| WireGuard | Tunnel traffic, matching-peer handshake and received bytes passed with module loading disabled, no loaded modules and no `/sbin/modprobe` |
| Guest provenance | Ze kernel `7.2.9-ze`, cached amd64 image `c8263f4fef5e9c76`, runtime kernel key `7.2.9-runtime-amd64-runtime-docker-36272e92-34cba457` |
| Initial failure and correction | NULL established its Child SA but its proposal log was buffered. `8544ee50de` selects stderr and `flush_line`; assertions, proposals and deadlines are unchanged |
| Product prerequisites | `ba03d7c2c7` lands ESP transform and MTU changes with scoped race and RFC5282 evidence |
| Reproducible harness | `22f782e1c2`; run the native command above |

Main read the complete successful outer report in
`tmp/session/2026-10-22-644a4e78-0122-4eeb-8642-eb50638a1641/scratch/vpn-native-guest-proof-green-20261027.log`.
The failed aggregate took 623.02 seconds; its corrected aggregate took 451.97
seconds. These are run durations, not a performance comparison.

This proves neither arm64/HVF nor the separate product WireGuard configuration
workflow. It does not claim external both-role rekey coverage. The later default
appliance image readback is recorded by `34eede6b0c` in the parent appliance
spec; it is separate from this Docker-host traffic proof.

### Native arm64 preparation failure, 2026-10-27

The Mac ran handover 27 at `7aa2f4849e`. Its native arm64 kernel 7.2.9 and
docker-host image built cold, with image cache key `a6574e77e607b55b`. The
emitted-config prerequisite completed and the guest booted under HVF with SSH.
Preparation then exited with only `Lima boot.sh: exit status 1`; Docker did not
answer within the existing 120-second bound. No ESP or WireGuard case ran.
The VM stopped without interruption. The outer zero exit fields do not turn
unreached checks or traffic into passing evidence.

The retained input is `tmp/mac-vpn-proof-2026-10-27/`: `FINDINGS.md`,
`vpn-kernel.log`, `docker-lab-console-2223.log`, `image-build.log` and
`kernel-config-arm64`. Linux traced the script through `vpnGuestPreparation`,
`dockerLabGuest.cidataFiles` and `serveForever`. The pinned gokrazy
`loadModules` attempts `pwm-fan.ko` before that preparation, and the Mac built
that module. At this first-run stage, successful insertion remained unproved
without the guest module list; built-in VPN requirements alone do not exclude
unrelated loaded modules.

The bounded repair preserves every module guard, traces only the fixed
preparation block and checks both procfs reads before testing their values.
The new `TestVPNPreparationRefusesUnknownModuleState` first exposed a silent
loaded-module refusal and acceptance of a failed module-list read. A direct
Linux shell smoke then passed empty, loaded and unreadable module-state cases,
including the boundary that stops tracing before the next phase. This does not
establish the original Mac cause or an arm64 traffic pass.

The post-fix package run stopped at unrelated unfinished
`internal/plugins/dnsresolver` references, including `flightTable`, `limiter`
and `resolverStats`; no post-fix package pass is claimed. The exact logs and
direct-shell receipts are recorded in this spec's session state. Handover 27
then requested one bounded diagnostic Mac rerun, with no timeout increase,
module exception or manual cache override.

### Native arm64 diagnostic rerun, 2026-10-27

The second Mac run at `835d147952` includes the diagnostic guard repair
`c135dceac6`. Evidence is retained in `tmp/mac-vpn-proof-2026-10-27-rerun/`
(`host.txt`, `commit.txt`, `image-build.log`, `vpn-kernel.log` and
`docker-lab-console-2223.log`), without replacing the first-run evidence.
It reused image `a6574e77e607b55b`, passed the emitted-config prerequisite and
booted under HVF. The console recorded `modules_disabled=1`, followed by
`loaded_modules='pwm_fan 20480 0 - Live 0xffff80007b830000'`.
The strict `test -z` then failed. The modprobe guards, Docker readiness and
all five ESP cases and WireGuard traffic were not reached. Cleanup completed
and the VM stopped. Outer zero exit fields are not evidence for those
unreached steps.

This confirms the loaded-module blocker. The pinned gokrazy startup path
calls `loadModules` before services; that function opens `pwm-fan.ko` and
calls `FinitModule`. The console does not identify the actual syscall caller.
The original arm64 config has `CONFIG_SENSORS_PWM_FAN=m`.

### Built-in fan repair: implementation and local checks

The repair preserves fan functionality without changing the guest guards:
`runtime.config` requests `CONFIG_HWMON=y`, `CONFIG_PWM=y` and
`CONFIG_SENSORS_PWM_FAN=y`, and `runtime.require` enforces all three.
The independent compiled runtime floor also requires `CONFIG_SENSORS_PWM_FAN`.
The regression contract accepts the built-in driver and refuses modular,
disabled or absent fan configurations even when the supplied manifest omits
the fan requirement. Eight focused enforcement, pairing and cache tests passed.
Independent review then found that the existing PPPoE rejection fixture omitted
other floor entries too. Its observed failure named the newly required fan
instead of PPPoE. Deriving the fixture from the complete runtime config and
removing only PPPoE preserved its original assertion; the affected
`internal/appliance` package then passed, including that entry-point test.

A bounded native amd64 smoke checked the pinned Linux 7.2.9 tarball digest,
applied the tracked patches, merged both current fragments and ran
`olddefconfig`. All 156 distinct symbols in the two manifests resolved to `=y`,
including HWMON, PWM and SENSORS_PWM_FAN, with MODULES retained and neither OF
nor COMPILE_TEST enabled. The fan object compiled with a built-in initcall.
The receipt and emitted config are in
`tmp/session/2026-10-22-644a4e78-0122-4eeb-8642-eb50638a1641/scratch/builtin-fan-native-amd64/`.
This was not a full kernel link, installed module tree, image build, boot or
arm64 proof. The disposable container was removed.

The smoke's initial CPU-quota refusal and inaccessible cache-symlink mount
were invocation setup failures, not kernel results. The successful invocation
removed only the added quota and mounted the resolved source tar read-only;
compilation remained limited to two jobs. Both independent source and proof
reviews are clear after the fixture correction. Exact test and smoke logs are
retained in this spec's session state. No broad verifier was rerun.

Handover 27 requested one native arm64 image build and HVF proof through the
existing commands, using normal cache resolution rather than a forced cold
build, cache purge or old-key override. The result follows.

### Native arm64 success after the fan repair, 2026-10-27

The owner returned one successful run at
`ceb83b7140c088445a1f7edcdfa682186201e3f2`, including `6e8ff49248`.
Linux read the transferred host, revision, build, proof and console files.
The commands were handover 27's named, admitted
`setup docker host build arch arm64` and
`test qemu vpn-kernel-test timeout 30m`. No passing command was rerun.

| Evidence | Observed result |
|----------|-----------------|
| Native host and image | `Darwin arm64`; cold build, `built true`, image key `4c4d23d6ca4234aa` |
| Native guest | `accelerator hvf`, `check-exit 0`, `interrupted false` |
| Fan and preparation | Build log shows non-modular `CC drivers/hwmon/pwm-fan.o`; console reports missing `pwm-fan.ko`, continues, then reads `modules_disabled=1` and an empty loaded-module list; both modprobe absence guards pass |
| ESP | `psk-site-to-site`, `esp-aes-cbc-sha256`, `esp-chacha20poly1305`, `esp-3des-sha1`, `esp-integrity-only-null` each pass |
| WireGuard | `wireguard-kernel-ping` passes |
| Cleanup | `lab-exit 0`, `left-behind none`, `VM stopped` |

The WireGuard result is not a ping-only claim. In
`internal/le/test/deployment/wireguardproof.go`, `checkWireGuardProof` requires
a successful tunnel ping, then calls `requireWireGuardReceipt` for each peer.
The latter rejects a different peer key, zero handshake timestamp or zero
received bytes. Both peers must pass. Linux read these producers and found
no change from the recorded Mac commit in the WireGuard, inner aggregate and
outer QEMU proof files.

The same outer source calls `vpnKernelConfig` before starting the guest.
`requireVPNBuiltins` refuses any manifest symbol not present as `=y` in the
resolver's emitted config. This is configuration-check evidence, not a pasted
copy of that emitted config or its installed module manifest.

The transferred files are retained in `tmp/mac-vpn-proof-2026-10-27-pwmfan/`.
Their SHA-256 receipts are:

| File | SHA-256 |
|------|---------|
| `host.txt` | `16b8cd028eb757aab907ab404e1656e01783846457156eec99372a91fc1b104d` |
| `commit.txt` | `9b15c3ac1c0ba8783238fff88cb3de4f85f73653d9754bc7773ff3761b6b6589` |
| `image-build.log` | `73b7e7bd458b14da4704915aedd8cf7fc75869b33c01f5a18bbf8ae32b0f10bb` |
| `vpn-kernel.log` | `81dd190da0e7b55878f07a585c41963365b02b9a8375634d1ce3c58169a9dbd5` |
| `docker-lab-console-2223.log` | `5d8d04d70d27ff14d8998494d6ba6bcc80b79f50bba38b3fc41c47762ca67c4b` |

The explicit emitted-config and installed `modules.builtin` readback requested
by handover 27 was not among these five files. Preserve the Mac cache for that
read-only reconciliation; no new boot or traffic run is needed to retrieve it.
The earlier two failure receipts and image remain preserved. Linux has since
completed the four deliberate breaks, unmutated restoration and default-image
readback recorded above and below. The missing Mac cache/config/module-tree
copies and closure gates still prevent closure; none of these kernel proofs
proves the separate product WireGuard configuration-commit workflow.

## Current native amd64 kernel and AC-7 default-image evidence

On 2026-10-28, the native runtime build through
`ze appliance kernel --target runtime --arch amd64 --builder docker --jobs 4`
passed and published
`7.2.9-runtime-amd64-runtime-docker-43d6bdb1-34cba457`.
Its emitted config contains all 156 symbols in `kernel.require` and
`runtime.require` as `=y`, including HWMON, PWM and SENSORS_PWM_FAN.
The installed `modules.builtin` names the fan; the only loadable module in
that tree is `x86_pkg_temp_thermal.ko`.

Current AC-2/AC-3 and fan lines read from that emitted config:

```text
CONFIG_XFRM_USER=y
CONFIG_XFRM_INTERFACE=y
CONFIG_INET_AH=y
CONFIG_INET_ESP=y
CONFIG_INET6_AH=y
CONFIG_INET6_ESP=y
CONFIG_WIREGUARD=y
CONFIG_CRYPTO_CBC=y
CONFIG_CRYPTO_DES=y
CONFIG_CRYPTO_NULL=y
CONFIG_CRYPTO_GCM=y
CONFIG_CRYPTO_CHACHA20POLY1305=y
CONFIG_CRYPTO_SHA1=y
CONFIG_CRYPTO_SHA256=y
CONFIG_CRYPTO_SHA512=y
CONFIG_HWMON=y
CONFIG_PWM=y
CONFIG_SENSORS_PWM_FAN=y
```

The first attempt completed kernel compilation but failed before cache
publication when the ownership-repair invocation exceeded its two-minute
deadline, leaving a created but unstarted container. Resuming that exact owned
container passed. The subsequent bounded native build used the unchanged
deadline and passed. The first outputs remain preserved separately, with an
explicit stale-provenance warning; they were not manually imported into the
kernel cache.

The normal `ze appliance build t2` entry point then built the default amd64
image from that warm kernel, with no kernel/image override. Readback extracted
`vmlinuz` from the actual GPT EFI partition, then read its x86-header-defined
zstd payload and embedded kernel config. Both kernel bytes and config bytes
match the selected cache. All 156 required symbols are built in; none is
missing. This satisfies the packaged-config condition of AC-7, not a boot,
L2TP/PPP or WireGuard configuration-commit proof.

| Artifact | SHA-256 |
|----------|---------|
| Default `t2/ze-20261028-005448.img` | `761f0cccb37042c42c1da4d41ea139c68811ef7605cf09f5934e53ddc6ab4b17` |
| Packaged and cached kernel | `c7f8358004c5e5efa8bd7d4ace48cdfa09f101b98c2b3ea78e78eab2eb64cf67` |
| Embedded and cached config | `7647aa85ab64e089250d61a2873cb03f732a20ba053ee36de3efba8521a11399` |

Receipts are under session
`2026-10-22-644a4e78-0122-4eeb-8642-eb50638a1641`:
`scratch/job-vpn-kernel-four-jobs-654b7451.log`,
`scratch/job-vpn-default-image-032c88df.log` and
`scratch/vpn-default-readback/receipt.json`. The native Docker-host prerequisite
also built successfully from this kernel, as image `6fff09d78be893ac`;
that image is separate from the default appliance image above.


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
- [ ] Scoped `./le verify changed` evidence recorded after the closing commits; full verification remains Thomas's release check, not an agent closure gate
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
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only
