# Spec: dhcp-server-client-classes

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Client classes are an absent DHCP server capability; nothing Ze ships today depends on them |
| RFC gaps | - |
| Depends | `plan/post-release/features/spec-dhcp-pool-options.md` (a class that hands out options reuses its option syntax and encoder) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Clients that share one subnet often need different treatment: IP phones need a
provisioning URL, set-top boxes need another range, a subscriber line behind a
relay needs its own pool. Today Ze's DHCPv4 server cannot tell them apart. It
reads option 60 only to recognize a PXE client (`isPXEClient`), and it reads no
host name and no relay agent information at all.

The work adds named client classes to the DHCP server. A class matches a
request on one or more conditions:

| Condition | Source in the request | Match forms |
|-----------|-----------------------|-------------|
| host name | option 12 (RFC 2132 Section 3.14) | exact, or contains-anywhere |
| vendor class identifier | option 60 (RFC 2132 Section 9.13) | exact, or contains-anywhere |
| relay agent information | option 82 sub-options circuit-id and remote-id (RFC 3046 Section 2.0) | exact |

A range can be limited to the clients of a class, and a class can carry its own
options, independent of the range the lease comes from.

## VyOS reference

VyOS checkout `~/Code/github.com/vyos/vyos-1x`, ticket T9267 (`git log
--grep=T9267`). Its fixes are the pitfalls this spec turns into acceptance
criteria:

| Commit | VyOS defect | Ze acceptance criterion |
|--------|-------------|-------------------------|
| `2e5ea7a83` | the feature: host name and vendor class matching, class options | AC-1, AC-2, AC-6 |
| `46e06d8fd` | "substring" compared a fixed slice at a fixed offset, an anchored prefix match, so a model string inside a longer host name never matched | AC-2: contains-anywhere, case-insensitive |
| `da0bea0d1` | a separate substring length leaf could only make the match wrong | the design has no length leaf |
| `d6c9aa14b` | a hex value with an odd digit count gave a zero-length compare that never matched | AC-8: a hex value is refused unless it is whole octets |
| `579fc38ce` | a class with no match condition, or a host name node with no value, passed verify and broke the server config at commit | AC-4 |
| `3dcb64996` | an empty relay agent information node (no circuit-id, no remote-id) passed the "has a condition" guard | AC-4: presence of a node is not a condition; a condition has a value |
| `349bf9632` | the "has a condition" check also ran on disabled classes, which the server never loads, so a saved config stopped committing and took the DHCP service down at boot | AC-5: a disabled class is not checked and not loaded |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/provisioning/dhcp-server.md` - the DHCP server design
  → Constraint: a segment chooses the handler; a handler never chooses itself (`segment.go`)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2131.md` and `rfc/short/rfc2132.md`
- RFC 3046 has no summary and none is written here: no release profile lists it
  (`ai/META-DECISIONS.md`, RFC validation scope). Its text is in
  `rfc/full/rfc3046.txt`.

RFC text read for this spec:

- RFC 2132 Section 9.13: "Servers not equipped to interpret the class-specific
  information sent by a client MUST ignore it (although it may be reported)."
- RFC 2132 Section 3.14: "This option specifies the name of the client."
- RFC 3046 Section 2.2: "DHCP servers claiming to support the Relay Agent
  Information option SHALL echo the entire contents of the Relay Agent
  Information option in all replies." Matching on option 82 is such a claim,
  so the echo is owed (AC-7). The same section: "Servers SHALL NOT place the
  echoed Relay Agent Information option in the overloaded sname or file
  fields."

**Key insights:**
- A class with no condition would match every client. VyOS hit that three ways;
  Ze refuses it at verify, on the values, not on the presence of a node.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/dhcpserver/handler.go` - `isPXEClient` reads option 60
  for the `PXEClient:` prefix only; `parseOptionBytes` returns an option's data.
  No reader for option 12 or option 82. `buildReply` copies no option 82.
- [ ] `internal/plugins/dhcpserver/segment.go` - `segment.discover` offers a
  static mapping, then the held binding, then the requested address, then each
  subnet's pool in order. Nothing filters a pool by the client.
- [ ] `internal/plugins/dhcpserver/config.go` - `subnetConfig` and
  `addressRange` carry no class reference.

**Behavior to preserve:**
- A config with no class allocates exactly as today.
- PXE detection and its options.

**Behavior to change:**
- A range limited to a class serves only clients the class matches.
- A matched class adds its options to the reply.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Option reader | `internal/plugins/dhcpserver/handler.go` `parseOptionBytes`, `findOption` | Finds one option's data in a request | extends: the class matcher reads options 12, 60 and 82 through it |
| PXE client detection | `handler.go` `isPXEClient`, `isIPXE` | A hard-coded vendor class test | does not apply as a change: PXE keeps its own test; a class is the operator's tool and does not replace it |
| Pool selection | `internal/plugins/dhcpserver/segment.go` `segment.discover`, `pool.allocate` | Walks the segment's subnets and allocates | extends: a range limited to a class is skipped for a client the class does not match |
| Arbitrary options | `plan/post-release/features/spec-dhcp-pool-options.md` | Planned subnet option syntax and encoder | extends: a class's options use the same syntax and encoder, so no second option model exists |
| Static mapping | `config.go` `staticMapping` | Pins one MAC to one address | does not apply: a class selects a pool and options, never one address |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `service dhcp-server client-class <name>` and a `client-class` reference on a range.
- Wire: a DHCPDISCOVER or DHCPREQUEST carrying options 12, 60 or 82.

### Transformation Path
1. Config parse builds the classes, skipping disabled ones; verify refuses a class without a condition.
2. On a request, the matcher evaluates every loaded class once, in config order.
3. Pool selection skips ranges whose class did not match.
4. `buildReply` appends the matched classes' options, and echoes option 82 last.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire to plugin | DHCP over UDP 67 | No |

### Changed-Wire Census

**Search evidence:** to be written at design. Replies gain option 82 (echo)
and class options; no existing option changes.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| DHCP reply options | writer | `handler.go` `buildReply` | read for this spec | add option 82 echo and class options |

### Integration Points
- `segment.discover` and the request paths that call `pool.allocate`.

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
| A-1 | Relays Ze meets (ISC dhcrelay, FRR dhcrelay, accel-ppp) put circuit-id and remote-id as RFC 3046 sub-options 1 and 2 | RFC 3046 Section 2.0 | a match on option 82 never fires | interop with a relay | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A client that matches a class whose ranges are full gets no address | silence on DISCOVER | the design decides whether the client falls back to unrestricted ranges, and logs the full class by name either way |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | clients get the wrong pool or options, or none |
| How is it reverted? | single commit revert |
| Who else touches this path? | `spec-dhcp-pool-options.md`, `spec-dhcp-server-renew-rebind-time.md`, `spec-ipoe-subscriber-sessions.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| class on vendor class id, range limited to it | → | the class matcher and `segment.discover` | `dhcp-client-class` (`test/plugin/`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | class `phones` with vendor class identifier exact `Yealink`, range `phones` limited to it; a client sends option 60 `Yealink` | the client is offered an address from range `phones` | none |
| AC-2 | class with host name contains `t42s`; a client sends option 12 `office-T42S-0011` | matched (contains-anywhere, case-insensitive) | none |
| AC-3 | a client that matches no class | it is never offered an address from a class-limited range; unrestricted ranges serve it as today | none |
| AC-4 | a class with no condition, a host name or vendor condition with no value, or a relay agent information condition with neither circuit-id nor remote-id | commit refused; the message names the class and the leaf to set | none |
| AC-5 | a disabled class with no condition | commit accepted; the class matches nothing and is not loaded | none |
| AC-6 | a matched class that carries options | the reply carries them, in the option encoding of `spec-dhcp-pool-options.md` | none |
| AC-7 | a relayed request with option 82 when any class matches on option 82 | every reply echoes the entire option 82 as the last option, never in sname or file (RFC 3046 Section 2.2) | none |
| AC-8 | a hex match value with an odd number of digits | refused at commit | none |
| AC-9 | a range that names a class that does not exist | refused at commit, naming the range and the class | none |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestClientClassMatch` | `internal/plugins/dhcpserver/` | AC-1 to AC-3 | |
| `TestClientClassVerify` | `internal/plugins/dhcpserver/config_test.go` | AC-4, AC-5, AC-8, AC-9 | |
| `TestRelayAgentInfoEcho` | `internal/plugins/dhcpserver/handler_test.go` | AC-7 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dhcp-client-class` | `test/plugin/` | phones get the phone pool; other clients the default pool | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `dhcp-relay-class` | `test/interop/scenarios/` | ISC or FRR DHCP relay inserting option 82 | AC-7 and an option 82 class match | |

## Files to Modify
- `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` - `client-class` list, range reference
- `internal/plugins/dhcpserver/config.go` - parse and verify
- `internal/plugins/dhcpserver/segment.go` - class-limited pool selection
- `internal/plugins/dhcpserver/handler.go` - option 12 and 82 readers, option 82 echo, class options

## Files to Create
- the functional test `dhcp-client-class` under `test/plugin/`

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
- [ ] Targeted tests pass: `./le test functional dhcp`, the `dhcpserver` package test, and the `dhcp-relay-class` scenario through `./le test integration`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
