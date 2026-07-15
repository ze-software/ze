# Spec: fixit-ping-size-followups

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-15 |

## Task

Two follow-ups left open by `c00c795cf` (feat(ping): add size to show ping, wire
the web form field). Both were deferred for the same reason: every file they
touch was dirty from a concurrent session, so no scoped commit was possible
without pulling in another session's uncommitted work.

### 1. Document the `show ping ... size <bytes>` argument

`c00c795cf` added a `size` leaf to `show ping` (range 1..65507, in
`internal/plugins/ping-cmd/yang/ze-ping-cmd.yang`). The YANG description carries
the CLI help, so nothing is factually wrong today, but the argument is
undocumented outside the schema. `ai/rules/discovery-updates.md` wants a new
feature discoverable from the docs.

Candidate files (BOTH were dirty at the time, hence the deferral):
- `docs/features/cli-commands.md` — shows `ze show ping 8.8.8.8 count 5 timeout 3s`, no `size`
- `docs/guide/command-reference.md` — mentions `show ping`, does not enumerate args

→ Constraint: `size` is the ICMP PAYLOAD size, not the total packet. iputils'
familiar "64 bytes" is 56 payload + 8 header, so any doc example must be explicit
or it will be misread. The engine default payload is `[]byte("ze-ping")`, 7 bytes
(`internal/component/ping/cmd/ping.go:152`); the web form ships 64, so a web ping
now sends a different packet than it did before `c00c795cf`.

### 2. `monitor ping` parses `count` and `size` and ignores both

`monitorPingLocal` (`internal/component/ping/cmd/register.go:62`) calls the
shared `parsePingArgs` and discards both count and opts at `:65`. So `monitor
ping <dest> count 5 size 100` silently ignores both. The count half predates
`c00c795cf`; the size half was added by it, and is documented with a comment at
the call site (`register.go:63-64`) rather than silently plumbed.

Decide one of:
- plumb size (and count) into `NewPingSession` (`internal/component/ping/cmd/stream.go:24`), which today builds a fixed `[]byte("ze-ping")` payload at `stream.go:65`; OR
- reject the unsupported arguments with a clear error per `ai/rules/error-messages.md`.

→ Constraint: `NewPingSession` has three callers — `register.go`,
`cmd/ze/hub/session_factory.go`, `internal/component/cli/client/main.go`. The
latter two were dirty from another session, which is why this was not attempted.
→ Constraint: `monitor ping` is NOT YANG-modelled for leaves at all (the
monitor/ping container in `ze-ping-cmd.yang` has no `leaf`), so giving it a real
`size` argument is a schema change, not just plumbing.
→ Decision: silently accepting an argument and ignoring it is the operator trap
`ai/rules/no-workarounds-for-missing-behavior.md` bans. Whichever option is
chosen, "parse and drop" is not an acceptable end state.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] (fill on pickup)

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/ping/cmd/register.go` - `monitorPingLocal` (:62) discards count and opts (:65)
- [ ] `internal/component/ping/cmd/stream.go` - `NewPingSession` (:24) builds a fixed `[]byte("ze-ping")` payload (:65)
- [ ] `internal/component/ping/cmd/ping.go` - default payload `[]byte("ze-ping")` (:152), overridden when `opts.size > 0` (:153-156)
- [ ] (re-read on pickup; anchors above verified 2026-07-15)

**Behavior to preserve:**
- `show ping ... size <bytes>` as shipped in `c00c795cf`; `test/plugin/show-ping.ci` asserts it end-to-end

**Behavior to change:**
- (decide on pickup — see the two options in Task item 2)

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- (fill on pickup) `monitor ping <dest> [count N] [size N]` via the local command registry

### Transformation Path
1. (fill on pickup) `monitorPingLocal` (`register.go:62`) -> `parsePingArgs` -> `NewPingSession` (`stream.go:24`)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| (fill on pickup) | | [ ] |

### Integration Points
- (fill on pickup)

## Wiring Test (MANDATORY — NOT deferrable)

<!-- Names are concrete so the intent survives; the chosen option (plumb vs
     reject) decides which assertion each one makes. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `monitor ping <dest> size 100` | → | `monitorPingLocal` (`register.go:62`) | `TestMonitorPingSizeNotSilentlyDropped` |
| `monitor ping <dest> count 5` | → | `monitorPingLocal` (`register.go:62`) | `TestMonitorPingCountNotSilentlyDropped` |
| `show ping <dest> size 1400` | → | `parsePingArgs` -> `doPing` | `show-ping` (`test/plugin/show-ping.ci`, exists and passing since `c00c795cf`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `monitor ping <dest> size 100` | Either the probe payload is 100 bytes, or the command errors naming `size` as unsupported. Never silently ignored |
| AC-2 | `monitor ping <dest> count 5` | Same rule as AC-1 for `count` |
| AC-3 | `show ping` docs | `size` appears in user-facing docs, stated as the ICMP payload size (not the iputils total-packet convention) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMonitorPingSizeNotSilentlyDropped` | `internal/component/ping/cmd/register_test.go` | AC-1 | |
| `TestMonitorPingCountNotSilentlyDropped` | `internal/component/ping/cmd/register_test.go` | AC-2 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-ping` | `test/plugin/show-ping.ci` | operator sets a packet size and the probe uses it (exists; extend if `monitor ping` gains `size`) | done (`c00c795cf`) |
| `monitor-ping-args` | `test/plugin/monitor-ping-args.ci` | operator passes `size`/`count` to `monitor ping` and is not silently ignored | to create |

## Files to Modify
- `internal/component/ping/cmd/register.go` - resolve the parse-and-drop of count/size
- `docs/features/cli-commands.md` - document the `size` argument
- (fill on pickup)

## Implementation Steps

### Implementation Phases
1. **Phase: (fill on pickup)**

## Known Limitations
- (fill on pickup)

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `make ze-test` passes
