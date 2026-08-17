---
kind: note
level:
stage:
---
Step 3's custom kernel is conditional for a LAB, not automatic: use `--kernel`
only when a `CONFIG_*` the lab needs is absent from the stock Alpine kernel.
L2TP and PPPoE need it (`CONFIG_PPPOL2TP`, `CONFIG_PPPOE`); VRRP does not,
because the stock Alpine kernel already creates macvlan (bridge
mode), bridge, veth and netns. Probe the stock kernel before
reaching for it, so a lab that gains nothing does not gain a precondition.

`make ze-kernel-build` routes through the durable architecture- and config-keyed cache
under `~/.cache/ze`, so it materializes on a cache hit and builds only on a miss
or after a config fragment changes. The two functional targets
(`ze-qemu-test-all`, `ze-qemu-needs-linux-test`) use `--kernel` unconditionally.
