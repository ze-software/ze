# Claude Continuation State

**Last Updated:** 2025-12-21

---

## CURRENT PRIORITY

**Implement ALIGN items from `plan/align-implementation.md`**

31 items across 8 phases (26 original + 5 from RFC violations)

Start with Phase 1 (Critical): 5 items

---

## ACTIVE WORK

### ExaBGP Alignment Review

1. **Comparison report created:** `.claude/zebgp/EXABGP_COMPARISON_REPORT.md`
   - Comprehensive analysis of ZeBGP vs ExaBGP
   - Covers: messages, capabilities, attributes, NLRI, FSM, reactor, config

2. **Alignment plan created:** `plan/exabgp-alignment.md`
   - 35 decision items organized in 9 phases
   - Each item: ALIGN (change ZeBGP) / KEEP (keep ZeBGP) / SKIP (defer)
   - **User must review and mark decisions**

### Phase 1 (Critical) Items:
- RFC 8203/9003 shutdown communication
- Per-message-type length validation
- Extended message size integration
- KEEPALIVE payload validation

---

## RECENT COMMITS

- `4503f2b` Add ExaBGP comparison report and improve session protocols
- `73c8a8d` Add ASN4-aware AS_PATH encoding based on capability negotiation

---

## TEST STATUS

Run `make test` to verify current state.

---

## KEY FILES

| Purpose | File |
|---------|------|
| Alignment plan | `plan/exabgp-alignment.md` |
| Comparison report | `.claude/zebgp/EXABGP_COMPARISON_REPORT.md` |
| This file | `plan/CLAUDE_CONTINUATION.md` |
| Protocols | `.claude/ESSENTIAL_PROTOCOLS.md` |
| TDD rules | `.claude/TDD_ENFORCEMENT.md` |

---

## NOTES

- All code changes require TDD (test first, show failure, implement, show pass)
- Plans go in `plan/`, protocols go in `.claude/`
- Check ExaBGP reference before implementing BGP features
- **ALWAYS run `make test` at the end of work BEFORE requesting a commit**
