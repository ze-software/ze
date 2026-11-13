# Exit cleanup runs after the transport it needs has closed

A component's cleanup on its way out has to reach another party (withdraw a
route, release a claim, send a last message), but a deferred stop that runs
first, often inside a library call the component does not own, has already
closed the channel that call needs. The cleanup is refused every time, so the
work it was written to do never happens on that path.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-13 | single-owner-lifecycle-5-session-end-locals (review) | `sdk.Plugin.Run`, ddos-flowspec `runEngine` | `Run` defers `CloseCallbacks`, which calls `StopDispatch`, so an in-process plugin's cleanup after `Run` gets `ErrBridgeClosed`: `stopResponder`'s withdrawal is always refused, and a plugin ending while the daemon runs leaves its FlowSpec rule at the peers until its next start. Only a forked plugin withdraws there | open: the engine withdraws a plugin's announcements when its generation ends, or the SDK runs exit cleanup before dispatch stops |
