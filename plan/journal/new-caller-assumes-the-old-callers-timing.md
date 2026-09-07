# New caller assumes the old caller's timing

A function gets a second caller. Everything it does was safe because the FIRST
caller could only run at one point in the lifecycle: after the registries are
frozen, after the fan-out has run, after the peers are up. The new caller fires
from a different event, one that can arrive earlier, so the same call now runs
before the state it depends on exists, or repeats work that must happen once.

Nothing in the function says which stage it needs. The sentence that states it
sits at the OLD call site, or in the comment the new caller was written from, so
the reviewer reads a claim about timing rather than a check on it.

The tell is a new call whose comment asserts a lifecycle fact ("this runs after
X") instead of testing one, next to a trigger the same change has just made
reachable.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-07 | spec-plugin-respawn-leaf-restarts-nothing | plugin engine, `restartHandshake` (`internal/component/plugin/server/restart.go`) | The restart path was given `sendPostStartupToNames` so a restarted plugin runs its `OnAllPluginsReady` handler, under the comment "the replacement joins after signalStartupComplete has run". That held for the one caller it had, a config-reload rollback. The same spec added a second trigger: a plugin's own exit reaches `applyFailurePolicy`, and `runPluginPhase` starts its supervisors at the end of EACH phase, so a plugin that exits between two phases is restarted before `signalStartupComplete`. Its replacement then got the callback twice, once early and once from the fan-out, and the handlers are written for a single call. The early one also ran before the registries were frozen, which is the state the callback's contract promises the handler | fixed in the closure. `Server.startupComplete` (`server/startup.go`) reads the `startupDone` channel without waiting, and the delivery is made only when it answers true; before that the coming `sendPostStartupToAll` reaches the replacement, because it enumerates every process the manager holds. `TestRestartBeforeStartupCompletesLeavesTheFanOutToDeliverOnce` reds against the unguarded call in 1.20s |
