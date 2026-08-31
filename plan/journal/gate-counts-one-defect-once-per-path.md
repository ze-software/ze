# A gate counts one defect once for each path that reaches it

A command declared at two paths is one declaration and two rows in every report
that walks the tree. The defect is in the declaration. The count is a property
of the walk.

The number then moves when nobody touched the defect. A reader who watches the
count reads an alias as a regression. A reader who fixes the count by deleting
the alias has removed a grammar rather than a fault.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-31 | - | `./le cli-grammar`, R9 over `announce flowspec` | The gate reports 12 grammar violations where it reported 6. All 12 are one defect. Six FlowSpec component keywords collide with a sibling namespace: `destination-ipv4`, `destination-ipv6` and `destination-port` sit beside `destination`, and `source-ipv4`, `source-ipv6` and `source-port` beside `source`. R9 (`checker.go`) asks for `destination ipv4` instead. The keywords arrived with the augment in `b47d1db6e` on 2026-08-30 and the gate has been red since. Making announce reachable at a second path on 2026-08-31 instantiated the same containers under `peer announce flowspec`, so each row appears twice | not fixed, and not this change's to fix. The names are the FlowSpec codec's operator vocabulary: `isComponentKeyword` (`internal/component/bgp/plugins/nlri/flowspec/plugin_encode_text.go`) is their single producer, and `parseComponentText` dispatches on them. A split of `destination-ipv4` into `destination ipv4` changes what an operator types and what the codec parses. It belongs to the plugin that owns the vocabulary. The exemption route exists: `cligrammar.go` counts indivisible compounds. It would take the count to zero and decide nothing, which is why this change does not take it |
