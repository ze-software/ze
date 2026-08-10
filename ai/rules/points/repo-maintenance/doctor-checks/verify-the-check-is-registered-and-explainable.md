---
kind: directive
level: MUST
stage:
---
- **After implementation, the check MUST be verified as registered and explainable: `go test ./internal/component/doctor -run 'TestDoctorCoverageCodesRegistered|TestRunChecksExecutesRegisteredPluginCheck'`**
- **If you added a runtime dependency and no registered doctor check declares its `doctor-*` code, you missed the readiness check or its diagnostic metadata.**
