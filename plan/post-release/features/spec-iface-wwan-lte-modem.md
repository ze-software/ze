# Spec: iface-wwan-lte-modem

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release: a cellular (LTE or 5G) uplink through a WWAN modem. No shipped behavior depends on it |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A CPE or an out-of-band router often has an LTE or 5G modem (M.2 or USB) as a
backup or primary uplink. An operator must be able to configure a WWAN
interface with an APN, credentials and IP type, have Ze bring the data
session up, keep it up across signal loss, take it down on request, and show
signal, registration and session state. VyOS has `interfaces wwan`; Ze has no
cellular support at all. VyOS's recent work (T9350, T9326, T9388, T7487)
lists the traps.

| # | Operator need |
|---|---------------|
| 1 | `wwan0` with APN, optional username and password, PIN, IP type ipv4, ipv6 or ipv4v6 |
| 2 | Addresses from the bearer (DHCP or static bearer config), IPv6 by SLAAC or DHCPv6 |
| 3 | Automatic re-dial after signal loss; an operator `disconnect` that stays disconnected until `connect` |
| 4 | `show interface wwan wwan0` with signal, operator, registration, bearer state, on QMI and MBIM modems alike |
| 5 | Several modems on one box, each bound to the right interface |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/iface/logical-name-resolution.md` - selectors
  → Constraint: a modem that reports no permanent MAC gets a random one per probe, so `mac match` cannot select it; the binding needs another key (A-3)
- [ ] `ai/doors/appliance.md` - the gokrazy appliance runs no extra daemons, and the kernel is Ze's
  → Constraint: ModemManager and its D-Bus are not on the appliance; the design must say what speaks to the modem (A-1)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - interface kinds are ethernet, dummy, veth, bridge, tunnel, wireguard, xfrm, pppoe-client, loopback; no wwan kind
- [ ] `internal/component/iface/backend.go` - no modem or bearer operation
- [ ] `internal/component/kernelcap/kernelcap.go` - a subsystem declares a kernel capability, a config predicate and a probe; `ze doctor` and the startup gate report a missing one

**Behavior to preserve:**
- Nothing existing changes.

**Behavior to change:**
- A WWAN interface kind with modem control, session supervision, and show and doctor surfaces.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| PPPoE client session supervision | `internal/component/iface/pppoe_client.go` `PPPoEClient.run` | A long-lived worker dials, supervises and redials one uplink session with configured timing | Extends the pattern: one worker per WWAN interface, inside the daemon (VyOS moved re-dial out of cron into its daemon for the same reason, T9350 `f148cc7c4`) |
| DHCP and DHCPv6 clients per interface | `internal/plugins/iface/dhcp` | Address an interface from a lease | Extends: the WWAN net device takes the same clients when the bearer says DHCP |
| Kernel capability declaration | `internal/component/kernelcap/kernelcap.go` | Doctor and startup refusal for missing kernel features | Extends: qmi_wwan, cdc_mbim, the WWAN subsystem and the USB serial drivers |
| Feature gates | `feature-gates.txt` | Compile a feature out | Extends: the modem code is a gated feature, since most builds carry no modem |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: a new `wwan` interface list (shape settled at design).
- Commands: `connect interface wwan <name>`, `disconnect interface wwan <name>`, `show interface wwan <name>`, served through the command registry on every surface.

### Transformation Path
1. Find the modem that owns the configured net device, by port ownership (A-3).
2. Wait until the modem is dialable (SIM ready, registered), not merely present.
3. Set the attach-time PDN (initial EPS bearer) IP type to the configured one, always, before connect.
4. Connect the data bearer; configure addresses from it (static bearer config or DHCP), IPv6 by SLAAC or DHCPv6.
5. Supervise: on bearer loss, re-dial with back-off unless the operator disconnected.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to modem | QMI, MBIM or AT over the control port (A-1) | No |
| iface to kernel | the modem's net device (wwanN, wwan0mbim0 ...) | No |

### Changed-Wire Census

**Search evidence:** a new wire (modem control protocol); census owed at design.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [finding] | [disposition] |

### Integration Points
- The interface apply, DHCP clients, the command registry, doctor.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Ze must speak to the modem itself (MBIM and QMI, AT as a fallback), because the appliance cannot run ModemManager | `ai/doors/appliance.md`: one static Go system, no extra daemons | A ModemManager-based design for distro builds only | Design decision with Thomas; any third-party Go module for MBIM or QMI is a new module and needs his approval before use (`ai/META-DECISIONS.md`) | unvalidated |
| A-2 | Ze's kernel carries `CONFIG_USB_NET_QMI_WWAN`, `CONFIG_USB_NET_CDC_MBIM`, `CONFIG_WWAN` and the MHI PCI driver for M.2 modems | not checked | Kernel config change needed | Read `gokrazy/kernel/runtime.config` | unvalidated |
| A-3 | The modem that owns a net device is found from the device's sysfs ancestry (the USB or PCI device both the control port and the net device hang off), never from the interface number | VyOS T7487 `a272bffcf`: interface number and modem index enumerate independently | The wrong modem is commanded on a multi-modem box | Integration test with two modems, or a sysfs fixture | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A modem that is listed but still probing fails the first dial and nothing re-dials | Interface up with no session | Wait for dialable state (AC-2) |
| R-2 | The re-dial loop undoes an operator disconnect | Session back within the loop period | Record the operator's intent (AC-6) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The cellular uplink, often the only out-of-band path |
| How is it reverted? | Remove the interface |
| Who else touches this path? | DHCP clients, routes from the bearer, the slot selector spec (modems have no permanent MAC) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `wwan0` with an APN committed | → | modem find, wait, PDN set, connect | a modem simulator test chosen at design (an MBIM and a QMI responder on a fake control port); hardware proof on a real modem |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | APN and ip-type ipv4v6 on a QMI modem, and again on an MBIM modem | A data session comes up, the net device has an IPv4 and an IPv6 address, a ping to the internet succeeds | None |
| AC-2 | The modem is present but still probing at commit | Ze waits until it is dialable, then dials; no failed first dial left without a re-dial (VyOS T9350 `79738ffce`) | None |
| AC-3 | ip-type ipv4 after a previous ipv6 attach | The attach-time PDN type is set to ipv4 before connect, and the connect succeeds (VyOS T9326 `5299606d0`) | None |
| AC-4 | ipv4v6 on a network that admits one combined PDN per APN | One dual-stack bearer, not a second IPv6-only request (VyOS T9326 `97d46c8fe`) | None |
| AC-5 | IPv6 by SLAAC only, no IPv6 address configured | An IPv6-capable bearer is requested (VyOS T9388 `22165e862`) | None |
| AC-6 | `disconnect interface wwan wwan0` | The session stays down across re-dial passes until `connect` or a commit that changes the interface (VyOS T9350 `112fb62b8`) | None |
| AC-7 | Signal lost, then back | Re-dial with back-off; the session returns without operator action | None |
| AC-8 | Two modems | Each interface commands the modem that owns its net device (VyOS T7487 `a272bffcf`) | None |
| AC-9 | `show interface wwan wwan0` on an MBIM modem | Signal, operator, registration and bearer state shown, through the protocol the modem speaks, with no QMI-only call (VyOS T9350 `9c88884a6`) | None |
| AC-10 | A malformed or truncated modem answer | Logged with the field and the modem, the worker keeps running (VyOS T7487 `1aa2cd1df`) | None |
| AC-11 | A boot with the config in place | Dialing runs inside the daemon after the config is applied, never from a job that can run before it (VyOS T9350 `f148cc7c4`) | None |
| AC-12 | Modem net device with no permanent MAC | Never recorded as a card identity; no "replaced hardware" report each boot (VyOS T3871 `015663a31`) | None |
| AC-13 | Kernel lacks the modem drivers | `ze doctor` names the missing kernel option | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Adds an LTE backup uplink | config, modem, bearer, addresses, route | simulator test named at design |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWWANModemByPortOwnership` | named at design | AC-8 over a sysfs fixture | |
| `TestWWANDisconnectIntentSurvivesRedial` | named at design | AC-6 | |

### Boundary Tests (numeric inputs)
N-A until the design names numeric leaves.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| named at design | `test/plugin/` | AC-1 with a simulator, AC-6 | |

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - the wwan kind (or the owning plugin's YANG, per design)
- `internal/component/kernelcap/kernelcap.go` registration from the owning package

## Files to Create
- The modem control package and its plugin, placed per `docs/architecture/module-tiers.md` at design.

## Implementation Steps

1. **Phase: Design decision** - A-1 with Thomas: who speaks to the modem, which module if any.
2. **Phase: Wiring** - kind, gate, failing test.
3. **Phase: Modem control** - find, wait, PDN, connect, show.
4. **Phase: Supervision** - re-dial, operator intent, idle-free supervision.

Proof: `./le test unit plugins`, `./le test functional plugin`, and a hardware run on a real modem.

## VyOS reference

VyOS checkout `~/Code/github.com/vyos/vyos-1x`, read 2026-11-08 at `611550d23`.

| Ticket / commit | Lesson for Ze |
|-----------------|---------------|
| T9350, `79738ffce` "wait for the modem to become dialable, not merely listed" | AC-2 |
| T9350, `9c88884a6` "query ModemManager instead of the QMI control port" | An MBIM modem has no QMI port; every show command failed after a timeout. AC-9 |
| T9350, `112fb62b8` "properly disconnect" | The re-dial loop undid an operator disconnect within 30 s. AC-6 |
| T9350, `f148cc7c4` "move session re-dial from cron into vyos-netlinkd" | A cron re-entering config mode before boot config was populated took the teardown branch and killed the uplink. AC-11 |
| T9326, `5299606d0`, `97d46c8fe` | Always set the attach-time PDN type; one combined dual-stack bearer on single-PDN networks. AC-3, AC-4 |
| T9388, `22165e862` | SLAAC-only IPv6 got an IPv4-only bearer. AC-5 |
| T7487, `a272bffcf`, `1aa2cd1df` | Modem found by port ownership, not interface number; malformed answers caught. AC-8, AC-10 |
| T3871, `015663a31` | A modem's random MAC read as hardware identity. AC-12 |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- SMS, GNSS and firmware update of the modem are not in this spec.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason): a real modem on a live network is the interop proof
