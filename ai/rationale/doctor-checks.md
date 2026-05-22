# Rationale: Doctor Checks

Ze targets gokrazy appliances: no systemd, no package manager, no shell,
no `apt install`. When a runtime dependency is missing (a socket, a file,
a kernel module, a port), there is no interactive recovery path. The
appliance either works or it doesn't.

`ze doctor` is the only way an agent (human or AI) can verify system
readiness before hitting a cryptic runtime failure. Without the check,
the failure mode is: ze starts, silently does nothing because dependency X
is absent, someone debugs for an hour before discovering the real cause.

The check costs one function (a diagnostic that probes for the dependency
and reports present/absent with severity). The missing check costs an
incident every time a new deployment forgets the dependency.

Every new runtime dependency (file, socket, kernel module, port, external
binary) must come with a doctor check so that `ze doctor` remains a
single-command proof of readiness. The alternative is a growing list of
undocumented prerequisites that operators discover through failure.
