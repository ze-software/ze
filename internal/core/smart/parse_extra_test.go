// VALIDATES: ParseNVMeBuf decodes the AvailableSpare (byte 3) and PercentUsed
// (byte 5) wear fields of the NVMe SMART log page and the temperature field at
// its kelvin-to-celsius boundary.
// PREVENTS: the disk-wear indicators (spare capacity, endurance used) being
// misread from the wrong offset, or a 273 K reading not resolving to 0 C.

package smart

import (
	"encoding/binary"
	"testing"
)

func TestParseNVMeBufWearFields(t *testing.T) {
	var buf [512]byte
	buf[0] = 0                                   // no critical warning -> healthy
	buf[3] = 90                                  // AvailableSpare percent
	buf[5] = 7                                   // PercentUsed (endurance)
	binary.LittleEndian.PutUint16(buf[1:3], 313) // 40 C

	info := ParseNVMeBuf(&buf)
	if !info.Healthy {
		t.Error("Healthy = false, want true for zero critical-warning byte")
	}
	if info.AvailableSpare != 90 {
		t.Errorf("AvailableSpare = %d, want 90", info.AvailableSpare)
	}
	if info.PercentUsed != 7 {
		t.Errorf("PercentUsed = %d, want 7", info.PercentUsed)
	}
	if info.TempCelsius != 40 {
		t.Errorf("TempCelsius = %d, want 40 (313 K)", info.TempCelsius)
	}
}

func TestParseNVMeBufKelvinBoundary(t *testing.T) {
	var buf [512]byte
	// Exactly 273 K: the kelvin>0 branch is taken and must resolve to 0 C.
	binary.LittleEndian.PutUint16(buf[1:3], 273)

	info := ParseNVMeBuf(&buf)
	if info.TempCelsius != 0 {
		t.Errorf("TempCelsius = %d, want 0 (273 K boundary)", info.TempCelsius)
	}
}
