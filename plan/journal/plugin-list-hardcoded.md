| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-04-03 | - | tests | adding a plugin broke `TestAllPluginsRegistered` | updated `all/all_test.go` |
| 2026-04-11 | - | tests | fixing one plugin test list broke the other | updated both files in the same commit |
| 2026-08-10 | fixit-vpp-ipsec-inoperable | tests | `scripts/evidence/effective-vpp.py` built its `ze` with a hardcoded `ze_core,ze_distro`, so every gated feature was missing and `make ze-deployment-vpp-test` died on `unknown top-level keyword: bgp`; its firewall fixture had also drifted from the YANG grammar unnoticed underneath that | `feature_tags()` derives the tags from `feature-gates.txt`, the source `ZE_FEATURES` reads, and the fixture keywords were corrected |
