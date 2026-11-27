---
spec: dropped-error-7-plugin-infra
topic: Plugins
found-by: review
---

## Before

Six runtime failures in plugin management, the managed hub listener, and the Go plugin SDK logged only at debug level. Operators at the default WARN log level saw no log line and no entry in `ze show errors` or `ze show doctor`:

- A non-`ESRCH` failure to send `SIGKILL` to a forked plugin process group stayed at debug level. Child processes can keep running.
- A failure to close the parent stderr write pipe or arm the post-reap read deadline stayed at debug level. A lingering child can delay stop or restart.
- A failed RPC error or result reply on a live plugin connection stayed at debug level. The plugin waited without its answer.
- A non-closed error from aborting a collided plugin runtime connection stayed at debug level. Teardown does not retry the close.
- A failed `config-fetch` reply on a live managed hub connection stayed at debug level. The managed device kept its previous configuration.
- A failed async callback answer on a live SDK connection stayed at debug level. The engine waited for the callback receipt.

## After

Each live failure now logs at WARN with what failed, why it matters, and the fix command. Each daemon site also attaches a registered doctor code and raises an event for `ze show errors` and `ze show doctor`:

- Process-group `SIGKILL` failures other than `ESRCH` log at WARN with `doctor-plugin-group-signal-failed` and fix `ze show log recent component plugin.relay`. `ESRCH` stays quiet.
- Parent stderr write-close and post-reap deadline failures log at WARN with `doctor-plugin-stderr-drain-unbounded` and fix `ze show log recent component plugin.relay`. The deadline warning logs once per reader, and teardown close races stay quiet.
- Live RPC reply failures log at WARN with `doctor-plugin-answer-undelivered` and fix `ze request plugin restart <plugin>`. Events rate-limit per plugin, and closed or stopping plugins stay quiet.
- Non-closed collision abort errors log at WARN with `doctor-plugin-request-refused` and fix `ze request plugin restart <plugin>`. Already-closed connections stay quiet.
- Live managed `config-fetch` reply failures log at WARN with `doctor-managed-config-fetch-undelivered` and fix `ze show fleet devices`. The event names the client and error with no client secret. Closed or stopping sessions stay quiet.
- Live SDK async callback reply failures log at WARN with the plugin name, request ID, error, and fix `ze show log recent component plugin.relay`. Stopped loops and closed connections stay quiet.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
