# Claude Continuation State

**Last Updated:** 2025-12-20

---

## CURRENT PRIORITY

**1. Annotate existing code with RFC references**
   - Plan: `plan/rfc-annotation.md`
   - Document any RFC violations found

**2. Create implementation tasks from 26 ALIGN items**
   - Source: `plan/exabgp-alignment.md`
   - Merge with RFC violations from step 1

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
