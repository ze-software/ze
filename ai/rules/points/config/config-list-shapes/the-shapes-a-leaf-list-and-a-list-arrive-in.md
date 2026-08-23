---
kind: table
level:
stage:
---
`Tree.ToMap` does not hand a reader what the YANG node type suggests, and JSON
delivery adds one more shape on top.

| Node | Members | Shape in process | Shape after JSON |
|------|---------|------------------|------------------|
| `leaf-list` | none active | key absent | key absent |
| `leaf-list` | exactly one | bare `string` | bare `string` |
| `leaf-list` | two or more | `[]string` | `[]any` |
| `list` | any count, one included | `map[string]any` keyed by the list key | `map[string]any` keyed by the list key |

A `list` is never a slice, and its key leaf is the map key rather than a field
inside the entry.
