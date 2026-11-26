---
spec: plugin-restart-bounded-and-configuration-true
topic: Plugins
found-by: review
---

## Before
Removing a configuration section whose plugin was already stopped left the removed plugin in tab completion, kept `ze show doctor` and `ze show warnings` reporting it as stopped, and allowed `ze request plugin restart` to start it again from stale configuration. A refused configuration reload whose newly auto-loaded plugin failed startup left `doctor-plugin-stopped` active on the warning bus and marked plugin health down for a plugin the running configuration never named, and a rejected reload could stop a pre-existing dependency during background recovery. Restarting a slow or stuck forked plugin serialized unrelated plugin restarts behind a manager-wide respawn lock; automatic failure-policy restarts ran outside the configuration transaction lock with an unbounded handshake and could clobber concurrent startup barriers or lose track of earlier stopped processes still owing cleanup. Hitting the respawn limit while stage updates were in flight could re-create a deleted `ze_plugin_status` metric series; a refused `ze request plugin restart` cleared a plugin's respawn-limit disable and counters before checking refusals; running `ze request plugin restart` during daemon startup could race supervision; `ze show warnings` omitted the cleanup-owed refusal explanation and daemon-restart last resort shown by `ze show doctor`; and withdrawing 1,000,000 plugin-installed routes on restart took 3.192 s, exceeding the 3 s cleanup grace window.

## After
Accepted configuration reloads drop removed down plugins from the configured set, tab completion, `ze show doctor`, and `ze show warnings` while rejected reloads preserve them; failed auto-loads from a refused reload leave no `doctor-plugin-stopped` warning or stale configured entry, and rejected reloads stop only plugins they started without disturbing pre-existing dependencies. Plugin respawns serialize per plugin name rather than across the whole manager, failure-policy restarts serialize with reloads under bounded handshakes without clobbering concurrent startup barriers, and cleanup-owed tracking retains every stopped process per plugin name until all finish. Respawn-limit disables keep `ze_plugin_status` deleted under concurrent stage transitions, refused restarts preserve respawn-limit disable state and stop records, restarts requested during daemon boot are refused cleanly until startup completes, `ze show warnings` and `ze show doctor` render identical cleanup-owed messages with automatic bus refresh when cleanup ends, and chunked Loc-RIB batch withdrawal brings 1,000,000-route cleanup to 1.806 s, well inside the 3 s grace window.

## Corrects
none

## Exposure
none

## Upstream
none

## Asked by
none
