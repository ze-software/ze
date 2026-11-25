# Spec: ExaBGP Raw-Text Event Transport

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P2 |
| Release reason | Q-E7 benchmark on Apple M4 Max (2026-11-25) measured ~1.7 us/route/script CPU and ~72-94 MiB private RAM per `ze exabgp plugin` endpoint because every BGP UPDATE is decoded into `FormatFull` JSON (~1.5 KiB), JSON-string-escaped inside `deliver-batch`, TLS-encrypted N times, unmarshaled into `map[string]any` in every wrapper, and re-parsed off its wire hex anyway. |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-bus-observer-delivery-decoupling.md` |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-25 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Eliminate the redundant JSON formatting and `map[string]any` reflection unmarshaling between `ze` and `ze exabgp plugin` (and any external plugin opting into raw-binary-in-text delivery) by extending Ze's existing raw-in-text wire event format (`encoding text` + `format raw` / `b64`, staying inside the newline-framed `MuxConn` / `deliver-batch` line protocol) so `ze` emits raw BGP wire bytes plus compact session scalars without building `FormatFull` JSON, and `ze exabgp plugin` decodes BGP wire messages directly through Ze's wire decoders into ExaBGP JSON/text lines.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/bgp-engine.md`, `ai/doors/system-plugins.md`
- [ ] `docs/architecture/exabgp-bridge.md`
- [ ] `docs/architecture/api/process-protocol.md`
- [ ] `docs/architecture/api/json-format.md`

**Key insights:**
- Skeleton created from Q-E7 bare-metal findings (commit `fb9fce5cb8`). Research begins once `spec-bus-observer-delivery-decoupling.md` completes design/implementation.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/format/text_update.go` - `appendRawFromResult` already emits `peer <ip> <dir> update raw <hex>` for `EncodingText` + `FormatRaw`, omitting session scalars (`local`, ASNs, `router-id`, `connection-id`, `asn4`, `add-path`).
- [ ] `internal/plugins/exabgp/bridgerun/fleet.go` - `Fleet.Broadcast` calls `json.Unmarshal` into `map[string]any` per event.
- [ ] `internal/plugins/exabgp/bridgerun/forms.go` - `EventFormat` currently selects `plugin.FormatFull` whenever `bgp:update` is allowed.

**Behavior to preserve:**
- Exact ExaBGP 4.2, 5.0, and 6.0 JSON and text output emitted onto script stdin (including `--form`, `bgp:negotiated` grant gating, and Q-E4 behavior when `bgp:state`/`bgp:negotiated` are not subscribed).
- Existing `encoding json` default and newline-framed `MuxConn` / `deliver-batch` contract for third-party external plugins.
- Existing `encoding text` + `format raw` (`peer <ip> <dir> update raw <hex>`) output for existing callers/tests.

**Behavior to change:**
- Add a raw-binary-in-text event format carrying raw BGP wire bytes (base64 or hex) plus the minimal session scalars needed by ExaBGP without JSON formatting in `ze`.
- Opt `ze exabgp plugin` into that raw-binary-in-text format and decode BGP wire messages directly without `map[string]any` reflection.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Raw text UPDATE formatter | `internal/component/bgp/format/text_update.go`, `appendRawFromResult` | Emits `peer <ip> <dir> update raw <hex>` without parsing attributes or NLRIs | Extends: carries compact session scalars and/or base64 wire payload so `ze exabgp plugin` needs no `FormatFull` JSON |
| Base64 wire encoding enum | `internal/component/plugin/types.go`, `WireEncodingB64` | Declares `b64` wire encoding (+33% overhead vs +100% hex) | Extends: reuses compact base64 wire encoding on the raw-in-text event rail |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| [to be derived at design via `./le spec design documents`] | - | - | - | - | - |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Inbound/outbound BGP `RawMessage` in `internal/component/bgp/server/events.go`.

### Transformation Path
1. To be traced in Research phase.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ `ze exabgp plugin` | Newline-framed `deliver-batch` carrying raw-in-text BGP wire frames | No |

### Changed-Wire Census

**Search evidence:** To be filled at Research phase.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [to be filled at design] | - | - | - | - |

### Integration Points
- To be filled at Research phase.

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
| A-1 | All ExaBGP UPDATE/OPEN/NOTIFICATION/KEEPALIVE/REFRESH fields can be reconstructed from raw BGP wire bytes + per-session scalars (`peer`, `local`, ASNs, `router-id`, `connection-id`, `asn4`, `add-path`) | `internal/exabgp/bridge/bridge_event.go` | Some ExaBGP field needs engine-computed state not in wire bytes | Audit every field read in `internal/exabgp/bridge/` during Research | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Subtle ExaBGP 4.2/5.0/6.0 formatting drift between JSON-read and wire-read paths | `./le test functional exabgp` diff | Prove byte-identical output across the full `test/exabgp/` and `test/plugin/exabgp-*` corpus |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | ExaBGP scripts receive malformed or subtly different JSON/text lines |
| How is it reverted? | Single commit revert; external plugin default remains JSON |
| Who else touches this path? | `internal/component/bgp/format`, `internal/plugins/exabgp`, `internal/exabgp/bridge` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| To be designed | → | To be designed | To be designed |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | To be designed | To be designed | To be designed |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs 1, 5, and 20 `ze exabgp plugin` endpoints on a full-table feed | Raw-in-text wire transport + direct wire decode + ExaBGP render | Mac Q-E7 re-measurement + `test/exabgp/` suite |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| To be designed | - | - | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| To be designed | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| To be designed | - | - | |

## Files to Modify
- To be filled at Research/Design phase.

## Files to Create
- To be filled at Research/Design phase.

## Design presentation

### What changes
To be filled at Design phase.

### Before and after
To be filled at Design phase.

### Alternatives set aside
| Alternative | Why set aside |
|-------------|---------------|
| Custom binary `MuxConn` framing | Breaks newline-framed line protocol for ~10-15 ns saved over hex/b64 raw-in-text |

### Documents departed from
To be filled at Design phase.

### Choices for Thomas
| # | Choice | Options | Decision |
|---|--------|---------|----------|
| - | - | - | - |

## Implementation Steps
1. To be filled at Design phase.

## Checklist
- [ ] Pre-Spec Verification complete
- [ ] Design judged and approved
