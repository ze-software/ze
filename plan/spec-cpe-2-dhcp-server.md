# Spec: cpe-2-dhcp-server

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 7/7 |
| Updated | 2026-05-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugins/ntp/` - reference plugin pattern (registration, config, lifecycle)
4. `internal/plugins/l2tppool/` - address pool management reference

## Task

Add a DHCP server plugin to Ze. Ze currently has DHCP **client** support (per-interface `dhcp { enabled true }` in ze-iface-conf.yang). CPE/home-router deployments need to serve DHCP leases to LAN clients: subnet configuration, address pools with start/stop ranges, static MAC-to-IP mappings, configurable lease times, and DHCP options (default-router, DNS servers, domain-name).

Modeled as a plugin (like ntp, static, sysctl) because it is an independent service with its own config tree and lifecycle.

**Motivation:** VyOS home.conf uses `service dhcp-server { shared-network-name LAN { subnet 192.168.1.0/24 { range ...; option ...; static-mapping ... } } }`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - plugin registration pattern, component isolation
  → Decision: Follow plugin skeleton: register.go with init(), config.go, schema/register.go
- [ ] `ai/patterns/plugin.md` - plugin creation pattern
  → Constraint: Plugin must register via init() in register.go
- [ ] `internal/plugins/ntp/register.go` - reference plugin registration and lifecycle
  → Decision: Same pattern: init() registers plugin name and run function
- [ ] `internal/plugins/l2tppool/pool.go` - address pool management (start/end range, allocation)
  → Decision: Reuse allocation patterns but not the pool code itself (different domain)

### RFC Summaries (MUST for protocol work)
- [ ] RFC 2131 - DHCP protocol (DISCOVER/OFFER/REQUEST/ACK/NAK/RELEASE state machine)
  → Constraint: Must support full 4-message exchange and handle RELEASE
- [ ] RFC 2132 - DHCP options (router option 3, DNS option 6, domain-name option 15, lease-time option 51)
  → Constraint: Minimum required options: subnet-mask, router, DNS, lease-time

**Key insights:**
- DHCP server listens on UDP port 67, responds on UDP port 68
- Lease state tracked in-memory with per-lease expiry timers
- Static mappings bind MAC address to fixed IP (bypasses pool)
- Multiple subnets per shared-network with independent pools
- Listen-interface restricts which interfaces the server operates on

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/schema/ze-iface-conf.yang` - DHCP client config per-interface unit (dhcp block with enabled, client-id, hostname)
- [ ] `internal/plugins/ntp/register.go` - plugin init() registration pattern
- [ ] `internal/plugins/l2tppool/pool.go` - address pool with start/end allocation

**Behavior to preserve:**
- Per-interface DHCP client in ze-iface-conf.yang unchanged
- Plugin registration pattern via init() in register.go
- No coupling to existing l2tp pool code

**Behavior to change:**
- New plugin `dhcpserver` registered via init()
- New YANG schema `ze-dhcp-server-conf.yang` for server config
- DHCP packet handling on configured listen-interfaces

## Data Flow (MANDATORY)

### Entry Point
- Config commit containing `dhcp-server { listen-interface br0; shared-network LAN { subnet ... } }` block

### Transformation Path
1. YANG schema validates config (subnets within range, non-overlapping pools, valid MACs)
2. Plugin started via init() registration; config extracted from tree
3. Raw UDP socket bound to port 67 on each listen-interface
4. DHCPDISCOVER received -> allocate IP from pool (or static mapping) -> send DHCPOFFER
5. DHCPREQUEST received -> confirm allocation -> send DHCPACK with options
6. Lease tracked in-memory with expiry timer; expired leases return IP to pool
7. DHCPRELEASE received -> free lease immediately

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> plugin | config tree extract at startup and reload | [ ] |
| Plugin -> network | raw UDP socket on port 67 per listen-interface | [ ] |
| Plugin -> lease state | in-memory lease table with per-lease expiry goroutine | [ ] |

### Integration Points
- `internal/component/iface/` - listen-interface name resolution
- `cmd/ze/hub/loader_create.go` - plugin wiring into daemon startup

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling (does not import l2tp pool)
- [ ] No duplicated functionality (independent pool implementation for DHCP domain)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| config commit with dhcp-server block | → | dhcpserver plugin start + pool init | `test/dhcp/server.ci` |
| DHCPDISCOVER on listen-interface | → | pool allocate + DHCPOFFER | `TestDHCPDiscoverOffer` |
| DHCPREQUEST for offered IP | → | lease confirm + DHCPACK | `TestDHCPRequestAck` |
| static-mapping MAC received | → | fixed IP returned in OFFER | `TestDHCPStaticMapping` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with dhcp-server, listen-interface, subnet, range, options | Config parses and validates successfully |
| AC-2 | DHCPDISCOVER from client on listen-interface | DHCPOFFER with IP from configured range and correct options |
| AC-3 | DHCPREQUEST for previously offered IP | DHCPACK with lease-time, router, DNS options included |
| AC-4 | Static mapping configured: MAC -> IP | Client with that MAC always gets the mapped IP |
| AC-5 | Lease expiry after configured lease-time seconds | IP returned to pool, available for new clients |
| AC-6 | DHCPRELEASE from client | Lease freed immediately, IP available for allocation |
| AC-7 | All IPs in pool allocated (pool exhaustion) | DHCPNAK sent to new DHCPDISCOVER requests |
| AC-8 | Multiple subnets in same shared-network | Each subnet has independent pool and options |
| AC-9 | Config reload with range change | Active leases preserved; new range effective for new allocations |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDHCPServerConfigParse` | `internal/plugins/dhcpserver/config_test.go` | Config extraction: listen-interface, subnets, ranges, options | |
| `TestPoolAllocate` | `internal/plugins/dhcpserver/pool_test.go` | IP allocation from range, skipping static mappings | |
| `TestPoolExhaustion` | `internal/plugins/dhcpserver/pool_test.go` | Returns error when all IPs allocated | |
| `TestPoolRelease` | `internal/plugins/dhcpserver/pool_test.go` | Released IP becomes available again | |
| `TestLeaseExpiry` | `internal/plugins/dhcpserver/lease_test.go` | Lease expires after configured time, IP freed | |
| `TestLeaseRenew` | `internal/plugins/dhcpserver/lease_test.go` | Renew extends lease, same IP retained | |
| `TestDHCPDiscoverOffer` | `internal/plugins/dhcpserver/handler_test.go` | DISCOVER produces OFFER with correct options | |
| `TestDHCPRequestAck` | `internal/plugins/dhcpserver/handler_test.go` | REQUEST produces ACK with lease-time | |
| `TestDHCPStaticMapping` | `internal/plugins/dhcpserver/handler_test.go` | MAC with static mapping always gets fixed IP | |
| `TestDHCPRelease` | `internal/plugins/dhcpserver/handler_test.go` | RELEASE frees lease | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| lease | 60-604800 | 604800 | 59 | 604801 |
| subnet prefix-length | 1-30 | 30 | 0 | 31 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dhcp-server-config` | `test/parse/dhcp-server-config.ci` | Config with subnet, range, options, static mapping validates | pass |
| `dhcp-server-static-only` | `test/parse/dhcp-server-static-only.ci` | Static-only subnet without dynamic range validates | pass |
| `dhcp-server-multi-subnet` | `test/parse/dhcp-server-multi-subnet.ci` | Multiple subnets in shared-network validates | pass |

## Files to Modify
- `internal/component/plugin/all/all.go` - wired via `make generate` (codegen adds blank imports)
- `internal/component/plugin/all/all_test.go` - add dhcpserver to expected plugin list
- `cmd/ze/main_test.go` - add dhcpserver to expected plugin list

## Files to Create
- `internal/plugins/dhcpserver/register.go` - plugin entry, init() registration
- `internal/plugins/dhcpserver/config.go` - config extraction from tree
- `internal/plugins/dhcpserver/config_test.go` - config parsing tests
- `internal/plugins/dhcpserver/pool.go` - address pool allocate/release
- `internal/plugins/dhcpserver/pool_test.go` - pool allocation tests
- `internal/plugins/dhcpserver/lease.go` - lease state with expiry timers
- `internal/plugins/dhcpserver/lease_test.go` - lease lifecycle tests
- `internal/plugins/dhcpserver/handler.go` - DHCP packet encode/decode + state machine (RFC 2131)
- `internal/plugins/dhcpserver/handler_test.go` - packet handling tests
- `internal/plugins/dhcpserver/schema/ze-dhcp-server-conf.yang` - YANG config schema
- `internal/plugins/dhcpserver/schema/embed.go` - YANG schema embed
- `internal/plugins/dhcpserver/schema/register.go` - YANG module registration
- `internal/plugins/dhcpserver/socket_linux.go` - Linux SO_BINDTODEVICE socket binding
- `internal/plugins/dhcpserver/socket_other.go` - non-Linux socket fallback

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist |
| 7-9. Fix cycle | Fix issues, re-verify |
| 10. Deliverables | Deliverables Checklist |
| 11. Security | Security Review Checklist |

### Implementation Phases

1. **Phase: YANG schema + registration** - Define ze-dhcp-server-conf.yang, register plugin via init()
   - Tests: `TestDHCPServerConfigParse`
   - Files: `schema/ze-dhcp-server-conf.yang`, `schema/register.go`, `register.go`, `config.go`
   - Verify: config parse test fails -> implement schema -> passes

2. **Phase: Address pool** - Pool allocation with start/end ranges, static mapping exclusion, exhaustion
   - Tests: `TestPoolAllocate`, `TestPoolExhaustion`, `TestPoolRelease`
   - Files: `pool.go`
   - Verify: pool tests fail -> implement -> pass

3. **Phase: Lease tracking** - In-memory lease table with per-lease expiry timers
   - Tests: `TestLeaseExpiry`, `TestLeaseRenew`
   - Files: `lease.go`
   - Verify: lease tests fail -> implement -> pass

4. **Phase: DHCP handler** - DISCOVER/OFFER/REQUEST/ACK/NAK/RELEASE packet processing
   - Tests: `TestDHCPDiscoverOffer`, `TestDHCPRequestAck`, `TestDHCPStaticMapping`, `TestDHCPRelease`
   - Files: `handler.go`
   - Verify: handler tests fail -> implement -> pass

5. **Phase: Socket binding** - UDP 67 listener per listen-interface
   - Files: `register.go` (run function)

6. **Phase: Daemon wiring** - Wire into loader_create.go
   - Files: `cmd/ze/hub/loader_create.go`

7. **Phase: Functional tests** - End-to-end with dhclient or test harness
   - Tests: `test/dhcp/server.ci`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All AC-1 through AC-9 have implementation with file:line |
| Correctness | DHCP state machine matches RFC 2131 Section 4 |
| Naming | Plugin name `dhcpserver` follows existing naming (ntp, static, sysctl) |
| Data flow | Packets received on listen-interface only; responses sent to correct client |
| Rule: no-layering | Does not import or duplicate l2tp pool code |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| YANG schema | `ls internal/plugins/dhcpserver/schema/ze-dhcp-server-conf.yang` |
| Plugin registration | `grep dhcpserver cmd/ze/hub/loader_create.go` |
| Pool allocator | `ls internal/plugins/dhcpserver/pool.go` |
| Lease manager | `ls internal/plugins/dhcpserver/lease.go` |
| DHCP handler | `ls internal/plugins/dhcpserver/handler.go` |
| Unit tests pass | `go test ./internal/plugins/dhcpserver/...` |
| Functional test | `ls test/dhcp/server.ci` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | MAC format validation, IP within subnet, non-overlapping ranges |
| Resource exhaustion | Max leases per subnet, DHCP flood rate limiting per source MAC |
| Privilege | UDP port 67 requires CAP_NET_BIND_SERVICE or root |
| Spoofing | Validate DHCP message fields (htype, hlen, chaddr) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read RFC 2131 and source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong then DESIGN; if AC correct then IMPLEMENT |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: DISCOVER/OFFER/REQUEST/ACK state machine (RFC 2131 Section 4.3), option encoding (RFC 2132 Section 3), lease-time handling (RFC 2131 Section 4.4.5).

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1 through AC-9 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/plugins/dhcpserver/*`)
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added (RFC 2131, 2132)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling
