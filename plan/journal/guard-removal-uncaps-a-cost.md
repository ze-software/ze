# Guard removal uncaps a cost

A guard that REJECTS an input is also, by construction, a limit on the work that
input can cause. Replacing "reject" with "handle" moves that limit onto the
handler, and a fix written against the correctness defect alone does not carry
it. Ask what the rejected input would have cost before you make it legal.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-10 | fixit-rs-community-strip-arity | bgp filter_community | the removal helper returned immediately on a buffer of more than one value. That guard was the defect, and unremarked it was also the cost cap. Admitting the multi-value buffer exposed a peer-controlled quadratic of 874 to 889 ms per destination peer at the attribute ceiling RFC 4271 sets. The first fix for THAT sized its map from the raw value count and allocated 939,312 bytes to store one entry | `newRemovalSet` chooses the membership representation once per attribute. It answers from a hintless read-before-insert map above a `min(source values, removal values)` threshold. Both peer-chosen shapes carry a byte ceiling in test, so neither side of the trade can grow in silence |
