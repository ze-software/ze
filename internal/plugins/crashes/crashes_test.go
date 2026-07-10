// VALIDATES: showFile returns exit code 1 with a "not found" notice for empty
// crash content and exit code 0 when content is present.
// PREVENTS: the offline crash viewer reporting success on a missing report (or
// failure on a present one), which would mislead an operator inspecting a crash.

package crashes

import "testing"

func TestShowFile(t *testing.T) {
	if rc := showFile(""); rc != 1 {
		t.Errorf("showFile(empty) = %d, want 1", rc)
	}
	if rc := showFile("panic: boom\ngoroutine 1 ...\n"); rc != 0 {
		t.Errorf("showFile(content) = %d, want 0", rc)
	}
}
