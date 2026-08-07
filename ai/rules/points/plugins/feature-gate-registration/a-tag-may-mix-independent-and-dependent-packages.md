---
kind: note
level:
stage:
---
A tag may MIX independent and dependent packages (feature-gate-12): `ze_radius`
gates `internal/component/radius` (RADIUS system auth, usable alone) AND
`internal/component/l2tp/plugins/authradius` (needs the BNG). The generator
splits such a tag's group by per-package constraint: `all_ze_radius.go` carries
the plain tag with the independent imports, and `all_ze_radius_ze_l2tp.go`
carries `//go:build ze_l2tp && ze_radius` with the nested subset, so a
radius-without-l2tp build keeps its schema and an l2tp-with-local-auth build
links zero radius symbols. A tag whose packages ALL share one constraint keeps
the single historic `all_<tag>.go` file.
