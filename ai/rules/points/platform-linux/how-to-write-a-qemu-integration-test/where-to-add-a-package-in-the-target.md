---
kind: fence
level:
stage:
---
```makefile
ze-qemu-integration-test:
    python3 scripts/evidence/qemu-run.py \
        --packages "nftables iproute2 iputils-ping kmod iptables" \
        --run 'go test -tags integration -count=1 -timeout 120s \
            ./internal/component/iface/... \
            ./internal/component/config/system/... \   # <-- add here
            ...'
```
