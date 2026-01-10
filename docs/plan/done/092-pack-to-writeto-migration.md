# Spec: pack-to-writeto-migration

**Status:** BACKFILL - Implementation completed Jan 2026

## Task
Complete migration from allocating `Pack()` methods to zero-allocation `WriteTo()` pattern across all packages. Extends work from specs 073 (buffer-writer) and 075 (nlri-writeto-zero-alloc).

## Required Reading
- [x] `.claude/zebgp/wire/BUFFER_WRITER.md` - Zero-alloc architecture
- [x] `.claude/zebgp/ENCODING_CONTEXT.md` - Context-dependent encoding
- [x] `docs/plan/done/073-spec-buffer-writer.md` - Original WriteTo design
- [x] `docs/plan/done/075-nlri-writeto-zero-alloc.md` - NLRI WriteTo implementation

**Key insights:**
- WriteTo writes directly to pre-allocated buffers (zero heap allocation)
- Len() methods enable accurate buffer pre-allocation
- Extended-length boundary (255→256 bytes) requires flag change per RFC 4271

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestASPathWriteTo` | `pkg/bgp/attribute/aspath_test.go` | ASPath encoding matches Pack() |
| `TestAttributeWriteTo` | `pkg/bgp/attribute/attribute_test.go` | All attribute types |
| `TestCommunityWriteTo` | `pkg/bgp/attribute/community_test.go` | Community/ExtComm/LargeCommunity |
| `TestLenWriteTo` | `pkg/bgp/attribute/len_writeto_test.go` | Len() accuracy, extended-length boundary |
| `TestUpdateWriteTo` | `pkg/bgp/message/update_test.go` | Full UPDATE message encoding |
| `TestReactorWriteTo` | `pkg/reactor/reactor_test.go` | Session buffer integration |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| Existing 37 tests | `qa/tests/` | Regression - all pass with new encoding |

## Files Modified

### Core Encoding
| File | Changes |
|------|---------|
| `pkg/bgp/attribute/origin.go` | Add Len() method |
| `pkg/bgp/attribute/aspath_test.go` | +244 lines: WriteTo vs Pack tests |
| `pkg/bgp/attribute/attribute_test.go` | +247 lines: all attribute WriteTo tests |
| `pkg/bgp/attribute/community_test.go` | +315 lines: community WriteTo tests |
| `pkg/bgp/attribute/len_writeto_test.go` | +263 lines: Len accuracy, boundary tests |

### Message Building
| File | Changes |
|------|---------|
| `pkg/bgp/message/update.go` | Add AttributesSizeWithContext() |
| `pkg/bgp/message/update_build.go` | Migrate builders to WriteTo |
| `pkg/bgp/message/update_split.go` | Use WriteTo in splitting |
| `pkg/bgp/message/update_test.go` | +63 lines: UPDATE WriteTo tests |

### RIB Integration
| File | Changes |
|------|---------|
| `pkg/rib/commit.go` | Use WriteAttrTo |
| `pkg/rib/grouping.go` | Use WriteTo |
| `pkg/rib/outgoing.go` | Use WriteTo |
| `pkg/rib/route.go` | Add Len/WriteTo helpers |
| `pkg/rib/store.go` | Use WriteTo |
| `pkg/rib/update.go` | Use WriteTo |

### Reactor/Session
| File | Changes |
|------|---------|
| `pkg/reactor/peer.go` | +51 lines: WriteTo in peer sending |
| `pkg/reactor/reactor.go` | +476 lines: full WriteTo integration |
| `pkg/reactor/reactor_test.go` | +593 lines: comprehensive tests |
| `pkg/reactor/session.go` | +86 lines: session buffer management |

### Other
| File | Changes |
|------|---------|
| `cmd/zebgp/encode.go` | Use WriteTo in CLI |
| `pkg/plugin/commit_manager.go` | Use WriteTo |
| `pkg/plugin/text.go` | Use WriteTo |
| `pkg/cbor/base64.go`, `hex.go` | Add WriteTo helpers |
| `pkg/bgp/nlri/ipvpn.go` | Add RD WriteTo |

## Key Implementation Details

### AttributesSizeWithContext()
New function for accurate buffer pre-allocation:
```go
func AttributesSizeWithContext(attrs []attribute.Attribute, ctx *context.EncodingCtx) int
```

### Extended-Length Boundary Testing
Comprehensive tests for RFC 4271 Section 4.3:
- Attributes exactly 255 bytes (regular length)
- Attributes exactly 256 bytes (extended length flag)
- Large AS_PATH with >255 ASNs

### Offset Parameter Correctness
All WriteTo methods accept offset parameter for writing into middle of buffers:
```go
func (a *Attribute) WriteTo(buf []byte, off int, ctx *EncodingCtx) (int, error)
```

## RFC References
- RFC 4271 Section 4.3 - Extended Length flag auto-set when >255 bytes
- RFC 7911 - ADD-PATH encoding via WriteNLRI() with PackContext
- RFC 6793 - ASN4-aware encoding via WriteToWithContext()

## Statistics
- **Total changes:** +2,623 / -131 lines
- **New test lines:** ~1,600
- **Files modified:** 31

## Commit
`f0d1201` - refactor: migrate Pack() to zero-allocation WriteTo() across packages

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (initial)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes (37 tests)

### Documentation
- [x] Required docs read
- [x] RFC references added
- [x] Extends specs 073, 075

### Completion
- [x] Spec moved to `docs/plan/done/092-pack-to-writeto-migration.md`
