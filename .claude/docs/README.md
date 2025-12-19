# ZeBGP Documentation Index

Project documentation and session notes.

---

## Project Reference

| Document | Purpose |
|----------|---------|
| `../ZE_IMPLEMENTATION_PLAN.md` | Complete implementation plan |
| `../zebgp/CODEBASE_ARCHITECTURE.md` | Code organization |
| `../zebgp/DATA_FLOW_GUIDE.md` | BGP data flow |
| `../zebgp/WIRE_FORMAT_PATTERNS.md` | Zero-copy patterns |

---

## Documentation Placement

| Type | Location |
|------|----------|
| Implementation plans | Project root (`ZE_IMPLEMENTATION_PLAN.md`) |
| Protocol docs | `.claude/` |
| Codebase reference | `.claude/zebgp/` |
| Session notes | `.claude/docs/` |
| Active work | `plan/` (when created) |

---

## ExaBGP Reference

The Python implementation is in `../main/`.

Key reference paths:
- `../main/qa/encoding/` - Encoding test vectors
- `../main/qa/decoding/` - Decoding test vectors
- `../main/src/exabgp/bgp/message/` - Message implementation reference

---

**Last Updated:** 2025-12-19
