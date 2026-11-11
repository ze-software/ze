| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-11 | - | `TestRecoveryQueuedDeliveryTransport/ipc/*`, `TestRSFastReceiptRecipientExactlyOnce/ipc` (`plugin/process`) | Red, full tags: DirectBridge activates in the ipc subtests. Since ae1aac228b `registry.Lookup` returns a clone, so `reg.RunEngine = wrapper` writes a copy; `GetInternalPluginRunner` runs the registry's own RunEngine and the subtests no longer test IPC | open: a registry test seam that installs a wrapped RunEngine for one registration, honouring the seal |
