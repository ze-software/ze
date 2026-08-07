---
kind: directive
level: MUST
stage:
---
- **A plugin owns its ENTIRE feature surface. Remove the plugin and every one of its features disappears; every OTHER plugin and the core keep working.** Section "Plugin Self-Containment".
- **A plugin that calls a same-process-effect function in another package directly MUST check `sdk.Plugin.IsInternal()` and then refuse to start, or warn, when it runs external.** Section "Plugin Process Boundary".
- **Never use switch/case to dispatch subcommands: register each handler into a dispatcher, then call `Dispatch(args)`.** Section "Registration-Based Dispatch".
- **A compile-out-able feature is declared ONCE in `feature-gates.txt`, and no always-on (untagged, non-test) package may import it.** Section "Feature-Gate Registration".
