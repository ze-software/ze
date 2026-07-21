### `static` functional suite `004-show` -- pre-existing, obsolete config syntax + darwin daemon boot

Discovered 2026-07-21 while implementing `spec-fixit-static-per-route-isolation`
(unrelated: that spec touches Go apply/skip logic, not config parsing). Running
`make ze-static-test` on darwin skips the five `needs-linux` tests (001, 002,
003, 005, 006) and runs `004-show`, which fails:

```
error: load config: parse config: line 7: unknown field in route: next-hop (line 7)
expected exit code 0, got 1
```

Two independent pre-existing problems, neither caused by this spec:

1. **Obsolete syntax.** `test/static/004-show.ci` still writes the flat
   `next-hop 10.0.0.1 { weight 3; }` form. The current static YANG schema models
   next-hops as a nested container: `next { hop 10.0.0.1 { weight 3; } }` (see
   `001-boot-apply.ci`). `parseRoute` (`internal/plugins/static/config.go`)
   rejects the unknown `next-hop` field. Fix: rewrite the two routes in the 004
   config to the `next { hop ... }` form.

2. **darwin cannot boot a static daemon.** Even with valid syntax, booting `ze -`
   with a static section on darwin fails before `OnConfigure`: the `interface`
   component is auto-loaded and aborts the shared startup driver with
   `interface: no backend configured and no OS default available` (netlink is
   linux-only). So `004-show` (no `option=needs-linux`) can never pass natively
   on darwin. Fix: add `option=needs-linux` (like the other five) and a `setup.py`
   creating a dummy interface so the gateways resolve -- but note that fixing the
   syntax also changes the expected `static show` output once this spec's
   per-route isolation is in effect (unreachable gateways now show `skipped`),
   so 004's expectations must be revisited together with a `setup.py`.

Scope: `static` is a release-evidence-only suite (`mk/test-functional.mk`: not in
the default functional list), so this does NOT block `make ze-verify`. Left for a
session that owns the static-show test rather than expanding the isolation spec
into an entangled test rewrite. The new `007-per-route-isolation.ci` added by
this spec is correctly gated `needs-linux`.
