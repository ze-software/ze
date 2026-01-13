# Spec: pack-removal

## Task

Remove deprecated Pack() methods from Message and Attribute types. All callers should use WriteTo() with EncodingContext instead.

## Background

This is a follow-up to `spec-wirewriter-unification.md` which:
- Added WireWriter interface with `Len(ctx)` and `WriteTo(buf, off, ctx)`
- Implemented WireWriter on all Message types
- Kept Pack() for backward compatibility during migration

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/encoding-context.md` - EncodingContext usage

### RFC Summaries
- [ ] `docs/rfc/rfc4271.md` - BGP message formats

**Key insights:**
- Pack() allocates; WriteTo() writes to pre-allocated buffer
- message.Negotiated is duplicate of capability.Negotiated sub-components

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMessageNoPackMethod` | `pkg/bgp/message/message_test.go` | Message interface has no Pack | |
| `TestWriteToReplacePack` | `pkg/bgp/message/message_test.go` | WriteTo produces same output as Pack | |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| existing | `test/data/` | Existing tests validate wire format unchanged | |

## Files to Modify

### Delete
- `pkg/bgp/message/message.go` - Remove `message.Negotiated` struct
- `pkg/bgp/message/message.go` - Remove `message.Family` type

### Modify
- `pkg/bgp/message/message.go` - Remove Pack() from Message interface
- `pkg/bgp/message/keepalive.go` - Remove Pack() method
- `pkg/bgp/message/open.go` - Remove Pack() method
- `pkg/bgp/message/update.go` - Remove Pack() method
- `pkg/bgp/message/notification.go` - Remove Pack() method
- `pkg/bgp/message/routerefresh.go` - Remove Pack() method
- `pkg/reactor/reactor.go` - Use WriteTo instead of Pack
- `pkg/reactor/session.go` - Use WriteTo instead of Pack
- `pkg/reactor/session_test.go` - Use WriteTo instead of Pack
- `pkg/reactor/collision_test.go` - Use WriteTo instead of Pack
- `pkg/rib/commit.go` - Use EncodingContext instead of message.Negotiated

## Files to Create

None - removing deprecated code.

## Implementation Steps

1. **Create helper** - Add `PackMessage(msg Message, ctx *EncodingContext) []byte` helper
2. **Write tests** - Tests verifying WriteTo produces same output as Pack
3. **Run tests** - Verify FAIL (helper not implemented)
4. **Implement helper** - Allocate buffer, call WriteTo
5. **Update callers** - Replace Pack() calls with WriteTo or helper
6. **Run tests** - Verify PASS
7. **Remove Pack** - Delete from interface and implementations
8. **Delete message.Negotiated** - Remove ephemeral shim
9. **Verify all** - `make lint && make test && make functional`

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
- [ ] Required docs read
- [ ] RFC references added to code

### Completion
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together

## Status: NOT STARTED

This spec will be implemented after wirewriter-unification is committed.
