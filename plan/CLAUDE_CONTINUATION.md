# Claude Continuation State

**Last Updated:** 2025-12-22

---

## CURRENT STATUS

🔴 **Active Priority:** API Commit-Based Route Batching

See: `plan/api-commit-batching.md`

**Why:** Converting ALL 45 `.run` scripts to use commit-based batching is REQUIRED for `.ci` tests to pass. Without explicit commit semantics, ZeBGP cannot reproduce ExaBGP's UPDATE message grouping.

### Progress (2025-12-22)

**Infrastructure completed:**
- ✅ Process spawning with working directory
- ✅ API server process integration
- ✅ self-check loads API tests from `test/data/api/`
- ✅ Socket path env var (`zebgp_api_socketpath`)
- ✅ testpeer ignores non-raw `.ci` lines

**Current state:**
- API test `fast` runs and sends routes
- Message mismatch: missing LOCAL_PREF, wrong AS_PATH
- Need to convert `.run` scripts to produce correct attributes

**Next:** Convert `fast.run` to match `.ci` expected output.

---

## CRITICAL REVIEW FINDINGS (2025-12-22)

**Major discovery:** Alignment plan was outdated. 7 of 36 items already implemented.

### Items Already Done (No Work Needed)
| Item | Feature | Evidence |
|------|---------|----------|
| 1.1 | RFC 9003 Shutdown Communication | `notification.go:210-249` |
| 1.2 | Per-message-type length validation | `header.go:111-163` |
| 1.3 | RFC 8654 Extended Message validation | `session.go:294-311`, `session.go:590-594` |
| 1.4 | KEEPALIVE payload rejection | `keepalive.go:42-55` |
| 4.2 | AS_PATH auto-split at 255 | `aspath.go:139-178` |
| 4.4 | Large community deduplication | `community.go:228-301` |
| 4.7 | Attribute ordering on send | `origin.go:100-137`, `commit.go` |
| 5.1 | Family validation against negotiated | `session.go:440-526` |
| 3.2 | Hold Time Validation (0 or >=3s) | `session.go:385-401` |
| 8.2 | RFC 7606 Error Recovery | `message/rfc7606.go`, `session.go:validateUpdateRFC7606()` |

---

## HIGH PRIORITY (Test Compatibility)

| Item | Description | Plan |
|------|-------------|------|
| **API Commit System** | Required for all 45 `.run` tests to pass | `api-commit-batching.md` |

## MEDIUM PRIORITY (Functionality)

| Item | Description |
|------|-------------|
| 2.1 | RFC 9072 Extended Optional Parameters |
| 2.2 | Enhanced Route Refresh (RFC 7313) |
| 5.2 | Extended Next-Hop Support |
| 5.3 | MP-NLRI Chunking |

---

## RECENTLY COMPLETED

**Critical Review** - 2025-12-22
- ✅ Verified all Phase 1 claims against code
- ✅ Verified Phase 4-5 claims against code
- ✅ Reviewed KEEP decision rationales
- ✅ Verified ExaBGP claims against source
- ✅ Downloaded RFC 7606
- ✅ Updated `rfc/README.md`
- ✅ Updated `plan/exabgp-alignment.md`

**Phase 3 Internal Refactoring** - **COMPLETE ✅**

Full neighbor→peer terminology unification:
- ✅ config.PeerConfig (was NeighborConfig)
- ✅ reactor.PeerSettings (was Neighbor)
- ✅ All tests updated and passing

**Named Commit System** - **COMPLETE ✅**

Phase 3 API commit commands fully implemented.

---

## REFERENCE DOCS

| Doc | Purpose |
|-----|---------|
| `exabgp-alignment.md` | Review decisions (18 ALIGN, 7 KEEP, 2 SKIP, 9 DONE) |
| `ARCHITECTURE.md` | Codebase architecture overview |

---

## TEST STATUS

✅ **All unit tests pass** (`make test`)
✅ **Lint clean** (`make lint` - 0 issues)

---

## KEY FILES

| Purpose | File |
|---------|------|
| Extended msg validation | `pkg/bgp/message/header.go` |
| RFC 7606 validation | `pkg/bgp/message/rfc7606.go` |
| Session receive path | `pkg/reactor/session.go` |
| RFC 7606 | `rfc/rfc7606.txt` |
| Alignment plan | `plan/exabgp-alignment.md` |
| This file | `plan/CLAUDE_CONTINUATION.md` |
| Protocols | `.claude/ESSENTIAL_PROTOCOLS.md` |

---

## NOTES

- All code changes require TDD (test first, show failure, implement, show pass)
- Plans go in `plan/`, protocols go in `.claude/`
- Check ExaBGP reference before implementing BGP features
- **RFC 7606 implemented** - treat-as-withdraw, attribute-discard, session-reset tactics
- **ALWAYS run `make test && make lint` before requesting a commit**
