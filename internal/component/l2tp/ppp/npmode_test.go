// Design: docs/architecture/l2tp/bng-5-pppoe.md -- shared unit mode API.
// Related: npmode.go -- typed mode admission and Linux ABI mapping.

package ppp

import "testing"

// TestNPModeKernelValue checks the actual Linux enum values independently of
// Ze's named enum, and proves zero and unnamed input cannot request forwarding.
func TestNPModeKernelValue(t *testing.T) {
	for _, tc := range []struct {
		mode NPMode
		want int32
	}{
		{NPModePass, 0}, {NPModeDrop, 1},
	} {
		got, err := tc.mode.kernelValue()
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Fatalf("mode %d = %d, want Linux %d", tc.mode, got, tc.want)
		}
	}
	for _, mode := range []NPMode{NPModeUnspecified, 3, 255} {
		if _, err := mode.kernelValue(); err == nil {
			t.Fatalf("invalid mode %d admitted", mode)
		}
	}
}
