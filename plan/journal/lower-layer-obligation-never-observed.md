# An obligation met by a lower layer is never observed

Ze meets a requirement by installing state that a lower layer acts on: the
kernel's conntrack, its forwarding path, VPP's NAT. The requirement is written
up as met by that layer, and no test drives a packet through the state Ze
installs to watch the layer do it, so the claim rests on a reading of the
layer, or on none (`ai/rules/rfc-compliance.md`: a requirement met through a
lower layer still carries a test of what Ze produces).

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-15 | - | ICMP errors across NAT on both firewall backends: nft (`lowerMasquerade`, `lowerSNAT`, `lowerDNAT`, then nf_conntrack) and vpp (`applyMasquerade`, `applySNAT`, `applyDNAT`, then VPP NAT44-ED) | No `.ci`, integration or lab check sends a fragmentation-needed, port-unreachable or time-exceeded error through a NAT Ze installed, so RFC4787-9-1, RFC4787-10-1 and RFC5382-7.3-1 (and the 9-3 and 7.3-2 SHOULDs) rest on a reading of the kernel and an unverified VPP. Read, not run | open: per backend, a test that the translated error reaches the inner host and the mapping survives |
