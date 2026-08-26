---
kind: note
level: MUST
stage:
---
On Linux: the invoking user MUST be in the `kvm` group. `/dev/kvm` is
`root:kvm` 0660, so a user outside it does not get a slow run, it gets no run:
qemu exits with `Could not access KVM kernel module: Permission denied` and the
calling evidence script reports the generic "did not reach SSH within the
timeout", which reads as flakiness. `./le setup` checks this as `kvm-access`
and applies `sudo usermod -aG kvm $USER`; the new group only reaches a new
login, so use `sg kvm -c '<command>'` in an existing shell. A host with no
`/dev/kvm` reports `n/a` and legitimately runs under TCG.
