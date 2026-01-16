# Spec: ExaBGP Compatibility

## Task

Provide tools for migrating from ExaBGP to ZeBGP:

1. **`zebgp exabgp plugin`** - Run existing ExaBGP plugins with ZeBGP (runtime bridge)
2. **`zebgp exabgp migrate`** - Convert ExaBGP configs to ZeBGP format (one-time conversion)

## Required Reading

### Architecture Docs
- [x] `docs/architecture/api/architecture.md` - ZeBGP plugin JSON format
- [x] `docs/architecture/api/capability-contract.md` - 5-stage startup protocol, RIB ownership
- [x] `docs/architecture/api/json-format.md` - ZeBGP JSON event format, negotiated message
- [x] `docs/architecture/config/syntax.md` - ZeBGP config syntax
- [x] `docs/exabgp/exabgp-code-map.md` - ExaBGP to ZeBGP mapping
- [x] `docs/exabgp/exabgp-differences.md` - Behavioral differences

### External Reference
- [x] ExaBGP `reactor/api/response/json.py` - ExaBGP JSON output format
- [x] ExaBGP `reactor/api/response/text.py` - ExaBGP text command format
- [x] ExaBGP config format - neighbor, process, api blocks

**Key insights:**
- ZeBGP uses `"ipv4/unicast"` family format; ExaBGP uses `"ipv4 unicast"` (space)
- ZeBGP direction: `"received"`/`"sent"`; ExaBGP: `"receive"`/`"send"`
- ZeBGP uses `peer` keyword; ExaBGP uses `neighbor`
- ExaBGP wraps all JSON in `{"exabgp": "5.0.0", ...}` envelope
- ZeBGP has 5-stage startup protocol; ExaBGP plugins don't participate
- **ExaBGP handles RIB internally; ZeBGP delegates RIB to plugins**
- ExaBGP plugins can't declare capabilities; bridge must synthesize from CLI flags

---

## Component 1: Plugin Bridge (`zebgp exabgp plugin`)

### Purpose

Wrap ExaBGP plugins so they can run with ZeBGP. Translates:
- ZeBGP JSON events → ExaBGP JSON format (stdin to plugin)
- ExaBGP text commands → ZeBGP commands (stdout from plugin)

### Architecture

```
pkg/exabgp/
├── bridge.go         # ZebgpToExabgpJSON(), ExabgpToZebgpCommand(), Bridge
└── bridge_test.go    # Unit tests

cmd/zebgp/exabgp.go   # CLI: zebgp exabgp plugin <plugin>
```

### Data Flow

```
ZeBGP Engine
    │ (stdin: ZeBGP JSON)
    ▼
zebgp exabgp plugin        ← Bridge process
    │ translates JSON
    │ spawns subprocess
    ▼
ExaBGP Plugin              ← User's existing plugin
    │ (stdout: ExaBGP commands)
    ▼
zebgp exabgp plugin
    │ translates commands
    ▼
ZeBGP Engine (stdout: ZeBGP commands)
```

### Implementation Status

| Feature | Status | Blocking |
|---------|--------|----------|
| JSON event translation (update, state) | ✅ Done | - |
| Command translation (announce, withdraw) | ✅ Done | - |
| 5-stage startup protocol handling | ❌ TODO | **Yes** - bridge killed after 5s without this |
| Capability CLI flags | ❌ TODO | No - defaults work |
| `negotiated` message conversion | ❌ TODO | No - can ignore initially |

### Conversion Tables

**ZeBGP → ExaBGP JSON:**

| ZeBGP | ExaBGP |
|-------|--------|
| `"message": {"type": "update"}` | `"type": "update"` (top-level) |
| `"peer": {"address": "X"}` | `"neighbor": {"address": {"peer": "X"}}` |
| `"ipv4/unicast"` | `"ipv4 unicast"` |
| `"direction": "received"` | `"direction": "receive"` |
| Top-level attributes | `"attribute": {...}` nested |

**ExaBGP commands → ZeBGP:**

| ExaBGP | ZeBGP |
|--------|-------|
| `neighbor X announce route` | `peer X update text nlri ipv4/unicast add` |
| `neighbor X withdraw route` | `peer X update text nlri ipv4/unicast del` |
| `next-hop Y` | `nhop set Y` |
| `origin igp` | `origin set igp` |
| `as-path [A B]` | `as-path set A B` |

### TODO: Startup Protocol Handling

ZeBGP plugins participate in a 5-stage startup protocol. **This is mandatory** - plugins that don't complete stages within timeout (default 5s) are killed.

**Problem:** Bridge runs as ZeBGP plugin, must complete startup before ExaBGP plugin sees any events.

**Solution:** Bridge handles startup internally, responds on behalf of ExaBGP plugin.

| Stage | Direction | Bridge Must | Timeout |
|-------|-----------|-------------|---------|
| Registration | Plugin → ZeBGP | Send `declare done` (with optional family/encoding) | 5s |
| Config | ZeBGP → Plugin | Read lines until `config done`, discard | - |
| Capability | Plugin → ZeBGP | Send `capability done` (with optional caps from CLI) | 5s |
| Registry | ZeBGP → Plugin | Read lines until `registry done`, discard | - |
| Ready | Plugin → ZeBGP | Send `ready` | 5s |
| Running | Bidirectional | Begin pass-through JSON translation | - |

**Wire Format (text commands, not JSON):**

```
# Stage 1: Registration (bridge → ZeBGP)
declare encoding text
declare family ipv4 unicast
declare family ipv6 unicast
declare done

# Stage 2: Config (ZeBGP → bridge) - bridge discards
config peer 10.0.0.1 local-as 65001
config done

# Stage 3: Capability (bridge → ZeBGP)
capability done

# Stage 4: Registry (ZeBGP → bridge) - bridge discards
registry name exabgp-compat
registry done

# Stage 5: Ready (bridge → ZeBGP)
ready

# Stage 6: Running - normal JSON event translation begins
```

**Key insight:** Bridge reads from stdin (text protocol during startup, JSON during running). Must detect stage transitions by parsing `config done` and `registry done` markers.

### TODO: Capability CLI Flags

```
zebgp exabgp plugin [flags] <plugin-command>

Flags:
  --family <family>         Add supported family (repeatable)
                            Default: ipv4/unicast
  --route-refresh           Enable route-refresh capability
  --add-path <mode>         ADD-PATH mode: none, receive, send, both
  --asn4                    Enable 4-byte ASN (default: true)
```

**Mapping to startup protocol:**

| CLI Flag | Stage 1 (declare) | Stage 3 (capability) |
|----------|-------------------|----------------------|
| `--family ipv4/unicast` | `declare family ipv4 unicast` | - |
| `--family ipv6/unicast` | `declare family ipv6 unicast` | - |
| `--route-refresh` | - | `capability hex 2 00` (RFC 2918) |
| `--add-path receive` | - | `capability hex 69 <encoded>` |
| `--asn4` | - | (handled by engine, not plugin) |

---

## Component 2: Config Migration (`zebgp exabgp migrate`)

### Purpose

Convert ExaBGP configuration files to ZeBGP format. One-time migration tool.

### Key Syntax Differences

| Concept | ExaBGP | ZeBGP |
|---------|--------|-------|
| Peer definition | `neighbor <ip> { }` | `peer <ip> { }` |
| Plugin definition | `process NAME { }` (top-level) | `plugin NAME { }` (top-level) |
| Plugin binding | `api { processes [...]; }` | `process NAME { }` (inside peer) |
| Capability syntax | `capability { route-refresh; }` | `capability { route-refresh enable; }` |

See `docs/architecture/config/syntax.md` for full ZeBGP config reference.

### Critical Difference: RIB Ownership

| Feature | ExaBGP | ZeBGP |
|---------|--------|-------|
| Route storage | Internal | External plugin |
| Graceful restart state | Internal | Plugin responsibility |
| Route refresh replay | Internal | Plugin responsibility |

**Implication:** If ExaBGP config uses features requiring RIB, migrated config MUST include a RIB plugin.

### RIB-Requiring Features

| ExaBGP Feature | Requires RIB Plugin |
|----------------|---------------------|
| `graceful-restart` | Yes - plugin must store/replay routes |
| `route-refresh` (capability) | Yes - plugin must respond to refresh |
| Any `api { receive { update; } }` | Yes - plugin likely stores routes |
| Simple announce-only process | No - stateless, no RIB needed |

### Migration Examples

**Simple case (no RIB needed):**

```
# ExaBGP
neighbor 10.0.0.1 {
    router-id 1.1.1.1;
    local-address 10.0.0.2;
    local-as 65001;
    peer-as 65002;
}

# ZeBGP (migrated)
peer 10.0.0.1 {
    router-id 1.1.1.1;
    local-address 10.0.0.2;
    local-as 65001;
    peer-as 65002;
}
```

**With graceful-restart (RIB plugin injected):**

```
# ExaBGP
neighbor 10.0.0.1 {
    router-id 1.1.1.1;
    local-as 65001;
    peer-as 65002;
    capability {
        graceful-restart 120;
    }
}

process my-plugin {
    run /path/to/plugin.py;
    encoder json;
}

# ZeBGP (migrated) - RIB plugin auto-injected
plugin rib {
    run "zebgp plugin rib";
}

plugin my-plugin-compat {
    run "zebgp exabgp plugin /path/to/plugin.py";
    encoder json;
}

peer 10.0.0.1 {
    router-id 1.1.1.1;
    local-as 65001;
    peer-as 65002;
    capability {
        graceful-restart 120;
    }

    process rib {
        send { update; state; }
        receive { update; }
    }

    process my-plugin-compat {
        send { update; state; }
    }
}
```

**With route-refresh:**

```
# ExaBGP
neighbor 10.0.0.1 {
    capability {
        route-refresh;
    }
    api {
        processes [ my-plugin ];
        receive { update; }
    }
}

# ZeBGP (migrated) - RIB plugin required for refresh response
plugin rib {
    run "zebgp plugin rib";
}

plugin my-plugin-compat {
    run "zebgp exabgp plugin /path/to/plugin.py";
    encoder json;
}

peer 10.0.0.1 {
    capability {
        route-refresh enable;
    }

    process rib {
        send { update; state; refresh; }
        receive { update; }
    }

    process my-plugin-compat {
        send { update; }
    }
}
```

### Syntax Mapping

| ExaBGP | ZeBGP |
|--------|-------|
| `neighbor <ip> { ... }` | `peer <ip> { ... }` |
| `local-as N;` | `local-as N;` |
| `peer-as N;` | `peer-as N;` |
| `router-id X;` | `router-id X;` |
| `capability { graceful-restart N; }` | `capability { graceful-restart N; }` |
| `capability { route-refresh; }` | `capability { route-refresh enable; }` |
| `capability { asn4; }` | `capability { asn4 enable; }` (default) |
| `process NAME { run CMD; }` | `plugin NAME { run "zebgp exabgp plugin CMD"; }` |
| `api { processes [ P ]; }` | `process P { send {...}; receive {...}; }` (inside peer) |

### Implementation Status

| Feature | Status |
|---------|--------|
| Basic syntax conversion | ❌ TODO |
| RIB plugin injection | ❌ TODO |
| Process wrapping with bridge | ❌ TODO |
| Validation of unsupported features | ❌ TODO |

---

## 🧪 TDD Test Plan

### Unit Tests - Bridge

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestZebgpToExabgpJSON_*` | `pkg/exabgp/bridge_test.go` | ZeBGP JSON → ExaBGP JSON | ✅ |
| `TestExabgpToZebgpCommand_*` | `pkg/exabgp/bridge_test.go` | ExaBGP commands → ZeBGP | ✅ |
| `TestRoundTrip` | `pkg/exabgp/bridge_test.go` | Essential info preserved | ✅ |
| `TestNegotiatedConversion` | `pkg/exabgp/bridge_test.go` | `negotiated` message format | TODO |
| `TestStartupProtocol` | `pkg/exabgp/bridge_test.go` | 5-stage startup handling | TODO |
| `TestCapabilityFlags` | `cmd/zebgp/exabgp_test.go` | CLI flag parsing | TODO |

### Unit Tests - Migration

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMigrateSimple` | `pkg/exabgp/migrate_test.go` | Basic neighbor → peer | TODO |
| `TestMigrateWithGR` | `pkg/exabgp/migrate_test.go` | GR config injects RIB plugin | TODO |
| `TestMigrateWithRR` | `pkg/exabgp/migrate_test.go` | Route-refresh injects RIB | TODO |
| `TestMigrateProcess` | `pkg/exabgp/migrate_test.go` | Process wrapped with bridge | TODO |
| `TestMigrateUnsupported` | `pkg/exabgp/migrate_test.go` | Error on unsupported features | TODO |

### Functional Tests - Migration

File-based tests comparing ExaBGP input → ZeBGP output:

```
test/data/migrate/
├── simple/
│   ├── input.conf      # ExaBGP config
│   └── expected.conf   # ZeBGP config after migration
├── graceful-restart/
│   ├── input.conf      # ExaBGP with GR
│   └── expected.conf   # ZeBGP with RIB plugin injected
├── route-refresh/
│   ├── input.conf      # ExaBGP with route-refresh
│   └── expected.conf   # ZeBGP with RIB plugin injected
└── process/
    ├── input.conf      # ExaBGP with process
    └── expected.conf   # ZeBGP with bridge wrapper
```

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `migrate-simple` | `test/data/migrate/simple/` | Basic neighbor → peer | TODO |
| `migrate-gr` | `test/data/migrate/graceful-restart/` | GR injects RIB plugin | TODO |
| `migrate-rr` | `test/data/migrate/route-refresh/` | Route-refresh injects RIB | TODO |
| `migrate-process` | `test/data/migrate/process/` | Process wrapped with bridge | TODO |

**Note:** Existing `pkg/config/migration/` handles ZeBGP internal syntax evolution (e.g., old ZeBGP → new ZeBGP). ExaBGP→ZeBGP conversion is a **separate concern** requiring new code in `pkg/exabgp/migrate.go`.

---

## Files to Modify

- `cmd/zebgp/main.go` - Add `config migrate` subcommand
- `pkg/exabgp/bridge.go` - Add startup protocol, capability flags

## Files to Create

- `pkg/exabgp/migrate.go` - ExaBGP→ZeBGP config migration logic
- `pkg/exabgp/migrate_test.go` - Migration unit tests
- `cmd/zebgp/migrate.go` - `zebgp exabgp migrate --from exabgp` command
- `test/data/migrate/simple/input.conf` - Simple ExaBGP config
- `test/data/migrate/simple/expected.conf` - Expected ZeBGP output
- `test/data/migrate/graceful-restart/input.conf` - ExaBGP with GR
- `test/data/migrate/graceful-restart/expected.conf` - ZeBGP with RIB plugin
- `test/data/migrate/route-refresh/input.conf` - ExaBGP with RR
- `test/data/migrate/route-refresh/expected.conf` - ZeBGP with RIB plugin
- `test/data/migrate/process/input.conf` - ExaBGP with process
- `test/data/migrate/process/expected.conf` - ZeBGP with bridge wrapper

## Existing Files (Done)

- `pkg/exabgp/bridge.go` - Translation functions
- `pkg/exabgp/bridge_test.go` - Bridge unit tests
- `cmd/zebgp/exabgp.go` - CLI wrapper

---

## Implementation Steps

### Phase 1: Bridge Startup Protocol (BLOCKING - TODO)
1. ~~Research startup protocol wire format~~ ✅ Done - text commands in `pkg/plugin/registration.go`
2. Write `TestStartupProtocol` - verify FAIL
3. Implement startup stage handling:
   - Parse stdin for `config ...` and `registry ...` lines during startup
   - Send `declare family ... done` for Stage 1
   - Send `capability done` for Stage 3
   - Send `ready` for Stage 5
   - Switch to JSON mode after `ready`
4. Verify test PASS
5. Integration test: bridge survives 5s timeout

### Phase 2: Bridge Enhancements (TODO)
1. Write `TestCapabilityFlags` - verify FAIL
2. Implement CLI flag parsing (`--family`, `--route-refresh`)
3. Map flags to `declare family` and `capability` commands
4. Write `TestNegotiatedConversion` - verify FAIL
5. Implement `negotiated` message translation
6. Verify all tests PASS

### Phase 3: Config Migration (TODO)

**Note:** This is ExaBGP→ZeBGP conversion, separate from existing `pkg/config/migration/` (ZeBGP syntax evolution).

1. Create file-based test structure:
   - `test/data/migrate/simple/input.conf` + `expected.conf`
   - `test/data/migrate/graceful-restart/input.conf` + `expected.conf`
   - `test/data/migrate/process/input.conf` + `expected.conf`
2. Write `TestMigrateSimple` - verify FAIL
3. Implement basic syntax conversion (`neighbor→peer`, capability syntax)
4. Write `TestMigrateWithGR` - verify FAIL
5. Implement RIB plugin injection logic
6. Write `TestMigrateProcess` - verify FAIL
7. Implement process wrapping with bridge
8. Add functional test runner (compare output to expected.conf)
9. Verify all tests PASS

---

## Important Clarification: Two Migration Systems

| System | Location | Purpose |
|--------|----------|---------|
| ZeBGP internal migration | `pkg/config/migration/` | Evolve ZeBGP config syntax over time |
| ExaBGP→ZeBGP migration | `pkg/exabgp/migrate.go` (NEW) | Convert ExaBGP configs to ZeBGP |

The existing `pkg/config/migration/` handles things like:
- `neighbor→peer` (ZeBGP syntax change)
- `api→new-format` (ZeBGP API syntax change)

The **new** `pkg/exabgp/migrate.go` handles:
- ExaBGP syntax → ZeBGP syntax
- RIB plugin injection for GR/RR
- Process wrapping with bridge

These are **separate concerns** but may share some transformations.

---

## Open Questions

1. ~~**Startup protocol format**~~ **RESOLVED** - Text commands (`declare`, `capability`, `ready`). See `pkg/plugin/registration.go`.

2. **RIB plugin selection** - Should migration use `zebgp plugin rib` or `zebgp plugin rr`? Depends on use case:
   - Single peer: `rib`
   - Route server (multi-peer): `rr`
   - **Proposal:** Default to `rib`, add `--route-server` flag for `rr`

3. **Unsupported features** - What ExaBGP features have no ZeBGP equivalent? Should migration fail or warn?

4. **Process capability inference** - Can we infer needed capabilities from ExaBGP `api { receive { ... } }` block?

5. **Bridge stdin multiplexing** - During startup, bridge reads text commands. After `ready`, it reads JSON events. Need to handle this transition cleanly.

---

---

## Critical Issues Found (2024-01 Review)

### Issue 1: Dead Code in migrate.go ✅ FIXED

`migrateAPIBlock()` checks for `api` container but LegacyBGPSchema doesn't define `api` inside neighbor - it uses `process { processes [...] }`. The function never executes its main logic.

**Fix:** ✅ Added `pkg/exabgp/schema.go` with ExaBGP-specific schema that includes `api` block.

### Issue 2: Test Data Uses Wrong Syntax ✅ FIXED

Test files in `test/data/migrate/` use `process { processes [...] }` syntax instead of actual ExaBGP `api { processes [...] }`. Tests pass but don't test real ExaBGP migration.

**Fix:** ✅ Updated `test/data/migrate/process/input.conf` to use real ExaBGP `api { processes [...] }` syntax.

### Issue 3: Missing Transforms (Partial)

From spec examples not implemented:
- `group-updates false;` handling - TODO
- `family { ipv4 unicast; }` → `family { ipv4/unicast; }` conversion - TODO
- Capability bare flags → `enable` - ✅ Done (route-refresh)

### Issue 4: Incomplete serializeTree ✅ FIXED

Only serializes plugin/peer/capability/process/send/receive. Missing: family, announce, static, etc.

**Fix:** ✅ `SerializeTree()` now handles all blocks needed for current migration scenarios. Additional blocks can be added as needed.

---

## Implementation Summary

### What Was Implemented (Migration Component)

1. **ExaBGP Schema** (`pkg/exabgp/schema.go`)
   - Defines ExaBGP-specific config parsing with `api` block support
   - `ParseExaBGPConfig()` function for parsing ExaBGP configs

2. **migrate.go Updates**
   - `api` block handling in `NeedsRIBPlugin()` - detects `receive { update; }`
   - `migrateProcessBindings()` handles both `api { processes [...] }` and `process { processes [...] }`
   - `SerializeTree()` for output generation

3. **CLI Command** (`cmd/zebgp/exabgp.go`)
   - Added `zebgp exabgp migrate <file>` subcommand
   - Reads config, parses with ExaBGP schema, migrates, outputs to stdout

4. **Test Data** (`test/data/migrate/process/input.conf`)
   - Updated to use real ExaBGP `api { processes [...] }` syntax

5. **Tests** (`pkg/exabgp/migrate_test.go`)
   - Uses `ParseExaBGPConfig()` for proper ExaBGP parsing
   - File-based tests validate all 4 migration scenarios

### Verification Results

```
make test       # All unit tests pass
make lint       # 0 issues
make functional # 30 tests pass (12 parsing, 18 decoding)
```

### Remaining Work (Not Blocking)

| Feature | Status | Notes |
|---------|--------|-------|
| Bridge startup protocol | TODO | BLOCKING for plugin bridge |
| Capability CLI flags | TODO | Optional |
| `negotiated` message | TODO | Optional |
| Family syntax conversion | TODO | `ipv4 unicast` → `ipv4/unicast` |

---

## Checklist

### 🧪 TDD (Bridge JSON/Command Translation)
- [x] Tests written
- [x] Tests FAIL
- [x] Tests PASS

### Bridge (Component 1)
- [x] JSON translation implemented (14 tests)
- [x] Command translation implemented
- [ ] **Startup protocol handling (BLOCKING)**
- [ ] Capability CLI flags
- [ ] `negotiated` message conversion

### Migration (Component 2)
- [x] Basic syntax conversion
- [x] RIB plugin injection (works for GR/RR detection)
- [x] Process wrapping
- [x] Functional tests (file-based, structural validation)
- [x] ExaBGP schema for proper parsing (`pkg/exabgp/schema.go`)
- [x] `zebgp exabgp migrate` CLI command
- [ ] Family syntax conversion (`ipv4 unicast` → `ipv4/unicast`)

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes
- [ ] Integration test with real ExaBGP plugin

### Documentation
- [x] `.claude/rules/compatibility.md` updated

### Completion
- [ ] All tests pass
- [ ] Commit (when user approves)
