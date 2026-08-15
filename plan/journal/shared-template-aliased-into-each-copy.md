# Shared template aliased into each copy

A template is merged or copied into many derived structures, and the merge
assigns the template's nested value rather than a copy. Every derivative then
points at the same object, so the next write into one derivative changes what
its siblings hold. The first derivative is usually correct when it is built,
which is why the defect reads as ordering luck rather than aliasing.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-15 | fixit-peer-process-event-filter | bgp config resolution | `deepMergeAt` (`internal/component/bgp/config/resolve.go`) assigned `dst[k] = srcVal` whenever dst held no map at that key, so a member peer's resolved tree ALIASED its group's nested containers. `ResolveBGPTree` passes the group's own `groupFields` as layer 2 without copying it (the bgp-level defaults beside it ARE copied), so layer 3 then merged the member's values into the GROUP's map. A group naming `attach process looking-glass { receive [ update-received state ] }` with one member restating `receive [ state ]` left every sibling of that member holding `[ state ]` too. Every nested container carries the same exposure, not only an attach block. Found by `TestGraphMemberListReplacesGroupList` (`internal/component/bgp/config/delivery_graph_test.go`), whose second half asserts the siblings keep the group's list | fixed at the root rather than at the call site: `deepMergeAt` copies the map (`deepCopyMap`), so `deepMergeMaps` leaves dst independent of src for every caller, `resolveDynamicGroup` included. The cumulative leaf-list branch beside it had the same class through a slice: `append(dstSlice, srcSlice...)` can write into a backing array a sibling shares, so it now builds a fresh slice. Proven both ways: the test fails on the aliasing build with the sibling holding `[state]`, and the whole `internal/component/bgp/config` package passes with the copy |
