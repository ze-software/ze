// Design: docs/architecture/doctor-and-health-checks.md -- shared private artifact failures.
package web

import "github.com/ze-software/ze/internal/component/config/artifact"

func reportBackupStorageFailure() { artifact.ReportStorageFailure() }
