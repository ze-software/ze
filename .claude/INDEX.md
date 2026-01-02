# ZeBGP Documentation Index

## Architecture Docs

Read when working on specific areas:

| Area | Doc |
|------|-----|
| Wire formats | `zebgp/wire/MESSAGES.md` |
| NLRI types | `zebgp/wire/NLRI.md` |
| Attributes | `zebgp/wire/ATTRIBUTES.md` |
| Capabilities | `zebgp/wire/CAPABILITIES.md` |
| UPDATE building | `zebgp/UPDATE_BUILDING.md` |
| Memory pools | `zebgp/POOL_ARCHITECTURE.md` |
| Zero-copy | `zebgp/ENCODING_CONTEXT.md` |
| ExaBGP mapping | `zebgp/EXABGP_CODE_MAP.md` |
| ExaBGP compat | `zebgp/EXABGP_COMPATIBILITY.md` |
| FSM | `zebgp/behavior/FSM.md` |
| API | `zebgp/api/ARCHITECTURE.md` |
| Config syntax | `zebgp/config/SYNTAX.md` |

## Rules (auto-loaded by path)

| Rule | Applies To |
|------|------------|
| `rules/tdd.md` | `**/*.go` |
| `rules/go-standards.md` | `**/*.go` |
| `rules/rfc-compliance.md` | `pkg/bgp/**/*.go` |
| `rules/git-safety.md` | `*` |

## Edge Cases

| Topic | Doc |
|-------|-----|
| ASN4 handling | `zebgp/edge-cases/AS4.md` |
| ADD-PATH | `zebgp/edge-cases/ADDPATH.md` |
| Extended messages | `zebgp/edge-cases/EXTENDED_MESSAGE.md` |

## Reference

- Current state: `plan/CLAUDE_CONTINUATION.md`
- RFC folder: `rfc/`
- ExaBGP: `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/`
