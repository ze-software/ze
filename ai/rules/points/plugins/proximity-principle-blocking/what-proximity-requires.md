---
kind: table
level:
stage:
---
| Rule | Meaning |
|------|---------|
| All code for a concern in its folder | Commands, handlers, registration, logic, not scattered across packages |
| No external references to internals | Infrastructure, reactor, other units never import a specific plugin/command module |
| Blank import is the only coupling | A single `_ "package"` triggers init(); removing it cleanly disables the unit |
| Engine core works without any command module | Reactor, FSM, wire layer must function without CLI command handlers |
