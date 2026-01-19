# ZeBGP TODO

## Spec Status Overview

| Spec | Status | Action |
|------|--------|--------|
| spec-parser-unification.md | ❌ NOT IMPLEMENTED | Implement or abandon |
| spec-async-api-parser.md | ⏸️ PLACEHOLDER | Design needed |
| spec-writeto-bounds-safety.md | ✅ COMPLETE | Move to done/ |
| spec-api-rr.md | 🟡 PARTIAL | Complete or document scope |
| spec-api-sync.md | ✅ COMPLETE | Move to done/ |
| spec-rfc9234-role.md | ❌ NOT IMPLEMENTED | Implement |
| spec-api-plugin-commands.md | ✅ DONE | Move to done/ |
| spec-api-command-serial.md | ✅ IMPLEMENTED | Move to done/ |
| spec-adjribout-memory-profiling.md | ❌ Not Started | Schedule or remove |
| spec-context-full-integration.md | ⏸️ Ready | Implement when needed |
| spec-static-route-updatebuilder.md | ⚠️ Partially Obsolete | Review/archive |
| spec-unified-handle-nlri.md | ⏸️ Ready | Implement when needed |
| spec-api-capability-contract.md | ❓ Unknown | Review status |
| spec-attribute-context-wire-container.md | ❓ Unknown | Review status |
| spec-rfc7606-validation-cache.md | ❓ Unknown | Review status |

---

## ❌ Critical: Incomplete Specs

### 1. spec-parser-unification.md - NOT IMPLEMENTED

**Goal:** Unify API and config parsing for `update` commands via shared tokenizer interface.

**Prerequisite:** `new-syntax.md` - ✅ COMPLETED (docs/plan/done/089-new-syntax.md)

**Status:** Design doc only. Core architecture not built.

#### Spec Checklist (ALL UNCHECKED)

```
[ ] Token types defined
[ ] API tokenizer works
[ ] Config adapter works
[ ] ParseUpdate() handles text encoding
[ ] ParseUpdate() handles wire encodings
[ ] API integrated
[ ] Config integrated
[ ] Old code removed
[ ] make test passes
[ ] make lint passes
[ ] make functional passes
```

#### Missing Components

| Planned File | Status | Notes |
|--------------|--------|-------|
| `internal/parse/token.go` | ❌ | Tokenizer interface never created |
| `internal/parse/api_tokenizer.go` | ❌ | API uses `[]string` directly |
| `internal/parse/config_adapter.go` | ❌ | No adapter for config tokenizer |
| `internal/parse/update.go` | ❌ | No unified `ParseUpdate(Tokenizer)` |
| `internal/parse/attributes.go` | ❌ | Attrs parsed inline in update_text.go |
| `internal/parse/nlri.go` | ❌ | NLRI parsed inline in update_text.go |
| `internal/parse/family.go` | ❌ | Family parsing inline |
| `internal/parse/wire.go` | ❌ | Wire parsing in update_wire.go |
| `internal/parse/update_test.go` | ❌ | Tests in internal/api/ instead |

#### What Exists Instead

| Component | Location | Notes |
|-----------|----------|-------|
| `ParseUpdateText()` | `internal/api/update_text.go:430` | Takes `[]string`, no tokenizer |
| `ParseUpdateWire()` | `internal/api/update_wire.go:32` | Takes `[]string`, no tokenizer |
| `tokenize()` | `internal/api/command.go:294` | Splits input to `[]string` |
| Config tokenizer | `internal/config/tokenizer.go` | Concrete struct, not interface |
| Community parsing | `internal/parse/community.go` | Only shared piece |

#### Planned Token Interface (Never Built)

```go
// internal/parse/token.go - SPEC DESIGN, NOT IMPLEMENTED

type TokenType int

const (
    TokenEOF TokenType = iota
    TokenWord      // unquoted word
    TokenString    // quoted string
    TokenLBrace    // {
    TokenRBrace    // }
    TokenLBracket  // [
    TokenRBracket  // ]
    TokenSemicolon // ;
)

type Token struct {
    Type  TokenType
    Value string
    Line  int
    Col   int
}

type Tokenizer interface {
    Next() Token
    Peek() Token
}
```

#### Planned vs Actual Architecture

**Spec design:**
```
Input (API or Config)
        ↓
Format-specific Tokenizer (implements interface)
        ↓
Common Token Stream: []Token
        ↓
Shared Parser: ParseUpdate(tok Tokenizer)
        ↓
UpdateCommand struct
```

**Actual implementation:**
```
API Input (string)
        ↓
tokenize() → []string
        ↓
ParseUpdateText([]string) or ParseUpdateWire([]string)
        ↓
UpdateTextResult struct

Config Input (string)
        ↓
*Tokenizer (concrete) → []Token
        ↓
Separate parsing (no update command support)
```

#### Planned Error Messages (Unified format with line/col)

```
line 1: expected encoding (text|hex|b64|cbor), got "foo"
line 1: invalid next-hop: invalid IP address
line 1: expected 'attr', got "nlri"
line 1: unknown attribute: foobar
line 1: prefix family mismatch: 2001:db8::/32 is IPv6, expected IPv4
```

**Actual:** Errors lack line/col context since API parses `[]string` not tokens.

#### Planned Implementation Phases (ALL INCOMPLETE)

| Phase | Status | Description |
|-------|--------|-------------|
| Phase 1: Token Foundation | ❌ | Create token.go, api_tokenizer.go |
| Phase 2: Shared Parser (TDD) | ❌ | Write ParseUpdate() with tests |
| Phase 3: API Integration | ❌ | Update handleUpdate() to use shared parser |
| Phase 4: Config Adapter | ❌ | Create config_adapter.go |
| Phase 5: Cleanup | ❌ | Remove duplicate code |

#### Planned Tests (Never Written)

```go
// SPEC DESIGN - TESTS NOT IMPLEMENTED
func TestParseUpdate_TextEncoding(t *testing.T)
func TestParseUpdate_HexEncoding(t *testing.T)
func TestParseUpdate_WatchdogInNLRI(t *testing.T)
func TestParseUpdate_ScalarDelConditional(t *testing.T)
```

#### Impact

- API and Config cannot share update parsing
- No path to config-file-based route announcements using same syntax
- Duplicate effort if config update support needed later
- No unified error format with line/column numbers

#### Resolution Options

1. **Complete the spec** - Build tokenizer interface, adapt both parsers (~2-3 days)
2. **Abandon unification** - Move spec to `docs/plan/abandoned/`, document decision
3. **Partial unification** - Extract more shared code to `internal/parse/` without full tokenizer interface

---

### 2. spec-rfc9234-role.md - NOT IMPLEMENTED

**Status:** "Ready for Implementation" but no code exists.

**What's Missing:**

| Component | Status |
|-----------|--------|
| `internal/bgp/capability/role.go` | ❌ |
| `CapRole` constant | ❌ |
| `RoleCapability` struct | ❌ |
| `internal/bgp/attribute/otc.go` | ❌ |
| `AttrOTC` (Type 35) | ❌ |
| Peer role storage in reactor | ❌ |
| RouteTag with SourceRole | ❌ |

**Purpose:** Enables API-driven routing policy without attribute parsing.

---

### 3. spec-adjribout-memory-profiling.md - Not Started

**Status:** "Not Started"

---

## ✅ Complete But Not Moved to done/

These specs should be moved to `docs/plan/done/`:

| Spec | Next Number |
|------|-------------|
| spec-writeto-bounds-safety.md | 093 |
| spec-api-sync.md | 094 |
| spec-api-plugin-commands.md | 095 |
| spec-api-command-serial.md | 096 |

**Command to move:**
```bash
LAST=$(command ls -1 docs/plan/done/ 2>/dev/null | sort -n | tail -1 | cut -c1-3)
test -z "$LAST" && LAST=0
# Then: mv docs/plan/spec-<name>.md docs/plan/done/$(printf "%03d" $((LAST + 1)))-<name>.md
```

---

## 🟡 Partial Implementation

### spec-api-rr.md

**Implementation exists:** `internal/api/rr/` (server.go, rib.go, peer.go)

**Unclear:** Is this complete or partial? Spec not moved to done/.

---

## ⏸️ Placeholders / Ready for Implementation

| Spec | Notes |
|------|-------|
| spec-async-api-parser.md | PLACEHOLDER - needs design |
| spec-context-full-integration.md | "Ready for phased implementation" |
| spec-unified-handle-nlri.md | "Ready for implementation" |

---

## ⚠️ Partially Obsolete

### spec-static-route-updatebuilder.md

**Status:** "Partially Obsolete" - review if still relevant.

---

## ❓ Unknown Status

Need review:
- spec-api-capability-contract.md
- spec-attribute-context-wire-container.md
- spec-rfc7606-validation-cache.md

---

## Technical Debt

### 1. Functional test reporter message merging bug (Priority: Low)

**Location:** `internal/test/runner/record.go`

- All messages in check.ci use index `1:`, causing them to merge
- Report shows wrong "EXPECTED MESSAGE 1" (shows last message only)
- Actual testpeer comparison is correct (order-agnostic)
- Only affects diagnostic output, not test correctness

### 2. check.ci order documentation mismatch (Priority: Low)

- CI file shows: EOR → EOR → routes
- ZeBGP sends: routes → EOR → EOR
- Both are valid BGP (testpeer is order-agnostic)
- CI comments are misleading

### 3. Multiple inherit not supported (Priority: Low - design limitation)

- `inherit` is defined as `Leaf(TypeString)`, not a List
- Second `inherit` statement overwrites first
- Workaround: use single template with multiple api blocks

**Related spec:** `docs/plan/spec-api-test-features.md`

---

## Code Quality Issues

### Linter Warning

```
go.mod:33:28 - golang.org/x/term should be direct (go mod tidy)
```

**Fix:** `go mod tidy`

---

## Uncommitted Changes

```
Modified:
  M internal/bgp/message/chunk_mp_nlri_test.go
  M internal/bgp/message/update_split_test.go
  M docs/plan/spec-parser-unification.md

Untracked:
  ? .claude/commands/code-review.md
  ? package-lock.json
  ? docs/plan/spec-async-api-parser.md
  ? docs/plan/spec-writeto-bounds-safety.md
  ? scripts/analyze-writeto.go
  ? scripts/check-buffer-overflow.sh
  ? scripts/migrate-api-syntax.py
  ? yolo
```

**Action:** Review and commit or discard.

---

## Housekeeping Tasks

1. [ ] Run `go mod tidy` to fix linter warning
2. [ ] Move completed specs to `docs/plan/done/`
3. [ ] Review unknown status specs
4. [ ] Decide on spec-parser-unification.md (implement/abandon)
5. [ ] Clean up untracked files
6. [ ] Commit or discard modified test files
