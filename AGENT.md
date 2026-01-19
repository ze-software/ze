# ZeBGP - Agent Instructions

## Commands
- `make lint` - Run golangci-lint (26 linters)
- `make test` - Run unit tests (`go test -race -v ./...`)
- `make functional` - Run functional tests (37 tests)

## Workflow - Non-Trivial Features

### Pre-Implementation Checklist (MANDATORY)
```
[ ] 1. Check existing spec: `docs/plan/spec-<task>.md`
[ ] 2. Read `.claude/INDEX.md` for doc navigation
[ ] 3. Scan `docs/plan/spec-*.md` for related specs
[ ] 4. Match task keywords to docs (see table below)
[ ] 5. Read ALL identified architecture docs
[ ] 6. RFC Summary Check (for protocol work):
     a. Identify ALL RFCs needed
     b. Check `docs/rfc/rfcNNNN.md` exists
     c. If missing: `curl -o rfc/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`
     d. Read ALL relevant RFC summaries
[ ] 7. Read source code for affected area
[ ] 8. TDD Planning - identify tests BEFORE implementation
[ ] 9. Present implementation plan to user
[ ] 10. Begin TDD cycle (test fails → implement → test passes)
```

### Keyword → Documentation Mapping

| Keywords in task | Required docs | RFC summaries |
|------------------|---------------|---------------|
| UPDATE, message, build, route, announce | `docs/architecture/UPDATE_BUILDING.md`, `ENCODING_CONTEXT.md` | `rfc4271.md`, `rfc4760.md` |
| attribute, community, AS_PATH, NEXT_HOP | `docs/architecture/wire/ATTRIBUTES.md`, `UPDATE_BUILDING.md` | `rfc4271.md`, `rfc1997.md`, `rfc4360.md` |
| NLRI, prefix, MP_REACH, MP_UNREACH | `docs/architecture/wire/NLRI.md` | `rfc4760.md` |
| capability, OPEN, negotiate | `docs/architecture/wire/CAPABILITIES.md` | `rfc5492.md`, `rfc9072.md` |
| pool, memory, dedup, zero-copy | `docs/architecture/POOL_ARCHITECTURE.md`, `ENCODING_CONTEXT.md` | |
| forward, reflect, wire cache | `docs/architecture/ENCODING_CONTEXT.md`, `UPDATE_BUILDING.md` | |
| FSM, state, session, peer | `docs/architecture/behavior/FSM.md` | `rfc4271.md`, `rfc4724.md` |
| API, command, announce, withdraw | `docs/architecture/api/ARCHITECTURE.md`, `api/CAPABILITY_CONTRACT.md` | |
| config, YAML, load | `docs/architecture/config/SYNTAX.md` | |
| FlowSpec | `docs/architecture/wire/NLRI.md`, `wire/NLRI_FLOWSPEC.md` | `rfc8955.md`, `rfc8956.md` |
| VPN, L3VPN, MPLS-VPN | `docs/architecture/wire/NLRI.md` | `rfc4364.md`, `rfc4659.md`, `rfc8277.md` |
| EVPN | `docs/architecture/wire/NLRI.md`, `wire/NLRI_EVPN.md` | `rfc7432.md`, `rfc9136.md` |
| BGP-LS, link-state | `docs/architecture/wire/NLRI_BGPLS.md` | `rfc7752.md`, `rfc9085.md`, `rfc9514.md` |
| ExaBGP, compatibility | `docs/exabgp/EXABGP_CODE_MAP.md`, `EXABGP_COMPATIBILITY.md` | |
| design, transition, architecture | `docs/architecture/rib-transition.md` | |
| ASN4, AS4, 4-byte AS | `docs/architecture/edge-cases/AS4.md` | `rfc6793.md` |
| ADD-PATH, path-id | `docs/architecture/edge-cases/ADDPATH.md` | `rfc7911.md` |
| extended message | `docs/architecture/edge-cases/EXTENDED_MESSAGE.md` | `rfc8654.md` |
| graceful restart, GR | `docs/architecture/behavior/FSM.md` | `rfc4724.md` |
| route-refresh | | `rfc2918.md`, `rfc7313.md` |
| error handling, notification | | `rfc7606.md`, `rfc9003.md` |
| large community | | `rfc8092.md` |
| extended community, RT | | `rfc4360.md`, `rfc5701.md` |
| role, OTC, route leak | | `rfc9234.md` |
| IPv6 next hop | | `rfc8950.md` |
| labeled unicast, label | | `rfc8277.md`, `rfc3032.md` |

## TDD (Test-Driven Development)

### TDD Cycle
1. Write test with `VALIDATES:` and `PREVENTS:` comments
2. Run test → MUST FAIL (paste output)
3. Write minimum implementation
4. Run test → MUST PASS (paste output)
5. Refactor while green

### Test Documentation Required
```go
// TestFeatureName verifies [behavior].
//
// VALIDATES: [what correct behavior looks like]
// PREVENTS: [what bug this catches]
// REPRODUCES: (for bug fixes) [original issue description]
func TestFeatureName(t *testing.T) { ... }
```

### Round-Trip Testing (Wire Format)
Every pack/unpack MUST pass round-trip:
```go
func TestHeaderRoundTrip(t *testing.T) {
    original := &Header{Marker: Marker, Length: 19, Type: TypeKEEPALIVE}
    packed := original.Pack()
    unpacked, err := ParseHeader(packed)
    require.NoError(t, err)
    assert.Equal(t, original, unpacked)
}
```

### Fuzzing (MANDATORY for Wire Format)
All code parsing external input MUST have fuzz tests:
- Message parsing (untrusted network data)
- Attribute parsing (untrusted network data)
- NLRI parsing (untrusted network data)
- Config tokenizer (user-provided file)
- CLI command parsing (user commands)

```go
// FuzzParseNLRI tests NLRI parsing robustness.
//
// VALIDATES: Parser handles arbitrary bytes without crashing.
// PREVENTS: Remote crash via malformed UPDATE, buffer overflow, panics.
func FuzzParseNLRI(f *testing.F) {
    f.Add([]byte{24, 10, 0, 0})  // Valid: 10.0.0.0/24
    f.Add([]byte{})              // Empty
    f.Add([]byte{33, 10, 0, 0})  // Invalid prefix length
    f.Fuzz(func(t *testing.T, data []byte) {
        _, _ = ParseNLRI(data)  // MUST NOT panic
    })
}
```

## Go Coding Standards

### Required
- Go 1.21+ features (slog, generics)
- `golangci-lint` must pass
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Context for cancellation: `context.Context` as first param

### Error Handling
```go
// ALWAYS wrap errors with context
if err != nil {
    return fmt.Errorf("parsing header: %w", err)
}

// NEVER ignore errors
f, _ := os.Open(path)  // FORBIDDEN
```

### Fail-Early Rule
Configuration/parsing errors MUST propagate immediately:
```go
// GOOD: fail early
if prefix == "" {
    return nil, fmt.Errorf("missing required prefix")
}

// BAD: silent ignore
if prefix == "" {
    prefix = "0.0.0.0/0"  // FORBIDDEN
}
```

### Forbidden
- `panic()` for error handling
- `f, _ := func()` (ignoring errors)
- Global mutable state
- `init()` functions (except registry patterns)

## RFC Compliance

### Before Implementing BGP Features
1. Find RFC in `rfc/` folder
2. If missing: `curl -o rfc/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`
3. Read relevant sections
4. Note MUST/SHOULD/MAY requirements
5. Check ExaBGP reference

### Priority Order
1. **RFC compliance** - Always follow RFC specification
2. **ExaBGP API compatibility** - Match ExaBGP's interface
3. **ExaBGP implementation** - Follow approach when RFC-compliant

### Wire Format Documentation (MANDATORY)
Never modify protocol code without documenting the wire format:
```go
// VPLS represents a VPLS NLRI (RFC 4761 Section 3.2.2)
//
// Wire format (19 bytes):
//
//     0                   1                   2                   3
//     0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    |           Length (2)          |    Route Distinguisher (8)    |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    |          VE ID (2)            |      Label Block Offset (2)   |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
type VPLS struct { ... }
```

### RFC Constraint Comments
When code enforces an RFC rule, document it:
```go
// RFC 4271 Section 6.3: "If the UPDATE message is received from an external peer"
// MUST check that AS_PATH first segment is neighbor's AS
if peer.IsExternal() && path.FirstAS() != peer.RemoteAS {
    return ErrInvalidASPath
}
```

## Quality Standards

### Core Principle
**Do the work properly. No shortcuts.**

### Never Disable Linters to Hide Problems
When facing lint issues:
1. **FIX the issue** - this is the only acceptable action
2. **DO NOT** disable linters, checks, or rules to make issues disappear
3. **DO NOT** add exclusion patterns to avoid fixing code

### Acceptable Exclusions
- `fieldalignment` (govet) - confirm with user if silenced
- Test file exclusions for `dupl`, `goconst`, `prealloc`, `gosec`

## Testing Commands

### Linters in `make lint`
| Linter | Checks |
|--------|--------|
| `govet` | Suspicious constructs (printf args, struct tags, etc.) |
| `staticcheck` | Static analysis, bugs, simplifications |
| `errcheck` | Unchecked error returns |
| `gosec` | Security issues |
| `gocritic` | Performance (`hugeParam`, `rangeValCopy`), style, diagnostics |
| `prealloc` | Slice preallocation opportunities |
| `exhaustive` | Missing switch cases |
| `dupl` | Duplicate code blocks |

Full list: `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`, `gocritic`, `gosec`, `misspell`, `unconvert`, `unparam`, `nakedret`, `prealloc`, `noctx`, `bodyclose`, `dupl`, `errorlint`, `exhaustive`, `forcetypeassert`, `goconst`, `godot`, `nilerr`, `nilnil`, `tparallel`, `wastedassign`, `gofmt`, `goimports`

### Individual Commands
```bash
go test -race ./internal/bgp/message/... -v       # Single package
go test -race ./... -run TestName -v          # Single test
go test -race -cover ./...                    # Coverage
go test -bench=. -benchmem ./internal/...          # Benchmarks
go test -fuzz=FuzzParseHeader -fuzztime=30s ./internal/bgp/message/...  # Fuzz
```

## Config Design Rules

### No Version Numbers
- No `version N;` fields in config syntax
- All config changes must be machine-transformable
- Migration framework handles old→new conversion

### Fail on Unknown
ZeBGP MUST reject configs with unknown variables/blocks:
- Unknown key at any level → fail with clear error
- No silent ignore of typos or deprecated fields

## Git Safety

### Commit Rules
- ONLY commit when user explicitly says "commit"
- Run `make test && make lint && make functional` before commit - ALL must pass

### Before Any Commit
```bash
make test && make lint && make functional  # ALL must pass with zero issues
git status              # Review changes
git diff --staged       # Review what's staged
```

### Forbidden Without Explicit Permission
- `git reset` (any form)
- `git revert`
- `git checkout -- <file>`
- `git restore` (to discard changes)
- `git stash drop`
- `git push --force`

## Reference Paths
- ExaBGP: `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/`
- RFCs: `rfc/` directory
- RFC summaries: `docs/rfc/`

## Architecture Docs
Read when working on specific areas:
- Wire formats: `docs/architecture/wire/MESSAGES.md`
- NLRI types: `docs/architecture/wire/NLRI.md`
- Attributes: `docs/architecture/wire/ATTRIBUTES.md`
- Capabilities: `docs/architecture/wire/CAPABILITIES.md`
- UPDATE building: `docs/architecture/UPDATE_BUILDING.md` (Build vs Forward paths)
- Memory pools: `docs/architecture/POOL_ARCHITECTURE.md`
- Zero-copy: `docs/architecture/ENCODING_CONTEXT.md`
- ExaBGP compat: `docs/exabgp/EXABGP_CODE_MAP.md`
- FSM: `docs/architecture/behavior/FSM.md`
- API: `docs/architecture/api/ARCHITECTURE.md`
- API Capabilities: `docs/architecture/api/CAPABILITY_CONTRACT.md`
- Config: `docs/architecture/config/SYNTAX.md`

## Communication Style

### Emoji Reference
| Category | Emoji | Meaning |
|----------|-------|---------|
| **Status** | ✅ ❌ ⏳ ⏸️ ⏭️ 🔄 | Success, Fail, Running, Paused, Skipped, Retry |
| **Priority** | 🔴 🟡 🟢 🔵 ⚪ | High, Medium, Low, Info, Neutral |
| **Quality** | ✨ 🐛 🔧 🚧 💥 ⚠️ 🚨 | New, Bug, Fix, WIP, Breaking, Warning, Critical |
| **Files** | 📁 📄 📝 ➕ ➖ 📋 | Dir, File, Edit, Add, Remove, List |
| **Code** | 🔍 🔬 🏗️ 🧪 📊 🎯 | Search, Analyze, Build, Test, Metrics, Target |
| **Git** | 📝 ⬆️ ⬇️ 🔀 ⏪ 🏷️ | Commit, Push, Pull, Merge, Revert, Tag |
| **Comm** | 💬 💭 💡 ❓ ⁉️ | Prompt, Note, Idea, Question, Confusion |

### Style Rules
- Terse, emoji-prefixed status lines
- Start lines with emoji: `✅ Tests pass`
- Direct statements: "Fixed" "Tests pass" "Found 3 issues"
- Facts, not feelings: "Tests failed. 3 errors in header.go:45, 67, 89"
- Paste command output as proof
- No fluff, no reassurance

### Output Patterns
```
✅ Tests pass (go test: 42, lint: clean)
❌ Tests failed: header.go:45 - undefined: Marker
📁 Modified: header.go, open.go, header_test.go
🧪 Tests: ✅ lint: clean, ✅ go test: 42 passed, ❌ integration: failed
```

## Key Rules Summary
- **Design before code** - Search codebase first. Reuse/extend existing code. Think deeply before implementing.
- **TDD MANDATORY** - Test must exist and fail before implementation
- **RFC compliance** - BGP code must follow RFCs, add `// RFC NNNN` comments
- **Verify before claiming** - Paste command output as proof
- **Git safety** - Never commit/push without explicit request
- **Quality** - Fix lint issues, never disable linters to hide problems
