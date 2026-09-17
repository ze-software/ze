package doctor

import (
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestDoctorCheckCodesRegistered looks up every code this component's checks
// put in a diagnostic.
//
// The codes are referenced rather than spelled, so this asks the one question
// the compiler cannot: does the registry still hold the code the constant
// names? A builtin entry deleted or renamed turns this red.
//
// VALIDATES: `ze explain <code>` resolves every code `ze doctor` reports here.
// PREVENTS: a diagnostic an operator cannot look up.
func TestDoctorCheckCodesRegistered(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()

	// The last four are emitted by the Linux-only checks (checks_linux.go). The
	// registry answers for them on every platform, and `ze explain` is asked on
	// every platform too.
	codes := []string{
		diagnostic.CodeDoctorStoreIntegrity,
		diagnostic.CodeDoctorStorePermissions,
		diagnostic.CodeDoctorDiskSpace,
		diagnostic.CodeDoctorWriteDestination,
		diagnostic.CodeDoctorTLSMissing,
		diagnostic.CodeDoctorTLSInvalid,
		diagnostic.CodeDoctorTLSExpired,
		diagnostic.CodeDoctorPKICert,
		diagnostic.CodeDoctorModuleMissing,
		diagnostic.CodeDoctorRandomSeed,
		diagnostic.CodeDoctorIfaceSelectorUnmatched,
		diagnostic.CodeDoctorIfaceSelectorAmbiguous,
	}
	for _, code := range codes {
		if diagnostic.Lookup(code) == nil {
			t.Errorf("diagnostic code %q is emitted but not registered", code)
		}
	}
}
