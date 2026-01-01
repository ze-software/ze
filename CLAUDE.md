# ZeBGP - Claude Instructions

## Commands
- `make test` - Run unit tests
- `make lint` - Run golangci-lint
- `make functional` - Run functional tests (37 tests)

## Workflow
1. For BGP code: read RFC from `rfc/` folder first
2. Write test, see it FAIL, implement, see it PASS (TDD)
3. Run `make test && make lint && make functional` before claiming done
4. Only commit when explicitly requested

## Key Rules
- **TDD MANDATORY** - Test must exist and fail before implementation
- **RFC compliance** - BGP code must follow RFCs, add `// RFC NNNN` comments
- **Verify before claiming** - Paste command output as proof
- **Git safety** - Never commit/push without explicit request

## Reference Paths
- ExaBGP: `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/`
- RFCs: `rfc/` directory
- Current state: `plan/CLAUDE_CONTINUATION.md`

## Architecture Docs
Read when working on specific areas:
- Wire formats: `.claude/zebgp/wire/MESSAGES.md`
- NLRI types: `.claude/zebgp/wire/NLRI.md`
- Attributes: `.claude/zebgp/wire/ATTRIBUTES.md`
- Capabilities: `.claude/zebgp/wire/CAPABILITIES.md`
- UPDATE building: `.claude/zebgp/UPDATE_BUILDING.md` (Build vs Forward paths)
- Memory pools: `.claude/zebgp/POOL_ARCHITECTURE.md`
- Zero-copy: `.claude/zebgp/ENCODING_CONTEXT.md`
- ExaBGP compat: `.claude/zebgp/EXABGP_CODE_MAP.md`
- FSM: `.claude/zebgp/behavior/FSM.md`
- API: `.claude/zebgp/api/ARCHITECTURE.md`
- Config: `.claude/zebgp/config/SYNTAX.md`

## Style
- Terse, emoji-prefixed status lines
- No fluff, no reassurance
- Paste command output as proof
