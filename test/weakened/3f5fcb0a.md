| Test | Reason |
|---|---|
| rpf_linux_test | Nothing left the suite: the skip belongs to the new TestReplaceIPv6RPFUnprivileged, which proves the unprivileged path and would program the host nftables as root, where the unprivileged path does not exist. Red without the empty-set permission guard, green with it. |
