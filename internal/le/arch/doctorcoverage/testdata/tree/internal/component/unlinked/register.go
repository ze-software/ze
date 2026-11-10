package unlinked

import "github.com/ze-software/ze/internal/core/diagnostic"

func init() {
	diagnostic.RegisterDoctorCheck(unlinkedDoctorCheck)
}
