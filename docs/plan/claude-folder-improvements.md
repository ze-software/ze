# Plan: .claude Folder Improvements

## Problem Statement

Claude does not read protocol files "on demand" when tasks require them. The current system says "load GIT_VERIFICATION_PROTOCOL.md for git operations" but Claude skips this step.

**Root cause:** Reading protocols is a separate optional step that can be skipped.

**Solution:** Make protocol reading a mandatory part of planning via `/prep` skill, and consolidate protocols so fewer files exist to miss.

---

## Solution Overview

| Component | Purpose |
|-----------|---------|
| `/prep` skill | Forces protocol reading during planning |
| Protocol consolidation | Fewer files = less to miss |
| README.md as index | Pure file listing, no duplicated content |
| CLAUDE.md enforcement | Require /prep for non-trivial tasks |

---

## Part 1: `/prep` Skill Specification

### Purpose

When user types `/prep <task description>`, the skill:
1. **Reads** ESSENTIAL_PROTOCOLS.md (always)
2. **Extracts** 3-5 KEY rules per relevant section (not entire content)
3. **Detects** task type from description
4. **Reads** relevant additional protocols
5. **Extracts** 3-5 KEY rules from those files
6. **Embeds** extracted rules directly in plan
7. **Writes** plan to `docs/plan/<task-name>.md`

### Default Rules (ALL tasks get these)

Every task, regardless of type, gets:
- TDD core rule: "Tests MUST exist and FAIL before implementation"
- Verification rule: "Run `make test && make lint` before claiming done"
- Work preservation: "NEVER discard uncommitted work without asking"

### Task Type Detection (Additional Rules)

| Keywords in Task | Additional Protocols to Read |
|------------------|------------------------------|
| implement, add, create, feature | TDD_ENFORCEMENT.md, CODING_STANDARDS.md |
| git, commit, push, merge | (embedded in ESSENTIAL after consolidation) |
| refactor, rename, move | (embedded in ESSENTIAL after consolidation) |
| test, fix test, failing | TESTING_PROTOCOL.md, CI_TESTING.md |
| API, endpoint, command | zebgp/api/COMMANDS.md |
| FSM, state, session | zebgp/behavior/FSM.md |
| wire, message, parse, encode | zebgp/wire/*.md, RFC_DOCUMENTATION_PROTOCOL.md |
| config, configuration | zebgp/config/*.md |

### Rule Extraction Examples

**From TDD_ENFORCEMENT.md (519 lines) → Extract 3-5 rules:**
```
- Tests MUST exist and FAIL before implementation code exists
- Every test MUST have VALIDATES and PREVENTS documentation
- Run `go test -race ./pkg/...` for fast feedback during dev
- Run `make test && make lint` before claiming done
```

**From CODING_STANDARDS.md (491 lines) → Extract 3-5 rules:**
```
- Error handling: NEVER ignore errors
- Use context.Context for cancellation
- NEVER use panic() for normal error handling
- golangci-lint must pass
```

**NOT this (bad - too much):**
```
[Entire 519-line TDD_ENFORCEMENT.md pasted here]
```

### Output Format

```markdown
# Spec: <task-name>

## Task
<original task description>

## Embedded Protocol Requirements

### Core Rules (from ESSENTIAL_PROTOCOLS.md)
- <actual rules extracted, not references>
- <specific to this task type>

### TDD Requirements (from TDD_ENFORCEMENT.md)
- <if implementation task>

### Additional Requirements
- <from other relevant protocols>

## Files to Read First
- <list of files to understand before implementing>

## Implementation Steps
1. <step with specific verification>
2. <step with specific verification>
...

## Verification
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] <task-specific checks>
```

### Key Design Decisions

1. **Embed rules, don't reference** - The plan contains actual rules so Claude sees them when reading the plan later

2. **Task-type detection is keyword-based** - Simple and predictable

3. **Output goes to docs/plan/ directory** - Follows existing file location conventions

4. **Checkboxes for verification** - Explicit completion criteria

### Skill File Location

`.claude/commands/prep.md`

---

## Part 2: Protocol Consolidation

### Files to MERGE into ESSENTIAL_PROTOCOLS.md

| File | Lines | Reason to Merge |
|------|-------|-----------------|
| GIT_VERIFICATION_PROTOCOL.md | 210 | Git safety is always essential |
| ERROR_RECOVERY_PROTOCOL.md | 216 | Recovery rules needed anytime |
| MANDATORY_REFACTORING_PROTOCOL.md | 207 | Refactoring rules needed anytime |
| SESSION_END_CHECKLIST.md | 125 | Session management is essential |
| PRE_FLIGHT_CHECKLIST.md | 71 | Already duplicated in ESSENTIAL |

**Total lines to merge:** ~829 lines

**New ESSENTIAL_PROTOCOLS.md size:** ~1500 lines (currently 689)

This is large but acceptable because:
- It's read once at session start
- Contains ALL protocols, not just some
- No more "load on demand" that gets skipped

**REQUIRED: Add Table of Contents at top:**
```markdown
## Table of Contents
1. [Session Start Checklist](#session-start-checklist)
2. [File Locations](#file-locations)
3. [TDD Rules](#tdd-rules)
4. [Git Safety](#git-safety)
5. [Error Recovery](#error-recovery)
6. [Refactoring Protocol](#refactoring-protocol)
7. [Session End Checklist](#session-end-checklist)
8. [Verification Rules](#verification-rules)
9. [Communication Style](#communication-style)
10. [Forbidden Phrases](#forbidden-phrases)
```

TOC enables quick navigation in large file.

### Files to KEEP Separate (Reference Material)

| File | Lines | Reason to Keep |
|------|-------|----------------|
| CODING_STANDARDS.md | 491 | Detailed Go style lookup |
| TDD_ENFORCEMENT.md | 519 | Detailed TDD workflow, heavily cross-referenced |
| TESTING_PROTOCOL.md | 355 | Test command reference |
| CI_TESTING.md | 348 | CI command reference |
| RFC_DOCUMENTATION_PROTOCOL.md | 257 | Wire format documentation guide |
| DOCUMENTATION_PLACEMENT_GUIDE.md | 181 | Where to put docs (lookup) |
| FILE_NAMING_CONVENTIONS.md | 198 | Naming patterns (lookup) |

These are **lookup references**, not protocols to follow. The `/prep` skill reads them when relevant.

### README.md: Convert to Index Only

**Current:** 233 lines with duplicated session start instructions

**New:** ~50 lines, pure index:
```markdown
# .claude/ Directory Index

## Core Protocol
- **ESSENTIAL_PROTOCOLS.md** - All protocols consolidated (read at session start)

## Reference Files
- CODING_STANDARDS.md - Go style guide
- TDD_ENFORCEMENT.md - Detailed TDD workflow
- TESTING_PROTOCOL.md - Test commands
- CI_TESTING.md - CI commands
- RFC_DOCUMENTATION_PROTOCOL.md - Wire format docs
- DOCUMENTATION_PLACEMENT_GUIDE.md - Doc placement
- FILE_NAMING_CONVENTIONS.md - Naming patterns

## Subdirectories
- commands/ - Skills (/prep, etc.)
- hooks/ - Auto-linter, etc.
- output-styles/ - Communication style
- backups/ - Work preservation
- zebgp/ - Codebase reference docs
```

**No duplicated content. Just a map.**

### Files to DELETE

| File | Lines | Reason |
|------|-------|--------|
| PRE_FLIGHT_CHECKLIST.md | 71 | Merged into ESSENTIAL |

### zebgp/ Subdirectory

**Keep as-is.** These are codebase reference docs, not protocols:
- api/ - API documentation
- behavior/ - Runtime behavior docs
- config/ - Configuration docs
- edge-cases/ - Special case handling
- wire/ - Wire format docs
- EXABGP_*.md - ExaBGP compatibility docs
- POOL_*.md - Pool architecture docs
- etc.

The `/prep` skill reads these when task keywords match.

---

## Part 3: Resulting Structure

```
.claude/
├── ESSENTIAL_PROTOCOLS.md      # ~1500 lines, ALL protocols consolidated
├── CODING_STANDARDS.md         # Reference: Go style
├── TDD_ENFORCEMENT.md          # Reference: TDD workflow
├── TESTING_PROTOCOL.md         # Reference: Test commands
├── CI_TESTING.md               # Reference: CI commands
├── RFC_DOCUMENTATION_PROTOCOL.md # Reference: Wire format docs
├── DOCUMENTATION_PLACEMENT_GUIDE.md # Reference: Doc placement
├── FILE_NAMING_CONVENTIONS.md  # Reference: Naming
├── settings.json
├── settings.local.json
├── commands/
│   └── prep.md                 # NEW: /prep skill
├── hooks/
│   ├── auto_linter.sh
│   └── README.md
├── output-styles/
│   └── zebgp.md
├── backups/
├── docs/
│   └── README.md
└── zebgp/                      # Codebase reference (unchanged)
    ├── api/
    ├── behavior/
    ├── config/
    ├── edge-cases/
    ├── wire/
    └── *.md
```

**Converted:**
- README.md → Pure index (~50 lines)

**Deleted:**
- PRE_FLIGHT_CHECKLIST.md (merged)
- GIT_VERIFICATION_PROTOCOL.md (merged)
- ERROR_RECOVERY_PROTOCOL.md (merged)
- MANDATORY_REFACTORING_PROTOCOL.md (merged)
- SESSION_END_CHECKLIST.md (merged)

---

## Implementation Steps

### Phase 1: Create /prep Skill
- [ ] Create `.claude/commands/` directory
- [ ] Write `.claude/commands/prep.md` skill
- [ ] Test skill with sample task

### Phase 2: Consolidate Protocols
- [ ] Add Table of Contents to top of ESSENTIAL_PROTOCOLS.md
- [ ] Merge GIT_VERIFICATION_PROTOCOL.md into ESSENTIAL_PROTOCOLS.md
- [ ] Merge ERROR_RECOVERY_PROTOCOL.md into ESSENTIAL_PROTOCOLS.md
- [ ] Merge MANDATORY_REFACTORING_PROTOCOL.md into ESSENTIAL_PROTOCOLS.md
- [ ] Merge SESSION_END_CHECKLIST.md into ESSENTIAL_PROTOCOLS.md
- [ ] Merge PRE_FLIGHT_CHECKLIST.md content (if any new)
- [ ] Reorganize ESSENTIAL_PROTOCOLS.md sections for clarity

### Phase 3: Cleanup
- [ ] Convert README.md to pure index (remove duplicated content)
- [ ] Delete PRE_FLIGHT_CHECKLIST.md
- [ ] Delete merged protocol files
- [ ] Update any cross-references in remaining files

### Phase 4: Enforce /prep Usage
- [ ] Update CLAUDE.md to add: "For non-trivial tasks, use `/prep` first"
- [ ] Add to ESSENTIAL_PROTOCOLS.md: "/prep requirement for multi-step tasks"

### Phase 5: Verification
- [ ] Test `/prep implement feature` - should embed TDD + coding rules
- [ ] Test `/prep fix git issue` - should embed git rules (from ESSENTIAL)
- [ ] Test `/prep refactor code` - should embed refactoring rules
- [ ] Verify ESSENTIAL_PROTOCOLS.md TOC links work
- [ ] Start new Claude session, verify it reads consolidated file

### Phase 6: Optional Enhancement
- [ ] Consider adding hook to validate plan format
  - Hook runs on Write to `docs/plan/*.md`
  - Checks for "Embedded Protocol Requirements" section
  - Warns if missing

---

## Success Criteria

1. **`/prep` skill works** - Reads protocols, extracts 3-5 key rules, embeds in output
2. **Single consolidated file** - ESSENTIAL_PROTOCOLS.md contains all protocols with TOC
3. **No redundant files** - Each piece of information exists once
4. **README.md is index only** - Pure file listing, no duplicated content
5. **CLAUDE.md enforces /prep** - Non-trivial tasks require /prep first
6. **Default rules always applied** - Every /prep output includes TDD + verification rules

---

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| ESSENTIAL_PROTOCOLS.md too long | TOC at top, clear section headers |
| /prep skill too complex | Start simple, iterate based on usage |
| Cross-references break after deletion | Search and update before deleting |
| Claude ignores /prep and plans manually | CLAUDE.md enforcement + user discipline |
| Rule extraction too verbose | Strict "3-5 rules per protocol" limit with examples |
| Rule extraction misses important rules | Default rules (TDD, verification) always included |

---

## Questions Resolved

1. ~~Keep flat or use subdirectories?~~ → Keep mostly flat, add `commands/` for skills
2. ~~What about empty docs/ directory?~~ → Keep as-is, low priority
3. ~~Merge EXABGP docs?~~ → No, keep in zebgp/ as codebase reference

---

## Next Action

Start with **Phase 1: Create /prep skill** - this is the core innovation that solves the "not reading protocols" problem.
