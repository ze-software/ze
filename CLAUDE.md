# ZeBGP - Claude Instructions

## MANDATORY READING (use Read tool on these files)

1. `.claude/ESSENTIAL_PROTOCOLS.md` - Session rules, TDD, verification
2. `.claude/INDEX.md` - Navigation, find what docs to load
3. `plan/CLAUDE_CONTINUATION.md` - Current state, priorities

## RULE ZERO: THE SYSTEM IS MANDATORY

```
╔═══════════════════════════════════════════════════════════════════════════════╗
║                                                                               ║
║   This .claude folder IS the system. It was designed to prevent mistakes.     ║
║                                                                               ║
║   - The system comes BEFORE any user request                                  ║
║   - The system comes BEFORE your judgment about efficiency                    ║
║   - The system is NOT optional, NOT guidelines, NOT suggestions               ║
║                                                                               ║
║   When you start: Execute the system. Then respond to the user.               ║
║                                                                               ║
║   THE WORD IS: SYSTEM                                                         ║
║                                                                               ║
╚═══════════════════════════════════════════════════════════════════════════════╝
```

## SESSION START (MANDATORY)

```
┌─────────────────────────────────────────────────────────────────┐
│  Run `git status` - if modified, ASK user before proceeding     │
│                                                                 │
│  DO NOT SKIP READING ESSENTIAL_PROTOCOLS.md                     │
└─────────────────────────────────────────────────────────────────┘
```

## BEFORE ANY IMPLEMENTATION (BLOCKING)

```
┌─────────────────────────────────────────────────────────────────┐
│  USE /prep <task> BEFORE WRITING ANY CODE                       │
│                                                                 │
│  /prep forces:                                                   │
│  1. Reading INDEX.md to know WHAT docs to load                  │
│  2. Loading architecture docs into context                      │
│  3. Reading ACTUAL SOURCE CODE (not just docs)                  │
│  4. Identifying patterns to follow                              │
│  5. Context Loading Verification block (CANNOT skip)            │
│                                                                 │
│  Without /prep = designing without understanding = BAD CODE     │
└─────────────────────────────────────────────────────────────────┘
```

## CONTEXT LOADING (THE REAL PROBLEM)

```
┌─────────────────────────────────────────────────────────────────┐
│  YOU CANNOT DESIGN GOOD CODE WITHOUT CONTEXT                    │
│                                                                 │
│  BAD: Read task → Design → Implement                            │
│  GOOD: Read task → Load docs → Read source → Learn patterns →   │
│        Verify context loaded → THEN design → Implement          │
│                                                                 │
│  The /prep command enforces this. Do not skip steps.            │
└─────────────────────────────────────────────────────────────────┘
```

## KEY RULES

- **`INDEX.md`:** Read `.claude/INDEX.md` to find what docs to load for your task
- **`/prep`:** Run `/prep <task>` before any implementation
- **Context:** Read source code, not just docs. Show file:line numbers.
- **Patterns:** Identify existing patterns BEFORE designing new code
- **TDD:** Tests MUST exist and FAIL before implementation
- **ExaBGP:** Check `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/` before implementing BGP features
- **Verify:** Run `make test && make lint` before claiming success
- **Terse:** Emoji-prefixed status lines, no fluff

## CURRENT PRIORITY

Review ExaBGP alignment plan: `plan/exabgp-alignment.md`
