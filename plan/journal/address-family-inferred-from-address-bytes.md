# Address family inferred from address bytes

An address representation that admits both IPv4 and mapped IPv6 is used to
infer a family already declared by its containing prefix or wire record.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-13 | spec-rfc4301-architecture-gaps | netlink `selFromPolicy`, `xfrm_policy_linux.go` | A 128-bit mapped IPv6 prefix became AF_INET, differing from its state scope | fixed: `352f231413` selects family from mask width and copies all IPv6 bytes. Mapped-host/network tests first failed, then passed in `job-ipsec-scope-params-family-green-67381781.log`; upstream-style tests and ordered patch series landed in `c9ce6ba81d` |
