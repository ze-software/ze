# ZeBGP - Claude Instructions

## SESSION START (MANDATORY)

1. **Read `.claude/ESSENTIAL_PROTOCOLS.md`** - core rules
1. **Read `plan/CLAUDE_CONTINUATION.md`** - current priorities and state
3. **Run `git status`** - if modified files exist, ASK user before proceeding

## CURRENT PRIORITY

Review ExaBGP alignment plan: `plan/exabgp-alignment.md`

## KEY RULES

- **TDD:** Tests MUST exist and FAIL before implementation
- **ExaBGP:** Check `../src/exabgp/` before implementing BGP features
- **Verify:** Run `make test` before claiming success
- **Plans:** Go in `plan/`, protocols go in `.claude/`
- **Terse:** Emoji-prefixed status lines, no fluff
