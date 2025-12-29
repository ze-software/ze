# Context Loading Protocol

**Purpose:** Detailed instructions for loading context before implementation.
**Used by:** `/prep` command Phase 2

---

## Why This Matters

Claude tends to:
1. Jump into implementation without understanding existing code
2. Design in isolation instead of learning project patterns
3. Use generic solutions instead of project-specific ones

**This protocol FORCES context loading before any design work.**

---

## Step 1: Load Protocol Rules

Always load these with the Read tool:
```
.claude/ESSENTIAL_PROTOCOLS.md
.claude/QUICK_REFERENCE.md
```

---

## Step 2: Load Task-Specific Docs

**Use INDEX.md Quick Navigation table** to find which docs to load for your task.

Read the TL;DR section at the top of each doc first. Read full doc if needed.

---

## Step 3: Read Source Code

**CRITICAL:** Reading docs alone is insufficient. You MUST read source code.

For EACH architecture doc loaded, identify and READ the source files mentioned:

```
📖 Read: pkg/api/server.go:403-424 (OnUpdateReceived)
📖 Read: pkg/bgp/nlri/inet.go:1-100 (INET struct, PackNLRI)
```

Show specific line numbers. Summarize what the code does.

---

## Step 4: Check ExaBGP Reference

If task involves BGP protocol code:
```bash
ls /Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/bgp/message/
```

Read the relevant ExaBGP Python file to understand expected behavior.

---

## Step 5: Verification Block (MANDATORY)

Output this block. **If you cannot fill it completely, load more context.**

```
╔═══════════════════════════════════════════════════════════════════════════════╗
║  CONTEXT LOADING VERIFICATION                                                 ║
╠═══════════════════════════════════════════════════════════════════════════════╣
║                                                                               ║
║  Protocol Rules:                                                              ║
║  ✅ ESSENTIAL_PROTOCOLS.md - [key rules]                                      ║
║  ✅ QUICK_REFERENCE.md - [key patterns]                                       ║
║                                                                               ║
║  Architecture Docs:                                                           ║
║  ✅ [doc1] - [what you learned]                                               ║
║  ✅ [doc2] - [what you learned]                                               ║
║                                                                               ║
║  Source Code (with line numbers):                                             ║
║  ✅ pkg/xxx/file.go:123-145 - [what this code does]                           ║
║  ✅ pkg/yyy/file.go:67-89 - [what this code does]                             ║
║                                                                               ║
║  ExaBGP Reference (if applicable):                                            ║
║  ✅ exabgp/bgp/.../file.py - [how ExaBGP handles this]                        ║
║                                                                               ║
║  Patterns Identified:                                                         ║
║  • [Pattern 1 used in this area]                                              ║
║  • [Pattern 2 that new code must follow]                                      ║
║                                                                               ║
╚═══════════════════════════════════════════════════════════════════════════════╝
```

---

## Anti-Patterns

❌ Reading INDEX.md but not the docs it points to
❌ Reading docs but not source code
❌ Reading source without identifying patterns
❌ Designing before verification block is complete
❌ Claiming "context loaded" without specific file:line references
