---
kind: table
level:
stage:
---
| The fixture | What the change did to it | Why nothing named it |
|-------------|---------------------------|----------------------|
| `test/plugin/bgp-rs-reactor-fastpath-fallback.ci` | its config stopped feeding `bgp-rs`, so it exercised the reactor fast path instead of the `bgp-rs` fallback rail its name certifies | it kept PASSING and no assertion moved. It burned its authored budget on every run, 31.5s against a 15s timeout, and it was found days later through suite instability |
| `test/plugin/role-otc-rs-withdraw-eor.ci` | went RED for the same missing attachment | the commit never edited it, so the relaxation audit never opened it. That audit reports the diff, and this file was not in the diff |
| `test/plugin/local-pref-strip-ebgp.ci` | gained a route server it deliberately ran without, because a peer that attaches `bgp-rs` becomes a destination in `selectForwardTargets` (`internal/component/bgp/plugins/rs/server_forward.go`) | its header still states that no route-server plugin is loaded. A header is read by people, and no gate compares one against the config below it |
