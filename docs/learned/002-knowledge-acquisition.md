# 002 — Knowledge Acquisition

## Objective

Systematically study ExaBGP source code and produce reference architecture documents covering wire format, JSON output, configuration syntax, API commands, and behavioral edge cases.

## Decisions

Mechanical documentation effort, no design decisions.

## Patterns

- Each ExaBGP subsystem was mapped directly to a `docs/architecture/` reference document.
- MUP, VPLS, MVPN, and RTC NLRI types were deliberately deferred (less common, not needed for initial implementation).

## Gotchas

- JSON encoder structure (Phase 3) and some config section docs were not completed during this sprint — those tasks remained open.

## Files

- `docs/architecture/wire/` — MESSAGES.md, CAPABILITIES.md, ATTRIBUTES.md, NLRI.md, NLRI_EVPN.md, NLRI_FLOWSPEC.md, NLRI_BGPLS.md, QUALIFIERS.md
- `docs/architecture/api/` — JSON_FORMAT.md, JSON_EXAMPLES.md, COMMANDS.md, PROCESS_PROTOCOL.md
- `docs/architecture/config/` — TOKENIZER.md, SYNTAX.md, ENVIRONMENT.md
- `docs/architecture/behavior/` — FSM.md, SIGNALS.md
- `docs/architecture/edge-cases/` — AS4.md, EXTENDED_MESSAGE.md, ADDPATH.md
