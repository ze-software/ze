# Ze - Claude Instructions

## ⛔ TOP 5 RULES (READ EVERY SESSION)

**These rules are BLOCKING. Violating them wastes time and money.**

| # | Rule | Why | Check |
|---|------|-----|-------|
| 1 | **Read selected spec FIRST** | Spec contains decisions already made | `cat .claude/selected-spec` → `docs/plan/<name>` |
| 2 | **Read source before writing code** | You will invent conflicting designs without seeing existing code | Read ALL files in spec's "Files to Modify" |
| 3 | **No code without understanding** | Duplicate code, wrong patterns, broken integrations | Can you name 3 related files? |
| 4 | **TDD: Test must FAIL first** | Proves test actually validates something | `go test` shows RED before implementation |
| 5 | **Preserve existing behavior** | Breaking changes waste debugging time | Document current output format BEFORE changing |

**Session start checklist:**
```
1. [ ] Read .claude/selected-spec
2. [ ] Read docs/plan/<spec-name> (if selected)
3. [ ] Read .claude/session-state.md (if exists)
4. [ ] Check git status for modified files
5. [ ] ONLY THEN start working
```

---

## Naming Convention

**"Ze" = "The" with a French accent.** It's a pun.

| Context | Use | Example |
|---------|-----|---------|
| Application name | `ze` | "Start ze BGP daemon" |
| CLI binary | `ze` | `ze bgp server config.conf` |
| BGP config YANG | `ze-bgp-conf` | `module ze-bgp-conf { ... }` |
| BGP JSON format | `ze-bgp` | `"format": "ze-bgp"` |
| Go variables for BGP | `ZeBGPConf*` | `ZeBGPConfYANG` |
| Prose/docs | `Ze` or `ze` | "Ze BGP running" |

**Rule:** Use "ze" where "the" would work grammatically.

## ⛔ Post-Compaction Recovery

**If you see "continued from a previous conversation":** Follow the session start checklist in TOP 5 RULES above, then see `.claude/rules/post-compaction.md` for the full recovery procedure.

## Core Architecture (MUST UNDERSTAND)

**READ `docs/architecture/core-design.md` for full details.**

### System Components

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              ZeBGP ENGINE                                    │
│                                                                             │
│   ┌─────────┐  ┌─────────┐  ┌─────────┐                                    │
│   │ Peer 1  │  │ Peer 2  │  │ Peer N  │   (BGP sessions)                   │
│   │  FSM    │  │  FSM    │  │  FSM    │                                    │
│   └────┬────┘  └────┬────┘  └────┬────┘                                    │
│        │            │            │                                          │
│        └────────────┼────────────┘                                          │
│                     ▼                                                       │
│              ┌─────────────┐                                                │
│              │   Reactor   │  (event loop, BGP cache)                      │
│              └─────────────┘                                                │
└─────────────────────────────────────────────────────────────────────────────┘
                              │                 ▲
          JSON events (down)  │                 │  commands (up)
          + base64 wire bytes │                 │  update/forward/withdraw
                              ▼                 │
═══════════════════════ PROCESS BOUNDARY (stdin/stdout pipes) ════════════════
                              │                 ▲
                              ▼                 │
                      ┌───────────────┐
                      │    Plugin     │  (Go/Python/Rust/etc.)
                      │  (RIB / RR)   │
                      └───────────────┘
```

**Key insight:** Engine passes wire bytes to plugins. Plugins implement RIB, deduplication, policy.

### Peer Context & Negotiated Capabilities

Decoding/encoding BGP messages requires **negotiated capabilities** from OPEN exchange:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Negotiated (per-peer)                        │
│         See internal/bgp/capability/negotiated.go for full struct    │
├─────────────────────────────────────────────────────────────────┤
│ ASN4            bool                 → 4-byte ASN support       │
│ AddPath         map[Family]Mode      → Receive/Send/Both        │
│ ExtendedMsg     bool                 → 65535 byte messages      │
│ ExtendedNextHop map[Family]AFI       → Per-family NH mapping    │
│ Families()      []Family             → Negotiated families      │
│ GracefulRestart *GracefulRestart     → RFC 4724 GR state        │
│ RouteRefresh    bool                 → RFC 2918 support         │
└─────────────────────────────────────────────────────────────────┘
```

**Why it matters:**
- Same wire bytes parse differently based on negotiated caps
- `AS_PATH [00 01 FD E8]` = ASN 65000 (ASN4) or two ASNs 1, 64488 (ASN2)
- NLRI `[00 00 00 01 18 0a 00 00]` = path-id + prefix (ADD-PATH) or two prefixes (no ADD-PATH)

**ContextID:** Identifies encoding context for zero-copy decisions. Same ContextID = same caps = can forward wire bytes unchanged.

**Wire Writing:** All wire types implement `BufWriter` interface:
- `WriteTo(buf, off) int` - write to pre-allocated buffer (caller guarantees capacity)
- `CheckedWriteTo(buf, off) (int, error)` - validates capacity first
- Context-dependent types (NLRI, ASPath) take `*PackContext` for ADD-PATH/ASN4 handling

### BGP UPDATE = Container

```
UPDATE Message (wire bytes)
├── Header (19 bytes)
├── Withdrawn Routes (IPv4 unicast)
├── Path Attributes
│   ├── ORIGIN, AS_PATH, NEXT_HOP, MED, LOCAL_PREF, ...
│   ├── MP_REACH_NLRI (NLRI for non-IPv4-unicast)
│   └── MP_UNREACH_NLRI (withdrawals for non-IPv4-unicast)
└── NLRI (IPv4 unicast only)
```

### WireUpdate vs RIB

```
WireUpdate (transport)              RIB (storage)
┌─────────────────────┐             ┌────────────────────────────────────┐
│ Attributes (shared) │             │ NLRI 10.0.0.0/24 → origin_ref ─────┼─→ Pool: IGP
│ NLRI: 10.0.0.0/24   │   ────▶     │                    aspath_ref ─────┼─→ Pool: [65001]
│ NLRI: 10.0.1.0/24   │             │                    localpref_ref ──┼─→ Pool: 100
│ NLRI: 10.0.2.0/24   │             │ NLRI 10.0.1.0/24 → (same refs) ────┼─→ (shared)
└─────────────────────┘             └────────────────────────────────────┘
```

**Key points:**
- `WireUpdate` = transport (UPDATE bytes, lazy parse via iterators)
- `RIB` = storage (NLRI → attribute refs, NOT WireUpdate)
- Per-attribute-type pools (ORIGIN, AS_PATH, LOCAL_PREF, MED, COMMUNITY, etc.)
- Per-family NLRI pools (`map[Family]*Pool[NLRI]`)
- Next-hop: special encoding (attribute vs MP_REACH_NLRI depending on family)

### API Command Syntax

```
Text mode:   update text origin set igp nhop set 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24
Binary mode: update hex attr set 400101... nlri ipv4/unicast add 180a00
```

Both produce WireUpdate with wire bytes.
- Full syntax: `docs/architecture/api/update-syntax.md`
- Full design: `docs/architecture/core-design.md`

---

## Commands
- `make test` - Run unit tests
- `make lint` - Run golangci-lint
- `make functional` - Run functional tests (80 tests)

## Workflow
0. **UNDERSTAND FIRST** - No coding until you understand the code structure
1. **DESIGN FIRST** - Search for existing code. Extend, don't duplicate. Think deeply.
2. For BGP code: read RFC from `rfc/` folder first
3. Write test, see it FAIL, implement, see it PASS (TDD)
4. Run `make verify` (test + lint + functional) before claiming done
5. Only commit when explicitly requested

## Planning (see `.claude/rules/planning.md` for full details)

**Re-read planning.md before starting AND before asking to commit.**

1. Write spec to `docs/plan/spec-<task>.md`
2. `git add` the spec immediately (track early)
3. Unit tests BEFORE implementation (strict TDD)
4. Functional tests AFTER feature works
5. On completion:
   - Update architecture docs with learnings
   - Update spec with Implementation Summary
   - Move spec to `docs/plan/done/NNN-<name>.md`
   - Commit ALL files together (code + tests + docs + spec)

**Investigation → Test Rule:** If you investigate/debug something, add a test so future devs don't have to re-investigate.

## ⛔ Before Writing ANY Spec

**BLOCKING - ENFORCED AT EVERY START:**

Before writing or editing ANY spec file (`docs/plan/spec-*.md`):

1. **READ the source files that will be modified** - Not docs, the ACTUAL CODE
2. **Document current behavior** - What does the code do NOW?
3. **Preserve behavior by default** - Unless user explicitly says to change it

**Why:** I invented a new JSON format instead of reading `decode.go` and preserving the existing one. This wasted money and broke tests.

**Verification question before spec writing:**
> "Have I read the actual source files listed in 'Files to Modify'? Can I describe what they currently do?"

If NO → READ THE CODE FIRST. Do not proceed.

## Key Rules
- **Read source before spec** - BLOCKING: Read actual code files before writing specs. Document current behavior. Preserve it unless told otherwise.
- **No code without understanding** - BLOCKING: Do not write any code until you have read relevant existing code and understand the architecture. Search, read, analyze BEFORE proposing changes.
- **Design before code** - Search codebase first. Reuse/extend existing code. Think deeply before implementing.
- **TDD MANDATORY** - Test must exist and fail before implementation
- **RFC compliance** - BGP code must follow RFCs, add `// RFC NNNN` comments
- **Verify before claiming** - Paste command output as proof
- **Git safety** - Never commit/push without explicit request

## Spec Selection

Only ONE spec at a time. Track in `.claude/selected-spec`:
```bash
echo "spec-rfc9234-role.md" > .claude/selected-spec  # Select
echo "" > .claude/selected-spec                       # Clear after done
```

**Session state:** Track progress in `.claude/session-state.md` (copy from `.template`).

**Checkboxes are lies.** `[x]` means read in PREVIOUS session. Re-read everything.

## Reference Paths
- ExaBGP: `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/`
- RFCs: `rfc/` directory
- RFC summaries: `rfc/short/`

## Codeberg CLI

Use `tea` for Codeberg interactions (PRs, issues):
```bash
tea pr list                      # List PRs
tea pr create --title "..." --description "..."
tea issue list
tea issue create --title "..."
```

## Architecture Docs
Read when working on specific areas:
- **Core design: `docs/architecture/core-design.md` (START HERE)**
- Buffer-first: `docs/architecture/buffer-architecture.md`
- Wire formats: `docs/architecture/wire/messages.md`
- NLRI types: `docs/architecture/wire/nlri.md`
- Attributes: `docs/architecture/wire/attributes.md`
- Capabilities: `docs/architecture/wire/capabilities.md`
- UPDATE building: `docs/architecture/update-building.md` (Build vs Forward paths)
- Memory pools: `docs/architecture/pool-architecture.md`
- Zero-copy: `docs/architecture/encoding-context.md`
- ExaBGP compat: `docs/exabgp/exabgp-code-map.md`
- FSM: `docs/architecture/behavior/fsm.md`
- API: `docs/architecture/api/architecture.md`
- API Capabilities: `docs/architecture/api/capability-contract.md`
- Config: `docs/architecture/config/syntax.md`

## Style
- Terse, emoji-prefixed status lines
- No fluff, no reassurance
- Paste command output as proof
