---
kind: note
level:
stage:
---
`authz.Store.Authorize` (`internal/component/authz/authz.go`): with no
assignment and no config users (`hasUsers == false`) it returns
`BuiltinAdminProfile()`. An empty profile set is indistinguishable from "never
seen", because `aaa.RecordLoginProfiles`
(`internal/component/aaa/login_profiles.go`) early-returns on
`len(profiles) == 0` and records nothing. The zero value means ADMIN, so an
empty profile is not the same as no access.
