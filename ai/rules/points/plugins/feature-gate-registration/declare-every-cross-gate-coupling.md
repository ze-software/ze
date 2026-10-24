---
kind: directive
level: MUST
stage:
---
- **A plugin that consumes what only a plugin under another build gate produces MUST declare an edge to that producer, and the edge kind MUST follow whether config makes the consumer depend on it.** A dependency MAY be soft: a build without the provider still runs the owner and refuses only the config that needs it. When a setting makes the owner depend on the provider's output, the provider MUST go in `OptionalDependencies` with a `ConfigNeeds` entry naming that setting, so a build without it refuses the setting with the build tag that would carry it and a full build pulls the provider only when the setting holds. When the owner only listens and works without it, the provider MUST go in `StartAfter`, which orders without loading. A plugin that cannot work at all without another MUST declare it in `Dependencies`, and a build missing it refuses on every load path, naming the tag. An allowlist MUST NOT stand in for an edge. `./le plugin coupling check` refuses an undeclared coupling, and `docs/architecture/plugin/feature-gates.md`, "Declaring a cross-gate coupling", holds the mechanism.
