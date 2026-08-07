---
kind: directive
level:
stage:
---
**Reading the Status column of the closing spec's OWN shard is a NEW step, and no earlier check covers it.** The grep closure already requires ("Spec Closure" above, "Closure resolves the spec's deferral rows") searches every shard for this spec as a **Destination**. It never reads the closing spec's own shard as a **Source**. Do not assume the existing grep answered this question: it answers a different one.
