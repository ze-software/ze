---
kind: note
level:
stage:
---
Step 3's custom kernel is conditional, not automatic: use `--kernel` only when a
`CONFIG_*` the lab needs is absent from the stock Alpine kernel. L2TP and PPPoE
need it (`CONFIG_PPPOL2TP`, `CONFIG_PPPOE`); VRRP does not, because the stock
Alpine 6.12.13-0-virt kernel already creates macvlan (bridge mode), bridge, veth
and netns (probed 2026-07-15). Adding `--kernel` when it is not needed forces a
~30-minute `make ze-kernel` build on everyone who runs the lab, so probe the
stock kernel before reaching for it.
