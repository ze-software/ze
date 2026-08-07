---
kind: directive
level:
stage:
---
- **After implementation, verify the check is registered and explainable: `go test ./internal/component/doctor -run 'TestDoctorCoverageCodesRegistered|TestRunChecksExecutesRegisteredPluginCheck'`**
- **If you added a runtime dependency and no registered doctor check declares its `doctor-*` code, you missed the readiness check or its diagnostic metadata.**
