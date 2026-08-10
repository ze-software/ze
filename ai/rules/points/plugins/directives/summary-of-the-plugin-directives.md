---
kind: directive
level: MUST
stage:
---
- **A plugin MUST own its ENTIRE feature surface. Removing the plugin MUST make every one of its features disappear; every OTHER plugin and the core MUST keep working.** Section "Plugin Self-Containment".
- **A plugin that calls a same-process-effect function in another package directly MUST check `sdk.Plugin.IsInternal()` and then refuse to start, or warn, when it runs external.** Section "Plugin Process Boundary".
- **MUST NOT use switch/case to dispatch subcommands: register each handler into a dispatcher, then call `Dispatch(args)`.** Section "Registration-Based Dispatch".
- **A compile-out-able feature MUST be declared ONCE in `feature-gates.txt`, and an always-on (untagged, non-test) package MUST NOT import it.** Section "Feature-Gate Registration".
