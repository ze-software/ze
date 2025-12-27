# ZeBGP - Claude Instructions

@.claude/ESSENTIAL_PROTOCOLS.md
@plan/CLAUDE_CONTINUATION.md

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
│  /prep forces protocol reading and embeds rules in the spec.    │
│  Skipping /prep = skipping protocols = mistakes.                │
└─────────────────────────────────────────────────────────────────┘
```

## KEY RULES

- **`/prep`:** Run `/prep <task>` before any implementation
- **TDD:** Tests MUST exist and FAIL before implementation
- **ExaBGP:** Check `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/` before implementing BGP features
- **Verify:** Run `make test && make lint` before claiming success
- **Plans:** Go in `plan/`, protocols go in `.claude/`
- **Terse:** Emoji-prefixed status lines, no fluff

## CURRENT PRIORITY

Review ExaBGP alignment plan: `plan/exabgp-alignment.md`
