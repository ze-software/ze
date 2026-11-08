# Spec: wireguard-peer-reset

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Priority | P3 |
| Release reason | An operational command Ze lacks; WireGuard config apply is unaffected |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A WireGuard peer learns the remote endpoint from every authenticated packet,
so after a roam, a NAT rebinding or a remote that moved and came back, the
kernel can keep sending to an address that no longer answers. The handshake
state can also be stale after a remote's key material or restart. Today the
only way to put a peer back on its configured endpoint is a config change and
commit, and nothing drops a peer's session on demand.

The work adds an operational command that resets one peer, or every peer of
one WireGuard interface:

- the peer's endpoint returns to the configured `endpoint ip` and `port`;
- the peer's session is dropped, so the next packet starts a new handshake;
- a peer with no configured endpoint has its session dropped and its learned
  endpoint cleared, so it waits for the remote to start the handshake.

The command reaches every surface (CLI, SSH, MCP, REST, gRPC, web) through the
command registry, as every operator command does (`ai/META-DECISIONS.md`).

## VyOS reference

VyOS checkout `~/Code/github.com/vyos/vyos-1x`: `reset wireguard interface
<if> [peer <name>]`, implemented in `src/op_mode/reset_wireguard.py` and
`python/vyos/ifconfig/wireguard.py` `WireGuardOperational.reset_peer`, which
runs `wg set <if> peer <key> endpoint <configured address>:<port>` for each
selected peer. Ticket T9538, commit `ab0b2975d` (`op-mode: T9538: fix interface
check in reset wireguard`): the existence check read the wrong argument name,
so it only tested whether ANY WireGuard interface existed. An unconfigured name
slipped through and crashed, or the error named the interface as "None".

| VyOS lesson | Ze acceptance criterion |
|-------------|-------------------------|
| T9538: an unknown interface was not refused by name | AC-4: an unknown name, or a name that is not a WireGuard interface, is refused with that name in the message |
| VyOS silently skips a peer with no address and prints a line | AC-3: Ze still resets that peer's session and says so in the answer |
| VyOS skips a disabled peer | AC-5: a disabled peer is refused by name when named, and reported as skipped under `all` |

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` - WireGuard interface types
- [ ] `docs/architecture/cli/command-namespacing.md` - verb-first grammar
  → Constraint: the command extends the `clear interface` tree, which already
  refuses a bare form with no target (`clear interface counters`)

**Key insights:**
- The reset is a runtime action on kernel state; it never edits the config.
- `ConfigureWireguardDevice` already writes the configured endpoint on every
  apply (`ReplacePeers: true`), so the reset is a one-peer apply plus a
  session drop.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/wireguard.go` - `WireguardSpec` and
  `WireguardPeerSpec`: name, keys, `EndpointIP`, `EndpointPort`, allowed IPs,
  keepalive, `Disable`.
- [ ] `internal/component/iface/backend.go` - `Backend` declares
  `CreateWireguardDevice`, `ConfigureWireguardDevice` (whole spec,
  `ReplacePeers`) and `GetWireguardDevice`. No per-peer action.
- [ ] `internal/plugins/iface/netlink/wireguard_linux.go` -
  `ConfigureWireguardDevice` applies the whole spec through wgctrl;
  `GetWireguardDevice` reads kernel state back.
- [ ] `internal/plugins/iface/vpp/wireguard.go` - the VPP backend programs
  peers through the wireguard plugin API.
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `list wireguard`,
  `list peer` with `endpoint ip` (numeric only) and `port`, `disable`.
- [ ] `internal/component/iface/yang/ze-iface-interface-cmd.yang` - the
  `clear interface` tree holds only `counters`. No WireGuard operational command.

**Behavior to preserve:**
- Config apply of WireGuard interfaces and peers.
- `clear interface ... counters`.

**Behavior to change:**
- None; the command is new.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `clear interface` command tree | `internal/component/iface/yang/ze-iface-interface-cmd.yang` `container clear` | Resets counters for one interface or all | extends: a WireGuard peer reset under the same tree, keeping its "no bare form" rule |
| WireGuard backend apply | `internal/plugins/iface/netlink/wireguard_linux.go` `ConfigureWireguardDevice` | Applies the configured peer set | extends: a one-peer form that sets the configured endpoint and drops the session |
| Interface backend interface | `internal/component/iface/backend.go` `Backend` | The seam both backends implement | extends: a reset method; the VPP backend implements it or refuses it with its reason (exact or refuse, `ai/doors/forwarding.md`) |
| Command registry | `docs/architecture/api/commands.md` | One declaration reaches every surface | extends: the YANG node and handler register like `clear-interface-counters` |
| Interface name resolution | `docs/architecture/iface/logical-name-resolution.md` | Turns a configured name into the device | extends: the command resolves through it, never straight to the kernel |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Command: `clear interface name <wg> wireguard peer <peer>` and `... wireguard peer all` (grammar settled at design).

### Transformation Path
1. The handler checks the name is a configured WireGuard interface, and the peer exists and is enabled.
2. It reads the configured peer from the running config.
3. The backend drops the peer's session and sets the configured endpoint.
4. The answer lists each peer reset, skipped or refused, with the endpoint before and after.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Command dispatcher to iface backend | registered RPC | No |

### Changed-Wire Census

**Search evidence:** no protocol wire changes; WireGuard handshakes are the
kernel's. The new command answer is a new JSON shape.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| command answer | writer | the new handler | none yet | new |

### Integration Points
- to be designed

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to be designed |
| No unintended coupling (components stay isolated) | No | to be designed |
| No duplicated functionality (extends existing, does not recreate) | No | to be designed |
| Zero-copy preserved where applicable (refs, not copies) | No | to be designed |
| Registration over hardcoding, outbound | No | to be designed |
| Registration over hardcoding, inbound | No | to be designed |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Removing and re-adding a peer through wgctrl drops its session keys and forces a new handshake | to be read in the kernel `drivers/net/wireguard` source at design | the reset moves the endpoint only | integration test that observes a new handshake time | unvalidated |
| A-2 | The VPP wireguard API can remove and re-add a peer | `internal/plugins/iface/vpp/wireguard.go` | VPP refuses the command by name | design read | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Traffic to the peer drops for one handshake | a short loss in the test | documented in the command help |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | one tunnel stays down until the next commit |
| How is it reverted? | single commit revert |
| Who else touches this path? | WireGuard config apply in both backends |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| the reset command on a roamed peer | → | backend reset | `TestWireguardPeerResetRestoresEndpoint` (QEMU integration) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | a peer whose kernel endpoint roamed away from the configured one | after the reset the kernel endpoint is the configured `ip:port`, and a new handshake completes | none |
| AC-2 | `all` on an interface with three enabled peers | each is reset; the answer lists all three | none |
| AC-3 | a peer with no configured endpoint | its session is dropped and its learned endpoint cleared; the answer says it waits for the remote | none |
| AC-4 | an unknown interface name, or one that is not WireGuard | refused; the message names the interface and the command that lists WireGuard interfaces | none |
| AC-5 | an unknown or disabled peer named | refused, naming the peer; under `all`, a disabled peer is reported as skipped | none |
| AC-6 | the bare form with no peer and no `all` | refused, as `clear interface counters` is | none |
| AC-7 | the same command over MCP or REST | the same answer as the CLI | none |
| AC-8 | the VPP backend | works, or refuses with the reason and no partial change | none |
| AC-9 | the config after a reset | unchanged; `show config` is identical | none |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWireguardResetRefusals` | `internal/component/iface/` | AC-4 to AC-6 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `wireguard-peer-reset` | `test/plugin/` | the operator resets a peer and reads the answer | |

## Files to Modify
- `internal/component/iface/yang/ze-iface-interface-cmd.yang` - the command node
- `internal/component/iface/backend.go` - the reset method
- `internal/plugins/iface/netlink/wireguard_linux.go` - netlink implementation
- `internal/plugins/iface/vpp/wireguard.go` - VPP implementation or refusal
- `docs/guide/command-reference.md` - the command

## Files to Create
- the handler and its tests, placed at design

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| to be answered at design | | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| to be answered at design | | | |

## Implementation Steps

1. **Phase: Wiring** -- to be designed

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Targeted tests pass: `./le test integration iface`, `./le test functional plugin`, `./le test qemu all` for the kernel proof

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
