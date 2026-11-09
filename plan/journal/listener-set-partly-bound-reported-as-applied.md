# A listener set partly bound is reported as applied

A service configured with several listen addresses binds each one on its own.
One bind fails, the failure is only logged, the others serve, and the start or
the commit reports success, so the service runs half-bound with nothing an
operator reads saying so.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-09 | - | BMP receiver, `startListenerLocked` (`internal/component/bgp/plugins/bmp/receiver.go`) | `applyReceiverConfig` binds each wanted address alone and returns nothing; a failed bind is logged at Error and the rest serve, so start and reload succeed with the BMP receiver half-bound. SSH had the same shape until home-cpe child 7 made `bindListeners` all or none (7ff7614e5f) | open: bind the wanted set all or none, keep the old set on a reload failure, and answer the error to the config transaction |
