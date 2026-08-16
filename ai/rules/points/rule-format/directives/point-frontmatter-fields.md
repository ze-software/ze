---
kind: table
level:
stage:
---
| Field | Values | Meaning |
|-------|--------|---------|
| `kind` | `directive`, `table`, `note`, `heading`, `fence` | What the block IS. `heading` and `fence` are structural, so `make ze-rules-gate-map` excludes them from the gated and ungated counts |
| `level` | `MUST`, `MUST NOT`, `SHOULD`, `SHOULD NOT`, `MAY`, or empty | The strongest RFC 2119 level the body states. About 95% of the corpus states none, and empty is the normal value |
| `stage` | empty | Reserved, and deliberately empty everywhere today. It is what will let a design-phase agent skip implementation directives. Leave it empty |
| `rationale` | a repo-relative path, or the line absent | The record of WHY this instruction exists: a `plan/journal/<class>.md` class file, or an `ai/rationale/*.md` file. `make ze-rules-gate-map` fails when it names no file on disk |
| `excepted-by` | one or more point ids, comma-separated, or the line absent | The point or points that carve an exception out of THIS one. Declared on the GENERAL point, because a reader who stops at the general statement is the one who is misled. `make ze-rules-gate-map` fails when it names no point, so deleting an exception can never be silent |
