---
kind: directive
level: MUST
stage:
---
**A red you have not looked at MUST NOT be assumed to be somebody else's.** Read the failing assertion once, and decide whether your own diff produces the symbol it names.
**A change that could break a package you did not edit MUST have its reverse-dependency closure compiled once.** Scope-to-changed tests the packages you edited, not the packages your edit breaks TRANSITIVELY: a new import broke `bgp/config` through `plugin/all`, a missing YANG typedef failed a consumer rather than the plugin that introduced it, and adding a plugin invalidates the `plugin/all` golden snapshots. `go vet` over that closure answers it in seconds, and the full gate is not the way to ask.
