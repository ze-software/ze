---
kind: note
level:
stage:
---
Step 3's custom kernel is conditional for a LAB, not automatic: use `--kernel`
only when a `CONFIG_*` the lab needs is absent from the stock Alpine kernel.
L2TP and PPPoE need it (`CONFIG_PPPOL2TP`, `CONFIG_PPPOE`); VRRP does not,
because the stock Alpine 6.12.13-0-virt kernel already creates macvlan (bridge
mode), bridge, veth and netns (probed 2026-07-15). Probe the stock kernel before
reaching for it, so a lab that gains nothing does not gain a precondition.

**The cost that used to decide this is gone (2026-08-07).** `make ze-kernel`
routes through the durable architecture- and config-keyed cache under
`~/.cache/ze`, so it materializes in seconds on a hit and builds only on a miss
or after a config fragment changes. The older advice, that `--kernel` "forces a
~30-minute build on everyone who runs the lab", described a checkout where the
kernel lived in `tmp/`. It now costs a copy. The two functional targets
(`ze-qemu-all-test`, `ze-qemu-needs-linux-test`) use `--kernel` unconditionally
for that reason.
