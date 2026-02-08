# Spec: fix-exabgp-tests

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `test/exabgp-compat/bin/exabgp` - wrapper script
4. `test/exabgp-compat/bin/bgp` - mock BGP peer (Python)
5. `test/exabgp-compat/bin/functional` - test runner (Python)
6. `internal/plugin/bgp/schema/ze-bgp-conf.yang` - attribute container

## Task

Fix all 37 ExaBGP compatibility tests broken by commit `c87c41f` which changed `.ci` format from `option:file:` to `option=file:` without updating the Python test scripts. Also fix related infrastructure issues (wrong command path, missing socket env var, missing YANG leaf, missing config file).

**Target: 33/37 tests passing.** 4 tests require deeper feature work (deferred).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config format

### Source Files
- [ ] `test/exabgp-compat/bin/bgp` - mock peer, option parsing at line 1482
- [ ] `test/exabgp-compat/bin/functional` - test runner, option parsing at lines 1137, 1543
- [ ] `test/exabgp-compat/bin/exabgp` - wrapper, migrate call at line 59
- [ ] `internal/plugin/bgp/schema/ze-bgp-conf.yang` - attribute container at line 274

**Key insights:**
- All `.ci` files use `option=file:` (new format) but Python scripts parse `option:file:` (old format)
- `exabgp` wrapper calls `ze bgp config migrate` but correct command is `ze exabgp migrate`
- YANG schema missing `rd` leaf — downstream parser and struct field already exist
- No `ze_bgp_api_socketpath` set — Ze defaults to `/var/run/ze-bgp.sock` requiring root

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `test/exabgp-compat/bin/bgp` - parses `option:file:` at line 1482, fails on `option=file:` with "invalid rule"
- [ ] `test/exabgp-compat/bin/functional` - parses `option:file:` at line 1543, silently gets no config files
- [ ] `test/exabgp-compat/bin/exabgp` - calls `ze bgp config migrate` at line 59 which doesn't exist
- [ ] `internal/plugin/bgp/schema/ze-bgp-conf.yang` - attribute container (line 274-302) has label, path-information but NO rd
- [ ] `internal/config/routeattr.go` - ParseRouteDistinguisher() at line 656 already exists
- [ ] `internal/config/bgp.go` - StaticRouteConfig.RD field at line 225 already exists

**Behavior to preserve:**
- Ze functional tests (`make functional`) must continue passing
- ExaBGP test runner's overall architecture (server=bgp peer, client=exabgp wrapper)

**Behavior to change:**
- Python scripts updated to parse `option=` format
- Migrate command path fixed
- Socket path env var added to wrapper
- YANG schema gets `rd` leaf
- Missing config file copied from ExaBGP reference

## Data Flow

### Entry Point
- `make functional-exabgp` → `./test/exabgp-compat/bin/functional encoding --timeout 60`

### Transformation Path
1. `functional` reads `.ci` files, extracts `option=file:` config references
2. `functional` starts `bgp` server (mock peer) with `.ci` expectations
3. `functional` starts `exabgp` client wrapper with `.conf` config
4. `exabgp` wrapper calls `ze exabgp migrate <conf>` to convert ExaBGP→Ze format
5. `exabgp` wrapper runs `ze <migrated.conf>` to start BGP session
6. Ze connects to bgp server, sends BGP messages
7. bgp server validates messages against expectations

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| functional → bgp | subprocess + .ci file | [ ] |
| functional → exabgp | subprocess + .conf path | [ ] |
| exabgp → ze exabgp migrate | subprocess + stdout | [ ] |
| exabgp → ze (BGP) | subprocess + temp config | [ ] |

### Integration Points
- `ze exabgp migrate` (cmd/ze/exabgp/main.go) - converts ExaBGP→Ze config format
- `ze validate` (cmd/ze/main.go) - YANG-driven config validation
- `internal/config/parser.go` - YANG schema validator that rejects unknown fields
- `internal/config/routeattr.go` - attribute parsing (rd, label, path-information already implemented)
- `cmd/ze-test/bgp.go:491-498` - reference pattern for socket path env var

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| N/A | N/A | This is a test infrastructure fix — no new unit tests needed | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| ExaBGP suite | `make functional-exabgp` | 33+ of 37 ExaBGP compat tests pass | |
| Ze suite | `make functional` | All 224 tests still pass (no regression) | |

## Files to Modify

- `test/exabgp-compat/bin/bgp` - `option:` → `option=` (already done)
- `test/exabgp-compat/bin/functional` - `option:` → `option=` + comments (already done)
- `test/exabgp-compat/bin/exabgp` - fix migrate command + add socket path
- `internal/plugin/bgp/schema/ze-bgp-conf.yang` - add `leaf rd`

## Files to Create

- `test/exabgp-compat/etc/api-watchdog.conf` - copy from ExaBGP reference

## Implementation Steps

### Step 1: Verify already-applied fixes (DONE)
- `option:` → `option=` in `bgp` and `functional` scripts
- `ze bgp config migrate` → `ze exabgp migrate` in `exabgp` wrapper

### Step 2: Add socket path to exabgp wrapper
- In `test/exabgp-compat/bin/exabgp`, before running `ze`, set:
  `os.environ['ze_bgp_api_socketpath'] = f'/tmp/ze-exabgp-test-{port}.sock'`
- Use the port from `exabgp_tcp_port` env var for uniqueness
- Pattern from `cmd/ze-test/bgp.go:491-498`

### Step 3: Add `rd` leaf to YANG schema
- In `internal/plugin/bgp/schema/ze-bgp-conf.yang`, inside `container attribute` (line 274)
- Add after `path-information` (line 293):
  `leaf rd { type string; description "Route Distinguisher (RFC 4364)"; }`
- The downstream parser `ParseRouteDistinguisher()` in `routeattr.go:656` and `StaticRouteConfig.RD` field in `bgp.go:225` already exist

### Step 4: Copy missing config file
- Copy `/Users/thomas/Code/github.com/exa-networks/exabgp/main/etc/exabgp/api-watchdog.conf` to `test/exabgp-compat/etc/api-watchdog.conf`

### Step 5: Run single test to verify
- `./test/exabgp-compat/bin/functional encoding 5` (conf-ebgp — simple test)
- Verify it passes end-to-end

### Step 6: Run full ExaBGP suite
- `make functional-exabgp`
- Expect 33+ pass, 4 fail (3 VPN wire encoding, 1 watchdog process)

### Step 7: Run regular functional tests
- `make functional`
- Verify no regression

## Expected Results

| Category | Count | Tests |
|----------|-------|-------|
| Pass | 33 | All non-VPN, non-watchdog tests |
| Fail (VPN wire encoding) | 3 | conf-addpath, conf-parity, conf-vpn |
| Fail (watchdog process) | 1 | conf-watchdog |

### Why 3 VPN tests still fail
Adding `rd` to YANG lets the config parse, but these tests also need Ze to actually encode VPN NLRI with RD+label into wire UPDATE messages from config routes. That's a feature gap in the config→wire path, not a schema issue.

### Why watchdog test still fails
The ExaBGP watchdog model uses `process { run ./watchdog.run; }` with ExaBGP's process API. Ze handles plugins differently via YANG RPC. The exabgp wrapper would need to bridge the process model.

## Implementation Summary

<!-- Fill after implementation -->

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Fix option: → option= in bgp | | | |
| Fix option: → option= in functional | | | |
| Fix migrate command path | | | |
| Add socket path env var | | | |
| Add rd to YANG schema | | | |
| Copy api-watchdog.conf | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| ExaBGP suite 33+ pass | | | |
| Ze suite no regression | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| test/exabgp-compat/bin/bgp | | |
| test/exabgp-compat/bin/functional | | |
| test/exabgp-compat/bin/exabgp | | |
| internal/plugin/bgp/schema/ze-bgp-conf.yang | | |
| test/exabgp-compat/etc/api-watchdog.conf | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Feature code integrated into codebase (`internal/*`, `cmd/*`)
- [ ] Functional tests verify end-user behavior (`.ci` files)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read

### Completion
- [ ] Implementation Audit completed
- [ ] Spec updated with Implementation Summary
