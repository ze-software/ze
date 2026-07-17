# Spec: fixit-parser-fuzz-gaps

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/6 |
| Updated | 2026-07-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `mk/test-fuzz.mk` - fuzz target registration and runner (`ze-fuzz-test`, `ze-fuzz-one`)
4. Source files in Current Behavior below
5. Model harnesses: `internal/component/bfd/packet/fuzz_test.go`, `internal/plugins/vrrp/packet/fuzz_test.go`

## Task

**[MEDIUM]** Three attacker-facing / semi-trusted network-input parsers have no fuzz target,
unlike the well-fuzzed BGP-peer path and the BFD/L2TP/ISIS/OSPF/VRRP/TACACS parsers. Verifier
notes (2026-07-16 audit) confirm all three are currently bounds-safe by code reasoning (see A-2),
so the value here is regression protection, not a live crash fix: a future edit could drop a
bound unnoticed, and the seeded fuzz corpus becomes a permanent regression test.

Gaps (all verified to have **no** `func Fuzz` today):
- **BMP receiver TLV** — `DecodeTLV` (`internal/component/bgp/plugins/bmp/tlv.go:77`, doc comment
  at :75) and its framing loop `DecodeTLVs` (`tlv.go:103`). The BMP package is a receiver: the
  real consumers are the message decoders `internal/component/bgp/plugins/bmp/msg.go:132,140,189,302`,
  which call `DecodeTLVs(buf, off, end)`. Bytes come from a configured (still remote / MITM-able)
  monitoring station.
- **RADIUS reply / VSA** — `DecodeVSA` (`internal/component/radius/attr.go:122`, doc comment at
  :120). Real consumers are the L2TP RADIUS client `internal/component/l2tp/plugins/authradius/handler.go:206`
  and `.../authradius/extract_vsa.go:21,36`, decoding vendor-specific attributes out of RADIUS
  server replies.
- **DHCP server packet** — `internal/plugins/dhcpserver/handler.go`: `handle:100`, `parseMsgType:367`,
  `parseOptionAddr:386`, `extractMAC:456`; guarded by the `minPacketLen` (244) check at `handle:101`.
  On-link unauthenticated UDP. The real entry point is the UDP receive loop
  `serveMulti` at `internal/plugins/dhcpserver/register.go:185`, which calls `h.handle(pkt)` (`register.go:202`)
  and then `logExchange(log, pkt, resp, serverIPs)` (`register.go:208`; `logExchange` defined at
  `register.go:224` with its own `len(req) < minPacketLen` guard at `register.go:225`).
  `handle` is the only attacker-facing UDP decoder in the tree without a regression fuzzer.

Add Go fuzz harnesses for each, seeded with truncated/oversized/zero-length inputs, registered in
`mk/test-fuzz.mk` alongside the existing targets, and listed in `docs/functional-tests.md`.

## Required Reading

### Architecture Docs
- [ ] `mk/test-fuzz.mk` - how existing fuzz targets are declared and run
  → Constraint: new targets are registered by adding one `$(GO_TEST) -fuzz=<Name> -fuzztime=10s -timeout=60s <exact-pkg-path>` line each. Discovery is manual enumeration, not a glob.
  → Constraint: use an **exact** package path (no `/...`). Each target package has a `yang/` subpackage, and `/...` triggers Go fuzz's "matches more than one package" error (see the file's own comment and the `./internal/core/bgp/nlri` exact-path precedent, lines 35-38).
- [ ] `ai/rules/testing.md` - fuzz/regression test conventions; "Back-Fill New Test Types (BLOCKING)"
  → Constraint: a fuzz corpus seed must include the boundary cases (truncated, oversized, zero-length).
  → Constraint: this spec IS the back-fill of the fuzz technique to three existing parsers; that rule mandates covering the existing code, not only new code.
- [ ] `docs/functional-tests.md` (Fuzz Testing, line 1615) - the operator-facing fuzz target list
  → Constraint: the "Fuzz Target Areas" table and the target count must include the three new targets.

**Key insights:**
- Sibling spec `plan/spec-improve-8-fuzz-decode-context.md` (ready) adds capability/MP-NLRI decode-context fuzzing to the BGP path — different parsers; this spec covers BMP/RADIUS/DHCP.
- Model harness house style for a packet parser is a per-package `fuzz_test.go` with a `seedCorpus()` helper and a `f.Fuzz(func(t, data []byte){ ... if err != nil { return }; assert invariants })` body (`internal/component/bfd/packet/fuzz_test.go`, `internal/plugins/vrrp/packet/fuzz_test.go`).
- `docs/functional-tests.md:1617` claims "54 fuzz targets" but `mk/test-fuzz.mk` already enumerates 60 and 69 distinct `func Fuzz` names exist — the doc count is already stale. This spec bumps the registered set by 3; note the pre-existing drift but do not expand scope to reconcile it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/bmp/tlv.go` - `DecodeTLV(buf []byte, off int) (TLV, int, error)` (:77), `DecodeTLVs(buf []byte, off, end int) ([]TLV, error)` (:103)
  → Constraint: bounds-safe today. `DecodeTLV` guards `len(buf)-off < TLVHeaderSize` (negative when `off > len(buf)`, so still caught) and `end > len(buf)` before slicing `buf[off+4:end]`. Precondition: `off >= 0` (satisfied by callers and any harness passing off=0).
- [ ] `internal/component/radius/attr.go` - `DecodeVSA(data []byte) (uint32, uint8, []byte, error)` (:122)
  → Constraint: bounds-safe today. Guards `len(data) < 6` and `vendorLen < 2 || 4+vendorLen > len(data)`; the slice `data[6:4+vendorLen]` is valid because `vendorLen >= 2` implies `4+vendorLen >= 6` and the guard ensures `4+vendorLen <= len(data)`. Input shape is the attribute value AFTER the outer Type+Length (real callers pass `val` / `data[2:]`).
- [ ] `internal/plugins/dhcpserver/handler.go` - `handle`, `parseMsgType`, `parseOptionAddr`, `extractMAC`, `minPacketLen`, `buildReply`, `safeAppendOption`
  → Constraint: bounds-safe today. `handle` returns nil unless `len(pkt) >= minPacketLen` (244); all subsequent fixed-offset reads are `< 244`; option loops guard (`i < len(opts)-2`, `i+6 <= len(opts)`); `extractMAC` caps `hlen` at 16; `buildReply`/`buildNak` write into a fresh 1500/300-byte buffer via `safeAppendOption`'s limit guard. `handle` is a **method** on `*dhcpHandler`, not a free function (bears on A-1).
- [ ] `internal/plugins/dhcpserver/register.go` - `serveMulti` (:185) receive loop, `logExchange` (:224)
  → Constraint: real receive path reads ≤1500 bytes, calls `h.handle(pkt)` then `logExchange(log, pkt, resp, serverIPs)`. `logExchange` re-parses the same packet for logging (`parseMsgType`, `extractMAC`, `parseOptionAddr`, `msgTypeName`) behind its own `len < minPacketLen` guard.
- [ ] `internal/plugins/dhcpserver/handler_test.go` - `newTestServer`, `buildDiscover`, `buildRequest`
  → Constraint: in-package tests build a handler via `newDHCPHandler(subnetConfig{Prefix: 192.168.1.0/24, ...}, serverIP, pxeConfig{})`; the fuzz harness reuses this exact shim for the DHCP handler.
- [ ] `internal/component/bfd/packet/fuzz_test.go`, `internal/plugins/vrrp/packet/fuzz_test.go` - model harnesses
  → Constraint: match the `seedCorpus()` + `f.Add(seed)` + `f.Fuzz` + `if err != nil { return }` + invariant-assertion shape.
- [ ] `mk/test-fuzz.mk` - existing fuzz target list and runner wiring (60 targets today)

**Behavior to preserve:**
- All three parsers' current (verified bounds-safe) behavior — fuzzers assert no panic and structural invariants, not a behavior change.
- The existing fuzz targets and their corpora; the existing exported signatures the fuzzers call.

**Behavior to change:**
- Add three new fuzz targets, register them in `mk/test-fuzz.mk`, and list them in `docs/functional-tests.md`; fix a parser only if a fuzzer finds a defect.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **BMP:** bytes from a configured monitoring station arrive as a BMP message; `msg.go` decoders slice out the TLV region and call `DecodeTLVs(buf, off, end)`.
- **RADIUS:** a RADIUS server reply's vendor-specific attribute value reaches the L2TP `authradius` client, which calls `radius.DecodeVSA(val)`.
- **DHCP:** an on-link UDP datagram (≤1500 bytes) reaches `serveMulti`, which calls `h.handle(pkt)` then `logExchange(...)`.

### Transformation Path
1. Raw bytes reach the parser (`DecodeTLVs`/`DecodeTLV`, `DecodeVSA`, or DHCP `handle`+`logExchange`).
2. The parser walks length-prefixed fields (TLV header+length; VSA vendor-length; DHCP option code+length loop).
3. A fuzz target feeds arbitrary/truncated/oversized/zero-length bytes and asserts no panic plus a structural invariant on any accepted result (see AC table).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Network ↔ parser | raw bytes to decode function / handler method | [ ] |
| Parser ↔ fuzz gate | new `Fuzz*` targets registered in `mk/test-fuzz.mk` and listed in `docs/functional-tests.md` | [ ] |

### Integration Points
- `mk/test-fuzz.mk` (registration, exact package paths), the three decode entry points, the fuzz corpus directories (`testdata/fuzz/<Name>/`), `docs/functional-tests.md` (target list).

### Architectural Verification
- [ ] No bypassed layers (BMP fuzz drives `DecodeTLVs`, the loop the receiver uses; DHCP fuzz drives `handle` + `logExchange`, the two receive-path consumers; RADIUS fuzz drives `DecodeVSA`, the exact function the authradius client calls)
- [ ] No duplicated functionality (reuse the `seedCorpus()`+`f.Fuzz` pattern and the in-package `newDHCPHandler`/`buildDiscover` helpers)
- [ ] Registration over hardcoding — new fuzz targets are registered in the fuzz gate and doc list, not run ad hoc (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The BMP and RADIUS decoders are callable in isolation with a `[]byte` (+ int offsets); the DHCP `handle` needs a constructed `*dhcpHandler` shim | `DecodeTLV(buf,off)`/`DecodeTLVs(buf,off,end)` and `DecodeVSA(data)` are free funcs; `handle` is a method on `*dhcpHandler` built by `newDHCPHandler` (handler.go:81, handler_test.go `newTestServer`) | DHCP harness needs the in-package handler shim, not a bare function call | read each signature; confirmed DHCP shim exists in-package | confirmed (DHCP resolved via in-package `newDHCPHandler` shim; A-1 as written is partially broken for DHCP) |
| A-2 | The parsers are bounds-safe today (fuzz confirms, not fixes) | code reasoning per function (Current Behavior → Constraint annotations for tlv.go:77/103, attr.go:122, handler.go:100/367/386/456) | A fuzzer finds a live panic (then it becomes a real bug fix under R-1) | run each fuzzer briefly during implement | **confirmed (empirical)**: 15s `make ze-fuzz-one` per target executed 0.9M–1.8M inputs each with zero crashes (BMP 921k, RADIUS 1.2M, DHCP 1.79M); seed corpora pass as normal tests. No reachable panic found. |
| A-3 | New targets are discovered by the gate only if enumerated with an exact package path in `mk/test-fuzz.mk` | `mk/test-fuzz.mk` comment + `./internal/core/bgp/nlri` exact-path precedent; each package has a `yang/` subpackage | `/...` path errors with "matches more than one package"; target silently not run | add each with exact path; run `make ze-fuzz-one` per target | **confirmed**: `go list` shows bmp/yang, radius/yang, dhcpserver/yang siblings; all three registered with exact paths; `make ze-fuzz-one` discovered and ran each (no "matches more than one package"). |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A fuzzer immediately finds a reachable panic | fuzz crash on first run | Treat as a real defect: add the crashing seed to the corpus and fix the parser in this spec |
| R-2 | DHCP `handle` mutates handler state (`pool.reserve`, `leases.add`, `pool.markUnavailable`); reusing one handler across millions of inputs exhausts the pool and shrinks coverage | later inputs stop reaching `buildReply` (allocate returns ok=false) | Build a fresh `*dhcpHandler` per fuzz iteration inside the `f.Fuzz` closure for determinism |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| fuzz gate runs BMP TLV target | -> | `DecodeTLVs`/`DecodeTLV` never panic | `FuzzDecodeBMPTLV` |
| fuzz gate runs RADIUS VSA target | -> | `DecodeVSA` never panics | `FuzzDecodeRADIUSVSA` |
| fuzz gate runs DHCP handle target | -> | DHCP `handle` (+`logExchange`) never panic | `FuzzDHCPHandle` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `FuzzDecodeBMPTLV` seeded with truncated/oversized/zero-length TLVs | no panic; on any accepted TLV, `len(t.Value) == int(t.Length)` and consumed bytes stay within `len(data)` |
| AC-2 | `FuzzDecodeRADIUSVSA` seeded with truncated/oversized/short VSAs | no panic; on success, `len(value) == vendorLen-2` and `4+vendorLen <= len(data)` (value never escapes input) |
| AC-3 | `FuzzDHCPHandle` seeded with truncated/exactly-244/oversized/malformed option streams | no panic; on any non-nil reply, `resp[0] == opReply`; `logExchange` on the same input never panics |
| AC-4 | All three targets | registered in `mk/test-fuzz.mk` with exact package paths and run by `make ze-fuzz-test`; listed in `docs/functional-tests.md` Fuzz Target Areas |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `FuzzDecodeBMPTLV` | `internal/component/bgp/plugins/bmp/fuzz_test.go` | AC-1 | |
| `FuzzDecodeRADIUSVSA` | `internal/component/radius/fuzz_test.go` | AC-2 | |
| `FuzzDHCPHandle` | `internal/plugins/dhcpserver/fuzz_test.go` | AC-3 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| BMP TLV length | 0..65535 | fits remaining buffer | N/A | length > remaining buffer (truncated value) |
| RADIUS vendorLen (data[5]) | 2..255 | `4+vendorLen == len(data)` | vendorLen < 2 | `4+vendorLen > len(data)` |
| DHCP packet length | 0..oversized | `>= minPacketLen` (244) | `< 244` (silently dropped) | `> 1500` (still handled) |

### Functional Tests
Test infrastructure only; no user-facing features. Regression fuzz harnesses verified by `make ze-fuzz-test`; no `.ci` functional test applies.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Per-package `fuzz_test.go` file | `<file>_fuzz_test.go` (skeleton's original) | Matches the closest packet-parser models (bfd, vrrp). File name is irrelevant to discovery (registration is by `-fuzz=<Name>` + package path), so pick house style. |
| BMP harness drives `DecodeTLVs(data,0,len(data))` and asserts per-TLV `len(Value)==Length`, plus a direct `DecodeTLV(data,0)` boundary call | fuzz `DecodeTLV` alone with a fixed off | `DecodeTLVs` is the loop the real receiver (`msg.go`) uses; driving it exercises framing over arbitrary bytes while still hitting the `DecodeTLV` primitive. |
| RADIUS seed shape = attribute value after outer Type+Length (`EncodeVSA(...)[2:]`) | full attribute incl. Type+Length | `DecodeVSA` documents and real callers pass "everything after the outer Type+Length" (attr.go:120); seeds must match that shape. |
| DHCP harness builds a **fresh** `*dhcpHandler` per iteration via the in-package `newDHCPHandler` shim (192.168.1.0/24), calls `handle(data)` then `logExchange(discardLogger, data, resp, serverIPs)` | reuse one handler across iterations; fuzz `parseMsgType`/`extractMAC` directly | Fresh handler avoids pool/lease state accumulation (R-2). Calling both `handle` and `logExchange` covers every sub-parser through the two real receive-path consumers rather than testing them below the 244-byte guard they never see in production. |
| Register each target with an **exact** package path in `mk/test-fuzz.mk` | `/...` suffix | Each package has a `yang/` subpackage; `/...` triggers Go fuzz's "matches more than one package" error (mk comment + nlri precedent, A-3). |
| Seed corpora built from existing in-package encoders/builders (`WriteTLV`, `EncodeVSA`, `buildDiscover`/`buildRequest`) plus hand-crafted malformed cases | hardcoded hex blobs | Reuses verified builders (bfd `seedCorpus` model) and guarantees at least one seed reaches the deep accept path. |

## Files to Modify
- `mk/test-fuzz.mk` - register the three new fuzz targets (exact package paths)
- `docs/functional-tests.md` - add the three targets to the Fuzz Target Areas table and bump the count (discovery update)
- `internal/component/bgp/plugins/bmp/tlv.go` - only if a fuzzer finds a defect
- `internal/component/radius/attr.go` - only if a fuzzer finds a defect
- `internal/plugins/dhcpserver/handler.go` - only if a fuzzer finds a defect

## Files to Create
- `internal/component/bgp/plugins/bmp/fuzz_test.go`
- `internal/component/radius/fuzz_test.go`
- `internal/plugins/dhcpserver/fuzz_test.go`

## Implementation Steps

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add the three `Fuzz*` targets (minimal bodies calling the real entry point), register in `mk/test-fuzz.mk` with exact package paths, list in `docs/functional-tests.md`, confirm the gate discovers each via `make ze-fuzz-one FUZZ=<Name> PKG=<exact-path>`.
   - Tests: `FuzzDecodeBMPTLV`, `FuzzDecodeRADIUSVSA`, `FuzzDHCPHandle`
   - Files: the three `fuzz_test.go`, `mk/test-fuzz.mk`, `docs/functional-tests.md`
   - Verify: each target runs (not "matches more than one package"); A-3 resolved.
2. **Phase: seed corpora** — add truncated/oversized/zero-length seeds for each, built from `WriteTLV` / `EncodeVSA` / `buildDiscover`/`buildRequest` plus malformed variants (bfd `seedCorpus` model).
   - Verify: seeds compile and at least one reaches the deep accept path (BMP TLV with Value; VSA success; DHCP `buildReply`).
3. **Phase: invariant assertions** — add the AC-1..AC-3 structural invariants after `if err != nil { return }` (or after nil-reply checks for DHCP).
   - Verify: assertions hold on all seeds.
4. **Phase: run + triage** — run each briefly (`make ze-fuzz-one ... TIME=30s`); if a crash is found, add the seed under `testdata/fuzz/<Name>/` and fix the parser here (R-1).
5. **Full verification** — `make ze-verify` plus a bounded `make ze-fuzz-test` run.
6. **Complete spec** — audit tables, `plan/learned/NNN-<name>.md`, two-commit closure.

### Discovery-Update Obligation (`ai/rules/discovery-updates.md`)
- Source of truth: `mk/test-fuzz.mk` (exact-path enumeration) — the fuzz gate.
- Operator doc: `docs/functional-tests.md` Fuzz Target Areas table + count.
- Rule reinforced: `ai/rules/testing.md` "Back-Fill New Test Types" — this spec back-fills the fuzz technique to three pre-existing parsers.
- No new make target, test format, or runtime dependency is introduced (targets live inside the existing `ze-fuzz-test` runner), so no `ai/INDEX.md` / hook-mapping / doctor updates are required.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Three targets exist, seeded, invariant-asserted, and registered |
| Correctness | Each fuzz target calls the real decode entry point (BMP `DecodeTLVs`, RADIUS `DecodeVSA`, DHCP `handle`+`logExchange`) |
| Registration | Exact package paths in `mk/test-fuzz.mk`; each runs via `make ze-fuzz-one` (A-3) |
| Discovery updates | `docs/functional-tests.md` lists the new targets and the count is bumped (`ai/rules/discovery-updates.md`) |
| Back-fill | Fuzz technique applied to existing parsers, not only new code (`ai/rules/testing.md`) |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] `make ze-fuzz-test` runs the new targets
- [ ] Registration over hardcoding respected (exact package paths)
- [ ] Discovery update done (`docs/functional-tests.md`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary seeds (zero-length, truncated, oversized) present

**TDD evidence.** "Tests FAIL first" is N/A for this spec by design: the three parsers are
verified bounds-safe (A-2), so there is no red state to reach — this is regression protection,
not a crash fix (Task, R-1). The honest equivalent is recorded instead:
- Tests written: `FuzzDecodeBMPTLV`, `FuzzDecodeRADIUSVSA`, `FuzzDHCPHandle` with seed corpora +
  invariant assertions (AC-1..AC-3).
- Seeds PASS as normal tests: `go test -run 'Fuzz...' ./…/bmp ./…/radius ./…/dhcpserver` → 3× `ok`.
- Live fuzz PASS: 15s `make ze-fuzz-one` per target executed 0.9M–1.8M inputs, zero crashes.
- Boundary seeds present: zero-length, truncated (<min), exactly-min, oversized in every corpus
  (see `bmpTLVSeeds`/`radiusVSASeeds`/`dhcpSeeds`).

## Security Review Checklist
| Concern | Finding for this spec |
|---------|----------------------|
| New attack surface | None. Change is test-only (three `*_test.go` fuzz files + a makefile registration + a doc). No production code path added or altered. |
| Untrusted-input bounds (the parsers themselves) | These fuzzers ARE the check. BMP TLV, RADIUS VSA, and DHCP `handle`+`logExchange` drive attacker/semi-trusted network bytes; 0.9M–1.8M adversarial inputs each ran without a panic or over-read (A-2). |
| Injection / path traversal / crypto misuse | N/A — the harnesses decode wire bytes only; no shell, filesystem, or crypto surface. |
| DoS / unbounded allocation in the harness | Bounded: fresh `*dhcpHandler` per iteration with `defer leases.stop()` (R-2); no goroutine started at construction; no unbounded growth. |
| Information leakage | `logExchange` is driven with `slogutil.DiscardLogger()`, so fuzz iterations emit nothing. |

## Deliverables Checklist
| Deliverable | Verification method | Evidence |
|-------------|---------------------|----------|
| `FuzzDecodeBMPTLV` harness | `go test` + `ze-fuzz-one` | `internal/component/bgp/plugins/bmp/fuzz_test.go`; seed run `ok`; 921k execs, 0 crash |
| `FuzzDecodeRADIUSVSA` harness | `go test` + `ze-fuzz-one` | `internal/component/radius/fuzz_test.go`; seed run `ok`; 1.2M execs, 0 crash |
| `FuzzDHCPHandle` harness | `go test` + `ze-fuzz-one` | `internal/plugins/dhcpserver/fuzz_test.go`; seed run `ok`; 1.79M execs, 0 crash |
| Gate registration (exact paths) | grep `mk/test-fuzz.mk`; `ze-fuzz-one` discovers each | 3 lines added; no "matches more than one package" |
| Discovery doc update | `make ze-doc-test` | `docs/functional-tests.md` count 54→57, table row added, 3 source anchors resolve; doc-test PASSED |

## Documentation Update Checklist
| Category | Update? | File / detail |
|----------|---------|---------------|
| Feature list / user guide / config syntax / CLI reference / API-RPC / plugin SDK / wire format / RFC compliance / comparison table | No | No user-facing feature, config, CLI, RPC, wire, or RFC-coverage change — this is internal regression test infrastructure. |
| Test infrastructure | **Yes** | `docs/functional-tests.md` Fuzz Testing section: headline count 54→57, "Receiver/server parsers" table row (`FuzzDecodeBMPTLV`, `FuzzDecodeRADIUSVSA`, `FuzzDHCPHandle`), 3 `<!-- source: … -->` anchors. |
| Architecture design | No | No architectural change; grep of `docs/` for the three changed test files finds no stale anchor. |

## Goal Validation
| Goal (Task) | Evidence |
|-------------|----------|
| BMP receiver TLV parser gains a regression fuzzer | `FuzzDecodeBMPTLV` drives `DecodeTLVs`/`DecodeTLV`, registered in the gate, 921k execs clean |
| RADIUS reply VSA parser gains a regression fuzzer | `FuzzDecodeRADIUSVSA` drives `DecodeVSA`, registered, 1.2M execs clean |
| DHCP server packet path gains a regression fuzzer | `FuzzDHCPHandle` drives `handle`+`logExchange` (the two receive-path consumers), registered, 1.79M execs clean |
| Targets discoverable by the gate + listed for operators | 3 exact-path lines in `mk/test-fuzz.mk`; `docs/functional-tests.md` Fuzz Target Areas updated |

## Pre-Commit Verification
- **Changed-package tests (green):** `go test ./…/bmp/... ./…/radius/... ./…/dhcpserver/...` → all `ok`.
- **Fuzz gate (green):** `make ze-fuzz-one` per target ran 0.9M–1.8M inputs, 0 crashes (A-2).
- **Lint (green):** `make ze-lint-changed` → 0 issues.
- **Docs (green):** `make ze-doc-test` → PASSED (new source anchors resolve).
- **Full `make ze-verify-changed` = red, but entirely environmental / known-red** (this is an
  unprivileged sandbox; per `plan/known-failures.md:280` such reds are NOT structural):
  `web:unknown` (all ~48 web tests — no browser harness), `ospf/ospfv3` timeouts, `appliance:vpp`
  timeout, `policy` mismatch, `l2tp session-stopccn-cascade` (documented known-red,
  `known-failures.md:430`), and `bmp-locrib` (netlink EPERM `operation not permitted` +
  collector `connection refused`, `known-failures.md:233-281`). NONE is in a package this spec
  changed in a way that alters runtime; all changes are additive test-only files + a makefile
  registration + a doc. Verification scoped to changed per `ai/rules/git-safety.md` (Known-Red
  Full Verify: Scope to Changed).

## Review Gate
Pre-checks: `make ze-validate` → all checks passed; `python3 scripts/dev/audit-test-relaxation.py`
→ clean (0 changed test files; all three fuzz files are new additions, no test deleted/weakened).

### Run 1 (initial)
| Severity | Finding | Location | Action |
|----------|---------|----------|--------|
| NOTE | `docs/functional-tests.md` Fuzz Target Areas table sums to 57 while `mk/test-fuzz.mk` enumerates 63 — pre-existing 6-target drift unrelated to this spec (Notes) | `docs/functional-tests.md:1617` | Flagged to user; no change (this spec does not reconcile it) |

Result: **0 BLOCKER, 0 ISSUE, 1 NOTE**. Wiring (fuzz targets reachable via the gate registration),
logic (all three harness invariants traced + 0.9M–1.8M-exec clean fuzz runs), allocation (fixed
seed sizes; per-iteration handler is the documented R-2 tradeoff), removed-behavior (pure additions),
and RFC (no protocol behavior added) all clean. Gate satisfied.

## Notes
- Skeleton captured from the 2026-07-16 repository audit; deepened to design on 2026-07-16 after reading each decoder, its real callers, and the bfd/vrrp model harnesses. All three parsers verified bounds-safe today by code reasoning; this is regression protection. Sibling: `plan/spec-improve-8-fuzz-decode-context.md`.
- Correctness fix vs skeleton: the fuzz gate target is `make ze-fuzz-test` (mk/test-fuzz.mk:12), not `make ze-fuzz`.
- Pre-existing drift observed (not in scope to reconcile): `docs/functional-tests.md` said "54 fuzz targets" (== the sum of its own Fuzz Target Areas table) while `mk/test-fuzz.mk` enumerated 60 — a pre-existing 6-target gap. **Done:** bumped the doc headline + table by 3 (→ 57, table stays internally consistent) and added the three targets; did NOT reconcile the 6-target gap (now table 57 vs makefile 63). Flagged to the user for a follow-up doc-sync fixit if desired.
- Implementation note (2026-07-17): three fuzz files added, registered with exact package paths (all three packages have a `yang/` sibling, confirmed by `go list`). `handle` sub-parsers (`parseMsgType`/`parseOptionAddr`) are only bounds-safe behind the caller's `len>=244` guard, so the DHCP harness drives `handle`+`logExchange` (both guarded), never the sub-parsers directly. A-2 and A-3 confirmed (see Assumptions).
