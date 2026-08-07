---
kind: directive
level:
stage:
---
**A shard that still holds a live row SURVIVES its source spec, and keeps its source-keyed name.** The row's home is the destination spec named in its Destination cell. The shard is only where the row is written down, so deleting the shard deletes a record of live work whose home is somewhere else entirely.
