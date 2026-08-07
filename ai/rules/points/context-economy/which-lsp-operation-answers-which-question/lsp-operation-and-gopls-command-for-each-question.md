---
kind: table
level:
stage:
---
| Question | LSP tool operation | `gopls` CLI, available everywhere | What comes back |
|----------|-----------|-----------------|-----------------|
| What is in this file? | `documentSymbol` | `gopls symbols <file>` | every symbol with its line range: the map you would otherwise read the whole file to build |
| What does this one symbol declare or say? | `goToDefinition`, then `hover` | `gopls definition <file>:<line>:<col>` | the declaration and its doc comment, not the file around it |
| Who calls this? | `findReferences` | `gopls references <file>:<line>:<col>` | every call site as file plus line. `grep` on a common name returns the comments and the string literals too |
| Who calls this, and from inside WHICH function? | `callHierarchy` | `gopls call_hierarchy <file>:<line>:<col>` | each caller's range AND the enclosing function that `references` leaves you to work out |
| Where does a name I can spell actually live? | `workspaceSymbol` | `gopls workspace_symbol <name>` | the file holding it, without guessing a directory |
| Does this file compile, and with what errors? | (diagnostics) | `gopls check <file>` | the type errors for that file. Silence and exit 0 mean clean |
