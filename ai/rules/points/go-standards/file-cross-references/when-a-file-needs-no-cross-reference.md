---
kind: directive
level: MAY
stage:
---
**A file MAY skip a cross-reference when it is:**
- Standalone in package (no strong coupling to siblings)
- Only related through package's public API
- Relationship is obvious from filename alone (see "Not a Directory Listing" below)
