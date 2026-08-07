---
kind: directive
level:
stage:
---
1. Write fixed bytes (marker, type)
2. **Skip** length field -- save position (`lengthPos := off; off += 2`)
3. Write payload forward at advancing offset
4. **Backfill** length at saved position (`buf[lengthPos] = byte(totalLen >> 8)`)
