---
kind: directive
level: MUST
stage:
---
**A shard that still holds a live row MUST survive its source spec, and keep its source-keyed name.** The row's home is the destination spec named in its Destination cell. The shard is only where the row is written down, so deleting the shard deletes a record of live work whose home is somewhere else entirely.
