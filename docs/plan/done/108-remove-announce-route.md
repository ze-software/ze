# Spec: Remove "announce route" References

## Task

Clean up all `announce route` references from the codebase. The command handler was removed in spec 104, but references remain in comments, test data, and documentation.

## Required Reading

### Architecture Docs
- [x] `docs/architecture/api/update-syntax.md` - New syntax reference
- [x] `docs/plan/done/104-remove-announce-route.md` - Previous removal work

**Key insights:**
- `announce route` handler removed, `update text` is canonical syntax
- Migration: `announce route <p> next-hop <nh>` → `update text nhop set <nh> nlri ipv4/unicast add <p>`
- `.ci` file `:cmd:` lines are documentation, not executed (routes come from `.conf` files)

## Scope Analysis

### Category 1: Dead Code/Comments (delete)
| File | Line | Content |
|------|------|---------|
| `internal/plugin/route.go` | 208 | `// NOTE: "announce route" removed...` |
| `internal/plugin/commit_test.go` | 292-323 | `// TestCommit...Route removed...` (5 comments) |

### Category 2: Test Data Comments (update for accuracy)
| File | Line | Content |
|------|------|---------|
| `internal/plugin/server_test.go` | 340 | Test case uses `"announce route"` as example |
| `internal/plugin/command_test.go` | 161 | Test case tokenizes `"announce route 10.0.0.0/24"` |
| `internal/plugin/plugin_test.go` | 248 | Test input `"announce route 10.0.0.0/24"` |

### Category 3: Generic Comments (update wording)
| File | Line | Content |
|------|------|---------|
| `internal/plugin/types.go` | 596 | `// Watchdog pool name for announce routes` |
| `internal/config/bgp.go` | 146 | `// Used by both static and announce route schemas` |
| `internal/test/runner/record.go` | 550 | `// Parse: "1:cmd:announce route..."` |

### Category 4: Test Data `.ci` Files (documentation update)
~30 `.ci` files have `:cmd:announce route ...` lines. These are **documentation only** - the actual test data comes from `.conf` files. Update for documentation accuracy.

### Category 5: Scripts (keep)
| File | Purpose |
|------|---------|
| `scripts/migrate-api-syntax.py` | Migration tool - useful for reference |
| `test/data/scripts/dynamic-1.sh` | Dynamic test script |
| `test/data/scripts/dynamic-1.pl` | Dynamic test script |

### Category 6: Active Documentation (update)
| File | Priority | Lines |
|------|----------|-------|
| `docs/architecture/api/commands.md` | High | 128, 134, 174-175, 194, 200, 332, 547 |
| `docs/architecture/api/architecture.md` | High | 150, 251, 663, 666, 1010 |
| `docs/architecture/api/process-protocol.md` | Medium | 111, 128, 131 |
| `docs/architecture/api/capability-contract.md` | Medium | 159 |
| `docs/architecture/overview.md` | Medium | 519, 716, 1023 |
| `docs/architecture/buffer-architecture.md` | Low | 344 |
| `docs/architecture/edge-cases/addpath.md` | Low | 231 |
| `docs/functional-tests.md` | Low | 144, 311 |
| `docs/test-inventory.md` | Low | 32, 348 |
| `docs/exabgp/exabgp-differences.md` | Low | 53-56, 61 |
| `docs/plan/spec-api-plugin-commands.md` | Low | 52, 57 |
| `docs/plan/plugin-system.md` | Low | 1584 |
| `docs/plan/spec-api-command-serial.md` | Low | 13, 28, 119-120 |
| `docs/plan/spec-plugin-rr.md` | Low | 622, 758 |

### Category 7: Historical Specs (DO NOT MODIFY)
Files in `docs/plan/done/` are historical records. Do not modify.

### Category 8: External RFCs (DO NOT MODIFY)
`rfc/rfc8195.txt` - "announce route" is BGP terminology, not our API.

## Implementation Steps

1. **Baseline verification** - Run `make lint && make test && make functional` (paste output)
2. **Remove dead comments** - Delete NOTE in route.go:208, remove "removed" comments in commit_test.go
3. **Update generic comments** - Fix wording in types.go, bgp.go, record.go
4. **Update test data** - Change example strings in server_test.go, command_test.go, plugin_test.go
5. **Update .ci files** - Migrate `:cmd:` documentation lines to `update text` syntax
6. **Update high-priority docs** - commands.md, architecture.md
7. **Update medium-priority docs** - process-protocol.md, capability-contract.md, overview.md
8. **Update low-priority docs** - remaining documentation files
9. **Final verification** - `make lint && make test && make functional` (paste output)

## Files to Modify

### Code Files
- `internal/plugin/route.go` - Remove line 208
- `internal/plugin/commit_test.go` - Remove lines 292-323
- `internal/plugin/types.go` - Update line 596
- `internal/config/bgp.go` - Update line 146
- `internal/test/runner/record.go` - Update line 550
- `internal/plugin/server_test.go` - Update line 340
- `internal/plugin/command_test.go` - Update line 161
- `internal/plugin/plugin_test.go` - Update line 248

### Test Data Files
~30 `.ci` files in `test/data/encode/` and `test/data/plugin/`

### Documentation Files
~15 files listed in Category 6 above

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| Existing tests | `internal/plugin/*_test.go` | No regressions from comment/string changes |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| encoding tests | `test/data/encode/*.ci` | All 37 tests pass after `:cmd:` updates |
| plugin tests | `test/data/plugin/*.ci` | Plugin tests pass after updates |

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Active docs updated
- [ ] Historical specs untouched

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
