//go:build linux

package host

import "testing"

func TestDetectSMART_TestdataMode(t *testing.T) {
	d := &Detector{Root: "testdata"}
	info := d.detectSMART("sda")
	if info != nil {
		t.Error("expected nil in testdata mode")
	}
}
