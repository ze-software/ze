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
// VALIDATES: `ze show diagnostic <code>` resolves every code `ze doctor` reports here.
// PREVENTS: a diagnostic an operator cannot look up.
func TestDoctorCheckCodesRegistered(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()

	// Platform-specific codes (such as random-seed) are still registered on
	// every platform so `ze show diagnostic` answers everywhere.
	codes := []string{
		diagnostic.CodeDoctorStoreIntegrity,
		codeDoctorStorePermissions,
		codeDoctorDiskSpace,
		codeDoctorStorageNotGrown,
		codeDoctorWriteDestination,
		codeDoctorRuntimeNotChecked,
		diagnostic.CodeDoctorTLSMissing,
		diagnostic.CodeDoctorTLSInvalid,
		diagnostic.CodeDoctorTLSExpired,
		diagnostic.CodeDoctorModuleMissing,
		codeDoctorRandomSeed,
	}
	for _, code := range codes {
		if diagnostic.Lookup(code) == nil {
			t.Errorf("diagnostic code %q is emitted but not registered", code)
		}
	}
}
