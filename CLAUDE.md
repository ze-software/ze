# ZeBGP - Claude Instructions

## Commands
- `make test` - Run unit tests
- `make lint` - Run golangci-lint
- `make functional` - Run functional tests (80 tests)

## Workflow
0. **DESIGN FIRST** - Search for existing code. Extend, don't duplicate. Think deeply.
1. For BGP code: read RFC from `rfc/` folder first
2. Write test, see it FAIL, implement, see it PASS (TDD)
3. Run `make test && make lint && make functional` before claiming done
4. Only commit when explicitly requested

## Key Rules
- **Design before code** - Search codebase first. Reuse/extend existing code. Think deeply before implementing.
- **TDD MANDATORY** - Test must exist and fail before implementation
- **RFC compliance** - BGP code must follow RFCs, add `// RFC NNNN` comments
- **Verify before claiming** - Paste command output as proof
- **Git safety** - Never commit/push without explicit request

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

## Style
- Terse, emoji-prefixed status lines
- No fluff, no reassurance
- Paste command output as proof
