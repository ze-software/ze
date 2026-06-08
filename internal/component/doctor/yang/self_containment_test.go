package yang

import (
	"strings"
	"testing"
)

func TestDoctorCmdSchemaOwnsShowDoctor(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:doctor"`,
		"container doctor",
	} {
		if !strings.Contains(ZeDoctorCmdYANG, want) {
			t.Errorf("ze-doctor-cmd.yang must declare %q so removing the doctor plugin removes the surface", want)
		}
	}
}
